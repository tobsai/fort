import Foundation

#if canImport(FoundationNetworking)
import FoundationNetworking
#endif

public struct GatewayMachine: Codable, Sendable, Identifiable, Hashable {
    public let machineID: String
    public let name: String
    public let fingerprint: String
    public let online: Bool
    public let publicKey: String

    public var id: String { machineID }

    enum CodingKeys: String, CodingKey {
        case machineID = "machine_id"
        case name
        case fingerprint
        case online
        case publicKey = "public_key"
    }
}

/// Correlation identifiers shared by direct and gateway-backed Fort requests.
/// UUID strings are normalized before use so the Go ingress accepts them as
/// canonical request IDs rather than replacing them at the server boundary.
public enum FortRequestID {
    public static let header = "X-Fort-Request-ID"

    public static func make() -> String {
        UUID().uuidString.lowercased()
    }

    public static func canonical(_ candidate: String? = nil) -> String {
        guard let candidate, let uuid = UUID(uuidString: candidate) else { return make() }
        return uuid.uuidString.lowercased()
    }
}

public enum GatewayRelayError: Error, Sendable {
    case invalidMachineKey
    case invalidGatewayResponse
    case missingFrame
    case wrongFrame(expected: String, actual: String)
    case httpStatus(Int, String)
    case requestFailed(requestID: String, status: Int?, message: String)
    case fingerprintChanged(expected: String, actual: String)

    public var statusCode: Int? {
        switch self {
        case .httpStatus(let status, _):
            return status
        case .requestFailed(_, let status, _):
            return status
        default:
            return nil
        }
    }

    fileprivate static func correlated(_ error: Error, requestID: String) -> GatewayRelayError {
        if let gatewayError = error as? GatewayRelayError {
            if case let .requestFailed(_, status, message) = gatewayError {
                return .requestFailed(requestID: requestID, status: status, message: message)
            }
            let message: String
            switch gatewayError {
            case .httpStatus(let status, _):
                if status == 401 {
                    message = "Your gateway session expired. Sign in again."
                } else if status == 403 {
                    message = "This account is not allowed to use the Fort gateway."
                } else if status == 502 || status == 503 || status == 504 {
                    message = "The selected Fort is temporarily unavailable (gateway \(status)). Try again."
                } else {
                    message = "The Fort gateway returned HTTP \(status)."
                }
            default:
                message = gatewayError.localizedDescription
            }
            return .requestFailed(requestID: requestID, status: gatewayError.statusCode, message: message)
        }
        if let urlError = error as? URLError {
            return .requestFailed(
                requestID: requestID,
                status: nil,
                message: "The Fort gateway request failed (network error \(urlError.code.rawValue))."
            )
        }
        return .requestFailed(
            requestID: requestID,
            status: nil,
            message: "The Fort gateway request failed."
        )
    }
}

extension GatewayRelayError: LocalizedError {
    public var errorDescription: String? {
        switch self {
        case .invalidMachineKey:
            return "The selected Fort has an invalid relay identity. Reconnect it from the gateway."
        case .invalidGatewayResponse:
            return "The gateway returned an invalid response. Try again."
        case .missingFrame:
            return "The encrypted relay returned an incomplete response. Try again."
        case .wrongFrame(let expected, let actual):
            return "The encrypted relay returned \(actual) instead of \(expected). Reconnect the selected Fort."
        case .httpStatus(let status, let body):
            if status == 401 {
                return "Your gateway session expired. Sign in again."
            }
            if status == 403 {
                return "This account is not allowed to use the Fort gateway."
            }
            let detail = gatewayErrorDetail(body)
            let suffix = detail.isEmpty ? "" : " \(detail)"
            if status == 502 || status == 503 || status == 504 {
                return "The selected Fort is temporarily unavailable (gateway \(status)).\(suffix) Try again."
            }
            return "The Fort gateway returned HTTP \(status).\(suffix)"
        case .requestFailed(let requestID, _, let message):
            return "\(message) Request ID: \(requestID)."
        case .fingerprintChanged:
            return "The selected Fort's encrypted identity changed. Verify its fingerprint before reconnecting."
        }
    }
}

