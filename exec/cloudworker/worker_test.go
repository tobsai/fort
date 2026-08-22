package cloudworker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tobsai/fort/cloud/controlapi"
	"github.com/tobsai/fort/core/runtime"
	coreworker "github.com/tobsai/fort/core/worker"
)

func TestWorkerRunOneRechecksThenRunsExactAssignmentAndCommitsPlaintextOutput(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 22, 0, 0, 0, time.UTC)
	assignment := testAssignment(now)
	steps := &stepLog{}
	control := &fakeControl{assignment: assignment, steps: steps}
	run := completedRun("attempt:1", "exact answer")
	runtime := &recordingRuntime{run: run, steps: steps}
	readiness := &fakeReadiness{snapshot: testReadiness(), steps: steps}
	registry, err := NewNativeRegistry([]ApprovedBinding{{Pins: assignment.Pins, Execution: assignment.Execution}})
	if err != nil {
		t.Fatal(err)
	}
	worker := Worker{
		Identity: Identity{AccountID: testAccountID, WorkerID: assignment.WorkerID, MachineID: assignment.MachineID},
		Control:  control, Runtime: runtime, Readiness: readiness, Adapters: registry,
		Clock: func() time.Time { return now }, IDs: &sequenceIDs{},
		Heartbeat: func(time.Duration) (<-chan time.Time, func()) { return make(chan time.Time), func() {} },
	}

	claimed, err := worker.RunOne(context.Background())
	if err != nil || !claimed {
		t.Fatalf("RunOne = %t, %v", claimed, err)
	}
	wantSpec := runtimepkgSpec(assignment)
	if !reflect.DeepEqual(runtime.spec, wantSpec) {
		t.Fatalf("runtime spec = %#v, want %#v", runtime.spec, wantSpec)
	}
	if got, want := steps.values(), []string{"readiness", "claim_next", "context_page", "recheck", "heartbeat", "dispatch", "artifact_create", "artifact_chunk", "artifact_finalize", "terminal"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("steps = %#v, want %#v", got, want)
	}
	if len(control.chunk.Plaintext) == 0 || string(control.chunk.Plaintext) != "exact answer" || len(control.chunk.Ciphertext) != 0 {
		t.Fatalf("artifact chunk = %#v", control.chunk)
	}
	if control.terminal.OutputMessagePlaintext == nil || *control.terminal.OutputMessagePlaintext != "exact answer" ||
		len(control.terminal.ReceiptPlaintext) == 0 || len(control.terminal.Receipt.Ciphertext) != 0 {
		t.Fatalf("terminal = %#v", control.terminal)
	}
}

func TestWorkerRunOneCancelsExactRunAndStopsWritingAfterStaleFence(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 22, 1, 0, 0, time.UTC)
	assignment := testAssignment(now)
	ticks := make(chan time.Time, 1)
	ticks <- now.Add(time.Second)
	run := newBlockingRun("attempt:1")
	control := &fakeControl{assignment: assignment, heartbeats: []fakeHeartbeatResponse{
		{result: continueHeartbeat(assignment)},
		{err: controlapi.ErrWorkerStaleLease},
	}, steps: &stepLog{}}
	registry, err := NewNativeRegistry([]ApprovedBinding{{Pins: assignment.Pins, Execution: assignment.Execution}})
	if err != nil {
		t.Fatal(err)
	}
	worker := Worker{
		Identity: Identity{AccountID: testAccountID, WorkerID: assignment.WorkerID, MachineID: assignment.MachineID},
		Control:  control, Runtime: &recordingRuntime{run: run}, Readiness: &fakeReadiness{snapshot: testReadiness()},
		Adapters: registry, Clock: func() time.Time { return now }, IDs: &sequenceIDs{},
		Heartbeat: func(time.Duration) (<-chan time.Time, func()) { return ticks, func() {} },
	}

	claimed, err := worker.RunOne(context.Background())
	if !claimed || !errors.Is(err, controlapi.ErrWorkerStaleLease) {
		t.Fatalf("RunOne = %t, %v, want claimed stale fence", claimed, err)
	}
	if run.cancelCalls != 1 {
		t.Fatalf("exact run cancel calls = %d, want 1", run.cancelCalls)
	}
	if control.create.ArtifactID != "" || control.terminal.TargetID != "" {
		t.Fatalf("stale worker wrote artifact/terminal: %#v %#v", control.create, control.terminal)
	}
}

