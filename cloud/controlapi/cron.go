package controlapi

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
	"github.com/tobsai/fort/core/conversation"
)

const (
	// DefaultScheduleCatchUp bounds how much scheduler downtime one invocation
	// reconciles. Older gaps require explicit operator attention rather than an
	// unbounded Vercel invocation.
	DefaultScheduleCatchUp = 5 * time.Minute
	// DefaultScheduleLookAhead materializes the next minute without making its
	// occurrences worker-claimable before their exact due time.
	DefaultScheduleLookAhead = time.Minute
	// MaximumScheduleRoutines and MaximumScheduleOccurrences are hard one-shot
	// work limits. Exceeding either aborts the transaction and its watermark.
	MaximumScheduleRoutines    = 128
	MaximumScheduleOccurrences = 512
	MaximumScheduleLateness    = 90 * time.Second
	// MaximumExpiredWorkerLeaseRecoveries bounds the worker recovery work
	// performed by one scheduled invocation.
	MaximumExpiredWorkerLeaseRecoveries = 128
	// MaximumLateRoutineRunExpirations bounds persisted Routine lateness
	// reconciliation performed by one scheduled invocation.
	MaximumLateRoutineRunExpirations = 128
)

var (
	ErrScheduleClockNotMonotonic = errors.New("schedule tick clock is not monotonic")
	ErrScheduleTickBoundExceeded = errors.New("schedule tick bound exceeded")
	schedulerIDPattern           = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)
)

// RoutineOccurrenceState deliberately separates future materialization from
// work eligibility. Workers may claim only queued occurrences.
type RoutineOccurrenceState string

const (
	OccurrenceScheduled            RoutineOccurrenceState = "scheduled"
	OccurrenceQueued               RoutineOccurrenceState = "queued"
	OccurrenceMissedNeedsAttention RoutineOccurrenceState = "missed_needs_attention"
)

// RoutineSchedule is the immutable scheduling portion of one active
// fort_cloud Routine revision.
type RoutineSchedule struct {
	RoutineID         string
	RoutineRevisionID string
	Expression        string
	Timezone          string
	StartsAt          time.Time
}

// RoutineOccurrence is the exact durable timestamp materialized by a tick.
// OccurrenceID and IdempotencyKey are deterministic functions of RoutineID and
// ScheduledFor, so recycled or overlapping functions converge on one row.
type RoutineOccurrence struct {
	OccurrenceID      string
	RoutineID         string
	RoutineRevisionID string
	ScheduledFor      time.Time
	State             RoutineOccurrenceState
	IdempotencyKey    string
	RecordedAt        time.Time
}

// ScheduleTickTransaction is an account-scoped database transaction holding
// both the advisory lock and the watermark row lock for one scheduler.
type ScheduleTickTransaction interface {
	Watermark(context.Context, string) (time.Time, bool, error)
	RecoverExpiredWorkerLeases(context.Context, time.Time, int) (int, error)
	ExpireLateRoutineRuns(context.Context, time.Time, int) (int, error)
	ActiveRoutineSchedules(context.Context, int) ([]RoutineSchedule, error)
	ApplyOccurrence(context.Context, RoutineOccurrence) (bool, error)
	SaveWatermark(context.Context, string, string, time.Time) error
}

// ScheduleRepository provides one bounded, atomic schedule-tick transaction.
// A false acquired result means another invocation owns the advisory lock.
type ScheduleRepository interface {
	WithScheduleTick(context.Context, string, string, func(ScheduleTickTransaction) error) (acquired bool, err error)
}

// ScheduleTickResult is safe operational metadata returned to Vercel Cron.
type ScheduleTickResult struct {
	Status                 string    `json:"status"`
	TickID                 string    `json:"tick_id,omitempty"`
	Watermark              time.Time `json:"watermark,omitempty"`
	OccurrencesChanged     int       `json:"occurrences_changed"`
	InvalidRoutinesSkipped int       `json:"invalid_routines_skipped"`
	ExpiredLeasesRecovered int       `json:"expired_leases_recovered"`
	LateRoutineRunsExpired int       `json:"late_routine_runs_expired"`
}

// ScheduleTicker is the narrow handler seam; it cannot run a provider or a
// permanent scheduler loop.
type ScheduleTicker interface {
	Tick(context.Context, string, string) (ScheduleTickResult, error)
}

// ScheduleTickService deterministically expands six-field cron schedules
// inside a single repository transaction.
type ScheduleTickService struct {
	Repository ScheduleRepository
	Clock      func() time.Time
	TickIDs    func() string
}

