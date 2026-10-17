import SwiftUI
import UniformTypeIdentifiers

private func readBundleFile(_ url: URL) throws -> Data {
    let accessed = url.startAccessingSecurityScopedResource()
    defer { if accessed { url.stopAccessingSecurityScopedResource() } }
    let values = try url.resourceValues(forKeys:[.isRegularFileKey,.isSymbolicLinkKey,.fileSizeKey])
    guard values.isRegularFile == true, values.isSymbolicLink != true, let size = values.fileSize, size <= 1_000_000 else {
        throw ClientError.service("invalidSkillBundle","Choose a regular Fort bundle JSON file within 1 MB.")
    }
    let data = try Data(contentsOf:url)
    guard data.count <= 1_000_000 else { throw ClientError.service("invalidSkillBundle","The bundle file exceeds 1 MB.") }
    return data
}

private struct BundleDocument: FileDocument {
    static var readableContentTypes: [UTType] { [.json] }
    var data: Data
    init(data: Data) { self.data = data }
    init(configuration: ReadConfiguration) throws {
        guard let data = configuration.file.regularFileContents else { throw ClientError.invalidResponse }
        self.data = data
    }
    func fileWrapper(configuration: WriteConfiguration) throws -> FileWrapper { FileWrapper(regularFileWithContents:data) }
}

struct SkillBundleLibrary: View {
    @Bindable var model: MessengerModel
    @Environment(\.dismiss) private var dismiss
    @State private var importing = false
    @State private var selected: InstructionSkill?
    @State private var errorText: String?
    var body: some View {
        NavigationStack {
            List {
                Section {
                    Text("Import a complete Fort bundle JSON file to review its source, files and requested tools. Importing executes nothing.")
                    Button("Import new bundle") { importing = true }.disabled(model.workspaceSaving || !model.supportsCommands)
                }
                Section("Saved bundles") {
                    ForEach(model.skills.filter { $0.bundle != nil }) { skill in
                        Button { selected = skill } label: {
                            VStack(alignment:.leading) {
                                Text(skill.name)
                                Text(skill.canInvoke ? "Enabled revision" : "Review or enable revision").font(.caption).foregroundStyle(.secondary)
                            }
                        }
                    }
                }
                if let errorText { Text(errorText).foregroundStyle(.red) }
                Text(model.status).font(.caption).foregroundStyle(.secondary)
            }
            .navigationTitle("Skill bundles")
            .toolbar { ToolbarItem(placement:.cancellationAction) { Button("Done") { dismiss() } } }
            .fileImporter(isPresented:$importing,allowedContentTypes:[.json]) { result in
                Task {
                    do { let data = try readBundleFile(result.get()); errorText = nil; selected = await model.importSkillBundle(data) }
                    catch { errorText = error.localizedDescription }
                }
            }
            .sheet(item:$selected) { SkillBundleReviewView(model:model,skill:$0) }
        }
        #if os(macOS)
        .frame(minWidth:540,minHeight:500)
        #endif
    }
}

