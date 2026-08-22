package conversation_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/tobsai/fort/core/conversation"
)

func TestHandoffAuthorityIsTheIntersectionOfEveryDelegationLayer(t *testing.T) {
	layers := []conversation.AuthorityGrant{
		{ID: "grant:human", Permissions: []string{"read", "write", "browser"}},
		{ID: "grant:parent", Permissions: []string{"read", "write"}},
		{ID: "policy:handoff", Permissions: []string{"read"}},
		{ID: "policy:recipient", Permissions: []string{"read", "admin"}},
		{ID: "emitter:request", Permissions: []string{"read", "browser"}},
		{ID: "approval:receipt", Permissions: []string{"read", "write"}},
	}

	effective, err := conversation.ComputeEffectiveAuthority([]string{"read"}, layers...)
	if err != nil {
		t.Fatalf("compute effective authority: %v", err)
	}
	if !reflect.DeepEqual(effective.Permissions, []string{"read"}) {
		t.Fatalf("effective permissions = %#v, want [read]", effective.Permissions)
	}
	if _, err := conversation.ComputeEffectiveAuthority([]string{"admin"}, layers...); err == nil {
		t.Fatal("recipient standing authority escalated the initiating delegation grant")
	}
}

func TestHandoffEffectiveAuthorityCannotExceedTheExplicitRequest(t *testing.T) {
	layers := []conversation.AuthorityGrant{
		{ID: "grant:human", Permissions: []string{"read", "write"}},
		{ID: "policy:handoff", Permissions: []string{"read", "write"}},
	}

	effective, err := conversation.ComputeEffectiveAuthority([]string{"read"}, layers...)
	if err != nil {
		t.Fatalf("compute effective authority: %v", err)
	}
	if !reflect.DeepEqual(effective.Permissions, []string{"read"}) {
		t.Fatalf("effective permissions = %#v, want only the explicitly requested permission", effective.Permissions)
	}
}

func TestHandoffStartEnforcesApprovalMessageDeadlineAndBudgetEvidence(t *testing.T) {
	handoff := validHandoff(t)
	if err := handoff.CanStart(handoff.CreatedAt.Add(time.Second), conversation.MaxGroupAgentMessages-1); err != nil {
		t.Fatalf("eligible Handoff start: %v", err)
	}
	if err := handoff.CanStart(handoff.CreatedAt.Add(time.Second), conversation.MaxGroupAgentMessages); !errors.Is(err, conversation.ErrHandoffNeedsYou) {
		t.Fatalf("message exhaustion error = %v", err)
	}
	if err := handoff.CanStart(handoff.Deadline, 0); !errors.Is(err, conversation.ErrHandoffNeedsYou) {
		t.Fatalf("deadline exhaustion error = %v", err)
	}

	approvalRequired := handoff
	approvalRequired.ApprovalRequired = true
	if err := approvalRequired.CanStart(handoff.CreatedAt.Add(time.Second), 0); !errors.Is(err, conversation.ErrHandoffNeedsYou) {
		t.Fatalf("missing approval error = %v", err)
	}

	hardBudget := handoff
	hardBudget.BudgetClass = conversation.LimitHard
	if err := hardBudget.Validate(); err == nil {
		t.Fatal("accepted a hard Handoff budget without enforceability evidence")
	}
	hardBudget.BudgetLimitEvidenceID = "evidence:adapter-budget-cap"
	if err := hardBudget.Validate(); err != nil {
		t.Fatalf("hard Handoff budget with evidence: %v", err)
	}
}

func TestHandoffPinsOneOutputAndCreatesReferenceOnlyOtherProjections(t *testing.T) {
	handoff := validHandoff(t)
	if err := handoff.Validate(); err != nil {
		t.Fatalf("valid Handoff: %v", err)
	}
	withoutParentAuthority := handoff
	withoutParentAuthority.ParentStageAuthority = nil
	if err := withoutParentAuthority.Validate(); err == nil {
		t.Fatal("Agent-initiated Handoff omitted the source stage's delegated authority")
	}

	result := conversation.HandoffResult{
		HandoffID: handoff.ID, OutputConversationID: handoff.OutputConversationID,
		MessageID: "message:result", Body: "The requested result.",
	}
	if err := result.ValidateFor(handoff); err == nil {
		t.Fatal("queued Handoff created an authoritative result message")
	}
	handoff.State = conversation.HandoffCompleted
	if err := result.ValidateFor(handoff); err != nil {
		t.Fatalf("valid authoritative Handoff result: %v", err)
	}
	projection, err := handoff.ReferenceProjection(handoff.SourceConversationID, result)
	if err != nil {
		t.Fatalf("reference projection: %v", err)
	}
	if projection.ConversationID != handoff.SourceConversationID || projection.AuthoritativeMessageID != result.MessageID || projection.OutputConversationID != handoff.OutputConversationID {
		t.Fatalf("reference projection = %#v", projection)
	}

	wrongOutput := result
	wrongOutput.OutputConversationID = handoff.SourceConversationID
	if err := wrongOutput.ValidateFor(handoff); err == nil {
		t.Fatal("accepted an authoritative result in a Conversation not pinned by the Handoff")
	}

	self := handoff
	self.RecipientAgentID = handoff.SourceAgentID
	if err := self.Validate(); err == nil {
		t.Fatal("accepted a self-Handoff")
	}
}

