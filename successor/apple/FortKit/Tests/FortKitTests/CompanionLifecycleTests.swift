import Foundation
import Testing
@testable import FortKit

private actor CompanionTransport: FortTransport {
    var paths: [String] = []
    var reply = #"{"deviceId":"phone","mode":"owner-local","expiresAt":null,"role":"companion"}"#
    var lostReplies = 0
    var leaveReply = #"{"installationId":"one","deviceId":"phone","revoked":true,"scope":"companion-only"}"#
    var holdLeave = false
    private var leaveContinuation: CheckedContinuation<Void,Never>?
    func hold() { holdLeave = true }
    func waitUntilHeld() async throws {
        for _ in 0..<500 {
            if leaveContinuation != nil { return }
            try await Task.sleep(for:.milliseconds(1))
        }
        throw ClientError.invalidResponse
    }
    func release() { holdLeave = false; leaveContinuation?.resume(); leaveContinuation = nil }
    func configure(lostReplies: Int = 0, leaveReply: String? = nil, pairing: String? = nil) {
        self.lostReplies = lostReplies
        if let leaveReply { self.leaveReply = leaveReply }
        if let pairing { reply = pairing }
    }
    func send(_ request: FortRequest) async throws -> FortResponse {
        paths.append(request.path)
        let body: String
        switch request.path {
        case "/v1/installation": body = #"{"contractVersion":"1.0","installationId":"one","recoveryEpoch":"epoch","mode":"fixture","serviceName":"Synthetic","capabilities":["ownerPairing","companionPairing","runtimeRecovery"]}"#
        case "/v1/agents": body = #"{"agents":[]}"#
        case "/v1/runtime": body = #"{"state":"stopped","canRestart":true,"detail":"Synthetic paused host"}"#
        case "/v1/pairing": body = reply
        case "/v1/pairing/leave":
            #expect(request.token == "synthetic-phone")
            #expect(try JSONDecoder().decode([String:String].self,from:request.body ?? Data()) == ["deviceId":"phone"])
            if holdLeave { await withCheckedContinuation { leaveContinuation = $0 } }
            if lostReplies > 0 { lostReplies -= 1; throw URLError(.networkConnectionLost) }
            body = leaveReply
        case "/v1/pairing/disconnect": body = #"{"runtimeStopped":true,"revoked":true}"#
        case "/v1/host/stop": body = #"{"runtimeStopped":true,"revoked":false}"#
        case "/v1/runtime/restart": body = #"{"state":"ready","canRestart":false,"detail":"Synthetic"}"#
        default: throw ClientError.invalidResponse
        }
        return FortResponse(status:200,body:Data(body.utf8))
    }
}

#if os(macOS)
extension CompanionTransport: OwnerSetupControl {
    func beginOwnerSetup(timeout: TimeInterval) throws -> OwnerSetupState { throw ClientError.invalidResponse }
    func completeOwnerSetup(_ request: OwnerSetupRequest, timeout: TimeInterval) throws -> OwnerSetupReceipt { throw ClientError.invalidResponse }
}
private final class CompanionCredentials: DeviceCredentials {
    var values: [String:String] = [:]
    var removed: [DeviceConnection] = []
    var failRemoval = false
    func save(_ connection: DeviceConnection, token: String) throws { values[connection.deviceID] = token }
    func token(for connection: DeviceConnection) throws -> String {
        guard let token = values[connection.deviceID] else { throw DeviceStorageError.keychain(-25300) }
        return token
    }
    func remove(_ connection: DeviceConnection) throws {
        if failRemoval { throw URLError(.cannotRemoveFile) }
        values.removeValue(forKey:connection.deviceID); removed.append(connection)
    }
}
@MainActor private final class CompanionFixture {
    let directory: URL
    let credentials = CompanionCredentials()
    let transport = CompanionTransport()
    let storage: DeviceStorage
    let connection: DeviceConnection
    init() throws {
        directory = FileManager.default.temporaryDirectory.appendingPathComponent("fort-companion-native-" + UUID().uuidString)
        storage = try DeviceStorage(directory:directory,credentials:credentials)
        let endpoint = DeviceEndpoint.macAgent(try MacAgentEndpoint(serviceIdentifier:"io.fort.synthetic.companion",signingTeam:"AAAAAAAAAA",appGroup:"group.io.fort.synthetic"))
        connection = try DeviceConnection(endpoint:endpoint,installationID:"one",deviceID:"phone")
        try storage.save(connection,token:"synthetic-phone")
    }
    func model() throws -> MessengerModel {
        try MessengerModel(storage:storage,session:.shared,ownerEndpoint:connection.endpoint,ownerSetupControl:transport,connectionTransport:{ [transport] _ in transport })
    }
    func close() { try? FileManager.default.removeItem(at:directory) }
}

