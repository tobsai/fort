#if os(macOS)
import Foundation
import Darwin
import Testing
@testable import FortKit

struct HostBundleFixture {
    let root: URL
    let app: URL
    let helper: URL
    let endpoint: MacAgentEndpoint
    let configuration: Data
    var manifest: URL { app.appendingPathComponent("Contents/Resources/fort-host.json") }
    var helperManifest: URL { helper.appendingPathComponent("Contents/Resources/fort-host.json") }
    var launchAgent: URL { app.appendingPathComponent("Contents/Library/LaunchAgents/" + endpoint.serviceIdentifier + ".plist") }
    init() throws {
        root = FileManager.default.temporaryDirectory.appendingPathComponent("Fort059HostBundle-" + UUID().uuidString).resolvingSymlinksInPath()
        app = root.appendingPathComponent("Fort.app",isDirectory:true)
        helper = app.appendingPathComponent("Contents/Helpers/FortHost.app",isDirectory:true)
        endpoint = try MacAgentEndpoint(serviceIdentifier:"io.fort.fixture.host",signingTeam:"ABCDE12345",appGroup:"group.io.fort.fixture")
        configuration = try JSONEncoder().encode(["format":"1","clientIdentifier":"io.fort.fixture","serviceIdentifier":endpoint.serviceIdentifier,"signingTeam":endpoint.signingTeam,"appGroup":endpoint.appGroup])
        for directory in [app.appendingPathComponent("Contents/Resources"),helper.appendingPathComponent("Contents/Resources"),helper.appendingPathComponent("Contents/MacOS"),app.appendingPathComponent("Contents/Library/LaunchAgents")] {
            try FileManager.default.createDirectory(at:directory,withIntermediateDirectories:true)
        }
        try configuration.write(to:manifest); try configuration.write(to:helperManifest)
        let plist: [String:Any] = ["Label":endpoint.serviceIdentifier,"BundleProgram":"Contents/Helpers/FortHost.app/Contents/MacOS/FortHost","MachServices":[endpoint.machServiceName:true]]
        try PropertyListSerialization.data(fromPropertyList:plist,format:.xml,options:0).write(to:launchAgent)
        for bundle in [app,helper] { try Data("synthetic profile presence only".utf8).write(to:bundle.appendingPathComponent("Contents/embedded.provisionprofile")) }
        let executable = helper.appendingPathComponent("Contents/MacOS/FortHost")
        try Data("synthetic executable presence only".utf8).write(to:executable)
        try FileManager.default.setAttributes([.posixPermissions:0o700],ofItemAtPath:executable.path)
    }
    func remove() throws { try FileManager.default.removeItem(at:root) }
    var signature: MacHostCodeSignature { MacHostCodeSignature(appSandbox:true,inheritsSandbox:false,applicationGroups:[endpoint.appGroup]) }
    func inspect() throws -> MacHostBundle {
        try MacHostBundle.inspect(applicationURL:app) { _, _, _ in signature }
    }
    func writeLaunchAgent(_ fields: [String:Any]) throws {
        try PropertyListSerialization.data(fromPropertyList:fields,format:.xml,options:0).write(to:launchAgent)
    }
}

@Test func hostBundleInspectionPreservesExactLayoutAndSeparatesProfilePresence() throws {
    let fixture = try HostBundleFixture(); defer { try? fixture.remove() }
    var checked: [(URL,String,String)] = []
    let result = try MacHostBundle.inspect(applicationURL:fixture.app) { url, identifier, team in
        checked.append((url,identifier,team)); return fixture.signature
    }
    #expect(result.applicationURL == fixture.app)
    #expect(result.helperURL == fixture.helper)
    #expect(result.endpoint == fixture.endpoint)
    #expect(result.clientIdentifier == "io.fort.fixture")
    #expect(result.provisioningEvidence == .presentUnverified)
    try #require(checked.count == 2)
    #expect(checked[0].0 == fixture.app && checked[0].1 == "io.fort.fixture" && checked[0].2 == "ABCDE12345")
    #expect(checked[1].0 == fixture.helper && checked[1].1 == "io.fort.fixture.host")
    #expect(try Data(contentsOf:fixture.manifest) == fixture.configuration)
    #expect(try Data(contentsOf:fixture.helperManifest) == fixture.configuration)
}

