package apple_test

import (
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestPrimaryChannelsAreTheShippingAppleRoot(t *testing.T) {
	shared := readAppleSource(t, "FortKit/Sources/FortKit/PrimaryChannelsView.swift")
	mac := readAppleSource(t, "macOS/FortMacApp.swift")
	phone := readAppleSource(t, "iOS/FortApp.swift")

	for _, required := range []string{
		"public struct PrimaryChannelsView",
		"PrimaryChannelTranscript",
		"PrimaryScheduleList",
		"PrimaryNeedsYouList",
		"PrimaryAgentSettings",
		"fort.primary.theme.v1",
		"quiet-intelligence",
		"private-channels",
		"native-daylight",
		"Text-only chat",
		"FortAgentOrbView",
	} {
		if !strings.Contains(shared, required) {
			t.Fatalf("shared Primary Channels UI missing %q", required)
		}
	}
	if strings.Contains(shared, "/api/chat") {
		t.Fatal("Primary Channels UI must never use the legacy chat endpoint")
	}
	if !strings.Contains(mac, "PrimaryChannelsView()") || strings.Contains(mac, "FortWindow()") {
		t.Fatal("macOS shipping root must be Primary Channels, not the legacy Command Deck")
	}
	if !strings.Contains(phone, "PrimaryChannelsView()") || strings.Contains(phone, "NavigationStack { BoardView() }") {
		t.Fatal("iPhone shipping root must be Primary Channels, not the legacy Command Deck")
	}
}

func TestPrimarySchedulesAndModelIdentityUseClosedPresentation(t *testing.T) {
	shared := readAppleSource(t, "FortKit/Sources/FortKit/PrimaryChannelsView.swift")

	for _, required := range []string{
		"PrimarySchedulePresentation.occurrenceLabel",
		"PrimarySchedulePresentation.definitionLabel",
		`case "fired": return "Fired"`,
		`case "running": return "Running"`,
		"requestedModel: option.authority.requestedModel",
		"resolvedModel: option.authority.resolvedModel",
		`identityRow("Requested model"`,
		`identityRow("Resolved model"`,
	} {
		if !strings.Contains(shared, required) {
			t.Errorf("shared Primary presentation missing %q", required)
		}
	}
	if got := strings.Count(shared, "timeZoneID: item.timezone"); got < 3 {
		t.Errorf("schedule-list timestamps using configured timezone = %d, want at least 3", got)
	}
	if got := strings.Count(shared, "timeZoneID: displayedItem.timezone"); got < 3 {
		t.Errorf("schedule-detail timestamps using authoritative timezone = %d, want at least 3", got)
	}
	for _, forbidden := range []string{
		"item.latestOccurrence?.state.capitalized",
		"occurrence.state.capitalized",
	} {
		if strings.Contains(shared, forbidden) {
			t.Errorf("shared Primary presentation leaks raw wire label %q", forbidden)
		}
	}
}

func TestPrimaryPendingTurnsPersistAndSettingsGroupByComputer(t *testing.T) {
	shared := readAppleSource(t, "FortKit/Sources/FortKit/PrimaryChannelsView.swift")

	for _, required := range []string{
		`"fort.primary.pending-turns.v1"`,
		"pendingTurnStore.save(pendingTurns)",
		"pendingTurnStore.reconciled(pendingTurns, with: detail)",
		"detail.turns.contains(where: { $0.clientTurnID == pending.clientTurnID })",
		"PrimaryAgentOptionGrouping.groups(for:",
		`Section("Computer · \(group.machine)")`,
	} {
		if !strings.Contains(shared, required) {
			t.Errorf("shared Primary persistence/grouping missing %q", required)
		}
	}
	if strings.Contains(shared, "ForEach(agent?.options ?? [])") {
		t.Error("Primary Agent Settings still renders one ungrouped option list")
	}
}

func TestPrimarySendScheduleEvidenceAndTransportTasksStayTruthful(t *testing.T) {
	shared := readAppleSource(t, "FortKit/Sources/FortKit/PrimaryChannelsView.swift")
	client := readAppleSource(t, "FortKit/Sources/FortKit/FortClient.swift")

	for _, required := range []string{
		"PrimarySendOutcomeReducer.failure(for: error)",
		"PrimarySendOutcomeReducer.pendingTurn(for: outcome, submission: submission)",
		"return outcome == .accepted",
		"detail?.item ?? listItem",
		"PrimarySchedulePresentation.occurrenceAction(for:",
		`case .viewSchedule: return "View schedule"`,
		`case .openRun: return "Open run"`,
		`case .viewResult: return "View result"`,
		`case .reviewFailure: return "Review failure"`,
		"client.transportGeneration",
	} {
		if !strings.Contains(shared+client, required) {
			t.Errorf("Primary send/schedule/transport source missing %q", required)
		}
	}
	if strings.Contains(shared, "return pendingTurns[channelID] == nil") {
		t.Error("Primary send still mistakes cleared pending state for accepted server outcome")
	}
	if strings.Contains(shared, "client.runDetail(") {
		t.Error("Primary schedule UI crossed into the legacy run-detail API")
	}
	if got := strings.Count(shared, "client.transportGeneration"); got < 2 {
		t.Errorf("Primary task identities using transport generation = %d, want both reload and SSE", got)
	}
}

func TestNativeGatewayRequiresHTTPSAndPublishesTransportReplacement(t *testing.T) {
	address := readAppleSource(t, "FortKit/Sources/FortKit/GatewayAddress.swift")
	client := readAppleSource(t, "FortKit/Sources/FortKit/FortClient.swift")
	for _, required := range []string{
		`guard scheme == "https" else`,
		"@Published public private(set) var transportGeneration",
		"transportGeneration &+= 1",
	} {
		if !strings.Contains(address+client, required) {
			t.Errorf("native gateway boundary missing %q", required)
		}
	}
	if strings.Contains(address, `scheme == "https" || scheme == "http"`) {
		t.Error("native gateway still permits cleartext bearer transport")
	}
}

func TestPrimaryAppleReleaseBumpsEveryShippingBuild(t *testing.T) {
	project := readAppleSource(t, "project.yml")
	const buildMarker = `CURRENT_PROJECT_VERSION: "`
	start := strings.Index(project, buildMarker)
	if start == -1 {
		t.Fatal("project version marker missing")
	}
	start += len(buildMarker)
	end := strings.Index(project[start:], `"`)
	if end == -1 {
		t.Fatal("project version terminator missing")
	}
	build, err := strconv.Atoi(project[start : start+end])
	if err != nil || build <= 2608091 {
		t.Fatalf("Apple release build = %q, must be newer than legacy-bearing uploaded 2608091", project[start:start+end])
	}
	for _, target := range []string{"Fort:", "FortMac:"} {
		if !strings.Contains(project, target) {
			t.Fatalf("shipping release target %q disappeared", target)
		}
	}
}

func readAppleSource(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
