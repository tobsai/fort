package machines

import "testing"

const sample = `
version: 1
machines:
  - name: mac-mini
    url: http://mac-mini.local:4087
    agents: [claude, codex, hermes, openclaw]
  - name: macbook-pro
    url: http://macbook-pro.local:4087
    agents: [claude, codex]
`

func mustParse(t *testing.T, local string) *Registry {
	t.Helper()
	r, err := Parse([]byte(sample), local)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return r
}

func TestParseAndLookup(t *testing.T) {
	r := mustParse(t, "mac-mini")
	if got := len(r.Machines); got != 2 {
		t.Fatalf("machines = %d, want 2", got)
	}
	if r.Local() != "mac-mini" {
		t.Fatalf("local = %q, want mac-mini", r.Local())
	}
	m, ok := r.Machine("macbook-pro")
	if !ok || m.URL != "http://macbook-pro.local:4087" {
		t.Fatalf("lookup macbook-pro = %+v ok=%v", m, ok)
	}
	// local name is canonicalized to the registry's stored casing
	r2 := mustParse(t, "MAC-MINI")
	if r2.Local() != "mac-mini" {
		t.Fatalf("canonical local = %q, want mac-mini", r2.Local())
	}
}

func TestParseRejectsBadRegistries(t *testing.T) {
	cases := map[string]string{
		"empty":     "version: 1\nmachines: []\n",
		"no-name":   "machines:\n  - url: http://x:1\n    agents: [claude]\n",
		"no-url":    "machines:\n  - name: a\n    agents: [claude]\n",
		"no-agents": "machines:\n  - name: a\n    url: http://x:1\n",
		"dup-name":  "machines:\n  - {name: a, url: http://x:1, agents: [claude]}\n  - {name: a, url: http://y:1, agents: [codex]}\n",
	}
	for name, doc := range cases {
		if _, err := Parse([]byte(doc), "a"); err == nil {
			t.Errorf("%s: expected parse error, got nil", name)
		}
	}
}

func TestPlacePin(t *testing.T) {
	r := mustParse(t, "mac-mini")
	// explicit pin that offers the agent
	if m, err := r.Place("codex", "macbook-pro"); err != nil || m != "macbook-pro" {
		t.Fatalf("pin codex@macbook-pro = %q,%v", m, err)
	}
	// pin is case-insensitive but returns canonical name
	if m, err := r.Place("codex", "MacBook-Pro"); err != nil || m != "macbook-pro" {
		t.Fatalf("pin canonicalization = %q,%v", m, err)
	}
	// pin to a machine that does not offer the agent
	if _, err := r.Place("openclaw", "macbook-pro"); err == nil {
		t.Fatal("expected error: macbook-pro does not offer openclaw")
	}
	// pin to an unknown machine
	if _, err := r.Place("claude", "nope"); err == nil {
		t.Fatal("expected error: unknown pinned machine")
	}
}

func TestPlacePrefersLocalThenOrder(t *testing.T) {
	// local (mac-mini) offers claude -> stays local
	r := mustParse(t, "mac-mini")
	if m, err := r.Place("claude", ""); err != nil || m != "mac-mini" {
		t.Fatalf("local-pref claude = %q,%v", m, err)
	}
	// local does NOT offer the agent -> first machine in order that does
	local := mustParse(t, "macbook-pro") // offers claude,codex only
	if m, err := local.Place("openclaw", ""); err != nil || m != "mac-mini" {
		t.Fatalf("fallthrough openclaw = %q,%v", m, err)
	}
	// no machine offers the agent
	if _, err := r.Place("ghost", ""); err == nil {
		t.Fatal("expected error: no machine offers ghost")
	}
}

// TestPlacementIsDeterministic mirrors the router's determinism guarantee:
// Place is a pure function of (agent, pin, registry) — same inputs, same output,
// every time, with no model calls on the path.
func TestPlacementIsDeterministic(t *testing.T) {
	r := mustParse(t, "mac-mini")
	first, err := r.Place("codex", "")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 1000; i++ {
		got, err := r.Place("codex", "")
		if err != nil || got != first {
			t.Fatalf("iteration %d: %q,%v != %q", i, got, err, first)
		}
	}
}
