package ui_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/tobsai/fort/ui"
)

type primaryInventoryFake struct {
	accepted string
	value    ui.ScheduleInventory
	err      error
}

func (f *primaryInventoryFake) Inventory(_ context.Context, accepted string) (ui.ScheduleInventory, error) {
	f.accepted = accepted
	return f.value, f.err
}

func TestPrimaryAgentSettingsExposeSchedulePromotionInventory(t *testing.T) {
	primary := newPrimaryAPIFake()
	inventory := &primaryInventoryFake{
		value: ui.ScheduleInventory{
			CurrentDigest: "schedule-inventory:v1:current", State: ui.ScheduleInventoryUnaccepted,
			Items: []ui.ScheduleInventoryItem{},
		},
		err: errors.New("schedule_inventory_unaccepted"),
	}
	mux := http.NewServeMux()
	ui.New(ui.Deps{
		Primary: primary, ScheduleInventory: inventory,
		AcceptedScheduleInventory: "schedule-inventory:v1:accepted",
	}).RegisterPrimaryRoutes(mux)
	body := primaryRequest(t, mux, http.MethodGet, "/api/settings/primary-agent", nil, http.StatusOK)
	var response struct {
		ScheduleInventory *ui.ScheduleInventory `json:"schedule_inventory"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatal(err)
	}
	if inventory.accepted != "schedule-inventory:v1:accepted" || response.ScheduleInventory == nil ||
		response.ScheduleInventory.State != ui.ScheduleInventoryUnaccepted || response.ScheduleInventory.Items == nil {
		t.Fatalf("accepted=%q response=%+v", inventory.accepted, response.ScheduleInventory)
	}
}

func TestPrimaryAgentMutationsDoNotReportFailureAfterInventoryProjectionFails(t *testing.T) {
	primary := newPrimaryAPIFake()
	inventory := &primaryInventoryFake{err: errors.New("inventory database unavailable")}
	mux := http.NewServeMux()
	ui.New(ui.Deps{Primary: primary, ScheduleInventory: inventory}).RegisterPrimaryRoutes(mux)

	primaryRequest(t, mux, http.MethodPut, "/api/settings/primary-agent", map[string]any{
		"option_id": "primary-option:v1:accepted",
	}, http.StatusOK)
	primaryRequest(t, mux, http.MethodPost, "/api/settings/primary-agent/recheck", nil, http.StatusOK)
	primaryRequest(t, mux, http.MethodGet, "/api/settings/primary-agent", nil, http.StatusServiceUnavailable)
	if primary.lastOption != "primary-option:v1:accepted" {
		t.Fatalf("setting mutation did not complete: option=%q calls=%v", primary.lastOption, primary.calls)
	}
}
