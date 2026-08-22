package controlapi_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tobsai/fort/cloud/controlapi"
	coreworker "github.com/tobsai/fort/core/worker"
)

const workerTestAccountID = "4af424a4-d81a-47d5-a495-400868883b86"

func TestWorkerHandlerAuthenticatesMachineTokenBeforeReadiness(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 20, 0, 0, 0, time.UTC)
	token := base64.RawURLEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	tokenDigest := sha256.Sum256([]byte(token))
	repository := &fakeWorkerRepository{
		credential: controlapi.MachineCredential{
			AccountID: workerTestAccountID,
			WorkerID:  "worker:mac-studio", MachineID: "machine:mac-studio",
			TokenHash: hex.EncodeToString(tokenDigest[:]), State: controlapi.MachineCredentialEnrolled,
		},
	}
	handler := controlapi.WorkerHandler(repository, func() time.Time { return now })

	missing := httptest.NewRecorder()
	handler.ServeHTTP(missing, workerRequest(`{"operation":"readiness"}`, ""))
	if missing.Code != http.StatusUnauthorized {
		t.Fatalf("missing token status = %d, want 401; body=%s", missing.Code, missing.Body.String())
	}
	if repository.credentialReads != 0 || repository.readinessCalls != 0 {
		t.Fatalf("missing token reached repository: credential=%d readiness=%d", repository.credentialReads, repository.readinessCalls)
	}

	wrong := httptest.NewRecorder()
	handler.ServeHTTP(wrong, workerRequest(`{"operation":"readiness"}`, base64.RawURLEncoding.EncodeToString([]byte("abcdef0123456789abcdef0123456789"))))
	if wrong.Code != http.StatusUnauthorized {
		t.Fatalf("wrong token status = %d, want 401; body=%s", wrong.Code, wrong.Body.String())
	}
	if repository.credentialReads != 1 || repository.readinessCalls != 0 {
		t.Fatalf("wrong token calls = credential %d readiness %d, want 1/0", repository.credentialReads, repository.readinessCalls)
	}

	evidence := json.RawMessage(`{"frameworks":["openclaw"],"ready":true}`)
	evidenceDigest := sha256.Sum256(evidence)
	body := `{
		"operation":"readiness",
		"account_id":"` + workerTestAccountID + `",
		"worker_id":"worker:mac-studio",
		"machine_id":"machine:mac-studio",
		"idempotency_key":"readiness:2026-08-21T20:00:00Z",
		"readiness":{
			"capability_revision_id":"capability:7",
			"revision":7,
			"capability_evidence":` + string(evidence) + `,
			"evidence_digest":"` + hex.EncodeToString(evidenceDigest[:]) + `"
		}
	}`
	accepted := httptest.NewRecorder()
	handler.ServeHTTP(accepted, workerRequest(body, token))
	if accepted.Code != http.StatusOK {
		t.Fatalf("readiness status = %d, want 200; body=%s", accepted.Code, accepted.Body.String())
	}
	if repository.readinessCalls != 1 {
		t.Fatalf("readiness calls = %d, want 1", repository.readinessCalls)
	}
	command := repository.readiness
	if command.AccountID != workerTestAccountID || command.WorkerID != "worker:mac-studio" || command.MachineID != "machine:mac-studio" {
		t.Fatalf("authenticated readiness identity = %#v", command)
	}
	if command.ObservedAt != now || command.CapabilityRevisionID != "capability:7" || command.Revision != 7 {
		t.Fatalf("readiness evidence/time = %#v", command)
	}
	if string(command.CapabilityEvidence) != string(evidence) || command.EvidenceDigest != hex.EncodeToString(evidenceDigest[:]) {
		t.Fatalf("readiness evidence = %s/%s", command.CapabilityEvidence, command.EvidenceDigest)
	}
	if strings.Contains(repository.lastTokenHashInput, token) {
		t.Fatal("raw machine token was passed to the credential repository")
	}
}

func TestWorkerHandlerRejectsMalformedRequestAfterAuthentication(t *testing.T) {
	t.Parallel()

	token := base64.RawURLEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	digest := sha256.Sum256([]byte(token))
	repository := &fakeWorkerRepository{credential: controlapi.MachineCredential{
		AccountID: workerTestAccountID, WorkerID: "worker:mac-studio", MachineID: "machine:mac-studio",
		TokenHash: hex.EncodeToString(digest[:]), State: controlapi.MachineCredentialEnrolled,
	}}
	handler := controlapi.WorkerHandler(repository, time.Now)

	request := workerRequest(`{
		"operation":"readiness",
		"account_id":"`+workerTestAccountID+`",
		"worker_id":"worker:mac-studio",
		"machine_id":"machine:mac-studio",
		"idempotency_key":"ready:1",
		"readiness":{},
		"unexpected":true
	}`, token)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("unknown-field status = %d, want 400; body=%s", recorder.Code, recorder.Body.String())
	}
	if repository.credentialReads != 1 || repository.readinessCalls != 0 {
		t.Fatalf("unknown-field calls = credential %d readiness %d, want 1/0", repository.credentialReads, repository.readinessCalls)
	}
}

