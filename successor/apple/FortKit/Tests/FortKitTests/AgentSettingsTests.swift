import Foundation
import Testing
@testable import FortKit
#if os(macOS)
import AppKit
import SwiftUI

@Test @MainActor func pairingSettingsAllowsNormalQuitWithoutChangingDraftOrConnection() async throws {
    let root = FileManager.default.temporaryDirectory.appendingPathComponent("FortAccountQA-" + UUID().uuidString)
    defer { try? FileManager.default.removeItem(at:root) }
    let model = try await AccountSignInFixture.make(directory:root)
    model.draft = "Retained synthetic draft"
    let connectionURL = root.appendingPathComponent("connection.json")
    let connection = try Data(contentsOf:connectionURL)
    let window = NSWindow(contentRect:NSRect(x:0,y:0,width:500,height:600),styleMask:[.titled],backing:.buffered,defer:false)
    window.isReleasedWhenClosed = false
    defer { window.close() }
    #expect(window.preventsApplicationTerminationWhenModal)
    let content = NSHostingView(rootView:PairingSettingsView(model:model))
    window.contentView = content
    content.layoutSubtreeIfNeeded()
    #expect(!window.preventsApplicationTerminationWhenModal)
    #expect(model.draft == "Retained synthetic draft")
    #expect(try Data(contentsOf:connectionURL) == connection)
}
#endif
private let legacyAgent = #"{"agentId":"writer","name":"Writer","homeConversationId":"home","readiness":"requiresLiveReview","instructions":"  Keep my voice.\n","binding":{"bindingRevisionId":"one","provider":"codex","runtimeVersion":"0.157.1","selectedModel":"exact-model","hostId":"local-codex","enabled":true}}"#
@Test @MainActor func agentSettingsPreserveLegacyIdentityAndPersistBundledAvatarAcrossRelaunch() throws {
    var agent = try JSONDecoder().decode(Agent.self,from:Data(legacyAgent.utf8))
    #expect(agent.avatar == nil)
    let form = WorkspaceForm(agent:agent)
    #expect(form.instructions == "  Keep my voice.\n")
    #expect(form.exactModel == "exact-model")
    agent.avatar = .sprout
    let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
    defer { try? FileManager.default.removeItem(at:root) }
    let installation = Installation(contractVersion:"1.0",installationId:"settings-test",recoveryEpoch:"epoch",mode:"openai",serviceName:"Fort Prototype")
    try DeviceStorage(directory:root).saveNavigation(SavedNavigation(installation:installation,agents:[agent],selectedAgentID:agent.id))
    let restored = try #require(DeviceStorage(directory:root).navigation(installationID:installation.installationId)?.agents.first)
    #expect(restored == agent)
    #expect(restored.instructions == "  Keep my voice.\n")
}
@Test func modelPickerKeepsMissingSelectionAndNeverSubstitutesAfterDiscoveryFailure() {
    let ready = ModelCatalog(provider:"codex",state:"ready",models:[ModelOption(id:"listed",name:"Listed",availability:"available")],checkedAt:"2026-10-17T00:00:00Z")
    #expect(ready.options(preserving:"retired").last?.id == "retired")
    #expect(ready.options(preserving:"retired").last?.availability == "unavailable")
    let failed = ModelCatalog(provider:"codex",state:"error",models:[],detail:"Discovery failed")
    #expect(failed.options(preserving:"retired").map(\.id) == ["retired"])
    #expect(failed.options(preserving:"retired").last?.availability == "unchecked")
    #expect(!failed.canSelect("retired"))
    #expect(ready.canSelect("listed"))
}
@Test func avatarMotionIsStaticUnderReducedMotion() {
    #expect(AvatarMotion.phase(time:10,reducedMotion:true) == 0)
    #expect(AvatarMotion.phase(time:10,reducedMotion:false) != AvatarMotion.phase(time:11,reducedMotion:false))
}

@Test func providerNeutralCatalogAllowsOnlyTheExactAvailableModel() {
    let catalog = ModelCatalog(provider:"local-stateless",state:"ready",models:[ModelOption(id:"local-exact",name:"Local model",availability:"available")])
    #expect(catalog.canSelect("local-exact"))
    #expect(!catalog.canSelect("codex-default"))
    let installation = Installation(contractVersion:"1.0",installationId:"local",recoveryEpoch:"one",mode:"local",serviceName:"Fort")
    #expect(installation.supportsCommands)
}
