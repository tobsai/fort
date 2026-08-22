package ledger_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/ledger"
)

func TestSQLiteAgentDirectChatSendIsAtomicIdempotentAndReadable(t *testing.T) {
	repository := openAgentChatRepository(t)
	created := stableAgentCommand(t)
	if _, err := repository.CreateAgent(context.Background(), created); err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	command := directAgentTurnCommand(created, created.Home.ID, "one")

	first, err := repository.SendAgentTurn(context.Background(), command)
	if err != nil {
		t.Fatalf("SendAgentTurn: %v", err)
	}
	if !first.Created || first.Message.Body != command.Body || first.Message.AuthorID != command.HumanID ||
		first.Turn.PromptMessageID != first.Message.ID || first.Target.RunID != command.RunID ||
		first.Target.AgentID != created.Agent.ID || first.Target.BehaviorRevisionID != created.Behavior.ID ||
		first.Target.BindingRevisionID != created.Binding.ID || first.Context.MessageIDs[len(first.Context.MessageIDs)-1] != first.Message.ID {
		t.Fatalf("direct Send result = %+v", first)
	}

	replayed, err := repository.SendAgentTurn(context.Background(), command)
	if err != nil {
		t.Fatalf("SendAgentTurn replay: %v", err)
	}
	if replayed.Created || replayed.Message.ID != first.Message.ID || replayed.Turn.ID != first.Turn.ID ||
		replayed.Target.ID != first.Target.ID || replayed.Context.ID != first.Context.ID {
		t.Fatalf("direct Send replay = %+v", replayed)
	}

	projection, err := repository.ReadAgentConversation(context.Background(), created.Agent.AccountID, created.Agent.ID, created.Home.ID)
	if err != nil {
		t.Fatalf("ReadAgentConversation: %v", err)
	}
	if projection.Messages == nil || projection.Turns == nil || projection.Targets == nil ||
		len(projection.Messages) != 1 || len(projection.Turns) != 1 || len(projection.Targets) != 1 ||
		projection.Messages[0].Body != command.Body || projection.Targets[0].BindingRevisionID != created.Binding.ID {
		t.Fatalf("direct Conversation projection = %+v", projection)
	}

	conflict := command
	conflict.Body = "different command under the same key"
	if _, err := repository.SendAgentTurn(context.Background(), conflict); !errors.Is(err, ledger.ErrIdempotencyConflict) {
		t.Fatalf("conflicting Send error = %v, want idempotency conflict", err)
	}
}

func TestSQLiteAgentDirectChatNewTurnUsesCurrentRevisionAndOldTargetKeepsPins(t *testing.T) {
	repository := openAgentChatRepository(t)
	created := stableAgentCommand(t)
	if _, err := repository.CreateAgent(context.Background(), created); err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	first, err := repository.SendAgentTurn(context.Background(), directAgentTurnCommand(created, created.Home.ID, "before"))
	if err != nil {
		t.Fatalf("first SendAgentTurn: %v", err)
	}

	acceptedAt := created.Binding.ActivatedAt.Add(time.Minute)
	behavior := created.Behavior
	behavior.ID, behavior.Revision, behavior.StandingInstructions, behavior.CreatedAt = "behavior:researcher:2", 2, "Use exact citations.", acceptedAt
	binding := created.Binding
	binding.ID, binding.Revision, binding.BehaviorRevisionID = "binding:researcher:2", 2, behavior.ID
	binding.SeatID, binding.SupersedesRevisionID, binding.ActivatedAt = "seat:researcher:2", created.Binding.ID, acceptedAt
	participant := created.Participant
	participant.ID, participant.SeatID, participant.CreatedAt = "participant:researcher:2", binding.SeatID, acceptedAt
	if _, err := repository.AppendAgentBehavior(context.Background(), ledger.AppendAgentBehaviorCommand{
		IdempotencyKey: "behavior-2", AccountID: created.Agent.AccountID, AgentID: created.Agent.ID,
		ExpectedBehaviorRevisionID: created.Behavior.ID, ExpectedBindingRevisionID: created.Binding.ID,
		Behavior: behavior, Binding: binding, Participant: participant,
		ReadinessEvidence: []string{"ready:2"}, AuthorityEvidence: []string{"authority:2"},
		AcceptedBy: "human:toby", AcceptedAt: acceptedAt,
	}); err != nil {
		t.Fatalf("AppendAgentBehavior: %v", err)
	}

	current := created
	current.Behavior, current.Binding = behavior, binding
	second, err := repository.SendAgentTurn(context.Background(), directAgentTurnCommand(current, created.Home.ID, "after"))
	if err != nil {
		t.Fatalf("second SendAgentTurn: %v", err)
	}
	if second.Target.BehaviorRevisionID != behavior.ID || second.Target.BindingRevisionID != binding.ID {
		t.Fatalf("new ordinary turn pins = %+v", second.Target)
	}
	projection, err := repository.ReadAgentConversation(context.Background(), created.Agent.AccountID, created.Agent.ID, created.Home.ID)
	if err != nil {
		t.Fatalf("ReadAgentConversation: %v", err)
	}
	if len(projection.Targets) != 2 || projection.Targets[0].BindingRevisionID != first.Target.BindingRevisionID ||
		projection.Targets[0].BindingRevisionID != created.Binding.ID || projection.Targets[1].BindingRevisionID != binding.ID {
		t.Fatalf("historical/current target pins = %+v", projection.Targets)
	}
}

