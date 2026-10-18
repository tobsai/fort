import Foundation

public struct PairingStatus: Codable, Equatable, Sendable {
    public enum Mode: String, Codable, Sendable { case temporary; case ownerLocal = "owner-local" }
    public let deviceId: String
    public let mode: Mode
    public let expiresAt: String?
}
public struct HostStopReceipt: Codable, Sendable {
    public let runtimeStopped: Bool
    public let revoked: Bool
}
public struct OwnedHostStop: Sendable {
    public let hostStopped: Bool
    public let pairingRevoked: Bool
    public let startupUnregistered: Bool
    public init(hostStopped: Bool, pairingRevoked: Bool, startupUnregistered: Bool) {
        self.hostStopped = hostStopped; self.pairingRevoked = pairingRevoked; self.startupUnregistered = startupUnregistered
    }
}
public struct PairingRemovalIntent: Codable, Equatable, Sendable {
    public enum State: String, Codable, Sendable { case requested, confirmed, locallyForgotten }
    public let connection: DeviceConnection
    public let state: State
}
public struct PairingRecoveryProof: Sendable {
    public let connection: DeviceConnection
    public let hostStopped: Bool
    public let pairingRevoked: Bool
    public init(connection: DeviceConnection, hostStopped: Bool, pairingRevoked: Bool) {
        self.connection = connection; self.hostStopped = hostStopped; self.pairingRevoked = pairingRevoked
    }
}
@MainActor public protocol OwnedMacInstallation {
    var appURL: URL { get }
    var supportsHistoryDeletion: Bool { get }
    func stopAndRevoke() async throws -> OwnedHostStop
    func recoverPairingRemoval(_ connection: DeviceConnection) async throws -> PairingRecoveryProof
    func removeHistory() throws
}
extension OwnedMacInstallation {
    public var supportsHistoryDeletion: Bool { false }
    public func recoverPairingRemoval(_ connection: DeviceConnection) async throws -> PairingRecoveryProof { throw OwnerLifecycleError.unavailable }
}
public enum HistoryRemoval: Sendable, Equatable { case keep; case deleteConfirmed }
public struct PreparedAppRemoval: Sendable, Equatable {
    public let appURL: URL
    public let history: HistoryRemoval
}
public enum OwnerLifecycleError: Error, LocalizedError {
    case stopUnconfirmed, startupUnconfirmed, wrongPairing, unavailable, historyDeletionUnavailable
    public var errorDescription: String? {
        switch self {
        case .stopUnconfirmed: "Shutdown or revocation was not confirmed. The app and local pairing have been kept for recovery."
        case .startupUnconfirmed: "Startup unregistration was not confirmed. The app and local pairing have been kept for recovery."
        case .wrongPairing: "The saved pairing changed. Reload settings before continuing."
        case .unavailable: "The installed host has not registered a verified removal path."
        case .historyDeletionUnavailable: "This installation keeps history during removal. No removal action was started."
        }
    }
}
@MainActor public struct OwnerLifecycle {
    private let installation: any OwnedMacInstallation
    private let removeCredential: () throws -> Void
    public init(installation: any OwnedMacInstallation, removeCredential: @escaping () throws -> Void) {
        self.installation = installation; self.removeCredential = removeCredential
    }
    public func prepareForRemoval(history: HistoryRemoval = .keep) async throws -> PreparedAppRemoval {
        guard history != .deleteConfirmed || installation.supportsHistoryDeletion else { throw OwnerLifecycleError.historyDeletionUnavailable }
        let receipt = try await installation.stopAndRevoke()
        guard receipt.hostStopped, receipt.pairingRevoked else { throw OwnerLifecycleError.stopUnconfirmed }
        guard receipt.startupUnregistered else { throw OwnerLifecycleError.startupUnconfirmed }
        try removeCredential()
        if history == .deleteConfirmed { try installation.removeHistory() }
        return PreparedAppRemoval(appURL:installation.appURL,history:history)
    }
}
