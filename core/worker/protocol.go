// Package worker models Fort's deterministic cloud worker protocol. It owns
// no database, network, clock, or runtime integration.
package worker

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrInvalid               = errors.New("worker protocol command is invalid")
	ErrAlreadyLeased         = errors.New("target already has a lease")
	ErrStaleAttempt          = errors.New("worker attempt is stale")
	ErrNeedsAttention        = errors.New("target recovery needs human attention")
	ErrIdempotencyConflict   = errors.New("worker protocol idempotency key conflicts with an earlier command")
	ErrTerminalReceiptExists = errors.New("attempt already has a terminal receipt")
)

type MachineState string

const (
	MachineEnrolled MachineState = "enrolled"
	MachineRevoked  MachineState = "revoked"
)

// EnrolledMachine is the account-scoped worker identity allowed to claim work.
type EnrolledMachine struct {
	ID         string       `json:"id"`
	AccountID  string       `json:"account_id"`
	State      MachineState `json:"state"`
	EnrolledAt time.Time    `json:"enrolled_at"`
}

func (m EnrolledMachine) Validate(at time.Time) error {
	if blank(m.ID) || blank(m.AccountID) || m.EnrolledAt.IsZero() {
		return fmt.Errorf("%w: enrolled machine identity and enrollment time are required", ErrInvalid)
	}
	if m.State != MachineEnrolled {
		return fmt.Errorf("%w: machine %q is not enrolled", ErrInvalid, m.ID)
	}
	if at.Before(m.EnrolledAt) {
		return fmt.Errorf("%w: claim predates machine enrollment", ErrInvalid)
	}
	return nil
}

// AuthoritySnapshot is the exact effective grant accepted for one Target. It
// contains values, rather than only a mutable authority identifier.
type AuthoritySnapshot struct {
	ID               string   `json:"id"`
	Revision         string   `json:"revision"`
	Permissions      []string `json:"permissions"`
	ContextRecordIDs []string `json:"context_record_ids"`
}

func (s AuthoritySnapshot) validate() error {
	if blank(s.ID) || blank(s.Revision) {
		return fmt.Errorf("%w: effective authority identity and revision are required", ErrInvalid)
	}
	if err := uniqueNonBlank("authority permission", s.Permissions); err != nil {
		return err
	}
	return uniqueNonBlank("authority context record id", s.ContextRecordIDs)
}

func (s AuthoritySnapshot) clone() AuthoritySnapshot {
	s.Permissions = append([]string(nil), s.Permissions...)
	s.ContextRecordIDs = append([]string(nil), s.ContextRecordIDs...)
	return s
}

// ExecutionPins are immutable execution identity and authority evidence.
type ExecutionPins struct {
	AgentID                    string            `json:"agent_id"`
	BehaviorRevisionID         string            `json:"behavior_revision_id"`
	BindingRevisionID          string            `json:"binding_revision_id"`
	SeatID                     string            `json:"seat_id"`
	EffectiveAuthoritySnapshot AuthoritySnapshot `json:"effective_authority_snapshot"`
}

func (p ExecutionPins) validate() error {
	if blank(p.AgentID) || blank(p.BehaviorRevisionID) || blank(p.BindingRevisionID) || blank(p.SeatID) {
		return fmt.Errorf("%w: Agent, Behavior Revision, Binding Revision, and seat pins are required", ErrInvalid)
	}
	return p.EffectiveAuthoritySnapshot.validate()
}

func (p ExecutionPins) clone() ExecutionPins {
	p.EffectiveAuthoritySnapshot = p.EffectiveAuthoritySnapshot.clone()
	return p
}

type TargetSpec struct {
	ID        string        `json:"id"`
	AccountID string        `json:"account_id"`
	Pins      ExecutionPins `json:"pins"`
}

type TargetState string

const (
	TargetQueued                    TargetState = "queued"
	TargetLeased                    TargetState = "leased"
	TargetWorking                   TargetState = "working"
	TargetCancelRequested           TargetState = "cancel_requested"
	TargetRecoverableNeedsAttention TargetState = "recoverable_needs_attention"
	TargetCompleted                 TargetState = "completed"
	TargetFailed                    TargetState = "failed"
	TargetCanceled                  TargetState = "canceled"
)

