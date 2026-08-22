package cloudworker

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tobsai/fort/cloud/controlapi"
)

// ExecutionContext is the complete immutable, typed context selected by Fort.
// A separately approved adapter owns any provider-specific encoding; Worker
// never concatenates these records into the assignment prompt.
type ExecutionContext struct {
	ManifestID     string
	ManifestDigest string
	Items          []controlapi.WorkerContextItem
}

func (worker *Worker) readExecutionContext(ctx context.Context, assignment controlapi.WorkerAssignment) (ExecutionContext, error) {
	result := ExecutionContext{ManifestID: assignment.ContextManifestID}
	cursor := ""
	seenCursors := map[string]bool{"": true}
	lastOrdinal, lastRank := -1, -1
	totalEncoded := 0
	for {
		page, err := worker.Control.ReadWorkerContextPage(ctx, controlapi.WorkerContextPageCommand{
			AccountID: worker.Identity.AccountID, WorkerID: worker.Identity.WorkerID, MachineID: worker.Identity.MachineID,
			TargetID: assignment.TargetID, ExecutionAttemptID: assignment.ExecutionAttemptID,
			LeaseID: assignment.LeaseID, FenceToken: assignment.FenceToken,
			IdempotencyKey: worker.IDs.New("context-page"), Cursor: cursor, ObservedAt: worker.Clock().UTC(),
		})
		if err != nil {
			return ExecutionContext{}, fmt.Errorf("cloud worker read context page: %w", err)
		}
		encoded, err := json.Marshal(page)
		if err != nil || len(encoded)+1 > controlapi.MaximumWorkerContextPageEncodedBytes {
			return ExecutionContext{}, fmt.Errorf("%w: context page exceeds encoded limit", ErrWorkerInvalid)
		}
		totalEncoded += len(encoded) + 1
		if totalEncoded > controlapi.MaximumArtifactPlaintextBytes {
			return ExecutionContext{}, fmt.Errorf("%w: complete execution context exceeds logical limit", ErrWorkerInvalid)
		}
		if page.ContextManifestID != assignment.ContextManifestID || !lowerDigest(page.ManifestDigest) ||
			(result.ManifestDigest != "" && page.ManifestDigest != result.ManifestDigest) ||
			len(page.Items) > controlapi.MaximumWorkerContextPageItems {
			return ExecutionContext{}, fmt.Errorf("%w: context page changed immutable manifest", ErrWorkerInvalid)
		}
		if result.ManifestDigest == "" {
			result.ManifestDigest = page.ManifestDigest
		}
		for _, item := range page.Items {
			rank, ok := validContextItem(item)
			if !ok || item.Ordinal < lastOrdinal || (item.Ordinal == lastOrdinal && rank <= lastRank) {
				return ExecutionContext{}, fmt.Errorf("%w: context items are not strictly ordered", ErrWorkerInvalid)
			}
			lastOrdinal, lastRank = item.Ordinal, rank
			result.Items = append(result.Items, item)
		}
		if page.NextCursor == "" {
			return result, nil
		}
		if len(page.Items) == 0 || !validContextCursor(page.NextCursor) || seenCursors[page.NextCursor] {
			return ExecutionContext{}, fmt.Errorf("%w: context cursor did not advance", ErrWorkerInvalid)
		}
		seenCursors[page.NextCursor] = true
		cursor = page.NextCursor
	}
}

func validContextItem(item controlapi.WorkerContextItem) (int, bool) {
	if item.Ordinal < 0 {
		return 0, false
	}
	switch item.Kind {
	case controlapi.WorkerContextMessageKind:
		message := item.Message
		return 0, message != nil && item.Artifact == nil && message.MessageID > 0 &&
			!blank(message.ConversationID, message.MessageKind, message.AuthorKind, message.AuthorID) && !message.CreatedAt.IsZero()
	case controlapi.WorkerContextArtifactKind:
		artifact := item.Artifact
		return 1, artifact != nil && item.Message == nil &&
			!blank(artifact.ArtifactID, artifact.Kind, artifact.ExecutionAttemptID) &&
			artifact.ExpectedChunkCount >= 1 && artifact.ExpectedChunkCount <= controlapi.MaximumArtifactChunks &&
			artifact.ExpectedPlaintextLength >= 0 && artifact.ExpectedPlaintextLength <= controlapi.MaximumArtifactPlaintextBytes &&
			artifact.ExpectedEncodedLength > 0 && artifact.ExpectedEncodedLength <= controlapi.MaximumArtifactEncodedBytes &&
			lowerDigest(artifact.LogicalDigest) && !artifact.CreatedAt.IsZero() && !artifact.FinalizedAt.IsZero() &&
			!artifact.FinalizedAt.Before(artifact.CreatedAt)
	default:
		return 0, false
	}
}

func validContextCursor(cursor string) bool {
	if cursor == "" || len(cursor) > controlapi.MaximumWorkerContextCursorBytes {
		return false
	}
	for _, character := range cursor {
		if !(character >= 'A' && character <= 'Z') && !(character >= 'a' && character <= 'z') &&
			!(character >= '0' && character <= '9') && character != '_' && character != '-' {
			return false
		}
	}
	return true
}

func lowerDigest(value string) bool {
	if len(value) != 64 || strings.ToLower(value) != value {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
