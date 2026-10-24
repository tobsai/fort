import Foundation

/// Sunday planning, held on the device until you found the week. Nothing here reaches the core until `request` is sent.
public struct PlanDraft: Equatable, Sendable {
    public struct Goal: Identifiable, Equatable, Sendable {
        public var id: String
        public var text: String
        public var zone: WeekZone
        public var lead: WeekOwner
        public var carriedFrom: String?
    }
    public struct PlannedTask: Identifiable, Equatable, Sendable {
        public var id: String
        public var goalID: String
        public var text: String
        public var owner: WeekOwner
        public var size: WeekSize
        public var unlocks: String?
        public var carried: Bool
    }
    public struct Load: Identifiable, Equatable, Sendable {
        public var id: WeekOwner { owner }
        public let owner: WeekOwner; public let points: Int; public let capacity: Int
        public var overbooked: Bool { points > capacity }
    }
    public static let maximumGoals = 6
    /// Points a person can take on in one week before the planner warns. A guess until real weeks show the right number.
    public static let defaultCapacity = 12

    public var name: String
    public var startsOn: String
    public var goals: [Goal]
    public var tasks: [PlannedTask]
    public var mainGoalID: String?
    private var nextID = 0

    /// Starts from the most recent week: unmet goals with open work come back as suggestions, their open tasks as carry-overs.
    public init(previous: WeekView?, today: String, name: String) {
        self.name = name
        startsOn = PlanDraft.startDate(previous:previous,today:today)
        goals = []; tasks = []
        guard let previous else { return }
        for goal in previous.goals {
            let open = previous.plan.filter { $0.goalId == goal.goalId && $0.doneBuildId == nil }
            let met = previous.progress.goals.first { $0.goalId == goal.goalId }?.met == true
            guard !open.isEmpty, !met, goals.count < PlanDraft.maximumGoals else { continue }
            let id = makeID()
            goals.append(Goal(id:id,text:goal.text,zone:goal.zone,lead:goal.lead,carriedFrom:goal.goalId))
            if goal.goalId == previous.mainGoalId { mainGoalID = id }
            for item in open { tasks.append(PlannedTask(id:makeID(),goalID:id,text:item.task,owner:item.owner,size:item.size,unlocks:item.unlocks,carried:true)) }
        }
    }

    /// The day after the last week ends; otherwise tomorrow on a Sunday, or this week's Monday.
    public static func startDate(previous: WeekView?, today: String) -> String {
        if let previous, let end = CalendarDay(previous.endsOn), let now = CalendarDay(today), now.ordinal <= end.ordinal + 7 { return end.adding(1).string }
        guard let now = CalendarDay(today) else { return today }
        return now.weekday == 6 ? now.adding(1).string : now.adding(-now.weekday).string
    }

    @discardableResult public mutating func addGoal(text: String, zone: WeekZone, lead: WeekOwner) -> String? {
        guard goals.count < PlanDraft.maximumGoals else { return nil }
        let id = makeID(); goals.append(Goal(id:id,text:text,zone:zone,lead:lead,carriedFrom:nil))
        if mainGoalID == nil { mainGoalID = id }
        return id
    }
    public mutating func removeGoal(_ id: String) {
        goals.removeAll { $0.id == id }; tasks.removeAll { $0.goalID == id }
        if mainGoalID == id { mainGoalID = goals.first?.id }
    }
    @discardableResult public mutating func addTask(goalID: String, text: String, owner: WeekOwner, size: WeekSize) -> String? {
        guard goals.contains(where:{ $0.id == goalID }) else { return nil }
        let id = makeID(); tasks.append(PlannedTask(id:id,goalID:goalID,text:text,owner:owner,size:size,unlocks:nil,carried:false)); return id
    }
    public mutating func removeTask(_ id: String) { tasks.removeAll { $0.id == id } }

