package apple_test

import (
	"strings"
	"testing"
)

const debugSimulatorGuard = "#if DEBUG && targetEnvironment(simulator)"
const desktopOrDebugSimulatorGuard = "#if os(macOS) || (DEBUG && targetEnvironment(simulator))"

func TestIPhonePhysicalReleaseCompilesOutDirectHost(t *testing.T) {
	app := readAppleSource(t, "iOS/FortApp.swift")
	coordinator := readAppleSource(t, "iOS/GatewayCoordinator.swift")
	client := readAppleSource(t, "FortKit/Sources/FortKit/FortClient.swift")

	for _, required := range []string{
		"FortClient.gatewayOnly()",
		"#else\n        return FortClient.gatewayOnly()",
		"gateway.hasPrimaryTransport",
		"client.disconnectGateway()",
		debugSimulatorGuard,
	} {
		if !strings.Contains(app+coordinator, required) {
			t.Fatalf("iPhone release transport boundary missing %q", required)
		}
	}
	directHostAPI := swiftGuardContents(t, client, desktopOrDebugSimulatorGuard)
	if !strings.Contains(directHostAPI, "public func useDirectHost(_ url: URL)") {
		t.Fatal("FortClient direct-host action must compile only for macOS or DEBUG Simulator")
	}

	releaseSource := stripSwiftGuard(t, app, debugSimulatorGuard) + stripSwiftGuard(t, coordinator, debugSimulatorGuard)
	for _, forbidden := range []string{
		"FORT_DIRECT_HOST_URL",
		"directHostEnabled",
		"useDirectHost(",
		`Section("Control-plane host")`,
		`Button("Use direct host")`,
		"urlText",
		"127.0.0.1:4087",
	} {
		if strings.Contains(releaseSource, forbidden) {
			t.Fatalf("physical iPhone Release still contains direct-host surface %q", forbidden)
		}
	}
}

func TestIPhoneDebugSimulatorRetainsIsolatedFixtureHost(t *testing.T) {
	app := readAppleSource(t, "iOS/FortApp.swift")
	coordinator := readAppleSource(t, "iOS/GatewayCoordinator.swift")
	guarded := swiftGuardContents(t, app, debugSimulatorGuard) + swiftGuardContents(t, coordinator, debugSimulatorGuard)

	for _, required := range []string{
		`ProcessInfo.processInfo.environment["FORT_DIRECT_HOST_URL"]`,
		`"http://127.0.0.1:4087"`,
		"client.useDirectHost(",
		`Section("Control-plane host")`,
		`Button("Use direct host")`,
	} {
		if !strings.Contains(guarded, required) {
			t.Fatalf("DEBUG iOS Simulator QA path missing %q", required)
		}
	}
}

func TestIPhoneReleaseDocsRequireAuthenticatedRelay(t *testing.T) {
	readme := readAppleSource(t, "README.md")
	testflight := readAppleSource(t, "../../docs/notes/testflight.md")
	combined := readme + testflight

	for _, required := range []string{
		"authenticated encrypted gateway relay",
		"DEBUG iOS Simulator",
		"never available on a physical iPhone",
	} {
		if !strings.Contains(combined, required) {
			t.Fatalf("iPhone transport documentation missing %q", required)
		}
	}
	for _, stale := range []string{
		"Direct LAN/simulator mode is an explicit fallback",
		"explicit direct-host development fallback may point at a simulator or LAN host",
	} {
		if strings.Contains(combined, stale) {
			t.Fatalf("iPhone transport documentation still permits physical LAN fallback %q", stale)
		}
	}
}

