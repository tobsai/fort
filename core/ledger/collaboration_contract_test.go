package ledger_test

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/ledger"
	"github.com/tobsai/fort/core/store"
)

func TestSQLiteCollaborationLedgerCreatesGroupAndOneFrozenFanoutAtomically(t *testing.T) {
	repository := openCollaborationRepository(t)
	researcher := stableAgentCommandForCollaboration(t, "researcher", "Researcher")
	builder := stableAgentCommandForCollaboration(t, "builder", "Builder")
	createStableAgent(t, repository, researcher)
	createStableAgent(t, repository, builder)

	create := groupCommand(t, researcher, builder)
	created, err := repository.CreateGroup(context.Background(), create)
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	if created.Group.ID != create.Group.ID || created.Conversation.ID != create.Conversation.ID {
		t.Fatalf("created Group = %+v", created)
	}
	if len(created.Membership.Members) != 2 || len(created.MemberBindings) != 2 {
		t.Fatalf("created membership = %+v bindings = %+v", created.Membership, created.MemberBindings)
	}
	replayedGroup, err := repository.CreateGroup(context.Background(), create)
	if err != nil {
		t.Fatalf("replay CreateGroup: %v", err)
	}
	if replayedGroup.Group.ID != created.Group.ID || replayedGroup.Membership.ID != created.Membership.ID {
		t.Fatalf("replayed Group changed identity: %+v", replayedGroup)
	}

	send := groupSendCommand(t, create)
	turn, err := repository.SendGroupTurn(context.Background(), send)
	if err != nil {
		t.Fatalf("SendGroupTurn: %v", err)
	}
	if turn.Message.ID == 0 || turn.Message.Body != send.Body || turn.Message.AuthorID != send.HumanID {
		t.Fatalf("persisted human message = %+v", turn.Message)
	}
	if len(turn.Recipients) != 2 || len(turn.InitialTargets) != 2 {
		t.Fatalf("frozen recipients = %+v targets = %+v", turn.Recipients, turn.InitialTargets)
	}
	if turn.Envelope.CancellationPolicyID != send.Envelope.CancellationPolicyID ||
		turn.Envelope.CancellationPolicyRevision != send.Envelope.CancellationPolicyRevision ||
		turn.Envelope.ApprovalPolicyID != send.Envelope.ApprovalPolicyID ||
		turn.Envelope.ApprovalPolicyRevision != send.Envelope.ApprovalPolicyRevision {
		t.Fatalf("persisted Group Turn lost cancellation or approval policy: %+v", turn.Envelope)
	}
	if turn.Envelope.RootDelegationGrant.ID != send.Envelope.RootDelegationGrant.ID ||
		!reflect.DeepEqual(turn.Envelope.RootDelegationGrant.Permissions, send.Envelope.RootDelegationGrant.Permissions) ||
		!reflect.DeepEqual(turn.Envelope.RootDelegationGrant.ContextRecordIDs,
			[]string{"message:" + strconv.FormatInt(turn.Message.ID, 10)}) {
		t.Fatalf("persisted Group Turn did not authorize its frozen context: %+v", turn.Envelope.RootDelegationGrant)
	}
	for i, target := range turn.InitialTargets {
		if target.Wave != 0 || target.AgentID != send.Envelope.Recipients[i].AgentID ||
			target.BindingRevisionID != send.Envelope.Recipients[i].BindingRevisionID ||
			target.ID != send.TargetIDs[i] {
			t.Fatalf("initial target %d = %+v", i, target)
		}
	}

	replayedTurn, err := repository.SendGroupTurn(context.Background(), send)
	if err != nil {
		t.Fatalf("replay SendGroupTurn: %v", err)
	}
	if replayedTurn.Message.ID != turn.Message.ID || len(replayedTurn.InitialTargets) != len(turn.InitialTargets) {
		t.Fatalf("replayed Group Turn duplicated work: first %+v replay %+v", turn, replayedTurn)
	}
	conflict := send
	conflict.Body = "A different command under the same key."
	if _, err := repository.SendGroupTurn(context.Background(), conflict); !errors.Is(err, ledger.ErrIdempotencyConflict) {
		t.Fatalf("conflicting Group Send error = %v, want %v", err, ledger.ErrIdempotencyConflict)
	}

	invalid := send
	invalid.Envelope.ID = "group-turn:invalid"
	invalid.Envelope.ClientTurnID = "client-turn:invalid"
	invalid.Envelope.IdempotencyKey = "group-send:invalid"
	invalid.TargetIDs = []string{"target:invalid:researcher", "target:invalid:builder"}
	invalid.Envelope.Recipients = append([]conversation.GroupRecipient(nil), send.Envelope.Recipients...)
	invalid.Envelope.Recipients[1].BindingRevisionID = "binding:foreign"
	if _, err := repository.SendGroupTurn(context.Background(), invalid); err == nil {
		t.Fatal("SendGroupTurn accepted a recipient binding without persisted evidence")
	}
	turns, err := repository.ListGroupTurns(context.Background(), create.Group.AccountID, create.Group.ID)
	if err != nil {
		t.Fatalf("ListGroupTurns: %v", err)
	}
	if len(turns) != 1 || turns[0].Envelope.ID != send.Envelope.ID {
		t.Fatalf("failed Group Send persisted partial rows: %+v", turns)
	}

	groups, err := repository.ListGroups(context.Background(), create.Group.AccountID, conversation.ConversationOpen)
	if err != nil {
		t.Fatalf("ListGroups: %v", err)
	}
	if len(groups) != 1 || groups[0].Group.ID != create.Group.ID {
		t.Fatalf("listed Groups = %+v", groups)
	}
	foreign, err := repository.ListGroups(context.Background(), "account:foreign", conversation.ConversationOpen)
	if err != nil {
		t.Fatalf("ListGroups foreign account: %v", err)
	}
	if foreign == nil || len(foreign) != 0 {
		t.Fatalf("foreign-account Groups = %#v, want non-nil []", foreign)
	}
	if _, err := repository.GetGroup(context.Background(), "account:foreign", create.Group.ID); !errors.Is(err, ledger.ErrNotFound) {
		t.Fatalf("foreign-account GetGroup error = %v, want %v", err, ledger.ErrNotFound)
	}
}

