package ui_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tobsai/fort/control"
	corecap "github.com/tobsai/fort/core/capability"
	"github.com/tobsai/fort/core/engine"
	"github.com/tobsai/fort/core/flow"
	"github.com/tobsai/fort/core/graph"
	"github.com/tobsai/fort/core/router"
	"github.com/tobsai/fort/core/rules"
	"github.com/tobsai/fort/core/store"
	"github.com/tobsai/fort/core/task"
	"github.com/tobsai/fort/exec/fake"
	"github.com/tobsai/fort/ui"
)

func openStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "fort.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func loadFlows(t *testing.T) []graph.Flow {
	t.Helper()
	f, err := flow.LoadDir(filepath.Join("..", "flows"))
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// full mode: execution plane wired (engine dispatcher + flow runner).
func newFullUI(t *testing.T) (*ui.Server, *store.Store) {
	t.Helper()
	st := openStore(t)
	data, err := os.ReadFile(filepath.Join("..", "rules", "v1.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	rs, err := rules.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	rt := fake.New()
	eng := engine.New(router.New(rs), rt, st, t.TempDir())
	flows := loadFlows(t)
	ids := make([]string, len(flows))
	for i, f := range flows {
		ids[i] = f.ID
	}
	executor := graph.NewExecutor(rt, st)
	t.Cleanup(executor.Wait)
	return ui.New(ui.Deps{
		Dispatcher: control.NewEngineDispatcher(eng),
		Runner:     control.NewFlowExecutor(executor, flows),
		Store:      st,
		FlowIDs:    ids,
	}), st
}

// control-only mode: no execution plane at all.
func newControlUI(t *testing.T) (*ui.Server, *store.Store) {
	t.Helper()
	st := openStore(t)
	return ui.New(ui.Deps{
		Dispatcher: control.NewQueueDispatcher(st),
		Runner:     nil,
		Store:      st,
	}), st
}

func do(t *testing.T, s *ui.Server, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	s.Register(mux)
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode: %v (%s)", err, rec.Body)
	}
	return v
}

func waitForGate(t *testing.T, st *store.Store, runID, nodeID, status string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		nodes, err := st.NodeRuns(runID)
		if err == nil {
			for _, node := range nodes {
				if node.NodeID == nodeID && node.Status == status {
					return
				}
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("gate %s/%s never reached %q", runID, nodeID, status)
}

func waitForRunStatus(t *testing.T, st *store.Store, runID string, statuses ...string) {
	t.Helper()
	want := make(map[string]bool, len(statuses))
	for _, status := range statuses {
		want[status] = true
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if run, err := st.GetRun(runID); err == nil && want[run.Status] {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("run %s never reached one of %v", runID, statuses)
}

func TestWebIcon(t *testing.T) {
	s, _ := newControlUI(t)
	page := do(t, s, "GET", "/", nil)
	if !strings.Contains(page.Body.String(), `href="/fort-icon.png"`) {
		t.Fatal("page does not link the Fort icon")
	}

	icon := do(t, s, "GET", "/fort-icon.png", nil)
	if icon.Code != http.StatusOK {
		t.Fatalf("code %d, want %d", icon.Code, http.StatusOK)
	}
	if got := icon.Header().Get("Content-Type"); got != "image/png" {
		t.Fatalf("content type = %q, want image/png", got)
	}
	if !bytes.HasPrefix(icon.Body.Bytes(), []byte("\x89PNG\r\n\x1a\n")) {
		t.Fatal("response is not a PNG")
	}
}

func TestShippingAppIconAssetsConformToCanonicalIdentity(t *testing.T) {
	type pngAsset struct {
		path          string
		data          []byte
		width, height int
	}
	readPNG := func(path string) pngAsset {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		config, err := png.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("decode PNG config %s: %v", path, err)
		}
		return pngAsset{path: path, data: data, width: config.Width, height: config.Height}
	}
	requireDimensions := func(asset pngAsset, width, height int) {
		t.Helper()
		if asset.width != width || asset.height != height {
			t.Errorf("%s dimensions = %dx%d, want %dx%d", asset.path, asset.width, asset.height, width, height)
		}
	}

	canonical := readPNG(filepath.Join("..", "assets", "fort-icon.png"))
	requireDimensions(canonical, 1024, 1024)
	shipping1024 := []pngAsset{
		readPNG(filepath.Join("apple", "iOS", "Assets.xcassets", "AppIcon.appiconset", "icon-1024.png")),
		readPNG(filepath.Join("apple", "macOS", "Assets.xcassets", "AppIcon.appiconset", "icon_512x512@2x.png")),
	}
	for _, asset := range shipping1024 {
		requireDimensions(asset, 1024, 1024)
		if !bytes.Equal(asset.data, canonical.data) {
			t.Errorf("%s is not byte-identical to canonical identity %s", asset.path, canonical.path)
		}
	}

	macCatalogPath := filepath.Join("apple", "macOS", "Assets.xcassets", "AppIcon.appiconset", "Contents.json")
	macCatalogData, err := os.ReadFile(macCatalogPath)
	if err != nil {
		t.Fatalf("read %s: %v", macCatalogPath, err)
	}
	var macCatalog struct {
		Images []struct {
			Filename string `json:"filename"`
		} `json:"images"`
	}
	if err := json.Unmarshal(macCatalogData, &macCatalog); err != nil {
		t.Fatalf("decode %s: %v", macCatalogPath, err)
	}
	expectedMacDimensions := map[string][2]int{
		"icon_16x16.png":      {16, 16},
		"icon_16x16@2x.png":   {32, 32},
		"icon_32x32.png":      {32, 32},
		"icon_32x32@2x.png":   {64, 64},
		"icon_128x128.png":    {128, 128},
		"icon_128x128@2x.png": {256, 256},
		"icon_256x256.png":    {256, 256},
		"icon_256x256@2x.png": {512, 512},
		"icon_512x512.png":    {512, 512},
		"icon_512x512@2x.png": {1024, 1024},
	}
	if len(macCatalog.Images) != len(expectedMacDimensions) {
		t.Fatalf("macOS app-icon catalog entries = %d, want %d", len(macCatalog.Images), len(expectedMacDimensions))
	}
	seenMac := make(map[string]bool, len(macCatalog.Images))
	for _, entry := range macCatalog.Images {
		dimensions, ok := expectedMacDimensions[entry.Filename]
		if !ok {
			t.Errorf("macOS app-icon catalog has unexpected filename %q", entry.Filename)
			continue
		}
		if seenMac[entry.Filename] {
			t.Errorf("macOS app-icon catalog duplicates filename %q", entry.Filename)
			continue
		}
		seenMac[entry.Filename] = true
		requireDimensions(readPNG(filepath.Join(filepath.Dir(macCatalogPath), entry.Filename)), dimensions[0], dimensions[1])
	}
	for filename := range expectedMacDimensions {
		if !seenMac[filename] {
			t.Errorf("macOS app-icon catalog is missing %q", filename)
		}
	}

	requireDimensions(readPNG("fort-icon.png"), 256, 256)
	orbPath := filepath.Join("apple", "macOS", "Assets.xcassets", "FortAgentOrb.imageset", "fort-agent-orb.png")
	orb := readPNG(orbPath)
	if _, err := png.Decode(bytes.NewReader(orb.data)); err != nil {
		t.Errorf("decode FortAgentOrb PNG %s: %v", orbPath, err)
	}
}

func TestAgentOrbIsServedAsPNG(t *testing.T) {
	s, _ := newControlUI(t)
	orb := do(t, s, "GET", "/fort-agent-orb.png", nil)
	if orb.Code != http.StatusOK {
		t.Fatalf("code %d, want %d", orb.Code, http.StatusOK)
	}
	if got := orb.Header().Get("Content-Type"); got != "image/png" {
		t.Fatalf("content type = %q, want image/png", got)
	}
	if !bytes.HasPrefix(orb.Body.Bytes(), []byte("\x89PNG\r\n\x1a\n")) || orb.Body.Len() < 10_000 {
		t.Fatal("agent orb is not a production PNG asset")
	}
}

// ---- full mode ----

func TestChatCreatesRoutedTask(t *testing.T) {
	s, st := newFullUI(t)
	rec := do(t, s, "POST", "/api/chat", ui.ChatRequest{Text: "please summarize the repo"})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("code %d: %s", rec.Code, rec.Body)
	}
	res := decode[ui.ChatResult](t, rec)
	if res.Kind != "task" || res.Route != "claude" || res.Queued || !res.Accepted || res.Delivery != "assignment" {
		t.Errorf("res = %+v, want accepted task/claude assignment not queued", res)
	}
	if rec.Header().Get("Location") != "/api/runs/"+res.RunID {
		t.Fatalf("Location = %q, want durable run detail", rec.Header().Get("Location"))
	}
	if run, err := st.GetRun(res.RunID); err != nil || run.ID != res.RunID {
		t.Fatalf("accepted run = %+v, %v", run, err)
	}
	waitForRunStatus(t, st, res.RunID, "succeeded", "failed", "canceled")
}

func TestChatShipInstantiatesFlow(t *testing.T) {
	s, st := newFullUI(t)
	rec := do(t, s, "POST", "/api/chat", ui.ChatRequest{Text: "ship dark mode toggle"})
	res := decode[ui.ChatResult](t, rec)
	if rec.Code != http.StatusAccepted || res.Kind != "flow" || res.FlowID != "ship-feature" || res.Paused != "" || !res.Accepted || res.Delivery != "assignment" {
		t.Fatalf("res = %+v status=%d, want accepted flow/ship-feature", res, rec.Code)
	}
	waitForGate(t, st, res.RunID, "plan_gate", "waiting")
}

func TestGateDecisionResumesFlow(t *testing.T) {
	s, st := newFullUI(t)
	rec := do(t, s, "POST", "/api/chat", ui.ChatRequest{Text: "ship a thing"})
	started := decode[ui.ChatResult](t, rec)
	waitForGate(t, st, started.RunID, "plan_gate", "waiting")

	rec = do(t, s, "POST", "/api/gate", ui.GateDecision{RunID: started.RunID, NodeID: "plan_gate", Decision: "approve"})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("gate code %d: %s", rec.Code, rec.Body)
	}
	ar := decode[ui.ActionResult](t, rec)
	if ar.State != "accepted" || ar.PausedNode != "" {
		t.Errorf("after plan approve = %+v, want accepted continuation", ar)
	}
	if rec.Header().Get("Location") != "/api/runs/"+started.RunID {
		t.Fatalf("Location = %q", rec.Header().Get("Location"))
	}
	waitForGate(t, st, started.RunID, "merge_gate", "waiting")
}

func TestGateRejectRecordsNote(t *testing.T) {
	s, st := newFullUI(t)
	rec := do(t, s, "POST", "/api/chat", ui.ChatRequest{Text: "ship a smaller thing"})
	started := decode[ui.ChatResult](t, rec)
	waitForGate(t, st, started.RunID, "plan_gate", "waiting")

	rec = do(t, s, "POST", "/api/gate", ui.GateDecision{RunID: started.RunID, NodeID: "plan_gate", Decision: "reject", Note: "smaller please"})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("gate code %d: %s", rec.Code, rec.Body)
	}
	rec = do(t, s, "GET", "/api/runs/"+started.RunID, nil)
	d := decode[ui.RunDetail](t, rec)
	found := false
	for _, e := range d.Events {
		if e.Type == "gate" && e.NodeID == "plan_gate" && strings.Contains(e.Data, "smaller please") && strings.Contains(e.Data, `"rejected"`) {
			found = true
		}
	}
	if !found {
		t.Fatalf("no gate event with the redirect note in %+v", d.Events)
	}
}

func TestBoardListsRunsAndGates(t *testing.T) {
	s, st := newFullUI(t)
	started := decode[ui.ChatResult](t, do(t, s, "POST", "/api/chat", ui.ChatRequest{Text: "ship something"}))
	waitForGate(t, st, started.RunID, "plan_gate", "waiting")
	rec := do(t, s, "GET", "/api/board", nil)
	b := decode[ui.Board](t, rec)
	if len(b.Runs) != 1 {
		t.Errorf("runs = %d, want 1", len(b.Runs))
	}
	if len(b.Gates) != 1 || b.Gates[0].NodeID != "plan_gate" {
		t.Errorf("gates = %+v, want [plan_gate]", b.Gates)
	}
}

func TestBoardCarriesTimestampsAndCheckpoints(t *testing.T) {
	s, st := newFullUI(t)
	started := decode[ui.ChatResult](t, do(t, s, "POST", "/api/chat", ui.ChatRequest{Text: "ship checkpoints"}))
	waitForGate(t, st, started.RunID, "plan_gate", "waiting")
	rec := do(t, s, "GET", "/api/board", nil)
	if strings.Contains(rec.Body.String(), "null") {
		t.Fatalf("board body contains null: %s", rec.Body)
	}
	b := decode[ui.Board](t, rec)
	if len(b.Runs) != 1 {
		t.Fatalf("runs = %d, want 1", len(b.Runs))
	}
	r := b.Runs[0]
	if r.CreatedAt == "" || r.UpdatedAt == "" {
		t.Errorf("run timestamps missing: %+v", r)
	}
	if r.Checkpoints == nil {
		t.Fatalf("flow run has no checkpoint summary: %+v", r)
	}
	// ship-feature has gates plan_gate, merge_gate, escalate; one is waiting.
	if r.Checkpoints.Total != 3 || r.Checkpoints.Waiting != 1 || r.Checkpoints.Accepted != 0 {
		t.Errorf("checkpoints = %+v, want total 3 waiting 1", r.Checkpoints)
	}
	if len(b.Gates) != 1 || b.Gates[0].Since == "" {
		t.Errorf("gate since missing: %+v", b.Gates)
	}
}

func TestOpenClawMessageBecomesTask(t *testing.T) {
	s, _ := newFullUI(t)
	rec := do(t, s, "POST", "/api/openclaw", ui.OpenClawMessage{From: "+15550100", Text: "tell me when the build is done"})
	res := decode[ui.ChatResult](t, rec)
	if res.Kind != "task" || res.Route != "openclaw" {
		t.Errorf("res = %+v, want task routed to openclaw", res)
	}
}

// ---- control-only mode ----

func TestControlOnlyChatQueuesTask(t *testing.T) {
	s, st := newControlUI(t)
	rec := do(t, s, "POST", "/api/chat", ui.ChatRequest{Text: "remember to water plants"})
	res := decode[ui.ChatResult](t, rec)
	if res.Kind != "task" || !res.Queued || res.Route != "" {
		t.Fatalf("res = %+v, want a queued task with no route", res)
	}
	run, err := st.GetRun(res.RunID)
	if err != nil || run.Status != "queued" {
		t.Errorf("run = %+v err=%v, want queued", run, err)
	}
}

func TestControlOnlyShipDoesNotRunFlow(t *testing.T) {
	s, _ := newControlUI(t)
	rec := do(t, s, "POST", "/api/chat", ui.ChatRequest{Text: "ship dark mode"})
	res := decode[ui.ChatResult](t, rec)
	// with no execution plane, "ship X" degrades to a boarded task, not a flow.
	if res.Kind != "task" || !res.Queued {
		t.Errorf("res = %+v, want a queued task (no flow without execution)", res)
	}
}

func TestControlOnlyGateReturns409(t *testing.T) {
	s, _ := newControlUI(t)
	rec := do(t, s, "POST", "/api/gate", ui.GateDecision{RunID: "x", NodeID: "g", Decision: "approve"})
	if rec.Code != http.StatusConflict {
		t.Errorf("code = %d, want 409 (no execution plane)", rec.Code)
	}
}

func TestControlOnlyBoardAndSummaryWork(t *testing.T) {
	s, _ := newControlUI(t)
	_ = do(t, s, "POST", "/api/chat", ui.ChatRequest{Text: "task one"})
	_ = do(t, s, "POST", "/api/chat", ui.ChatRequest{Text: "task two"})

	board := decode[ui.Board](t, do(t, s, "GET", "/api/board", nil))
	if len(board.Runs) != 2 {
		t.Errorf("board runs = %d, want 2", len(board.Runs))
	}
	sum := decode[ui.Summary](t, do(t, s, "GET", "/api/summary", nil))
	if sum.Total != 2 || sum.Queued != 2 || sum.Execution {
		t.Errorf("summary = %+v, want total/queued 2 and execution=false", sum)
	}
}

// Array fields must serialize as [] never null, so strictly-typed clients
// (the Swift surfaces) decode cleanly. Regression: FortKit failed on gates:null.
func TestArraysSerializeAsEmptyNotNull(t *testing.T) {
	s, _ := newControlUI(t)
	for _, path := range []string{"/api/summary", "/api/board"} {
		body := do(t, s, "GET", path, nil).Body.String()
		if strings.Contains(body, "null") {
			t.Errorf("%s emitted null for an array (want []): %s", path, strings.TrimSpace(body))
		}
	}
}

// ---- multi-machine (spec 022) ----

// capturingDispatcher records the task it received (to assert wiring).
type capturingDispatcher struct{ last task.Task }

func (d *capturingDispatcher) Submit(_ context.Context, t task.Task) (ui.RunRef, error) {
	d.last = t
	return ui.RunRef{RunID: "cap", Route: t.Agent, Machine: t.Machine}, nil
}

type stubMachines struct{ list []ui.MachineStatus }

func (s stubMachines) Machines() []ui.MachineStatus { return s.list }

func TestChatPinsMachine(t *testing.T) {
	st := openStore(t)
	cd := &capturingDispatcher{}
	s := ui.New(ui.Deps{Dispatcher: cd, Store: st})
	rec := do(t, s, "POST", "/api/chat", ui.ChatRequest{Text: "build it", Machine: "macbook-pro"})
	res := decode[ui.ChatResult](t, rec)
	if cd.last.Machine != "macbook-pro" {
		t.Fatalf("task.Machine = %q, want macbook-pro", cd.last.Machine)
	}
	if res.Machine != "macbook-pro" {
		t.Fatalf("result.Machine = %q, want macbook-pro", res.Machine)
	}
}

func TestChatForcesAgent(t *testing.T) {
	st := openStore(t)
	cd := &capturingDispatcher{}
	s := ui.New(ui.Deps{Dispatcher: cd, Store: st})
	rec := do(t, s, "POST", "/api/chat", ui.ChatRequest{Text: "build it", Agent: "codex"})
	res := decode[ui.ChatResult](t, rec)
	if cd.last.Agent != "codex" {
		t.Fatalf("task.Agent = %q, want codex", cd.last.Agent)
	}
	if res.Route != "codex" {
		t.Fatalf("result.Route = %q, want codex", res.Route)
	}
}

func TestProfilesExposeClosedCatalogChoices(t *testing.T) {
	st := openStore(t)
	s := ui.New(ui.Deps{Dispatcher: &capturingDispatcher{}, Store: st})
	rec := do(t, s, "GET", "/api/profiles", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body)
	}
	profiles := decode[[]ui.ProfileOption](t, rec)
	want := map[string]struct{ model, display string }{
		"codex:gpt-5.6-sol":   {model: "gpt-5.6-sol", display: "Codex · GPT-5.6 Sol"},
		"codex:gpt-5.6-terra": {model: "gpt-5.6-terra", display: "Codex · GPT-5.6 Terra"},
		"codex:gpt-5.6-luna":  {model: "gpt-5.6-luna", display: "Codex · GPT-5.6 Luna"},
	}
	for _, profile := range profiles {
		expected, ok := want[profile.ID]
		if !ok {
			continue
		}
		if profile.Agent != "codex" || profile.Model != expected.model || profile.DisplayName != expected.display {
			t.Fatalf("profile = %+v", profile)
		}
		if profile.State != corecap.OfferUnknown || profile.Reason != corecap.ReasonStale {
			t.Fatalf("inventory-free profile state = %q/%q, want unknown/stale", profile.State, profile.Reason)
		}
		delete(want, profile.ID)
	}
	if len(want) != 0 {
		t.Fatalf("closed Codex profiles missing: %#v", want)
	}
	var raw []map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	configuredDefaults := 0
	for _, profile := range raw {
		var id string
		if err := json.Unmarshal(profile["id"], &id); err != nil {
			t.Fatal(err)
		}
		if !strings.HasSuffix(id, ":configured-default") {
			continue
		}
		configuredDefaults++
		if _, present := profile["model"]; present {
			t.Fatalf("configured-default profile %q invented a concrete model: %s", id, rec.Body.String())
		}
	}
	if configuredDefaults == 0 {
		t.Fatal("closed profile catalog exposed no configured-default profiles")
	}
}

