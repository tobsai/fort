import Foundation

#if canImport(FoundationNetworking)
import FoundationNetworking
#endif

public enum FortCloudAgentState: String, Decodable, Sendable, Hashable {
    case open
    case archived
}

public struct FortCloudAgent: Decodable, Sendable, Hashable, Identifiable {
    public let id: String
    public let accountID: String
    public let state: FortCloudAgentState
    public let canonicalConversationID: String

    enum CodingKeys: String, CodingKey {
        case id, state
        case accountID = "account_id"
        case canonicalConversationID = "canonical_conversation_id"
    }
}

public struct FortCloudAgentProfile: Decodable, Sendable, Hashable {
    public let id: String
    public let agentID: String
    public let name: String
    public let title: String?
    public let avatarURL: String?
    public let pinned: Bool

    enum CodingKeys: String, CodingKey {
        case id, name, title, pinned
        case agentID = "agent_id"
        case avatarURL = "avatar_url"
    }
}

public struct FortCloudAgentBinding: Decodable, Sendable, Hashable {
    public let id: String
    public let agentID: String
    public let behaviorRevisionID: String
    public let provider: String
    public let requestedModel: String
    public let resolvedModel: String
    public let computerID: String?
    public let cloudRuntime: String?
    public let adapterID: String
    public let adapterRevision: String

    enum CodingKeys: String, CodingKey {
        case id, provider
        case agentID = "agent_id"
        case behaviorRevisionID = "behavior_revision_id"
        case requestedModel = "requested_model"
        case resolvedModel = "resolved_model"
        case computerID = "computer_id"
        case cloudRuntime = "cloud_runtime"
        case adapterID = "adapter_id"
        case adapterRevision = "adapter_revision"
    }
}

public struct FortCloudExecutionSource: Decodable, Sendable, Hashable {
    public let id: String
    public let framework: String
    public let displayName: String

    enum CodingKeys: String, CodingKey {
        case id, framework
        case displayName = "display_name"
    }
}

public enum FortCloudConversationState: String, Decodable, Sendable, Hashable {
    case open
    case archived
}

public struct FortCloudConversation: Decodable, Sendable, Hashable, Identifiable {
    public let id: String
    public let title: String
    public let state: FortCloudConversationState
}

public struct FortCloudAgentRecord: Decodable, Sendable, Hashable, Identifiable {
    public var id: String { agent.id }
    public let agent: FortCloudAgent
    public let profile: FortCloudAgentProfile
    public let binding: FortCloudAgentBinding
    public let executionSource: FortCloudExecutionSource
    public let home: FortCloudConversation

    enum CodingKeys: String, CodingKey {
        case agent, profile, binding, home
        case executionSource = "execution_source"
    }

    public init(from decoder: Decoder) throws {
        let values = try decoder.container(keyedBy: CodingKeys.self)
        agent = try values.decode(FortCloudAgent.self, forKey: .agent)
        profile = try values.decode(FortCloudAgentProfile.self, forKey: .profile)
        binding = try values.decode(FortCloudAgentBinding.self, forKey: .binding)
        executionSource = try values.decode(FortCloudExecutionSource.self, forKey: .executionSource)
        home = try values.decode(FortCloudConversation.self, forKey: .home)
        guard !agent.id.isEmpty, profile.agentID == agent.id, binding.agentID == agent.id,
              agent.canonicalConversationID == home.id, home.state == .open
        else { throw FortCloudClientError.invalidProjection }
    }
}

public enum FortCloudConversationKind: String, Decodable, Sendable, Hashable {
    case canonical
    case secondary
}

public struct FortCloudAgentConversationLink: Decodable, Sendable, Hashable {
    public let agentID: String
    public let conversationID: String
    public let kind: FortCloudConversationKind

    enum CodingKeys: String, CodingKey {
        case kind
        case agentID = "agent_id"
        case conversationID = "conversation_id"
    }
}

public struct FortCloudAgentConversationRecord: Decodable, Sendable, Hashable, Identifiable {
    public var id: String { conversation.id }
    public let conversation: FortCloudConversation
    public let link: FortCloudAgentConversationLink
    public let pinned: Bool
    public let pinnedAt: String?

    enum CodingKeys: String, CodingKey {
        case conversation, link, pinned
        case pinnedAt = "pinned_at"
    }

    public init(from decoder: Decoder) throws {
        let values = try decoder.container(keyedBy: CodingKeys.self)
        conversation = try values.decode(FortCloudConversation.self, forKey: .conversation)
        link = try values.decode(FortCloudAgentConversationLink.self, forKey: .link)
        pinned = try values.decode(Bool.self, forKey: .pinned)
        pinnedAt = try values.decodeIfPresent(String.self, forKey: .pinnedAt)
        guard link.conversationID == conversation.id,
              link.kind != .canonical || (!pinned && conversation.state == .open),
              !pinned || pinnedAt?.isEmpty == false
        else { throw FortCloudClientError.invalidProjection }
    }
}

public struct FortCloudGroup: Decodable, Sendable, Hashable, Identifiable {
    public let id: String
    public let accountID: String
    public let conversationID: String
    public let state: FortCloudConversationState
    public let currentMembershipRevisionID: String

    enum CodingKeys: String, CodingKey {
        case id, state
        case accountID = "account_id"
        case conversationID = "conversation_id"
        case currentMembershipRevisionID = "current_membership_revision_id"
    }
}

public struct FortCloudGroupMember: Decodable, Sendable, Hashable {
    public let agentID: String
    public let position: Int

    enum CodingKeys: String, CodingKey {
        case position
        case agentID = "agent_id"
    }
}

public struct FortCloudGroupMembership: Decodable, Sendable, Hashable {
    public let id: String
    public let groupID: String
    public let revision: Int
    public let members: [FortCloudGroupMember]

    enum CodingKeys: String, CodingKey {
        case id, revision, members
        case groupID = "group_id"
    }
}

public struct FortCloudGroupMemberBinding: Decodable, Sendable, Hashable {
    public let agentID: String
    public let behaviorRevisionID: String
    public let bindingRevisionID: String
    public let participantID: String

    enum CodingKeys: String, CodingKey {
        case agentID = "agent_id"
        case behaviorRevisionID = "behavior_revision_id"
        case bindingRevisionID = "binding_revision_id"
        case participantID = "participant_id"
    }
}

public struct FortCloudGroupRecord: Decodable, Sendable, Hashable, Identifiable {
    public var id: String { group.id }
    public let group: FortCloudGroup
    public let conversation: FortCloudConversation
    public let membership: FortCloudGroupMembership
    public let memberBindings: [FortCloudGroupMemberBinding]

    enum CodingKeys: String, CodingKey {
        case group, conversation, membership
        case memberBindings = "member_bindings"
    }

    public init(from decoder: Decoder) throws {
        let values = try decoder.container(keyedBy: CodingKeys.self)
        group = try values.decode(FortCloudGroup.self, forKey: .group)
        conversation = try values.decode(FortCloudConversation.self, forKey: .conversation)
        membership = try values.decode(FortCloudGroupMembership.self, forKey: .membership)
        memberBindings = try values.decode([FortCloudGroupMemberBinding].self, forKey: .memberBindings)
        let ordered = membership.members.enumerated().allSatisfy { $0.offset == $0.element.position }
        let unique = Set(membership.members.map(\.agentID)).count == membership.members.count
        guard group.conversationID == conversation.id, group.state == conversation.state,
              group.currentMembershipRevisionID == membership.id, membership.groupID == group.id,
              (2...6).contains(membership.members.count), ordered, unique,
              memberBindings.map(\.agentID) == membership.members.map(\.agentID)
        else { throw FortCloudClientError.invalidProjection }
    }
}

public enum FortCloudGroupRecipientSelection: String, Codable, Sendable, Hashable {
    case explicit
    case everyone
}

public enum FortCloudGroupConcurrencyPolicy: String, Codable, Sendable, Hashable {
    case sequential
    case concurrent
}

public struct FortCloudGroupTurnEnvelope: Decodable, Sendable, Hashable, Identifiable {
    public let id: String
    public let groupID: String
    public let conversationID: String
    public let clientTurnID: String
    public let membershipRevisionID: String
    public let selection: FortCloudGroupRecipientSelection
    public let concurrencyPolicy: FortCloudGroupConcurrencyPolicy
    public let maxAgentMessages: Int
    public let maxHandoffDepth: Int
    public let deadline: String
    public let createdAt: String

    enum CodingKeys: String, CodingKey {
        case id, selection, deadline
        case groupID = "group_id"
        case conversationID = "conversation_id"
        case clientTurnID = "client_turn_id"
        case membershipRevisionID = "membership_revision_id"
        case concurrencyPolicy = "concurrency_policy"
        case maxAgentMessages = "max_agent_messages"
        case maxHandoffDepth = "max_handoff_depth"
        case createdAt = "created_at"
    }
}

public struct FortCloudGroupInitialTarget: Decodable, Sendable, Hashable, Identifiable {
    public let id: String
    public let groupTurnID: String
    public let agentID: String
    public let behaviorRevisionID: String
    public let bindingRevisionID: String
    public let participantID: String
    public let wave: Int
    public let state: String
    public let createdAt: String

    enum CodingKeys: String, CodingKey {
        case id, wave, state
        case groupTurnID = "group_turn_id"
        case agentID = "agent_id"
        case behaviorRevisionID = "behavior_revision_id"
        case bindingRevisionID = "binding_revision_id"
        case participantID = "participant_id"
        case createdAt = "created_at"
    }
}

public struct FortCloudGroupTurnRecord: Decodable, Sendable, Hashable, Identifiable {
    public var id: String { envelope.id }
    public let message: FortCloudConversationMessage
    public let envelope: FortCloudGroupTurnEnvelope
    public let recipients: [FortCloudGroupMemberBinding]
    public let initialTargets: [FortCloudGroupInitialTarget]

    enum CodingKeys: String, CodingKey {
        case message, envelope, recipients
        case initialTargets = "initial_targets"
    }

