import Foundation

public enum DeviceEndpoint: Equatable, Sendable, Codable {
    case network(baseURL: URL, fixtureLoopback: Bool)
    case macHost(serviceIdentifier: String, signingTeam: String)
    case macAgent(MacAgentEndpoint)

    private enum Keys: String, CodingKey { case kind, baseURL, fixtureLoopback, serviceIdentifier, signingTeam, agent }
    public init(from decoder: any Decoder) throws {
        let values = try decoder.container(keyedBy:Keys.self)
        switch try values.decode(String.self,forKey:.kind) {
        case "network":
            guard !values.contains(.serviceIdentifier), !values.contains(.signingTeam), !values.contains(.agent) else { throw ClientError.invalidResponse }
            self = .network(baseURL:try values.decode(URL.self,forKey:.baseURL),fixtureLoopback:try values.decode(Bool.self,forKey:.fixtureLoopback))
        case "macHost":
            guard !values.contains(.baseURL), !values.contains(.fixtureLoopback), !values.contains(.agent) else { throw ClientError.invalidResponse }
            self = .macHost(serviceIdentifier:try values.decode(String.self,forKey:.serviceIdentifier),signingTeam:try values.decode(String.self,forKey:.signingTeam))
        case "macAgent":
            guard !values.contains(.baseURL), !values.contains(.fixtureLoopback), !values.contains(.serviceIdentifier), !values.contains(.signingTeam) else { throw ClientError.invalidResponse }
            self = .macAgent(try values.decode(MacAgentEndpoint.self,forKey:.agent))
        default: throw ClientError.invalidResponse
        }
        try validate()
    }
    public func encode(to encoder: any Encoder) throws {
        try validate()
        var values = encoder.container(keyedBy:Keys.self)
        switch self {
        case .network(let baseURL,let fixtureLoopback):
            try values.encode("network",forKey:.kind); try values.encode(baseURL,forKey:.baseURL); try values.encode(fixtureLoopback,forKey:.fixtureLoopback)
        case .macHost(let serviceIdentifier,let signingTeam):
            try values.encode("macHost",forKey:.kind); try values.encode(serviceIdentifier,forKey:.serviceIdentifier); try values.encode(signingTeam,forKey:.signingTeam)
        case .macAgent(let agent):
            try values.encode("macAgent",forKey:.kind); try values.encode(agent,forKey:.agent)
        }
    }
    func validate() throws {
        switch self {
        case .network(let url,let fixture): _ = try HTTPFortTransport(baseURL:url,fixtureLoopback:fixture,session:.shared)
        case .macHost(let identifier,let team): _ = try macPeerRequirement(identifier:identifier,signingTeam:team)
        case .macAgent: break
        }
    }
    var connectionSchemaVersion: Int { if case .macAgent = self { 3 } else { 2 } }
    var isLocalHost: Bool { if case .network = self { false } else { true } }
    var displayName: String {
        switch self { case .network(let url,_): url.absoluteString; case .macHost,.macAgent: "This Mac" }
    }
    func transport(session: URLSession) throws -> any FortTransport {
        switch self {
        case .network(let url,let fixture): return try HTTPFortTransport(baseURL:url,fixtureLoopback:fixture,session:session)
        case .macHost(let identifier,let team):
            #if os(macOS)
            return try MacXPCTransport(serviceIdentifier:identifier,signingTeam:team)
            #else
            throw ClientError.localHostUnavailable
            #endif
        case .macAgent(let agent):
            #if os(macOS)
            return try MacXPCTransport(agent:agent)
            #else
            throw ClientError.localHostUnavailable
            #endif
        }
    }
}

public struct DeviceConnection: Codable, Equatable, Sendable {
    public let endpoint: DeviceEndpoint
    public let installationID: String
    public let deviceID: String
    public init(endpoint: DeviceEndpoint, installationID: String, deviceID: String) throws {
        self.endpoint = endpoint; self.installationID = installationID; self.deviceID = deviceID
        try validate()
    }
    init(baseURL: URL, installationID: String, deviceID: String, fixtureLoopback: Bool) {
        endpoint = .network(baseURL:baseURL,fixtureLoopback:fixtureLoopback)
        self.installationID = installationID; self.deviceID = deviceID
    }
    private enum Keys: String, CodingKey { case schemaVersion, endpoint, installationID, deviceID, baseURL, fixtureLoopback }
    public init(from decoder: any Decoder) throws {
        let values = try decoder.container(keyedBy:Keys.self)
        if values.contains(.schemaVersion) {
            let version = try values.decode(Int.self,forKey:.schemaVersion)
            guard [2,3].contains(version),
                  !values.contains(.baseURL), !values.contains(.fixtureLoopback) else { throw ClientError.invalidResponse }
            endpoint = try values.decode(DeviceEndpoint.self,forKey:.endpoint)
            guard version == endpoint.connectionSchemaVersion else { throw ClientError.invalidResponse }
        } else {
            guard !values.contains(.endpoint) else { throw ClientError.invalidResponse }
            endpoint = .network(baseURL:try values.decode(URL.self,forKey:.baseURL),fixtureLoopback:try values.decode(Bool.self,forKey:.fixtureLoopback))
        }
        installationID = try values.decode(String.self,forKey:.installationID)
        deviceID = try values.decode(String.self,forKey:.deviceID)
        try validate()
    }
    public func encode(to encoder: any Encoder) throws {
        try validate()
        var values = encoder.container(keyedBy:Keys.self)
        try values.encode(endpoint.connectionSchemaVersion,forKey:.schemaVersion); try values.encode(endpoint,forKey:.endpoint)
        try values.encode(installationID,forKey:.installationID); try values.encode(deviceID,forKey:.deviceID)
    }
    func validate() throws {
        guard [installationID,deviceID].allSatisfy({ !$0.isEmpty && $0.utf8.count <= 4096 && !$0.contains("/") && $0.rangeOfCharacter(from:.controlCharacters) == nil }) else { throw ClientError.invalidResponse }
        try endpoint.validate()
    }
}
