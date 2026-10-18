#if os(macOS)
import Foundation
import Darwin
import Security
import Testing
@testable import FortKit

private actor RemovalStartup: MacHostStartupBackend {
    var current: MacHostStartupStatus = .enabled
    var removals = 0
    var registrations = 0
    private var reenableAfterInspection = false
    func reenableOnNextInspection() { reenableAfterInspection = true }
    func status() -> MacHostStartupStatus {
        let result = current
        if reenableAfterInspection { current = .enabled; reenableAfterInspection = false }
        return result
    }
    func register() throws { registrations += 1; throw MacHostStartupError.unavailable }
    private var beforeRemoval: (@Sendable () async throws -> Void)?
    private var held = false
    private var waiter: CheckedContinuation<Void,Never>?
    var waiting: Bool { waiter != nil }
    func setStatus(_ value: MacHostStartupStatus) { current = value }
    func beforeUnregister(_ action: @escaping @Sendable () async throws -> Void) { beforeRemoval = action }
    func holdUnregistration() { held = true }
    func release() { held = false; waiter?.resume(); waiter = nil }
    func unregister() async throws {
        removals += 1
        try await beforeRemoval?()
        current = .notRegistered
        if held { await withCheckedContinuation { waiter = $0 } }
    }
}
private actor RemovalRecovery {
    let observation: MacHostRecoveryObservation
    var calls = 0
    init(_ observation: MacHostRecoveryObservation) { self.observation = observation }
    func recover(_ request: MacOwnerRecoveryRequest) throws -> MacHostRecoveryObservation {
        calls += 1
        try observation.receipt.validate(for:request)
        return observation
    }
}
private final class RemovalChild {
    let process = Process()
    private let input = Pipe()
    init() throws {
        process.executableURL = URL(fileURLWithPath:"/bin/cat")
        process.arguments = []; process.environment = [:]
        process.standardInput = input; process.standardOutput = FileHandle.nullDevice; process.standardError = FileHandle.nullDevice
        try process.run()
    }
    func finish() throws { try input.fileHandleForWriting.close(); process.waitUntilExit() }
    deinit { try? input.fileHandleForWriting.close(); process.waitUntilExit() }
}

@Test @MainActor func hostRemovalRestartsAfterUnregistrationWithoutReactivatingTheHost() async throws {
    let directory = FileManager.default.temporaryDirectory.appendingPathComponent("fort-host-removal-" + UUID().uuidString)
    try FileManager.default.createDirectory(at:directory,withIntermediateDirectories:false,attributes:[.posixPermissions:0o700])
    defer { try? FileManager.default.removeItem(at:directory) }
    let endpoint = try MacAgentEndpoint(serviceIdentifier:"io.fort.synthetic.agent",signingTeam:"AAAAAAAAAA",appGroup:"group.io.fort.synthetic")
    let connection = try DeviceConnection(endpoint:.macAgent(endpoint),installationID:"installation",deviceID:"device")
    let connectionFile = directory.appendingPathComponent("connection.json")
    try JSONEncoder().encode(connection).write(to:connectionFile)
    try FileManager.default.setAttributes([.posixPermissions:0o600],ofItemAtPath:connectionFile.path)
    let historyFile = directory.appendingPathComponent("history-marker")
    let history = Data("retained synthetic history".utf8)
    try history.write(to:historyFile)
    let child = try RemovalChild()
    let observed = try MacHostProcess.observe(processID:child.process.processIdentifier,effectiveUserID:geteuid())
    let receipt = MacOwnerRecoveryReceipt(installationID:connection.installationID,deviceID:connection.deviceID,runtimeStopped:true,pairingRevoked:true,coreStopped:true)
    let recovery = RemovalRecovery(MacHostRecoveryObservation(receipt:receipt,host:observed)), backend = RemovalStartup()
    let startup = try MacHostStartup(endpoint:endpoint,backend:backend)
    let first = try MacHostRemoval(connection:connection,directory:directory,applicationURL:URL(fileURLWithPath:"/synthetic/Fort.app"),verifyBundle:{},startup:startup,recover:{ try await recovery.recover($0) },exitTimeout:0.03)
    await #expect(throws:URLError(.timedOut)) { try await first.stopAndRevoke() }
    #expect(child.process.isRunning)
    #expect(await backend.removals == 1)
    #expect(await recovery.calls == 1)
    try child.finish()
    let restarted = try MacHostRemoval(connection:connection,directory:directory,applicationURL:URL(fileURLWithPath:"/synthetic/Fort.app"),verifyBundle:{},startup:MacHostStartup(endpoint:endpoint,backend:backend),recover:{ try await recovery.recover($0) },exitTimeout:1)
    let stopped = try await restarted.stopAndRevoke()
    #expect(stopped.hostStopped && stopped.pairingRevoked && stopped.startupUnregistered)
    _ = try await restarted.stopAndRevoke()
    #expect(await backend.registrations == 0)
    #expect(await backend.removals == 1)
    #expect(await recovery.calls == 1)
    await backend.reenableOnNextInspection()
    await #expect(throws:OwnerLifecycleError.self) { try await restarted.stopAndRevoke() }
    #expect(await backend.removals == 1)
    #expect(await backend.current == .enabled)
    #expect(try Data(contentsOf:historyFile) == history)
    #expect(try JSONDecoder().decode(DeviceConnection.self,from:Data(contentsOf:connectionFile)) == connection)
}