func TestSQLiteCollaborationLedgerManagesGroupLifecycleAndImmutableMembership(t *testing.T) {
	repository := openCollaborationRepository(t)
	researcher := stableAgentCommandForCollaboration(t, "researcher", "Researcher")
	builder := stableAgentCommandForCollaboration(t, "builder", "Builder")
	reviewer := stableAgentCommandForCollaboration(t, "reviewer", "Reviewer")
	createStableAgent(t, repository, researcher)
	createStableAgent(t, repository, builder)
	createStableAgent(t, repository, reviewer)

	create := groupCommand(t, researcher, builder)
	if _, err := repository.CreateGroup(context.Background(), create); err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	now := create.Group.CreatedAt.Add(time.Minute)
	renamed, err := repository.RenameGroup(context.Background(), ledger.RenameGroupCommand{
		IdempotencyKey: "group:rename:launch", AccountID: create.Group.AccountID, GroupID: create.Group.ID,
		ExpectedTitle: create.Conversation.Title, Title: "Launch council", ChangedBy: "human:toby", ChangedAt: now,
	})
	if err != nil {
		t.Fatalf("RenameGroup: %v", err)
	}
	if renamed.Conversation.Title != "Launch council" || renamed.Group.ID != create.Group.ID {
		t.Fatalf("renamed Group = %+v", renamed)
	}
	if _, err := repository.RenameGroup(context.Background(), ledger.RenameGroupCommand{
		IdempotencyKey: "group:rename:stale", AccountID: create.Group.AccountID, GroupID: create.Group.ID,
		ExpectedTitle: create.Conversation.Title, Title: "Stale rename", ChangedBy: "human:toby", ChangedAt: now.Add(time.Second),
	}); !errors.Is(err, ledger.ErrRevisionConflict) {
		t.Fatalf("stale rename error = %v, want %v", err, ledger.ErrRevisionConflict)
	}

	archived, err := repository.SetGroupState(context.Background(), ledger.SetGroupStateCommand{
		IdempotencyKey: "group:archive:launch", AccountID: create.Group.AccountID, GroupID: create.Group.ID,
		ExpectedState: conversation.ConversationOpen, State: conversation.ConversationArchived,
		ChangedBy: "human:toby", ChangedAt: now.Add(2 * time.Second),
	})
	if err != nil || archived.Group.State != conversation.ConversationArchived ||
		archived.Conversation.State != conversation.ConversationArchived {
		t.Fatalf("archive Group = %+v, %v", archived, err)
	}

	replacement := ledger.ReplaceGroupMembersCommand{
		IdempotencyKey: "group:members:launch:2", AccountID: create.Group.AccountID, GroupID: create.Group.ID,
		ExpectedMembershipRevisionID: create.Membership.ID,
		Membership: conversation.GroupMembershipRevision{
			ID: "membership:launch:2", GroupID: create.Group.ID, Revision: 2,
			Members:   []conversation.GroupMember{{AgentID: builder.Agent.ID, Position: 0}, {AgentID: reviewer.Agent.ID, Position: 1}},
			CreatedAt: now.Add(3 * time.Second),
		},
		MemberBindings: []conversation.GroupRecipient{
			{AgentID: builder.Agent.ID, BehaviorRevisionID: builder.Behavior.ID, BindingRevisionID: builder.Binding.ID, ParticipantID: create.MemberBindings[1].ParticipantID},
			{AgentID: reviewer.Agent.ID, BehaviorRevisionID: reviewer.Behavior.ID, BindingRevisionID: reviewer.Binding.ID, ParticipantID: "participant:launch:2:reviewer"},
		},
		ChangedBy: "human:toby", ChangedAt: now.Add(3 * time.Second),
	}
	if _, err := repository.ReplaceGroupMembers(context.Background(), replacement); !errors.Is(err, ledger.ErrStateConflict) {
		t.Fatalf("archived membership replacement error = %v, want %v", err, ledger.ErrStateConflict)
	}

	reopened, err := repository.SetGroupState(context.Background(), ledger.SetGroupStateCommand{
		IdempotencyKey: "group:reopen:launch", AccountID: create.Group.AccountID, GroupID: create.Group.ID,
		ExpectedState: conversation.ConversationArchived, State: conversation.ConversationOpen,
		ChangedBy: "human:toby", ChangedAt: now.Add(4 * time.Second),
	})
	if err != nil || reopened.Group.State != conversation.ConversationOpen {
		t.Fatalf("reopen Group = %+v, %v", reopened, err)
	}
	replaced, err := repository.ReplaceGroupMembers(context.Background(), replacement)
	if err != nil {
		t.Fatalf("ReplaceGroupMembers: %v", err)
	}
	if replaced.Membership.ID != replacement.Membership.ID || replaced.Membership.Revision != 2 ||
		!reflect.DeepEqual(replaced.Membership.Members, replacement.Membership.Members) ||
		!reflect.DeepEqual(replaced.MemberBindings, replacement.MemberBindings) {
		t.Fatalf("replacement membership = %+v bindings = %+v", replaced.Membership, replaced.MemberBindings)
	}
	replayed, err := repository.ReplaceGroupMembers(context.Background(), replacement)
	if err != nil || replayed.Membership.ID != replacement.Membership.ID {
		t.Fatalf("replay ReplaceGroupMembers = %+v, %v", replayed, err)
	}
	stale := replacement
	stale.IdempotencyKey = "group:members:stale"
	stale.Membership.ID = "membership:launch:3"
	stale.Membership.Revision = 3
	stale.Membership.CreatedAt = now.Add(5 * time.Second)
	stale.ChangedAt = stale.Membership.CreatedAt
	for index := range stale.MemberBindings {
		stale.MemberBindings[index].ParticipantID += ":stale"
	}
	if _, err := repository.ReplaceGroupMembers(context.Background(), stale); !errors.Is(err, ledger.ErrRevisionConflict) {
		t.Fatalf("stale membership error = %v, want %v", err, ledger.ErrRevisionConflict)
	}
	loaded, err := repository.GetGroup(context.Background(), create.Group.AccountID, create.Group.ID)
	if err != nil || loaded.Membership.ID != replacement.Membership.ID {
		t.Fatalf("failed stale replacement changed current membership: %+v, %v", loaded, err)
	}

	activeCreate := groupCommand(t, researcher, reviewer)
	activeCreate.IdempotencyKey = "create-group:active"
	activeCreate.Group.ID = "group:active"
	activeCreate.Group.ConversationID = "conversation:group:active"
	activeCreate.Group.CurrentMembershipRevisionID = "membership:active:1"
	activeCreate.Conversation.ID = activeCreate.Group.ConversationID
	activeCreate.Membership.ID = activeCreate.Group.CurrentMembershipRevisionID
	activeCreate.Membership.GroupID = activeCreate.Group.ID
	for index := range activeCreate.MemberBindings {
		activeCreate.MemberBindings[index].ParticipantID = "participant:active:" + activeCreate.MemberBindings[index].AgentID
	}
	if _, err := repository.CreateGroup(context.Background(), activeCreate); err != nil {
		t.Fatalf("CreateGroup(active): %v", err)
	}
	activeSend := groupSendCommand(t, activeCreate)
	activeSend.Envelope.ID = "group-turn:active:1"
	activeSend.Envelope.ClientTurnID = "client-turn:active:1"
	activeSend.Envelope.IdempotencyKey = "group-send:active:1"
	activeSend.Envelope.ContextSnapshotID = "context:active:1"
	activeSend.TargetIDs = []string{"target:active:researcher", "target:active:reviewer"}
	if _, err := repository.SendGroupTurn(context.Background(), activeSend); err != nil {
		t.Fatalf("SendGroupTurn(active): %v", err)
	}
	if _, err := repository.RenameGroup(context.Background(), ledger.RenameGroupCommand{
		IdempotencyKey: "group:rename:active", AccountID: activeCreate.Group.AccountID, GroupID: activeCreate.Group.ID,
		ExpectedTitle: activeCreate.Conversation.Title, Title: "Blocked rename",
		ChangedBy: "human:toby", ChangedAt: now.Add(6 * time.Second),
	}); !errors.Is(err, ledger.ErrStateConflict) {
		t.Fatalf("active Group rename error = %v, want %v", err, ledger.ErrStateConflict)
	}
	activeReplacement := replacement
	activeReplacement.IdempotencyKey = "group:members:active:2"
	activeReplacement.GroupID = activeCreate.Group.ID
	activeReplacement.ExpectedMembershipRevisionID = activeCreate.Membership.ID
	activeReplacement.Membership.ID = "membership:active:2"
	activeReplacement.Membership.GroupID = activeCreate.Group.ID
	activeReplacement.Membership.Revision = 2
	activeReplacement.Membership.CreatedAt = now.Add(6 * time.Second)
	activeReplacement.ChangedAt = activeReplacement.Membership.CreatedAt
	for index := range activeReplacement.MemberBindings {
		activeReplacement.MemberBindings[index].ParticipantID = "participant:active:2:" + activeReplacement.MemberBindings[index].AgentID
	}
	if _, err := repository.ReplaceGroupMembers(context.Background(), activeReplacement); !errors.Is(err, ledger.ErrStateConflict) {
		t.Fatalf("active Group membership error = %v, want %v", err, ledger.ErrStateConflict)
	}
	if _, err := repository.SetGroupState(context.Background(), ledger.SetGroupStateCommand{
		IdempotencyKey: "group:archive:active", AccountID: activeCreate.Group.AccountID, GroupID: activeCreate.Group.ID,
		ExpectedState: conversation.ConversationOpen, State: conversation.ConversationArchived,
		ChangedBy: "human:toby", ChangedAt: now.Add(7 * time.Second),
	}); !errors.Is(err, ledger.ErrStateConflict) {
		t.Fatalf("active Group archive error = %v, want %v", err, ledger.ErrStateConflict)
	}
}