func TestWorkerHandlerDispatchesFencedLeaseLifecycleCommands(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 20, 5, 0, 0, time.UTC)
	token, repository := authenticatedWorkerRepository()
	handler := controlapi.WorkerHandler(repository, func() time.Time { return now })

	claim := `{
		"operation":"claim","account_id":"` + workerTestAccountID + `",
		"worker_id":"worker:mac-studio","machine_id":"machine:mac-studio",
		"idempotency_key":"claim:1",
		"claim":{"target_id":"target:1","execution_attempt_id":"attempt:1","lease_id":"lease:1","capability_revision_id":"capability:7"}
	}`
	repository.assignment = controlapi.WorkerAssignment{
		TargetID: "target:1", ExecutionAttemptID: "attempt:1", LeaseID: "lease:1", FenceToken: 41,
		WorkerID: "worker:mac-studio", MachineID: "machine:mac-studio", CapabilityRevisionID: "capability:7",
		OutputConversationID: "conversation:agent:researcher", OutputMessageKind: "agent",
		OutputAuthorAgentID: "agent:researcher", MaximumOutputPlaintextBytes: 128 << 20,
		InlineOutputPlaintextBytes: 2 << 20,
		ExpiresAt:                  now.Add(controlapi.DefaultWorkerLease),
	}
	claimResponse := httptest.NewRecorder()
	handler.ServeHTTP(claimResponse, workerRequest(claim, token))
	if claimResponse.Code != http.StatusOK {
		t.Fatalf("claim status = %d, want 200; body=%s", claimResponse.Code, claimResponse.Body.String())
	}
	if repository.claim.ClaimedAt != now || repository.claim.ExpiresAt != now.Add(controlapi.DefaultWorkerLease) ||
		repository.claim.TargetID != "target:1" || repository.claim.ExecutionAttemptID != "attempt:1" || repository.claim.LeaseID != "lease:1" {
		t.Fatalf("claim command = %#v", repository.claim)
	}

	heartbeat := `{
		"operation":"lease_heartbeat","account_id":"` + workerTestAccountID + `",
		"worker_id":"worker:mac-studio","machine_id":"machine:mac-studio",
		"idempotency_key":"heartbeat:1",
		"lease_heartbeat":{"target_id":"target:1","execution_attempt_id":"attempt:1","lease_id":"lease:1","fence_token":41}
	}`
	repository.heartbeatResult = controlapi.WorkerLeaseHeartbeatResult{
		TargetID: "target:1", ExecutionAttemptID: "attempt:1", LeaseID: "lease:1", FenceToken: 41,
		Directive: coreworker.DirectiveCancel, ExpiresAt: now.Add(controlapi.DefaultWorkerLease),
	}
	heartbeatResponse := httptest.NewRecorder()
	handler.ServeHTTP(heartbeatResponse, workerRequest(heartbeat, token))
	if heartbeatResponse.Code != http.StatusOK {
		t.Fatalf("heartbeat status = %d, want 200; body=%s", heartbeatResponse.Code, heartbeatResponse.Body.String())
	}
	if repository.heartbeat.FenceToken != 41 || repository.heartbeat.ObservedAt != now ||
		repository.heartbeat.ExtendUntil != now.Add(controlapi.DefaultWorkerLease) {
		t.Fatalf("heartbeat command = %#v", repository.heartbeat)
	}
	if !strings.Contains(heartbeatResponse.Body.String(), `"directive":"cancel"`) {
		t.Fatalf("heartbeat response = %s, want cancel directive", heartbeatResponse.Body.String())
	}

	cancelAck := `{
		"operation":"cancel_ack","account_id":"` + workerTestAccountID + `",
		"worker_id":"worker:mac-studio","machine_id":"machine:mac-studio",
		"idempotency_key":"cancel-ack:1",
		"cancel_ack":{"target_id":"target:1","execution_attempt_id":"attempt:1","lease_id":"lease:1","fence_token":41,"acknowledgement_id":"cancel-ack:1"}
	}`
	repository.cancelAckResult = controlapi.WorkerCancellationAck{
		AcknowledgementID: "cancel-ack:1", TargetID: "target:1", ExecutionAttemptID: "attempt:1",
		LeaseID: "lease:1", FenceToken: 41, AcknowledgedAt: now,
	}
	cancelResponse := httptest.NewRecorder()
	handler.ServeHTTP(cancelResponse, workerRequest(cancelAck, token))
	if cancelResponse.Code != http.StatusOK {
		t.Fatalf("cancel ack status = %d, want 200; body=%s", cancelResponse.Code, cancelResponse.Body.String())
	}
	if repository.cancelAck.AcknowledgementID != "cancel-ack:1" || repository.cancelAck.AcknowledgedAt != now {
		t.Fatalf("cancel acknowledgement command = %#v", repository.cancelAck)
	}

	outputDigest := strings.Repeat("b", 64)
	terminal := `{
		"operation":"terminal","account_id":"` + workerTestAccountID + `",
		"worker_id":"worker:mac-studio","machine_id":"machine:mac-studio",
		"idempotency_key":"terminal:1",
		"terminal":{
			"target_id":"target:1","execution_attempt_id":"attempt:1","lease_id":"lease:1","fence_token":41,"terminal_receipt_id":"receipt:1","status":"canceled",
			"receipt":{"status":"canceled","reason":"cancel_acknowledged"},
			"output":{"artifact_id":"artifact:output:1","digest":"` + outputDigest + `"}
		}
	}`
	repository.terminalResult = controlapi.WorkerTerminalResult{
		TargetID: "target:1", ExecutionAttemptID: "attempt:1", LeaseID: "lease:1", FenceToken: 41,
		TerminalReceiptID: "receipt:1", Status: coreworker.TerminalCanceled,
		Output:      controlapi.WorkerOutputReference{ArtifactID: "artifact:output:1", Digest: outputDigest},
		CommittedAt: now, Created: true,
	}
	terminalResponse := httptest.NewRecorder()
	handler.ServeHTTP(terminalResponse, workerRequest(terminal, token))
	if terminalResponse.Code != http.StatusOK {
		t.Fatalf("terminal status = %d, want 200; body=%s", terminalResponse.Code, terminalResponse.Body.String())
	}
	if repository.terminal.FenceToken != 41 || repository.terminal.TerminalReceiptID != "receipt:1" || repository.terminal.Status != coreworker.TerminalCanceled ||
		repository.terminal.CommittedAt != now || repository.terminal.Output.ArtifactID != "artifact:output:1" {
		t.Fatalf("terminal command = %#v", repository.terminal)
	}
}

