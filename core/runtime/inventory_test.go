package runtime_test

import (
	"context"
	"testing"
	"time"

	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/runtime"
)

func TestAgentSourceInventoryReturnsSourceQualifiedDiscoveryEvidence(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 18, 0, 0, 0, time.UTC)
	want := runtime.AgentSourceInventorySnapshot{
		ExecutionSource: conversation.ExecutionSource{
			ID: "source:studio", AccountID: "account:one", Framework: "openclaw",
			InstanceID: "instance:studio", GatewayID: "gateway:studio", DisplayName: "OpenClaw · Studio",
			ResourceSharing: conversation.ResourceSharingDisclosure{
				ProviderCredentials: conversation.ResourceMachineShared,
				Filesystem:          conversation.ResourceMachineShared,
				BrowserSessions:     conversation.ResourceMachineShared,
				FrameworkSessions:   conversation.ResourceProfileScoped,
				SourceMemory:        conversation.ResourceProfileScoped,
				ToolConfiguration:   conversation.ResourceProfileScoped,
			},
		},
		Agents: []runtime.SourceAgentInventory{{
			SourceAgent: conversation.SourceAgent{
				ID: "source-agent:studio:researcher", ExecutionSourceID: "source:studio",
				OpaqueSourceAgentID: "researcher", DisplayName: "Researcher",
			},
			Capabilities: []string{"text", "web"},
			Readiness: runtime.SourceAgentReadiness{
				Ready: true, ContractID: "readiness:chat", ContractRevision: "readiness:1",
				Evidence: []string{"authenticated", "model-resolved"},
			},
		}},
		ObservedAt: now,
	}
	var inventory runtime.AgentSourceInventory = fakeAgentSourceInventory{snapshot: want}

	got, err := inventory.Inventory(context.Background(), runtime.AgentSourceInventoryRequest{
		ExecutionSourceID: "source:studio",
	})
	if err != nil {
		t.Fatalf("Inventory: %v", err)
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("snapshot validation: %v", err)
	}
	identity, identityErr := got.Agents[0].SourceAgent.Identity()
	if identityErr != nil || got.ExecutionSource.ID != "source:studio" || identity == (conversation.SourceAgentIdentity{}) {
		t.Fatalf("inventory lost exact source identity: %+v", got)
	}
}

func TestAgentSourceInventorySnapshotRejectsCrossSourceAndAmbiguousEvidence(t *testing.T) {
	t.Parallel()

	base := validInventorySnapshot()
	base.Agents[0].SourceAgent.ExecutionSourceID = "source:other"
	if err := base.Validate(); err == nil {
		t.Fatal("snapshot accepted Source Agent from another Execution Source")
	}

	base = validInventorySnapshot()
	base.Agents[0].Capabilities = nil
	if err := base.Validate(); err == nil {
		t.Fatal("snapshot accepted missing capability evidence")
	}

	base = validInventorySnapshot()
	base.Agents[0].Readiness.ContractRevision = ""
	if err := base.Validate(); err == nil {
		t.Fatal("snapshot accepted incomplete readiness evidence")
	}
}

type fakeAgentSourceInventory struct {
	snapshot runtime.AgentSourceInventorySnapshot
}

func (fake fakeAgentSourceInventory) Inventory(context.Context, runtime.AgentSourceInventoryRequest) (runtime.AgentSourceInventorySnapshot, error) {
	return fake.snapshot, nil
}

func validInventorySnapshot() runtime.AgentSourceInventorySnapshot {
	now := time.Date(2026, 8, 21, 18, 0, 0, 0, time.UTC)
	return runtime.AgentSourceInventorySnapshot{
		ExecutionSource: conversation.ExecutionSource{
			ID: "source:studio", AccountID: "account:one", Framework: "openclaw",
			InstanceID: "instance:studio", GatewayID: "gateway:studio", DisplayName: "OpenClaw · Studio",
			ResourceSharing: conversation.ResourceSharingDisclosure{
				ProviderCredentials: conversation.ResourceMachineShared,
				Filesystem:          conversation.ResourceMachineShared, BrowserSessions: conversation.ResourceMachineShared,
				FrameworkSessions: conversation.ResourceProfileScoped, SourceMemory: conversation.ResourceProfileScoped,
				ToolConfiguration: conversation.ResourceProfileScoped,
			},
		},
		Agents: []runtime.SourceAgentInventory{{
			SourceAgent:  conversation.SourceAgent{ID: "source-agent:studio:researcher", ExecutionSourceID: "source:studio", OpaqueSourceAgentID: "researcher", DisplayName: "Researcher"},
			Capabilities: []string{"text"},
			Readiness:    runtime.SourceAgentReadiness{Ready: true, ContractID: "readiness:chat", ContractRevision: "readiness:1", Evidence: []string{"authenticated"}},
		}},
		ObservedAt: now,
	}
}
