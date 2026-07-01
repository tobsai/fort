// Package ui is Fort's interface module (backlog Phase 3): the event/command
// contract (AO-031), the live board (AO-032), the SSE live-feed transport
// (AO-033), the chat surface (AO-034), the gate inbox (AO-035), and the
// OpenClaw inbound channel (AO-036). It imports core; core never imports ui.
//
// Contract summary (published for clients, incl. the iOS shell, AO-037):
//
//	GET  /api/board                 -> Board (runs + waiting gates)
//	GET  /api/runs/{id}             -> RunDetail (run + nodes + events; replayable)
//	GET  /api/gates                 -> []GateItem
//	POST /api/gate                  <- GateDecision  -> ActionResult
//	POST /api/chat                  <- ChatRequest   -> ChatResult
//	POST /api/openclaw              <- OpenClawMessage-> ChatResult
//	GET  /api/events[?since=N]      -> text/event-stream of Event frames
package ui

// Event is the wire form of one append-only event-log row (the live-feed unit).
type Event struct {
	ID    int64  `json:"id"`
	RunID string `json:"run_id"`
	Type  string `json:"type"`
	Data  string `json:"data,omitempty"`
	Code  int    `json:"code,omitempty"`
	Time  string `json:"time"`
}

// RunSummary is a board card.
type RunSummary struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Agent   string `json:"agent"`
	Status  string `json:"status"`
	Machine string `json:"machine,omitempty"` // host the run is placed on (spec 022)
	FlowID  string `json:"flow_id,omitempty"`
}

// MachineStatus is one host in the roster (GET /api/machines, spec 022).
type MachineStatus struct {
	Name      string   `json:"name"`
	URL       string   `json:"url,omitempty"`
	Agents    []string `json:"agents"`
	Local     bool     `json:"local"`
	Reachable bool     `json:"reachable"`
}

// NodeSummary is a node's state within a run.
type NodeSummary struct {
	NodeID   string `json:"node_id"`
	Type     string `json:"type"`
	Status   string `json:"status"`
	Attempts int    `json:"attempts,omitempty"`
}

// GateItem is a gate awaiting a human decision (the gate inbox).
type GateItem struct {
	RunID  string `json:"run_id"`
	NodeID string `json:"node_id"`
	Input  string `json:"input,omitempty"`
}

// Board is the live board payload.
type Board struct {
	Runs  []RunSummary `json:"runs"`
	Gates []GateItem   `json:"gates"`
}

// RunDetail makes a run replayable from the event log.
type RunDetail struct {
	Run    RunSummary    `json:"run"`
	Nodes  []NodeSummary `json:"nodes"`
	Events []Event       `json:"events"`
}

// GateDecision is the command body for POST /api/gate.
type GateDecision struct {
	RunID    string `json:"run_id"`
	NodeID   string `json:"node_id"`
	Decision string `json:"decision"` // approve | reject
	Edit     string `json:"edit,omitempty"`
}

// ChatRequest is the command body for POST /api/chat.
type ChatRequest struct {
	Text    string `json:"text"`
	Agent   string `json:"agent,omitempty"`   // force a specific agent
	Machine string `json:"machine,omitempty"` // pin a target host (spec 022)
}

// ChatResult is the response for chat/openclaw.
type ChatResult struct {
	Kind    string `json:"kind"` // task | flow
	RunID   string `json:"run_id"`
	Route   string `json:"route,omitempty"`   // agent, for task kind (execution plane)
	Machine string `json:"machine,omitempty"` // resolved host (spec 022)
	Queued  bool   `json:"queued,omitempty"`  // true when only boarded (control-only)
	FlowID  string `json:"flow_id,omitempty"` // for flow kind
	Paused  string `json:"paused,omitempty"`  // gate id if the flow paused
}

// Summary is the glanceable control-plane snapshot for constrained surfaces
// (watch complication, CarPlay). Served at GET /api/summary.
type Summary struct {
	Total     int        `json:"total"`
	Running   int        `json:"running"`
	Queued    int        `json:"queued"`
	Blocked   int        `json:"blocked"` // paused at a gate
	Succeeded int        `json:"succeeded"`
	Failed    int        `json:"failed"`
	Execution bool       `json:"execution"` // whether an execution plane is attached
	Gates     []GateItem `json:"gates"`
}

// OpenClawMessage is an inbound OpenClaw message (AO-036).
type OpenClawMessage struct {
	From string `json:"from"`
	Text string `json:"text"`
}

// ActionResult is a generic command result (gate decisions).
type ActionResult struct {
	State      string `json:"state"`
	PausedNode string `json:"paused_node,omitempty"`
}
