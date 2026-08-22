package runtime_test

import (
	"context"
	"testing"
	"time"

	"github.com/tobsai/fort/core/runtime"
)

func TestHandoffEmitterReturnsOneStructuredSourcePinnedCommand(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 18, 0, 0, 0, time.UTC)
	request := runtime.HandoffEmitterRequest{
		AdapterID: "adapter:openclaw", AdapterRevision: "adapter-revision:1",
		InvocationID: "invocation:41", AccountID: "account:one", SourceAgentID: "agent:researcher",
		SourceBehaviorRevisionID: "behavior:researcher:2", SourceBindingRevisionID: "binding:researcher:3",
		StructuredPayload: []byte(`{"recipient_option_id":"recipient:builder","request":"Review."}`),
	}
	want := runtime.StructuredHandoffEmission{
		AdapterID: request.AdapterID, AdapterRevision: request.AdapterRevision, InvocationID: request.InvocationID,
		AccountID: request.AccountID, SourceAgentID: request.SourceAgentID,
		SourceBehaviorRevisionID: request.SourceBehaviorRevisionID, SourceBindingRevisionID: request.SourceBindingRevisionID,
		RecipientAgentID: "agent:builder", ContextRecordIDs: []string{"message:41"},
		RequestedResult: "Review the evidence.", RequestedAuthority: []string{"read"}, EmittedAt: now,
	}
	var emitter runtime.HandoffEmitter = fakeHandoffEmitter{emission: want}

	got, err := emitter.EmitHandoff(context.Background(), request)
	if err != nil {
		t.Fatalf("EmitHandoff: %v", err)
	}
	if err := got.ValidateFor(request); err != nil {
		t.Fatalf("structured emission validation: %v", err)
	}
	if got.RecipientAgentID != "agent:builder" || len(got.ContextRecordIDs) != 1 {
		t.Fatalf("structured emission lost exact recipient/context: %+v", got)
	}
}

func TestStructuredHandoffEmissionRejectsProseLikeOrUnpinnedEvidence(t *testing.T) {
	t.Parallel()

	request, emission := validHandoffEmission()
	emission.AdapterRevision = "adapter-revision:other"
	if err := emission.ValidateFor(request); err == nil {
		t.Fatal("emission accepted a different adapter revision")
	}

	_, emission = validHandoffEmission()
	emission.RecipientAgentID = emission.SourceAgentID
	if err := emission.ValidateFor(request); err == nil {
		t.Fatal("emission accepted a self-Handoff")
	}

	_, emission = validHandoffEmission()
	emission.ContextRecordIDs = []string{"message:41", "message:41"}
	if err := emission.ValidateFor(request); err == nil {
		t.Fatal("emission accepted duplicate context evidence")
	}

	request, emission = validHandoffEmission()
	request.StructuredPayload = nil
	if err := emission.ValidateFor(request); err == nil {
		t.Fatal("emission accepted an absent provider-native structured payload")
	}
}

func TestRoutineAuthorityInspectsAndFencesWithoutAnEnqueueCapability(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 18, 0, 0, 0, time.UTC)
	request := runtime.RoutineAuthorityInspectionRequest{
		AccountID: "account:one", ExecutionSourceID: "source:studio", SourceAgentID: "source-agent:researcher",
		OpaqueSourceRoutineID: "weekly-research",
	}
	evidence := runtime.SourceRoutineAuthorityEvidence{
		AccountID: request.AccountID, ExecutionSourceID: request.ExecutionSourceID, SourceAgentID: request.SourceAgentID,
		OpaqueSourceRoutineID: request.OpaqueSourceRoutineID, ProjectionRevision: 4, Enabled: true,
		LastOccurrence: now.Add(-7 * 24 * time.Hour), NextOccurrence: now.Add(7 * 24 * time.Hour), ObservedAt: now,
	}
	receipt := runtime.SourceRoutineFenceReceipt{
		AccountID: request.AccountID, ExecutionSourceID: request.ExecutionSourceID,
		OpaqueSourceRoutineID: request.OpaqueSourceRoutineID, ProjectionRevision: evidence.ProjectionRevision,
		FenceReceiptID: "fence:weekly-research:4", SourceDisabledAt: now.Add(time.Minute),
		LastOccurrence: evidence.LastOccurrence, NextOccurrence: evidence.NextOccurrence,
	}
	var authority runtime.RoutineAuthority = fakeRoutineAuthority{evidence: evidence, receipt: receipt}

	inspected, err := authority.InspectSourceRoutine(context.Background(), request)
	if err != nil || inspected.ValidateFor(request) != nil {
		t.Fatalf("InspectSourceRoutine = %+v, %v", inspected, err)
	}
	fenced, err := authority.FenceSourceRoutine(context.Background(), runtime.RoutineAuthorityFenceRequest{
		Inspection: inspected, RequestedAt: now.Add(30 * time.Second),
	})
	if err != nil || fenced.ValidateFor(inspected) != nil {
		t.Fatalf("FenceSourceRoutine = %+v, %v", fenced, err)
	}
}

