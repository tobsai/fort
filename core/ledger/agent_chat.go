package ledger

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/tobsai/fort/core/conversation"
)

const MaxAgentMessageBytes = 2 << 20

// AgentDirectChatRepository is the narrow stable-Agent chat ledger seam. The
// caller identifies only Fort-owned records; execution identity is resolved
// from the Agent's accepted revisions inside the write transaction.
type AgentDirectChatRepository interface {
	SendAgentTurn(context.Context, SendAgentTurnCommand) (AgentTurnDispatch, error)
	RetryAgentTarget(context.Context, RetryAgentTargetCommand) (AgentConversationTarget, error)
	CancelAgentTarget(context.Context, CancelAgentTargetCommand) (AgentConversationTarget, error)
	ReadAgentConversation(context.Context, string, string, string) (AgentConversationProjection, error)
}

// ExecutionSourceConfigObservationRepository stores source-neutral inventory
// evidence separately from immutable accepted Bindings. Dispatch compares the
// latest append to the exact Binding pin and fails closed on drift.
type ExecutionSourceConfigObservationRepository interface {
	ObserveExecutionSourceConfig(context.Context, ObserveExecutionSourceConfigCommand) (ExecutionSourceConfigObservation, error)
	LatestExecutionSourceConfigObservation(context.Context, string, string) (ExecutionSourceConfigObservation, error)
}

// ObserveExecutionSourceConfigCommand accepts only Fort-owned opaque source
// identity and a digest. Provider, model, and machine selection are not part
// of this evidence command.
type ObserveExecutionSourceConfigCommand struct {
	IdempotencyKey     string    `json:"idempotency_key"`
	ObservationID      string    `json:"observation_id"`
	AccountID          string    `json:"account_id"`
	ExecutionSourceID  string    `json:"execution_source_id"`
	SourceConfigDigest string    `json:"source_config_digest"`
	ObservedBy         string    `json:"observed_by"`
	ObservedAt         time.Time `json:"observed_at"`
}

func (command ObserveExecutionSourceConfigCommand) Validate() error {
	for label, value := range map[string]string{
		"idempotency key": command.IdempotencyKey, "observation id": command.ObservationID,
		"account id": command.AccountID, "Execution Source id": command.ExecutionSourceID,
		"observer": command.ObservedBy,
	} {
		if strings.TrimSpace(value) == "" || strings.TrimSpace(value) != value {
			return fmt.Errorf("Execution Source configuration observation %s is required and must be canonical", label)
		}
	}
	if !isLowerSHA256Digest(command.SourceConfigDigest) {
		return fmt.Errorf("Execution Source configuration observation digest must be lowercase SHA-256")
	}
	if command.ObservedAt.IsZero() {
		return fmt.Errorf("Execution Source configuration observation time is required")
	}
	return nil
}

func (command ObserveExecutionSourceConfigCommand) Digest() (string, error) {
	canonical := command
	canonical.IdempotencyKey = ""
	// Observation identity and receipt time are server allocated. A replay on
	// another control instance may regenerate both and must return the original.
	canonical.ObservationID = ""
	canonical.ObservedAt = time.Time{}
	return commandDigest(canonical)
}

type ExecutionSourceConfigObservation struct {
	ID                 string    `json:"id"`
	Sequence           int64     `json:"sequence"`
	AccountID          string    `json:"account_id"`
	ExecutionSourceID  string    `json:"execution_source_id"`
	SourceConfigDigest string    `json:"source_config_digest"`
	ObservedBy         string    `json:"observed_by"`
	ObservedAt         time.Time `json:"observed_at"`
}

