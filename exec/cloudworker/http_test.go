package cloudworker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/tobsai/fort/cloud/controlapi"
	coreworker "github.com/tobsai/fort/core/worker"
)

func TestHTTPClientUsesAuthenticatedPlaintextWorkerContract(t *testing.T) {
	t.Parallel()

	operations := make([]string, 0, 3)
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodPost || request.Header.Get("Authorization") != "Bearer "+testWorkerToken ||
			request.Header.Get(controlapi.WorkerAccountHeader) != testAccountID ||
			request.Header.Get(controlapi.WorkerIDHeader) != "worker:studio" ||
			request.Header.Get(controlapi.WorkerMachineHeader) != "machine:studio" {
			t.Errorf("worker HTTP identity = %s %#v", request.Method, request.Header)
			return jsonHTTPResponse(http.StatusUnauthorized, `{}`), nil
		}
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Error(err)
			return nil, err
		}
		var envelope map[string]json.RawMessage
		if err := json.Unmarshal(body, &envelope); err != nil {
			t.Error(err)
			return nil, err
		}
		var operation string
		_ = json.Unmarshal(envelope["operation"], &operation)
		operations = append(operations, operation)
		switch operation {
		case "claim_next":
			encoded, _ := json.Marshal(testAssignment(time.Now().UTC()))
			return jsonHTTPResponse(http.StatusOK, string(encoded)), nil
		case "artifact_chunk":
			payload := string(envelope["artifact_chunk"])
			if !strings.Contains(payload, `"plaintext":"aGVsbG8="`) || strings.Contains(payload, "ciphertext") || strings.Contains(payload, "nonce") {
				t.Errorf("artifact plaintext payload = %s", payload)
			}
			return jsonHTTPResponse(http.StatusOK, `{"artifact_id":"artifact:1","chunk_index":0}`), nil
		case "terminal":
			payload := string(envelope["terminal"])
			if !strings.Contains(payload, `"receipt":{"status":"completed"}`) || !strings.Contains(payload, `"output_message":"hello"`) ||
				strings.Contains(payload, "ciphertext") || strings.Contains(payload, "key_id") {
				t.Errorf("terminal plaintext payload = %s", payload)
			}
			return jsonHTTPResponse(http.StatusOK, `{"target_id":"target:1","status":"completed"}`), nil
		default:
			return jsonHTTPResponse(http.StatusBadRequest, `{}`), nil
		}
	})}

	client, err := NewHTTPClient(HTTPConfig{Endpoint: "https://fort-control.example/api/v2/worker", Identity: Identity{
		AccountID: testAccountID, WorkerID: "worker:studio", MachineID: "machine:studio",
	}, Token: testWorkerToken, Client: httpClient})
	if err != nil {
		t.Fatal(err)
	}
	assignment, err := client.ClaimNextWorkerTarget(context.Background(), controlapi.WorkerClaimNextCommand{
		AccountID: testAccountID, WorkerID: "worker:studio", MachineID: "machine:studio",
		ExecutionAttemptID: "attempt:1", LeaseID: "lease:1", IdempotencyKey: "claim:1", CapabilityRevisionID: "capability:7",
	})
	if err != nil || assignment.TargetID != "target:1" {
		t.Fatalf("ClaimNextWorkerTarget = %#v, %v", assignment, err)
	}
	plaintext := []byte("hello")
	digest := sha256.Sum256(plaintext)
	if _, err := client.AppendWorkerArtifactChunk(context.Background(), controlapi.WorkerArtifactChunkCommand{
		AccountID: testAccountID, WorkerID: "worker:studio", MachineID: "machine:studio", TargetID: "target:1",
		ExecutionAttemptID: "attempt:1", LeaseID: "lease:1", FenceToken: 41, IdempotencyKey: "chunk:1", ArtifactID: "artifact:1",
		ChunkIndex: 0, Plaintext: plaintext, PlaintextDigest: hex.EncodeToString(digest[:]),
	}); err != nil {
		t.Fatal(err)
	}
	message := "hello"
	if _, err := client.CommitWorkerTerminal(context.Background(), controlapi.WorkerTerminalCommand{
		AccountID: testAccountID, WorkerID: "worker:studio", MachineID: "machine:studio", TargetID: "target:1",
		ExecutionAttemptID: "attempt:1", LeaseID: "lease:1", FenceToken: 41, IdempotencyKey: "terminal:1",
		TerminalReceiptID: "receipt:1", Status: coreworker.TerminalCompleted,
		ReceiptPlaintext: json.RawMessage(`{"status":"completed"}`), Output: controlapi.WorkerOutputReference{ArtifactID: "artifact:1", Digest: hex.EncodeToString(digest[:])},
		OutputMessagePlaintext: &message,
	}); err != nil {
		t.Fatal(err)
	}
	if strings.Join(operations, ",") != "claim_next,artifact_chunk,terminal" {
		t.Fatalf("operations = %#v", operations)
	}
}

