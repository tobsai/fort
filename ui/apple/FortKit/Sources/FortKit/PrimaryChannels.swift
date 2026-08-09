//
//  PrimaryChannels.swift
//  FortKit
//
//  Exact wire models and pure presentation primitives for Spec 044 Phase 1.
//

import Foundation

// MARK: - Primary Agent authority

public struct PrimarySeat: Codable, Sendable, Hashable, Identifiable {
    public let id: String
    public let profile: String
    public let agent: String
    public let model: String?
    public let machine: String
    public let displayName: String
    public let state: String
    public let reason: String?

    enum CodingKeys: String, CodingKey {
        case id, profile, agent, model, machine, state, reason
        case displayName = "display_name"
    }
}

public struct PrimarySubscriptionPolicy: Codable, Sendable, Hashable {
    public let policyID: String
    public let policyRevision: String
    public let adapterID: String
    public let adapterRevision: String
    public let codexVersion: String
    public let codexExecutableRevision: String
    public let codexSchemaRevision: String
    public let runtimeContract: String
    public let reasoningEffort: String
    public let reasoningContext: String
    public let requestTimeoutMillis: Int
    public let developerInstructionRevision: String
    public let accountType: String
    public let accountPlan: String
    public let threadMode: String
    public let sandboxMode: String
    public let approvalPolicy: String
    public let workdirMode: String
    public let dynamicToolsMode: String
    public let mcpMode: String
    public let commandPolicy: String
    public let fileReadPolicy: String
    public let isolationRevision: String

    enum CodingKeys: String, CodingKey {
        case policyID = "policy_id"
        case policyRevision = "policy_revision"
        case adapterID = "adapter_id"
        case adapterRevision = "adapter_revision"
        case codexVersion = "codex_version"
        case codexExecutableRevision = "codex_executable_revision"
        case codexSchemaRevision = "codex_schema_revision"
        case runtimeContract = "runtime_contract"
        case reasoningEffort = "reasoning_effort"
        case reasoningContext = "reasoning_context"
        case requestTimeoutMillis = "request_timeout_millis"
        case developerInstructionRevision = "developer_instruction_revision"
        case accountType = "account_type"
        case accountPlan = "account_plan"
        case threadMode = "thread_mode"
        case sandboxMode = "sandbox_mode"
        case approvalPolicy = "approval_policy"
        case workdirMode = "workdir_mode"
        case dynamicToolsMode = "dynamic_tools_mode"
        case mcpMode = "mcp_mode"
        case commandPolicy = "command_policy"
        case fileReadPolicy = "file_read_policy"
        case isolationRevision = "isolation_revision"
    }
}

public struct PrimaryAuthorityOffer: Codable, Sendable, Hashable {
    public let offerVersion: Int
    public let machineID: String
    public let seatID: String
    public let agentKey: String
    public let profileID: String
    public let requestedModel: String
    public let resolvedModel: String
    public let accountType: String
    public let accountPlan: String
    public let policyID: String
    public let policyRevision: String
    public let runtimeContract: String
    public let reasoningEffort: String
    public let reasoningContext: String
    public let requestTimeoutMillis: Int
    public let developerInstructionRevision: String
    public let adapterID: String
    public let adapterRevision: String
    public let codexVersion: String
    public let codexExecutableRevision: String
    public let codexSchemaRevision: String
    public let threadMode: String
    public let sandboxMode: String
    public let approvalPolicy: String
    public let workdirMode: String
    public let dynamicToolsMode: String
    public let mcpMode: String
    public let commandPolicy: String
    public let fileReadPolicy: String
    public let isolationRevision: String

    enum CodingKeys: String, CodingKey {
        case offerVersion = "offer_version"
        case machineID = "machine_id"
        case seatID = "seat_id"
        case agentKey = "agent_key"
        case profileID = "profile_id"
        case requestedModel = "requested_model"
        case resolvedModel = "resolved_model"
        case accountType = "account_type"
        case accountPlan = "account_plan"
        case policyID = "policy_id"
        case policyRevision = "policy_revision"
        case runtimeContract = "runtime_contract"
        case reasoningEffort = "reasoning_effort"
        case reasoningContext = "reasoning_context"
        case requestTimeoutMillis = "request_timeout_millis"
        case developerInstructionRevision = "developer_instruction_revision"
        case adapterID = "adapter_id"
        case adapterRevision = "adapter_revision"
        case codexVersion = "codex_version"
        case codexExecutableRevision = "codex_executable_revision"
        case codexSchemaRevision = "codex_schema_revision"
        case threadMode = "thread_mode"
        case sandboxMode = "sandbox_mode"
        case approvalPolicy = "approval_policy"
        case workdirMode = "workdir_mode"
        case dynamicToolsMode = "dynamic_tools_mode"
        case mcpMode = "mcp_mode"
        case commandPolicy = "command_policy"
        case fileReadPolicy = "file_read_policy"
        case isolationRevision = "isolation_revision"
    }
}

