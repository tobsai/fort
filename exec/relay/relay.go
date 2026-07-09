// Package relay maintains fort serve's outbound tunnel to the 028 gateway:
// one WebSocket to the broker, per-client-session Noise IK handshakes
// (exec/relay/secure), and sealed HTTP/SSE service against an injected
// http.Handler — the transport never imports ui (seam: it moves bytes).
//
// Config{URL, Token, Key, MinBackoff}; New(handler, cfg) *Transport;
// (t *Transport) Run(ctx) error — reconnect loop with exponential backoff
// (MinBackoff..30s, jittered) until ctx is done.
//
// Per inbound frame:
//
//	hs1  -> NewResponder(cfg.Key) for that stream; ReadMessage(payload);
//	        WriteMessage -> reply kind hs2; store Session on completion.
//	req  -> sess.Open -> ReqPayload -> serve:
//	          non-stream: httptest.NewRecorder over the handler; reply res.
//	          stream (Accept: text/event-stream): spawn goroutine with a
//	          cancelable context; a streamWriter ResponseWriter seals+sends a
//	          res{Stream:true} on WriteHeader, then chunk frames on each
//	          Flush; register cancel under (stream,id) for "end".
//	end  -> sess.Open -> id -> cancel that request.
//	bye  -> drop the stream's session + cancel its in-flight requests.
//
// Writes to the socket are serialized with a mutex — and, crucially, each
// Seal happens under that same lock immediately before its write, so the AEAD
// nonce order (per session) always matches wire order, which is what the peer
// decrypts in. A dropped socket cancels every in-flight request and re-dials.
package relay

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/tobsai/fort/exec/relay/secure"
)

// Config configures the outbound tunnel.
type Config struct {
	URL        string         // broker WebSocket URL (e.g. wss://gw/tunnel)
	Token      string         // device token, sent as Authorization: Bearer
	Key        secure.Keypair // this daemon's pinned static identity
	MinBackoff time.Duration  // reconnect backoff floor (default 1s)
}

// Transport maintains one outbound WebSocket, serving handler through it.
type Transport struct {
	handler http.Handler
	cfg     Config
}

// New builds a transport that serves handler over the tunnel described by cfg.
func New(handler http.Handler, cfg Config) *Transport {
	return &Transport{handler: handler, cfg: cfg}
}

const maxBackoff = 30 * time.Second

