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
private func setupCore(_ directory: URL, behavior: String = "") async throws -> MacCoreProcessTransport {
    let artifact = ProcessInfo.processInfo.environment["FORT_TEST_CORE_ARTIFACT"].map { URL(fileURLWithPath:$0) }
    let core = artifact?.appendingPathComponent("service/openai.js") ?? setupSource.appendingPathComponent("service/openai.ts")
    let channel = artifact?.appendingPathComponent("host/private-channel.js") ?? setupSource.appendingPathComponent("host/private-channel.ts")
    let flags = artifact == nil ? ["--no-warnings"] : ["--jitless","--no-experimental-strip-types","--no-warnings"]
    return try await MacCoreProcessTransport.launch(executable:try setupNode(),arguments:flags + [setupSource.appendingPathComponent("host/fixtures/owner-core.mjs").path,directory.path,core.path,channel.path,"initial",behavior],directory:directory,environment:["PATH":"/usr/bin:/bin","HOME":directory.path,"TMPDIR":directory.path],lifecycle:.processGroup)
}
private func stopSetupCore(_ process: MacCoreProcessTransport, directory: URL) async throws {
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
    let process = try await setupCore(directory,behavior:"slow")
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
            await #expect(throws:(any Error).self) { try await process.completeOwnerSetup(request,timeout:0.01) }
            #expect(FileManager.default.fileExists(atPath:directory.appendingPathComponent("setup-committed.json").path))
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
        for timeout in [0,-1,31,Double.infinity,Double.nan] {
            await #expect(throws:ClientError.self) { try await process.beginOwnerSetup(timeout:timeout) }
            await #expect(throws:ClientError.self) { try await process.completeOwnerSetup(request,timeout:timeout) }
        }
        guard case .available = try await process.beginOwnerSetup() else { Issue.record("Invalid deadlines must leave setup untouched"); _ = try await process.close(); return }
        try await stopSetupCore(process,directory:directory)
    } catch { _ = try? await process.close(); throw error }
}
#endif
