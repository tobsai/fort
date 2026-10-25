import Foundation

/// What the agent editor shows as a character sheet: gear the agent can equip, and a size note only when it matters.
enum AgentSheet {
    static let instructionLimit = 16_384

    struct Gear: Equatable, Sendable, Identifiable {
        let id: String
        let name: String
        let selectable: Bool
        /// Why a model can't be picked, or what the runtime says about it. Unavailable models stay listed so nothing silently disappears.
        let note: String?
    }

    static func gear(_ options: [ModelOption]) -> [Gear] {
        options.map { option in
            switch option.availability {
            case "available": Gear(id:option.id,name:option.name,selectable:true,note:option.detail)
            case "unchecked": Gear(id:option.id,name:option.name,selectable:false,note:option.detail ?? "Not checked yet.")
            default: Gear(id:option.id,name:option.name,selectable:false,note:option.detail ?? "Unavailable right now.")
            }
        }
    }

    /// The byte counter only appears near the limit (three quarters full), and says how much to cut once over it.
    static func sizeNote(_ text: String) -> String? {
        let count = text.utf8.count
        if count > instructionLimit { return "Too long by \((count - instructionLimit).formatted()) bytes." }
        if count * 4 >= instructionLimit * 3 { return "\(count.formatted()) / \(instructionLimit.formatted()) bytes" }
        return nil
    }

    static func instructionsTitle(name: String) -> String {
        let trimmed = name.trimmingCharacters(in:.whitespacesAndNewlines)
        return "How \(trimmed.isEmpty ? "this agent" : trimmed) should work"
    }
}

/// The phone's join card: paste the invitation, then join. The individual fields stay reachable in the Workshop.
enum JoinPanel: Equatable, Sendable {
    case paste
    case ready(host: String)
    case pending

    static func make(address: String, code: String, hasPending: Bool) -> JoinPanel {
        if hasPending { return .pending }
        guard !code.isEmpty, let host = URL(string:address)?.host(), !host.isEmpty else { return .paste }
        return .ready(host:host)
    }
}
