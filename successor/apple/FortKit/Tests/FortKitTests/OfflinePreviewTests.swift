#if DEBUG
import Foundation
import Testing
@testable import FortKit

@Test @MainActor func offlinePreviewNeverRestoresEnrollsOrSends() async throws {
    let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
    defer { try? FileManager.default.removeItem(at:root) }
    let model = try OfflinePreview.make(directory:root)
    #expect(model.isOfflinePreview && !model.showConnection)
    #expect(model.snapshot?.messages.count == 11 && model.snapshot?.runs.count == 6)
    #expect(model.snapshot?.messages.allSatisfy { !$0.text.isEmpty } == true)
    #expect(!model.canSend && !model.supportsCommands)
    model.endpoint = "https://example.invalid"
    model.enrollmentCode = "not-an-enrollment"
    model.draft = "Read-only sample"
    await model.restore(); await model.enroll(); await model.send(); await model.refresh()
    #expect(!model.canSend && !model.connected && model.pending.isEmpty)
    #expect(try FileManager.default.contentsOfDirectory(atPath:root.path).isEmpty)
}
#endif
