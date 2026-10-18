#if os(macOS)
import Foundation
import Security
import Testing
import Darwin
@testable import FortKit

private final class CancellationCredentials: DeviceCredentials {
    var tokens: [String:String] = [:]
    var reads = 0
    var saves = 0
    var removals = 0
    var readFailure: OSStatus?
    var saveFailure: String?
    var removalFailure: String?
    var onRemove: (() throws -> Void)?
    func save(_ connection: DeviceConnection, token: String) throws {
        saves += 1
        if saveFailure == "before" { throw URLError(.cannotWriteToFile) }
        tokens[connection.installationID + "/" + connection.deviceID] = token
        if saveFailure == "after" { throw URLError(.cannotWriteToFile) }
    }
    func token(for connection: DeviceConnection) throws -> String {
        reads += 1
        if let readFailure { throw DeviceStorageError.keychain(readFailure) }
        guard let token = tokens[connection.installationID + "/" + connection.deviceID] else { throw DeviceStorageError.keychain(errSecItemNotFound) }
        return token
    }
    func remove(_ connection: DeviceConnection) throws {
        removals += 1
        try onRemove?()
        if removalFailure == "before" { throw URLError(.cannotWriteToFile) }
        if removalFailure != "ignored" { tokens.removeValue(forKey:connection.installationID + "/" + connection.deviceID) }
        if removalFailure == "after" { throw URLError(.cannotWriteToFile) }
    }
}

@MainActor private final class CancellationControl: OwnerSetupControl, OwnerSetupCancellationControl {
    let offer = try! OwnerSetupOffer(installationID:"installation",setupID:"setup",expiresAt:"2026-10-18T00:05:00.000Z")
    var begins = 0
    var completions: [OwnerSetupRequest] = []
    var cancellations: [OwnerSetupIdentity] = []
    var loseCompletionReply = true
    var loseCancellationReply = false
    var cancellationFailure = false
    var wrongCancellationProof = false
    var pauseCancellation = false
    var waiting = false
    var onCancel: (() throws -> Void)?
    func beginOwnerSetup(timeout: TimeInterval) async throws -> OwnerSetupState { begins += 1; return .available(offer) }
    func completeOwnerSetup(_ request: OwnerSetupRequest, timeout: TimeInterval) async throws -> OwnerSetupReceipt {
        completions.append(request)
        if loseCompletionReply { loseCompletionReply = false; throw URLError(.networkConnectionLost) }
        return try OwnerSetupReceipt(installationID:request.installationID,deviceID:request.deviceID,expiresAt:nil)
    }
    func cancelOwnerSetup(_ identity: OwnerSetupIdentity, timeout: TimeInterval) async throws -> OwnerSetupCancellation {
        cancellations.append(identity)
        try onCancel?()
        if pauseCancellation {
            waiting = true
            defer { waiting = false }
            try await Task.sleep(for:.seconds(30))
        }
        if cancellationFailure { throw URLError(.cannotConnectToHost) }
        if loseCancellationReply { loseCancellationReply = false; throw URLError(.networkConnectionLost) }
        let proofIdentity = wrongCancellationProof ? try OwnerSetupIdentity(installationID:identity.installationID,setupID:"foreign",deviceID:identity.deviceID) : identity
        return completions.isEmpty ? .notPaired(proofIdentity) : .revoked(proofIdentity)
    }
}