func TestWorkerRunOneCancelsAndAcknowledgesTheExactRunBeforeCanceledTerminal(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 22, 1, 30, 0, time.UTC)
	assignment := testAssignment(now)
	ticks := make(chan time.Time, 1)
	ticks <- now.Add(time.Second)
	run := newBlockingRun("attempt:1")
	control := &fakeControl{
		assignment: assignment,
		heartbeats: []fakeHeartbeatResponse{
			{result: continueHeartbeat(assignment)},
			{result: controlapi.WorkerLeaseHeartbeatResult{
				TargetID: assignment.TargetID, ExecutionAttemptID: assignment.ExecutionAttemptID,
				LeaseID: assignment.LeaseID, FenceToken: assignment.FenceToken,
				Directive: coreworker.DirectiveCancel,
			}},
		},
		ack: controlapi.WorkerCancellationAck{
			AcknowledgementID: "cancel-ack:1", TargetID: assignment.TargetID,
			ExecutionAttemptID: assignment.ExecutionAttemptID, LeaseID: assignment.LeaseID,
			FenceToken: assignment.FenceToken, AcknowledgedAt: now.Add(time.Second),
		},
		steps: &stepLog{},
	}
	registry, err := NewNativeRegistry([]ApprovedBinding{{Pins: assignment.Pins, Execution: assignment.Execution}})
	if err != nil {
		t.Fatal(err)
	}
	worker := Worker{
		Identity: Identity{AccountID: testAccountID, WorkerID: assignment.WorkerID, MachineID: assignment.MachineID},
		Control:  control, Runtime: &recordingRuntime{run: run}, Readiness: &fakeReadiness{snapshot: testReadiness()},
		Adapters: registry, Clock: func() time.Time { return now }, IDs: &sequenceIDs{},
		Heartbeat: func(time.Duration) (<-chan time.Time, func()) { return ticks, func() {} },
	}

	claimed, err := worker.RunOne(context.Background())
	if err != nil || !claimed {
		t.Fatalf("RunOne = %t, %v", claimed, err)
	}
	if run.cancelCalls != 1 {
		t.Fatalf("exact run cancel calls = %d, want 1", run.cancelCalls)
	}
	if control.cancellationAck.TargetID != assignment.TargetID ||
		control.cancellationAck.ExecutionAttemptID != assignment.ExecutionAttemptID ||
		control.cancellationAck.LeaseID != assignment.LeaseID ||
		control.cancellationAck.FenceToken != assignment.FenceToken {
		t.Fatalf("cancellation acknowledgement = %#v", control.cancellationAck)
	}
	if control.terminal.Status != coreworker.TerminalCanceled {
		t.Fatalf("terminal status = %q, want canceled", control.terminal.Status)
	}
	if got, want := control.steps.values(), []string{"readiness", "claim_next", "context_page", "heartbeat", "heartbeat", "cancel_ack", "artifact_create", "artifact_chunk", "artifact_finalize", "terminal"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("steps = %#v, want %#v", got, want)
	}
}