    public init(from decoder: Decoder) throws {
        let values = try decoder.container(keyedBy: CodingKeys.self)
        message = try values.decode(FortCloudConversationMessage.self, forKey: .message)
        envelope = try values.decode(FortCloudGroupTurnEnvelope.self, forKey: .envelope)
        recipients = try values.decode([FortCloudGroupMemberBinding].self, forKey: .recipients)
        initialTargets = try values.decode([FortCloudGroupInitialTarget].self, forKey: .initialTargets)
        let exactTargets = zip(recipients, initialTargets).allSatisfy { recipient, target in
            target.groupTurnID == envelope.id && target.wave == 0 &&
                target.agentID == recipient.agentID &&
                target.behaviorRevisionID == recipient.behaviorRevisionID &&
                target.bindingRevisionID == recipient.bindingRevisionID &&
                target.participantID == recipient.participantID
        }
        guard message.turnID == envelope.id,
              message.conversationID == envelope.conversationID,
              message.authorKind == "human", message.authorAgentID == nil,
              envelope.maxAgentMessages == 10, envelope.maxHandoffDepth == 3,
              !recipients.isEmpty, recipients.count == initialTargets.count,
              exactTargets
        else { throw FortCloudClientError.invalidProjection }
    }
}

public struct FortCloudGroupProjection: Decodable, Sendable, Hashable, Identifiable {
    public var id: String { group.id }
    public let group: FortCloudGroupRecord
    public let turns: [FortCloudGroupTurnRecord]
    public let messages: [FortCloudConversationMessage]

    enum CodingKeys: String, CodingKey { case group, turns, messages }

    public init(from decoder: Decoder) throws {
        let values = try decoder.container(keyedBy: CodingKeys.self)
        group = try values.decode(FortCloudGroupRecord.self, forKey: .group)
        turns = try values.decode([FortCloudGroupTurnRecord].self, forKey: .turns)
        messages = try values.decode([FortCloudConversationMessage].self, forKey: .messages)
        let memberBindings = Dictionary(uniqueKeysWithValues: group.memberBindings.map { ($0.agentID, $0) })
        let validTurns = turns.allSatisfy { turn in
            turn.envelope.groupID == group.group.id &&
                turn.envelope.conversationID == group.conversation.id &&
                turn.envelope.membershipRevisionID == group.membership.id &&
                turn.recipients.allSatisfy { recipient in
                    guard let current = memberBindings[recipient.agentID] else { return false }
                    return current == recipient
                }
        }
        var previousMessageID: Int64 = 0
        let validMessages = messages.allSatisfy { message in
            defer { previousMessageID = message.id }
            guard message.id > previousMessageID,
                  message.conversationID == group.conversation.id,
                  !message.authorID.isEmpty,
                  ["human", "agent", "system"].contains(message.authorKind)
            else { return false }
            if message.authorKind == "agent" {
                guard let agentID = message.authorAgentID else { return false }
                return memberBindings[agentID] != nil
            }
            return message.authorAgentID == nil
        }
        guard validTurns, validMessages else { throw FortCloudClientError.invalidProjection }
    }
}

public enum FortCloudHandoffActorKind: String, Codable, Sendable, Hashable {
    case human
    case agent
}

public enum FortCloudHandoffState: String, Codable, Sendable, Hashable {
    case queued
    case needsYou = "needs_you"
    case working
    case completed
    case failed
    case canceled
}

public enum FortCloudHandoffBudgetClass: String, Codable, Sendable, Hashable {
    case hard
    case unknown
}

public enum FortCloudContextReferenceKind: String, Codable, Sendable, Hashable {
    case message
    case contextArtifact = "context_artifact"
    case outputArtifact = "output_artifact"
}

public struct FortCloudAuthorityGrant: Codable, Sendable, Hashable, Identifiable {
    public let id: String
    public let permissions: [String]
    public let contextRecordIDs: [String]

    enum CodingKeys: String, CodingKey {
        case id, permissions
        case contextRecordIDs = "context_record_ids"
    }

    public init(from decoder: Decoder) throws {
        let values = try decoder.container(keyedBy: CodingKeys.self)
        id = try values.decode(String.self, forKey: .id)
        permissions = try values.decodeIfPresent([String].self, forKey: .permissions) ?? []
        contextRecordIDs = try values.decodeIfPresent([String].self, forKey: .contextRecordIDs) ?? []
        guard fortCloudIdentity(id), fortCloudUniqueNonempty(permissions),
              fortCloudUniqueNonempty(contextRecordIDs)
        else { throw FortCloudClientError.invalidProjection }
    }
}

public struct FortCloudContextReference: Codable, Sendable, Hashable, Identifiable {
    public var id: String { recordID }
    public let kind: FortCloudContextReferenceKind
    public let recordID: String
    public let accountID: String
    public let immutable: Bool
    public let finalized: Bool
    public let digest: String
    public let observedDigest: String
    public let size: Int64
    public let observedSize: Int64

    enum CodingKeys: String, CodingKey {
        case kind, immutable, finalized, digest, size
        case recordID = "id"
        case accountID = "account_id"
        case observedDigest = "observed_digest"
        case observedSize = "observed_size"
    }

    public init(from decoder: Decoder) throws {
        let values = try decoder.container(keyedBy: CodingKeys.self)
        kind = try values.decode(FortCloudContextReferenceKind.self, forKey: .kind)
        recordID = try values.decode(String.self, forKey: .recordID)
        accountID = try values.decode(String.self, forKey: .accountID)
        immutable = try values.decode(Bool.self, forKey: .immutable)
        finalized = try values.decodeIfPresent(Bool.self, forKey: .finalized) ?? false
        digest = try values.decodeIfPresent(String.self, forKey: .digest) ?? ""
        observedDigest = try values.decodeIfPresent(String.self, forKey: .observedDigest) ?? ""
        size = try values.decodeIfPresent(Int64.self, forKey: .size) ?? 0
        observedSize = try values.decodeIfPresent(Int64.self, forKey: .observedSize) ?? 0
        guard fortCloudIdentity(recordID), fortCloudIdentity(accountID), immutable else {
            throw FortCloudClientError.invalidProjection
        }
        if kind != .message {
            guard finalized, fortCloudSHA256(digest), observedDigest == digest,
                  size >= 0, observedSize == size
            else { throw FortCloudClientError.invalidProjection }
        }
    }
}

public struct FortCloudHandoffContextManifest: Codable, Sendable, Hashable {
    public let references: [FortCloudContextReference]
}

public struct FortCloudHandoff: Codable, Sendable, Hashable, Identifiable {
    public let id: String
    public let accountID: String
    public let idempotencyKey: String
    public let state: FortCloudHandoffState
    public let createdByKind: FortCloudHandoffActorKind
    public let createdByID: String
    public let groupTurnID: String?
    public let sourceMessageID: String
    public let sourceAgentID: String?
    public let sourceBehaviorRevisionID: String?
    public let sourceBindingRevisionID: String?
    public let recipientAgentID: String
    public let recipientBehaviorRevisionID: String
    public let recipientBindingRevisionID: String
    public let sourceConversationID: String
    public let outputConversationID: String
    public let context: FortCloudHandoffContextManifest
    public let requestedResult: String
    public let replyToMessageID: String?
    public let rootDelegationGrant: FortCloudAuthorityGrant
    public let parentStageAuthority: FortCloudAuthorityGrant?
    public let handoffPolicy: FortCloudAuthorityGrant
    public let recipientBindingPolicy: FortCloudAuthorityGrant
    public let emitterRequest: FortCloudAuthorityGrant?
    public let approvalRequired: Bool
    public let approvalReceipt: FortCloudAuthorityGrant?
    public let requestedAuthority: [String]
    public let effectiveAuthority: FortCloudAuthorityGrant
    public let structuredEmitterID: String?
    public let budgetClass: FortCloudHandoffBudgetClass
    public let budgetLimitEvidenceID: String?
    public let maxAgentMessages: Int
    public let maxDepth: Int
    public let depth: Int
    public let deadline: String
    public let ancestorAgentIDs: [String]
    public let parentHandoffID: String?
    public let createdAt: String

    enum CodingKeys: String, CodingKey {
        case id, state, context, depth, deadline
        case accountID = "account_id"
        case idempotencyKey = "idempotency_key"
        case createdByKind = "created_by_kind"
        case createdByID = "created_by_id"
        case groupTurnID = "group_turn_id"
        case sourceMessageID = "source_message_id"
        case sourceAgentID = "source_agent_id"
        case sourceBehaviorRevisionID = "source_behavior_revision_id"
        case sourceBindingRevisionID = "source_binding_revision_id"
        case recipientAgentID = "recipient_agent_id"
        case recipientBehaviorRevisionID = "recipient_behavior_revision_id"
        case recipientBindingRevisionID = "recipient_binding_revision_id"
        case sourceConversationID = "source_conversation_id"
        case outputConversationID = "output_conversation_id"
        case requestedResult = "requested_result"
        case replyToMessageID = "reply_to_message_id"
        case rootDelegationGrant = "root_delegation_grant"
        case parentStageAuthority = "parent_stage_authority"
        case handoffPolicy = "handoff_policy"
        case recipientBindingPolicy = "recipient_binding_policy"
        case emitterRequest = "emitter_request"
        case approvalRequired = "approval_required"
        case approvalReceipt = "approval_receipt"
        case requestedAuthority = "requested_authority"
        case effectiveAuthority = "effective_authority"
        case structuredEmitterID = "structured_emitter_id"
        case budgetClass = "budget_class"
        case budgetLimitEvidenceID = "budget_limit_evidence_id"
        case maxAgentMessages = "max_agent_messages"
        case maxDepth = "max_depth"
        case ancestorAgentIDs = "ancestor_agent_ids"
        case parentHandoffID = "parent_handoff_id"
        case createdAt = "created_at"
    }

