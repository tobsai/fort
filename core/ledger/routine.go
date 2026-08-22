package ledger

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/tobsai/fort/core/conversation"
)

type RoutinePauseReason string

const RoutinePauseNeedsRevalidation RoutinePauseReason = "needs_revalidation"

var (
	ErrRoutineNeedsRevalidation = errors.New("Routine needs revalidation")
	ErrRoutineRunTerminal       = errors.New("Routine run is already terminal")
)

// RoutineRecord is the complete current read model for one Agent-owned
// Routine. Revisions and import receipts remain append-only.
type RoutineRecord struct {
	Routine         conversation.Routine         `json:"routine"`
	CurrentRevision conversation.RoutineRevision `json:"current_revision"`
	PauseReason     RoutinePauseReason           `json:"pause_reason,omitempty"`
	ImportReceipt   *RoutineImportReceipt        `json:"import_receipt,omitempty"`
}

type CreateRoutineCommand struct {
	IdempotencyKey string                       `json:"idempotency_key"`
	Routine        conversation.Routine         `json:"routine"`
	Revision       conversation.RoutineRevision `json:"revision"`
}

func (c CreateRoutineCommand) Validate() error {
	if err := validateIdempotencyKey("Routine creation", c.IdempotencyKey); err != nil {
		return err
	}
	if err := c.Routine.Validate(c.Revision); err != nil {
		return err
	}
	if c.Routine.State != conversation.RoutineActive {
		return fmt.Errorf("new Routine must be active")
	}
	if c.Revision.Revision != 1 {
		return fmt.Errorf("new Routine must begin at revision 1")
	}
	return nil
}

func (c CreateRoutineCommand) Digest() (string, error) {
	canonical := c
	canonical.IdempotencyKey = ""
	canonical.Routine.ID = ""
	canonical.Routine.CurrentRevisionID = ""
	canonical.Routine.CreatedAt = time.Time{}
	canonical.Revision.ID = ""
	canonical.Revision.RoutineID = ""
	canonical.Revision.Revision = 0
	canonical.Revision.BehaviorRevisionID = ""
	canonical.Revision.BindingRevisionID = ""
	canonical.Revision.Authority = ""
	canonical.Revision.CreatedAt = time.Time{}
	return collaborationDigest(canonical)
}

// SourceRoutineProjection is immutable read-only evidence of one framework-
// native schedule observation. It is not an executable Fort Routine.
type SourceRoutineProjection struct {
	ID                    string          `json:"id"`
	AccountID             string          `json:"account_id"`
	ExecutionSourceID     string          `json:"execution_source_id"`
	OpaqueSourceRoutineID string          `json:"opaque_source_routine_id"`
	ProjectionRevision    int             `json:"projection_revision"`
	ScheduleSnapshot      json.RawMessage `json:"schedule_snapshot"`
	ProjectionDigest      string          `json:"projection_digest"`
	LastOccurrenceAt      time.Time       `json:"last_occurrence_at,omitempty"`
	NextOccurrenceAt      time.Time       `json:"next_occurrence_at,omitempty"`
	ObservedAt            time.Time       `json:"observed_at"`
}

func (p SourceRoutineProjection) Validate() error {
	if err := requireRoutineStrings("source Routine projection", map[string]string{
		"id": p.ID, "account id": p.AccountID, "Execution Source id": p.ExecutionSourceID,
		"opaque source Routine id": p.OpaqueSourceRoutineID,
	}); err != nil {
		return err
	}
	if p.ProjectionRevision < 1 || p.ObservedAt.IsZero() {
		return fmt.Errorf("source Routine projection revision and observation time are required")
	}
	var object map[string]any
	if err := json.Unmarshal(p.ScheduleSnapshot, &object); err != nil || object == nil {
		return fmt.Errorf("source Routine projection schedule snapshot must be a JSON object")
	}
	if !matchesSHA256(p.ScheduleSnapshot, p.ProjectionDigest) {
		return fmt.Errorf("source Routine projection digest does not match its exact snapshot")
	}
	if !p.LastOccurrenceAt.IsZero() && p.LastOccurrenceAt.After(p.ObservedAt) {
		return fmt.Errorf("source Routine last occurrence cannot follow observation")
	}
	if !p.NextOccurrenceAt.IsZero() && p.NextOccurrenceAt.Before(p.ObservedAt) {
		return fmt.Errorf("source Routine next occurrence cannot precede observation")
	}
	if !p.LastOccurrenceAt.IsZero() && !p.NextOccurrenceAt.IsZero() && !p.NextOccurrenceAt.After(p.LastOccurrenceAt) {
		return fmt.Errorf("source Routine occurrence evidence is out of order")
	}
	return nil
}

