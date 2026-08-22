package controlapi

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/ledger"
)

const (
	rebindAcceptanceAudience = "fort-control"
	rebindAcceptanceRoute    = "owner.agents.rebind"
	maximumRebindTokenTTL    = 5 * time.Minute
	maximumRebindTokenBytes  = 64 * 1024
)

var ErrNoEligibleAgentOption = errors.New("no approved eligible Agent option")

// EligibleAgentOption is server-held execution evidence selected by one
// opaque ID. Identity, revision, seat, and activation fields are deliberately
// absent from the option and are allocated by Fort for each command.
type EligibleAgentOption struct {
	ID                       string                            `json:"id"`
	ExecutionSource          conversation.ExecutionSource      `json:"execution_source"`
	SourceAgent              conversation.SourceAgent          `json:"source_agent"`
	Binding                  conversation.AgentBindingRevision `json:"binding"`
	NonTransferableResources []ledger.RebindResource           `json:"non_transferable_resources"`
	ReadinessEvidence        []string                          `json:"readiness_evidence"`
	AuthorityEvidence        []string                          `json:"authority_evidence"`
}

type AgentOptionResolver interface {
	ResolveEligibleAgentOption(context.Context, string, string) (EligibleAgentOption, error)
}

type AgentCreateRepository interface {
	CreateAgent(context.Context, ledger.CreateAgentCommand) (ledger.AgentRecord, error)
}

type AgentRebindRepository interface {
	GetAgent(context.Context, string, string) (ledger.AgentRecord, error)
	PreviewAgentRebind(context.Context, ledger.PreviewAgentRebindCommand) (ledger.AgentRebindPreview, error)
	AcceptAgentRebind(context.Context, ledger.AcceptAgentRebindCommand) (ledger.AgentBindingAdvanceResult, error)
}

type AgentLifecycleCommandRepository interface {
	AgentCreateRepository
	AgentRebindRepository
}

type rejectingAgentOptionResolver struct{}

func (rejectingAgentOptionResolver) ResolveEligibleAgentOption(context.Context, string, string) (EligibleAgentOption, error) {
	return EligibleAgentOption{}, ErrNoEligibleAgentOption
}

// NoEligibleAgentOptions is the production-safe default. Enrollment and
// Rebind remain unavailable until composition injects an approved inventory.
func NoEligibleAgentOptions() AgentOptionResolver { return rejectingAgentOptionResolver{} }

type agentCreateRequest struct {
	IdempotencyKey string                `json:"idempotency_key"`
	OptionID       string                `json:"option_id"`
	Profile        agentProfileMutation  `json:"profile"`
	Behavior       agentBehaviorMutation `json:"behavior"`
}

func AgentCreateHandler(repository AgentCreateRepository, resolver AgentOptionResolver, clock func() time.Time) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		prepareOwnerJSON(response)
		if request.Method != http.MethodPost {
			response.Header().Set("Allow", http.MethodPost)
			writeJSON(response, http.StatusMethodNotAllowed, map[string]string{"code": "method_not_allowed"})
			return
		}
		accountID, ok := AccountIDFromContext(request.Context())
		if !ok || len(request.URL.Query()) != 0 {
			writeJSON(response, http.StatusBadRequest, map[string]string{"code": "agent_create_invalid"})
			return
		}
		var input agentCreateRequest
		if decodeStrictOwnerJSON(response, request, &input) != nil || !validAgentCreateRequest(input) {
			writeJSON(response, http.StatusBadRequest, map[string]string{"code": "agent_create_invalid"})
			return
		}
		if repository == nil || resolver == nil {
			writeJSON(response, http.StatusServiceUnavailable, map[string]string{"code": "agent_create_unavailable"})
			return
		}
		option, err := resolver.ResolveEligibleAgentOption(request.Context(), accountID, input.OptionID)
		if err != nil {
			writeAgentOptionError(response, err, "agent_create_unavailable")
			return
		}
		option = canonicalEligibleAgentOption(option)
		now := ownerClock(clock)
		command := buildAgentCreateCommand(accountID, input, option, now)
		if err := validateEligibleAgentOption(accountID, input.OptionID, option, now); err != nil || command.Validate() != nil {
			writeJSON(response, http.StatusConflict, map[string]string{"code": "agent_option_ineligible"})
			return
		}
		record, err := repository.CreateAgent(request.Context(), command)
		if err != nil {
			writeOwnerRepositoryError(response, err, "agent_create_unavailable")
			return
		}
		writeBoundedOwnerJSONStatus(response, http.StatusCreated, record)
	})
}

