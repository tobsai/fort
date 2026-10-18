#if os(macOS)
import Foundation
import Darwin
import Testing
@testable import FortKit

private let setupSource = URL(fileURLWithPath:#filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
private func setupNode() throws -> URL {
    let environment = ProcessInfo.processInfo.environment
    let node = environment["FORT_TEST_NODE"] ?? (environment["PATH"] ?? "").split(separator:":").map { String($0) + "/node" }.first { FileManager.default.isExecutableFile(atPath:$0) }
    return URL(fileURLWithPath:try #require(node))
}
private func setupDirectory() throws -> URL {
    let directory = FileManager.default.temporaryDirectory.appendingPathComponent("fort-native-setup-" + UUID().uuidString)
    try FileManager.default.createDirectory(at:directory,withIntermediateDirectories:false,attributes:[.posixPermissions:0o700])
    return directory
}
func setupCore(_ directory: URL, behavior: String = "", enroll: Bool = true) async throws -> MacCoreProcessTransport {
    let artifact = ProcessInfo.processInfo.environment["FORT_TEST_CORE_ARTIFACT"].map { URL(fileURLWithPath:$0) }
    let core = artifact?.appendingPathComponent("service/openai.js") ?? setupSource.appendingPathComponent("service/openai.ts")
    let channel = artifact?.appendingPathComponent("host/private-channel.js") ?? setupSource.appendingPathComponent("host/private-channel.ts")
    let flags = artifact == nil ? ["--no-warnings"] : ["--jitless","--no-experimental-strip-types","--no-warnings"]
    return try await MacCoreProcessTransport.launch(executable:try setupNode(),arguments:flags + [setupSource.appendingPathComponent("host/fixtures/owner-core.mjs").path,directory.path,core.path,channel.path,"initial",behavior,enroll ? "enroll" : "no-enrollment"],directory:directory,environment:["PATH":"/usr/bin:/bin","HOME":directory.path,"TMPDIR":directory.path],lifecycle:.processGroup)
}

@Test(arguments:[false,true])
func nativeOwnerCancellationConfirmsTheExactSetupAcrossCoreRestart(completed: Bool) async throws {
    let directory = try setupDirectory()
    defer { try? FileManager.default.removeItem(at:directory) }
    let first = try await setupCore(directory,enroll:false)
    do {
        guard case .available(let offer) = try await first.beginOwnerSetup() else { Issue.record("Expected fresh setup offer"); _ = try await first.close(); return }
        let identity = try OwnerSetupIdentity(installationID:offer.installationID,setupID:offer.setupID,deviceID:UUID().uuidString)
        let request = try OwnerSetupRequest(installationID:identity.installationID,setupID:identity.setupID,deviceID:identity.deviceID,deviceToken:String(repeating:"A",count:43))
        if completed { _ = try await first.completeOwnerSetup(request) }
        let proof: OwnerSetupCancellation = completed ? .revoked(identity) : .notPaired(identity)
        #expect(try await first.cancelOwnerSetup(identity) == proof)
        await #expect(throws:(any Error).self) { try await first.completeOwnerSetup(request) }
        try await stopSetupCore(first,directory:directory)
        let second = try await setupCore(directory,enroll:false)
        do {
            #expect(try await second.cancelOwnerSetup(identity) == proof)
            await #expect(throws:(any Error).self) { try await FortClient(installationID:identity.installationID,token:request.deviceToken,transport:second).installation() }
            if completed {
                await #expect(throws:(any Error).self) { try await second.beginOwnerSetup() }
            } else {
                guard case .available(let fresh) = try await second.beginOwnerSetup() else { Issue.record("Expected explicit replacement offer"); _ = try await second.close(); return }
                #expect(fresh.setupID != identity.setupID)
            }
            try await stopSetupCore(second,directory:directory)
        } catch { _ = try? await second.close(); throw error }
    } catch { _ = try? await first.close(); throw error }
}
func stopSetupCore(_ process: MacCoreProcessTransport, directory: URL) async throws {
    let stopped = try await process.close()
    #expect(stopped.exitedCleanly && !stopped.forced)
    #expect(kill(process.processID,0) == -1 && errno == ESRCH)
    #expect(kill(-process.processID,0) == -1 && errno == ESRCH)
    #expect(!FileManager.default.fileExists(atPath:directory.appendingPathComponent("core.sock").path))
}

@Test func nativePreparedOwnerSetupConfirmsOneCredentialAcrossCoreRestart() async throws {
    let directory = try setupDirectory()
    defer { try? FileManager.default.removeItem(at:directory) }
    let first = try await setupCore(directory)
    do {
        let state = try await first.beginOwnerSetup()
        guard case .available(let offer) = state else { Issue.record("Expected fresh setup offer"); _ = try await first.close(); return }
        #expect(try await first.beginOwnerSetup() == state)
        let request = try OwnerSetupRequest(installationID:offer.installationID,setupID:offer.setupID,deviceID:UUID().uuidString,deviceToken:String(repeating:"A",count:43))
        let receipt = try await first.completeOwnerSetup(request)
        #expect(receipt.installationID == request.installationID && receipt.deviceID == request.deviceID)
        #expect(receipt.expiresAt == "2026-10-19T00:00:00.000Z")
        #expect(try await first.beginOwnerSetup() == .paired(receipt))
        #expect(try await FortClient(installationID:request.installationID,token:request.deviceToken,transport:first).installation().installationId == request.installationID)
        try await stopSetupCore(first,directory:directory)
        let second = try await setupCore(directory)
        do {
            #expect(try await second.completeOwnerSetup(request) == receipt)
            #expect(try await second.beginOwnerSetup() == .paired(receipt))
            #expect(try await FortClient(installationID:request.installationID,token:request.deviceToken,transport:second).installation().installationId == request.installationID)
            try await stopSetupCore(second,directory:directory)
        } catch { _ = try? await second.close(); throw error }
    } catch { _ = try? await first.close(); throw error }
}

@Test(arguments:[false,true])
func nativeOwnerSetupRetriesAfterDeadlineOrCancellationFollowingCommit(cancel: Bool) async throws {
    let directory = try setupDirectory()
    defer { try? FileManager.default.removeItem(at:directory) }
    let process = try await setupCore(directory,behavior:cancel ? "slow" : "deadline-held")
    do {
        guard case .available(let offer) = try await process.beginOwnerSetup() else { Issue.record("Expected setup offer"); _ = try await process.close(); return }
        let request = try OwnerSetupRequest(installationID:offer.installationID,setupID:offer.setupID,deviceID:UUID().uuidString,deviceToken:String(repeating:"A",count:43))
        if cancel {
            let work = Task { try await process.completeOwnerSetup(request,timeout:3) }
            let committed = directory.appendingPathComponent("setup-committed.json")
            for _ in 0..<100 where !FileManager.default.fileExists(atPath:committed.path) { try await Task.sleep(for:.milliseconds(5)) }
            #expect(FileManager.default.fileExists(atPath:committed.path))
            work.cancel()
            await #expect(throws:CancellationError.self) { try await work.value }
        } else {
            let release = directory.appendingPathComponent("release-setup-response")
            defer { try? Data().write(to:release) }
            let earliestDeadline = ContinuousClock.now.advanced(by:.seconds(1))
            let work = Task { try await process.completeOwnerSetup(request,timeout:1) }
            let committed = directory.appendingPathComponent("setup-committed.json")
            for _ in 0..<100 where !FileManager.default.fileExists(atPath:committed.path) { try await Task.sleep(for:.milliseconds(5)) }
            try #require(FileManager.default.fileExists(atPath:committed.path))
            try #require(ContinuousClock.now < earliestDeadline)
            await #expect(throws:(any Error).self) { try await work.value }
        }
        let receipt = try await process.completeOwnerSetup(request,timeout:3)
        #expect(receipt.installationID == request.installationID && receipt.deviceID == request.deviceID)
        #expect(try await process.beginOwnerSetup() == .paired(receipt))
        #expect(try await FortClient(installationID:request.installationID,token:request.deviceToken,transport:process).installation().installationId == request.installationID)
        try await stopSetupCore(process,directory:directory)
    } catch { _ = try? await process.close(); throw error }
}

@Test(arguments:["setup-proof-installation","setup-proof-device","setup-proof-expiry","setup-proof-missing-expiry","setup-proof-status","setup-offer-kind","setup-offer-installation","setup-offer-id","setup-offer-expiry"])
func nativeOwnerSetupRejectsForeignOrMalformedProof(mode: String) async throws {
    let directory = try setupDirectory()
    defer { try? FileManager.default.removeItem(at:directory) }
    let process = try await MacCoreProcessTransport.launch(executable:try setupNode(),arguments:["--jitless","--no-warnings",setupSource.appendingPathComponent("host/fixtures/native-channel.mjs").path,mode],directory:directory,environment:["PATH":"/usr/bin:/bin"],lifecycle:.processGroup)
    do {
        if mode.hasPrefix("setup-offer-") {
            await #expect(throws:ClientError.self) { try await process.beginOwnerSetup() }
        } else {
            let request = try OwnerSetupRequest(installationID:"installation",setupID:"setup",deviceID:"device",deviceToken:String(repeating:"A",count:43))
            await #expect(throws:ClientError.self) { try await process.completeOwnerSetup(request) }
        }
        try await stopSetupCore(process,directory:directory)
    } catch { _ = try? await process.close(); throw error }
}

@Test(arguments:[false,true],[false,true])
func nativeOwnerCancellationRetriesAfterDeadlineOrTaskCancellation(completed: Bool, canceledReply: Bool) async throws {
    let directory = try setupDirectory()
    defer { try? FileManager.default.removeItem(at:directory) }
    let process = try await setupCore(directory,behavior:"cancel-slow",enroll:false)
    do {
        guard case .available(let offer) = try await process.beginOwnerSetup() else { Issue.record("Expected setup offer"); _ = try await process.close(); return }
        let identity = try OwnerSetupIdentity(installationID:offer.installationID,setupID:offer.setupID,deviceID:UUID().uuidString)
        let request = try OwnerSetupRequest(installationID:identity.installationID,setupID:identity.setupID,deviceID:identity.deviceID,deviceToken:String(repeating:"A",count:43))
        if completed { _ = try await process.completeOwnerSetup(request) }
        if canceledReply {
            let work = Task { try await process.cancelOwnerSetup(identity,timeout:3) }
            let committed = directory.appendingPathComponent("cancellation-committed.json")
            for _ in 0..<100 where !FileManager.default.fileExists(atPath:committed.path) { try await Task.sleep(for:.milliseconds(5)) }
            #expect(FileManager.default.fileExists(atPath:committed.path))
            work.cancel()
            await #expect(throws:CancellationError.self) { try await work.value }
        } else {
            await #expect(throws:(any Error).self) { try await process.cancelOwnerSetup(identity,timeout:0.01) }
        }
        let proof: OwnerSetupCancellation = completed ? .revoked(identity) : .notPaired(identity)
        #expect(try await process.cancelOwnerSetup(identity,timeout:3) == proof)
        await #expect(throws:(any Error).self) { try await process.completeOwnerSetup(request) }
        try await stopSetupCore(process,directory:directory)
    } catch { _ = try? await process.close(); throw error }
}

@Test(arguments:["installation","setup","device","outcome","runtime","missing-runtime","status","not-paired-stop"])
func nativeOwnerCancellationRejectsForeignOrUnconfirmedProof(failure: String) async throws {
    let directory = try setupDirectory()
    defer { try? FileManager.default.removeItem(at:directory) }
    let process = try await MacCoreProcessTransport.launch(executable:try setupNode(),arguments:["--jitless","--no-warnings",setupSource.appendingPathComponent("host/fixtures/native-channel.mjs").path,"setup-cancel-proof-" + failure],directory:directory,environment:["PATH":"/usr/bin:/bin"],lifecycle:.processGroup)
    do {
        let identity = try OwnerSetupIdentity(installationID:"installation",setupID:"setup",deviceID:"device")
        await #expect(throws:ClientError.self) { try await process.cancelOwnerSetup(identity) }
        try await stopSetupCore(process,directory:directory)
    } catch { _ = try? await process.close(); throw error }
}

@Test func nativeOwnerCancellationValuesPreserveDistinctProofAndValidateIdentity() throws {
    let identity = try OwnerSetupIdentity(installationID:"installation",setupID:"setup",deviceID:"device")
    for proof in [OwnerSetupCancellation.notPaired(identity),.revoked(identity)] {
        let encoded = try JSONEncoder().encode(proof)
        #expect(try JSONDecoder().decode(OwnerSetupCancellation.self,from:encoded) == proof)
        let fields = try #require(JSONSerialization.jsonObject(with:encoded) as? [String:Any])
        #expect(fields["deviceToken"] == nil)
        #expect(proof.identity == identity)
        if case .notPaired = proof { #expect(fields["runtimeStopped"] == nil) }
        else { #expect(fields["runtimeStopped"] as? Bool == true) }
    }
    for invalid in ["","wrong/path",String(repeating:"a",count:129),"é"] {
        #expect(throws:ClientError.self) { try OwnerSetupIdentity(installationID:invalid,setupID:"setup",deviceID:"device") }
        #expect(throws:ClientError.self) { try OwnerSetupIdentity(installationID:"installation",setupID:invalid,deviceID:"device") }
        #expect(throws:ClientError.self) { try OwnerSetupIdentity(installationID:"installation",setupID:"setup",deviceID:invalid) }
    }
    for changes in [["outcome":"revoked"],["outcome":"revoked","runtimeStopped":false],["outcome":"revoked","runtimeStopped":"true"],["outcome":"unknown"],["outcome":"notPaired","runtimeStopped":true],["outcome":"notPaired","setupId":"wrong/path"]] as [[String:Any]] {
        let fields: [String:Any] = ["installationId":"installation","setupId":"setup","deviceId":"device"]
        let bytes = try JSONSerialization.data(withJSONObject:fields.merging(changes) { _,new in new })
        #expect(throws:(any Error).self) { try JSONDecoder().decode(OwnerSetupCancellation.self,from:bytes) }
    }
}

@Test func nativeOwnerSetupValuesValidateWireIdentityTokenAndExpiry() throws {
    let validToken = String(repeating:"A",count:43)
    for token in ["x",String(repeating:"+",count:43),String(repeating:"A",count:42) + "B",String(repeating:"é",count:43)] {
        #expect(throws:ClientError.self) { try OwnerSetupRequest(installationID:"installation",setupID:"setup",deviceID:"device",deviceToken:token) }
        let bytes = try JSONSerialization.data(withJSONObject:["installationId":"installation","setupId":"setup","deviceId":"device","deviceToken":token])
        #expect(throws:ClientError.self) { try JSONDecoder().decode(OwnerSetupRequest.self,from:bytes) }
    }
    for identity in ["","wrong/path",String(repeating:"a",count:129)] {
        #expect(throws:ClientError.self) { try OwnerSetupRequest(installationID:identity,setupID:"setup",deviceID:"device",deviceToken:validToken) }
        #expect(throws:ClientError.self) { try OwnerSetupOffer(installationID:"installation",setupID:identity,expiresAt:"2026-10-18T00:00:00.000Z") }
        #expect(throws:ClientError.self) { try OwnerSetupReceipt(installationID:"installation",deviceID:identity,expiresAt:nil) }
    }
    for expiry in ["invalid","2026-10-18T00:00:00Z","2026-02-30T00:00:00.000Z","2026-10-18T00:00:00.000+00:00"] {
        #expect(throws:ClientError.self) { try OwnerSetupOffer(installationID:"installation",setupID:"setup",expiresAt:expiry) }
        #expect(throws:ClientError.self) { try OwnerSetupReceipt(installationID:"installation",deviceID:"device",expiresAt:expiry) }
    }
    let request = try OwnerSetupRequest(installationID:"installation",setupID:"setup",deviceID:"device",deviceToken:validToken)
    #expect(try JSONDecoder().decode(OwnerSetupRequest.self,from:JSONEncoder().encode(request)) == request)
    for state in [OwnerSetupState.available(try OwnerSetupOffer(installationID:"installation",setupID:"setup",expiresAt:"2026-10-18T00:00:00.000Z")), .paired(try OwnerSetupReceipt(installationID:"installation",deviceID:"device",expiresAt:nil))] {
        #expect(try JSONDecoder().decode(OwnerSetupState.self,from:JSONEncoder().encode(state)) == state)
    }
}

@Test func nativeOwnerSetupRejectsInvalidDeadlinesBeforeSending() async throws {
    let directory = try setupDirectory()
    defer { try? FileManager.default.removeItem(at:directory) }
    let process = try await setupCore(directory)
    do {
        let request = try OwnerSetupRequest(installationID:"installation",setupID:"setup",deviceID:"device",deviceToken:String(repeating:"A",count:43))
        let identity = try OwnerSetupIdentity(installationID:"installation",setupID:"setup",deviceID:"device")
        for timeout in [0,-1,31,Double.infinity,Double.nan] {
            await #expect(throws:ClientError.self) { try await process.beginOwnerSetup(timeout:timeout) }
            await #expect(throws:ClientError.self) { try await process.completeOwnerSetup(request,timeout:timeout) }
            await #expect(throws:ClientError.self) { try await process.cancelOwnerSetup(identity,timeout:timeout) }
        }
        guard case .available = try await process.beginOwnerSetup() else { Issue.record("Invalid deadlines must leave setup untouched"); _ = try await process.close(); return }
        try await stopSetupCore(process,directory:directory)
    } catch { _ = try? await process.close(); throw error }
}
#endif
