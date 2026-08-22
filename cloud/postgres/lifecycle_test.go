package postgres

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/ledger"
)

func TestAppendAgentProfileUsesExpectedCurrentRevisionAndAppendsAuditEvent(t *testing.T) {
	t.Parallel()

	base := postgresAgentCommand()
	revision := base.Profile
	revision.ID = "profile:researcher:2"
	revision.Revision = 2
	revision.Name = "Principal Researcher"
	revision.CreatedAt = revision.CreatedAt.Add(time.Minute)
	command := ledger.AppendAgentProfileCommand{
		IdempotencyKey: "profile-2", AccountID: testAccountID, AgentID: base.Agent.ID,
		ExpectedProfileRevisionID: base.Profile.ID, Revision: revision, AcceptedBy: "human:toby",
	}
	updated := base
	updated.Agent.CurrentProfileRevisionID = revision.ID
	updated.Profile = revision
	tx := &fakeTransaction{queryRowHook: func(sql string, _ []any) row {
		switch {
		case strings.Contains(sql, "current_profile_revision_id, profile.revision"):
			return fakeRow{values: []any{base.Profile.ID, base.Profile.Revision}}
		case strings.Contains(sql, "from fort_private.stable_agent as agent"):
			return agentRecordRow(t, updated)
		default:
			return fakeRow{err: errors.New("unexpected profile query")}
		}
	}}
	store, err := newStore(&fakeDatabase{transactions: []transaction{tx}}, testAccountID)
	if err != nil {
		t.Fatalf("newStore: %v", err)
	}
	result, err := store.AppendAgentProfile(context.Background(), command)
	if err != nil {
		t.Fatalf("AppendAgentProfile: %v", err)
	}
	if result.Profile.ID != revision.ID {
		t.Fatalf("current profile = %+v", result.Profile)
	}
	if !recordedSQLContains(tx.execs, "agent_profile_revision") ||
		!recordedSQLContains(tx.execs, "current_profile_revision_id = $1") ||
		!recordedSQLContains(tx.execs, "agent.profile.advanced") {
		t.Fatalf("profile lifecycle SQL = %+v", tx.execs)
	}
}

func TestAppendAgentBehaviorCreatesImmutableSuccessorAndTransition(t *testing.T) {
	t.Parallel()

	base := postgresAgentCommand()
	acceptedAt := base.Binding.ActivatedAt.Add(time.Minute)
	behavior := base.Behavior
	behavior.ID = "behavior:researcher:2"
	behavior.Revision = 2
	behavior.StandingInstructions = "Cite primary sources."
	behavior.CreatedAt = acceptedAt
	binding := base.Binding
	binding.ID = "binding:researcher:2"
	binding.Revision = 2
	binding.BehaviorRevisionID = behavior.ID
	binding.SupersedesRevisionID = base.Binding.ID
	binding.SeatID = "seat:researcher:2"
	binding.ActivatedAt = acceptedAt
	participant := base.Participant
	participant.ID = "participant:researcher:2"
	participant.SeatID = binding.SeatID
	participant.CreatedAt = acceptedAt
	command := ledger.AppendAgentBehaviorCommand{
		IdempotencyKey: "behavior-2", AccountID: testAccountID, AgentID: base.Agent.ID,
		ExpectedBehaviorRevisionID: base.Behavior.ID, ExpectedBindingRevisionID: base.Binding.ID,
		Behavior: behavior, Binding: binding, Participant: participant,
		ReadinessEvidence: []string{"ready:binding-2"}, AuthorityEvidence: []string{"authority:binding-2"},
		AcceptedBy: "human:toby", AcceptedAt: acceptedAt,
	}
	updated := base
	updated.Agent.CurrentBehaviorRevisionID = behavior.ID
	updated.Agent.CurrentBindingRevisionID = binding.ID
	updated.Behavior = behavior
	updated.Binding = binding
	updated.Participant = participant
	readCount := 0
	tx := &fakeTransaction{queryRowHook: func(sql string, _ []any) row {
		if strings.Contains(sql, "from fort_private.stable_agent as agent") {
			readCount++
			if readCount == 1 {
				return agentRecordRow(t, base)
			}
			return agentRecordRow(t, updated)
		}
		return fakeRow{err: errors.New("unexpected Behavior query")}
	}}
	store, err := newStore(&fakeDatabase{transactions: []transaction{tx}}, testAccountID)
	if err != nil {
		t.Fatalf("newStore: %v", err)
	}
	result, err := store.AppendAgentBehavior(context.Background(), command)
	if err != nil {
		t.Fatalf("AppendAgentBehavior: %v", err)
	}
	if result.Agent.Behavior.ID != behavior.ID || result.Agent.Binding.ID != binding.ID ||
		result.Transition.PreviousBindingRevisionID != base.Binding.ID {
		t.Fatalf("Behavior result = %+v", result)
	}
	for _, fragment := range []string{"agent_behavior_revision", "agent_binding_revision", "agent_binding_transition", "agent.binding.advanced"} {
		if !recordedSQLContains(tx.execs, fragment) {
			t.Fatalf("Behavior lifecycle omitted %q: %+v", fragment, tx.execs)
		}
	}
	for _, statement := range tx.execs {
		if strings.Contains(strings.ToLower(statement.sql), "update fort_private.agent_binding_revision") {
			t.Fatalf("Behavior lifecycle mutated predecessor Binding: %q", statement.sql)
		}
	}
}