func validAgentCreateRequest(input agentCreateRequest) bool {
	if !ownerIntentString(input.IdempotencyKey, 256) || !ownerPathIdentity(input.OptionID) || input.OptionID != strings.TrimSpace(input.OptionID) {
		return false
	}
	profile := input.Profile
	if profile.Name == "" || profile.Name != strings.TrimSpace(profile.Name) || len([]byte(profile.Name)) > 120 ||
		profile.Title != strings.TrimSpace(profile.Title) || len([]byte(profile.Title)) > 512 ||
		profile.AvatarURL != strings.TrimSpace(profile.AvatarURL) || len([]byte(profile.AvatarURL)) > 2_048 {
		return false
	}
	behavior := input.Behavior
	return behavior.EnabledSkills != nil && behavior.EnabledTools != nil && behavior.Role != "" &&
		behavior.Role == strings.TrimSpace(behavior.Role) && len([]byte(behavior.Role)) <= 4_096 &&
		len([]byte(behavior.StandingInstructions)) <= 100_000 && len([]byte(behavior.PromptMaterial)) <= 100_000
}

func buildAgentCreateCommand(accountID string, input agentCreateRequest, option EligibleAgentOption, now time.Time) ledger.CreateAgentCommand {
	seed := []string{accountID, input.IdempotencyKey, option.ID}
	agentID := ownerCommandID("agent", seed...)
	profileID := ownerCommandID("profile", seed...)
	behaviorID := ownerCommandID("behavior", seed...)
	bindingID := ownerCommandID("binding", seed...)
	homeID := ownerCommandID("conversation", seed...)
	seatID := ownerCommandID("seat", seed...)
	participantID := ownerCommandID("participant", seed...)
	binding := materializeBinding(option.Binding, agentID, behaviorID, bindingID, seatID, "", 1, now)
	profile := conversation.AgentProfileRevision{
		ID: profileID, AgentID: agentID, Revision: 1, Name: input.Profile.Name, Title: input.Profile.Title,
		AvatarURL: input.Profile.AvatarURL, Hidden: input.Profile.Hidden, Pinned: input.Profile.Pinned,
		SortOrder: input.Profile.SortOrder, CreatedAt: now,
	}
	behavior := conversation.AgentBehaviorRevision{
		ID: behaviorID, AgentID: agentID, Revision: 1, Role: input.Behavior.Role,
		StandingInstructions: input.Behavior.StandingInstructions,
		EnabledSkills:        append([]string{}, input.Behavior.EnabledSkills...),
		EnabledTools:         append([]string{}, input.Behavior.EnabledTools...), PromptMaterial: input.Behavior.PromptMaterial, CreatedAt: now,
	}
	home := conversation.Conversation{ID: homeID, Title: "Home", State: conversation.ConversationOpen, CreatedAt: now, UpdatedAt: now}
	return ledger.CreateAgentCommand{
		IdempotencyKey: input.IdempotencyKey,
		Agent: conversation.Agent{
			ID: agentID, AccountID: accountID, State: conversation.AgentOpen, CurrentProfileRevisionID: profileID,
			CurrentBehaviorRevisionID: behaviorID, CurrentBindingRevisionID: bindingID, CanonicalConversationID: homeID, CreatedAt: now,
		},
		Profile: profile, Behavior: behavior, Binding: binding, ExecutionSource: option.ExecutionSource, SourceAgent: option.SourceAgent,
		Home: home,
		Participant: conversation.Participant{
			ID: participantID, ConversationID: homeID, SeatID: seatID, Profile: binding.FortProfile, Agent: binding.Provider,
			Model: binding.RequestedModel, Machine: bindingLocation(binding), DisplayName: profile.Name,
			Position: 0, State: conversation.ParticipantActive, CreatedAt: now,
		},
		Link: conversation.AgentConversation{AgentID: agentID, ConversationID: homeID, Kind: conversation.AgentConversationCanonical, CreatedAt: now},
	}
}