private let recoveryEndpoint = DeviceEndpoint.macHost(serviceIdentifier:"io.fort.synthetic.host",signingTeam:"AAAAAAAAAA")
@MainActor private func recoveryCoordinator(_ storage: DeviceStorage, _ control: any OwnerSetupControl, endpoint: DeviceEndpoint = recoveryEndpoint) throws -> OwnerSetupCoordinator {
    try OwnerSetupCoordinator(storage:storage,endpoint:endpoint,control:control,tokenGenerator:{ String(repeating:"A",count:43) })
}
private func recoveryDirectory() -> URL { FileManager.default.temporaryDirectory.appendingPathComponent("fort-setup-recovery-" + UUID().uuidString) }
private func recoveryRecord(_ directory: URL) throws -> [String:Any] {
    try #require(JSONSerialization.jsonObject(with:Data(contentsOf:directory.appendingPathComponent("owner-setup.json"))) as? [String:Any])
}
private func writeRecoveryRecord(_ record: [String:Any], directory: URL) throws {
    let file = directory.appendingPathComponent("owner-setup.json")
    try JSONSerialization.data(withJSONObject:record).write(to:file,options:.atomic)
    try FileManager.default.setAttributes([.posixPermissions:0o600],ofItemAtPath:file.path)
}
@MainActor private struct SetupOnlyControl: OwnerSetupControl {
    let base: CancellationControl
    func beginOwnerSetup(timeout: TimeInterval) async throws -> OwnerSetupState { try await base.beginOwnerSetup(timeout:timeout) }
    func completeOwnerSetup(_ request: OwnerSetupRequest, timeout: TimeInterval) async throws -> OwnerSetupReceipt { try await base.completeOwnerSetup(request,timeout:timeout) }
}
private struct UncertainSetupCore: OwnerSetupControl, OwnerSetupCancellationControl {
    let core: MacCoreProcessTransport
    func beginOwnerSetup(timeout: TimeInterval) async throws -> OwnerSetupState { try await core.beginOwnerSetup(timeout:timeout) }
    func completeOwnerSetup(_ request: OwnerSetupRequest, timeout: TimeInterval) async throws -> OwnerSetupReceipt {
        _ = try await core.completeOwnerSetup(request,timeout:timeout)
        throw URLError(.networkConnectionLost)
    }
    func cancelOwnerSetup(_ identity: OwnerSetupIdentity, timeout: TimeInterval) async throws -> OwnerSetupCancellation { try await core.cancelOwnerSetup(identity,timeout:timeout) }
}

