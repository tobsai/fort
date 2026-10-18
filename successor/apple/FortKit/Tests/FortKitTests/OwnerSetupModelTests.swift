#if os(macOS)
import Foundation
import Security
import Testing
@testable import FortKit

private final class SetupModelCredentials: DeviceCredentials {
    var values: [String:String] = [:]
    var reads = 0
    var saves = 0
    var removals = 0
    var failRemoval = false
    func save(_ connection: DeviceConnection, token: String) throws {
        saves += 1; values[connection.installationID + "/" + connection.deviceID] = token
    }
    func token(for connection: DeviceConnection) throws -> String {
        reads += 1
        guard let token = values[connection.installationID + "/" + connection.deviceID] else { throw DeviceStorageError.keychain(errSecItemNotFound) }
        return token
    }
    func remove(_ connection: DeviceConnection) throws {
        if failRemoval { throw URLError(.cannotRemoveFile) }
        removals += 1; values.removeValue(forKey:connection.installationID + "/" + connection.deviceID)
    }
}
@MainActor private final class SetupModelControl: OwnerSetupControl, OwnerSetupCancellationControl {
    var begins = 0
    var requests: [OwnerSetupRequest] = []
    var cancellations: [OwnerSetupIdentity] = []
    var loseReply = false
    var cancelFailure = false
    var pauseCompletion = false
    var waiting = false
    func beginOwnerSetup(timeout: TimeInterval) async throws -> OwnerSetupState {
        begins += 1
        return .available(try OwnerSetupOffer(installationID:"installation",setupID:"setup",expiresAt:"2026-10-18T00:05:00.000Z"))
    }
    func completeOwnerSetup(_ request: OwnerSetupRequest, timeout: TimeInterval) async throws -> OwnerSetupReceipt {
        requests.append(request)
        if pauseCompletion {
            waiting = true
            defer { waiting = false }
            try await Task.sleep(for:.seconds(30))
        }
        if loseReply { loseReply = false; throw URLError(.networkConnectionLost) }
        return try OwnerSetupReceipt(installationID:request.installationID,deviceID:request.deviceID,expiresAt:nil)
    }
    func cancelOwnerSetup(_ identity: OwnerSetupIdentity, timeout: TimeInterval) async throws -> OwnerSetupCancellation {
        cancellations.append(identity)
        if cancelFailure { throw URLError(.networkConnectionLost) }
        return .revoked(identity)
    }
}
private actor SetupModelTransport: FortTransport {
    var paths: [String] = []
    func send(_ request: FortRequest) async throws -> FortResponse {
        paths.append(request.path)
        let body: String
        switch request.path {
        case "/v1/installation": body = #"{"contractVersion":"1.0","installationId":"installation","recoveryEpoch":"epoch","mode":"local","serviceName":"Synthetic host"}"#
        case "/v1/agents": body = #"{"agents":[]}"#
        case "/v1/conversations": body = #"{"conversations":[]}"#
        case "/v1/skills": body = #"{"skills":[]}"#
        default: throw ClientError.invalidResponse
        }
        return FortResponse(status:200,body:Data(body.utf8))
    }
}
private let setupModelEndpoint = DeviceEndpoint.macHost(serviceIdentifier:"io.fort.synthetic.host",signingTeam:"AAAAAAAAAA")
private let setupModelAgentEndpoint = DeviceEndpoint.macAgent(try! MacAgentEndpoint(serviceIdentifier:"io.fort.synthetic.agent",signingTeam:"AAAAAAAAAA",appGroup:"group.io.fort.synthetic"))
@MainActor private struct SetupModelWithoutCancellation: OwnerSetupControl {
    let base: SetupModelControl
    func beginOwnerSetup(timeout: TimeInterval) async throws -> OwnerSetupState { try await base.beginOwnerSetup(timeout:timeout) }
    func completeOwnerSetup(_ request: OwnerSetupRequest, timeout: TimeInterval) async throws -> OwnerSetupReceipt { try await base.completeOwnerSetup(request,timeout:timeout) }
}
private func setupModelDirectory() -> URL { FileManager.default.temporaryDirectory.appendingPathComponent("fort-setup-model-" + UUID().uuidString) }
@MainActor private func setupModel(_ storage: DeviceStorage, _ control: any OwnerSetupControl, _ transport: SetupModelTransport, endpoint: DeviceEndpoint = setupModelEndpoint) throws -> MessengerModel {
    try MessengerModel(storage:storage,session:.shared,ownerEndpoint:endpoint,ownerSetupControl:control,connectionTransport:{ _ in transport })
}

