package postgres

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/tobsai/fort/cloud/controlapi"
	"github.com/tobsai/fort/cloud/securebody"
)

const workerContextCursorVersion = 1

var _ controlapi.WorkerContextRepository = (*Store)(nil)

type workerContextPosition struct {
	ordinal int
	rank    int
}

type workerContextCursor struct {
	Version            int    `json:"v"`
	AccountID          string `json:"c"`
	TargetID           string `json:"t"`
	ExecutionAttemptID string `json:"a"`
	LeaseID            string `json:"l"`
	FenceToken         int64  `json:"f"`
	ContextManifestID  string `json:"m"`
	ManifestDigest     string `json:"d"`
	Ordinal            int    `json:"o"`
	Rank               int    `json:"r"`
}

type workerContextMessageMetadata struct {
	MessageID      int64  `json:"message_id"`
	ConversationID string `json:"conversation_id"`
	TurnID         string `json:"turn_id"`
	TargetID       string `json:"target_id"`
	HandoffID      string `json:"handoff_id"`
	RoutineRunID   string `json:"routine_run_id"`
	MessageKind    string `json:"message_kind"`
	TurnKind       string `json:"turn_kind"`
	AuthorKind     string `json:"author_kind"`
	AuthorID       string `json:"author_id"`
	AuthorAgentID  string `json:"author_agent_id"`
}

type workerContextArtifactMetadata struct {
	ArtifactID              string `json:"artifact_id"`
	Kind                    string `json:"kind"`
	ExecutionAttemptID      string `json:"execution_attempt_id"`
	ExpectedChunkCount      int    `json:"expected_chunk_count"`
	ExpectedPlaintextLength int64  `json:"expected_plaintext_length"`
	ExpectedEncodedLength   int64  `json:"expected_encoded_length"`
	LogicalDigest           string `json:"logical_digest"`
}

// ReadWorkerContextPage returns one immutable page from the manifest pinned by
// the assignment's Conversation Turn. The worker cannot select a manifest,
// and every page request revalidates the exact active lease and fence before
// any encrypted message is read.
func (store *Store) ReadWorkerContextPage(ctx context.Context, command controlapi.WorkerContextPageCommand) (controlapi.WorkerContextPage, error) {
	accountID, err := store.operationAccount(command.AccountID)
	if err != nil {
		return controlapi.WorkerContextPage{}, err
	}
	if err := validateWorkerContextCommand(command); err != nil {
		return controlapi.WorkerContextPage{}, err
	}
	cipher, err := store.collaborationBodies()
	if err != nil {
		return controlapi.WorkerContextPage{}, err
	}
	var page controlapi.WorkerContextPage
	err = store.withTransaction(ctx, accountID, func(tx transaction) error {
		manifestID, manifestDigest, err := requireActiveWorkerContextLease(ctx, tx, accountID, command)
		if err != nil {
			return err
		}
		position, err := decodeWorkerContextCursor(command, manifestID, manifestDigest)
		if err != nil {
			return err
		}
		page, err = loadWorkerContextPage(ctx, tx, cipher, accountID, command, manifestID, manifestDigest, position)
		return err
	})
	return page, err
}

