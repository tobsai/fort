# Mesh Enrollment (`fort mesh`) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement spec 024 — `fort mesh invite|join|remove` so pairing machines requires zero manual token/YAML management.

**Architecture:** All enrollment runs inside the `fort serve` daemon (spec D8); the CLI subcommands are thin HTTP clients. A shared `machines.Live` (atomic registry pointer) feeds placer, cluster, and roster so joins apply hot. The mesh token becomes dynamic (`func() string`) so first-invite bootstrap needs no restart. Registry persists as Fort-managed `machines.yaml` beside the DB; each node's identity/token persists in `node.yaml` (0600).

**Tech Stack:** Go 1.22 stdlib (`net/http`, `crypto/rand`, `crypto/subtle`, `encoding/base32`, `sync/atomic`), `gopkg.in/yaml.v3`, modernc SQLite via existing `core/store`.

**Ground rules (CLAUDE.md):** TDD every task — write the failing test, watch it fail, minimal code, watch it pass. `go test ./...` stays green after every task. `-race` on anything concurrent. Respect seams: `core` never imports `ui`/concrete exec; `ui` never imports engine/router/native. Commit after each task on the feature branch.

**Spec:** `specs/024-mesh-enrollment.md` (approved). Read it before starting.

---

### Task 0: Branch + spec commit

**Files:** none (git only)

- [ ] **Step 0.1:** `git checkout -b feat/024-mesh-enrollment`
- [ ] **Step 0.2:** `git add specs/024-mesh-enrollment.md docs/superpowers/plans/2026-07-04-mesh-enrollment.md && git commit -m "spec(024): mesh enrollment — self-managing pairing"`
  (Do NOT `git add` unrelated dirty files — `ui/apple/*`, `docs/notes/testflight.md`, `ui/apple/ExportOptions.plist` belong to TestFlight work, leave them untouched.)

---

### Task 1: `core/machines` — immutable mutation helpers + atomic Save

**Files:**
- Create: `core/machines/save.go`
- Test: `core/machines/save_test.go`

- [ ] **Step 1.1: Write the failing tests**

```go
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
	// Update by name (case-insensitive) replaces url+agents in place.
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
```

- [ ] **Step 1.2:** `go test ./core/machines/ -run 'WithMachine|Without|SaveRoundTrips|LiveSwap' -v` — expect FAIL (undefined: WithMachine/Without/Save/Live).

- [ ] **Step 1.3: Implement `core/machines/save.go`**

```go
package machines

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"gopkg.in/yaml.v3"
)

// WithMachine returns a copy of r with m added, or — when a machine of the
// same name (case-insensitive) exists — that entry's url and agents replaced.
// The stored (canonical) casing of an existing name wins. r is not mutated.
func (r *Registry) WithMachine(m Machine) *Registry {
	out := &Registry{Version: r.Version, local: r.local}
	if out.Version == 0 {
		out.Version = 1
	}
	replaced := false
	for _, e := range r.Machines {
		if strings.EqualFold(e.Name, m.Name) {
			e.URL, e.Agents = m.URL, m.Agents // keep canonical e.Name
			replaced = true
		}
		out.Machines = append(out.Machines, e)
	}
	if !replaced {
		out.Machines = append(out.Machines, m)
	}
	return out
}

// Without returns a copy of r with the named machine (case-insensitive)
// removed. r is not mutated.
func (r *Registry) Without(name string) *Registry {
	out := &Registry{Version: r.Version, local: r.local}
	for _, e := range r.Machines {
		if !strings.EqualFold(e.Name, name) {
			out.Machines = append(out.Machines, e)
		}
	}
	return out
}

// Save writes r as machines.yaml at path atomically: temp file in the same
// directory, then rename. machines.yaml holds no secrets (0644 is fine).
func Save(path string, r *Registry) error {
	data, err := yaml.Marshal(r)
	if err != nil {
		return fmt.Errorf("machines: marshal: %w", err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("machines: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".machines-*.yaml")
	if err != nil {
		return fmt.Errorf("machines: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return fmt.Errorf("machines: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("machines: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("machines: %w", err)
	}
	return nil
}

// Live is a swappable registry shared by the placer, the cluster runtime's
// wiring, and the roster (spec 024): enrollment stores a new *Registry and
// every reader sees it immediately. The zero value holds no registry, which
// behaves exactly like single-machine mode.
type Live struct {
	p atomic.Pointer[Registry]
}

// Load returns the current registry, or nil when none is installed.
func (l *Live) Load() *Registry { return l.p.Load() }

// Store installs reg as the current registry.
func (l *Live) Store(reg *Registry) { l.p.Store(reg) }

// Place implements engine.Placer over the current registry. With no registry
// installed it preserves single-machine semantics: no placement, but an
// explicit pin is an error (there is nothing to pin to).
func (l *Live) Place(agent, pin string) (string, error) {
	reg := l.p.Load()
	if reg == nil {
		if pin != "" {
			return "", fmt.Errorf("machines: pinned machine %q but no registry is configured", pin)
		}
		return "", nil
	}
	return reg.Place(agent, pin)
}
```

Note the yaml round-trip: `Registry` has yaml tags for Version/Machines and an unexported `local` — marshal emits exactly `version` + `machines`. Verify `yaml.Marshal` output loads via existing `Parse` (the round-trip test covers it).

- [ ] **Step 1.4:** `go test ./core/machines/ -v` — expect PASS (all, including existing tests).
- [ ] **Step 1.5:** `git add core/machines/ && git commit -m "feat(machines): immutable mutation helpers, atomic Save, Live swappable registry (spec 024)"`

