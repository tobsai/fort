package worker_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/tobsai/fort/core/worker"
)

func TestClaimCreatesOneExactPinnedLease(t *testing.T) {
	claimedAt := time.Date(2026, 8, 21, 15, 0, 0, 0, time.UTC)
	authority := worker.AuthoritySnapshot{
		ID:               "authority:effective:1",
		Revision:         "revision:7",
		Permissions:      []string{"conversation.read", "message.append"},
		ContextRecordIDs: []string{"message:input:1"},
	}
	target, err := worker.NewTarget(worker.TargetSpec{
		ID:        "target:1",
		AccountID: "account:1",
		Pins: worker.ExecutionPins{
			AgentID:                    "agent:researcher",
			BehaviorRevisionID:         "behavior:4",
			BindingRevisionID:          "binding:9",
			SeatID:                     "seat:researcher",
			EffectiveAuthoritySnapshot: authority,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// The Target owns the persisted snapshot; caller-owned slices cannot drift it.
	authority.Permissions[0] = "account.admin"
	command := worker.ClaimCommand{
		TargetID:       "target:1",
		AttemptID:      "attempt:1",
		MachineID:      "machine:mac-studio",
		IdempotencyKey: "claim:1",
		ClaimedAt:      claimedAt,
		ExpiresAt:      claimedAt.Add(2 * time.Minute),
	}
	lease, err := target.Claim(enrolledMachine(claimedAt), command)
	if err != nil {
		t.Fatal(err)
	}

	wantPins := worker.ExecutionPins{
		AgentID:            "agent:researcher",
		BehaviorRevisionID: "behavior:4",
		BindingRevisionID:  "binding:9",
		SeatID:             "seat:researcher",
		EffectiveAuthoritySnapshot: worker.AuthoritySnapshot{
			ID:               "authority:effective:1",
			Revision:         "revision:7",
			Permissions:      []string{"conversation.read", "message.append"},
			ContextRecordIDs: []string{"message:input:1"},
		},
	}
	if lease.TargetID != command.TargetID || lease.AttemptID != command.AttemptID || lease.MachineID != command.MachineID {
		t.Fatalf("lease identity = %#v, want target/attempt/machine from claim", lease)
	}
	if !reflect.DeepEqual(lease.Pins, wantPins) {
		t.Fatalf("lease pins = %#v, want %#v", lease.Pins, wantPins)
	}
	if lease.ExpiresAt != command.ExpiresAt || lease.State != worker.LeaseClaimed {
		t.Fatalf("lease state/expiry = %q/%v, want %q/%v", lease.State, lease.ExpiresAt, worker.LeaseClaimed, command.ExpiresAt)
	}
	replayed, err := target.Claim(enrolledMachine(claimedAt), command)
	if err != nil {
		t.Fatalf("idempotent claim replay: %v", err)
	}
	if !reflect.DeepEqual(replayed, lease) {
		t.Fatalf("claim replay = %#v, want original lease %#v", replayed, lease)
	}
	conflict := command
	conflict.ExpiresAt = command.ExpiresAt.Add(time.Minute)
	if _, err := target.Claim(enrolledMachine(claimedAt), conflict); !errors.Is(err, worker.ErrIdempotencyConflict) {
		t.Fatalf("conflicting claim replay error = %v, want %v", err, worker.ErrIdempotencyConflict)
	}

	other := enrolledMachine(claimedAt)
	other.ID = "machine:macbook"
	_, err = target.Claim(other, worker.ClaimCommand{
		TargetID: "target:1", AttemptID: "attempt:2", MachineID: other.ID,
		IdempotencyKey: "claim:2", ClaimedAt: claimedAt, ExpiresAt: claimedAt.Add(2 * time.Minute),
	})
	if !errors.Is(err, worker.ErrAlreadyLeased) {
		t.Fatalf("second claim error = %v, want %v", err, worker.ErrAlreadyLeased)
	}
}

func TestHeartbeatRejectsStaleAttemptAndExpiredLeaseNeedsAttention(t *testing.T) {
	claimedAt := time.Date(2026, 8, 21, 15, 0, 0, 0, time.UTC)
	target, initialLease := claimedTarget(t, claimedAt)

	_, err := target.Heartbeat(worker.HeartbeatCommand{
		TargetID: "target:1", AttemptID: "attempt:stale", MachineID: initialLease.MachineID,
		ObservedAt: claimedAt.Add(30 * time.Second), ExtendUntil: claimedAt.Add(3 * time.Minute),
	})
	if !errors.Is(err, worker.ErrStaleAttempt) {
		t.Fatalf("wrong-attempt heartbeat error = %v, want %v", err, worker.ErrStaleAttempt)
	}

	result, err := target.Heartbeat(worker.HeartbeatCommand{
		TargetID: initialLease.TargetID, AttemptID: initialLease.AttemptID, MachineID: initialLease.MachineID,
		ObservedAt: claimedAt.Add(time.Minute), ExtendUntil: claimedAt.Add(4 * time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Lease.State != worker.LeaseWorking || result.Lease.ExpiresAt != claimedAt.Add(4*time.Minute) {
		t.Fatalf("renewed lease = %#v, want Working through exact extension", result.Lease)
	}
	if result.Directive != worker.DirectiveContinue {
		t.Fatalf("heartbeat directive = %q, want %q", result.Directive, worker.DirectiveContinue)
	}

	_, err = target.Heartbeat(worker.HeartbeatCommand{
		TargetID: initialLease.TargetID, AttemptID: initialLease.AttemptID, MachineID: initialLease.MachineID,
		ObservedAt: claimedAt.Add(4 * time.Minute), ExtendUntil: claimedAt.Add(5 * time.Minute),
	})
	if !errors.Is(err, worker.ErrStaleAttempt) {
		t.Fatalf("expired-at-boundary heartbeat error = %v, want %v", err, worker.ErrStaleAttempt)
	}

	snapshot := target.Snapshot()
	if snapshot.State != worker.TargetRecoverableNeedsAttention || snapshot.Recovery == nil {
		t.Fatalf("expired target snapshot = %#v, want explicit recovery needing attention", snapshot)
	}
	if snapshot.Recovery.Reason != worker.RecoveryLeaseExpired || snapshot.Recovery.AttemptID != initialLease.AttemptID ||
		snapshot.Recovery.MachineID != initialLease.MachineID || snapshot.Recovery.ObservedAt != claimedAt.Add(4*time.Minute) {
		t.Fatalf("recovery evidence = %#v, want exact expired attempt evidence", snapshot.Recovery)
	}

	other := enrolledMachine(claimedAt)
	other.ID = "machine:macbook"
	_, err = target.Claim(other, worker.ClaimCommand{
		TargetID: "target:1", AttemptID: "attempt:2", MachineID: other.ID,
		IdempotencyKey: "claim:2", ClaimedAt: claimedAt.Add(4 * time.Minute), ExpiresAt: claimedAt.Add(6 * time.Minute),
	})
	if !errors.Is(err, worker.ErrNeedsAttention) {
		t.Fatalf("reroute after expiry error = %v, want %v", err, worker.ErrNeedsAttention)
	}
}

func TestCancellationIsDurableAndReturnedByHeartbeat(t *testing.T) {
	claimedAt := time.Date(2026, 8, 21, 15, 0, 0, 0, time.UTC)
	target, lease := claimedTarget(t, claimedAt)
	command := worker.CancelCommand{
		TargetID: lease.TargetID, AttemptID: lease.AttemptID, MachineID: lease.MachineID,
		IdempotencyKey: "cancel:1", RequestedAt: claimedAt.Add(30 * time.Second), Reason: "requested_by_owner",
	}
	cancellation, err := target.RequestCancel(command)
	if err != nil {
		t.Fatal(err)
	}
	if cancellation.TargetID != lease.TargetID || cancellation.AttemptID != lease.AttemptID || cancellation.MachineID != lease.MachineID {
		t.Fatalf("cancellation identity = %#v, want exact active lease", cancellation)
	}

	replayed, err := target.RequestCancel(command)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(replayed, cancellation) {
		t.Fatalf("replayed cancellation = %#v, want %#v", replayed, cancellation)
	}
	conflict := command
	conflict.Reason = "different_reason"
	if _, err := target.RequestCancel(conflict); !errors.Is(err, worker.ErrIdempotencyConflict) {
		t.Fatalf("conflicting cancel replay error = %v, want %v", err, worker.ErrIdempotencyConflict)
	}

	heartbeat, err := target.Heartbeat(worker.HeartbeatCommand{
		TargetID: lease.TargetID, AttemptID: lease.AttemptID, MachineID: lease.MachineID,
		ObservedAt: claimedAt.Add(time.Minute), ExtendUntil: claimedAt.Add(3 * time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	if heartbeat.Directive != worker.DirectiveCancel || heartbeat.Lease.State != worker.LeaseCancelRequested {
		t.Fatalf("post-cancel heartbeat = %#v, want durable cancel directive", heartbeat)
	}
	snapshot := target.Snapshot()
	if snapshot.State != worker.TargetCancelRequested || !reflect.DeepEqual(snapshot.Cancellation, &cancellation) {
		t.Fatalf("post-cancel snapshot = %#v, want durable cancellation", snapshot)
	}
}

func TestTerminalReceiptIsSingleIdempotentAndRejectsStaleWrites(t *testing.T) {
	claimedAt := time.Date(2026, 8, 21, 15, 0, 0, 0, time.UTC)
	target, lease := claimedTarget(t, claimedAt)
	_, err := target.RequestCancel(worker.CancelCommand{
		TargetID: lease.TargetID, AttemptID: lease.AttemptID, MachineID: lease.MachineID,
		IdempotencyKey: "cancel:1", RequestedAt: claimedAt.Add(30 * time.Second), Reason: "requested_by_owner",
	})
	if err != nil {
		t.Fatal(err)
	}
	receipt := worker.TerminalReceipt{
		TargetID: lease.TargetID, AttemptID: lease.AttemptID, MachineID: lease.MachineID,
		IdempotencyKey: "terminal:1", Status: worker.TerminalCanceled,
		ResultDigest: strings.Repeat("a", 64), CommittedAt: claimedAt.Add(time.Minute),
	}

	committed, created, err := target.CommitTerminal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if !created || !reflect.DeepEqual(committed, receipt) {
		t.Fatalf("first terminal commit = %#v/%t, want exact receipt/created", committed, created)
	}
	replayed, created, err := target.CommitTerminal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if created || !reflect.DeepEqual(replayed, receipt) {
		t.Fatalf("terminal replay = %#v/%t, want exact receipt/not created", replayed, created)
	}

	conflict := receipt
	conflict.ResultDigest = strings.Repeat("b", 64)
	if _, _, err := target.CommitTerminal(conflict); !errors.Is(err, worker.ErrIdempotencyConflict) {
		t.Fatalf("conflicting terminal replay error = %v, want %v", err, worker.ErrIdempotencyConflict)
	}
	second := receipt
	second.IdempotencyKey = "terminal:2"
	if _, _, err := target.CommitTerminal(second); !errors.Is(err, worker.ErrTerminalReceiptExists) {
		t.Fatalf("second terminal receipt error = %v, want %v", err, worker.ErrTerminalReceiptExists)
	}
	snapshot := target.Snapshot()
	if snapshot.State != worker.TargetCanceled || snapshot.TerminalReceipt == nil || !reflect.DeepEqual(*snapshot.TerminalReceipt, receipt) {
		t.Fatalf("terminal snapshot = %#v, want one canceled receipt", snapshot)
	}

	expired, expiredLease := claimedTarget(t, claimedAt)
	if _, err := expired.ObserveExpiry(expiredLease.ExpiresAt); err != nil {
		t.Fatal(err)
	}
	stale := receipt
	stale.IdempotencyKey = "terminal:expired"
	stale.Status = worker.TerminalCompleted
	stale.CommittedAt = expiredLease.ExpiresAt
	if _, _, err := expired.CommitTerminal(stale); !errors.Is(err, worker.ErrStaleAttempt) {
		t.Fatalf("expired terminal write error = %v, want %v", err, worker.ErrStaleAttempt)
	}
	if expired.Snapshot().State != worker.TargetRecoverableNeedsAttention {
		t.Fatalf("expired terminal write changed recovery state: %#v", expired.Snapshot())
	}
}

func enrolledMachine(now time.Time) worker.EnrolledMachine {
	return worker.EnrolledMachine{
		ID:         "machine:mac-studio",
		AccountID:  "account:1",
		State:      worker.MachineEnrolled,
		EnrolledAt: now.Add(-time.Hour),
	}
}

func claimedTarget(t *testing.T, claimedAt time.Time) (*worker.Target, worker.Lease) {
	t.Helper()
	target, err := worker.NewTarget(worker.TargetSpec{
		ID: "target:1", AccountID: "account:1",
		Pins: worker.ExecutionPins{
			AgentID: "agent:researcher", BehaviorRevisionID: "behavior:4", BindingRevisionID: "binding:9",
			SeatID: "seat:researcher", EffectiveAuthoritySnapshot: worker.AuthoritySnapshot{
				ID: "authority:effective:1", Revision: "revision:7", Permissions: []string{"message.append"},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	lease, err := target.Claim(enrolledMachine(claimedAt), worker.ClaimCommand{
		TargetID: "target:1", AttemptID: "attempt:1", MachineID: "machine:mac-studio",
		IdempotencyKey: "claim:1", ClaimedAt: claimedAt, ExpiresAt: claimedAt.Add(2 * time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	return target, lease
}
