import SwiftUI
import Observation
import UniformTypeIdentifiers

@Observable @MainActor final class RoutineForm {
    let source: RoutineRevision?
    var definition: RoutineDefinition
    init(revision: RoutineRevision? = nil, agent: Agent? = nil, imported: RoutineDefinition? = nil) {
        source = revision
        definition = imported ?? revision?.definition ?? RoutineDefinition(schemaVersion:1,name:"",agentId:agent?.id ?? "",conversationId:agent?.homeConversationId ?? "",bindingRevisionId:agent?.binding.bindingRevisionId ?? "",instructions:"Follow the declared steps using the supplied inputs.",inputs:[],skills:[],steps:[RoutineStep(stepId:UUID().uuidString.lowercased(),name:"Task",type:"task",instructions:"",inputNames:[],outputStepIds:[])])
    }
    func bind(_ agent: Agent) { definition.agentId = agent.id; definition.conversationId = agent.homeConversationId; definition.bindingRevisionId = agent.binding.bindingRevisionId }
    var boundAgent: Agent? { source?.agentSnapshot.binding.bindingRevisionId == definition.bindingRevisionId ? source?.agentSnapshot : nil }
    func renameInput(at index: Int, to name: String) {
        let old = definition.inputs[index].name; definition.inputs[index].name = name
        for step in definition.steps.indices { definition.steps[step].inputNames = definition.steps[step].inputNames?.map { $0 == old ? name : $0 } }
    }
    func removeStep(_ id: String) {
        definition.steps.removeAll { $0.id == id }
        for index in definition.steps.indices { definition.steps[index].outputStepIds?.removeAll { $0 == id } }
    }
    func moveStep(_ id: String, by offset: Int) {
        guard let index = definition.steps.firstIndex(where:{$0.id == id}), definition.steps.indices.contains(index + offset) else { return }
        definition.steps.swapAt(index,index+offset)
        var prior: Set<String> = []
        for index in definition.steps.indices {
            definition.steps[index].outputStepIds = definition.steps[index].outputStepIds?.filter { prior.contains($0) }
            if definition.steps[index].type == "task" { prior.insert(definition.steps[index].id) }
        }
    }
    func save(using model: MessengerModel) async -> Bool { await model.saveRoutine(source,definition:definition) }
}

private struct RoutineDocument: FileDocument {
    static var readableContentTypes: [UTType] { [.json] }
    var data: Data
    init(_ definition: RoutineDefinition) { let encoder = JSONEncoder(); encoder.outputFormatting = [.prettyPrinted,.sortedKeys]; data = (try? encoder.encode(definition)) ?? Data() }
    init(configuration: ReadConfiguration) throws { guard let data = configuration.file.regularFileContents else { throw ClientError.invalidResponse }; self.data = data }
    func fileWrapper(configuration: WriteConfiguration) throws -> FileWrapper { FileWrapper(regularFileWithContents:data) }
}

struct RoutineLibrary: View {
    @Bindable var model: MessengerModel
    @Environment(\.dismiss) private var dismiss
    @State private var creating = false
    @State private var importing = false
    @State private var imported: RoutineDefinition?
    @State private var editing: RoutineRevision?
    @State private var running: RoutineRevision?
    @State private var scheduling: RoutineRevision?
    @State private var error: String?
    var body: some View {
        NavigationStack {
            List {
                Section {
                    Text("A routine saves an exact Agent, Home, reviewed skills and ordered tasks. Approval steps wait for your decision.")
                    Button("Create routine") { imported = nil; creating = true }.disabled(model.agents.isEmpty)
                    Button("Import routine JSON") { importing = true }
                }
                ForEach(model.routines) { routine in
                    VStack(alignment:.leading,spacing:8) {
                        Text(routine.definition.name).font(.headline)
                        Text("\(routine.agentSnapshot.name) · \(routine.agentSnapshot.binding.selectedModel)").font(.caption)
                        if model.installation?.supports("schedules") == true { Button("Schedules") { scheduling = routine } }
                        HStack { Button("Edit") { editing = routine }; Button("Run now") { running = routine }.disabled(!routine.definition.isSupported) }.buttonStyle(.bordered)
                    }
                }
                if let error { Text(error).foregroundStyle(.red) }
                Text(model.status).font(.caption)
            }.navigationTitle("Routines")
            .toolbar { ToolbarItem(placement:.cancellationAction) { Button("Done") { dismiss() } } }
            .fileImporter(isPresented:$importing,allowedContentTypes:[.json]) { result in
                do {
                    let url = try result.get(), accessed = url.startAccessingSecurityScopedResource()
                    defer { if accessed { url.stopAccessingSecurityScopedResource() } }
                    let properties = try url.resourceValues(forKeys:[.isRegularFileKey,.isSymbolicLinkKey,.fileSizeKey])
                    guard properties.isRegularFile == true, properties.isSymbolicLink != true, (properties.fileSize ?? Int.max) <= 1_000_000 else { throw ClientError.invalidResponse }
                    let data = try Data(contentsOf:url); guard data.count <= 1_000_000 else { throw ClientError.invalidResponse }
                    let definition = try JSONDecoder().decode(RoutineDefinition.self,from:data)
                    guard definition.isSupported else { throw ClientError.service("unsupportedRoutine","This routine uses unsupported steps or input types.") }
                    imported = definition; creating = true; error = nil
                } catch { self.error = error.localizedDescription }
            }
            .sheet(isPresented:$creating) { RoutineEditor(model:model,imported:imported) }
            .sheet(item:$editing) { RoutineEditor(model:model,revision:$0) }
            .sheet(item:$running) { RoutineRunView(model:model,revision:$0) }
            .sheet(item:$scheduling) { ScheduleLibrary(model:model,routine:$0) }
        }
        #if os(macOS)
        .frame(minWidth:560,minHeight:550)
        #endif
    }
}