// RoutineImportReceipt proves source-native scheduling was disabled before
// Fort assumed fort_cloud authority.
type RoutineImportReceipt struct {
	ID                          string    `json:"id"`
	AccountID                   string    `json:"account_id"`
	SourceRoutineProjectionID   string    `json:"source_routine_projection_id"`
	RoutineID                   string    `json:"routine_id"`
	RoutineRevisionID           string    `json:"routine_revision_id"`
	SourceDisabledAt            time.Time `json:"source_disabled_at"`
	ExactLastSourceOccurrenceAt time.Time `json:"exact_last_source_occurrence_at,omitempty"`
	ExactNextSourceOccurrenceAt time.Time `json:"exact_next_source_occurrence_at,omitempty"`
	FencingReceiptCiphertext    []byte    `json:"fencing_receipt_ciphertext"`
	FencingReceiptKeyID         string    `json:"fencing_receipt_key_id"`
	FencingReceiptNonce         []byte    `json:"fencing_receipt_nonce"`
	FencingReceiptDigest        string    `json:"fencing_receipt_digest"`
	ImportedAt                  time.Time `json:"imported_at"`
}

func (r RoutineImportReceipt) Validate() error {
	if err := requireRoutineStrings("Routine import receipt", map[string]string{
		"id": r.ID, "account id": r.AccountID, "source projection id": r.SourceRoutineProjectionID,
		"Routine id": r.RoutineID, "Routine Revision id": r.RoutineRevisionID,
		"fencing key id": r.FencingReceiptKeyID,
	}); err != nil {
		return err
	}
	if r.SourceDisabledAt.IsZero() || r.ImportedAt.IsZero() || r.ImportedAt.Before(r.SourceDisabledAt) {
		return fmt.Errorf("Routine import requires ordered source-disable and import times")
	}
	if len(r.FencingReceiptCiphertext) == 0 || len(r.FencingReceiptNonce) < 12 ||
		!matchesSHA256(r.FencingReceiptCiphertext, r.FencingReceiptDigest) {
		return fmt.Errorf("Routine import requires a valid encrypted fencing receipt")
	}
	return nil
}

type ImportRoutineCommand struct {
	Create  CreateRoutineCommand `json:"create"`
	Receipt RoutineImportReceipt `json:"receipt"`
}

func (c ImportRoutineCommand) Validate(projection SourceRoutineProjection) error {
	if err := c.Create.Validate(); err != nil {
		return err
	}
	if err := c.Receipt.Validate(); err != nil {
		return err
	}
	if err := projection.Validate(); err != nil {
		return err
	}
	if c.Receipt.AccountID != c.Create.Routine.AccountID || projection.AccountID != c.Create.Routine.AccountID ||
		c.Receipt.SourceRoutineProjectionID != projection.ID || c.Receipt.RoutineID != c.Create.Routine.ID ||
		c.Receipt.RoutineRevisionID != c.Create.Revision.ID {
		return fmt.Errorf("Routine import owner, projection, Routine, and Revision evidence must match")
	}
	if !sameRoutineTime(c.Receipt.ExactLastSourceOccurrenceAt, projection.LastOccurrenceAt) ||
		!sameRoutineTime(c.Receipt.ExactNextSourceOccurrenceAt, projection.NextOccurrenceAt) {
		return fmt.Errorf("Routine import occurrence evidence does not match the source projection")
	}
	if c.Receipt.SourceDisabledAt.Before(projection.ObservedAt) {
		return fmt.Errorf("Routine source must be disabled after its exact schedule observation")
	}
	if !c.Receipt.ExactNextSourceOccurrenceAt.IsZero() &&
		!c.Receipt.SourceDisabledAt.Before(c.Receipt.ExactNextSourceOccurrenceAt) {
		return fmt.Errorf("Routine source must be disabled before its exact next occurrence")
	}
	if c.Create.Revision.Trigger == conversation.RoutineTriggerSchedule &&
		!sameRoutineTime(c.Create.Revision.NextOccurrence, c.Receipt.ExactNextSourceOccurrenceAt) {
		return fmt.Errorf("imported Routine next occurrence must match the fenced source occurrence")
	}
	return nil
}

