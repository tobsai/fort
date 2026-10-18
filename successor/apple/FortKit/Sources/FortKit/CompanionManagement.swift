import Foundation

private struct CompanionManagementKey: CodingKey {
    let stringValue: String
    let intValue: Int? = nil
    init?(stringValue: String) { self.stringValue = stringValue }
    init?(intValue: Int) { return nil }
}
private func managementKeys(_ decoder: Decoder, _ expected: Set<String>) throws {
    let values = try decoder.container(keyedBy:CompanionManagementKey.self)
    guard Set(values.allKeys.map(\.stringValue)) == expected else { throw ClientError.invalidResponse }
}
private func managementDate(_ value: String) -> Bool {
    guard value.utf8.count == 24 else { return false }
    let formatter = ISO8601DateFormatter()
    formatter.formatOptions = [.withInternetDateTime,.withFractionalSeconds]
    guard let date = formatter.date(from:value) else { return false }
    return formatter.string(from:date) == value
}
public struct CompanionInvitation: Decodable, Equatable, Sendable {
    public let installationId: String
    public let invitationId: String
    public let expiresAt: String
    private enum CodingKeys: String, CodingKey { case installationId, invitationId, expiresAt }
    public init(from decoder: Decoder) throws {
        try managementKeys(decoder,["installationId","invitationId","expiresAt"])
        let values = try decoder.container(keyedBy:CodingKeys.self)
        installationId = try values.decode(String.self,forKey:.installationId)
        invitationId = try values.decode(String.self,forKey:.invitationId)
        expiresAt = try values.decode(String.self,forKey:.expiresAt)
        guard validCompanionIdentifier(installationId), validCompanionIdentifier(invitationId), managementDate(expiresAt) else { throw ClientError.invalidResponse }
    }
}
public struct CompanionOverview: Decodable, Equatable, Sendable {
    public let installationId: String
    public let hostDeviceId: String
    public let pairing: CompanionPairingState
    private enum CodingKeys: String, CodingKey { case installationId, hostDeviceId, pairing }
    public init(from decoder: Decoder) throws {
        try managementKeys(decoder,["installationId","hostDeviceId","pairing"])
        let values = try decoder.container(keyedBy:CodingKeys.self)
        installationId = try values.decode(String.self,forKey:.installationId)
        hostDeviceId = try values.decode(String.self,forKey:.hostDeviceId)
        pairing = try values.decode(CompanionPairingState.self,forKey:.pairing)
        guard validCompanionIdentifier(installationId), validCompanionIdentifier(hostDeviceId) else { throw ClientError.invalidResponse }
        if case .paired(let deviceID,_,_,_) = pairing, deviceID == hostDeviceId { throw ClientError.invalidResponse }
    }
}
public enum CompanionPairingState: Decodable, Equatable, Sendable {
    case available
    case invited(invitationID: String, expiresAt: String)
    case paired(deviceID: String, deviceName: String, expiresAt: String?, active: Bool)
    private enum CodingKeys: String, CodingKey { case state, invitationId, expiresAt, deviceId, deviceName, active }
    public init(from decoder: Decoder) throws {
        let values = try decoder.container(keyedBy:CodingKeys.self)
        switch try values.decode(String.self,forKey:.state) {
        case "available":
            try managementKeys(decoder,["state"]); self = .available
        case "invited":
            try managementKeys(decoder,["state","invitationId","expiresAt"])
            let id = try values.decode(String.self,forKey:.invitationId), expiry = try values.decode(String.self,forKey:.expiresAt)
            guard validCompanionIdentifier(id), managementDate(expiry) else { throw ClientError.invalidResponse }
            self = .invited(invitationID:id,expiresAt:expiry)
        case "paired":
            try managementKeys(decoder,["state","deviceId","deviceName","expiresAt","active"])
            let id = try values.decode(String.self,forKey:.deviceId), name = try values.decode(String.self,forKey:.deviceName)
            let expiry = try values.decodeIfPresent(String.self,forKey:.expiresAt), active = try values.decode(Bool.self,forKey:.active)
            guard validCompanionIdentifier(id), !name.trimmingCharacters(in:.whitespacesAndNewlines).isEmpty,
                  name.utf8.count <= 128, expiry.map(managementDate) ?? true, expiry != nil || active else { throw ClientError.invalidResponse }
            self = .paired(deviceID:id,deviceName:name,expiresAt:expiry,active:active)
        default: throw ClientError.invalidResponse
        }
    }
}
