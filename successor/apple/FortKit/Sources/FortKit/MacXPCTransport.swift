import Foundation

func macPeerRequirement(identifier: String, signingTeam: String) throws -> String {
    guard identifier.utf8.count <= 255,
          identifier.range(of:"^[A-Za-z0-9-]+(?:\\.[A-Za-z0-9-]+)+$",options:.regularExpression) != nil,
          signingTeam.range(of:"^[A-Z0-9]{10}$",options:.regularExpression) != nil else { throw ClientError.invalidResponse }
    return "anchor apple generic and identifier \"\(identifier)\" and certificate leaf[subject.OU] = \"\(signingTeam)\""
}

#if os(macOS)
@objc(FortHostChannel) public protocol FortHostChannel {
    func exchange(_ request: Data, reply: @escaping (Data?, NSError?) -> Void)
}

public struct MacXPCTransport: FortTransport {
    private let serviceIdentifier: String
    private let requirement: String
    public init(serviceIdentifier: String, signingTeam: String) throws {
        self.serviceIdentifier = serviceIdentifier
        requirement = try macPeerRequirement(identifier:serviceIdentifier,signingTeam:signingTeam)
    }
    public func send(_ request: FortRequest) async throws -> FortResponse {
        try Task.checkCancellation()
        _ = try request.route()
        let encoded = try JSONEncoder().encode(request)
        guard encoded.count <= 2 * 1024 * 1024 else { throw ClientError.invalidResponse }
        let call = XPCRequest(serviceIdentifier:serviceIdentifier,requirement:requirement,timeout:request.timeout)
        let data = try await withTaskCancellationHandler {
            try await withCheckedThrowingContinuation { call.start(encoded,continuation:$0) }
        } onCancel: { call.cancel() }
        guard let response = try? JSONDecoder().decode(FortResponse.self,from:data),
              (100..<600).contains(response.status), response.body.count <= 8 * 1024 * 1024 else { throw ClientError.invalidResponse }
        return response
    }
}

private final class XPCRequest: @unchecked Sendable {
    private let queue = DispatchQueue(label:"Fort.XPCRequest")
    private let serviceIdentifier: String
    private let requirement: String
    private let timeout: TimeInterval
    private var continuation: CheckedContinuation<Data, any Error>?
    private var connection: NSXPCConnection?
    private var deadline: DispatchWorkItem?
    private var outcome: Result<Data, any Error>?

    init(serviceIdentifier: String, requirement: String, timeout: TimeInterval) {
        self.serviceIdentifier = serviceIdentifier; self.requirement = requirement; self.timeout = timeout
    }
    func start(_ bytes: Data, continuation: CheckedContinuation<Data, any Error>) {
        queue.async {
            if let outcome = self.outcome { continuation.resume(with:outcome); return }
            self.continuation = continuation
            let connection = NSXPCConnection(serviceName:self.serviceIdentifier)
            connection.remoteObjectInterface = NSXPCInterface(with:FortHostChannel.self)
            connection.setCodeSigningRequirement(self.requirement)
            self.connection = connection
            let deadline = DispatchWorkItem { [weak self] in self?.finish(.failure(URLError(.timedOut))) }
            self.deadline = deadline; self.queue.asyncAfter(deadline:.now() + self.timeout,execute:deadline)
            connection.resume()
            let proxy = connection.remoteObjectProxyWithErrorHandler { [weak self] error in self?.complete(.failure(error)) }
            guard let channel = proxy as? FortHostChannel else { self.finish(.failure(ClientError.invalidResponse)); return }
            channel.exchange(bytes) { [weak self] data,error in
                if let error { self?.complete(.failure(error)) }
                else if let data, data.count <= 12 * 1024 * 1024 { self?.complete(.success(data)) }
                else { self?.complete(.failure(ClientError.invalidResponse)) }
            }
        }
    }
    func cancel() { complete(.failure(CancellationError())) }
    private func complete(_ result: Result<Data, any Error>) { queue.async { self.finish(result) } }
    private func finish(_ result: Result<Data, any Error>) {
        guard outcome == nil else { return }
        outcome = result; deadline?.cancel(); deadline = nil
        connection?.invalidate(); connection = nil
        continuation?.resume(with:result); continuation = nil
    }
}
#endif
