#if os(macOS)
import Foundation
import Darwin
import Testing
@testable import FortKit

private func hostDirectory() throws -> URL {
    let directory = FileManager.default.temporaryDirectory.appendingPathComponent("fort-core-host-" + UUID().uuidString)
    try FileManager.default.createDirectory(at:directory,withIntermediateDirectories:false,attributes:[.posixPermissions:0o700])
    return directory
}

@Test func coreHostRecoversOneExactPairingAndRetainsItsReceiptAfterChildExit() async throws {
    let directory = try hostDirectory()
    defer { try? FileManager.default.removeItem(at:directory) }
    let core = try await setupCore(directory,enroll:false)
    do {
        let host = MacCoreHost(core:core,directory:directory)
        guard case .available(let offer) = try await host.beginOwnerSetup(timeout:3) else { Issue.record("Expected fresh offer"); _ = try await core.close(); return }
        let request = try OwnerSetupRequest(installationID:offer.installationID,setupID:offer.setupID,deviceID:"synthetic-device",deviceToken:String(repeating:"A",count:43))
        _ = try await host.completeOwnerSetup(request,timeout:3)
        let client = FortClient(installationID:request.installationID,token:request.deviceToken,transport:host)
        #expect(try await client.installation().installationId == request.installationID)
        let recovery = try MacOwnerRecoveryRequest(installationID:request.installationID,deviceID:request.deviceID,timeout:3)
        let started = ContinuousClock.now
        let receipt = try await host.recoverPairing(recovery)
        let elapsed = started.duration(to:.now)
        try receipt.validate(for:recovery)
        #expect(elapsed < .seconds(2))
        print("CORE_HOST_RECOVERY_DURATION=\(elapsed)")
        #expect(kill(core.processID,0) == -1 && errno == ESRCH)
        #expect(kill(-core.processID,0) == -1 && errno == ESRCH)
        #expect(!FileManager.default.fileExists(atPath:directory.appendingPathComponent("core.sock").path))
        try await host.recoverPairing(recovery).validate(for:recovery)
        await #expect(throws:(any Error).self) { try await host.recoverPairing(MacOwnerRecoveryRequest(installationID:request.installationID,deviceID:"foreign")) }
        await #expect(throws:(any Error).self) { try await client.installation() }
        await #expect(throws:(any Error).self) { try await host.beginOwnerSetup(timeout:3) }
        let stopped = try await host.shutdown()
        #expect(stopped.exitedCleanly && !stopped.forced)
        try await host.recoverPairing(recovery).validate(for:recovery)
    } catch { _ = try? await core.close(); throw error }
}
@Test func coreHostRetainsRevocationProofWhileSocketCleanupIsUnconfirmed() async throws {
    let directory = try hostDirectory()
    defer { try? FileManager.default.removeItem(at:directory) }
    let core = try await setupCore(directory,behavior:"recovery-residue",enroll:false)
    do {
        let host = MacCoreHost(core:core,directory:directory)
        guard case .available(let offer) = try await host.beginOwnerSetup(timeout:3) else { Issue.record("Expected fresh offer"); _ = try await core.close(); return }
        let request = try OwnerSetupRequest(installationID:offer.installationID,setupID:offer.setupID,deviceID:"synthetic-device",deviceToken:String(repeating:"A",count:43))
        _ = try await host.completeOwnerSetup(request,timeout:3)
        let recovery = try MacOwnerRecoveryRequest(installationID:request.installationID,deviceID:request.deviceID,timeout:3)
        await #expect(throws:MacCoreHostError.self) { try await host.recoverPairing(recovery) }
        #expect(kill(core.processID,0) == -1 && errno == ESRCH)
        #expect(try String(contentsOf:directory.appendingPathComponent("core.sock"),encoding:.utf8) == "synthetic residue")
        await #expect(throws:(any Error).self) { try await host.beginOwnerSetup(timeout:3) }
        await #expect(throws:(any Error).self) { try await host.recoverPairing(MacOwnerRecoveryRequest(installationID:request.installationID,deviceID:"foreign")) }
        try FileManager.default.removeItem(at:directory.appendingPathComponent("core.sock"))
        try await host.recoverPairing(recovery).validate(for:recovery)
        #expect(try await host.shutdown().exitedCleanly)
    } catch { _ = try? await core.close(); throw error }
}
private func pairHost(_ host: MacCoreHost) async throws -> (OwnerSetupRequest, MacOwnerRecoveryRequest) {
    guard case .available(let offer) = try await host.beginOwnerSetup(timeout:3) else { throw ClientError.invalidResponse }
    let request = try OwnerSetupRequest(installationID:offer.installationID,setupID:offer.setupID,deviceID:"synthetic-device",deviceToken:String(repeating:"A",count:43))
    _ = try await host.completeOwnerSetup(request,timeout:3)
    return (request,try MacOwnerRecoveryRequest(installationID:request.installationID,deviceID:request.deviceID,timeout:3))
}