    public init(from decoder: Decoder) throws {
        let values = try decoder.container(keyedBy: CodingKeys.self)
        id = try values.decode(String.self, forKey: .id)
        accountID = try values.decode(String.self, forKey: .accountID)
        idempotencyKey = try values.decode(String.self, forKey: .idempotencyKey)
        state = try values.decode(FortCloudHandoffState.self, forKey: .state)
        createdByKind = try values.decode(FortCloudHandoffActorKind.self, forKey: .createdByKind)
        createdByID = try values.decode(String.self, forKey: .createdByID)
        groupTurnID = try values.decodeIfPresent(String.self, forKey: .groupTurnID)
        sourceMessageID = try values.decode(String.self, forKey: .sourceMessageID)
        sourceAgentID = try values.decodeIfPresent(String.self, forKey: .sourceAgentID)
        sourceBehaviorRevisionID = try values.decodeIfPresent(String.self, forKey: .sourceBehaviorRevisionID)
        sourceBindingRevisionID = try values.decodeIfPresent(String.self, forKey: .sourceBindingRevisionID)
        recipientAgentID = try values.decode(String.self, forKey: .recipientAgentID)
        recipientBehaviorRevisionID = try values.decode(String.self, forKey: .recipientBehaviorRevisionID)
        recipientBindingRevisionID = try values.decode(String.self, forKey: .recipientBindingRevisionID)
        sourceConversationID = try values.decode(String.self, forKey: .sourceConversationID)
        outputConversationID = try values.decode(String.self, forKey: .outputConversationID)
        context = try values.decode(FortCloudHandoffContextManifest.self, forKey: .context)
        requestedResult = try values.decode(String.self, forKey: .requestedResult)
        replyToMessageID = try values.decodeIfPresent(String.self, forKey: .replyToMessageID)
        rootDelegationGrant = try values.decode(FortCloudAuthorityGrant.self, forKey: .rootDelegationGrant)
        parentStageAuthority = try values.decodeIfPresent(FortCloudAuthorityGrant.self, forKey: .parentStageAuthority)
        handoffPolicy = try values.decode(FortCloudAuthorityGrant.self, forKey: .handoffPolicy)
        recipientBindingPolicy = try values.decode(FortCloudAuthorityGrant.self, forKey: .recipientBindingPolicy)
        emitterRequest = try values.decodeIfPresent(FortCloudAuthorityGrant.self, forKey: .emitterRequest)
        approvalRequired = try values.decode(Bool.self, forKey: .approvalRequired)
        approvalReceipt = try values.decodeIfPresent(FortCloudAuthorityGrant.self, forKey: .approvalReceipt)
        requestedAuthority = try values.decodeIfPresent([String].self, forKey: .requestedAuthority) ?? []
        effectiveAuthority = try values.decode(FortCloudAuthorityGrant.self, forKey: .effectiveAuthority)
        structuredEmitterID = try values.decodeIfPresent(String.self, forKey: .structuredEmitterID)
        budgetClass = try values.decode(FortCloudHandoffBudgetClass.self, forKey: .budgetClass)
        budgetLimitEvidenceID = try values.decodeIfPresent(String.self, forKey: .budgetLimitEvidenceID)
        maxAgentMessages = try values.decode(Int.self, forKey: .maxAgentMessages)
        maxDepth = try values.decode(Int.self, forKey: .maxDepth)
        depth = try values.decode(Int.self, forKey: .depth)
        deadline = try values.decode(String.self, forKey: .deadline)
        ancestorAgentIDs = try values.decodeIfPresent([String].self, forKey: .ancestorAgentIDs) ?? []
        parentHandoffID = try values.decodeIfPresent(String.self, forKey: .parentHandoffID)
        createdAt = try values.decode(String.self, forKey: .createdAt)

        let requiredIDs = [id, accountID, createdByID, sourceMessageID, recipientAgentID,
                           recipientBehaviorRevisionID, recipientBindingRevisionID,
                           sourceConversationID, outputConversationID]
        let contextIsValid = context.references.count <= 64 && context.references.allSatisfy {
            $0.accountID == accountID && rootDelegationGrant.contextRecordIDs.contains("\($0.kind.rawValue):\($0.recordID)")
        }
        let uniqueContext = Set(context.references.map { "\($0.kind.rawValue):\($0.recordID)" }).count == context.references.count
        let created = fortCloudTimestamp(createdAt)
        let due = fortCloudTimestamp(deadline)
        let optionalIDs = [groupTurnID, sourceAgentID, sourceBehaviorRevisionID, sourceBindingRevisionID,
                           replyToMessageID, structuredEmitterID, budgetLimitEvidenceID, parentHandoffID]
        guard requiredIDs.allSatisfy(fortCloudIdentity), fortCloudIdempotencyKey(idempotencyKey),
              fortCloudNonemptyText(requestedResult, maximumBytes: 2 * 1024 * 1024),
              optionalIDs.allSatisfy({ $0 == nil || fortCloudIdentity($0!) }),
              maxAgentMessages == 10, maxDepth == 3, (1...3).contains(depth),
              (depth == 1) == (parentHandoffID == nil),
              groupTurnID == nil || outputConversationID == sourceConversationID,
              sourceAgentID == nil || sourceAgentID != recipientAgentID,
              created != nil, due != nil, due! > created!, contextIsValid, uniqueContext,
              fortCloudUniqueNonempty(requestedAuthority), fortCloudUniqueNonempty(ancestorAgentIDs),
              !ancestorAgentIDs.contains(recipientAgentID),
              budgetClass != .hard || budgetLimitEvidenceID != nil
        else { throw FortCloudClientError.invalidProjection }
        if createdByKind == .agent {
            guard sourceAgentID == createdByID, sourceBehaviorRevisionID != nil,
                  sourceBindingRevisionID != nil, structuredEmitterID != nil,
                  emitterRequest != nil, parentStageAuthority != nil,
                  ancestorAgentIDs.last == sourceAgentID
            else { throw FortCloudClientError.invalidProjection }
        }
    }
}

public enum FortCloudHandoffTargetState: String, Codable, Sendable, Hashable {
    case queued
    case working
    case answered
    case failed
    case canceled
}

public struct FortCloudHandoffTarget: Codable, Sendable, Hashable, Identifiable {
    public let id: String
    public let handoffID: String
    public let conversationID: String
    public let agentID: String
    public let behaviorRevisionID: String
    public let bindingRevisionID: String
    public let participantID: String
    public let state: FortCloudHandoffTargetState
    public let createdAt: String

    enum CodingKeys: String, CodingKey {
        case id, state
        case handoffID = "handoff_id"
        case conversationID = "conversation_id"
        case agentID = "agent_id"
        case behaviorRevisionID = "behavior_revision_id"
        case bindingRevisionID = "binding_revision_id"
        case participantID = "participant_id"
        case createdAt = "created_at"
    }

    public init(from decoder: Decoder) throws {
        let values = try decoder.container(keyedBy: CodingKeys.self)
        id = try values.decode(String.self, forKey: .id)
        handoffID = try values.decode(String.self, forKey: .handoffID)
        conversationID = try values.decode(String.self, forKey: .conversationID)
        agentID = try values.decode(String.self, forKey: .agentID)
        behaviorRevisionID = try values.decode(String.self, forKey: .behaviorRevisionID)
        bindingRevisionID = try values.decode(String.self, forKey: .bindingRevisionID)
        participantID = try values.decode(String.self, forKey: .participantID)
        state = try values.decode(FortCloudHandoffTargetState.self, forKey: .state)
        createdAt = try values.decode(String.self, forKey: .createdAt)
        guard [id, handoffID, conversationID, agentID, behaviorRevisionID, bindingRevisionID, participantID]
            .allSatisfy(fortCloudIdentity), fortCloudTimestamp(createdAt) != nil
        else { throw FortCloudClientError.invalidProjection }
    }
}

public enum FortCloudHandoffAttemptState: String, Codable, Sendable, Hashable {
    case working
    case completed
    case failed
    case canceled
}

public struct FortCloudHandoffAttempt: Codable, Sendable, Hashable, Identifiable {
    public let id: String
    public let handoffID: String
    public let leaseID: String
    public let machineID: String
    public let fenceToken: String
    public let state: FortCloudHandoffAttemptState
    public let startedAt: String
    public let leaseExpiresAt: String
    public let terminalReceiptID: String?
    public let completedAt: String?

    enum CodingKeys: String, CodingKey {
        case id, state
        case handoffID = "handoff_id"
        case leaseID = "lease_id"
        case machineID = "machine_id"
        case fenceToken = "fence_token"
        case startedAt = "started_at"
        case leaseExpiresAt = "lease_expires_at"
        case terminalReceiptID = "terminal_receipt_id"
        case completedAt = "completed_at"
    }

    public init(from decoder: Decoder) throws {
        let values = try decoder.container(keyedBy: CodingKeys.self)
        id = try values.decode(String.self, forKey: .id)
        handoffID = try values.decode(String.self, forKey: .handoffID)
        leaseID = try values.decode(String.self, forKey: .leaseID)
        machineID = try values.decode(String.self, forKey: .machineID)
        fenceToken = try values.decode(String.self, forKey: .fenceToken)
        state = try values.decode(FortCloudHandoffAttemptState.self, forKey: .state)
        startedAt = try values.decode(String.self, forKey: .startedAt)
        leaseExpiresAt = try values.decode(String.self, forKey: .leaseExpiresAt)
        terminalReceiptID = try values.decodeIfPresent(String.self, forKey: .terminalReceiptID)
        let rawCompletedAt = try values.decodeIfPresent(String.self, forKey: .completedAt)
        completedAt = rawCompletedAt == "0001-01-01T00:00:00Z" ? nil : rawCompletedAt
        let started = fortCloudTimestamp(startedAt)
        let expires = fortCloudTimestamp(leaseExpiresAt)
        let completed = completedAt.flatMap(fortCloudTimestamp)
        guard [id, handoffID, leaseID, machineID, fenceToken].allSatisfy(fortCloudIdentity),
              started != nil, expires != nil, expires! > started!,
              terminalReceiptID == nil || fortCloudIdentity(terminalReceiptID!),
              completedAt == nil || (completed != nil && completed! >= started!),
              state == .working || (terminalReceiptID != nil && completedAt != nil)
        else { throw FortCloudClientError.invalidProjection }
    }
}

public enum FortCloudHandoffCancellationState: String, Codable, Sendable, Hashable {
    case requested
    case canceled
}

public struct FortCloudHandoffCancellation: Codable, Sendable, Hashable {
    public let handoffID: String
    public let targetID: String
    public let agentID: String
    public let behaviorRevisionID: String
    public let bindingRevisionID: String
    public let participantID: String
    public let state: FortCloudHandoffCancellationState
    public let requestedBy: String
    public let requestedAt: String

    enum CodingKeys: String, CodingKey {
        case state
        case handoffID = "handoff_id"
        case targetID = "target_id"
        case agentID = "agent_id"
        case behaviorRevisionID = "behavior_revision_id"
        case bindingRevisionID = "binding_revision_id"
        case participantID = "participant_id"
        case requestedBy = "requested_by"
        case requestedAt = "requested_at"
    }
}

