// Package cloudworker runs one exact cloud-leased target through Fort's native
// runtime without importing any provider credential or cloud database client.
package cloudworker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/tobsai/fort/cloud/controlapi"
	"github.com/tobsai/fort/core/runtime"
	coreworker "github.com/tobsai/fort/core/worker"
)

var (
	ErrAdapterNotApproved = errors.New("cloud worker adapter binding is not approved")
	ErrWorkerInvalid      = errors.New("cloud worker configuration or assignment is invalid")
)

type Identity struct {
	AccountID string
	WorkerID  string
	MachineID string
}

type ReadinessSnapshot struct {
	CapabilityRevisionID string
	Revision             int
	Evidence             json.RawMessage
	EvidenceDigest       string
}

// Readiness owns local capability observation. Recheck must probe the same
// exact local evidence again; Worker invokes it immediately before Dispatch.
type Readiness interface {
	Snapshot(context.Context) (ReadinessSnapshot, error)
	Recheck(context.Context, controlapi.WorkerAssignment) error
}

// Control is transport-agnostic. HTTPClient implements this interface while
// unit tests can prove ordering, fencing, and cancellation without a network.
type Control interface {
	RecordWorkerReadiness(context.Context, controlapi.WorkerReadinessCommand) (controlapi.WorkerReadinessResult, error)
	ClaimNextWorkerTarget(context.Context, controlapi.WorkerClaimNextCommand) (controlapi.WorkerAssignment, error)
	ReadWorkerContextPage(context.Context, controlapi.WorkerContextPageCommand) (controlapi.WorkerContextPage, error)
	HeartbeatWorkerLease(context.Context, controlapi.WorkerLeaseHeartbeatCommand) (controlapi.WorkerLeaseHeartbeatResult, error)
	AcknowledgeWorkerCancellation(context.Context, controlapi.WorkerCancellationAckCommand) (controlapi.WorkerCancellationAck, error)
	CreateWorkerArtifact(context.Context, controlapi.WorkerArtifactCreateCommand) (controlapi.WorkerArtifact, error)
	GetWorkerArtifactStatus(context.Context, controlapi.WorkerArtifactStatusCommand) (controlapi.WorkerArtifact, error)
	AppendWorkerArtifactChunk(context.Context, controlapi.WorkerArtifactChunkCommand) (controlapi.WorkerArtifactChunk, error)
	FinalizeWorkerArtifact(context.Context, controlapi.WorkerArtifactFinalizeCommand) (controlapi.WorkerArtifact, error)
	CommitWorkerTerminal(context.Context, controlapi.WorkerTerminalCommand) (controlapi.WorkerTerminalResult, error)
}

type AdapterRegistry interface {
	Prepare(controlapi.WorkerAssignment, ExecutionContext) (runtime.RunSpec, error)
}

type ApprovedBinding struct {
	Pins      coreworker.ExecutionPins          `json:"pins"`
	Execution controlapi.WorkerExecutionBinding `json:"execution"`
}

// NativeRegistry is an explicit local allow-list. Every immutable execution
// selector, including the server-pinned workdir, must match before a RunSpec is
// produced. It never falls back to another provider, model, adapter, or path.
type NativeRegistry struct {
	bindings []ApprovedBinding
}

func NewNativeRegistry(bindings []ApprovedBinding) (*NativeRegistry, error) {
	if len(bindings) == 0 {
		return nil, fmt.Errorf("%w: approved adapter registry is empty", ErrWorkerInvalid)
	}
	result := &NativeRegistry{bindings: append([]ApprovedBinding(nil), bindings...)}
	for _, binding := range result.bindings {
		if !validApprovedBinding(binding) {
			return nil, fmt.Errorf("%w: approved adapter binding is incomplete", ErrWorkerInvalid)
		}
	}
	return result, nil
}