func TestProfilesAggregateReadyMachinesWithoutSubstitution(t *testing.T) {
	st := openStore(t)
	snapshot := corecap.Snapshot{Machines: []corecap.MachineInventory{
		{
			Name: "laptop", Reachable: true,
			Profiles: []corecap.ProfileOffer{{ID: "codex:gpt-5.5", State: corecap.OfferReady}},
		},
		{
			Name: "mac-mini", Reachable: true,
			Profiles: []corecap.ProfileOffer{{ID: "codex:gpt-5.5", State: corecap.OfferSetupRequired, Reason: corecap.ReasonModelUnavailable}},
		},
	}}
	s := ui.New(ui.Deps{
		Dispatcher: &capturingDispatcher{}, Store: st,
		Capabilities: stubCapabilities{snapshot: snapshot, generation: 9},
	})
	profiles := decode[[]ui.ProfileOption](t, do(t, s, http.MethodGet, "/api/profiles", nil))
	for _, profile := range profiles {
		if profile.ID != "codex:gpt-5.5" {
			continue
		}
		if profile.State != corecap.OfferReady || profile.Reason != "" {
			t.Fatalf("aggregate state = %q/%q, want ready with no reason", profile.State, profile.Reason)
		}
		if len(profile.Machines) != 1 || profile.Machines[0] != "laptop" {
			t.Fatalf("ready machines = %#v, want only laptop", profile.Machines)
		}
		return
	}
	t.Fatal("codex:gpt-5.5 profile missing")
}

