package meshjoin

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tobsai/fort/core/config"
	"github.com/tobsai/fort/core/machines"
	"github.com/tobsai/fort/core/runtime"
	"github.com/tobsai/fort/core/store"
	"github.com/tobsai/fort/exec/cluster"
	"github.com/tobsai/fort/exec/fake"
)

// --- TokenStore ---

func TestTokenStoreEnsureMintsOnceAndPersists(t *testing.T) {
	dir := t.TempDir()
	ts := NewTokenStore("", dir, "hub", "127.0.0.1:4087")

	if got := ts.Get(); got != "" {
		t.Fatalf("Get before mint = %q, want empty", got)
	}

	tok, minted, err := ts.Ensure()
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if !minted {
		t.Fatal("first Ensure: minted = false, want true")
	}
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(tok) {
		t.Fatalf("token %q is not 32 bytes of hex", tok)
	}

	// Persisted to node.yaml, mode 0600.
	path := filepath.Join(dir, "node.yaml")
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("node.yaml not written: %v", err)
	}
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Fatalf("node.yaml mode = %o, want 0600", perm)
	}
	nf, err := config.ReadNodeFile(dir)
	if err != nil {
		t.Fatalf("ReadNodeFile: %v", err)
	}
	if nf.Name != "hub" || nf.Token != tok || nf.Addr != "127.0.0.1:4087" {
		t.Fatalf("node.yaml = %+v, want name=hub token=%s addr=127.0.0.1:4087", nf, tok)
	}

	// Second Ensure: same token, not minted again.
	tok2, minted2, err := ts.Ensure()
	if err != nil {
		t.Fatalf("second Ensure: %v", err)
	}
	if minted2 || tok2 != tok {
		t.Fatalf("second Ensure = (%q, %v), want (%q, false)", tok2, minted2, tok)
	}
	if got := ts.Get(); got != tok {
		t.Fatalf("Get after mint = %q, want %q", got, tok)
	}
}

func TestTokenStorePreconfiguredTokenIsNotReminted(t *testing.T) {
	dir := t.TempDir()
	ts := NewTokenStore("preset-token", dir, "hub", "0.0.0.0:4087")
	if got := ts.Get(); got != "preset-token" {
		t.Fatalf("Get = %q, want preset-token", got)
	}
	tok, minted, err := ts.Ensure()
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if minted || tok != "preset-token" {
		t.Fatalf("Ensure = (%q, %v), want (preset-token, false)", tok, minted)
	}
	if _, err := os.Stat(filepath.Join(dir, "node.yaml")); !os.IsNotExist(err) {
		t.Fatalf("node.yaml should not be written for a preconfigured token (stat err = %v)", err)
	}
}

// --- test harness ---

const hubAdvertise = "http://100.64.0.9:4087" // hermetic: no real interface probing

type harness struct {
	t      *testing.T
	dir    string
	srv    *Server
	mux    *http.ServeMux
	st     *store.Store
	live   *machines.Live
	clus   *cluster.Runtime
	tokens *TokenStore
	now    time.Time // settable per test; read via Server.Now
}

func newTestServer(t *testing.T) *harness {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "fort.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	h := &harness{
		t:      t,
		dir:    dir,
		st:     st,
		live:   &machines.Live{},
		clus:   cluster.New("hub", fake.New(), nil),
		tokens: NewTokenStore("", dir, "hub", "127.0.0.1:0"),
		now:    time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC),
	}
	h.srv = &Server{
		Live:         h.live,
		RegistryPath: filepath.Join(dir, "machines.yaml"),
		Managed:      true,
		Cluster:      h.clus,
		Store:        st,
		Tokens:       h.tokens,
		NodeName:     "hub",
		Port:         "4087",
		ProbeAgents:  func() []string { return []string{"claude"} },
		Now:          func() time.Time { return h.now },
		Log:          slog.Default(),
	}
	h.mux = http.NewServeMux()
	h.srv.Register(h.mux)
	return h
}

func (h *harness) do(method, path, remoteAddr string, body any) *httptest.ResponseRecorder {
	h.t.Helper()
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			h.t.Fatalf("marshal body: %v", err)
		}
		rd = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, rd)
	if remoteAddr != "" {
		req.RemoteAddr = remoteAddr
	}
	rec := httptest.NewRecorder()
	h.mux.ServeHTTP(rec, req)
	return rec
}