---

### Task 2: `core/config` — data dir, node.yaml, layered Load

**Files:**
- Create: `core/config/node.go`
- Modify: `core/config/config.go` (add `MachinesManaged` field only)
- Test: `core/config/node_test.go`

- [ ] **Step 2.1: Write the failing tests**

```go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestDataDirFollowsDBPath(t *testing.T) {
	c := Default()
	if got := c.DataDir(); got != ".fort-native" {
		t.Fatalf("DataDir = %q", got)
	}
	c.DBPath = "/var/lib/fort/fort.db"
	if got := c.DataDir(); got != "/var/lib/fort" {
		t.Fatalf("DataDir = %q", got)
	}
}

func TestNodeFileRoundTripAndMode(t *testing.T) {
	dir := t.TempDir()
	nf := NodeFile{Name: "hub", Token: "sekrit", Addr: "0.0.0.0:4087"}
	if err := SaveNodeFile(dir, nf); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(filepath.Join(dir, "node.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("node.yaml mode = %o, want 0600", fi.Mode().Perm())
	}
	back, err := ReadNodeFile(dir)
	if err != nil {
		t.Fatal(err)
	}
	if back != nf {
		t.Fatalf("round trip: %+v != %+v", back, nf)
	}
}

func TestReadNodeFileMissingIsZero(t *testing.T) {
	nf, err := ReadNodeFile(t.TempDir())
	if err != nil || nf != (NodeFile{}) {
		t.Fatalf("missing node.yaml: %+v, %v", nf, err)
	}
}

func TestLoadPrecedenceEnvOverNodeFileOverDefault(t *testing.T) {
	dir := t.TempDir()
	db := filepath.Join(dir, "fort.db")
	if err := SaveNodeFile(dir, NodeFile{Name: "filename", Token: "filetoken", Addr: "1.2.3.4:1"}); err != nil {
		t.Fatal(err)
	}
	// node.yaml fills gaps…
	c := Load(env(map[string]string{"FORT_DB": db}))
	if c.NodeName != "filename" || c.NodeToken != "filetoken" || c.Addr != "1.2.3.4:1" {
		t.Fatalf("node.yaml layer not applied: %+v", c)
	}
	// …but env wins.
	c = Load(env(map[string]string{
		"FORT_DB": db, "FORT_NODE_TOKEN": "envtoken", "FORT_ADDR": "9.9.9.9:9", "FORT_NODE_NAME": "envname",
	}))
	if c.NodeName != "envname" || c.NodeToken != "envtoken" || c.Addr != "9.9.9.9:9" {
		t.Fatalf("env must win: %+v", c)
	}
}

func TestLoadManagedRegistryDiscovery(t *testing.T) {
	dir := t.TempDir()
	db := filepath.Join(dir, "fort.db")
	// No managed file, no env: single-machine.
	c := Load(env(map[string]string{"FORT_DB": db}))
	if c.MachinesPath != "" || c.MachinesManaged {
		t.Fatalf("expected single-machine: %+v", c)
	}
	// Managed file exists: auto-load, flagged managed.
	managed := filepath.Join(dir, "machines.yaml")
	if err := os.WriteFile(managed, []byte("machines: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c = Load(env(map[string]string{"FORT_DB": db}))
	if c.MachinesPath != managed || !c.MachinesManaged {
		t.Fatalf("managed discovery failed: %+v", c)
	}
	// Explicit FORT_MACHINES overrides and is NOT managed.
	c = Load(env(map[string]string{"FORT_DB": db, "FORT_MACHINES": "/tmp/op.yaml"}))
	if c.MachinesPath != "/tmp/op.yaml" || c.MachinesManaged {
		t.Fatalf("operator override broken: %+v", c)
	}
}
```

- [ ] **Step 2.2:** `go test ./core/config/ -v` — expect FAIL (undefined: DataDir/NodeFile/Load…).

- [ ] **Step 2.3: Implement.** Add to `core/config/config.go` inside the `Config` struct, after `NodeToken`:

```go
	// MachinesManaged is true when MachinesPath points at the Fort-managed
	// registry in the data dir (spec 024). Enrollment only ever writes the
	// managed file; an operator-set FORT_MACHINES is never touched.
	MachinesManaged bool
```

Create `core/config/node.go`:

