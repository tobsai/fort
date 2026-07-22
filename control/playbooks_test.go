package control

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/tobsai/fort/core/graph"
	"github.com/tobsai/fort/core/runtime"
	"github.com/tobsai/fort/exec/fake"
	"github.com/tobsai/fort/ui"
)

func playbookByID(t *testing.T, items []ui.Playbook, id string) ui.Playbook {
	t.Helper()
	for _, item := range items {
		if item.ID == id {
			return item
		}
	}
	t.Fatalf("playbook %q not found in %+v", id, items)
	return ui.Playbook{}
}

func TestPlaybookCatalogSeedsImmutableDefaultsAndPreservesDescriptions(t *testing.T) {
	st := newStore(t)
	cat := NewPlaybookCatalog(st)

	items, err := cat.List(context.Background())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(items) != 4 {
		t.Fatalf("seeded playbooks = %d, want 4", len(items))
	}
	if !items[0].IsDefault || items[0].ID != "feature-work" {
		t.Fatalf("first playbook = %+v, want the default first for the editor rail", items[0])
	}
	original := playbookByID(t, items, "feature-work")
	original.Name = "Feature delivery"
	original.Stages[0].Description = "Turn the direction into a reviewable plan."
	saved, err := cat.Save(context.Background(), original)
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if saved.Revision != 2 || saved.Stages[0].Description != original.Stages[0].Description {
		t.Fatalf("saved = %+v", saved)
	}

	items, err = cat.List(context.Background())
	if err != nil {
		t.Fatalf("list after save: %v", err)
	}
	latest := playbookByID(t, items, "feature-work")
	if latest.Revision != 2 || latest.Name != "Feature delivery" || latest.Stages[0].Description != original.Stages[0].Description {
		t.Fatalf("latest = %+v", latest)
	}
	prior, err := st.PlaybookRevision("feature-work", 1)
	if err != nil {
		t.Fatalf("load prior: %v", err)
	}
	if strings.Contains(prior.Data, "reviewable plan") || !strings.Contains(prior.Data, `"revision":1`) {
		t.Fatalf("revision 1 was not immutable: %s", prior.Data)
	}
}

func TestPlaybookCatalogValidatesBeforeAppending(t *testing.T) {
	st := newStore(t)
	cat := NewPlaybookCatalog(st)
	items, err := cat.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	invalid := playbookByID(t, items, "bug-fix")
	invalid.Stages[0].Assignments = nil
	if _, err := cat.Save(context.Background(), invalid); err == nil {
		t.Fatal("invalid playbook save succeeded")
	}
	latest, err := st.LatestPlaybookRevisions()
	if err != nil {
		t.Fatal(err)
	}
	for _, revision := range latest {
		if revision.ID == "bug-fix" && revision.Revision != 1 {
			t.Fatalf("invalid definition appended revision %d", revision.Revision)
		}
	}
}

func TestPlaybookCatalogRejectsStaleWholeDocumentSave(t *testing.T) {
	cat := NewPlaybookCatalog(newStore(t))
	items, err := cat.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	stale := playbookByID(t, items, "bug-fix")
	first := stale
	first.Name = "Bug repair"
	saved, err := cat.Save(context.Background(), first)
	if err != nil {
		t.Fatal(err)
	}
	stale.Name = "Overwrite from stale editor"
	if _, err := cat.Save(context.Background(), stale); err == nil || !strings.Contains(err.Error(), "stale revision") {
		t.Fatalf("stale save error = %v", err)
	}
	latest, err := cat.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	got := playbookByID(t, latest, "bug-fix")
	if got.Revision != saved.Revision || got.Name != "Bug repair" {
		t.Fatalf("stale save changed latest: %+v", got)
	}
}

func TestPlaybookCatalogDuplicatesAsDisabledNonDefaultCopy(t *testing.T) {
	cat := NewPlaybookCatalog(newStore(t))
	copy, err := cat.Duplicate(context.Background(), "bug-fix")
	if err != nil {
		t.Fatalf("duplicate: %v", err)
	}
	if copy.ID != "bug-fix-copy" || copy.Revision != 1 || copy.IsDefault || copy.Trigger.Enabled {
		t.Fatalf("copy = %+v", copy)
	}
	second, err := cat.Duplicate(context.Background(), "bug-fix")
	if err != nil {
		t.Fatalf("second duplicate: %v", err)
	}
	if second.ID != "bug-fix-copy-2" {
		t.Fatalf("second copy id = %q", second.ID)
	}
}

