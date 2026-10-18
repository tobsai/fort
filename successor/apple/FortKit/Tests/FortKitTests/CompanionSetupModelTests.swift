import Foundation
import Testing
@testable import FortKit

private func modelPhoneToken(_ byte: UInt8) -> String {
    Data(repeating:byte,count:32).base64EncodedString().replacingOccurrences(of:"+",with:"-").replacingOccurrences(of:"/",with:"_").replacingOccurrences(of:"=",with:"")
}
private final class ModelPhoneCredentials: DeviceCredentials {
    var values: [String:String] = [:]
    var generations = 0
    func save(_ connection: DeviceConnection, token: String) throws { values[connection.deviceID] = token }
    func token(for connection: DeviceConnection) throws -> String {
        guard let token = values[connection.deviceID] else { throw DeviceStorageError.keychain(-25300) }; return token
    }
    func remove(_ connection: DeviceConnection) throws { values.removeValue(forKey:connection.deviceID) }
}
private actor ModelPhoneTransport: FortTransport {
    var requests: [FortRequest] = []
    private var device: String?, token: String?
    private var holdJoin = false, loseReply = true
    private var continuation: CheckedContinuation<Void,Never>?
    func holdSuccessfulJoin() { holdJoin = true; loseReply = false }
    func waitUntilHeld() async throws {
        for _ in 0..<500 { if continuation != nil { return }; try await Task.sleep(for:.milliseconds(1)) }
        throw ClientError.invalidResponse
    }
    func release() { holdJoin = false; continuation?.resume(); continuation = nil }
    func send(_ request: FortRequest) async throws -> FortResponse {
        requests.append(request)
        let body: String
        switch request.path {
        case "/v1/devices/join":
            let input = try JSONDecoder().decode([String:String].self,from:request.body ?? Data())
            #expect(input["invitationId"] == "invite" && input["code"] == modelPhoneToken(1))
            #expect(input["deviceName"] == "Synthetic phone" && request.installationID == "phone-host")
            let joinedDevice = try #require(input["deviceId"]); device = joinedDevice; token = try #require(input["deviceToken"])
            if holdJoin { await withCheckedContinuation { continuation = $0 } }
            if loseReply { throw URLError(.networkConnectionLost) }
            body = "{\"installationId\":\"phone-host\",\"deviceId\":\"\(joinedDevice)\",\"expiresAt\":null,\"role\":\"companion\"}"
        default:
            #expect(request.installationID == "phone-host" && request.token == token)
            let device = try #require(device)
            switch request.path {
            case "/v1/installation": body = #"{"contractVersion":"1.0","installationId":"phone-host","recoveryEpoch":"epoch","mode":"local","serviceName":"Synthetic host","capabilities":["ownerPairing","companionPairing"]}"#
            case "/v1/pairing": body = "{\"deviceId\":\"\(device)\",\"mode\":\"owner-local\",\"expiresAt\":null,\"role\":\"companion\"}"
            case "/v1/agents": body = #"{"agents":[]}"#
            case "/v1/conversations": body = #"{"conversations":[]}"#
            case "/v1/skills": body = #"{"skills":[]}"#
            case "/v1/pairing/leave": body = "{\"installationId\":\"phone-host\",\"deviceId\":\"\(device)\",\"revoked\":true,\"scope\":\"companion-only\"}"
            default: throw ClientError.invalidResponse
            }
        }
        return FortResponse(status:200,body:Data(body.utf8))
    }
}
@MainActor private final class PhoneModelFixture {
    let directory: URL, storage: DeviceStorage
    let credentials = ModelPhoneCredentials(), transport = ModelPhoneTransport()
    init() throws {
        directory = FileManager.default.temporaryDirectory.appendingPathComponent("fort-phone-model-" + UUID().uuidString)
        storage = try DeviceStorage(directory:directory,credentials:credentials)
    }
    func model() -> MessengerModel {
        let coordinator = CompanionSetupCoordinator(storage:storage,tokenGenerator:{ self.credentials.generations += 1; return modelPhoneToken(2) },transport:{ [transport] _ in transport })
        return MessengerModel(storage:storage,session:.shared,companionSetup:coordinator,connectionTransport:{ [transport] _ in transport })
    }
    func configure(_ model: MessengerModel) {
        model.companionAddress = "https://phone-model.invalid"; model.companionInstallationID = "phone-host"
        model.companionInvitationID = "invite"; model.companionInvitationCode = modelPhoneToken(1); model.companionDeviceName = "Synthetic phone"
    }
    func close() { try? FileManager.default.removeItem(at:directory) }
}

