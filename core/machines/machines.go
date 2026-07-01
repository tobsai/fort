// Package machines is Fort's static machine registry and deterministic
// placement (spec 022). It is pure core data — it imports neither ui nor any
// exec package — so placement, like routing, takes zero model calls (asserted by
// TestPlacementIsDeterministic). cmd/fort loads the registry and hands it to the
// engine as a Placer and to exec/cluster as the transport map.
package machines

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Machine is one host in the registry: its stable name, the base URL of its
// Fort HTTP API, and the agents (providers) it can run.
type Machine struct {
	Name   string   `yaml:"name" json:"name"`
	URL    string   `yaml:"url" json:"url"`
	Agents []string `yaml:"agents" json:"agents"`
}

// Registry is a parsed machines.yaml plus this machine's identity.
type Registry struct {
	Version  int       `yaml:"version" json:"version"`
	Machines []Machine `yaml:"machines" json:"machines"`
	local    string    // canonical name of the local machine
}

// Parse parses machines.yaml and records localName as this machine's identity.
// localName is matched case-insensitively against the registry and canonicalized
// to the stored casing (so downstream name comparisons are exact).
func Parse(data []byte, localName string) (*Registry, error) {
	var r Registry
	if err := yaml.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("machines: invalid YAML: %w", err)
	}
	if err := r.validate(); err != nil {
		return nil, err
	}
	r.local = localName
	if m, ok := r.Machine(localName); ok {
		r.local = m.Name
	}
	return &r, nil
}

// Load reads and parses machines.yaml from path.
func Load(path, localName string) (*Registry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("machines: %w", err)
	}
	return Parse(data, localName)
}

func (r *Registry) validate() error {
	if len(r.Machines) == 0 {
		return fmt.Errorf("machines: registry lists no machines")
	}
	seen := map[string]bool{}
	for _, m := range r.Machines {
		if m.Name == "" {
			return fmt.Errorf("machines: a machine is missing a name")
		}
		key := strings.ToLower(m.Name)
		if seen[key] {
			return fmt.Errorf("machines: duplicate machine name %q", m.Name)
		}
		seen[key] = true
		if m.URL == "" {
			return fmt.Errorf("machines: machine %q is missing a url", m.Name)
		}
		if len(m.Agents) == 0 {
			return fmt.Errorf("machines: machine %q lists no agents", m.Name)
		}
	}
	return nil
}

// Local returns the canonical name of this machine.
func (r *Registry) Local() string { return r.local }

// Machine returns the machine with the given name (case-insensitive).
func (r *Registry) Machine(name string) (Machine, bool) {
	for _, m := range r.Machines {
		if strings.EqualFold(m.Name, name) {
			return m, true
		}
	}
	return Machine{}, false
}

// Names returns machine names in registry order.
func (r *Registry) Names() []string {
	out := make([]string, len(r.Machines))
	for i, m := range r.Machines {
		out[i] = m.Name
	}
	return out
}

func offers(m Machine, agent string) bool {
	for _, a := range m.Agents {
		if strings.EqualFold(a, agent) {
			return true
		}
	}
	return false
}

// Place chooses, deterministically, the machine that will run agent:
//
//  1. an explicit pin — must exist and offer the agent, else an error;
//  2. otherwise the local machine, if it offers the agent;
//  3. otherwise the first machine in registry order that offers the agent;
//  4. otherwise an error.
//
// It returns the canonical machine name. Place is a pure function of its inputs
// and the registry — no model calls (see TestPlacementIsDeterministic).
func (r *Registry) Place(agent, pin string) (string, error) {
	if pin != "" {
		m, ok := r.Machine(pin)
		if !ok {
			return "", fmt.Errorf("machines: pinned machine %q is not in the registry", pin)
		}
		if !offers(m, agent) {
			return "", fmt.Errorf("machines: machine %q does not offer agent %q", m.Name, agent)
		}
		return m.Name, nil
	}
	if r.local != "" {
		if m, ok := r.Machine(r.local); ok && offers(m, agent) {
			return m.Name, nil
		}
	}
	for _, m := range r.Machines {
		if offers(m, agent) {
			return m.Name, nil
		}
	}
	return "", fmt.Errorf("machines: no machine offers agent %q", agent)
}
