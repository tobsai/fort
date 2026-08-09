package ui_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/tobsai/fort/core/scheduler"
	"github.com/tobsai/fort/core/store"
	"github.com/tobsai/fort/ui"
)

type scheduleReadAPI struct {
	list            ui.ScheduleList
	detail          ui.ScheduleDetail
	occurrences     []scheduler.Occurrence
	listErr         error
	detailErr       error
	occurrencesErr  error
	listCalls       int
	detailCalls     int
	occurrenceCalls int
	lastFilter      ui.ScheduleFilter
	lastScheduleID  string
	lastPage        ui.OccurrencePage
}

func (p *scheduleReadAPI) List(_ context.Context, filter ui.ScheduleFilter) (ui.ScheduleList, error) {
	p.listCalls++
	p.lastFilter = filter
	return p.list, p.listErr
}

func (p *scheduleReadAPI) Get(_ context.Context, id string) (ui.ScheduleDetail, error) {
	p.detailCalls++
	p.lastScheduleID = id
	return p.detail, p.detailErr
}

func (p *scheduleReadAPI) Occurrences(_ context.Context, id string, page ui.OccurrencePage) ([]scheduler.Occurrence, error) {
	p.occurrenceCalls++
	p.lastScheduleID, p.lastPage = id, page
	return p.occurrences, p.occurrencesErr
}

func TestScheduleReadListRouteDelegatesFiltersAndKeepsItemsNonNull(t *testing.T) {
	observedAt := time.Date(2026, 8, 8, 18, 0, 0, 0, time.UTC)
	port := &scheduleReadAPI{list: ui.ScheduleList{SnapshotID: "schedule-snapshot:v1:fixture", ObservedAt: observedAt}}
	mux := scheduleReadMux(port)

	for _, test := range []struct {
		path string
		want ui.ScheduleFilter
	}{
		{path: "/api/schedules", want: ui.ScheduleFilterAll},
		{path: "/api/schedules?state=all", want: ui.ScheduleFilterAll},
		{path: "/api/schedules?state=active", want: ui.ScheduleFilterActive},
		{path: "/api/schedules?state=paused", want: ui.ScheduleFilterPaused},
	} {
		body := scheduleHTTP(t, mux, test.path, http.StatusOK)
		if port.lastFilter != test.want {
			t.Fatalf("%s filter = %q, want %q", test.path, port.lastFilter, test.want)
		}
		var got ui.ScheduleList
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatal(err)
		}
		if got.SnapshotID != port.list.SnapshotID || got.Items == nil || len(got.Items) != 0 {
			t.Fatalf("%s response = %+v", test.path, got)
		}
	}

	before := port.listCalls
	for _, path := range []string{
		"/api/schedules?state=",
		"/api/schedules?state=running",
		"/api/schedules?state=all&state=active",
		"/api/schedules?state=all;unexpected=value",
		"/api/schedules?unexpected=value",
	} {
		scheduleHTTP(t, mux, path, http.StatusBadRequest)
	}
	if port.listCalls != before {
		t.Fatalf("invalid list queries delegated %d calls", port.listCalls-before)
	}
}

func TestScheduleReadListReturnsClosedCatalogLimitError(t *testing.T) {
	port := &scheduleReadAPI{listErr: store.ErrScheduleCatalogLimit}
	mux := scheduleReadMux(port)
	body := scheduleHTTP(t, mux, "/api/schedules", http.StatusConflict)
	var response map[string]string
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatal(err)
	}
	if response["code"] != "schedule_catalog_limit" || response["error"] == "" {
		t.Fatalf("catalog error = %+v", response)
	}
}

func TestScheduleReadDetailDelegatesExactIDAndKeepsProjectionsNonNull(t *testing.T) {
	port := &scheduleReadAPI{detail: ui.ScheduleDetail{
		Item: ui.ScheduleItem{ID: "daily", TargetKind: "flow", TargetID: "brief"},
	}}
	mux := scheduleReadMux(port)
	body := scheduleHTTP(t, mux, "/api/schedules/daily", http.StatusOK)
	if port.detailCalls != 1 || port.lastScheduleID != "daily" {
		t.Fatalf("detail delegation = calls=%d id=%q", port.detailCalls, port.lastScheduleID)
	}
	var got ui.ScheduleDetail
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got.Item.TargetKind != "flow" || got.Item.TargetID != "brief" || got.Upcoming == nil || got.Recent == nil {
		t.Fatalf("detail response = %+v", got)
	}

	before := port.detailCalls
	scheduleHTTP(t, mux, "/api/schedules/daily?unexpected=value", http.StatusBadRequest)
	if port.detailCalls != before {
		t.Fatal("invalid detail query reached port")
	}
	port.detailErr = sql.ErrNoRows
	scheduleHTTP(t, mux, "/api/schedules/missing", http.StatusNotFound)
}