func TestGatewayIdentityReplacementFailsClosedBeforeMachineRefresh(t *testing.T) {
	coordinator := readAppleSource(t, "iOS/GatewayCoordinator.swift")
	for _, required := range []string{
		"func refreshMachines(client: FortClient) async",
		"disconnectPrimaryTransport(client: client)",
		"await self.refreshMachines(client: client)",
		"connectedMachineID = nil",
		"client.disconnectGateway()",
	} {
		if !strings.Contains(coordinator, required) {
			t.Errorf("gateway fail-closed replacement missing %q", required)
		}
	}

	disconnect := strings.Index(coordinator, "self.disconnectPrimaryTransport(client: client)")
	refresh := strings.Index(coordinator, "await self.refreshMachines(client: client)")
	if disconnect == -1 || refresh == -1 || disconnect > refresh {
		t.Error("gateway re-auth refreshes machines before disconnecting the old relay")
	}

	unauthorized := strings.Index(coordinator, "where error.statusCode == 401")
	if unauthorized == -1 {
		t.Fatal("gateway refresh has no explicit 401 branch")
	}
	branch := coordinator[unauthorized:]
	if end := strings.Index(branch, "} catch {"); end >= 0 {
		branch = branch[:end]
	}
	if !strings.Contains(branch, "disconnectPrimaryTransport(client: client)") {
		t.Error("gateway 401 leaves the old authenticated relay connected")
	}

	refreshStart := strings.Index(coordinator, "func refreshMachines(client: FortClient) async")
	fetch := strings.Index(coordinator[refreshStart:], "GatewayService.machines(")
	prefix := coordinator[refreshStart : refreshStart+fetch]
	if !strings.Contains(prefix, "disconnectPrimaryTransport(client: client)") {
		t.Error("machine refresh starts before the old relay is disconnected")
	}

	genericCatch := strings.Index(coordinator[unauthorized:], "} catch {")
	connectStart := strings.Index(coordinator, "func connect(_ machine: GatewayMachine")
	if genericCatch == -1 || connectStart == -1 {
		t.Fatal("gateway refresh/connect source shape disappeared")
	}
	generic := coordinator[unauthorized+genericCatch : connectStart]
	for _, required := range []string{"machines = []", "disconnectPrimaryTransport(client: client)"} {
		if !strings.Contains(generic, required) {
			t.Errorf("gateway refresh failure does not fail closed: missing %q", required)
		}
	}

	connect := coordinator[connectStart:]
	disconnect = strings.Index(connect, "disconnectPrimaryTransport(client: client)")
	useGateway := strings.Index(connect, "client.useGateway(")
	if disconnect == -1 || useGateway == -1 || disconnect > useGateway {
		t.Error("same-origin machine switch installs B before disconnecting A")
	}
	if !strings.Contains(coordinator, "if machines.count == 1") {
		t.Error("gateway re-auth can auto-connect zero or multiple machine choices")
	}
}

func TestIPhonePersistsSessionSecurelyAndEntersChatAfterConnection(t *testing.T) {
	app := readAppleSource(t, "iOS/FortApp.swift")
	coordinator := readAppleSource(t, "iOS/GatewayCoordinator.swift")
	credentials := readAppleSource(t, "iOS/GatewaySessionTokenStore.swift")
	channels := readAppleSource(t, "FortKit/Sources/FortKit/PrimaryChannelsView.swift")

	for _, required := range []string{
		"kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly",
		"SecItemCopyMatching",
		"SecItemUpdate",
		"SecItemAdd",
		"SecItemDelete",
		"GatewaySessionTokenStore.load()",
		"GatewaySessionTokenStore.save(account.bearerToken)",
	} {
		if !strings.Contains(credentials+coordinator, required) {
			t.Errorf("durable native session missing %q", required)
		}
	}
	if strings.Contains(coordinator, "account.bearerToken = nil\n            account.selectedMachineID = nil") {
		t.Error("expired credential still discards the trusted machine selection")
	}
	for _, required := range []string{
		".onChange(of: gateway.connectedMachineID)",
		"dismiss()",
		"destination = .channel(selectedID)",
		"await loadChannel(id: selectedID, client: client)",
	} {
		if !strings.Contains(app+channels, required) {
			t.Errorf("direct-to-chat entry missing %q", required)
		}
	}
}

func stripSwiftGuard(t *testing.T, source, guard string) string {
	t.Helper()
	lines := strings.Split(source, "\n")
	var kept []string
	depth := 0
	seen := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if depth == 0 && trimmed == guard {
			depth = 1
			seen = true
			continue
		}
		if depth > 0 {
			if strings.HasPrefix(trimmed, "#if ") {
				depth++
			} else if trimmed == "#endif" {
				depth--
			}
			continue
		}
		kept = append(kept, line)
	}
	if !seen || depth != 0 {
		t.Fatalf("missing or unbalanced Swift guard %q", guard)
	}
	return strings.Join(kept, "\n")
}

func swiftGuardContents(t *testing.T, source, guard string) string {
	t.Helper()
	lines := strings.Split(source, "\n")
	var guarded []string
	depth := 0
	seen := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if depth == 0 && trimmed == guard {
			depth = 1
			seen = true
			continue
		}
		if depth > 0 {
			if strings.HasPrefix(trimmed, "#if ") {
				depth++
			} else if trimmed == "#endif" {
				depth--
				if depth == 0 {
					continue
				}
			}
			guarded = append(guarded, line)
		}
	}
	if !seen || depth != 0 {
		t.Fatalf("missing or unbalanced Swift guard %q", guard)
	}
	return strings.Join(guarded, "\n")
}
