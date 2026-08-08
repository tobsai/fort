package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

type Repository interface {
	CreateSchedule(Definition) error
	ListSchedules() ([]Definition, error)
	UpsertScheduleOccurrence(Occurrence) error
	TransitionScheduleOccurrence(id string, from, to OccurrenceState, runID, errorMessage string) (bool, error)
	UpdateScheduleFire(id string, lastFireAt, nextFireAt time.Time) error
}

type Trigger func(context.Context, Definition, Occurrence) error

type DurableScheduler struct {
	repository      Repository
	trigger         Trigger
	clock           *Scheduler
	now             func() time.Time
	displayLocation *time.Location

	lifecycleMu sync.Mutex
	mu          sync.Mutex
	ctx         context.Context
	started     bool
	stopped     bool
	registered  map[string]bool
}

func NewDurableScheduler(repository Repository, trigger Trigger, displayLocation *time.Location) *DurableScheduler {
	if displayLocation == nil {
		panic("scheduler: display location is required")
	}
	return &DurableScheduler{
		repository: repository, trigger: trigger, clock: New(), now: time.Now,
		displayLocation: displayLocation, registered: map[string]bool{},
	}
}

// Start reloads durable definitions before the in-process clock begins.
func (s *DurableScheduler) Start(ctx context.Context) error {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return fmt.Errorf("durable scheduler stopped")
	}
	if s.started {
		s.mu.Unlock()
		return fmt.Errorf("durable scheduler already started")
	}
	s.ctx, s.started = ctx, true
	s.mu.Unlock()
	fail := func(err error) error {
		s.rollbackFailedStart()
		return err
	}
	definitions, err := s.repository.ListSchedules()
	if err != nil {
		return fail(err)
	}
	// Build the durable Today window before any timer can dispatch work. A
	// failed startup is therefore safe to retry on this manager.
	if err := s.materializeDisplayWindow(ctx, s.now()); err != nil {
		return fail(err)
	}
	for _, definition := range definitions {
		if err := s.register(definition); err != nil {
			return fail(err)
		}
	}
	if _, err := s.clock.Cron(dayMaterializationCron(s.displayLocation), func() {
		if err := s.materializeDisplayWindow(ctx, s.now()); err != nil {
			slog.Error("schedule occurrence materialization failed", "reason", err)
		}
	}); err != nil {
		return fail(err)
	}
	s.clock.Start()
	return nil
}

// Stop joins active callbacks and permanently closes this manager. A daemon
// restart constructs a fresh manager from the durable repository.
func (s *DurableScheduler) Stop() {
	s.lifecycleMu.Lock()
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		s.lifecycleMu.Unlock()
		return
	}
	s.stopped = true
	s.started = false
	s.ctx = nil
	clock := s.clock
	s.mu.Unlock()
	s.lifecycleMu.Unlock()
	clock.Stop()
}

// Create persists and hot-registers a definition in the running daemon.
func (s *DurableScheduler) Create(ctx context.Context, definition Definition) (Definition, error) {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return Definition{}, fmt.Errorf("durable scheduler stopped")
	}
	if !s.started {
		s.mu.Unlock()
		return Definition{}, fmt.Errorf("durable scheduler is not started")
	}
	s.mu.Unlock()
	if err := validateDefinition(definition); err != nil {
		return Definition{}, err
	}
	now := s.now().UTC()
	if definition.Title == "" {
		definition.Title = definition.FlowID
	}
	nextFireAt, err := NextFire(definition, now)
	if err != nil {
		return Definition{}, err
	}
	definition.NextFireAt = nextFireAt
	if definition.CreatedAt.IsZero() {
		definition.CreatedAt = now
	}
	definition.UpdatedAt = now
	if err := s.repository.CreateSchedule(definition); err != nil {
		return Definition{}, err
	}
	if err := s.register(definition); err != nil {
		return Definition{}, err
	}
	if err := s.materializeDisplayWindow(ctx, now); err != nil {
		// Persistence plus registration is the schedule-create contract. Once
		// both have succeeded, a Today projection write cannot turn that success
		// into an ambiguous client-visible failure.
		slog.Error("schedule occurrence materialization failed", "schedule_id", definition.ID, "reason", err)
	}
	return definition, nil
}

func (s *DurableScheduler) Definitions(context.Context) ([]Definition, error) {
	return s.repository.ListSchedules()
}

