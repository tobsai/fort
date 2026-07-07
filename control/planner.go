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

// finalOutputs scans a run's events for candidate final-answer texts. results
// are claude's terminal stream-json result line(s) (stored as raw stdout
// events), in order; lastMsg is the last normalized message event (plain-text
// providers like hermes). It deliberately does NOT concatenate message events
// (claude emits partial deltas AND a terminal line, which would double the
// array — spec 026 D6).
func finalOutputs(evs []store.Event) (results []string, lastMsg string) {
	for _, e := range evs {
		switch e.Type {
		case "stdout":
			var line struct {
				Type   string `json:"type"`
				Result string `json:"result"`
			}
			if json.Unmarshal([]byte(e.Data), &line) == nil && line.Type == "result" {
				results = append(results, line.Result)
			}
		case "message":
			lastMsg = e.Data
		}
	}
	return results, lastMsg
}

// finalText is the single best raw text for the unparsed-fallback body: claude's
// terminal result line if present, else the last message event.
func finalText(evs []store.Event) string {
	results, lastMsg := finalOutputs(evs)
	if len(results) > 0 {
		return results[len(results)-1]
	}
	return lastMsg
}

// parsePlan extracts the sub-task list from a run's events. It tries each
// candidate final-answer text (result line(s), then the message fallback) and
// returns the first that decodes to a valid plan — so a later non-plan result
// (e.g. a failover retry emitting prose) can't clobber an earlier plan-bearing
// one. Returns ok=false for unparseable output (caller writes the raw fallback);
// ok=true with zero items for a valid empty plan.
func parsePlan(evs []store.Event) ([]subTask, bool) {
	results, lastMsg := finalOutputs(evs)
	candidates := results
	if lastMsg != "" {
		candidates = append(candidates, lastMsg)
	}
	for _, text := range candidates {
		if subs, ok := decodePlan(text); ok {
			return subs, true
		}
	}
	return nil, false
}

// decodePlan extracts a JSON array of sub-tasks from a single final-answer text.
// It decodes the FIRST complete JSON array starting at the first '[', using a
// json.Decoder — which respects string literals (so brackets and ```fences``` in
// a title are treated as content, not structure) and stops at the array's end
// (so trailing prose or an illustrative fenced example is ignored). If a second
// adjacent top-level array follows, the output is ambiguous (which is the
// plan?), so it is rejected to the unparsed fallback rather than guessed.
func decodePlan(text string) ([]subTask, bool) {
	start := strings.Index(text, "[")
	if start < 0 {
		return nil, false
	}
	dec := json.NewDecoder(strings.NewReader(text[start:]))
	var raws []subTask
	if dec.Decode(&raws) != nil {
		return nil, false // not a JSON array-of-objects (e.g. [1,2,3])
	}
	if rest := strings.TrimSpace(text[start+int(dec.InputOffset()):]); strings.HasPrefix(rest, "[") {
		return nil, false // a second adjacent array -> ambiguous -> unparsed
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