// Target is one queued request and at most one worker attempt lease.
type Target struct {
	spec            TargetSpec
	state           TargetState
	claim           *ClaimCommand
	lease           *Lease
	cancellation    *Cancellation
	recovery        *Recovery
	terminalReceipt *TerminalReceipt
}

func NewTarget(spec TargetSpec) (*Target, error) {
	if blank(spec.ID) || blank(spec.AccountID) {
		return nil, fmt.Errorf("%w: target and account identities are required", ErrInvalid)
	}
	if err := spec.Pins.validate(); err != nil {
		return nil, err
	}
	spec.Pins = spec.Pins.clone()
	return &Target{spec: spec, state: TargetQueued}, nil
}

type ClaimCommand struct {
	TargetID       string    `json:"target_id"`
	AttemptID      string    `json:"attempt_id"`
	MachineID      string    `json:"machine_id"`
	IdempotencyKey string    `json:"idempotency_key"`
	ClaimedAt      time.Time `json:"claimed_at"`
	ExpiresAt      time.Time `json:"expires_at"`
}

type LeaseState string

const (
	LeaseClaimed         LeaseState = "claimed"
	LeaseWorking         LeaseState = "working"
	LeaseCancelRequested LeaseState = "cancel_requested"
	LeaseExpired         LeaseState = "expired"
	LeaseCompleted       LeaseState = "completed"
	LeaseFailed          LeaseState = "failed"
	LeaseCanceled        LeaseState = "canceled"
)

type Lease struct {
	TargetID  string        `json:"target_id"`
	AttemptID string        `json:"attempt_id"`
	MachineID string        `json:"machine_id"`
	Pins      ExecutionPins `json:"pins"`
	State     LeaseState    `json:"state"`
	ClaimedAt time.Time     `json:"claimed_at"`
	ExpiresAt time.Time     `json:"expires_at"`
}

func (l Lease) clone() Lease {
	l.Pins = l.Pins.clone()
	return l
}

func (t *Target) Claim(machine EnrolledMachine, command ClaimCommand) (Lease, error) {
	if t == nil {
		return Lease{}, fmt.Errorf("%w: target is required", ErrInvalid)
	}
	if blank(command.TargetID) || blank(command.AttemptID) || blank(command.MachineID) ||
		blank(command.IdempotencyKey) || command.ClaimedAt.IsZero() || !command.ExpiresAt.After(command.ClaimedAt) {
		return Lease{}, fmt.Errorf("%w: complete claim identity and a future expiry are required", ErrInvalid)
	}
	if command.TargetID != t.spec.ID || command.MachineID != machine.ID || machine.AccountID != t.spec.AccountID {
		return Lease{}, fmt.Errorf("%w: claim target, machine, or account does not match", ErrInvalid)
	}
	if err := machine.Validate(command.ClaimedAt); err != nil {
		return Lease{}, err
	}
	if t.claim != nil {
		if sameClaim(*t.claim, command) {
			return t.lease.clone(), nil
		}
		if t.claim.IdempotencyKey == command.IdempotencyKey {
			return Lease{}, ErrIdempotencyConflict
		}
	}
	if t.state == TargetRecoverableNeedsAttention {
		return Lease{}, ErrNeedsAttention
	}
	if t.state != TargetQueued {
		return Lease{}, ErrAlreadyLeased
	}
	lease := Lease{
		TargetID: command.TargetID, AttemptID: command.AttemptID, MachineID: command.MachineID,
		Pins: t.spec.Pins.clone(), State: LeaseClaimed, ClaimedAt: command.ClaimedAt, ExpiresAt: command.ExpiresAt,
	}
	storedClaim := command
	t.claim = &storedClaim
	t.lease = &lease
	t.state = TargetLeased
	return lease.clone(), nil
}

