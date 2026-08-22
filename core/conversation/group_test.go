package conversation_test

import (
	"errors"
	"testing"
	"time"

	"github.com/tobsai/fort/core/conversation"
)

func TestGroupTurnCreatesOneFrozenInitialTargetPerSelectedStableAgent(t *testing.T) {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	group := conversation.GroupConversation{
		ID: "group:launch", AccountID: "account:one", ConversationID: "conversation:launch",
		State: conversation.ConversationOpen, CurrentMembershipRevisionID: "membership:1", CreatedAt: now,
	}
	membership := conversation.GroupMembershipRevision{
		ID: "membership:1", GroupID: group.ID, Revision: 1, CreatedAt: now,
		Members: []conversation.GroupMember{
			{AgentID: "agent:researcher", Position: 0},
			{AgentID: "agent:builder", Position: 1},
			{AgentID: "agent:reviewer", Position: 2},
		},
	}
	envelope := conversation.GroupTurnEnvelope{
		ID: "group-turn:1", GroupID: group.ID, ConversationID: group.ConversationID,
		ClientTurnID: "client-turn:1", IdempotencyKey: "send:1", MembershipRevisionID: membership.ID,
		Selection: conversation.GroupRecipientSelectionExplicit,
		Recipients: []conversation.GroupRecipient{
			{AgentID: "agent:researcher", BehaviorRevisionID: "behavior:researcher:2", BindingRevisionID: "binding:researcher:3", ParticipantID: "participant:researcher:3"},
			{AgentID: "agent:reviewer", BehaviorRevisionID: "behavior:reviewer:1", BindingRevisionID: "binding:reviewer:1", ParticipantID: "participant:reviewer:1"},
		},
		ContextSnapshotID: "context:1", RootDelegationGrant: conversation.AuthorityGrant{ID: "grant:group:1", Permissions: []string{"read"}},
		ConcurrencyPolicy:    conversation.GroupConcurrent,
		CancellationPolicyID: "group-cancel:human-or-deadline", CancellationPolicyRevision: "1",
		ApprovalPolicyID: "group-approval:explicit", ApprovalPolicyRevision: "1",
		MaxAgentMessages: conversation.MaxGroupAgentMessages, MaxHandoffDepth: conversation.MaxGroupHandoffDepth,
		CostLimitClass: conversation.LimitUnknown, TokenLimitClass: conversation.LimitUnknown,
		Deadline: now.Add(10 * time.Minute), CreatedAt: now,
	}

	targets, err := envelope.InitialTargets(group, membership)
	if err != nil {
		t.Fatalf("initial targets: %v", err)
	}
	if len(targets) != 2 {
		t.Fatalf("target count = %d, want 2", len(targets))
	}
	if targets[0].Wave != 0 || targets[0].AgentID != "agent:researcher" || targets[0].BindingRevisionID != "binding:researcher:3" {
		t.Fatalf("first frozen target = %#v", targets[0])
	}
	if targets[1].Wave != 0 || targets[1].AgentID != "agent:reviewer" || targets[1].BindingRevisionID != "binding:reviewer:1" {
		t.Fatalf("second frozen target = %#v", targets[1])
	}
}

func TestGroupTurnStartEnforcesSharedMessageAndDeadlineBounds(t *testing.T) {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	group := conversation.GroupConversation{
		ID: "group:launch", AccountID: "account:one", ConversationID: "conversation:launch",
		State: conversation.ConversationOpen, CurrentMembershipRevisionID: "membership:1", CreatedAt: now,
	}
	membership := conversation.GroupMembershipRevision{
		ID: "membership:1", GroupID: group.ID, Revision: 1, CreatedAt: now,
		Members: []conversation.GroupMember{{AgentID: "agent:researcher", Position: 0}, {AgentID: "agent:builder", Position: 1}},
	}
	envelope := conversation.GroupTurnEnvelope{
		ID: "group-turn:1", GroupID: group.ID, ConversationID: group.ConversationID,
		ClientTurnID: "client-turn:1", IdempotencyKey: "send:1", MembershipRevisionID: membership.ID,
		Selection: conversation.GroupRecipientSelectionExplicit,
		Recipients: []conversation.GroupRecipient{{
			AgentID: "agent:researcher", BehaviorRevisionID: "behavior:1",
			BindingRevisionID: "binding:1", ParticipantID: "participant:1",
		}},
		ContextSnapshotID: "context:1", RootDelegationGrant: conversation.AuthorityGrant{ID: "grant:group:1", Permissions: []string{"read"}},
		ConcurrencyPolicy:    conversation.GroupSequential,
		CancellationPolicyID: "group-cancel:human-or-deadline", CancellationPolicyRevision: "1",
		ApprovalPolicyID: "group-approval:explicit", ApprovalPolicyRevision: "1",
		MaxAgentMessages: conversation.MaxGroupAgentMessages, MaxHandoffDepth: conversation.MaxGroupHandoffDepth,
		CostLimitClass: conversation.LimitUnknown, TokenLimitClass: conversation.LimitUnknown,
		Deadline: now.Add(time.Minute), CreatedAt: now,
	}
	if err := envelope.CanStart(group, membership, now, 9); err != nil {
		t.Fatalf("start below bounds: %v", err)
	}
	if err := envelope.CanStart(group, membership, now, 10); !errors.Is(err, conversation.ErrGroupNeedsYou) {
		t.Fatalf("message-bound error = %v, want Needs You", err)
	}
	if err := envelope.CanStart(group, membership, envelope.Deadline, 0); !errors.Is(err, conversation.ErrGroupNeedsYou) {
		t.Fatalf("deadline-bound error = %v, want Needs You", err)
	}
}