func TestChatUsesExactCatalogProfile(t *testing.T) {
	st := openStore(t)
	cd := &capturingDispatcher{}
	s := ui.New(ui.Deps{Dispatcher: cd, Store: st})
	rec := do(t, s, "POST", "/api/chat", ui.ChatRequest{
		Text: "build it", Profile: "codex:gpt-5.6-terra", Machine: "mac-mini",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body)
	}
	if cd.last.Profile != "codex:gpt-5.6-terra" || cd.last.Agent != "codex" || cd.last.Model != "gpt-5.6-terra" || cd.last.Machine != "mac-mini" {
		t.Fatalf("task = %+v, want exact profile-derived selection", cd.last)
	}
}

func TestChatRejectsUnknownOrMismatchedProfileBeforeDispatch(t *testing.T) {
	for _, request := range []ui.ChatRequest{
		{Text: "build it", Profile: "codex:not-real"},
		{Text: "build it", Agent: "claude", Profile: "codex:gpt-5.5"},
	} {
		st := openStore(t)
		cd := &capturingDispatcher{}
		s := ui.New(ui.Deps{Dispatcher: cd, Store: st})
		rec := do(t, s, "POST", "/api/chat", request)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("request=%+v status=%d body=%s", request, rec.Code, rec.Body)
		}
		if cd.last.ID != "" {
			t.Fatalf("invalid profile dispatched task %+v", cd.last)
		}
	}
}

