#if os(macOS)
import SwiftUI

struct MacHostControlView: View {
    @Bindable var model: MessengerModel
    @Bindable var host: MacHostControl
    var body: some View {
        Section("Local host") {
            Text(host.message).font(.callout).accessibilityIdentifier("local-host-status")
            if host.isBusy { ProgressView("Checking the local host…") }
            if host.status == .notRegistered {
                Button("Enable local host") { Task { await model.enableLocalHost() } }
                    .disabled(!model.canEnableLocalHost).accessibilityIdentifier("local-host-enable")
            }
            Button("Check host status") { Task { await model.inspectLocalHost() } }
                .disabled(!model.canInspectLocalHost).accessibilityIdentifier("local-host-inspect")
            Text("Enabling lets macOS start Fort’s local host. Pairing and provider access are checked separately.").font(.caption)
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
