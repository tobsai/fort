import SwiftUI

struct WorkspaceEditor: View {
    @Bindable var model: MessengerModel
    @Environment(\.dismiss) private var dismiss
    @State private var kind = "Agent"
    @State private var editingID = "new"
    @State private var name = ""
    @State private var exactModel = "gpt-6-astra"
    @State private var instructions = ""
    @State private var participants: Set<String> = []
    private var valid: Bool {
        !name.trimmingCharacters(in:.whitespacesAndNewlines).isEmpty &&
        (kind == "Conversation" ? participants.count >= 2 && participants.count <= 8 : !instructions.trimmingCharacters(in:.whitespacesAndNewlines).isEmpty) &&
        (kind != "Agent" || !exactModel.trimmingCharacters(in:.whitespacesAndNewlines).isEmpty)
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
                        else { ForEach(model.skills) { Text($0.name).tag($0.id) } }
                    }
                }
                TextField("Name", text:$name)
                if kind == "Agent" { TextField("Exact model ID", text:$exactModel).autocorrectionDisabled() }
                if kind == "Conversation" {
                    Section("Participants") {
                        ForEach(model.agents) { agent in
                            Toggle(agent.name,isOn:SwiftUI.Binding(get:{ participants.contains(agent.id) },set:{ if $0 { participants.insert(agent.id) } else { participants.remove(agent.id) } }))
                        }
                    }
                    Text("Choose one recipient when sending each message.").font(.caption)
                } else {
                    Section(kind == "Skill" ? "Instruction skill" : "Agent instructions") {
                        TextEditor(text:$instructions).frame(minHeight:130).accessibilityLabel("Instructions")
                    }
                    if kind == "Skill" { Text("Selected instructions apply only to the messages you choose. Skills do not install tools or execute files.").font(.caption) }
                }
                Text(model.status).font(.caption).foregroundStyle(.secondary)
            }.formStyle(.grouped)
            .navigationTitle("Fort workspace")
            .toolbar {
                ToolbarItem(placement:.cancellationAction) { Button("Cancel") { dismiss() }.disabled(model.workspaceSaving) }
                ToolbarItem(placement:.confirmationAction) { Button("Save") { Task { await save() } }.disabled(!valid || model.workspaceSaving) }
            }
        }.frame(minWidth:440,minHeight:470)
        .onChange(of:kind) { _,_ in editingID = "new"; load() }
        .onChange(of:editingID) { _,_ in load() }
    }
    private func load() {
        name = ""; instructions = ""; participants = []
        if kind == "Agent", let agent = model.agents.first(where:{$0.id == editingID}) { name = agent.name; instructions = agent.instructions ?? ""; exactModel = agent.binding.selectedModel }
        if kind == "Skill", let skill = model.skills.first(where:{$0.id == editingID}) { name = skill.name; instructions = skill.instructions }
    }
    private func save() async {
        let saved: Bool
        if kind == "Agent" { saved = await model.saveAgent(model.agents.first(where:{$0.id == editingID}),name:name,model:exactModel,instructions:instructions) }
        else if kind == "Skill" { saved = await model.saveSkill(model.skills.first(where:{$0.id == editingID}),name:name,instructions:instructions) }
        else { saved = await model.createConversation(name:name,participantAgentIDs:model.agents.filter { participants.contains($0.id) }.map(\.id)) }
        if saved { dismiss() }
    }
}
