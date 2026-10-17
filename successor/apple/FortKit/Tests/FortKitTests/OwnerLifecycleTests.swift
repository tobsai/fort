import Foundation
import Testing
@testable import FortKit

private final class LifecycleCredentials: DeviceCredentials {
    var removed: [DeviceConnection] = []
    var reads = 0
    var failRemoval = false
    func save(_ connection: DeviceConnection, token: String) throws {}
    func token(for connection: DeviceConnection) throws -> String { reads += 1; return "fixture-only" }
    func remove(_ connection: DeviceConnection) throws {
        if failRemoval { throw URLError(.cannotRemoveFile) }
        removed.append(connection)
    }
}
@Test @MainActor func disconnectRemovesOnlyExactCredentialAndRestoresHistoryWithoutConnecting() async throws {
    let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
    defer { try? FileManager.default.removeItem(at:root) }
    let credentials = LifecycleCredentials(), storage = try DeviceStorage(directory:root,credentials:credentials)
    let connection = DeviceConnection(baseURL:try #require(URL(string:"https://fixture.invalid")),installationID:"one",deviceID:"device",fixtureLoopback:false)
    try storage.save(connection,token:"fixture-only")
    let other = DeviceConnection(baseURL:connection.baseURL,installationID:"two",deviceID:"device",fixtureLoopback:false)
    #expect(throws:(any Error).self) { try storage.removePairing(other) }
    #expect(credentials.removed.isEmpty)
    let installation = Installation(contractVersion:"1.0",installationId:"one",recoveryEpoch:"epoch",mode:"openai",serviceName:"Fort Prototype")
    try storage.saveNavigation(SavedNavigation(installation:installation,agents:[],selectedAgentID:nil))
    try storage.removePairing(connection)
    #expect(credentials.removed == [connection]); #expect(try storage.connection() == nil)
    #expect(try storage.disconnectedConnection() == connection)
    #expect(try storage.navigation(installationID:"one")?.installation == installation)
    let model = MessengerModel(storage:storage,session:.shared); await model.restore()
    #expect(model.installation == installation); #expect(!model.supportsCommands)
    #expect(credentials.reads == 0); #expect(!model.showConnection)
}
@Test func failedCredentialRemovalKeepsConnectionForRecovery() throws {
    let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
    defer { try? FileManager.default.removeItem(at:root) }
    let credentials = LifecycleCredentials(), storage = try DeviceStorage(directory:root,credentials:credentials)
    let connection = DeviceConnection(baseURL:try #require(URL(string:"https://fixture.invalid")),installationID:"one",deviceID:"device",fixtureLoopback:false)
    try storage.save(connection,token:"fixture-only"); credentials.failRemoval = true
    #expect(throws:(any Error).self) { try storage.removePairing(connection) }
    #expect(try storage.connection() == connection)
}
@MainActor private final class RemovalFixture: OwnedMacInstallation {
    let appURL = URL(fileURLWithPath:"/synthetic/Fort.app")
    var operations: [String] = []
    var stopped = false
    func stopAndRevoke() async throws -> OwnedHostStop { operations.append("stop"); return OwnedHostStop(hostStopped:stopped,pairingRevoked:stopped) }
    func removeApp() throws { operations.append("app") }
    func removeHistory() throws { operations.append("history") }
}
@Test @MainActor func uninstallRequiresOwnedHostProofAndKeepsHistoryByDefault() async throws {
    let installation = RemovalFixture()
    let removal = OwnerLifecycle(installation:installation,removeCredential:{ installation.operations.append("credential") })
    await #expect(throws:(any Error).self) { try await removal.uninstall() }
    #expect(installation.operations == ["stop"])
    installation.stopped = true; installation.operations = []
    try await removal.uninstall()
    #expect(installation.operations == ["stop","credential","app"])
    installation.operations = []
    try await removal.uninstall(history:.deleteConfirmed)
    #expect(installation.operations == ["stop","credential","app","history"])
}
@Test @MainActor func uninstallStopsBeforeRemovingAppWhenCredentialRemovalFails() async throws {
    let installation = RemovalFixture(); installation.stopped = true
    let removal = OwnerLifecycle(installation:installation,removeCredential:{ throw URLError(.cannotRemoveFile) })
    await #expect(throws:(any Error).self) { try await removal.uninstall() }
    #expect(installation.operations == ["stop"])
}
private final class LifecycleHTTPProtocol: URLProtocol, @unchecked Sendable {
    override class func canInit(with request: URLRequest) -> Bool { true }
    override class func canonicalRequest(for request: URLRequest) -> URLRequest { request }
    override func startLoading() {
        let path = request.url?.path ?? ""
        let body: String
        switch path {
        case "/v1/installation": body = #"{"contractVersion":"1.0","installationId":"one","recoveryEpoch":"epoch","mode":"openai","serviceName":"Fort Prototype","capabilities":["ownerPairing"]}"#
        case "/v1/agents": body = #"{"agents":[]}"#
        case "/v1/conversations": body = #"{"conversations":[]}"#
        case "/v1/skills": body = #"{"skills":[]}"#
        case "/v1/pairing", "/v1/pairing/expiry": body = #"{"deviceId":"device","mode":"owner-local","expiresAt":null}"#
        case "/v1/pairing/disconnect": body = #"{"runtimeStopped":true,"revoked":true}"#
        case "/v1/host/stop": body = #"{"runtimeStopped":false,"revoked":false}"#
        default: client?.urlProtocol(self,didFailWithError:URLError(.unsupportedURL)); return
        }
        guard let url = request.url, let response = HTTPURLResponse(url:url,statusCode:200,httpVersion:nil,headerFields:["Content-Type":"application/json"]) else { return }
        client?.urlProtocol(self,didReceive:response,cacheStoragePolicy:.notAllowed)
        client?.urlProtocol(self,didLoad:Data(body.utf8)); client?.urlProtocolDidFinishLoading(self)
    }
    override func stopLoading() {}
}
@Test @MainActor func nativeLifecycleRejectsUnconfirmedStopAndRetriesOnlyLocalCredentialCleanupAfterRelaunch() async throws {
    let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
    defer { try? FileManager.default.removeItem(at:root) }
    let credentials = LifecycleCredentials(), storage = try DeviceStorage(directory:root,credentials:credentials)
    let connection = DeviceConnection(baseURL:try #require(URL(string:"https://fixture.invalid")),installationID:"one",deviceID:"device",fixtureLoopback:false)
    try storage.save(connection,token:"fixture-only")
    let config = URLSessionConfiguration.ephemeral; config.protocolClasses = [LifecycleHTTPProtocol.self]
    let session = URLSession(configuration:config); defer { session.invalidateAndCancel() }
    let model = MessengerModel(storage:storage,session:session); await model.restore(); await model.refreshPairing()
    #expect(model.pairingStatus?.expiresAt == nil); #expect(model.canManagePairing)
    await model.setPairingExpiry(days:nil); #expect(model.pairingMessage == "Pairing expiry saved.")
    await model.stopLocalHost(disconnect:false)
    #expect(credentials.removed.isEmpty); #expect(model.canManagePairing)
    credentials.failRemoval = true; await model.stopLocalHost(disconnect:true)
    #expect(model.pairingCleanupPending); #expect(!model.supportsCommands)
    let reads = credentials.reads
    let relaunched = MessengerModel(storage:storage,session:session); await relaunched.restore()
    #expect(relaunched.pairingCleanupPending); #expect(credentials.reads == reads)
    #expect(!relaunched.canManagePairing); #expect(!relaunched.supportsCommands)
    credentials.failRemoval = false; relaunched.finishPairingRemoval()
    #expect(!relaunched.pairingCleanupPending); #expect(try storage.connection() == nil)
    #expect(credentials.removed == [connection]); #expect(try storage.navigation(installationID:"one") != nil)
}