```go
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// NodeFile is the persisted mesh identity of one machine (spec 024): written
// by `fort mesh join` on workers and by the first `fort mesh invite` on the
// hub. It contains the shared mesh token, so it is always written 0600.
type NodeFile struct {
	Name  string `yaml:"name"`
	Token string `yaml:"token"`
	Addr  string `yaml:"addr"`
}

// DataDir is where Fort keeps machine-local state (node.yaml, the managed
// machines.yaml): the directory holding the SQLite DB.
func (c Config) DataDir() string { return filepath.Dir(c.DBPath) }

// ReadNodeFile loads dir/node.yaml. A missing file is not an error — it
// returns the zero NodeFile (nothing enrolled yet).
func ReadNodeFile(dir string) (NodeFile, error) {
	var nf NodeFile
	data, err := os.ReadFile(filepath.Join(dir, "node.yaml"))
	if os.IsNotExist(err) {
		return nf, nil
	}
	if err != nil {
		return nf, fmt.Errorf("config: node.yaml: %w", err)
	}
	if err := yaml.Unmarshal(data, &nf); err != nil {
		return nf, fmt.Errorf("config: node.yaml: %w", err)
	}
	return nf, nil
}

// SaveNodeFile writes dir/node.yaml atomically with mode 0600 throughout —
// the temp file is created 0600 before any bytes are written, so the token
// is never world-readable, even transiently.
func SaveNodeFile(dir string, nf NodeFile) error {
	data, err := yaml.Marshal(nf)
	if err != nil {
		return fmt.Errorf("config: node.yaml: %w", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("config: %w", err)
	}
	tmp := filepath.Join(dir, ".node.yaml.tmp")
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmp)
		return fmt.Errorf("config: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("config: %w", err)
	}
	if err := os.Rename(tmp, filepath.Join(dir, "node.yaml")); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("config: %w", err)
	}
	return nil
}

// Load is FromEnv plus the spec-024 layers, precedence env > node.yaml >
// defaults for NodeToken / NodeName / Addr, and managed-registry discovery:
// FORT_MACHINES env > <data-dir>/machines.yaml exists > single-machine.
func Load(getenv func(string) string) Config {
	c := FromEnv(getenv)
	nf, err := ReadNodeFile(c.DataDir())
	if err == nil {
		if getenv("FORT_NODE_TOKEN") == "" && nf.Token != "" {
			c.NodeToken = nf.Token
		}
		if getenv("FORT_NODE_NAME") == "" && nf.Name != "" {
			c.NodeName = nf.Name
		}
		if getenv("FORT_ADDR") == "" && nf.Addr != "" {
			c.Addr = nf.Addr
		}
	}
	if getenv("FORT_MACHINES") == "" {
		managed := filepath.Join(c.DataDir(), "machines.yaml")
		if _, statErr := os.Stat(managed); statErr == nil {
			c.MachinesPath = managed
			c.MachinesManaged = true
		}
	}
	return c
}
```

(A corrupt `node.yaml` is surfaced later by the daemon; `Load` deliberately ignores the read error so read-only CLI commands still work. The daemon path re-reads via `ReadNodeFile` where errors are checked — Task 6.)

- [ ] **Step 2.4:** `go test ./core/config/ -v` — expect PASS.
- [ ] **Step 2.5:** `git add core/config/ && git commit -m "feat(config): data dir, node.yaml layer, managed-registry discovery (spec 024)"`

---

### Task 3: `core/store` — invite table

**Files:**
- Create: `core/store/invites.go`
- Modify: `core/store/store.go:114` (schema string: add table)
- Test: `core/store/invites_test.go`

- [ ] **Step 3.1: Write the failing tests**

```go
package store

import (
	"path/filepath"
	"testing"
	"time"
)

func openTest(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "fort.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestInviteLifecycle(t *testing.T) {
	s := openTest(t)
	now := time.Now()
	if err := s.CreateInvite("hash1", now.Add(15*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := s.CheckInvite("hash1", now); err != nil {
		t.Fatalf("fresh invite should check: %v", err)
	}
	if err := s.CheckInvite("nope", now); err != ErrInviteInvalid {
		t.Fatalf("unknown code: %v, want ErrInviteInvalid", err)
	}
	if err := s.CheckInvite("hash1", now.Add(16*time.Minute)); err != ErrInviteExpired {
		t.Fatalf("expired: %v, want ErrInviteExpired", err)
	}
	if err := s.MarkInviteUsed("hash1", now); err != nil {
		t.Fatal(err)
	}
	if err := s.CheckInvite("hash1", now); err != ErrInviteInvalid {
		t.Fatalf("used code must be invalid: %v", err)
	}
	// Marking twice is an error (single use is enforced at mark time too).
	if err := s.MarkInviteUsed("hash1", now); err == nil {
		t.Fatal("second MarkInviteUsed must fail")
	}
}
```

- [ ] **Step 3.2:** `go test ./core/store/ -run Invite -v` — expect FAIL.

- [ ] **Step 3.3: Implement.** In `core/store/store.go`, extend the `schema` const (inside `migrate`, after the `event` index line, before the closing backtick):

```sql
CREATE TABLE IF NOT EXISTS invite (
  code_hash TEXT PRIMARY KEY, created_at TEXT, expires_at TEXT, used_at TEXT
);
```

Create `core/store/invites.go`:

```go
package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Invite errors distinguish the join responses (spec 024): 401 vs 410.
var (
	ErrInviteInvalid = errors.New("store: invite invalid or already used")
	ErrInviteExpired = errors.New("store: invite expired")
)

// CreateInvite records a hashed single-use invite code.
func (s *Store) CreateInvite(codeHash string, expires time.Time) error {
	_, err := s.db.Exec(
		`INSERT INTO invite (code_hash, created_at, expires_at) VALUES (?, ?, ?)`,
		codeHash, nowOr(time.Time{}), expires.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("store: create invite: %w", err)
	}
	return nil
}

// CheckInvite verifies codeHash names an unused, unexpired invite. It does
// not consume it — the join flow persists the registry first and only then
// calls MarkInviteUsed (spec 024 ordering).
func (s *Store) CheckInvite(codeHash string, now time.Time) error {
	var expiresAt string
	var usedAt sql.NullString
	err := s.db.QueryRow(
		`SELECT expires_at, used_at FROM invite WHERE code_hash = ?`, codeHash,
	).Scan(&expiresAt, &usedAt)
	if err == sql.ErrNoRows {
		return ErrInviteInvalid
	}
	if err != nil {
		return fmt.Errorf("store: check invite: %w", err)
	}
	if usedAt.Valid {
		return ErrInviteInvalid
	}
	if now.UTC().After(parseTime(expiresAt)) {
		return ErrInviteExpired
	}
	return nil
}

// MarkInviteUsed consumes the invite. The WHERE used_at IS NULL guard makes
// consumption single-use even under concurrent joins.
func (s *Store) MarkInviteUsed(codeHash string, now time.Time) error {
	res, err := s.db.Exec(
		`UPDATE invite SET used_at = ? WHERE code_hash = ? AND used_at IS NULL`,
		now.UTC().Format(time.RFC3339Nano), codeHash,
	)
	if err != nil {
		return fmt.Errorf("store: mark invite used: %w", err)
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return ErrInviteInvalid
	}
	return nil
}
```