func (c ImportRoutineCommand) Digest() (string, error) {
	canonical := c
	canonical.Create.IdempotencyKey = ""
	return collaborationDigest(canonical)
}

// RoutineOccurrence is the exact idempotent trigger materialization used by
// scheduled and Test Routine requests alike.
type RoutineOccurrence struct {
	ID                 string                       `json:"id"`
	AccountID          string                       `json:"account_id"`
	RoutineID          string                       `json:"routine_id"`
	RoutineRevisionID  string                       `json:"routine_revision_id"`
	Kind               conversation.RoutineRunKind  `json:"kind"`
	State              conversation.RoutineRunState `json:"state"`
	ScheduledFor       time.Time                    `json:"scheduled_for"`
	IdempotencyKey     string                       `json:"idempotency_key"`
	ApprovalEvidenceID string                       `json:"approval_evidence_id"`
	CreatedAt          time.Time                    `json:"created_at"`
	UpdatedAt          time.Time                    `json:"updated_at"`
}

// RoutineRunActivity is append-only attempt, lease, failure, and next-action
// evidence for one run transition.
type RoutineRunActivity struct {
	Sequence       int64                        `json:"sequence"`
	State          conversation.RoutineRunState `json:"state"`
	AttemptID      string                       `json:"attempt_id,omitempty"`
	LeaseID        string                       `json:"lease_id,omitempty"`
	LeaseExpiresAt time.Time                    `json:"lease_expires_at,omitempty"`
	Activity       string                       `json:"activity"`
	FailureCode    string                       `json:"failure_code,omitempty"`
	NextAction     string                       `json:"next_action,omitempty"`
	CreatedAt      time.Time                    `json:"created_at"`
}

type RoutineRunRecord struct {
	Occurrence           RoutineOccurrence       `json:"occurrence"`
	Run                  conversation.RoutineRun `json:"run"`
	ResultConversationID string                  `json:"result_conversation_id"`
	AttemptID            string                  `json:"attempt_id,omitempty"`
	LeaseID              string                  `json:"lease_id,omitempty"`
	LeaseExpiresAt       time.Time               `json:"lease_expires_at,omitempty"`
	FailureCode          string                  `json:"failure_code,omitempty"`
	NextAction           string                  `json:"next_action,omitempty"`
	Activities           []RoutineRunActivity    `json:"activities"`
}

type EnqueueRoutineOccurrenceCommand struct {
	AccountID          string                      `json:"account_id"`
	RoutineID          string                      `json:"routine_id"`
	RoutineRevisionID  string                      `json:"routine_revision_id"`
	OccurrenceID       string                      `json:"occurrence_id"`
	RunID              string                      `json:"run_id"`
	Kind               conversation.RoutineRunKind `json:"kind"`
	ScheduledFor       time.Time                   `json:"scheduled_for"`
	IdempotencyKey     string                      `json:"idempotency_key"`
	ApprovalEvidenceID string                      `json:"approval_evidence_id"`
	CreatedAt          time.Time                   `json:"created_at"`
}

func (c EnqueueRoutineOccurrenceCommand) Validate() error {
	if err := validateIdempotencyKey("Routine occurrence", c.IdempotencyKey); err != nil {
		return err
	}
	if err := requireRoutineStrings("Routine occurrence", map[string]string{
		"account id": c.AccountID, "Routine id": c.RoutineID, "Routine Revision id": c.RoutineRevisionID,
		"id": c.OccurrenceID, "run id": c.RunID, "approval evidence id": c.ApprovalEvidenceID,
	}); err != nil {
		return err
	}
	if c.Kind != conversation.RoutineRunScheduled && c.Kind != conversation.RoutineRunTest {
		return fmt.Errorf("Routine occurrence kind is invalid")
	}
	if c.ScheduledFor.IsZero() || c.CreatedAt.IsZero() {
		return fmt.Errorf("Routine occurrence schedule and creation times are required")
	}
	return nil
}

