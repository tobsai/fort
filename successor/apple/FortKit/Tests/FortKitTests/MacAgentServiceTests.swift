#if os(macOS)
import Foundation
import Darwin
import Testing
@testable import FortKit

@MainActor private final class AgentListenerFixture: MacAgentServiceListener {
    var resumed = 0
    var invalidated = 0
    var refuse = false
    func resume() throws { resumed += 1; if refuse { throw ClientError.invalidResponse } }
    func invalidate() { invalidated += 1 }
}
@MainActor private final class AgentServiceFixture {
    let root: URL
    let endpoint = try! MacAgentEndpoint(serviceIdentifier:"io.fort.service.host",signingTeam:"ABCDE12345",appGroup:"group.io.fort.service")
    let listener = AgentListenerFixture()
    var prepared = 0
    var launches = 0
    var listeners = 0
    var refuseStorage = false
    var refuseLaunch = false
    var refuseListener = false
    var holdLaunch = false
    var waiting: CheckedContinuation<Void,Never>?
    var core: MacCoreHost?
    var pid: Int32?
    var directory: URL?
    init() throws {
        root = FileManager.default.temporaryDirectory.appendingPathComponent("fort-agent-service-" + UUID().uuidString,isDirectory:true)
        try FileManager.default.createDirectory(at:root,withIntermediateDirectories:false,attributes:[.posixPermissions:0o700])
    }
    func service() -> MacAgentService {
        MacAgentService(prepareStorage:{
            self.prepared += 1
            if self.refuseStorage { throw MacAgentStorageError.groupUnavailable }
            return try MacAgentStorage.prepare(for:self.endpoint,resolveContainer:{ _ in self.root })
        },launch:{ directory in
            self.launches += 1; self.directory = directory
            if self.refuseLaunch { throw ClientError.invalidResponse }
            let transport = try await setupCore(directory,enroll:false)
            self.pid = transport.processID
            let core = MacCoreHost(core:transport,directory:directory); self.core = core
            if self.holdLaunch { await withCheckedContinuation { self.waiting = $0 } }
            return core
        },makeListener:{ core in
            #expect(self.core === core)
            self.listeners += 1
            if self.refuseListener { throw ClientError.invalidResponse }
            return self.listener
        })
    }
    func waitForLaunch() async throws {
        for _ in 0..<1000 where waiting == nil { try await Task.sleep(for:.milliseconds(5)) }
        #expect(waiting != nil)
    }
    func release() { waiting?.resume(); waiting = nil }
    func absent() {
        guard let pid else { return }
        #expect(kill(pid,0) == -1 && errno == ESRCH)
        #expect(kill(-pid,0) == -1 && errno == ESRCH)
    }
    func cleanup() async {
        release(); _ = try? await core?.shutdown()
        try? FileManager.default.removeItem(at:root)
    }
}

@Test @MainActor func agentServiceIsInertUntilStartedAndOwnsOneListenerAndCore() async throws {
    let fixture = try AgentServiceFixture(), service = fixture.service()
    do {
        #expect(fixture.prepared == 0 && fixture.launches == 0 && fixture.listeners == 0)
        try await service.start()
        #expect(fixture.prepared == 1 && fixture.launches == 1 && fixture.listeners == 1)
        #expect(fixture.listener.resumed == 1 && fixture.listener.invalidated == 0)
        let core = try #require(fixture.core)
        guard case .available = try await core.beginOwnerSetup(timeout:3) else { Issue.record("Expected ready synthetic core"); await fixture.cleanup(); return }
        await #expect(throws:(any Error).self) { try await service.start() }
        let stopped = try await service.shutdown()
        #expect(stopped.exitedCleanly && !stopped.forced)
        #expect(try await service.shutdown().exitedCleanly)
        #expect(fixture.listener.invalidated == 1 && fixture.launches == 1)
        fixture.absent()
        await #expect(throws:(any Error).self) { try await service.start() }
    } catch { await fixture.cleanup(); throw error }
    await fixture.cleanup()
}