func TestExplicitRebindRequiresDigestBoundPreviewAndAppendsTransitionEvent(t *testing.T) {
	t.Parallel()

	base := postgresAgentCommand()
	proposal := postgresRebindProposal(base)
	previewTx := &fakeTransaction{queryRowHook: func(sql string, _ []any) row {
		if strings.Contains(sql, "from fort_private.stable_agent as agent") {
			return agentRecordRow(t, base)
		}
		return fakeRow{err: errors.New("unexpected Rebind preview query")}
	}}
	updated := base
	updated.Agent.CurrentBindingRevisionID = proposal.Binding.ID
	updated.Binding = proposal.Binding
	updated.ExecutionSource = proposal.ExecutionSource
	updated.SourceAgent = proposal.SourceAgent
	updated.Participant = proposal.Participant
	acceptReads := 0
	acceptTx := &fakeTransaction{queryRowHook: func(sql string, _ []any) row {
		if strings.Contains(sql, "from fort_private.stable_agent as agent") {
			acceptReads++
			if acceptReads == 1 {
				return agentRecordRow(t, base)
			}
			return agentRecordRow(t, updated)
		}
		return fakeRow{err: errors.New("unexpected Rebind accept query")}
	}}
	store, err := newStore(&fakeDatabase{transactions: []transaction{previewTx, acceptTx}}, testAccountID)
	if err != nil {
		t.Fatalf("newStore: %v", err)
	}
	preview, err := store.PreviewAgentRebind(context.Background(), proposal)
	if err != nil {
		t.Fatalf("PreviewAgentRebind: %v", err)
	}
	if preview.Digest == "" || preview.CurrentBinding.ID != base.Binding.ID || preview.ProposedBinding.ID != proposal.Binding.ID {
		t.Fatalf("preview = %+v", preview)
	}
	result, err := store.AcceptAgentRebind(context.Background(), ledger.AcceptAgentRebindCommand{
		IdempotencyKey: "rebind-2", Preview: preview, AcceptedBy: "human:toby", AcceptedAt: proposal.Binding.ActivatedAt,
	})
	if err != nil {
		t.Fatalf("AcceptAgentRebind: %v", err)
	}
	if result.Agent.Binding.ID != proposal.Binding.ID || result.Agent.Behavior.ID != base.Behavior.ID ||
		result.Transition.PreviewDigest != preview.Digest {
		t.Fatalf("Rebind result = %+v", result)
	}
	for _, fragment := range []string{"agent_binding_revision", "conversation_participant", "agent_binding_transition", "agent.binding.advanced"} {
		if !recordedSQLContains(acceptTx.execs, fragment) {
			t.Fatalf("Rebind omitted %q: %+v", fragment, acceptTx.execs)
		}
	}
	for _, statement := range acceptTx.execs {
		if strings.Contains(strings.ToLower(statement.sql), "update fort_private.agent_binding_revision") {
			t.Fatalf("Rebind mutated predecessor Binding: %q", statement.sql)
		}
	}
}

