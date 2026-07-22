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

public enum GatewayRelayError: Error, Sendable {
    case invalidMachineKey
    case invalidGatewayResponse
    case missingFrame
    case wrongFrame(expected: String, actual: String)
    case httpStatus(Int, String)
    case fingerprintChanged(expected: String, actual: String)
}

private struct GatewayMachinesResponse: Decodable { let machines: [GatewayMachine] }

public enum GatewayService {
    public static func machines(
        at gatewayURL: URL,
        bearerToken: String,
        session: URLSession = .shared
    ) async throws -> [GatewayMachine] {
        var request = URLRequest(url: gatewayURL.appendingPathComponent("api/machines"))
        request.setValue("Bearer \(bearerToken)", forHTTPHeaderField: "Authorization")
        request.setValue("application/json", forHTTPHeaderField: "Accept")
        let (data, response) = try await session.data(for: request)
        guard let http = response as? HTTPURLResponse else { throw GatewayRelayError.invalidGatewayResponse }
        guard (200...299).contains(http.statusCode) else {
            throw GatewayRelayError.httpStatus(http.statusCode, String(data: data, encoding: .utf8) ?? "")
        }
        return try JSONDecoder().decode(GatewayMachinesResponse.self, from: data).machines
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
        body: Data? = nil
    ) async throws -> (data: Data, status: Int) {
        let tunnel = RelayTunnel(
            gatewayURL: gatewayURL,
            bearerToken: bearerToken,
            machineID: machineID,
            machinePublicKey: machinePublicKey,
            urlSession: session
        )
        do {
            try await tunnel.connect()
            let response = try await tunnel.fetch(path: path, method: method, headers: headers, body: body)
            await tunnel.close()
            return (response.body ?? Data(), response.status)
        } catch {
            await tunnel.close()
            throw error
        }
    }

    public func events(path: String) -> AsyncThrowingStream<Data, Error> {
        AsyncThrowingStream { continuation in
            let task = Task {
                let tunnel = RelayTunnel(
                    gatewayURL: gatewayURL,
                    bearerToken: bearerToken,
                    machineID: machineID,
                    machinePublicKey: machinePublicKey,
                    urlSession: session
                )
                do {
                    try await tunnel.connect()
                    try await tunnel.stream(path: path) { continuation.yield($0) }
                    await tunnel.close()
                    continuation.finish()
                } catch is CancellationError {
                    await tunnel.close()
                    continuation.finish()
                } catch {
                    await tunnel.close()
                    continuation.finish(throwing: error)
                }
            }
            continuation.onTermination = { _ in task.cancel() }
        }
    }
}

private final class RelayTunnel: @unchecked Sendable {
    private let gatewayURL: URL
    private let bearerToken: String
    private let machineID: String
    private let machinePublicKey: Data
    private let urlSession: URLSession
    private let streamID = UUID().uuidString.replacingOccurrences(of: "-", with: "").lowercased()
    private var noise: RelayNoiseInitiator?
    private var transport: RelayNoiseSession?
    private var started = false

    init(gatewayURL: URL, bearerToken: String, machineID: String, machinePublicKey: Data, urlSession: URLSession) {
        self.gatewayURL = gatewayURL
        self.bearerToken = bearerToken
        self.machineID = machineID
        self.machinePublicKey = machinePublicKey
        self.urlSession = urlSession
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
        let payload = RelayRequestPayload(id: randomID(), method: method, path: path, headers: headers, body: body)
        let plaintext = try JSONEncoder().encode(payload)
        let replies = try await post(frame: GatewayFrame(stream: streamID, kind: "req", data: try transport.seal(plaintext)))
        guard let reply = replies.first else { throw GatewayRelayError.missingFrame }
        return try openResponse(reply, transport: transport)
    }

    func stream(path: String, onChunk: (Data) -> Void) async throws {
        guard let transport else { throw RelaySecurityError.handshakeIncomplete }
        let payload = RelayRequestPayload(
            id: randomID(), method: "GET", path: path,
            headers: ["Accept": "text/event-stream"], body: nil
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
        request.httpBody = try JSONEncoder().encode(GatewayFrameRequest(machineID: machineID, frame: frame))
        return request
    }

    private func randomID() -> String { UUID().uuidString.replacingOccurrences(of: "-", with: "").lowercased() }
}
