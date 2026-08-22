package handler

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tobsai/fort/cloud/controlapi"
	"github.com/tobsai/fort/cloud/securebody"
)

func TestOwnerEndpointDispatchesHandoffCollectionDetailAndCancellation(t *testing.T) {
	t.Parallel()
	key := []byte("0123456789abcdef0123456789abcdef")
	values := ownerHandoffEnvironment(key)
	store := &fakeOwnerStore{claimed: make(map[string]struct{})}
	handler := newOwnerEndpoint(func(key string) string { return values[key] }, func(context.Context, string, securebody.KeyRing) (ownerControlStore, error) {
		return store, nil
	})

	list := httptest.NewRequest(http.MethodGet, "/api/v2/owner?resource=handoffs", nil)
	list.Header.Set(controlapi.ServiceAssertionHeader, ownerAssertion(t, key, "owner.handoffs.list", "owner-handoff-list-nonce-00000000001"))
	listed := httptest.NewRecorder()
	handler.ServeHTTP(listed, list)
	if listed.Code != http.StatusOK || listed.Body.String() != "[]\n" {
		t.Fatalf("Handoff list = %d %q", listed.Code, listed.Body.String())
	}

	deadline := time.Now().UTC().Add(10 * time.Minute).Format(time.RFC3339Nano)
	createBody := `{"idempotency_key":"handoff:create:owner","source_conversation_id":"conversation:launch","source_message_id":"41","recipient_agent_id":"agent:builder","context_message_ids":["41"],"requested_result":"Review.","hard_deadline":"` + deadline + `"}`
	create := httptest.NewRequest(http.MethodPost, "/api/v2/owner?resource=handoffs", strings.NewReader(createBody))
	create.Header.Set(controlapi.ServiceAssertionHeader, ownerAssertionBody(t, key, "owner.handoffs.create", createBody, "owner-handoff-create-nonce-00000001"))
	created := httptest.NewRecorder()
	handler.ServeHTTP(created, create)
	if created.Code != http.StatusAccepted || store.createdHandoff.AccountID != ownerAccountID ||
		store.createdHandoff.RecipientAgentID != "agent:builder" || store.createdHandoff.HandoffID == "" {
		t.Fatalf("Handoff create = %d %q / %+v", created.Code, created.Body.String(), store.createdHandoff)
	}

	detail := httptest.NewRequest(http.MethodGet, "/api/v2/owner?resource=handoff_detail&handoff_id="+store.createdHandoff.HandoffID, nil)
	detail.Header.Set(controlapi.ServiceAssertionHeader, ownerAssertion(t, key, "owner.handoffs.read", "owner-handoff-read-nonce-00000000001"))
	read := httptest.NewRecorder()
	handler.ServeHTTP(read, detail)
	if read.Code != http.StatusOK || !strings.Contains(read.Body.String(), `"id":"`+store.createdHandoff.HandoffID+`"`) {
		t.Fatalf("Handoff detail = %d %q", read.Code, read.Body.String())
	}

	cancelBody := `{"idempotency_key":"handoff:cancel:owner"}`
	cancel := httptest.NewRequest(http.MethodPost, "/api/v2/owner?resource=handoff_cancel&handoff_id="+store.createdHandoff.HandoffID, strings.NewReader(cancelBody))
	cancel.Header.Set(controlapi.ServiceAssertionHeader, ownerAssertionBody(t, key, "owner.handoffs.cancel", cancelBody, "owner-handoff-cancel-nonce-0000001"))
	canceled := httptest.NewRecorder()
	handler.ServeHTTP(canceled, cancel)
	if canceled.Code != http.StatusAccepted || store.canceledHandoff.HandoffID != store.createdHandoff.HandoffID ||
		store.canceledHandoff.AccountID != ownerAccountID {
		t.Fatalf("Handoff cancel = %d %q / %+v", canceled.Code, canceled.Body.String(), store.canceledHandoff)
	}
}

