package playbook_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/tobsai/fort/core/graph"
	"github.com/tobsai/fort/core/playbook"
)

func testCatalog() playbook.Catalog {
	defaultAssignment := func(agent string) []playbook.Assignment {
		return []playbook.Assignment{{Agent: agent}}
	}
	return playbook.Catalog{Playbooks: []playbook.Playbook{
		{
			ID: "quick-answer", Name: "Quick answer", Revision: 3,
			Trigger:  playbook.Trigger{Kind: playbook.TaskQuestion, Enabled: true},
			Delivery: playbook.DeliveryAnswer,
			Stages:   []playbook.Stage{{Order: 1, Name: "Answer", Assignments: defaultAssignment("hermes")}},
		},
		{
			ID: "bug-fix", Name: "Bug fix", Revision: 4,
			Trigger:  playbook.Trigger{Kind: playbook.TaskBug, Enabled: true},
			Delivery: playbook.DeliveryAssignment,
			Stages:   []playbook.Stage{{Order: 1, Name: "Fix", Assignments: defaultAssignment("codex")}},
		},
		{
			ID: "research", Name: "Research", Revision: 2,
			Trigger:  playbook.Trigger{Kind: playbook.TaskResearch, Enabled: true},
			Delivery: playbook.DeliveryAssignment,
			Stages:   []playbook.Stage{{Order: 1, Name: "Research", Assignments: defaultAssignment("hermes")}},
		},
		{
			ID: "standard-delivery", Name: "Standard delivery", Revision: 9,
			IsDefault: true, PlanGate: true,
			Trigger:  playbook.Trigger{Kind: playbook.TaskFeature, Enabled: true},
			Delivery: playbook.DeliveryAssignment,
			Stages: []playbook.Stage{
				{Order: 2, Name: "Build", Assignments: []playbook.Assignment{
					{TaskType: playbook.TaskBug, Agent: "codex", Model: "bug-model"},
					{Agent: "claude", Model: "default-model"},
				}},
				{Order: 1, Name: "Plan", Memory: true, Assignments: defaultAssignment("hermes")},
			},
		},
	}}
}

func TestValidateRequiresExactlyOneDefaultAssignmentPerStage(t *testing.T) {
	base := func(assignments []playbook.Assignment) playbook.Catalog {
		return playbook.Catalog{Playbooks: []playbook.Playbook{{
			ID: "default", Name: "Default", Revision: 1, IsDefault: true,
			Trigger:  playbook.Trigger{Kind: playbook.TaskFeature, Enabled: true},
			Delivery: playbook.DeliveryAssignment,
			Stages:   []playbook.Stage{{Order: 1, Name: "Build", Assignments: assignments}},
		}}}
	}

	for name, tc := range map[string]struct {
		assignments []playbook.Assignment
		wantErr     string
	}{
		"none": {
			assignments: []playbook.Assignment{{TaskType: playbook.TaskBug, Agent: "codex"}},
			wantErr:     "exactly one default assignment",
		},
		"two": {
			assignments: []playbook.Assignment{{Agent: "codex"}, {Agent: "claude"}},
			wantErr:     "exactly one default assignment",
		},
		"one": {
			assignments: []playbook.Assignment{{TaskType: playbook.TaskBug, Agent: "codex"}, {Agent: "claude"}},
		},
	} {
		t.Run(name, func(t *testing.T) {
			err := playbook.Validate(base(tc.assignments))
			if tc.wantErr == "" && err != nil {
				t.Fatalf("Validate: %v", err)
			}
			if tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)) {
				t.Fatalf("Validate error = %v, want substring %q", err, tc.wantErr)
			}
		})
	}
}

func TestValidateRejectsInvalidTriggerDeliveryAndRevision(t *testing.T) {
	for name, mutate := range map[string]func(*playbook.Playbook){
		"trigger":  func(p *playbook.Playbook) { p.Trigger.Kind = "guess" },
		"delivery": func(p *playbook.Playbook) { p.Delivery = "telepathy" },
		"revision": func(p *playbook.Playbook) { p.Revision = 0 },
	} {
		t.Run(name, func(t *testing.T) {
			c := testCatalog()
			mutate(&c.Playbooks[0])
			if err := playbook.Validate(c); err == nil {
				t.Fatalf("Validate accepted invalid %s", name)
			}
		})
	}
}

