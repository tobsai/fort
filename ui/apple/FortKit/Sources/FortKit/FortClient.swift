//
//  FortClient.swift
//  FortKit
//
//  The single HTTP/SSE client every Apple surface (iOS app, macOS app,
//  watch complication, CarPlay) uses to talk to Fort's control plane.
//
//  Reads and commands use URLSession's async APIs; the live feed uses
//  URLSession.bytes(for:) to stream and parse Server-Sent Events into `Event`s.
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

/// The control-plane client. One instance per base URL; safe to share and
/// mutate `baseURL` (guarded internally). Conforms to `ObservableObject` so
/// SwiftUI surfaces can hold it as `@StateObject` / `@EnvironmentObject`.
public final class FortClient: ObservableObject, @unchecked Sendable {

    /// An inert URL used by gateway-only clients before an authenticated relay
    /// is selected. Its custom scheme fails closed if a caller accidentally
    /// attempts a request while disconnected.
    private static let gatewayRequiredBaseURL = URL(string: "fort-gateway-required://disconnected")!

    /// The control-plane base URL. Default is the local control endpoint.
    /// Publishes changes so views bound to it refresh.
    @Published public var baseURL: URL

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

    // MARK: - Reads

    /// `GET /api/summary` — the glanceable control-plane snapshot.
    public func summary() async throws -> Summary {
        try await get("/api/summary")
    }

    /// `GET /api/board` — runs plus waiting gates.
    public func board() async throws -> Board {
        try await get("/api/board")
    }

    /// `GET /api/gates` — the gate inbox.
    public func gates() async throws -> [GateItem] {
        try await get("/api/gates")
    }

    /// `GET /api/runs/{id}` — a replayable run detail.
    public func runDetail(_ id: String) async throws -> RunDetail {
        let escaped = id.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? id
        return try await get("/api/runs/\(escaped)")
    }

    /// `GET /api/backlog` — the pending-task backlog (spec 025).
    public func backlog() async throws -> [BacklogItem] {
        try await get("/api/backlog")
    }

    /// `GET /api/machines` — the machine roster + reachability (spec 022).
    /// Empty in single-machine mode; the server always emits `[]`, never null.
    public func machines() async throws -> [MachineSummary] {
        try await get("/api/machines")
    }

    /// `GET /api/profiles` — closed Fort-owned profile choices with current
    /// readiness and the machines that can execute each exact profile.
    public func profiles() async throws -> [ProfileOption] {
        try await get("/api/profiles")
    }

    /// `GET /api/metrics` — human-decision scorecards for the crew.
    public func metrics(days: Int = 30, lane: String? = nil) async throws -> MetricsResponse {
        var path = "/api/metrics?days=\(days)"
        if let lane, !lane.isEmpty {
            let escaped = lane.addingPercentEncoding(withAllowedCharacters: .urlQueryAllowed) ?? lane
            path += "&lane=\(escaped)"
        }
        return try await get(path)
    }

    /// `GET /api/playbooks` — latest immutable revision of every playbook.
    public func playbooks() async throws -> [Playbook] {
        try await get("/api/playbooks")
    }

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

    /// `POST /api/chat` — submit a chat turn. Returns the resulting route.
    @discardableResult
    public func chat(
        _ text: String,
        agent: String? = nil,
        profile: String? = nil,
        machine: String? = nil,
        playbookID: String? = nil,
        playbookRevision: Int? = nil,
        taskType: String? = nil,
        planGate: Bool? = nil
    ) async throws -> ChatResult {
        try await chat(ChatRequest(
            text: text,
            agent: agent,
            profile: profile,
            machine: machine,
            playbookID: playbookID,
            playbookRevision: playbookRevision,
            taskType: taskType,
            planGate: planGate
        ))
    }

    /// `POST /api/chat` — submit a fully resolved playbook handoff.
    @discardableResult
    public func chat(_ request: ChatRequest) async throws -> ChatResult {
        try await post("/api/chat", body: request)
    }

    /// `POST /api/route` — resolve a route without dispatching or persisting.
    public func route(_ request: RouteRequest) async throws -> RoutePreview {
        try await post("/api/route", body: request)
    }

