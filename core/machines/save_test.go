package machines

import (
	"os"
	"path/filepath"
	"testing"
)

func testReg(t *testing.T) *Registry {
	t.Helper()
	r, err := Parse([]byte("version: 1\nmachines:\n  - name: hub\n    url: http://10.0.0.1:4087\n    agents: [claude]\n"), "hub")
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestWithMachineAddsAndUpdates(t *testing.T) {
	r := testReg(t)
	r2 := r.WithMachine(Machine{Name: "mini", URL: "http://10.0.0.2:4087", Agents: []string{"claude"}})
	if len(r.Machines) != 1 || len(r2.Machines) != 2 {
		t.Fatalf("WithMachine must not mutate the receiver: %d/%d", len(r.Machines), len(r2.Machines))
	}
	if r2.Local() != "hub" {
		t.Fatalf("local identity lost: %q", r2.Local())
	}
	r3 := r2.WithMachine(Machine{Name: "MINI", URL: "http://10.0.0.9:4087", Agents: []string{"codex"}})
	m, ok := r3.Machine("mini")
	if !ok || m.URL != "http://10.0.0.9:4087" || len(r3.Machines) != 2 {
		t.Fatalf("update-by-name failed: %+v len=%d", m, len(r3.Machines))
	}
	if m.Name != "mini" {
		t.Fatalf("canonical stored casing must win on update, got %q", m.Name)
	}
}

func TestWithoutRemoves(t *testing.T) {
	r := testReg(t).WithMachine(Machine{Name: "mini", URL: "http://10.0.0.2:4087", Agents: []string{"claude"}})
	r2 := r.Without("MINI")
	if len(r2.Machines) != 1 {
		t.Fatalf("Without: got %d machines", len(r2.Machines))
	}
	if _, ok := r2.Machine("mini"); ok {
		t.Fatal("mini still present")
	}
}

func TestSaveRoundTripsAtomically(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "machines.yaml")
	r := testReg(t).WithMachine(Machine{Name: "mini", URL: "http://10.0.0.2:4087", Agents: []string{"claude"}})
	if err := Save(path, r); err != nil {
		t.Fatal(err)
	}
	back, err := Load(path, "hub")
	if err != nil {
		t.Fatal(err)
	}
	if len(back.Machines) != 2 || back.Machines[1].Name != "mini" {
		t.Fatalf("round trip mismatch: %+v", back.Machines)
	}
	ents, _ := os.ReadDir(dir)
	if len(ents) != 1 {
		t.Fatalf("temp file left behind: %v", ents)
	}
}

func TestLiveSwapAndNilPlace(t *testing.T) {
	var l Live
	if got, err := l.Place("claude", ""); got != "" || err != nil {
		t.Fatalf("nil registry Place = %q,%v; want \"\",nil", got, err)
	}
	if _, err := l.Place("claude", "mini"); err == nil {
		t.Fatal("pin with no registry must error")
	}
	l.Store(testReg(t))
	if got, _ := l.Place("claude", ""); got != "hub" {
		t.Fatalf("Place = %q, want hub", got)
	}
	if l.Load().Local() != "hub" {
		t.Fatal("Load lost registry")
	}
}
