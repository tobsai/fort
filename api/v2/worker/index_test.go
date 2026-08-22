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

const workerEndpointAccountID = "4af424a4-d81a-47d5-a495-400868883b86"

func TestWorkerEndpointComposesMachineAuthenticatedReadinessOnSharedPool(t *testing.T) {
	t.Parallel()

	token := base64.RawURLEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	bodyKey := base64.RawURLEncoding.EncodeToString([]byte("abcdef0123456789abcdef0123456789"))
	tokenHash := sha256.Sum256([]byte(token))
	evidence := `{"frameworks":["openclaw"],"ready":true}`
	evidenceHash := sha256.Sum256([]byte(evidence))
	store := &fakeWorkerControlStore{credential: controlapi.MachineCredential{
		AccountID: workerEndpointAccountID, WorkerID: "worker:mac-studio", MachineID: "machine:mac-studio",
		TokenHash: hex.EncodeToString(tokenHash[:]), State: controlapi.MachineCredentialEnrolled,
	}}
	opens := 0
	handler := newWorkerEndpoint(func(key string) string {
		switch key {
		case "DATABASE_URL":
			return "postgresql://runtime.test/fort?sslmode=require"
		case "FORT_AUTHORITY_MODE":
			return controlapi.CloudWriteAuthorityMode
		case "FORT_AUTHORITY_EPOCH":
			return "7"
		case "FORT_BODY_ACTIVE_KID":
			return "body-2026-08"
		case "FORT_BODY_KEYS_JSON":
			return `{"body-2026-08":"` + bodyKey + `"}`
		}
		return ""
	}, func(_ context.Context, databaseURL string) (workerControlStore, error) {
		opens++
		if databaseURL != "postgresql://runtime.test/fort?sslmode=require" {
			t.Fatalf("database URL = %q", databaseURL)
		}
		return store, nil
	}, func() time.Time { return time.Date(2026, 8, 21, 20, 0, 0, 0, time.UTC) })

	body := `{
		"operation":"readiness","account_id":"` + workerEndpointAccountID + `",
		"worker_id":"worker:mac-studio","machine_id":"machine:mac-studio","idempotency_key":"readiness:1",
		"readiness":{"capability_revision_id":"capability:1","revision":1,"capability_evidence":` + evidence + `,"evidence_digest":"` + hex.EncodeToString(evidenceHash[:]) + `"}
	}`
	request := httptest.NewRequest(http.MethodPost, "/api/v2/worker", strings.NewReader(body))
	request.Header.Set(controlapi.WorkerAccountHeader, workerEndpointAccountID)
	request.Header.Set(controlapi.WorkerIDHeader, "worker:mac-studio")
	request.Header.Set(controlapi.WorkerMachineHeader, "machine:mac-studio")
	request.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	if opens != 1 || store.readinessCalls != 1 || store.credentialReads != 1 {
		t.Fatalf("opens/credential/readiness = %d/%d/%d, want 1/1/1", opens, store.credentialReads, store.readinessCalls)
	}
}

func TestWorkerEndpointRejectsMissingBodyKeyRingBeforeDatabaseOpen(t *testing.T) {
	t.Parallel()

	opens := 0
	handler := newWorkerEndpoint(func(key string) string {
		switch key {
		case "DATABASE_URL":
			return "postgresql://runtime.test/fort?sslmode=require"
		case "FORT_AUTHORITY_MODE":
			return controlapi.CloudWriteAuthorityMode
		case "FORT_AUTHORITY_EPOCH":
			return "10"
		default:
			return ""
		}
	}, func(context.Context, string) (workerControlStore, error) {
		opens++
		return &fakeWorkerControlStore{}, nil
	}, time.Now)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/v2/worker", strings.NewReader(`{}`)))
	if recorder.Code != http.StatusServiceUnavailable || opens != 0 {
		t.Fatalf("status/opens = %d/%d, want 503/0", recorder.Code, opens)
	}
}

func TestWorkerEndpointRejectsInactiveWriteAuthorityBeforeDatabaseOpen(t *testing.T) {
	t.Parallel()

	opens := 0
	handler := newWorkerEndpoint(func(key string) string {
		if key == "DATABASE_URL" {
			return "postgresql://runtime.test/fort?sslmode=require"
		}
		return ""
	}, func(context.Context, string) (workerControlStore, error) {
		opens++
		return &fakeWorkerControlStore{}, nil
	}, time.Now)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/v2/worker", strings.NewReader(`{}`)))
	if recorder.Code != http.StatusConflict || !strings.Contains(recorder.Body.String(), "write_authority_inactive") || opens != 0 {
		t.Fatalf("status/body/opens = %d/%q/%d, want 409/write_authority_inactive/0", recorder.Code, recorder.Body.String(), opens)
	}
}