// invite mints a code from loopback with a hermetic advertise URL.
func (h *harness) invite() (code string, resp map[string]any) {
	h.t.Helper()
	rec := h.do("POST", "/api/mesh/invite", "127.0.0.1:555",
		map[string]any{"ttl": "", "advertise": hubAdvertise})
	if rec.Code != http.StatusOK {
		h.t.Fatalf("invite: %d %s", rec.Code, rec.Body)
	}
	resp = decode(h.t, rec)
	return resp["code"].(string), resp
}

func (h *harness) join(remoteAddr string, body map[string]any) *httptest.ResponseRecorder {
	h.t.Helper()
	return h.do("POST", "/api/mesh/join", remoteAddr, body)
}

func joinBody(code string) map[string]any {
	return map[string]any{
		"code": code, "port": 4087, "name": "mini",
		"agents": []string{"claude"}, "advertise_url": "",
	}
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("bad JSON response %q: %v", rec.Body, err)
	}
	return m
}

// hashOf mirrors the server's code normalization: uppercase, hyphens stripped,
// SHA-256 hex.
func hashOf(code string) string {
	norm := strings.ToUpper(strings.ReplaceAll(code, "-", ""))
	sum := sha256.Sum256([]byte(norm))
	return hex.EncodeToString(sum[:])
}

// assertNoToken fails if the (already minted) mesh token appears in the body.
func (h *harness) assertNoToken(rec *httptest.ResponseRecorder) {
	h.t.Helper()
	tok := h.tokens.Get()
	if tok == "" {
		h.t.Fatal("token not minted yet — no-token assertion is vacuous")
	}
	if strings.Contains(rec.Body.String(), tok) {
		h.t.Fatalf("response leaked the mesh token: %d %s", rec.Code, rec.Body)
	}
}

// routeErr dispatches with a canceled context: a known route errs on the
// transport ("context canceled"), an unknown one errs with "no route".
func (h *harness) routeErr(machine string) error {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := h.clus.Dispatch(ctx, runtime.RunSpec{Machine: machine})
	return err
}

func (h *harness) eventsOfType(typ string) []store.Event {
	h.t.Helper()
	evs, err := h.st.EventsSince(0)
	if err != nil {
		h.t.Fatalf("EventsSince: %v", err)
	}
	var out []store.Event
	for _, e := range evs {
		if e.Type == typ {
			out = append(out, e)
		}
	}
	return out
}

// --- loopbackOnly ---

func TestAdminEndpointsAreLoopbackOnly(t *testing.T) {
	h := newTestServer(t)

	// httptest's default RemoteAddr is 192.0.2.1:1234 — not loopback.
	if rec := h.do("POST", "/api/mesh/invite", "", map[string]any{"ttl": ""}); rec.Code != http.StatusForbidden {
		t.Fatalf("invite from non-loopback: %d, want 403", rec.Code)
	}
	if rec := h.do("DELETE", "/api/mesh/machines/mini", "", nil); rec.Code != http.StatusForbidden {
		t.Fatalf("remove from non-loopback: %d, want 403", rec.Code)
	}

	// Loopback v4 and v6 pass the gate.
	if rec := h.do("POST", "/api/mesh/invite", "127.0.0.1:555",
		map[string]any{"ttl": "", "advertise": hubAdvertise}); rec.Code != http.StatusOK {
		t.Fatalf("invite from 127.0.0.1: %d %s, want 200", rec.Code, rec.Body)
	}
	if rec := h.do("POST", "/api/mesh/invite", "[::1]:555",
		map[string]any{"ttl": "", "advertise": hubAdvertise}); rec.Code != http.StatusOK {
		t.Fatalf("invite from ::1: %d %s, want 200", rec.Code, rec.Body)
	}

	// join is NOT loopback-gated: a remote (private-network) caller passes
	// input validation and reaches the code check → 401 for a bad code, never
	// 403.
	if rec := h.join("100.64.0.5:12345", joinBody("0000-0000")); rec.Code != http.StatusUnauthorized {
		t.Fatalf("join from a non-loopback private source: %d, want 401 (bad code), not 403", rec.Code)
	}
}

// --- invite ---