- [ ] **Step 3.4:** `go test ./core/store/ -v` — expect PASS.
- [ ] **Step 3.5:** `git add core/store/ && git commit -m "feat(store): single-use invite table (spec 024)"`

---

### Task 4: `exec/cluster` — hot Add/Remove

**Files:**
- Modify: `exec/cluster/cluster.go`
- Test: `exec/cluster/cluster_test.go` (extend)

- [ ] **Step 4.1: Write the failing test** (append to the existing test file; reuse its fake runtime if one exists, else this stub):

```go
func TestHotAddRemove(t *testing.T) {
	local := fake.New()
	c := cluster.New("hub", local, nil)
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
	c := cluster.New("hub", fake.New(), nil)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(2)
		go func(i int) { defer wg.Done(); c.Add(fmt.Sprintf("m%d", i), fake.New()) }(i)
		go func() { defer wg.Done(); _, _ = c.Dispatch(context.Background(), runtime.RunSpec{RunID: "r", Machine: "hub"}) }()
	}
	wg.Wait()
}
```

- [ ] **Step 4.2:** `go test ./exec/cluster/ -race -v` — expect FAIL (undefined Add/Remove).

- [ ] **Step 4.3: Implement** — in `exec/cluster/cluster.go` add `"sync"` to imports, a `mu sync.RWMutex` field guarding `remotes`, and:

```go
// Add installs (or replaces) the transport for a peer machine. Used by mesh
// enrollment (spec 024) to apply a join without restarting the daemon.
func (r *Runtime) Add(name string, rt runtime.Runtime) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.remotes[name] = rt
}

// Remove drops the transport for a peer machine.
func (r *Runtime) Remove(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.remotes, name)
}
```

and in `Dispatch`, guard the map read:

```go
	r.mu.RLock()
	rt, ok := r.remotes[spec.Machine]
	r.mu.RUnlock()
```

- [ ] **Step 4.4:** `go test ./exec/cluster/ -race -v` — expect PASS.
- [ ] **Step 4.5:** `git add exec/cluster/ && git commit -m "feat(cluster): mutex'd hot Add/Remove of peer transports (spec 024)"`

---

### Task 5: `exec/node` — dynamic token

**Files:**
- Modify: `exec/node/node.go` (`New` takes `func() string`; `authed` calls it)
- Modify: every `node.New(` call site — `cmd/fort/main.go:195` and `exec/node/node_test.go`

- [ ] **Step 5.1: Update the test first.** In `exec/node/node_test.go`, change constructions like `node.New(rt, "tok")` to `node.New(rt, func() string { return "tok" })` and add:

```go
func TestTokenBecomesLiveWithoutRestart(t *testing.T) {
	tok := ""
	srv := node.New(fake.New(), func() string { return tok })
	mux := http.NewServeMux()
	srv.Register(mux)
	req := httptest.NewRequest("POST", "/api/exec", strings.NewReader("{}"))
	req.Header.Set("Authorization", "Bearer later")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("empty token: %d, want 403", rec.Code)
	}
	tok = "later" // first `mesh invite` minted the token — no restart
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/api/exec", strings.NewReader("{}")))
	// wrong/missing header still 401
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no header: %d, want 401", rec.Code)
	}
}
```

- [ ] **Step 5.2:** `go test ./exec/node/` — expect compile FAIL.
- [ ] **Step 5.3: Implement:** in `node.go` change the field to `token func() string`, `New(rt runtime.Runtime, token func() string) *Server`, and in `authed`: `tok := s.token()`, empty → the existing 403 branch; compare against `[]byte("Bearer " + tok)`. Update `cmd/fort/main.go:194-200`: build the node server unconditionally (`nodeSrv := node.New(a.localRT, tokenFn)` — `tokenFn` arrives in Task 6; for this commit pass `func() string { return a.cfg.NodeToken }`) and always mount it (drop the `if a.cfg.NodeToken != ""` guard — 403-when-empty is the disabled behavior, matching spec 024 rollback semantics).
- [ ] **Step 5.4:** `go test ./exec/node/ ./cmd/... -race` — expect PASS.
- [ ] **Step 5.5:** `git add exec/node/ cmd/fort/ && git commit -m "feat(node): dynamic token source; exec endpoint always mounted, 403 until token exists (spec 024)"`

---

### Task 6: `exec/meshjoin` — the enrollment server

**Files:**
- Create: `exec/meshjoin/meshjoin.go` (server + handlers)
- Create: `exec/meshjoin/netcheck.go` (URL validation + hub-URL detection)
- Create: `exec/meshjoin/token.go` (TokenStore)
- Test: `exec/meshjoin/meshjoin_test.go`, `exec/meshjoin/netcheck_test.go`

This is the heart of the spec. Public surface:

