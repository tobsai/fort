package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/tobsai/fort/cloud/securebody"
	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/ledger"
)

var _ ledger.CollaborationRepository = (*Store)(nil)

func TestCreateGroupPersistsNormalizedAggregateInOneAccountScopedTransaction(t *testing.T) {
	t.Parallel()

	researcher, builder := postgresCollaborationAgents(t)
	command := postgresGroupCommand(researcher, builder)
	tx := &fakeTransaction{}
	tx.queryRowHook = func(sql string, arguments []any) row {
		if !strings.Contains(sql, "from fort_private.stable_agent") || len(arguments) != 2 || arguments[0] != testAccountID {
			return fakeRow{err: errors.New("unexpected Group member lookup")}
		}
		switch arguments[1] {
		case researcher.Agent.ID:
			return agentRecordRow(t, researcher)
		case builder.Agent.ID:
			return agentRecordRow(t, builder)
		default:
			return fakeRow{err: errors.New("unknown Group member")}
		}
	}
	database := &fakeDatabase{transactions: []transaction{tx}}
	store, err := newStore(database, testAccountID)
	if err != nil {
		t.Fatalf("newStore: %v", err)
	}

	created, err := store.CreateGroup(context.Background(), command)
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	if !reflect.DeepEqual(created, groupRecordFromCommand(command)) {
		t.Fatalf("created Group =\n%+v\nwant\n%+v", created, groupRecordFromCommand(command))
	}
	if database.begins != 1 || tx.commits != 1 || tx.rollbacks != 0 {
		t.Fatalf("transaction lifecycle = begins %d commits %d rollbacks %d", database.begins, tx.commits, tx.rollbacks)
	}
	written := make([]string, 0, len(tx.execs))
	for _, statement := range tx.execs {
		written = append(written, statement.sql)
	}
	for _, statement := range tx.queries {
		written = append(written, statement.sql)
	}
	for _, table := range []string{
		"idempotency_record", "conversation", "group_conversation",
		"conversation_membership_revision", "conversation_member_revision", "conversation_participant",
		"conversation_member_binding",
	} {
		if !containsSQL(written, "fort_private."+table) {
			t.Fatalf("CreateGroup did not persist %s; statements = %#v", table, written)
		}
	}
	for _, statement := range append(tx.execs, tx.queries...) {
		if strings.Contains(statement.sql, "fort_private.") && (len(statement.args) == 0 || statement.args[0] != testAccountID) {
			t.Fatalf("statement lacks explicit account scope: %q %#v", statement.sql, statement.args)
		}
	}
}

func TestCollaborationOperationsRejectForeignAccountBeforeOpeningPostgres(t *testing.T) {
	t.Parallel()

	database := &fakeDatabase{}
	store, err := newStore(database, testAccountID)
	if err != nil {
		t.Fatalf("newStore: %v", err)
	}
	foreign := "21d9b714-9407-4302-853d-358fcb9abb91"
	if _, err := store.GetGroup(context.Background(), foreign, "group:launch"); err == nil {
		t.Fatal("GetGroup accepted a foreign account")
	}
	if _, err := store.ListHandoffs(context.Background(), foreign); err == nil {
		t.Fatal("ListHandoffs accepted a foreign account")
	}
	if database.begins != 0 {
		t.Fatalf("database began %d transactions for foreign accounts", database.begins)
	}
}

func TestPostgresGroupRenameUsesOptimisticTitleAndAppendsAuditEvent(t *testing.T) {
	t.Parallel()
	researcher, builder := postgresCollaborationAgents(t)
	create := postgresGroupCommand(researcher, builder)
	command := ledger.RenameGroupCommand{
		IdempotencyKey: "group:rename:launch", AccountID: testAccountID, GroupID: create.Group.ID,
		ExpectedTitle: create.Conversation.Title, Title: "Launch council", ChangedBy: "human:toby",
		ChangedAt: create.Group.CreatedAt.Add(time.Minute),
	}
	want := create
	want.Conversation.Title = command.Title
	want.Conversation.UpdatedAt = command.ChangedAt
	tx := &fakeTransaction{queryRows: groupMemberRows(want)}
	tx.queryRowHook = func(sql string, _ []any) row {
		switch {
		case strings.Contains(sql, "for update of conversation"):
			return fakeRow{values: []any{create.Group.ConversationID, create.Conversation.Title}}
		case strings.Contains(sql, "select exists"):
			return fakeRow{values: []any{false}}
		case strings.Contains(sql, "from fort_private.group_conversation"):
			return groupRecordRow(want)
		default:
			return fakeRow{err: fmt.Errorf("unexpected Group rename query: %s", sql)}
		}
	}
	store, err := newStore(&fakeDatabase{transactions: []transaction{tx}}, testAccountID)
	if err != nil {
		t.Fatal(err)
	}
	renamed, err := store.RenameGroup(context.Background(), command)
	if err != nil {
		t.Fatalf("RenameGroup: %v", err)
	}
	if renamed.Conversation.Title != command.Title || tx.commits != 1 || tx.rollbacks != 0 ||
		!containsSQL(statementsSQL(tx.execs), "fort_private.ledger_event") {
		t.Fatalf("renamed Group/transaction = %+v / commits %d rollbacks %d", renamed, tx.commits, tx.rollbacks)
	}
}

