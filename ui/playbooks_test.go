package ui_test

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/tobsai/fort/control"
	"github.com/tobsai/fort/core/graph"
	"github.com/tobsai/fort/core/store"
	"github.com/tobsai/fort/exec/fake"
	"github.com/tobsai/fort/ui"
)

type stubPlaybooks struct {
	items       []ui.Playbook
	saved       ui.Playbook
	duplicated  string
	route       ui.RoutePreview
	routeCalls  int
	lastRequest ui.RouteRequest
	saveErr     error
}

func (s *stubPlaybooks) List(context.Context) ([]ui.Playbook, error) {
	return append([]ui.Playbook(nil), s.items...), nil
}

func (s *stubPlaybooks) Save(_ context.Context, p ui.Playbook) (ui.Playbook, error) {
	s.saved = p
	if s.saveErr != nil {
		return ui.Playbook{}, s.saveErr
	}
	p.Revision++
	return p, nil
}

func TestPlaybookStaleSaveReturnsConflict(t *testing.T) {
	st := openStore(t)
	cat := &stubPlaybooks{saveErr: fmt.Errorf("control: conditional append: %w", store.ErrPlaybookRevisionStale)}
	s := ui.New(ui.Deps{Dispatcher: &capturingDispatcher{}, Store: st, Playbooks: cat})
	rec := do(t, s, http.MethodPut, "/api/playbooks", samplePlaybook())
	if rec.Code != http.StatusConflict {
		t.Fatalf("stale save status = %d, want 409: %s", rec.Code, rec.Body)
	}
}

func TestPlaybookStaleSaveFromAnotherStoreReturnsConflict(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fort.db")
	firstStore, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { firstStore.Close() })
	firstCatalog := control.NewPlaybookCatalog(firstStore)

	secondStore, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { secondStore.Close() })
	secondCatalog := control.NewPlaybookCatalog(secondStore)

	staleItems, err := secondCatalog.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	stale := staleItems[0]
	updated := stale
	updated.Name = "Feature delivery elsewhere"
	if _, err := firstCatalog.Save(context.Background(), updated); err != nil {
		t.Fatal(err)
	}

	s := ui.New(ui.Deps{Dispatcher: &capturingDispatcher{}, Store: secondStore, Playbooks: secondCatalog})
	rec := do(t, s, http.MethodPut, "/api/playbooks", stale)
	if rec.Code != http.StatusConflict {
		t.Fatalf("cross-store stale save status = %d, want 409: %s", rec.Code, rec.Body)
	}
}

func (s *stubPlaybooks) Duplicate(_ context.Context, id string) (ui.Playbook, error) {
	s.duplicated = id
	return ui.Playbook{ID: id + "-copy", Name: "Copy", Revision: 1, Stages: []ui.PlaybookStage{}}, nil
}

func (s *stubPlaybooks) Route(_ context.Context, req ui.RouteRequest) (ui.RoutePreview, error) {
	s.routeCalls++
	s.lastRequest = req
	return s.route, nil
}

type stubPlaybookRunner struct {
	calls     int
	route     ui.RoutePreview
	runID     string
	direction string
	result    ui.PlaybookRunResult
}

func (s *stubPlaybookRunner) StartPlaybook(_ context.Context, route ui.RoutePreview, runID, direction string) (ui.PlaybookRunResult, error) {
	s.calls++
	s.route, s.runID, s.direction = route, runID, direction
	return s.result, nil
}

func samplePlaybook() ui.Playbook {
	return ui.Playbook{
		ID: "feature-work", Name: "Feature work", IsDefault: true, PlanGate: true, Revision: 3,
		Trigger:  ui.PlaybookTrigger{Kind: "feature", Enabled: true},
		Delivery: "assignment",
		Stages: []ui.PlaybookStage{{
			Order: 1, Name: "Break down", Memory: true,
			Assignments: []ui.PlaybookAssignment{{Agent: "hermes", Model: "gpt-5.6-sol"}},
		}},
	}
}

func sampleRoute(delivery string) ui.RoutePreview {
	return ui.RoutePreview{
		PlaybookID: "feature-work", PlaybookRevision: 3, PlaybookName: "Feature work",
		TaskType: "feature", Source: "manual", PlanGate: true, Delivery: delivery,
		Stages: []ui.ResolvedPlaybookStage{{Order: 1, Name: "Break down", Agent: "hermes", Model: "gpt-5.6-sol", Memory: true}},
	}
}