public struct FortCloudHandoffProjection: Codable, Sendable, Hashable {
    public let handoffID: String
    public let conversationID: String
    public let outputConversationID: String
    public let authoritativeMessageID: String
    public let state: FortCloudHandoffState

    enum CodingKeys: String, CodingKey {
        case state
        case handoffID = "handoff_id"
        case conversationID = "conversation_id"
        case outputConversationID = "output_conversation_id"
        case authoritativeMessageID = "authoritative_message_id"
    }
}

public struct FortCloudHandoffResult: Codable, Sendable, Hashable {
    public let handoffID: String
    public let outputConversationID: String
    public let messageID: String
    public let body: String

    enum CodingKeys: String, CodingKey {
        case body
        case handoffID = "handoff_id"
        case outputConversationID = "output_conversation_id"
        case messageID = "message_id"
    }
}

public struct FortCloudHandoffRecord: Codable, Sendable, Hashable, Identifiable {
    public var id: String { handoff.id }
    public let handoff: FortCloudHandoff
    public let target: FortCloudHandoffTarget
    public let attempt: FortCloudHandoffAttempt?
    public let cancellation: FortCloudHandoffCancellation?
    public let projections: [FortCloudHandoffProjection]
    public let result: FortCloudHandoffResult?

    enum CodingKeys: String, CodingKey {
        case handoff, target, attempt, cancellation, projections, result
    }

    public init(from decoder: Decoder) throws {
        let values = try decoder.container(keyedBy: CodingKeys.self)
        handoff = try values.decode(FortCloudHandoff.self, forKey: .handoff)
        target = try values.decode(FortCloudHandoffTarget.self, forKey: .target)
        attempt = try values.decodeIfPresent(FortCloudHandoffAttempt.self, forKey: .attempt)
        cancellation = try values.decodeIfPresent(FortCloudHandoffCancellation.self, forKey: .cancellation)
        projections = try values.decode([FortCloudHandoffProjection].self, forKey: .projections)
        result = try values.decodeIfPresent(FortCloudHandoffResult.self, forKey: .result)

        let targetMatches = target.handoffID == handoff.id &&
            target.conversationID == handoff.outputConversationID &&
            target.agentID == handoff.recipientAgentID &&
            target.behaviorRevisionID == handoff.recipientBehaviorRevisionID &&
            target.bindingRevisionID == handoff.recipientBindingRevisionID
        let projectionIDs = projections.map(\.conversationID)
        let projectionsMatch = Set(projectionIDs).count == projectionIDs.count && projections.allSatisfy {
            $0.handoffID == handoff.id && $0.outputConversationID == handoff.outputConversationID &&
                $0.conversationID != handoff.outputConversationID && $0.state == handoff.state &&
                fortCloudIdentity($0.conversationID)
        }
        guard targetMatches, attempt?.handoffID == nil || attempt?.handoffID == handoff.id,
              cancellation?.handoffID == nil || cancellation?.handoffID == handoff.id,
              cancellation == nil || (cancellation!.targetID == target.id &&
                  cancellation!.agentID == target.agentID &&
                  cancellation!.behaviorRevisionID == target.behaviorRevisionID &&
                  cancellation!.bindingRevisionID == target.bindingRevisionID &&
                  cancellation!.participantID == target.participantID &&
                  fortCloudIdentity(cancellation!.requestedBy) &&
                  fortCloudTimestamp(cancellation!.requestedAt) != nil),
              projectionsMatch
        else { throw FortCloudClientError.invalidProjection }

        if handoff.state == .completed {
            guard target.state == .answered, let result,
                  result.handoffID == handoff.id,
                  result.outputConversationID == handoff.outputConversationID,
                  fortCloudIdentity(result.messageID),
                  fortCloudNonemptyText(result.body, maximumBytes: 2 * 1024 * 1024),
                  projections.allSatisfy({ $0.authoritativeMessageID == result.messageID })
            else { throw FortCloudClientError.invalidProjection }
        } else {
            guard result == nil, projections.allSatisfy({ $0.authoritativeMessageID.isEmpty }) else {
                throw FortCloudClientError.invalidProjection
            }
        }
    }
}

public enum FortCloudRoutineState: String, Codable, Sendable, Hashable {
    case active
    case paused
    case archived
}

public enum FortCloudRoutineAuthority: String, Codable, Sendable, Hashable {
    case fortCloud = "fort_cloud"
}

public enum FortCloudRoutineTrigger: String, Codable, Sendable, Hashable {
    case schedule
    case event
}

public enum FortCloudRoutineMissingInputBehavior: String, Codable, Sendable, Hashable {
    case skip
    case needsYou = "needs_you"
    case fail
}

public enum FortCloudRoutinePauseReason: String, Codable, Sendable, Hashable {
    case needsRevalidation = "needs_revalidation"
}

public struct FortCloudRoutine: Codable, Sendable, Hashable, Identifiable {
    public let id: String
    public let accountID: String
    public let agentID: String
    public let currentRevisionID: String
    public let state: FortCloudRoutineState
    public let createdAt: String

    enum CodingKeys: String, CodingKey {
        case id, state
        case accountID = "account_id"
        case agentID = "agent_id"
        case currentRevisionID = "current_revision_id"
        case createdAt = "created_at"
    }

    public init(from decoder: Decoder) throws {
        let values = try decoder.container(keyedBy: CodingKeys.self)
        id = try values.decode(String.self, forKey: .id)
        accountID = try values.decode(String.self, forKey: .accountID)
        agentID = try values.decode(String.self, forKey: .agentID)
        currentRevisionID = try values.decode(String.self, forKey: .currentRevisionID)
        state = try values.decode(FortCloudRoutineState.self, forKey: .state)
        createdAt = try values.decode(String.self, forKey: .createdAt)
        guard [id, accountID, agentID, currentRevisionID].allSatisfy(fortCloudIdentity),
              fortCloudTimestamp(createdAt) != nil
        else { throw FortCloudClientError.invalidProjection }
    }
}

public struct FortCloudRoutineRevision: Codable, Sendable, Hashable, Identifiable {
    public let id: String
    public let routineID: String
    public let revision: Int
    public let agentID: String
    public let behaviorRevisionID: String
    public let bindingRevisionID: String
    public let authority: FortCloudRoutineAuthority
    public let trigger: FortCloudRoutineTrigger
    public let schedule: String?
    public let timezone: String?
    public let nextOccurrence: String?
    public let inputSource: String
    public let freshnessSeconds: Int64
    public let expectedResult: String
    public let resultConversationID: String
    public let approvalBoundary: String
    public let missingInputBehavior: FortCloudRoutineMissingInputBehavior
    public let retryPolicy: String
    public let catchUpPolicy: String
    public let latenessPolicy: String
    public let createdAt: String

    enum CodingKeys: String, CodingKey {
        case id, revision, authority, trigger, schedule, timezone
        case routineID = "routine_id"
        case agentID = "agent_id"
        case behaviorRevisionID = "behavior_revision_id"
        case bindingRevisionID = "binding_revision_id"
        case nextOccurrence = "next_occurrence"
        case inputSource = "input_source"
        case freshnessSeconds = "freshness_seconds"
        case expectedResult = "expected_result"
        case resultConversationID = "result_conversation_id"
        case approvalBoundary = "approval_boundary"
        case missingInputBehavior = "missing_input_behavior"
        case retryPolicy = "retry_policy"
        case catchUpPolicy = "catch_up_policy"
        case latenessPolicy = "lateness_policy"
        case createdAt = "created_at"
    }

    public init(from decoder: Decoder) throws {
        let values = try decoder.container(keyedBy: CodingKeys.self)
        id = try values.decode(String.self, forKey: .id)
        routineID = try values.decode(String.self, forKey: .routineID)
        revision = try values.decode(Int.self, forKey: .revision)
        agentID = try values.decode(String.self, forKey: .agentID)
        behaviorRevisionID = try values.decode(String.self, forKey: .behaviorRevisionID)
        bindingRevisionID = try values.decode(String.self, forKey: .bindingRevisionID)
        authority = try values.decode(FortCloudRoutineAuthority.self, forKey: .authority)
        trigger = try values.decode(FortCloudRoutineTrigger.self, forKey: .trigger)
        schedule = try values.decodeIfPresent(String.self, forKey: .schedule)
        timezone = try values.decodeIfPresent(String.self, forKey: .timezone)
        nextOccurrence = try values.decodeIfPresent(String.self, forKey: .nextOccurrence)
        inputSource = try values.decode(String.self, forKey: .inputSource)
        freshnessSeconds = try values.decode(Int64.self, forKey: .freshnessSeconds)
        expectedResult = try values.decode(String.self, forKey: .expectedResult)
        resultConversationID = try values.decode(String.self, forKey: .resultConversationID)
        approvalBoundary = try values.decode(String.self, forKey: .approvalBoundary)
        missingInputBehavior = try values.decode(FortCloudRoutineMissingInputBehavior.self, forKey: .missingInputBehavior)
        retryPolicy = try values.decode(String.self, forKey: .retryPolicy)
        catchUpPolicy = try values.decode(String.self, forKey: .catchUpPolicy)
        latenessPolicy = try values.decode(String.self, forKey: .latenessPolicy)
        createdAt = try values.decode(String.self, forKey: .createdAt)

        let identifiers = [id, routineID, agentID, behaviorRevisionID, bindingRevisionID, resultConversationID]
        let intent = [inputSource, expectedResult, approvalBoundary, retryPolicy, catchUpPolicy, latenessPolicy]
        guard identifiers.allSatisfy(fortCloudIdentity), revision > 0,
              intent.allSatisfy({ fortCloudRoutineIntent($0, maximumBytes: 4_096) }),
              freshnessSeconds > 0, freshnessSeconds <= 365 * 24 * 60 * 60,
              fortCloudTimestamp(createdAt) != nil
        else { throw FortCloudClientError.invalidProjection }

        switch trigger {
        case .schedule:
            guard let schedule, let timezone, let nextOccurrence,
                  fortCloudRoutineIntent(schedule, maximumBytes: 512),
                  fortCloudRoutineIntent(timezone, maximumBytes: 128),
                  fortCloudTimestamp(nextOccurrence) != nil
            else { throw FortCloudClientError.invalidProjection }
        case .event:
            guard schedule == nil || schedule?.isEmpty == true,
                  timezone == nil || timezone?.isEmpty == true,
                  nextOccurrence == nil || nextOccurrence == "0001-01-01T00:00:00Z"
            else { throw FortCloudClientError.invalidProjection }
        }
    }
}

