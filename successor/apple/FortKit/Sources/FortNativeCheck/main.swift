import Foundation
import FortKit

struct Configuration: Codable { let baseURL: URL; let installationId: String; let codeA: String; let codeB: String; let stateDirectory: URL }
struct CheckState: Codable { let tokenA: String; let tokenB: String; let pending: PendingSubmission; let expectedSnapshot: Snapshot }
struct CheckFailure: Error { let reason: String }
func require(_ condition: Bool, _ message: String) throws { if !condition { throw CheckFailure(reason:message) } }

func awaitState(_ client: FortClient, conversationID: String, state: String, runID: String? = nil) async throws -> Snapshot {
    for _ in 0..<100 {
        let snapshot = try await client.snapshot(conversationID:conversationID)
        if snapshot.runs.contains(where:{$0.state == state && (runID == nil || $0.runId == runID)}) { return snapshot }
        try await Task.sleep(for:.milliseconds(100))
    }
    throw CheckFailure(reason:"fixture did not reach expected state")
}
@main struct NativeCheck {
    static func main() async throws {
        guard CommandLine.arguments.count == 3 else { throw CheckFailure(reason:"expected phase and fixture configuration path") }
        let config = try JSONDecoder().decode(Configuration.self,from:Data(contentsOf:URL(fileURLWithPath:CommandLine.arguments[2])))
        if CommandLine.arguments[1] == "redirect" {
            let client = try FortClient(baseURL:config.baseURL,installationID:config.installationId,token:"synthetic-bearer-canary",fixtureLoopback:true)
            do { _ = try await client.enroll(code:"SYNTHETIC-ENROLLMENT-CANARY",deviceName:"redirect regression"); throw CheckFailure(reason:"redirected enrollment was accepted") }
            catch is ClientError { }
            do { _ = try await client.installation(); throw CheckFailure(reason:"redirected authenticated read was accepted") }
            catch is ClientError { }
            print("PASS native refuses redirected enrollment and authenticated read")
            return
        }
        let stateFile = config.stateDirectory.appendingPathComponent("native-check.json")
        let store = try PendingStore(directory:config.stateDirectory,installationID:config.installationId)
        if CommandLine.arguments[1] == "controls-prepare" {
            let initial = try FortClient(baseURL:config.baseURL,installationID:config.installationId,fixtureLoopback:true)
            let a = try await initial.enroll(code:config.codeA,deviceName:"Native control A")
            let b = try await initial.enroll(code:config.codeB,deviceName:"Native control B")
            let client = try FortClient(baseURL:config.baseURL,installationID:config.installationId,token:a.deviceToken,fixtureLoopback:true)
            guard let agent = try await client.agents().first else { throw CheckFailure(reason:"missing fixture agent") }
            let pending = try store.enqueue(conversationID:agent.homeConversationId,bindingRevision:agent.binding.bindingRevisionId,recoveryEpoch:a.installation.recoveryEpoch,text:"Approve this fixture reply")
            _ = try await client.submit(pending)
            let snapshot = try await awaitState(client,conversationID:pending.conversationID,state:"waitingForApproval")
            try require(snapshot.approvals.first?.canDecide == true,"pending approval unavailable")
            try JSONEncoder().encode(CheckState(tokenA:a.deviceToken,tokenB:b.deviceToken,pending:pending,expectedSnapshot:snapshot)).write(to:stateFile,options:.atomic)
            try FileManager.default.setAttributes([.posixPermissions:0o600],ofItemAtPath:stateFile.path)
            print("PASS native exact approval contract and parked fixture run")
            return
        }
        if CommandLine.arguments[1] == "controls-replay" {
            let state = try JSONDecoder().decode(CheckState.self,from:Data(contentsOf:stateFile))
            let a = try FortClient(baseURL:config.baseURL,installationID:config.installationId,token:state.tokenA,fixtureLoopback:true)
            let b = try FortClient(baseURL:config.baseURL,installationID:config.installationId,token:state.tokenB,fixtureLoopback:true)
            let snapshot = try await a.snapshot(conversationID:state.pending.conversationID)
            try require(snapshot == state.expectedSnapshot,"parked approval changed across restart")
            guard let approval = snapshot.approvals.first else { throw CheckFailure(reason:"missing parked approval") }
            let winner = try await b.resolve(approval,approve:true)
            let repeated = try await a.resolve(approval,approve:false)
            try require(winner.approval == repeated.approval && winner.approval.state == "approved","approval decision winner changed")
            _ = try await awaitState(a,conversationID:snapshot.conversationId,state:"completed")
            let pending = try store.enqueue(conversationID:state.pending.conversationID,bindingRevision:state.pending.bindingRevision,recoveryEpoch:state.pending.recoveryEpoch,text:"Cancel this fixture reply")
            let accepted = try await a.submit(pending)
            _ = try await awaitState(b,conversationID:pending.conversationID,state:"waitingForApproval",runID:accepted.receipt.runId)
            let canceled = try await a.cancel(runID:accepted.receipt.runId,recoveryEpoch:pending.recoveryEpoch)
            try require(canceled.state == "canceled","parked cancellation was not terminal")
            let final = try await b.snapshot(conversationID:pending.conversationID)
            try require(!final.messages.contains(where:{$0.requestId == pending.requestID && $0.role == "assistant"}),"canceled reply appeared")
            print("PASS native restarted approval, exact decision winner, fixture continuation and cancellation")
            return
        }
        if CommandLine.arguments[1] == "prepare" {
            let enrolling = try FortClient(baseURL:config.baseURL,installationID:config.installationId,fixtureLoopback:true)
            let a = try await enrolling.enroll(code:config.codeA,deviceName:"Swift fixture client A")
            let b = try await enrolling.enroll(code:config.codeB,deviceName:"Swift fixture client B")
            let clientA = try FortClient(baseURL:config.baseURL,installationID:config.installationId,token:a.deviceToken,fixtureLoopback:true)
            let clientB = try FortClient(baseURL:config.baseURL,installationID:config.installationId,token:b.deviceToken,fixtureLoopback:true)
            let agents = try await clientA.agents()
            guard let agent = agents.first else { throw CheckFailure(reason:"missing fixture agent") }
            let pending = try store.enqueue(conversationID:agent.homeConversationId,bindingRevision:agent.binding.bindingRevisionId,recoveryEpoch:a.installation.recoveryEpoch,text:"  /native exact\r\ne\u{301} 🌱  ")
            let receipt = try await clientA.submit(pending)
            try require(receipt.receipt.requestId == pending.requestID,"acceptance correlation")
            var snapshot = try await clientB.snapshot(conversationID:pending.conversationID)
            for _ in 0..<100 where snapshot.runs.first?.state != "completed" {
                try await Task.sleep(for:.milliseconds(100))
                snapshot = try await clientB.snapshot(conversationID:pending.conversationID)
            }
            try require(snapshot.runs.first?.state == "completed","fixture did not complete")
            try require(snapshot.messages.count == 2 && snapshot.messages[0].text == pending.text,"exact durable transcript")
            let same = try await clientA.snapshot(conversationID:pending.conversationID)
            try require(same == snapshot,"two native clients diverged")
            let state = CheckState(tokenA:a.deviceToken,tokenB:b.deviceToken,pending:pending,expectedSnapshot:snapshot)
            try JSONEncoder().encode(state).write(to:stateFile,options:.atomic)
            try FileManager.default.setAttributes([.posixPermissions:0o600],ofItemAtPath:stateFile.path)
            print("PASS native HTTP enrollment, exact submit, fixture reply and two-client snapshot")
        } else {
            let state = try JSONDecoder().decode(CheckState.self,from:Data(contentsOf:stateFile))
            let clientA = try FortClient(baseURL:config.baseURL,installationID:config.installationId,token:state.tokenA,fixtureLoopback:true)
            let clientB = try FortClient(baseURL:config.baseURL,installationID:config.installationId,token:state.tokenB,fixtureLoopback:true)
            let saved = try store.pending()
            try require(saved == [state.pending],"pending identity changed across client restart")
            let repeated = try await clientA.submit(saved[0])
            try require(repeated.receipt.requestId == state.pending.requestID,"duplicate receipt identity")
            let snapshot = try await clientB.snapshot(conversationID:state.pending.conversationID)
            try require(snapshot == state.expectedSnapshot,"service restart changed canonical transcript")
            let events = try await clientA.events(after:0,recoveryEpoch:state.pending.recoveryEpoch)
            try require(events.cursor >= snapshot.cursor && !events.events.isEmpty,"durable replay unavailable")
            if let last = snapshot.messages.last {
                try await clientB.acknowledgeRead(conversationID:snapshot.conversationId,messageID:last.messageId,cursor:snapshot.cursor,recoveryEpoch:snapshot.recoveryEpoch)
                let read = try await clientA.snapshot(conversationID:snapshot.conversationId)
                try require(read.lastReadMessageSequence == last.sequence,"shared read state")
            }
            try store.remove(requestID:state.pending.requestID)
            print("PASS native client/service restart, stable retry, durable replay and shared read state")
        }
    }
}
