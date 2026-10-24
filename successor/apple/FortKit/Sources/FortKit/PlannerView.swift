import SwiftUI

/// Sunday planning in four steps: look back, pick goals, lay out the blueprint, found the city.
struct PlannerView: View {
    @Bindable var model: MessengerModel
    @State private var draft: PlanDraft
    @State private var step = 0
    @State private var founding = false
    @State private var newGoal = ""
    @State private var newTasks: [String:String] = [:]
    @Environment(\.dismiss) private var dismiss
    private let previous: WeekView?
    static let steps = ["Debrief","Goals","Blueprint","Found"]
    static let cityNames = ["Glass Bay","Harbor Point","Cedar Flats","Old Mill","Copper Ridge","Lantern Row","North Quay","Saltmarsh"]

    init(model: MessengerModel, today: String = CityHUDView.today()) {
        self.model = model
        let previous = model.weeks.first
        self.previous = previous
        let name = Self.cityNames[(previous?.number ?? 0) % Self.cityNames.count]
        _draft = State(initialValue:PlanDraft(previous:previous,today:today,name:name))
    }

    var body: some View {
        VStack(spacing:0) {
            header
            Divider()
            ScrollView {
                VStack(alignment:.leading,spacing:16) {
                    switch step {
                    case 0: debrief
                    case 1: goalPicker
                    case 2: blueprint
                    default: found
                    }
                }.padding(20).frame(maxWidth:.infinity,alignment:.leading)
            }
            Divider()
            footer
        }
        .frame(minWidth:640,minHeight:560)
        .environment(\.colorScheme,.dark)
        .background(HUDStyle.panel)
    }

    private var header: some View {
        HStack {
            VStack(alignment:.leading,spacing:2) {
                Text("SUNDAY PLANNING").font(.caption2.weight(.heavy)).foregroundStyle(HUDStyle.gold)
                Text("Plan Week \((model.weeks.first?.number ?? 0) + 1)").font(.system(.title,design:.rounded).weight(.black))
            }
            Spacer()
            ForEach(Array(Self.steps.enumerated()),id:\.offset) { index, title in
                Button { step = index } label: {
                    Text(index < step ? "✓ \(title)" : "\(index + 1) · \(title)").font(.caption.weight(.heavy))
                        .padding(.horizontal,10).padding(.vertical,6)
                        .background(index == step ? HUDStyle.gold : .clear,in:RoundedRectangle(cornerRadius:6))
                        .foregroundStyle(index == step ? Color.black : .primary)
                }.buttonStyle(.plain).disabled(index > step).accessibilityIdentifier("planner-step-\(index)")
            }
            Button("Close",systemImage:"xmark") { dismiss() }.labelStyle(.iconOnly).keyboardShortcut(.cancelAction)
        }.padding(16)
    }

    @ViewBuilder private var debrief: some View {
        if let previous {
            Text("Week \(previous.number) · \(previous.name)").font(.title2.weight(.heavy))
            Text("\(previous.progress.points) points · \(previous.progress.goalsMet) of \(previous.progress.goalsCounted) crew goals met\(previous.progress.grade.map { " · grade \($0)" } ?? "")").foregroundStyle(.secondary)
            ForEach(previous.goals) { goal in
                let progress = previous.progress.goals.first { $0.goalId == goal.goalId }
                HStack {
                    Image(systemName:progress?.met == true ? "checkmark.seal.fill" : "circle.dashed").foregroundStyle(progress?.met == true ? .green : .secondary)
                    Text(goal.text)
                    Spacer()
                    Text("\(progress?.points ?? 0)/\(goal.targetPoints)").monospacedDigit().foregroundStyle(.secondary)
                }
            }
            let carried = draft.tasks.filter(\.carried).count
            Text(carried == 0 ? "Nothing carries over." : "\(carried) unfinished \(carried == 1 ? "task carries" : "tasks carry") over into the next step. Drop any you no longer want.").font(.callout).foregroundStyle(.secondary)
        } else {
            Text("Your first week").font(.title2.weight(.heavy))
            Text("Pick up to \(PlanDraft.maximumGoals) goals, give each one tasks for the crew or for you, then found the city. Each finished task becomes a building.").foregroundStyle(.secondary)
        }
    }

    @ViewBuilder private var goalPicker: some View {
        Text("Pick up to \(PlanDraft.maximumGoals) goals. The starred one is the main quest and raises the landmark.").foregroundStyle(.secondary)
        ForEach($draft.goals) { $goal in
            HStack(spacing:10) {
                Button { draft.mainGoalID = goal.id } label: { Image(systemName:draft.mainGoalID == goal.id ? "star.fill" : "star") }.buttonStyle(.plain).foregroundStyle(HUDStyle.gold).accessibilityLabel("Main quest")
                TextField("Goal",text:$goal.text).textFieldStyle(.roundedBorder)
                Picker("Zone",selection:$goal.zone) { ForEach(WeekZone.allCases,id:\.self) { Text(zoneName($0)).tag($0) } }.labelsHidden().frame(width:110)
                Picker("Lead",selection:$goal.lead) { ownerOptions }.labelsHidden().frame(width:120)
                if goal.carriedFrom != nil { Text("CARRIED").font(.caption2.weight(.heavy)).foregroundStyle(.secondary) }
                Button("Remove",systemImage:"trash") { draft.removeGoal(goal.id) }.labelStyle(.iconOnly).buttonStyle(.borderless)
            }
        }
        if draft.goals.count < PlanDraft.maximumGoals {
            HStack {
                TextField("Add a goal, like “Ship the beta”",text:$newGoal).textFieldStyle(.roundedBorder).onSubmit(addGoal).accessibilityIdentifier("planner-new-goal")
                Button("Add",action:addGoal).disabled(newGoal.trimmingCharacters(in:.whitespaces).isEmpty)
            }
        }
    }

