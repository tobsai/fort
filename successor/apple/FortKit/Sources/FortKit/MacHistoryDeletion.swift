#if os(macOS)
import Foundation

public struct MacHistoryDeletionRequest: Codable, Equatable, Sendable {
    public let installationID: String
    public let deviceID: String
    public let recoveryEpoch: String
    public let deletionID: String
    public let timeout: TimeInterval
    enum CodingKeys: String, CodingKey {
        case installationID = "installationId", deviceID = "deviceId", recoveryEpoch, deletionID = "deletionId", timeout
    }
    public init(installationID: String, deviceID: String, recoveryEpoch: String, deletionID: String, timeout: TimeInterval = 10) throws {
        guard [installationID,deviceID,recoveryEpoch,deletionID].allSatisfy(Self.validID),
              timeout.isFinite, timeout > 0, timeout <= 30 else { throw ClientError.invalidResponse }
        self.installationID = installationID; self.deviceID = deviceID
        self.recoveryEpoch = recoveryEpoch; self.deletionID = deletionID; self.timeout = timeout
    }
    public init(from decoder: any Decoder) throws {
        let fields = try decoder.container(keyedBy:CodingKeys.self)
        try self.init(installationID:fields.decode(String.self,forKey:.installationID),deviceID:fields.decode(String.self,forKey:.deviceID),
            recoveryEpoch:fields.decode(String.self,forKey:.recoveryEpoch),deletionID:fields.decode(String.self,forKey:.deletionID),timeout:fields.decode(TimeInterval.self,forKey:.timeout))
    }
    static func validID(_ value: String) -> Bool {
        !value.isEmpty && value.utf8.count <= 128 && value.utf8.allSatisfy { (48...57).contains($0) || (65...90).contains($0) || (97...122).contains($0) || $0 == 45 || $0 == 95 }
    }
}

public struct MacHistoryDeletionReceipt: Codable, Equatable, Sendable {
    public let installationId: String
    public let deviceId: String
    public let deletionId: String
    public let previousRecoveryEpoch: String
    public let recoveryEpoch: String
    public let deletedRecords: Int
    func validate(for request: MacHistoryDeletionRequest) throws {
        guard installationId == request.installationID, deviceId == request.deviceID, deletionId == request.deletionID,
              previousRecoveryEpoch == request.recoveryEpoch, recoveryEpoch != previousRecoveryEpoch,
              MacHistoryDeletionRequest.validID(recoveryEpoch), deletedRecords >= 0 else { throw ClientError.invalidResponse }
    }
}
#endif
