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

func TestSaveNodeFileTightensStaleTempFile(t *testing.T) {
	dir := t.TempDir()
	// A stale temp file with loose permissions must not leak into node.yaml:
	// O_CREATE only applies the mode when the file is created fresh.
	tmp := filepath.Join(dir, ".node.yaml.tmp")
	if err := os.WriteFile(tmp, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SaveNodeFile(dir, NodeFile{Name: "hub", Token: "sekrit"}); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(filepath.Join(dir, "node.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("node.yaml mode = %o after stale tmp, want 0600", fi.Mode().Perm())
	}
}

func TestLoadCorruptNodeFileIsNonFatal(t *testing.T) {
	dir := t.TempDir()
	db := filepath.Join(dir, "fort.db")
	if err := os.WriteFile(filepath.Join(dir, "node.yaml"), []byte("::: not yaml"), 0o600); err != nil {
		t.Fatal(err)
	}
	c := Load(env(map[string]string{"FORT_DB": db, "FORT_NODE_TOKEN": "envtoken"}))
	if c.NodeToken != "envtoken" || c.Addr != Default().Addr {
		t.Fatalf("corrupt node.yaml must not disturb env/default values: %+v", c)
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