    /// `PUT /api/playbooks` — append a new immutable revision.
    @discardableResult
    public func savePlaybook(_ playbook: Playbook) async throws -> Playbook {
        try await put("/api/playbooks", body: playbook)
    }

    /// `POST /api/playbooks/{id}/duplicate` — create an editable copy.
    @discardableResult
    public func duplicatePlaybook(_ id: String) async throws -> Playbook {
        let escaped = id.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? id
        return try await post("/api/playbooks/\(escaped)/duplicate", body: Optional<NoBody>.none)
    }

    /// `POST /api/openclaw` — deliver an inbound OpenClaw message.
    @discardableResult
    public func openclaw(from: String, text: String) async throws -> ChatResult {
        let body = OpenClawMessage(from: from, text: text)
        return try await post("/api/openclaw", body: body)
    }

    /// `POST /api/backlog/{id}/dispatch` — promote a backlog item to a run
    /// (spec 025). The body is empty; the item is identified by path.
    @discardableResult
    public func dispatchBacklog(_ id: String) async throws -> ChatResult {
        let escaped = id.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? id
        return try await post("/api/backlog/\(escaped)/dispatch", body: Optional<NoBody>.none)
    }

    /// `POST /api/backlog` — add a task to Ready (spec 025). `title` is the
    /// first line of the compose field; `body` the rest. Returns the created item.
    @discardableResult
    public func addBacklog(
        title: String,
        body: String? = nil,
        agent: String? = nil,
        machine: String? = nil
    ) async throws -> BacklogItem {
        let request = BacklogRequest(title: title, body: body, agent: agent, machine: machine)
        return try await post("/api/backlog", body: request)
    }

    /// `PATCH /api/backlog/{id}` — pin or reassign Up-next work.
    @discardableResult
    public func reassignBacklog(_ id: String, agent: String) async throws -> BacklogItem {
        let escaped = id.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? id
        return try await patch("/api/backlog/\(escaped)", body: BacklogPatch(agent: agent))
    }

    /// `POST /api/breakdown` — ask the planner to decompose a goal into backlog
    /// sub-tasks (spec 026). Returns the visible planner run's id.
    @discardableResult
    public func breakdown(_ text: String, agent: String? = nil) async throws -> BreakdownResult {
        let body = BreakdownRequest(text: text, agent: agent)
        return try await post("/api/breakdown", body: body)
    }

    /// `POST /api/gate` — decide a gate.
    ///
    /// Returns `false` when the server replies HTTP 409 (no execution plane —
    /// control-only mode), and `true` on success. Non-409 error statuses throw.
    @discardableResult
    public func decideGate(
        run: String,
        node: String,
        decision: String,
        edit: String? = nil,
        note: String? = nil
    ) async throws -> Bool {
        let body = GateDecision(runID: run, nodeID: node, decision: decision, edit: edit, note: note)
        let data = try encoder.encode(body)
        let response = try await perform(path: "/api/gate", method: "POST", body: data)
        if response.status == 409 {
            return false // no execution plane; caller shows "no execution plane"
        }
        try Self.throwIfNotOK(
            status: response.status,
            data: response.data,
            requestID: response.requestID
        )
        return true
    }

    // MARK: - Live feed