type agentRebindRequest struct {
	Action                    string `json:"action"`
	IdempotencyKey            string `json:"idempotency_key,omitempty"`
	OptionID                  string `json:"option_id,omitempty"`
	ExpectedBindingRevisionID string `json:"expected_binding_revision_id,omitempty"`
	AcceptanceToken           string `json:"acceptance_token,omitempty"`
}

type agentRebindPreviewResponse struct {
	Preview         ledger.AgentRebindPreview `json:"preview"`
	AcceptanceToken string                    `json:"acceptance_token"`
	ExpiresAt       time.Time                 `json:"expires_at"`
}

func AgentRebindHandler(repository AgentRebindRepository, resolver AgentOptionResolver, tokens RebindAcceptanceTokens, clock func() time.Time) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		prepareOwnerJSON(response)
		if request.Method != http.MethodPost {
			response.Header().Set("Allow", http.MethodPost)
			writeJSON(response, http.StatusMethodNotAllowed, map[string]string{"code": "method_not_allowed"})
			return
		}
		accountID, ok := AccountIDFromContext(request.Context())
		agentID := strings.TrimSpace(request.PathValue("agent_id"))
		if !ok || !ownerPathIdentity(agentID) || len(request.URL.Query()) != 0 {
			writeJSON(response, http.StatusBadRequest, map[string]string{"code": "agent_rebind_invalid"})
			return
		}
		var input agentRebindRequest
		if decodeStrictOwnerJSON(response, request, &input) != nil || !validAgentRebindRequest(input) {
			writeJSON(response, http.StatusBadRequest, map[string]string{"code": "agent_rebind_invalid"})
			return
		}
		if repository == nil || resolver == nil || tokens == nil {
			writeJSON(response, http.StatusServiceUnavailable, map[string]string{"code": "agent_rebind_unavailable"})
			return
		}
		now := ownerClock(clock)
		if input.Action == "preview" {
			previewAgentRebind(response, request, repository, resolver, tokens, accountID, agentID, input, now)
			return
		}
		acceptAgentRebind(response, request, repository, resolver, tokens, accountID, agentID, input, now)
	})
}

func validAgentRebindRequest(input agentRebindRequest) bool {
	switch input.Action {
	case "preview":
		return input.IdempotencyKey == "" && input.AcceptanceToken == "" && ownerPathIdentity(input.OptionID) &&
			input.OptionID == strings.TrimSpace(input.OptionID) && ownerPathIdentity(input.ExpectedBindingRevisionID)
	case "accept":
		return ownerIntentString(input.IdempotencyKey, 256) && input.OptionID == "" && input.ExpectedBindingRevisionID == "" &&
			input.AcceptanceToken != "" && input.AcceptanceToken == strings.TrimSpace(input.AcceptanceToken) &&
			len([]byte(input.AcceptanceToken)) <= maximumRebindTokenBytes
	default:
		return false
	}
}

