import Foundation

// Spec 068: agent memory. Which memory system each Agent uses, what it holds, and switching it, the same records the
// hub's /v1/memory routes return and the web's Setup, Agents page shows.

/// A memory system the host offers. "none" is No memory, which needs no module.
public struct MemorySystem: Codable, Equatable, Sendable, Identifiable {
    public let id: String
    public let version: String
    public let name: String
    public let summary: String
    public let storage: String
    public let capabilities: [String]
    public func can(_ capability: String) -> Bool { capabilities.contains(capability) }
    public static let noMemoryID = "none"
}
public struct MemoryChoice: Codable, Equatable, Sendable {
    public let agentId: String
    public let providerId: String
    public let revisionId: String
    public let changedAt: String
}
/// One Agent's memory at a glance. state is ready, missing (its module was removed or isn't answering) or off (No memory).
public struct AgentMemoryStatus: Codable, Equatable, Sendable {
    public let agentId: String
    public let agentName: String
    public let choice: MemoryChoice
    public let provider: MemorySystem?
    public let state: String
    public let detail: String
    public let total: Int
}
public struct MemoryNote: Codable, Equatable, Sendable, Identifiable {
    public var id: String { entryId }
    public let entryId: String
    public let agentId: String
    public let kind: String
    public let text: String
    public let createdAt: String
}
public struct MemoryOverview: Codable, Equatable, Sendable {
    public let providers: [MemorySystem]
    public let defaultProviderId: String
    public let agents: [AgentMemoryStatus]
}
/// One Agent's memory page: its status, the systems on offer and the notes it holds (or those matching a search).
public struct AgentMemoryPage: Codable, Equatable, Sendable {
    public let agentId: String
    public let agentName: String
    public let choice: MemoryChoice
    public let provider: MemorySystem?
    public let state: String
    public let detail: String
    public let total: Int
    public let providers: [MemorySystem]
    public let entries: [MemoryNote]
    public var status: AgentMemoryStatus { AgentMemoryStatus(agentId:agentId,agentName:agentName,choice:choice,provider:provider,state:state,detail:detail,total:total) }
}
public struct MemorySwitchReceipt: Codable, Equatable, Sendable {
    public let choice: MemoryChoice
    public let state: String
    public let carried: Int
    public var carryDetail: String? = nil
}

/// The words the apps show for memory, the same as the web's transcript.js memoryLine and canCarryMemory.
enum AgentMemoryCopy {
    static func line(_ status: AgentMemoryStatus?) -> String {
        guard let status else { return "Memory: checking…" }
        switch status.state {
        case "off": return "No memory"
        case "missing": return status.provider.map { "\($0.name) isn’t answering" } ?? "Memory system missing"
        default: return "\(status.provider?.name ?? "Memory") · \(notes(status.total))"
        }
    }
    static func notes(_ count: Int) -> String { count == 1 ? "1 note" : "\(count) notes" }
    /// Notes come along on a switch only when the old system can hand them over and the new one can take them.
    static func canCarry(from fromID: String, to: MemorySystem?, providers: [MemorySystem]) -> Bool {
        guard let to, let from = providers.first(where:{ $0.id == fromID }), from.id != to.id else { return false }
        return from.can("export") && to.can("import")
    }
    /// What picking a system means, with a reminder that the old system keeps its notes.
    static func about(selected: MemorySystem?, page: AgentMemoryPage) -> String {
        guard let selected else { return page.detail }
        if selected.id == page.choice.providerId && page.state == "missing" { return page.detail }
        var text = [selected.summary,selected.storage].filter { !$0.isEmpty }.joined(separator:" ")
        if selected.id != page.choice.providerId, let current = page.provider, current.id != MemorySystem.noMemoryID {
            text += " \(current.name) keeps its notes, so switching back finds them."
        }
        return text
    }
}
