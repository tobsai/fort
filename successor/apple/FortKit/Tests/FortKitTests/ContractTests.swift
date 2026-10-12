import Foundation
import Testing
@testable import FortKit

@Test func nativeRefusesInsecureNonLoopbackEndpoint() {
    #expect(throws: ClientError.self) { try FortClient(baseURL: URL(string: "http://192.0.2.1")!, installationID: "one", fixtureLoopback: true) }
    #expect(throws: ClientError.self) { try FortClient(baseURL: URL(string: "https://user:secret@example.invalid")!, installationID: "one") }
}
@Test func recoveryEpochPreventsAutomaticResubmission() throws {
    let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
    defer { try? FileManager.default.removeItem(at: root) }
    let store = try PendingStore(directory: root, installationID: "one")
    let pending = try store.enqueue(conversationID: "home", bindingRevision: "binding", recoveryEpoch: "before-restore", text: "hello")
    #expect(!pending.canRetry(in: "after-restore"))
    #expect(pending.canRetry(in: "before-restore"))
}
@Test func snapshotsIgnoreStaleDeliveryAndRejectForeignInstallation() throws {
    let current = Snapshot(installationId:"one", recoveryEpoch:"epoch", conversationId:"home", cursor:2, messages:[], runs:[], lastReadMessageSequence:0)
    var transcript = Transcript(installationID:"one", conversationID:"home")
    try transcript.apply(current)
    try transcript.apply(Snapshot(installationId:"one", recoveryEpoch:"epoch", conversationId:"home", cursor:1, messages:[], runs:[], lastReadMessageSequence:0))
    #expect(transcript.snapshot == current)
    #expect(throws: ContractError.wrongInstallation) { try transcript.apply(Snapshot(installationId:"two", recoveryEpoch:"epoch", conversationId:"home", cursor:3, messages:[], runs:[], lastReadMessageSequence:0)) }
}
@Test func unknownRunStateIsInformative() {
    let run = Run(runId:"run", requestId:"request", conversationId:"home", bindingRevisionId:"binding", state:"future-state", selectedModel:"selected", effectiveModel:nil, modelEvidence:"unreported", attemptId:nil)
    #expect(run.displayState.contains("Unsupported"))
}

@Test func sharedTypeScriptFixturesDecodeAndReplayInSwift() throws {
    let root = URL(fileURLWithPath:#filePath).deletingLastPathComponent().appendingPathComponent("../../../../contracts/fixtures")
    let installation = try JSONDecoder().decode(Installation.self,from:Data(contentsOf:root.appendingPathComponent("installation.json")))
    let agents = try JSONDecoder().decode(AgentList.self,from:Data(contentsOf:root.appendingPathComponent("agents.json")))
    let snapshot = try JSONDecoder().decode(Snapshot.self,from:Data(contentsOf:root.appendingPathComponent("snapshot.json")))
    #expect(installation.supportsCommands)
    #expect(agents.agents.first?.homeConversationId == snapshot.conversationId)
    var transcript = Transcript(installationID:installation.installationId,conversationID:snapshot.conversationId)
    try transcript.apply(snapshot)
    try transcript.apply(snapshot)
    #expect(transcript.snapshot?.messages.count == 2)
    #expect(transcript.snapshot?.messages.first?.text == "  /hello\r\nCafé e\u{301} 👋  ")
}
