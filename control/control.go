// Package control provides the adapters that plug Fort's deterministic
// components into the control-plane ports (ui.Dispatcher, ui.FlowRunner).
//
// This is the composition seam for "control plane, optionally with execution":
//   - EngineDispatcher / FlowExecutor wrap the real router + DAG engine (full mode).
//   - QueueDispatcher boards tasks with no execution plane at all (control-only mode).
//
// cmd/fort picks which adapters to wire; the ui module only ever sees the ports.
package control

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/tobsai/fort/core/engine"
	"github.com/tobsai/fort/core/graph"
	"github.com/tobsai/fort/core/playbook"
	"github.com/tobsai/fort/core/store"
	"github.com/tobsai/fort/core/task"
	"github.com/tobsai/fort/ui"
)

// EngineDispatcher routes + dispatches via the deterministic engine.
type EngineDispatcher struct{ e *engine.Engine }

// NewEngineDispatcher adapts an engine to ui.Dispatcher.
func NewEngineDispatcher(e *engine.Engine) EngineDispatcher { return EngineDispatcher{e: e} }

// Submit routes the task and starts native execution.
func (d EngineDispatcher) Submit(ctx context.Context, t task.Task) (ui.RunRef, error) {
	dec := d.e.Route(t)
	runID, machine, err := d.e.SubmitRef(ctx, t)
	if err != nil {
		return ui.RunRef{}, err
	}
	return ui.RunRef{RunID: runID, Route: dec.Route, Machine: machine}, nil
}

// QueueDispatcher boards a task as a "queued" run with no execution plane.
type QueueDispatcher struct {
	s     *store.Store
	newID func() string
}

// NewQueueDispatcher adapts a store to ui.Dispatcher for control-only mode.
func NewQueueDispatcher(s *store.Store) QueueDispatcher {
	return QueueDispatcher{s: s, newID: uuid.NewString}
}

// Submit records the task on the board as queued; it is never dispatched.
func (d QueueDispatcher) Submit(_ context.Context, t task.Task) (ui.RunRef, error) {
	runID := d.newID()
	title := t.Title
	if title == "" {
		title = t.ID
	}
	if err := d.s.CreateRun(store.Run{ID: runID, Title: title, Agent: "unassigned", Status: "queued"}); err != nil {
		return ui.RunRef{}, err
	}
	return ui.RunRef{RunID: runID, Queued: true}, nil
}

// FlowExecutor adapts the DAG executor to ui.FlowRunner (id-based).
type FlowExecutor struct {
	x         *graph.Executor
	flows     map[string]graph.Flow
	playbooks *PlaybookCatalog
}

var _ ui.PlaybookRunner = FlowExecutor{}

// NewFlowExecutor adapts a graph executor + flow set to ui.FlowRunner.
func NewFlowExecutor(x *graph.Executor, flows []graph.Flow) FlowExecutor {
	m := make(map[string]graph.Flow, len(flows))
	for _, f := range flows {
		m[f.ID] = f
	}
	return FlowExecutor{x: x, flows: m}
}

// WithPlaybooks enables immutable dynamic playbook flows on the same executor
// value used for static ui.FlowRunner operations.
func (f FlowExecutor) WithPlaybooks(catalog *PlaybookCatalog) FlowExecutor {
	f.playbooks = catalog
	return f
}

func (f FlowExecutor) lookup(id string) (graph.Flow, error) {
	fl, ok := f.flows[id]
	if ok {
		return fl, nil
	}
	metadata, dynamic, err := parsePlaybookFlowID(id)
	if err != nil {
		return graph.Flow{}, err
	}
	if !dynamic || f.playbooks == nil {
		return graph.Flow{}, fmt.Errorf("control: unknown flow %q", id)
	}
	planGate := metadata.planGate
	preview, err := f.playbooks.Route(context.Background(), ui.RouteRequest{
		PlaybookID: metadata.id, PlaybookRevision: metadata.revision,
		TaskType: metadata.taskType, PlanGate: &planGate,
	})
	if err != nil {
		return graph.Flow{}, fmt.Errorf("control: reconstruct flow %q: %w", id, err)
	}
	if preview.Delivery != metadata.delivery {
		return graph.Flow{}, fmt.Errorf("control: flow %q delivery does not match immutable revision", id)
	}
	fl = playbook.Compile(toCoreResolvedRoute(preview))
	fl.ID = id
	return fl, nil
}

