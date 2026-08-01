package scheduler

import (
	"context"
	"fmt"
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
	repository Repository
	trigger    Trigger
	clock      *Scheduler
	now        func() time.Time

	mu         sync.Mutex
	ctx        context.Context
	started    bool
	registered map[string]bool
}

func NewDurableScheduler(repository Repository, trigger Trigger) *DurableScheduler {
	return &DurableScheduler{repository: repository, trigger: trigger, clock: New(), now: time.Now, registered: map[string]bool{}}
}

// Start reloads durable definitions before the in-process clock begins.
func (s *DurableScheduler) Start(ctx context.Context) error {
	definitions, err := s.repository.ListSchedules()
	if err != nil {
		return err
	}
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return fmt.Errorf("durable scheduler already started")
	}
	s.ctx, s.started = ctx, true
	s.mu.Unlock()
	for _, definition := range definitions {
		if err := s.register(definition); err != nil {
			return err
		}
	}
	if err := s.MaterializeDay(ctx, s.now(), time.Local); err != nil {
		return err
	}
	s.clock.Start()
	return nil
}

func (s *DurableScheduler) Stop() {
	s.clock.Stop()
	s.mu.Lock()
	s.started = false
	s.mu.Unlock()
}

// Create persists and hot-registers a definition in the running daemon.
func (s *DurableScheduler) Create(_ context.Context, definition Definition) error {
	if err := validateDefinition(definition); err != nil {
		return err
	}
	now := s.now().UTC()
	if definition.Title == "" {
		definition.Title = definition.FlowID
	}
	nextFireAt, err := NextFire(definition, now)
	if err != nil {
		return err
	}
	definition.NextFireAt = nextFireAt
	if definition.CreatedAt.IsZero() {
		definition.CreatedAt = now
	}
	definition.UpdatedAt = now
	if err := s.repository.CreateSchedule(definition); err != nil {
		return err
	}
	if err := s.register(definition); err != nil {
		return err
	}
	return s.MaterializeDay(context.Background(), now, time.Local)
}

func (s *DurableScheduler) Definitions(context.Context) ([]Definition, error) {
	return s.repository.ListSchedules()
}

// MaterializeDay makes the right rail a projection of real durable rows. It
// is safe to call on every Today request because occurrence IDs are stable.
func (s *DurableScheduler) MaterializeDay(_ context.Context, day time.Time, viewerLocation *time.Location) error {
	definitions, err := s.repository.ListSchedules()
	if err != nil {
		return err
	}
	now := s.now().UTC()
	for _, definition := range definitions {
		occurrences, expandErr := OccurrencesForDay(definition, day, viewerLocation)
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
	s.registered[definition.ID] = true
	ctx := s.ctx
	s.mu.Unlock()
	if ctx == nil {
		ctx = context.Background()
	}
	switch definition.Kind {
	case KindCron:
		_, err := s.clock.Cron("CRON_TZ="+definition.Timezone+" "+definition.Expression, func() {
			scheduledFor := s.now().UTC().Truncate(time.Second)
			s.fire(ctx, definition, scheduledFor)
		})
		return err
	case KindOnce:
		at, err := parseOnce(definition)
		if err != nil {
			return err
		}
		if !at.After(s.now()) {
			return nil
		}
		s.clock.OnceAt(at, func() { s.fire(ctx, definition, at) })
		return nil
	default:
		return fmt.Errorf("unknown schedule kind %q", definition.Kind)
	}
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