@MainActor private final class RemovalFixture {
    let directory: URL
    let endpoint: MacAgentEndpoint
    let connection: DeviceConnection
    let child: RemovalChild
    let recovery: RemovalRecovery
    let backend = RemovalStartup()
    var progressURL: URL { directory.appendingPathComponent("host-removal.json") }
    init() throws {
        directory = FileManager.default.temporaryDirectory.appendingPathComponent("fort-host-removal-" + UUID().uuidString)
        try FileManager.default.createDirectory(at:directory,withIntermediateDirectories:false,attributes:[.posixPermissions:0o700])
        endpoint = try MacAgentEndpoint(serviceIdentifier:"io.fort.synthetic.agent",signingTeam:"AAAAAAAAAA",appGroup:"group.io.fort.synthetic")
        connection = try DeviceConnection(endpoint:.macAgent(endpoint),installationID:"installation",deviceID:"device")
        let connectionFile = directory.appendingPathComponent("connection.json")
        try JSONEncoder().encode(connection).write(to:connectionFile)
        try FileManager.default.setAttributes([.posixPermissions:0o600],ofItemAtPath:connectionFile.path)
        child = try RemovalChild()
        let observed = try MacHostProcess.observe(processID:child.process.processIdentifier,effectiveUserID:geteuid())
        recovery = RemovalRecovery(MacHostRecoveryObservation(receipt:MacOwnerRecoveryReceipt(installationID:connection.installationID,deviceID:connection.deviceID,runtimeStopped:true,pairingRevoked:true,coreStopped:true),host:observed))
    }
    func removal(timeout: TimeInterval = 0.03, verifyBundle: @escaping @MainActor () async throws -> Void = {}, recover: (@Sendable (MacOwnerRecoveryRequest) async throws -> MacHostRecoveryObservation)? = nil, historyActions: MacHistoryRemovalActions? = nil) throws -> MacHostRemoval {
        let recovery = self.recovery
        return try MacHostRemoval(connection:connection,directory:directory,applicationURL:URL(fileURLWithPath:"/synthetic/Fort.app"),verifyBundle:verifyBundle,startup:MacHostStartup(endpoint:endpoint,backend:backend),recover:recover ?? { try await recovery.recover($0) },exitTimeout:timeout,historyActions:historyActions)
    }
    func clean() { try? child.finish(); try? FileManager.default.removeItem(at:directory) }
}
private func removalACL(_ url: URL, add: Bool) throws {
    let process = Process()
    process.executableURL = URL(fileURLWithPath:"/bin/chmod")
    process.environment = [:]
    process.arguments = add ? ["+a","everyone allow read",url.path] : ["-N",url.path]
    try process.run(); process.waitUntilExit()
    #expect(process.terminationStatus == 0)
}

@Test @MainActor func hostRemovalRefusesDirectoryLockAndRecordACLsWithoutRepair() async throws {
    for target in ["directory","owner-setup.lock","connection.json","host-removal.json"] {
        let fixture = try RemovalFixture()
        defer { fixture.clean() }
        let file = target == "directory" ? fixture.directory : fixture.directory.appendingPathComponent(target)
        if target == "owner-setup.lock" || target == "host-removal.json" {
            if target == "owner-setup.lock" { try Data().write(to:file) }
            else {
                await #expect(throws:URLError(.timedOut)) { try await fixture.removal().stopAndRevoke() }
            }
            try FileManager.default.setAttributes([.posixPermissions:0o600],ofItemAtPath:file.path)
        }
        try removalACL(file,add:true)
        defer { try? removalACL(file,add:false) }
        let calls = await fixture.recovery.calls, removals = await fixture.backend.removals
        await #expect(throws:ClientError.self) { try await fixture.removal().stopAndRevoke() }
        #expect(await fixture.recovery.calls == calls)
        #expect(await fixture.backend.removals == removals)
        var info = stat()
        #expect(lstat(file.path,&info) == 0)
        let acl = try #require(acl_get_file(file.path,ACL_TYPE_EXTENDED))
        defer { acl_free(UnsafeMutableRawPointer(acl)) }
        var entry: acl_entry_t?
        #expect(acl_get_entry(acl,Int32(ACL_FIRST_ENTRY.rawValue),&entry) == 0)
    }
}

private func removalPhase(_ directory: URL) throws -> String? {
    let bytes = try Data(contentsOf:directory.appendingPathComponent("host-removal.json"))
    return (try JSONSerialization.jsonObject(with:bytes) as? [String:Any])?["phase"] as? String
}

@Test @MainActor func hostRemovalPersistsBeforeEachExternalActionAndKeepsConnectionMetadata() async throws {
    let fixture = try RemovalFixture()
    defer { fixture.clean() }
    let directory = fixture.directory, recovery = fixture.recovery
    await fixture.backend.beforeUnregister {
        #expect(try removalPhase(directory) == "recovered")
        try await MainActor.run { try fixture.child.finish() }
    }
    let removal = try fixture.removal(timeout:1,recover:{ request in
        #expect(try removalPhase(directory) == "requested")
        return try await recovery.recover(request)
    })
    let result = try await removal.stopAndRevoke()
    #expect(result.hostStopped && result.pairingRevoked && result.startupUnregistered)
    #expect(try removalPhase(directory) == "confirmed")
    #expect(await recovery.calls == 1)
    #expect(await fixture.backend.removals == 1)
    #expect(await fixture.backend.registrations == 0)
    let fields = try #require(try JSONSerialization.jsonObject(with:Data(contentsOf:fixture.progressURL)) as? [String:Any])
    #expect(Set(fields.keys) == ["version","connection","phase","receipt","host"])
    let host = try #require(fields["host"] as? [String:Any])
    #expect(Set(host.keys) == ["processID","effectiveUserID","startedSeconds","startedMicroseconds"])
    var permissions = stat()
    #expect(lstat(fixture.progressURL.path,&permissions) == 0 && permissions.st_mode & 0o777 == 0o600)
    #expect(try JSONDecoder().decode(DeviceConnection.self,from:Data(contentsOf:directory.appendingPathComponent("connection.json"))) == fixture.connection)
}

@Test @MainActor func hostRemovalCancellationWaitsForStartupCallbackAndRetainsRecoveryForRetry() async throws {
    let fixture = try RemovalFixture()
    defer { fixture.clean() }
    await fixture.backend.holdUnregistration()
    let removal = try fixture.removal(timeout:1)
    let work = Task { try await removal.stopAndRevoke() }
    for _ in 0..<200 where await !fixture.backend.waiting { try await Task.sleep(for:.milliseconds(5)) }
    #expect(await fixture.backend.waiting)
    #expect(try removalPhase(fixture.directory) == "recovered")
    await #expect(throws:MacHostRemovalError.self) { try await removal.stopAndRevoke() }
    await #expect(throws:OwnerSetupPreparationError.self) { try await fixture.removal().stopAndRevoke() }
    work.cancel()
    #expect(await fixture.backend.waiting)
    try fixture.child.finish()
    await fixture.backend.release()
    await #expect(throws:CancellationError.self) { try await work.value }
    #expect(try removalPhase(fixture.directory) == "recovered")
    _ = try await fixture.removal(timeout:1).stopAndRevoke()
    #expect(try removalPhase(fixture.directory) == "confirmed")
    #expect(await fixture.recovery.calls == 1)
    #expect(await fixture.backend.removals == 1)
}

