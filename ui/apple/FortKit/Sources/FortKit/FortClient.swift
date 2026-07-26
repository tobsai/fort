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

/// Errors surfaced by ``FortClient`` beyond the transport errors thrown by
/// `URLSession` itself.
public enum FortClientError: Error, Sendable {
    /// The server returned a non-2xx status for a request where that is fatal.
    /// `status` is the HTTP status code; `body` is the (possibly empty) response body.
    case httpStatus(status: Int, body: String, requestID: String? = nil)
    /// The response was not an `HTTPURLResponse` (should not happen over HTTP(S)).
    case nonHTTPResponse
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

/// The control-plane client. One instance per base URL; safe to share and
/// mutate `baseURL` (guarded internally). Conforms to `ObservableObject` so
/// SwiftUI surfaces can hold it as `@StateObject` / `@EnvironmentObject`.
public final class FortClient: ObservableObject, @unchecked Sendable {

    /// The control-plane base URL. Default is the local control endpoint.
    /// Publishes changes so views bound to it refresh.
    @Published public var baseURL: URL

    private let session: URLSession
    private let decoder: JSONDecoder
    private let encoder: JSONEncoder
    private var relayTransport: GatewayRelayTransport?

    /// Creates a client pointed at `baseURL` (default `http://127.0.0.1:4087`).
    /// `session` is injectable for deterministic contract checks; apps use the
    /// cache-free streaming configuration created here.
    public init(
        baseURL: URL = URL(string: "http://127.0.0.1:4087")!,
        session: URLSession? = nil
    ) {
        self.baseURL = baseURL

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
        relayTransport = try GatewayRelayTransport(
            gatewayURL: gatewayURL,
            bearerToken: bearerToken,
            machineID: machine.machineID,
            machinePublicKey: publicKey
        )
        baseURL = gatewayURL
    }

    public func useDirectHost(_ url: URL) {
        relayTransport = nil
        baseURL = url
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

    // MARK: - Commands

    /// `POST /api/chat` — submit a chat turn. Returns the resulting route.
    @discardableResult
    public func chat(
        _ text: String,
        agent: String? = nil,
        machine: String? = nil,
        playbookID: String? = nil,
        playbookRevision: Int? = nil,
        taskType: String? = nil,
        planGate: Bool? = nil
    ) async throws -> ChatResult {
        try await chat(ChatRequest(
            text: text,
            agent: agent,
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

    // MARK: - HTTP plumbing

    /// Empty stand-in for no-body command endpoints.
    private struct NoBody: Encodable {}

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

    private func relayEvents(since: Int, transport: GatewayRelayTransport) -> AsyncThrowingStream<Event, Error> {
        let decoder = self.decoder
        return AsyncThrowingStream { continuation in
            let task = Task {
                var buffer = ""
                do {
                    for try await chunk in transport.events(path: "/api/events?since=\(since)") {
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
}
