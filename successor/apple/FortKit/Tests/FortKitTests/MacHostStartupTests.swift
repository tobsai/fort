#if os(macOS)
import Foundation
import Testing
@testable import FortKit

private actor StartupBackend: MacHostStartupBackend {
    var current: MacHostStartupStatus = .notRegistered
    var inspections = 0
    var registrations = 0
    var removals = 0
    var holdRemoval = false
    var removalWaiter: CheckedContinuation<Void,Never>?
    var registrationFailure: String?
    var removalFailure: String?
    var registrationResult: MacHostStartupStatus = .enabled
    var holdInspection = false
    var inspectionWaiter: CheckedContinuation<Void,Never>?
    var holdRegistration = false
    var registrationWaiter: CheckedContinuation<Void,Never>?
    func holdUnregistration() { holdRemoval = true }
    func finishUnregistration() { removalWaiter?.resume(); removalWaiter = nil }
    func setStatus(_ value: MacHostStartupStatus) { current = value }
    func failRegistration(_ failure: String?) { registrationFailure = failure }
    func failRemoval(_ failure: String?) { removalFailure = failure }
    func setRegistrationResult(_ value: MacHostStartupStatus) { registrationResult = value }
    func holdStatusRead() { holdInspection = true }
    func finishStatusRead() { holdInspection = false; inspectionWaiter?.resume(); inspectionWaiter = nil }
    func holdRegister() { holdRegistration = true }
    func finishRegister() { registrationWaiter?.resume(); registrationWaiter = nil }
    func status() async -> MacHostStartupStatus {
        inspections += 1
        if holdInspection { await withCheckedContinuation { inspectionWaiter = $0 } }
        return current
    }
    func register() async throws {
        registrations += 1
        if registrationFailure == "before" { throw URLError(.networkConnectionLost) }
        if registrationFailure != "ignored" { current = registrationResult }
        if holdRegistration { await withCheckedContinuation { registrationWaiter = $0 } }
        if registrationFailure == "after" { throw URLError(.networkConnectionLost) }
    }
    func unregister() async throws {
        removals += 1
        if removalFailure == "before" { throw URLError(.networkConnectionLost) }
        if removalFailure != "ignored" { current = .notRegistered }
        if holdRemoval { await withCheckedContinuation { removalWaiter = $0 } }
        if removalFailure == "after" { throw URLError(.networkConnectionLost) }
    }
}
private let startupEndpoint = try! MacAgentEndpoint(serviceIdentifier:"io.fort.synthetic.agent",signingTeam:"AAAAAAAAAA",appGroup:"group.io.fort.synthetic")

@Test @MainActor func macHostStartupInspectsWithoutRegistrationAndEnablesOnlyOnExplicitAction() async throws {
    let backend = StartupBackend(), startup = try MacHostStartup(endpoint:startupEndpoint,backend:backend)
    #expect(startup.plistName == "io.fort.synthetic.agent.plist")
    #expect(startup.status == .unknown && startup.operation == .idle)
    #expect(await backend.inspections == 0)
    #expect(try await startup.inspect() == .notRegistered)
    #expect(await backend.registrations == 0)
    #expect(try await startup.enable() == .enabled)
    #expect(startup.status == .enabled && startup.operation == .idle)
    #expect(try await startup.enable() == .enabled)
    #expect(await backend.registrations == 1)
    #expect(await backend.removals == 0)
}

@Test @MainActor func macHostStartupWaitsForUnregistrationCompletionAndRejectsOverlappingActions() async throws {
    let backend = StartupBackend(), startup = try MacHostStartup(endpoint:startupEndpoint,backend:backend)
    #expect(try await startup.enable() == .enabled)
    await backend.holdUnregistration()
    let work = Task { try await startup.disable() }
    for _ in 0..<100 where await backend.removalWaiter == nil { try await Task.sleep(for:.milliseconds(5)) }
    #expect(startup.operation == .disabling && startup.status == .enabled)
    await #expect(throws:MacHostStartupError.self) { try await startup.inspect() }
    await #expect(throws:MacHostStartupError.self) { try await startup.enable() }
    await #expect(throws:MacHostStartupError.self) { try await startup.disable() }
    #expect(await backend.removals == 1)
    await backend.finishUnregistration()
    #expect(try await work.value == .notRegistered)
    #expect(startup.operation == .idle && startup.status == .notRegistered)
    #expect(try await startup.disable() == .notRegistered)
    #expect(await backend.removals == 1)
    #expect(await backend.registrations == 1)
}

