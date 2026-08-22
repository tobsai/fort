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

var ErrAlreadyCompleted = errors.New("ledger resource already has an authoritative completion")

// GroupRecord reconstructs a Group's current immutable membership and the
// exact participant/binding evidence accepted for that revision.
type GroupRecord struct {
	Group          conversation.GroupConversation       `json:"group"`
	Conversation   conversation.Conversation            `json:"conversation"`
	Membership     conversation.GroupMembershipRevision `json:"membership"`
	MemberBindings []conversation.GroupRecipient        `json:"member_bindings"`
}

// InitialTargetRecord is one persisted wave-zero target. There is no
// repository command for creating a later reply wave.
type InitialTargetRecord struct {
	ID string `json:"id"`
	conversation.GroupInitialTarget
	State     conversation.TargetState `json:"state"`
	CreatedAt time.Time                `json:"created_at"`
}

// GroupTurnRecord is the atomic durable result of one human Group Send.
type GroupTurnRecord struct {
	Message        conversation.Message           `json:"message"`
	Envelope       conversation.GroupTurnEnvelope `json:"envelope"`
	Recipients     []conversation.GroupRecipient  `json:"recipients"`
	InitialTargets []InitialTargetRecord          `json:"initial_targets"`
}

type CreateGroupCommand struct {
	IdempotencyKey string                               `json:"idempotency_key"`
	Group          conversation.GroupConversation       `json:"group"`
	Conversation   conversation.Conversation            `json:"conversation"`
	Membership     conversation.GroupMembershipRevision `json:"membership"`
	MemberBindings []conversation.GroupRecipient        `json:"member_bindings"`
}

type SendGroupTurnCommand struct {
	AccountID string                         `json:"account_id"`
	HumanID   string                         `json:"human_id"`
	Body      string                         `json:"body"`
	Envelope  conversation.GroupTurnEnvelope `json:"envelope"`
	TargetIDs []string                       `json:"target_ids"`
}

// RenameGroupCommand changes only the Group Conversation's presentation
// title. ExpectedTitle is the optimistic concurrency boundary supplied by the
// owner; every identity and audit field is allocated by Fort.
type RenameGroupCommand struct {
	IdempotencyKey string    `json:"idempotency_key"`
	AccountID      string    `json:"account_id"`
	GroupID        string    `json:"group_id"`
	ExpectedTitle  string    `json:"expected_title"`
	Title          string    `json:"title"`
	ChangedBy      string    `json:"changed_by"`
	ChangedAt      time.Time `json:"changed_at"`
}

// SetGroupStateCommand archives or reopens one Group without changing its
// stable identity, Conversation, or current membership revision.
type SetGroupStateCommand struct {
	IdempotencyKey string                         `json:"idempotency_key"`
	AccountID      string                         `json:"account_id"`
	GroupID        string                         `json:"group_id"`
	ExpectedState  conversation.ConversationState `json:"expected_state"`
	State          conversation.ConversationState `json:"state"`
	ChangedBy      string                         `json:"changed_by"`
	ChangedAt      time.Time                      `json:"changed_at"`
}

// ReplaceGroupMembersCommand appends one complete successor membership. The
// owner supplies only the expected predecessor and ordered stable Agent IDs;
// Fort resolves Membership, binding, and participant evidence before calling
// the repository.
type ReplaceGroupMembersCommand struct {
	IdempotencyKey               string                               `json:"idempotency_key"`
	AccountID                    string                               `json:"account_id"`
	GroupID                      string                               `json:"group_id"`
	ExpectedMembershipRevisionID string                               `json:"expected_membership_revision_id"`
	Membership                   conversation.GroupMembershipRevision `json:"membership"`
	MemberBindings               []conversation.GroupRecipient        `json:"member_bindings"`
	ChangedBy                    string                               `json:"changed_by"`
	ChangedAt                    time.Time                            `json:"changed_at"`
}