// materializeDay persists planned rows for one daemon-owned display day. The
// Today API remains a read-only projection of these durable rows.
func (s *DurableScheduler) materializeDay(_ context.Context, day time.Time) error {
	definitions, err := s.repository.ListSchedules()
	if err != nil {
		return err
	}
	now := s.now().UTC()
	for _, definition := range definitions {
		occurrences, expandErr := OccurrencesForDay(definition, day, s.displayLocation)
		if expandErr != nil {
			return expandErr
		}
		for _, occurrence := range occurrences {
			occurrence.CreatedAt, occurrence.UpdatedAt = now, now
			if err := s.repository.UpsertScheduleOccurrence(occurrence); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *DurableScheduler) materializeDisplayWindow(ctx context.Context, now time.Time) error {
	if err := s.materializeDay(ctx, now); err != nil {
		return err
	}
	return s.materializeDay(ctx, now.In(s.displayLocation).AddDate(0, 0, 1))
}

func dayMaterializationCron(location *time.Location) string {
	return "CRON_TZ=" + location.String() + " 0 0 0 * * *"
}

func (s *DurableScheduler) rollbackFailedStart() {
	// Start owns lifecycleMu until the failed clock is fully quiescent.
	s.mu.Lock()
	clock := s.clock
	s.clock = New()
	s.ctx = nil
	s.started = false
	s.registered = map[string]bool{}
	s.mu.Unlock()
	clock.Stop()
}

func (s *DurableScheduler) register(definition Definition) error {
	if !definition.Enabled {
		return nil
	}
	if err := validateDefinition(definition); err != nil {
		return err
	}
	s.mu.Lock()
	if s.registered[definition.ID] {
		s.mu.Unlock()
		return nil
	}
	ctx := s.ctx
	s.mu.Unlock()
	if ctx == nil {
		ctx = context.Background()
	}
	var registrationErr error
	switch definition.Kind {
	case KindCron:
		_, registrationErr = s.clock.CronScheduled("CRON_TZ="+definition.Timezone+" "+definition.Expression, func(scheduledFor time.Time) {
			s.fire(ctx, definition, scheduledFor.UTC())
		})
	case KindOnce:
		at, err := parseOnce(definition)
		if err != nil {
			return err
		}
		if at.After(s.now()) {
			registrationErr = s.clock.OnceAt(at, func() { s.fire(ctx, definition, at) })
		}
	default:
		return fmt.Errorf("unknown schedule kind %q", definition.Kind)
	}
	if registrationErr != nil {
		return registrationErr
	}
	s.mu.Lock()
	s.registered[definition.ID] = true
	s.mu.Unlock()
	return nil
}

func (s *DurableScheduler) fire(ctx context.Context, definition Definition, scheduledFor time.Time) {
	now := s.now().UTC()
	runID := "schedule-" + OccurrenceID(definition.ID, scheduledFor)
	occurrence := Occurrence{
		ID: OccurrenceID(definition.ID, scheduledFor), ScheduleID: definition.ID,
		RunID: runID, ScheduledFor: scheduledFor.UTC(), State: OccurrenceScheduled, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.repository.UpsertScheduleOccurrence(occurrence); err != nil {
		return
	}
	changed, err := s.repository.TransitionScheduleOccurrence(occurrence.ID, OccurrenceScheduled, OccurrenceRunning, runID, "")
	if err != nil || !changed {
		return
	}
	triggerErr := s.trigger(ctx, definition, occurrence)
	nextFireAt, _ := NextFire(definition, scheduledFor.Add(time.Second))
	_ = s.repository.UpdateScheduleFire(definition.ID, scheduledFor.UTC(), nextFireAt)
	if triggerErr != nil {
		_, _ = s.repository.TransitionScheduleOccurrence(occurrence.ID, OccurrenceRunning, OccurrenceFailed, runID, triggerErr.Error())
		return
	}
	_, _ = s.repository.TransitionScheduleOccurrence(occurrence.ID, OccurrenceRunning, OccurrenceSucceeded, runID, "")
}

func validateDefinition(definition Definition) error {
	if definition.ID == "" || definition.FlowID == "" || definition.Expression == "" || definition.Timezone == "" {
		return fmt.Errorf("schedule id, flow_id, expression, and timezone are required")
	}
	if _, err := time.LoadLocation(definition.Timezone); err != nil {
		return fmt.Errorf("schedule timezone %q: %w", definition.Timezone, err)
	}
	switch definition.Kind {
	case KindCron:
		parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
		if _, err := parser.Parse(definition.Expression); err != nil {
			return fmt.Errorf("cron expression %q: %w", definition.Expression, err)
		}
		return nil
	case KindOnce:
		_, err := parseOnce(definition)
		return err
	default:
		return fmt.Errorf("unknown schedule kind %q", definition.Kind)
	}
}

func parseOnce(definition Definition) (time.Time, error) {
	location, err := time.LoadLocation(definition.Timezone)
	if err != nil {
		return time.Time{}, err
	}
	at, err := time.Parse(time.RFC3339, definition.Expression)
	if err == nil {
		return at, nil
	}
	at, err = time.ParseInLocation("2006-01-02T15:04:05", definition.Expression, location)
	if err != nil {
		return time.Time{}, fmt.Errorf("one-shot time %q: %w", definition.Expression, err)
	}
	return at, nil
}
