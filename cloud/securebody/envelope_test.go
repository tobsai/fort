package securebody_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/tobsai/fort/cloud/securebody"
)

func TestEnvelopeEncryptsAndBindsAccountRecordAndKeyVersion(t *testing.T) {
	t.Parallel()

	ring := securebody.KeyRing{
		ActiveKeyID: "dek-2026-08",
		Keys: map[string][]byte{
			"dek-2026-08": bytes.Repeat([]byte{0x42}, 32),
		},
		Random: bytes.NewReader(bytes.Repeat([]byte{0x17}, 64)),
	}
	scope := securebody.Scope{
		AccountID:  "4af424a4-d81a-47d5-a495-400868883b86",
		RecordType: "conversation_message",
		RecordID:   "message:01",
	}
	plaintext := []byte("private agent message")
	envelope, err := ring.Encrypt(scope, plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if envelope.Version != 1 || envelope.KeyID != "dek-2026-08" || envelope.Ciphertext == string(plaintext) {
		t.Fatalf("unexpected envelope: %+v", envelope)
	}
	decrypted, err := ring.Decrypt(scope, envelope)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if !bytes.Equal(decrypted, plaintext) {
		t.Fatalf("decrypted = %q, want %q", decrypted, plaintext)
	}

	foreign := scope
	foreign.AccountID = "f23a9db2-a58a-459b-ac7f-13ac9ffec960"
	if _, err := ring.Decrypt(foreign, envelope); !errors.Is(err, securebody.ErrAuthentication) {
		t.Fatalf("cross-account decrypt error = %v, want authentication failure", err)
	}
}

func TestEnvelopeKeyRotationRetainsOldDecryptionOnlyWhileKeyExists(t *testing.T) {
	t.Parallel()

	scope := securebody.Scope{AccountID: "4af424a4-d81a-47d5-a495-400868883b86", RecordType: "handoff_result", RecordID: "handoff:01"}
	oldKey := bytes.Repeat([]byte{0x11}, 32)
	newKey := bytes.Repeat([]byte{0x22}, 32)
	oldRing := securebody.KeyRing{
		ActiveKeyID: "dek-old",
		Keys:        map[string][]byte{"dek-old": oldKey},
		Random:      bytes.NewReader(bytes.Repeat([]byte{0x01}, 64)),
	}
	oldEnvelope, err := oldRing.Encrypt(scope, []byte("old result"))
	if err != nil {
		t.Fatalf("encrypt old envelope: %v", err)
	}

	rotated := securebody.KeyRing{
		ActiveKeyID: "dek-new",
		Keys:        map[string][]byte{"dek-old": oldKey, "dek-new": newKey},
		Random:      bytes.NewReader(bytes.Repeat([]byte{0x02}, 64)),
	}
	if _, err := rotated.Decrypt(scope, oldEnvelope); err != nil {
		t.Fatalf("rotated ring cannot decrypt retained old key: %v", err)
	}
	newEnvelope, err := rotated.Encrypt(scope, []byte("new result"))
	if err != nil {
		t.Fatalf("encrypt new envelope: %v", err)
	}
	if newEnvelope.KeyID != "dek-new" {
		t.Fatalf("new envelope key = %q, want dek-new", newEnvelope.KeyID)
	}

	retired := securebody.KeyRing{ActiveKeyID: "dek-new", Keys: map[string][]byte{"dek-new": newKey}}
	if _, err := retired.Decrypt(scope, oldEnvelope); !errors.Is(err, securebody.ErrKeyUnavailable) {
		t.Fatalf("retired-key decrypt error = %v, want key unavailable", err)
	}
}

func TestEnvelopeEnforcesPlaintextChunkLimit(t *testing.T) {
	t.Parallel()

	ring := securebody.KeyRing{
		ActiveKeyID: "dek",
		Keys:        map[string][]byte{"dek": bytes.Repeat([]byte{0x42}, 32)},
		Random:      bytes.NewReader(bytes.Repeat([]byte{0x17}, 64)),
	}
	scope := securebody.Scope{AccountID: "4af424a4-d81a-47d5-a495-400868883b86", RecordType: "artifact_chunk", RecordID: "chunk:01"}
	if _, err := ring.Encrypt(scope, make([]byte, securebody.MaximumPlaintextChunkBytes+1)); !errors.Is(err, securebody.ErrPayloadLimit) {
		t.Fatalf("oversize Encrypt error = %v, want payload limit", err)
	}
}
