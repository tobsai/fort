import Foundation
import Testing
@testable import FortKit

#if DEBUG
@Test @MainActor func transcriptKeepsEachStatusBesideItsLastMessage() throws {
    let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
    defer { try? FileManager.default.removeItem(at:root) }
    let snapshot = try #require(OfflinePreview.make(directory:root).snapshot)
    let rows = snapshot.transcriptRows
    #expect(rows.suffix(2).map(\.id) == ["message:sample-reply-6", "run:sample-run-6"])
    #expect(rows.map(\.id).prefix(6) == ["message:sample-user-1", "message:sample-reply-1", "run:sample-run-1", "message:sample-user-2", "run:sample-run-2", "message:sample-user-3"])
    #expect(rows.compactMap { if case .message(let value) = $0 { value } else { nil } } == snapshot.messages)
    #expect(rows.count == snapshot.messages.count + snapshot.runs.count)
}
#endif

@Test func transcriptPreservesInterleavedMessagesAndAssociatesStatusesByRequest() {
    let messages = [sampleMessage("a-input",request:"a",sequence:1), sampleMessage("b-input",request:"b",sequence:2), sampleMessage("b-reply",request:"b",sequence:3), sampleMessage("a-reply",request:"a",sequence:4)]
    let snapshot = sampleSnapshot(messages:messages,runs:[sampleRun("a"),sampleRun("b")])
    #expect(snapshot.transcriptRows.map(\.id) == ["message:a-input","message:b-input","message:b-reply","run:b","message:a-reply","run:a"])
    #expect(snapshot.transcriptRows.compactMap { if case .message(let value) = $0 { value } else { nil } } == messages)
}

@Test func transcriptRetainsApprovalsAndRunsWithoutMessagesExactlyOnce() {
    let approvals = [sampleApproval("a",run:"a"),sampleApproval("b",run:"b"),sampleApproval("orphan",run:"missing")]
    let snapshot = sampleSnapshot(messages:[sampleMessage("a",request:"a",sequence:1)],runs:[sampleRun("b"),sampleRun("a")],approvals:approvals)
    #expect(snapshot.transcriptRows.map(\.id) == ["message:a","run:a","approval:a","run:b","approval:b","approval:orphan"])
    #expect(Set(snapshot.transcriptRows.map(\.id)).count == 6)
    #expect(snapshot.transcriptRows.compactMap { if case .approval(let value) = $0 { value } else { nil } } == approvals)
    #expect(sampleSnapshot(messages:[],runs:[]).transcriptRows.isEmpty)
}

private func sampleMessage(_ id: String, request: String, sequence: Int) -> Message {
    Message(messageId:id,conversationId:"home",requestId:request,role:"user",text:"  Exact sample \(id)\n",sequence:sequence,createdAt:"2026-10-17T00:00:00Z")
}
private func sampleRun(_ id: String) -> Run {
    Run(runId:id,requestId:id,conversationId:"home",bindingRevisionId:"binding",state:"waitingForApproval",selectedModel:"fixture",effectiveModel:nil,modelEvidence:"unreported",attemptId:nil)
}
private func sampleApproval(_ id: String, run: String) -> Approval {
    Approval(approvalId:id,installationId:"sample",recoveryEpoch:"epoch",runId:run,attemptId:"attempt",bindingRevisionId:"binding",actionDigest:"digest",resourceScope:["fixture:reply"],actionDescription:"Sample approval",expiresAt:"2026-10-17T00:00:00Z",state:"pending",decidedByDeviceId:nil,decidedAt:nil)
}
private func sampleSnapshot(messages: [Message], runs: [Run], approvals: [Approval] = []) -> Snapshot {
    Snapshot(installationId:"sample",recoveryEpoch:"epoch",conversationId:"home",cursor:messages.count,messages:messages,runs:runs,lastReadMessageSequence:0,approvals:approvals)
}