func TestAgentSecondaryConversationLifecycleIsAccountScopedAndHomeGuarded(t *testing.T) {
	t.Parallel()

	base := postgresAgentCommand()
	now := base.Home.CreatedAt.Add(time.Minute)
	command := ledger.CreateSecondaryConversationCommand{
		IdempotencyKey: "secondary-market", AccountID: testAccountID, AgentID: base.Agent.ID,
		Conversation: conversation.Conversation{ID: "conversation:market", Title: "Market map", State: conversation.ConversationOpen, CreatedAt: now, UpdatedAt: now},
		Link:         conversation.AgentConversation{AgentID: base.Agent.ID, ConversationID: "conversation:market", Kind: conversation.AgentConversationSecondary, CreatedAt: now},
		CreatedBy:    "human:toby",
	}
	createTx := &fakeTransaction{queryRowHook: func(sql string, _ []any) row {
		switch {
		case strings.Contains(sql, "select state, canonical_conversation_id"):
			return fakeRow{values: []any{string(base.Agent.State), base.Agent.CanonicalConversationID}}
		case strings.Contains(sql, "from fort_private.agent_conversation"):
			return postgresAgentConversationRow(command.Conversation, command.Link, false, time.Time{})
		default:
			return fakeRow{err: errors.New("unexpected secondary create query")}
		}
	}}
	renamedConversation := command.Conversation
	renamedConversation.Title = "Market landscape"
	renamedConversation.UpdatedAt = now.Add(15 * time.Second)
	renameTx := &fakeTransaction{queryRowHook: func(sql string, _ []any) row {
		switch {
		case strings.Contains(sql, "select relation.kind, item.title"):
			return fakeRow{values: []any{string(command.Link.Kind), command.Conversation.Title}}
		case strings.Contains(sql, "from fort_private.agent_conversation"):
			return postgresAgentConversationRow(renamedConversation, command.Link, false, time.Time{})
		default:
			return fakeRow{err: errors.New("unexpected rename query")}
		}
	}}
	pinAt := now.Add(30 * time.Second)
	pinTx := &fakeTransaction{queryRowHook: func(sql string, _ []any) row {
		switch {
		case strings.Contains(sql, "select relation.kind"):
			return fakeRow{values: []any{string(command.Link.Kind), false, 0}}
		case strings.Contains(sql, "from fort_private.agent_conversation"):
			return postgresAgentConversationRow(command.Conversation, command.Link, true, pinAt)
		default:
			return fakeRow{err: errors.New("unexpected pin query")}
		}
	}}
	listTx := &fakeTransaction{
		queryRows: &fakeRows{values: [][]any{
			postgresAgentConversationValues(base.Home, base.Link, false, time.Time{}),
			postgresAgentConversationValues(command.Conversation, command.Link, true, pinAt),
		}},
		queryRowHook: func(sql string, _ []any) row {
			if strings.Contains(sql, "from fort_private.stable_agent") {
				return fakeRow{values: []any{1}}
			}
			return fakeRow{err: errors.New("unexpected list parent query")}
		},
	}
	archiveTx := &fakeTransaction{queryRowHook: func(sql string, _ []any) row {
		switch {
		case strings.Contains(sql, "select agent.state, relation.kind, item.state"):
			return fakeRow{values: []any{string(base.Agent.State), string(command.Link.Kind), string(conversation.ConversationOpen)}}
		case strings.Contains(sql, "from fort_private.agent_conversation"):
			archived := command.Conversation
			archived.State = conversation.ConversationArchived
			archived.UpdatedAt = now.Add(time.Minute)
			return postgresAgentConversationRow(archived, command.Link, true, pinAt)
		default:
			return fakeRow{err: errors.New("unexpected archive query")}
		}
	}}
	store, err := newStore(&fakeDatabase{transactions: []transaction{createTx, renameTx, pinTx, listTx, archiveTx}}, testAccountID)
	if err != nil {
		t.Fatalf("newStore: %v", err)
	}
	if _, err := store.CreateSecondaryConversation(context.Background(), command); err != nil {
		t.Fatalf("CreateSecondaryConversation: %v", err)
	}
	for _, fragment := range []string{"conversation_membership_revision", "conversation_member_revision", "agent_conversation", "ledger_event"} {
		if !recordedSQLContains(createTx.execs, fragment) {
			t.Fatalf("secondary create omitted %q: %+v", fragment, createTx.execs)
		}
	}
	renamed, err := store.RenameAgentConversation(context.Background(), ledger.RenameAgentConversationCommand{
		IdempotencyKey: "rename-market", AccountID: testAccountID, AgentID: base.Agent.ID,
		ConversationID: command.Conversation.ID, ExpectedTitle: command.Conversation.Title, Title: renamedConversation.Title,
		ChangedBy: "human:toby", ChangedAt: renamedConversation.UpdatedAt,
	})
	if err != nil || renamed.Conversation.Title != renamedConversation.Title {
		t.Fatalf("rename secondary = %+v, %v", renamed, err)
	}
	if !recordedSQLContains(renameTx.execs, "update fort_private.conversation set title") ||
		!recordedSQLContains(renameTx.execs, "ledger_event") || !recordedArgumentContains(renameTx.execs, "agent.conversation.renamed") {
		t.Fatalf("rename lifecycle SQL = %+v", renameTx.execs)
	}
	pinned, err := store.SetAgentConversationPin(context.Background(), ledger.SetAgentConversationPinCommand{
		IdempotencyKey: "pin-market", AccountID: testAccountID, AgentID: base.Agent.ID,
		ConversationID: command.Conversation.ID, ExpectedPinned: false, Pinned: true,
		ChangedBy: "human:toby", ChangedAt: pinAt,
	})
	if err != nil || !pinned.Pinned || !pinned.PinnedAt.Equal(pinAt) {
		t.Fatalf("pin secondary = %+v, %v", pinned, err)
	}
	listed, err := store.ListAgentConversations(context.Background(), testAccountID, base.Agent.ID)
	if err != nil || len(listed) != 2 || listed[0].Link.Kind != conversation.AgentConversationCanonical || !listed[1].Pinned {
		t.Fatalf("ListAgentConversations = %+v, %v", listed, err)
	}
	if len(listTx.queries) < 2 || !strings.Contains(listTx.queries[0].sql, "fort_private.stable_agent") {
		t.Fatalf("ListAgentConversations did not verify its parent Agent: %+v", listTx.queries)
	}
	archived, err := store.SetAgentConversationState(context.Background(), ledger.SetAgentConversationStateCommand{
		IdempotencyKey: "archive-market", AccountID: testAccountID, AgentID: base.Agent.ID,
		ConversationID: command.Conversation.ID, ExpectedState: conversation.ConversationOpen,
		State: conversation.ConversationArchived, ChangedBy: "human:toby", ChangedAt: now.Add(time.Minute),
	})
	if err != nil || archived.Conversation.State != conversation.ConversationArchived {
		t.Fatalf("archive secondary = %+v, %v", archived, err)
	}
}

