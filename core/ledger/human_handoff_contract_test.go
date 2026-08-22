package ledger_test

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/ledger"
)

func TestSQLiteHumanHandoffDerivesExactRecipientAndReplaysOriginalAllocation(t *testing.T) {
	repository := openCollaborationRepository(t)
	researcher := stableAgentCommandForCollaboration(t, "researcher", "Researcher")
	builder := stableAgentCommandForCollaboration(t, "builder", "Builder")
	createStableAgent(t, repository, researcher)
	createStableAgent(t, repository, builder)
	group := groupCommand(t, researcher, builder)
	if _, err := repository.CreateGroup(context.Background(), group); err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	turn, err := repository.SendGroupTurn(context.Background(), groupSendCommand(t, group))
	if err != nil {
		t.Fatalf("SendGroupTurn: %v", err)
	}
	now := turn.Envelope.CreatedAt.Add(time.Minute)
	command := ledger.CreateHumanHandoffCommand{
		IdempotencyKey: "human-handoff:one", AccountID: group.Group.AccountID,
		SourceConversationID: group.Conversation.ID, SourceMessageID: strconv.FormatInt(turn.Message.ID, 10),
		RecipientAgentID: builder.Agent.ID, ContextMessageIDs: []string{strconv.FormatInt(turn.Message.ID, 10)},
		RequestedResult: "Review the launch evidence.", ReplyToMessageID: strconv.FormatInt(turn.Message.ID, 10),
		HardDeadline: turn.Envelope.Deadline, HandoffID: "handoff:human:one",
		TargetID: "target:human:one", RootDelegationGrantID: "grant:human:one",
		CreatedByID: "human:toby", CreatedAt: now,
	}
	created, err := repository.CreateHumanHandoff(context.Background(), command)
	if err != nil {
		t.Fatalf("CreateHumanHandoff: %v", err)
	}
	if created.Handoff.ID != command.HandoffID || created.Handoff.CreatedByKind != conversation.HandoffActorHuman ||
		created.Handoff.RecipientBehaviorRevisionID != builder.Behavior.ID ||
		created.Handoff.RecipientBindingRevisionID != builder.Binding.ID ||
		created.Handoff.OutputConversationID != group.Conversation.ID || created.Handoff.GroupTurnID != turn.Envelope.ID ||
		created.Handoff.RootDelegationGrant.ID != turn.Envelope.RootDelegationGrant.ID ||
		created.Handoff.Deadline != turn.Envelope.Deadline || created.Target.ID != command.TargetID ||
		created.Target.ParticipantID != group.MemberBindings[1].ParticipantID {
		t.Fatalf("derived human Handoff = %+v", created)
	}
	if len(created.Handoff.RequestedAuthority) != 0 || len(created.Handoff.EffectiveAuthority.Permissions) != 0 ||
		len(created.Handoff.HandoffPolicy.Permissions) != 0 || len(created.Handoff.RecipientBindingPolicy.Permissions) != 0 ||
		created.Handoff.RecipientBindingPolicy.ID == "" || created.Handoff.HandoffPolicy.ID == "" {
		t.Fatalf("human Handoff authority was not fail-closed: %+v", created.Handoff)
	}
	if len(created.Projections) != 0 {
		t.Fatalf("Group Handoff must remain in its Group without a duplicate projection: %+v", created.Projections)
	}

	replay := command
	replay.HandoffID = "handoff:server-reallocated"
	replay.TargetID = "target:server-reallocated"
	replay.RootDelegationGrantID = "grant:server-reallocated"
	replay.CreatedAt = now.Add(time.Hour)
	replay.CreatedByID = "human:server-reformatted"
	replayed, err := repository.CreateHumanHandoff(context.Background(), replay)
	if err != nil {
		t.Fatalf("replay CreateHumanHandoff: %v", err)
	}
	if replayed.Handoff.ID != created.Handoff.ID || replayed.Target.ID != created.Target.ID {
		t.Fatalf("replay changed allocation: first %+v replay %+v", created, replayed)
	}
	conflict := replay
	conflict.RequestedResult = "A conflicting request."
	if _, err := repository.CreateHumanHandoff(context.Background(), conflict); !errors.Is(err, ledger.ErrIdempotencyConflict) {
		t.Fatalf("conflicting human Handoff error = %v, want %v", err, ledger.ErrIdempotencyConflict)
	}
}