public struct FortCloudRoutineRecord: Codable, Sendable, Hashable, Identifiable {
    public var id: String { routine.id }
    public let routine: FortCloudRoutine
    public let currentRevision: FortCloudRoutineRevision
    public let pauseReason: FortCloudRoutinePauseReason?

    enum CodingKeys: String, CodingKey {
        case routine
        case currentRevision = "current_revision"
        case pauseReason = "pause_reason"
    }

    public init(from decoder: Decoder) throws {
        let values = try decoder.container(keyedBy: CodingKeys.self)
        routine = try values.decode(FortCloudRoutine.self, forKey: .routine)
        currentRevision = try values.decode(FortCloudRoutineRevision.self, forKey: .currentRevision)
        pauseReason = try values.decodeIfPresent(FortCloudRoutinePauseReason.self, forKey: .pauseReason)
        guard routine.currentRevisionID == currentRevision.id,
              routine.id == currentRevision.routineID,
              routine.agentID == currentRevision.agentID,
              (routine.state == .paused) == (pauseReason == .needsRevalidation)
        else { throw FortCloudClientError.invalidProjection }
    }
}

public enum FortCloudRoutineRunKind: String, Codable, Sendable, Hashable {
    case scheduled
    case test
}

public enum FortCloudRoutineRunState: String, Codable, Sendable, Hashable {
    case queued
    case working
    case needsYou = "needs_you"
    case succeeded
    case failed
    case canceled
}

public struct FortCloudRoutineOccurrence: Codable, Sendable, Hashable, Identifiable {
    public let id: String
    public let accountID: String
    public let routineID: String
    public let routineRevisionID: String
    public let kind: FortCloudRoutineRunKind
    public let state: FortCloudRoutineRunState
    public let scheduledFor: String
    public let idempotencyKey: String
    public let approvalEvidenceID: String
    public let createdAt: String
    public let updatedAt: String

    enum CodingKeys: String, CodingKey {
        case id, kind, state
        case accountID = "account_id"
        case routineID = "routine_id"
        case routineRevisionID = "routine_revision_id"
        case scheduledFor = "scheduled_for"
        case idempotencyKey = "idempotency_key"
        case approvalEvidenceID = "approval_evidence_id"
        case createdAt = "created_at"
        case updatedAt = "updated_at"
    }

    public init(from decoder: Decoder) throws {
        let values = try decoder.container(keyedBy: CodingKeys.self)
        id = try values.decode(String.self, forKey: .id)
        accountID = try values.decode(String.self, forKey: .accountID)
        routineID = try values.decode(String.self, forKey: .routineID)
        routineRevisionID = try values.decode(String.self, forKey: .routineRevisionID)
        kind = try values.decode(FortCloudRoutineRunKind.self, forKey: .kind)
        state = try values.decode(FortCloudRoutineRunState.self, forKey: .state)
        scheduledFor = try values.decode(String.self, forKey: .scheduledFor)
        idempotencyKey = try values.decode(String.self, forKey: .idempotencyKey)
        approvalEvidenceID = try values.decode(String.self, forKey: .approvalEvidenceID)
        createdAt = try values.decode(String.self, forKey: .createdAt)
        updatedAt = try values.decode(String.self, forKey: .updatedAt)
        guard [id, accountID, routineID, routineRevisionID, approvalEvidenceID].allSatisfy(fortCloudIdentity),
              fortCloudIdempotencyKey(idempotencyKey), fortCloudTimestamp(scheduledFor) != nil,
              fortCloudTimestamp(createdAt) != nil, fortCloudTimestamp(updatedAt) != nil
        else { throw FortCloudClientError.invalidProjection }
    }
}

public struct FortCloudRoutineRun: Codable, Sendable, Hashable, Identifiable {
    public let id: String
    public let routineID: String
    public let routineRevisionID: String
    public let agentID: String
    public let behaviorRevisionID: String
    public let bindingRevisionID: String
    public let occurrenceID: String
    public let kind: FortCloudRoutineRunKind
    public let state: FortCloudRoutineRunState
    public let normalizedResult: String?
    public let resultMessageID: String?
    public let createdAt: String

    enum CodingKeys: String, CodingKey {
        case id, kind, state
        case routineID = "routine_id"
        case routineRevisionID = "routine_revision_id"
        case agentID = "agent_id"
        case behaviorRevisionID = "behavior_revision_id"
        case bindingRevisionID = "binding_revision_id"
        case occurrenceID = "occurrence_id"
        case normalizedResult = "normalized_result"
        case resultMessageID = "result_message_id"
        case createdAt = "created_at"
    }

    public init(from decoder: Decoder) throws {
        let values = try decoder.container(keyedBy: CodingKeys.self)
        id = try values.decode(String.self, forKey: .id)
        routineID = try values.decode(String.self, forKey: .routineID)
        routineRevisionID = try values.decode(String.self, forKey: .routineRevisionID)
        agentID = try values.decode(String.self, forKey: .agentID)
        behaviorRevisionID = try values.decode(String.self, forKey: .behaviorRevisionID)
        bindingRevisionID = try values.decode(String.self, forKey: .bindingRevisionID)
        occurrenceID = try values.decode(String.self, forKey: .occurrenceID)
        kind = try values.decode(FortCloudRoutineRunKind.self, forKey: .kind)
        state = try values.decode(FortCloudRoutineRunState.self, forKey: .state)
        normalizedResult = try values.decodeIfPresent(String.self, forKey: .normalizedResult)
        resultMessageID = try values.decodeIfPresent(String.self, forKey: .resultMessageID)
        createdAt = try values.decode(String.self, forKey: .createdAt)
        let identities = [id, routineID, routineRevisionID, agentID, behaviorRevisionID, bindingRevisionID, occurrenceID]
        guard identities.allSatisfy(fortCloudIdentity), fortCloudTimestamp(createdAt) != nil,
              state == .succeeded
                ? (normalizedResult.map { fortCloudNonemptyText($0, maximumBytes: 2 * 1024 * 1024) } == true &&
                    resultMessageID.map(fortCloudIdentity) == true)
                : (normalizedResult == nil && resultMessageID == nil)
        else { throw FortCloudClientError.invalidProjection }
    }
}

public struct FortCloudRoutineRunActivity: Codable, Sendable, Hashable {
    public let sequence: Int64
    public let state: FortCloudRoutineRunState
    public let attemptID: String?
    public let leaseID: String?
    public let leaseExpiresAt: String?
    public let activity: String
    public let failureCode: String?
    public let nextAction: String?
    public let createdAt: String

    enum CodingKeys: String, CodingKey {
        case sequence, state, activity
        case attemptID = "attempt_id"
        case leaseID = "lease_id"
        case leaseExpiresAt = "lease_expires_at"
        case failureCode = "failure_code"
        case nextAction = "next_action"
        case createdAt = "created_at"
    }

    public init(from decoder: Decoder) throws {
        let values = try decoder.container(keyedBy: CodingKeys.self)
        sequence = try values.decode(Int64.self, forKey: .sequence)
        state = try values.decode(FortCloudRoutineRunState.self, forKey: .state)
        attemptID = try values.decodeIfPresent(String.self, forKey: .attemptID)
        leaseID = try values.decodeIfPresent(String.self, forKey: .leaseID)
        let rawLeaseExpiresAt = try values.decodeIfPresent(String.self, forKey: .leaseExpiresAt)
        leaseExpiresAt = rawLeaseExpiresAt == "0001-01-01T00:00:00Z" ? nil : rawLeaseExpiresAt
        activity = try values.decode(String.self, forKey: .activity)
        failureCode = try values.decodeIfPresent(String.self, forKey: .failureCode)
        nextAction = try values.decodeIfPresent(String.self, forKey: .nextAction)
        createdAt = try values.decode(String.self, forKey: .createdAt)
        let optionalIDs = [attemptID, leaseID]
        guard sequence > 0, optionalIDs.allSatisfy({ $0 == nil || fortCloudIdentity($0!) }),
              leaseExpiresAt == nil || fortCloudTimestamp(leaseExpiresAt!) != nil,
              fortCloudNonemptyText(activity, maximumBytes: 4_096),
              failureCode == nil || fortCloudRoutineIntent(failureCode!, maximumBytes: 512),
              nextAction == nil || fortCloudNonemptyText(nextAction!, maximumBytes: 4_096),
              fortCloudTimestamp(createdAt) != nil
        else { throw FortCloudClientError.invalidProjection }
    }
}

public struct FortCloudRoutineRunRecord: Codable, Sendable, Hashable, Identifiable {
    public var id: String { run.id }
    public let occurrence: FortCloudRoutineOccurrence
    public let run: FortCloudRoutineRun
    public let resultConversationID: String
    public let attemptID: String?
    public let leaseID: String?
    public let leaseExpiresAt: String?
    public let failureCode: String?
    public let nextAction: String?
    public let activities: [FortCloudRoutineRunActivity]

    enum CodingKeys: String, CodingKey {
        case occurrence, run, activities
        case resultConversationID = "result_conversation_id"
        case attemptID = "attempt_id"
        case leaseID = "lease_id"
        case leaseExpiresAt = "lease_expires_at"
        case failureCode = "failure_code"
        case nextAction = "next_action"
    }

    public init(from decoder: Decoder) throws {
        let values = try decoder.container(keyedBy: CodingKeys.self)
        occurrence = try values.decode(FortCloudRoutineOccurrence.self, forKey: .occurrence)
        run = try values.decode(FortCloudRoutineRun.self, forKey: .run)
        resultConversationID = try values.decode(String.self, forKey: .resultConversationID)
        attemptID = try values.decodeIfPresent(String.self, forKey: .attemptID)
        leaseID = try values.decodeIfPresent(String.self, forKey: .leaseID)
        let rawLeaseExpiresAt = try values.decodeIfPresent(String.self, forKey: .leaseExpiresAt)
        leaseExpiresAt = rawLeaseExpiresAt == "0001-01-01T00:00:00Z" ? nil : rawLeaseExpiresAt
        failureCode = try values.decodeIfPresent(String.self, forKey: .failureCode)
        nextAction = try values.decodeIfPresent(String.self, forKey: .nextAction)
        activities = try values.decode([FortCloudRoutineRunActivity].self, forKey: .activities)
        var previousSequence: Int64 = 0
        let orderedActivities = activities.allSatisfy { activity in
            defer { previousSequence = activity.sequence }
            return activity.sequence > previousSequence
        }
        guard fortCloudIdentity(resultConversationID), occurrence.id == run.occurrenceID,
              occurrence.routineID == run.routineID,
              occurrence.routineRevisionID == run.routineRevisionID,
              occurrence.kind == run.kind, occurrence.state == run.state,
              !activities.isEmpty, orderedActivities, activities.last?.state == run.state,
              attemptID == nil || fortCloudIdentity(attemptID!),
              leaseID == nil || fortCloudIdentity(leaseID!),
              leaseExpiresAt == nil || fortCloudTimestamp(leaseExpiresAt!) != nil
        else { throw FortCloudClientError.invalidProjection }
    }
}