func TestSQLiteCollaborationLedgerAcceptsAndCompletesOneHandoffResult(t *testing.T) {
	repository := openCollaborationRepository(t)
	researcher := stableAgentCommandForCollaboration(t, "researcher", "Researcher")
	builder := stableAgentCommandForCollaboration(t, "builder", "Builder")
	createStableAgent(t, repository, researcher)
	createStableAgent(t, repository, builder)
	createGroup := groupCommand(t, researcher, builder)
	if _, err := repository.CreateGroup(context.Background(), createGroup); err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	groupTurn, err := repository.SendGroupTurn(context.Background(), groupSendCommand(t, createGroup))
	if err != nil {
		t.Fatalf("SendGroupTurn: %v", err)
	}

	accept := handoffCommand(t, groupTurn, createGroup, builder)
	accepted, err := repository.AcceptHandoff(context.Background(), accept)
	if err != nil {
		t.Fatalf("AcceptHandoff: %v", err)
	}
	if accepted.Handoff.OutputConversationID != builder.Home.ID ||
		!reflect.DeepEqual(accepted.Handoff.Context, accept.Handoff.Context) ||
		!reflect.DeepEqual(accepted.Handoff.EffectiveAuthority, accept.Handoff.EffectiveAuthority) {
		t.Fatalf("accepted Handoff lost exact output, context, or authority: %+v", accepted.Handoff)
	}
	if accepted.Target.ID != accept.TargetID || accepted.Target.ParticipantID != builder.Participant.ID ||
		accepted.Target.AgentID != builder.Agent.ID || accepted.Target.State != conversation.TargetQueued {
		t.Fatalf("accepted Handoff target = %+v", accepted.Target)
	}
	if len(accepted.Projections) != 1 || accepted.Projections[0].ConversationID != createGroup.Conversation.ID ||
		accepted.Projections[0].OutputConversationID != builder.Home.ID || accepted.Projections[0].AuthoritativeMessageID != "" {
		t.Fatalf("queued reference-only projections = %+v", accepted.Projections)
	}
	replayed, err := repository.AcceptHandoff(context.Background(), accept)
	if err != nil {
		t.Fatalf("replay AcceptHandoff: %v", err)
	}
	if replayed.Handoff.ID != accepted.Handoff.ID || replayed.Target.ID != accepted.Target.ID {
		t.Fatalf("replayed Handoff duplicated work: first %+v replay %+v", accepted, replayed)
	}
	conflict := accept
	conflict.Handoff.RequestedResult = "A conflicting requested result."
	if _, err := repository.AcceptHandoff(context.Background(), conflict); !errors.Is(err, ledger.ErrIdempotencyConflict) {
		t.Fatalf("conflicting Handoff acceptance error = %v, want %v", err, ledger.ErrIdempotencyConflict)
	}

	invalid := accept
	invalid.Handoff.ID = "handoff:invalid"
	invalid.Handoff.IdempotencyKey = "handoff-command:invalid"
	invalid.TargetID = "target:handoff:invalid"
	invalid.ParticipantID = researcher.Participant.ID
	if _, err := repository.AcceptHandoff(context.Background(), invalid); err == nil {
		t.Fatal("AcceptHandoff accepted recipient participant evidence for another Agent")
	}
	handoffs, err := repository.ListHandoffs(context.Background(), accept.Handoff.AccountID)
	if err != nil {
		t.Fatalf("ListHandoffs: %v", err)
	}
	if len(handoffs) != 1 || handoffs[0].Handoff.ID != accept.Handoff.ID {
		t.Fatalf("failed Handoff acceptance persisted partial rows: %+v", handoffs)
	}

	start := ledger.StartHandoffCommand{
		AccountID: accept.Handoff.AccountID, HandoffID: accept.Handoff.ID,
		IdempotencyKey: "start:handoff:1", AttemptID: "attempt:handoff:1",
		LeaseID: "lease:handoff:1", MachineID: builder.Binding.ComputerID, FenceToken: "fence:handoff:1",
		StartedAt:      accept.Handoff.CreatedAt.Add(30 * time.Second),
		LeaseExpiresAt: accept.Handoff.CreatedAt.Add(5 * time.Minute),
	}
	complete := ledger.CompleteHandoffCommand{
		AccountID: accept.Handoff.AccountID, HandoffID: accept.Handoff.ID,
		IdempotencyKey: "complete:handoff:1", AuthorAgentID: builder.Agent.ID,
		AttemptID: start.AttemptID, LeaseID: start.LeaseID, FenceToken: start.FenceToken,
		TerminalReceiptID: "receipt:handoff:1", Body: "The requested evidence is ready.",
		CreatedAt: accept.Handoff.CreatedAt.Add(time.Minute),
	}
	if _, err := repository.CompleteHandoff(context.Background(), complete); err == nil {
		t.Fatal("CompleteHandoff accepted queued work without a persisted attempt and lease")
	}
	started, err := repository.StartHandoff(context.Background(), start)
	if err != nil {
		t.Fatalf("StartHandoff: %v", err)
	}
	if started.Handoff.State != conversation.HandoffWorking || started.Target.State != conversation.TargetWorking ||
		started.Attempt == nil || started.Attempt.ID != start.AttemptID || started.Attempt.FenceToken != start.FenceToken {
		t.Fatalf("started Handoff attempt = %+v", started)
	}
	replayedStart, err := repository.StartHandoff(context.Background(), start)
	if err != nil || replayedStart.Attempt == nil || replayedStart.Attempt.ID != start.AttemptID {
		t.Fatalf("replay StartHandoff = %+v, %v", replayedStart, err)
	}
	stale := complete
	stale.IdempotencyKey = "complete:handoff:stale"
	stale.FenceToken = "fence:stale"
	if _, err := repository.CompleteHandoff(context.Background(), stale); err == nil {
		t.Fatal("CompleteHandoff accepted a stale fence token")
	}
	completed, err := repository.CompleteHandoff(context.Background(), complete)
	if err != nil {
		t.Fatalf("CompleteHandoff: %v", err)
	}
	if completed.Handoff.State != conversation.HandoffCompleted || completed.Result == nil ||
		completed.Result.OutputConversationID != builder.Home.ID || completed.Result.Body != complete.Body ||
		completed.Result.MessageID == "" {
		t.Fatalf("completed authoritative Handoff result = %+v", completed)
	}
	if len(completed.Projections) != 1 ||
		completed.Projections[0].AuthoritativeMessageID != completed.Result.MessageID ||
		completed.Projections[0].State != conversation.HandoffCompleted {
		t.Fatalf("completed reference-only projection linkage = %+v", completed.Projections)
	}
	replayedCompletion, err := repository.CompleteHandoff(context.Background(), complete)
	if err != nil {
		t.Fatalf("replay CompleteHandoff: %v", err)
	}
	if replayedCompletion.Result == nil || replayedCompletion.Result.MessageID != completed.Result.MessageID {
		t.Fatalf("replayed completion created another result: first %+v replay %+v", completed.Result, replayedCompletion.Result)
	}
	second := complete
	second.IdempotencyKey = "complete:handoff:second"
	second.Body = "A conflicting second result."
	if _, err := repository.CompleteHandoff(context.Background(), second); !errors.Is(err, ledger.ErrAlreadyCompleted) {
		t.Fatalf("second completion error = %v, want %v", err, ledger.ErrAlreadyCompleted)
	}

	loaded, err := repository.GetHandoff(context.Background(), accept.Handoff.AccountID, accept.Handoff.ID)
	if err != nil {
		t.Fatalf("GetHandoff: %v", err)
	}
	if loaded.Result == nil || loaded.Result.MessageID != completed.Result.MessageID {
		t.Fatalf("loaded Handoff result = %+v", loaded.Result)
	}
	empty, err := repository.ListHandoffs(context.Background(), "account:foreign")
	if err != nil {
		t.Fatalf("ListHandoffs foreign account: %v", err)
	}
	if empty == nil || len(empty) != 0 {
		t.Fatalf("foreign-account Handoffs = %#v, want non-nil []", empty)
	}
}

