package main

import (
	"path/filepath"
	"testing"
)

func TestBuildAppDoesNotResolvePresentationTimezoneForUnrelatedCommands(t *testing.T) {
	root := t.TempDir()
	t.Setenv("FORT_FAKE", "1")
	t.Setenv("FORT_CAPABILITY_PLANNING", "0")
	t.Setenv("FORT_DISPLAY_TIMEZONE", "Not/A_Zone")
	t.Setenv("FORT_DB", filepath.Join(root, "fort.db"))
	t.Setenv("FORT_WORKROOT", filepath.Join(root, "work"))
	t.Setenv("FORT_MACHINES", "")

	a, err := buildApp()
	if err != nil {
		t.Fatalf("unrelated CLI composition resolved the presentation timezone: %v", err)
	}
	t.Cleanup(func() { a.store.Close() })
}