func TestInviteTTLValidation(t *testing.T) {
	h := newTestServer(t)
	cases := []struct {
		ttl      string
		wantCode int
	}{
		{"", http.StatusOK}, // default 15m
		{"15m", http.StatusOK},
		{"1h", http.StatusOK}, // cap boundary is inclusive
		{"2h", http.StatusBadRequest},
		{"61m", http.StatusBadRequest},
		{"-5m", http.StatusBadRequest},
		{"nonsense", http.StatusBadRequest},
	}
	for _, c := range cases {
		rec := h.do("POST", "/api/mesh/invite", "127.0.0.1:555",
			map[string]any{"ttl": c.ttl, "advertise": hubAdvertise})
		if rec.Code != c.wantCode {
			t.Errorf("ttl %q: %d %s, want %d", c.ttl, rec.Code, rec.Body, c.wantCode)
		}
		if c.wantCode == http.StatusBadRequest && !strings.Contains(rec.Body.String(), "1h") {
			t.Errorf("ttl %q: 400 body %q does not mention the 1h cap", c.ttl, rec.Body)
		}
	}
}

func TestInviteDefaultTTLIs15m(t *testing.T) {
	h := newTestServer(t)
	code, _ := h.invite() // empty ttl
	hash := hashOf(code)
	if err := h.st.CheckInvite(hash, h.now.Add(14*time.Minute)); err != nil {
		t.Fatalf("invite should still be valid at +14m: %v", err)
	}
	if err := h.st.CheckInvite(hash, h.now.Add(16*time.Minute)); !errors.Is(err, store.ErrInviteExpired) {
		t.Fatalf("invite at +16m: %v, want ErrInviteExpired", err)
	}
}

func TestInviteRefusesOperatorManagedRegistry(t *testing.T) {
	h := newTestServer(t)
	h.srv.Managed = false
	rec := h.do("POST", "/api/mesh/invite", "127.0.0.1:555",
		map[string]any{"ttl": "", "advertise": hubAdvertise})
	if rec.Code != http.StatusConflict {
		t.Fatalf("unmanaged invite: %d %s, want 409", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "FORT_MACHINES") {
		t.Fatalf("409 body %q should mention FORT_MACHINES", rec.Body)
	}
	// Refused before minting or writing anything.
	if _, err := os.Stat(filepath.Join(h.dir, "node.yaml")); !os.IsNotExist(err) {
		t.Fatalf("node.yaml written on refused invite (stat err = %v)", err)
	}
	if _, err := os.Stat(h.srv.RegistryPath); !os.IsNotExist(err) {
		t.Fatalf("machines.yaml written on refused invite (stat err = %v)", err)
	}
}

func TestInviteMintsTokenOnceAndReportsIt(t *testing.T) {
	h := newTestServer(t)
	_, resp := h.invite()
	if resp["minted"] != true {
		t.Fatalf("first invite: minted = %v, want true", resp["minted"])
	}
	tok := h.tokens.Get()
	if tok == "" {
		t.Fatal("token not live after first invite")
	}
	fi, err := os.Stat(filepath.Join(h.dir, "node.yaml"))
	if err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("node.yaml after invite: err=%v mode=%v, want 0600", err, fi.Mode())
	}
	_, resp2 := h.invite()
	if resp2["minted"] != false {
		t.Fatalf("second invite: minted = %v, want false", resp2["minted"])
	}
	if h.tokens.Get() != tok {
		t.Fatal("token changed on second invite")
	}
}

func TestInviteResponseShape(t *testing.T) {
	h := newTestServer(t)
	code, resp := h.invite()
	// Crockford base32: no I, L, O, U; grouped XXXX-XXXX.
	if !regexp.MustCompile(`^[0-9A-HJKMNP-TV-Z]{4}-[0-9A-HJKMNP-TV-Z]{4}$`).MatchString(code) {
		t.Fatalf("code %q is not XXXX-XXXX Crockford base32", code)
	}
	if resp["hub_url"] != hubAdvertise {
		t.Fatalf("hub_url = %v, want %s", resp["hub_url"], hubAdvertise)
	}
	wantCmd := "fort mesh join " + hubAdvertise + " --code " + code
	if resp["join_cmd"] != wantCmd {
		t.Fatalf("join_cmd = %v, want %q", resp["join_cmd"], wantCmd)
	}
}

