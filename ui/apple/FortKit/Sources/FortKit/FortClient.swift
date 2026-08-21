//
//  FortClient.swift
//  FortKit
//
//  The bounded HTTP/SSE client used by the native iPhone and Mac Phase 1
//  surfaces.
//
//  Reads and commands use URLSession's async APIs; selected-Channel updates use
//  URLSession.bytes(for:) to decode replacement snapshots from Server-Sent
//  Events.
//

import Foundation

#if canImport(FoundationNetworking)
import FoundationNetworking
#endif

#if canImport(Combine)
import Combine
#endif

/// A forward-compatible typed Fort server error code. Known Primary Channel
/// values are exposed as static members while newer server codes still decode.
public struct FortServerErrorCode: RawRepresentable, Codable, Sendable, Hashable {
    public let rawValue: String

    public init(rawValue: String) {
        self.rawValue = rawValue
    }

    public init(_ rawValue: String) {
        self.init(rawValue: rawValue)
    }

    public init(from decoder: Decoder) throws {
        let container = try decoder.singleValueContainer()
        self.init(try container.decode(String.self))
    }

    public func encode(to encoder: Encoder) throws {
        var container = encoder.singleValueContainer()
        try container.encode(rawValue)
    }

    public static let primaryAgentNotConfigured = Self("primary_agent_not_configured")
    public static let primaryAgentUnready = Self("primary_agent_unready")
    public static let primaryAgentDrift = Self("primary_agent_drift")
    public static let chatPolicyUnavailable = Self("chat_policy_unavailable")
    public static let chatAuthorityViolation = Self("chat_authority_violation")
    public static let primaryChannelInvariant = Self("primary_channel_invariant")
    public static let providerResultUnknown = Self("provider_result_unknown")
    public static let providerIncomplete = Self("provider_incomplete")
    public static let providerRefusal = Self("provider_refusal")
    public static let providerFailed = Self("provider_failed")
    public static let seatUnready = Self("seat_unready")
}

/// The structured `{code,error}` body returned by coded Fort failures.
public struct FortCodedError: Codable, Sendable, Hashable {
    public let code: FortServerErrorCode
    public let message: String

    enum CodingKeys: String, CodingKey {
        case code
        case message = "error"
    }
}

/// Errors surfaced by ``FortClient`` beyond the transport errors thrown by
/// `URLSession` itself.
public enum FortClientError: Error, Sendable {
    /// The server returned a non-2xx status for a request where that is fatal.
    /// `status` is the HTTP status code; `body` is the (possibly empty) response body.
    case httpStatus(status: Int, body: String, requestID: String? = nil)
    /// The response was not an `HTTPURLResponse` (should not happen over HTTP(S)).
    case nonHTTPResponse
}

extension FortClientError {
    /// Decodes a structured Fort failure without changing the stable enum case
    /// shape consumed by existing Apple clients.
    public var codedError: FortCodedError? {
        guard case .httpStatus(_, let body, _) = self else { return nil }
        return try? JSONDecoder().decode(FortCodedError.self, from: Data(body.utf8))
    }

    public var serverCode: FortServerErrorCode? { codedError?.code }
}

extension FortClientError: LocalizedError {
    public var errorDescription: String? {
        switch self {
        case .httpStatus(let status, _, let requestID):
            let correlation = requestID.map { " Request ID: \($0)." } ?? ""
            return "Fort returned HTTP \(status).\(correlation)"
        case .nonHTTPResponse:
            return "Fort returned an unexpected non-HTTP response."
        }
    }
}

/// The bounded client seam shared by direct and authenticated relay transport.
/// Production uses ``GatewayRelayTransport``; deterministic contract checks can
/// inject an in-memory stream without weakening the gateway's trust checks.
public protocol FortRelayTransporting: Sendable {
    func request(
        path: String,
        method: String,
        headers: [String: String]?,
        body: Data?,
        requestID: String
    ) async throws -> (data: Data, status: Int, requestID: String)

    func events(path: String, requestID: String) -> AsyncThrowingStream<Data, Error>
}

extension GatewayRelayTransport: FortRelayTransporting {}

/// The control-plane client. One instance owns one currently selected
/// direct-or-relay transport. Conforms to `ObservableObject` so SwiftUI
/// surfaces can hold it as `@StateObject` / `@EnvironmentObject`.
public final class FortClient: ObservableObject, @unchecked Sendable {

