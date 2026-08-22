package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tobsai/fort/cloud/controlapi"
)

const cronEndpointAccountID = "4af424a4-d81a-47d5-a495-400868883b86"

func TestCronEndpointRejectsSecretBeforeOpeningDatabase(t *testing.T) {
	t.Parallel()

	values := map[string]string{
		"CRON_SECRET":          "exact-secret",
		"FORT_AUTHORITY_MODE":  "cloud_v2_write",
		"FORT_CRON_ACCOUNT_ID": cronEndpointAccountID,
		"DATABASE_URL":         "postgresql://runtime.test/fort?sslmode=require",
	}
	opens := 0
	handler := newCronEndpoint(func(key string) string { return values[key] }, func(context.Context, string, string) (scheduleStore, error) {
		opens++
		return &fakeScheduleStore{}, nil
	})

	for _, authorization := range []string{"", "Bearer wrong", "Bearer exact-secret "} {
		request := httptest.NewRequest(http.MethodGet, "/api/v2/cron/tick", nil)
		request.Header.Set("Authorization", authorization)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("authorization %q status = %d, want 401", authorization, response.Code)
		}
	}
	if opens != 0 {
		t.Fatalf("invalid secrets opened database %d times", opens)
	}
}

func TestCronEndpointMissingConfiguredSecretFailsBeforeOpeningDatabase(t *testing.T) {
	t.Parallel()

	values := map[string]string{
		"FORT_AUTHORITY_MODE":  "cloud_v2_write",
		"FORT_CRON_ACCOUNT_ID": cronEndpointAccountID,
		"DATABASE_URL":         "postgresql://runtime.test/fort?sslmode=require",
	}
	opens := 0
	handler := newCronEndpoint(func(key string) string { return values[key] }, func(context.Context, string, string) (scheduleStore, error) {
		opens++
		return &fakeScheduleStore{}, nil
	})
	request := httptest.NewRequest(http.MethodGet, "/api/v2/cron/tick", nil)
	request.Header.Set("Authorization", "Bearer any-value")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable || opens != 0 {
		t.Fatalf("missing CRON_SECRET status/opens = %d/%d, want 503/0", response.Code, opens)
	}
}

func TestCronEndpointRollbackModeDoesNotOpenDatabase(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		mode       string
		wantStatus int
	}{{"legacy_v1_write", http.StatusOK}, {"", http.StatusServiceUnavailable}, {"future_mode", http.StatusServiceUnavailable}} {
		values := map[string]string{
			"CRON_SECRET":          "exact-secret",
			"FORT_AUTHORITY_MODE":  test.mode,
			"FORT_CRON_ACCOUNT_ID": cronEndpointAccountID,
			"DATABASE_URL":         "postgresql://runtime.test/fort?sslmode=require",
		}
		opens := 0
		handler := newCronEndpoint(func(key string) string { return values[key] }, func(context.Context, string, string) (scheduleStore, error) {
			opens++
			return &fakeScheduleStore{}, nil
		})
		request := httptest.NewRequest(http.MethodGet, "/api/v2/cron/tick", nil)
		request.Header.Set("Authorization", "Bearer exact-secret")
		response := httptest.NewRecorder()

		handler.ServeHTTP(response, request)

		if response.Code != test.wantStatus || opens != 0 {
			t.Fatalf("mode %q status/opens = %d/%d, want %d/0", test.mode, response.Code, opens, test.wantStatus)
		}
	}
}

func TestCronEndpointRunsBoundedTickAndReusesWarmStore(t *testing.T) {
	t.Parallel()

	values := map[string]string{
		"CRON_SECRET":          "exact-secret",
		"FORT_AUTHORITY_MODE":  "cloud_v2_write",
		"FORT_CRON_ACCOUNT_ID": cronEndpointAccountID,
		"DATABASE_URL":         "postgresql://runtime.test/fort?sslmode=require",
	}
	store := &fakeScheduleStore{}
	opens := 0
	handler := newCronEndpoint(func(key string) string { return values[key] }, func(_ context.Context, databaseURL, accountID string) (scheduleStore, error) {
		opens++
		if databaseURL != values["DATABASE_URL"] || accountID != cronEndpointAccountID {
			t.Fatalf("open scope = %q/%q", databaseURL, accountID)
		}
		return store, nil
	})

	for index := 0; index < 2; index++ {
		request := httptest.NewRequest(http.MethodGet, "/api/v2/cron/tick", nil)
		request.Header.Set("Authorization", "Bearer exact-secret")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("request %d status = %d body=%s", index, response.Code, response.Body.String())
		}
	}
	if opens != 1 || store.ticks != 2 {
		t.Fatalf("opens/ticks = %d/%d, want 1/2", opens, store.ticks)
	}
}

func TestCronEndpointInvalidConfigurationFailsBeforeOpen(t *testing.T) {
	t.Parallel()

	values := map[string]string{
		"CRON_SECRET":          "exact-secret",
		"FORT_AUTHORITY_MODE":  "cloud_v2_write",
		"FORT_CRON_ACCOUNT_ID": cronEndpointAccountID,
	}
	opens := 0
	handler := newCronEndpoint(func(key string) string { return values[key] }, func(context.Context, string, string) (scheduleStore, error) {
		opens++
		return &fakeScheduleStore{}, nil
	})
	request := httptest.NewRequest(http.MethodGet, "/api/v2/cron/tick", nil)
	request.Header.Set("Authorization", "Bearer exact-secret")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable || opens != 0 {
		t.Fatalf("invalid config status/opens = %d/%d, want 503/0", response.Code, opens)
	}
}

type fakeScheduleStore struct {
	watermark time.Time
	ticks     int
}

func (store *fakeScheduleStore) WithScheduleTick(
	_ context.Context,
	_, _ string,
	operation func(controlapi.ScheduleTickTransaction) error,
) (bool, error) {
	store.ticks++
	return true, operation(fakeScheduleTransaction{store: store})
}

func (store *fakeScheduleStore) Close() error { return nil }

type fakeScheduleTransaction struct{ store *fakeScheduleStore }

func (transaction fakeScheduleTransaction) Watermark(context.Context, string) (time.Time, bool, error) {
	return transaction.store.watermark, !transaction.store.watermark.IsZero(), nil
}

func (fakeScheduleTransaction) ActiveRoutineSchedules(context.Context, int) ([]controlapi.RoutineSchedule, error) {
	return []controlapi.RoutineSchedule{}, nil
}

func (fakeScheduleTransaction) RecoverExpiredWorkerLeases(context.Context, time.Time, int) (int, error) {
	return 0, nil
}

func (fakeScheduleTransaction) ExpireLateRoutineRuns(context.Context, time.Time, int) (int, error) {
	return 0, nil
}

func (fakeScheduleTransaction) ApplyOccurrence(context.Context, controlapi.RoutineOccurrence) (bool, error) {
	return false, nil
}

func (transaction fakeScheduleTransaction) SaveWatermark(_ context.Context, _, _ string, watermark time.Time) error {
	transaction.store.watermark = watermark
	return nil
}