@Test func hostBundleInspectionRejectsNumericMachServiceBeforeSignatureChecks() throws {
    let fixture = try HostBundleFixture(); defer { try? fixture.remove() }
    try fixture.writeLaunchAgent(["Label":fixture.endpoint.serviceIdentifier,
        "BundleProgram":"Contents/Helpers/FortHost.app/Contents/MacOS/FortHost",
        "MachServices":[fixture.endpoint.machServiceName:1]])
    var signatureChecks = 0
    do {
        _ = try MacHostBundle.inspect(applicationURL:fixture.app) { _, _, _ in
            signatureChecks += 1; return fixture.signature
        }
        Issue.record("Numeric Mach service value was accepted")
    } catch MacHostBundleError.launchAgent {} catch { Issue.record("Expected a launch definition error, got \(error)") }
    #expect(signatureChecks == 0)
}

@Test(arguments:["extra", "version", "sameIdentity", "invalidClient", "invalidService", "invalidTeam", "invalidGroup", "malformed", "oversized", "missing", "differentHelper"])
func hostBundleInspectionRejectsConfigurationDrift(_ change: String) throws {
    let fixture = try HostBundleFixture(); defer { try? fixture.remove() }
    var fields = try JSONDecoder().decode([String:String].self,from:fixture.configuration)
    switch change {
    case "extra": fields["runtime"] = "/foreign/runtime"
    case "version": fields["format"] = "2"
    case "sameIdentity": fields["clientIdentifier"] = fixture.endpoint.serviceIdentifier
    case "invalidClient": fields["clientIdentifier"] = "bad\"identity"
    case "invalidService": fields["serviceIdentifier"] = "../foreign"
    case "invalidTeam": fields["signingTeam"] = "not-a-team"
    case "invalidGroup": fields["appGroup"] = "../foreign"
    default: break
    }
    var data = try JSONEncoder().encode(fields)
    if change == "malformed" { data = Data("{broken".utf8) }
    if change == "oversized" { data = Data(repeating:32,count:8193) }
    try data.write(to:fixture.manifest); try data.write(to:fixture.helperManifest)
    if change == "missing" { try FileManager.default.removeItem(at:fixture.manifest) }
    if change == "differentHelper" { try (data + Data(" ".utf8)).write(to:fixture.helperManifest) }
    var checked = false
    #expect(throws:(any Error).self) {
        try MacHostBundle.inspect(applicationURL:fixture.app) { _, _, _ in checked = true; return fixture.signature }
    }
    #expect(!checked)
    if change != "missing" { #expect(try Data(contentsOf:fixture.manifest) == data) }
}

@Test(arguments:["label", "program", "machName", "machFalse", "machExtra", "RunAtLoad", "KeepAlive", "EnvironmentVariables", "ProgramArguments"])
func hostBundleInspectionRefusesAlternateStartupBehavior(_ change: String) throws {
    let fixture = try HostBundleFixture(); defer { try? fixture.remove() }
    var fields: [String:Any] = ["Label":fixture.endpoint.serviceIdentifier,
        "BundleProgram":"Contents/Helpers/FortHost.app/Contents/MacOS/FortHost",
        "MachServices":[fixture.endpoint.machServiceName:true]]
    switch change {
    case "label": fields["Label"] = "io.foreign.host"
    case "program": fields["BundleProgram"] = "/foreign/runtime"
    case "machName": fields["MachServices"] = ["group.io.foreign.host":true]
    case "machFalse": fields["MachServices"] = [fixture.endpoint.machServiceName:false]
    case "machExtra": fields["MachServices"] = [fixture.endpoint.machServiceName:true,"foreign":true]
    case "EnvironmentVariables": fields[change] = ["PATH":"/foreign"]
    case "ProgramArguments": fields[change] = ["/foreign/runtime"]
    default: fields[change] = true
    }
    try fixture.writeLaunchAgent(fields)
    let before = try Data(contentsOf:fixture.launchAgent)
    #expect(throws:MacHostBundleError.self) { try fixture.inspect() }
    #expect(try Data(contentsOf:fixture.launchAgent) == before)
}