func TestGroupCommandDigestsIgnoreControlAllocatedIdentityAndFrozenRevisionEvidence(t *testing.T) {
	researcher := stableAgentCommandForCollaboration(t, "researcher", "Researcher")
	builder := stableAgentCommandForCollaboration(t, "builder", "Builder")
	create := groupCommand(t, researcher, builder)
	createdDigest, err := create.Digest()
	if err != nil {
		t.Fatalf("CreateGroup Digest: %v", err)
	}
	replayedCreate := create
	replayedCreate.Group.ID = "group:other-control-allocation"
	replayedCreate.Group.ConversationID = "conversation:other-control-allocation"
	replayedCreate.Group.CurrentMembershipRevisionID = "membership:other-control-allocation"
	replayedCreate.Group.CreatedAt = replayedCreate.Group.CreatedAt.Add(time.Hour)
	replayedCreate.Conversation.ID = replayedCreate.Group.ConversationID
	replayedCreate.Conversation.CreatedAt = replayedCreate.Group.CreatedAt
	replayedCreate.Conversation.UpdatedAt = replayedCreate.Group.CreatedAt
	replayedCreate.Membership.ID = replayedCreate.Group.CurrentMembershipRevisionID
	replayedCreate.Membership.GroupID = replayedCreate.Group.ID
	replayedCreate.Membership.CreatedAt = replayedCreate.Group.CreatedAt
	replayedCreate.MemberBindings = append([]conversation.GroupRecipient{}, replayedCreate.MemberBindings...)
	for index := range replayedCreate.MemberBindings {
		replayedCreate.MemberBindings[index].BehaviorRevisionID += ":new"
		replayedCreate.MemberBindings[index].BindingRevisionID += ":new"
		replayedCreate.MemberBindings[index].ParticipantID += ":new"
	}
	replayedDigest, err := replayedCreate.Digest()
	if err != nil || replayedDigest != createdDigest {
		t.Fatalf("server-derived CreateGroup fields changed digest: %s != %s, err=%v", replayedDigest, createdDigest, err)
	}
	replayedCreate.Conversation.Title = "Different client title"
	if conflictDigest, _ := replayedCreate.Digest(); conflictDigest == createdDigest {
		t.Fatal("client-visible Group title did not change digest")
	}

	send := groupSendCommand(t, create)
	sendDigest, err := send.Digest()
	if err != nil {
		t.Fatalf("SendGroupTurn Digest: %v", err)
	}
	replayedSend := send
	replayedSend.Envelope.ID = "group-turn:other-control-allocation"
	replayedSend.Envelope.MembershipRevisionID = "membership:new"
	replayedSend.Envelope.ContextSnapshotID = "context:new"
	replayedSend.Envelope.CreatedAt = replayedSend.Envelope.CreatedAt.Add(time.Hour)
	replayedSend.Envelope.Recipients = append([]conversation.GroupRecipient{}, replayedSend.Envelope.Recipients...)
	for index := range replayedSend.Envelope.Recipients {
		replayedSend.Envelope.Recipients[index].BehaviorRevisionID += ":new"
		replayedSend.Envelope.Recipients[index].BindingRevisionID += ":new"
		replayedSend.Envelope.Recipients[index].ParticipantID += ":new"
	}
	replayedSend.TargetIDs = []string{"target:new:one", "target:new:two"}
	replayedSendDigest, err := replayedSend.Digest()
	if err != nil || replayedSendDigest != sendDigest {
		t.Fatalf("server-derived Group Send fields changed digest: %s != %s, err=%v", replayedSendDigest, sendDigest, err)
	}
	replayedSend.Body = "Different client prompt"
	if conflictDigest, _ := replayedSend.Digest(); conflictDigest == sendDigest {
		t.Fatal("client-visible Group prompt did not change digest")
	}
}