```go
// token.go
package meshjoin

import (
	"crypto/rand"
	"encoding/hex"
	"sync"

	"github.com/tobsai/fort/core/config"
)

// TokenStore holds the mesh token for live readers (node auth, outbound
// remotes) and persists it to node.yaml on first mint (spec 024 bootstrap).
type TokenStore struct {
	mu      sync.Mutex
	val     string
	dataDir string
	name    string // this machine's NodeName, persisted alongside
	addr    string // this machine's listen addr, persisted alongside
}

func NewTokenStore(initial, dataDir, name, addr string) *TokenStore {
	return &TokenStore{val: initial, dataDir: dataDir, name: name, addr: addr}
}

// Get returns the current token ("" until minted or configured).
func (t *TokenStore) Get() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.val
}

// Ensure returns the token, minting and persisting it on first use.
// minted reports whether this call created it (the caller prints the
// hub-now-accepts-exec notice exactly once).
func (t *TokenStore) Ensure() (tok string, minted bool, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.val != "" {
		return t.val, false, nil
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", false, err
	}
	tok = hex.EncodeToString(buf)
	if err := config.SaveNodeFile(t.dataDir, config.NodeFile{Name: t.name, Token: tok, Addr: t.addr}); err != nil {
		return "", false, err
	}
	t.val = tok
	return tok, true, nil
}
```

```go
// netcheck.go
package meshjoin

import (
	"fmt"
	"net"
	"net/url"
)

// privateIP reports whether ip is on a network we trust with the cleartext
// mesh token: RFC1918, CGNAT 100.64/10 (Tailscale), or IPv6 ULA. Loopback and
// anything public are rejected (spec 024 wire validation).
func privateIP(ip net.IP) bool {
	if ip == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	if ip.IsPrivate() { // RFC1918 + ULA fc00::/7
		return true
	}
	if cgnat := (&net.IPNet{IP: net.IPv4(100, 64, 0, 0), Mask: net.CIDRMask(10, 32)}); cgnat.Contains(ip) {
		return true
	}
	return false
}

// validateWorkerURL enforces spec 024 on a derived or advertised worker URL:
// http scheme, parsable host, and every address it names must be private.
// Hostnames are resolved at validation time; all resolved IPs must pass.
func validateWorkerURL(raw string, resolve func(string) ([]net.IP, error)) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "http" || u.Host == "" {
		return fmt.Errorf("meshjoin: url %q must be http://host:port", raw)
	}
	host := u.Hostname()
	if ip := net.ParseIP(host); ip != nil {
		if !privateIP(ip) {
			return fmt.Errorf("meshjoin: %s is not a private/tailnet address", host)
		}
		return nil
	}
	ips, err := resolve(host)
	if err != nil || len(ips) == 0 {
		return fmt.Errorf("meshjoin: cannot resolve %q", host)
	}
	for _, ip := range ips {
		if !privateIP(ip) {
			return fmt.Errorf("meshjoin: %s resolves to non-private %s", host, ip)
		}
	}
	return nil
}

// detectHubURL picks the hub's advertised URL: the first non-loopback
// interface address, preferring the CGNAT/Tailscale 100.64/10 range.
func detectHubURL(port string, addrs func() ([]net.Addr, error)) (string, error) {
	list, err := addrs()
	if err != nil {
		return "", err
	}
	var fallback string
	for _, a := range list {
		ipn, ok := a.(*net.IPNet)
		if !ok || ipn.IP.To4() == nil || !privateIP(ipn.IP) {
			continue
		}
		u := fmt.Sprintf("http://%s:%s", ipn.IP, port)
		if cgnat := (&net.IPNet{IP: net.IPv4(100, 64, 0, 0), Mask: net.CIDRMask(10, 32)}); cgnat.Contains(ipn.IP) {
			return u, nil // tailscale address wins immediately
		}
		if fallback == "" {
			fallback = u
		}
	}
	if fallback == "" {
		return "", fmt.Errorf("meshjoin: no non-loopback private interface found; pass --advertise")
	}
	return fallback, nil
}
```

```go
// meshjoin.go (signatures + behavior contract; write the bodies to match)
package meshjoin

// Server hosts the spec-024 enrollment endpoints on the hub daemon:
//
//	POST   /api/mesh/join              worker enrollment (code-authenticated)
//	POST   /api/mesh/invite            admin: mint invite   (loopback only)
//	DELETE /api/mesh/machines/{name}   admin: remove peer   (loopback only)
type Server struct {
	Live         *machines.Live        // shared registry pointer
	RegistryPath string                // managed machines.yaml path
	Managed      bool                  // false ⇒ FORT_MACHINES set ⇒ refuse writes
	Cluster      *cluster.Runtime      // hot Add/Remove
	Store        *store.Store          // invites + events
	Tokens       *TokenStore
	NodeName     string                // hub identity
	Port         string                // hub bind port (for detectHubURL)
	ProbeAgents  func() []string       // $PATH probe, injected (cmd/fort)
	Now          func() time.Time      // injectable clock for tests
	Log          *slog.Logger
}

func (s *Server) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/mesh/join", s.handleJoin)
	mux.HandleFunc("POST /api/mesh/invite", s.loopbackOnly(s.handleInvite))
	mux.HandleFunc("DELETE /api/mesh/machines/{name}", s.loopbackOnly(s.handleRemove))
}
```

Behavior contract (each bullet is a test in `meshjoin_test.go` — use `httptest`, a temp dir, `store.Open(t.TempDir()+"/fort.db")`, `fake` runtimes, `Now` pinned):

