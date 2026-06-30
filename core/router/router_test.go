package router

import (
	"testing"
	"time"

	"github.com/tobsai/fort/core/rules"
	"github.com/tobsai/fort/core/task"
)

// at builds a Task whose CreatedAt is today at the given 24h clock time.
func at(hh, mm int) time.Time {
	return time.Date(2026, 6, 30, hh, mm, 0, 0, time.UTC)
}

func mustRuleset(t *testing.T, y string) *rules.Ruleset {
	t.Helper()
	rs, err := rules.Parse([]byte(y))
	if err != nil {
		t.Fatalf("parse ruleset: %v", err)
	}
	return rs
}

const lanes = `
version: 1
defaults:
  route: claude
rules:
  - id: explicit-agent
    when:
      agent: codex
    route: codex
  - id: dev-lane
    when:
      label: [dev, feature, bug]
    route: codex
  - id: design-lane
    when:
      label: [design, chat, research]
    route: claude
  - id: pkg-path
    when:
      path: ["packages/**/*.ts"]
    route: codex
  - id: errand-night
    when:
      all:
        - label: [errand]
        - time:
            after: "18:00"
            before: "06:00"
    route: openclaw
  - id: big-fort-research
    when:
      all:
        - repo: fort
        - size: [L, XL]
        - any:
            - label: [knowledge]
            - label: [longrun]
    route: hermes
`

func TestMatcherTable(t *testing.T) {
	r := New(mustRuleset(t, lanes))

	cases := []struct {
		name      string
		task      task.Task
		wantRoute string
		wantRule  string
	}{
		{"explicit @agent beats label", task.Task{Agent: "codex", Labels: []string{"design"}}, "codex", "explicit-agent"},
		{"dev label", task.Task{Labels: []string{"feature"}}, "codex", "dev-lane"},
		{"design label", task.Task{Labels: []string{"research"}}, "claude", "design-lane"},
		{"path glob", task.Task{Paths: []string{"packages/core/src/x.ts"}}, "codex", "pkg-path"},
		{"errand at night", task.Task{Labels: []string{"errand"}, CreatedAt: at(23, 0)}, "openclaw", "errand-night"},
		{"errand in daytime falls through to default", task.Task{Labels: []string{"errand"}, CreatedAt: at(12, 0)}, "claude", ""},
		{"big fort knowledge -> hermes", task.Task{Repo: "fort", Size: "L", Labels: []string{"knowledge"}}, "hermes", "big-fort-research"},
		{"big fort wrong-repo falls through", task.Task{Repo: "other", Size: "L", Labels: []string{"knowledge"}}, "claude", ""},
		{"no signals -> default", task.Task{}, "claude", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := r.Route(tc.task)
			if d.Route != tc.wantRoute {
				t.Errorf("route = %q, want %q", d.Route, tc.wantRoute)
			}
			if d.MatchedRule != tc.wantRule {
				t.Errorf("matched rule = %q, want %q", d.MatchedRule, tc.wantRule)
			}
			if tc.wantRule == "" && !d.Default {
				t.Errorf("expected Default=true for fallthrough")
			}
		})
	}
}

func TestFirstMatchWins(t *testing.T) {
	// dev-lane appears before design-lane; a task with both labels takes dev-lane.
	r := New(mustRuleset(t, lanes))
	d := r.Route(task.Task{Labels: []string{"design", "feature"}})
	if d.MatchedRule != "dev-lane" {
		t.Errorf("matched %q, want dev-lane (first match wins)", d.MatchedRule)
	}
}

func TestTimeWindowWrapsMidnight(t *testing.T) {
	r := New(mustRuleset(t, lanes))
	// 02:00 is inside the 18:00-06:00 wrap window.
	d := r.Route(task.Task{Labels: []string{"errand"}, CreatedAt: at(2, 0)})
	if d.Route != "openclaw" {
		t.Errorf("02:00 errand route = %q, want openclaw", d.Route)
	}
}

func TestRoutingIsDeterministic(t *testing.T) {
	r := New(mustRuleset(t, lanes))
	tk := task.Task{Repo: "fort", Size: "XL", Labels: []string{"longrun"}}
	first := r.Route(tk)
	for i := 0; i < 50; i++ {
		if got := r.Route(tk); got != first {
			t.Fatalf("non-deterministic routing: %+v vs %+v", got, first)
		}
	}
}