func TestInviteRegistersHubSelfEntry(t *testing.T) {
	h := newTestServer(t)
	h.invite()

	reg := h.live.Load()
	if reg == nil {
		t.Fatal("Live registry nil after invite")
	}
	m, ok := reg.Machine("hub")
	if !ok {
		t.Fatal("hub self-entry missing from live registry")
	}
	if m.URL != hubAdvertise || len(m.Agents) != 1 || m.Agents[0] != "claude" {
		t.Fatalf("hub entry = %+v, want url=%s agents=[claude]", m, hubAdvertise)
	}

	// Persisted and load-back-identical.
	onDisk, err := machines.Load(h.srv.RegistryPath, "hub")
	if err != nil {
		t.Fatalf("load managed registry: %v", err)
	}
	dm, ok := onDisk.Machine("hub")
	if !ok || dm.URL != m.URL {
		t.Fatalf("on-disk hub entry = %+v, %v", dm, ok)
	}

	// Second invite does not duplicate the entry.
	h.invite()
	if n := len(h.live.Load().Machines); n != 1 {
		t.Fatalf("machines after second invite = %d, want 1", n)
	}
}

// An agent-less hub claims no registry entry (the registry validator requires
// ≥1 agent per machine): the registry is created by the first join instead,
// and unpinned placement then goes to the registry's first entry.
func TestInviteOnAgentlessHubDefersRegistryToFirstJoin(t *testing.T) {
	h := newTestServer(t)
	h.srv.ProbeAgents = func() []string { return nil }

	code, _ := h.invite()
	if h.live.Load() != nil {
		t.Fatal("agent-less hub should not have created a registry")
	}
	if _, err := os.Stat(h.srv.RegistryPath); !os.IsNotExist(err) {
		t.Fatalf("machines.yaml should not exist yet (stat err = %v)", err)
	}

	rec := h.join("100.64.0.5:12345", joinBody(code))
	if rec.Code != http.StatusOK {
		t.Fatalf("join: %d %s", rec.Code, rec.Body)
	}
	reg := h.live.Load()
	if reg == nil || len(reg.Machines) != 1 || reg.Machines[0].Name != "mini" {
		t.Fatalf("registry after first join = %+v, want [mini]", reg)
	}
	if name, err := reg.Place("claude", ""); err != nil || name != "mini" {
		t.Fatalf("unpinned placement = %q, %v, want mini (registry-first-entry)", name, err)
	}
	if _, err := machines.Load(h.srv.RegistryPath, "hub"); err != nil {
		t.Fatalf("worker-only registry does not load back: %v", err)
	}
}

// --- join ---

func TestJoinInvalidCode401WithoutToken(t *testing.T) {
	h := newTestServer(t)
	h.invite() // mints the token; we join with a *different* code
	rec := h.join("100.64.0.5:12345", joinBody("0000-0000"))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("bad code: %d %s, want 401", rec.Code, rec.Body)
	}
	h.assertNoToken(rec)
}

func TestJoinExpiredCode410WithoutToken(t *testing.T) {
	h := newTestServer(t)
	code, _ := h.invite() // 15m default
	h.now = h.now.Add(16 * time.Minute)
	rec := h.join("100.64.0.5:12345", joinBody(code))
	if rec.Code != http.StatusGone {
		t.Fatalf("expired code: %d %s, want 410", rec.Code, rec.Body)
	}
	h.assertNoToken(rec)
}

func TestJoinValidationRejections(t *testing.T) {
	h := newTestServer(t)
	code, _ := h.invite()

	mod := func(k string, v any) map[string]any {
		b := joinBody(code)
		b[k] = v
		return b
	}
	cases := []struct {
		name       string
		remoteAddr string
		body       map[string]any
	}{
		{"empty name", "100.64.0.5:1", mod("name", "")},
		{"hub cannot join itself", "100.64.0.5:1", mod("name", "hub")},
		{"hub name case-insensitive", "100.64.0.5:1", mod("name", "HUB")},
		{"empty agents", "100.64.0.5:1", mod("agents", []string{})},
		{"port zero", "100.64.0.5:1", mod("port", 0)},
		{"port too big", "100.64.0.5:1", mod("port", 70000)},
		{"loopback advertise", "100.64.0.5:1", mod("advertise_url", "http://127.0.0.1:9")},
		{"https advertise", "100.64.0.5:1", mod("advertise_url", "https://10.0.0.1:9")},
		{"public advertise", "100.64.0.5:1", mod("advertise_url", "http://8.8.8.8:9")},
		{"public observed source", "8.8.8.8:1", joinBody(code)},
		{"loopback observed source", "127.0.0.1:44", joinBody(code)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := h.join(c.remoteAddr, c.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("%d %s, want 400", rec.Code, rec.Body)
			}
			h.assertNoToken(rec)
		})
	}

	// None of the 400s consumed the code: the same one still joins.
	rec := h.join("100.64.0.5:12345", joinBody(code))
	if rec.Code != http.StatusOK {
		t.Fatalf("join after 400s: %d %s, want 200 (code must not be consumed)", rec.Code, rec.Body)
	}
}

