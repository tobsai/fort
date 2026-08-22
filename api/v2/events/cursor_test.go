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
)

func TestCursorEndpointAuthenticatesSignedAccountAndReusesSharedPool(t *testing.T) {
	t.Parallel()

	key := []byte("0123456789abcdef0123456789abcdef")
	values := map[string]string{
		"DATABASE_URL":                     "postgresql://runtime.test/fort?sslmode=require",
		"FORT_CONTROL_ASSERTION_KEYS_JSON": `{"service-2026-08":"` + base64.RawURLEncoding.EncodeToString(key) + `"}`,
	}
	store := &fakeEventStore{claimed: make(map[string]struct{})}
	opens := 0
	handler := newCursorEndpoint(func(key string) string { return values[key] }, func(_ context.Context, databaseURL string) (eventStore, error) {
		opens++
		if databaseURL != values["DATABASE_URL"] {
			t.Fatalf("database URL = %q", databaseURL)
		}
		return store, nil
	})

	body := `{"after_cursor":"cursor-0"}`
	request := httptest.NewRequest(http.MethodPost, "/api/v2/events/cursor", strings.NewReader(body))
	request.Header.Set(controlapi.ServiceAssertionHeader, cursorAssertion(t, key, body, "nonce-for-cursor-endpoint-000001"))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	const accountID = "4af424a4-d81a-47d5-a495-400868883b86"
	if store.claimAccount != accountID || store.readAccount != accountID {
		t.Fatalf("signed account claim/read = %q/%q", store.claimAccount, store.readAccount)
	}
	if store.afterCursor != "cursor-0" {
		t.Fatalf("after cursor = %q", store.afterCursor)
	}

	secondBody := `{"after_cursor":"cursor-9"}`
	second := httptest.NewRequest(http.MethodPost, "/api/v2/events/cursor", strings.NewReader(secondBody))
	second.Header.Set(controlapi.ServiceAssertionHeader, cursorAssertion(t, key, secondBody, "nonce-for-cursor-endpoint-000002"))
	secondRecorder := httptest.NewRecorder()
	handler.ServeHTTP(secondRecorder, second)
	if secondRecorder.Code != http.StatusOK || opens != 1 {
		t.Fatalf("second status/opens = %d/%d, want 200/1", secondRecorder.Code, opens)
	}
}

func TestCursorEndpointFailsClosedBeforeDatabaseUseWhenMethodOrConfigurationIsInvalid(t *testing.T) {
	t.Parallel()

	opens := 0
	handler := newCursorEndpoint(func(string) string { return "" }, func(context.Context, string) (eventStore, error) {
		opens++
		return &fakeEventStore{}, nil
	})
	get := httptest.NewRecorder()
	handler.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/api/v2/events/cursor", nil))
	if get.Code != http.StatusMethodNotAllowed || opens != 0 {
		t.Fatalf("GET status/opens = %d/%d, want 405/0", get.Code, opens)
	}

	post := httptest.NewRecorder()
	handler.ServeHTTP(post, httptest.NewRequest(http.MethodPost, "/api/v2/events/cursor", strings.NewReader(`{"after_cursor":"cursor-0"}`)))
	if post.Code != http.StatusServiceUnavailable || opens != 0 {
		t.Fatalf("unconfigured POST status/opens = %d/%d, want 503/0", post.Code, opens)
	}
}

type fakeEventStore struct {
	claimed      map[string]struct{}
	claimAccount string
	readAccount  string
	afterCursor  string
}

func (store *fakeEventStore) Claim(_ context.Context, accountID, keyID, nonce string, _ time.Time) (bool, error) {
	store.claimAccount = accountID
	claim := accountID + ":" + keyID + ":" + nonce
	if _, ok := store.claimed[claim]; ok {
		return false, nil
	}
	store.claimed[claim] = struct{}{}
	return true, nil
}

func (store *fakeEventStore) ReadCursorPage(_ context.Context, accountID, afterCursor string) (controlapi.CursorPage, error) {
	store.readAccount = accountID
	store.afterCursor = afterCursor
	return controlapi.CursorPage{Events: []controlapi.CursorEvent{}, NextCursor: afterCursor}, nil
}

func (store *fakeEventStore) Close() error { return nil }

func cursorAssertion(t *testing.T, key []byte, body, nonce string) string {
	t.Helper()
	now := time.Now().UTC()
	digestBytes := sha256.Sum256([]byte(body))
	token, err := controlapi.IssueServiceAssertion(key, controlapi.ServiceAssertion{
		KeyID: "service-2026-08", AccountID: "4af424a4-d81a-47d5-a495-400868883b86",
		RouteClass: "owner.events.read", Audience: "fort-control",
		RequestDigest: hex.EncodeToString(digestBytes[:]), IssuedAt: now.Add(-time.Second),
		ExpiresAt: now.Add(30 * time.Second), Nonce: nonce,
	})
	if err != nil {
		t.Fatalf("issue assertion: %v", err)
	}
	return token
}
