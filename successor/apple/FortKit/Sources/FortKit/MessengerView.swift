import SwiftUI
import UniformTypeIdentifiers

public struct MessengerView: View {
    @Bindable private var model: MessengerModel
    @State private var uncertainRun: Run?
    @State private var memoryForm: ConversationMemoryForm?
    @State private var showImageImporter = false
    @State private var showMenu = false
    @State private var menuTab: PauseTab?
    @State private var showChatDetails = false
    @AppStorage("fort.showCity") private var showCity = false
    @AppStorage("fort.showDesk") private var showDesk = true
    @Environment(\.scenePhase) private var scenePhase
    #if os(iOS)
    @Environment(\.horizontalSizeClass) private var horizontalSizeClass
    /// The city's HUD panels are fixed widths that can't fit a phone, so it stays a Mac and iPad view.
    private var compactWidth: Bool { horizontalSizeClass == .compact }
    #else
    private let compactWidth = false
    #endif
    public init(model: MessengerModel) { self.model = model }
    public var body: some View {
        Group {
        // Home is the Quest log; the city is the Week view behind it, and a chat opens from either.
        if showDesk && model.hasWorkspace {
            QuestLogView(model:model,openChat:{ id in showDesk = false; showCity = false; Task { await model.selectConversation(id) } },
                         showCity:model.supportsWeeks ? { showDesk = false; showCity = true } : nil,openMenu:{ openMenu(nil) })
        } else if showCity && model.supportsWeeks && !compactWidth { CityHUDView(model:model,showMessages:{ showCity = false; showDesk = model.hasWorkspace },openMenu:{ openMenu(nil) }) }
        else if model.hasWorkspace {
            // A chat is the web's chat screen: the comic top bar, then "‹ Quest log", the chat and the composer.
            VStack(spacing:0) {
                ComicTopBar(summary:model.questLog.summary,showCity:model.supportsWeeks && !compactWidth ? { showCity = true } : nil,openSetup:{ openMenu(nil) },compact:compactWidth)
                chatColumn(back:{ showDesk = true })
            }
            .background(FortTheme.paper.ignoresSafeArea())
        } else {
        // Without a workspace (the development fixture and the offline preview) Agents are picked from a sidebar.
        NavigationSplitView {
            List(selection:$model.selectedAgentID) {
                Section("Crew") {
                    ForEach(model.agents) { agent in
                        NavigationLink(value:agent.agentId) { CrewRow(model:model,agent:agent) }.tag(agent.agentId)
                    }
                }
            }
            .navigationTitle(model.isInteractiveFixture ? "Fort QA · First wave" : "Fort")
            .toolbar {
                if model.supportsWeeks && !compactWidth { Button("City",systemImage:"building.2") { showCity = true }.accessibilityIdentifier("show-city") }
                if !PauseMenu.make(model.pauseFacts).entries.isEmpty {
                    Button("Setup",systemImage:"slider.horizontal.3") { openMenu(nil) }.keyboardShortcut(",",modifiers:.command).accessibilityIdentifier("pause-menu-open")
                }
            }
        } detail: {
            chatColumn(back:nil)
            .toolbar { Button("Refresh",systemImage:"arrow.clockwise") { Task { await model.refresh() } }.disabled(model.isBusy) }
            .navigationTitle((model.isInteractiveFixture ? "Fort QA — " : "") + (model.selectedConversation?.name ?? model.selectedAgent?.name ?? "Conversation"))
        }
        }
        }
        .safeAreaInset(edge:.top,spacing:0) { HubSignInBanner(model:model) }
        .confirmationDialog("Close this uncertain run?",isPresented:SwiftUI.Binding(get:{ uncertainRun != nil },set:{ if !$0 { uncertainRun = nil } }),presenting:uncertainRun) { run in
            Button("Close without retry",role:.destructive) { Task { await model.closeUnknown(run) } }
        } message: { _ in Text("The original outcome remains unknown. Closing preserves its record and allows future messages. Fort will not retry this run.") }
        .sheet(isPresented:$model.showConnection) { connectionForm }
        .sheet(item:$memoryForm) { ConversationMemoryEditor(model:model,form:$0) }
        #if !os(macOS)
        .fileImporter(isPresented:$showImageImporter,allowedContentTypes:[.png,.jpeg]) { result in
            if case .success(let url) = result { Task { await model.importLocalImage(url) } }
        }
        #endif
        .sheet(isPresented:$showMenu) { PauseMenuView(model:model,tab:menuTab) }
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

    private func openMenu(_ tab: PauseTab?) { menuTab = tab; showMenu = true }

    /// One chat, laid out like the web's: its header, anything stopping the crew, the transcript and the composer.
    private func chatColumn(back: (() -> Void)?) -> some View {
        VStack(spacing:0) {
            chatHeader(back:back)
            #if DEBUG
            if model.isInteractiveFixture {
                HStack {
                    Button("Settings",systemImage:"gearshape") { openMenu(.crew) }.accessibilityIdentifier("qa-open-settings")
                    Button("Add sample image",systemImage:"photo") { Task { await model.addFixtureImage() } }.accessibilityIdentifier("qa-add-image")
                    Button("Reload saved fixture",systemImage:"arrow.clockwise") { Task { await model.reloadInteractiveFixture() } }.accessibilityIdentifier("qa-reload")
                }.buttonStyle(ComicButtonStyle()).padding(.horizontal).padding(.vertical,8)
            }
            #endif
            TroubleStrip(cards:TroubleCard.make(model.troubleFacts)) { openMenu($0) }
            conversationHistory
            messageComposer
        }
        .background(FortTheme.paper)
        .foregroundStyle(FortTheme.ink)
        .environment(\.fortSurface,.comic)
    }

    /// The chat header: back to the Quest log, the chat's name in the display face, its one light and sentence.
    /// The model, host and recovery controls sit behind Details.
    private func chatHeader(back: (() -> Void)?) -> some View {
        let header = ChatQuiet.header(connected:model.connected,preview:model.isOfflinePreview || model.isInteractiveFixture,runtimeState:model.runtimeReadiness?.state,status:model.status)
        return VStack(alignment:.leading,spacing:8) {
            if model.isInteractiveFixture || model.isOfflinePreview || !model.hasWorkspace {
                Label(model.isInteractiveFixture ? "Interactive QA · simulated models and replies · no network" : model.isOfflinePreview ? "Offline preview · sample conversation" : "Development fixture · no live provider",systemImage:"hammer").font(.caption.weight(.bold)).foregroundStyle(FortTheme.muted)
            }
            HStack(spacing:10) {
                if let back { Button("‹ Quest log",action:back).buttonStyle(ComicButtonStyle()).accessibilityIdentifier("show-quest-log") }
                Text(model.selectedConversation?.name ?? model.selectedAgent?.name ?? "Chat").font(FortTheme.display(22)).tracking(0.9).lineLimit(1)
                    .accessibilityAddTraits(.isHeader)
                Spacer(minLength:8)
                HStack(spacing:6) {
                    LampView(lamp:header.lamp)
                    Text(header.line).font(.caption.weight(.bold)).foregroundStyle(FortTheme.muted).lineLimit(1)
                }.accessibilityElement(children:.combine)
                if model.canEditMemory, let conversationID = model.activeConversationID {
                    Button("Notebook") { memoryForm = ConversationMemoryForm(conversationID:conversationID,memory:model.conversationMemory) }
                        .buttonStyle(ComicButtonStyle()).accessibilityIdentifier("chat-notebook")
                }
                Button("Details") { showChatDetails.toggle() }.buttonStyle(ComicButtonStyle(fill:showChatDetails ? FortTheme.caption : FortTheme.panel))
                    .accessibilityValue(showChatDetails ? "Expanded" : "Collapsed").accessibilityIdentifier("chat-details")
            }
            if showChatDetails {
                VStack(alignment:.leading,spacing:6) {
                    if let agent = model.recipient { Text(CrewRow.line(provider:PauseMenu.className(provider:agent.binding.provider,runtimes:model.runtimes),model:agent.binding.selectedModel) + " · " + agent.binding.hostId).font(.caption.weight(.bold)).foregroundStyle(FortTheme.muted) }
                    if let runtime = model.runtimeReadiness {
                        CopyableText(runtime.detail).font(.caption).foregroundStyle(FortTheme.muted)
                        if runtime.state == "restartRequired" && (model.installation?.supports("ownerPairing") != true || model.canControlHost) { Button("Restart runtime") { Task { await model.restartRuntime() } }.buttonStyle(ComicButtonStyle()).disabled(!runtime.canRestart || model.isBusy) }
                        if runtime.state == "stopped" && model.canManagePairing { Button("Resume local work") { Task { await model.restartRuntime() } }.buttonStyle(ComicButtonStyle()).disabled(!model.canResumeLocalWork) }
                    }
                    if let scheduler = model.schedulerStatus, scheduler.state == "fault" {
                        CopyableText(scheduler.detail ?? "Schedules need attention.",id:"scheduler-fault").font(.callout).foregroundStyle(FortTheme.failed)
                    }
                    CopyableText(model.status,id:"connection-status").font(.callout)
                }
                .padding(10).frame(maxWidth:.infinity,alignment:.leading).comicPanel()
            }
        }
        .padding(.horizontal,14).padding(.vertical,8).frame(maxWidth:.infinity,minHeight:54,alignment:.leading)
        .background(FortTheme.bar)
        .overlay(alignment:.bottom) { FortTheme.line.frame(height:3) }
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
                    if let error = model.imageError { CopyableText(error).font(.caption).foregroundStyle(.orange) }
                    if model.displaySnapshot?.window?.olderBefore == nil {
                        ForEach(model.localImages.filter { $0.afterSequence == 0 }) { image in LocalImageCard(image:image) { model.removeLocalImage(image) } }
                    }
                    ForEach(model.displaySnapshot?.transcriptRows ?? []) { row in
                        switch row {
                        case .message(let message):
                            messageRow(message)
                            ForEach(model.localImages.filter { $0.afterSequence == message.sequence }) { image in LocalImageCard(image:image) { model.removeLocalImage(image) } }
                        case .run(let run): if ChatQuiet.showsRun(state:run.state,isRoutine:run.routine != nil) { runRow(run) }
                        case .approval(let approval): approvalRow(approval)
                        }
                    }
                    ForEach(model.localImages.filter { image in model.displaySnapshot?.window == nil && image.afterSequence != 0 && model.displaySnapshot?.messages.contains(where:{$0.sequence == image.afterSequence}) != true }) { image in LocalImageCard(image:image) { model.removeLocalImage(image) } }
                    ForEach(model.pending.filter { $0.conversationID == model.activeConversationID }) { pending in pendingRow(pending) }
                    Color.clear.frame(height:1).id("conversation-bottom")
                }.padding()
            }
            .defaultScrollAnchor(model.isViewingEarlierMessages ? .top : .bottom)
            .accessibilityIdentifier("conversation-history")
            .onChange(of:model.pending.count) { _,_ in position.scrollTo("conversation-bottom",anchor:.bottom) }
            .onChange(of:model.localImages.count) { _,_ in position.scrollTo("conversation-bottom",anchor:.bottom) }
            .onChange(of:model.snapshot?.messages.last?.id) { _,_ in if !model.isViewingEarlierMessages { position.scrollTo("conversation-bottom",anchor:.bottom) } }
            .onChange(of:model.isViewingEarlierMessages) { _,earlier in if !earlier { position.scrollTo("conversation-bottom",anchor:.bottom) } }
        }
    }
    /// A speech bubble, like .msg on the web: an Agent on the left beside its badge, you on the right.
    private func messageRow(_ message: Message) -> some View {
        let mine = message.role == "user"
        let author = model.agents.first(where:{ $0.name == message.agentName })
        let group = (model.selectedConversation?.participantAgentIds.count ?? 1) > 1
        let who = mine ? (message.recipientAgentName.map { "You → \($0)" } ?? "You") : message.agentName ?? "Agent"
        return HStack(alignment:.bottom,spacing:8) {
            if mine { Spacer(minLength:60) } else if let author { CrewBadge(model:model,agent:author,size:28) } else { AgentAvatarView(avatar:.orbit,size:28) }
            VStack(alignment:.leading,spacing:4) {
                if !mine || group { SpeakerCaption(text:who) }
                Text(message.text).textSelection(.enabled).fixedSize(horizontal:false,vertical:true)
                if let image = message.image { ConversationImageView(model:model,image:image) }
                if model.isDrawingImage(message) { Label("Drawing on your ChatGPT plan…",systemImage:"paintbrush").font(.caption).foregroundStyle(FortTheme.muted) }
            }
            .speechBubble(mine:mine)
            .frame(maxWidth:720,alignment:mine ? .trailing : .leading)
            .accessibilityElement(children:.combine)
            .accessibilityIdentifier("message-" + message.id)
            .accessibilityLabel(Text("\(mine ? "You" : message.agentName ?? "Agent")\n\(message.text)"))
            if !mine { Spacer(minLength:60) }
        }
        .task { await model.markRead(message) }
    }
    /// A run as a caption box with its lamp, like .run on the web. Mac-only controls (cancel, routine steps) sit under it.
    private func runRow(_ run: Run) -> some View {
        VStack(alignment:.leading,spacing:6) {
            RunCaption(line:RunCaptionLine.make(state:run.state,displayState:run.displayState,detail:run.statusDetail))
            Group {
                if let configuration = run.configuration, !configuration.skills.isEmpty { Text("Skills: " + configuration.skills.map(\.name).joined(separator:", ")).font(.caption2) }
                if let routine = run.routine {
                    DisclosureGroup(routine.definition.name) {
                        ForEach(routine.steps) { step in
                            Text((routine.definition.steps.first(where:{$0.id == step.id})?.name ?? step.id) + " · " + step.displayState).font(.caption)
                            if let output = step.output { Text(output.text).font(.caption).textSelection(.enabled) }
                            if let detail = step.detail { Text(detail).font(.caption2) }
                        }
                    }
                }
                if let memory = run.configuration?.memory, !memory.text.isEmpty { DisclosureGroup("Notebook used for this turn") { Text(memory.text).font(.caption).textSelection(.enabled) } }
                if run.state == "completed" {
                    Text(run.effectiveModel.map { "Reported model: \($0)" } ?? "Effective model was not reported").font(.caption2)
                }
            }.foregroundStyle(FortTheme.muted)
            if run.canRequestCancellation || run.canCloseUnknown {
                HStack(spacing:8) {
                    if run.canRequestCancellation { Button("Cancel run",role:.destructive) { Task { await model.cancel(run) } }.buttonStyle(ComicButtonStyle()) }
                    if run.canCloseUnknown { Button("Close without retry") { uncertainRun = run }.buttonStyle(ComicButtonStyle()) }
                }.controlSize(.small).disabled(model.isBusy || !model.supportsCommands)
            }
        }
    }
    /// An Agent asking to go on, like .approval on the web: "Needs you", what it wants, and Approve or Reject.
    private func approvalRow(_ approval: Approval) -> some View {
        let copy = ApprovalCopy.make(kind:approval.kind ?? "",state:approval.state)
        let open = approval.state == "pending"
        let detail = model.displaySnapshot?.runs.first(where:{ $0.runId == approval.runId })?.statusDetail
        return VStack(alignment:.leading,spacing:6) {
            ApprovalLabel(text:copy.label,open:open)
            Text(approval.actionDescription).font(.system(.body,design:.rounded).weight(.heavy)).fixedSize(horizontal:false,vertical:true)
            if let detail, detail != approval.actionDescription { Text(detail).font(.callout).foregroundStyle(FortTheme.muted) }
            if open {
                HStack(spacing:8) {
                    Button(copy.approve) { Task { await model.decide(approval,approve:true) } }.buttonStyle(ComicButtonStyle(fill:FortTheme.accent))
                    Button(copy.reject) { Task { await model.decide(approval,approve:false) } }.buttonStyle(ComicButtonStyle())
                }.padding(.top,4).disabled(!approval.canDecide || model.isBusy || !model.supportsCommands)
            }
        }
        .padding(.horizontal,14).padding(.vertical,12).frame(maxWidth:720,alignment:.leading)
        .comicPanel(live:open)
        .accessibilityElement(children:.contain)
    }
    private func pendingRow(_ pending: PendingSubmission) -> some View {
        VStack(alignment:.leading,spacing:4) {
            Text(pending.text)
            if let issue = model.pendingErrors[pending.requestID] { CopyableText(issue).font(.caption).foregroundStyle(FortTheme.failed) }
            if let reason = pending.reviewRequired { CopyableText(reason).font(.caption).foregroundStyle(FortTheme.failed) }
            Text(pending.cancelRequested == true ? "Cancellation saved on this device · confirmation pending" : pending.canRetry(in:model.installation?.recoveryEpoch ?? "") ? "Saved on this device · acceptance not confirmed" : "Review required · automatic retry paused").font(.caption).foregroundStyle(FortTheme.muted)
            if pending.cancelRequested != true, model.installation?.supports("requestCancellation") == true, pending.recoveryEpoch == model.installation?.recoveryEpoch {
                Button("Cancel pending request",role:.destructive) { Task { await model.cancelPending(pending) } }.buttonStyle(ComicButtonStyle())
            }
        }
        .padding(.horizontal,14).padding(.vertical,10)
        .overlay(RoundedRectangle(cornerRadius:FortTheme.bubbleRadius).strokeBorder(FortTheme.line,style:StrokeStyle(lineWidth:2,dash:[5,4])))
        .frame(maxWidth:.infinity,alignment:.trailing)
    }
    /// The composer, like .composer on the web: an ink-outlined pill and the gold Send button.
    private var messageComposer: some View {
        VStack(alignment:.leading,spacing:8) {
            #if !os(macOS)
            if !model.isOfflinePreview, model.activeConversationID != nil {
                Button(model.importingImage ? "Opening image…" : "Show local image",systemImage:"photo") { showImageImporter = true }.font(.caption).disabled(model.importingImage)
            }
            #endif
            if let conversation = model.selectedConversation, conversation.kind == "shared" {
                Picker("Send to",selection:$model.recipientAgentID) {
                    Text("Choose an Agent").tag(Optional<String>.none)
                    ForEach(model.agents.filter { conversation.participantAgentIds.contains($0.id) }) { Text($0.name).tag(Optional($0.id)) }
                }.fixedSize()
            }
            if model.hasWorkspace, !model.skills.isEmpty {
                DisclosureGroup("Skills for this message (\(model.selectedSkillIDs.count))") {
                    ForEach(model.skills) { skill in
                        Toggle(skill.name + (skill.canInvoke ? "" : " · review required"),isOn:SwiftUI.Binding(get:{ model.selectedSkillIDs.contains(skill.id) },set:{ if $0 && skill.canInvoke { model.selectedSkillIDs.insert(skill.id) } else { model.selectedSkillIDs.remove(skill.id) } }))
                            .disabled(!skill.canInvoke && !model.selectedSkillIDs.contains(skill.id))
                        if skill.bundle?.supportsTextChat(provider:model.runtimeProvider) == false { Text("This bundle needs a runtime that supports its files and requested tools.").font(.caption) }
                    }
                }.font(.callout.weight(.bold))
            }
            if model.imagePrompt != nil {
                Text("Sending draws one image on your ChatGPT plan.").font(.caption).foregroundStyle(FortTheme.muted).accessibilityIdentifier("image-hint")
            }
            HStack(alignment:.bottom,spacing:8) {
                let pill = RoundedRectangle(cornerRadius:22)
                Group {
                    #if os(macOS)
                    MessageComposer(text:$model.draft,editable:!model.isOfflinePreview,canSend:model.canSend) { Task { await model.send() } }
                    #else
                    TextField("Message",text:$model.draft,axis:.vertical).autocorrectionDisabled().lineLimit(1...6).textFieldStyle(.plain).accessibilityLabel("Message").accessibilityIdentifier("message-composer").disabled(model.isOfflinePreview)
                    #endif
                }
                .frame(maxWidth:.infinity,minHeight:24).padding(.horizontal,16).padding(.vertical,10)
                .background(pill.fill(FortTheme.panel)).overlay(pill.strokeBorder(FortTheme.line,lineWidth:FortTheme.stroke))
                Button("Send") { Task { await model.send() } }.buttonStyle(ComicButtonStyle(fill:FortTheme.accent)).frame(minHeight:44)
                    .disabled(!model.canSend).keyboardShortcut(.return,modifiers:.command).accessibilityIdentifier("send-message")
            }
        }
        .padding(.horizontal,12).padding(.vertical,10)
        .background(FortTheme.bar.ignoresSafeArea(edges:.bottom))
        .overlay(alignment:.top) { FortTheme.line.frame(height:3) }
    }
    private var connectionForm: some View {
        NavigationStack {
            Form { ConnectionSections(model:model) }.formStyle(.grouped).navigationTitle("Connect to Fort")
            .toolbar {
                ToolbarItem(placement:.cancellationAction) {
                    Button("Close setup") { model.showConnection = false }
                        .keyboardShortcut(.cancelAction).accessibilityIdentifier("setup-close")
                }
            }
        }.frame(minWidth:320,minHeight:440)
        #if os(macOS)
        .background(NonBlockingSheetWindow().allowsHitTesting(false).accessibilityHidden(true))
        #endif
    }
}