@Test @MainActor func agentServiceStorageFailureStartsNothingAndShutdownPreventsLaterStart() async throws {
    let fixture = try AgentServiceFixture(); fixture.refuseStorage = true
    let service = fixture.service()
    await #expect(throws:(any Error).self) { try await service.start() }
    #expect(fixture.launches == 0 && fixture.listeners == 0)
    #expect(try await service.shutdown().exitedCleanly)
    fixture.refuseStorage = false
    await #expect(throws:(any Error).self) { try await service.start() }
    #expect(fixture.prepared == 1 && fixture.launches == 0)
    let neverStarted = fixture.service()
    #expect(try await neverStarted.shutdown().exitedCleanly)
    await #expect(throws:(any Error).self) { try await neverStarted.start() }
    await fixture.cleanup()
}

@Test(arguments:[false,true]) @MainActor func agentServiceListenerFailureClosesTheReadyCoreBeforeReturning(construction: Bool) async throws {
    let fixture = try AgentServiceFixture(); fixture.listener.refuse = !construction; fixture.refuseListener = construction
    let service = fixture.service()
    await #expect(throws:(any Error).self) { try await service.start() }
    #expect(fixture.listener.resumed == (construction ? 0 : 1) && fixture.listener.invalidated == (construction ? 0 : 1))
    fixture.absent()
    #expect(try await service.shutdown().exitedCleanly)
    #expect(fixture.listener.invalidated == (construction ? 0 : 1))
    await fixture.cleanup()
}

@Test @MainActor func agentServiceCanceledStartClosesTheCoreWithoutOpeningItsListener() async throws {
    let fixture = try AgentServiceFixture(); fixture.holdLaunch = true
    let service = fixture.service(), pending = Task { try await service.start() }
    try await fixture.waitForLaunch(); pending.cancel(); fixture.release()
    await #expect(throws:(any Error).self) { try await pending.value }
    #expect(fixture.listener.resumed == 0 && fixture.listeners == 0)
    #expect(try await service.shutdown().exitedCleanly); fixture.absent()
    await fixture.cleanup()
}

@Test @MainActor func agentServiceConcurrentShutdownWaitsForAnInProgressLaunch() async throws {
    let fixture = try AgentServiceFixture(); fixture.holdLaunch = true
    let service = fixture.service(), starting = Task { try await service.start() }
    try await fixture.waitForLaunch()
    await #expect(throws:MacAgentServiceError.self) { try await service.start() }
    let first = Task { try await service.shutdown() }, second = Task { try await service.shutdown() }
    await Task.yield(); fixture.release()
    #expect(try await first.value.exitedCleanly)
    #expect(try await second.value.exitedCleanly)
    await #expect(throws:(any Error).self) { try await starting.value }
    #expect(fixture.launches == 1 && fixture.listeners == 0); fixture.absent()
    await fixture.cleanup()
}

@Test @MainActor func agentServiceLaunchFailureCannotBecomeACleanShutdownReceipt() async throws {
    let fixture = try AgentServiceFixture(); fixture.refuseLaunch = true
    let service = fixture.service()
    await #expect(throws:(any Error).self) { try await service.start() }
    await #expect(throws:MacAgentServiceError.self) { try await service.shutdown() }
    await #expect(throws:(any Error).self) { try await service.start() }
    #expect(fixture.launches == 1 && fixture.listeners == 0)
    await fixture.cleanup()
}

@Test @MainActor func agentServiceRecoveryCanReplyBeforeTheHelperListenerIsStopped() async throws {
    let fixture = try AgentServiceFixture(), service = fixture.service()
    do {
        try await service.start()
        let host = try #require(fixture.core)
        guard case .available(let offer) = try await host.beginOwnerSetup(timeout:3) else { throw ClientError.invalidResponse }
        let setup = try OwnerSetupRequest(installationID:offer.installationID,setupID:offer.setupID,deviceID:"synthetic-device",deviceToken:String(repeating:"A",count:43))
        _ = try await host.completeOwnerSetup(setup,timeout:3)
        let request = try MacOwnerRecoveryRequest(installationID:setup.installationID,deviceID:setup.deviceID,timeout:3)
        try await host.recoverPairing(request).validate(for:request)
        #expect(fixture.listener.resumed == 1 && fixture.listener.invalidated == 0); fixture.absent()
        #expect(try await service.shutdown().exitedCleanly)
        #expect(fixture.listener.invalidated == 1)
    } catch { _ = try? await service.shutdown(); await fixture.cleanup(); throw error }
    await fixture.cleanup()
}

