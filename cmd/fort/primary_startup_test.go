package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/tobsai/fort/control"
	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/ui"
)

func TestPrimaryPromotionRequiresAcceptedInventoryAndFreshReadySelection(t *testing.T) {
	ready := func(context.Context) (ui.PrimaryAgentView, error) {
		return ui.PrimaryAgentView{
			Selection: &conversation.PrimaryAgentSetting{OptionID: "primary-option:v1:ready"},
			State:     control.PrimaryAgentReady, Options: []ui.PrimaryAgentOption{},
		}, nil
	}
	accepted := ui.ScheduleInventory{State: ui.ScheduleInventoryAccepted, CurrentDigest: "schedule-inventory:v1:accepted"}
	if err := validatePrimaryPromotion(context.Background(), ui.PrimaryChannelsPrimary, accepted, nil, ready); err != nil {
		t.Fatalf("accepted promotion: %v", err)
	}
	if err := validatePrimaryPromotion(context.Background(), ui.PrimaryChannelsPreview, ui.ScheduleInventory{}, control.ErrScheduleInventoryUnaccepted, nil); err != nil {
		t.Fatalf("preview was blocked: %v", err)
	}

	tests := []struct {
		name      string
		inventory ui.ScheduleInventory
		invErr    error
		recheck   func(context.Context) (ui.PrimaryAgentView, error)
		wantCode  string
	}{
		{name: "unaccepted inventory", inventory: ui.ScheduleInventory{State: ui.ScheduleInventoryUnaccepted}, invErr: control.ErrScheduleInventoryUnaccepted, recheck: ready, wantCode: "schedule_inventory_unaccepted"},
		{name: "drifted inventory", inventory: ui.ScheduleInventory{State: ui.ScheduleInventoryDrift}, invErr: control.ErrScheduleInventoryDrift, recheck: ready, wantCode: "schedule_inventory_drift"},
		{name: "missing capability service", inventory: accepted, recheck: nil, wantCode: "primary_agent_unready"},
		{name: "failed recheck", inventory: accepted, recheck: func(context.Context) (ui.PrimaryAgentView, error) { return ui.PrimaryAgentView{}, errors.New("probe") }, wantCode: "primary_agent_unready"},
		{name: "not configured", inventory: accepted, recheck: func(context.Context) (ui.PrimaryAgentView, error) {
			return ui.PrimaryAgentView{State: control.PrimaryAgentNotConfigured}, nil
		}, wantCode: "primary_agent_not_configured"},
		{name: "drifted selection", inventory: accepted, recheck: func(context.Context) (ui.PrimaryAgentView, error) {
			return ui.PrimaryAgentView{Selection: &conversation.PrimaryAgentSetting{OptionID: "x"}, State: control.PrimaryAgentDrifted}, nil
		}, wantCode: "primary_agent_drift"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validatePrimaryPromotion(context.Background(), ui.PrimaryChannelsPrimary, test.inventory, test.invErr, test.recheck)
			if err == nil || !strings.Contains(err.Error(), test.wantCode) {
				t.Fatalf("error=%v, want closed code %q", err, test.wantCode)
			}
		})
	}
}

func TestPrimaryPromotionValidationPrecedesDurableSchedulerStart(t *testing.T) {
	accepted := ui.ScheduleInventory{
		State:         ui.ScheduleInventoryAccepted,
		CurrentDigest: "schedule-inventory:v1:accepted",
	}
	ready := func(context.Context) (ui.PrimaryAgentView, error) {
		return ui.PrimaryAgentView{
			Selection: &conversation.PrimaryAgentSetting{OptionID: "primary-option:v1:ready"},
			State:     control.PrimaryAgentReady,
		}, nil
	}

	started := false
	err := startDurableSchedulerAfterPrimaryPromotion(
		context.Background(), ui.PrimaryChannelsPrimary,
		ui.ScheduleInventory{State: ui.ScheduleInventoryUnaccepted},
		control.ErrScheduleInventoryUnaccepted, ready,
		func(context.Context) error {
			started = true
			return nil
		},
	)
	if err == nil || !strings.Contains(err.Error(), "schedule_inventory_unaccepted") {
		t.Fatalf("error=%v, want schedule_inventory_unaccepted", err)
	}
	if started {
		t.Fatal("durable scheduler started before Primary promotion passed")
	}

	order := []string{}
	err = startDurableSchedulerAfterPrimaryPromotion(
		context.Background(), ui.PrimaryChannelsPrimary, accepted, nil,
		func(context.Context) (ui.PrimaryAgentView, error) {
			order = append(order, "recheck")
			return ready(context.Background())
		},
		func(context.Context) error {
			order = append(order, "start")
			return nil
		},
	)
	if err != nil {
		t.Fatalf("accepted promotion: %v", err)
	}
	if got := strings.Join(order, ","); got != "recheck,start" {
		t.Fatalf("startup order=%q, want recheck,start", got)
	}
}
