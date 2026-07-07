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
	"github.com/tobsai/fort/core/store"
	"github.com/tobsai/fort/core/task"
)

// Deps are the control-plane collaborators — ports only. With no Runner and a
// queue Dispatcher this serves a full control plane (board, chat, scheduler,
// gate inbox) that needs none of the deterministic execution components.
type Deps struct {
	Dispatcher Dispatcher    // required
	Runner     FlowRunner    // nil in control-only mode
	Store      *store.Store  // required
	FlowIDs    []string      // available flow ids (for chat templates); empty in control-only
	Machines   MachineLister // nil in single-machine mode (spec 022)
	Planner    Planner       // nil in control-only mode (spec 026)
}

// Server holds the ui handlers.
type Server struct {
	d       Deps
	flowIDs map[string]bool
}

// New builds a ui server.
func New(d Deps) *Server {
	set := map[string]bool{}
	for _, id := range d.FlowIDs {
		set[id] = true
	}
	return &Server{d: d, flowIDs: set}
}

// Register mounts the ui routes onto mux.
func (s *Server) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /", s.handlePage)
	mux.HandleFunc("GET /api/board", s.handleBoard)
	mux.HandleFunc("GET /api/summary", s.handleSummary)
	mux.HandleFunc("GET /api/runs/{id}", s.handleRunDetail)
	mux.HandleFunc("GET /api/gates", s.handleGates)
	mux.HandleFunc("GET /api/machines", s.handleMachines)
	mux.HandleFunc("POST /api/gate", s.handleGate)
	mux.HandleFunc("POST /api/chat", s.handleChat)
	mux.HandleFunc("GET /api/backlog", s.handleBacklogList)
	mux.HandleFunc("POST /api/backlog", s.handleBacklogAdd)
	mux.HandleFunc("POST /api/backlog/{id}/dispatch", s.handleBacklogDispatch)
	mux.HandleFunc("DELETE /api/backlog/{id}", s.handleBacklogDelete)
	mux.HandleFunc("POST /api/openclaw", s.handleOpenClaw)
	mux.HandleFunc("GET /api/events", s.handleEvents)
}

// HasExecution reports whether an execution plane is wired (for diagnostics).
func (s *Server) HasExecution() bool { return s.d.Runner != nil }

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
	// Always emit [] (never null) for array fields so strictly-typed clients
	// (the Swift surfaces via FortKit) decode cleanly.
	b := Board{Runs: []RunSummary{}, Gates: []GateItem{}}
	for _, r := range runs {
		b.Runs = append(b.Runs, RunSummary{ID: r.ID, Title: r.Title, Agent: r.Agent, Status: r.Status, Machine: r.Machine, FlowID: r.FlowID})
	}
	for _, g := range gates {
		b.Gates = append(b.Gates, GateItem{RunID: g.RunID, NodeID: g.NodeID, Input: g.Input})
	}
	writeJSON(w, http.StatusOK, b)
}

// handleSummary is the glanceable payload for constrained surfaces (watch
// complication, CarPlay). Counts by status + the pending gates.
func (s *Server) handleSummary(w http.ResponseWriter, _ *http.Request) {
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
	sum := Summary{Total: len(runs), Execution: s.d.Runner != nil, Gates: []GateItem{}}
	for _, r := range runs {
		switch r.Status {
		case "running":
			sum.Running++
		case "queued":
			sum.Queued++
		case "blocked":
			sum.Blocked++
		case "succeeded":
			sum.Succeeded++
		case "failed":
			sum.Failed++
		}
	}
	for i, g := range gates {
		if i >= 10 {
			break
		}
		sum.Gates = append(sum.Gates, GateItem{RunID: g.RunID, NodeID: g.NodeID, Input: g.Input})
	}
	writeJSON(w, http.StatusOK, sum)
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
	d := RunDetail{
		Run:    RunSummary{ID: run.ID, Title: run.Title, Agent: run.Agent, Status: run.Status, Machine: run.Machine, FlowID: run.FlowID},
		Nodes:  []NodeSummary{},
		Events: []Event{},
	}
	for _, n := range nodes {
		d.Nodes = append(d.Nodes, NodeSummary{NodeID: n.NodeID, Type: n.Type, Status: n.Status, Attempts: n.Attempts})
	}
	for _, e := range evs {
		d.Events = append(d.Events, toEvent(e))
	}
	writeJSON(w, http.StatusOK, d)
}

