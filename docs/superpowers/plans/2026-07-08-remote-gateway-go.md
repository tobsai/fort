# Remote Gateway — Go side (spec 028, part 1 of 2) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The daemon side of the 028 gateway: an E2E crypto unit (Noise IK + XChaCha20-Poly1305), an outbound WebSocket relay transport that serves Fort's own HTTP/SSE mux through the tunnel, `fort relay join|status|remove`, and `relay.yaml` config — all fully testable against an in-process fake broker (no cloud deploy needed).

**Architecture:** `exec/relay/secure` is the Fort-owned crypto contract (keys, fingerprints, handshake, AEAD framing) — both ends of every test use it, proving round-trip. `exec/relay` dials out (device-token auth), completes a Noise IK handshake per client session relayed opaquely by the broker, then serves sealed HTTP/SSE frames by invoking an injected `http.Handler` (the same in-process mux `fort serve` builds — the transport never imports `ui`). `cmd/fort` wires it when `relay.yaml` exists. Part 2 (separate plan) builds `gateway/worker` + `gateway/web`; the frame schema defined here IS the wire contract part 2 implements.

**Tech Stack + NEW dependencies (flagged per Fort's minimal-deps discipline):**
- `github.com/flynn/noise` — the standard Go Noise Protocol implementation (Noise IK); Fort-owned wrapper keeps it swappable.
- `golang.org/x/crypto` — already an indirect dep; used directly for chacha20poly1305 constants via flynn/noise's cipher suite.
- `github.com/coder/websocket` — minimal, context-first WebSocket client/server (successor of nhooyr.io/websocket). Used by the transport and the test broker.

---

## Wire contract (part 2 implements the same shapes)

Every WebSocket message is one JSON envelope:

```json
{"stream":"<id>","kind":"<kind>","b64":"<base64 payload>"}
```

Kinds, daemon-perspective:
- `hs1` (inbound) / `hs2` (outbound) — Noise IK handshake messages for a client session; `stream` = the session id chosen by the broker. Payloads relayed opaquely by the broker.
- `req` (inbound, sealed) — one HTTP request for an established session. Plaintext (after `Open`) is JSON: `{"id":"<request id>","method":"GET","path":"/api/board","headers":{"accept":"…"},"body":"<base64>"}`.
- `res` (outbound, sealed) — `{"id":"…","status":200,"headers":{…},"body":"<base64>"}` for buffered responses.
- `chunk` (outbound, sealed) — `{"id":"…","data":"<base64>"}` streaming piece (SSE); first chunk is preceded by a `res` with `"stream":true` and no body.
- `end` (either direction, sealed for client-origin) — `{"id":"…"}` closes a streaming request.
- `bye` (inbound, plaintext) — the broker dropped the client session; free its state.

Session model: `stream` identifies a client session (one Noise handshake each); `id` inside sealed payloads multiplexes concurrent requests within a session. The daemon keeps `sessions map[string]*secure.Session`.

---

## File Structure

- `exec/relay/secure/secure.go` — keys, fingerprint, handshake, session Seal/Open.
- `exec/relay/secure/secure_test.go` — round-trip, tamper, wrong-key, passive-observer tests.
- `exec/relay/relay.go` — the transport: dial, auth, session handshakes, request serving, SSE streaming, reconnect backoff.
- `exec/relay/frame.go` — the envelope + payload types above (one place; part 2 mirrors it).
- `exec/relay/relay_test.go` — fake broker (in-process WS server) + fake client (uses `secure` as initiator) driving full round-trips.
- `core/config/relay.go` (+ test) — `RelayConfig` load/save (0600) in `DataDir()`.
- `cmd/fort/relay.go` — `fort relay join|status|remove`.
- `cmd/fort/main.go` — usage lines + serve wiring (dial when configured).

---

### Task 1: `exec/relay/secure` (TDD)

- [ ] **Step 1.1: deps**

```bash
cd /Users/tobiasgunn/dev/fort && go get github.com/flynn/noise@latest github.com/coder/websocket@latest && go mod tidy
```

- [ ] **Step 1.2: failing tests** — create `exec/relay/secure/secure_test.go`:

```go
package secure

import (
	"bytes"
	"testing"
)

// handshake drives initiator/responder to completion, returning both sessions.
func handshake(t *testing.T, daemon, client Keypair, pinnedPub []byte) (cli, srv *Session, err error) {
	t.Helper()
	init, err := NewInitiator(client, pinnedPub)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := NewResponder(daemon)
	if err != nil {
		t.Fatal(err)
	}
	m1, err := init.WriteMessage(nil)
	if err != nil {
		return nil, nil, err
	}
	if _, err = resp.ReadMessage(m1); err != nil {
		return nil, nil, err
	}
	m2, err := resp.WriteMessage(nil)
	if err != nil {
		return nil, nil, err
	}
	if _, err = init.ReadMessage(m2); err != nil {
		return nil, nil, err
	}
	return init.Session(), resp.Session(), nil
}

func TestHandshakeAndRoundTrip(t *testing.T) {
	d, _ := GenerateKeypair()
	c, _ := GenerateKeypair()
	cli, srv, err := handshake(t, d, c, d.Public)
	if err != nil {
		t.Fatalf("handshake: %v", err)
	}
	ct := cli.Seal([]byte("GET /api/board"))
	if bytes.Contains(ct, []byte("api/board")) {
		t.Fatal("ciphertext leaks plaintext")
	}
	pt, err := srv.Open(ct)
	if err != nil || string(pt) != "GET /api/board" {
		t.Fatalf("open: %q err=%v", pt, err)
	}
	// and the reverse direction
	ct2 := srv.Seal([]byte("200 ok"))
	pt2, err := cli.Open(ct2)
	if err != nil || string(pt2) != "200 ok" {
		t.Fatalf("reverse: %q err=%v", pt2, err)
	}
}

func TestTamperedFrameRejected(t *testing.T) {
	d, _ := GenerateKeypair()
	c, _ := GenerateKeypair()
	cli, srv, err := handshake(t, d, c, d.Public)
	if err != nil {
		t.Fatal(err)
	}
	ct := cli.Seal([]byte("secret"))
	ct[len(ct)-1] ^= 0x01
	if _, err := srv.Open(ct); err == nil {
		t.Fatal("tampered ciphertext must be rejected")
	}
}

func TestWrongPinnedKeyFailsHandshake(t *testing.T) {
	d, _ := GenerateKeypair()
	evil, _ := GenerateKeypair() // a MITM broker's substituted key
	c, _ := GenerateKeypair()
	if _, _, err := handshake(t, d, c, evil.Public); err == nil {
		t.Fatal("handshake against a non-pinned static key must fail")
	}
}

func TestFingerprintStable(t *testing.T) {
	k, _ := GenerateKeypair()
	f1, f2 := k.Fingerprint(), k.Fingerprint()
	if f1 == "" || f1 != f2 || len(f1) < 12 {
		t.Fatalf("fingerprint %q/%q", f1, f2)
	}
	other, _ := GenerateKeypair()
	if other.Fingerprint() == f1 {
		t.Fatal("distinct keys must have distinct fingerprints")
	}
}

func TestPassiveObserverCannotDecrypt(t *testing.T) {
	d, _ := GenerateKeypair()
	c, _ := GenerateKeypair()
	cli, _, err := handshake(t, d, c, d.Public)
	if err != nil {
		t.Fatal(err)
	}
	// an observer with only the daemon's PUBLIC key and the frames
	obs, _ := GenerateKeypair()
	_, obsSess, err := handshake(t, obs, c, obs.Public) // unrelated session
	if err != nil {
		t.Fatal(err)
	}
	ct := cli.Seal([]byte("private"))
	if pt, err := obsSess.Open(ct); err == nil {
		t.Fatalf("observer decrypted: %q", pt)
	}
}
```

Run: `go test ./exec/relay/secure/` → FAIL (package missing).

- [ ] **Step 1.3: implement** — create `exec/relay/secure/secure.go`:

```go
// Package secure is Fort's E2E crypto contract for the relay (spec 028): a
// Noise IK handshake (X25519) between a client and the daemon's pinned static
// key, then XChaCha-family AEAD framing. The gateway broker relays these frames
// opaquely — it can neither read nor forge them. Both ends of Fort's tests use
// this package, proving the contract round-trips.
package secure

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"errors"

	"github.com/flynn/noise"
)

// suite: X25519 DH, ChaChaPoly AEAD, BLAKE2s hash — the WireGuard family.
var suite = noise.NewCipherSuite(noise.DH25519, noise.CipherChaChaPoly, noise.HashBLAKE2s)

// Keypair is a long-term X25519 static identity.
type Keypair struct {
	Private []byte
	Public  []byte
}

// GenerateKeypair mints a fresh static identity.
func GenerateKeypair() (Keypair, error) {
	dh, err := suite.GenerateKeypair(rand.Reader)
	if err != nil {
		return Keypair{}, err
	}
	return Keypair{Private: dh.Private, Public: dh.Public}, nil
}

// Fingerprint is the human-comparable identity of a public key: base32
// (no padding) of sha256(pub), grouped for reading. Shown by `fort relay
// join` and on the gateway machine list; clients pin the key it names.
func (k Keypair) Fingerprint() string { return FingerprintOf(k.Public) }

// FingerprintOf fingerprints any public key.
func FingerprintOf(pub []byte) string {
	sum := sha256.Sum256(pub)
	s := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(sum[:10])
	// group as xxxx-xxxx-xxxx-xxxx for reading
	out := make([]byte, 0, len(s)+3)
	for i, c := range []byte(s) {
		if i > 0 && i%4 == 0 {
			out = append(out, '-')
		}
		out = append(out, c)
	}
	return string(out)
}

// Handshake is one side of a Noise IK handshake.
type Handshake struct {
	hs   *noise.HandshakeState
	init bool
	sess *Session
}

// NewInitiator starts the client side, pinning the daemon's static public key
// (IK: the initiator must know the responder's key — a substituted key fails).
func NewInitiator(static Keypair, pinnedPeerPub []byte) (*Handshake, error) {
	hs, err := noise.NewHandshakeState(noise.Config{
		CipherSuite:   suite,
		Pattern:       noise.HandshakeIK,
		Initiator:     true,
		StaticKeypair: noise.DHKey{Private: static.Private, Public: static.Public},
		PeerStatic:    pinnedPeerPub,
	})
	if err != nil {
		return nil, err
	}
	return &Handshake{hs: hs, init: true}, nil
}

// NewResponder starts the daemon side with its static identity.
func NewResponder(static Keypair) (*Handshake, error) {
	hs, err := noise.NewHandshakeState(noise.Config{
		CipherSuite:   suite,
		Pattern:       noise.HandshakeIK,
		StaticKeypair: noise.DHKey{Private: static.Private, Public: static.Public},
	})
	if err != nil {
		return nil, err
	}
	return &Handshake{hs: hs}, nil
}

// WriteMessage produces the next handshake message (payload may be nil).
func (h *Handshake) WriteMessage(payload []byte) ([]byte, error) {
	msg, cs1, cs2, err := h.hs.WriteMessage(nil, payload)
	if err != nil {
		return nil, err
	}
	h.finish(cs1, cs2)
	return msg, nil
}

// ReadMessage consumes the peer's handshake message.
func (h *Handshake) ReadMessage(msg []byte) ([]byte, error) {
	payload, cs1, cs2, err := h.hs.ReadMessage(nil, msg)
	if err != nil {
		return nil, err
	}
	h.finish(cs1, cs2)
	return payload, nil
}

func (h *Handshake) finish(cs1, cs2 *noise.CipherState) {
	if cs1 == nil || cs2 == nil {
		return
	}
	// Noise convention: cs1 encrypts initiator->responder, cs2 the reverse.
	if h.init {
		h.sess = &Session{enc: cs1, dec: cs2}
	} else {
		h.sess = &Session{enc: cs2, dec: cs1}
	}
}

// Session returns the transport session once the handshake completed (nil before).
func (h *Handshake) Session() *Session { return h.sess }

// Session seals/opens transport frames after a completed handshake.
type Session struct {
	enc *noise.CipherState
	dec *noise.CipherState
}

// Seal encrypts one frame.
func (s *Session) Seal(plaintext []byte) []byte {
	ct, err := s.enc.Encrypt(nil, nil, plaintext)
	if err != nil {
		// CipherState.Encrypt errs only on nonce exhaustion (2^64 frames).
		panic("secure: seal: " + err.Error())
	}
	return ct
}

// Open decrypts one frame, rejecting any tampering.
func (s *Session) Open(ciphertext []byte) ([]byte, error) {
	if s == nil {
		return nil, errors.New("secure: no session")
	}
	return s.dec.Decrypt(nil, nil, ciphertext)
}
```

- [ ] **Step 1.4: run — expect PASS**: `go test ./exec/relay/secure/ -race -v` (all 5). If flynn/noise's API differs (e.g. `Encrypt` signature without error), adapt minimally and note it — the TESTS are the contract.

- [ ] **Step 1.5: commit**

```bash
git add exec/relay/secure/ go.mod go.sum
git commit -m "feat(relay): secure — Noise IK + AEAD session unit, pinned-key E2E (spec 028)"
```

---

### Task 2: frames + the transport (TDD against a fake broker)

- [ ] **Step 2.1: frame types** — create `exec/relay/frame.go`:

```go
package relay

// Frame is the single WebSocket envelope of the gateway wire contract
// (spec 028). The broker routes on Stream/Kind and never sees inside B64
// once a session is sealed.
type Frame struct {
	Stream string `json:"stream"`
	Kind   string `json:"kind"` // hs1|hs2|req|res|chunk|end|bye
	B64    string `json:"b64,omitempty"`
}

// ReqPayload is the sealed plaintext of a "req" frame.
type ReqPayload struct {
	ID      string            `json:"id"`
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    []byte            `json:"body,omitempty"`
}

// ResPayload is the sealed plaintext of a "res" frame.
type ResPayload struct {
	ID      string            `json:"id"`
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    []byte            `json:"body,omitempty"`
	Stream  bool              `json:"stream,omitempty"` // true => chunks follow
}

// ChunkPayload is the sealed plaintext of a "chunk" frame (SSE piece).
type ChunkPayload struct {
	ID   string `json:"id"`
	Data []byte `json:"data,omitempty"`
	End  bool   `json:"end,omitempty"`
}
```

- [ ] **Step 2.2: failing transport test** — create `exec/relay/relay_test.go` with a fake broker + fake client. Complete requirements the test must assert (write these as real tests; the broker is ~60 lines using `coder/websocket` in an `httptest.Server`):

```go
package relay_test

// The fake broker accepts ONE daemon socket (asserting the Authorization
// header carries the device token) and exposes helper methods for the test
// to act as a gateway client: open a session (relay hs1/hs2 frames), then
// exchange sealed frames. The fake CLIENT side uses exec/relay/secure as
// the initiator with the daemon's real public key (pinning), proving E2E.

// TestDialAuthAndProxyRoundTrip:
//  - handler: mux with GET /api/summary -> {"execution":true}
//  - start fake broker; transport.Run(ctx) with cfg{URL, token, keypair}
//  - broker sees Authorization: Bearer <token> (else 401s and the test fails)
//  - client opens session s1: initiator handshake via hs1/hs2 -> Session
//  - client seals ReqPayload{ID:"r1",Method:"GET",Path:"/api/summary"} as kind req
//  - daemon responds kind res sealed; client Opens it; assert status 200 and
//    body contains "execution"
//  - the BROKER captures every frame it relayed: assert none of the relayed
//    b64-decoded payloads (other than hs1/hs2) contain the plaintext
//    "execution" (ciphertext-only relay, asserted at the broker)

// TestSSEStreamsAsChunks:
//  - handler: GET /api/events writes "data: one\n\n", flushes, writes
//    "data: two\n\n", flushes, then blocks until request context done
//  - client sends sealed req for /api/events with Headers{"accept":"text/event-stream"}
//  - daemon replies res{Stream:true}, then chunk frames; client Opens chunks
//    and asserts it receives both "one" and "two" without the request ending
//  - client sends sealed end{ID}; daemon cancels the handler context (the
//    blocked handler returns; no goroutine leak — assert via done channel)

// TestReconnectAfterDrop:
//  - broker closes the daemon socket after the first successful round-trip
//  - transport redials (backoff floor ~100ms in tests via cfg.MinBackoff)
//  - a NEW client session on the second socket round-trips successfully

// TestWrongTokenRejected:
//  - broker 401s a bad token; transport.Run returns/logs and retries;
//    assert no session ever establishes (bounded wait), ctx cancel ends Run.
```

Write the four tests fully (they define the transport's public API surface):

```go
// Transport public API the tests compile against:
//   cfg := relay.Config{URL: broker.URL, Token: "dev-token", Key: daemonKey,
//                       MinBackoff: 50 * time.Millisecond}
//   tr := relay.New(handler, cfg)     // handler: the in-process http.Handler
//   go tr.Run(ctx)                    // maintains the outbound socket until ctx ends
```

Run: `go test ./exec/relay/` → FAIL (package relay missing).

- [ ] **Step 2.3: implement** — create `exec/relay/relay.go`. Required behavior (implement completely; ~250 lines):

```go
// Package relay maintains fort serve's outbound tunnel to the 028 gateway:
// one WebSocket to the broker, per-client-session Noise IK handshakes
// (exec/relay/secure), and sealed HTTP/SSE service against an injected
// http.Handler — the transport never imports ui (seam: it moves bytes).
//
// Config{URL, Token, Key, MinBackoff}; New(handler, cfg) *Transport;
// (t *Transport) Run(ctx) error — reconnect loop with exponential backoff
// (MinBackoff..30s, jittered), until ctx is done.
//
// Per inbound frame:
//   hs1  -> NewResponder(cfg.Key) for that stream; ReadMessage(payload);
//           WriteMessage -> reply kind hs2; store Session on completion.
//   req  -> sess.Open -> ReqPayload -> serve:
//             non-stream: httptest.NewRecorder over the handler; reply res.
//             stream (Accept: text/event-stream): spawn goroutine with a
//             cancelable context; a flushWriter ResponseWriter seals+sends a
//             res{Stream:true} on WriteHeader, then chunk frames on each
//             Flush/Write; register cancel under (stream,id) for "end".
//   end  -> sess.Open -> ChunkPayload/ReqPayload id -> cancel that request.
//   bye  -> drop the stream's session + cancel its in-flight requests.
// Writes to the socket are serialized with a mutex (concurrent SSE goroutines).
// A dropped socket cancels all in-flight requests and re-dials.
```

Key implementation notes (bake into code):
- Use `websocket.Dial(ctx, cfg.URL, &websocket.DialOptions{HTTPHeader: http.Header{"Authorization": {"Bearer " + cfg.Token}}})`.
- Read loop: `wsjson.Read` into `Frame` (or `websocket.MessageText` + `json.Unmarshal`).
- Serve non-stream requests with `httptest.NewRecorder()` + `http.NewRequestWithContext`.
- The streaming ResponseWriter implements `http.ResponseWriter` + `http.Flusher`; buffered writes are cut into chunk frames on Flush.
- Never let a panicking handler kill the transport: `defer recover()` around handler invocations, reply 500. (The mux is Fort's own, but the tunnel must outlive a bug.)

Run: `go test ./exec/relay/ -race` → all 4 PASS. Iterate until green.

- [ ] **Step 2.4: commit**

```bash
git add exec/relay/
git commit -m "feat(relay): outbound tunnel transport — sealed HTTP/SSE over one socket (spec 028)"
```

---

### Task 3: config + CLI + serve wiring

- [ ] **Step 3.1: failing config test** — create `core/config/relay_test.go`:

```go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRelayConfigRoundTripAndPerms(t *testing.T) {
	dir := t.TempDir()
	rc := RelayConfig{GatewayURL: "https://gw.example", DeviceToken: "tok",
		MachineID: "m1", PrivateKey: []byte{1, 2}, PublicKey: []byte{3, 4}}
	if err := SaveRelay(dir, rc); err != nil {
		t.Fatal(err)
	}
	got, err := LoadRelay(dir)
	if err != nil || got.GatewayURL != rc.GatewayURL || got.DeviceToken != "tok" ||
		string(got.PrivateKey) != string(rc.PrivateKey) {
		t.Fatalf("got %+v err=%v", got, err)
	}
	fi, _ := os.Stat(filepath.Join(dir, "relay.yaml"))
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("relay.yaml perms = %v, want 0600", fi.Mode().Perm())
	}
	// absent file -> ok=false error sentinel, not a hard error
	if _, err := LoadRelay(t.TempDir()); err == nil {
		t.Fatal("missing relay.yaml should error (os.IsNotExist-compatible)")
	}
}
```

- [ ] **Step 3.2: implement `core/config/relay.go`** — `RelayConfig{GatewayURL, DeviceToken, MachineID string; PrivateKey, PublicKey []byte}` with yaml tags (keys base64 via yaml `!!binary` default handling or explicit base64 strings — pick explicit base64 string fields marshaled in Save/Load for clarity); `SaveRelay(dir, rc)` writes `relay.yaml` 0600; `LoadRelay(dir)` reads it (missing file returns the os.IsNotExist error). Follow the style of the existing node.yaml handling in this package (read it first).

Run → PASS. Then: `go test ./core/config/ -race`.

- [ ] **Step 3.3: CLI** — create `cmd/fort/relay.go` (mirror `cmd/fort/mesh.go`'s and `breakdown.go`'s patterns — loopback-free, talks to the REMOTE gateway):

```go
// fort relay join <gateway-url> --code XXXX-XXXX [--name N]
//   1. GenerateKeypair (secure)
//   2. POST <gateway-url>/api/relay/join {"code":..,"name":..,"public_key":<b64>}
//      -> 200 {"device_token":"..","machine_id":".."} (else print server body)
//   3. SaveRelay(cfg.DataDir(), RelayConfig{...})
//   4. Print: machine id, gateway URL, and the key FINGERPRINT with a note to
//      verify it matches the gateway's machine list.
// fort relay status  — prints gateway URL, machine id, fingerprint (from relay.yaml), or "not joined".
// fort relay remove  — deletes relay.yaml locally and best-effort DELETE
//   <gateway-url>/api/relay/machines/<id> with the device token; prints a note
//   that gateway-side revocation is authoritative.
```

Name resolution defaults to the mesh node-name/hostname logic used by `mesh join` (read that code and reuse its helper if exported, else replicate the default). Add the three commands to `cmd/fort/main.go`'s usage text and command switch (mirror how `mesh` dispatches). Test: an `httptest.Server` standing in for the gateway asserts the join POST shape and returns a token; the test then checks `relay.yaml` exists 0600 with the token and that a reused code (server returns 409) surfaces the server's message. Put it in `cmd/fort/relay_test.go`.

- [ ] **Step 3.4: serve wiring** — in `cmd/fort/main.go` `cmdServe`, after the mux is composed (the `mount := func(mux …)` block builds `uiSrv`/`nodeSrv`/`meshSrv`; the actual `*http.ServeMux` lives inside `core/server` — so build the relay handler the same way `server.New` mounts: create a `http.NewServeMux()`, call `mount(m)`, and hand THAT to the transport):

```go
	// Remote gateway (spec 028): when this machine has joined a gateway,
	// maintain the outbound tunnel and serve the same mux through it.
	if rc, err := config.LoadRelay(a.cfg.DataDir()); err == nil {
		rmux := http.NewServeMux()
		mount(rmux)
		tr := relay.New(rmux, relay.Config{
			URL:   rc.GatewayURL + "/tunnel",
			Token: rc.DeviceToken,
			Key:   secure.Keypair{Private: rc.PrivateKey, Public: rc.PublicKey},
		})
		go func() { _ = tr.Run(ctx) }()
		fmt.Printf("fort relay: tunnel to %s (machine %s, fingerprint %s)\n",
			rc.GatewayURL, rc.MachineID, secure.FingerprintOf(rc.PublicKey))
	}
