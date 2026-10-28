#if os(macOS)
import SwiftUI

/// Connects this Mac to a cloud hub as its runner (Spec 063). The hub's web client shows a one-time
/// "fort-runner:" invitation; pasting it here enrolls the Mac, and the Mac then takes the hub's turns.
struct CloudHubView: View {
    @Bindable var model: MessengerModel
    @State private var invitation = ""
    private var busy: Bool { model.hubRunnerBusy || model.lifecycleBusy || model.isBusy }
    var body: some View {
        Section("Cloud hub") {
            Text("Use Fort from your phone and the web. This Mac connects out to your hub and runs its turns on your own accounts.").font(.callout)
            if let status = model.hubRunner, status.enrolled {
                LabeledContent("Hub",value:status.hubURL ?? "Unknown")
                LabeledContent("This Mac",value:status.runnerId ?? "Unknown")
                LabeledContent("Connection",value:status.summary).accessibilityIdentifier("cloud-hub-state")
                Button("Disconnect from hub",role:.destructive) { Task { await model.removeHubRunner() } }
                    .disabled(busy).accessibilityIdentifier("cloud-hub-remove")
            } else {
                // A bordered box with a label, so it reads as somewhere to paste rather than as instructions.
                TextField("Invitation",text:$invitation,prompt:Text("fort-runner:…"),axis:.vertical).lineLimit(2...4)
                    .textFieldStyle(.roundedBorder).font(.caption.monospaced()).accessibilityIdentifier("cloud-hub-invitation")
                Button("Connect to cloud hub") { Task { await model.enrollHubRunner(invitation:invitation); if model.hubRunner?.enrolled == true { invitation = "" } } }
                    .disabled(busy || !invitation.trimmingCharacters(in:.whitespacesAndNewlines).hasPrefix("fort-runner:")).accessibilityIdentifier("cloud-hub-connect")
                Text("In the hub's web page, open Setup, choose Add a Mac, and copy the text it shows. It works once, for 5 minutes.").font(.caption).foregroundStyle(.secondary)
            }
            if !model.hubRunnerMessage.isEmpty { CopyableText(model.hubRunnerMessage,id:"cloud-hub-status").font(.callout) }
        }
        .task {
            while !Task.isCancelled {
                await model.refreshHubRunner()
                try? await Task.sleep(for:.seconds(5))
            }
        }
    }
}
#endif