func TestWorkerHandlerClaimsNextCompatibleTargetAndReturnsOnlyPreparedCommand(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 20, 5, 0, 0, time.UTC)
	token, repository := authenticatedWorkerRepository()
	repository.assignment = controlapi.WorkerAssignment{
		TargetID: "target:1", ExecutionAttemptID: "attempt:1", LeaseID: "lease:1", FenceToken: 41,
		WorkerID: "worker:mac-studio", MachineID: "machine:mac-studio", CapabilityRevisionID: "capability:7",
		Pins: coreworker.ExecutionPins{
			AgentID: "agent:researcher", BehaviorRevisionID: "behavior:4", BindingRevisionID: "binding:9",
			SeatID: "seat:researcher", EffectiveAuthoritySnapshot: coreworker.AuthoritySnapshot{
				ID: "grant:1", Revision: "authority:7", Permissions: []string{"message.append"},
			},
		},
		Execution: controlapi.WorkerExecutionBinding{
			ExecutionSourceID: "source:studio", SourceAgentID: "source-agent:researcher",
			OpaqueSourceAgentID: "researcher", FortProfile: "openclaw:researcher", Provider: "openclaw",
			RequestedModel: "openclaw-main", ResolvedModel: "openclaw-main",
			AdapterID: "model.chat.openclaw", AdapterRevision: "adapter:1",
			SourceConfigDigest: strings.Repeat("a", 64), AuthorityID: "authority:binding:1",
			AuthorityRevision: "authority:7", PolicyID: "policy:1", PolicyRevision: "policy:2",
			ReadinessContractID: "ready:openclaw", ReadinessContractRevision: "ready:4",
			Workdir:    "/Users/fort/Workspaces/researcher",
			ComputerID: "machine:mac-studio",
		},
		ContextManifestID: "context:1", Prompt: "Use the frozen evidence.",
		OutputConversationID: "conversation:agent:researcher", OutputMessageKind: "agent",
		OutputAuthorAgentID: "agent:researcher", MaximumOutputPlaintextBytes: 128 << 20,
		InlineOutputPlaintextBytes: 2 << 20,
		ClaimedAt:                  now, ExpiresAt: now.Add(controlapi.DefaultWorkerLease), HardDeadline: now.Add(time.Hour),
	}
	handler := controlapi.WorkerHandler(repository, func() time.Time { return now })
	body := `{
		"operation":"claim_next","account_id":"` + workerTestAccountID + `",
		"worker_id":"worker:mac-studio","machine_id":"machine:mac-studio",
		"idempotency_key":"claim-next:1",
		"claim_next":{"execution_attempt_id":"attempt:1","lease_id":"lease:1","capability_revision_id":"capability:7"}
	}`
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, workerRequest(body, token))
	if response.Code != http.StatusOK {
		t.Fatalf("claim-next status = %d, want 200; body=%s", response.Code, response.Body.String())
	}
	if repository.claimNext.ExecutionAttemptID != "attempt:1" || repository.claimNext.LeaseID != "lease:1" ||
		repository.claimNext.CapabilityRevisionID != "capability:7" || repository.claimNext.ClaimedAt != now ||
		repository.claimNext.ExpiresAt != now.Add(controlapi.DefaultWorkerLease) {
		t.Fatalf("claim-next command = %#v", repository.claimNext)
	}
	var decoded map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["prompt"] != "Use the frozen evidence." {
		t.Fatalf("prepared prompt = %#v", decoded["prompt"])
	}
	if decoded["output_conversation_id"] != "conversation:agent:researcher" || decoded["output_message_kind"] != "agent" ||
		decoded["output_author_agent_id"] != "agent:researcher" || decoded["maximum_output_plaintext_bytes"] != float64(128<<20) ||
		decoded["inline_output_plaintext_bytes"] != float64(2<<20) || decoded["hard_deadline"] == nil {
		t.Fatalf("immutable output contract = %#v", decoded)
	}
	execution, ok := decoded["execution"].(map[string]any)
	if !ok || execution["workdir"] != "/Users/fort/Workspaces/researcher" {
		t.Fatalf("prepared execution workdir = %#v", decoded["execution"])
	}
	encoded := response.Body.String()
	for _, forbidden := range []string{"ciphertext", "key_id", "nonce"} {
		if strings.Contains(encoded, forbidden) {
			t.Fatalf("worker assignment exposed server-only envelope field %q: %s", forbidden, encoded)
		}
	}
}