func validApprovedBinding(binding ApprovedBinding) bool {
	pins, execution := binding.Pins, binding.Execution
	if blank(pins.AgentID, pins.BehaviorRevisionID, pins.BindingRevisionID, pins.SeatID,
		pins.EffectiveAuthoritySnapshot.ID, pins.EffectiveAuthoritySnapshot.Revision,
		execution.ExecutionSourceID, execution.SourceAgentID, execution.OpaqueSourceAgentID,
		execution.FortProfile, execution.Provider, execution.RequestedModel, execution.ResolvedModel,
		execution.AdapterID, execution.AdapterRevision, execution.SourceConfigDigest,
		execution.AuthorityID, execution.AuthorityRevision, execution.PolicyID, execution.PolicyRevision,
		execution.SessionBehavior, execution.MemoryBehavior, execution.ReadinessContractID,
		execution.ReadinessContractRevision, execution.Workdir, execution.ComputerID) {
		return false
	}
	if execution.CloudRuntime != "" || execution.AdapterID != "model.chat."+execution.Provider ||
		len(execution.SourceConfigDigest) != 64 || strings.ToLower(execution.SourceConfigDigest) != execution.SourceConfigDigest {
		return false
	}
	if _, err := hex.DecodeString(execution.SourceConfigDigest); err != nil {
		return false
	}
	if !filepath.IsAbs(execution.Workdir) || filepath.Clean(execution.Workdir) != execution.Workdir || execution.Workdir == string(filepath.Separator) {
		return false
	}
	var evidence map[string]any
	return len(execution.CapabilityEvidence) > 0 && json.Unmarshal(execution.CapabilityEvidence, &evidence) == nil && evidence != nil
}

func blank(values ...string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return true
		}
	}
	return false
}

func (registry *NativeRegistry) Prepare(assignment controlapi.WorkerAssignment, executionContext ExecutionContext) (runtime.RunSpec, error) {
	if registry == nil {
		return runtime.RunSpec{}, ErrAdapterNotApproved
	}
	if executionContext.ManifestID != assignment.ContextManifestID || !lowerDigest(executionContext.ManifestDigest) || len(executionContext.Items) != 0 {
		return runtime.RunSpec{}, ErrAdapterNotApproved
	}
	for _, approved := range registry.bindings {
		if reflect.DeepEqual(approved.Pins, assignment.Pins) && reflect.DeepEqual(approved.Execution, assignment.Execution) {
			return runtime.RunSpec{
				RunID: assignment.ExecutionAttemptID, Profile: assignment.Execution.FortProfile,
				Agent: assignment.Execution.Provider, Model: assignment.Execution.RequestedModel,
				Prompt: assignment.Prompt, Workdir: assignment.Execution.Workdir,
			}, nil
		}
	}
	return runtime.RunSpec{}, ErrAdapterNotApproved
}

type IDSource interface {
	New(kind string) string
}

type HeartbeatFactory func(time.Duration) (<-chan time.Time, func())

type Worker struct {
	Identity          Identity
	Control           Control
	Runtime           runtime.Runtime
	Readiness         Readiness
	Adapters          AdapterRegistry
	Clock             func() time.Time
	IDs               IDSource
	Heartbeat         HeartbeatFactory
	HeartbeatInterval time.Duration
}

