package control

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/tobsai/fort/core/graph"
	"github.com/tobsai/fort/core/scheduler"
	"github.com/tobsai/fort/core/store"
)

type fakeScheduleReadRepository struct {
	rows             []store.ScheduleReadRow
	detail           store.ScheduleReadDetail
	occurrences      []scheduler.Occurrence
	catalogErr       error
	detailErr        error
	occurrencesErr   error
	catalogCalls     int
	detailCalls      int
	occurrenceCalls  int
	lastEnabled      *bool
	lastCatalogLimit int
	lastDetailAt     time.Time
	lastDetailLimit  int
	lastPageLimit    int
	lastBefore       time.Time
	lastBeforeID     string
}

func (r *fakeScheduleReadRepository) ReadScheduleCatalog(_ context.Context, enabled *bool, limit int) ([]store.ScheduleReadRow, error) {
	r.catalogCalls++
	if enabled != nil {
		copyEnabled := *enabled
		r.lastEnabled = &copyEnabled
	} else {
		r.lastEnabled = nil
	}
	r.lastCatalogLimit = limit
	return append([]store.ScheduleReadRow(nil), r.rows...), r.catalogErr
}

func (r *fakeScheduleReadRepository) ReadScheduleDetail(_ context.Context, _ string, observedAt time.Time, limit int) (store.ScheduleReadDetail, error) {
	r.detailCalls++
	r.lastDetailAt, r.lastDetailLimit = observedAt, limit
	return r.detail, r.detailErr
}

func (r *fakeScheduleReadRepository) ReadScheduleOccurrences(_ context.Context, _ string, limit int, before time.Time, beforeID string) ([]scheduler.Occurrence, error) {
	r.occurrenceCalls++
	r.lastPageLimit, r.lastBefore, r.lastBeforeID = limit, before, beforeID
	return append([]scheduler.Occurrence(nil), r.occurrences...), r.occurrencesErr
}

func TestScheduleReadListUsesCanonicalBucketsAndStableSnapshot(t *testing.T) {
	observedAt := time.Date(2026, 8, 8, 18, 0, 0, 123, time.UTC)
	repository := &fakeScheduleReadRepository{rows: []store.ScheduleReadRow{
		{Definition: scheduleDefinition("paused", false, time.Time{}, observedAt.Add(4*time.Hour))},
		{Definition: scheduleDefinition("active-no-next-a", true, time.Time{}, observedAt.Add(2*time.Hour))},
		{Definition: scheduleDefinition("active-next-b", true, observedAt.Add(time.Hour), observedAt)},
		{Definition: scheduleDefinition("active-no-next-b", true, time.Time{}, observedAt.Add(3*time.Hour))},
		{
			Definition:       scheduleDefinition("active-next-a", true, observedAt.Add(30*time.Minute), observedAt),
			LatestOccurrence: &scheduler.Occurrence{ID: "occ-1", ScheduleID: "active-next-a", RunID: "run-1", ScheduledFor: observedAt.Add(-time.Hour), State: scheduler.OccurrenceFailed, Error: "boom"},
			RelatedChannel:   &store.ScheduleChannelLink{ID: "channel-1", Name: "Morning brief"},
		},
	}}
	service := NewScheduleReadService(repository, SchedulerOwnershipActive, nil)
	service.now = func() time.Time { return observedAt }

	list, err := service.List(context.Background(), ScheduleFilterAll)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if repository.catalogCalls != 1 || repository.lastEnabled != nil || repository.lastCatalogLimit != MaxScheduleDefinitions {
		t.Fatalf("catalog call = calls=%d enabled=%v limit=%d", repository.catalogCalls, repository.lastEnabled, repository.lastCatalogLimit)
	}
	wantOrder := []string{"active-next-a", "active-next-b", "active-no-next-b", "active-no-next-a", "paused"}
	if len(list.Items) != len(wantOrder) {
		t.Fatalf("items = %+v", list.Items)
	}
	for index, want := range wantOrder {
		if list.Items[index].ID != want {
			t.Fatalf("items[%d].id = %q, want %q", index, list.Items[index].ID, want)
		}
		if !list.Items[index].ObservedAt.Equal(observedAt) {
			t.Fatalf("items[%d].observed_at = %s", index, list.Items[index].ObservedAt)
		}
		if list.Items[index].TargetKind != "flow" || list.Items[index].TargetID != "flow-"+want {
			t.Fatalf("items[%d] target = %q/%q", index, list.Items[index].TargetKind, list.Items[index].TargetID)
		}
		if list.Items[index].SchedulerOwnership != SchedulerOwnershipActive {
			t.Fatalf("items[%d] ownership = %q", index, list.Items[index].SchedulerOwnership)
		}
	}
	linked := list.Items[0]
	if linked.RelatedChannel == nil || linked.RelatedChannel.ID != "channel-1" || linked.LatestOccurrence == nil || linked.LatestOccurrence.RunID != "run-1" {
		t.Fatalf("linked item = %+v", linked)
	}
	if list.Items[1].RelatedChannel != nil {
		t.Fatalf("unlinked item invented Channel = %+v", list.Items[1].RelatedChannel)
	}
	if list.SnapshotID == "" || list.SnapshotID[:len("schedule-snapshot:v1:")] != "schedule-snapshot:v1:" {
		t.Fatalf("snapshot id = %q", list.SnapshotID)
	}

	service.now = func() time.Time { return observedAt.Add(time.Hour) }
	second, err := service.List(context.Background(), ScheduleFilterAll)
	if err != nil {
		t.Fatal(err)
	}
	if second.SnapshotID != list.SnapshotID {
		t.Fatalf("observation time changed durable snapshot id: first=%q second=%q", list.SnapshotID, second.SnapshotID)
	}
	if second.ObservedAt.Equal(list.ObservedAt) {
		t.Fatal("fresh observation did not change observed_at")
	}
	repository.rows[4].LatestOccurrence.Error = "changed"
	changed, err := service.List(context.Background(), ScheduleFilterAll)
	if err != nil {
		t.Fatal(err)
	}
	if changed.SnapshotID == second.SnapshotID {
		t.Fatalf("changed returned evidence retained snapshot id %q", changed.SnapshotID)
	}
}