func TestWorkerHandlerFailsClosedBeforeWritingOversizedAssignment(t *testing.T) {
	t.Parallel()

	token, repository := authenticatedWorkerRepository()
	repository.assignment = controlapi.WorkerAssignment{Prompt: strings.Repeat("x", controlapi.MaximumFunctionBodyBytes)}
	handler := controlapi.WorkerHandler(repository, time.Now)
	body := `{
		"operation":"claim_next","account_id":"` + workerTestAccountID + `",
		"worker_id":"worker:mac-studio","machine_id":"machine:mac-studio",
		"idempotency_key":"claim-next:oversized",
		"claim_next":{"execution_attempt_id":"attempt:1","lease_id":"lease:1","capability_revision_id":"capability:7"}
	}`
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, workerRequest(body, token))
	if response.Code != http.StatusInternalServerError || response.Body.Len() >= controlapi.MaximumFunctionBodyBytes ||
		!strings.Contains(response.Body.String(), `"code":"worker_response_limit"`) {
		t.Fatalf("oversized assignment response = %d bytes=%d body-prefix=%q", response.Code,
			response.Body.Len(), response.Body.String()[:min(response.Body.Len(), 200)])
	}
}

func TestWorkerHandlerAcceptsArtifactOnlyOrDigestMatchedInlineCompletedTerminal(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 20, 5, 0, 0, time.UTC)
	message := "renderable answer"
	messageDigest := sha256.Sum256([]byte(message))
	outputDigest := hex.EncodeToString(messageDigest[:])
	terminalBody := func(outputMessage *string) string {
		terminal := map[string]any{
			"target_id": "target:1", "execution_attempt_id": "attempt:1", "lease_id": "lease:1",
			"fence_token": int64(41), "terminal_receipt_id": "receipt:1", "status": "completed",
			"receipt": map[string]any{"status": "completed", "exit_code": 0},
			"output":  controlapi.WorkerOutputReference{ArtifactID: "artifact:output:1", Digest: outputDigest},
		}
		if outputMessage != nil {
			terminal["output_message"] = *outputMessage
		}
		encoded, err := json.Marshal(map[string]any{
			"operation": "terminal", "account_id": workerTestAccountID,
			"worker_id": "worker:mac-studio", "machine_id": "machine:mac-studio",
			"idempotency_key": "terminal:completed:1", "terminal": terminal,
		})
		if err != nil {
			t.Fatalf("encode terminal request: %v", err)
		}
		return string(encoded)
	}

	token, repository := authenticatedWorkerRepository()
	handler := controlapi.WorkerHandler(repository, func() time.Time { return now })
	accepted := httptest.NewRecorder()
	handler.ServeHTTP(accepted, workerRequest(terminalBody(&message), token))
	if accepted.Code != http.StatusOK {
		t.Fatalf("completed terminal status = %d, want 200; body=%s", accepted.Code, accepted.Body.String())
	}
	if repository.terminal.OutputMessagePlaintext == nil || *repository.terminal.OutputMessagePlaintext != message {
		t.Fatalf("completed terminal output message = %#v", repository.terminal.OutputMessagePlaintext)
	}

	artifactOnlyToken, artifactOnlyRepository := authenticatedWorkerRepository()
	artifactOnlyHandler := controlapi.WorkerHandler(artifactOnlyRepository, func() time.Time { return now })
	artifactOnly := httptest.NewRecorder()
	artifactOnlyHandler.ServeHTTP(artifactOnly, workerRequest(terminalBody(nil), artifactOnlyToken))
	if artifactOnly.Code != http.StatusOK || artifactOnlyRepository.terminal.OutputMessagePlaintext != nil {
		t.Fatalf("artifact-only completed terminal = %d %s, command=%#v", artifactOnly.Code,
			artifactOnly.Body.String(), artifactOnlyRepository.terminal)
	}

	changed := "changed answer"
	for name, invalid := range map[string]*string{"digest mismatch": &changed} {
		t.Run(name, func(t *testing.T) {
			token, repository := authenticatedWorkerRepository()
			handler := controlapi.WorkerHandler(repository, func() time.Time { return now })
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, workerRequest(terminalBody(invalid), token))
			if response.Code != http.StatusBadRequest || repository.terminal.TargetID != "" {
				t.Fatalf("invalid completed terminal = %d %s, command=%#v", response.Code, response.Body.String(), repository.terminal)
			}
		})
	}
}

