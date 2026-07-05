package meshjoin

import (
	"crypto/rand"
	"encoding/hex"
	"sync"

	"github.com/tobsai/fort/core/config"
)

// TokenStore holds the mesh token for live readers (node auth, outbound
// remotes) and persists it to node.yaml on first mint (spec 024 bootstrap).
type TokenStore struct {
	mu      sync.Mutex
	val     string
	dataDir string
	name    string // this machine's NodeName, persisted alongside
	addr    string // this machine's listen addr, persisted alongside
}

func NewTokenStore(initial, dataDir, name, addr string) *TokenStore {
	return &TokenStore{val: initial, dataDir: dataDir, name: name, addr: addr}
}

// Get returns the current token ("" until minted or configured).
func (t *TokenStore) Get() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.val
}

// Ensure returns the token, minting and persisting it on first use.
// minted reports whether this call created it (the caller prints the
// hub-now-accepts-exec notice exactly once).
func (t *TokenStore) Ensure() (tok string, minted bool, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.val != "" {
		return t.val, false, nil
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", false, err
	}
	tok = hex.EncodeToString(buf)
	if err := config.SaveNodeFile(t.dataDir, config.NodeFile{Name: t.name, Token: tok, Addr: t.addr}); err != nil {
		return "", false, err
	}
	t.val = tok
	return tok, true, nil
}