func TestPlaybookCatalogRoutesExactRevisionWithPlanGateOverride(t *testing.T) {
	st := newStore(t)
	cat := NewPlaybookCatalog(st)
	items, err := cat.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	updated := playbookByID(t, items, "feature-work")
	updated.Name = "Changed later"
	updated.Stages[0].Assignments[0].Model = "new-model"
	if _, err := cat.Save(context.Background(), updated); err != nil {
		t.Fatalf("save revision 2: %v", err)
	}
	withoutGate := false
	req := ui.RouteRequest{
		Text: "Fix the broken export", PlaybookID: "feature-work",
		PlaybookRevision: 1, TaskType: "bug", PlanGate: &withoutGate,
	}
	first, err := cat.Route(context.Background(), req)
	if err != nil {
		t.Fatalf("route: %v", err)
	}
	second, err := cat.Route(context.Background(), req)
	if err != nil {
		t.Fatalf("route again: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("route is not deterministic:\nfirst=%+v\nsecond=%+v", first, second)
	}
	if first.PlaybookRevision != 1 || first.PlaybookName != "Feature work" || first.TaskType != "bug" || first.Source != "manual" || first.PlanGate {
		t.Fatalf("route = %+v", first)
	}
	if first.Stages[0].Model != "Codex 5.6 Sol" {
		t.Fatalf("route used edited revision model %q", first.Stages[0].Model)
	}
	latest, err := st.LatestPlaybookRevisions()
	if err != nil {
		t.Fatal(err)
	}
	if len(latest) != 4 {
		t.Fatalf("pure previews changed persistence: %d revisions", len(latest))
	}
}

func TestPlaybookCatalogRejectsGateOverrideForAnswerDelivery(t *testing.T) {
	cat := NewPlaybookCatalog(newStore(t))
	withGate := true
	_, err := cat.Route(context.Background(), ui.RouteRequest{
		Text: "Why was it skipped?", PlaybookID: "quick-answer", PlaybookRevision: 1,
		TaskType: "question", PlanGate: &withGate,
	})
	if err == nil || !strings.Contains(err.Error(), "answer route") {
		t.Fatalf("gated answer route error = %v", err)
	}
}

func TestPlaybookCatalogSeedsAtStartupSoRouteIsReadOnly(t *testing.T) {
	st := newStore(t)
	cat := NewPlaybookCatalog(st)
	before, err := st.LatestPlaybookRevisions()
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != 4 {
		t.Fatalf("startup revisions = %d, want 4 before the first route", len(before))
	}
	if _, err := cat.Route(context.Background(), ui.RouteRequest{Text: "Build export"}); err != nil {
		t.Fatal(err)
	}
	after, err := st.LatestPlaybookRevisions()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("route mutated catalog:\nbefore=%+v\nafter=%+v", before, after)
	}
}