@Test @MainActor func phoneSetupModelRestoresPendingWithoutNetworkAndRecoversOnlyOnExplicitAction() async throws {
    let fixture = try PhoneModelFixture(); defer { fixture.close() }
    let first = fixture.model(); fixture.configure(first)
    first.inspectCompanionSetup()
    #expect(await fixture.transport.requests.isEmpty && fixture.credentials.generations == 0)
    await first.connectCompanion()
    #expect(first.pendingCompanionSetup != nil)
    #expect(first.companionInvitationCode.isEmpty)
    #expect(try fixture.storage.connection() == nil)
    let requests = await fixture.transport.requests.count
    let restored = fixture.model(); await restored.restore()
    #expect(restored.pendingCompanionSetup == first.pendingCompanionSetup)
    #expect(restored.companionDeviceName == "Synthetic phone")
    #expect(await fixture.transport.requests.count == requests)
    await restored.recoverCompanionSetup()
    #expect(restored.connected && restored.isCompanionPairing)
    #expect(restored.pendingCompanionSetup == nil)
    #expect(!restored.canControlHost && !restored.canManageProviderAccount)
    #expect(fixture.credentials.generations == 1)
    #expect(await fixture.transport.requests.filter { $0.path == "/v1/devices/join" }.count == 1)
}

@Test(arguments:[false,true]) @MainActor
func phoneSetupModelSeparatesConfirmedPendingRemovalFromLocalForgetting(forget: Bool) async throws {
    let fixture = try PhoneModelFixture(); defer { fixture.close() }
    let model = fixture.model(); fixture.configure(model); await model.connectCompanion()
    let pending = try #require(model.pendingCompanionSetup), before = await fixture.transport.requests.count
    if forget { model.forgetPendingCompanion(pending) }
    else { await model.disconnectPendingCompanion(pending) }
    #expect(model.pendingCompanionSetup == nil)
    #expect(fixture.credentials.values.isEmpty)
    #expect(try fixture.storage.pairingRemoval()?.state == (forget ? .locallyForgotten : .confirmed))
    #expect(try fixture.storage.pairingRemoval()?.scope == .companion)
    #expect(await fixture.transport.requests.count == before + (forget ? 0 : 1))
    #expect(model.companionInvitationCode.isEmpty)
    model.companionInvitationCode = modelPhoneToken(1)
    #expect(model.canSubmitCompanionInvitation == !forget)
}

@Test @MainActor func phoneSetupModelRefusesAStaleForgetConfirmation() async throws {
    let fixture = try PhoneModelFixture(); defer { fixture.close() }
    let model = fixture.model(); fixture.configure(model); await model.connectCompanion()
    let pending = try #require(model.pendingCompanionSetup), before = await fixture.transport.requests.count
    let file = fixture.directory.appendingPathComponent("companion-setup.json")
    var value = try #require(JSONSerialization.jsonObject(with:Data(contentsOf:file)) as? [String:Any])
    value["deviceName"] = "Changed phone"; try JSONSerialization.data(withJSONObject:value).write(to:file)
    model.forgetPendingCompanion(pending)
    #expect(fixture.credentials.values.count == 1)
    #expect(await fixture.transport.requests.count == before)
    #expect(try fixture.storage.pairingRemoval() == nil)
    #expect(model.pendingCompanionSetup?.deviceName == "Changed phone")
}

@Test @MainActor func phoneSetupModelNeverAdoptsAForeignConnectionAfterALateJoin() async throws {
    let fixture = try PhoneModelFixture(); defer { fixture.close() }
    await fixture.transport.holdSuccessfulJoin()
    let model = fixture.model(); fixture.configure(model)
    let request = Task { await model.connectCompanion() }; try await fixture.transport.waitUntilHeld()
    let foreign = try DeviceConnection(endpoint:.network(baseURL:URL(string:"https://different.invalid")!,fixtureLoopback:false),installationID:"foreign",deviceID:"foreign-phone",role:.companion)
    try fixture.storage.save(foreign,token:modelPhoneToken(3))
    await fixture.transport.release(); await request.value
    #expect(!model.connected)
    #expect(try fixture.storage.connection() == foreign)
    #expect(await fixture.transport.requests.count == 1)
    #expect(fixture.credentials.values.count == 2)
}

