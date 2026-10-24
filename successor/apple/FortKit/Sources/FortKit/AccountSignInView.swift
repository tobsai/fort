import SwiftUI

struct AccountSignInView: View {
    @Bindable var model: MessengerModel
    @State private var showAdvanced = false
    private var canStart: Bool { !model.hasPendingAccountSignIn && model.accountSignInStatus?.isActive != true }
    private var blocked: Bool { model.accountSignInBusy || model.lifecycleBusy || model.isBusy || model.loadingModels }
    var body: some View {
        Section("Provider account") {
            Text(model.runtimeName).font(.headline)
            Text(model.accountSignInMessage).font(.caption).foregroundStyle(.secondary)
            if let status = model.accountSignInStatus, let account = status.reportedAccount {
                Text(account.userLabel).textSelection(.enabled).accessibilityIdentifier("provider-account-label")
            }
            if let status = model.accountSignInStatus, let operation = status.operation, let challenge = status.challenge {
                TimelineView(.periodic(from:.now,by:1)) { context in
                    if Int64(context.date.timeIntervalSince1970 * 1000) < operation.expiresAt {
                        if let code = challenge.userCode {
                            Text(code).font(.system(.title2,design:.monospaced)).textSelection(.enabled)
                                .accessibilityIdentifier("provider-device-code")
                        }
                        Link(challenge.userCode == nil ? "Open sign-in page" : "Open verification page",destination:challenge.verificationURL)
                            .accessibilityIdentifier("provider-verification-page")
                        Text("Expires at \(Date(timeIntervalSince1970:Double(operation.expiresAt) / 1000).formatted(date:.omitted,time:.standard)).")
                            .font(.caption)
                    } else { Text(challenge.userCode == nil ? "The sign-in window has closed. Check status before starting another attempt." : "The code has expired. Check status before starting another attempt.").font(.caption) }
                }
            }
            if canStart && model.accountSignInStatus?.reportedAccount == nil {
                Button("Sign in") { Task { await model.startAccountSignIn() } }
                    .disabled(blocked || model.runtimeReadiness?.state == "stopped")
                    .accessibilityIdentifier("provider-sign-in")
            }
            Button { showAdvanced.toggle() } label: {
                Label("Advanced account management",systemImage:showAdvanced ? "chevron.down" : "chevron.right")
            }.buttonStyle(.plain).accessibilityValue(showAdvanced ? "Expanded" : "Collapsed")
                .accessibilityIdentifier("provider-account-advanced")
            if showAdvanced {
                Text("Fort uses one active runtime account. Changing it affects future requests. Saved history is kept, and queued requests never run under a different account.")
                    .font(.caption).foregroundStyle(.secondary)
                if let status = model.accountSignInStatus, let account = status.reportedAccount {
                    Text("Workspace: \(account.accountId)").font(.caption).textSelection(.enabled)
                    if !status.accountSelected {
                        Button("Use this account for future requests") { Task { await model.selectReportedAccount() } }
                            .disabled(blocked).accessibilityIdentifier("provider-account-select")
                    }
                    if canStart {
                        Button("Sign in to change account") { Task { await model.startAccountSignIn() } }
                            .disabled(blocked || model.runtimeReadiness?.state == "stopped")
                            .accessibilityIdentifier("provider-account-change")
                    }
                }
            }
            Button("Check sign-in status") { Task { await model.refreshAccountSignIn() } }
                .disabled(model.accountSignInBusy).accessibilityIdentifier("provider-sign-in-status")
            if model.hasPendingAccountSignIn || model.accountSignInStatus?.isActive == true {
                Button("Cancel sign-in attempt") { Task { await model.cancelAccountSignIn() } }
                    .disabled(model.accountSignInBusy || model.lifecycleBusy).accessibilityIdentifier("provider-sign-in-cancel")
                if model.accountSignInStatus == nil {
                    Button("Retry saved sign-in request") { Task { await model.retryAccountSignIn() } }
                        .disabled(model.accountSignInBusy || model.lifecycleBusy).accessibilityIdentifier("provider-sign-in-retry")
                    Button("Discard expired request") { Task { await model.discardExpiredAccountSignIn() } }
                        .disabled(model.accountSignInBusy || model.lifecycleBusy)
                }
            }
        }.task { await model.refreshAccountSignIn() }
    }
}
