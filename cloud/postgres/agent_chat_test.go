package postgres

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/tobsai/fort/cloud/securebody"
	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/ledger"
)

func TestSendAgentTurnEncryptsAndPinsCurrentAgentRevisionAtomically(t *testing.T) {
	t.Parallel()
	agent := postgresAgentCommand()
	command := postgresDirectAgentTurnCommand(agent, "one")
	tx := &fakeTransaction{queryRows: &fakeRows{values: [][]any{{int64(40)}, {int64(41)}}}}
	tx.queryRowHook = func(sql string, _ []any) row {
		switch {
		case strings.Contains(sql, "from fort_private.stable_agent as agent") && strings.Contains(sql, "for update"):
			return postgresDirectParentRow(agent, agent.Binding.SourceConfigDigest)
		case strings.Contains(sql, "from fort_private.conversation_participant"):
			return fakeRow{err: pgx.ErrNoRows}
		case strings.Contains(sql, "returning message_id"):
			return fakeRow{values: []any{int64(41)}}
		default:
			return fakeRow{err: errors.New("unexpected direct Send query")}
		}
	}
	store, err := newStoreWithKeyRing(&fakeDatabase{transactions: []transaction{tx}}, testAccountID, collaborationTestKeyRing())
	if err != nil {
		t.Fatalf("newStoreWithKeyRing: %v", err)
	}
	result, err := store.SendAgentTurn(context.Background(), command)
	if err != nil {
		t.Fatalf("SendAgentTurn: %v", err)
	}
	if !result.Created || result.Message.ID != 41 || result.Message.Body != command.Body ||
		result.Target.BehaviorRevisionID != agent.Behavior.ID || result.Target.BindingRevisionID != agent.Binding.ID ||
		result.Target.RunID != command.RunID || result.Context.MessageIDs[len(result.Context.MessageIDs)-1] != 41 {
		t.Fatalf("Agent direct Send = %+v", result)
	}
	statements := append(append([]recordedStatement{}, tx.execs...), tx.queries...)
	for _, statement := range statements {
		for _, argument := range statement.args {
			switch value := argument.(type) {
			case string:
				if value == command.Body {
					t.Fatalf("plaintext body reached Postgres in %q", statement.sql)
				}
			case []byte:
				if bytes.Equal(value, []byte(command.Body)) {
					t.Fatalf("plaintext body reached Postgres in %q", statement.sql)
				}
			}
		}
	}
	for _, table := range []string{"idempotency_record", "conversation_participant", "context_manifest",
		"context_manifest_message", "delegation_grant", "conversation_message", "conversation_turn",
		"conversation_target", "conversation_target_binding", "ledger_event"} {
		if !agentChatSQLContains(statements, "fort_private."+table) {
			t.Fatalf("direct Send omitted %s: %+v", table, tx.execs)
		}
	}
	if tx.commits != 1 || tx.rollbacks != 0 {
		t.Fatalf("direct Send transaction = commits %d rollbacks %d", tx.commits, tx.rollbacks)
	}
}

func agentChatSQLContains(statements []recordedStatement, fragment string) bool {
	for _, statement := range statements {
		if strings.Contains(statement.sql, fragment) {
			return true
		}
	}
	return false
}

func TestSendAgentTurnFailsClosedOnExecutionSourceDigestDrift(t *testing.T) {
	t.Parallel()
	agent := postgresAgentCommand()
	command := postgresDirectAgentTurnCommand(agent, "drift")
	tx := &fakeTransaction{queryRowHook: func(sql string, _ []any) row {
		if strings.Contains(sql, "from fort_private.stable_agent as agent") && strings.Contains(sql, "for update") {
			return postgresDirectParentRow(agent, strings.Repeat("b", 64))
		}
		return fakeRow{err: errors.New("unexpected drift query")}
	}}
	store, err := newStoreWithKeyRing(&fakeDatabase{transactions: []transaction{tx}}, testAccountID, collaborationTestKeyRing())
	if err != nil {
		t.Fatalf("newStoreWithKeyRing: %v", err)
	}
	if _, err := store.SendAgentTurn(context.Background(), command); !errors.Is(err, ledger.ErrSourceDrift) {
		t.Fatalf("drift Send error = %v, want source drift", err)
	}
	if recordedSQLContains(tx.execs, "fort_private.conversation_message") || tx.commits != 0 || tx.rollbacks != 1 {
		t.Fatalf("drift Send wrote work: %+v", tx.execs)
	}
	if len(tx.queries) == 0 || !strings.Contains(tx.queries[len(tx.queries)-1].sql,
		"fort_private.execution_source_config_observation") {
		t.Fatalf("drift Send did not resolve the latest source observation: %+v", tx.queries)
	}
}