func requireActiveWorkerContextLease(ctx context.Context, tx transaction, accountID string,
	command controlapi.WorkerContextPageCommand) (string, string, error) {
	var manifestID, manifestDigest, attemptState, targetState string
	var expiresAt time.Time
	err := tx.queryRow(ctx, `select turn.context_manifest_id, manifest.manifest_digest,
  lease.expires_at, attempt.state, target.state
from fort_private.worker_lease as lease
join fort_private.execution_attempt as attempt
  on attempt.account_id = lease.account_id
 and attempt.execution_attempt_id = lease.execution_attempt_id
 and attempt.target_id = lease.target_id
 and attempt.worker_id = lease.worker_id
join fort_private.conversation_target as target
  on target.account_id = attempt.account_id and target.target_id = attempt.target_id
join fort_private.conversation_turn as turn
  on turn.account_id = target.account_id and turn.turn_id = target.turn_id
join fort_private.context_manifest as manifest
  on manifest.account_id = turn.account_id
 and manifest.context_manifest_id = turn.context_manifest_id
join fort_private.worker as machine
  on machine.account_id = lease.account_id and machine.worker_id = lease.worker_id
where lease.account_id = $1 and lease.lease_id = $2 and lease.execution_attempt_id = $3
  and lease.target_id = $4 and lease.worker_id = $5 and lease.fence_token = $6
  and machine.machine_id = $7 and machine.state <> 'revoked' and lease.state = 'active'
for update of lease`, accountID, command.LeaseID, command.ExecutionAttemptID, command.TargetID,
		command.WorkerID, command.FenceToken, command.MachineID).scan(
		&manifestID, &manifestDigest, &expiresAt, &attemptState, &targetState)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", controlapi.ErrWorkerStaleLease
	}
	if err != nil {
		return "", "", err
	}
	if !command.ObservedAt.Before(expiresAt) ||
		(attemptState != "leased" && attemptState != "working") ||
		(targetState != "claimed" && targetState != "working") ||
		!validWorkerIdentifier(manifestID) || !lowerWorkerDigest(manifestDigest) {
		return "", "", controlapi.ErrWorkerStaleLease
	}
	return manifestID, manifestDigest, nil
}

func loadWorkerContextPage(ctx context.Context, tx transaction, cipher collaborationBodyCipher, accountID string,
	command controlapi.WorkerContextPageCommand, manifestID, manifestDigest string,
	position workerContextPosition) (controlapi.WorkerContextPage, error) {
	result, err := tx.query(ctx, `select ordinal, item_rank, item_kind, metadata, item_created_at,
  item_finalized_at, ciphertext, envelope_version, key_id, nonce, digest, plaintext_length
from (
  select link.ordinal, 0 as item_rank, 'message'::text as item_kind,
    jsonb_build_object(
      'message_id', message.message_id,
      'conversation_id', message.conversation_id,
      'turn_id', coalesce(message.turn_id, ''),
      'target_id', coalesce(message.target_id, ''),
      'handoff_id', coalesce(message.handoff_id, ''),
      'routine_run_id', coalesce(message.routine_run_id, ''),
      'message_kind', message.message_kind,
      'turn_kind', coalesce(turn.kind, ''),
      'author_kind', message.author_kind,
      'author_id', message.author_id,
      'author_agent_id', coalesce(message.author_agent_id, '')
    )::text as metadata,
    message.created_at as item_created_at,
    'epoch'::timestamptz as item_finalized_at,
    message.body_ciphertext as ciphertext,
    message.body_envelope_version::bigint as envelope_version,
    message.body_key_id as key_id,
    message.body_nonce as nonce,
    message.body_digest as digest,
    message.body_plaintext_length::bigint as plaintext_length
  from fort_private.context_manifest_message as link
  join fort_private.conversation_message as message
    on message.account_id = link.account_id and message.message_id = link.message_id
  left join fort_private.conversation_turn as turn
    on turn.account_id = message.account_id and turn.turn_id = message.turn_id
  where link.account_id = $1 and link.context_manifest_id = $2

  union all

  select link.ordinal, 1 as item_rank, 'artifact'::text as item_kind,
    jsonb_build_object(
      'artifact_id', artifact.artifact_id,
      'kind', artifact.kind,
      'execution_attempt_id', artifact.execution_attempt_id,
      'expected_chunk_count', artifact.expected_chunk_count,
      'expected_plaintext_length', artifact.expected_plaintext_length,
      'expected_encoded_length', artifact.expected_encoded_length,
      'logical_digest', artifact.logical_digest
    )::text as metadata,
    artifact.created_at as item_created_at,
    artifact.finalized_at as item_finalized_at,
    ''::bytea as ciphertext,
    0::bigint as envelope_version,
    ''::text as key_id,
    ''::bytea as nonce,
    ''::text as digest,
    0::bigint as plaintext_length
  from fort_private.context_manifest_artifact as link
  join fort_private.artifact as artifact
    on artifact.account_id = link.account_id and artifact.artifact_id = link.artifact_id
  where link.account_id = $1 and link.context_manifest_id = $2
    and artifact.state = 'finalized' and artifact.finalized_at is not null
) as context_item
where ordinal > $3 or (ordinal = $3 and item_rank > $4)
order by ordinal, item_rank
limit $5`, accountID, manifestID, position.ordinal, position.rank, controlapi.MaximumWorkerContextPageItems+1)
	if err != nil {
		return controlapi.WorkerContextPage{}, err
	}
	defer result.close()

	page := controlapi.WorkerContextPage{
		ContextManifestID: manifestID,
		ManifestDigest:    manifestDigest,
		Items:             make([]controlapi.WorkerContextItem, 0),
	}
	last := position
	for result.next() {
		if len(page.Items) == controlapi.MaximumWorkerContextPageItems {
			page.NextCursor = encodeWorkerContextCursor(command, manifestID, manifestDigest, last)
			return page, nil
		}
		item, itemPosition, err := scanWorkerContextItem(result, cipher, accountID)
		if err != nil {
			return controlapi.WorkerContextPage{}, err
		}
		candidate := page
		candidate.Items = append(append([]controlapi.WorkerContextItem{}, page.Items...), item)
		candidate.NextCursor = encodeWorkerContextCursor(command, manifestID, manifestDigest, itemPosition)
		if workerContextPageEncodedSize(candidate) > controlapi.MaximumWorkerContextPageEncodedBytes {
			if len(page.Items) == 0 {
				candidate.NextCursor = ""
				if workerContextPageEncodedSize(candidate) <= controlapi.MaximumWorkerContextPageEncodedBytes {
					if result.next() {
						return controlapi.WorkerContextPage{}, fmt.Errorf("%w: item requires a cursor but leaves no cursor space", controlapi.ErrWorkerContextPageLimit)
					}
					if err := result.errResult(); err != nil {
						return controlapi.WorkerContextPage{}, err
					}
					return candidate, nil
				}
				return controlapi.WorkerContextPage{}, fmt.Errorf("%w: one manifest item cannot fit", controlapi.ErrWorkerContextPageLimit)
			}
			page.NextCursor = encodeWorkerContextCursor(command, manifestID, manifestDigest, last)
			return page, nil
		}
		page = candidate
		last = itemPosition
	}
	if err := result.errResult(); err != nil {
		return controlapi.WorkerContextPage{}, err
	}
	page.NextCursor = ""
	if workerContextPageEncodedSize(page) > controlapi.MaximumWorkerContextPageEncodedBytes {
		return controlapi.WorkerContextPage{}, controlapi.ErrWorkerContextPageLimit
	}
	return page, nil
}

