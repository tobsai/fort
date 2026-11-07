#if os(macOS)
import AppKit
import Foundation
import SwiftUI
import Testing
@testable import FortKit

// Side-by-side screenshots, Mac half: the first run, the Quest log, a chat and Setup drawn from design/screens/fixture.json,
// the same records the web half (design/screens/web-screens.mjs) serves to the web client. Runs only when
// FORT_SCREENSHOTS names a folder, as the native CI job does, and writes one PNG per screen and mode.

private struct ScreenFixture: Decodable {
    let installation: Installation
    let agents: [Agent]
    let conversations: [Conversation]
    let snapshots: [String: Snapshot]
    let open: String
    let memory: MemoryFixture
}
private struct MemoryFixture: Decodable {
    let providers: [MemorySystem]
    let agents: [AgentMemoryStatus]
    let entries: [String: [MemoryNote]]
}

private let screenshotFolder = ProcessInfo.processInfo.environment["FORT_SCREENSHOTS"]

@MainActor private func fixtureModel(_ root: URL) throws -> MessengerModel {
    let file = URL(fileURLWithPath:#filePath).deletingLastPathComponent().appendingPathComponent("../../../../design/screens/fixture.json").standardizedFileURL
    let fixture = try JSONDecoder().decode(ScreenFixture.self,from:Data(contentsOf:file))
    let model = try MessengerModel(directory:root)
    model.installation = fixture.installation
    model.agents = fixture.agents
    model.conversations = fixture.conversations
    model.deskSnapshots = fixture.snapshots
    model.selectedConversationID = fixture.open
    model.snapshot = fixture.snapshots[fixture.open]
    model.connected = true
    model.status = "Connected to the hub"
    // Spec 068: what the hub answers for each Agent's memory, so Setup draws it without a hub.
    model.memoryStatuses = Dictionary(uniqueKeysWithValues:fixture.memory.agents.map { ($0.agentId,$0) })
    for status in fixture.memory.agents {
        model.memoryPages[status.agentId] = AgentMemoryPage(agentId:status.agentId,agentName:status.agentName,choice:status.choice,provider:status.provider,state:status.state,
            detail:status.detail,total:status.total,providers:fixture.memory.providers,entries:fixture.memory.entries[status.agentId] ?? [])
    }
    return model
}

@MainActor private func capture(_ view: some View, name: String, size: CGSize) async throws {
    let folder = try #require(screenshotFolder)
    try FileManager.default.createDirectory(atPath:folder,withIntermediateDirectories:true)
    for (mode, appearance) in [("light",NSAppearance.Name.aqua),("dark",NSAppearance.Name.darkAqua)] {
        let window = NSWindow(contentRect:NSRect(origin:.zero,size:size),styleMask:[.titled],backing:.buffered,defer:false)
        window.isReleasedWhenClosed = false
        window.appearance = NSAppearance(named:appearance)
        let host = NSHostingView(rootView:view.frame(width:size.width,height:size.height))
        host.frame = NSRect(origin:.zero,size:size)
        window.contentView = host
        // Let SwiftUI lay out, load and settle before drawing.
        for _ in 0..<4 { host.layoutSubtreeIfNeeded(); try await Task.sleep(for:.milliseconds(250)) }
        let scale: CGFloat = 1
        let rep = try #require(NSBitmapImageRep(bitmapDataPlanes:nil,pixelsWide:Int(size.width * scale),pixelsHigh:Int(size.height * scale),bitsPerSample:8,samplesPerPixel:4,hasAlpha:true,isPlanar:false,colorSpaceName:.deviceRGB,bytesPerRow:0,bitsPerPixel:0))
        rep.size = size
        host.cacheDisplay(in:host.bounds,to:rep)
        let png = try #require(rep.representation(using:.png,properties:[:]))
        try png.write(to:URL(fileURLWithPath:folder).appendingPathComponent("mac-\(name)-\(mode).png"))
        window.close()
    }
}

@Suite(.serialized, .enabled(if:screenshotFolder != nil,"Set FORT_SCREENSHOTS to write the side-by-side screenshots"))
@MainActor struct ScreenshotTests {
    @Test func questLog() async throws {
        let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        defer { try? FileManager.default.removeItem(at:root) }
        let model = try fixtureModel(root)
        try await capture(QuestLogView(model:model,openChat:{ _ in },showCity:nil,openMenu:{ _ in }),name:"quest-log",size:CGSize(width:1280,height:800))
    }

    @Test func chat() async throws {
        let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        defer { try? FileManager.default.removeItem(at:root) }
        let model = try fixtureModel(root)
        let defaults = UserDefaults.standard, saved = defaults.object(forKey:"fort.showDesk")
        defaults.set(false,forKey:"fort.showDesk"); defaults.set(false,forKey:"fort.showCity")
        defer { if let saved { defaults.set(saved,forKey:"fort.showDesk") } else { defaults.removeObject(forKey:"fort.showDesk") } }
        try await capture(MessengerView(model:model),name:"chat",size:CGSize(width:1280,height:800))
    }

    @Test func firstRun() async throws {
        let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        defer { try? FileManager.default.removeItem(at:root) }
        let model = try fixtureModel(root)
        model.agents = []; model.conversations = []; model.deskSnapshots = [:]
        try await capture(QuestLogView(model:model,openChat:{ _ in },showCity:nil,openMenu:{ _ in }),name:"first-run",size:CGSize(width:1280,height:800))
    }

    @Test func macConnect() async throws {
        let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        defer { try? FileManager.default.removeItem(at:root) }
        let model = try fixtureModel(root)
        model.status = ""
        try await capture(NavigationStack { MacConnectView(model:model,facts:DevicesBoard.Facts(hostSetup:.needsEnable)) },name:"mac-connect",size:CGSize(width:560,height:520))
    }

    /// The Connect to Fort sheet as it stood before the guided steps: every host and owner control at once.
    /// Kept as a screenshot so the before and after can be compared; the same controls now sit under More options.
    @Test func macConnectAllOptions() async throws {
        let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        defer { try? FileManager.default.removeItem(at:root) }
        let model = try MessengerModel(directory:root,localHost:MacHostControl())
        try await capture(NavigationStack { Form { ConnectionSections(model:model) }.formStyle(.grouped) },name:"mac-connect-all-options",size:CGSize(width:560,height:800))
    }

    @Test func setup() async throws {
        let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        defer { try? FileManager.default.removeItem(at:root) }
        let model = try fixtureModel(root)
        try await capture(PauseMenuView(model:model,tab:.crew),name:"setup",size:CGSize(width:900,height:600))
    }

    @Test func agentMemory() async throws {
        let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        defer { try? FileManager.default.removeItem(at:root) }
        let model = try fixtureModel(root)
        let agent = try #require(model.agents.first)
        try await capture(NavigationStack { AgentMemoryView(model:model,agentID:agent.agentId) }.tint(FortTheme.accent),name:"agent-memory",size:CGSize(width:640,height:720))
    }

    @Test func setupDevices() async throws {
        let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        defer { try? FileManager.default.removeItem(at:root) }
        let model = try fixtureModel(root)
        try await capture(PauseMenuView(model:model,tab:.devices),name:"setup-devices",size:CGSize(width:900,height:600))
    }
}
#endif
