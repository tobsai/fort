package graph

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tobsai/fort/core/requestid"
	"github.com/tobsai/fort/core/runtime"
	"github.com/tobsai/fort/core/store"
	"github.com/tobsai/fort/exec/fake"
)

func newExec(t *testing.T) (*Executor, *store.Store, *fake.Runtime) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "fort.db"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	rt := fake.New()
	return NewExecutor(rt, st), st, rt
}

func TestStartPersistsIngressRequestIDBeforeFlowWork(t *testing.T) {
	ex, st, _ := newExec(t)
	const id = "018f3f1c-7d3a-7c1d-a176-9c52c606c6e4"
	ctx := requestid.With(context.Background(), id)
	_, err := ex.Start(ctx, Flow{ID: "trace", Start: "step", Nodes: []Node{{
		ID: "step", Type: Transform, Transform: &TransformSpec{Op: "identity"},
	}}}, "trace-run", "payload")
	if err != nil {
		t.Fatal(err)
	}
	events, err := st.Events("trace-run")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) == 0 || events[0].Type != "ingress" || events[0].Data != `{"request_id":"`+id+`"}` {
		t.Fatalf("events=%+v", events)
	}
}

func nodeStatus(t *testing.T, st *store.Store, runID, nodeID string) (string, bool) {
	t.Helper()
	for _, n := range mustNodeRuns(t, st, runID) {
		if n.NodeID == nodeID {
			return n.Status, true
		}
	}
	return "", false
}

func mustNodeRuns(t *testing.T, st *store.Store, runID string) []store.NodeRun {
	t.Helper()
	ns, err := st.NodeRuns(runID)
	if err != nil {
		t.Fatalf("noderuns: %v", err)
	}
	return ns
}

func TestLinearFlowOnlyTaskInvokesRuntime(t *testing.T) {
	ex, st, rt := newExec(t)
	f := Flow{
		ID: "f1", Start: "t0",
		Nodes: []Node{
			{ID: "t0", Type: Transform, Transform: &TransformSpec{Op: "identity"}, Edges: []Edge{{On: OutAlways, To: "c1"}}},
			{ID: "c1", Type: Check, Check: &CheckSpec{Command: []string{"sh", "-c", "exit 0"}}, Edges: []Edge{{On: OutPass, To: "k1"}}},
			{ID: "k1", Type: Task, Agent: "codex", Prompt: "do work"},
		},
	}
	res, err := ex.Start(context.Background(), f, "run1", "payload")
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if res.State != "completed" {
		t.Fatalf("state = %q, want completed", res.State)
	}
	// Only the task node touched the runtime.
	if d := rt.Dispatched(); len(d) != 1 || d[0].Agent != "codex" {
		t.Errorf("dispatched = %+v, want exactly one codex task", d)
	}
	for _, id := range []string{"t0", "c1", "k1"} {
		if s, ok := nodeStatus(t, st, "run1", id); !ok || s != "succeeded" {
			t.Errorf("node %s status=%q ok=%v, want succeeded", id, s, ok)
		}
	}
}

func TestCheckFailTakesFailEdge(t *testing.T) {
	ex, st, _ := newExec(t)
	f := Flow{
		ID: "f2", Start: "c1",
		Nodes: []Node{
			{ID: "c1", Type: Check, Check: &CheckSpec{Command: []string{"sh", "-c", "exit 1"}},
				Edges: []Edge{{On: OutPass, To: "good"}, {On: OutFail, To: "bad"}}},
			{ID: "good", Type: Task, Agent: "codex"},
			{ID: "bad", Type: Task, Agent: "claude"},
		},
	}
	if _, err := ex.Start(context.Background(), f, "run2", ""); err != nil {
		t.Fatalf("start: %v", err)
	}
	if s, ok := nodeStatus(t, st, "run2", "bad"); !ok || s != "succeeded" {
		t.Errorf("bad branch not taken: status=%q ok=%v", s, ok)
	}
	if _, ok := nodeStatus(t, st, "run2", "good"); ok {
		t.Errorf("good branch should not have run")
	}
}

