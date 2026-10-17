import SwiftUI
import UniformTypeIdentifiers

private struct ScheduleDocument: FileDocument {
    static var readableContentTypes: [UTType] { [.json] }
    var data: Data
    init(_ definition: ScheduleDefinition) { let encoder = JSONEncoder(); encoder.outputFormatting = [.prettyPrinted,.sortedKeys]; data = (try? encoder.encode(definition)) ?? Data() }
    init(configuration: ReadConfiguration) throws { guard let data = configuration.file.regularFileContents else { throw ClientError.invalidResponse }; self.data = data }
    func fileWrapper(configuration: WriteConfiguration) throws -> FileWrapper { FileWrapper(regularFileWithContents:data) }
}

struct ScheduleLibrary: View {
    @Bindable var model: MessengerModel
    let routine: RoutineRevision
    @Environment(\.dismiss) private var dismiss
    @State private var creating = false
    @State private var importing = false
    @State private var imported: ScheduleDefinition?
    @State private var selected: ScheduleRevision?
    @State private var editing: ScheduleRevision?
    @State private var error: String?
    var body: some View {
        NavigationStack {
            List {
                Section {
                    Text(routine.definition.name).font(.headline)
                    Text("The service must be running at the scheduled time. A sleeping or offline host may miss occurrences.")
                    Button("Create paused schedule") { imported = nil; creating = true }
                    Button("Import schedule JSON") { importing = true }
                }
                ForEach(model.schedules.filter { $0.definition.routineId == routine.id }) { schedule in
                    VStack(alignment:.leading,spacing:8) {
                        Text("\(schedule.definition.kind.capitalized) · \(schedule.definition.time) · \(schedule.definition.timezone)")
                        Text(schedule.state).font(.caption)
                        HStack { Button("Review and manage") { selected = schedule }; Button("Edit") { editing = schedule } }.buttonStyle(.bordered)
                    }
                }
                if let error { Text(error).foregroundStyle(.red) }
                Text(model.status).font(.caption)
            }.navigationTitle("Schedules")
            .toolbar { ToolbarItem(placement:.cancellationAction) { Button("Done") { dismiss() } } }
            .sheet(isPresented:$creating) { ScheduleEditor(model:model,routine:routine,imported:imported) }
            .sheet(item:$editing) { ScheduleEditor(model:model,routine:routine,revision:$0) }
            .sheet(item:$selected) { ScheduleReview(model:model,revision:$0) }
            .fileImporter(isPresented:$importing,allowedContentTypes:[.json]) { result in
                do {
                    let url = try result.get(), accessed = url.startAccessingSecurityScopedResource()
                    defer { if accessed { url.stopAccessingSecurityScopedResource() } }
                    let metadata = try url.resourceValues(forKeys:[.isRegularFileKey,.isSymbolicLinkKey,.fileSizeKey])
                    guard metadata.isRegularFile == true, metadata.isSymbolicLink != true, (metadata.fileSize ?? Int.max) <= 1_000_000 else { throw ClientError.invalidResponse }
                    let data = try Data(contentsOf:url); guard data.count <= 1_000_000 else { throw ClientError.invalidResponse }
                    let definition = try JSONDecoder().decode(ScheduleDefinition.self,from:data)
                    guard definition.isSupported, definition.routineId == routine.id else { throw ClientError.service("scheduleMismatch","Select a supported schedule for this routine.") }
                    imported = definition; creating = true; error = nil
                } catch { self.error = error.localizedDescription }
            }
        }
        #if os(macOS)
        .frame(minWidth:550,minHeight:500)
        #endif
    }
}

