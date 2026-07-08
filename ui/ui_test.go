package ui_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tobsai/fort/control"
	"github.com/tobsai/fort/core/engine"
	"github.com/tobsai/fort/core/flow"
	"github.com/tobsai/fort/core/graph"
	"github.com/tobsai/fort/core/router"
	"github.com/tobsai/fort/core/rules"
	"github.com/tobsai/fort/core/store"
	"github.com/tobsai/fort/core/task"
	"github.com/tobsai/fort/exec/fake"
	"github.com/tobsai/fort/ui"
)

func openStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "fort.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func loadFlows(t *testing.T) []graph.Flow {
	t.Helper()
	f, err := flow.LoadDir(filepath.Join("..", "flows"))
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// full mode: execution plane wired (engine dispatcher + flow runner).
func newFullUI(t *testing.T) (*ui.Server, *store.Store) {
	t.Helper()
	st := openStore(t)
	data, err := os.ReadFile(filepath.Join("..", "rules", "v1.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	rs, err := rules.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	rt := fake.New()
	eng := engine.New(router.New(rs), rt, st, t.TempDir())
	flows := loadFlows(t)
	ids := make([]string, len(flows))
	for i, f := range flows {
		ids[i] = f.ID
	}
	return ui.New(ui.Deps{
		Dispatcher: control.NewEngineDispatcher(eng),
		Runner:     control.NewFlowExecutor(graph.NewExecutor(rt, st), flows),
		Store:      st,
		FlowIDs:    ids,
	}), st
}

// control-only mode: no execution plane at all.
func newControlUI(t *testing.T) (*ui.Server, *store.Store) {
	t.Helper()
	st := openStore(t)
	return ui.New(ui.Deps{
		Dispatcher: control.NewQueueDispatcher(st),
		Runner:     nil,
		Store:      st,
	}), st
}

func do(t *testing.T, s *ui.Server, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	s.Register(mux)
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode: %v (%s)", err, rec.Body)
	}
	return v
}

// ---- full mode ----

func TestChatCreatesRoutedTask(t *testing.T) {
	s, _ := newFullUI(t)
	rec := do(t, s, "POST", "/api/chat", ui.ChatRequest{Text: "please summarize the repo"})
	if rec.Code != 200 {
		t.Fatalf("code %d: %s", rec.Code, rec.Body)
	}
	res := decode[ui.ChatResult](t, rec)
	if res.Kind != "task" || res.Route != "claude" || res.Queued {
		t.Errorf("res = %+v, want task/claude not queued", res)
	}
}

func TestChatShipInstantiatesFlow(t *testing.T) {
	s, _ := newFullUI(t)
	rec := do(t, s, "POST", "/api/chat", ui.ChatRequest{Text: "ship dark mode toggle"})
	res := decode[ui.ChatResult](t, rec)
	if res.Kind != "flow" || res.FlowID != "ship-feature" || res.Paused != "plan_gate" {
		t.Fatalf("res = %+v, want flow/ship-feature paused at plan_gate", res)
	}
}

func TestGateDecisionResumesFlow(t *testing.T) {
	s, _ := newFullUI(t)
	rec := do(t, s, "POST", "/api/chat", ui.ChatRequest{Text: "ship a thing"})
	started := decode[ui.ChatResult](t, rec)

	rec = do(t, s, "POST", "/api/gate", ui.GateDecision{RunID: started.RunID, NodeID: "plan_gate", Decision: "approve"})
	if rec.Code != 200 {
		t.Fatalf("gate code %d: %s", rec.Code, rec.Body)
	}
	ar := decode[ui.ActionResult](t, rec)
	if ar.State != "paused" || ar.PausedNode != "merge_gate" {
		t.Errorf("after plan approve = %+v, want paused at merge_gate", ar)
	}
}

func TestBoardListsRunsAndGates(t *testing.T) {
	s, _ := newFullUI(t)
	_ = do(t, s, "POST", "/api/chat", ui.ChatRequest{Text: "ship something"})
	rec := do(t, s, "GET", "/api/board", nil)
	b := decode[ui.Board](t, rec)
	if len(b.Runs) != 1 {
		t.Errorf("runs = %d, want 1", len(b.Runs))
	}
	if len(b.Gates) != 1 || b.Gates[0].NodeID != "plan_gate" {
		t.Errorf("gates = %+v, want [plan_gate]", b.Gates)
	}
}

func TestOpenClawMessageBecomesTask(t *testing.T) {
	s, _ := newFullUI(t)
	rec := do(t, s, "POST", "/api/openclaw", ui.OpenClawMessage{From: "+15550100", Text: "tell me when the build is done"})
	res := decode[ui.ChatResult](t, rec)
	if res.Kind != "task" || res.Route != "openclaw" {
		t.Errorf("res = %+v, want task routed to openclaw", res)
	}
}

// ---- control-only mode ----

func TestControlOnlyChatQueuesTask(t *testing.T) {
	s, st := newControlUI(t)
	rec := do(t, s, "POST", "/api/chat", ui.ChatRequest{Text: "remember to water plants"})
	res := decode[ui.ChatResult](t, rec)
	if res.Kind != "task" || !res.Queued || res.Route != "" {
		t.Fatalf("res = %+v, want a queued task with no route", res)
	}
	run, err := st.GetRun(res.RunID)
	if err != nil || run.Status != "queued" {
		t.Errorf("run = %+v err=%v, want queued", run, err)
	}
}

func TestControlOnlyShipDoesNotRunFlow(t *testing.T) {
	s, _ := newControlUI(t)
	rec := do(t, s, "POST", "/api/chat", ui.ChatRequest{Text: "ship dark mode"})
	res := decode[ui.ChatResult](t, rec)
	// with no execution plane, "ship X" degrades to a boarded task, not a flow.
	if res.Kind != "task" || !res.Queued {
		t.Errorf("res = %+v, want a queued task (no flow without execution)", res)
	}
}

func TestControlOnlyGateReturns409(t *testing.T) {
	s, _ := newControlUI(t)
	rec := do(t, s, "POST", "/api/gate", ui.GateDecision{RunID: "x", NodeID: "g", Decision: "approve"})
	if rec.Code != http.StatusConflict {
		t.Errorf("code = %d, want 409 (no execution plane)", rec.Code)
	}
}

func TestControlOnlyBoardAndSummaryWork(t *testing.T) {
	s, _ := newControlUI(t)
	_ = do(t, s, "POST", "/api/chat", ui.ChatRequest{Text: "task one"})
	_ = do(t, s, "POST", "/api/chat", ui.ChatRequest{Text: "task two"})

	board := decode[ui.Board](t, do(t, s, "GET", "/api/board", nil))
	if len(board.Runs) != 2 {
		t.Errorf("board runs = %d, want 2", len(board.Runs))
	}
	sum := decode[ui.Summary](t, do(t, s, "GET", "/api/summary", nil))
	if sum.Total != 2 || sum.Queued != 2 || sum.Execution {
		t.Errorf("summary = %+v, want total/queued 2 and execution=false", sum)
	}
}

// Array fields must serialize as [] never null, so strictly-typed clients
// (the Swift surfaces) decode cleanly. Regression: FortKit failed on gates:null.
func TestArraysSerializeAsEmptyNotNull(t *testing.T) {
	s, _ := newControlUI(t)
	for _, path := range []string{"/api/summary", "/api/board"} {
		body := do(t, s, "GET", path, nil).Body.String()
		if strings.Contains(body, "null") {
			t.Errorf("%s emitted null for an array (want []): %s", path, strings.TrimSpace(body))
		}
	}
}

// ---- multi-machine (spec 022) ----

// capturingDispatcher records the task it received (to assert wiring).
type capturingDispatcher struct{ last task.Task }

func (d *capturingDispatcher) Submit(_ context.Context, t task.Task) (ui.RunRef, error) {
	d.last = t
	return ui.RunRef{RunID: "cap", Route: t.Agent, Machine: t.Machine}, nil
}

type stubMachines struct{ list []ui.MachineStatus }

func (s stubMachines) Machines() []ui.MachineStatus { return s.list }

func TestChatPinsMachine(t *testing.T) {
	st := openStore(t)
	cd := &capturingDispatcher{}
	s := ui.New(ui.Deps{Dispatcher: cd, Store: st})
	rec := do(t, s, "POST", "/api/chat", ui.ChatRequest{Text: "build it", Machine: "macbook-pro"})
	res := decode[ui.ChatResult](t, rec)
	if cd.last.Machine != "macbook-pro" {
		t.Fatalf("task.Machine = %q, want macbook-pro", cd.last.Machine)
	}
	if res.Machine != "macbook-pro" {
		t.Fatalf("result.Machine = %q, want macbook-pro", res.Machine)
	}
}

func TestChatForcesAgent(t *testing.T) {
	st := openStore(t)
	cd := &capturingDispatcher{}
	s := ui.New(ui.Deps{Dispatcher: cd, Store: st})
	rec := do(t, s, "POST", "/api/chat", ui.ChatRequest{Text: "build it", Agent: "codex"})
	res := decode[ui.ChatResult](t, rec)
	if cd.last.Agent != "codex" {
		t.Fatalf("task.Agent = %q, want codex", cd.last.Agent)
	}
	if res.Route != "codex" {
		t.Fatalf("result.Route = %q, want codex", res.Route)
	}
}

func TestChatSplitsTitleAndBody(t *testing.T) {
	st := openStore(t)
	cd := &capturingDispatcher{}
	s := ui.New(ui.Deps{Dispatcher: cd, Store: st})
	rec := do(t, s, "POST", "/api/chat", ui.ChatRequest{Text: "fix the header\n# Details\n- step one"})
	if rec.Code != 200 {
		t.Fatalf("status=%d", rec.Code)
	}
	if cd.last.Title != "fix the header" || cd.last.Body != "# Details\n- step one" {
		t.Fatalf("title=%q body=%q", cd.last.Title, cd.last.Body)
	}
}

func TestChatSingleLineHasNoBody(t *testing.T) {
	st := openStore(t)
	cd := &capturingDispatcher{}
	s := ui.New(ui.Deps{Dispatcher: cd, Store: st})
	rec := do(t, s, "POST", "/api/chat", ui.ChatRequest{Text: "just a title"})
	if rec.Code != 200 || cd.last.Title != "just a title" || cd.last.Body != "" {
		t.Fatalf("code=%d title=%q body=%q", rec.Code, cd.last.Title, cd.last.Body)
	}
}

func TestChatTitleSkipsLeadingBlankLines(t *testing.T) {
	st := openStore(t)
	cd := &capturingDispatcher{}
	s := ui.New(ui.Deps{Dispatcher: cd, Store: st})
	rec := do(t, s, "POST", "/api/chat", ui.ChatRequest{Text: "\nDo the thing\ndetails"})
	if rec.Code != 200 || cd.last.Title != "Do the thing" || cd.last.Body != "details" {
		t.Fatalf("code=%d title=%q body=%q", rec.Code, cd.last.Title, cd.last.Body)
	}
	// A whitespace-only first line is skipped too.
	rec = do(t, s, "POST", "/api/chat", ui.ChatRequest{Text: "   \nDo the thing"})
	if rec.Code != 200 || cd.last.Title != "Do the thing" || cd.last.Body != "" {
		t.Fatalf("code=%d title=%q body=%q", rec.Code, cd.last.Title, cd.last.Body)
	}
}

func TestMachinesEndpointReturnsRoster(t *testing.T) {
	st := openStore(t)
	roster := stubMachines{list: []ui.MachineStatus{
		{Name: "mac-mini", Agents: []string{"claude", "codex"}, Local: true, Reachable: true},
		{Name: "macbook-pro", Agents: []string{"codex"}, Reachable: false},
	}}
	s := ui.New(ui.Deps{Dispatcher: control.NewQueueDispatcher(st), Store: st, Machines: roster})
	got := decode[[]ui.MachineStatus](t, do(t, s, "GET", "/api/machines", nil))
	if len(got) != 2 || got[0].Name != "mac-mini" || !got[0].Local || got[1].Reachable {
		t.Fatalf("machines = %+v", got)
	}
}

func TestMachinesEmptyWhenSingleMachine(t *testing.T) {
	s, _ := newControlUI(t)
	rec := do(t, s, "GET", "/api/machines", nil)
	if got := decode[[]ui.MachineStatus](t, rec); len(got) != 0 {
		t.Fatalf("want empty roster, got %+v", got)
	}
	if strings.Contains(rec.Body.String(), "null") {
		t.Errorf("machines emitted null (want []): %s", strings.TrimSpace(rec.Body.String()))
	}
}

// ---- shared ----

func TestEventsSSEReplaysLog(t *testing.T) {
	s, st := newControlUI(t)
	_ = st.CreateRun(store.Run{ID: "r1", Agent: "codex", Status: "running"})
	_, _ = st.AppendEvent(store.Event{RunID: "r1", Type: "started", Data: "codex"})
	_, _ = st.AppendEvent(store.Event{RunID: "r1", Type: "message", Data: "hello world"})

	mux := http.NewServeMux()
	s.Register(mux)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL+"/api/events?since=0", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("sse: %v", err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("content-type = %q", ct)
	}
	var got []string
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		if line := sc.Text(); strings.HasPrefix(line, "data: ") {
			got = append(got, line)
			if len(got) >= 2 {
				break
			}
		}
	}
	if len(got) < 2 || !strings.Contains(strings.Join(got, "\n"), "hello world") {
		t.Fatalf("SSE frames = %v", got)
	}
}

func TestRunDetailIncludesNodeID(t *testing.T) {
	s, st := newControlUI(t)
	_ = st.CreateRun(store.Run{ID: "rd1", Agent: "codex", Status: "running"})
	_, _ = st.AppendEvent(store.Event{RunID: "rd1", NodeID: "implement", Type: "message", Data: "hi"})
	_, _ = st.AppendEvent(store.Event{RunID: "rd1", Type: "stdout", Data: "raw"})

	rec := do(t, s, "GET", "/api/runs/rd1", nil)
	if rec.Code != 200 {
		t.Fatalf("status = %d", rec.Code)
	}
	d := decode[ui.RunDetail](t, rec)
	if len(d.Events) != 2 {
		t.Fatalf("events = %d, want 2", len(d.Events))
	}
	if d.Events[0].NodeID != "implement" || d.Events[1].NodeID != "" {
		t.Fatalf("node_ids = %q,%q want implement,\"\"", d.Events[0].NodeID, d.Events[1].NodeID)
	}
}
