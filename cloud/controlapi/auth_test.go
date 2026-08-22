package controlapi_test

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tobsai/fort/cloud/controlapi"
)

func TestServiceAuthorizationUsesOnlySignedAccountAndPreservesBody(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_787_331_600, 0).UTC()
	body := `{"account_id":"forged","message":"hello"}`
	verifier, token := serviceAuthorizationFixture(t, now, body, "owner.commands.create")

	var gotAccount, gotBody string
	next := http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		gotAccount, _ = controlapi.AccountIDFromContext(request.Context())
		payload, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatalf("read preserved body: %v", err)
		}
		gotBody = string(payload)
		response.WriteHeader(http.StatusNoContent)
	})
	handler := controlapi.RequireServiceAssertion(verifier, "owner.commands.create", next)
	request := httptest.NewRequest(http.MethodPost, "/api/v2/commands", strings.NewReader(body))
	request.Header.Set(controlapi.ServiceAssertionHeader, token)
	request.Header.Set("X-Fort-Account-ID", "also-forged")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusNoContent, recorder.Body.String())
	}
	if gotAccount != "4af424a4-d81a-47d5-a495-400868883b86" {
		t.Fatalf("context account = %q, want signed account", gotAccount)
	}
	if gotBody != body {
		t.Fatalf("preserved body = %q, want %q", gotBody, body)
	}
}

func TestServiceAuthorizationRejectsMissingAssertionAndOversizeBody(t *testing.T) {
	t.Parallel()

	verifier := controlapi.ServiceAssertionVerifier{}
	nextCalled := false
	handler := controlapi.RequireServiceAssertion(verifier, "owner.commands.create", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		nextCalled = true
	}))

	missing := httptest.NewRecorder()
	handler.ServeHTTP(missing, httptest.NewRequest(http.MethodPost, "/api/v2/commands", nil))
	if missing.Code != http.StatusUnauthorized {
		t.Fatalf("missing assertion status = %d, want %d", missing.Code, http.StatusUnauthorized)
	}

	oversize := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v2/commands", strings.NewReader(strings.Repeat("x", controlapi.MaximumFunctionBodyBytes+1)))
	request.Header.Set(controlapi.ServiceAssertionHeader, "invalid")
	handler.ServeHTTP(oversize, request)
	if oversize.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversize status = %d, want %d", oversize.Code, http.StatusRequestEntityTooLarge)
	}
	if nextCalled {
		t.Fatal("unauthorized request reached control handler")
	}
}

func serviceAuthorizationFixture(t *testing.T, now time.Time, body, routeClass string) (controlapi.ServiceAssertionVerifier, string) {
	t.Helper()
	digestBytes := sha256.Sum256([]byte(body))
	digest := hex.EncodeToString(digestBytes[:])
	key := []byte("0123456789abcdef0123456789abcdef")
	token, err := controlapi.IssueServiceAssertion(key, controlapi.ServiceAssertion{
		KeyID:         "service-2026-08",
		AccountID:     "4af424a4-d81a-47d5-a495-400868883b86",
		RouteClass:    routeClass,
		Audience:      "fort-control",
		RequestDigest: digest,
		IssuedAt:      now.Add(-time.Second),
		ExpiresAt:     now.Add(30 * time.Second),
		Nonce:         "908b3b526cf8472e91b1e6f71fb8df99",
	})
	if err != nil {
		t.Fatalf("issue assertion: %v", err)
	}
	return controlapi.ServiceAssertionVerifier{
		Audience: "fort-control",
		Keys:     map[string][]byte{"service-2026-08": key},
		Clock:    func() time.Time { return now },
		Nonces:   &memoryNonceClaimer{seen: make(map[string]struct{})},
		MaxTTL:   time.Minute,
	}, token
}
