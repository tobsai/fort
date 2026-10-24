import Foundation

/// What the city HUD shows, built only from real records: the live week, the crew, and the open conversation's runs and approvals.
public struct CityHUD: Equatable, Sendable {
    public enum MemberState: Equatable, Sendable { case ready, busy, needsYou }
    public struct Member: Identifiable, Equatable, Sendable {
        public let id: String; public let name: String; public let avatar: AgentAvatar; public let state: MemberState
        public let nextTask: String?; public let openTasks: Int
    }
    public struct GoalLine: Identifiable, Equatable, Sendable {
        public let id: String; public let text: String; public let zone: WeekZone; public let points: Int; public let targetPoints: Int; public let isMain: Bool
        public var fraction: Double { targetPoints > 0 ? min(1,Double(points)/Double(targetPoints)) : 0 }
    }
    public enum OrderKind: Equatable, Sendable { case decision(Approval), quest(WeekPlanItem) }
    public struct Order: Identifiable, Equatable, Sendable {
        public let id: String; public let title: String; public let detail: String; public let kind: OrderKind
    }
    public let title: String
    public let dayLabel: String?
    public let daysLeft: Int?
    public let points: Int
    public let grade: String?
    public let members: [Member]
    public let goals: [GoalLine]
    public let orders: [Order]

    /// - Parameter today: the owner's local calendar date as YYYY-MM-DD.
    public static func make(week: WeekView?, agents: [Agent], snapshot: Snapshot?, today: String) -> CityHUD {
        let approvals = (snapshot?.approvals ?? []).filter { $0.state == "pending" }
        let runs = snapshot?.runs ?? []
        let open = week?.plan.filter { $0.doneBuildId == nil } ?? []
        let members = agents.map { agent -> Member in
            let mine = open.filter { $0.owner == .agent(agent.agentId) }
            let inHome = snapshot?.conversationId == agent.homeConversationId
            let waiting = inHome && approvals.contains { approval in runs.contains { $0.runId == approval.runId } }
            let working = inHome && runs.contains { ["queued","running","cancelRequested"].contains($0.state) }
            return Member(id:agent.agentId,name:agent.name,avatar:agent.avatar ?? .orbit,state:waiting ? .needsYou : working ? .busy : .ready,nextTask:mine.first?.task,openTasks:mine.count)
        }
        let goals = (week?.goals ?? []).map { goal in
            let progress = week?.progress.goals.first { $0.goalId == goal.goalId }
            return GoalLine(id:goal.goalId,text:goal.text,zone:goal.zone,points:progress?.points ?? 0,targetPoints:goal.targetPoints,isMain:goal.goalId == week?.mainGoalId)
        }
        let decisions = approvals.map { Order(id:$0.approvalId,title:$0.actionDescription,detail:"Decision",kind:.decision($0)) }
        let quests = open.filter { $0.owner == .you }.map { item in
            let goal = week?.goals.first { $0.goalId == item.goalId }?.text ?? ""
            return Order(id:item.itemId,title:item.task,detail:item.unlocks.map { "\(goal) · unlocks \($0)" } ?? goal,kind:.quest(item))
        }
        var dayLabel: String?, daysLeft: Int?
        if let week, week.isLive, let start = day(week.startsOn), let now = day(today) {
            let index = max(0,min(6,Int((now - start) / 86_400)))
            dayLabel = ["Mon","Tue","Wed","Thu","Fri","Sat","Sun"][(weekday(week.startsOn) + index) % 7]
            daysLeft = 6 - index
        }
        return CityHUD(title:week.map { "Week \($0.number) · \($0.name)" } ?? "No week planned",dayLabel:dayLabel,daysLeft:daysLeft,points:week?.progress.points ?? 0,grade:week?.progress.grade,members:members,goals:goals,orders:decisions + quests)
    }
    private static func day(_ value: String) -> TimeInterval? {
        var calendar = Calendar(identifier:.gregorian); calendar.timeZone = TimeZone(identifier:"UTC")!
        let parts = value.split(separator:"-").compactMap { Int($0) }
        guard parts.count == 3, let date = calendar.date(from:DateComponents(year:parts[0],month:parts[1],day:parts[2])) else { return nil }
        return date.timeIntervalSince1970
    }
    /// Monday is 0.
    private static func weekday(_ value: String) -> Int {
        guard let seconds = day(value) else { return 0 }
        return (Int(seconds / 86_400) + 3) % 7
    }
}