type HandoffTargetRecord struct {
	ID                 string                   `json:"id"`
	HandoffID          string                   `json:"handoff_id"`
	ConversationID     string                   `json:"conversation_id"`
	AgentID            string                   `json:"agent_id"`
	BehaviorRevisionID string                   `json:"behavior_revision_id"`
	BindingRevisionID  string                   `json:"binding_revision_id"`
	ParticipantID      string                   `json:"participant_id"`
	State              conversation.TargetState `json:"state"`
	CreatedAt          time.Time                `json:"created_at"`
}

type HandoffRecord struct {
	Handoff      conversation.Handoff             `json:"handoff"`
	Target       HandoffTargetRecord              `json:"target"`
	Attempt      *HandoffAttemptRecord            `json:"attempt,omitempty"`
	Cancellation *HandoffCancellationRecord       `json:"cancellation,omitempty"`
	Projections  []conversation.HandoffProjection `json:"projections"`
	Result       *conversation.HandoffResult      `json:"result,omitempty"`
}

type HandoffAttemptState string

const (
	HandoffAttemptWorking   HandoffAttemptState = "working"
	HandoffAttemptCompleted HandoffAttemptState = "completed"
	HandoffAttemptFailed    HandoffAttemptState = "failed"
	HandoffAttemptCanceled  HandoffAttemptState = "canceled"
)

// HandoffAttemptRecord is the exact leased execution evidence required before
// a Handoff can commit its one authoritative result.
type HandoffAttemptRecord struct {
	ID                string              `json:"id"`
	HandoffID         string              `json:"handoff_id"`
	LeaseID           string              `json:"lease_id"`
	MachineID         string              `json:"machine_id"`
	FenceToken        string              `json:"fence_token"`
	State             HandoffAttemptState `json:"state"`
	StartedAt         time.Time           `json:"started_at"`
	LeaseExpiresAt    time.Time           `json:"lease_expires_at"`
	TerminalReceiptID string              `json:"terminal_receipt_id,omitempty"`
	CompletedAt       time.Time           `json:"completed_at,omitempty"`
}

type AcceptHandoffCommand struct {
	Handoff                   conversation.Handoff `json:"handoff"`
	TargetID                  string               `json:"target_id"`
	ParticipantID             string               `json:"participant_id"`
	ProjectionConversationIDs []string             `json:"projection_conversation_ids"`
	// RequireCurrentRecipient is server-owned acceptance policy. When true,
	// adapters revalidate the recipient pins against the Agent's current
	// Behavior and Binding in the accepting write transaction.
	RequireCurrentRecipient bool `json:"-"`
}

// CreateHumanHandoffCommand is the narrow owner-facing Handoff command. The
// first block is client-visible intent; Fort allocates every identity and
// receipt field in the second block. Digest deliberately excludes the latter
// so a network replay returns the originally accepted Handoff.
type CreateHumanHandoffCommand struct {
	IdempotencyKey       string    `json:"idempotency_key"`
	AccountID            string    `json:"account_id"`
	SourceConversationID string    `json:"source_conversation_id"`
	SourceMessageID      string    `json:"source_message_id"`
	RecipientAgentID     string    `json:"recipient_agent_id"`
	ContextMessageIDs    []string  `json:"context_message_ids"`
	RequestedResult      string    `json:"requested_result"`
	ReplyToMessageID     string    `json:"reply_to_message_id,omitempty"`
	HardDeadline         time.Time `json:"hard_deadline"`

	HandoffID             string    `json:"handoff_id"`
	TargetID              string    `json:"target_id"`
	RootDelegationGrantID string    `json:"root_delegation_grant_id"`
	CreatedByID           string    `json:"created_by_id"`
	CreatedAt             time.Time `json:"created_at"`
}

// CancelHandoffCommand records an owner's durable request against the one
// target already pinned by a Handoff. It never accepts a replacement target.
type CancelHandoffCommand struct {
	IdempotencyKey string    `json:"idempotency_key"`
	AccountID      string    `json:"account_id"`
	HandoffID      string    `json:"handoff_id"`
	CanceledBy     string    `json:"canceled_by"`
	CanceledAt     time.Time `json:"canceled_at"`
}