func TestWorkerEndpointRejectsMethodAndMissingConfigurationBeforeDatabaseOpen(t *testing.T) {
	t.Parallel()

	opens := 0
	handler := newWorkerEndpoint(func(key string) string {
		switch key {
		case "FORT_AUTHORITY_MODE":
			return controlapi.CloudWriteAuthorityMode
		case "FORT_AUTHORITY_EPOCH":
			return "9"
		default:
			return ""
		}
	}, func(context.Context, string) (workerControlStore, error) {
		opens++
		return &fakeWorkerControlStore{}, nil
	}, time.Now)

	get := httptest.NewRecorder()
	handler.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/api/v2/worker", nil))
	if get.Code != http.StatusMethodNotAllowed || opens != 0 {
		t.Fatalf("GET status/opens = %d/%d, want 405/0", get.Code, opens)
	}

	post := httptest.NewRecorder()
	handler.ServeHTTP(post, httptest.NewRequest(http.MethodPost, "/api/v2/worker", strings.NewReader(`{}`)))
	if post.Code != http.StatusServiceUnavailable || opens != 0 {
		t.Fatalf("unconfigured POST status/opens = %d/%d, want 503/0", post.Code, opens)
	}
}

type fakeWorkerControlStore struct {
	credential      controlapi.MachineCredential
	credentialReads int
	readinessCalls  int
}

func (store *fakeWorkerControlStore) MachineCredential(context.Context, string, string, string) (controlapi.MachineCredential, error) {
	store.credentialReads++
	return store.credential, nil
}

func (store *fakeWorkerControlStore) RecordWorkerReadiness(_ context.Context, command controlapi.WorkerReadinessCommand) (controlapi.WorkerReadinessResult, error) {
	store.readinessCalls++
	return controlapi.WorkerReadinessResult{Status: "ready", CapabilityRevisionID: command.CapabilityRevisionID, ObservedAt: command.ObservedAt}, nil
}

func (*fakeWorkerControlStore) ClaimWorkerTarget(context.Context, controlapi.WorkerClaimCommand) (controlapi.WorkerAssignment, error) {
	panic("unexpected ClaimWorkerTarget")
}

func (*fakeWorkerControlStore) ClaimNextWorkerTarget(context.Context, controlapi.WorkerClaimNextCommand) (controlapi.WorkerAssignment, error) {
	panic("unexpected ClaimNextWorkerTarget")
}

func (*fakeWorkerControlStore) HeartbeatWorkerLease(context.Context, controlapi.WorkerLeaseHeartbeatCommand) (controlapi.WorkerLeaseHeartbeatResult, error) {
	panic("unexpected HeartbeatWorkerLease")
}

func (*fakeWorkerControlStore) AcknowledgeWorkerCancellation(context.Context, controlapi.WorkerCancellationAckCommand) (controlapi.WorkerCancellationAck, error) {
	panic("unexpected AcknowledgeWorkerCancellation")
}

func (*fakeWorkerControlStore) CommitWorkerTerminal(context.Context, controlapi.WorkerTerminalCommand) (controlapi.WorkerTerminalResult, error) {
	panic("unexpected CommitWorkerTerminal")
}

func (*fakeWorkerControlStore) CreateWorkerArtifact(context.Context, controlapi.WorkerArtifactCreateCommand) (controlapi.WorkerArtifact, error) {
	panic("unexpected CreateWorkerArtifact")
}

func (*fakeWorkerControlStore) GetWorkerArtifactStatus(context.Context, controlapi.WorkerArtifactStatusCommand) (controlapi.WorkerArtifact, error) {
	panic("unexpected GetWorkerArtifactStatus")
}

func (*fakeWorkerControlStore) AppendWorkerArtifactChunk(context.Context, controlapi.WorkerArtifactChunkCommand) (controlapi.WorkerArtifactChunk, error) {
	panic("unexpected AppendWorkerArtifactChunk")
}

func (*fakeWorkerControlStore) FinalizeWorkerArtifact(context.Context, controlapi.WorkerArtifactFinalizeCommand) (controlapi.WorkerArtifact, error) {
	panic("unexpected FinalizeWorkerArtifact")
}

func (*fakeWorkerControlStore) Close() error { return nil }
