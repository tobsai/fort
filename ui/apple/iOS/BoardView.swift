import SwiftUI
import FortKit

private enum MobileDeckView: String, CaseIterable, Identifiable {
    case deck, assign, projects, today, more, playbooks, performance, week

    static let primary: [MobileDeckView] = [.deck, .assign, .projects, .today, .more]

    var id: String { rawValue }
    var title: String {
        switch self {
        case .deck: return "Deck"
        case .projects: return "Projects"
        case .assign: return "Direction"
        case .performance: return "Crew"
        case .week: return "Week"
        case .today: return "Today"
        case .more: return "More"
        case .playbooks: return "Playbooks"
        }
    }
    var icon: String {
        switch self {
        case .deck: return "rectangle.3.group"
        case .assign: return "paperplane.fill"
        case .projects: return "square.grid.2x2"
        case .today: return "clock"
        case .more: return "ellipsis"
        case .playbooks: return "link"
        case .performance: return "person.2"
        case .week: return "calendar"
        }
    }

    var primarySelection: MobileDeckView {
        MobileDeckView.primary.contains(self) ? self : .more
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
    @State private var metrics: MetricsResponse?
    @State private var playbooks: [Playbook] = []
    @State private var selected: MobileDeckView = .deck
    @State private var loadError: String?
    @State private var deciding: Set<String> = []
    @State private var redirectGate: GateItem?
    @State private var draft = ""
    @State private var selectedAgent = ""
    @State private var proposePlan = true
    @State private var handoffMode: MobileHandoffMode = .assignment
    @State private var routePreview: RoutePreview?
    @State private var selectedPlaybookID: String?
    @State private var inlineAnswer: String?
    @State private var showPlaybookPicker = false
    @State private var showFeed = false
    @State private var showSettings = false
    @State private var sending = false
    @State private var notice: String?

    var body: some View {
        ZStack {
            FortPalette.page.ignoresSafeArea()
            VStack(spacing: 0) {
                ScrollView {
                    content
                        .padding(16)
                }
                .refreshable { await reload() }
                if selected == .assign { handoffButton }
                primaryTabBar
            }
        }
        .foregroundStyle(FortPalette.primary)
        .navigationTitle(selected == .deck ? "FORT" : selected.title)
        .navigationBarTitleDisplayMode(.inline)
        .navigationDestination(for: String.self) { RunDetailView(runID: $0) }
        .task(id: client.baseURL) { await runLoop() }
        .task(id: routePreviewKey) { await refreshRoutePreview() }
        .sheet(item: $redirectGate) { gate in
            RedirectSheet(gate: gate) { note in
                Task { await decide(gate, decision: "reject", note: note) }
            }
            .presentationDetents([.medium])
        }
        .sheet(isPresented: $showPlaybookPicker) { playbookPicker }
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
        case .projects: projectsView
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

            sectionTitle("PROJECTS", count: projectRuns.count)
            ForEach(projectRuns.prefix(5)) { compactProject($0) }

            if !backlog.isEmpty {
                sectionTitle("UP NEXT", count: backlog.count)
                ForEach(backlog.prefix(4)) { upNextRow($0) }
            }

            sectionTitle("CREW", count: crewNames.count)
            ForEach(crewNames, id: \.self) { crewRow($0) }

            if let loadError { errorCard(loadError) }
        }
    }