// SendAgentTurnCommand contains no provider, model, machine, participant, or
// revision choice. Those are deliberately not client-selectable on ordinary
// Send and are pinned from the current Agent aggregate by the repository.
type SendAgentTurnCommand struct {
	IdempotencyKey    string    `json:"idempotency_key"`
	AccountID         string    `json:"account_id"`
	AgentID           string    `json:"agent_id"`
	ConversationID    string    `json:"conversation_id"`
	TurnID            string    `json:"turn_id"`
	ClientTurnID      string    `json:"client_turn_id"`
	ContextManifestID string    `json:"context_manifest_id"`
	DelegationGrantID string    `json:"delegation_grant_id"`
	TargetID          string    `json:"target_id"`
	RunID             string    `json:"run_id"`
	HumanID           string    `json:"human_id"`
	Body              string    `json:"body"`
	CreatedBy         string    `json:"created_by"`
	CreatedAt         time.Time `json:"created_at"`
	HardDeadline      time.Time `json:"hard_deadline"`
}

func (command SendAgentTurnCommand) Validate() error {
	fields := map[string]string{
		"idempotency key": command.IdempotencyKey, "account id": command.AccountID,
		"Agent id": command.AgentID, "Conversation id": command.ConversationID,
		"Turn id": command.TurnID, "client Turn id": command.ClientTurnID,
		"Context Manifest id": command.ContextManifestID, "Delegation Grant id": command.DelegationGrantID,
		"Target id": command.TargetID, "Run id": command.RunID, "human id": command.HumanID,
		"creator": command.CreatedBy,
	}
	for label, value := range fields {
		if strings.TrimSpace(value) == "" || strings.TrimSpace(value) != value {
			return fmt.Errorf("Agent direct Send %s is required and must be canonical", label)
		}
	}
	if strings.TrimSpace(command.Body) == "" {
		return fmt.Errorf("Agent direct Send body is required")
	}
	if len([]byte(command.Body)) > MaxAgentMessageBytes {
		return fmt.Errorf("Agent direct Send body exceeds %d bytes", MaxAgentMessageBytes)
	}
	if command.CreatedAt.IsZero() || !command.HardDeadline.After(command.CreatedAt) {
		return fmt.Errorf("Agent direct Send requires a creation time and later hard deadline")
	}
	return nil
}

func (command SendAgentTurnCommand) Digest() (string, error) {
	canonical := command
	canonical.IdempotencyKey = ""
	// Every identifier below is allocated by the stateless control handler.
	// A network retry may reach another instance and allocate different values;
	// idempotency is therefore bound to the client-visible command semantics,
	// not those proposed internal record ids or wall-clock receipt time.
	canonical.TurnID = ""
	canonical.ContextManifestID = ""
	canonical.DelegationGrantID = ""
	canonical.TargetID = ""
	canonical.RunID = ""
	canonical.CreatedAt = time.Time{}
	canonical.HardDeadline = canonical.HardDeadline.UTC()
	return commandDigest(canonical)
}

type RetryAgentTargetCommand struct {
	IdempotencyKey string    `json:"idempotency_key"`
	AccountID      string    `json:"account_id"`
	AgentID        string    `json:"agent_id"`
	ConversationID string    `json:"conversation_id"`
	TargetID       string    `json:"target_id"`
	RetriedBy      string    `json:"retried_by"`
	RetriedAt      time.Time `json:"retried_at"`
}

func (command RetryAgentTargetCommand) Validate() error {
	for label, value := range map[string]string{
		"idempotency key": command.IdempotencyKey, "account id": command.AccountID,
		"Agent id": command.AgentID, "Conversation id": command.ConversationID,
		"Target id": command.TargetID, "actor": command.RetriedBy,
	} {
		if strings.TrimSpace(value) == "" || strings.TrimSpace(value) != value {
			return fmt.Errorf("Agent Target retry %s is required and must be canonical", label)
		}
	}
	if command.RetriedAt.IsZero() {
		return fmt.Errorf("Agent Target retry time is required")
	}
	return nil
}

func (command RetryAgentTargetCommand) Digest() (string, error) {
	canonical := command
	canonical.IdempotencyKey = ""
	canonical.RetriedAt = time.Time{}
	return commandDigest(canonical)
}

