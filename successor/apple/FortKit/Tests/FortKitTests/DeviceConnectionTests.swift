import Foundation
import Testing
@testable import FortKit

private final class ConnectionCredentials: DeviceCredentials {
    var saved: [DeviceConnection] = []
    var removed: [DeviceConnection] = []
    var reads = 0
    func save(_ connection: DeviceConnection, token: String) throws { saved.append(connection) }
    func token(for connection: DeviceConnection) throws -> String { reads += 1; return "synthetic" }
    func remove(_ connection: DeviceConnection) throws { removed.append(connection) }
}

@Test func legacyConnectionDecodesWithoutRewritingOrReadingCredentials() throws {
    let directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
    defer { try? FileManager.default.removeItem(at:directory) }
    let credentials = ConnectionCredentials(), storage = try DeviceStorage(directory:directory,credentials:credentials)
    let bytes = Data(#"{"baseURL":"https://fixture.invalid/base","installationID":"one","deviceID":"device","fixtureLoopback":false}"#.utf8)
    let file = directory.appendingPathComponent("connection.json"); try bytes.write(to:file)
    let savedConnection = try storage.connection()
    let connection = try #require(savedConnection)
    #expect(connection.endpoint == .network(baseURL:URL(string:"https://fixture.invalid/base")!,fixtureLoopback:false))
    #expect(connection.installationID == "one" && connection.deviceID == "device")
    #expect(try Data(contentsOf:file) == bytes)
    #expect(credentials.saved.isEmpty && credentials.removed.isEmpty && credentials.reads == 0)
    let current = try JSONEncoder().encode(connection)
    #expect(try JSONDecoder().decode(DeviceConnection.self,from:current) == connection)
    let record = try #require(JSONSerialization.jsonObject(with:current) as? [String:Any])
    #expect(record["schemaVersion"] as? Int == 2 && record["baseURL"] == nil && record["endpoint"] != nil)
}

@Test @MainActor func localHostConnectionRetainsIdentityAndDisconnectedHistoryWithoutCredentialAccess() async throws {
    let directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
    defer { try? FileManager.default.removeItem(at:directory) }
    let credentials = ConnectionCredentials(), storage = try DeviceStorage(directory:directory,credentials:credentials)
    let connection = try DeviceConnection(endpoint:.macHost(serviceIdentifier:"io.fort.synthetic.host",signingTeam:"AAAAAAAAAA"),installationID:"one",deviceID:"device")
    try storage.save(connection,token:"synthetic")
    #expect(try storage.connection() == connection); #expect(credentials.saved == [connection])
    let installation = Installation(contractVersion:"1.0",installationId:"one",recoveryEpoch:"epoch",mode:"local",serviceName:"Fort")
    try storage.saveNavigation(SavedNavigation(installation:installation,agents:[],selectedAgentID:nil))
    try storage.removePairing(connection)
    #expect(try storage.disconnectedConnection() == connection)
    #expect(try storage.pairingRemoval()?.connection == connection)
    #expect(credentials.removed == [connection])
    let model = MessengerModel(storage:storage,session:.shared); await model.restore()
    #expect(model.installation == installation && !model.connected && !model.showConnection)
    #expect(model.endpoint == "This Mac")
    #expect(credentials.reads == 0)
}

@Test func connectionDecoderRejectsUnknownMixedOrUnsafeFormats() throws {
    let bad = [
        #"{"schemaVersion":3,"endpoint":{"kind":"network","baseURL":"https://fixture.invalid","fixtureLoopback":false},"installationID":"one","deviceID":"device"}"#,
        #"{"schemaVersion":2,"endpoint":{"kind":"future"},"installationID":"one","deviceID":"device"}"#,
        #"{"schemaVersion":2,"baseURL":"https://fixture.invalid","endpoint":{"kind":"macHost","serviceIdentifier":"io.fort.host","signingTeam":"AAAAAAAAAA"},"installationID":"one","deviceID":"device"}"#,
        #"{"baseURL":"http://192.0.2.1","installationID":"one","deviceID":"device","fixtureLoopback":true}"#,
        #"{"baseURL":"https://user:password@fixture.invalid","installationID":"one","deviceID":"device","fixtureLoopback":false}"#,
        #"{"schemaVersion":2,"endpoint":{"kind":"macHost","serviceIdentifier":"invalid\" or true","signingTeam":"AAAAAAAAAA"},"installationID":"one","deviceID":"device"}"#,
        #"{"schemaVersion":2,"endpoint":{"kind":"macHost","serviceIdentifier":"io.fort.host","signingTeam":"bad"},"installationID":"one","deviceID":"device"}"#,
        #"{"schemaVersion":2,"endpoint":{"kind":"network","baseURL":"https://fixture.invalid","fixtureLoopback":false,"serviceIdentifier":"io.fort.host"},"installationID":"one","deviceID":"device"}"#
    ]
    for json in bad { #expect(throws:(any Error).self) { try JSONDecoder().decode(DeviceConnection.self,from:Data(json.utf8)) } }
}

@Test @MainActor func invalidConnectionCannotReadOrChangeCredentials() async throws {
    let directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
    defer { try? FileManager.default.removeItem(at:directory) }
    let credentials = ConnectionCredentials(), storage = try DeviceStorage(directory:directory,credentials:credentials)
    let invalid = DeviceConnection(baseURL:URL(string:"http://192.0.2.1")!,installationID:"one",deviceID:"device",fixtureLoopback:false)
    #expect(throws:(any Error).self) { try storage.save(invalid,token:"synthetic") }
    #expect(throws:(any Error).self) { try storage.token(for:invalid) }
    let bytes = Data(#"{"schemaVersion":999,"installationID":"one","deviceID":"device"}"#.utf8)
    let file = directory.appendingPathComponent("connection.json"); try bytes.write(to:file)
    let model = MessengerModel(storage:storage,session:.shared); await model.restore()
    #expect(!model.connected && credentials.reads == 0 && credentials.saved.isEmpty && credentials.removed.isEmpty)
    #expect(try Data(contentsOf:file) == bytes)
    for identity in ["","a/b","line\nbreak"] {
        #expect(throws:(any Error).self) { try DeviceConnection(endpoint:.macHost(serviceIdentifier:"io.fort.host",signingTeam:"AAAAAAAAAA"),installationID:identity,deviceID:"device") }
    }
}
