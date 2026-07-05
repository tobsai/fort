package cluster

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/tobsai/fort/core/runtime"
	"github.com/tobsai/fort/exec/fake"
)

func lastAgent(t *testing.T, rt *fake.Runtime) string {
	t.Helper()
	specs := rt.Dispatched()
	if len(specs) == 0 {
		return ""
	}
	return specs[len(specs)-1].Agent
}

func TestDispatchesLocalForEmptyOrLocalMachine(t *testing.T) {
	local := fake.New()
	remote := fake.New()
	c := New("mac-mini", local, map[string]runtime.Runtime{"macbook-pro": remote})

	for _, m := range []string{"", "mac-mini"} {
		if _, err := c.Dispatch(context.Background(), runtime.RunSpec{RunID: "x", Agent: "codex", Machine: m}); err != nil {
			t.Fatalf("dispatch machine=%q: %v", m, err)
		}
	}
	if got := len(local.Dispatched()); got != 2 {
		t.Fatalf("local dispatched = %d, want 2", got)
	}
	if got := len(remote.Dispatched()); got != 0 {
		t.Fatalf("remote dispatched = %d, want 0", got)
	}
}

func TestDispatchesRemoteForPeerMachine(t *testing.T) {
	local := fake.New()
	remote := fake.New()
	c := New("mac-mini", local, map[string]runtime.Runtime{"macbook-pro": remote})

	if _, err := c.Dispatch(context.Background(), runtime.RunSpec{RunID: "y", Agent: "codex", Machine: "macbook-pro"}); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if got := len(remote.Dispatched()); got != 1 {
		t.Fatalf("remote dispatched = %d, want 1", got)
	}
	if got := len(local.Dispatched()); got != 0 {
		t.Fatalf("local dispatched = %d, want 0", got)
	}
	if a := lastAgent(t, remote); a != "codex" {
		t.Fatalf("remote agent = %q", a)
	}
}

func TestUnknownMachineErrors(t *testing.T) {
	c := New("mac-mini", fake.New(), nil)
	if _, err := c.Dispatch(context.Background(), runtime.RunSpec{RunID: "z", Agent: "codex", Machine: "ghost"}); err == nil {
		t.Fatal("expected error for unknown machine")
	}
}

func TestHotAddRemove(t *testing.T) {
	local := fake.New()
	c := New("hub", local, nil)
	if _, err := c.Dispatch(context.Background(), runtime.RunSpec{RunID: "r1", Machine: "mini"}); err == nil {
		t.Fatal("dispatch to unknown machine must fail")
	}
	c.Add("mini", fake.New())
	if _, err := c.Dispatch(context.Background(), runtime.RunSpec{RunID: "r2", Machine: "mini"}); err != nil {
		t.Fatalf("dispatch after Add: %v", err)
	}
	c.Remove("mini")
	if _, err := c.Dispatch(context.Background(), runtime.RunSpec{RunID: "r3", Machine: "mini"}); err == nil {
		t.Fatal("dispatch after Remove must fail")
	}
}

func TestAddIsRaceSafe(t *testing.T) {
	c := New("hub", fake.New(), nil)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(2)
		go func(i int) { defer wg.Done(); c.Add(fmt.Sprintf("m%d", i), fake.New()) }(i)
		go func(i int) {
			defer wg.Done()
			_, _ = c.Dispatch(context.Background(), runtime.RunSpec{RunID: "r", Machine: fmt.Sprintf("m%d", i)})
		}(i)
	}
	wg.Wait()
}
