package postgres

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/ledger"
)

func TestPostgresGroupLifecycleAndMembershipRevisionIntegration(t *testing.T) {
	databaseURL := integrationDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	accountID := uuid.NewString()
	workerID := "worker:" + uuid.NewString()
	now := time.Now().UTC().Truncate(time.Microsecond)
	admin, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("open admin pool: %v", err)
	}
	defer admin.Close()
	if _, err := admin.Exec(ctx, `insert into fort_private.fort_account(account_id,normalized_email)
values($1,$2)`, accountID, accountID+"@group-lifecycle.fort.test"); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	if _, err := admin.Exec(ctx, `insert into fort_private.worker(
  account_id,worker_id,machine_id,display_name,identity_key_digest,enrollment_token_hash,state,enrolled_at
) values($1,$2,$2,'Group Lifecycle Worker',$3,$4,'enrolled',$5)`, accountID, workerID,
		strings.Repeat("b", 64), strings.Repeat("c", 64), now.Add(-time.Hour)); err != nil {
		t.Fatalf("seed worker: %v", err)
	}
	config, err := SupavisorTransactionConfig(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	config.AfterConnect = func(ctx context.Context, connection *pgx.Conn) error {
		_, err := connection.Exec(ctx, "set role fort_gateway")
		return err
	}
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewWithKeyRing(pool, accountID, collaborationTestKeyRing())
	if err != nil {
		pool.Close()
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	researcher := integrationAgentCommand(accountID, workerID)
	builder := integrationAgentCommand(accountID, workerID)
	reviewer := integrationAgentCommand(accountID, workerID)
	builder.ExecutionSource.GatewayID, builder.ExecutionSource.InstanceID = "gateway:"+uuid.NewString(), "instance:"+uuid.NewString()
	reviewer.ExecutionSource.GatewayID, reviewer.ExecutionSource.InstanceID = "gateway:"+uuid.NewString(), "instance:"+uuid.NewString()
	for name, command := range map[string]ledger.CreateAgentCommand{
		"researcher": researcher, "builder": builder, "reviewer": reviewer,
	} {
		if _, err := store.CreateAgent(ctx, command); err != nil {
			t.Fatalf("CreateAgent %s: %v", name, err)
		}
	}

	create := postgresGroupCommand(researcher, builder)
	create.Group.AccountID = accountID
	create.Group.CreatedAt = now.Add(-10 * time.Minute)
	create.Conversation.CreatedAt, create.Conversation.UpdatedAt = create.Group.CreatedAt, create.Group.CreatedAt
	create.Membership.CreatedAt = create.Group.CreatedAt
	if _, err := store.CreateGroup(ctx, create); err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	renamed, err := store.RenameGroup(ctx, ledger.RenameGroupCommand{
		IdempotencyKey: "rename:" + uuid.NewString(), AccountID: accountID, GroupID: create.Group.ID,
		ExpectedTitle: create.Conversation.Title, Title: "Launch council", ChangedBy: "human:" + accountID,
		ChangedAt: now.Add(-9 * time.Minute),
	})
	if err != nil || renamed.Conversation.Title != "Launch council" {
		t.Fatalf("RenameGroup = %+v, %v", renamed, err)
	}
	archived, err := store.SetGroupState(ctx, ledger.SetGroupStateCommand{
		IdempotencyKey: "archive:" + uuid.NewString(), AccountID: accountID, GroupID: create.Group.ID,
		ExpectedState: conversation.ConversationOpen, State: conversation.ConversationArchived,
		ChangedBy: "human:" + accountID, ChangedAt: now.Add(-8 * time.Minute),
	})
	if err != nil || archived.Group.State != conversation.ConversationArchived {
		t.Fatalf("SetGroupState archive = %+v, %v", archived, err)
	}

	replacement := ledger.ReplaceGroupMembersCommand{
		IdempotencyKey: "members:" + uuid.NewString(), AccountID: accountID, GroupID: create.Group.ID,
		ExpectedMembershipRevisionID: create.Membership.ID,
		Membership: conversation.GroupMembershipRevision{
			ID: "membership:lifecycle:2", GroupID: create.Group.ID, Revision: 2,
			Members:   []conversation.GroupMember{{AgentID: builder.Agent.ID, Position: 0}, {AgentID: reviewer.Agent.ID, Position: 1}},
			CreatedAt: now.Add(-6 * time.Minute),
		},
		MemberBindings: []conversation.GroupRecipient{
			create.MemberBindings[1],
			{AgentID: reviewer.Agent.ID, BehaviorRevisionID: reviewer.Behavior.ID,
				BindingRevisionID: reviewer.Binding.ID, ParticipantID: "participant:lifecycle:reviewer"},
		},
		ChangedBy: "human:" + accountID, ChangedAt: now.Add(-6 * time.Minute),
	}
	if _, err := store.ReplaceGroupMembers(ctx, replacement); !errors.Is(err, ledger.ErrStateConflict) {
		t.Fatalf("archived ReplaceGroupMembers error = %v", err)
	}
	if _, err := store.SetGroupState(ctx, ledger.SetGroupStateCommand{
		IdempotencyKey: "reopen:" + uuid.NewString(), AccountID: accountID, GroupID: create.Group.ID,
		ExpectedState: conversation.ConversationArchived, State: conversation.ConversationOpen,
		ChangedBy: "human:" + accountID, ChangedAt: now.Add(-7 * time.Minute),
	}); err != nil {
		t.Fatalf("SetGroupState reopen: %v", err)
	}
	replaced, err := store.ReplaceGroupMembers(ctx, replacement)
	if err != nil {
		t.Fatalf("ReplaceGroupMembers: %v", err)
	}
	if replaced.Membership.ID != replacement.Membership.ID || replaced.Membership.Revision != 2 ||
		len(replaced.MemberBindings) != 2 || replaced.MemberBindings[0] != create.MemberBindings[1] ||
		replaced.MemberBindings[1] != replacement.MemberBindings[1] {
		t.Fatalf("replacement Group = %+v", replaced)
	}
	stale := replacement
	stale.IdempotencyKey = "members-stale:" + uuid.NewString()
	stale.Membership.ID = "membership:lifecycle:3"
	stale.Membership.Revision = 3
	stale.Membership.CreatedAt = now.Add(-5 * time.Minute)
	stale.ChangedAt = stale.Membership.CreatedAt
	if _, err := store.ReplaceGroupMembers(ctx, stale); !errors.Is(err, ledger.ErrRevisionConflict) {
		t.Fatalf("stale ReplaceGroupMembers error = %v", err)
	}
	var membershipRevisions, memberBindings int
	if err := admin.QueryRow(ctx, `select
  (select count(*) from fort_private.conversation_membership_revision
    where account_id=$1 and conversation_id=$2),
  (select count(*) from fort_private.conversation_member_binding
    where account_id=$1 and conversation_id=$2)`, accountID, create.Group.ConversationID).Scan(
		&membershipRevisions, &memberBindings); err != nil {
		t.Fatalf("read immutable membership history: %v", err)
	}
	if membershipRevisions != 2 || memberBindings != 4 {
		t.Fatalf("membership history = revisions %d bindings %d, want 2/4", membershipRevisions, memberBindings)
	}

	active := postgresGroupCommand(researcher, reviewer)
	active.IdempotencyKey = "create-group:active"
	active.Group.AccountID, active.Group.ID = accountID, "group:active"
	active.Group.ConversationID, active.Group.CurrentMembershipRevisionID = "conversation:group:active", "membership:active:1"
	active.Group.CreatedAt = now.Add(-4 * time.Minute)
	active.Conversation.ID = active.Group.ConversationID
	active.Conversation.CreatedAt, active.Conversation.UpdatedAt = active.Group.CreatedAt, active.Group.CreatedAt
	active.Membership.ID, active.Membership.GroupID, active.Membership.CreatedAt = active.Group.CurrentMembershipRevisionID, active.Group.ID, active.Group.CreatedAt
	for index := range active.MemberBindings {
		active.MemberBindings[index].ParticipantID = "participant:active:" + active.MemberBindings[index].AgentID
	}
	if _, err := store.CreateGroup(ctx, active); err != nil {
		t.Fatalf("CreateGroup active: %v", err)
	}
	send := postgresGroupSendCommand(active)
	send.AccountID = accountID
	send.Envelope.ID, send.Envelope.ClientTurnID = "group-turn:active:1", "client-turn:active:1"
	send.Envelope.IdempotencyKey, send.Envelope.ContextSnapshotID = "group-send:active:1", "context:active:1"
	send.Envelope.CreatedAt, send.Envelope.Deadline = now.Add(-3*time.Minute), now.Add(10*time.Minute)
	send.TargetIDs = []string{"target:active:researcher", "target:active:reviewer"}
	turn, err := store.SendGroupTurn(ctx, send)
	if err != nil {
		t.Fatalf("SendGroupTurn active: %v", err)
	}
	if _, err := store.RenameGroup(ctx, ledger.RenameGroupCommand{
		IdempotencyKey: "rename-active:" + uuid.NewString(), AccountID: accountID, GroupID: active.Group.ID,
		ExpectedTitle: active.Conversation.Title, Title: "Blocked rename", ChangedBy: "human:" + accountID,
		ChangedAt: now.Add(-2 * time.Minute),
	}); !errors.Is(err, ledger.ErrStateConflict) {
		t.Fatalf("active RenameGroup error = %v", err)
	}
	activeReplacement := replacement
	activeReplacement.IdempotencyKey = "members-active:" + uuid.NewString()
	activeReplacement.GroupID, activeReplacement.ExpectedMembershipRevisionID = active.Group.ID, active.Membership.ID
	activeReplacement.Membership.ID, activeReplacement.Membership.GroupID = "membership:active:2", active.Group.ID
	activeReplacement.Membership.Revision, activeReplacement.Membership.CreatedAt = 2, now.Add(-2*time.Minute)
	activeReplacement.ChangedAt = activeReplacement.Membership.CreatedAt
	activeReplacement.MemberBindings[0].ParticipantID = "participant:active:builder"
	activeReplacement.MemberBindings[1].ParticipantID = active.MemberBindings[1].ParticipantID
	if _, err := store.ReplaceGroupMembers(ctx, activeReplacement); !errors.Is(err, ledger.ErrStateConflict) {
		t.Fatalf("active target ReplaceGroupMembers error = %v", err)
	}

	sourceMessageID := strconv.FormatInt(turn.Message.ID, 10)
	handoff, err := store.CreateHumanHandoff(ctx, ledger.CreateHumanHandoffCommand{
		IdempotencyKey: "handoff:" + uuid.NewString(), AccountID: accountID,
		SourceConversationID: active.Conversation.ID, SourceMessageID: sourceMessageID,
		RecipientAgentID: reviewer.Agent.ID, ContextMessageIDs: []string{sourceMessageID},
		RequestedResult: "Review active Group evidence.", ReplyToMessageID: sourceMessageID,
		HardDeadline: send.Envelope.Deadline, HandoffID: "handoff:" + uuid.NewString(), TargetID: "target:" + uuid.NewString(),
		RootDelegationGrantID: "grant:unused:" + uuid.NewString(), CreatedByID: "human:" + accountID,
		CreatedAt: now.Add(-time.Minute),
	})
	if err != nil {
		t.Fatalf("CreateHumanHandoff active: %v", err)
	}
	if _, err := admin.Exec(ctx, `update fort_private.conversation_target
set state='succeeded',updated_at=$3
where account_id=$1 and turn_id=$2 and target_kind='initial'`, accountID, turn.Envelope.ID, now); err != nil {
		t.Fatalf("settle initial targets fixture: %v", err)
	}
	if _, err := admin.Exec(ctx, `update fort_private.conversation_turn
set state='settled',updated_at=$3 where account_id=$1 and turn_id=$2`, accountID, turn.Envelope.ID, now); err != nil {
		t.Fatalf("settle Group Turn fixture: %v", err)
	}
	activeReplacement.IdempotencyKey = "members-active-handoff:" + uuid.NewString()
	if _, err := store.ReplaceGroupMembers(ctx, activeReplacement); !errors.Is(err, ledger.ErrStateConflict) {
		t.Fatalf("active Handoff ReplaceGroupMembers error = %v", err)
	}
	if handoff.Handoff.State != conversation.HandoffQueued {
		t.Fatalf("active Handoff state = %q", handoff.Handoff.State)
	}
}
