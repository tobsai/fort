package secure

// This test is the CROSS-STACK CORRECTNESS GATE for spec 028 part 2. It drives a
// full Noise IK handshake with the flynn/noise engine under a DETERMINISTIC
// random source (fixed static + ephemeral private keys), then emits a JSON test
// vector to gateway/shared/testdata/ik-vector.json. The TypeScript mirror
// (@fort/gateway-shared) loads the SAME vector and must reproduce every byte:
// msg1 (initiator handshake), open Go's msg2, and derive transport keys that
// seal/open byte-identically. If the Go and TS stacks ever diverge, one side
// stops matching this file.
//
// It only WRITES a file inside the repo and uses no network, so `go test
// ./exec/relay/secure/` stays green and offline.

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/flynn/noise"
)

// seed derives a fixed 32-byte private key from a label, so the vector is fully
// deterministic and reproducible across runs (the committed JSON never drifts).
func seed(label string) []byte {
	s := sha256.Sum256([]byte("fort/028/ik-vector/" + label))
	return s[:]
}

func b64(b []byte) string { return base64.StdEncoding.EncodeToString(b) }

// ikVector is the on-disk schema shared with the TypeScript test. All byte
// fields are standard base64. Private keys are the RAW 32 bytes fed to X25519
// (unclamped, exactly as flynn stores them); public keys are X25519(priv, base).
type ikVector struct {
	ProtocolName          string `json:"protocolName"`
	InitStaticPriv        string `json:"initStaticPriv"`
	InitStaticPub         string `json:"initStaticPub"`
	RespStaticPriv        string `json:"respStaticPriv"`
	RespStaticPub         string `json:"respStaticPub"`
	InitEphPriv           string `json:"initEphPriv"`
	RespEphPriv           string `json:"respEphPriv"`
	Prologue              string `json:"prologue"`
	Payload1              string `json:"payload1"`
	Payload2              string `json:"payload2"`
	Msg1                  string `json:"msg1"`
	Msg2                  string `json:"msg2"`
	RespStaticFingerprint string `json:"respStaticFingerprint"`

	Transport struct {
		PlaintextI      string `json:"plaintextI"`      // initiator -> responder
		InitiatorSealed string `json:"initiatorSealed"` // Seal(plaintextI) with cs1, nonce 0
		PlaintextR      string `json:"plaintextR"`      // responder -> initiator
		ResponderSealed string `json:"responderSealed"` // Seal(plaintextR) with cs2, nonce 0
	} `json:"transport"`

	// ReqPayloadJSON is the exact bytes Go's encoding/json produced for a
	// relay.ReqPayload{ID,Method,Path}. The TS frame layer must marshal the
	// same object to byte-identical JSON (field order + omitempty).
	ReqPayloadJSON string `json:"reqPayloadJSON"`
}

