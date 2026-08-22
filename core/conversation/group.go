package conversation

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrGroupNeedsYou = errors.New("Group Turn needs human attention")

const (
	MinGroupAgents        = 2
	MaxGroupAgents        = 6
	MaxGroupAgentMessages = 10
	MaxGroupHandoffDepth  = 3
)

type GroupRecipientSelection string

const (
	GroupRecipientSelectionExplicit GroupRecipientSelection = "explicit"
	GroupRecipientSelectionEveryone GroupRecipientSelection = "everyone"
)

type GroupConcurrencyPolicy string

const (
	GroupSequential GroupConcurrencyPolicy = "sequential"
	GroupConcurrent GroupConcurrencyPolicy = "concurrent"
)

type LimitClassification string

const (
	LimitHard    LimitClassification = "hard"
	LimitUnknown LimitClassification = "unknown"
)

type GroupConversation struct {
	ID                          string            `json:"id"`
	AccountID                   string            `json:"account_id"`
	ConversationID              string            `json:"conversation_id"`
	State                       ConversationState `json:"state"`
	CurrentMembershipRevisionID string            `json:"current_membership_revision_id"`
	CreatedAt                   time.Time         `json:"created_at"`
}

type GroupMember struct {
	AgentID  string `json:"agent_id"`
	Position int    `json:"position"`
}

type GroupMembershipRevision struct {
	ID        string        `json:"id"`
	GroupID   string        `json:"group_id"`
	Revision  int           `json:"revision"`
	Members   []GroupMember `json:"members"`
	CreatedAt time.Time     `json:"created_at"`
}

type GroupRecipient struct {
	AgentID            string `json:"agent_id"`
	BehaviorRevisionID string `json:"behavior_revision_id"`
	BindingRevisionID  string `json:"binding_revision_id"`
	ParticipantID      string `json:"participant_id"`
}

type GroupTurnEnvelope struct {
	ID                         string                  `json:"id"`
	GroupID                    string                  `json:"group_id"`
	ConversationID             string                  `json:"conversation_id"`
	ClientTurnID               string                  `json:"client_turn_id"`
	IdempotencyKey             string                  `json:"idempotency_key"`
	MembershipRevisionID       string                  `json:"membership_revision_id"`
	Selection                  GroupRecipientSelection `json:"selection"`
	Recipients                 []GroupRecipient        `json:"recipients"`
	ContextSnapshotID          string                  `json:"context_snapshot_id"`
	RootDelegationGrant        AuthorityGrant          `json:"root_delegation_grant"`
	ConcurrencyPolicy          GroupConcurrencyPolicy  `json:"concurrency_policy"`
	CancellationPolicyID       string                  `json:"cancellation_policy_id"`
	CancellationPolicyRevision string                  `json:"cancellation_policy_revision"`
	ApprovalPolicyID           string                  `json:"approval_policy_id"`
	ApprovalPolicyRevision     string                  `json:"approval_policy_revision"`
	MaxAgentMessages           int                     `json:"max_agent_messages"`
	MaxHandoffDepth            int                     `json:"max_handoff_depth"`
	CostLimitClass             LimitClassification     `json:"cost_limit_class"`
	CostLimitEvidenceID        string                  `json:"cost_limit_evidence_id,omitempty"`
	TokenLimitClass            LimitClassification     `json:"token_limit_class"`
	TokenLimitEvidenceID       string                  `json:"token_limit_evidence_id,omitempty"`
	Deadline                   time.Time               `json:"deadline"`
	CreatedAt                  time.Time               `json:"created_at"`
}

type GroupInitialTarget struct {
	GroupTurnID        string `json:"group_turn_id"`
	Wave               int    `json:"wave"`
	AgentID            string `json:"agent_id"`
	BehaviorRevisionID string `json:"behavior_revision_id"`
	BindingRevisionID  string `json:"binding_revision_id"`
	ParticipantID      string `json:"participant_id"`
}

