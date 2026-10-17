import Foundation

public struct Installation: Codable, Equatable, Sendable {
    public let contractVersion: String
    public let installationId: String
    public let recoveryEpoch: String
    public let mode: String
    public let serviceName: String
    public var capabilities: [String]? = nil
    public func supports(_ capability: String) -> Bool { supportsCommands && capabilities?.contains(capability) == true }
    public var supportsCommands: Bool { contractVersion == "1.0" && ["fixture", "openai"].contains(mode) }
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
    public var instructions: String? = nil
}
public struct AgentList: Codable, Sendable { public let agents: [Agent] }
public struct SkillReference: Codable, Equatable, Sendable { public let skillId: String; public let revisionId: String }
public struct InstructionSkill: Codable, Equatable, Sendable, Identifiable {
    public var id: String { skillId }
    public let skillId: String; public let revisionId: String; public let name: String; public let instructions: String
    public var bundle: SkillBundle? = nil
    public var review: SkillReview? = nil
    public var previousRevisionId: String? = nil
    public var reference: SkillReference { SkillReference(skillId:skillId, revisionId:revisionId) }
    public var canEditInstructions: Bool {
        guard let bundle else { return true }
        return bundle.source == "owner-local:Fort-instructions" && bundle.files.count == 1 && bundle.files[0].path == bundle.entrypoint && !bundle.files[0].executable && bundle.requestedTools.isEmpty && bundle.scripts.isEmpty && bundle.supportedRuntimes == ["codex"]
    }
    public var canInvoke: Bool {
        guard let bundle else { return false }
        return bundle.schemaVersion == 1 && review?.state == "enabled" && review?.digest == bundle.digest
    }
}
public struct SkillReview: Codable, Equatable, Sendable { public let state: String; public let digest: String }
public struct SkillBundleFile: Codable, Equatable, Sendable, Identifiable {
    public var id: String { path }
    public let path: String; public let kind: String; public let executable: Bool
    public let contentBase64: String; public let bytes: Int; public let sha256: String
    public var text: String? { Data(base64Encoded:contentBase64).flatMap { String(data:$0,encoding:.utf8) } }
}
public struct SkillBundleScript: Codable, Equatable, Sendable {
    public let path: String; public let interpreter: String; public let resourceScope: [String]
}
public struct SkillFileChange: Equatable, Sendable, Identifiable {
    public var id: String { path }
    public let path: String; public let change: String
}
public struct SkillBundle: Codable, Equatable, Sendable {
    public let schemaVersion: Int; public let name: String; public let version: String
    public let source: String; public let license: String; public let entrypoint: String
    public let supportedRuntimes: [String]; public let requestedTools: [String]
    public let scripts: [SkillBundleScript]; public let files: [SkillBundleFile]; public let digest: String
    public var supportsCurrentChat: Bool { schemaVersion == 1 && supportedRuntimes.contains("codex") && requestedTools.isEmpty && scripts.isEmpty && files.contains { $0.path == entrypoint } && files.allSatisfy { $0.kind == "file" && !$0.executable && $0.text != nil } }
    public func changes(from previous: SkillBundle?) -> [SkillFileChange] {
        let before = Dictionary((previous?.files ?? []).map { ($0.path,$0) },uniquingKeysWith:{ first,_ in first })
        let after = Dictionary(files.map { ($0.path,$0) },uniquingKeysWith:{ first,_ in first })
        return Set(before.keys).union(after.keys).sorted().compactMap { path in
            guard before[path] != after[path] else { return nil }
            return SkillFileChange(path:path,change:before[path] == nil ? "Added" : after[path] == nil ? "Removed" : "Changed")
        }
    }
}
public struct Conversation: Codable, Equatable, Sendable, Identifiable {
    public var id: String { conversationId }
    public let conversationId: String; public let name: String; public let kind: String; public let participantAgentIds: [String]
}
public struct TurnConfiguration: Codable, Equatable, Sendable {
    public let agentId: String; public let agentName: String; public let bindingRevisionId: String
    public let selectedModel: String; public let instructions: String; public let skills: [InstructionSkill]
}
public struct Message: Codable, Equatable, Sendable, Identifiable {
    public var id: String { messageId }
    public let messageId: String
    public let conversationId: String
    public let requestId: String
    public let role: String
    public let text: String
    public let sequence: Int
    public let createdAt: String
    public var agentId: String? = nil
    public var agentName: String? = nil
    public var recipientAgentId: String? = nil
    public var recipientAgentName: String? = nil
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
    public var routine: RoutineExecution? = nil
    public var configuration: TurnConfiguration? = nil
    public var statusDetail: String? = nil
    public var canRequestCancellation: Bool { ["queued","running","waitingForApproval"].contains(state) }
    public var canCloseUnknown: Bool { state == "outcomeUnknown" }
    public var displayState: String {
        switch state {
        case "queued": "Accepted · waiting for host"
        case "running": "Agent is responding"
        case "completed": "Reply saved"
        case "failed": "Could not complete this turn"
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
    public var kind: String? = nil
    public var routineRevisionId: String? = nil
    public var stepId: String? = nil
    public var canDecide: Bool {
        let formatter = ISO8601DateFormatter(); formatter.formatOptions = [.withInternetDateTime,.withFractionalSeconds]
        let recognized = (kind == nil && resourceScope == ["fixture:reply"]) || (kind == "routineGate" && resourceScope == ["routine:continue"] && !(routineRevisionId ?? "").isEmpty && !(stepId ?? "").isEmpty)
        guard state == "pending", recognized, let expiry = formatter.date(from:expiresAt) else { return false }
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
public struct Receipt: Codable, Equatable, Sendable {
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

public enum RoutineValue: Codable, Equatable, Sendable {
    case text(String), number(Double), boolean(Bool)
    public var displayText: String { switch self { case .text(let value): value; case .number(let value): String(value); case .boolean(let value): value ? "Yes" : "No" } }
    public init(from decoder: Decoder) throws {
        let value = try decoder.singleValueContainer()
        if let bool = try? value.decode(Bool.self) { self = .boolean(bool) }
        else if let number = try? value.decode(Double.self), number.isFinite { self = .number(number) }
        else { self = .text(try value.decode(String.self)) }
    }
    public func encode(to encoder: Encoder) throws {
        var value = encoder.singleValueContainer()
        switch self { case .text(let text): try value.encode(text); case .number(let number): try value.encode(number); case .boolean(let bool): try value.encode(bool) }
    }
}
public struct RoutineInput: Codable, Equatable, Sendable { public var name: String; public var type: String; public var required: Bool }
public struct RoutineSkill: Codable, Equatable, Sendable { public var skillId: String; public var revisionId: String; public var digest: String }
public struct RoutineStep: Codable, Equatable, Sendable, Identifiable {
    public var id: String { stepId }
    public var stepId: String; public var name: String; public var type: String
    public var instructions: String? = nil; public var inputNames: [String]? = nil; public var outputStepIds: [String]? = nil; public var prompt: String? = nil
}
public struct RoutineDefinition: Codable, Equatable, Sendable {
    public var schemaVersion: Int; public var name: String; public var agentId: String; public var conversationId: String; public var bindingRevisionId: String; public var instructions: String
    public var inputs: [RoutineInput]; public var skills: [RoutineSkill]; public var steps: [RoutineStep]
    public var isSupported: Bool { schemaVersion == 1 && !steps.isEmpty && steps.count <= 16 && inputs.allSatisfy { ["text","number","boolean"].contains($0.type) } && steps.allSatisfy { ["task","approval"].contains($0.type) } }
}
public struct RoutineRevision: Codable, Equatable, Sendable, Identifiable {
    public var id: String { routineId }
    public let routineId: String; public let revisionId: String; public let digest: String
    public let definition: RoutineDefinition; public let agentSnapshot: Agent; public let skills: [InstructionSkill]
}
public struct RoutineStepProgress: Codable, Equatable, Sendable, Identifiable {
    public var id: String { stepId }
    public let stepId: String; public let stepRevisionId: String; public let state: String
    public var attemptId: String? = nil; public var approvalId: String? = nil; public var detail: String? = nil; public var output: Output? = nil
    public struct Output: Codable, Equatable, Sendable { public let messageId: String; public let text: String; public let digest: String }
    public var displayState: String {
        switch state { case "pending": "Waiting"; case "running": "Running"; case "completed": "Completed"; case "waitingForApproval": "Waiting for approval"; case "approved": "Approved"; case "rejected": "Rejected"; case "failed": "Failed"; case "outcomeUnknown": "Outcome unknown"; case "canceled": "Canceled"; default: "Unsupported step state" }
    }
}
public struct RoutineExecution: Codable, Equatable, Sendable {
    public let routineId: String; public let revisionId: String; public let definition: RoutineDefinition; public let inputs: [String:RoutineValue]; public let steps: [RoutineStepProgress]
}
public struct RoutineCommand: Codable, Equatable, Sendable {
    public let routineId: String; public let revisionId: String; public let definition: RoutineDefinition; public let inputs: [String:RoutineValue]
}

public struct ScheduleDefinition: Codable, Equatable, Sendable {
    public var schemaVersion: Int; public var routineId: String; public var routineRevisionId: String; public var timezone: String; public var time: String
    public var inputs: [String:RoutineValue]; public var missed: String; public var kind: String; public var date: String? = nil; public var weekdays: [Int]? = nil
    public var isSupported: Bool { schemaVersion == 1 && ["once","daily","weekly"].contains(kind) && ["skip","catchUpOne"].contains(missed) }
}
public struct CalendarOccurrence: Codable, Equatable, Sendable { public let localDate: String; public let localTime: String; public let intendedAt: String?; public let decisionAt: String }
public struct SchedulePreview: Codable, Equatable, Sendable {
    public let revisionId: String; public let digest: String; public let expiresAt: String; public let timezone: String; public let occurrences: [CalendarOccurrence]; public let overlap: String; public let missed: String
}
public struct ScheduleRevision: Codable, Equatable, Sendable, Identifiable {
    public var id: String { scheduleId }
    public let scheduleId: String; public let revisionId: String; public let digest: String; public let definition: ScheduleDefinition; public let state: String; public let checkedThrough: Double
    public var preview: SchedulePreview? = nil
}
public struct ScheduleOccurrence: Codable, Equatable, Sendable, Identifiable {
    public var id: String { occurrenceId }
    public let occurrenceId: String; public let scheduleId: String; public let scheduleRevisionId: String; public let routineId: String; public let routineRevisionId: String
    public let intendedAt: String?; public let localDate: String; public let localTime: String; public let state: String
    public var runId: String? = nil; public var detail: String? = nil
    public var displayState: String {
        switch state { case "accepted": "Accepted"; case "skippedOverlap": "Skipped: another run was active"; case "skippedMissed": "Skipped after downtime"; case "skippedGap": "Skipped: local time did not exist"; case "blocked": "Blocked: review required"; case "skippedMissedRange": "Earlier downtime skipped"; default: "Unsupported occurrence state" }
    }
}
