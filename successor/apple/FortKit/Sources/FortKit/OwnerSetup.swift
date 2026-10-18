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

public struct OwnerSetupIdentity: Codable, Equatable, Sendable {
    public let installationID: String
    public let setupID: String
    public let deviceID: String
    enum CodingKeys: String, CodingKey { case installationID = "installationId", setupID = "setupId", deviceID = "deviceId" }
    public init(installationID: String, setupID: String, deviceID: String) throws {
        guard [installationID,setupID,deviceID].allSatisfy(setupIdentifier) else { throw ClientError.invalidResponse }
        self.installationID = installationID; self.setupID = setupID; self.deviceID = deviceID
    }
    public init(from decoder: any Decoder) throws {
        let value = try decoder.container(keyedBy:CodingKeys.self)
        try self.init(installationID:value.decode(String.self,forKey:.installationID),setupID:value.decode(String.self,forKey:.setupID),deviceID:value.decode(String.self,forKey:.deviceID))
    }
}

public enum OwnerSetupCancellation: Codable, Equatable, Sendable {
    case notPaired(OwnerSetupIdentity)
    case revoked(OwnerSetupIdentity)
    public var identity: OwnerSetupIdentity {
        switch self { case .notPaired(let value), .revoked(let value): value }
    }
    enum CodingKeys: String, CodingKey { case outcome, runtimeStopped }
    public init(from decoder: any Decoder) throws {
        let value = try decoder.container(keyedBy:CodingKeys.self)
        let identity = try OwnerSetupIdentity(from:decoder)
        switch try value.decode(String.self,forKey:.outcome) {
        case "notPaired":
            guard !value.contains(.runtimeStopped) else { throw ClientError.invalidResponse }
            self = .notPaired(identity)
        case "revoked":
            guard try value.decode(Bool.self,forKey:.runtimeStopped) else { throw ClientError.invalidResponse }
            self = .revoked(identity)
        default: throw ClientError.invalidResponse
        }
    }
    public func encode(to encoder: any Encoder) throws {
        var value = encoder.container(keyedBy:CodingKeys.self)
        try identity.encode(to:encoder)
        switch self {
        case .notPaired: try value.encode("notPaired",forKey:.outcome)
        case .revoked: try value.encode("revoked",forKey:.outcome); try value.encode(true,forKey:.runtimeStopped)
        }
    }
}

public protocol OwnerSetupCancellationControl: Sendable {
    func cancelOwnerSetup(_ identity: OwnerSetupIdentity, timeout: TimeInterval) async throws -> OwnerSetupCancellation
}

func ownerSetupUnavailable() -> NSError {
    NSError(domain:"Fort.OwnerSetup",code:1,userInfo:[NSLocalizedDescriptionKey:"Owner setup is unavailable on this host."])
}
func ownerSetupUnconfirmed() -> NSError {
    NSError(domain:"Fort.OwnerSetup",code:2,userInfo:[NSLocalizedDescriptionKey:"Owner setup was not confirmed. Retry with the same prepared device."])
}

enum OwnerSetupCommand: Codable, Sendable {
    case begin(TimeInterval)
    case complete(OwnerSetupRequest,TimeInterval)
    case cancel(OwnerSetupIdentity,TimeInterval)
    var timeout: TimeInterval {
        switch self { case .begin(let value): value; case .complete(_,let value), .cancel(_,let value): value }
    }
    func validate() throws {
        guard timeout.isFinite, timeout > 0, timeout <= 30 else { throw ClientError.invalidResponse }
    }
    enum CodingKeys: String, CodingKey { case operation, timeout }
    private struct Field: CodingKey {
        let stringValue: String
        var intValue: Int? { nil }
        init?(stringValue: String) { self.stringValue = stringValue }
        init?(intValue: Int) { return nil }
    }
    init(from decoder: any Decoder) throws {
        let value = try decoder.container(keyedBy:CodingKeys.self)
        let timeout = try value.decode(TimeInterval.self,forKey:.timeout)
        let operation = try value.decode(String.self,forKey:.operation)
        let fields = try decoder.container(keyedBy:Field.self).allKeys.map(\.stringValue)
        let allowed = operation == "begin" ? ["operation","timeout"] : ["operation","timeout","installationId","setupId","deviceId"] + (operation == "complete" ? ["deviceToken"] : [])
        guard fields.allSatisfy(allowed.contains) else { throw ClientError.invalidResponse }
        switch operation {
        case "begin": self = .begin(timeout)
        case "complete": self = .complete(try OwnerSetupRequest(from:decoder),timeout)
        case "cancel": self = .cancel(try OwnerSetupIdentity(from:decoder),timeout)
        default: throw ClientError.invalidResponse
        }
        try validate()
    }
    func encode(to encoder: any Encoder) throws {
        try validate()
        var value = encoder.container(keyedBy:CodingKeys.self)
        try value.encode(timeout,forKey:.timeout)
        switch self {
        case .begin: try value.encode("begin",forKey:.operation)
        case .complete(let request,_): try value.encode("complete",forKey:.operation); try request.encode(to:encoder)
        case .cancel(let identity,_): try value.encode("cancel",forKey:.operation); try identity.encode(to:encoder)
        }
    }
}
