// Package ledger defines Fort's durable control-plane persistence contracts.
// It depends on domain records, but not on SQLite, Postgres, or transports.
package ledger

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/tobsai/fort/core/conversation"
)

var (
	ErrNotFound            = errors.New("ledger resource not found")
	ErrIdempotencyConflict = errors.New("ledger idempotency key conflicts with an earlier command")
	ErrRevisionConflict    = errors.New("ledger current revision does not match the expected revision")
	ErrStateConflict       = errors.New("ledger resource is not in a valid state for the command")
	ErrSourceDrift         = errors.New("ledger execution source no longer matches the accepted binding")
)

// AgentRecord is the complete immutable evidence created with a stable Agent.
// Later commands may advance the Agent's current revision links, but never
// rewrite any revision returned here.
type AgentRecord struct {
	Agent           conversation.Agent                 `json:"agent"`
	Profile         conversation.AgentProfileRevision  `json:"profile"`
	Behavior        conversation.AgentBehaviorRevision `json:"behavior"`
	Binding         conversation.AgentBindingRevision  `json:"binding"`
	ExecutionSource conversation.ExecutionSource       `json:"execution_source"`
	SourceAgent     conversation.SourceAgent           `json:"source_agent"`
	Home            conversation.Conversation          `json:"home"`
	Participant     conversation.Participant           `json:"participant"`
	Link            conversation.AgentConversation     `json:"link"`
}

// CreateAgentCommand is the single atomic boundary for a new stable Agent and
// its permanent Home Conversation. IdempotencyKey is account-scoped.
type CreateAgentCommand struct {
	IdempotencyKey  string                             `json:"idempotency_key"`
	Agent           conversation.Agent                 `json:"agent"`
	Profile         conversation.AgentProfileRevision  `json:"profile"`
	Behavior        conversation.AgentBehaviorRevision `json:"behavior"`
	Binding         conversation.AgentBindingRevision  `json:"binding"`
	ExecutionSource conversation.ExecutionSource       `json:"execution_source"`
	SourceAgent     conversation.SourceAgent           `json:"source_agent"`
	Home            conversation.Conversation          `json:"home"`
	Participant     conversation.Participant           `json:"participant"`
	Link            conversation.AgentConversation     `json:"link"`
}

// AgentRepository is implemented by both local SQLite and the cloud ledger.
// Lists must return an allocated empty slice rather than nil.
type AgentRepository interface {
	CreateAgent(context.Context, CreateAgentCommand) (AgentRecord, error)
	GetAgent(context.Context, string, string) (AgentRecord, error)
	ListAgents(context.Context, string, conversation.AgentState) ([]AgentRecord, error)
	Close() error
}

// AgentLifecycleRepository adds explicit revision and Conversation lifecycle
// commands without widening the create/read contract used by migration code.
type AgentLifecycleRepository interface {
	AgentRepository
	AppendAgentProfile(context.Context, AppendAgentProfileCommand) (AgentRecord, error)
	AppendAgentBehavior(context.Context, AppendAgentBehaviorCommand) (AgentBindingAdvanceResult, error)
	PreviewAgentRebind(context.Context, PreviewAgentRebindCommand) (AgentRebindPreview, error)
	AcceptAgentRebind(context.Context, AcceptAgentRebindCommand) (AgentBindingAdvanceResult, error)
	CreateSecondaryConversation(context.Context, CreateSecondaryConversationCommand) (AgentConversationRecord, error)
	ListAgentConversations(context.Context, string, string) ([]AgentConversationRecord, error)
	RenameAgentConversation(context.Context, RenameAgentConversationCommand) (AgentConversationRecord, error)
	SetAgentConversationState(context.Context, SetAgentConversationStateCommand) (AgentConversationRecord, error)
	SetAgentConversationPin(context.Context, SetAgentConversationPinCommand) (AgentConversationRecord, error)
}