func previewAgentRebind(
	response http.ResponseWriter,
	request *http.Request,
	repository AgentRebindRepository,
	resolver AgentOptionResolver,
	tokens RebindAcceptanceTokens,
	accountID, agentID string,
	input agentRebindRequest,
	now time.Time,
) {
	current, err := ownedOpenAgent(request.Context(), repository, accountID, agentID)
	if err != nil {
		writeOwnerRepositoryError(response, err, "agent_rebind_unavailable")
		return
	}
	if current.Binding.ID != input.ExpectedBindingRevisionID {
		writeJSON(response, http.StatusConflict, map[string]string{"code": "state_conflict"})
		return
	}
	option, err := resolver.ResolveEligibleAgentOption(request.Context(), accountID, input.OptionID)
	if err != nil {
		writeAgentOptionError(response, err, "agent_rebind_unavailable")
		return
	}
	option = canonicalEligibleAgentOption(option)
	if err := validateEligibleAgentOption(accountID, input.OptionID, option, now); err != nil {
		writeJSON(response, http.StatusConflict, map[string]string{"code": "agent_option_ineligible"})
		return
	}
	command := buildAgentRebindProposal(accountID, agentID, input.OptionID, option, current, now)
	preview, err := repository.PreviewAgentRebind(request.Context(), command)
	if err != nil {
		writeOwnerRepositoryError(response, err, "agent_rebind_unavailable")
		return
	}
	grant := RebindAcceptanceGrant{
		Audience: rebindAcceptanceAudience, Route: rebindAcceptanceRoute, AccountID: accountID,
		AgentID: agentID, OptionID: input.OptionID, Preview: preview, IssuedAt: now,
	}
	token, expiresAt, err := tokens.Issue(grant)
	if err != nil {
		writeJSON(response, http.StatusServiceUnavailable, map[string]string{"code": "agent_rebind_unavailable"})
		return
	}
	writeBoundedOwnerJSON(response, agentRebindPreviewResponse{Preview: preview, AcceptanceToken: token, ExpiresAt: expiresAt})
}

func acceptAgentRebind(
	response http.ResponseWriter,
	request *http.Request,
	repository AgentRebindRepository,
	resolver AgentOptionResolver,
	tokens RebindAcceptanceTokens,
	accountID, agentID string,
	input agentRebindRequest,
	now time.Time,
) {
	grant, err := tokens.Verify(input.AcceptanceToken, now)
	if err != nil || grant.Audience != rebindAcceptanceAudience || grant.Route != rebindAcceptanceRoute ||
		grant.AccountID != accountID || grant.AgentID != agentID {
		writeJSON(response, http.StatusBadRequest, map[string]string{"code": "agent_rebind_token_invalid"})
		return
	}
	option, err := resolver.ResolveEligibleAgentOption(request.Context(), accountID, grant.OptionID)
	if err != nil {
		writeAgentOptionError(response, err, "agent_rebind_unavailable")
		return
	}
	option = canonicalEligibleAgentOption(option)
	if err := validateEligibleAgentOption(accountID, grant.OptionID, option, now); err != nil || !optionMatchesPreview(option, grant.Preview) {
		writeJSON(response, http.StatusConflict, map[string]string{"code": "agent_option_changed"})
		return
	}
	current, err := ownedOpenAgent(request.Context(), repository, accountID, agentID)
	if err != nil {
		writeOwnerRepositoryError(response, err, "agent_rebind_unavailable")
		return
	}
	switch current.Binding.ID {
	case grant.Preview.CurrentBinding.ID:
		proposal := proposalFromPreview(grant.Preview)
		fresh, previewErr := repository.PreviewAgentRebind(request.Context(), proposal)
		if previewErr != nil {
			writeOwnerRepositoryError(response, previewErr, "agent_rebind_unavailable")
			return
		}
		if fresh.Digest != grant.Preview.Digest {
			writeJSON(response, http.StatusConflict, map[string]string{"code": "agent_rebind_preview_stale"})
			return
		}
	case grant.Preview.ProposedBinding.ID:
		// Exact idempotent replay is delegated to the repository's claimed
		// command digest. A different key still conflicts there.
	default:
		writeJSON(response, http.StatusConflict, map[string]string{"code": "state_conflict"})
		return
	}
	acceptedAt := grant.Preview.ProposedBinding.ActivatedAt
	result, err := repository.AcceptAgentRebind(request.Context(), ledger.AcceptAgentRebindCommand{
		IdempotencyKey: input.IdempotencyKey, Preview: grant.Preview, AcceptedBy: "human:" + accountID, AcceptedAt: acceptedAt,
	})
	if err != nil {
		writeOwnerRepositoryError(response, err, "agent_rebind_unavailable")
		return
	}
	writeBoundedOwnerJSONStatus(response, http.StatusAccepted, result)
}

