package flow_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/tobsai/fort/core/flow"
	"github.com/tobsai/fort/core/graph"
	"github.com/tobsai/fort/core/store"
	"github.com/tobsai/fort/exec/fake"
)

func exec(t *testing.T) (*graph.Executor, *store.Store) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "fort.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return graph.NewExecutor(fake.New(), st), st
}

// AO-026: the game-starter pipeline runs end-to-end as a deterministic DAG.
func TestGameStarterRunsAsDAG(t *testing.T) {
	f, err := flow.LoadFile("../../flows/game-starter.yaml")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	ex, st := exec(t)
	res, err := ex.Start(context.Background(), f, "gs1", "batch-001")
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if res.State != "completed" {
		t.Fatalf("state = %q, want completed", res.State)
	}
	for _, id := range []string{"batch_uniformity", "normalize", "catalog", "import_drop"} {
		found := false
		for _, n := range mustNodes(t, st, "gs1") {
			if n.NodeID == id && n.Status == "succeeded" {
				found = true
			}
		}
		if !found {
			t.Errorf("node %s did not succeed", id)
		}
	}
}

// AO-027 (exit gate): ship-feature runs unattended except at plan_gate and
// merge_gate.
func TestShipFeatureRunsUnattendedExceptGates(t *testing.T) {
	f, err := flow.LoadFile("../../flows/ship-feature.yaml")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	ex, _ := exec(t)
	ctx := context.Background()

	res, err := ex.Start(ctx, f, "sf1", "add dark mode")
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if res.State != "paused" || res.PausedNode != "plan_gate" {
		t.Fatalf("res = %+v, want paused at plan_gate", res)
	}

	if err := ex.Approve("sf1", "plan_gate", ""); err != nil {
		t.Fatalf("approve plan: %v", err)
	}
	res, err = ex.Resume(ctx, f, "sf1")
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	if res.State != "paused" || res.PausedNode != "merge_gate" {
		t.Fatalf("res = %+v, want paused at merge_gate", res)
	}

	if err := ex.Approve("sf1", "merge_gate", ""); err != nil {
		t.Fatalf("approve merge: %v", err)
	}
	res, err = ex.Resume(ctx, f, "sf1")
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	if res.State != "completed" {
		t.Errorf("final state = %q, want completed", res.State)
	}
}

func mustNodes(t *testing.T, st *store.Store, runID string) []store.NodeRun {
	t.Helper()
	ns, err := st.NodeRuns(runID)
	if err != nil {
		t.Fatal(err)
	}
	return ns
}
