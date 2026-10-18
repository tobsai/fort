import SwiftUI

struct PairingSettingsView: View {
    @Bindable var model: MessengerModel
    @State private var noExpiry = true
    @State private var days = "7"
    @State private var action: Action?
    @State private var showConfirmation = false
    @State private var historyTarget: DeviceConnection?
    private enum Action { case stop, disconnect, forget, uninstall, deleteHistory, deleteSavedHistory }
    private var validDays: Int? { guard let value = Int(days), (1...365).contains(value) else { return nil }; return value }
    private var title: String {
        switch action {
        case .stop: "Stop local work?"
        case .disconnect: "Disconnect this device?"
        case .forget: "Forget only this device’s credential?"
        case .uninstall: "Prepare to remove Fort?"
        case .deleteHistory: "Also delete local history?"
        case .deleteSavedHistory: "Delete saved history on this device?"
        case nil: "Confirm"
        }
    }
    private var explanation: String {
        switch action {
        case .stop: "Stops the connected runtime and pauses new work. Pairing, saved conversations and owner controls remain available. Resume requires an explicit action."
        case .disconnect: "Stops the connected runtime, revokes this pairing and removes its Keychain token. Saved conversations are kept."
        case .forget: "Removes the credential from this device. It does not confirm that the service revoked access or stopped; local host recovery is still required. History is kept."
        case .uninstall: "Stops the owned host, unregisters startup and removes pairing. Conversation history is kept. Then remove the Fort app using TestFlight or macOS."
        case .deleteHistory: "Stops the owned host, unregisters startup and removes pairing and local conversation history. History deletion cannot be undone. Then remove the Fort app using TestFlight or macOS."
        case .deleteSavedHistory: "Deletes saved conversations, pending submissions and local image previews for installation \(historyTarget?.installationID ?? "") on this device. The host ledger, settings, provider history, other devices and recovery records are kept. This cannot be undone."
        case nil: ""
        }
    }
    private func confirm(_ selected: Action) {
        historyTarget = selected == .deleteSavedHistory ? model.savedHistoryDeletionTarget : nil
        if selected == .deleteSavedHistory && historyTarget == nil { return }
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
                    if model.runtimeReadiness?.state == "stopped" {
                        Button("Resume local work") { Task { await model.restartRuntime() } }.disabled(!model.canResumeLocalWork)
                    } else {
                        Button("Stop local work") { confirm(.stop) }.disabled(model.isBusy || model.lifecycleBusy)
                    }
                    Button("Disconnect this device",role:.destructive) { confirm(.disconnect) }.disabled(model.isBusy || model.lifecycleBusy)
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
                Button("Prepare removal and delete local history…",role:.destructive) { confirm(.deleteHistory) }
                    .disabled(!model.canDeleteLocalHistory || model.isBusy || model.lifecycleBusy)
                if model.removableAppURL != nil && !model.canDeleteLocalHistory { Text("This installation keeps history during removal.").font(.caption).foregroundStyle(.secondary) }
                if model.removableAppURL == nil { Text("Removal preparation is available after this Mac’s owned host is installed. This development build cannot yet verify its shutdown and startup unregistration.").font(.caption).foregroundStyle(.secondary) }
                if let ready = model.preparedAppRemoval { Text("Ready for system removal: \(ready.appURL.path)").font(.caption) }
                Text("History is kept by default. Deleting it requires a separate confirmation.").font(.caption)
            }
            #endif
        }.formStyle(.grouped).navigationTitle("Pairing and local host")
        .task { await model.refreshPairing(); noExpiry = model.pairingStatus?.expiresAt == nil }
        .alert(title,isPresented:$showConfirmation) {
            Button("Cancel",role:.cancel) {}
            Button("Confirm",role:.destructive) {
                let selected = action
                Task {
                    switch selected {
                    case .stop: await model.stopLocalHost(disconnect:false)
                    case .disconnect: await model.stopLocalHost(disconnect:true)
                    case .forget: model.forgetLocalPairing()
                    case .uninstall: await model.prepareForRemoval()
                    case .deleteHistory: await model.prepareForRemoval(history:.deleteConfirmed)
                    case .deleteSavedHistory:
                        if let historyTarget { model.deleteSavedHistory(historyTarget) }
                    case nil: break
                    }
                }
            }
        } message: { Text(explanation) }
    }
}