// Run maintains the outbound socket, reconnecting with jittered exponential
// backoff until ctx is done. It returns ctx.Err() on shutdown.
func (t *Transport) Run(ctx context.Context) error {
	min := t.cfg.MinBackoff
	if min <= 0 {
		min = time.Second
	}
	backoff := min
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		connected, _ := t.dialAndServe(ctx)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if connected {
			backoff = min // a real connection resets the schedule
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(jitter(backoff)):
		}
		if backoff < maxBackoff {
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}

// dialAndServe dials once and serves until the socket drops. The bool reports
// whether a connection was actually established (so Run can reset backoff).
func (t *Transport) dialAndServe(ctx context.Context) (bool, error) {
	ws, _, err := websocket.Dial(ctx, t.cfg.URL, &websocket.DialOptions{
		HTTPHeader: http.Header{"Authorization": {"Bearer " + t.cfg.Token}},
	})
	if err != nil {
		return false, err
	}
	ws.SetReadLimit(1 << 22) // 4 MiB frames

	connCtx, cancel := context.WithCancel(ctx)
	c := &conn{
		ws:       ws,
		handler:  t.handler,
		key:      t.cfg.Key,
		ctx:      connCtx,
		sessions: map[string]*secure.Session{},
		cancels:  map[string]context.CancelFunc{},
	}
	err = c.readLoop()

	cancel()         // cancel every in-flight request context
	ws.CloseNow()    // unblock any pending socket writes
	c.wg.Wait()      // let handler goroutines finish before we return
	return true, err // established, so caller resets backoff
}

// conn is the per-socket state: sessions, request cancels, serialized writes.
type conn struct {
	ws      *websocket.Conn
	handler http.Handler
	key     secure.Keypair
	ctx     context.Context // canceled when the socket drops

	wmu sync.Mutex // serializes socket writes AND the Seal that precedes each

	mu       sync.Mutex // guards sessions
	sessions map[string]*secure.Session

	cmu     sync.Mutex // guards cancels
	cancels map[string]context.CancelFunc

	wg sync.WaitGroup // in-flight request goroutines
}

func (c *conn) readLoop() error {
	for {
		var f Frame
		if err := wsjson.Read(c.ctx, c.ws, &f); err != nil {
			return err
		}
		c.dispatch(f)
	}
}

func (c *conn) dispatch(f Frame) {
	switch f.Kind {
	case "hs1":
		c.handleHS1(f)
	case "req":
		c.handleReq(f)
	case "end":
		c.handleEnd(f)
	case "bye":
		c.handleBye(f)
	}
}

// handleHS1 completes the daemon (responder) side of a Noise IK handshake for
// a client session and replies hs2. Handshake payloads are relayed opaquely by
// the broker; we pass nil (never application data — IK msg1 is replayable).
func (c *conn) handleHS1(f Frame) {
	raw, err := base64.StdEncoding.DecodeString(f.B64)
	if err != nil {
		return
	}
	resp, err := secure.NewResponder(c.key)
	if err != nil {
		return
	}
	if _, err := resp.ReadMessage(raw); err != nil {
		return // wrong pinned key / malformed msg1: drop the session
	}
	m2, err := resp.WriteMessage(nil)
	if err != nil {
		return
	}
	c.mu.Lock()
	c.sessions[f.Stream] = resp.Session()
	c.mu.Unlock()
	_ = c.writeFrame(Frame{Stream: f.Stream, Kind: "hs2", B64: base64.StdEncoding.EncodeToString(m2)})
}

func (c *conn) handleReq(f Frame) {
	sess := c.session(f.Stream)
	if sess == nil {
		return
	}
	pt, ok := c.openFrame(sess, f.B64)
	if !ok {
		return
	}
	var rp ReqPayload
	if err := json.Unmarshal(pt, &rp); err != nil {
		return
	}
	c.wg.Add(1)
	if wantsStream(rp.Headers) {
		go c.serveStream(f.Stream, sess, rp)
	} else {
		go c.serveBuffered(f.Stream, sess, rp)
	}
}

func (c *conn) handleEnd(f Frame) {
	sess := c.session(f.Stream)
	if sess == nil {
		return
	}
	pt, ok := c.openFrame(sess, f.B64)
	if !ok {
		return
	}
	var cp ChunkPayload
	if err := json.Unmarshal(pt, &cp); err != nil {
		return
	}
	c.cancelReq(f.Stream + "|" + cp.ID)
}

func (c *conn) handleBye(f Frame) {
	c.mu.Lock()
	delete(c.sessions, f.Stream)
	c.mu.Unlock()
	c.cancelStream(f.Stream)
}

// serveBuffered runs a non-streaming request against the handler and replies
// with a single sealed res. A panicking handler never kills the tunnel.
func (c *conn) serveBuffered(stream string, sess *secure.Session, rp ReqPayload) {
	defer c.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			_ = c.sealSend(stream, "res", sess, mustJSON(ResPayload{
				ID: rp.ID, Status: http.StatusInternalServerError,
			}))
		}
	}()
	req := buildRequest(c.ctx, rp)
	rec := httptest.NewRecorder()
	c.handler.ServeHTTP(rec, req)
	res := rec.Result()
	body, _ := io.ReadAll(res.Body)
	_ = c.sealSend(stream, "res", sess, mustJSON(ResPayload{
		ID:      rp.ID,
		Status:  res.StatusCode,
		Headers: flatten(res.Header),
		Body:    body,
	}))
}

// serveStream runs a streaming (SSE) request. WriteHeader emits res{Stream:true}
// and each Flush emits a chunk; an "end" frame (or a socket drop) cancels the
// request context, and a final chunk{End:true} closes the stream.
func (c *conn) serveStream(stream string, sess *secure.Session, rp ReqPayload) {
	reqCtx, cancel := context.WithCancel(c.ctx)
	key := stream + "|" + rp.ID
	c.regCancel(key, cancel)
	defer func() {
		_ = recover() // a panic in a streaming handler must not kill the tunnel
		c.unregCancel(key)
		cancel()
		_ = c.sealSend(stream, "chunk", sess, mustJSON(ChunkPayload{ID: rp.ID, End: true}))
		c.wg.Done()
	}()
	w := &streamWriter{c: c, stream: stream, sess: sess, id: rp.ID}
	c.handler.ServeHTTP(w, buildRequest(reqCtx, rp))
}