func scanWorkerContextItem(result rows, cipher collaborationBodyCipher, accountID string) (controlapi.WorkerContextItem, workerContextPosition, error) {
	var ordinal, rank int
	var kind, metadataJSON, keyID, digest string
	var createdAt, finalizedAt time.Time
	var ciphertext, nonce []byte
	var envelopeVersion, plaintextLength int64
	if err := result.scan(&ordinal, &rank, &kind, &metadataJSON, &createdAt, &finalizedAt,
		&ciphertext, &envelopeVersion, &keyID, &nonce, &digest, &plaintextLength); err != nil {
		return controlapi.WorkerContextItem{}, workerContextPosition{}, err
	}
	position := workerContextPosition{ordinal: ordinal, rank: rank}
	if ordinal < 0 || ordinal >= 256 {
		return controlapi.WorkerContextItem{}, position, fmt.Errorf("persisted context ordinal %d is invalid", ordinal)
	}
	switch kind {
	case controlapi.WorkerContextMessageKind:
		if rank != 0 {
			return controlapi.WorkerContextItem{}, position, fmt.Errorf("persisted context message rank is invalid")
		}
		var metadata workerContextMessageMetadata
		if err := json.Unmarshal([]byte(metadataJSON), &metadata); err != nil {
			return controlapi.WorkerContextItem{}, position, fmt.Errorf("decode context message metadata: %w", err)
		}
		recordType, recordID, err := workerContextMessageScope(metadata)
		if err != nil {
			return controlapi.WorkerContextItem{}, position, err
		}
		body, err := cipher.open(securebody.Scope{AccountID: accountID, RecordType: recordType, RecordID: recordID}, collaborationEncryptedBody{
			Version: int(envelopeVersion), Ciphertext: ciphertext, KeyID: keyID, Nonce: nonce,
			Digest: digest, PlaintextBytes: int(plaintextLength),
		})
		if err != nil {
			return controlapi.WorkerContextItem{}, position, fmt.Errorf("decrypt context message %d: %w", metadata.MessageID, err)
		}
		return controlapi.WorkerContextItem{
			Kind: kind, Ordinal: ordinal,
			Message: &controlapi.WorkerContextMessage{
				MessageID: metadata.MessageID, ConversationID: metadata.ConversationID,
				TurnID: metadata.TurnID, TargetID: metadata.TargetID, HandoffID: metadata.HandoffID,
				RoutineRunID: metadata.RoutineRunID, MessageKind: metadata.MessageKind,
				AuthorKind: metadata.AuthorKind, AuthorID: metadata.AuthorID,
				AuthorAgentID: metadata.AuthorAgentID, Body: body, CreatedAt: createdAt.UTC(),
			},
		}, position, nil
	case controlapi.WorkerContextArtifactKind:
		if rank != 1 || finalizedAt.IsZero() {
			return controlapi.WorkerContextItem{}, position, fmt.Errorf("persisted context artifact is not finalized")
		}
		var metadata workerContextArtifactMetadata
		if err := json.Unmarshal([]byte(metadataJSON), &metadata); err != nil {
			return controlapi.WorkerContextItem{}, position, fmt.Errorf("decode context artifact metadata: %w", err)
		}
		if !validWorkerIdentifier(metadata.ArtifactID) || !validWorkerIdentifier(metadata.ExecutionAttemptID) ||
			(metadata.Kind != "context" && metadata.Kind != "output") || !lowerWorkerDigest(metadata.LogicalDigest) ||
			metadata.ExpectedChunkCount < 1 || metadata.ExpectedChunkCount > controlapi.MaximumArtifactChunks ||
			metadata.ExpectedPlaintextLength < 0 || metadata.ExpectedPlaintextLength > controlapi.MaximumArtifactPlaintextBytes ||
			metadata.ExpectedEncodedLength < 0 || metadata.ExpectedEncodedLength > controlapi.MaximumArtifactEncodedBytes {
			return controlapi.WorkerContextItem{}, position, fmt.Errorf("persisted context artifact metadata is invalid")
		}
		return controlapi.WorkerContextItem{
			Kind: kind, Ordinal: ordinal,
			Artifact: &controlapi.WorkerContextArtifactReference{
				ArtifactID: metadata.ArtifactID, Kind: metadata.Kind,
				ExecutionAttemptID:      metadata.ExecutionAttemptID,
				ExpectedChunkCount:      metadata.ExpectedChunkCount,
				ExpectedPlaintextLength: metadata.ExpectedPlaintextLength,
				ExpectedEncodedLength:   metadata.ExpectedEncodedLength,
				LogicalDigest:           metadata.LogicalDigest,
				CreatedAt:               createdAt.UTC(), FinalizedAt: finalizedAt.UTC(),
			},
		}, position, nil
	default:
		return controlapi.WorkerContextItem{}, position, fmt.Errorf("persisted context item kind %q is invalid", kind)
	}
}

