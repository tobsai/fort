import Foundation
import Testing
@testable import FortKit

@Test func olderPendingReceiptMatchesItsExplicitMessageOutsideTheLatestPage() throws {
    let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
    defer { try? FileManager.default.removeItem(at:root) }
    let pending = try PendingStore(directory:root,installationID:"one").enqueue(conversationID:"home",bindingRevision:"binding",recoveryEpoch:"epoch",text:"Exact old request")
    let receipt = Receipt(requestId:pending.requestID,conversationId:"home",messageId:"old",runId:"run",bindingRevisionId:"binding",inputDigest:"digest",acceptedAt:"2026-10-18T00:00:00Z")
    let latest = Snapshot(installationId:"one",recoveryEpoch:"epoch",conversationId:"home",cursor:300,messages:[],runs:[],lastReadMessageSequence:0)
    let message = Message(messageId:"old",conversationId:"home",requestId:pending.requestID,role:"user",text:"Exact old request",sequence:1,createdAt:"2026-10-18T00:00:00Z")
    let receiptData = try JSONEncoder().encode(receipt)
    let snapshotData = try JSONEncoder().encode(latest)
    let messageData = try JSONEncoder().encode(message)
    let response = Data(("{\"receipt\":" + String(decoding:receiptData,as:UTF8.self) + ",\"snapshot\":" + String(decoding:snapshotData,as:UTF8.self) + ",\"message\":" + String(decoding:messageData,as:UTF8.self) + "}").utf8)
    let accepted = try JSONDecoder().decode(Acceptance.self,from:response)
    #expect(pending.matches(accepted))
}

@Test func pendingSubmissionSurvivesClientRestartWithoutChangingBytesOrIdentity() throws {
    let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
    defer { try? FileManager.default.removeItem(at: root) }
    let text = "  /literal\r\ne\u{301} 🌱  "
    let store = try PendingStore(directory: root, installationID: "installation-1")
    let original = try store.enqueue(conversationID: "home-1", bindingRevision: "binding-1", recoveryEpoch: "epoch-1", text: text)
    let reopened = try PendingStore(directory: root, installationID: "installation-1")
    #expect(try reopened.pending() == [original])
    #expect(Array(original.text.utf8) == Array(text.utf8))
    try reopened.remove(requestID: original.requestID)
    #expect(try store.pending().isEmpty)
}

@Test func installationsCannotSharePendingSubmissions() throws {
    let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
    defer { try? FileManager.default.removeItem(at: root) }
    let first = try PendingStore(directory: root, installationID: "one")
    _ = try first.enqueue(conversationID: "home", bindingRevision: "binding", recoveryEpoch: "epoch", text: "hello")
    let second = try PendingStore(directory: root, installationID: "two")
    #expect(try second.pending().isEmpty)
}

@Test func utf8ByteLimitIsExact() throws {
    let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
    defer { try? FileManager.default.removeItem(at: root) }
    let store = try PendingStore(directory: root, installationID: "one")
    _ = try store.enqueue(conversationID: "home", bindingRevision: "binding", recoveryEpoch: "epoch", text: String(repeating: "é", count: 32_768))
    #expect(throws: PendingStoreError.textTooLarge) {
        try store.enqueue(conversationID: "home", bindingRevision: "binding", recoveryEpoch: "epoch", text: String(repeating: "é", count: 32_768) + "x")
    }
}
