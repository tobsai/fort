#if os(macOS)
import Foundation
import Darwin
import Testing
@testable import FortKit

@Test func historyDeletionRequestAndReceiptRequireExactScope() throws {
    let request = try MacHistoryDeletionRequest(installationID:"installation",deviceID:"device",recoveryEpoch:"old",deletionID:"confirmed")
    #expect(try JSONDecoder().decode(MacHistoryDeletionRequest.self,from:JSONEncoder().encode(request)) == request)
    for identifier in ["", "foreign/path", String(repeating:"x",count:129), "é", "a\n"] {
        #expect(throws:ClientError.self) { try MacHistoryDeletionRequest(installationID:identifier,deviceID:"device",recoveryEpoch:"old",deletionID:"confirmed") }
    }
    for timeout in [Double.nan,Double.infinity,0,-1,30.01] {
        #expect(throws:ClientError.self) { try MacHistoryDeletionRequest(installationID:"installation",deviceID:"device",recoveryEpoch:"old",deletionID:"confirmed",timeout:timeout) }
    }
    let valid: [String:Any] = ["installationId":"installation","deviceId":"device","deletionId":"confirmed","previousRecoveryEpoch":"old","recoveryEpoch":"new","deletedRecords":4]
    let decoder = JSONDecoder()
    try decoder.decode(MacHistoryDeletionReceipt.self,from:JSONSerialization.data(withJSONObject:valid)).validate(for:request)
    for key in ["installationId","deviceId","deletionId","previousRecoveryEpoch"] {
        var changed = valid; changed[key] = "foreign"
        #expect(throws:ClientError.self) { try decoder.decode(MacHistoryDeletionReceipt.self,from:JSONSerialization.data(withJSONObject:changed)).validate(for:request) }
    }
    for patch: [String:Any] in [["recoveryEpoch":"old"],["recoveryEpoch":"invalid/path"],["deletedRecords":-1]] {
        let changed = valid.merging(patch) { _, new in new }
        #expect(throws:ClientError.self) { try decoder.decode(MacHistoryDeletionReceipt.self,from:JSONSerialization.data(withJSONObject:changed)).validate(for:request) }
    }
}

@Test(.enabled(if:ProcessInfo.processInfo.environment["FORT_TEST_HISTORY_RESOURCES"] != nil))
func nativeHistoryMaintenanceDeletesOnlyAfterCoreExitAndReopensTheSameInstallation() async throws {
    let environment = ProcessInfo.processInfo.environment
    let resources = URL(fileURLWithPath:try #require(environment["FORT_TEST_HISTORY_RESOURCES"]))
    let node = URL(fileURLWithPath:try #require(environment["FORT_TEST_NODE"]))
    let guardian = URL(fileURLWithPath:try #require(environment["FORT_TEST_GUARDIAN"]))
    let source = URL(fileURLWithPath:#filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
    let root = FileManager.default.temporaryDirectory.appendingPathComponent("fort-history-maintenance-" + UUID().uuidString)
    try FileManager.default.createDirectory(at:root,withIntermediateDirectories:false,attributes:[.posixPermissions:0o700])
    defer { try? FileManager.default.removeItem(at:root) }
    let directory = root.resolvingSymlinksInPath()
    let flags = ["--jitless","--no-experimental-strip-types","--no-warnings","--openssl-config=" + resources.appendingPathComponent("node-openssl.cnf").path]
    func launchCore() async throws -> MacCoreHost {
        try await MacCoreHost.launch(executable:node,arguments:flags + [source.appendingPathComponent("host/fixtures/owned-core.mjs").path,resources.appendingPathComponent("core/host/owned-core.js").path,directory.path],directory:directory,environment:[:],guardian:guardian)
    }
    let host = try await launchCore()
    let installation: Installation, agent: Agent, command: PendingSubmission
    do {
        guard case .available(let offer) = try await host.beginOwnerSetup(timeout:3) else { throw ClientError.invalidResponse }
        let setup = try OwnerSetupRequest(installationID:offer.installationID,setupID:offer.setupID,deviceID:"device",deviceToken:String(repeating:"A",count:43))
        _ = try await host.completeOwnerSetup(setup,timeout:3)
        let client = FortClient(installationID:setup.installationID,token:setup.deviceToken,transport:host)
        installation = try await client.installation()
        agent = try await client.saveAgent(nil,name:"Keep settings",model:"exact",instructions:"Synthetic only")
        command = PendingSubmission(requestID:"old-pending",conversationID:agent.homeConversationId,bindingRevision:agent.binding.bindingRevisionId,recoveryEpoch:installation.recoveryEpoch,text:"Synthetic history",createdAt:Date(),recipientAgentID:agent.agentId,skills:[])
        _ = try await client.submit(command)
        #expect(try await client.snapshot(conversationID:agent.homeConversationId).messages.count > 0)
        _ = try await host.recoverPairing(MacOwnerRecoveryRequest(installationID:installation.installationId,deviceID:setup.deviceID))
        #expect(try await host.shutdown().exitedCleanly)
    } catch { _ = try? await host.shutdown(); throw error }
    let request = try MacHistoryDeletionRequest(installationID:installation.installationId,deviceID:"device",recoveryEpoch:installation.recoveryEpoch,deletionID:"confirmed")
    func erase() async throws -> MacHistoryDeletionReceipt {
        let worker = try await MacCoreProcessTransport.launch(executable:node,arguments:flags + [resources.appendingPathComponent("history-maintenance.mjs").path],directory:directory,environment:[:],lifecycle:.parentExit(guardian:guardian))
        do {
            let result = try await worker.deleteHistory(request)
            let stopped = try await worker.close()
            #expect(stopped.exitedCleanly && !stopped.forced)
            #expect(kill(worker.processID,0) == -1 && errno == ESRCH)
            #expect(kill(-worker.processID,0) == -1 && errno == ESRCH)
            return result
        } catch { _ = try? await worker.close(); throw error }
    }
    let start = ContinuousClock.now
    let receipt = try await erase()
    let elapsed = start.duration(to:.now).components
    let milliseconds = Double(elapsed.seconds) * 1000 + Double(elapsed.attoseconds) / 1e15
    print("Synthetic native history deletion: \(milliseconds) ms; guarded group absent.")
    #expect(milliseconds <= 5000)
    #expect(try await erase() == receipt)
    let reopened = try await launchCore()
    do {
        guard case .available(let offer) = try await reopened.beginOwnerSetup(timeout:3) else { throw ClientError.invalidResponse }
        let setup = try OwnerSetupRequest(installationID:offer.installationID,setupID:offer.setupID,deviceID:"replacement",deviceToken:String(repeating:"A",count:43))
        _ = try await reopened.completeOwnerSetup(setup,timeout:3)
        let client = FortClient(installationID:setup.installationID,token:setup.deviceToken,transport:reopened)
        #expect(try await client.installation().installationId == installation.installationId)
        #expect(try await client.installation().recoveryEpoch == receipt.recoveryEpoch)
        #expect(try await client.agents() == [agent])
        #expect(try await client.snapshot(conversationID:agent.homeConversationId).messages.isEmpty)
        await #expect(throws:(any Error).self) { try await client.submit(command) }
        #expect(try await reopened.shutdown().exitedCleanly)
    } catch { _ = try? await reopened.shutdown(); throw error }
}
#endif
