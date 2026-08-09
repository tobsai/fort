package apple_test

import (
	"os"
	"strings"
	"testing"
)

func TestPhase1AppleProjectShipsOnlyApprovedTargetsAndSources(t *testing.T) {
	project := readAppleSource(t, "project.yml")

	for _, forbidden := range []string{
		"FortWatch:",
		"FortComplication:",
		"CarPlay/",
		"CPTemplateApplicationSceneSessionRoleApplication",
		"watchOS:",
		"LSUIElement — no dock icon",
	} {
		if strings.Contains(project, forbidden) {
			t.Errorf("Phase 1 Apple project still ships %q", forbidden)
		}
	}

	phone := between(t, project, "  Fort:\n", "  FortMac:\n")
	for _, required := range []string{
		"- path: iOS/FortApp.swift",
		"- path: iOS/GatewayCoordinator.swift",
		"- path: iOS/Assets.xcassets",
	} {
		if !strings.Contains(phone, required) {
			t.Errorf("iPhone Phase 1 source allowlist missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"- path: iOS\n",
		"BoardView.swift",
		"FeedView.swift",
		"GatesView.swift",
		"FortWatch",
		"CarPlay",
	} {
		if strings.Contains(phone, forbidden) {
			t.Errorf("iPhone Phase 1 target still includes %q", forbidden)
		}
	}

	mac := between(t, project, "  FortMac:\n", "  FortMacUITests:\n")
	for _, required := range []string{
		"- path: macOS/FortMacApp.swift",
		"- path: macOS/MenuContent.swift",
		"- path: macOS/Assets.xcassets",
	} {
		if !strings.Contains(mac, required) {
			t.Errorf("Mac Phase 1 source allowlist missing %q", required)
		}
	}
	for _, forbidden := range []string{"- path: macOS\n", "FortWindow.swift"} {
		if strings.Contains(mac, forbidden) {
			t.Errorf("Mac Phase 1 target still includes %q", forbidden)
		}
	}
}

func TestPhase1AppleTreeHasNoDormantConstrainedOrCommandDeckScreens(t *testing.T) {
	for _, path := range []string{
		"CarPlay",
		"watch",
		"Support/Complication-Info.plist",
		"Support/FortComplicationBundle.swift",
		"Support/FortWatch-Info.plist",
		"iOS/BoardView.swift",
		"iOS/FeedView.swift",
		"iOS/GatesView.swift",
		"macOS/FortWindow.swift",
	} {
		if _, err := os.Stat(path); err == nil {
			t.Errorf("out-of-scope Apple surface still exists at %s", path)
		} else if !os.IsNotExist(err) {
			t.Fatalf("inspect %s: %v", path, err)
		}
	}
}

func TestPhase1IPhoneManifestDoesNotClaimLocalNetworkOrCarPlay(t *testing.T) {
	plist := readAppleSource(t, "Support/Fort-iOS-Info.plist")
	for _, forbidden := range []string{
		"NSAppTransportSecurity",
		"NSLocalNetworkUsageDescription",
		"UIApplicationSceneManifest",
		"CPTemplateApplicationSceneSessionRoleApplication",
		"FortCarPlay",
	} {
		if strings.Contains(plist, forbidden) {
			t.Errorf("Phase 1 iPhone manifest still contains %q", forbidden)
		}
	}
}
