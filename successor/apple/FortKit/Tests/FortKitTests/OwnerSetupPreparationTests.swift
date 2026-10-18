#if os(macOS)
import Foundation
import Security
import Darwin
import Testing
@testable import FortKit

private final class PreparedCredentials: DeviceCredentials {
    var tokens: [String:String] = [:]
    var saves = 0
    var saveFailure: String?
    var readFailure: OSStatus?
    func save(_ connection: DeviceConnection, token: String) throws {
        saves += 1
        let failure = saveFailure; saveFailure = nil
        if failure == "before" { throw URLError(.cannotWriteToFile) }
        tokens[connection.deviceID] = token
        if failure == "after" { throw URLError(.cannotWriteToFile) }
    }
    func token(for connection: DeviceConnection) throws -> String {
        if let readFailure { throw DeviceStorageError.keychain(readFailure) }
        guard let token = tokens[connection.deviceID] else { throw DeviceStorageError.keychain(errSecItemNotFound) }
        return token
    }
}
@MainActor private final class PreparedControl: OwnerSetupControl {
    let offer = try! OwnerSetupOffer(installationID:"installation",setupID:"setup",expiresAt:"2026-10-18T00:05:00.000Z")
    var begins = 0
    var requests: [OwnerSetupRequest] = []
    var attempts = 0
    var loseReply = true
    var wrongReceipt = false
    var pauseCompletion = false
    var waiting = false
    var existing: OwnerSetupReceipt?
    var onComplete: ((OwnerSetupRequest) throws -> Void)?
    func beginOwnerSetup(timeout: TimeInterval) async throws -> OwnerSetupState { begins += 1; return existing.map { .paired($0) } ?? .available(offer) }
    func completeOwnerSetup(_ request: OwnerSetupRequest, timeout: TimeInterval) async throws -> OwnerSetupReceipt {
        attempts += 1
        try onComplete?(request)
        if let first = requests.first, first != request { throw OwnerSetupPreparationError.changedPreparation }
        requests.append(request)
        if pauseCompletion {
            waiting = true
            defer { waiting = false }
            try await Task.sleep(for:.seconds(30))
        }
        if loseReply { loseReply = false; throw URLError(.networkConnectionLost) }
        return try OwnerSetupReceipt(installationID:request.installationID,deviceID:wrongReceipt ? "foreign" : request.deviceID,expiresAt:nil)
    }
}

@Test @MainActor func ownerSetupPreparationNeverRegeneratesOrSubmitsAnAlteredReadyCredential() async throws {
    let directory = FileManager.default.temporaryDirectory.appendingPathComponent("fort-setup-ready-" + UUID().uuidString)
    defer { try? FileManager.default.removeItem(at:directory) }
    let credentials = PreparedCredentials(), control = PreparedControl()
    let storage = try DeviceStorage(directory:directory,credentials:credentials)
    var generated = 0
    let coordinator = try OwnerSetupCoordinator(storage:storage,endpoint:preparedEndpoint,control:control,tokenGenerator:{ generated += 1; return String(repeating:"A",count:43) })
    await #expect(throws:(any Error).self) { try await coordinator.connect() }
    let request = try #require(control.requests.first)
    credentials.tokens.removeAll()
    await #expect(throws:OwnerSetupPreparationError.self) { try await coordinator.connect() }
    #expect(generated == 1 && control.attempts == 1)
    credentials.tokens[request.deviceID] = String(repeating:"B",count:42) + "A"
    await #expect(throws:OwnerSetupPreparationError.self) { try await coordinator.connect() }
    #expect(generated == 1 && control.attempts == 1)
    #expect(try storage.connection() == nil)
    credentials.tokens[request.deviceID] = request.deviceToken
    #expect(try await coordinator.connect().deviceID == request.deviceID)
}