// StartFlow starts the named flow.
func (f FlowExecutor) StartFlow(ctx context.Context, flowID, runID, payload string) (ui.RunResult, error) {
	fl, err := f.lookup(flowID)
	if err != nil {
		return ui.RunResult{}, err
	}
	r, err := f.x.Start(ctx, fl, runID, payload)
	return ui.RunResult{State: r.State, PausedNode: r.PausedNode}, err
}

// ResumeFlow resumes the named flow.
func (f FlowExecutor) ResumeFlow(ctx context.Context, flowID, runID string) (ui.RunResult, error) {
	fl, err := f.lookup(flowID)
	if err != nil {
		return ui.RunResult{}, err
	}
	r, err := f.x.Resume(ctx, fl, runID)
	return ui.RunResult{State: r.State, PausedNode: r.PausedNode}, err
}

// Approve records a gate approval.
func (f FlowExecutor) Approve(runID, nodeID, edit string) error {
	return f.x.Approve(runID, nodeID, edit)
}

// Reject records a gate rejection with an optional redirect note.
func (f FlowExecutor) Reject(runID, nodeID, note string) error {
	return f.x.Reject(runID, nodeID, note)
}

// Plan exposes a flow's node list to the control plane (spec 033).
func (f FlowExecutor) Plan(flowID string) []ui.FlowNode {
	fl, err := f.lookup(flowID)
	if err != nil {
		return nil
	}
	out := make([]ui.FlowNode, 0, len(fl.Nodes))
	for _, n := range fl.Nodes {
		out = append(out, ui.FlowNode{ID: n.ID, Type: string(n.Type)})
	}
	return out
}

// StartPlaybook compiles and starts an immutable route preview. The canonical
// flow id carries every field needed to reconstruct the exact flow after a
// process restart, while its delivery suffix lets operational views exclude
// answer-only history.
func (f FlowExecutor) StartPlaybook(ctx context.Context, route ui.RoutePreview, runID, direction string) (ui.PlaybookRunResult, error) {
	if f.playbooks == nil {
		return ui.PlaybookRunResult{}, fmt.Errorf("control: playbook catalog is not configured")
	}
	flowID, err := canonicalPlaybookFlowID(route)
	if err != nil {
		return ui.PlaybookRunResult{}, err
	}
	fl := playbook.Compile(toCoreResolvedRoute(route))
	fl.ID = flowID
	if title := firstNonemptyLine(direction); title != "" {
		fl.Name = title
	}
	result, runErr := f.x.Start(ctx, fl, runID, direction)
	out := ui.PlaybookRunResult{State: result.State, PausedNode: result.PausedNode, FlowID: flowID}
	if route.Delivery == string(playbook.DeliveryAnswer) {
		if runErr != nil {
			return out, runErr
		}
		if result.State != "completed" {
			return out, fmt.Errorf("control: quick answer failed with state %q", result.State)
		}
		answer, err := f.lastTaskOutput(fl, runID)
		if err != nil && runErr == nil {
			runErr = err
		}
		if runErr == nil && strings.TrimSpace(answer) == "" {
			runErr = fmt.Errorf("control: quick answer completed without output")
		}
		out.Answer = answer
	}
	return out, runErr
}

