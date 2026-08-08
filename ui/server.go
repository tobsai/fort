package ui

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	corecap "github.com/tobsai/fort/core/capability"
	"github.com/tobsai/fort/core/store"
	"github.com/tobsai/fort/core/task"
)

//go:embed fort-icon.png
var fortIcon []byte

//go:embed fort-agent-orb.png
var fortAgentOrb []byte

// Deps are the control-plane collaborators — ports only. With no Runner and a
// queue Dispatcher this serves a full control plane (board, chat, scheduler,
// gate inbox) that needs none of the deterministic execution components.
type Deps struct {
	Dispatcher     Dispatcher                // required
	Runner         FlowRunner                // nil in control-only mode
	Store          *store.Store              // required
	FlowIDs        []string                  // available flow ids (for chat templates); empty in control-only
	Machines       MachineLister             // nil in single-machine mode (spec 022)
	Capabilities   CapabilityLister          // nil until capability inventory is wired (spec 039)
	Planner        Planner                   // nil in control-only mode (spec 026)
	Playbooks      PlaybookCatalog           // deterministic catalog + preview (spec 036)
	PlaybookRunner PlaybookRunner            // nil in control-only mode
	Conversations  ConversationPort          // durable shared conversations (spec 041)
	SeatRechecker  ConversationSeatRechecker // nil without functional capability probes (spec 041)
	Today          TodayPort                 // truthful right-rail projection (spec 041)
	TodayLocation  *time.Location            // one Fort-configured IANA display timezone (spec 041)
	Schedules      SchedulePort              // durable daemon scheduler (spec 041)
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
	mux.HandleFunc("GET /legacy", s.handleLegacyPage)
	mux.HandleFunc("GET /fort-icon.png", s.handleIcon)
	mux.HandleFunc("GET /fort-agent-orb.png", s.handleAgentOrb)
	mux.HandleFunc("GET /api/board", s.handleBoard)
	mux.HandleFunc("GET /api/summary", s.handleSummary)
	mux.HandleFunc("GET /api/runs/{id}", s.handleRunDetail)
	mux.HandleFunc("GET /api/gates", s.handleGates)
	mux.HandleFunc("GET /api/machines", s.handleMachines)
	mux.HandleFunc("GET /api/capabilities", s.handleCapabilities)
	mux.HandleFunc("GET /api/profiles", s.handleProfiles)
	mux.HandleFunc("POST /api/gate", s.handleGate)
	mux.HandleFunc("POST /api/chat", s.handleChat)
	mux.HandleFunc("GET /api/backlog", s.handleBacklogList)
	mux.HandleFunc("POST /api/backlog", s.handleBacklogAdd)
	mux.HandleFunc("POST /api/backlog/{id}/dispatch", s.handleBacklogDispatch)
	mux.HandleFunc("PATCH /api/backlog/{id}", s.handleBacklogPatch)
	mux.HandleFunc("DELETE /api/backlog/{id}", s.handleBacklogDelete)
	mux.HandleFunc("POST /api/breakdown", s.handleBreakdown)
	mux.HandleFunc("GET /api/metrics", s.handleMetrics)
	mux.HandleFunc("GET /api/playbooks", s.handlePlaybooksList)
	mux.HandleFunc("PUT /api/playbooks", s.handlePlaybookSave)
	mux.HandleFunc("POST /api/playbooks/{id}/duplicate", s.handlePlaybookDuplicate)
	mux.HandleFunc("POST /api/route", s.handleRoute)
	mux.HandleFunc("POST /api/openclaw", s.handleOpenClaw)
	mux.HandleFunc("GET /api/events", s.handleEvents)
	mux.HandleFunc("GET /api/conversation-seats", s.handleConversationSeats)
	mux.HandleFunc("POST /api/conversation-seats/recheck", s.handleConversationSeatRecheck)
	mux.HandleFunc("GET /api/today", s.handleToday)
	mux.HandleFunc("POST /api/schedules", s.handleScheduleCreate)
	mux.HandleFunc("GET /api/projects", s.handleProjectsList)
	mux.HandleFunc("POST /api/projects", s.handleProjectCreate)
	mux.HandleFunc("PATCH /api/projects/{id}", s.handleProjectPatch)
	mux.HandleFunc("DELETE /api/projects/{id}", s.handleProjectDelete)
	mux.HandleFunc("GET /api/conversations", s.handleConversationsList)
	mux.HandleFunc("POST /api/conversations", s.handleConversationCreate)
	mux.HandleFunc("GET /api/conversations/{id}", s.handleConversationGet)
	mux.HandleFunc("PATCH /api/conversations/{id}", s.handleConversationPatch)
	mux.HandleFunc("DELETE /api/conversations/{id}", s.handleConversationDelete)
	mux.HandleFunc("POST /api/conversations/{id}/participants", s.handleConversationParticipantAdd)
	mux.HandleFunc("DELETE /api/conversations/{id}/participants/{participant_id}", s.handleConversationParticipantDelete)
	mux.HandleFunc("POST /api/conversations/{id}/turns", s.handleConversationTurn)
	mux.HandleFunc("POST /api/conversations/{id}/targets/{target_id}/retry", s.handleConversationTargetRetry)
	mux.HandleFunc("POST /api/conversations/{id}/targets/{target_id}/cancel", s.handleConversationTargetCancel)
	mux.HandleFunc("GET /api/conversations/{id}/events", s.handleConversationEvents)
	mux.HandleFunc("POST /api/conversation-targets/{id}/retry", s.handleConversationTargetRetry)
	mux.HandleFunc("POST /api/conversation-targets/{id}/cancel", s.handleConversationTargetCancel)
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
	gates = assignmentGates(gates, runs)
	nodesByRun := map[string][]store.NodeRun{}
	if all, err := s.d.Store.AllNodeRuns(); err == nil {
		for _, n := range all {
			nodesByRun[n.RunID] = append(nodesByRun[n.RunID], n)
		}
	}
	// Always emit [] (never null) for array fields so strictly-typed clients
	// (the Swift surfaces via FortKit) decode cleanly.
	b := Board{Runs: []RunSummary{}, Gates: []GateItem{}}
	for _, r := range assignmentRuns(runs) {
		b.Runs = append(b.Runs, RunSummary{
			ID: r.ID, Title: r.Title, Body: r.Body, Agent: r.Agent, Status: r.Status,
			Profile: r.Profile, Model: r.Model, Machine: r.Machine, FlowID: r.FlowID,
			CreatedAt:   r.CreatedAt.UTC().Format(time.RFC3339),
			UpdatedAt:   r.UpdatedAt.UTC().Format(time.RFC3339),
			Checkpoints: s.checkpoints(r.FlowID, nodesByRun[r.ID]),
		})
	}
	for _, g := range gates {
		b.Gates = append(b.Gates, GateItem{RunID: g.RunID, NodeID: g.NodeID, Input: g.Input,
			Since: g.CreatedAt.UTC().Format(time.RFC3339)})
	}
	writeJSON(w, http.StatusOK, b)
}