func TestValidateAcceptsManualTriggerForExplicitOnlyPlaybooks(t *testing.T) {
	c := testCatalog()
	c.Playbooks[2].Trigger = playbook.Trigger{Kind: playbook.TaskManual, Enabled: false}
	if err := playbook.Validate(c); err != nil {
		t.Fatalf("Validate manual trigger: %v", err)
	}
	c.Playbooks[2].Trigger.Enabled = true
	if err := playbook.Validate(c); err == nil || !strings.Contains(err.Error(), "manual trigger cannot be enabled") {
		t.Fatalf("Validate enabled manual trigger = %v", err)
	}

	c = testCatalog()
	c.Playbooks[0].Stages[0].Assignments = append(c.Playbooks[0].Stages[0].Assignments,
		playbook.Assignment{TaskType: playbook.TaskManual, Agent: "hermes"})
	if err := playbook.Validate(c); err == nil || !strings.Contains(err.Error(), "invalid task type") {
		t.Fatalf("Validate manual assignment branch = %v", err)
	}
	if _, err := testCatalog().Resolve(playbook.RouteRequest{TaskType: playbook.TaskManual}); err == nil {
		t.Fatal("manual task type resolved, want explicit playbook selection instead")
	}
}

func TestValidateAnswerPlaybooksHaveExactlyOneUngatedStage(t *testing.T) {
	for name, mutate := range map[string]func(*playbook.Playbook){
		"gate": func(p *playbook.Playbook) { p.PlanGate = true },
		"multiple stages": func(p *playbook.Playbook) {
			p.Stages = append(p.Stages, playbook.Stage{Order: 2, Name: "More", Assignments: []playbook.Assignment{{Agent: "hermes"}}})
		},
	} {
		t.Run(name, func(t *testing.T) {
			c := testCatalog()
			mutate(&c.Playbooks[0])
			if err := playbook.Validate(c); err == nil || !strings.Contains(err.Error(), "answer playbook") {
				t.Fatalf("Validate answer invariant = %v", err)
			}
		})
	}
}

func TestClassifyTaskTypeUsesFixedRules(t *testing.T) {
	for _, tc := range []struct {
		text string
		want playbook.TaskType
	}{
		{"How does routing work?", playbook.TaskQuestion},
		{"Reply OK", playbook.TaskQuestion},
		{"Reply with exactly OK", playbook.TaskQuestion},
		{"Respond with exactly OK", playbook.TaskQuestion},
		{"Answer OK", playbook.TaskQuestion},
		{"Say OK", playbook.TaskQuestion},
		{"Please fix the crash in the board", playbook.TaskBug},
		{"Investigate and compare scheduler libraries", playbook.TaskResearch},
		{"Build a new dashboard view", playbook.TaskFeature},
		{"", playbook.TaskFeature},
	} {
		if got := playbook.ClassifyTaskType(tc.text); got != tc.want {
			t.Errorf("ClassifyTaskType(%q) = %q, want %q", tc.text, got, tc.want)
		}
	}
}

