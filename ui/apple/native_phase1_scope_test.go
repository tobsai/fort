package apple_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFortKitShippingSurfaceContainsOnlyPhaseOneEndpoints(t *testing.T) {
	client := readAppleSource(t, "FortKit/Sources/FortKit/FortClient.swift")
	for _, endpoint := range []string{
		`"/api/summary"`,
		`"/api/board"`,
		`"/api/gates"`,
		`"/api/runs/`,
		`"/api/backlog`,
		`"/api/profiles"`,
		`"/api/metrics`,
		`"/api/playbooks`,
		`"/api/chat"`,
		`"/api/route"`,
		`"/api/openclaw"`,
		`"/api/breakdown"`,
		`"/api/gate"`,
		`"/api/events`,
	} {
		if strings.Contains(client, endpoint) {
			t.Errorf("FortKit still exposes deferred native endpoint %s", endpoint)
		}
	}

	for _, endpoint := range []string{
		`"/api/settings/primary-agent"`,
		`"/api/channels?state=`,
		`"/api/needs-you"`,
		`"/api/schedules?state=`,
	} {
		if !strings.Contains(client, endpoint) {
			t.Errorf("FortKit lost Phase 1 endpoint %s", endpoint)
		}
	}
}

func TestFortKitRemovesLegacyCommandDeckSources(t *testing.T) {
	for _, path := range []string{
		"FortKit/Sources/FortKit/CommandDeck.swift",
		"FortKit/Sources/FortKit/CommandDeckStyle.swift",
		"FortKit/Sources/FortKit/Models.swift",
	} {
		if _, err := os.Stat(path); err == nil {
			t.Errorf("legacy FortKit source still ships: %s", path)
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}

	styleMatches, err := filepath.Glob("FortKit/Sources/FortKit/*Style.swift")
	if err != nil {
		t.Fatal(err)
	}
	if len(styleMatches) != 1 || filepath.Base(styleMatches[0]) != "PrimaryChannelsStyle.swift" {
		t.Fatalf("FortKit Phase 1 style sources = %v, want only PrimaryChannelsStyle.swift", styleMatches)
	}
}

func TestMacServiceUIExposesRecoveryWithoutAdminTeardown(t *testing.T) {
	view := readAppleSource(t, "FortKit/Sources/FortKit/PrimaryChannelsView.swift")
	controller := readAppleSource(t, "FortKit/Sources/FortKit/ServiceController.swift")
	app := readAppleSource(t, "macOS/FortMacApp.swift")

	for _, required := range []string{
		`Button("Install")`,
		`Button("Start")`,
		`Button("Restart")`,
		`service.status.running`,
		`service.install()`,
		`service.start()`,
		`service.restart()`,
	} {
		if !strings.Contains(view, required) {
			t.Errorf("Mac recovery surface missing %q", required)
		}
	}
	for _, forbidden := range []string{
		`Button("Stop")`,
		`Button("Uninstall")`,
		`service.stop()`,
		`service.uninstall()`,
		"service.status.detail",
		"confirmServiceUninstall",
	} {
		if strings.Contains(view, forbidden) {
			t.Errorf("Mac Phase 1 Settings still expose admin control %q", forbidden)
		}
	}
	for _, forbidden := range []string{
		`func stop()`,
		`func uninstall()`,
		`["service", "stop"]`,
		`["service", "uninstall"]`,
		`public func run(_ args: [String])`,
	} {
		if strings.Contains(controller, forbidden) {
			t.Errorf("ServiceController still exposes deferred teardown %q", forbidden)
		}
	}
	for _, stale := range []string{
		"install/start/stop/restart",
		`window's sidebar "Service" controls`,
	} {
		if strings.Contains(app, stale) {
			t.Errorf("FortMac root still documents removed service UI %q", stale)
		}
	}
}

func TestFortClientKeepsTransportMutationInsideClosedActions(t *testing.T) {
	client := readAppleSource(t, "FortKit/Sources/FortKit/FortClient.swift")
	if !strings.Contains(client, "@Published public private(set) var baseURL: URL") {
		t.Fatal("FortClient baseURL must be readable for transport identity but not arbitrarily mutable")
	}
	if strings.Contains(client, "@Published public var baseURL: URL") {
		t.Fatal("FortClient still exposes arbitrary host mutation")
	}
}

func TestReleaseConnectionCopyDoesNotPromiseDirectHost(t *testing.T) {
	view := readAppleSource(t, "FortKit/Sources/FortKit/PrimaryChannelsView.swift")
	if strings.Contains(view, "choose an explicit direct host") {
		t.Fatal("shared release Settings still promises a DEBUG-only direct-host action")
	}
}

func TestFortKitPackageTargetsOnlyPhaseOneApplePlatforms(t *testing.T) {
	manifest := readAppleSource(t, "FortKit/Package.swift")
	for _, required := range []string{".iOS(.v16)", ".macOS(.v13)"} {
		if !strings.Contains(manifest, required) {
			t.Errorf("FortKit manifest lost Phase 1 platform %q", required)
		}
	}
	if strings.Contains(manifest, ".watchOS(") {
		t.Fatal("FortKit still advertises a deferred watchOS product surface")
	}

	view := readAppleSource(t, "FortKit/Sources/FortKit/PrimaryChannelsView.swift")
	for _, stale := range []string{
		"Watch and CarPlay",
		"#if !os(watchOS)",
		"relay/direct-host management",
	} {
		if strings.Contains(view, stale) {
			t.Errorf("PrimaryChannelsView still carries deferred-platform claim %q", stale)
		}
	}

	service := readAppleSource(t, "FortKit/Sources/FortKit/ServiceController.swift")
	if strings.Contains(service, "watchOS") {
		t.Fatal("macOS-only ServiceController still documents a removed watchOS target")
	}
}

func TestFortKitReadmeDocumentsOnlyPhaseOneContract(t *testing.T) {
	readme := readAppleSource(t, "FortKit/README.md")
	for _, required := range []string{
		"iOS 16+",
		"macOS 13+",
		"same-host",
		"PrimaryChannels.swift",
		"PrimaryChannelsView.swift",
		"GatewayRelay.swift",
		"ServiceController.swift",
		"/api/settings/primary-agent",
		"/api/channels",
	} {
		if !strings.Contains(readme, required) {
			t.Errorf("FortKit README lost Phase 1 contract %q", required)
		}
	}
	for _, stale := range []string{
		"every Fort Apple surface",
		"watchOS",
		"CarPlay",
		"`Models.swift`",
		"client.summary()",
		"client.board()",
		"client.gates()",
		"client.chat(",
		"client.openclaw(",
		"client.decideGate(",
		"events(since:)",
		"GET /api/events",
		"setting `baseURL`",
		"Remote macOS connections",
	} {
		if strings.Contains(readme, stale) {
			t.Errorf("FortKit README still advertises deferred contract %q", stale)
		}
	}
}
