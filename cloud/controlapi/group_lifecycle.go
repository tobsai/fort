package controlapi

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/ledger"
)

type GroupMutationRepository interface {
	RenameGroup(context.Context, ledger.RenameGroupCommand) (ledger.GroupRecord, error)
	SetGroupState(context.Context, ledger.SetGroupStateCommand) (ledger.GroupRecord, error)
}

type GroupMembersRepository interface {
	GetGroup(context.Context, string, string) (ledger.GroupRecord, error)
	GetAgent(context.Context, string, string) (ledger.AgentRecord, error)
	ReplaceGroupMembers(context.Context, ledger.ReplaceGroupMembersCommand) (ledger.GroupRecord, error)
}

type groupMutationRequest struct {
	IdempotencyKey string `json:"idempotency_key"`
	Action         string `json:"action"`
	ExpectedTitle  string `json:"expected_title,omitempty"`
	Title          string `json:"title,omitempty"`
}

// GroupMutationHandler exposes the closed presentation/state lifecycle for a
// stable Group. It never accepts membership or execution revision fields.
func GroupMutationHandler(repository GroupMutationRepository, clock func() time.Time) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		prepareOwnerJSON(response)
		if request.Method != http.MethodPatch {
			response.Header().Set("Allow", http.MethodPatch)
			writeJSON(response, http.StatusMethodNotAllowed, map[string]string{"code": "group_mutation_method_not_allowed"})
			return
		}
		accountID, ok := AccountIDFromContext(request.Context())
		groupID := strings.TrimSpace(request.PathValue("group_id"))
		if !ok || !ownerPathIdentity(groupID) || len(request.URL.Query()) != 0 {
			writeJSON(response, http.StatusBadRequest, map[string]string{"code": "group_mutation_invalid"})
			return
		}
		var input groupMutationRequest
		if decodeStrictOwnerJSON(response, request, &input) != nil {
			writeJSON(response, http.StatusBadRequest, map[string]string{"code": "group_mutation_invalid"})
			return
		}
		now := ownerClock(clock)
		actorID := "human:" + accountID
		var record ledger.GroupRecord
		var err error
		switch input.Action {
		case "rename":
			command := ledger.RenameGroupCommand{
				IdempotencyKey: input.IdempotencyKey, AccountID: accountID, GroupID: groupID,
				ExpectedTitle: input.ExpectedTitle, Title: input.Title, ChangedBy: actorID, ChangedAt: now,
			}
			if command.Validate() != nil {
				writeJSON(response, http.StatusBadRequest, map[string]string{"code": "group_mutation_invalid"})
				return
			}
			if repository != nil {
				record, err = repository.RenameGroup(request.Context(), command)
			}
		case "archive", "reopen":
			if input.ExpectedTitle != "" || input.Title != "" {
				writeJSON(response, http.StatusBadRequest, map[string]string{"code": "group_mutation_invalid"})
				return
			}
			expected, desired := conversation.ConversationOpen, conversation.ConversationArchived
			if input.Action == "reopen" {
				expected, desired = desired, expected
			}
			command := ledger.SetGroupStateCommand{
				IdempotencyKey: input.IdempotencyKey, AccountID: accountID, GroupID: groupID,
				ExpectedState: expected, State: desired, ChangedBy: actorID, ChangedAt: now,
			}
			if command.Validate() != nil {
				writeJSON(response, http.StatusBadRequest, map[string]string{"code": "group_mutation_invalid"})
				return
			}
			if repository != nil {
				record, err = repository.SetGroupState(request.Context(), command)
			}
		default:
			writeJSON(response, http.StatusBadRequest, map[string]string{"code": "group_mutation_invalid"})
			return
		}
		if repository == nil {
			writeJSON(response, http.StatusServiceUnavailable, map[string]string{"code": "group_mutation_unavailable"})
			return
		}
		if err != nil {
			writeOwnerRepositoryError(response, err, "group_mutation_unavailable")
			return
		}
		writeBoundedOwnerJSON(response, record)
	})
}

type groupMembersRequest struct {
	IdempotencyKey               string   `json:"idempotency_key"`
	ExpectedMembershipRevisionID string   `json:"expected_membership_revision_id"`
	AgentIDs                     []string `json:"agent_ids"`
}

