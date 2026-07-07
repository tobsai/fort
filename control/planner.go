package control

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tobsai/fort/core/engine"
	"github.com/tobsai/fort/core/store"
	"github.com/tobsai/fort/core/task"
	"github.com/tobsai/fort/ui"
)

// Planner implements ui.Planner: it runs a planner agent to decompose a goal
// into backlog sub-tasks (spec 026). The planner is a normal, visible run;
// Breakdown returns its id immediately and the sub-tasks are created when the
// run completes.
type Planner struct {
	e            *engine.Engine
	s            *store.Store
	defaultAgent string
	log          *slog.Logger
}

var _ ui.Planner = Planner{}

// NewPlanner adapts the engine + store to ui.Planner. defaultAgent is the
// planner agent used when a breakdown request doesn't specify one (FORT_PLANNER;
// falls back to "claude").
func NewPlanner(e *engine.Engine, s *store.Store, defaultAgent string) Planner {
	if defaultAgent == "" {
		defaultAgent = "claude"
	}
	return Planner{e: e, s: s, defaultAgent: defaultAgent, log: slog.Default()}
}

type subTask struct {
	Title   string `json:"title"`
	Agent   string `json:"agent"`
	Machine string `json:"machine"`
}

func plannerPrompt(goal string) string {
	return "Break the following work into a short list (3 to 8) of concrete, " +
		"independently-runnable sub-tasks. Output ONLY a JSON array; each element " +
		"an object with a \"title\" (string) and optional \"agent\" and \"machine\" " +
		"strings. No prose, no explanation, no markdown code fences. If the work " +
		"needs no breakdown, output []. Work:\n" + goal
}

// Breakdown dispatches the planner run and returns its id; a goroutine ingests
// its output into the backlog once it completes. NOTE (spec 026 crash window):
// if the process stops between the run finishing and ingest writing items, the
// sub-tasks are not written — re-run the breakdown.
func (p Planner) Breakdown(ctx context.Context, goal, agent, machine string) (string, error) {
	ag := agent
	if ag == "" {
		ag = p.defaultAgent
	}
	t := task.Task{
		ID: uuid.NewString(), Title: "breakdown: " + goal, Body: plannerPrompt(goal),
		Agent: ag, Machine: machine, CreatedAt: time.Now(),
	}
	runID, _, err := p.e.SubmitRef(ctx, t)
	if err != nil {
		return "", err
	}
	go func() {
		p.e.Wait(runID)
		p.ingest(runID, goal)
	}()
	return runID, nil
}

// ingest reads a completed planner run's output and creates backlog items.
func (p Planner) ingest(runID, goal string) {
	run, err := p.s.GetRun(runID)
	if err != nil || run.Status != "succeeded" {
		return // failed/canceled planner -> no items
	}
	evs, err := p.s.Events(runID)
	if err != nil {
		p.log.Error("planner: read events", "run", runID, "err", err)
		return
	}
	subs, ok := parsePlan(evs)
	if !ok {
		p.log.Warn("planner: unparseable output", "run", runID)
		_ = p.s.CreateBacklogItem(store.BacklogItem{
			ID: uuid.NewString(), Title: "breakdown (unparsed): " + goal,
			Body: finalText(evs), Source: "agent", CreatedAt: time.Now(),
		})
		return
	}
	for _, st := range subs {
		_ = p.s.CreateBacklogItem(store.BacklogItem{
			ID: uuid.NewString(), Title: st.Title, Agent: st.Agent, Machine: st.Machine,
			Source: "agent", CreatedAt: time.Now(),
		})
	}
}

// finalText recovers the planner's final answer from a single authoritative
// source: claude's terminal stream-json result line (stored as a raw stdout
// event); falling back to the last normalized message event for plain-text
// providers. It deliberately does NOT concatenate message events (claude emits
// partial deltas AND a terminal line, which would double the array).
func finalText(evs []store.Event) string {
	var result, lastMsg string
	for _, e := range evs {
		switch e.Type {
		case "stdout":
			var line struct {
				Type   string `json:"type"`
				Result string `json:"result"`
			}
			if json.Unmarshal([]byte(e.Data), &line) == nil && line.Type == "result" {
				result = line.Result
			}
		case "message":
			lastMsg = e.Data
		}
	}
	if result != "" {
		return result
	}
	return lastMsg
}

// parsePlan extracts the sub-task list from a run's events. Returns ok=false for
// unparseable output (caller writes the raw fallback); ok=true with zero items
// for a valid empty plan.
func parsePlan(evs []store.Event) ([]subTask, bool) {
	arr, ok := extractArray(finalText(evs))
	if !ok {
		return nil, false
	}
	var raws []subTask
	if json.Unmarshal([]byte(arr), &raws) != nil {
		return nil, false // not an array-of-objects (e.g. [1,2,3]) -> unparsed
	}
	var out []subTask
	for _, r := range raws {
		if strings.TrimSpace(r.Title) == "" {
			continue // shape-validate: every sub-task needs a title
		}
		out = append(out, r)
	}
	return out, true // empty array -> (nil, true): valid, zero items
}

// extractArray strips an optional ```json fence and returns the outermost
// balanced [...] span, or ok=false if there is none.
func extractArray(text string) (string, bool) {
	s := strings.TrimSpace(text)
	if i := strings.Index(s, "```"); i >= 0 {
		s = s[i+3:]
		s = strings.TrimPrefix(strings.TrimSpace(s), "json")
		if j := strings.LastIndex(s, "```"); j >= 0 {
			s = s[:j]
		}
	}
	a := strings.Index(s, "[")
	b := strings.LastIndex(s, "]")
	if a < 0 || b <= a {
		return "", false
	}
	return s[a : b+1], true
}
