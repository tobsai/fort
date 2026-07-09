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