func TestSQLiteAgentDirectChatVerifiesFullParentChainAndOpenState(t *testing.T) {
	repository := openAgentChatRepository(t)
	created := stableAgentCommand(t)
	if _, err := repository.CreateAgent(context.Background(), created); err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	now := created.Home.CreatedAt.Add(time.Minute)
	secondary := ledger.CreateSecondaryConversationCommand{
		IdempotencyKey: "secondary-chat", AccountID: created.Agent.AccountID, AgentID: created.Agent.ID,
		Conversation: conversation.Conversation{ID: "conversation:secondary", Title: "Secondary", State: conversation.ConversationOpen, CreatedAt: now, UpdatedAt: now},
		Link:         conversation.AgentConversation{AgentID: created.Agent.ID, ConversationID: "conversation:secondary", Kind: conversation.AgentConversationSecondary, CreatedAt: now},
		CreatedBy:    "human:toby",
	}
	if _, err := repository.CreateSecondaryConversation(context.Background(), secondary); err != nil {
		t.Fatalf("CreateSecondaryConversation: %v", err)
	}
	if _, err := repository.SendAgentTurn(context.Background(), directAgentTurnCommand(created, secondary.Conversation.ID, "secondary")); err != nil {
		t.Fatalf("secondary SendAgentTurn: %v", err)
	}
	foreign := directAgentTurnCommand(created, "conversation:foreign", "foreign")
	if _, err := repository.SendAgentTurn(context.Background(), foreign); !errors.Is(err, ledger.ErrNotFound) {
		t.Fatalf("foreign child error = %v, want not found", err)
	}
	if _, err := repository.SetAgentConversationState(context.Background(), ledger.SetAgentConversationStateCommand{
		IdempotencyKey: "archive-secondary", AccountID: created.Agent.AccountID, AgentID: created.Agent.ID,
		ConversationID: secondary.Conversation.ID, ExpectedState: conversation.ConversationOpen,
		State: conversation.ConversationArchived, ChangedBy: "human:toby", ChangedAt: now.Add(time.Minute),
	}); err != nil {
		t.Fatalf("archive secondary: %v", err)
	}
	archived := directAgentTurnCommand(created, secondary.Conversation.ID, "archived")
	if _, err := repository.SendAgentTurn(context.Background(), archived); !errors.Is(err, ledger.ErrStateConflict) {
		t.Fatalf("archived child error = %v, want state conflict", err)
	}
}