func TestConversationalImperativesResolveToQuickAnswerWithoutOverridingExplicitRoute(t *testing.T) {
	c := testCatalog()
	for _, direction := range []string{
		"Reply OK",
		"Reply with exactly OK",
		"Respond with exactly OK",
		"Answer OK",
		"Say OK",
	} {
		t.Run(direction, func(t *testing.T) {
			route, err := c.Resolve(playbook.RouteRequest{Direction: direction})
			if err != nil {
				t.Fatal(err)
			}
			if route.PlaybookID != "quick-answer" || route.TaskType != playbook.TaskQuestion || route.Source != playbook.SourceTrigger || route.Delivery != playbook.DeliveryAnswer {
				t.Fatalf("route = %+v, want triggered quick-answer/question", route)
			}
		})
	}

	feature := "Build a new dashboard view"
	if got := playbook.ClassifyTaskType(feature); got != playbook.TaskFeature {
		t.Fatalf("ClassifyTaskType(%q) = %q, want %q", feature, got, playbook.TaskFeature)
	}
	route, err := c.Resolve(playbook.RouteRequest{Direction: feature})
	if err != nil {
		t.Fatal(err)
	}
	if route.PlaybookID != "standard-delivery" || route.TaskType != playbook.TaskFeature {
		t.Fatalf("feature route = %+v, want standard-delivery/feature", route)
	}

	route, err = c.Resolve(playbook.RouteRequest{
		Direction: "Reply OK", TaskType: playbook.TaskBug, PlaybookID: "research",
	})
	if err != nil {
		t.Fatal(err)
	}
	if route.PlaybookID != "research" || route.TaskType != playbook.TaskBug || route.Source != playbook.SourceManual {
		t.Fatalf("explicit route = %+v, want research/bug/manual", route)
	}
}

func TestResolvePrecedenceAndResolvedStages(t *testing.T) {
	c := testCatalog()
	if err := playbook.Validate(c); err != nil {
		t.Fatalf("fixture invalid: %v", err)
	}

	t.Run("explicit playbook beats explicit type and text", func(t *testing.T) {
		r, err := c.Resolve(playbook.RouteRequest{
			Direction: "How do I fix this crash?", TaskType: playbook.TaskBug, PlaybookID: "research",
		})
		if err != nil {
			t.Fatal(err)
		}
		if r.PlaybookID != "research" || r.Source != playbook.SourceManual || r.TaskType != playbook.TaskBug {
			t.Fatalf("route = %+v", r)
		}
		if r.PlaybookRevision != 2 {
			t.Fatalf("revision = %d, want immutable selected revision 2", r.PlaybookRevision)
		}
	})

	for _, tc := range []struct {
		name string
		req  playbook.RouteRequest
		id   string
		typ  playbook.TaskType
	}{
		{"explicit type", playbook.RouteRequest{Direction: "build something", TaskType: playbook.TaskQuestion}, "quick-answer", playbook.TaskQuestion},
		{"bug text", playbook.RouteRequest{Direction: "the build is broken"}, "bug-fix", playbook.TaskBug},
		{"research text", playbook.RouteRequest{Direction: "research storage options"}, "research", playbook.TaskResearch},
		{"feature default", playbook.RouteRequest{Direction: "add command palette"}, "standard-delivery", playbook.TaskFeature},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, err := c.Resolve(tc.req)
			if err != nil {
				t.Fatal(err)
			}
			if r.PlaybookID != tc.id || r.TaskType != tc.typ {
				t.Fatalf("route = %+v, want %s/%s", r, tc.id, tc.typ)
			}
		})
	}

	// An explicit type controls a stage branch, while stage order is canonicalized.
	r, err := c.Resolve(playbook.RouteRequest{PlaybookID: "standard-delivery", TaskType: playbook.TaskBug})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Stages) != 2 || r.Stages[0].Order != 1 || r.Stages[1].Order != 2 {
		t.Fatalf("resolved stage order = %+v", r.Stages)
	}
	if r.Stages[1].Agent != "codex" || r.Stages[1].Model != "bug-model" {
		t.Fatalf("bug branch = %+v, want codex/bug-model", r.Stages[1])
	}

	// The resolved value is a snapshot, not a view into the mutable catalog slice.
	c.Playbooks[3].Stages[0].Assignments[0].Agent = "changed-later"
	if r.Stages[1].Agent != "codex" || r.PlaybookRevision != 9 {
		t.Fatalf("resolved route mutated with catalog: %+v", r)
	}
}