func TestScheduleReadDefendsHardCatalogLimitWithoutTruncation(t *testing.T) {
	rows := make([]store.ScheduleReadRow, MaxScheduleDefinitions+1)
	for index := range rows {
		rows[index].Definition = scheduler.Definition{ID: fmt.Sprintf("schedule-%04d", index), Enabled: true}
	}
	repository := &fakeScheduleReadRepository{rows: rows}
	service := NewScheduleReadService(repository, SchedulerOwnershipUnknown, nil)
	if list, err := service.List(context.Background(), ScheduleFilterAll); !errors.Is(err, store.ErrScheduleCatalogLimit) || list.Items != nil {
		t.Fatalf("over-limit list = %+v err=%v", list, err)
	}
	if inventory, err := service.Inventory(context.Background(), ""); !errors.Is(err, store.ErrScheduleCatalogLimit) || inventory.Items != nil {
		t.Fatalf("over-limit inventory = %+v err=%v", inventory, err)
	}
}

func TestScheduleReadFiltersDetailAndOccurrenceValidation(t *testing.T) {
	observedAt := time.Date(2026, 8, 8, 18, 0, 0, 0, time.UTC)
	row := store.ScheduleReadRow{Definition: scheduleDefinition("daily", true, observedAt.Add(time.Hour), observedAt)}
	repository := &fakeScheduleReadRepository{
		rows:        []store.ScheduleReadRow{row},
		detail:      store.ScheduleReadDetail{Row: row, Upcoming: nil, Recent: nil},
		occurrences: nil,
	}
	service := NewScheduleReadService(repository, SchedulerOwnershipInactive, nil)
	service.now = func() time.Time { return observedAt }

	if _, err := service.List(context.Background(), ScheduleFilter("enabled")); !errors.Is(err, ErrInvalidScheduleFilter) || repository.catalogCalls != 0 {
		t.Fatalf("invalid filter = err=%v calls=%d", err, repository.catalogCalls)
	}
	if _, err := service.List(context.Background(), ScheduleFilterActive); err != nil || repository.lastEnabled == nil || !*repository.lastEnabled {
		t.Fatalf("active filter = enabled=%v err=%v", repository.lastEnabled, err)
	}
	if _, err := service.List(context.Background(), ScheduleFilterPaused); err != nil || repository.lastEnabled == nil || *repository.lastEnabled {
		t.Fatalf("paused filter = enabled=%v err=%v", repository.lastEnabled, err)
	}

	detail, err := service.Get(context.Background(), "daily")
	if err != nil {
		t.Fatal(err)
	}
	if detail.Upcoming == nil || detail.Recent == nil || len(detail.Upcoming) != 0 || len(detail.Recent) != 0 {
		t.Fatalf("detail arrays must be non-null empty slices: %+v", detail)
	}
	if repository.lastDetailLimit != ScheduleDetailOccurrenceLimit || !repository.lastDetailAt.Equal(observedAt) || detail.Item.SchedulerOwnership != SchedulerOwnershipInactive {
		t.Fatalf("detail bounds/ownership = %+v repo=%+v", detail, repository)
	}

	invalidPages := []OccurrencePage{
		{Limit: 0}, {Limit: 51},
		{Limit: 10, BeforeID: "occ"},
		{Limit: 10, Before: observedAt},
	}
	for _, page := range invalidPages {
		if _, err := service.Occurrences(context.Background(), "daily", page); !errors.Is(err, ErrInvalidOccurrencePage) {
			t.Fatalf("page %+v error = %v", page, err)
		}
	}
	if repository.occurrenceCalls != 0 {
		t.Fatalf("invalid pages reached repository %d times", repository.occurrenceCalls)
	}
	items, err := service.Occurrences(context.Background(), "daily", OccurrencePage{Limit: 50, Before: observedAt, BeforeID: "occ"})
	if err != nil || items == nil || repository.lastPageLimit != 50 || repository.lastBeforeID != "occ" {
		t.Fatalf("valid page = items=%+v err=%v repo=%+v", items, err, repository)
	}
	if ownership := NewScheduleReadService(repository, SchedulerOwnershipUnknown, nil); ownership.ownership != SchedulerOwnershipUnknown {
		t.Fatalf("unknown ownership changed to %q", ownership.ownership)
	}
}