// AppendAgentProfileCommand appends one presentation-only revision and moves
// the Agent's current profile pointer if and only if the expected revision is
// still current. It cannot alter execution identity.
type AppendAgentProfileCommand struct {
	IdempotencyKey            string                            `json:"idempotency_key"`
	AccountID                 string                            `json:"account_id"`
	AgentID                   string                            `json:"agent_id"`
	ExpectedProfileRevisionID string                            `json:"expected_profile_revision_id"`
	Revision                  conversation.AgentProfileRevision `json:"revision"`
	AcceptedBy                string                            `json:"accepted_by"`
}

func (c AppendAgentProfileCommand) Validate() error {
	if err := validateLifecycleIdentity(c.IdempotencyKey, c.AccountID, c.AgentID, c.AcceptedBy); err != nil {
		return err
	}
	if strings.TrimSpace(c.ExpectedProfileRevisionID) == "" {
		return fmt.Errorf("expected Agent Profile Revision id is required")
	}
	if err := c.Revision.Validate(); err != nil {
		return err
	}
	if c.Revision.AgentID != c.AgentID {
		return fmt.Errorf("Agent Profile Revision belongs to another Agent")
	}
	if c.Revision.ID == c.ExpectedProfileRevisionID {
		return fmt.Errorf("successor Agent Profile Revision must have a new id")
	}
	return nil
}

func (c AppendAgentProfileCommand) Digest() (string, error) {
	canonical := c
	canonical.IdempotencyKey = ""
	return commandDigest(canonical)
}

type BindingTransitionKind string

const (
	BindingTransitionBehavior BindingTransitionKind = "behavior"
	BindingTransitionRebind   BindingTransitionKind = "rebind"
)

type RebindResource string

const (
	RebindResourceSourceMemory RebindResource = "source_managed_memory"
	RebindResourceSkills       RebindResource = "skills"
	RebindResourceSessions     RebindResource = "sessions"
	RebindResourceFiles        RebindResource = "files"
	RebindResourceToolState    RebindResource = "tool_state"
)

// AgentBindingTransition is immutable acceptance evidence. The predecessor
// Binding itself is never updated; retirement is represented by this record,
// the successor's SupersedesRevisionID, and an append-only ledger event.
type AgentBindingTransition struct {
	AccountID                   string                `json:"account_id"`
	AgentID                     string                `json:"agent_id"`
	Kind                        BindingTransitionKind `json:"kind"`
	PreviousBehaviorRevisionID  string                `json:"previous_behavior_revision_id"`
	SuccessorBehaviorRevisionID string                `json:"successor_behavior_revision_id"`
	PreviousBindingRevisionID   string                `json:"previous_binding_revision_id"`
	SuccessorBindingRevisionID  string                `json:"successor_binding_revision_id"`
	PreviewDigest               string                `json:"preview_digest"`
	NonTransferableResources    []RebindResource      `json:"non_transferable_resources"`
	ReadinessEvidence           []string              `json:"readiness_evidence"`
	AuthorityEvidence           []string              `json:"authority_evidence"`
	AcceptedBy                  string                `json:"accepted_by"`
	AcceptedAt                  time.Time             `json:"accepted_at"`
}

type AgentBindingAdvanceResult struct {
	Agent      AgentRecord            `json:"agent"`
	Transition AgentBindingTransition `json:"transition"`
}

// AppendAgentBehaviorCommand accepts immutable Fort-owned behavior and an
// otherwise identical successor Binding. Execution changes require Rebind.
type AppendAgentBehaviorCommand struct {
	IdempotencyKey             string                             `json:"idempotency_key"`
	AccountID                  string                             `json:"account_id"`
	AgentID                    string                             `json:"agent_id"`
	ExpectedBehaviorRevisionID string                             `json:"expected_behavior_revision_id"`
	ExpectedBindingRevisionID  string                             `json:"expected_binding_revision_id"`
	Behavior                   conversation.AgentBehaviorRevision `json:"behavior"`
	Binding                    conversation.AgentBindingRevision  `json:"binding"`
	Participant                conversation.Participant           `json:"participant"`
	ReadinessEvidence          []string                           `json:"readiness_evidence"`
	AuthorityEvidence          []string                           `json:"authority_evidence"`
	AcceptedBy                 string                             `json:"accepted_by"`
	AcceptedAt                 time.Time                          `json:"accepted_at"`
}

