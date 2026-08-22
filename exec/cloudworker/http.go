package cloudworker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tobsai/fort/cloud/controlapi"
)

type HTTPConfig struct {
	Endpoint string
	Identity Identity
	Token    string
	Client   *http.Client
}

// HTTPClient is the enrolled machine's bounded HTTPS implementation of
// Control. It sends plaintext only to the authenticated control endpoint; it
// has no application AEAD key and no Supabase credential.
type HTTPClient struct {
	endpoint string
	identity Identity
	token    string
	client   *http.Client
}

func NewHTTPClient(config HTTPConfig) (*HTTPClient, error) {
	parsed, err := url.Parse(config.Endpoint)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("%w: worker control endpoint", ErrWorkerInvalid)
	}
	if parsed.Scheme != "https" && !(parsed.Scheme == "http" && loopbackHost(parsed.Hostname())) {
		return nil, fmt.Errorf("%w: worker control endpoint requires HTTPS", ErrWorkerInvalid)
	}
	if _, err := uuid.Parse(config.Identity.AccountID); err != nil || config.Identity.WorkerID == "" || config.Identity.MachineID == "" ||
		len(config.Token) < 32 || strings.ContainsAny(config.Token, " \t\r\n") {
		return nil, fmt.Errorf("%w: worker HTTP identity", ErrWorkerInvalid)
	}
	client := config.Client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &HTTPClient{endpoint: parsed.String(), identity: config.Identity, token: config.Token, client: client}, nil
}

func loopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	address := net.ParseIP(host)
	return address != nil && address.IsLoopback()
}

func (client *HTTPClient) RecordWorkerReadiness(ctx context.Context, command controlapi.WorkerReadinessCommand) (controlapi.WorkerReadinessResult, error) {
	var result controlapi.WorkerReadinessResult
	err := client.do(ctx, "readiness", command.IdempotencyKey, struct {
		CapabilityRevisionID string          `json:"capability_revision_id"`
		Revision             int             `json:"revision"`
		CapabilityEvidence   json.RawMessage `json:"capability_evidence"`
		EvidenceDigest       string          `json:"evidence_digest"`
	}{command.CapabilityRevisionID, command.Revision, command.CapabilityEvidence, command.EvidenceDigest}, &result)
	return result, err
}

func (client *HTTPClient) ClaimNextWorkerTarget(ctx context.Context, command controlapi.WorkerClaimNextCommand) (controlapi.WorkerAssignment, error) {
	var result controlapi.WorkerAssignment
	err := client.do(ctx, "claim_next", command.IdempotencyKey, struct {
		ExecutionAttemptID   string `json:"execution_attempt_id"`
		LeaseID              string `json:"lease_id"`
		CapabilityRevisionID string `json:"capability_revision_id"`
	}{command.ExecutionAttemptID, command.LeaseID, command.CapabilityRevisionID}, &result)
	return result, err
}

func (client *HTTPClient) ReadWorkerContextPage(ctx context.Context, command controlapi.WorkerContextPageCommand) (controlapi.WorkerContextPage, error) {
	var result controlapi.WorkerContextPage
	payload := struct {
		workerLeasePayload
		Cursor string `json:"cursor"`
	}{leasePayload(command.TargetID, command.ExecutionAttemptID, command.LeaseID, command.FenceToken), command.Cursor}
	err := client.doWithLimit(ctx, "context_page", command.IdempotencyKey, payload, &result,
		controlapi.MaximumWorkerContextPageEncodedBytes)
	return result, err
}

func (client *HTTPClient) HeartbeatWorkerLease(ctx context.Context, command controlapi.WorkerLeaseHeartbeatCommand) (controlapi.WorkerLeaseHeartbeatResult, error) {
	var result controlapi.WorkerLeaseHeartbeatResult
	err := client.do(ctx, "lease_heartbeat", command.IdempotencyKey, leasePayload(command.TargetID, command.ExecutionAttemptID,
		command.LeaseID, command.FenceToken), &result)
	return result, err
}

func (client *HTTPClient) AcknowledgeWorkerCancellation(ctx context.Context, command controlapi.WorkerCancellationAckCommand) (controlapi.WorkerCancellationAck, error) {
	var result controlapi.WorkerCancellationAck
	payload := struct {
		workerLeasePayload
		AcknowledgementID string `json:"acknowledgement_id"`
	}{leasePayload(command.TargetID, command.ExecutionAttemptID, command.LeaseID, command.FenceToken), command.AcknowledgementID}
	err := client.do(ctx, "cancel_ack", command.IdempotencyKey, payload, &result)
	return result, err
}

func (client *HTTPClient) CreateWorkerArtifact(ctx context.Context, command controlapi.WorkerArtifactCreateCommand) (controlapi.WorkerArtifact, error) {
	var result controlapi.WorkerArtifact
	payload := struct {
		workerArtifactIdentityPayload
		ExpectedChunkCount      int    `json:"expected_chunk_count"`
		ExpectedPlaintextLength int64  `json:"expected_plaintext_length"`
		LogicalDigest           string `json:"logical_digest"`
	}{artifactPayload(command.TargetID, command.ExecutionAttemptID, command.LeaseID, command.FenceToken, command.ArtifactID),
		command.ExpectedChunkCount, command.ExpectedPlaintextLength, command.LogicalDigest}
	err := client.do(ctx, "artifact_create", command.IdempotencyKey, payload, &result)
	return result, err
}

func (client *HTTPClient) GetWorkerArtifactStatus(ctx context.Context, command controlapi.WorkerArtifactStatusCommand) (controlapi.WorkerArtifact, error) {
	var result controlapi.WorkerArtifact
	err := client.do(ctx, "artifact_status", command.IdempotencyKey,
		artifactPayload(command.TargetID, command.ExecutionAttemptID, command.LeaseID, command.FenceToken, command.ArtifactID), &result)
	return result, err
}