    private var projectsView: some View {
        VStack(alignment: .leading, spacing: 14) {
            Text("PROJECT ROOMS").sectionLabel()
            ForEach(projectRuns) { run in
                NavigationLink(value: run.id) {
                    projectCard(run)
                }
                .buttonStyle(.plain)
            }
            ForEach(backlog) { upNextCard($0) }
            if projectRuns.isEmpty { emptyCard("No projects yet", "Give Fort direction to start one.") }
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
                        .frame(maxWidth: .infinity, minHeight: 40)
                        .background(handoffMode == mode ? FortPalette.raised : Color.clear, in: RoundedRectangle(cornerRadius: 8))
                    }
                    .buttonStyle(.plain)
                }
            }
            .padding(3)
            .background(FortPalette.panel, in: RoundedRectangle(cornerRadius: 10))
            .overlay(RoundedRectangle(cornerRadius: 10).stroke(FortPalette.line))

            TextEditor(text: $draft)
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
            Text("Switch a route from Give direction. Editing stays on desktop.")
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
                    withAnimation(.easeOut(duration: 0.18)) { selected = item }
                } label: {
                    VStack(spacing: 4) {
                        if item == .assign {
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

    private var handoffButton: some View {
        Button { Task { await handOff() } } label: {
            HStack {
                if sending { ProgressView().tint(FortPalette.page) }
                Text("Hand it off").font(.body.weight(.semibold))
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
                    .lineLimit(5)
                HStack(spacing: 8) {
                    Button { Task { await decide(gate, decision: "approve") } } label: {
                        Text("Accept").frame(maxWidth: .infinity, minHeight: 44)
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
                Text("\(run.agent) stopped. Open the assignment to see what happened and give direction.")
                    .font(.callout).foregroundStyle(FortPalette.body)
                NavigationLink(value: run.id) {
                    Text("View what happened").frame(minHeight: 44)
                }
                .buttonStyle(.bordered)
                .tint(FortPalette.failed)
            }
        }
    }

    private func compactProject(_ run: RunSummary) -> some View {
        let state = FortProjectState.resolve(run: run, gates: board.gates)
        return NavigationLink(value: run.id) {
            HStack(spacing: 12) {
                FortSigilView(name: title(run), state: state, size: 36)
                VStack(alignment: .leading, spacing: 4) {
                    Text(title(run)).font(.callout.weight(.semibold)).foregroundStyle(FortPalette.primary).lineLimit(1)
                    Text(run.checkpoints?.deckCaption ?? activityLine(run))
                        .font(.caption).foregroundStyle(FortPalette.muted).lineLimit(1)
                }
                Spacer()
                Image(systemName: "chevron.right").font(.caption).foregroundStyle(FortPalette.faint)
            }
            .frame(minHeight: 44)
        }
        .buttonStyle(.plain)
    }

    private func projectCard(_ run: RunSummary) -> some View {
        let state = FortProjectState.resolve(run: run, gates: board.gates)
        return FortDeckCard(accent: state == .needsYou ? state.color : nil) {
            VStack(alignment: .leading, spacing: 13) {
                HStack(spacing: 12) {
                    FortSigilView(name: title(run), state: state, size: 46)
                    VStack(alignment: .leading, spacing: 3) {
                        Text(title(run)).font(.headline).foregroundStyle(FortPalette.primary)
                        Text([run.agent, run.machine].compactMap { $0 }.filter { !$0.isEmpty }.joined(separator: " · "))
                            .font(.caption).foregroundStyle(FortPalette.muted)
                    }
                    Spacer()
                    FortStatusPill(state)
                }
                FortCheckpointBar(run.checkpoints)
                Text(run.checkpoints?.deckCaption ?? activityLine(run))
                    .font(.caption).foregroundStyle(FortPalette.muted)
                if let body = run.body, !body.isEmpty {
                    Text(body).font(.callout).foregroundStyle(FortPalette.body).lineLimit(3)
                }
            }
        }
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

    private var projectRuns: [RunSummary] {
        board.runs.sorted { stateRank($0) < stateRank($1) }
    }
    private var workingRuns: [RunSummary] { board.runs.filter { $0.status == "running" } }
    private var failedRuns: [RunSummary] {
        board.runs
            .filter { ["failed", "error"].contains($0.status.lowercased()) }
            .sorted { ($0.updatedAt ?? $0.createdAt ?? "") > ($1.updatedAt ?? $1.createdAt ?? "") }
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
        let runAgents = board.runs.map(\.agent).filter { !$0.isEmpty }
        let machineAgents = machines.flatMap { $0.agents ?? [] }
        return Array(Set(runAgents + machineAgents)).sorted()
    }
    private func stateRank(_ run: RunSummary) -> Int {
        switch FortProjectState.resolve(run: run, gates: board.gates) {
        case .needsYou: return 0
        case .failed: return 1
        case .working: return 2
        case .idle: return 3
        case .delivered: return 4
        }
    }
    private func agentState(_ agent: String) -> FortProjectState {
        let runs = board.runs.filter { $0.agent == agent }
        if runs.contains(where: { run in board.gates.contains { $0.runID == run.id } }) { return .needsYou }
        if runs.contains(where: { $0.status == "running" }) { return .working }
        return .idle
    }
    private func title(_ run: RunSummary) -> String { run.title.isEmpty ? run.id : run.title }
    private func gateTitle(_ gate: GateItem) -> String {
        gate.nodeID.replacingOccurrences(of: "_", with: " ").replacingOccurrences(of: "-", with: " ").capitalized
    }
    private func activityLine(_ run: RunSummary) -> String {
        switch FortProjectState.resolve(run: run, gates: board.gates) {
        case .needsYou: return "awaiting your sign-off"
        case .working: return "\(run.agent) working · \(FortTime.elapsed(run.createdAt))"
        case .delivered: return "all accepted"
        case .failed: return "needs attention"
        case .idle: return run.status == "queued" ? "up next" : "idle"
        }
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
            summary = try await nextSummary
            board = try await nextBoard
            backlog = try await nextBacklog
            machines = try await nextMachines
            metrics = try await nextMetrics
            playbooks = try await nextPlaybooks
            if handoffMode == .quickQuestion {
                let currentIsAnswer = playbooks.contains { $0.id == selectedPlaybookID && $0.delivery == "answer" }
                if !currentIsAnswer { selectedPlaybookID = FortPlaybookRouting.quickAnswer(in: playbooks)?.id }
            }
            loadError = nil
        } catch { loadError = errorText(error) }
    }

    private func decide(_ gate: GateItem, decision: String, note: String? = nil) async {
        deciding.insert(gate.id); defer { deciding.remove(gate.id) }
        do {
            let applied = try await client.decideGate(run: gate.runID, node: gate.nodeID, decision: decision, note: note)
            if !applied { notice = "No execution plane is attached, so this sign-off cannot be applied yet." }
            await reload()
        } catch { notice = errorText(error) }
    }

    private func handOff() async {
        let text = draft.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !text.isEmpty else { return }
        guard !quickModeUnavailable else {
            notice = "Quick question needs an answer playbook before Fort can hand it off."
            return
        }
        sending = true; defer { sending = false }
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
            case .assignment where handoffMode == .quickQuestion || resolved.delivery == "answer":
                inlineAnswer = nil
                notice = "Quick answer did not return an inline answer. Check run \(result.runID) for the failure."
            case .assignment:
                draft = ""
                routePreview = nil
                inlineAnswer = nil
                notice = resolved.planGate ? "Fort is drafting the project plan." : "The assignment is underway."
                selected = .deck
            }
            await reload()
        } catch { notice = errorText(error) }
    }

    private func dispatch(_ item: BacklogItem) async {
        do { _ = try await client.dispatchBacklog(item.id); await reload() }
        catch { notice = errorText(error) }
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
