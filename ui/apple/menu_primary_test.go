package apple_test

import (
	"strings"
	"testing"
)

func TestMacMenuBarUsesPrimaryGlanceOnly(t *testing.T) {
	menu := readAppleSource(t, "macOS/MenuContent.swift")
	app := readAppleSource(t, "macOS/FortMacApp.swift")

	for _, required := range []string{
		"client.primaryNeedsYou()",
		"client.primaryChannels(state: .open)",
		"model.needsYou",
		"model.channels",
		`Button("Open Fort")`,
		`openWindow(id: "main")`,
		`Button("Quit Fort")`,
	} {
		if !strings.Contains(menu, required) {
			t.Fatalf("Primary menu-bar glance missing %q", required)
		}
	}
	for _, legacy := range []string{
		"client.chat",
		"client.summary",
		"decideGate",
		"GateItem",
		"Request changes",
	} {
		if strings.Contains(menu, legacy) {
			t.Fatalf("Primary menu-bar glance still contains legacy surface %q", legacy)
		}
	}

	for _, required := range []string{
		"@Published var needsYou: [PrimaryNeedsYouItem]",
		"@Published var channels: [PrimaryChannelSummary]",
		"var pendingNeedsYou: Int",
		"model.pendingNeedsYou > 0",
	} {
		if !strings.Contains(app, required) {
			t.Fatalf("Primary menu-bar model missing %q", required)
		}
	}
	if strings.Contains(app, "@Published var summary: Summary?") || strings.Contains(app, "pendingGates") {
		t.Fatal("macOS menu-bar model still badges the legacy gate summary")
	}
}
