package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/ledger"
)

func TestGetAgentReconstructsExactImmutableEvidenceInsideAccountTransaction(t *testing.T) {
	t.Parallel()

	command := postgresAgentCommand()
	tx := &fakeTransaction{}
	tx.queryRowHook = func(sql string, arguments []any) row {
		if !strings.Contains(sql, "from fort_private.stable_agent") ||
			!strings.Contains(sql, "where agent.account_id = $1 and agent.agent_id = $2") {
			return fakeRow{err: errors.New("GetAgent query is not explicitly account scoped")}
		}
		if len(arguments) != 2 || arguments[0] != testAccountID || arguments[1] != command.Agent.ID {
			return fakeRow{err: errors.New("GetAgent query has wrong arguments")}
		}
		return agentRecordRow(t, command)
	}
	store, err := newStore(&fakeDatabase{transactions: []transaction{tx}}, testAccountID)
	if err != nil {
		t.Fatalf("newStore: %v", err)
	}

	record, err := store.GetAgent(context.Background(), testAccountID, command.Agent.ID)
	if err != nil {
		t.Fatalf("GetAgent: %v", err)
	}
	if !reflect.DeepEqual(record, agentRecordFromCommand(command)) {
		t.Fatalf("Agent record =\n%+v\nwant\n%+v", record, agentRecordFromCommand(command))
	}
	if tx.commits != 1 || tx.rollbacks != 0 {
		t.Fatalf("GetAgent lifecycle = commits %d rollbacks %d", tx.commits, tx.rollbacks)
	}
}

func TestGetAgentMapsMissingAccountScopedRowToLedgerNotFound(t *testing.T) {
	t.Parallel()

	tx := &fakeTransaction{queryRowHook: func(string, []any) row { return fakeRow{err: pgx.ErrNoRows} }}
	store, err := newStore(&fakeDatabase{transactions: []transaction{tx}}, testAccountID)
	if err != nil {
		t.Fatalf("newStore: %v", err)
	}
	_, err = store.GetAgent(context.Background(), testAccountID, "agent:missing")
	if !errors.Is(err, ledger.ErrNotFound) {
		t.Fatalf("GetAgent error = %v, want not found", err)
	}
	if tx.commits != 0 || tx.rollbacks != 1 {
		t.Fatalf("missing Agent lifecycle = commits %d rollbacks %d", tx.commits, tx.rollbacks)
	}
}

func TestCreateAgentPersistsCompleteAggregateAtomically(t *testing.T) {
	t.Parallel()

	tx := &fakeTransaction{}
	database := &fakeDatabase{transactions: []transaction{tx}}
	store, err := newStore(database, testAccountID)
	if err != nil {
		t.Fatalf("newStore: %v", err)
	}
	command := postgresAgentCommand()

	created, err := store.CreateAgent(context.Background(), command)
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	want := agentRecordFromCommand(command)
	if !reflect.DeepEqual(created, want) {
		t.Fatalf("created record =\n%+v\nwant\n%+v", created, want)
	}
	if tx.commits != 1 || tx.rollbacks != 0 {
		t.Fatalf("transaction lifecycle = commits %d rollbacks %d", tx.commits, tx.rollbacks)
	}

	written := make([]string, 0, len(tx.execs))
	for _, statement := range tx.execs {
		written = append(written, statement.sql)
	}
	for _, table := range []string{
		"idempotency_record", "execution_source", "source_agent", "conversation",
		"stable_agent", "agent_profile_revision", "agent_behavior_revision",
		"agent_binding_revision", "agent_conversation",
		"conversation_membership_revision", "conversation_member_revision",
		"conversation_participant",
	} {
		if !containsSQL(written, "fort_private."+table) {
			t.Fatalf("CreateAgent did not persist %s; statements = %#v", table, written)
		}
	}
	if len(tx.execs) < 2 || !strings.Contains(tx.execs[1].sql, "fort_private.idempotency_record") {
		t.Fatalf("first scoped write = %q, want idempotency reservation", tx.execs[1].sql)
	}
}

