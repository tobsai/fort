package main

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// TestPlistCarriesPATH pins the agent-discovery contract: launchd gives a bare
// PATH (/usr/bin:/bin:/usr/sbin:/sbin), so without an explicit PATH the daemon
// probes ZERO agent CLIs — brew binaries and ~/.local/bin ones are both invisible.
// The installing shell's PATH is baked in so the daemon can run whatever the
// operator can run.
func TestPlistCarriesPATH(t *testing.T) {
	sc := serviceConfig{
		Label:   "io.tobsai.fort",
		BinPath: "/opt/homebrew/bin/fort",
		Args:    []string{"serve"},
		Path:    "/Users/x/.local/bin:/opt/homebrew/bin:/usr/bin:/bin",
	}
	got := renderPlist(sc)
	if !strings.Contains(got, "<key>PATH</key>") {
		t.Fatalf("plist has no PATH key — the daemon would find no agent CLIs:\n%s", got)
	}
	if !strings.Contains(got, "<string>/Users/x/.local/bin:/opt/homebrew/bin:/usr/bin:/bin</string>") {
		t.Errorf("plist PATH value not carried through:\n%s", got)
	}
	// An empty Path must omit the key entirely rather than emit an empty string
	// (an empty PATH is worse than none — it breaks exec lookup outright).
	if strings.Contains(renderPlist(serviceConfig{Label: "l", BinPath: "/b"}), "<key>PATH</key>") {
		t.Error("empty Path should omit the PATH key")
	}
}

func TestPlistCarriesCapabilityPlanningRollback(t *testing.T) {
	sc := serviceConfig{
		Label: "io.tobsai.fort", BinPath: "/opt/homebrew/bin/fort",
		Args: []string{"serve"}, CapabilityPlanning: "0",
	}
	got := renderPlist(sc)
	for _, want := range []string{
		"<key>FORT_CAPABILITY_PLANNING</key>", "<string>0</string>",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("plist missing %q\n%s", want, got)
		}
	}
	if strings.Contains(renderPlist(serviceConfig{Label: "l", BinPath: "/b"}), "<key>FORT_CAPABILITY_PLANNING</key>") {
		t.Error("unset capability planning flag should use the binary default")
	}
}

func TestPlistCarriesConfiguredDisplayTimezone(t *testing.T) {
	sc := serviceConfig{
		Label: "io.tobsai.fort", BinPath: "/opt/homebrew/bin/fort",
		Args: []string{"serve"}, DisplayTimezone: "America/Chicago",
	}
	got := renderPlist(sc)
	for _, want := range []string{
		"<key>FORT_DISPLAY_TIMEZONE</key>", "<string>America/Chicago</string>",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("plist missing %q\n%s", want, got)
		}
	}
	if strings.Contains(renderPlist(serviceConfig{Label: "l", BinPath: "/b"}), "<key>FORT_DISPLAY_TIMEZONE</key>") {
		t.Error("unset display timezone should resolve from the daemon host")
	}
}

func TestPlistCarriesExplicitPrimaryPromotionConfiguration(t *testing.T) {
	sc := serviceConfig{
		Label: "io.tobsai.fort", BinPath: "/Applications/FortMac.app/Contents/Resources/fort",
		Args: []string{"serve"}, PrimaryChannels: "primary",
		AcceptedScheduleInventory: "schedule-inventory:v1:reviewed-on-this-machine",
	}
	got := renderPlist(sc)
	for _, want := range []string{
		"<key>FORT_PRIMARY_CHANNELS</key>\n    <string>primary</string>",
		"<key>FORT_ACCEPTED_SCHEDULE_INVENTORY</key>\n    <string>schedule-inventory:v1:reviewed-on-this-machine</string>",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("plist missing %q\n%s", want, got)
		}
	}

	closed := renderPlist(serviceConfig{Label: "l", BinPath: "/new/fort"})
	for _, key := range []string{"FORT_PRIMARY_CHANNELS", "FORT_ACCEPTED_SCHEDULE_INVENTORY"} {
		if strings.Contains(closed, "<key>"+key+"</key>") {
			t.Errorf("unset %s must remain omitted so startup stays off/fail-closed:\n%s", key, closed)
		}
	}
}

