import SwiftUI
import FortKit

private enum MobileDeckView: String, CaseIterable, Identifiable {
    case deck, newConversation, conversation, assign, today, more, playbooks, performance, week

    static let primary: [MobileDeckView] = [.deck, .newConversation, .assign, .today, .more]

    var id: String { rawValue }
    var title: String {
        switch self {
        case .deck: return "Chats"
        case .newConversation: return "New"
        case .conversation: return "Conversation"
        case .assign: return "Assign"
        case .performance: return "Crew"
        case .week: return "Week"
        case .today: return "Today"
        case .more: return "More"
        case .playbooks: return "Playbooks"
        }
    }
    var icon: String {
        switch self {
        case .deck: return "bubble.left.and.bubble.right"
        case .newConversation: return "plus"
        case .conversation: return "bubble.left"
        case .assign: return "paperplane.fill"
        case .today: return "clock"
        case .more: return "ellipsis"
        case .playbooks: return "link"
        case .performance: return "person.2"
        case .week: return "calendar"
        }
    }

    var primarySelection: MobileDeckView {
        if self == .conversation { return .deck }
        return MobileDeckView.primary.contains(self) ? self : .more
    }
}

private enum MobileHandoffMode: String, CaseIterable, Identifiable {
    case assignment
    case quickQuestion

    var id: String { rawValue }
    var title: String { self == .assignment ? "Assignment" : "Quick question" }
    var icon: String? { self == .quickQuestion ? "bolt.fill" : nil }
}

struct BoardView: View {
    @EnvironmentObject private var client: FortClient

    @State private var summary: Summary?
    @State private var board = Board(runs: [], gates: [])
    @State private var backlog: [BacklogItem] = []
    @State private var machines: [MachineSummary] = []
    @State private var profiles: [ProfileOption] = []
    @State private var metrics: MetricsResponse?
    @State private var playbooks: [Playbook] = []
    @State private var selected: MobileDeckView = .deck
    @State private var loadError: String?
    @State private var deciding: Set<String> = []
    @State private var redirectGate: GateItem?
    @State private var draft = ""
    @State private var conversationDraft = ""
    @State private var selectedAgent = ""
    @State private var selectedProfileID = ""
    @State private var selectedMachine = ""
    @State private var proposePlan = true
    @State private var handoffMode: MobileHandoffMode = .assignment
    @State private var routePreview: RoutePreview?
    @State private var selectedPlaybookID: String?
    @State private var inlineAnswer: String?
    @State private var conversationAnswer: String?
    @State private var conversationStatus: String?
    @State private var selectedConversationID = ""
    @State private var conversationEvents: [Event] = []
    @State private var conversationEventCursor = 0
    @State private var directSending = false
    @State private var showConversationHistory = false
    @State private var showPlaybookPicker = false
    @State private var showFeed = false
    @State private var showSettings = false
    @State private var sending = false
    @State private var notice: String?

    private let newConversationID = "__new_conversation__"

    var body: some View {
        ZStack {
            FortPalette.page.ignoresSafeArea()
            VStack(spacing: 0) {
                if isConversationScreen {
                    conversationHeader
                    ScrollView {
                        conversationThread
                            .padding(16)
                    }
                    .scrollDismissesKeyboard(.interactively)
                    if let gate = selectedConversationGate {
                        conversationGateDock(gate)
                    } else {
                        conversationComposer
                    }
                } else {
                    ScrollView {
                        content
                            .padding(16)
                    }
                    .refreshable { await reload() }
                    if selected == .assign { assignmentButton }
                }
                primaryTabBar
            }
        }
        .foregroundStyle(FortPalette.primary)
        .navigationTitle(navigationTitle)
        .navigationBarTitleDisplayMode(.inline)
        .toolbar(mainNavigationHidden ? .hidden : .visible, for: .navigationBar)
        .task(id: client.baseURL) { await runLoop() }
        .task(id: client.baseURL) { await consumeConversationEvents() }
        .task(id: selectedConversation?.id) {
            if let run = selectedConversation { await loadConversationEvents(for: run) }
        }
        .task(id: routePreviewKey) { await refreshRoutePreview() }
        .sheet(item: $redirectGate) { gate in
            RedirectSheet(gate: gate) { note in
                Task { await decide(gate, decision: "reject", note: note) }
            }
            .presentationDetents([.medium])
        }
        .sheet(isPresented: $showPlaybookPicker) { playbookPicker }
        .sheet(isPresented: $showConversationHistory) { conversationHistorySheet }
        .sheet(isPresented: $showFeed) {
            NavigationStack { FeedView() }.environmentObject(client)
        }
        .sheet(isPresented: $showSettings) {
            SettingsView().environmentObject(client)
        }
        .alert("Fort", isPresented: Binding(
            get: { notice != nil },
            set: { if !$0 { notice = nil } }
        )) { Button("OK", role: .cancel) { notice = nil } } message: { Text(notice ?? "") }
    }

    @ViewBuilder private var content: some View {
        switch selected {
        case .deck: deckView
        case .newConversation, .conversation: EmptyView()
        case .assign: assignView
        case .performance: performanceView
        case .week: weekView
        case .today: todayView
        case .more: moreView
        case .playbooks: playbooksView
        }
    }

    private var deckView: some View {
        VStack(alignment: .leading, spacing: 20) {
            HStack(spacing: 12) {
                FortAgentOrbView(name: "Fort", state: fortOrbState, size: 44)
                VStack(alignment: .leading, spacing: 2) {
                    Text("FORT").font(.headline.weight(.bold)).tracking(4)
                    Text(fortOrbState == .working ? "Crew activity is live" : "Conversation command center")
                        .font(.caption).foregroundStyle(FortPalette.muted)
                }
                Spacer()
                if !board.gates.isEmpty {
                    Text("\(board.gates.count) needs you")
                        .deckChip(color: FortPalette.needsYou)
                }
            }

            HStack(alignment: .firstTextBaseline) {
                VStack(alignment: .leading, spacing: 3) {
                    Text("WHAT NEEDS YOU").sectionLabel(color: FortPalette.needsYou)
                    Text(attentionHeadline)
                        .font(.title3.weight(.semibold))
                }
                Spacer()
                if summary?.execution == false {
                    Label("Control-only", systemImage: "bolt.slash")
                        .font(.caption2).foregroundStyle(FortPalette.muted)
                }
            }

            if board.gates.isEmpty && failedRuns.isEmpty {
                FortDeckCard {
                    Label(attentionEmptyMessage, systemImage: "checkmark.circle")
                        .font(.callout).foregroundStyle(FortPalette.muted)
                }
            } else {
                ForEach(board.gates) { needsYouCard($0) }
                ForEach(failedRuns.prefix(3)) { failedRunCard($0) }
            }

            sectionTitle("CONVERSATIONS", count: conversationRuns.count)
            ForEach(conversationRuns) { conversationRow($0) }

            if !backlog.isEmpty {
                sectionTitle("UP NEXT", count: backlog.count)
                ForEach(backlog.prefix(4)) { upNextRow($0) }
            }

            sectionTitle("CREW", count: crewNames.count)
            ForEach(crewNames, id: \.self) { crewRow($0) }

            if let loadError { errorCard(loadError) }
        }
    }

