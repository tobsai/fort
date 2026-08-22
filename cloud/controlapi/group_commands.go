package controlapi

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/ledger"
)

type GroupCreateRepository interface {
	GetAgent(context.Context, string, string) (ledger.AgentRecord, error)
	CreateGroup(context.Context, ledger.CreateGroupCommand) (ledger.GroupRecord, error)
}

type GroupDetailRepository interface {
	GetGroup(context.Context, string, string) (ledger.GroupRecord, error)
	ListGroupTurns(context.Context, string, string) ([]ledger.GroupTurnRecord, error)
	ListGroupMessages(context.Context, string, string) ([]ledger.AgentConversationMessage, error)
}

type GroupTurnRepository interface {
	GetAgent(context.Context, string, string) (ledger.AgentRecord, error)
	GetGroup(context.Context, string, string) (ledger.GroupRecord, error)
	SendGroupTurn(context.Context, ledger.SendGroupTurnCommand) (ledger.GroupTurnRecord, error)
}

type groupCreateRequest struct {
	IdempotencyKey string   `json:"idempotency_key"`
	Title          string   `json:"title"`
	AgentIDs       []string `json:"agent_ids"`
}

func GroupCreateHandler(repository GroupCreateRepository, clock func() time.Time) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		prepareOwnerJSON(response)
		if request.Method != http.MethodPost {
			response.Header().Set("Allow", http.MethodPost)
			writeJSON(response, http.StatusMethodNotAllowed, map[string]string{"code": "method_not_allowed"})
			return
		}
		accountID, ok := AccountIDFromContext(request.Context())
		if !ok {
			writeJSON(response, http.StatusUnauthorized, map[string]string{"code": "service_assertion_required"})
			return
		}
		if repository == nil {
			writeJSON(response, http.StatusServiceUnavailable, map[string]string{"code": "group_create_unavailable"})
			return
		}
		var input groupCreateRequest
		if err := decodeStrictOwnerJSON(response, request, &input); err != nil ||
			strings.TrimSpace(input.Title) == "" || input.Title != strings.TrimSpace(input.Title) ||
			len([]byte(input.Title)) > 512 || len(input.AgentIDs) < conversation.MinGroupAgents ||
			len(input.AgentIDs) > conversation.MaxGroupAgents {
			writeJSON(response, http.StatusBadRequest, map[string]string{"code": "group_create_invalid"})
			return
		}
		now := ownerClock(clock)
		seed := []string{accountID, input.IdempotencyKey}
		groupID := ownerCommandID("group", seed...)
		conversationID := ownerCommandID("conversation", seed...)
		membershipID := ownerCommandID("membership", seed...)
		members := make([]conversation.GroupMember, 0, len(input.AgentIDs))
		bindings := make([]conversation.GroupRecipient, 0, len(input.AgentIDs))
		seen := make(map[string]struct{}, len(input.AgentIDs))
		for position, rawAgentID := range input.AgentIDs {
			agentID := strings.TrimSpace(rawAgentID)
			if !ownerPathIdentity(agentID) || agentID != rawAgentID {
				writeJSON(response, http.StatusBadRequest, map[string]string{"code": "group_create_invalid"})
				return
			}
			if _, exists := seen[agentID]; exists {
				writeJSON(response, http.StatusBadRequest, map[string]string{"code": "group_create_invalid"})
				return
			}
			seen[agentID] = struct{}{}
			record, err := repository.GetAgent(request.Context(), accountID, agentID)
			if err != nil {
				writeOwnerRepositoryError(response, err, "group_create_unavailable")
				return
			}
			if record.Agent.ID != agentID || record.Agent.AccountID != accountID || record.Agent.State != conversation.AgentOpen ||
				record.Agent.CurrentBehaviorRevisionID != record.Behavior.ID ||
				record.Agent.CurrentBindingRevisionID != record.Binding.ID ||
				record.Binding.AgentID != agentID || record.Binding.BehaviorRevisionID != record.Behavior.ID {
				writeJSON(response, http.StatusConflict, map[string]string{"code": "group_member_unready"})
				return
			}
			members = append(members, conversation.GroupMember{AgentID: agentID, Position: position})
			bindings = append(bindings, conversation.GroupRecipient{
				AgentID: agentID, BehaviorRevisionID: record.Behavior.ID, BindingRevisionID: record.Binding.ID,
				ParticipantID: ownerCommandID("participant", accountID, groupID, agentID, record.Binding.ID),
			})
		}
		command := ledger.CreateGroupCommand{
			IdempotencyKey: input.IdempotencyKey,
			Group: conversation.GroupConversation{
				ID: groupID, AccountID: accountID, ConversationID: conversationID, State: conversation.ConversationOpen,
				CurrentMembershipRevisionID: membershipID, CreatedAt: now,
			},
			Conversation: conversation.Conversation{
				ID: conversationID, Title: input.Title, State: conversation.ConversationOpen, CreatedAt: now, UpdatedAt: now,
			},
			Membership: conversation.GroupMembershipRevision{
				ID: membershipID, GroupID: groupID, Revision: 1, Members: members, CreatedAt: now,
			},
			MemberBindings: bindings,
		}
		if err := command.Validate(); err != nil {
			writeJSON(response, http.StatusBadRequest, map[string]string{"code": "group_create_invalid"})
			return
		}
		record, err := repository.CreateGroup(request.Context(), command)
		if err != nil {
			writeOwnerRepositoryError(response, err, "group_create_unavailable")
			return
		}
		writeBoundedOwnerJSONStatus(response, http.StatusCreated, record)
	})
}