public struct FortCloudConversationMessage: Decodable, Sendable, Hashable, Identifiable {
    public let id: Int64
    public let conversationID: String
    public let turnID: String?
    public let targetID: String?
    public let authorKind: String
    public let authorID: String
    public let authorAgentID: String?
    public let body: String
    public let createdAt: String

    enum CodingKeys: String, CodingKey {
        case id, body
        case conversationID = "conversation_id"
        case turnID = "turn_id"
        case targetID = "target_id"
        case authorKind = "author_kind"
        case authorID = "author_id"
        case authorAgentID = "author_agent_id"
        case createdAt = "created_at"
    }
}

public struct FortCloudConversationTurn: Decodable, Sendable, Hashable, Identifiable {
    public let id: String
    public let conversationID: String
    public let clientTurnID: String
    public let promptMessageID: Int64
    public let throughMessageID: Int64
    public let membershipRevisionID: String
    public let contextManifestID: String
    public let state: String
    public let createdAt: String

    enum CodingKeys: String, CodingKey {
        case id, state
        case conversationID = "conversation_id"
        case clientTurnID = "client_turn_id"
        case promptMessageID = "prompt_message_id"
        case throughMessageID = "through_message_id"
        case membershipRevisionID = "membership_revision_id"
        case contextManifestID = "context_manifest_id"
        case createdAt = "created_at"
    }
}

public struct FortCloudConversationTarget: Decodable, Sendable, Hashable, Identifiable {
    public let id: String
    public let turnID: String
    public let conversationID: String
    public let agentID: String
    public let behaviorRevisionID: String
    public let bindingRevisionID: String
    public let participantID: String
    public let runID: String
    public let state: String
    public let attemptCount: Int
    public let createdAt: String
    public let updatedAt: String

    enum CodingKeys: String, CodingKey {
        case id, state
        case turnID = "turn_id"
        case conversationID = "conversation_id"
        case agentID = "agent_id"
        case behaviorRevisionID = "behavior_revision_id"
        case bindingRevisionID = "binding_revision_id"
        case participantID = "participant_id"
        case runID = "run_id"
        case attemptCount = "attempt_count"
        case createdAt = "created_at"
        case updatedAt = "updated_at"
    }
}

public struct FortCloudContextManifest: Decodable, Sendable, Hashable, Identifiable {
    public let id: String
    public let conversationID: String
    public let throughMessageID: Int64
    public let messageIDs: [Int64]
    public let digest: String
    public let createdAt: String

    enum CodingKeys: String, CodingKey {
        case id, digest
        case conversationID = "conversation_id"
        case throughMessageID = "through_message_id"
        case messageIDs = "message_ids"
        case createdAt = "created_at"
    }
}

public struct FortCloudConversationProjection: Decodable, Sendable, Hashable {
    public let conversation: FortCloudAgentConversationRecord
    public let messages: [FortCloudConversationMessage]
    public let turns: [FortCloudConversationTurn]
    public let targets: [FortCloudConversationTarget]

    enum CodingKeys: String, CodingKey {
        case conversation, messages, turns, targets
    }

    public init(from decoder: Decoder) throws {
        let values = try decoder.container(keyedBy: CodingKeys.self)
        conversation = try values.decode(FortCloudAgentConversationRecord.self, forKey: .conversation)
        messages = try values.decode([FortCloudConversationMessage].self, forKey: .messages)
        turns = try values.decode([FortCloudConversationTurn].self, forKey: .turns)
        targets = try values.decode([FortCloudConversationTarget].self, forKey: .targets)
        guard messages.allSatisfy({ $0.conversationID == conversation.id }),
              turns.allSatisfy({ $0.conversationID == conversation.id }),
              targets.allSatisfy({ $0.conversationID == conversation.id && $0.agentID == conversation.link.agentID })
        else { throw FortCloudClientError.invalidProjection }
    }
}

public struct FortCloudTurnDispatch: Decodable, Sendable, Hashable {
    public let message: FortCloudConversationMessage
    public let turn: FortCloudConversationTurn
    public let context: FortCloudContextManifest
    public let target: FortCloudConversationTarget
    public let created: Bool
}

public enum FortCloudClientError: Error, Sendable, Equatable {
    case invalidConfiguration
    case invalidProjection
    case payloadLimit
    case httpStatus(Int, String)
    case nonHTTPResponse
}

extension FortCloudClientError: LocalizedError {
    public var errorDescription: String? {
        switch self {
        case .invalidConfiguration: return "Fort cloud access is not configured."
        case .invalidProjection: return "Fort returned an invalid cloud projection."
        case .payloadLimit: return "Fort cloud payload exceeded its safe limit."
        case .httpStatus(let status, _): return "Fort cloud returned HTTP \(status)."
        case .nonHTTPResponse: return "Fort cloud returned an unexpected response."
        }
    }
}

public final class FortCloudClient: @unchecked Sendable {
    public static let maximumBodyBytes = 4 * 1024 * 1024

    private let gatewayURL: URL
    private let bearerToken: String
    private let session: URLSession
    private let decoder = JSONDecoder()
    private let encoder = JSONEncoder()

    public init(gatewayURL: URL, bearerToken: String, session: URLSession = .shared) throws {
        guard var components = URLComponents(url: gatewayURL, resolvingAgainstBaseURL: false),
              components.scheme?.lowercased() == "https", components.host != nil,
              components.user == nil, components.password == nil,
              components.query == nil, components.fragment == nil,
              components.path.isEmpty || components.path == "/",
              bearerToken == bearerToken.trimmingCharacters(in: .whitespacesAndNewlines),
              !bearerToken.isEmpty, bearerToken.utf8.count <= 8_192
        else { throw FortCloudClientError.invalidConfiguration }
        components.path = ""
        guard let normalized = components.url else { throw FortCloudClientError.invalidConfiguration }
        self.gatewayURL = normalized
        self.bearerToken = bearerToken
        self.session = session
    }

    public func agents() async throws -> [FortCloudAgentRecord] {
        try await request(path: "/api/v2/agents?state=open", method: "GET", body: nil)
    }

    public func groups() async throws -> [FortCloudGroupRecord] {
        try await request(path: "/api/v2/groups?state=open", method: "GET", body: nil)
    }

    public func handoffs() async throws -> [FortCloudHandoffRecord] {
        try await request(path: "/api/v2/handoffs", method: "GET", body: nil)
    }

    public func handoff(handoffID: String) async throws -> FortCloudHandoffRecord {
        guard fortCloudPathIdentity(handoffID) else { throw FortCloudClientError.invalidConfiguration }
        let record: FortCloudHandoffRecord = try await request(
            path: handoffPath(handoffID), method: "GET", body: nil
        )
        guard record.id == handoffID else { throw FortCloudClientError.invalidProjection }
        return record
    }

    public func createHandoff(
        idempotencyKey: String,
        sourceConversationID: String,
        sourceMessageID: String,
        recipientAgentID: String,
        contextMessageIDs: [String],
        requestedResult: String,
        replyToMessageID: String? = nil,
        hardDeadline: Date
    ) async throws -> FortCloudHandoffRecord {
        let contextIsValid = contextMessageIDs.count <= 64 &&
            Set(contextMessageIDs).count == contextMessageIDs.count &&
            contextMessageIDs.allSatisfy(fortCloudIdentity)
        guard fortCloudIdempotencyKey(idempotencyKey),
              [sourceConversationID, sourceMessageID, recipientAgentID].allSatisfy(fortCloudIdentity),
              contextIsValid, fortCloudNonemptyText(requestedResult, maximumBytes: 2 * 1024 * 1024),
              replyToMessageID == nil || fortCloudIdentity(replyToMessageID!)
        else { throw FortCloudClientError.invalidConfiguration }
        let input = HandoffCreateRequest(
            idempotencyKey: idempotencyKey,
            sourceConversationID: sourceConversationID,
            sourceMessageID: sourceMessageID,
            recipientAgentID: recipientAgentID,
            contextMessageIDs: contextMessageIDs,
            requestedResult: requestedResult,
            replyToMessageID: replyToMessageID,
            hardDeadline: Self.timestamp(hardDeadline)
        )
        let record: FortCloudHandoffRecord = try await request(
            path: "/api/v2/handoffs", method: "POST", body: try encoder.encode(input)
        )
        let expectedDeadline = fortCloudTimestamp(input.hardDeadline)
        let responseContext = record.handoff.context.references.compactMap {
            $0.kind == .message ? $0.recordID : nil
        }
        guard record.handoff.idempotencyKey == idempotencyKey,
              record.handoff.sourceConversationID == sourceConversationID,
              record.handoff.sourceMessageID == sourceMessageID,
              record.handoff.recipientAgentID == recipientAgentID,
              responseContext == contextMessageIDs,
              record.handoff.requestedResult == requestedResult,
              record.handoff.replyToMessageID == replyToMessageID,
              expectedDeadline != nil,
              fortCloudTimestamp(record.handoff.deadline) == expectedDeadline
        else { throw FortCloudClientError.invalidProjection }
        return record
    }

    public func cancelHandoff(
        handoffID: String,
        idempotencyKey: String
    ) async throws -> FortCloudHandoffRecord {
        guard fortCloudPathIdentity(handoffID), fortCloudIdempotencyKey(idempotencyKey) else {
            throw FortCloudClientError.invalidConfiguration
        }
        let record: FortCloudHandoffRecord = try await request(
            path: "\(handoffPath(handoffID))/cancel",
            method: "POST",
            body: try encoder.encode(RetryRequest(idempotencyKey: idempotencyKey))
        )
        guard record.id == handoffID else { throw FortCloudClientError.invalidProjection }
        return record
    }