func TestJoinSuccess(t *testing.T) {
	h := newTestServer(t)
	code, _ := h.invite()

	rec := h.join("100.64.0.5:12345", joinBody(code))
	if rec.Code != http.StatusOK {
		t.Fatalf("join: %d %s", rec.Code, rec.Body)
	}
	resp := decode(t, rec)
	if resp["token"] != h.tokens.Get() || h.tokens.Get() == "" {
		t.Fatalf("token = %v, want the live mesh token", resp["token"])
	}
	if resp["name"] != "mini" {
		t.Fatalf("name = %v, want mini", resp["name"])
	}
	if resp["url"] != "http://100.64.0.5:4087" {
		t.Fatalf("url = %v, want http://100.64.0.5:4087 (observed source + advertised port)", resp["url"])
	}

	// Registry: hub + mini, live and on disk.
	reg := h.live.Load()
	if len(reg.Machines) != 2 {
		t.Fatalf("live machines = %v, want [hub mini]", reg.Names())
	}
	m, _ := reg.Machine("mini")
	if m.URL != "http://100.64.0.5:4087" || len(m.Agents) != 1 || m.Agents[0] != "claude" {
		t.Fatalf("mini entry = %+v", m)
	}
	onDisk, err := machines.Load(h.srv.RegistryPath, "hub")
	if err != nil || len(onDisk.Machines) != 2 {
		t.Fatalf("on-disk registry: %v, %v", err, onDisk)
	}

	// Cluster transport installed hot.
	if err := h.routeErr("mini"); err == nil || strings.Contains(err.Error(), "no route") {
		t.Fatalf("route to mini after join: %v, want transport error, not 'no route'", err)
	}
	if err := h.routeErr("ghost"); err == nil || !strings.Contains(err.Error(), "no route") {
		t.Fatalf("route to ghost: %v, want 'no route'", err)
	}

	// Code consumed; event appended with empty run_id.
	if err := h.st.CheckInvite(hashOf(code), h.now); !errors.Is(err, store.ErrInviteInvalid) {
		t.Fatalf("CheckInvite after join: %v, want ErrInviteInvalid (consumed)", err)
	}
	evs := h.eventsOfType("machine_joined")
	if len(evs) != 1 || evs[0].RunID != "" {
		t.Fatalf("machine_joined events = %+v, want one with empty run_id", evs)
	}
	if !strings.Contains(evs[0].Data, `"name":"mini"`) || !strings.Contains(evs[0].Data, "http://100.64.0.5:4087") {
		t.Fatalf("machine_joined data = %q", evs[0].Data)
	}
}

func TestJoinNormalizesCodeCaseAndHyphens(t *testing.T) {
	h := newTestServer(t)
	code, _ := h.invite()
	scrambled := strings.ToLower(strings.ReplaceAll(code, "-", ""))
	rec := h.join("100.64.0.5:12345", joinBody(scrambled))
	if rec.Code != http.StatusOK {
		t.Fatalf("join with %q: %d %s, want 200", scrambled, rec.Code, rec.Body)
	}
}

func TestJoinAdvertiseURLWins(t *testing.T) {
	h := newTestServer(t)
	code, _ := h.invite()
	body := joinBody(code)
	body["advertise_url"] = "http://10.1.2.3:5000"
	rec := h.join("100.64.0.5:12345", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("join: %d %s", rec.Code, rec.Body)
	}
	if resp := decode(t, rec); resp["url"] != "http://10.1.2.3:5000" {
		t.Fatalf("url = %v, want the advertise_url", resp["url"])
	}
	m, _ := h.live.Load().Machine("mini")
	if m.URL != "http://10.1.2.3:5000" {
		t.Fatalf("registered url = %q, want the advertise_url", m.URL)
	}
}