func TestWorkerHandlerDispatchesResumableOutputArtifactOperations(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 20, 6, 0, 0, time.UTC)
	token, repository := authenticatedWorkerRepository()
	repository.artifact = controlapi.WorkerArtifact{
		ArtifactID: "artifact:output:1", ExecutionAttemptID: "attempt:1", Kind: "output", State: "uploading",
		ExpectedChunkCount: 2, ExpectedPlaintextLength: 11, ExpectedEncodedLength: 43,
		LogicalDigest: strings.Repeat("d", 64), EncryptionKeyID: "key:1", CreatedAt: now,
	}
	handler := controlapi.WorkerHandler(repository, func() time.Time { return now })
	identity := `"target_id":"target:1","execution_attempt_id":"attempt:1","lease_id":"lease:1","fence_token":41`
	base := `"account_id":"` + workerTestAccountID + `","worker_id":"worker:mac-studio","machine_id":"machine:mac-studio"`

	create := `{` + base + `,"operation":"artifact_create","idempotency_key":"artifact:create:1","artifact_create":{` + identity + `,
		"artifact_id":"artifact:output:1","expected_chunk_count":2,"expected_plaintext_length":11,
		"logical_digest":"` + strings.Repeat("d", 64) + `"}}`
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, workerRequest(create, token))
	if response.Code != http.StatusOK {
		t.Fatalf("artifact create = %d %s", response.Code, response.Body.String())
	}
	if repository.artifactCreate.ArtifactID != "artifact:output:1" || repository.artifactCreate.CreatedAt != now ||
		repository.artifactCreate.ExpectedChunkCount != 2 || repository.artifactCreate.FenceToken != 41 {
		t.Fatalf("artifact create command = %#v", repository.artifactCreate)
	}

	status := `{` + base + `,"operation":"artifact_status","idempotency_key":"artifact:status:1","artifact_status":{` + identity + `,"artifact_id":"artifact:output:1"}}`
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, workerRequest(status, token))
	if response.Code != http.StatusOK || repository.artifactStatus.ObservedAt != now || repository.artifactStatus.ArtifactID != "artifact:output:1" {
		t.Fatalf("artifact status = %d %s command=%#v", response.Code, response.Body.String(), repository.artifactStatus)
	}

	plaintext := []byte("hello")
	plaintextDigest := sha256.Sum256(plaintext)
	chunk := `{` + base + `,"operation":"artifact_chunk","idempotency_key":"artifact:chunk:1","artifact_chunk":{` + identity + `,
		"artifact_id":"artifact:output:1","chunk_index":1,"plaintext":"` + base64.StdEncoding.EncodeToString(plaintext) + `","plaintext_length":5,
		"plaintext_digest":"` + hex.EncodeToString(plaintextDigest[:]) + `"}}`
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, workerRequest(chunk, token))
	if response.Code != http.StatusOK {
		t.Fatalf("artifact chunk = %d %s", response.Code, response.Body.String())
	}
	if repository.artifactChunk.ChunkIndex != 1 || repository.artifactChunk.CreatedAt != now ||
		string(repository.artifactChunk.Plaintext) != "hello" || repository.artifactChunk.PlaintextDigest != hex.EncodeToString(plaintextDigest[:]) ||
		len(repository.artifactChunk.Ciphertext) != 0 || repository.artifactChunk.EncryptionKeyID != "" {
		t.Fatalf("artifact chunk command = %#v", repository.artifactChunk)
	}

	finalize := `{` + base + `,"operation":"artifact_finalize","idempotency_key":"artifact:finalize:1","artifact_finalize":{` + identity + `,"artifact_id":"artifact:output:1"}}`
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, workerRequest(finalize, token))
	if response.Code != http.StatusOK || repository.artifactFinalize.FinalizedAt != now || repository.artifactFinalize.ArtifactID != "artifact:output:1" {
		t.Fatalf("artifact finalize = %d %s command=%#v", response.Code, response.Body.String(), repository.artifactFinalize)
	}
}

