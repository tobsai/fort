import Foundation
import Testing
@testable import FortKit

@Test func macAgentEndpointKeepsMachLookupSeparateFromPeerIdentityAndUsesANewStoredVersion() throws {
    let agent = try MacAgentEndpoint(serviceIdentifier:"io.fort.synthetic.agent",signingTeam:"AAAAAAAAAA",appGroup:"group.io.fort.synthetic")
    #expect(agent.machServiceName == "group.io.fort.synthetic.fort.host")
    #expect(agent.serviceIdentifier == "io.fort.synthetic.agent" && agent.signingTeam == "AAAAAAAAAA")
    let connection = try DeviceConnection(endpoint:.macAgent(agent),installationID:"installation",deviceID:"device")
    let data = try JSONEncoder().encode(connection)
    let record = try #require(JSONSerialization.jsonObject(with:data) as? [String:Any])
    #expect(record["schemaVersion"] as? Int == 3)
    #expect(try JSONDecoder().decode(DeviceConnection.self,from:data) == connection)
    #expect(connection.endpoint.displayName == "This Mac")
    let old = try DeviceConnection(endpoint:.macHost(serviceIdentifier:agent.serviceIdentifier,signingTeam:agent.signingTeam),installationID:"installation",deviceID:"device")
    let oldRecord = try #require(JSONSerialization.jsonObject(with:JSONEncoder().encode(old)) as? [String:Any])
    #expect(oldRecord["schemaVersion"] as? Int == 2 && old.endpoint != connection.endpoint)
}

@Test func macAgentEndpointRejectsUnboundedOrUnpinnedConfiguration() throws {
    for group in ["", "io.fort.synthetic", "group.", "group.io..fort", "group.io.fort/other", "group.io.fort\u{0}", "group.éxample", "group." + String(repeating:"a",count:112)] {
        #expect(throws:(any Error).self) { try MacAgentEndpoint(serviceIdentifier:"io.fort.agent",signingTeam:"AAAAAAAAAA",appGroup:group) }
    }
    for identifier in ["", "agent", "io.fort.\" or true", "io.fort/agent", String(repeating:"a",count:256) + ".agent"] {
        #expect(throws:(any Error).self) { try MacAgentEndpoint(serviceIdentifier:identifier,signingTeam:"AAAAAAAAAA",appGroup:"group.io.fort.synthetic") }
    }
    for team in ["", "short", "aaaaaaaaaa", "AAAAAAAAAAA", "AAAA AAAA"] {
        #expect(throws:(any Error).self) { try MacAgentEndpoint(serviceIdentifier:"io.fort.agent",signingTeam:team,appGroup:"group.io.fort.synthetic") }
    }
    let maximum = try MacAgentEndpoint(serviceIdentifier:"io.fort.agent",signingTeam:"AAAAAAAAAA",appGroup:"group." + String(repeating:"a",count:111))
    #expect(maximum.machServiceName.utf8.count == 127)
}

@Test func macAgentConnectionRejectsMixedAndUnknownSchemasWithoutRewritingSavedBytes() throws {
    let directory = FileManager.default.temporaryDirectory.appendingPathComponent("fort-agent-endpoint-" + UUID().uuidString)
    defer { try? FileManager.default.removeItem(at:directory) }
    let agent = try MacAgentEndpoint(serviceIdentifier:"io.fort.synthetic.agent",signingTeam:"AAAAAAAAAA",appGroup:"group.io.fort.synthetic")
    let connection = try DeviceConnection(endpoint:.macAgent(agent),installationID:"installation",deviceID:"device")
    let data = try JSONEncoder().encode(connection)
    let storage = try DeviceStorage(directory:directory)
    let file = directory.appendingPathComponent("connection.json")
    try data.write(to:file)
    #expect(try storage.connection() == connection)
    #expect(try Data(contentsOf:file) == data)
    let base = try #require(JSONSerialization.jsonObject(with:data) as? [String:Any])
    for mutation in ["schema2","future","network","embedded","mixed","bad-agent","no-agent"] {
        var record = base
        var endpoint = try #require(record["endpoint"] as? [String:Any])
        switch mutation {
        case "schema2": record["schemaVersion"] = 2
        case "future": record["schemaVersion"] = 4
        case "network": endpoint = ["kind":"network","baseURL":"https://fixture.invalid","fixtureLoopback":false]
        case "embedded": endpoint = ["kind":"macHost","serviceIdentifier":agent.serviceIdentifier,"signingTeam":agent.signingTeam]
        case "mixed": endpoint["baseURL"] = "https://fixture.invalid"
        case "bad-agent": endpoint["agent"] = ["serviceIdentifier":agent.serviceIdentifier,"signingTeam":agent.signingTeam,"appGroup":"invalid"]
        default: endpoint.removeValue(forKey:"agent")
        }
        record["endpoint"] = endpoint
        let invalid = try JSONSerialization.data(withJSONObject:record)
        try invalid.write(to:file)
        #expect(throws:(any Error).self) { try storage.connection() }
        #expect(try Data(contentsOf:file) == invalid)
    }
}

@Test func macAgentAddressDecodingValidatesPinsAndRefusesUnknownFields() throws {
    for input in [
        #"{"serviceIdentifier":"io.fort.agent","signingTeam":"bad","appGroup":"group.io.fort.synthetic"}"#,
        #"{"serviceIdentifier":"io.fort.agent","signingTeam":"AAAAAAAAAA","appGroup":"group.io.fort.synthetic","machServiceName":"another.service"}"#,
        #"{"serviceIdentifier":"io.fort.agent","signingTeam":"AAAAAAAAAA","appGroup":"group.io.fort.synthetic","future":true}"#
    ] { #expect(throws:(any Error).self) { try JSONDecoder().decode(MacAgentEndpoint.self,from:Data(input.utf8)) } }
}
