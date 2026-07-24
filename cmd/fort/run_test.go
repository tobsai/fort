package main

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/tobsai/fort/core/engine"
	"github.com/tobsai/fort/core/router"
	"github.com/tobsai/fort/core/rules"
	"github.com/tobsai/fort/core/store"
	"github.com/tobsai/fort/core/task"
	"github.com/tobsai/fort/exec/fake"
)

func TestStreamRunCancelableCancelsEngineRun(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "fort.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	rs, err := rules.Parse(defaultRulesYAML)
	if err != nil {
		t.Fatal(err)
	}
	rt := fake.New()
	rt.Block = true
	eng := engine.New(router.New(rs), rt, st, t.TempDir())
	runID, err := eng.Submit(context.Background(), task.Task{
		ID: "interrupt", Title: "interrupt", Agent: "claude",
	})
	if err != nil {
		t.Fatal(err)
	}
	a := &app{store: st, engine: eng}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := streamRunCancelable(ctx, a, runID); err != nil {
		t.Fatal(err)
	}
	run, err := st.GetRun(runID)
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != "canceled" {
		t.Fatalf("run status = %q, want canceled", run.Status)
	}
}