func TestPlaybookCatalogHTTPContract(t *testing.T) {
	st := openStore(t)
	cat := &stubPlaybooks{items: []ui.Playbook{samplePlaybook()}}
	s := ui.New(ui.Deps{Dispatcher: &capturingDispatcher{}, Store: st, Playbooks: cat})

	listRec := do(t, s, http.MethodGet, "/api/playbooks", nil)
	if listRec.Code != http.StatusOK {
		t.Fatalf("GET status = %d: %s", listRec.Code, listRec.Body)
	}
	list := decode[[]ui.Playbook](t, listRec)
	if len(list) != 1 || list[0].Revision != 3 || list[0].Stages[0].Assignments[0].Model != "gpt-5.6-sol" {
		t.Fatalf("catalog = %+v", list)
	}

	put := samplePlaybook()
	put.Name = "Feature delivery"
	putRec := do(t, s, http.MethodPut, "/api/playbooks", put)
	if putRec.Code != http.StatusOK || cat.saved.Name != "Feature delivery" {
		t.Fatalf("PUT status=%d saved=%+v body=%s", putRec.Code, cat.saved, putRec.Body)
	}
	if got := decode[ui.Playbook](t, putRec); got.Revision != 4 {
		t.Fatalf("saved revision = %d, want 4", got.Revision)
	}

	dupRec := do(t, s, http.MethodPost, "/api/playbooks/feature-work/duplicate", nil)
	if dupRec.Code != http.StatusOK || cat.duplicated != "feature-work" {
		t.Fatalf("duplicate status=%d id=%q", dupRec.Code, cat.duplicated)
	}
}

