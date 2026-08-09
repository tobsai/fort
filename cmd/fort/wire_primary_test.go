package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	coreruntime "github.com/tobsai/fort/core/runtime"
)

func TestBuildAppProtectsNodeAndLocalExecutionWithClosedRuntimeMux(t *testing.T) {
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("FORT_FAKE", "1")
	t.Setenv("FORT_CAPABILITY_PLANNING", "0")
	t.Setenv("FORT_DB", filepath.Join(t.TempDir(), "fort.db"))
	t.Setenv("FORT_WORKROOT", filepath.Join(t.TempDir(), "work"))
	t.Setenv("FORT_RULES", filepath.Join(root, "..", "..", "rules", "v1.yaml"))
	t.Setenv("FORT_MACHINES", "")

	a, err := buildApp()
	if err != nil {
		t.Fatal(err)
	}
	defer a.store.Close()
	if got := a.localRT.Name(); got != "local-runtime-mux" {
		t.Fatalf("local runtime = %q, want closed mux", got)
	}
	run, err := a.localRT.Dispatch(context.Background(), coreruntime.RunSpec{RunID: "legacy", Agent: "codex"})
	if err != nil {
		t.Fatalf("legacy dispatch: %v", err)
	}
	_ = run.Wait()
	if _, err := a.localRT.Dispatch(context.Background(), coreruntime.RunSpec{
		RunID: "cross-route", Agent: "codex-subscription", Profile: "codex-subscription:gpt-5.6-sol",
	}); err == nil {
		t.Fatal("subscription identity crossed into the legacy runtime")
	}
}
