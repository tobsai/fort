package postgres

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/tobsai/fort/cloud/controlapi"
	"github.com/tobsai/fort/cloud/securebody"
)

const (
	workerArtifactCreateScope   = "worker.artifact.create"
	workerArtifactChunkScope    = "worker.artifact.chunk"
	workerArtifactFinalizeScope = "worker.artifact.finalize"
)

type workerArtifactLeaseIdentity struct {
	accountID, workerID, machineID, targetID, attemptID, leaseID string
	fenceToken                                                   int64
	observedAt                                                   time.Time
}

func (store *Store) CreateWorkerArtifact(ctx context.Context, command controlapi.WorkerArtifactCreateCommand) (controlapi.WorkerArtifact, error) {
	accountID, err := store.operationAccount(command.AccountID)
	if err != nil {
		return controlapi.WorkerArtifact{}, err
	}
	if err := validateWorkerArtifactCreate(command); err != nil {
		return controlapi.WorkerArtifact{}, err
	}
	cipher, err := store.collaborationBodies()
	if err != nil {
		return controlapi.WorkerArtifact{}, err
	}
	digest, err := workerArtifactCreateDigest(command)
	if err != nil {
		return controlapi.WorkerArtifact{}, err
	}
	command.ExpectedEncodedLength = command.ExpectedPlaintextLength + int64(command.ExpectedChunkCount*securebody.AEADOverheadBytes)
	command.EncryptionKeyID = cipher.activeKeyID()
	if !validWorkerIdentifier(command.EncryptionKeyID) || command.ExpectedEncodedLength > controlapi.MaximumArtifactEncodedBytes {
		return controlapi.WorkerArtifact{}, fmt.Errorf("worker artifact encryption configuration is invalid")
	}
	result := workerArtifactFromCreate(command)
	err = store.withTransaction(ctx, accountID, func(tx transaction) error {
		if err := requireActiveWorkerArtifactLease(ctx, tx, artifactLeaseForCreate(command)); err != nil {
			return err
		}
		claimed, err := claimWorkerIdempotency(ctx, tx, accountID, workerArtifactCreateScope, command.IdempotencyKey,
			digest, "artifact", command.ArtifactID, command.CreatedAt)
		if err != nil {
			return err
		}
		if !claimed {
			existing, err := loadWorkerArtifactManifest(ctx, tx, accountID, command.ArtifactID, true)
			if err != nil {
				return err
			}
			existing.Created = false
			result = existing
			return nil
		}
		affected, err := tx.exec(ctx, `insert into fort_private.artifact (
  account_id, artifact_id, execution_attempt_id, kind, state,
  expected_chunk_count, expected_plaintext_length, expected_encoded_length,
  logical_digest, encryption_key_id, created_at
) values ($1, $2, $3, 'output', 'uploading', $4, $5, $6, $7, $8, $9)
on conflict (account_id, artifact_id) do nothing`, accountID, command.ArtifactID,
			command.ExecutionAttemptID, command.ExpectedChunkCount, command.ExpectedPlaintextLength,
			command.ExpectedEncodedLength, command.LogicalDigest, command.EncryptionKeyID, command.CreatedAt.UTC())
		if err != nil {
			return fmt.Errorf("insert worker output artifact: %w", err)
		}
		if affected == 1 {
			result.Created = true
			return nil
		}
		if affected != 0 {
			return fmt.Errorf("insert worker output artifact affected %d rows", affected)
		}
		existing, err := loadWorkerArtifactManifest(ctx, tx, accountID, command.ArtifactID, true)
		if err != nil {
			return err
		}
		if !workerArtifactManifestMatchesCreate(existing, command) {
			return controlapi.ErrWorkerIdempotencyConflict
		}
		existing.Created = false
		result = existing
		return nil
	})
	return result, err
}