func TestObserveExecutionSourceConfigAppendsAndReadsExactPostgresEvidence(t *testing.T) {
	t.Parallel()
	agent := postgresAgentCommand()
	command := ledger.ObserveExecutionSourceConfigCommand{
		IdempotencyKey: "observe:source", ObservationID: "source-observation:source",
		AccountID: testAccountID, ExecutionSourceID: agent.Binding.ExecutionSourceID,
		SourceConfigDigest: strings.Repeat("b", 64), ObservedBy: "worker:studio",
		ObservedAt: agent.Agent.CreatedAt.Add(time.Minute),
	}
	tx := &fakeTransaction{queryRowHook: func(sql string, _ []any) row {
		if strings.Contains(sql, "from fort_private.execution_source_config_observation") {
			return postgresSourceConfigObservationRow(command, 2)
		}
		return fakeRow{err: errors.New("unexpected source observation query")}
	}}
	store, err := newStore(&fakeDatabase{transactions: []transaction{tx}}, testAccountID)
	if err != nil {
		t.Fatalf("newStore: %v", err)
	}
	result, err := store.ObserveExecutionSourceConfig(context.Background(), command)
	if err != nil {
		t.Fatalf("ObserveExecutionSourceConfig: %v", err)
	}
	if result.ID != command.ObservationID || result.Sequence != 2 ||
		result.SourceConfigDigest != command.SourceConfigDigest || result.ObservedBy != command.ObservedBy {
		t.Fatalf("source observation = %+v", result)
	}
	if !recordedSQLContains(tx.execs, "fort_private.execution_source_config_observation") ||
		!recordedSQLContains(tx.execs, "fort_private.idempotency_record") {
		t.Fatalf("source observation writes = %+v", tx.execs)
	}
	for _, statement := range tx.execs {
		if strings.Contains(strings.ToLower(statement.sql), "update fort_private.execution_source_config_observation") ||
			strings.Contains(strings.ToLower(statement.sql), "delete from fort_private.execution_source_config_observation") {
			t.Fatalf("source observation was not append-only: %q", statement.sql)
		}
	}
}

func TestLatestExecutionSourceConfigObservationReadsHighestAppendSequence(t *testing.T) {
	t.Parallel()
	agent := postgresAgentCommand()
	command := ledger.ObserveExecutionSourceConfigCommand{
		ObservationID: "source-observation:latest", AccountID: testAccountID,
		ExecutionSourceID: agent.Binding.ExecutionSourceID, SourceConfigDigest: strings.Repeat("b", 64),
		ObservedBy: "worker:studio", ObservedAt: agent.Agent.CreatedAt.Add(time.Minute),
	}
	tx := &fakeTransaction{queryRowHook: func(sql string, _ []any) row {
		if !strings.Contains(sql, "order by observation_sequence desc") {
			return fakeRow{err: errors.New("latest observation is not sequence ordered")}
		}
		return postgresSourceConfigObservationRow(command, 9)
	}}
	store, err := newStore(&fakeDatabase{transactions: []transaction{tx}}, testAccountID)
	if err != nil {
		t.Fatalf("newStore: %v", err)
	}
	result, err := store.LatestExecutionSourceConfigObservation(context.Background(), testAccountID,
		agent.Binding.ExecutionSourceID)
	if err != nil || result.Sequence != 9 {
		t.Fatalf("LatestExecutionSourceConfigObservation = %+v, %v", result, err)
	}
}

