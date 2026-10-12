import SwiftUI

public struct MessengerView: View {
    @Bindable private var model: MessengerModel
    @Environment(\.scenePhase) private var scenePhase
    public init(model: MessengerModel) { self.model = model }
    public var body: some View {
        NavigationSplitView {
            List(selection:$model.selectedAgentID) {
                Section("Your Agents") {
                    ForEach(model.agents) { agent in
                        VStack(alignment:.leading,spacing:4) {
                            Text(agent.name).font(.headline)
                            Text("Fixture · \(agent.binding.selectedModel)").font(.caption).foregroundStyle(.secondary)
                        }.tag(agent.agentId)
                    }
                }
            }
            .navigationTitle("Fort")
            .toolbar { Button("Connection",systemImage:"network") { model.showConnection = true } }
        } detail: {
            VStack(spacing:0) {
                VStack(alignment:.leading,spacing:6) {
                    Label("Development fixture · no live provider",systemImage:"hammer").font(.caption).foregroundStyle(.orange)
                    if let agent = model.selectedAgent {
                        Text(agent.name).font(.title2.bold())
                        Text("\(agent.binding.selectedModel) · \(agent.binding.hostId)").font(.caption).foregroundStyle(.secondary)
                    }
                    Text(model.status).font(.callout).accessibilityIdentifier("connection-status")
                }.frame(maxWidth:.infinity,alignment:.leading).padding()
                Divider()
                ScrollView {
                    LazyVStack(alignment:.leading,spacing:16) {
                        ForEach(model.snapshot?.messages ?? []) { message in
                            VStack(alignment:.leading,spacing:5) {
                                Text(message.role == "user" ? "You" : "Fixture Agent").font(.caption.bold()).foregroundStyle(.secondary)
                                Text(message.text).textSelection(.enabled).frame(maxWidth:.infinity,alignment:.leading)
                            }
                            .padding(12)
                            .background(message.role == "user" ? Color.accentColor.opacity(0.09) : Color.secondary.opacity(0.06),in:RoundedRectangle(cornerRadius:12))
                            .accessibilityElement(children:.combine)
                            .task { await model.markRead(message) }
                        }
                        ForEach(model.snapshot?.runs ?? []) { run in
                            VStack(alignment:.leading) {
                                Text(run.displayState).font(.caption)
                                if run.state == "completed" {
                                    Text(run.effectiveModel.map { "Reported model: \($0) (fixture evidence)" } ?? "Effective model was not reported").font(.caption2)
                                }
                            }.foregroundStyle(.secondary)
                        }
                        ForEach(model.pending.filter { $0.conversationID == model.selectedAgent?.homeConversationId }) { pending in
                            VStack(alignment:.leading,spacing:4) {
                                Text(pending.text)
                                Text(pending.canRetry(in:model.installation?.recoveryEpoch ?? "") ? "Saved on this device · acceptance not confirmed" : "Recovery changed · review required before retry").font(.caption).foregroundStyle(.secondary)
                            }.padding().overlay(RoundedRectangle(cornerRadius:12).stroke(.secondary,style:StrokeStyle(lineWidth:1,dash:[4])))
                        }
                    }.padding()
                }
                Divider()
                HStack(alignment:.bottom,spacing:12) {
                    TextField("Message your Agent",text:$model.draft,axis:.vertical).lineLimit(1...6).textFieldStyle(.plain).padding(10).background(.quaternary,in:RoundedRectangle(cornerRadius:10)).accessibilityLabel("Message").accessibilityIdentifier("message-composer")
                    Button("Send",systemImage:"arrow.up.circle.fill") { Task { await model.send() } }.disabled(!model.canSend).keyboardShortcut(.return,modifiers:.command).accessibilityIdentifier("send-message")
                }.padding()
            }
            .toolbar { Button("Refresh",systemImage:"arrow.clockwise") { Task { await model.refresh() } }.disabled(model.isBusy) }
            .navigationTitle(model.selectedAgent?.name ?? "Conversation")
        }
        .sheet(isPresented:$model.showConnection) { connectionForm }
        .task {
            await model.restore()
            while !Task.isCancelled {
                try? await Task.sleep(for:.seconds(2))
                if scenePhase == .active { await model.refresh() }
            }
        }
        .onChange(of:model.selectedAgentID) { _,_ in Task { await model.refresh() } }
        .onChange(of:scenePhase) { _,phase in if phase == .active { Task { await model.refresh() } } }
    }
    private var connectionForm: some View {
        NavigationStack {
            Form {
                Section("Fixture installation") {
                    Text("Use the address, installation ID and one-time enrollment code supplied by your local fixture service. A physical iPhone needs a trusted HTTPS endpoint; this slice exposes loopback only.").font(.callout)
                    TextField("Service address",text:$model.endpoint).autocorrectionDisabled()
                    TextField("Installation ID",text:$model.expectedInstallation).autocorrectionDisabled()
                    TextField("Device name",text:$model.deviceName)
                    SecureField("One-time enrollment code",text:$model.enrollmentCode)
                }
                Text(model.status).font(.callout)
                Button("Enroll this device") { Task { await model.enroll() } }.disabled(model.isBusy || model.enrollmentCode.isEmpty || model.expectedInstallation.isEmpty)
                if model.installation != nil { Button("Return to conversation") { model.showConnection = false } }
            }.formStyle(.grouped).navigationTitle("Connect to Fort")
        }.frame(minWidth:320,minHeight:440)
    }
}
