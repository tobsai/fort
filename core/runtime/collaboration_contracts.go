package runtime

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// HandoffEmitter is the only adapter seam allowed to translate one
// provider-native structured tool/action into Agent-initiated Handoff intent.
// It has no method for parsing prose or dispatching work; persistence and
// authority intersection remain Fort commands outside the adapter.
type HandoffEmitter interface {
	EmitHandoff(context.Context, HandoffEmitterRequest) (StructuredHandoffEmission, error)
}

type HandoffEmitterRequest struct {
	AdapterID                string `json:"adapter_id"`
	AdapterRevision          string `json:"adapter_revision"`
	InvocationID             string `json:"invocation_id"`
	AccountID                string `json:"account_id"`
	SourceAgentID            string `json:"source_agent_id"`
	SourceBehaviorRevisionID string `json:"source_behavior_revision_id"`
	SourceBindingRevisionID  string `json:"source_binding_revision_id"`
	StructuredPayload        []byte `json:"structured_payload"`
}

// StructuredHandoffEmission is adapter-validated intent only. Recipient
// revisions, output Conversation, delegation grant, policy intersection,
// target, lease, and lifecycle evidence are resolved and persisted by Fort.
type StructuredHandoffEmission struct {
	AdapterID                string    `json:"adapter_id"`
	AdapterRevision          string    `json:"adapter_revision"`
	InvocationID             string    `json:"invocation_id"`
	AccountID                string    `json:"account_id"`
	SourceAgentID            string    `json:"source_agent_id"`
	SourceBehaviorRevisionID string    `json:"source_behavior_revision_id"`
	SourceBindingRevisionID  string    `json:"source_binding_revision_id"`
	RecipientAgentID         string    `json:"recipient_agent_id"`
	ContextRecordIDs         []string  `json:"context_record_ids"`
	RequestedResult          string    `json:"requested_result"`
	ReplyToMessageID         string    `json:"reply_to_message_id,omitempty"`
	RequestedAuthority       []string  `json:"requested_authority"`
	EmittedAt                time.Time `json:"emitted_at"`
}

func (request HandoffEmitterRequest) Validate() error {
	if err := requireRuntimeIDs("Handoff emitter request", map[string]string{
		"adapter id": request.AdapterID, "adapter revision": request.AdapterRevision,
		"invocation id": request.InvocationID, "account id": request.AccountID,
		"source Agent id": request.SourceAgentID, "source Behavior Revision id": request.SourceBehaviorRevisionID,
		"source Binding Revision id": request.SourceBindingRevisionID,
	}); err != nil {
		return err
	}
	if len(request.StructuredPayload) == 0 {
		return fmt.Errorf("Handoff emitter requires a provider-native structured payload")
	}
	return nil
}

func (emission StructuredHandoffEmission) ValidateFor(request HandoffEmitterRequest) error {
	if err := request.Validate(); err != nil {
		return err
	}
	if err := requireRuntimeIDs("structured Handoff emission", map[string]string{
		"adapter id": emission.AdapterID, "adapter revision": emission.AdapterRevision,
		"invocation id": emission.InvocationID, "account id": emission.AccountID,
		"source Agent id": emission.SourceAgentID, "source Behavior Revision id": emission.SourceBehaviorRevisionID,
		"source Binding Revision id": emission.SourceBindingRevisionID, "recipient Agent id": emission.RecipientAgentID,
		"requested result": emission.RequestedResult,
	}); err != nil {
		return err
	}
	if emission.AdapterID != request.AdapterID || emission.AdapterRevision != request.AdapterRevision ||
		emission.InvocationID != request.InvocationID || emission.AccountID != request.AccountID ||
		emission.SourceAgentID != request.SourceAgentID ||
		emission.SourceBehaviorRevisionID != request.SourceBehaviorRevisionID ||
		emission.SourceBindingRevisionID != request.SourceBindingRevisionID {
		return fmt.Errorf("structured Handoff emission does not match its exact adapter and source revisions")
	}
	if emission.SourceAgentID == emission.RecipientAgentID {
		return fmt.Errorf("structured Handoff emission cannot address its source Agent")
	}
	if emission.EmittedAt.IsZero() {
		return fmt.Errorf("structured Handoff emission time is required")
	}
	if emission.ReplyToMessageID != "" && emission.ReplyToMessageID != strings.TrimSpace(emission.ReplyToMessageID) {
		return fmt.Errorf("structured Handoff reply message id is not normalized")
	}
	if emission.ContextRecordIDs == nil || emission.RequestedAuthority == nil {
		return fmt.Errorf("structured Handoff context and requested authority must be allocated lists")
	}
	if err := validateRuntimeValues("structured Handoff context record", emission.ContextRecordIDs); err != nil {
		return err
	}
	return validateRuntimeValues("structured Handoff authority", emission.RequestedAuthority)
}

// RoutineAuthority can inspect and fence a framework-native schedule before a
// Fort Routine import. Deliberately, it exposes no enqueue or execution method.
type RoutineAuthority interface {
	InspectSourceRoutine(context.Context, RoutineAuthorityInspectionRequest) (SourceRoutineAuthorityEvidence, error)
	FenceSourceRoutine(context.Context, RoutineAuthorityFenceRequest) (SourceRoutineFenceReceipt, error)
}

type RoutineAuthorityInspectionRequest struct {
	AccountID             string `json:"account_id"`
	ExecutionSourceID     string `json:"execution_source_id"`
	SourceAgentID         string `json:"source_agent_id"`
	OpaqueSourceRoutineID string `json:"opaque_source_routine_id"`
}