@Test @MainActor func macHostStartupKeepsApprovalSeparateAndReconcilesExternalChanges() async throws {
    let backend = StartupBackend(), startup = try MacHostStartup(endpoint:startupEndpoint,backend:backend)
    await backend.setRegistrationResult(.requiresApproval)
    #expect(try await startup.enable() == .requiresApproval)
    #expect(try await startup.enable() == .requiresApproval)
    #expect(await backend.registrations == 1)
    await backend.setStatus(.enabled)
    #expect(try await startup.inspect() == .enabled)
    await backend.setStatus(.requiresApproval)
    #expect(try await startup.inspect() == .requiresApproval)
    #expect(try await startup.enable() == .requiresApproval)
    #expect(await backend.registrations == 1)
    #expect(try await startup.disable() == .notRegistered)
    #expect(await backend.removals == 1)
}

@Test(arguments:[MacHostStartupStatus.notFound,.unsupported,.unknown]) @MainActor
func macHostStartupRefusesUnavailableStatesWithoutMutation(state: MacHostStartupStatus) async throws {
    let backend = StartupBackend(), startup = try MacHostStartup(endpoint:startupEndpoint,backend:backend)
    await backend.setStatus(state)
    #expect(try await startup.inspect() == state)
    await #expect(throws:MacHostStartupError.self) { try await startup.enable() }
    await #expect(throws:MacHostStartupError.self) { try await startup.disable() }
    #expect(startup.status == state && startup.operation == .idle)
    #expect(await backend.registrations == 0)
    #expect(await backend.removals == 0)
}