private func gatewayErrorDetail(_ body: String) -> String {
    let trimmed = body.trimmingCharacters(in: .whitespacesAndNewlines)
    guard !trimmed.isEmpty else { return "" }
    if let data = trimmed.data(using: .utf8),
       let object = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
       let message = object["error"] as? String {
        return String(message.prefix(300))
    }
    return String(trimmed.prefix(300))
}

/// Retries only the pre-command Noise handshake. Callers must create a fresh
/// tunnel inside `operation`; encrypted application requests are never replayed.
public enum GatewayRelayRetry {
    public static func handshake<T>(_ operation: () async throws -> T) async throws -> T {
        do {
            return try await operation()
        } catch {
            guard isTransientHandshakeFailure(error) else { throw error }
            try await Task.sleep(nanoseconds: 150_000_000)
            return try await operation()
        }
    }

    private static func isTransientHandshakeFailure(_ error: Error) -> Bool {
        if case GatewayRelayError.httpStatus(let status, _) = error {
            return status == 502 || status == 503 || status == 504
        }
        guard let urlError = error as? URLError else { return false }
        return [
            .timedOut,
            .cannotFindHost,
            .cannotConnectToHost,
            .networkConnectionLost,
            .notConnectedToInternet,
        ].contains(urlError.code)
    }
}

private struct GatewayMachinesResponse: Decodable { let machines: [GatewayMachine] }

public enum GatewayService {
    public static func machines(
        at gatewayURL: URL,
        bearerToken: String,
        session: URLSession = .shared
    ) async throws -> [GatewayMachine] {
        let requestID = FortRequestID.make()
        var request = URLRequest(url: gatewayURL.appendingPathComponent("api/machines"))
        request.setValue("Bearer \(bearerToken)", forHTTPHeaderField: "Authorization")
        request.setValue("application/json", forHTTPHeaderField: "Accept")
        request.setValue(requestID, forHTTPHeaderField: FortRequestID.header)
        do {
            let (data, response) = try await session.data(for: request)
            guard let http = response as? HTTPURLResponse else { throw GatewayRelayError.invalidGatewayResponse }
            guard (200...299).contains(http.statusCode) else {
                throw GatewayRelayError.httpStatus(http.statusCode, String(data: data, encoding: .utf8) ?? "")
            }
            return try JSONDecoder().decode(GatewayMachinesResponse.self, from: data).machines
        } catch is CancellationError {
            throw CancellationError()
        } catch {
            throw GatewayRelayError.correlated(error, requestID: requestID)
        }
    }
}

private struct GatewayFrame: Codable, Sendable {
    let stream: String
    let kind: String
    let b64: String?

    init(stream: String, kind: String, data: Data? = nil) {
        self.stream = stream
        self.kind = kind
        self.b64 = data?.base64EncodedString()
    }
}

private struct GatewayFrameRequest: Encodable {
    let machineID: String
    let frame: GatewayFrame
    enum CodingKeys: String, CodingKey { case machineID = "machine_id"; case frame }
}

private struct GatewayFramesResponse: Decodable { let frames: [GatewayFrame] }

private struct RelayRequestPayload: Encodable {
    let id: String
    let method: String
    let path: String
    let headers: [String: String]?
    let body: Data?
}

private struct RelayResponsePayload: Decodable {
    let id: String
    let status: Int
    let headers: [String: String]?
    let body: Data?
    let stream: Bool?
}

private struct RelayChunkPayload: Decodable {
    let id: String
    let data: Data?
    let end: Bool?
}

public final class GatewayRelayTransport: @unchecked Sendable {
    public let gatewayURL: URL
    public let machineID: String
    public let machinePublicKey: Data

    private let bearerToken: String
    private let session: URLSession

    public init(
        gatewayURL: URL,
        bearerToken: String,
        machineID: String,
        machinePublicKey: Data,
        session: URLSession? = nil
    ) throws {
        guard machinePublicKey.count == 32 else { throw GatewayRelayError.invalidMachineKey }
        self.gatewayURL = gatewayURL
        self.bearerToken = bearerToken
        self.machineID = machineID
        self.machinePublicKey = machinePublicKey
        if let session {
            self.session = session
        } else {
            let configuration = URLSessionConfiguration.default
            configuration.requestCachePolicy = .reloadIgnoringLocalCacheData
            configuration.urlCache = nil
            configuration.timeoutIntervalForRequest = 60
            self.session = URLSession(configuration: configuration)
        }
    }