    /// `GET /api/events?since=N` — the SSE live feed, as an async sequence of
    /// ``Event``s parsed from the stream. Each server frame looks like:
    ///
    /// ```
    /// id: <n>
    /// event: <type>
    /// data: <Event json>
    ///
    /// ```
    ///
    /// The stream ends when the server closes the connection or the task is
    /// cancelled; transport/parse failures finish the stream with the error.
    public func events(since: Int = 0) -> AsyncThrowingStream<Event, Error> {
        if let relayTransport {
            return relayEvents(since: since, transport: relayTransport)
        }
        // Snapshot the base URL up front so the stream is stable if it changes.
        let base = baseURL
        let session = self.session
        let decoder = self.decoder
        let requestID = FortRequestID.make()

        return AsyncThrowingStream { continuation in
            let task = Task {
                do {
                    var components = URLComponents(
                        url: base.appendingPathComponent("api/events"),
                        resolvingAgainstBaseURL: false
                    )
                    components?.queryItems = [URLQueryItem(name: "since", value: String(since))]
                    guard let url = components?.url else {
                        continuation.finish(throwing: URLError(.badURL))
                        return
                    }

                    var request = URLRequest(url: url)
                    request.setValue("text/event-stream", forHTTPHeaderField: "Accept")
                    request.setValue("no-cache", forHTTPHeaderField: "Cache-Control")
                    request.setValue(requestID, forHTTPHeaderField: FortRequestID.header)

                    let (bytes, response) = try await session.bytes(for: request)
                    if let http = response as? HTTPURLResponse,
                       !(200...299).contains(http.statusCode) {
                        continuation.finish(
                            throwing: FortClientError.httpStatus(
                                status: http.statusCode,
                                body: "",
                                requestID: requestID
                            )
                        )
                        return
                    }

                    // Accumulate the `data:` payload of the current frame. A
                    // blank line terminates a frame; other fields (id/event) are
                    // carried by the Event JSON itself, so we ignore them.
                    var dataBuffer = ""

                    for try await line in bytes.lines {
                        if line.isEmpty {
                            // Frame boundary — emit if we collected a data payload.
                            if !dataBuffer.isEmpty {
                                if let event = Self.decodeEvent(dataBuffer, using: decoder) {
                                    continuation.yield(event)
                                }
                                dataBuffer = ""
                            }
                            continue
                        }

                        if line.hasPrefix(":") {
                            continue // SSE comment / heartbeat
                        }

                        if let payload = Self.value(ofField: "data", in: line) {
                            // Multiple data: lines in one frame are newline-joined per spec.
                            dataBuffer += dataBuffer.isEmpty ? payload : "\n" + payload
                        }
                        // id: and event: fields are informational here; the Event
                        // JSON already carries id/type, so we don't need them.
                    }

                    // Stream closed by the server — flush any trailing frame.
                    if !dataBuffer.isEmpty,
                       let event = Self.decodeEvent(dataBuffer, using: decoder) {
                        continuation.yield(event)
                    }
                    continuation.finish()
                } catch is CancellationError {
                    continuation.finish()
                } catch {
                    continuation.finish(throwing: error)
                }
            }

            continuation.onTermination = { _ in
                task.cancel()
            }
        }
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

    // MARK: - HTTP plumbing

    /// Empty stand-in for no-body command endpoints.
    private struct NoBody: Encodable {}

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

    private func patch<Body: Encodable, T: Decodable>(_ path: String, body: Body) async throws -> T {
        let response = try await perform(path: path, method: "PATCH", body: try encoder.encode(body))
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

    private func relayEvents(
        since: Int,
        transport: any FortRelayTransporting
    ) -> AsyncThrowingStream<Event, Error> {
        let decoder = self.decoder
        return AsyncThrowingStream { continuation in
            let task = Task {
                var buffer = ""
                do {
                    for try await chunk in transport.events(
                        path: "/api/events?since=\(since)",
                        requestID: FortRequestID.make()
                    ) {
                        buffer += String(decoding: chunk, as: UTF8.self).replacingOccurrences(of: "\r\n", with: "\n")
                        while let boundary = buffer.range(of: "\n\n") {
                            let frame = String(buffer[..<boundary.lowerBound])
                            buffer.removeSubrange(..<boundary.upperBound)
                            let payload = frame.split(separator: "\n").compactMap {
                                Self.value(ofField: "data", in: String($0))
                            }.joined(separator: "\n")
                            if !payload.isEmpty, let event = Self.decodeEvent(payload, using: decoder) {
                                continuation.yield(event)
                            }
                        }
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

    private static func decodeEvent(_ json: String, using decoder: JSONDecoder) -> Event? {
        guard let data = json.data(using: .utf8) else { return nil }
        return try? decoder.decode(Event.self, from: data)
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
}

public enum PrimaryChannelRequestError: Error, Sendable, Equatable {
    case exactlyOneChangeRequired
}

extension PrimaryChannelRequestError: LocalizedError {
    public var errorDescription: String? {
        "Exactly one of name, state, or pinned is required."
    }
}