public struct PrimaryAgentSetting: Codable, Sendable, Hashable {
    public let optionID: String
    public let seat: PrimarySeat
    public let authority: String
    public let policy: PrimarySubscriptionPolicy
    public let updatedAt: String

    enum CodingKeys: String, CodingKey {
        case optionID = "option_id"
        case seat, authority, policy
        case updatedAt = "updated_at"
    }
}

public struct PrimaryAgentOption: Codable, Sendable, Hashable, Identifiable {
    public var id: String { optionID }
    public let optionID: String
    public let state: String
    public let reason: String?
    public let seat: PrimarySeat
    public let authority: PrimaryAuthorityOffer
    public let displayName: String

    enum CodingKeys: String, CodingKey {
        case optionID = "option_id"
        case state, reason, seat, authority
        case displayName = "display_name"
    }
}

public struct PrimaryScheduleInventoryItem: Codable, Sendable, Hashable, Identifiable {
    public let id: String
    public let kind: String
    public let expression: String
    public let timezone: String
    public let flowID: String
    public let flowDigest: String

    enum CodingKeys: String, CodingKey {
        case id, kind, expression, timezone
        case flowID = "flow_id"
        case flowDigest = "flow_digest"
    }
}

public struct PrimaryScheduleInventory: Codable, Sendable, Hashable {
    public let currentDigest: String
    public let acceptedDigest: String?
    public let state: String
    public let items: [PrimaryScheduleInventoryItem]

    enum CodingKeys: String, CodingKey {
        case currentDigest = "current_digest"
        case acceptedDigest = "accepted_digest"
        case state, items
    }
}

public struct PrimaryAgentView: Codable, Sendable, Hashable {
    public let selection: PrimaryAgentSetting?
    public let state: String
    public let reason: String?
    public let options: [PrimaryAgentOption]
    public let scheduleInventory: PrimaryScheduleInventory?

    enum CodingKeys: String, CodingKey {
        case selection, state, reason, options
        case scheduleInventory = "schedule_inventory"
    }
}

// MARK: - Primary Channels

public struct PrimaryConversation: Codable, Sendable, Hashable, Identifiable {
    public let id: String
    public let projectID: String?
    public let title: String
    public let state: String
    public let createdAt: String
    public let updatedAt: String

    enum CodingKeys: String, CodingKey {
        case id, title, state
        case projectID = "project_id"
        case createdAt = "created_at"
        case updatedAt = "updated_at"
    }
}

public struct PrimaryParticipant: Codable, Sendable, Hashable, Identifiable {
    public let id: String
    public let conversationID: String
    public let seatID: String
    public let profile: String
    public let agent: String
    public let model: String?
    public let machine: String
    public let displayName: String
    public let position: Int
    public let state: String
    public let createdAt: String
    public let removedAt: String?

    enum CodingKeys: String, CodingKey {
        case id, profile, agent, model, machine, position, state
        case conversationID = "conversation_id"
        case seatID = "seat_id"
        case displayName = "display_name"
        case createdAt = "created_at"
        case removedAt = "removed_at"
    }
}

public struct PrimaryTurn: Codable, Sendable, Hashable, Identifiable {
    public let id: String
    public let conversationID: String
    public let clientTurnID: String
    public let promptMessageID: Int64
    public let throughMessageID: Int64
    public let createdAt: String

    enum CodingKeys: String, CodingKey {
        case id
        case conversationID = "conversation_id"
        case clientTurnID = "client_turn_id"
        case promptMessageID = "prompt_message_id"
        case throughMessageID = "through_message_id"
        case createdAt = "created_at"
    }
}

public struct PrimaryTargetAuthority: Codable, Sendable, Hashable {
    public let authority: String
    public let policy: PrimarySubscriptionPolicy
    public let requestedModel: String

    enum CodingKeys: String, CodingKey {
        case authority, policy
        case requestedModel = "requested_model"
    }
}

