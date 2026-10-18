#if os(macOS)
import Foundation
import Observation
import ServiceManagement

public enum MacHostStartupStatus: Sendable { case unknown, notRegistered, enabled, requiresApproval, notFound, unsupported }
public enum MacHostStartupOperation: Sendable { case idle, inspecting, enabling, disabling }
public enum MacHostStartupError: Error, LocalizedError {
    case busy, unavailable, unconfirmed
    public var errorDescription: String? {
        switch self {
        case .busy: "A local host startup action is already in progress."
        case .unavailable: "This app does not have an available local host registration."
        case .unconfirmed: "The requested startup change could not be confirmed. Check its current status before retrying."
        }
    }
}
protocol MacHostStartupBackend: Sendable {
    func status() async -> MacHostStartupStatus
    func register() async throws
    func unregister() async throws
}
private actor SystemMacHostStartup: MacHostStartupBackend {
    private let plistName: String
    private lazy var service = SMAppService.agent(plistName:plistName)
    init(plistName: String) { self.plistName = plistName }
    func status() -> MacHostStartupStatus {
        switch service.status {
        case .notRegistered: .notRegistered
        case .enabled: .enabled
        case .requiresApproval: .requiresApproval
        case .notFound: .notFound
        @unknown default: .unsupported
        }
    }
    func register() throws { try service.register() }
    func unregister() async throws {
        try await withCheckedThrowingContinuation { (continuation: CheckedContinuation<Void,any Error>) in
            service.unregister { error in
                if let error { continuation.resume(throwing:error) }
                else { continuation.resume() }
            }
        }
    }
}

@Observable @MainActor public final class MacHostStartup {
    public let endpoint: MacAgentEndpoint
    public var plistName: String { endpoint.serviceIdentifier + ".plist" }
    public private(set) var status: MacHostStartupStatus = .unknown
    public private(set) var operation: MacHostStartupOperation = .idle
    private let backend: any MacHostStartupBackend
    public convenience init(endpoint: MacAgentEndpoint) throws {
        try self.init(endpoint:endpoint,backend:SystemMacHostStartup(plistName:endpoint.serviceIdentifier + ".plist"))
    }
    init(endpoint: MacAgentEndpoint, backend: any MacHostStartupBackend) throws {
        guard (endpoint.serviceIdentifier + ".plist").utf8.count <= 255 else { throw ClientError.invalidResponse }
        self.endpoint = endpoint; self.backend = backend
    }
    public func inspect() async throws -> MacHostStartupStatus {
        guard operation == .idle else { throw MacHostStartupError.busy }
        try Task.checkCancellation()
        operation = .inspecting
        defer { operation = .idle }
        status = await backend.status()
        try Task.checkCancellation()
        return status
    }
    public func enable() async throws -> MacHostStartupStatus {
        guard operation == .idle else { throw MacHostStartupError.busy }
        try Task.checkCancellation()
        operation = .enabling
        defer { operation = .idle }
        status = await backend.status()
        try Task.checkCancellation()
        switch status {
        case .enabled,.requiresApproval: return status
        case .notRegistered: break
        case .unknown,.notFound,.unsupported: throw MacHostStartupError.unavailable
        }
        do { try await backend.register() } catch {}
        status = await backend.status()
        try Task.checkCancellation()
        guard status == .enabled || status == .requiresApproval else { throw MacHostStartupError.unconfirmed }
        return status
    }
    public func disable() async throws -> MacHostStartupStatus {
        guard operation == .idle else { throw MacHostStartupError.busy }
        try Task.checkCancellation()
        operation = .disabling
        defer { operation = .idle }
        status = await backend.status()
        try Task.checkCancellation()
        switch status {
        case .notRegistered: return status
        case .enabled,.requiresApproval: break
        case .unknown,.notFound,.unsupported: throw MacHostStartupError.unavailable
        }
        do { try await backend.unregister() } catch {}
        status = await backend.status()
        try Task.checkCancellation()
        guard status == .notRegistered else { throw MacHostStartupError.unconfirmed }
        return status
    }
}
#endif