@Test @MainActor func nativeCompanionDisconnectLeavesOnlyThePhoneAndKeepsHistory() async throws {
    let fixture = try CompanionFixture(); defer { fixture.close() }
    let model = try fixture.model(); await model.restore(); await model.refreshPairing()
    await model.stopLocalHost(disconnect:true)
    #expect(await fixture.transport.paths.contains("/v1/pairing/leave"))
    #expect(await !fixture.transport.paths.contains("/v1/pairing/disconnect"))
    #expect(fixture.credentials.removed == [fixture.connection])
    #expect(try fixture.storage.connection() == nil)
    #expect(try fixture.storage.navigation(installationID:"one") != nil)
    #expect(!model.pairingMessage.contains("shutdown"))
}

@Test @MainActor func nativeCompanionLostLeaveReplyRestoresItsExactRemovalScope() async throws {
    let fixture = try CompanionFixture(); defer { fixture.close() }
    await fixture.transport.configure(lostReplies:1)
    let first = try fixture.model(); await first.restore(); await first.refreshPairing()
    await first.stopLocalHost(disconnect:true)
    #expect(first.pairingRemovalPending && fixture.credentials.removed.isEmpty)
    let reopened = try fixture.model(); await reopened.restore(); await reopened.retryPairingRemoval()
    #expect(try fixture.storage.connection() == nil)
    #expect(fixture.credentials.removed == [fixture.connection])
    #expect(await fixture.transport.paths.filter { $0 == "/v1/pairing/leave" }.count == 2)
    #expect(await !fixture.transport.paths.contains("/v1/pairing/disconnect"))
    #expect(!reopened.pairingMessage.contains("shutdown"))
}