func sameClaim(left, right ClaimCommand) bool {
	return left.TargetID == right.TargetID && left.AttemptID == right.AttemptID &&
		left.MachineID == right.MachineID && left.IdempotencyKey == right.IdempotencyKey &&
		left.ClaimedAt.Equal(right.ClaimedAt) && left.ExpiresAt.Equal(right.ExpiresAt)
}

type WorkerDirective string

const (
	DirectiveContinue WorkerDirective = "continue"
	DirectiveCancel   WorkerDirective = "cancel"
)

type HeartbeatCommand struct {
	TargetID    string    `json:"target_id"`
	AttemptID   string    `json:"attempt_id"`
	MachineID   string    `json:"machine_id"`
	ObservedAt  time.Time `json:"observed_at"`
	ExtendUntil time.Time `json:"extend_until"`
}

type HeartbeatResult struct {
	Lease     Lease           `json:"lease"`
	Directive WorkerDirective `json:"directive"`
}

type RecoveryReason string

const (
	RecoveryLeaseExpired RecoveryReason = "lease_expired"
)

// Recovery preserves the exact lease that requires an explicit human
// decision. It is not a new queue entry and cannot be claimed automatically.
type Recovery struct {
	TargetID   string         `json:"target_id"`
	AttemptID  string         `json:"attempt_id"`
	MachineID  string         `json:"machine_id"`
	Reason     RecoveryReason `json:"reason"`
	ExpiresAt  time.Time      `json:"expires_at"`
	ObservedAt time.Time      `json:"observed_at"`
}

type TargetSnapshot struct {
	ID              string           `json:"id"`
	AccountID       string           `json:"account_id"`
	State           TargetState      `json:"state"`
	Pins            ExecutionPins    `json:"pins"`
	Lease           *Lease           `json:"lease,omitempty"`
	Cancellation    *Cancellation    `json:"cancellation,omitempty"`
	Recovery        *Recovery        `json:"recovery,omitempty"`
	TerminalReceipt *TerminalReceipt `json:"terminal_receipt,omitempty"`
}

func (t *Target) Snapshot() TargetSnapshot {
	if t == nil {
		return TargetSnapshot{}
	}
	snapshot := TargetSnapshot{
		ID: t.spec.ID, AccountID: t.spec.AccountID, State: t.state, Pins: t.spec.Pins.clone(),
	}
	if t.lease != nil {
		lease := t.lease.clone()
		snapshot.Lease = &lease
	}
	if t.cancellation != nil {
		cancellation := *t.cancellation
		snapshot.Cancellation = &cancellation
	}
	if t.recovery != nil {
		recovery := *t.recovery
		snapshot.Recovery = &recovery
	}
	if t.terminalReceipt != nil {
		receipt := *t.terminalReceipt
		snapshot.TerminalReceipt = &receipt
	}
	return snapshot
}

func (t *Target) Heartbeat(command HeartbeatCommand) (HeartbeatResult, error) {
	if t == nil || t.lease == nil {
		return HeartbeatResult{}, ErrStaleAttempt
	}
	if command.TargetID != t.lease.TargetID || command.AttemptID != t.lease.AttemptID || command.MachineID != t.lease.MachineID {
		return HeartbeatResult{}, ErrStaleAttempt
	}
	if command.ObservedAt.IsZero() || command.ExtendUntil.IsZero() || !command.ExtendUntil.After(command.ObservedAt) {
		return HeartbeatResult{}, fmt.Errorf("%w: heartbeat time and future extension are required", ErrInvalid)
	}
	if t.state != TargetLeased && t.state != TargetWorking && t.state != TargetCancelRequested {
		return HeartbeatResult{}, ErrStaleAttempt
	}
	if !command.ObservedAt.Before(t.lease.ExpiresAt) {
		t.expire(command.ObservedAt)
		return HeartbeatResult{}, ErrStaleAttempt
	}
	if command.ExtendUntil.Before(t.lease.ExpiresAt) {
		return HeartbeatResult{}, fmt.Errorf("%w: heartbeat cannot shorten a lease", ErrInvalid)
	}
	directive := DirectiveContinue
	if t.state == TargetCancelRequested {
		t.lease.State = LeaseCancelRequested
		directive = DirectiveCancel
	} else {
		t.lease.State = LeaseWorking
		t.state = TargetWorking
	}
	t.lease.ExpiresAt = command.ExtendUntil
	return HeartbeatResult{Lease: t.lease.clone(), Directive: directive}, nil
}