func TestHandoffContextManifestAcceptsOnlyGrantedImmutableFortRecords(t *testing.T) {
	digest := strings.Repeat("a", 64)
	manifest := conversation.ContextManifest{References: []conversation.ContextReference{
		{Kind: conversation.ContextMessage, ID: "message:1", AccountID: "account:one", Immutable: true},
		{Kind: conversation.ContextArtifact, ID: "artifact:1", AccountID: "account:one", Immutable: true, Finalized: true, Digest: digest, ObservedDigest: digest, Size: 42, ObservedSize: 42},
	}}
	grant := conversation.AuthorityGrant{
		ID: "grant:human", Permissions: []string{"read"},
		ContextRecordIDs: []string{"message:message:1", "context_artifact:artifact:1"},
	}

	if err := conversation.ValidateContextManifest("account:one", grant, manifest); err != nil {
		t.Fatalf("valid context manifest: %v", err)
	}

	localPath := manifest
	localPath.References = append([]conversation.ContextReference(nil), manifest.References...)
	localPath.References[0].Kind = conversation.ContextReferenceKind("local_path")
	if err := conversation.ValidateContextManifest("account:one", grant, localPath); err == nil {
		t.Fatal("accepted a local path in Handoff context")
	}

	tampered := manifest
	tampered.References = append([]conversation.ContextReference(nil), manifest.References...)
	tampered.References[1].ObservedDigest = strings.Repeat("b", 64)
	if err := conversation.ValidateContextManifest("account:one", grant, tampered); err == nil {
		t.Fatal("accepted an artifact whose persisted digest did not match")
	}
}

func validHandoff(t *testing.T) conversation.Handoff {
	t.Helper()
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	root := conversation.AuthorityGrant{
		ID: "grant:human", Permissions: []string{"read", "write"},
		ContextRecordIDs: []string{"message:message:source"},
	}
	parent := conversation.AuthorityGrant{ID: "stage:source", Permissions: []string{"read", "write"}}
	policy := conversation.AuthorityGrant{ID: "policy:handoff", Permissions: []string{"read"}}
	recipient := conversation.AuthorityGrant{ID: "policy:recipient", Permissions: []string{"read", "admin"}}
	emitter := conversation.AuthorityGrant{ID: "emitter:request", Permissions: []string{"read"}}
	effective, err := conversation.ComputeEffectiveAuthority([]string{"read"}, root, parent, policy, recipient, emitter)
	if err != nil {
		t.Fatalf("effective authority fixture: %v", err)
	}
	return conversation.Handoff{
		ID: "handoff:1", AccountID: "account:one", IdempotencyKey: "handoff-command:1",
		State: conversation.HandoffQueued, CreatedByKind: conversation.HandoffActorAgent, CreatedByID: "agent:sender",
		SourceMessageID: "message:source", SourceAgentID: "agent:sender",
		SourceBehaviorRevisionID: "behavior:sender:1", SourceBindingRevisionID: "binding:sender:1",
		RecipientAgentID: "agent:recipient", RecipientBehaviorRevisionID: "behavior:recipient:2",
		RecipientBindingRevisionID: "binding:recipient:2", SourceConversationID: "conversation:sender",
		OutputConversationID: "conversation:recipient-home", RequestedResult: "Review the evidence.",
		Context: conversation.ContextManifest{References: []conversation.ContextReference{
			{Kind: conversation.ContextMessage, ID: "message:source", AccountID: "account:one", Immutable: true},
		}},
		RootDelegationGrant: root, ParentStageAuthority: &parent, HandoffPolicy: policy,
		RecipientBindingPolicy: recipient, EmitterRequest: &emitter,
		RequestedAuthority: []string{"read"}, EffectiveAuthority: effective,
		StructuredEmitterID: "emitter:openclaw:1", BudgetClass: conversation.LimitUnknown,
		MaxAgentMessages: conversation.MaxGroupAgentMessages, MaxDepth: conversation.MaxGroupHandoffDepth,
		Depth: 1, Deadline: now.Add(10 * time.Minute), AncestorAgentIDs: []string{"agent:sender"}, CreatedAt: now,
	}
}