type HandoffCancellationState string

const (
	HandoffCancellationRequested HandoffCancellationState = "requested"
	HandoffCancellationCanceled  HandoffCancellationState = "canceled"
)

// HandoffCancellationRecord is durable evidence that cancellation addressed
// the original target and exact accepted revision pins.
type HandoffCancellationRecord struct {
	HandoffID          string                   `json:"handoff_id"`
	TargetID           string                   `json:"target_id"`
	AgentID            string                   `json:"agent_id"`
	BehaviorRevisionID string                   `json:"behavior_revision_id"`
	BindingRevisionID  string                   `json:"binding_revision_id"`
	ParticipantID      string                   `json:"participant_id"`
	State              HandoffCancellationState `json:"state"`
	RequestedBy        string                   `json:"requested_by"`
	RequestedAt        time.Time                `json:"requested_at"`
}

type CompleteHandoffCommand struct {
	AccountID         string    `json:"account_id"`
	HandoffID         string    `json:"handoff_id"`
	IdempotencyKey    string    `json:"idempotency_key"`
	AuthorAgentID     string    `json:"author_agent_id"`
	AttemptID         string    `json:"attempt_id"`
	LeaseID           string    `json:"lease_id"`
	FenceToken        string    `json:"fence_token"`
	TerminalReceiptID string    `json:"terminal_receipt_id"`
	Body              string    `json:"body"`
	CreatedAt         time.Time `json:"created_at"`
}

type StartHandoffCommand struct {
	AccountID      string    `json:"account_id"`
	HandoffID      string    `json:"handoff_id"`
	IdempotencyKey string    `json:"idempotency_key"`
	AttemptID      string    `json:"attempt_id"`
	LeaseID        string    `json:"lease_id"`
	MachineID      string    `json:"machine_id"`
	FenceToken     string    `json:"fence_token"`
	StartedAt      time.Time `json:"started_at"`
	LeaseExpiresAt time.Time `json:"lease_expires_at"`
}

// CollaborationRepository is the shared persistence seam for the first
// stable-Agent Group and Handoff slice. SQLite and Postgres implement the same
// account-scoped operations; list operations return allocated empty slices.
type CollaborationRepository interface {
	AgentRepository
	CreateGroup(context.Context, CreateGroupCommand) (GroupRecord, error)
	GetGroup(context.Context, string, string) (GroupRecord, error)
	ListGroups(context.Context, string, conversation.ConversationState) ([]GroupRecord, error)
	RenameGroup(context.Context, RenameGroupCommand) (GroupRecord, error)
	SetGroupState(context.Context, SetGroupStateCommand) (GroupRecord, error)
	ReplaceGroupMembers(context.Context, ReplaceGroupMembersCommand) (GroupRecord, error)
	SendGroupTurn(context.Context, SendGroupTurnCommand) (GroupTurnRecord, error)
	ListGroupTurns(context.Context, string, string) ([]GroupTurnRecord, error)
	CreateHumanHandoff(context.Context, CreateHumanHandoffCommand) (HandoffRecord, error)
	AcceptHandoff(context.Context, AcceptHandoffCommand) (HandoffRecord, error)
	GetHandoff(context.Context, string, string) (HandoffRecord, error)
	ListHandoffs(context.Context, string) ([]HandoffRecord, error)
	CancelHandoff(context.Context, CancelHandoffCommand) (HandoffRecord, error)
	StartHandoff(context.Context, StartHandoffCommand) (HandoffRecord, error)
	CompleteHandoff(context.Context, CompleteHandoffCommand) (HandoffRecord, error)
}

