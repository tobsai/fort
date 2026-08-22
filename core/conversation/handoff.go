package conversation

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

var ErrHandoffNeedsYou = errors.New("Handoff needs human attention")

// AuthorityGrant is one immutable delegation or policy layer. Permissions are
// opaque Fort-owned capability identifiers; no layer can add a permission
// missing from another applicable layer.
type AuthorityGrant struct {
	ID               string   `json:"id"`
	Permissions      []string `json:"permissions"`
	ContextRecordIDs []string `json:"context_record_ids"`
}

func (g AuthorityGrant) Validate() error {
	if strings.TrimSpace(g.ID) == "" {
		return fmt.Errorf("authority grant id is required")
	}
	if err := requireUniqueValues("authority permission", g.Permissions); err != nil {
		return err
	}
	return requireUniqueValues("authority context record id", g.ContextRecordIDs)
}

// ComputeEffectiveAuthority intersects every persisted authority layer and
// rejects requested permissions outside the resulting grant.
func ComputeEffectiveAuthority(requested []string, layers ...AuthorityGrant) (AuthorityGrant, error) {
	if len(layers) == 0 {
		return AuthorityGrant{}, fmt.Errorf("at least one authority layer is required")
	}
	if err := requireUniqueValues("requested authority permission", requested); err != nil {
		return AuthorityGrant{}, err
	}
	remaining := make(map[string]struct{}, len(layers[0].Permissions))
	for _, permission := range layers[0].Permissions {
		remaining[permission] = struct{}{}
	}
	for _, layer := range layers {
		if err := layer.Validate(); err != nil {
			return AuthorityGrant{}, err
		}
		allowed := make(map[string]struct{}, len(layer.Permissions))
		for _, permission := range layer.Permissions {
			allowed[permission] = struct{}{}
		}
		for permission := range remaining {
			if _, exists := allowed[permission]; !exists {
				delete(remaining, permission)
			}
		}
	}
	for _, permission := range requested {
		if _, exists := remaining[permission]; !exists {
			return AuthorityGrant{}, fmt.Errorf("requested authority permission %q exceeds the effective delegation", permission)
		}
	}
	permissions := append([]string(nil), requested...)
	sort.Strings(permissions)
	return AuthorityGrant{ID: "effective", Permissions: permissions}, nil
}

const MaxHandoffContextReferences = 64

type ContextReferenceKind string

const (
	ContextMessage        ContextReferenceKind = "message"
	ContextArtifact       ContextReferenceKind = "context_artifact"
	ContextOutputArtifact ContextReferenceKind = "output_artifact"
)

type ContextReference struct {
	Kind           ContextReferenceKind `json:"kind"`
	ID             string               `json:"id"`
	AccountID      string               `json:"account_id"`
	Immutable      bool                 `json:"immutable"`
	Finalized      bool                 `json:"finalized,omitempty"`
	Digest         string               `json:"digest,omitempty"`
	ObservedDigest string               `json:"observed_digest,omitempty"`
	Size           int64                `json:"size,omitempty"`
	ObservedSize   int64                `json:"observed_size,omitempty"`
}

type ContextManifest struct {
	References []ContextReference `json:"references"`
}

func (r ContextReference) Key() string {
	return string(r.Kind) + ":" + r.ID
}