    /// An inert URL used by gateway-only clients before an authenticated relay
    /// is selected. Its custom scheme fails closed if a caller accidentally
    /// attempts a request while disconnected.
    private static let gatewayRequiredBaseURL = URL(string: "fort-gateway-required://disconnected")!

    /// The selected transport origin. Callers may observe it for identity and
    /// refresh behavior; only the closed transport actions below may change it.
    @Published public private(set) var baseURL: URL

    /// Monotonically changes whenever the underlying direct/relay transport is
    /// replaced, even when two gateway machines share the same public origin.
    @Published public private(set) var transportGeneration: UInt64 = 0

    private let session: URLSession
    private let decoder: JSONDecoder
    private let encoder: JSONEncoder
    private var relayTransport: (any FortRelayTransporting)?

    /// Creates a client pointed at `baseURL` (default `http://127.0.0.1:4087`).
    /// `session` is injectable for deterministic contract checks; apps use the
    /// cache-free streaming configuration created here.
    public init(
        baseURL: URL = URL(string: "http://127.0.0.1:4087")!,
        session: URLSession? = nil,
        relayTransport: (any FortRelayTransporting)? = nil
    ) {
        self.baseURL = baseURL
        self.relayTransport = relayTransport

        if let session {
            self.session = session
        } else {
            // SSE requires that responses are never coalesced or buffered by a
            // cache, and that the connection stays open indefinitely.
            let config = URLSessionConfiguration.default
            config.requestCachePolicy = .reloadIgnoringLocalCacheData
            config.urlCache = nil
            config.timeoutIntervalForRequest = 0 // no per-request timeout: SSE is long-lived
            self.session = URLSession(configuration: config)
        }

        // Models carry explicit snake_case CodingKeys, so we do NOT apply a
        // key-decoding strategy (that would double-convert and fail).
        self.decoder = JSONDecoder()
        self.encoder = JSONEncoder()
    }

    /// Creates the fail-closed client used by physical iPhone/TestFlight builds.
    /// Only ``useGateway(account:machine:)`` can attach a usable transport.
    public static func gatewayOnly() -> FortClient {
        FortClient(baseURL: gatewayRequiredBaseURL)
    }

    /// Routes all typed Fort API calls through the selected gateway machine's
    /// pinned, end-to-end encrypted relay instead of connecting to localhost.
    public func useGateway(account: GatewayAccount, machine: GatewayMachine) throws {
        guard let configuredGatewayURL = account.gatewayURL,
              let bearerToken = account.bearerToken,
              let publicKey = Data(base64Encoded: machine.publicKey)
        else { throw GatewayRelayError.invalidMachineKey }
        let gatewayURL = try GatewayAddress.normalize(configuredGatewayURL)
        if let pinned = account.pinnedPublicKeys[machine.machineID], pinned != machine.publicKey {
            throw GatewayRelayError.fingerprintChanged(expected: pinned, actual: machine.publicKey)
        }
        let computed = RelayFingerprint.of(publicKey: publicKey)
        guard computed == machine.fingerprint else {
            throw GatewayRelayError.fingerprintChanged(expected: machine.fingerprint, actual: computed)
        }
        let nextTransport = try GatewayRelayTransport(
            gatewayURL: gatewayURL,
            bearerToken: bearerToken,
            machineID: machine.machineID,
            machinePublicKey: publicKey
        )
        relayTransport = nextTransport
        baseURL = gatewayURL
        transportGeneration &+= 1
    }

    #if os(macOS) || (DEBUG && targetEnvironment(simulator))
    public func useDirectHost(_ url: URL) {
        relayTransport = nil
        baseURL = url
        transportGeneration &+= 1
    }
    #endif

    /// Clears an authenticated relay without converting the client into a
    /// localhost or LAN client. Physical iPhone sign-out uses this fail-closed
    /// state until another pinned gateway machine is selected.
    public func disconnectGateway() {
        relayTransport = nil
        baseURL = Self.gatewayRequiredBaseURL
        transportGeneration &+= 1
    }

    // MARK: - Phase 1 reads

    /// `GET /api/settings/primary-agent` — exact selected authority and inventory.
    public func primaryAgent() async throws -> PrimaryAgentView {
        try await get("/api/settings/primary-agent")
    }

    /// `GET /api/channels?state=…` — Phase 1 private Channels.
    public func primaryChannels(
        state: PrimaryChannelFilter = .open
    ) async throws -> [PrimaryChannelSummary] {
        try await get("/api/channels?state=\(state.rawValue)")
    }

    /// `GET /api/channels/{id}` — one bounded Channel snapshot.
    public func primaryChannel(id: String) async throws -> PrimaryChannelDetail {
        try await get("/api/channels/\(Self.primaryPathComponent(id))")
    }

