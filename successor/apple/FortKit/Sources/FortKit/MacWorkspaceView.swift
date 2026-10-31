#if os(macOS)
import SwiftUI

/// Spec 063 (revision 2026-10-31): once this Mac hands its Agents to the cloud hub, the window opens on the hub's workspace,
/// and the Mac is one more way to reach them. "This Mac" keeps the Mac's own host: its settings, account sign-ins, the
/// Agents that stay here, and the read-only history of the ones that moved.
public struct MacWorkspaceView: View {
    @Bindable private var model: MessengerModel
    @AppStorage("fort.showThisMac") private var showThisMac = false
    public init(model: MessengerModel) { self.model = model }
    public var body: some View {
        Group {
            if let hub = model.hubWorkspace, !showThisMac { MessengerView(model:hub).id("hub") }
            else { MessengerView(model:model).id("mac") }
        }
        .safeAreaInset(edge:.top,spacing:0) {
            if model.hubWorkspace != nil {
                Picker("Workspace",selection:$showThisMac) { Text("Hub").tag(false); Text("This Mac").tag(true) }
                    .pickerStyle(.segmented).labelsHidden().fixedSize().padding(.vertical,6)
                    .accessibilityIdentifier("workspace-picker")
            }
        }
        .task {
            // The Mac's host hands Agents over in the background; check for the hub's credential until it arrives, then keep it current.
            while !Task.isCancelled {
                await model.refreshHubWorkspace()
                try? await Task.sleep(for:.seconds(15))
            }
        }
    }
}
#endif
