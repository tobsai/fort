#if os(macOS)
import AppKit
import Foundation
import SwiftUI
import Testing
@testable import FortKit

// Side-by-side screenshots, Mac half: the Quest log, a chat and Setup drawn from design/screens/fixture.json,
// the same records the web half (design/screens/web-screens.mjs) serves to the web client. Runs only when
// FORT_SCREENSHOTS names a folder, as the native CI job does, and writes one PNG per screen and mode.

private struct ScreenFixture: Decodable {
    let installation: Installation
    let agents: [Agent]
    let conversations: [Conversation]
    let snapshots: [String: Snapshot]
    let open: String
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
        try await capture(QuestLogView(model:model,openChat:{ _ in },showCity:nil,openMenu:{}),name:"quest-log",size:CGSize(width:1280,height:800))
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

    @Test func setup() async throws {
        let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        defer { try? FileManager.default.removeItem(at:root) }
        let model = try fixtureModel(root)
        try await capture(PauseMenuView(model:model,tab:.crew),name:"setup",size:CGSize(width:900,height:600))
    }
}
#endif
