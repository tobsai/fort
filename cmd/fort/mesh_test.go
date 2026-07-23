package main

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tobsai/fort/core/config"
	"github.com/tobsai/fort/exec/native"
)

// TestManagedRegistryPathManaged: with a managed registry (or single-machine),
// the daemon writes machines.yaml beside the DB in the data dir.
func TestManagedRegistryPathManaged(t *testing.T) {
	dir := t.TempDir()
	db := filepath.Join(dir, "fort.db")

	// Single-machine: no MachinesPath at all → data-dir/machines.yaml.
	cfg := config.Config{DBPath: db}
	if got, want := managedRegistryPath(cfg), filepath.Join(dir, "machines.yaml"); got != want {
		t.Fatalf("single-machine: managedRegistryPath = %q, want %q", got, want)
	}

	// Managed discovery: MachinesPath set to the managed file, MachinesManaged true.
	cfg = config.Config{DBPath: db, MachinesPath: filepath.Join(dir, "machines.yaml"), MachinesManaged: true}
	if got, want := managedRegistryPath(cfg), filepath.Join(dir, "machines.yaml"); got != want {
		t.Fatalf("managed: managedRegistryPath = %q, want %q", got, want)
	}
}

// TestManagedRegistryPathOperatorOverride: an operator FORT_MACHINES path is
// used verbatim (the Managed flag, not this path, gates whether writes happen).
func TestManagedRegistryPathOperatorOverride(t *testing.T) {
	cfg := config.Config{DBPath: "/data/fort.db", MachinesPath: "/etc/fort/op.yaml", MachinesManaged: false}
	if got := managedRegistryPath(cfg); got != "/etc/fort/op.yaml" {
		t.Fatalf("operator override: managedRegistryPath = %q, want /etc/fort/op.yaml", got)
	}
}

// TestProbeAgentsSubsetOfProviders: probeAgents returns only names that are
// also provider names, in provider order, and never errors. (What is actually
// on $PATH varies by machine, so we only assert the subset relationship.)
func TestProbeAgentsSubsetOfProviders(t *testing.T) {
	valid := map[string]bool{}
	for _, p := range native.DefaultProviders() {
		valid[p.Name] = true
	}
	got := probeAgents()
	for _, name := range got {
		if !valid[name] {
			t.Fatalf("probeAgents returned %q, not a known provider name", name)
		}
	}
}

func TestProbeAgentsRejectsInstalledCLIWithDriftedCommandContract(t *testing.T) {
	providers := []native.Provider{
		{Name: "valid", Probe: []string{"valid", "exec", "--help"}},
		{Name: "drifted", Probe: []string{"drifted", "run", "--help"}},
		{Name: "missing", Probe: []string{"missing", "--help"}},
	}
	got := probeAgentProviders(
		providers,
		func(name string) (string, error) {
			if name == "missing" {
				return "", errors.New("not found")
			}
			return "/bin/" + name, nil
		},
		func(p native.Provider) error {
			if p.Name == "drifted" {
				return errors.New("unknown command run")
			}
			return nil
		},
	)
	if len(got) != 1 || got[0] != "valid" {
		t.Fatalf("probeAgentProviders = %v, want [valid]", got)
	}
}

// TestCmdMeshUsageErrors: no subcommand and an unknown subcommand both return a
// usage error (and send no request).
func TestCmdMeshUsageErrors(t *testing.T) {
	if err := cmdMesh(nil); err == nil || !strings.Contains(err.Error(), "usage") {
		t.Fatalf("cmdMesh(nil) = %v, want a usage error", err)
	}
	if err := cmdMesh([]string{"bogus"}); err == nil || !strings.Contains(err.Error(), "usage") {
		t.Fatalf("cmdMesh([bogus]) = %v, want a usage error", err)
	}
}