func (g GroupConversation) Validate() error {
	if err := requireStrings("Group", map[string]string{
		"id": g.ID, "account id": g.AccountID, "Conversation id": g.ConversationID,
		"current membership revision id": g.CurrentMembershipRevisionID,
	}); err != nil {
		return err
	}
	if g.State != ConversationOpen && g.State != ConversationArchived {
		return fmt.Errorf("Group state must be open or archived")
	}
	if g.CreatedAt.IsZero() {
		return fmt.Errorf("Group creation time is required")
	}
	return nil
}

func (r GroupMembershipRevision) Validate(group GroupConversation) error {
	if err := group.Validate(); err != nil {
		return err
	}
	if err := requireStrings("Group Membership Revision", map[string]string{"id": r.ID, "Group id": r.GroupID}); err != nil {
		return err
	}
	if r.GroupID != group.ID || r.ID != group.CurrentMembershipRevisionID {
		return fmt.Errorf("Group Membership Revision does not match the Group's current revision")
	}
	if r.Revision < 1 || r.CreatedAt.IsZero() {
		return fmt.Errorf("Group Membership Revision number and creation time are required")
	}
	if len(r.Members) < MinGroupAgents || len(r.Members) > MaxGroupAgents {
		return fmt.Errorf("Group must contain %d to %d Agents", MinGroupAgents, MaxGroupAgents)
	}
	seen := make(map[string]struct{}, len(r.Members))
	for position, member := range r.Members {
		if err := requireStrings("Group member", map[string]string{"Agent id": member.AgentID}); err != nil {
			return err
		}
		if member.Position != position {
			return fmt.Errorf("Group member positions must be consecutive and ordered")
		}
		if _, exists := seen[member.AgentID]; exists {
			return fmt.Errorf("Group contains duplicate Agent %q", member.AgentID)
		}
		seen[member.AgentID] = struct{}{}
	}
	return nil
}

