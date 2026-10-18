import Foundation

public struct MacAgentEndpoint: Codable, Equatable, Sendable {
    public let serviceIdentifier: String
    public let signingTeam: String
    public let appGroup: String
    public var machServiceName: String { appGroup + ".fort.host" }
    public init(serviceIdentifier: String, signingTeam: String, appGroup: String) throws {
        _ = try macPeerRequirement(identifier:serviceIdentifier,signingTeam:signingTeam)
        _ = try macPeerRequirement(identifier:appGroup,signingTeam:signingTeam)
        guard appGroup.hasPrefix("group."), (appGroup + ".fort.host").utf8.count < 128 else { throw ClientError.invalidResponse }
        self.serviceIdentifier = serviceIdentifier; self.signingTeam = signingTeam; self.appGroup = appGroup
    }
    public init(from decoder: any Decoder) throws {
        let values = try [String:String](from:decoder)
        guard Set(values.keys) == ["serviceIdentifier","signingTeam","appGroup"],
              let identifier = values["serviceIdentifier"], let team = values["signingTeam"], let group = values["appGroup"] else { throw ClientError.invalidResponse }
        try self.init(serviceIdentifier:identifier,signingTeam:team,appGroup:group)
    }
}