// RunOne records readiness, atomically claims at most one compatible target,
// executes it, and durably commits its plaintext output through Control.
func (worker *Worker) RunOne(ctx context.Context) (bool, error) {
	if err := worker.validate(); err != nil {
		return false, err
	}
	now := worker.Clock().UTC()
	readiness, err := worker.Readiness.Snapshot(ctx)
	if err != nil {
		return false, fmt.Errorf("cloud worker readiness snapshot: %w", err)
	}
	if !validReadinessSnapshot(readiness) {
		return false, fmt.Errorf("%w: readiness snapshot", ErrWorkerInvalid)
	}
	if _, err := worker.Control.RecordWorkerReadiness(ctx, controlapi.WorkerReadinessCommand{
		AccountID: worker.Identity.AccountID, WorkerID: worker.Identity.WorkerID, MachineID: worker.Identity.MachineID,
		IdempotencyKey:       "readiness:" + readiness.CapabilityRevisionID,
		CapabilityRevisionID: readiness.CapabilityRevisionID, Revision: readiness.Revision,
		CapabilityEvidence: append(json.RawMessage(nil), readiness.Evidence...), EvidenceDigest: readiness.EvidenceDigest,
		ObservedAt: now,
	}); err != nil {
		return false, fmt.Errorf("cloud worker record readiness: %w", err)
	}
	attemptID, leaseID := worker.IDs.New("attempt"), worker.IDs.New("lease")
	assignment, err := worker.Control.ClaimNextWorkerTarget(ctx, controlapi.WorkerClaimNextCommand{
		AccountID: worker.Identity.AccountID, WorkerID: worker.Identity.WorkerID, MachineID: worker.Identity.MachineID,
		ExecutionAttemptID: attemptID, LeaseID: leaseID, IdempotencyKey: worker.IDs.New("claim"),
		CapabilityRevisionID: readiness.CapabilityRevisionID, ClaimedAt: now, ExpiresAt: now.Add(controlapi.DefaultWorkerLease),
	})
	if errors.Is(err, controlapi.ErrWorkerNoCompatibleTarget) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("cloud worker claim next: %w", err)
	}
	if err := worker.validateAssignment(assignment, attemptID, leaseID, readiness.CapabilityRevisionID, now); err != nil {
		return true, err
	}

	executionContext, err := worker.readExecutionContext(ctx, assignment)
	if err != nil {
		return true, err
	}
	spec, err := worker.Adapters.Prepare(assignment, executionContext)
	if err != nil {
		if finishErr := worker.finish(ctx, assignment, coreworker.TerminalFailed, []byte("adapter_not_authorized"), 0); finishErr != nil {
			return true, errors.Join(err, finishErr)
		}
		return true, nil
	}
	// No network, persistence, routing, or mutable adapter selection may occur
	// between this exact local recheck and provider start.
	if err := worker.Readiness.Recheck(ctx, assignment); err != nil {
		if finishErr := worker.finish(ctx, assignment, coreworker.TerminalFailed, []byte("readiness_recheck_failed"), 0); finishErr != nil {
			return true, errors.Join(err, finishErr)
		}
		return true, nil
	}
	// This fenced heartbeat is the durable Working transition. It must succeed
	// after the exact local recheck and before any provider process starts.
	startHeartbeatAt := worker.Clock().UTC()
	startHeartbeat, err := worker.heartbeat(ctx, assignment, startHeartbeatAt)
	if err != nil {
		return true, err
	}
	switch startHeartbeat.Directive {
	case coreworker.DirectiveContinue:
	case coreworker.DirectiveCancel:
		if err := worker.acknowledgeCancellation(ctx, assignment, startHeartbeatAt); err != nil {
			return true, err
		}
		if err := worker.finish(ctx, assignment, coreworker.TerminalCanceled, []byte("canceled_before_start"), 0); err != nil {
			return true, err
		}
		return true, nil
	default:
		return true, fmt.Errorf("%w: heartbeat returned an unknown directive", ErrWorkerInvalid)
	}
	run, err := worker.Runtime.Dispatch(ctx, spec)
	if err != nil {
		if finishErr := worker.finish(ctx, assignment, coreworker.TerminalFailed, []byte("runtime_start_failed"), 0); finishErr != nil {
			return true, errors.Join(err, finishErr)
		}
		return true, nil
	}
	if run == nil || run.ID() != assignment.ExecutionAttemptID {
		if run != nil {
			_ = run.Cancel()
		}
		return true, fmt.Errorf("%w: runtime returned wrong run identity", ErrWorkerInvalid)
	}

	output, terminalStatus, exitCode, err := worker.consumeRun(ctx, assignment, run)
	if err != nil {
		return true, err
	}
	if err := worker.finish(ctx, assignment, terminalStatus, output, exitCode); err != nil {
		return true, err
	}
	return true, nil
}

