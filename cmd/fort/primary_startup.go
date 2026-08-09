package main

import (
	"context"
	"fmt"

	"github.com/tobsai/fort/control"
	"github.com/tobsai/fort/ui"
)

type primaryAgentRecheck func(context.Context) (ui.PrimaryAgentView, error)
type durableSchedulerStart func(context.Context) error

func startDurableSchedulerAfterPrimaryPromotion(
	ctx context.Context,
	mode ui.PrimaryChannelsMode,
	inventory ui.ScheduleInventory,
	inventoryErr error,
	recheck primaryAgentRecheck,
	start durableSchedulerStart,
) error {
	if err := validatePrimaryPromotion(ctx, mode, inventory, inventoryErr, recheck); err != nil {
		return fmt.Errorf("serve: Primary Channels promotion blocked: %w", err)
	}
	if start == nil {
		return fmt.Errorf("start durable scheduler: unavailable")
	}
	if err := start(ctx); err != nil {
		return fmt.Errorf("start durable scheduler: %w", err)
	}
	return nil
}

func validatePrimaryPromotion(
	ctx context.Context,
	mode ui.PrimaryChannelsMode,
	inventory ui.ScheduleInventory,
	inventoryErr error,
	recheck primaryAgentRecheck,
) error {
	if mode != ui.PrimaryChannelsPrimary {
		return nil
	}
	if inventoryErr != nil || inventory.State != ui.ScheduleInventoryAccepted {
		code := "schedule_inventory_drift"
		if inventory.State == ui.ScheduleInventoryUnaccepted {
			code = "schedule_inventory_unaccepted"
		}
		if inventoryErr != nil {
			return fmt.Errorf("%s: %w", code, inventoryErr)
		}
		return fmt.Errorf("%s", code)
	}
	if recheck == nil {
		return fmt.Errorf("%s: capability service unavailable", control.ErrorPrimaryAgentUnready)
	}
	view, err := recheck(ctx)
	if err != nil {
		return fmt.Errorf("%s: %w", control.ErrorPrimaryAgentUnready, err)
	}
	if view.Selection == nil {
		return fmt.Errorf("%s", control.ErrorPrimaryAgentNotConfigured)
	}
	switch view.State {
	case control.PrimaryAgentReady:
		return nil
	case control.PrimaryAgentDrifted:
		return fmt.Errorf("%s", control.ErrorPrimaryAgentDrift)
	default:
		return fmt.Errorf("%s", control.ErrorPrimaryAgentUnready)
	}
}
