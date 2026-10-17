#if DEBUG
import Foundation
import Testing
@testable import FortKit

@Test @MainActor func interactiveFixtureUsesRealSettingsAndComposerWithoutEnrollmentOrCredentials() async throws {
    let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
    defer { try? FileManager.default.removeItem(at:root) }
    let model = try await InteractiveFixture.make(directory:root)
    #expect(model.isInteractiveFixture && model.hasWorkspace && !model.isOfflinePreview)
    #expect(!model.showConnection && model.supportsCommands)
    #expect(model.localImages.count == 1)
    let agent = try #require(model.selectedAgent)
    await model.refreshModels()
    #expect(model.modelCatalog.canSelect("fixture-beta"))
    #expect(await model.saveAgent(agent,name:"QA editor",model:"fixture-beta",instructions:"  Keep my voice.\n",avatar:.ember))
    model.draft = "A fixture message"; #expect(model.canSend); await model.send()
    #expect(model.pending.isEmpty)
    #expect(model.snapshot?.messages.last?.text.contains("Keep my voice.") == true)
    #expect(model.snapshot?.messages.last?.text.contains("fixture-beta") == true)
    let snapshot = model.snapshot
    let restored = try await InteractiveFixture.make(directory:root)
    #expect(restored.selectedAgent?.name == "QA editor")
    #expect(restored.selectedAgent?.instructions == "  Keep my voice.\n")
    #expect(restored.selectedAgent?.avatar == .ember)
    #expect(restored.snapshot?.messages == snapshot?.messages)
    #expect(restored.localImages.count == 1)
    await restored.setFixtureCatalog(.missingSelection)
    #expect(!restored.modelCatalog.canSelect("fixture-beta"))
    restored.endpoint = "https://must-not-contact.invalid"; restored.enrollmentCode = "not-a-credential"
    await restored.enroll(); await restored.restore()
    #expect(!restored.showConnection)
    #expect(!FileManager.default.fileExists(atPath:root.appendingPathComponent("connection.json").path))
    #expect(!FileManager.default.fileExists(atPath:root.appendingPathComponent("disconnected.json").path))
}
@Test @MainActor func interactiveFixtureExercisesUnavailableModelsAndImageRemovalAcrossReload() async throws {
    let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
    defer { try? FileManager.default.removeItem(at:root) }
    let model = try await InteractiveFixture.make(directory:root)
    let original = try #require(model.selectedAgent)
    await model.setFixtureCatalog(.missingSelection)
    #expect(!model.modelCatalog.canSelect(original.binding.selectedModel))
    #expect(model.modelCatalog.options(preserving:original.binding.selectedModel).last?.availability == "unavailable")
    await model.setFixtureCatalog(.failure)
    #expect(model.modelCatalog.state == "error")
    #expect(model.selectedAgent?.binding.selectedModel == original.binding.selectedModel)
    await model.setFixtureCatalog(.ready)
    #expect(model.modelCatalog.canSelect(original.binding.selectedModel))
    model.removeLocalImage(try #require(model.localImages.first))
    let restored = try await InteractiveFixture.make(directory:root)
    #expect(restored.localImages.isEmpty)
    await restored.addFixtureImage(); #expect(restored.localImages.count == 1)
    let otherRoot = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
    defer { try? FileManager.default.removeItem(at:otherRoot) }
    let other = try await InteractiveFixture.make(directory:otherRoot)
    #expect(other.installation?.installationId != restored.installation?.installationId)
}
#endif