    /// `GET /api/needs-you` — recoverable latest failed targets only.
    public func primaryNeedsYou() async throws -> [PrimaryNeedsYouItem] {
        try await get("/api/needs-you")
    }

    // MARK: - Agent Channel reads

    /// `GET /api/agent-options` — provider-neutral, server-resolved choices.
    public func agentOptions() async throws -> [AgentOption] {
        try await get("/api/agent-options")
    }

    /// `GET /api/agent-channels?state=…` — durable agent destinations.
    public func agentChannels(
        state: AgentChannelFilter = .open
    ) async throws -> [AgentChannelSummary] {
        try await get("/api/agent-channels?state=\(state.rawValue)")
    }

    /// `GET /api/agent-channels/{channel_id}` — one agent and its shortcuts.
    public func agentChannel(id: String) async throws -> AgentChannelDetail {
        try await get(agentChannelPath(channelID: id))
    }

    /// `GET /api/agent-channels/{channel_id}/conversations?state=…`.
    public func agentConversations(
        channelID: String,
        state: AgentConversationFilter = .open
    ) async throws -> [AgentConversationSummary] {
        try await get("\(agentChannelPath(channelID: channelID))/conversations?state=\(state.rawValue)")
    }

    /// `GET /api/agent-channels/{channel_id}/conversations/{conversation_id}`.
    public func agentConversation(
        channelID: String,
        conversationID: String
    ) async throws -> AgentConversationDetail {
        try await get(agentConversationPath(channelID: channelID, conversationID: conversationID))
    }

    /// `GET /api/agent-needs-you` — failures with their owning agent and transcript.
    public func agentNeedsYou() async throws -> [AgentNeedsYouItem] {
        try await get("/api/agent-needs-you")
    }

    /// `GET /api/schedules?state=…` — durable schedule snapshot.
    public func primarySchedules(
        filter: PrimaryScheduleFilter = .all
    ) async throws -> PrimaryScheduleList {
        try await get("/api/schedules?state=\(filter.rawValue)")
    }

    /// `GET /api/schedules/{id}` — one durable schedule with nearby occurrences.
    public func primarySchedule(id: String) async throws -> PrimaryScheduleDetail {
        try await get("/api/schedules/\(Self.primaryPathComponent(id))")
    }

    /// `GET /api/schedules/{id}/occurrences` — stable cursor pagination.
    public func primaryScheduleOccurrences(
        id: String,
        before: String? = nil,
        beforeID: String? = nil,
        limit: Int = 50
    ) async throws -> [PrimaryScheduleOccurrence] {
        var query = ["limit=\(limit)"]
        if let before, !before.isEmpty {
            query.append("before=\(Self.primaryQueryValue(before))")
        }
        if let beforeID, !beforeID.isEmpty {
            query.append("before_id=\(Self.primaryQueryValue(beforeID))")
        }
        let escapedID = Self.primaryPathComponent(id)
        return try await get("/api/schedules/\(escapedID)/occurrences?\(query.joined(separator: "&"))")
    }

    // MARK: - Commands

    /// `PUT /api/settings/primary-agent` — select one closed option ID.
    @discardableResult
    public func setPrimaryAgent(optionID: String) async throws -> PrimaryAgentView {
        try await put("/api/settings/primary-agent", body: PrimaryAgentSelectionRequest(optionID: optionID))
    }

    /// `DELETE /api/settings/primary-agent` — clear the future-Channel default.
    public func clearPrimaryAgent() async throws {
        try await requestNoContent(path: "/api/settings/primary-agent", method: "DELETE")
    }

    /// `POST /api/settings/primary-agent/recheck` — refresh exact readiness.
    @discardableResult
    public func recheckPrimaryAgent() async throws -> PrimaryAgentView {
        try await postEmpty("/api/settings/primary-agent/recheck")
    }

    // MARK: - Agent Channel commands

    /// `POST /api/agent-options/recheck` — bounded readiness probes only.
    @discardableResult
    public func recheckAgentOptions() async throws -> [AgentOption] {
        try await postEmpty("/api/agent-options/recheck")
    }

    /// `POST /api/agent-channels` — create from one opaque server option.
    @discardableResult
    public func createAgentChannel(optionID: String, name: String) async throws -> AgentChannelDetail {
        try await post(
            "/api/agent-channels",
            body: AgentChannelCreateRequest(agentOptionID: optionID, name: name)
        )
    }

