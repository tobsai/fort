package conversation_test

import (
	"strings"
	"testing"
	"time"

	"github.com/tobsai/fort/core/conversation"
)

func TestStableAgentRevisionSetPinsBehaviorBindingAndCanonicalHome(t *testing.T) {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	agent := conversation.Agent{
		ID: "agent:researcher", AccountID: "account:one", State: conversation.AgentOpen,
		CurrentProfileRevisionID: "profile:1", CurrentBehaviorRevisionID: "behavior:1",
		CurrentBindingRevisionID: "binding:1", CanonicalConversationID: "conversation:home",
		CreatedAt: now,
	}
	profile := conversation.AgentProfileRevision{
		ID: "profile:1", AgentID: agent.ID, Revision: 1, Name: "Researcher",
		Title: "Market research", CreatedAt: now,
	}
	behavior := conversation.AgentBehaviorRevision{
		ID: "behavior:1", AgentID: agent.ID, Revision: 1, Role: "Researcher",
		StandingInstructions: "Cite primary sources.", EnabledSkills: []string{"research"},
		EnabledTools: []string{"web"}, PromptMaterial: "Use concise evidence.", CreatedAt: now,
	}
	binding := conversation.AgentBindingRevision{
		ID: "binding:1", AgentID: agent.ID, Revision: 1, BehaviorRevisionID: behavior.ID,
		ExecutionSourceID: "source:studio", SourceAgentID: "source-agent:researcher",
		SeatID: "seat:researcher", FortProfile: "openclaw:researcher", Provider: "openclaw",
		RequestedModel: "openclaw-main", ResolvedModel: "openclaw-main", ComputerID: "computer:studio",
		AdapterID: "model.chat.openclaw", AdapterRevision: "adapter:1",
		SourceConfigDigest: strings.Repeat("a", 64), AuthorityID: "authority:chat",
		AuthorityRevision: "authority:1", PolicyID: "policy:chat", PolicyRevision: "policy:1",
		SessionBehavior: "agent_managed", MemoryBehavior: "agent_managed",
		CapabilityEvidence: []string{"text"}, ReadinessContractID: "readiness:chat",
		ReadinessContractRevision: "readiness:1", ActivatedAt: now,
	}
	home := conversation.AgentConversation{
		AgentID: agent.ID, ConversationID: agent.CanonicalConversationID,
		Kind: conversation.AgentConversationCanonical, CreatedAt: now,
	}

	if err := conversation.ValidateAgentRevisionSet(agent, profile, behavior, binding, []conversation.AgentConversation{home}); err != nil {
		t.Fatalf("valid stable Agent revision set: %v", err)
	}

	drifted := binding
	drifted.BehaviorRevisionID = "behavior:2"
	if err := conversation.ValidateAgentRevisionSet(agent, profile, behavior, drifted, []conversation.AgentConversation{home}); err == nil {
		t.Fatal("accepted a binding that did not pin the Agent's current Behavior Revision")
	}

	withoutCapabilities := binding
	withoutCapabilities.CapabilityEvidence = nil
	if err := withoutCapabilities.Validate(); err == nil {
		t.Fatal("accepted a Binding Revision without capability evidence")
	}
}

func TestSourceAgentIdentityIsQualifiedByExecutionSource(t *testing.T) {
	studio := conversation.SourceAgent{
		ID: "source-agent:studio:researcher", ExecutionSourceID: "source:studio",
		OpaqueSourceAgentID: "researcher", DisplayName: "Researcher",
	}
	mini := conversation.SourceAgent{
		ID: "source-agent:mini:researcher", ExecutionSourceID: "source:mini",
		OpaqueSourceAgentID: "researcher", DisplayName: "Researcher",
	}

	studioIdentity, err := studio.Identity()
	if err != nil {
		t.Fatalf("studio identity: %v", err)
	}
	miniIdentity, err := mini.Identity()
	if err != nil {
		t.Fatalf("mini identity: %v", err)
	}
	if studioIdentity == miniIdentity {
		t.Fatal("same-named Source Agents on separate Execution Sources were merged")
	}
}