func (worker *Worker) consumeRun(ctx context.Context, assignment controlapi.WorkerAssignment, run runtime.Run) ([]byte, coreworker.TerminalStatus, int, error) {
	interval := worker.HeartbeatInterval
	if interval <= 0 {
		interval = controlapi.DefaultWorkerLease / 3
	}
	ticks, stop := worker.Heartbeat(interval)
	defer stop()
	var output []byte
	cancelAcknowledged := false
	stream := run.Stream()
	for stream != nil {
		select {
		case <-ctx.Done():
			_ = run.Cancel()
			return nil, "", 0, ctx.Err()
		case event, open := <-stream:
			if !open {
				stream = nil
				continue
			}
			if event.RunID != "" && event.RunID != assignment.ExecutionAttemptID {
				_ = run.Cancel()
				return nil, "", 0, fmt.Errorf("%w: runtime event has wrong run identity", ErrWorkerInvalid)
			}
			if event.Type == runtime.EventMessage {
				if int64(len(event.Data)) > assignment.MaximumOutputPlaintextBytes {
					_ = run.Cancel()
					return []byte("output_limit_exceeded"), coreworker.TerminalFailed, 0, nil
				}
				output = append(output[:0], event.Data...)
			}
		case <-ticks:
			now := worker.Clock().UTC()
			if !assignment.HardDeadline.IsZero() && !now.Before(assignment.HardDeadline) {
				_ = run.Cancel()
				return nil, "", 0, controlapi.ErrWorkerStaleLease
			}
			heartbeat, err := worker.heartbeat(ctx, assignment, now)
			if err != nil {
				_ = run.Cancel()
				if errors.Is(err, controlapi.ErrWorkerStaleLease) {
					_ = run.Wait()
					return nil, "", 0, controlapi.ErrWorkerStaleLease
				}
				return nil, "", 0, err
			}
			if heartbeat.Directive == coreworker.DirectiveCancel && !cancelAcknowledged {
				if err := run.Cancel(); err != nil {
					return nil, "", 0, fmt.Errorf("cloud worker cancel exact run: %w", err)
				}
				if err := worker.acknowledgeCancellation(ctx, assignment, now); err != nil {
					return nil, "", 0, err
				}
				cancelAcknowledged = true
			} else if heartbeat.Directive != coreworker.DirectiveContinue && heartbeat.Directive != coreworker.DirectiveCancel {
				_ = run.Cancel()
				return nil, "", 0, fmt.Errorf("%w: heartbeat returned an unknown directive", ErrWorkerInvalid)
			}
		}
	}

	status := run.Wait()
	terminalStatus := coreworker.TerminalFailed
	switch status.State {
	case runtime.StateSucceeded:
		terminalStatus = coreworker.TerminalCompleted
	case runtime.StateCanceled:
		terminalStatus = coreworker.TerminalCanceled
	}
	if terminalStatus == coreworker.TerminalCompleted && len(output) == 0 {
		terminalStatus, output = coreworker.TerminalFailed, []byte("normalized_output_unavailable")
	}
	if len(output) == 0 {
		if terminalStatus == coreworker.TerminalCanceled {
			output = []byte("canceled")
		} else {
			output = []byte("runtime_failed")
		}
	}
	return output, terminalStatus, status.ExitCode, nil
}

func (worker *Worker) finish(ctx context.Context, assignment controlapi.WorkerAssignment, status coreworker.TerminalStatus, output []byte, exitCode int) error {
	if len(output) == 0 || int64(len(output)) > assignment.MaximumOutputPlaintextBytes {
		return fmt.Errorf("%w: terminal output length", ErrWorkerInvalid)
	}
	now := worker.Clock().UTC()
	if !assignment.HardDeadline.IsZero() && !now.Before(assignment.HardDeadline) {
		return controlapi.ErrWorkerStaleLease
	}
	digestBytes := sha256.Sum256(output)
	digest := hex.EncodeToString(digestBytes[:])
	chunkCount := (len(output) + controlapi.MaximumArtifactChunkPlaintextBytes - 1) / controlapi.MaximumArtifactChunkPlaintextBytes
	artifactID := worker.IDs.New("artifact")
	identity := worker.artifactIdentity(assignment)
	if _, err := worker.Control.CreateWorkerArtifact(ctx, controlapi.WorkerArtifactCreateCommand{
		AccountID: identity.AccountID, WorkerID: identity.WorkerID, MachineID: identity.MachineID,
		TargetID: identity.TargetID, ExecutionAttemptID: identity.ExecutionAttemptID, LeaseID: identity.LeaseID,
		FenceToken: identity.FenceToken, IdempotencyKey: worker.IDs.New("artifact-create"), ArtifactID: artifactID,
		ExpectedChunkCount: chunkCount, ExpectedPlaintextLength: int64(len(output)), LogicalDigest: digest, CreatedAt: now,
	}); err != nil {
		return fmt.Errorf("cloud worker create output artifact: %w", err)
	}
	for index := 0; index < chunkCount; index++ {
		start := index * controlapi.MaximumArtifactChunkPlaintextBytes
		end := start + controlapi.MaximumArtifactChunkPlaintextBytes
		if end > len(output) {
			end = len(output)
		}
		plaintext := append([]byte(nil), output[start:end]...)
		chunkDigest := sha256.Sum256(plaintext)
		if _, err := worker.Control.AppendWorkerArtifactChunk(ctx, controlapi.WorkerArtifactChunkCommand{
			AccountID: identity.AccountID, WorkerID: identity.WorkerID, MachineID: identity.MachineID,
			TargetID: identity.TargetID, ExecutionAttemptID: identity.ExecutionAttemptID, LeaseID: identity.LeaseID,
			FenceToken: identity.FenceToken, IdempotencyKey: worker.IDs.New("artifact-chunk"), ArtifactID: artifactID,
			ChunkIndex: index, Plaintext: plaintext, PlaintextDigest: hex.EncodeToString(chunkDigest[:]), CreatedAt: worker.Clock().UTC(),
		}); err != nil {
			return fmt.Errorf("cloud worker append output chunk: %w", err)
		}
	}
	if _, err := worker.Control.FinalizeWorkerArtifact(ctx, controlapi.WorkerArtifactFinalizeCommand{
		AccountID: identity.AccountID, WorkerID: identity.WorkerID, MachineID: identity.MachineID,
		TargetID: identity.TargetID, ExecutionAttemptID: identity.ExecutionAttemptID, LeaseID: identity.LeaseID,
		FenceToken: identity.FenceToken, IdempotencyKey: worker.IDs.New("artifact-finalize"), ArtifactID: artifactID,
		FinalizedAt: worker.Clock().UTC(),
	}); err != nil {
		return fmt.Errorf("cloud worker finalize output artifact: %w", err)
	}
	receipt, err := json.Marshal(struct {
		Status   coreworker.TerminalStatus `json:"status"`
		ExitCode int                       `json:"exit_code"`
	}{Status: status, ExitCode: exitCode})
	if err != nil {
		return err
	}
	var message *string
	if status == coreworker.TerminalCompleted && int64(len(output)) <= assignment.InlineOutputPlaintextBytes {
		value := string(output)
		message = &value
	}
	if _, err := worker.Control.CommitWorkerTerminal(ctx, controlapi.WorkerTerminalCommand{
		AccountID: identity.AccountID, WorkerID: identity.WorkerID, MachineID: identity.MachineID,
		TargetID: identity.TargetID, ExecutionAttemptID: identity.ExecutionAttemptID, LeaseID: identity.LeaseID,
		FenceToken: identity.FenceToken, TerminalReceiptID: worker.IDs.New("receipt"),
		IdempotencyKey: worker.IDs.New("terminal"), Status: status, ReceiptPlaintext: receipt,
		Output:                 controlapi.WorkerOutputReference{ArtifactID: artifactID, Digest: digest},
		OutputMessagePlaintext: message, CommittedAt: worker.Clock().UTC(),
	}); err != nil {
		return fmt.Errorf("cloud worker commit terminal: %w", err)
	}
	return nil
}