func TestCreateAgentReplaysExactCommandAndRejectsConflictingKey(t *testing.T) {
	t.Parallel()

	command := postgresAgentCommand()
	digest, err := command.Digest()
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}

	t.Run("exact replay", func(t *testing.T) {
		t.Parallel()
		tx := replayTransaction(digest, command.Agent.ID)
		store, err := newStore(&fakeDatabase{transactions: []transaction{tx}}, testAccountID)
		if err != nil {
			t.Fatalf("newStore: %v", err)
		}
		replayed, err := store.CreateAgent(context.Background(), command)
		if err != nil {
			t.Fatalf("CreateAgent replay: %v", err)
		}
		if !reflect.DeepEqual(replayed, agentRecordFromCommand(command)) {
			t.Fatalf("replayed record = %+v", replayed)
		}
		if tx.commits != 1 || tx.rollbacks != 0 || len(tx.execs) != 2 {
			t.Fatalf("replay lifecycle = commits %d rollbacks %d writes %d", tx.commits, tx.rollbacks, len(tx.execs))
		}
	})

	t.Run("conflicting replay", func(t *testing.T) {
		t.Parallel()
		conflict := command
		conflict.Profile.Title = "A different title"
		tx := replayTransaction(digest, command.Agent.ID)
		store, err := newStore(&fakeDatabase{transactions: []transaction{tx}}, testAccountID)
		if err != nil {
			t.Fatalf("newStore: %v", err)
		}
		_, err = store.CreateAgent(context.Background(), conflict)
		if !errors.Is(err, ledger.ErrIdempotencyConflict) {
			t.Fatalf("CreateAgent error = %v, want idempotency conflict", err)
		}
		if tx.commits != 0 || tx.rollbacks != 1 || len(tx.execs) != 2 {
			t.Fatalf("conflict lifecycle = commits %d rollbacks %d writes %d", tx.commits, tx.rollbacks, len(tx.execs))
		}
	})
}

func TestCreateAgentRollsBackAggregateOnWriteFailure(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("profile insert unavailable")
	tx := &fakeTransaction{}
	tx.execHook = func(sql string, _ []any) (int64, error) {
		if strings.Contains(sql, "fort_private.agent_profile_revision") {
			return 0, wantErr
		}
		return 1, nil
	}
	store, err := newStore(&fakeDatabase{transactions: []transaction{tx}}, testAccountID)
	if err != nil {
		t.Fatalf("newStore: %v", err)
	}

	if _, err := store.CreateAgent(context.Background(), postgresAgentCommand()); !errors.Is(err, wantErr) {
		t.Fatalf("CreateAgent error = %v, want %v", err, wantErr)
	}
	if tx.commits != 0 || tx.rollbacks != 1 {
		t.Fatalf("failed aggregate lifecycle = commits %d rollbacks %d", tx.commits, tx.rollbacks)
	}
}

func replayTransaction(digest, agentID string) *fakeTransaction {
	tx := &fakeTransaction{}
	tx.execHook = func(sql string, _ []any) (int64, error) {
		if strings.Contains(sql, "fort_private.idempotency_record") {
			return 0, nil
		}
		return 1, nil
	}
	tx.queryRowHook = func(sql string, _ []any) row {
		if strings.Contains(sql, "fort_private.idempotency_record") {
			return fakeRow{values: []any{digest, "stable_agent", agentID}}
		}
		return fakeRow{err: errors.New("unexpected replay query")}
	}
	return tx
}

func containsSQL(statements []string, fragment string) bool {
	for _, statement := range statements {
		if strings.Contains(statement, fragment) {
			return true
		}
	}
	return false
}