func TestServicePlistNeverPersistsNodeToken(t *testing.T) {
	t.Setenv("FORT_NODE_TOKEN", "do-not-write-this-secret-to-a-plist")
	sc, err := buildServiceConfig()
	if err != nil {
		t.Fatal(err)
	}
	definition := renderPlist(sc)
	if strings.Contains(definition, "FORT_NODE_TOKEN") || strings.Contains(definition, "do-not-write-this-secret-to-a-plist") {
		t.Fatalf("service plist persisted the mesh bearer token instead of relying on 0600 node.yaml:\n%s", definition)
	}
}

func TestBuildServiceConfigCarriesOnlyExplicitPrimaryPromotionConfiguration(t *testing.T) {
	t.Setenv("FORT_PRIMARY_CHANNELS", "preview")
	t.Setenv("FORT_ACCEPTED_SCHEDULE_INVENTORY", "schedule-inventory:v1:explicit-review")
	sc, err := buildServiceConfig()
	if err != nil {
		t.Fatal(err)
	}
	if sc.PrimaryChannels != "preview" {
		t.Fatalf("primary channels = %q, want preview", sc.PrimaryChannels)
	}
	if sc.AcceptedScheduleInventory != "schedule-inventory:v1:explicit-review" {
		t.Fatalf("accepted schedule inventory = %q", sc.AcceptedScheduleInventory)
	}
}

func TestServiceDefinitionRolloutPreservesPromotionAndUpdatesBinary(t *testing.T) {
	preparers := map[string]func(string, serviceConfig) error{
		"install": prepareServiceInstall,
		"restart": prepareServiceRestart,
	}
	for name, prepare := range preparers {
		t.Run(name, func(t *testing.T) {
			home := t.TempDir()
			label := "io.tobsai.fort.test"
			accepted := "schedule-inventory:v1:operator-reviewed-value-" + name
			if err := writePlist(home, serviceConfig{
				Label: label, BinPath: "/old/fort", Args: []string{"serve"},
				PrimaryChannels: "primary", AcceptedScheduleInventory: accepted,
			}); err != nil {
				t.Fatal(err)
			}

			if err := prepare(home, serviceConfig{
				Label: label, BinPath: "/Applications/FortMac.app/Contents/Resources/fort", Args: []string{"serve"},
			}); err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile(plistPath(home, label))
			if err != nil {
				t.Fatal(err)
			}
			definition := string(raw)
			for _, want := range []string{
				"<string>/Applications/FortMac.app/Contents/Resources/fort</string>",
				"<key>FORT_PRIMARY_CHANNELS</key>\n    <string>primary</string>",
				"<key>FORT_ACCEPTED_SCHEDULE_INVENTORY</key>\n    <string>" + accepted + "</string>",
			} {
				if !strings.Contains(definition, want) {
					t.Errorf("rolled-out plist missing %q\n%s", want, definition)
				}
			}
			if strings.Contains(definition, "<string>/old/fort</string>") {
				t.Fatalf("rollout retained the old binary path:\n%s", definition)
			}
		})
	}
}