func TestWorkerRunOneRejectsCancellationAcknowledgementForAnotherFence(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 22, 1, 45, 0, time.UTC)
	assignment := testAssignment(now)
	ticks := make(chan time.Time, 1)
	ticks <- now.Add(time.Second)
	run := newBlockingRun("attempt:1")
	control := &fakeControl{
		assignment: assignment,
		heartbeats: []fakeHeartbeatResponse{
			{result: continueHeartbeat(assignment)},
			{result: controlapi.WorkerLeaseHeartbeatResult{
				TargetID: assignment.TargetID, ExecutionAttemptID: assignment.ExecutionAttemptID,
				LeaseID: assignment.LeaseID, FenceToken: assignment.FenceToken,
				Directive: coreworker.DirectiveCancel,
			}},
		},
		ack: controlapi.WorkerCancellationAck{
			AcknowledgementID: "cancel-ack:1", TargetID: "target:other",
			ExecutionAttemptID: assignment.ExecutionAttemptID, LeaseID: assignment.LeaseID,
			FenceToken: assignment.FenceToken, AcknowledgedAt: now.Add(time.Second),
		},
	}
	registry, err := NewNativeRegistry([]ApprovedBinding{{Pins: assignment.Pins, Execution: assignment.Execution}})
	if err != nil {
		t.Fatal(err)
	}
	worker := Worker{
		Identity: Identity{AccountID: testAccountID, WorkerID: assignment.WorkerID, MachineID: assignment.MachineID},
		Control:  control, Runtime: &recordingRuntime{run: run}, Readiness: &fakeReadiness{snapshot: testReadiness()},
		Adapters: registry, Clock: func() time.Time { return now }, IDs: &sequenceIDs{},
		Heartbeat: func(time.Duration) (<-chan time.Time, func()) { return ticks, func() {} },
	}

	claimed, err := worker.RunOne(context.Background())
	if !claimed || !errors.Is(err, ErrWorkerInvalid) {
		t.Fatalf("RunOne = %t, %v, want invalid cancellation acknowledgement", claimed, err)
	}
	if run.cancelCalls != 1 || control.terminal.TargetID != "" {
		t.Fatalf("wrong-fence cancellation wrote terminal: cancels=%d terminal=%#v", run.cancelCalls, control.terminal)
	}
}

func TestWorkerRunOneHonorsCancelHeartbeatBeforeProviderStart(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 22, 1, 50, 0, time.UTC)
	assignment := testAssignment(now)
	cancel := continueHeartbeat(assignment)
	cancel.Directive = coreworker.DirectiveCancel
	steps := &stepLog{}
	control := &fakeControl{
		assignment: assignment, heartbeat: cancel,
		ack: controlapi.WorkerCancellationAck{
			AcknowledgementID: "cancel-ack:1", TargetID: assignment.TargetID,
			ExecutionAttemptID: assignment.ExecutionAttemptID, LeaseID: assignment.LeaseID,
			FenceToken: assignment.FenceToken, AcknowledgedAt: now,
		},
		steps: steps,
	}
	runtime := &recordingRuntime{run: completedRun("attempt:1", "must not run"), steps: steps}
	registry, err := NewNativeRegistry([]ApprovedBinding{{Pins: assignment.Pins, Execution: assignment.Execution}})
	if err != nil {
		t.Fatal(err)
	}
	worker := Worker{
		Identity: Identity{AccountID: testAccountID, WorkerID: assignment.WorkerID, MachineID: assignment.MachineID},
		Control:  control, Runtime: runtime, Readiness: &fakeReadiness{snapshot: testReadiness(), steps: steps},
		Adapters: registry, Clock: func() time.Time { return now }, IDs: &sequenceIDs{},
		Heartbeat: func(time.Duration) (<-chan time.Time, func()) { return make(chan time.Time), func() {} },
	}

	claimed, err := worker.RunOne(context.Background())
	if err != nil || !claimed {
		t.Fatalf("RunOne = %t, %v", claimed, err)
	}
	if runtime.dispatchCalls != 0 {
		t.Fatalf("provider dispatch calls = %d, want zero", runtime.dispatchCalls)
	}
	if control.terminal.Status != coreworker.TerminalCanceled {
		t.Fatalf("terminal status = %q, want canceled", control.terminal.Status)
	}
	if got, want := steps.values(), []string{"readiness", "claim_next", "context_page", "recheck", "heartbeat", "cancel_ack", "artifact_create", "artifact_chunk", "artifact_finalize", "terminal"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("steps = %#v, want %#v", got, want)
	}
}