func (c EnqueueRoutineOccurrenceCommand) Digest() (string, error) {
	canonical := c
	canonical.IdempotencyKey = ""
	canonical.OccurrenceID = ""
	canonical.RunID = ""
	canonical.CreatedAt = time.Time{}
	if canonical.Kind == conversation.RoutineRunTest {
		canonical.RoutineRevisionID = ""
		canonical.ScheduledFor = time.Time{}
	}
	return collaborationDigest(canonical)
}

type AdvanceRoutineRunCommand struct {
	AccountID        string                       `json:"account_id"`
	RunID            string                       `json:"run_id"`
	IdempotencyKey   string                       `json:"idempotency_key"`
	FromState        conversation.RoutineRunState `json:"from_state"`
	ToState          conversation.RoutineRunState `json:"to_state"`
	AttemptID        string                       `json:"attempt_id,omitempty"`
	LeaseID          string                       `json:"lease_id,omitempty"`
	LeaseExpiresAt   time.Time                    `json:"lease_expires_at,omitempty"`
	Activity         string                       `json:"activity"`
	FailureCode      string                       `json:"failure_code,omitempty"`
	NextAction       string                       `json:"next_action,omitempty"`
	NormalizedResult string                       `json:"normalized_result,omitempty"`
	OccurredAt       time.Time                    `json:"occurred_at"`
}

func (c AdvanceRoutineRunCommand) Validate() error {
	if err := validateIdempotencyKey("Routine run activity", c.IdempotencyKey); err != nil {
		return err
	}
	if err := requireRoutineStrings("Routine run activity", map[string]string{
		"account id": c.AccountID, "run id": c.RunID, "activity": c.Activity,
	}); err != nil {
		return err
	}
	if !c.FromState.Valid() || !c.ToState.Valid() || c.FromState == c.ToState || c.OccurredAt.IsZero() {
		return fmt.Errorf("Routine run activity requires distinct valid states and an occurrence time")
	}
	if !validRoutineRunTransition(c.FromState, c.ToState) {
		return fmt.Errorf("Routine run transition from %q to %q is invalid", c.FromState, c.ToState)
	}
	needsLease := c.ToState == conversation.RoutineRunWorking || c.FromState == conversation.RoutineRunWorking
	if needsLease && (strings.TrimSpace(c.AttemptID) == "" || strings.TrimSpace(c.LeaseID) == "" ||
		c.LeaseExpiresAt.IsZero() || !c.LeaseExpiresAt.After(c.OccurredAt)) {
		return fmt.Errorf("working Routine run activity requires an exact unexpired attempt and lease")
	}
	if c.ToState == conversation.RoutineRunNeedsYou && (strings.TrimSpace(c.FailureCode) == "" || strings.TrimSpace(c.NextAction) == "") {
		return fmt.Errorf("Needs You Routine run activity requires failure and next-action evidence")
	}
	if c.ToState == conversation.RoutineRunFailed && strings.TrimSpace(c.FailureCode) == "" {
		return fmt.Errorf("failed Routine run activity requires failure evidence")
	}
	if c.ToState == conversation.RoutineRunSucceeded {
		if strings.TrimSpace(c.NormalizedResult) == "" {
			return fmt.Errorf("successful Routine run requires a normalized result")
		}
	} else if strings.TrimSpace(c.NormalizedResult) != "" {
		return fmt.Errorf("only a successful Routine run may persist a normalized result")
	}
	return nil
}

func (c AdvanceRoutineRunCommand) Digest() (string, error) {
	canonical := c
	canonical.IdempotencyKey = ""
	return collaborationDigest(canonical)
}

// RevalidateRoutineCommand appends one successor Routine Revision after an
// Agent Behavior or Binding change. It never rewrites the prior revision.
type RevalidateRoutineCommand struct {
	AccountID      string                       `json:"account_id"`
	RoutineID      string                       `json:"routine_id"`
	IdempotencyKey string                       `json:"idempotency_key"`
	Revision       conversation.RoutineRevision `json:"revision"`
}

