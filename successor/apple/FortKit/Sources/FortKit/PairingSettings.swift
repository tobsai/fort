import SwiftUI
#if os(macOS)
import AppKit
#endif

struct PairingSettingsView: View {
    @Bindable var model: MessengerModel
    @State private var noExpiry = true
    @State private var days = "7"
    @State private var action: Action?
    @State private var showConfirmation = false
    @State private var historyTarget: DeviceConnection?
    @State private var showWorkshop = false
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
        case .uninstall: "Stops the owned host, unregisters startup, removes pairing and deletes Fort’s saved provider sign-in from this Mac. Conversation history, provider history and provider-side grants are kept. Then remove the Fort app using TestFlight or macOS."
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
            DevicesBoardSection(model:model,showWorkshop:$showWorkshop)
            Section {
                Button { showWorkshop.toggle() } label: {
                    Label("WORKSHOP",systemImage:showWorkshop ? "chevron.down" : "chevron.right").font(.caption.weight(.heavy)).foregroundStyle(.secondary)
                }.buttonStyle(.plain).accessibilityValue(showWorkshop ? "Expanded" : "Collapsed").accessibilityIdentifier("devices-workshop")
            } footer: { Text("Setup, pairing, certificates, stopping, disconnecting and removal.").font(.caption) }
            if showWorkshop {
            ConnectionSections(model:model)
            #if os(macOS)
            if model.canManagePhoneAccess { PhoneAccessView(model:model) }
            #endif
            Section("Pairing") {
                if let pairing = model.pairingStatus {
                    Text(pairing.expiresAt.map { "Expires: \($0)" } ?? "No expiry · stays paired until you disconnect")
                } else { Text(model.isCompanionPairing ? "This phone’s access is currently unconfirmed." : "New owner pairings have no default expiry.") }
                CopyableText(model.pairingMessage).font(.caption).foregroundStyle(.secondary)
                if model.canManagePairing {
                    if model.pairingStatus != nil {
                        Toggle("No expiry",isOn:$noExpiry).disabled(!model.canEditPairingExpiry)
                        if !noExpiry { TextField("Days from now (1–365)",text:$days).disabled(!model.canEditPairingExpiry) }
                        Button("Save expiry") { Task { await model.setPairingExpiry(days:noExpiry ? nil : validDays) } }
                            .disabled(!noExpiry && validDays == nil || !model.canEditPairingExpiry)
                    }
                    if model.canControlHost {
                        if model.runtimeReadiness?.state == "stopped" {
                            Button("Resume local work") { Task { await model.restartRuntime() } }.disabled(!model.canResumeLocalWork)
                        } else {
                            Button("Stop local work") { confirm(.stop) }.disabled(model.isBusy || model.lifecycleBusy)
                        }
                    }
                } else if !model.pairingCleanupPending && !model.isCompanionPairing {
                    Text("Temporary demos keep their original deadline. Owner setup is required before these controls become available.").font(.caption).foregroundStyle(.secondary)
                }
                if model.canDisconnectPairing {
                    Button(model.isCompanionPairing ? "Disconnect this phone" : "Disconnect host and devices",role:.destructive) { confirm(.disconnect) }
                        .disabled(model.isBusy || model.lifecycleBusy).accessibilityIdentifier("pairing-disconnect")
                }
                if model.isCompanionPairing { Text("Host stop, resume and removal are managed on the owning Mac.").font(.caption) }
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
            if !model.isCompanionPairing { Section("Remove Fort") {
                Button("Prepare to remove Fort…",role:.destructive) { confirm(.uninstall) }
                    .disabled(model.removableAppURL == nil || model.isBusy || model.lifecycleBusy)
                Button("Delete retained host history…",role:.destructive) { confirm(.deleteHistory) }
                    .disabled(!model.canDeleteRetainedHostHistory)
                    .accessibilityIdentifier("delete-retained-host-history")
                if model.removableAppURL == nil { Text("Removal preparation is available after this Mac’s owned host is installed. This development build cannot yet verify its shutdown and startup unregistration.").font(.caption).foregroundStyle(.secondary) }
                if let ready = model.preparedAppRemoval { Text("Ready for system removal: \(ready.appURL.path)").font(.caption) }
                Text("History is kept by default. To delete it, pause all schedules before preparing removal, finish pairing cleanup, then confirm deletion separately.").font(.caption)
            }
            }
            #endif
            }
        }.formStyle(.grouped).navigationTitle("Devices")
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
        #if os(macOS)
        .background(NonBlockingSheetWindow().allowsHitTesting(false).accessibilityHidden(true))
        #endif
    }
}