@Test(arguments:[setupModelEndpoint,setupModelAgentEndpoint]) @MainActor
func ownerSetupModelInspectsWithoutActivationAndConnectsOnlyOnExplicitAction(endpoint: DeviceEndpoint) async throws {
    let root = FileManager.default.temporaryDirectory.appendingPathComponent("fort-setup-model-" + UUID().uuidString)
    defer { try? FileManager.default.removeItem(at:root) }
    let credentials = SetupModelCredentials(), control = SetupModelControl(), transport = SetupModelTransport()
    let storage = try DeviceStorage(directory:root,credentials:credentials)
    let model = try setupModel(storage,control,transport,endpoint:endpoint)
    #expect(model.hasOwnerSetup && !model.canCompleteOwnerSetup)
    await model.restore()
    #expect(model.canCompleteOwnerSetup && model.pendingOwnerSetup == nil)
    #expect(control.begins == 0 && credentials.reads == 0 && credentials.saves == 0)
    #expect(await transport.paths.isEmpty)
    await model.completeOwnerSetup()
    #expect(model.connected && !model.showConnection && model.pendingOwnerSetup == nil)
    #expect(control.begins == 1 && control.requests.count == 1 && credentials.saves == 1)
    #expect(try storage.connection()?.endpoint == endpoint)
    #expect(await transport.paths.contains("/v1/installation"))
    #expect(!model.canCompleteOwnerSetup)
    await model.completeOwnerSetup()
    #expect(control.begins == 1 && control.requests.count == 1)
}

@Test @MainActor func ownerSetupModelAgentCancellationRetainsExactEndpointAcrossReopen() async throws {
    let root = setupModelDirectory()
    defer { try? FileManager.default.removeItem(at:root) }
    let credentials = SetupModelCredentials(), control = SetupModelControl(), transport = SetupModelTransport()
    let storage = try DeviceStorage(directory:root,credentials:credentials)
    let model = try setupModel(storage,control,transport,endpoint:setupModelAgentEndpoint)
    await model.restore(); control.loseReply = true; await model.completeOwnerSetup()
    let pending = try #require(model.pendingOwnerSetup)
    #expect(pending.endpoint == setupModelAgentEndpoint)
    control.cancelFailure = true; await model.cancelOwnerSetup(pending.identity)
    #expect(model.pendingOwnerSetup?.phase == .canceling)
    let reopened = try setupModel(DeviceStorage(directory:root,credentials:credentials),control,transport,endpoint:setupModelAgentEndpoint)
    let reads = credentials.reads
    await reopened.restore()
    #expect(reopened.pendingOwnerSetup?.endpoint == setupModelAgentEndpoint && credentials.reads == reads)
    control.cancelFailure = false; await reopened.cancelOwnerSetup(pending.identity)
    #expect(reopened.pendingOwnerSetup == nil && credentials.values.isEmpty)
    #expect(control.cancellations == [pending.identity,pending.identity])
    #expect(await transport.paths.isEmpty)
}

@Test @MainActor func ownerSetupModelRetainsUncertainSetupAcrossReopenWithoutReadingItsCredential() async throws {
    let root = setupModelDirectory()
    defer { try? FileManager.default.removeItem(at:root) }
    let credentials = SetupModelCredentials(), control = SetupModelControl(), transport = SetupModelTransport()
    let storage = try DeviceStorage(directory:root,credentials:credentials)
    let model = try setupModel(storage,control,transport)
    await model.restore(); control.loseReply = true
    await model.completeOwnerSetup()
    #expect(model.pendingOwnerSetup?.phase == .ready && model.canCompleteOwnerSetup && model.canCancelOwnerSetup)
    #expect(!model.connected && model.showConnection)
    let reads = credentials.reads, identity = try #require(model.pendingOwnerSetup?.identity)
    let reopened = try setupModel(DeviceStorage(directory:root,credentials:credentials),control,transport)
    await reopened.restore()
    #expect(reopened.pendingOwnerSetup?.identity == identity && credentials.reads == reads)
    #expect(control.begins == 1 && control.requests.count == 1)
    #expect(await transport.paths.isEmpty)
    await reopened.completeOwnerSetup()
    #expect(reopened.connected && control.requests.count == 2 && control.requests[0] == control.requests[1])
    #expect(credentials.saves == 1 && reopened.pendingOwnerSetup == nil)
}

