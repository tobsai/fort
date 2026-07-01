//
//  FortMacApp.swift
//  FortMac
//
//  Fort's macOS surface: a menu-bar app built on `MenuBarExtra`. It holds the
//  shared `FortClient` (from ../FortKit) at the app root, polls the control
//  plane for a `Summary`, and renders the glanceable counts + gate inbox in the
//  menu. The menu-bar title/badge shows the pending-gate count.
//
//  There is no window: the whole app lives in the status bar.
//

import SwiftUI
import FortKit

@main
struct FortMacApp: App {
    /// The single control-plane client, shared with `MenuContent` via the
    /// environment. `FortClient` is an `ObservableObject`; hold it as
    /// `@StateObject` so it lives for the app's lifetime.
    @StateObject private var client = FortClient() // http://127.0.0.1:4087

    /// The latest control-plane snapshot, refreshed on a timer by `MenuContent`.
    /// Kept at the app root so the menu-bar label can badge the gate count even
    /// while the menu is closed.
    @StateObject private var model = MenuModel()

    var body: some Scene {
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
