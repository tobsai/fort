// Package flow loads and validates Fort flow definitions (YAML) into
// graph.Flow values (backlog AO-026/027).
package flow

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tobsai/fort/core/graph"
	"gopkg.in/yaml.v3"
)

// Load parses and validates a single flow definition.
func Load(data []byte) (graph.Flow, error) {
	var f graph.Flow
	if err := yaml.Unmarshal(data, &f); err != nil {
		return graph.Flow{}, fmt.Errorf("flow: invalid YAML: %w", err)
	}
	if err := validate(&f); err != nil {
		return graph.Flow{}, err
	}
	return f, nil
}

// LoadFile loads a flow from a file path.
func LoadFile(path string) (graph.Flow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return graph.Flow{}, err
	}
	f, err := Load(data)
	if err != nil {
		return graph.Flow{}, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	return f, nil
}

// LoadDir loads every *.yaml/*.yml flow in dir, sorted by id.
func LoadDir(dir string) ([]graph.Flow, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var flows []graph.Flow
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}
		f, err := LoadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		flows = append(flows, f)
	}
	sort.Slice(flows, func(i, j int) bool { return flows[i].ID < flows[j].ID })
	return flows, nil
}

func validate(f *graph.Flow) error {
	if f.ID == "" {
		return fmt.Errorf("flow: missing id")
	}
	ids := map[string]bool{}
	for _, n := range f.Nodes {
		if n.ID == "" {
			return fmt.Errorf("flow %s: a node is missing an id", f.ID)
		}
		if ids[n.ID] {
			return fmt.Errorf("flow %s: duplicate node id %q", f.ID, n.ID)
		}
		ids[n.ID] = true
	}
	if f.Start == "" || !ids[f.Start] {
		return fmt.Errorf("flow %s: start %q does not reference a known node", f.ID, f.Start)
	}
	for _, n := range f.Nodes {
		for _, e := range n.Edges {
			if !ids[e.To] {
				return fmt.Errorf("flow %s: node %s has a dangling edge to %q", f.ID, n.ID, e.To)
			}
		}
		if n.Type == graph.Fanout && n.FaninID != "" && !ids[n.FaninID] {
			return fmt.Errorf("flow %s: fanout %s references unknown fanin %q", f.ID, n.ID, n.FaninID)
		}
	}
	return nil
}