func ownedOpenAgent(ctx context.Context, repository AgentRebindRepository, accountID, agentID string) (ledger.AgentRecord, error) {
	record, err := repository.GetAgent(ctx, accountID, agentID)
	if err != nil {
		return ledger.AgentRecord{}, err
	}
	if record.Agent.AccountID != accountID || record.Agent.ID != agentID {
		return ledger.AgentRecord{}, ledger.ErrNotFound
	}
	if record.Agent.State != conversation.AgentOpen {
		return ledger.AgentRecord{}, ledger.ErrStateConflict
	}
	return record, nil
}

func buildAgentRebindProposal(accountID, agentID, optionID string, option EligibleAgentOption, current ledger.AgentRecord, now time.Time) ledger.PreviewAgentRebindCommand {
	seed := []string{accountID, agentID, current.Binding.ID, optionID}
	binding := materializeBinding(option.Binding, agentID, current.Behavior.ID, ownerCommandID("binding", seed...),
		ownerCommandID("seat", seed...), current.Binding.ID, current.Binding.Revision+1, now)
	return ledger.PreviewAgentRebindCommand{
		AccountID: accountID, AgentID: agentID, ExpectedBindingRevisionID: current.Binding.ID,
		Binding: binding, ExecutionSource: option.ExecutionSource, SourceAgent: option.SourceAgent,
		Participant: conversation.Participant{
			ID: ownerCommandID("participant", seed...), ConversationID: current.Home.ID, SeatID: binding.SeatID,
			Profile: binding.FortProfile, Agent: binding.Provider, Model: binding.RequestedModel,
			Machine: bindingLocation(binding), DisplayName: current.Profile.Name, Position: 0,
			State: conversation.ParticipantActive, CreatedAt: now,
		},
		NonTransferableResources: append([]ledger.RebindResource{}, option.NonTransferableResources...),
		ReadinessEvidence:        append([]string{}, option.ReadinessEvidence...), AuthorityEvidence: append([]string{}, option.AuthorityEvidence...),
		GeneratedAt: now,
	}
}

func proposalFromPreview(preview ledger.AgentRebindPreview) ledger.PreviewAgentRebindCommand {
	return ledger.PreviewAgentRebindCommand{
		AccountID: preview.AccountID, AgentID: preview.AgentID, ExpectedBindingRevisionID: preview.CurrentBinding.ID,
		Binding: preview.ProposedBinding, ExecutionSource: preview.ProposedExecutionSource, SourceAgent: preview.ProposedSourceAgent,
		Participant: preview.Participant, NonTransferableResources: append([]ledger.RebindResource{}, preview.NonTransferableResources...),
		ReadinessEvidence: append([]string{}, preview.ReadinessEvidence...), AuthorityEvidence: append([]string{}, preview.AuthorityEvidence...),
		GeneratedAt: preview.GeneratedAt,
	}
}

func materializeBinding(template conversation.AgentBindingRevision, agentID, behaviorID, bindingID, seatID, predecessorID string, revision int, activatedAt time.Time) conversation.AgentBindingRevision {
	binding := template
	binding.ID = bindingID
	binding.AgentID = agentID
	binding.Revision = revision
	binding.BehaviorRevisionID = behaviorID
	binding.SeatID = seatID
	binding.SupersedesRevisionID = predecessorID
	binding.ActivatedAt = activatedAt
	binding.RetiredAt = time.Time{}
	binding.CapabilityEvidence = append([]string{}, template.CapabilityEvidence...)
	return binding
}

