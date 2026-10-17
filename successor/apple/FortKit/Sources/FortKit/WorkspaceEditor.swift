import SwiftUI
import Observation

struct WorkspaceEditor: View {
    @Bindable var model: MessengerModel
    @Environment(\.dismiss) private var dismiss
    @State private var showBundles = false
    @State private var kind = "Agent"
    @State private var editingID = "new"
    @State private var form = WorkspaceForm(agent:nil)
    @State private var participants: Set<String> = []
    private var valid: Bool {
        !form.name.trimmingCharacters(in:.whitespacesAndNewlines).isEmpty &&
        (kind == "Conversation" ? participants.count >= 2 && participants.count <= 8 : !form.instructions.trimmingCharacters(in:.whitespacesAndNewlines).isEmpty) &&
        (kind != "Agent" || !form.exactModel.trimmingCharacters(in:.whitespacesAndNewlines).isEmpty)
    }
    var body: some View {
        NavigationStack {
            Form {
                Picker("Create or edit", selection:$kind) { Text("Agent").tag("Agent"); Text("Skill").tag("Skill"); Text("Conversation").tag("Conversation") }
                    .pickerStyle(.segmented)
                if kind != "Conversation" {
                    Picker("Item", selection:$editingID) {
                        Text("Create new").tag("new")
                        if kind == "Agent" { ForEach(model.agents) { Text($0.name).tag($0.id) } }
                        else { ForEach(model.skills.filter { $0.bundle == nil }) { Text($0.name).tag($0.id) } }
                    }
                }
                TextField("Name", text:$form.name)
                if kind == "Agent" { TextField("Exact model ID", text:$form.exactModel).autocorrectionDisabled() }
                if kind == "Conversation" {
                    Section("Participants") {
                        ForEach(model.agents) { agent in
                            Toggle(agent.name,isOn:SwiftUI.Binding(get:{ participants.contains(agent.id) },set:{ if $0 { participants.insert(agent.id) } else { participants.remove(agent.id) } }))
                        }
                    }
                    Text("Choose one recipient when sending each message.").font(.caption)
                } else {
                    Section(kind == "Skill" ? "Instruction skill" : "Agent instructions") {
                        TextEditor(text:$form.instructions).frame(minHeight:130).accessibilityLabel("Instructions")
                    }
                    if kind == "Skill" { Text("Selected instructions apply only to the messages you choose. Skills do not install tools or execute files.").font(.caption) }
                }
                if kind == "Skill" { Button("Import and review complete bundles") { showBundles = true } }
                Text(model.status).font(.caption).foregroundStyle(.secondary)
            }.formStyle(.grouped)
            .navigationTitle("Fort workspace")
            .toolbar {
                ToolbarItem(placement:.cancellationAction) { Button("Cancel") { dismiss() }.disabled(model.workspaceSaving) }
                ToolbarItem(placement:.confirmationAction) { Button("Save") { Task { await save() } }.disabled(!valid || model.workspaceSaving) }
            }
        }.frame(minWidth:440,minHeight:470)
        .sheet(isPresented:$showBundles) { SkillBundleLibrary(model:model) }
        .onChange(of:kind) { _,_ in editingID = "new"; load() }
        .onChange(of:editingID) { _,_ in load() }
    }
    private func load() {
        participants = []
        if kind == "Skill" { form = WorkspaceForm(skill:model.skills.first(where:{$0.id == editingID})) }
        else { form = WorkspaceForm(agent:model.agents.first(where:{$0.id == editingID})) }
    }
    private func save() async {
        let saved: Bool
        if kind == "Conversation" { saved = await model.createConversation(name:form.name,participantAgentIDs:model.agents.filter { participants.contains($0.id) }.map(\.id)) }
        else { saved = await form.save(using:model) }
        if saved { dismiss() }
    }
}

@Observable @MainActor final class WorkspaceForm {
    private enum Source { case agent(Agent?); case skill(InstructionSkill?) }
    private let source: Source
    var name: String
    var exactModel: String
    var instructions: String
    init(agent: Agent?) {
        source = .agent(agent); name = agent?.name ?? ""; exactModel = agent?.binding.selectedModel ?? "gpt-6-astra"; instructions = agent?.instructions ?? ""
    }
    init(skill: InstructionSkill?) {
        source = .skill(skill); name = skill?.name ?? ""; exactModel = ""; instructions = skill?.instructions ?? ""
    }
    func save(using model: MessengerModel) async -> Bool {
        switch source {
        case .agent(let loaded): await model.saveAgent(loaded,name:name,model:exactModel,instructions:instructions)
        case .skill(let loaded): await model.saveSkill(loaded,name:name,instructions:instructions)
        }
    }
}
