package scheduler

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type controlledCronWait struct {
	requests chan cronWaitRequest
}

type cronWaitRequest struct {
	plannedFor time.Time
	observedAt chan time.Time
}

func newControlledCronWait() *controlledCronWait {
	return &controlledCronWait{requests: make(chan cronWaitRequest)}
}

func (w *controlledCronWait) wait(plannedFor time.Time, stop <-chan struct{}) (time.Time, bool) {
	request := cronWaitRequest{plannedFor: plannedFor, observedAt: make(chan time.Time, 1)}
	select {
	case w.requests <- request:
	case <-stop:
		return time.Time{}, false
	}
	select {
	case observedAt := <-request.observedAt:
		return observedAt, true
	case <-stop:
		return time.Time{}, false
	}
}

func awaitCronWait(t *testing.T, wait *controlledCronWait) cronWaitRequest {
	t.Helper()
	select {
	case request := <-wait.requests:
		return request
	case <-time.After(time.Second):
		t.Fatal("planned cron did not begin waiting")
		return cronWaitRequest{}
	}
}

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
	if err := s.Once(150*time.Millisecond, func() { fired <- struct{}{} }); err != nil {
		t.Fatal(err)
	}
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

func TestCronScheduledCannotBeMutatedThroughRobfigInspection(t *testing.T) {
	s := New()
	defer s.Stop()
	if _, err := s.CronScheduled("0 * * * * *", func(time.Time) {}); err != nil {
		t.Fatal(err)
	}
	if entries := s.c.Entries(); len(entries) != 0 {
		t.Fatalf("planned cron leaked through robfig inspection: %+v", entries)
	}
}

func TestCronScheduledWaitsForStartAndHotRegistrationStartsImmediately(t *testing.T) {
	now := time.Date(2026, 8, 3, 2, 59, 59, 0, time.UTC)
	wait := newControlledCronWait()
	s := New()
	s.cronNow = func() time.Time { return now }
	s.cronWait = wait.wait
	defer s.Stop()

	if _, err := s.CronScheduled("0 * * * * *", func(time.Time) {}); err != nil {
		t.Fatal(err)
	}
	select {
	case request := <-wait.requests:
		t.Fatalf("pre-Start registration launched for %s", request.plannedFor)
	case <-time.After(50 * time.Millisecond):
	}

	s.Start()
	preStart := awaitCronWait(t, wait)
	if want := time.Date(2026, 8, 3, 3, 0, 0, 0, time.UTC); !preStart.plannedFor.Equal(want) {
		t.Fatalf("pre-Start planned time = %s, want %s", preStart.plannedFor, want)
	}

	hotFired := make(chan time.Time, 1)
	if _, err := s.CronScheduled("30 * * * * *", func(scheduledFor time.Time) { hotFired <- scheduledFor }); err != nil {
		t.Fatal(err)
	}
	hot := awaitCronWait(t, wait)
	wantHot := time.Date(2026, 8, 3, 3, 0, 30, 0, time.UTC)
	if !hot.plannedFor.Equal(wantHot) {
		t.Fatalf("hot planned time = %s, want %s", hot.plannedFor, wantHot)
	}
	hot.observedAt <- hot.plannedFor
	select {
	case scheduledFor := <-hotFired:
		if !scheduledFor.Equal(wantHot) {
			t.Fatalf("hot callback time = %s, want %s", scheduledFor, wantHot)
		}
	case <-time.After(time.Second):
		t.Fatal("hot scheduled cron did not fire")
	}
}

func TestCronScheduledPreservesCRONTZAcrossDSTGap(t *testing.T) {
	// America/Chicago jumps from 01:59:59 to 03:00:00 on this date, so
	// 02:30 does not exist and robfig correctly plans the following day.
	now := time.Date(2026, 3, 8, 7, 59, 59, 0, time.UTC)
	wait := newControlledCronWait()
	s := New()
	s.cronNow = func() time.Time { return now }
	s.cronWait = wait.wait
	defer s.Stop()
	if _, err := s.CronScheduled("CRON_TZ=America/Chicago 0 30 2 * * *", func(time.Time) {}); err != nil {
		t.Fatal(err)
	}
	s.Start()
	request := awaitCronWait(t, wait)
	want := time.Date(2026, 3, 9, 7, 30, 0, 0, time.UTC)
	if !request.plannedFor.Equal(want) {
		t.Fatalf("planned time across DST gap = %s, want %s", request.plannedFor, want)
	}
}

func TestStoppedSchedulerRejectsOneShotRegistration(t *testing.T) {
	s := New()
	s.Stop()
	if err := s.Once(time.Hour, func() {}); err == nil {
		t.Fatal("stopped scheduler silently accepted a relative one-shot")
	}
	if err := s.OnceAt(time.Now().Add(time.Hour), func() {}); err == nil {
		t.Fatal("stopped scheduler silently accepted an absolute one-shot")
	}
}

func TestStopWaitsForRunningOneShot(t *testing.T) {
	s := New()
	started := make(chan struct{})
	release := make(chan struct{})
	if err := s.Once(0, func() {
		close(started)
		<-release
	}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("one-shot callback did not start")
	}

	stopped := make(chan struct{})
	go func() {
		s.Stop()
		close(stopped)
	}()
	returnedEarly := false
	select {
	case <-stopped:
		returnedEarly = true
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("Stop did not return after one-shot callback completed")
	}
	if returnedEarly {
		t.Fatal("Stop returned while a one-shot callback was running")
	}
}

func TestStopWaitsForRunningCronCallback(t *testing.T) {
	s := New()
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	if _, err := s.Cron("* * * * * *", func() {
		select {
		case started <- struct{}{}:
		default:
		}
		<-release
	}); err != nil {
		t.Fatal(err)
	}
	s.Start()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		s.Stop()
		t.Fatal("cron callback did not start")
	}

	stopped := make(chan struct{})
	go func() {
		s.Stop()
		close(stopped)
	}()
	returnedEarly := false
	select {
	case <-stopped:
		returnedEarly = true
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("Stop did not return after cron callback completed")
	}
	if returnedEarly {
		t.Fatal("Stop returned while a cron callback was running")
	}
}

func TestStopConcurrentlyWaitsForRunningScheduledCronCallback(t *testing.T) {
	now := time.Date(2026, 8, 3, 2, 59, 59, 0, time.UTC)
	wait := newControlledCronWait()
	s := New()
	s.cronNow = func() time.Time { return now }
	s.cronWait = wait.wait
	started := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseCallback := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(func() {
		releaseCallback()
		s.Stop()
	})
	if _, err := s.CronScheduled("0 * * * * *", func(time.Time) {
		close(started)
		<-release
	}); err != nil {
		t.Fatal(err)
	}
	s.Start()
	request := awaitCronWait(t, wait)
	request.observedAt <- request.plannedFor
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("scheduled cron callback did not start")
	}

	stopped := make(chan struct{}, 2)
	for range 2 {
		go func() {
			s.Stop()
			stopped <- struct{}{}
		}()
	}
	select {
	case <-stopped:
		t.Fatal("concurrent Stop returned while a scheduled cron callback was running")
	case <-time.After(50 * time.Millisecond):
	}
	releaseCallback()
	for range 2 {
		select {
		case <-stopped:
		case <-time.After(time.Second):
			t.Fatal("concurrent Stop did not return after the callback completed")
		}
	}

	returned := make(chan struct{})
	go func() {
		s.Stop()
		close(returned)
	}()
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("idempotent Stop did not return")
	}
}