func TestWorkerRunOneKeepsLargeSuccessfulOutputInArtifactWithoutInlineMessage(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 22, 2, 0, 0, time.UTC)
	assignment := testAssignment(now)
	large := strings.Repeat("x", controlapi.MaximumArtifactChunkPlaintextBytes+1)
	control := &fakeControl{assignment: assignment}
	registry, err := NewNativeRegistry([]ApprovedBinding{{Pins: assignment.Pins, Execution: assignment.Execution}})
	if err != nil {
		t.Fatal(err)
	}
	worker := Worker{
		Identity: Identity{AccountID: testAccountID, WorkerID: assignment.WorkerID, MachineID: assignment.MachineID},
		Control:  control, Runtime: &recordingRuntime{run: completedRun("attempt:1", large)},
		Readiness: &fakeReadiness{snapshot: testReadiness()}, Adapters: registry,
		Clock: func() time.Time { return now }, IDs: &sequenceIDs{},
		Heartbeat: func(time.Duration) (<-chan time.Time, func()) { return make(chan time.Time), func() {} },
	}
	claimed, err := worker.RunOne(context.Background())
	if err != nil || !claimed {
		t.Fatalf("RunOne = %t, %v", claimed, err)
	}
	if control.terminal.Status != coreworker.TerminalCompleted || control.terminal.OutputMessagePlaintext != nil {
		t.Fatalf("large terminal = %#v", control.terminal)
	}
	if control.create.ExpectedChunkCount != 2 || len(control.chunks) != 2 ||
		len(control.chunks[0].Plaintext)+len(control.chunks[1].Plaintext) != len(large) {
		t.Fatalf("large artifact = create %#v chunks=%d", control.create, len(control.chunks))
	}
}

func TestWorkerRunOneRejectsMismatchedPinnedOutputContractBeforeExecution(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 22, 2, 30, 0, time.UTC)
	assignment := testAssignment(now)
	assignment.OutputMessageKind = "handoff_result"
	control := &fakeControl{assignment: assignment}
	runtimeRecorder := &recordingRuntime{run: completedRun("attempt:1", "must not run")}
	registry, err := NewNativeRegistry([]ApprovedBinding{{Pins: assignment.Pins, Execution: assignment.Execution}})
	if err != nil {
		t.Fatal(err)
	}
	worker := Worker{
		Identity: Identity{AccountID: testAccountID, WorkerID: assignment.WorkerID, MachineID: assignment.MachineID},
		Control:  control, Runtime: runtimeRecorder, Readiness: &fakeReadiness{snapshot: testReadiness()},
		Adapters: registry, Clock: func() time.Time { return now }, IDs: &sequenceIDs{},
		Heartbeat: func(time.Duration) (<-chan time.Time, func()) { return make(chan time.Time), func() {} },
	}

	claimed, err := worker.RunOne(context.Background())
	if !claimed || !errors.Is(err, ErrWorkerInvalid) {
		t.Fatalf("RunOne = %t, %v, want invalid pinned output contract", claimed, err)
	}
	if runtimeRecorder.dispatchCalls != 0 || len(control.contextCursors) != 0 || control.terminal.TargetID != "" {
		t.Fatalf("invalid output contract progressed: dispatch=%d context=%#v terminal=%#v",
			runtimeRecorder.dispatchCalls, control.contextCursors, control.terminal)
	}
}

func TestNativeRegistryRejectsImmutableSelectorOrWorkdirDrift(t *testing.T) {
	t.Parallel()

	assignment := testAssignment(time.Now())
	registry, err := NewNativeRegistry([]ApprovedBinding{{Pins: assignment.Pins, Execution: assignment.Execution}})
	if err != nil {
		t.Fatal(err)
	}
	assignment.Execution.Workdir = "/Users/fort/Workspaces/other"
	if _, err := registry.Prepare(assignment, ExecutionContext{ManifestID: assignment.ContextManifestID, ManifestDigest: repeatDigest("c")}); !errors.Is(err, ErrAdapterNotApproved) {
		t.Fatalf("Prepare drift error = %v, want adapter not approved", err)
	}
}

func TestNativeRegistryRejectsIncompleteImmutableExecutionSelectors(t *testing.T) {
	t.Parallel()

	assignment := testAssignment(time.Now())
	assignment.Execution.RequestedModel = ""
	if _, err := NewNativeRegistry([]ApprovedBinding{{Pins: assignment.Pins, Execution: assignment.Execution}}); !errors.Is(err, ErrWorkerInvalid) {
		t.Fatalf("NewNativeRegistry error = %v, want invalid incomplete selector", err)
	}
}