@Test(arguments:["before","after","ignored"]) @MainActor
func macHostStartupReconcilesPartialRegistrationBeforeRetryOrReopen(failure: String) async throws {
    let backend = StartupBackend(), startup = try MacHostStartup(endpoint:startupEndpoint,backend:backend)
    await backend.failRegistration(failure)
    if failure == "after" { #expect(try await startup.enable() == .enabled) }
    else { await #expect(throws:MacHostStartupError.self) { try await startup.enable() } }
    #expect(startup.operation == .idle)
    await backend.failRegistration(nil)
    let reopened = try MacHostStartup(endpoint:startupEndpoint,backend:backend)
    #expect(try await reopened.enable() == .enabled)
    #expect(await backend.registrations == (failure == "after" ? 1 : 2))
    #expect(await backend.removals == 0)
}

@Test(arguments:["before","after","ignored"]) @MainActor
func macHostStartupReconcilesPartialRemovalBeforeRetryOrReopen(failure: String) async throws {
    let backend = StartupBackend(), startup = try MacHostStartup(endpoint:startupEndpoint,backend:backend)
    await backend.setStatus(.enabled); await backend.failRemoval(failure)
    if failure == "after" { #expect(try await startup.disable() == .notRegistered) }
    else { await #expect(throws:MacHostStartupError.self) { try await startup.disable() } }
    #expect(startup.operation == .idle)
    await backend.failRemoval(nil)
    let reopened = try MacHostStartup(endpoint:startupEndpoint,backend:backend)
    #expect(try await reopened.disable() == .notRegistered)
    #expect(await backend.removals == (failure == "after" ? 1 : 2))
    #expect(await backend.registrations == 0)
}

@Test(arguments:["inspect","enable","disable"]) @MainActor
func macHostStartupCancellationBeforeDispatchMakesNoBackendCall(action: String) async throws {
    let backend = StartupBackend(), startup = try MacHostStartup(endpoint:startupEndpoint,backend:backend)
    let work = Task {
        switch action {
        case "enable": try await startup.enable()
        case "disable": try await startup.disable()
        default: try await startup.inspect()
        }
    }
    work.cancel()
    await #expect(throws:CancellationError.self) { try await work.value }
    #expect(startup.operation == .idle && startup.status == .unknown)
    #expect(await backend.inspections == 0)
    #expect(await backend.registrations == 0)
    #expect(await backend.removals == 0)
}

@Test @MainActor func macHostStartupCancellationDuringInspectionCannotRegisterAfterward() async throws {
    let backend = StartupBackend(), startup = try MacHostStartup(endpoint:startupEndpoint,backend:backend)
    await backend.holdStatusRead()
    let work = Task { try await startup.enable() }
    for _ in 0..<100 where await backend.inspectionWaiter == nil { try await Task.sleep(for:.milliseconds(5)) }
    #expect(startup.operation == .enabling)
    work.cancel(); await backend.finishStatusRead()
    await #expect(throws:CancellationError.self) { try await work.value }
    #expect(startup.operation == .idle && startup.status == .notRegistered)
    #expect(await backend.registrations == 0)
}

@Test @MainActor func macHostStartupCancellationAfterUnregisterDispatchWaitsAndNeverRegistersAgain() async throws {
    let backend = StartupBackend(), startup = try MacHostStartup(endpoint:startupEndpoint,backend:backend)
    await backend.setStatus(.enabled); await backend.holdUnregistration()
    let work = Task { try await startup.disable() }
    for _ in 0..<100 where await backend.removalWaiter == nil { try await Task.sleep(for:.milliseconds(5)) }
    work.cancel()
    #expect(startup.operation == .disabling && startup.status == .enabled)
    await #expect(throws:MacHostStartupError.self) { try await startup.enable() }
    await backend.finishUnregistration()
    await #expect(throws:CancellationError.self) { try await work.value }
    #expect(startup.operation == .idle && startup.status == .notRegistered)
    #expect(try await startup.disable() == .notRegistered)
    #expect(await backend.removals == 1)
    #expect(await backend.registrations == 0)
}

@Test @MainActor func macHostStartupCancellationAfterRegisterDispatchRetainsObservedStateWithoutUndoingIt() async throws {
    let backend = StartupBackend(), startup = try MacHostStartup(endpoint:startupEndpoint,backend:backend)
    await backend.holdRegister()
    let work = Task { try await startup.enable() }
    for _ in 0..<100 where await backend.registrationWaiter == nil { try await Task.sleep(for:.milliseconds(5)) }
    work.cancel()
    #expect(startup.operation == .enabling && startup.status == .notRegistered)
    await #expect(throws:MacHostStartupError.self) { try await startup.disable() }
    await backend.finishRegister()
    await #expect(throws:CancellationError.self) { try await work.value }
    #expect(startup.operation == .idle && startup.status == .enabled)
    #expect(try await startup.enable() == .enabled)
    #expect(await backend.registrations == 1)
    #expect(await backend.removals == 0)
}

@Test @MainActor func macHostStartupRejectsAnUnusablePlistNameBeforeInspectingTheBackend() async throws {
    let endpoint = try MacAgentEndpoint(serviceIdentifier:String(repeating:"a",count:249) + ".b",signingTeam:"AAAAAAAAAA",appGroup:"group.io.fort.synthetic")
    let backend = StartupBackend()
    #expect(throws:(any Error).self) { try MacHostStartup(endpoint:endpoint,backend:backend) }
    #expect(await backend.inspections == 0)
}

@Test(.enabled(if:ProcessInfo.processInfo.environment["FORT_STARTUP_BENCHMARK"] == "1"))
@MainActor func macHostStartupSyntheticInspectionMeetsLocalProjectionBudget() async throws {
    let backend = StartupBackend()
    await backend.setStatus(.requiresApproval)
    var samples: [Double] = []
    for _ in 0..<20 {
        let startup = try MacHostStartup(endpoint:startupEndpoint,backend:backend)
        let started = ContinuousClock.now
        #expect(try await startup.inspect() == .requiresApproval)
        let elapsed = started.duration(to:.now).components
        samples.append(Double(elapsed.seconds) * 1000 + Double(elapsed.attoseconds) / 1e15)
    }
    let p95 = samples.sorted()[18]
    print("FORT_STARTUP_PROJECTION_BUDGET " + String(decoding:try JSONSerialization.data(withJSONObject:["samplesMs":samples,"p95Ms":p95,"budgetMs":50]),as:UTF8.self))
    #expect(p95 <= 50)
    #expect(await backend.registrations == 0)
    #expect(await backend.removals == 0)
}
#endif
