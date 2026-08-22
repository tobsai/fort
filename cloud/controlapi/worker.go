package controlapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	coreworker "github.com/tobsai/fort/core/worker"
)

const (
	WorkerAccountHeader = "X-Fort-Account-ID"
	WorkerIDHeader      = "X-Fort-Worker-ID"
	WorkerMachineHeader = "X-Fort-Machine-ID"
	DefaultWorkerLease  = 2 * time.Minute
	maximumEvidenceSize = 64 << 10

	// WorkerOutputMessageRecordType is the authenticated-encryption scope for
	// a renderable terminal output before the control plane persists it as an
	// authoritative Conversation message.
	WorkerOutputMessageRecordType = "worker_output_message"

	MaximumArtifactChunkPlaintextBytes = 2 << 20
	MaximumArtifactChunks              = 64
	MaximumArtifactPlaintextBytes      = 128 << 20
	MaximumArtifactEncodedBytes        = MaximumArtifactChunks * MaximumFunctionBodyBytes
)

var (
	ErrWorkerNotFound            = errors.New("worker not found")
	ErrWorkerRevoked             = errors.New("worker credential revoked")
	ErrWorkerNoCompatibleTarget  = errors.New("worker target is not compatible")
	ErrWorkerStaleLease          = errors.New("worker lease or fence is stale")
	ErrWorkerIdempotencyConflict = errors.New("worker idempotency key conflicts")
	ErrWorkerRequestInvalid      = errors.New("worker request is invalid")
	ErrWorkerArtifactIncomplete  = errors.New("worker artifact is incomplete")
	errWorkerUnauthorized        = errors.New("worker credential is invalid")
	workerTokenPattern           = regexp.MustCompile(`^[A-Za-z0-9_-]{43,342}$`)
	workerIdentityPattern        = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,256}$`)
)

type MachineCredentialState string

const (
	MachineCredentialEnrolled MachineCredentialState = "enrolled"
	MachineCredentialOffline  MachineCredentialState = "offline"
	MachineCredentialRevoked  MachineCredentialState = "revoked"
)

// MachineCredential is server-side enrollment evidence. TokenHash is the
// lowercase SHA-256 digest of a random bearer; raw machine tokens are never
// passed into the persistence layer.
type MachineCredential struct {
	AccountID string
	WorkerID  string
	MachineID string
	TokenHash string
	State     MachineCredentialState
}

type WorkerReadinessCommand struct {
	AccountID            string
	WorkerID             string
	MachineID            string
	IdempotencyKey       string
	CapabilityRevisionID string
	Revision             int
	CapabilityEvidence   json.RawMessage
	EvidenceDigest       string
	ObservedAt           time.Time
}

type WorkerReadinessResult struct {
	Status               string    `json:"status"`
	CapabilityRevisionID string    `json:"capability_revision_id"`
	ObservedAt           time.Time `json:"observed_at"`
}

type EncryptedEnvelope struct {
	Ciphertext      []byte `json:"ciphertext"`
	KeyID           string `json:"key_id"`
	Nonce           []byte `json:"nonce"`
	Digest          string `json:"digest"`
	PlaintextLength int    `json:"plaintext_length"`
}

type WorkerClaimCommand struct {
	AccountID            string
	WorkerID             string
	MachineID            string
	TargetID             string
	ExecutionAttemptID   string
	LeaseID              string
	IdempotencyKey       string
	CapabilityRevisionID string
	ClaimedAt            time.Time
	ExpiresAt            time.Time
}

// WorkerClaimNextCommand asks the control plane to atomically select the
// oldest queued target whose immutable Binding is assigned to this exact
// worker and machine. Target selection remains server-side so a worker never
// guesses or scans account work.
type WorkerClaimNextCommand struct {
	AccountID            string
	WorkerID             string
	MachineID            string
	ExecutionAttemptID   string
	LeaseID              string
	IdempotencyKey       string
	CapabilityRevisionID string
	ClaimedAt            time.Time
	ExpiresAt            time.Time
}

// WorkerExecutionBinding is the immutable execution selector copied from the
// target's pinned Agent Binding Revision. It contains no provider credential,
// key material, or server-side application-encryption secret.
type WorkerExecutionBinding struct {
	ExecutionSourceID         string          `json:"execution_source_id"`
	SourceAgentID             string          `json:"source_agent_id"`
	OpaqueSourceAgentID       string          `json:"opaque_source_agent_id"`
	FortProfile               string          `json:"fort_profile"`
	Provider                  string          `json:"provider"`
	RequestedModel            string          `json:"requested_model"`
	ResolvedModel             string          `json:"resolved_model"`
	AdapterID                 string          `json:"adapter_id"`
	AdapterRevision           string          `json:"adapter_revision"`
	SourceConfigDigest        string          `json:"source_config_digest"`
	AuthorityID               string          `json:"authority_id"`
	AuthorityRevision         string          `json:"authority_revision"`
	PolicyID                  string          `json:"policy_id"`
	PolicyRevision            string          `json:"policy_revision"`
	SessionBehavior           string          `json:"session_behavior"`
	MemoryBehavior            string          `json:"memory_behavior"`
	CapabilityEvidence        json.RawMessage `json:"capability_evidence"`
	ReadinessContractID       string          `json:"readiness_contract_id"`
	ReadinessContractRevision string          `json:"readiness_contract_revision"`
	Workdir                   string          `json:"workdir"`
	ComputerID                string          `json:"computer_id,omitempty"`
	CloudRuntime              string          `json:"cloud_runtime,omitempty"`
}

type WorkerAssignment struct {
	TargetID                    string                   `json:"target_id"`
	TargetKind                  string                   `json:"target_kind"`
	OriginID                    string                   `json:"origin_id"`
	ExecutionAttemptID          string                   `json:"execution_attempt_id"`
	LeaseID                     string                   `json:"lease_id"`
	FenceToken                  int64                    `json:"fence_token"`
	WorkerID                    string                   `json:"worker_id"`
	MachineID                   string                   `json:"machine_id"`
	CapabilityRevisionID        string                   `json:"capability_revision_id"`
	Pins                        coreworker.ExecutionPins `json:"pins"`
	Execution                   WorkerExecutionBinding   `json:"execution"`
	ContextManifestID           string                   `json:"context_manifest_id"`
	Prompt                      string                   `json:"prompt"`
	OutputConversationID        string                   `json:"output_conversation_id"`
	OutputMessageKind           string                   `json:"output_message_kind"`
	OutputAuthorAgentID         string                   `json:"output_author_agent_id"`
	MaximumOutputPlaintextBytes int64                    `json:"maximum_output_plaintext_bytes"`
	InlineOutputPlaintextBytes  int64                    `json:"inline_output_plaintext_bytes"`
	// PromptEnvelope exists only between the key-ring-enabled repository and
	// its decryption boundary. It is never serialized to a worker.
	PromptEnvelope EncryptedEnvelope `json:"-"`
	ClaimedAt      time.Time         `json:"claimed_at"`
	ExpiresAt      time.Time         `json:"expires_at"`
	HardDeadline   time.Time         `json:"hard_deadline"`
}

type WorkerLeaseHeartbeatCommand struct {
	AccountID          string
	WorkerID           string
	MachineID          string
	TargetID           string
	ExecutionAttemptID string
	LeaseID            string
	FenceToken         int64
	IdempotencyKey     string
	ObservedAt         time.Time
	ExtendUntil        time.Time
}

type WorkerLeaseHeartbeatResult struct {
	TargetID           string                     `json:"target_id"`
	ExecutionAttemptID string                     `json:"execution_attempt_id"`
	LeaseID            string                     `json:"lease_id"`
	FenceToken         int64                      `json:"fence_token"`
	Directive          coreworker.WorkerDirective `json:"directive"`
	ExpiresAt          time.Time                  `json:"expires_at"`
}

type WorkerCancellationAckCommand struct {
	AccountID          string
	WorkerID           string
	MachineID          string
	TargetID           string
	ExecutionAttemptID string
	LeaseID            string
	FenceToken         int64
	AcknowledgementID  string
	IdempotencyKey     string
	AcknowledgedAt     time.Time
}

type WorkerCancellationAck struct {
	AcknowledgementID  string    `json:"acknowledgement_id"`
	TargetID           string    `json:"target_id"`
	ExecutionAttemptID string    `json:"execution_attempt_id"`
	LeaseID            string    `json:"lease_id"`
	FenceToken         int64     `json:"fence_token"`
	AcknowledgedAt     time.Time `json:"acknowledged_at"`
}

type WorkerOutputReference struct {
	ArtifactID string `json:"artifact_id"`
	Digest     string `json:"digest"`
}

// WorkerOutputMessageRecordID binds a renderable output envelope to one exact
// target execution fence. Account identity is the remaining securebody scope
// component, so a stale or cross-account envelope cannot be replayed.
func WorkerOutputMessageRecordID(targetID, executionAttemptID string, fenceToken int64) string {
	return targetID + "/" + executionAttemptID + "/" + strconv.FormatInt(fenceToken, 10)
}

type WorkerTerminalCommand struct {
	AccountID              string
	WorkerID               string
	MachineID              string
	TargetID               string
	ExecutionAttemptID     string
	LeaseID                string
	FenceToken             int64
	TerminalReceiptID      string
	IdempotencyKey         string
	Status                 coreworker.TerminalStatus
	ReceiptPlaintext       json.RawMessage
	OutputMessagePlaintext *string
	// Receipt and OutputMessage are populated only by the key-ring-enabled
	// repository. Authenticated workers submit bounded plaintext over TLS and
	// never receive or use the application AEAD key.
	Receipt       EncryptedEnvelope
	Output        WorkerOutputReference
	OutputMessage *EncryptedEnvelope
	CommittedAt   time.Time
}

type WorkerTerminalResult struct {
	TargetID           string                    `json:"target_id"`
	ExecutionAttemptID string                    `json:"execution_attempt_id"`
	LeaseID            string                    `json:"lease_id"`
	FenceToken         int64                     `json:"fence_token"`
	TerminalReceiptID  string                    `json:"terminal_receipt_id"`
	Status             coreworker.TerminalStatus `json:"status"`
	Output             WorkerOutputReference     `json:"output"`
	MessageID          int64                     `json:"message_id,omitempty"`
	CommittedAt        time.Time                 `json:"committed_at"`
	Created            bool                      `json:"created"`
}

// WorkerArtifact is the resumable upload projection returned to a worker. It
// deliberately contains manifest and receipt metadata only; encrypted chunk
// bodies remain private Postgres data and never inflate a status response.
type WorkerArtifact struct {
	ArtifactID              string                `json:"artifact_id"`
	ExecutionAttemptID      string                `json:"execution_attempt_id"`
	Kind                    string                `json:"kind"`
	State                   string                `json:"state"`
	ExpectedChunkCount      int                   `json:"expected_chunk_count"`
	ExpectedPlaintextLength int64                 `json:"expected_plaintext_length"`
	ExpectedEncodedLength   int64                 `json:"expected_encoded_length"`
	LogicalDigest           string                `json:"logical_digest"`
	EncryptionKeyID         string                `json:"encryption_key_id"`
	Chunks                  []WorkerArtifactChunk `json:"chunks"`
	CreatedAt               time.Time             `json:"created_at"`
	FinalizedAt             *time.Time            `json:"finalized_at,omitempty"`
	Created                 bool                  `json:"created"`
}

type WorkerArtifactChunk struct {
	ArtifactID          string    `json:"artifact_id"`
	ChunkIndex          int       `json:"chunk_index"`
	EncodedLength       int       `json:"encoded_length"`
	PlaintextLength     int       `json:"plaintext_length"`
	EncryptionKeyID     string    `json:"encryption_key_id"`
	AuthenticatedDigest string    `json:"authenticated_digest"`
	CreatedAt           time.Time `json:"created_at"`
	Created             bool      `json:"created"`
}

type WorkerArtifactCreateCommand struct {
	AccountID, WorkerID, MachineID, TargetID, ExecutionAttemptID, LeaseID string
	FenceToken                                                            int64
	IdempotencyKey, ArtifactID                                            string
	ExpectedChunkCount                                                    int
	ExpectedPlaintextLength, ExpectedEncodedLength                        int64
	LogicalDigest, EncryptionKeyID                                        string
	CreatedAt                                                             time.Time
}

type WorkerArtifactStatusCommand struct {
	AccountID, WorkerID, MachineID, TargetID, ExecutionAttemptID, LeaseID string
	FenceToken                                                            int64
	IdempotencyKey, ArtifactID                                            string
	ObservedAt                                                            time.Time
}

type WorkerArtifactChunkCommand struct {
	AccountID, WorkerID, MachineID, TargetID, ExecutionAttemptID, LeaseID string
	FenceToken                                                            int64
	IdempotencyKey, ArtifactID                                            string
	ChunkIndex                                                            int
	Plaintext                                                             []byte
	PlaintextDigest                                                       string
	// The remaining fields are populated only by the key-ring-enabled
	// repository after it encrypts Plaintext. They are never worker input.
	Ciphertext                     []byte
	EncodedLength, PlaintextLength int
	EncryptionKeyID                string
	Nonce                          []byte
	AuthenticatedDigest            string
	CreatedAt                      time.Time
}

type WorkerArtifactFinalizeCommand struct {
	AccountID, WorkerID, MachineID, TargetID, ExecutionAttemptID, LeaseID string
	FenceToken                                                            int64
	IdempotencyKey, ArtifactID                                            string
	FinalizedAt                                                           time.Time
}

// WorkerRepository is the narrow server-side command seam. Implementations
// receive an already authenticated account/worker/machine identity and retain
// all atomic claim, lease, fence, and receipt rules inside the ledger adapter.
type WorkerRepository interface {
	MachineCredential(context.Context, string, string, string) (MachineCredential, error)
	RecordWorkerReadiness(context.Context, WorkerReadinessCommand) (WorkerReadinessResult, error)
	ClaimWorkerTarget(context.Context, WorkerClaimCommand) (WorkerAssignment, error)
	ClaimNextWorkerTarget(context.Context, WorkerClaimNextCommand) (WorkerAssignment, error)
	HeartbeatWorkerLease(context.Context, WorkerLeaseHeartbeatCommand) (WorkerLeaseHeartbeatResult, error)
	AcknowledgeWorkerCancellation(context.Context, WorkerCancellationAckCommand) (WorkerCancellationAck, error)
	CommitWorkerTerminal(context.Context, WorkerTerminalCommand) (WorkerTerminalResult, error)
	CreateWorkerArtifact(context.Context, WorkerArtifactCreateCommand) (WorkerArtifact, error)
	GetWorkerArtifactStatus(context.Context, WorkerArtifactStatusCommand) (WorkerArtifact, error)
	AppendWorkerArtifactChunk(context.Context, WorkerArtifactChunkCommand) (WorkerArtifactChunk, error)
	FinalizeWorkerArtifact(context.Context, WorkerArtifactFinalizeCommand) (WorkerArtifact, error)
}

type workerRequestBody struct {
	Operation        string                         `json:"operation"`
	AccountID        string                         `json:"account_id"`
	WorkerID         string                         `json:"worker_id"`
	MachineID        string                         `json:"machine_id"`
	IdempotencyKey   string                         `json:"idempotency_key"`
	Readiness        *workerReadinessPayload        `json:"readiness,omitempty"`
	Claim            *workerClaimPayload            `json:"claim,omitempty"`
	ClaimNext        *workerClaimNextPayload        `json:"claim_next,omitempty"`
	LeaseHeartbeat   *workerHeartbeatPayload        `json:"lease_heartbeat,omitempty"`
	CancelAck        *workerCancellationPayload     `json:"cancel_ack,omitempty"`
	Terminal         *workerTerminalPayload         `json:"terminal,omitempty"`
	ArtifactCreate   *workerArtifactCreatePayload   `json:"artifact_create,omitempty"`
	ArtifactStatus   *workerArtifactIdentityPayload `json:"artifact_status,omitempty"`
	ArtifactChunk    *workerArtifactChunkPayload    `json:"artifact_chunk,omitempty"`
	ArtifactFinalize *workerArtifactIdentityPayload `json:"artifact_finalize,omitempty"`
	ContextPage      *workerContextPagePayload      `json:"context_page,omitempty"`
}

type workerReadinessPayload struct {
	CapabilityRevisionID string          `json:"capability_revision_id"`
	Revision             int             `json:"revision"`
	CapabilityEvidence   json.RawMessage `json:"capability_evidence"`
	EvidenceDigest       string          `json:"evidence_digest"`
}

type workerClaimPayload struct {
	TargetID             string `json:"target_id"`
	ExecutionAttemptID   string `json:"execution_attempt_id"`
	LeaseID              string `json:"lease_id"`
	CapabilityRevisionID string `json:"capability_revision_id"`
}

type workerClaimNextPayload struct {
	ExecutionAttemptID   string `json:"execution_attempt_id"`
	LeaseID              string `json:"lease_id"`
	CapabilityRevisionID string `json:"capability_revision_id"`
}

type workerHeartbeatPayload struct {
	TargetID           string `json:"target_id"`
	ExecutionAttemptID string `json:"execution_attempt_id"`
	LeaseID            string `json:"lease_id"`
	FenceToken         int64  `json:"fence_token"`
}

type workerCancellationPayload struct {
	TargetID           string `json:"target_id"`
	ExecutionAttemptID string `json:"execution_attempt_id"`
	LeaseID            string `json:"lease_id"`
	FenceToken         int64  `json:"fence_token"`
	AcknowledgementID  string `json:"acknowledgement_id"`
}

type workerTerminalPayload struct {
	TargetID           string                    `json:"target_id"`
	ExecutionAttemptID string                    `json:"execution_attempt_id"`
	LeaseID            string                    `json:"lease_id"`
	FenceToken         int64                     `json:"fence_token"`
	TerminalReceiptID  string                    `json:"terminal_receipt_id"`
	Status             coreworker.TerminalStatus `json:"status"`
	Receipt            json.RawMessage           `json:"receipt"`
	Output             WorkerOutputReference     `json:"output"`
	OutputMessage      *string                   `json:"output_message,omitempty"`
}

type workerArtifactIdentityPayload struct {
	TargetID           string `json:"target_id"`
	ExecutionAttemptID string `json:"execution_attempt_id"`
	LeaseID            string `json:"lease_id"`
	FenceToken         int64  `json:"fence_token"`
	ArtifactID         string `json:"artifact_id"`
}

type workerArtifactCreatePayload struct {
	workerArtifactIdentityPayload
	ExpectedChunkCount      int    `json:"expected_chunk_count"`
	ExpectedPlaintextLength int64  `json:"expected_plaintext_length"`
	LogicalDigest           string `json:"logical_digest"`
}

type workerArtifactChunkPayload struct {
	workerArtifactIdentityPayload
	ChunkIndex      int    `json:"chunk_index"`
	Plaintext       []byte `json:"plaintext"`
	PlaintextLength int    `json:"plaintext_length"`
	PlaintextDigest string `json:"plaintext_digest"`
}

// WorkerHandler serves one bounded POST /api/v2/worker command. Machine
// authentication is intentionally independent from owner assertions and cron
// secrets, and completes before a request can invoke account-scoped work.
func WorkerHandler(repository WorkerRepository, clock func() time.Time) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Cache-Control", "no-store")
		response.Header().Set("Content-Type", "application/json")
		if request.Method != http.MethodPost {
			response.Header().Set("Allow", http.MethodPost)
			writeJSON(response, http.StatusMethodNotAllowed, map[string]string{"code": "method_not_allowed"})
			return
		}
		if repository == nil || clock == nil {
			writeJSON(response, http.StatusServiceUnavailable, map[string]string{"code": "worker_unavailable"})
			return
		}

		principal, err := authenticateWorker(request, repository)
		if err != nil {
			if !errors.Is(err, errWorkerUnauthorized) {
				writeJSON(response, http.StatusServiceUnavailable, map[string]string{"code": "worker_auth_unavailable"})
				return
			}
			response.Header().Set("WWW-Authenticate", "Bearer")
			writeJSON(response, http.StatusUnauthorized, map[string]string{"code": "worker_unauthorized"})
			return
		}

		body, err := decodeWorkerRequest(response, request)
		if err != nil {
			var maxBytesError *http.MaxBytesError
			if errors.As(err, &maxBytesError) {
				writeJSON(response, http.StatusRequestEntityTooLarge, map[string]string{"code": "payload_limit"})
				return
			}
			writeJSON(response, http.StatusBadRequest, map[string]string{"code": "worker_request_invalid"})
			return
		}
		if body.AccountID != principal.AccountID || body.WorkerID != principal.WorkerID || body.MachineID != principal.MachineID {
			writeJSON(response, http.StatusBadRequest, map[string]string{"code": "worker_request_invalid"})
			return
		}
		now := clock().UTC()
		if now.IsZero() || !workerIdentityPattern.MatchString(body.IdempotencyKey) {
			writeJSON(response, http.StatusBadRequest, map[string]string{"code": "worker_request_invalid"})
			return
		}

		result, status, err := executeWorkerRequest(request.Context(), repository, principal, body, now)
		if err != nil {
			writeWorkerCommandError(response, err)
			return
		}
		writeBoundedWorkerJSON(response, status, result)
	})
}

func writeBoundedWorkerJSON(response http.ResponseWriter, status int, result any) {
	encoded, err := json.Marshal(result)
	if err != nil || len(encoded)+1 > MaximumFunctionBodyBytes {
		writeJSON(response, http.StatusInternalServerError, map[string]string{"code": "worker_response_limit"})
		return
	}
	response.WriteHeader(status)
	_, _ = response.Write(append(encoded, '\n'))
}

func authenticateWorker(request *http.Request, repository WorkerRepository) (MachineCredential, error) {
	accountID, accountOK := oneCanonicalUUIDHeader(request, WorkerAccountHeader)
	workerID, workerOK := oneIdentityHeader(request, WorkerIDHeader)
	machineID, machineOK := oneIdentityHeader(request, WorkerMachineHeader)
	authorizations := request.Header.Values("Authorization")
	if !accountOK || !workerOK || !machineOK || len(authorizations) != 1 {
		return MachineCredential{}, errWorkerUnauthorized
	}
	authorization := authorizations[0]
	if !strings.HasPrefix(authorization, "Bearer ") {
		return MachineCredential{}, errWorkerUnauthorized
	}
	token := strings.TrimPrefix(authorization, "Bearer ")
	if !workerTokenPattern.MatchString(token) {
		return MachineCredential{}, errWorkerUnauthorized
	}
	credential, err := repository.MachineCredential(request.Context(), accountID, workerID, machineID)
	if errors.Is(err, ErrWorkerNotFound) || errors.Is(err, ErrWorkerRevoked) {
		return MachineCredential{}, errWorkerUnauthorized
	}
	if err != nil {
		return MachineCredential{}, fmt.Errorf("load machine credential: %w", err)
	}
	if credential.AccountID != accountID || credential.WorkerID != workerID || credential.MachineID != machineID ||
		credential.State == MachineCredentialRevoked || !lowerSHA256Digest(credential.TokenHash) {
		return MachineCredential{}, errWorkerUnauthorized
	}
	digest := sha256.Sum256([]byte(token))
	if subtle.ConstantTimeCompare([]byte(hex.EncodeToString(digest[:])), []byte(credential.TokenHash)) != 1 {
		return MachineCredential{}, errWorkerUnauthorized
	}
	return credential, nil
}

func decodeWorkerRequest(response http.ResponseWriter, request *http.Request) (workerRequestBody, error) {
	request.Body = http.MaxBytesReader(response, request.Body, MaximumFunctionBodyBytes)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	var body workerRequestBody
	if err := decoder.Decode(&body); err != nil {
		return workerRequestBody{}, err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return workerRequestBody{}, fmt.Errorf("worker request must contain one JSON object")
	}
	return body, nil
}

func executeWorkerRequest(ctx context.Context, repository WorkerRepository, principal MachineCredential, body workerRequestBody, now time.Time) (any, int, error) {
	payloads := 0
	for _, present := range []bool{body.Readiness != nil, body.Claim != nil, body.ClaimNext != nil, body.LeaseHeartbeat != nil, body.CancelAck != nil, body.Terminal != nil,
		body.ArtifactCreate != nil, body.ArtifactStatus != nil, body.ArtifactChunk != nil, body.ArtifactFinalize != nil, body.ContextPage != nil} {
		if present {
			payloads++
		}
	}
	if payloads != 1 {
		return nil, 0, fmt.Errorf("%w: operation requires exactly one payload", ErrWorkerRequestInvalid)
	}

	switch body.Operation {
	case "readiness":
		if body.Readiness == nil || !validReadiness(*body.Readiness) {
			return nil, 0, fmt.Errorf("%w: readiness", ErrWorkerRequestInvalid)
		}
		result, err := repository.RecordWorkerReadiness(ctx, WorkerReadinessCommand{
			AccountID: principal.AccountID, WorkerID: principal.WorkerID, MachineID: principal.MachineID,
			IdempotencyKey: body.IdempotencyKey, CapabilityRevisionID: body.Readiness.CapabilityRevisionID,
			Revision: body.Readiness.Revision, CapabilityEvidence: append(json.RawMessage(nil), body.Readiness.CapabilityEvidence...),
			EvidenceDigest: body.Readiness.EvidenceDigest, ObservedAt: now,
		})
		return result, http.StatusOK, err
	case "claim":
		if body.Claim == nil || !validClaim(*body.Claim) {
			return nil, 0, fmt.Errorf("%w: claim", ErrWorkerRequestInvalid)
		}
		result, err := repository.ClaimWorkerTarget(ctx, WorkerClaimCommand{
			AccountID: principal.AccountID, WorkerID: principal.WorkerID, MachineID: principal.MachineID,
			TargetID: body.Claim.TargetID, ExecutionAttemptID: body.Claim.ExecutionAttemptID,
			LeaseID: body.Claim.LeaseID, IdempotencyKey: body.IdempotencyKey,
			CapabilityRevisionID: body.Claim.CapabilityRevisionID,
			ClaimedAt:            now, ExpiresAt: now.Add(DefaultWorkerLease),
		})
		return result, http.StatusOK, err
	case "claim_next":
		if body.ClaimNext == nil || !validClaimNext(*body.ClaimNext) {
			return nil, 0, fmt.Errorf("%w: claim next", ErrWorkerRequestInvalid)
		}
		result, err := repository.ClaimNextWorkerTarget(ctx, WorkerClaimNextCommand{
			AccountID: principal.AccountID, WorkerID: principal.WorkerID, MachineID: principal.MachineID,
			ExecutionAttemptID: body.ClaimNext.ExecutionAttemptID, LeaseID: body.ClaimNext.LeaseID,
			IdempotencyKey: body.IdempotencyKey, CapabilityRevisionID: body.ClaimNext.CapabilityRevisionID,
			ClaimedAt: now, ExpiresAt: now.Add(DefaultWorkerLease),
		})
		return result, http.StatusOK, err
	case "lease_heartbeat":
		if body.LeaseHeartbeat == nil || !validLeaseIdentity(body.LeaseHeartbeat.TargetID, body.LeaseHeartbeat.ExecutionAttemptID, body.LeaseHeartbeat.LeaseID, body.LeaseHeartbeat.FenceToken) {
			return nil, 0, fmt.Errorf("%w: lease heartbeat", ErrWorkerRequestInvalid)
		}
		result, err := repository.HeartbeatWorkerLease(ctx, WorkerLeaseHeartbeatCommand{
			AccountID: principal.AccountID, WorkerID: principal.WorkerID, MachineID: principal.MachineID,
			TargetID: body.LeaseHeartbeat.TargetID, ExecutionAttemptID: body.LeaseHeartbeat.ExecutionAttemptID,
			LeaseID: body.LeaseHeartbeat.LeaseID, FenceToken: body.LeaseHeartbeat.FenceToken,
			IdempotencyKey: body.IdempotencyKey, ObservedAt: now, ExtendUntil: now.Add(DefaultWorkerLease),
		})
		return result, http.StatusOK, err
	case "cancel_ack":
		if body.CancelAck == nil || !validLeaseIdentity(body.CancelAck.TargetID, body.CancelAck.ExecutionAttemptID, body.CancelAck.LeaseID, body.CancelAck.FenceToken) ||
			!workerIdentityPattern.MatchString(body.CancelAck.AcknowledgementID) {
			return nil, 0, fmt.Errorf("%w: cancellation acknowledgement", ErrWorkerRequestInvalid)
		}
		result, err := repository.AcknowledgeWorkerCancellation(ctx, WorkerCancellationAckCommand{
			AccountID: principal.AccountID, WorkerID: principal.WorkerID, MachineID: principal.MachineID,
			TargetID: body.CancelAck.TargetID, ExecutionAttemptID: body.CancelAck.ExecutionAttemptID,
			LeaseID: body.CancelAck.LeaseID, FenceToken: body.CancelAck.FenceToken,
			AcknowledgementID: body.CancelAck.AcknowledgementID, IdempotencyKey: body.IdempotencyKey, AcknowledgedAt: now,
		})
		return result, http.StatusOK, err
	case "terminal":
		if body.Terminal == nil || !validLeaseIdentity(body.Terminal.TargetID, body.Terminal.ExecutionAttemptID, body.Terminal.LeaseID, body.Terminal.FenceToken) ||
			!workerIdentityPattern.MatchString(body.Terminal.TerminalReceiptID) || !validTerminalStatus(body.Terminal.Status) ||
			!validTerminalReceipt(body.Terminal.Receipt) || !validOutputReference(body.Terminal.Output) ||
			!validTerminalOutputMessage(body.Terminal.Status, body.Terminal.Output, body.Terminal.OutputMessage) {
			return nil, 0, fmt.Errorf("%w: terminal receipt", ErrWorkerRequestInvalid)
		}
		result, err := repository.CommitWorkerTerminal(ctx, WorkerTerminalCommand{
			AccountID: principal.AccountID, WorkerID: principal.WorkerID, MachineID: principal.MachineID,
			TargetID: body.Terminal.TargetID, ExecutionAttemptID: body.Terminal.ExecutionAttemptID,
			LeaseID: body.Terminal.LeaseID, FenceToken: body.Terminal.FenceToken,
			TerminalReceiptID: body.Terminal.TerminalReceiptID,
			IdempotencyKey:    body.IdempotencyKey, Status: body.Terminal.Status,
			ReceiptPlaintext: append(json.RawMessage(nil), body.Terminal.Receipt...), Output: body.Terminal.Output,
			OutputMessagePlaintext: body.Terminal.OutputMessage, CommittedAt: now,
		})
		return result, http.StatusOK, err
	case "artifact_create":
		if body.ArtifactCreate == nil || !validArtifactCreate(*body.ArtifactCreate) {
			return nil, 0, fmt.Errorf("%w: artifact create", ErrWorkerRequestInvalid)
		}
		payload := body.ArtifactCreate
		result, err := repository.CreateWorkerArtifact(ctx, WorkerArtifactCreateCommand{
			AccountID: principal.AccountID, WorkerID: principal.WorkerID, MachineID: principal.MachineID,
			TargetID: payload.TargetID, ExecutionAttemptID: payload.ExecutionAttemptID, LeaseID: payload.LeaseID,
			FenceToken: payload.FenceToken, IdempotencyKey: body.IdempotencyKey, ArtifactID: payload.ArtifactID,
			ExpectedChunkCount: payload.ExpectedChunkCount, ExpectedPlaintextLength: payload.ExpectedPlaintextLength,
			LogicalDigest: payload.LogicalDigest, CreatedAt: now,
		})
		return result, http.StatusOK, err
	case "artifact_status":
		if body.ArtifactStatus == nil || !validArtifactIdentity(*body.ArtifactStatus) {
			return nil, 0, fmt.Errorf("%w: artifact status", ErrWorkerRequestInvalid)
		}
		payload := body.ArtifactStatus
		result, err := repository.GetWorkerArtifactStatus(ctx, WorkerArtifactStatusCommand{
			AccountID: principal.AccountID, WorkerID: principal.WorkerID, MachineID: principal.MachineID,
			TargetID: payload.TargetID, ExecutionAttemptID: payload.ExecutionAttemptID, LeaseID: payload.LeaseID,
			FenceToken: payload.FenceToken, IdempotencyKey: body.IdempotencyKey, ArtifactID: payload.ArtifactID, ObservedAt: now,
		})
		return result, http.StatusOK, err
	case "artifact_chunk":
		if body.ArtifactChunk == nil || !validArtifactChunk(*body.ArtifactChunk) {
			return nil, 0, fmt.Errorf("%w: artifact chunk", ErrWorkerRequestInvalid)
		}
		payload := body.ArtifactChunk
		result, err := repository.AppendWorkerArtifactChunk(ctx, WorkerArtifactChunkCommand{
			AccountID: principal.AccountID, WorkerID: principal.WorkerID, MachineID: principal.MachineID,
			TargetID: payload.TargetID, ExecutionAttemptID: payload.ExecutionAttemptID, LeaseID: payload.LeaseID,
			FenceToken: payload.FenceToken, IdempotencyKey: body.IdempotencyKey, ArtifactID: payload.ArtifactID,
			ChunkIndex: payload.ChunkIndex, Plaintext: append([]byte(nil), payload.Plaintext...),
			PlaintextLength: payload.PlaintextLength, PlaintextDigest: payload.PlaintextDigest, CreatedAt: now,
		})
		return result, http.StatusOK, err
	case "artifact_finalize":
		if body.ArtifactFinalize == nil || !validArtifactIdentity(*body.ArtifactFinalize) {
			return nil, 0, fmt.Errorf("%w: artifact finalize", ErrWorkerRequestInvalid)
		}
		payload := body.ArtifactFinalize
		result, err := repository.FinalizeWorkerArtifact(ctx, WorkerArtifactFinalizeCommand{
			AccountID: principal.AccountID, WorkerID: principal.WorkerID, MachineID: principal.MachineID,
			TargetID: payload.TargetID, ExecutionAttemptID: payload.ExecutionAttemptID, LeaseID: payload.LeaseID,
			FenceToken: payload.FenceToken, IdempotencyKey: body.IdempotencyKey, ArtifactID: payload.ArtifactID, FinalizedAt: now,
		})
		return result, http.StatusOK, err
	case "context_page":
		return executeWorkerContextPage(ctx, repository, principal, body, now)
	default:
		return nil, 0, fmt.Errorf("%w: operation", ErrWorkerRequestInvalid)
	}
}

func validReadiness(payload workerReadinessPayload) bool {
	if !workerIdentityPattern.MatchString(payload.CapabilityRevisionID) || payload.Revision < 1 || payload.Revision > 1_000_000 ||
		len(payload.CapabilityEvidence) == 0 || len(payload.CapabilityEvidence) > maximumEvidenceSize || !lowerSHA256Digest(payload.EvidenceDigest) {
		return false
	}
	var object map[string]any
	if err := json.Unmarshal(payload.CapabilityEvidence, &object); err != nil || object == nil {
		return false
	}
	digest := sha256.Sum256(payload.CapabilityEvidence)
	return subtle.ConstantTimeCompare([]byte(payload.EvidenceDigest), []byte(hex.EncodeToString(digest[:]))) == 1
}

func validClaim(payload workerClaimPayload) bool {
	return workerIdentityPattern.MatchString(payload.TargetID) &&
		workerIdentityPattern.MatchString(payload.ExecutionAttemptID) &&
		workerIdentityPattern.MatchString(payload.LeaseID) &&
		workerIdentityPattern.MatchString(payload.CapabilityRevisionID)
}

func validClaimNext(payload workerClaimNextPayload) bool {
	return workerIdentityPattern.MatchString(payload.ExecutionAttemptID) &&
		workerIdentityPattern.MatchString(payload.LeaseID) &&
		workerIdentityPattern.MatchString(payload.CapabilityRevisionID)
}

func validLeaseIdentity(targetID, attemptID, leaseID string, fenceToken int64) bool {
	return workerIdentityPattern.MatchString(targetID) && workerIdentityPattern.MatchString(attemptID) &&
		workerIdentityPattern.MatchString(leaseID) && fenceToken > 0
}

func validTerminalStatus(status coreworker.TerminalStatus) bool {
	return status == coreworker.TerminalCompleted || status == coreworker.TerminalFailed || status == coreworker.TerminalCanceled
}

func validOutputReference(output WorkerOutputReference) bool {
	return workerIdentityPattern.MatchString(output.ArtifactID) && lowerSHA256Digest(output.Digest)
}

func validTerminalReceipt(receipt json.RawMessage) bool {
	if len(receipt) == 0 || len(receipt) > maximumEvidenceSize {
		return false
	}
	var object map[string]any
	return json.Unmarshal(receipt, &object) == nil && object != nil
}

func validTerminalOutputMessage(status coreworker.TerminalStatus, output WorkerOutputReference, message *string) bool {
	if status != coreworker.TerminalCompleted {
		return message == nil
	}
	if message == nil {
		// The repository verifies that artifact-only completion is used only
		// above the inline plaintext limit, then writes a canonical reference.
		return true
	}
	if len([]byte(*message)) > MaximumArtifactChunkPlaintextBytes {
		return false
	}
	return digestMatches([]byte(*message), output.Digest)
}

func validArtifactIdentity(payload workerArtifactIdentityPayload) bool {
	return validLeaseIdentity(payload.TargetID, payload.ExecutionAttemptID, payload.LeaseID, payload.FenceToken) &&
		workerIdentityPattern.MatchString(payload.ArtifactID)
}

func validArtifactCreate(payload workerArtifactCreatePayload) bool {
	return validArtifactIdentity(payload.workerArtifactIdentityPayload) &&
		payload.ExpectedChunkCount >= 1 && payload.ExpectedChunkCount <= MaximumArtifactChunks &&
		payload.ExpectedPlaintextLength >= 0 && payload.ExpectedPlaintextLength <= MaximumArtifactPlaintextBytes &&
		payload.ExpectedPlaintextLength <= int64(payload.ExpectedChunkCount*MaximumArtifactChunkPlaintextBytes) &&
		lowerSHA256Digest(payload.LogicalDigest)
}

func validArtifactChunk(payload workerArtifactChunkPayload) bool {
	return validArtifactIdentity(payload.workerArtifactIdentityPayload) &&
		payload.ChunkIndex >= 0 && payload.ChunkIndex < MaximumArtifactChunks &&
		len(payload.Plaintext) > 0 && len(payload.Plaintext) <= MaximumArtifactChunkPlaintextBytes &&
		payload.PlaintextLength == len(payload.Plaintext) && lowerSHA256Digest(payload.PlaintextDigest) &&
		digestMatches(payload.Plaintext, payload.PlaintextDigest)
}

func digestMatches(payload []byte, expected string) bool {
	digest := sha256.Sum256(payload)
	return subtle.ConstantTimeCompare([]byte(hex.EncodeToString(digest[:])), []byte(expected)) == 1
}

func oneCanonicalUUIDHeader(request *http.Request, name string) (string, bool) {
	values := request.Header.Values(name)
	if len(values) != 1 || values[0] != strings.TrimSpace(values[0]) {
		return "", false
	}
	parsed, err := uuid.Parse(values[0])
	return values[0], err == nil && parsed.String() == values[0]
}

func oneIdentityHeader(request *http.Request, name string) (string, bool) {
	values := request.Header.Values(name)
	returnValue := ""
	if len(values) == 1 {
		returnValue = values[0]
	}
	return returnValue, len(values) == 1 && workerIdentityPattern.MatchString(returnValue)
}

func lowerSHA256Digest(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && hex.EncodeToString(decoded) == value
}

func writeWorkerCommandError(response http.ResponseWriter, err error) {
	status, code := http.StatusServiceUnavailable, "worker_command_failed"
	switch {
	case errors.Is(err, ErrWorkerRequestInvalid):
		status, code = http.StatusBadRequest, "worker_request_invalid"
	case errors.Is(err, ErrWorkerNotFound), errors.Is(err, ErrWorkerRevoked):
		status, code = http.StatusUnauthorized, "worker_unauthorized"
	case errors.Is(err, ErrWorkerNoCompatibleTarget):
		status, code = http.StatusConflict, "worker_target_unavailable"
	case errors.Is(err, ErrWorkerStaleLease):
		status, code = http.StatusConflict, "worker_stale_lease"
	case errors.Is(err, ErrWorkerIdempotencyConflict):
		status, code = http.StatusConflict, "idempotency_conflict"
	case errors.Is(err, ErrWorkerArtifactIncomplete):
		status, code = http.StatusConflict, "artifact_incomplete"
	case errors.Is(err, ErrWorkerContextPageLimit):
		status, code = http.StatusRequestEntityTooLarge, "payload_limit"
	}
	writeJSON(response, status, map[string]string{"code": code})
}

func validEnvelope(envelope EncryptedEnvelope) bool {
	return len(envelope.Ciphertext) > 0 && len(envelope.Ciphertext) <= MaximumFunctionBodyBytes &&
		workerIdentityPattern.MatchString(envelope.KeyID) && len(envelope.Nonce) >= 12 && len(envelope.Nonce) <= 64 &&
		lowerSHA256Digest(envelope.Digest) && envelope.PlaintextLength >= 0 && envelope.PlaintextLength <= 2<<20
}

func digestJSON(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(bytes.TrimSpace(encoded))
	return hex.EncodeToString(digest[:]), nil
}
