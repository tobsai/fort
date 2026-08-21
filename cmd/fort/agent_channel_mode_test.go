package main

import (
	"strings"
	"testing"

	"github.com/tobsai/fort/ui"
)

func TestAgentChannelsModeIsClosedAndOffByDefault(t *testing.T) {
	tests := []struct {
		value   string
		want    ui.AgentChannelsMode
		wantErr bool
	}{
		{value: "", want: ui.AgentChannelsOff},
		{value: "off", want: ui.AgentChannelsOff},
		{value: "primary", want: ui.AgentChannelsPrimary},
		{value: "on", wantErr: true},
		{value: "preview", wantErr: true},
		{value: "Primary", wantErr: true},
		{value: " primary", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.value, func(t *testing.T) {
			got, err := agentChannelsMode(func(string) string { return test.value })
			if (err != nil) != test.wantErr || (!test.wantErr && got != test.want) {
				t.Fatalf("mode=%q error=%v, want %q error=%v", got, err, test.want, test.wantErr)
			}
		})
	}
}

func TestProductUISurfaceLabelMakesAgentCutoverVisible(t *testing.T) {
	if got := productUISurfaceLabel(ui.ProductMode{
		PrimaryChannels: ui.PrimaryChannelsPreview,
		AgentChannels:   ui.AgentChannelsPrimary,
	}); got != "Agent Channels · nested conversations · Needs you" {
		t.Fatalf("Agent Channel surface label = %q", got)
	}
	if got := productUISurfaceLabel(ui.ProductMode{
		PrimaryChannels: ui.PrimaryChannelsPrimary,
		AgentChannels:   ui.AgentChannelsOff,
	}); got != primaryUISurfaceLabel(ui.PrimaryChannelsPrimary) {
		t.Fatalf("Primary compatibility surface label = %q", got)
	}
}

func TestAgentCutoverModeRequiresPrimaryRollbackSurface(t *testing.T) {
	if err := validateAgentChannelsCutover(ui.ProductMode{
		PrimaryChannels: ui.PrimaryChannelsOff,
		AgentChannels:   ui.AgentChannelsPrimary,
	}); err == nil || !strings.Contains(err.Error(), "FORT_PRIMARY_CHANNELS") {
		t.Fatalf("Agent cutover validation error = %v", err)
	}
	if err := validateAgentChannelsCutover(ui.ProductMode{
		PrimaryChannels: ui.PrimaryChannelsPreview,
		AgentChannels:   ui.AgentChannelsPrimary,
	}); err != nil {
		t.Fatalf("preview-backed Agent cutover rejected: %v", err)
	}
}
