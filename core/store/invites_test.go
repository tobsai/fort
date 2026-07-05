package store

import (
	"testing"
	"time"
)

func TestInviteLifecycle(t *testing.T) {
	s := openTemp(t)
	now := time.Now()
	if err := s.CreateInvite("hash1", now.Add(15*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := s.CheckInvite("hash1", now); err != nil {
		t.Fatalf("fresh invite should check: %v", err)
	}
	if err := s.CheckInvite("nope", now); err != ErrInviteInvalid {
		t.Fatalf("unknown code: %v, want ErrInviteInvalid", err)
	}
	if err := s.CheckInvite("hash1", now.Add(16*time.Minute)); err != ErrInviteExpired {
		t.Fatalf("expired: %v, want ErrInviteExpired", err)
	}
	if err := s.MarkInviteUsed("hash1", now); err != nil {
		t.Fatal(err)
	}
	if err := s.CheckInvite("hash1", now); err != ErrInviteInvalid {
		t.Fatalf("used code must be invalid: %v", err)
	}
	// Marking twice is an error (single use is enforced at mark time too).
	if err := s.MarkInviteUsed("hash1", now); err == nil {
		t.Fatal("second MarkInviteUsed must fail")
	}
}
