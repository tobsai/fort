package relay_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/tobsai/fort/exec/relay"
	"github.com/tobsai/fort/exec/relay/secure"
)

// --- fake broker ---------------------------------------------------------
//
// broker accepts ONE daemon socket at a time (asserting the device token),
// then pumps frames off it. It exposes helpers so the test can act as a
// gateway client: open a session (relay hs1/hs2), then exchange sealed
// frames. Every frame the broker relays (either direction) is captured so a
// test can prove ciphertext-only relay.

type broker struct {
	srv   *httptest.Server
	token string

	mu     sync.Mutex // guards daemon
	daemon *websocket.Conn
	wmu    sync.Mutex // serializes writes to the daemon

	attaches chan struct{} // one signal per daemon connection

	routeMu sync.Mutex
	routes  map[string]chan relay.Frame // stream -> frames for that client

	recMu  sync.Mutex
	frames []relay.Frame // every relayed frame, both directions
}

func newBroker(t *testing.T, token string) *broker {
	b := &broker{
		token:    token,
		attaches: make(chan struct{}, 8),
		routes:   map[string]chan relay.Frame{},
	}
	b.srv = httptest.NewServer(http.HandlerFunc(b.handle))
	t.Cleanup(b.srv.Close)
	return b
}

func (b *broker) url() string { return "ws" + strings.TrimPrefix(b.srv.URL, "http") }

func (b *broker) handle(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") != "Bearer "+b.token {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return
	}
	c.SetReadLimit(1 << 22)
	b.mu.Lock()
	b.daemon = c
	b.mu.Unlock()
	select {
	case b.attaches <- struct{}{}:
	default:
	}
	ctx := context.Background()
	for {
		var f relay.Frame
		if err := wsjson.Read(ctx, c, &f); err != nil {
			return
		}
		b.record(f)
		b.deliver(f)
	}
}

func (b *broker) record(f relay.Frame) {
	b.recMu.Lock()
	b.frames = append(b.frames, f)
	b.recMu.Unlock()
}

func (b *broker) deliver(f relay.Frame) {
	b.routeMu.Lock()
	ch := b.routes[f.Stream]
	b.routeMu.Unlock()
	if ch != nil {
		ch <- f
	}
}

func (b *broker) toDaemon(ctx context.Context, f relay.Frame) error {
	b.record(f)
	b.mu.Lock()
	d := b.daemon
	b.mu.Unlock()
	b.wmu.Lock()
	defer b.wmu.Unlock()
	return wsjson.Write(ctx, d, f)
}

func (b *broker) waitAttach(t *testing.T) {
	t.Helper()
	select {
	case <-b.attaches:
	case <-time.After(3 * time.Second):
		t.Fatal("daemon never attached")
	}
}

func (b *broker) dropDaemon() {
	b.mu.Lock()
	d := b.daemon
	b.mu.Unlock()
	if d != nil {
		d.CloseNow()
	}
}

// assertNoPlaintext proves the broker only ever saw ciphertext: no relayed
// non-handshake payload contains needle.
func (b *broker) assertNoPlaintext(t *testing.T, needle string) {
	t.Helper()
	b.recMu.Lock()
	defer b.recMu.Unlock()
	seen := 0
	for _, f := range b.frames {
		if f.Kind == "hs1" || f.Kind == "hs2" {
			continue
		}
		seen++
		raw, err := base64.StdEncoding.DecodeString(f.B64)
		if err != nil {
			continue
		}
		if bytes.Contains(raw, []byte(needle)) {
			t.Fatalf("relayed %q frame leaked plaintext %q", f.Kind, needle)
		}
	}
	if seen == 0 {
		t.Fatal("no sealed frames were relayed; nothing was proven")
	}
}

// --- fake client (initiator) ---------------------------------------------

type client struct {
	b      *broker
	stream string
	in     chan relay.Frame
	sess   *secure.Session
}

func (b *broker) newClient(stream string) *client {
	ch := make(chan relay.Frame, 64)
	b.routeMu.Lock()
	b.routes[stream] = ch
	b.routeMu.Unlock()
	return &client{b: b, stream: stream, in: ch}
}

