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
        !form.instructions.trimmingCharacters(in:.whitespacesAndNewlines).isEmpty && form.instructions.utf8.count <= 16_384 &&
        (model.modelCatalog.canSelect(form.exactModel) || agent?.binding.selectedModel == form.exactModel)
    }
    var body: some View {
        Form {
            #if DEBUG
            if model.isInteractiveFixture { InteractiveFixtureControls(model:model) }
            #endif
            Section("Identity") {
                TextField("Name",text:$form.name)
                TextEditor(text:$form.instructions).frame(minHeight:140).accessibilityLabel("Agent identity")
                Text("Describe this agent’s role, tone and preferences. Used as instructions for future messages; permissions and earlier turns stay unchanged.").font(.caption).foregroundStyle(.secondary)
                Text("\(form.instructions.utf8.count) / 16,384 bytes").font(.caption).foregroundStyle(form.instructions.utf8.count > 16_384 ? .red : .secondary)
            }
            #if os(macOS)
            if model.canManageProviderAccount && model.accountSignInStatus?.accountSelected != true {
                AccountSignInView(model:model)
            }
            #endif
            Section("Model") {
                // Spec 064: a new Agent picks its provider once; an existing Agent shows the one it keeps.
                if agent == nil && model.runtimes.count > 1 {
                    Picker("Provider",selection:SwiftUI.Binding(get:{ model.focusedRuntime?.provider ?? "" },set:{ value in form.exactModel = ""; Task { await model.focusRuntime(value) } })) {
                        ForEach(model.runtimes,id:\.provider) { runtime in Text(runtime.name).tag(runtime.provider) }
                    }.disabled(!model.canChangeAccountProvider).accessibilityIdentifier("agent-provider")
                    Text("An Agent keeps this provider for good. Fort never switches it to another.").font(.caption).foregroundStyle(.secondary)
                }
                Text(model.isInteractiveFixture ? "Simulated catalog · no provider account" : "Runtime: " + model.focusedRuntimeName).font(.caption)
                Picker("Model",selection:$form.exactModel) {
                    if form.exactModel.isEmpty { Text("Choose a model").tag("") }
                    ForEach(model.modelCatalog.options(preserving:form.exactModel)) { choice in
                        Text(choice.name + (choice.availability == "available" ? "" : choice.availability == "unchecked" ? " · not checked" : " · unavailable")).tag(choice.id).disabled(choice.availability != "available")
                    }
                }
                if let choice = model.modelCatalog.options(preserving:form.exactModel).first(where:{$0.id == form.exactModel}), let detail = choice.detail { Text(detail).font(.caption) }
                if let detail = model.modelCatalog.detail { Text(detail).font(.caption).foregroundStyle(.secondary) }
                if model.modelCatalog.state == "ready" { Text(model.isInteractiveFixture ? "Simulated availability only. Sending exercises local QA replies and does not verify a real model." : "Listed by the connected runtime. Availability is checked again when you send; Fort never substitutes another model.").font(.caption).foregroundStyle(.secondary) }
                if let requirement = model.modelAccountRequirement { Text(requirement).font(.caption) }
                Button(model.loadingModels ? "Checking models…" : "Refresh models") { Task { await model.refreshModels() } }.disabled(!model.canRefreshModels)
            }
            Section("Avatar") {
                HStack(spacing:18) {
                    ForEach(AgentAvatar.allCases) { avatar in
                        Button { form.avatar = avatar } label: {
                            VStack { AgentAvatarView(avatar:avatar,size:56); Text(avatar.name).font(.caption); Image(systemName:form.avatar == avatar ? "checkmark.circle.fill" : "circle") }
                        }.buttonStyle(.plain).accessibilityLabel(avatar.name + (form.avatar == avatar ? ", selected" : ""))
                    }
                }
                Text("Bundled avatars animate gently and respect Reduce Motion.").font(.caption).foregroundStyle(.secondary)
            }
            Text(model.status).font(.caption).foregroundStyle(.secondary)
        }.formStyle(.grouped)
        .navigationTitle(agent?.name ?? "New agent")
        .toolbar { ToolbarItem(placement:.confirmationAction) { Button("Save") { Task { if await form.save(using:model) { dismiss() } } }.disabled(!valid || model.workspaceSaving) } }
        .task {
            if let agent { await model.focusRuntime(agent.binding.provider) }
            await model.refreshModels()
        }
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