// ValidateContextManifest accepts only immutable Fort messages and finalized,
// digest-verified Fort artifacts explicitly authorized by the root grant.
func ValidateContextManifest(accountID string, rootGrant AuthorityGrant, manifest ContextManifest) error {
	if strings.TrimSpace(accountID) == "" {
		return fmt.Errorf("context account id is required")
	}
	if err := rootGrant.Validate(); err != nil {
		return err
	}
	if len(manifest.References) > MaxHandoffContextReferences {
		return fmt.Errorf("Handoff context exceeds %d references", MaxHandoffContextReferences)
	}
	allowed := make(map[string]struct{}, len(rootGrant.ContextRecordIDs))
	for _, id := range rootGrant.ContextRecordIDs {
		allowed[id] = struct{}{}
	}
	seen := make(map[string]struct{}, len(manifest.References))
	for _, reference := range manifest.References {
		if err := requireStrings("Handoff context reference", map[string]string{
			"id": reference.ID, "account id": reference.AccountID,
		}); err != nil {
			return err
		}
		if reference.Kind != ContextMessage && reference.Kind != ContextArtifact && reference.Kind != ContextOutputArtifact {
			return fmt.Errorf("Handoff context reference kind %q is not a Fort record", reference.Kind)
		}
		key := reference.Key()
		if _, exists := seen[key]; exists {
			return fmt.Errorf("Handoff context reference %q is duplicated", key)
		}
		seen[key] = struct{}{}
		if reference.AccountID != accountID {
			return fmt.Errorf("Handoff context reference %q belongs to another account", key)
		}
		if _, exists := allowed[key]; !exists {
			return fmt.Errorf("Handoff context reference %q is not authorized by the root delegation grant", key)
		}
		if !reference.Immutable {
			return fmt.Errorf("Handoff context reference %q is mutable", key)
		}
		if reference.Kind == ContextMessage {
			continue
		}
		if !reference.Finalized || !isLowerSHA256(reference.Digest) || reference.ObservedDigest != reference.Digest ||
			reference.Size < 0 || reference.ObservedSize != reference.Size {
			return fmt.Errorf("Handoff artifact %q is not finalized with matching digest and size", key)
		}
	}
	return nil
}

type HandoffActorKind string

const (
	HandoffActorHuman HandoffActorKind = "human"
	HandoffActorAgent HandoffActorKind = "agent"
)

type HandoffState string

const (
	HandoffQueued    HandoffState = "queued"
	HandoffNeedsYou  HandoffState = "needs_you"
	HandoffWorking   HandoffState = "working"
	HandoffCompleted HandoffState = "completed"
	HandoffFailed    HandoffState = "failed"
	HandoffCanceled  HandoffState = "canceled"
)

type Handoff struct {
	ID                          string              `json:"id"`
	AccountID                   string              `json:"account_id"`
	IdempotencyKey              string              `json:"idempotency_key"`
	State                       HandoffState        `json:"state"`
	CreatedByKind               HandoffActorKind    `json:"created_by_kind"`
	CreatedByID                 string              `json:"created_by_id"`
	GroupTurnID                 string              `json:"group_turn_id,omitempty"`
	SourceMessageID             string              `json:"source_message_id"`
	SourceAgentID               string              `json:"source_agent_id,omitempty"`
	SourceBehaviorRevisionID    string              `json:"source_behavior_revision_id,omitempty"`
	SourceBindingRevisionID     string              `json:"source_binding_revision_id,omitempty"`
	RecipientAgentID            string              `json:"recipient_agent_id"`
	RecipientBehaviorRevisionID string              `json:"recipient_behavior_revision_id"`
	RecipientBindingRevisionID  string              `json:"recipient_binding_revision_id"`
	SourceConversationID        string              `json:"source_conversation_id"`
	OutputConversationID        string              `json:"output_conversation_id"`
	Context                     ContextManifest     `json:"context"`
	RequestedResult             string              `json:"requested_result"`
	ReplyToMessageID            string              `json:"reply_to_message_id,omitempty"`
	RootDelegationGrant         AuthorityGrant      `json:"root_delegation_grant"`
	ParentStageAuthority        *AuthorityGrant     `json:"parent_stage_authority,omitempty"`
	HandoffPolicy               AuthorityGrant      `json:"handoff_policy"`
	RecipientBindingPolicy      AuthorityGrant      `json:"recipient_binding_policy"`
	EmitterRequest              *AuthorityGrant     `json:"emitter_request,omitempty"`
	ApprovalRequired            bool                `json:"approval_required"`
	ApprovalReceipt             *AuthorityGrant     `json:"approval_receipt,omitempty"`
	RequestedAuthority          []string            `json:"requested_authority"`
	EffectiveAuthority          AuthorityGrant      `json:"effective_authority"`
	StructuredEmitterID         string              `json:"structured_emitter_id,omitempty"`
	BudgetClass                 LimitClassification `json:"budget_class"`
	BudgetLimitEvidenceID       string              `json:"budget_limit_evidence_id,omitempty"`
	MaxAgentMessages            int                 `json:"max_agent_messages"`
	MaxDepth                    int                 `json:"max_depth"`
	Depth                       int                 `json:"depth"`
	Deadline                    time.Time           `json:"deadline"`
	AncestorAgentIDs            []string            `json:"ancestor_agent_ids"`
	ParentHandoffID             string              `json:"parent_handoff_id,omitempty"`
	CreatedAt                   time.Time           `json:"created_at"`
}