func TestRoutePreviewIsPureAndCarriesExactRevision(t *testing.T) {
	st := openStore(t)
	cat := &stubPlaybooks{route: sampleRoute("assignment")}
	runner := &stubPlaybookRunner{}
	s := ui.New(ui.Deps{Dispatcher: &capturingDispatcher{}, Store: st, Playbooks: cat, PlaybookRunner: runner})

	rec := do(t, s, http.MethodPost, "/api/route", ui.RouteRequest{
		Text: "Add export", PlaybookID: "feature-work", PlaybookRevision: 3, TaskType: "feature",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("route status = %d: %s", rec.Code, rec.Body)
	}
	got := decode[ui.RoutePreview](t, rec)
	if got.PlaybookRevision != 3 || got.Stages[0].Model != "gpt-5.6-sol" || cat.lastRequest.Text != "Add export" {
		t.Fatalf("preview=%+v request=%+v", got, cat.lastRequest)
	}
	if runner.calls != 0 {
		t.Fatalf("route preview dispatched %d runs", runner.calls)
	}
}

func TestChatPlaybookOverrideExecutesPreviewedRevision(t *testing.T) {
	st := openStore(t)
	cat := &stubPlaybooks{route: sampleRoute("assignment")}
	runner := &stubPlaybookRunner{result: ui.PlaybookRunResult{State: "paused", PausedNode: "plan_gate", FlowID: "playbook:feature-work:3:feature"}}
	s := ui.New(ui.Deps{Dispatcher: &capturingDispatcher{}, Store: st, Playbooks: cat, PlaybookRunner: runner})

	rec := do(t, s, http.MethodPost, "/api/chat", ui.ChatRequest{
		Text: "Add CSV export", PlaybookID: "feature-work", PlaybookRevision: 3, TaskType: "feature",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("chat status = %d: %s", rec.Code, rec.Body)
	}
	got := decode[ui.ChatResult](t, rec)
	if got.Kind != "flow" || got.FlowID != "playbook:feature-work:3:feature" || got.Paused != "plan_gate" {
		t.Fatalf("chat result = %+v", got)
	}
	if runner.calls != 1 || runner.route.PlaybookRevision != 3 || runner.direction != "Add CSV export" || runner.runID == "" {
		t.Fatalf("runner calls=%d route=%+v direction=%q run=%q", runner.calls, runner.route, runner.direction, runner.runID)
	}
}

func TestQuickAnswerReturnsInlineText(t *testing.T) {
	st := openStore(t)
	cat := &stubPlaybooks{route: sampleRoute("answer")}
	runner := &stubPlaybookRunner{result: ui.PlaybookRunResult{State: "completed", FlowID: "playbook:quick-answer:1:question", Answer: "The retry window closed."}}
	s := ui.New(ui.Deps{Dispatcher: &capturingDispatcher{}, Store: st, Playbooks: cat, PlaybookRunner: runner})

	rec := do(t, s, http.MethodPost, "/api/chat", ui.ChatRequest{Text: "Why was it skipped?", TaskType: "question", PlaybookID: "quick-answer", PlaybookRevision: 1})
	if rec.Code != http.StatusOK {
		t.Fatalf("chat status = %d: %s", rec.Code, rec.Body)
	}
	got := decode[ui.ChatResult](t, rec)
	if got.Kind != "answer" || got.Answer != "The retry window closed." || got.RunID == "" || got.Accepted || got.Delivery != "" {
		t.Fatalf("answer result = %+v", got)
	}
}

func TestProductionQuickAnswerReturnsDurableAcceptedRunBeforeOutput(t *testing.T) {
	st := openStore(t)
	cat := control.NewPlaybookCatalog(st)
	rt := fake.New()
	fx := control.NewFlowExecutor(graph.NewExecutor(rt, st), nil).WithPlaybooks(cat)
	s := ui.New(ui.Deps{
		Dispatcher: &capturingDispatcher{}, Store: st, Runner: fx,
		Playbooks: cat, PlaybookRunner: fx,
	})

	rec := do(t, s, http.MethodPost, "/api/chat", ui.ChatRequest{
		Text: "Reply OK", PlaybookID: "quick-answer", PlaybookRevision: 1, TaskType: "question",
	})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202: %s", rec.Code, rec.Body)
	}
	result := decode[ui.ChatResult](t, rec)
	if result.Kind != "answer" || result.RunID == "" || result.Answer != "" || !result.Accepted || result.Delivery != "answer" {
		t.Fatalf("accepted answer = %+v", result)
	}
	if rec.Header().Get("Location") != "/api/runs/"+result.RunID {
		t.Fatalf("Location = %q", rec.Header().Get("Location"))
	}
	if run, err := st.GetRun(result.RunID); err != nil || run.FlowID != result.FlowID {
		t.Fatalf("durable answer run = %+v, %v", run, err)
	}
	waitForRunStatus(t, st, result.RunID, "succeeded", "failed")
}

func TestProductionPlaybookRequestChangesReturnsToSameGateWithoutDownstreamDispatch(t *testing.T) {
	st := openStore(t)
	cat := control.NewPlaybookCatalog(st)
	rt := fake.New()
	fx := control.NewFlowExecutor(graph.NewExecutor(rt, st), nil).WithPlaybooks(cat)
	s := ui.New(ui.Deps{
		Dispatcher: &capturingDispatcher{}, Store: st, Runner: fx,
		Playbooks: cat, PlaybookRunner: fx,
	})

	started := decode[ui.ChatResult](t, do(t, s, http.MethodPost, "/api/chat", ui.ChatRequest{
		Text: "Build export", PlaybookID: "feature-work", PlaybookRevision: 1, TaskType: "feature",
	}))
	waitForGate(t, st, started.RunID, "plan-gate", "waiting")
	before := len(rt.Dispatched())

	rec := do(t, s, http.MethodPost, "/api/gate", ui.GateDecision{
		RunID: started.RunID, NodeID: "plan-gate", Decision: "reject", Note: "narrow the scope",
	})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("request changes status = %d: %s", rec.Code, rec.Body)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		nodes, _ := st.NodeRuns(started.RunID)
		for _, node := range nodes {
			if node.NodeID == "plan-gate" && node.Status == "waiting" {
				run, _ := st.GetRun(started.RunID)
				if run.Status != "blocked" {
					t.Fatalf("request-changes run status = %q, want blocked", run.Status)
				}
				if got := len(rt.Dispatched()); got != before {
					t.Fatalf("request changes dispatched downstream work: before=%d after=%d", before, got)
				}
				return
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("playbook gate did not return to waiting after request changes")
}

func TestQuickAnswerCannotReturnSuccessWithoutCompletedOutput(t *testing.T) {
	for _, tc := range []struct {
		name   string
		result ui.PlaybookRunResult
	}{
		{name: "failed", result: ui.PlaybookRunResult{State: "failed", FlowID: "playbook:quick-answer:1:question:gate0:answer"}},
		{name: "empty", result: ui.PlaybookRunResult{State: "completed", FlowID: "playbook:quick-answer:1:question:gate0:answer"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := openStore(t)
			cat := &stubPlaybooks{route: sampleRoute("answer")}
			runner := &stubPlaybookRunner{result: tc.result}
			s := ui.New(ui.Deps{Dispatcher: &capturingDispatcher{}, Store: st, Playbooks: cat, PlaybookRunner: runner})

			rec := do(t, s, http.MethodPost, "/api/chat", ui.ChatRequest{
				Text: "Why was it skipped?", TaskType: "question", PlaybookID: "quick-answer", PlaybookRevision: 1,
			})
			if rec.Code != http.StatusInternalServerError {
				t.Fatalf("status = %d, want 500: %s", rec.Code, rec.Body)
			}
		})
	}
}

func TestPlaybookExecutionRequiresExecutionPlane(t *testing.T) {
	st := openStore(t)
	cat := &stubPlaybooks{route: sampleRoute("assignment")}
	s := ui.New(ui.Deps{Dispatcher: &capturingDispatcher{}, Store: st, Playbooks: cat})
	rec := do(t, s, http.MethodPost, "/api/chat", ui.ChatRequest{Text: "Build it", PlaybookID: "feature-work", PlaybookRevision: 3})
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409: %s", rec.Code, rec.Body)
	}
}