1. `loopbackOnly` splits `r.RemoteAddr` host; non-loopback → 403. (httptest requests default to `192.0.2.1:1234` → forbidden; set `req.RemoteAddr = "127.0.0.1:555"` for allowed paths.)
2. **invite**: `{"ttl":"15m","advertise":""}`.
   - ttl > 1h → 400 `"ttl capped at 1h"`. ttl empty → 15m.
   - If `!Managed` and `RegistryPath` came from FORT_MACHINES → 409 with guidance (test asserts message mentions FORT_MACHINES).
   - Ensures token (`Tokens.Ensure`), on `minted` includes `"minted":true` in the response (CLI prints the posture notice).
   - Hub self-entry: if `Live.Load()==nil` or hub name absent — build `Machine{Name: NodeName, URL: hubURL, Agents: ProbeAgents()}`, `WithMachine`, `machines.Save(RegistryPath, reg)`, `Live.Store(reg)`. hubURL = advertise override else `detectHubURL(Port, net.InterfaceAddrs)`.
   - Mints code: 5 random bytes → Crockford base32 (alphabet `0123456789ABCDEFGHJKMNPQRSTVWXYZ`) → 8 chars, hyphenated `XXXX-XXXX` for display; SHA-256 hex of the **un-hyphenated** code stored via `CreateInvite`.
   - Response: `{"code":"XXXX-XXXX","hub_url":"http://…","join_cmd":"fort mesh join http://… --code XXXX-XXXX","minted":false}`.
