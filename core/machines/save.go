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
// directory, then rename. machines.yaml holds no secrets, but CreateTemp
// yields 0600 and rename preserves it — harmless, and consistent with
// node.yaml.
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