func (c RevalidateRoutineCommand) Validate(record RoutineRecord) error {
	if err := validateIdempotencyKey("Routine revalidation", c.IdempotencyKey); err != nil {
		return err
	}
	if err := requireRoutineStrings("Routine revalidation", map[string]string{
		"account id": c.AccountID, "Routine id": c.RoutineID,
	}); err != nil {
		return err
	}
	if record.Routine.AccountID != c.AccountID || record.Routine.ID != c.RoutineID ||
		record.Routine.State != conversation.RoutinePaused || record.PauseReason != RoutinePauseNeedsRevalidation {
		return fmt.Errorf("Routine is not paused for revalidation")
	}
	candidate := record.Routine
	candidate.CurrentRevisionID = c.Revision.ID
	candidate.State = conversation.RoutineActive
	if err := candidate.Validate(c.Revision); err != nil {
		return err
	}
	if c.Revision.Revision != record.CurrentRevision.Revision+1 {
		return fmt.Errorf("Routine revalidation must append the next immutable revision")
	}
	return nil
}

func (c RevalidateRoutineCommand) Digest() (string, error) {
	canonical := c
	canonical.IdempotencyKey = ""
	canonical.Revision.ID = ""
	canonical.Revision.Revision = 0
	canonical.Revision.BehaviorRevisionID = ""
	canonical.Revision.BindingRevisionID = ""
	canonical.Revision.Authority = ""
	canonical.Revision.CreatedAt = time.Time{}
	return collaborationDigest(canonical)
}

// RoutineRepository is the shared SQLite/Postgres persistence seam for
// Agent-owned Routines. List operations return allocated empty slices.
type RoutineRepository interface {
	AgentRepository
	RecordSourceRoutineProjection(context.Context, SourceRoutineProjection) (SourceRoutineProjection, error)
	ListSourceRoutineProjections(context.Context, string, string) ([]SourceRoutineProjection, error)
	CreateRoutine(context.Context, CreateRoutineCommand) (RoutineRecord, error)
	ImportSourceRoutine(context.Context, ImportRoutineCommand) (RoutineRecord, error)
	GetRoutine(context.Context, string, string) (RoutineRecord, error)
	ListRoutines(context.Context, string, string) ([]RoutineRecord, error)
	EnqueueRoutineOccurrence(context.Context, EnqueueRoutineOccurrenceCommand) (RoutineRunRecord, error)
	AdvanceRoutineRun(context.Context, AdvanceRoutineRunCommand) (RoutineRunRecord, error)
	GetRoutineRun(context.Context, string, string) (RoutineRunRecord, error)
	ListRoutineRuns(context.Context, string, string) ([]RoutineRunRecord, error)
	RevalidateRoutine(context.Context, RevalidateRoutineCommand) (RoutineRecord, error)
}

func requireRoutineStrings(subject string, fields map[string]string) error {
	for name, value := range fields {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s %s is required", subject, name)
		}
	}
	return nil
}

func matchesSHA256(value []byte, digest string) bool {
	if len(digest) != sha256.Size*2 || digest != strings.ToLower(digest) {
		return false
	}
	want := sha256.Sum256(value)
	return hex.EncodeToString(want[:]) == digest
}

func sameRoutineTime(left, right time.Time) bool {
	return (left.IsZero() && right.IsZero()) || left.Equal(right)
}

func validRoutineRunTransition(from, to conversation.RoutineRunState) bool {
	switch from {
	case conversation.RoutineRunQueued:
		return to == conversation.RoutineRunWorking || to == conversation.RoutineRunNeedsYou ||
			to == conversation.RoutineRunFailed || to == conversation.RoutineRunCanceled
	case conversation.RoutineRunWorking:
		return to == conversation.RoutineRunNeedsYou || to == conversation.RoutineRunSucceeded ||
			to == conversation.RoutineRunFailed || to == conversation.RoutineRunCanceled
	case conversation.RoutineRunNeedsYou:
		return to == conversation.RoutineRunQueued || to == conversation.RoutineRunCanceled
	default:
		return false
	}
}