/// Connecting this device to Fort: the Mac's local host and owner setup, a phone's invitation, or the loopback fixture.
/// Shown by the first-run setup sheet and at the top of Devices in the pause menu.
struct ConnectionSections: View {
    @Bindable var model: MessengerModel
    var body: some View {
        #if os(macOS)
        if let host = model.localHost {
            MacHostControlView(model:model,host:host)
            if model.hasOwnerSetup { OwnerSetupView(model:model) }
            else { CopyableText(model.ownerSetupMessage).font(.callout) }
        }
        else if model.hasOwnerSetup { OwnerSetupView(model:model) }
        else if model.canPrepareCompanion { CompanionSetupView(model:model) }
        else { fixtureConnectionFields }
        #else
        if model.canPrepareCompanion { CompanionSetupView(model:model) }
        else { fixtureConnectionFields }
        #endif
        CopyableText(model.status).font(.callout)
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

#if os(macOS)
struct NonBlockingSheetWindow: NSViewRepresentable {
    final class SheetView: NSView {
        override func viewDidMoveToWindow() {
            super.viewDidMoveToWindow()
            window?.preventsApplicationTerminationWhenModal = false
        }
    }
    func makeNSView(context: Context) -> SheetView { SheetView() }
    func updateNSView(_ view: SheetView, context: Context) {}
}
#endif

/// Trouble above the chat: one card per problem, each with the one place to fix it.
struct TroubleStrip: View {
    let cards: [TroubleCard]
    let open: (PauseTab) -> Void
    var body: some View {
        if !cards.isEmpty {
            VStack(spacing:6) {
                ForEach(cards) { card in
                    HStack(spacing:10) {
                        LampView(lamp:.attention)
                        VStack(alignment:.leading,spacing:2) {
                            Text(card.title.uppercased()).font(.caption.weight(.heavy))
                            Text(card.line).font(.caption).foregroundStyle(.secondary).lineLimit(2)
                        }
                        Spacer(minLength:8)
                        Button(card.button) { open(card.tab) }.modifier(GoldButton(on:true)).controlSize(.small)
                    }
                    .padding(8).hudFrame()
                    .accessibilityElement(children:.contain).accessibilityIdentifier("trouble-\(card.id)")
                }
            }.padding(.horizontal).padding(.vertical,8)
        }
    }
}

extension MessengerModel {
    var troubleFacts: TroubleCard.Facts {
        let account = AccountPanel.make(status:accountSignInStatus,hasPendingRequest:hasPendingAccountSignIn,now:Int64(Date().timeIntervalSince1970 * 1000))
        return TroubleCard.Facts(
            lostContact:hasLostContact,
            status:status,
            runtimeState:runtimeReadiness?.state,
            runtimeDetail:runtimeReadiness?.detail,
            accountNeedsYou:canManageProviderAccount && accountSignInStatus != nil && (account.action == .signIn || account.action == .useAccount),
            scheduleFault:schedulerStatus?.state == "fault" ? schedulerStatus?.detail ?? "Schedules need attention." : nil
        )
    }
}
