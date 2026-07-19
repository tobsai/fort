package graph

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/tobsai/fort/core/runtime"
	"github.com/tobsai/fort/core/store"
)

// Executor runs flows deterministically. Only task nodes invoke the Runtime.
type Executor struct {
	rt    runtime.Runtime
	store *store.Store
}

// NewExecutor builds an executor.
func NewExecutor(rt runtime.Runtime, st *store.Store) *Executor {
	return &Executor{rt: rt, store: st}
}

// Result is the outcome of a Start/Resume call.
type Result struct {
	State      string // completed | failed | paused
	PausedNode string // gate id when paused
}

// terminal node-run statuses that mean "do not re-execute on resume".
func isTerminal(status string) bool {
	switch status {
	case "succeeded", "failed", "approved", "rejected", "skipped":
		return true
	}
	return false
}

// Start runs a flow from its start node with an initial payload.
func (e *Executor) Start(ctx context.Context, f Flow, runID, payload string) (Result, error) {
	_ = e.store.CreateRun(store.Run{ID: runID, Title: f.Name, Agent: "flow:" + f.ID, Status: "running", FlowID: f.ID})
	return e.walkFrom(ctx, f, runID, f.Start, payload, "")
}

// Resume continues a paused flow. It re-walks from the start, replaying
// already-completed nodes from persisted state (no re-execution) until it
// reaches the (now-decided) gate or a still-undecided one.
func (e *Executor) Resume(ctx context.Context, f Flow, runID string) (Result, error) {
	return e.walkFrom(ctx, f, runID, f.Start, "", "")
}

// Approve records an approve decision on a waiting gate, optionally editing the
// payload that flows downstream.
func (e *Executor) Approve(runID, nodeID, edited string) error {
	return e.decideGate(runID, nodeID, "approved", edited, edited)
}

// Reject records a reject decision on a waiting gate. The note is the human's
// redirect note ("" = none); it is recorded in the event log, not the payload.
func (e *Executor) Reject(runID, nodeID, note string) error {
	return e.decideGate(runID, nodeID, "rejected", "", note)
}

func (e *Executor) decideGate(runID, nodeID, status, edited, note string) error {
	nr, ok, err := e.nodeRun(runID, nodeID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("gate %s/%s not found (not waiting)", runID, nodeID)
	}
	out := nr.Input
	if edited != "" {
		out = edited
	}
	if err := e.store.UpsertNodeRun(store.NodeRun{
		ID: nrID(runID, nodeID), RunID: runID, NodeID: nodeID, Type: "gate",
		Status: status, Input: nr.Input, Output: out,
	}); err != nil {
		return err
	}
	// Append-only decision record (spec 033): the node_run upsert keeps only the
	// latest state, so the event row is what preserves decision history.
	data, _ := json.Marshal(map[string]string{"decision": status, "note": note})
	_, _ = e.store.AppendEvent(store.Event{RunID: runID, NodeID: nodeID, Type: "gate", Data: string(data)})
	return nil
}

// walkFrom walks the graph from start, stopping (without executing) when it
// reaches stopAt (used for fanout branches). An empty stopAt walks to the end.
func (e *Executor) walkFrom(ctx context.Context, f Flow, runID, start, payload, stopAt string) (Result, error) {
	cur := start
	for {
		if stopAt != "" && cur == stopAt {
			return Result{State: "reached"}, nil
		}
		node, ok := f.node(cur)
		if !ok {
			return Result{}, fmt.Errorf("flow %s: unknown node %q", f.ID, cur)
		}

		var outcome Outcome
		nr, exists, err := e.nodeRun(runID, cur)
		if err != nil {
			return Result{}, err
		}

		switch {
		case exists && isTerminal(nr.Status):
			// Replay a completed node (resume path) — never re-execute.
			outcome = deriveOutcome(node.Type, nr.Status)
			payload = replayPayload(node.Type, nr, payload)

		case node.Type == Gate:
			// Undecided gate -> pause.
			if !exists {
				_ = e.store.UpsertNodeRun(store.NodeRun{
					ID: nrID(runID, cur), RunID: runID, NodeID: cur, Type: "gate",
					Status: "waiting", Input: payload,
				})
			}
			_ = e.store.UpdateRunStatus(runID, "blocked", 0, "")
			return Result{State: "paused", PausedNode: cur}, nil

		case node.Type == Fanout:
			if r, paused, err := e.runFanout(ctx, f, runID, node, payload); err != nil {
				return Result{}, err
			} else if paused {
				return r, nil
			}
			cur = node.FaninID
			continue

		default:
			outcome, payload, err = e.execute(ctx, f, runID, node, payload)
			if err != nil {
				return Result{}, err
			}
		}

		nexts := node.next(outcome)
		if len(nexts) == 0 {
			return e.finish(runID, outcome), nil
		}
		cur = nexts[0] // single active path
	}
}

// runFanout executes each branch sequentially up to the fanin node.
func (e *Executor) runFanout(ctx context.Context, f Flow, runID string, node Node, payload string) (Result, bool, error) {
	_ = e.store.UpsertNodeRun(store.NodeRun{ID: nrID(runID, node.ID), RunID: runID, NodeID: node.ID, Type: "fanout", Status: "succeeded", Input: payload, Output: payload})
	for _, br := range node.next(OutAlways) {
		r, err := e.walkFrom(ctx, f, runID, br, payload, node.FaninID)
		if err != nil {
			return Result{}, false, err
		}
		if r.State == "paused" {
			return r, true, nil
		}
	}
	return Result{}, false, nil
}