// DNS-rebinding defense: a hostname advertise_url that resolves to a private
// IP is stored/registered as the validated literal IP, not the hostname — so
// the transport connects to the exact address that passed validation and the
// attacker's DNS cannot flip it to a public host at dispatch time.
func TestJoinPinsHostnameAdvertiseURLToValidatedIP(t *testing.T) {
	h := newTestServer(t)
	code, _ := h.invite()
	h.srv.Resolve = func(string) ([]net.IP, error) { return []net.IP{net.ParseIP("10.2.3.4")}, nil }

	body := joinBody(code)
	body["advertise_url"] = "http://worker.local:5000"
	rec := h.join("100.64.0.5:12345", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("join: %d %s", rec.Code, rec.Body)
	}
	const want = "http://10.2.3.4:5000"
	if resp := decode(t, rec); resp["url"] != want {
		t.Fatalf("url = %v, want %s (pinned to the validated IP, not the hostname)", resp["url"], want)
	}
	m, _ := h.live.Load().Machine("mini")
	if m.URL != want {
		t.Fatalf("registered url = %q, want %s", m.URL, want)
	}
}

func TestJoinRejoinWithFreshCodeUpdatesEntry(t *testing.T) {
	h := newTestServer(t)
	code, _ := h.invite()
	if rec := h.join("100.64.0.5:12345", joinBody(code)); rec.Code != http.StatusOK {
		t.Fatalf("first join: %d %s", rec.Code, rec.Body)
	}

	// Same machine re-joins after an IP change, different casing, new agents.
	code2, _ := h.invite()
	body := joinBody(code2)
	body["name"] = "MINI"
	body["port"] = 5000
	body["agents"] = []string{"codex"}
	rec := h.join("10.0.0.7:999", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("re-join: %d %s", rec.Code, rec.Body)
	}
	if resp := decode(t, rec); resp["name"] != "mini" {
		t.Fatalf("re-join canonical name = %v, want mini", resp["name"])
	}

	reg := h.live.Load()
	if len(reg.Machines) != 2 { // hub + mini, no duplicate
		t.Fatalf("machines after re-join = %v, want exactly [hub mini]", reg.Names())
	}
	m, _ := reg.Machine("mini")
	if m.URL != "http://10.0.0.7:5000" || len(m.Agents) != 1 || m.Agents[0] != "codex" {
		t.Fatalf("mini after re-join = %+v, want updated url+agents", m)
	}
}

func TestJoinSequentialCodeReuseIs401(t *testing.T) {
	h := newTestServer(t)
	code, _ := h.invite()
	if rec := h.join("100.64.0.5:12345", joinBody(code)); rec.Code != http.StatusOK {
		t.Fatalf("first join: %d %s", rec.Code, rec.Body)
	}
	body := joinBody(code)
	body["name"] = "other"
	rec := h.join("10.0.0.8:5", body)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("reused code: %d %s, want 401", rec.Code, rec.Body)
	}
	h.assertNoToken(rec)
	if _, ok := h.live.Load().Machine("other"); ok {
		t.Fatal("machine registered despite consumed code")
	}
}

// Two distinct machines joining at the same time (two codes) must both end up
// registered: the registry read-modify-write is serialized, so neither join
// loses the other's entry (a lost update the atomic pointer alone would allow).
func TestConcurrentJoinsOfDistinctMachinesBothRegister(t *testing.T) {
	h := newTestServer(t)
	codeA, _ := h.invite()
	codeB, _ := h.invite()

	type result struct {
		name string
		code int
		body string
	}
	results := make(chan result, 2)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for _, j := range []struct{ code, name, addr string }{
		{codeA, "worker-a", "10.0.0.11:1"},
		{codeB, "worker-b", "10.0.0.12:1"},
	} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			body := joinBody(j.code)
			body["name"] = j.name
			rec := h.do("POST", "/api/mesh/join", j.addr, body)
			results <- result{j.name, rec.Code, rec.Body.String()}
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	for r := range results {
		if r.code != http.StatusOK {
			t.Fatalf("%s: %d %s", r.name, r.code, r.body)
		}
	}

	reg := h.live.Load()
	if len(reg.Machines) != 3 {
		t.Fatalf("machines = %v, want [hub worker-a worker-b]", reg.Names())
	}
	onDisk, err := machines.Load(h.srv.RegistryPath, "hub")
	if err != nil || len(onDisk.Machines) != 3 {
		t.Fatalf("on-disk registry lost an entry: %v, %v", err, onDisk)
	}
}

