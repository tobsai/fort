package router_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tobsai/fort/core/router"
	"github.com/tobsai/fort/core/rules"
	"github.com/tobsai/fort/core/task"
)

// TestV1RulesetAccuracy is the AO-015 exit gate: the v1 ruleset routes a labeled
// sample set with >=90% accuracy. It also covers AO-017 ("sample tasks for each
// lane route to the intended agent").
func TestV1RulesetAccuracy(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "rules", "v1.yaml"))
	if err != nil {
		t.Fatalf("read v1.yaml: %v", err)
	}
	rs, err := rules.Parse(data)
	if err != nil {
		t.Fatalf("parse v1.yaml: %v", err)
	}
	r := router.New(rs)

	sample := []struct {
		t    task.Task
		want string
	}{
		// dev lane -> codex
		{task.Task{Title: "implement parser", Labels: []string{"feature"}}, "codex"},
		{task.Task{Title: "fix crash", Labels: []string{"bug"}}, "codex"},
		{task.Task{Title: "refactor store", Labels: []string{"refactor"}}, "codex"},
		{task.Task{Title: "add CI", Labels: []string{"ci"}}, "codex"},
		{task.Task{Title: "edit code", Paths: []string{"core/router/router.go"}}, "codex"},
		// design/chat lane -> claude
		{task.Task{Title: "design event schema", Labels: []string{"design"}}, "claude"},
		{task.Task{Title: "decide license", Labels: []string{"decision"}}, "claude"},
		{task.Task{Title: "just chatting", Labels: []string{"chat"}}, "claude"},
		// errand lane -> openclaw
		{task.Task{Title: "provision box", Labels: []string{"provision"}}, "openclaw"},
		{task.Task{Title: "send a message", Labels: []string{"message"}}, "openclaw"},
		{task.Task{Title: "rotate secrets", Labels: []string{"secrets"}}, "openclaw"},
		// research/memory lane -> hermes
		{task.Task{Title: "read the repo", Labels: []string{"research"}}, "hermes"},
		{task.Task{Title: "long digest", Labels: []string{"longrun"}}, "hermes"},
		{task.Task{Title: "remember this", Labels: []string{"knowledge"}}, "hermes"},
		// explicit @agent overrides label
		{task.Task{Title: "force claude", Agent: "claude", Labels: []string{"feature"}}, "claude"},
		// default
		{task.Task{Title: "unlabeled note", CreatedAt: time.Now()}, "claude"},
	}

	correct := 0
	for _, c := range sample {
		got := r.Route(c.t).Route
		if got == c.want {
			correct++
		} else {
			t.Logf("MISROUTE %q -> %q (want %q)", c.t.Title, got, c.want)
		}
	}
	acc := float64(correct) / float64(len(sample))
	if acc < 0.90 {
		t.Errorf("routing accuracy %.0f%% (%d/%d) < 90%%", acc*100, correct, len(sample))
	} else {
		t.Logf("routing accuracy %.0f%% (%d/%d)", acc*100, correct, len(sample))
	}
}