func TestWorkerHandlerRejectsInvalidArtifactChunksAndMixedPayloads(t *testing.T) {
	t.Parallel()

	token, repository := authenticatedWorkerRepository()
	handler := controlapi.WorkerHandler(repository, func() time.Time {
		return time.Date(2026, 8, 21, 20, 6, 0, 0, time.UTC)
	})
	identity := `"target_id":"target:1","execution_attempt_id":"attempt:1","lease_id":"lease:1","fence_token":41`
	base := `"account_id":"` + workerTestAccountID + `","worker_id":"worker:mac-studio","machine_id":"machine:mac-studio","idempotency_key":"artifact:invalid:1"`
	plaintext := []byte("hello")
	digest := sha256.Sum256(plaintext)
	validChunk := identity + `,"artifact_id":"artifact:output:1","chunk_index":0,"plaintext":"` +
		base64.StdEncoding.EncodeToString(plaintext) + `","plaintext_length":5,"plaintext_digest":"` + hex.EncodeToString(digest[:]) + `"`

	for name, body := range map[string]string{
		"plaintext length mismatch": `{` + base + `,"operation":"artifact_chunk","artifact_chunk":{` + strings.Replace(validChunk, `"plaintext_length":5`, `"plaintext_length":4`, 1) + `}}`,
		"plaintext too large":       `{` + base + `,"operation":"artifact_chunk","artifact_chunk":{` + strings.Replace(validChunk, `"plaintext_length":5`, `"plaintext_length":2097153`, 1) + `}}`,
		"digest mismatch": `{` + base + `,"operation":"artifact_chunk","artifact_chunk":{` + strings.Replace(validChunk,
			hex.EncodeToString(digest[:]), strings.Repeat("f", 64), 1) + `}}`,
		"noncanonical digest": `{` + base + `,"operation":"artifact_chunk","artifact_chunk":{` + strings.Replace(validChunk,
			hex.EncodeToString(digest[:]), strings.ToUpper(hex.EncodeToString(digest[:])), 1) + `}}`,
		"too many chunks": `{` + base + `,"operation":"artifact_create","artifact_create":{` + identity + `,"artifact_id":"artifact:output:1",
			"expected_chunk_count":65,"expected_plaintext_length":1,"logical_digest":"` + strings.Repeat("d", 64) + `"}}`,
		"aggregate too large": `{` + base + `,"operation":"artifact_create","artifact_create":{` + identity + `,"artifact_id":"artifact:output:1",
			"expected_chunk_count":64,"expected_plaintext_length":134217729,"logical_digest":"` + strings.Repeat("d", 64) + `"}}`,
		"mixed payload": `{` + base + `,"operation":"artifact_status","artifact_status":{` + identity + `,"artifact_id":"artifact:output:1"},"artifact_chunk":{` + validChunk + `}}`,
	} {
		t.Run(name, func(t *testing.T) {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, workerRequest(body, token))
			if response.Code != http.StatusBadRequest {
				t.Fatalf("response = %d %s, want 400", response.Code, response.Body.String())
			}
		})
	}
	if repository.artifactChunk.ArtifactID != "" || repository.artifactStatus.ArtifactID != "" {
		t.Fatalf("invalid artifact request reached repository: chunk=%#v status=%#v", repository.artifactChunk, repository.artifactStatus)
	}
}

func TestWorkerHandlerMapsStaleFenceToConflict(t *testing.T) {
	t.Parallel()

	token, repository := authenticatedWorkerRepository()
	repository.heartbeatErr = controlapi.ErrWorkerStaleLease
	handler := controlapi.WorkerHandler(repository, func() time.Time {
		return time.Date(2026, 8, 21, 20, 5, 0, 0, time.UTC)
	})
	body := `{
		"operation":"lease_heartbeat","account_id":"` + workerTestAccountID + `",
		"worker_id":"worker:mac-studio","machine_id":"machine:mac-studio",
		"idempotency_key":"heartbeat:stale",
		"lease_heartbeat":{"target_id":"target:1","execution_attempt_id":"attempt:stale","lease_id":"lease:old","fence_token":40}
	}`
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, workerRequest(body, token))
	if recorder.Code != http.StatusConflict || !strings.Contains(recorder.Body.String(), "worker_stale_lease") {
		t.Fatalf("stale fence response = %d %s, want 409 worker_stale_lease", recorder.Code, recorder.Body.String())
	}
}

