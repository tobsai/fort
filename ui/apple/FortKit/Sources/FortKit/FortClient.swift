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
    case httpStatus(status: Int, body: String)
    /// The response was not an `HTTPURLResponse` (should not happen over HTTP(S)).
    case nonHTTPResponse
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

    /// Creates a client pointed at `baseURL` (default `http://127.0.0.1:4087`).
    public init(baseURL: URL = URL(string: "http://127.0.0.1:4087")!) {
        self.baseURL = baseURL

        // SSE requires that responses are never coalesced or buffered by a
        // cache, and that the connection stays open indefinitely.
        let config = URLSessionConfiguration.default
        config.requestCachePolicy = .reloadIgnoringLocalCacheData
        config.urlCache = nil
        config.timeoutIntervalForRequest = 0 // no per-request timeout: SSE is long-lived
        self.session = URLSession(configuration: config)

        // Models carry explicit snake_case CodingKeys, so we do NOT apply a
        // key-decoding strategy (that would double-convert and fail).
        self.decoder = JSONDecoder()
        self.encoder = JSONEncoder()
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

    // MARK: - Commands

    /// `POST /api/chat` — submit a chat turn. Returns the resulting route.
    @discardableResult
    public func chat(_ text: String, agent: String? = nil) async throws -> ChatResult {
        let body = ChatRequest(text: text, agent: agent)
        return try await post("/api/chat", body: body)
    }

    /// `POST /api/openclaw` — deliver an inbound OpenClaw message.
    @discardableResult
    public func openclaw(from: String, text: String) async throws -> ChatResult {
        let body = OpenClawMessage(from: from, text: text)
        return try await post("/api/openclaw", body: body)
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
        edit: String? = nil
    ) async throws -> Bool {
        let body = GateDecision(runID: run, nodeID: node, decision: decision, edit: edit)
        let request = try makeRequest(path: "/api/gate", method: "POST", jsonBody: body)
        let (data, response) = try await session.data(for: request)
        guard let http = response as? HTTPURLResponse else {
            throw FortClientError.nonHTTPResponse
        }
        if http.statusCode == 409 {
            return false // no execution plane; caller shows "no execution plane"
        }
        try Self.throwIfNotOK(http, data: data)
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
        // Snapshot the base URL up front so the stream is stable if it changes.
        let base = baseURL
        let session = self.session
        let decoder = self.decoder

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

                    let (bytes, response) = try await session.bytes(for: request)
                    if let http = response as? HTTPURLResponse,
                       !(200...299).contains(http.statusCode) {
                        continuation.finish(
                            throwing: FortClientError.httpStatus(status: http.statusCode, body: "")
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

    /// Empty stand-in for "no request body" (avoids referencing `Never`, whose
    /// `Encodable` conformance is gated to newer OSes).
    private struct NoBody: Encodable {}

    private func get<T: Decodable>(_ path: String) async throws -> T {
        let request = try makeRequest(path: path, method: "GET", jsonBody: Optional<NoBody>.none)
        let (data, response) = try await session.data(for: request)
        guard let http = response as? HTTPURLResponse else {
            throw FortClientError.nonHTTPResponse
        }
        try Self.throwIfNotOK(http, data: data)
        return try decoder.decode(T.self, from: data)
    }

    private func post<Body: Encodable, T: Decodable>(_ path: String, body: Body) async throws -> T {
        let request = try makeRequest(path: path, method: "POST", jsonBody: body)
        let (data, response) = try await session.data(for: request)
        guard let http = response as? HTTPURLResponse else {
            throw FortClientError.nonHTTPResponse
        }
        try Self.throwIfNotOK(http, data: data)
        return try decoder.decode(T.self, from: data)
    }

    /// Builds a request. Pass `Optional<Never>.none` as `jsonBody` for no body.
    private func makeRequest<Body: Encodable>(
        path: String,
        method: String,
        jsonBody: Body?
    ) throws -> URLRequest {
        // path begins with "/"; strip it so appendingPathComponent joins cleanly
        // regardless of whether baseURL has a trailing slash.
        let relative = path.hasPrefix("/") ? String(path.dropFirst()) : path
        let url = baseURL.appendingPathComponent(relative)
        var request = URLRequest(url: url)
        request.httpMethod = method
        request.setValue("application/json", forHTTPHeaderField: "Accept")
        if let jsonBody {
            request.setValue("application/json", forHTTPHeaderField: "Content-Type")
            request.httpBody = try encoder.encode(jsonBody)
        }
        return request
    }

    private static func throwIfNotOK(_ http: HTTPURLResponse, data: Data) throws {
        guard (200...299).contains(http.statusCode) else {
            let body = String(data: data, encoding: .utf8) ?? ""
            throw FortClientError.httpStatus(status: http.statusCode, body: body)
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
