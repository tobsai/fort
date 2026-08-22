package conversation_test

import (
	"testing"
	"time"

	"github.com/tobsai/fort/core/conversation"
)

func TestRoutineIsAgentOwnedFortCloudWorkWithNormalizedSuccessOnlyOutput(t *testing.T) {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	routine := conversation.Routine{
		ID: "routine:weekly", AccountID: "account:one", AgentID: "agent:researcher",
		CurrentRevisionID: "routine-revision:1", State: conversation.RoutineActive, CreatedAt: now,
	}
	revision := conversation.RoutineRevision{
		ID: "routine-revision:1", RoutineID: routine.ID, Revision: 1,
		AgentID: routine.AgentID, BehaviorRevisionID: "behavior:researcher:1", BindingRevisionID: "binding:researcher:1",
		Authority: conversation.RoutineAuthorityFortCloud, Trigger: conversation.RoutineTriggerSchedule,
		Schedule: "0 0 9 * * 1", Timezone: "America/Chicago", NextOccurrence: now.Add(7 * 24 * time.Hour),
		InputSource: "fort:conversation:research", FreshnessSeconds: 86_400,
		ExpectedResult: "Weekly research brief", ResultConversationID: "conversation:researcher-home",
		ApprovalBoundary: "before_external_side_effect", MissingInputBehavior: "needs_you",
		RetryPolicy: "once", CatchUpPolicy: "skip", LatenessPolicy: "within_90s", CreatedAt: now,
	}
	if err := routine.Validate(revision); err != nil {
		t.Fatalf("valid fort_cloud Routine: %v", err)
	}

	run := conversation.RoutineRun{
		ID: "routine-run:1", RoutineID: routine.ID, RoutineRevisionID: revision.ID,
		AgentID: routine.AgentID, BehaviorRevisionID: revision.BehaviorRevisionID,
		BindingRevisionID: revision.BindingRevisionID, OccurrenceID: "occurrence:2026-08-24T09:00:00-05:00",
		Kind: conversation.RoutineRunTest, State: conversation.RoutineRunQueued, CreatedAt: now,
	}
	if err := run.ValidateFor(routine, revision); err != nil {
		t.Fatalf("valid test occurrence: %v", err)
	}

	succeeded := run
	succeeded.State = conversation.RoutineRunSucceeded
	succeeded.NormalizedResult = "Primary-source brief."
	succeeded.ResultMessageID = "message:routine-result"
	if err := succeeded.ValidateFor(routine, revision); err != nil {
		t.Fatalf("valid normalized result: %v", err)
	}

	failedWithMessage := run
	failedWithMessage.State = conversation.RoutineRunFailed
	failedWithMessage.NormalizedResult = "partial output"
	failedWithMessage.ResultMessageID = "message:leak"
	if err := failedWithMessage.ValidateFor(routine, revision); err == nil {
		t.Fatal("failed Routine run created a Conversation message")
	}

	sourceAuthoritative := revision
	sourceAuthoritative.Authority = conversation.RoutineAuthority("source_native")
	if err := routine.Validate(sourceAuthoritative); err == nil {
		t.Fatal("accepted executable source-authoritative Routine in the first cloud release")
	}
}

func TestScheduledRoutineRevisionRequiresExactSixFieldCronAndTimezone(t *testing.T) {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	valid := conversation.RoutineRevision{
		ID: "routine-revision:1", RoutineID: "routine:weekly", Revision: 1,
		AgentID: "agent:researcher", BehaviorRevisionID: "behavior:researcher:1",
		BindingRevisionID: "binding:researcher:1", Authority: conversation.RoutineAuthorityFortCloud,
		Trigger: conversation.RoutineTriggerSchedule, Schedule: "0 0 9 * * 1", Timezone: "America/Chicago",
		NextOccurrence: now.Add(7 * 24 * time.Hour), InputSource: "fort:conversation:research",
		FreshnessSeconds: 86_400, ExpectedResult: "Weekly research brief",
		ResultConversationID: "conversation:researcher-home", ApprovalBoundary: "none",
		MissingInputBehavior: "needs_you", RetryPolicy: "once", CatchUpPolicy: "skip",
		LatenessPolicy: "within_90s", CreatedAt: now,
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid six-field schedule: %v", err)
	}
	for name, mutate := range map[string]func(*conversation.RoutineRevision){
		"five fields":        func(revision *conversation.RoutineRevision) { revision.Schedule = "0 9 * * 1" },
		"invalid cron":       func(revision *conversation.RoutineRevision) { revision.Schedule = "0 0 25 * * *" },
		"noncanonical space": func(revision *conversation.RoutineRevision) { revision.Schedule = "0  0 9 * * 1" },
		"unknown timezone":   func(revision *conversation.RoutineRevision) { revision.Timezone = "Mars/Olympus" },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := valid
			mutate(&candidate)
			if err := candidate.Validate(); err == nil {
				t.Fatalf("accepted invalid scheduled Routine Revision: %+v", candidate)
			}
		})
	}
}

func TestRoutineRevisionRejectsUnsupportedFirstReleasePolicyValues(t *testing.T) {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	valid := conversation.RoutineRevision{
		ID: "routine-revision:1", RoutineID: "routine:weekly", Revision: 1,
		AgentID: "agent:researcher", BehaviorRevisionID: "behavior:researcher:1",
		BindingRevisionID: "binding:researcher:1", Authority: conversation.RoutineAuthorityFortCloud,
		Trigger: conversation.RoutineTriggerSchedule, Schedule: "0 0 9 * * 1", Timezone: "America/Chicago",
		NextOccurrence: now.Add(7 * 24 * time.Hour), InputSource: "fort:conversation:research",
		FreshnessSeconds: 86_400, ExpectedResult: "Weekly research brief",
		ResultConversationID: "conversation:researcher-home", ApprovalBoundary: "none",
		MissingInputBehavior: "needs_you", RetryPolicy: "once", CatchUpPolicy: "skip",
		LatenessPolicy: "within_90s", CreatedAt: now,
	}
	for name, mutate := range map[string]func(*conversation.RoutineRevision){
		"unknown approval":      func(revision *conversation.RoutineRevision) { revision.ApprovalBoundary = "model_decides" },
		"unknown missing input": func(revision *conversation.RoutineRevision) { revision.MissingInputBehavior = "invent" },
		"too-late policy":       func(revision *conversation.RoutineRevision) { revision.LatenessPolicy = "within_1h" },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := valid
			mutate(&candidate)
			if err := candidate.Validate(); err == nil {
				t.Fatalf("accepted unsupported first-release policy: %+v", candidate)
			}
		})
	}

	event := valid
	event.Trigger, event.Schedule, event.Timezone, event.NextOccurrence = conversation.RoutineTriggerEvent, "", "", time.Time{}
	event.InputSource, event.LatenessPolicy = "fort:event:mail", "none"
	if err := event.Validate(); err != nil {
		t.Fatalf("valid event policy: %v", err)
	}
}