func TestSQLiteDirectHumanHandoffCreatesNewRootAndPinsCurrentRecipientHome(t *testing.T) {
	repository := openCollaborationRepository(t)
	sender := stableAgentCommandForCollaboration(t, "sender", "Sender")
	recipient := stableAgentCommandForCollaboration(t, "recipient", "Recipient")
	createStableAgent(t, repository, sender)
	createStableAgent(t, repository, recipient)
	chat := repository.(ledger.AgentDirectChatRepository)
	direct := directAgentTurnCommand(sender, sender.Home.ID, "human-handoff-source")
	sent, err := chat.SendAgentTurn(context.Background(), direct)
	if err != nil {
		t.Fatalf("SendAgentTurn: %v", err)
	}
	sourceMessageID := strconv.FormatInt(sent.Message.ID, 10)
	command := ledger.CreateHumanHandoffCommand{
		IdempotencyKey: "human-handoff:direct", AccountID: sender.Agent.AccountID,
		SourceConversationID: sender.Home.ID, SourceMessageID: sourceMessageID,
		RecipientAgentID: recipient.Agent.ID, ContextMessageIDs: []string{sourceMessageID},
		RequestedResult: "Review the direct request.", ReplyToMessageID: sourceMessageID,
		HardDeadline: direct.CreatedAt.Add(20 * time.Minute), HandoffID: "handoff:direct",
		TargetID: "target:direct", RootDelegationGrantID: "grant:direct",
		CreatedByID: "human:toby", CreatedAt: direct.CreatedAt.Add(time.Minute),
	}
	created, err := repository.CreateHumanHandoff(context.Background(), command)
	if err != nil {
		t.Fatalf("CreateHumanHandoff: %v", err)
	}
	if created.Handoff.GroupTurnID != "" || created.Handoff.OutputConversationID != recipient.Home.ID ||
		created.Handoff.RootDelegationGrant.ID != command.RootDelegationGrantID ||
		created.Handoff.RecipientBehaviorRevisionID != recipient.Behavior.ID ||
		created.Handoff.RecipientBindingRevisionID != recipient.Binding.ID ||
		created.Target.ParticipantID != recipient.Participant.ID || len(created.Projections) != 1 ||
		created.Projections[0].ConversationID != sender.Home.ID {
		t.Fatalf("direct human Handoff = %+v", created)
	}
}