func (worker *Worker) validate() error {
	if worker == nil || worker.Control == nil || worker.Runtime == nil || worker.Readiness == nil || worker.Adapters == nil ||
		worker.Clock == nil || worker.IDs == nil || worker.Identity.AccountID == "" || worker.Identity.WorkerID == "" || worker.Identity.MachineID == "" {
		return fmt.Errorf("%w: dependencies", ErrWorkerInvalid)
	}
	if worker.Heartbeat == nil {
		worker.Heartbeat = func(interval time.Duration) (<-chan time.Time, func()) {
			ticker := time.NewTicker(interval)
			return ticker.C, ticker.Stop
		}
	}
	return nil
}

func (worker *Worker) validateAssignment(assignment controlapi.WorkerAssignment, attemptID, leaseID, capabilityID string, now time.Time) error {
	expectedMessageKind := ""
	switch assignment.TargetKind {
	case "initial":
		expectedMessageKind = "agent"
	case "handoff":
		expectedMessageKind = "handoff_result"
	case "routine":
		expectedMessageKind = "routine_result"
	}
	if assignment.WorkerID != worker.Identity.WorkerID || assignment.MachineID != worker.Identity.MachineID ||
		assignment.Execution.ComputerID != worker.Identity.MachineID || assignment.ExecutionAttemptID != attemptID ||
		assignment.LeaseID != leaseID || assignment.CapabilityRevisionID != capabilityID || assignment.TargetID == "" ||
		assignment.FenceToken < 1 || assignment.Execution.Workdir == "" || assignment.Prompt == "" ||
		blank(assignment.OriginID, assignment.ContextManifestID, assignment.OutputConversationID, assignment.OutputAuthorAgentID) ||
		expectedMessageKind == "" || assignment.OutputMessageKind != expectedMessageKind ||
		assignment.OutputAuthorAgentID != assignment.Pins.AgentID ||
		assignment.MaximumOutputPlaintextBytes != controlapi.MaximumArtifactPlaintextBytes ||
		assignment.InlineOutputPlaintextBytes != controlapi.MaximumArtifactChunkPlaintextBytes ||
		assignment.HardDeadline.IsZero() || !now.Before(assignment.HardDeadline) || !now.Before(assignment.ExpiresAt) {
		return fmt.Errorf("%w: claimed assignment identity or deadline", ErrWorkerInvalid)
	}
	return nil
}