func TestChatSplitsTitleAndBody(t *testing.T) {
	st := openStore(t)
	cd := &capturingDispatcher{}
	s := ui.New(ui.Deps{Dispatcher: cd, Store: st})
	rec := do(t, s, "POST", "/api/chat", ui.ChatRequest{Text: "fix the header\n# Details\n- step one"})
	if rec.Code != 200 {
		t.Fatalf("status=%d", rec.Code)
	}
	if cd.last.Title != "fix the header" || cd.last.Body != "# Details\n- step one" {
		t.Fatalf("title=%q body=%q", cd.last.Title, cd.last.Body)
	}
}

func TestChatSingleLineHasNoBody(t *testing.T) {
	st := openStore(t)
	cd := &capturingDispatcher{}
	s := ui.New(ui.Deps{Dispatcher: cd, Store: st})
	rec := do(t, s, "POST", "/api/chat", ui.ChatRequest{Text: "just a title"})
	if rec.Code != 200 || cd.last.Title != "just a title" || cd.last.Body != "" {
		t.Fatalf("code=%d title=%q body=%q", rec.Code, cd.last.Title, cd.last.Body)
	}
}

func TestChatTitleSkipsLeadingBlankLines(t *testing.T) {
	st := openStore(t)
	cd := &capturingDispatcher{}
	s := ui.New(ui.Deps{Dispatcher: cd, Store: st})
	rec := do(t, s, "POST", "/api/chat", ui.ChatRequest{Text: "\nDo the thing\ndetails"})
	if rec.Code != 200 || cd.last.Title != "Do the thing" || cd.last.Body != "details" {
		t.Fatalf("code=%d title=%q body=%q", rec.Code, cd.last.Title, cd.last.Body)
	}
	// A whitespace-only first line is skipped too.
	rec = do(t, s, "POST", "/api/chat", ui.ChatRequest{Text: "   \nDo the thing"})
	if rec.Code != 200 || cd.last.Title != "Do the thing" || cd.last.Body != "" {
		t.Fatalf("code=%d title=%q body=%q", rec.Code, cd.last.Title, cd.last.Body)
	}
}