// CRITICAL (token exfiltration): two concurrent joins with the SAME valid code
// but DIFFERENT attacker-controlled names+urls. check-and-consume must be
// atomic, and the token-bearing transport must be installed only AFTER the
// code is consumed — so the loser installs NOTHING. Otherwise the loser's
// remote (holding the mesh token, pointed at the attacker URL) survives and a
// later run placed on that name ships the token off-box.
func TestConcurrentJoinsOfSameCodeAdmitExactlyOneNoLeak(t *testing.T) {
	h := newTestServer(t)
	code, _ := h.invite()

	type result struct {
		name string
		code int
		body string
	}
	results := make(chan result, 2)
	var wg sync.WaitGroup
	start := make(chan struct{})
	racers := []struct{ name, addr string }{
		{"good", "100.64.0.5:1"},
		{"evil", "10.9.9.9:1"},
	}
	for _, j := range racers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			body := joinBody(code)
			body["name"] = j.name
			<-start
			rec := h.do("POST", "/api/mesh/join", j.addr, body)
			results <- result{j.name, rec.Code, rec.Body.String()}
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	var winners, losers []string
	for r := range results {
		switch r.code {
		case http.StatusOK:
			winners = append(winners, r.name)
			if !strings.Contains(r.body, h.tokens.Get()) {
				t.Fatalf("winner %s: 200 body should carry the token", r.name)
			}
		case http.StatusUnauthorized:
			losers = append(losers, r.name)
			if strings.Contains(r.body, h.tokens.Get()) {
				t.Fatalf("loser %s leaked the token in its 401: %s", r.name, r.body)
			}
		default:
			t.Fatalf("%s: unexpected status %d %s", r.name, r.code, r.body)
		}
	}
	if len(winners) != 1 || len(losers) != 1 {
		t.Fatalf("winners=%v losers=%v, want exactly one of each", winners, losers)
	}

	loser := losers[0]
	// The loser must not be in the live registry...
	if _, ok := h.live.Load().Machine(loser); ok {
		t.Fatalf("loser %s survived in the live registry — token-bearing ghost entry", loser)
	}
	// ...and no token-bearing transport may have been installed for it: a
	// dispatch to the loser's name must fail with a no-route error.
	if err := h.routeErr(loser); err == nil || !strings.Contains(err.Error(), "no route") {
		t.Fatalf("route to loser %s: %v, want 'no route' (no transport installed)", loser, err)
	}
	// Exactly one join event, for the winner.
	evs := h.eventsOfType("machine_joined")
	if len(evs) != 1 || !strings.Contains(evs[0].Data, `"name":"`+winners[0]+`"`) {
		t.Fatalf("machine_joined events = %+v, want exactly one for the winner", evs)
	}
}

func TestJoinRegistryWriteFailureKeepsCodeValid(t *testing.T) {
	h := newTestServer(t)
	code, _ := h.invite()

	// Force machines.Save to fail: point the registry into a "directory" that
	// is actually a file.
	blocked := filepath.Join(h.dir, "blocked")
	if err := os.WriteFile(blocked, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	goodPath := h.srv.RegistryPath
	h.srv.RegistryPath = filepath.Join(blocked, "machines.yaml")

	rec := h.join("100.64.0.5:12345", joinBody(code))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("join with failing save: %d %s, want 500", rec.Code, rec.Body)
	}
	h.assertNoToken(rec)
	if err := h.st.CheckInvite(hashOf(code), h.now); err != nil {
		t.Fatalf("code must survive a failed registry write: %v", err)
	}
	if err := h.routeErr("mini"); err == nil || !strings.Contains(err.Error(), "no route") {
		t.Fatalf("no transport should be installed on failure: %v", err)
	}
	if evs := h.eventsOfType("machine_joined"); len(evs) != 0 {
		t.Fatalf("no machine_joined event expected on failure, got %+v", evs)
	}

	// Same code succeeds once the path is writable again.
	h.srv.RegistryPath = goodPath
	if rec := h.join("100.64.0.5:12345", joinBody(code)); rec.Code != http.StatusOK {
		t.Fatalf("join after restoring path: %d %s, want 200", rec.Code, rec.Body)
	}
}

