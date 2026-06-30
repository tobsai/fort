package ui

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

	"github.com/tobsai/fort/core/engine"
	"github.com/tobsai/fort/core/flow"
	"github.com/tobsai/fort/core/graph"
	"github.com/tobsai/fort/core/router"
	"github.com/tobsai/fort/core/rules"
	"github.com/tobsai/fort/core/store"
	"github.com/tobsai/fort/exec/fake"
)

func newUI(t *testing.T) (*Server, *store.Store) {
	t.Helper()
	tmp := t.TempDir()
	st, err := store.Open(filepath.Join(tmp, "fort.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	data, err := os.ReadFile(filepath.Join("..", "rules", "v1.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	rs, err := rules.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	rt := fake.New()
	eng := engine.New(router.New(rs), rt, st, tmp)
	ex := graph.NewExecutor(rt, st)
	flows, err := flow.LoadDir(filepath.Join("..", "flows"))
	if err != nil {
		t.Fatal(err)
	}
	return New(Deps{Engine: eng, Exec: ex, Store: st, Flows: flows}), st
}

func do(t *testing.T, s *Server, method, path string, body any) *httptest.ResponseRecorder {
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

func TestChatCreatesRoutedTask(t *testing.T) {
	s, _ := newUI(t)
	rec := do(t, s, "POST", "/api/chat", ChatRequest{Text: "please summarize the repo"})
	if rec.Code != 200 {
		t.Fatalf("code %d: %s", rec.Code, rec.Body)
	}
	var res ChatResult
	_ = json.Unmarshal(rec.Body.Bytes(), &res)
	if res.Kind != "task" || res.Route != "claude" {
		t.Errorf("res = %+v, want task/claude", res)
	}
}

func TestChatShipInstantiatesFlow(t *testing.T) {
	s, _ := newUI(t)
	rec := do(t, s, "POST", "/api/chat", ChatRequest{Text: "ship dark mode toggle"})
	var res ChatResult
	_ = json.Unmarshal(rec.Body.Bytes(), &res)
	if res.Kind != "flow" || res.FlowID != "ship-feature" {
		t.Fatalf("res = %+v, want flow/ship-feature", res)
	}
	if res.Paused != "plan_gate" {
		t.Errorf("paused = %q, want plan_gate", res.Paused)
	}
}

func TestGateDecisionResumesFlow(t *testing.T) {
	s, _ := newUI(t)
	rec := do(t, s, "POST", "/api/chat", ChatRequest{Text: "ship a thing"})
	var started ChatResult
	_ = json.Unmarshal(rec.Body.Bytes(), &started)

	rec = do(t, s, "POST", "/api/gate", GateDecision{RunID: started.RunID, NodeID: "plan_gate", Decision: "approve"})
	if rec.Code != 200 {
		t.Fatalf("gate code %d: %s", rec.Code, rec.Body)
	}
	var ar ActionResult
	_ = json.Unmarshal(rec.Body.Bytes(), &ar)
	if ar.State != "paused" || ar.PausedNode != "merge_gate" {
		t.Errorf("after plan approve = %+v, want paused at merge_gate", ar)
	}
}

func TestBoardListsRunsAndGates(t *testing.T) {
	s, _ := newUI(t)
	_ = do(t, s, "POST", "/api/chat", ChatRequest{Text: "ship something"})
	rec := do(t, s, "GET", "/api/board", nil)
	var b Board
	_ = json.Unmarshal(rec.Body.Bytes(), &b)
	if len(b.Runs) != 1 {
		t.Errorf("runs = %d, want 1", len(b.Runs))
	}
	if len(b.Gates) != 1 || b.Gates[0].NodeID != "plan_gate" {
		t.Errorf("gates = %+v, want [plan_gate]", b.Gates)
	}
}

func TestOpenClawMessageBecomesTask(t *testing.T) {
	s, _ := newUI(t)
	rec := do(t, s, "POST", "/api/openclaw", OpenClawMessage{From: "+15550100", Text: "tell me when the build is done"})
	var res ChatResult
	_ = json.Unmarshal(rec.Body.Bytes(), &res)
	if res.Kind != "task" || res.Route != "openclaw" {
		t.Errorf("res = %+v, want task routed to openclaw", res)
	}
}

func TestEventsSSEReplaysLog(t *testing.T) {
	s, st := newUI(t)
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
		line := sc.Text()
		if strings.HasPrefix(line, "data: ") {
			got = append(got, line)
			if len(got) >= 2 {
				break
			}
		}
	}
	if len(got) < 2 {
		t.Fatalf("got %d data frames, want >=2: %v", len(got), got)
	}
	if !strings.Contains(strings.Join(got, "\n"), "hello world") {
		t.Errorf("SSE frames missing message: %v", got)
	}
}
