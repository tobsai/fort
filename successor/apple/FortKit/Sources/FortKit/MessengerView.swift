import SwiftUI
import UniformTypeIdentifiers

public struct MessengerView: View {
    @Bindable private var model: MessengerModel
    @State private var uncertainRun: Run?
    @State private var memoryForm: ConversationMemoryForm?
    @State private var showImageImporter = false
    @State private var showSettings = false
    @State private var showWorkspace = false
    @State private var showRoutines = false
    @Environment(\.scenePhase) private var scenePhase
    public init(model: MessengerModel) { self.model = model }
    public var body: some View {
        NavigationSplitView {
            List(selection:SwiftUI.Binding(get:{ model.hasWorkspace ? model.activeConversationID : model.selectedAgentID },set:{ id in if model.hasWorkspace { Task { await model.selectConversation(id) } } else { model.selectedAgentID = id } })) {
                Section("Your Agents") {
                    ForEach(model.agents) { agent in
                        NavigationLink(value:model.hasWorkspace ? agent.homeConversationId : agent.agentId) {
                            HStack(spacing:10) {
                            AgentAvatarView(avatar:agent.avatar ?? .orbit)
                            VStack(alignment:.leading,spacing:4) {
                                Text(agent.name).font(.headline)
                                Text(agent.binding.selectedModel).font(.caption).foregroundStyle(.secondary)
                            }
                            }
                        }.tag(model.hasWorkspace ? agent.homeConversationId : agent.agentId)
                    }
                }
                if model.hasWorkspace {
                    Section("Shared conversations") { ForEach(model.conversations.filter { $0.kind == "shared" }) { room in NavigationLink(room.name,value:room.id).tag(room.id) } }
                }
            }
            .navigationTitle(model.isInteractiveFixture ? "Fort QA · First wave" : "Fort")
            .toolbar {
                if model.hasWorkspace { Button("Settings",systemImage:"gearshape") { showSettings = true } }
                if model.supportsRoutines { Button("Routines",systemImage:"list.bullet.rectangle") { showRoutines = true } }
                if !model.isOfflinePreview && !model.isInteractiveFixture { Button("Connection",systemImage:"network") { model.showConnection = true } }
                if model.hasWorkspace && !model.isInteractiveFixture { Button("Library",systemImage:"books.vertical") { showWorkspace = true } }
            }
        } detail: {
            VStack(spacing:0) {
                conversationHeader
                #if DEBUG
                if model.isInteractiveFixture {
                    HStack {
                        Button("Settings",systemImage:"gearshape") { showSettings = true }.accessibilityIdentifier("qa-open-settings")
                        Button("Add sample image",systemImage:"photo") { Task { await model.addFixtureImage() } }.accessibilityIdentifier("qa-add-image")
                        Button("Reload saved fixture",systemImage:"arrow.clockwise") { Task { await model.reloadInteractiveFixture() } }.accessibilityIdentifier("qa-reload")
                    }.padding(.horizontal).padding(.bottom,8)
                }
                #endif
                Divider()
                conversationHistory
                Divider()
                messageComposer
            }
            .toolbar {
                if model.canEditMemory, let conversationID = model.activeConversationID { Button("Memory",systemImage:"note.text") { memoryForm = ConversationMemoryForm(conversationID:conversationID,memory:model.conversationMemory) } }
                Button("Refresh",systemImage:"arrow.clockwise") { Task { await model.refresh() } }.disabled(model.isBusy)
            }
            .navigationTitle((model.isInteractiveFixture ? "Fort QA — " : "") + (model.selectedConversation?.name ?? model.selectedAgent?.name ?? "Conversation"))
        }
        .confirmationDialog("Close this uncertain run?",isPresented:SwiftUI.Binding(get:{ uncertainRun != nil },set:{ if !$0 { uncertainRun = nil } }),presenting:uncertainRun) { run in
            Button("Close without retry",role:.destructive) { Task { await model.closeUnknown(run) } }
        } message: { _ in Text("The original outcome remains unknown. Closing preserves its record and allows future messages. Fort will not retry this run.") }
        .sheet(isPresented:$model.showConnection) { connectionForm }
        .sheet(item:$memoryForm) { ConversationMemoryEditor(model:model,form:$0) }
        .fileImporter(isPresented:$showImageImporter,allowedContentTypes:[.png,.jpeg]) { result in
            if case .success(let url) = result { Task { await model.importLocalImage(url) } }
        }
        .sheet(isPresented:$showSettings) { FortSettingsView(model:model) }
        .sheet(isPresented:$showWorkspace) { WorkspaceEditor(model:model) }
        .sheet(isPresented:$showRoutines) { RoutineLibrary(model:model) }
        .task(id:scenePhase) {
            guard !model.isOfflinePreview, scenePhase == .active else { return }
            await model.restore()
            guard !model.isInteractiveFixture else { return }
            while !Task.isCancelled {
                await model.waitForChanges()
            }
        }
        .onChange(of:model.selectedAgentID) { _,_ in Task { await model.refresh() } }
    }

