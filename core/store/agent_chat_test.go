package store

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/ledger"
)

func TestAgentDirectSendFailsClosedOnLatestSourceObservationDrift(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "fort.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer store.Close()
	command := sqliteDirectChatAgentFixture()
	if _, err := store.CreateAgent(context.Background(), command); err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	if _, err := store.ObserveExecutionSourceConfig(context.Background(), ledger.ObserveExecutionSourceConfigCommand{
		IdempotencyKey: "observe:drift", ObservationID: "source-observation:drift",
		AccountID: command.Agent.AccountID, ExecutionSourceID: command.Binding.ExecutionSourceID,
		SourceConfigDigest: strings.Repeat("b", 64), ObservedBy: "worker:drift",
		ObservedAt: command.Agent.CreatedAt.Add(time.Minute),
	}); err != nil {
		t.Fatalf("append drift observation: %v", err)
	}
	createdAt := command.Agent.CreatedAt.Add(2 * time.Minute)
	_, err = store.SendAgentTurn(context.Background(), ledger.SendAgentTurnCommand{
		IdempotencyKey: "send:drift", AccountID: command.Agent.AccountID, AgentID: command.Agent.ID,
		ConversationID: command.Home.ID, TurnID: "turn:drift", ClientTurnID: "client:drift",
		ContextManifestID: "context:drift", DelegationGrantID: "grant:drift", TargetID: "target:drift",
		RunID: "run:drift", HumanID: "human:toby", Body: "must not dispatch", CreatedBy: "human:toby",
		CreatedAt: createdAt, HardDeadline: createdAt.Add(time.Minute),
	})
	if !errors.Is(err, ledger.ErrSourceDrift) {
		t.Fatalf("SendAgentTurn error = %v, want source drift", err)
	}
	var messages, turns, targets int
	if err := store.db.QueryRow(`SELECT
  (SELECT COUNT(*) FROM conversation_message),
  (SELECT COUNT(*) FROM conversation_turn),
  (SELECT COUNT(*) FROM conversation_target)`).Scan(&messages, &turns, &targets); err != nil {
		t.Fatalf("count direct records: %v", err)
	}
	if messages != 0 || turns != 0 || targets != 0 {
		t.Fatalf("drift persisted message/turn/target = %d/%d/%d", messages, turns, targets)
	}
}

func sqliteDirectChatAgentFixture() ledger.CreateAgentCommand {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	agent := conversation.Agent{ID: "agent:drift", AccountID: "account:drift", State: conversation.AgentOpen,
		CurrentProfileRevisionID: "profile:drift:1", CurrentBehaviorRevisionID: "behavior:drift:1",
		CurrentBindingRevisionID: "binding:drift:1", CanonicalConversationID: "conversation:drift:home", CreatedAt: now}
	profile := conversation.AgentProfileRevision{ID: agent.CurrentProfileRevisionID, AgentID: agent.ID,
		Revision: 1, Name: "Drift", Title: "Drift Agent", CreatedAt: now}
	behavior := conversation.AgentBehaviorRevision{ID: agent.CurrentBehaviorRevisionID, AgentID: agent.ID,
		Revision: 1, Role: "Chat", StandingInstructions: "Answer.", EnabledSkills: []string{"chat"},
		EnabledTools: []string{"text"}, CreatedAt: now}
	binding := conversation.AgentBindingRevision{ID: agent.CurrentBindingRevisionID, AgentID: agent.ID,
		Revision: 1, BehaviorRevisionID: behavior.ID, ExecutionSourceID: "source:drift",
		SourceAgentID: "source-agent:drift", SeatID: "seat:drift", FortProfile: "openclaw:drift",
		Provider: "openclaw", RequestedModel: "main", ResolvedModel: "main", ComputerID: "computer:drift",
		AdapterID: "model.chat.openclaw", AdapterRevision: "adapter:1", SourceConfigDigest: strings.Repeat("a", 64),
		AuthorityID: "authority:chat", AuthorityRevision: "1", PolicyID: "policy:chat", PolicyRevision: "1",
		SessionBehavior: "agent_managed", MemoryBehavior: "agent_managed", CapabilityEvidence: []string{"text"},
		ReadinessContractID: "ready:chat", ReadinessContractRevision: "1", ActivatedAt: now}
	source := conversation.ExecutionSource{ID: binding.ExecutionSourceID, AccountID: agent.AccountID,
		Framework: "openclaw", InstanceID: "instance:drift", GatewayID: "gateway:drift", DisplayName: "Drift Source",
		ResourceSharing: conversation.ResourceSharingDisclosure{ProviderCredentials: conversation.ResourceMachineShared,
			Filesystem: conversation.ResourceMachineShared, BrowserSessions: conversation.ResourceMachineShared,
			FrameworkSessions: conversation.ResourceProfileScoped, SourceMemory: conversation.ResourceProfileScoped,
			ToolConfiguration: conversation.ResourceProfileScoped}}
	sourceAgent := conversation.SourceAgent{ID: binding.SourceAgentID, ExecutionSourceID: source.ID,
		OpaqueSourceAgentID: "drift", DisplayName: "Drift"}
	home := conversation.Conversation{ID: agent.CanonicalConversationID, Title: "Home",
		State: conversation.ConversationOpen, CreatedAt: now, UpdatedAt: now}
	participant := conversation.Participant{ID: "participant:drift:1", ConversationID: home.ID,
		SeatID: binding.SeatID, Profile: binding.FortProfile, Agent: binding.Provider, Model: binding.RequestedModel,
		Machine: binding.ComputerID, DisplayName: profile.Name, State: conversation.ParticipantActive, CreatedAt: now}
	return ledger.CreateAgentCommand{IdempotencyKey: "create:drift", Agent: agent, Profile: profile,
		Behavior: behavior, Binding: binding, ExecutionSource: source, SourceAgent: sourceAgent, Home: home,
		Participant: participant, Link: conversation.AgentConversation{AgentID: agent.ID, ConversationID: home.ID,
			Kind: conversation.AgentConversationCanonical, CreatedAt: now}}
}
