// Package scheduler fires flows on cron schedules and one-shot times
// (backlog AO-028) — the basis for "assign and walk away" and recurring digests.
package scheduler

import (
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

// Scheduler runs cron and one-shot triggers.
type Scheduler struct {
	c      *cron.Cron
	mu     sync.Mutex
	timers []*time.Timer
}

// New builds a scheduler with seconds-precision cron support.
func New() *Scheduler {
	return &Scheduler{c: cron.New(cron.WithSeconds())}
}

// Cron registers a recurring trigger. spec is a 6-field cron (with seconds),
// e.g. "0 0 9 * * *" for 09:00 daily, or "@every 1h".
func (s *Scheduler) Cron(spec string, fn func()) (cron.EntryID, error) {
	return s.c.AddFunc(spec, fn)
}

// Once registers a one-shot trigger firing after d.
func (s *Scheduler) Once(d time.Duration, fn func()) {
	t := time.AfterFunc(d, fn)
	s.mu.Lock()
	s.timers = append(s.timers, t)
	s.mu.Unlock()
}

// OnceAt registers a one-shot trigger firing at the given time.
func (s *Scheduler) OnceAt(at time.Time, fn func()) {
	s.Once(time.Until(at), fn)
}

// Start begins firing cron triggers.
func (s *Scheduler) Start() { s.c.Start() }

// Stop halts cron triggers and cancels pending one-shots.
func (s *Scheduler) Stop() {
	s.c.Stop()
	s.mu.Lock()
	for _, t := range s.timers {
		t.Stop()
	}
	s.timers = nil
	s.mu.Unlock()
}