func TestAgentConversationLifecycleRejectsCanonicalHomeMutations(t *testing.T) {
	t.Parallel()
	base := postgresAgentCommand()
	now := base.Home.UpdatedAt.Add(time.Minute)
	renameTx := &fakeTransaction{queryRowHook: func(sql string, _ []any) row {
		if strings.Contains(sql, "select relation.kind, item.title") {
			return fakeRow{values: []any{string(conversation.AgentConversationCanonical), base.Home.Title}}
		}
		return fakeRow{err: errors.New("unexpected Home rename query")}
	}}
	pinTx := &fakeTransaction{queryRowHook: func(sql string, _ []any) row {
		if strings.Contains(sql, "select relation.kind") {
			return fakeRow{values: []any{string(conversation.AgentConversationCanonical), false, 0}}
		}
		return fakeRow{err: errors.New("unexpected Home pin query")}
	}}
	archiveTx := &fakeTransaction{queryRowHook: func(sql string, _ []any) row {
		if strings.Contains(sql, "select agent.state, relation.kind, item.state") {
			return fakeRow{values: []any{string(conversation.AgentOpen), string(conversation.AgentConversationCanonical), string(conversation.ConversationOpen)}}
		}
		return fakeRow{err: errors.New("unexpected Home archive query")}
	}}
	store, err := newStore(&fakeDatabase{transactions: []transaction{renameTx, pinTx, archiveTx}}, testAccountID)
	if err != nil {
		t.Fatalf("newStore: %v", err)
	}
	if _, err := store.RenameAgentConversation(context.Background(), ledger.RenameAgentConversationCommand{
		IdempotencyKey: "rename-home", AccountID: testAccountID, AgentID: base.Agent.ID,
		ConversationID: base.Home.ID, ExpectedTitle: base.Home.Title, Title: "Elsewhere",
		ChangedBy: "human:toby", ChangedAt: now,
	}); !errors.Is(err, ledger.ErrStateConflict) {
		t.Fatalf("Home rename error = %v, want state conflict", err)
	}
	if _, err := store.SetAgentConversationPin(context.Background(), ledger.SetAgentConversationPinCommand{
		IdempotencyKey: "pin-home", AccountID: testAccountID, AgentID: base.Agent.ID,
		ConversationID: base.Home.ID, ExpectedPinned: false, Pinned: true,
		ChangedBy: "human:toby", ChangedAt: now,
	}); !errors.Is(err, ledger.ErrStateConflict) {
		t.Fatalf("Home pin error = %v, want state conflict", err)
	}
	if _, err := store.SetAgentConversationState(context.Background(), ledger.SetAgentConversationStateCommand{
		IdempotencyKey: "archive-home", AccountID: testAccountID, AgentID: base.Agent.ID,
		ConversationID: base.Home.ID, ExpectedState: conversation.ConversationOpen, State: conversation.ConversationArchived,
		ChangedBy: "human:toby", ChangedAt: now,
	}); !errors.Is(err, ledger.ErrStateConflict) {
		t.Fatalf("Home archive error = %v, want state conflict", err)
	}
}

