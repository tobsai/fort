package graph

import (
	"context"
	"testing"
)

// TestFanoutFaninRunsAllBranches exercises the fanout/fanin node types: a
// fanout splits into two transform branches that converge on a fanin, after
// which a final task runs once.
func TestFanoutFaninRunsAllBranches(t *testing.T) {
	ex, st, rt := newExec(t)
	f := Flow{
		ID: "fo", Start: "split",
		Nodes: []Node{
			{ID: "split", Type: Fanout, FaninID: "join",
				Edges: []Edge{{On: OutAlways, To: "a"}, {On: OutAlways, To: "b"}}},
			{ID: "a", Type: Transform, Transform: &TransformSpec{Op: "upper"}, Edges: []Edge{{On: OutAlways, To: "join"}}},
			{ID: "b", Type: Transform, Transform: &TransformSpec{Op: "lower"}, Edges: []Edge{{On: OutAlways, To: "join"}}},
			{ID: "join", Type: Fanin, Edges: []Edge{{On: OutAlways, To: "k"}}},
			{ID: "k", Type: Task, Agent: "codex"},
		},
	}
	res, err := ex.Start(context.Background(), f, "fan1", "Hi")
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if res.State != "completed" {
		t.Fatalf("state = %q, want completed", res.State)
	}
	for _, id := range []string{"split", "a", "b", "join", "k"} {
		if s, ok := nodeStatus(t, st, "fan1", id); !ok || s != "succeeded" {
			t.Errorf("node %s status=%q ok=%v", id, s, ok)
		}
	}
	// the post-join task ran exactly once
	if d := rt.Dispatched(); len(d) != 1 {
		t.Errorf("dispatched %d, want 1", len(d))
	}
}