    /// `PATCH /api/agent-channels/{channel_id}` — change name or state, never identity.
    public func updateAgentChannel(
        id: String,
        name: String? = nil,
        state: AgentChannelState? = nil
    ) async throws {
        let changes = [name != nil, state != nil].filter { $0 }.count
        guard changes == 1 else { throw AgentChannelRequestError.exactlyOneChangeRequired }
        try await requestNoContent(
            path: agentChannelPath(channelID: id),
            method: "PATCH",
            body: try encoder.encode(AgentChannelUpdateRequest(name: name, state: state?.rawValue))
        )
    }

    /// `POST /api/agent-channels/{channel_id}/conversations` — create an empty transcript.
    @discardableResult
    public func createAgentConversation(
        channelID: String,
        name: String
    ) async throws -> AgentConversationDetail {
        try await post(
            "\(agentChannelPath(channelID: channelID))/conversations",
            body: AgentConversationCreateRequest(name: name)
        )
    }

    /// `PATCH .../conversations/{conversation_id}` — change one presentation field or pin.
    public func updateAgentConversation(
        channelID: String,
        conversationID: String,
        name: String? = nil,
        state: AgentConversationState? = nil,
        pinned: Bool? = nil
    ) async throws {
        let changes = [name != nil, state != nil, pinned != nil].filter { $0 }.count
        guard changes == 1 else { throw AgentConversationRequestError.exactlyOneChangeRequired }
        let body = AgentConversationUpdateRequest(
            name: name,
            state: state?.rawValue,
            pinned: pinned
        )
        try await requestNoContent(
            path: agentConversationPath(channelID: channelID, conversationID: conversationID),
            method: "PATCH",
            body: try encoder.encode(body)
        )
    }

    /// `POST /api/agent-channels/{channel_id}/turns` — atomically create a
    /// Conversation and its first durable turn.
    @discardableResult
    public func postFirstAgentTurn(
        channelID: String,
        name: String,
        clientTurnID: String,
        text: String
    ) async throws -> AgentFirstTurnResult {
        try await post(
            "\(agentChannelPath(channelID: channelID))/turns",
            body: AgentFirstTurnRequest(
                name: name,
                clientTurnID: clientTurnID,
                text: text
            )
        )
    }

    /// `POST .../conversations/{conversation_id}/turns` — one nested durable turn.
    @discardableResult
    public func postAgentTurn(
        channelID: String,
        conversationID: String,
        clientTurnID: String,
        text: String
    ) async throws -> AgentTurnResult {
        try await post(
            "\(agentConversationPath(channelID: channelID, conversationID: conversationID))/turns",
            body: AgentTurnRequest(clientTurnID: clientTurnID, text: text)
        )
    }

    /// `POST .../targets/{target_id}/retry` — retries under the same parent identity.
    @discardableResult
    public func retryAgentTarget(
        channelID: String,
        conversationID: String,
        targetID: String
    ) async throws -> AgentTarget {
        try await postEmpty(agentTargetPath(
            channelID: channelID,
            conversationID: conversationID,
            targetID: targetID,
            action: "retry"
        ))
    }

    /// `POST .../targets/{target_id}/cancel`.
    public func cancelAgentTarget(
        channelID: String,
        conversationID: String,
        targetID: String
    ) async throws {
        try await requestNoContent(
            path: agentTargetPath(
                channelID: channelID,
                conversationID: conversationID,
                targetID: targetID,
                action: "cancel"
            ),
            method: "POST"
        )
    }

    /// `POST /api/channels` — create one private Channel.
    @discardableResult
    public func createPrimaryChannel(name: String) async throws -> PrimaryChannelDetail {
        try await post("/api/channels", body: PrimaryChannelCreateRequest(name: name))
    }

    /// `PATCH /api/channels/{id}` — change exactly one mutable Channel field.
    public func updatePrimaryChannel(
        id: String,
        name: String? = nil,
        state: PrimaryChannelState? = nil,
        pinned: Bool? = nil
    ) async throws {
        let changes = [name != nil, state != nil, pinned != nil].filter { $0 }.count
        guard changes == 1 else { throw PrimaryChannelRequestError.exactlyOneChangeRequired }
        let body = PrimaryChannelUpdateRequest(name: name, state: state?.rawValue, pinned: pinned)
        try await requestNoContent(
            path: "/api/channels/\(Self.primaryPathComponent(id))",
            method: "PATCH",
            body: try encoder.encode(body)
        )
    }

