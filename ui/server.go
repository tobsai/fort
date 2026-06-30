package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tobsai/fort/core/engine"
	"github.com/tobsai/fort/core/graph"
	"github.com/tobsai/fort/core/store"
	"github.com/tobsai/fort/core/task"
)

// Deps are the ui module's collaborators (all from core).
type Deps struct {
	Engine *engine.Engine
	Exec   *graph.Executor
	Store  *store.Store
	Flows  []graph.Flow
}

// Server holds the ui handlers.
type Server struct {
	d     Deps
	flows map[string]graph.Flow
}

// New builds a ui server.
func New(d Deps) *Server {
	m := map[string]graph.Flow{}
	for _, f := range d.Flows {
		m[f.ID] = f
	}
	return &Server{d: d, flows: m}
}

// Register mounts the ui routes onto mux.
func (s *Server) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /", s.handlePage)
	mux.HandleFunc("GET /api/board", s.handleBoard)
	mux.HandleFunc("GET /api/runs/{id}", s.handleRunDetail)
	mux.HandleFunc("GET /api/gates", s.handleGates)
	mux.HandleFunc("POST /api/gate", s.handleGate)
	mux.HandleFunc("POST /api/chat", s.handleChat)
	mux.HandleFunc("POST /api/openclaw", s.handleOpenClaw)
	mux.HandleFunc("GET /api/events", s.handleEvents)
}

func (s *Server) handleBoard(w http.ResponseWriter, _ *http.Request) {
	runs, err := s.d.Store.ListRuns()
	if err != nil {
		httpError(w, err)
		return
	}
	gates, err := s.d.Store.WaitingGates()
	if err != nil {
		httpError(w, err)
		return
	}
	b := Board{}
	for _, r := range runs {
		b.Runs = append(b.Runs, RunSummary{ID: r.ID, Title: r.Title, Agent: r.Agent, Status: r.Status, FlowID: r.FlowID})
	}
	for _, g := range gates {
		b.Gates = append(b.Gates, GateItem{RunID: g.RunID, NodeID: g.NodeID, Input: g.Input})
	}
	writeJSON(w, http.StatusOK, b)
}

func (s *Server) handleRunDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	run, err := s.d.Store.GetRun(id)
	if err != nil {
		http.Error(w, "run not found", http.StatusNotFound)
		return
	}
	nodes, _ := s.d.Store.NodeRuns(id)
	evs, _ := s.d.Store.Events(id)
	d := RunDetail{Run: RunSummary{ID: run.ID, Title: run.Title, Agent: run.Agent, Status: run.Status, FlowID: run.FlowID}}
	for _, n := range nodes {
		d.Nodes = append(d.Nodes, NodeSummary{NodeID: n.NodeID, Type: n.Type, Status: n.Status, Attempts: n.Attempts})
	}
	for _, e := range evs {
		d.Events = append(d.Events, toEvent(e))
	}
	writeJSON(w, http.StatusOK, d)
}

func (s *Server) handleGates(w http.ResponseWriter, _ *http.Request) {
	gates, err := s.d.Store.WaitingGates()
	if err != nil {
		httpError(w, err)
		return
	}
	out := []GateItem{}
	for _, g := range gates {
		out = append(out, GateItem{RunID: g.RunID, NodeID: g.NodeID, Input: g.Input})
	}
	writeJSON(w, http.StatusOK, out)
}

// handleGate approves/rejects a gate and resumes the flow (AO-035).
func (s *Server) handleGate(w http.ResponseWriter, r *http.Request) {
	var dec GateDecision
	if err := json.NewDecoder(r.Body).Decode(&dec); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	run, err := s.d.Store.GetRun(dec.RunID)
	if err != nil {
		http.Error(w, "run not found", http.StatusNotFound)
		return
	}
	f, ok := s.flows[run.FlowID]
	if !ok {
		http.Error(w, "flow not found for run", http.StatusNotFound)
		return
	}
	switch dec.Decision {
	case "approve":
		err = s.d.Exec.Approve(dec.RunID, dec.NodeID, dec.Edit)
	case "reject":
		err = s.d.Exec.Reject(dec.RunID, dec.NodeID)
	default:
		http.Error(w, "decision must be approve|reject", http.StatusBadRequest)
		return
	}
	if err != nil {
		httpError(w, err)
		return
	}
	res, err := s.d.Exec.Resume(r.Context(), f, dec.RunID)
	if err != nil {
		httpError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ActionResult{State: res.State, PausedNode: res.PausedNode})
}