func workerContextMessageScope(metadata workerContextMessageMetadata) (string, string, error) {
	switch metadata.MessageKind {
	case "human":
		if metadata.TurnID != "" {
			if metadata.TurnKind == "human_group" {
				return "group_turn_prompt", metadata.TurnID, nil
			}
			return "conversation_message", metadata.TurnID, nil
		}
	case "system":
		if metadata.TurnID != "" {
			return "conversation_message", metadata.TurnID, nil
		}
	case "agent":
		if metadata.TargetID != "" {
			return "conversation_message", metadata.TargetID, nil
		}
	case "handoff_result":
		if metadata.HandoffID != "" {
			return "handoff_result", metadata.HandoffID, nil
		}
	case "routine_result":
		if metadata.RoutineRunID != "" {
			return "routine_result", metadata.RoutineRunID, nil
		}
	}
	return "", "", fmt.Errorf("context message %d has unsupported or incomplete encryption scope", metadata.MessageID)
}

func validateWorkerContextCommand(command controlapi.WorkerContextPageCommand) error {
	if !validWorkerIdentifier(command.WorkerID) || !validWorkerIdentifier(command.MachineID) ||
		!validWorkerIdentifier(command.TargetID) || !validWorkerIdentifier(command.ExecutionAttemptID) ||
		!validWorkerIdentifier(command.LeaseID) || command.FenceToken < 1 ||
		!validWorkerIdentifier(command.IdempotencyKey) || command.ObservedAt.IsZero() ||
		len(command.Cursor) > controlapi.MaximumWorkerContextCursorBytes {
		return fmt.Errorf("%w: context page command", controlapi.ErrWorkerRequestInvalid)
	}
	return nil
}