    private var assignView: some View {
        VStack(alignment: .leading, spacing: 14) {
            HStack(spacing: 3) {
                ForEach(MobileHandoffMode.allCases) { mode in
                    Button {
                        handoffMode = mode
                        selectedPlaybookID = mode == .quickQuestion
                            ? FortPlaybookRouting.quickAnswer(in: playbooks)?.id
                            : nil
                        inlineAnswer = nil
                    } label: {
                        HStack(spacing: 5) {
                            if let icon = mode.icon { Image(systemName: icon).font(.caption) }
                            Text(mode.title)
                        }
                        .font(.callout.weight(handoffMode == mode ? .semibold : .regular))
                        .foregroundStyle(handoffMode == mode ? FortPalette.primary : FortPalette.muted)
                        .frame(maxWidth: .infinity, minHeight: 44)
                        .background(handoffMode == mode ? FortPalette.raised : Color.clear, in: RoundedRectangle(cornerRadius: 8))
                    }
                    .buttonStyle(.plain)
                }
            }
            .padding(3)
            .background(FortPalette.panel, in: RoundedRectangle(cornerRadius: 10))
            .overlay(RoundedRectangle(cornerRadius: 10).stroke(FortPalette.line))

            TextEditor(text: $draft)
                .accessibilityLabel("Assignment")
                .scrollContentBackground(.hidden)
                .foregroundStyle(FortPalette.primary)
                .frame(minHeight: 130)
                .padding(10)
                .background(FortPalette.panel, in: RoundedRectangle(cornerRadius: 10))
                .overlay(RoundedRectangle(cornerRadius: 10).stroke(FortPalette.raised))

            if quickModeUnavailable {
                FortDeckCard(accent: FortPalette.failed) {
                    Label("Quick question is unavailable because no answer playbook is configured.", systemImage: "exclamationmark.triangle")
                        .font(.callout).foregroundStyle(FortPalette.failed)
                }
            } else if let routePreview {
                FortRoutePreviewCard(routePreview) { showPlaybookPicker = true }
            } else if !draft.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
                FortDeckCard {
                    HStack(spacing: 9) {
                        ProgressView().controlSize(.small).tint(FortPalette.working)
                        Text("Resolving the route…").font(.callout).foregroundStyle(FortPalette.muted)
                    }
                }
            } else {
                FortDeckCard {
                    Label("Your route will appear here before anything starts.", systemImage: "point.3.connected.trianglepath.dotted")
                        .font(.callout).foregroundStyle(FortPalette.muted)
                }
            }

            if handoffMode == .assignment {
                Toggle(isOn: $proposePlan) {
                    VStack(alignment: .leading, spacing: 2) {
                        Text("Plan first").font(.callout.weight(.semibold))
                        Text("You sign off before build starts.").font(.caption).foregroundStyle(FortPalette.muted)
                    }
                }
                .tint(FortPalette.brass)
            }

            if let inlineAnswer {
                FortDeckCard {
                    VStack(alignment: .leading, spacing: 7) {
                        Text("QUICK ANSWER").sectionLabel(color: FortPalette.working)
                        Text(inlineAnswer).font(.body).foregroundStyle(FortPalette.primary).textSelection(.enabled)
                    }
                }
            }
        }
    }

    private var conversationHeader: some View {
        HStack(spacing: 10) {
            Button { showConversationHistory = true } label: {
                Image(systemName: "sidebar.left")
                    .font(.system(size: 17, weight: .semibold))
                    .frame(width: 44, height: 44)
            }
            .buttonStyle(.plain)
            .accessibilityLabel("Open conversations")

            if let run = selectedConversation {
                FortAgentOrbView(name: conversationAgent(run), state: conversationActivity(run).projectState, size: 30)
            } else {
                FortAgentOrbView(name: "Fort", state: .idle, size: 30)
            }

            VStack(alignment: .leading, spacing: 2) {
                Text(selectedConversation.map(title) ?? "New conversation")
                    .font(.callout.weight(.semibold))
                    .lineLimit(1)
                if let run = selectedConversation {
                    Text(conversationActivity(run) == .pausedForReview ? "Needs approval" : conversationActivity(run).label)
                        .font(.caption2.weight(.semibold))
                        .foregroundStyle(conversationActivity(run).projectState.color)
                } else {
                    Text("Choose who should respond")
                        .font(.caption2).foregroundStyle(FortPalette.muted)
                }
            }
            Spacer()
            Button { beginNewConversation() } label: {
                Image(systemName: "square.and.pencil")
                    .frame(width: 44, height: 44)
            }
            .buttonStyle(.plain)
            .accessibilityLabel("New conversation")
        }
        .padding(.horizontal, 8)
        .frame(minHeight: 52)
        .background(FortPalette.canvas)
        .overlay(alignment: .bottom) { Rectangle().fill(FortPalette.line).frame(height: 1) }
    }

    @ViewBuilder private var conversationThread: some View {
        VStack(alignment: .leading, spacing: 16) {
            if let run = selectedConversation {
                conversationMessage(
                    name: "You",
                    detail: FortTime.relative(run.createdAt),
                    body: conversationPrompt(run),
                    state: .idle,
                    role: .human
                )
                conversationMessage(
                    name: conversationAgent(run),
                    detail: FortTime.relative(run.updatedAt ?? run.createdAt),
                    body: conversationResponse(run),
                    model: run.model,
                    state: conversationActivity(run).projectState,
                    role: .agent
                )

                ForEach(gatesForConversation(run)) { gate in
                    conversationGateCard(gate)
                }

                if FortConversationPromotion.isEligible(run, gates: board.gates) {
                    promotionCard(run)
                }

                conversationProgress(run)
                conversationActivityTimeline(run)
            } else {
                conversationMessage(
                    name: "Fort",
                    detail: "ready",
                    body: "Choose an agent, an exact model, and an eligible machine — or let Fort decide — then send your first message.",
                    state: .idle,
                    role: .agent
                )
            }

            if let conversationAnswer, !conversationAnswer.isEmpty {
                conversationMessage(
                    name: selectedProfile?.agent.capitalized ?? "Fort",
                    detail: "just now",
                    body: conversationAnswer,
                    model: selectedProfile?.model,
                    state: .delivered,
                    role: .agent
                )
            }

            if let conversationStatus, !conversationStatus.isEmpty, !directSending {
                FortDeckCard(accent: FortPalette.failed) {
                    Label(conversationStatus, systemImage: "exclamationmark.triangle.fill")
                        .font(.callout).foregroundStyle(FortPalette.failed)
                        .textSelection(.enabled)
                }
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }

    private var conversationComposer: some View {
        VStack(spacing: 8) {
            ZStack(alignment: .topLeading) {
                if conversationDraft.isEmpty {
                    Text("Message Fort…")
                        .font(.callout).foregroundStyle(FortPalette.faint)
                        .padding(.horizontal, 15).padding(.vertical, 14)
                        .allowsHitTesting(false)
                }
                TextEditor(text: $conversationDraft)
                    .accessibilityLabel("Message Fort")
                    .font(.callout)
                    .scrollContentBackground(.hidden)
                    .foregroundStyle(FortPalette.primary)
                    .frame(minHeight: 58, maxHeight: 92)
                    .padding(.horizontal, 8).padding(.vertical, 4)
            }
            .background(FortPalette.panel, in: RoundedRectangle(cornerRadius: 12))
            .overlay(RoundedRectangle(cornerRadius: 12).stroke(FortPalette.raised))

            HStack(spacing: 6) {
                Picker("Agent", selection: $selectedAgent) {
                    Text("Agent").tag("")
                    ForEach(availableAgents, id: \.self) { agent in
                        Text(agent.capitalized).tag(agent)
                    }
                }
                .pickerStyle(.menu)
                .font(.caption)
                .lineLimit(1)
                .minimumScaleFactor(0.72)
                .frame(maxWidth: .infinity, minHeight: 44)
                .background(FortPalette.panel, in: RoundedRectangle(cornerRadius: 9))
                .onChange(of: selectedAgent) { _ in selectDefaultProfileForAgent() }

                Picker("Model", selection: $selectedProfileID) {
                    Text("Model").tag("")
                    ForEach(profilesForSelectedAgent) { profile in
                        Text(modelOptionLabel(profile)).tag(profile.id)
                            .disabled(!profileIsReady(profile))
                    }
                }
                .pickerStyle(.menu)
                .font(.caption)
                .lineLimit(1)
                .minimumScaleFactor(0.72)
                .frame(maxWidth: .infinity, minHeight: 44)
                .background(FortPalette.panel, in: RoundedRectangle(cornerRadius: 9))
                .onChange(of: selectedProfileID) { _ in profileSelectionChanged() }

                Picker("Machine", selection: $selectedMachine) {
                    Text("Machine").tag("")
                    ForEach(availableMachineNames, id: \.self) { machine in
                        Text(machine).tag(machine)
                    }
                }
                .pickerStyle(.menu)
                .font(.caption)
                .lineLimit(1)
                .minimumScaleFactor(0.72)
                .frame(maxWidth: .infinity, minHeight: 44)
                .background(FortPalette.panel, in: RoundedRectangle(cornerRadius: 9))
            }

            if selectedProfileIsUnavailable {
                Text("That exact model is not ready. Choose a ready model before sending.")
                    .font(.caption).foregroundStyle(FortPalette.failed)
                    .frame(maxWidth: .infinity, alignment: .leading)
            }

            HStack(spacing: 10) {
                if directSending {
                    FortAgentOrbView(name: "Fort", state: .idle, size: 22)
                    Text("Submitting to Fort…")
                        .font(.caption.weight(.semibold))
                        .foregroundStyle(FortPalette.working)
                        .accessibilityLabel("Submitting to Fort")
                } else {
                    Text(selectedProfile?.displayName ?? "Fort will choose a ready profile")
                        .font(.caption).foregroundStyle(FortPalette.muted).lineLimit(1)
                }
                Spacer()
                Button { beginConversationSend() } label: {
                    Label("Send", systemImage: "paperplane.fill")
                        .font(.callout.weight(.semibold))
                        .frame(minWidth: 76, minHeight: 44)
                }
                .buttonStyle(.borderedProminent)
                .tint(FortPalette.working)
                .disabled(
                    directSending || composerSelectionIsInvalid ||
                    conversationDraft.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
                )
            }
        }
        .padding(.horizontal, 10).padding(.vertical, 9)
        .background(FortPalette.canvas)
        .overlay(alignment: .top) { Rectangle().fill(FortPalette.line).frame(height: 1) }
    }

    private var conversationHistorySheet: some View {
        NavigationStack {
            ZStack {
                FortPalette.page.ignoresSafeArea()
                ScrollView {
                    VStack(alignment: .leading, spacing: 16) {
                        Button { showConversationHistory = false; beginNewConversation() } label: {
                            Label("New conversation", systemImage: "plus")
                                .font(.callout.weight(.semibold))
                                .frame(maxWidth: .infinity, minHeight: 48)
                        }
                        .buttonStyle(.borderedProminent)
                        .tint(FortPalette.working)

                        if !board.gates.isEmpty {
                            sectionTitle("NEEDS YOU", count: board.gates.count)
                            ForEach(board.gates) { gate in
                                if let run = board.runs.first(where: { $0.id == gate.runID }) {
                                    conversationRow(run)
                                }
                            }
                        }

                        sectionTitle("CONVERSATIONS", count: conversationRuns.count)
                        ForEach(conversationRuns) { conversationRow($0) }
                    }
                    .padding(16)
                }
            }
            .foregroundStyle(FortPalette.primary)
            .navigationTitle("Conversations")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .confirmationAction) {
                    Button("Done") { showConversationHistory = false }
                }
            }
        }
        .presentationDetents([.large])
    }

    private var performanceView: some View {
        VStack(alignment: .leading, spacing: 14) {
            Text("CREW PERFORMANCE").sectionLabel()
            Text("last \(metrics?.windowDays ?? 30) days · \(metrics?.assignments ?? 0) assignments")
                .font(.callout).foregroundStyle(FortPalette.muted)
            if let agents = metrics?.agents, !agents.isEmpty {
                ForEach(agents) { metricCard($0) }
            } else {
                emptyCard("No scorecards yet", "Performance appears after sign-offs create a useful sample.")
            }
        }
    }

    private var weekView: some View {
        VStack(alignment: .leading, spacing: 14) {
            Text("THE WEEK").sectionLabel()
            Text("Upcoming work by crew member").font(.title3.weight(.semibold))
            ForEach(crewNames, id: \.self) { agent in
                scheduleCard(agent: agent, includeQueued: true)
            }
            if crewNames.isEmpty { emptyCard("Open capacity", "No crew assignments are scheduled.") }
        }
    }

    private var todayView: some View {
        VStack(alignment: .leading, spacing: 14) {
            Text("TODAY").sectionLabel()
            Text(board.gates.isEmpty ? "Your day is clear." : "\(board.gates.count) sign-off\(board.gates.count == 1 ? "" : "s") need you now.")
                .font(.title3.weight(.semibold))
            FortDeckCard(accent: board.gates.isEmpty ? FortPalette.accepted : FortPalette.needsYou) {
                VStack(alignment: .leading, spacing: 10) {
                    Text("YOU").sectionLabel(color: FortPalette.brass)
                    if board.gates.isEmpty {
                        Text("No checkpoints are waiting for you.").foregroundStyle(FortPalette.muted)
                    } else {
                        ForEach(board.gates) { gate in
                            Label(gateTitle(gate), systemImage: "signature")
                                .font(.callout.weight(.semibold))
                                .foregroundStyle(FortPalette.needsYou)
                        }
                    }
                }
            }
            ForEach(crewNames, id: \.self) { scheduleCard(agent: $0, includeQueued: false) }
        }
    }

    private var moreView: some View {
        VStack(alignment: .leading, spacing: 14) {
            Text("FORT").sectionLabel(color: FortPalette.brassBright)
            moreRow("Playbooks", subtitle: "Who does what, with which model", icon: "link") { selected = .playbooks }
            moreRow("Crew", subtitle: "Human-accepted performance scorecards", icon: "person.2") { selected = .performance }
            moreRow("The week", subtitle: "Upcoming work and open capacity", icon: "calendar") { selected = .week }
            moreRow("Activity", subtitle: "The append-only live feed", icon: "dot.radiowaves.left.and.right") { showFeed = true }
            moreRow("Settings", subtitle: client.baseURL.absoluteString, icon: "gearshape") { showSettings = true }
        }
    }

    private var playbooksView: some View {
        VStack(alignment: .leading, spacing: 14) {
            Text("WHO DOES WHAT, WITH WHICH MODEL")
                .sectionLabel(color: FortPalette.working)
            Text("Choose a route from Assign. Editing stays on desktop.")
                .font(.callout).foregroundStyle(FortPalette.muted)
            if playbooks.isEmpty {
                emptyCard("No playbooks available", "Fort has not returned a route catalog yet.")
            } else {
                ForEach(playbooks) { playbook in FortPlaybookCard(playbook) }
            }
        }
    }

    private var primaryTabBar: some View {
        HStack(alignment: .bottom, spacing: 0) {
            ForEach(MobileDeckView.primary) { item in
                Button {
                    selectPrimary(item)
                } label: {
                    VStack(spacing: 4) {
                        if item == .newConversation {
                            ZStack {
                                Circle().fill(FortPalette.brass).frame(width: 46, height: 46)
                                Image(systemName: item.icon).font(.system(size: 17, weight: .semibold)).foregroundStyle(FortPalette.page)
                            }
                            .overlay(Circle().stroke(FortPalette.brassBright.opacity(0.45)))
                            .offset(y: -7)
                            .padding(.bottom, -7)
                        } else {
                            Image(systemName: item.icon).font(.system(size: 17, weight: .semibold))
                                .frame(height: 24)
                        }
                        Text(item.title).font(.caption2.weight(.semibold))
                    }
                    .foregroundStyle(selected.primarySelection == item ? FortPalette.brassBright : FortPalette.muted)
                    .frame(maxWidth: .infinity, minHeight: 58)
                    .contentShape(Rectangle())
                }
                .buttonStyle(.plain)
                .accessibilityLabel(item.title)
                .accessibilityAddTraits(selected.primarySelection == item ? .isSelected : [])
            }
        }
        .padding(.horizontal, 6)
        .padding(.top, 4)
        .background(FortPalette.canvas)
        .overlay(alignment: .top) { Rectangle().fill(FortPalette.line).frame(height: 1) }
    }

    private var assignmentButton: some View {
        Button { beginAssignment() } label: {
            HStack {
                if sending { ProgressView().tint(FortPalette.page) }
                Text(sending ? "Starting assignment…" : "Start assignment").font(.body.weight(.semibold))
            }
            .frame(maxWidth: .infinity, minHeight: 50)
        }
        .buttonStyle(.borderedProminent)
        .tint(FortPalette.brass)
        .foregroundStyle(FortPalette.page)
        .disabled(sending || quickModeUnavailable || draft.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)
        .padding(.horizontal, 16).padding(.vertical, 10)
        .background(FortPalette.page)
    }

    private var playbookPicker: some View {
        NavigationStack {
            List(availablePlaybooks) { playbook in
                Button {
                    selectedPlaybookID = playbook.id
                    showPlaybookPicker = false
                } label: {
                    HStack(spacing: 12) {
                        Image(systemName: playbook.delivery == "answer" ? "bolt.fill" : "link")
                            .foregroundStyle(FortPalette.brassBright)
                        VStack(alignment: .leading, spacing: 3) {
                            Text(playbook.name).font(.headline)
                            Text("\(playbook.stages.count) stage\(playbook.stages.count == 1 ? "" : "s") · revision \(playbook.revision)")
                                .font(.caption).foregroundStyle(.secondary)
                        }
                        Spacer()
                        if selectedPlaybookID == playbook.id || routePreview?.playbookID == playbook.id {
                            Image(systemName: "checkmark").foregroundStyle(FortPalette.brass)
                        }
                    }
                }
                .buttonStyle(.plain)
            }
            .navigationTitle("Choose a playbook")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) { Button("Cancel") { showPlaybookPicker = false } }
                ToolbarItem(placement: .confirmationAction) {
                    Button("Fort decides") { selectedPlaybookID = nil; showPlaybookPicker = false }
                }
            }
        }
        .presentationDetents([.medium, .large])
    }

    private var availablePlaybooks: [Playbook] {
        playbooks.filter { handoffMode == .quickQuestion ? $0.delivery == "answer" : $0.delivery != "answer" }
    }

    private func moreRow(_ title: String, subtitle: String, icon: String, action: @escaping () -> Void) -> some View {
        Button(action: action) {
            FortDeckCard {
                HStack(spacing: 13) {
                    Image(systemName: icon).font(.title3).foregroundStyle(FortPalette.brassBright).frame(width: 28)
                    VStack(alignment: .leading, spacing: 3) {
                        Text(title).font(.headline).foregroundStyle(FortPalette.primary)
                        Text(subtitle).font(.caption).foregroundStyle(FortPalette.muted).lineLimit(1)
                    }
                    Spacer()
                    Image(systemName: "chevron.right").font(.caption.weight(.semibold)).foregroundStyle(FortPalette.faint)
                }
            }
        }
        .buttonStyle(.plain)
    }

    private func needsYouCard(_ gate: GateItem) -> some View {
        let run = board.runs.first { $0.id == gate.runID }
        return FortDeckCard(accent: FortPalette.needsYou) {
            VStack(alignment: .leading, spacing: 12) {
                HStack {
                    Text(gateTitle(gate)).font(.headline)
                    Spacer()
                    Text(FortTime.relative(gate.since)).font(.caption.monospaced()).foregroundStyle(FortPalette.faint)
                }
                Text(gate.input?.isEmpty == false ? gate.input! : "\(run?.title ?? gate.runID) reached a checkpoint and needs your sign-off.")
                    .font(.callout).foregroundStyle(FortPalette.body)
                    .fixedSize(horizontal: false, vertical: true)
                VStack(spacing: 8) {
                    Button { Task { await decide(gate, decision: "approve") } } label: {
                        Text("Approve & continue")
                            .foregroundStyle(FortPalette.page)
                            .frame(maxWidth: .infinity, minHeight: 44)
                    }
                        .buttonStyle(.borderedProminent).tint(FortPalette.accepted)
                    Button { redirectGate = gate } label: {
                        Text("Request changes").frame(maxWidth: .infinity, minHeight: 44)
                    }
                        .buttonStyle(.bordered).tint(FortPalette.muted)
                    if deciding.contains(gate.id) { ProgressView().controlSize(.small) }
                }
                .disabled(deciding.contains(gate.id))
            }
        }
    }

    private func failedRunCard(_ run: RunSummary) -> some View {
        FortDeckCard(accent: FortPalette.failed) {
            VStack(alignment: .leading, spacing: 12) {
                HStack {
                    Text("\(title(run)) hit a wall").font(.headline)
                    Spacer()
                    Text(FortTime.relative(run.updatedAt ?? run.createdAt))
                        .font(.caption.monospaced()).foregroundStyle(FortPalette.faint)
                }
                Text("\(run.agent) stopped. Open the conversation to see what happened and choose the next step.")
                    .font(.callout).foregroundStyle(FortPalette.body)
                Button { selectConversation(run) } label: {
                    Text("View what happened").frame(minHeight: 44)
                }
                .buttonStyle(.bordered)
                .tint(FortPalette.failed)
            }
        }
    }

    private func conversationRow(_ run: RunSummary) -> some View {
        let activity = conversationActivity(run)
        return Button { selectConversation(run) } label: {
            HStack(spacing: 12) {
                FortAgentOrbView(name: conversationAgent(run), state: activity.projectState, size: 36)
                VStack(alignment: .leading, spacing: 4) {
                    Text(title(run)).font(.callout.weight(.semibold)).foregroundStyle(FortPalette.primary).lineLimit(1)
                    Text(conversationTimestamp(run))
                        .font(.caption).foregroundStyle(FortPalette.muted).lineLimit(1)
                }
                Spacer()
                Text(activity == .pausedForReview ? "Needs approval" : activity.label)
                    .deckChip(color: activity.projectState.color)
                Image(systemName: "chevron.right").font(.caption).foregroundStyle(FortPalette.faint)
            }
            .frame(minHeight: 44)
        }
        .buttonStyle(.plain)
    }

    private func upNextRow(_ item: BacklogItem) -> some View {
        HStack(spacing: 10) {
            RoundedRectangle(cornerRadius: 4).fill(FortPalette.queued).frame(width: 9, height: 32)
            VStack(alignment: .leading, spacing: 2) {
                Text(item.title).font(.callout.weight(.semibold)).lineLimit(1)
                Text(item.agent?.isEmpty == false ? item.agent! : "Fort decides")
                    .font(.caption).foregroundStyle(FortPalette.muted)
            }
            Spacer()
            Button { Task { await dispatch(item) } } label: { Text("Start").frame(minHeight: 44) }
                .buttonStyle(.bordered).tint(FortPalette.brass)
        }
    }

    private func upNextCard(_ item: BacklogItem) -> some View {
        FortDeckCard {
            VStack(alignment: .leading, spacing: 9) {
                HStack { Text(item.title).font(.headline); Spacer(); Text("up next").deckChip(color: FortPalette.muted) }
                if let body = item.body, !body.isEmpty { Text(body).font(.callout).foregroundStyle(FortPalette.body).lineLimit(3) }
                HStack {
                    Text(item.agent?.isEmpty == false ? item.agent! : "Fort decides").font(.caption).foregroundStyle(FortPalette.muted)
                    Spacer()
                    Button { Task { await dispatch(item) } } label: { Text("Start").frame(minHeight: 44) }
                        .buttonStyle(.borderedProminent).tint(FortPalette.brass).foregroundStyle(FortPalette.page)
                }
            }
        }
    }

    private func crewRow(_ agent: String) -> some View {
        let state = agentState(agent)
        let active = board.runs.first { $0.agent == agent && ["running", "blocked"].contains($0.status) }
        return HStack(spacing: 10) {
            Circle().fill(state.color).frame(width: 8, height: 8)
            Text(agent).font(.callout.weight(.semibold))
            Spacer()
            Text(active.map(activityLine) ?? "available")
                .font(.caption).foregroundStyle(FortPalette.muted).lineLimit(1)
        }
    }

    private func crewCard(_ agent: String) -> some View {
        FortDeckCard(accent: agentState(agent).color) {
            VStack(alignment: .leading, spacing: 7) {
                HStack { Text(agent).font(.headline); Spacer(); FortStatusPill(agentState(agent)) }
                Text(board.runs.first { $0.agent == agent && $0.status == "running" }.map(activityLine) ?? "Open capacity — ready for an assignment.")
                    .font(.callout).foregroundStyle(FortPalette.body)
                Button { selectedAgent = agent; selected = .assign } label: {
                    Text("Assign work").frame(minHeight: 44)
                }
                    .buttonStyle(.bordered).tint(FortPalette.brass)
            }
        }
    }

    private func metricCard(_ metric: AgentMetrics) -> some View {
        FortDeckCard(accent: trendColor(metric.trend)) {
            VStack(alignment: .leading, spacing: 13) {
                HStack { Text(metric.agent).font(.headline); Spacer(); Text(trendLabel(metric.trend)).font(.caption.weight(.semibold)).foregroundStyle(trendColor(metric.trend)) }
                HStack(alignment: .top, spacing: 18) {
                    metricValue("\(Int(metric.firstPassPct.rounded()))%", "first-pass\naccepted")
                    metricValue(String(format: "%.2f", metric.redirectsPerAssignment), "redirects /\nassignment")
                    metricValue(metric.costKnown ? String(format: "$%.2f", metric.costPerAccepted) : "—", "per accepted\ncheckpoint")
                }
                Text("\(metric.firstPass) of \(metric.decided) sign-offs accepted first pass")
                    .font(.caption).foregroundStyle(FortPalette.muted)
                if let best = metric.best.first { Text("best at: \(best)").deckChip(color: FortPalette.accepted) }
                if let weak = metric.weak.first { Text("weak: \(weak)").deckChip(color: FortPalette.faint) }
            }
        }
    }

    private func scheduleCard(agent: String, includeQueued: Bool) -> some View {
        let active = board.runs.filter { $0.agent == agent && ["running", "blocked"].contains($0.status) }
        let queued = backlog.filter { ($0.agent ?? "") == agent }
        return FortDeckCard {
            VStack(alignment: .leading, spacing: 9) {
                Text(agent).font(.headline)
                ForEach(active) { run in
                    scheduleBlock(title(run), state: FortProjectState.resolve(run: run, gates: board.gates))
                }
                if includeQueued {
                    ForEach(queued) { item in scheduleBlock(item.title, state: .idle) }
                }
                if active.isEmpty && (!includeQueued || queued.isEmpty) {
                    Text("open capacity — assign work").font(.callout.italic()).foregroundStyle(FortPalette.faint)
                }
            }
        }
    }

    private func scheduleBlock(_ text: String, state: FortProjectState) -> some View {
        Text(text).font(.caption.weight(.semibold)).foregroundStyle(state == .idle ? FortPalette.body : FortPalette.page)
            .lineLimit(1).padding(.horizontal, 10).padding(.vertical, 8).frame(maxWidth: .infinity, alignment: .leading)
            .background(state == .idle ? FortPalette.queued : state.color, in: RoundedRectangle(cornerRadius: 7))
    }

    private func agentChip(_ agent: String, label: String) -> some View {
        Button(label) { selectedAgent = agent }
            .font(.caption.weight(.semibold))
            .foregroundStyle(selectedAgent == agent ? FortPalette.brassBright : FortPalette.muted)
            .padding(.horizontal, 11).frame(minHeight: 44)
            .background(selectedAgent == agent ? FortPalette.brass.opacity(0.12) : FortPalette.panel, in: Capsule())
            .overlay(Capsule().stroke(selectedAgent == agent ? FortPalette.brass : FortPalette.outline))
    }

    private func sectionTitle(_ title: String, count: Int) -> some View {
        HStack { Text(title).sectionLabel(); Text("\(count)").font(.caption.monospaced()).foregroundStyle(FortPalette.faint); Spacer() }
    }

    private func emptyCard(_ title: String, _ body: String) -> some View {
        FortDeckCard { VStack(alignment: .leading, spacing: 5) { Text(title).font(.headline); Text(body).font(.callout).foregroundStyle(FortPalette.muted) } }
    }

    private func errorCard(_ message: String) -> some View {
        FortDeckCard(accent: FortPalette.failed) { Label(message, systemImage: "exclamationmark.triangle").font(.callout).foregroundStyle(FortPalette.failed) }
    }

    private func metricValue(_ value: String, _ label: String) -> some View {
        VStack(alignment: .leading, spacing: 4) {
            Text(value).font(.title3.monospacedDigit().weight(.semibold))
            Text(label).font(.caption2).foregroundStyle(FortPalette.faint)
        }
    }

    private func conversationMessage(
        name: String,
        detail: String,
        body: String,
        model: String? = nil,
        state: FortProjectState,
        role: ConversationMessageRole
    ) -> some View {
        HStack(alignment: .top, spacing: 11) {
            if role == .human {
                HumanConversationAvatar(size: 34)
            } else {
                FortAgentOrbView(name: name, state: state, size: 34)
            }
            VStack(alignment: .leading, spacing: 5) {
                HStack(spacing: 7) {
                    Text(name).font(.caption.weight(.semibold))
                    if let model, !model.isEmpty {
                        Text(model)
                            .font(.caption2.monospaced())
                            .foregroundStyle(FortPalette.muted)
                            .padding(.horizontal, 6).padding(.vertical, 2)
                            .background(FortPalette.raised.opacity(0.35), in: Capsule())
                    }
                    Text(detail).font(.caption2.monospaced()).foregroundStyle(FortPalette.faint)
                }
                Text(body)
                    .font(.callout)
                    .foregroundStyle(FortPalette.body)
                    .fixedSize(horizontal: false, vertical: true)
                    .textSelection(.enabled)
            }
        }
    }

    private func conversationGateCard(_ gate: GateItem) -> some View {
        VStack(alignment: .leading, spacing: 11) {
            HStack(spacing: 7) {
                Image(systemName: "checkmark.seal.fill")
                Text("Work is paused until you approve")
                    .font(.callout.weight(.semibold))
                Spacer()
            }
            .foregroundStyle(FortPalette.needsYou)

            Text("Needs approval")
                .deckChip(color: FortPalette.needsYou)

            Text(gate.input?.isEmpty == false ? gate.input! : "Review this checkpoint, then approve it or request a specific change.")
                .font(.callout).foregroundStyle(FortPalette.body)
                .fixedSize(horizontal: false, vertical: true)
        }
        .padding(13)
        .background(FortPalette.needsYou.opacity(0.08), in: RoundedRectangle(cornerRadius: 12))
        .overlay(RoundedRectangle(cornerRadius: 12).stroke(FortPalette.needsYou.opacity(0.7)))
    }

    private func conversationGateDock(_ gate: GateItem) -> some View {
        VStack(spacing: 8) {
            HStack(spacing: 8) {
                Image(systemName: "checkmark.seal.fill")
                Text(deciding.contains(gate.id) ? "Recording your decision…" : "Your sign-off is required")
                    .font(.caption.weight(.semibold))
                Spacer(minLength: 0)
                Text("Paused").deckChip(color: FortPalette.needsYou)
            }
            .foregroundStyle(FortPalette.needsYou)

            Button { Task { await decide(gate, decision: "approve") } } label: {
                Text("Approve & continue")
                    .foregroundStyle(FortPalette.page)
                    .frame(maxWidth: .infinity, minHeight: 44)
            }
            .buttonStyle(.borderedProminent).tint(FortPalette.accepted)

            Button { redirectGate = gate } label: {
                Text("Request changes").frame(maxWidth: .infinity, minHeight: 44)
            }
            .buttonStyle(.bordered).tint(FortPalette.needsYou)
        }
        .disabled(deciding.contains(gate.id))
        .padding(.horizontal, 10).padding(.vertical, 9)
        .background(FortPalette.canvas)
        .overlay(alignment: .top) { Rectangle().fill(FortPalette.needsYou.opacity(0.65)).frame(height: 1) }
    }

    private func promotionCard(_ run: RunSummary) -> some View {
        FortDeckCard(accent: FortPalette.working) {
            VStack(alignment: .leading, spacing: 10) {
                HStack(spacing: 10) {
                    FortAgentOrbView(name: "Fort", state: .idle, size: 28)
                    VStack(alignment: .leading, spacing: 2) {
                        Text("Turn this into work").font(.callout.weight(.semibold)).foregroundStyle(FortPalette.working)
                        Text("Create one routed assignment from this finished conversation.")
                            .font(.caption).foregroundStyle(FortPalette.muted)
                    }
                }
                Button { promoteConversation(run) } label: {
                    Label("Assign work", systemImage: "arrow.right.circle.fill")
                        .frame(maxWidth: .infinity, minHeight: 44)
                }
                .buttonStyle(.bordered).tint(FortPalette.working)
            }
        }
    }

    private func conversationProgress(_ run: RunSummary) -> some View {
        let activity = conversationActivity(run)
        return FortDeckCard {
            VStack(alignment: .leading, spacing: 11) {
                HStack {
                    Text("ASSIGNMENT").sectionLabel(color: FortPalette.faint)
                    Spacer()
                    Text(activity.label).deckChip(color: activity.projectState.color)
                }
                FortCheckpointBar(run.checkpoints)
                LazyVGrid(columns: [GridItem(.flexible()), GridItem(.flexible())], alignment: .leading, spacing: 10) {
                    progressDatum("Agent", conversationAgent(run))
                    progressDatum("Model", run.model?.isEmpty == false ? run.model! : "Configured default")
                    progressDatum("Machine", run.machine ?? "Fort placed")
                    progressDatum("Elapsed", FortTime.elapsed(run.createdAt))
                }
            }
        }
    }

    private func progressDatum(_ label: String, _ value: String) -> some View {
        VStack(alignment: .leading, spacing: 3) {
            Text(label.uppercased()).font(.caption2.weight(.semibold)).foregroundStyle(FortPalette.faint)
            Text(value).font(.caption.weight(.medium)).foregroundStyle(FortPalette.body).lineLimit(2)
        }
    }

    private func conversationActivityTimeline(_ run: RunSummary) -> some View {
        let events = meaningfulConversationEvents(for: run)
        let activity = conversationActivity(run)
        return FortDeckCard {
            VStack(alignment: .leading, spacing: 9) {
                HStack {
                    Text("ACTIVITY").sectionLabel(color: FortPalette.faint)
                    Spacer()
                    Text("durable event log").font(.caption2).foregroundStyle(FortPalette.faint)
                }
                if events.isEmpty {
                    HStack(spacing: 8) {
                        Circle().stroke(activity.projectState.color, lineWidth: 1.3).frame(width: 9, height: 9)
                        Text(activity.label).font(.caption.weight(.semibold)).foregroundStyle(activity.projectState.color)
                        Text("No execution evidence yet.").font(.caption).foregroundStyle(FortPalette.faint)
                    }
                } else {
                    ForEach(Array(events.suffix(8))) { event in
                        conversationTimelineRow(event)
                    }
                }
            }
        }
    }

    private func conversationTimelineRow(_ event: Event) -> some View {
        HStack(alignment: .top, spacing: 9) {
            Image(systemName: timelineIcon(event))
                .font(.caption.weight(.semibold))
                .foregroundStyle(timelineColor(event))
                .frame(width: 17, height: 18)
            VStack(alignment: .leading, spacing: 2) {
                Text(timelineCopy(event))
                    .font(.caption).foregroundStyle(event.type.lowercased() == "error" ? FortPalette.failed : FortPalette.body)
                    .fixedSize(horizontal: false, vertical: true)
                    .textSelection(.enabled)
                Text(FortTime.relative(event.time))
                    .font(.caption2.monospaced()).foregroundStyle(FortPalette.faint)
            }
            Spacer(minLength: 0)
        }
    }

    private var conversationRuns: [RunSummary] {
        FortConversationOrdering.newestFirst(board.runs, gates: board.gates, events: conversationEvents)
    }
    private var selectedConversation: RunSummary? {
        guard !selectedConversationID.isEmpty, selectedConversationID != newConversationID else { return nil }
        return board.runs.first { $0.id == selectedConversationID }
    }
    private var selectedConversationGate: GateItem? {
        guard let run = selectedConversation else { return nil }
        return board.gates.first { $0.runID == run.id }
    }
    private var isConversationScreen: Bool {
        selected == .newConversation || selected == .conversation
    }
    private var mainNavigationHidden: Bool {
        selected == .deck || isConversationScreen
    }
    private var navigationTitle: String {
        if isConversationScreen { return selectedConversation.map(title) ?? "New conversation" }
        return selected == .deck ? "FORT" : selected.title
    }
    private var workingRuns: [RunSummary] {
        board.runs.filter { conversationActivity($0) == .working }
    }
    private var failedRuns: [RunSummary] {
        FortAttention.recentFailures(in: board.runs, gates: board.gates)
    }
    private var attentionHeadline: String {
        if board.gates.isEmpty && failedRuns.isEmpty { return "Everything is moving." }
        if failedRuns.isEmpty {
            return "\(board.gates.count) sign-off\(board.gates.count == 1 ? "" : "s") waiting"
        }
        if board.gates.isEmpty {
            return "\(failedRuns.count) assignment\(failedRuns.count == 1 ? "" : "s") need attention"
        }
        return "\(board.gates.count) sign-off\(board.gates.count == 1 ? "" : "s") and \(failedRuns.count) failure\(failedRuns.count == 1 ? "" : "s")"
    }
    private var attentionEmptyMessage: String {
        guard !workingRuns.isEmpty else { return "All quiet — nothing needs you." }
        return "That's everything — \(workingRuns.count) crew member\(workingRuns.count == 1 ? " is" : "s are") working and \(workingRuns.count == 1 ? "doesn't" : "don't") need you."
    }
    private var crewNames: [String] {
        let runAgents = board.runs.map(\.agent).filter { !$0.isEmpty && !$0.hasPrefix("flow:") }
        let machineAgents = machines.flatMap { $0.agents ?? [] }
        return Array(Set(runAgents + machineAgents)).sorted()
    }
    private var availableAgents: [String] {
        Array(Set(profiles.map(\.agent).filter { !$0.isEmpty })).sorted()
    }
    private var profilesForSelectedAgent: [ProfileOption] {
        let filtered = selectedAgent.isEmpty ? profiles : profiles.filter { $0.agent == selectedAgent }
        return filtered.sorted {
            if profileIsReady($0) != profileIsReady($1) { return profileIsReady($0) }
            return $0.displayName < $1.displayName
        }
    }
    private var selectedProfile: ProfileOption? {
        profiles.first { $0.id == selectedProfileID }
    }
    private var selectedProfileIsUnavailable: Bool {
        !selectedProfileID.isEmpty && selectedProfile.map(profileIsReady) != true
    }
    private var composerSelectionIsInvalid: Bool {
        selectedProfileIsUnavailable || (!selectedAgent.isEmpty && selectedProfile == nil)
    }
    private var profileMachineNames: [String] {
        if let selectedProfile, profileIsReady(selectedProfile) {
            return selectedProfile.machines.sorted()
        }
        return machines.filter(\.reachable).map(\.name).sorted()
    }
    private var availableMachineNames: [String] {
        var names = profileMachineNames
        if !selectedMachine.isEmpty && !names.contains(selectedMachine) { names.append(selectedMachine) }
        return names
    }
    private var fortOrbState: FortProjectState {
        workingRuns.isEmpty ? .idle : .working
    }
    private func agentState(_ agent: String) -> FortProjectState {
        let runs = board.runs.filter { $0.agent == agent }
        if runs.contains(where: { run in board.gates.contains { $0.runID == run.id } }) { return .needsYou }
        if runs.contains(where: { conversationActivity($0) == .working }) { return .working }
        return .idle
    }
    private func title(_ run: RunSummary) -> String { run.title.isEmpty ? run.id : run.title }
    private func conversationAgent(_ run: RunSummary) -> String {
        run.agent.hasPrefix("flow:") ? "Fort" : (run.agent.isEmpty ? "Fort" : run.agent.capitalized)
    }
    private func gateTitle(_ gate: GateItem) -> String {
        gate.nodeID.replacingOccurrences(of: "_", with: " ").replacingOccurrences(of: "-", with: " ").capitalized
    }
    private func activityLine(_ run: RunSummary) -> String {
        conversationActivity(run).label
    }
    private func conversationActivity(_ run: RunSummary) -> FortConversationActivity {
        FortConversationActivity.resolve(run: run, gates: board.gates, events: conversationEvents)
    }
    private func conversationTimestamp(_ run: RunSummary) -> String {
        var candidates = [run.updatedAt, run.createdAt].compactMap { $0 }
        candidates.append(contentsOf: conversationEvents.lazy.filter { $0.runID == run.id }.map(\.time))
        candidates.append(contentsOf: board.gates.lazy.filter { $0.runID == run.id }.compactMap(\.since))
        let latest = candidates.max { parseEventDate($0) < parseEventDate($1) }
        return FortTime.relative(latest)
    }
    private func gatesForConversation(_ run: RunSummary) -> [GateItem] {
        board.gates.filter { $0.runID == run.id }
    }
    private func conversationPrompt(_ run: RunSummary) -> String {
        let body = run.body?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
        return body.isEmpty ? title(run) : body
    }
    private func conversationResponse(_ run: RunSummary) -> String {
        if let failure = exactFailureReason(for: run) { return failure }
        if let message = meaningfulConversationEvents(for: run)
            .last(where: { $0.type.lowercased() == "message" })?.data?
            .trimmingCharacters(in: .whitespacesAndNewlines), !message.isEmpty {
            return embeddedErrorMessage(message) ?? message
        }
        switch conversationActivity(run) {
        case .starting: return "Fort accepted this conversation and is waiting for the first execution event."
        case .working: return "Work is active. The persisted activity below shows what is happening."
        case .pausedForReview: return "Work is paused at a checkpoint and needs your decision below."
        case .paused: return "This conversation is paused."
        case .finished: return "This conversation finished. Its durable activity is below."
        case .failed: return "This conversation failed. Open the latest activity for the recorded reason."
        case .canceled: return "This conversation was canceled."
        case .ready: return "This conversation is ready."
        }
    }
    private func profileIsReady(_ profile: ProfileOption) -> Bool { profile.state == "ready" }
    private func modelOptionLabel(_ profile: ProfileOption) -> String {
        let model = profile.model?.isEmpty == false ? profile.model! : "Configured default"
        guard !profileIsReady(profile) else { return model }
        let reason = profile.reason?.isEmpty == false ? profile.reason! : profile.state
        return "\(model) — \(reason.replacingOccurrences(of: "_", with: " "))"
    }
    private func selectDefaultProfileForAgent() {
        guard !selectedAgent.isEmpty else {
            selectedProfileID = ""
            selectedMachine = ""
            return
        }
        if let selectedProfile, selectedProfile.agent == selectedAgent, profileIsReady(selectedProfile) { return }
        selectedProfileID = profiles.first { $0.agent == selectedAgent && profileIsReady($0) }?.id ?? ""
        if !profileMachineNames.contains(selectedMachine) { selectedMachine = "" }
    }
    private func profileSelectionChanged() {
        if let selectedProfile {
            selectedAgent = selectedProfile.agent
            if !selectedProfile.machines.contains(selectedMachine) { selectedMachine = "" }
        } else if selectedProfileID.isEmpty {
            selectedMachine = ""
        }
    }
    private func applyConversationSelection(_ run: RunSummary) {
        selectedAgent = run.agent.hasPrefix("flow:") ? "" : run.agent
        selectedMachine = run.machine ?? ""
        if let profile = run.profile, profiles.contains(where: { $0.id == profile }) {
            selectedProfileID = profile
            selectedAgent = profiles.first(where: { $0.id == profile })?.agent ?? selectedAgent
            return
        }
        let exact = profiles.first {
            $0.agent == run.agent && ($0.model ?? "") == (run.model ?? "") && profileIsReady($0)
        }
        selectedProfileID = exact?.id ?? ""
    }
    private func meaningfulConversationEvents(for run: RunSummary) -> [Event] {
        conversationEvents
            .filter { event in
                guard event.runID == run.id else { return false }
                switch event.type.lowercased() {
                case "placement", "started", "stderr", "tool", "subagent", "message", "gate", "error", "exited":
                    return true
                case "stdout":
                    return structuredStdoutType(event.data) != nil
                default:
                    return false
                }
            }
            .sorted {
                let left = parseEventDate($0.time)
                let right = parseEventDate($1.time)
                return left == right ? $0.id < $1.id : left < right
            }
    }
    private func structuredStdoutType(_ data: String?) -> String? {
        guard let type = jsonObject(data)?["type"] as? String,
              ["turn.started", "turn.completed"].contains(type) else { return nil }
        return type
    }
    private func timelineCopy(_ event: Event) -> String {
        let kind = event.type.lowercased()
        let object = jsonObject(event.data)
        let data = event.data?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
        switch kind {
        case "placement":
            let agent = object?["agent"] as? String
            let machine = object?["machine"] as? String
            if let agent, let machine { return "Placed \(agent) on \(machine)" }
            if let machine { return "Placed on \(machine)" }
            return data.isEmpty ? "Placement resolved" : data
        case "started": return data.isEmpty ? "Execution started" : "\(data.capitalized) started"
        case "stdout": return structuredStdoutType(event.data) == "turn.completed" ? "Turn completed" : "Turn started"
        case "stderr": return data.isEmpty ? "Provider diagnostic output" : data
        case "tool":
            let name = (object?["name"] as? String)?.trimmingCharacters(in: .whitespacesAndNewlines)
            let summary = (object?["summary"] as? String)?.trimmingCharacters(in: .whitespacesAndNewlines)
            if let name, !name.isEmpty, let summary, !summary.isEmpty { return "\(name) — \(summary)" }
            return name?.isEmpty == false ? name! : (data.isEmpty ? "Tool activity" : data)
        case "subagent": return data.isEmpty ? "Subagent activity" : data
        case "message": return embeddedErrorMessage(event.data) ?? (data.isEmpty ? "Agent message" : data)
        case "gate":
            let decision = (object?["decision"] as? String)?.lowercased()
            let note = (object?["note"] as? String)?.trimmingCharacters(in: .whitespacesAndNewlines)
            if decision == "approved" { return "Review approved" }
            if decision == "rejected", let note, !note.isEmpty { return "Changes requested — \(note)" }
            if decision == "rejected" { return "Changes requested" }
            return data.isEmpty ? "Review checkpoint" : data
        case "error": return data.isEmpty ? "Execution failed without a recorded reason" : data
        case "exited": return (event.code ?? 0) == 0 ? "Execution exited successfully" : "Execution exited with code \(event.code ?? 0)"
        default: return data.isEmpty ? event.type : data
        }
    }
    private func timelineIcon(_ event: Event) -> String {
        switch event.type.lowercased() {
        case "placement": return "desktopcomputer"
        case "started": return "play.fill"
        case "stdout": return structuredStdoutType(event.data) == "turn.completed" ? "checkmark.circle.fill" : "sparkles"
        case "stderr": return "waveform.path.ecg"
        case "tool": return "wrench.and.screwdriver.fill"
        case "subagent": return "person.2.fill"
        case "message": return embeddedErrorMessage(event.data) == nil ? "text.bubble.fill" : "exclamationmark.triangle.fill"
        case "gate": return "checkmark.seal.fill"
        case "error": return "exclamationmark.triangle.fill"
        case "exited": return (event.code ?? 0) == 0 ? "checkmark.circle.fill" : "xmark.circle.fill"
        default: return "circle.fill"
        }
    }
    private func timelineColor(_ event: Event) -> Color {
        switch event.type.lowercased() {
        case "error", "message" where embeddedErrorMessage(event.data) != nil: return FortPalette.failed
        case "stderr", "gate": return FortPalette.needsYou
        case "exited" where (event.code ?? 0) != 0: return FortPalette.failed
        case "exited": return FortPalette.accepted
        case "stdout" where structuredStdoutType(event.data) == "turn.completed": return FortPalette.accepted
        default: return FortPalette.working
        }
    }
    private func exactFailureReason(for run: RunSummary) -> String? {
        let events = meaningfulConversationEvents(for: run)
        for event in events.reversed() {
            let data = event.data?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
            if event.type.lowercased() == "error", !data.isEmpty { return data }
            if event.type.lowercased() == "message", let message = embeddedErrorMessage(event.data) { return message }
        }
        return nil
    }
    private func embeddedErrorMessage(_ data: String?) -> String? {
        guard let object = jsonObject(data), (object["type"] as? String)?.lowercased() == "error" else { return nil }
        if let error = object["error"] as? [String: Any],
           let message = error["message"] as? String,
           !message.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty { return message }
        if let message = object["message"] as? String,
           !message.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty { return message }
        return nil
    }
    private func jsonObject(_ data: String?) -> [String: Any]? {
        guard let data, let bytes = data.data(using: .utf8) else { return nil }
        return try? JSONSerialization.jsonObject(with: bytes) as? [String: Any]
    }
    private func parseEventDate(_ value: String) -> Date {
        let formatter = ISO8601DateFormatter()
        if let date = formatter.date(from: value) { return date }
        formatter.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        return formatter.date(from: value) ?? .distantPast
    }
    private func trendColor(_ trend: String) -> Color {
        trend == "improving" ? FortPalette.accepted : (trend == "slipping" ? FortPalette.failed : FortPalette.muted)
    }
    private func trendLabel(_ trend: String) -> String {
        trend == "improving" ? "▲ improving" : (trend == "slipping" ? "▼ slipping" : "→ steady")
    }

    private var selectedPlaybook: Playbook? {
        if let selectedPlaybookID,
           let selected = playbooks.first(where: { $0.id == selectedPlaybookID }),
           (handoffMode == .quickQuestion) == (selected.delivery == "answer") {
            return selected
        }
        return handoffMode == .quickQuestion ? FortPlaybookRouting.quickAnswer(in: playbooks) : nil
    }

    private var quickModeUnavailable: Bool {
        handoffMode == .quickQuestion && selectedPlaybook == nil
    }

    private var routePreviewKey: String {
        [
            client.baseURL.absoluteString,
            selected.rawValue,
            handoffMode.rawValue,
            selectedPlaybookID ?? "automatic",
            selectedPlaybook?.revision.description ?? "latest",
            proposePlan.description,
            draft,
        ].joined(separator: "|")
    }

    private func routeRequest(for text: String) -> RouteRequest {
        RouteRequest(
            text: text,
            playbookID: selectedPlaybook?.id,
            playbookRevision: selectedPlaybook?.revision,
            taskType: handoffMode == .quickQuestion ? "question" : nil,
            planGate: handoffMode == .quickQuestion ? false : proposePlan
        )
    }

    private func refreshRoutePreview() async {
        guard selected == .assign else { return }
        let text = draft.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !text.isEmpty else { routePreview = nil; return }
        guard !quickModeUnavailable else { routePreview = nil; return }
        do { try await Task.sleep(nanoseconds: 350_000_000) } catch { return }
        guard !Task.isCancelled else { return }
        do { routePreview = try await client.route(routeRequest(for: text)) }
        catch { routePreview = nil }
    }

    private func selectPrimary(_ item: MobileDeckView) {
        if item == .newConversation {
            beginNewConversation()
            return
        }
        withAnimation(.easeOut(duration: 0.18)) { selected = item }
    }

    private func beginNewConversation() {
        selectedConversationID = newConversationID
        selected = .newConversation
        conversationDraft = ""
        conversationAnswer = nil
        conversationStatus = nil
        selectedAgent = ""
        selectedProfileID = ""
        selectedMachine = ""
    }

    private func selectConversation(_ run: RunSummary) {
        selectedConversationID = run.id
        selected = .conversation
        conversationAnswer = nil
        conversationStatus = nil
        applyConversationSelection(run)
        showConversationHistory = false
    }

    private func promoteConversation(_ run: RunSummary) {
        guard FortConversationPromotion.isEligible(run, gates: board.gates) else { return }
        draft = [title(run), run.body ?? ""]
            .filter { !$0.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty }
            .joined(separator: "\n")
        handoffMode = .assignment
        selectedPlaybookID = FortPlaybookRouting.defaultAssignment(in: playbooks)?.id
        routePreview = nil
        inlineAnswer = nil
        selected = .assign
    }

    private func beginConversationSend() {
        let text = conversationDraft.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !directSending, !text.isEmpty, !composerSelectionIsInvalid else { return }
        directSending = true
        conversationStatus = "Submitting to Fort…"
        Task { await sendConversation(text) }
    }

    private func beginAssignment() {
        let text = draft.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !sending, !text.isEmpty, !quickModeUnavailable else { return }
        sending = true
        notice = nil
        Task { await handOff(text) }
    }

    private func sendConversation(_ text: String) async {
        defer { directSending = false }
        do {
            let profile = selectedProfile
            let result = try await client.chat(ChatRequest(
                text: text,
                agent: profile?.agent,
                profile: profile?.id,
                machine: selectedMachine.isEmpty ? nil : selectedMachine
            ))
            conversationDraft = ""
            conversationStatus = nil
            switch result.handoffOutcome {
            case .answer(let answer):
                conversationAnswer = answer
                selectedConversationID = result.runID
                await reload()
                if let run = conversationRuns.first(where: { $0.id == result.runID }) {
                    selectConversation(run)
                }
            case .failure(let message):
                conversationAnswer = nil
                conversationStatus = message
            case .assignment:
                conversationAnswer = nil
                selectedConversationID = result.runID
                selected = .conversation
                await reload()
                if let run = conversationRuns.first(where: { $0.id == result.runID }) {
                    selectConversation(run)
                    await loadConversationEvents(for: run)
                } else {
                    conversationStatus = "Fort accepted run \(result.runID). Waiting for it to appear in the event log."
                }
            }
        } catch {
            conversationStatus = errorText(error)
        }
    }

    private func consumeConversationEvents() async {
        conversationEvents = []
        conversationEventCursor = 0
        while !Task.isCancelled {
            do {
                for try await event in client.events(since: conversationEventCursor) {
                    guard !Task.isCancelled else { return }
                    conversationEventCursor = max(conversationEventCursor, event.id)
                    guard !conversationEvents.contains(where: { $0.id == event.id }) else { continue }
                    conversationEvents.append(event)
                    conversationEvents.sort { $0.id < $1.id }
                }
            } catch is CancellationError {
                return
            } catch {
                // Reconnect from the durable cursor; no command is retried.
            }
            do { try await Task.sleep(nanoseconds: 750_000_000) } catch { return }
        }
    }

    private func loadConversationEvents(for run: RunSummary) async {
        do {
            let detail = try await client.runDetail(run.id)
            var byID = Dictionary(uniqueKeysWithValues: conversationEvents.map { ($0.id, $0) })
            for event in detail.events { byID[event.id] = event }
            conversationEvents = byID.values.sorted { $0.id < $1.id }
            conversationEventCursor = max(conversationEventCursor, conversationEvents.last?.id ?? 0)
        } catch {
            // The SSE stream remains authoritative; detail is replay recovery.
        }
    }

    private func runLoop() async {
        await reload()
        while !Task.isCancelled {
            try? await Task.sleep(nanoseconds: 3_000_000_000)
            if !Task.isCancelled { await reload() }
        }
    }

    private func reload() async {
        do {
            async let nextSummary = client.summary()
            async let nextBoard = client.board()
            async let nextBacklog = client.backlog()
            async let nextMachines = client.machines()
            async let nextMetrics = client.metrics()
            async let nextPlaybooks = client.playbooks()
            async let nextProfiles = client.profiles()
            let firstProfileLoad = profiles.isEmpty
            summary = try await nextSummary
            board = try await nextBoard
            backlog = try await nextBacklog
            machines = try await nextMachines
            metrics = try await nextMetrics
            playbooks = try await nextPlaybooks
            profiles = (try? await nextProfiles) ?? []
            if firstProfileLoad, let run = selectedConversation {
                applyConversationSelection(run)
            }
            if handoffMode == .quickQuestion {
                let currentIsAnswer = playbooks.contains { $0.id == selectedPlaybookID && $0.delivery == "answer" }
                if !currentIsAnswer { selectedPlaybookID = FortPlaybookRouting.quickAnswer(in: playbooks)?.id }
            }
#if targetEnvironment(simulator)
            if selectedConversationID.isEmpty {
                switch ProcessInfo.processInfo.environment["FORT_QA_SCREEN"] {
                case "new":
                    beginNewConversation()
                case "approval":
                    if let gate = board.gates.first,
                       let run = board.runs.first(where: { $0.id == gate.runID }) {
                        selectConversation(run)
                    }
                case "conversation":
                    if let run = conversationRuns.first { selectConversation(run) }
                default:
                    break
                }
            }
#endif
            loadError = nil
        } catch { loadError = errorText(error) }
    }

    private func decide(_ gate: GateItem, decision: String, note: String? = nil) async {
        deciding.insert(gate.id); defer { deciding.remove(gate.id) }
        do {
            let applied = try await client.decideGate(run: gate.runID, node: gate.nodeID, decision: decision, note: note)
            if !applied {
                let message = "No execution plane is attached, so this sign-off cannot be applied yet."
                if isConversationScreen { conversationStatus = message } else { notice = message }
            } else if isConversationScreen {
                conversationStatus = nil
            }
            await reload()
        } catch {
            if isConversationScreen { conversationStatus = errorText(error) }
            else { notice = errorText(error) }
        }
    }

    private func handOff(_ text: String) async {
        defer { sending = false }
        do {
            let resolved = try await client.route(routeRequest(for: text))
            let result = try await client.chat(ChatRequest(
                text: text,
                agent: selectedAgent.isEmpty ? nil : selectedAgent,
                playbookID: resolved.playbookID,
                playbookRevision: resolved.playbookRevision,
                taskType: resolved.taskType,
                planGate: resolved.delivery == "answer" ? false : resolved.planGate
            ))
            switch result.handoffOutcome {
            case .answer(let answer):
                inlineAnswer = answer
            case .failure(let message):
                inlineAnswer = nil
                notice = message
            case .assignment:
                draft = ""
                routePreview = nil
                inlineAnswer = nil
                selectedConversationID = result.runID
                selected = .conversation
            }
            await reload()
            if let run = conversationRuns.first(where: { $0.id == result.runID }) {
                selectConversation(run)
                await loadConversationEvents(for: run)
            } else if result.handoffOutcome == .assignment {
                conversationStatus = "Fort accepted run \(result.runID). Waiting for its first durable event."
            }
        } catch { notice = errorText(error) }
    }

    private func dispatch(_ item: BacklogItem) async {
        do { _ = try await client.dispatchBacklog(item.id); await reload() }
        catch { notice = errorText(error) }
    }
}