    private var conversationHeader: some View {
                VStack(alignment:.leading,spacing:6) {
                    Label(model.isInteractiveFixture ? "Interactive QA · simulated models and replies · no network" : model.isOfflinePreview ? "Offline preview · sample conversation" : model.hasWorkspace ? "Local prototype · provider review required" : "Development fixture · no live provider",systemImage:"hammer").font(.caption).foregroundStyle(.orange)
                    if let agent = model.recipient {
                        Text(agent.name).font(.title2.bold())
                        Text("\(agent.binding.selectedModel) · \(agent.binding.hostId)").font(.caption).foregroundStyle(.secondary)
                    }
                    if let memory = model.conversationMemory, !memory.text.isEmpty { Text("Saved memory applies to this conversation").font(.caption).foregroundStyle(.secondary) }
                    if let runtime = model.runtimeReadiness {
                        Text(runtime.detail).font(.caption).foregroundStyle(.secondary)
                        if runtime.state == "restartRequired" { Button("Restart runtime") { Task { await model.restartRuntime() } }.disabled(!runtime.canRestart || model.isBusy) }
                        if runtime.state == "stopped" && model.canManagePairing { Button("Resume local work") { Task { await model.restartRuntime() } }.disabled(!model.canResumeLocalWork) }
                    }
                    if let scheduler = model.schedulerStatus, scheduler.state == "fault" {
                        Text(scheduler.detail ?? "Schedules need attention.").font(.callout).foregroundStyle(.orange).accessibilityIdentifier("scheduler-fault")
                    }
                    Text(model.status).font(.callout).accessibilityIdentifier("connection-status")
                }.frame(maxWidth:.infinity,alignment:.leading).padding()

    }
    private var conversationHistory: some View {
        ScrollViewReader { position in
            ScrollView {
                LazyVStack(alignment:.leading,spacing:16) {
                    if model.isViewingEarlierMessages {
                        HStack {
                            Text("Earlier messages").font(.caption).foregroundStyle(.secondary)
                            Spacer()
                            Button("Latest messages") { model.showLatestMessages() }.accessibilityIdentifier("history-latest")
                        }
                    }
                    if model.displaySnapshot?.window?.olderBefore != nil {
                        Button(model.loadingEarlierMessages ? "Loading earlier messages…" : "Earlier messages") {
                            Task { await model.loadEarlierMessages() }
                        }.disabled(model.loadingEarlierMessages).accessibilityIdentifier("history-earlier")
                    }
                    if let error = model.imageError { Text(error).font(.caption).foregroundStyle(.orange) }
                    if model.displaySnapshot?.window?.olderBefore == nil {
                        ForEach(model.localImages.filter { $0.afterSequence == 0 }) { image in LocalImageCard(image:image) { model.removeLocalImage(image) } }
                    }
                    ForEach(model.displaySnapshot?.transcriptRows ?? []) { row in
                        switch row {
                        case .message(let message):
                            messageRow(message)
                            ForEach(model.localImages.filter { $0.afterSequence == message.sequence }) { image in LocalImageCard(image:image) { model.removeLocalImage(image) } }
                        case .run(let run): runRow(run)
                        case .approval(let approval): approvalRow(approval)
                        }
                    }
                    ForEach(model.localImages.filter { image in model.displaySnapshot?.window == nil && image.afterSequence != 0 && model.displaySnapshot?.messages.contains(where:{$0.sequence == image.afterSequence}) != true }) { image in LocalImageCard(image:image) { model.removeLocalImage(image) } }
                    ForEach(model.pending.filter { $0.conversationID == model.activeConversationID }) { pending in pendingRow(pending) }
                    Color.clear.frame(height:1).id("conversation-bottom")
                }.padding()
            }
            .accessibilityIdentifier("conversation-history")
            .onChange(of:model.pending.count) { _,_ in position.scrollTo("conversation-bottom",anchor:.bottom) }
            .onChange(of:model.localImages.count) { _,_ in position.scrollTo("conversation-bottom",anchor:.bottom) }
            .onChange(of:model.snapshot?.messages.last?.id) { _,_ in if !model.isViewingEarlierMessages { position.scrollTo("conversation-bottom",anchor:.bottom) } }
            .onChange(of:model.isViewingEarlierMessages) { _,earlier in if !earlier { position.scrollTo("conversation-bottom",anchor:.bottom) } }
        }
    }
    private func messageRow(_ message: Message) -> some View {
        VStack(alignment:.leading,spacing:5) {
            Text(message.role == "user" ? message.recipientAgentName.map { "You → \($0)" } ?? "You" : message.agentName ?? "Fixture Agent").font(.caption.bold()).foregroundStyle(.secondary)
            Text(message.text).textSelection(.enabled).frame(maxWidth:.infinity,alignment:.leading)
        }
        .padding(12)
        .background(message.role == "user" ? Color.accentColor.opacity(0.09) : Color.secondary.opacity(0.06),in:RoundedRectangle(cornerRadius:12))
        .accessibilityElement(children:.combine)
        .accessibilityIdentifier("message-" + message.id)
        .accessibilityLabel(Text("\(message.role == "user" ? "You" : message.agentName ?? "Fixture Agent")\n\(message.text)"))
        .task { await model.markRead(message) }
    }
    private func runRow(_ run: Run) -> some View {
        VStack(alignment:.leading) {
            Text(run.displayState).font(.caption)
            if let detail = run.statusDetail { Text(detail).font(.caption2) }
            if let configuration = run.configuration {
                Text("\(configuration.agentName) · \(configuration.selectedModel)").font(.caption2)
                if !configuration.skills.isEmpty { Text("Skills: " + configuration.skills.map(\.name).joined(separator:", ")).font(.caption2) }
            }
            if let routine = run.routine {
                DisclosureGroup(routine.definition.name) {
                    ForEach(routine.steps) { step in
                        Text((routine.definition.steps.first(where:{$0.id == step.id})?.name ?? step.id) + " · " + step.displayState).font(.caption)
                        if let output = step.output { Text(output.text).font(.caption).textSelection(.enabled) }
                        if let detail = step.detail { Text(detail).font(.caption2) }
                    }
                }
            }
            if let memory = run.configuration?.memory, !memory.text.isEmpty { DisclosureGroup("Memory used for this turn") { Text(memory.text).font(.caption).textSelection(.enabled) } }
            if run.canRequestCancellation {
                Button("Cancel run",role:.destructive) { Task { await model.cancel(run) } }.disabled(model.isBusy || !model.supportsCommands)
            }
            if run.canCloseUnknown {
                Button("Close without retry") { uncertainRun = run }.disabled(model.isBusy || !model.supportsCommands)
            }
            if run.state == "completed" {
                Text(run.effectiveModel.map { "Reported model: \($0) (fixture evidence)" } ?? "Effective model was not reported").font(.caption2)
            }
        }.foregroundStyle(.secondary)
    }
    private func approvalRow(_ approval: Approval) -> some View {
        VStack(alignment:.leading,spacing:8) {
            Label(approval.actionDescription,systemImage:"hand.raised").font(.headline)
            Text("Scope: " + approval.resourceScope.joined(separator:", ")).font(.caption)
            Text("Approval: " + approval.state).font(.caption)
            if approval.state == "pending" {
                HStack {
                    Button(approval.kind == "routineGate" ? "Continue routine" : "Approve fixture reply") { Task { await model.decide(approval,approve:true) } }
                    Button("Reject",role:.destructive) { Task { await model.decide(approval,approve:false) } }
                }.disabled(!approval.canDecide || model.isBusy || !model.supportsCommands)
            }
        }.padding().background(.quaternary,in:RoundedRectangle(cornerRadius:12))
    }
    private func pendingRow(_ pending: PendingSubmission) -> some View {
        VStack(alignment:.leading,spacing:4) {
            Text(pending.text)
            if let issue = model.pendingErrors[pending.requestID] { Text(issue).font(.caption).foregroundStyle(.orange) }
            if let reason = pending.reviewRequired { Text(reason).font(.caption).foregroundStyle(.orange) }
            Text(pending.cancelRequested == true ? "Cancellation saved on this device · confirmation pending" : pending.canRetry(in:model.installation?.recoveryEpoch ?? "") ? "Saved on this device · acceptance not confirmed" : "Review required · automatic retry paused").font(.caption).foregroundStyle(.secondary)
            if pending.cancelRequested != true, model.installation?.supports("requestCancellation") == true, pending.recoveryEpoch == model.installation?.recoveryEpoch {
                Button("Cancel pending request",role:.destructive) { Task { await model.cancelPending(pending) } }
            }
        }.padding().overlay(RoundedRectangle(cornerRadius:12).stroke(.secondary,style:StrokeStyle(lineWidth:1,dash:[4])))
    }
    private var messageComposer: some View {
        VStack(alignment:.leading,spacing:8) {
            if !model.isOfflinePreview, model.activeConversationID != nil {
                Button(model.importingImage ? "Opening image…" : "Show local image",systemImage:"photo") { showImageImporter = true }.font(.caption).disabled(model.importingImage)
            }
            if let conversation = model.selectedConversation, conversation.kind == "shared" {
                Picker("Recipient",selection:$model.recipientAgentID) {
                    Text("Choose an Agent").tag(Optional<String>.none)
                    ForEach(model.agents.filter { conversation.participantAgentIds.contains($0.id) }) { Text($0.name).tag(Optional($0.id)) }
                }
            }
            if model.hasWorkspace, !model.skills.isEmpty {
                DisclosureGroup("Skills for this message (\(model.selectedSkillIDs.count))") {
                    ForEach(model.skills) { skill in
                        Toggle(skill.name + (skill.canInvoke ? "" : " · review required"),isOn:SwiftUI.Binding(get:{ model.selectedSkillIDs.contains(skill.id) },set:{ if $0 && skill.canInvoke { model.selectedSkillIDs.insert(skill.id) } else { model.selectedSkillIDs.remove(skill.id) } }))
                            .disabled(!skill.canInvoke && !model.selectedSkillIDs.contains(skill.id))
                        if skill.bundle?.supportsTextChat(provider:model.runtimeProvider) == false { Text("This bundle needs a runtime that supports its files and requested tools.").font(.caption) }
                    }
                }
            }
                HStack(alignment:.bottom,spacing:12) {
                    TextField("Message your Agent",text:$model.draft,axis:.vertical).autocorrectionDisabled().lineLimit(1...6).textFieldStyle(.plain).padding(10).background(.quaternary,in:RoundedRectangle(cornerRadius:10)).accessibilityLabel("Message").accessibilityIdentifier("message-composer").disabled(model.isOfflinePreview)
                    Button("Send",systemImage:"arrow.up.circle.fill") { Task { await model.send() } }.disabled(!model.canSend).keyboardShortcut(.return,modifiers:.command).accessibilityIdentifier("send-message")
                }
        }.padding()
    }
    private var connectionForm: some View {
        NavigationStack {
            Form {
                #if os(macOS)
                if let host = model.localHost {
                    MacHostControlView(model:model,host:host)
                    if model.hasOwnerSetup { OwnerSetupView(model:model) }
                    else { Text(model.ownerSetupMessage).font(.callout) }
                }
                else if model.hasOwnerSetup { OwnerSetupView(model:model) }
                else { fixtureConnectionFields }
                #else
                fixtureConnectionFields
                #endif
                Text(model.status).font(.callout)
                if model.installation != nil { Button("Return to conversation") { model.showConnection = false } }
            }.formStyle(.grouped).navigationTitle("Connect to Fort")
        }.frame(minWidth:320,minHeight:440)
    }
    @ViewBuilder private var fixtureConnectionFields: some View {
        Section("Local installation") {
            Text("Use the address, installation ID and one-time enrollment code from the approved local Fort session. This prototype uses loopback on this Mac.").font(.callout)
            TextField("Service address",text:$model.endpoint).autocorrectionDisabled()
            TextField("Installation ID",text:$model.expectedInstallation).autocorrectionDisabled()
            TextField("Device name",text:$model.deviceName)
            SecureField("One-time enrollment code",text:$model.enrollmentCode)
        }
        Button("Enroll this device") { Task { await model.enroll() } }.disabled(model.isBusy || model.enrollmentCode.isEmpty || model.expectedInstallation.isEmpty)
    }
}