@Test(arguments:[false,true]) @MainActor
func ownerSetupModelCancelsAfterLostReplyAndRetriesHostUncertainty(failFirst: Bool) async throws {
    let root = setupModelDirectory()
    defer { try? FileManager.default.removeItem(at:root) }
    let credentials = SetupModelCredentials(), control = SetupModelControl(), transport = SetupModelTransport()
    let storage = try DeviceStorage(directory:root,credentials:credentials)
    let model = try setupModel(storage,control,transport)
    await model.restore(); control.loseReply = true
    await model.completeOwnerSetup()
    let identity = try #require(model.pendingOwnerSetup?.identity)
    control.cancelFailure = failFirst
    await model.cancelOwnerSetup(identity)
    if failFirst {
        #expect(model.pendingOwnerSetup?.phase == .canceling && !model.canCompleteOwnerSetup && model.canCancelOwnerSetup)
        #expect(credentials.removals == 0)
        await model.completeOwnerSetup()
        #expect(control.requests.count == 1)
        control.cancelFailure = false
        let reopened = try setupModel(DeviceStorage(directory:root,credentials:credentials),control,transport)
        await reopened.restore(); await reopened.cancelOwnerSetup(identity)
        #expect(reopened.pendingOwnerSetup == nil && reopened.canCompleteOwnerSetup)
    } else {
        #expect(model.pendingOwnerSetup == nil && model.canCompleteOwnerSetup)
        #expect(model.status == "No unfinished setup remains. Saved conversations have been kept.")
    }
    #expect(credentials.values.isEmpty && credentials.removals == 1)
    #expect(await transport.paths.isEmpty)
}

@Test @MainActor func ownerSetupModelRetainsConfirmedCancellationUntilCredentialCleanupSucceeds() async throws {
    let root = setupModelDirectory()
    defer { try? FileManager.default.removeItem(at:root) }
    let credentials = SetupModelCredentials(), control = SetupModelControl(), transport = SetupModelTransport()
    let storage = try DeviceStorage(directory:root,credentials:credentials)
    let model = try setupModel(storage,control,transport)
    await model.restore(); control.loseReply = true
    await model.completeOwnerSetup()
    let identity = try #require(model.pendingOwnerSetup?.identity)
    credentials.failRemoval = true
    await model.cancelOwnerSetup(identity)
    #expect(model.pendingOwnerSetup?.phase == .canceled && !model.canCompleteOwnerSetup && model.canCancelOwnerSetup)
    credentials.failRemoval = false; control.cancelFailure = true
    let reopened = try setupModel(DeviceStorage(directory:root,credentials:credentials),control,transport)
    let reads = credentials.reads
    await reopened.restore()
    #expect(credentials.reads == reads)
    await reopened.cancelOwnerSetup(identity)
    #expect(reopened.pendingOwnerSetup == nil && control.cancellations == [identity])
    #expect(credentials.values.isEmpty && credentials.removals == 1)
}