func TestPostgresGroupRenameRejectsActiveGroupWork(t *testing.T) {
	t.Parallel()
	researcher, builder := postgresCollaborationAgents(t)
	create := postgresGroupCommand(researcher, builder)
	command := ledger.RenameGroupCommand{
		IdempotencyKey: "group:rename:active", AccountID: testAccountID, GroupID: create.Group.ID,
		ExpectedTitle: create.Conversation.Title, Title: "Blocked rename", ChangedBy: "human:toby",
		ChangedAt: create.Group.CreatedAt.Add(time.Minute),
	}
	tx := &fakeTransaction{}
	tx.queryRowHook = func(sql string, _ []any) row {
		switch {
		case strings.Contains(sql, "for update of conversation"):
			return fakeRow{values: []any{create.Group.ConversationID, create.Conversation.Title}}
		case strings.Contains(sql, "select exists"):
			return fakeRow{values: []any{true}}
		default:
			return fakeRow{err: fmt.Errorf("unexpected Group rename query: %s", sql)}
		}
	}
	store, err := newStore(&fakeDatabase{transactions: []transaction{tx}}, testAccountID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.RenameGroup(context.Background(), command); !errors.Is(err, ledger.ErrStateConflict) {
		t.Fatalf("RenameGroup error = %v, want %v", err, ledger.ErrStateConflict)
	}
	if tx.commits != 0 || tx.rollbacks != 1 || containsSQL(statementsSQL(tx.execs), "set title") {
		t.Fatalf("active Group rename transaction = commits %d rollbacks %d statements %#v", tx.commits, tx.rollbacks, tx.execs)
	}
}

func TestPostgresGroupStateChangeRejectsActiveGroupWork(t *testing.T) {
	t.Parallel()
	researcher, builder := postgresCollaborationAgents(t)
	create := postgresGroupCommand(researcher, builder)
	command := ledger.SetGroupStateCommand{
		IdempotencyKey: "group:archive:launch", AccountID: testAccountID, GroupID: create.Group.ID,
		ExpectedState: conversation.ConversationOpen, State: conversation.ConversationArchived,
		ChangedBy: "human:toby", ChangedAt: create.Group.CreatedAt.Add(time.Minute),
	}
	tx := &fakeTransaction{}
	tx.queryRowHook = func(sql string, _ []any) row {
		switch {
		case strings.Contains(sql, "for update of conversation"):
			return fakeRow{values: []any{create.Group.ConversationID, string(conversation.ConversationOpen)}}
		case strings.Contains(sql, "select exists"):
			return fakeRow{values: []any{true}}
		default:
			return fakeRow{err: fmt.Errorf("unexpected Group state query: %s", sql)}
		}
	}
	store, err := newStore(&fakeDatabase{transactions: []transaction{tx}}, testAccountID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetGroupState(context.Background(), command); !errors.Is(err, ledger.ErrStateConflict) {
		t.Fatalf("SetGroupState error = %v, want %v", err, ledger.ErrStateConflict)
	}
	if tx.commits != 0 || tx.rollbacks != 1 || containsSQL(statementsSQL(tx.execs), "set state") {
		t.Fatalf("active Group state transaction = commits %d rollbacks %d statements %#v", tx.commits, tx.rollbacks, tx.execs)
	}
}

func statementsSQL(statements []recordedStatement) []string {
	result := make([]string, 0, len(statements))
	for _, statement := range statements {
		result = append(result, statement.sql)
	}
	return result
}

func TestSendGroupTurnEncryptsOneMessageAndCreatesOnlyWaveZeroTargetsAtomically(t *testing.T) {
	t.Parallel()

	researcher, builder := postgresCollaborationAgents(t)
	create := postgresGroupCommand(researcher, builder)
	command := postgresGroupSendCommand(create)
	tx := &fakeTransaction{queryRows: groupMemberRows(create)}
	tx.queryRowHook = func(sql string, arguments []any) row {
		switch {
		case strings.Contains(sql, "from fort_private.group_conversation"):
			return groupRecordRow(create)
		case strings.Contains(sql, "from fort_private.stable_agent as agent"):
			if arguments[1] == researcher.Agent.ID {
				return currentGroupRecipientRow(researcher)
			}
			return currentGroupRecipientRow(builder)
		case strings.Contains(sql, "select participant_id from fort_private.conversation_participant"):
			if arguments[2] == researcher.Agent.ID {
				return fakeRow{values: []any{command.Envelope.Recipients[0].ParticipantID}}
			}
			return fakeRow{values: []any{command.Envelope.Recipients[1].ParticipantID}}
		case strings.Contains(sql, "returning message_id"):
			return fakeRow{values: []any{int64(41)}}
		case strings.Contains(sql, "array_agg(message_id order by message_id)"):
			return fakeRow{values: []any{[]int64{31, 41}}}
		default:
			return fakeRow{err: errors.New("unexpected Send Group Turn query")}
		}
	}
	database := &fakeDatabase{transactions: []transaction{tx}}
	store, err := newStoreWithKeyRing(database, testAccountID, collaborationTestKeyRing())
	if err != nil {
		t.Fatalf("newStoreWithKeyRing: %v", err)
	}

	created, err := store.SendGroupTurn(context.Background(), command)
	if err != nil {
		t.Fatalf("SendGroupTurn: %v", err)
	}
	if created.Message.ID != 41 || created.Message.Body != command.Body ||
		!reflect.DeepEqual(created.Recipients, command.Envelope.Recipients) || len(created.InitialTargets) != 2 {
		t.Fatalf("created Group Turn = %+v", created)
	}
	if !reflect.DeepEqual(created.Envelope.RootDelegationGrant.ContextRecordIDs, []string{"message:31", "message:41"}) {
		t.Fatalf("Group Turn root grant did not authorize frozen context: %+v", created.Envelope.RootDelegationGrant)
	}
	for index, target := range created.InitialTargets {
		if target.ID != command.TargetIDs[index] || target.Wave != 0 ||
			target.AgentID != command.Envelope.Recipients[index].AgentID || target.State != conversation.TargetQueued {
			t.Fatalf("initial target %d = %+v", index, target)
		}
	}
	for _, statement := range tx.execs {
		for _, argument := range statement.args {
			switch value := argument.(type) {
			case string:
				if value == command.Body {
					t.Fatalf("plaintext Group body was sent to Postgres in %q", statement.sql)
				}
			case []byte:
				if bytes.Equal(value, []byte(command.Body)) {
					t.Fatalf("plaintext Group body was sent to Postgres in %q", statement.sql)
				}
			}
		}
	}
	written := make([]string, 0, len(tx.execs))
	for _, statement := range tx.execs {
		written = append(written, statement.sql)
	}
	for _, statement := range tx.queries {
		written = append(written, statement.sql)
	}
	for _, table := range []string{"context_manifest", "context_manifest_message", "delegation_grant", "conversation_message", "conversation_turn", "conversation_target", "conversation_target_binding"} {
		if !containsSQL(written, "fort_private."+table) {
			t.Fatalf("SendGroupTurn did not persist %s; statements = %#v", table, written)
		}
	}
	manifestMessages := 0
	wantManifestMessages := []int64{31, 41}
	for _, statement := range tx.execs {
		if strings.Contains(statement.sql, "fort_private.context_manifest_message") {
			if manifestMessages >= len(wantManifestMessages) || len(statement.args) != 4 || statement.args[0] != testAccountID ||
				statement.args[1] != command.Envelope.ContextSnapshotID ||
				statement.args[2] != manifestMessages || statement.args[3] != wantManifestMessages[manifestMessages] {
				t.Fatalf("frozen Group context row %d = %#v", manifestMessages, statement.args)
			}
			manifestMessages++
		}
	}
	if manifestMessages != 2 {
		t.Fatalf("frozen Group context rows = %d, want 2", manifestMessages)
	}
	if tx.commits != 1 || tx.rollbacks != 0 {
		t.Fatalf("SendGroupTurn lifecycle = commits %d rollbacks %d", tx.commits, tx.rollbacks)
	}
}

func TestSendGroupTurnPinsCurrentReboundAgentWithoutChangingMembership(t *testing.T) {
	t.Parallel()

	researcher, builder := postgresCollaborationAgents(t)
	create := postgresGroupCommand(researcher, builder)
	command := postgresGroupSendCommand(create)
	rebound := researcher
	rebound.Behavior.ID += ":2"
	rebound.Behavior.Revision++
	rebound.Binding.ID += ":2"
	rebound.Binding.Revision++
	rebound.Binding.BehaviorRevisionID = rebound.Behavior.ID
	rebound.Binding.SupersedesRevisionID = researcher.Binding.ID
	rebound.Agent.CurrentBehaviorRevisionID = rebound.Behavior.ID
	rebound.Agent.CurrentBindingRevisionID = rebound.Binding.ID
	command.Envelope.Recipients[0] = conversation.GroupRecipient{
		AgentID: researcher.Agent.ID, BehaviorRevisionID: rebound.Behavior.ID,
		BindingRevisionID: rebound.Binding.ID,
		ParticipantID: postgresGroupParticipantID(testAccountID, create.Group.ID,
			researcher.Agent.ID, rebound.Binding.ID),
	}

	tx := &fakeTransaction{queryRows: groupMemberRows(create)}
	tx.queryRowHook = func(sql string, arguments []any) row {
		switch {
		case strings.Contains(sql, "from fort_private.group_conversation"):
			return groupRecordRow(create)
		case strings.Contains(sql, "from fort_private.stable_agent as agent"):
			if arguments[1] == researcher.Agent.ID {
				return currentGroupRecipientRow(rebound)
			}
			return currentGroupRecipientRow(builder)
		case strings.Contains(sql, "select participant_id from fort_private.conversation_participant"):
			if arguments[2] == researcher.Agent.ID {
				return fakeRow{err: pgx.ErrNoRows}
			}
			return fakeRow{values: []any{command.Envelope.Recipients[1].ParticipantID}}
		case strings.Contains(sql, "returning message_id"):
			return fakeRow{values: []any{int64(41)}}
		case strings.Contains(sql, "array_agg(message_id order by message_id)"):
			return fakeRow{values: []any{[]int64{41}}}
		default:
			return fakeRow{err: errors.New("unexpected rebound Group Turn query")}
		}
	}
	store, err := newStoreWithKeyRing(&fakeDatabase{transactions: []transaction{tx}}, testAccountID, collaborationTestKeyRing())
	if err != nil {
		t.Fatal(err)
	}

	if _, err := store.SendGroupTurn(context.Background(), command); err != nil {
		t.Fatalf("SendGroupTurn after Rebind: %v", err)
	}
	insertedParticipant := false
	pinnedTarget := false
	for _, statement := range tx.execs {
		if strings.Contains(statement.sql, "insert into fort_private.conversation_participant") &&
			containsArgument(statement.args, rebound.Behavior.ID) && containsArgument(statement.args, rebound.Binding.ID) {
			insertedParticipant = true
		}
		if strings.Contains(statement.sql, "insert into fort_private.conversation_target_binding") &&
			containsArgument(statement.args, rebound.Behavior.ID) && containsArgument(statement.args, rebound.Binding.ID) {
			pinnedTarget = true
		}
	}
	if !insertedParticipant || !pinnedTarget {
		t.Fatalf("rebound participant/target evidence = %t/%t", insertedParticipant, pinnedTarget)
	}
}

func TestListGroupMessagesDecryptsOrderedHumanAndAttributedAgentTranscript(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 20, 0, 0, 0, time.UTC)
	ring := collaborationTestKeyRing()
	cipher := secureCollaborationBodyCipher{ring: ring}
	prompt, err := cipher.seal(securebody.Scope{AccountID: testAccountID,
		RecordType: "group_turn_prompt", RecordID: "turn:1"}, "Compare the evidence.")
	if err != nil {
		t.Fatal(err)
	}
	answer, err := cipher.seal(securebody.Scope{AccountID: testAccountID,
		RecordType: "conversation_message", RecordID: "target:research"}, "The evidence agrees.")
	if err != nil {
		t.Fatal(err)
	}
	base := &fakeTransaction{queryRowHook: func(query string, arguments []any) row {
		if !strings.Contains(query, "from fort_private.group_conversation") || len(arguments) != 2 ||
			arguments[0] != testAccountID || arguments[1] != "group:launch" {
			return fakeRow{err: errors.New("Group transcript parent is not account scoped")}
		}
		return fakeRow{values: []any{"conversation:launch"}}
	}}
	tx := &agentChatRowsTransaction{fakeTransaction: base, rowSets: []rows{&fakeRows{values: [][]any{
		{int64(41), "conversation:launch", "turn:1", "", "", "", "human", string(conversation.AuthorHuman),
			"human:toby", "", prompt.Ciphertext, prompt.KeyID, prompt.Nonce, prompt.Digest, prompt.PlaintextBytes, now},
		{int64(73), "conversation:launch", "turn:1", "target:research", "", "", "agent", string(conversation.AuthorAssistant),
			"agent:research", "agent:research", answer.Ciphertext, answer.KeyID, answer.Nonce, answer.Digest, answer.PlaintextBytes, now.Add(time.Second)},
	}}}}
	store, err := newStoreWithKeyRing(&fakeDatabase{transactions: []transaction{tx}}, testAccountID, ring)
	if err != nil {
		t.Fatal(err)
	}

	messages, err := store.ListGroupMessages(context.Background(), testAccountID, "group:launch")
	if err != nil {
		t.Fatalf("ListGroupMessages: %v", err)
	}
	if len(messages) != 2 || messages[0].Body != "Compare the evidence." ||
		messages[1].Body != "The evidence agrees." || messages[1].AuthorAgentID != "agent:research" ||
		messages[1].TargetID != "target:research" {
		t.Fatalf("Group transcript = %+v", messages)
	}
}

func TestGetPostgresGroupTurnReconstructsPersistedLimitsAndEncryptedPrompt(t *testing.T) {
	t.Parallel()

	researcher, builder := postgresCollaborationAgents(t)
	create := postgresGroupCommand(researcher, builder)
	command := postgresGroupSendCommand(create)
	cipher := secureCollaborationBodyCipher{ring: collaborationTestKeyRing()}
	encrypted, err := cipher.seal(securebody.Scope{
		AccountID: testAccountID, RecordType: "group_turn_prompt", RecordID: command.Envelope.ID,
	}, command.Body)
	if err != nil {
		t.Fatalf("encrypt prompt fixture: %v", err)
	}
	cancellation, err := json.Marshal(collaborationPolicySnapshot{
		ID: command.Envelope.CancellationPolicyID, Revision: command.Envelope.CancellationPolicyRevision,
	})
	if err != nil {
		t.Fatal(err)
	}
	approval, err := json.Marshal(collaborationPolicySnapshot{
		ID: command.Envelope.ApprovalPolicyID, Revision: command.Envelope.ApprovalPolicyRevision,
		FortGroup: &collaborationGroupTurnMeta{
			GroupID: command.Envelope.GroupID, Selection: command.Envelope.Selection,
			Recipients: command.Envelope.Recipients, TargetIDs: command.TargetIDs,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	grant, err := json.Marshal(command.Envelope.RootDelegationGrant)
	if err != nil {
		t.Fatal(err)
	}
	tx := &fakeTransaction{queryRows: &fakeRows{values: [][]any{
		{command.TargetIDs[0], command.Envelope.Recipients[0].AgentID,
			command.Envelope.Recipients[0].BehaviorRevisionID, command.Envelope.Recipients[0].BindingRevisionID,
			command.Envelope.Recipients[0].ParticipantID, "queued", command.Envelope.CreatedAt},
		{command.TargetIDs[1], command.Envelope.Recipients[1].AgentID,
			command.Envelope.Recipients[1].BehaviorRevisionID, command.Envelope.Recipients[1].BindingRevisionID,
			command.Envelope.Recipients[1].ParticipantID, "queued", command.Envelope.CreatedAt},
	}}}
	tx.queryRowHook = func(sql string, _ []any) row {
		switch {
		case strings.Contains(sql, "from fort_private.conversation_turn as turn"):
			manifestDigest, err := evidenceDigest(struct {
				Version          int     `json:"version"`
				ConversationID   string  `json:"conversation_id"`
				ThroughMessageID int64   `json:"through_message_id"`
				MessageIDs       []int64 `json:"message_ids"`
			}{1, command.Envelope.ConversationID, 41, []int64{41}})
			if err != nil {
				t.Fatal(err)
			}
			return fakeRow{values: []any{
				command.Envelope.ID, command.Envelope.ConversationID, command.Envelope.ClientTurnID,
				command.Envelope.IdempotencyKey, command.Envelope.MembershipRevisionID,
				command.Envelope.ContextSnapshotID, int64(41), manifestDigest,
				"parallel", string(cancellation), string(approval),
				command.Envelope.MaxAgentMessages, command.Envelope.MaxHandoffDepth,
				command.Envelope.CostLimitClass, command.Envelope.TokenLimitClass,
				command.Envelope.Deadline, command.Envelope.CreatedAt, string(grant),
				int64(41), command.HumanID, encrypted.Ciphertext, encrypted.KeyID, encrypted.Nonce,
				encrypted.Digest, encrypted.PlaintextBytes, command.Envelope.CreatedAt,
			}}
		case strings.Contains(sql, "fort_private.context_manifest_message"):
			return fakeRow{values: []any{[]int64{41}}}
		case strings.Contains(sql, "as frozen_messages"):
			return fakeRow{values: []any{[]int64{41}}}
		default:
			return fakeRow{err: fmt.Errorf("unexpected Group Turn query: %s", sql)}
		}
	}

	record, err := getPostgresGroupTurn(context.Background(), tx, cipher, testAccountID, command.Envelope.ID)
	if err != nil {
		t.Fatalf("getPostgresGroupTurn: %v", err)
	}
	if record.Envelope.CostLimitClass != command.Envelope.CostLimitClass ||
		record.Envelope.TokenLimitClass != command.Envelope.TokenLimitClass || record.Message.Body != command.Body {
		t.Fatalf("reconstructed Group Turn = %+v", record)
	}
}

func TestAcceptHandoffPersistsExactEncryptedCommandTargetAndReferenceOnlyProjection(t *testing.T) {
	t.Parallel()

	researcher, builder := postgresCollaborationAgents(t)
	create := postgresGroupCommand(researcher, builder)
	turn := groupTurnRecord(postgresGroupSendCommand(create), 41,
		mustInitialTargets(t, postgresGroupSendCommand(create), create))
	command := postgresHandoffCommand(t, turn, create, builder)
	tx := &fakeTransaction{}
	tx.queryRowHook = func(sql string, arguments []any) row {
		switch {
		case strings.Contains(sql, "select conversation_id, coalesce(turn_id") && strings.Contains(sql, "conversation_message"):
			return fakeRow{values: []any{command.Handoff.SourceConversationID, turn.Envelope.ID}}
		case strings.Contains(sql, "from fort_private.conversation_participant"):
			return fakeRow{values: []any{"membership:builder:home"}}
		case strings.Contains(sql, "select conversation_id from fort_private.conversation"):
			return fakeRow{values: []any{arguments[1].(string)}}
		case strings.Contains(sql, "select message_id from fort_private.conversation_message"):
			return fakeRow{values: []any{int64(41)}}
		default:
			return fakeRow{err: fmt.Errorf("unexpected Handoff acceptance query: %s %#v", sql, arguments)}
		}
	}
	database := &fakeDatabase{transactions: []transaction{tx}}
	store, err := newStoreWithKeyRing(database, testAccountID, collaborationTestKeyRing())
	if err != nil {
		t.Fatalf("newStoreWithKeyRing: %v", err)
	}

	accepted, err := store.AcceptHandoff(context.Background(), command)
	if err != nil {
		t.Fatalf("AcceptHandoff: %v", err)
	}
	if accepted.Handoff.ID != command.Handoff.ID || accepted.Target.ID != command.TargetID ||
		accepted.Target.State != conversation.TargetQueued || accepted.Result != nil || accepted.Attempt != nil {
		t.Fatalf("accepted Handoff = %+v", accepted)
	}
	if len(accepted.Projections) != 1 || accepted.Projections[0].ConversationID != create.Conversation.ID ||
		accepted.Projections[0].AuthoritativeMessageID != "" || accepted.Projections[0].State != conversation.HandoffQueued {
		t.Fatalf("accepted projections = %+v", accepted.Projections)
	}
	for _, statement := range append(tx.execs, tx.queries...) {
		for _, argument := range statement.args {
			if value, ok := argument.(string); ok && value == command.Handoff.RequestedResult {
				t.Fatalf("plaintext requested result was sent to Postgres in %q", statement.sql)
			}
		}
	}
	written := make([]string, 0, len(tx.execs))
	for _, statement := range tx.execs {
		written = append(written, statement.sql)
	}
	for _, table := range []string{"context_manifest", "context_manifest_message", "delegation_grant", "conversation_turn", "conversation_target", "conversation_target_binding", "handoff", "handoff_projection"} {
		if !containsSQL(written, "fort_private."+table) {
			t.Fatalf("AcceptHandoff did not persist %s; statements = %#v", table, written)
		}
	}
	if tx.commits != 1 || tx.rollbacks != 0 {
		t.Fatalf("AcceptHandoff lifecycle = commits %d rollbacks %d", tx.commits, tx.rollbacks)
	}
}

func TestHandoffStartAndCompletionRejectNonDecimalFenceBeforePostgres(t *testing.T) {
	t.Parallel()

	database := &fakeDatabase{}
	store, err := newStoreWithKeyRing(database, testAccountID, collaborationTestKeyRing())
	if err != nil {
		t.Fatalf("newStoreWithKeyRing: %v", err)
	}
	now := time.Date(2026, 8, 21, 22, 0, 0, 0, time.UTC)
	start := ledger.StartHandoffCommand{
		AccountID: testAccountID, HandoffID: "handoff:1", IdempotencyKey: "start:handoff:1",
		AttemptID: "attempt:handoff:1", LeaseID: "lease:handoff:1", MachineID: "machine:studio",
		FenceToken: "opaque-fence", StartedAt: now, LeaseExpiresAt: now.Add(time.Minute),
	}
	if _, err := store.StartHandoff(context.Background(), start); err == nil || !strings.Contains(err.Error(), "decimal") {
		t.Fatalf("StartHandoff error = %v, want canonical decimal fence rejection", err)
	}
	complete := ledger.CompleteHandoffCommand{
		AccountID: testAccountID, HandoffID: "handoff:1", IdempotencyKey: "complete:handoff:1",
		AuthorAgentID: "agent:builder", AttemptID: start.AttemptID, LeaseID: start.LeaseID,
		FenceToken: start.FenceToken, TerminalReceiptID: "receipt:handoff:1", Body: "done", CreatedAt: now,
	}
	if _, err := store.CompleteHandoff(context.Background(), complete); err == nil || !strings.Contains(err.Error(), "decimal") {
		t.Fatalf("CompleteHandoff error = %v, want canonical decimal fence rejection", err)
	}
	if database.begins != 0 {
		t.Fatalf("invalid fences opened %d Postgres transactions", database.begins)
	}
}

func TestAcceptHandoffRejectsCreationActorAndEmitterInconsistencyBeforePostgres(t *testing.T) {
	t.Parallel()

	researcher, builder := postgresCollaborationAgents(t)
	create := postgresGroupCommand(researcher, builder)
	send := postgresGroupSendCommand(create)
	turn := groupTurnRecord(send, 41, mustInitialTargets(t, send, create))
	base := postgresHandoffCommand(t, turn, create, builder)

	t.Run("agent actor must be the source Agent", func(t *testing.T) {
		command := base
		command.Handoff.CreatedByKind = conversation.HandoffActorAgent
		command.Handoff.CreatedByID = "agent:not-the-source"
		command.Handoff.SourceAgentID = researcher.Agent.ID
		command.Handoff.SourceBehaviorRevisionID = researcher.Behavior.ID
		command.Handoff.SourceBindingRevisionID = researcher.Binding.ID
		command.Handoff.StructuredEmitterID = "emitter:handoff:1"
		parent := conversation.AuthorityGrant{ID: "authority:parent", Permissions: []string{"read"}}
		emitter := conversation.AuthorityGrant{ID: "authority:emitter", Permissions: []string{"read"}}
		command.Handoff.ParentStageAuthority = &parent
		command.Handoff.EmitterRequest = &emitter
		command.Handoff.AncestorAgentIDs = []string{researcher.Agent.ID}
		effective, err := conversation.ComputeEffectiveAuthority(command.Handoff.RequestedAuthority,
			command.Handoff.RootDelegationGrant, parent, command.Handoff.HandoffPolicy,
			command.Handoff.RecipientBindingPolicy, emitter)
		if err != nil {
			t.Fatal(err)
		}
		command.Handoff.EffectiveAuthority = effective

		database := &fakeDatabase{}
		store, err := newStoreWithKeyRing(database, testAccountID, collaborationTestKeyRing())
		if err != nil {
			t.Fatal(err)
		}
		if _, err := store.AcceptHandoff(context.Background(), command); err == nil ||
			!strings.Contains(err.Error(), "creation actor") {
			t.Fatalf("AcceptHandoff error = %v, want creation actor mismatch", err)
		}
		if database.begins != 0 {
			t.Fatalf("actor mismatch opened %d Postgres transactions", database.begins)
		}
	})

	t.Run("human actor cannot claim an emitter receipt", func(t *testing.T) {
		command := base
		command.Handoff.StructuredEmitterID = "emitter:human:1"
		database := &fakeDatabase{}
		store, err := newStoreWithKeyRing(database, testAccountID, collaborationTestKeyRing())
		if err != nil {
			t.Fatal(err)
		}
		if _, err := store.AcceptHandoff(context.Background(), command); err == nil ||
			!strings.Contains(err.Error(), "emitter") {
			t.Fatalf("AcceptHandoff error = %v, want human emitter rejection", err)
		}
		if database.begins != 0 {
			t.Fatalf("human emitter mismatch opened %d Postgres transactions", database.begins)
		}
	})
}

func TestValidatePostgresEmitterReceiptRejectsAReceiptForAnotherStructuredCommand(t *testing.T) {
	t.Parallel()

	handoff := conversation.Handoff{
		AccountID: testAccountID, CreatedByKind: conversation.HandoffActorAgent,
		StructuredEmitterID: "emitter:handoff:1", SourceAgentID: "agent:researcher",
		SourceBehaviorRevisionID: "behavior:researcher:1",
		SourceBindingRevisionID:  "binding:researcher:1",
	}
	queriedDigest := false
	tx := &fakeTransaction{queryRowHook: func(sql string, _ []any) row {
		queriedDigest = strings.Contains(sql, "structured_command_digest") &&
			strings.Contains(sql, "conversation_message")
		if queriedDigest {
			return fakeRow{values: []any{handoff.StructuredEmitterID, "attempt:source:1", strings.Repeat("b", 64)}}
		}
		return fakeRow{values: []any{handoff.StructuredEmitterID}}
	}}
	_, err := validatePostgresEmitterReceipt(context.Background(), tx, handoff, strings.Repeat("a", 64), 41)
	if err == nil {
		t.Fatal("validatePostgresEmitterReceipt accepted a receipt for another structured command")
	}
	if !queriedDigest {
		t.Fatal("emitter receipt validation did not bind the command digest and source message")
	}
}

func TestValidatePostgresEmitterReceiptReturnsExactSourceAttemptForPersistence(t *testing.T) {
	t.Parallel()

	handoff := conversation.Handoff{
		AccountID: testAccountID, CreatedByKind: conversation.HandoffActorAgent,
		StructuredEmitterID: "emitter:handoff:1", SourceAgentID: "agent:researcher",
		SourceBehaviorRevisionID: "behavior:researcher:1",
		SourceBindingRevisionID:  "binding:researcher:1",
	}
	digest := strings.Repeat("a", 64)
	tx := &fakeTransaction{queryRowHook: func(sql string, _ []any) row {
		if !strings.Contains(sql, "source_execution_attempt_id") {
			return fakeRow{err: errors.New("emitter receipt lookup omitted source execution attempt")}
		}
		return fakeRow{values: []any{handoff.StructuredEmitterID, "attempt:source:1", digest}}
	}}
	attemptID, err := validatePostgresEmitterReceipt(context.Background(), tx, handoff, digest, 41)
	if err != nil {
		t.Fatalf("validatePostgresEmitterReceipt: %v", err)
	}
	if attemptID != "attempt:source:1" {
		t.Fatalf("source execution attempt = %q", attemptID)
	}
}

func TestPersistPostgresHandoffApprovalStoresAuthorityOnlyInsideEncryptedReceipt(t *testing.T) {
	t.Parallel()

	receipt := conversation.AuthorityGrant{ID: "approval:handoff:1", Permissions: []string{"read"}}
	handoff := conversation.Handoff{
		ID: "handoff:1", AccountID: testAccountID, CreatedByID: "human:toby",
		ApprovalReceipt: &receipt, CreatedAt: time.Date(2026, 8, 21, 22, 30, 0, 0, time.UTC),
	}
	tx := &fakeTransaction{}
	cipher := secureCollaborationBodyCipher{ring: collaborationTestKeyRing()}
	if err := persistPostgresHandoffApproval(context.Background(), tx, cipher, handoff); err != nil {
		t.Fatalf("persistPostgresHandoffApproval: %v", err)
	}
	if len(tx.execs) != 1 {
		t.Fatalf("approval writes = %d, want 1", len(tx.execs))
	}
	statement := tx.execs[0]
	if strings.Contains(statement.sql, "authority_delta") {
		t.Fatalf("approval SQL retained plaintext authority duplicate: %s", statement.sql)
	}
	for _, argument := range statement.args {
		if value, ok := argument.(string); ok && strings.Contains(value, `"permissions"`) {
			t.Fatalf("approval authority was sent to Postgres in plaintext: %q", value)
		}
	}
}

func postgresCollaborationAgents(t *testing.T) (ledger.CreateAgentCommand, ledger.CreateAgentCommand) {
	t.Helper()
	researcher := postgresAgentCommand()
	payload, err := json.Marshal(researcher)
	if err != nil {
		t.Fatal(err)
	}
	var builder ledger.CreateAgentCommand
	if err := json.Unmarshal([]byte(strings.ReplaceAll(string(payload), "researcher", "builder")), &builder); err != nil {
		t.Fatal(err)
	}
	return researcher, builder
}

func postgresGroupCommand(commands ...ledger.CreateAgentCommand) ledger.CreateGroupCommand {
	now := time.Date(2026, 8, 21, 21, 0, 0, 0, time.UTC)
	group := conversation.GroupConversation{
		ID: "group:launch", AccountID: testAccountID, ConversationID: "conversation:group:launch",
		State: conversation.ConversationOpen, CurrentMembershipRevisionID: "membership:launch:1", CreatedAt: now,
	}
	members := make([]conversation.GroupMember, 0, len(commands))
	bindings := make([]conversation.GroupRecipient, 0, len(commands))
	for position, command := range commands {
		members = append(members, conversation.GroupMember{AgentID: command.Agent.ID, Position: position})
		bindings = append(bindings, conversation.GroupRecipient{
			AgentID: command.Agent.ID, BehaviorRevisionID: command.Behavior.ID,
			BindingRevisionID: command.Binding.ID,
			ParticipantID: postgresGroupParticipantID(testAccountID, group.ID,
				command.Agent.ID, command.Binding.ID),
		})
	}
	return ledger.CreateGroupCommand{
		IdempotencyKey: "create-group:launch",
		Group:          group,
		Conversation: conversation.Conversation{
			ID: group.ConversationID, Title: "Launch", State: conversation.ConversationOpen,
			CreatedAt: now, UpdatedAt: now,
		},
		Membership: conversation.GroupMembershipRevision{
			ID: group.CurrentMembershipRevisionID, GroupID: group.ID, Revision: 1,
			Members: members, CreatedAt: now,
		},
		MemberBindings: bindings,
	}
}

func groupRecordFromCommand(command ledger.CreateGroupCommand) ledger.GroupRecord {
	command.Membership.Members = append([]conversation.GroupMember{}, command.Membership.Members...)
	command.MemberBindings = append([]conversation.GroupRecipient{}, command.MemberBindings...)
	return ledger.GroupRecord{
		Group: command.Group, Conversation: command.Conversation,
		Membership: command.Membership, MemberBindings: command.MemberBindings,
	}
}

func postgresGroupSendCommand(create ledger.CreateGroupCommand) ledger.SendGroupTurnCommand {
	now := create.Group.CreatedAt.Add(time.Minute)
	return ledger.SendGroupTurnCommand{
		AccountID: create.Group.AccountID, HumanID: "human:toby", Body: "Compare the launch evidence.",
		Envelope: conversation.GroupTurnEnvelope{
			ID: "group-turn:launch:1", GroupID: create.Group.ID, ConversationID: create.Group.ConversationID,
			ClientTurnID: "client-turn:launch:1", IdempotencyKey: "group-send:launch:1",
			MembershipRevisionID: create.Membership.ID, Selection: conversation.GroupRecipientSelectionEveryone,
			Recipients:           append([]conversation.GroupRecipient{}, create.MemberBindings...),
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

func groupRecordRow(command ledger.CreateGroupCommand) row {
	return fakeRow{values: []any{
		command.Group.ID, command.Group.AccountID, command.Group.ConversationID,
		command.Group.State, command.Group.CurrentMembershipRevisionID, command.Group.CreatedAt,
		command.Conversation.Title, command.Conversation.CreatedAt, command.Conversation.UpdatedAt,
		command.Membership.ID, command.Membership.Revision, command.Membership.CreatedAt,
	}}
}

func groupMemberRows(command ledger.CreateGroupCommand) *fakeRows {
	values := make([][]any, 0, len(command.Membership.Members))
	for index, member := range command.Membership.Members {
		binding := command.MemberBindings[index]
		values = append(values, []any{member.AgentID, member.Position, binding.BehaviorRevisionID,
			binding.BindingRevisionID, binding.ParticipantID})
	}
	return &fakeRows{values: values}
}

func currentGroupRecipientRow(command ledger.CreateAgentCommand) row {
	worker := command.Binding.ComputerID
	if worker == "" {
		worker = command.Binding.CloudRuntime
	}
	return fakeRow{values: []any{
		string(command.Agent.State), command.Agent.CurrentBehaviorRevisionID,
		command.Agent.CurrentBindingRevisionID, command.Behavior.ID,
		command.Binding.ID, command.Binding.AgentID, command.Binding.BehaviorRevisionID,
		command.Binding.SeatID, command.Binding.FortProfile, command.Binding.Provider,
		command.Binding.RequestedModel, worker, command.Binding.AuthorityID,
		command.Binding.AuthorityRevision, command.Binding.PolicyID,
		command.Binding.PolicyRevision, command.Profile.Name,
	}}
}

func containsArgument(arguments []any, want string) bool {
	for _, argument := range arguments {
		if argument == want {
			return true
		}
	}
	return false
}

func collaborationTestKeyRing() securebody.KeyRing {
	return securebody.KeyRing{
		ActiveKeyID: "test-key", Keys: map[string][]byte{
			"test-key": []byte("0123456789abcdef0123456789abcdef"),
		},
		Random: bytes.NewReader(bytes.Repeat([]byte{7}, 4096)),
	}
}

func mustInitialTargets(t *testing.T, command ledger.SendGroupTurnCommand,
	create ledger.CreateGroupCommand) []conversation.GroupInitialTarget {
	t.Helper()
	targets, err := command.Envelope.InitialTargets(create.Group, create.Membership)
	if err != nil {
		t.Fatal(err)
	}
	return targets
}

func postgresHandoffCommand(t *testing.T, turn ledger.GroupTurnRecord, group ledger.CreateGroupCommand,
	recipient ledger.CreateAgentCommand) ledger.AcceptHandoffCommand {
	t.Helper()
	now := turn.Envelope.CreatedAt.Add(time.Minute)
	root := conversation.AuthorityGrant{
		ID: "grant:human:handoff", Permissions: []string{"read", "write"},
		ContextRecordIDs: []string{"message:41"},
	}
	policy := conversation.AuthorityGrant{ID: "policy:handoff", Permissions: []string{"read"}}
	recipientPolicy := conversation.AuthorityGrant{ID: "policy:recipient", Permissions: []string{"read", "browser"}}
	effective, err := conversation.ComputeEffectiveAuthority([]string{"read"}, root, policy, recipientPolicy)
	if err != nil {
		t.Fatal(err)
	}
	return ledger.AcceptHandoffCommand{
		Handoff: conversation.Handoff{
			ID: "handoff:1", AccountID: testAccountID, IdempotencyKey: "handoff-command:1",
			State: conversation.HandoffQueued, CreatedByKind: conversation.HandoffActorHuman, CreatedByID: "human:toby",
			SourceMessageID: "41", RecipientAgentID: recipient.Agent.ID,
			RecipientBehaviorRevisionID: recipient.Behavior.ID, RecipientBindingRevisionID: recipient.Binding.ID,
			SourceConversationID: group.Conversation.ID, OutputConversationID: recipient.Home.ID,
			Context: conversation.ContextManifest{References: []conversation.ContextReference{{
				Kind: conversation.ContextMessage, ID: "41", AccountID: testAccountID, Immutable: true,
			}}},
			RequestedResult: "Review the launch evidence.", RootDelegationGrant: root,
			HandoffPolicy: policy, RecipientBindingPolicy: recipientPolicy,
			RequestedAuthority: []string{"read"}, EffectiveAuthority: effective,
			BudgetClass: conversation.LimitUnknown, MaxAgentMessages: conversation.MaxGroupAgentMessages,
			MaxDepth: conversation.MaxGroupHandoffDepth, Depth: 1,
			Deadline: now.Add(10 * time.Minute), CreatedAt: now,
		},
		TargetID: "target:handoff:1", ParticipantID: recipient.Participant.ID,
		ProjectionConversationIDs: []string{group.Conversation.ID},
	}
}
