package scheduler

import (
	"fmt"
	"strconv"
	"time"

	"github.com/robfig/cron/v3"
)

type Kind string

const (
	KindCron Kind = "cron"
	KindOnce Kind = "once"
)

type OccurrenceState string

const (
	OccurrenceScheduled OccurrenceState = "scheduled"
	OccurrenceRunning   OccurrenceState = "running"
	OccurrenceSucceeded OccurrenceState = "succeeded"
	OccurrenceFailed    OccurrenceState = "failed"
	OccurrenceCanceled  OccurrenceState = "canceled"
)

type Definition struct {
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	Kind       Kind      `json:"kind"`
	Expression string    `json:"expression"`
	FlowID     string    `json:"flow_id"`
	Timezone   string    `json:"timezone"`
	Enabled    bool      `json:"enabled"`
	NextFireAt time.Time `json:"next_fire_at,omitempty"`
	LastFireAt time.Time `json:"last_fire_at,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func NextFire(definition Definition, after time.Time) (time.Time, error) {
	if !definition.Enabled {
		return time.Time{}, nil
	}
	location, err := time.LoadLocation(definition.Timezone)
	if err != nil {
		return time.Time{}, err
	}
	switch definition.Kind {
	case KindOnce:
		at, err := time.Parse(time.RFC3339, definition.Expression)
		if err != nil {
			at, err = time.ParseInLocation("2006-01-02T15:04:05", definition.Expression, location)
		}
		if err != nil {
			return time.Time{}, err
		}
		if at.After(after) {
			return at.UTC(), nil
		}
		return time.Time{}, nil
	case KindCron:
		parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
		parsed, err := parser.Parse(definition.Expression)
		if err != nil {
			return time.Time{}, err
		}
		return parsed.Next(after.In(location)).UTC(), nil
	default:
		return time.Time{}, fmt.Errorf("unknown schedule kind %q", definition.Kind)
	}
}

type Occurrence struct {
	ID           string          `json:"id"`
	ScheduleID   string          `json:"schedule_id"`
	RunID        string          `json:"run_id,omitempty"`
	ScheduledFor time.Time       `json:"scheduled_for"`
	State        OccurrenceState `json:"state"`
	Error        string          `json:"error,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

func OccurrenceID(scheduleID string, scheduledFor time.Time) string {
	return scheduleID + ":" + strconv.FormatInt(scheduledFor.UTC().Unix(), 10)
}

// OccurrencesForDay expands one durable definition into real occurrences
// whose instants fall inside the requested viewer-local calendar day.
func OccurrencesForDay(definition Definition, day time.Time, viewerLocation *time.Location) ([]Occurrence, error) {
	if !definition.Enabled {
		return []Occurrence{}, nil
	}
	if viewerLocation == nil {
		viewerLocation = time.Local
	}
	localDay := day.In(viewerLocation)
	start := time.Date(localDay.Year(), localDay.Month(), localDay.Day(), 0, 0, 0, 0, viewerLocation)
	end := start.AddDate(0, 0, 1)
	scheduleLocation, err := time.LoadLocation(definition.Timezone)
	if err != nil {
		return nil, fmt.Errorf("schedule timezone %q: %w", definition.Timezone, err)
	}
	makeOccurrence := func(at time.Time) Occurrence {
		at = at.UTC()
		return Occurrence{ID: OccurrenceID(definition.ID, at), ScheduleID: definition.ID, ScheduledFor: at, State: OccurrenceScheduled}
	}
	switch definition.Kind {
	case KindOnce:
		at, parseErr := time.Parse(time.RFC3339, definition.Expression)
		if parseErr != nil {
			at, parseErr = time.ParseInLocation("2006-01-02T15:04:05", definition.Expression, scheduleLocation)
		}
		if parseErr != nil {
			return nil, fmt.Errorf("one-shot time %q: %w", definition.Expression, parseErr)
		}
		if !at.Before(start) && at.Before(end) {
			return []Occurrence{makeOccurrence(at)}, nil
		}
		return []Occurrence{}, nil
	case KindCron:
		parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
		schedule, parseErr := parser.Parse(definition.Expression)
		if parseErr != nil {
			return nil, fmt.Errorf("cron expression %q: %w", definition.Expression, parseErr)
		}
		cursor := start.In(scheduleLocation).Add(-time.Second)
		out := []Occurrence{}
		for len(out) < 10_000 {
			next := schedule.Next(cursor)
			if !next.Before(end) {
				break
			}
			if !next.Before(start) {
				out = append(out, makeOccurrence(next))
			}
			cursor = next
		}
		if len(out) == 10_000 {
			return nil, fmt.Errorf("schedule %s exceeds 10000 occurrences in one day", definition.ID)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unknown schedule kind %q", definition.Kind)
	}
}