func TestAgentTargetCancelAndRetryKeepImmutablePostgresPins(t *testing.T) {
	t.Parallel()
	agent := postgresAgentCommand()
	command := postgresDirectAgentTurnCommand(agent, "target-life")
	cancelTx := &fakeTransaction{queryRowHook: func(sql string, _ []any) row {
		if strings.Contains(sql, "from fort_private.conversation_target as target") {
			return postgresTargetMutationRow(agent, command, "queued", 0)
		}
		return fakeRow{err: errors.New("unexpected cancel query")}
	}}
	retryTx := &fakeTransaction{queryRowHook: func(sql string, _ []any) row {
		if strings.Contains(sql, "from fort_private.conversation_target as target") {
			return postgresTargetMutationRow(agent, command, "canceled", 0)
		}
		return fakeRow{err: errors.New("unexpected retry query")}
	}}
	store, err := newStore(&fakeDatabase{transactions: []transaction{cancelTx, retryTx}}, testAccountID)
	if err != nil {
		t.Fatalf("newStore: %v", err)
	}
	canceledAt := command.CreatedAt.Add(1e9)
	canceled, err := store.CancelAgentTarget(context.Background(), ledger.CancelAgentTargetCommand{
		IdempotencyKey: "cancel:target-life", AccountID: testAccountID, AgentID: agent.Agent.ID,
		ConversationID: command.ConversationID, TargetID: command.TargetID, CanceledBy: "human:toby",
		CanceledAt: canceledAt,
	})
	if err != nil || canceled.State != "canceled" || canceled.BindingRevisionID != agent.Binding.ID {
		t.Fatalf("CancelAgentTarget = %+v, %v", canceled, err)
	}
	retried, err := store.RetryAgentTarget(context.Background(), ledger.RetryAgentTargetCommand{
		IdempotencyKey: "retry:target-life", AccountID: testAccountID, AgentID: agent.Agent.ID,
		ConversationID: command.ConversationID, TargetID: command.TargetID, RetriedBy: "human:toby",
		RetriedAt: canceledAt.Add(1e9),
	})
	if err != nil || retried.State != "queued" || retried.BehaviorRevisionID != agent.Behavior.ID ||
		retried.BindingRevisionID != agent.Binding.ID || retried.RunID != command.RunID {
		t.Fatalf("RetryAgentTarget = %+v, %v", retried, err)
	}
	for _, tx := range []*fakeTransaction{cancelTx, retryTx} {
		for _, statement := range tx.execs {
			if strings.Contains(strings.ToLower(statement.sql), "update fort_private.conversation_target_binding") {
				t.Fatalf("target lifecycle mutated pins: %q", statement.sql)
			}
		}
		if !recordedSQLContains(tx.execs, "fort_private.ledger_event") {
			t.Fatalf("target lifecycle omitted event: %+v", tx.execs)
		}
	}
}

func TestReadAgentConversationDecryptsOrderedClientProjection(t *testing.T) {
	t.Parallel()
	agent := postgresAgentCommand()
	command := postgresDirectAgentTurnCommand(agent, "read")
	ring := collaborationTestKeyRing()
	cipher := secureCollaborationBodyCipher{ring: ring}
	encrypted, err := cipher.seal(securebody.Scope{AccountID: testAccountID,
		RecordType: "conversation_message", RecordID: command.TurnID}, command.Body)
	if err != nil {
		t.Fatalf("seal fixture: %v", err)
	}
	membershipID := homeMembershipID(testAccountID, agent.Agent.ID, agent.Home.ID)
	base := &fakeTransaction{queryRowHook: func(sql string, _ []any) row {
		if strings.Contains(sql, "from fort_private.agent_conversation") {
			return postgresAgentConversationRow(agent.Home, agent.Link, false, time.Time{})
		}
		return fakeRow{err: errors.New("unexpected projection query row")}
	}}
	tx := &agentChatRowsTransaction{fakeTransaction: base, rowSets: []rows{
		&fakeRows{values: [][]any{{int64(41), agent.Home.ID, command.TurnID, "",
			"", "", "human", string(conversation.AuthorHuman), command.HumanID, "", encrypted.Ciphertext, encrypted.KeyID,
			encrypted.Nonce, encrypted.Digest, encrypted.PlaintextBytes, command.CreatedAt}}},
		&fakeRows{values: [][]any{{command.TurnID, agent.Home.ID, command.ClientTurnID, int64(41), int64(41),
			membershipID, command.ContextManifestID, "open", command.CreatedAt}}},
		&fakeRows{values: [][]any{{command.TargetID, command.TurnID, agent.Home.ID, agent.Agent.ID,
			agent.Behavior.ID, agent.Binding.ID, agent.Participant.ID, command.RunID, "queued", 0,
			command.CreatedAt, command.CreatedAt}}},
	}}
	store, err := newStoreWithKeyRing(&fakeDatabase{transactions: []transaction{tx}}, testAccountID, ring)
	if err != nil {
		t.Fatalf("newStoreWithKeyRing: %v", err)
	}
	projection, err := store.ReadAgentConversation(context.Background(), testAccountID, agent.Agent.ID, agent.Home.ID)
	if err != nil {
		t.Fatalf("ReadAgentConversation: %v", err)
	}
	if projection.Messages == nil || projection.Turns == nil || projection.Targets == nil ||
		len(projection.Messages) != 1 || len(projection.Turns) != 1 || len(projection.Targets) != 1 ||
		projection.Messages[0].Body != command.Body || projection.Targets[0].BindingRevisionID != agent.Binding.ID {
		t.Fatalf("Agent Conversation projection = %+v", projection)
	}
}