@Test(arguments:["before","after"]) @MainActor
func ownerSetupPreparationRecoversPartialCredentialWritesWithoutSubmittingEarly(failure: String) async throws {
    let directory = FileManager.default.temporaryDirectory.appendingPathComponent("fort-setup-credential-" + UUID().uuidString)
    defer { try? FileManager.default.removeItem(at:directory) }
    let credentials = PreparedCredentials(), control = PreparedControl()
    credentials.saveFailure = failure; control.loseReply = false
    let storage = try DeviceStorage(directory:directory,credentials:credentials)
    var generated = 0
    let coordinator = try OwnerSetupCoordinator(storage:storage,endpoint:preparedEndpoint,control:control,tokenGenerator:{ generated += 1; return String(repeating:generated == 1 ? "A" : "B",count:42) + "A" })
    await #expect(throws:(any Error).self) { try await coordinator.connect() }
    #expect(control.attempts == 0)
    let bytes = try Data(contentsOf:directory.appendingPathComponent("owner-setup.json"))
    let pending = try #require(JSONSerialization.jsonObject(with:bytes) as? [String:Any])
    #expect(pending["phase"] as? String == "preparing")
    let original = try #require(pending["connection"] as? [String:Any])
    let reopened = try OwnerSetupCoordinator(storage:DeviceStorage(directory:directory,credentials:credentials),endpoint:preparedEndpoint,control:control,tokenGenerator:{ generated += 1; return String(repeating:"B",count:42) + "A" })
    let connection = try await reopened.connect()
    #expect(connection.deviceID == original["deviceID"] as? String)
    #expect(control.attempts == 1 && control.begins == 1)
    #expect(generated == (failure == "after" ? 1 : 2))
}
private let preparedEndpoint = DeviceEndpoint.macHost(serviceIdentifier:"io.fort.synthetic.host",signingTeam:"AAAAAAAAAA")

@Test @MainActor func ownerSetupPreparationPersistsBeforeSubmissionAndRetriesTheSameCredentialAfterRestart() async throws {
    let directory = FileManager.default.temporaryDirectory.appendingPathComponent("fort-setup-preparation-" + UUID().uuidString)
    defer { try? FileManager.default.removeItem(at:directory) }
    let credentials = PreparedCredentials(), control = PreparedControl()
    var generated = 0
    let generate = { generated += 1; return String(repeating:"A",count:43) }
    let storage = try DeviceStorage(directory:directory,credentials:credentials)
    control.onComplete = { request in
        #expect(try credentials.token(for:DeviceConnection(endpoint:preparedEndpoint,installationID:request.installationID,deviceID:request.deviceID)) == request.deviceToken)
        let bytes = try Data(contentsOf:directory.appendingPathComponent("owner-setup.json"))
        let record = try #require(JSONSerialization.jsonObject(with:bytes) as? [String:Any])
        #expect(record["phase"] as? String == "ready")
        #expect(!String(decoding:bytes,as:UTF8.self).contains(request.deviceToken))
    }
    let first = try OwnerSetupCoordinator(storage:storage,endpoint:preparedEndpoint,control:control,tokenGenerator:generate)
    await #expect(throws:(any Error).self) { try await first.connect() }
    #expect(try storage.connection() == nil)
    let reopened = try DeviceStorage(directory:directory,credentials:credentials)
    let second = try OwnerSetupCoordinator(storage:reopened,endpoint:preparedEndpoint,control:control,tokenGenerator:generate)
    let connection = try await second.connect()
    #expect(control.begins == 1 && control.requests.count == 2)
    #expect(generated == 1 && credentials.saves == 1)
    #expect(try reopened.connection() == connection)
    #expect(!FileManager.default.fileExists(atPath:directory.appendingPathComponent("owner-setup.json").path))
    #expect(try await second.connect() == connection)
    #expect(control.requests.count == 2)
}

@Test @MainActor func ownerSetupPreparationKeepsIdentityWhenCredentialReadOrGenerationFails() async throws {
    let directory = FileManager.default.temporaryDirectory.appendingPathComponent("fort-setup-denial-" + UUID().uuidString)
    defer { try? FileManager.default.removeItem(at:directory) }
    let credentials = PreparedCredentials(), control = PreparedControl()
    control.loseReply = false; credentials.readFailure = errSecInteractionNotAllowed
    let storage = try DeviceStorage(directory:directory,credentials:credentials)
    var generated = 0, invalid = true
    let coordinator = try OwnerSetupCoordinator(storage:storage,endpoint:preparedEndpoint,control:control,tokenGenerator:{ generated += 1; return invalid ? "invalid" : String(repeating:"A",count:43) })
    await #expect(throws:(any Error).self) { try await coordinator.connect() }
    #expect(generated == 0 && credentials.saves == 0 && control.attempts == 0)
    let original = try Data(contentsOf:directory.appendingPathComponent("owner-setup.json"))
    credentials.readFailure = nil
    await #expect(throws:ClientError.self) { try await coordinator.connect() }
    #expect(credentials.saves == 0 && control.attempts == 0)
    #expect(try Data(contentsOf:directory.appendingPathComponent("owner-setup.json")) == original)
    invalid = false
    _ = try await coordinator.connect()
    #expect(control.begins == 1 && control.attempts == 1 && credentials.saves == 1)
}

