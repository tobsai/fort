package handler

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tobsai/fort/cloud/controlapi"
	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/ledger"
)

func TestAgentsEndpointUsesSignedAccountAndOneSharedPool(t *testing.T) {
	t.Parallel()

	key := []byte("0123456789abcdef0123456789abcdef")
	values := map[string]string{
		"DATABASE_URL":                     "postgresql://runtime.test/fort?sslmode=require",
		"FORT_CONTROL_ASSERTION_KEYS_JSON": `{"service-2026-08":"` + base64.RawURLEncoding.EncodeToString(key) + `"}`,
	}
	store := &fakeAgentControlStore{claimed: make(map[string]struct{})}
	opens := 0
	handler := newAgentsEndpoint(func(key string) string { return values[key] }, func(context.Context, string) (agentControlStore, error) {
		opens++
		return store, nil
	})

	request := httptest.NewRequest(http.MethodGet, "/api/v2/agents?state=open", nil)
	request.Header.Set(controlapi.ServiceAssertionHeader, agentsAssertion(t, key, "nonce-for-agent-list-endpoint-001"))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || strings.TrimSpace(recorder.Body.String()) != "[]" {
		t.Fatalf("first response = %d %q", recorder.Code, recorder.Body.String())
	}
	if store.accountID != "4af424a4-d81a-47d5-a495-400868883b86" || opens != 1 {
		t.Fatalf("signed account/opens = %q/%d", store.accountID, opens)
	}
}

type fakeAgentControlStore struct {
	claimed   map[string]struct{}
	accountID string
}

func (store *fakeAgentControlStore) Claim(_ context.Context, accountID, keyID, nonce string, _ time.Time) (bool, error) {
	claim := accountID + ":" + keyID + ":" + nonce
	if _, found := store.claimed[claim]; found {
		return false, nil
	}
	store.claimed[claim] = struct{}{}
	return true, nil
}

func (store *fakeAgentControlStore) ListAgents(_ context.Context, accountID string, _ conversation.AgentState) ([]ledger.AgentRecord, error) {
	store.accountID = accountID
	return []ledger.AgentRecord{}, nil
}

func (store *fakeAgentControlStore) Close() error { return nil }

func agentsAssertion(t *testing.T, key []byte, nonce string) string {
	t.Helper()
	now := time.Now().UTC()
	digest := sha256.Sum256(nil)
	token, err := controlapi.IssueServiceAssertion(key, controlapi.ServiceAssertion{
		KeyID: "service-2026-08", AccountID: "4af424a4-d81a-47d5-a495-400868883b86",
		RouteClass: agentsListRouteClass, Audience: "fort-control", RequestDigest: hex.EncodeToString(digest[:]),
		IssuedAt: now.Add(-time.Second), ExpiresAt: now.Add(30 * time.Second), Nonce: nonce,
	})
	if err != nil {
		t.Fatalf("issue assertion: %v", err)
	}
	return token
}
