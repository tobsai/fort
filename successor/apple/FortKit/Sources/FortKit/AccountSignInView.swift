import SwiftUI

struct AccountSignInView: View {
    @Bindable var model: MessengerModel
    var body: some View {
        Section("Provider account") {
            Text(model.runtimeName).font(.headline)
            Text(model.accountSignInMessage).font(.caption).foregroundStyle(.secondary)
            if let status = model.accountSignInStatus, let account = status.reportedAccount {
                Text(account.userLabel).textSelection(.enabled).accessibilityIdentifier("provider-account-label")
                Text("Workspace: \(account.accountId)").font(.caption).textSelection(.enabled)
                if !status.accountSelected {
                    Button("Use this account and workspace") { Task { await model.selectReportedAccount() } }
                        .disabled(model.accountSignInBusy || model.lifecycleBusy || model.isBusy || model.loadingModels)
                        .accessibilityIdentifier("provider-account-select")
                }
            }
            if let status = model.accountSignInStatus, let operation = status.operation, let challenge = status.challenge {
                TimelineView(.periodic(from:.now,by:1)) { context in
                    if Int64(context.date.timeIntervalSince1970 * 1000) < operation.expiresAt {
                        Text(challenge.userCode).font(.system(.title2,design:.monospaced)).textSelection(.enabled)
                            .accessibilityIdentifier("provider-device-code")
                        Link("Open verification page",destination:challenge.verificationURL)
                            .accessibilityIdentifier("provider-verification-page")
                        Text("Expires at \(Date(timeIntervalSince1970:Double(operation.expiresAt) / 1000).formatted(date:.omitted,time:.standard)).")
                            .font(.caption)
                    } else { Text("The code has expired. Check status before starting another attempt.").font(.caption) }
                }
            }
            if !model.hasPendingAccountSignIn && model.accountSignInStatus?.isActive != true {
                Button("Sign in to runtime") { Task { await model.startAccountSignIn() } }
                    .disabled(model.accountSignInBusy || model.lifecycleBusy || model.isBusy || model.runtimeReadiness?.state == "stopped")
                    .accessibilityIdentifier("provider-sign-in")
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
