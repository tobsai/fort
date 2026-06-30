// Package router is Fort's deterministic matcher engine (backlog AO-013).
//
// It maps a Task to exactly one route by applying an ordered ruleset with
// first-match-wins semantics and a default fallback. Every decision is recorded
// as a RouteDecision. There are zero model/LLM calls on this path: Route is a
// pure function of the Task and the Ruleset (proven by TestRoutingIsDeterministic).
package router

import (
	"fmt"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/tobsai/fort/core/rules"
	"github.com/tobsai/fort/core/task"
)

// RouteDecision is the recorded outcome of routing a single task.
type RouteDecision struct {
	TaskID      string `json:"task_id"`
	Route       string `json:"route"`        // the chosen agent
	MatchedRule string `json:"matched_rule"` // rule id, "" when defaulted
	Default     bool   `json:"default"`      // true when no rule matched
	Reason      string `json:"reason"`
}

// Router applies a ruleset to tasks.
type Router struct {
	rs *rules.Ruleset
}

// New builds a Router over a validated ruleset.
func New(rs *rules.Ruleset) *Router { return &Router{rs: rs} }

// Route returns the single route for a task. It walks rules in order and
// returns the first match; if none match it returns the default route.
func (r *Router) Route(t task.Task) RouteDecision {
	for _, rule := range r.rs.Rules {
		if matches(&rule.When, t) {
			return RouteDecision{
				TaskID:      t.ID,
				Route:       rule.Route,
				MatchedRule: rule.ID,
				Reason:      fmt.Sprintf("matched rule %q", rule.ID),
			}
		}
	}
	return RouteDecision{
		TaskID:  t.ID,
		Route:   r.rs.Defaults.Route,
		Default: true,
		Reason:  "no rule matched; used defaults.route",
	}
}

// matches evaluates a matcher against a task. A leaf with multiple signal
// fields requires all of them (AND). Any/All compose sub-matchers.
func matches(m *rules.Matcher, t task.Task) bool {
	if len(m.Any) > 0 {
		any := false
		for i := range m.Any {
			if matches(&m.Any[i], t) {
				any = true
				break
			}
		}
		if !any {
			return false
		}
	}
	for i := range m.All {
		if !matches(&m.All[i], t) {
			return false
		}
	}
	if len(m.Label) > 0 && !hasAny(t.Labels, m.Label) {
		return false
	}
	if len(m.Path) > 0 && !anyPathMatches(m.Path, t.Paths) {
		return false
	}
	if m.Repo != "" && !strings.EqualFold(m.Repo, t.Repo) {
		return false
	}
	if m.Agent != "" && !strings.EqualFold(m.Agent, t.Agent) {
		return false
	}
	if len(m.Size) > 0 && !containsFold(m.Size, t.Size) {
		return false
	}
	if m.Time != nil && !inTimeWindow(m.Time, t.CreatedAt) {
		return false
	}
	return true
}

func hasAny(have, want []string) bool {
	for _, w := range want {
		for _, h := range have {
			if strings.EqualFold(h, w) {
				return true
			}
		}
	}
	return false
}

func containsFold(set []string, v string) bool {
	for _, s := range set {
		if strings.EqualFold(s, v) {
			return true
		}
	}
	return false
}

func anyPathMatches(globs, paths []string) bool {
	for _, g := range globs {
		for _, p := range paths {
			if ok, _ := doublestar.Match(g, p); ok {
				return true
			}
		}
	}
	return false
}

// inTimeWindow reports whether ts's clock time is within [After, Before).
// When After > Before the window wraps midnight (e.g. 18:00–06:00).
func inTimeWindow(w *rules.TimeWindow, ts time.Time) bool {
	after, ok1 := parseHHMM(w.After)
	before, ok2 := parseHHMM(w.Before)
	if !ok1 || !ok2 {
		return false
	}
	cur := ts.Hour()*60 + ts.Minute()
	if after <= before {
		return cur >= after && cur < before
	}
	// Wrapping window: inside if at/after start OR before end.
	return cur >= after || cur < before
}

func parseHHMM(s string) (int, bool) {
	var h, m int
	if _, err := fmt.Sscanf(s, "%d:%d", &h, &m); err != nil {
		return 0, false
	}
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, false
	}
	return h*60 + m, true
}
