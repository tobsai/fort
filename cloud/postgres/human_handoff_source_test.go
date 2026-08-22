package postgres

import (
	"strings"
	"testing"
	"time"

	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/ledger"
)

func TestDeriveHumanHandoffCausalEvidencePinsInitialAgentResult(t *testing.T) {
	evidence, err := deriveHumanHandoffCausalEvidence(postgresHumanHandoffSource{
		messageID:        41,
		targetID:         "target:researcher:1",
		authorAgentID:    "agent:researcher",
		targetAgentID:    "agent:researcher",
		targetBehaviorID: "behavior:researcher:3",
		targetBindingID:  "binding:researcher:7",
	})
	if err != nil {
		t.Fatal(err)
	}
	if evidence.sourceAgentID != "agent:researcher" ||
		evidence.sourceBehaviorID != "behavior:researcher:3" ||
		evidence.sourceBindingID != "binding:researcher:7" ||
		evidence.parentHandoffID != "" || evidence.depth != 1 ||
		len(evidence.ancestorAgentIDs) != 1 || evidence.ancestorAgentIDs[0] != "agent:researcher" {
		t.Fatalf("causal evidence = %+v", evidence)
	}
}

func TestDeriveHumanHandoffCausalEvidenceExtendsCompletedParent(t *testing.T) {
	now := time.Date(2026, 8, 21, 22, 0, 0, 0, time.UTC)
	parentAuthority := conversation.AuthorityGrant{ID: "effective", Permissions: []string{"read"}}
	parent := ledger.HandoffRecord{
		Handoff: conversation.Handoff{
			ID: "handoff:parent", State: conversation.HandoffCompleted,
			RecipientAgentID: "agent:builder", RecipientBehaviorRevisionID: "behavior:builder:2",
			RecipientBindingRevisionID: "binding:builder:4", GroupTurnID: "turn:group:1",
			RootDelegationGrant: conversation.AuthorityGrant{ID: "grant:group:1", Permissions: []string{"read"}},
			EffectiveAuthority:  parentAuthority,
			MaxAgentMessages:    conversation.MaxGroupAgentMessages, MaxDepth: conversation.MaxGroupHandoffDepth,
			Depth: 2, Deadline: now.Add(time.Hour),
			AncestorAgentIDs: []string{"agent:researcher", "agent:planner"},
		},
		Result: &conversation.HandoffResult{HandoffID: "handoff:parent", MessageID: "57"},
		Target: ledger.HandoffTargetRecord{ID: "target:parent"},
	}
	evidence, err := deriveHumanHandoffCausalEvidence(postgresHumanHandoffSource{
		messageID: 57, targetID: parent.Target.ID, handoffID: parent.Handoff.ID,
		authorAgentID: "agent:builder", parent: &parent,
	})
	if err != nil {
		t.Fatal(err)
	}
	if evidence.sourceAgentID != parent.Handoff.RecipientAgentID ||
		evidence.sourceBehaviorID != parent.Handoff.RecipientBehaviorRevisionID ||
		evidence.sourceBindingID != parent.Handoff.RecipientBindingRevisionID ||
		evidence.parentHandoffID != parent.Handoff.ID || evidence.depth != 3 ||
		evidence.parentStageAuthority == nil || evidence.parentStageAuthority.ID != parentAuthority.ID ||
		evidence.rootDelegationGrant == nil || evidence.rootDelegationGrant.ID != "grant:group:1" ||
		evidence.groupTurnID != "turn:group:1" {
		t.Fatalf("nested causal evidence = %+v", evidence)
	}
	wantAncestors := []string{"agent:researcher", "agent:planner", "agent:builder"}
	if len(evidence.ancestorAgentIDs) != len(wantAncestors) {
		t.Fatalf("ancestors = %v, want %v", evidence.ancestorAgentIDs, wantAncestors)
	}
	for index := range wantAncestors {
		if evidence.ancestorAgentIDs[index] != wantAncestors[index] {
			t.Fatalf("ancestors = %v, want %v", evidence.ancestorAgentIDs, wantAncestors)
		}
	}
}

func TestDeriveHumanHandoffCausalEvidenceRejectsUnattributableAgentMessage(t *testing.T) {
	_, err := deriveHumanHandoffCausalEvidence(postgresHumanHandoffSource{
		messageID: 61, authorAgentID: "agent:orphan",
	})
	if err == nil {
		t.Fatal("accepted an Agent message without immutable target revision evidence")
	}
	if !strings.Contains(err.Error(), "target revision evidence") {
		t.Fatalf("error = %v", err)
	}
}
