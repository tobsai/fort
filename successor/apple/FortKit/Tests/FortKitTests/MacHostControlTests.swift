#if os(macOS)
import Foundation
import Testing
@testable import FortKit

private actor HostControlBackend: MacHostStartupBackend {
    var current: MacHostStartupStatus = .notRegistered
    var inspections = 0
    var registrations = 0
    var unregistrations = 0
    var removalFails = false
    func setRemovalFails() { removalFails = true }
    var registrationResult: MacHostStartupStatus = .enabled
    func setStatus(_ value: MacHostStartupStatus) { current = value }
    func setRegistrationResult(_ value: MacHostStartupStatus) { registrationResult = value }
    func status() -> MacHostStartupStatus { inspections += 1; return current }
    func register() { registrations += 1; current = registrationResult }
    func unregister() { unregistrations += 1; if !removalFails { current = .notRegistered } }
}
@MainActor private final class HostControlFixture {
    let app = URL(fileURLWithPath:"/synthetic/Fort.app",isDirectory:true)
    let endpoint = try! MacAgentEndpoint(serviceIdentifier:"io.fort.control.host",signingTeam:"ABCDE12345",appGroup:"group.io.fort.control")
    let backend = HostControlBackend()
    var inspections = 0
    var invalidSignature = false
    var changedIdentity = false
    var wrongApp = false
    var unavailableStartup = false
    var hostPrepared = false
    var holdInspection = false
    var waiting: CheckedContinuation<Void,Never>?
    func inspect(_ url: URL) async throws -> MacHostBundle {
        #expect(url == app); inspections += 1
        if holdInspection { await withCheckedContinuation { waiting = $0 } }
        if invalidSignature { throw MacHostBundleError.signature }
        return MacHostBundle(applicationURL:wrongApp ? URL(fileURLWithPath:"/foreign/Fort.app") : app,
            helperURL:app.appendingPathComponent("Contents/Helpers/FortHost.app",isDirectory:true),endpoint:endpoint,
            clientIdentifier:changedIdentity ? "io.fort.foreign" : "io.fort.control")
    }
    func control() -> MacHostControl {
        MacHostControl(applicationURL:app,inspectBundle:inspect,makeStartup:{
            if self.unavailableStartup { throw MacHostStartupError.unavailable }
            return try MacHostStartup(endpoint:$0,backend:self.backend)
        },requireUnprepared:{ _ in if self.hostPrepared { throw MacAgentStorageError.unsafeDirectory } })
    }
}

@Test @MainActor func hostControlRemovesFreshRegistrationBeforePairing() async throws {
    let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
    try FileManager.default.createDirectory(at:root,withIntermediateDirectories:false,attributes:[.posixPermissions:0o700])
    defer { try? FileManager.default.removeItem(at:root) }
    let fixture = HostControlFixture(), control = fixture.control()
    await fixture.backend.setStatus(.enabled); await control.inspect()
    try await control.disableUnpaired(storageDirectory:root)
    #expect(control.status == .notRegistered)
    #expect(await fixture.backend.unregistrations == 1)
    #expect(try FileManager.default.contentsOfDirectory(atPath:root.path) == ["owner-setup.lock"])
}


