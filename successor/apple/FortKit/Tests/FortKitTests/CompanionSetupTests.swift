import Foundation
import Testing
@testable import FortKit

private func syntheticSecret(_ byte: UInt8) -> String {
    Data(repeating:byte,count:32).base64EncodedString().replacingOccurrences(of:"+",with:"-").replacingOccurrences(of:"/",with:"_").replacingOccurrences(of:"=",with:"")
}
private final class SetupCredentials: DeviceCredentials {
    var tokens: [String:String] = [:]
    var generations = 0
    var failRemoval = false
    func save(_ connection: DeviceConnection, token: String) throws { tokens[connection.deviceID] = token }
    func token(for connection: DeviceConnection) throws -> String {
        guard let token = tokens[connection.deviceID] else { throw DeviceStorageError.keychain(-25300) }
        return token
    }
    func remove(_ connection: DeviceConnection) throws {
        if failRemoval { throw URLError(.cannotRemoveFile) }
        tokens.removeValue(forKey:connection.deviceID)
    }
}
private actor JoiningTransport: FortTransport {
    var requests: [FortRequest] = []
    var joined: (deviceID: String, token: String)?
    var loseReply = true
    var revoked = false
    var loseLeave = false
    var denyReads = false
    var foreignJoinReceipt = false
    var recoveryChange: String?
    func allowJoinReply() { loseReply = false }
    func changeRecovery(_ value: String) { recoveryChange = value }
    func loseNextLeave() { loseLeave = true }
    func rejectReads() { denyReads = true }
    func changeJoinReceipt() { foreignJoinReceipt = true; loseReply = false }
    let beforeJoin: @MainActor @Sendable (String,String) throws -> Void
    init(beforeJoin: @escaping @MainActor @Sendable (String,String) throws -> Void) { self.beforeJoin = beforeJoin }
    func send(_ request: FortRequest) async throws -> FortResponse {
        requests.append(request)
        let body: String
        if request.path == "/v1/devices/join" {
            let value = try JSONDecoder().decode([String:String].self,from:request.body ?? Data())
            let deviceID = try #require(value["deviceId"]), token = try #require(value["deviceToken"])
            try await beforeJoin(deviceID,token)
            #expect(request.installationID == "one" && request.token == nil)
            #expect(value["invitationId"] == "invitation" && value["code"] == syntheticSecret(1))
            if let joined { #expect(joined.deviceID == deviceID && joined.token == token) }
            joined = (deviceID,token)
            if loseReply { loseReply = false; throw URLError(.networkConnectionLost) }
            body = "{\"installationId\":\"one\",\"deviceId\":\"\(foreignJoinReceipt ? "foreign" : deviceID)\",\"expiresAt\":null,\"role\":\"companion\"}"
        } else {
            let joined = try #require(joined)
            #expect(request.token == joined.token && request.installationID == "one")
            if request.path != "/v1/pairing/leave", denyReads || revoked {
                return FortResponse(status:401,body:Data(#"{"error":{"code":"unauthorized","message":"Synthetic refusal"}}"#.utf8))
            }
            switch request.path {
            case "/v1/installation": body = #"{"contractVersion":"1.0","installationId":"one","recoveryEpoch":"epoch","mode":"local","serviceName":"Fort","capabilities":["ownerPairing","companionPairing"]}"#.replacingOccurrences(of:"\"installationId\":\"one\"",with:recoveryChange == "installation" ? "\"installationId\":\"foreign\"" : "\"installationId\":\"one\"")
            case "/v1/pairing": body = "{\"deviceId\":\"\(recoveryChange == "device" ? "foreign" : joined.deviceID)\",\"expiresAt\":null,\"mode\":\"owner-local\",\"role\":\(recoveryChange == "role" ? "null" : "\"companion\"")}"
            case "/v1/pairing/leave":
                revoked = true
                if loseLeave { loseLeave = false; throw URLError(.networkConnectionLost) }
                body = "{\"installationId\":\"one\",\"deviceId\":\"\(joined.deviceID)\",\"revoked\":true,\"scope\":\"companion-only\"}"
            default: throw ClientError.invalidResponse
            }
        }
        return FortResponse(status:200,body:Data(body.utf8))
    }
}

@MainActor private final class PhoneSetupFixture {
    let root: URL
    let credentials = SetupCredentials()
    let storage: DeviceStorage
    let transport: JoiningTransport
    init() throws {
        root = FileManager.default.temporaryDirectory.appendingPathComponent("fort-phone-preparation-" + UUID().uuidString)
        storage = try DeviceStorage(directory:root,credentials:credentials)
        let credentials = credentials
        transport = JoiningTransport { deviceID,token in #expect(credentials.tokens[deviceID] == token) }
    }
    func coordinator() -> CompanionSetupCoordinator {
        CompanionSetupCoordinator(storage:storage,tokenGenerator:{ self.credentials.generations += 1; return syntheticSecret(2) },transport:{ [transport] _ in transport })
    }
    func connect() async throws -> DeviceConnection {
        try await coordinator().connect(address:URL(string:"https://fort.invalid")!,installationID:"one",invitationID:"invitation",code:syntheticSecret(1),deviceName:"Synthetic phone")
    }
    func close() { try? FileManager.default.removeItem(at:root) }
}

@Test(arguments:["missing","changed"]) @MainActor func preparedPhoneNeverReplacesAMissingOrChangedReadyCredential(change: String) async throws {
    let fixture = try PhoneSetupFixture(); defer { fixture.close() }
    await #expect(throws:URLError.self) { try await fixture.connect() }
    let pending = try #require(try fixture.coordinator().pendingSetup()), requests = await fixture.transport.requests.count
    if change == "missing" { fixture.credentials.tokens.removeAll() }
    else { fixture.credentials.tokens[pending.connection.deviceID] = syntheticSecret(3) }
    await #expect(throws:(any Error).self) { try await fixture.coordinator().recover() }
    await #expect(throws:(any Error).self) { try await fixture.connect() }
    #expect(fixture.credentials.generations == 1)
    #expect(await fixture.transport.requests.count == requests)
    #expect(try fixture.coordinator().pendingSetup() == pending)
    #expect(try fixture.storage.connection() == nil)
}

@Test @MainActor func preparedPhoneKeepsUncertaintyAfterDeniedRecoveryOrWrongJoinReceipt() async throws {
    let fixture = try PhoneSetupFixture(); defer { fixture.close() }
    await fixture.transport.changeJoinReceipt()
    await #expect(throws:OwnerLifecycleError.self) { try await fixture.connect() }
    await fixture.transport.rejectReads()
    await #expect(throws:ClientError.self) { try await fixture.coordinator().recover() }
    #expect(try fixture.coordinator().pendingSetup() != nil)
    #expect(try fixture.storage.connection() == nil)
    #expect(fixture.credentials.tokens.count == 1 && fixture.credentials.generations == 1)
}

@Test @MainActor func preparedPhoneCancellationRetriesTheSameLeaveThenOnlyLocalCleanup() async throws {
    let fixture = try PhoneSetupFixture(); defer { fixture.close() }
    await #expect(throws:URLError.self) { try await fixture.connect() }
    await fixture.transport.loseNextLeave()
    await #expect(throws:URLError.self) { try await fixture.coordinator().disconnectPending() }
    #expect(try fixture.coordinator().pendingSetup() != nil)
    #expect(fixture.credentials.tokens.count == 1)
    fixture.credentials.failRemoval = true
    await #expect(throws:URLError.self) { try await fixture.coordinator().disconnectPending() }
    let count = await fixture.transport.requests.count
    fixture.credentials.failRemoval = false
    try await fixture.coordinator().disconnectPending()
    #expect(await fixture.transport.requests.count == count)
    #expect(try fixture.coordinator().pendingSetup() == nil)
    #expect(fixture.credentials.tokens.isEmpty)
    #expect(try fixture.storage.pairingRemoval()?.state == .confirmed)
    #expect(try fixture.storage.pairingRemoval()?.scope == .companion)
    #expect(await !fixture.transport.requests.contains { $0.path == "/v1/pairing/disconnect" || $0.path == "/v1/host/stop" })
}

@Test @MainActor func preparedPhoneLocalForgettingKeepsRevocationUnconfirmedWithoutNetwork() async throws {
    let fixture = try PhoneSetupFixture(); defer { fixture.close() }
    await #expect(throws:URLError.self) { try await fixture.connect() }
    let count = await fixture.transport.requests.count
    try fixture.coordinator().forgetPending()
    #expect(await fixture.transport.requests.count == count)
    #expect(try fixture.coordinator().pendingSetup() == nil)
    #expect(fixture.credentials.tokens.isEmpty)
    #expect(try fixture.storage.pairingRemoval()?.state == .locallyForgotten)
    #expect(try fixture.storage.pairingRemoval()?.scope == .companion)
}

@Test(arguments:["owner-setup.json","host-removal.json","history-removal.json","account-sign-in.json","companion-link"])
@MainActor func preparedPhoneRefusesCompetingSetupAndPrivateRecordSubstitution(barrier: String) async throws {
    let fixture = try PhoneSetupFixture(); defer { fixture.close() }
    if barrier == "companion-link" {
        let target = fixture.root.appendingPathComponent("retained.json")
        try Data("retained synthetic data".utf8).write(to:target)
        try FileManager.default.createSymbolicLink(at:fixture.root.appendingPathComponent("companion-setup.json"),withDestinationURL:target)
    } else {
        let files = try OwnerSetupFiles(directory:fixture.root); defer { files.close() }
        let record = try #require(OwnerSetupFiles.Record(rawValue:barrier))
        try files.write(Data("{}".utf8),to:record)
    }
    await #expect(throws:(any Error).self) { try await fixture.connect() }
    #expect(fixture.credentials.generations == 0 && fixture.credentials.tokens.isEmpty)
    #expect(await fixture.transport.requests.isEmpty)
    if barrier == "companion-link" { #expect(try String(contentsOf:fixture.root.appendingPathComponent("retained.json"),encoding:.utf8) == "retained synthetic data") }
}

@Test @MainActor func preparedPhoneRejectsChangedJournalFieldsBeforeRecovery() async throws {
    let fixture = try PhoneSetupFixture(); defer { fixture.close() }
    await #expect(throws:URLError.self) { try await fixture.connect() }
    let path = fixture.root.appendingPathComponent("companion-setup.json")
    var value = try #require(JSONSerialization.jsonObject(with:Data(contentsOf:path)) as? [String:Any])
    value["unexpected"] = "changed record"
    let changed = try JSONSerialization.data(withJSONObject:value)
    try changed.write(to:path)
    let requests = await fixture.transport.requests.count
    await #expect(throws:(any Error).self) { try await fixture.coordinator().recover() }
    #expect(await fixture.transport.requests.count == requests)
    #expect(try fixture.storage.connection() == nil)
    #expect(try Data(contentsOf:path) == changed)
}

@Test @MainActor func preparedPhoneBlocksSavedHistoryDeletionForAnOlderConfirmedPairing() async throws {
    let fixture = try PhoneSetupFixture(); defer { fixture.close() }
    let old = try DeviceConnection(endpoint:.network(baseURL:URL(string:"https://old.invalid")!,fixtureLoopback:false),installationID:"older",deviceID:"older-phone",role:.companion)
    try fixture.storage.save(old,token:syntheticSecret(3)); try fixture.storage.removePairing(old)
    await #expect(throws:URLError.self) { try await fixture.connect() }
    let pending = try fixture.coordinator().pendingSetup()
    #expect(throws:(any Error).self) { try fixture.storage.deleteSavedHistory(for:old) }
    #expect(try fixture.coordinator().pendingSetup() == pending)
}

@Test @MainActor func preparedPhoneCannotOverwriteAnUnconfirmedOlderRemoval() async throws {
    let fixture = try PhoneSetupFixture(); defer { fixture.close() }
    let old = try DeviceConnection(endpoint:.network(baseURL:URL(string:"https://old.invalid")!,fixtureLoopback:false),installationID:"older",deviceID:"older-phone",role:.companion)
    let files = try OwnerSetupFiles(directory:fixture.root)
    let retained = try JSONEncoder().encode(PairingRemovalIntent(connection:old,state:.locallyForgotten,scope:.companion))
    try files.write(retained,to:.removal); try files.write(JSONEncoder().encode(old),to:.disconnected); files.close()
    await #expect(throws:(any Error).self) { try await fixture.connect() }
    #expect(fixture.credentials.generations == 0)
    #expect(await fixture.transport.requests.isEmpty)
    #expect(try Data(contentsOf:fixture.root.appendingPathComponent("pairing-removal.json")) == retained)
}

#if os(macOS)
private actor CompetingOwnerSetup: OwnerSetupControl {
    var begins = 0
    func beginOwnerSetup(timeout: TimeInterval) throws -> OwnerSetupState {
        begins += 1
        return .available(try OwnerSetupOffer(installationID:"different-host",setupID:"new-owner",expiresAt:"2099-01-01T00:00:00.000Z"))
    }
    func completeOwnerSetup(_ request: OwnerSetupRequest, timeout: TimeInterval) throws -> OwnerSetupReceipt {
        try OwnerSetupReceipt(installationID:request.installationID,deviceID:request.deviceID,expiresAt:nil)
    }
}
@Test(arguments:["account-sign-in.json","history-removal.json"])
@MainActor func preparedPhonePrimarySetupRefusesOtherPendingWorkBeforeHostContact(barrier: String) async throws {
    let fixture = try PhoneSetupFixture(); defer { fixture.close() }
    let files = try OwnerSetupFiles(directory:fixture.root)
    try files.write(Data("{}".utf8),to:try #require(OwnerSetupFiles.Record(rawValue:barrier))); files.close()
    let control = CompetingOwnerSetup()
    let owner = try OwnerSetupCoordinator(storage:fixture.storage,endpoint:.macHost(serviceIdentifier:"io.fort.synthetic.host",signingTeam:"AAAAAAAAAA"),control:control,tokenGenerator:{ syntheticSecret(4) })
    await #expect(throws:(any Error).self) { try await owner.connect() }
    #expect(await control.begins == 0)
    #expect(fixture.credentials.tokens.isEmpty)
}
@Test @MainActor func preparedPhoneBlocksCompetingPrimarySetupBeforeHostContact() async throws {
    let fixture = try PhoneSetupFixture(); defer { fixture.close() }
    await #expect(throws:URLError.self) { try await fixture.connect() }
    let control = CompetingOwnerSetup()
    let owner = try OwnerSetupCoordinator(storage:fixture.storage,endpoint:.macHost(serviceIdentifier:"io.fort.synthetic.host",signingTeam:"AAAAAAAAAA"),control:control,tokenGenerator:{ syntheticSecret(4) })
    await #expect(throws:(any Error).self) { try await owner.connect() }
    #expect(await control.begins == 0)
    #expect(try fixture.storage.connection() == nil)
    #expect(try fixture.coordinator().pendingSetup() != nil)
}
#endif

@Test @MainActor func preparedPhoneLateJoinCannotPublishAChangedPlan() async throws {
    let root = FileManager.default.temporaryDirectory.appendingPathComponent("fort-late-phone-" + UUID().uuidString)
    defer { try? FileManager.default.removeItem(at:root) }
    let credentials = SetupCredentials(), storage = try DeviceStorage(directory:root,credentials:credentials)
    let transport = JoiningTransport { _,_ in
        let file = root.appendingPathComponent("companion-setup.json")
        var value = try #require(JSONSerialization.jsonObject(with:Data(contentsOf:file)) as? [String:Any])
        value["deviceName"] = "Changed while awaiting reply"
        try JSONSerialization.data(withJSONObject:value).write(to:file)
    }
    await transport.allowJoinReply()
    let coordinator = CompanionSetupCoordinator(storage:storage,tokenGenerator:{ syntheticSecret(2) },transport:{ _ in transport })
    await #expect(throws:(any Error).self) {
        try await coordinator.connect(address:URL(string:"https://fort.invalid")!,installationID:"one",invitationID:"invitation",code:syntheticSecret(1),deviceName:"Synthetic phone")
    }
    #expect(try storage.connection() == nil)
    #expect(credentials.tokens.count == 1)
    #expect(try String(contentsOf:root.appendingPathComponent("companion-setup.json"),encoding:.utf8).contains("Changed while awaiting reply"))
}

@Test(arguments:["installation","device","role"])
@MainActor func preparedPhoneRecoveryRefusesForeignIdentityOrRole(change: String) async throws {
    let fixture = try PhoneSetupFixture(); defer { fixture.close() }
    await #expect(throws:URLError.self) { try await fixture.connect() }
    await fixture.transport.changeRecovery(change)
    await #expect(throws:(any Error).self) { try await fixture.coordinator().recover() }
    #expect(try fixture.storage.connection() == nil)
    #expect(try fixture.coordinator().pendingSetup() != nil)
    #expect(fixture.credentials.generations == 1 && fixture.credentials.tokens.count == 1)
    #expect(await fixture.transport.requests.filter { $0.path == "/v1/devices/join" }.count == 1)
}

@Test @MainActor func preparedPhoneSurvivesLostJoinReplyWithoutRegeneratingOrResending() async throws {
    let root = FileManager.default.temporaryDirectory.appendingPathComponent("fort-companion-setup-" + UUID().uuidString)
    defer { try? FileManager.default.removeItem(at:root) }
    let credentials = SetupCredentials(), storage = try DeviceStorage(directory:root,credentials:credentials)
    let transport = JoiningTransport { deviceID,token in
        #expect(credentials.tokens[deviceID] == token)
        #expect(try storage.connection() == nil)
        let records = try FileManager.default.contentsOfDirectory(at:root,includingPropertiesForKeys:nil)
        #expect(records.contains { $0.pathExtension == "json" })
        for file in records where file.pathExtension == "json" {
            let text = try String(contentsOf:file,encoding:.utf8)
            #expect(!text.contains(token) && !text.contains(syntheticSecret(1)))
        }
    }
    func coordinator() -> CompanionSetupCoordinator {
        CompanionSetupCoordinator(storage:storage,tokenGenerator:{ credentials.generations += 1; return syntheticSecret(2) },transport:{ _ in transport })
    }
    await #expect(throws:URLError.self) {
        try await coordinator().connect(address:URL(string:"https://fort.invalid")!,installationID:"one",invitationID:"invitation",code:syntheticSecret(1),deviceName:"Synthetic phone")
    }
    #expect(try storage.connection() == nil && credentials.generations == 1)
    let recovered = try await coordinator().recover()
    #expect(recovered.role == .companion && recovered.installationID == "one")
    #expect(try storage.connection() == recovered)
    #expect(try storage.token(for:recovered) == syntheticSecret(2))
    #expect(credentials.generations == 1)
    #expect(await transport.requests.filter { $0.path == "/v1/devices/join" }.count == 1)
}

@Test func companionConnectionRetainsItsRoleAcrossRelaunchAndKeepsLegacyConnections() throws {
    let bytes = Data(#"{"schemaVersion":4,"endpoint":{"kind":"network","baseURL":"https://fort.invalid","fixtureLoopback":false},"installationID":"one","deviceID":"phone","role":"companion"}"#.utf8)
    let connection = try JSONDecoder().decode(DeviceConnection.self,from:bytes)
    let encoded = try #require(JSONSerialization.jsonObject(with:JSONEncoder().encode(connection)) as? [String:Any])
    #expect(encoded["role"] as? String == "companion")
    #expect(encoded["schemaVersion"] as? Int == 4)
    #expect(connection.installationID == "one" && connection.deviceID == "phone")
    let old = Data(#"{"schemaVersion":2,"endpoint":{"kind":"network","baseURL":"https://fort.invalid","fixtureLoopback":false},"installationID":"one","deviceID":"old-primary"}"#.utf8)
    #expect(try JSONDecoder().decode(DeviceConnection.self,from:old).deviceID == "old-primary")
}

@Test(arguments:["missing-role","unknown-role","http","fixture","local-host","legacy-role"])
func companionConnectionRejectsAmbiguousOrInsecureRoleRecords(change: String) throws {
    var value: [String:Any] = ["schemaVersion":4,"endpoint":["kind":"network","baseURL":"https://fort.invalid","fixtureLoopback":false],"installationID":"one","deviceID":"phone","role":"companion"]
    switch change {
    case "missing-role": value.removeValue(forKey:"role")
    case "unknown-role": value["role"] = "host"
    case "http": value["endpoint"] = ["kind":"network","baseURL":"http://127.0.0.1","fixtureLoopback":true]
    case "fixture": value["endpoint"] = ["kind":"network","baseURL":"https://fort.invalid","fixtureLoopback":true]
    case "local-host": value["endpoint"] = ["kind":"macHost","serviceIdentifier":"io.fort.synthetic.host","signingTeam":"AAAAAAAAAA"]
    default: value["schemaVersion"] = 2
    }
    let bytes = try JSONSerialization.data(withJSONObject:value)
    #expect(throws:(any Error).self) { try JSONDecoder().decode(DeviceConnection.self,from:bytes) }
}

@Test(.enabled(if:ProcessInfo.processInfo.environment["FORT_NATIVE_COMPANION_SETUP_BENCHMARK"] != nil))
@MainActor func preparedPhoneRecoveryMeetsItsIsolatedBudget() async throws {
    var samples: [Double] = []
    for index in 0..<25 {
        let fixture = try PhoneSetupFixture(); defer { fixture.close() }
        let started = ContinuousClock.now
        await #expect(throws:URLError.self) { try await fixture.connect() }
        let connection = try await fixture.coordinator().recover()
        #expect(try fixture.storage.connection() == connection)
        #expect(fixture.credentials.generations == 1)
        #expect(await fixture.transport.requests.filter { $0.path == "/v1/devices/join" }.count == 1)
        let elapsed = started.duration(to:.now).components
        if index >= 5 { samples.append(Double(elapsed.seconds) * 1000 + Double(elapsed.attoseconds) / 1e15) }
    }
    samples.sort(); #expect(samples[18] < 100)
    let result: [String:Any] = ["warmups":5,"samples":samples,"p50":samples[9],"p95":samples[18],"maximum":samples[19],"budgetMs":100]
    let destination = try #require(ProcessInfo.processInfo.environment["FORT_NATIVE_COMPANION_SETUP_BENCHMARK"])
    try JSONSerialization.data(withJSONObject:result,options:.sortedKeys).write(to:URL(fileURLWithPath:destination),options:.withoutOverwriting)
}