@Test func coreHostSharesRecoveryAcrossRetriesAndFinishesDespiteCallerCancellation() async throws {
    let directory = try hostDirectory()
    defer { try? FileManager.default.removeItem(at:directory) }
    let core = try await setupCore(directory,behavior:"recovery-held",enroll:false)
    do {
        let host = MacCoreHost(core:core,directory:directory)
        let (setup,recovery) = try await pairHost(host)
        let first = Task { try await host.recoverPairing(recovery) }
        let marker = directory.appendingPathComponent("recovery-started.json")
        for _ in 0..<200 where !FileManager.default.fileExists(atPath:marker.path) { try await Task.sleep(for:.milliseconds(5)) }
        #expect(try String(contentsOf:marker,encoding:.utf8) == "1")
        first.cancel()
        let second = Task { try await host.recoverPairing(recovery) }
        await #expect(throws:MacCoreHostError.self) { try await host.recoverPairing(MacOwnerRecoveryRequest(installationID:setup.installationID,deviceID:"foreign")) }
        await #expect(throws:MacCoreHostError.self) { try await FortClient(installationID:setup.installationID,token:setup.deviceToken,transport:host).installation() }
        await #expect(throws:MacCoreHostError.self) { try await host.beginOwnerSetup(timeout:3) }
        await #expect(throws:MacCoreHostError.self) { try await host.completeOwnerSetup(setup,timeout:3) }
        let identity = try OwnerSetupIdentity(installationID:setup.installationID,setupID:setup.setupID,deviceID:setup.deviceID)
        await #expect(throws:MacCoreHostError.self) { try await host.cancelOwnerSetup(identity,timeout:3) }
        let shutdown = Task { try await host.shutdown() }
        try Data().write(to:directory.appendingPathComponent("release-recovery"))
        await #expect(throws:CancellationError.self) { try await first.value }
        try await second.value.validate(for:recovery)
        #expect(try await shutdown.value.exitedCleanly)
        #expect(try String(contentsOf:marker,encoding:.utf8) == "1")
        #expect(kill(-core.processID,0) == -1 && errno == ESRCH)
    } catch { try? Data().write(to:directory.appendingPathComponent("release-recovery")); _ = try? await core.close(); throw error }
}

@Test func coreHostRequiresExplicitRecoveryRetryAfterRuntimeStopFailure() async throws {
    let directory = try hostDirectory()
    defer { try? FileManager.default.removeItem(at:directory) }
    let core = try await setupCore(directory,behavior:"recovery-fail-once",enroll:false)
    do {
        let host = MacCoreHost(core:core,directory:directory)
        let (setup,recovery) = try await pairHost(host)
        await #expect(throws:MacCoreHostError.self) { try await host.recoverPairing(recovery) }
        #expect(kill(core.processID,0) == 0)
        await #expect(throws:MacCoreHostError.self) { try await FortClient(installationID:setup.installationID,token:setup.deviceToken,transport:host).installation() }
        await #expect(throws:MacCoreHostError.self) { try await host.beginOwnerSetup(timeout:3) }
        #expect(try String(contentsOf:directory.appendingPathComponent("recovery-started.json"),encoding:.utf8) == "1")
        await #expect(throws:MacCoreHostError.self) { try await host.recoverPairing(MacOwnerRecoveryRequest(installationID:setup.installationID,deviceID:"foreign")) }
        await #expect(throws:MacCoreHostError.self) { try await host.beginOwnerSetup(timeout:3) }
        try await host.recoverPairing(recovery).validate(for:recovery)
        #expect(try String(contentsOf:directory.appendingPathComponent("recovery-started.json"),encoding:.utf8) == "3")
        #expect(kill(-core.processID,0) == -1 && errno == ESRCH)
    } catch { _ = try? await core.close(); throw error }
}