@Test @MainActor func ownerSetupModelBlocksPartiallySavedConnectionUntilExplicitCancellationAndKeepsHistory() async throws {
    let root = setupModelDirectory()
    defer { try? FileManager.default.removeItem(at:root) }
    let credentials = SetupModelCredentials(), control = SetupModelControl(), transport = SetupModelTransport()
    let storage = try DeviceStorage(directory:root,credentials:credentials)
    let model = try setupModel(storage,control,transport)
    await model.restore(); control.loseReply = true
    await model.completeOwnerSetup()
    let request = try #require(control.requests.first), identity = try #require(model.pendingOwnerSetup?.identity)
    let connection = try DeviceConnection(endpoint:setupModelEndpoint,installationID:request.installationID,deviceID:request.deviceID)
    try storage.save(connection,token:request.deviceToken)
    let installation = Installation(contractVersion:"1.0",installationId:"installation",recoveryEpoch:"epoch",mode:"local",serviceName:"Saved history")
    try storage.saveNavigation(SavedNavigation(installation:installation,agents:[],selectedAgentID:nil))
    let reopened = try setupModel(DeviceStorage(directory:root,credentials:credentials),control,transport)
    let reads = credentials.reads
    await reopened.restore()
    #expect(credentials.reads == reads && !reopened.connected && reopened.showConnection)
    #expect(await transport.paths.isEmpty)
    await reopened.cancelOwnerSetup(identity)
    #expect(reopened.installation == installation && !reopened.connected && !reopened.supportsCommands)
    #expect(try storage.connection() == nil)
    #expect(try storage.disconnectedConnection() == connection)
    #expect(await transport.paths.isEmpty)
}

@Test @MainActor func ownerSetupModelSerializesExplicitActionsAndDoesNotUseFixtureEnrollment() async throws {
    let root = setupModelDirectory()
    defer { try? FileManager.default.removeItem(at:root) }
    let credentials = SetupModelCredentials(), control = SetupModelControl(), transport = SetupModelTransport()
    let storage = try DeviceStorage(directory:root,credentials:credentials)
    let model = try setupModel(storage,control,transport)
    await model.restore(); control.pauseCompletion = true
    let work = Task { await model.completeOwnerSetup() }
    for _ in 0..<100 where !control.waiting { try await Task.sleep(for:.milliseconds(5)) }
    #expect(control.waiting && model.ownerSetupBusy && !model.canCompleteOwnerSetup)
    let reads = credentials.reads
    await model.completeOwnerSetup(); await model.restore(); model.inspectOwnerSetup()
    model.expectedInstallation = "fixture"; model.enrollmentCode = "synthetic-code"
    await model.enroll()
    #expect(credentials.reads == reads && control.begins == 1 && control.requests.count == 1)
    #expect(await transport.paths.isEmpty)
    work.cancel(); await work.value
    #expect(!model.ownerSetupBusy && model.pendingOwnerSetup?.phase == .ready && model.canCompleteOwnerSetup)
    control.pauseCompletion = false
    await model.completeOwnerSetup()
    #expect(model.connected && credentials.saves == 1)
}

@Test @MainActor func ownerSetupModelRejectsAStaleCancellationConfirmation() async throws {
    let root = setupModelDirectory()
    defer { try? FileManager.default.removeItem(at:root) }
    let credentials = SetupModelCredentials(), control = SetupModelControl(), transport = SetupModelTransport()
    let storage = try DeviceStorage(directory:root,credentials:credentials)
    let model = try setupModel(storage,control,transport)
    await model.restore(); control.loseReply = true
    await model.completeOwnerSetup()
    let original = try #require(model.pendingOwnerSetup?.identity), file = root.appendingPathComponent("owner-setup.json")
    var record = try #require(JSONSerialization.jsonObject(with:Data(contentsOf:file)) as? [String:Any])
    var offer = try #require(record["offer"] as? [String:Any]); offer["setupId"] = "replacement"; record["offer"] = offer
    try JSONSerialization.data(withJSONObject:record).write(to:file)
    await model.cancelOwnerSetup(original)
    #expect(model.pendingOwnerSetup?.identity.setupID == "replacement")
    #expect(control.cancellations.isEmpty && credentials.removals == 0)
    await model.cancelOwnerSetup(original)
    #expect(control.cancellations.isEmpty && credentials.removals == 0)
}