func TestMachinesEndpointReturnsRoster(t *testing.T) {
	st := openStore(t)
	roster := stubMachines{list: []ui.MachineStatus{
		{Name: "mac-mini", Agents: []string{"claude", "codex"}, Local: true, Reachable: true},
		{Name: "macbook-pro", Agents: []string{"codex"}, Reachable: false},
	}}
	s := ui.New(ui.Deps{Dispatcher: control.NewQueueDispatcher(st), Store: st, Machines: roster})
	got := decode[[]ui.MachineStatus](t, do(t, s, "GET", "/api/machines", nil))
	if len(got) != 2 || got[0].Name != "mac-mini" || !got[0].Local || got[1].Reachable {
		t.Fatalf("machines = %+v", got)
	}
}

func TestMachinesEmptyWhenSingleMachine(t *testing.T) {
	s, _ := newControlUI(t)
	rec := do(t, s, "GET", "/api/machines", nil)
	if got := decode[[]ui.MachineStatus](t, rec); len(got) != 0 {
		t.Fatalf("want empty roster, got %+v", got)
	}
	if strings.Contains(rec.Body.String(), "null") {
		t.Errorf("machines emitted null (want []): %s", strings.TrimSpace(rec.Body.String()))
	}
}

type stubCapabilities struct {
	snapshot   corecap.Snapshot
	generation uint64
}