func TestScheduleInventoryExactDigestsAndClosedComparison(t *testing.T) {
	repository := &fakeScheduleReadRepository{}
	service := NewScheduleReadService(repository, SchedulerOwnershipUnknown, map[string]string{})
	empty, err := service.Inventory(context.Background(), "")
	if !errors.Is(err, ErrScheduleInventoryUnaccepted) {
		t.Fatalf("empty unaccepted error = %v", err)
	}
	const emptyDigest = "schedule-inventory:v1:7d5bf4173fd97e9d036d7acd974925bbc4b2ed0553c0c8e9e9ed210d9cea7b76"
	if empty.CurrentDigest != emptyDigest || empty.Items == nil || len(empty.Items) != 0 || empty.State != ScheduleInventoryUnaccepted {
		t.Fatalf("empty inventory = %+v", empty)
	}
	accepted, err := service.Inventory(context.Background(), emptyDigest)
	if err != nil || accepted.State != ScheduleInventoryAccepted || accepted.AcceptedDigest != emptyDigest {
		t.Fatalf("accepted empty inventory = %+v err=%v", accepted, err)
	}
	if drift, err := service.Inventory(context.Background(), "schedule-inventory:v1:wrong"); !errors.Is(err, ErrScheduleInventoryDrift) || drift.State != ScheduleInventoryDrift {
		t.Fatalf("mismatched inventory = %+v err=%v", drift, err)
	}

	definition := graph.Flow{
		ID: "brief", Name: "Brief", Start: "draft",
		Nodes: []graph.Node{{ID: "draft", Type: graph.Task}},
	}
	flowDigest, err := FlowDefinitionDigest(definition)
	if err != nil {
		t.Fatal(err)
	}
	const expectedFlowDigest = "flow-definition:v1:0dcc8c3e3b59ed6990b5464989a313c149a9f35fbc0da5c399a0dd80faf0a9be"
	if flowDigest != expectedFlowDigest {
		t.Fatalf("flow digest = %q, want exact %q", flowDigest, expectedFlowDigest)
	}
	repository.rows = []store.ScheduleReadRow{
		{Definition: scheduler.Definition{ID: "z-paused", Kind: scheduler.KindOnce, Expression: "2099-01-01T00:00:00Z", Timezone: "UTC", FlowID: "ignored", Enabled: false}},
		{Definition: scheduler.Definition{ID: "b", Kind: scheduler.KindCron, Expression: "0 0 9 * * *", Timezone: "America/Chicago", FlowID: "brief", Enabled: true}},
		{Definition: scheduler.Definition{ID: "a", Kind: scheduler.KindOnce, Expression: "2099-01-01T00:00:00Z", Timezone: "UTC", FlowID: "brief", Enabled: true}},
	}
	service = NewScheduleReadService(repository, SchedulerOwnershipUnknown, map[string]string{"brief": flowDigest})
	inventory, err := service.Inventory(context.Background(), "")
	if !errors.Is(err, ErrScheduleInventoryUnaccepted) {
		t.Fatalf("nonempty unaccepted error = %v", err)
	}
	if len(inventory.Items) != 2 || inventory.Items[0].ID != "a" || inventory.Items[1].ID != "b" || repository.lastEnabled == nil || !*repository.lastEnabled {
		t.Fatalf("enabled canonical rows = %+v filter=%v", inventory.Items, repository.lastEnabled)
	}
	if inventory.Items[0].FlowDigest != flowDigest || inventory.CurrentDigest == emptyDigest {
		t.Fatalf("inventory digest/flow = %+v", inventory)
	}
	const expectedInventoryDigest = "schedule-inventory:v1:506831d1a0596591f92f192e19fe7e664f2f670f534d7c3ef0f4dd9115643820"
	if inventory.CurrentDigest != expectedInventoryDigest {
		t.Fatalf("inventory digest = %q, want exact %q", inventory.CurrentDigest, expectedInventoryDigest)
	}
	accepted, err = service.Inventory(context.Background(), inventory.CurrentDigest)
	if err != nil || accepted.State != ScheduleInventoryAccepted {
		t.Fatalf("exact inventory acceptance = %+v err=%v", accepted, err)
	}

	service = NewScheduleReadService(repository, SchedulerOwnershipUnknown, map[string]string{})
	missing, err := service.Inventory(context.Background(), inventory.CurrentDigest)
	if !errors.Is(err, ErrScheduleInventoryDrift) || missing.State != ScheduleInventoryDrift ||
		!strings.HasPrefix(missing.CurrentDigest, "schedule-inventory:v1:") || missing.CurrentDigest == inventory.CurrentDigest {
		t.Fatalf("missing flow digest = %+v err=%v", missing, err)
	}
}