private struct RoutineEditor: View {
    @Bindable var model: MessengerModel
    @State private var form: RoutineForm
    @State private var exporting = false
    @Environment(\.dismiss) private var dismiss
    init(model: MessengerModel, revision: RoutineRevision? = nil, imported: RoutineDefinition? = nil) {
        self.model = model; _form = State(initialValue:RoutineForm(revision:revision,agent:model.recipient ?? model.agents.first,imported:imported))
    }
    var body: some View {
        NavigationStack {
            Form {
                TextField("Routine name",text:$form.definition.name)
                Section("Destination and authority") {
                    Text("Home: " + form.definition.conversationId).font(.caption)
                    Text("Binding: " + form.definition.bindingRevisionId).font(.caption).textSelection(.enabled)
                    if let agent = form.boundAgent ?? model.agents.first(where:{$0.binding.bindingRevisionId == form.definition.bindingRevisionId}) { Text("Pinned \(agent.name) · \(agent.binding.selectedModel)") }
                    Menu("Choose current Agent revision") { ForEach(model.agents.filter(\.binding.enabled)) { agent in Button("\(agent.name) · \(agent.binding.selectedModel)") { form.bind(agent) } } }
                    TextEditor(text:$form.definition.instructions).frame(minHeight:80).accessibilityLabel("Routine instructions")
                }
                Section("Typed inputs") {
                    ForEach(form.definition.inputs.indices,id:\.self) { index in
                        TextField("Input name",text:SwiftUI.Binding(get:{ form.definition.inputs[index].name },set:{ form.renameInput(at:index,to:$0) })).autocorrectionDisabled()
                        Picker("Type",selection:$form.definition.inputs[index].type) { Text("Text").tag("text"); Text("Number").tag("number"); Text("Boolean").tag("boolean") }
                        Toggle("Required",isOn:$form.definition.inputs[index].required)
                        Button("Remove input",role:.destructive) {
                            let name = form.definition.inputs.remove(at:index).name
                            for step in form.definition.steps.indices { form.definition.steps[step].inputNames?.removeAll { $0 == name } }
                        }
                    }
                    Button("Add input") { form.definition.inputs.append(RoutineInput(name:"input_\(form.definition.inputs.count+1)",type:"text",required:true)) }.disabled(form.definition.inputs.count >= 16)
                }
                Section("Reviewed skills") {
                    ForEach(form.definition.skills,id:\.skillId) { ref in
                        HStack { Text(model.skills.first(where:{$0.id == ref.skillId})?.name ?? ref.skillId); Text(ref.revisionId).font(.caption); Button("Remove") { form.definition.skills.removeAll { $0.skillId == ref.skillId } } }
                    }
                    Menu("Select enabled bundle") {
                        ForEach(model.skills.filter { skill in skill.canInvoke && skill.bundle != nil && !form.definition.skills.contains(where: { $0.skillId == skill.id }) }) { skill in
                            Button(skill.name) { if let bundle = skill.bundle { form.definition.skills.append(RoutineSkill(skillId:skill.id,revisionId:skill.revisionId,digest:bundle.digest)) } }
                        }
                    }
                }
                ForEach(form.definition.steps.indices,id:\.self) { index in stepEditor(index) }
                Section {
                    HStack {
                        Button("Add task") { form.definition.steps.append(RoutineStep(stepId:UUID().uuidString.lowercased(),name:"Task",type:"task",instructions:"",inputNames:[],outputStepIds:[])) }
                        Button("Add approval") { form.definition.steps.append(RoutineStep(stepId:UUID().uuidString.lowercased(),name:"Review",type:"approval",prompt:"Continue with the saved results?")) }
                    }.disabled(form.definition.steps.count >= 16)
                    Text("Saving does not run this routine or enable a schedule.").font(.caption)
                    Button("Export routine JSON") { exporting = true }
                    Text(model.status).font(.caption)
                }
            }.formStyle(.grouped).navigationTitle(form.source == nil ? "New routine" : "Edit routine")
            .toolbar {
                ToolbarItem(placement:.cancellationAction) { Button("Cancel") { dismiss() } }
                ToolbarItem(placement:.confirmationAction) { Button("Save") { Task { if await form.save(using:model) { dismiss() } } }.disabled(model.workspaceSaving || !form.definition.isSupported || form.definition.name.isEmpty || form.definition.agentId.isEmpty) }
            }
            .fileExporter(isPresented:$exporting,document:RoutineDocument(form.definition),contentType:.json,defaultFilename:"Fort-routine") { result in if case .failure(let error) = result { model.status = error.localizedDescription } }
        }
        #if os(macOS)
        .frame(minWidth:600,minHeight:650)
        #endif
    }
    @ViewBuilder private func stepEditor(_ index: Int) -> some View {
        Section("Step \(index+1) · \(form.definition.steps[index].type)") {
            TextField("Step name",text:$form.definition.steps[index].name)
            if form.definition.steps[index].type == "task" {
                TextEditor(text:SwiftUI.Binding(get:{ form.definition.steps[index].instructions ?? "" },set:{ form.definition.steps[index].instructions = $0 })).frame(minHeight:80).accessibilityLabel("Task instructions")
                ForEach(form.definition.inputs,id:\.name) { input in referenceToggle(input.name,index:index,output:false,label:"Read input: " + input.name) }
                ForEach(form.definition.steps.prefix(index).filter { $0.type == "task" }) { step in referenceToggle(step.id,index:index,output:true,label:"Read output: " + step.name) }
            } else { TextField("Approval question",text:SwiftUI.Binding(get:{ form.definition.steps[index].prompt ?? "" },set:{ form.definition.steps[index].prompt = $0 })) }
            HStack {
                Button("Move up") { form.moveStep(form.definition.steps[index].id,by:-1) }.disabled(index == 0)
                Button("Move down") { form.moveStep(form.definition.steps[index].id,by:1) }.disabled(index == form.definition.steps.count-1)
                Button("Remove step",role:.destructive) { form.removeStep(form.definition.steps[index].id) }
            }
        }
    }
    private func referenceToggle(_ id: String, index: Int, output: Bool, label: String) -> some View {
        Toggle(label,isOn:SwiftUI.Binding(get:{ (output ? form.definition.steps[index].outputStepIds : form.definition.steps[index].inputNames)?.contains(id) == true },set:{ enabled in
            var values = (output ? form.definition.steps[index].outputStepIds : form.definition.steps[index].inputNames) ?? []
            values.removeAll { $0 == id }; if enabled { values.append(id) }
            if output { form.definition.steps[index].outputStepIds = values } else { form.definition.steps[index].inputNames = values }
        }))
    }
}