func TestRoutineAuthorityReceiptMustMatchExactObservedOccurrencesAndFenceBeforeNext(t *testing.T) {
	t.Parallel()

	request, evidence, receipt := validRoutineAuthorityEvidence()
	if err := evidence.ValidateFor(request); err != nil {
		t.Fatalf("valid evidence: %v", err)
	}
	receipt.NextOccurrence = receipt.NextOccurrence.Add(time.Hour)
	if err := receipt.ValidateFor(evidence); err == nil {
		t.Fatal("fence receipt accepted a changed next occurrence")
	}

	_, evidence, receipt = validRoutineAuthorityEvidence()
	receipt.SourceDisabledAt = evidence.NextOccurrence
	if err := receipt.ValidateFor(evidence); err == nil {
		t.Fatal("fence receipt accepted disabling at the next occurrence boundary")
	}
}

type fakeHandoffEmitter struct {
	emission runtime.StructuredHandoffEmission
}

func (fake fakeHandoffEmitter) EmitHandoff(context.Context, runtime.HandoffEmitterRequest) (runtime.StructuredHandoffEmission, error) {
	return fake.emission, nil
}

type fakeRoutineAuthority struct {
	evidence runtime.SourceRoutineAuthorityEvidence
	receipt  runtime.SourceRoutineFenceReceipt
}

func (fake fakeRoutineAuthority) InspectSourceRoutine(context.Context, runtime.RoutineAuthorityInspectionRequest) (runtime.SourceRoutineAuthorityEvidence, error) {
	return fake.evidence, nil
}

func (fake fakeRoutineAuthority) FenceSourceRoutine(context.Context, runtime.RoutineAuthorityFenceRequest) (runtime.SourceRoutineFenceReceipt, error) {
	return fake.receipt, nil
}

func validHandoffEmission() (runtime.HandoffEmitterRequest, runtime.StructuredHandoffEmission) {
	now := time.Date(2026, 8, 21, 18, 0, 0, 0, time.UTC)
	request := runtime.HandoffEmitterRequest{
		AdapterID: "adapter:openclaw", AdapterRevision: "adapter-revision:1", InvocationID: "invocation:41",
		AccountID: "account:one", SourceAgentID: "agent:researcher",
		SourceBehaviorRevisionID: "behavior:researcher:2", SourceBindingRevisionID: "binding:researcher:3",
		StructuredPayload: []byte(`{"recipient_option_id":"recipient:builder"}`),
	}
	emission := runtime.StructuredHandoffEmission{
		AdapterID: request.AdapterID, AdapterRevision: request.AdapterRevision, InvocationID: request.InvocationID,
		AccountID: request.AccountID, SourceAgentID: request.SourceAgentID,
		SourceBehaviorRevisionID: request.SourceBehaviorRevisionID, SourceBindingRevisionID: request.SourceBindingRevisionID,
		RecipientAgentID: "agent:builder", ContextRecordIDs: []string{"message:41"},
		RequestedResult: "Review.", RequestedAuthority: []string{"read"}, EmittedAt: now,
	}
	return request, emission
}

func validRoutineAuthorityEvidence() (runtime.RoutineAuthorityInspectionRequest, runtime.SourceRoutineAuthorityEvidence, runtime.SourceRoutineFenceReceipt) {
	now := time.Date(2026, 8, 21, 18, 0, 0, 0, time.UTC)
	request := runtime.RoutineAuthorityInspectionRequest{
		AccountID: "account:one", ExecutionSourceID: "source:studio", SourceAgentID: "source-agent:researcher",
		OpaqueSourceRoutineID: "weekly-research",
	}
	evidence := runtime.SourceRoutineAuthorityEvidence{
		AccountID: request.AccountID, ExecutionSourceID: request.ExecutionSourceID, SourceAgentID: request.SourceAgentID,
		OpaqueSourceRoutineID: request.OpaqueSourceRoutineID, ProjectionRevision: 2, Enabled: true,
		LastOccurrence: now.Add(-time.Hour), NextOccurrence: now.Add(time.Hour), ObservedAt: now,
	}
	receipt := runtime.SourceRoutineFenceReceipt{
		AccountID: request.AccountID, ExecutionSourceID: request.ExecutionSourceID,
		OpaqueSourceRoutineID: request.OpaqueSourceRoutineID, ProjectionRevision: evidence.ProjectionRevision,
		FenceReceiptID: "fence:weekly:2", SourceDisabledAt: now.Add(time.Minute),
		LastOccurrence: evidence.LastOccurrence, NextOccurrence: evidence.NextOccurrence,
	}
	return request, evidence, receipt
}