func openCollaborationRepository(t *testing.T) ledger.CollaborationRepository {
	t.Helper()
	repository, err := store.Open(filepath.Join(t.TempDir(), "fort.db"))
	if err != nil {
		t.Fatalf("open Store: %v", err)
	}
	t.Cleanup(func() { _ = repository.Close() })
	return repository
}

func createStableAgent(t *testing.T, repository ledger.CollaborationRepository, command ledger.CreateAgentCommand) {
	t.Helper()
	if _, err := repository.CreateAgent(context.Background(), command); err != nil {
		t.Fatalf("CreateAgent(%s): %v", command.Agent.ID, err)
	}
}

func stableAgentCommandForCollaboration(t *testing.T, slug, displayName string) ledger.CreateAgentCommand {
	t.Helper()
	command := stableAgentCommand(t)
	command.IdempotencyKey = "create-" + slug
	command.Agent.ID = "agent:" + slug
	command.Agent.CurrentProfileRevisionID = "profile:" + slug + ":1"
	command.Agent.CurrentBehaviorRevisionID = "behavior:" + slug + ":1"
	command.Agent.CurrentBindingRevisionID = "binding:" + slug + ":1"
	command.Agent.CanonicalConversationID = "conversation:" + slug + ":home"
	command.Profile.ID = command.Agent.CurrentProfileRevisionID
	command.Profile.AgentID = command.Agent.ID
	command.Profile.Name = displayName
	command.Profile.Title = displayName + " Agent"
	command.Behavior.ID = command.Agent.CurrentBehaviorRevisionID
	command.Behavior.AgentID = command.Agent.ID
	command.Behavior.Role = displayName
	command.Binding.ID = command.Agent.CurrentBindingRevisionID
	command.Binding.AgentID = command.Agent.ID
	command.Binding.BehaviorRevisionID = command.Behavior.ID
	command.Binding.SourceAgentID = "source-agent:studio:" + slug
	command.Binding.SeatID = "seat:" + slug
	command.Binding.FortProfile = "openclaw:" + slug
	command.SourceAgent.ID = command.Binding.SourceAgentID
	command.SourceAgent.OpaqueSourceAgentID = slug
	command.SourceAgent.DisplayName = displayName
	command.Home.ID = command.Agent.CanonicalConversationID
	command.Participant.ID = "participant:" + slug + ":home:1"
	command.Participant.ConversationID = command.Home.ID
	command.Participant.SeatID = command.Binding.SeatID
	command.Participant.Profile = command.Binding.FortProfile
	command.Participant.DisplayName = displayName
	command.Link.AgentID = command.Agent.ID
	command.Link.ConversationID = command.Home.ID
	return command
}