func validateEligibleAgentOption(accountID, optionID string, option EligibleAgentOption, now time.Time) error {
	if option.ID != optionID || !ownerPathIdentity(option.ID) || option.ID != strings.TrimSpace(option.ID) {
		return fmt.Errorf("eligible option id does not match")
	}
	if err := option.ExecutionSource.Validate(); err != nil {
		return err
	}
	if err := option.SourceAgent.Validate(); err != nil {
		return err
	}
	if option.ExecutionSource.AccountID != accountID || option.SourceAgent.ExecutionSourceID != option.ExecutionSource.ID ||
		option.Binding.ExecutionSourceID != option.ExecutionSource.ID || option.Binding.SourceAgentID != option.SourceAgent.ID {
		return fmt.Errorf("eligible option source evidence does not match")
	}
	if option.Binding.ID != "" || option.Binding.AgentID != "" || option.Binding.Revision != 0 ||
		option.Binding.BehaviorRevisionID != "" || option.Binding.SeatID != "" || option.Binding.SupersedesRevisionID != "" ||
		!option.Binding.ActivatedAt.IsZero() || !option.Binding.RetiredAt.IsZero() {
		return fmt.Errorf("eligible option contains server-owned revision evidence")
	}
	probe := materializeBinding(option.Binding, "agent:probe", "behavior:probe", "binding:probe", "seat:probe", "", 1, now)
	if err := probe.Validate(); err != nil {
		return err
	}
	proposal := ledger.PreviewAgentRebindCommand{
		AccountID: accountID, AgentID: "agent:probe", ExpectedBindingRevisionID: "binding:old", Binding: probe,
		ExecutionSource: option.ExecutionSource, SourceAgent: option.SourceAgent,
		Participant: conversation.Participant{
			ID: "participant:probe", ConversationID: "conversation:probe", SeatID: probe.SeatID, Profile: probe.FortProfile,
			Agent: probe.Provider, Model: probe.RequestedModel, Machine: bindingLocation(probe), DisplayName: "Probe",
			State: conversation.ParticipantActive, CreatedAt: now,
		},
		NonTransferableResources: append([]ledger.RebindResource{}, option.NonTransferableResources...),
		ReadinessEvidence:        append([]string{}, option.ReadinessEvidence...), AuthorityEvidence: append([]string{}, option.AuthorityEvidence...), GeneratedAt: now,
	}
	proposal.Binding.SupersedesRevisionID = proposal.ExpectedBindingRevisionID
	return proposal.Validate()
}

func optionMatchesPreview(option EligibleAgentOption, preview ledger.AgentRebindPreview) bool {
	option = canonicalEligibleAgentOption(option)
	template := preview.ProposedBinding
	template.ID = ""
	template.AgentID = ""
	template.Revision = 0
	template.BehaviorRevisionID = ""
	template.SeatID = ""
	template.SupersedesRevisionID = ""
	template.ActivatedAt = time.Time{}
	template.RetiredAt = time.Time{}
	return reflect.DeepEqual(option.ExecutionSource, preview.ProposedExecutionSource) &&
		reflect.DeepEqual(option.SourceAgent, preview.ProposedSourceAgent) && reflect.DeepEqual(option.Binding, template) &&
		reflect.DeepEqual(option.NonTransferableResources, preview.NonTransferableResources) &&
		reflect.DeepEqual(option.ReadinessEvidence, preview.ReadinessEvidence) &&
		reflect.DeepEqual(option.AuthorityEvidence, preview.AuthorityEvidence)
}

