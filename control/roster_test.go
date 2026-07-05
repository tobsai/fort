package control

import (
	"testing"

	"github.com/tobsai/fort/core/machines"
)

const rosterYAML = `
version: 1
machines:
  - name: mac-mini
    url: http://mac-mini.local:4087
    agents: [claude, codex, hermes, openclaw]
  - name: macbook-pro
    url: http://macbook-pro.local:4087
    agents: [claude, codex]
`

func TestRosterMapsLocalAndReachability(t *testing.T) {
	reg, err := machines.Parse([]byte(rosterYAML), "mac-mini")
	if err != nil {
		t.Fatal(err)
	}
	var live machines.Live
	live.Store(reg)
	r := NewRoster(&live)
	got := r.Machines()
	if len(got) != 2 {
		t.Fatalf("machines = %d, want 2", len(got))
	}
	// local machine: flagged local + reachable by definition, in registry order.
	if got[0].Name != "mac-mini" || !got[0].Local || !got[0].Reachable {
		t.Errorf("local entry = %+v", got[0])
	}
	// peer: not local, and unreachable until a successful probe.
	if got[1].Name != "macbook-pro" || got[1].Local || got[1].Reachable {
		t.Errorf("peer entry = %+v (want not-local, not-yet-reachable)", got[1])
	}
	if len(got[1].Agents) != 2 {
		t.Errorf("peer agents = %v", got[1].Agents)
	}
}

// TestRosterSeesHotJoin verifies Roster reads the Live pointer per call, so a
// registry installed after construction (e.g. a mesh enrollment) is visible
// without restarting the process (spec 024).
func TestRosterSeesHotJoin(t *testing.T) {
	var live machines.Live
	r := NewRoster(&live)
	if pre := r.Machines(); pre == nil {
		// Contract: /api/machines must emit [] never null (ui/ui_test.go).
		t.Fatal("nil live: Machines() returned nil; want non-nil empty slice")
	} else if len(pre) != 0 {
		t.Fatalf("empty live: %d machines", len(pre))
	}
	reg, _ := machines.Parse([]byte("machines:\n  - name: hub\n    url: http://10.0.0.1:4087\n    agents: [claude]\n"), "hub")
	live.Store(reg)
	got := r.Machines()
	if len(got) != 1 || got[0].Name != "hub" || !got[0].Local || !got[0].Reachable {
		t.Fatalf("after hot join: %+v", got)
	}
}
