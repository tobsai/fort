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
	ID     int64  `json:"id"`
	RunID  string `json:"run_id"`
	NodeID string `json:"node_id,omitempty"`
	Type   string `json:"type"`
	Data   string `json:"data,omitempty"`
	Code   int    `json:"code,omitempty"`
	Time   string `json:"time"`
}

// RunSummary is a board card.
type RunSummary struct {
	ID          string             `json:"id"`
	Title       string             `json:"title"`
	Body        string             `json:"body,omitempty"`
	Agent       string             `json:"agent"`
	Status      string             `json:"status"`
	Machine     string             `json:"machine,omitempty"` // host the run is placed on (spec 022)
	FlowID      string             `json:"flow_id,omitempty"`
	CreatedAt   string             `json:"created_at,omitempty"` // RFC3339 (spec 033)
	UpdatedAt   string             `json:"updated_at,omitempty"` // RFC3339 (spec 033)
	Checkpoints *CheckpointSummary `json:"checkpoints,omitempty"`
}

// CheckpointSummary is a run's human-checkpoint progress: checkpoints are the
// flow's gate nodes — progress is what the human accepted, never an agent
// estimate (spec 033).
type CheckpointSummary struct {
	Total    int `json:"total"`    // gate nodes in the plan (executed-only when no plan is known)
	Accepted int `json:"accepted"` // approved gates
	Waiting  int `json:"waiting"`  // gates awaiting sign-off
	Rejected int `json:"rejected"` // rejected gates
	Done     int `json:"done"`     // non-gate nodes finished (for in-progress inference)
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
	Since  string `json:"since,omitempty"` // RFC3339 — when the gate began waiting (spec 033)
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
	Note     string `json:"note,omitempty"` // redirect note on reject (spec 033)
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

// BacklogItem is a pending task queued on the board (spec 025).
type BacklogItem struct {
	ID      string   `json:"id"`
	Title   string   `json:"title"`
	Body    string   `json:"body,omitempty"`
	Agent   string   `json:"agent,omitempty"`
	Machine string   `json:"machine,omitempty"`
	Labels  []string `json:"labels,omitempty"`
	Source  string   `json:"source"` // "user" | "agent"
}

// BacklogRequest is the command body for POST /api/backlog.
type BacklogRequest struct {
	Title   string   `json:"title"`
	Body    string   `json:"body,omitempty"`
	Agent   string   `json:"agent,omitempty"`
	Machine string   `json:"machine,omitempty"`
	Labels  []string `json:"labels,omitempty"`
	Source  string   `json:"source,omitempty"` // defaults to "user"
}

// BacklogPatch is the command body for PATCH /api/backlog/{id} (spec 033):
// reassign an Up-next item to another agent ("" clears the pin).
type BacklogPatch struct {
	Agent string `json:"agent"`
}

// BreakdownRequest is the command body for POST /api/breakdown.
type BreakdownRequest struct {
	Text    string `json:"text"`
	Agent   string `json:"agent,omitempty"`
	Machine string `json:"machine,omitempty"`
}

// BreakdownResult is the response for POST /api/breakdown: the visible planner
// run's id. Sub-tasks appear in the backlog when that run completes.
type BreakdownResult struct {
	RunID string `json:"run_id"`
}

// AgentMetrics is one agent's scorecard over the metrics window (spec 033).
// Everything is derived from the append-only event log + run rows — sign-off
// counts are human decisions, never agent estimates. Sample sizes (Assignments,
// Decided) ship alongside every ratio because 30-day windows are small.
type AgentMetrics struct {
	Agent         string    `json:"agent"`
	Assignments   int       `json:"assignments"`              // routed runs + flow task-node executions
	Decided       int       `json:"decided"`                  // sign-offs that reached a decision
	FirstPass     int       `json:"first_pass"`               // approved first try, no note
	FirstPassPct  float64   `json:"first_pass_pct"`           // 0 when Decided==0
	Accepted      int       `json:"accepted"`                 // finally-approved sign-offs
	Redirects     int       `json:"redirects"`                // rejects + approves-with-edits
	RedirectsPer  float64   `json:"redirects_per_assignment"` // 0 when Assignments==0
	CostUSD       float64   `json:"cost_usd"`                 // parsed engine cost; 0 = unknown
	CostPerAccept float64   `json:"cost_per_accepted"`        // 0 = unknown
	CostKnown     bool      `json:"cost_known"`
	Trend         string    `json:"trend"`       // improving | steady | slipping
	TrendDelta    float64   `json:"trend_delta"` // pct-point change between window halves
	Spark         []float64 `json:"spark"`       // 7 first-pass-% buckets, carried forward
	Best          []string  `json:"best"`        // strongest routing lanes (≥3 terminal runs)
	Weak          []string  `json:"weak"`
}

// MetricsResponse is the payload of GET /api/metrics (spec 033).
type MetricsResponse struct {
	WindowDays  int            `json:"window_days"`
	Assignments int            `json:"assignments"`
	Agents      []AgentMetrics `json:"agents"`
	Lanes       []string       `json:"lanes"` // distinct matched_rule values seen in the window
}
