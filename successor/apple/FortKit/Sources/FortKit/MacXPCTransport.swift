import Foundation

func macPeerRequirement(identifier: String, signingTeam: String) throws -> String {
    guard identifier.utf8.count <= 255,
          identifier.range(of:"^[A-Za-z0-9-]+(?:\\.[A-Za-z0-9-]+)+$",options:.regularExpression) != nil,
          signingTeam.range(of:"^[A-Z0-9]{10}$",options:.regularExpression) != nil else { throw ClientError.invalidResponse }
    // Mac App Store and TestFlight copies are re-signed by Apple, whose leaf certificate (marked by
    // 1.2.840.113635.100.6.1.9) doesn't carry the team in its OU. Apple only signs those for the team that
    // owns the bundle ID, so accept that marker as well as the team's own certificate, the same pair
    // Xcode writes into a distribution build's designated requirement. Callers that read signing
    // information still compare the team identifier exactly.
    return "anchor apple generic and identifier \"\(identifier)\" and (certificate leaf[field.1.2.840.113635.100.6.1.9] exists or certificate leaf[subject.OU] = \"\(signingTeam)\")"
}

#if os(macOS)
@objc(FortHostChannel) public protocol FortHostChannel {
    func exchange(_ request: Data, reply: @escaping (Data?, NSError?) -> Void)
    func recoverPairing(_ request: Data, reply: @escaping (Data?, NSError?) -> Void)
    func setupOwner(_ request: Data, reply: @escaping (Data?, NSError?) -> Void)
    func controlNetwork(_ request: Data, reply: @escaping (Data?, NSError?) -> Void)
}

public struct MacXPCTransport: FortTransport, MacOwnerControl, OwnerSetupControl, OwnerSetupCancellationControl, MacNetworkControl {
    private let destination: MacXPCDestination
    private let requirement: String
    public init(serviceIdentifier: String, signingTeam: String) throws {
        destination = .embedded(serviceIdentifier)
        requirement = try macPeerRequirement(identifier:serviceIdentifier,signingTeam:signingTeam)
    }
    public init(agent: MacAgentEndpoint) throws {
        destination = .agent(agent)
        requirement = try macPeerRequirement(identifier:agent.serviceIdentifier,signingTeam:agent.signingTeam)
    }
    public func send(_ request: FortRequest) async throws -> FortResponse {
        try Task.checkCancellation()
        _ = try request.route()
        let encoded = try JSONEncoder().encode(request)
        guard encoded.count <= 2 * 1024 * 1024 else { throw ClientError.invalidResponse }
        let call = XPCRequest(destination:destination,requirement:requirement,timeout:request.timeout)
        let reply = try await withTaskCancellationHandler {
            try await withCheckedThrowingContinuation { call.start(encoded,continuation:$0) }
        } onCancel: { call.cancel() }
        guard let response = try? JSONDecoder().decode(FortResponse.self,from:reply.data),
              (100..<600).contains(response.status), response.body.count <= 8 * 1024 * 1024 else { throw ClientError.invalidResponse }
        return response
    }
    public func recoverPairing(_ request: MacOwnerRecoveryRequest) async throws -> MacOwnerRecoveryReceipt {
        try await recovery(request).receipt
    }
    public func recoverPairingAndObserveHost(_ request: MacOwnerRecoveryRequest) async throws -> MacHostRecoveryObservation {
        let (receipt,reply) = try await recovery(request)
        let host = try MacHostProcess.observe(processID:reply.processID,effectiveUserID:reply.effectiveUserID)
        return MacHostRecoveryObservation(receipt:receipt,host:host)
    }
    private func recovery(_ request: MacOwnerRecoveryRequest) async throws -> (receipt: MacOwnerRecoveryReceipt, reply: XPCReply) {
        try Task.checkCancellation(); try request.validate()
        let encoded = try JSONEncoder().encode(request)
        guard encoded.count <= 1024 else { throw ClientError.invalidResponse }
        let call = XPCRequest(destination:destination,requirement:requirement,timeout:request.timeout,operation:.recovery,responseLimit:1024)
        let reply = try await withTaskCancellationHandler {
            try await withCheckedThrowingContinuation { call.start(encoded,continuation:$0) }
        } onCancel: { call.cancel() }
        let receipt = try JSONDecoder().decode(MacOwnerRecoveryReceipt.self,from:reply.data)
        try receipt.validate(for:request)
        return (receipt,reply)
    }
    public func beginOwnerSetup(timeout: TimeInterval = 30) async throws -> OwnerSetupState {
        let data = try await setup(.begin(timeout))
        guard let state = try? JSONDecoder().decode(OwnerSetupState.self,from:data) else { throw ClientError.invalidResponse }
        return state
    }
    public func completeOwnerSetup(_ request: OwnerSetupRequest, timeout: TimeInterval = 30) async throws -> OwnerSetupReceipt {
        let data = try await setup(.complete(request,timeout))
        guard let receipt = try? JSONDecoder().decode(OwnerSetupReceipt.self,from:data),
              receipt.installationID == request.installationID, receipt.deviceID == request.deviceID else { throw ClientError.invalidResponse }
        return receipt
    }
    public func cancelOwnerSetup(_ identity: OwnerSetupIdentity, timeout: TimeInterval = 30) async throws -> OwnerSetupCancellation {
        let data = try await setup(.cancel(identity,timeout))
        guard let receipt = try? JSONDecoder().decode(OwnerSetupCancellation.self,from:data), receipt.identity == identity else { throw ClientError.invalidResponse }
        return receipt
    }
    public func controlNetwork(_ request: MacNetworkRequest) async throws -> MacNetworkStatus {
        try Task.checkCancellation(); try request.validate()
        let encoded = try JSONEncoder().encode(request)
        guard encoded.count <= 192 * 1024 else { throw ClientError.invalidResponse }
        let call = XPCRequest(destination:destination,requirement:requirement,timeout:request.timeout,operation:.network,responseLimit:4096)
        let reply = try await withTaskCancellationHandler {
            try await withCheckedThrowingContinuation { call.start(encoded,continuation:$0) }
        } onCancel: { call.cancel() }
        let status = try JSONDecoder().decode(MacNetworkStatus.self,from:reply.data)
        try status.validate(for:request)
        return status
    }
    private func setup(_ command: OwnerSetupCommand) async throws -> Data {
        try Task.checkCancellation(); try command.validate()
        let encoded = try JSONEncoder().encode(command)
        guard encoded.count <= 1024 else { throw ClientError.invalidResponse }
        let call = XPCRequest(destination:destination,requirement:requirement,timeout:command.timeout,operation:.setup,responseLimit:1024)
        let reply = try await withTaskCancellationHandler {
            try await withCheckedThrowingContinuation { call.start(encoded,continuation:$0) }
        } onCancel: { call.cancel() }
        return reply.data
    }
}

