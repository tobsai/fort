package main

import (
	"testing"

	"github.com/tobsai/fort/ui"
)

func TestPrimaryChannelsModeIsClosedAndOffByDefault(t *testing.T) {
	tests := []struct {
		value   string
		want    ui.PrimaryChannelsMode
		wantErr bool
	}{
		{value: "", want: ui.PrimaryChannelsOff},
		{value: "off", want: ui.PrimaryChannelsOff},
		{value: "preview", want: ui.PrimaryChannelsPreview},
		{value: "primary", want: ui.PrimaryChannelsPrimary},
		{value: "on", wantErr: true},
		{value: "Preview", wantErr: true},
		{value: " preview", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.value, func(t *testing.T) {
			got, err := primaryChannelsMode(func(string) string { return test.value })
			if (err != nil) != test.wantErr || (!test.wantErr && got != test.want) {
				t.Fatalf("mode=%q error=%v, want %q error=%v", got, err, test.want, test.wantErr)
			}
		})
	}
}

func TestPrimaryUISurfaceLabelMatchesMountedProduct(t *testing.T) {
	tests := []struct {
		mode ui.PrimaryChannelsMode
		want string
	}{
		{mode: ui.PrimaryChannelsOff, want: "legacy admin"},
		{mode: ui.PrimaryChannelsPreview, want: "private Channels · Scheduled · Needs you · Settings"},
		{mode: ui.PrimaryChannelsPrimary, want: "private Channels · Scheduled · Needs you · Settings"},
	}
	for _, test := range tests {
		if got := primaryUISurfaceLabel(test.mode); got != test.want {
			t.Errorf("mode %q UI label = %q, want %q", test.mode, got, test.want)
		}
	}
}
