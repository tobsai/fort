import SwiftUI

/// The provider account inside a form, for the agent editor when no account is selected yet.
struct AccountSignInView: View {
    @Bindable var model: MessengerModel
    var body: some View {
        Section("Account") { AccountSignInPanel(model:model,framed:false) }
    }
}

/// One row per plan with a light, a sentence and one main action. Recovery steps live in the Workshop drawer.
struct AccountSignInPanel: View {
    @Bindable var model: MessengerModel
    var framed = true
    @State private var showWorkshop = false
    private var canStart: Bool { !model.hasPendingAccountSignIn && model.accountSignInStatus?.isActive != true }
    // Background refreshes don't disable controls (they made the buttons blink); actions wait for them instead.
    private var blocked: Bool { model.accountSignInBusy || model.lifecycleBusy || (model.isBusy && !model.isRefreshing) }
    private var runtimes: [RuntimeDescriptor] { model.runtimes.isEmpty ? model.focusedRuntime.map { [$0] } ?? [] : model.runtimes }

    var body: some View {
        let panel = AccountPanel.make(status:model.accountSignInStatus,hasPendingRequest:model.hasPendingAccountSignIn,now:Int64(Date().timeIntervalSince1970 * 1000))
        VStack(alignment:.leading,spacing:10) {
            if runtimes.isEmpty { focusedRow(name:model.runtimeName,panel:panel) }
            ForEach(runtimes,id:\.provider) { runtime in
                if runtime.provider == model.focusedRuntime?.provider { focusedRow(name:runtime.name,panel:panel) }
                else { otherRow(runtime) }
            }
            Text(model.accountSignInMessage).font(.caption).foregroundStyle(.secondary)
            workshop
        }
        .task { await model.refreshAccountSignIn() }
        .task(id:panel.keepsChecking) {
            guard panel.keepsChecking else { return }
            while !Task.isCancelled {
                try? await Task.sleep(for:.seconds(3))
                if Task.isCancelled { break }
                await model.refreshAccountSignIn()
            }
        }
    }

    private func focusedRow(name: String, panel: AccountPanel) -> some View {
        VStack(alignment:.leading,spacing:10) {
            HStack(alignment:.top,spacing:12) {
                LampView(lamp:panel.lamp).padding(.top,5)
                VStack(alignment:.leading,spacing:3) {
                    Text(name.uppercased()).font(.system(.headline,design:.rounded).weight(.black))
                    Text(panel.line).font(.callout).foregroundStyle(.secondary)
                    if let account = model.accountSignInStatus?.reportedAccount {
                        Text(account.userLabel).font(.caption).textSelection(.enabled).accessibilityIdentifier("provider-account-label")
                    }
                }
                Spacer(minLength:8)
                actionButton(panel.action,name:name)
            }
            if model.runtimeReadiness?.state == "stopped" && panel.action == .signIn {
                Text("Start the crew on this Mac first, under Devices.").font(.caption).foregroundStyle(.orange)
            }
            if let status = model.accountSignInStatus, let operation = status.operation, let challenge = status.challenge {
                TimelineView(.periodic(from:.now,by:1)) { context in
                    if Int64(context.date.timeIntervalSince1970 * 1000) < operation.expiresAt {
                        VStack(alignment:.leading,spacing:6) {
                            if let code = challenge.userCode {
                                Text(code).font(.system(size:30,weight:.black,design:.monospaced)).foregroundStyle(HUDStyle.gold)
                                    .padding(.horizontal,16).padding(.vertical,10).frame(maxWidth:.infinity)
                                    .overlay(RoundedRectangle(cornerRadius:6).strokeBorder(HUDStyle.gold,style:StrokeStyle(lineWidth:2,dash:[6,4])))
                                    .textSelection(.enabled).accessibilityIdentifier("provider-device-code")
                            }
                            HStack {
                                Link(challenge.userCode == nil ? "Open sign-in page" : "Open verification page",destination:challenge.verificationURL)
                                    .accessibilityIdentifier("provider-verification-page")
                                Spacer()
                                Text("Expires at \(Date(timeIntervalSince1970:Double(operation.expiresAt) / 1000).formatted(date:.omitted,time:.shortened))").font(.caption).foregroundStyle(.secondary)
                            }
                        }
                    }
                }
            }
        }
        .padding(framed ? 12 : 0).modifier(OptionalHUDFrame(on:framed))
    }