func TestAppDrivenServiceRolloutPreservesClosedOperationalEnvironment(t *testing.T) {
	home := t.TempDir()
	label := "io.tobsai.fort.test"
	existing := serviceConfig{
		Label: label, BinPath: "/old/fort", Args: []string{"serve"},
		Addr: "0.0.0.0:4087", DBPath: filepath.Join(home, ".fort", ".fort-native", "fort.db"),
		WorkRoot:           filepath.Join(home, ".fort", ".fort-native", "work"),
		Path:               "/Users/test/.local/bin:/opt/homebrew/bin:/usr/bin:/bin",
		FlowsPath:          "/Users/test/.local/share/fort/v0.13.0/flows",
		CapabilityPlanning: "1", DisplayTimezone: "America/Chicago",
		PrimaryChannels: "preview",
	}
	if err := writePlist(home, existing); err != nil {
		t.Fatal(err)
	}
	// Unknown values in an existing plist are never carried into the refreshed
	// definition; preservation is a closed Fort-owned contract.
	path := plistPath(home, label)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	raw = []byte(strings.Replace(
		string(raw), "  </dict>\n  <key>WorkingDirectory</key>",
		"    <key>UNRECOGNIZED_SECRET</key>\n    <string>do-not-copy</string>\n  </dict>\n  <key>WorkingDirectory</key>", 1,
	))
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}

	// This models FortMac launching its bundled binary: defaults are populated,
	// but no FORT_* override was explicitly supplied by the app process.
	next := serviceConfig{
		Label: label, BinPath: "/Applications/FortMac.app/Contents/Resources/fort", Args: []string{"serve"},
		Addr: "127.0.0.1:4087", DBPath: filepath.Join(home, ".fort-native", "fort.db"),
		WorkRoot: filepath.Join(home, ".fort-native", "work"), Path: "/usr/bin:/bin",
		explicitEnvironment: map[string]bool{},
	}
	if err := prepareServiceRestart(home, next); err != nil {
		t.Fatal(err)
	}
	refreshed, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	definition := string(refreshed)
	for _, want := range []string{
		"<string>/Applications/FortMac.app/Contents/Resources/fort</string>",
		"<key>FORT_ADDR</key>\n    <string>0.0.0.0:4087</string>",
		"<key>FORT_DB</key>\n    <string>" + existing.DBPath + "</string>",
		"<key>FORT_WORKROOT</key>\n    <string>" + existing.WorkRoot + "</string>",
		"<key>FORT_FLOWS</key>\n    <string>" + existing.FlowsPath + "</string>",
		"<key>PATH</key>\n    <string>" + existing.Path + "</string>",
		"<key>FORT_CAPABILITY_PLANNING</key>\n    <string>1</string>",
		"<key>FORT_DISPLAY_TIMEZONE</key>\n    <string>America/Chicago</string>",
		"<key>FORT_PRIMARY_CHANNELS</key>\n    <string>preview</string>",
	} {
		if !strings.Contains(definition, want) {
			t.Errorf("refreshed service missing %q\n%s", want, definition)
		}
	}
	if strings.Contains(definition, "UNRECOGNIZED_SECRET") || strings.Contains(definition, "do-not-copy") {
		t.Fatalf("refreshed service copied an unknown environment key:\n%s", definition)
	}
}

func TestExplicitOperationalEnvironmentWinsAndCanClearExistingValue(t *testing.T) {
	home := t.TempDir()
	label := "io.tobsai.fort.test"
	if err := writePlist(home, serviceConfig{
		Label: label, BinPath: "/old/fort", Args: []string{"serve"},
		Addr: "0.0.0.0:4087", DBPath: "/existing/fort.db", FlowsPath: "/existing/flows",
	}); err != nil {
		t.Fatal(err)
	}

	next := serviceConfig{
		Label: label, BinPath: "/new/fort", Args: []string{"serve"},
		Addr: "127.0.0.1:4187", FlowsPath: "",
		explicitEnvironment: map[string]bool{
			"FORT_ADDR":  true,
			"FORT_FLOWS": true,
		},
	}
	if err := prepareServiceRestart(home, next); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(plistPath(home, label))
	if err != nil {
		t.Fatal(err)
	}
	definition := string(raw)
	if !strings.Contains(definition, "<key>FORT_ADDR</key>\n    <string>127.0.0.1:4187</string>") {
		t.Fatalf("explicit address did not replace existing value:\n%s", definition)
	}
	if !strings.Contains(definition, "<key>FORT_DB</key>\n    <string>/existing/fort.db</string>") {
		t.Fatalf("non-explicit database was not preserved:\n%s", definition)
	}
	if strings.Contains(definition, "FORT_FLOWS") || strings.Contains(definition, "/existing/flows") {
		t.Fatalf("explicit empty flow path did not clear existing value:\n%s", definition)
	}
}

func TestServiceRolloutRejectsInvalidPreservedTimezoneBeforeRewrite(t *testing.T) {
	home := t.TempDir()
	label := "io.tobsai.fort.test"
	existing := serviceConfig{
		Label: label, BinPath: "/old/fort", Args: []string{"serve"},
		DisplayTimezone: "Not/A_Zone",
	}
	if err := writePlist(home, existing); err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(plistPath(home, label))
	if err != nil {
		t.Fatal(err)
	}

	err = prepareServiceRestart(home, serviceConfig{
		Label: label, BinPath: "/new/fort", Args: []string{"serve"},
		explicitEnvironment: map[string]bool{},
	})
	if err == nil || !strings.Contains(err.Error(), `display timezone "Not/A_Zone"`) {
		t.Fatalf("invalid preserved timezone error = %v", err)
	}
	after, readErr := os.ReadFile(plistPath(home, label))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !reflect.DeepEqual(after, original) {
		t.Fatal("invalid preserved service definition was rewritten before validation")
	}
}