    public func routines(agentID: String) async throws -> [FortCloudRoutineRecord] {
        guard fortCloudPathIdentity(agentID) else { throw FortCloudClientError.invalidConfiguration }
        let records: [FortCloudRoutineRecord] = try await request(
            path: "\(agentPath(agentID))/routines", method: "GET", body: nil
        )
        guard records.allSatisfy({ $0.routine.agentID == agentID }) else {
            throw FortCloudClientError.invalidProjection
        }
        return records
    }

    public func createRoutine(
        agentID: String,
        idempotencyKey: String,
        trigger: FortCloudRoutineTrigger,
        schedule: String? = nil,
        timezone: String? = nil,
        nextOccurrence: Date? = nil,
        inputSource: String,
        freshnessSeconds: Int64,
        expectedResult: String,
        resultConversationID: String,
        approvalBoundary: String,
        missingInputBehavior: FortCloudRoutineMissingInputBehavior,
        retryPolicy: String,
        catchUpPolicy: String,
        latenessPolicy: String
    ) async throws -> FortCloudRoutineRecord {
        let scheduled: Bool
        switch trigger {
        case .schedule:
            scheduled = schedule.map { fortCloudRoutineIntent($0, maximumBytes: 512) } == true &&
                timezone.map { fortCloudRoutineIntent($0, maximumBytes: 128) } == true && nextOccurrence != nil
        case .event:
            scheduled = schedule == nil && timezone == nil && nextOccurrence == nil
        }
        let intent = [
            (inputSource, 4_096), (expectedResult, 4_096), (approvalBoundary, 512),
            (retryPolicy, 512), (catchUpPolicy, 512), (latenessPolicy, 512),
        ]
        guard fortCloudPathIdentity(agentID), fortCloudIdempotencyKey(idempotencyKey), scheduled,
              freshnessSeconds > 0, freshnessSeconds <= 365 * 24 * 60 * 60,
              fortCloudPathIdentity(resultConversationID),
              intent.allSatisfy({ fortCloudRoutineIntent($0.0, maximumBytes: $0.1) })
        else { throw FortCloudClientError.invalidConfiguration }
        let input = RoutineCreateRequest(
            idempotencyKey: idempotencyKey,
            trigger: trigger,
            schedule: schedule,
            timezone: timezone,
            nextOccurrence: nextOccurrence.map(Self.timestamp),
            inputSource: inputSource,
            freshnessSeconds: freshnessSeconds,
            expectedResult: expectedResult,
            resultConversationID: resultConversationID,
            approvalBoundary: approvalBoundary,
            missingInputBehavior: missingInputBehavior,
            retryPolicy: retryPolicy,
            catchUpPolicy: catchUpPolicy,
            latenessPolicy: latenessPolicy
        )
        let record: FortCloudRoutineRecord = try await request(
            path: "\(agentPath(agentID))/routines", method: "POST", body: try encoder.encode(input)
        )
        guard record.routine.agentID == agentID, record.currentRevision.authority == .fortCloud,
              record.currentRevision.trigger == trigger,
              record.currentRevision.resultConversationID == resultConversationID
        else { throw FortCloudClientError.invalidProjection }
        return record
    }

    public func revalidateRoutine(
        agentID: String,
        routineID: String,
        idempotencyKey: String
    ) async throws -> FortCloudRoutineRecord {
        guard fortCloudPathIdentity(agentID), fortCloudPathIdentity(routineID),
              fortCloudIdempotencyKey(idempotencyKey)
        else { throw FortCloudClientError.invalidConfiguration }
        let record: FortCloudRoutineRecord = try await request(
            path: routinePath(agentID, routineID),
            method: "PATCH",
            body: try encoder.encode(RoutineMutationRequest(
                idempotencyKey: idempotencyKey, action: .revalidate
            ))
        )
        guard record.id == routineID, record.routine.agentID == agentID,
              record.routine.state == .active, record.currentRevision.authority == .fortCloud
        else { throw FortCloudClientError.invalidProjection }
        return record
    }

    public func testRoutine(
        agentID: String,
        routineID: String,
        idempotencyKey: String
    ) async throws -> FortCloudRoutineRunRecord {
        guard fortCloudPathIdentity(agentID), fortCloudPathIdentity(routineID),
              fortCloudIdempotencyKey(idempotencyKey)
        else { throw FortCloudClientError.invalidConfiguration }
        let record: FortCloudRoutineRunRecord = try await request(
            path: "\(routinePath(agentID, routineID))/test",
            method: "POST",
            body: try encoder.encode(RetryRequest(idempotencyKey: idempotencyKey))
        )
        guard record.run.agentID == agentID, record.run.routineID == routineID,
              record.run.kind == .test, record.occurrence.idempotencyKey == idempotencyKey
        else { throw FortCloudClientError.invalidProjection }
        return record
    }

    public func group(groupID: String) async throws -> FortCloudGroupProjection {
        try await request(path: groupPath(groupID), method: "GET", body: nil)
    }

    public func createGroup(
        idempotencyKey: String,
        title: String,
        agentIDs: [String]
    ) async throws -> FortCloudGroupRecord {
        guard (2...6).contains(agentIDs.count), Set(agentIDs).count == agentIDs.count else {
            throw FortCloudClientError.invalidConfiguration
        }
        return try await request(
            path: "/api/v2/groups",
            method: "POST",
            body: try encoder.encode(GroupCreateRequest(
                idempotencyKey: idempotencyKey,
                title: title,
                agentIDs: agentIDs
            ))
        )
    }

    public func sendGroup(
        groupID: String,
        idempotencyKey: String,
        clientTurnID: String,
        text: String,
        selection: FortCloudGroupRecipientSelection,
        recipientAgentIDs: [String],
        concurrencyPolicy: FortCloudGroupConcurrencyPolicy,
        hardDeadline: Date
    ) async throws -> FortCloudGroupTurnRecord {
        guard !recipientAgentIDs.isEmpty, Set(recipientAgentIDs).count == recipientAgentIDs.count else {
            throw FortCloudClientError.invalidConfiguration
        }
        let formatter = ISO8601DateFormatter()
        formatter.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        return try await request(
            path: "\(groupPath(groupID))/turns",
            method: "POST",
            body: try encoder.encode(GroupTurnRequest(
                idempotencyKey: idempotencyKey,
                clientTurnID: clientTurnID,
                text: text,
                selection: selection,
                recipientAgentIDs: recipientAgentIDs,
                concurrencyPolicy: concurrencyPolicy,
                hardDeadline: formatter.string(from: hardDeadline)
            ))
        )
    }

    public func agentConversations(agentID: String) async throws -> [FortCloudAgentConversationRecord] {
        let records: [FortCloudAgentConversationRecord] = try await request(
            path: "\(agentPath(agentID))/conversations", method: "GET", body: nil
        )
        guard records.allSatisfy({ $0.link.agentID == agentID }) else {
            throw FortCloudClientError.invalidProjection
        }
        return records
    }

    public func createConversation(
        agentID: String,
        idempotencyKey: String,
        title: String
    ) async throws -> FortCloudAgentConversationRecord {
        guard !idempotencyKey.isEmpty, idempotencyKey.utf8.count <= 256,
              !title.isEmpty, title == title.trimmingCharacters(in: .whitespacesAndNewlines),
              title.utf8.count <= 512
        else { throw FortCloudClientError.invalidConfiguration }
        let record: FortCloudAgentConversationRecord = try await request(
            path: "\(agentPath(agentID))/conversations",
            method: "POST",
            body: try encoder.encode(ConversationCreateRequest(
                idempotencyKey: idempotencyKey,
                title: title
            ))
        )
        guard record.link.agentID == agentID, record.link.kind == .secondary else {
            throw FortCloudClientError.invalidProjection
        }
        return record
    }

    public func renameConversation(
        agentID: String,
        conversationID: String,
        idempotencyKey: String,
        expectedTitle: String,
        title: String
    ) async throws -> FortCloudAgentConversationRecord {
        guard expectedTitle != title,
              !expectedTitle.isEmpty, expectedTitle == expectedTitle.trimmingCharacters(in: .whitespacesAndNewlines),
              !title.isEmpty, title == title.trimmingCharacters(in: .whitespacesAndNewlines),
              expectedTitle.utf8.count <= 512, title.utf8.count <= 512
        else { throw FortCloudClientError.invalidConfiguration }
        return try await mutateConversation(
            agentID: agentID,
            conversationID: conversationID,
            request: ConversationMutationRequest(
                idempotencyKey: idempotencyKey,
                action: .rename,
                expectedTitle: expectedTitle,
                title: title
            )
        )
    }

    public func setConversationPinned(
        agentID: String,
        conversationID: String,
        idempotencyKey: String,
        pinned: Bool
    ) async throws -> FortCloudAgentConversationRecord {
        try await mutateConversation(
            agentID: agentID,
            conversationID: conversationID,
            request: ConversationMutationRequest(
                idempotencyKey: idempotencyKey,
                action: pinned ? .pin : .unpin,
                expectedTitle: nil,
                title: nil
            )
        )
    }

    public func setConversationArchived(
        agentID: String,
        conversationID: String,
        idempotencyKey: String,
        archived: Bool
    ) async throws -> FortCloudAgentConversationRecord {
        try await mutateConversation(
            agentID: agentID,
            conversationID: conversationID,
            request: ConversationMutationRequest(
                idempotencyKey: idempotencyKey,
                action: archived ? .archive : .reopen,
                expectedTitle: nil,
                title: nil
            )
        )
    }

    public func conversation(agentID: String, conversationID: String) async throws -> FortCloudConversationProjection {
        try await request(path: conversationPath(agentID, conversationID), method: "GET", body: nil)
    }

    public func send(
        agentID: String,
        conversationID: String,
        idempotencyKey: String,
        clientTurnID: String,
        text: String,
        hardDeadline: Date
    ) async throws -> FortCloudTurnDispatch {
        let formatter = ISO8601DateFormatter()
        formatter.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        let input = TurnRequest(
            idempotencyKey: idempotencyKey,
            clientTurnID: clientTurnID,
            text: text,
            hardDeadline: formatter.string(from: hardDeadline)
        )
        return try await request(
            path: "\(conversationPath(agentID, conversationID))/turns",
            method: "POST",
            body: try encoder.encode(input)
        )
    }