func TestWorkerRunOneFailsClaimWithExplicitUnauthorizedAdapterReceipt(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 22, 3, 0, 0, time.UTC)
	assignment := testAssignment(now)
	control := &fakeControl{assignment: assignment}
	worker := Worker{
		Identity: Identity{AccountID: testAccountID, WorkerID: assignment.WorkerID, MachineID: assignment.MachineID},
		Control:  control, Runtime: &recordingRuntime{run: completedRun("attempt:1", "must not run")},
		Readiness: &fakeReadiness{snapshot: testReadiness()}, Adapters: deniedAdapters{},
		Clock: func() time.Time { return now }, IDs: &sequenceIDs{},
		Heartbeat: func(time.Duration) (<-chan time.Time, func()) { return make(chan time.Time), func() {} },
	}

	claimed, err := worker.RunOne(context.Background())
	if err != nil || !claimed {
		t.Fatalf("RunOne = %t, %v", claimed, err)
	}
	if control.terminal.Status != coreworker.TerminalFailed || len(control.chunks) != 1 || string(control.chunks[0].Plaintext) != "adapter_not_authorized" {
		t.Fatalf("unauthorized adapter terminal = %#v chunks=%#v", control.terminal, control.chunks)
	}
}

type deniedAdapters struct{}

func (deniedAdapters) Prepare(controlapi.WorkerAssignment, ExecutionContext) (runtime.RunSpec, error) {
	return runtime.RunSpec{}, ErrAdapterNotApproved
}

const testAccountID = "4af424a4-d81a-47d5-a495-400868883b86"

func testAssignment(now time.Time) controlapi.WorkerAssignment {
	return controlapi.WorkerAssignment{
		TargetID: "target:1", TargetKind: "initial", OriginID: "turn:1",
		ExecutionAttemptID: "attempt:1", LeaseID: "lease:1", FenceToken: 41,
		WorkerID: "worker:studio", MachineID: "machine:studio", CapabilityRevisionID: "capability:7",
		Pins: coreworker.ExecutionPins{AgentID: "agent:researcher", BehaviorRevisionID: "behavior:4", BindingRevisionID: "binding:9", SeatID: "seat:researcher",
			EffectiveAuthoritySnapshot: coreworker.AuthoritySnapshot{ID: "grant:1", Revision: "authority:7", Permissions: []string{"message.append"}}},
		Execution: controlapi.WorkerExecutionBinding{
			ExecutionSourceID: "source:studio", SourceAgentID: "source-agent:researcher", OpaqueSourceAgentID: "researcher",
			FortProfile: "openclaw:researcher", Provider: "openclaw", RequestedModel: "openclaw-main", ResolvedModel: "openclaw-main",
			AdapterID: "model.chat.openclaw", AdapterRevision: "adapter:1", SourceConfigDigest: repeatDigest("a"),
			AuthorityID: "authority:binding:1", AuthorityRevision: "authority:7", PolicyID: "policy:1", PolicyRevision: "policy:2",
			SessionBehavior: "isolated", MemoryBehavior: "source_managed", CapabilityEvidence: json.RawMessage(`{"values":["openclaw-ready","workdir=/Users/fort/Workspaces/researcher"],"location_kind":"computer"}`),
			ReadinessContractID: "ready:openclaw", ReadinessContractRevision: "ready:4", Workdir: "/Users/fort/Workspaces/researcher", ComputerID: "machine:studio",
		},
		ContextManifestID: "context:1", Prompt: "Use exact evidence.",
		OutputConversationID: "conversation:1", OutputMessageKind: "agent", OutputAuthorAgentID: "agent:researcher",
		MaximumOutputPlaintextBytes: controlapi.MaximumArtifactPlaintextBytes,
		InlineOutputPlaintextBytes:  controlapi.MaximumArtifactChunkPlaintextBytes, ClaimedAt: now,
		ExpiresAt: now.Add(2 * time.Minute), HardDeadline: now.Add(10 * time.Minute),
	}
}

func testReadiness() ReadinessSnapshot {
	evidence := json.RawMessage(`{"frameworks":["openclaw"],"ready":true}`)
	digest := sha256.Sum256(evidence)
	return ReadinessSnapshot{CapabilityRevisionID: "capability:7", Revision: 7, Evidence: evidence, EvidenceDigest: hex.EncodeToString(digest[:])}
}

func runtimepkgSpec(assignment controlapi.WorkerAssignment) runtime.RunSpec {
	return runtime.RunSpec{RunID: assignment.ExecutionAttemptID, Profile: assignment.Execution.FortProfile,
		Agent: assignment.Execution.Provider, Model: assignment.Execution.RequestedModel,
		Prompt: assignment.Prompt, Workdir: assignment.Execution.Workdir}
}