/// The top of Devices: each device's light, sentence and next step, from `DevicesBoard`.
struct DevicesBoardSection: View {
    @Bindable var model: MessengerModel
    @SwiftUI.Binding var showWorkshop: Bool
    var body: some View {
        Section {
            ForEach(DevicesBoard.make(model.devicesFacts).rows) { row in
                VStack(alignment:.leading,spacing:8) {
                    HStack(alignment:.top,spacing:12) {
                        LampView(lamp:row.lamp).padding(.top,5)
                        VStack(alignment:.leading,spacing:3) {
                            Text(row.title.uppercased()).font(.system(.headline,design:.rounded).weight(.black))
                            Text(row.line).font(.callout).foregroundStyle(.secondary).fixedSize(horizontal:false,vertical:true)
                        }
                        Spacer(minLength:8)
                        if let action = row.action { button(action,row:row) }
                    }
                    if row.id == "phone" { invitation }
                }
                .padding(.vertical,4)
                .accessibilityElement(children:.contain).accessibilityIdentifier("devices-row-\(row.id)")
            }
        }
    }
    @ViewBuilder private func button(_ action: DevicesBoard.Action, row: DevicesBoard.Row) -> some View {
        switch action {
        case .resume:
            Button("Start crew") { Task { await model.restartRuntime() } }.buttonStyle(.borderedProminent).tint(HUDStyle.gold).foregroundStyle(.black)
                .disabled(!model.canResumeLocalWork).accessibilityIdentifier("devices-resume")
        case .restart:
            Button("Restart crew") { Task { await model.restartRuntime() } }.buttonStyle(.borderedProminent).tint(HUDStyle.gold).foregroundStyle(.black)
                .disabled(model.runtimeReadiness?.canRestart != true || model.isBusy || (model.installation?.supports("ownerPairing") == true && !model.canControlHost))
                .accessibilityIdentifier("devices-restart")
        case .openWorkshop:
            Button("Show me") { showWorkshop = true }.buttonStyle(.bordered).accessibilityIdentifier("devices-open-workshop-\(row.id)")
        default:
            macButton(action,row:row)
        }
    }
    // Host setup and phone access exist only on the owning Mac.
    @ViewBuilder private func macButton(_ action: DevicesBoard.Action, row: DevicesBoard.Row) -> some View {
        #if os(macOS)
        let phoneBusy = model.phoneAccessBusy || model.lifecycleBusy || model.isBusy
        switch action {
        case .enableHost:
            Button("Turn on") { Task { await model.enableLocalHost() } }.buttonStyle(.borderedProminent).tint(HUDStyle.gold).foregroundStyle(.black)
                .disabled(!model.canEnableLocalHost).accessibilityIdentifier("devices-enable-host")
        case .connect:
            Button("Connect this Mac") { Task { await model.completeOwnerSetup() } }.buttonStyle(.borderedProminent).tint(HUDStyle.gold).foregroundStyle(.black)
                .disabled(!model.canCompleteOwnerSetup).accessibilityIdentifier("devices-connect")
        case .startListener:
            Button("Turn on") { Task { await model.startPhoneListener() } }.buttonStyle(.bordered)
                .disabled(phoneBusy || model.phoneServerName == nil).accessibilityIdentifier("devices-phone-listener")
        case .invite:
            Button(row.lamp == .waiting ? "New invitation" : "Invite iPhone") { Task { await model.invitePhone() } }.buttonStyle(.borderedProminent).tint(HUDStyle.gold).foregroundStyle(.black)
                .disabled(phoneBusy || !model.phoneListenerActive).accessibilityIdentifier("devices-phone-invite")
        default:
            EmptyView()
        }
        #endif
    }
    @ViewBuilder private var invitation: some View {
        #if os(macOS)
        if let invitation = model.phoneInvitation {
            VStack(alignment:.leading,spacing:6) {
                Text(invitation.text).font(.caption.monospaced()).textSelection(.enabled).lineLimit(3)
                    .padding(8).frame(maxWidth:.infinity,alignment:.leading)
                    .overlay(RoundedRectangle(cornerRadius:6).strokeBorder(HUDStyle.gold,style:StrokeStyle(lineWidth:1.5,dash:[6,4])))
                HStack {
                    Button("Copy invitation") { NSPasteboard.general.clearContents(); NSPasteboard.general.setString(invitation.text,forType:.string) }
                        .accessibilityIdentifier("devices-phone-copy")
                    Text("Paste it into Fort on your iPhone. It works once.").font(.caption).foregroundStyle(.secondary)
                }
            }
        }
        #endif
    }
}

extension MessengerModel {
    var devicesFacts: DevicesBoard.Facts {
        var facts = DevicesBoard.Facts()
        #if os(macOS)
        facts.isCompanion = isCompanionPairing
        if let host = localHost {
            switch host.status {
            case .unknown: facts.hostSetup = .unknown
            case .notRegistered,.notFound: facts.hostSetup = .needsEnable
            case .requiresApproval: facts.hostSetup = .needsApproval
            case .enabled: facts.hostSetup = .enabled
            case .unsupported: facts.hostSetup = .unsupported
            }
        }
        facts.managesPhone = canManagePhoneAccess
        facts.phoneAddress = phoneServerName
        facts.listenerActive = phoneListenerActive
        facts.phone = phonePairing
        #else
        facts.isCompanion = true
        #endif
        facts.connected = connected && installation != nil
        facts.runtimeState = runtimeReadiness?.state
        facts.claudeServing = installation?.runtimes?.contains(where:{ $0.provider == "claude-code" }) == true
        return facts
    }
}