type groupDetailProjection struct {
	Group    ledger.GroupRecord                `json:"group"`
	Turns    []ledger.GroupTurnRecord          `json:"turns"`
	Messages []ledger.AgentConversationMessage `json:"messages"`
}

func GroupDetailHandler(repository GroupDetailRepository) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		prepareOwnerJSON(response)
		if request.Method != http.MethodGet {
			response.Header().Set("Allow", http.MethodGet)
			writeJSON(response, http.StatusMethodNotAllowed, map[string]string{"code": "method_not_allowed"})
			return
		}
		accountID, ok := AccountIDFromContext(request.Context())
		groupID := strings.TrimSpace(request.PathValue("group_id"))
		if !ok || !ownerPathIdentity(groupID) || len(request.URL.Query()) != 0 {
			writeJSON(response, http.StatusBadRequest, map[string]string{"code": "group_read_invalid"})
			return
		}
		if repository == nil {
			writeJSON(response, http.StatusServiceUnavailable, map[string]string{"code": "group_read_unavailable"})
			return
		}
		group, err := repository.GetGroup(request.Context(), accountID, groupID)
		if err != nil {
			writeOwnerRepositoryError(response, err, "group_read_unavailable")
			return
		}
		turns, err := repository.ListGroupTurns(request.Context(), accountID, groupID)
		if err != nil {
			writeOwnerRepositoryError(response, err, "group_read_unavailable")
			return
		}
		if turns == nil {
			turns = []ledger.GroupTurnRecord{}
		}
		messages, err := repository.ListGroupMessages(request.Context(), accountID, groupID)
		if err != nil {
			writeOwnerRepositoryError(response, err, "group_read_unavailable")
			return
		}
		if messages == nil {
			messages = []ledger.AgentConversationMessage{}
		}
		writeBoundedOwnerJSON(response, groupDetailProjection{Group: group, Turns: turns, Messages: messages})
	})
}

type groupTurnRequest struct {
	IdempotencyKey    string                               `json:"idempotency_key"`
	ClientTurnID      string                               `json:"client_turn_id"`
	Text              string                               `json:"text"`
	Selection         conversation.GroupRecipientSelection `json:"selection"`
	RecipientAgentIDs []string                             `json:"recipient_agent_ids"`
	ConcurrencyPolicy conversation.GroupConcurrencyPolicy  `json:"concurrency_policy"`
	HardDeadline      time.Time                            `json:"hard_deadline"`
}