func (c AppendAgentBehaviorCommand) Validate() error {
	if err := validateLifecycleIdentity(c.IdempotencyKey, c.AccountID, c.AgentID, c.AcceptedBy); err != nil {
		return err
	}
	if strings.TrimSpace(c.ExpectedBehaviorRevisionID) == "" || strings.TrimSpace(c.ExpectedBindingRevisionID) == "" {
		return fmt.Errorf("expected Behavior and Binding Revision ids are required")
	}
	if err := c.Behavior.Validate(); err != nil {
		return err
	}
	if err := c.Binding.Validate(); err != nil {
		return err
	}
	if c.Behavior.AgentID != c.AgentID || c.Binding.AgentID != c.AgentID {
		return fmt.Errorf("Behavior or Binding Revision belongs to another Agent")
	}
	if c.Binding.BehaviorRevisionID != c.Behavior.ID || c.Binding.SupersedesRevisionID != c.ExpectedBindingRevisionID {
		return fmt.Errorf("successor Binding must reference the accepted Behavior and expected predecessor")
	}
	if !c.Binding.RetiredAt.IsZero() {
		return fmt.Errorf("successor Binding cannot already be retired")
	}
	if err := validateParticipant(c.Participant); err != nil {
		return err
	}
	if err := validateEvidenceList("readiness", c.ReadinessEvidence); err != nil {
		return err
	}
	if err := validateEvidenceList("authority", c.AuthorityEvidence); err != nil {
		return err
	}
	if c.AcceptedAt.IsZero() || !c.AcceptedAt.Equal(c.Binding.ActivatedAt) {
		return fmt.Errorf("Behavior acceptance time must equal successor Binding activation time")
	}
	return nil
}

func (c AppendAgentBehaviorCommand) Digest() (string, error) {
	canonical := c
	canonical.IdempotencyKey = ""
	canonical.Behavior.EnabledSkills = canonicalStrings(c.Behavior.EnabledSkills)
	canonical.Behavior.EnabledTools = canonicalStrings(c.Behavior.EnabledTools)
	canonical.Binding.CapabilityEvidence = canonicalStrings(c.Binding.CapabilityEvidence)
	canonical.ReadinessEvidence = canonicalStrings(c.ReadinessEvidence)
	canonical.AuthorityEvidence = canonicalStrings(c.AuthorityEvidence)
	return commandDigest(canonical)
}

type PreviewAgentRebindCommand struct {
	AccountID                 string                            `json:"account_id"`
	AgentID                   string                            `json:"agent_id"`
	ExpectedBindingRevisionID string                            `json:"expected_binding_revision_id"`
	Binding                   conversation.AgentBindingRevision `json:"binding"`
	ExecutionSource           conversation.ExecutionSource      `json:"execution_source"`
	SourceAgent               conversation.SourceAgent          `json:"source_agent"`
	Participant               conversation.Participant          `json:"participant"`
	NonTransferableResources  []RebindResource                  `json:"non_transferable_resources"`
	ReadinessEvidence         []string                          `json:"readiness_evidence"`
	AuthorityEvidence         []string                          `json:"authority_evidence"`
	GeneratedAt               time.Time                         `json:"generated_at"`
}

type AgentRebindPreview struct {
	AccountID                string                            `json:"account_id"`
	AgentID                  string                            `json:"agent_id"`
	CurrentBinding           conversation.AgentBindingRevision `json:"current_binding"`
	CurrentExecutionSource   conversation.ExecutionSource      `json:"current_execution_source"`
	CurrentSourceAgent       conversation.SourceAgent          `json:"current_source_agent"`
	ProposedBinding          conversation.AgentBindingRevision `json:"proposed_binding"`
	ProposedExecutionSource  conversation.ExecutionSource      `json:"proposed_execution_source"`
	ProposedSourceAgent      conversation.SourceAgent          `json:"proposed_source_agent"`
	Participant              conversation.Participant          `json:"participant"`
	NonTransferableResources []RebindResource                  `json:"non_transferable_resources"`
	ReadinessEvidence        []string                          `json:"readiness_evidence"`
	AuthorityEvidence        []string                          `json:"authority_evidence"`
	GeneratedAt              time.Time                         `json:"generated_at"`
	Digest                   string                            `json:"digest"`
}

