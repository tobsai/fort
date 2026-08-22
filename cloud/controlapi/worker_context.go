package controlapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"time"
)

const (
	// MaximumWorkerContextPageEncodedBytes is the normative JSON response
	// boundary from Spec 047. It includes the trailing newline written by the
	// worker handler.
	MaximumWorkerContextPageEncodedBytes = 1 << 20
	MaximumWorkerContextPageItems        = 256
	MaximumWorkerContextCursorBytes      = 2048

	WorkerContextMessageKind  = "message"
	WorkerContextArtifactKind = "artifact"
)

var (
	ErrWorkerContextPageLimit  = errors.New("worker context page exceeds payload limit")
	workerContextCursorPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
)

// WorkerContextPageCommand identifies one exact, actively fenced assignment.
// ContextManifestID is intentionally absent: the repository resolves the
// immutable manifest pinned by the target's Turn rather than trusting worker
// input.
type WorkerContextPageCommand struct {
	AccountID          string
	WorkerID           string
	MachineID          string
	TargetID           string
	ExecutionAttemptID string
	LeaseID            string
	FenceToken         int64
	IdempotencyKey     string
	Cursor             string
	ObservedAt         time.Time
}

// WorkerContextPage contains only decrypted message content and immutable
// artifact references. Application AEAD keys, ciphertext, nonces, and raw
// artifact chunks are deliberately not represented by this transport type.
type WorkerContextPage struct {
	ContextManifestID string              `json:"context_manifest_id"`
	ManifestDigest    string              `json:"manifest_digest"`
	Items             []WorkerContextItem `json:"items"`
	NextCursor        string              `json:"next_cursor"`
}

type WorkerContextItem struct {
	Kind     string                          `json:"kind"`
	Ordinal  int                             `json:"ordinal"`
	Message  *WorkerContextMessage           `json:"message,omitempty"`
	Artifact *WorkerContextArtifactReference `json:"artifact,omitempty"`
}

type WorkerContextMessage struct {
	MessageID      int64     `json:"message_id"`
	ConversationID string    `json:"conversation_id"`
	TurnID         string    `json:"turn_id,omitempty"`
	TargetID       string    `json:"target_id,omitempty"`
	HandoffID      string    `json:"handoff_id,omitempty"`
	RoutineRunID   string    `json:"routine_run_id,omitempty"`
	MessageKind    string    `json:"message_kind"`
	AuthorKind     string    `json:"author_kind"`
	AuthorID       string    `json:"author_id"`
	AuthorAgentID  string    `json:"author_agent_id,omitempty"`
	Body           string    `json:"body"`
	CreatedAt      time.Time `json:"created_at"`
}

type WorkerContextArtifactReference struct {
	ArtifactID              string    `json:"artifact_id"`
	Kind                    string    `json:"kind"`
	ExecutionAttemptID      string    `json:"execution_attempt_id"`
	ExpectedChunkCount      int       `json:"expected_chunk_count"`
	ExpectedPlaintextLength int64     `json:"expected_plaintext_length"`
	ExpectedEncodedLength   int64     `json:"expected_encoded_length"`
	LogicalDigest           string    `json:"logical_digest"`
	CreatedAt               time.Time `json:"created_at"`
	FinalizedAt             time.Time `json:"finalized_at"`
}

// WorkerContextRepository is kept separate from WorkerRepository so rollout
// remains additive: older endpoint fakes and adapters continue to satisfy the
// base machine-command contract, while context_page fails closed unless the
// backing repository explicitly implements this capability.
type WorkerContextRepository interface {
	ReadWorkerContextPage(context.Context, WorkerContextPageCommand) (WorkerContextPage, error)
}

type workerContextPagePayload struct {
	TargetID           string `json:"target_id"`
	ExecutionAttemptID string `json:"execution_attempt_id"`
	LeaseID            string `json:"lease_id"`
	FenceToken         int64  `json:"fence_token"`
	Cursor             string `json:"cursor"`
}

func validWorkerContextPagePayload(payload workerContextPagePayload) bool {
	return validLeaseIdentity(payload.TargetID, payload.ExecutionAttemptID, payload.LeaseID, payload.FenceToken) &&
		validWorkerContextCursorInput(payload.Cursor)
}

func validWorkerContextCursorInput(cursor string) bool {
	return cursor == "" || (len(cursor) <= MaximumWorkerContextCursorBytes && workerContextCursorPattern.MatchString(cursor))
}

func executeWorkerContextPage(ctx context.Context, repository WorkerRepository, principal MachineCredential,
	body workerRequestBody, now time.Time) (any, int, error) {
	if body.ContextPage == nil || !validWorkerContextPagePayload(*body.ContextPage) {
		return nil, 0, fmt.Errorf("%w: context page", ErrWorkerRequestInvalid)
	}
	contextRepository, ok := repository.(WorkerContextRepository)
	if !ok {
		return nil, 0, fmt.Errorf("worker context repository is unavailable")
	}
	payload := body.ContextPage
	result, err := contextRepository.ReadWorkerContextPage(ctx, WorkerContextPageCommand{
		AccountID: principal.AccountID, WorkerID: principal.WorkerID, MachineID: principal.MachineID,
		TargetID: payload.TargetID, ExecutionAttemptID: payload.ExecutionAttemptID,
		LeaseID: payload.LeaseID, FenceToken: payload.FenceToken,
		IdempotencyKey: body.IdempotencyKey, Cursor: payload.Cursor, ObservedAt: now,
	})
	return result, http.StatusOK, err
}
