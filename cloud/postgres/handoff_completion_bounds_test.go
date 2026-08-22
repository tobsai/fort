package postgres

import (
	"testing"
	"time"
)

func TestHandoffCompletionWindowRequiresLeaseAndHardDeadline(t *testing.T) {
	started := time.Date(2026, 8, 21, 23, 0, 0, 0, time.UTC)
	hardDeadline := started.Add(30 * time.Second)
	leaseExpiry := started.Add(time.Minute)
	if !handoffCompletionWithinBounds(started.Add(time.Second), started, leaseExpiry, hardDeadline) {
		t.Fatal("rejected completion inside both immutable bounds")
	}
	if handoffCompletionWithinBounds(hardDeadline, started, leaseExpiry, hardDeadline) {
		t.Fatal("accepted completion at the exact hard deadline")
	}
	if handoffCompletionWithinBounds(leaseExpiry, started, leaseExpiry, hardDeadline.Add(time.Minute)) {
		t.Fatal("accepted completion at the exact lease expiry")
	}
}