type CancelCommand struct {
	TargetID       string    `json:"target_id"`
	AttemptID      string    `json:"attempt_id"`
	MachineID      string    `json:"machine_id"`
	IdempotencyKey string    `json:"idempotency_key"`
	RequestedAt    time.Time `json:"requested_at"`
	Reason         string    `json:"reason"`
}

type Cancellation struct {
	TargetID       string    `json:"target_id"`
	AttemptID      string    `json:"attempt_id"`
	MachineID      string    `json:"machine_id"`
	IdempotencyKey string    `json:"idempotency_key"`
	RequestedAt    time.Time `json:"requested_at"`
	Reason         string    `json:"reason"`
}

func (t *Target) RequestCancel(command CancelCommand) (Cancellation, error) {
	if t == nil || t.lease == nil || command.TargetID != t.lease.TargetID ||
		command.AttemptID != t.lease.AttemptID || command.MachineID != t.lease.MachineID {
		return Cancellation{}, ErrStaleAttempt
	}
	if blank(command.IdempotencyKey) || blank(command.Reason) || command.RequestedAt.IsZero() || command.RequestedAt.Before(t.lease.ClaimedAt) {
		return Cancellation{}, fmt.Errorf("%w: cancellation idempotency, reason, and request time are required", ErrInvalid)
	}
	if t.cancellation != nil {
		if sameCancellation(*t.cancellation, command) {
			return *t.cancellation, nil
		}
		return Cancellation{}, ErrIdempotencyConflict
	}
	if t.state != TargetLeased && t.state != TargetWorking {
		return Cancellation{}, ErrStaleAttempt
	}
	if !command.RequestedAt.Before(t.lease.ExpiresAt) {
		t.expire(command.RequestedAt)
		return Cancellation{}, ErrStaleAttempt
	}
	cancellation := Cancellation(command)
	t.cancellation = &cancellation
	t.lease.State = LeaseCancelRequested
	t.state = TargetCancelRequested
	return cancellation, nil
}

func sameCancellation(cancellation Cancellation, command CancelCommand) bool {
	return cancellation.TargetID == command.TargetID && cancellation.AttemptID == command.AttemptID &&
		cancellation.MachineID == command.MachineID && cancellation.IdempotencyKey == command.IdempotencyKey &&
		cancellation.RequestedAt.Equal(command.RequestedAt) && cancellation.Reason == command.Reason
}

type TerminalStatus string

const (
	TerminalCompleted TerminalStatus = "completed"
	TerminalFailed    TerminalStatus = "failed"
	TerminalCanceled  TerminalStatus = "canceled"
)

func (s TerminalStatus) valid() bool {
	return s == TerminalCompleted || s == TerminalFailed || s == TerminalCanceled
}

// TerminalReceipt is the single normalized outcome accepted for an attempt.
// ResultDigest identifies the bounded persisted terminal payload/output.
type TerminalReceipt struct {
	TargetID       string         `json:"target_id"`
	AttemptID      string         `json:"attempt_id"`
	MachineID      string         `json:"machine_id"`
	IdempotencyKey string         `json:"idempotency_key"`
	Status         TerminalStatus `json:"status"`
	ResultDigest   string         `json:"result_digest"`
	CommittedAt    time.Time      `json:"committed_at"`
}