func TestScheduleOccurrenceRouteValidatesBoundedExclusiveCursorAndNonNullArray(t *testing.T) {
	port := &scheduleReadAPI{}
	mux := scheduleReadMux(port)
	body := scheduleHTTP(t, mux, "/api/schedules/daily/occurrences", http.StatusOK)
	if string(body) != "[]\n" || port.lastScheduleID != "daily" || port.lastPage.Limit != 50 || !port.lastPage.Before.IsZero() || port.lastPage.BeforeID != "" {
		t.Fatalf("default page = body=%s id=%q page=%+v", body, port.lastScheduleID, port.lastPage)
	}

	before := time.Date(2026, 8, 8, 17, 0, 0, 123_000_000, time.FixedZone("offset", -5*60*60))
	query := url.Values{"limit": {"7"}, "before": {before.Format(time.RFC3339Nano)}, "before_id": {"daily:cursor"}}
	scheduleHTTP(t, mux, "/api/schedules/daily/occurrences?"+query.Encode(), http.StatusOK)
	if port.lastPage.Limit != 7 || !port.lastPage.Before.Equal(before) || port.lastPage.BeforeID != "daily:cursor" {
		t.Fatalf("cursor page = %+v", port.lastPage)
	}

	calls := port.occurrenceCalls
	for _, rawQuery := range []string{
		"limit=0", "limit=51", "limit=word", "limit=1&limit=2", "limit=",
		"before=2026-08-08T18%3A00%3A00Z", "before_id=occurrence",
		"before=not-a-time&before_id=occurrence",
		"before=2026-08-08T18%3A00%3A00Z&before_id=",
		"unexpected=value",
	} {
		scheduleHTTP(t, mux, "/api/schedules/daily/occurrences?"+rawQuery, http.StatusBadRequest)
	}
	if port.occurrenceCalls != calls {
		t.Fatalf("invalid occurrence queries delegated %d calls", port.occurrenceCalls-calls)
	}

	port.occurrencesErr = sql.ErrNoRows
	scheduleHTTP(t, mux, "/api/schedules/missing/occurrences", http.StatusNotFound)
}

func TestScheduleReadRoutesRejectLegacyPostAndMapUnknownFailures(t *testing.T) {
	readPort := &scheduleReadAPI{listErr: errors.New("read failed")}
	legacy := &legacyScheduleAPI{}
	server := ui.New(ui.Deps{ScheduleRead: readPort, Schedules: legacy})
	mux := http.NewServeMux()
	if err := server.RegisterMode(mux, ui.PrimaryChannelsPreview); err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/schedules", strings.NewReader(`{"id":"daily","kind":"cron","expression":"0 0 9 * * *","flow_id":"brief","timezone":"UTC"}`))
	request.RemoteAddr = "127.0.0.1:4000"
	result := httptest.NewRecorder()
	mux.ServeHTTP(result, request)
	if result.Code != http.StatusNotFound || legacy.calls != 0 {
		t.Fatalf("legacy POST status=%d calls=%d body=%s", result.Code, legacy.calls, result.Body.String())
	}
	scheduleHTTP(t, mux, "/api/schedules", http.StatusInternalServerError)
}

func TestScheduleReadRoutesRequireLoopbackPeerAndHost(t *testing.T) {
	for _, path := range []string{
		"/api/schedules",
		"/api/schedules/daily",
		"/api/schedules/daily/occurrences",
	} {
		for _, access := range []struct {
			name       string
			remoteAddr string
			host       string
		}{
			{name: "remote peer", remoteAddr: "192.0.2.25:4000", host: "127.0.0.1:4087"},
			{name: "LAN host", remoteAddr: "127.0.0.1:4000", host: "192.168.1.25:4087"},
		} {
			t.Run(access.name+" "+path, func(t *testing.T) {
				port := &scheduleReadAPI{}
				request := httptest.NewRequest(http.MethodGet, path, nil)
				request.RemoteAddr, request.Host = access.remoteAddr, access.host
				result := httptest.NewRecorder()
				scheduleReadMux(port).ServeHTTP(result, request)
				if result.Code != http.StatusForbidden || port.listCalls != 0 || port.detailCalls != 0 || port.occurrenceCalls != 0 {
					t.Fatalf("status=%d calls=%d/%d/%d body=%s", result.Code, port.listCalls, port.detailCalls, port.occurrenceCalls, result.Body.String())
				}
			})
		}
	}
}

type legacyScheduleAPI struct{ calls int }

func (p *legacyScheduleAPI) Create(_ context.Context, definition scheduler.Definition) (scheduler.Definition, error) {
	p.calls++
	return definition, nil
}

func scheduleReadMux(port ui.ScheduleReadPort) *http.ServeMux {
	server := ui.New(ui.Deps{ScheduleRead: port})
	mux := http.NewServeMux()
	if err := server.RegisterMode(mux, ui.PrimaryChannelsPreview); err != nil {
		panic(err)
	}
	return mux
}

func scheduleHTTP(t *testing.T, handler http.Handler, path string, wantStatus int) []byte {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, path, nil)
	request.RemoteAddr = "127.0.0.1:4000"
	request.Host = "127.0.0.1:4087"
	result := httptest.NewRecorder()
	handler.ServeHTTP(result, request)
	if result.Code != wantStatus {
		t.Fatalf("GET %s status=%d want=%d body=%s", path, result.Code, wantStatus, result.Body.String())
	}
	return result.Body.Bytes()
}
