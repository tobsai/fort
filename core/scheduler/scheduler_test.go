package scheduler

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestCronFires(t *testing.T) {
	s := New()
	var n int32
	if _, err := s.Cron("* * * * * *", func() { atomic.AddInt32(&n, 1) }); err != nil {
		t.Fatalf("cron: %v", err)
	}
	s.Start()
	defer s.Stop()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(&n) >= 1 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("cron did not fire within 3s (n=%d)", atomic.LoadInt32(&n))
}

func TestOnceFires(t *testing.T) {
	s := New()
	defer s.Stop()
	fired := make(chan struct{}, 1)
	s.Once(150*time.Millisecond, func() { fired <- struct{}{} })
	select {
	case <-fired:
	case <-time.After(2 * time.Second):
		t.Fatal("one-shot trigger did not fire")
	}
}

func TestInvalidCronRejected(t *testing.T) {
	s := New()
	if _, err := s.Cron("not a cron", func() {}); err == nil {
		t.Fatal("expected error for invalid cron spec")
	}
}
