package controlapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const cronTestAccountID = "4af424a4-d81a-47d5-a495-400868883b86"

func TestCronHandlerRejectsMissingOrInexactSecretBeforeTickProvider(t *testing.T) {
	t.Parallel()

	providerCalls := 0
	handler := CronHandler(CronHandlerConfig{
		Secret: "an-exact-cron-secret", AuthorityMode: "cloud_v2_write",
		AccountID: cronTestAccountID, SchedulerID: "fort-cloud",
	}, func(context.Context) (ScheduleTicker, error) {
		providerCalls++
		return &fakeScheduleTicker{}, nil
	})

	for _, authorization := range []string{
		"", "an-exact-cron-secret", "Bearer wrong", "bearer an-exact-cron-secret",
		"Bearer an-exact-cron-secret ", "Bearer  an-exact-cron-secret",
	} {
		request := httptest.NewRequest(http.MethodGet, "/api/v2/cron/tick", nil)
		request.Header.Set("Authorization", authorization)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("authorization %q status = %d, want 401", authorization, response.Code)
		}
	}
	multiple := httptest.NewRequest(http.MethodGet, "/api/v2/cron/tick", nil)
	multiple.Header.Add("Authorization", "Bearer an-exact-cron-secret")
	multiple.Header.Add("Authorization", "Bearer an-exact-cron-secret")
	multipleResponse := httptest.NewRecorder()
	handler.ServeHTTP(multipleResponse, multiple)
	if multipleResponse.Code != http.StatusUnauthorized {
		t.Fatalf("multiple Authorization headers status = %d, want 401", multipleResponse.Code)
	}
	if providerCalls != 0 {
		t.Fatalf("invalid secrets reached tick provider %d times", providerCalls)
	}
}

func TestCronHandlerRollbackModeIsAuthenticatedAndDatabaseFree(t *testing.T) {
	t.Parallel()

	providerCalls := 0
	handler := CronHandler(CronHandlerConfig{
		Secret: "rollback-secret", AuthorityMode: "legacy_v1_write",
		AccountID: cronTestAccountID, SchedulerID: "fort-cloud",
	}, func(context.Context) (ScheduleTicker, error) {
		providerCalls++
		return &fakeScheduleTicker{}, nil
	})
	request := httptest.NewRequest(http.MethodGet, "/api/v2/cron/tick", nil)
	request.Header.Set("Authorization", "Bearer rollback-secret")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK || providerCalls != 0 {
		t.Fatalf("rollback status/provider calls = %d/%d, want 200/0", response.Code, providerCalls)
	}
	if !strings.Contains(response.Body.String(), `"status":"disabled"`) {
		t.Fatalf("rollback body = %s", response.Body.String())
	}
}

func TestCronHandlerRunsOneBoundedTick(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 20, 30, 0, 0, time.UTC)
	ticker := &fakeScheduleTicker{result: ScheduleTickResult{
		Status: "ok", TickID: "tick-1", Watermark: now, OccurrencesChanged: 2,
	}}
	providerCalls := 0
	handler := CronHandler(CronHandlerConfig{
		Secret: "valid-secret", AuthorityMode: "cloud_v2_write",
		AccountID: cronTestAccountID, SchedulerID: "fort-cloud",
	}, func(context.Context) (ScheduleTicker, error) {
		providerCalls++
		return ticker, nil
	})
	request := httptest.NewRequest(http.MethodGet, "/api/v2/cron/tick", nil)
	request.Header.Set("Authorization", "Bearer valid-secret")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK || providerCalls != 1 || ticker.calls != 1 {
		t.Fatalf("status/provider/tick calls = %d/%d/%d, want 200/1/1", response.Code, providerCalls, ticker.calls)
	}
	if ticker.accountID != cronTestAccountID || ticker.schedulerID != "fort-cloud" {
		t.Fatalf("tick scope = %q/%q", ticker.accountID, ticker.schedulerID)
	}
	var body ScheduleTickResult
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.TickID != "tick-1" || body.OccurrencesChanged != 2 {
		t.Fatalf("tick response = %+v", body)
	}
}

