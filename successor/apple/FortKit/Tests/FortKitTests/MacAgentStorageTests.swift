#if os(macOS)
import Foundation
import Darwin
import Testing
@testable import FortKit

private func agentStorageRoot() throws -> URL {
    let root = FileManager.default.temporaryDirectory.appendingPathComponent("fort-agent-storage-" + UUID().uuidString)
    try FileManager.default.createDirectory(at:root,withIntermediateDirectories:false,attributes:[.posixPermissions:0o700])
    return root
}
private func storageEndpoint() throws -> MacAgentEndpoint {
    try MacAgentEndpoint(serviceIdentifier:"io.fort.synthetic.agent",signingTeam:"AAAAAAAAAA",appGroup:"group.io.fort.synthetic")
}

@Test func agentStorageInspectionNeverCreatesMissingStorage() throws {
    let root = try agentStorageRoot(); defer { try? FileManager.default.removeItem(at:root) }
    let endpoint = try storageEndpoint()
    var lookups: [String] = []
    #expect(throws:(any Error).self) {
        try MacAgentStorage.inspectExisting(for:endpoint,resolveContainer:{ lookups.append($0); return root })
    }
    #expect(lookups == [endpoint.appGroup])
    #expect(try FileManager.default.contentsOfDirectory(atPath:root.path).isEmpty)
    #expect(throws:MacAgentStorageError.self) { try MacAgentStorage.inspectExisting(for:endpoint,resolveContainer:{ _ in nil }) }
    #expect(throws:(any Error).self) { try MacAgentStorage.inspectExisting(for:endpoint,resolveContainer:{ _ in root.appendingPathComponent("absent") }) }
    #expect(try FileManager.default.contentsOfDirectory(atPath:root.path).isEmpty)
}

@Test func agentStorageResolvesOnlyItsExactGroupAndPreservesHistoryAcrossReopen() throws {
    let root = try agentStorageRoot()
    defer { try? FileManager.default.removeItem(at:root) }
    let endpoint = try storageEndpoint()
    var lookups: [String] = []
    let storage = try MacAgentStorage.prepare(for:endpoint,resolveContainer:{ group in lookups.append(group); return root })
    #expect(lookups == [endpoint.appGroup])
    let directory = try storage.validatedDirectory()
    let canonical = try #require(realpath(root.path,nil))
    defer { free(canonical) }
    #expect(directory.path == String(cString:canonical) + "/FortHost")
    var info = stat()
    #expect(lstat(directory.path,&info) == 0)
    #expect(info.st_mode & 0o777 == 0o700 && info.st_uid == geteuid())
    #expect(try FileManager.default.contentsOfDirectory(atPath:root.path) == ["FortHost"])
    let history = directory.appendingPathComponent("retained-history")
    let bytes = Data("  existing synthetic history\n🦉\n".utf8)
    try bytes.write(to:history)
    let reopened = try MacAgentStorage.prepare(for:endpoint,resolveContainer:{ _ in root })
    #expect(try reopened.validatedDirectory() == directory)
    #expect(try Data(contentsOf:history) == bytes)
    #expect(try FileManager.default.contentsOfDirectory(atPath:directory.path) == ["retained-history"])
    var inspectedGroups: [String] = []
    let inspected = try MacAgentStorage.inspectExisting(for:endpoint,resolveContainer:{ inspectedGroups.append($0); return root })
    #expect(inspectedGroups == [endpoint.appGroup])
    #expect(try inspected.validatedDirectory() == directory)
    #expect(try Data(contentsOf:history) == bytes)
    #expect(try FileManager.default.contentsOfDirectory(atPath:directory.path) == ["retained-history"])
}
@Test func agentStorageMissingOrInvalidGroupNeverSelectsAnotherLocation() throws {
    let endpoint = try storageEndpoint()
    var lookups: [String] = []
    #expect(throws:MacAgentStorageError.self) {
        try MacAgentStorage.prepare(for:endpoint,resolveContainer:{ group in lookups.append(group); return nil })
    }
    #expect(lookups == [endpoint.appGroup])
    for value in ["https://fixture.invalid/group", "file://foreign.invalid/group"] {
        #expect(throws:MacAgentStorageError.self) {
            try MacAgentStorage.prepare(for:endpoint,resolveContainer:{ _ in URL(string:value) })
        }
    }
    let root = try agentStorageRoot()
    defer { try? FileManager.default.removeItem(at:root) }
    let absent = root.appendingPathComponent("absent")
    #expect(throws:(any Error).self) { try MacAgentStorage.prepare(for:endpoint,resolveContainer:{ _ in absent }) }
    #expect(try FileManager.default.contentsOfDirectory(atPath:root.path).isEmpty)
}