@Test @MainActor func ownerSetupModelDoesNotRestoreAConnectionForAnotherHost() async throws {
    let root = setupModelDirectory()
    defer { try? FileManager.default.removeItem(at:root) }
    let credentials = SetupModelCredentials(), control = SetupModelControl(), transport = SetupModelTransport()
    let storage = try DeviceStorage(directory:root,credentials:credentials)
    let foreign = try DeviceConnection(endpoint:.macHost(serviceIdentifier:"io.fort.another.host",signingTeam:"BBBBBBBBBB"),installationID:"installation",deviceID:"device")
    try storage.save(foreign,token:String(repeating:"A",count:43))
    let model = try setupModel(storage,control,transport)
    await model.restore()
    #expect(credentials.reads == 0 && !model.connected)
    #expect(await transport.paths.isEmpty)
    #expect(try storage.connection() == foreign)
}

@Test(arguments:["future","link"]) @MainActor
func ownerSetupModelKeepsUnsafePreparationBlockedWithoutReadingCredentials(kind: String) async throws {
    let root = setupModelDirectory()
    defer { try? FileManager.default.removeItem(at:root) }
    let credentials = SetupModelCredentials(), control = SetupModelControl(), transport = SetupModelTransport()
    let storage = try DeviceStorage(directory:root,credentials:credentials)
    let record = Data(#"{"version":99}"#.utf8), pending = root.appendingPathComponent("owner-setup.json")
    if kind == "link" {
        let retained = root.appendingPathComponent("retained.json")
        try record.write(to:retained)
        try FileManager.default.createSymbolicLink(at:pending,withDestinationURL:retained)
    } else {
        try record.write(to:pending)
        try FileManager.default.setAttributes([.posixPermissions:0o600],ofItemAtPath:pending.path)
    }
    let model = try setupModel(storage,control,transport)
    await model.restore(); await model.completeOwnerSetup()
    #expect(!model.canCompleteOwnerSetup && !model.canCancelOwnerSetup && model.showConnection && !model.connected)
    #expect(credentials.reads == 0 && credentials.saves == 0 && control.begins == 0)
    #expect(try Data(contentsOf:pending) == record)
    #expect(await transport.paths.isEmpty)
}

@Test @MainActor func ownerSetupModelDoesNotInferAnUnconfiguredHostOrEnablePreviewSetup() async throws {
    let root = setupModelDirectory()
    defer { try? FileManager.default.removeItem(at:root) }
    let credentials = SetupModelCredentials(), storage = try DeviceStorage(directory:root,credentials:credentials)
    let model = MessengerModel(storage:storage,session:.shared)
    await model.restore(); model.inspectOwnerSetup(); await model.completeOwnerSetup()
    #expect(!model.hasOwnerSetup && !model.canCompleteOwnerSetup && !model.canCancelOwnerSetup)
    #expect(credentials.reads == 0 && credentials.saves == 0)
    let preview = MessengerModel(storage:storage,session:.shared,isOfflinePreview:true)
    await preview.restore(); preview.inspectOwnerSetup(); await preview.completeOwnerSetup()
    #expect(!preview.hasOwnerSetup && !preview.canCompleteOwnerSetup)
    #expect(credentials.reads == 0 && credentials.saves == 0)
    #expect(!FileManager.default.fileExists(atPath:root.appendingPathComponent("owner-setup.json").path))
}

@Test @MainActor func ownerSetupModelKeepsReadyPreparationWhenCancellationCapabilityIsAbsent() async throws {
    let root = setupModelDirectory()
    defer { try? FileManager.default.removeItem(at:root) }
    let credentials = SetupModelCredentials(), control = SetupModelControl(), transport = SetupModelTransport()
    let storage = try DeviceStorage(directory:root,credentials:credentials)
    let model = try setupModel(storage,SetupModelWithoutCancellation(base:control),transport)
    await model.restore(); control.loseReply = true; await model.completeOwnerSetup()
    let identity = try #require(model.pendingOwnerSetup?.identity)
    await model.cancelOwnerSetup(identity)
    #expect(model.pendingOwnerSetup?.phase == .ready && model.canCompleteOwnerSetup)
    #expect(control.cancellations.isEmpty && credentials.removals == 0)
    #expect(model.ownerSetupMessage.contains("could not finish"))
}

@Test @MainActor func ownerSetupModelRestoresAnExistingPinnedConnectionWithoutNewSetup() async throws {
    let root = setupModelDirectory()
    defer { try? FileManager.default.removeItem(at:root) }
    let credentials = SetupModelCredentials(), control = SetupModelControl(), transport = SetupModelTransport()
    let storage = try DeviceStorage(directory:root,credentials:credentials)
    let connection = try DeviceConnection(endpoint:setupModelEndpoint,installationID:"installation",deviceID:"device")
    try storage.save(connection,token:String(repeating:"A",count:43))
    let model = try setupModel(storage,control,transport)
    await model.restore()
    #expect(model.connected && !model.showConnection && !model.canCompleteOwnerSetup)
    #expect(control.begins == 0 && control.requests.isEmpty && credentials.saves == 1)
    #expect(try storage.connection() == connection)
}

@Test(arguments:[setupModelEndpoint,setupModelAgentEndpoint]) @MainActor
func ownerSetupModelInspectionMeetsLocalBudgetWithoutActivation(endpoint: DeviceEndpoint) async throws {
    let root = setupModelDirectory()
    defer { try? FileManager.default.removeItem(at:root) }
    let credentials = SetupModelCredentials(), control = SetupModelControl(), transport = SetupModelTransport()
    let storage = try DeviceStorage(directory:root,credentials:credentials)
    let model = try setupModel(storage,control,transport,endpoint:endpoint)
    await model.restore(); control.loseReply = true; await model.completeOwnerSetup()
    let reads = credentials.reads
    var samples: [Double] = []
    for _ in 0..<20 {
        let reopened = try setupModel(DeviceStorage(directory:root,credentials:credentials),control,transport,endpoint:endpoint)
        let started = ContinuousClock.now
        reopened.inspectOwnerSetup()
        let elapsed = started.duration(to:.now).components
        samples.append(Double(elapsed.seconds) * 1000 + Double(elapsed.attoseconds) / 1e15)
        #expect(reopened.pendingOwnerSetup?.phase == .ready && reopened.canCompleteOwnerSetup)
    }
    let p95 = samples.sorted()[18]
    print("FORT_OWNER_SETUP_INSPECTION_BUDGET " + String(decoding:try JSONSerialization.data(withJSONObject:["samplesMs":samples,"p95Ms":p95,"budgetMs":50,"connectionSchemaVersion":endpoint.connectionSchemaVersion]),as:UTF8.self))
    #expect(p95 <= 50)
    #expect(credentials.reads == reads && control.begins == 1 && control.requests.count == 1)
    #expect(await transport.paths.isEmpty)
}
private actor SetupHostBackend: MacHostStartupBackend {
    var current: MacHostStartupStatus = .notRegistered
    var registrations = 0
    func status() -> MacHostStartupStatus { current }
    func register() { registrations += 1; current = .enabled }
    func unregister() { Issue.record("Unexpected startup removal") }
    func setStatus(_ status: MacHostStartupStatus) { current = status }
}
@MainActor private final class SetupHostFixture {
    let app = URL(fileURLWithPath:"/synthetic/Fort.app",isDirectory:true)
    let backend = SetupHostBackend()
    let endpoint = try! MacAgentEndpoint(serviceIdentifier:"io.fort.synthetic.agent",signingTeam:"AAAAAAAAAA",appGroup:"group.io.fort.synthetic")
    var valid = true
    var inspections = 0
    lazy var host = MacHostControl(applicationURL:app,inspectBundle:{ url in
        self.inspections += 1
        guard self.valid else { throw MacHostBundleError.signature }
        return MacHostBundle(applicationURL:url,helperURL:url.appendingPathComponent("Contents/Helpers/FortHost.app",isDirectory:true),endpoint:self.endpoint,clientIdentifier:"io.fort.synthetic")
    },makeStartup:{ try MacHostStartup(endpoint:$0,backend:self.backend) })
}

@Test @MainActor func ownerSetupModelRequiresFreshEnabledHostBeforeCreatingCredential() async throws {
    let root = setupModelDirectory(); defer { try? FileManager.default.removeItem(at:root) }
    let fixture = SetupHostFixture(), credentials = SetupModelCredentials(), setup = SetupModelControl(), transport = SetupModelTransport()
    let storage = try DeviceStorage(directory:root,credentials:credentials)
    let model = try MessengerModel(storage:storage,session:.shared,ownerEndpoint:.macAgent(fixture.endpoint),ownerSetupControl:setup,connectionTransport:{ _ in transport },localHost:fixture.host)
    await model.restore()
    #expect(!model.canCompleteOwnerSetup && model.canEnableLocalHost)
    await model.completeOwnerSetup()
    #expect(setup.begins == 0 && credentials.reads == 0 && credentials.saves == 0)
    #expect(await fixture.backend.registrations == 0)
    await model.enableLocalHost()
    #expect(model.canCompleteOwnerSetup && !model.canEnableLocalHost)
    #expect(await fixture.backend.registrations == 1)
    fixture.valid = false
    await model.completeOwnerSetup()
    #expect(!model.canCompleteOwnerSetup && !model.connected)
    #expect(setup.begins == 0 && credentials.reads == 0 && credentials.saves == 0)
    #expect(await transport.paths.isEmpty)
    fixture.valid = true
    await model.inspectLocalHost(); await model.completeOwnerSetup()
    #expect(model.connected && setup.begins == 1 && credentials.saves == 1)
    #expect(try storage.connection()?.endpoint == .macAgent(fixture.endpoint))
}

@Test(arguments:[false,true]) @MainActor
func ownerSetupModelNeverEnablesStartupOverASavedHostRemoval(_ addedAfterInspection: Bool) async throws {
    let root = setupModelDirectory(); defer { try? FileManager.default.removeItem(at:root) }
    let fixture = SetupHostFixture(), credentials = SetupModelCredentials(), setup = SetupModelControl(), transport = SetupModelTransport()
    let storage = try DeviceStorage(directory:root,credentials:credentials)
    let model = try MessengerModel(storage:storage,session:.shared,ownerEndpoint:.macAgent(fixture.endpoint),ownerSetupControl:setup,connectionTransport:{ _ in transport },localHost:fixture.host)
    if addedAfterInspection { await model.inspectLocalHost(); #expect(model.canEnableLocalHost) }
    let record = Data("unverified host removal evidence must be preserved".utf8)
    do {
        let files = try OwnerSetupFiles(directory:root); defer { files.close() }
        try files.write(record,to:.hostRemoval)
    }
    if !addedAfterInspection { await model.inspectLocalHost() }
    await model.enableLocalHost(); await model.completeOwnerSetup()
    #expect(!model.canEnableLocalHost && !model.canCompleteOwnerSetup)
    let blocked = model.ownerSetupMessage
    model.inspectOwnerSetup()
    #expect(model.ownerSetupMessage == blocked)
    #expect(await fixture.backend.registrations == 0)
    #expect(setup.begins == 0 && credentials.reads == 0 && credentials.saves == 0)
    #expect(try Data(contentsOf:root.appendingPathComponent("host-removal.json")) == record)
}

@Test @MainActor func ownerSetupModelRejectsSavedForeignHostBeforeCredentialLookup() async throws {
    let root = setupModelDirectory(); defer { try? FileManager.default.removeItem(at:root) }
    let fixture = SetupHostFixture(), credentials = SetupModelCredentials(), setup = SetupModelControl(), transport = SetupModelTransport()
    let storage = try DeviceStorage(directory:root,credentials:credentials)
    let foreign = try DeviceConnection(endpoint:setupModelEndpoint,installationID:"installation",deviceID:"device")
    try storage.save(foreign,token:"synthetic-only")
    let before = try Data(contentsOf:root.appendingPathComponent("connection.json"))
    let model = try MessengerModel(storage:storage,session:.shared,ownerEndpoint:.macAgent(fixture.endpoint),ownerSetupControl:setup,connectionTransport:{ _ in transport },localHost:fixture.host)
    await fixture.backend.setStatus(.enabled)
    await model.restore(); await model.completeOwnerSetup()
    #expect(!model.canCompleteOwnerSetup && !model.connected)
    #expect(setup.begins == 0 && credentials.reads == 0 && credentials.saves == 1)
    #expect(await transport.paths.isEmpty)
    #expect(try Data(contentsOf:root.appendingPathComponent("connection.json")) == before)
}
#endif