func TestSQLiteAgentDirectTargetCancelAndRetryRetainOriginalPins(t *testing.T) {
	repository := openAgentChatRepository(t)
	created := stableAgentCommand(t)
	if _, err := repository.CreateAgent(context.Background(), created); err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	sent, err := repository.SendAgentTurn(context.Background(), directAgentTurnCommand(created, created.Home.ID, "cancel"))
	if err != nil {
		t.Fatalf("SendAgentTurn: %v", err)
	}
	canceledAt := sent.Target.CreatedAt.Add(time.Minute)
	cancel := ledger.CancelAgentTargetCommand{IdempotencyKey: "cancel:one", AccountID: created.Agent.AccountID,
		AgentID: created.Agent.ID, ConversationID: created.Home.ID, TargetID: sent.Target.ID,
		CanceledBy: "human:toby", CanceledAt: canceledAt}
	canceled, err := repository.CancelAgentTarget(context.Background(), cancel)
	if err != nil {
		t.Fatalf("CancelAgentTarget: %v", err)
	}
	if canceled.State != "canceled" || canceled.BindingRevisionID != sent.Target.BindingRevisionID {
		t.Fatalf("canceled Target = %+v", canceled)
	}
	if replay, err := repository.CancelAgentTarget(context.Background(), cancel); err != nil || replay.State != "canceled" {
		t.Fatalf("cancel replay = %+v, %v", replay, err)
	}
	retried, err := repository.RetryAgentTarget(context.Background(), ledger.RetryAgentTargetCommand{
		IdempotencyKey: "retry:one", AccountID: created.Agent.AccountID, AgentID: created.Agent.ID,
		ConversationID: created.Home.ID, TargetID: sent.Target.ID, RetriedBy: "human:toby",
		RetriedAt: canceledAt.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("RetryAgentTarget: %v", err)
	}
	if retried.State != "queued" || retried.BehaviorRevisionID != sent.Target.BehaviorRevisionID ||
		retried.BindingRevisionID != sent.Target.BindingRevisionID || retried.ParticipantID != sent.Target.ParticipantID ||
		retried.RunID != sent.Target.RunID || retried.AttemptCount != 1 {
		t.Fatalf("retried Target changed original pins = %+v", retried)
	}
}

func TestAgentDirectCommandDigestsIgnoreServerAllocatedIDsAndReceiptTimes(t *testing.T) {
	agent := stableAgentCommand(t)
	first := directAgentTurnCommand(agent, agent.Home.ID, "digest")
	second := first
	second.TurnID, second.ContextManifestID, second.DelegationGrantID = "turn:other", "context:other", "grant:other"
	second.TargetID, second.RunID, second.CreatedAt = "target:other", "run:other", first.CreatedAt.Add(time.Hour)
	firstDigest, err := first.Digest()
	if err != nil {
		t.Fatalf("first Digest: %v", err)
	}
	secondDigest, err := second.Digest()
	if err != nil {
		t.Fatalf("second Digest: %v", err)
	}
	if firstDigest != secondDigest {
		t.Fatalf("server-generated fields changed Send digest: %s != %s", firstDigest, secondDigest)
	}
	retry := ledger.RetryAgentTargetCommand{IdempotencyKey: "retry", AccountID: agent.Agent.AccountID,
		AgentID: agent.Agent.ID, ConversationID: agent.Home.ID, TargetID: "target:one",
		RetriedBy: "human:toby", RetriedAt: first.CreatedAt}
	retryDigest, _ := retry.Digest()
	retry.RetriedAt = retry.RetriedAt.Add(time.Hour)
	replayedDigest, _ := retry.Digest()
	if retryDigest != replayedDigest {
		t.Fatal("server retry receipt time changed idempotency digest")
	}
}

func TestSQLiteExecutionSourceConfigObservationsAreAppendOnlyIdempotentAndLatest(t *testing.T) {
	repository := openAgentChatRepository(t)
	created := stableAgentCommand(t)
	if _, err := repository.CreateAgent(context.Background(), created); err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	initial, err := repository.LatestExecutionSourceConfigObservation(context.Background(),
		created.Agent.AccountID, created.Binding.ExecutionSourceID)
	if err != nil {
		t.Fatalf("LatestExecutionSourceConfigObservation: %v", err)
	}
	if initial.SourceConfigDigest != created.Binding.SourceConfigDigest || initial.Sequence < 1 {
		t.Fatalf("initial source observation = %+v", initial)
	}

	command := ledger.ObserveExecutionSourceConfigCommand{
		IdempotencyKey: "observe:drift", ObservationID: "source-observation:drift",
		AccountID: created.Agent.AccountID, ExecutionSourceID: created.Binding.ExecutionSourceID,
		SourceConfigDigest: strings.Repeat("b", 64), ObservedBy: "worker:studio",
		ObservedAt: created.Agent.CreatedAt.Add(time.Minute),
	}
	observed, err := repository.ObserveExecutionSourceConfig(context.Background(), command)
	if err != nil {
		t.Fatalf("ObserveExecutionSourceConfig: %v", err)
	}
	replay := command
	replay.ObservationID = "source-observation:regenerated"
	replay.ObservedAt = command.ObservedAt.Add(time.Minute)
	replayed, err := repository.ObserveExecutionSourceConfig(context.Background(), replay)
	if err != nil {
		t.Fatalf("ObserveExecutionSourceConfig replay: %v", err)
	}
	if replayed.ID != observed.ID || replayed.Sequence != observed.Sequence || replayed.ObservedAt != observed.ObservedAt {
		t.Fatalf("source observation replay = %+v, want original %+v", replayed, observed)
	}
	latest, err := repository.LatestExecutionSourceConfigObservation(context.Background(),
		created.Agent.AccountID, created.Binding.ExecutionSourceID)
	if err != nil || latest.ID != observed.ID || latest.SourceConfigDigest != command.SourceConfigDigest {
		t.Fatalf("latest source observation = %+v, %v", latest, err)
	}

	send := directAgentTurnCommand(created, created.Home.ID, "observed-drift")
	if _, err := repository.SendAgentTurn(context.Background(), send); !errors.Is(err, ledger.ErrSourceDrift) {
		t.Fatalf("SendAgentTurn after source drift = %v, want source drift", err)
	}
}

func TestExecutionSourceConfigObservationDigestIgnoresServerFields(t *testing.T) {
	command := ledger.ObserveExecutionSourceConfigCommand{
		IdempotencyKey: "observe:one", ObservationID: "observation:one", AccountID: "account:one",
		ExecutionSourceID: "source:one", SourceConfigDigest: strings.Repeat("a", 64),
		ObservedBy: "worker:one", ObservedAt: time.Date(2026, 8, 21, 20, 0, 0, 0, time.UTC),
	}
	first, err := command.Digest()
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	command.ObservationID = "observation:regenerated"
	command.ObservedAt = command.ObservedAt.Add(time.Hour)
	second, err := command.Digest()
	if err != nil {
		t.Fatalf("replay Digest: %v", err)
	}
	if first != second {
		t.Fatalf("server fields changed observation digest: %s != %s", first, second)
	}
}

type agentChatRepository interface {
	ledger.AgentLifecycleRepository
	ledger.AgentDirectChatRepository
	ledger.ExecutionSourceConfigObservationRepository
}

func openAgentChatRepository(t *testing.T) agentChatRepository {
	t.Helper()
	repository := openAgentRepository(t)
	chat, ok := repository.(agentChatRepository)
	if !ok {
		t.Fatal("Store does not implement stable Agent direct chat")
	}
	return chat
}

func directAgentTurnCommand(agent ledger.CreateAgentCommand, conversationID, suffix string) ledger.SendAgentTurnCommand {
	createdAt := agent.Agent.CreatedAt.Add(10 * time.Minute)
	return ledger.SendAgentTurnCommand{
		IdempotencyKey: "send:" + suffix, AccountID: agent.Agent.AccountID, AgentID: agent.Agent.ID,
		ConversationID: conversationID, TurnID: "turn:" + suffix, ClientTurnID: "client-turn:" + suffix,
		ContextManifestID: "context:" + suffix, DelegationGrantID: "grant:" + suffix,
		TargetID: "target:" + suffix, RunID: "run:" + suffix, HumanID: "human:toby",
		Body: "hello " + suffix, CreatedBy: "human:toby", CreatedAt: createdAt,
		HardDeadline: createdAt.Add(10 * time.Minute),
	}
}