@Test @MainActor func hostRemovalDoesNotInventRevocationForAnUnregisteredOrUnavailableHost() async throws {
    for status in [MacHostStartupStatus.notRegistered,.notFound,.unsupported,.unknown] {
        let fixture = try RemovalFixture()
        defer { fixture.clean() }
        await fixture.backend.setStatus(status)
        await #expect(throws:OwnerLifecycleError.self) { try await fixture.removal().stopAndRevoke() }
        #expect(try removalPhase(fixture.directory) == "requested")
        #expect(await fixture.recovery.calls == 0)
        #expect(await fixture.backend.removals == 0)
        #expect(await fixture.backend.registrations == 0)
    }
}

@Test @MainActor func hostRemovalFailedOrForeignRecoveryNeverUnregistersAndCanRetryItsIntent() async throws {
    let fixture = try RemovalFixture()
    defer { fixture.clean() }
    let failed = try fixture.removal(recover:{ _ in throw URLError(.networkConnectionLost) })
    await #expect(throws:URLError(.networkConnectionLost)) { try await failed.stopAndRevoke() }
    #expect(try removalPhase(fixture.directory) == "requested")
    let host = try MacHostProcess.observe(processID:fixture.child.process.processIdentifier,effectiveUserID:geteuid())
    for receipt in [MacOwnerRecoveryReceipt(installationID:"other",deviceID:"device",runtimeStopped:true,pairingRevoked:true,coreStopped:true),MacOwnerRecoveryReceipt(installationID:"installation",deviceID:"device",runtimeStopped:true,pairingRevoked:true,coreStopped:false)] {
        let wrong = try fixture.removal(recover:{ _ in MacHostRecoveryObservation(receipt:receipt,host:host) })
        await #expect(throws:ClientError.self) { try await wrong.stopAndRevoke() }
        #expect(try removalPhase(fixture.directory) == "requested")
    }
    #expect(await fixture.backend.removals == 0)
    await fixture.backend.beforeUnregister { try await MainActor.run { try fixture.child.finish() } }
    _ = try await fixture.removal(timeout:1).stopAndRevoke()
    #expect(try removalPhase(fixture.directory) == "confirmed")
}

@Test @MainActor func hostRemovalRefusesChangedConnectionAndProgressBeforeUnregistration() async throws {
    for target in ["connection.json","host-removal.json"] {
        let fixture = try RemovalFixture()
        defer { fixture.clean() }
        let file = fixture.directory.appendingPathComponent(target), recovery = fixture.recovery
        let replacement = Data("changed synthetic metadata".utf8)
        let removal = try fixture.removal(recover:{ request in
            let result = try await recovery.recover(request)
            try replacement.write(to:file)
            return result
        })
        await #expect(throws:(any Error).self) { try await removal.stopAndRevoke() }
        #expect(await fixture.backend.removals == 0)
        #expect(try Data(contentsOf:file) == replacement)
    }
}

@Test @MainActor func hostRemovalRefusesACLChangesWhileRecoveryIsPending() async throws {
    for target in ["directory","owner-setup.lock"] {
        let fixture = try RemovalFixture()
        defer { fixture.clean() }
        let file = target == "directory" ? fixture.directory : fixture.directory.appendingPathComponent(target)
        defer { try? removalACL(file,add:false) }
        let recovery = fixture.recovery
        let removal = try fixture.removal(recover:{ request in
            let result = try await recovery.recover(request)
            try removalACL(file,add:true)
            return result
        })
        await #expect(throws:ClientError.self) { try await removal.stopAndRevoke() }
        #expect(await fixture.backend.removals == 0)
        #expect(try removalPhase(fixture.directory) == "requested")
    }
}

@Test @MainActor func hostRemovalRejectsCorruptOrForeignSavedProofBeforeStartupChanges() async throws {
    for kind in ["version","phase","requested-with-proof","foreign-connection","missing-host","foreign-user","invalid-pid","invalid-time","partial-receipt","shared-file"] {
        let fixture = try RemovalFixture()
        defer { fixture.clean() }
        await #expect(throws:URLError(.timedOut)) { try await fixture.removal().stopAndRevoke() }
        var fields = try #require(try JSONSerialization.jsonObject(with:Data(contentsOf:fixture.progressURL)) as? [String:Any])
        var host = try #require(fields["host"] as? [String:Any])
        switch kind {
        case "version": fields["version"] = 2
        case "phase": fields["phase"] = "unexpected"
        case "requested-with-proof": fields["phase"] = "requested"
        case "foreign-connection":
            var connection = try #require(fields["connection"] as? [String:Any])
            connection["deviceID"] = "foreign"; fields["connection"] = connection
        case "missing-host": fields.removeValue(forKey:"host")
        case "foreign-user": host["effectiveUserID"] = geteuid() + 1; fields["host"] = host
        case "invalid-pid": host["processID"] = 0; fields["host"] = host
        case "invalid-time": host["startedMicroseconds"] = 1_000_000; fields["host"] = host
        case "partial-receipt":
            var receipt = try #require(fields["receipt"] as? [String:Any])
            receipt["coreStopped"] = false; fields["receipt"] = receipt
        default: break
        }
        let changed = try JSONSerialization.data(withJSONObject:fields)
        try changed.write(to:fixture.progressURL)
        if kind == "shared-file" { try FileManager.default.setAttributes([.posixPermissions:0o644],ofItemAtPath:fixture.progressURL.path) }
        await #expect(throws:(any Error).self) { try await fixture.removal().stopAndRevoke() }
        #expect(await fixture.recovery.calls == 1)
        #expect(await fixture.backend.removals == 1)
        #expect(try Data(contentsOf:fixture.progressURL) == changed)
    }
}