private enum MacXPCDestination: Sendable {
    case embedded(String), agent(MacAgentEndpoint)
    func connection() -> NSXPCConnection {
        switch self {
        case .embedded(let identifier): NSXPCConnection(serviceName:identifier)
        case .agent(let agent): NSXPCConnection(machServiceName:agent.machServiceName,options:[])
        }
    }
}

private struct XPCReply: Sendable {
    let data: Data
    let processID: Int32
    let effectiveUserID: UInt32
}

private enum XPCOperation { case exchange, recovery, setup, network }
private final class XPCRequest: @unchecked Sendable {
    private let queue = DispatchQueue(label:"Fort.XPCRequest")
    private let destination: MacXPCDestination
    private let requirement: String
    private let timeout: TimeInterval
    private let operation: XPCOperation
    private let responseLimit: Int
    private var continuation: CheckedContinuation<XPCReply, any Error>?
    private var connection: NSXPCConnection?
    private var deadline: DispatchWorkItem?
    private var outcome: Result<XPCReply, any Error>?

    init(destination: MacXPCDestination, requirement: String, timeout: TimeInterval, operation: XPCOperation = .exchange, responseLimit: Int = 12 * 1024 * 1024) {
        self.destination = destination; self.requirement = requirement; self.timeout = timeout
        self.operation = operation; self.responseLimit = responseLimit
    }
    func start(_ bytes: Data, continuation: CheckedContinuation<XPCReply, any Error>) {
        queue.async { [self] in
            if let outcome = self.outcome { continuation.resume(with:outcome); return }
            self.continuation = continuation
            let connection = self.destination.connection()
            connection.remoteObjectInterface = NSXPCInterface(with:FortHostChannel.self)
            connection.setCodeSigningRequirement(self.requirement)
            self.connection = connection
            let deadline = DispatchWorkItem { [weak self] in self?.finish(.failure(URLError(.timedOut))) }
            self.deadline = deadline; self.queue.asyncAfter(deadline:.now() + self.timeout,execute:deadline)
            connection.resume()
            let proxy = connection.remoteObjectProxyWithErrorHandler { [weak self] error in self?.complete(.failure(error)) }
            guard let channel = proxy as? FortHostChannel else { self.finish(.failure(ClientError.invalidResponse)); return }
            let reply: (Data?,NSError?) -> Void = { [weak self] data,error in
                guard let self else { return }
                if let error { self.complete(.failure(error)) }
                else if let data, data.count <= self.responseLimit { self.received(data) }
                else { self.complete(.failure(ClientError.invalidResponse)) }
            }
            switch self.operation {
            case .exchange: channel.exchange(bytes,reply:reply)
            case .recovery: channel.recoverPairing(bytes,reply:reply)
            case .setup: channel.setupOwner(bytes,reply:reply)
            case .network: channel.controlNetwork(bytes,reply:reply)
            }
        }
    }
    func cancel() { complete(.failure(CancellationError())) }
    private func received(_ data: Data) {
        queue.async {
            guard self.outcome == nil, let connection = self.connection else { return }
            let pid = connection.processIdentifier, uid = connection.effectiveUserIdentifier
            guard pid > 1, pid != getpid(), uid == geteuid() else { self.finish(.failure(ClientError.invalidResponse)); return }
            self.finish(.success(XPCReply(data:data,processID:pid,effectiveUserID:uid)))
        }
    }
    private func complete(_ result: Result<XPCReply, any Error>) { queue.async { self.finish(result) } }
    private func finish(_ result: Result<XPCReply, any Error>) {
        guard outcome == nil else { return }
        outcome = result; deadline?.cancel(); deadline = nil
        connection?.invalidate(); connection = nil
        continuation?.resume(with:result); continuation = nil
    }
}
#endif