func (c RenameGroupCommand) Validate() error {
	if err := validateGroupLifecycleIdentity(c.IdempotencyKey, c.AccountID, c.GroupID, c.ChangedBy, c.ChangedAt); err != nil {
		return err
	}
	if strings.TrimSpace(c.ExpectedTitle) == "" || strings.TrimSpace(c.Title) == "" ||
		c.ExpectedTitle != strings.TrimSpace(c.ExpectedTitle) || c.Title != strings.TrimSpace(c.Title) ||
		len([]byte(c.ExpectedTitle)) > 512 || len([]byte(c.Title)) > 512 || c.ExpectedTitle == c.Title {
		return fmt.Errorf("Group rename requires distinct normalized titles no greater than 512 UTF-8 bytes")
	}
	return nil
}

func (c RenameGroupCommand) Digest() (string, error) {
	canonical := c
	canonical.IdempotencyKey = ""
	canonical.ChangedAt = time.Time{}
	return collaborationDigest(canonical)
}

func (c SetGroupStateCommand) Validate() error {
	if err := validateGroupLifecycleIdentity(c.IdempotencyKey, c.AccountID, c.GroupID, c.ChangedBy, c.ChangedAt); err != nil {
		return err
	}
	if (c.ExpectedState != conversation.ConversationOpen && c.ExpectedState != conversation.ConversationArchived) ||
		(c.State != conversation.ConversationOpen && c.State != conversation.ConversationArchived) ||
		c.ExpectedState == c.State {
		return fmt.Errorf("Group state command requires distinct open and archived states")
	}
	return nil
}

func (c SetGroupStateCommand) Digest() (string, error) {
	canonical := c
	canonical.IdempotencyKey = ""
	canonical.ChangedAt = time.Time{}
	return collaborationDigest(canonical)
}

func (c ReplaceGroupMembersCommand) Validate() error {
	if err := validateGroupLifecycleIdentity(c.IdempotencyKey, c.AccountID, c.GroupID, c.ChangedBy, c.ChangedAt); err != nil {
		return err
	}
	if strings.TrimSpace(c.ExpectedMembershipRevisionID) == "" {
		return fmt.Errorf("expected Group Membership Revision id is required")
	}
	group := conversation.GroupConversation{
		ID: c.GroupID, AccountID: c.AccountID, ConversationID: "server-resolved",
		State: conversation.ConversationOpen, CurrentMembershipRevisionID: c.Membership.ID,
		CreatedAt: c.Membership.CreatedAt,
	}
	if err := c.Membership.Validate(group); err != nil {
		return err
	}
	if c.Membership.GroupID != c.GroupID || c.Membership.CreatedAt.IsZero() || !c.Membership.CreatedAt.Equal(c.ChangedAt) {
		return fmt.Errorf("successor Group Membership identity and time do not match the command")
	}
	if len(c.MemberBindings) != len(c.Membership.Members) {
		return fmt.Errorf("Group requires one binding snapshot per member")
	}
	seenParticipants := make(map[string]struct{}, len(c.MemberBindings))
	for position, binding := range c.MemberBindings {
		member := c.Membership.Members[position]
		if binding.AgentID != member.AgentID || strings.TrimSpace(binding.BehaviorRevisionID) == "" ||
			strings.TrimSpace(binding.BindingRevisionID) == "" || strings.TrimSpace(binding.ParticipantID) == "" {
			return fmt.Errorf("Group member binding order and revision evidence must match membership")
		}
		if _, exists := seenParticipants[binding.ParticipantID]; exists {
			return fmt.Errorf("Group contains duplicate participant %q", binding.ParticipantID)
		}
		seenParticipants[binding.ParticipantID] = struct{}{}
	}
	return nil
}