private func storageACL(_ directory: URL, add: Bool) throws {
    let process = Process()
    process.executableURL = URL(fileURLWithPath:"/bin/chmod")
    process.arguments = add ? ["+a","everyone allow read",directory.path] : ["-N",directory.path]
    process.environment = [:]
    try process.run(); process.waitUntilExit()
    #expect(process.terminationReason == .exit && process.terminationStatus == 0)
}

@Test func agentStorageRejectsUnsafeContainersBeforeCreatingItsChild() throws {
    let endpoint = try storageEndpoint()
    for kind in ["file","symlink","fifo","shared","acl"] {
        let root = try agentStorageRoot()
        defer { try? FileManager.default.removeItem(at:root) }
        let container = root.appendingPathComponent("container")
        let outside = root.appendingPathComponent("outside")
        try FileManager.default.createDirectory(at:outside,withIntermediateDirectories:false,attributes:[.posixPermissions:0o700])
        let marker = Data("keep this synthetic marker".utf8)
        switch kind {
        case "file": try marker.write(to:container)
        case "symlink": try FileManager.default.createSymbolicLink(at:container,withDestinationURL:outside)
        case "fifo": #expect(mkfifo(container.path,0o600) == 0)
        default:
            try FileManager.default.createDirectory(at:container,withIntermediateDirectories:false,attributes:[.posixPermissions:0o700])
            if kind == "shared" { #expect(chmod(container.path,0o770) == 0) }
            else { try storageACL(container,add:true) }
        }
        var before = stat(); #expect(lstat(container.path,&before) == 0)
        #expect(throws:(any Error).self) { try MacAgentStorage.inspectExisting(for:endpoint,resolveContainer:{ _ in container }) }
        #expect(throws:(any Error).self) { try MacAgentStorage.prepare(for:endpoint,resolveContainer:{ _ in container }) }
        var after = stat(); #expect(lstat(container.path,&after) == 0)
        #expect(before.st_ino == after.st_ino && before.st_mode == after.st_mode)
        #expect(try FileManager.default.contentsOfDirectory(atPath:outside.path).isEmpty)
        if kind == "file" { #expect(try Data(contentsOf:container) == marker) }
        if kind == "shared" || kind == "acl" { #expect(try FileManager.default.contentsOfDirectory(atPath:container.path).isEmpty) }
        if kind == "acl" {
            #expect(throws:(any Error).self) { try MacAgentStorage.prepare(for:endpoint,resolveContainer:{ _ in container }) }
            try storageACL(container,add:false)
            _ = try MacAgentStorage.prepare(for:endpoint,resolveContainer:{ _ in container }).validatedDirectory()
        }
    }
    #expect(geteuid() != 0)
    #expect(throws:MacAgentStorageError.self) { try MacAgentStorage.prepare(for:endpoint,resolveContainer:{ _ in URL(fileURLWithPath:"/",isDirectory:true) }) }
}

@Test func agentStoragePreservesAndRefusesUnsafeExistingChildren() throws {
    let endpoint = try storageEndpoint()
    for kind in ["file","symlink","fifo","shared","owner-read-only","acl"] {
        let root = try agentStorageRoot()
        defer { try? FileManager.default.removeItem(at:root) }
        let directory = root.appendingPathComponent("FortHost")
        let outside = root.appendingPathComponent("outside")
        try FileManager.default.createDirectory(at:outside,withIntermediateDirectories:false,attributes:[.posixPermissions:0o700])
        let marker = Data("keep this synthetic history".utf8)
        switch kind {
        case "file": try marker.write(to:directory)
        case "symlink": try FileManager.default.createSymbolicLink(at:directory,withDestinationURL:outside)
        case "fifo": #expect(mkfifo(directory.path,0o600) == 0)
        default:
            try FileManager.default.createDirectory(at:directory,withIntermediateDirectories:false,attributes:[.posixPermissions:0o700])
            try marker.write(to:directory.appendingPathComponent("history"))
            if kind == "acl" { try storageACL(directory,add:true) }
            else { #expect(chmod(directory.path,kind == "shared" ? 0o750 : 0o500) == 0) }
        }
        var before = stat(); #expect(lstat(directory.path,&before) == 0)
        #expect(throws:(any Error).self) { try MacAgentStorage.inspectExisting(for:endpoint,resolveContainer:{ _ in root }) }
        #expect(throws:(any Error).self) { try MacAgentStorage.prepare(for:endpoint,resolveContainer:{ _ in root }) }
        var after = stat(); #expect(lstat(directory.path,&after) == 0)
        #expect(before.st_ino == after.st_ino && before.st_mode == after.st_mode)
        #expect(try FileManager.default.contentsOfDirectory(atPath:outside.path).isEmpty)
        if kind == "file" { #expect(try Data(contentsOf:directory) == marker) }
        if ["shared","owner-read-only","acl"].contains(kind) {
            #expect(try Data(contentsOf:directory.appendingPathComponent("history")) == marker)
            if kind == "acl" {
                #expect(throws:(any Error).self) { try MacAgentStorage.prepare(for:endpoint,resolveContainer:{ _ in root }) }
                try storageACL(directory,add:false)
                _ = try MacAgentStorage.prepare(for:endpoint,resolveContainer:{ _ in root }).validatedDirectory()
                #expect(try Data(contentsOf:directory.appendingPathComponent("history")) == marker)
            }
            else { #expect(chmod(directory.path,0o700) == 0) }
        }
    }
}

@Test func agentStorageRevalidationRefusesChangedIdentitiesPermissionsAndACLs() throws {
    let endpoint = try storageEndpoint()
    for kind in ["child-replaced","container-replaced","symlink","removed","permissions","acl","container-permissions","container-acl"] {
        let root = try agentStorageRoot()
        defer { try? FileManager.default.removeItem(at:root) }
        let container = root.appendingPathComponent("group")
        try FileManager.default.createDirectory(at:container,withIntermediateDirectories:false,attributes:[.posixPermissions:0o700])
        let storage = try MacAgentStorage.prepare(for:endpoint,resolveContainer:{ _ in container })
        let inspected = try MacAgentStorage.inspectExisting(for:endpoint,resolveContainer:{ _ in container })
        let directory = try storage.validatedDirectory()
        let bytes = Data("preserved synthetic history".utf8)
        try bytes.write(to:directory.appendingPathComponent("history"))
        let moved = root.appendingPathComponent("moved")
        switch kind {
        case "child-replaced","symlink","removed":
            try FileManager.default.moveItem(at:directory,to:moved)
            if kind == "child-replaced" { try FileManager.default.createDirectory(at:directory,withIntermediateDirectories:false,attributes:[.posixPermissions:0o700]) }
            if kind == "symlink" { try FileManager.default.createSymbolicLink(at:directory,withDestinationURL:moved) }
        case "container-replaced":
            try FileManager.default.moveItem(at:container,to:moved)
            try FileManager.default.createDirectory(at:container,withIntermediateDirectories:false,attributes:[.posixPermissions:0o700])
            _ = try MacAgentStorage.prepare(for:endpoint,resolveContainer:{ _ in container })
        case "permissions": #expect(chmod(directory.path,0o750) == 0)
        case "container-permissions": #expect(chmod(container.path,0o755) == 0)
        case "container-acl": try storageACL(container,add:true)
        default: try storageACL(directory,add:true)
        }
        #expect(throws:(any Error).self) { try storage.validatedDirectory() }
        #expect(throws:(any Error).self) { try inspected.validatedDirectory() }
        let retained = kind == "container-replaced" ? moved.appendingPathComponent("FortHost") : ["child-replaced","symlink","removed"].contains(kind) ? moved : directory
        #expect(try Data(contentsOf:retained.appendingPathComponent("history")) == bytes)
        if kind == "acl" { try storageACL(directory,add:false) }
        if kind == "container-acl" { try storageACL(container,add:false) }
    }
}

@Test func agentStorageConcurrentPreparationConvergesOnOneCanonicalDirectory() async throws {
    let root = try agentStorageRoot()
    defer { try? FileManager.default.removeItem(at:root) }
    let endpoint = try storageEndpoint()
    let directories = try await withThrowingTaskGroup(of:URL.self) { group in
        for _ in 0..<20 { group.addTask { try MacAgentStorage.prepare(for:endpoint,resolveContainer:{ _ in root }).validatedDirectory() } }
        var results: [URL] = []
        for try await directory in group { results.append(directory) }
        return results
    }
    #expect(directories.count == 20 && Set(directories).count == 1)
    #expect(try FileManager.default.contentsOfDirectory(atPath:root.path) == ["FortHost"])
}

@Test(.enabled(if:ProcessInfo.processInfo.environment["FORT_TEST_GUARDIAN"] != nil))
func agentStorageRunsTheCompiledOwnerCoreThroughItsGuardedNativeHost() async throws {
    let environment = ProcessInfo.processInfo.environment
    let node = URL(fileURLWithPath:try #require(environment["FORT_TEST_NODE"]))
    let guardian = URL(fileURLWithPath:try #require(environment["FORT_TEST_GUARDIAN"]))
    let artifact = URL(fileURLWithPath:try #require(environment["FORT_TEST_CORE_ARTIFACT"]))
    let source = URL(fileURLWithPath:#filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
    let root = try agentStorageRoot()
    defer { try? FileManager.default.removeItem(at:root) }
    let storage = try MacAgentStorage.prepare(for:storageEndpoint(),resolveContainer:{ _ in root })
    let directory = try storage.validatedDirectory()
    let arguments = ["--jitless","--no-experimental-strip-types","--no-warnings",source.appendingPathComponent("host/fixtures/owned-core.mjs").path,artifact.appendingPathComponent("host/owned-core.js").path,directory.path,"restore","ordinary"]
    let host = try await MacCoreHost.launch(executable:node,arguments:arguments,directory:storage.validatedDirectory(),environment:["PATH":"/usr/bin:/bin","HOME":directory.path,"TMPDIR":directory.path],guardian:guardian)
    do {
        let child = try JSONDecoder().decode(Int32.self,from:Data(contentsOf:directory.appendingPathComponent("core-pid.json")))
        let group = getpgid(child)
        #expect(group > 1 && group != getpgrp())
        guard case .available(let offer) = try await host.beginOwnerSetup(timeout:3) else { throw ClientError.invalidResponse }
        let setup = try OwnerSetupRequest(installationID:offer.installationID,setupID:offer.setupID,deviceID:"synthetic-device",deviceToken:String(repeating:"A",count:43))
        _ = try await host.completeOwnerSetup(setup,timeout:3)
        let client = FortClient(installationID:setup.installationID,token:setup.deviceToken,transport:host)
        #expect(try await client.installation().installationId == setup.installationID)
        let recovery = try MacOwnerRecoveryRequest(installationID:setup.installationID,deviceID:setup.deviceID,timeout:3)
        try await host.recoverPairing(recovery).validate(for:recovery)
        #expect(kill(child,0) == -1 && errno == ESRCH)
        #expect(kill(-group,0) == -1 && errno == ESRCH)
        #expect(try await host.shutdown().exitedCleanly)
        #expect(try storage.validatedDirectory() == directory)
        #expect(FileManager.default.fileExists(atPath:directory.appendingPathComponent("openai-candidate-b.sqlite").path))
    } catch { _ = try? await host.shutdown(); throw error }
}

@Test(.enabled(if:ProcessInfo.processInfo.environment["FORT_AGENT_STORAGE_BENCHMARK"] == "1"))
func agentStoragePreparationAndInspectionStayWithinTheirIsolatedBudget() throws {
    let endpoint = try storageEndpoint()
    var samples: [String:[Double]] = ["prepare":[],"reopen":[],"validate":[]]
    func milliseconds(_ operation: () throws -> Void) rethrows -> Double {
        let started = ContinuousClock.now
        try operation()
        let elapsed = started.duration(to:.now).components
        return Double(elapsed.seconds) * 1_000 + Double(elapsed.attoseconds) / 1e15
    }
    for _ in 0..<20 {
        let root = try agentStorageRoot()
        defer { try? FileManager.default.removeItem(at:root) }
        samples["prepare",default:[]].append(try milliseconds { _ = try MacAgentStorage.prepare(for:endpoint,resolveContainer:{ _ in root }) })
        samples["reopen",default:[]].append(try milliseconds { _ = try MacAgentStorage.prepare(for:endpoint,resolveContainer:{ _ in root }) })
        let storage = try MacAgentStorage.prepare(for:endpoint,resolveContainer:{ _ in root })
        samples["validate",default:[]].append(try milliseconds { _ = try storage.validatedDirectory() })
    }
    for values in samples.values { #expect(values.sorted()[18] <= 50) }
    print("AGENT_STORAGE_BUDGET=" + String(decoding:try JSONEncoder().encode(samples),as:UTF8.self))
}

@Test func agentStoragePreparesPrivateRuntimeDirectoriesWithoutAnotherIdentityStore() throws {
    let root = try agentStorageRoot(); defer { try? FileManager.default.removeItem(at:root) }
    let endpoint = try storageEndpoint()
    let storage = try MacAgentStorage.prepare(for:endpoint,resolveContainer:{ _ in root })
    let directory = try storage.validatedDirectory(), runtime = try storage.prepareRuntimeDirectory()
    #expect(runtime == directory.appendingPathComponent("Runtime",isDirectory:true))
    #expect(try FileManager.default.contentsOfDirectory(atPath:directory.path) == ["Runtime"])
    #expect(try FileManager.default.contentsOfDirectory(atPath:runtime.path).sorted() == ["home","state","tmp","workspace"])
    for name in ["", "home", "state", "tmp", "workspace"] {
        var info = stat(); #expect(lstat(runtime.appendingPathComponent(name).path,&info) == 0)
        #expect(info.st_mode & 0o7777 == 0o700 && info.st_uid == geteuid())
    }
    let retained = runtime.appendingPathComponent("state/retained-synthetic")
    try Data("keep".utf8).write(to:retained)
    let reopened = try MacAgentStorage.prepare(for:endpoint,resolveContainer:{ _ in root })
    #expect(try reopened.prepareRuntimeDirectory() == runtime)
    #expect(try Data(contentsOf:retained) == Data("keep".utf8))
}

@Test(arguments:["Runtime", "Runtime/home", "Runtime/state", "Runtime/workspace", "Runtime/tmp"], ["symlink","file","fifo","permissions","acl"])
func agentStorageRefusesUnsafeRuntimeDirectoriesWithoutRepair(_ path: String, _ fault: String) throws {
    let root = try agentStorageRoot(); defer { try? FileManager.default.removeItem(at:root) }
    let storage = try MacAgentStorage.prepare(for:storageEndpoint(),resolveContainer:{ _ in root })
    let directory = try storage.validatedDirectory()
    _ = try storage.prepareRuntimeDirectory()
    let target = directory.appendingPathComponent(path), outside = root.appendingPathComponent("outside")
    try FileManager.default.createDirectory(at:outside,withIntermediateDirectories:false,attributes:[.posixPermissions:0o700])
    try FileManager.default.removeItem(at:target)
    switch fault {
    case "symlink": try FileManager.default.createSymbolicLink(at:target,withDestinationURL:outside)
    case "file": try Data("preserve".utf8).write(to:target)
    case "fifo": #expect(mkfifo(target.path,0o600) == 0)
    default:
        try FileManager.default.createDirectory(at:target,withIntermediateDirectories:false,attributes:[.posixPermissions:0o700])
        if fault == "acl" { try storageACL(target,add:true) }
        else { #expect(chmod(target.path,0o750) == 0) }
    }
    var before = stat(); #expect(lstat(target.path,&before) == 0)
    #expect(throws:(any Error).self) { try storage.prepareRuntimeDirectory() }
    var after = stat(); #expect(lstat(target.path,&after) == 0)
    #expect(before.st_ino == after.st_ino && before.st_mode == after.st_mode)
    #expect(try FileManager.default.contentsOfDirectory(atPath:outside.path).isEmpty)
    if fault == "file" { #expect(try Data(contentsOf:target) == Data("preserve".utf8)) }
    if fault == "acl" { try storageACL(target,add:false) }
}

@Test(.enabled(if:ProcessInfo.processInfo.environment["FORT_RUNTIME_STORAGE_BENCHMARK"] != nil))
func agentStorageRuntimePreparationBudget() throws {
    let root = try agentStorageRoot(); defer { try? FileManager.default.removeItem(at:root) }
    let storage = try MacAgentStorage.prepare(for:storageEndpoint(),resolveContainer:{ _ in root })
    var samples: [Double] = []
    for _ in 0..<20 {
        let start = ContinuousClock.now
        _ = try storage.prepareRuntimeDirectory()
        let value = start.duration(to:.now).components
        samples.append(Double(value.seconds) * 1000 + Double(value.attoseconds) / 1e15)
    }
    let destination = try #require(ProcessInfo.processInfo.environment["FORT_RUNTIME_STORAGE_BENCHMARK"])
    try JSONEncoder().encode(samples).write(to:URL(fileURLWithPath:destination),options:.withoutOverwriting)
    #expect(samples.sorted()[18] <= 50)
}

@Test(.enabled(if:ProcessInfo.processInfo.environment["FORT_STORAGE_INSPECTION_SAMPLES"] != nil))
func agentStorageExistingInspectionBudget() throws {
    let root = try agentStorageRoot(); defer { try? FileManager.default.removeItem(at:root) }
    let endpoint = try storageEndpoint()
    let storage = try MacAgentStorage.prepare(for:endpoint,resolveContainer:{ _ in root })
    let directory = try storage.validatedDirectory()
    var samples: [Double] = []
    for _ in 0..<20 {
        let start = ContinuousClock.now
        let observed = try MacAgentStorage.inspectExisting(for:endpoint,resolveContainer:{ _ in root }).validatedDirectory()
        let elapsed = start.duration(to:.now).components
        samples.append(Double(elapsed.seconds) * 1000 + Double(elapsed.attoseconds) / 1e15)
        #expect(observed == directory)
    }
    let destination = try #require(ProcessInfo.processInfo.environment["FORT_STORAGE_INSPECTION_SAMPLES"])
    try JSONEncoder().encode(samples).write(to:URL(fileURLWithPath:destination),options:.withoutOverwriting)
    #expect(samples.sorted()[18] <= 50)
}

@Test func agentStorageProviderCleanupPreservesHistoryAndUnrelatedCredentials() throws {
    let root = try agentStorageRoot(); defer { try? FileManager.default.removeItem(at:root) }
    let storage = try MacAgentStorage.prepare(for:storageEndpoint(),resolveContainer:{ _ in root })
    let runtime = try storage.prepareRuntimeDirectory(), auth = runtime.appendingPathComponent("state/auth.json")
    let retained = [root.appendingPathComponent("auth.json"),runtime.appendingPathComponent("state/history.json"),try storage.validatedDirectory().appendingPathComponent("openai-candidate-b.sqlite")]
    for file in retained + [auth] {
        try Data("synthetic private marker; intentionally not JSON".utf8).write(to:file)
        try FileManager.default.setAttributes([.posixPermissions:0o600],ofItemAtPath:file.path)
    }
    try storage.removeCodexCredential()
    #expect(!FileManager.default.fileExists(atPath:auth.path))
    try storage.removeCodexCredential()
    for file in retained { #expect(try String(contentsOf:file,encoding:.utf8) == "synthetic private marker; intentionally not JSON") }
}

@Test(arguments:["symlink","broken-symlink","hardlink","fifo","directory","shared","executable","acl"])
func agentStorageProviderCleanupRefusesUnsafeCredential(_ fault: String) throws {
    let root = try agentStorageRoot(); defer { try? FileManager.default.removeItem(at:root) }
    let storage = try MacAgentStorage.prepare(for:storageEndpoint(),resolveContainer:{ _ in root })
    let auth = try storage.prepareRuntimeDirectory().appendingPathComponent("state/auth.json")
    let other = root.appendingPathComponent("unrelated-auth")
    try Data("synthetic unrelated account".utf8).write(to:other)
    try FileManager.default.setAttributes([.posixPermissions:0o600],ofItemAtPath:other.path)
    switch fault {
    case "symlink": try FileManager.default.createSymbolicLink(at:auth,withDestinationURL:other)
    case "broken-symlink": try FileManager.default.createSymbolicLink(at:auth,withDestinationURL:root.appendingPathComponent("absent"))
    case "hardlink": try FileManager.default.linkItem(at:other,to:auth)
    case "fifo": #expect(mkfifo(auth.path,0o600) == 0)
    case "directory": try FileManager.default.createDirectory(at:auth,withIntermediateDirectories:false,attributes:[.posixPermissions:0o700])
    default:
        try Data("synthetic Fort credential".utf8).write(to:auth)
        #expect(chmod(auth.path,fault == "shared" ? 0o640 : fault == "executable" ? 0o700 : 0o600) == 0)
        if fault == "acl" { try storageACL(auth,add:true) }
    }
    var before = stat(); #expect(lstat(auth.path,&before) == 0)
    #expect(throws:(any Error).self) { try storage.removeCodexCredential() }
    var after = stat(); #expect(lstat(auth.path,&after) == 0)
    #expect(after.st_ino == before.st_ino && after.st_mode == before.st_mode)
    #expect(try String(contentsOf:other,encoding:.utf8) == "synthetic unrelated account")
    if fault == "acl" { try storageACL(auth,add:false) }
}

@Test(arguments:["Runtime","Runtime/state"], ["missing","symlink","shared","acl"])
func agentStorageProviderCleanupRefusesUnsafeDirectories(_ path: String, _ fault: String) throws {
    let root = try agentStorageRoot(); defer { try? FileManager.default.removeItem(at:root) }
    let storage = try MacAgentStorage.prepare(for:storageEndpoint(),resolveContainer:{ _ in root })
    let runtime = try storage.prepareRuntimeDirectory(), directory = try storage.validatedDirectory()
    let auth = runtime.appendingPathComponent("state/auth.json")
    try Data("synthetic Fort credential".utf8).write(to:auth)
    #expect(chmod(auth.path,0o600) == 0)
    let selected = directory.appendingPathComponent(path), retained = root.appendingPathComponent("retained-directory")
    if fault == "missing" || fault == "symlink" {
        try FileManager.default.moveItem(at:selected,to:retained)
        if fault == "symlink" { try FileManager.default.createSymbolicLink(at:selected,withDestinationURL:retained) }
    } else if fault == "shared" { #expect(chmod(selected.path,0o750) == 0) }
    else { try storageACL(selected,add:true) }
    #expect(throws:(any Error).self) { try storage.removeCodexCredential() }
    let preserved = fault == "missing" || fault == "symlink" ? retained.appendingPathComponent(path == "Runtime" ? "state/auth.json" : "auth.json") : auth
    #expect(try String(contentsOf:preserved,encoding:.utf8) == "synthetic Fort credential")
    if fault == "acl" { try storageACL(selected,add:false) }
}

@Test func agentStorageProviderCleanupRefusesAReplacedHostDirectory() throws {
    let root = try agentStorageRoot(); defer { try? FileManager.default.removeItem(at:root) }
    let storage = try MacAgentStorage.prepare(for:storageEndpoint(),resolveContainer:{ _ in root })
    let directory = try storage.validatedDirectory()
    _ = try storage.prepareRuntimeDirectory()
    try FileManager.default.moveItem(at:directory,to:root.appendingPathComponent("retained-host"))
    let replacement = try MacAgentStorage.prepare(for:storageEndpoint(),resolveContainer:{ _ in root })
    let auth = try replacement.prepareRuntimeDirectory().appendingPathComponent("state/auth.json")
    try Data("synthetic replacement credential".utf8).write(to:auth)
    #expect(chmod(auth.path,0o600) == 0)
    #expect(throws:(any Error).self) { try storage.removeCodexCredential() }
    #expect(try String(contentsOf:auth,encoding:.utf8) == "synthetic replacement credential")
}

@Test(.enabled(if:ProcessInfo.processInfo.environment["FORT_PROVIDER_CLEANUP_SAMPLES"] != nil))
func agentStorageProviderCleanupBudget() throws {
    let root = try agentStorageRoot(); defer { try? FileManager.default.removeItem(at:root) }
    let storage = try MacAgentStorage.prepare(for:storageEndpoint(),resolveContainer:{ _ in root })
    let auth = try storage.prepareRuntimeDirectory().appendingPathComponent("state/auth.json")
    var samples: [Double] = []
    for index in 0..<25 {
        try Data("synthetic credential".utf8).write(to:auth); #expect(chmod(auth.path,0o600) == 0)
        let start = ContinuousClock.now
        try storage.removeCodexCredential()
        let elapsed = start.duration(to:.now).components
        if index >= 5 { samples.append(Double(elapsed.seconds) * 1000 + Double(elapsed.attoseconds) / 1e15) }
    }
    let destination = try #require(ProcessInfo.processInfo.environment["FORT_PROVIDER_CLEANUP_SAMPLES"])
    try JSONEncoder().encode(samples).write(to:URL(fileURLWithPath:destination),options:.withoutOverwriting)
    #expect(samples.sorted()[18] <= 25)
}
#endif