type CancelAgentTargetCommand struct {
	IdempotencyKey string    `json:"idempotency_key"`
	AccountID      string    `json:"account_id"`
	AgentID        string    `json:"agent_id"`
	ConversationID string    `json:"conversation_id"`
	TargetID       string    `json:"target_id"`
	CanceledBy     string    `json:"canceled_by"`
	CanceledAt     time.Time `json:"canceled_at"`
}

func (command CancelAgentTargetCommand) Validate() error {
	for label, value := range map[string]string{
		"idempotency key": command.IdempotencyKey, "account id": command.AccountID,
		"Agent id": command.AgentID, "Conversation id": command.ConversationID,
		"Target id": command.TargetID, "actor": command.CanceledBy,
	} {
		if strings.TrimSpace(value) == "" || strings.TrimSpace(value) != value {
			return fmt.Errorf("Agent Target cancel %s is required and must be canonical", label)
		}
	}
	if command.CanceledAt.IsZero() {
		return fmt.Errorf("Agent Target cancellation time is required")
	}
	return nil
}

func (command CancelAgentTargetCommand) Digest() (string, error) {
	canonical := command
	canonical.IdempotencyKey = ""
	canonical.CanceledAt = time.Time{}
	return commandDigest(canonical)
}

type AgentConversationMessage struct {
	ID             int64                   `json:"id"`
	ConversationID string                  `json:"conversation_id"`
	TurnID         string                  `json:"turn_id,omitempty"`
	TargetID       string                  `json:"target_id,omitempty"`
	AuthorKind     conversation.AuthorKind `json:"author_kind"`
	AuthorID       string                  `json:"author_id"`
	AuthorAgentID  string                  `json:"author_agent_id,omitempty"`
	Body           string                  `json:"body"`
	CreatedAt      time.Time               `json:"created_at"`
}

type AgentConversationTurn struct {
	ID                   string    `json:"id"`
	ConversationID       string    `json:"conversation_id"`
	ClientTurnID         string    `json:"client_turn_id"`
	PromptMessageID      int64     `json:"prompt_message_id"`
	ThroughMessageID     int64     `json:"through_message_id"`
	MembershipRevisionID string    `json:"membership_revision_id"`
	ContextManifestID    string    `json:"context_manifest_id"`
	State                string    `json:"state"`
	CreatedAt            time.Time `json:"created_at"`
}

type AgentContextManifest struct {
	ID               string    `json:"id"`
	ConversationID   string    `json:"conversation_id"`
	ThroughMessageID int64     `json:"through_message_id"`
	MessageIDs       []int64   `json:"message_ids"`
	Digest           string    `json:"digest"`
	CreatedAt        time.Time `json:"created_at"`
}

type AgentConversationTarget struct {
	ID                 string    `json:"id"`
	TurnID             string    `json:"turn_id"`
	ConversationID     string    `json:"conversation_id"`
	AgentID            string    `json:"agent_id"`
	BehaviorRevisionID string    `json:"behavior_revision_id"`
	BindingRevisionID  string    `json:"binding_revision_id"`
	ParticipantID      string    `json:"participant_id"`
	RunID              string    `json:"run_id"`
	State              string    `json:"state"`
	AttemptCount       int       `json:"attempt_count"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type AgentTurnDispatch struct {
	Message AgentConversationMessage `json:"message"`
	Turn    AgentConversationTurn    `json:"turn"`
	Context AgentContextManifest     `json:"context"`
	Target  AgentConversationTarget  `json:"target"`
	Created bool                     `json:"created"`
}

type AgentConversationProjection struct {
	Conversation AgentConversationRecord    `json:"conversation"`
	Messages     []AgentConversationMessage `json:"messages"`
	Turns        []AgentConversationTurn    `json:"turns"`
	Targets      []AgentConversationTarget  `json:"targets"`
}
