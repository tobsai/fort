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
	// O_CREATE applies the 0600 mode only when the file is created fresh; a
	// stale temp file would keep its old (possibly looser) permissions. Remove
	// it first so the mode always holds. The error is ignored — the file
	// usually doesn't exist.
	os.Remove(tmp)
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