```

(Verify the exact URL path `/tunnel` matches frame.go's doc — part 2 implements it.) Note the composition-root rule: `cmd/fort` may import `exec/relay`; nothing in `core`/`ui` does.

- [ ] **Step 3.5: full gates + commit**

```bash
go test ./... -count=1 && go test -race ./exec/relay/... ./core/config/ && go vet ./...
go list -deps ./ui | grep -E 'exec/relay' || echo "ui clean of relay"
go list -deps ./exec/relay | grep -E 'core/router|core/engine|core/graph|tobsai/fort/ui' || echo "relay seam clean"
git add cmd/fort/ core/config/
git commit -m "feat(cmd,config): fort relay join/status/remove + serve tunnel wiring (spec 028)"
```

Expected: green, `ui clean of relay`, `relay seam clean`.

---

## Self-review

**Spec coverage (Go side):** outbound-only socket + device token (T2 dial/auth); Noise IK + pinning + AEAD with the exact test criteria from spec 028 (handshake key agreement, tamper rejection, wrong-key rejection, broker-can't-read asserted AT the broker) (T1, T2 round-trip); SSE streaming through the tunnel (T2); reconnect/backoff (T2); join → keypair + token + fingerprint printed, relay.yaml 0600, reused code surfaces server error (T3); revocation → socket drop handled as reconnect-with-401 (T2 wrong-token test) and `relay remove` (T3); determinism/seams — transport takes an opaque `http.Handler`, imports no ui/router/engine, asserted by `go list -deps` (T3.5). The wire contract (frames) is written down once in `frame.go` for part 2 to mirror.
**Placeholder scan:** T2.2/T2.3 specify the four tests and the transport behavior as contracts with exact API/assertions rather than full listings (the fake broker + transport are ~300 lines; the contracts are complete and unambiguous — every kind, payload shape, header, error path, and assertion is stated). No TBDs.
**Type consistency:** `secure.Keypair/Handshake/Session` used identically in T1/T2/T3; `relay.Config{URL,Token,Key,MinBackoff}` matches the test API and serve wiring; `Frame/ReqPayload/ResPayload/ChunkPayload` names consistent; `config.RelayConfig` fields match the CLI writes and serve reads.