3. **join**: `{"code":"XXXX-XXXX","port":4087,"name":"mini","agents":["claude"],"advertise_url":""}`.
   - Normalizes code (strip hyphens, uppercase) → SHA-256 → `Store.CheckInvite(hash, Now())`. `ErrInviteInvalid` → 401, `ErrInviteExpired` → 410. **Token never appears in either response** (assert body).
   - Empty `agents` → 400.
   - URL: `advertise_url` if set, else `http://<host of r.RemoteAddr>:<port>`; `validateWorkerURL(url, net.LookupIP)` failure → 400. Loopback advertise → 400.
   - Name defaults empty→400 (CLI always sends one). Existing name (case-insensitive) ⇒ update url+agents (idempotent re-join, fresh code). New name ⇒ append.
   - Ordering: `machines.Save` FIRST; on save error → 500 and `CheckInvite` still passes afterward (code not consumed — test asserts). Then `Live.Store`, then `Cluster.Add(name, remote.New(name, url, Tokens.Get()))` (skip Add for the hub's own name), then `MarkInviteUsed`, then `Store.AppendEvent(store.Event{RunID: "", Type: "machine_joined", Data: <json of machine>})`.
   - Response `{"token":"…","name":"mini","url":"http://…"}` (url echoed so the operator sees what got registered).
   - Concurrent double-join with one code: exactly one 200 (the `MarkInviteUsed` guard makes the loser… actually the loser already wrote the registry — acceptable: same machine, idempotent. Test single-use via **sequential** second join → 401.)
4. **remove**: unknown name → 404. Known → `Without`, `Save`, `Live.Store`, `Cluster.Remove`, `AppendEvent("machine_removed")`, respond `{"warning":"<name> still holds the mesh token; rotate it to revoke access (see docs/notes/threat-model.md)"}`. Removing the hub's own entry → 400.

Steps:

- [ ] **Step 6.1:** Write `netcheck_test.go` (table test: loopback/public/RFC1918/CGNAT/ULA IPs; hostname resolving to private vs public via injected `resolve`; `detectHubURL` prefers 100.64/10 via injected `addrs`). Run — FAIL.
- [ ] **Step 6.2:** Implement `netcheck.go` (code above). Run — PASS.
- [ ] **Step 6.3:** Write `meshjoin_test.go` covering every numbered bullet above. Run — FAIL (compile).
- [ ] **Step 6.4:** Implement `token.go` + `meshjoin.go` handlers per contract. Run `go test ./exec/meshjoin/ -race -v` — PASS.
- [ ] **Step 6.5:** `git add exec/meshjoin/ && git commit -m "feat(meshjoin): daemon-side enrollment — invite/join/remove (spec 024)"`

---

### Task 7: roster over Live

**Files:**
- Modify: `control/roster.go` (take `*machines.Live`, read per call)
- Modify: `control/roster_test.go` (if present; adjust constructor)
- Modify: call sites `cmd/fort/main.go:185`, `cmd/fort/control.go:55` (compile fix deferred to Task 8 — this task keeps `control/` green)

- [ ] **Step 7.1: Failing test** (in `control/roster_test.go`):

```go
func TestRosterSeesHotJoin(t *testing.T) {
	var live machines.Live
	r := control.NewRoster(&live)
	if got := r.Machines(); len(got) != 0 {
		t.Fatalf("empty live: %d machines", len(got))
	}
	reg, _ := machines.Parse([]byte("machines:\n  - name: hub\n    url: http://10.0.0.1:4087\n    agents: [claude]\n"), "hub")
	live.Store(reg)
	got := r.Machines()
	if len(got) != 1 || got[0].Name != "hub" || !got[0].Local || !got[0].Reachable {
		t.Fatalf("after hot join: %+v", got)
	}
}
```

- [ ] **Step 7.2:** Run `go test ./control/ -run Roster -v` — FAIL.
- [ ] **Step 7.3:** Change `Roster.reg *machines.Registry` → `live *machines.Live`; `NewRoster(live *machines.Live)`; `Machines()` and `probe()` start with `reg := r.live.Load(); if reg == nil { return nil }` (probe: plain return) then proceed as before. Update `cmd/fort` call sites minimally so the tree compiles (`control.NewRoster(live)` — the `live` var lands properly in Task 8; a temporary `live := &machines.Live{}; live.Store(a.reg)` shim in both cmd files is fine for this commit).
- [ ] **Step 7.4:** `go test ./control/ ./cmd/... -v` — PASS.
- [ ] **Step 7.5:** `git add control/ cmd/fort/ && git commit -m "feat(control): roster reads the live registry pointer (spec 024)"`

---

### Task 8: composition root — wire it all + `fort mesh` CLI

**Files:**
- Create: `cmd/fort/mesh.go`
- Modify: `cmd/fort/wire.go` (Live + always-cluster + managed registry via `config.Load`)
- Modify: `cmd/fort/main.go` (mount meshjoin in serve; usage text; mesh dispatch in `main()`)
- Modify: `cmd/fort/control.go` (use `config.Load` so the managed registry auto-loads)

- [ ] **Step 8.1: wire.go.** Replace `config.FromEnv(os.Getenv)` with `config.Load(os.Getenv)`. Replace the registry block:

```go
	// Multi-machine (spec 022/024): the registry lives behind a Live pointer so
	// mesh enrollment can swap it at runtime. The engine ALWAYS dispatches
	// through a cluster runtime now — with zero remotes it is a pass-through to
	// the local runtime, and enrollment can hot-add peers without re-wiring.
	live := &machines.Live{}
	if cfg.MachinesPath != "" {
		r, err := machines.Load(cfg.MachinesPath, cfg.NodeName)
		if err != nil {
			return nil, err
		}
		live.Store(r)
	}
	clus := cluster.New(localName(live, cfg), localRT, nil)
	if r := live.Load(); r != nil {
		for _, m := range r.Machines {
			if m.Name == r.Local() {
				continue
			}
			clus.Add(m.Name, remote.New(m.Name, m.URL, cfg.NodeToken))
		}
	}
	var rt runtime.Runtime = clus
```

with `func localName(l *machines.Live, cfg config.Config) string { if r := l.Load(); r != nil { return r.Local() }; return cfg.NodeName }`, `eng.UsePlacer(live)` unconditionally, and `app` gaining `live *machines.Live` + `clus *cluster.Runtime` fields (keep `reg` removed — update its two readers in main.go to use `a.live.Load()`).
  **Cluster local-name note:** after a hot join the hub's canonical local name may differ in casing from `cfg.NodeName`; `machines.Live.Store` in the invite path always uses `NodeName` verbatim for the hub entry, so `cluster` local matching stays exact. `handleJoin` never `Add`s the hub's own name.

- [ ] **Step 8.2: main.go serve wiring.** After `uiSrv := ui.New(deps)` build the mesh server and token store:

```go
	tokens := meshjoin.NewTokenStore(a.cfg.NodeToken, a.cfg.DataDir(), a.cfg.NodeName, a.cfg.Addr)
	_, port, _ := net.SplitHostPort(a.cfg.Addr)
	meshSrv := &meshjoin.Server{
		Live: a.live, RegistryPath: managedRegistryPath(a.cfg), Managed: a.cfg.MachinesManaged || a.cfg.MachinesPath == "",
		Cluster: a.clus, Store: a.store, Tokens: tokens, NodeName: a.cfg.NodeName,
		Port: port, ProbeAgents: probeAgents, Now: time.Now, Log: slog.Default(),
	}
	nodeSrv := node.New(a.localRT, tokens.Get)
	mount := func(mux *http.ServeMux) { uiSrv.Register(mux); nodeSrv.Register(mux); meshSrv.Register(mux) }
```

with helpers in mesh.go: `func managedRegistryPath(cfg config.Config) string { if cfg.MachinesManaged || cfg.MachinesPath == "" { return filepath.Join(cfg.DataDir(), "machines.yaml") }; return cfg.MachinesPath }` and `func probeAgents() []string { var out []string; for _, p := range native.DefaultProviders() { if _, err := exec.LookPath(p.Name); err == nil { out = append(out, p.Name) } }; return out }`. Roster: always `roster := control.NewRoster(a.live); go roster.Poll(ctx, 10*time.Second); deps.Machines = roster` (delete the `a.reg != nil` conditional and the Task-7 shim). Startup print: read `a.live.Load()` for machine count.

- [ ] **Step 8.3: mesh.go CLI.** `cmdMesh(args)` dispatching `invite|join|remove`; add `case "mesh": err = cmdMesh(os.Args[2:])` in `main()` and usage lines:

```
  fort mesh invite [--ttl 15m] [--advertise URL]   mint a join code (hub must be running)
  fort mesh join <hub-url> --code C [--name N] [--port 4087] [--agents a,b] [--advertise URL]
  fort mesh remove <name>                          drop a machine from the mesh
```

Implementation contract:
- `invite`: `cfg := config.Load(os.Getenv)`; POST `http://127.0.0.1:<port of cfg.Addr>/api/mesh/invite` body `{"ttl":"...","advertise":"..."}`. Connection-refused → `errors.New("fort serve is not running on this machine — start it first (mesh invite runs inside the daemon)")`. Print `join_cmd`, and when `minted` print: `mesh token created — this hub now also accepts mesh exec requests (see docs/notes/threat-model.md)`.
- `join`: requires positional hub-url + `--code`; agents = `--agents` split on comma, else `probeAgents()`; empty → `errors.New("no agent CLIs found on PATH — pass --agents")` (no request sent). Name default `os.Hostname()`. POST `<hub-url>/api/mesh/join`; on 200 write `config.SaveNodeFile(cfg.DataDir(), config.NodeFile{Name: resp.Name, Token: resp.Token, Addr: fmt.Sprintf("0.0.0.0:%d", port)})`; print registered URL + `start `fort serve` (or restart the service) to begin accepting work`. 401/410 → surface the server message verbatim.
- `remove`: DELETE loopback URL; print the `warning` field from the response.

- [ ] **Step 8.4:** `go build ./... && go test ./... ` — expect PASS (everything compiles, no behavior regressions; single-machine defaults still boot: `FORT_FAKE=1 ./bin/fort serve` smoke check by hand, Ctrl-C).
- [ ] **Step 8.5:** `git add cmd/fort/ && git commit -m "feat(cmd): fort mesh invite|join|remove + spec-024 wiring (live registry, always-cluster)"`

---### Task 9: end-to-end pairing test

**Files:**
- Create: `cmd/fort/mesh_e2e_test.go` (package `main` test — it may use buildApp/serve internals)

- [ ] **Step 9.1: Write the test.** Shape (pin every env var via `t.Setenv`; two temp data dirs A=hub, B=worker; `FORT_FAKE=1`):

```go
func TestMeshPairingEndToEnd(t *testing.T) {
	// hub: buildApp with FORT_DB in dirA, serve its handler on httptest server (loopback).
	// 1. POST /api/mesh/invite (RemoteAddr loopback) → code; assert minted==true,
	//    node.yaml exists in dirA with mode 0600, registry has hub entry.
	// 2. POST /api/mesh/join from a request with RemoteAddr "127.0.0.1:x" and
	//    advertise_url pointing at a second httptest server (worker node endpoint,
	//    fake runtime, token func reading dirB's node.yaml after CLI-equivalent save)
	//    → 200; assert token in body == hub token; registry now 2 machines;
	//    /api/machines lists both; second join with same code → 401.
	//    NOTE: advertise_url must be private — httptest binds 127.0.0.1, which
	//    validateWorkerURL rejects; inject Server.Now/testing seam? NO — instead
	//    run the join with advertise_url "" and RemoteAddr "100.64.0.5:1" then
	//    assert the derived URL; for the DISPATCH leg use a separate direct
	//    cluster.Add with the httptest URL (transport-level, not join-level).
	// 3. engine.Submit a task pinned --machine worker-name; assert the run's
	//    events arrive (fake runtime emits exited) and run.machine == worker name.
	// 4. DELETE /api/mesh/machines/worker → placement to it now errors,
	//    /api/machines drops it.
}
```

Keep it one focused test + helpers; `-race` must pass. The loopback-vs-private constraint above is the fiddly part — the dispatch leg goes through a manually `Add`ed httptest transport, while join-path assertions use the derived-URL response field.

- [ ] **Step 9.2:** `go test ./cmd/... -race -run MeshPairing -v` — iterate until PASS.
- [ ] **Step 9.3:** Full gates: `go test ./... && go test -race ./... && go vet ./...` — all green, including `core/arch_test.go` (meshjoin lives in exec/, imported only by cmd — seam intact).
- [ ] **Step 9.4:** `git add cmd/fort/mesh_e2e_test.go && git commit -m "test(mesh): end-to-end pairing — invite, join, hot dispatch, remove (spec 024)"`

---

### Task 10: docs

**Files:**
- Modify: `README.md` (multi-machine section: lead with `fort mesh`, keep env-var setup as the manual alternative)
- Modify: `machines.example.yaml` (note: "hand-managed alternative — `fort mesh` maintains this for you in `.fort-native/machines.yaml`")
- Modify: `.env.example` (note on node.yaml precedence)
- Modify: `docs/notes/threat-model.md` (mesh token: minted by first invite; hub posture change; **rotation runbook**: stop nodes → delete/edit node.yaml everywhere → `fort mesh invite` + re-join each worker; `mesh remove` is roster-only)
- Modify: `cmd/fort/main.go` usage const (already done in Task 8 — verify)

- [ ] **Step 10.1:** Write the four doc updates. Keep each diff minimal and factual; the README quickstart becomes:

```sh
# hub (laptop)
fort serve &
fort mesh invite            # prints: fort mesh join http://100.x.y.z:4087 --code XXXX-XXXX

# new machine (paste the printed line)
fort mesh join http://100.x.y.z:4087 --code XXXX-XXXX
fort serve
```

- [ ] **Step 10.2:** `go test ./...` one last time; `git add README.md machines.example.yaml .env.example docs/notes/threat-model.md && git commit -m "docs: fort mesh enrollment flow, token rotation runbook (spec 024)"`

---

## Self-review checklist (run after Task 10)

- Every spec-024 test criterion maps to a test: invite lifecycle (T3/T6), no-code oracle (T6.3), re-join update (T6), URL validation (T6.1), ordering/500 (T6), atomic registry + FORT_MACHINES refusal (T1/T6), 0600 incl. temp window (T2), precedence (T2), hub self-entry + placement preference (T6/T9), hot add/remove + /api/machines (T4/T7/T9), empty probe refusal (T8 CLI), loopback admin (T6.1), determinism guard (no Runtime calls in meshjoin — assert by import review + `go list -deps ./exec/meshjoin` contains no model/provider packages), 022 env-only unchanged (full suite green + config precedence tests).
- `fort control` never mounts meshjoin/node endpoints (control.go untouched except `config.Load`).
- Grep the plan's code for drift when implementing: names `Live/WithMachine/Without/Save/NodeFile/ReadNodeFile/SaveNodeFile/Load/CreateInvite/CheckInvite/MarkInviteUsed/Add/Remove/TokenStore.Ensure` are the single vocabulary — do not introduce synonyms.
