package controlapi

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/ledger"
)

// AgentMutationRepository is the owner-facing presentation/behavior seam.
// The handler can advance Fort-owned revisions but cannot select or alter any
// provider, model, machine, adapter, authority, policy, or source identity.
type AgentMutationRepository interface {
	GetAgent(context.Context, string, string) (ledger.AgentRecord, error)
	AppendAgentProfile(context.Context, ledger.AppendAgentProfileCommand) (ledger.AgentRecord, error)
	AppendAgentBehavior(context.Context, ledger.AppendAgentBehaviorCommand) (ledger.AgentBindingAdvanceResult, error)
}

type agentMutationRequest struct {
	Action                     string                 `json:"action"`
	IdempotencyKey             string                 `json:"idempotency_key"`
	ExpectedProfileRevisionID  string                 `json:"expected_profile_revision_id,omitempty"`
	ExpectedBehaviorRevisionID string                 `json:"expected_behavior_revision_id,omitempty"`
	ExpectedBindingRevisionID  string                 `json:"expected_binding_revision_id,omitempty"`
	Profile                    *agentProfileMutation  `json:"profile,omitempty"`
	Behavior                   *agentBehaviorMutation `json:"behavior,omitempty"`
}

type agentProfileMutation struct {
	Name      string `json:"name"`
	Title     string `json:"title"`
	AvatarURL string `json:"avatar_url"`
	Hidden    bool   `json:"hidden"`
	Pinned    bool   `json:"pinned"`
	SortOrder int    `json:"sort_order"`
}

type agentBehaviorMutation struct {
	Role                 string   `json:"role"`
	StandingInstructions string   `json:"standing_instructions"`
	EnabledSkills        []string `json:"enabled_skills"`
	EnabledTools         []string `json:"enabled_tools"`
	PromptMaterial       string   `json:"prompt_material"`
}

func AgentMutationHandler(repository AgentMutationRepository, clock func() time.Time) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		prepareOwnerJSON(response)
		if request.Method != http.MethodPatch {
			response.Header().Set("Allow", http.MethodPatch)
			writeJSON(response, http.StatusMethodNotAllowed, map[string]string{"code": "method_not_allowed"})
			return
		}
		accountID, ok := AccountIDFromContext(request.Context())
		agentID := strings.TrimSpace(request.PathValue("agent_id"))
		if !ok || !ownerPathIdentity(agentID) || len(request.URL.Query()) != 0 {
			writeJSON(response, http.StatusBadRequest, map[string]string{"code": "agent_mutation_invalid"})
			return
		}
		var input agentMutationRequest
		if decodeStrictOwnerJSON(response, request, &input) != nil || !validAgentMutationRequest(input) {
			writeJSON(response, http.StatusBadRequest, map[string]string{"code": "agent_mutation_invalid"})
			return
		}
		if repository == nil {
			writeJSON(response, http.StatusServiceUnavailable, map[string]string{"code": "agent_mutation_unavailable"})
			return
		}
		current, err := repository.GetAgent(request.Context(), accountID, agentID)
		if err != nil {
			writeOwnerRepositoryError(response, err, "agent_mutation_unavailable")
			return
		}
		if current.Agent.AccountID != accountID || current.Agent.ID != agentID || current.Agent.State != conversation.AgentOpen {
			writeJSON(response, http.StatusConflict, map[string]string{"code": "agent_state_conflict"})
			return
		}
		now := ownerClock(clock)
		seed := []string{accountID, agentID, input.IdempotencyKey}
		switch input.Action {
		case "profile":
			profile := *input.Profile
			result, err := repository.AppendAgentProfile(request.Context(), ledger.AppendAgentProfileCommand{
				IdempotencyKey: input.IdempotencyKey, AccountID: accountID, AgentID: agentID,
				ExpectedProfileRevisionID: input.ExpectedProfileRevisionID,
				Revision: conversation.AgentProfileRevision{
					ID: ownerCommandID("profile", seed...), AgentID: agentID, Revision: current.Profile.Revision + 1,
					Name: profile.Name, Title: profile.Title, AvatarURL: profile.AvatarURL,
					Hidden: profile.Hidden, Pinned: profile.Pinned, SortOrder: profile.SortOrder, CreatedAt: now,
				},
				AcceptedBy: "human:" + accountID,
			})
			if err != nil {
				writeOwnerRepositoryError(response, err, "agent_mutation_unavailable")
				return
			}
			writeBoundedOwnerJSONStatus(response, http.StatusAccepted, result)
		case "behavior":
			command := buildAgentBehaviorCommand(accountID, agentID, input, current, now, seed)
			if err := command.Validate(); err != nil {
				writeJSON(response, http.StatusBadRequest, map[string]string{"code": "agent_mutation_invalid"})
				return
			}
			result, err := repository.AppendAgentBehavior(request.Context(), command)
			if err != nil {
				writeOwnerRepositoryError(response, err, "agent_mutation_unavailable")
				return
			}
			writeBoundedOwnerJSONStatus(response, http.StatusAccepted, result)
		}
	})
}