func groupCommand(t *testing.T, commands ...ledger.CreateAgentCommand) ledger.CreateGroupCommand {
	t.Helper()
	now := time.Date(2026, 8, 21, 18, 0, 0, 0, time.UTC)
	members := make([]conversation.GroupMember, 0, len(commands))
	bindings := make([]conversation.GroupRecipient, 0, len(commands))
	for position, command := range commands {
		members = append(members, conversation.GroupMember{AgentID: command.Agent.ID, Position: position})
		bindings = append(bindings, conversation.GroupRecipient{
			AgentID: command.Agent.ID, BehaviorRevisionID: command.Behavior.ID,
			BindingRevisionID: command.Binding.ID, ParticipantID: "participant:launch:" + command.Agent.ID,
		})
	}
	group := conversation.GroupConversation{
		ID: "group:launch", AccountID: "account:one", ConversationID: "conversation:group:launch",
		State: conversation.ConversationOpen, CurrentMembershipRevisionID: "membership:launch:1", CreatedAt: now,
	}
	return ledger.CreateGroupCommand{
		IdempotencyKey: "create-group:launch",
		Group:          group,
		Conversation: conversation.Conversation{
			ID: group.ConversationID, Title: "Launch", State: conversation.ConversationOpen,
			CreatedAt: now, UpdatedAt: now,
		},
		Membership: conversation.GroupMembershipRevision{
			ID: group.CurrentMembershipRevisionID, GroupID: group.ID, Revision: 1, Members: members, CreatedAt: now,
		},
		MemberBindings: bindings,
	}
}