func TestServiceDefinitionRolloutDoesNotInventPromotionConfiguration(t *testing.T) {
	preparers := map[string]func(string, serviceConfig) error{
		"install": prepareServiceInstall,
		"restart": prepareServiceRestart,
	}
	for name, prepare := range preparers {
		t.Run(name, func(t *testing.T) {
			home := t.TempDir()
			label := "io.tobsai.fort.test"
			if err := prepare(home, serviceConfig{Label: label, BinPath: "/new/fort", Args: []string{"serve"}}); err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile(plistPath(home, label))
			if err != nil {
				t.Fatal(err)
			}
			for _, key := range []string{"FORT_PRIMARY_CHANNELS", "FORT_ACCEPTED_SCHEDULE_INVENTORY"} {
				if strings.Contains(string(raw), "<key>"+key+"</key>") {
					t.Fatalf("%s invented %s instead of remaining off/fail-closed:\n%s", name, key, raw)
				}
			}
		})
	}
}

func TestExplicitPrimaryPromotionConfigurationWinsDuringRollout(t *testing.T) {
	home := t.TempDir()
	label := "io.tobsai.fort.test"
	if err := writePlist(home, serviceConfig{
		Label: label, BinPath: "/old/fort", Args: []string{"serve"},
		PrimaryChannels: "primary", AcceptedScheduleInventory: "schedule-inventory:v1:old-review",
	}); err != nil {
		t.Fatal(err)
	}
	if err := prepareServiceRestart(home, serviceConfig{
		Label: label, BinPath: "/new/fort", Args: []string{"serve"},
		PrimaryChannels: "off", AcceptedScheduleInventory: "schedule-inventory:v1:new-review",
	}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(plistPath(home, label))
	if err != nil {
		t.Fatal(err)
	}
	definition := string(raw)
	if !strings.Contains(definition, "<key>FORT_PRIMARY_CHANNELS</key>\n    <string>off</string>") {
		t.Fatalf("explicit rollback mode did not win:\n%s", definition)
	}
	if !strings.Contains(definition, "<key>FORT_ACCEPTED_SCHEDULE_INVENTORY</key>\n    <string>schedule-inventory:v1:new-review</string>") {
		t.Fatalf("explicit accepted inventory did not win:\n%s", definition)
	}
}

func TestServiceInstallRejectsInvalidDisplayTimezoneBeforeLaunchd(t *testing.T) {
	err := validateServiceDisplayTimezone(serviceConfig{DisplayTimezone: "Not/A_Zone"})
	if err == nil {
		t.Fatal("invalid display timezone was accepted for service installation")
	}
	if !strings.Contains(err.Error(), `display timezone "Not/A_Zone"`) {
		t.Fatalf("error = %v", err)
	}
}

// TestPlistIsSelfContained pins the boot contract. launchd starts a user agent
// with cwd "/" (read-only), so any RELATIVE path in the daemon's config — the
// default DB (.fort-native/fort.db) and work root (.fort-native/work) — makes it
// die with "mkdir .fort-native: read-only file system". The plist must therefore
// set a writable WorkingDirectory AND absolute FORT_DB / FORT_WORKROOT.
func TestPlistIsSelfContained(t *testing.T) {
	sc := serviceConfig{
		Label: "io.tobsai.fort", BinPath: "/opt/homebrew/bin/fort", Args: []string{"serve"},
		WorkDir: "/Users/x", DBPath: "/Users/x/.fort-native/fort.db", WorkRoot: "/Users/x/.fort-native/work",
	}
	got := renderPlist(sc)
	for _, want := range []string{
		"<key>WorkingDirectory</key>", "<string>/Users/x</string>",
		"<key>FORT_DB</key>", "<string>/Users/x/.fort-native/fort.db</string>",
		"<key>FORT_WORKROOT</key>", "<string>/Users/x/.fort-native/work</string>",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("plist missing %q\n%s", want, got)
		}
	}
}

// TestAbsUnderHome: relative config paths are anchored to a writable home, never
// left relative (they would resolve against launchd's read-only "/").
func TestAbsUnderHome(t *testing.T) {
	if got := absUnderHome("/Users/x", ".fort-native/fort.db"); got != "/Users/x/.fort-native/fort.db" {
		t.Errorf("relative: %q", got)
	}
	if got := absUnderHome("/Users/x", "/tmp/explicit.db"); got != "/tmp/explicit.db" {
		t.Errorf("absolute must pass through unchanged: %q", got)
	}
	if got := absUnderHome("/Users/x", ""); got != "" {
		t.Errorf("empty stays empty: %q", got)
	}
}