// Tick performs exactly one bounded reconciliation and then returns.
func (service ScheduleTickService) Tick(ctx context.Context, accountID, schedulerID string) (ScheduleTickResult, error) {
	if service.Repository == nil || service.Clock == nil || service.TickIDs == nil {
		return ScheduleTickResult{}, fmt.Errorf("schedule tick service is incomplete")
	}
	parsedAccount, err := uuid.Parse(accountID)
	if err != nil || parsedAccount.String() != accountID {
		return ScheduleTickResult{}, fmt.Errorf("schedule tick account id is invalid")
	}
	if !schedulerIDPattern.MatchString(schedulerID) {
		return ScheduleTickResult{}, fmt.Errorf("schedule tick scheduler id is invalid")
	}
	now := service.Clock().UTC()
	if now.IsZero() {
		return ScheduleTickResult{}, fmt.Errorf("schedule tick clock returned zero time")
	}
	tickID := strings.TrimSpace(service.TickIDs())
	if tickID == "" || len(tickID) > 256 {
		return ScheduleTickResult{}, fmt.Errorf("schedule tick id is invalid")
	}

	result := ScheduleTickResult{Status: "ok", TickID: tickID, Watermark: now}
	acquired, err := service.Repository.WithScheduleTick(ctx, accountID, schedulerID, func(transaction ScheduleTickTransaction) error {
		watermark, exists, err := transaction.Watermark(ctx, schedulerID)
		if err != nil {
			return err
		}
		if exists && !watermark.Before(now) {
			return ErrScheduleClockNotMonotonic
		}
		recovered, err := transaction.RecoverExpiredWorkerLeases(ctx, now, MaximumExpiredWorkerLeaseRecoveries)
		if err != nil {
			return err
		}
		result.ExpiredLeasesRecovered = recovered
		expiredRuns, err := transaction.ExpireLateRoutineRuns(ctx, now, MaximumLateRoutineRunExpirations)
		if err != nil {
			return err
		}
		result.LateRoutineRunsExpired = expiredRuns

		windowStart := now.Add(-DefaultScheduleCatchUp)
		windowEnd := now.Add(DefaultScheduleLookAhead)
		watermarkToSave := now
		if exists {
			windowStart = watermark.UTC()
			if watermark.Before(now.Add(-DefaultScheduleCatchUp)) {
				// Reconcile old gaps in durable slices instead of silently
				// dropping intended occurrences outside the latest window.
				windowEnd = watermark.Add(DefaultScheduleCatchUp).UTC()
				watermarkToSave = windowEnd
			}
		}
		result.Watermark = watermarkToSave

		routines, err := transaction.ActiveRoutineSchedules(ctx, MaximumScheduleRoutines+1)
		if err != nil {
			return err
		}
		if len(routines) > MaximumScheduleRoutines {
			return fmt.Errorf("%w: more than %d active Routines", ErrScheduleTickBoundExceeded, MaximumScheduleRoutines)
		}

		attempted := 0
		for _, routine := range routines {
			schedule, location, err := parseRoutineSchedule(routine)
			if err != nil {
				result.InvalidRoutinesSkipped++
				continue
			}
			routineWindowStart := windowStart
			if !routine.StartsAt.IsZero() && routine.StartsAt.After(routineWindowStart) {
				routineWindowStart = routine.StartsAt.UTC()
			}
			cursor := routineWindowStart.In(location).Add(-time.Nanosecond)
			for dueLocal := schedule.Next(cursor); !dueLocal.After(windowEnd); dueLocal = schedule.Next(cursor) {
				attempted++
				if attempted > MaximumScheduleOccurrences {
					return fmt.Errorf("%w: more than %d occurrences", ErrScheduleTickBoundExceeded, MaximumScheduleOccurrences)
				}
				due := dueLocal.UTC()
				occurrence := newRoutineOccurrence(routine, due, now)
				changed, err := transaction.ApplyOccurrence(ctx, occurrence)
				if err != nil {
					return err
				}
				if changed {
					result.OccurrencesChanged++
				}
				cursor = dueLocal
			}
		}
		return transaction.SaveWatermark(ctx, schedulerID, tickID, watermarkToSave)
	})
	if err != nil {
		return ScheduleTickResult{}, err
	}
	if !acquired {
		return ScheduleTickResult{Status: "overlap_skipped"}, nil
	}
	return result, nil
}

