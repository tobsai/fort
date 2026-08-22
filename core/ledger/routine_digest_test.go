package ledger

import (
	"testing"
	"time"

	"github.com/tobsai/fort/core/conversation"
)

func TestRoutineOwnerCommandDigestsExcludeOnlyServerAllocation(t *testing.T) {
	t.Parallel()
	base := digestRoutineCommand()
	reallocated := base
	reallocated.Routine.ID = "routine:server-two"
	reallocated.Routine.CurrentRevisionID = "revision:server-two"
	reallocated.Routine.CreatedAt = base.Routine.CreatedAt.Add(time.Hour)
	reallocated.Revision.ID = reallocated.Routine.CurrentRevisionID
	reallocated.Revision.RoutineID = reallocated.Routine.ID
	reallocated.Revision.BehaviorRevisionID = "behavior:new-current"
	reallocated.Revision.BindingRevisionID = "binding:new-current"
	reallocated.Revision.CreatedAt = reallocated.Routine.CreatedAt
	first, err := base.Digest()
	if err != nil {
		t.Fatal(err)
	}
	second, err := reallocated.Digest()
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("server allocation changed create digest: %s != %s", first, second)
	}
	semanticChange := base
	semanticChange.Revision.ExpectedResult = "Different result"
	changed, _ := semanticChange.Digest()
	if changed == first {
		t.Fatal("semantic Routine change did not change create digest")
	}

	testOccurrence := EnqueueRoutineOccurrenceCommand{AccountID: "account", RoutineID: "routine",
		RoutineRevisionID: "revision:one", OccurrenceID: "occurrence:one", RunID: "run:one",
		Kind: conversation.RoutineRunTest, ScheduledFor: base.Routine.CreatedAt,
		IdempotencyKey: "test", ApprovalEvidenceID: "approval:test", CreatedAt: base.Routine.CreatedAt}
	replayedTest := testOccurrence
	replayedTest.RoutineRevisionID = "revision:current-later"
	replayedTest.OccurrenceID = "occurrence:two"
	replayedTest.RunID = "run:two"
	replayedTest.ScheduledFor = testOccurrence.ScheduledFor.Add(time.Hour)
	replayedTest.CreatedAt = replayedTest.ScheduledFor
	testDigest, _ := testOccurrence.Digest()
	replayDigest, _ := replayedTest.Digest()
	if testDigest != replayDigest {
		t.Fatalf("server Test allocation changed occurrence digest: %s != %s", testDigest, replayDigest)
	}
	scheduled := testOccurrence
	scheduled.Kind = conversation.RoutineRunScheduled
	scheduledLater := scheduled
	scheduledLater.ScheduledFor = scheduled.ScheduledFor.Add(time.Hour)
	scheduledDigest, _ := scheduled.Digest()
	scheduledLaterDigest, _ := scheduledLater.Digest()
	if scheduledDigest == scheduledLaterDigest {
		t.Fatal("scheduled occurrence time did not change digest")
	}
}

func digestRoutineCommand() CreateRoutineCommand {
	now := time.Date(2026, 8, 21, 20, 0, 0, 0, time.UTC)
	return CreateRoutineCommand{IdempotencyKey: "routine:create", Routine: conversation.Routine{
		ID: "routine:server-one", AccountID: "account", AgentID: "agent",
		CurrentRevisionID: "revision:server-one", State: conversation.RoutineActive, CreatedAt: now,
	}, Revision: conversation.RoutineRevision{
		ID: "revision:server-one", RoutineID: "routine:server-one", Revision: 1, AgentID: "agent",
		BehaviorRevisionID: "behavior:current", BindingRevisionID: "binding:current",
		Authority: conversation.RoutineAuthorityFortCloud, Trigger: conversation.RoutineTriggerSchedule,
		Schedule: "0 0 9 * * 1", Timezone: "America/Chicago", NextOccurrence: now.Add(24 * time.Hour),
		InputSource: "source", FreshnessSeconds: 60, ExpectedResult: "Result",
		ResultConversationID: "conversation", ApprovalBoundary: "none", MissingInputBehavior: "skip",
		RetryPolicy: "once", CatchUpPolicy: "skip", LatenessPolicy: "within_90s", CreatedAt: now,
	}}
}
