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
    var revoked = true
    var unregistered = true
    var failHistory = false
    var wrongScope = false
    func stopAndRevoke() async throws -> OwnedHostStop { operations.append("stop"); return OwnedHostStop(hostStopped:stopped,pairingRevoked:revoked,startupUnregistered:unregistered) }
    func recoverPairingRemoval(_ connection: DeviceConnection) async throws -> PairingRecoveryProof {
        operations.append("recover")
        let scope = wrongScope ? DeviceConnection(baseURL:connection.baseURL,installationID:"another-installation",deviceID:connection.deviceID,fixtureLoopback:connection.fixtureLoopback) : connection
        return PairingRecoveryProof(connection:scope,hostStopped:stopped,pairingRevoked:stopped)
    }
    func removeHistory() throws { operations.append("history"); if failHistory { throw URLError(.cannotRemoveFile) } }
}
@Test @MainActor func removalPreparationRequiresOwnedHostProofAndKeepsHistoryByDefault() async throws {
    let installation = RemovalFixture()
    let removal = OwnerLifecycle(installation:installation,removeCredential:{ installation.operations.append("credential") })
    await #expect(throws:(any Error).self) { try await removal.prepareForRemoval() }
    #expect(installation.operations == ["stop"])
    installation.stopped = true; installation.operations = []
    let kept = try await removal.prepareForRemoval()
    #expect(kept.appURL == installation.appURL); #expect(kept.history == .keep)
    #expect(installation.operations == ["stop","credential"])
    installation.operations = []
    let deleted = try await removal.prepareForRemoval(history:.deleteConfirmed)
    #expect(deleted.appURL == installation.appURL); #expect(deleted.history == .deleteConfirmed)
    #expect(installation.operations == ["stop","credential","history"])
}
@Test @MainActor func removalPreparationFailsWhenCredentialRemovalFails() async throws {
    let installation = RemovalFixture(); installation.stopped = true
    let removal = OwnerLifecycle(installation:installation,removeCredential:{ throw URLError(.cannotRemoveFile) })
    await #expect(throws:(any Error).self) { try await removal.prepareForRemoval(history:.deleteConfirmed) }
    #expect(installation.operations == ["stop"])
}
@Test @MainActor func removalPreparationRequiresRevocationAndStartupUnregistration() async throws {
    let installation = RemovalFixture(); installation.stopped = true
    let removal = OwnerLifecycle(installation:installation,removeCredential:{ installation.operations.append("credential") })
    installation.revoked = false
    await #expect(throws:(any Error).self) { try await removal.prepareForRemoval() }
    #expect(installation.operations == ["stop"])
    installation.revoked = true; installation.unregistered = false; installation.operations = []
    await #expect(throws:(any Error).self) { try await removal.prepareForRemoval() }
    #expect(installation.operations == ["stop"])
    installation.unregistered = true; installation.operations = []
    #expect(try await removal.prepareForRemoval().history == .keep)
    #expect(installation.operations == ["stop","credential"])
}
@Test @MainActor func removalPreparationRetainsAppHandoffUntilHistoryCleanupSucceeds() async throws {
    let installation = RemovalFixture(); installation.stopped = true; installation.failHistory = true
    let removal = OwnerLifecycle(installation:installation,removeCredential:{ installation.operations.append("credential") })
    await #expect(throws:(any Error).self) { try await removal.prepareForRemoval(history:.deleteConfirmed) }
    #expect(installation.operations == ["stop","credential","history"])
    installation.failHistory = false; installation.operations = []
    #expect(try await removal.prepareForRemoval(history:.deleteConfirmed).history == .deleteConfirmed)
    #expect(installation.operations == ["stop","credential","history"])
}
private final class LifecycleHTTPProtocol: URLProtocol, @unchecked Sendable {
    override class func canInit(with request: URLRequest) -> Bool { true }
    override class func canonicalRequest(for request: URLRequest) -> URLRequest { request }
    override func startLoading() {
        let path = request.url?.path ?? ""
        if path == "/v1/pairing/disconnect", request.url?.host == "lost.fixture.invalid" {
            client?.urlProtocol(self,didFailWithError:URLError(.networkConnectionLost)); return
        }
        if path == "/v1/pairing/disconnect", request.url?.host == "expired.fixture.invalid", let url = request.url, let response = HTTPURLResponse(url:url,statusCode:401,httpVersion:nil,headerFields:nil) {
            client?.urlProtocol(self,didReceive:response,cacheStoragePolicy:.notAllowed)
            client?.urlProtocol(self,didLoad:Data(#"{"error":{"code":"unauthorized","message":"Fixture credential expired"}}"#.utf8))
            client?.urlProtocolDidFinishLoading(self); return
        }
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

@Test @MainActor func lostDisconnectReplyAndExpiredCredentialRetainIntentWithoutClaimingRevocation() async throws {
    for host in ["lost.fixture.invalid","expired.fixture.invalid"] {
        let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        defer { try? FileManager.default.removeItem(at:root) }
        let credentials = LifecycleCredentials(), storage = try DeviceStorage(directory:root,credentials:credentials)
        let connection = DeviceConnection(baseURL:try #require(URL(string:"https://" + host)),installationID:"one",deviceID:"device",fixtureLoopback:false)
        try storage.save(connection,token:"fixture-only")
        let config = URLSessionConfiguration.ephemeral; config.protocolClasses = [LifecycleHTTPProtocol.self]
        let session = URLSession(configuration:config); defer { session.invalidateAndCancel() }
        let model = MessengerModel(storage:storage,session:session); await model.restore()
        await model.stopLocalHost(disconnect:true)
        #expect(try storage.pairingRemoval()?.state == .requested)
        #expect(credentials.removed.isEmpty); #expect(model.pairingRemovalPending); #expect(!model.supportsCommands)
        let reads = credentials.reads
        let restored = MessengerModel(storage:storage,session:session); await restored.restore()
        #expect(credentials.reads == reads); #expect(restored.pairingRemovalPending)
        restored.forgetLocalPairing()
        #expect(try storage.connection() == nil); #expect(try storage.pairingRemoval()?.state == .locallyForgotten)
        #expect(restored.pairingMessage.contains("unconfirmed")); #expect(credentials.removed == [connection])
        let host = RemovalFixture()
        let recovery = MessengerModel(storage:storage,session:session,ownedInstallation:host); await recovery.restore()
        await recovery.retryPairingRemoval()
        #expect(try storage.pairingRemoval()?.state == .locallyForgotten)
        host.stopped = true; host.wrongScope = true; await recovery.retryPairingRemoval()
        #expect(try storage.pairingRemoval()?.state == .locallyForgotten)
        host.wrongScope = false; await recovery.retryPairingRemoval()
        #expect(try storage.pairingRemoval()?.state == .confirmed)
        #expect(host.operations == ["recover","recover","recover"])
        #expect(try storage.navigation(installationID:"one") != nil)
    }
}

@Test @MainActor func nativeRemovalPreparationReportsSystemHandoffOnlyAfterConfirmedCleanup() async throws {
    let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
    defer { try? FileManager.default.removeItem(at:root) }
    let credentials = LifecycleCredentials(), storage = try DeviceStorage(directory:root,credentials:credentials)
    let connection = DeviceConnection(baseURL:try #require(URL(string:"https://fixture.invalid")),installationID:"one",deviceID:"device",fixtureLoopback:false)
    try storage.save(connection,token:"fixture-only")
    let config = URLSessionConfiguration.ephemeral; config.protocolClasses = [LifecycleHTTPProtocol.self]
    let session = URLSession(configuration:config); defer { session.invalidateAndCancel() }
    let installation = RemovalFixture(); installation.stopped = true; installation.unregistered = false
    let model = MessengerModel(storage:storage,session:session,ownedInstallation:installation)
    await model.restore(); await model.prepareForRemoval()
    #expect(model.preparedAppRemoval == nil); #expect(credentials.removed.isEmpty)
    #expect(model.pairingMessage.contains("Startup unregistration was not confirmed"))
    installation.unregistered = true; credentials.failRemoval = true
    await model.prepareForRemoval()
    #expect(model.preparedAppRemoval == nil); #expect(model.pairingCleanupPending)
    credentials.failRemoval = false
    await model.prepareForRemoval()
    #expect(model.preparedAppRemoval?.appURL == installation.appURL)
    #expect(model.preparedAppRemoval?.history == .keep)
    #expect(try storage.connection() == nil); #expect(try storage.disconnectedConnection() == connection)
    #expect(model.pairingMessage.contains("Remove the Fort app using TestFlight or macOS."))
    #expect(model.pairingMessage.contains("History is kept.")); #expect(!model.supportsCommands)
    installation.failHistory = true
    await model.prepareForRemoval(history:.deleteConfirmed)
    #expect(model.preparedAppRemoval == nil)
    installation.failHistory = false
    await model.prepareForRemoval(history:.deleteConfirmed)
    #expect(model.preparedAppRemoval?.history == .deleteConfirmed)
    #expect(model.pairingMessage.contains("The confirmed local history was deleted."))
}
