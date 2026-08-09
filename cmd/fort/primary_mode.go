package main

import (
	"fmt"

	"github.com/tobsai/fort/ui"
)

func primaryChannelsMode(getenv func(string) string) (ui.PrimaryChannelsMode, error) {
	value := getenv("FORT_PRIMARY_CHANNELS")
	switch value {
	case "", string(ui.PrimaryChannelsOff):
		return ui.PrimaryChannelsOff, nil
	case string(ui.PrimaryChannelsPreview):
		return ui.PrimaryChannelsPreview, nil
	case string(ui.PrimaryChannelsPrimary):
		return ui.PrimaryChannelsPrimary, nil
	default:
		return "", fmt.Errorf("FORT_PRIMARY_CHANNELS must be off, preview, or primary (got %q)", value)
	}
}