// checkpoints summarizes a run's human-checkpoint progress (spec 033):
// checkpoints are the flow's gate nodes. The plan (when a FlowRunner is wired
// and knows the flow) supplies the total; otherwise only executed gates count.
// Runs with neither a flow id nor node state have no checkpoints (nil).
func (s *Server) checkpoints(flowID string, nodes []store.NodeRun) *CheckpointSummary {
	if flowID == "" && len(nodes) == 0 {
		return nil
	}
	c := &CheckpointSummary{}
	seen := map[string]bool{}
	for _, n := range nodes {
		seen[n.NodeID] = true
		if n.Type == "gate" {
			c.Total++
			switch n.Status {
			case "approved":
				c.Accepted++
			case "waiting":
				c.Waiting++
			case "rejected":
				c.Rejected++
			}
			continue
		}
		if n.Status == "succeeded" {
			c.Done++
		}
	}
	if s.d.Runner != nil && flowID != "" {
		for _, n := range s.d.Runner.Plan(flowID) {
			if n.Type == "gate" && !seen[n.ID] {
				c.Total++
			}
		}
	}
	return c
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
	gates = assignmentGates(gates, runs)
	runs = assignmentRuns(runs)
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

// assignmentRuns hides answer-delivery playbooks from operational surfaces.
// Their run rows, node state, and events remain intact for direct history and
// replay; the delivery marker is part of the immutable, resumable flow id.
func assignmentRuns(runs []store.Run) []store.Run {
	out := make([]store.Run, 0, len(runs))
	for _, run := range runs {
		if isAnswerRun(run) {
			continue
		}
		out = append(out, run)
	}
	return out
}

func assignmentGates(gates []store.NodeRun, runs []store.Run) []store.NodeRun {
	hidden := make(map[string]bool)
	for _, run := range runs {
		if isAnswerRun(run) {
			hidden[run.ID] = true
		}
	}
	out := make([]store.NodeRun, 0, len(gates))
	for _, gate := range gates {
		if !hidden[gate.RunID] {
			out = append(out, gate)
		}
	}
	return out
}

func isAnswerRun(run store.Run) bool {
	return strings.HasPrefix(run.FlowID, "playbook:") && strings.HasSuffix(run.FlowID, ":answer")
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
		Run: RunSummary{ID: run.ID, Title: run.Title, Body: run.Body, Agent: run.Agent, Profile: run.Profile, Model: run.Model, Status: run.Status, Machine: run.Machine, FlowID: run.FlowID,
			CreatedAt: run.CreatedAt.UTC().Format(time.RFC3339), UpdatedAt: run.UpdatedAt.UTC().Format(time.RFC3339)},
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

func (s *Server) handleCapabilities(w http.ResponseWriter, _ *http.Request) {
	if s.d.Capabilities == nil {
		http.Error(w, "capability inventory unavailable", http.StatusServiceUnavailable)
		return
	}
	snapshot, generation := s.d.Capabilities.Capabilities()
	if generation == 0 {
		http.Error(w, "capability inventory initializing", http.StatusServiceUnavailable)
		return
	}
	if snapshot.Machines == nil {
		snapshot.Machines = []corecap.MachineInventory{}
	}
	writeJSON(w, http.StatusOK, CapabilitiesResponse{Generation: generation, Snapshot: snapshot})
}

func (s *Server) handleProfiles(w http.ResponseWriter, _ *http.Request) {
	catalog := corecap.CatalogV2()
	var snapshot corecap.Snapshot
	var generation uint64
	if s.d.Capabilities != nil {
		snapshot, generation = s.d.Capabilities.Capabilities()
	}
	out := make([]ProfileOption, 0, len(catalog.Profiles))
	for _, definition := range catalog.Profiles {
		agent, model, ok := catalog.RuntimeSelection(definition.ID)
		if !ok {
			continue
		}
		option := ProfileOption{
			ID: definition.ID, Agent: agent, Model: model, DisplayName: definition.DisplayName,
			State: corecap.OfferUnknown, Reason: corecap.ReasonStale, Machines: []string{},
		}
		if generation > 0 {
			var states []corecap.OfferState
			var reasons []corecap.Reason
			for _, machine := range snapshot.Machines {
				for _, offer := range machine.Profiles {
					if offer.ID != definition.ID {
						continue
					}
					states = append(states, offer.State)
					if offer.Reason != "" {
						reasons = append(reasons, offer.Reason)
					}
					if machine.Reachable && offer.State == corecap.OfferReady {
						option.Machines = append(option.Machines, machine.Name)
					}
				}
			}
			option.State, option.Reason = aggregateProfileState(states, reasons)
		}
		out = append(out, option)
	}
	writeJSON(w, http.StatusOK, out)
}

func aggregateProfileState(states []corecap.OfferState, reasons []corecap.Reason) (corecap.OfferState, corecap.Reason) {
	if len(states) == 0 {
		return corecap.OfferUnknown, corecap.ReasonStale
	}
	for _, state := range states {
		if state == corecap.OfferReady {
			return corecap.OfferReady, ""
		}
	}
	for _, state := range states {
		if state == corecap.OfferSetupRequired {
			return corecap.OfferSetupRequired, corecap.FirstReason(reasons...)
		}
	}
	for _, state := range states {
		if state == corecap.OfferUnknown {
			return corecap.OfferUnknown, corecap.FirstReason(reasons...)
		}
	}
	return corecap.OfferUnavailable, corecap.FirstReason(reasons...)
}

func (s *Server) handleGates(w http.ResponseWriter, _ *http.Request) {
	gates, err := s.d.Store.WaitingGates()
	if err != nil {
		httpError(w, err)
		return
	}
	runs, err := s.d.Store.ListRuns()
	if err != nil {
		httpError(w, err)
		return
	}
	gates = assignmentGates(gates, runs)
	out := []GateItem{}
	for _, g := range gates {
		out = append(out, GateItem{RunID: g.RunID, NodeID: g.NodeID, Input: g.Input,
			Since: g.CreatedAt.UTC().Format(time.RFC3339)})
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
		err = s.d.Runner.Reject(dec.RunID, dec.NodeID, dec.Note)
	default:
		http.Error(w, "decision must be approve|reject", http.StatusBadRequest)
		return
	}
	if err != nil {
		httpError(w, err)
		return
	}
	if runner, ok := s.d.Runner.(AcceptedFlowRunner); ok {
		if err := runner.ResumeFlowAsync(r.Context(), run.FlowID, dec.RunID); err != nil {
			httpError(w, err)
			return
		}
		writeAccepted(w, dec.RunID, ActionResult{State: "accepted"})
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
	resolvedModel := ""
	if req.Profile != "" {
		if req.PlaybookID != "" || req.PlaybookRevision != 0 || req.TaskType != "" || req.PlanGate != nil {
			http.Error(w, "profile cannot be combined with a playbook route", http.StatusBadRequest)
			return
		}
		agent, model, ok := corecap.CatalogV2().RuntimeSelection(req.Profile)
		if !ok {
			http.Error(w, "unknown profile", http.StatusBadRequest)
			return
		}
		if req.Agent != "" && req.Agent != agent {
			http.Error(w, "profile does not belong to agent", http.StatusBadRequest)
			return
		}
		req.Agent = agent
		// Model is derived from the closed profile and never accepted directly.
		resolvedModel = model
	}
	if req.PlaybookID != "" || req.PlaybookRevision != 0 || req.TaskType != "" || req.PlanGate != nil {
		s.handlePlaybookChat(w, r, req)
		return
	}
	// The title is the first NON-EMPTY line: skip leading blank/whitespace-only
	// lines so they can't produce an untitled run (the blank-text guard above
	// already rejected fully-blank input).
	text := req.Text
	for {
		i := strings.IndexByte(text, '\n')
		if i < 0 {
			break
		}
		if strings.TrimSpace(text[:i]) != "" {
			break
		}
		text = text[i+1:]
	}
	title, body := text, ""
	if i := strings.IndexByte(text, '\n'); i >= 0 {
		title, body = strings.TrimSpace(text[:i]), strings.TrimSpace(text[i+1:])
	}
	// Flow templates require an execution plane.
	if s.d.Runner != nil && req.Profile == "" {
		if flowID, input, ok := matchFlow(title, s.flowIDs); ok {
			runID := uuid.NewString()
			if runner, ok := s.d.Runner.(AcceptedFlowRunner); ok {
				if _, err := runner.StartFlowAsync(r.Context(), flowID, runID, input); err != nil {
					httpError(w, err)
					return
				}
				writeAccepted(w, runID, ChatResult{
					Kind: "flow", RunID: runID, FlowID: flowID, Accepted: true, Delivery: "assignment",
				})
				return
			}
			res, err := s.d.Runner.StartFlow(r.Context(), flowID, runID, input)
			if err != nil {
				httpError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, ChatResult{Kind: "flow", RunID: runID, FlowID: flowID, Paused: res.PausedNode})
			return
		}
	}
	t := task.Task{ID: uuid.NewString(), Title: title, Body: body, Agent: req.Agent, Profile: req.Profile, Model: resolvedModel, Machine: req.Machine, CreatedAt: time.Now()}
	if dispatcher, ok := s.d.Dispatcher.(AcceptedDispatcher); ok {
		ref, err := dispatcher.Accept(r.Context(), t)
		if err != nil {
			httpError(w, err)
			return
		}
		writeAccepted(w, ref.RunID, ChatResult{
			Kind: "task", RunID: ref.RunID, Accepted: true, Delivery: "assignment",
			Route: ref.Route, Machine: ref.Machine, Queued: ref.Queued,
		})
		return
	}
	ref, err := s.d.Dispatcher.Submit(r.Context(), t)
	if err != nil {
		httpError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ChatResult{Kind: "task", RunID: ref.RunID, Route: ref.Route, Machine: ref.Machine, Queued: ref.Queued})
}

func (s *Server) handlePlaybookChat(w http.ResponseWriter, r *http.Request, req ChatRequest) {
	if s.d.Playbooks == nil {
		http.Error(w, "playbooks are not configured", http.StatusConflict)
		return
	}
	preview, err := s.d.Playbooks.Route(r.Context(), RouteRequest{
		Text: req.Text, PlaybookID: req.PlaybookID, PlaybookRevision: req.PlaybookRevision,
		TaskType: req.TaskType, PlanGate: req.PlanGate,
	})
	if err != nil {
		httpError(w, err)
		return
	}
	if s.d.PlaybookRunner == nil {
		http.Error(w, "no execution plane: playbook handoff needs the deterministic engine", http.StatusConflict)
		return
	}
	runID := uuid.NewString()
	if runner, ok := s.d.PlaybookRunner.(AcceptedPlaybookRunner); ok {
		res, err := runner.StartPlaybookAsync(r.Context(), preview, runID, req.Text)
		if err != nil {
			httpError(w, err)
			return
		}
		kind := "flow"
		if preview.Delivery == "answer" {
			kind = "answer"
		}
		writeAccepted(w, runID, ChatResult{
			Kind: kind, RunID: runID, Accepted: true, Delivery: preview.Delivery, FlowID: res.FlowID,
			PlaybookID: preview.PlaybookID, PlaybookRevision: preview.PlaybookRevision,
		})
		return
	}
	res, err := s.d.PlaybookRunner.StartPlaybook(r.Context(), preview, runID, req.Text)
	if err != nil {
		httpError(w, err)
		return
	}
	kind := "flow"
	if preview.Delivery == "answer" {
		if res.State != "completed" {
			httpError(w, fmt.Errorf("quick answer failed with state %q", res.State))
			return
		}
		if strings.TrimSpace(res.Answer) == "" {
			httpError(w, fmt.Errorf("quick answer completed without output"))
			return
		}
		kind = "answer"
	}
	writeJSON(w, http.StatusOK, ChatResult{
		Kind: kind, RunID: runID, FlowID: res.FlowID, Paused: res.PausedNode, Answer: res.Answer,
		PlaybookID: preview.PlaybookID, PlaybookRevision: preview.PlaybookRevision,
	})
}

func (s *Server) handlePlaybooksList(w http.ResponseWriter, r *http.Request) {
	if s.d.Playbooks == nil {
		writeJSON(w, http.StatusOK, []Playbook{})
		return
	}
	items, err := s.d.Playbooks.List(r.Context())
	if err != nil {
		httpError(w, err)
		return
	}
	if items == nil {
		items = []Playbook{}
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handlePlaybookSave(w http.ResponseWriter, r *http.Request) {
	if s.d.Playbooks == nil {
		http.Error(w, "playbooks are not configured", http.StatusConflict)
		return
	}
	var p Playbook
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	saved, err := s.d.Playbooks.Save(r.Context(), p)
	if err != nil {
		if errors.Is(err, store.ErrPlaybookRevisionStale) {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		httpError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

func (s *Server) handlePlaybookDuplicate(w http.ResponseWriter, r *http.Request) {
	if s.d.Playbooks == nil {
		http.Error(w, "playbooks are not configured", http.StatusConflict)
		return
	}
	p, err := s.d.Playbooks.Duplicate(r.Context(), r.PathValue("id"))
	if err != nil {
		httpError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) handleRoute(w http.ResponseWriter, r *http.Request) {
	if s.d.Playbooks == nil {
		http.Error(w, "playbooks are not configured", http.StatusConflict)
		return
	}
	var req RouteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Text) == "" {
		http.Error(w, "text is required", http.StatusBadRequest)
		return
	}
	preview, err := s.d.Playbooks.Route(r.Context(), req)
	if err != nil {
		httpError(w, err)
		return
	}
	if preview.Stages == nil {
		preview.Stages = []ResolvedPlaybookStage{}
	}
	writeJSON(w, http.StatusOK, preview)
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
	ref, err := s.d.Dispatcher.Submit(r.Context(), t)
	if err != nil {
		httpError(w, err)
		return
	}
	_ = s.d.Store.DeleteBacklogItem(b.ID)
	writeJSON(w, http.StatusOK, ChatResult{Kind: "task", RunID: ref.RunID, Route: ref.Route, Machine: ref.Machine, Queued: ref.Queued})
}

// handleBacklogPatch reassigns an Up-next item to another agent (spec 033,
// Week-view drag between rows).
func (s *Server) handleBacklogPatch(w http.ResponseWriter, r *http.Request) {
	var req BacklogPatch
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	id := r.PathValue("id")
	if err := s.d.Store.UpdateBacklogAgent(id, req.Agent); err != nil {
		http.Error(w, "backlog item not found", http.StatusNotFound)
		return
	}
	b, err := s.d.Store.GetBacklogItem(id)
	if err != nil {
		httpError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toBacklogItem(b))
}

func (s *Server) handleBacklogDelete(w http.ResponseWriter, r *http.Request) {
	if err := s.d.Store.DeleteBacklogItem(r.PathValue("id")); err != nil {
		httpError(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// handleBreakdown runs a planner agent to decompose a goal into backlog
// sub-tasks (spec 026). It returns the visible planner run id immediately; the
// sub-tasks appear in the backlog when that run completes. 409s with no
// execution plane (control-only mode), like gates.
func (s *Server) handleBreakdown(w http.ResponseWriter, r *http.Request) {
	if s.d.Planner == nil {
		http.Error(w, "no execution plane: breakdown needs the engine", http.StatusConflict)
		return
	}
	var req BreakdownRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Text) == "" {
		http.Error(w, "text is required", http.StatusBadRequest)
		return
	}
	runID, err := s.d.Planner.Breakdown(r.Context(), req.Text, req.Agent, req.Machine)
	if err != nil {
		httpError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, BreakdownResult{RunID: runID})
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
	_, _ = w.Write([]byte(conversationPageHTML))
}

func (s *Server) handleLegacyPage(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(boardHTML))
}

func (s *Server) handleIcon(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "image/png")
	_, _ = w.Write(fortIcon)
}

func (s *Server) handleAgentOrb(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "image/png")
	_, _ = w.Write(fortAgentOrb)
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
	return Event{ID: e.ID, RunID: e.RunID, NodeID: e.NodeID, Type: e.Type, Data: e.Data, Code: e.Code, Time: e.CreatedAt.Format(time.RFC3339)}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeAccepted(w http.ResponseWriter, runID string, v any) {
	w.Header().Set("Location", "/api/runs/"+runID)
	writeJSON(w, http.StatusAccepted, v)
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