private enum ConversationMessageRole {
    case human
    case agent
}

private struct HumanConversationAvatar: View {
    let size: CGFloat

    var body: some View {
        ZStack {
            Circle().fill(FortPalette.raised.opacity(0.4))
            Circle().stroke(FortPalette.outline, lineWidth: 1.4)
            Image(systemName: "person.fill")
                .font(.system(size: size * 0.42, weight: .semibold))
                .foregroundStyle(FortPalette.body)
        }
        .frame(width: size, height: size)
        .accessibilityLabel("You")
    }
}

private struct RedirectSheet: View {
    @Environment(\.dismiss) private var dismiss
    let gate: GateItem
    let onSubmit: (String) -> Void
    @State private var note = ""

    var body: some View {
        NavigationStack {
            VStack(alignment: .leading, spacing: 14) {
                Text("Tell the crew member what should change before the next sign-off.").font(.callout).foregroundStyle(.secondary)
                TextEditor(text: $note).frame(minHeight: 130).padding(8)
                    .accessibilityLabel("Requested changes")
                    .background(Color.secondary.opacity(0.08), in: RoundedRectangle(cornerRadius: 10))
                Spacer()
            }
            .padding()
            .navigationTitle("Request changes")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) { Button("Cancel") { dismiss() } }
                ToolbarItem(placement: .confirmationAction) {
                    Button("Send") { onSubmit(note.trimmingCharacters(in: .whitespacesAndNewlines)); dismiss() }
                        .disabled(note.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)
                }
            }
        }
    }
}

