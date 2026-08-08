// Package scheduler fires flows on cron schedules and one-shot times
// (backlog AO-028) — the basis for "assign and walk away" and recurring digests.
package scheduler

import (
	"fmt"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

// Scheduler runs cron and one-shot triggers.
type Scheduler struct {
	c             *cron.Cron
	mu            sync.Mutex
	timers        []*time.Timer
	onceJobs      sync.WaitGroup
	plannedCron   []*plannedCronJob
	cronNow       func() time.Time
	cronWait      cronWaitFunc
	nextPlannedID cron.EntryID
	started       bool
	stopped       bool
	stopDone      chan struct{}
}

type cronWaitFunc func(time.Time, <-chan struct{}) (time.Time, bool)

type plannedCronJob struct {
	schedule  cron.Schedule
	fn        func(time.Time)
	now       func() time.Time
	wait      cronWaitFunc
	launch    func(func())
	stop      chan struct{}
	done      chan struct{}
	callbacks sync.WaitGroup
}

func (j *plannedCronJob) start() {
	go j.run()
}

func (j *plannedCronJob) run() {
	defer close(j.done)
	for plannedFor := j.schedule.Next(j.now()); !plannedFor.IsZero(); {
		observedAt, ok := j.wait(plannedFor, j.stop)
		if !ok {
			return
		}
		scheduledFor := plannedFor
		j.callbacks.Add(1)
		j.launch(func() {
			defer j.callbacks.Done()
			j.fn(scheduledFor)
		})
		plannedFor = j.schedule.Next(observedAt)
	}
}

func launchCronCallback(callback func()) {
	go callback()
}

func waitForCron(plannedFor time.Time, stop <-chan struct{}) (time.Time, bool) {
	timer := time.NewTimer(time.Until(plannedFor))
	select {
	case <-timer.C:
		return time.Now(), true
	case <-stop:
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		return time.Time{}, false
	}
}

// New builds a scheduler with seconds-precision cron support.
func New() *Scheduler {
	return &Scheduler{
		c: cron.New(cron.WithSeconds()), cronNow: time.Now, cronWait: waitForCron,
		stopDone: make(chan struct{}),
	}
}

// Cron registers a recurring trigger. spec is a 6-field cron (with seconds),
// e.g. "0 0 9 * * *" for 09:00 daily, or "@every 1h".
func (s *Scheduler) Cron(spec string, fn func()) (cron.EntryID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return 0, fmt.Errorf("scheduler stopped")
	}
	return s.c.AddFunc(spec, fn)
}

// CronScheduled registers a recurring trigger and reports the occurrence time
// planned by the cron clock, even when its callback starts late.
func (s *Scheduler) CronScheduled(spec string, fn func(time.Time)) (cron.EntryID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return 0, fmt.Errorf("scheduler stopped")
	}
	parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	schedule, err := parser.Parse(spec)
	if err != nil {
		return 0, err
	}
	job := &plannedCronJob{
		schedule: schedule, fn: fn, now: s.cronNow, wait: s.cronWait, launch: launchCronCallback,
		stop: make(chan struct{}), done: make(chan struct{}),
	}
	s.plannedCron = append(s.plannedCron, job)
	if s.started {
		job.start()
	}
	return s.allocatePlannedID(), nil
}

func (s *Scheduler) allocatePlannedID() cron.EntryID {
	s.nextPlannedID++
	return s.nextPlannedID
}

// Once registers a one-shot trigger firing after d.
func (s *Scheduler) Once(d time.Duration, fn func()) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return fmt.Errorf("scheduler stopped")
	}
	s.onceJobs.Add(1)
	t := time.AfterFunc(d, func() {
		defer s.onceJobs.Done()
		fn()
	})
	s.timers = append(s.timers, t)
	return nil
}

// OnceAt registers a one-shot trigger firing at the given time.
func (s *Scheduler) OnceAt(at time.Time, fn func()) error {
	return s.Once(time.Until(at), fn)
}

// Start begins firing cron triggers.
func (s *Scheduler) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped || s.started {
		return
	}
	s.started = true
	s.c.Start()
	for _, job := range s.plannedCron {
		job.start()
	}
}

// Stop halts cron triggers and cancels pending one-shots.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	if s.stopped {
		stopDone := s.stopDone
		s.mu.Unlock()
		<-stopDone
		return
	}
	s.stopped = true
	for _, t := range s.timers {
		if t.Stop() {
			s.onceJobs.Done()
		}
	}
	s.timers = nil
	var plannedCron []*plannedCronJob
	if s.started {
		plannedCron = append(plannedCron, s.plannedCron...)
		for _, job := range plannedCron {
			close(job.stop)
		}
	}
	s.started = false
	s.mu.Unlock()
	cronJobs := s.c.Stop()
	<-cronJobs.Done()
	for _, job := range plannedCron {
		<-job.done
		job.callbacks.Wait()
	}
	s.onceJobs.Wait()
	close(s.stopDone)
}