func TestTransformRecordsOutputAndHash(t *testing.T) {
	ex, st, _ := newExec(t)
	f := Flow{
		ID: "f3", Start: "t0",
		Nodes: []Node{{ID: "t0", Type: Transform, Transform: &TransformSpec{Op: "upper"}}},
	}
	if _, err := ex.Start(context.Background(), f, "run3", "abc"); err != nil {
		t.Fatalf("start: %v", err)
	}
	ns := mustNodeRuns(t, st, "run3")
	if len(ns) != 1 || ns[0].Input != "abc" || ns[0].Output != "ABC" {
		t.Fatalf("transform node = %+v, want in=abc out=ABC", ns)
	}
	// a content hash is recorded as an event
	evs, _ := st.Events("run3")
	foundHash := false
	for _, e := range evs {
		if e.Type == "transform" && len(e.Data) > 0 {
			foundHash = true
		}
	}
	if !foundHash {
		t.Errorf("expected a transform event recording a hash, got %+v", evs)
	}
}

func gateFlow() Flow {
	return Flow{
		ID: "g", Start: "g1",
		Nodes: []Node{
			{ID: "g1", Type: Gate, Edges: []Edge{{On: OutApprove, To: "k1"}, {On: OutReject, To: "k2"}}},
			{ID: "k1", Type: Task, Agent: "codex"},  // approved path
			{ID: "k2", Type: Task, Agent: "claude"}, // rejected path
		},
	}
}

func TestGatePausesAndApproveResumes(t *testing.T) {
	ex, st, _ := newExec(t)
	f := gateFlow()
	res, err := ex.Start(context.Background(), f, "run4", "")
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if res.State != "paused" || res.PausedNode != "g1" {
		t.Fatalf("res = %+v, want paused at g1", res)
	}
	if s, _ := nodeStatus(t, st, "run4", "g1"); s != "waiting" {
		t.Errorf("gate status=%q, want waiting", s)
	}
	if err := ex.Approve("run4", "g1", ""); err != nil {
		t.Fatalf("approve: %v", err)
	}
	res, err = ex.Resume(context.Background(), f, "run4")
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	if res.State != "completed" {
		t.Errorf("state = %q, want completed", res.State)
	}
	if s, _ := nodeStatus(t, st, "run4", "k1"); s != "succeeded" {
		t.Errorf("approved task k1 status=%q", s)
	}
	if _, ok := nodeStatus(t, st, "run4", "k2"); ok {
		t.Errorf("rejected task k2 should not have run")
	}
}

func TestGateRejectTakesRejectEdge(t *testing.T) {
	ex, st, _ := newExec(t)
	f := gateFlow()
	_, _ = ex.Start(context.Background(), f, "run5", "")
	if err := ex.Reject("run5", "g1", ""); err != nil {
		t.Fatalf("reject: %v", err)
	}
	if _, err := ex.Resume(context.Background(), f, "run5"); err != nil {
		t.Fatalf("resume: %v", err)
	}
	if s, _ := nodeStatus(t, st, "run5", "k2"); s != "succeeded" {
		t.Errorf("rejected path k2 status=%q, want succeeded", s)
	}
}

func TestGateDecisionsAppendEvents(t *testing.T) {
	ex, st, _ := newExec(t)
	f := gateFlow()
	if _, err := ex.Start(context.Background(), f, "run9", ""); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := ex.Reject("run9", "g1", "tighten the scope"); err != nil {
		t.Fatalf("reject: %v", err)
	}
	if err := ex.Approve("run9", "g1", "ship it smaller"); err != nil {
		t.Fatalf("approve: %v", err)
	}
	evs, _ := st.Events("run9")
	var gates []store.Event
	for _, e := range evs {
		if e.Type == "gate" {
			gates = append(gates, e)
		}
	}
	if len(gates) != 2 {
		t.Fatalf("want 2 gate events, got %d (%+v)", len(gates), evs)
	}
	if gates[0].NodeID != "g1" || !strings.Contains(gates[0].Data, `"rejected"`) || !strings.Contains(gates[0].Data, "tighten the scope") {
		t.Fatalf("bad reject event: %+v", gates[0])
	}
	if gates[1].NodeID != "g1" || !strings.Contains(gates[1].Data, `"approved"`) || !strings.Contains(gates[1].Data, "ship it smaller") {
		t.Fatalf("bad approve event: %+v", gates[1])
	}
}

func TestGateEditMutatesDownstreamInput(t *testing.T) {
	ex, _, rt := newExec(t)
	f := Flow{
		ID: "ge", Start: "g1",
		Nodes: []Node{
			{ID: "g1", Type: Gate, Edges: []Edge{{On: OutApprove, To: "k1"}}},
			{ID: "k1", Type: Task, Agent: "codex"}, // empty prompt -> uses payload
		},
	}
	_, _ = ex.Start(context.Background(), f, "run6", "original")
	if err := ex.Approve("run6", "g1", "EDITED"); err != nil {
		t.Fatalf("approve: %v", err)
	}
	if _, err := ex.Resume(context.Background(), f, "run6"); err != nil {
		t.Fatalf("resume: %v", err)
	}
	d := rt.Dispatched()
	if len(d) != 1 || d[0].Prompt != "EDITED" {
		t.Errorf("dispatched prompt = %+v, want EDITED", d)
	}
}

