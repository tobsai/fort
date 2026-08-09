package control

import (
	"context"
	"fmt"

	"github.com/tobsai/fort/core/scheduler"
	"github.com/tobsai/fort/ui"
)

// ScheduleReadAdapter is the control-plane port adapter for the deterministic
// schedule service. ui owns the HTTP wire types; the service retains its
// control-owned normalized representation and promotion errors.
type ScheduleReadAdapter struct {
	service *ScheduleReadService
}

func NewScheduleReadAdapter(service *ScheduleReadService) *ScheduleReadAdapter {
	return &ScheduleReadAdapter{service: service}
}

func (a *ScheduleReadAdapter) List(ctx context.Context, filter ui.ScheduleFilter) (ui.ScheduleList, error) {
	if a == nil || a.service == nil {
		return ui.ScheduleList{}, fmt.Errorf("schedule read is unavailable")
	}
	value, err := a.service.List(ctx, ScheduleFilter(filter))
	if err != nil {
		return ui.ScheduleList{}, err
	}
	items := make([]ui.ScheduleItem, 0, len(value.Items))
	for _, item := range value.Items {
		items = append(items, uiScheduleItem(item))
	}
	return ui.ScheduleList{SnapshotID: value.SnapshotID, ObservedAt: value.ObservedAt, Items: items}, nil
}

func (a *ScheduleReadAdapter) Get(ctx context.Context, id string) (ui.ScheduleDetail, error) {
	if a == nil || a.service == nil {
		return ui.ScheduleDetail{}, fmt.Errorf("schedule read is unavailable")
	}
	value, err := a.service.Get(ctx, id)
	if err != nil {
		return ui.ScheduleDetail{}, err
	}
	upcoming := append([]scheduler.Occurrence(nil), value.Upcoming...)
	recent := append([]scheduler.Occurrence(nil), value.Recent...)
	if upcoming == nil {
		upcoming = []scheduler.Occurrence{}
	}
	if recent == nil {
		recent = []scheduler.Occurrence{}
	}
	return ui.ScheduleDetail{Item: uiScheduleItem(value.Item), Upcoming: upcoming, Recent: recent}, nil
}

func (a *ScheduleReadAdapter) Occurrences(ctx context.Context, id string, page ui.OccurrencePage) ([]scheduler.Occurrence, error) {
	if a == nil || a.service == nil {
		return nil, fmt.Errorf("schedule read is unavailable")
	}
	return a.service.Occurrences(ctx, id, OccurrencePage{Limit: page.Limit, Before: page.Before, BeforeID: page.BeforeID})
}

func (a *ScheduleReadAdapter) Inventory(ctx context.Context, acceptedDigest string) (ui.ScheduleInventory, error) {
	if a == nil || a.service == nil {
		return ui.ScheduleInventory{}, fmt.Errorf("schedule inventory is unavailable")
	}
	value, err := a.service.Inventory(ctx, acceptedDigest)
	items := make([]ui.ScheduleInventoryItem, 0, len(value.Items))
	for _, item := range value.Items {
		items = append(items, ui.ScheduleInventoryItem{
			ID: item.ID, Kind: item.Kind, Expression: item.Expression, Timezone: item.Timezone,
			FlowID: item.FlowID, FlowDigest: item.FlowDigest,
		})
	}
	return ui.ScheduleInventory{
		CurrentDigest: value.CurrentDigest, AcceptedDigest: value.AcceptedDigest,
		State: ui.ScheduleInventoryState(value.State), Items: items,
	}, err
}

func uiScheduleItem(item ScheduleItem) ui.ScheduleItem {
	var related *ui.RelatedChannel
	if item.RelatedChannel != nil {
		related = &ui.RelatedChannel{ID: item.RelatedChannel.ID, Name: item.RelatedChannel.Name}
	}
	return ui.ScheduleItem{
		ID: item.ID, Title: item.Title, Enabled: item.Enabled, Kind: item.Kind,
		Expression: item.Expression, Recurrence: item.Recurrence, Timezone: item.Timezone,
		NextFireAt: item.NextFireAt, LastFireAt: item.LastFireAt,
		TargetKind: item.TargetKind, TargetID: item.TargetID, RelatedChannel: related,
		LatestOccurrence: item.LatestOccurrence, SchedulerOwnership: ui.SchedulerOwnership(item.SchedulerOwnership),
		ObservedAt: item.ObservedAt,
	}
}