func canonicalEligibleAgentOption(option EligibleAgentOption) EligibleAgentOption {
	option.Binding.CapabilityEvidence = append([]string{}, option.Binding.CapabilityEvidence...)
	sort.Strings(option.Binding.CapabilityEvidence)
	option.NonTransferableResources = append([]ledger.RebindResource{}, option.NonTransferableResources...)
	sort.Slice(option.NonTransferableResources, func(i, j int) bool {
		return option.NonTransferableResources[i] < option.NonTransferableResources[j]
	})
	option.ReadinessEvidence = append([]string{}, option.ReadinessEvidence...)
	sort.Strings(option.ReadinessEvidence)
	option.AuthorityEvidence = append([]string{}, option.AuthorityEvidence...)
	sort.Strings(option.AuthorityEvidence)
	return option
}

func writeAgentOptionError(response http.ResponseWriter, err error, unavailable string) {
	if errors.Is(err, ErrNoEligibleAgentOption) || errors.Is(err, ledger.ErrNotFound) {
		writeJSON(response, http.StatusConflict, map[string]string{"code": "agent_option_unavailable"})
		return
	}
	writeJSON(response, http.StatusServiceUnavailable, map[string]string{"code": unavailable})
}

// RebindAcceptanceGrant is the complete server-issued disclosure accepted by
// the human. It is never decoded from an unsigned request body.
type RebindAcceptanceGrant struct {
	Audience  string                    `json:"aud"`
	Route     string                    `json:"route"`
	AccountID string                    `json:"account_id"`
	AgentID   string                    `json:"agent_id"`
	OptionID  string                    `json:"option_id"`
	Preview   ledger.AgentRebindPreview `json:"preview"`
	IssuedAt  time.Time                 `json:"issued_at"`
	ExpiresAt time.Time                 `json:"expires_at"`
}

type RebindAcceptanceTokens interface {
	Issue(RebindAcceptanceGrant) (string, time.Time, error)
	Verify(string, time.Time) (RebindAcceptanceGrant, error)
}

type hmacRebindAcceptanceTokens struct {
	key []byte
	ttl time.Duration
}

func NewHMACRebindAcceptanceTokens(key []byte, ttl time.Duration) (RebindAcceptanceTokens, error) {
	if len(key) < 32 || ttl <= 0 || ttl > maximumRebindTokenTTL {
		return nil, fmt.Errorf("Rebind acceptance token key or TTL is invalid")
	}
	return &hmacRebindAcceptanceTokens{key: append([]byte{}, key...), ttl: ttl}, nil
}

// HMACRebindAcceptanceTokensFromEnvironment loads the dedicated, server-only
// Rebind acceptance key. It intentionally does not fall back to assertion,
// body-encryption, or client-session key material.
func HMACRebindAcceptanceTokensFromEnvironment(getenv func(string) string) (RebindAcceptanceTokens, error) {
	if getenv == nil {
		return nil, fmt.Errorf("Rebind acceptance token environment is unavailable")
	}
	encoded := strings.TrimSpace(getenv("FORT_REBIND_ACCEPTANCE_KEY_B64URL"))
	if encoded == "" {
		return nil, fmt.Errorf("FORT_REBIND_ACCEPTANCE_KEY_B64URL is required")
	}
	key, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || base64.RawURLEncoding.EncodeToString(key) != encoded {
		return nil, fmt.Errorf("FORT_REBIND_ACCEPTANCE_KEY_B64URL must be canonical base64url")
	}
	ttl := 2 * time.Minute
	if raw := strings.TrimSpace(getenv("FORT_REBIND_ACCEPTANCE_TTL_SECONDS")); raw != "" {
		seconds, parseErr := strconv.ParseInt(raw, 10, 64)
		if parseErr != nil || seconds < 1 || seconds > int64(maximumRebindTokenTTL/time.Second) {
			return nil, fmt.Errorf("FORT_REBIND_ACCEPTANCE_TTL_SECONDS is invalid")
		}
		ttl = time.Duration(seconds) * time.Second
	}
	return NewHMACRebindAcceptanceTokens(key, ttl)
}