struct StatusBadge: View {
    let status: String
    var body: some View {
        let state: FortProjectState = status == "running" ? .working : (["succeeded", "done"].contains(status) ? .delivered : (["failed", "error"].contains(status) ? .failed : .idle))
        FortStatusPill(state)
    }
}

struct RunDetailView: View {
    @EnvironmentObject private var client: FortClient
    let runID: String
    @State private var detail: RunDetail?
    @State private var loadError: String?

    var body: some View {
        List {
            if let detail {
                Section("Assignment") {
                    Text(detail.run.title.isEmpty ? detail.run.id : detail.run.title).font(.headline)
                    Text(detail.run.agent).foregroundStyle(.secondary)
                    FortCheckpointBar(detail.run.checkpoints)
                }
                Section("Checkpoints") {
                    ForEach(detail.nodes) { node in
                        HStack { Text(node.nodeID); Spacer(); StatusBadge(status: node.status) }
                    }
                }
                Section("Activity") { ForEach(detail.events) { EventRow(event: $0) } }
            } else if let loadError { Text(loadError).foregroundStyle(.red) }
            else { ProgressView() }
        }
        .navigationTitle("Assignment")
        .task { await load() }
    }

    private func load() async {
        do { detail = try await client.runDetail(runID); loadError = nil }
        catch { loadError = errorText(error) }
    }
}

private extension Text {
    func sectionLabel(color: Color = FortPalette.muted) -> some View {
        font(.caption.weight(.semibold)).tracking(1.2).foregroundStyle(color)
    }
    func deckChip(color: Color) -> some View {
        font(.caption2.weight(.semibold)).foregroundStyle(color).padding(.horizontal, 8).padding(.vertical, 4)
            .background(color.opacity(0.12), in: Capsule())
    }
}
