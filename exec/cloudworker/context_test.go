package cloudworker

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/tobsai/fort/cloud/controlapi"
	"github.com/tobsai/fort/core/runtime"
)

func TestWorkerRunOnePagesOrderedTypedContextBeforeRecheckWithoutConcatenatingPrompt(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 23, 45, 0, 0, time.UTC)
	assignment := testAssignment(now)
	steps := &stepLog{}
	control := &fakeControl{assignment: assignment, steps: steps, contextPages: []controlapi.WorkerContextPage{
		{
			ContextManifestID: assignment.ContextManifestID, ManifestDigest: repeatDigest("c"), NextCursor: "cursor_one",
			Items: []controlapi.WorkerContextItem{{Kind: controlapi.WorkerContextMessageKind, Ordinal: 0,
				Message: &controlapi.WorkerContextMessage{MessageID: 7, ConversationID: "conversation:1", MessageKind: "human",
					AuthorKind: "human", AuthorID: "human:1", Body: "Pinned context", CreatedAt: now.Add(-time.Minute)}}},
		},
		{
			ContextManifestID: assignment.ContextManifestID, ManifestDigest: repeatDigest("c"),
			Items: []controlapi.WorkerContextItem{{Kind: controlapi.WorkerContextArtifactKind, Ordinal: 1,
				Artifact: &controlapi.WorkerContextArtifactReference{ArtifactID: "artifact:context:1", Kind: "context",
					ExecutionAttemptID: "attempt:source", ExpectedChunkCount: 1, ExpectedPlaintextLength: 12,
					ExpectedEncodedLength: 28, LogicalDigest: repeatDigest("d"), CreatedAt: now.Add(-time.Minute), FinalizedAt: now}}},
		},
	}}
	adapters := &recordingContextAdapters{spec: runtimepkgSpec(assignment)}
	runtimeRecorder := &recordingRuntime{run: completedRun("attempt:1", "exact answer"), steps: steps}
	worker := Worker{
		Identity: Identity{AccountID: testAccountID, WorkerID: assignment.WorkerID, MachineID: assignment.MachineID},
		Control:  control, Runtime: runtimeRecorder, Readiness: &fakeReadiness{snapshot: testReadiness(), steps: steps},
		Adapters: adapters, Clock: func() time.Time { return now }, IDs: &sequenceIDs{},
		Heartbeat: func(time.Duration) (<-chan time.Time, func()) { return make(chan time.Time), func() {} },
	}

	claimed, err := worker.RunOne(context.Background())
	if err != nil || !claimed {
		t.Fatalf("RunOne = %t, %v", claimed, err)
	}
	if adapters.context.ManifestID != assignment.ContextManifestID || adapters.context.ManifestDigest != repeatDigest("c") ||
		len(adapters.context.Items) != 2 || adapters.context.Items[0].Message == nil || adapters.context.Items[1].Artifact == nil {
		t.Fatalf("adapter context = %#v", adapters.context)
	}
	if got, want := control.contextCursors, []string{"", "cursor_one"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("context cursors = %#v, want %#v", got, want)
	}
	if runtimeRecorder.spec.Prompt != assignment.Prompt {
		t.Fatalf("runtime prompt = %q, want unmodified assignment prompt %q", runtimeRecorder.spec.Prompt, assignment.Prompt)
	}
	if got, want := steps.values(), []string{"readiness", "claim_next", "context_page", "context_page", "recheck", "heartbeat", "dispatch", "artifact_create", "artifact_chunk", "artifact_finalize", "terminal"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("steps = %#v, want %#v", got, want)
	}
}

func TestWorkerRunOneRejectsRepeatedContextCursorBeforeRecheckOrRuntime(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 23, 46, 0, 0, time.UTC)
	assignment := testAssignment(now)
	control := &fakeControl{assignment: assignment, contextPages: []controlapi.WorkerContextPage{
		{ContextManifestID: assignment.ContextManifestID, ManifestDigest: repeatDigest("c"),
			Items: []controlapi.WorkerContextItem{{Kind: controlapi.WorkerContextMessageKind, Ordinal: 0,
				Message: &controlapi.WorkerContextMessage{MessageID: 7, ConversationID: "conversation:1", MessageKind: "human", AuthorKind: "human", AuthorID: "human:1", Body: "one", CreatedAt: now}}},
			NextCursor: "cursor_one"},
		{ContextManifestID: assignment.ContextManifestID, ManifestDigest: repeatDigest("c"),
			Items: []controlapi.WorkerContextItem{{Kind: controlapi.WorkerContextMessageKind, Ordinal: 1,
				Message: &controlapi.WorkerContextMessage{MessageID: 8, ConversationID: "conversation:1", MessageKind: "human", AuthorKind: "human", AuthorID: "human:1", Body: "two", CreatedAt: now}}},
			NextCursor: "cursor_one"},
	}}
	readiness := &fakeReadiness{snapshot: testReadiness()}
	runtimeRecorder := &recordingRuntime{run: completedRun("attempt:1", "must not run")}
	worker := Worker{
		Identity: Identity{AccountID: testAccountID, WorkerID: assignment.WorkerID, MachineID: assignment.MachineID},
		Control:  control, Runtime: runtimeRecorder, Readiness: readiness,
		Adapters: &recordingContextAdapters{spec: runtimepkgSpec(assignment)},
		Clock:    func() time.Time { return now }, IDs: &sequenceIDs{},
		Heartbeat: func(time.Duration) (<-chan time.Time, func()) { return make(chan time.Time), func() {} },
	}

	claimed, err := worker.RunOne(context.Background())
	if !claimed || !errors.Is(err, ErrWorkerInvalid) {
		t.Fatalf("RunOne = %t, %v, want invalid repeated cursor", claimed, err)
	}
	if runtimeRecorder.dispatchCalls != 0 || control.terminal.TargetID != "" {
		t.Fatalf("invalid context started/wrote terminal: dispatch=%d terminal=%#v", runtimeRecorder.dispatchCalls, control.terminal)
	}
}

func TestNativeRegistryDeniesNonEmptyContextWithoutApprovedEncoding(t *testing.T) {
	t.Parallel()

	assignment := testAssignment(time.Now())
	registry, err := NewNativeRegistry([]ApprovedBinding{{Pins: assignment.Pins, Execution: assignment.Execution}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = registry.Prepare(assignment, ExecutionContext{ManifestID: assignment.ContextManifestID,
		ManifestDigest: repeatDigest("c"), Items: []controlapi.WorkerContextItem{{Kind: controlapi.WorkerContextMessageKind, Ordinal: 0,
			Message: &controlapi.WorkerContextMessage{MessageID: 1, Body: "context"}}}})
	if !errors.Is(err, ErrAdapterNotApproved) {
		t.Fatalf("Prepare non-empty context error = %v, want not approved", err)
	}
}

type recordingContextAdapters struct {
	spec    runtime.RunSpec
	context ExecutionContext
}

func (registry *recordingContextAdapters) Prepare(_ controlapi.WorkerAssignment, executionContext ExecutionContext) (runtime.RunSpec, error) {
	registry.context = executionContext
	return registry.spec, nil
}