// CommitTerminal records exactly one terminal receipt. An exact replay is a
// successful no-op; every competing receipt conflicts.
func (t *Target) CommitTerminal(receipt TerminalReceipt) (TerminalReceipt, bool, error) {
	if t == nil || t.lease == nil || receipt.TargetID != t.lease.TargetID ||
		receipt.AttemptID != t.lease.AttemptID || receipt.MachineID != t.lease.MachineID {
		return TerminalReceipt{}, false, ErrStaleAttempt
	}
	if t.terminalReceipt != nil {
		if sameTerminalReceipt(*t.terminalReceipt, receipt) {
			return *t.terminalReceipt, false, nil
		}
		if receipt.IdempotencyKey == t.terminalReceipt.IdempotencyKey {
			return TerminalReceipt{}, false, ErrIdempotencyConflict
		}
		return TerminalReceipt{}, false, ErrTerminalReceiptExists
	}
	if blank(receipt.IdempotencyKey) || !receipt.Status.valid() || !lowerSHA256(receipt.ResultDigest) ||
		receipt.CommittedAt.IsZero() || receipt.CommittedAt.Before(t.lease.ClaimedAt) {
		return TerminalReceipt{}, false, fmt.Errorf("%w: terminal status, digest, idempotency, and commit time are required", ErrInvalid)
	}
	if t.state != TargetLeased && t.state != TargetWorking && t.state != TargetCancelRequested {
		return TerminalReceipt{}, false, ErrStaleAttempt
	}
	if !receipt.CommittedAt.Before(t.lease.ExpiresAt) {
		t.expire(receipt.CommittedAt)
		return TerminalReceipt{}, false, ErrStaleAttempt
	}
	if t.state == TargetCancelRequested && receipt.Status != TerminalCanceled {
		return TerminalReceipt{}, false, fmt.Errorf("%w: a canceled attempt requires a canceled terminal receipt", ErrInvalid)
	}
	if t.state != TargetCancelRequested && receipt.Status == TerminalCanceled {
		return TerminalReceipt{}, false, fmt.Errorf("%w: canceled terminal receipt requires a durable cancellation", ErrInvalid)
	}
	t.terminalReceipt = &receipt
	switch receipt.Status {
	case TerminalCompleted:
		t.state, t.lease.State = TargetCompleted, LeaseCompleted
	case TerminalFailed:
		t.state, t.lease.State = TargetFailed, LeaseFailed
	case TerminalCanceled:
		t.state, t.lease.State = TargetCanceled, LeaseCanceled
	}
	return receipt, true, nil
}

func sameTerminalReceipt(left, right TerminalReceipt) bool {
	return left.TargetID == right.TargetID && left.AttemptID == right.AttemptID && left.MachineID == right.MachineID &&
		left.IdempotencyKey == right.IdempotencyKey && left.Status == right.Status && left.ResultDigest == right.ResultDigest &&
		left.CommittedAt.Equal(right.CommittedAt)
}

func lowerSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, char := range value {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return false
		}
	}
	return true
}

// ObserveExpiry records an elapsed active lease as actionable recovery. A
// false result means either the deadline has not elapsed or recovery was
// already recorded.
func (t *Target) ObserveExpiry(observedAt time.Time) (bool, error) {
	if t == nil || observedAt.IsZero() {
		return false, fmt.Errorf("%w: target and observation time are required", ErrInvalid)
	}
	if t.state != TargetLeased && t.state != TargetWorking && t.state != TargetCancelRequested {
		return false, nil
	}
	if observedAt.Before(t.lease.ExpiresAt) {
		return false, nil
	}
	t.expire(observedAt)
	return true, nil
}

func (t *Target) expire(observedAt time.Time) {
	t.lease.State = LeaseExpired
	t.recovery = &Recovery{
		TargetID: t.lease.TargetID, AttemptID: t.lease.AttemptID, MachineID: t.lease.MachineID,
		Reason: RecoveryLeaseExpired, ExpiresAt: t.lease.ExpiresAt, ObservedAt: observedAt,
	}
	t.state = TargetRecoverableNeedsAttention
}

func blank(value string) bool { return strings.TrimSpace(value) == "" }

func uniqueNonBlank(label string, values []string) error {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if blank(value) {
			return fmt.Errorf("%w: %s is blank", ErrInvalid, label)
		}
		if _, exists := seen[value]; exists {
			return fmt.Errorf("%w: duplicate %s %q", ErrInvalid, label, value)
		}
		seen[value] = struct{}{}
	}
	return nil
}
