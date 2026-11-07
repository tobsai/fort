import SwiftUI

/// Spec 068: one Agent's memory, opened from its settings in Setup, Agents. A picker at the top steps between Agents,
/// so every Agent's memory can be checked and switched without going back to the list.
struct AgentMemoryView: View {
    @Bindable var model: MessengerModel
    @State private var agentID: String
    @State private var selection: String = ""
    @State private var carry = true
    @State private var query = ""
    @State private var note = ""
    @State private var kind = "fact"
    @State private var forgetting: MemoryNote?
    init(model: MessengerModel, agentID: String) { self.model = model; _agentID = State(initialValue:agentID) }

    private var page: AgentMemoryPage? { model.memoryPages[agentID] }

    var body: some View {
        Form {
            if model.agents.count > 1 {
                Section {
                    Picker("Agent",selection:$agentID) {
                        ForEach(model.agents.filter { !$0.isOnHub }) { agent in Text(agent.name).tag(agent.agentId) }
                    }.accessibilityIdentifier("memory-agent")
                } footer: { Text("Pick another agent to see its memory.") }
            }
            if let page {
                systemSection(page)
                if page.state == "ready", let provider = page.provider { notesSections(page,provider:provider) }
                else if page.state == "off" { Section { Text("This agent starts every conversation fresh.").foregroundStyle(FortTheme.muted) } }
            } else {
                Section { ProgressView("Checking memory…") }
            }
            if let message = model.memoryMessage { Section { CopyableText(message).font(.callout).foregroundStyle(FortTheme.failed) } }
        }
        .formStyle(.grouped)
        .navigationTitle("Memory")
        .task(id:agentID) { model.memoryMessage = nil; query = ""; await model.loadMemory(agentID:agentID); selection = page?.choice.providerId ?? "" }
        .onChange(of:page?.choice.providerId) { _, current in if let current { selection = current } }
        .confirmationDialog("Forget this note?",isPresented:SwiftUI.Binding(get:{ forgetting != nil },set:{ if !$0 { forgetting = nil } }),presenting:forgetting) { item in
            Button("Forget",role:.destructive) { Task { await model.forgetNote(agentID:agentID,entryID:item.entryId) } }
        } message: { _ in Text("The agent won’t recall it again.") }
    }

    @ViewBuilder private func systemSection(_ page: AgentMemoryPage) -> some View {
        let chosen = page.providers.first { $0.id == selection }
        let changed = !selection.isEmpty && selection != page.choice.providerId
        let carryable = AgentMemoryCopy.canCarry(from:page.choice.providerId,to:chosen,providers:page.providers)
        Section {
            Picker("Memory system",selection:$selection) {
                ForEach(page.providers) { system in Text(system.name).tag(system.id) }
                if page.provider == nil { Text("Missing: \(page.choice.providerId)").tag(page.choice.providerId) }
            }.accessibilityIdentifier("memory-system")
            Text(AgentMemoryCopy.about(selected:chosen,page:page)).font(.callout).foregroundStyle(page.state == "missing" && !changed ? FortTheme.failed : FortTheme.muted)
            if changed, let chosen {
                if carryable { Toggle("Bring the notes along",isOn:$carry).accessibilityIdentifier("memory-carry") }
                Button("Switch to \(chosen.name)") { Task { await model.switchMemory(agentID:agentID,to:chosen.id,carry:carryable ? carry : nil) } }
                    .modifier(GoldButton(on:true)).disabled(model.memoryBusy).accessibilityIdentifier("memory-switch")
            }
        }
    }

    @ViewBuilder private func notesSections(_ page: AgentMemoryPage, provider: MemorySystem) -> some View {
        Section {
            if provider.can("search") {
                TextField("Search notes",text:$query,prompt:Text("Search notes")).labelsHidden().onSubmit { Task { await model.loadMemory(agentID:agentID,query:query.trimmingCharacters(in:.whitespacesAndNewlines)) } }
                    .textFieldStyle(.roundedBorder).accessibilityIdentifier("memory-search")
            }
            if page.entries.isEmpty {
                Text(query.isEmpty ? "Nothing kept yet. The agent adds notes as you talk, or add one below." : "No note matches.").foregroundStyle(FortTheme.muted)
            }
            ForEach(page.entries) { entry in
                HStack(alignment:.firstTextBaseline,spacing:10) {
                    CaptionBox(text:entry.kind == "event" ? "Event" : "Fact")
                    VStack(alignment:.leading,spacing:2) {
                        Text(entry.text).textSelection(.enabled)
                        Text(Self.day(entry.createdAt)).font(.caption).foregroundStyle(FortTheme.muted)
                    }
                    Spacer(minLength:0)
                    if provider.can("forget") {
                        Button("Forget") { forgetting = entry }.buttonStyle(.borderless).foregroundStyle(FortTheme.link).disabled(model.memoryBusy)
                            .accessibilityIdentifier("memory-forget-\(entry.entryId)")
                    }
                }
            }
        } header: { Text(query.isEmpty ? "Notes · \(AgentMemoryCopy.notes(page.total))" : "Notes matching “\(query)”") }
        if provider.can("remember") {
            Section {
                TextField("New note",text:$note,prompt:Text("Something this agent should remember"),axis:.vertical).lineLimit(1...4)
                    .labelsHidden().textFieldStyle(.roundedBorder).accessibilityIdentifier("memory-note")
                HStack {
                    Picker("Kind",selection:$kind) { Text("Fact").tag("fact"); Text("Event").tag("event") }.pickerStyle(.segmented).labelsHidden().frame(maxWidth:180)
                    Spacer()
                    Button("Add note") { Task { if await model.rememberNote(agentID:agentID,text:note,kind:kind) { note = "" } } }
                        .disabled(note.trimmingCharacters(in:.whitespacesAndNewlines).isEmpty || note.utf8.count > 600 || model.memoryBusy)
                        .accessibilityIdentifier("memory-add")
                }
            } header: { Text("Add a note") }
        }
    }

    static func day(_ stamp: String) -> String {
        let parser = ISO8601DateFormatter(); parser.formatOptions = [.withInternetDateTime,.withFractionalSeconds]
        guard let date = parser.date(from:stamp) ?? ISO8601DateFormatter().date(from:stamp) else { return "" }
        return date.formatted(date:.abbreviated,time:.omitted)
    }
}

/// The Memory row in an Agent's settings: the system it uses and how many notes, opening the memory page.
struct AgentMemoryRow: View {
    @Bindable var model: MessengerModel
    let agent: Agent
    var body: some View {
        Section {
            NavigationLink { AgentMemoryView(model:model,agentID:agent.agentId) } label: {
                VStack(alignment:.leading,spacing:2) {
                    Text("Memory").font(.body.weight(.semibold))
                    Text(AgentMemoryCopy.line(model.memoryStatuses[agent.agentId])).font(.caption).foregroundStyle(FortTheme.muted)
                }
            }.accessibilityIdentifier("agent-memory")
        } header: { Text("Memory") } footer: { Text("What this agent keeps between conversations, and which memory system keeps it.") }
        .task { if model.memoryStatuses[agent.agentId] == nil { await model.refreshMemoryOverview() } }
    }
}
