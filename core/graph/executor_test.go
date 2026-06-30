package graph

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/tobsai/fort/core/store"
	"github.com/tobsai/fort/exec/fake"
)

func newExec(t *testing.T) (*Executor, *store.Store, *fake.Runtime) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "fort.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	rt := fake.New()
	return NewExecutor(rt, st), st, rt
}

func nodeStatus(t *testing.T, st *store.Store, runID, nodeID string) (string, bool) {
	t.Helper()
	for _, n := range mustNodeRuns(t, st, runID) {
		if n.NodeID == nodeID {
			return n.Status, true
		}
	}
	return "", false
}

func mustNodeRuns(t *testing.T, st *store.Store, runID string) []store.NodeRun {
	t.Helper()
	ns, err := st.NodeRuns(runID)
	if err != nil {
		t.Fatalf("noderuns: %v", err)
	}
	return ns
}

func TestLinearFlowOnlyTaskInvokesRuntime(t *testing.T) {
	ex, st, rt := newExec(t)
	f := Flow{
		ID: "f1", Start: "t0",
		Nodes: []Node{
			{ID: "t0", Type: Transform, Transform: &TransformSpec{Op: "identity"}, Edges: []Edge{{On: OutAlways, To: "c1"}}},
			{ID: "c1", Type: Check, Check: &CheckSpec{Command: []string{"sh", "-c", "exit 0"}}, Edges: []Edge{{On: OutPass, To: "k1"}}},
			{ID: "k1", Type: Task, Agent: "codex", Prompt: "do work"},
		},
	}
	res, err := ex.Start(context.Background(), f, "run1", "payload")
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if res.State != "completed" {
		t.Fatalf("state = %q, want completed", res.State)
	}
	// Only the task node touched the runtime.
	if d := rt.Dispatched(); len(d) != 1 || d[0].Agent != "codex" {
		t.Errorf("dispatched = %+v, want exactly one codex task", d)
	}
	for _, id := range []string{"t0", "c1", "k1"} {
		if s, ok := nodeStatus(t, st, "run1", id); !ok || s != "succeeded" {
			t.Errorf("node %s status=%q ok=%v, want succeeded", id, s, ok)
		}
	}
}

func TestCheckFailTakesFailEdge(t *testing.T) {
	ex, st, _ := newExec(t)
	f := Flow{
		ID: "f2", Start: "c1",
		Nodes: []Node{
			{ID: "c1", Type: Check, Check: &CheckSpec{Command: []string{"sh", "-c", "exit 1"}},
				Edges: []Edge{{On: OutPass, To: "good"}, {On: OutFail, To: "bad"}}},
			{ID: "good", Type: Task, Agent: "codex"},
			{ID: "bad", Type: Task, Agent: "claude"},
		},
	}
	if _, err := ex.Start(context.Background(), f, "run2", ""); err != nil {
		t.Fatalf("start: %v", err)
	}
	if s, ok := nodeStatus(t, st, "run2", "bad"); !ok || s != "succeeded" {
		t.Errorf("bad branch not taken: status=%q ok=%v", s, ok)
	}
	if _, ok := nodeStatus(t, st, "run2", "good"); ok {
		t.Errorf("good branch should not have run")
	}
}

func TestTransformRecordsOutputAndHash(t *testing.T) {
	ex, st, _ := newExec(t)
	f := Flow{
		ID: "f3", Start: "t0",
		Nodes: []Node{{ID: "t0", Type: Transform, Transform: &TransformSpec{Op: "upper"}}},
	}
	if _, err := ex.Start(context.Background(), f, "run3", "abc"); err != nil {
		t.Fatalf("start: %v", err)
	}
	ns := mustNodeRuns(t, st, "run3")
	if len(ns) != 1 || ns[0].Input != "abc" || ns[0].Output != "ABC" {
		t.Fatalf("transform node = %+v, want in=abc out=ABC", ns)
	}
	// a content hash is recorded as an event
	evs, _ := st.Events("run3")
	foundHash := false
	for _, e := range evs {
		if e.Type == "transform" && len(e.Data) > 0 {
			foundHash = true
		}
	}
	if !foundHash {
		t.Errorf("expected a transform event recording a hash, got %+v", evs)
	}
}

func gateFlow() Flow {
	return Flow{
		ID: "g", Start: "g1",
		Nodes: []Node{
			{ID: "g1", Type: Gate, Edges: []Edge{{On: OutApprove, To: "k1"}, {On: OutReject, To: "k2"}}},
			{ID: "k1", Type: Task, Agent: "codex"}, // approved path
			{ID: "k2", Type: Task, Agent: "claude"}, // rejected path
		},
	}
}