func postgresAgentCommand() ledger.CreateAgentCommand {
	now := time.Date(2026, 8, 21, 20, 0, 0, 0, time.UTC)
	agent := conversation.Agent{
		ID: "agent:researcher", AccountID: testAccountID, State: conversation.AgentOpen,
		CurrentProfileRevisionID: "profile:researcher:1", CurrentBehaviorRevisionID: "behavior:researcher:1",
		CurrentBindingRevisionID: "binding:researcher:1", CanonicalConversationID: "conversation:researcher:home",
		CreatedAt: now,
	}
	profile := conversation.AgentProfileRevision{
		ID: agent.CurrentProfileRevisionID, AgentID: agent.ID, Revision: 1, Name: "Researcher",
		Title: "Research Agent", Pinned: true, CreatedAt: now,
	}
	behavior := conversation.AgentBehaviorRevision{
		ID: agent.CurrentBehaviorRevisionID, AgentID: agent.ID, Revision: 1, Role: "Researcher",
		StandingInstructions: "Cite sources.", EnabledSkills: []string{"research"}, EnabledTools: []string{"web"}, CreatedAt: now,
	}
	binding := conversation.AgentBindingRevision{
		ID: agent.CurrentBindingRevisionID, AgentID: agent.ID, Revision: 1, BehaviorRevisionID: behavior.ID,
		ExecutionSourceID: "source:studio", SourceAgentID: "source-agent:studio:researcher",
		SeatID: "seat:researcher", FortProfile: "openclaw:researcher", Provider: "openclaw",
		RequestedModel: "openclaw-main", ResolvedModel: "openclaw-main", ComputerID: "worker:studio",
		AdapterID: "model.chat.openclaw", AdapterRevision: "adapter:1", SourceConfigDigest: strings.Repeat("a", 64),
		AuthorityID: "authority:chat", AuthorityRevision: "authority:1", PolicyID: "policy:chat", PolicyRevision: "policy:1",
		SessionBehavior: "agent_managed", MemoryBehavior: "agent_managed",
		CapabilityEvidence:  []string{"text", "workdir=/Users/fort/Workspaces/researcher"},
		ReadinessContractID: "readiness:chat", ReadinessContractRevision: "readiness:1", ActivatedAt: now,
	}
	source := conversation.ExecutionSource{
		ID: binding.ExecutionSourceID, AccountID: agent.AccountID, Framework: "openclaw", InstanceID: "instance:studio",
		GatewayID: "gateway:studio", DisplayName: "OpenClaw · Studio", LastSeenAt: now,
		ResourceSharing: conversation.ResourceSharingDisclosure{
			ProviderCredentials: conversation.ResourceMachineShared, Filesystem: conversation.ResourceMachineShared,
			BrowserSessions: conversation.ResourceMachineShared, FrameworkSessions: conversation.ResourceProfileScoped,
			SourceMemory: conversation.ResourceProfileScoped, ToolConfiguration: conversation.ResourceProfileScoped,
		},
	}
	sourceAgent := conversation.SourceAgent{
		ID: binding.SourceAgentID, ExecutionSourceID: source.ID, OpaqueSourceAgentID: "researcher", DisplayName: "Researcher", LastSeenAt: now,
	}
	home := conversation.Conversation{
		ID: agent.CanonicalConversationID, Title: "Home", State: conversation.ConversationOpen, CreatedAt: now, UpdatedAt: now,
	}
	participant := conversation.Participant{
		ID: "participant:researcher:1", ConversationID: home.ID, SeatID: binding.SeatID,
		Profile: binding.FortProfile, Agent: binding.Provider, Model: binding.RequestedModel, Machine: binding.ComputerID,
		DisplayName: profile.Name, Position: 0, State: conversation.ParticipantActive, CreatedAt: now,
	}
	link := conversation.AgentConversation{
		AgentID: agent.ID, ConversationID: home.ID, Kind: conversation.AgentConversationCanonical, CreatedAt: now,
	}
	return ledger.CreateAgentCommand{
		IdempotencyKey: "create-researcher", Agent: agent, Profile: profile, Behavior: behavior,
		Binding: binding, ExecutionSource: source, SourceAgent: sourceAgent, Home: home,
		Participant: participant, Link: link,
	}
}

func agentRecordFromCommand(command ledger.CreateAgentCommand) ledger.AgentRecord {
	command.Behavior.EnabledSkills = append([]string{}, command.Behavior.EnabledSkills...)
	command.Behavior.EnabledTools = append([]string{}, command.Behavior.EnabledTools...)
	command.Binding.CapabilityEvidence = append([]string{}, command.Binding.CapabilityEvidence...)
	sort.Strings(command.Behavior.EnabledSkills)
	sort.Strings(command.Behavior.EnabledTools)
	sort.Strings(command.Binding.CapabilityEvidence)
	return ledger.AgentRecord{
		Agent: command.Agent, Profile: command.Profile, Behavior: command.Behavior, Binding: command.Binding,
		ExecutionSource: command.ExecutionSource, SourceAgent: command.SourceAgent, Home: command.Home,
		Participant: command.Participant, Link: command.Link,
	}
}