func (c ReplaceGroupMembersCommand) Digest() (string, error) {
	canonical := struct {
		AccountID                    string                     `json:"account_id"`
		GroupID                      string                     `json:"group_id"`
		ExpectedMembershipRevisionID string                     `json:"expected_membership_revision_id"`
		Members                      []conversation.GroupMember `json:"members"`
		ChangedBy                    string                     `json:"changed_by"`
	}{
		AccountID: c.AccountID, GroupID: c.GroupID,
		ExpectedMembershipRevisionID: c.ExpectedMembershipRevisionID,
		Members:                      append([]conversation.GroupMember{}, c.Membership.Members...), ChangedBy: c.ChangedBy,
	}
	return collaborationDigest(canonical)
}

func validateGroupLifecycleIdentity(idempotencyKey, accountID, groupID, changedBy string, changedAt time.Time) error {
	if err := validateIdempotencyKey("Group lifecycle", idempotencyKey); err != nil {
		return err
	}
	if strings.TrimSpace(accountID) == "" || strings.TrimSpace(groupID) == "" || strings.TrimSpace(changedBy) == "" || changedAt.IsZero() {
		return fmt.Errorf("Group lifecycle account, Group, actor, and time are required")
	}
	return nil
}

func (c CreateGroupCommand) Validate() error {
	if err := validateIdempotencyKey("Group creation", c.IdempotencyKey); err != nil {
		return err
	}
	if err := c.Group.Validate(); err != nil {
		return err
	}
	if err := c.Membership.Validate(c.Group); err != nil {
		return err
	}
	if c.Group.State != conversation.ConversationOpen {
		return fmt.Errorf("new Group must be open")
	}
	if strings.TrimSpace(c.Conversation.ID) == "" || strings.TrimSpace(c.Conversation.Title) == "" {
		return fmt.Errorf("Group Conversation id and title are required")
	}
	if c.Conversation.ID != c.Group.ConversationID || c.Conversation.State != c.Group.State {
		return fmt.Errorf("Group does not match its Conversation")
	}
	if c.Conversation.CreatedAt.IsZero() || c.Conversation.UpdatedAt.IsZero() {
		return fmt.Errorf("Group Conversation creation and update times are required")
	}
	if len(c.MemberBindings) != len(c.Membership.Members) {
		return fmt.Errorf("Group requires one binding snapshot per member")
	}
	seenParticipants := make(map[string]struct{}, len(c.MemberBindings))
	for position, binding := range c.MemberBindings {
		member := c.Membership.Members[position]
		if binding.AgentID != member.AgentID {
			return fmt.Errorf("Group member binding order does not match membership")
		}
		if strings.TrimSpace(binding.BehaviorRevisionID) == "" || strings.TrimSpace(binding.BindingRevisionID) == "" || strings.TrimSpace(binding.ParticipantID) == "" {
			return fmt.Errorf("Group member binding requires Behavior, Binding, and participant ids")
		}
		if _, exists := seenParticipants[binding.ParticipantID]; exists {
			return fmt.Errorf("Group contains duplicate participant %q", binding.ParticipantID)
		}
		seenParticipants[binding.ParticipantID] = struct{}{}
	}
	return nil
}

func (c CreateGroupCommand) Digest() (string, error) {
	// Stable IDs, timestamps, revision pins, and participant evidence are
	// allocated or resolved by Fort. Network retries bind to the owner's
	// visible title and ordered stable-Agent selection, not a warm instance's
	// proposed internal records.
	canonical := struct {
		AccountID string                         `json:"account_id"`
		Title     string                         `json:"title"`
		State     conversation.ConversationState `json:"state"`
		Members   []conversation.GroupMember     `json:"members"`
	}{
		AccountID: c.Group.AccountID,
		Title:     c.Conversation.Title,
		State:     c.Group.State,
		Members:   append([]conversation.GroupMember{}, c.Membership.Members...),
	}
	return collaborationDigest(canonical)
}