@Test @MainActor func ownerSetupPreparationSerializesOverlappingCoordinatorsAndRetainsCanceledCommit() async throws {
    let directory = FileManager.default.temporaryDirectory.appendingPathComponent("fort-setup-lock-" + UUID().uuidString)
    defer { try? FileManager.default.removeItem(at:directory) }
    let credentials = PreparedCredentials(), control = PreparedControl()
    control.loseReply = false; control.pauseCompletion = true
    let first = try OwnerSetupCoordinator(storage:DeviceStorage(directory:directory,credentials:credentials),endpoint:preparedEndpoint,control:control,tokenGenerator:{ String(repeating:"A",count:43) })
    let second = try OwnerSetupCoordinator(storage:DeviceStorage(directory:directory,credentials:credentials),endpoint:preparedEndpoint,control:control,tokenGenerator:{ Issue.record("Must reuse prepared token"); return String(repeating:"B",count:42) + "A" })
    let pending = Task { try await first.connect() }
    for _ in 0..<100 where !control.waiting { try await Task.sleep(for:.milliseconds(5)) }
    #expect(control.waiting)
    await #expect(throws:OwnerSetupPreparationError.self) { try await second.connect() }
    #expect(control.attempts == 1 && credentials.saves == 1)
    pending.cancel()
    await #expect(throws:CancellationError.self) { try await pending.value }
    control.pauseCompletion = false
    let connection = try await second.connect()
    #expect(connection.deviceID == control.requests.first?.deviceID)
    #expect(control.begins == 1 && control.attempts == 2 && credentials.saves == 1)
}

@Test @MainActor func ownerSetupPreparationRefusesForeignHostAndReceiptWithoutReplacingPendingState() async throws {
    let directory = FileManager.default.temporaryDirectory.appendingPathComponent("fort-setup-identity-" + UUID().uuidString)
    defer { try? FileManager.default.removeItem(at:directory) }
    let credentials = PreparedCredentials(), control = PreparedControl()
    control.loseReply = false; control.wrongReceipt = true
    let storage = try DeviceStorage(directory:directory,credentials:credentials)
    let first = try OwnerSetupCoordinator(storage:storage,endpoint:preparedEndpoint,control:control,tokenGenerator:{ String(repeating:"A",count:43) })
    await #expect(throws:OwnerSetupPreparationError.self) { try await first.connect() }
    #expect(try storage.connection() == nil)
    let original = try Data(contentsOf:directory.appendingPathComponent("owner-setup.json"))
    let other = try OwnerSetupCoordinator(storage:storage,endpoint:.macHost(serviceIdentifier:"io.fort.foreign.host",signingTeam:"AAAAAAAAAA"),control:control,tokenGenerator:{ Issue.record("Must not change host"); return "invalid" })
    await #expect(throws:OwnerSetupPreparationError.self) { try await other.connect() }
    #expect(try Data(contentsOf:directory.appendingPathComponent("owner-setup.json")) == original)
    #expect(control.attempts == 1 && credentials.saves == 1)
    control.wrongReceipt = false
    let connection = try await first.connect()
    await #expect(throws:OwnerSetupPreparationError.self) { try await other.connect() }
    #expect(try storage.connection() == connection)
}

@Test @MainActor func ownerSetupPreparationRetriesConnectionWriteAndInterruptedFinalizationWithoutLosingHistory() async throws {
    let directory = FileManager.default.temporaryDirectory.appendingPathComponent("fort-setup-finalize-" + UUID().uuidString)
    defer { try? FileManager.default.removeItem(at:directory) }
    let credentials = PreparedCredentials(), control = PreparedControl()
    control.loseReply = false
    let storage = try DeviceStorage(directory:directory,credentials:credentials)
    let history = directory.appendingPathComponent("retained-history.json")
    try Data("synthetic retained history".utf8).write(to:history)
    let coordinator = try OwnerSetupCoordinator(storage:storage,endpoint:preparedEndpoint,control:control,tokenGenerator:{ String(repeating:"A",count:43) })
    let connectionFile = directory.appendingPathComponent("connection.json")
    control.onComplete = { _ in try FileManager.default.createDirectory(at:connectionFile,withIntermediateDirectories:false) }
    await #expect(throws:(any Error).self) { try await coordinator.connect() }
    #expect(FileManager.default.fileExists(atPath:directory.appendingPathComponent("owner-setup.json").path))
    let request = try #require(control.requests.first)
    let connection = try DeviceConnection(endpoint:preparedEndpoint,installationID:request.installationID,deviceID:request.deviceID)
    try FileManager.default.removeItem(at:connectionFile)
    try JSONEncoder().encode(connection).write(to:connectionFile,options:.atomic)
    try FileManager.default.setAttributes([.posixPermissions:0o600],ofItemAtPath:connectionFile.path)
    control.onComplete = nil
    #expect(try await coordinator.connect() == connection)
    #expect(control.attempts == 2 && credentials.saves == 1)
    #expect(!FileManager.default.fileExists(atPath:directory.appendingPathComponent("owner-setup.json").path))
    #expect(try String(contentsOf:history,encoding:.utf8) == "synthetic retained history")
}

