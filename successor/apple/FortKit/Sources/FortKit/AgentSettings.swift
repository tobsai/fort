import SwiftUI

public struct ModelOption: Codable, Equatable, Sendable, Identifiable {
    public let id: String
    public let name: String
    public let availability: String
    public var detail: String? = nil
}
public struct ModelCatalog: Codable, Equatable, Sendable {
    public let provider: String
    public let state: String
    public let models: [ModelOption]
    public var checkedAt: String? = nil
    public var detail: String? = nil
    public func canSelect(_ id: String) -> Bool { !provider.isEmpty && state == "ready" && models.contains { $0.id == id && $0.availability == "available" } }
    public func options(preserving selected: String) -> [ModelOption] {
        var result = state == "ready" ? models : []
        if !selected.isEmpty && !result.contains(where:{$0.id == selected}) {
            result.append(ModelOption(id:selected,name:selected,availability:state == "ready" ? "unavailable" : "unchecked",detail:state == "ready" ? "Not listed by this runtime." : "Availability has not been verified."))
        }
        return result
    }
    static let unchecked = ModelCatalog(provider:"",state:"unavailable",models:[],detail:"Connect to the local runtime to check models.")
}
struct AgentSettingsEditor: View {
    @Bindable var model: MessengerModel
    let agent: Agent?
    @State private var form: WorkspaceForm
    @Environment(\.dismiss) private var dismiss
    init(model: MessengerModel, agent: Agent?) { self.model = model; self.agent = agent; _form = State(initialValue:WorkspaceForm(agent:agent)) }
    private var valid: Bool {
        !form.name.trimmingCharacters(in:.whitespacesAndNewlines).isEmpty && form.name.utf8.count <= 128 &&
        !form.instructions.trimmingCharacters(in:.whitespacesAndNewlines).isEmpty && form.instructions.utf8.count <= AgentSheet.instructionLimit &&
        (model.modelCatalog.canSelect(form.exactModel) || agent?.binding.selectedModel == form.exactModel)
    }
    @State private var showWorkshop = false
    @State private var portraitNote = ""
    private var liveAgent: Agent? { agent.map { current in model.agents.first { $0.agentId == current.agentId } ?? current } }
    private var className: String { agent.map { PauseMenu.className(provider:$0.binding.provider,runtimes:model.runtimes) } ?? model.focusedRuntimeName }
    var body: some View {
        Form {
            #if DEBUG
            if model.isInteractiveFixture { InteractiveFixtureControls(model:model) }
            #endif
            Section {
                HStack(alignment:.center,spacing:16) {
                    Group {
                        if let liveAgent, liveAgent.portrait != nil { AgentPortraitView(model:model,agent:liveAgent,size:96) }
                        else { AgentAvatarView(avatar:form.avatar,size:96) }
                    }.padding(6).hudFrame()
                    VStack(alignment:.leading,spacing:8) {
                        TextField("Name",text:$form.name).font(.system(.title2,design:.rounded).weight(.black))
                        HStack(spacing:6) {
                            Text(className.uppercased()).font(.caption.weight(.heavy))
                                .padding(.horizontal,6).padding(.vertical,2).background(HUDStyle.gold,in:RoundedRectangle(cornerRadius:3)).foregroundStyle(.black)
                            if agent != nil { Image(systemName:"lock.fill").font(.caption).foregroundStyle(.secondary).accessibilityLabel("Class locked") }
                        }
                    }
                }
                HStack(spacing:12) {
                    ForEach(AgentAvatar.allCases) { avatar in
                        Button { form.avatar = avatar } label: {
                            AgentAvatarView(avatar:avatar,size:40).padding(4)
                                .overlay(RoundedRectangle(cornerRadius:6).strokeBorder(form.avatar == avatar ? HUDStyle.gold : .clear,lineWidth:2))
                        }.buttonStyle(.plain).accessibilityLabel(avatar.name + (form.avatar == avatar ? ", selected" : ""))
                    }
                }
                if let liveAgent, model.supportsPortraits, PortraitDrawing.canDraw(liveAgent) {
                    PortraitDrawControls(model:model,agent:liveAgent,note:$portraitNote)
                }
            } header: { Text("Portrait") }
            #if os(macOS)
            if model.canManageProviderAccount && model.accountSignInStatus?.accountSelected != true {
                AccountSignInView(model:model)
            }
            #endif
            Section {
                // Spec 064: a new Agent picks its provider once; an existing Agent shows the one it keeps.
                if agent == nil && model.runtimes.count > 1 {
                    Picker("Class",selection:SwiftUI.Binding(get:{ model.focusedRuntime?.provider ?? "" },set:{ value in form.exactModel = ""; Task { await model.focusRuntime(value) } })) {
                        ForEach(model.runtimes,id:\.provider) { runtime in Text(runtime.name).tag(runtime.provider) }
                    }.pickerStyle(.segmented).disabled(!model.canChangeAccountProvider).accessibilityIdentifier("agent-provider")
                }
                Text(agent == nil ? "An agent keeps its class for good. Fort never switches it to another." : "Locked to \(className). Fort never switches it to another.").font(.caption).foregroundStyle(.secondary)
            } header: { Text("Class") }
            Section {
                let gear = AgentSheet.gear(model.modelCatalog.options(preserving:form.exactModel))
                if gear.isEmpty { Text(model.modelCatalog.detail ?? "No models listed yet. Refresh them in the Workshop.").font(.callout).foregroundStyle(.secondary) }
                ForEach(gear) { item in
                    Button { form.exactModel = item.id } label: {
                        HStack(spacing:10) {
                            Image(systemName:form.exactModel == item.id ? "largecircle.fill.circle" : "circle").foregroundStyle(form.exactModel == item.id ? HUDStyle.gold : .secondary)
                            VStack(alignment:.leading,spacing:2) {
                                Text(item.name).font(.body.weight(.semibold))
                                if let note = item.note { Text(note).font(.caption).foregroundStyle(.secondary) }
                            }
                            Spacer(minLength:0)
                        }.contentShape(Rectangle()).opacity(item.selectable || form.exactModel == item.id ? 1 : 0.5)
                    }
                    .buttonStyle(.plain).disabled(!item.selectable)
                    .accessibilityIdentifier("agent-gear-\(item.id)")
                    .accessibilityAddTraits(form.exactModel == item.id ? .isSelected : [])
                }
                if let requirement = model.modelAccountRequirement { Text(requirement).font(.caption) }
            } header: { Text("Gear") } footer: {
                Text(model.isInteractiveFixture ? "Simulated availability only. Sending exercises local QA replies and does not verify a real model." : "Checked again when you send. Fort never swaps in another model.")
            }
            Section {
                TextEditor(text:$form.instructions).frame(minHeight:140).accessibilityLabel("Agent identity")
                if let note = AgentSheet.sizeNote(form.instructions) {
                    Text(note).font(.caption).foregroundStyle(form.instructions.utf8.count > AgentSheet.instructionLimit ? .red : .secondary)
                }
            } header: { Text(AgentSheet.instructionsTitle(name:form.name)) } footer: {
                Text("Role, tone and preferences. Applies to future messages; permissions and earlier turns stay unchanged.")
            }
            Section {
                WorkshopDrawer(isExpanded:$showWorkshop,identifier:"agent-workshop") {
                    Text(model.isInteractiveFixture ? "Simulated catalog · no provider account" : "Runtime: " + model.focusedRuntimeName).font(.caption)
                    if let detail = model.modelCatalog.detail { Text(detail).font(.caption).foregroundStyle(.secondary) }
                    Button(model.loadingModels ? "Checking models…" : "Refresh models") { Task { await model.refreshModels() } }.disabled(!model.canRefreshModels)
                    Text(model.status).font(.caption).foregroundStyle(.secondary)
                }
            }
        }.formStyle(.grouped)
        .navigationTitle(agent?.name ?? "Recruit")
        .toolbar { ToolbarItem(placement:.confirmationAction) { Button("Save") { Task { if await form.save(using:model) { dismiss() } } }.disabled(!valid || model.workspaceSaving) } }
        .task {
            if let agent { await model.focusRuntime(agent.binding.provider) }
            await model.refreshModels()
        }
    }
}

