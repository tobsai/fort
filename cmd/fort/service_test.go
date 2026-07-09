package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