func repeatDigest(value string) string {
	result := ""
	for len(result) < 64 {
		result += value
	}
	return result[:64]
}

type fakeControl struct {
	assignment       controlapi.WorkerAssignment
	contextPages     []controlapi.WorkerContextPage
	contextCursors   []string
	contextPageCalls int
	heartbeat        controlapi.WorkerLeaseHeartbeatResult
	heartbeatErr     error
	heartbeats       []fakeHeartbeatResponse
	heartbeatCalls   int
	ack              controlapi.WorkerCancellationAck
	steps            *stepLog
	cancellationAck  controlapi.WorkerCancellationAckCommand
	create           controlapi.WorkerArtifactCreateCommand
	chunk            controlapi.WorkerArtifactChunkCommand
	chunks           []controlapi.WorkerArtifactChunkCommand
	terminal         controlapi.WorkerTerminalCommand
}

func (control *fakeControl) add(step string) {
	if control.steps != nil {
		control.steps.add(step)
	}
}

func (control *fakeControl) RecordWorkerReadiness(_ context.Context, command controlapi.WorkerReadinessCommand) (controlapi.WorkerReadinessResult, error) {
	control.add("readiness")
	return controlapi.WorkerReadinessResult{Status: "ready", CapabilityRevisionID: command.CapabilityRevisionID, ObservedAt: command.ObservedAt}, nil
}
func (control *fakeControl) ClaimNextWorkerTarget(context.Context, controlapi.WorkerClaimNextCommand) (controlapi.WorkerAssignment, error) {
	control.add("claim_next")
	return control.assignment, nil
}
func (control *fakeControl) ReadWorkerContextPage(_ context.Context, command controlapi.WorkerContextPageCommand) (controlapi.WorkerContextPage, error) {
	control.add("context_page")
	control.contextCursors = append(control.contextCursors, command.Cursor)
	if control.contextPageCalls < len(control.contextPages) {
		page := control.contextPages[control.contextPageCalls]
		control.contextPageCalls++
		return page, nil
	}
	control.contextPageCalls++
	return controlapi.WorkerContextPage{
		ContextManifestID: control.assignment.ContextManifestID,
		ManifestDigest:    repeatDigest("c"), Items: []controlapi.WorkerContextItem{},
	}, nil
}
func (control *fakeControl) HeartbeatWorkerLease(context.Context, controlapi.WorkerLeaseHeartbeatCommand) (controlapi.WorkerLeaseHeartbeatResult, error) {
	control.add("heartbeat")
	if control.heartbeatCalls < len(control.heartbeats) {
		response := control.heartbeats[control.heartbeatCalls]
		control.heartbeatCalls++
		return response.result, response.err
	}
	control.heartbeatCalls++
	if control.heartbeat == (controlapi.WorkerLeaseHeartbeatResult{}) && control.heartbeatErr == nil {
		return continueHeartbeat(control.assignment), nil
	}
	return control.heartbeat, control.heartbeatErr
}
func (control *fakeControl) AcknowledgeWorkerCancellation(_ context.Context, command controlapi.WorkerCancellationAckCommand) (controlapi.WorkerCancellationAck, error) {
	control.add("cancel_ack")
	control.cancellationAck = command
	return control.ack, nil
}
func (control *fakeControl) CreateWorkerArtifact(_ context.Context, command controlapi.WorkerArtifactCreateCommand) (controlapi.WorkerArtifact, error) {
	control.add("artifact_create")
	control.create = command
	return controlapi.WorkerArtifact{ArtifactID: command.ArtifactID}, nil
}
func (control *fakeControl) GetWorkerArtifactStatus(context.Context, controlapi.WorkerArtifactStatusCommand) (controlapi.WorkerArtifact, error) {
	return controlapi.WorkerArtifact{}, nil
}
func (control *fakeControl) AppendWorkerArtifactChunk(_ context.Context, command controlapi.WorkerArtifactChunkCommand) (controlapi.WorkerArtifactChunk, error) {
	control.add("artifact_chunk")
	control.chunk = command
	control.chunks = append(control.chunks, command)
	return controlapi.WorkerArtifactChunk{ArtifactID: command.ArtifactID, ChunkIndex: command.ChunkIndex}, nil
}
func (control *fakeControl) FinalizeWorkerArtifact(_ context.Context, command controlapi.WorkerArtifactFinalizeCommand) (controlapi.WorkerArtifact, error) {
	control.add("artifact_finalize")
	return controlapi.WorkerArtifact{ArtifactID: command.ArtifactID, State: "finalized"}, nil
}
func (control *fakeControl) CommitWorkerTerminal(_ context.Context, command controlapi.WorkerTerminalCommand) (controlapi.WorkerTerminalResult, error) {
	control.add("terminal")
	control.terminal = command
	return controlapi.WorkerTerminalResult{TargetID: command.TargetID, Status: command.Status}, nil
}

