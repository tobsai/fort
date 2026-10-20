import Foundation
import Testing
@testable import FortKit

#if os(macOS)
private actor AccountTransport: FortTransport, OwnerSetupControl {
    let directory: URL
    var paths: [String] = []
    var requestBodies: [Data] = []
    var lostStarts = 0, lostCancels = 0, companion = false, holdStart = false, holdSelection = false
    private var held: CheckedContinuation<Void,Never>?
    private var modelsEnabled = false, holdModels = false, modelsWaiting = false
    private var heldModels: CheckedContinuation<Void,Never>?
    func enableModels(hold: Bool = false) { modelsEnabled = true; holdModels = hold }
    func waitForModels() async throws {
        for _ in 0..<500 { if modelsWaiting { return }; try await Task.sleep(for:.milliseconds(1)) }
        throw ClientError.invalidResponse
    }
    func releaseModels() { holdModels = false; heldModels?.resume(); heldModels = nil }
    func configure(lostStarts: Int = 0, lostCancels: Int = 0, companion: Bool = false, holdStart: Bool = false, holdSelection: Bool = false) {
        self.lostStarts = lostStarts; self.lostCancels = lostCancels; self.companion = companion; self.holdStart = holdStart; self.holdSelection = holdSelection
    }
    func waitUntilHeld() async throws {
        for _ in 0..<500 { if held != nil { return }; try await Task.sleep(for:.milliseconds(1)) }
        throw ClientError.invalidResponse
    }
    func release() { holdStart = false; holdSelection = false; held?.resume(); held = nil }
    func setStatus(_ value: String) { current = value }
    var current = #"{"state":"idle","revision":0}"#
    init(directory: URL) { self.directory = directory }
    func beginOwnerSetup(timeout: TimeInterval) throws -> OwnerSetupState { throw ClientError.invalidResponse }
    func completeOwnerSetup(_ request: OwnerSetupRequest, timeout: TimeInterval) throws -> OwnerSetupReceipt { throw ClientError.invalidResponse }
    func send(_ request: FortRequest) async throws -> FortResponse {
        paths.append(request.path)
        let body: String
        switch request.path {
        case "/v1/installation":
            body = modelsEnabled
                ? #"{"contractVersion":"1.0","installationId":"one","recoveryEpoch":"epoch","mode":"fixture","serviceName":"Synthetic","capabilities":["ownerPairing","companionPairing","accountSignIn","modelCatalog"],"runtime":{"provider":"independent-account","name":"Synthetic account runtime","runtimeVersion":"fixture-7","hostId":"fixture-host","session":"stateless","capabilities":["modelCatalog"]}}"#
                : #"{"contractVersion":"1.0","installationId":"one","recoveryEpoch":"epoch","mode":"fixture","serviceName":"Synthetic","capabilities":["ownerPairing","companionPairing","accountSignIn"]}"#
        case "/v1/models":
            let selected = (try JSONSerialization.jsonObject(with:Data(current.utf8)) as? [String:Any])?["selected"] as? Bool == true
            if holdModels { modelsWaiting = true; await withCheckedContinuation { heldModels = $0 } }
            body = selected
                ? #"{"provider":"independent-account","state":"ready","models":[{"id":"exact-synthetic","name":"Exact synthetic","availability":"available"}]}"#
                : #"{"provider":"independent-account","state":"unavailable","models":[],"detail":"Select the reported provider account."}"#
        case "/v1/agents": body = #"{"agents":[]}"#
        case "/v1/pairing": body = companion ? #"{"deviceId":"mac","mode":"owner-local","expiresAt":null,"role":"companion"}"# : #"{"deviceId":"mac","mode":"owner-local","expiresAt":null}"#
        case "/v1/account/sign-in": body = current
        case "/v1/account/sign-in/start":
            let data = try #require(request.body)
            requestBodies.append(data)
            #expect(FileManager.default.fileExists(atPath:directory.appendingPathComponent("account-sign-in.json").path))
            let input = try #require(JSONSerialization.jsonObject(with:data) as? [String:Any])
            let reply: [String:Any] = ["state":"challenge", "revision":(try #require(input["expectedRevision"] as? Int)) + 1, "operationId":try #require(input["operationId"]), "expiresAt":try #require(input["expiresAt"]),
                "challenge":["kind":"deviceCode","verificationURL":"https://device.fixture.invalid/verify","userCode":"FORT-TEST"]]
            current = String(decoding:try JSONSerialization.data(withJSONObject:reply),as:UTF8.self)
            if holdStart { await withCheckedContinuation { held = $0 } }
            if lostStarts > 0 { lostStarts -= 1; throw URLError(.networkConnectionLost) }
            body = current
        case "/v1/account/sign-in/select":
            if holdSelection { await withCheckedContinuation { held = $0 } }
            let data = try #require(request.body), input = try #require(JSONSerialization.jsonObject(with:data) as? [String:Any])
            requestBodies.append(data)
            var reply = try #require(JSONSerialization.jsonObject(with:Data(current.utf8)) as? [String:Any])
            let result = try #require(reply["result"] as? [String:Any])
            #expect(input["operationId"] as? String == reply["operationId"] as? String)
            #expect(input["revision"] as? Int == reply["revision"] as? Int)
            #expect(input["recoveryEpoch"] as? String == "epoch")
            #expect(NSDictionary(dictionary:try #require(input["account"] as? [String:Any])) == NSDictionary(dictionary:try #require(result["account"] as? [String:Any])))
            reply["selected"] = true
            current = String(decoding:try JSONSerialization.data(withJSONObject:reply),as:UTF8.self); body = current
        case "/v1/account/sign-in/cancel":
            let data = try #require(request.body), input = try #require(JSONSerialization.jsonObject(with:data) as? [String:Any])
            var reply = try #require(JSONSerialization.jsonObject(with:Data(current.utf8)) as? [String:Any])
            #expect(input["operationId"] as? String == reply["operationId"] as? String)
            #expect(input["revision"] as? Int == reply["revision"] as? Int)
            reply.removeValue(forKey:"challenge"); reply["state"] = "finished"; reply["result"] = ["kind":"canceled"]
            current = String(decoding:try JSONSerialization.data(withJSONObject:reply),as:UTF8.self)
            if lostCancels > 0 { lostCancels -= 1; throw URLError(.networkConnectionLost) }; body = current
        case "/v1/host/stop": body = #"{"runtimeStopped":true,"revoked":false}"#
        case "/v1/pairing/disconnect": body = #"{"runtimeStopped":true,"revoked":true}"#
        default: throw ClientError.invalidResponse
        }
        return FortResponse(status:200,body:Data(body.utf8))
    }
}
private final class AccountCredentials: DeviceCredentials {
    func save(_ connection: DeviceConnection, token: String) throws {}
    func token(for connection: DeviceConnection) throws -> String { "synthetic-mac-token" }
    func remove(_ connection: DeviceConnection) throws {}
}

@Test @MainActor func modelSelectionLoadsCatalogAfterExplicitAccountConfirmation() async throws {
    let fixture = try AccountFixture(); defer { fixture.close() }
    await fixture.transport.enableModels()
    await fixture.transport.setStatus(#"{"state":"finished","revision":1,"operationId":"synthetic-complete","expiresAt":1900000000000,"result":{"kind":"signedIn","account":{"accountId":"workspace-fixture","userLabel":"owner@fixture.invalid"}},"selected":false}"#)
    let model = try fixture.model(); await model.restore(); await model.refreshModels()
    #expect(model.accountSignInStatus?.accountSelected == false)
    #expect(model.modelCatalog.state == "unavailable" && !model.modelCatalog.canSelect("exact-synthetic"))
    model.draft = "Keep my unsent message."
    await model.selectReportedAccount()
    #expect(model.accountSignInStatus?.accountSelected == true)
    #expect(model.modelCatalog.canSelect("exact-synthetic"))
    #expect(model.modelCatalog.models.map(\.id) == ["exact-synthetic"])
    #expect(!model.loadingModels && model.draft == "Keep my unsent message.")
    #expect(await !fixture.transport.paths.contains("/v1/account/sign-in/start"))
    #expect(await fixture.transport.paths.filter { $0 == "/v1/account/sign-in/select" }.count == 1)
}

@Test @MainActor func modelSelectionWaitsForDiscoveryBeforeConfirmingAccount() async throws {
    let fixture = try AccountFixture(); defer { fixture.close() }
    await fixture.transport.enableModels(hold:true)
    await fixture.transport.setStatus(#"{"state":"finished","revision":1,"operationId":"synthetic-complete","expiresAt":1900000000000,"result":{"kind":"signedIn","account":{"accountId":"workspace-fixture","userLabel":"owner@fixture.invalid"}},"selected":false}"#)
    let model = try fixture.model(); await model.restore()
    let discovery = Task { await model.refreshModels() }; try await fixture.transport.waitForModels()
    #expect(model.loadingModels)
    await model.selectReportedAccount()
    #expect(model.accountSignInStatus?.accountSelected == false)
    #expect(await !fixture.transport.paths.contains("/v1/account/sign-in/select"))
    await fixture.transport.releaseModels(); await discovery.value
    await model.selectReportedAccount()
    #expect(model.accountSignInStatus?.accountSelected == true && model.modelCatalog.canSelect("exact-synthetic"))
}
@Test @MainActor func modelSelectionDefersDiscoveryUntilAccountConfirmationCompletes() async throws {
    let fixture = try AccountFixture(); defer { fixture.close() }
    await fixture.transport.enableModels()
    await fixture.transport.configure(holdSelection:true)
    await fixture.transport.setStatus(#"{"state":"finished","revision":1,"operationId":"synthetic-complete","expiresAt":1900000000000,"result":{"kind":"signedIn","account":{"accountId":"workspace-fixture","userLabel":"owner@fixture.invalid"}},"selected":false}"#)
    let model = try fixture.model(); await model.restore()
    let confirmation = Task { await model.selectReportedAccount() }; try await fixture.transport.waitUntilHeld()
    await model.refreshModels()
    #expect(await !fixture.transport.paths.contains("/v1/models"))
    await fixture.transport.release(); await confirmation.value
    #expect(model.accountSignInStatus?.accountSelected == true && model.modelCatalog.canSelect("exact-synthetic"))
}
@MainActor private final class AccountFixture {
    let directory: URL, storage: DeviceStorage, connection: DeviceConnection, transport: AccountTransport
    init() throws {
        directory = FileManager.default.temporaryDirectory.appendingPathComponent("fort-account-native-" + UUID().uuidString)
        storage = try DeviceStorage(directory:directory,credentials:AccountCredentials())
        let endpoint = DeviceEndpoint.macAgent(try MacAgentEndpoint(serviceIdentifier:"io.fort.synthetic.account",signingTeam:"AAAAAAAAAA",appGroup:"group.io.fort.synthetic"))
        connection = try DeviceConnection(endpoint:endpoint,installationID:"one",deviceID:"mac")
        try storage.save(connection,token:"synthetic-mac-token")
        transport = AccountTransport(directory:directory)
    }
    func model() throws -> MessengerModel {
        try MessengerModel(storage:storage,session:.shared,ownerEndpoint:connection.endpoint,ownerSetupControl:transport,connectionTransport:{ [transport] _ in transport })
    }
    func close() { try? FileManager.default.removeItem(at:directory) }
}

@Test @MainActor func nativeAccountSelectionIsExplicitAndStatusRestoresWithoutSelectingAgain() async throws {
    let fixture = try AccountFixture(); defer { fixture.close() }
    let model = try fixture.model(); await model.restore()
    await fixture.transport.setStatus(#"{"state":"finished","revision":1,"operationId":"synthetic-complete","expiresAt":1900000000000,"result":{"kind":"signedIn","account":{"accountId":"workspace-fixture","userLabel":"owner@fixture.invalid"}},"selected":false}"#)
    await model.refreshAccountSignIn()
    #expect(model.accountSignInStatus?.reportedAccount?.userLabel == "owner@fixture.invalid")
    #expect(model.accountSignInStatus?.accountSelected == false)
    #expect(await !fixture.transport.paths.contains("/v1/account/sign-in/select"))
    await model.selectReportedAccount()
    #expect(model.accountSignInStatus?.accountSelected == true)
    #expect(model.accountSignInMessage.contains("must match"))
    let restored = try fixture.model(); await restored.restore(); await restored.refreshAccountSignIn()
    #expect(restored.accountSignInStatus?.accountSelected == true)
    #expect(await fixture.transport.paths.filter { $0 == "/v1/account/sign-in/select" }.count == 1)
}

@Test func nativeAccountIdentityDecoderRefusesMissingMalformedOrSecretFields() throws {
    for account in [#"{"accountId":"workspace"}"#, #"{"accountId":"workspace","userLabel":null}"#, #"{"accountId":"","userLabel":"owner@fixture.invalid"}"#, #"{"accountId":"workspace","userLabel":"owner\n@fixture.invalid"}"#, #"{"accountId":"workspace","userLabel":"owner@fixture.invalid","token":"synthetic"}"#] {
        let text = #"{"state":"finished","revision":1,"operationId":"synthetic","expiresAt":1900000000000,"result":{"kind":"signedIn","account":\#(account)},"selected":true}"#
        #expect(throws: Error.self) { try JSONDecoder().decode(AccountSignInStatus.self,from:Data(text.utf8)) }
    }
}

@Test @MainActor func nativeAccountSignedInExplicitlyLeavesIdentityUnverified() async throws {
    let fixture = try AccountFixture(); defer { fixture.close() }
    let model = try fixture.model(); await model.restore()
    await fixture.transport.setStatus(#"{"state":"finished","revision":1,"operationId":"synthetic-complete","expiresAt":1900000000000,"result":{"kind":"signedIn"}}"#)
    await model.refreshAccountSignIn()
    #expect(model.accountSignInStatus?.finishedResult == .signedIn)
    #expect(model.accountSignInMessage.contains("account identity is not verified"))
    #expect(await !fixture.transport.paths.contains("/v1/account/sign-in/start"))
}

@Test @MainActor func nativeAccountStartPersistsItsExactIntentBeforeRequestAndRestoreOnlyReadsStatus() async throws {
    let fixture = try AccountFixture(); defer { fixture.close() }
    let model = try fixture.model(); await model.restore(); await model.refreshPairing()
    #expect(await !fixture.transport.paths.contains("/v1/account/sign-in/start"))
    await model.startAccountSignIn()
    #expect(model.accountSignInStatus?.challenge?.userCode == "FORT-TEST")
    let intent = try String(contentsOf:fixture.directory.appendingPathComponent("account-sign-in.json"),encoding:.utf8)
    #expect(!intent.contains("FORT-TEST") && !intent.contains("device.fixture") && !intent.contains("synthetic-mac-token"))
    let restored = try fixture.model(); await restored.restore()
    #expect(restored.accountSignInStatus?.challenge?.userCode == "FORT-TEST")
    #expect(await fixture.transport.paths.filter { $0 == "/v1/account/sign-in/start" }.count == 1)
}
@Test @MainActor func nativeAccountLostReplyRetriesTheExactSavedRequestAndCancelReconcilesOnRestore() async throws {
    let fixture = try AccountFixture(); defer { fixture.close() }
    await fixture.transport.configure(lostStarts:1,lostCancels:1)
    let model = try fixture.model(); await model.restore(); await model.startAccountSignIn()
    #expect(model.accountSignInStatus == nil && model.hasPendingAccountSignIn)
    let restored = try fixture.model(); await restored.restore()
    #expect(restored.accountSignInStatus?.challenge?.userCode == "FORT-TEST")
    #expect(await fixture.transport.requestBodies.count == 1)
    await restored.retryAccountSignIn()
    let bodies = await fixture.transport.requestBodies
    #expect(bodies.count == 2)
    #expect(try JSONDecoder().decode(AccountSignInRequest.self,from:bodies[0]) == JSONDecoder().decode(AccountSignInRequest.self,from:bodies[1]))
    await restored.cancelAccountSignIn()
    #expect(restored.accountSignInStatus == nil && restored.hasPendingAccountSignIn)
    let again = try fixture.model(); await again.restore()
    #expect(again.accountSignInStatus?.finishedResult == .canceled && !again.hasPendingAccountSignIn)
    #expect(!FileManager.default.fileExists(atPath:fixture.directory.appendingPathComponent("account-sign-in.json").path))
}

@Test @MainActor func nativeAccountCompanionCannotReadOrStartProviderLogin() async throws {
    let fixture = try AccountFixture(); defer { fixture.close() }
    await fixture.transport.configure(companion:true)
    let model = try fixture.model(); await model.restore(); await model.startAccountSignIn(); await model.cancelAccountSignIn()
    #expect(!model.canManageProviderAccount)
    #expect(await !fixture.transport.paths.contains(where:{ $0.hasPrefix("/v1/account/") }))
}

@Test @MainActor func nativeAccountLateReplyCannotExposeCodeAfterConnectionChanges() async throws {
    let fixture = try AccountFixture(); defer { fixture.close() }
    await fixture.transport.configure(holdStart:true)
    let model = try fixture.model(); await model.restore()
    let operation = Task { await model.startAccountSignIn() }; try await fixture.transport.waitUntilHeld()
    let changed = try DeviceConnection(endpoint:fixture.connection.endpoint,installationID:"one",deviceID:"replacement")
    try fixture.storage.save(changed,token:"synthetic-other")
    await fixture.transport.release(); await operation.value
    #expect(model.accountSignInStatus == nil)
    #expect(await fixture.transport.requestBodies.count == 1)
}

@Test @MainActor func nativeAccountExpiredUnacceptedRequestNeedsExplicitDiscardAndNeverExtendsTime() async throws {
    let fixture = try AccountFixture(); defer { fixture.close() }
    let request = AccountSignInRequest(operationId:"expired",expectedRevision:0,recoveryEpoch:"epoch",expiresAt:Int64(Date().timeIntervalSince1970 * 1000) - 6000)
    try fixture.storage.prepareAccountSignIn(AccountSignInIntent(format:1,connection:fixture.connection,request:request))
    let model = try fixture.model(); await model.restore(); await model.retryAccountSignIn()
    #expect(model.hasPendingAccountSignIn && model.accountSignInStatus == nil)
    #expect(await fixture.transport.requestBodies.isEmpty)
    await model.discardExpiredAccountSignIn()
    #expect(!model.hasPendingAccountSignIn && model.accountSignInStatus == .idle(0))
    #expect(await fixture.transport.requestBodies.isEmpty)
}

@Test @MainActor func nativeAccountIntentRefusesAdditionalFieldsAndKeepsEvidence() async throws {
    let fixture = try AccountFixture(); defer { fixture.close() }
    let model = try fixture.model(); await model.restore(); await model.startAccountSignIn()
    let path = fixture.directory.appendingPathComponent("account-sign-in.json")
    var record = try #require(JSONSerialization.jsonObject(with:Data(contentsOf:path)) as? [String:Any])
    record["token"] = "synthetic-unexpected"
    try JSONSerialization.data(withJSONObject:record).write(to:path)
    let restored = try fixture.model(); await restored.restore(); await restored.retryAccountSignIn()
    #expect(restored.accountSignInStatus == nil)
    #expect(await fixture.transport.requestBodies.count == 1)
    #expect(try String(contentsOf:path,encoding:.utf8).contains("synthetic-unexpected"))
}

@Test @MainActor func nativeAccountConfirmedDisconnectRetiresOnlyItsExactPendingIntent() async throws {
    let fixture = try AccountFixture(); defer { fixture.close() }
    let model = try fixture.model(); await model.restore(); await model.startAccountSignIn()
    await model.stopLocalHost(disconnect:true)
    #expect(model.accountSignInStatus == nil)
    #expect(!FileManager.default.fileExists(atPath:fixture.directory.appendingPathComponent("account-sign-in.json").path))
    #expect(try fixture.storage.connection() == nil)
}

@Test func nativeAccountDecoderRejectsForeignStatesExtraFieldsAndUnsafeVerificationURLs() throws {
    let identity: [String:Any] = ["state":"challenge","revision":1,"operationId":"synthetic","expiresAt":1_900_000_000_000]
    for url in ["http://device.fixture.invalid/", "https://secret@device.fixture.invalid/", "https://device.fixture.invalid/?token=hidden", "https://device.fixture.invalid/#hidden", "https://device.fixture.invalid:443/"] {
        let value = identity.merging(["challenge":["kind":"deviceCode","verificationURL":url,"userCode":"FORT-TEST"]]) { _,new in new }
        #expect(throws:(any Error).self) { try JSONDecoder().decode(AccountSignInStatus.self,from:JSONSerialization.data(withJSONObject:value)) }
    }
    for value: [String:Any] in [["state":"foreign","revision":0], ["state":"idle","revision":0,"token":"unexpected"], ["state":"idle","revision":-1]] {
        #expect(throws:(any Error).self) { try JSONDecoder().decode(AccountSignInStatus.self,from:JSONSerialization.data(withJSONObject:value)) }
    }
}

@Test @MainActor func nativeAccountProtectedIntentRefusesLinksAndCompetingRemoval() async throws {
    for kind in ["symlink","hardlink","removal","companion"] {
        let fixture = try AccountFixture(); defer { fixture.close() }
        let target = fixture.directory.appendingPathComponent("synthetic-retained"), path = fixture.directory.appendingPathComponent("account-sign-in.json")
        try Data("synthetic retained evidence".utf8).write(to:target)
        try FileManager.default.setAttributes([.posixPermissions:0o600],ofItemAtPath:target.path)
        if kind == "symlink" { try FileManager.default.createSymbolicLink(at:path,withDestinationURL:target) }
        else if kind == "hardlink" { try FileManager.default.linkItem(at:target,to:path) }
        else if kind == "companion" {
            let files = try OwnerSetupFiles(directory:fixture.directory)
            try files.write(Data("{}".utf8),to:.companionSetup); files.close()
        } else { try fixture.storage.beginPairingRemoval(fixture.connection) }
        let model = try fixture.model(); await model.restore(); await model.startAccountSignIn()
        #expect(await fixture.transport.requestBodies.isEmpty)
        #expect(try String(contentsOf:target,encoding:.utf8) == "synthetic retained evidence")
    }
}

@Test @MainActor func nativeAccountRepairedPairingAllowsOnlyConfirmedHistoricalRemoval() async throws {
    for confirmed in [true,false] {
        let fixture = try AccountFixture(); defer { fixture.close() }
        let old = try DeviceConnection(endpoint:fixture.connection.endpoint,installationID:"one",deviceID:"prior-device")
        try fixture.storage.save(old,token:"synthetic-old")
        try fixture.storage.removePairing(old,revocationConfirmed:confirmed)
        try fixture.storage.save(fixture.connection,token:"synthetic-mac-token")
        let model = try fixture.model(); await model.restore(); await model.startAccountSignIn()
        #expect((model.accountSignInStatus?.challenge != nil) == confirmed)
        #expect(await fixture.transport.requestBodies.count == (confirmed ? 1 : 0))
        #expect(try fixture.storage.pairingRemoval()?.connection == old)
    }
}

@Test @MainActor func nativeAccountLateReplyCannotReplaceChangedSavedIntent() async throws {
    let fixture = try AccountFixture(); defer { fixture.close() }
    await fixture.transport.configure(holdStart:true)
    let model = try fixture.model(); await model.restore()
    let pending = Task { await model.startAccountSignIn() }; try await fixture.transport.waitUntilHeld()
    let original = try #require(try fixture.storage.accountSignInIntent(for:fixture.connection))
    try fixture.storage.clearAccountSignIn(original)
    let changed = AccountSignInIntent(format:1,connection:fixture.connection,request:AccountSignInRequest(operationId:"replacement",expectedRevision:1,recoveryEpoch:"epoch",expiresAt:original.request.expiresAt))
    try fixture.storage.prepareAccountSignIn(changed)
    await fixture.transport.release(); await pending.value
    #expect(model.accountSignInStatus == nil)
    #expect(try fixture.storage.accountSignInIntent(for:fixture.connection) == changed)
}

@Test @MainActor func nativeAccountLateChallengeStaysHiddenAfterStoppingLocalWork() async throws {
    let fixture = try AccountFixture(); defer { fixture.close() }
    await fixture.transport.configure(holdStart:true)
    let model = try fixture.model(); await model.restore()
    let pending = Task { await model.startAccountSignIn() }; try await fixture.transport.waitUntilHeld()
    await model.stopLocalHost(disconnect:false)
    await fixture.transport.release(); await pending.value
    #expect(model.accountSignInStatus == nil)
    #expect(model.hasPendingAccountSignIn)
    #expect(model.pairingMessage.contains("Local work stopped"))
}

@Test @MainActor func nativeAccountExpiredChallengeIsNeverPresented() async throws {
    let fixture = try AccountFixture(); defer { fixture.close() }
    let expires = Int64(Date().timeIntervalSince1970 * 1000) - 1
    let record: [String:Any] = ["state":"challenge","revision":1,"operationId":"expired","expiresAt":expires,
        "challenge":["kind":"deviceCode","verificationURL":"https://device.fixture.invalid/verify","userCode":"FORT-TEST"]]
    await fixture.transport.setStatus(String(decoding:try JSONSerialization.data(withJSONObject:record),as:UTF8.self))
    let model = try fixture.model(); await model.restore()
    #expect(model.accountSignInStatus?.challenge == nil)
    #expect(model.accountSignInMessage.contains("expired"))
    #expect(await fixture.transport.requestBodies.isEmpty)
}

@Test(.enabled(if:ProcessInfo.processInfo.environment["FORT_NATIVE_ACCOUNT_BENCHMARK"] != nil))
@MainActor func nativeAccountJourneysMeetTheirIsolatedBudget() async throws {
    let fixture = try AccountFixture(); defer { fixture.close() }
    let model = try fixture.model(); await model.restore(); var samples: [Double] = []
    for index in 0..<25 {
        let before = ContinuousClock.now
        await model.startAccountSignIn(); #expect(model.accountSignInStatus?.challenge?.userCode == "FORT-TEST")
        await model.cancelAccountSignIn(); #expect(model.accountSignInStatus?.finishedResult == .canceled)
        let duration = before.duration(to:.now).components
        if index >= 5 { samples.append(Double(duration.seconds) * 1000 + Double(duration.attoseconds) / 1e15) }
    }
    samples.sort(); #expect(samples[18] < 100)
    let result: [String:Any] = ["samples":samples,"warmups":5,"p50":samples[9],"p95":samples[18],"maximum":samples[19],"budgetMs":100]
    let path = try #require(ProcessInfo.processInfo.environment["FORT_NATIVE_ACCOUNT_BENCHMARK"])
    try JSONSerialization.data(withJSONObject:result,options:.sortedKeys).write(to:URL(fileURLWithPath:path),options:.withoutOverwriting)
}
#endif