func (h Handoff) Validate() error {
	if err := requireStrings("Handoff", map[string]string{
		"id": h.ID, "account id": h.AccountID, "idempotency key": h.IdempotencyKey,
		"creation actor id": h.CreatedByID, "source message id": h.SourceMessageID,
		"recipient Agent id": h.RecipientAgentID, "recipient Behavior Revision id": h.RecipientBehaviorRevisionID,
		"recipient Binding Revision id": h.RecipientBindingRevisionID, "source Conversation id": h.SourceConversationID,
		"output Conversation id": h.OutputConversationID, "requested result": h.RequestedResult,
	}); err != nil {
		return err
	}
	if !h.State.Valid() {
		return fmt.Errorf("Handoff state is invalid")
	}
	if h.CreatedByKind != HandoffActorHuman && h.CreatedByKind != HandoffActorAgent {
		return fmt.Errorf("Handoff creation actor must be human or Agent")
	}
	if h.CreatedByKind == HandoffActorAgent {
		if err := requireStrings("Agent-initiated Handoff", map[string]string{
			"source Agent id": h.SourceAgentID, "source Behavior Revision id": h.SourceBehaviorRevisionID,
			"source Binding Revision id": h.SourceBindingRevisionID, "structured emitter id": h.StructuredEmitterID,
		}); err != nil {
			return err
		}
		if h.EmitterRequest == nil {
			return fmt.Errorf("Agent-initiated Handoff requires a structured emitter request")
		}
		if h.ParentStageAuthority == nil {
			return fmt.Errorf("Agent-initiated Handoff requires its source stage authority")
		}
	}
	if h.SourceAgentID != "" && h.SourceAgentID == h.RecipientAgentID {
		return fmt.Errorf("self-Handoffs are not allowed")
	}
	if h.GroupTurnID != "" && h.OutputConversationID != h.SourceConversationID {
		return fmt.Errorf("Group Handoff output must remain in its Group Conversation")
	}
	if h.MaxAgentMessages != MaxGroupAgentMessages || h.MaxDepth != MaxGroupHandoffDepth {
		return fmt.Errorf("Handoff requires the first-release ten-message and depth-three limits")
	}
	if h.Depth < 1 || h.Depth > h.MaxDepth {
		return fmt.Errorf("Handoff depth exceeds its persisted limit")
	}
	if h.Depth == 1 && h.ParentHandoffID != "" {
		return fmt.Errorf("root Handoff cannot name a parent Handoff")
	}
	if h.Depth > 1 && strings.TrimSpace(h.ParentHandoffID) == "" {
		return fmt.Errorf("nested Handoff requires its parent Handoff id")
	}
	if !h.BudgetClass.Valid() {
		return fmt.Errorf("Handoff budget classification must be hard or unknown")
	}
	if h.BudgetClass == LimitHard && strings.TrimSpace(h.BudgetLimitEvidenceID) == "" {
		return fmt.Errorf("hard Handoff budget requires pre-start enforceability evidence")
	}
	if h.CreatedAt.IsZero() || !h.Deadline.After(h.CreatedAt) {
		return fmt.Errorf("Handoff requires a hard deadline after creation")
	}
	if err := h.validateCausalPath(); err != nil {
		return err
	}
	if err := ValidateContextManifest(h.AccountID, h.RootDelegationGrant, h.Context); err != nil {
		return err
	}
	layers := []AuthorityGrant{h.RootDelegationGrant}
	if h.ParentStageAuthority != nil {
		layers = append(layers, *h.ParentStageAuthority)
	}
	layers = append(layers, h.HandoffPolicy, h.RecipientBindingPolicy)
	if h.EmitterRequest != nil {
		layers = append(layers, *h.EmitterRequest)
	}
	if h.ApprovalReceipt != nil {
		layers = append(layers, *h.ApprovalReceipt)
	}
	effective, err := ComputeEffectiveAuthority(h.RequestedAuthority, layers...)
	if err != nil {
		return err
	}
	if h.EffectiveAuthority.ID != effective.ID || !equalStrings(h.EffectiveAuthority.Permissions, effective.Permissions) {
		return fmt.Errorf("Handoff effective authority does not match the persisted delegation intersection")
	}
	return nil
}

