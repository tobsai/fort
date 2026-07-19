//
//  FortMacApp.swift
//  FortMac
//
//  Fort's macOS surface: a menu-bar app built on `MenuBarExtra` PLUS a full
//  window (spec 032). It holds the shared `FortClient` (from ../FortKit) at the
//  app root, polls the control plane for a `Summary`, and renders the glanceable
//  counts + gate inbox in the menu. The menu-bar title/badge shows the
//  pending-gate count.
//
//  The `Window("Fort")` scene hosts `FortWindow` — a native mirror of the 031
//  dashboard plus `fort service` daemon controls (via `ServiceController`).
//

import SwiftUI
import FortKit

@main
struct FortMacApp: App {
    /// The single control-plane client, shared with `MenuContent` and
    /// `FortWindow` via the environment. `FortClient` is an `ObservableObject`;
    /// hold it as `@StateObject` so it lives for the app's lifetime.
    @StateObject private var client = FortClient() // http://127.0.0.1:4087

    /// The latest control-plane snapshot, refreshed on a timer by `MenuContent`.
    /// Kept at the app root so the menu-bar label can badge the gate count even
    /// while the menu is closed.
    @StateObject private var model = MenuModel()

    /// Drives the `fort service` launchd daemon (install/start/stop/restart).
    /// Bound to the window's sidebar "Service" controls.
    @StateObject private var service = ServiceController(fortBinaryURL: FortMacApp.bundledFort())

    var body: some Scene {
        // Window FIRST so it opens on launch — SwiftUI treats the first scene as
        // primary. (When MenuBarExtra was first, the app launched with no window,
        // so double-clicking the app "did nothing".) The menu-bar item still
        // appears via the MenuBarExtra scene below.
        Window("Fort", id: "main") {
            FortWindow()
                .environmentObject(client)
                .environmentObject(service)
                .frame(minWidth: 720, minHeight: 480)
        }

        MenuBarExtra {
            MenuContent()
                .environmentObject(client)
                .environmentObject(model)
        } label: {
            // Icon plus a pending-gate badge. `MenuBarExtra` renders the label
            // in the status bar; SF Symbols keep it crisp at menu-bar size.
            Label {
                Text(model.badgeText)
            } icon: {
                Image(systemName: model.pendingGates > 0
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
}

/// Observable state the menu-bar label binds to. Owned at the app root so the
/// badge stays live independent of whether the menu is open.
@MainActor
final class MenuModel: ObservableObject {
    @Published var summary: Summary?
    @Published var lastError: String?

    /// A transient, non-fatal notice (e.g. control-only mode on a gate action).
    /// Cleared automatically after a short delay.
    @Published var notice: String?

    var pendingGates: Int { summary?.gates.count ?? 0 }

    /// Text shown next to the menu-bar icon. Empty when there is nothing
    /// pending, so the icon stands alone.
    var badgeText: String {
        pendingGates > 0 ? "\(pendingGates)" : ""
    }

    /// Whether an execution plane is attached. `false` == control-only: chat
    /// only boards a queued task and gate decisions return HTTP 409.
    var isControlOnly: Bool {
        guard let summary else { return false }
        return !summary.execution
    }

    private var noticeClearTask: Task<Void, Never>?

    /// Shows a transient notice, replacing any prior one, and clears it after
    /// `seconds`.
    func flash(_ message: String, seconds: Double = 4) {
        notice = message
        noticeClearTask?.cancel()
        noticeClearTask = Task { [weak self] in
            try? await Task.sleep(nanoseconds: UInt64(seconds * 1_000_000_000))
            guard !Task.isCancelled else { return }
            self?.notice = nil
        }
    }
}