// execute runs a non-gate, non-fanout node and returns its outcome + payload.
func (e *Executor) execute(ctx context.Context, f Flow, runID string, node Node, payload string) (Outcome, string, error) {
	switch node.Type {
	case Task:
		return e.execTask(ctx, runID, node, payload)
	case Check:
		return e.execCheck(runID, node, payload)
	case Transform:
		return e.execTransform(runID, node, payload)
	case Fanin:
		_ = e.store.UpsertNodeRun(store.NodeRun{ID: nrID(runID, node.ID), RunID: runID, NodeID: node.ID, Type: "fanin", Status: "succeeded", Input: payload, Output: payload})
		return OutSuccess, payload, nil
	default:
		return "", payload, fmt.Errorf("flow %s: node %s has unsupported type %q", f.ID, node.ID, node.Type)
	}
}

func (e *Executor) execTask(ctx context.Context, runID string, node Node, payload string) (Outcome, string, error) {
	max := 0
	if node.Retry != nil {
		max = node.Retry.Max
	}
	prompt := node.Prompt
	if prompt == "" {
		prompt = payload
	}
	attempts := 0
	var lastOut string
	for {
		attempts++
		run, err := e.rt.Dispatch(ctx, runtime.RunSpec{RunID: runID, Agent: node.Agent, Prompt: prompt})
		if err != nil {
			return "", payload, err
		}
		var msgs []string
		for ev := range run.Stream() {
			_, _ = e.store.AppendEvent(store.Event{RunID: runID, NodeID: node.ID, Type: string(ev.Type), Data: ev.Data, Code: ev.Code, CreatedAt: ev.Time})
			if ev.Type == runtime.EventMessage {
				msgs = append(msgs, ev.Data)
			}
		}
		st := run.Wait()
		lastOut = strings.Join(msgs, "\n")
		if st.State == runtime.StateSucceeded {
			e.persistNode(runID, node, "succeeded", payload, lastOut, attempts)
			return OutSuccess, lastOut, nil
		}
		if attempts > max {
			e.persistNode(runID, node, "failed", payload, lastOut, attempts)
			return OutFail, lastOut, nil
		}
		// retry
	}
}

func (e *Executor) execCheck(runID string, node Node, payload string) (Outcome, string, error) {
	pass := false
	switch {
	case node.Check == nil:
		pass = true
	case node.Check.FileExists != "":
		_, err := os.Stat(node.Check.FileExists)
		pass = err == nil
	case len(node.Check.Command) > 0:
		cmd := exec.Command(node.Check.Command[0], node.Check.Command[1:]...)
		pass = cmd.Run() == nil
	}
	if pass {
		e.persistNode(runID, node, "succeeded", payload, "", 1)
		return OutPass, payload, nil
	}
	e.persistNode(runID, node, "failed", payload, "", 1)
	return OutFail, payload, nil
}

func (e *Executor) execTransform(runID string, node Node, payload string) (Outcome, string, error) {
	out := applyTransform(node.Transform, payload)
	e.persistNode(runID, node, "succeeded", payload, out, 1)
	inHash := sha256Hex(payload)
	outHash := sha256Hex(out)
	_, _ = e.store.AppendEvent(store.Event{
		RunID: runID, Type: "transform",
		Data: fmt.Sprintf("%s in=%s out=%s", node.ID, inHash, outHash),
	})
	return OutSuccess, out, nil
}

func applyTransform(t *TransformSpec, in string) string {
	if t == nil {
		return in
	}
	switch t.Op {
	case "upper":
		return strings.ToUpper(in)
	case "lower":
		return strings.ToLower(in)
	case "prefix":
		return t.Arg + in
	case "suffix":
		return in + t.Arg
	case "identity", "":
		return in
	default:
		return in
	}
}

func (e *Executor) persistNode(runID string, node Node, status, in, out string, attempts int) {
	_ = e.store.UpsertNodeRun(store.NodeRun{
		ID: nrID(runID, node.ID), RunID: runID, NodeID: node.ID, Type: string(node.Type),
		Status: status, Input: in, Output: out, Attempts: attempts,
	})
}

func (e *Executor) finish(runID string, last Outcome) Result {
	if last == OutFail || last == OutReject {
		_ = e.store.UpdateRunStatus(runID, "failed", 1, "")
		return Result{State: "failed"}
	}
	_ = e.store.UpdateRunStatus(runID, "succeeded", 0, "")
	return Result{State: "completed"}
}

func (e *Executor) nodeRun(runID, nodeID string) (store.NodeRun, bool, error) {
	ns, err := e.store.NodeRuns(runID)
	if err != nil {
		return store.NodeRun{}, false, err
	}
	for _, n := range ns {
		if n.NodeID == nodeID {
			return n, true, nil
		}
	}
	return store.NodeRun{}, false, nil
}

func deriveOutcome(t NodeType, status string) Outcome {
	switch status {
	case "approved":
		return OutApprove
	case "rejected":
		return OutReject
	case "failed":
		return OutFail
	}
	// succeeded
	if t == Check {
		return OutPass
	}
	return OutSuccess
}

// replayPayload reconstructs the payload after a replayed node. Transform, gate,
// and task carry their stored Output forward; check leaves it unchanged.
func replayPayload(t NodeType, nr store.NodeRun, payload string) string {
	switch t {
	case Transform, Gate, Task, Fanout, Fanin:
		if nr.Output != "" {
			return nr.Output
		}
	}
	return payload
}

func nrID(runID, nodeID string) string { return runID + ":" + nodeID }

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", sum[:8])
}
