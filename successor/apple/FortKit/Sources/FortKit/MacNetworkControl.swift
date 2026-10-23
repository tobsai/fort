#if os(macOS)
import Foundation

private func networkIdentifier(_ value: String) -> Bool {
    !value.isEmpty && value.utf8.count <= 128 && value.rangeOfCharacter(from:CharacterSet(charactersIn:"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_-").inverted) == nil
}
func networkControlUnavailable() -> NSError {
    NSError(domain:"Fort.NetworkControl",code:1,userInfo:[NSLocalizedDescriptionKey:"This host has no phone listener control."])
}

public struct MacNetworkOwner: Codable, Equatable, Sendable {
    public let installationId: String
    public let deviceId: String
    public init(installationId: String, deviceId: String) { self.installationId = installationId; self.deviceId = deviceId }
    var valid: Bool { networkIdentifier(installationId) && networkIdentifier(deviceId) }
}

public struct MacNetworkConfiguration: Codable, Equatable, Sendable {
    public let bindAddress: String
    public let prefixLength: Int
    public let port: Int
    public let serverName: String
    public let certificate: String
    public let privateKey: String
    public init(bindAddress: String, prefixLength: Int, port: Int, serverName: String, certificate: Data, privateKey: Data) {
        self.bindAddress = bindAddress; self.prefixLength = prefixLength; self.port = port; self.serverName = serverName
        self.certificate = certificate.base64EncodedString(); self.privateKey = privateKey.base64EncodedString()
    }
}

public enum MacNetworkCommand: Codable, Equatable, Sendable {
    case inspect(owner: MacNetworkOwner)
    case start(owner: MacNetworkOwner, sessionId: String, generation: Int, configuration: MacNetworkConfiguration)
    case stop(owner: MacNetworkOwner, sessionId: String, generation: Int)
    private enum CodingKeys: String, CodingKey { case operation, owner, sessionId, generation, configuration }
    public init(from decoder: Decoder) throws {
        let values = try decoder.container(keyedBy:CodingKeys.self)
        let owner = try values.decode(MacNetworkOwner.self,forKey:.owner)
        switch try values.decode(String.self,forKey:.operation) {
        case "inspect": self = .inspect(owner:owner)
        case "start": self = .start(owner:owner,sessionId:try values.decode(String.self,forKey:.sessionId),generation:try values.decode(Int.self,forKey:.generation),configuration:try values.decode(MacNetworkConfiguration.self,forKey:.configuration))
        case "stop": self = .stop(owner:owner,sessionId:try values.decode(String.self,forKey:.sessionId),generation:try values.decode(Int.self,forKey:.generation))
        default: throw ClientError.invalidResponse
        }
    }
    public func encode(to encoder: Encoder) throws {
        var values = encoder.container(keyedBy:CodingKeys.self)
        switch self {
        case .inspect(let owner):
            try values.encode("inspect",forKey:.operation); try values.encode(owner,forKey:.owner)
        case .start(let owner,let session,let generation,let configuration):
            try values.encode("start",forKey:.operation); try values.encode(owner,forKey:.owner)
            try values.encode(session,forKey:.sessionId); try values.encode(generation,forKey:.generation); try values.encode(configuration,forKey:.configuration)
        case .stop(let owner,let session,let generation):
            try values.encode("stop",forKey:.operation); try values.encode(owner,forKey:.owner)
            try values.encode(session,forKey:.sessionId); try values.encode(generation,forKey:.generation)
        }
    }
    var owner: MacNetworkOwner {
        switch self { case .inspect(let owner), .start(let owner,_,_,_), .stop(let owner,_,_): owner }
    }
}

public struct MacNetworkRequest: Codable, Equatable, Sendable {
    public let command: MacNetworkCommand
    public let timeout: TimeInterval
    public init(command: MacNetworkCommand, timeout: TimeInterval = 30) throws {
        self.command = command; self.timeout = timeout
        try validate()
    }
    func validate() throws {
        guard command.owner.valid, timeout.isFinite, timeout > 0, timeout <= 30 else { throw ClientError.invalidResponse }
    }
}

public enum MacNetworkListener: Codable, Equatable, Sendable {
    case stopped(cause: String)
    case starting, stopping, unconfirmed
    case active(url: URL)
    private enum CodingKeys: String, CodingKey { case state, cause, url }
    public init(from decoder: Decoder) throws {
        let values = try decoder.container(keyedBy:CodingKeys.self)
        switch try values.decode(String.self,forKey:.state) {
        case "stopped": self = .stopped(cause:try values.decode(String.self,forKey:.cause))
        case "starting": self = .starting
        case "stopping": self = .stopping
        case "unconfirmed": self = .unconfirmed
        case "active":
            guard let url = URL(string:try values.decode(String.self,forKey:.url)), url.scheme == "https" else { throw ClientError.invalidResponse }
            self = .active(url:url)
        default: throw ClientError.invalidResponse
        }
    }
    public func encode(to encoder: Encoder) throws {
        var values = encoder.container(keyedBy:CodingKeys.self)
        switch self {
        case .stopped(let cause): try values.encode("stopped",forKey:.state); try values.encode(cause,forKey:.cause)
        case .starting: try values.encode("starting",forKey:.state)
        case .stopping: try values.encode("stopping",forKey:.state)
        case .unconfirmed: try values.encode("unconfirmed",forKey:.state)
        case .active(let url): try values.encode("active",forKey:.state); try values.encode(url.absoluteString,forKey:.url)
        }
    }
}

public struct MacNetworkStatus: Codable, Equatable, Sendable {
    public let owner: MacNetworkOwner
    public let sessionId: String
    public let generation: Int
    public let listener: MacNetworkListener
    func validate(for request: MacNetworkRequest) throws {
        guard owner == request.command.owner, networkIdentifier(sessionId), generation >= 0 else { throw ClientError.invalidResponse }
    }
}

public protocol MacNetworkControl: Sendable {
    func controlNetwork(_ request: MacNetworkRequest) async throws -> MacNetworkStatus
}
#endif
