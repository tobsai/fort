//
//  FortMacApp.swift
//  FortMac
//
//  Fort's macOS surface: one closed Agent/Primary chat window plus the matching
//  glanceable menu-bar inbox. Settings retains daemon controls via the existing
//  ServiceController.
//

import SwiftUI
import FortKit

@main
struct FortMacApp: App {
    /// The single control-plane client, shared with `MenuContent` and
    /// the shared Channels view via the environment. `FortClient` is an `ObservableObject`;
    /// hold it as `@StateObject` so it lives for the app's lifetime.
    @StateObject private var client = FortMacApp.makeClient()

    /// The latest bounded mode-matched glance, refreshed by `MenuContent`.
    /// Kept at the app root so the menu-bar label can badge Needs You even while
    /// the menu is closed.
    @StateObject private var model = MenuModel()

    /// Drives bounded launchd status and Install/Start/Restart recovery from
    /// Primary Settings. Administrative teardown remains CLI-only.
    @StateObject private var service = ServiceController(fortBinaryURL: FortMacApp.bundledFort())
    @State private var mainWindowVisible = false
    private let presentationMode = AgentChannelsPresentationMode.configured

    var body: some Scene {
        // Window FIRST so it opens on launch — SwiftUI treats the first scene as
        // primary. (When MenuBarExtra was first, the app launched with no window,
        // so double-clicking the app "did nothing".) The menu-bar item still
        // appears via the MenuBarExtra scene below.
        Window("Fort", id: "main") {
            FortMarkSurface {
                FortNativeChatView(mode: presentationMode)
            }
                .fortMarkWindowVisible(mainWindowVisible)
                .background {
                    FortWindowVisibilityObserver(isVisible: $mainWindowVisible)
                        .frame(width: 0, height: 0)
                }
                .primaryServiceController(service)
                .environmentObject(client)
                .environmentObject(service)
                .frame(minWidth: 760, minHeight: 520)
        }
        .defaultSize(width: 1200, height: 800)

        MenuBarExtra {
            MenuContent(mode: presentationMode)
                .environmentObject(client)
                .environmentObject(model)
        } label: {
            // Icon plus a Needs You badge. `MenuBarExtra` renders the label
            // in the status bar; SF Symbols keep it crisp at menu-bar size.
            Label {
                Text(model.badgeText)
            } icon: {
                Image(systemName: model.pendingNeedsYou > 0
                      ? "shield.lefthalf.filled"
                      : "shield")
            }
        }
        .menuBarExtraStyle(.window) // richer content (fields, buttons) than .menu
    }

    /// The `fort` binary the `ServiceController` shells out to: the copy bundled
    /// in the app's `Contents/Resources` (see `make mac-dmg`), else a Homebrew
    /// fallback so a dev build off the menu-bar app still works.
    static func bundledFort() -> URL {
        Bundle.main.url(forResource: "fort", withExtension: nil)
            ?? URL(fileURLWithPath: "/opt/homebrew/bin/fort")
    }

    /// DEBUG builds may point only the native client at an isolated QA fixture.
    /// Release builds compile this environment seam out and retain FortClient's
    /// production localhost default. This never installs or restarts launchd.
    private static func makeClient() -> FortClient {
        #if DEBUG
        if let configured = ProcessInfo.processInfo.environment["FORT_DIRECT_HOST_URL"],
           let directURL = URL(string: configured),
           ["http", "https"].contains(directURL.scheme?.lowercased() ?? ""),
           directURL.host != nil {
            return FortClient(baseURL: directURL)
        }
        #endif
        return FortClient()
    }
}

/// Observable state the menu-bar label binds to. Owned at the app root so the
/// badge stays live independent of whether the menu is open.
@MainActor
final class MenuModel: ObservableObject {
    @Published var needsYou: [PrimaryNeedsYouItem] = []
    @Published var channels: [PrimaryChannelSummary] = []
    @Published var agentNeedsYou: [AgentNeedsYouItem] = []
    @Published var agentChannels: [AgentChannelSummary] = []
    @Published private(set) var activeNeedsYouCount = 0
    @Published var lastError: String?

    var pendingNeedsYou: Int { activeNeedsYouCount }

    /// Text shown next to the menu-bar icon. Empty when there is nothing
    /// pending, so the icon stands alone.
    var badgeText: String {
        pendingNeedsYou > 0 ? "\(pendingNeedsYou)" : ""
    }

    func acceptPrimary(
        needsYou nextNeedsYou: [PrimaryNeedsYouItem],
        channels nextChannels: [PrimaryChannelSummary]
    ) {
        needsYou = nextNeedsYou
        channels = nextChannels
        agentNeedsYou = []
        agentChannels = []
        activeNeedsYouCount = nextNeedsYou.count
    }

    func acceptAgentChannels(
        needsYou nextNeedsYou: [AgentNeedsYouItem],
        channels nextChannels: [AgentChannelSummary]
    ) {
        agentNeedsYou = nextNeedsYou
        agentChannels = nextChannels
        needsYou = []
        channels = []
        activeNeedsYouCount = nextNeedsYou.count
    }
}