    public func target(for goalID: String) -> Int { tasks.filter { $0.goalID == goalID }.reduce(0) { $0 + $1.size.points } }
    /// Everyone named on the plan, you first, then agents in crew order.
    public func loads(agents: [Agent], capacity: Int = PlanDraft.defaultCapacity) -> [Load] {
        let owners: [WeekOwner] = [.you] + agents.map { .agent($0.agentId) }
        return owners.map { owner in Load(owner:owner,points:tasks.filter { $0.owner == owner }.reduce(0) { $0 + $1.size.points },capacity:capacity) }
    }
    /// What still stops the week from being founded, in the order you would fix it.
    public var problems: [String] {
        var found: [String] = []
        if name.trimmingCharacters(in:.whitespacesAndNewlines).isEmpty { found.append("Name the city.") }
        if goals.isEmpty { found.append("Pick at least one goal.") }
        if goals.contains(where:{ $0.text.trimmingCharacters(in:.whitespacesAndNewlines).isEmpty }) { found.append("Give every goal a name.") }
        if let empty = goals.first(where:{ target(for:$0.id) == 0 }) { found.append("Add a task to “\(empty.text)”.") }
        if tasks.contains(where:{ $0.text.trimmingCharacters(in:.whitespacesAndNewlines).isEmpty }) { found.append("Give every task a name.") }
        return found
    }
    public var canFound: Bool { problems.isEmpty }

    public struct Request: Encodable, Equatable, Sendable {
        public struct Goal: Encodable, Equatable, Sendable { public let key: String; public let text: String; public let zone: WeekZone; public let lead: WeekOwner; public let targetPoints: Int }
        public struct Item: Encodable, Equatable, Sendable { public let goalKey: String; public let task: String; public let owner: WeekOwner; public let size: WeekSize; public let unlocks: String? }
        public let name: String; public let startsOn: String; public let goals: [Goal]; public let plan: [Item]; public let mainGoalKey: String?
    }
    /// A goal's target is the sum of its planned tasks, so finishing the plan meets the goal.
    public var request: Request {
        let trim = { (text: String) in text.trimmingCharacters(in:.whitespacesAndNewlines) }
        return Request(name:trim(name),startsOn:startsOn,
                       goals:goals.map { Request.Goal(key:$0.id,text:trim($0.text),zone:$0.zone,lead:$0.lead,targetPoints:target(for:$0.id)) },
                       plan:tasks.map { Request.Item(goalKey:$0.goalID,task:trim($0.text),owner:$0.owner,size:$0.size,unlocks:$0.unlocks) },
                       mainGoalKey:mainGoalID)
    }
    private mutating func makeID() -> String { nextID += 1; return "draft-\(nextID)" }
}

/// A calendar date with no time zone, counted in days so week arithmetic never crosses a daylight-saving change.
struct CalendarDay: Equatable {
    let ordinal: Int
    init?(_ value: String) {
        let parts = value.split(separator:"-").compactMap { Int($0) }
        var calendar = Calendar(identifier:.gregorian); calendar.timeZone = TimeZone(identifier:"UTC")!
        guard parts.count == 3, let date = calendar.date(from:DateComponents(year:parts[0],month:parts[1],day:parts[2])) else { return nil }
        ordinal = Int(date.timeIntervalSince1970 / 86_400)
    }
    private init(ordinal: Int) { self.ordinal = ordinal }
    func adding(_ days: Int) -> CalendarDay { CalendarDay(ordinal:ordinal + days) }
    /// Monday is 0, Sunday is 6.
    var weekday: Int { ((ordinal + 3) % 7 + 7) % 7 }
    var string: String {
        var calendar = Calendar(identifier:.gregorian); calendar.timeZone = TimeZone(identifier:"UTC")!
        let parts = calendar.dateComponents([.year,.month,.day],from:Date(timeIntervalSince1970:TimeInterval(ordinal) * 86_400))
        return String(format:"%04d-%02d-%02d",parts.year ?? 0,parts.month ?? 0,parts.day ?? 0)
    }
}
