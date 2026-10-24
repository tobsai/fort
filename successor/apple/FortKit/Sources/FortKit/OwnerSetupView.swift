#if os(macOS)
import SwiftUI

struct MacHostControlView: View {
    @Bindable var model: MessengerModel
    @Bindable var host: MacHostControl
    @State private var removeUnpairedStartup = false
    @State private var providerMessage = "ChatGPT is always on. Turn on Claude to add Claude Agents beside your ChatGPT ones."
    @State private var startFailure: String?
    private func refreshStartFailure() { startFailure = (try? host.lastStartFailure()) ?? nil }
    // The host ends itself when the choice changes (Spec 064), and the app's next request starts a fresh one with the new
    // choice. Refresh until the host reports it, or reports why Claude couldn't start, for up to about 30 seconds.
    private func reconnectAfterRestart(claude: Bool) {
        Task {
            for _ in 0..<15 {
                try? await Task.sleep(for:.seconds(2))
                await model.refresh(); refreshStartFailure()
                let serving = model.installation?.runtimes?.contains(where:{ $0.provider == "claude-code" }) == true
                if model.connected && (serving == claude || startFailure != nil) { await model.refreshModels(); return }
            }
        }
    }
    var body: some View {
        Section("Local host") {
            Text(host.message).font(.callout).accessibilityIdentifier("local-host-status")
            if host.isBusy { ProgressView("Checking the local host…") }
            if model.hasCompletedHostRemoval {
                Button("Set up Fort again") { Task { await model.prepareLocalReinstall() } }
                    .disabled(!model.canPrepareLocalReinstall).accessibilityIdentifier("local-host-prepare-reinstall")
                Text("Keeps saved conversations. You’ll enable the local host and connect this Mac separately afterward.").font(.caption)
            }
            if host.canEnable {
                Button("Enable local host") { Task { await model.enableLocalHost() } }
                    .disabled(!model.canEnableLocalHost).accessibilityIdentifier("local-host-enable")
            }
            if model.canDisableUnpairedLocalHost {
                Button("Remove unused startup registration…") { removeUnpairedStartup = true }
                    .accessibilityIdentifier("local-host-remove-unpaired")
            }
            if let startFailure {
                Text("The local host reported a problem at its last start: \(startFailure)").font(.callout).foregroundStyle(.red).textSelection(.enabled)
                    .accessibilityIdentifier("local-host-start-failure")
            }
            Button("Check host status") { Task { await model.inspectLocalHost(); refreshStartFailure() } }
                .disabled(!model.canInspectLocalHost).accessibilityIdentifier("local-host-inspect")
            Text("Enabling lets macOS start Fort’s local host. Pairing and provider access are checked separately.").font(.caption)
            // Specs 062 and 064: runs your installed Claude Code read-only beside ChatGPT, signed in through Claude's own login.
            Button("Turn on Claude") {
                do { try host.selectClaudeCode(); providerMessage = "Turning on Claude. The local host restarts by itself in a few seconds. Then sign in to Claude under Settings, Pairing and local host, and pick Claude when you create an Agent."; reconnectAfterRestart(claude:true) }
                catch { providerMessage = "Claude could not be turned on: \(error.localizedDescription)" }
            }.disabled(host.isBusy).accessibilityIdentifier("local-host-use-claude")
            Button("Turn off Claude") {
                do { try host.removeClaudeCode(); providerMessage = "Turning off Claude. The local host restarts by itself in a few seconds. Claude Agents stay saved and won’t run until you turn it on again."; reconnectAfterRestart(claude:false) }
                catch { providerMessage = "Claude could not be turned off: \(error.localizedDescription)" }
            }.disabled(host.isBusy).accessibilityIdentifier("local-host-use-chatgpt")
            Text(providerMessage).font(.caption).accessibilityIdentifier("local-host-provider")
        }.task { refreshStartFailure() }
        .alert("Remove unused startup registration?",isPresented:$removeUnpairedStartup) {
            Button("Cancel",role:.cancel) {}
            Button("Remove registration",role:.destructive) { Task { await model.disableUnpairedLocalHost() } }
        } message: {
            Text("Removes this app’s startup registration only if no pairing or host data exists. Saved files and credentials are kept.")
        }
    }
}

struct OwnerSetupView: View {
    @Bindable var model: MessengerModel
    @State private var cancellation: OwnerSetupIdentity?
    private var connectTitle: String {
        switch model.pendingOwnerSetup?.phase {
        case .preparing: "Continue setup"
        case .ready: "Retry setup"
        default: "Connect this Mac"
        }
    }
    var body: some View {
        Section("This Mac") {
            Text("Pair this Mac with its configured Fort host. The device credential stays in Keychain. Pairing has no default expiry; you can disconnect later.")
            Text(model.ownerSetupMessage).font(.callout).accessibilityIdentifier("owner-setup-status")
            if model.ownerSetupBusy { ProgressView("Finishing the requested setup action…") }
            if model.pendingOwnerSetup?.phase != .canceling && model.pendingOwnerSetup?.phase != .canceled {
                Button(connectTitle) { Task { await model.completeOwnerSetup() } }
                    .disabled(!model.canCompleteOwnerSetup).accessibilityIdentifier("owner-setup-connect")
            }
            if let pending = model.pendingOwnerSetup {
                switch pending.phase {
                case .preparing,.ready:
                    Button("Cancel setup…",role:.destructive) { cancellation = pending.identity }
                        .disabled(!model.canCancelOwnerSetup).accessibilityIdentifier("owner-setup-cancel")
                case .canceling,.canceled:
                    Button(pending.phase == .canceled ? "Finish local cleanup" : "Retry cancellation") {
                        Task { await model.cancelOwnerSetup(pending.identity) }
                    }.disabled(!model.canCancelOwnerSetup).accessibilityIdentifier("owner-setup-recover")
                }
            }
            Button("Check saved setup") { model.inspectOwnerSetup() }
                .disabled(model.ownerSetupBusy).accessibilityIdentifier("owner-setup-inspect")
        }
        .confirmationDialog("Cancel this Mac’s saved setup?",isPresented:SwiftUI.Binding(get:{ cancellation != nil },set:{ if !$0 { cancellation = nil } }),presenting:cancellation) { expected in
            Button("Cancel saved setup",role:.destructive) { Task { await model.cancelOwnerSetup(expected) } }
            Button("Keep setup",role:.cancel) {}
        } message: { _ in
            Text("If pairing already completed, this stops its runtime and revokes this device’s access before removing the prepared credential. Saved conversations are kept.")
        }
        .task { model.inspectOwnerSetup() }
    }
}
#endif