@Test @MainActor func hostRemovalConfirmedRecordRetryBudget() async throws {
    guard let destination = ProcessInfo.processInfo.environment["FORT_HOST_REMOVAL_BENCHMARK"] else { return }
    let fixture = try RemovalFixture()
    defer { fixture.clean() }
    await fixture.backend.beforeUnregister { try await MainActor.run { try fixture.child.finish() } }
    _ = try await fixture.removal(timeout:1).stopAndRevoke()
    let before = try Data(contentsOf:fixture.progressURL)
    var info = stat(); #expect(lstat(fixture.progressURL.path,&info) == 0)
    var samples: [Double] = []
    for _ in 0..<20 {
        let removal = try fixture.removal(timeout:1)
        let started = ContinuousClock.now
        let result = try await removal.stopAndRevoke()
        let duration = started.duration(to:.now).components
        samples.append(Double(duration.seconds) * 1000 + Double(duration.attoseconds) / 1e15)
        #expect(result.hostStopped && result.pairingRevoked && result.startupUnregistered)
    }
    var after = stat(); #expect(lstat(fixture.progressURL.path,&after) == 0)
    #expect(try Data(contentsOf:fixture.progressURL) == before)
    #expect(info.st_ino == after.st_ino && info.st_mtimespec.tv_sec == after.st_mtimespec.tv_sec && info.st_mtimespec.tv_nsec == after.st_mtimespec.tv_nsec)
    #expect(await fixture.recovery.calls == 1)
    #expect(await fixture.backend.removals == 1)
    try JSONEncoder().encode(samples).write(to:URL(fileURLWithPath:destination))
    #expect(samples.sorted()[18] <= 50)
}

@Test @MainActor func hostRemovalRefreshesRecoveryWhenUnregistrationFailedAndAgentRemainsRegistered() async throws {
    let fixture = try RemovalFixture()
    defer { fixture.clean() }
    await fixture.backend.beforeUnregister { throw URLError(.networkConnectionLost) }
    await #expect(throws:MacHostStartupError.self) { try await fixture.removal().stopAndRevoke() }
    #expect(try removalPhase(fixture.directory) == "recovered")
    #expect(await fixture.backend.current == .enabled)
    await fixture.backend.beforeUnregister { try await MainActor.run { try fixture.child.finish() } }
    _ = try await fixture.removal(timeout:1).stopAndRevoke()
    #expect(try removalPhase(fixture.directory) == "confirmed")
    #expect(await fixture.recovery.calls == 2)
    #expect(await fixture.backend.removals == 2)
}

@Test @MainActor func hostRemovalCanFinishAfterLocalForgettingButRejectsAnUnrelatedConnection() async throws {
    let fixture = try RemovalFixture()
    defer { fixture.clean() }
    await #expect(throws:URLError(.timedOut)) { try await fixture.removal().stopAndRevoke() }
    let connectionFile = fixture.directory.appendingPathComponent("connection.json")
    let other = try DeviceConnection(endpoint:.macAgent(fixture.endpoint),installationID:"other",deviceID:"device")
    try JSONEncoder().encode(other).write(to:connectionFile)
    await #expect(throws:OwnerLifecycleError.self) { try await fixture.removal().stopAndRevoke() }
    #expect(try removalPhase(fixture.directory) == "recovered")
    try FileManager.default.removeItem(at:connectionFile)
    await #expect(throws:OwnerLifecycleError.self) { try await fixture.removal().stopAndRevoke() }
    let disconnected = fixture.directory.appendingPathComponent("disconnected.json"), intent = fixture.directory.appendingPathComponent("pairing-removal.json")
    try JSONEncoder().encode(fixture.connection).write(to:disconnected)
    try JSONEncoder().encode(PairingRemovalIntent(connection:fixture.connection,state:.locallyForgotten)).write(to:intent)
    for file in [disconnected,intent] { try FileManager.default.setAttributes([.posixPermissions:0o600],ofItemAtPath:file.path) }
    try fixture.child.finish()
    _ = try await fixture.removal(timeout:1).stopAndRevoke()
    #expect(try removalPhase(fixture.directory) == "confirmed")
    #expect(await fixture.recovery.calls == 1)
    #expect(await fixture.backend.removals == 1)
    #expect(try JSONDecoder().decode(PairingRemovalIntent.self,from:Data(contentsOf:intent)).state == .locallyForgotten)
    #expect(!FileManager.default.fileExists(atPath:connectionFile.path))
}

@Test @MainActor func hostRemovalRevalidatesBundleBeforeRecoveryOrProgressWrites() async throws {
    let fixture = try RemovalFixture(); defer { fixture.clean() }
    let removal = try fixture.removal(verifyBundle:{ throw MacHostBundleError.signature })
    await #expect(throws:MacHostBundleError.self) { try await removal.stopAndRevoke() }
    await #expect(throws:MacHostBundleError.self) { try await removal.recoverPairingRemoval(fixture.connection) }
    #expect(await fixture.recovery.calls == 0)
    #expect(await fixture.backend.removals == 0)
    #expect(!FileManager.default.fileExists(atPath:fixture.progressURL.path))
    #expect(fixture.child.process.isRunning)
}

@Test @MainActor func hostRemovalDisconnectRecoveryPreservesStartupAndRejectsForeignPairing() async throws {
    let fixture = try RemovalFixture(); defer { fixture.clean() }
    let removal = try fixture.removal()
    let foreign = try DeviceConnection(endpoint:.macAgent(fixture.endpoint),installationID:"foreign",deviceID:"device")
    await #expect(throws:OwnerLifecycleError.self) { try await removal.recoverPairingRemoval(foreign) }
    #expect(await fixture.recovery.calls == 0)
    let proof = try await removal.recoverPairingRemoval(fixture.connection)
    #expect(proof.connection == fixture.connection && proof.hostStopped && proof.pairingRevoked)
    #expect(await fixture.recovery.calls == 1)
    #expect(await fixture.backend.current == .enabled)
    #expect(await fixture.backend.removals == 0)
    #expect(!FileManager.default.fileExists(atPath:fixture.progressURL.path))
    #expect(fixture.child.process.isRunning)
}