func TestWorkerHandlerMapsIncompleteArtifactFinalizationToConflict(t *testing.T) {
	t.Parallel()

	token, repository := authenticatedWorkerRepository()
	repository.artifactFinalizeErr = controlapi.ErrWorkerArtifactIncomplete
	handler := controlapi.WorkerHandler(repository, func() time.Time {
		return time.Date(2026, 8, 21, 20, 7, 0, 0, time.UTC)
	})
	body := `{
		"operation":"artifact_finalize","account_id":"` + workerTestAccountID + `",
		"worker_id":"worker:mac-studio","machine_id":"machine:mac-studio","idempotency_key":"artifact:finalize:incomplete",
		"artifact_finalize":{"target_id":"target:1","execution_attempt_id":"attempt:1","lease_id":"lease:1","fence_token":41,"artifact_id":"artifact:output:1"}
	}`
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, workerRequest(body, token))
	if recorder.Code != http.StatusConflict || !strings.Contains(recorder.Body.String(), "artifact_incomplete") {
		t.Fatalf("incomplete artifact response = %d %s, want 409 artifact_incomplete", recorder.Code, recorder.Body.String())
	}
}

func TestWorkerHandlerFailsClosedOnRepositoryFailure(t *testing.T) {
	t.Parallel()

	token, repository := authenticatedWorkerRepository()
	repository.readinessErr = errors.New("database unavailable")
	handler := controlapi.WorkerHandler(repository, func() time.Time {
		return time.Date(2026, 8, 21, 20, 5, 0, 0, time.UTC)
	})
	evidence := `{"ready":true}`
	digest := sha256.Sum256([]byte(evidence))
	body := `{
		"operation":"readiness","account_id":"` + workerTestAccountID + `",
		"worker_id":"worker:mac-studio","machine_id":"machine:mac-studio","idempotency_key":"readiness:failure",
		"readiness":{"capability_revision_id":"capability:7","revision":7,"capability_evidence":` + evidence + `,"evidence_digest":"` + hex.EncodeToString(digest[:]) + `"}
	}`
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, workerRequest(body, token))
	if recorder.Code != http.StatusServiceUnavailable || !strings.Contains(recorder.Body.String(), "worker_command_failed") {
		t.Fatalf("repository failure response = %d %s, want 503 worker_command_failed", recorder.Code, recorder.Body.String())
	}
}

func TestWorkerHandlerReportsCredentialStoreFailureWithoutDispatchingWork(t *testing.T) {
	t.Parallel()

	token, repository := authenticatedWorkerRepository()
	repository.credentialErr = errors.New("database unavailable")
	handler := controlapi.WorkerHandler(repository, time.Now)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, workerRequest(`{"operation":"readiness"}`, token))

	if recorder.Code != http.StatusServiceUnavailable || !strings.Contains(recorder.Body.String(), "worker_auth_unavailable") {
		t.Fatalf("credential-store failure response = %d %s, want 503 worker_auth_unavailable", recorder.Code, recorder.Body.String())
	}
	if repository.credentialReads != 1 || repository.readinessCalls != 0 {
		t.Fatalf("credential-store failure calls = credential %d readiness %d, want 1/0", repository.credentialReads, repository.readinessCalls)
	}
}

func TestWorkerHandlerRejectsOversizedAuthenticatedBody(t *testing.T) {
	t.Parallel()

	token, repository := authenticatedWorkerRepository()
	handler := controlapi.WorkerHandler(repository, time.Now)
	body := `{"padding":"` + strings.Repeat("a", controlapi.MaximumFunctionBodyBytes) + `"}`
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, workerRequest(body, token))
	if recorder.Code != http.StatusRequestEntityTooLarge || !strings.Contains(recorder.Body.String(), "payload_limit") {
		t.Fatalf("oversized response = %d %s, want 413 payload_limit", recorder.Code, recorder.Body.String())
	}
	if repository.readinessCalls != 0 {
		t.Fatalf("oversized body reached worker command %d times", repository.readinessCalls)
	}
}

func workerRequest(body, token string) *http.Request {
	request := httptest.NewRequest(http.MethodPost, "/api/v2/worker", strings.NewReader(body))
	request.Header.Set("X-Fort-Account-ID", workerTestAccountID)
	request.Header.Set("X-Fort-Worker-ID", "worker:mac-studio")
	request.Header.Set("X-Fort-Machine-ID", "machine:mac-studio")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	return request
}