    public func request(
        path: String,
        method: String = "GET",
        headers: [String: String]? = nil,
        body: Data? = nil,
        requestID: String = FortRequestID.make()
    ) async throws -> (data: Data, status: Int, requestID: String) {
        let requestID = FortRequestID.canonical(requestID)
        var tunnel: RelayTunnel?
        do {
            let connected = try await connectedTunnel(requestID: requestID)
            tunnel = connected
            let response = try await connected.fetch(path: path, method: method, headers: headers, body: body)
            await connected.close()
            return (response.body ?? Data(), response.status, requestID)
        } catch is CancellationError {
            if let tunnel { await tunnel.close() }
            throw CancellationError()
        } catch {
            if let tunnel { await tunnel.close() }
            throw GatewayRelayError.correlated(error, requestID: requestID)
        }
    }

    public func events(
        path: String,
        requestID: String = FortRequestID.make()
    ) -> AsyncThrowingStream<Data, Error> {
        let requestID = FortRequestID.canonical(requestID)
        return AsyncThrowingStream { continuation in
            let task = Task {
                var tunnel: RelayTunnel?
                do {
                    let connected = try await connectedTunnel(requestID: requestID)
                    tunnel = connected
                    try await connected.stream(path: path) { continuation.yield($0) }
                    await connected.close()
                    continuation.finish()
                } catch is CancellationError {
                    if let tunnel { await tunnel.close() }
                    continuation.finish()
                } catch {
                    if let tunnel { await tunnel.close() }
                    continuation.finish(
                        throwing: GatewayRelayError.correlated(error, requestID: requestID)
                    )
                }
            }
            continuation.onTermination = { _ in task.cancel() }
        }
    }

    private func connectedTunnel(requestID: String) async throws -> RelayTunnel {
        try await GatewayRelayRetry.handshake {
            let tunnel = RelayTunnel(
                gatewayURL: gatewayURL,
                bearerToken: bearerToken,
                machineID: machineID,
                machinePublicKey: machinePublicKey,
                urlSession: session,
                requestID: requestID
            )
            do {
                try await tunnel.connect()
                return tunnel
            } catch {
                await tunnel.close()
                throw error
            }
        }
    }
}

private final class RelayTunnel: @unchecked Sendable {
    private let gatewayURL: URL
    private let bearerToken: String
    private let machineID: String
    private let machinePublicKey: Data
    private let urlSession: URLSession
    private let requestID: String
    private let streamID = UUID().uuidString.replacingOccurrences(of: "-", with: "").lowercased()
    private var noise: RelayNoiseInitiator?
    private var transport: RelayNoiseSession?
    private var started = false

    init(
        gatewayURL: URL,
        bearerToken: String,
        machineID: String,
        machinePublicKey: Data,
        urlSession: URLSession,
        requestID: String
    ) {
        self.gatewayURL = gatewayURL
        self.bearerToken = bearerToken
        self.machineID = machineID
        self.machinePublicKey = machinePublicKey
        self.urlSession = urlSession
        self.requestID = requestID
    }

    func connect() async throws {
        let handshake = try RelayNoiseInitiator(responderPublicKey: machinePublicKey)
        noise = handshake
        started = true
        let replies = try await post(frame: GatewayFrame(stream: streamID, kind: "hs1", data: try handshake.writeMessage()))
        guard let reply = replies.first else { throw GatewayRelayError.missingFrame }
        guard reply.kind == "hs2" else { throw GatewayRelayError.wrongFrame(expected: "hs2", actual: reply.kind) }
        guard let encoded = reply.b64, let message = Data(base64Encoded: encoded) else { throw GatewayRelayError.missingFrame }
        _ = try handshake.readMessage(message)
        transport = try handshake.session()
    }

