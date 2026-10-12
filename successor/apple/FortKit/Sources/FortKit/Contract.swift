import Foundation

public struct Installation: Codable, Equatable, Sendable {
    public let contractVersion: String
    public let installationId: String
    public let recoveryEpoch: String
    public let mode: String
    public let serviceName: String
    public var supportsCommands: Bool { contractVersion == "1.0" && mode == "fixture" }
}
public struct Enrollment: Codable, Sendable {
    public let installation: Installation
    public let deviceId: String
    public let deviceToken: String
    public let expiresAt: String
}
public struct Binding: Codable, Equatable, Sendable {
    public let bindingRevisionId: String
    public let provider: String
    public let runtimeVersion: String
    public let selectedModel: String
    public let hostId: String
    public let enabled: Bool
}
public struct Agent: Codable, Equatable, Sendable, Identifiable {
    public var id: String { agentId }
    public let agentId: String
    public let name: String
    public let homeConversationId: String
    public let binding: Binding
    public let readiness: String
}
public struct AgentList: Codable, Sendable { public let agents: [Agent] }
public struct Message: Codable, Equatable, Sendable, Identifiable {
    public var id: String { messageId }
    public let messageId: String
    public let conversationId: String
    public let requestId: String
    public let role: String
    public let text: String
    public let sequence: Int
    public let createdAt: String
}
public struct Run: Codable, Equatable, Sendable, Identifiable {
    public var id: String { runId }
    public let runId: String
    public let requestId: String
    public let conversationId: String
    public let bindingRevisionId: String
    public let state: String
    public let selectedModel: String
    public let effectiveModel: String?
    public let modelEvidence: String
    public let attemptId: String?
    public var displayState: String {
        switch state {
        case "queued": "Accepted · waiting for fixture host"
        case "running": "Fixture is running"
        case "completed": "Fixture reply saved"
        case "outcomeUnknown": "Outcome unknown · review required"
        case "canceled": "Canceled"
        case "closedUnknown": "Closed without retry · original outcome remains unknown"
        case "cancelRequested": "Cancellation requested · waiting for confirmation"
        case "waitingForApproval": "Waiting for your approval"
        default: "Unsupported state · refresh or update Fort"
        }
    }
}
public struct Approval: Codable, Equatable, Sendable, Identifiable {
    public var id: String { approvalId }
    public let approvalId: String
    public let installationId: String
    public let recoveryEpoch: String
    public let runId: String
    public let attemptId: String
    public let bindingRevisionId: String
    public let actionDigest: String
    public let resourceScope: [String]
    public let actionDescription: String
    public let expiresAt: String
    public let state: String
    public let decidedByDeviceId: String?
    public let decidedAt: String?
    public var canDecide: Bool {
        let formatter = ISO8601DateFormatter(); formatter.formatOptions = [.withInternetDateTime,.withFractionalSeconds]
        guard state == "pending", resourceScope == ["fixture:reply"], let expiry = formatter.date(from:expiresAt) else { return false }
        return expiry > Date()
    }
}
public struct ApprovalResolution: Codable, Sendable { public let approval: Approval; public let run: Run }
public struct RunResponse: Codable, Sendable { public let run: Run }
public struct Snapshot: Codable, Equatable, Sendable {
    public let installationId: String
    public let recoveryEpoch: String
    public let conversationId: String
    public let cursor: Int
    public let messages: [Message]
    public let runs: [Run]
    public let lastReadMessageSequence: Int
    public let approvals: [Approval]
    public init(installationId: String, recoveryEpoch: String, conversationId: String, cursor: Int, messages: [Message], runs: [Run], lastReadMessageSequence: Int, approvals: [Approval] = []) {
        self.installationId=installationId; self.recoveryEpoch=recoveryEpoch; self.conversationId=conversationId; self.cursor=cursor; self.messages=messages; self.runs=runs; self.lastReadMessageSequence=lastReadMessageSequence; self.approvals=approvals
    }
    private enum CodingKeys: CodingKey { case installationId,recoveryEpoch,conversationId,cursor,messages,runs,lastReadMessageSequence,approvals }
    public init(from decoder: Decoder) throws {
        let values = try decoder.container(keyedBy:CodingKeys.self)
        installationId = try values.decode(String.self,forKey:.installationId)
        recoveryEpoch = try values.decode(String.self,forKey:.recoveryEpoch)
        conversationId = try values.decode(String.self,forKey:.conversationId)
        cursor = try values.decode(Int.self,forKey:.cursor)
        messages = try values.decode([Message].self,forKey:.messages)
        runs = try values.decode([Run].self,forKey:.runs)
        lastReadMessageSequence = try values.decode(Int.self,forKey:.lastReadMessageSequence)
        approvals = try values.decodeIfPresent([Approval].self,forKey:.approvals) ?? []
    }
}
public struct Receipt: Codable, Sendable {
    public let requestId: String
    public let conversationId: String
    public let messageId: String
    public let runId: String
    public let bindingRevisionId: String
    public let inputDigest: String
    public let acceptedAt: String
}
public struct Acceptance: Codable, Sendable { public let receipt: Receipt; public let snapshot: Snapshot }
public struct EventPage: Codable, Sendable {
    public let installationId: String
    public let recoveryEpoch: String
    public let cursor: Int
    public let events: [DurableEvent]
}
public struct DurableEvent: Codable, Sendable {
    public let schemaVersion: Int
    public let installationId: String
    public let recoveryEpoch: String
    public let eventId: String
    public let sequence: Int
    public let entityId: String
    public let conversationId: String
    public let kind: String
    public let createdAt: String
}
public enum ContractError: Error { case wrongInstallation, wrongConversation, invalidSnapshot }
public struct Transcript: Sendable {
    public private(set) var snapshot: Snapshot?
    public let installationID: String
    public let conversationID: String
    public init(installationID: String, conversationID: String) { self.installationID = installationID; self.conversationID = conversationID }
    public mutating func apply(_ incoming: Snapshot) throws {
        guard incoming.installationId == installationID else { throw ContractError.wrongInstallation }
        guard incoming.conversationId == conversationID else { throw ContractError.wrongConversation }
        guard incoming.cursor >= 0, Set(incoming.messages.map(\.messageId)).count == incoming.messages.count,
              incoming.messages.allSatisfy({ $0.conversationId == conversationID && $0.sequence > 0 && $0.sequence <= incoming.cursor }),
              zip(incoming.messages, incoming.messages.dropFirst()).allSatisfy({ $0.sequence < $1.sequence }) else { throw ContractError.invalidSnapshot }
        if let existing = snapshot, existing.recoveryEpoch == incoming.recoveryEpoch, incoming.cursor < existing.cursor { return }
        snapshot = incoming
    }
}