func (c *client) handshake(ctx context.Context, key secure.Keypair, daemonPub []byte) error {
	init, err := secure.NewInitiator(key, daemonPub)
	if err != nil {
		return err
	}
	m1, err := init.WriteMessage(nil)
	if err != nil {
		return err
	}
	if err := c.b.toDaemon(ctx, relay.Frame{Stream: c.stream, Kind: "hs1", B64: enc(m1)}); err != nil {
		return err
	}
	f, err := c.recv(ctx)
	if err != nil {
		return err
	}
	if f.Kind != "hs2" {
		return fmt.Errorf("expected hs2, got %q", f.Kind)
	}
	m2, err := dec(f.B64)
	if err != nil {
		return err
	}
	if _, err := init.ReadMessage(m2); err != nil {
		return err
	}
	if c.sess = init.Session(); c.sess == nil {
		return fmt.Errorf("no session after handshake")
	}
	return nil
}

func (c *client) sendSealed(ctx context.Context, kind string, payload []byte) error {
	ct := c.sess.Seal(payload)
	return c.b.toDaemon(ctx, relay.Frame{Stream: c.stream, Kind: kind, B64: enc(ct)})
}

func (c *client) recv(ctx context.Context) (relay.Frame, error) {
	select {
	case f := <-c.in:
		return f, nil
	case <-ctx.Done():
		return relay.Frame{}, ctx.Err()
	}
}

func (c *client) open(f relay.Frame) ([]byte, error) {
	raw, err := dec(f.B64)
	if err != nil {
		return nil, err
	}
	return c.sess.Open(raw)
}

func enc(b []byte) string          { return base64.StdEncoding.EncodeToString(b) }
func dec(s string) ([]byte, error) { return base64.StdEncoding.DecodeString(s) }

// --- tests ---------------------------------------------------------------

