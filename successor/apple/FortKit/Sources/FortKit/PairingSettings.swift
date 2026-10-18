import SwiftUI

struct PairingSettingsView: View {
    @Bindable var model: MessengerModel
    @State private var noExpiry = true
    @State private var days = "7"
    @State private var action: Action?
    @State private var showConfirmation = false
    @State private var historyTarget: DeviceConnection?
    #if os(macOS)
    @State private var hostHistoryTarget: MacHistoryDeletionRequest?
    #endif
    private enum Action { case stop, disconnect, forget, uninstall, deleteHistory, deleteSavedHistory }
    private var validDays: Int? { guard let value = Int(days), (1...365).contains(value) else { return nil }; return value }
    private var title: String {
        switch action {
        case .stop: "Stop local work?"
        case .disconnect: model.isCompanionPairing ? "Disconnect this phone?" : "Disconnect host and devices?"
        case .forget: "Forget only this device’s credential?"
        case .uninstall: "Prepare to remove Fort?"
        case .deleteHistory: "Delete retained host history?"
        case .deleteSavedHistory: "Delete saved history on this device?"
        case nil: "Confirm"
        }
    }
    private var explanation: String {
        switch action {
        case .stop: "Stops the connected runtime and pauses new work. Pairing, saved conversations and owner controls remain available. Resume requires an explicit action."
        case .disconnect: model.isCompanionPairing ? "Revokes this phone’s access and removes its Keychain token. The Mac host, accepted work and saved conversations are kept." : "Stops the connected runtime, revokes this Mac and any paired phone, and removes this Mac’s Keychain token. Saved conversations are kept."
        case .forget: model.isCompanionPairing ? "Removes only this phone’s credential. Revocation remains unconfirmed; revoke the phone on its owning Mac. History is kept." : "Removes the credential from this device. It does not confirm that the service revoked access or stopped; local host recovery is still required. History is kept."
        case .uninstall: "Stops the owned host, unregisters startup and removes pairing. Conversation history is kept. Then remove the Fort app using TestFlight or macOS."
        case .deleteHistory:
            #if os(macOS)
            "Deletes retained host conversations and this Mac's saved copy for installation \(hostHistoryTarget?.installationID ?? ""). Owner notes, settings, other devices and provider history are kept. This cannot be undone."
            #else
            "Host history deletion is available on its owning Mac."
            #endif
        case .deleteSavedHistory: "Deletes saved conversations, pending submissions and local image previews for installation \(historyTarget?.installationID ?? "") on this device. The host ledger, settings, provider history, other devices and recovery records are kept. This cannot be undone."
        case nil: ""
        }
    }
    private func confirm(_ selected: Action) {
        historyTarget = selected == .deleteSavedHistory ? model.savedHistoryDeletionTarget : nil
        if selected == .deleteSavedHistory && historyTarget == nil { return }
        #if os(macOS)
        hostHistoryTarget = selected == .deleteHistory ? model.retainedHostHistoryDeletionTarget : nil
        if selected == .deleteHistory && hostHistoryTarget == nil { return }
        #endif
        action = selected; showConfirmation = true
    }
    var body: some View {
        Form {
            #if os(macOS)
            if model.hasOwnerSetup && (model.pendingOwnerSetup != nil || model.showConnection) { OwnerSetupView(model:model) }
            #endif
            Section("Pairing") {
                if let pairing = model.pairingStatus {
                    Text(pairing.expiresAt.map { "Expires: \($0)" } ?? "No expiry · stays paired until you disconnect")
                } else { Text("New owner pairings have no default expiry.") }
                Text(model.pairingMessage).font(.caption).foregroundStyle(.secondary)
                if model.canManagePairing {
                    Toggle("No expiry",isOn:$noExpiry)
                    if !noExpiry { TextField("Days from now (1–365)",text:$days) }
                    Button("Save expiry") { Task { await model.setPairingExpiry(days:noExpiry ? nil : validDays) } }
                        .disabled(!noExpiry && validDays == nil || model.lifecycleBusy)
                    if model.canControlHost {
                        if model.runtimeReadiness?.state == "stopped" {
                            Button("Resume local work") { Task { await model.restartRuntime() } }.disabled(!model.canResumeLocalWork)
                        } else {
                            Button("Stop local work") { confirm(.stop) }.disabled(model.isBusy || model.lifecycleBusy)
                        }
                    }
                    Button(model.isCompanionPairing ? "Disconnect this phone" : "Disconnect host and devices",role:.destructive) { confirm(.disconnect) }
                        .disabled(!model.canDisconnectPairing || model.isBusy || model.lifecycleBusy)
                    if model.isCompanionPairing { Text("Host stop, resume and removal are managed on the owning Mac.").font(.caption) }
                } else if !model.pairingCleanupPending {
                    Text("Temporary demos keep their original deadline. Owner setup is required before these controls become available.").font(.caption).foregroundStyle(.secondary)
                }
                if model.pairingRemovalPending {
                    Button("Retry disconnect recovery") { Task { await model.retryPairingRemoval() } }.disabled(model.lifecycleBusy)
                    if !model.localPairingForgotten { Button("Forget local credential only…",role:.destructive) { confirm(.forget) }.disabled(model.lifecycleBusy) }
                }
                if model.pairingCleanupPending { Button("Finish removing local pairing") { model.finishPairingRemoval() } }
                Text("Stopping or disconnecting keeps your saved conversations and local image previews.").font(.caption)
            }
            Section("Saved history on this device") {
                Button("Delete saved history on this device…",role:.destructive) { confirm(.deleteSavedHistory) }
                    .disabled(!model.canDeleteSavedHistory)
                    .accessibilityIdentifier("delete-saved-history")
                Text("Disconnect and finish removing this device’s pairing first. This clears the saved copy on this device; the host ledger is kept.").font(.caption)
            }
            #if os(macOS)
            Section("Remove Fort") {
                Button("Prepare to remove Fort…",role:.destructive) { confirm(.uninstall) }
                    .disabled(model.removableAppURL == nil || model.isBusy || model.lifecycleBusy)
                Button("Delete retained host history…",role:.destructive) { confirm(.deleteHistory) }
                    .disabled(!model.canDeleteRetainedHostHistory)
                    .accessibilityIdentifier("delete-retained-host-history")
                if model.removableAppURL == nil { Text("Removal preparation is available after this Mac’s owned host is installed. This development build cannot yet verify its shutdown and startup unregistration.").font(.caption).foregroundStyle(.secondary) }
                if let ready = model.preparedAppRemoval { Text("Ready for system removal: \(ready.appURL.path)").font(.caption) }
                Text("History is kept by default. To delete it, pause all schedules before preparing removal, finish pairing cleanup, then confirm deletion separately.").font(.caption)
            }
            #endif
        }.formStyle(.grouped).navigationTitle(model.isCompanionPairing ? "Phone pairing" : "Pairing and local host")
        .task { await model.refreshPairing(); noExpiry = model.pairingStatus?.expiresAt == nil }
        .alert(title,isPresented:$showConfirmation) {
            Button("Cancel",role:.cancel) {}
            Button("Confirm",role:.destructive) {
                let selected = action
                let savedHistoryTarget = historyTarget
                #if os(macOS)
                let retainedHistoryTarget = hostHistoryTarget
                #endif
                Task {
                    switch selected {
                    case .stop: await model.stopLocalHost(disconnect:false)
                    case .disconnect: await model.stopLocalHost(disconnect:true)
                    case .forget: model.forgetLocalPairing()
                    case .uninstall: await model.prepareForRemoval()
                    case .deleteHistory:
                        #if os(macOS)
                        if let retainedHistoryTarget { await model.deleteRetainedHostHistory(retainedHistoryTarget) }
                        #endif
                    case .deleteSavedHistory:
                        if let savedHistoryTarget { model.deleteSavedHistory(savedHistoryTarget) }
                    case nil: break
                    }
                }
            }
        } message: { Text(explanation) }
    }
}