type AcceptAgentRebindCommand struct {
	IdempotencyKey string             `json:"idempotency_key"`
	Preview        AgentRebindPreview `json:"preview"`
	AcceptedBy     string             `json:"accepted_by"`
	AcceptedAt     time.Time          `json:"accepted_at"`
}

type AgentConversationRecord struct {
	Conversation conversation.Conversation      `json:"conversation"`
	Link         conversation.AgentConversation `json:"link"`
	Pinned       bool                           `json:"pinned"`
	PinnedAt     time.Time                      `json:"pinned_at,omitempty"`
}

type CreateSecondaryConversationCommand struct {
	IdempotencyKey string                         `json:"idempotency_key"`
	AccountID      string                         `json:"account_id"`
	AgentID        string                         `json:"agent_id"`
	Conversation   conversation.Conversation      `json:"conversation"`
	Link           conversation.AgentConversation `json:"link"`
	CreatedBy      string                         `json:"created_by"`
}

// RenameAgentConversationCommand changes only the presentation title of one
// secondary Conversation. ExpectedTitle provides an optimistic concurrency
// boundary; the permanent canonical Home is rejected by repositories.
type RenameAgentConversationCommand struct {
	IdempotencyKey string    `json:"idempotency_key"`
	AccountID      string    `json:"account_id"`
	AgentID        string    `json:"agent_id"`
	ConversationID string    `json:"conversation_id"`
	ExpectedTitle  string    `json:"expected_title"`
	Title          string    `json:"title"`
	ChangedBy      string    `json:"changed_by"`
	ChangedAt      time.Time `json:"changed_at"`
}

type SetAgentConversationStateCommand struct {
	IdempotencyKey string                         `json:"idempotency_key"`
	AccountID      string                         `json:"account_id"`
	AgentID        string                         `json:"agent_id"`
	ConversationID string                         `json:"conversation_id"`
	ExpectedState  conversation.ConversationState `json:"expected_state"`
	State          conversation.ConversationState `json:"state"`
	ChangedBy      string                         `json:"changed_by"`
	ChangedAt      time.Time                      `json:"changed_at"`
}

type SetAgentConversationPinCommand struct {
	IdempotencyKey string    `json:"idempotency_key"`
	AccountID      string    `json:"account_id"`
	AgentID        string    `json:"agent_id"`
	ConversationID string    `json:"conversation_id"`
	ExpectedPinned bool      `json:"expected_pinned"`
	Pinned         bool      `json:"pinned"`
	ChangedBy      string    `json:"changed_by"`
	ChangedAt      time.Time `json:"changed_at"`
}

