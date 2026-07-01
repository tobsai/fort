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
	r := NewRoster(reg)
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
