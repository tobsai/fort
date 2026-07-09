// Package secure is Fort's E2E crypto contract for the relay (spec 028): a
// Noise IK handshake (X25519) between a client and the daemon's pinned static
// key, then ChaCha20-Poly1305 AEAD framing. The gateway broker relays these
// frames opaquely — it can neither read nor forge them. Both ends of Fort's
// tests use this package, proving the contract round-trips.
package secure

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"errors"

	"github.com/flynn/noise"
)

// suite: X25519 DH, ChaCha20-Poly1305 AEAD, BLAKE2s hash — the WireGuard family.
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
	// Noise convention (Split): cs1 encrypts initiator->responder, cs2 the reverse.
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