func (s HandoffState) Valid() bool {
	switch s {
	case HandoffQueued, HandoffNeedsYou, HandoffWorking, HandoffCompleted, HandoffFailed, HandoffCanceled:
		return true
	default:
		return false
	}
}

func (h Handoff) validateCausalPath() error {
	seen := make(map[string]struct{}, len(h.AncestorAgentIDs))
	for _, agentID := range h.AncestorAgentIDs {
		if strings.TrimSpace(agentID) == "" {
			return fmt.Errorf("Handoff causal path contains a blank Agent id")
		}
		if _, exists := seen[agentID]; exists {
			return fmt.Errorf("Handoff causal path contains a cycle")
		}
		seen[agentID] = struct{}{}
		if agentID == h.RecipientAgentID {
			return fmt.Errorf("Handoff recipient creates a causal cycle")
		}
	}
	if h.CreatedByKind == HandoffActorAgent {
		if len(h.AncestorAgentIDs) == 0 || h.AncestorAgentIDs[len(h.AncestorAgentIDs)-1] != h.SourceAgentID {
			return fmt.Errorf("Agent-initiated Handoff causal path must end at its source Agent")
		}
	}
	return nil
}

// CanStart applies the shared hard message/depth/deadline bounds immediately
// before provider start. Exhaustion is actionable rather than silently ignored.
func (h Handoff) CanStart(now time.Time, agentMessages int) error {
	if err := h.Validate(); err != nil {
		return err
	}
	if h.State != HandoffQueued {
		return fmt.Errorf("Handoff must be queued before provider start")
	}
	if h.ApprovalRequired && h.ApprovalReceipt == nil {
		return fmt.Errorf("%w: Handoff requires an approval receipt", ErrHandoffNeedsYou)
	}
	if agentMessages < 0 {
		return fmt.Errorf("Handoff Agent message count cannot be negative")
	}
	if !now.Before(h.Deadline) || agentMessages >= h.MaxAgentMessages {
		return fmt.Errorf("%w: Handoff turn limit exhausted", ErrHandoffNeedsYou)
	}
	return nil
}

type HandoffResult struct {
	HandoffID            string `json:"handoff_id"`
	OutputConversationID string `json:"output_conversation_id"`
	MessageID            string `json:"message_id"`
	Body                 string `json:"body"`
}

func (r HandoffResult) ValidateFor(h Handoff) error {
	if err := requireStrings("Handoff result", map[string]string{
		"Handoff id": r.HandoffID, "output Conversation id": r.OutputConversationID,
		"message id": r.MessageID, "body": r.Body,
	}); err != nil {
		return err
	}
	if r.HandoffID != h.ID || r.OutputConversationID != h.OutputConversationID {
		return fmt.Errorf("Handoff result must use the one persisted output Conversation")
	}
	if h.State != HandoffCompleted {
		return fmt.Errorf("only a successfully completed Handoff may create an authoritative result message")
	}
	return nil
}

// HandoffProjection is reference-only by construction: it contains linkage to
// the one authoritative message but no result or context body field.
type HandoffProjection struct {
	HandoffID              string       `json:"handoff_id"`
	ConversationID         string       `json:"conversation_id"`
	OutputConversationID   string       `json:"output_conversation_id"`
	AuthoritativeMessageID string       `json:"authoritative_message_id"`
	State                  HandoffState `json:"state"`
}

func (h Handoff) ReferenceProjection(conversationID string, result HandoffResult) (HandoffProjection, error) {
	if err := h.Validate(); err != nil {
		return HandoffProjection{}, err
	}
	if err := result.ValidateFor(h); err != nil {
		return HandoffProjection{}, err
	}
	if strings.TrimSpace(conversationID) == "" || conversationID == h.OutputConversationID {
		return HandoffProjection{}, fmt.Errorf("reference-only Handoff projection requires another affected Conversation")
	}
	return HandoffProjection{
		HandoffID: h.ID, ConversationID: conversationID, OutputConversationID: h.OutputConversationID,
		AuthoritativeMessageID: result.MessageID, State: h.State,
	}, nil
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