    /// `POST /api/channels/{id}/turns` — durably submit one idempotent turn.
    @discardableResult
    public func postPrimaryTurn(
        channelID: String,
        clientTurnID: String,
        text: String
    ) async throws -> PrimaryTurnResult {
        let body = PrimaryTurnRequest(clientTurnID: clientTurnID, text: text)
        return try await post(
            "/api/channels/\(Self.primaryPathComponent(channelID))/turns",
            body: body
        )
    }

    /// `POST /api/channels/{id}/targets/{target}/retry`.
    @discardableResult
    public func retryPrimaryTarget(
        channelID: String,
        targetID: String
    ) async throws -> PrimaryTarget {
        try await postEmpty(primaryTargetPath(channelID: channelID, targetID: targetID, action: "retry"))
    }

    /// `POST /api/channels/{id}/targets/{target}/recheck-and-retry`.
    @discardableResult
    public func recheckAndRetryPrimaryTarget(
        channelID: String,
        targetID: String
    ) async throws -> PrimaryTarget {
        try await postEmpty(primaryTargetPath(
            channelID: channelID,
            targetID: targetID,
            action: "recheck-and-retry"
        ))
    }

    /// `POST /api/channels/{id}/targets/{target}/cancel`.
    public func cancelPrimaryTarget(channelID: String, targetID: String) async throws {
        try await requestNoContent(
            path: primaryTargetPath(channelID: channelID, targetID: targetID, action: "cancel"),
            method: "POST"
        )
    }

    /// `GET /api/channels/{id}/events` — replacement snapshots for one Primary
    /// Channel. Every data frame must decode; malformed snapshots terminate the
    /// stream instead of leaving a native client on silently stale state.
    public func primaryChannelEvents(
        channelID: String
    ) -> AsyncThrowingStream<PrimaryChannelDetail, Error> {
        let path = "/api/channels/\(Self.primaryPathComponent(channelID))/events"
        if let relayTransport {
            return relayPrimaryChannelEvents(path: path, transport: relayTransport)
        }

        let session = self.session
        let decoder = self.decoder
        let requestID = FortRequestID.make()
        let requestResult = Result {
            var request = try makeRequest(
                path: path,
                method: "GET",
                rawBody: nil,
                requestID: requestID
            )
            request.setValue("text/event-stream", forHTTPHeaderField: "Accept")
            request.setValue("no-cache", forHTTPHeaderField: "Cache-Control")
            return request
        }

        return AsyncThrowingStream { continuation in
            let task = Task {
                do {
                    let request = try requestResult.get()
                    let (bytes, response) = try await session.bytes(for: request)
                    guard let http = response as? HTTPURLResponse else {
                        throw FortClientError.nonHTTPResponse
                    }
                    guard (200...299).contains(http.statusCode) else {
                        throw FortClientError.httpStatus(
                            status: http.statusCode,
                            body: "",
                            requestID: requestID
                        )
                    }

                    var buffer = Data()
                    for try await byte in bytes {
                        buffer.append(byte)
                        while let frame = Self.takePrimarySSEFrame(from: &buffer) {
                            if let payload = Self.primaryDataPayload(in: frame) {
                                continuation.yield(try Self.decodePrimarySnapshot(payload, using: decoder))
                            }
                        }
                    }
                    if !buffer.isEmpty,
                       let payload = Self.primaryDataPayload(
                        in: String(decoding: buffer, as: UTF8.self)
                            .replacingOccurrences(of: "\r\n", with: "\n")
                       ) {
                        continuation.yield(try Self.decodePrimarySnapshot(payload, using: decoder))
                    }
                    continuation.finish()
                } catch is CancellationError {
                    continuation.finish()
                } catch {
                    continuation.finish(throwing: error)
                }
            }
            continuation.onTermination = { _ in task.cancel() }
        }
    }

