import Foundation
import FortKit

struct Configuration: Codable { let baseURL: URL; let installationId: String; let codeA: String; let codeB: String; let stateDirectory: URL }
struct CheckState: Codable { let tokenA: String; let tokenB: String; let pending: PendingSubmission; let expectedSnapshot: Snapshot }
struct RoutineCheckState: Codable { let revision: RoutineRevision; let pending: PendingSubmission; let receipt: Receipt; let approval: Approval }
struct RecoveryCheckState: Codable { let uncertain: Receipt; let queued: Receipt }
struct CheckFailure: Error { let reason: String }
struct WorkspaceCheckState: Codable { let token: String; let writer: Agent; let reviewer: Agent; let room: Conversation; let skill: InstructionSkill; let pending: PendingSubmission; let receipts: [Receipt]; let completedRunIDs: [String]; let pendingRequestCount: Int }
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
        if CommandLine.arguments[1].hasPrefix("workspace-pending-") {
            let state = try JSONDecoder().decode(WorkspaceCheckState.self,from:Data(contentsOf:stateFile))
            let client = try FortClient(baseURL:config.baseURL,installationID:config.installationId,token:state.token,fixtureLoopback:true)
            let installation = try await client.installation()
            try require(installation.supports("requestCancellation"),"request cancellation capability missing")
            if CommandLine.arguments[1] == "workspace-pending-prepare" {
                try require(try store.pending().isEmpty,"pending cancellation fixture requires an empty queue")
                let unsent = try store.enqueue(conversationID:state.writer.homeConversationId,bindingRevision:state.writer.binding.bindingRevisionId,recoveryEpoch:installation.recoveryEpoch,text:"Synthetic never submitted",recipientAgentID:state.writer.id,skills:[])
                try store.requestCancellation(unsent)
                let uncertain = try store.enqueue(conversationID:state.writer.homeConversationId,bindingRevision:state.writer.binding.bindingRevisionId,recoveryEpoch:installation.recoveryEpoch,text:"Synthetic pending cancellation acceptance lost",recipientAgentID:state.writer.id,skills:[])
                do { _ = try await client.submit(uncertain); throw CheckFailure(reason:"expected a discarded acceptance") }
                catch ClientError.service(let code,_) { try require(code == "syntheticLostAcceptance","unexpected acceptance error") }
                try store.requestCancellation(uncertain)
                do { _ = try await client.cancelPending(uncertain); throw CheckFailure(reason:"expected a discarded cancellation") }
                catch ClientError.service(let code,_) { try require(code == "syntheticLostCancellation","unexpected cancellation error") }
                try require(try store.pending().allSatisfy { $0.cancelRequested == true && !$0.canRetry(in:installation.recoveryEpoch) },"cancellation intent was not durably held")
                print("PASS native cancellation intent persisted after discarded acceptance and cancellation responses")
            } else if CommandLine.arguments[1] == "workspace-pending-replay" {
                let pending = try store.pending(); try require(pending.count == 2,"restart lost pending cancellations")
                for item in pending {
                    try require(item.cancelRequested == true && !item.canRetry(in:installation.recoveryEpoch),"restart would replay canceled submission")
                    let result = try await client.cancelPending(item)
                    try require(result.requestId == item.requestID && result.conversationId == item.conversationID && result.bindingRevisionId == item.bindingRevision && result.recoveryEpoch == item.recoveryEpoch,"cancellation scope mismatch")
                    if item.text == "Synthetic never submitted" {
                        try require(result.status == .notAccepted && result.receipt == nil,"unsent request unexpectedly accepted")
                        do { _ = try await client.submit(item); throw CheckFailure(reason:"canceled request accepted late") }
                        catch ClientError.service(let code,_) { try require(code == "requestCanceled","wrong late-submission error") }
                    } else {
                        guard let receipt = result.receipt, let snapshot = result.snapshot else { throw CheckFailure(reason:"lost acceptance not reconciled") }
                        let original = try await client.receipt(requestID:item.requestID)
                        try require(result.status == .accepted && result.run?.state == "canceled" && snapshot.runs.contains { $0.id == receipt.runId && $0.state == "canceled" } && receipt == original.receipt && item.matches(original),"accepted queued request not canceled")
                    }
                    try store.remove(requestID:item.requestID)
                }
                try require(try store.pending().isEmpty,"cancellation acknowledgement did not clear pending")
                print("PASS native cancellation replay after restart, exact acceptance reconciliation and late-submission fence")
            } else { throw CheckFailure(reason:"unknown pending cancellation phase") }
            return
        }
        if CommandLine.arguments[1] == "workspace-prepare" {
            let initial = try FortClient(baseURL:config.baseURL,installationID:config.installationId,fixtureLoopback:true)
            let enrollment = try await initial.enroll(code:config.codeA,deviceName:"Synthetic native workspace")
            let client = try FortClient(baseURL:config.baseURL,installationID:config.installationId,token:enrollment.deviceToken,fixtureLoopback:true)
            let writer = try await client.saveAgent(nil,name:"Writer",model:"gpt-6-astra",instructions:"Reply briefly using supplied context.")
            let reviewer = try await client.saveAgent(nil,name:"Reviewer",model:"gpt-6-astra",instructions:"Check the sample claim.")
            let draftSkill = try await client.saveSkill(nil,name:"Check marker",instructions:"End with [CHECKED].")
            try require(!draftSkill.canInvoke && draftSkill.bundle?.files.count == 1,"instruction authoring bypassed bundle review")
            let skill = try await client.reviewSkill(draftSkill,enable:true)
            let room = try await client.createConversation(name:"Sample planning",participantAgentIds:[writer.id,reviewer.id])
            let replayBaseline = try await client.installation()
            guard let eventCursor = replayBaseline.eventCursor else { throw CheckFailure(reason:"missing event cursor") }
            async let firstChange = client.events(after:eventCursor,recoveryEpoch:replayBaseline.recoveryEpoch,waitMilliseconds:5000)
            var first: PendingSubmission?
            var receipts: [Receipt] = [], completedRunIDs: [String] = []
            let samples: [(Agent,String,String,[SkillReference])] = [
                (writer,writer.homeConversationId,"Remember the sample word cedar. Reply with that word only.",[]),
                (reviewer,reviewer.homeConversationId,"In one sentence, explain why a checklist is useful.",[]),
                (writer,writer.homeConversationId,"Suggest one short title for a sample packing list.",[skill.reference]),
                (reviewer,room.id,"Our sample plan is to pack a red notebook. Repeat the item.",[])
            ]
            for (agent,conversationID,text,selectedSkills) in samples {
                let pending = try store.enqueue(conversationID:conversationID,bindingRevision:agent.binding.bindingRevisionId,recoveryEpoch:enrollment.installation.recoveryEpoch,text:text,recipientAgentID:agent.id,skills:selectedSkills)
                if first == nil { first = pending }
                let acceptance = try await client.submit(pending)
                let replay = try await client.receipt(requestID:pending.requestID)
                try require(replay.receipt.runId == acceptance.receipt.runId,"receipt lookup changed run")
                let snapshot = try await awaitState(client,conversationID:conversationID,state:"completed",runID:acceptance.receipt.runId)
                let reply = snapshot.messages.first { $0.requestId == pending.requestID && $0.role == "assistant" }
                try require(reply?.agentId == agent.id,"reply attribution changed")
                let acceptedRun = snapshot.runs.first { $0.runId == acceptance.receipt.runId }
                try require(acceptedRun?.configuration?.skills.map(\.reference) == selectedSkills,"selected skill revision changed")
                try require(acceptedRun?.state == "completed","sample run was not completed")
                receipts.append(acceptance.receipt); completedRunIDs.append(acceptance.receipt.runId)
                try store.remove(requestID:pending.requestID)
            }
            let changed = try await firstChange
            try require(changed.semantics == "invalidation" && !changed.events.isEmpty,"native change replay did not wake")
            let caughtUp = try await client.installation()
            guard let caughtUpCursor = caughtUp.eventCursor else { throw CheckFailure(reason:"missing refreshed cursor") }
            let idle = try await client.events(after:caughtUpCursor,recoveryEpoch:caughtUp.recoveryEpoch,waitMilliseconds:20)
            try require(idle.events.isEmpty && idle.cursor == caughtUpCursor,"native idle replay advanced state")
            try require(try store.pending().isEmpty,"native queue is not empty")
            try require(receipts.count == 4 && Set(completedRunIDs).count == 4,"four distinct completed sample receipts are required")
            let state = WorkspaceCheckState(token:enrollment.deviceToken,writer:writer,reviewer:reviewer,room:room,skill:skill,pending:first!,receipts:receipts,completedRunIDs:completedRunIDs,pendingRequestCount:try store.pending().count)
            try JSONEncoder().encode(state).write(to:stateFile,options:.atomic)
            try FileManager.default.setAttributes([.posixPermissions:0o600],ofItemAtPath:stateFile.path)
            print("PASS native workspace creation, four synthetic turns, receipts, skills, attribution, invalidation/idle replay and empty pending queue")
            return
        }
        if CommandLine.arguments[1] == "workspace-replay" {
            let state = try JSONDecoder().decode(WorkspaceCheckState.self,from:Data(contentsOf:stateFile))
            try require(state.receipts.count == 4 && Set(state.completedRunIDs).count == 4 && state.pendingRequestCount == 0,"restart gate is missing first-session evidence")
            try require(try store.pending().isEmpty,"restart gate refuses a nonempty native pending queue")
            let client = try FortClient(baseURL:config.baseURL,installationID:config.installationId,token:state.token,fixtureLoopback:true)
            for receipt in state.receipts {
                let saved = try await client.receipt(requestID:receipt.requestId)
                let snapshot = try await client.snapshot(conversationID:receipt.conversationId)
                try require(saved.receipt.runId == receipt.runId && state.completedRunIDs.contains(receipt.runId),"sample receipt identity changed")
                try require(snapshot.runs.contains { $0.runId == receipt.runId && $0.state == "completed" },"sample completion is missing after restart")
            }
            let agents = try await client.agents(), skills = try await client.skills(), rooms = try await client.conversations()
            try require(agents.contains(state.writer) && agents.contains(state.reviewer),"Agent identity changed after restart")
            try require(skills == [state.skill] && rooms.contains(state.room),"workspace changed after restart")
            let duplicate = try await client.submit(state.pending)
            try require(duplicate.receipt.requestId == state.pending.requestID,"request identity changed")
            let pending = try store.enqueue(conversationID:state.writer.homeConversationId,bindingRevision:state.writer.binding.bindingRevisionId,recoveryEpoch:state.pending.recoveryEpoch,text:"What sample word did I ask you to remember earlier? Reply with that word only.",recipientAgentID:state.writer.id,skills:[])
            let acceptance = try await client.submit(pending)
            let snapshot = try await awaitState(client,conversationID:pending.conversationID,state:"completed",runID:acceptance.receipt.runId)
            try require(snapshot.messages.last?.text == "cedar","synthetic restart reply was not saved")
            try store.remove(requestID:pending.requestID)
            try require(try store.pending().isEmpty,"pending queue remained after restart sample")
            print("PASS native and service restart, exact workspace identity, duplicate receipt and fifth synthetic reply")
            return
        }
        if CommandLine.arguments[1].hasPrefix("workspace-recovery-") {
            let state = try JSONDecoder().decode(WorkspaceCheckState.self,from:Data(contentsOf:stateFile))
            let client = try FortClient(baseURL:config.baseURL,installationID:config.installationId,token:state.token,fixtureLoopback:true)
            let recoveryFile = config.stateDirectory.appendingPathComponent("recovery-check.json")
            let conversationID = state.writer.homeConversationId
            let epoch = state.pending.recoveryEpoch
            func enqueue(_ text: String) throws -> PendingSubmission {
                try store.enqueue(conversationID:conversationID,bindingRevision:state.writer.binding.bindingRevisionId,recoveryEpoch:epoch,text:text,recipientAgentID:state.writer.id,skills:[])
            }
            switch CommandLine.arguments[1] {
            case "workspace-recovery-prepare":
                let uncertain = try enqueue("Synthetic uncertain turn")
                let first = try await client.submit(uncertain)
                _ = try await awaitState(client,conversationID:conversationID,state:"outcomeUnknown",runID:first.receipt.runId)
                try store.remove(requestID:uncertain.requestID)
                let queued = try enqueue("Synthetic queued turn to cancel after restart")
                let second = try await client.submit(queued)
                try store.remove(requestID:queued.requestID)
                let snapshot = try await client.snapshot(conversationID:conversationID)
                try require(snapshot.runs.last?.state == "queued","uncertainty did not hold later work")
                try JSONEncoder().encode(RecoveryCheckState(uncertain:first.receipt,queued:second.receipt)).write(to:recoveryFile,options:.atomic)
                print("PASS native unknown outcome holds later work before restart")
            case "workspace-recovery-close":
                let recovery = try JSONDecoder().decode(RecoveryCheckState.self,from:Data(contentsOf:recoveryFile))
                let snapshot = try await client.snapshot(conversationID:conversationID)
                try require(snapshot.runs.contains { $0.runId == recovery.uncertain.runId && $0.state == "outcomeUnknown" },"restart lost uncertainty")
                try require(snapshot.runs.contains { $0.runId == recovery.queued.runId && $0.state == "queued" },"restart lost held work")
                for _ in 0..<2 {
                    let canceled = try await client.cancel(runID:recovery.queued.runId,recoveryEpoch:epoch)
                    try require(canceled.state == "canceled","held work was not canceled")
                    let closed = try await client.closeUnknown(runID:recovery.uncertain.runId,recoveryEpoch:epoch)
                    try require(closed.state == "closedUnknown","unknown work was not explicitly closed")
                }
                let pending = try enqueue("Synthetic acceptance response lost")
                do {
                    _ = try await client.submit(pending)
                    throw CheckFailure(reason:"expected the synthetic gateway to discard acceptance")
                } catch ClientError.service(let code, _) {
                    try require(code == "syntheticLostAcceptance","unexpected service error")
                }
                try require(try store.pending() == [pending],"lost acceptance changed the durable pending identity")
                print("PASS native restarted queued cancellation, idempotent explicit closure and retained pending after lost acceptance")
            case "workspace-recovery-replay":
                let pending = try store.pending()
                try require(pending.count == 1,"expected exactly one saved unacknowledged submission")
                let saved = try await client.receipt(requestID:pending[0].requestID)
                let replay = try await client.submit(pending[0])
                try require(replay.receipt.runId == saved.receipt.runId && replay.receipt.messageId == saved.receipt.messageId && replay.receipt.requestId == saved.receipt.requestId && replay.receipt.inputDigest == saved.receipt.inputDigest,"lost acceptance replay changed the receipt")
                _ = try await awaitState(client,conversationID:conversationID,state:"completed",runID:saved.receipt.runId)
                try store.remove(requestID:pending[0].requestID)
                let next = try enqueue("Continue in this Home after recovery. What sample word was saved?")
                let accepted = try await client.submit(next)
                let snapshot = try await awaitState(client,conversationID:conversationID,state:"completed",runID:accepted.receipt.runId)
                try require(snapshot.messages.last?.text == "cedar","same-Home continuation lost its synthetic context")
                try require(snapshot.messages.filter { $0.requestId == pending[0].requestID }.count == 2,"lost acceptance duplicated a message or reply")
                try require(snapshot.runs.contains { $0.state == "closedUnknown" } && snapshot.runs.contains { $0.state == "canceled" },"recovery decisions did not persist")
                try store.remove(requestID:next.requestID)
                try require(try store.pending().isEmpty,"recovery left native submissions pending")
                print("PASS native lost acceptance reconciliation after restart, no duplicate execution and same-Home continuation")
            default: throw CheckFailure(reason:"unknown workspace recovery phase")
            }
            return
        }
        if CommandLine.arguments[1].hasPrefix("workspace-routine") {
            let state = try JSONDecoder().decode(WorkspaceCheckState.self,from:Data(contentsOf:config.stateDirectory.appendingPathComponent("native-check.json")))
            let client = try FortClient(baseURL:config.baseURL,installationID:config.installationId,token:state.token,fixtureLoopback:true)
            let installation = try await client.installation(), file = config.stateDirectory.appendingPathComponent("routine-check.json")
            let store = try PendingStore(directory:config.stateDirectory,installationID:config.installationId)
            if CommandLine.arguments[1] == "workspace-routine-prepare" {
                try require(installation.supports("routines"),"routine capability missing")
                let raw: [String:Any] = ["schemaVersion":1,"name":"Native summary","agentId":state.writer.id,"conversationId":state.writer.homeConversationId,"bindingRevisionId":state.writer.binding.bindingRevisionId,"instructions":"Use only declared inputs and outputs.","inputs":[["name":"topic","type":"text","required":true]],"skills":[],"steps":[["stepId":"draft","name":"Draft","type":"task","instructions":"Draft the topic.","inputNames":["topic"],"outputStepIds":[]],["stepId":"review","name":"Review","type":"approval","prompt":"Continue with this saved draft?"],["stepId":"finish","name":"Finish","type":"task","instructions":"Summarize the approved draft.","inputNames":[],"outputStepIds":["draft"]]]]
                let definition = try JSONDecoder().decode(RoutineDefinition.self,from:JSONSerialization.data(withJSONObject:raw))
                let saved = try await client.saveRoutine(nil,definition:definition)
                try require(saved.definition == definition,"native routine definition changed")
                let old = try await client.routineRevision(routineID:saved.id,revisionID:saved.revisionId)
                try require(old == saved,"immutable routine export changed")
                let pending = try store.enqueueRoutine(saved,inputs:["topic":.text("  Synthetic Café e\u{301}\r\n")],recoveryEpoch:installation.recoveryEpoch)
                let accepted = try await client.submit(pending); try require(pending.matches(accepted),"native routine receipt did not match")
                let snapshot = try await awaitState(client,conversationID:pending.conversationID,state:"waitingForApproval",runID:accepted.receipt.runId)
                guard let approval = snapshot.approvals.first(where:{$0.runId == accepted.receipt.runId}) else { throw CheckFailure(reason:"routine gate missing") }
                try require(approval.canDecide && approval.resourceScope == ["routine:continue"],"native routine gate is not recognized")
                try require(snapshot.runs.first(where:{$0.id == accepted.receipt.runId})?.routine?.steps.map(\.state) == ["completed","waitingForApproval","pending"],"routine checkpoint did not persist")
                try JSONEncoder().encode(RoutineCheckState(revision:saved,pending:pending,receipt:accepted.receipt,approval:approval)).write(to:file,options:.atomic)
                print("PASS native routine client contract, typed inputs, immutable export, saved task and exact owner gate")
            } else {
                let saved = try JSONDecoder().decode(RoutineCheckState.self,from:Data(contentsOf:file))
                try require(try store.pending() == [saved.pending],"routine pending command changed across restart")
                let duplicate = try await client.submit(saved.pending); try require(duplicate.receipt == saved.receipt && saved.pending.matches(duplicate),"routine retry changed acceptance")
                let snapshot = try await client.snapshot(conversationID:saved.pending.conversationID)
                try require(snapshot.approvals.contains(saved.approval),"restart changed gate identity")
                let approved = try await client.resolve(saved.approval,approve:true)
                let repeated = try await client.resolve(saved.approval,approve:false)
                try require(approved.approval == repeated.approval && approved.approval.state == "approved","routine decision winner changed")
                let finished = try await awaitState(client,conversationID:saved.pending.conversationID,state:"completed",runID:saved.receipt.runId)
                try require(finished.runs.first(where:{$0.id == saved.receipt.runId})?.routine?.steps.map(\.state) == ["completed","approved","completed"],"routine continuation lost checkpoints")
                try store.remove(requestID:saved.pending.requestID)
                let raw: [String:Any] = ["schemaVersion":1,"routineId":saved.revision.id,"routineRevisionId":saved.revision.revisionId,"kind":"daily","timezone":"Pacific/Auckland","time":"09:00","inputs":["topic":"Synthetic schedule"],"missed":"skip"]
                let definition = try JSONDecoder().decode(ScheduleDefinition.self,from:JSONSerialization.data(withJSONObject:raw))
                let schedule = try await client.saveSchedule(nil,definition:definition)
                try require(schedule.state == "paused" && schedule.definition == definition,"saving enabled or changed schedule")
                let preview = try await client.previewSchedule(schedule)
                try require(preview.revisionId == schedule.revisionId && preview.occurrences.count == 5 && preview.overlap == "skip","schedule preview changed authority")
                let enabled = try await client.setSchedule(schedule,enabled:true,preview:preview,recoveryEpoch:installation.recoveryEpoch)
                try require(enabled.state == "enabled","explicit schedule consent was not saved")
                let paused = try await client.setSchedule(enabled,enabled:false,preview:nil,recoveryEpoch:installation.recoveryEpoch)
                try require(paused.state == "paused","schedule did not pause")
                try require(try await client.scheduleOccurrences(paused).isEmpty,"schedule unexpectedly dispatched during native review")
                try require(try store.pending().isEmpty,"native routine pending remained")
                print("PASS native routine restart, idempotent receipt and gate decision, declared-output continuation, schedule preview/enable/pause with no schedule execution")
            }
            return
        }
        if CommandLine.arguments[1] == "workspace-bundles" {
            let state = try JSONDecoder().decode(WorkspaceCheckState.self,from:Data(contentsOf:stateFile))
            let client = try FortClient(baseURL:config.baseURL,installationID:config.installationId,token:state.token,fixtureLoopback:true)
            let asset = Data([0,255,10]).base64EncodedString()
            var files: [[String:Any]] = [["path":"SKILL.md","kind":"file","executable":false,"contentBase64":Data("Use the complete reviewed bundle.".utf8).base64EncodedString()],["path":"assets/sample.bin","kind":"file","executable":false,"contentBase64":asset]]
            var input: [String:Any] = ["schemaVersion":1,"name":"Native reviewed bundle","version":"1","source":"owner-local:synthetic-native","license":"CC0-1.0","entrypoint":"SKILL.md","supportedRuntimes":["codex"],"requestedTools":[],"scripts":[],"files":files]
            let imported = try await client.importSkillBundle(JSONSerialization.data(withJSONObject:input))
            guard let original = imported.bundle else { throw CheckFailure(reason:"native import dropped bundle metadata") }
            try require(original.files.count == 2 && original.files.last?.contentBase64 == asset && !imported.canInvoke,"inert native import changed files or enabled itself")
            try require(!original.supportsTextChat(provider:"codex"),"multi-file bundle hid the current runtime limitation")
            let enabled = try await client.reviewSkill(imported,enable:true)
            try require(enabled.canInvoke,"explicit native review did not enable the exact revision")
            input["version"] = "2"; files[0]["contentBase64"] = Data("Changed reviewed instructions.".utf8).base64EncodedString(); input["files"] = files
            let changed = try await client.importSkillBundle(JSONSerialization.data(withJSONObject:input),replacing:enabled)
            try require(!changed.canInvoke && changed.previousRevisionId == enabled.revisionId,"bundle edit lost history or bypassed review")
            let prior = try await client.skillRevision(skillID:enabled.id,revisionID:enabled.revisionId)
            try require(prior.bundle == original,"old bundle changed after a new revision")
            try require(changed.bundle?.changes(from:prior.bundle).map { $0.path + ":" + $0.change } == ["SKILL.md:Changed"],"native review did not expose the changed instruction file")
            do { _ = try await client.reviewSkill(enabled,enable:true); throw CheckFailure(reason:"stale native review succeeded") }
            catch ClientError.service(let code,_) { try require(code == "skillChanged","stale review returned the wrong conflict") }
            let exported = try JSONEncoder().encode(original)
            let roundTrip = try await client.importSkillBundle(exported)
            try require(roundTrip.bundle == original && !roundTrip.canInvoke,"native export/import changed complete contents or retained enablement")
            let disabled = try await client.reviewSkill(enabled,enable:false)
            try require(!disabled.canInvoke,"explicit native disable did not revoke the older revision")
            print("PASS native complete bundle import/review, binary asset preservation, changed-file review, stale-revision rejection and export/import without enablement")
            return
        }
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