func TestScheduleTickMaterializesBoundedCatchUpAndLookAhead(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 20, 30, 0, 0, time.UTC)
	repository := &memoryScheduleRepository{
		watermark: now.Add(-2 * time.Minute),
		routines: []RoutineSchedule{{
			RoutineID: "routine-1", RoutineRevisionID: "routine-revision-1",
			Expression: "0 * * * * *", Timezone: "UTC",
		}},
	}
	service := ScheduleTickService{
		Repository: repository,
		Clock:      func() time.Time { return now },
		TickIDs:    func() string { return "tick-1" },
	}

	result, err := service.Tick(context.Background(), cronTestAccountID, "fort-cloud")
	if err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if result.Status != "ok" || !result.Watermark.Equal(now) || repository.savedTickID != "tick-1" {
		t.Fatalf("tick result/repository = %+v tick=%q", result, repository.savedTickID)
	}
	if len(repository.occurrences) != 4 {
		t.Fatalf("occurrence count = %d, want 4 (two catch-up, due, look-ahead): %+v", len(repository.occurrences), repository.occurrences)
	}
	wantTimes := []time.Time{
		now.Add(-2 * time.Minute), now.Add(-time.Minute), now, now.Add(time.Minute),
	}
	wantStates := []RoutineOccurrenceState{
		OccurrenceMissedNeedsAttention, OccurrenceQueued, OccurrenceQueued, OccurrenceScheduled,
	}
	for index, occurrence := range repository.occurrences {
		if !occurrence.ScheduledFor.Equal(wantTimes[index]) || occurrence.State != wantStates[index] {
			t.Fatalf("occurrence %d = %+v, want %s/%s", index, occurrence, wantTimes[index], wantStates[index])
		}
		if occurrence.OccurrenceID == "" || occurrence.IdempotencyKey == "" {
			t.Fatalf("occurrence %d lacks deterministic identity: %+v", index, occurrence)
		}
	}
	if repository.occurrences[0].State != OccurrenceMissedNeedsAttention {
		t.Fatal("an occurrence more than 90 seconds late was allowed to queue")
	}
	if repository.occurrences[1].State != OccurrenceQueued {
		t.Fatal("an occurrence inside the 90-second boundary was not queueable")
	}
	if repository.occurrences[3].State != OccurrenceScheduled {
		t.Fatal("a look-ahead occurrence was made worker-claimable before due time")
	}
}

func TestScheduleTickRecoversExpiredWorkerLeasesBeforeScheduling(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 20, 30, 0, 0, time.UTC)
	repository := &memoryScheduleRepository{
		watermark:     now.Add(-time.Minute),
		expiredLeases: 3,
		routines:      []RoutineSchedule{},
	}
	service := ScheduleTickService{
		Repository: repository,
		Clock:      func() time.Time { return now },
		TickIDs:    func() string { return "tick-recovery" },
	}

	result, err := service.Tick(context.Background(), cronTestAccountID, "fort-cloud")
	if err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if result.ExpiredLeasesRecovered != 3 {
		t.Fatalf("expired leases recovered = %d, want 3", result.ExpiredLeasesRecovered)
	}
	if !repository.recoveryObservedAt.Equal(now) || repository.recoveryLimit != MaximumExpiredWorkerLeaseRecoveries {
		t.Fatalf("recovery scope = %s/%d, want %s/%d", repository.recoveryObservedAt,
			repository.recoveryLimit, now, MaximumExpiredWorkerLeaseRecoveries)
	}
}

func TestScheduleTickExpiresUnclaimedRoutineRunsPastLatenessPolicy(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 20, 30, 0, 0, time.UTC)
	repository := &memoryScheduleRepository{
		watermark:       now.Add(-time.Minute),
		expiredLateRuns: 2,
		routines:        []RoutineSchedule{},
	}
	service := ScheduleTickService{Repository: repository, Clock: func() time.Time { return now }, TickIDs: func() string { return "tick-late-recovery" }}

	result, err := service.Tick(context.Background(), cronTestAccountID, "fort-cloud")
	if err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if result.LateRoutineRunsExpired != 2 {
		t.Fatalf("late Routine runs expired = %d, want 2", result.LateRoutineRunsExpired)
	}
	if !repository.lateObservedAt.Equal(now) || repository.lateLimit != MaximumLateRoutineRunExpirations {
		t.Fatalf("late recovery scope = %s/%d", repository.lateObservedAt, repository.lateLimit)
	}
}