    /// `GET .../conversations/{conversation_id}/events` — strict replacement
    /// snapshots scoped by both the owning Agent Channel and Conversation.
    public func agentConversationEvents(
        channelID: String,
        conversationID: String
    ) -> AsyncThrowingStream<AgentConversationDetail, Error> {
        let path = "\(agentConversationPath(channelID: channelID, conversationID: conversationID))/events"
        if let relayTransport {
            return relayAgentConversationEvents(path: path, transport: relayTransport)
        }

        let session = self.session
        let decoder = self.decoder
        let requestID = FortRequestID.make()
        let requestResult = Result {
            var request = try makeRequest(
                path: path,
                method: "GET",
                rawBody: nil,
                requestID: requestID
            )
            request.setValue("text/event-stream", forHTTPHeaderField: "Accept")
            request.setValue("no-cache", forHTTPHeaderField: "Cache-Control")
            return request
        }

        return AsyncThrowingStream { continuation in
            let task = Task {
                do {
                    let request = try requestResult.get()
                    let (bytes, response) = try await session.bytes(for: request)
                    guard let http = response as? HTTPURLResponse else {
                        throw FortClientError.nonHTTPResponse
                    }
                    guard (200...299).contains(http.statusCode) else {
                        throw FortClientError.httpStatus(
                            status: http.statusCode,
                            body: "",
                            requestID: requestID
                        )
                    }

                    var buffer = Data()
                    for try await byte in bytes {
                        buffer.append(byte)
                        while let frame = Self.takePrimarySSEFrame(from: &buffer) {
                            if let payload = Self.primaryDataPayload(in: frame) {
                                continuation.yield(try Self.decodeAgentConversationSnapshot(payload, using: decoder))
                            }
                        }
                    }
                    if !buffer.isEmpty,
                       let payload = Self.primaryDataPayload(
                        in: String(decoding: buffer, as: UTF8.self)
                            .replacingOccurrences(of: "\r\n", with: "\n")
                       ) {
                        continuation.yield(try Self.decodeAgentConversationSnapshot(payload, using: decoder))
                    }
                    continuation.finish()
                } catch is CancellationError {
                    continuation.finish()
                } catch {
                    continuation.finish(throwing: error)
                }
            }
            continuation.onTermination = { _ in task.cancel() }
        }
    }

    // MARK: - HTTP plumbing

    private struct PrimaryAgentSelectionRequest: Encodable {
        let optionID: String
        enum CodingKeys: String, CodingKey { case optionID = "option_id" }
    }

    private struct PrimaryChannelCreateRequest: Encodable {
        let name: String
    }

    private struct PrimaryChannelUpdateRequest: Encodable {
        let name: String?
        let state: String?
        let pinned: Bool?
    }

    private struct AgentChannelCreateRequest: Encodable {
        let agentOptionID: String
        let name: String
        enum CodingKeys: String, CodingKey {
            case agentOptionID = "agent_option_id"
            case name
        }
    }

    private struct AgentChannelUpdateRequest: Encodable {
        let name: String?
        let state: String?
    }

    private struct AgentConversationCreateRequest: Encodable {
        let name: String
    }

    private struct AgentConversationUpdateRequest: Encodable {
        let name: String?
        let state: String?
        let pinned: Bool?
    }

    private struct AgentFirstTurnRequest: Encodable {
        let name: String
        let clientTurnID: String
        let text: String

        enum CodingKeys: String, CodingKey {
            case name
            case clientTurnID = "client_turn_id"
            case text
        }
    }

    private func get<T: Decodable>(_ path: String) async throws -> T {
        let response = try await perform(path: path, method: "GET", body: nil)
        try Self.throwIfNotOK(status: response.status, data: response.data, requestID: response.requestID)
        return try decoder.decode(T.self, from: response.data)
    }

    private func post<Body: Encodable, T: Decodable>(_ path: String, body: Body) async throws -> T {
        let response = try await perform(path: path, method: "POST", body: try encoder.encode(body))
        try Self.throwIfNotOK(status: response.status, data: response.data, requestID: response.requestID)
        return try decoder.decode(T.self, from: response.data)
    }

    private func postEmpty<T: Decodable>(_ path: String) async throws -> T {
        let response = try await perform(path: path, method: "POST", body: nil)
        try Self.throwIfNotOK(status: response.status, data: response.data, requestID: response.requestID)
        return try decoder.decode(T.self, from: response.data)
    }

    private func requestNoContent(path: String, method: String, body: Data? = nil) async throws {
        let response = try await perform(path: path, method: method, body: body)
        try Self.throwIfNotOK(status: response.status, data: response.data, requestID: response.requestID)
    }

    private func put<Body: Encodable, T: Decodable>(_ path: String, body: Body) async throws -> T {
        let response = try await perform(path: path, method: "PUT", body: try encoder.encode(body))
        try Self.throwIfNotOK(status: response.status, data: response.data, requestID: response.requestID)
        return try decoder.decode(T.self, from: response.data)
    }

