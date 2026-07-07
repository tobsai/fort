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
		body := finalText(evs)
		if strings.TrimSpace(body) == "" {
			body = "(planner produced no output)"
		}
		_ = p.s.CreateBacklogItem(store.BacklogItem{
			ID: uuid.NewString(), Title: "breakdown (unparsed): " + goal,
			Body: body, Source: "agent", CreatedAt: time.Now(),
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

// decodePlan extracts the sub-task list from a single final-answer text. The
// planner is instructed to output ONLY a JSON array, so decodePlan tolerates
// minor deviations (leading prose, a ```json fence, trailing whitespace) but is
// deliberately conservative about ambiguity: it scans for every TOP-LEVEL JSON
// array that decodes as a plan — an empty array, or one with at least one
// titled object — and accepts a result ONLY when exactly one exists. Zero plan
// arrays -> unparsed (caller writes the raw fallback). Two or more (an
// illustrative example beside the real plan, whether it leads or trails, or a
// doubled array) -> ambiguous, also unparsed: never guess which is the plan.
//
// A json.Decoder respects string literals, so brackets and ```fences``` inside a
// title are content, not structure. Two spans are skipped whole so their
// contents can't be miscounted: a decoded plan array (its inner brackets), and
// any JSON object — an array nested as an object field (e.g. claude refusing
// with {"decision":"…","example":[…]}) is NOT the plan, so consuming the object
// prevents that field array from being mistaken for a top-level plan.
func decodePlan(text string) ([]subTask, bool) {
	var found []subTask
	count := 0
	for i := 0; i < len(text); {
		switch text[i] {
		case '[':
			dec := json.NewDecoder(strings.NewReader(text[i:]))
			var raws []subTask
			if dec.Decode(&raws) != nil {
				i++ // this '[' doesn't begin a JSON array-of-objects
				continue
			}
			if items := planItems(raws); len(raws) == 0 || len(items) > 0 {
				count++ // plan-shaped: a valid empty array, or has titled objects
				found = items
			}
			i += int(dec.InputOffset()) // skip past this array (its inner brackets)
		case '{':
			dec := json.NewDecoder(strings.NewReader(text[i:]))
			var obj json.RawMessage
			if dec.Decode(&obj) != nil {
				i++ // not a JSON object; keep scanning
				continue
			}
			i += int(dec.InputOffset()) // consume the object + any nested arrays
		default:
			i++
		}
	}
	if count != 1 {
		return nil, false
	}
	return found, true // exactly one plan array; empty -> (nil, true) valid zero
}

// planItems returns the sub-tasks with a non-empty title (shape validation): an
// array of titleless objects yields none, so decodePlan treats it as not
// plan-shaped rather than a valid empty plan.
func planItems(raws []subTask) []subTask {
	var out []subTask
	for _, r := range raws {
		if strings.TrimSpace(r.Title) != "" {
			out = append(out, r)
		}
	}
	return out
}