func groupSendCommand(t *testing.T, create ledger.CreateGroupCommand) ledger.SendGroupTurnCommand {
	t.Helper()
	now := create.Group.CreatedAt.Add(time.Minute)
	return ledger.SendGroupTurnCommand{
		AccountID: create.Group.AccountID, HumanID: "human:toby", Body: "Compare the launch evidence.",
		Envelope: conversation.GroupTurnEnvelope{
			ID: "group-turn:launch:1", GroupID: create.Group.ID, ConversationID: create.Group.ConversationID,
			ClientTurnID: "client-turn:launch:1", IdempotencyKey: "group-send:launch:1",
			MembershipRevisionID: create.Membership.ID, Selection: conversation.GroupRecipientSelectionEveryone,
			Recipients:           append([]conversation.GroupRecipient(nil), create.MemberBindings...),
			ContextSnapshotID:    "context:launch:1",
			RootDelegationGrant:  conversation.AuthorityGrant{ID: "grant:group:launch", Permissions: []string{"read"}},
			ConcurrencyPolicy:    conversation.GroupConcurrent,
			CancellationPolicyID: "group-cancel:human-or-deadline", CancellationPolicyRevision: "1",
			ApprovalPolicyID: "group-approval:explicit", ApprovalPolicyRevision: "1",
			MaxAgentMessages: conversation.MaxGroupAgentMessages, MaxHandoffDepth: conversation.MaxGroupHandoffDepth,
			CostLimitClass: conversation.LimitUnknown, TokenLimitClass: conversation.LimitUnknown,
			Deadline: now.Add(10 * time.Minute), CreatedAt: now,
		},
		TargetIDs: []string{"target:launch:researcher", "target:launch:builder"},
	}
}