@Test @MainActor func hostRemovalLifecycleRetriesCredentialCleanupWithoutReactivatingOrDeletingHistory() async throws {
    let fixture = try RemovalFixture(); defer { fixture.clean() }
    let history = fixture.directory.appendingPathComponent("synthetic-history")
    try Data("retained".utf8).write(to:history)
    let removal = try fixture.removal(timeout:1)
    var cleanupAttempts = 0
    let lifecycle = OwnerLifecycle(installation:removal,removeCredential:{
        cleanupAttempts += 1
        if cleanupAttempts == 1 { throw URLError(.cannotRemoveFile) }
    })
    await #expect(throws:OwnerLifecycleError.self) { try await lifecycle.prepareForRemoval(history:.deleteConfirmed) }
    #expect(cleanupAttempts == 0)
    #expect(await fixture.recovery.calls == 0)
    #expect(await fixture.backend.removals == 0)
    try fixture.child.finish()
    await #expect(throws:URLError.self) { try await lifecycle.prepareForRemoval() }
    #expect(try removalPhase(fixture.directory) == "confirmed")
    let restarted = OwnerLifecycle(installation:try fixture.removal(timeout:1),removeCredential:{ cleanupAttempts += 1 })
    let prepared = try await restarted.prepareForRemoval()
    #expect(prepared.appURL == URL(fileURLWithPath:"/synthetic/Fort.app") && prepared.history == .keep)
    #expect(cleanupAttempts == 2)
    #expect(await fixture.recovery.calls == 1)
    #expect(await fixture.backend.removals == 1)
    #expect(await fixture.backend.registrations == 0)
    #expect(try String(contentsOf:history,encoding:.utf8) == "retained")
}
@MainActor private func completedReinstallFixture() async throws -> RemovalFixture {
    let fixture = try RemovalFixture()
    var ready = false; defer { if !ready { fixture.clean() } }
    try fixture.child.finish()
    _ = try await fixture.removal(timeout:1).stopAndRevoke()
    let files = try OwnerSetupFiles(directory:fixture.directory); defer { files.close() }
    try files.write(JSONEncoder().encode(fixture.connection),to:.disconnected)
    try files.write(JSONEncoder().encode(PairingRemovalIntent(connection:fixture.connection,state:.confirmed)),to:.removal)
    try files.removeConnection()
    ready = true
    return fixture
}

@Test @MainActor func hostRemovalExplicitReinstallPreservesHistoryAndDoesNotEnableOrPair() async throws {
    let fixture = try await completedReinstallFixture(); defer { fixture.clean() }
    let history = fixture.directory.appendingPathComponent("retained-history")
    try Data("synthetic retained history".utf8).write(to:history)
    let disconnected = try Data(contentsOf:fixture.directory.appendingPathComponent("disconnected.json"))
    let revocation = try Data(contentsOf:fixture.directory.appendingPathComponent("pairing-removal.json"))
    var credentialChecks = 0
    let removal = try fixture.removal(timeout:1)
    try await removal.prepareForSetup { credentialChecks += 1 }
    #expect(credentialChecks == 1)
    #expect(!FileManager.default.fileExists(atPath:fixture.progressURL.path))
    #expect(try Data(contentsOf:fixture.directory.appendingPathComponent("disconnected.json")) == disconnected)
    #expect(try Data(contentsOf:fixture.directory.appendingPathComponent("pairing-removal.json")) == revocation)
    #expect(try String(contentsOf:history,encoding:.utf8) == "synthetic retained history")
    #expect(await fixture.recovery.calls == 1)
    #expect(await fixture.backend.removals == 1)
    #expect(await fixture.backend.registrations == 0)
    try await removal.prepareForSetup { credentialChecks += 1 }
    #expect(credentialChecks == 1)
}
@Test(arguments:["requested","recovered","active-connection","pending","foreign-disconnected","unconfirmed-local","startup","live-helper","credential","bundle","changed-progress"])
@MainActor func hostRemovalReinstallPreservesUnconfirmedChangedOrForeignState(_ barrier: String) async throws {
    let fixture = try await completedReinstallFixture(); defer { fixture.clean() }
    var record = try #require(JSONSerialization.jsonObject(with:Data(contentsOf:fixture.progressURL)) as? [String:Any])
    let live = barrier == "live-helper" ? try RemovalChild() : nil
    defer { try? live?.finish() }
    if barrier == "requested" { record["phase"] = "requested"; record.removeValue(forKey:"host"); record.removeValue(forKey:"receipt") }
    if barrier == "recovered" { record["phase"] = "recovered" }
    if let live { record["host"] = try JSONSerialization.jsonObject(with:JSONEncoder().encode(MacHostProcess.observe(processID:live.process.processIdentifier,effectiveUserID:geteuid()).record)) }
    do {
        let files = try OwnerSetupFiles(directory:fixture.directory); defer { files.close() }
        try files.write(JSONSerialization.data(withJSONObject:record),to:.hostRemoval)
        if barrier == "active-connection" { try files.write(JSONEncoder().encode(fixture.connection),to:.connection) }
        if barrier == "pending" { try files.write(Data("pending synthetic setup".utf8),to:.pending) }
        if barrier == "foreign-disconnected" {
            let foreign = try DeviceConnection(endpoint:.macAgent(fixture.endpoint),installationID:"foreign",deviceID:"device")
            try files.write(JSONEncoder().encode(foreign),to:.disconnected)
        }
        if barrier == "unconfirmed-local" { try files.write(JSONEncoder().encode(PairingRemovalIntent(connection:fixture.connection,state:.locallyForgotten)),to:.removal) }
    }
    if barrier == "startup" { await fixture.backend.setStatus(.enabled) }
    let before = try Data(contentsOf:fixture.progressURL), changed = Data("synthetic replacement progress".utf8)
    let removal = try fixture.removal(verifyBundle:{ if barrier == "bundle" { throw MacHostBundleError.signature } })
    var credentialChecks = 0
    await #expect(throws:(any Error).self) {
        try await removal.prepareForSetup {
            credentialChecks += 1
            if barrier == "credential" { throw DeviceStorageError.removalUnavailable }
            if barrier == "changed-progress" { try changed.write(to:fixture.progressURL) }
        }
    }
    #expect(try Data(contentsOf:fixture.progressURL) == (barrier == "changed-progress" ? changed : before))
    #expect(credentialChecks == (["credential","changed-progress"].contains(barrier) ? 1 : 0))
    #expect(await fixture.recovery.calls == 1)
    #expect(await fixture.backend.removals == 1)
    #expect(await fixture.backend.registrations == 0)
}

