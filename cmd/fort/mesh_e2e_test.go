package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tobsai/fort/control"
	corecap "github.com/tobsai/fort/core/capability"
	"github.com/tobsai/fort/core/config"
	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/machines"
	"github.com/tobsai/fort/core/runtime"
	"github.com/tobsai/fort/core/store"
	execcap "github.com/tobsai/fort/exec/capability"
	"github.com/tobsai/fort/exec/cluster"
	"github.com/tobsai/fort/exec/fake"
	"github.com/tobsai/fort/exec/meshjoin"
	"github.com/tobsai/fort/exec/node"
	"github.com/tobsai/fort/exec/remote"
	"github.com/tobsai/fort/ui"
)

type meshReadyCapabilityProber struct {
	configuredModel string
}

func (p meshReadyCapabilityProber) Probe(_ context.Context, request execcap.ProbeRequest) execcap.ProbeObservation {
	observation := execcap.ProbeObservation{
		State:         corecap.PredicateSatisfied,
		StableBinding: []string{"mesh-e2e=" + request.PredicateID},
	}
	if request.ProfileID == "codex:configured-default" && request.PredicateID == "predicate.codex.model.codex:configured-default.v1" {
		observation.ResolvedModel = p.configuredModel
	}
	return observation
}

func newMeshCapabilityRegistry(t *testing.T, nodeID string, key byte, configuredModel string, now time.Time) *execcap.Registry {
	t.Helper()
	registry, err := execcap.NewRegistry(execcap.RegistryOptions{
		NodeID: nodeID, Platform: "darwin/arm64", RevisionKey: bytes.Repeat([]byte{key}, 32),
		Prober: meshReadyCapabilityProber{configuredModel: configuredModel}, Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("new %s capability registry: %v", nodeID, err)
	}
	return registry
}

type meshRecordingRuntime struct {
	next runtime.Runtime

	mu    sync.Mutex
	specs []runtime.RunSpec
}

func (r *meshRecordingRuntime) Name() string { return "mesh-recorder(" + r.next.Name() + ")" }

func (r *meshRecordingRuntime) Dispatch(ctx context.Context, spec runtime.RunSpec) (runtime.Run, error) {
	r.mu.Lock()
	r.specs = append(r.specs, spec)
	r.mu.Unlock()
	return r.next.Dispatch(ctx, spec)
}

func (r *meshRecordingRuntime) Dispatched() []runtime.RunSpec {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]runtime.RunSpec(nil), r.specs...)
}

