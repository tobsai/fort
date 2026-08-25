package channelturn_test

import (
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/tobsai/fort/core/channelturn"
	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/ledger"
	"github.com/tobsai/fort/core/store"
)

func TestSubmitIsDurablyAcceptedReplayableAndIdempotent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	databasePath := filepath.Join(t.TempDir(), "fort.db")
	firstStore, err := store.Open(databasePath)
	if err != nil {
		t.Fatalf("open Store: %v", err)
	}
	agent := relayAgentFixture()
	if _, err := firstStore.CreateAgent(ctx, agent); err != nil {
		t.Fatalf("create exact relay binding: %v", err)
	}

	var turns channelturn.Module = firstStore.ChannelTurns()
	receipt, err := turns.Submit(ctx, agent.Binding.ID, "client-attempt:one", "hello Hermes")
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if receipt.AttemptID == "" || receipt.BindingID != agent.Binding.ID ||
		receipt.ConversationID != agent.Home.ID || receipt.ClientAttemptID != "client-attempt:one" ||
		receipt.State != channelturn.StateAccepted || receipt.AcceptedSequence != 1 || receipt.AcceptedAt.IsZero() {
		t.Fatalf("acceptance receipt = %+v", receipt)
	}

	accepted, err := turns.Events(ctx, receipt.AttemptID, 0)
	if err != nil {
		t.Fatalf("events after 0: %v", err)
	}
	if accepted.AttemptID != receipt.AttemptID || accepted.AfterSequence != 0 || len(accepted.Events) != 1 {
		t.Fatalf("accepted event stream = %+v", accepted)
	}
	wantEvent := channelturn.Event{
		ID: receipt.AttemptID + ":1", AttemptID: receipt.AttemptID,
		ConversationID: agent.Home.ID, BindingID: agent.Binding.ID, ClientAttemptID: "client-attempt:one",
		Sequence: 1, Type: channelturn.EventAccepted, ProtocolRevision: agent.Binding.AdapterRevision,
		CreatedAt: receipt.AcceptedAt,
	}
	if !reflect.DeepEqual(accepted.Events[0], wantEvent) {
		t.Fatalf("accepted event = %+v, want %+v", accepted.Events[0], wantEvent)
	}

	if err := firstStore.Close(); err != nil {
		t.Fatalf("close first Store: %v", err)
	}
	reopenedStore, err := store.Open(databasePath)
	if err != nil {
		t.Fatalf("reopen Store: %v", err)
	}
	t.Cleanup(func() { _ = reopenedStore.Close() })
	turns = reopenedStore.ChannelTurns()

	replayed, err := turns.Events(ctx, receipt.AttemptID, 0)
	if err != nil {
		t.Fatalf("replay events after reopen: %v", err)
	}
	if !reflect.DeepEqual(replayed, accepted) {
		t.Fatalf("replayed stream = %+v, want %+v", replayed, accepted)
	}

	duplicate, err := turns.Submit(ctx, agent.Binding.ID, "client-attempt:one", "hello Hermes")
	if err != nil {
		t.Fatalf("idempotent submit: %v", err)
	}
	if !reflect.DeepEqual(duplicate, receipt) {
		t.Fatalf("idempotent receipt = %+v, want %+v", duplicate, receipt)
	}
	afterAccepted, err := turns.Events(ctx, receipt.AttemptID, receipt.AcceptedSequence)
	if err != nil {
		t.Fatalf("events after accepted cursor: %v", err)
	}
	if len(afterAccepted.Events) != 0 {
		t.Fatalf("duplicate submit appended events: %+v", afterAccepted.Events)
	}
}

