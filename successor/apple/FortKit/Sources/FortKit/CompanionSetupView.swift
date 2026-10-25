import SwiftUI

/// Joining from a phone: paste the invitation, then join. The exact fields and recovery steps stay in the Workshop.
struct CompanionSetupView: View {
    @Bindable var model: MessengerModel
    @State private var forgetTarget: PendingCompanionSetup?
    @State private var showForget = false
    @State private var showWorkshop = false
    private var busy: Bool { model.companionSetupBusy || model.lifecycleBusy || model.isBusy }
    var body: some View {
        let panel = JoinPanel.make(address:model.companionAddress,code:model.companionInvitationCode,hasPending:model.pendingCompanionSetup != nil)
        Section("Join your Fort") {
            switch panel {
            case .paste:
                Text("On your Mac, open Menu → Devices → Invite a phone and copy the invitation.").font(.callout)
                pasteButton(prominent:true)
            case .ready(let host):
                Text("Invitation from \(host). Join when you're ready.").font(.callout)
                HStack {
                    joinButton("Join")
                    pasteButton(prominent:false)
                }
            case .pending:
                if model.pendingCompanionSetup?.removalState == nil {
                    SecureField("Re-enter the same invitation code",text:$model.companionInvitationCode)
                        .autocorrectionDisabled().disabled(busy).accessibilityIdentifier("phone-setup-code")
                    joinButton("Retry exact invitation")
                }
            }
            CopyableText(model.companionSetupMessage,id:"phone-setup-status").font(.callout)
        }
        Section {
            WorkshopDrawer(isExpanded:$showWorkshop,forceOpen:panel == .pending,identifier:"phone-setup-workshop") {
                Group {
                    TextField("HTTPS address",text:$model.companionAddress)
                        .accessibilityIdentifier("phone-setup-address")
                    TextField("Installation ID",text:$model.companionInstallationID)
                        .accessibilityIdentifier("phone-setup-installation")
                    TextField("Invitation ID",text:$model.companionInvitationID)
                        .accessibilityIdentifier("phone-setup-invitation")
                    TextField("Phone name",text:$model.companionDeviceName)
                        .accessibilityIdentifier("phone-setup-name")
                    if panel != .pending {
                        SecureField("Invitation code",text:$model.companionInvitationCode)
                            .accessibilityIdentifier("phone-setup-code")
                        if panel == .paste { joinButton("Pair this phone") }
                    }
                }.autocorrectionDisabled().disabled(busy || model.pendingCompanionSetup != nil)
                if let pending = model.pendingCompanionSetup {
                    if pending.credentialPrepared && pending.removalState == nil {
                        Button("Recover prepared pairing") { Task { await model.recoverCompanionSetup() } }
                            .disabled(busy).accessibilityIdentifier("phone-setup-recover")
                    }
                    Button("Cancel pending phone setup",role:.destructive) { Task { await model.disconnectPendingCompanion(pending) } }
                        .disabled(busy).accessibilityIdentifier("phone-setup-cancel")
                    Button("Forget local preparation only…",role:.destructive) {
                        forgetTarget = pending; showForget = true
                    }.disabled(busy).accessibilityIdentifier("phone-setup-forget")
                    Text("Recovery checks the prepared credential. Retrying sends the same invitation request. Neither happens automatically.").font(.caption)
                }
                Text("Pairing stays active until you disconnect, unless you set an expiry. Saved conversations are kept when you disconnect.").font(.caption).foregroundStyle(.secondary)
            }
        }
        .task { model.inspectCompanionSetup() }
        .alert("Forget only this phone’s preparation?",isPresented:$showForget) {
            Button("Cancel",role:.cancel) {}
            Button("Forget local preparation",role:.destructive) {
                if let forgetTarget { model.forgetPendingCompanion(forgetTarget) }
            }
        } message: {
            Text("Removes the local credential for \(forgetTarget?.deviceName ?? "this phone"). Host revocation remains unconfirmed. Resolve the phone on its owning Mac before pairing again. Saved conversations are kept.")
        }
    }

    private func pasteButton(prominent: Bool) -> some View {
        Button(prominent ? "Paste invitation" : "Paste again") {
            #if os(iOS)
            model.applyCompanionInvitation(UIPasteboard.general.string ?? "")
            #else
            model.applyCompanionInvitation(NSPasteboard.general.string(forType:.string) ?? "")
            #endif
        }
        .modifier(GoldButton(on:prominent))
        .disabled(busy || model.pendingCompanionSetup != nil).accessibilityIdentifier("phone-setup-paste")
    }

    private func joinButton(_ title: String) -> some View {
        Button(title) { Task { await model.connectCompanion() } }
            .modifier(GoldButton(on:true))
            .disabled(!model.canSubmitCompanionInvitation).accessibilityIdentifier("phone-setup-connect")
    }
}

/// The one gold button on a panel; everything else stays plain.
struct GoldButton: ViewModifier {
    let on: Bool
    func body(content: Content) -> some View {
        if on { content.buttonStyle(.borderedProminent).tint(HUDStyle.gold).foregroundStyle(.black) } else { content.buttonStyle(.bordered) }
    }
}