// TestDialAuthAndProxyRoundTrip: the transport dials with the bearer token, a
// client pins the daemon's public key and handshakes, a sealed GET round-trips
// to a real handler, and the broker proves it only relayed ciphertext.
func TestDialAuthAndProxyRoundTrip(t *testing.T) {
	daemonKey, _ := secure.GenerateKeypair()
	clientKey, _ := secure.GenerateKeypair()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/summary", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"execution":true}`))
	})

	b := newBroker(t, "dev-token")
	tr := relay.New(mux, relay.Config{
		URL:        b.url(),
		Token:      "dev-token",
		Key:        daemonKey,
		MinBackoff: 50 * time.Millisecond,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	runDone := make(chan struct{})
	go func() { _ = tr.Run(ctx); close(runDone) }()

	b.waitAttach(t)

	cl := b.newClient("s1")
	if err := cl.handshake(ctx, clientKey, daemonKey.Public); err != nil {
		t.Fatalf("handshake: %v", err)
	}

	req := mustJSON(relay.ReqPayload{ID: "r1", Method: "GET", Path: "/api/summary"})
	if err := cl.sendSealed(ctx, "req", req); err != nil {
		t.Fatalf("send req: %v", err)
	}
	f, err := cl.recv(ctx)
	if err != nil {
		t.Fatalf("recv res: %v", err)
	}
	if f.Kind != "res" {
		t.Fatalf("kind = %q, want res", f.Kind)
	}
	pt, err := cl.open(f)
	if err != nil {
		t.Fatalf("open res: %v", err)
	}
	var rp relay.ResPayload
	if err := json.Unmarshal(pt, &rp); err != nil {
		t.Fatalf("unmarshal res: %v", err)
	}
	if rp.Status != http.StatusOK {
		t.Fatalf("status = %d, want 200", rp.Status)
	}
	if !bytes.Contains(rp.Body, []byte("execution")) {
		t.Fatalf("body = %q, want it to contain execution", rp.Body)
	}

	// The broker never saw the plaintext: only ciphertext was relayed.
	b.assertNoPlaintext(t, "execution")

	cancel()
	<-runDone
}

// TestCommandPostRoundTrip pins the native command path, not just read-only
// snapshots: method, JSON body, and content type must survive the sealed relay
// and the command response must return through the reverse cipher direction.
func TestCommandPostRoundTrip(t *testing.T) {
	daemonKey, _ := secure.GenerateKeypair()
	clientKey, _ := secure.GenerateKeypair()
	const command = `{"text":"repair the provider","task_type":"bug","plan_gate":false}`
	const result = `{"kind":"assignment","run_id":"run-command"}`

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/chat", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", got)
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != command {
			t.Errorf("command body = %q, want %q", body, command)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(result))
	})

	b := newBroker(t, "command-token")
	tr := relay.New(mux, relay.Config{
		URL: b.url(), Token: "command-token", Key: daemonKey, MinBackoff: 50 * time.Millisecond,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	runDone := make(chan struct{})
	go func() { _ = tr.Run(ctx); close(runDone) }()
	b.waitAttach(t)

	cl := b.newClient("command-stream")
	if err := cl.handshake(ctx, clientKey, daemonKey.Public); err != nil {
		t.Fatalf("handshake: %v", err)
	}
	req := mustJSON(relay.ReqPayload{
		ID: "command-1", Method: "POST", Path: "/api/chat",
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    []byte(command),
	})
	if err := cl.sendSealed(ctx, "req", req); err != nil {
		t.Fatalf("send command: %v", err)
	}
	frame, err := cl.recv(ctx)
	if err != nil {
		t.Fatalf("receive command response: %v", err)
	}
	plaintext, err := cl.open(frame)
	if err != nil {
		t.Fatalf("open command response: %v", err)
	}
	var response relay.ResPayload
	if err := json.Unmarshal(plaintext, &response); err != nil {
		t.Fatalf("decode command response: %v", err)
	}
	if response.Status != http.StatusOK || string(response.Body) != result {
		t.Fatalf("command response = status %d body %q", response.Status, response.Body)
	}
	b.assertNoPlaintext(t, "repair the provider")

	cancel()
	<-runDone
}

// TestSSEStreamsAsChunks: a flushing SSE handler streams two events as chunks
// without the request ending; a sealed end cancels the handler's context.
func TestSSEStreamsAsChunks(t *testing.T) {
	daemonKey, _ := secure.GenerateKeypair()
	clientKey, _ := secure.GenerateKeypair()

	handlerDone := make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("/api/events", func(w http.ResponseWriter, r *http.Request) {
		fl, ok := w.(http.Flusher)
		if !ok {
			t.Error("streaming ResponseWriter is not a http.Flusher")
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: one\n\n"))
		fl.Flush()
		_, _ = w.Write([]byte("data: two\n\n"))
		fl.Flush()
		<-r.Context().Done()
		close(handlerDone)
	})

	b := newBroker(t, "tok")
	tr := relay.New(mux, relay.Config{URL: b.url(), Token: "tok", Key: daemonKey, MinBackoff: 50 * time.Millisecond})
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	runDone := make(chan struct{})
	go func() { _ = tr.Run(ctx); close(runDone) }()

	b.waitAttach(t)

	cl := b.newClient("s1")
	if err := cl.handshake(ctx, clientKey, daemonKey.Public); err != nil {
		t.Fatalf("handshake: %v", err)
	}

	req := mustJSON(relay.ReqPayload{
		ID: "r1", Method: "GET", Path: "/api/events",
		Headers: map[string]string{"accept": "text/event-stream"},
	})
	if err := cl.sendSealed(ctx, "req", req); err != nil {
		t.Fatalf("send req: %v", err)
	}

	// First frame: res{Stream:true}, no body.
	f, err := cl.recv(ctx)
	if err != nil {
		t.Fatalf("recv res: %v", err)
	}
	if f.Kind != "res" {
		t.Fatalf("first frame kind = %q, want res", f.Kind)
	}
	pt, err := cl.open(f)
	if err != nil {
		t.Fatalf("open res: %v", err)
	}
	var rp relay.ResPayload
	_ = json.Unmarshal(pt, &rp)
	if !rp.Stream {
		t.Fatalf("res.Stream = false, want true (SSE)")
	}

	// Then two chunks, without the request ending.
	var got []string
	for len(got) < 2 {
		f, err := cl.recv(ctx)
		if err != nil {
			t.Fatalf("recv chunk: %v", err)
		}
		if f.Kind != "chunk" {
			t.Fatalf("kind = %q, want chunk", f.Kind)
		}
		pt, err := cl.open(f)
		if err != nil {
			t.Fatalf("open chunk: %v", err)
		}
		var cp relay.ChunkPayload
		_ = json.Unmarshal(pt, &cp)
		got = append(got, string(cp.Data))
	}
	if !strings.Contains(got[0], "one") || !strings.Contains(got[1], "two") {
		t.Fatalf("chunks = %q, want [one two]", got)
	}

	// The handler is still blocked; the request has not ended.
	select {
	case <-handlerDone:
		t.Fatal("handler ended before the end frame was sent")
	case <-time.After(100 * time.Millisecond):
	}

	// A sealed end cancels the streaming handler's context.
	if err := cl.sendSealed(ctx, "end", mustJSON(relay.ChunkPayload{ID: "r1"})); err != nil {
		t.Fatalf("send end: %v", err)
	}
	select {
	case <-handlerDone:
	case <-time.After(3 * time.Second):
		t.Fatal("handler context not canceled after end frame (goroutine leak)")
	}

	cancel()
	<-runDone
}

// TestReconnectAfterDrop: the broker drops the socket after one round-trip;
// the transport redials and a fresh session on the new socket round-trips.
func TestReconnectAfterDrop(t *testing.T) {
	daemonKey, _ := secure.GenerateKeypair()
	clientKey, _ := secure.GenerateKeypair()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("pong"))
	})

	b := newBroker(t, "tok")
	tr := relay.New(mux, relay.Config{URL: b.url(), Token: "tok", Key: daemonKey, MinBackoff: 20 * time.Millisecond})
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	runDone := make(chan struct{})
	go func() { _ = tr.Run(ctx); close(runDone) }()

	b.waitAttach(t)
	roundTrip(t, ctx, b, "s1", clientKey, daemonKey.Public)

	// Broker drops the daemon socket; the transport must redial.
	b.dropDaemon()
	b.waitAttach(t)
	roundTrip(t, ctx, b, "s2", clientKey, daemonKey.Public)

	cancel()
	<-runDone
}