    func fetch(path: String, method: String, headers: [String: String]?, body: Data?) async throws -> RelayResponsePayload {
        guard let transport else { throw RelaySecurityError.handshakeIncomplete }
        var requestHeaders = headers ?? [:]
        requestHeaders[FortRequestID.header] = requestID
        let payload = RelayRequestPayload(
            id: randomID(), method: method, path: path, headers: requestHeaders, body: body
        )
        let plaintext = try JSONEncoder().encode(payload)
        let replies = try await post(frame: GatewayFrame(stream: streamID, kind: "req", data: try transport.seal(plaintext)))
        guard let reply = replies.first else { throw GatewayRelayError.missingFrame }
        return try openResponse(reply, transport: transport)
    }

    func stream(path: String, onChunk: (Data) -> Void) async throws {
        guard let transport else { throw RelaySecurityError.handshakeIncomplete }
        let payload = RelayRequestPayload(
            id: randomID(), method: "GET", path: path,
            headers: [
                "Accept": "text/event-stream",
                FortRequestID.header: requestID,
            ],
            body: nil
        )
        let frame = GatewayFrame(stream: streamID, kind: "req", data: try transport.seal(JSONEncoder().encode(payload)))
        var request = try gatewayRequest(path: "api/sse", frame: frame)
        request.setValue("application/x-ndjson", forHTTPHeaderField: "Accept")
        let (bytes, response) = try await urlSession.bytes(for: request)
        guard let http = response as? HTTPURLResponse else { throw GatewayRelayError.invalidGatewayResponse }
        guard (200...299).contains(http.statusCode) else { throw GatewayRelayError.httpStatus(http.statusCode, "") }
        for try await line in bytes.lines where !line.isEmpty {
            let gatewayFrame = try JSONDecoder().decode(GatewayFrame.self, from: Data(line.utf8))
            guard let encoded = gatewayFrame.b64, let ciphertext = Data(base64Encoded: encoded) else {
                throw GatewayRelayError.missingFrame
            }
            if gatewayFrame.kind == "res" {
                _ = try openResponse(gatewayFrame, transport: transport)
            } else if gatewayFrame.kind == "chunk" {
                let chunk = try JSONDecoder().decode(RelayChunkPayload.self, from: transport.open(ciphertext))
                if let data = chunk.data { onChunk(data) }
                if chunk.end == true { return }
            }
        }
    }

    func close() async {
        guard started else { return }
        started = false
        _ = try? await post(frame: GatewayFrame(stream: streamID, kind: "bye"))
        transport = nil
        noise = nil
    }

    private func openResponse(_ frame: GatewayFrame, transport: RelayNoiseSession) throws -> RelayResponsePayload {
        guard frame.kind == "res" else { throw GatewayRelayError.wrongFrame(expected: "res", actual: frame.kind) }
        guard let encoded = frame.b64, let ciphertext = Data(base64Encoded: encoded) else { throw GatewayRelayError.missingFrame }
        return try JSONDecoder().decode(RelayResponsePayload.self, from: transport.open(ciphertext))
    }

    private func post(frame: GatewayFrame) async throws -> [GatewayFrame] {
        let request = try gatewayRequest(path: "api/req", frame: frame)
        let (data, response) = try await urlSession.data(for: request)
        guard let http = response as? HTTPURLResponse else { throw GatewayRelayError.invalidGatewayResponse }
        guard (200...299).contains(http.statusCode) else {
            throw GatewayRelayError.httpStatus(http.statusCode, String(data: data, encoding: .utf8) ?? "")
        }
        return try JSONDecoder().decode(GatewayFramesResponse.self, from: data).frames
    }

    private func gatewayRequest(path: String, frame: GatewayFrame) throws -> URLRequest {
        var request = URLRequest(url: gatewayURL.appendingPathComponent(path))
        request.httpMethod = "POST"
        request.setValue("Bearer \(bearerToken)", forHTTPHeaderField: "Authorization")
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.setValue(requestID, forHTTPHeaderField: FortRequestID.header)
        request.httpBody = try JSONEncoder().encode(GatewayFrameRequest(machineID: machineID, frame: frame))
        return request
    }

    private func randomID() -> String { UUID().uuidString.replacingOccurrences(of: "-", with: "").lowercased() }
}