private struct RoutineRunView: View {
    @Bindable var model: MessengerModel
    let revision: RoutineRevision
    @State private var values: [String:String] = [:]
    @State private var booleans: [String:Bool] = [:]
    @State private var error: String?
    @Environment(\.dismiss) private var dismiss
    var body: some View {
        NavigationStack {
            Form {
                Text(revision.definition.name).font(.headline)
                Text("\(revision.agentSnapshot.name) · \(revision.agentSnapshot.binding.selectedModel)")
                Text("Results go to this Agent’s Home.")
                Text("Skills: " + (revision.skills.isEmpty ? "None" : revision.skills.map { $0.name + " (" + $0.revisionId + ")" }.joined(separator:", "))).font(.caption)
                DisclosureGroup("Routine instructions") { Text(revision.definition.instructions).textSelection(.enabled) }
                Text("Revision: " + revision.revisionId).font(.caption).textSelection(.enabled)
                ForEach(revision.definition.inputs,id:\.name) { input in
                    if input.type == "boolean" { Toggle(input.name,isOn:SwiftUI.Binding(get:{ booleans[input.name] ?? false },set:{ booleans[input.name] = $0 })) }
                    else { TextField(input.name + (input.required ? " (required)" : " (optional)"),text:SwiftUI.Binding(get:{ values[input.name] ?? "" },set:{ values[input.name] = $0 })) }
                }
                ForEach(revision.definition.steps) { step in Text("\(step.name) · \(step.type)") }
                if let error { Text(error).foregroundStyle(.red) }
                Text(model.status).font(.caption)
            }.navigationTitle("Run routine")
            .toolbar {
                ToolbarItem(placement:.cancellationAction) { Button("Cancel") { dismiss() } }
                ToolbarItem(placement:.confirmationAction) { Button("Run now") { Task { await run() } }.disabled(model.isBusy || !revision.definition.isSupported) }
            }
        }
        #if os(macOS)
        .frame(minWidth:500,minHeight:450)
        #endif
    }
    private func run() async {
        var inputs: [String:RoutineValue] = [:]
        for input in revision.definition.inputs {
            if input.type == "boolean" { inputs[input.name] = .boolean(booleans[input.name] ?? false); continue }
            let value = values[input.name] ?? ""
            if value.isEmpty && !input.required { continue }
            if input.type == "number" { guard let number = Double(value), number.isFinite else { error = "Enter a finite number for " + input.name; return }; inputs[input.name] = .number(number) }
            else { guard !value.isEmpty || !input.required else { error = "Enter " + input.name; return }; inputs[input.name] = .text(value) }
        }
        if await model.runRoutine(revision,inputs:inputs) { dismiss() }
    }
}
