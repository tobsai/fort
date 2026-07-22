import SwiftUI
import Combine
import FortKit

private enum MacDeckRoute: String, CaseIterable, Identifiable {
    case deck, projects, today, crew, playbooks, assign, week
    static let primary: [MacDeckRoute] = [.deck, .projects, .today, .crew, .playbooks]
    var id: String { rawValue }
    var title: String {
        switch self {
        case .deck: return "Command Deck"
        case .crew: return "Crew"
        case .playbooks: return "Playbooks"
        case .assign: return "Give direction"
        default: return rawValue.capitalized
        }
    }
    var icon: String {
        switch self {
        case .deck: return "rectangle.3.group"
        case .projects: return "square.grid.2x2"
        case .assign: return "paperplane"
        case .crew: return "person.2"
        case .playbooks: return "link"
        case .week: return "calendar"
        case .today: return "clock"
        }
    }
}

struct FortWindow: View {
    @EnvironmentObject private var client: FortClient
    @EnvironmentObject private var service: ServiceController

    @State private var route: MacDeckRoute? = .deck
    @State private var summary: Summary?
    @State private var board = Board(runs: [], gates: [])
    @State private var backlog: [BacklogItem] = []
    @State private var machines: [MachineSummary] = []
    @State private var metrics: MetricsResponse?
    @State private var playbooks: [Playbook] = []
    @State private var composeText = ""
    @State private var selectedAgent = ""
    @State private var proposePlan = true
    @State private var handoffPlaybookID: String?
    @State private var routePreview: RoutePreview?
    @State private var inlineAnswer: String?
    @State private var showHandoffPlaybookPicker = false
    @State private var savingPlaybooks: Set<String> = []
    @State private var busy = false
    @State private var lastError: String?
    @State private var decidingGates: Set<String> = []
    @State private var redirectGate: GateItem?
    @State private var selectedRun: RunSummary?

    private let refresh = Timer.publish(every: 3, on: .main, in: .common).autoconnect()

    var body: some View {
        NavigationSplitView {
            sidebar.frame(minWidth: 205)
        } detail: {
            ZStack {
                FortPalette.page.ignoresSafeArea()
                VStack(spacing: 0) {
                    topBar
                    detail
                }
            }
            .foregroundStyle(FortPalette.primary)
        }
        .preferredColorScheme(.dark)
        .tint(FortPalette.brass)
        .task { await service.refresh(); await reload() }
        .task(id: routePreviewKey) { await refreshRoutePreview() }
        .onReceive(refresh) { _ in Task { await reload() } }
        .sheet(item: $redirectGate) { gate in
            MacRedirectSheet(gate: gate) { note in Task { await decide(gate, "reject", note: note) } }
        }
        .sheet(item: $selectedRun) { run in
            MacRunDetailSheet(run: run).environmentObject(client)
        }
        .sheet(isPresented: $showHandoffPlaybookPicker) { handoffPlaybookPicker }
    }

    private var sidebar: some View {
        List(selection: $route) {
            Section("Command") {
                ForEach(MacDeckRoute.primary) { item in
                    Label(item.title, systemImage: item.icon).tag(Optional(item))
                }
            }
            Section("Service") {
                HStack {
                    Circle().fill(service.status.running ? FortPalette.accepted : FortPalette.faint).frame(width: 8, height: 8)
                    Text(service.status.running ? "Running" : "Stopped")
                    Spacer()
                }
                HStack(spacing: 5) {
                    Button("Start") { Task { await service.start() } }
                    Button("Stop") { Task { await service.stop() } }
                    Button("Restart") { Task { await service.restart() } }
                }
                .controlSize(.mini)
            }
            Section("Machines") {
                if machines.isEmpty { Text("Single-machine mode").foregroundStyle(.secondary) }
                ForEach(machines) { machine in
                    Label(machine.name, systemImage: machine.local ? "desktopcomputer" : "network")
                        .foregroundStyle(machine.reachable ? FortPalette.body : FortPalette.faint)
                }
            }
        }
        .listStyle(.sidebar)
    }

    private var topBar: some View {
        HStack(spacing: 14) {
            Text("FORT").font(.system(.callout, design: .monospaced).weight(.bold)).tracking(3.5).foregroundStyle(FortPalette.brassBright)
            if attentionCount > 0 {
                Text("\(attentionCount) need you").font(.caption.weight(.semibold)).foregroundStyle(FortPalette.needsYou)
                    .padding(.horizontal, 10).padding(.vertical, 5).background(FortPalette.needsYou.opacity(0.14), in: Capsule())
            }
            Spacer()
            if let local = machines.first(where: { $0.local }) {
                Circle().fill(local.reachable ? FortPalette.accepted : FortPalette.failed).frame(width: 7, height: 7)
                Text(local.name).font(.caption.monospaced()).foregroundStyle(FortPalette.muted)
            }
            Button("Give direction") { route = .assign }
                .buttonStyle(.borderedProminent).tint(FortPalette.brass).foregroundStyle(FortPalette.page)
        }
        .padding(.horizontal, 22).padding(.vertical, 12)
        .background(FortPalette.canvas)
        .overlay(alignment: .bottom) { Rectangle().fill(FortPalette.line).frame(height: 1) }
    }