func TestFlowExecutorPlaybookFlowResumesFromCanonicalRevisionAfterRestart(t *testing.T) {
	st := newStore(t)
	cat := NewPlaybookCatalog(st)
	rt := fake.New()
	fx := NewFlowExecutor(graph.NewExecutor(rt, st), nil).WithPlaybooks(cat)
	route, err := cat.Route(context.Background(), ui.RouteRequest{
		Text: "Build export", PlaybookID: "feature-work", PlaybookRevision: 1, TaskType: "feature",
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := fx.StartPlaybook(context.Background(), route, "pb-run", "\n  Build CSV export\nwith audit logs")
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	const flowID = "playbook:feature-work:1:feature:gate1:assignment"
	if result.FlowID != flowID || result.State != "paused" || result.PausedNode != "plan-gate" {
		t.Fatalf("start result = %+v", result)
	}
	run, err := st.GetRun("pb-run")
	if err != nil {
		t.Fatal(err)
	}
	if run.Title != "Build CSV export" || run.FlowID != flowID {
		t.Fatalf("run = %+v", run)
	}

	items, err := cat.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	edited := playbookByID(t, items, "feature-work")
	edited.Stages[1].Assignments[0].Agent = "hermes"
	if _, err := cat.Save(context.Background(), edited); err != nil {
		t.Fatalf("save later revision: %v", err)
	}

	restarted := NewFlowExecutor(graph.NewExecutor(rt, st), nil).WithPlaybooks(cat)
	plan := restarted.Plan(flowID)
	if len(plan) != 4 || plan[0].ID != "stage-1" || plan[1].ID != "plan-gate" {
		t.Fatalf("dynamic plan = %+v", plan)
	}
	if err := restarted.Approve("pb-run", "plan-gate", "approved implementation plan"); err != nil {
		t.Fatalf("approve: %v", err)
	}
	resumed, err := restarted.ResumeFlow(context.Background(), flowID, "pb-run")
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	if resumed.State != "completed" {
		t.Fatalf("resume result = %+v", resumed)
	}
	specs := rt.Dispatched()
	if len(specs) != 3 || specs[1].Agent != "openclaw" {
		t.Fatalf("dispatches = %+v; resume did not use immutable revision 1", specs)
	}
}

func TestFlowExecutorQuickAnswerReturnsLastTaskOutput(t *testing.T) {
	st := newStore(t)
	cat := NewPlaybookCatalog(st)
	fx := NewFlowExecutor(graph.NewExecutor(fake.New(), st), nil).WithPlaybooks(cat)
	route, err := cat.Route(context.Background(), ui.RouteRequest{
		Text: "Why was export skipped?", PlaybookID: "quick-answer", PlaybookRevision: 1, TaskType: "question",
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := fx.StartPlaybook(context.Background(), route, "answer-run", "Why was export skipped?")
	if err != nil {
		t.Fatalf("start answer: %v", err)
	}
	if result.State != "completed" || result.FlowID != "playbook:quick-answer:1:question:gate0:answer" {
		t.Fatalf("result = %+v", result)
	}
	if !strings.Contains(result.Answer, "Why was export skipped?") {
		t.Fatalf("answer = %q, want last task output", result.Answer)
	}
}

func TestFlowExecutorQuickAnswerFailureReturnsAnError(t *testing.T) {
	st := newStore(t)
	cat := NewPlaybookCatalog(st)
	rt := fake.New()
	rt.ExitCode = 1
	fx := NewFlowExecutor(graph.NewExecutor(rt, st), nil).WithPlaybooks(cat)
	route, err := cat.Route(context.Background(), ui.RouteRequest{
		Text: "Why was export skipped?", PlaybookID: "quick-answer", PlaybookRevision: 1, TaskType: "question",
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := fx.StartPlaybook(context.Background(), route, "failed-answer", "Why was export skipped?")
	if err == nil || !strings.Contains(err.Error(), "quick answer failed") {
		t.Fatalf("failed answer result=%+v err=%v", result, err)
	}
	if result.State != "failed" || result.Answer != "" {
		t.Fatalf("failed answer result = %+v", result)
	}
}

type silentRuntime struct{}

func (silentRuntime) Name() string { return "silent" }

func (silentRuntime) Dispatch(_ context.Context, spec runtime.RunSpec) (runtime.Run, error) {
	events := make(chan runtime.RunEvent)
	close(events)
	return silentRun{id: spec.RunID, events: events}, nil
}

type silentRun struct {
	id     string
	events <-chan runtime.RunEvent
}

func (r silentRun) ID() string                      { return r.id }
func (r silentRun) Stream() <-chan runtime.RunEvent { return r.events }
func (silentRun) Signal(string) error               { return nil }
func (silentRun) Cancel() error                     { return nil }
func (silentRun) Status() runtime.Status            { return runtime.Status{State: runtime.StateSucceeded} }
func (silentRun) Wait() runtime.Status              { return runtime.Status{State: runtime.StateSucceeded} }

func TestFlowExecutorQuickAnswerEmptyOutputReturnsAnError(t *testing.T) {
	st := newStore(t)
	cat := NewPlaybookCatalog(st)
	fx := NewFlowExecutor(graph.NewExecutor(silentRuntime{}, st), nil).WithPlaybooks(cat)
	route, err := cat.Route(context.Background(), ui.RouteRequest{
		Text: "Why was export skipped?", PlaybookID: "quick-answer", PlaybookRevision: 1, TaskType: "question",
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := fx.StartPlaybook(context.Background(), route, "empty-answer", "Why was export skipped?")
	if err == nil || !strings.Contains(err.Error(), "without output") {
		t.Fatalf("empty answer result=%+v err=%v", result, err)
	}
	if result.State != "completed" || result.Answer != "" {
		t.Fatalf("empty answer result = %+v", result)
	}
}