func relayAgentFixture() ledger.CreateAgentCommand {
	now := time.Date(2026, 8, 23, 19, 0, 0, 0, time.UTC)
	agent := conversation.Agent{
		ID: "agent:hermes", AccountID: "account:one", State: conversation.AgentOpen,
		CurrentProfileRevisionID: "profile:hermes:1", CurrentBehaviorRevisionID: "behavior:hermes:1",
		CurrentBindingRevisionID: "binding:hermes:relay:1", CanonicalConversationID: "conversation:hermes:home",
		CreatedAt: now,
	}
	profile := conversation.AgentProfileRevision{
		ID: agent.CurrentProfileRevisionID, AgentID: agent.ID, Revision: 1,
		Name: "Hermes", Title: "Hermes Agent", Pinned: true, CreatedAt: now,
	}
	behavior := conversation.AgentBehaviorRevision{
		ID: agent.CurrentBehaviorRevisionID, AgentID: agent.ID, Revision: 1,
		Role: "Assistant", StandingInstructions: "Answer through Hermes.", EnabledSkills: []string{"chat"},
		EnabledTools: []string{"hermes-managed"}, CreatedAt: now,
	}
	binding := conversation.AgentBindingRevision{
		ID: agent.CurrentBindingRevisionID, AgentID: agent.ID, Revision: 1, BehaviorRevisionID: behavior.ID,
		ExecutionSourceID: "source:hermes:studio", SourceAgentID: "source-agent:hermes:default",
		SeatID: "seat:hermes", FortProfile: "hermes:default", Provider: "openai-codex",
		RequestedModel: "gpt-5.6-luna", ResolvedModel: "gpt-5.6-luna", ComputerID: "computer:studio",
		AdapterID: channelturn.TransportHermesPlatformRelayV1, AdapterRevision: "protocol:1",
		SourceConfigDigest: strings.Repeat("a", 64), AuthorityID: "authority:hermes-relay",
		AuthorityRevision: "authority:1", PolicyID: "policy:hermes-relay", PolicyRevision: "policy:1",
		SessionBehavior: "source_managed", MemoryBehavior: "source_managed", CapabilityEvidence: []string{"text"},
		ReadinessContractID: "readiness:hermes-relay", ReadinessContractRevision: "readiness:1", ActivatedAt: now,
	}
	source := conversation.ExecutionSource{
		ID: binding.ExecutionSourceID, AccountID: agent.AccountID, Framework: "hermes",
		InstanceID: "instance:hermes:studio", GatewayID: "gateway:studio", DisplayName: "Hermes · Studio",
		ResourceSharing: conversation.ResourceSharingDisclosure{
			ProviderCredentials: conversation.ResourceProfileScoped, Filesystem: conversation.ResourceMachineShared,
			BrowserSessions: conversation.ResourceMachineShared, FrameworkSessions: conversation.ResourceProfileScoped,
			SourceMemory: conversation.ResourceProfileScoped, ToolConfiguration: conversation.ResourceProfileScoped,
		},
	}
	sourceAgent := conversation.SourceAgent{
		ID: binding.SourceAgentID, ExecutionSourceID: source.ID,
		OpaqueSourceAgentID: "default", DisplayName: "Hermes",
	}
	home := conversation.Conversation{
		ID: agent.CanonicalConversationID, Title: "Home", State: conversation.ConversationOpen,
		CreatedAt: now, UpdatedAt: now,
	}
	participant := conversation.Participant{
		ID: "participant:hermes:1", ConversationID: home.ID, SeatID: binding.SeatID,
		Profile: binding.FortProfile, Agent: binding.Provider, Model: binding.RequestedModel, Machine: binding.ComputerID,
		DisplayName: profile.Name, Position: 0, State: conversation.ParticipantActive, CreatedAt: now,
	}
	return ledger.CreateAgentCommand{
		IdempotencyKey: "create:hermes", Agent: agent, Profile: profile, Behavior: behavior,
		Binding: binding, ExecutionSource: source, SourceAgent: sourceAgent, Home: home, Participant: participant,
		Link: conversation.AgentConversation{
			AgentID: agent.ID, ConversationID: home.ID, Kind: conversation.AgentConversationCanonical, CreatedAt: now,
		},
	}
}