    private func perform(
        path: String,
        method: String,
        body: Data?
    ) async throws -> (data: Data, status: Int, requestID: String) {
        let requestID = FortRequestID.make()
        if let relayTransport {
            return try await relayTransport.request(
                path: path,
                method: method,
                headers: body == nil ? nil : ["Content-Type": "application/json"],
                body: body,
                requestID: requestID
            )
        }
        let request = try makeRequest(
            path: path,
            method: method,
            rawBody: body,
            requestID: requestID
        )
        let (data, response) = try await session.data(for: request)
        guard let http = response as? HTTPURLResponse else { throw FortClientError.nonHTTPResponse }
        return (data, http.statusCode, requestID)
    }

    private func makeRequest(
        path: String,
        method: String,
        rawBody: Data?,
        requestID: String
    ) throws -> URLRequest {
        // path begins with "/"; strip it so appendingPathComponent joins cleanly
        // regardless of whether baseURL has a trailing slash.
        let relative = path.hasPrefix("/") ? String(path.dropFirst()) : path
        let pieces = relative.split(separator: "?", maxSplits: 1, omittingEmptySubsequences: false)
        var url = baseURL.appendingPathComponent(String(pieces[0]))
        if pieces.count == 2, var components = URLComponents(url: url, resolvingAgainstBaseURL: false) {
            components.percentEncodedQuery = String(pieces[1])
            if let queryURL = components.url { url = queryURL }
        }
        var request = URLRequest(url: url)
        request.httpMethod = method
        request.setValue("application/json", forHTTPHeaderField: "Accept")
        request.setValue(requestID, forHTTPHeaderField: FortRequestID.header)
        if let rawBody {
            request.setValue("application/json", forHTTPHeaderField: "Content-Type")
            request.httpBody = rawBody
        }
        return request
    }

    private static func throwIfNotOK(status: Int, data: Data, requestID: String) throws {
        guard (200...299).contains(status) else {
            let body = String(data: data, encoding: .utf8) ?? ""
            throw FortClientError.httpStatus(status: status, body: body, requestID: requestID)
        }
    }

    private func primaryTargetPath(channelID: String, targetID: String, action: String) -> String {
        "/api/channels/\(Self.primaryPathComponent(channelID))/targets/\(Self.primaryPathComponent(targetID))/\(action)"
    }

    private func agentChannelPath(channelID: String) -> String {
        "/api/agent-channels/\(Self.primaryPathComponent(channelID))"
    }

    private func agentConversationPath(channelID: String, conversationID: String) -> String {
        "\(agentChannelPath(channelID: channelID))/conversations/\(Self.primaryPathComponent(conversationID))"
    }

    private func agentTargetPath(
        channelID: String,
        conversationID: String,
        targetID: String,
        action: String
    ) -> String {
        "\(agentConversationPath(channelID: channelID, conversationID: conversationID))/targets/\(Self.primaryPathComponent(targetID))/\(action)"
    }

    private static func primaryPathComponent(_ value: String) -> String {
        var allowed = CharacterSet.urlPathAllowed
        allowed.remove(charactersIn: "/?#")
        return value.addingPercentEncoding(withAllowedCharacters: allowed) ?? value
    }

    private static func primaryQueryValue(_ value: String) -> String {
        var allowed = CharacterSet.urlQueryAllowed
        allowed.remove(charactersIn: "&=+?")
        return value.addingPercentEncoding(withAllowedCharacters: allowed) ?? value
    }

    private func relayPrimaryChannelEvents(
        path: String,
        transport: any FortRelayTransporting
    ) -> AsyncThrowingStream<PrimaryChannelDetail, Error> {
        let decoder = self.decoder
        return AsyncThrowingStream { continuation in
            let task = Task {
                var buffer = ""
                do {
                    for try await chunk in transport.events(
                        path: path,
                        requestID: FortRequestID.make()
                    ) {
                        buffer += String(decoding: chunk, as: UTF8.self)
                        buffer = buffer.replacingOccurrences(of: "\r\n", with: "\n")
                        while let boundary = buffer.range(of: "\n\n") {
                            let frame = String(buffer[..<boundary.lowerBound])
                            buffer.removeSubrange(..<boundary.upperBound)
                            if let payload = Self.primaryDataPayload(in: frame) {
                                continuation.yield(try Self.decodePrimarySnapshot(payload, using: decoder))
                            }
                        }
                    }
                    if let payload = Self.primaryDataPayload(in: buffer) {
                        continuation.yield(try Self.decodePrimarySnapshot(payload, using: decoder))
                    }
                    continuation.finish()
                } catch is CancellationError {
                    continuation.finish()
                } catch {
                    continuation.finish(throwing: error)
                }
            }
            continuation.onTermination = { _ in task.cancel() }
        }
    }

