package conversation

import (
	"strings"
	"testing"
	"time"
)

func TestAgentChannelIdentityIsCompleteAndIndependentOfPresentation(t *testing.T) {
	binding := validAgentBinding()
	id, err := AgentChannelID(binding)
	if err != nil {
		t.Fatalf("derive valid ID: %v", err)
	}
	channel := AgentChannel{
		ID: id, Name: "OpenClaw — Personal", State: AgentChannelOpen,
		OptionID: "agent-option:v1:personal", Binding: binding,
		CreatedAt: time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC),
	}
	if err := channel.Validate(); err != nil {
		t.Fatalf("validate Channel: %v", err)
	}

	renamed := channel
	renamed.Name = "Renamed"
	renamed.State = AgentChannelArchived
	if renamedID, err := AgentChannelID(renamed.Binding); err != nil || renamedID != id {
		t.Fatalf("presentation changed identity: id=%q err=%v", renamedID, err)
	}

	drifted := binding
	drifted.Authority.ExecutionPolicy = cloneExecutionPolicy(binding.Authority.ExecutionPolicy)
	drifted.Authority.ExecutionPolicy["authority_revision"] = strings.Repeat("d", 64)
	if driftedID, err := AgentChannelID(drifted); err != nil || driftedID == id {
		t.Fatalf("authority drift did not change identity: id=%q err=%v", driftedID, err)
	}

	incomplete := binding
	incomplete.Seat.Machine = ""
	if _, err := AgentChannelID(incomplete); err == nil {
		t.Fatal("incomplete binding produced an Agent Channel ID")
	}

	mismatched := channel
	mismatched.ID = "agent-channel:v1:" + strings.Repeat("0", 64)
	if err := mismatched.Validate(); err == nil {
		t.Fatal("Channel accepted an ID that did not match its binding")
	}
}

func validAgentBinding() AgentBinding {
	return AgentBinding{
		Seat: AgentSeatIdentity{
			ID: "seat:v1:openclaw-personal", Profile: "openclaw:personal", Agent: "openclaw",
			Model: "openclaw-main", Machine: "studio",
		},
		Authority: AgentAuthoritySnapshot{
			RequestedModel: "openclaw-main", ResolvedModel: UnknownProviderIdentity,
			Authority: "agent_chat_v1", PolicyID: "openclaw-chat-v1", PolicyRevision: strings.Repeat("a", 64),
			AdapterID: "model.chat.openclaw", AdapterRevision: strings.Repeat("b", 64),
			RuntimeContract: "openclaw_chat_v1", SessionMode: "agent_managed", MemoryMode: "agent_managed",
			ExecutionPolicy: map[string]string{"authority_revision": strings.Repeat("c", 64)},
		},
	}
}

func cloneExecutionPolicy(source map[string]string) map[string]string {
	copy := make(map[string]string, len(source))
	for key, value := range source {
		copy[key] = value
	}
	return copy
}