func TestRoutineOccurrenceStateUsesExactNinetySecondBoundary(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 20, 30, 0, 0, time.UTC)
	if got := routineOccurrenceState(now.Add(-90*time.Second), now); got != OccurrenceQueued {
		t.Fatalf("exact 90-second state = %q, want queued", got)
	}
	if got := routineOccurrenceState(now.Add(-90*time.Second-time.Nanosecond), now); got != OccurrenceMissedNeedsAttention {
		t.Fatalf("beyond 90-second state = %q, want missed_needs_attention", got)
	}
}

func TestScheduleTickKeepsRoutineTimezoneAcrossEveryOccurrence(t *testing.T) {
	t.Parallel()

	// August 21 is daylight time in Chicago: local 09:30 is 14:30 UTC.
	now := time.Date(2026, 8, 21, 14, 30, 0, 0, time.UTC)
	repository := &memoryScheduleRepository{
		watermark: now.Add(-2 * time.Minute),
		routines: []RoutineSchedule{{
			RoutineID: "local-business-hour", RoutineRevisionID: "revision-1",
			Expression: "0 * 9 * * *", Timezone: "America/Chicago",
		}},
	}
	service := ScheduleTickService{Repository: repository, Clock: func() time.Time { return now }, TickIDs: func() string { return "tick-timezone" }}

	if _, err := service.Tick(context.Background(), cronTestAccountID, "fort-cloud"); err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if len(repository.occurrences) != 4 {
		t.Fatalf("timezone occurrence count = %d, want four local-hour minutes: %+v", len(repository.occurrences), repository.occurrences)
	}
	for _, occurrence := range repository.occurrences {
		if occurrence.ScheduledFor.Hour() != 14 {
			t.Fatalf("local 09:xx occurrence drifted out of 14:xx UTC: %s", occurrence.ScheduledFor)
		}
	}
}

func TestScheduleTickSkipsMalformedStoredRoutineWithoutAbortingValidSchedules(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 20, 30, 0, 0, time.UTC)
	repository := &memoryScheduleRepository{
		watermark: now.Add(-time.Minute),
		routines: []RoutineSchedule{
			{RoutineID: "broken", RoutineRevisionID: "revision-broken", Expression: "0 9 * * 1", Timezone: "UTC"},
			{RoutineID: "valid", RoutineRevisionID: "revision-valid", Expression: "0 * * * * *", Timezone: "UTC"},
		},
	}
	service := ScheduleTickService{Repository: repository, Clock: func() time.Time { return now }, TickIDs: func() string { return "tick-malformed" }}

	result, err := service.Tick(context.Background(), cronTestAccountID, "fort-cloud")
	if err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if result.InvalidRoutinesSkipped != 1 {
		t.Fatalf("invalid Routines skipped = %d, want 1", result.InvalidRoutinesSkipped)
	}
	if len(repository.occurrences) != 3 {
		t.Fatalf("valid Routine occurrences = %+v, want three", repository.occurrences)
	}
	for _, occurrence := range repository.occurrences {
		if occurrence.RoutineID != "valid" {
			t.Fatalf("malformed Routine materialized an occurrence: %+v", occurrence)
		}
	}
	if repository.savedTickID != "tick-malformed" {
		t.Fatalf("malformed sibling prevented watermark commit: %q", repository.savedTickID)
	}
}

func TestScheduleTickNeverBackfillsBeforeRoutineRevisionStart(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 20, 30, 0, 0, time.UTC)
	repository := &memoryScheduleRepository{
		watermark: now.Add(-2 * time.Minute),
		routines: []RoutineSchedule{{
			RoutineID: "new-routine", RoutineRevisionID: "revision-1",
			Expression: "0 * * * * *", Timezone: "UTC", StartsAt: now,
		}},
	}
	service := ScheduleTickService{Repository: repository, Clock: func() time.Time { return now }, TickIDs: func() string { return "tick-new-routine" }}

	if _, err := service.Tick(context.Background(), cronTestAccountID, "fort-cloud"); err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if len(repository.occurrences) != 2 || repository.occurrences[0].ScheduledFor.Before(now) {
		t.Fatalf("new Routine was backfilled before its revision start: %+v", repository.occurrences)
	}
}

