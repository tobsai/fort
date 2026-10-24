import SwiftUI

struct CompanionSetupView: View {
    @Bindable var model: MessengerModel
    @State private var forgetTarget: PendingCompanionSetup?
    @State private var showForget = false
    private var busy: Bool { model.companionSetupBusy || model.lifecycleBusy || model.isBusy }
    var body: some View {
        Section("Connect this phone") {
            Text("On your Mac, open Menu → Devices → Invite a phone, copy the invitation, then paste it here.").font(.callout)
            Button("Paste invitation") {
                #if os(iOS)
                model.applyCompanionInvitation(UIPasteboard.general.string ?? "")
                #else
                model.applyCompanionInvitation(NSPasteboard.general.string(forType:.string) ?? "")
                #endif
            }.accessibilityIdentifier("phone-setup-paste")
            TextField("HTTPS address",text:$model.companionAddress)
                .accessibilityIdentifier("phone-setup-address")
            TextField("Installation ID",text:$model.companionInstallationID)
                .accessibilityIdentifier("phone-setup-installation")
            TextField("Invitation ID",text:$model.companionInvitationID)
                .accessibilityIdentifier("phone-setup-invitation")
            TextField("Phone name",text:$model.companionDeviceName)
                .accessibilityIdentifier("phone-setup-name")
        }.autocorrectionDisabled().disabled(busy || model.pendingCompanionSetup != nil)
        Section {
            if model.pendingCompanionSetup?.removalState == nil {
                SecureField(model.pendingCompanionSetup == nil ? "Invitation code" : "Re-enter the same invitation code",text:$model.companionInvitationCode)
                    .autocorrectionDisabled().disabled(busy).accessibilityIdentifier("phone-setup-code")
                Button(model.pendingCompanionSetup == nil ? "Pair this phone" : "Retry exact invitation") {
                    Task { await model.connectCompanion() }
                }.disabled(!model.canSubmitCompanionInvitation).accessibilityIdentifier("phone-setup-connect")
            }
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
            Text(model.companionSetupMessage).font(.callout).accessibilityIdentifier("phone-setup-status")
            Text("Pairing stays active until you disconnect, unless you set an expiry. Saved conversations are kept when you disconnect.").font(.caption)
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
}
