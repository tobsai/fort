package apple_test

import (
	"strings"
	"testing"
)

func TestAgentChannelsNativeHostsUseOneClosedPresentationMode(t *testing.T) {
	phone := readAppleSource(t, "iOS/FortApp.swift")
	mac := readAppleSource(t, "macOS/FortMacApp.swift")
	project := readAppleSource(t, "project.yml")

	for _, required := range []string{
		"AgentChannelsPresentationMode.configured",
		"FortMarkSurface",
		"FortNativeChatView(mode: presentationMode)",
		".primaryConnectionSettings",
		".agentConnectionSettings",
		`ProgressView("Connecting to Fort…")`,
		"FortProductMarkView(activity: .ambient",
	} {
		if !strings.Contains(phone, required) {
			t.Errorf("iPhone Agent Channels host missing %q", required)
		}
	}
	for _, required := range []string{
		"AgentChannelsPresentationMode.configured",
		"FortMarkSurface",
		"FortNativeChatView(mode: presentationMode)",
		".fortMarkWindowVisible(mainWindowVisible)",
		"FortWindowVisibilityObserver(isVisible: $mainWindowVisible)",
		".primaryServiceController(service)",
		"MenuContent(mode: presentationMode)",
	} {
		if !strings.Contains(mac, required) {
			t.Errorf("macOS Agent Channels host missing %q", required)
		}
	}
	for _, required := range []string{
		`FORT_AGENT_CHANNELS: "off"`,
		`FORT_AGENT_CHANNELS: "$(FORT_AGENT_CHANNELS)"`,
		"macOS/WindowVisibility.swift",
	} {
		if !strings.Contains(project, required) {
			t.Errorf("Apple activation configuration missing %q", required)
		}
	}
}

func TestMacLivingMarkUsesActualWindowVisibility(t *testing.T) {
	observer := readAppleSource(t, "macOS/WindowVisibility.swift")
	for _, required := range []string{
		"NSWindow.didChangeOcclusionStateNotification",
		"NSWindow.didMiniaturizeNotification",
		"NSWindow.didDeminiaturizeNotification",
		"NSWindow.willCloseNotification",
		"window.isVisible",
		"!window.isMiniaturized",
		"window.occlusionState.contains(.visible)",
		"center.removeObserver",
	} {
		if !strings.Contains(observer, required) {
			t.Errorf("macOS mark visibility observer missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"NSApplication.shared.windows",
		"Timer.",
		"sleep(",
	} {
		if strings.Contains(observer, forbidden) {
			t.Errorf("macOS mark visibility observer uses unbounded mechanism %q", forbidden)
		}
	}
}

func TestMacMenuGlanceMatchesExactPresentationMode(t *testing.T) {
	menu := readAppleSource(t, "macOS/MenuContent.swift")
	for _, required := range []string{
		"let mode: AgentChannelsPresentationMode",
		"case .off:",
		"client.primaryNeedsYou()",
		"client.primaryChannels(state: .open)",
		"case .primary:",
		"client.agentNeedsYou()",
		"client.agentChannels(state: .open)",
		"FortMarkSurface",
		"FortProductMarkView(activity: .ambient",
	} {
		if !strings.Contains(menu, required) {
			t.Errorf("mode-matched menu glance missing %q", required)
		}
	}
	if strings.Contains(menu, "catch { await reloadPrimary") {
		t.Fatal("menu silently falls back across product modes")
	}
}
