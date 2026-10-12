import Foundation
import Testing
@testable import FortKit

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
