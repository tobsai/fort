package ledger_test

import (
	"testing"
	"time"

	"github.com/tobsai/fort/core/ledger"
)

func TestCreateHumanHandoffDigestExcludesServerAllocatedEvidence(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 21, 20, 0, 0, 0, time.UTC)
	command := ledger.CreateHumanHandoffCommand{
		IdempotencyKey: "handoff:create:one", AccountID: "account:one",
		SourceConversationID: "conversation:source", SourceMessageID: "41",
		RecipientAgentID: "agent:recipient", ContextMessageIDs: []string{"39", "41"},
		RequestedResult: "Review the launch evidence.", ReplyToMessageID: "41",
		HardDeadline: now.Add(10 * time.Minute), HandoffID: "handoff:one",
		TargetID: "target:one", RootDelegationGrantID: "grant:one",
		CreatedByID: "human:account:one", CreatedAt: now,
	}
	want, err := command.Digest()
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	replay := command
	replay.HandoffID = "handoff:other-allocation"
	replay.TargetID = "target:other-allocation"
	replay.RootDelegationGrantID = "grant:other-allocation"
	replay.CreatedByID = "human:another-server-format"
	replay.CreatedAt = now.Add(time.Hour)
	got, err := replay.Digest()
	if err != nil || got != want {
		t.Fatalf("server-derived fields changed digest: %q != %q, err=%v", got, want, err)
	}
	replay.RequestedResult = "A different request."
	if conflict, _ := replay.Digest(); conflict == want {
		t.Fatal("client-visible requested result did not change digest")
	}
}

func TestCancelHandoffDigestExcludesServerReceiptEvidence(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 21, 20, 0, 0, 0, time.UTC)
	command := ledger.CancelHandoffCommand{
		IdempotencyKey: "handoff:cancel:one", AccountID: "account:one",
		HandoffID: "handoff:one", CanceledBy: "human:account:one", CanceledAt: now,
	}
	want, err := command.Digest()
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	replay := command
	replay.CanceledBy = "human:another-server-format"
	replay.CanceledAt = now.Add(time.Hour)
	got, err := replay.Digest()
	if err != nil || got != want {
		t.Fatalf("server cancellation evidence changed digest: %q != %q, err=%v", got, want, err)
	}
	replay.HandoffID = "handoff:other"
	if conflict, _ := replay.Digest(); conflict == want {
		t.Fatal("Handoff identity did not change cancel digest")
	}
}