// Validate rejects an aggregate that could not be reconstructed without
// inference. In particular, every execution identity and the canonical Home
// evidence must agree before any durable row is written.
func (c CreateAgentCommand) Validate() error {
	if strings.TrimSpace(c.IdempotencyKey) == "" {
		return fmt.Errorf("Agent creation idempotency key is required")
	}
	if len([]byte(c.IdempotencyKey)) > 256 {
		return fmt.Errorf("Agent creation idempotency key exceeds 256 UTF-8 bytes")
	}
	if err := conversation.ValidateAgentRevisionSet(c.Agent, c.Profile, c.Behavior, c.Binding, []conversation.AgentConversation{c.Link}); err != nil {
		return err
	}
	if err := c.ExecutionSource.Validate(); err != nil {
		return err
	}
	if err := c.SourceAgent.Validate(); err != nil {
		return err
	}
	if c.ExecutionSource.AccountID != c.Agent.AccountID {
		return fmt.Errorf("Execution Source belongs to another account")
	}
	if c.Binding.ExecutionSourceID != c.ExecutionSource.ID ||
		c.SourceAgent.ExecutionSourceID != c.ExecutionSource.ID ||
		c.Binding.SourceAgentID != c.SourceAgent.ID {
		return fmt.Errorf("Agent Binding Revision source evidence does not match")
	}
	if strings.TrimSpace(c.Home.ID) == "" || strings.TrimSpace(c.Home.Title) == "" {
		return fmt.Errorf("Agent Home id and title are required")
	}
	if c.Home.State != conversation.ConversationOpen {
		return fmt.Errorf("Agent Home must be open")
	}
	if c.Home.CreatedAt.IsZero() || c.Home.UpdatedAt.IsZero() {
		return fmt.Errorf("Agent Home creation and update times are required")
	}
	if c.Home.ID != c.Agent.CanonicalConversationID || c.Link.ConversationID != c.Home.ID || c.Link.Kind != conversation.AgentConversationCanonical {
		return fmt.Errorf("Agent Home must be its canonical Conversation")
	}
	if err := validateParticipant(c.Participant); err != nil {
		return err
	}
	if c.Participant.ConversationID != c.Home.ID {
		return fmt.Errorf("Agent Home participant belongs to another Conversation")
	}
	if c.Participant.SeatID != c.Binding.SeatID ||
		c.Participant.Profile != c.Binding.FortProfile ||
		c.Participant.Agent != c.Binding.Provider ||
		c.Participant.Model != c.Binding.RequestedModel ||
		c.Participant.Machine != bindingLocation(c.Binding) ||
		c.Participant.DisplayName != c.Profile.Name {
		return fmt.Errorf("Agent Home participant does not match the accepted Binding Revision")
	}
	return nil
}

// Digest returns the lowercase SHA-256 digest used for idempotent replay. It
// excludes the key itself and canonicalizes set-like evidence slices, so two
// logically identical commands have one digest without mutating caller data.
func (c CreateAgentCommand) Digest() (string, error) {
	canonical := c
	canonical.IdempotencyKey = ""
	canonical.Behavior.EnabledSkills = canonicalStrings(c.Behavior.EnabledSkills)
	canonical.Behavior.EnabledTools = canonicalStrings(c.Behavior.EnabledTools)
	canonical.Binding.CapabilityEvidence = canonicalStrings(c.Binding.CapabilityEvidence)
	payload, err := json.Marshal(canonical)
	if err != nil {
		return "", fmt.Errorf("canonicalize Agent creation: %w", err)
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}

func canonicalStrings(values []string) []string {
	out := append([]string{}, values...)
	sort.Strings(out)
	return out
}

func validateParticipant(participant conversation.Participant) error {
	required := []struct {
		name  string
		value string
	}{
		{"id", participant.ID}, {"Conversation id", participant.ConversationID},
		{"seat id", participant.SeatID}, {"profile", participant.Profile},
		{"Agent", participant.Agent}, {"machine", participant.Machine},
		{"display name", participant.DisplayName},
	}
	for _, field := range required {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("Agent Home participant %s is required", field.name)
		}
	}
	if participant.Position < 0 {
		return fmt.Errorf("Agent Home participant position cannot be negative")
	}
	if participant.State != conversation.ParticipantActive || !participant.RemovedAt.IsZero() {
		return fmt.Errorf("Agent Home participant must be active")
	}
	if participant.CreatedAt.IsZero() {
		return fmt.Errorf("Agent Home participant creation time is required")
	}
	return nil
}

func bindingLocation(binding conversation.AgentBindingRevision) string {
	if binding.ComputerID != "" {
		return binding.ComputerID
	}
	return binding.CloudRuntime
}

func validateLifecycleIdentity(idempotencyKey, accountID, agentID, acceptedBy string) error {
	fields := []struct {
		name  string
		value string
	}{
		{"idempotency key", idempotencyKey},
		{"account id", accountID},
		{"Agent id", agentID},
		{"accepted by", acceptedBy},
	}
	for _, field := range fields {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("Agent lifecycle %s is required", field.name)
		}
	}
	if len([]byte(idempotencyKey)) > 256 {
		return fmt.Errorf("Agent lifecycle idempotency key exceeds 256 UTF-8 bytes")
	}
	return nil
}