    private func relayAgentConversationEvents(
        path: String,
        transport: any FortRelayTransporting
    ) -> AsyncThrowingStream<AgentConversationDetail, Error> {
        let decoder = self.decoder
        return AsyncThrowingStream { continuation in
            let task = Task {
                var buffer = ""
                do {
                    for try await chunk in transport.events(
                        path: path,
                        requestID: FortRequestID.make()
                    ) {
                        buffer += String(decoding: chunk, as: UTF8.self)
                        buffer = buffer.replacingOccurrences(of: "\r\n", with: "\n")
                        while let boundary = buffer.range(of: "\n\n") {
                            let frame = String(buffer[..<boundary.lowerBound])
                            buffer.removeSubrange(..<boundary.upperBound)
                            if let payload = Self.primaryDataPayload(in: frame) {
                                continuation.yield(try Self.decodeAgentConversationSnapshot(payload, using: decoder))
                            }
                        }
                    }
                    if let payload = Self.primaryDataPayload(in: buffer) {
                        continuation.yield(try Self.decodeAgentConversationSnapshot(payload, using: decoder))
                    }
                    continuation.finish()
                } catch is CancellationError {
                    continuation.finish()
                } catch {
                    continuation.finish(throwing: error)
                }
            }
            continuation.onTermination = { _ in task.cancel() }
        }
    }

    // MARK: - SSE parsing helpers

    /// Extracts the value of an SSE field line like `field: value`.
    /// Per the SSE spec, exactly one optional leading space after the colon is
    /// stripped. Returns nil if the line is not this field.
    private static func value(ofField field: String, in line: String) -> String? {
        guard line.hasPrefix(field) else { return nil }
        let afterField = line.dropFirst(field.count)
        guard afterField.first == ":" else { return nil }
        var value = afterField.dropFirst() // drop ":"
        if value.first == " " {
            value = value.dropFirst() // drop the single optional leading space
        }
        return String(value)
    }

    private static func primaryDataPayload(in frame: String) -> String? {
        let payload = frame.split(separator: "\n", omittingEmptySubsequences: false).compactMap {
            value(ofField: "data", in: String($0))
        }.joined(separator: "\n")
        return payload.isEmpty ? nil : payload
    }

    private static func takePrimarySSEFrame(from buffer: inout Data) -> String? {
        let lineFeedBoundary = Data([0x0A, 0x0A])
        let carriageReturnBoundary = Data([0x0D, 0x0A, 0x0D, 0x0A])
        let lineFeedRange = buffer.range(of: lineFeedBoundary)
        let carriageReturnRange = buffer.range(of: carriageReturnBoundary)
        let boundary: Range<Data.Index>?
        switch (lineFeedRange, carriageReturnRange) {
        case (let lhs?, let rhs?):
            boundary = lhs.lowerBound < rhs.lowerBound ? lhs : rhs
        case (let lhs?, nil):
            boundary = lhs
        case (nil, let rhs?):
            boundary = rhs
        case (nil, nil):
            boundary = nil
        }
        guard let boundary else { return nil }
        let frame = String(decoding: buffer[..<boundary.lowerBound], as: UTF8.self)
            .replacingOccurrences(of: "\r\n", with: "\n")
        buffer.removeSubrange(..<boundary.upperBound)
        return frame
    }

    private static func decodePrimarySnapshot(
        _ json: String,
        using decoder: JSONDecoder
    ) throws -> PrimaryChannelDetail {
        try decoder.decode(PrimaryChannelDetail.self, from: Data(json.utf8))
    }

    private static func decodeAgentConversationSnapshot(
        _ json: String,
        using decoder: JSONDecoder
    ) throws -> AgentConversationDetail {
        try decoder.decode(AgentConversationDetail.self, from: Data(json.utf8))
    }
}

public enum PrimaryChannelRequestError: Error, Sendable, Equatable {
    case exactlyOneChangeRequired
}

extension PrimaryChannelRequestError: LocalizedError {
    public var errorDescription: String? {
        "Exactly one of name, state, or pinned is required."
    }
}

public enum AgentChannelRequestError: Error, Sendable, Equatable {
    case exactlyOneChangeRequired
}

extension AgentChannelRequestError: LocalizedError {
    public var errorDescription: String? {
        "Exactly one of name or state is required."
    }
}

public enum AgentConversationRequestError: Error, Sendable, Equatable {
    case exactlyOneChangeRequired
}

extension AgentConversationRequestError: LocalizedError {
    public var errorDescription: String? {
        "Exactly one of name, state, or pinned is required."
    }
}
