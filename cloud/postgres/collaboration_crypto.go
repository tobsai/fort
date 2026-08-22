package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tobsai/fort/cloud/securebody"
)

var errCollaborationKeyRingRequired = errors.New("Postgres collaboration body key ring is required")

type collaborationEncryptedBody struct {
	Version        int
	Ciphertext     []byte
	KeyID          string
	Nonce          []byte
	Digest         string
	PlaintextBytes int
}

type collaborationBodyCipher interface {
	seal(securebody.Scope, string) (collaborationEncryptedBody, error)
	sealWithKey(securebody.Scope, string, string) (collaborationEncryptedBody, error)
	open(securebody.Scope, collaborationEncryptedBody) (string, error)
	activeKeyID() string
}

type secureCollaborationBodyCipher struct {
	ring securebody.KeyRing
}

// OpenWithKeyRing creates an account-bound Store that can read and write the
// application-level AEAD payloads required by the collaboration ledger.
func OpenWithKeyRing(ctx context.Context, databaseURL, accountID string, ring securebody.KeyRing) (*Store, error) {
	if err := validateSupavisorRuntimeDatabaseURL(databaseURL); err != nil {
		return nil, err
	}
	config, err := SupavisorTransactionConfig(databaseURL)
	if err != nil {
		return nil, err
	}
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("open Postgres pool: %w", err)
	}
	store, err := NewWithKeyRing(pool, accountID, ring)
	if err != nil {
		pool.Close()
		return nil, err
	}
	return store, nil
}

// NewWithKeyRing binds an existing pool and the server-only AEAD key ring to
// one account. The supplied pool is closed by Store.Close.
func NewWithKeyRing(pool *pgxpool.Pool, accountID string, ring securebody.KeyRing) (*Store, error) {
	if pool == nil {
		return nil, fmt.Errorf("Postgres pool is required")
	}
	return newStoreWithKeyRing(pgxDatabase{pool: pool}, accountID, ring)
}

func newStoreWithKeyRing(database database, accountID string, ring securebody.KeyRing) (*Store, error) {
	store, err := newStore(database, accountID)
	if err != nil {
		return nil, err
	}
	store.bodyCipher = secureCollaborationBodyCipher{ring: ring}
	return store, nil
}

func (cipher secureCollaborationBodyCipher) seal(scope securebody.Scope, plaintext string) (collaborationEncryptedBody, error) {
	envelope, err := cipher.ring.Encrypt(scope, []byte(plaintext))
	if err != nil {
		return collaborationEncryptedBody{}, err
	}
	ciphertext, err := base64.RawURLEncoding.DecodeString(envelope.Ciphertext)
	if err != nil {
		return collaborationEncryptedBody{}, fmt.Errorf("decode encrypted body ciphertext: %w", err)
	}
	nonce, err := base64.RawURLEncoding.DecodeString(envelope.Nonce)
	if err != nil {
		return collaborationEncryptedBody{}, fmt.Errorf("decode encrypted body nonce: %w", err)
	}
	digest := sha256.Sum256([]byte(plaintext))
	return collaborationEncryptedBody{
		Version: envelope.Version, Ciphertext: ciphertext, KeyID: envelope.KeyID, Nonce: nonce,
		Digest: hex.EncodeToString(digest[:]), PlaintextBytes: len([]byte(plaintext)),
	}, nil
}

func (cipher secureCollaborationBodyCipher) sealWithKey(scope securebody.Scope, plaintext, keyID string) (collaborationEncryptedBody, error) {
	ring := cipher.ring
	ring.ActiveKeyID = keyID
	return secureCollaborationBodyCipher{ring: ring}.seal(scope, plaintext)
}

func (cipher secureCollaborationBodyCipher) open(scope securebody.Scope, body collaborationEncryptedBody) (string, error) {
	version := body.Version
	if version == 0 {
		// Versionless rows predate the explicit ledger column. Their AEAD AAD
		// was version one, so retaining this interpretation preserves reads
		// while every new persisted row records the version explicitly.
		version = securebody.EnvelopeVersion
	}
	plaintext, err := cipher.ring.Decrypt(scope, securebody.Envelope{
		Version: version, KeyID: body.KeyID,
		Nonce:      base64.RawURLEncoding.EncodeToString(body.Nonce),
		Ciphertext: base64.RawURLEncoding.EncodeToString(body.Ciphertext),
	})
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(plaintext)
	if hex.EncodeToString(digest[:]) != body.Digest ||
		(body.PlaintextBytes >= 0 && len(plaintext) != body.PlaintextBytes) {
		return "", fmt.Errorf("encrypted collaboration body digest or length mismatch")
	}
	return string(plaintext), nil
}

func (cipher secureCollaborationBodyCipher) activeKeyID() string { return cipher.ring.ActiveKeyID }

func (store *Store) collaborationBodies() (collaborationBodyCipher, error) {
	if store == nil || store.bodyCipher == nil {
		return nil, errCollaborationKeyRingRequired
	}
	return store.bodyCipher, nil
}