func TestGenerateIKVector(t *testing.T) {
	initStaticPriv := seed("init-static")
	respStaticPriv := seed("resp-static")
	initEphPriv := seed("init-ephemeral")
	respEphPriv := seed("resp-ephemeral")

	// Derive public keys via the suite (reads 32 bytes as the private scalar,
	// then pub = X25519(priv, base)) — identical to what the TS side does.
	initStatic, err := suite.GenerateKeypair(bytes.NewReader(initStaticPriv))
	if err != nil {
		t.Fatalf("init static: %v", err)
	}
	respStatic, err := suite.GenerateKeypair(bytes.NewReader(respStaticPriv))
	if err != nil {
		t.Fatalf("resp static: %v", err)
	}

	// Fixed random sources pin each side's ephemeral to a known private key, so
	// msg1 and msg2 are byte-reproducible by the TS mirror.
	initHS, err := noise.NewHandshakeState(noise.Config{
		CipherSuite:   suite,
		Pattern:       noise.HandshakeIK,
		Initiator:     true,
		StaticKeypair: noise.DHKey{Private: initStatic.Private, Public: initStatic.Public},
		PeerStatic:    respStatic.Public,
		Random:        bytes.NewReader(initEphPriv),
	})
	if err != nil {
		t.Fatalf("init hs: %v", err)
	}
	respHS, err := noise.NewHandshakeState(noise.Config{
		CipherSuite:   suite,
		Pattern:       noise.HandshakeIK,
		Initiator:     false,
		StaticKeypair: noise.DHKey{Private: respStatic.Private, Public: respStatic.Public},
		Random:        bytes.NewReader(respEphPriv),
	})
	if err != nil {
		t.Fatalf("resp hs: %v", err)
	}

	payload1 := []byte("fort ik handshake payload one")
	payload2 := []byte("fort ik handshake payload two")

	// msg1: initiator writes {e, es, s, ss} + sealed payload1.
	msg1, ics1w, ics2w, err := initHS.WriteMessage(nil, payload1)
	if err != nil {
		t.Fatalf("write msg1: %v", err)
	}
	if ics1w != nil || ics2w != nil {
		t.Fatal("handshake completed too early on msg1")
	}
	// responder reads msg1, recovering payload1.
	rp1, _, _, err := respHS.ReadMessage(nil, msg1)
	if err != nil {
		t.Fatalf("responder read msg1: %v", err)
	}
	if !bytes.Equal(rp1, payload1) {
		t.Fatalf("responder recovered payload1 = %q", rp1)
	}
	// msg2: responder writes {e, ee, se} + sealed payload2; handshake completes.
	msg2, rcs1, rcs2, err := respHS.WriteMessage(nil, payload2)
	if err != nil {
		t.Fatalf("write msg2: %v", err)
	}
	if rcs1 == nil || rcs2 == nil {
		t.Fatal("responder did not split after msg2")
	}
	// initiator reads msg2, recovering payload2; handshake completes.
	ip2, ics1, ics2, err := initHS.ReadMessage(nil, msg2)
	if err != nil {
		t.Fatalf("initiator read msg2: %v", err)
	}
	if !bytes.Equal(ip2, payload2) {
		t.Fatalf("initiator recovered payload2 = %q", ip2)
	}
	if ics1 == nil || ics2 == nil {
		t.Fatal("initiator did not split after msg2")
	}

	// Wrap the split CipherStates into the REAL secure.Session (same package),
	// matching Fort's convention: initiator enc=cs1/dec=cs2, responder mirror.
	initSess := &Session{enc: ics1, dec: ics2}
	respSess := &Session{enc: rcs2, dec: rcs1}

	plaintextI := []byte("transport frame: initiator to responder")
	plaintextR := []byte("transport frame: responder to initiator")
	initiatorSealed := initSess.Seal(plaintextI) // cs1, nonce 0
	responderSealed := respSess.Seal(plaintextR) // cs2, nonce 0

	// Sanity: the two sides interoperate (proves the Go side itself is coherent).
	if got, err := respSess.Open(initiatorSealed); err != nil || !bytes.Equal(got, plaintextI) {
		t.Fatalf("responder open initiator frame: %q err=%v", got, err)
	}
	if got, err := initSess.Open(responderSealed); err != nil || !bytes.Equal(got, plaintextR) {
		t.Fatalf("initiator open responder frame: %q err=%v", got, err)
	}

	// A representative req payload marshaled exactly as the daemon would (the TS
	// frame layer must reproduce these bytes). No headers/body: deterministic,
	// no map-ordering or HTML-escaping to worry about.
	reqJSON, err := json.Marshal(struct {
		ID     string `json:"id"`
		Method string `json:"method"`
		Path   string `json:"path"`
	}{ID: "r1", Method: "GET", Path: "/api/summary"})
	if err != nil {
		t.Fatalf("marshal req: %v", err)
	}

	var v ikVector
	v.ProtocolName = "Noise_IK_25519_ChaChaPoly_BLAKE2s"
	v.InitStaticPriv = b64(initStatic.Private)
	v.InitStaticPub = b64(initStatic.Public)
	v.RespStaticPriv = b64(respStatic.Private)
	v.RespStaticPub = b64(respStatic.Public)
	v.InitEphPriv = b64(initEphPriv)
	v.RespEphPriv = b64(respEphPriv)
	v.Prologue = b64(nil)
	v.Payload1 = b64(payload1)
	v.Payload2 = b64(payload2)
	v.Msg1 = b64(msg1)
	v.Msg2 = b64(msg2)
	v.RespStaticFingerprint = FingerprintOf(respStatic.Public)
	v.Transport.PlaintextI = b64(plaintextI)
	v.Transport.InitiatorSealed = b64(initiatorSealed)
	v.Transport.PlaintextR = b64(plaintextR)
	v.Transport.ResponderSealed = b64(responderSealed)
	v.ReqPayloadJSON = b64(reqJSON)

	out, err := json.MarshalIndent(&v, "", "  ")
	if err != nil {
		t.Fatalf("marshal vector: %v", err)
	}
	out = append(out, '\n')

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "gateway", "shared", "testdata")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir testdata: %v", err)
	}
	path := filepath.Join(dir, "ik-vector.json")
	if err := os.WriteFile(path, out, 0o644); err != nil {
		t.Fatalf("write vector: %v", err)
	}
	t.Logf("wrote cross-stack IK vector: %s (%d bytes)", path, len(out))
}