// TestWrongTokenRejected: a bad token is 401'd; no session ever establishes,
// and ctx cancel ends Run cleanly despite the retry loop.
func TestWrongTokenRejected(t *testing.T) {
	daemonKey, _ := secure.GenerateKeypair()

	b := newBroker(t, "correct-token")
	tr := relay.New(http.NewServeMux(), relay.Config{
		URL: b.url(), Token: "wrong-token", Key: daemonKey, MinBackoff: 20 * time.Millisecond,
	})
	ctx, cancel := context.WithCancel(context.Background())
	runDone := make(chan struct{})
	go func() { _ = tr.Run(ctx); close(runDone) }()

	// Despite repeated retries, no daemon socket ever attaches.
	select {
	case <-b.attaches:
		t.Fatal("daemon attached despite a wrong token")
	case <-time.After(500 * time.Millisecond):
	}

	cancel()
	select {
	case <-runDone:
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not return after ctx cancel")
	}
}

func roundTrip(t *testing.T, ctx context.Context, b *broker, stream string, key secure.Keypair, daemonPub []byte) {
	t.Helper()
	cl := b.newClient(stream)
	if err := cl.handshake(ctx, key, daemonPub); err != nil {
		t.Fatalf("handshake %s: %v", stream, err)
	}
	req := mustJSON(relay.ReqPayload{ID: "r", Method: "GET", Path: "/api/ping"})
	if err := cl.sendSealed(ctx, "req", req); err != nil {
		t.Fatalf("send %s: %v", stream, err)
	}
	f, err := cl.recv(ctx)
	if err != nil {
		t.Fatalf("recv %s: %v", stream, err)
	}
	if f.Kind != "res" {
		t.Fatalf("%s kind = %q, want res", stream, f.Kind)
	}
	pt, err := cl.open(f)
	if err != nil {
		t.Fatalf("open %s: %v", stream, err)
	}
	var rp relay.ResPayload
	_ = json.Unmarshal(pt, &rp)
	if rp.Status != http.StatusOK || !bytes.Contains(rp.Body, []byte("pong")) {
		t.Fatalf("%s res = %d %q", stream, rp.Status, rp.Body)
	}
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
