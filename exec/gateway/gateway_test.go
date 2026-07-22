package gateway

import (
	"context"
	"path/filepath"
	"sync"
	"testing"

	"github.com/tobsai/fort/core/runtime"
	"github.com/tobsai/fort/exec/fake"
	"github.com/tobsai/fort/exec/native"
)

type recTracer struct {
	mu    sync.Mutex
	calls []string
}

func (r *recTracer) Dispatch(agent, runID string, cost float64) {
	r.mu.Lock()
	r.calls = append(r.calls, agent)
	r.mu.Unlock()
}

func TestPerFlowBudgetCapEnforced(t *testing.T) {
	tr := &recTracer{}
	g := New(fake.New(), Options{Limit: 2, DefaultCost: 1, Tracer: tr})

	for i := 0; i < 2; i++ {
		if _, err := g.Dispatch(context.Background(), runtime.RunSpec{RunID: "r", Agent: "codex"}); err != nil {
			t.Fatalf("dispatch %d under budget should succeed: %v", i, err)
		}
	}
	// 3rd call exceeds the cap of 2.
	if _, err := g.Dispatch(context.Background(), runtime.RunSpec{RunID: "r", Agent: "codex"}); err == nil {
		t.Fatal("expected budget-exceeded error on the 3rd dispatch")
	}
	if g.Spent() != 2 {
		t.Errorf("spent = %v, want 2", g.Spent())
	}
	// every (admitted) model call was traced
	if len(tr.calls) != 2 {
		t.Errorf("traced %d calls, want 2", len(tr.calls))
	}
}

func TestFailoverToFallbackAgent(t *testing.T) {
	// underlying native runtime knows only "up"; "down" has no provider.
	var fallbackSpec runtime.RunSpec
	under := native.New(filepath.Join(t.TempDir()),
		native.Provider{Name: "up", Command: func(spec runtime.RunSpec) []string {
			fallbackSpec = spec
			return []string{"sh", "-c", "echo ok"}
		}})
	tr := &recTracer{}
	g := New(under, Options{DefaultCost: 0, Tracer: tr, Failover: map[string]string{"down": "up"}})

	run, err := g.Dispatch(context.Background(), runtime.RunSpec{RunID: "r", Agent: "down", Model: "primary-only-model"})
	if err != nil {
		t.Fatalf("failover dispatch: %v", err)
	}
	st := run.Wait()
	if st.State != runtime.StateSucceeded {
		t.Errorf("state = %v, want succeeded via failover", st.State)
	}
	// traced the primary attempt and the failover
	if len(tr.calls) < 2 || tr.calls[len(tr.calls)-1] != "up" {
		t.Errorf("trace = %v, want a failover to up", tr.calls)
	}
	if fallbackSpec.Agent != "up" || fallbackSpec.Model != "" {
		t.Errorf("fallback spec = %+v, want up with provider default model", fallbackSpec)
	}
}

func TestImplementsRuntime(t *testing.T) {
	var _ runtime.Runtime = New(fake.New(), Options{})
}