func (c SendGroupTurnCommand) Validate(group conversation.GroupConversation, membership conversation.GroupMembershipRevision) error {
	if strings.TrimSpace(c.AccountID) == "" || strings.TrimSpace(c.HumanID) == "" {
		return fmt.Errorf("Group Send account and human ids are required")
	}
	if strings.TrimSpace(c.Body) == "" {
		return fmt.Errorf("Group Send body is required")
	}
	if c.AccountID != group.AccountID {
		return fmt.Errorf("Group Send belongs to another account")
	}
	if err := c.Envelope.Validate(group, membership); err != nil {
		return err
	}
	if len(c.TargetIDs) != len(c.Envelope.Recipients) {
		return fmt.Errorf("Group Send requires one target id per frozen recipient")
	}
	seen := make(map[string]struct{}, len(c.TargetIDs))
	for _, targetID := range c.TargetIDs {
		if strings.TrimSpace(targetID) == "" {
			return fmt.Errorf("Group Send target id is required")
		}
		if _, exists := seen[targetID]; exists {
			return fmt.Errorf("Group Send contains duplicate target %q", targetID)
		}
		seen[targetID] = struct{}{}
	}
	return nil
}

func (c SendGroupTurnCommand) Digest() (string, error) {
	recipientAgentIDs := make([]string, 0, len(c.Envelope.Recipients))
	for _, recipient := range c.Envelope.Recipients {
		recipientAgentIDs = append(recipientAgentIDs, recipient.AgentID)
	}
	canonical := struct {
		AccountID         string                               `json:"account_id"`
		HumanID           string                               `json:"human_id"`
		Body              string                               `json:"body"`
		GroupID           string                               `json:"group_id"`
		ClientTurnID      string                               `json:"client_turn_id"`
		Selection         conversation.GroupRecipientSelection `json:"selection"`
		RecipientAgentIDs []string                             `json:"recipient_agent_ids"`
		Concurrency       conversation.GroupConcurrencyPolicy  `json:"concurrency_policy"`
		Deadline          time.Time                            `json:"deadline"`
	}{
		AccountID: c.AccountID, HumanID: c.HumanID, Body: c.Body,
		GroupID: c.Envelope.GroupID, ClientTurnID: c.Envelope.ClientTurnID,
		Selection: c.Envelope.Selection, RecipientAgentIDs: recipientAgentIDs,
		Concurrency: c.Envelope.ConcurrencyPolicy, Deadline: c.Envelope.Deadline.UTC(),
	}
	return collaborationDigest(canonical)
}

func (c AcceptHandoffCommand) Validate() error {
	if err := c.Handoff.Validate(); err != nil {
		return err
	}
	if c.Handoff.State != conversation.HandoffQueued {
		return fmt.Errorf("accepted Handoff must be queued")
	}
	if c.Handoff.ApprovalRequired && c.Handoff.ApprovalReceipt == nil {
		return fmt.Errorf("accepted Handoff requires its approval receipt")
	}
	if strings.TrimSpace(c.TargetID) == "" || strings.TrimSpace(c.ParticipantID) == "" {
		return fmt.Errorf("accepted Handoff target and participant ids are required")
	}
	seen := make(map[string]struct{}, len(c.ProjectionConversationIDs))
	for _, conversationID := range c.ProjectionConversationIDs {
		if strings.TrimSpace(conversationID) == "" || conversationID == c.Handoff.OutputConversationID {
			return fmt.Errorf("reference-only Handoff projection requires another affected Conversation")
		}
		if _, exists := seen[conversationID]; exists {
			return fmt.Errorf("Handoff contains duplicate projection Conversation %q", conversationID)
		}
		seen[conversationID] = struct{}{}
	}
	if c.Handoff.SourceConversationID != c.Handoff.OutputConversationID {
		if _, exists := seen[c.Handoff.SourceConversationID]; !exists {
			return fmt.Errorf("Handoff source Conversation requires a reference-only projection")
		}
	}
	return nil
}