@Test(arguments:["connection.json","owner-setup.json","disconnected.json","pairing-removal.json","host-removal.json","history-removal.json","account-sign-in.json","companion-setup.json","host-data","signature","identity","unregister-failure"])
@MainActor func hostControlUnpairedRemovalRefusesSavedOrUnconfirmedState(_ variant: String) async throws {
    let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
    try FileManager.default.createDirectory(at:root,withIntermediateDirectories:false,attributes:[.posixPermissions:0o700])
    defer { try? FileManager.default.removeItem(at:root) }
    let fixture = HostControlFixture(), control = fixture.control()
    await fixture.backend.setStatus(.enabled); await control.inspect()
    if variant.hasSuffix(".json") {
        let file = root.appendingPathComponent(variant)
        try Data("preserve existing state".utf8).write(to:file)
        try FileManager.default.setAttributes([.posixPermissions:0o600],ofItemAtPath:file.path)
    }
    if variant == "host-data" { fixture.hostPrepared = true }
    if variant == "signature" { fixture.invalidSignature = true }
    if variant == "identity" { fixture.changedIdentity = true }
    if variant == "unregister-failure" { await fixture.backend.setRemovalFails() }
    await #expect(throws:(any Error).self) { try await control.disableUnpaired(storageDirectory:root) }
    #expect(await fixture.backend.unregistrations == (variant == "unregister-failure" ? 1 : 0))
    #expect(control.status != .notRegistered)
    if variant.hasSuffix(".json") { #expect(try Data(contentsOf:root.appendingPathComponent(variant)) == Data("preserve existing state".utf8)) }
}

@Test @MainActor func hostControlConstructionAndInspectionNeverRegisterOrPair() async throws {
    let fixture = HostControlFixture(), control = fixture.control()
    #expect(fixture.inspections == 0 && control.status == .unknown && !control.canEnable && !control.canPair)
    #expect(await fixture.backend.inspections == 0)
    #expect(await fixture.backend.registrations == 0)
    await control.inspect()
    #expect(fixture.inspections == 1 && control.status == .notRegistered && control.canEnable && !control.canPair)
    #expect(control.endpoint == fixture.endpoint)
    #expect(await fixture.backend.inspections == 1)
    #expect(await fixture.backend.registrations == 0)
    await control.enable()
    #expect(fixture.inspections == 2 && control.status == .enabled && !control.canEnable && control.canPair)
    #expect(await fixture.backend.registrations == 1)
    await control.enable()
    #expect(await fixture.backend.registrations == 1)
}

@Test @MainActor func hostControlOffersExplicitRegistrationForAVerifiedUnseenHost() async throws {
    let fixture = HostControlFixture(), control = fixture.control()
    await fixture.backend.setStatus(.notFound)
    await fixture.backend.setRegistrationResult(.requiresApproval)
    await control.inspect()
    #expect(control.status == .notFound && control.canEnable && !control.canPair)
    #expect(await fixture.backend.registrations == 0)
    await control.enable()
    #expect(fixture.inspections == 2)
    #expect(control.status == .requiresApproval && !control.canEnable && !control.canPair)
    #expect(await fixture.backend.registrations == 1)
    await fixture.backend.setStatus(.enabled)
    await control.inspect()
    #expect(control.canPair && !control.canEnable)
}

@Test @MainActor func hostControlPublicConstructionOnlyTargetsItsRunningApp() {
    let control = MacHostControl()
    #expect(control.applicationURL == Bundle.main.bundleURL)
    #expect(control.status == .unknown && control.endpoint == nil && !control.canEnable && !control.canPair)
}

@Test @MainActor func hostControlKeepsVerifiedIdentityWhenStatusIsUnavailable() async throws {
    let fixture = HostControlFixture(), control = fixture.control()
    fixture.unavailableStartup = true
    await control.inspect()
    #expect(control.status == .unknown && control.endpoint == nil)
    fixture.changedIdentity = true; fixture.unavailableStartup = false
    await control.inspect()
    #expect(control.status == .unknown && !control.canEnable && control.endpoint == nil)
    #expect(await fixture.backend.inspections == 0)
}

@Test(arguments:["signature", "identity", "application"], [MacHostStartupStatus.notRegistered,.notFound]) @MainActor
func hostControlRechecksThePackageBeforeRegistration(_ change: String, _ state: MacHostStartupStatus) async throws {
    let fixture = HostControlFixture(), control = fixture.control()
    await fixture.backend.setStatus(state)
    await control.inspect()
    if change == "signature" { fixture.invalidSignature = true }
    if change == "identity" { fixture.changedIdentity = true }
    if change == "application" { fixture.wrongApp = true }
    await control.enable()
    #expect(control.status == .unknown && control.endpoint == nil && !control.canEnable && !control.canPair)
    #expect(fixture.inspections == 2)
    #expect(await fixture.backend.inspections == 1)
    #expect(await fixture.backend.registrations == 0)
}

@Test(arguments:[MacHostStartupStatus.enabled,.requiresApproval,.unsupported,.unknown]) @MainActor
func hostControlKeepsOSStatesDistinctWithoutRegistering(_ state: MacHostStartupStatus) async throws {
    let fixture = HostControlFixture(), control = fixture.control()
    await fixture.backend.setStatus(state)
    await control.inspect(); await control.enable()
    #expect(control.status == state && !control.canEnable && control.canPair == (state == .enabled))
    #expect(await fixture.backend.registrations == 0)
}

@Test @MainActor func hostControlApprovalRequiredDoesNotAuthorizePairing() async throws {
    let fixture = HostControlFixture(), control = fixture.control()
    await fixture.backend.setRegistrationResult(.requiresApproval)
    await control.inspect(); await control.enable()
    #expect(control.status == .requiresApproval && !control.canPair && !control.canEnable)
    await control.enable()
    #expect(await fixture.backend.registrations == 1)
    await fixture.backend.setStatus(.enabled)
    await control.inspect()
    #expect(control.canPair && !control.canEnable)
}

@Test @MainActor func hostControlCanceledInspectionBlocksConcurrentEnableAndNeedsFreshStatus() async throws {
    let fixture = HostControlFixture(), control = fixture.control()
    await control.inspect()
    fixture.holdInspection = true
    let task = Task { await control.inspect() }
    defer { fixture.waiting?.resume(); fixture.waiting = nil }
    for _ in 0..<1000 {
        if fixture.waiting != nil { break }
        try await Task.sleep(for:.milliseconds(1))
    }
    try #require(fixture.waiting != nil)
    #expect(control.isBusy && !control.canEnable && !control.canPair)
    await control.enable()
    task.cancel(); fixture.waiting?.resume(); fixture.waiting = nil
    await task.value
    #expect(!control.isBusy && control.status == .unknown && control.endpoint == nil)
    #expect(await fixture.backend.registrations == 0)
    #expect(await fixture.backend.inspections == 1)
    fixture.holdInspection = false
    await control.inspect()
    #expect(control.canEnable)
}

@Test(.enabled(if:ProcessInfo.processInfo.environment["FORT_HOST_CONTROL_BENCHMARK"] != nil)) @MainActor
func hostControlSyntheticInspectionBudget() async throws {
    let fixture = HostControlFixture(), control = fixture.control()
    let destination = try #require(ProcessInfo.processInfo.environment["FORT_HOST_CONTROL_BENCHMARK"])
    var samples: [Double] = []
    for _ in 0..<20 {
        let start = ContinuousClock.now
        await control.inspect()
        let duration = start.duration(to:.now).components
        samples.append(Double(duration.seconds) * 1000 + Double(duration.attoseconds) / 1e15)
        #expect(control.canEnable && !control.canPair)
    }
    #expect(await fixture.backend.registrations == 0)
    try JSONEncoder().encode(samples).write(to:URL(fileURLWithPath:destination))
    #expect(samples.sorted()[18] <= 50)
}
#endif
