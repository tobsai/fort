import SwiftUI
import Observation

struct WorkspaceEditor: View {
    @Bindable var model: MessengerModel
    @Environment(\.dismiss) private var dismiss
    @State private var showBundles = false
    @State private var savedSkill: InstructionSkill?
    @State private var kind = "Skill"
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
                Picker("Create or edit", selection:$kind) { Text("Skill").tag("Skill"); Text("Conversation").tag("Conversation") }
                    .pickerStyle(.segmented)
                if kind != "Conversation" {
                    Picker("Item", selection:$editingID) {
                        Text("Create new").tag("new")
                        if kind == "Agent" { ForEach(model.agents) { Text($0.name).tag($0.id) } }
                        else { ForEach(model.skills.filter(\.canEditInstructions)) { Text($0.name).tag($0.id) } }
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
                    if kind == "Skill" { Text("Saving creates a complete private instruction bundle. Review and enable its exact revision before selecting it for a message or routine.").font(.caption) }
                }
                if kind == "Skill" { Button("Import and review complete bundles") { showBundles = true } }
                CopyableText(model.status).font(.caption).foregroundStyle(.secondary)
            }.formStyle(.grouped)
            .navigationTitle("Fort workspace")
            .toolbar {
                ToolbarItem(placement:.cancellationAction) { Button("Cancel") { dismiss() }.disabled(model.workspaceSaving) }
                ToolbarItem(placement:.confirmationAction) { Button(kind == "Skill" ? "Save for review" : "Save") { Task { await save() } }.disabled(!valid || model.workspaceSaving) }
            }
        }.frame(minWidth:440,minHeight:470)
        .sheet(isPresented:$showBundles) { SkillBundleLibrary(model:model) }
        .sheet(item:$savedSkill,onDismiss:{ dismiss() }) { SkillBundleReviewView(model:model,skill:$0) }
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
        if saved { if kind == "Skill", let skill = model.lastSavedSkill, skill.bundle != nil { savedSkill = skill } else { dismiss() } }
    }
}

@Observable @MainActor final class WorkspaceForm {
    private enum Source { case agent(Agent?); case skill(InstructionSkill?) }
    private let source: Source
    var name: String
    var exactModel: String
    var instructions: String
    var avatar: AgentAvatar = .orbit
    init(agent: Agent?) {
        source = .agent(agent); avatar = agent?.avatar ?? .orbit; name = agent?.name ?? ""; exactModel = agent?.binding.selectedModel ?? ""; instructions = agent?.instructions ?? ""
    }
    init(skill: InstructionSkill?) {
        source = .skill(skill); name = skill?.name ?? ""; exactModel = ""; instructions = skill?.instructions ?? ""
    }
    func save(using model: MessengerModel) async -> Bool {
        switch source {
        case .agent(let loaded): await model.saveAgent(loaded,name:name,model:exactModel,instructions:instructions,avatar:avatar,provider:loaded == nil ? model.focusedRuntime?.provider : nil)
        case .skill(let loaded): await model.saveSkill(loaded,name:name,instructions:instructions)
        }
    }
}

@Observable @MainActor final class ConversationMemoryForm: Identifiable {
    nonisolated var id: String { conversationID }
    nonisolated let conversationID: String
    private let loaded: ConversationMemory?
    var text: String
    init(conversationID: String, memory: ConversationMemory?) { self.conversationID = conversationID; loaded = memory; text = memory?.text ?? "" }
    func save(using model: MessengerModel) async -> Bool { await model.saveMemory(conversationID:conversationID,memory:loaded,text:text) }
}

struct ConversationMemoryEditor: View {
    @Bindable var model: MessengerModel
    @Bindable var form: ConversationMemoryForm
    @Environment(\.dismiss) private var dismiss
    var body: some View {
        NavigationStack {
            Form {
                Text("Write notes you want the selected agents to use in this conversation. Other conversations keep their own notes.").font(.callout)
                TextEditor(text:$form.text).frame(minHeight:180).accessibilityLabel("Conversation memory notes")
                Text("Applies to future chat messages. Clearing these notes keeps earlier messages and the memory recorded with accepted turns. Routines keep their saved plan.").font(.caption).foregroundStyle(.secondary)
                Text("\(form.text.utf8.count) of 8192 bytes").font(.caption)
                Button("Clear notes") { form.text = "" }.disabled(model.workspaceSaving)
                CopyableText(model.status).font(.caption)
            }.formStyle(.grouped)
            .navigationTitle("Conversation memory")
            .toolbar {
                ToolbarItem(placement:.cancellationAction) { Button("Cancel") { dismiss() }.disabled(model.workspaceSaving) }
                ToolbarItem(placement:.confirmationAction) { Button("Save memory") { Task { if await form.save(using:model) { dismiss() } } }.disabled(model.workspaceSaving || form.text.utf8.count > 8192) }
            }
        }.frame(minWidth:320,minHeight:430)
    }
}