func (c AcceptHandoffCommand) Digest() (string, error) {
	if c.Handoff.CreatedByKind == conversation.HandoffActorHuman {
		return humanHandoffDigest(c.Handoff.AccountID, c.Handoff.SourceConversationID,
			c.Handoff.SourceMessageID, c.Handoff.RecipientAgentID, c.Handoff.Context,
			c.Handoff.RequestedResult, c.Handoff.ReplyToMessageID, c.Handoff.Deadline)
	}
	canonical := c
	canonical.Handoff.IdempotencyKey = ""
	canonical.RequireCurrentRecipient = false
	canonical.ProjectionConversationIDs = append([]string{}, c.ProjectionConversationIDs...)
	sort.Strings(canonical.ProjectionConversationIDs)
	return collaborationDigest(canonical)
}

func (c CreateHumanHandoffCommand) Validate() error {
	if err := validateIdempotencyKey("human Handoff creation", c.IdempotencyKey); err != nil {
		return err
	}
	for name, value := range map[string]string{
		"account id": c.AccountID, "source Conversation id": c.SourceConversationID,
		"source message id": c.SourceMessageID, "recipient Agent id": c.RecipientAgentID,
		"requested result": c.RequestedResult, "Handoff id": c.HandoffID,
		"target id": c.TargetID, "root delegation grant id": c.RootDelegationGrantID,
		"creation actor id": c.CreatedByID,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("human Handoff %s is required", name)
		}
	}
	if !strings.HasPrefix(c.CreatedByID, "human:") {
		return fmt.Errorf("human Handoff creation actor must be human")
	}
	if len([]byte(c.RequestedResult)) > MaxAgentMessageBytes {
		return fmt.Errorf("human Handoff requested result exceeds %d UTF-8 bytes", MaxAgentMessageBytes)
	}
	if len(c.ContextMessageIDs) > conversation.MaxHandoffContextReferences {
		return fmt.Errorf("human Handoff context exceeds %d messages", conversation.MaxHandoffContextReferences)
	}
	seen := make(map[string]struct{}, len(c.ContextMessageIDs))
	for _, messageID := range c.ContextMessageIDs {
		if strings.TrimSpace(messageID) == "" || messageID != strings.TrimSpace(messageID) {
			return fmt.Errorf("human Handoff context message id is invalid")
		}
		if _, exists := seen[messageID]; exists {
			return fmt.Errorf("human Handoff context message %q is duplicated", messageID)
		}
		seen[messageID] = struct{}{}
	}
	if c.CreatedAt.IsZero() || !c.HardDeadline.After(c.CreatedAt) {
		return fmt.Errorf("human Handoff requires a future hard deadline")
	}
	return nil
}

func (c CreateHumanHandoffCommand) Digest() (string, error) {
	context := conversation.ContextManifest{References: make([]conversation.ContextReference, 0, len(c.ContextMessageIDs))}
	for _, messageID := range c.ContextMessageIDs {
		context.References = append(context.References, conversation.ContextReference{Kind: conversation.ContextMessage, ID: messageID})
	}
	return humanHandoffDigest(c.AccountID, c.SourceConversationID, c.SourceMessageID,
		c.RecipientAgentID, context, c.RequestedResult, c.ReplyToMessageID, c.HardDeadline)
}

func (c CancelHandoffCommand) Validate() error {
	if err := validateIdempotencyKey("Handoff cancellation", c.IdempotencyKey); err != nil {
		return err
	}
	if strings.TrimSpace(c.AccountID) == "" || strings.TrimSpace(c.HandoffID) == "" ||
		strings.TrimSpace(c.CanceledBy) == "" || c.CanceledAt.IsZero() {
		return fmt.Errorf("Handoff cancellation account, Handoff, actor, and receipt time are required")
	}
	if !strings.HasPrefix(c.CanceledBy, "human:") {
		return fmt.Errorf("Handoff cancellation actor must be human")
	}
	return nil
}

func (c CancelHandoffCommand) Digest() (string, error) {
	return collaborationDigest(struct {
		AccountID string `json:"account_id"`
		HandoffID string `json:"handoff_id"`
	}{AccountID: c.AccountID, HandoffID: c.HandoffID})
}