func TestScheduleTickAdvancesOldWatermarkInBoundedSlicesWithoutDroppingOccurrences(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 20, 30, 0, 0, time.UTC)
	repository := &memoryScheduleRepository{
		watermark: now.Add(-10 * time.Minute),
		routines: []RoutineSchedule{{
			RoutineID: "delayed-routine", RoutineRevisionID: "revision-1",
			Expression: "0 * * * * *", Timezone: "UTC",
		}},
	}
	tickNumber := 0
	service := ScheduleTickService{Repository: repository, Clock: func() time.Time { return now }, TickIDs: func() string {
		tickNumber++
		return "catch-up-tick-" + string(rune('0'+tickNumber))
	}}

	first, err := service.Tick(context.Background(), cronTestAccountID, "fort-cloud")
	if err != nil {
		t.Fatalf("first Tick: %v", err)
	}
	if !first.Watermark.Equal(now.Add(-5*time.Minute)) || len(repository.occurrences) != 6 {
		t.Fatalf("first catch-up slice = %+v occurrences=%d", first, len(repository.occurrences))
	}
	for _, occurrence := range repository.occurrences {
		if occurrence.State != OccurrenceMissedNeedsAttention {
			t.Fatalf("old catch-up occurrence silently queued: %+v", occurrence)
		}
	}
	second, err := service.Tick(context.Background(), cronTestAccountID, "fort-cloud")
	if err != nil {
		t.Fatalf("second Tick: %v", err)
	}
	if !second.Watermark.Equal(now) || len(repository.occurrences) != 12 {
		t.Fatalf("second catch-up slice = %+v occurrences=%d", second, len(repository.occurrences))
	}
	if repository.occurrences[len(repository.occurrences)-1].State != OccurrenceScheduled {
		t.Fatalf("caught-up slice did not restore bounded look-ahead: %+v", repository.occurrences)
	}
}

func TestScheduleTickDuplicateAndOverlapCannotDuplicateOccurrences(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 20, 30, 0, 0, time.UTC)
	repository := &memoryScheduleRepository{
		watermark: now.Add(-time.Minute),
		routines: []RoutineSchedule{{
			RoutineID: "routine-1", RoutineRevisionID: "revision-1",
			Expression: "0 * * * * *", Timezone: "UTC",
		}},
	}
	currentTime := now
	tickNumber := 0
	service := ScheduleTickService{Repository: repository, Clock: func() time.Time { return currentTime }, TickIDs: func() string {
		tickNumber++
		return "tick-" + string(rune('0'+tickNumber))
	}}

	first, err := service.Tick(context.Background(), cronTestAccountID, "fort-cloud")
	if err != nil {
		t.Fatalf("first Tick: %v", err)
	}
	identities := make(map[string]struct{}, len(repository.occurrences))
	for _, occurrence := range repository.occurrences {
		identities[occurrence.OccurrenceID] = struct{}{}
	}
	currentTime = now.Add(time.Second)
	if _, err := service.Tick(context.Background(), cronTestAccountID, "fort-cloud"); err != nil {
		t.Fatalf("sequential duplicate Tick: %v", err)
	}
	if len(repository.occurrences) != len(identities) || first.OccurrencesChanged != len(identities) {
		t.Fatalf("duplicate tick changed occurrences: %+v", repository.occurrences)
	}

	repository.lockAvailable = boolPointer(false)
	result, err := service.Tick(context.Background(), cronTestAccountID, "fort-cloud")
	if err != nil || result.Status != "overlap_skipped" {
		t.Fatalf("overlap result/error = %+v/%v", result, err)
	}
	if repository.transactionCalls != 2 {
		t.Fatalf("tick callback calls = %d, want 2 (overlap never enters callback)", repository.transactionCalls)
	}
}

func TestScheduleTickFailsWithoutAdvancingWatermarkWhenBoundExceeded(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 20, 30, 0, 0, time.UTC)
	repository := &memoryScheduleRepository{
		watermark: now.Add(-DefaultScheduleCatchUp),
		routines: []RoutineSchedule{
			{RoutineID: "every-second-1", RoutineRevisionID: "revision-1", Expression: "* * * * * *", Timezone: "UTC"},
			{RoutineID: "every-second-2", RoutineRevisionID: "revision-2", Expression: "* * * * * *", Timezone: "UTC"},
		},
	}
	service := ScheduleTickService{Repository: repository, Clock: func() time.Time { return now }, TickIDs: func() string { return "tick-overflow" }}

	_, err := service.Tick(context.Background(), cronTestAccountID, "fort-cloud")
	if !errors.Is(err, ErrScheduleTickBoundExceeded) {
		t.Fatalf("Tick error = %v, want bound exceeded", err)
	}
	if repository.savedTickID != "" || !repository.savedWatermark.IsZero() {
		t.Fatalf("failed tick advanced watermark to %s/%q", repository.savedWatermark, repository.savedTickID)
	}
	if len(repository.occurrences) != 0 {
		t.Fatalf("failed tick committed %d partial occurrences", len(repository.occurrences))
	}
}

