#if os(macOS)
import Foundation
import Darwin
import Testing
@testable import FortKit

private let inheritedKeys = ["com.apple.security.app-sandbox","com.apple.security.inherit"]
private func programFiles(_ fixture: HostBundleFixture) throws -> [URL] {
    let files = [fixture.helper.appendingPathComponent("Contents/MacOS/node"),fixture.helper.appendingPathComponent("Contents/MacOS/FortProcessGuardian"),fixture.helper.appendingPathComponent("Contents/Resources/host/main.mjs"),fixture.helper.appendingPathComponent("Contents/MacOS/codex"),fixture.helper.appendingPathComponent("Contents/Resources/host/node-openssl.cnf"),fixture.helper.appendingPathComponent("Contents/Resources/host/history-maintenance.mjs")]
    try FileManager.default.createDirectory(at:files[2].deletingLastPathComponent(),withIntermediateDirectories:true)
    for (index,file) in files.enumerated() {
        try Data((index == 4 ? "nodejs_conf = fort_node\n[fort_node]\n" : "synthetic presence only").utf8).write(to:file)
        try FileManager.default.setAttributes([.posixPermissions:[0,1,3].contains(index) ? 0o700 : 0o600],ofItemAtPath:file.path)
    }
    return files
}
private func programSignature(_ fixture: HostBundleFixture, identifier: String) -> MacHostCodeSignature {
    if identifier == fixture.endpoint.serviceIdentifier + ".node" || identifier == fixture.endpoint.serviceIdentifier + ".guardian" || identifier == fixture.endpoint.serviceIdentifier + ".codex" {
        return MacHostCodeSignature(appSandbox:true,inheritsSandbox:true,applicationGroups:[],entitlementKeys:inheritedKeys)
    }
    return fixture.signature
}

@Test func hostProgramInspectsOnlyItsOwnSignedResources() throws {
    let fixture = try HostBundleFixture(); defer { try? fixture.remove() }
    _ = try programFiles(fixture)
    for file in [fixture.manifest,fixture.launchAgent,fixture.app.appendingPathComponent("Contents/embedded.provisionprofile")] {
        try FileManager.default.removeItem(at:file)
    }
    var inspected: [URL] = []
    let result = try MacHostProgram.inspect(helperURL:fixture.helper) { url,identifier,team in
        #expect(team == fixture.endpoint.signingTeam)
        inspected.append(url)
        return programSignature(fixture,identifier:identifier)
    }
    #expect(result.bundle.endpoint == fixture.endpoint)
    #expect(result.bundle.clientIdentifier == "io.fort.fixture")
    #expect(inspected.allSatisfy { $0 == fixture.helper || $0.path.hasPrefix(fixture.helper.path + "/") })
}

@Test func hostProgramPinsItsOwnNestedBundleAndFixedChildCodeRequirements() throws {
    let fixture = try HostBundleFixture(); defer { try? fixture.remove() }
    let files = try programFiles(fixture)
    var checks: [(URL,String,String)] = []
    let result = try MacHostProgram.inspect(helperURL:fixture.helper) { url, identifier, team in
        checks.append((url,identifier,team)); return programSignature(fixture,identifier:identifier)
    }
    #expect(result.bundle.helperURL == fixture.helper && result.bundle.applicationURL == fixture.app)
    #expect(result.executable == files[0] && result.guardian == files[1] && result.bootstrap == files[2] && result.historyMaintenance == files[5])
    try #require(checks.count == 4)
    #expect(result.codex == files[3] && checks[3].0 == files[3] && checks[3].1 == fixture.endpoint.serviceIdentifier + ".codex")
    #expect(checks[1].0 == files[0] && checks[1].1 == fixture.endpoint.serviceIdentifier + ".node")
    #expect(checks[2].0 == files[1] && checks[2].1 == fixture.endpoint.serviceIdentifier + ".guardian")
    #expect(checks.allSatisfy { $0.2 == fixture.endpoint.signingTeam })
}

@Test(arguments:["manifest","profile","sandbox","inheritance","group","signature","layout"])
func hostProgramRejectsInvalidOwnBundle(_ variant: String) throws {
    let fixture = try HostBundleFixture(); defer { try? fixture.remove() }
    _ = try programFiles(fixture)
    if variant == "manifest" { try Data("{}".utf8).write(to:fixture.helperManifest) }
    if variant == "profile" { try FileManager.default.removeItem(at:fixture.helper.appendingPathComponent("Contents/embedded.provisionprofile")) }
    var helper = fixture.helper
    if variant == "layout" {
        helper = fixture.helper.deletingLastPathComponent().appendingPathComponent("Foreign.app",isDirectory:true)
        try FileManager.default.moveItem(at:fixture.helper,to:helper)
    }
    #expect(throws:(any Error).self) {
        try MacHostProgram.inspect(helperURL:helper) { _,identifier,_ in
            guard identifier == fixture.endpoint.serviceIdentifier else { return programSignature(fixture,identifier:identifier) }
            if variant == "signature" { throw MacHostBundleError.signature }
            return MacHostCodeSignature(appSandbox:variant != "sandbox",inheritsSandbox:variant == "inheritance",
                applicationGroups:variant == "group" ? ["group.foreign"] : [fixture.endpoint.appGroup])
        }
    }
}