// TestMeshPairingEndToEnd drives the spec-024 enrollment and spec-041 shared
// conversation flows through the real HTTP handlers: invite → join → raw and
// conversation dispatch → remove. It assembles two Forts:
//
//   - a HUB: a meshjoin.Server + a roster (GET /api/machines) mounted on one
//     mux, served over an httptest server; its data dir is dirA.
//   - a WORKER: a node.Server over a fake runtime, served over a second httptest
//     server, authenticating with the token it persists to dirB's node.yaml.
//
// The loopback-vs-private constraint (validateWorkerURL rejects the 127.0.0.1
// httptest bind address) is handled exactly as the plan requires: the JOIN leg
// runs against the hub mux with a private CGNAT RemoteAddr and advertise_url=""
// so the DERIVED url can be asserted; the DISPATCH leg installs a transport
// directly with cluster.Add pointing at the real httptest worker URL, and runs
// a RunSpec{Machine: worker} through the cluster to prove the run crosses
// machines.
func TestMeshPairingEndToEnd(t *testing.T) {
	now := time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)
	const workerName = "worker"

	// --- worker Fort: node exec endpoint over a fake runtime ---
	dirB := t.TempDir()
	workerFake := fake.New()
	workerCapabilities := newMeshCapabilityRegistry(t, workerName, 0x22, "gpt-5.6-terra", now)
	// The worker token becomes live only after the CLI-equivalent SaveNodeFile;
	// node.New reads it fresh per request via this closure (spec 024).
	workerTokenOf := func() string {
		nf, _ := config.ReadNodeFile(dirB)
		return nf.Token
	}
	workerNode := node.New(workerFake, workerTokenOf)
	workerNode.UseCapabilities(workerCapabilities)
	workerMux := http.NewServeMux()
	workerNode.Register(workerMux)
	workerSrv := httptest.NewServer(workerMux)
	defer workerSrv.Close()

	// --- hub Fort: meshjoin server + roster, on one mux (mirrors serve's mount) ---
	dirA := t.TempDir()
	hubStore, err := store.Open(filepath.Join(dirA, "fort.db"))
	if err != nil {
		t.Fatalf("hub store.Open: %v", err)
	}
	defer hubStore.Close()

	live := &machines.Live{}
	// The hub's own cluster: local runtime is a fake; peers are hot-added.
	hubFake := fake.New()
	clus := cluster.New("hub", hubFake, nil)
	tokens := meshjoin.NewTokenStore("", dirA, "hub", "127.0.0.1:4087")
	regPath := filepath.Join(dirA, "machines.yaml")
	hubCapabilities := newMeshCapabilityRegistry(t, "hub", 0x11, "gpt-5.6-sol", now)
	capabilityCoordinator, err := control.NewCapabilityCoordinator(control.CapabilityCoordinatorOptions{
		Live: live, LocalName: "hub", Local: hubCapabilities,
		Peers: execcap.NewClientWithToken(tokens.Get), Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("new hub capability coordinator: %v", err)
	}
	// Record the exact Fort-owned profile before the gate lowers it to the
	// legacy provider selector; the gate itself wraps the real local/remote
	// cluster and performs a fresh exact-machine preflight before every target.
	conversationRuntime := &meshRecordingRuntime{next: execcap.NewProfileGate(clus, capabilityCoordinator)}
	conversationSeats := control.SnapshotConversationSeats{Source: capabilityCoordinator}
	conversationService := control.NewConversationService(hubStore, conversationRuntime, conversationSeats, dirA)
	defer conversationService.Close()

	meshSrv := &meshjoin.Server{
		Live:         live,
		RegistryPath: regPath,
		Managed:      true,
		Cluster:      clus,
		Store:        hubStore,
		Tokens:       tokens,
		NodeName:     "hub",
		Port:         "4087",
		ProbeAgents:  func() []string { return []string{"claude"} },
		Now:          func() time.Time { return now },
		Log:          slog.Default(),
	}
	roster := control.NewRoster(live)
	uiSrv := ui.New(ui.Deps{
		Store: hubStore, Machines: roster, Conversations: conversationService,
		Capabilities: capabilityCoordinator, SeatRechecker: capabilityCoordinator,
	})

	hubMux := http.NewServeMux()
	uiSrv.Register(hubMux)
	meshSrv.Register(hubMux)
	hubHTTP := httptest.NewServer(hubMux)
	defer hubHTTP.Close()

	// Helpers -----------------------------------------------------------------

	// doHub drives the hub mux directly so RemoteAddr is controllable (httptest
	// clients always source from 127.0.0.1, which validateWorkerURL rejects and
	// the derived-URL assertion needs a private CGNAT literal for).
	doHub := func(method, path, remoteAddr string, body any) *httptest.ResponseRecorder {
		t.Helper()
		var rd io.Reader
		if body != nil {
			b, err := json.Marshal(body)
			if err != nil {
				t.Fatalf("marshal body: %v", err)
			}
			rd = bytes.NewReader(b)
		}
		req := httptest.NewRequest(method, path, rd)
		if remoteAddr != "" {
			req.RemoteAddr = remoteAddr
		}
		rec := httptest.NewRecorder()
		hubMux.ServeHTTP(rec, req)
		return rec
	}
	decode := func(rec *httptest.ResponseRecorder) map[string]any {
		t.Helper()
		var m map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
			t.Fatalf("bad JSON %q: %v", rec.Body, err)
		}
		return m
	}
	// rosterNames fetches GET /api/machines over the real HTTP surface.
	rosterNames := func() []string {
		t.Helper()
		resp, err := http.Get(hubHTTP.URL + "/api/machines")
		if err != nil {
			t.Fatalf("GET /api/machines: %v", err)
		}
		defer resp.Body.Close()
		var list []ui.MachineStatus
		if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
			t.Fatalf("decode /api/machines: %v", err)
		}
		names := make([]string, len(list))
		for i, m := range list {
			names[i] = m.Name
		}
		return names
	}

	// =========================================================================
	// (1) INVITE — loopback admin, mints token + hub self-entry.
	// =========================================================================
	rec := doHub("POST", "/api/mesh/invite", "127.0.0.1:5555",
		map[string]any{"ttl": "", "advertise": "http://100.64.0.1:4087"})
	if rec.Code != http.StatusOK {
		t.Fatalf("invite: %d %s, want 200", rec.Code, rec.Body)
	}
	inv := decode(rec)
	if inv["minted"] != true {
		t.Fatalf("first invite minted = %v, want true", inv["minted"])
	}
	code := inv["code"].(string)

	// node.yaml written 0600 in the hub data dir.
	fi, err := os.Stat(filepath.Join(dirA, "node.yaml"))
	if err != nil {
		t.Fatalf("hub node.yaml not written: %v", err)
	}
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Fatalf("hub node.yaml mode = %o, want 0600", perm)
	}
	hubToken := tokens.Get()
	if hubToken == "" {
		t.Fatal("hub token not live after invite")
	}

	// Hub self-entry present in the registry with the probed agent.
	reg := live.Load()
	if reg == nil {
		t.Fatal("registry nil after invite")
	}
	hubEntry, ok := reg.Machine("hub")
	if !ok {
		t.Fatal("hub self-entry missing from registry")
	}
	if hubEntry.URL != "http://100.64.0.1:4087" || len(hubEntry.Agents) != 1 || hubEntry.Agents[0] != "claude" {
		t.Fatalf("hub entry = %+v, want url=http://100.64.0.1:4087 agents=[claude]", hubEntry)
	}
	if names := rosterNames(); len(names) != 1 || names[0] != "hub" {
		t.Fatalf("/api/machines after invite = %v, want [hub]", names)
	}

	// =========================================================================
	// (2) JOIN — private source, valid code → 200; derived URL; token echoed.
	// =========================================================================
	joinBody := map[string]any{
		"code": code, "port": 4087, "name": workerName,
		"agents": []string{"claude"}, "advertise_url": "",
	}
	rec = doHub("POST", "/api/mesh/join", "100.64.0.5:12345", joinBody)
	if rec.Code != http.StatusOK {
		t.Fatalf("join: %d %s, want 200", rec.Code, rec.Body)
	}
	jr := decode(rec)
	if jr["token"] != hubToken {
		t.Fatalf("join token = %v, want the hub token %q", jr["token"], hubToken)
	}
	const wantWorkerURL = "http://100.64.0.5:4087" // derived: observed source IP + advertised port
	if jr["url"] != wantWorkerURL {
		t.Fatalf("join url = %v, want %s (derived from source IP)", jr["url"], wantWorkerURL)
	}
	if jr["name"] != workerName {
		t.Fatalf("join name = %v, want %s", jr["name"], workerName)
	}

	// Registry + roster now list hub + worker.
	reg = live.Load()
	if len(reg.Machines) != 2 {
		t.Fatalf("registry after join = %v, want [hub worker]", reg.Names())
	}
	if names := rosterNames(); len(names) != 2 {
		t.Fatalf("/api/machines after join = %v, want [hub worker]", names)
	}

	// =========================================================================
	// (3) SECOND JOIN, SAME CODE — single-use → 401, no token in body.
	// =========================================================================
	body2 := map[string]any{
		"code": code, "port": 4087, "name": "intruder",
		"agents": []string{"claude"}, "advertise_url": "",
	}
	rec = doHub("POST", "/api/mesh/join", "100.64.0.6:1", body2)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("reused code: %d %s, want 401", rec.Code, rec.Body)
	}
	if bytes.Contains(rec.Body.Bytes(), []byte(hubToken)) {
		t.Fatalf("401 body leaked the mesh token: %s", rec.Body)
	}
	if _, ok := live.Load().Machine("intruder"); ok {
		t.Fatal("intruder registered despite a consumed code")
	}

	// =========================================================================
	// (4a) JOIN INSTALLED A HOT TRANSPORT — the "hot dispatch" headline. The
	//      join-installed transport points at the derived 100.64.0.5 address
	//      (unreachable here, and validateWorkerURL forbids advertising a
	//      127.0.0.1 httptest URL), so we can't stream through it. But we CAN
	//      prove it EXISTS: a dispatch to the worker name must fail trying to
	//      DIAL 100.64.0.5 — a transport-level error — and must NOT be the
	//      cluster "no route to machine" error (which is what a no-op join
	//      Cluster.Add would produce). This distinguishes "join installed a
	//      transport" from "no transport installed".
	dialCtx, dialCancel := context.WithTimeout(context.Background(), 2*time.Second)
	_, dialErr := clus.Dispatch(dialCtx, runtime.RunSpec{RunID: "r-dial", Machine: workerName})
	dialCancel()
	if dialErr == nil {
		t.Fatal("dispatch through the join-installed transport should fail (100.64.0.5 is unreachable)")
	}
	if strings.Contains(dialErr.Error(), "no route") {
		t.Fatalf("join did not install a transport: %v (got the cluster no-route error, want a dial failure to 100.64.0.5)", dialErr)
	}
	if !strings.Contains(dialErr.Error(), "100.64.0.5") {
		t.Fatalf("dial error should name the derived worker address 100.64.0.5: %v", dialErr)
	}

	// =========================================================================
	// (4b) CROSS-MACHINE DISPATCH — replace the join-installed transport with
	//      one pointing at the REAL httptest worker and prove a run crosses
	//      machines end to end (streaming the terminal event back).
	// =========================================================================
	// CLI-equivalent: the worker persists its identity so its node endpoint is
	// authorized with the hub token.
	if err := config.SaveNodeFile(dirB, config.NodeFile{Name: workerName, Token: hubToken, Addr: "0.0.0.0:4087"}); err != nil {
		t.Fatalf("worker SaveNodeFile: %v", err)
	}
	clus.Add(workerName, remote.New(workerName, workerSrv.URL, hubToken))
	// Keep the enrollment assertions above tied to the derived private URL, then
	// point this in-process test cluster's live transport registry at the real
	// worker server. Both capability discovery and execution now traverse that
	// authenticated node HTTP surface.
	workerEntry, ok := live.Load().Machine(workerName)
	if !ok {
		t.Fatal("worker missing before installing test transport URL")
	}
	workerEntry.URL = workerSrv.URL
	live.Store(live.Load().WithMachine(workerEntry))

	run, err := clus.Dispatch(context.Background(), runtime.RunSpec{
		RunID: "r-e2e", Agent: "claude", Prompt: "hello mesh", Machine: workerName,
	})
	if err != nil {
		t.Fatalf("cross-machine dispatch: %v", err)
	}
	var sawTerminal bool
	for ev := range run.Stream() {
		if ev.Type == runtime.EventExited {
			sawTerminal = true
		}
	}
	if st := run.Wait(); st.State != runtime.StateSucceeded {
		t.Fatalf("remote run state = %v, want succeeded", st.State)
	}
	if !sawTerminal {
		t.Fatal("no terminal exited event streamed back from the worker")
	}
	// The run actually reached the worker's fake runtime, and the node stripped
	// the placement label (Machine cleared) before running it locally.
	dispatched := workerFake.Dispatched()
	if len(dispatched) != 1 {
		t.Fatalf("worker received %d specs, want 1", len(dispatched))
	}
	if got := dispatched[0]; got.RunID != "r-e2e" || got.Machine != "" {
		t.Fatalf("worker spec = %+v, want RunID=r-e2e Machine=\"\" (node strips placement)", got)
	}

	// =========================================================================
	// (4c) SHARED CONVERSATION — one durable human turn fans out through the
	//      real local + remote transports and restores both attributed answers
	//      from the hub conversation API.
	// =========================================================================
	rec = doHub("GET", "/api/conversation-seats", "127.0.0.1:5555", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get unprobed mesh conversation seats: %d %s, want 200", rec.Code, rec.Body)
	}
	var seats []conversation.Seat
	if err := json.Unmarshal(rec.Body.Bytes(), &seats); err != nil {
		t.Fatalf("decode unprobed mesh conversation seats: %v", err)
	}
	if len(seats) != 0 {
		t.Fatalf("unprobed mesh conversation seats = %+v, want none before functional recheck", seats)
	}
	rec = doHub("POST", "/api/conversation-seats/recheck", "127.0.0.1:5555", map[string]any{})
	if rec.Code != http.StatusOK {
		t.Fatalf("recheck mesh conversation seats: %d %s, want 200", rec.Code, rec.Body)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &seats); err != nil {
		t.Fatalf("decode rechecked mesh conversation seats: %v", err)
	}
	findReadySeat := func(profile, agent, model, machine string) string {
		t.Helper()
		for _, seat := range seats {
			if seat.Profile != profile || seat.Machine != machine {
				continue
			}
			if !strings.HasPrefix(seat.ID, "seat:v1:") || strings.Contains(seat.ID, profile) ||
				strings.Contains(seat.ID, machine) || strings.Contains(seat.ID, model) ||
				seat.Agent != agent || seat.Model != model || seat.State != string(corecap.OfferReady) || seat.Reason != "" {
				t.Fatalf("rechecked exact seat = %+v, want ready %s/%s on %s", seat, agent, model, machine)
			}
			return seat.ID
		}
		t.Fatalf("rechecked seats do not contain %s on %s: %+v", profile, machine, seats)
		return ""
	}
	hubSeatID := findReadySeat("codex:configured-default", "codex", "gpt-5.6-sol", "hub")
	workerSeatID := findReadySeat("codex:configured-default", "codex", "gpt-5.6-terra", workerName)
	if hubSeatID == workerSeatID {
		t.Fatalf("model-versioned seats collided across machines: %q", hubSeatID)
	}
	// Capability discovery is evidence only: it must not invoke either runtime.
	if got := len(hubFake.Dispatched()); got != 0 {
		t.Fatalf("seat recheck dispatched %d hub runs, want 0", got)
	}
	if got := len(workerFake.Dispatched()); got != 1 {
		t.Fatalf("seat recheck changed worker dispatch count to %d, want only the prior raw dispatch", got)
	}
	if got := len(conversationRuntime.Dispatched()); got != 0 {
		t.Fatalf("seat recheck dispatched %d conversation targets, want 0", got)
	}
	rec = doHub("POST", "/api/conversations", "127.0.0.1:5555", map[string]any{
		"title":    "Mesh conversation",
		"seat_ids": []string{hubSeatID, workerSeatID},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create mesh conversation: %d %s, want 201", rec.Code, rec.Body)
	}
	var created store.ConversationDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode mesh conversation: %v", err)
	}
	if len(created.Participants) != 2 {
		t.Fatalf("mesh conversation participants = %+v, want two exact seats", created.Participants)
	}
	for _, participant := range created.Participants {
		wantModel := "gpt-5.6-sol"
		if participant.Machine == workerName {
			wantModel = "gpt-5.6-terra"
		}
		if participant.Profile != "codex:configured-default" || participant.Model != wantModel ||
			participant.SeatID != findReadySeat(participant.Profile, "codex", wantModel, participant.Machine) {
			t.Fatalf("persisted dynamic participant lost exact identity: %+v", participant)
		}
	}
	participantIDs := make([]string, 0, len(created.Participants))
	for _, participant := range created.Participants {
		participantIDs = append(participantIDs, participant.ID)
	}
	rec = doHub("POST", "/api/conversations/"+created.Conversation.ID+"/turns", "127.0.0.1:5555", map[string]any{
		"client_turn_id":  "mesh-conversation-turn",
		"text":            "Answer together across the mesh.",
		"participant_ids": participantIDs,
	})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("post mesh conversation turn: %d %s, want 202", rec.Code, rec.Body)
	}

	var restored store.ConversationDetail
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		rec = doHub("GET", "/api/conversations/"+created.Conversation.ID, "127.0.0.1:5555", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("restore mesh conversation: %d %s, want 200", rec.Code, rec.Body)
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &restored); err != nil {
			t.Fatalf("decode restored mesh conversation: %v", err)
		}
		answered := 0
		for _, target := range restored.Targets {
			if target.State == conversation.TargetAnswered {
				answered++
			}
		}
		if answered == 2 && len(restored.Messages) == 3 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(restored.Targets) != 2 || len(restored.Messages) != 3 {
		t.Fatalf("restored mesh conversation = %+v, want one human message and two answers", restored)
	}
	for _, target := range restored.Targets {
		if target.State != conversation.TargetAnswered {
			t.Fatalf("mesh conversation target = %+v, want answered", target)
		}
	}
	authors := map[string]bool{}
	for _, message := range restored.Messages {
		if message.AuthorKind == conversation.AuthorAssistant {
			authors[message.AuthorID] = true
		}
	}
	for _, participant := range restored.Participants {
		if !authors[participant.ID] {
			t.Fatalf("participant %s on %s has no attributed answer: %+v", participant.ID, participant.Machine, restored.Messages)
		}
	}
	hubSpecs := hubFake.Dispatched()
	if got := len(hubSpecs); got != 1 {
		t.Fatalf("hub received %d conversation specs, want 1", got)
	}
	workerSpecs := workerFake.Dispatched()
	if got := len(workerSpecs); got != 2 {
		t.Fatalf("worker received %d total specs, want raw dispatch plus one conversation target", got)
	}
	conversationSpecs := conversationRuntime.Dispatched()
	if got := len(conversationSpecs); got != 2 {
		t.Fatalf("conversation service dispatched %d specs, want 2", got)
	}
	var hubSpec, workerSpec runtime.RunSpec
	for _, spec := range conversationSpecs {
		switch spec.Machine {
		case "hub":
			hubSpec = spec
		case workerName:
			workerSpec = spec
		}
	}
	if hubSpec.Profile != "codex:configured-default" || hubSpec.Agent != "codex" || hubSpec.Model != "gpt-5.6-sol" {
		t.Fatalf("hub conversation dispatch lost exact seat: %+v", hubSpec)
	}
	if workerSpec.Profile != "codex:configured-default" || workerSpec.Agent != "codex" || workerSpec.Model != "gpt-5.6-terra" {
		t.Fatalf("worker conversation dispatch lost exact seat: %+v", workerSpec)
	}
	// Spec 039 intentionally lowers the Fort-owned profile before the legacy
	// node execution protocol. The exact seat remains in the durable run and
	// compiled prompt; the worker receives the exact provider selector and runs
	// locally after node strips the spent placement label.
	if got := workerSpecs[1]; got.Profile != "" || got.Agent != "codex" || got.Model != "gpt-5.6-terra" || got.Machine != "" {
		t.Fatalf("worker provider spec = %+v, want lowered exact selector and local placement", got)
	}
	if got := hubSpecs[0]; got.Profile != "" || got.Agent != "codex" || got.Model != "gpt-5.6-sol" || got.Machine != "hub" {
		t.Fatalf("hub provider spec = %+v, want lowered exact selector on persisted local placement", got)
	}
	type compiledPrompt struct {
		Participant struct {
			Profile string `json:"profile"`
			Machine string `json:"machine"`
		} `json:"participant"`
		Context json.RawMessage `json:"context"`
	}
	var hubPrompt, workerPrompt compiledPrompt
	if err := json.Unmarshal([]byte(hubSpec.Prompt), &hubPrompt); err != nil {
		t.Fatalf("decode hub conversation prompt: %v", err)
	}
	if err := json.Unmarshal([]byte(workerSpec.Prompt), &workerPrompt); err != nil {
		t.Fatalf("decode worker conversation prompt: %v", err)
	}
	if hubPrompt.Participant.Profile != "codex:configured-default" || hubPrompt.Participant.Machine != "hub" {
		t.Fatalf("hub prompt participant = %+v", hubPrompt.Participant)
	}
	if workerPrompt.Participant.Profile != "codex:configured-default" || workerPrompt.Participant.Machine != workerName {
		t.Fatalf("worker prompt participant = %+v", workerPrompt.Participant)
	}
	if !bytes.Equal(hubPrompt.Context, workerPrompt.Context) ||
		!bytes.Contains(hubPrompt.Context, []byte(`"body":"Answer together across the mesh."`)) ||
		!bytes.Contains(hubPrompt.Context, []byte(`"machine":"hub"`)) ||
		!bytes.Contains(hubPrompt.Context, []byte(`"machine":"worker"`)) {
		t.Fatalf("conversation targets did not receive one frozen exact context:\nhub: %s\nworker: %s", hubPrompt.Context, workerPrompt.Context)
	}

	// =========================================================================
	// (5) PLACEMENT-AFTER-JOIN — unpinned resolves the hub; pinned resolves worker.
	// =========================================================================
	// This verifies unpinned Place resolves to the hub as the FIRST machine in
	// registry order that offers the agent — NOT local-preference. A hot-created
	// registry has Registry.local=="" (meshjoin builds from &machines.Registry{}
	// via WithMachine, never machines.Parse), so Place skips the local branch and
	// falls through to first-in-registry-order; the hub wins because it was
	// inserted first at invite time. After a daemon restart, machines.Load
	// re-derives local from NodeName, so true local-preference and this
	// first-in-order behavior coincide for the hub (which is both first AND
	// local). Local-preference itself is unit-tested in core/machines.
	if got, err := live.Place("claude", ""); err != nil || got != "hub" {
		t.Fatalf("unpinned Place(claude) = %q, %v, want hub (first machine in registry order offering the agent)", got, err)
	}
	if got, err := live.Place("claude", workerName); err != nil || got != workerName {
		t.Fatalf("pinned Place(claude, worker) = %q, %v, want %s", got, err, workerName)
	}

	// =========================================================================
	// (6) REMOVE — loopback admin → 200 with the rotation warning; worker gone.
	// =========================================================================
	rec = doHub("DELETE", "/api/mesh/machines/"+workerName, "127.0.0.1:5555", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("remove: %d %s, want 200", rec.Code, rec.Body)
	}
	rr := decode(rec)
	wantWarn := workerName + " still holds the mesh token; rotate it to revoke access (see docs/notes/threat-model.md)"
	if rr["warning"] != wantWarn {
		t.Fatalf("remove warning = %v, want %q", rr["warning"], wantWarn)
	}
	// The cluster transport was torn down too: a dispatch to the removed worker
	// now fails with the cluster "no route to machine" error (a no-op
	// Cluster.Remove would leave the token-bearing transport installed and this
	// would instead attempt to dial it).
	if _, err := clus.Dispatch(context.Background(), runtime.RunSpec{RunID: "r-gone", Machine: workerName}); err == nil || !strings.Contains(err.Error(), "no route") {
		t.Fatalf("dispatch after remove = %v, want a cluster 'no route' error (transport must be torn down)", err)
	}
	// Placement to the worker now errors; roster no longer lists it.
	if _, err := live.Place("claude", workerName); err == nil {
		t.Fatal("Place to the removed worker should error")
	}
	if names := rosterNames(); len(names) != 1 || names[0] != "hub" {
		t.Fatalf("/api/machines after remove = %v, want [hub]", names)
	}
	if _, ok := live.Load().Machine(workerName); ok {
		t.Fatal("worker still in the live registry after remove")
	}
}