type SourceRoutineAuthorityEvidence struct {
	AccountID             string    `json:"account_id"`
	ExecutionSourceID     string    `json:"execution_source_id"`
	SourceAgentID         string    `json:"source_agent_id"`
	OpaqueSourceRoutineID string    `json:"opaque_source_routine_id"`
	ProjectionRevision    int       `json:"projection_revision"`
	Enabled               bool      `json:"enabled"`
	LastOccurrence        time.Time `json:"last_occurrence"`
	NextOccurrence        time.Time `json:"next_occurrence"`
	ObservedAt            time.Time `json:"observed_at"`
}

type RoutineAuthorityFenceRequest struct {
	Inspection  SourceRoutineAuthorityEvidence `json:"inspection"`
	RequestedAt time.Time                      `json:"requested_at"`
}

type SourceRoutineFenceReceipt struct {
	AccountID             string    `json:"account_id"`
	ExecutionSourceID     string    `json:"execution_source_id"`
	OpaqueSourceRoutineID string    `json:"opaque_source_routine_id"`
	ProjectionRevision    int       `json:"projection_revision"`
	FenceReceiptID        string    `json:"fence_receipt_id"`
	SourceDisabledAt      time.Time `json:"source_disabled_at"`
	LastOccurrence        time.Time `json:"last_occurrence"`
	NextOccurrence        time.Time `json:"next_occurrence"`
}

func (evidence SourceRoutineAuthorityEvidence) ValidateFor(request RoutineAuthorityInspectionRequest) error {
	if err := requireRuntimeIDs("Routine authority inspection", map[string]string{
		"account id": request.AccountID, "Execution Source id": request.ExecutionSourceID,
		"Source Agent id": request.SourceAgentID, "opaque source Routine id": request.OpaqueSourceRoutineID,
	}); err != nil {
		return err
	}
	if evidence.AccountID != request.AccountID || evidence.ExecutionSourceID != request.ExecutionSourceID ||
		evidence.SourceAgentID != request.SourceAgentID || evidence.OpaqueSourceRoutineID != request.OpaqueSourceRoutineID {
		return fmt.Errorf("source Routine authority evidence does not match the inspected source identity")
	}
	return evidence.validate()
}

func (evidence SourceRoutineAuthorityEvidence) validate() error {
	if err := requireRuntimeIDs("source Routine authority evidence", map[string]string{
		"account id": evidence.AccountID, "Execution Source id": evidence.ExecutionSourceID,
		"Source Agent id": evidence.SourceAgentID, "opaque source Routine id": evidence.OpaqueSourceRoutineID,
	}); err != nil {
		return err
	}
	if evidence.ProjectionRevision < 1 || evidence.LastOccurrence.IsZero() || evidence.NextOccurrence.IsZero() ||
		evidence.ObservedAt.IsZero() || !evidence.NextOccurrence.After(evidence.LastOccurrence) {
		return fmt.Errorf("source Routine authority evidence requires an exact revision, observation, and ordered occurrences")
	}
	return nil
}

func (request RoutineAuthorityFenceRequest) Validate() error {
	if err := request.Inspection.validate(); err != nil {
		return err
	}
	if !request.Inspection.Enabled {
		return fmt.Errorf("source Routine is already disabled")
	}
	if request.RequestedAt.IsZero() || request.RequestedAt.Before(request.Inspection.ObservedAt) ||
		!request.RequestedAt.Before(request.Inspection.NextOccurrence) {
		return fmt.Errorf("source Routine fencing must be requested after inspection and before its next occurrence")
	}
	return nil
}

func (receipt SourceRoutineFenceReceipt) ValidateFor(evidence SourceRoutineAuthorityEvidence) error {
	if err := evidence.validate(); err != nil {
		return err
	}
	if err := requireRuntimeIDs("source Routine fence receipt", map[string]string{
		"account id": receipt.AccountID, "Execution Source id": receipt.ExecutionSourceID,
		"opaque source Routine id": receipt.OpaqueSourceRoutineID, "fence receipt id": receipt.FenceReceiptID,
	}); err != nil {
		return err
	}
	if receipt.AccountID != evidence.AccountID || receipt.ExecutionSourceID != evidence.ExecutionSourceID ||
		receipt.OpaqueSourceRoutineID != evidence.OpaqueSourceRoutineID ||
		receipt.ProjectionRevision != evidence.ProjectionRevision ||
		!receipt.LastOccurrence.Equal(evidence.LastOccurrence) || !receipt.NextOccurrence.Equal(evidence.NextOccurrence) {
		return fmt.Errorf("source Routine fence receipt does not match the exact inspected occurrences")
	}
	if receipt.SourceDisabledAt.Before(evidence.ObservedAt) || !receipt.SourceDisabledAt.Before(evidence.NextOccurrence) {
		return fmt.Errorf("source Routine must be disabled after inspection and before its next occurrence")
	}
	return nil
}

func requireRuntimeIDs(subject string, values map[string]string) error {
	for label, value := range values {
		if strings.TrimSpace(value) == "" || value != strings.TrimSpace(value) {
			return fmt.Errorf("%s %s is required and normalized", subject, label)
		}
	}
	return nil
}

func validateRuntimeValues(subject string, values []string) error {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) == "" || value != strings.TrimSpace(value) {
			return fmt.Errorf("%s is required and normalized", subject)
		}
		if _, exists := seen[value]; exists {
			return fmt.Errorf("%s is duplicated", subject)
		}
		seen[value] = struct{}{}
	}
	return nil
}