func TestGroupTurnRequiresOpenGroupAndEvidenceForHardLimits(t *testing.T) {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	group := conversation.GroupConversation{
		ID: "group:launch", AccountID: "account:one", ConversationID: "conversation:launch",
		State: conversation.ConversationOpen, CurrentMembershipRevisionID: "membership:1", CreatedAt: now,
	}
	membership := conversation.GroupMembershipRevision{
		ID: "membership:1", GroupID: group.ID, Revision: 1, CreatedAt: now,
		Members: []conversation.GroupMember{{AgentID: "agent:researcher", Position: 0}, {AgentID: "agent:builder", Position: 1}},
	}
	envelope := conversation.GroupTurnEnvelope{
		ID: "group-turn:1", GroupID: group.ID, ConversationID: group.ConversationID,
		ClientTurnID: "client-turn:1", IdempotencyKey: "send:1", MembershipRevisionID: membership.ID,
		Selection: conversation.GroupRecipientSelectionExplicit,
		Recipients: []conversation.GroupRecipient{
			{AgentID: "agent:researcher", BehaviorRevisionID: "behavior:1", BindingRevisionID: "binding:1", ParticipantID: "participant:1"},
		},
		ContextSnapshotID: "context:1", RootDelegationGrant: conversation.AuthorityGrant{ID: "grant:group:1", Permissions: []string{"read"}},
		ConcurrencyPolicy:    conversation.GroupSequential,
		CancellationPolicyID: "group-cancel:human-or-deadline", CancellationPolicyRevision: "1",
		ApprovalPolicyID: "group-approval:explicit", ApprovalPolicyRevision: "1",
		MaxAgentMessages: conversation.MaxGroupAgentMessages, MaxHandoffDepth: conversation.MaxGroupHandoffDepth,
		CostLimitClass: conversation.LimitHard, TokenLimitClass: conversation.LimitUnknown,
		Deadline: now.Add(time.Minute), CreatedAt: now,
	}

	if err := envelope.Validate(group, membership); err == nil {
		t.Fatal("accepted a hard cost limit without pre-start enforceability evidence")
	}
	envelope.CostLimitEvidenceID = "evidence:adapter-cost-cap"
	if err := envelope.Validate(group, membership); err != nil {
		t.Fatalf("hard cost limit with evidence: %v", err)
	}

	group.State = conversation.ConversationArchived
	if err := envelope.Validate(group, membership); err == nil {
		t.Fatal("accepted a new turn in an archived Group")
	}
}

func TestGroupTurnRequiresExactCancellationAndApprovalPolicies(t *testing.T) {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	group := conversation.GroupConversation{
		ID: "group:launch", AccountID: "account:one", ConversationID: "conversation:launch",
		State: conversation.ConversationOpen, CurrentMembershipRevisionID: "membership:1", CreatedAt: now,
	}
	membership := conversation.GroupMembershipRevision{
		ID: "membership:1", GroupID: group.ID, Revision: 1, CreatedAt: now,
		Members: []conversation.GroupMember{{AgentID: "agent:researcher", Position: 0}, {AgentID: "agent:builder", Position: 1}},
	}
	envelope := conversation.GroupTurnEnvelope{
		ID: "group-turn:1", GroupID: group.ID, ConversationID: group.ConversationID,
		ClientTurnID: "client-turn:1", IdempotencyKey: "send:1", MembershipRevisionID: membership.ID,
		Selection: conversation.GroupRecipientSelectionExplicit,
		Recipients: []conversation.GroupRecipient{{
			AgentID: "agent:researcher", BehaviorRevisionID: "behavior:1",
			BindingRevisionID: "binding:1", ParticipantID: "participant:1",
		}},
		ContextSnapshotID: "context:1", RootDelegationGrant: conversation.AuthorityGrant{ID: "grant:group:1", Permissions: []string{"read"}},
		ConcurrencyPolicy: conversation.GroupSequential,
		MaxAgentMessages:  conversation.MaxGroupAgentMessages, MaxHandoffDepth: conversation.MaxGroupHandoffDepth,
		CostLimitClass: conversation.LimitUnknown, TokenLimitClass: conversation.LimitUnknown,
		Deadline: now.Add(time.Minute), CreatedAt: now,
	}
	if err := envelope.Validate(group, membership); err == nil {
		t.Fatal("accepted a Group Turn without frozen cancellation and approval policy revisions")
	}
	envelope.CancellationPolicyID = "group-cancel:human-or-deadline"
	envelope.CancellationPolicyRevision = "1"
	envelope.ApprovalPolicyID = "group-approval:explicit"
	envelope.ApprovalPolicyRevision = "1"
	if err := envelope.Validate(group, membership); err != nil {
		t.Fatalf("Group Turn with frozen policies: %v", err)
	}
}