func TestSQLiteHumanHandoffCancellationPreservesExactQueuedAndWorkingTargets(t *testing.T) {
	repository := openCollaborationRepository(t)
	researcher := stableAgentCommandForCollaboration(t, "researcher", "Researcher")
	builder := stableAgentCommandForCollaboration(t, "builder", "Builder")
	createStableAgent(t, repository, researcher)
	createStableAgent(t, repository, builder)
	group := groupCommand(t, researcher, builder)
	if _, err := repository.CreateGroup(context.Background(), group); err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	turn, err := repository.SendGroupTurn(context.Background(), groupSendCommand(t, group))
	if err != nil {
		t.Fatalf("SendGroupTurn: %v", err)
	}
	now := turn.Envelope.CreatedAt.Add(time.Minute)
	base := ledger.CreateHumanHandoffCommand{
		IdempotencyKey: "human-handoff:queued", AccountID: group.Group.AccountID,
		SourceConversationID: group.Conversation.ID, SourceMessageID: strconv.FormatInt(turn.Message.ID, 10),
		RecipientAgentID: builder.Agent.ID, RequestedResult: "Review it.",
		HardDeadline: turn.Envelope.Deadline, HandoffID: "handoff:queued", TargetID: "target:queued",
		RootDelegationGrantID: "grant:queued", CreatedByID: "human:toby", CreatedAt: now,
	}
	queued, err := repository.CreateHumanHandoff(context.Background(), base)
	if err != nil {
		t.Fatalf("create queued Handoff: %v", err)
	}
	cancel := ledger.CancelHandoffCommand{IdempotencyKey: "cancel:queued", AccountID: base.AccountID,
		HandoffID: base.HandoffID, CanceledBy: "human:toby", CanceledAt: now.Add(time.Second)}
	canceled, err := repository.CancelHandoff(context.Background(), cancel)
	if err != nil {
		t.Fatalf("CancelHandoff queued: %v", err)
	}
	if canceled.Handoff.State != conversation.HandoffCanceled || canceled.Target.State != conversation.TargetCanceled ||
		canceled.Target.ID != queued.Target.ID || canceled.Cancellation == nil ||
		canceled.Cancellation.State != ledger.HandoffCancellationCanceled ||
		canceled.Cancellation.BindingRevisionID != queued.Target.BindingRevisionID {
		t.Fatalf("queued cancellation = %+v", canceled)
	}
	replay := cancel
	replay.CanceledAt = cancel.CanceledAt.Add(time.Hour)
	replay.CanceledBy = "human:server-reformatted"
	if got, err := repository.CancelHandoff(context.Background(), replay); err != nil || got.Target.ID != queued.Target.ID {
		t.Fatalf("replay queued cancellation = %+v, %v", got, err)
	}

	workingCommand := base
	workingCommand.IdempotencyKey = "human-handoff:working"
	workingCommand.HandoffID = "handoff:working"
	workingCommand.TargetID = "target:working"
	workingCommand.RootDelegationGrantID = "grant:working"
	working, err := repository.CreateHumanHandoff(context.Background(), workingCommand)
	if err != nil {
		t.Fatalf("create working Handoff: %v", err)
	}
	start := ledger.StartHandoffCommand{AccountID: base.AccountID, HandoffID: working.Handoff.ID,
		IdempotencyKey: "start:working", AttemptID: "attempt:working", LeaseID: "lease:working",
		MachineID: builder.Binding.ComputerID, FenceToken: "fence:working",
		StartedAt: now.Add(2 * time.Second), LeaseExpiresAt: now.Add(5 * time.Minute)}
	if _, err := repository.StartHandoff(context.Background(), start); err != nil {
		t.Fatalf("StartHandoff: %v", err)
	}
	workingCancel := ledger.CancelHandoffCommand{IdempotencyKey: "cancel:working", AccountID: base.AccountID,
		HandoffID: working.Handoff.ID, CanceledBy: "human:toby", CanceledAt: now.Add(3 * time.Second)}
	requested, err := repository.CancelHandoff(context.Background(), workingCancel)
	if err != nil {
		t.Fatalf("CancelHandoff working: %v", err)
	}
	if requested.Handoff.State != conversation.HandoffCanceled || requested.Target.ID != working.Target.ID ||
		requested.Target.State != conversation.TargetWorking || requested.Cancellation == nil ||
		requested.Cancellation.State != ledger.HandoffCancellationRequested ||
		requested.Cancellation.BehaviorRevisionID != working.Target.BehaviorRevisionID ||
		requested.Cancellation.BindingRevisionID != working.Target.BindingRevisionID {
		t.Fatalf("working cancellation = %+v", requested)
	}
	if _, err := repository.GetHandoff(context.Background(), "account:foreign", working.Handoff.ID); !errors.Is(err, ledger.ErrNotFound) {
		t.Fatalf("foreign GetHandoff error = %v, want %v", err, ledger.ErrNotFound)
	}
}