func commandDigest(command any) (string, error) {
	payload, err := json.Marshal(command)
	if err != nil {
		return "", fmt.Errorf("canonicalize Agent lifecycle command: %w", err)
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}

func (c PreviewAgentRebindCommand) Validate() error {
	if strings.TrimSpace(c.AccountID) == "" || strings.TrimSpace(c.AgentID) == "" || strings.TrimSpace(c.ExpectedBindingRevisionID) == "" {
		return fmt.Errorf("Rebind account, Agent, and expected Binding Revision ids are required")
	}
	if err := c.Binding.Validate(); err != nil {
		return err
	}
	if err := c.ExecutionSource.Validate(); err != nil {
		return err
	}
	if err := c.SourceAgent.Validate(); err != nil {
		return err
	}
	if c.Binding.AgentID != c.AgentID || c.Binding.SupersedesRevisionID != c.ExpectedBindingRevisionID {
		return fmt.Errorf("proposed Binding does not supersede the expected Agent Binding")
	}
	if c.ExecutionSource.AccountID != c.AccountID || c.Binding.ExecutionSourceID != c.ExecutionSource.ID ||
		c.SourceAgent.ExecutionSourceID != c.ExecutionSource.ID || c.Binding.SourceAgentID != c.SourceAgent.ID {
		return fmt.Errorf("proposed Rebind source evidence does not match")
	}
	if !c.Binding.RetiredAt.IsZero() {
		return fmt.Errorf("proposed Binding cannot already be retired")
	}
	if err := validateParticipant(c.Participant); err != nil {
		return err
	}
	if c.Participant.SeatID != c.Binding.SeatID || c.Participant.Profile != c.Binding.FortProfile ||
		c.Participant.Agent != c.Binding.Provider || c.Participant.Model != c.Binding.RequestedModel ||
		c.Participant.Machine != bindingLocation(c.Binding) {
		return fmt.Errorf("proposed participant does not match the Binding")
	}
	if err := validateRebindResources(c.NonTransferableResources); err != nil {
		return err
	}
	if err := validateEvidenceList("readiness", c.ReadinessEvidence); err != nil {
		return err
	}
	if err := validateEvidenceList("authority", c.AuthorityEvidence); err != nil {
		return err
	}
	if c.GeneratedAt.IsZero() {
		return fmt.Errorf("Rebind preview generation time is required")
	}
	return nil
}

func (p AgentRebindPreview) CalculateDigest() (string, error) {
	canonical := p
	canonical.Digest = ""
	canonical.CurrentBinding.CapabilityEvidence = canonicalStrings(p.CurrentBinding.CapabilityEvidence)
	canonical.ProposedBinding.CapabilityEvidence = canonicalStrings(p.ProposedBinding.CapabilityEvidence)
	canonical.NonTransferableResources = canonicalRebindResources(p.NonTransferableResources)
	canonical.ReadinessEvidence = canonicalStrings(p.ReadinessEvidence)
	canonical.AuthorityEvidence = canonicalStrings(p.AuthorityEvidence)
	return commandDigest(canonical)
}

func (p AgentRebindPreview) Validate() error {
	if strings.TrimSpace(p.AccountID) == "" || strings.TrimSpace(p.AgentID) == "" {
		return fmt.Errorf("Rebind preview account and Agent ids are required")
	}
	if err := p.CurrentBinding.Validate(); err != nil {
		return err
	}
	if err := p.ProposedBinding.Validate(); err != nil {
		return err
	}
	if err := p.CurrentExecutionSource.Validate(); err != nil {
		return err
	}
	if err := p.ProposedExecutionSource.Validate(); err != nil {
		return err
	}
	if err := p.CurrentSourceAgent.Validate(); err != nil {
		return err
	}
	if err := p.ProposedSourceAgent.Validate(); err != nil {
		return err
	}
	if err := validateParticipant(p.Participant); err != nil {
		return err
	}
	if p.CurrentBinding.AgentID != p.AgentID || p.ProposedBinding.AgentID != p.AgentID ||
		p.ProposedBinding.SupersedesRevisionID != p.CurrentBinding.ID {
		return fmt.Errorf("Rebind preview identities do not form one successor chain")
	}
	if p.CurrentExecutionSource.AccountID != p.AccountID || p.ProposedExecutionSource.AccountID != p.AccountID ||
		p.CurrentBinding.ExecutionSourceID != p.CurrentExecutionSource.ID ||
		p.ProposedBinding.ExecutionSourceID != p.ProposedExecutionSource.ID ||
		p.CurrentBinding.SourceAgentID != p.CurrentSourceAgent.ID ||
		p.ProposedBinding.SourceAgentID != p.ProposedSourceAgent.ID ||
		p.CurrentSourceAgent.ExecutionSourceID != p.CurrentExecutionSource.ID ||
		p.ProposedSourceAgent.ExecutionSourceID != p.ProposedExecutionSource.ID {
		return fmt.Errorf("Rebind preview source evidence does not match")
	}
	if p.Participant.SeatID != p.ProposedBinding.SeatID || p.Participant.Profile != p.ProposedBinding.FortProfile ||
		p.Participant.Agent != p.ProposedBinding.Provider || p.Participant.Model != p.ProposedBinding.RequestedModel ||
		p.Participant.Machine != bindingLocation(p.ProposedBinding) {
		return fmt.Errorf("Rebind preview participant does not match the proposed Binding")
	}
	if err := validateRebindResources(p.NonTransferableResources); err != nil {
		return err
	}
	if err := validateEvidenceList("readiness", p.ReadinessEvidence); err != nil {
		return err
	}
	if err := validateEvidenceList("authority", p.AuthorityEvidence); err != nil {
		return err
	}
	if p.GeneratedAt.IsZero() || !isLowerSHA256Digest(p.Digest) {
		return fmt.Errorf("Rebind preview generation time and digest are required")
	}
	digest, err := p.CalculateDigest()
	if err != nil {
		return err
	}
	if digest != p.Digest {
		return fmt.Errorf("Rebind preview digest does not match its evidence")
	}
	return nil
}

func (c AcceptAgentRebindCommand) Validate() error {
	if err := validateLifecycleIdentity(c.IdempotencyKey, c.Preview.AccountID, c.Preview.AgentID, c.AcceptedBy); err != nil {
		return err
	}
	if err := c.Preview.Validate(); err != nil {
		return err
	}
	if c.AcceptedAt.IsZero() || !c.AcceptedAt.Equal(c.Preview.ProposedBinding.ActivatedAt) {
		return fmt.Errorf("Rebind acceptance time must equal successor Binding activation time")
	}
	return nil
}

func (c AcceptAgentRebindCommand) Digest() (string, error) {
	canonical := c
	canonical.IdempotencyKey = ""
	return commandDigest(canonical)
}

func (c CreateSecondaryConversationCommand) Validate() error {
	if err := validateLifecycleIdentity(c.IdempotencyKey, c.AccountID, c.AgentID, c.CreatedBy); err != nil {
		return err
	}
	if strings.TrimSpace(c.Conversation.ID) == "" || strings.TrimSpace(c.Conversation.Title) == "" ||
		c.Conversation.State != conversation.ConversationOpen || c.Conversation.CreatedAt.IsZero() || c.Conversation.UpdatedAt.IsZero() {
		return fmt.Errorf("secondary Conversation must have id, title, open state, and timestamps")
	}
	if err := c.Link.Validate(); err != nil {
		return err
	}
	if c.Link.AgentID != c.AgentID || c.Link.ConversationID != c.Conversation.ID || c.Link.Kind != conversation.AgentConversationSecondary {
		return fmt.Errorf("secondary Conversation link does not match its Agent and Conversation")
	}
	return nil
}

func (c CreateSecondaryConversationCommand) Digest() (string, error) {
	canonical := c
	canonical.IdempotencyKey = ""
	canonical.Conversation.ID = ""
	canonical.Conversation.CreatedAt = time.Time{}
	canonical.Conversation.UpdatedAt = time.Time{}
	canonical.Link.ConversationID = ""
	canonical.Link.CreatedAt = time.Time{}
	return commandDigest(canonical)
}

func (c RenameAgentConversationCommand) Validate() error {
	if err := validateLifecycleIdentity(c.IdempotencyKey, c.AccountID, c.AgentID, c.ChangedBy); err != nil {
		return err
	}
	if strings.TrimSpace(c.ConversationID) == "" || c.ChangedAt.IsZero() {
		return fmt.Errorf("Conversation id and rename time are required")
	}
	if strings.TrimSpace(c.ExpectedTitle) == "" || strings.TrimSpace(c.Title) == "" ||
		c.ExpectedTitle != strings.TrimSpace(c.ExpectedTitle) || c.Title != strings.TrimSpace(c.Title) ||
		len([]byte(c.ExpectedTitle)) > 512 || len([]byte(c.Title)) > 512 || c.ExpectedTitle == c.Title {
		return fmt.Errorf("Conversation rename requires distinct normalized titles no greater than 512 UTF-8 bytes")
	}
	return nil
}

func (c RenameAgentConversationCommand) Digest() (string, error) {
	canonical := c
	canonical.IdempotencyKey = ""
	canonical.ChangedAt = time.Time{}
	return commandDigest(canonical)
}

func (c SetAgentConversationStateCommand) Validate() error {
	if err := validateLifecycleIdentity(c.IdempotencyKey, c.AccountID, c.AgentID, c.ChangedBy); err != nil {
		return err
	}
	if strings.TrimSpace(c.ConversationID) == "" || c.ChangedAt.IsZero() {
		return fmt.Errorf("Conversation id and state-change time are required")
	}
	if !validConversationState(c.ExpectedState) || !validConversationState(c.State) || c.ExpectedState == c.State {
		return fmt.Errorf("Conversation state command requires distinct open and archived states")
	}
	return nil
}

func (c SetAgentConversationStateCommand) Digest() (string, error) {
	canonical := c
	canonical.IdempotencyKey = ""
	canonical.ChangedAt = time.Time{}
	return commandDigest(canonical)
}

func (c SetAgentConversationPinCommand) Validate() error {
	if err := validateLifecycleIdentity(c.IdempotencyKey, c.AccountID, c.AgentID, c.ChangedBy); err != nil {
		return err
	}
	if strings.TrimSpace(c.ConversationID) == "" || c.ChangedAt.IsZero() {
		return fmt.Errorf("Conversation id and pin-change time are required")
	}
	if c.ExpectedPinned == c.Pinned {
		return fmt.Errorf("Conversation pin command requires a state change")
	}
	return nil
}

func (c SetAgentConversationPinCommand) Digest() (string, error) {
	canonical := c
	canonical.IdempotencyKey = ""
	canonical.ChangedAt = time.Time{}
	return commandDigest(canonical)
}

func validateEvidenceList(subject string, values []string) error {
	if len(values) == 0 {
		return fmt.Errorf("%s evidence is required", subject)
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) == "" || strings.TrimSpace(value) != value {
			return fmt.Errorf("%s evidence must be normalized", subject)
		}
		if _, exists := seen[value]; exists {
			return fmt.Errorf("%s evidence is duplicated", subject)
		}
		seen[value] = struct{}{}
	}
	return nil
}

func validateRebindResources(resources []RebindResource) error {
	if resources == nil {
		return fmt.Errorf("Rebind non-transferable resources must be an allocated list")
	}
	seen := make(map[RebindResource]struct{}, len(resources))
	for _, resource := range resources {
		switch resource {
		case RebindResourceSourceMemory, RebindResourceSkills, RebindResourceSessions, RebindResourceFiles, RebindResourceToolState:
		default:
			return fmt.Errorf("Rebind resource %q is invalid", resource)
		}
		if _, exists := seen[resource]; exists {
			return fmt.Errorf("Rebind resource %q is duplicated", resource)
		}
		seen[resource] = struct{}{}
	}
	return nil
}

func canonicalRebindResources(resources []RebindResource) []RebindResource {
	out := append([]RebindResource{}, resources...)
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func validConversationState(state conversation.ConversationState) bool {
	return state == conversation.ConversationOpen || state == conversation.ConversationArchived
}

func isLowerSHA256Digest(value string) bool {
	if len(value) != 64 || strings.ToLower(value) != value {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size
}
