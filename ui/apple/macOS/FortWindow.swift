import SwiftUI
import Combine
import FortKit

private enum MacDeckRoute: String, CaseIterable, Identifiable {
    case deck, today, crew, playbooks, assign, week
    static let primary: [MacDeckRoute] = [.deck, .assign, .crew, .week, .today, .playbooks]
    var id: String { rawValue }
    var title: String {
        switch self {
        case .deck: return "Deck"
        case .crew: return "Performance"
        case .playbooks: return "Playbooks"
        case .assign: return "Assign"
        default: return rawValue.capitalized
        }
    }
    var icon: String {
        switch self {
        case .deck: return "rectangle.3.group"
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
    @State private var profiles: [ProfileOption] = []
    @State private var metrics: MetricsResponse?
    @State private var playbooks: [Playbook] = []
    @State private var composeText = ""
    @State private var selectedAgent = ""
    @State private var selectedProfileID = ""
    @State private var selectedMachine = ""
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
    @State private var selectedConversationID = ""
    @State private var conversationEvents: [Event] = []
    @State private var conversationEventCursor = 0

    private let newConversationID = "__new_conversation__"

    private let refresh = Timer.publish(every: 3, on: .main, in: .common).autoconnect()

    var body: some View {
        ZStack {
            FortPalette.page.ignoresSafeArea()
            VStack(spacing: 0) {
                topBar
                detail
            }
        }
        .frame(minWidth: 1080, minHeight: 680)
        .foregroundStyle(FortPalette.primary)
        .preferredColorScheme(.dark)
        .tint(FortPalette.brass)
        .task { await service.refresh(); await reload() }
        .task(id: client.baseURL) { await consumeConversationEvents() }
		.task(id: selectedConversation?.id) {
			if let run = selectedConversation { await loadConversationEvents(for: run) }
		}
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
                    Label(item.title, systemImage: item.icon).tag(item)
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
            Section("Execution mesh") {
                if machines.isEmpty {
                    Label("This Mac", systemImage: "desktopcomputer")
                        .foregroundStyle(FortPalette.body)
                }
                ForEach(machines) { machine in
                    Label(machine.name, systemImage: machine.local ? "desktopcomputer" : "network")
                        .foregroundStyle(machine.reachable ? FortPalette.body : FortPalette.faint)
                }
                Text("Status only — Fort places work automatically.")
                    .font(.caption2)
                    .foregroundStyle(FortPalette.faint)
            }
        }
        .listStyle(.sidebar)
    }

    private var topBar: some View {
        let mesh = FortMeshSummary.resolve(machines)
        return HStack(spacing: 13) {
            FortAgentOrbAvatar(name: "Fort", state: fortOrbState, size: 28)
            Text("FORT").font(.system(.callout, design: .monospaced).weight(.bold)).tracking(3.5).foregroundStyle(FortPalette.brassBright)
            ForEach(MacDeckRoute.primary) { item in
                Button(item.title) { route = item }
                    .buttonStyle(.plain)
                    .font(.caption.weight(route == item ? .semibold : .regular))
                    .foregroundStyle(route == item ? FortPalette.primary : FortPalette.muted)
            }
            if attentionCount > 0 {
                Button { focusFirstAttention() } label: {
                    Text("\(attentionCount) need you").font(.caption.weight(.semibold)).foregroundStyle(FortPalette.needsYou)
                        .padding(.horizontal, 10).padding(.vertical, 5).background(FortPalette.needsYou.opacity(0.14), in: Capsule())
                }
                .buttonStyle(.plain)
                .help("Open the first assignment waiting for your approval")
            }
            Spacer()
            Circle()
                .fill(mesh.reachable == mesh.total ? FortPalette.accepted : (mesh.reachable == 0 ? FortPalette.failed : FortPalette.needsYou))
                .frame(width: 7, height: 7)
            Text(mesh.title).font(.caption.weight(.semibold)).foregroundStyle(FortPalette.body)
            if let detail = mesh.detail {
                Text(detail).font(.caption.monospaced()).foregroundStyle(FortPalette.muted)
            }
        }
        .padding(.horizontal, 16).padding(.vertical, 9)
        .background(FortPalette.canvas)
        .overlay(alignment: .bottom) { Rectangle().fill(FortPalette.line).frame(height: 1) }
    }

    @ViewBuilder private var detail: some View {
        switch route ?? .deck {
        case .deck: deckView
        case .assign: assignView
        case .crew: performanceView
        case .playbooks: playbooksView
        case .week: weekView
        case .today: todayView
        }
    }

    private var deckView: some View {
        HStack(spacing: 0) {
            conversationSidebar
                .frame(width: 265)
                .background(FortPalette.canvas)
            Rectangle().fill(FortPalette.line).frame(width: 1)
            conversationCenter
                .frame(maxWidth: .infinity, maxHeight: .infinity)
            Rectangle().fill(FortPalette.line).frame(width: 1)
            commandRail
                .frame(width: 270)
                .background(FortPalette.canvas)
        }
    }

    private var conversationSidebar: some View {
        VStack(alignment: .leading, spacing: 0) {
            Button {
                beginNewConversation()
            } label: {
                Text("New conversation")
                    .font(.caption.weight(.semibold))
                    .frame(maxWidth: .infinity)
                    .padding(.vertical, 7)
            }
            .buttonStyle(.plain)
            .foregroundStyle(FortPalette.working)
            .background(FortPalette.working.opacity(0.12), in: RoundedRectangle(cornerRadius: 7))
            .overlay(RoundedRectangle(cornerRadius: 7).stroke(FortPalette.working.opacity(0.7)))
            .padding(12)

            ScrollView {
                VStack(alignment: .leading, spacing: 7) {
                    Text("INBOX").deckSection().padding(.horizontal, 5)
                    sidebarLine("Needs you", trailing: "\(attentionCount)", selected: attentionCount > 0) {
                        focusFirstAttention()
                    }
                    sidebarLine("Updates", trailing: "\(workingRuns.count)", selected: false) { }

                    Text("CONVERSATIONS").deckSection().padding(.horizontal, 5).padding(.top, 10)
                    ForEach(conversationRuns) { run in
                        let activity = conversationActivity(run)
                        sidebarLine(
                            title(run),
                            trailing: conversationTimestamp(run),
                            selected: selectedConversation?.id == run.id,
                            status: activity.label,
                            statusColor: activity.projectState.color
                        ) {
                            selectConversation(run)
                        }
                    }
                }
                .padding(.horizontal, 10).padding(.bottom, 14)
            }

            HStack(spacing: 7) {
                Circle().fill(FortPalette.accepted).frame(width: 7, height: 7)
                Text("Fort decides placement").font(.caption).foregroundStyle(FortPalette.muted)
            }
            .padding(13).frame(maxWidth: .infinity, alignment: .leading)
            .overlay(alignment: .top) { Rectangle().fill(FortPalette.line).frame(height: 1) }
        }
    }

    private var conversationCenter: some View {
        VStack(spacing: 0) {
            HStack(spacing: 8) {
                Text(selectedConversation.map(title) ?? "New conversation")
                    .font(.callout.weight(.semibold))
                    .lineLimit(1)
                if let run = selectedConversation {
                    let activity = conversationActivity(run)
                    Text(activity.label)
                        .font(.caption2.weight(.semibold))
                        .foregroundStyle(activity.projectState.color)
                        .padding(.horizontal, 7).padding(.vertical, 3)
                        .background(activity.projectState.color.opacity(0.12), in: Capsule())
                }
                Spacer()
                if let run = selectedConversation {
                    Button("Details") { selectedRun = run }.buttonStyle(.plain).font(.caption).foregroundStyle(FortPalette.muted)
                }
            }
            .padding(.horizontal, 20).frame(height: 49)
            .overlay(alignment: .bottom) { Rectangle().fill(FortPalette.line).frame(height: 1) }

            ScrollView {
                VStack(alignment: .leading, spacing: 15) {
                    if let run = selectedConversation {
                        conversationMessage("You", detail: FortTime.relative(run.createdAt), body: conversationPrompt(run), state: .idle, role: .human)
                        conversationMessage(
                            conversationAgent(run),
                            detail: FortTime.relative(run.updatedAt ?? run.createdAt),
                            body: conversationResponse(run),
                            model: run.model,
                            state: conversationActivity(run).projectState
                        )

                        ForEach(gatesForConversation(run)) { gate in
                            conversationGateCard(gate)
                        }

                        if FortConversationPromotion.isEligible(run, gates: board.gates) {
                            HStack(spacing: 10) {
                                FortAgentOrbAvatar(name: title(run), state: .idle, size: 28)
                                VStack(alignment: .leading, spacing: 2) {
                                    Text("Turn this into work").font(.caption.weight(.semibold)).foregroundStyle(FortPalette.working)
                                    Text("Create a routed assignment from this conversation.").font(.caption2).foregroundStyle(FortPalette.muted)
                                }
                                Spacer()
                                Button("Assign work") {
                                    composeText = "\(title(run))\n\(run.body ?? "")".trimmingCharacters(in: .whitespacesAndNewlines)
                                    handoffPlaybookID = FortPlaybookRouting.defaultAssignment(in: playbooks)?.id
                                    routePreview = nil
                                    route = .assign
                                }
                                .buttonStyle(.bordered).controlSize(.small).tint(FortPalette.working)
                            }
                            .padding(10)
                            .background(FortPalette.panel, in: RoundedRectangle(cornerRadius: 8))
                            .overlay(RoundedRectangle(cornerRadius: 8).stroke(FortPalette.working.opacity(0.7)))
                            .padding(.leading, 44)
                        }

                        conversationProgress(run)
                        conversationActivityTimeline(run)
                    } else {
                        conversationMessage("Fort", detail: "ready", body: "Choose an agent, model, and machine — or let Fort decide — then send your first message.", state: .idle)
                    }
                }
                .padding(20).frame(maxWidth: 780, alignment: .leading)
            }

            VStack(spacing: 8) {
                TextEditor(text: $composeText)
                    .font(.callout)
                    .scrollContentBackground(.hidden)
                    .frame(height: 45)
                HStack(spacing: 8) {
                    Picker("Agent", selection: $selectedAgent) {
                        Text("Fort decides").tag("")
                        ForEach(availableAgents, id: \.self) { agent in
                            Text(agent.capitalized).tag(agent)
                        }
                    }
                    .pickerStyle(.menu)
                    .frame(maxWidth: 130)
                    .onChange(of: selectedAgent) { _ in selectDefaultProfileForAgent() }

                    Picker("Model", selection: $selectedProfileID) {
                        Text("Automatic").tag("")
                        ForEach(profilesForSelectedAgent) { profile in
                            Text(modelOptionLabel(profile)).tag(profile.id)
                                .disabled(!profileIsReady(profile))
                        }
                    }
                    .pickerStyle(.menu)
                    .frame(maxWidth: 190)
                    .onChange(of: selectedProfileID) { _ in profileSelectionChanged() }

                    Picker("Machine", selection: $selectedMachine) {
                        Text("Fort places").tag("")
                        ForEach(availableMachineNames, id: \.self) { machine in
                            Text(machine).tag(machine)
                        }
                    }
                    .pickerStyle(.menu)
                    .frame(maxWidth: 160)
                    Spacer()
                    if busy {
                        Text("Submitting to Fort…")
                            .font(.caption2.weight(.medium))
                            .foregroundStyle(FortPalette.working)
                    }
                    Button("Assign") { route = .assign }.buttonStyle(.bordered).controlSize(.small).tint(FortPalette.working)
                    Button("Send") { Task { await sendConversation() } }
                        .buttonStyle(.borderedProminent).controlSize(.small).tint(FortPalette.working)
                        .disabled(composeText.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty || busy || composerSelectionIsInvalid)
                }
            }
            .padding(10)
            .background(FortPalette.panel, in: RoundedRectangle(cornerRadius: 8))
            .overlay(RoundedRectangle(cornerRadius: 8).stroke(FortPalette.raised))
            .padding(.horizontal, 15).padding(.bottom, 14)
        }
        .background(FortPalette.page)
    }

    private var commandRail: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 18) {
                Text("CURRENT AGENT").deckSection()
                if let run = selectedConversation {
                    commandAgentCard(conversationAgent(run), state: FortProjectState.resolve(run: run, gates: board.gates), detail: activityLine(run))
                } else {
                    Text("Fort will choose an agent.").font(.caption).foregroundStyle(FortPalette.faint)
                }

                Text("OTHER AGENTS").deckSection()
                ForEach(crewNames.filter { $0 != selectedConversation.map(conversationAgent) }, id: \.self) { agent in
                    commandAgentCard(agent, state: agentState(agent), detail: board.runs.first { $0.agent == agent && $0.status == "running" }.map(activityLine) ?? "Ready")
                }

                Text("MACHINES").deckSection()
                if machines.isEmpty {
                    commandMachineCard("This Mac", detail: "local control plane", ready: true)
                }
                ForEach(machines) { machine in
                    commandMachineCard(machine.name, detail: (machine.agents ?? []).joined(separator: ", "), ready: machine.reachable)
                }

                HStack(spacing: 6) {
                    Circle().fill(FortPalette.accepted).frame(width: 6, height: 6)
                    Text("All systems operational")
                }
                .font(.caption2).foregroundStyle(FortPalette.muted).padding(.top, 8)
            }
            .padding(16)
        }
    }

    private func sidebarLine(
        _ text: String,
        trailing: String,
        selected: Bool,
        status: String? = nil,
        statusColor: Color = FortPalette.muted,
        action: @escaping () -> Void
    ) -> some View {
        Button(action: action) {
            HStack(spacing: 8) {
                Text(text).lineLimit(1)
                Spacer()
                if let status {
                    Text(status)
                        .font(.caption2.weight(.semibold))
                        .foregroundStyle(statusColor)
                        .padding(.horizontal, 6).padding(.vertical, 2)
                        .background(statusColor.opacity(0.11), in: Capsule())
                }
                Text(trailing).font(.caption2.monospaced()).foregroundStyle(FortPalette.faint)
            }
            .font(.caption).foregroundStyle(selected ? FortPalette.primary : FortPalette.muted)
            .padding(.horizontal, 7).padding(.vertical, 6)
            .background(selected ? FortPalette.working.opacity(0.11) : Color.clear, in: RoundedRectangle(cornerRadius: 6))
        }
        .buttonStyle(.plain)
    }

    private func conversationMessage(
        _ name: String,
        detail: String,
        body: String,
        model: String? = nil,
        state: FortProjectState,
        role: ConversationMessageRole = .agent
    ) -> some View {
        HStack(alignment: .top, spacing: 10) {
            if role == .human {
                HumanConversationAvatar(size: 32)
            } else {
                FortAgentOrbAvatar(name: name, state: state, size: 32)
            }
            VStack(alignment: .leading, spacing: 3) {
                HStack(spacing: 8) {
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
                Text(body).font(.callout).foregroundStyle(FortPalette.body).fixedSize(horizontal: false, vertical: true)
            }
        }
    }

    private func conversationGateCard(_ gate: GateItem) -> some View {
        VStack(alignment: .leading, spacing: 10) {
            HStack(spacing: 7) {
                Image(systemName: "checkmark.seal.fill")
                Text("Work is paused until you approve")
                    .font(.callout.weight(.semibold))
                Spacer()
                Text("Needs approval")
                    .font(.caption2.weight(.semibold))
                    .foregroundStyle(FortPalette.needsYou)
            }
            .foregroundStyle(FortPalette.needsYou)

            Text(gate.input?.isEmpty == false ? gate.input! : "Review this checkpoint, then approve it or request a specific change.")
                .font(.caption)
                .foregroundStyle(FortPalette.body)
                .fixedSize(horizontal: false, vertical: true)

            HStack(spacing: 8) {
                Button("Approve & continue") { Task { await decide(gate, "approve") } }
                    .buttonStyle(.borderedProminent)
                    .tint(FortPalette.accepted)
                Button("Request changes") { redirectGate = gate }
                    .buttonStyle(.bordered)
                    .tint(FortPalette.needsYou)
                if decidingGates.contains(gate.id) { ProgressView().controlSize(.small) }
            }
            .disabled(decidingGates.contains(gate.id))
        }
        .padding(12)
        .background(FortPalette.needsYou.opacity(0.08), in: RoundedRectangle(cornerRadius: 8))
        .overlay(RoundedRectangle(cornerRadius: 8).stroke(FortPalette.needsYou.opacity(0.7)))
        .padding(.leading, 44)
    }

    private func conversationProgress(_ run: RunSummary) -> some View {
        let activity = conversationActivity(run)
        return VStack(alignment: .leading, spacing: 9) {
            HStack {
                Text(title(run)).font(.caption.weight(.semibold)).lineLimit(1)
                Spacer()
                Text(activity.label)
                    .font(.caption2.weight(.semibold)).foregroundStyle(activity.projectState.color)
            }
            FortCheckpointBar(run.checkpoints)
            HStack(spacing: 24) {
                progressDatum("Agent", conversationAgent(run))
                progressDatum("Model", run.model?.isEmpty == false ? run.model! : "Configured default")
                progressDatum("Machine", run.machine ?? "Fort placed")
                progressDatum("Elapsed", FortTime.elapsed(run.createdAt))
                if let checkpoints = run.checkpoints { progressDatum("Progress", "\(checkpoints.accepted) of \(checkpoints.total)") }
            }
        }
        .padding(12)
        .background(FortPalette.canvas, in: RoundedRectangle(cornerRadius: 8))
        .overlay(RoundedRectangle(cornerRadius: 8).stroke(FortPalette.line))
        .padding(.leading, 44)
    }

    private func conversationActivityTimeline(_ run: RunSummary) -> some View {
        let events = meaningfulConversationEvents(for: run)
        let activity = conversationActivity(run)
        return VStack(alignment: .leading, spacing: 8) {
            HStack {
                Text("ACTIVITY").deckSection(color: FortPalette.faint)
                Spacer()
                Text("durable event log")
                    .font(.caption2)
                    .foregroundStyle(FortPalette.faint)
            }

            if events.isEmpty {
                HStack(spacing: 8) {
                    Circle().stroke(activity.projectState.color, lineWidth: 1.3).frame(width: 8, height: 8)
                    Text(activity.label)
                        .font(.caption.weight(.semibold))
                        .foregroundStyle(activity.projectState.color)
                    Text("No execution evidence yet.")
                        .font(.caption)
                        .foregroundStyle(FortPalette.faint)
                }
            } else {
                ForEach(Array(events.suffix(8))) { event in
                    conversationTimelineRow(event)
                }
            }
        }
        .padding(12)
        .background(FortPalette.canvas, in: RoundedRectangle(cornerRadius: 8))
        .overlay(RoundedRectangle(cornerRadius: 8).stroke(FortPalette.line))
        .padding(.leading, 44)
    }

    private func conversationTimelineRow(_ event: Event) -> some View {
        let color = timelineColor(event)
        return HStack(alignment: .top, spacing: 8) {
            Image(systemName: timelineIcon(event))
                .font(.caption2.weight(.semibold))
                .foregroundStyle(color)
                .frame(width: 14, height: 16)
            Text(timelineCopy(event))
                .font(.caption)
                .foregroundStyle(event.type.lowercased() == "error" ? FortPalette.failed : FortPalette.body)
                .lineLimit(3)
                .textSelection(.enabled)
            Spacer(minLength: 8)
            Text(FortTime.relative(event.time))
                .font(.caption2.monospaced())
                .foregroundStyle(FortPalette.faint)
        }
    }

    private func progressDatum(_ label: String, _ value: String) -> some View {
        VStack(alignment: .leading, spacing: 2) {
            Text(label).font(.caption2).foregroundStyle(FortPalette.faint)
            Text(value).font(.caption2.weight(.medium)).foregroundStyle(FortPalette.body).lineLimit(1)
        }
    }

    private func commandAgentCard(_ agent: String, state: FortProjectState, detail: String) -> some View {
        HStack(spacing: 9) {
            FortAgentOrbAvatar(name: agent, state: state, size: 30)
            VStack(alignment: .leading, spacing: 2) {
                Text(agent).font(.caption.weight(.semibold)).lineLimit(1)
                if let model = currentModel(for: agent) {
                    Text(model).font(.caption2.monospaced()).foregroundStyle(FortPalette.faint).lineLimit(1)
                }
                Text(detail).font(.caption2).foregroundStyle(FortPalette.muted).lineLimit(2)
            }
            Spacer()
            HStack(spacing: 4) { Circle().fill(state.color).frame(width: 6, height: 6); Text(state == .idle ? "Ready" : state.label) }
                .font(.caption2).foregroundStyle(state.color)
        }
        .padding(10)
        .background(FortPalette.panel, in: RoundedRectangle(cornerRadius: 8))
        .overlay(RoundedRectangle(cornerRadius: 8).stroke(FortPalette.line))
    }

    private func commandMachineCard(_ name: String, detail: String, ready: Bool) -> some View {
        HStack(spacing: 9) {
            Image(systemName: "desktopcomputer").foregroundStyle(FortPalette.muted).frame(width: 25)
            VStack(alignment: .leading, spacing: 2) {
                Text(name).font(.caption.weight(.semibold))
                Text(detail.isEmpty ? "execution node" : detail).font(.caption2).foregroundStyle(FortPalette.muted).lineLimit(1)
            }
            Spacer()
            Circle().fill(ready ? FortPalette.accepted : FortPalette.failed).frame(width: 6, height: 6)
        }
        .padding(10)
        .background(FortPalette.panel, in: RoundedRectangle(cornerRadius: 8))
        .overlay(RoundedRectangle(cornerRadius: 8).stroke(FortPalette.line))
    }

    private var selectedConversation: RunSummary? {
        if selectedConversationID == newConversationID { return nil }
        if let selected = conversationRuns.first(where: { $0.id == selectedConversationID }) { return selected }
        return conversationRuns.first
    }

    private func beginNewConversation() {
        composeText = ""
        selectedConversationID = newConversationID
        selectedAgent = ""
        selectedProfileID = ""
        selectedMachine = ""
        route = .deck
    }

    private func selectConversation(_ run: RunSummary) {
        selectedConversationID = run.id
        applyConversationSelection(run)
        route = .deck
    }

    private func focusFirstAttention() {
        if let gate = board.gates.first,
           let run = conversationRuns.first(where: { $0.id == gate.runID }) {
            selectConversation(run)
            return
        }
        if let run = failedRuns.first { selectConversation(run) }
    }

    private func gatesForConversation(_ run: RunSummary) -> [GateItem] {
        board.gates.filter { $0.runID == run.id }
    }

    private func needsApproval(_ run: RunSummary) -> Bool {
        !gatesForConversation(run).isEmpty
    }

    private func conversationPrompt(_ run: RunSummary) -> String {
        let body = run.body?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
        guard !body.isEmpty else { return title(run) }
        var lines = body.components(separatedBy: .newlines)
        if lines.first?.trimmingCharacters(in: .whitespacesAndNewlines) == title(run) {
            lines.removeFirst()
        }
        let prompt = lines.joined(separator: "\n").trimmingCharacters(in: .whitespacesAndNewlines)
        return prompt.isEmpty ? title(run) : prompt
    }

    private func conversationAgent(_ run: RunSummary) -> String {
        run.agent.hasPrefix("flow:") ? "Fort" : run.agent
    }

    private func conversationResponse(_ run: RunSummary) -> String {
        if let failure = exactFailureReason(for: run) { return failure }
        if let message = meaningfulConversationEvents(for: run)
            .last(where: { $0.type.lowercased() == "message" })?.data?
            .trimmingCharacters(in: .whitespacesAndNewlines), !message.isEmpty {
            return embeddedErrorMessage(message) ?? message
        }
        switch conversationActivity(run) {
        case .pausedForReview:
            return "I reached a checkpoint and need your direction before I continue."
        case .working:
            return "I’m on it. The durable activity below shows exactly what is happening."
        case .starting:
            return "Fort accepted this conversation and is preparing the first execution step."
        case .finished:
            return "The assignment is finished and its accepted checkpoints are recorded."
        case .failed:
            return exactFailureReason(for: run) ?? "The assignment failed without a recorded error reason."
        case .canceled:
            return "This assignment was canceled."
        case .paused:
            return "This assignment is paused."
        case .ready:
            return "This assignment is ready for the next direction."
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

    private var crewColumn: some View {
        VStack(alignment: .leading, spacing: 10) {
            Text("CREW").deckSection()
            ForEach(crewNames, id: \.self) { crewRow($0) }
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
            pageHeader("Assign work", subtitle: "Name the outcome; Fort handles the machinery.")
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

    private var conversationRuns: [RunSummary] {
        FortConversationOrdering.newestFirst(
            board.runs,
            gates: board.gates,
            events: conversationEvents
        )
    }
    private var workingRuns: [RunSummary] { board.runs.filter { $0.status == "running" } }
    private var failedRuns: [RunSummary] {
        FortAttention.recentFailures(in: board.runs, gates: board.gates)
    }
    private var attentionCount: Int { board.gates.count + failedRuns.count }
    private var attentionEmptyMessage: String {
        guard !workingRuns.isEmpty else { return "All quiet — nothing needs you." }
        return "That's everything — \(workingRuns.count) crew member\(workingRuns.count == 1 ? " is" : "s are") working and \(workingRuns.count == 1 ? "doesn't" : "don't") need you."
    }
    private var crewNames: [String] {
		let runAgents = board.runs.map(\.agent).filter { !$0.isEmpty && !$0.hasPrefix("flow:") }
		return Array(Set(runAgents + machines.flatMap { $0.agents ?? [] })).sorted()
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
        !selectedProfileID.isEmpty && (selectedProfile.map(profileIsReady) != true)
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
    private func agentState(_ agent: String) -> FortProjectState {
        let runs = board.runs.filter { $0.agent == agent }
        if runs.contains(where: { run in board.gates.contains { $0.runID == run.id } }) { return .needsYou }
        if runs.contains(where: { conversationActivity($0).projectState == .working }) { return .working }
        return .idle
    }
    private var fortOrbState: FortProjectState {
        board.runs.contains(where: { conversationActivity($0).projectState == .working }) ? .working : .idle
    }
    private func title(_ run: RunSummary) -> String { run.title.isEmpty ? run.id : run.title }
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
    private func currentModel(for agent: String) -> String? {
        if let run = selectedConversation,
           conversationAgent(run).caseInsensitiveCompare(agent) == .orderedSame,
           let model = run.model, !model.isEmpty {
            return model
        }
        if let model = conversationRuns.first(where: { $0.agent == agent && $0.model?.isEmpty == false })?.model {
            return model
        }
        return profiles.first(where: { $0.agent == agent && profileIsReady($0) })?.model
    }
    private func gateTitle(_ gate: GateItem) -> String { gate.nodeID.replacingOccurrences(of: "_", with: " ").replacingOccurrences(of: "-", with: " ").capitalized }
    private func conversationActivity(_ run: RunSummary) -> FortConversationActivity {
        FortConversationActivity.resolve(run: run, gates: board.gates, events: conversationEvents)
    }
    private func activityLine(_ run: RunSummary) -> String {
        conversationActivity(run).label
    }
    private func conversationTimestamp(_ run: RunSummary) -> String {
        var candidates = [run.updatedAt, run.createdAt].compactMap { $0 }
        candidates.append(contentsOf: conversationEvents.lazy.filter { $0.runID == run.id }.map(\.time))
        candidates.append(contentsOf: board.gates.lazy.filter { $0.runID == run.id }.compactMap(\.since))
        let latest = candidates.max { parseEventDate($0) < parseEventDate($1) }
        return FortTime.relative(latest)
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
              ["turn.started", "turn.completed"].contains(type) else {
            return nil
        }
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
        case "started":
            return data.isEmpty ? "Execution started" : "\(data.capitalized) started"
        case "stdout":
            return structuredStdoutType(event.data) == "turn.completed" ? "Turn completed" : "Turn started"
		case "stderr":
			return data.isEmpty ? "Provider diagnostic output" : data
        case "tool":
            let name = (object?["name"] as? String)?.trimmingCharacters(in: .whitespacesAndNewlines)
            let summary = (object?["summary"] as? String)?.trimmingCharacters(in: .whitespacesAndNewlines)
            if let name, !name.isEmpty, let summary, !summary.isEmpty { return "\(name) — \(summary)" }
            if let name, !name.isEmpty { return name }
            return data.isEmpty ? "Tool activity" : data
        case "subagent":
            let agent = (object?["agent"] as? String)?.trimmingCharacters(in: .whitespacesAndNewlines)
            let description = (object?["description"] as? String)?.trimmingCharacters(in: .whitespacesAndNewlines)
            if let description, !description.isEmpty, let agent, !agent.isEmpty { return "\(agent) — \(description)" }
            if let description, !description.isEmpty { return description }
            return data.isEmpty ? "Subagent started" : data
        case "message":
            return embeddedErrorMessage(event.data) ?? (data.isEmpty ? "Agent message" : data)
        case "gate":
            let decision = (object?["decision"] as? String)?.lowercased()
            let note = (object?["note"] as? String)?.trimmingCharacters(in: .whitespacesAndNewlines)
            if decision == "approved" { return "Review approved" }
            if decision == "rejected", let note, !note.isEmpty { return "Changes requested — \(note)" }
            if decision == "rejected" { return "Changes requested" }
            return data.isEmpty ? "Review checkpoint" : data
        case "error":
            return data.isEmpty ? "Execution failed without a recorded reason" : data
        case "exited":
            let code = event.code ?? 0
            return code == 0 ? "Execution exited successfully" : "Execution exited with code \(code)"
        default:
            return data.isEmpty ? event.type : data
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
        case "error": return FortPalette.failed
		case "stderr": return FortPalette.needsYou
        case "message" where embeddedErrorMessage(event.data) != nil: return FortPalette.failed
        case "gate": return FortPalette.needsYou
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
        if let exit = events.last(where: { $0.type.lowercased() == "exited" && ($0.code ?? 0) != 0 }) {
            return "Execution exited with code \(exit.code ?? 0)."
        }
        return nil
    }
    private func embeddedErrorMessage(_ data: String?) -> String? {
        guard let object = jsonObject(data),
              (object["type"] as? String)?.lowercased() == "error" else {
            return nil
        }
        if let error = object["error"] as? [String: Any],
           let message = error["message"] as? String,
           !message.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
            return message
        }
        if let message = object["message"] as? String,
           !message.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
            return message
        }
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
                // A dropped SSE connection is recoverable. The cursor keeps the
                // next connection append-only and deduplicated.
            }

            do {
                try await Task.sleep(nanoseconds: 750_000_000)
            } catch {
                return
            }
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
			// Live SSE remains authoritative; detail replay is a selected-run
			// recovery path when the stream reconnects or starts behind history.
		}
	}

    private func reload() async {
        do {
            async let nextSummary = client.summary(); async let nextBoard = client.board()
            async let nextBacklog = client.backlog(); async let nextMachines = client.machines(); async let nextMetrics = client.metrics(); async let nextPlaybooks = client.playbooks()
            async let nextProfiles = client.profiles()
            let firstProfileLoad = profiles.isEmpty
            summary = try await nextSummary; board = try await nextBoard; backlog = try await nextBacklog
            machines = try await nextMachines; metrics = try await nextMetrics; playbooks = try await nextPlaybooks; lastError = nil
            profiles = (try? await nextProfiles) ?? []
            if firstProfileLoad, selectedConversationID != newConversationID, let run = selectedConversation {
                applyConversationSelection(run)
            }
        } catch { lastError = friendly(error) }
    }

    private func sendConversation() async {
        let text = composeText.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !text.isEmpty else { return }
        guard !composerSelectionIsInvalid else {
            lastError = "Choose a ready model for the selected agent before sending."
            return
        }
        busy = true; defer { busy = false }
        do {
            let profile = selectedProfile
            let result = try await client.chat(ChatRequest(
                text: text,
                agent: profile?.agent,
                profile: profile?.id,
                machine: selectedMachine.isEmpty ? nil : selectedMachine
            ))
            switch result.handoffOutcome {
            case .answer(let answer):
                inlineAnswer = answer
                lastError = nil
            case .failure(let message):
                inlineAnswer = nil
                lastError = message
            case .assignment:
                composeText = ""
                inlineAnswer = nil
                selectedConversationID = result.runID
                lastError = nil
                await reload()
                if let run = conversationRuns.first(where: { $0.id == result.runID }) {
                    applyConversationSelection(run)
                }
            }
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
        case FortClientError.httpStatus(let status, let body, let requestID):
            let correlation = requestID.map { " Request ID \($0)." } ?? ""
            let reason = body.trimmingCharacters(in: .whitespacesAndNewlines)
            return reason.isEmpty ? "Server error (\(status)).\(correlation)" : "\(reason)\(correlation)"
        case FortClientError.nonHTTPResponse: return "Unexpected response."
        case let url as URLError where [.cannotConnectToHost, .cannotFindHost, .networkConnectionLost].contains(url.code): return "Fort not reachable — is the service running?"
        default: return error.localizedDescription
        }
    }
}

private struct FortAgentOrbAvatar: View {
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    let name: String
    let state: FortProjectState
    let size: CGFloat

    var body: some View {
        let pulsing = FortOrbMotion.shouldPulse(state: state)
        let spatialMotion = FortOrbMotion.allowsSpatialMotion(state: state, reduceMotion: reduceMotion)
        TimelineView(.animation(minimumInterval: 1 / 30, paused: !pulsing)) { timeline in
            let phase = timeline.date.timeIntervalSinceReferenceDate
            let energy = pulsing ? (sin(phase * 1.1) + 1) / 2 : 0
            let drift = spatialMotion ? sin(phase * 1.25) * 3.5 : 0
            ZStack {
                Image("FortAgentOrb")
                    .resizable()
                    .scaledToFill()
                    .clipShape(Circle())
                    .rotationEffect(.degrees(drift))
                    .scaleEffect(spatialMotion ? 0.99 + energy * 0.045 : 1)

                Circle()
                    .stroke(state.color, lineWidth: size < 24 ? 1.2 : 1.8)

                if state == .working {
                    Circle()
                        .trim(from: 0, to: 0.14)
                        .stroke(Color.white, style: StrokeStyle(lineWidth: 2, lineCap: .round))
                        .rotationEffect(.degrees(spatialMotion ? phase * 78 : 18))
                        .opacity(pulsing ? 0.72 + energy * 0.28 : 0.72)
                }
            }
            .shadow(
                color: state.color.opacity(pulsing ? 0.2 + energy * 0.22 : 0.18),
                radius: size * (pulsing ? 0.12 + energy * 0.1 : 0.12)
            )
        }
        .frame(width: size, height: size)
        .accessibilityLabel("\(name), \(state.label)")
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
    func conversationChip() -> some View { font(.caption2.weight(.medium)).foregroundStyle(FortPalette.body).padding(.horizontal, 8).padding(.vertical, 5).background(FortPalette.canvas, in: RoundedRectangle(cornerRadius: 6)).overlay(RoundedRectangle(cornerRadius: 6).stroke(FortPalette.outline)) }
}