public struct PrimaryTargetReceipt: Codable, Sendable, Hashable {
    public let observedAdapterID: String?
    public let observedAdapterRevision: String?
    public let observedCodexVersion: String?
    public let observedCodexExecutableRevision: String?
    public let observedCodexSchemaRevision: String?
    public let resolvedModel: String?
    public let providerThreadID: String?
    public let providerTerminalStatus: String?
    public let usageSource: String?
    public let inputTokens: Int64?
    public let cachedInputTokens: Int64?
    public let outputTokens: Int64?
    public let reasoningTokens: Int64?

    enum CodingKeys: String, CodingKey {
        case observedAdapterID = "observed_adapter_id"
        case observedAdapterRevision = "observed_adapter_revision"
        case observedCodexVersion = "observed_codex_version"
        case observedCodexExecutableRevision = "observed_codex_executable_revision"
        case observedCodexSchemaRevision = "observed_codex_schema_revision"
        case resolvedModel = "resolved_model"
        case providerThreadID = "provider_thread_id"
        case providerTerminalStatus = "provider_terminal_status"
        case usageSource = "usage_source"
        case inputTokens = "input_tokens"
        case cachedInputTokens = "cached_input_tokens"
        case outputTokens = "output_tokens"
        case reasoningTokens = "reasoning_tokens"
    }
}

public struct PrimaryTarget: Codable, Sendable, Hashable, Identifiable {
    public let id: String
    public let turnID: String
    public let participantID: String
    public let runID: String
    public let attempt: Int
    public let state: String
    public let errorCode: String?
    public let error: String?
    public let authority: PrimaryTargetAuthority?
    public let receipt: PrimaryTargetReceipt?
    public let createdAt: String
    public let updatedAt: String

    public init(
        id: String,
        turnID: String,
        participantID: String,
        runID: String,
        attempt: Int,
        state: String,
        errorCode: String? = nil,
        error: String? = nil,
        authority: PrimaryTargetAuthority? = nil,
        receipt: PrimaryTargetReceipt? = nil,
        createdAt: String,
        updatedAt: String
    ) {
        self.id = id
        self.turnID = turnID
        self.participantID = participantID
        self.runID = runID
        self.attempt = attempt
        self.state = state
        self.errorCode = errorCode
        self.error = error
        self.authority = authority
        self.receipt = receipt
        self.createdAt = createdAt
        self.updatedAt = updatedAt
    }

    enum CodingKeys: String, CodingKey {
        case id, attempt, state, error, authority, receipt
        case turnID = "turn_id"
        case participantID = "participant_id"
        case runID = "run_id"
        case errorCode = "error_code"
        case createdAt = "created_at"
        case updatedAt = "updated_at"
    }
}

public struct PrimaryMessage: Codable, Sendable, Hashable, Identifiable {
    public let id: Int64
    public let conversationID: String
    public let turnID: String?
    public let targetID: String?
    public let authorKind: String
    public let authorID: String
    public let body: String
    public let createdAt: String

    enum CodingKeys: String, CodingKey {
        case id, body
        case conversationID = "conversation_id"
        case turnID = "turn_id"
        case targetID = "target_id"
        case authorKind = "author_kind"
        case authorID = "author_id"
        case createdAt = "created_at"
    }
}

public struct PrimaryChannelIdentity: Codable, Sendable, Hashable {
    public let conversationID: String
    public let participantID: String
    public let authority: String
    public let policy: PrimarySubscriptionPolicy
    public let createdAt: String

    enum CodingKeys: String, CodingKey {
        case conversationID = "conversation_id"
        case participantID = "participant_id"
        case authority, policy
        case createdAt = "created_at"
    }
}

public struct PrimaryChannelSummary: Codable, Sendable, Hashable, Identifiable {
    public var id: String { conversation.id }
    public let conversation: PrimaryConversation
    public let participant: PrimaryParticipant
    public let primaryIdentity: PrimaryChannelIdentity
    public let pinned: Bool
    public let pinnedAt: String?

    enum CodingKeys: String, CodingKey {
        case conversation, participant, pinned
        case primaryIdentity = "primary_identity"
        case pinnedAt = "pinned_at"
    }
}

public struct PrimaryChannelReadiness: Codable, Sendable, Hashable {
    public let state: String
    public let reason: String?
    public let observedAt: String

    enum CodingKeys: String, CodingKey {
        case state, reason
        case observedAt = "observed_at"
    }
}

public struct PrimaryChannelDetail: Codable, Sendable, Hashable {
    public let conversation: PrimaryConversation
    public let participants: [PrimaryParticipant]
    public let messages: [PrimaryMessage]
    public let turns: [PrimaryTurn]
    public let targets: [PrimaryTarget]
    public let primaryIdentity: PrimaryChannelIdentity?
    public let readiness: PrimaryChannelReadiness

