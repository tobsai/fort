#if os(macOS)
import Foundation
import Darwin

public final class MacHostListener: NSObject, NSXPCListenerDelegate {
    private let requirement: String
    private let transport: any FortTransport
    private let ownerControl: (any MacOwnerControl)?
    public init(clientIdentifier: String, signingTeam: String, transport: any FortTransport, ownerControl: (any MacOwnerControl)? = nil) throws {
        requirement = try macPeerRequirement(identifier:clientIdentifier,signingTeam:signingTeam)
        self.transport = transport
        self.ownerControl = ownerControl
    }
    public func listener(_ listener: NSXPCListener, shouldAcceptNewConnection connection: NSXPCConnection) -> Bool {
        guard connection.effectiveUserIdentifier == geteuid() else { return false }
        connection.setCodeSigningRequirement(requirement)
        let channel = MacHostChannel(transport:transport,ownerControl:ownerControl)
        connection.exportedInterface = NSXPCInterface(with:FortHostChannel.self)
        connection.exportedObject = channel
        connection.interruptionHandler = { channel.cancel() }
        connection.invalidationHandler = { channel.cancel() }
        connection.resume()
        return true
    }
}

final class MacHostChannel: NSObject, FortHostChannel, @unchecked Sendable {
    private let transport: any FortTransport
    private let ownerControl: (any MacOwnerControl)?
    private let lock = NSLock()
    private var started = false
    private var canceled = false
    private var task: Task<Void,Never>?
    private var reply: ((Data?, NSError?) -> Void)?
    init(transport: any FortTransport, ownerControl: (any MacOwnerControl)? = nil) { self.transport = transport; self.ownerControl = ownerControl }
    func exchange(_ bytes: Data, reply: @escaping (Data?, NSError?) -> Void) {
        do {
            guard bytes.count <= 2 * 1024 * 1024 else { throw ClientError.invalidResponse }
            let request = try JSONDecoder().decode(FortRequest.self,from:bytes)
            _ = try request.route()
            try begin(reply:reply) {
                let response = try await self.transport.send(request)
                guard (100..<600).contains(response.status), response.body.count <= 8 * 1024 * 1024 else { throw ClientError.invalidResponse }
                let encoded = try JSONEncoder().encode(response)
                guard encoded.count <= 12 * 1024 * 1024 else { throw ClientError.invalidResponse }
                return encoded
            }
        } catch { reply(nil,error as NSError) }
    }
    func recoverPairing(_ bytes: Data, reply: @escaping (Data?, NSError?) -> Void) {
        do {
            guard bytes.count <= 1024 else { throw ClientError.invalidResponse }
            let request = try JSONDecoder().decode(MacOwnerRecoveryRequest.self,from:bytes)
            try request.validate()
            guard let ownerControl else { throw ownerControlUnavailable() }
            try begin(reply:reply) {
                let receipt = try await ownerControl.recoverPairing(request)
                try receipt.validate(for:request)
                let encoded = try JSONEncoder().encode(receipt)
                guard encoded.count <= 1024 else { throw ClientError.invalidResponse }
                return encoded
            }
        } catch { reply(nil,error as NSError) }
    }
    private func begin(reply: @escaping (Data?,NSError?) -> Void, operation: @escaping @Sendable () async throws -> Data) throws {
        let accepted = lock.withLock {
            guard !started, !canceled else { return false }
            started = true; self.reply = reply
            task = Task {
                do { self.complete(try await operation(),error:nil) }
                catch { self.complete(nil,error:error as NSError) }
            }
            return true
        }
        if !accepted { throw ClientError.invalidResponse }
    }
    func cancel() {
        lock.withLock { canceled = true; task?.cancel(); task = nil; reply = nil }
    }
    private func complete(_ bytes: Data?, error: NSError?) {
        let callback = lock.withLock {
            let callback = reply; reply = nil; task = nil
            return callback
        }
        callback?(bytes,error)
    }
}
#endif