// GroupMembersHandler replaces the complete ordered stable-Agent membership.
// Behavior, Binding, participant, revision number, IDs, actor, and time are
// resolved by Fort and cannot be selected by the client.
func GroupMembersHandler(repository GroupMembersRepository, clock func() time.Time) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		prepareOwnerJSON(response)
		if request.Method != http.MethodPost {
			response.Header().Set("Allow", http.MethodPost)
			writeJSON(response, http.StatusMethodNotAllowed, map[string]string{"code": "group_members_method_not_allowed"})
			return
		}
		accountID, ok := AccountIDFromContext(request.Context())
		groupID := strings.TrimSpace(request.PathValue("group_id"))
		if !ok || !ownerPathIdentity(groupID) || len(request.URL.Query()) != 0 {
			writeJSON(response, http.StatusBadRequest, map[string]string{"code": "group_members_invalid"})
			return
		}
		var input groupMembersRequest
		if decodeStrictOwnerJSON(response, request, &input) != nil ||
			!ownerPathIdentity(input.ExpectedMembershipRevisionID) ||
			len(input.AgentIDs) < conversation.MinGroupAgents || len(input.AgentIDs) > conversation.MaxGroupAgents {
			writeJSON(response, http.StatusBadRequest, map[string]string{"code": "group_members_invalid"})
			return
		}
		if repository == nil {
			writeJSON(response, http.StatusServiceUnavailable, map[string]string{"code": "group_members_unavailable"})
			return
		}
		group, err := repository.GetGroup(request.Context(), accountID, groupID)
		if err != nil {
			writeOwnerRepositoryError(response, err, "group_members_unavailable")
			return
		}
		if group.Group.ID != groupID || group.Group.AccountID != accountID || group.Membership.GroupID != groupID ||
			group.Group.ConversationID != group.Conversation.ID || group.Group.State != conversation.ConversationOpen {
			writeJSON(response, http.StatusConflict, map[string]string{"code": "state_conflict"})
			return
		}
		if group.Membership.ID != input.ExpectedMembershipRevisionID ||
			group.Group.CurrentMembershipRevisionID != input.ExpectedMembershipRevisionID {
			writeJSON(response, http.StatusConflict, map[string]string{"code": "state_conflict"})
			return
		}
		currentBindings := make(map[string]conversation.GroupRecipient, len(group.MemberBindings))
		for _, binding := range group.MemberBindings {
			currentBindings[binding.AgentID] = binding
		}
		members := make([]conversation.GroupMember, 0, len(input.AgentIDs))
		bindings := make([]conversation.GroupRecipient, 0, len(input.AgentIDs))
		seen := make(map[string]struct{}, len(input.AgentIDs))
		for position, rawAgentID := range input.AgentIDs {
			agentID := strings.TrimSpace(rawAgentID)
			if agentID != rawAgentID || !ownerPathIdentity(agentID) {
				writeJSON(response, http.StatusBadRequest, map[string]string{"code": "group_members_invalid"})
				return
			}
			if _, duplicate := seen[agentID]; duplicate {
				writeJSON(response, http.StatusBadRequest, map[string]string{"code": "group_members_invalid"})
				return
			}
			seen[agentID] = struct{}{}
			agent, err := repository.GetAgent(request.Context(), accountID, agentID)
			if err != nil {
				writeOwnerRepositoryError(response, err, "group_members_unavailable")
				return
			}
			if agent.Agent.ID != agentID || agent.Agent.AccountID != accountID || agent.Agent.State != conversation.AgentOpen ||
				agent.Agent.CurrentBehaviorRevisionID != agent.Behavior.ID || agent.Agent.CurrentBindingRevisionID != agent.Binding.ID ||
				agent.Binding.AgentID != agentID || agent.Binding.BehaviorRevisionID != agent.Behavior.ID {
				writeJSON(response, http.StatusConflict, map[string]string{"code": "group_member_unready"})
				return
			}
			participantID := ownerCommandID("participant", accountID, groupID, agentID, agent.Binding.ID)
			if current, exists := currentBindings[agentID]; exists && current.BehaviorRevisionID == agent.Behavior.ID &&
				current.BindingRevisionID == agent.Binding.ID {
				participantID = current.ParticipantID
			}
			members = append(members, conversation.GroupMember{AgentID: agentID, Position: position})
			bindings = append(bindings, conversation.GroupRecipient{
				AgentID: agentID, BehaviorRevisionID: agent.Behavior.ID,
				BindingRevisionID: agent.Binding.ID, ParticipantID: participantID,
			})
		}
		now := ownerClock(clock)
		membershipID := ownerCommandID("membership", accountID, groupID, input.IdempotencyKey)
		command := ledger.ReplaceGroupMembersCommand{
			IdempotencyKey: input.IdempotencyKey, AccountID: accountID, GroupID: groupID,
			ExpectedMembershipRevisionID: input.ExpectedMembershipRevisionID,
			Membership: conversation.GroupMembershipRevision{
				ID: membershipID, GroupID: groupID, Revision: group.Membership.Revision + 1,
				Members: members, CreatedAt: now,
			},
			MemberBindings: bindings, ChangedBy: "human:" + accountID, ChangedAt: now,
		}
		if command.Validate() != nil {
			writeJSON(response, http.StatusBadRequest, map[string]string{"code": "group_members_invalid"})
			return
		}
		record, err := repository.ReplaceGroupMembers(request.Context(), command)
		if err != nil {
			writeOwnerRepositoryError(response, err, "group_members_unavailable")
			return
		}
		writeBoundedOwnerJSON(response, record)
	})
}