func handoffCommand(t *testing.T, turn ledger.GroupTurnRecord, group ledger.CreateGroupCommand,
	recipient ledger.CreateAgentCommand) ledger.AcceptHandoffCommand {
	t.Helper()
	now := turn.Envelope.CreatedAt.Add(time.Minute)
	messageID := strconv.FormatInt(turn.Message.ID, 10)
	root := conversation.AuthorityGrant{
		ID: "grant:human:handoff", Permissions: []string{"read", "write"},
		ContextRecordIDs: []string{"message:" + messageID},
	}
	policy := conversation.AuthorityGrant{ID: "policy:handoff", Permissions: []string{"read"}}
	recipientPolicy := conversation.AuthorityGrant{ID: "policy:recipient", Permissions: []string{"read", "browser"}}
	effective, err := conversation.ComputeEffectiveAuthority([]string{"read"}, root, policy, recipientPolicy)
	if err != nil {
		t.Fatalf("compute effective authority: %v", err)
	}
	return ledger.AcceptHandoffCommand{
		Handoff: conversation.Handoff{
			ID: "handoff:1", AccountID: group.Group.AccountID, IdempotencyKey: "handoff-command:1",
			State: conversation.HandoffQueued, CreatedByKind: conversation.HandoffActorHuman, CreatedByID: "human:toby",
			SourceMessageID: messageID, RecipientAgentID: recipient.Agent.ID,
			RecipientBehaviorRevisionID: recipient.Behavior.ID, RecipientBindingRevisionID: recipient.Binding.ID,
			SourceConversationID: group.Conversation.ID, OutputConversationID: recipient.Home.ID,
			Context: conversation.ContextManifest{References: []conversation.ContextReference{{
				Kind: conversation.ContextMessage, ID: messageID, AccountID: group.Group.AccountID, Immutable: true,
			}}},
			RequestedResult: "Review the launch evidence.", RootDelegationGrant: root,
			HandoffPolicy: policy, RecipientBindingPolicy: recipientPolicy,
			RequestedAuthority: []string{"read"}, EffectiveAuthority: effective,
			BudgetClass: conversation.LimitUnknown, MaxAgentMessages: conversation.MaxGroupAgentMessages,
			MaxDepth: conversation.MaxGroupHandoffDepth, Depth: 1, Deadline: now.Add(10 * time.Minute), CreatedAt: now,
		},
		TargetID: "target:handoff:1", ParticipantID: recipient.Participant.ID,
		ProjectionConversationIDs: []string{group.Conversation.ID},
	}
}