@Test(arguments:["appSymlink", "helperSymlink", "manifestSymlink", "manifestHardlink", "manifestWritable", "appWritable", "missingExecutable", "nonExecutable", "executableFIFO"])
func hostBundleInspectionRefusesUnsafeLayout(_ change: String) throws {
    let fixture = try HostBundleFixture(); defer { try? fixture.remove() }
    let files = FileManager.default
    var app = fixture.app
    let executable = fixture.helper.appendingPathComponent("Contents/MacOS/FortHost")
    let external = fixture.root.appendingPathComponent("external")
    switch change {
    case "appSymlink":
        app = fixture.root.appendingPathComponent("alias.app")
        try files.createSymbolicLink(at:app,withDestinationURL:fixture.app)
    case "helperSymlink":
        try files.moveItem(at:fixture.helper,to:external)
        try files.createSymbolicLink(at:fixture.helper,withDestinationURL:external)
    case "manifestSymlink", "manifestHardlink":
        try files.moveItem(at:fixture.manifest,to:external)
        if change == "manifestSymlink" { try files.createSymbolicLink(at:fixture.manifest,withDestinationURL:external) }
        else { try files.linkItem(at:external,to:fixture.manifest) }
    case "manifestWritable": try files.setAttributes([.posixPermissions:0o666],ofItemAtPath:fixture.manifest.path)
    case "appWritable": try files.setAttributes([.posixPermissions:0o777],ofItemAtPath:fixture.app.path)
    case "missingExecutable": try files.removeItem(at:executable)
    case "nonExecutable": try files.setAttributes([.posixPermissions:0o600],ofItemAtPath:executable.path)
    case "executableFIFO":
        try files.removeItem(at:executable)
        try #require(mkfifo(executable.path,0o700) == 0)
    default: Issue.record("Unknown fixture mutation")
    }
    #expect(throws:MacHostBundleError.self) {
        try MacHostBundle.inspect(applicationURL:app) { _, _, _ in fixture.signature }
    }
    if change == "manifestSymlink" || change == "manifestHardlink" {
        #expect(try Data(contentsOf:external) == fixture.configuration)
    }
}

@Test(arguments:[false,true],["missing", "empty", "oversized", "symlink"])
func hostBundleInspectionRequiresEachOwnProfile(_ helper: Bool, _ change: String) throws {
    let fixture = try HostBundleFixture(); defer { try? fixture.remove() }
    let profile = (helper ? fixture.helper : fixture.app).appendingPathComponent("Contents/embedded.provisionprofile")
    switch change {
    case "missing": try FileManager.default.removeItem(at:profile)
    case "empty": try Data().write(to:profile)
    case "oversized": try Data(repeating:32,count:1_048_577).write(to:profile)
    case "symlink":
        let other = (helper ? fixture.app : fixture.helper).appendingPathComponent("Contents/embedded.provisionprofile")
        try FileManager.default.removeItem(at:profile)
        try FileManager.default.createSymbolicLink(at:profile,withDestinationURL:other)
    default: Issue.record("Unknown fixture mutation")
    }
    do { _ = try fixture.inspect(); Issue.record("Invalid profile accepted") }
    catch MacHostBundleError.profiles {} catch { Issue.record("Expected profile error, got \(error)") }
}

@Test(arguments:[false,true],["signature", "sandbox", "inherit", "foreignGroup", "extraGroup", "duplicateGroup"])
func hostBundleInspectionRequiresEachIndependentSignature(_ helper: Bool, _ change: String) throws {
    let fixture = try HostBundleFixture(); defer { try? fixture.remove() }
    #expect(throws:MacHostBundleError.self) {
        try MacHostBundle.inspect(applicationURL:fixture.app) { url, _, _ in
            guard url == (helper ? fixture.helper : fixture.app) else { return fixture.signature }
            if change == "signature" { throw MacHostBundleError.signature }
            var groups = [fixture.endpoint.appGroup]
            if change == "foreignGroup" { groups = ["group.io.foreign"] }
            if change == "extraGroup" { groups.append("group.io.foreign") }
            if change == "duplicateGroup" { groups.append(fixture.endpoint.appGroup) }
            return MacHostCodeSignature(appSandbox:change != "sandbox",inheritsSandbox:change == "inherit",applicationGroups:groups)
        }
    }
}

@Test func hostBundleInspectionDoesNotCachePreviouslyAcceptedMetadata() throws {
    let fixture = try HostBundleFixture(); defer { try? fixture.remove() }
    _ = try fixture.inspect()
    try FileManager.default.removeItem(at:fixture.helperManifest)
    #expect(throws:MacHostBundleError.self) { try fixture.inspect() }
    #expect(try Data(contentsOf:fixture.manifest) == fixture.configuration)
}