func humanHandoffDigest(accountID, sourceConversationID, sourceMessageID, recipientAgentID string,
	manifest conversation.ContextManifest, requestedResult, replyToMessageID string, deadline time.Time) (string, error) {
	context := make([]struct {
		Kind conversation.ContextReferenceKind `json:"kind"`
		ID   string                            `json:"id"`
	}, 0, len(manifest.References))
	for _, reference := range manifest.References {
		context = append(context, struct {
			Kind conversation.ContextReferenceKind `json:"kind"`
			ID   string                            `json:"id"`
		}{Kind: reference.Kind, ID: reference.ID})
	}
	return collaborationDigest(struct {
		AccountID            string `json:"account_id"`
		SourceConversationID string `json:"source_conversation_id"`
		SourceMessageID      string `json:"source_message_id"`
		RecipientAgentID     string `json:"recipient_agent_id"`
		Context              any    `json:"context"`
		RequestedResult      string `json:"requested_result"`
		ReplyToMessageID     string `json:"reply_to_message_id"`
		HardDeadline         string `json:"hard_deadline"`
	}{
		AccountID: accountID, SourceConversationID: sourceConversationID,
		SourceMessageID: sourceMessageID, RecipientAgentID: recipientAgentID,
		Context: context, RequestedResult: requestedResult, ReplyToMessageID: replyToMessageID,
		HardDeadline: deadline.UTC().Format(time.RFC3339Nano),
	})
}

func (c CompleteHandoffCommand) Validate() error {
	if strings.TrimSpace(c.AccountID) == "" || strings.TrimSpace(c.HandoffID) == "" ||
		strings.TrimSpace(c.AuthorAgentID) == "" || strings.TrimSpace(c.AttemptID) == "" ||
		strings.TrimSpace(c.LeaseID) == "" || strings.TrimSpace(c.FenceToken) == "" ||
		strings.TrimSpace(c.TerminalReceiptID) == "" {
		return fmt.Errorf("Handoff completion account, Handoff, author Agent, attempt, lease, fence, and terminal receipt ids are required")
	}
	if err := validateIdempotencyKey("Handoff completion", c.IdempotencyKey); err != nil {
		return err
	}
	if strings.TrimSpace(c.Body) == "" {
		return fmt.Errorf("Handoff completion body is required")
	}
	if c.CreatedAt.IsZero() {
		return fmt.Errorf("Handoff completion time is required")
	}
	return nil
}

func (c StartHandoffCommand) Validate() error {
	if strings.TrimSpace(c.AccountID) == "" || strings.TrimSpace(c.HandoffID) == "" ||
		strings.TrimSpace(c.AttemptID) == "" || strings.TrimSpace(c.LeaseID) == "" ||
		strings.TrimSpace(c.MachineID) == "" || strings.TrimSpace(c.FenceToken) == "" {
		return fmt.Errorf("Handoff start account, Handoff, attempt, lease, machine, and fence ids are required")
	}
	if err := validateIdempotencyKey("Handoff start", c.IdempotencyKey); err != nil {
		return err
	}
	if c.StartedAt.IsZero() || !c.LeaseExpiresAt.After(c.StartedAt) {
		return fmt.Errorf("Handoff start requires a future lease expiry")
	}
	return nil
}

func (c StartHandoffCommand) Digest() (string, error) {
	canonical := c
	canonical.IdempotencyKey = ""
	return collaborationDigest(canonical)
}

func (c CompleteHandoffCommand) Digest() (string, error) {
	canonical := c
	canonical.IdempotencyKey = ""
	return collaborationDigest(canonical)
}

func validateIdempotencyKey(subject, key string) error {
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("%s idempotency key is required", subject)
	}
	if len([]byte(key)) > 256 {
		return fmt.Errorf("%s idempotency key exceeds 256 UTF-8 bytes", subject)
	}
	return nil
}

func collaborationDigest(value any) (string, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("canonicalize collaboration command: %w", err)
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}