func (tokens *hmacRebindAcceptanceTokens) Issue(grant RebindAcceptanceGrant) (string, time.Time, error) {
	if tokens == nil || grant.IssuedAt.IsZero() {
		return "", time.Time{}, fmt.Errorf("Rebind acceptance signer is unavailable")
	}
	grant.ExpiresAt = grant.IssuedAt.Add(tokens.ttl)
	if err := validateRebindAcceptanceGrant(grant, grant.IssuedAt); err != nil {
		return "", time.Time{}, err
	}
	payload, err := json.Marshal(grant)
	if err != nil {
		return "", time.Time{}, err
	}
	body := "v1." + base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, tokens.key)
	_, _ = mac.Write([]byte(body))
	token := body + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if len([]byte(token)) > maximumRebindTokenBytes {
		return "", time.Time{}, fmt.Errorf("Rebind acceptance token exceeds limit")
	}
	return token, grant.ExpiresAt, nil
}

func (tokens *hmacRebindAcceptanceTokens) Verify(token string, now time.Time) (RebindAcceptanceGrant, error) {
	if tokens == nil || len([]byte(token)) > maximumRebindTokenBytes {
		return RebindAcceptanceGrant{}, fmt.Errorf("invalid Rebind acceptance token")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] != "v1" {
		return RebindAcceptanceGrant{}, fmt.Errorf("invalid Rebind acceptance token")
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return RebindAcceptanceGrant{}, fmt.Errorf("invalid Rebind acceptance signature")
	}
	mac := hmac.New(sha256.New, tokens.key)
	_, _ = mac.Write([]byte(parts[0] + "." + parts[1]))
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return RebindAcceptanceGrant{}, fmt.Errorf("invalid Rebind acceptance signature")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return RebindAcceptanceGrant{}, fmt.Errorf("invalid Rebind acceptance payload")
	}
	decoder := json.NewDecoder(strings.NewReader(string(payload)))
	decoder.DisallowUnknownFields()
	var grant RebindAcceptanceGrant
	if err := decoder.Decode(&grant); err != nil {
		return RebindAcceptanceGrant{}, fmt.Errorf("invalid Rebind acceptance payload")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return RebindAcceptanceGrant{}, fmt.Errorf("invalid Rebind acceptance payload")
	}
	if err := validateRebindAcceptanceGrant(grant, now); err != nil {
		return RebindAcceptanceGrant{}, err
	}
	return grant, nil
}

func validateRebindAcceptanceGrant(grant RebindAcceptanceGrant, now time.Time) error {
	if grant.Audience != rebindAcceptanceAudience || grant.Route != rebindAcceptanceRoute ||
		!ownerPathIdentity(grant.AccountID) || !ownerPathIdentity(grant.AgentID) || !ownerPathIdentity(grant.OptionID) ||
		grant.Preview.AccountID != grant.AccountID || grant.Preview.AgentID != grant.AgentID || grant.IssuedAt.IsZero() || grant.ExpiresAt.IsZero() ||
		grant.ExpiresAt.Sub(grant.IssuedAt) <= 0 || grant.ExpiresAt.Sub(grant.IssuedAt) > maximumRebindTokenTTL ||
		now.Before(grant.IssuedAt) || !now.Before(grant.ExpiresAt) ||
		!grant.Preview.GeneratedAt.Equal(grant.IssuedAt) || !grant.Preview.ProposedBinding.ActivatedAt.Equal(grant.IssuedAt) {
		return fmt.Errorf("invalid or expired Rebind acceptance grant")
	}
	return grant.Preview.Validate()
}

func bindingLocation(binding conversation.AgentBindingRevision) string {
	if binding.ComputerID != "" {
		return binding.ComputerID
	}
	return binding.CloudRuntime
}