func (client *HTTPClient) AppendWorkerArtifactChunk(ctx context.Context, command controlapi.WorkerArtifactChunkCommand) (controlapi.WorkerArtifactChunk, error) {
	var result controlapi.WorkerArtifactChunk
	payload := struct {
		workerArtifactIdentityPayload
		ChunkIndex      int    `json:"chunk_index"`
		Plaintext       []byte `json:"plaintext"`
		PlaintextLength int    `json:"plaintext_length"`
		PlaintextDigest string `json:"plaintext_digest"`
	}{artifactPayload(command.TargetID, command.ExecutionAttemptID, command.LeaseID, command.FenceToken, command.ArtifactID),
		command.ChunkIndex, command.Plaintext, len(command.Plaintext), command.PlaintextDigest}
	err := client.do(ctx, "artifact_chunk", command.IdempotencyKey, payload, &result)
	return result, err
}

func (client *HTTPClient) FinalizeWorkerArtifact(ctx context.Context, command controlapi.WorkerArtifactFinalizeCommand) (controlapi.WorkerArtifact, error) {
	var result controlapi.WorkerArtifact
	err := client.do(ctx, "artifact_finalize", command.IdempotencyKey,
		artifactPayload(command.TargetID, command.ExecutionAttemptID, command.LeaseID, command.FenceToken, command.ArtifactID), &result)
	return result, err
}

func (client *HTTPClient) CommitWorkerTerminal(ctx context.Context, command controlapi.WorkerTerminalCommand) (controlapi.WorkerTerminalResult, error) {
	var result controlapi.WorkerTerminalResult
	requestPayload := struct {
		workerLeasePayload
		TerminalReceiptID string                           `json:"terminal_receipt_id"`
		Status            string                           `json:"status"`
		Receipt           json.RawMessage                  `json:"receipt"`
		Output            controlapi.WorkerOutputReference `json:"output"`
		OutputMessage     *string                          `json:"output_message,omitempty"`
	}{leasePayload(command.TargetID, command.ExecutionAttemptID, command.LeaseID, command.FenceToken),
		command.TerminalReceiptID, string(command.Status), command.ReceiptPlaintext, command.Output, command.OutputMessagePlaintext}
	err := client.do(ctx, "terminal", command.IdempotencyKey, requestPayload, &result)
	return result, err
}

type workerLeasePayload struct {
	TargetID           string `json:"target_id"`
	ExecutionAttemptID string `json:"execution_attempt_id"`
	LeaseID            string `json:"lease_id"`
	FenceToken         int64  `json:"fence_token"`
}

func leasePayload(targetID, attemptID, leaseID string, fence int64) workerLeasePayload {
	return workerLeasePayload{targetID, attemptID, leaseID, fence}
}

type workerArtifactIdentityPayload struct {
	workerLeasePayload
	ArtifactID string `json:"artifact_id"`
}

func artifactPayload(targetID, attemptID, leaseID string, fence int64, artifactID string) workerArtifactIdentityPayload {
	return workerArtifactIdentityPayload{leasePayload(targetID, attemptID, leaseID, fence), artifactID}
}

func (client *HTTPClient) do(ctx context.Context, operation, idempotencyKey string, payload any, result any) error {
	return client.doWithLimit(ctx, operation, idempotencyKey, payload, result, controlapi.MaximumFunctionBodyBytes)
}

func (client *HTTPClient) doWithLimit(ctx context.Context, operation, idempotencyKey string, payload any, result any, responseLimit int64) error {
	requestBody := map[string]any{
		"operation": operation, "account_id": client.identity.AccountID,
		"worker_id": client.identity.WorkerID, "machine_id": client.identity.MachineID,
		"idempotency_key": idempotencyKey, operation: payload,
	}
	encoded, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("encode worker command: %w", err)
	}
	if len(encoded) > controlapi.MaximumFunctionBodyBytes {
		return fmt.Errorf("%w: worker request exceeds function body limit", ErrWorkerInvalid)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.endpoint, bytes.NewReader(encoded))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+client.token)
	request.Header.Set(controlapi.WorkerAccountHeader, client.identity.AccountID)
	request.Header.Set(controlapi.WorkerIDHeader, client.identity.WorkerID)
	request.Header.Set(controlapi.WorkerMachineHeader, client.identity.MachineID)
	response, err := client.client.Do(request)
	if err != nil {
		return fmt.Errorf("worker control request: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, responseLimit+1))
	if err != nil {
		return fmt.Errorf("read worker control response: %w", err)
	}
	if int64(len(body)) > responseLimit {
		return fmt.Errorf("%w: worker response exceeds function body limit", ErrWorkerInvalid)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return workerHTTPError(response.StatusCode, body)
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(result); err != nil {
		return fmt.Errorf("decode worker control response: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return fmt.Errorf("decode worker control response: trailing data")
	}
	return nil
}

func workerHTTPError(status int, body []byte) error {
	var response struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(body, &response)
	switch response.Code {
	case "worker_target_unavailable":
		return controlapi.ErrWorkerNoCompatibleTarget
	case "worker_stale_lease":
		return controlapi.ErrWorkerStaleLease
	case "idempotency_conflict":
		return controlapi.ErrWorkerIdempotencyConflict
	case "artifact_incomplete":
		return controlapi.ErrWorkerArtifactIncomplete
	case "worker_request_invalid":
		return controlapi.ErrWorkerRequestInvalid
	default:
		return fmt.Errorf("worker control HTTP %d (%s)", status, response.Code)
	}
}