func GroupTurnsHandler(repository GroupTurnRepository, clock func() time.Time) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		prepareOwnerJSON(response)
		if request.Method != http.MethodPost {
			response.Header().Set("Allow", http.MethodPost)
			writeJSON(response, http.StatusMethodNotAllowed, map[string]string{"code": "method_not_allowed"})
			return
		}
		accountID, ok := AccountIDFromContext(request.Context())
		groupID := strings.TrimSpace(request.PathValue("group_id"))
		var input groupTurnRequest
		if !ok || !ownerPathIdentity(groupID) || len(request.URL.Query()) != 0 {
			writeJSON(response, http.StatusBadRequest, map[string]string{"code": "group_turn_invalid"})
			return
		}
		if repository == nil {
			writeJSON(response, http.StatusServiceUnavailable, map[string]string{"code": "group_turn_unavailable"})
			return
		}
		if decodeStrictOwnerJSON(response, request, &input) != nil {
			writeJSON(response, http.StatusBadRequest, map[string]string{"code": "group_turn_invalid"})
			return
		}
		now := ownerClock(clock)
		deadline := input.HardDeadline.UTC()
		if !deadline.After(now) || deadline.After(now.Add(maximumDirectTurnWindow)) ||
			strings.TrimSpace(input.Text) == "" || len([]byte(input.Text)) > ledger.MaxAgentMessageBytes {
			writeJSON(response, http.StatusBadRequest, map[string]string{"code": "group_turn_invalid"})
			return
		}
		group, err := repository.GetGroup(request.Context(), accountID, groupID)
		if err != nil {
			writeOwnerRepositoryError(response, err, "group_turn_unavailable")
			return
		}
		members := make(map[string]struct{}, len(group.Membership.Members))
		for _, member := range group.Membership.Members {
			members[member.AgentID] = struct{}{}
		}
		recipients := make([]conversation.GroupRecipient, 0, len(input.RecipientAgentIDs))
		seen := make(map[string]struct{}, len(input.RecipientAgentIDs))
		for _, agentID := range input.RecipientAgentIDs {
			_, exists := members[agentID]
			if !exists || !ownerPathIdentity(agentID) {
				writeJSON(response, http.StatusBadRequest, map[string]string{"code": "group_turn_invalid"})
				return
			}
			if _, duplicate := seen[agentID]; duplicate {
				writeJSON(response, http.StatusBadRequest, map[string]string{"code": "group_turn_invalid"})
				return
			}
			seen[agentID] = struct{}{}
			agent, err := repository.GetAgent(request.Context(), accountID, agentID)
			if err != nil {
				writeOwnerRepositoryError(response, err, "group_turn_unavailable")
				return
			}
			if agent.Agent.ID != agentID || agent.Agent.AccountID != accountID ||
				agent.Agent.State != conversation.AgentOpen ||
				agent.Agent.CurrentBehaviorRevisionID != agent.Behavior.ID ||
				agent.Agent.CurrentBindingRevisionID != agent.Binding.ID ||
				agent.Binding.AgentID != agentID || agent.Binding.BehaviorRevisionID != agent.Behavior.ID {
				writeJSON(response, http.StatusConflict, map[string]string{"code": "group_member_unready"})
				return
			}
			recipients = append(recipients, conversation.GroupRecipient{
				AgentID: agentID, BehaviorRevisionID: agent.Behavior.ID, BindingRevisionID: agent.Binding.ID,
				ParticipantID: ownerCommandID("participant", accountID, groupID, agentID, agent.Binding.ID),
			})
		}
		seed := []string{accountID, groupID, input.IdempotencyKey}
		turnID := ownerCommandID("group-turn", seed...)
		targetIDs := make([]string, 0, len(recipients))
		for _, recipient := range recipients {
			targetIDs = append(targetIDs, ownerCommandID("target", accountID, turnID, recipient.AgentID))
		}
		command := ledger.SendGroupTurnCommand{
			AccountID: accountID, HumanID: "human:" + accountID, Body: input.Text,
			Envelope: conversation.GroupTurnEnvelope{
				ID: turnID, GroupID: groupID, ConversationID: group.Group.ConversationID,
				ClientTurnID: input.ClientTurnID, IdempotencyKey: input.IdempotencyKey,
				MembershipRevisionID: group.Membership.ID, Selection: input.Selection, Recipients: recipients,
				ContextSnapshotID:    ownerCommandID("context", seed...),
				RootDelegationGrant:  conversation.AuthorityGrant{ID: ownerCommandID("grant", seed...), Permissions: []string{}, ContextRecordIDs: []string{}},
				ConcurrencyPolicy:    input.ConcurrencyPolicy,
				CancellationPolicyID: "group-cancel:human-or-deadline", CancellationPolicyRevision: "1",
				ApprovalPolicyID: "group-approval:explicit", ApprovalPolicyRevision: "1",
				MaxAgentMessages: conversation.MaxGroupAgentMessages, MaxHandoffDepth: conversation.MaxGroupHandoffDepth,
				CostLimitClass: conversation.LimitUnknown, TokenLimitClass: conversation.LimitUnknown,
				Deadline: deadline, CreatedAt: now,
			},
			TargetIDs: targetIDs,
		}
		if err := command.Validate(group.Group, group.Membership); err != nil {
			writeJSON(response, http.StatusBadRequest, map[string]string{"code": "group_turn_invalid"})
			return
		}
		record, err := repository.SendGroupTurn(request.Context(), command)
		if err != nil {
			writeOwnerRepositoryError(response, err, "group_turn_unavailable")
			return
		}
		writeBoundedOwnerJSONStatus(response, http.StatusAccepted, record)
	})
}

func ownerClock(clock func() time.Time) time.Time {
	if clock != nil {
		return clock().UTC()
	}
	return time.Now().UTC()
}