func TestFlowDefinitionDigestRejectsInvalidFlowAndChangesWithDefinition(t *testing.T) {
	if _, err := FlowDefinitionDigest(graph.Flow{ID: "broken", Start: "missing"}); err == nil {
		t.Fatal("invalid loaded flow received a digest")
	}
	first := graph.Flow{ID: "brief", Name: "Brief", Start: "draft", Nodes: []graph.Node{{ID: "draft", Type: graph.Task}}}
	second := first
	second.Name = "Changed"
	firstDigest, err := FlowDefinitionDigest(first)
	if err != nil {
		t.Fatal(err)
	}
	secondDigest, err := FlowDefinitionDigest(second)
	if err != nil {
		t.Fatal(err)
	}
	if firstDigest == secondDigest {
		t.Fatalf("changed loaded flow retained digest %q", firstDigest)
	}
}

func scheduleDefinition(id string, enabled bool, nextFireAt, updatedAt time.Time) scheduler.Definition {
	return scheduler.Definition{
		ID: id, Title: id, Kind: scheduler.KindCron, Expression: "0 0 * * * *", FlowID: "flow-" + id,
		Timezone: "America/Chicago", Enabled: enabled, NextFireAt: nextFireAt,
		CreatedAt: updatedAt.Add(-time.Hour), UpdatedAt: updatedAt,
	}
}