private struct ScheduleEditor: View {
    @Bindable var model: MessengerModel
    let routine: RoutineRevision
    let revision: ScheduleRevision?
    @State private var definition: ScheduleDefinition
    @State private var textInputs: [String:String] = [:]
    @State private var boolInputs: [String:Bool] = [:]
    @State private var exporting = false
    @State private var error: String?
    @Environment(\.dismiss) private var dismiss
    init(model: MessengerModel, routine: RoutineRevision, revision: ScheduleRevision? = nil, imported: ScheduleDefinition? = nil) {
        self.model = model; self.routine = routine; self.revision = revision
        let definition = imported ?? revision?.definition ?? ScheduleDefinition(schemaVersion:1,routineId:routine.id,routineRevisionId:routine.revisionId,timezone:TimeZone.current.identifier,time:"09:00",inputs:[:],missed:"skip",kind:"daily")
        _definition = State(initialValue:definition)
        var texts: [String:String] = [:], bools: [String:Bool] = [:]
        for (key,value) in definition.inputs { switch value { case .text(let text): texts[key] = text; case .number(let number): texts[key] = String(number); case .boolean(let bool): bools[key] = bool } }
        _textInputs = State(initialValue:texts); _boolInputs = State(initialValue:bools)
    }
    var body: some View {
        NavigationStack {
            Form {
                Text("Routine revision: " + definition.routineRevisionId).font(.caption).textSelection(.enabled)
                if definition.routineRevisionId != routine.revisionId {
                    Text("This schedule pins an earlier routine. Existing inputs and authority stay pinned.")
                    Button("Use the reviewed routine shown here") { definition.routineRevisionId = routine.revisionId; definition.inputs = [:]; textInputs = [:]; boolInputs = [:] }
                }
                Picker("Repeat",selection:$definition.kind) { Text("One time").tag("once"); Text("Daily").tag("daily"); Text("Weekly").tag("weekly") }
                Picker("Timezone",selection:$definition.timezone) { ForEach(Array(Set(TimeZone.knownTimeZoneIdentifiers + ["UTC"])).sorted(),id:\.self) { Text($0).tag($0) } }
                DatePicker("Local time",selection:wallDate(time:true),displayedComponents:.hourAndMinute).environment(\.timeZone,TimeZone(secondsFromGMT:0)!)
                if definition.kind == "once" { DatePicker("Local date",selection:wallDate(time:false),displayedComponents:.date).environment(\.timeZone,TimeZone(secondsFromGMT:0)!) }
                if definition.kind == "weekly" {
                    ForEach(1...7,id:\.self) { day in
                        Toggle(["Monday","Tuesday","Wednesday","Thursday","Friday","Saturday","Sunday"][day-1],isOn:SwiftUI.Binding(get:{ definition.weekdays?.contains(day) == true },set:{ selected in var days = definition.weekdays ?? []; days.removeAll { $0 == day }; if selected { days.append(day) }; definition.weekdays = days.sorted() }))
                    }
                }
                Picker("After downtime",selection:$definition.missed) { Text("Skip missed occurrences").tag("skip"); Text("Run one latest catch-up").tag("catchUpOne") }
                Text("Only one active run per routine. Overlapping occurrences are skipped, including while a run waits for approval. Missing daylight-saving times are skipped; repeated times run once at the first occurrence.").font(.caption)
                if definition.routineRevisionId == routine.revisionId {
                    Section("Saved inputs") {
                        ForEach(routine.definition.inputs,id:\.name) { input in
                            if input.type == "boolean" { Toggle(input.name,isOn:SwiftUI.Binding(get:{ boolInputs[input.name] ?? false },set:{ boolInputs[input.name] = $0 })) }
                            else { TextField(input.name + (input.required ? " (required)" : " (optional)"),text:SwiftUI.Binding(get:{ textInputs[input.name] ?? "" },set:{ textInputs[input.name] = $0 })) }
                        }
                    }
                }
                Text("Save creates a paused revision. Review its next occurrences before enabling it.").font(.caption)
                Button("Export schedule JSON") { if prepare() { exporting = true } }
                if let error { Text(error).foregroundStyle(.red) }
                Text(model.status).font(.caption)
            }.formStyle(.grouped).navigationTitle("Schedule")
            .toolbar {
                ToolbarItem(placement:.cancellationAction) { Button("Cancel") { dismiss() } }
                ToolbarItem(placement:.confirmationAction) { Button("Save paused") { Task { if prepare(), await model.saveSchedule(revision,definition:definition) { dismiss() } } }.disabled(model.workspaceSaving || !definition.isSupported) }
            }
            .onChange(of:definition.kind) { _,kind in
                if kind == "once", definition.date == nil { let formatter = DateFormatter(); formatter.dateFormat = "yyyy-MM-dd"; formatter.timeZone = TimeZone(identifier:definition.timezone); definition.date = formatter.string(from:Date()) }
            }
            .fileExporter(isPresented:$exporting,document:ScheduleDocument(definition),contentType:.json,defaultFilename:"Fort-schedule") { result in if case .failure(let error) = result { self.error = error.localizedDescription } }
        }
        #if os(macOS)
        .frame(minWidth:560,minHeight:600)
        #endif
    }
    private func wallDate(time: Bool) -> SwiftUI.Binding<Date> {
        let formatter = DateFormatter(); formatter.locale = Locale(identifier:"en_US_POSIX"); formatter.timeZone = TimeZone(secondsFromGMT:0); formatter.dateFormat = time ? "HH:mm" : "yyyy-MM-dd"
        return SwiftUI.Binding(get:{ formatter.date(from:time ? definition.time : definition.date ?? formatter.string(from:Date())) ?? Date() },set:{ if time { definition.time = formatter.string(from:$0) } else { definition.date = formatter.string(from:$0) } })
    }
    private func prepare() -> Bool {
        if definition.kind != "once" { definition.date = nil }
        else if definition.date == nil { let formatter = DateFormatter(); formatter.dateFormat = "yyyy-MM-dd"; formatter.timeZone = TimeZone(identifier:definition.timezone); definition.date = formatter.string(from:Date()) }
        if definition.kind != "weekly" { definition.weekdays = nil }
        if definition.routineRevisionId == routine.revisionId {
            var inputs: [String:RoutineValue] = [:]
            for input in routine.definition.inputs {
                if input.type == "boolean" { inputs[input.name] = .boolean(boolInputs[input.name] ?? false); continue }
                let value = textInputs[input.name] ?? ""
                if value.isEmpty && !input.required { continue }
                if input.type == "number" { guard let number = Double(value), number.isFinite else { error = "Enter a finite number for " + input.name; return false }; inputs[input.name] = .number(number) }
                else { guard !value.isEmpty || !input.required else { error = "Enter " + input.name; return false }; inputs[input.name] = .text(value) }
            }
            definition.inputs = inputs
        }
        error = nil; return true
    }
}