    public func retry(
        agentID: String,
        conversationID: String,
        targetID: String,
        idempotencyKey: String
    ) async throws -> FortCloudConversationTarget {
        try await request(
            path: "\(conversationPath(agentID, conversationID))/targets/\(Self.pathComponent(targetID))/retry",
            method: "POST",
            body: try encoder.encode(RetryRequest(idempotencyKey: idempotencyKey))
        )
    }

    public func cancel(
        agentID: String,
        conversationID: String,
        targetID: String,
        idempotencyKey: String
    ) async throws -> FortCloudConversationTarget {
        try await request(
            path: "\(conversationPath(agentID, conversationID))/targets/\(Self.pathComponent(targetID))/cancel",
            method: "POST",
            body: try encoder.encode(RetryRequest(idempotencyKey: idempotencyKey))
        )
    }

    private struct TurnRequest: Encodable {
        let idempotencyKey: String
        let clientTurnID: String
        let text: String
        let hardDeadline: String

        enum CodingKeys: String, CodingKey {
            case text
            case idempotencyKey = "idempotency_key"
            case clientTurnID = "client_turn_id"
            case hardDeadline = "hard_deadline"
        }
    }

    private struct RetryRequest: Encodable {
        let idempotencyKey: String
        enum CodingKeys: String, CodingKey { case idempotencyKey = "idempotency_key" }
    }

    private struct ConversationCreateRequest: Encodable {
        let idempotencyKey: String
        let title: String

        enum CodingKeys: String, CodingKey {
            case title
            case idempotencyKey = "idempotency_key"
        }
    }

    private struct RoutineCreateRequest: Encodable {
        let idempotencyKey: String
        let trigger: FortCloudRoutineTrigger
        let schedule: String?
        let timezone: String?
        let nextOccurrence: String?
        let inputSource: String
        let freshnessSeconds: Int64
        let expectedResult: String
        let resultConversationID: String
        let approvalBoundary: String
        let missingInputBehavior: FortCloudRoutineMissingInputBehavior
        let retryPolicy: String
        let catchUpPolicy: String
        let latenessPolicy: String

        enum CodingKeys: String, CodingKey {
            case trigger, schedule, timezone
            case idempotencyKey = "idempotency_key"
            case nextOccurrence = "next_occurrence"
            case inputSource = "input_source"
            case freshnessSeconds = "freshness_seconds"
            case expectedResult = "expected_result"
            case resultConversationID = "result_conversation_id"
            case approvalBoundary = "approval_boundary"
            case missingInputBehavior = "missing_input_behavior"
            case retryPolicy = "retry_policy"
            case catchUpPolicy = "catch_up_policy"
            case latenessPolicy = "lateness_policy"
        }
    }

    private enum RoutineMutationAction: String, Encodable {
        case revalidate
    }

    private struct RoutineMutationRequest: Encodable {
        let idempotencyKey: String
        let action: RoutineMutationAction

        enum CodingKeys: String, CodingKey {
            case action
            case idempotencyKey = "idempotency_key"
        }
    }

    private enum ConversationMutationAction: String, Encodable {
        case rename, pin, unpin, archive, reopen
    }

    private struct ConversationMutationRequest: Encodable {
        let idempotencyKey: String
        let action: ConversationMutationAction
        let expectedTitle: String?
        let title: String?

        enum CodingKeys: String, CodingKey {
            case action, title
            case idempotencyKey = "idempotency_key"
            case expectedTitle = "expected_title"
        }
    }

    private struct GroupCreateRequest: Encodable {
        let idempotencyKey: String
        let title: String
        let agentIDs: [String]

        enum CodingKeys: String, CodingKey {
            case title
            case idempotencyKey = "idempotency_key"
            case agentIDs = "agent_ids"
        }
    }

    private struct GroupTurnRequest: Encodable {
        let idempotencyKey: String
        let clientTurnID: String
        let text: String
        let selection: FortCloudGroupRecipientSelection
        let recipientAgentIDs: [String]
        let concurrencyPolicy: FortCloudGroupConcurrencyPolicy
        let hardDeadline: String

        enum CodingKeys: String, CodingKey {
            case text, selection
            case idempotencyKey = "idempotency_key"
            case clientTurnID = "client_turn_id"
            case recipientAgentIDs = "recipient_agent_ids"
            case concurrencyPolicy = "concurrency_policy"
            case hardDeadline = "hard_deadline"
        }
    }

    private struct HandoffCreateRequest: Encodable {
        let idempotencyKey: String
        let sourceConversationID: String
        let sourceMessageID: String
        let recipientAgentID: String
        let contextMessageIDs: [String]
        let requestedResult: String
        let replyToMessageID: String?
        let hardDeadline: String

        enum CodingKeys: String, CodingKey {
            case idempotencyKey = "idempotency_key"
            case sourceConversationID = "source_conversation_id"
            case sourceMessageID = "source_message_id"
            case recipientAgentID = "recipient_agent_id"
            case contextMessageIDs = "context_message_ids"
            case requestedResult = "requested_result"
            case replyToMessageID = "reply_to_message_id"
            case hardDeadline = "hard_deadline"
        }
    }

    private func request<Response: Decodable>(path: String, method: String, body: Data?) async throws -> Response {
        if let body, body.count > Self.maximumBodyBytes { throw FortCloudClientError.payloadLimit }
        let relative = path.hasPrefix("/") ? String(path.dropFirst()) : path
        let pieces = relative.split(separator: "?", maxSplits: 1, omittingEmptySubsequences: false)
        var url = gatewayURL.appendingPathComponent(String(pieces[0]))
        if pieces.count == 2, var components = URLComponents(url: url, resolvingAgainstBaseURL: false) {
            components.percentEncodedQuery = String(pieces[1])
            if let queryURL = components.url { url = queryURL }
        }
        var request = URLRequest(url: url)
        request.httpMethod = method
        request.setValue("Bearer \(bearerToken)", forHTTPHeaderField: "Authorization")
        request.setValue("application/json", forHTTPHeaderField: "Accept")
        request.setValue(FortRequestID.make(), forHTTPHeaderField: FortRequestID.header)
        request.cachePolicy = .reloadIgnoringLocalCacheData
        if let body {
            request.setValue("application/json", forHTTPHeaderField: "Content-Type")
            request.httpBody = body
        }
        let (data, response) = try await session.data(for: request)
        guard let http = response as? HTTPURLResponse else { throw FortCloudClientError.nonHTTPResponse }
        guard data.count <= Self.maximumBodyBytes else { throw FortCloudClientError.payloadLimit }
        guard (200...299).contains(http.statusCode) else {
            throw FortCloudClientError.httpStatus(http.statusCode, String(decoding: data.prefix(1_024), as: UTF8.self))
        }
        do {
            return try decoder.decode(Response.self, from: data)
        } catch let error as FortCloudClientError {
            throw error
        } catch {
            throw FortCloudClientError.invalidProjection
        }
    }

    private func mutateConversation(
        agentID: String,
        conversationID: String,
        request input: ConversationMutationRequest
    ) async throws -> FortCloudAgentConversationRecord {
        guard !input.idempotencyKey.isEmpty, input.idempotencyKey.utf8.count <= 256 else {
            throw FortCloudClientError.invalidConfiguration
        }
        let record: FortCloudAgentConversationRecord = try await request(
            path: conversationPath(agentID, conversationID),
            method: "PATCH",
            body: try encoder.encode(input)
        )
        guard record.link.agentID == agentID, record.id == conversationID,
              record.link.kind == .secondary
        else { throw FortCloudClientError.invalidProjection }
        return record
    }

    private func agentPath(_ agentID: String) -> String {
        "/api/v2/agents/\(Self.pathComponent(agentID))"
    }

    private func conversationPath(_ agentID: String, _ conversationID: String) -> String {
        "\(agentPath(agentID))/conversations/\(Self.pathComponent(conversationID))"
    }

    private func groupPath(_ groupID: String) -> String {
        "/api/v2/groups/\(Self.pathComponent(groupID))"
    }

    private func handoffPath(_ handoffID: String) -> String {
        "/api/v2/handoffs/\(Self.pathComponent(handoffID))"
    }

    private func routinePath(_ agentID: String, _ routineID: String) -> String {
        "\(agentPath(agentID))/routines/\(Self.pathComponent(routineID))"
    }

    private static func timestamp(_ date: Date) -> String {
        let formatter = ISO8601DateFormatter()
        formatter.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        return formatter.string(from: date)
    }

    private static func pathComponent(_ value: String) -> String {
        var allowed = CharacterSet.urlPathAllowed
        allowed.remove(charactersIn: "/?#")
        return value.addingPercentEncoding(withAllowedCharacters: allowed) ?? ""
    }
}

private func fortCloudIdentity(_ value: String) -> Bool {
    !value.isEmpty && value == value.trimmingCharacters(in: .whitespacesAndNewlines) &&
        value.utf8.count <= 512 && !value.contains("\r") && !value.contains("\n") && !value.contains("\0")
}

private func fortCloudPathIdentity(_ value: String) -> Bool {
    fortCloudIdentity(value) && !value.contains("/") && !value.contains("\\")
}

private func fortCloudIdempotencyKey(_ value: String) -> Bool {
    fortCloudIdentity(value) && value.utf8.count <= 256
}

private func fortCloudNonemptyText(_ value: String, maximumBytes: Int) -> Bool {
    !value.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty && value.utf8.count <= maximumBytes
}

private func fortCloudRoutineIntent(_ value: String, maximumBytes: Int) -> Bool {
    fortCloudNonemptyText(value, maximumBytes: maximumBytes) &&
        value == value.trimmingCharacters(in: .whitespacesAndNewlines) &&
        !value.contains("\r") && !value.contains("\n") && !value.contains("\0")
}

private func fortCloudUniqueNonempty(_ values: [String]) -> Bool {
    values.allSatisfy(fortCloudIdentity) && Set(values).count == values.count
}

private func fortCloudTimestamp(_ value: String) -> Date? {
    let fractional = ISO8601DateFormatter()
    fractional.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
    if let date = fractional.date(from: value) { return date }
    let whole = ISO8601DateFormatter()
    whole.formatOptions = [.withInternetDateTime]
    return whole.date(from: value)
}

private func fortCloudSHA256(_ value: String) -> Bool {
    value.utf8.count == 64 && value.allSatisfy { "0123456789abcdef".contains($0) }
}
