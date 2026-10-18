import Foundation

private func setupIdentifier(_ value: String) -> Bool {
    let invalid = CharacterSet(charactersIn:"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_-").inverted
    return !value.isEmpty && value.utf8.count <= 128 && value.rangeOfCharacter(from:invalid) == nil
}
private func setupExpiry(_ value: String) -> Bool {
    guard value.utf8.count == 24 else { return false }
    let formatter = ISO8601DateFormatter()
    formatter.formatOptions = [.withInternetDateTime,.withFractionalSeconds]
    guard let date = formatter.date(from:value) else { return false }
    return formatter.string(from:date) == value
}

public struct OwnerSetupOffer: Codable, Equatable, Sendable {
    public let installationID: String
    public let setupID: String
    public let expiresAt: String
    enum CodingKeys: String, CodingKey { case installationID = "installationId", setupID = "setupId", expiresAt }
    public init(installationID: String, setupID: String, expiresAt: String) throws {
        guard setupIdentifier(installationID), setupIdentifier(setupID), setupExpiry(expiresAt) else { throw ClientError.invalidResponse }
        self.installationID = installationID; self.setupID = setupID; self.expiresAt = expiresAt
    }
    public init(from decoder: any Decoder) throws {
        let value = try decoder.container(keyedBy:CodingKeys.self)
        try self.init(installationID:value.decode(String.self,forKey:.installationID),setupID:value.decode(String.self,forKey:.setupID),expiresAt:value.decode(String.self,forKey:.expiresAt))
    }
}

public struct OwnerSetupReceipt: Codable, Equatable, Sendable {
    public let installationID: String
    public let deviceID: String
    public let expiresAt: String?
    enum CodingKeys: String, CodingKey { case installationID = "installationId", deviceID = "deviceId", expiresAt }
    public init(installationID: String, deviceID: String, expiresAt: String?) throws {
        guard setupIdentifier(installationID), setupIdentifier(deviceID), expiresAt.map(setupExpiry) ?? true else { throw ClientError.invalidResponse }
        self.installationID = installationID; self.deviceID = deviceID; self.expiresAt = expiresAt
    }
    public init(from decoder: any Decoder) throws {
        let value = try decoder.container(keyedBy:CodingKeys.self)
        guard value.contains(.expiresAt) else { throw ClientError.invalidResponse }
        try self.init(installationID:value.decode(String.self,forKey:.installationID),deviceID:value.decode(String.self,forKey:.deviceID),expiresAt:value.decodeIfPresent(String.self,forKey:.expiresAt))
    }
    public func encode(to encoder: any Encoder) throws {
        var value = encoder.container(keyedBy:CodingKeys.self)
        try value.encode(installationID,forKey:.installationID); try value.encode(deviceID,forKey:.deviceID)
        try value.encode(expiresAt,forKey:.expiresAt)
    }
}

public enum OwnerSetupState: Codable, Equatable, Sendable {
    case available(OwnerSetupOffer)
    case paired(OwnerSetupReceipt)
    enum CodingKeys: String, CodingKey { case kind }
    public init(from decoder: any Decoder) throws {
        let value = try decoder.container(keyedBy:CodingKeys.self)
        switch try value.decode(String.self,forKey:.kind) {
        case "available": self = .available(try OwnerSetupOffer(from:decoder))
        case "paired": self = .paired(try OwnerSetupReceipt(from:decoder))
        default: throw ClientError.invalidResponse
        }
    }
    public func encode(to encoder: any Encoder) throws {
        var value = encoder.container(keyedBy:CodingKeys.self)
        switch self {
        case .available(let offer): try value.encode("available",forKey:.kind); try offer.encode(to:encoder)
        case .paired(let receipt): try value.encode("paired",forKey:.kind); try receipt.encode(to:encoder)
        }
    }
}

public struct OwnerSetupRequest: Codable, Equatable, Sendable {
    public let installationID: String
    public let setupID: String
    public let deviceID: String
    public let deviceToken: String
    enum CodingKeys: String, CodingKey { case installationID = "installationId", setupID = "setupId", deviceID = "deviceId", deviceToken }
    public init(installationID: String, setupID: String, deviceID: String, deviceToken: String) throws {
        guard [installationID,setupID,deviceID].allSatisfy(setupIdentifier), deviceToken.utf8.count == 43,
              let token = Data(base64Encoded:deviceToken.replacingOccurrences(of:"-",with:"+").replacingOccurrences(of:"_",with:"/") + "="), token.count == 32,
              token.base64EncodedString().replacingOccurrences(of:"+",with:"-").replacingOccurrences(of:"/",with:"_").replacingOccurrences(of:"=",with:"") == deviceToken else { throw ClientError.invalidResponse }
        self.installationID = installationID; self.setupID = setupID; self.deviceID = deviceID; self.deviceToken = deviceToken
    }
    public init(from decoder: any Decoder) throws {
        let value = try decoder.container(keyedBy:CodingKeys.self)
        try self.init(installationID:value.decode(String.self,forKey:.installationID),setupID:value.decode(String.self,forKey:.setupID),deviceID:value.decode(String.self,forKey:.deviceID),deviceToken:value.decode(String.self,forKey:.deviceToken))
    }
}

public protocol OwnerSetupControl: Sendable {
    func beginOwnerSetup(timeout: TimeInterval) async throws -> OwnerSetupState
    func completeOwnerSetup(_ request: OwnerSetupRequest, timeout: TimeInterval) async throws -> OwnerSetupReceipt
}
