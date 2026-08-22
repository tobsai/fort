package ledger_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/ledger"
	"github.com/tobsai/fort/core/store"
)

func TestSQLiteAgentLedgerCreatesOneStableAgentAndCanonicalHomeAtomically(t *testing.T) {
	repository := openAgentRepository(t)
	command := stableAgentCommand(t)

	created, err := repository.CreateAgent(context.Background(), command)
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	if created.Agent.ID != command.Agent.ID || created.Agent.CanonicalConversationID != command.Home.ID {
		t.Fatalf("created stable Agent = %+v", created.Agent)
	}
	if created.Profile.ID != command.Profile.ID || created.Behavior.ID != command.Behavior.ID || created.Binding.ID != command.Binding.ID {
		t.Fatalf("created revisions = profile %+v behavior %+v binding %+v", created.Profile, created.Behavior, created.Binding)
	}
	if created.Link.Kind != conversation.AgentConversationCanonical || created.Home.Title != "Home" {
		t.Fatalf("created canonical Home = link %+v Conversation %+v", created.Link, created.Home)
	}
	if created.Participant.ID != command.Participant.ID || created.Participant.ConversationID != command.Home.ID {
		t.Fatalf("created participant evidence = %+v", created.Participant)
	}

	replayed, err := repository.CreateAgent(context.Background(), command)
	if err != nil {
		t.Fatalf("replay CreateAgent: %v", err)
	}
	if replayed.Agent.ID != created.Agent.ID || replayed.Home.ID != created.Home.ID || replayed.Binding.ID != created.Binding.ID {
		t.Fatalf("idempotent replay changed identity: first %+v replay %+v", created, replayed)
	}

	listed, err := repository.ListAgents(context.Background(), command.Agent.AccountID, conversation.AgentOpen)
	if err != nil {
		t.Fatalf("ListAgents: %v", err)
	}
	if len(listed) != 1 || listed[0].Agent.ID != command.Agent.ID {
		t.Fatalf("listed Agents = %+v", listed)
	}
}

func TestSQLiteAgentLedgerRejectsConflictingReplayAndLeavesNoPartialAgent(t *testing.T) {
	repository := openAgentRepository(t)
	command := stableAgentCommand(t)
	if _, err := repository.CreateAgent(context.Background(), command); err != nil {
		t.Fatalf("initial CreateAgent: %v", err)
	}

	conflict := command
	conflict.Profile.Name = "Different identity"
	if _, err := repository.CreateAgent(context.Background(), conflict); !errors.Is(err, ledger.ErrIdempotencyConflict) {
		t.Fatalf("conflicting replay error = %v, want %v", err, ledger.ErrIdempotencyConflict)
	}

	invalid := stableAgentCommand(t)
	invalid.IdempotencyKey = "create-invalid"
	invalid.Agent.ID = "agent:invalid"
	invalid.Profile.AgentID = invalid.Agent.ID
	invalid.Behavior.AgentID = invalid.Agent.ID
	invalid.Binding.AgentID = invalid.Agent.ID
	invalid.Agent.CurrentProfileRevisionID = invalid.Profile.ID
	invalid.Agent.CurrentBehaviorRevisionID = invalid.Behavior.ID
	invalid.Agent.CurrentBindingRevisionID = invalid.Binding.ID
	invalid.Link.AgentID = invalid.Agent.ID
	invalid.SourceAgent.ExecutionSourceID = "source:foreign"
	if _, err := repository.CreateAgent(context.Background(), invalid); err == nil {
		t.Fatal("CreateAgent accepted mismatched Execution Source")
	}
	if _, err := repository.GetAgent(context.Background(), invalid.Agent.AccountID, invalid.Agent.ID); !errors.Is(err, ledger.ErrNotFound) {
		t.Fatalf("invalid Agent persisted partially: %v", err)
	}
}

func TestSQLiteAgentLedgerReturnsEmptyListsNotNil(t *testing.T) {
	repository := openAgentRepository(t)
	agents, err := repository.ListAgents(context.Background(), "account:none", conversation.AgentOpen)
	if err != nil {
		t.Fatalf("ListAgents: %v", err)
	}
	if agents == nil || len(agents) != 0 {
		t.Fatalf("empty Agents = %#v, want non-nil []", agents)
	}
}

func openAgentRepository(t *testing.T) ledger.AgentRepository {
	t.Helper()
	repository, err := store.Open(filepath.Join(t.TempDir(), "fort.db"))
	if err != nil {
		t.Fatalf("open Store: %v", err)
	}
	t.Cleanup(func() { _ = repository.Close() })
	return repository
}

func stableAgentCommand(t *testing.T) ledger.CreateAgentCommand {
	t.Helper()
	now := time.Date(2026, 8, 21, 16, 0, 0, 0, time.UTC)
	agent := conversation.Agent{
		ID: "agent:researcher", AccountID: "account:one", State: conversation.AgentOpen,
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
		RequestedModel: "openclaw-main", ResolvedModel: "openclaw-main", ComputerID: "computer:studio",
		AdapterID: "model.chat.openclaw", AdapterRevision: "adapter:1", SourceConfigDigest: strings.Repeat("a", 64),
		AuthorityID: "authority:chat", AuthorityRevision: "authority:1", PolicyID: "policy:chat", PolicyRevision: "policy:1",
		SessionBehavior: "agent_managed", MemoryBehavior: "agent_managed", CapabilityEvidence: []string{"text"},
		ReadinessContractID: "readiness:chat", ReadinessContractRevision: "readiness:1", ActivatedAt: now,
	}
	source := conversation.ExecutionSource{
		ID: binding.ExecutionSourceID, AccountID: agent.AccountID, Framework: "openclaw", InstanceID: "instance:studio",
		GatewayID: "gateway:studio", DisplayName: "OpenClaw · Studio",
		ResourceSharing: conversation.ResourceSharingDisclosure{
			ProviderCredentials: conversation.ResourceMachineShared, Filesystem: conversation.ResourceMachineShared,
			BrowserSessions: conversation.ResourceMachineShared, FrameworkSessions: conversation.ResourceProfileScoped,
			SourceMemory: conversation.ResourceProfileScoped, ToolConfiguration: conversation.ResourceProfileScoped,
		},
	}
	sourceAgent := conversation.SourceAgent{
		ID: binding.SourceAgentID, ExecutionSourceID: source.ID, OpaqueSourceAgentID: "researcher", DisplayName: "Researcher",
	}
	home := conversation.Conversation{
		ID: agent.CanonicalConversationID, Title: "Home", State: conversation.ConversationOpen, CreatedAt: now, UpdatedAt: now,
	}
	participant := conversation.Participant{
		ID: "participant:researcher:1", ConversationID: home.ID, SeatID: binding.SeatID,
		Profile: binding.FortProfile, Agent: "openclaw", Model: binding.RequestedModel, Machine: binding.ComputerID,
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