struct SkillBundleReviewView: View {
    @Bindable var model: MessengerModel
    @Environment(\.dismiss) private var dismiss
    @State private var skill: InstructionSkill
    @State private var previous: InstructionSkill?
    @State private var comparing = true
    @State private var comparisonError: String?
    @State private var errorText: String?
    @State private var selectedFile: SkillBundleFile?
    @State private var importing = false
    @State private var exporting = false
    @State private var exportDocument: BundleDocument?
    init(model: MessengerModel, skill: InstructionSkill) { self.model = model; _skill = State(initialValue:skill) }
    var body: some View {
        NavigationStack {
            if let bundle = skill.bundle {
                Form {
                    Section("Revision") {
                        LabeledContent("Name",value:bundle.name)
                        LabeledContent("Version",value:bundle.version)
                        Text("Source: " + bundle.source).textSelection(.enabled)
                        Text("License: " + bundle.license).textSelection(.enabled)
                        Text("Digest: " + bundle.digest).font(.caption).textSelection(.enabled)
                        Text(skill.canInvoke ? "This exact revision is enabled." : "This revision is not enabled.")
                        if let previous { Text("Previous version: " + (previous.bundle?.version ?? "Instruction skill")) }
                    }
                    if let previous, previous.bundle == nil {
                        Section("Previous instructions") { Text(previous.instructions).textSelection(.enabled) }
                    }
                    Section("Files") {
                        ForEach(bundle.files) { file in
                            Button { selectedFile = file } label: {
                                VStack(alignment:.leading) {
                                    Text(file.path)
                                    Text("\(file.bytes) bytes" + (file.executable ? " · executable declared" : "")).font(.caption)
                                }
                            }
                        }
                    }
                    Section("Changes to review") {
                        if comparing { ProgressView("Loading previous revision") }
                        else if let comparisonError { Text(comparisonError).foregroundStyle(.red) }
                        else {
                            ForEach(bundle.changes(from:previous?.bundle)) { change in Text(change.change + ": " + change.path) }
                            if bundle.changes(from:previous?.bundle).isEmpty { Text("No file changes.") }
                        }
                    }
                    Section("Requested authority") {
                        Text("Runtimes: " + bundle.supportedRuntimes.joined(separator:", "))
                        Text("Tools: " + (bundle.requestedTools.isEmpty ? "None" : bundle.requestedTools.joined(separator:", ")))
                        if let prior = previous?.bundle {
                            Text("Previous tools: " + (prior.requestedTools.isEmpty ? "None" : prior.requestedTools.joined(separator:", "))).font(.caption)
                            Text("Previous runtimes: " + prior.supportedRuntimes.joined(separator:", ")).font(.caption)
                        }
                        ForEach(bundle.scripts,id:\.path) { script in
                            Text(script.path + " · interpreter: " + script.interpreter)
                            Text("Resources: " + (script.resourceScope.isEmpty ? "None declared" : script.resourceScope.joined(separator:", "))).font(.caption)
                        }
                        if let prior = previous?.bundle, prior.scripts != bundle.scripts { Text("Script declarations changed from the previous revision.") }
                        Text("Enablement records your review. The runtime still enforces its own permissions.").font(.caption)
                        if !bundle.supportsCurrentChat { Text("The current no-tool Codex chat supports UTF-8 text bundles declared for Codex. This bundle has binary content, scripts, requested tools or another runtime.").foregroundStyle(.secondary) }
                    }
                    Section {
                        Button(skill.canInvoke ? "Disable this revision" : "Enable reviewed revision") {
                            Task { if let updated = await model.reviewSkill(skill,enable:!skill.canInvoke) { skill = updated } }
                        }.disabled(comparing || comparisonError != nil || model.workspaceSaving || !model.supportsCommands || bundle.schemaVersion != 1)
                        Button("Import revised bundle") { importing = true }.disabled(model.workspaceSaving || !model.supportsCommands)
                        Button("Export complete bundle") {
                            do {
                                let encoder = JSONEncoder(); encoder.outputFormatting = [.prettyPrinted,.sortedKeys]
                                exportDocument = BundleDocument(data:try encoder.encode(bundle)); exporting = true
                            } catch { errorText = error.localizedDescription }
                        }
                        if let errorText { Text(errorText).foregroundStyle(.red) }
                        Text(model.status).font(.caption).foregroundStyle(.secondary)
                    }
                }.formStyle(.grouped)
                .navigationTitle("Review skill bundle")
                .toolbar { ToolbarItem(placement:.cancellationAction) { Button("Done") { dismiss() } } }
                .task(id:skill.revisionId) {
                    comparing = true; comparisonError = nil
                    do { previous = try await model.previousRevision(for:skill) }
                    catch { comparisonError = error.localizedDescription }
                    comparing = false
                }
                .fileImporter(isPresented:$importing,allowedContentTypes:[.json]) { result in
                    Task {
                        do { let data = try readBundleFile(result.get()); errorText = nil; if let imported = await model.importSkillBundle(data,replacing:skill) { skill = imported } }
                        catch { errorText = error.localizedDescription }
                    }
                }
                .fileExporter(isPresented:$exporting,document:exportDocument,contentType:.json,defaultFilename:"Fort-skill-bundle") { result in
                    if case .failure(let error) = result { errorText = error.localizedDescription }
                }
                .sheet(item:$selectedFile) { file in
                    NavigationStack {
                        ScrollView {
                            VStack(alignment:.leading,spacing:12) {
                                Text("SHA-256: " + file.sha256).font(.caption)
                                Text(file.text ?? "Binary asset · \(file.bytes) bytes. Export the complete bundle to inspect this asset with your chosen tool.")
                            }.textSelection(.enabled).padding()
                        }.navigationTitle(file.path)
                        .toolbar { ToolbarItem(placement:.cancellationAction) { Button("Done") { selectedFile = nil } } }
                    }
                    #if os(macOS)
                    .frame(minWidth:500,minHeight:420)
                    #endif
                }
            }
        }
        #if os(macOS)
        .frame(minWidth:560,minHeight:560)
        #endif
    }
}