func recordedSQLContains(statements []recordedStatement, fragment string) bool {
	for _, statement := range statements {
		if strings.Contains(statement.sql, fragment) {
			return true
		}
	}
	return false
}

func recordedArgumentContains(statements []recordedStatement, want string) bool {
	for _, statement := range statements {
		for _, value := range statement.args {
			if value == want {
				return true
			}
		}
	}
	return false
}

func postgresRebindProposal(base ledger.CreateAgentCommand) ledger.PreviewAgentRebindCommand {
	acceptedAt := base.Binding.ActivatedAt.Add(5 * time.Minute)
	binding := base.Binding
	binding.ID = "binding:researcher:2"
	binding.Revision = 2
	binding.ExecutionSourceID = "source:mini"
	binding.SourceAgentID = "source-agent:mini:researcher"
	binding.SeatID = "seat:researcher:mini"
	binding.FortProfile = "hermes:researcher"
	binding.Provider = "hermes"
	binding.RequestedModel = "hermes-main"
	binding.ResolvedModel = "hermes-main"
	binding.ComputerID = "worker:mini"
	binding.AdapterID = "model.chat.hermes"
	binding.AdapterRevision = "adapter:2"
	binding.SourceConfigDigest = strings.Repeat("b", 64)
	binding.SupersedesRevisionID = base.Binding.ID
	binding.ActivatedAt = acceptedAt
	source := base.ExecutionSource
	source.ID = binding.ExecutionSourceID
	source.Framework = "hermes"
	source.InstanceID = "instance:mini"
	source.GatewayID = "gateway:mini"
	source.DisplayName = "Hermes · Mini"
	source.LastSeenAt = acceptedAt
	sourceAgent := base.SourceAgent
	sourceAgent.ID = binding.SourceAgentID
	sourceAgent.ExecutionSourceID = source.ID
	sourceAgent.LastSeenAt = acceptedAt
	participant := base.Participant
	participant.ID = "participant:researcher:mini"
	participant.SeatID = binding.SeatID
	participant.Profile = binding.FortProfile
	participant.Agent = binding.Provider
	participant.Model = binding.RequestedModel
	participant.Machine = binding.ComputerID
	participant.CreatedAt = acceptedAt
	return ledger.PreviewAgentRebindCommand{
		AccountID: testAccountID, AgentID: base.Agent.ID, ExpectedBindingRevisionID: base.Binding.ID,
		Binding: binding, ExecutionSource: source, SourceAgent: sourceAgent, Participant: participant,
		NonTransferableResources: []ledger.RebindResource{ledger.RebindResourceFiles, ledger.RebindResourceSessions},
		ReadinessEvidence:        []string{"ready:hermes-mini"}, AuthorityEvidence: []string{"authority:revalidated"},
		GeneratedAt: acceptedAt.Add(-time.Minute),
	}
}

func postgresAgentConversationValues(item conversation.Conversation, link conversation.AgentConversation, pinned bool, pinnedAt time.Time) []any {
	var pinnedAtValue *time.Time
	if !pinnedAt.IsZero() {
		pinnedAtValue = &pinnedAt
	}
	return []any{item.ID, item.Title, string(item.State), item.CreatedAt, item.UpdatedAt,
		link.AgentID, link.ConversationID, string(link.Kind), link.CreatedAt, pinned, pinnedAtValue}
}

func postgresAgentConversationRow(item conversation.Conversation, link conversation.AgentConversation, pinned bool, pinnedAt time.Time) row {
	return fakeRow{values: postgresAgentConversationValues(item, link, pinned, pinnedAt)}
}