type fakeWorkerRepository struct {
	credential          controlapi.MachineCredential
	credentialErr       error
	credentialReads     int
	lastTokenHashInput  string
	readinessCalls      int
	readiness           controlapi.WorkerReadinessCommand
	readinessErr        error
	claim               controlapi.WorkerClaimCommand
	claimNext           controlapi.WorkerClaimNextCommand
	assignment          controlapi.WorkerAssignment
	heartbeat           controlapi.WorkerLeaseHeartbeatCommand
	heartbeatResult     controlapi.WorkerLeaseHeartbeatResult
	heartbeatErr        error
	cancelAck           controlapi.WorkerCancellationAckCommand
	cancelAckResult     controlapi.WorkerCancellationAck
	terminal            controlapi.WorkerTerminalCommand
	terminalResult      controlapi.WorkerTerminalResult
	artifactCreate      controlapi.WorkerArtifactCreateCommand
	artifactStatus      controlapi.WorkerArtifactStatusCommand
	artifactChunk       controlapi.WorkerArtifactChunkCommand
	artifactFinalize    controlapi.WorkerArtifactFinalizeCommand
	artifact            controlapi.WorkerArtifact
	artifactFinalizeErr error
}

func (repository *fakeWorkerRepository) MachineCredential(_ context.Context, accountID, workerID, machineID string) (controlapi.MachineCredential, error) {
	repository.credentialReads++
	repository.lastTokenHashInput = accountID + ":" + workerID + ":" + machineID
	return repository.credential, repository.credentialErr
}

func (repository *fakeWorkerRepository) RecordWorkerReadiness(_ context.Context, command controlapi.WorkerReadinessCommand) (controlapi.WorkerReadinessResult, error) {
	repository.readinessCalls++
	repository.readiness = command
	if repository.readinessErr != nil {
		return controlapi.WorkerReadinessResult{}, repository.readinessErr
	}
	return controlapi.WorkerReadinessResult{
		Status: "ready", CapabilityRevisionID: command.CapabilityRevisionID, ObservedAt: command.ObservedAt,
	}, nil
}

func (repository *fakeWorkerRepository) ClaimWorkerTarget(_ context.Context, command controlapi.WorkerClaimCommand) (controlapi.WorkerAssignment, error) {
	repository.claim = command
	return repository.assignment, nil
}

func (repository *fakeWorkerRepository) ClaimNextWorkerTarget(_ context.Context, command controlapi.WorkerClaimNextCommand) (controlapi.WorkerAssignment, error) {
	repository.claimNext = command
	return repository.assignment, nil
}

func (repository *fakeWorkerRepository) HeartbeatWorkerLease(_ context.Context, command controlapi.WorkerLeaseHeartbeatCommand) (controlapi.WorkerLeaseHeartbeatResult, error) {
	repository.heartbeat = command
	return repository.heartbeatResult, repository.heartbeatErr
}

func (repository *fakeWorkerRepository) AcknowledgeWorkerCancellation(_ context.Context, command controlapi.WorkerCancellationAckCommand) (controlapi.WorkerCancellationAck, error) {
	repository.cancelAck = command
	return repository.cancelAckResult, nil
}

func (repository *fakeWorkerRepository) CommitWorkerTerminal(_ context.Context, command controlapi.WorkerTerminalCommand) (controlapi.WorkerTerminalResult, error) {
	repository.terminal = command
	return repository.terminalResult, nil
}

func (repository *fakeWorkerRepository) CreateWorkerArtifact(_ context.Context, command controlapi.WorkerArtifactCreateCommand) (controlapi.WorkerArtifact, error) {
	repository.artifactCreate = command
	return repository.artifact, nil
}

func (repository *fakeWorkerRepository) GetWorkerArtifactStatus(_ context.Context, command controlapi.WorkerArtifactStatusCommand) (controlapi.WorkerArtifact, error) {
	repository.artifactStatus = command
	return repository.artifact, nil
}

func (repository *fakeWorkerRepository) AppendWorkerArtifactChunk(_ context.Context, command controlapi.WorkerArtifactChunkCommand) (controlapi.WorkerArtifactChunk, error) {
	repository.artifactChunk = command
	return controlapi.WorkerArtifactChunk{ArtifactID: command.ArtifactID, ChunkIndex: command.ChunkIndex}, nil
}

func (repository *fakeWorkerRepository) FinalizeWorkerArtifact(_ context.Context, command controlapi.WorkerArtifactFinalizeCommand) (controlapi.WorkerArtifact, error) {
	repository.artifactFinalize = command
	return repository.artifact, repository.artifactFinalizeErr
}

func authenticatedWorkerRepository() (string, *fakeWorkerRepository) {
	token := base64.RawURLEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	digest := sha256.Sum256([]byte(token))
	return token, &fakeWorkerRepository{credential: controlapi.MachineCredential{
		AccountID: workerTestAccountID, WorkerID: "worker:mac-studio", MachineID: "machine:mac-studio",
		TokenHash: hex.EncodeToString(digest[:]), State: controlapi.MachineCredentialEnrolled,
	}}
}
