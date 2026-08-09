package main

import (
	"fmt"
	"net/http"

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

func registerNativeRelayRoutes(mux *http.ServeMux, server *ui.Server, mode ui.PrimaryChannelsMode) error {
	return server.RegisterNativeRelayRoutes(mux, mode)
}

func primaryUISurfaceLabel(mode ui.PrimaryChannelsMode) string {
	if mode == ui.PrimaryChannelsOff {
		return "legacy admin"
	}
	return "private Channels · Scheduled · Needs you · Settings"
}