func TestJoinRefusesOperatorManagedRegistry(t *testing.T) {
	h := newTestServer(t)
	code, _ := h.invite()
	h.srv.Managed = false
	rec := h.join("100.64.0.5:12345", joinBody(code))
	if rec.Code != http.StatusConflict {
		t.Fatalf("unmanaged join: %d %s, want 409", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "FORT_MACHINES") {
		t.Fatalf("409 body %q should mention FORT_MACHINES", rec.Body)
	}
	h.assertNoToken(rec)
	if err := h.st.CheckInvite(hashOf(code), h.now); err != nil {
		t.Fatalf("code must not be consumed by a refused join: %v", err)
	}
	h.srv.Managed = true
	if rec := h.join("100.64.0.5:12345", joinBody(code)); rec.Code != http.StatusOK {
		t.Fatalf("join after re-enabling: %d %s", rec.Code, rec.Body)
	}
}

// --- remove ---

func TestRemoveUnknownMachine404(t *testing.T) {
	h := newTestServer(t)
	// With no registry at all.
	if rec := h.do("DELETE", "/api/mesh/machines/ghost", "127.0.0.1:555", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("remove with nil registry: %d, want 404", rec.Code)
	}
	// With a registry that lacks the name.
	h.invite()
	if rec := h.do("DELETE", "/api/mesh/machines/ghost", "127.0.0.1:555", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("remove unknown: %d, want 404", rec.Code)
	}
}

func TestRemoveHubOwnEntry400(t *testing.T) {
	h := newTestServer(t)
	h.invite()
	for _, name := range []string{"hub", "HUB"} {
		if rec := h.do("DELETE", "/api/mesh/machines/"+name, "127.0.0.1:555", nil); rec.Code != http.StatusBadRequest {
			t.Fatalf("remove %s: %d, want 400", name, rec.Code)
		}
	}
}

func TestRemoveRefusesOperatorManagedRegistry(t *testing.T) {
	h := newTestServer(t)
	code, _ := h.invite()
	if rec := h.join("100.64.0.5:12345", joinBody(code)); rec.Code != http.StatusOK {
		t.Fatalf("join: %d %s", rec.Code, rec.Body)
	}
	h.srv.Managed = false
	rec := h.do("DELETE", "/api/mesh/machines/mini", "127.0.0.1:555", nil)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "FORT_MACHINES") {
		t.Fatalf("unmanaged remove: %d %s, want 409 mentioning FORT_MACHINES", rec.Code, rec.Body)
	}
	if _, ok := h.live.Load().Machine("mini"); !ok {
		t.Fatal("machine removed despite operator-managed registry")
	}
}

func TestRemoveSuccess(t *testing.T) {
	h := newTestServer(t)
	code, _ := h.invite()
	if rec := h.join("100.64.0.5:12345", joinBody(code)); rec.Code != http.StatusOK {
		t.Fatalf("join: %d %s", rec.Code, rec.Body)
	}

	rec := h.do("DELETE", "/api/mesh/machines/mini", "127.0.0.1:555", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("remove: %d %s", rec.Code, rec.Body)
	}
	want := "mini still holds the mesh token; rotate it to revoke access (see docs/notes/threat-model.md)"
	if resp := decode(t, rec); resp["warning"] != want {
		t.Fatalf("warning = %v, want %q", resp["warning"], want)
	}

	// Gone from the live registry, the disk registry, and the cluster.
	if _, ok := h.live.Load().Machine("mini"); ok {
		t.Fatal("mini still in live registry")
	}
	onDisk, err := machines.Load(h.srv.RegistryPath, "hub")
	if err != nil || len(onDisk.Machines) != 1 || onDisk.Machines[0].Name != "hub" {
		t.Fatalf("on-disk registry after remove: %v, %v", err, onDisk)
	}
	if err := h.routeErr("mini"); err == nil || !strings.Contains(err.Error(), "no route") {
		t.Fatalf("route to mini after remove: %v, want 'no route'", err)
	}

	evs := h.eventsOfType("machine_removed")
	if len(evs) != 1 || evs[0].RunID != "" || !strings.Contains(evs[0].Data, `"name":"mini"`) {
		t.Fatalf("machine_removed events = %+v", evs)
	}
}