@Test func coreHostNeverTreatsAnUncleanExitAsConfirmedOwnerRecovery() async throws {
    let directory = try hostDirectory()
    defer { try? FileManager.default.removeItem(at:directory) }
    let core = try await setupCore(directory,behavior:"recovery-unclean",enroll:false)
    do {
        let host = MacCoreHost(core:core,directory:directory)
        let (_,recovery) = try await pairHost(host)
        await #expect(throws:MacCoreHostError.self) { try await host.recoverPairing(recovery) }
        await #expect(throws:MacCoreHostError.self) { try await host.recoverPairing(recovery) }
        let stopped = try await host.shutdown()
        #expect(!stopped.exitedCleanly)
        #expect(kill(-core.processID,0) == -1 && errno == ESRCH)
        #expect(!FileManager.default.fileExists(atPath:directory.appendingPathComponent("core.sock").path))
    } catch { _ = try? await core.close(); throw error }
}

@Test func coreHostCancellationBeforeRecoveryDoesNotStopAnAvailablePairing() async throws {
    let directory = try hostDirectory()
    defer { try? FileManager.default.removeItem(at:directory) }
    let core = try await setupCore(directory,enroll:false)
    do {
        let host = MacCoreHost(core:core,directory:directory)
        let (setup,recovery) = try await pairHost(host)
        let canceled = Task {
            withUnsafeCurrentTask { $0?.cancel() }
            return try await host.recoverPairing(recovery)
        }
        await #expect(throws:CancellationError.self) { try await canceled.value }
        #expect(!FileManager.default.fileExists(atPath:directory.appendingPathComponent("recovery-started.json").path))
        #expect(try await FortClient(installationID:setup.installationID,token:setup.deviceToken,transport:host).installation().installationId == setup.installationID)
        try await host.recoverPairing(recovery).validate(for:recovery)
    } catch { _ = try? await core.close(); throw error }
}
@Test(.enabled(if:ProcessInfo.processInfo.environment["FORT_TEST_GUARDIAN"] != nil))
func coreHostLaunchRequiresItsExactGuardianAndOwnsTheChildGroup() async throws {
    let environment = ProcessInfo.processInfo.environment
    let node = URL(fileURLWithPath:try #require(environment["FORT_TEST_NODE"]))
    let guardian = URL(fileURLWithPath:try #require(environment["FORT_TEST_GUARDIAN"]))
    let artifact = URL(fileURLWithPath:try #require(environment["FORT_TEST_CORE_ARTIFACT"]))
    let source = URL(fileURLWithPath:#filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
    let directory = try hostDirectory()
    defer { try? FileManager.default.removeItem(at:directory) }
    let arguments = ["--jitless","--no-experimental-strip-types","--no-warnings",source.appendingPathComponent("host/fixtures/owner-core.mjs").path,directory.path,artifact.appendingPathComponent("service/openai.js").path,artifact.appendingPathComponent("host/private-channel.js").path,"initial","","no-enrollment"]
    let processEnvironment = ["PATH":"/usr/bin:/bin","HOME":directory.path,"TMPDIR":directory.path]
    await #expect(throws:(any Error).self) {
        try await MacCoreHost.launch(executable:node,arguments:arguments,directory:directory,environment:processEnvironment,guardian:directory.appendingPathComponent("missing-guardian"))
    }
    #expect(try FileManager.default.contentsOfDirectory(atPath:directory.path).isEmpty)
    let host = try await MacCoreHost.launch(executable:node,arguments:arguments,directory:directory,environment:processEnvironment,guardian:guardian)
    do {
        let child = try JSONDecoder().decode(Int32.self,from:Data(contentsOf:directory.appendingPathComponent("core-pid.json")))
        let group = getpgid(child)
        #expect(group > 1 && group != child && group != getpgrp())
        let (_,recovery) = try await pairHost(host)
        try await host.recoverPairing(recovery).validate(for:recovery)
        #expect(kill(child,0) == -1 && errno == ESRCH)
        #expect(kill(group,0) == -1 && errno == ESRCH)
        #expect(kill(-group,0) == -1 && errno == ESRCH)
        #expect(try await host.shutdown().exitedCleanly)
    } catch { _ = try? await host.shutdown(); throw error }
}

@Test func coreHostShutdownIsIdempotentAndCannotClaimPairingRevocation() async throws {
    let directory = try hostDirectory()
    defer { try? FileManager.default.removeItem(at:directory) }
    let core = try await setupCore(directory,enroll:false)
    do {
        let host = MacCoreHost(core:core,directory:directory)
        let (setup,recovery) = try await pairHost(host)
        async let first = host.shutdown()
        async let second = host.shutdown()
        #expect(try await first.exitedCleanly)
        #expect(try await second.exitedCleanly)
        await #expect(throws:MacCoreHostError.self) { try await host.recoverPairing(recovery) }
        await #expect(throws:MacCoreHostError.self) { try await host.beginOwnerSetup(timeout:3) }
        let reopened = try await setupCore(directory,enroll:false)
        do {
            #expect(try await FortClient(installationID:setup.installationID,token:setup.deviceToken,transport:reopened).installation().installationId == setup.installationID)
            try await stopSetupCore(reopened,directory:directory)
        } catch { _ = try? await reopened.close(); throw error }
    } catch { _ = try? await core.close(); throw error }
}
@Test func coreHostRejectsForeignRecoveryWithoutDisruptingItsCurrentPairing() async throws {
    let directory = try hostDirectory()
    defer { try? FileManager.default.removeItem(at:directory) }
    let core = try await setupCore(directory,enroll:false)
    do {
        let host = MacCoreHost(core:core,directory:directory)
        let (setup,recovery) = try await pairHost(host)
        let client = FortClient(installationID:setup.installationID,token:setup.deviceToken,transport:host)
        for foreign in [try MacOwnerRecoveryRequest(installationID:"foreign",deviceID:setup.deviceID),try MacOwnerRecoveryRequest(installationID:setup.installationID,deviceID:"foreign")] {
            await #expect(throws:(any Error).self) { try await host.recoverPairing(foreign) }
            #expect(try await client.installation().installationId == setup.installationID)
        }
        try await host.recoverPairing(recovery).validate(for:recovery)
    } catch { _ = try? await core.close(); throw error }
}
@Test(arguments:["installation","device","outcome","extra","large","status"])
func coreHostMalformedRejectionCannotRestoreOrdinaryTraffic(failure: String) async throws {
    let environment = ProcessInfo.processInfo.environment
    let executable = environment["FORT_TEST_NODE"] ?? (environment["PATH"] ?? "").split(separator:":").map { String($0) + "/node" }.first { FileManager.default.isExecutableFile(atPath:$0) }
    let node = URL(fileURLWithPath:try #require(executable))
    let source = URL(fileURLWithPath:#filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
    let directory = try hostDirectory()
    defer { try? FileManager.default.removeItem(at:directory) }
    let core = try await MacCoreProcessTransport.launch(executable:node,arguments:["--jitless","--no-warnings",source.appendingPathComponent("host/fixtures/native-channel.mjs").path,"recovery-rejection-" + failure],directory:directory,environment:["PATH":"/usr/bin:/bin"],lifecycle:.processGroup)
    do {
        let host = MacCoreHost(core:core,directory:directory)
        await #expect(throws:MacCoreHostError.self) { try await host.recoverPairing(MacOwnerRecoveryRequest(installationID:"expected",deviceID:"expected")) }
        await #expect(throws:MacCoreHostError.self) { try await host.send(FortRequest(path:"/v1/installation",method:.get,installationID:"expected",token:nil,body:nil,timeout:3)) }
        #expect(try await host.shutdown().exitedCleanly)
    } catch { _ = try? await core.close(); throw error }
}
#endif
