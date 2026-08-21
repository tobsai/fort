//
//  AgentChannels.swift
//  FortKit
//
//  Provider-neutral wire models for the Spec 046 Agent Channel hierarchy.
//  Agent Channels are durable agent destinations; Conversations are separate
//  transcripts nested beneath exactly one immutable agent binding.
//

import Foundation

// MARK: - Immutable agent identity and authority

public struct AgentSeatIdentity: Codable, Sendable, Hashable, Identifiable {
    public let id: String
    public let profile: String
    public let agent: String
    public let model: String
    public let machine: String
}

public struct AgentAuthoritySnapshot: Codable, Sendable, Hashable {
    public let requestedModel: String
    public let resolvedModel: String
    public let authority: String
    public let policyID: String
    public let policyRevision: String
    public let adapterID: String
    public let adapterRevision: String
    public let runtimeContract: String
    public let sessionMode: String
    public let memoryMode: String
    public let executionPolicy: [String: String]

    enum CodingKeys: String, CodingKey {
        case requestedModel = "requested_model"
        case resolvedModel = "resolved_model"
        case authority
        case policyID = "policy_id"
        case policyRevision = "policy_revision"
        case adapterID = "adapter_id"
        case adapterRevision = "adapter_revision"
        case runtimeContract = "runtime_contract"
        case sessionMode = "session_mode"
        case memoryMode = "memory_mode"
        case executionPolicy = "execution_policy"
    }
}

public struct AgentBinding: Codable, Sendable, Hashable {
    public let seat: AgentSeatIdentity
    public let authority: AgentAuthoritySnapshot
}

public struct AgentOption: Codable, Sendable, Hashable, Identifiable {
    public var id: String { optionID }
    public let optionID: String
    public let state: String
    public let reason: String?
    public let displayName: String
    public let binding: AgentBinding

    enum CodingKeys: String, CodingKey {
        case optionID = "agent_option_id"
        case state, reason
        case displayName = "display_name"
        case binding
    }
}

// MARK: - Agent Channels and nested Conversations

public enum AgentChannelState: String, Codable, Sendable, CaseIterable {
    case open
    case archived
}

public enum AgentChannelFilter: String, Codable, Sendable, CaseIterable {
    case open
    case archived
    case all
}

public enum AgentConversationState: String, Codable, Sendable, CaseIterable {
    case open
    case archived
}

public enum AgentConversationFilter: String, Codable, Sendable, CaseIterable {
    case open
    case archived
    case all
}

public struct AgentChannel: Codable, Sendable, Hashable, Identifiable {
    public let id: String
    public let name: String
    public let state: AgentChannelState
    public let optionID: String
    public let binding: AgentBinding
    public let createdAt: String

    enum CodingKeys: String, CodingKey {
        case id, name, state, binding
        case optionID = "option_id"
        case createdAt = "created_at"
    }
}

// These are the shared durable conversation kernel already used by the legacy
// Primary client. Provider-neutral names keep new Agent Channel code free of
// Primary product semantics without creating a second decoder for one Go type.
public typealias AgentConversation = PrimaryConversation
public typealias AgentParticipant = PrimaryParticipant
public typealias AgentMessage = PrimaryMessage
public typealias AgentTurn = PrimaryTurn
public typealias AgentTarget = PrimaryTarget
public typealias AgentTurnResult = PrimaryTurnResult

public struct AgentConversationSummary: Codable, Sendable, Hashable, Identifiable {
    public var id: String { conversation.id }
    public let conversation: AgentConversation
    public let participant: AgentParticipant
    public let pinned: Bool
    public let pinnedAt: String?

    enum CodingKeys: String, CodingKey {
        case conversation, participant, pinned
        case pinnedAt = "pinned_at"
    }
}

public struct AgentChannelSummary: Codable, Sendable, Hashable, Identifiable {
    public var id: String { channel.id }
    public let channel: AgentChannel
    public let conversations: [AgentConversationSummary]
    public let readiness: PrimaryChannelReadiness
}

/// The Go port deliberately uses the same projection for list and detail.
public typealias AgentChannelDetail = AgentChannelSummary

public struct AgentConversationDetail: Codable, Sendable, Hashable, Identifiable {
    public var id: String { conversation.id }
    public let channelID: String
    public let conversation: AgentConversation
    public let participant: AgentParticipant
    public let messages: [AgentMessage]
    public let turns: [AgentTurn]
    public let targets: [AgentTarget]
    public let readiness: PrimaryChannelReadiness
    public let binding: AgentBinding
    public let pinned: Bool
    public let pinnedAt: String?

    enum CodingKeys: String, CodingKey {
        case channelID = "agent_channel_id"
        case conversation, participant, messages, turns, targets, readiness, binding, pinned
        case pinnedAt = "pinned_at"
    }
}

public struct AgentFirstTurnResult: Codable, Sendable, Hashable {
    public let conversation: AgentConversationDetail
    public let turn: AgentTurn
    public let targets: [AgentTarget]
}

public struct AgentNeedsYouItem: Codable, Sendable, Hashable, Identifiable {
    public var id: String { target.id }
    public let agentChannel: AgentChannel
    public let conversation: AgentConversation
    public let target: AgentTarget
    public let recoveryActions: [String]

    enum CodingKeys: String, CodingKey {
        case agentChannel = "agent_channel"
        case conversation, target
        case recoveryActions = "recovery_actions"
    }
}

public struct AgentTurnRequest: Codable, Sendable, Hashable {
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
