package control

import (
	"context"
	"errors"
	"testing"

	"github.com/tobsai/fort/ui"
)

var _ ui.ScheduleReadPort = (*ScheduleReadAdapter)(nil)
var _ ui.ScheduleInventoryPort = (*ScheduleReadAdapter)(nil)

func TestScheduleReadAdapterProjectsInventoryEvenWhenReviewIsRequired(t *testing.T) {
	service := NewScheduleReadService(&fakeScheduleReadRepository{}, SchedulerOwnershipActive, map[string]string{})
	adapter := NewScheduleReadAdapter(service)
	inventory, err := adapter.Inventory(context.Background(), "")
	if !errors.Is(err, ErrScheduleInventoryUnaccepted) {
		t.Fatalf("inventory error = %v", err)
	}
	if inventory.State != ui.ScheduleInventoryUnaccepted || inventory.CurrentDigest == "" || inventory.Items == nil {
		t.Fatalf("inventory = %+v", inventory)
	}
	list, err := adapter.List(context.Background(), ui.ScheduleFilterAll)
	if err != nil || list.Items == nil || list.ObservedAt.IsZero() {
		t.Fatalf("list = %+v error=%v", list, err)
	}
}