private struct ScheduleReview: View {
    @Bindable var model: MessengerModel
    let revision: ScheduleRevision
    @State private var pinnedRoutine: RoutineRevision?
    @State private var preview: SchedulePreview?
    @State private var occurrences: [ScheduleOccurrence] = []
    @State private var error: String?
    @Environment(\.dismiss) private var dismiss
    var body: some View {
        NavigationStack {
            List {
                Text("\(revision.definition.kind.capitalized) at \(revision.definition.time) · \(revision.definition.timezone)")
                Text("Routine revision: " + revision.definition.routineRevisionId).font(.caption)
                if let pinnedRoutine {
                    Text("\(pinnedRoutine.agentSnapshot.name) · \(pinnedRoutine.agentSnapshot.binding.selectedModel)")
                    Text("Home: " + pinnedRoutine.definition.conversationId).font(.caption)
                    Text("Skills: " + (pinnedRoutine.skills.isEmpty ? "None" : pinnedRoutine.skills.map { $0.name + " (" + $0.revisionId + ")" }.joined(separator:", "))).font(.caption)
                    ForEach(revision.definition.inputs.keys.sorted(),id:\.self) { key in Text(key + ": " + (revision.definition.inputs[key]?.displayText ?? "")) }
                }
                Text("Overlaps: skip. After downtime: " + (revision.definition.missed == "skip" ? "skip" : "one latest catch-up"))
                if let preview {
                    Section("Next occurrences") {
                        ForEach(preview.occurrences.indices,id:\.self) { index in
                            let occurrence = preview.occurrences[index]
                            Text("\(occurrence.localDate) \(occurrence.localTime) · " + (occurrence.intendedAt ?? "Skipped: this local time does not exist"))
                        }
                    }
                    if revision.state == "paused" {
                        Button("Enable this reviewed schedule") { Task { if await model.setSchedule(revision,enabled:true,preview:preview) { dismiss() } } }.disabled(model.workspaceSaving || preview.occurrences.isEmpty || preview.revisionId != revision.revisionId || !revision.definition.isSupported || pinnedRoutine?.definition.isSupported != true)
                    }
                }
                if revision.state == "enabled" {
                    Button("Pause future occurrences") { Task { if await model.setSchedule(revision,enabled:false,preview:nil) { dismiss() } } }.disabled(model.workspaceSaving)
                    Text("Pausing preserves history and does not stop work already accepted.").font(.caption)
                }
                Section("Occurrence history") { ForEach(occurrences) { occurrence in VStack(alignment:.leading) { Text("\(occurrence.localDate) \(occurrence.localTime) · \(occurrence.displayState)"); if let detail = occurrence.detail { Text(detail).font(.caption) } } } }
                if let error { Text(error).foregroundStyle(.red) }
                Text(model.status).font(.caption)
            }.navigationTitle("Review schedule")
            .toolbar { ToolbarItem(placement:.cancellationAction) { Button("Done") { dismiss() } } }
            .task { do { pinnedRoutine = try await model.routineRevision(routineID:revision.definition.routineId,revisionID:revision.definition.routineRevisionId); preview = try await model.schedulePreview(revision); occurrences = try await model.scheduleOccurrences(revision) } catch { self.error = error.localizedDescription } }
        }
        #if os(macOS)
        .frame(minWidth:550,minHeight:500)
        #endif
    }
}
