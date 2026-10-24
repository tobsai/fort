#if os(macOS)
import SwiftUI
import AppKit
import UniformTypeIdentifiers

struct PhoneAccessView: View {
    @Bindable var model: MessengerModel
    @State private var importing = false
    @State private var revokeTarget: String?
    @State private var showRevoke = false
    private var busy: Bool { model.phoneAccessBusy || model.lifecycleBusy || model.isBusy }
    var body: some View {
        Section("Phone access") {
            Text("Your iPhone connects to this Mac over Tailscale. Both devices need Tailscale signed in to the same tailnet.").font(.callout)
            LabeledContent("Certificate",value:model.phoneServerName ?? "Not imported")
            Button(model.phoneServerName == nil ? "Import Tailscale certificate…" : "Replace certificate…") { importing = true }
                .disabled(busy).accessibilityIdentifier("phone-access-import")
            Text("Run `tailscale cert <this Mac's tailnet name>` in Terminal, then choose both the .crt and .key files it creates.").font(.caption).foregroundStyle(.secondary)
            LabeledContent("Listener",value:listenerText)
            if model.phoneListenerActive {
                Button("Stop phone listener") { Task { await model.stopPhoneListener() } }.disabled(busy).accessibilityIdentifier("phone-access-stop")
            } else {
                Button("Start phone listener") { Task { await model.startPhoneListener() } }
                    .disabled(busy || model.phoneServerName == nil).accessibilityIdentifier("phone-access-start")
            }
            pairingRows
            if let invitation = model.phoneInvitation {
                Text(invitation.text).font(.caption.monospaced()).textSelection(.enabled).lineLimit(3).accessibilityIdentifier("phone-access-invitation")
                Button("Copy invitation") {
                    NSPasteboard.general.clearContents(); NSPasteboard.general.setString(invitation.text,forType:.string)
                }.accessibilityIdentifier("phone-access-copy")
                Text("With Universal Clipboard you can paste it straight into Fort on your iPhone. It works once.").font(.caption).foregroundStyle(.secondary)
            }
            Text(model.phoneAccessMessage).font(.callout).accessibilityIdentifier("phone-access-status")
        }
        .task { await model.refreshPhoneAccess() }
        .fileImporter(isPresented:$importing,allowedContentTypes:[.data,.x509Certificate],allowsMultipleSelection:true) { result in
            if case .success(let urls) = result { Task { await model.importPhoneIdentity(urls) } }
        }
        .alert("Revoke this phone?",isPresented:$showRevoke) {
            Button("Cancel",role:.cancel) {}
            Button("Revoke",role:.destructive) { if let id = revokeTarget { Task { await model.revokePhone(deviceID:id) } } }
        } message: { Text("The phone loses access to this Mac. Conversations on both devices are kept.") }
    }
    private var listenerText: String {
        switch model.phoneListener {
        case .active: "On"
        case .starting: "Starting"
        case .stopping: "Stopping"
        case .unconfirmed: "Unconfirmed"
        case .stopped, nil: "Off"
        }
    }
    @ViewBuilder private var pairingRows: some View {
        switch model.phonePairing {
        case .paired(let id,let name,_,let active):
            LabeledContent("Paired phone",value:active ? name : name + " (expired)")
            Button("Revoke phone…",role:.destructive) { revokeTarget = id; showRevoke = true }.disabled(busy).accessibilityIdentifier("phone-access-revoke")
        case .invited:
            LabeledContent("Paired phone",value:"Invitation waiting")
            Button("Create a new invitation") { Task { await model.invitePhone() } }.disabled(busy || !model.phoneListenerActive)
        case .available, nil:
            Button("Invite a phone") { Task { await model.invitePhone() } }
                .disabled(busy || !model.phoneListenerActive).accessibilityIdentifier("phone-access-invite")
        }
    }
}
#endif