func TestReadAgentConversationDecryptsOrdinaryHandoffAndRoutineMessageScopes(t *testing.T) {
	t.Parallel()
	agent := postgresAgentCommand()
	ring := collaborationTestKeyRing()
	cipher := secureCollaborationBodyCipher{ring: ring}
	type fixture struct {
		id                                  int64
		turnID, targetID, handoffID, runID  string
		kind, authorID, authorAgentID, body string
		encrypted                           collaborationEncryptedBody
	}
	fixtures := []fixture{
		{id: 41, turnID: "turn:direct", kind: "human", authorID: "human:toby", body: "Start here."},
		{id: 42, turnID: "turn:direct", targetID: "target:direct", kind: "agent", authorID: agent.Agent.ID,
			authorAgentID: agent.Agent.ID, body: "Ordinary answer."},
		{id: 43, turnID: "turn:handoff", targetID: "target:handoff", handoffID: "handoff:one", kind: "handoff_result",
			authorID: agent.Agent.ID, authorAgentID: agent.Agent.ID, body: "Handoff answer."},
		{id: 44, turnID: "turn:routine", targetID: "target:routine", runID: "routine-run:one", kind: "routine_result",
			authorID: agent.Agent.ID, authorAgentID: agent.Agent.ID, body: "Routine answer."},
	}
	for index := range fixtures {
		recordType, recordID := "conversation_message", fixtures[index].turnID
		switch fixtures[index].kind {
		case "agent":
			recordID = fixtures[index].targetID
		case "handoff_result":
			recordType, recordID = "handoff_result", fixtures[index].handoffID
		case "routine_result":
			recordType, recordID = "routine_result", fixtures[index].runID
		}
		encrypted, err := cipher.seal(securebody.Scope{AccountID: testAccountID,
			RecordType: recordType, RecordID: recordID}, fixtures[index].body)
		if err != nil {
			t.Fatal(err)
		}
		fixtures[index].encrypted = encrypted
	}
	messageValues := make([][]any, 0, len(fixtures))
	for _, item := range fixtures {
		authorKind := string(conversation.AuthorHuman)
		if item.authorAgentID != "" {
			authorKind = string(conversation.AuthorAssistant)
		}
		messageValues = append(messageValues, []any{
			item.id, agent.Home.ID, item.turnID, item.targetID, item.handoffID, item.runID, item.kind,
			authorKind, item.authorID, item.authorAgentID, item.encrypted.Ciphertext, item.encrypted.KeyID,
			item.encrypted.Nonce, item.encrypted.Digest, item.encrypted.PlaintextBytes,
			agent.Agent.CreatedAt.Add(time.Duration(item.id) * time.Second),
		})
	}
	base := &fakeTransaction{queryRowHook: func(sql string, _ []any) row {
		if strings.Contains(sql, "from fort_private.agent_conversation") {
			return postgresAgentConversationRow(agent.Home, agent.Link, false, time.Time{})
		}
		return fakeRow{err: errors.New("unexpected projection query row")}
	}}
	tx := &agentChatRowsTransaction{fakeTransaction: base, rowSets: []rows{
		&fakeRows{values: messageValues}, &fakeRows{}, &fakeRows{},
	}}
	store, err := newStoreWithKeyRing(&fakeDatabase{transactions: []transaction{tx}}, testAccountID, ring)
	if err != nil {
		t.Fatal(err)
	}

	projection, err := store.ReadAgentConversation(context.Background(), testAccountID, agent.Agent.ID, agent.Home.ID)
	if err != nil {
		t.Fatalf("ReadAgentConversation: %v", err)
	}
	if len(projection.Messages) != len(fixtures) {
		t.Fatalf("message count = %d, want %d", len(projection.Messages), len(fixtures))
	}
	for index, item := range fixtures {
		message := projection.Messages[index]
		if message.Body != item.body || message.AuthorAgentID != item.authorAgentID || message.TargetID != item.targetID {
			t.Fatalf("message %d = %+v", index, message)
		}
	}
}