type fakeScheduleTicker struct {
	result      ScheduleTickResult
	err         error
	calls       int
	accountID   string
	schedulerID string
}

func (ticker *fakeScheduleTicker) Tick(_ context.Context, accountID, schedulerID string) (ScheduleTickResult, error) {
	ticker.calls++
	ticker.accountID, ticker.schedulerID = accountID, schedulerID
	return ticker.result, ticker.err
}

type memoryScheduleRepository struct {
	watermark          time.Time
	routines           []RoutineSchedule
	occurrences        []RoutineOccurrence
	savedWatermark     time.Time
	savedTickID        string
	lockAvailable      *bool
	transactionCalls   int
	expiredLeases      int
	recoveryObservedAt time.Time
	recoveryLimit      int
	expiredLateRuns    int
	lateObservedAt     time.Time
	lateLimit          int
}

func (repository *memoryScheduleRepository) WithScheduleTick(
	_ context.Context,
	_, _ string,
	operation func(ScheduleTickTransaction) error,
) (bool, error) {
	if repository.lockAvailable != nil && !*repository.lockAvailable {
		return false, nil
	}
	repository.transactionCalls++
	working := *repository
	working.occurrences = append([]RoutineOccurrence{}, repository.occurrences...)
	transaction := &memoryScheduleTransaction{repository: &working}
	if err := operation(transaction); err != nil {
		return true, err
	}
	*repository = working
	return true, nil
}

type memoryScheduleTransaction struct{ repository *memoryScheduleRepository }

func (transaction *memoryScheduleTransaction) RecoverExpiredWorkerLeases(_ context.Context, observedAt time.Time, limit int) (int, error) {
	transaction.repository.recoveryObservedAt = observedAt
	transaction.repository.recoveryLimit = limit
	return transaction.repository.expiredLeases, nil
}

func (transaction *memoryScheduleTransaction) ExpireLateRoutineRuns(_ context.Context, observedAt time.Time, limit int) (int, error) {
	transaction.repository.lateObservedAt = observedAt
	transaction.repository.lateLimit = limit
	return transaction.repository.expiredLateRuns, nil
}

func (transaction *memoryScheduleTransaction) Watermark(context.Context, string) (time.Time, bool, error) {
	watermark := transaction.repository.watermark
	return watermark, !watermark.IsZero(), nil
}

func (transaction *memoryScheduleTransaction) ActiveRoutineSchedules(context.Context, int) ([]RoutineSchedule, error) {
	return append([]RoutineSchedule{}, transaction.repository.routines...), nil
}

func (transaction *memoryScheduleTransaction) ApplyOccurrence(_ context.Context, occurrence RoutineOccurrence) (bool, error) {
	for index, existing := range transaction.repository.occurrences {
		if existing.RoutineID == occurrence.RoutineID && existing.ScheduledFor.Equal(occurrence.ScheduledFor) {
			if existing.OccurrenceID != occurrence.OccurrenceID || existing.RoutineRevisionID != occurrence.RoutineRevisionID ||
				existing.IdempotencyKey != occurrence.IdempotencyKey {
				return false, errors.New("occurrence identity conflict")
			}
			if (existing.State == OccurrenceScheduled && occurrence.State != OccurrenceScheduled) ||
				(existing.State == OccurrenceQueued && occurrence.State == OccurrenceMissedNeedsAttention) {
				transaction.repository.occurrences[index].State = occurrence.State
				return true, nil
			}
			return false, nil
		}
	}
	transaction.repository.occurrences = append(transaction.repository.occurrences, occurrence)
	return true, nil
}

func (transaction *memoryScheduleTransaction) SaveWatermark(_ context.Context, schedulerID, tickID string, watermark time.Time) error {
	transaction.repository.savedWatermark = watermark
	transaction.repository.savedTickID = tickID
	transaction.repository.watermark = watermark
	return nil
}

func boolPointer(value bool) *bool { return &value }