    @ViewBuilder private func actionButton(_ action: AccountPanel.Action?, name: String) -> some View {
        switch action {
        case .signIn? where canStart && model.accountSignInStatus?.reportedAccount == nil:
            Button("Sign in to \(name)") { Task { await model.startAccountSignIn() } }
                .buttonStyle(.borderedProminent).tint(HUDStyle.gold).foregroundStyle(.black)
                .disabled(blocked || model.runtimeReadiness?.state == "stopped")
                .accessibilityIdentifier("provider-sign-in")
        case .useAccount?:
            Button("Use this account") { Task { await model.selectReportedAccount() } }
                .buttonStyle(.borderedProminent).tint(HUDStyle.gold).foregroundStyle(.black)
                .disabled(blocked).accessibilityIdentifier("provider-account-select")
        case .cancel? where model.hasPendingAccountSignIn || model.accountSignInStatus?.isActive == true:
            Button("Cancel") { Task { await model.cancelAccountSignIn() } }
                .buttonStyle(.bordered)
                .disabled(model.accountSignInBusy || model.lifecycleBusy).accessibilityIdentifier("provider-sign-in-cancel")
        default: EmptyView()
        }
    }

    private func otherRow(_ runtime: RuntimeDescriptor) -> some View {
        HStack(spacing:12) {
            LampView(lamp:.off)
            VStack(alignment:.leading,spacing:3) {
                Text(runtime.name.uppercased()).font(.system(.headline,design:.rounded).weight(.black))
                Text("Show this plan to check or change its sign-in.").font(.callout).foregroundStyle(.secondary)
            }
            Spacer(minLength:8)
            Button("Show") { Task { await model.focusRuntime(runtime.provider) } }
                .buttonStyle(.bordered).disabled(!model.canChangeAccountProvider)
                .accessibilityIdentifier("provider-account-runtime-\(runtime.provider)")
        }
        .padding(framed ? 12 : 0).modifier(OptionalHUDFrame(on:framed))
    }

    private var workshop: some View {
        WorkshopDrawer(isExpanded:$showWorkshop,forceOpen:model.hasPendingAccountSignIn && model.accountSignInStatus == nil,identifier:"provider-account-advanced") {
            Text("Fort uses one active account per plan. Changing it affects future requests. Saved history is kept, and queued requests never run under a different account.")
                .font(.caption).foregroundStyle(.secondary)
            if let status = model.accountSignInStatus, let account = status.reportedAccount {
                Text("Workspace: \(account.accountId)").font(.caption).textSelection(.enabled)
                if canStart {
                    Button("Sign in to change account") { Task { await model.startAccountSignIn() } }
                        .disabled(blocked || model.runtimeReadiness?.state == "stopped")
                        .accessibilityIdentifier("provider-account-change")
                }
            }
            Button("Check sign-in status") { Task { await model.refreshAccountSignIn() } }
                .disabled(model.accountSignInBusy).accessibilityIdentifier("provider-sign-in-status")
            if model.hasPendingAccountSignIn && model.accountSignInStatus == nil {
                Button("Cancel sign-in attempt") { Task { await model.cancelAccountSignIn() } }
                    .disabled(model.accountSignInBusy || model.lifecycleBusy)
                Button("Retry saved sign-in request") { Task { await model.retryAccountSignIn() } }
                    .disabled(model.accountSignInBusy || model.lifecycleBusy).accessibilityIdentifier("provider-sign-in-retry")
                Button("Discard expired request") { Task { await model.discardExpiredAccountSignIn() } }
                    .disabled(model.accountSignInBusy || model.lifecycleBusy)
            }
        }
    }
}