@Test @MainActor func ownerSetupRecoveryInspectsWithoutActivationAndCancelsTheExactPreparedCredential() async throws {
    let directory = recoveryDirectory()
    defer { try? FileManager.default.removeItem(at:directory) }
    let credentials = CancellationCredentials(), control = CancellationControl()
    let storage = try DeviceStorage(directory:directory,credentials:credentials)
    let coordinator = try recoveryCoordinator(storage,control)
    #expect(try coordinator.pendingSetup() == nil)
    #expect(credentials.reads == 0 && control.begins == 0)
    await #expect(throws:(any Error).self) { try await coordinator.connect() }
    let request = try #require(control.completions.first), reads = credentials.reads
    let pending = try #require(try coordinator.pendingSetup())
    #expect(pending.identity == (try OwnerSetupIdentity(installationID:request.installationID,setupID:request.setupID,deviceID:request.deviceID)))
    #expect(pending.endpoint == recoveryEndpoint && pending.phase == .ready)
    #expect(credentials.reads == reads && control.begins == 1 && control.completions.count == 1)
    control.onCancel = { let record = try recoveryRecord(directory); #expect(record["phase"] as? String == "canceling") }
    credentials.onRemove = { let record = try recoveryRecord(directory); #expect(record["phase"] as? String == "canceled") }
    #expect(try await coordinator.cancelPendingSetup(pending.identity) == .revoked(pending.identity))
    #expect(try coordinator.pendingSetup() == nil)
    #expect(credentials.tokens.isEmpty && credentials.removals == 1)
    #expect(try storage.connection() == nil)
    #expect(try await coordinator.cancelPendingSetup(pending.identity) == nil)
    #expect(control.cancellations == [pending.identity])
}

@Test(arguments:["lost","unavailable","foreign-proof"]) @MainActor
func ownerSetupRecoveryKeepsCancellationIntentAcrossFailuresAndReopen(failure: String) async throws {
    let directory = recoveryDirectory()
    defer { try? FileManager.default.removeItem(at:directory) }
    let credentials = CancellationCredentials(), control = CancellationControl()
    let storage = try DeviceStorage(directory:directory,credentials:credentials)
    let coordinator = try recoveryCoordinator(storage,control)
    await #expect(throws:(any Error).self) { try await coordinator.connect() }
    let pending = try #require(try coordinator.pendingSetup()), tokens = credentials.tokens
    control.loseCancellationReply = failure == "lost"
    control.cancellationFailure = failure == "unavailable"
    control.wrongCancellationProof = failure == "foreign-proof"
    await #expect(throws:(any Error).self) { try await coordinator.cancelPendingSetup(pending.identity) }
    #expect(try coordinator.pendingSetup()?.phase == .canceling)
    #expect(credentials.tokens == tokens && credentials.removals == 0)
    let reopened = try recoveryCoordinator(DeviceStorage(directory:directory,credentials:credentials),control)
    await #expect(throws:OwnerSetupPreparationError.self) { try await reopened.connect() }
    #expect(control.completions.count == 1 && credentials.saves == 1)
    control.cancellationFailure = false; control.wrongCancellationProof = false
    #expect(try await reopened.cancelPendingSetup(pending.identity) == .revoked(pending.identity))
    #expect(control.cancellations == [pending.identity,pending.identity])
    #expect(credentials.tokens.isEmpty)
    #expect(try reopened.pendingSetup() == nil)
}

@Test(arguments:["missing","altered","denied"]) @MainActor
func ownerSetupRecoveryHandlesMissingCredentialsAndPreservesAlteredOrUnreadableOnes(failure: String) async throws {
    let directory = recoveryDirectory()
    defer { try? FileManager.default.removeItem(at:directory) }
    let credentials = CancellationCredentials(), control = CancellationControl()
    let storage = try DeviceStorage(directory:directory,credentials:credentials)
    let coordinator = try recoveryCoordinator(storage,control)
    await #expect(throws:(any Error).self) { try await coordinator.connect() }
    let pending = try #require(try coordinator.pendingSetup()), original = credentials.tokens
    if failure == "missing" { credentials.tokens.removeAll() }
    if failure == "altered" { credentials.tokens = original.mapValues { _ in String(repeating:"B",count:42) + "A" } }
    if failure == "denied" { credentials.readFailure = errSecInteractionNotAllowed }
    if failure == "missing" {
        #expect(try await coordinator.cancelPendingSetup(pending.identity) == .revoked(pending.identity))
        #expect(credentials.removals == 0)
    } else {
        let kept = credentials.tokens
        await #expect(throws:(any Error).self) { try await coordinator.cancelPendingSetup(pending.identity) }
        #expect(try coordinator.pendingSetup()?.phase == .canceled)
        #expect(credentials.tokens == kept && credentials.removals == 0)
        await #expect(throws:OwnerSetupPreparationError.self) { try await coordinator.connect() }
        credentials.readFailure = nil; credentials.tokens = original
        control.cancellationFailure = true
        let reopened = try recoveryCoordinator(DeviceStorage(directory:directory,credentials:credentials),control)
        #expect(try await reopened.cancelPendingSetup(pending.identity) == .revoked(pending.identity))
    }
    #expect(control.cancellations == [pending.identity] && credentials.saves == 1)
    #expect(credentials.tokens.isEmpty)
}

@Test(arguments:["before","after","ignored"]) @MainActor
func ownerSetupRecoveryKeepsConfirmedProofUntilCredentialRemovalIsVerified(failure: String) async throws {
    let directory = recoveryDirectory()
    defer { try? FileManager.default.removeItem(at:directory) }
    let credentials = CancellationCredentials(), control = CancellationControl()
    let storage = try DeviceStorage(directory:directory,credentials:credentials)
    let coordinator = try recoveryCoordinator(storage,control)
    await #expect(throws:(any Error).self) { try await coordinator.connect() }
    let pending = try #require(try coordinator.pendingSetup())
    credentials.tokens["unrelated/credential"] = "retained-synthetic-value"
    credentials.removalFailure = failure
    await #expect(throws:(any Error).self) { try await coordinator.cancelPendingSetup(pending.identity) }
    #expect(try coordinator.pendingSetup()?.phase == .canceled)
    #expect(credentials.tokens["unrelated/credential"] == "retained-synthetic-value")
    await #expect(throws:OwnerSetupPreparationError.self) { try await coordinator.connect() }
    control.cancellationFailure = true; credentials.removalFailure = nil
    let reopened = try recoveryCoordinator(DeviceStorage(directory:directory,credentials:credentials),control)
    #expect(try await reopened.cancelPendingSetup(pending.identity) == .revoked(pending.identity))
    #expect(credentials.tokens == ["unrelated/credential":"retained-synthetic-value"])
    #expect(credentials.removals == (failure == "after" ? 1 : 2))
    #expect(control.cancellations == [pending.identity] && control.completions.count == 1)
}

@Test(arguments:["before","after"]) @MainActor
func ownerSetupRecoveryCancelsInterruptedCredentialPreparationWithoutCompletingIt(failure: String) async throws {
    let directory = recoveryDirectory()
    defer { try? FileManager.default.removeItem(at:directory) }
    let credentials = CancellationCredentials(), control = CancellationControl()
    credentials.saveFailure = failure
    let storage = try DeviceStorage(directory:directory,credentials:credentials)
    let coordinator = try recoveryCoordinator(storage,control)
    await #expect(throws:(any Error).self) { try await coordinator.connect() }
    let pending = try #require(try coordinator.pendingSetup())
    #expect(pending.phase == .preparing)
    credentials.onRemove = {
        let record = try recoveryRecord(directory)
        #expect(record["phase"] as? String == "canceled")
        #expect((record["credentialSHA256"] as? String)?.count == 64)
    }
    #expect(try await coordinator.cancelPendingSetup(pending.identity) == .notPaired(pending.identity))
    #expect(control.completions.isEmpty && credentials.saves == 1)
    #expect(credentials.tokens.isEmpty && credentials.removals == (failure == "before" ? 0 : 1))
}

@Test @MainActor func ownerSetupRecoveryRefusesStaleIdentityForeignHostsAndConnections() async throws {
    let directory = recoveryDirectory()
    defer { try? FileManager.default.removeItem(at:directory) }
    let credentials = CancellationCredentials(), control = CancellationControl()
    let storage = try DeviceStorage(directory:directory,credentials:credentials)
    let coordinator = try recoveryCoordinator(storage,control)
    await #expect(throws:(any Error).self) { try await coordinator.connect() }
    let pending = try #require(try coordinator.pendingSetup())
    let file = directory.appendingPathComponent("owner-setup.json"), before = try Data(contentsOf:file)
    for identity in [try OwnerSetupIdentity(installationID:"foreign",setupID:pending.identity.setupID,deviceID:pending.identity.deviceID),try OwnerSetupIdentity(installationID:pending.identity.installationID,setupID:"foreign",deviceID:pending.identity.deviceID),try OwnerSetupIdentity(installationID:pending.identity.installationID,setupID:pending.identity.setupID,deviceID:"foreign")] {
        await #expect(throws:OwnerSetupPreparationError.self) { try await coordinator.cancelPendingSetup(identity) }
    }
    let otherHost = try recoveryCoordinator(storage,control,endpoint:.macHost(serviceIdentifier:"io.fort.other.host",signingTeam:"AAAAAAAAAA"))
    #expect(throws:OwnerSetupPreparationError.self) { try otherHost.pendingSetup() }
    await #expect(throws:OwnerSetupPreparationError.self) { try await otherHost.cancelPendingSetup(pending.identity) }
    let foreign = try DeviceConnection(endpoint:recoveryEndpoint,installationID:"foreign",deviceID:"foreign")
    try storage.save(foreign,token:"unrelated-synthetic-value")
    let tokens = credentials.tokens
    #expect(throws:OwnerSetupPreparationError.self) { try coordinator.pendingSetup() }
    await #expect(throws:OwnerSetupPreparationError.self) { try await coordinator.cancelPendingSetup(pending.identity) }
    #expect(try Data(contentsOf:file) == before)
    #expect(try storage.connection() == foreign)
    #expect(credentials.tokens == tokens && credentials.removals == 0 && control.cancellations.isEmpty)
}

@Test @MainActor func ownerSetupRecoveryRequiresCancellationCapabilityAndDoesNotDiscardCompletedConnections() async throws {
    let directory = recoveryDirectory()
    defer { try? FileManager.default.removeItem(at:directory) }
    let credentials = CancellationCredentials(), control = CancellationControl()
    let storage = try DeviceStorage(directory:directory,credentials:credentials)
    let coordinator = try recoveryCoordinator(storage,control)
    await #expect(throws:(any Error).self) { try await coordinator.connect() }
    let pending = try #require(try coordinator.pendingSetup()), tokens = credentials.tokens
    let unsupported = try recoveryCoordinator(storage,SetupOnlyControl(base:control))
    await #expect(throws:(any Error).self) { try await unsupported.cancelPendingSetup(pending.identity) }
    #expect(try coordinator.pendingSetup()?.phase == .ready)
    #expect(credentials.tokens == tokens && control.cancellations.isEmpty)
    let connection = try await coordinator.connect()
    #expect(try coordinator.pendingSetup() == nil)
    #expect(try await coordinator.cancelPendingSetup(pending.identity) == nil)
    #expect(try storage.connection() == connection)
    #expect(credentials.tokens == tokens && credentials.removals == 0 && control.cancellations.isEmpty)
}

@Test(arguments:[false,true]) @MainActor
func ownerSetupRecoveryFinishesPartialConnectionPersistenceAndKeepsHistory(retryWriteFailure: Bool) async throws {
    let directory = recoveryDirectory()
    defer { try? FileManager.default.removeItem(at:directory) }
    let credentials = CancellationCredentials(), control = CancellationControl()
    let storage = try DeviceStorage(directory:directory,credentials:credentials)
    let coordinator = try recoveryCoordinator(storage,control)
    await #expect(throws:(any Error).self) { try await coordinator.connect() }
    let pending = try #require(try coordinator.pendingSetup()), request = try #require(control.completions.first)
    let connection = try DeviceConnection(endpoint:recoveryEndpoint,installationID:request.installationID,deviceID:request.deviceID)
    try storage.save(connection,token:request.deviceToken)
    let history = directory.appendingPathComponent("retained-history.json"), retained = Data("synthetic history remains".utf8)
    try retained.write(to:history)
    let removal = directory.appendingPathComponent("pairing-removal.json")
    if retryWriteFailure {
        try FileManager.default.createDirectory(at:removal,withIntermediateDirectories:false)
        await #expect(throws:(any Error).self) { try await coordinator.cancelPendingSetup(pending.identity) }
        #expect(try coordinator.pendingSetup()?.phase == .canceled)
        #expect(credentials.removals == 0 && !credentials.tokens.isEmpty)
        #expect(try storage.connection() == connection)
        try FileManager.default.removeItem(at:removal)
        control.cancellationFailure = true
    }
    let reopened = try recoveryCoordinator(DeviceStorage(directory:directory,credentials:credentials),control)
    #expect(try await reopened.cancelPendingSetup(pending.identity) == .revoked(pending.identity))
    #expect(try storage.connection() == nil)
    #expect(try storage.disconnectedConnection() == connection)
    #expect(try storage.pairingRemoval() == PairingRemovalIntent(connection:connection,state:.confirmed))
    #expect(try Data(contentsOf:history) == retained)
    #expect(control.cancellations == [pending.identity] && credentials.tokens.isEmpty)
}

@Test(arguments:["future","invalid-hash","foreign-proof","pending-link","removal-link","disconnected-link"]) @MainActor
func ownerSetupRecoveryPreservesMalformedOrUnsafeRecords(kind: String) async throws {
    let directory = recoveryDirectory()
    defer { try? FileManager.default.removeItem(at:directory) }
    let credentials = CancellationCredentials(), control = CancellationControl()
    let storage = try DeviceStorage(directory:directory,credentials:credentials)
    let coordinator = try recoveryCoordinator(storage,control)
    await #expect(throws:(any Error).self) { try await coordinator.connect() }
    let pending = try #require(try coordinator.pendingSetup()), tokens = credentials.tokens
    var record = try recoveryRecord(directory)
    let protected = directory.appendingPathComponent("protected-record.json")
    let contents = try JSONSerialization.data(withJSONObject:record)
    try contents.write(to:protected)
    if kind == "future" { record["version"] = 2; try writeRecoveryRecord(record,directory:directory) }
    if kind == "invalid-hash" { record["credentialSHA256"] = "invalid"; try writeRecoveryRecord(record,directory:directory) }
    if kind == "foreign-proof" {
        record["phase"] = "canceled"
        record["cancellation"] = ["installationId":pending.identity.installationID,"setupId":"foreign","deviceId":pending.identity.deviceID,"outcome":"revoked","runtimeStopped":true]
        try writeRecoveryRecord(record,directory:directory)
    }
    if kind.hasSuffix("-link") {
        let filename = kind == "pending-link" ? "owner-setup.json" : kind == "removal-link" ? "pairing-removal.json" : "disconnected.json"
        let link = directory.appendingPathComponent(filename)
        if kind == "pending-link" { try FileManager.default.removeItem(at:link) }
        else {
            let request = try #require(control.completions.first)
            try storage.save(DeviceConnection(endpoint:recoveryEndpoint,installationID:request.installationID,deviceID:request.deviceID),token:request.deviceToken)
        }
        try FileManager.default.createSymbolicLink(at:link,withDestinationURL:protected)
    }
    await #expect(throws:(any Error).self) { try await coordinator.cancelPendingSetup(pending.identity) }
    #expect(try Data(contentsOf:protected) == contents)
    #expect(credentials.tokens == tokens && credentials.removals == 0)
    #expect(control.cancellations.count == (kind == "removal-link" || kind == "disconnected-link" ? 1 : 0))
}

@Test @MainActor func ownerSetupRecoverySerializesConcurrentActionsAndRetainsCanceledIntent() async throws {
    let directory = recoveryDirectory()
    defer { try? FileManager.default.removeItem(at:directory) }
    let credentials = CancellationCredentials(), control = CancellationControl()
    let storage = try DeviceStorage(directory:directory,credentials:credentials)
    let coordinator = try recoveryCoordinator(storage,control)
    await #expect(throws:(any Error).self) { try await coordinator.connect() }
    let pending = try #require(try coordinator.pendingSetup()), tokens = credentials.tokens
    control.pauseCancellation = true
    let work = Task { try await coordinator.cancelPendingSetup(pending.identity) }
    for _ in 0..<100 where !control.waiting { try await Task.sleep(for:.milliseconds(5)) }
    #expect(control.waiting)
    let other = try recoveryCoordinator(DeviceStorage(directory:directory,credentials:credentials),control)
    #expect(throws:OwnerSetupPreparationError.self) { try other.pendingSetup() }
    await #expect(throws:OwnerSetupPreparationError.self) { try await other.connect() }
    await #expect(throws:OwnerSetupPreparationError.self) { try await other.cancelPendingSetup(pending.identity) }
    work.cancel()
    await #expect(throws:CancellationError.self) { try await work.value }
    #expect(try other.pendingSetup()?.phase == .canceling)
    #expect(credentials.tokens == tokens && credentials.removals == 0)
    control.pauseCancellation = false
    #expect(try await other.cancelPendingSetup(pending.identity) == .revoked(pending.identity))
    #expect(control.cancellations == [pending.identity,pending.identity])
    #expect(control.completions.count == 1 && credentials.saves == 1)
}

@Test(arguments:[false,true]) @MainActor
func ownerSetupRecoveryCancelsACommittedCompiledCorePairingAfterBothSidesReopen(missingCredential: Bool) async throws {
    let directory = recoveryDirectory()
    defer { try? FileManager.default.removeItem(at:directory) }
    let credentials = CancellationCredentials()
    let storage = try DeviceStorage(directory:directory,credentials:credentials)
    let core = try await setupCore(directory,enroll:false)
    let pending: PendingOwnerSetup
    let token: String
    do {
        let coordinator = try recoveryCoordinator(storage,UncertainSetupCore(core:core))
        await #expect(throws:URLError.self) { try await coordinator.connect() }
        pending = try #require(try coordinator.pendingSetup())
        token = try #require(credentials.tokens.values.first)
        try await stopSetupCore(core,directory:directory)
    } catch { _ = try? await core.close(); throw error }
    let reopenedCore = try await setupCore(directory,enroll:false)
    do {
        if missingCredential { credentials.tokens.removeAll() }
        let coordinator = try recoveryCoordinator(DeviceStorage(directory:directory,credentials:credentials),reopenedCore)
        #expect(try await coordinator.cancelPendingSetup(pending.identity) == .revoked(pending.identity))
        #expect(try coordinator.pendingSetup() == nil)
        #expect(credentials.tokens.isEmpty && credentials.saves == 1)
        await #expect(throws:(any Error).self) { try await FortClient(installationID:pending.identity.installationID,token:token,transport:reopenedCore).installation() }
        await #expect(throws:(any Error).self) { try await reopenedCore.beginOwnerSetup() }
        try await stopSetupCore(reopenedCore,directory:directory)
    } catch { _ = try? await reopenedCore.close(); throw error }
}

@Test @MainActor func ownerSetupRecoveryDoesNotContactHostWhenIntentCannotBeSaved() async throws {
    let directory = recoveryDirectory()
    defer {
        try? FileManager.default.setAttributes([.posixPermissions:0o700],ofItemAtPath:directory.path)
        try? FileManager.default.removeItem(at:directory)
    }
    let credentials = CancellationCredentials(), control = CancellationControl()
    let storage = try DeviceStorage(directory:directory,credentials:credentials)
    let coordinator = try recoveryCoordinator(storage,control)
    await #expect(throws:(any Error).self) { try await coordinator.connect() }
    let pending = try #require(try coordinator.pendingSetup()), tokens = credentials.tokens
    try FileManager.default.setAttributes([.posixPermissions:0o500],ofItemAtPath:directory.path)
    await #expect(throws:(any Error).self) { try await coordinator.cancelPendingSetup(pending.identity) }
    #expect(control.cancellations.isEmpty && credentials.tokens == tokens)
    #expect(try coordinator.pendingSetup()?.phase == .ready)
    try FileManager.default.setAttributes([.posixPermissions:0o700],ofItemAtPath:directory.path)
    #expect(try await coordinator.cancelPendingSetup(pending.identity) == .revoked(pending.identity))
}

@Test(arguments:["reply","removal"]) @MainActor
func ownerSetupRecoveryPreservesRecordsChangedDuringHostOrCredentialOperations(stage: String) async throws {
    let directory = recoveryDirectory()
    defer { try? FileManager.default.removeItem(at:directory) }
    let credentials = CancellationCredentials(), control = CancellationControl()
    let storage = try DeviceStorage(directory:directory,credentials:credentials)
    let coordinator = try recoveryCoordinator(storage,control)
    await #expect(throws:(any Error).self) { try await coordinator.connect() }
    let pending = try #require(try coordinator.pendingSetup())
    var retained: [String:Any]?
    let change: () throws -> Void = {
        retained = try recoveryRecord(directory)
        var changed = try #require(retained)
        changed["version"] = 2
        try writeRecoveryRecord(changed,directory:directory)
    }
    if stage == "reply" { control.onCancel = change } else { credentials.onRemove = change }
    await #expect(throws:(any Error).self) { try await coordinator.cancelPendingSetup(pending.identity) }
    #expect(try recoveryRecord(directory)["version"] as? Int == 2)
    #expect(credentials.removals == (stage == "reply" ? 0 : 1))
    try writeRecoveryRecord(#require(retained),directory:directory)
    control.onCancel = nil; credentials.onRemove = nil
    let reopened = try recoveryCoordinator(DeviceStorage(directory:directory,credentials:credentials),control)
    #expect(try await reopened.cancelPendingSetup(pending.identity) == .revoked(pending.identity))
    #expect(control.cancellations.count == (stage == "reply" ? 2 : 1))
    #expect(credentials.tokens.isEmpty)
}

@Test @MainActor func ownerSetupRecoveryMeetsSyntheticFileDurabilityBudget() async throws {
    var samples: [Double] = []
    for _ in 0..<20 {
        let directory = recoveryDirectory()
        defer { try? FileManager.default.removeItem(at:directory) }
        let credentials = CancellationCredentials(), control = CancellationControl()
        let storage = try DeviceStorage(directory:directory,credentials:credentials)
        let coordinator = try recoveryCoordinator(storage,control)
        await #expect(throws:(any Error).self) { try await coordinator.connect() }
        let pending = try #require(try coordinator.pendingSetup())
        let started = ContinuousClock.now
        #expect(try await coordinator.cancelPendingSetup(pending.identity) == .revoked(pending.identity))
        let elapsed = started.duration(to:.now).components
        samples.append(Double(elapsed.seconds) * 1000 + Double(elapsed.attoseconds) / 1e15)
        #expect(try coordinator.pendingSetup() == nil)
        #expect(credentials.tokens.isEmpty)
    }
    let p95 = samples.sorted()[18]
    print("FORT_OWNER_SETUP_CANCEL_BUDGET " + String(decoding:try JSONSerialization.data(withJSONObject:["samplesMs":samples,"p95Ms":p95,"budgetMs":500]),as:UTF8.self))
    #expect(p95 <= 500)
}

@Test(arguments:["pairing-removal.json","disconnected.json"]) @MainActor
func ownerSetupRecoveryPreservesHistoryReferencesForAnotherPairing(filename: String) async throws {
    let directory = recoveryDirectory()
    defer { try? FileManager.default.removeItem(at:directory) }
    let credentials = CancellationCredentials(), control = CancellationControl()
    let storage = try DeviceStorage(directory:directory,credentials:credentials)
    let coordinator = try recoveryCoordinator(storage,control)
    await #expect(throws:(any Error).self) { try await coordinator.connect() }
    let request = try #require(control.completions.first), pending = try #require(try coordinator.pendingSetup())
    let connection = try DeviceConnection(endpoint:recoveryEndpoint,installationID:request.installationID,deviceID:request.deviceID)
    try storage.save(connection,token:request.deviceToken)
    let foreign = try DeviceConnection(endpoint:recoveryEndpoint,installationID:"prior-installation",deviceID:"prior-device")
    let record = filename == "disconnected.json" ? try JSONEncoder().encode(foreign) : try JSONEncoder().encode(PairingRemovalIntent(connection:foreign,state:.confirmed))
    let file = directory.appendingPathComponent(filename)
    try record.write(to:file)
    try FileManager.default.setAttributes([.posixPermissions:0o600],ofItemAtPath:file.path)
    await #expect(throws:OwnerSetupPreparationError.self) { try await coordinator.cancelPendingSetup(pending.identity) }
    #expect(try Data(contentsOf:file) == record)
    #expect(try storage.connection() == connection)
    #expect(credentials.removals == 0 && credentials.tokens.values.contains(request.deviceToken))
    #expect(try coordinator.pendingSetup()?.phase == .canceled)
}

@Test @MainActor func ownerSetupRecoveryReleasesItsLeaseWhileADuplicateDescriptorRemainsOpen() async throws {
    let directory = recoveryDirectory()
    defer { try? FileManager.default.removeItem(at:directory) }
    let credentials = CancellationCredentials(), control = CancellationControl()
    let storage = try DeviceStorage(directory:directory,credentials:credentials)
    let coordinator = try recoveryCoordinator(storage,control)
    await #expect(throws:(any Error).self) { try await coordinator.connect() }
    let pending = try #require(try coordinator.pendingSetup())
    var duplicate: Int32 = -1
    defer { if duplicate >= 0 { Darwin.close(duplicate) } }
    control.onCancel = {
        var expected = stat()
        #expect(lstat(directory.appendingPathComponent("owner-setup.lock").path,&expected) == 0)
        for descriptor in 0..<min(getdtablesize(),8192) {
            var candidate = stat()
            if fstat(descriptor,&candidate) == 0, candidate.st_dev == expected.st_dev, candidate.st_ino == expected.st_ino {
                duplicate = fcntl(descriptor,F_DUPFD_CLOEXEC,0)
                break
            }
        }
        #expect(duplicate >= 0)
        #expect(throws:OwnerSetupPreparationError.self) { try coordinator.pendingSetup() }
    }
    #expect(try await coordinator.cancelPendingSetup(pending.identity) == .revoked(pending.identity))
    #expect(duplicate >= 0 && fcntl(duplicate,F_GETFD) >= 0)
    #expect(try coordinator.pendingSetup() == nil)
    #expect(try await coordinator.cancelPendingSetup(pending.identity) == nil)
}
#endif