func validReadinessSnapshot(snapshot ReadinessSnapshot) bool {
	if snapshot.CapabilityRevisionID == "" || snapshot.Revision < 1 || len(snapshot.Evidence) == 0 || len(snapshot.Evidence) > 64<<10 || len(snapshot.EvidenceDigest) != 64 {
		return false
	}
	digest := sha256.Sum256(snapshot.Evidence)
	return hex.EncodeToString(digest[:]) == snapshot.EvidenceDigest
}

func (worker *Worker) heartbeatCommand(assignment controlapi.WorkerAssignment, now time.Time) controlapi.WorkerLeaseHeartbeatCommand {
	return controlapi.WorkerLeaseHeartbeatCommand{
		AccountID: worker.Identity.AccountID, WorkerID: worker.Identity.WorkerID, MachineID: worker.Identity.MachineID,
		TargetID: assignment.TargetID, ExecutionAttemptID: assignment.ExecutionAttemptID,
		LeaseID: assignment.LeaseID, FenceToken: assignment.FenceToken,
		IdempotencyKey: worker.IDs.New("heartbeat"), ObservedAt: now, ExtendUntil: now.Add(controlapi.DefaultWorkerLease),
	}
}

func (worker *Worker) heartbeat(ctx context.Context, assignment controlapi.WorkerAssignment, now time.Time) (controlapi.WorkerLeaseHeartbeatResult, error) {
	heartbeat, err := worker.Control.HeartbeatWorkerLease(ctx, worker.heartbeatCommand(assignment, now))
	if err != nil {
		if errors.Is(err, controlapi.ErrWorkerStaleLease) {
			return controlapi.WorkerLeaseHeartbeatResult{}, controlapi.ErrWorkerStaleLease
		}
		return controlapi.WorkerLeaseHeartbeatResult{}, fmt.Errorf("cloud worker heartbeat: %w", err)
	}
	if heartbeat.TargetID != assignment.TargetID || heartbeat.ExecutionAttemptID != assignment.ExecutionAttemptID ||
		heartbeat.LeaseID != assignment.LeaseID || heartbeat.FenceToken != assignment.FenceToken {
		return controlapi.WorkerLeaseHeartbeatResult{}, fmt.Errorf("%w: heartbeat changed exact fence", ErrWorkerInvalid)
	}
	return heartbeat, nil
}

func (worker *Worker) acknowledgeCancellation(ctx context.Context, assignment controlapi.WorkerAssignment, now time.Time) error {
	ackID := worker.IDs.New("cancel-ack")
	acknowledgement, err := worker.Control.AcknowledgeWorkerCancellation(ctx, controlapi.WorkerCancellationAckCommand{
		AccountID: worker.Identity.AccountID, WorkerID: worker.Identity.WorkerID, MachineID: worker.Identity.MachineID,
		TargetID: assignment.TargetID, ExecutionAttemptID: assignment.ExecutionAttemptID,
		LeaseID: assignment.LeaseID, FenceToken: assignment.FenceToken,
		AcknowledgementID: ackID, IdempotencyKey: ackID, AcknowledgedAt: now,
	})
	if err != nil {
		return fmt.Errorf("cloud worker acknowledge cancel: %w", err)
	}
	if acknowledgement.AcknowledgementID != ackID || acknowledgement.TargetID != assignment.TargetID ||
		acknowledgement.ExecutionAttemptID != assignment.ExecutionAttemptID || acknowledgement.LeaseID != assignment.LeaseID ||
		acknowledgement.FenceToken != assignment.FenceToken {
		return fmt.Errorf("%w: cancellation acknowledgement changed exact fence", ErrWorkerInvalid)
	}
	return nil
}

type artifactIdentity struct {
	Identity
	TargetID, ExecutionAttemptID, LeaseID string
	FenceToken                            int64
}

func (worker *Worker) artifactIdentity(assignment controlapi.WorkerAssignment) artifactIdentity {
	return artifactIdentity{Identity: worker.Identity, TargetID: assignment.TargetID,
		ExecutionAttemptID: assignment.ExecutionAttemptID, LeaseID: assignment.LeaseID, FenceToken: assignment.FenceToken}
}