func (f FlowExecutor) lastTaskOutput(fl graph.Flow, runID string) (string, error) {
	nodeRuns, err := f.playbooks.store.NodeRuns(runID)
	if err != nil {
		return "", err
	}
	byID := make(map[string]store.NodeRun, len(nodeRuns))
	for _, nodeRun := range nodeRuns {
		byID[nodeRun.NodeID] = nodeRun
	}
	for i := len(fl.Nodes) - 1; i >= 0; i-- {
		if fl.Nodes[i].Type != graph.Task {
			continue
		}
		if nodeRun, ok := byID[fl.Nodes[i].ID]; ok {
			return nodeRun.Output, nil
		}
	}
	return "", nil
}

type playbookFlowMetadata struct {
	id       string
	revision int
	taskType string
	planGate bool
	delivery string
}

func canonicalPlaybookFlowID(route ui.RoutePreview) (string, error) {
	if route.PlaybookID == "" || route.PlaybookRevision < 1 || route.TaskType == "" {
		return "", fmt.Errorf("control: incomplete playbook route")
	}
	if route.Delivery != string(playbook.DeliveryAnswer) && route.Delivery != string(playbook.DeliveryAssignment) {
		return "", fmt.Errorf("control: invalid playbook delivery %q", route.Delivery)
	}
	gate := "gate0"
	if route.PlanGate {
		gate = "gate1"
	}
	return fmt.Sprintf("playbook:%s:%d:%s:%s:%s",
		url.QueryEscape(route.PlaybookID), route.PlaybookRevision,
		url.QueryEscape(route.TaskType), gate, route.Delivery), nil
}

func parsePlaybookFlowID(flowID string) (playbookFlowMetadata, bool, error) {
	if !strings.HasPrefix(flowID, "playbook:") {
		return playbookFlowMetadata{}, false, nil
	}
	parts := strings.Split(flowID, ":")
	if len(parts) != 6 {
		return playbookFlowMetadata{}, true, fmt.Errorf("control: invalid playbook flow id %q", flowID)
	}
	id, err := url.QueryUnescape(parts[1])
	if err != nil || id == "" {
		return playbookFlowMetadata{}, true, fmt.Errorf("control: invalid playbook flow id %q", flowID)
	}
	revision, err := strconv.Atoi(parts[2])
	if err != nil || revision < 1 {
		return playbookFlowMetadata{}, true, fmt.Errorf("control: invalid playbook flow revision in %q", flowID)
	}
	taskType, err := url.QueryUnescape(parts[3])
	if err != nil || taskType == "" {
		return playbookFlowMetadata{}, true, fmt.Errorf("control: invalid playbook task type in %q", flowID)
	}
	var planGate bool
	switch parts[4] {
	case "gate0":
	case "gate1":
		planGate = true
	default:
		return playbookFlowMetadata{}, true, fmt.Errorf("control: invalid playbook gate mode in %q", flowID)
	}
	if parts[5] != string(playbook.DeliveryAnswer) && parts[5] != string(playbook.DeliveryAssignment) {
		return playbookFlowMetadata{}, true, fmt.Errorf("control: invalid playbook delivery in %q", flowID)
	}
	return playbookFlowMetadata{id: id, revision: revision, taskType: taskType, planGate: planGate, delivery: parts[5]}, true, nil
}

func toCoreResolvedRoute(in ui.RoutePreview) playbook.ResolvedRoute {
	out := playbook.ResolvedRoute{
		PlaybookID: in.PlaybookID, PlaybookRevision: in.PlaybookRevision,
		PlaybookName: in.PlaybookName, TaskType: playbook.TaskType(in.TaskType),
		Source: playbook.RouteSource(in.Source), PlanGate: in.PlanGate,
		Delivery: playbook.Delivery(in.Delivery),
		Stages:   make([]playbook.ResolvedStage, 0, len(in.Stages)),
	}
	for _, stage := range in.Stages {
		out.Stages = append(out.Stages, playbook.ResolvedStage{
			Order: stage.Order, Name: stage.Name, Prompt: stage.Prompt,
			Profile: stage.Profile, Agent: stage.Agent, Model: stage.Model, Memory: stage.Memory,
		})
	}
	return out
}