@Test @MainActor func agentServiceRevalidatesStorageAfterLaunchBeforeOpeningTheListener() async throws {
    let fixture = try AgentServiceFixture(); fixture.holdLaunch = true
    let service = fixture.service(), starting = Task { try await service.start() }
    try await fixture.waitForLaunch()
    let directory = try #require(fixture.directory)
    try FileManager.default.moveItem(at:directory,to:fixture.root.appendingPathComponent("retained-original",isDirectory:true))
    try FileManager.default.createDirectory(at:directory,withIntermediateDirectories:false,attributes:[.posixPermissions:0o700])
    fixture.release()
    await #expect(throws:(any Error).self) { try await starting.value }
    #expect(fixture.listeners == 0 && fixture.listener.resumed == 0); fixture.absent()
    #expect(try await service.shutdown().exitedCleanly)
    await fixture.cleanup()
}

@Test @MainActor func agentServiceAlreadyCanceledStartDoesNotPrepareStorage() async throws {
    let fixture = try AgentServiceFixture(), service = fixture.service()
    let canceled = Task { withUnsafeCurrentTask { $0?.cancel() }; try await service.start() }
    await #expect(throws:CancellationError.self) { try await canceled.value }
    #expect(fixture.prepared == 0 && fixture.launches == 0 && fixture.listeners == 0)
    #expect(try await service.shutdown().exitedCleanly)
    await fixture.cleanup()
}

@Test @MainActor func agentServicePublicConstructionDoesNotResolveAnAppGroupOrLaunch() throws {
    let endpoint = try MacAgentEndpoint(serviceIdentifier:"io.fort.inert.host",signingTeam:"ABCDE12345",appGroup:"group.io.fort.inert")
    let app = URL(fileURLWithPath:"/synthetic/Fort.app",isDirectory:true)
    let bundle = MacHostBundle(applicationURL:app,helperURL:app.appendingPathComponent("Contents/Helpers/FortHost.app",isDirectory:true),endpoint:endpoint,clientIdentifier:"io.fort.inert")
    _ = MacAgentService(bundle:bundle,executable:URL(fileURLWithPath:"/missing/node"),arguments:[],environment:[:],guardian:URL(fileURLWithPath:"/missing/guardian"))
}