// handleChat creates a task — or, if the message matches a flow template,
// instantiates that flow deterministically (no LLM planner) (AO-034).
func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Text) == "" {
		http.Error(w, "text is required", http.StatusBadRequest)
		return
	}
	if flowID, input, ok := matchFlow(req.Text, s.flows); ok {
		runID := uuid.NewString()
		res, err := s.d.Exec.Start(r.Context(), s.flows[flowID], runID, input)
		if err != nil {
			httpError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, ChatResult{Kind: "flow", RunID: runID, FlowID: flowID, Paused: res.PausedNode})
		return
	}
	t := task.Task{
		ID: uuid.NewString(), Title: req.Text, Body: req.Text,
		Agent: req.Agent, CreatedAt: time.Now(),
	}
	dec := s.d.Engine.Route(t)
	runID, err := s.d.Engine.Submit(r.Context(), t)
	if err != nil {
		httpError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ChatResult{Kind: "task", RunID: runID, Route: dec.Route})
}

// handleOpenClaw maps an inbound OpenClaw message to a Fort task (AO-036).
func (s *Server) handleOpenClaw(w http.ResponseWriter, r *http.Request) {
	var m OpenClawMessage
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil || strings.TrimSpace(m.Text) == "" {
		http.Error(w, "text is required", http.StatusBadRequest)
		return
	}
	t := task.Task{
		ID: uuid.NewString(), Title: m.Text, Body: m.Text,
		Labels: []string{"message"}, // routes via the errand lane -> openclaw
		CreatedAt: time.Now(),
	}
	dec := s.d.Engine.Route(t)
	runID, err := s.d.Engine.Submit(r.Context(), t)
	if err != nil {
		httpError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ChatResult{Kind: "task", RunID: runID, Route: dec.Route})
}

// handleEvents streams the append-only event log as SSE (AO-033). ?since=N
// replays events after cursor N (a run is replayable from 0).
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	var cursor int64
	if v := r.URL.Query().Get("since"); v != "" {
		cursor, _ = strconv.ParseInt(v, 10, 64)
	}
	ctx := r.Context()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		evs, err := s.d.Store.EventsSince(cursor)
		if err == nil {
			for _, e := range evs {
				b, _ := json.Marshal(toEvent(e))
				fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", e.ID, e.Type, b)
				cursor = e.ID
			}
			flusher.Flush()
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Server) handlePage(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(boardHTML))
}

// matchFlow maps a chat message to a flow template (deterministic, not an LLM
// planner): "ship <X>" -> ship-feature with input <X>.
func matchFlow(text string, flows map[string]graph.Flow) (flowID, input string, ok bool) {
	lower := strings.ToLower(strings.TrimSpace(text))
	if rest, found := strings.CutPrefix(lower, "ship "); found {
		if _, has := flows["ship-feature"]; has {
			return "ship-feature", strings.TrimSpace(text[len(text)-len(rest):]), true
		}
	}
	return "", "", false
}

func toEvent(e store.Event) Event {
	return Event{ID: e.ID, RunID: e.RunID, Type: e.Type, Data: e.Data, Code: e.Code, Time: e.CreatedAt.Format(time.RFC3339)}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func httpError(w http.ResponseWriter, err error) {
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

// Run is a convenience for standalone serving (used in tests / embedding).
func (s *Server) Run(ctx context.Context, addr string) error {
	mux := http.NewServeMux()
	s.Register(mux)
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() { <-ctx.Done(); _ = srv.Shutdown(context.Background()) }()
	return srv.ListenAndServe()
}
