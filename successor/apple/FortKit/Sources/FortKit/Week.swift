import Foundation

/// A week of planned work, as the core stores it. Points, grade and goal progress come from the core; the app never recomputes them.
public enum WeekZone: String, Codable, Sendable, CaseIterable { case code, money, writing, personal }
public enum WeekSize: String, Codable, Sendable, CaseIterable {
    case small, moderate, significant, milestone
    public var points: Int { switch self { case .small: 1; case .moderate: 3; case .significant: 6; case .milestone: 12 } }
}
public enum WeekOwner: Codable, Equatable, Hashable, Sendable {
    case you, agent(String)
    private enum CodingKeys: String, CodingKey { case kind, agentId }
    public init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy:CodingKeys.self)
        switch try container.decode(String.self,forKey:.kind) {
        case "you": self = .you
        case "agent": self = .agent(try container.decode(String.self,forKey:.agentId))
        default: throw DecodingError.dataCorruptedError(forKey:.kind,in:container,debugDescription:"Unknown week owner")
        }
    }
    public func encode(to encoder: Encoder) throws {
        var container = encoder.container(keyedBy:CodingKeys.self)
        switch self {
        case .you: try container.encode("you",forKey:.kind)
        case .agent(let id): try container.encode("agent",forKey:.kind); try container.encode(id,forKey:.agentId)
        }
    }
}
public struct WeekGoal: Codable, Equatable, Sendable, Identifiable {
    public var id: String { goalId }
    public let goalId: String; public let text: String; public let zone: WeekZone; public let lead: WeekOwner; public let targetPoints: Int
}
public struct WeekPlanItem: Codable, Equatable, Sendable, Identifiable {
    public var id: String { itemId }
    public let itemId: String; public let goalId: String; public let task: String; public let owner: WeekOwner; public let size: WeekSize
    public var unlocks: String? = nil
    public let doneBuildId: String?
}
public struct WeekBuild: Codable, Equatable, Sendable, Identifiable {
    public var id: String { buildId }
    public let buildId: String; public let goalId: String; public let itemId: String?; public let task: String; public let size: WeekSize; public let by: WeekOwner; public let placedAt: String
    public var evidence: String? = nil
}
public struct WeekGoalProgress: Codable, Equatable, Sendable { public let goalId: String; public let points: Int; public let targetPoints: Int; public let met: Bool }
public struct WeekProgress: Codable, Equatable, Sendable { public let points: Int; public let goals: [WeekGoalProgress]; public let goalsMet: Int; public let goalsCounted: Int; public let grade: String? }
public struct WeekView: Codable, Equatable, Sendable, Identifiable {
    public var id: String { weekId }
    public let weekId: String; public let revisionId: String; public let number: Int; public let name: String
    public let startsOn: String; public let endsOn: String; public let state: String; public let mainGoalId: String?
    public let goals: [WeekGoal]; public let plan: [WeekPlanItem]; public let builds: [WeekBuild]
    public var closedAt: String? = nil
    public let progress: WeekProgress
    public var isLive: Bool { state == "live" }
}
