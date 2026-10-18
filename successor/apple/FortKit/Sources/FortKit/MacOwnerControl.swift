#if os(macOS)
import Foundation

public enum MacOwnerRecoveryError: Error {
    case identityRejected
}

func ownerControlUnavailable() -> NSError {
    NSError(domain:"Fort.OwnerControl",code:1,userInfo:[NSLocalizedDescriptionKey:"This host has no owner recovery control."])
}

public struct MacOwnerRecoveryRequest: Codable, Sendable {
    public let installationID: String
    public let deviceID: String
    public let timeout: TimeInterval
    public init(installationID: String, deviceID: String, timeout: TimeInterval = 30) throws {
        self.installationID = installationID; self.deviceID = deviceID; self.timeout = timeout
        try validate()
    }
    func validate() throws {
        let invalid = CharacterSet(charactersIn:"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_-").inverted
        guard [installationID,deviceID].allSatisfy({ !$0.isEmpty && $0.utf8.count <= 128 && $0.rangeOfCharacter(from:invalid) == nil }),
              timeout.isFinite, timeout > 0, timeout <= 30 else { throw ClientError.invalidResponse }
    }
}
public struct MacOwnerRecoveryReceipt: Codable, Sendable {
    public let installationID: String
    public let deviceID: String
    public let runtimeStopped: Bool
    public let pairingRevoked: Bool
    public let coreStopped: Bool
    public init(installationID: String, deviceID: String, runtimeStopped: Bool, pairingRevoked: Bool, coreStopped: Bool) {
        self.installationID = installationID; self.deviceID = deviceID
        self.runtimeStopped = runtimeStopped; self.pairingRevoked = pairingRevoked; self.coreStopped = coreStopped
    }
    func validate(for request: MacOwnerRecoveryRequest) throws {
        guard installationID == request.installationID, deviceID == request.deviceID,
              runtimeStopped, pairingRevoked, coreStopped else { throw ClientError.invalidResponse }
    }
}
public protocol MacOwnerControl: Sendable {
    func recoverPairing(_ request: MacOwnerRecoveryRequest) async throws -> MacOwnerRecoveryReceipt
}
#endif