    @ViewBuilder private var blueprint: some View {
        Text("Give each goal its tasks. Size sets the building: small 1, moderate 3, significant 6, milestone 12 points.").foregroundStyle(.secondary)
        ForEach(draft.goals) { goal in
            VStack(alignment:.leading,spacing:8) {
                HStack {
                    RoundedRectangle(cornerRadius:2).fill(HUDStyle.zone(goal.zone)).frame(width:10,height:10)
                    Text(goal.text).font(.headline)
                    if draft.mainGoalID == goal.id { Image(systemName:"star.fill").foregroundStyle(HUDStyle.gold) }
                    Spacer()
                    Text("\(draft.target(for:goal.id)) pts").font(.caption.monospacedDigit()).foregroundStyle(.secondary)
                }
                ForEach($draft.tasks.filter { $0.wrappedValue.goalID == goal.id }) { $task in
                    HStack(spacing:8) {
                        TextField("Task",text:$task.text).textFieldStyle(.roundedBorder)
                        Picker("Size",selection:$task.size) { ForEach(WeekSize.allCases,id:\.self) { Text("\($0.rawValue.capitalized) \($0.points)").tag($0) } }.labelsHidden().frame(width:140)
                        Picker("Owner",selection:$task.owner) { ownerOptions }.labelsHidden().frame(width:120)
                        Button("Remove",systemImage:"xmark") { draft.removeTask(task.id) }.labelStyle(.iconOnly).buttonStyle(.borderless)
                    }
                }
                HStack {
                    TextField("Add a task",text:SwiftUI.Binding(get:{ newTasks[goal.id] ?? "" },set:{ newTasks[goal.id] = $0 })).textFieldStyle(.roundedBorder).onSubmit { addTask(goal) }
                    Button("Add") { addTask(goal) }.disabled((newTasks[goal.id] ?? "").trimmingCharacters(in:.whitespaces).isEmpty)
                }
            }.padding(12).background(.black.opacity(0.25),in:RoundedRectangle(cornerRadius:8))
        }
        if draft.goals.isEmpty { Text("Go back and pick a goal first.").foregroundStyle(.secondary) }
    }

    @ViewBuilder private var found: some View {
        TextField("City name",text:$draft.name).textFieldStyle(.roundedBorder).font(.title3).accessibilityIdentifier("planner-city-name")
        Text("Starts \(draft.startsOn) · \(draft.goals.count) \(draft.goals.count == 1 ? "goal" : "goals") · \(draft.tasks.count) \(draft.tasks.count == 1 ? "task" : "tasks") on the blueprint").foregroundStyle(.secondary)
        if let live = model.liveWeek { Text("Founding closes Week \(live.number) · \(live.name). Its open tasks are already carried over here.").font(.callout).foregroundStyle(.orange) }
        ForEach(draft.problems,id:\.self) { Label($0,systemImage:"exclamationmark.triangle").foregroundStyle(.orange) }
    }

    private var footer: some View {
        HStack(alignment:.center,spacing:14) {
            Text("CAPACITY").font(.caption2.weight(.heavy)).foregroundStyle(HUDStyle.gold)
            ForEach(draft.loads(agents:model.agents)) { load in
                VStack(alignment:.leading,spacing:2) {
                    Text("\(ownerName(load.owner)) \(load.points)/\(load.capacity)").font(.caption2.monospacedDigit()).foregroundStyle(load.overbooked ? .red : .secondary)
                    ProgressView(value:min(1,Double(load.points)/Double(max(1,load.capacity)))).tint(load.overbooked ? .red : .green).frame(width:70)
                }
            }
            Spacer()
            if step > 0 { Button("Back") { step -= 1 } }
            if step < 3 { Button("\(Self.steps[step + 1]) ▸") { step += 1 }.buttonStyle(.borderedProminent).keyboardShortcut(.defaultAction).accessibilityIdentifier("planner-next") }
            else {
                Button(founding ? "Founding…" : "Found \(draft.name) ▸") {
                    founding = true
                    Task { if await model.foundWeek(draft) { dismiss() }; founding = false }
                }.buttonStyle(.borderedProminent).disabled(!draft.canFound || founding).keyboardShortcut(.defaultAction).accessibilityIdentifier("planner-found")
            }
        }.padding(16)
    }

    @ViewBuilder private var ownerOptions: some View {
        Text("You").tag(WeekOwner.you)
        ForEach(model.agents) { Text($0.name).tag(WeekOwner.agent($0.agentId)) }
    }
    private func ownerName(_ owner: WeekOwner) -> String {
        switch owner { case .you: "You"; case .agent(let id): model.agents.first { $0.agentId == id }?.name ?? "Former agent" }
    }
    private func zoneName(_ zone: WeekZone) -> String { switch zone { case .code: "Code"; case .money: "Money"; case .writing: "Writing"; case .personal: "Your own" } }
    private func addGoal() {
        let text = newGoal.trimmingCharacters(in:.whitespaces)
        guard !text.isEmpty else { return }
        draft.addGoal(text:text,zone:.code,lead:model.agents.first.map { .agent($0.agentId) } ?? .you); newGoal = ""
    }
    private func addTask(_ goal: PlanDraft.Goal) {
        let text = (newTasks[goal.id] ?? "").trimmingCharacters(in:.whitespaces)
        guard !text.isEmpty else { return }
        draft.addTask(goalID:goal.id,text:text,owner:goal.lead,size:.moderate); newTasks[goal.id] = ""
    }
}