func TestRetryThenEscalateToGate(t *testing.T) {
	ex, st, rt := newExec(t)
	rt.ExitCode = 1 // task always fails
	f := Flow{
		ID: "r", Start: "k1",
		Nodes: []Node{
			{ID: "k1", Type: Task, Agent: "codex", Retry: &Retry{Max: 2},
				Edges: []Edge{{On: OutSuccess, To: "done"}, {On: OutFail, To: "esc"}}},
			{ID: "done", Type: Transform, Transform: &TransformSpec{Op: "identity"}},
			{ID: "esc", Type: Gate},
		},
	}
	res, err := ex.Start(context.Background(), f, "run7", "")
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if res.State != "paused" || res.PausedNode != "esc" {
		t.Fatalf("res = %+v, want paused at esc", res)
	}
	// 1 initial + 2 retries = 3 dispatches
	if d := rt.Dispatched(); len(d) != 3 {
		t.Errorf("dispatched %d times, want 3", len(d))
	}
	ns := mustNodeRuns(t, st, "run7")
	for _, n := range ns {
		if n.NodeID == "k1" && n.Attempts != 3 {
			t.Errorf("k1 attempts = %d, want 3", n.Attempts)
		}
	}
}

func TestTaskInvocationsUseDeterministicNodeAttemptRunIDs(t *testing.T) {
	t.Run("stages", func(t *testing.T) {
		ex, st, rt := newExec(t)
		f := Flow{
			ID: "invocation-stages", Start: "plan",
			Nodes: []Node{
				{
					ID: "plan", Type: Task, Agent: "openclaw",
					Edges: []Edge{{On: OutSuccess, To: "build"}},
				},
				{ID: "build", Type: Task, Agent: "openclaw"},
			},
		}

		if _, err := ex.Start(context.Background(), f, "parent-run", "direction"); err != nil {
			t.Fatal(err)
		}
		got := rt.Dispatched()
		if len(got) != 2 {
			t.Fatalf("dispatches = %d, want 2", len(got))
		}
		if got[0].RunID != "parent-run:plan:1" || got[1].RunID != "parent-run:build:1" {
			t.Fatalf("invocation run ids = [%q %q], want node-scoped ids", got[0].RunID, got[1].RunID)
		}
		assertEventsRetainParentRunID(t, st, "parent-run")
	})

	t.Run("retries", func(t *testing.T) {
		ex, st, rt := newExec(t)
		rt.ExitCode = 1
		f := Flow{
			ID: "invocation-retries", Start: "work",
			Nodes: []Node{{
				ID: "work", Type: Task, Agent: "openclaw", Retry: &Retry{Max: 2},
			}},
		}

		if _, err := ex.Start(context.Background(), f, "retry-parent", "direction"); err != nil {
			t.Fatal(err)
		}
		got := rt.Dispatched()
		if len(got) != 3 {
			t.Fatalf("dispatches = %d, want 3", len(got))
		}
		for i, want := range []string{
			"retry-parent:work:1",
			"retry-parent:work:2",
			"retry-parent:work:3",
		} {
			if got[i].RunID != want {
				t.Errorf("dispatch %d run id = %q, want %q", i, got[i].RunID, want)
			}
		}
		assertEventsRetainParentRunID(t, st, "retry-parent")
	})
}

func TestTaskAttemptIsPersistedBeforeDispatch(t *testing.T) {
	ex, _, rt := newExec(t)
	var observed store.NodeRun
	var observedOK bool
	var observedErr error
	var dispatched runtime.RunSpec
	ex.rt = &beforeDispatchRuntime{
		delegate: rt,
		before: func(spec runtime.RunSpec) {
			dispatched = spec
			observed, observedOK, observedErr = ex.nodeRun("claim-parent", "work")
		},
	}
	f := Flow{
		ID: "claim-before-dispatch", Start: "work",
		Nodes: []Node{{
			ID: "work", Type: Task, Agent: "openclaw", Prompt: "do work",
		}},
	}

	if _, err := ex.Start(context.Background(), f, "claim-parent", "direction"); err != nil {
		t.Fatal(err)
	}
	if observedErr != nil {
		t.Fatal(observedErr)
	}
	if !observedOK {
		t.Fatal("node attempt was not persisted before runtime dispatch")
	}
	if observed.Status != "running" || observed.Attempts != 1 {
		t.Fatalf("node during dispatch = %+v, want running attempt 1", observed)
	}
	if dispatched.RunID != "claim-parent:work:1" {
		t.Fatalf("dispatch run id = %q, want claim-parent:work:1", dispatched.RunID)
	}
}

