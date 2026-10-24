import Foundation
import Testing
@testable import FortKit
#if os(macOS)
import AppKit
import SwiftUI
#endif

private let owner = PauseMenu.Facts(hasWorkspace:true,supportsRoutines:true,canManageAccount:true,accountSelected:true,showsDevices:true,runtimeState:"ready",pendingDecisions:0)

@Test func pauseMenuShowsCrewPlaybookAccountsAndDevicesForAMacOwner() {
    let menu = PauseMenu.make(owner)
    #expect(menu.entries.map(\.tab) == [.crew,.playbook,.accounts,.devices])
    #expect(menu.entries.allSatisfy { $0.badge == 0 })
    #expect(menu.opening(nil) == .crew)
}

@Test func pauseMenuHidesWhatThisDeviceCannotManage() {
    var phone = owner; phone.canManageAccount = false
    #expect(PauseMenu.make(phone).entries.map(\.tab) == [.crew,.playbook,.devices])
    let unenrolled = PauseMenu.Facts()
    #expect(PauseMenu.make(unenrolled).entries.map(\.tab) == [.devices])
    var preview = PauseMenu.Facts(); preview.showsDevices = false
    #expect(PauseMenu.make(preview).entries.isEmpty)
    #expect(PauseMenu.make(preview).opening(.crew) == nil)
    var routinesOnly = PauseMenu.Facts(); routinesOnly.supportsRoutines = true
    #expect(PauseMenu.make(routinesOnly).entries.map(\.tab) == [.playbook,.devices])
}

@Test func pauseMenuCountsWhatNeedsYouAndOpensThere() {
    var facts = owner
    facts.accountSelected = false
    facts.pendingDecisions = 2
    facts.runtimeState = "stopped"
    let menu = PauseMenu.make(facts)
    #expect(menu.entries.map(\.badge) == [2,0,1,1])
    #expect(menu.opening(nil) == .crew)
    #expect(menu.opening(.devices) == .devices)
    facts.pendingDecisions = 0
    #expect(PauseMenu.make(facts).opening(nil) == .accounts)
    facts.runtimeState = "restartRequired"; facts.accountSelected = true
    #expect(PauseMenu.make(facts).opening(nil) == .devices)
    facts.runtimeState = "ready"
    #expect(PauseMenu.make(facts).entries.allSatisfy { $0.badge == 0 })
}

@Test func agentClassIsTheRuntimeNameItIsBoundTo() {
    let runtimes = [RuntimeDescriptor(provider:"claude",name:"Claude",runtimeVersion:"1",hostId:"local",session:"s",capabilities:[]),
                    RuntimeDescriptor(provider:"codex",name:"ChatGPT",runtimeVersion:"1",hostId:"local",session:"s",capabilities:[])]
    #expect(PauseMenu.className(provider:"claude",runtimes:runtimes) == "Claude")
    #expect(PauseMenu.className(provider:"codex",runtimes:runtimes) == "ChatGPT")
    #expect(PauseMenu.className(provider:"local-stateless",runtimes:runtimes) == "local-stateless")
}

#if os(macOS)
@Test @MainActor func pauseMenuAllowsNormalQuitWithoutChangingDraftOrStoredState() throws {
    let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
    defer { try? FileManager.default.removeItem(at:root) }
    let model = try MessengerModel(directory:root)
    model.draft = "Unsent pause menu regression draft"
    let window = NSWindow(contentRect:NSRect(x:0,y:0,width:800,height:600),styleMask:[.titled],backing:.buffered,defer:false)
    window.isReleasedWhenClosed = false
    defer { window.close() }
    #expect(window.preventsApplicationTerminationWhenModal)
    let content = NSHostingView(rootView:PauseMenuView(model:model))
    window.contentView = content
    content.layoutSubtreeIfNeeded()
    #expect(!window.preventsApplicationTerminationWhenModal)
    #expect(!window.isVisible)
    #expect(model.draft == "Unsent pause menu regression draft")
    #expect(try FileManager.default.contentsOfDirectory(atPath:root.path).isEmpty)
}
#endif