@Test @MainActor func phoneSetupModelKeepsPriorHistoryButClearsItsDraftWhenConnectingElsewhere() async throws {
    let fixture = try PhoneModelFixture(); defer { fixture.close() }
    let old = try DeviceConnection(endpoint:.network(baseURL:URL(string:"https://old-phone.invalid")!,fixtureLoopback:false),installationID:"old-host",deviceID:"old-phone",role:.companion)
    try fixture.storage.save(old,token:modelPhoneToken(3))
    let installation = Installation(contractVersion:"1.0",installationId:"old-host",recoveryEpoch:"old-epoch",mode:"local",serviceName:"Earlier synthetic host")
    let agent = Agent(agentId:"old-agent",name:"Earlier agent",homeConversationId:"old-home",binding:Binding(bindingRevisionId:"old-binding",provider:"synthetic",runtimeVersion:"fixture",selectedModel:"synthetic-model",hostId:"old-host",enabled:true),readiness:"ready")
    let navigation = SavedNavigation(installation:installation,agents:[agent],selectedAgentID:agent.id)
    try fixture.storage.saveNavigation(navigation); try fixture.storage.removePairing(old)
    let model = fixture.model(); await model.restore()
    #expect(model.installation?.installationId == "old-host" && model.selectedAgentID == agent.id)
    model.draft = "Unsent text for the prior conversation"
    fixture.configure(model); await model.connectCompanion(); await model.recoverCompanionSetup()
    #expect(model.installation?.installationId == "phone-host")
    #expect(model.selectedAgentID == nil && model.draft.isEmpty)
    #expect(try fixture.storage.navigation(installationID:"old-host")?.agents == [agent])
}

#if DEBUG && os(macOS)
@Test @MainActor func phoneSetupFixtureReopensPendingAndOffersExpiredLeaveWithoutHostControls() async throws {
    let directory = FileManager.default.temporaryDirectory.appendingPathComponent("FortPhoneQA-" + UUID().uuidString)
    defer { try? FileManager.default.removeItem(at:directory) }
    let model = try await CompanionSetupFixture.make(directory:directory)
    #expect(model.canSubmitCompanionInvitation && model.pendingCompanionSetup == nil)
    await model.connectCompanion()
    let pending = try #require(model.pendingCompanionSetup)
    let reopened = try await CompanionSetupFixture.make(directory:directory)
    #expect(reopened.pendingCompanionSetup == pending && reopened.companionInvitationCode.isEmpty)
    await reopened.recoverCompanionSetup()
    #expect(reopened.connected && reopened.isCompanionPairing && reopened.canDisconnectPairing)
    let expired = try await CompanionSetupFixture.make(directory:directory,expired:true)
    await expired.refreshPairing()
    #expect(!expired.connected && expired.canDisconnectPairing)
    #expect(reopened.canEditPairingExpiry && !expired.canEditPairingExpiry)
    #expect(expired.pairingMessage == "Phone access could not be verified. You can still disconnect this exact phone.")
    await expired.setPairingExpiry(days:7)
    #expect(expired.pairingMessage == "Phone access could not be verified. You can still disconnect this exact phone.")
    #expect(!expired.canControlHost && !expired.canManageProviderAccount)
    await expired.stopLocalHost(disconnect:true)
    #expect(!expired.pairingRemovalPending && !expired.pairingCleanupPending)
    #expect(!expired.canDisconnectPairing)
}
#endif

@Test(.enabled(if:ProcessInfo.processInfo.environment["FORT_NATIVE_PHONE_CONTROLS_BENCHMARK"] != nil))
@MainActor func phoneSetupModelMeetsPreparationRecoveryBudget() async throws {
    var samples: [Double] = []
    for index in 0..<25 {
        let fixture = try PhoneModelFixture(); defer { fixture.close() }
        let model = fixture.model(); fixture.configure(model)
        let started = ContinuousClock.now
        model.inspectCompanionSetup(); await model.connectCompanion()
        let reopened = fixture.model(); await reopened.restore(); await reopened.recoverCompanionSetup()
        #expect(reopened.connected && fixture.credentials.generations == 1)
        #expect(await fixture.transport.requests.filter { $0.path == "/v1/devices/join" }.count == 1)
        let elapsed = started.duration(to:.now).components
        if index >= 5 { samples.append(Double(elapsed.seconds) * 1000 + Double(elapsed.attoseconds) / 1e15) }
    }
    samples.sort(); #expect(samples[18] < 100)
    let result: [String:Any] = ["warmups":5,"samples":samples,"p50":samples[9],"p95":samples[18],"maximum":samples[19],"budgetMs":100]
    let destination = try #require(ProcessInfo.processInfo.environment["FORT_NATIVE_PHONE_CONTROLS_BENCHMARK"])
    try JSONSerialization.data(withJSONObject:result,options:.sortedKeys).write(to:URL(fileURLWithPath:destination),options:.withoutOverwriting)
}

#if DEBUG
@Test @MainActor func legacyEnrollmentFixtureRequiresAnExplicitIsolatedUITestDirectory() throws {
    let root = FileManager.default.temporaryDirectory.appendingPathComponent("fort-ui-model-" + UUID().uuidString)
    defer { try? FileManager.default.removeItem(at:root) }
    #expect(throws:ClientError.self) { try MessengerModel(fixtureDirectory:root) }
    #expect(!FileManager.default.fileExists(atPath:root.path))
    let directory = root.appendingPathComponent("UITests/" + UUID().uuidString)
    let model = try MessengerModel(fixtureDirectory:directory)
    #expect(!model.canPrepareCompanion && !model.canSubmitCompanionInvitation)
}
#endif