@Test(arguments:["node","guardian","codex"], ["sandbox", "inheritance", "group", "extra-key", "missing-key", "signature"])
func hostProgramRejectsInvalidInheritedToolAuthority(_ tool: String, _ variant: String) throws {
    let fixture = try HostBundleFixture(); defer { try? fixture.remove() }
    _ = try programFiles(fixture)
    #expect(throws:(any Error).self) {
        try MacHostProgram.inspect(helperURL:fixture.helper) { _, identifier, _ in
            guard identifier == fixture.endpoint.serviceIdentifier + "." + tool else { return programSignature(fixture,identifier:identifier) }
            if variant == "signature" { throw MacHostBundleError.signature }
            return MacHostCodeSignature(appSandbox:variant != "sandbox",inheritsSandbox:variant != "inheritance",
                applicationGroups:variant == "group" ? [fixture.endpoint.appGroup] : [],
                entitlementKeys:variant == "extra-key" ? inheritedKeys + ["com.apple.security.network.client"] : variant == "missing-key" ? [inheritedKeys[0]] : inheritedKeys)
        }
    }
}

@Test(arguments:[0,1,2,3,4,5], ["missing", "empty", "directory", "symlink", "hardlink", "writable", "oversize"])
func hostProgramRejectsUnsafeOrMissingRuntimeFiles(_ index: Int, _ variant: String) throws {
    let fixture = try HostBundleFixture(); defer { try? fixture.remove() }
    let files = try programFiles(fixture), file = files[index]
    if ["missing","directory","symlink"].contains(variant) { try FileManager.default.removeItem(at:file) }
    switch variant {
    case "empty": try Data().write(to:file)
    case "directory": try FileManager.default.createDirectory(at:file,withIntermediateDirectories:false)
    case "symlink": try FileManager.default.createSymbolicLink(at:file,withDestinationURL:files[(index + 1) % files.count])
    case "hardlink": #expect(link(file.path,fixture.root.appendingPathComponent("extra-link").path) == 0)
    case "writable": try FileManager.default.setAttributes([.posixPermissions:0o777],ofItemAtPath:file.path)
    case "oversize":
        let handle = try FileHandle(forWritingTo:file); defer { try? handle.close() }
        let limits: [UInt64] = [512 * 1024 * 1024,8 * 1024 * 1024,256 * 1024,512 * 1024 * 1024,1024,256 * 1024]
        try handle.truncate(atOffset:limits[index] + 1)
    default: break
    }
    #expect(throws:(any Error).self) { try MacHostProgram.inspect(helperURL:fixture.helper) { _, identifier, _ in programSignature(fixture,identifier:identifier) } }
}

@Test func hostProgramRefusesAlteredNodeConfiguration() throws {
    let fixture = try HostBundleFixture(); defer { try? fixture.remove() }
    let files = try programFiles(fixture)
    try Data("nodejs_conf = foreign\n[foreign]\n".utf8).write(to:files[4])
    #expect(throws:(any Error).self) { try MacHostProgram.inspect(helperURL:fixture.helper) { _, identifier, _ in programSignature(fixture,identifier:identifier) } }
}

@Test(arguments:[0,1,3]) func hostProgramRequiresExecutableNativeTools(_ index: Int) throws {
    let fixture = try HostBundleFixture(); defer { try? fixture.remove() }
    let files = try programFiles(fixture)
    try FileManager.default.setAttributes([.posixPermissions:0o600],ofItemAtPath:files[index].path)
    #expect(throws:(any Error).self) { try MacHostProgram.inspect(helperURL:fixture.helper) { _, identifier, _ in programSignature(fixture,identifier:identifier) } }
}

@Test func hostProgramPublicInspectionRefusesTheTestRunnerAsAHelper() async {
    await #expect(throws:(any Error).self) { try await MacHostProgram.inspect() }
}

@Test(.enabled(if:ProcessInfo.processInfo.environment["FORT_HOST_PROGRAM_BENCHMARK"] != nil))
func hostProgramMetadataInspectionBudget() throws {
    let fixture = try HostBundleFixture(); defer { try? fixture.remove() }
    _ = try programFiles(fixture)
    var samples: [Double] = []
    for _ in 0..<20 {
        let start = ContinuousClock.now
        _ = try MacHostProgram.inspect(helperURL:fixture.helper) { _, identifier, _ in programSignature(fixture,identifier:identifier) }
        let value = start.duration(to:.now).components
        samples.append(Double(value.seconds) * 1000 + Double(value.attoseconds) / 1e15)
    }
    let destination = try #require(ProcessInfo.processInfo.environment["FORT_HOST_PROGRAM_BENCHMARK"])
    try JSONEncoder().encode(samples).write(to:URL(fileURLWithPath:destination),options:.withoutOverwriting)
    #expect(samples.sorted()[18] <= 50)
}
#endif