    @ViewBuilder private var detail: some View {
        switch route ?? .deck {
        case .deck: deckView
        case .projects: projectsView
        case .assign: assignView
        case .crew: performanceView
        case .playbooks: playbooksView
        case .week: weekView
        case .today: todayView
        }
    }

    private var deckView: some View {
        ScrollView {
            ViewThatFits(in: .horizontal) {
                HStack(alignment: .top, spacing: 0) {
                    needsColumn.padding(22).frame(maxWidth: .infinity, alignment: .topLeading)
                    Rectangle().fill(FortPalette.line).frame(width: 1)
                    VStack(alignment: .leading, spacing: 24) { projectsColumn; crewColumn }
                        .padding(22).frame(maxWidth: 440, alignment: .topLeading)
                }
                VStack(alignment: .leading, spacing: 24) { needsColumn; projectsColumn; crewColumn }.padding(22)
            }
        }
    }

    private var needsColumn: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text("NEEDS YOU").deckSection(color: FortPalette.needsYou)
            if board.gates.isEmpty && failedRuns.isEmpty {
                FortDeckCard {
                    Text(attentionEmptyMessage)
                        .font(.callout).foregroundStyle(FortPalette.faint)
                }
            } else {
                ForEach(board.gates) { gateCard($0) }
                ForEach(failedRuns.prefix(3)) { failedRunCard($0) }
            }
            if let lastError {
                FortDeckCard(accent: FortPalette.failed) { Label(lastError, systemImage: "exclamationmark.triangle").foregroundStyle(FortPalette.failed) }
            }
        }
    }

    private var projectsColumn: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text("PROJECTS").deckSection()
            ForEach(projectRuns.prefix(5)) { projectRow($0) }
            if !backlog.isEmpty {
                Text("UP NEXT").deckSection().padding(.top, 8)
                ForEach(backlog.prefix(4)) { backlogRow($0) }
            }
        }
    }

    private var crewColumn: some View {
        VStack(alignment: .leading, spacing: 10) {
            Text("CREW").deckSection()
            ForEach(crewNames, id: \.self) { crewRow($0) }
        }
    }

    private var projectsView: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 16) {
                pageHeader("Project rooms", subtitle: "Accepted checkpoints are the measure of progress.")
                LazyVGrid(columns: [GridItem(.adaptive(minimum: 310), spacing: 16)], spacing: 16) {
                    ForEach(projectRuns) { projectCard($0) }
                }
            }.padding(22)
        }
    }

    private var assignView: some View {
        ScrollView {
            ViewThatFits(in: .horizontal) {
                HStack(alignment: .top, spacing: 24) {
                    composer.frame(minWidth: 360, maxWidth: .infinity)
                    roster.frame(minWidth: 300, maxWidth: 420)
                }
                VStack(alignment: .leading, spacing: 24) { composer; roster }
            }
            .padding(22)
        }
    }

    private var composer: some View {
        VStack(alignment: .leading, spacing: 14) {
            pageHeader("Give direction", subtitle: "Name the outcome; Fort handles the machinery.")
            TextEditor(text: $composeText).font(.body).scrollContentBackground(.hidden)
                .frame(minHeight: 150).padding(10).background(FortPalette.panel, in: RoundedRectangle(cornerRadius: 10))
                .overlay(RoundedRectangle(cornerRadius: 10).stroke(FortPalette.raised))
            if !isAnswerHandoff {
                Toggle("Propose a plan first — I'll sign off before work starts", isOn: $proposePlan).tint(FortPalette.brass)
            }
            if let routePreview {
                FortRoutePreviewCard(routePreview) { showHandoffPlaybookPicker = true }
            } else if !composeText.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
                FortDeckCard {
                    HStack(spacing: 9) {
                        ProgressView().controlSize(.small).tint(FortPalette.working)
                        Text("Resolving the route…").foregroundStyle(FortPalette.muted)
                    }
                }
            }
            if let inlineAnswer {
                FortDeckCard {
                    VStack(alignment: .leading, spacing: 7) {
                        Text("QUICK ANSWER").deckSection(color: FortPalette.working)
                        Text(inlineAnswer).textSelection(.enabled)
                    }
                }
            }
            if let lastError {
                FortDeckCard(accent: FortPalette.failed) {
                    Label(lastError, systemImage: "exclamationmark.triangle")
                        .foregroundStyle(FortPalette.failed)
                }
            }
            HStack {
                Button("Add to Up next") { Task { await addToReady() } }.buttonStyle(.bordered)
                Spacer()
                Button("Hand it off") { Task { await handOff() } }
                    .buttonStyle(.borderedProminent).tint(FortPalette.brass).foregroundStyle(FortPalette.page)
            }
            .disabled(busy || composeText.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)
        }
    }

    private var roster: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text("THE ROSTER").deckSection()
            ForEach(crewNames, id: \.self) { agent in
                FortDeckCard(accent: agentState(agent).color) {
                    HStack { Text(agent).font(.headline); Spacer(); FortStatusPill(agentState(agent)) }
                    Text(board.runs.first { $0.agent == agent && $0.status == "running" }.map(activityLine) ?? "Open capacity — ready for an assignment.")
                        .font(.callout).foregroundStyle(FortPalette.body).padding(.top, 5)
                }
            }
        }
    }

    private var performanceView: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 16) {
                pageHeader("Crew performance", subtitle: "last \(metrics?.windowDays ?? 30) days · \(metrics?.assignments ?? 0) assignments")
                LazyVGrid(columns: [GridItem(.adaptive(minimum: 330), spacing: 16)], spacing: 16) {
                    ForEach(metrics?.agents ?? []) { metricCard($0) }
                }
                if metrics?.agents.isEmpty != false { FortDeckCard { Text("Scorecards appear after human sign-offs create a useful sample.").foregroundStyle(FortPalette.muted) } }
            }.padding(22)
        }
    }

    private var weekView: some View {
        ScrollView([.horizontal, .vertical]) {
            VStack(alignment: .leading, spacing: 16) {
                pageHeader("The week", subtitle: "Upcoming work and open capacity by crew member")
                scheduleLegend
                weekHeader
                ForEach(crewNames, id: \.self) { weekRow($0) }
            }.padding(22).frame(minWidth: 900, alignment: .leading)
        }
    }

    private var todayView: some View {
        ScrollView([.horizontal, .vertical]) {
            VStack(alignment: .leading, spacing: 16) {
                HStack(alignment: .firstTextBaseline) {
                    pageHeader("Today", subtitle: board.gates.isEmpty ? "Your day is clear." : "\(board.gates.count) sign-offs need you now.")
                    Spacer()
                    Button("Open week") { route = .week }.buttonStyle(.bordered)
                }
                scheduleLegend
                todayHeader
                humanTodayRow
                ForEach(crewNames, id: \.self) { todayRow($0) }
            }.padding(22).frame(minWidth: 1050, alignment: .leading)
        }
    }

    private var playbooksView: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 14) {
                HStack(alignment: .firstTextBaseline, spacing: 12) {
                    pageHeader("Playbooks", subtitle: "who does what, with which model")
                    Spacer()
                    if let starter = playbooks.first(where: { $0.isDefault == true }) ?? playbooks.first {
                        Button { Task { await duplicate(starter) } } label: {
                            Label("New playbook", systemImage: "plus")
                        }
                        .buttonStyle(.bordered).tint(FortPalette.brass)
                        .help("Create a new playbook by duplicating the default route")
                    }
                }

                if playbooks.isEmpty {
                    FortDeckCard { Text("No playbooks are available yet.").foregroundStyle(FortPalette.muted) }
                } else {
                    ForEach(playbooks) { playbook in
                        FortPlaybookCard(
                            playbook,
                            onDuplicate: { Task { await duplicate(playbook) } },
                            onPlanGateChange: { enabled in setPlanGate(playbook, enabled: enabled) }
                        )
                        .opacity(savingPlaybooks.contains(playbook.id) ? 0.65 : 1)
                        .disabled(savingPlaybooks.contains(playbook.id))
                    }
                }

                if !shortcutPlaybooks.isEmpty {
                    Text("SHORTCUTS").deckSection().padding(.top, 2)
                    ForEach(shortcutPlaybooks) { shortcutRow($0) }
                }
            }
            .padding(22)
            .frame(maxWidth: 1050, alignment: .leading)
        }
    }

    private func shortcutRow(_ playbook: Playbook) -> some View {
        HStack(spacing: 11) {
            Image(systemName: triggerIcon(playbook.trigger.kind))
                .foregroundStyle(playbook.delivery == "answer" ? FortPalette.brassBright : FortPalette.working)
                .frame(width: 20)
            Text(triggerSentence(playbook.trigger.kind))
                .font(.callout).foregroundStyle(FortPalette.body)
            Text(playbook.name).font(.callout.weight(.semibold))
            Spacer()
            Toggle("", isOn: Binding(
                get: { playbook.trigger.enabled },
                set: { setTrigger(playbook, enabled: $0) }
            ))
            .labelsHidden().toggleStyle(.switch).tint(FortPalette.brass)
        }
        .padding(.horizontal, 14).padding(.vertical, 11)
        .background(FortPalette.panel, in: RoundedRectangle(cornerRadius: 10))
        .overlay(RoundedRectangle(cornerRadius: 10).stroke(FortPalette.line))
        .disabled(savingPlaybooks.contains(playbook.id))
    }

    private var handoffPlaybookPicker: some View {
        VStack(alignment: .leading, spacing: 14) {
            HStack {
                Text("Choose a playbook").font(.title2.weight(.semibold))
                Spacer()
                Button("Done") { showHandoffPlaybookPicker = false }
            }
            ScrollView {
                VStack(spacing: 8) {
                    Button {
                        handoffPlaybookID = nil
                        showHandoffPlaybookPicker = false
                    } label: {
                        pickerRow("Fort decides", detail: "Use deterministic triggers", selected: handoffPlaybookID == nil)
                    }
                    .buttonStyle(.plain)
                    ForEach(playbooks) { playbook in
                        Button {
                            handoffPlaybookID = playbook.id
                            showHandoffPlaybookPicker = false
                        } label: {
                            pickerRow(playbook.name, detail: "\(playbook.stages.count) stages · revision \(playbook.revision)", selected: handoffPlaybookID == playbook.id)
                        }
                        .buttonStyle(.plain)
                    }
                }
            }
        }
        .padding(22).frame(width: 430, height: 430)
        .background(FortPalette.page)
    }

    private func pickerRow(_ title: String, detail: String, selected: Bool) -> some View {
        HStack(spacing: 11) {
            Image(systemName: "link").foregroundStyle(FortPalette.brassBright)
            VStack(alignment: .leading, spacing: 2) {
                Text(title).font(.callout.weight(.semibold))
                Text(detail).font(.caption).foregroundStyle(FortPalette.muted)
            }
            Spacer()
            if selected { Image(systemName: "checkmark").foregroundStyle(FortPalette.brass) }
        }
        .padding(12).background(FortPalette.panel, in: RoundedRectangle(cornerRadius: 9))
        .overlay(RoundedRectangle(cornerRadius: 9).stroke(selected ? FortPalette.brass : FortPalette.line))
    }

    private func gateCard(_ gate: GateItem) -> some View {
        let run = board.runs.first { $0.id == gate.runID }
        return FortDeckCard(accent: FortPalette.needsYou) {
            VStack(alignment: .leading, spacing: 11) {
                HStack { Text(gateTitle(gate)).font(.headline); Spacer(); Text(FortTime.relative(gate.since)).font(.caption.monospaced()).foregroundStyle(FortPalette.faint) }
                Text(gate.input?.isEmpty == false ? gate.input! : "\(run.map(title) ?? gate.runID) reached a checkpoint and needs your sign-off.")
                    .font(.callout).foregroundStyle(FortPalette.body).lineLimit(5)
                HStack {
                    Button("Accept") { Task { await decide(gate, "approve") } }.buttonStyle(.borderedProminent).tint(FortPalette.accepted)
                    Button("Request changes…") { redirectGate = gate }.buttonStyle(.bordered)
                    if decidingGates.contains(gate.id) { ProgressView().controlSize(.small) }
                }.disabled(decidingGates.contains(gate.id))
            }
        }
    }

    private func failedRunCard(_ run: RunSummary) -> some View {
        FortDeckCard(accent: FortPalette.failed) {
            VStack(alignment: .leading, spacing: 11) {
                HStack {
                    Text("\(title(run)) hit a wall").font(.headline)
                    Spacer()
                    Text(FortTime.relative(run.updatedAt ?? run.createdAt))
                        .font(.caption.monospaced()).foregroundStyle(FortPalette.faint)
                }
                Text("\(run.agent) stopped. Open the assignment to see what happened and give direction.")
                    .font(.callout).foregroundStyle(FortPalette.body).lineLimit(3)
                Button("View what happened") { selectedRun = run }
                    .buttonStyle(.bordered).tint(FortPalette.failed)
            }
        }
    }

    private func projectRow(_ run: RunSummary) -> some View {
        let state = FortProjectState.resolve(run: run, gates: board.gates)
        return Button { selectedRun = run } label: {
            HStack(spacing: 11) {
                FortSigilView(name: title(run), state: state, size: 34)
                VStack(alignment: .leading, spacing: 3) {
                    Text(title(run)).font(.callout.weight(.semibold)).foregroundStyle(FortPalette.primary).lineLimit(1)
                    Text(run.checkpoints?.deckCaption ?? activityLine(run)).font(.caption).foregroundStyle(FortPalette.muted).lineLimit(1)
                }
                Spacer()
            }.contentShape(Rectangle())
        }.buttonStyle(.plain)
    }

    private func projectCard(_ run: RunSummary) -> some View {
        let state = FortProjectState.resolve(run: run, gates: board.gates)
        return Button { selectedRun = run } label: {
            FortDeckCard(accent: state == .needsYou ? state.color : nil) {
                VStack(alignment: .leading, spacing: 14) {
                    HStack {
                        FortSigilView(name: title(run), state: state, size: 48)
                        VStack(alignment: .leading) {
                            Text(title(run)).font(.title3.weight(.semibold)).foregroundStyle(FortPalette.primary)
                            Text([run.agent, run.machine].compactMap { $0 }.filter { !$0.isEmpty }.joined(separator: " · ")).font(.caption).foregroundStyle(FortPalette.muted)
                        }
                        Spacer(); FortStatusPill(state)
                    }
                    FortCheckpointBar(run.checkpoints)
                    Text(run.checkpoints?.deckCaption ?? activityLine(run)).font(.caption).foregroundStyle(FortPalette.muted)
                    if let body = run.body, !body.isEmpty { Text(body).font(.callout).foregroundStyle(FortPalette.body).lineLimit(3) }
                }
            }
        }.buttonStyle(.plain)
    }

    private func crewRow(_ agent: String) -> some View {
        HStack(spacing: 9) {
            Circle().fill(agentState(agent).color).frame(width: 8, height: 8)
            Text(agent).font(.callout.weight(.semibold))
            Spacer()
            Text(board.runs.first { $0.agent == agent && ["running", "blocked"].contains($0.status) }.map(activityLine) ?? "available")
                .font(.caption).foregroundStyle(FortPalette.muted).lineLimit(1)
        }
    }

    private func backlogRow(_ item: BacklogItem) -> some View {
        HStack(spacing: 9) {
            RoundedRectangle(cornerRadius: 3).fill(FortPalette.queued).frame(width: 8, height: 30)
            VStack(alignment: .leading, spacing: 2) {
                Text(item.title).font(.callout.weight(.semibold)).lineLimit(1)
                Text(item.agent?.isEmpty == false ? item.agent! : "Fort decides").font(.caption).foregroundStyle(FortPalette.muted)
            }
            Spacer()
            Button("Start") { Task { await dispatch(item) } }.buttonStyle(.bordered).controlSize(.small)
        }
    }

    private func metricCard(_ metric: AgentMetrics) -> some View {
        FortDeckCard(accent: trendColor(metric.trend)) {
            VStack(alignment: .leading, spacing: 13) {
                HStack { Text(metric.agent).font(.headline); Spacer(); Text(trendLabel(metric.trend)).font(.caption.weight(.semibold)).foregroundStyle(trendColor(metric.trend)) }
                HStack(spacing: 24) {
                    metricValue("\(Int(metric.firstPassPct.rounded()))%", "first-pass accepted")
                    metricValue(String(format: "%.2f", metric.redirectsPerAssignment), "redirects / assignment")
                    metricValue(metric.costKnown ? String(format: "$%.2f", metric.costPerAccepted) : "—", "per accepted checkpoint")
                }
                Text("\(metric.firstPass) of \(metric.decided) sign-offs accepted first pass").font(.caption).foregroundStyle(FortPalette.muted)
                HStack {
                    if let best = metric.best.first { Text("best at: \(best)").chip(FortPalette.accepted) }
                    if let weak = metric.weak.first { Text("weak: \(weak)").chip(FortPalette.faint) }
                }
            }
        }
    }

    private var scheduleLegend: some View {
        HStack(spacing: 16) {
            legend("active now", FortPalette.working); legend("up next", FortPalette.queued)
            legend("waiting on you", FortPalette.needsYou); legend("open capacity", FortPalette.outline)
        }.font(.caption).foregroundStyle(FortPalette.muted)
    }

    private var weekHeader: some View {
        HStack(spacing: 6) {
            Text("CREW").frame(width: 120, alignment: .leading)
            ForEach(weekDays, id: \.self) { Text($0).font(.caption.monospaced()).foregroundStyle($0 == currentWeekday ? FortPalette.brass : FortPalette.muted).frame(width: 96) }
        }
    }

    private func weekRow(_ agent: String) -> some View {
        let agentRuns = board.runs.filter { $0.agent == agent && isInDisplayedWeek($0.createdAt) }
        let queued = backlog.filter { ($0.agent ?? "") == agent }
        return HStack(spacing: 6) {
            Text(agent).font(.callout.weight(.semibold)).frame(width: 120, alignment: .leading)
            ForEach(Array(weekDays.enumerated()), id: \.offset) { index, _ in
                let run = agentRuns.first { dayIndex($0.createdAt) == index }
                if let run { scheduleCell(title(run), FortProjectState.resolve(run: run, gates: board.gates)) }
                else if index == 1, let item = queued.first { backlogScheduleCell(item) }
                else if index == 0 && agentRuns.isEmpty { scheduleCell("open capacity", .idle) }
                else { Color.clear.frame(width: 96, height: 38) }
            }
        }
    }

    private var todayHeader: some View {
        HStack(spacing: 4) {
            Text("CREW").frame(width: 120, alignment: .leading)
            ForEach(8...19, id: \.self) { hour in Text(hour > 12 ? "\(hour - 12)p" : "\(hour)a").font(.caption2.monospaced()).foregroundStyle(FortPalette.muted).frame(width: 68) }
        }
    }

    private var humanTodayRow: some View {
        HStack(spacing: 4) {
            Text("You").font(.callout.weight(.semibold)).foregroundStyle(FortPalette.brass).frame(width: 120, alignment: .leading)
            if board.gates.isEmpty { scheduleWide("evening is clear", color: FortPalette.outline, width: 212) }
            else { scheduleWide("\(board.gates.count) sign-off\(board.gates.count == 1 ? "" : "s") waiting", color: FortPalette.needsYou, width: 212) }
            Spacer()
        }
    }

    private func todayRow(_ agent: String) -> some View {
        let run = board.runs.first { $0.agent == agent && ["running", "blocked"].contains($0.status) }
        return HStack(spacing: 4) {
            Text(agent).font(.callout.weight(.semibold)).frame(width: 120, alignment: .leading)
            if let run { scheduleWide("\(title(run)) → next checkpoint", color: FortProjectState.resolve(run: run, gates: board.gates).color, width: 280) }
            else { scheduleWide("open capacity — assign work", color: FortPalette.outline, width: 212) }
            Spacer()
        }
    }

    private func scheduleCell(_ text: String, _ state: FortProjectState) -> some View {
        Text(text).font(.caption.weight(.semibold)).lineLimit(1).foregroundStyle(state == .idle ? FortPalette.muted : FortPalette.page)
            .padding(.horizontal, 8).frame(width: 96, height: 38, alignment: .leading)
            .background(state == .idle ? FortPalette.outline.opacity(0.35) : state.color, in: RoundedRectangle(cornerRadius: 7))
    }

    private func backlogScheduleCell(_ item: BacklogItem) -> some View {
        Menu {
            Button("Start") { Task { await dispatch(item) } }
            Divider()
            ForEach(crewNames, id: \.self) { agent in
                Button("Assign to \(agent)") { Task { await reassign(item, to: agent) } }
            }
        } label: {
            Text(item.title).font(.caption.weight(.semibold)).lineLimit(1).foregroundStyle(FortPalette.body)
                .padding(.horizontal, 8).frame(width: 96, height: 38, alignment: .leading)
                .background(FortPalette.queued, in: RoundedRectangle(cornerRadius: 7))
        }.menuStyle(.borderlessButton)
    }

    private func scheduleWide(_ text: String, color: Color, width: CGFloat) -> some View {
        Text(text).font(.caption.weight(.semibold)).foregroundStyle(FortPalette.primary)
            .padding(.horizontal, 10).frame(width: width, height: 38, alignment: .leading).background(color, in: RoundedRectangle(cornerRadius: 7))
    }

    private func legend(_ text: String, _ color: Color) -> some View { HStack(spacing: 5) { RoundedRectangle(cornerRadius: 2).fill(color).frame(width: 12, height: 8); Text(text) } }
    private func pageHeader(_ title: String, subtitle: String) -> some View { VStack(alignment: .leading, spacing: 4) { Text(title).font(.title2.weight(.semibold)); Text(subtitle).font(.callout).foregroundStyle(FortPalette.muted) } }
    private func metricValue(_ value: String, _ label: String) -> some View { VStack(alignment: .leading, spacing: 3) { Text(value).font(.title2.monospacedDigit().weight(.semibold)); Text(label).font(.caption2).foregroundStyle(FortPalette.faint) } }
    private func agentChip(_ agent: String, label: String) -> some View {
        Button(label) { selectedAgent = agent }.buttonStyle(.plain).font(.caption.weight(.semibold))
            .foregroundStyle(selectedAgent == agent ? FortPalette.brassBright : FortPalette.muted)
            .padding(.horizontal, 10).padding(.vertical, 6).background(selectedAgent == agent ? FortPalette.brass.opacity(0.12) : FortPalette.panel, in: Capsule())
            .overlay(Capsule().stroke(selectedAgent == agent ? FortPalette.brass : FortPalette.outline))
    }

    private var projectRuns: [RunSummary] { board.runs.sorted { stateRank($0) < stateRank($1) } }
    private var workingRuns: [RunSummary] { board.runs.filter { $0.status == "running" } }
    private var failedRuns: [RunSummary] {
        board.runs
            .filter { ["failed", "error"].contains($0.status.lowercased()) }
            .sorted { ($0.updatedAt ?? $0.createdAt ?? "") > ($1.updatedAt ?? $1.createdAt ?? "") }
    }
    private var attentionCount: Int { board.gates.count + failedRuns.count }
    private var attentionEmptyMessage: String {
        guard !workingRuns.isEmpty else { return "All quiet — nothing needs you." }
        return "That's everything — \(workingRuns.count) crew member\(workingRuns.count == 1 ? " is" : "s are") working and \(workingRuns.count == 1 ? "doesn't" : "don't") need you."
    }
    private var crewNames: [String] {
        Array(Set(board.runs.map(\.agent).filter { !$0.isEmpty } + machines.flatMap { $0.agents ?? [] })).sorted()
    }
    private var weekDays: [String] { ["MON", "TUE", "WED", "THU", "FRI", "SAT", "SUN"] }
    private var currentWeekday: String {
        weekDays[FortSchedule.weekdayIndex(for: Date())]
    }
    private func dayIndex(_ iso: String?) -> Int {
        guard let iso, let date = ISO8601DateFormatter().date(from: iso) else { return 0 }
        return FortSchedule.weekdayIndex(for: date)
    }
    private func isInDisplayedWeek(_ iso: String?) -> Bool {
        guard let iso, let date = ISO8601DateFormatter().date(from: iso) else { return false }
        return FortSchedule.isInDisplayedWeek(date)
    }
    private func stateRank(_ run: RunSummary) -> Int {
        switch FortProjectState.resolve(run: run, gates: board.gates) { case .needsYou: return 0; case .failed: return 1; case .working: return 2; case .idle: return 3; case .delivered: return 4 }
    }
    private func agentState(_ agent: String) -> FortProjectState {
        let runs = board.runs.filter { $0.agent == agent }
        if runs.contains(where: { run in board.gates.contains { $0.runID == run.id } }) { return .needsYou }
        if runs.contains(where: { $0.status == "running" }) { return .working }
        return .idle
    }
    private func title(_ run: RunSummary) -> String { run.title.isEmpty ? run.id : run.title }
    private func gateTitle(_ gate: GateItem) -> String { gate.nodeID.replacingOccurrences(of: "_", with: " ").replacingOccurrences(of: "-", with: " ").capitalized }
    private func activityLine(_ run: RunSummary) -> String {
        switch FortProjectState.resolve(run: run, gates: board.gates) {
        case .needsYou: return "awaiting your sign-off"
        case .working: return "\(run.agent) working · \(FortTime.elapsed(run.createdAt))"
        case .delivered: return "all accepted"
        case .failed: return "needs attention"
        case .idle: return run.status == "queued" ? "up next" : "idle"
        }
    }
    private func trendColor(_ trend: String) -> Color { trend == "improving" ? FortPalette.accepted : (trend == "slipping" ? FortPalette.failed : FortPalette.muted) }
    private func trendLabel(_ trend: String) -> String { trend == "improving" ? "▲ improving" : (trend == "slipping" ? "▼ slipping" : "→ steady") }

    private var shortcutPlaybooks: [Playbook] {
        playbooks.filter { !$0.trigger.kind.isEmpty && $0.trigger.kind != "manual" }
    }

    private var selectedHandoffPlaybook: Playbook? {
        guard let handoffPlaybookID else { return nil }
        return playbooks.first { $0.id == handoffPlaybookID }
    }

    private var isAnswerHandoff: Bool {
        selectedHandoffPlaybook?.delivery == "answer" || routePreview?.delivery == "answer"
    }

    private var routePreviewKey: String {
        [client.baseURL.absoluteString, route?.rawValue ?? "deck", handoffPlaybookID ?? "automatic", selectedHandoffPlaybook?.revision.description ?? "latest", proposePlan.description, composeText]
            .joined(separator: "|")
    }

    private func routeRequest(for text: String) -> RouteRequest {
        RouteRequest(
            text: text,
            playbookID: selectedHandoffPlaybook?.id,
            playbookRevision: selectedHandoffPlaybook?.revision,
            planGate: selectedHandoffPlaybook?.delivery == "answer" ? false : proposePlan
        )
    }

    private func refreshRoutePreview() async {
        guard route == .assign else { return }
        let text = composeText.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !text.isEmpty else { routePreview = nil; return }
        do { try await Task.sleep(nanoseconds: 350_000_000) } catch { return }
        guard !Task.isCancelled else { return }
        do { routePreview = try await client.route(routeRequest(for: text)) }
        catch { routePreview = nil }
    }

    private func setPlanGate(_ playbook: Playbook, enabled: Bool) {
        let next = Playbook(
            id: playbook.id, name: playbook.name, revision: playbook.revision,
            isDefault: playbook.isDefault, planGate: enabled, delivery: playbook.delivery,
            trigger: playbook.trigger, stages: playbook.stages
        )
        replacePlaybook(next)
        Task { await save(next) }
    }

    private func setTrigger(_ playbook: Playbook, enabled: Bool) {
        let next = Playbook(
            id: playbook.id, name: playbook.name, revision: playbook.revision,
            isDefault: playbook.isDefault, planGate: playbook.planGate, delivery: playbook.delivery,
            trigger: PlaybookTrigger(kind: playbook.trigger.kind, enabled: enabled), stages: playbook.stages
        )
        replacePlaybook(next)
        Task { await save(next) }
    }

    private func save(_ playbook: Playbook) async {
        savingPlaybooks.insert(playbook.id); defer { savingPlaybooks.remove(playbook.id) }
        do { replacePlaybook(try await client.savePlaybook(playbook)); lastError = nil }
        catch {
            lastError = friendly(error)
            if let latest = try? await client.playbooks() { playbooks = latest }
        }
    }

    private func duplicate(_ playbook: Playbook) async {
        savingPlaybooks.insert(playbook.id); defer { savingPlaybooks.remove(playbook.id) }
        do {
            let copy = try await client.duplicatePlaybook(playbook.id)
            replacePlaybook(copy)
            lastError = nil
        } catch { lastError = friendly(error) }
    }

    private func replacePlaybook(_ playbook: Playbook) {
        if let index = playbooks.firstIndex(where: { $0.id == playbook.id }) { playbooks[index] = playbook }
        else { playbooks.append(playbook); playbooks.sort { $0.id < $1.id } }
    }

    private func triggerSentence(_ kind: String) -> String {
        switch kind {
        case "question": return "When I ask a question →"
        case "bug": return "When it's a bug report →"
        case "research": return "When I ask for research →"
        case "feature": return "When I describe feature work →"
        default: return "When the \(kind.replacingOccurrences(of: "_", with: " ")) trigger matches →"
        }
    }

    private func triggerIcon(_ kind: String) -> String {
        switch kind {
        case "question": return "bolt.fill"
        case "bug": return "ladybug.fill"
        case "research": return "magnifyingglass"
        case "feature": return "hammer.fill"
        default: return "arrow.triangle.branch"
        }
    }

    private func reload() async {
        do {
            async let nextSummary = client.summary(); async let nextBoard = client.board()
            async let nextBacklog = client.backlog(); async let nextMachines = client.machines(); async let nextMetrics = client.metrics(); async let nextPlaybooks = client.playbooks()
            summary = try await nextSummary; board = try await nextBoard; backlog = try await nextBacklog
            machines = try await nextMachines; metrics = try await nextMetrics; playbooks = try await nextPlaybooks; lastError = nil
        } catch { lastError = friendly(error) }
    }

    private func handOff() async {
        let text = composeText.trimmingCharacters(in: .whitespacesAndNewlines); guard !text.isEmpty else { return }
        busy = true; defer { busy = false }
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
                lastError = nil
            case .failure(let message):
                inlineAnswer = nil
                lastError = message
            case .assignment where resolved.delivery == "answer":
                inlineAnswer = nil
                lastError = "Quick answer did not return an inline answer. Check run \(result.runID) for the failure."
            case .assignment:
                composeText = ""; routePreview = nil; inlineAnswer = nil; route = .deck
                await reload()
            }
        } catch { lastError = friendly(error) }
    }

    private func addToReady() async {
        let parts = split(composeText); guard !parts.title.isEmpty else { return }
        busy = true; defer { busy = false }
        do { _ = try await client.addBacklog(title: parts.title, body: parts.body.isEmpty ? nil : parts.body, agent: selectedAgent.isEmpty ? nil : selectedAgent); composeText = ""; await reload() }
        catch { lastError = friendly(error) }
    }

    private func dispatch(_ item: BacklogItem) async {
        do { _ = try await client.dispatchBacklog(item.id); await reload() }
        catch { lastError = friendly(error) }
    }

    private func reassign(_ item: BacklogItem, to agent: String) async {
        do { _ = try await client.reassignBacklog(item.id, agent: agent); await reload() }
        catch { lastError = friendly(error) }
    }

    private func split(_ text: String) -> (title: String, body: String) {
        let lines = text.split(separator: "\n", maxSplits: 1, omittingEmptySubsequences: false)
        return (String(lines.first ?? "").trimmingCharacters(in: .whitespacesAndNewlines), lines.count > 1 ? String(lines[1]).trimmingCharacters(in: .whitespacesAndNewlines) : "")
    }

    private func decide(_ gate: GateItem, _ decision: String, note: String? = nil) async {
        decidingGates.insert(gate.id); defer { decidingGates.remove(gate.id) }
        do { let applied = try await client.decideGate(run: gate.runID, node: gate.nodeID, decision: decision, note: note); if !applied { lastError = "No execution plane — sign-off unavailable." }; await reload() }
        catch { lastError = friendly(error) }
    }

    private func friendly(_ error: Error) -> String {
        switch error {
        case FortClientError.httpStatus(let status, _): return "Server error (\(status))."
        case FortClientError.nonHTTPResponse: return "Unexpected response."
        case let url as URLError where [.cannotConnectToHost, .cannotFindHost, .networkConnectionLost].contains(url.code): return "Fort not reachable — is the service running?"
        default: return error.localizedDescription
        }
    }
}

