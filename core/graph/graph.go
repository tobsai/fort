// Package graph is Fort's deterministic DAG engine (backlog AO-021..025).
//
// A Flow is nodes + conditional edges. Node types: task, gate, check,
// transform, fanout, fanin. Only task nodes invoke the Runtime (inference);
// every other node type is deterministic. Flows are resumable: node state is
// persisted to the store and a paused gate continues after a process restart.
package graph

import "fmt"

// NodeType enumerates the node kinds.
type NodeType string

const (
	Task      NodeType = "task"      // invokes the Runtime (the only inference point)
	Gate      NodeType = "gate"      // halts for human approve/edit/reject
	Check     NodeType = "check"     // deterministic predicate -> pass/fail
	Transform NodeType = "transform" // deterministic data step, records hash
	Fanout    NodeType = "fanout"    // splits into parallel branches
	Fanin     NodeType = "fanin"     // joins branches
)

// ContextMode controls how a task prompt is assembled. The zero value keeps
// the original graph behavior. ContextPlaybook adds the reusable-pipeline
// context contract without changing ordinary flows.
type ContextMode string

const (
	ContextDefault  ContextMode = ""
	ContextPlaybook ContextMode = "playbook"
)

// Outcome is the result label a node produces; edges match on it.
type Outcome string

const (
	OutSuccess Outcome = "success"
	OutFail    Outcome = "fail"
	OutPass    Outcome = "pass"
	OutApprove Outcome = "approve"
	OutReject  Outcome = "reject"
	OutAlways  Outcome = "always"
)

// Edge is a conditional transition: take To when the source node's outcome is On.
type Edge struct {
	On Outcome `yaml:"on" json:"on"`
	To string  `yaml:"to" json:"to"`
}

// Retry configures per-task retry (AO-025).
type Retry struct {
	Max int `yaml:"max" json:"max"` // additional attempts after the first
}

// CheckSpec is a deterministic predicate (AO-022).
type CheckSpec struct {
	// Command is run; exit 0 => pass, non-zero => fail.
	Command []string `yaml:"command" json:"command"`
	// FileExists passes when the named path exists.
	FileExists string `yaml:"file_exists" json:"file_exists"`
}

// TransformSpec is a deterministic data step (AO-024). Ops: identity, upper,
// lower, prefix (Arg prepended), suffix (Arg appended).
type TransformSpec struct {
	Op  string `yaml:"op" json:"op"`
	Arg string `yaml:"arg" json:"arg"`
}

// Node is one DAG node.
type Node struct {
	ID        string         `yaml:"id" json:"id"`
	Type      NodeType       `yaml:"type" json:"type"`
	Profile   string         `yaml:"profile,omitempty" json:"profile,omitempty"`     // task
	Agent     string         `yaml:"agent,omitempty" json:"agent,omitempty"`         // task
	Model     string         `yaml:"model,omitempty" json:"model,omitempty"`         // task
	Prompt    string         `yaml:"prompt,omitempty" json:"prompt,omitempty"`       // task
	Context   ContextMode    `yaml:"context,omitempty" json:"context,omitempty"`     // task
	Memory    bool           `yaml:"memory,omitempty" json:"memory,omitempty"`       // task
	Retry     *Retry         `yaml:"retry,omitempty" json:"retry,omitempty"`         // task
	Check     *CheckSpec     `yaml:"check,omitempty" json:"check,omitempty"`         // check
	Transform *TransformSpec `yaml:"transform,omitempty" json:"transform,omitempty"` // transform
	FaninID   string         `yaml:"fanin,omitempty" json:"fanin,omitempty"`         // fanout -> join node
	Edges     []Edge         `yaml:"edges,omitempty" json:"edges,omitempty"`
}

// Flow is a named DAG.
type Flow struct {
	ID    string `yaml:"id" json:"id"`
	Name  string `yaml:"name" json:"name"`
	Start string `yaml:"start" json:"start"`
	Nodes []Node `yaml:"nodes" json:"nodes"`
}

// Validate checks the deterministic graph shape before execution mutates a run.
func (f Flow) Validate() error {
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
		for _, edge := range n.Edges {
			if !ids[edge.To] {
				return fmt.Errorf("flow %s: node %s has a dangling edge to %q", f.ID, n.ID, edge.To)
			}
		}
		if n.Type == Fanout && n.FaninID != "" && !ids[n.FaninID] {
			return fmt.Errorf("flow %s: fanout %s references unknown fanin %q", f.ID, n.ID, n.FaninID)
		}
	}
	return nil
}

// node returns the node with id, or false.
func (f *Flow) node(id string) (Node, bool) {
	for _, n := range f.Nodes {
		if n.ID == id {
			return n, true
		}
	}
	return Node{}, false
}

// next returns the targets whose edge condition matches outcome (or "always").
func (n *Node) next(outcome Outcome) []string {
	var out []string
	for _, e := range n.Edges {
		if e.On == outcome || e.On == OutAlways {
			out = append(out, e.To)
		}
	}
	return out
}