func TestHTTPClientMapsStaleFenceAndRejectsNonTLSRemoteEndpoint(t *testing.T) {
	t.Parallel()

	httpClient := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return jsonHTTPResponse(http.StatusConflict, `{"code":"worker_stale_lease"}`), nil
	})}
	client, err := NewHTTPClient(HTTPConfig{Endpoint: "https://fort-control.example/api/v2/worker", Identity: Identity{
		AccountID: testAccountID, WorkerID: "worker:studio", MachineID: "machine:studio",
	}, Token: testWorkerToken, Client: httpClient})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.HeartbeatWorkerLease(context.Background(), controlapi.WorkerLeaseHeartbeatCommand{IdempotencyKey: "heartbeat:1"})
	if !errors.Is(err, controlapi.ErrWorkerStaleLease) {
		t.Fatalf("heartbeat error = %v, want stale lease", err)
	}
	if _, err := NewHTTPClient(HTTPConfig{Endpoint: "http://fort-control.example/api/v2/worker", Identity: Identity{
		AccountID: testAccountID, WorkerID: "worker:studio", MachineID: "machine:studio",
	}, Token: testWorkerToken}); err == nil {
		t.Fatal("remote plaintext HTTP endpoint was accepted")
	}
}

func TestHTTPClientReadsBoundedPlaintextContextPageForExactFence(t *testing.T) {
	t.Parallel()

	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}
		var envelope map[string]json.RawMessage
		if err := json.Unmarshal(body, &envelope); err != nil {
			return nil, err
		}
		if string(envelope["operation"]) != `"context_page"` {
			t.Errorf("operation = %s", envelope["operation"])
		}
		payload := string(envelope["context_page"])
		for _, expected := range []string{`"target_id":"target:1"`, `"execution_attempt_id":"attempt:1"`,
			`"lease_id":"lease:1"`, `"fence_token":41`, `"cursor":"cursor_one"`} {
			if !strings.Contains(payload, expected) {
				t.Errorf("context payload = %s, missing %s", payload, expected)
			}
		}
		if strings.Contains(payload, "ciphertext") || strings.Contains(payload, "key_id") {
			t.Errorf("context payload exposed encryption fields: %s", payload)
		}
		return jsonHTTPResponse(http.StatusOK, `{"context_manifest_id":"context:1","manifest_digest":"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc","items":[{"kind":"message","ordinal":1,"message":{"message_id":7,"conversation_id":"conversation:1","message_kind":"human","author_kind":"human","author_id":"human:1","body":"pinned","created_at":"2026-08-21T23:00:00Z"}}],"next_cursor":""}`), nil
	})}
	client, err := NewHTTPClient(HTTPConfig{Endpoint: "https://fort-control.example/api/v2/worker", Identity: Identity{
		AccountID: testAccountID, WorkerID: "worker:studio", MachineID: "machine:studio",
	}, Token: testWorkerToken, Client: httpClient})
	if err != nil {
		t.Fatal(err)
	}
	page, err := client.ReadWorkerContextPage(context.Background(), controlapi.WorkerContextPageCommand{
		TargetID: "target:1", ExecutionAttemptID: "attempt:1", LeaseID: "lease:1", FenceToken: 41,
		IdempotencyKey: "context-page:1", Cursor: "cursor_one",
	})
	if err != nil || len(page.Items) != 1 || page.Items[0].Message == nil || page.Items[0].Message.Body != "pinned" {
		t.Fatalf("ReadWorkerContextPage = %#v, %v", page, err)
	}
}

const testWorkerToken = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY"

var _ Control = (*HTTPClient)(nil)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func jsonHTTPResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(body))}
}