// --- helpers on conn ---

func (c *conn) session(stream string) *secure.Session {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sessions[stream]
}

func (c *conn) openFrame(sess *secure.Session, b64 string) ([]byte, bool) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, false
	}
	pt, err := sess.Open(raw)
	if err != nil {
		return nil, false
	}
	return pt, true
}

// writeFrame serializes an unsealed frame (hs2) onto the socket.
func (c *conn) writeFrame(f Frame) error {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	return wsjson.Write(c.ctx, c.ws, f)
}

// sealSend seals payload for sess and writes it as one frame. Holding wmu
// across Seal+Write keeps per-session nonce order equal to wire order.
func (c *conn) sealSend(stream, kind string, sess *secure.Session, payload []byte) error {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	ct := sess.Seal(payload)
	return wsjson.Write(c.ctx, c.ws, Frame{
		Stream: stream, Kind: kind, B64: base64.StdEncoding.EncodeToString(ct),
	})
}

func (c *conn) regCancel(key string, cancel context.CancelFunc) {
	c.cmu.Lock()
	c.cancels[key] = cancel
	c.cmu.Unlock()
}

func (c *conn) unregCancel(key string) {
	c.cmu.Lock()
	delete(c.cancels, key)
	c.cmu.Unlock()
}

func (c *conn) cancelReq(key string) {
	c.cmu.Lock()
	cancel := c.cancels[key]
	delete(c.cancels, key)
	c.cmu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (c *conn) cancelStream(stream string) {
	prefix := stream + "|"
	c.cmu.Lock()
	for k, cancel := range c.cancels {
		if strings.HasPrefix(k, prefix) {
			cancel()
			delete(c.cancels, k)
		}
	}
	c.cmu.Unlock()
}

// streamWriter is the http.ResponseWriter+Flusher handed to SSE handlers. All
// methods are called only from the single handler goroutine.
type streamWriter struct {
	c      *conn
	stream string
	sess   *secure.Session
	id     string
	hdr    http.Header
	wrote  bool
	buf    []byte
}

func (w *streamWriter) Header() http.Header {
	if w.hdr == nil {
		w.hdr = http.Header{}
	}
	return w.hdr
}

func (w *streamWriter) WriteHeader(status int) {
	if w.wrote {
		return
	}
	w.wrote = true
	_ = w.c.sealSend(w.stream, "res", w.sess, mustJSON(ResPayload{
		ID: w.id, Status: status, Headers: flatten(w.hdr), Stream: true,
	}))
}

func (w *streamWriter) Write(p []byte) (int, error) {
	if !w.wrote {
		w.WriteHeader(http.StatusOK)
	}
	w.buf = append(w.buf, p...)
	return len(p), nil
}

func (w *streamWriter) Flush() {
	if !w.wrote {
		w.WriteHeader(http.StatusOK)
	}
	if len(w.buf) == 0 {
		return
	}
	data := w.buf
	w.buf = nil
	_ = w.c.sealSend(w.stream, "chunk", w.sess, mustJSON(ChunkPayload{ID: w.id, Data: data}))
}

// --- free helpers ---

func buildRequest(ctx context.Context, rp ReqPayload) *http.Request {
	method := rp.Method
	if method == "" {
		method = http.MethodGet
	}
	var body io.Reader
	if len(rp.Body) > 0 {
		body = bytes.NewReader(rp.Body)
	}
	req, err := http.NewRequestWithContext(ctx, method, rp.Path, body)
	if err != nil {
		req, _ = http.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
	}
	for k, v := range rp.Headers {
		req.Header.Set(k, v)
	}
	return req
}

func wantsStream(h map[string]string) bool {
	for k, v := range h {
		if strings.EqualFold(k, "Accept") && strings.Contains(v, "text/event-stream") {
			return true
		}
	}
	return false
}

func flatten(h http.Header) map[string]string {
	if len(h) == 0 {
		return nil
	}
	m := make(map[string]string, len(h))
	for k, v := range h {
		if len(v) > 0 {
			m[k] = v[0]
		}
	}
	return m
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func jitter(d time.Duration) time.Duration {
	if d <= 0 {
		return 0
	}
	half := d / 2
	return half + time.Duration(rand.Int63n(int64(half)+1))
}