func TestDisabledTriggerFallsBackButExplicitSelectionStillWorks(t *testing.T) {
	c := testCatalog()
	c.Playbooks[0].Trigger.Enabled = false
	r, err := c.Resolve(playbook.RouteRequest{Direction: "What is Fort?"})
	if err != nil {
		t.Fatal(err)
	}
	if r.PlaybookID != "standard-delivery" || r.Source != playbook.SourceDefault || r.TaskType != playbook.TaskQuestion {
		t.Fatalf("disabled-trigger route = %+v", r)
	}
	r, err = c.Resolve(playbook.RouteRequest{Direction: "What is Fort?", PlaybookID: "quick-answer"})
	if err != nil {
		t.Fatal(err)
	}
	if r.PlaybookID != "quick-answer" || r.Source != playbook.SourceManual {
		t.Fatalf("explicit disabled playbook route = %+v", r)
	}
}

func TestDefaultCatalogIsValidAndCoversEveryTaskType(t *testing.T) {
	c := playbook.DefaultCatalog()
	if err := playbook.Validate(c); err != nil {
		t.Fatalf("DefaultCatalog: %v", err)
	}
	for _, typ := range []playbook.TaskType{playbook.TaskQuestion, playbook.TaskBug, playbook.TaskResearch, playbook.TaskFeature} {
		first, err := c.Resolve(playbook.RouteRequest{TaskType: typ})
		if err != nil || len(first.Stages) == 0 {
			t.Errorf("Resolve(%s) = %+v, %v", typ, first, err)
			continue
		}
		second, err := c.Resolve(playbook.RouteRequest{TaskType: typ})
		if err != nil || !reflect.DeepEqual(first, second) {
			t.Errorf("Resolve(%s) is not deterministic: first=%+v second=%+v err=%v", typ, first, second, err)
		}
	}
	for _, definition := range c.Playbooks {
		for _, stage := range definition.Stages {
			for _, assignment := range stage.Assignments {
				if assignment.Agent == "codex" && assignment.Profile != "codex:gpt-5.5" {
					t.Errorf("%s/%s codex assignment profile = %q, want approved codex:gpt-5.5: %+v", definition.ID, stage.Name, assignment.Profile, assignment)
				}
			}
		}
	}
	var feature playbook.Playbook
	for _, p := range c.Playbooks {
		if p.IsDefault {
			feature = p
			break
		}
	}
	if feature.ID != "feature-work" || feature.Name != "Feature work" || len(feature.Stages) != 3 {
		t.Fatalf("design default = %+v, want Feature work with three stages", feature)
	}
	if got := feature.Stages[0].Assignments[0]; got.Profile != "codex:gpt-5.5" || got.Agent != "codex" || got.Model != "gpt-5.5" {
		t.Fatalf("breakdown assignment = %+v", got)
	}
	if got := feature.Stages[1].Assignments[0]; got.Profile != "codex:gpt-5.5" || got.Agent != "codex" || got.Model != "gpt-5.5" {
		t.Fatalf("design assignment = %+v", got)
	}
	buildAssignments := feature.Stages[2].Assignments
	if len(buildAssignments) != 2 || buildAssignments[0].TaskType != "" || buildAssignments[0].Agent != "claude" || buildAssignments[1].TaskType != playbook.TaskBug || buildAssignments[1].Profile != "codex:gpt-5.5" || buildAssignments[1].Agent != "codex" || buildAssignments[1].Model != "gpt-5.5" {
		t.Fatalf("build assignments = %+v, want features/default before bug fixes", buildAssignments)
	}
	if feature.Stages[0].Description == "" || feature.Stages[1].Description == "" || feature.Stages[2].Description == "" {
		t.Fatalf("source-design stage descriptions missing: %+v", feature.Stages)
	}
}

func TestLegacyGPT55DefaultCatalogKeepsImmutableOpenClawDesignStage(t *testing.T) {
	catalog := playbook.LegacyGPT55DefaultCatalog()
	for _, definition := range catalog.Playbooks {
		if definition.ID != "feature-work" {
			continue
		}
		got := definition.Stages[1].Assignments[0]
		if got.Agent != "openclaw" || got.Model != "Fable" || got.Profile != "" {
			t.Fatalf("legacy GPT-5.5 design assignment = %+v", got)
		}
		return
	}
	t.Fatal("legacy GPT-5.5 Feature work playbook missing")
}

