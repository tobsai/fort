// Package securebody encrypts sensitive Fort record bodies before they enter
// the cloud ledger. Routing metadata remains separate and plaintext.
package securebody

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/google/uuid"
)

const (
	EnvelopeVersion            = 1
	MaximumPlaintextChunkBytes = 2 << 20
	// AEADOverheadBytes is the fixed authentication-tag overhead of the
	// AES-256-GCM envelope used by Fort. It lets the server derive artifact
	// manifest lengths without disclosing a data-encryption key to a worker.
	AEADOverheadBytes = 16
)

var (
	ErrInvalid        = errors.New("secure body envelope invalid")
	ErrKeyUnavailable = errors.New("secure body key unavailable")
	ErrAuthentication = errors.New("secure body authentication failed")
	ErrPayloadLimit   = errors.New("secure body payload limit exceeded")
	ErrVersion        = errors.New("secure body envelope version unsupported")
)

// Scope is authenticated but not encrypted. It prevents ciphertext from one
// account or durable record from being replayed as another.
type Scope struct {
	AccountID  string `json:"account_id"`
	RecordType string `json:"record_type"`
	RecordID   string `json:"record_id"`
}

// Envelope is the versioned value stored in the ledger. The referenced key is
// held outside Supabase.
type Envelope struct {
	Version    int    `json:"version"`
	KeyID      string `json:"key_id"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

// KeyRing retains every decryption key still referenced by durable ciphertext
// and uses ActiveKeyID only for new writes.
type KeyRing struct {
	ActiveKeyID string
	Keys        map[string][]byte
	Random      io.Reader
}

func (ring KeyRing) Encrypt(scope Scope, plaintext []byte) (Envelope, error) {
	if err := validateScope(scope); err != nil {
		return Envelope{}, err
	}
	if len(plaintext) > MaximumPlaintextChunkBytes {
		return Envelope{}, ErrPayloadLimit
	}
	key, ok := ring.Keys[ring.ActiveKeyID]
	if !ok || len(key) != 32 || strings.TrimSpace(ring.ActiveKeyID) == "" {
		return Envelope{}, ErrKeyUnavailable
	}
	aead, err := newAEAD(key)
	if err != nil {
		return Envelope{}, err
	}
	nonce := make([]byte, aead.NonceSize())
	random := ring.Random
	if random == nil {
		random = rand.Reader
	}
	if _, err := io.ReadFull(random, nonce); err != nil {
		return Envelope{}, fmt.Errorf("generate secure body nonce: %w", err)
	}
	aad := associatedData(scope, ring.ActiveKeyID)
	ciphertext := aead.Seal(nil, nonce, plaintext, aad)
	return Envelope{
		Version:    EnvelopeVersion,
		KeyID:      ring.ActiveKeyID,
		Nonce:      base64.RawURLEncoding.EncodeToString(nonce),
		Ciphertext: base64.RawURLEncoding.EncodeToString(ciphertext),
	}, nil
}

func (ring KeyRing) Decrypt(scope Scope, envelope Envelope) ([]byte, error) {
	if err := validateScope(scope); err != nil {
		return nil, err
	}
	if envelope.Version != EnvelopeVersion {
		return nil, ErrVersion
	}
	key, ok := ring.Keys[envelope.KeyID]
	if !ok || len(key) != 32 || strings.TrimSpace(envelope.KeyID) == "" {
		return nil, ErrKeyUnavailable
	}
	aead, err := newAEAD(key)
	if err != nil {
		return nil, err
	}
	maximumCiphertextBytes := MaximumPlaintextChunkBytes + aead.Overhead()
	if len(envelope.Ciphertext) > base64.RawURLEncoding.EncodedLen(maximumCiphertextBytes) {
		return nil, ErrPayloadLimit
	}
	nonce, err := decodeCanonicalBase64(envelope.Nonce)
	if err != nil || len(nonce) != aead.NonceSize() {
		return nil, ErrInvalid
	}
	ciphertext, err := decodeCanonicalBase64(envelope.Ciphertext)
	if err != nil || len(ciphertext) < aead.Overhead() {
		return nil, ErrInvalid
	}
	plaintext, err := aead.Open(nil, nonce, ciphertext, associatedData(scope, envelope.KeyID))
	if err != nil {
		return nil, ErrAuthentication
	}
	if len(plaintext) > MaximumPlaintextChunkBytes {
		return nil, ErrPayloadLimit
	}
	return plaintext, nil
}

func newAEAD(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create secure body cipher: %w", err)
	}
	return cipher.NewGCM(block)
}

func validateScope(scope Scope) error {
	if _, err := uuid.Parse(scope.AccountID); err != nil {
		return ErrInvalid
	}
	if scope.RecordType == "" || scope.RecordType != strings.TrimSpace(scope.RecordType) ||
		scope.RecordID == "" || scope.RecordID != strings.TrimSpace(scope.RecordID) {
		return ErrInvalid
	}
	return nil
}

func associatedData(scope Scope, keyID string) []byte {
	payload, _ := json.Marshal(struct {
		Version int    `json:"version"`
		KeyID   string `json:"key_id"`
		Scope   Scope  `json:"scope"`
	}{Version: EnvelopeVersion, KeyID: keyID, Scope: scope})
	return payload
}

func decodeCanonicalBase64(value string) ([]byte, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || base64.RawURLEncoding.EncodeToString(decoded) != value {
		return nil, ErrInvalid
	}
	return decoded, nil
}