@Test(.enabled(if:ProcessInfo.processInfo.environment["FORT_TEST_REVIEW_PACKAGE"] != nil))
func hostBundleReviewAssemblyReachesProvisioningGateAndRefusesPublicActivation() async throws {
    let path = try #require(ProcessInfo.processInfo.environment["FORT_TEST_REVIEW_PACKAGE"])
    let app = URL(fileURLWithPath:path,isDirectory:true).resolvingSymlinksInPath()
    let manifest = app.appendingPathComponent("Contents/Resources/fort-host.json")
    let before = try Data(contentsOf:manifest)
    let fields = try JSONDecoder().decode([String:String].self,from:before)
    let group = try #require(fields["appGroup"])
    do {
        _ = try MacHostBundle.inspect(applicationURL:app) { _, _, _ in
            MacHostCodeSignature(appSandbox:true,inheritsSandbox:false,applicationGroups:[group])
        }
        Issue.record("Review assembly unexpectedly contained provisioning evidence")
    } catch MacHostBundleError.profiles { }
    do {
        _ = try await MacHostBundle.inspect(applicationURL:app)
        Issue.record("Unsigned review assembly passed normal inspection")
    } catch MacHostBundleError.signature { }
    #expect(try Data(contentsOf:manifest) == before)
    #expect(!FileManager.default.fileExists(atPath:app.appendingPathComponent("Contents/embedded.provisionprofile").path))
}

@Test func hostBundlePublicInspectionRejectsUnsignedSyntheticBundle() async throws {
    let fixture = try HostBundleFixture(); defer { try? fixture.remove() }
    do { _ = try await MacHostBundle.inspect(applicationURL:fixture.app); Issue.record("Unsigned bundle accepted") }
    catch MacHostBundleError.signature {} catch { Issue.record("Expected signature error, got \(error)") }
}

@Test(.enabled(if:ProcessInfo.processInfo.environment["FORT_HOST_BUNDLE_BENCHMARK"] != nil))
func hostBundleMetadataInspectionBudget() throws {
    let fixture = try HostBundleFixture(); defer { try? fixture.remove() }
    let destination = try #require(ProcessInfo.processInfo.environment["FORT_HOST_BUNDLE_BENCHMARK"])
    var samples: [Double] = []
    for _ in 0..<20 {
        let start = ContinuousClock.now
        _ = try fixture.inspect()
        let duration = start.duration(to:.now).components
        samples.append(Double(duration.seconds) * 1000 + Double(duration.attoseconds) / 1e15)
    }
    try JSONEncoder().encode(samples).write(to:URL(fileURLWithPath:destination))
    #expect(samples.sorted()[18] <= 50)
}

@Test(.enabled(if:ProcessInfo.processInfo.environment["FORT_TEST_SIGNED_HOST_BUNDLE"] != nil))
func hostBundleCodeSignatureVerifiesRetainedSignedProbe() throws {
    let environment = ProcessInfo.processInfo.environment
    let path = try #require(environment["FORT_TEST_SIGNED_HOST_BUNDLE"])
    let identifier = try #require(environment["FORT_TEST_SIGNED_HOST_IDENTIFIER"])
    let team = try #require(environment["FORT_TEST_SIGNED_HOST_TEAM"])
    let app = URL(fileURLWithPath:path,isDirectory:true)
    let signed = try MacHostBundle.codeSignature(at:app,identifier:identifier,team:team)
    #expect(signed.appSandbox && !signed.inheritsSandbox && signed.applicationGroups.isEmpty)
    #expect(Set(signed.entitlementKeys) == ["com.apple.security.app-sandbox"])
    #expect(throws:MacHostBundleError.self) { try MacHostBundle.codeSignature(at:app,identifier:"io.fort.foreign",team:team) }
    #expect(throws:MacHostBundleError.self) { try MacHostBundle.codeSignature(at:app,identifier:identifier,team:"ZZZZZ99999") }
    let copy = FileManager.default.temporaryDirectory.appendingPathComponent("Fort059SignatureCopy-" + UUID().uuidString + ".app")
    try FileManager.default.copyItem(at:app,to:copy)
    defer { try? FileManager.default.removeItem(at:copy) }
    _ = try MacHostBundle.codeSignature(at:copy,identifier:identifier,team:team)
    let resource = "Contents/Resources/runtime-fixture.mjs"
    let original = try Data(contentsOf:app.appendingPathComponent(resource))
    try Data("tampered synthetic resource\n".utf8).write(to:copy.appendingPathComponent(resource))
    #expect(throws:MacHostBundleError.self) { try MacHostBundle.codeSignature(at:copy,identifier:identifier,team:team) }
    #expect(try Data(contentsOf:app.appendingPathComponent(resource)) == original)
}
#endif
