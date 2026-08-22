package postgres

import (
	"errors"
	"testing"

	"github.com/tobsai/fort/cloud/securebody"
)

func TestCollaborationCipherCarriesAndAuthenticatesEnvelopeVersion(t *testing.T) {
	t.Parallel()

	cipher := secureCollaborationBodyCipher{ring: collaborationTestKeyRing()}
	scope := securebody.Scope{
		AccountID: testAccountID, RecordType: "conversation_message", RecordID: "message:versioned",
	}
	body, err := cipher.seal(scope, "versioned private body")
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	if body.Version != securebody.EnvelopeVersion {
		t.Fatalf("persisted envelope version = %d, want %d", body.Version, securebody.EnvelopeVersion)
	}

	unsupported := body
	unsupported.Version++
	if _, err := cipher.open(scope, unsupported); !errors.Is(err, securebody.ErrVersion) {
		t.Fatalf("unsupported envelope version error = %v, want %v", err, securebody.ErrVersion)
	}
}

func TestCollaborationCipherStillDecryptsVersionlessLegacyRowsAsVersionOne(t *testing.T) {
	t.Parallel()

	cipher := secureCollaborationBodyCipher{ring: collaborationTestKeyRing()}
	scope := securebody.Scope{
		AccountID: testAccountID, RecordType: "conversation_message", RecordID: "message:legacy",
	}
	body, err := cipher.seal(scope, "legacy private body")
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	body.Version = 0
	plaintext, err := cipher.open(scope, body)
	if err != nil {
		t.Fatalf("open versionless legacy row: %v", err)
	}
	if plaintext != "legacy private body" {
		t.Fatalf("legacy plaintext = %q", plaintext)
	}
}