@Test @MainActor func agentServiceForcedCoreCleanupRemainsUnconfirmedOnEveryRetry() async throws {
    let fixture = try AgentServiceFixture()
    let environment = ProcessInfo.processInfo.environment
    let node = URL(fileURLWithPath:try #require(environment["FORT_TEST_NODE"]))
    let source = URL(fileURLWithPath:#filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
    var process: MacCoreProcessTransport?
    let service = MacAgentService(prepareStorage:{ try MacAgentStorage.prepare(for:fixture.endpoint,resolveContainer:{ _ in fixture.root }) },launch:{ directory in
        let transport = try await MacCoreProcessTransport.launch(executable:node,arguments:[source.appendingPathComponent("host/fixtures/native-channel.mjs").path,"hang"],directory:directory,environment:[:],lifecycle:.processGroup)
        process = transport; fixture.pid = transport.processID; fixture.directory = directory
        let core = MacCoreHost(core:transport,directory:directory); fixture.core = core
        return core
    },makeListener:{ _ in fixture.listener })
    do {
        try await service.start()
        let directory = try #require(fixture.directory)
        for _ in 0..<500 where !FileManager.default.fileExists(atPath:directory.appendingPathComponent("descendant-pid.json").path) { try await Task.sleep(for:.milliseconds(5)) }
        let descendant = try JSONDecoder().decode(Int32.self,from:Data(contentsOf:directory.appendingPathComponent("descendant-pid.json")))
        await #expect(throws:MacAgentServiceError.self) { try await service.shutdown() }
        await #expect(throws:MacAgentServiceError.self) { try await service.shutdown() }
        #expect(fixture.listener.invalidated == 1); fixture.absent()
        #expect(kill(descendant,0) == -1 && errno == ESRCH)
        await #expect(throws:(any Error).self) { try await service.start() }
    } catch { _ = try? await process?.close(); await fixture.cleanup(); throw error }
    await fixture.cleanup()
}

@MainActor private func guardedServiceJourney() async throws -> Double {
    let environment = ProcessInfo.processInfo.environment
    let node = URL(fileURLWithPath:try #require(environment["FORT_TEST_NODE"]))
    let guardian = URL(fileURLWithPath:try #require(environment["FORT_TEST_GUARDIAN"]))
    let artifact = URL(fileURLWithPath:try #require(environment["FORT_TEST_CORE_ARTIFACT"]))
    let source = URL(fileURLWithPath:#filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
    let fixture = try AgentServiceFixture()
    var group: Int32 = -1
    let service = MacAgentService(prepareStorage:{ try MacAgentStorage.prepare(for:fixture.endpoint,resolveContainer:{ _ in fixture.root }) },launch:{ directory in
        fixture.directory = directory
        let host = try await MacCoreHost.launch(executable:node,
            arguments:["--jitless","--no-experimental-strip-types","--no-warnings",source.appendingPathComponent("host/fixtures/owned-core.mjs").path,artifact.appendingPathComponent("host/owned-core.js").path,directory.path,"restore","ordinary"],
            directory:directory,environment:["PATH":"/usr/bin:/bin","HOME":directory.path,"TMPDIR":directory.path],guardian:guardian)
        fixture.core = host
        let pid = try JSONDecoder().decode(Int32.self,from:Data(contentsOf:directory.appendingPathComponent("core-pid.json")))
        fixture.pid = pid; group = getpgid(pid)
        #expect(group > 1 && group != pid && group != getpgrp())
        return host
    },makeListener:{ _ in fixture.listener })
    let started = ContinuousClock.now
    do {
        try await service.start()
        #expect(fixture.listener.resumed == 1)
        let host = try #require(fixture.core)
        guard case .available = try await host.beginOwnerSetup(timeout:3) else { throw ClientError.invalidResponse }
        #expect(try await service.shutdown().exitedCleanly)
        #expect(fixture.listener.invalidated == 1)
        let pid = try #require(fixture.pid)
        #expect(kill(pid,0) == -1 && errno == ESRCH)
        #expect(kill(group,0) == -1 && errno == ESRCH)
        #expect(kill(-group,0) == -1 && errno == ESRCH)
        let elapsed = started.duration(to:.now).components
        let milliseconds = Double(elapsed.seconds) * 1000 + Double(elapsed.attoseconds) / 1e15
        await fixture.cleanup()
        return milliseconds
    } catch { _ = try? await service.shutdown(); await fixture.cleanup(); throw error }
}

@Test(.enabled(if:ProcessInfo.processInfo.environment["FORT_TEST_GUARDIAN"] != nil)) @MainActor
func agentServiceUsesTheGuardedCompiledCoreAndProvesItsOwnedGroupAbsent() async throws {
    _ = try await guardedServiceJourney()
}

@Test(.enabled(if:ProcessInfo.processInfo.environment["FORT_AGENT_SERVICE_BENCHMARK"] != nil)) @MainActor
func agentServiceGuardedStartupAndShutdownBudget() async throws {
    let destination = try #require(ProcessInfo.processInfo.environment["FORT_AGENT_SERVICE_BENCHMARK"])
    var samples: [Double] = []
    for _ in 0..<20 { samples.append(try await guardedServiceJourney()) }
    try JSONEncoder().encode(samples).write(to:URL(fileURLWithPath:destination),options:.withoutOverwriting)
    #expect(samples.sorted()[18] <= 2000)
}
#endif