enum PortraitDrawing {
    /// Only ChatGPT (Codex) Agents can draw; Claude has no image generation.
    static func canDraw(_ agent: Agent) -> Bool { agent.binding.provider == "codex" }
    static func buttonTitle(_ agent: Agent) -> String { agent.portraitDraw?.isDrawing == true ? "Drawing…" : agent.portrait == nil ? "Draw portrait" : "Redraw portrait" }
    static let costNote = "Fort draws one picture on your ChatGPT plan. An image uses more of your plan than a message."
}
struct PortraitDrawControls: View {
    let model: MessengerModel
    let agent: Agent
    @Binding var note: String
    var body: some View {
        let drawing = agent.portraitDraw?.isDrawing == true
        TextField("Look and props (optional)",text:$note,axis:.vertical).lineLimit(1...3).disabled(drawing)
            .accessibilityIdentifier("portrait-note")
        HStack {
            Button(PortraitDrawing.buttonTitle(agent)) { Task { await model.drawPortrait(agent,note:note) } }
                .disabled(drawing || model.portraitBusy || note.utf8.count > 500).accessibilityIdentifier("portrait-draw")
            if agent.portrait != nil {
                Button("Use stock icon",role:.destructive) { Task { await model.removePortrait(agent) } }
                    .disabled(drawing || model.portraitBusy).accessibilityIdentifier("portrait-remove")
            }
            if drawing { ProgressView().controlSize(.small) }
        }
        if let draw = agent.portraitDraw, !draw.isDrawing { Text(draw.detail).font(.caption).foregroundStyle(.red) }
        Text(PortraitDrawing.costNote).font(.caption).foregroundStyle(.secondary)
    }
}

#if DEBUG
struct InteractiveFixtureControls: View {
    @Bindable var model: MessengerModel
    var body: some View {
        Section("Interactive QA · no provider") {
            Picker("Catalog scenario",selection:SwiftUI.Binding(get:{model.fixtureCatalogScenario},set:{ scenario in Task { await model.setFixtureCatalog(scenario) } })) {
                ForEach(FixtureCatalogScenario.allCases) { Text($0.name).tag($0) }
            }.accessibilityIdentifier("qa-catalog-scenario")
            Text("Edits and replies are saved only in this temporary QA session. No enrollment, Keychain or network access.").font(.caption)
        }
    }
}
#endif