@Test @MainActor func hostRemovalCanceledReinstallKeepsItsConfirmedRecordForRetry() async throws {
    let fixture = try await completedReinstallFixture(); defer { fixture.clean() }
    let before = try Data(contentsOf:fixture.progressURL), removal = try fixture.removal()
    let work = Task {
        try await removal.prepareForSetup { withUnsafeCurrentTask { $0?.cancel() } }
    }
    await #expect(throws:CancellationError.self) { try await work.value }
    #expect(try Data(contentsOf:fixture.progressURL) == before)
    try await removal.prepareForSetup {}
    #expect(!FileManager.default.fileExists(atPath:fixture.progressURL.path))
}

@Test @MainActor func hostRemovalReinstallDurabilityBudget() async throws {
    guard let destination = ProcessInfo.processInfo.environment["FORT_REINSTALL_BENCHMARK"] else { return }
    var samples: [Double] = []
    for _ in 0..<20 {
        let fixture = try await completedReinstallFixture(); defer { fixture.clean() }
        let removal = try fixture.removal(timeout:1), started = ContinuousClock.now
        try await removal.prepareForSetup {}
        let duration = started.duration(to:.now).components
        samples.append(Double(duration.seconds) * 1000 + Double(duration.attoseconds) / 1e15)
    }
    try JSONEncoder().encode(samples).write(to:URL(fileURLWithPath:destination))
    print("Synthetic reinstall reconciliation p95: \(samples.sorted()[18]) ms")
    #expect(samples.sorted()[18] <= 500)
}