func (s stubCapabilities) Capabilities() (corecap.Snapshot, uint64) {
	return s.snapshot, s.generation
}

func TestCapabilitiesEndpointReturnsCurrentSecretFreeSnapshot(t *testing.T) {
	st := openStore(t)
	snapshot := corecap.Snapshot{
		CatalogVersion: corecap.CatalogVersion, ProfileMappingVersion: corecap.ProfileMappingVersion, LocalMachine: "laptop",
		Revision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Machines: []corecap.MachineInventory{{
			Name: "laptop", Local: true, Reachable: true, State: corecap.MachinePartial,
			Profiles: []corecap.ProfileOffer{}, Offers: []corecap.LogicalOffer{},
			Bindings: []corecap.ExecutionBindingOffer{},
		}},
	}
	s := ui.New(ui.Deps{
		Dispatcher: control.NewQueueDispatcher(st), Store: st,
		Capabilities: stubCapabilities{snapshot: snapshot, generation: 7},
	})
	rec := do(t, s, http.MethodGet, "/api/capabilities", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	got := decode[ui.CapabilitiesResponse](t, rec)
	if got.Generation != 7 || got.Snapshot.LocalMachine != "laptop" || got.Snapshot.Machines == nil {
		t.Fatalf("capabilities=%+v", got)
	}
}

func TestCapabilitiesEndpointFailsClosedWhenNotWired(t *testing.T) {
	s, _ := newControlUI(t)
	rec := do(t, s, http.MethodGet, "/api/capabilities", nil)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d, want 503", rec.Code)
	}
}