@Test(arguments:["metadata","pending-link","connection-link","lock-link","pending-fifo","pending-hardlink","directory-link","directory-permissions","lock-permissions"])
@MainActor func ownerSetupPreparationPreservesMalformedOrUnsafeFiles(kind: String) async throws {
    let root = FileManager.default.temporaryDirectory.appendingPathComponent("fort-setup-files-" + UUID().uuidString)
    defer { try? FileManager.default.removeItem(at:root) }
    let directory = root.appendingPathComponent("client")
    let credentials = PreparedCredentials(), control = PreparedControl()
    let storage = try DeviceStorage(directory:directory,credentials:credentials)
    let marker = root.appendingPathComponent("keep")
    try Data("synthetic marker".utf8).write(to:marker)
    try FileManager.default.setAttributes([.posixPermissions:0o600],ofItemAtPath:marker.path)
    let pending = directory.appendingPathComponent("owner-setup.json")
    switch kind {
    case "metadata": try Data(#"{"version":99,"phase":"ready"}"#.utf8).write(to:pending); try FileManager.default.setAttributes([.posixPermissions:0o600],ofItemAtPath:pending.path)
    case "pending-link": try FileManager.default.createSymbolicLink(at:pending,withDestinationURL:marker)
    case "connection-link": try FileManager.default.createSymbolicLink(at:directory.appendingPathComponent("connection.json"),withDestinationURL:marker)
    case "lock-link": try FileManager.default.createSymbolicLink(at:directory.appendingPathComponent("owner-setup.lock"),withDestinationURL:marker)
    case "pending-fifo": #expect(mkfifo(pending.path,0o600) == 0)
    case "pending-hardlink": try FileManager.default.linkItem(at:marker,to:pending)
    case "directory-link":
        let moved = root.appendingPathComponent("moved")
        try FileManager.default.moveItem(at:directory,to:moved)
        try FileManager.default.createSymbolicLink(at:directory,withDestinationURL:moved)
    case "directory-permissions": try FileManager.default.setAttributes([.posixPermissions:0o755],ofItemAtPath:directory.path)
    case "lock-permissions": try Data().write(to:directory.appendingPathComponent("owner-setup.lock")); try FileManager.default.setAttributes([.posixPermissions:0o644],ofItemAtPath:directory.appendingPathComponent("owner-setup.lock").path)
    default: Issue.record("Unexpected fixture")
    }
    let coordinator = try OwnerSetupCoordinator(storage:storage,endpoint:preparedEndpoint,control:control,tokenGenerator:{ Issue.record("Unsafe files cannot create credentials"); return "invalid" })
    await #expect(throws:(any Error).self) { try await coordinator.connect() }
    #expect(control.begins == 0 && control.attempts == 0 && credentials.saves == 0)
    #expect(try String(contentsOf:marker,encoding:.utf8) == "synthetic marker")
    if kind == "metadata" { #expect(try Data(contentsOf:pending) == Data(#"{"version":99,"phase":"ready"}"#.utf8)) }
}

@Test @MainActor func ownerSetupPreparationMeetsSyntheticFileDurabilityBudget() async throws {
    let root = FileManager.default.temporaryDirectory.appendingPathComponent("fort-setup-budget-" + UUID().uuidString)
    defer { try? FileManager.default.removeItem(at:root) }
    var samples: [Double] = []
    for index in 0..<20 {
        let credentials = PreparedCredentials(), control = PreparedControl(); control.loseReply = false
        let storage = try DeviceStorage(directory:root.appendingPathComponent(String(index)),credentials:credentials)
        let coordinator = try OwnerSetupCoordinator(storage:storage,endpoint:preparedEndpoint,control:control,tokenGenerator:{ String(repeating:"A",count:43) })
        let started = ContinuousClock.now
        _ = try await coordinator.connect()
        let elapsed = started.duration(to:.now).components
        samples.append(Double(elapsed.seconds) * 1000 + Double(elapsed.attoseconds) / 1e15)
        for name in ["connection.json","owner-setup.lock"] {
            let attributes = try FileManager.default.attributesOfItem(atPath:storage.directory.appendingPathComponent(name).path)
            #expect((attributes[.posixPermissions] as? NSNumber)?.intValue == 0o600)
        }
    }
    let p95 = samples.sorted()[18]
    print("OWNER_SETUP_FILE_BUDGET " + String(decoding:try JSONSerialization.data(withJSONObject:["samplesMs":samples,"p95Ms":p95,"budgetMs":500,"scope":"synthetic credential storage and in-process control; real Keychain/XPC excluded"]),as:UTF8.self))
    #expect(p95 <= 500)
}

@Test @MainActor func ownerSetupPreparationKeepsExistingHostPairingWithoutAdoptingOrReplacingIt() async throws {
    let directory = FileManager.default.temporaryDirectory.appendingPathComponent("fort-setup-existing-" + UUID().uuidString)
    defer { try? FileManager.default.removeItem(at:directory) }
    let credentials = PreparedCredentials(), control = PreparedControl()
    control.existing = try OwnerSetupReceipt(installationID:"installation",deviceID:"existing-device",expiresAt:nil)
    let storage = try DeviceStorage(directory:directory,credentials:credentials)
    let coordinator = try OwnerSetupCoordinator(storage:storage,endpoint:preparedEndpoint,control:control,tokenGenerator:{ Issue.record("Existing owner must stay paired"); return "invalid" })
    await #expect(throws:OwnerSetupPreparationError.self) { try await coordinator.connect() }
    #expect(control.begins == 1 && control.attempts == 0 && credentials.saves == 0)
    #expect(try storage.connection() == nil)
    #expect(!FileManager.default.fileExists(atPath:directory.appendingPathComponent("owner-setup.json").path))
}

@MainActor private final class LostCoreSetupReply: OwnerSetupControl {
    let core: MacCoreProcessTransport
    let loseReply: Bool
    init(core: MacCoreProcessTransport, loseReply: Bool) { self.core = core; self.loseReply = loseReply }
    func beginOwnerSetup(timeout: TimeInterval) async throws -> OwnerSetupState { try await core.beginOwnerSetup(timeout:timeout) }
    func completeOwnerSetup(_ request: OwnerSetupRequest, timeout: TimeInterval) async throws -> OwnerSetupReceipt {
        let receipt = try await core.completeOwnerSetup(request,timeout:timeout)
        if loseReply { throw URLError(.networkConnectionLost) }
        return receipt
    }
}

@Test @MainActor func ownerSetupPreparationRecoversACommittedCompiledCorePairingAfterBothSidesReopen() async throws {
    let root = FileManager.default.temporaryDirectory.appendingPathComponent("fort-setup-compiled-" + UUID().uuidString)
    defer { try? FileManager.default.removeItem(at:root) }
    let coreDirectory = root.appendingPathComponent("core"), clientDirectory = root.appendingPathComponent("client")
    try FileManager.default.createDirectory(at:coreDirectory,withIntermediateDirectories:true,attributes:[.posixPermissions:0o700])
    let credentials = PreparedCredentials()
    let first = try await setupCore(coreDirectory)
    do {
        let storage = try DeviceStorage(directory:clientDirectory,credentials:credentials)
        let preparation = try OwnerSetupCoordinator(storage:storage,endpoint:preparedEndpoint,control:LostCoreSetupReply(core:first,loseReply:true),tokenGenerator:{ String(repeating:"A",count:43) })
        await #expect(throws:(any Error).self) { try await preparation.connect() }
        #expect(try storage.connection() == nil && credentials.saves == 1)
        guard case .paired(let receipt) = try await first.beginOwnerSetup() else { Issue.record("Core should have committed before the reply was lost"); _ = try await first.close(); return }
        try await stopSetupCore(first,directory:coreDirectory)
        let second = try await setupCore(coreDirectory)
        do {
            let reopened = try DeviceStorage(directory:clientDirectory,credentials:credentials)
            let retry = try OwnerSetupCoordinator(storage:reopened,endpoint:preparedEndpoint,control:LostCoreSetupReply(core:second,loseReply:false),tokenGenerator:{ Issue.record("Committed credential cannot be replaced"); return "invalid" })
            let connection = try await retry.connect()
            #expect(connection.installationID == receipt.installationID && connection.deviceID == receipt.deviceID)
            let client = FortClient(installationID:connection.installationID,token:try reopened.token(for:connection),transport:second)
            #expect(try await client.installation().installationId == connection.installationID)
            #expect(try reopened.connection() == connection && credentials.saves == 1)
            try await stopSetupCore(second,directory:coreDirectory)
        } catch { _ = try? await second.close(); throw error }
    } catch { _ = try? await first.close(); throw error }
}
#endif
