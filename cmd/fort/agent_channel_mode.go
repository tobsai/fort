package main

import (
	"fmt"

	"github.com/tobsai/fort/ui"
)

func agentChannelsMode(getenv func(string) string) (ui.AgentChannelsMode, error) {
	value := getenv("FORT_AGENT_CHANNELS")
	switch value {
	case "", string(ui.AgentChannelsOff):
		return ui.AgentChannelsOff, nil
	case string(ui.AgentChannelsPrimary):
		return ui.AgentChannelsPrimary, nil
	default:
		return "", fmt.Errorf("FORT_AGENT_CHANNELS must be off or primary (got %q)", value)
	}
}

func validateAgentChannelsCutover(mode ui.ProductMode) error {
	if mode.AgentChannels == ui.AgentChannelsPrimary && mode.PrimaryChannels == ui.PrimaryChannelsOff {
		return fmt.Errorf("FORT_AGENT_CHANNELS=primary requires FORT_PRIMARY_CHANNELS=preview or primary so disabling Agent Channels restores the Primary Channels rollback surface")
	}
	return nil
}

func productUISurfaceLabel(mode ui.ProductMode) string {
	if mode.AgentChannels == ui.AgentChannelsPrimary {
		return "Agent Channels · nested conversations · Needs you"
	}
	return primaryUISurfaceLabel(mode.PrimaryChannels)
}
