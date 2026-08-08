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