func TestPlistContentsAndPath(t *testing.T) {
	sc := serviceConfig{
		Label:   "io.tobsai.fort",
		BinPath: "/opt/homebrew/bin/fort",
		Args:    []string{"serve"},
		Addr:    "127.0.0.1:4087",
		DBPath:  "/Users/x/.fort-native/fort.db",
		LogDir:  "/Users/x/Library/Logs/Fort",
	}
	got := renderPlist(sc)
	for _, want := range []string{
		"<key>Label</key>", "<string>io.tobsai.fort</string>",
		"<string>/opt/homebrew/bin/fort</string>", "<string>serve</string>",
		"<key>FORT_ADDR</key>", "<string>127.0.0.1:4087</string>",
		"<key>RunAtLoad</key>", "<key>KeepAlive</key>",
		"<key>StandardOutPath</key>",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("plist missing %q\n%s", want, got)
		}
	}
	if p := plistPath("/Users/x", sc.Label); p != "/Users/x/Library/LaunchAgents/io.tobsai.fort.plist" {
		t.Errorf("plistPath = %q", p)
	}
}

func TestInstallWritesPlistUninstallRemoves(t *testing.T) {
	home := t.TempDir()
	sc := serviceConfig{Label: "io.tobsai.fort.test", BinPath: "/bin/echo", Args: []string{"serve"}}
	if err := writePlist(home, sc); err != nil {
		t.Fatal(err)
	}
	p := plistPath(home, sc.Label)
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("plist not written: %v", err)
	}
	// idempotent rewrite
	if err := writePlist(home, sc); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if err := removePlist(home, sc.Label); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatalf("plist not removed")
	}
	// remove is idempotent (no error when already gone)
	if err := removePlist(home, sc.Label); err != nil {
		t.Fatalf("second remove: %v", err)
	}
	_ = filepath.Join // keep import if unused after edits
}

func TestPrepareServiceRestartRewritesRollbackFlag(t *testing.T) {
	home := t.TempDir()
	sc := serviceConfig{
		Label: "io.tobsai.fort.test", BinPath: "/bin/fort", Args: []string{"serve"},
		CapabilityPlanning: "0",
	}
	if err := prepareServiceRestart(home, sc); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(plistPath(home, sc.Label))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "<key>FORT_CAPABILITY_PLANNING</key>\n    <string>0</string>") {
		t.Fatalf("restart plist did not carry rollback flag:\n%s", raw)
	}
}

func TestServiceRestartReloadsLaunchdDefinition(t *testing.T) {
	sc := serviceConfig{Label: "io.tobsai.fort"}
	got := serviceRestartCommands("/Users/x", sc)
	want := [][]string{
		{"launchctl", "bootout", guiLabelTarget(sc.Label)},
		{"launchctl", "bootstrap", guiTarget(), "/Users/x/Library/LaunchAgents/io.tobsai.fort.plist"},
		{"launchctl", "kickstart", guiLabelTarget(sc.Label)},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("restart commands = %#v, want %#v", got, want)
	}
}

func TestRunServiceRestartRetriesTransientBootstrapError(t *testing.T) {
	sc := serviceConfig{Label: "io.tobsai.fort"}
	bootstrapAttempts := 0
	var delays []time.Duration
	run := func(command []string) ([]byte, error) {
		if len(command) > 1 && command[1] == "bootstrap" {
			bootstrapAttempts++
			if bootstrapAttempts == 1 {
				return []byte("Bootstrap failed: 5: Input/output error"), errors.New("exit status 5")
			}
		}
		return nil, nil
	}

	err := runServiceRestart("/Users/x", sc, run, func(delay time.Duration) {
		delays = append(delays, delay)
	})
	if err != nil {
		t.Fatalf("restart returned transient bootstrap failure: %v", err)
	}
	if bootstrapAttempts != 2 {
		t.Fatalf("bootstrap attempts = %d, want 2", bootstrapAttempts)
	}
	if !reflect.DeepEqual(delays, []time.Duration{serviceRestartRetryDelay}) {
		t.Fatalf("retry delays = %v, want [%v]", delays, serviceRestartRetryDelay)
	}
}