func TestResumeAdvancesFromPersistedRunningTaskAttempt(t *testing.T) {
	ex, st, rt := newExec(t)
	f := Flow{
		ID: "resume-running-attempt", Start: "work",
		Nodes: []Node{{
			ID: "work", Type: Task, Agent: "openclaw",
		}},
	}
	if err := st.CreateRun(store.Run{
		ID: "crash-parent", Title: f.Name, Agent: "flow:" + f.ID,
		Status: "running", FlowID: f.ID,
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.UpsertNodeRun(store.NodeRun{
		ID: nrID("crash-parent", "work"), RunID: "crash-parent",
		NodeID: "work", Type: string(Task), Status: "running",
		Input: "direction", Attempts: 1,
	}); err != nil {
		t.Fatal(err)
	}

	res, err := ex.Resume(context.Background(), f, "crash-parent")
	if err != nil {
		t.Fatal(err)
	}
	if res.State != "completed" {
		t.Fatalf("state = %q, want completed", res.State)
	}
	got := rt.Dispatched()
	if len(got) != 1 {
		t.Fatalf("dispatches = %d, want 1", len(got))
	}
	if got[0].RunID != "crash-parent:work:2" {
		t.Fatalf("dispatch run id = %q, want crash-parent:work:2", got[0].RunID)
	}
	if got[0].Prompt != "direction" {
		t.Fatalf("dispatch prompt = %q, want persisted input direction", got[0].Prompt)
	}
	runs := mustNodeRuns(t, st, "crash-parent")
	if len(runs) != 1 || runs[0].Status != "succeeded" || runs[0].Attempts != 2 {
		t.Fatalf("resumed node = %+v, want succeeded attempt 2", runs)
	}
}

func TestTaskDispatchFailsClosedWhenAttemptCannotBePersisted(t *testing.T) {
	ex, st, rt := newExec(t)
	ex.UsePlacer(&closeStorePlacer{store: st})
	f := Flow{
		ID: "attempt-persist-failure", Start: "work",
		Nodes: []Node{{
			ID: "work", Type: Task, Agent: "openclaw", Context: ContextPlaybook,
		}},
	}

	_, err := ex.Start(context.Background(), f, "persist-failure-parent", "direction")
	if err == nil {
		t.Fatal("start succeeded after the attempt store was closed")
	}
	if !strings.Contains(err.Error(), "persist") {
		t.Fatalf("error = %q, want attempt persistence context", err)
	}
	if got := rt.Dispatched(); len(got) != 0 {
		t.Fatalf("dispatches = %d, want 0 when the attempt claim is not durable", len(got))
	}
}

func assertEventsRetainParentRunID(t *testing.T, st *store.Store, parentRunID string) {
	t.Helper()
	events, err := st.Events(parentRunID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) == 0 {
		t.Fatal("parent run has no persisted events")
	}
	for _, event := range events {
		if event.RunID != parentRunID {
			t.Fatalf("persisted event run id = %q, want parent %q", event.RunID, parentRunID)
		}
	}
	if run, err := st.GetRun(parentRunID); err != nil || run.ID != parentRunID {
		t.Fatalf("parent run = %+v, %v", run, err)
	}
}

func TestResumeAfterRestart(t *testing.T) {
	ex, st, rt := newExec(t)
	f := gateFlow()
	_, _ = ex.Start(context.Background(), f, "run8", "")
	// Simulate a process restart: a fresh executor over the same store.
	ex2 := NewExecutor(rt, st)
	if err := ex2.Approve("run8", "g1", ""); err != nil {
		t.Fatalf("approve: %v", err)
	}
	res, err := ex2.Resume(context.Background(), f, "run8")
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	if res.State != "completed" {
		t.Errorf("state = %q, want completed", res.State)
	}
}

func TestTaskEventsCarryNodeID(t *testing.T) {
	ex, st, _ := newExec(t)
	f := Flow{
		ID: "f-nid", Start: "k1",
		Nodes: []Node{{ID: "k1", Type: Task, Agent: "codex", Prompt: "do work"}},
	}
	if _, err := ex.Start(context.Background(), f, "run-nid", "payload"); err != nil {
		t.Fatalf("start: %v", err)
	}
	evs, _ := st.Events("run-nid")
	if len(evs) == 0 {
		t.Fatal("task node produced no events")
	}
	for _, e := range evs {
		if e.NodeID != "k1" {
			t.Errorf("event %q node_id=%q, want k1", e.Type, e.NodeID)
		}
	}
}

func TestTransformEventHasEmptyNodeID(t *testing.T) {
	ex, st, _ := newExec(t)
	f := Flow{ID: "f-tr", Start: "t0", Nodes: []Node{{ID: "t0", Type: Transform, Transform: &TransformSpec{Op: "upper"}}}}
	if _, err := ex.Start(context.Background(), f, "run-tr", "abc"); err != nil {
		t.Fatalf("start: %v", err)
	}
	evs, _ := st.Events("run-tr")
	for _, e := range evs {
		if e.Type == "transform" && e.NodeID != "" {
			t.Errorf("run-level transform event node_id=%q, want empty", e.NodeID)
		}
	}
}

func TestOrdinaryTaskPromptBehaviorIsUnchanged(t *testing.T) {
	ex, _, rt := newExec(t)
	f := Flow{
		ID: "ordinary-prompt", Start: "task",
		Nodes: []Node{{ID: "task", Type: Task, Agent: "codex", Prompt: "fixed instruction"}},
	}
	if _, err := ex.Start(context.Background(), f, "ordinary-run", "incoming payload"); err != nil {
		t.Fatal(err)
	}
	d := rt.Dispatched()
	if len(d) != 1 || d[0].Prompt != "fixed instruction" {
		t.Fatalf("ordinary prompt = %+v, want the node prompt unchanged", d)
	}
}

func TestPlaybookContextCarriesDirectionApprovedPayloadAndMemory(t *testing.T) {
	ex, _, rt := newExec(t)
	f := Flow{
		ID: "playbook-context", Name: "Playbook", Start: "plan",
		Nodes: []Node{
			{
				ID: "plan", Type: Task, Agent: "hermes", Model: "planner-model",
				Prompt: "Draft the plan.", Context: ContextPlaybook, Memory: true,
				Edges: []Edge{{On: OutSuccess, To: "gate"}},
			},
			{ID: "gate", Type: Gate, Edges: []Edge{{On: OutApprove, To: "build"}}},
			{
				ID: "build", Type: Task, Agent: "codex", Model: "builder-model",
				Prompt: "Implement the plan.", Context: ContextPlaybook,
			},
		},
	}
	res, err := ex.Start(context.Background(), f, "playbook-run", "ORIGINAL DIRECTION")
	if err != nil {
		t.Fatal(err)
	}
	if res.State != "paused" || res.PausedNode != "gate" {
		t.Fatalf("start = %+v", res)
	}
	d := rt.Dispatched()
	if len(d) != 1 || d[0].Model != "planner-model" {
		t.Fatalf("first dispatch = %+v", d)
	}
	firstPrompt := d[0].Prompt
	for _, want := range []string{"Draft the plan.", "Original direction:\nORIGINAL DIRECTION", "Current approved payload:\nORIGINAL DIRECTION"} {
		if !strings.Contains(firstPrompt, want) {
			t.Errorf("first prompt missing %q:\n%s", want, firstPrompt)
		}
	}

	if err := ex.Approve("playbook-run", "gate", "APPROVED PAYLOAD"); err != nil {
		t.Fatal(err)
	}
	if _, err := ex.Resume(context.Background(), f, "playbook-run"); err != nil {
		t.Fatal(err)
	}
	d = rt.Dispatched()
	if len(d) != 2 || d[1].Model != "builder-model" {
		t.Fatalf("dispatches = %+v", d)
	}
	secondPrompt := d[1].Prompt
	for _, want := range []string{
		"Implement the plan.",
		"Original direction:\nORIGINAL DIRECTION",
		"Current approved payload:\nAPPROVED PAYLOAD",
		"Prior memory stage outputs:",
		"[plan]",
		firstPrompt,
	} {
		if !strings.Contains(secondPrompt, want) {
			t.Errorf("second prompt missing %q:\n%s", want, secondPrompt)
		}
	}
}

type recordingPlacer struct {
	machines map[string]string
	err      error
	calls    []string
}

func (p *recordingPlacer) Place(agent, pin string) (string, error) {
	p.calls = append(p.calls, agent+"@"+pin)
	if p.err != nil {
		return "", p.err
	}
	return p.machines[agent], nil
}

type closeStorePlacer struct {
	store *store.Store
}

func (p *closeStorePlacer) Place(string, string) (string, error) {
	return "", p.store.Close()
}

type dispatchErrorRuntime struct {
	err   error
	calls int
}

func (r *dispatchErrorRuntime) Name() string { return "dispatch-error" }

func (r *dispatchErrorRuntime) Dispatch(context.Context, runtime.RunSpec) (runtime.Run, error) {
	r.calls++
	return nil, r.err
}

type completedRuntime struct {
	events []runtime.RunEvent
	status runtime.Status
}

type beforeDispatchRuntime struct {
	delegate runtime.Runtime
	before   func(runtime.RunSpec)
}

func (r *beforeDispatchRuntime) Name() string { return "before-dispatch" }

func (r *beforeDispatchRuntime) Dispatch(ctx context.Context, spec runtime.RunSpec) (runtime.Run, error) {
	r.before(spec)
	return r.delegate.Dispatch(ctx, spec)
}

func (r *completedRuntime) Name() string { return "completed" }

func (r *completedRuntime) Dispatch(_ context.Context, spec runtime.RunSpec) (runtime.Run, error) {
	ch := make(chan runtime.RunEvent, len(r.events))
	for _, event := range r.events {
		event.RunID = spec.RunID
		ch <- event
	}
	close(ch)
	return &completedRun{id: spec.RunID, events: ch, status: r.status}, nil
}

type completedRun struct {
	id     string
	events <-chan runtime.RunEvent
	status runtime.Status
}

func (r *completedRun) ID() string                      { return r.id }
func (r *completedRun) Stream() <-chan runtime.RunEvent { return r.events }
func (r *completedRun) Signal(string) error             { return nil }
func (r *completedRun) Cancel() error                   { return nil }
func (r *completedRun) Status() runtime.Status          { return r.status }
func (r *completedRun) Wait() runtime.Status            { return r.status }

func TestPlaybookTaskUsesDeterministicPlacement(t *testing.T) {
	ex, st, rt := newExec(t)
	placer := &recordingPlacer{machines: map[string]string{"openclaw": "mac-mini"}}
	ex.UsePlacer(placer)
	f := Flow{
		ID: "placed-playbook", Name: "Placed playbook", Start: "design",
		Nodes: []Node{{
			ID: "design", Type: Task, Profile: "openclaw:main", Agent: "openclaw", Context: ContextPlaybook,
		}},
	}

	res, err := ex.Start(context.Background(), f, "placed-run", "direction")
	if err != nil {
		t.Fatal(err)
	}
	if res.State != "completed" {
		t.Fatalf("result = %+v, want completed", res)
	}
	if len(placer.calls) != 1 || placer.calls[0] != "openclaw@" {
		t.Fatalf("placement calls = %v, want [openclaw@]", placer.calls)
	}
	got := rt.Dispatched()
	if len(got) != 1 || got[0].Profile != "openclaw:main" || got[0].Machine != "mac-mini" {
		t.Fatalf("dispatched = %+v, want exact profile on machine mac-mini", got)
	}
	events, err := st.Events("placed-run")
	if err != nil {
		t.Fatal(err)
	}
	var placements []store.Event
	for _, event := range events {
		if event.Type == "placement" {
			placements = append(placements, event)
		}
	}
	if len(placements) != 1 || placements[0].NodeID != "design" ||
		!strings.Contains(placements[0].Data, `"agent":"openclaw"`) ||
		!strings.Contains(placements[0].Data, `"machine":"mac-mini"`) {
		t.Fatalf("placement events = %+v", placements)
	}
}

func TestPlaybookPlacementHappensOnceAcrossRetries(t *testing.T) {
	ex, st, rt := newExec(t)
	rt.ExitCode = 1
	placer := &recordingPlacer{machines: map[string]string{"hermes": "macbook-pro"}}
	ex.UsePlacer(placer)
	f := Flow{
		ID: "retry-playbook", Start: "plan",
		Nodes: []Node{{
			ID: "plan", Type: Task, Agent: "hermes", Context: ContextPlaybook,
			Retry: &Retry{Max: 2},
		}},
	}

	res, err := ex.Start(context.Background(), f, "retry-placed-run", "direction")
	if err != nil {
		t.Fatal(err)
	}
	if res.State != "failed" {
		t.Fatalf("result = %+v, want failed", res)
	}
	if len(placer.calls) != 1 {
		t.Fatalf("placement called %d times, want once: %v", len(placer.calls), placer.calls)
	}
	got := rt.Dispatched()
	if len(got) != 3 {
		t.Fatalf("dispatches = %d, want 3", len(got))
	}
	for i, spec := range got {
		if spec.Machine != "macbook-pro" {
			t.Errorf("dispatch %d machine = %q, want macbook-pro", i, spec.Machine)
		}
	}
	events, _ := st.Events("retry-placed-run")
	placements := 0
	for _, event := range events {
		if event.Type == "placement" {
			placements++
		}
	}
	if placements != 1 {
		t.Fatalf("placement events = %d, want 1 (%+v)", placements, events)
	}
}

func TestTerminalTaskFailurePersistsCauseOnParentRun(t *testing.T) {
	ex, st, rt := newExec(t)
	rt.ExitCode = 9
	f := Flow{
		ID: "terminal-failure", Start: "build",
		Nodes: []Node{{
			ID: "build", Type: Task, Agent: "codex", Context: ContextPlaybook,
		}},
	}

	res, err := ex.Start(context.Background(), f, "terminal-failure-run", "direction")
	if err != nil {
		t.Fatal(err)
	}
	if res.State != "failed" {
		t.Fatalf("result = %+v, want failed", res)
	}
	run, err := st.GetRun("terminal-failure-run")
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != "failed" || run.ExitCode != 9 ||
		!strings.Contains(run.Error, `agent "codex" exited with code 9`) {
		t.Fatalf("run = %+v, want failed/9 with the terminal task cause", run)
	}
	events, err := st.Events(run.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		if event.NodeID == "build" && event.Type == "error" &&
			strings.Contains(event.Data, `agent "codex" exited with code 9`) {
			return
		}
	}
	t.Fatalf("events = %+v, want a node-scoped terminal failure", events)
}

func TestProviderErrorIsNotDuplicatedByGraph(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "fort.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	rt := &completedRuntime{
		events: []runtime.RunEvent{
			{Type: runtime.EventStarted, Data: "codex"},
			{Type: runtime.EventError, Data: "provider model unavailable"},
			{Type: runtime.EventExited, Code: 7},
		},
		status: runtime.Status{State: runtime.StateFailed, ExitCode: 7, Err: "provider model unavailable"},
	}
	ex := NewExecutor(rt, st)
	f := Flow{
		ID: "provider-failure", Start: "work",
		Nodes: []Node{{ID: "work", Type: Task, Agent: "codex"}},
	}

	if _, err := ex.Start(context.Background(), f, "provider-failure-run", "direction"); err != nil {
		t.Fatal(err)
	}
	run, err := st.GetRun("provider-failure-run")
	if err != nil {
		t.Fatal(err)
	}
	if run.ExitCode != 7 || !strings.Contains(run.Error, "provider model unavailable") {
		t.Fatalf("run = %+v, want failed/7 with provider cause", run)
	}
	events, err := st.Events(run.ID)
	if err != nil {
		t.Fatal(err)
	}
	var errors int
	for _, event := range events {
		if event.NodeID == "work" && event.Type == "error" {
			errors++
		}
	}
	if errors != 1 {
		t.Fatalf("error events = %d (%+v), want the provider's single error event", errors, events)
	}
}

func TestRecoverableTaskFailureDoesNotPoisonLaterTerminalFailure(t *testing.T) {
	ex, st, rt := newExec(t)
	rt.ExitCode = 9
	f := Flow{
		ID: "recovery", Start: "work",
		Nodes: []Node{
			{
				ID: "work", Type: Task, Agent: "codex",
				Edges: []Edge{{On: OutFail, To: "verify"}},
			},
			{
				ID: "verify", Type: Check,
				Check: &CheckSpec{Command: []string{"sh", "-c", "exit 1"}},
			},
		},
	}

	if _, err := ex.Start(context.Background(), f, "recovery-run", "direction"); err != nil {
		t.Fatal(err)
	}
	run, err := st.GetRun("recovery-run")
	if err != nil {
		t.Fatal(err)
	}
	if run.ExitCode != 1 || strings.Contains(run.Error, `agent "codex"`) {
		t.Fatalf("run = %+v, want the terminal check failure, not the recovered task failure", run)
	}
}

func TestTerminalStderrIsBounded(t *testing.T) {
	got := retainTerminalStderr("", strings.Repeat("x", terminalStderrLimit+100))
	if len(got) != terminalStderrLimit {
		t.Fatalf("retained stderr bytes = %d, want %d", len(got), terminalStderrLimit)
	}
	if next := retainTerminalStderr(got, "last meaningful line"); next != "last meaningful line" {
		t.Fatalf("next stderr = %q, want last meaningful line", next)
	}
}

func TestStaticFlowStaysLocalWithPlacer(t *testing.T) {
	ex, st, rt := newExec(t)
	placer := &recordingPlacer{machines: map[string]string{"codex": "mac-mini"}}
	ex.UsePlacer(placer)
	f := Flow{
		ID: "static-flow", Start: "build",
		Nodes: []Node{{ID: "build", Type: Task, Agent: "codex"}},
	}

	if _, err := ex.Start(context.Background(), f, "static-run", "payload"); err != nil {
		t.Fatal(err)
	}
	if len(placer.calls) != 0 {
		t.Fatalf("static flow invoked placer: %v", placer.calls)
	}
	got := rt.Dispatched()
	if len(got) != 1 || got[0].Machine != "" {
		t.Fatalf("static dispatch = %+v, want local empty machine", got)
	}
	events, _ := st.Events("static-run")
	for _, event := range events {
		if event.Type == "placement" {
			t.Fatalf("static flow emitted placement event: %+v", event)
		}
	}
}

func TestPlaybookPlacementAndDispatchErrorsAreTerminalAndInspectable(t *testing.T) {
	t.Run("placement", func(t *testing.T) {
		ex, st, rt := newExec(t)
		ex.UsePlacer(&recordingPlacer{err: errors.New("no machine offers hermes")})
		f := Flow{ID: "placement-error", Start: "plan", Nodes: []Node{{
			ID: "plan", Type: Task, Agent: "hermes", Context: ContextPlaybook,
		}}}

		if _, err := ex.Start(context.Background(), f, "placement-error-run", "direction"); err == nil ||
			!strings.Contains(err.Error(), "no machine offers hermes") {
			t.Fatalf("start error = %v", err)
		}
		if len(rt.Dispatched()) != 0 {
			t.Fatalf("placement failure dispatched runtime: %+v", rt.Dispatched())
		}
		assertTerminalGraphFailure(t, st, "placement-error-run", "plan", "no machine offers hermes")
	})

	t.Run("dispatch", func(t *testing.T) {
		st, err := store.Open(filepath.Join(t.TempDir(), "fort.db"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = st.Close() })
		rt := &dispatchErrorRuntime{err: errors.New("remote node unavailable")}
		ex := NewExecutor(rt, st)
		ex.UsePlacer(&recordingPlacer{machines: map[string]string{"openclaw": "mac-mini"}})
		f := Flow{ID: "dispatch-error", Start: "design", Nodes: []Node{{
			ID: "design", Type: Task, Agent: "openclaw", Context: ContextPlaybook,
		}}}

		if _, err := ex.Start(context.Background(), f, "dispatch-error-run", "direction"); err == nil ||
			!strings.Contains(err.Error(), "remote node unavailable") {
			t.Fatalf("start error = %v", err)
		}
		if rt.calls != 1 {
			t.Fatalf("runtime calls = %d, want 1", rt.calls)
		}
		assertTerminalGraphFailure(t, st, "dispatch-error-run", "design", "remote node unavailable")
		events, _ := st.Events("dispatch-error-run")
		if len(events) < 2 || events[0].Type != "placement" || events[0].NodeID != "design" {
			t.Fatalf("dispatch failure events = %+v, want placement before error", events)
		}
	})
}

func assertTerminalGraphFailure(t *testing.T, st *store.Store, runID, nodeID, cause string) {
	t.Helper()
	if status, ok := nodeStatus(t, st, runID, nodeID); !ok || status != "failed" {
		t.Fatalf("node status = %q ok=%v, want failed", status, ok)
	}
	run, err := st.GetRun(runID)
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != "failed" || run.ExitCode != -1 || !strings.Contains(run.Error, cause) {
		t.Fatalf("run = %+v, want failed/-1 error containing %q", run, cause)
	}
	events, err := st.Events(runID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, event := range events {
		if event.Type == "error" && event.NodeID == nodeID && strings.Contains(event.Data, cause) {
			found = true
		}
	}
	if !found {
		t.Fatalf("events = %+v, want node-scoped error containing %q", events, cause)
	}
}