func postgresDirectAgentTurnCommand(agent ledger.CreateAgentCommand, suffix string) ledger.SendAgentTurnCommand {
	createdAt := agent.Agent.CreatedAt.Add(10 * 60 * 1e9)
	return ledger.SendAgentTurnCommand{
		IdempotencyKey: "send:" + suffix, AccountID: testAccountID, AgentID: agent.Agent.ID,
		ConversationID: agent.Home.ID, TurnID: "turn:" + suffix, ClientTurnID: "client-turn:" + suffix,
		ContextManifestID: "context:" + suffix, DelegationGrantID: "grant:" + suffix,
		TargetID: "target:" + suffix, RunID: "run:" + suffix, HumanID: "human:toby",
		Body: "hello " + suffix, CreatedBy: "human:toby", CreatedAt: createdAt,
		HardDeadline: createdAt.Add(10 * 60 * 1e9),
	}
}

func postgresDirectParentRow(agent ledger.CreateAgentCommand, observedDigest string) row {
	workerID, _ := bindingWorker(agent.Binding)
	return fakeRow{values: []any{
		string(agent.Agent.State), string(agent.Home.State), homeMembershipID(testAccountID, agent.Agent.ID, agent.Home.ID),
		agent.Behavior.ID, agent.Binding.ID, agent.Binding.ExecutionSourceID,
		agent.Binding.SourceConfigDigest, observedDigest, agent.Binding.SeatID, agent.Binding.FortProfile,
		agent.Binding.Provider, agent.Binding.RequestedModel, workerID, agent.Binding.AuthorityID,
		agent.Binding.AuthorityRevision, agent.Binding.PolicyID, agent.Binding.PolicyRevision,
		agent.Profile.Name, "enrolled",
	}}
}

func postgresSourceConfigObservationRow(command ledger.ObserveExecutionSourceConfigCommand, sequence int64) row {
	return fakeRow{values: []any{command.ObservationID, sequence, command.AccountID,
		command.ExecutionSourceID, command.SourceConfigDigest, command.ObservedBy, command.ObservedAt}}
}

func postgresTargetMutationRow(agent ledger.CreateAgentCommand, command ledger.SendAgentTurnCommand, state string, attempts int) row {
	return fakeRow{values: []any{
		command.TargetID, command.TurnID, command.ConversationID, command.AgentID,
		agent.Behavior.ID, agent.Binding.ID,
		postgresDirectParticipantID(testAccountID, command.AgentID, command.ConversationID, agent.Binding.ID),
		command.RunID, state, attempts, command.CreatedAt, command.CreatedAt,
		string(agent.Agent.State), string(agent.Home.State), agent.Binding.SourceConfigDigest,
		agent.Binding.SourceConfigDigest, "enrolled",
	}}
}

type agentChatRowsTransaction struct {
	*fakeTransaction
	rowSets []rows
}

func (tx *agentChatRowsTransaction) query(_ context.Context, sql string, args ...any) (rows, error) {
	tx.queries = append(tx.queries, recordedStatement{sql: sql, args: append([]any{}, args...)})
	if len(tx.rowSets) == 0 {
		return nil, errors.New("unexpected Agent chat rows query")
	}
	result := tx.rowSets[0]
	tx.rowSets = tx.rowSets[1:]
	return result, nil
}
