package main

import (
	"path/filepath"
	"testing"

	"github.com/tobsai/fort/control"
	"github.com/tobsai/fort/core/graph"
	"github.com/tobsai/fort/core/store"
	"github.com/tobsai/fort/exec/fake"
	"github.com/tobsai/fort/ui"
)

func TestWirePlaybooksInControlAndExecutionModes(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "fort.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	controlOnly := wirePlaybooks(ui.Deps{Store: st}, st, nil)
	if controlOnly.Playbooks == nil || controlOnly.PlaybookRunner != nil || controlOnly.Runner != nil {
		t.Fatalf("control-only deps = %+v", controlOnly)
	}

	runner := control.NewFlowExecutor(graph.NewExecutor(fake.New(), st), nil)
	full := wirePlaybooks(ui.Deps{Store: st}, st, &runner)
	if full.Playbooks == nil || full.PlaybookRunner == nil || full.Runner == nil {
		t.Fatalf("full deps = %+v", full)
	}
}