type fakeReadiness struct {
	snapshot ReadinessSnapshot
	err      error
	steps    *stepLog
}

func (readiness *fakeReadiness) Snapshot(context.Context) (ReadinessSnapshot, error) {
	return readiness.snapshot, readiness.err
}
func (readiness *fakeReadiness) Recheck(context.Context, controlapi.WorkerAssignment) error {
	if readiness.steps != nil {
		readiness.steps.add("recheck")
	}
	return readiness.err
}

type recordingRuntime struct {
	run           runtime.Run
	err           error
	spec          runtime.RunSpec
	steps         *stepLog
	dispatchCalls int
}

func (rt *recordingRuntime) Name() string { return "recording" }
func (rt *recordingRuntime) Dispatch(_ context.Context, spec runtime.RunSpec) (runtime.Run, error) {
	rt.dispatchCalls++
	rt.spec = spec
	if rt.steps != nil {
		rt.steps.add("dispatch")
	}
	return rt.run, rt.err
}

type fakeHeartbeatResponse struct {
	result controlapi.WorkerLeaseHeartbeatResult
	err    error
}

func continueHeartbeat(assignment controlapi.WorkerAssignment) controlapi.WorkerLeaseHeartbeatResult {
	return controlapi.WorkerLeaseHeartbeatResult{
		TargetID: assignment.TargetID, ExecutionAttemptID: assignment.ExecutionAttemptID,
		LeaseID: assignment.LeaseID, FenceToken: assignment.FenceToken,
		Directive: coreworker.DirectiveContinue, ExpiresAt: assignment.ExpiresAt,
	}
}

type fakeRun struct {
	id          string
	events      chan runtime.RunEvent
	status      runtime.Status
	cancelCalls int
	mu          sync.Mutex
}

func completedRun(id, message string) *fakeRun {
	events := make(chan runtime.RunEvent, 2)
	events <- runtime.RunEvent{RunID: id, Type: runtime.EventMessage, Data: message}
	events <- runtime.RunEvent{RunID: id, Type: runtime.EventExited, Code: 0}
	close(events)
	return &fakeRun{id: id, events: events, status: runtime.Status{State: runtime.StateSucceeded}}
}

func newBlockingRun(id string) *fakeRun {
	return &fakeRun{id: id, events: make(chan runtime.RunEvent), status: runtime.Status{State: runtime.StateRunning}}
}

func (run *fakeRun) ID() string                      { return run.id }
func (run *fakeRun) Stream() <-chan runtime.RunEvent { return run.events }
func (run *fakeRun) Signal(string) error             { return nil }
func (run *fakeRun) Status() runtime.Status          { run.mu.Lock(); defer run.mu.Unlock(); return run.status }
func (run *fakeRun) Wait() runtime.Status            { return run.Status() }
func (run *fakeRun) Cancel() error {
	run.mu.Lock()
	defer run.mu.Unlock()
	run.cancelCalls++
	if run.status.State == runtime.StateRunning {
		run.status = runtime.Status{State: runtime.StateCanceled}
		close(run.events)
	}
	return nil
}

type stepLog struct {
	mu    sync.Mutex
	steps []string
}

func (log *stepLog) add(step string) {
	log.mu.Lock()
	defer log.mu.Unlock()
	log.steps = append(log.steps, step)
}
func (log *stepLog) values() []string {
	log.mu.Lock()
	defer log.mu.Unlock()
	return append([]string(nil), log.steps...)
}

type sequenceIDs struct{ next map[string]int }

func (ids *sequenceIDs) New(kind string) string {
	if ids.next == nil {
		ids.next = make(map[string]int)
	}
	ids.next[kind]++
	return kind + ":" + string(rune('0'+ids.next[kind]))
}
