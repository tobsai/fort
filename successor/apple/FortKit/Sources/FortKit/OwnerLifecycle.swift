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
}
@MainActor public protocol OwnedMacInstallation {
    var appURL: URL { get }
    func stopAndRevoke() async throws -> OwnedHostStop
    func removeApp() throws
    func removeHistory() throws
}
public enum HistoryRemoval: Sendable { case keep; case deleteConfirmed }
public enum OwnerLifecycleError: Error, LocalizedError {
    case stopUnconfirmed, wrongPairing, unavailable
    public var errorDescription: String? {
        switch self {
        case .stopUnconfirmed: "Shutdown or revocation was not confirmed. The app and local pairing have been kept for recovery."
        case .wrongPairing: "The saved pairing changed. Reload settings before continuing."
        case .unavailable: "The installed host has not registered a verified removal path."
        }
    }
}
@MainActor public struct OwnerLifecycle {
    private let installation: any OwnedMacInstallation
    private let removeCredential: () throws -> Void
    public init(installation: any OwnedMacInstallation, removeCredential: @escaping () throws -> Void) {
        self.installation = installation; self.removeCredential = removeCredential
    }
    public func uninstall(history: HistoryRemoval = .keep) async throws {
        let receipt = try await installation.stopAndRevoke()
        guard receipt.hostStopped, receipt.pairingRevoked else { throw OwnerLifecycleError.stopUnconfirmed }
        try removeCredential()
        try installation.removeApp()
        if history == .deleteConfirmed { try installation.removeHistory() }
    }
}