    enum CodingKeys: String, CodingKey {
        case conversation, participants, messages, turns, targets, readiness
        case primaryIdentity = "primary_identity"
    }
}

public struct PrimaryTurnResult: Codable, Sendable, Hashable {
    public let turn: PrimaryTurn
    public let targets: [PrimaryTarget]
}

public struct PrimaryNeedsYouItem: Codable, Sendable, Hashable, Identifiable {
    public var id: String { target.id }
    public let channel: PrimaryChannelSummary
    public let target: PrimaryTarget
    public let recoveryActions: [String]

    enum CodingKeys: String, CodingKey {
        case channel, target
        case recoveryActions = "recovery_actions"
    }
}

// MARK: - Primary schedule read models

public struct PrimaryScheduleOccurrence: Codable, Sendable, Hashable, Identifiable {
    public let id: String
    public let scheduleID: String
    public let runID: String?
    public let scheduledFor: String
    public let state: String
    public let error: String?
    public let createdAt: String
    public let updatedAt: String

    enum CodingKeys: String, CodingKey {
        case id, state, error
        case scheduleID = "schedule_id"
        case runID = "run_id"
        case scheduledFor = "scheduled_for"
        case createdAt = "created_at"
        case updatedAt = "updated_at"
    }
}

public struct PrimaryRelatedChannel: Codable, Sendable, Hashable, Identifiable {
    public let id: String
    public let name: String
}

public struct PrimaryScheduleItem: Codable, Sendable, Hashable, Identifiable {
    public let id: String
    public let title: String
    public let enabled: Bool
    public let kind: String
    public let expression: String
    public let recurrence: String
    public let timezone: String
    public let nextFireAt: String?
    public let lastFireAt: String?
    public let targetKind: String
    public let targetID: String
    public let relatedChannel: PrimaryRelatedChannel?
    public let latestOccurrence: PrimaryScheduleOccurrence?
    public let schedulerOwnership: String
    public let observedAt: String

    enum CodingKeys: String, CodingKey {
        case id, title, enabled, kind, expression, recurrence, timezone
        case nextFireAt = "next_fire_at"
        case lastFireAt = "last_fire_at"
        case targetKind = "target_kind"
        case targetID = "target_id"
        case relatedChannel = "related_channel"
        case latestOccurrence = "latest_occurrence"
        case schedulerOwnership = "scheduler_ownership"
        case observedAt = "observed_at"
    }
}

public struct PrimaryScheduleList: Codable, Sendable, Hashable {
    public let snapshotID: String
    public let observedAt: String
    public let items: [PrimaryScheduleItem]

    enum CodingKeys: String, CodingKey {
        case snapshotID = "snapshot_id"
        case observedAt = "observed_at"
        case items
    }
}

public struct PrimaryScheduleDetail: Codable, Sendable, Hashable {
    public let item: PrimaryScheduleItem
    public let upcoming: [PrimaryScheduleOccurrence]
    public let recent: [PrimaryScheduleOccurrence]
}

// MARK: - Requests and filters

public enum PrimaryChannelFilter: String, Codable, Sendable, CaseIterable {
    case open
    case archived
    case all
}

public enum PrimaryChannelState: String, Codable, Sendable, CaseIterable {
    case open
    case archived
}

public enum PrimaryScheduleFilter: String, Codable, Sendable, CaseIterable {
    case active
    case paused
    case all
}

public struct PrimaryTurnRequest: Codable, Sendable, Hashable {
    public let clientTurnID: String
    public let text: String

    public init(clientTurnID: String, text: String) {
        self.clientTurnID = clientTurnID
        self.text = text
    }

    enum CodingKeys: String, CodingKey {
        case clientTurnID = "client_turn_id"
        case text
    }
}

/// A submission remains stable across ambiguous transport failures so a retry
/// reuses the same durable idempotency key. A coded HTTP rejection is terminal
/// for that pending submission.
public struct PrimaryPendingTurn: Sendable, Hashable {
    public let channelID: String
    public let text: String
    public let clientTurnID: String

    public init(channelID: String, text: String, clientTurnID: String) {
        self.channelID = channelID
        self.text = text
        self.clientTurnID = clientTurnID
    }

    public init(channelID: String, text: String, uuid: UUID = UUID()) {
        self.init(channelID: channelID, text: text, clientTurnID: uuid.uuidString.lowercased())
    }