func agentRecordRow(t *testing.T, command ledger.CreateAgentCommand) row {
	t.Helper()
	command = canonicalAgentCommand(command)
	skills, err := json.Marshal(command.Behavior.EnabledSkills)
	if err != nil {
		t.Fatal(err)
	}
	tools, err := json.Marshal(command.Behavior.EnabledTools)
	if err != nil {
		t.Fatal(err)
	}
	behaviorDigest, err := evidenceDigest(command.Behavior)
	if err != nil {
		t.Fatal(err)
	}
	capabilities, err := encodeCapabilityEvidence(command.Binding)
	if err != nil {
		t.Fatal(err)
	}
	sharing, err := json.Marshal(command.ExecutionSource.ResourceSharing)
	if err != nil {
		t.Fatal(err)
	}
	inventoryDigest, err := evidenceDigest(command.SourceAgent)
	if err != nil {
		t.Fatal(err)
	}
	seat, authority, participantDigest, err := participantEvidence(command)
	if err != nil {
		t.Fatal(err)
	}
	workerID, err := bindingWorker(command.Binding)
	if err != nil {
		t.Fatal(err)
	}

	return fakeRow{values: []any{
		command.Agent.ID, command.Agent.AccountID, command.Agent.State,
		command.Agent.CurrentProfileRevisionID, command.Agent.CurrentBehaviorRevisionID,
		command.Agent.CurrentBindingRevisionID, command.Agent.CanonicalConversationID,
		command.Agent.CreatedAt,

		command.Profile.ID, command.Profile.AgentID, command.Profile.Revision,
		command.Profile.Name, command.Profile.Title, command.Profile.AvatarURL,
		command.Profile.Hidden, command.Profile.Pinned, command.Profile.SortOrder,
		command.Profile.CreatedAt,

		command.Behavior.ID, command.Behavior.AgentID, command.Behavior.Revision,
		command.Behavior.Role, command.Behavior.StandingInstructions, string(skills),
		string(tools), command.Behavior.PromptMaterial, behaviorDigest,
		command.Behavior.CreatedAt,

		command.Binding.ID, command.Binding.AgentID, command.Binding.Revision,
		command.Binding.BehaviorRevisionID, command.Binding.ExecutionSourceID,
		command.Binding.SourceAgentID, workerID, command.Binding.SeatID,
		command.Binding.FortProfile, command.Binding.Provider, command.Binding.RequestedModel,
		command.Binding.ResolvedModel, command.Binding.AdapterID, command.Binding.AdapterRevision,
		command.Binding.SourceConfigDigest, command.Binding.AuthorityID,
		command.Binding.AuthorityRevision, command.Binding.PolicyID, command.Binding.PolicyRevision,
		command.Binding.SessionBehavior, command.Binding.MemoryBehavior, capabilities,
		command.Binding.ReadinessContractID, command.Binding.ReadinessContractRevision,
		command.Binding.SupersedesRevisionID, command.Binding.ActivatedAt,

		command.ExecutionSource.ID, command.ExecutionSource.AccountID, workerID,
		command.ExecutionSource.Framework, command.ExecutionSource.InstanceID,
		command.ExecutionSource.GatewayID, command.ExecutionSource.DisplayName,
		string(sharing), command.Binding.SourceConfigDigest, command.ExecutionSource.LastSeenAt,

		command.SourceAgent.ID, command.SourceAgent.ExecutionSourceID,
		command.SourceAgent.OpaqueSourceAgentID, command.SourceAgent.DisplayName,
		inventoryDigest, command.SourceAgent.LastSeenAt,

		command.Home.ID, command.Home.Title, command.Home.State,
		command.Home.CreatedAt, command.Home.UpdatedAt,

		command.Participant.ID, command.Participant.ConversationID, seat, authority,
		participantDigest, command.Participant.CreatedAt,

		command.Link.AgentID, command.Link.ConversationID, command.Link.Kind,
		command.Link.CreatedAt,
	}}
}