@Test @MainActor func retainedHistoryDeletionPersistsExactIntentBeforeMaintenanceAndRetriesCacheFailure() async throws {
    let fixture = try await completedReinstallFixture(); defer { fixture.clean() }
    let journal = fixture.directory.appendingPathComponent("history-removal.json")
    var requests: [MacHistoryDeletionRequest] = [], cacheAttempts = 0, credentialChecks = 0
    let actions = MacHistoryRemovalActions(verifyCredentialAbsent:{ credentialChecks += 1 },maintain:{ request in
        let record = try #require(JSONSerialization.jsonObject(with:Data(contentsOf:journal)) as? [String:Any])
        let stored = try JSONDecoder().decode(MacHistoryDeletionRequest.self,from:JSONSerialization.data(withJSONObject:try #require(record["request"])))
        #expect(stored == request)
        requests.append(request)
        return MacHistoryDeletionReceipt(installationId:request.installationID,deviceId:request.deviceID,deletionId:request.deletionID,previousRecoveryEpoch:request.recoveryEpoch,recoveryEpoch:"new-epoch",deletedRecords:4)
    },removeSavedHistory:{
        cacheAttempts += 1
        if cacheAttempts == 1 { throw URLError(.cannotRemoveFile) }
    })
    let first = try fixture.removal(historyActions:actions)
    let request = try first.historyDeletionRequest(recoveryEpoch:"old-epoch")
    #expect(request.installationID == fixture.connection.installationID && request.deviceID == fixture.connection.deviceID)
    await #expect(throws:URLError.self) { try await first.deleteRetainedHistory(request) }
    let restarted = try fixture.removal(historyActions:actions)
    #expect(try restarted.historyDeletionRequest(recoveryEpoch:nil) == request)
    let receipt = try await restarted.deleteRetainedHistory(request)
    #expect(receipt.recoveryEpoch == "new-epoch" && requests == [request,request])
    #expect(cacheAttempts == 2 && credentialChecks >= 4)
    let final = try #require(JSONSerialization.jsonObject(with:Data(contentsOf:journal)) as? [String:Any])
    #expect(final["complete"] as? Bool == true)
    #expect(await fixture.backend.registrations == 0)
    #expect(await fixture.backend.removals == 1)
    #expect(await fixture.recovery.calls == 1)
}

@Test @MainActor func retainedHistoryDeletionRechecksJournalAfterCredentialInspectionBeforeMaintenance() async throws {
    let fixture = try await completedReinstallFixture(); defer { fixture.clean() }
    let journal = fixture.directory.appendingPathComponent("history-removal.json")
    var calls = 0
    let actions = MacHistoryRemovalActions(verifyCredentialAbsent:{
        if FileManager.default.fileExists(atPath:journal.path) { try Data("changed during credential inspection".utf8).write(to:journal) }
    },maintain:{ request in
        calls += 1
        return MacHistoryDeletionReceipt(installationId:request.installationID,deviceId:request.deviceID,deletionId:request.deletionID,previousRecoveryEpoch:request.recoveryEpoch,recoveryEpoch:"new-epoch",deletedRecords:4)
    },removeSavedHistory:{ Issue.record("Cache deletion must not start") })
    let removal = try fixture.removal(historyActions:actions), request = try removal.historyDeletionRequest(recoveryEpoch:"old-epoch")
    await #expect(throws:(any Error).self) { try await removal.deleteRetainedHistory(request) }
    #expect(calls == 0)
}

private struct AbsentHistoryCredentials: DeviceCredentials {
    func save(_ connection: DeviceConnection, token: String) throws { throw DeviceStorageError.removalUnavailable }
    func token(for connection: DeviceConnection) throws -> String { throw DeviceStorageError.keychain(errSecItemNotFound) }
}

@Test(arguments:["startup","startup-changed","live-helper","credential","bundle","requested","recovered","future-host","active-connection","pending","unconfirmed-pairing","foreign-pairing","future-history","malformed-history","foreign-request"])
@MainActor func retainedHistoryDeletionRefusesUnverifiedStateBeforeMaintenance(_ barrier: String) async throws {
    let fixture = try await completedReinstallFixture(); defer { fixture.clean() }
    var calls = 0
    let actions = MacHistoryRemovalActions(verifyCredentialAbsent:{
        if barrier == "credential" { throw DeviceStorageError.keychain(errSecInteractionNotAllowed) }
    },maintain:{ request in
        calls += 1
        return MacHistoryDeletionReceipt(installationId:request.installationID,deviceId:request.deviceID,deletionId:request.deletionID,previousRecoveryEpoch:request.recoveryEpoch,recoveryEpoch:"new-epoch",deletedRecords:4)
    },removeSavedHistory:{ Issue.record("Cache deletion must not start") })
    let removal = try fixture.removal(verifyBundle:{ if barrier == "bundle" { throw MacHostBundleError.signature } },historyActions:actions)
    let request = try removal.historyDeletionRequest(recoveryEpoch:"old-epoch")
    let live = barrier == "live-helper" ? try RemovalChild() : nil
    defer { try? live?.finish() }
    if barrier == "startup" { await fixture.backend.setStatus(.enabled) }
    if barrier == "startup-changed" { await fixture.backend.reenableOnNextInspection() }
    do {
        let files = try OwnerSetupFiles(directory:fixture.directory); defer { files.close() }
        if ["requested","recovered","future-host","live-helper"].contains(barrier) {
            var host = try #require(JSONSerialization.jsonObject(with:Data(contentsOf:fixture.progressURL)) as? [String:Any])
            if barrier == "requested" { host["phase"] = "requested"; host.removeValue(forKey:"receipt"); host.removeValue(forKey:"host") }
            if barrier == "recovered" { host["phase"] = "recovered" }
            if barrier == "future-host" { host["version"] = 2 }
            if let live {
                let observed = try MacHostProcess.observe(processID:live.process.processIdentifier,effectiveUserID:geteuid())
                host["host"] = try JSONSerialization.jsonObject(with:JSONEncoder().encode(observed.record))
            }
            try files.write(JSONSerialization.data(withJSONObject:host),to:.hostRemoval)
        }
        if barrier == "active-connection" { try files.write(JSONEncoder().encode(fixture.connection),to:.connection) }
        if barrier == "pending" { try files.write(Data("{}".utf8),to:.pending) }
        if barrier == "unconfirmed-pairing" { try files.write(JSONEncoder().encode(PairingRemovalIntent(connection:fixture.connection,state:.locallyForgotten)),to:.removal) }
        if barrier == "foreign-pairing" {
            let other = try DeviceConnection(endpoint:fixture.connection.endpoint,installationID:"foreign",deviceID:"device")
            try files.write(JSONEncoder().encode(PairingRemovalIntent(connection:other,state:.confirmed)),to:.removal)
        }
        if barrier == "future-history" {
            let record: [String:Any] = ["version":2,"connection":try JSONSerialization.jsonObject(with:JSONEncoder().encode(fixture.connection)),"request":try JSONSerialization.jsonObject(with:JSONEncoder().encode(request)),"complete":false]
            try files.write(JSONSerialization.data(withJSONObject:record),to:.historyRemoval)
        }
        if barrier == "malformed-history" { try files.write(Data("{}".utf8),to:.historyRemoval) }
    }
    let supplied = barrier == "foreign-request" ? try MacHistoryDeletionRequest(installationID:"foreign",deviceID:request.deviceID,recoveryEpoch:request.recoveryEpoch,deletionID:request.deletionID) : request
    await #expect(throws:(any Error).self) { try await removal.deleteRetainedHistory(supplied) }
    #expect(calls == 0)
    #expect(await fixture.backend.registrations == 0)
    #expect(await fixture.backend.removals == 1)
}

@Test(arguments:["lost-reply","wrong-receipt","cancel","cache-changed","busy"])
@MainActor func retainedHistoryDeletionKeepsExactRetryAfterUnconfirmedEffects(_ failure: String) async throws {
    let fixture = try await completedReinstallFixture(); defer { fixture.clean() }
    var fail = true, calls = 0, cacheCalls = 0
    var removal: MacHostRemoval?
    let actions = MacHistoryRemovalActions(verifyCredentialAbsent:{},maintain:{ request in
        calls += 1
        if fail && failure == "lost-reply" { throw URLError(.networkConnectionLost) }
        if fail && failure == "cancel" { withUnsafeCurrentTask { $0?.cancel() } }
        if failure == "busy", let removal {
            await #expect(throws:MacHostRemovalError.busy) { try await removal.deleteRetainedHistory(request) }
            let another = try fixture.removal(historyActions:MacHistoryRemovalActions(verifyCredentialAbsent:{},maintain:{ _ in throw ClientError.invalidResponse },removeSavedHistory:{}))
            await #expect(throws:(any Error).self) { try await another.deleteRetainedHistory(request) }
        }
        return MacHistoryDeletionReceipt(installationId:request.installationID,deviceId:request.deviceID,deletionId:fail && failure == "wrong-receipt" ? "foreign" : request.deletionID,previousRecoveryEpoch:request.recoveryEpoch,recoveryEpoch:"new-epoch",deletedRecords:4)
    },removeSavedHistory:{
        cacheCalls += 1
        if fail && failure == "cache-changed" {
            let files = try OwnerSetupFiles(directory:fixture.directory); defer { files.close() }
            try files.write(Data("changed".utf8),to:.historyRemoval)
        }
    })
    let coordinator = try fixture.removal(historyActions:actions); removal = coordinator
    let request = try coordinator.historyDeletionRequest(recoveryEpoch:"old-epoch")
    if failure == "busy" { _ = try await coordinator.deleteRetainedHistory(request); return }
    let work = Task { try await coordinator.deleteRetainedHistory(request) }
    await #expect(throws:(any Error).self) { try await work.value }
    #expect(calls == 1 && cacheCalls == (failure == "cache-changed" ? 1 : 0))
    if failure == "cache-changed" {
        #expect(throws:(any Error).self) { try coordinator.historyDeletionRequest(recoveryEpoch:nil) }
        return
    }
    #expect(try coordinator.historyDeletionRequest(recoveryEpoch:nil) == request)
    let changed = try MacHistoryDeletionRequest(installationID:request.installationID,deviceID:request.deviceID,recoveryEpoch:request.recoveryEpoch,deletionID:"different-confirmation")
    await #expect(throws:MacHostRemovalError.changedProgress) { try await coordinator.deleteRetainedHistory(changed) }
    #expect(calls == 1)
    fail = false
    _ = try await coordinator.deleteRetainedHistory(request)
    #expect(calls == 2 && cacheCalls == 1)
}

@Test @MainActor func retainedHistoryDeletionRetryIsRetiredOnlyByExplicitReinstall() async throws {
    let fixture = try await completedReinstallFixture(); defer { fixture.clean() }
    var calls = 0
    let actions = MacHistoryRemovalActions(verifyCredentialAbsent:{},maintain:{ _ in calls += 1; throw URLError(.networkConnectionLost) },removeSavedHistory:{})
    let removal = try fixture.removal(historyActions:actions), request = try removal.historyDeletionRequest(recoveryEpoch:"old-epoch")
    await #expect(throws:URLError.self) { try await removal.deleteRetainedHistory(request) }
    #expect(try removal.historyDeletionRequest(recoveryEpoch:nil) == request)
    try await removal.prepareForSetup {}
    #expect(!FileManager.default.fileExists(atPath:fixture.directory.appendingPathComponent("history-removal.json").path))
    await #expect(throws:(any Error).self) { try await removal.deleteRetainedHistory(request) }
    #expect(calls == 1)
    #expect(await fixture.backend.registrations == 0)
}

@Test @MainActor func retainedHistoryDeletionCoordinatorRetryBudget() async throws {
    guard let destination = ProcessInfo.processInfo.environment["FORT_HISTORY_REMOVAL_BENCHMARK"] else { return }
    var samples: [Double] = []
    for _ in 0..<20 {
        let fixture = try await completedReinstallFixture(); defer { fixture.clean() }
        var interrupted = true
        let actions = MacHistoryRemovalActions(verifyCredentialAbsent:{},maintain:{ request in
            MacHistoryDeletionReceipt(installationId:request.installationID,deviceId:request.deviceID,deletionId:request.deletionID,previousRecoveryEpoch:request.recoveryEpoch,recoveryEpoch:"new-epoch",deletedRecords:4)
        },removeSavedHistory:{ if interrupted { throw URLError(.cannotRemoveFile) } })
        let first = try fixture.removal(historyActions:actions), request = try first.historyDeletionRequest(recoveryEpoch:"old-epoch")
        await #expect(throws:URLError.self) { try await first.deleteRetainedHistory(request) }
        interrupted = false
        let restarted = try fixture.removal(historyActions:actions), start = ContinuousClock.now
        _ = try await restarted.deleteRetainedHistory(restarted.historyDeletionRequest(recoveryEpoch:nil))
        let duration = start.duration(to:.now).components
        samples.append(Double(duration.seconds) * 1000 + Double(duration.attoseconds) / 1e15)
    }
    try JSONEncoder().encode(samples).write(to:URL(fileURLWithPath:destination))
    print("Synthetic history coordinator retry p95: \(samples.sorted()[18]) ms")
    #expect(samples.sorted()[18] <= 500)
}
@Test @MainActor func retainedHostHistoryModelUsesCapturedIdentityAndClearsOnlyAfterBothScopesComplete() async throws {
    let fixture = try await completedReinstallFixture(); defer { fixture.clean() }
    let storage = try DeviceStorage(directory:fixture.directory,credentials:AbsentHistoryCredentials())
    let installation = try JSONDecoder().decode(Installation.self,from:Data("{\"contractVersion\":\"1.0\",\"installationId\":\"installation\",\"recoveryEpoch\":\"old-epoch\",\"mode\":\"local\",\"serviceName\":\"Synthetic\"}".utf8))
    try storage.saveNavigation(SavedNavigation(installation:installation,agents:[],selectedAgentID:nil))
    var requests: [MacHistoryDeletionRequest] = []
    let actions = MacHistoryRemovalActions(verifyCredentialAbsent:{
        let credential = try storage.ownerSetupCredential(for:fixture.connection)
        #expect(credential == nil)
    },maintain:{ request in
        requests.append(request)
        return MacHistoryDeletionReceipt(installationId:request.installationID,deviceId:request.deviceID,deletionId:request.deletionID,previousRecoveryEpoch:request.recoveryEpoch,recoveryEpoch:"new-epoch",deletedRecords:4)
    },removeSavedHistory:{ try storage.deleteSavedHistory(for:fixture.connection) })
    let removal = try fixture.removal(historyActions:actions)
    let model = MessengerModel(storage:storage,session:.shared,ownedInstallation:removal)
    await model.restore()
    let captured = try #require(model.retainedHostHistoryDeletionTarget)
    let stale = try MacHistoryDeletionRequest(installationID:"another-installation",deviceID:captured.deviceID,recoveryEpoch:captured.recoveryEpoch,deletionID:captured.deletionID)
    await model.deleteRetainedHostHistory(stale)
    #expect(requests.isEmpty && model.installation != nil)
    #expect(try storage.navigation(installationID:"installation") != nil)
    await model.deleteRetainedHostHistory(captured)
    #expect(requests == [captured])
    #expect(model.installation == nil && model.pending.isEmpty && model.localImages.isEmpty)
    #expect(!model.canDeleteRetainedHostHistory)
    #expect(model.status == "Host history and this Mac's saved copy deleted")
    #expect(try storage.navigation(installationID:"installation") == nil)
    let reopened = MessengerModel(storage:storage,session:.shared,ownedInstallation:try fixture.removal(historyActions:actions))
    await reopened.restore()
    #expect(reopened.retainedHostHistoryDeletionTarget == nil && requests.count == 1)
}
#endif