    public var request: PrimaryTurnRequest {
        PrimaryTurnRequest(clientTurnID: clientTurnID, text: text)
    }

    public func retained(after errorCode: FortServerErrorCode?) -> PrimaryPendingTurn? {
        errorCode == nil ? self : nil
    }
}

// MARK: - Pure progressive status presentation

public enum PrimaryTargetAction: String, Codable, Sendable, Hashable {
    case cancel
    case retry
    case recheckAndRetry = "recheck_and_retry"
}

public enum PrimaryTargetPresentationKind: String, Codable, Sendable, Hashable {
    case queued
    case working
    case interrupted
    case failed
    case canceled
}

public struct PrimaryTargetPresentation: Sendable, Hashable {
    public let kind: PrimaryTargetPresentationKind
    public let title: String
    public let body: String
    public let action: PrimaryTargetAction?
    public let showsDetails: Bool

    public init(
        kind: PrimaryTargetPresentationKind,
        title: String,
        body: String = "",
        action: PrimaryTargetAction? = nil,
        showsDetails: Bool
    ) {
        self.kind = kind
        self.title = title
        self.body = body
        self.action = action
        self.showsDetails = showsDetails
    }
}

public enum PrimaryTargetStatusReducer {
    public static func latestAttempt(
        in targets: [PrimaryTarget],
        turnID: String,
        participantID: String
    ) -> PrimaryTarget? {
        targets
            .filter { $0.turnID == turnID && $0.participantID == participantID }
            .max { lhs, rhs in
                lhs.attempt == rhs.attempt ? lhs.id < rhs.id : lhs.attempt < rhs.attempt
            }
    }

    public static func latestAttemptsByTurn(_ targets: [PrimaryTarget]) -> [String: PrimaryTarget] {
        targets.reduce(into: [:]) { latest, target in
            guard let current = latest[target.turnID] else {
                latest[target.turnID] = target
                return
            }
            if target.attempt > current.attempt ||
                (target.attempt == current.attempt && target.id > current.id) {
                latest[target.turnID] = target
            }
        }
    }

    public static func recoveryActions(for errorCode: String?) -> [PrimaryTargetAction] {
        switch errorCode {
        case "seat_unready", "primary_agent_unready", "primary_agent_drift", "chat_policy_unavailable":
            return [.recheckAndRetry, .retry]
        case "daemon_interrupted", "provider_result_unknown", "provider_incomplete", "provider_failed":
            return [.retry]
        default:
            return []
        }
    }

    public static func presentation(
        for target: PrimaryTarget?,
        machine: String?
    ) -> PrimaryTargetPresentation? {
        guard let target else { return nil }
        switch target.state {
        case "answered":
            return nil
        case "queued":
            return PrimaryTargetPresentation(
                kind: .queued,
                title: "Starting Primary Agent…",
                action: .cancel,
                showsDetails: false
            )
        case "working":
            return PrimaryTargetPresentation(
                kind: .working,
                title: "Primary Agent is working",
                action: .cancel,
                showsDetails: false
            )
        case "canceled":
            return PrimaryTargetPresentation(
                kind: .canceled,
                title: "Canceled by you",
                showsDetails: false
            )
        default:
            break
        }

        let action = recoveryActions(for: target.errorCode).first
        switch target.errorCode {
        case "daemon_interrupted":
            return PrimaryTargetPresentation(
                kind: .interrupted,
                title: "Answer interrupted",
                body: "Fort kept your message. Retry uses the same saved Primary Agent.",
                action: action,
                showsDetails: true
            )
        case "primary_agent_drift":
            let savedMachine = machine.flatMap { $0.isEmpty ? nil : $0 } ?? "its saved computer"
            return PrimaryTargetPresentation(
                kind: .failed,
                title: "This didn’t start",
                body: "The saved Primary Agent on \(savedMachine) changed before Fort could begin. Fort kept your message.",
                action: action,
                showsDetails: true
            )
        case "primary_agent_unready", "seat_unready", "chat_policy_unavailable":
            return PrimaryTargetPresentation(
                kind: .failed,
                title: "This didn’t start",
                body: "The saved Primary Agent was not ready before Fort could begin. Fort kept your message.",
                action: action,
                showsDetails: true
            )
        default:
            return PrimaryTargetPresentation(
                kind: .failed,
                title: "Answer failed",
                body: "Fort couldn’t finish this answer. Fort kept your message.",
                action: action,
                showsDetails: true
            )
        }
    }
}