func TestLegacyDefaultCatalogRevision1MatchesShippedAssignments(t *testing.T) {
	catalog := playbook.LegacyDefaultCatalogRevision1()
	byID := make(map[string]playbook.Playbook, len(catalog.Playbooks))
	for _, definition := range catalog.Playbooks {
		byID[definition.ID] = definition
	}
	bug := byID["bug-fix"]
	if got := bug.Stages[1].Assignments[0]; got.Agent != "codex" || got.Model != "5.6 Sol" {
		t.Fatalf("legacy bug build = %+v", got)
	}
	feature := byID["feature-work"]
	if got := feature.Stages[2].Assignments[1]; got.TaskType != playbook.TaskBug || got.Agent != "codex" || got.Model != "5.6 Sol" {
		t.Fatalf("legacy feature bug branch = %+v", got)
	}
}

func TestCompileBuildsVersionedPlaybookFlow(t *testing.T) {
	r := playbook.ResolvedRoute{
		PlaybookID: "standard-delivery", PlaybookRevision: 7,
		PlaybookName: "Standard delivery", TaskType: playbook.TaskBug,
		Source: playbook.SourceManual, Delivery: playbook.DeliveryAssignment, PlanGate: true,
		Stages: []playbook.ResolvedStage{
			{Order: 1, Name: "Plan", Prompt: "Make a plan.", Agent: "hermes", Model: "planner-model", Memory: true},
			{Order: 2, Name: "Build", Prompt: "Build it.", Profile: "codex:gpt-5.5", Agent: "codex", Model: "gpt-5.5"},
		},
	}
	f := playbook.Compile(r)
	if f.ID != "playbook:standard-delivery:7:bug" || f.Name != "Standard delivery" || f.Start != "stage-1" {
		t.Fatalf("flow identity = %+v", f)
	}
	if len(f.Nodes) != 3 {
		t.Fatalf("nodes = %+v, want plan, gate, build", f.Nodes)
	}
	plan, gate, build := f.Nodes[0], f.Nodes[1], f.Nodes[2]
	if plan.Type != graph.Task || plan.Agent != "hermes" || plan.Model != "planner-model" || !plan.Memory || plan.Context != graph.ContextPlaybook {
		t.Fatalf("plan node = %+v", plan)
	}
	if gate.ID != "plan-gate" || gate.Type != graph.Gate || len(gate.Edges) != 1 || gate.Edges[0].To != "stage-2" {
		t.Fatalf("gate node = %+v", gate)
	}
	if build.ID != "stage-2" || build.Profile != "codex:gpt-5.5" || build.Agent != "codex" || build.Model != "gpt-5.5" || build.Context != graph.ContextPlaybook {
		t.Fatalf("build node = %+v", build)
	}
}

func TestValidateRejectsUnknownOrMismatchedExecutionProfile(t *testing.T) {
	for name, assignment := range map[string]playbook.Assignment{
		"unknown":     {Profile: "codex:invented", Agent: "codex", Model: "gpt-5.5"},
		"agent":       {Profile: "codex:gpt-5.5", Agent: "hermes", Model: "gpt-5.5"},
		"model":       {Profile: "codex:gpt-5.5", Agent: "codex", Model: "gpt-5.6-sol"},
		"missing":     {Agent: "codex", Model: "gpt-5.5"},
		"valid exact": {Profile: "codex:gpt-5.5", Agent: "codex", Model: "gpt-5.5"},
	} {
		t.Run(name, func(t *testing.T) {
			c := testCatalog()
			c.Playbooks[0].Stages[0].Assignments[0] = assignment
			err := playbook.Validate(c)
			if name == "valid exact" && err != nil {
				t.Fatalf("Validate exact profile: %v", err)
			}
			if name != "valid exact" && (err == nil || !strings.Contains(err.Error(), "profile")) {
				t.Fatalf("Validate error = %v, want profile rejection", err)
			}
		})
	}
}