func TestCapabilitiesEndpointFailsClosedWhileInitialInventoryRefreshes(t *testing.T) {
	st := openStore(t)
	s := ui.New(ui.Deps{
		Dispatcher:   control.NewQueueDispatcher(st),
		Store:        st,
		Capabilities: stubCapabilities{},
	})
	rec := do(t, s, http.MethodGet, "/api/capabilities", nil)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s, want 503", rec.Code, rec.Body.String())
	}
	if got := strings.TrimSpace(rec.Body.String()); got != "capability inventory initializing" {
		t.Fatalf("body=%q, want explicit initialization state", got)
	}
}

// ---- shared ----

func TestEventsSSEReplaysLog(t *testing.T) {
	s, st := newControlUI(t)
	_ = st.CreateRun(store.Run{ID: "r1", Agent: "codex", Status: "running"})
	_, _ = st.AppendEvent(store.Event{RunID: "r1", Type: "started", Data: "codex"})
	_, _ = st.AppendEvent(store.Event{RunID: "r1", Type: "message", Data: "hello world"})

	mux := http.NewServeMux()
	s.Register(mux)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL+"/api/events?since=0", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("sse: %v", err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("content-type = %q", ct)
	}
	var got []string
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		if line := sc.Text(); strings.HasPrefix(line, "data: ") {
			got = append(got, line)
			if len(got) >= 2 {
				break
			}
		}
	}
	if len(got) < 2 || !strings.Contains(strings.Join(got, "\n"), "hello world") {
		t.Fatalf("SSE frames = %v", got)
	}
}

func TestRunDetailIncludesNodeID(t *testing.T) {
	s, st := newControlUI(t)
	_ = st.CreateRun(store.Run{ID: "rd1", Agent: "codex", Status: "running"})
	_, _ = st.AppendEvent(store.Event{RunID: "rd1", NodeID: "implement", Type: "message", Data: "hi"})
	_, _ = st.AppendEvent(store.Event{RunID: "rd1", Type: "stdout", Data: "raw"})

	rec := do(t, s, "GET", "/api/runs/rd1", nil)
	if rec.Code != 200 {
		t.Fatalf("status = %d", rec.Code)
	}
	d := decode[ui.RunDetail](t, rec)
	if len(d.Events) != 2 {
		t.Fatalf("events = %d, want 2", len(d.Events))
	}
	if d.Events[0].NodeID != "implement" || d.Events[1].NodeID != "" {
		t.Fatalf("node_ids = %q,%q want implement,\"\"", d.Events[0].NodeID, d.Events[1].NodeID)
	}
}