func validAgentMutationRequest(input agentMutationRequest) bool {
	if strings.TrimSpace(input.IdempotencyKey) == "" || input.IdempotencyKey != strings.TrimSpace(input.IdempotencyKey) ||
		len([]byte(input.IdempotencyKey)) > 256 {
		return false
	}
	switch input.Action {
	case "profile":
		if input.Profile == nil || input.Behavior != nil || !ownerPathIdentity(input.ExpectedProfileRevisionID) ||
			input.ExpectedBehaviorRevisionID != "" || input.ExpectedBindingRevisionID != "" {
			return false
		}
		profile := input.Profile
		return profile.Name == strings.TrimSpace(profile.Name) && profile.Name != "" && len([]byte(profile.Name)) <= 120 &&
			profile.Title == strings.TrimSpace(profile.Title) && len([]byte(profile.Title)) <= 512 &&
			profile.AvatarURL == strings.TrimSpace(profile.AvatarURL) && len([]byte(profile.AvatarURL)) <= 2_048
	case "behavior":
		if input.Behavior == nil || input.Profile != nil || input.ExpectedProfileRevisionID != "" ||
			!ownerPathIdentity(input.ExpectedBehaviorRevisionID) || !ownerPathIdentity(input.ExpectedBindingRevisionID) {
			return false
		}
		behavior := input.Behavior
		return behavior.EnabledSkills != nil && behavior.EnabledTools != nil &&
			behavior.Role == strings.TrimSpace(behavior.Role) && behavior.Role != "" && len([]byte(behavior.Role)) <= 4_096 &&
			len([]byte(behavior.StandingInstructions)) <= 100_000 && len([]byte(behavior.PromptMaterial)) <= 100_000
	default:
		return false
	}
}

func buildAgentBehaviorCommand(
	accountID, agentID string,
	input agentMutationRequest,
	current ledger.AgentRecord,
	now time.Time,
	seed []string,
) ledger.AppendAgentBehaviorCommand {
	intent := *input.Behavior
	behavior := conversation.AgentBehaviorRevision{
		ID: ownerCommandID("behavior", seed...), AgentID: agentID, Revision: current.Behavior.Revision + 1,
		Role: intent.Role, StandingInstructions: intent.StandingInstructions,
		EnabledSkills: append([]string{}, intent.EnabledSkills...), EnabledTools: append([]string{}, intent.EnabledTools...),
		PromptMaterial: intent.PromptMaterial, CreatedAt: now,
	}
	binding := current.Binding
	binding.ID = ownerCommandID("binding", seed...)
	binding.Revision = current.Binding.Revision + 1
	binding.BehaviorRevisionID = behavior.ID
	binding.SeatID = ownerCommandID("seat", seed...)
	binding.SupersedesRevisionID = current.Binding.ID
	binding.ActivatedAt = now
	binding.RetiredAt = time.Time{}
	binding.CapabilityEvidence = append([]string{}, current.Binding.CapabilityEvidence...)
	participant := current.Participant
	participant.ID = ownerCommandID("participant", seed...)
	participant.ConversationID = current.Home.ID
	participant.SeatID = binding.SeatID
	participant.Profile = binding.FortProfile
	participant.Agent = binding.Provider
	participant.Model = binding.RequestedModel
	if binding.ComputerID != "" {
		participant.Machine = binding.ComputerID
	} else {
		participant.Machine = binding.CloudRuntime
	}
	participant.DisplayName = current.Profile.Name
	participant.Position = 0
	participant.State = conversation.ParticipantActive
	participant.CreatedAt = now
	participant.RemovedAt = time.Time{}
	return ledger.AppendAgentBehaviorCommand{
		IdempotencyKey: input.IdempotencyKey, AccountID: accountID, AgentID: agentID,
		ExpectedBehaviorRevisionID: input.ExpectedBehaviorRevisionID,
		ExpectedBindingRevisionID:  input.ExpectedBindingRevisionID,
		Behavior:                   behavior, Binding: binding, Participant: participant,
		ReadinessEvidence: []string{"readiness:" + binding.ReadinessContractID + "@" + binding.ReadinessContractRevision},
		AuthorityEvidence: []string{"authority:" + binding.AuthorityID + "@" + binding.AuthorityRevision},
		AcceptedBy:        "human:" + accountID, AcceptedAt: now,
	}
}