func TestGatePausesAndApproveResumes(t *testing.T) {
	ex, st, _ := newExec(t)
	f := gateFlow()
	res, err := ex.Start(context.Background(), f, "run4", "")
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if res.State != "paused" || res.PausedNode != "g1" {
		t.Fatalf("res = %+v, want paused at g1", res)
	}
	if s, _ := nodeStatus(t, st, "run4", "g1"); s != "waiting" {
		t.Errorf("gate status=%q, want waiting", s)
	}
	if err := ex.Approve("run4", "g1", ""); err != nil {
		t.Fatalf("approve: %v", err)
	}
	res, err = ex.Resume(context.Background(), f, "run4")
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	if res.State != "completed" {
		t.Errorf("state = %q, want completed", res.State)
	}
	if s, _ := nodeStatus(t, st, "run4", "k1"); s != "succeeded" {
		t.Errorf("approved task k1 status=%q", s)
	}
	if _, ok := nodeStatus(t, st, "run4", "k2"); ok {
		t.Errorf("rejected task k2 should not have run")
	}
}

func TestGateRejectTakesRejectEdge(t *testing.T) {
	ex, st, _ := newExec(t)
	f := gateFlow()
	_, _ = ex.Start(context.Background(), f, "run5", "")
	if err := ex.Reject("run5", "g1"); err != nil {
		t.Fatalf("reject: %v", err)
	}
	if _, err := ex.Resume(context.Background(), f, "run5"); err != nil {
		t.Fatalf("resume: %v", err)
	}
	if s, _ := nodeStatus(t, st, "run5", "k2"); s != "succeeded" {
		t.Errorf("rejected path k2 status=%q, want succeeded", s)
	}
}

func TestGateEditMutatesDownstreamInput(t *testing.T) {
	ex, _, rt := newExec(t)
	f := Flow{
		ID: "ge", Start: "g1",
		Nodes: []Node{
			{ID: "g1", Type: Gate, Edges: []Edge{{On: OutApprove, To: "k1"}}},
			{ID: "k1", Type: Task, Agent: "codex"}, // empty prompt -> uses payload
		},
	}
	_, _ = ex.Start(context.Background(), f, "run6", "original")
	if err := ex.Approve("run6", "g1", "EDITED"); err != nil {
		t.Fatalf("approve: %v", err)
	}
	if _, err := ex.Resume(context.Background(), f, "run6"); err != nil {
		t.Fatalf("resume: %v", err)
	}
	d := rt.Dispatched()
	if len(d) != 1 || d[0].Prompt != "EDITED" {
		t.Errorf("dispatched prompt = %+v, want EDITED", d)
	}
}

func TestRetryThenEscalateToGate(t *testing.T) {
	ex, st, rt := newExec(t)
	rt.ExitCode = 1 // task always fails
	f := Flow{
		ID: "r", Start: "k1",
		Nodes: []Node{
			{ID: "k1", Type: Task, Agent: "codex", Retry: &Retry{Max: 2},
				Edges: []Edge{{On: OutSuccess, To: "done"}, {On: OutFail, To: "esc"}}},
			{ID: "done", Type: Transform, Transform: &TransformSpec{Op: "identity"}},
			{ID: "esc", Type: Gate},
		},
	}
	res, err := ex.Start(context.Background(), f, "run7", "")
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if res.State != "paused" || res.PausedNode != "esc" {
		t.Fatalf("res = %+v, want paused at esc", res)
	}
	// 1 initial + 2 retries = 3 dispatches
	if d := rt.Dispatched(); len(d) != 3 {
		t.Errorf("dispatched %d times, want 3", len(d))
	}
	ns := mustNodeRuns(t, st, "run7")
	for _, n := range ns {
		if n.NodeID == "k1" && n.Attempts != 3 {
			t.Errorf("k1 attempts = %d, want 3", n.Attempts)
		}
	}
}

func TestResumeAfterRestart(t *testing.T) {
	ex, st, rt := newExec(t)
	f := gateFlow()
	_, _ = ex.Start(context.Background(), f, "run8", "")
	// Simulate a process restart: a fresh executor over the same store.
	ex2 := NewExecutor(rt, st)
	if err := ex2.Approve("run8", "g1", ""); err != nil {
		t.Fatalf("approve: %v", err)
	}
	res, err := ex2.Resume(context.Background(), f, "run8")
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	if res.State != "completed" {
		t.Errorf("state = %q, want completed", res.State)
	}
}