private struct MacRedirectSheet: View {
    @Environment(\.dismiss) private var dismiss
    let gate: GateItem
    let onSubmit: (String) -> Void
    @State private var note = ""
    var body: some View {
        VStack(alignment: .leading, spacing: 14) {
            Text("Request changes").font(.title2.weight(.semibold))
            Text("Say what should change before the next sign-off.").foregroundStyle(.secondary)
            TextEditor(text: $note).frame(minHeight: 120).padding(8).overlay(RoundedRectangle(cornerRadius: 8).stroke(.quaternary))
            HStack { Spacer(); Button("Cancel") { dismiss() }; Button("Send") { onSubmit(note.trimmingCharacters(in: .whitespacesAndNewlines)); dismiss() }.buttonStyle(.borderedProminent).disabled(note.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty) }
        }.padding(22).frame(width: 460)
    }
}

private struct MacRunDetailSheet: View {
    @EnvironmentObject private var client: FortClient
    @Environment(\.dismiss) private var dismiss
    let run: RunSummary
    @State private var detail: RunDetail?
    var body: some View {
        VStack(alignment: .leading, spacing: 16) {
            HStack { Text(run.title.isEmpty ? run.id : run.title).font(.title2.weight(.semibold)); Spacer(); Button("Done") { dismiss() } }
            FortCheckpointBar(run.checkpoints)
            if let detail {
                List {
                    Section("Checkpoints") { ForEach(detail.nodes) { node in HStack { Text(node.nodeID); Spacer(); Text(node.status).foregroundStyle(.secondary) } } }
                    Section("Activity") { ForEach(detail.events) { event in Text(event.data ?? event.type).lineLimit(2) } }
                }
            } else { ProgressView() }
        }.padding(22).frame(minWidth: 560, minHeight: 460).task { detail = try? await client.runDetail(run.id) }
    }
}

private extension Text {
    func deckSection(color: Color = FortPalette.muted) -> some View { font(.caption.weight(.semibold)).tracking(1.3).foregroundStyle(color) }
    func chip(_ color: Color) -> some View { font(.caption2.weight(.semibold)).foregroundStyle(color).padding(.horizontal, 8).padding(.vertical, 4).background(color.opacity(0.12), in: Capsule()) }
}