func parseRoutineSchedule(routine RoutineSchedule) (cron.Schedule, *time.Location, error) {
	if strings.TrimSpace(routine.RoutineID) == "" || strings.TrimSpace(routine.RoutineRevisionID) == "" {
		return nil, nil, fmt.Errorf("Routine schedule identity is invalid")
	}
	if err := conversation.ValidateRoutineSchedule(routine.Expression, routine.Timezone); err != nil {
		return nil, nil, fmt.Errorf("Routine %q schedule is invalid: %w", routine.RoutineID, err)
	}
	location, err := time.LoadLocation(routine.Timezone)
	if err != nil {
		return nil, nil, fmt.Errorf("Routine %q timezone is invalid", routine.RoutineID)
	}
	parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	schedule, err := parser.Parse(routine.Expression)
	if err != nil {
		return nil, nil, fmt.Errorf("Routine %q cron expression: %w", routine.RoutineID, err)
	}
	return schedule, location, nil
}

func newRoutineOccurrence(routine RoutineSchedule, due, now time.Time) RoutineOccurrence {
	due = due.UTC()
	exactTimestamp := due.Format(time.RFC3339Nano)
	idempotencyKey := routine.RoutineID + "@" + exactTimestamp
	digest := sha256.Sum256([]byte(idempotencyKey))
	return RoutineOccurrence{
		OccurrenceID:      "routine-occurrence-" + hex.EncodeToString(digest[:16]),
		RoutineID:         routine.RoutineID,
		RoutineRevisionID: routine.RoutineRevisionID,
		ScheduledFor:      due,
		State:             routineOccurrenceState(due, now),
		IdempotencyKey:    idempotencyKey,
		RecordedAt:        now,
	}
}

func routineOccurrenceState(due, now time.Time) RoutineOccurrenceState {
	if due.After(now) {
		return OccurrenceScheduled
	}
	if now.Sub(due) > MaximumScheduleLateness {
		return OccurrenceMissedNeedsAttention
	}
	return OccurrenceQueued
}

// CronHandlerConfig contains no database-selected account input. AccountID is
// trusted server configuration for this single-account first release.
type CronHandlerConfig struct {
	Secret        string
	AuthorityMode string
	AccountID     string
	SchedulerID   string
}

// ScheduleTickerProvider is invoked only after method, secret, authority mode,
// and static scope validation have succeeded.
type ScheduleTickerProvider func(context.Context) (ScheduleTicker, error)

// CronHandler authenticates and runs one bounded tick. In legacy_v1_write
// rollback mode it remains authenticated but performs no database operation.
func CronHandler(config CronHandlerConfig, provider ScheduleTickerProvider) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		response.Header().Set("Cache-Control", "no-store")
		if request.Method != http.MethodGet {
			response.Header().Set("Allow", http.MethodGet)
			writeJSON(response, http.StatusMethodNotAllowed, map[string]string{"code": "method_not_allowed"})
			return
		}
		if config.Secret == "" {
			writeJSON(response, http.StatusServiceUnavailable, map[string]string{"code": "cron_unavailable"})
			return
		}
		wantAuthorization := "Bearer " + config.Secret
		authorizations := request.Header.Values("Authorization")
		gotAuthorization := request.Header.Get("Authorization")
		if len(authorizations) != 1 || len(gotAuthorization) != len(wantAuthorization) ||
			subtle.ConstantTimeCompare([]byte(gotAuthorization), []byte(wantAuthorization)) != 1 {
			response.Header().Set("WWW-Authenticate", "Bearer")
			writeJSON(response, http.StatusUnauthorized, map[string]string{"code": "cron_unauthorized"})
			return
		}
		if config.AuthorityMode == "legacy_v1_write" {
			writeJSON(response, http.StatusOK, ScheduleTickResult{Status: "disabled"})
			return
		}
		parsedAccount, accountErr := uuid.Parse(config.AccountID)
		if config.AuthorityMode != "cloud_v2_write" || accountErr != nil || parsedAccount.String() != config.AccountID ||
			!schedulerIDPattern.MatchString(config.SchedulerID) || provider == nil {
			writeJSON(response, http.StatusServiceUnavailable, map[string]string{"code": "cron_unavailable"})
			return
		}
		ticker, err := provider(request.Context())
		if err != nil || ticker == nil {
			writeJSON(response, http.StatusServiceUnavailable, map[string]string{"code": "cron_unavailable"})
			return
		}
		result, err := ticker.Tick(request.Context(), config.AccountID, config.SchedulerID)
		if err != nil {
			writeJSON(response, http.StatusServiceUnavailable, map[string]string{"code": "cron_tick_failed"})
			return
		}
		writeJSON(response, http.StatusOK, result)
	})
}