func decodeWorkerContextCursor(command controlapi.WorkerContextPageCommand, manifestID,
	manifestDigest string) (workerContextPosition, error) {
	if command.Cursor == "" {
		return workerContextPosition{ordinal: -1, rank: -1}, nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(command.Cursor)
	if err != nil || base64.RawURLEncoding.EncodeToString(decoded) != command.Cursor {
		return workerContextPosition{}, fmt.Errorf("%w: context cursor encoding", controlapi.ErrWorkerRequestInvalid)
	}
	decoder := json.NewDecoder(bytes.NewReader(decoded))
	decoder.DisallowUnknownFields()
	var cursor workerContextCursor
	if err := decoder.Decode(&cursor); err != nil {
		return workerContextPosition{}, fmt.Errorf("%w: context cursor", controlapi.ErrWorkerRequestInvalid)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return workerContextPosition{}, fmt.Errorf("%w: context cursor trailing data", controlapi.ErrWorkerRequestInvalid)
	}
	canonical, err := json.Marshal(cursor)
	if err != nil || !bytes.Equal(canonical, decoded) || cursor.Version != workerContextCursorVersion ||
		cursor.AccountID != command.AccountID ||
		cursor.TargetID != command.TargetID || cursor.ExecutionAttemptID != command.ExecutionAttemptID ||
		cursor.LeaseID != command.LeaseID || cursor.FenceToken != command.FenceToken ||
		cursor.ContextManifestID != manifestID || cursor.ManifestDigest != manifestDigest ||
		cursor.Ordinal < 0 || cursor.Ordinal >= 256 || cursor.Rank < 0 || cursor.Rank > 1 {
		return workerContextPosition{}, fmt.Errorf("%w: context cursor binding", controlapi.ErrWorkerRequestInvalid)
	}
	return workerContextPosition{ordinal: cursor.Ordinal, rank: cursor.Rank}, nil
}

func encodeWorkerContextCursor(command controlapi.WorkerContextPageCommand, manifestID,
	manifestDigest string, position workerContextPosition) string {
	encoded, _ := json.Marshal(workerContextCursor{
		Version: workerContextCursorVersion, AccountID: command.AccountID, TargetID: command.TargetID,
		ExecutionAttemptID: command.ExecutionAttemptID, LeaseID: command.LeaseID,
		FenceToken: command.FenceToken, ContextManifestID: manifestID,
		ManifestDigest: manifestDigest, Ordinal: position.ordinal, Rank: position.rank,
	})
	return base64.RawURLEncoding.EncodeToString(encoded)
}

func workerContextPageEncodedSize(page controlapi.WorkerContextPage) int {
	encoded, err := json.Marshal(page)
	if err != nil {
		return controlapi.MaximumWorkerContextPageEncodedBytes + 1
	}
	return len(encoded) + 1
}