func TestOwnerEndpointFencesHandoffMutationBeforeDatabaseButKeepsReadRoute(t *testing.T) {
	t.Parallel()
	key := []byte("0123456789abcdef0123456789abcdef")
	values := ownerHandoffEnvironment(key)
	values["FORT_AUTHORITY_MODE"] = "legacy_v1_write"
	opens := 0
	handler := newOwnerEndpoint(func(key string) string { return values[key] }, func(context.Context, string, securebody.KeyRing) (ownerControlStore, error) {
		opens++
		return &fakeOwnerStore{claimed: make(map[string]struct{})}, nil
	})
	request := httptest.NewRequest(http.MethodPost, "/api/v2/owner?resource=handoffs", strings.NewReader(`{}`))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusConflict || !strings.Contains(recorder.Body.String(), `"code":"write_authority_inactive"`) || opens != 0 {
		t.Fatalf("status/body/opens = %d/%q/%d, want 409 authority fence before database", recorder.Code, recorder.Body.String(), opens)
	}

	read := httptest.NewRequest(http.MethodGet, "/api/v2/owner?resource=handoffs", nil)
	read.Header.Set(controlapi.ServiceAssertionHeader, ownerAssertion(t, key, "owner.handoffs.list", "owner-handoff-rollback-read-0000001"))
	readRecorder := httptest.NewRecorder()
	handler.ServeHTTP(readRecorder, read)
	if readRecorder.Code != http.StatusOK || opens != 1 {
		t.Fatalf("rollback read status/opens = %d/%d, want 200/1", readRecorder.Code, opens)
	}
}

func TestOwnerEndpointFailsClosedBeforeDatabaseWithoutBodyKeyRing(t *testing.T) {
	t.Parallel()
	key := []byte("0123456789abcdef0123456789abcdef")
	values := ownerHandoffEnvironment(key)
	delete(values, "FORT_BODY_KEYS_JSON")
	opens := 0
	handler := newOwnerEndpoint(func(key string) string { return values[key] }, func(context.Context, string, securebody.KeyRing) (ownerControlStore, error) {
		opens++
		return &fakeOwnerStore{claimed: make(map[string]struct{})}, nil
	})
	request := httptest.NewRequest(http.MethodGet, "/api/v2/owner?resource=handoffs", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusServiceUnavailable || opens != 0 {
		t.Fatalf("status/opens = %d/%d, want fail-closed 503/0", recorder.Code, opens)
	}
}

func TestOwnerHandoffSemanticRoutesRejectAmbiguousParameters(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ method, target string }{
		{http.MethodGet, "/api/v2/owner?resource=handoffs&state=open"},
		{http.MethodPost, "/api/v2/owner?resource=handoffs&handoff_id=forged"},
		{http.MethodGet, "/api/v2/owner?resource=handoff_detail"},
		{http.MethodGet, "/api/v2/owner?resource=handoff_detail&handoff_id=one&account_id=forged"},
		{http.MethodPost, "/api/v2/owner?resource=handoff_cancel&handoff_id=one&target_id=forged"},
	} {
		if _, _, _, ok := ownerRoute(httptest.NewRequest(test.method, test.target, nil)); ok {
			t.Fatalf("ambiguous route accepted: %s %s", test.method, test.target)
		}
	}
}

func ownerHandoffEnvironment(key []byte) map[string]string {
	return map[string]string{
		"DATABASE_URL":                     "postgresql://runtime.test/fort?sslmode=require",
		"FORT_CONTROL_ASSERTION_KEYS_JSON": `{"service-2026-08":"` + base64.RawURLEncoding.EncodeToString(key) + `"}`,
		"FORT_BODY_ACTIVE_KID":             "body-2026-08",
		"FORT_BODY_KEYS_JSON":              `{"body-2026-08":"` + base64.RawURLEncoding.EncodeToString(key) + `"}`,
		"FORT_AUTHORITY_MODE":              controlapi.CloudWriteAuthorityMode,
		"FORT_AUTHORITY_EPOCH":             "9",
	}
}