func (e GroupTurnEnvelope) Validate(group GroupConversation, membership GroupMembershipRevision) error {
	if err := membership.Validate(group); err != nil {
		return err
	}
	if group.State != ConversationOpen {
		return fmt.Errorf("new Group Turns require an open Group")
	}
	if err := requireStrings("Group Turn", map[string]string{
		"id": e.ID, "Group id": e.GroupID, "Conversation id": e.ConversationID,
		"client turn id": e.ClientTurnID, "idempotency key": e.IdempotencyKey,
		"membership revision id": e.MembershipRevisionID, "context snapshot id": e.ContextSnapshotID,
		"cancellation policy id":       e.CancellationPolicyID,
		"cancellation policy revision": e.CancellationPolicyRevision,
		"approval policy id":           e.ApprovalPolicyID,
		"approval policy revision":     e.ApprovalPolicyRevision,
	}); err != nil {
		return err
	}
	if e.GroupID != group.ID || e.ConversationID != group.ConversationID || e.MembershipRevisionID != membership.ID {
		return fmt.Errorf("Group Turn does not match its Group and Membership Revision")
	}
	if err := e.RootDelegationGrant.Validate(); err != nil {
		return fmt.Errorf("Group Turn root delegation grant: %w", err)
	}
	if e.Selection != GroupRecipientSelectionExplicit && e.Selection != GroupRecipientSelectionEveryone {
		return fmt.Errorf("Group Turn requires an explicit or Everyone recipient selection")
	}
	if len(e.Recipients) == 0 {
		return fmt.Errorf("Group Turn requires at least one explicitly selected Agent")
	}
	members := make(map[string]struct{}, len(membership.Members))
	for _, member := range membership.Members {
		members[member.AgentID] = struct{}{}
	}
	seen := make(map[string]struct{}, len(e.Recipients))
	for _, recipient := range e.Recipients {
		if err := requireStrings("Group recipient", map[string]string{
			"Agent id": recipient.AgentID, "Behavior Revision id": recipient.BehaviorRevisionID,
			"Binding Revision id": recipient.BindingRevisionID, "participant id": recipient.ParticipantID,
		}); err != nil {
			return err
		}
		if _, exists := members[recipient.AgentID]; !exists {
			return fmt.Errorf("Group recipient Agent %q is not a current member", recipient.AgentID)
		}
		if _, exists := seen[recipient.AgentID]; exists {
			return fmt.Errorf("Group Turn has more than one initial target for Agent %q", recipient.AgentID)
		}
		seen[recipient.AgentID] = struct{}{}
	}
	if e.Selection == GroupRecipientSelectionEveryone {
		if len(e.Recipients) != len(membership.Members) {
			return fmt.Errorf("Everyone selection must freeze every current Group member")
		}
		for i, member := range membership.Members {
			if e.Recipients[i].AgentID != member.AgentID {
				return fmt.Errorf("Everyone selection must preserve Group membership order")
			}
		}
	}
	if e.ConcurrencyPolicy != GroupSequential && e.ConcurrencyPolicy != GroupConcurrent {
		return fmt.Errorf("Group Turn concurrency policy is invalid")
	}
	if e.MaxAgentMessages != MaxGroupAgentMessages || e.MaxHandoffDepth != MaxGroupHandoffDepth {
		return fmt.Errorf("Group Turn requires the first-release ten-message and depth-three limits")
	}
	if !e.CostLimitClass.Valid() || !e.TokenLimitClass.Valid() {
		return fmt.Errorf("Group Turn cost and token limit classifications must be hard or unknown")
	}
	if e.CostLimitClass == LimitHard && strings.TrimSpace(e.CostLimitEvidenceID) == "" {
		return fmt.Errorf("hard Group Turn cost limit requires pre-start enforceability evidence")
	}
	if e.TokenLimitClass == LimitHard && strings.TrimSpace(e.TokenLimitEvidenceID) == "" {
		return fmt.Errorf("hard Group Turn token limit requires pre-start enforceability evidence")
	}
	if e.CreatedAt.IsZero() || !e.Deadline.After(e.CreatedAt) {
		return fmt.Errorf("Group Turn requires a hard deadline after creation")
	}
	return nil
}

// CanStart applies the shared first-release message and deadline bounds before
// any initial target or accepted Handoff starts a provider. agentMessages is
// the persisted total across the initial wave and its Handoff chain.
func (e GroupTurnEnvelope) CanStart(
	group GroupConversation,
	membership GroupMembershipRevision,
	now time.Time,
	agentMessages int,
) error {
	if err := e.Validate(group, membership); err != nil {
		return err
	}
	if agentMessages < 0 {
		return fmt.Errorf("Group Turn Agent message count cannot be negative")
	}
	if !now.Before(e.Deadline) || agentMessages >= e.MaxAgentMessages {
		return fmt.Errorf("%w: Group Turn limit exhausted", ErrGroupNeedsYou)
	}
	return nil
}

func (c LimitClassification) Valid() bool {
	return c == LimitHard || c == LimitUnknown
}

// InitialTargets exposes the only first-release fan-out wave. Agent output,
// prose mentions, and silence have no API here that can create another wave.
func (e GroupTurnEnvelope) InitialTargets(group GroupConversation, membership GroupMembershipRevision) ([]GroupInitialTarget, error) {
	if err := e.Validate(group, membership); err != nil {
		return nil, err
	}
	targets := make([]GroupInitialTarget, 0, len(e.Recipients))
	for _, recipient := range e.Recipients {
		targets = append(targets, GroupInitialTarget{
			GroupTurnID: e.ID, Wave: 0, AgentID: recipient.AgentID,
			BehaviorRevisionID: recipient.BehaviorRevisionID, BindingRevisionID: recipient.BindingRevisionID,
			ParticipantID: recipient.ParticipantID,
		})
	}
	return targets, nil
}
