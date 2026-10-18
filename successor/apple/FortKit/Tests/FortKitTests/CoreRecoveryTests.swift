#if os(macOS)
import Foundation
import Darwin
import Testing
@testable import FortKit

private let recoverySource = URL(fileURLWithPath:#filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
private struct RecoverySetup: Decodable {
    struct Identity: Decodable { let installationId: String; let recoveryEpoch: String }
    struct Enrollment: Decodable { let code: String }
    let installation: Identity
    let enrollment: Enrollment?
}
private func recoveryNodeExecutable() throws -> URL {
    let environment = ProcessInfo.processInfo.environment
    let node = environment["FORT_TEST_NODE"] ?? (environment["PATH"] ?? "").split(separator:":").map { String($0) + "/node" }.first { FileManager.default.isExecutableFile(atPath:$0) }
    return URL(fileURLWithPath:try #require(node))
}
private func ownerCore(_ directory: URL, mode: String) async throws -> MacCoreProcessTransport {
    let environment = ProcessInfo.processInfo.environment
    let executable = try recoveryNodeExecutable()
    let artifact = environment["FORT_TEST_CORE_ARTIFACT"].map { URL(fileURLWithPath:$0) }
    let core = artifact?.appendingPathComponent("service/openai.js") ?? recoverySource.appendingPathComponent("service/openai.ts")
    let channel = artifact?.appendingPathComponent("host/private-channel.js") ?? recoverySource.appendingPathComponent("host/private-channel.ts")
    let flags = artifact == nil ? ["--no-warnings"] : ["--jitless","--no-experimental-strip-types","--no-warnings"]
    return try await MacCoreProcessTransport.launch(executable:executable,arguments:flags + [recoverySource.appendingPathComponent("host/fixtures/owner-core.mjs").path,directory.path,core.path,channel.path,mode],directory:directory,environment:["PATH":"/usr/bin:/bin","HOME":directory.path,"TMPDIR":directory.path],lifecycle:.processGroup)
}
private func recoverySetup(_ directory: URL) throws -> RecoverySetup {
    try JSONDecoder().decode(RecoverySetup.self,from:Data(contentsOf:directory.appendingPathComponent("setup.json")))
}

@Test(arguments:["installation","device","runtime","revocation","status"])
func nativeOwnerRecoveryRejectsIncompleteOrForeignProof(failure: String) async throws {
    let directory = FileManager.default.temporaryDirectory.appendingPathComponent("fort-native-proof-" + UUID().uuidString)
    try FileManager.default.createDirectory(at:directory,withIntermediateDirectories:false,attributes:[.posixPermissions:0o700])
    defer { try? FileManager.default.removeItem(at:directory) }
    let process = try await MacCoreProcessTransport.launch(executable:try recoveryNodeExecutable(),arguments:["--jitless","--no-warnings",recoverySource.appendingPathComponent("host/fixtures/native-channel.mjs").path,"recovery-proof-" + failure],directory:directory,environment:["PATH":"/usr/bin:/bin"],lifecycle:.processGroup)
    do {
        await #expect(throws:ClientError.self) { try await process.recoverOwnerPairing(installationID:"expected",deviceID:"expected") }
        try await stoppedOwner(process,directory:directory)
    } catch { _ = try? await process.close(); throw error }
}
private func stoppedOwner(_ process: MacCoreProcessTransport, directory: URL) async throws {
    let stopped = try await process.close()
    #expect(stopped.exitedCleanly && !stopped.forced)
    #expect(kill(process.processID,0) == -1 && errno == ESRCH)
    #expect(kill(-process.processID,0) == -1 && errno == ESRCH)
    #expect(!FileManager.default.fileExists(atPath:directory.appendingPathComponent("core.sock").path))
}

@Test func ownedCoreRecoversExpiredPairingAndConfirmsProcessExitWithoutLosingHistory() async throws {
    let directory = FileManager.default.temporaryDirectory.appendingPathComponent("fort-native-owner-" + UUID().uuidString)
    try FileManager.default.createDirectory(at:directory,withIntermediateDirectories:false,attributes:[.posixPermissions:0o700])
    defer { try? FileManager.default.removeItem(at:directory) }
    let first = try await ownerCore(directory,mode:"initial")
    do {
        let setup = try recoverySetup(directory), installationID = setup.installation.installationId
        let enrolled = try await FortClient(installationID:installationID,transport:first).enroll(code:try #require(setup.enrollment?.code),deviceName:"Synthetic owner")
        let client = FortClient(installationID:installationID,token:enrolled.deviceToken,transport:first)
        let agent = try await client.saveAgent(nil,name:"Retained native Home",model:"exact",instructions:"Synthetic echo.")
        let text = "  Retained cedar 🌲 Café e\u{301}\r\n"
        _ = try await client.submit(PendingSubmission(requestID:"owner-recovery-turn",conversationID:agent.homeConversationId,bindingRevision:agent.binding.bindingRevisionId,recoveryEpoch:setup.installation.recoveryEpoch,text:text,createdAt:Date(),recipientAgentID:agent.agentId,skills:[]))
        var snapshot = try await client.snapshot(conversationID:agent.homeConversationId)
        for _ in 0..<50 where snapshot.messages.count != 2 {
            try await Task.sleep(for:.milliseconds(10)); snapshot = try await client.snapshot(conversationID:agent.homeConversationId)
        }
        #expect(snapshot.messages.map(\.text) == [text,text])
        try await stoppedOwner(first,directory:directory)
        let expired = try await ownerCore(directory,mode:"expired")
        do {
            let stale = FortClient(installationID:installationID,token:enrolled.deviceToken,transport:expired)
            await #expect(throws:ClientError.self) { try await stale.installation() }
            await #expect(throws:(any Error).self) { try await expired.recoverOwnerPairing(installationID:"foreign",deviceID:enrolled.deviceId) }
            await #expect(throws:(any Error).self) { try await expired.recoverOwnerPairing(installationID:installationID,deviceID:"foreign") }
            let proof = try await expired.recoverOwnerPairing(installationID:installationID,deviceID:enrolled.deviceId)
            #expect(proof.installationID == installationID && proof.deviceID == enrolled.deviceId)
            #expect(proof.runtimeStopped && proof.pairingRevoked)
            try await stoppedOwner(expired,directory:directory)
        } catch { _ = try? await expired.close(); throw error }
        let resumed = try await ownerCore(directory,mode:"resume")
        do {
            let setup = try recoverySetup(directory)
            let fresh = try await FortClient(installationID:installationID,transport:resumed).enroll(code:try #require(setup.enrollment?.code),deviceName:"Fresh synthetic owner")
            #expect(fresh.deviceId != enrolled.deviceId)
            let client = FortClient(installationID:installationID,token:fresh.deviceToken,transport:resumed)
            #expect(try await client.snapshot(conversationID:agent.homeConversationId).messages.map(\.text) == [text,text])
            await #expect(throws:(any Error).self) { try await resumed.recoverOwnerPairing(installationID:installationID,deviceID:enrolled.deviceId) }
            #expect(try await client.installation().installationId == installationID)
            try await stoppedOwner(resumed,directory:directory)
        } catch { _ = try? await resumed.close(); throw error }
    } catch { _ = try? await first.close(); throw error }
}
#endif
