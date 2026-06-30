package flow

import (
	"strings"
	"testing"

	"github.com/tobsai/fort/core/graph"
)

const sample = `
id: demo
name: Demo Flow
start: a
nodes:
  - id: a
    type: transform
    transform: { op: identity }
    edges:
      - { on: success, to: b }
  - id: b
    type: task
    agent: codex
`

func TestLoadParsesFlow(t *testing.T) {
	f, err := Load([]byte(sample))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if f.ID != "demo" || f.Start != "a" || len(f.Nodes) != 2 {
		t.Fatalf("flow = %+v", f)
	}
	if f.Nodes[0].Type != graph.Transform || f.Nodes[1].Agent != "codex" {
		t.Errorf("nodes = %+v", f.Nodes)
	}
	if len(f.Nodes[0].Edges) != 1 || f.Nodes[0].Edges[0].To != "b" {
		t.Errorf("edges = %+v", f.Nodes[0].Edges)
	}
}

func TestLoadRejectsUnknownStart(t *testing.T) {
	_, err := Load([]byte("id: x\nstart: nope\nnodes:\n  - {id: a, type: task}\n"))
	if err == nil || !strings.Contains(err.Error(), "start") {
		t.Fatalf("expected start error, got %v", err)
	}
}

func TestLoadRejectsDanglingEdge(t *testing.T) {
	bad := `
id: x
start: a
nodes:
  - id: a
    type: task
    edges:
      - { on: success, to: ghost }
`
	_, err := Load([]byte(bad))
	if err == nil || !strings.Contains(err.Error(), "ghost") {
		t.Fatalf("expected dangling-edge error naming ghost, got %v", err)
	}
}

func TestLoadDirLoadsRealFlows(t *testing.T) {
	flows, err := LoadDir("../../flows")
	if err != nil {
		t.Fatalf("loaddir: %v", err)
	}
	if len(flows) < 2 {
		t.Fatalf("loaded %d flows, want >=2 (game-starter, ship-feature)", len(flows))
	}
	byID := map[string]graph.Flow{}
	for _, f := range flows {
		byID[f.ID] = f
	}
	if _, ok := byID["game-starter"]; !ok {
		t.Errorf("missing game-starter flow; have %v", keys(byID))
	}
	if _, ok := byID["ship-feature"]; !ok {
		t.Errorf("missing ship-feature flow; have %v", keys(byID))
	}
}

func keys(m map[string]graph.Flow) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}