func (store *Store) GetWorkerArtifactStatus(ctx context.Context, command controlapi.WorkerArtifactStatusCommand) (controlapi.WorkerArtifact, error) {
	accountID, err := store.operationAccount(command.AccountID)
	if err != nil {
		return controlapi.WorkerArtifact{}, err
	}
	if err := validateWorkerArtifactStatus(command); err != nil {
		return controlapi.WorkerArtifact{}, err
	}
	var result controlapi.WorkerArtifact
	err = store.withTransaction(ctx, accountID, func(tx transaction) error {
		if err := requireActiveWorkerArtifactLease(ctx, tx, artifactLeaseForStatus(command)); err != nil {
			return err
		}
		result, err = loadWorkerArtifactManifest(ctx, tx, accountID, command.ArtifactID, false)
		if err != nil {
			return err
		}
		if result.ExecutionAttemptID != command.ExecutionAttemptID || result.Kind != "output" {
			return controlapi.ErrWorkerStaleLease
		}
		result.Chunks, err = loadWorkerArtifactChunks(ctx, tx, accountID, command.ArtifactID)
		return err
	})
	return result, err
}

func (store *Store) AppendWorkerArtifactChunk(ctx context.Context, command controlapi.WorkerArtifactChunkCommand) (controlapi.WorkerArtifactChunk, error) {
	accountID, err := store.operationAccount(command.AccountID)
	if err != nil {
		return controlapi.WorkerArtifactChunk{}, err
	}
	if err := validateWorkerArtifactChunk(command); err != nil {
		return controlapi.WorkerArtifactChunk{}, err
	}
	digest, err := workerArtifactChunkDigest(command)
	if err != nil {
		return controlapi.WorkerArtifactChunk{}, err
	}
	cipher, err := store.collaborationBodies()
	if err != nil {
		return controlapi.WorkerArtifactChunk{}, err
	}
	var result controlapi.WorkerArtifactChunk
	err = store.withTransaction(ctx, accountID, func(tx transaction) error {
		if err := requireActiveWorkerArtifactLease(ctx, tx, artifactLeaseForChunk(command)); err != nil {
			return err
		}
		artifact, err := loadWorkerArtifactManifest(ctx, tx, accountID, command.ArtifactID, true)
		if err != nil {
			return err
		}
		if artifact.ExecutionAttemptID != command.ExecutionAttemptID || artifact.Kind != "output" ||
			command.ChunkIndex >= artifact.ExpectedChunkCount {
			return controlapi.ErrWorkerStaleLease
		}
		resultID := command.ArtifactID + "#" + strconv.Itoa(command.ChunkIndex)
		if _, err := claimWorkerIdempotency(ctx, tx, accountID, workerArtifactChunkScope, command.IdempotencyKey,
			digest, "artifact_chunk", resultID, command.CreatedAt); err != nil {
			return err
		}
		existing, err := loadWorkerArtifactChunkBody(ctx, tx, accountID, command.ArtifactID, command.ChunkIndex)
		if err == nil {
			if !workerArtifactChunkMatches(existing, command) {
				return controlapi.ErrWorkerIdempotencyConflict
			}
			existing.Created = false
			result = existing.WorkerArtifactChunk
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if artifact.State != "uploading" {
			return fmt.Errorf("%w: output artifact is not uploading", controlapi.ErrWorkerRequestInvalid)
		}
		encrypted, err := cipher.sealWithKey(workerArtifactBodyScope(accountID, command.ArtifactID, command.ChunkIndex),
			string(command.Plaintext), artifact.EncryptionKeyID)
		if err != nil {
			return fmt.Errorf("encrypt worker artifact chunk: %w", err)
		}
		if encrypted.Digest != command.PlaintextDigest || encrypted.PlaintextBytes != len(command.Plaintext) {
			return fmt.Errorf("worker artifact plaintext digest changed during encryption")
		}
		command.Ciphertext = encrypted.Ciphertext
		command.EncodedLength = len(encrypted.Ciphertext)
		command.PlaintextLength = encrypted.PlaintextBytes
		command.EncryptionKeyID = encrypted.KeyID
		command.Nonce = encrypted.Nonce
		command.AuthenticatedDigest = encrypted.Digest
		result = workerArtifactChunkFromCommand(command)
		affected, err := tx.exec(ctx, `insert into fort_private.artifact_chunk (
  account_id, artifact_id, chunk_index, ciphertext, encoded_length,
  plaintext_length, encryption_key_id, nonce, authenticated_digest, created_at
) values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
on conflict (account_id, artifact_id, chunk_index) do nothing`, accountID, command.ArtifactID,
			command.ChunkIndex, command.Ciphertext, command.EncodedLength, command.PlaintextLength,
			command.EncryptionKeyID, command.Nonce, command.AuthenticatedDigest, command.CreatedAt.UTC())
		if err != nil {
			return fmt.Errorf("insert worker artifact chunk: %w", err)
		}
		if affected != 1 {
			return controlapi.ErrWorkerIdempotencyConflict
		}
		result.Created = true
		return nil
	})
	return result, err
}

func (store *Store) FinalizeWorkerArtifact(ctx context.Context, command controlapi.WorkerArtifactFinalizeCommand) (controlapi.WorkerArtifact, error) {
	accountID, err := store.operationAccount(command.AccountID)
	if err != nil {
		return controlapi.WorkerArtifact{}, err
	}
	if err := validateWorkerArtifactFinalize(command); err != nil {
		return controlapi.WorkerArtifact{}, err
	}
	digest, err := workerArtifactFinalizeDigest(command)
	if err != nil {
		return controlapi.WorkerArtifact{}, err
	}
	cipher, err := store.collaborationBodies()
	if err != nil {
		return controlapi.WorkerArtifact{}, err
	}
	var result controlapi.WorkerArtifact
	err = store.withTransaction(ctx, accountID, func(tx transaction) error {
		if err := requireActiveWorkerArtifactLease(ctx, tx, artifactLeaseForFinalize(command)); err != nil {
			return err
		}
		result, err = loadWorkerArtifactManifest(ctx, tx, accountID, command.ArtifactID, true)
		if err != nil {
			return err
		}
		if result.ExecutionAttemptID != command.ExecutionAttemptID || result.Kind != "output" {
			return controlapi.ErrWorkerStaleLease
		}
		if _, err := claimWorkerIdempotency(ctx, tx, accountID, workerArtifactFinalizeScope, command.IdempotencyKey,
			digest, "artifact", command.ArtifactID, command.FinalizedAt); err != nil {
			return err
		}
		if result.State == "finalized" {
			result.Created = false
			return nil
		}
		if result.State != "uploading" {
			return fmt.Errorf("%w: output artifact is terminal", controlapi.ErrWorkerRequestInvalid)
		}
		if err := verifyWorkerArtifactPlaintextDigest(ctx, tx, cipher, accountID, result); err != nil {
			return err
		}
		affected, err := tx.exec(ctx, `update fort_private.artifact
set state = 'finalized', finalized_at = $1
where account_id = $2 and artifact_id = $3 and execution_attempt_id = $4
  and kind = 'output' and state = 'uploading'`, command.FinalizedAt.UTC(), accountID,
			command.ArtifactID, command.ExecutionAttemptID)
		if err != nil {
			return mapWorkerArtifactTransitionError(err)
		}
		if affected != 1 {
			return controlapi.ErrWorkerIdempotencyConflict
		}
		finalizedAt := command.FinalizedAt.UTC()
		result.State, result.FinalizedAt, result.Created = "finalized", &finalizedAt, true
		return nil
	})
	return result, err
}

func verifyWorkerArtifactPlaintextDigest(ctx context.Context, tx transaction, cipher collaborationBodyCipher, accountID string, artifact controlapi.WorkerArtifact) error {
	result, err := tx.query(ctx, `select chunk_index, ciphertext, encoded_length, plaintext_length,
  encryption_key_id, nonce, authenticated_digest
from fort_private.artifact_chunk
where account_id = $1 and artifact_id = $2
order by chunk_index`, accountID, artifact.ArtifactID)
	if err != nil {
		return err
	}
	defer result.close()

	hasher := sha256.New()
	var plaintextLength, encodedLength int64
	chunkCount := 0
	for result.next() {
		var chunkIndex, chunkEncodedLength, chunkPlaintextLength int
		var ciphertext, nonce []byte
		var keyID, digest string
		if err := result.scan(&chunkIndex, &ciphertext, &chunkEncodedLength, &chunkPlaintextLength,
			&keyID, &nonce, &digest); err != nil {
			return err
		}
		if chunkIndex != chunkCount || keyID != artifact.EncryptionKeyID ||
			chunkEncodedLength != len(ciphertext) || chunkPlaintextLength < 1 ||
			chunkPlaintextLength > controlapi.MaximumArtifactChunkPlaintextBytes {
			return fmt.Errorf("%w: output artifact chunk manifest does not match", controlapi.ErrWorkerArtifactIncomplete)
		}
		plaintext, err := cipher.open(workerArtifactBodyScope(accountID, artifact.ArtifactID, chunkIndex), collaborationEncryptedBody{
			Ciphertext: ciphertext, KeyID: keyID, Nonce: nonce, Digest: digest, PlaintextBytes: chunkPlaintextLength,
		})
		if err != nil {
			return fmt.Errorf("%w: verify output artifact chunk %d: %v", controlapi.ErrWorkerArtifactIncomplete, chunkIndex, err)
		}
		_, _ = hasher.Write([]byte(plaintext))
		plaintextLength += int64(chunkPlaintextLength)
		encodedLength += int64(chunkEncodedLength)
		chunkCount++
	}
	if err := result.errResult(); err != nil {
		return err
	}
	if chunkCount != artifact.ExpectedChunkCount || plaintextLength != artifact.ExpectedPlaintextLength ||
		encodedLength != artifact.ExpectedEncodedLength || hex.EncodeToString(hasher.Sum(nil)) != artifact.LogicalDigest {
		return fmt.Errorf("%w: output artifact plaintext aggregate does not match", controlapi.ErrWorkerArtifactIncomplete)
	}
	return nil
}

func workerArtifactBodyScope(accountID, artifactID string, chunkIndex int) securebody.Scope {
	return securebody.Scope{
		AccountID: accountID, RecordType: "worker_output_artifact_chunk",
		RecordID: artifactID + "#" + strconv.Itoa(chunkIndex),
	}
}

func requireActiveWorkerArtifactLease(ctx context.Context, tx transaction, identity workerArtifactLeaseIdentity) error {
	var expiresAt time.Time
	var attemptState, targetState string
	err := tx.queryRow(ctx, `select lease.expires_at, attempt.state, target.state
from fort_private.worker_lease as lease
join fort_private.execution_attempt as attempt
  on attempt.account_id = lease.account_id and attempt.execution_attempt_id = lease.execution_attempt_id
join fort_private.conversation_target as target
  on target.account_id = lease.account_id and target.target_id = lease.target_id
join fort_private.worker as machine
  on machine.account_id = lease.account_id and machine.worker_id = lease.worker_id
where lease.account_id = $1 and lease.lease_id = $2 and lease.execution_attempt_id = $3
  and lease.target_id = $4 and lease.worker_id = $5 and lease.fence_token = $6
  and machine.machine_id = $7 and machine.state <> 'revoked' and lease.state = 'active'
for update of lease`, identity.accountID, identity.leaseID, identity.attemptID, identity.targetID,
		identity.workerID, identity.fenceToken, identity.machineID).scan(&expiresAt, &attemptState, &targetState)
	if errors.Is(err, pgx.ErrNoRows) {
		return controlapi.ErrWorkerStaleLease
	}
	if err != nil {
		return err
	}
	if !identity.observedAt.Before(expiresAt) ||
		(attemptState != "leased" && attemptState != "working") ||
		(targetState != "claimed" && targetState != "working") {
		return controlapi.ErrWorkerStaleLease
	}
	return nil
}

func loadWorkerArtifactManifest(ctx context.Context, tx transaction, accountID, artifactID string, lock bool) (controlapi.WorkerArtifact, error) {
	query := `select execution_attempt_id, kind, state, expected_chunk_count,
  expected_plaintext_length, expected_encoded_length, logical_digest,
  encryption_key_id, created_at, finalized_at
from fort_private.artifact
where account_id = $1 and artifact_id = $2`
	if lock {
		query += "\nfor update"
	}
	var result controlapi.WorkerArtifact
	var finalizedAt sql.NullTime
	err := tx.queryRow(ctx, query, accountID, artifactID).scan(&result.ExecutionAttemptID, &result.Kind,
		&result.State, &result.ExpectedChunkCount, &result.ExpectedPlaintextLength, &result.ExpectedEncodedLength,
		&result.LogicalDigest, &result.EncryptionKeyID, &result.CreatedAt, &finalizedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return controlapi.WorkerArtifact{}, controlapi.ErrWorkerStaleLease
	}
	if err != nil {
		return controlapi.WorkerArtifact{}, err
	}
	result.ArtifactID = artifactID
	result.CreatedAt = result.CreatedAt.UTC()
	result.Chunks = make([]controlapi.WorkerArtifactChunk, 0)
	if finalizedAt.Valid {
		value := finalizedAt.Time.UTC()
		result.FinalizedAt = &value
	}
	return result, nil
}

func loadWorkerArtifactChunks(ctx context.Context, tx transaction, accountID, artifactID string) ([]controlapi.WorkerArtifactChunk, error) {
	result, err := tx.query(ctx, `select chunk_index, encoded_length, plaintext_length,
  encryption_key_id, authenticated_digest, created_at
from fort_private.artifact_chunk
where account_id = $1 and artifact_id = $2
order by chunk_index`, accountID, artifactID)
	if err != nil {
		return nil, err
	}
	defer result.close()
	chunks := make([]controlapi.WorkerArtifactChunk, 0)
	for result.next() {
		var chunk controlapi.WorkerArtifactChunk
		chunk.ArtifactID = artifactID
		if err := result.scan(&chunk.ChunkIndex, &chunk.EncodedLength, &chunk.PlaintextLength,
			&chunk.EncryptionKeyID, &chunk.AuthenticatedDigest, &chunk.CreatedAt); err != nil {
			return nil, err
		}
		chunk.CreatedAt = chunk.CreatedAt.UTC()
		chunks = append(chunks, chunk)
	}
	if err := result.errResult(); err != nil {
		return nil, err
	}
	return chunks, nil
}

type workerArtifactChunkBody struct {
	controlapi.WorkerArtifactChunk
	ciphertext []byte
	nonce      []byte
}

func loadWorkerArtifactChunkBody(ctx context.Context, tx transaction, accountID, artifactID string, chunkIndex int) (workerArtifactChunkBody, error) {
	var result workerArtifactChunkBody
	result.ArtifactID, result.ChunkIndex = artifactID, chunkIndex
	err := tx.queryRow(ctx, `select ciphertext, encoded_length, plaintext_length,
  encryption_key_id, nonce, authenticated_digest, created_at
from fort_private.artifact_chunk
where account_id = $1 and artifact_id = $2 and chunk_index = $3`, accountID, artifactID, chunkIndex).scan(
		&result.ciphertext, &result.EncodedLength, &result.PlaintextLength, &result.EncryptionKeyID,
		&result.nonce, &result.AuthenticatedDigest, &result.CreatedAt)
	result.CreatedAt = result.CreatedAt.UTC()
	return result, err
}

func workerArtifactChunkMatches(existing workerArtifactChunkBody, command controlapi.WorkerArtifactChunkCommand) bool {
	return existing.PlaintextLength == len(command.Plaintext) && existing.AuthenticatedDigest == command.PlaintextDigest
}

func workerArtifactFromCreate(command controlapi.WorkerArtifactCreateCommand) controlapi.WorkerArtifact {
	return controlapi.WorkerArtifact{
		ArtifactID: command.ArtifactID, ExecutionAttemptID: command.ExecutionAttemptID,
		Kind: "output", State: "uploading", ExpectedChunkCount: command.ExpectedChunkCount,
		ExpectedPlaintextLength: command.ExpectedPlaintextLength, ExpectedEncodedLength: command.ExpectedEncodedLength,
		LogicalDigest: command.LogicalDigest, EncryptionKeyID: command.EncryptionKeyID,
		Chunks: make([]controlapi.WorkerArtifactChunk, 0), CreatedAt: command.CreatedAt.UTC(),
	}
}

func workerArtifactChunkFromCommand(command controlapi.WorkerArtifactChunkCommand) controlapi.WorkerArtifactChunk {
	return controlapi.WorkerArtifactChunk{
		ArtifactID: command.ArtifactID, ChunkIndex: command.ChunkIndex, EncodedLength: command.EncodedLength,
		PlaintextLength: command.PlaintextLength, EncryptionKeyID: command.EncryptionKeyID,
		AuthenticatedDigest: command.AuthenticatedDigest, CreatedAt: command.CreatedAt.UTC(),
	}
}

func workerArtifactManifestMatchesCreate(existing controlapi.WorkerArtifact, command controlapi.WorkerArtifactCreateCommand) bool {
	return existing.ExecutionAttemptID == command.ExecutionAttemptID && existing.Kind == "output" &&
		existing.ExpectedChunkCount == command.ExpectedChunkCount && existing.ExpectedPlaintextLength == command.ExpectedPlaintextLength &&
		existing.ExpectedEncodedLength == command.ExpectedEncodedLength && existing.LogicalDigest == command.LogicalDigest
}

func validateWorkerArtifactCreate(command controlapi.WorkerArtifactCreateCommand) error {
	if !validWorkerArtifactLeaseIdentity(artifactLeaseForCreate(command)) || !validWorkerIdentifier(command.IdempotencyKey) ||
		!validWorkerIdentifier(command.ArtifactID) || command.ExpectedChunkCount < 1 || command.ExpectedChunkCount > controlapi.MaximumArtifactChunks ||
		command.ExpectedPlaintextLength < 0 || command.ExpectedPlaintextLength > controlapi.MaximumArtifactPlaintextBytes ||
		command.ExpectedPlaintextLength > int64(command.ExpectedChunkCount*controlapi.MaximumArtifactChunkPlaintextBytes) ||
		!lowerWorkerDigest(command.LogicalDigest) {
		return fmt.Errorf("worker artifact create command is invalid")
	}
	return nil
}

func validateWorkerArtifactStatus(command controlapi.WorkerArtifactStatusCommand) error {
	if !validWorkerArtifactLeaseIdentity(artifactLeaseForStatus(command)) || !validWorkerIdentifier(command.IdempotencyKey) ||
		!validWorkerIdentifier(command.ArtifactID) {
		return fmt.Errorf("worker artifact status command is invalid")
	}
	return nil
}

func validateWorkerArtifactChunk(command controlapi.WorkerArtifactChunkCommand) error {
	if !validWorkerArtifactLeaseIdentity(artifactLeaseForChunk(command)) || !validWorkerIdentifier(command.IdempotencyKey) ||
		!validWorkerIdentifier(command.ArtifactID) || command.ChunkIndex < 0 || command.ChunkIndex >= controlapi.MaximumArtifactChunks ||
		len(command.Plaintext) < 1 || len(command.Plaintext) > controlapi.MaximumArtifactChunkPlaintextBytes ||
		!lowerWorkerDigest(command.PlaintextDigest) {
		return fmt.Errorf("worker artifact chunk command is invalid")
	}
	digest := sha256.Sum256(command.Plaintext)
	if hex.EncodeToString(digest[:]) != command.PlaintextDigest {
		return fmt.Errorf("worker artifact chunk plaintext digest does not match")
	}
	return nil
}

func validateWorkerArtifactFinalize(command controlapi.WorkerArtifactFinalizeCommand) error {
	if !validWorkerArtifactLeaseIdentity(artifactLeaseForFinalize(command)) || !validWorkerIdentifier(command.IdempotencyKey) ||
		!validWorkerIdentifier(command.ArtifactID) {
		return fmt.Errorf("worker artifact finalize command is invalid")
	}
	return nil
}

func validWorkerArtifactLeaseIdentity(identity workerArtifactLeaseIdentity) bool {
	return validWorkerIdentifier(identity.workerID) && validWorkerIdentifier(identity.machineID) &&
		validWorkerIdentifier(identity.targetID) && validWorkerIdentifier(identity.attemptID) &&
		validWorkerIdentifier(identity.leaseID) && identity.fenceToken > 0 && !identity.observedAt.IsZero()
}

func artifactLeaseForCreate(command controlapi.WorkerArtifactCreateCommand) workerArtifactLeaseIdentity {
	return workerArtifactLeaseIdentity{command.AccountID, command.WorkerID, command.MachineID, command.TargetID,
		command.ExecutionAttemptID, command.LeaseID, command.FenceToken, command.CreatedAt}
}

func artifactLeaseForStatus(command controlapi.WorkerArtifactStatusCommand) workerArtifactLeaseIdentity {
	return workerArtifactLeaseIdentity{command.AccountID, command.WorkerID, command.MachineID, command.TargetID,
		command.ExecutionAttemptID, command.LeaseID, command.FenceToken, command.ObservedAt}
}

func artifactLeaseForChunk(command controlapi.WorkerArtifactChunkCommand) workerArtifactLeaseIdentity {
	return workerArtifactLeaseIdentity{command.AccountID, command.WorkerID, command.MachineID, command.TargetID,
		command.ExecutionAttemptID, command.LeaseID, command.FenceToken, command.CreatedAt}
}

func artifactLeaseForFinalize(command controlapi.WorkerArtifactFinalizeCommand) workerArtifactLeaseIdentity {
	return workerArtifactLeaseIdentity{command.AccountID, command.WorkerID, command.MachineID, command.TargetID,
		command.ExecutionAttemptID, command.LeaseID, command.FenceToken, command.FinalizedAt}
}

func workerArtifactCreateDigest(command controlapi.WorkerArtifactCreateCommand) (string, error) {
	return workerCommandDigest(struct {
		AccountID, WorkerID, MachineID, TargetID, ExecutionAttemptID, LeaseID string
		FenceToken                                                            int64
		IdempotencyKey, ArtifactID                                            string
		ExpectedChunkCount                                                    int
		ExpectedPlaintextLength                                               int64
		LogicalDigest                                                         string
	}{command.AccountID, command.WorkerID, command.MachineID, command.TargetID, command.ExecutionAttemptID,
		command.LeaseID, command.FenceToken, command.IdempotencyKey, command.ArtifactID,
		command.ExpectedChunkCount, command.ExpectedPlaintextLength, command.LogicalDigest})
}

func workerArtifactChunkDigest(command controlapi.WorkerArtifactChunkCommand) (string, error) {
	return workerCommandDigest(struct {
		AccountID, WorkerID, MachineID, TargetID, ExecutionAttemptID, LeaseID string
		FenceToken                                                            int64
		IdempotencyKey, ArtifactID                                            string
		ChunkIndex                                                            int
		Plaintext                                                             []byte
		PlaintextDigest                                                       string
	}{command.AccountID, command.WorkerID, command.MachineID, command.TargetID, command.ExecutionAttemptID,
		command.LeaseID, command.FenceToken, command.IdempotencyKey, command.ArtifactID, command.ChunkIndex,
		command.Plaintext, command.PlaintextDigest})
}

func workerArtifactFinalizeDigest(command controlapi.WorkerArtifactFinalizeCommand) (string, error) {
	return workerCommandDigest(struct {
		AccountID, WorkerID, MachineID, TargetID, ExecutionAttemptID, LeaseID string
		FenceToken                                                            int64
		IdempotencyKey, ArtifactID                                            string
	}{command.AccountID, command.WorkerID, command.MachineID, command.TargetID, command.ExecutionAttemptID,
		command.LeaseID, command.FenceToken, command.IdempotencyKey, command.ArtifactID})
}

func mapWorkerArtifactTransitionError(err error) error {
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) && postgresError.Code == "23514" && postgresError.Message == "artifact_incomplete" {
		return fmt.Errorf("%w: %v", controlapi.ErrWorkerArtifactIncomplete, err)
	}
	return err
}