// handleMachines returns the machine roster + reachability (spec 022). Empty in
// single-machine mode (no MachineLister wired).
func (s *Server) handleMachines(w http.ResponseWriter, _ *http.Request) {
	out := []MachineStatus{}
	if s.d.Machines != nil {
		out = s.d.Machines.Machines()
	}
	writeJSON(w, http.StatusOK, out)
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

// handleGate approves/rejects a gate and resumes the flow (AO-035). Requires an
// execution plane; in control-only mode it returns 409.
func (s *Server) handleGate(w http.ResponseWriter, r *http.Request) {
	if s.d.Runner == nil {
		http.Error(w, "no execution plane: gate actions need the deterministic engine", http.StatusConflict)
		return
	}
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
	switch dec.Decision {
	case "approve":
		err = s.d.Runner.Approve(dec.RunID, dec.NodeID, dec.Edit)
	case "reject":
		err = s.d.Runner.Reject(dec.RunID, dec.NodeID)
	default:
		http.Error(w, "decision must be approve|reject", http.StatusBadRequest)
		return
	}
	if err != nil {
		httpError(w, err)
		return
	}
	res, err := s.d.Runner.ResumeFlow(r.Context(), run.FlowID, dec.RunID)
	if err != nil {
		httpError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ActionResult{State: res.State, PausedNode: res.PausedNode})
}

// handleChat boards a task — or, with an execution plane, routes it or (for a
// template trigger like "ship X") instantiates that flow deterministically.
func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Text) == "" {
		http.Error(w, "text is required", http.StatusBadRequest)
		return
	}
	// Flow templates require an execution plane.
	if s.d.Runner != nil {
		if flowID, input, ok := matchFlow(req.Text, s.flowIDs); ok {
			runID := uuid.NewString()
			res, err := s.d.Runner.StartFlow(r.Context(), flowID, runID, input)
			if err != nil {
				httpError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, ChatResult{Kind: "flow", RunID: runID, FlowID: flowID, Paused: res.PausedNode})
			return
		}
	}
	t := task.Task{ID: uuid.NewString(), Title: req.Text, Body: req.Text, Agent: req.Agent, Machine: req.Machine, CreatedAt: time.Now()}
	ref, err := s.d.Dispatcher.Submit(r.Context(), t)
	if err != nil {
		httpError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ChatResult{Kind: "task", RunID: ref.RunID, Route: ref.Route, Machine: ref.Machine, Queued: ref.Queued})
}

func (s *Server) handleBacklogList(w http.ResponseWriter, _ *http.Request) {
	items, err := s.d.Store.ListBacklog()
	if err != nil {
		httpError(w, err)
		return
	}
	out := []BacklogItem{}
	for _, b := range items {
		out = append(out, toBacklogItem(b))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleBacklogAdd(w http.ResponseWriter, r *http.Request) {
	var req BacklogRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Title) == "" {
		http.Error(w, "title is required", http.StatusBadRequest)
		return
	}
	src := req.Source
	if src == "" {
		src = "user"
	}
	b := store.BacklogItem{
		ID: uuid.NewString(), Title: req.Title, Body: req.Body,
		Agent: req.Agent, Machine: req.Machine, Labels: req.Labels,
		Source: src, CreatedAt: time.Now(),
	}
	if err := s.d.Store.CreateBacklogItem(b); err != nil {
		httpError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toBacklogItem(b))
}

func (s *Server) handleBacklogDispatch(w http.ResponseWriter, r *http.Request) {
	b, err := s.d.Store.GetBacklogItem(r.PathValue("id"))
	if err != nil {
		http.Error(w, "backlog item not found", http.StatusNotFound)
		return
	}
	t := task.Task{
		ID: uuid.NewString(), Title: b.Title, Body: b.Body,
		Labels: b.Labels, Agent: b.Agent, Machine: b.Machine, CreatedAt: time.Now(),
	}
	if t.Body == "" {
		t.Body = b.Title
	}
	ref, err := s.d.Dispatcher.Submit(r.Context(), t)
	if err != nil {
		httpError(w, err)
		return
	}
	_ = s.d.Store.DeleteBacklogItem(b.ID)
	writeJSON(w, http.StatusOK, ChatResult{Kind: "task", RunID: ref.RunID, Route: ref.Route, Machine: ref.Machine, Queued: ref.Queued})
}

func (s *Server) handleBacklogDelete(w http.ResponseWriter, r *http.Request) {
	if err := s.d.Store.DeleteBacklogItem(r.PathValue("id")); err != nil {
		httpError(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func toBacklogItem(b store.BacklogItem) BacklogItem {
	return BacklogItem{ID: b.ID, Title: b.Title, Body: b.Body, Agent: b.Agent, Machine: b.Machine, Labels: b.Labels, Source: b.Source}
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
		Labels:    []string{"message"}, // routes via the errand lane -> openclaw
		CreatedAt: time.Now(),
	}
	ref, err := s.d.Dispatcher.Submit(r.Context(), t)
	if err != nil {
		httpError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ChatResult{Kind: "task", RunID: ref.RunID, Route: ref.Route, Machine: ref.Machine, Queued: ref.Queued})
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
func matchFlow(text string, flowIDs map[string]bool) (flowID, input string, ok bool) {
	lower := strings.ToLower(strings.TrimSpace(text))
	if rest, found := strings.CutPrefix(lower, "ship "); found {
		if flowIDs["ship-feature"] {
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
