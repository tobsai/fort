package apple_test

import (
	"strings"
	"testing"
)

func TestMacDirectHostOverrideIsDebugOnly(t *testing.T) {
	app := readAppleSource(t, "macOS/FortMacApp.swift")

	if !strings.Contains(app, "@StateObject private var client = FortMacApp.makeClient()") {
		t.Fatal("FortMac must construct its client through the bounded QA seam")
	}
	debugStart := strings.Index(app, "#if DEBUG")
	if debugStart == -1 {
		t.Fatal("macOS direct-host override must be enclosed by #if DEBUG")
	}
	debugEnd := strings.Index(app[debugStart:], "#endif")
	if debugEnd == -1 {
		t.Fatal("macOS direct-host override must close its #if DEBUG block")
	}
	debugEnd += debugStart
	debugOnly := app[debugStart:debugEnd]
	for _, required := range []string{
		`ProcessInfo.processInfo.environment["FORT_DIRECT_HOST_URL"]`,
		"FortClient(baseURL: directURL)",
	} {
		if !strings.Contains(debugOnly, required) {
			t.Fatalf("DEBUG host seam missing %q", required)
		}
	}
	outsideDebug := app[:debugStart] + app[debugEnd+len("#endif"):]
	if strings.Contains(outsideDebug, "FORT_DIRECT_HOST_URL") {
		t.Fatal("FORT_DIRECT_HOST_URL escaped the DEBUG-only block")
	}
	if !strings.Contains(outsideDebug, "return FortClient()") {
		t.Fatal("production builds must retain the default FortClient host")
	}
}

func TestMacPrimaryUITestUsesIsolatedDirectHost(t *testing.T) {
	uiTest := readAppleSource(t, "macOSUITests/FortMacNavigationTests.swift")
	for _, required := range []string{
		`app.launchEnvironment["FORT_DIRECT_HOST_URL"]`,
		`"http://127.0.0.1:4187"`,
		`app.staticTexts["Needs You"]`,
		`app.staticTexts["Appearance on this device"]`,
	} {
		if !strings.Contains(uiTest, required) {
			t.Fatalf("Primary macOS UI test missing %q", required)
		}
	}
	for _, legacy := range []string{"Projects", "Project rooms", "Playbooks"} {
		if strings.Contains(uiTest, legacy) {
			t.Fatalf("macOS UI test still asserts legacy Command Deck text %q", legacy)
		}
	}
	if strings.Contains(uiTest, `"http://127.0.0.1:4087"`) {
		t.Fatal("macOS UI test must never target the real launchd service")
	}
}

func TestAppleReleaseDocsDescribePrimaryAndOneUpload(t *testing.T) {
	root := readAppleSource(t, "../../README.md")
	readme := readAppleSource(t, "README.md")
	ios := readAppleSource(t, "iOS/README.md")
	macReadme := readAppleSource(t, "macOS/README.md")
	mac := readAppleSource(t, "../../docs/notes/mac-app.md")
	testflight := readAppleSource(t, "../../docs/notes/testflight.md")
	controlPlane := readAppleSource(t, "../../docs/notes/control-plane.md")
	eventContract := readAppleSource(t, "../../docs/notes/event-contract.md")

	for _, legacy := range []string{"five-tab Command Deck", "windowed Command Deck"} {
		if strings.Contains(readme, legacy) {
			t.Fatalf("Apple README still claims %q", legacy)
		}
	}
	for _, legacy := range []string{"`FortWindow`", "gate inbox", "LSUIElement — it appears"} {
		if strings.Contains(mac, legacy) {
			t.Fatalf("Mac release notes still claim legacy surface %q", legacy)
		}
	}
	for _, required := range []string{
		"Primary Channels",
		"FORT_DIRECT_HOST_URL",
		"DEBUG",
		"VERSION=<release-version>",
	} {
		if !strings.Contains(readme+mac, required) {
			t.Fatalf("Apple release docs missing %q", required)
		}
	}
	if strings.Contains(testflight, "xcrun altool --upload-app") {
		t.Fatal("TestFlight notes still direct a duplicate altool upload")
	}
	for _, required := range []string{
		"destination=upload",
		"performs the upload",
	} {
		if !strings.Contains(testflight, required) {
			t.Fatalf("TestFlight notes missing one-upload guidance %q", required)
		}
	}
	for _, forbidden := range []string{
		"FortWatch",
		"FortComplication",
		"watchOS",
		"CarPlay",
		"watch complication",
		"watch |",
	} {
		if strings.Contains(readme+ios+testflight+controlPlane, forbidden) {
			t.Errorf("Apple release/control-plane docs still claim removed surface %q", forbidden)
		}
	}
	for _, forbidden := range []string{
		"BoardView.swift",
		"FeedView.swift",
		"GatesView.swift",
		"Deck, Direction",
		"direct LAN host",
		"NSAppTransportSecurity",
		"NSLocalNetworkUsageDescription",
	} {
		if strings.Contains(ios, forbidden) {
			t.Errorf("iPhone Phase 1 documentation still claims removed capability %q", forbidden)
		}
	}
	for _, required := range []string{
		"Primary Channels",
		"PrimaryChannelsView",
		"FortMacApp.swift",
		"MenuContent.swift",
	} {
		if !strings.Contains(macReadme, required) {
			t.Errorf("Mac Phase 1 documentation missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"FortWindow.swift",
		"Command Deck",
		"Playbooks",
		"Summary.execution",
		"gate quick-approve",
	} {
		if strings.Contains(macReadme, forbidden) {
			t.Errorf("Mac Phase 1 documentation still claims removed capability %q", forbidden)
		}
	}
	for _, stale := range []string{
		"web, iOS, macOS,\nCarPlay, and Apple Watch",
		"iOS / macOS / CarPlay / watch clients",
		"macOS (menu bar), CarPlay, and watchOS",
		"CarPlay ships only with Apple's category-gated entitlement",
	} {
		if strings.Contains(root, stale) {
			t.Errorf("root README still claims removed shipping surface %q", stale)
		}
	}
	for _, stale := range []string{
		"FortMac globs `macOS/*.swift`",
		"sidebar roster from `/api/machines`",
		"full `ASWebAuthenticationSession` OAuth flow is a documented follow-on",
		"service install|start|stop|restart|status|uninstall",
	} {
		if strings.Contains(mac, stale) {
			t.Errorf("Mac release notes still claim removed or already-replaced surface %q", stale)
		}
	}
	for _, stale := range []string{
		"glanceable (watch/CarPlay)",
		"The iOS shell (`ui/ios/`) consumes",
	} {
		if strings.Contains(eventContract, stale) {
			t.Errorf("legacy event-contract note still claims a removed shipping consumer %q", stale)
		}
	}
	for _, stale := range []string{
		"shipping client surfaces: web, iOS, and Mac",
		"All surfaces speak the same HTTP/SSE contract",
	} {
		if strings.Contains(controlPlane, stale) {
			t.Errorf("control-plane note still conflates legacy administration with Phase 1 clients %q", stale)
		}
	}
}