@Test(arguments:["companion","invalid"]) @MainActor func nativeCompanionOrUnknownRoleCannotControlTheHost(role: String) async throws {
    let fixture = try CompanionFixture(); defer { fixture.close() }
    if role == "invalid" { await fixture.transport.configure(pairing:#"{"deviceId":"phone","mode":"owner-local","role":"unknown","expiresAt":null}"#) }
    let model = try fixture.model(); await model.restore()
    #expect(!model.canControlHost && !model.canResumeLocalWork)
    await model.stopLocalHost(disconnect:false); await model.restartRuntime()
    let paths = await fixture.transport.paths
    #expect(!paths.contains("/v1/host/stop") && !paths.contains("/v1/runtime/restart"))
    #expect(fixture.credentials.removed.isEmpty)
    if role == "invalid" {
        await model.stopLocalHost(disconnect:true)
        #expect(await !fixture.transport.paths.contains("/v1/pairing/leave"))
        #expect(try fixture.storage.pairingRemoval() == nil)
    }
}

@Test(arguments:["installationId","deviceId","revoked","scope"]) @MainActor func nativeCompanionRejectsEveryForeignLeaveReceipt(field: String) async throws {
    let fixture = try CompanionFixture(); defer { fixture.close() }
    var receipt: [String:Any] = ["installationId":"one","deviceId":"phone","revoked":true,"scope":"companion-only"]
    receipt[field] = field == "revoked" ? false : "foreign"
    let body = String(decoding:try JSONSerialization.data(withJSONObject:receipt),as:UTF8.self)
    await fixture.transport.configure(leaveReply:body)
    let model = try fixture.model(); await model.restore(); await model.stopLocalHost(disconnect:true)
    #expect(model.pairingRemovalPending && fixture.credentials.removed.isEmpty)
    #expect(try fixture.storage.connection() == fixture.connection)
    #expect(try fixture.storage.pairingRemoval()?.state == .requested)
}

@Test @MainActor func nativeCompanionConfirmedCleanupPreservesScopeAndAllowsSeparateSavedHistoryDeletion() async throws {
    let fixture = try CompanionFixture(); defer { fixture.close() }
    let model = try fixture.model(); await model.restore(); fixture.credentials.failRemoval = true
    await model.stopLocalHost(disconnect:true)
    #expect(model.pairingCleanupPending)
    #expect(try fixture.storage.pairingRemoval()?.state == .confirmed)
    #expect(try fixture.storage.pairingRemoval()?.scope == .companion)
    let paths = await fixture.transport.paths
    let reopened = try fixture.model(); await reopened.restore(); fixture.credentials.failRemoval = false
    reopened.finishPairingRemoval()
    #expect(await fixture.transport.paths == paths)
    #expect(reopened.canDeleteSavedHistory)
    reopened.deleteSavedHistory(fixture.connection)
    #expect(reopened.pairingMessage.contains("were deleted"))
    #expect(try fixture.storage.navigation(installationID:"one") == nil)
    #expect(try fixture.storage.pairingRemoval()?.scope == .companion)
}

@Test @MainActor func nativeCompanionLocalForgetCannotClaimOrAttemptHostRecovery() async throws {
    let fixture = try CompanionFixture(); defer { fixture.close() }
    await fixture.transport.configure(lostReplies:1)
    let model = try fixture.model(); await model.restore(); await model.stopLocalHost(disconnect:true)
    model.forgetLocalPairing()
    #expect(model.localPairingForgotten)
    #expect(try fixture.storage.pairingRemoval()?.scope == .companion)
    let paths = await fixture.transport.paths
    let reopened = try fixture.model(); await reopened.restore(); await reopened.retryPairingRemoval()
    #expect(await fixture.transport.paths == paths)
    #expect(reopened.pairingRemovalPending && !reopened.canDeleteSavedHistory)
    #expect(reopened.pairingMessage.contains("owning Mac"))
}

@Test(arguments:["connection","intent"]) @MainActor func nativeCompanionLateReplyCannotRemoveChangedLocalState(change: String) async throws {
    let fixture = try CompanionFixture(); defer { fixture.close() }
    let model = try fixture.model(); await model.restore(); await fixture.transport.hold()
    let action = Task { await model.stopLocalHost(disconnect:true) }
    try await fixture.transport.waitUntilHeld()
    let other = try DeviceConnection(endpoint:fixture.connection.endpoint,installationID:"other",deviceID:"replacement")
    if change == "connection" { try fixture.storage.save(other,token:"synthetic-replacement") }
    else {
        let changed = PairingRemovalIntent(connection:fixture.connection,state:.locallyForgotten,scope:.companion)
        try JSONEncoder().encode(changed).write(to:fixture.directory.appendingPathComponent("pairing-removal.json"))
    }
    await fixture.transport.release(); await action.value
    #expect(try fixture.storage.connection() == (change == "connection" ? other : fixture.connection))
    #expect(fixture.credentials.removed.isEmpty)
    #expect(model.pairingRemovalPending)
}

@Test(.enabled(if:ProcessInfo.processInfo.environment["FORT_NATIVE_COMPANION_BENCHMARK"] != nil)) @MainActor
func nativeCompanionLostReplyRecoveryMeetsItsIsolatedBudget() async throws {
    let destination = try #require(ProcessInfo.processInfo.environment["FORT_NATIVE_COMPANION_BENCHMARK"])
    var samples: [Double] = []
    for index in 0..<25 {
        let fixture = try CompanionFixture(); defer { fixture.close() }
        await fixture.transport.configure(lostReplies:1)
        let first = try fixture.model(); await first.restore()
        let started = ProcessInfo.processInfo.systemUptime
        await first.stopLocalHost(disconnect:true)
        let restored = try fixture.model(); await restored.restore(); await restored.retryPairingRemoval()
        let elapsed = (ProcessInfo.processInfo.systemUptime - started) * 1000
        #expect(try fixture.storage.connection() == nil)
        #expect(fixture.credentials.removed == [fixture.connection])
        if index >= 5 { samples.append(elapsed) }
    }
    let sorted = samples.sorted()
    let report: [String:Any] = ["warmups":5,"samplesMs":samples,"p50Ms":sorted[9],"p95Ms":sorted[18],"maximumMs":sorted[19],"boundary":"Synthetic leave, lost reply, relaunch and exact retry; private file journal, fake credential store, no network or provider"]
    try JSONSerialization.data(withJSONObject:report,options:[.prettyPrinted,.sortedKeys]).write(to:URL(fileURLWithPath:destination),options:.withoutOverwriting)
    #expect(sorted[18] < 100)
}
#endif

@Test func nativePairingRetainsCompanionAuthorityFromTheService() async throws {
    let transport = CompanionTransport()
    let client = FortClient(installationID:"one",token:"synthetic",transport:transport)
    let status = try await client.pairing()
    let encoded = try #require(JSONSerialization.jsonObject(with:JSONEncoder().encode(status)) as? [String:Any])
    #expect(encoded["role"] as? String == "companion")
    #expect(status.deviceId == "phone" && status.mode == .ownerLocal && status.expiresAt == nil)
}
