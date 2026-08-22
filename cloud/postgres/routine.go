package postgres

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/tobsai/fort/cloud/controlapi"
	"github.com/tobsai/fort/cloud/securebody"
	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/ledger"
	coreworker "github.com/tobsai/fort/core/worker"
)

const (
	postgresRoutineNotApplicable = "fort:not_applicable"
	routineCreateScope           = "routine.create"
)

var _ ledger.RoutineRepository = (*Store)(nil)

type routineValue struct {
	Value string `json:"value"`
}

type routineFreshness struct {
	Seconds int64 `json:"seconds"`
}

type routineBindingCompatibility struct {
	BehaviorRevisionID string `json:"behavior_revision_id"`
	BindingRevisionID  string `json:"binding_revision_id"`
}

type routineActivityMetadata struct {
	State              conversation.RoutineRunState `json:"state"`
	AttemptID          string                       `json:"attempt_id,omitempty"`
	LeaseID            string                       `json:"lease_id,omitempty"`
	LeaseExpiresAt     time.Time                    `json:"lease_expires_at,omitempty"`
	Activity           string                       `json:"activity"`
	FailureCode        string                       `json:"failure_code,omitempty"`
	NextAction         string                       `json:"next_action,omitempty"`
	ApprovalEvidenceID string                       `json:"approval_evidence_id,omitempty"`
}

func postgresRoutineTrigger(trigger conversation.RoutineTrigger, schedule, timezone string) (string, string, string) {
	if trigger == conversation.RoutineTriggerEvent {
		return "event", postgresRoutineNotApplicable, postgresRoutineNotApplicable
	}
	return "cron", schedule, timezone
}

func domainRoutineTrigger(trigger, schedule, timezone string) (conversation.RoutineTrigger, string, string, error) {
	switch trigger {
	case "cron":
		return conversation.RoutineTriggerSchedule, schedule, timezone, nil
	case "event":
		if schedule != postgresRoutineNotApplicable || timezone != postgresRoutineNotApplicable {
			return "", "", "", fmt.Errorf("event Routine contains schedule semantics")
		}
		return conversation.RoutineTriggerEvent, "", "", nil
	default:
		return "", "", "", fmt.Errorf("unsupported first-release Routine trigger %q", trigger)
	}
}

func (store *Store) RecordSourceRoutineProjection(ctx context.Context, projection ledger.SourceRoutineProjection) (ledger.SourceRoutineProjection, error) {
	accountID, err := store.operationAccount(projection.AccountID)
	if err != nil {
		return ledger.SourceRoutineProjection{}, err
	}
	projection.AccountID = accountID
	if err := projection.Validate(); err != nil {
		return ledger.SourceRoutineProjection{}, err
	}
	var stored ledger.SourceRoutineProjection
	err = store.withTransaction(ctx, accountID, func(tx transaction) error {
		stored, err = getPostgresSourceRoutineProjection(ctx, tx, accountID, projection.ID)
		if err == nil {
			if !reflect.DeepEqual(stored, projection) {
				return fmt.Errorf("%w: source Routine projection %q", ledger.ErrIdempotencyConflict, projection.ID)
			}
			return nil
		}
		if !errors.Is(err, ledger.ErrNotFound) {
			return err
		}
		var exists int
		if err := tx.queryRow(ctx, `select 1 from fort_private.execution_source
where account_id=$1 and execution_source_id=$2`, accountID, projection.ExecutionSourceID).scan(&exists); err != nil {
			if isNoRows(err) {
				return fmt.Errorf("%w: Execution Source %q", ledger.ErrNotFound, projection.ExecutionSourceID)
			}
			return err
		}
		if _, err := tx.exec(ctx, `insert into fort_private.source_routine_projection (
  account_id,source_routine_projection_id,execution_source_id,opaque_source_routine_id,
  projection_revision,authority,schedule_snapshot,projection_digest,last_occurrence_at,
  next_occurrence_at,observed_at
) values ($1,$2,$3,$4,$5,'source_native',$6::jsonb,$7,$8,$9,$10)`, accountID, projection.ID,
			projection.ExecutionSourceID, projection.OpaqueSourceRoutineID, projection.ProjectionRevision,
			string(projection.ScheduleSnapshot), projection.ProjectionDigest, nullablePostgresTime(projection.LastOccurrenceAt),
			nullablePostgresTime(projection.NextOccurrenceAt), projection.ObservedAt.UTC()); err != nil {
			return translateRoutineWriteError(err, "source Routine projection")
		}
		stored, err = getPostgresSourceRoutineProjection(ctx, tx, accountID, projection.ID)
		return err
	})
	return stored, err
}

func (store *Store) ListSourceRoutineProjections(ctx context.Context, accountID, executionSourceID string) ([]ledger.SourceRoutineProjection, error) {
	accountID, err := store.operationAccount(accountID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(executionSourceID) == "" {
		return nil, fmt.Errorf("source Routine projection Execution Source id is required")
	}
	items := make([]ledger.SourceRoutineProjection, 0)
	err = store.withTransaction(ctx, accountID, func(tx transaction) error {
		var exists int
		if err := tx.queryRow(ctx, `select 1 from fort_private.execution_source
where account_id=$1 and execution_source_id=$2`, accountID, executionSourceID).scan(&exists); err != nil {
			if isNoRows(err) {
				return fmt.Errorf("%w: Execution Source %q", ledger.ErrNotFound, executionSourceID)
			}
			return err
		}
		result, err := tx.query(ctx, `select source_routine_projection_id,account_id::text,execution_source_id,
  opaque_source_routine_id,projection_revision,schedule_snapshot::text,projection_digest,
  last_occurrence_at,next_occurrence_at,observed_at
from fort_private.source_routine_projection
where account_id=$1 and execution_source_id=$2
order by observed_at,source_routine_projection_id`, accountID, executionSourceID)
		if err != nil {
			return err
		}
		defer result.close()
		for result.next() {
			item, err := scanPostgresSourceRoutineProjection(result)
			if err != nil {
				return err
			}
			items = append(items, item)
		}
		return result.errResult()
	})
	return items, err
}

func getPostgresSourceRoutineProjection(ctx context.Context, tx transaction, accountID, projectionID string) (ledger.SourceRoutineProjection, error) {
	projection, err := scanPostgresSourceRoutineProjection(tx.queryRow(ctx, `select source_routine_projection_id,
  account_id::text,execution_source_id,opaque_source_routine_id,projection_revision,
  schedule_snapshot::text,projection_digest,last_occurrence_at,next_occurrence_at,observed_at
from fort_private.source_routine_projection where account_id=$1 and source_routine_projection_id=$2`,
		accountID, projectionID))
	if isNoRows(err) {
		return ledger.SourceRoutineProjection{}, fmt.Errorf("%w: source Routine projection %q", ledger.ErrNotFound, projectionID)
	}
	return projection, err
}

func scanPostgresSourceRoutineProjection(source row) (ledger.SourceRoutineProjection, error) {
	var projection ledger.SourceRoutineProjection
	var snapshot string
	var last, next sql.NullTime
	if err := source.scan(&projection.ID, &projection.AccountID, &projection.ExecutionSourceID,
		&projection.OpaqueSourceRoutineID, &projection.ProjectionRevision, &snapshot,
		&projection.ProjectionDigest, &last, &next, &projection.ObservedAt); err != nil {
		return ledger.SourceRoutineProjection{}, err
	}
	projection.ScheduleSnapshot = json.RawMessage(snapshot)
	if last.Valid {
		projection.LastOccurrenceAt = last.Time.UTC()
	}
	if next.Valid {
		projection.NextOccurrenceAt = next.Time.UTC()
	}
	projection.ObservedAt = projection.ObservedAt.UTC()
	return projection, nil
}

func (store *Store) CreateRoutine(ctx context.Context, command ledger.CreateRoutineCommand) (ledger.RoutineRecord, error) {
	return store.createPostgresRoutine(ctx, command, nil)
}

func (store *Store) ImportSourceRoutine(ctx context.Context, command ledger.ImportRoutineCommand) (ledger.RoutineRecord, error) {
	return store.createPostgresRoutine(ctx, command.Create, &command)
}

func (store *Store) createPostgresRoutine(ctx context.Context, create ledger.CreateRoutineCommand, imported *ledger.ImportRoutineCommand) (ledger.RoutineRecord, error) {
	accountID, err := store.operationAccount(create.Routine.AccountID)
	if err != nil {
		return ledger.RoutineRecord{}, err
	}
	create.Routine.AccountID = accountID
	if imported == nil {
		if err := create.Validate(); err != nil {
			return ledger.RoutineRecord{}, err
		}
	} else {
		imported.Create = create
	}
	var digest string
	if imported == nil {
		digest, err = create.Digest()
	} else {
		digest, err = imported.Digest()
	}
	if err != nil {
		return ledger.RoutineRecord{}, err
	}
	var record ledger.RoutineRecord
	err = store.withTransaction(ctx, accountID, func(tx transaction) error {
		claimed, err := claimLifecycleIdempotency(ctx, tx, accountID, routineCreateScope,
			create.IdempotencyKey, digest, "routine", create.Routine.ID, create.Routine.CreatedAt)
		if err != nil {
			return err
		}
		if !claimed {
			record, err = getPostgresRoutine(ctx, tx, accountID, create.Routine.ID)
			return err
		}
		if imported != nil {
			projection, err := getPostgresSourceRoutineProjection(ctx, tx, accountID,
				imported.Receipt.SourceRoutineProjectionID)
			if err != nil {
				return err
			}
			if err := imported.Validate(projection); err != nil {
				return err
			}
		}
		if err := validatePostgresRoutineParents(ctx, tx, accountID, create); err != nil {
			return err
		}
		createdAt := create.Routine.CreatedAt.UTC()
		if _, err := tx.exec(ctx, `insert into fort_private.routine (
  account_id,routine_id,agent_id,authority,state,current_revision_id,created_at,updated_at
) values ($1,$2,$3,'fort_cloud','active',$4,$5,$5)`, accountID, create.Routine.ID,
			create.Routine.AgentID, create.Routine.CurrentRevisionID, createdAt); err != nil {
			return translateRoutineWriteError(err, "Routine")
		}
		if err := insertPostgresRoutineRevision(ctx, tx, accountID, create.Revision, digest, ""); err != nil {
			return err
		}
		if imported != nil {
			receipt := imported.Receipt
			if _, err := tx.exec(ctx, `insert into fort_private.routine_import_receipt (
  account_id,routine_import_receipt_id,source_routine_projection_id,routine_id,routine_revision_id,
  source_disabled_at,exact_last_source_occurrence_at,exact_next_source_occurrence_at,
  fencing_receipt_ciphertext,fencing_receipt_key_id,fencing_receipt_nonce,fencing_receipt_digest,imported_at
) values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, accountID, receipt.ID,
				receipt.SourceRoutineProjectionID, receipt.RoutineID, receipt.RoutineRevisionID,
				receipt.SourceDisabledAt.UTC(), nullablePostgresTime(receipt.ExactLastSourceOccurrenceAt),
				nullablePostgresTime(receipt.ExactNextSourceOccurrenceAt), receipt.FencingReceiptCiphertext,
				receipt.FencingReceiptKeyID, receipt.FencingReceiptNonce, receipt.FencingReceiptDigest,
				receipt.ImportedAt.UTC()); err != nil {
				return translateRoutineWriteError(err, "Routine import")
			}
		}
		if err := appendRoutineEvent(ctx, tx, accountID, create.Routine.ID, "routine.created",
			map[string]any{"agent_id": create.Routine.AgentID, "routine_revision_id": create.Revision.ID}, createdAt); err != nil {
			return err
		}
		record, err = getPostgresRoutine(ctx, tx, accountID, create.Routine.ID)
		return err
	})
	return record, err
}

func validatePostgresRoutineParents(ctx context.Context, tx transaction, accountID string, command ledger.CreateRoutineCommand) error {
	parent, err := loadPostgresAgentDirectParent(ctx, tx, accountID, command.Routine.AgentID,
		command.Revision.ResultConversationID)
	if err != nil {
		return err
	}
	if parent.agentState != string(conversation.AgentOpen) || parent.conversationState != string(conversation.ConversationOpen) {
		return fmt.Errorf("%w: Routine owner and result Conversation must be open", ledger.ErrStateConflict)
	}
	if parent.behaviorRevisionID != command.Revision.BehaviorRevisionID ||
		parent.bindingRevisionID != command.Revision.BindingRevisionID {
		return fmt.Errorf("%w: Routine must pin the owning Agent's current Behavior and Binding Revisions", ledger.ErrRevisionConflict)
	}
	return nil
}

func insertPostgresRoutineRevision(ctx context.Context, tx transaction, accountID string,
	revision conversation.RoutineRevision, digest, supersedes string) error {
	trigger, schedule, timezone := postgresRoutineTrigger(revision.Trigger, revision.Schedule, revision.Timezone)
	input, _ := json.Marshal(routineValue{Value: revision.InputSource})
	freshness, _ := json.Marshal(routineFreshness{Seconds: revision.FreshnessSeconds})
	approval, _ := json.Marshal(routineValue{Value: revision.ApprovalBoundary})
	retry, _ := json.Marshal(routineValue{Value: revision.RetryPolicy})
	catchUp, _ := json.Marshal(routineValue{Value: revision.CatchUpPolicy})
	lateness, _ := json.Marshal(routineValue{Value: revision.LatenessPolicy})
	compatibility, _ := json.Marshal(routineBindingCompatibility{BehaviorRevisionID: revision.BehaviorRevisionID,
		BindingRevisionID: revision.BindingRevisionID})
	if _, err := tx.exec(ctx, `insert into fort_private.routine_revision (
  account_id,routine_revision_id,routine_id,agent_id,revision,behavior_revision_id,binding_revision_id,
  trigger_kind,schedule_expression,timezone,next_occurrence_at,input_source,freshness_policy,
  expected_result,result_conversation_id,approval_policy,missing_input_policy,retry_policy,
  catch_up_policy,lateness_policy,binding_compatibility,command_digest,supersedes_routine_revision_id,created_at
) values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12::jsonb,$13::jsonb,$14,$15,$16::jsonb,$17,
  $18::jsonb,$19::jsonb,$20::jsonb,$21::jsonb,$22,$23,$24)`, accountID, revision.ID, revision.RoutineID,
		revision.AgentID, revision.Revision, revision.BehaviorRevisionID, revision.BindingRevisionID,
		trigger, schedule, timezone, nullablePostgresTime(revision.NextOccurrence), string(input), string(freshness),
		revision.ExpectedResult, revision.ResultConversationID, string(approval), revision.MissingInputBehavior,
		string(retry), string(catchUp), string(lateness), string(compatibility), digest,
		nullablePostgresString(supersedes), revision.CreatedAt.UTC()); err != nil {
		return translateRoutineWriteError(err, "Routine Revision")
	}
	return nil
}

func (store *Store) GetRoutine(ctx context.Context, accountID, routineID string) (ledger.RoutineRecord, error) {
	accountID, err := store.operationAccount(accountID)
	if err != nil {
		return ledger.RoutineRecord{}, err
	}
	if strings.TrimSpace(routineID) == "" {
		return ledger.RoutineRecord{}, fmt.Errorf("Routine id is required")
	}
	var record ledger.RoutineRecord
	err = store.withTransaction(ctx, accountID, func(tx transaction) error {
		record, err = getPostgresRoutine(ctx, tx, accountID, routineID)
		return err
	})
	return record, err
}

func (store *Store) ListRoutines(ctx context.Context, accountID, agentID string) ([]ledger.RoutineRecord, error) {
	accountID, err := store.operationAccount(accountID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(agentID) == "" {
		return nil, fmt.Errorf("Routine Agent id is required")
	}
	items := make([]ledger.RoutineRecord, 0)
	err = store.withTransaction(ctx, accountID, func(tx transaction) error {
		var exists int
		if err := tx.queryRow(ctx, `select 1 from fort_private.stable_agent
where account_id=$1 and agent_id=$2`, accountID, agentID).scan(&exists); err != nil {
			if isNoRows(err) {
				return fmt.Errorf("%w: Agent %q", ledger.ErrNotFound, agentID)
			}
			return err
		}
		rows, err := tx.query(ctx, `select routine_id from fort_private.routine
where account_id=$1 and agent_id=$2 order by created_at,routine_id`, accountID, agentID)
		if err != nil {
			return err
		}
		defer rows.close()
		ids := make([]string, 0)
		for rows.next() {
			var id string
			if err := rows.scan(&id); err != nil {
				return err
			}
			ids = append(ids, id)
		}
		if err := rows.errResult(); err != nil {
			return err
		}
		for _, id := range ids {
			item, err := getPostgresRoutine(ctx, tx, accountID, id)
			if err != nil {
				return err
			}
			items = append(items, item)
		}
		return nil
	})
	return items, err
}

func getPostgresRoutine(ctx context.Context, tx transaction, accountID, routineID string) (ledger.RoutineRecord, error) {
	var record ledger.RoutineRecord
	var state string
	err := tx.queryRow(ctx, `select routine_id,account_id::text,agent_id,current_revision_id,state,created_at
from fort_private.routine where account_id=$1 and routine_id=$2`, accountID, routineID).scan(
		&record.Routine.ID, &record.Routine.AccountID, &record.Routine.AgentID,
		&record.Routine.CurrentRevisionID, &state, &record.Routine.CreatedAt)
	if isNoRows(err) {
		return ledger.RoutineRecord{}, fmt.Errorf("%w: Routine %q", ledger.ErrNotFound, routineID)
	}
	if err != nil {
		return ledger.RoutineRecord{}, err
	}
	record.Routine.State, record.PauseReason, err = domainRoutineState(state)
	if err != nil {
		return ledger.RoutineRecord{}, err
	}
	record.Routine.CreatedAt = record.Routine.CreatedAt.UTC()
	record.CurrentRevision, err = scanPostgresRoutineRevision(tx.queryRow(ctx, `select routine_revision_id,
  routine_id,revision,agent_id,behavior_revision_id,binding_revision_id,trigger_kind,
  schedule_expression,timezone,next_occurrence_at,input_source::text,freshness_policy::text,
  expected_result,result_conversation_id,approval_policy::text,missing_input_policy,
  retry_policy::text,catch_up_policy::text,lateness_policy::text,binding_compatibility::text,created_at
from fort_private.routine_revision
where account_id=$1 and routine_id=$2 and routine_revision_id=$3`, accountID, routineID,
		record.Routine.CurrentRevisionID))
	if err != nil {
		return ledger.RoutineRecord{}, err
	}
	receipt, err := scanPostgresRoutineImportReceipt(tx.queryRow(ctx, `select routine_import_receipt_id,
  account_id::text,source_routine_projection_id,routine_id,routine_revision_id,source_disabled_at,
  exact_last_source_occurrence_at,exact_next_source_occurrence_at,fencing_receipt_ciphertext,
  fencing_receipt_key_id,fencing_receipt_nonce,fencing_receipt_digest,imported_at
from fort_private.routine_import_receipt where account_id=$1 and routine_id=$2`, accountID, routineID))
	if err == nil {
		record.ImportReceipt = &receipt
	} else if !isNoRows(err) {
		return ledger.RoutineRecord{}, err
	}
	return record, nil
}

func scanPostgresRoutineRevision(source row) (conversation.RoutineRevision, error) {
	var revision conversation.RoutineRevision
	var trigger, schedule, timezone string
	var next sql.NullTime
	var inputJSON, freshnessJSON, approvalJSON, retryJSON, catchJSON, latenessJSON, compatibilityJSON string
	if err := source.scan(&revision.ID, &revision.RoutineID, &revision.Revision, &revision.AgentID,
		&revision.BehaviorRevisionID, &revision.BindingRevisionID, &trigger, &schedule, &timezone,
		&next, &inputJSON, &freshnessJSON, &revision.ExpectedResult, &revision.ResultConversationID,
		&approvalJSON, &revision.MissingInputBehavior, &retryJSON, &catchJSON, &latenessJSON,
		&compatibilityJSON, &revision.CreatedAt); err != nil {
		return conversation.RoutineRevision{}, err
	}
	var err error
	revision.Trigger, revision.Schedule, revision.Timezone, err = domainRoutineTrigger(trigger, schedule, timezone)
	if err != nil {
		return conversation.RoutineRevision{}, err
	}
	if next.Valid {
		revision.NextOccurrence = next.Time.UTC()
	}
	if revision.InputSource, err = decodeRoutineValue(inputJSON); err != nil {
		return conversation.RoutineRevision{}, err
	}
	var freshness routineFreshness
	if err := json.Unmarshal([]byte(freshnessJSON), &freshness); err != nil || freshness.Seconds <= 0 {
		return conversation.RoutineRevision{}, fmt.Errorf("decode Routine freshness policy")
	}
	revision.FreshnessSeconds = freshness.Seconds
	if revision.ApprovalBoundary, err = decodeRoutineValue(approvalJSON); err != nil {
		return conversation.RoutineRevision{}, err
	}
	if revision.RetryPolicy, err = decodeRoutineValue(retryJSON); err != nil {
		return conversation.RoutineRevision{}, err
	}
	if revision.CatchUpPolicy, err = decodeRoutineValue(catchJSON); err != nil {
		return conversation.RoutineRevision{}, err
	}
	if revision.LatenessPolicy, err = decodeRoutineValue(latenessJSON); err != nil {
		return conversation.RoutineRevision{}, err
	}
	var compatibility routineBindingCompatibility
	if err := json.Unmarshal([]byte(compatibilityJSON), &compatibility); err != nil ||
		compatibility.BehaviorRevisionID != revision.BehaviorRevisionID ||
		compatibility.BindingRevisionID != revision.BindingRevisionID {
		return conversation.RoutineRevision{}, fmt.Errorf("Routine Binding compatibility does not match exact pins")
	}
	revision.Authority = conversation.RoutineAuthorityFortCloud
	revision.CreatedAt = revision.CreatedAt.UTC()
	return revision, nil
}

func scanPostgresRoutineImportReceipt(source row) (ledger.RoutineImportReceipt, error) {
	var receipt ledger.RoutineImportReceipt
	var last, next sql.NullTime
	if err := source.scan(&receipt.ID, &receipt.AccountID, &receipt.SourceRoutineProjectionID,
		&receipt.RoutineID, &receipt.RoutineRevisionID, &receipt.SourceDisabledAt, &last, &next,
		&receipt.FencingReceiptCiphertext, &receipt.FencingReceiptKeyID, &receipt.FencingReceiptNonce,
		&receipt.FencingReceiptDigest, &receipt.ImportedAt); err != nil {
		return ledger.RoutineImportReceipt{}, err
	}
	if last.Valid {
		receipt.ExactLastSourceOccurrenceAt = last.Time.UTC()
	}
	if next.Valid {
		receipt.ExactNextSourceOccurrenceAt = next.Time.UTC()
	}
	receipt.SourceDisabledAt = receipt.SourceDisabledAt.UTC()
	receipt.ImportedAt = receipt.ImportedAt.UTC()
	return receipt, nil
}

func (store *Store) RevalidateRoutine(ctx context.Context, command ledger.RevalidateRoutineCommand) (ledger.RoutineRecord, error) {
	accountID, err := store.operationAccount(command.AccountID)
	if err != nil {
		return ledger.RoutineRecord{}, err
	}
	command.AccountID = accountID
	digest, err := command.Digest()
	if err != nil {
		return ledger.RoutineRecord{}, err
	}
	var record ledger.RoutineRecord
	err = store.withTransaction(ctx, accountID, func(tx transaction) error {
		claimed, err := claimLifecycleIdempotency(ctx, tx, accountID, "routine.revalidate:"+command.RoutineID,
			command.IdempotencyKey, digest, "routine_revision", command.Revision.ID, command.Revision.CreatedAt)
		if err != nil {
			return err
		}
		if !claimed {
			record, err = getPostgresRoutine(ctx, tx, accountID, command.RoutineID)
			return err
		}
		record, err = getPostgresRoutine(ctx, tx, accountID, command.RoutineID)
		if err != nil {
			return err
		}
		if err := command.Validate(record); err != nil {
			return fmt.Errorf("%w: %v", ledger.ErrStateConflict, err)
		}
		create := ledger.CreateRoutineCommand{Routine: record.Routine, Revision: command.Revision}
		create.Routine.CurrentRevisionID = command.Revision.ID
		create.Routine.State = conversation.RoutineActive
		if err := validatePostgresRoutineParents(ctx, tx, accountID, create); err != nil {
			return err
		}
		if err := insertPostgresRoutineRevision(ctx, tx, accountID, command.Revision, digest,
			record.CurrentRevision.ID); err != nil {
			return err
		}
		affected, err := tx.exec(ctx, `update fort_private.routine
set current_revision_id=$1,state='active',updated_at=$2
where account_id=$3 and routine_id=$4 and agent_id=$5 and state='paused_needs_revalidation'`,
			command.Revision.ID, command.Revision.CreatedAt.UTC(), accountID, command.RoutineID, record.Routine.AgentID)
		if err != nil {
			return err
		}
		if affected != 1 {
			return fmt.Errorf("%w: Routine is not paused for revalidation", ledger.ErrStateConflict)
		}
		if err := appendRoutineEvent(ctx, tx, accountID, command.RoutineID, "routine.revalidated",
			map[string]any{"previous_revision_id": record.CurrentRevision.ID,
				"routine_revision_id": command.Revision.ID}, command.Revision.CreatedAt); err != nil {
			return err
		}
		record, err = getPostgresRoutine(ctx, tx, accountID, command.RoutineID)
		return err
	})
	return record, err
}

func (store *Store) EnqueueRoutineOccurrence(ctx context.Context, command ledger.EnqueueRoutineOccurrenceCommand) (ledger.RoutineRunRecord, error) {
	return store.enqueueRoutineOccurrence(ctx, command, nil)
}

func (store *Store) enqueueRoutineOccurrence(ctx context.Context, command ledger.EnqueueRoutineOccurrenceCommand,
	existingTransaction transaction) (ledger.RoutineRunRecord, error) {
	accountID, err := store.operationAccount(command.AccountID)
	if err != nil {
		return ledger.RoutineRunRecord{}, err
	}
	command.AccountID = accountID
	if err := command.Validate(); err != nil {
		return ledger.RoutineRunRecord{}, err
	}
	digest, err := command.Digest()
	if err != nil {
		return ledger.RoutineRunRecord{}, err
	}
	cipher, err := store.collaborationBodies()
	if err != nil {
		return ledger.RoutineRunRecord{}, err
	}
	var record ledger.RoutineRunRecord
	var needsRevalidation bool
	operation := func(tx transaction) error {
		scope := "routine.enqueue:" + command.RoutineID
		var existingDigest, existingKind, existingID string
		err := tx.queryRow(ctx, `select command_digest, result_kind, result_id
from fort_private.idempotency_record
where account_id=$1 and scope=$2 and idempotency_key=$3`, accountID, scope,
			command.IdempotencyKey).scan(&existingDigest, &existingKind, &existingID)
		if err == nil {
			if existingDigest != digest || existingKind != "routine_run" || existingID != command.RunID {
				return fmt.Errorf("%w: %q", ledger.ErrIdempotencyConflict, command.IdempotencyKey)
			}
			record, err = getPostgresRoutineRun(ctx, tx, cipher, accountID, command.RunID)
			return err
		}
		if !isNoRows(err) {
			return err
		}
		routine, err := getPostgresRoutine(ctx, tx, accountID, command.RoutineID)
		if err != nil {
			return err
		}
		if routine.PauseReason == ledger.RoutinePauseNeedsRevalidation {
			needsRevalidation = true
			return nil
		}
		if routine.Routine.State != conversation.RoutineActive {
			return fmt.Errorf("%w: Routine is not active", ledger.ErrStateConflict)
		}
		if routine.CurrentRevision.ID != command.RoutineRevisionID {
			return fmt.Errorf("%w: Routine occurrence must pin the current Revision", ledger.ErrRevisionConflict)
		}
		parent, err := loadPostgresAgentDirectParent(ctx, tx, accountID, routine.Routine.AgentID,
			routine.CurrentRevision.ResultConversationID)
		if err != nil {
			return err
		}
		if parent.behaviorRevisionID != routine.CurrentRevision.BehaviorRevisionID ||
			parent.bindingRevisionID != routine.CurrentRevision.BindingRevisionID {
			if _, err := tx.exec(ctx, `update fort_private.routine
set state='paused_needs_revalidation',updated_at=$1
where account_id=$2 and routine_id=$3 and state='active'`, command.CreatedAt.UTC(), accountID,
				command.RoutineID); err != nil {
				return err
			}
			needsRevalidation = true
			return nil
		}
		if parent.agentState != string(conversation.AgentOpen) || parent.conversationState != string(conversation.ConversationOpen) {
			return fmt.Errorf("%w: Routine owner and result Conversation must be open", ledger.ErrStateConflict)
		}
		claimed, err := claimLifecycleIdempotency(ctx, tx, accountID, scope, command.IdempotencyKey,
			digest, "routine_run", command.RunID, command.CreatedAt)
		if err != nil {
			return err
		}
		if !claimed {
			record, err = getPostgresRoutineRun(ctx, tx, cipher, accountID, command.RunID)
			return err
		}
		occurrenceExists := false
		existingOccurrenceState := ""
		var existingOccurrenceID, existingRevisionID, existingOccurrenceKey string
		var existingIsTest bool
		err = tx.queryRow(ctx, `select routine_occurrence_id,routine_revision_id,is_test,state,idempotency_key
from fort_private.routine_occurrence
where account_id=$1 and routine_id=$2 and scheduled_for=$3
for update`, accountID, command.RoutineID, command.ScheduledFor.UTC()).scan(
			&existingOccurrenceID, &existingRevisionID, &existingIsTest, &existingOccurrenceState,
			&existingOccurrenceKey)
		if err == nil {
			occurrenceExists = true
			wantIsTest := command.Kind == conversation.RoutineRunTest
			if existingOccurrenceID != command.OccurrenceID || existingRevisionID != command.RoutineRevisionID ||
				existingOccurrenceKey != command.IdempotencyKey || existingIsTest != wantIsTest {
				return fmt.Errorf("%w: Routine occurrence at %s", ledger.ErrIdempotencyConflict,
					command.ScheduledFor.UTC().Format(time.RFC3339Nano))
			}
			if existingOccurrenceState == "scheduled" && command.CreatedAt.Before(command.ScheduledFor) {
				return fmt.Errorf("%w: Routine occurrence is not due", ledger.ErrStateConflict)
			}
			var existingRunID string
			err = tx.queryRow(ctx, `select routine_run_id from fort_private.routine_run
where account_id=$1 and routine_occurrence_id=$2`, accountID, command.OccurrenceID).scan(&existingRunID)
			if err == nil {
				if existingRunID != command.RunID {
					return fmt.Errorf("%w: Routine occurrence already belongs to another run", ledger.ErrIdempotencyConflict)
				}
				record, err = getPostgresRoutineRun(ctx, tx, cipher, accountID, command.RunID)
				return err
			}
			if !isNoRows(err) {
				return err
			}
		} else if !isNoRows(err) {
			return err
		}
		decision, err := evaluatePostgresRoutineStart(ctx, tx, cipher, accountID, routine, command,
			existingOccurrenceState)
		if err != nil {
			return err
		}
		participantID, err := ensurePostgresAgentDirectParticipant(ctx, tx, accountID,
			ledger.SendAgentTurnCommand{AgentID: routine.Routine.AgentID,
				ConversationID: routine.CurrentRevision.ResultConversationID, CreatedAt: command.CreatedAt}, parent)
		if err != nil {
			return err
		}
		turnID, targetID := routineTurnID(command.RunID), routineTargetID(command.RunID)
		manifestID, grantID := routineManifestID(command.RunID), routineGrantID(command.RunID)
		promptText := routinePrompt(routine.CurrentRevision, decision.Input)
		body, err := cipher.seal(securebody.Scope{AccountID: accountID,
			RecordType: "conversation_message", RecordID: turnID}, promptText)
		if err != nil {
			return fmt.Errorf("encrypt Routine prompt: %w", err)
		}
		var messageID int64
		if err := tx.queryRow(ctx, `insert into fort_private.conversation_message (
  account_id,conversation_id,turn_id,target_id,handoff_id,routine_run_id,message_kind,
  author_kind,author_id,author_agent_id,body_ciphertext,body_key_id,body_nonce,body_digest,
  body_plaintext_length,created_at
) values ($1,$2,$3,null,null,null,'system','system',$4,null,$5,$6,$7,$8,$9,$10)
returning message_id`, accountID, routine.CurrentRevision.ResultConversationID, turnID,
			"routine:"+routine.Routine.ID, body.Ciphertext, body.KeyID, body.Nonce, body.Digest,
			body.PlaintextBytes, command.CreatedAt.UTC()).scan(&messageID); err != nil {
			return fmt.Errorf("insert Routine prompt: %w", err)
		}
		messageIDs := make([]int64, 0, 2)
		if decision.SourceMessageID != 0 {
			messageIDs = append(messageIDs, decision.SourceMessageID)
		}
		messageIDs = append(messageIDs, messageID)
		manifestDigest, err := postgresRoutineManifestDigest(routine.CurrentRevision.InputSource,
			messageID, messageIDs)
		if err != nil {
			return err
		}
		if _, err := tx.exec(ctx, `insert into fort_private.context_manifest (
  account_id,context_manifest_id,purpose,manifest_digest,created_by,created_at
) values ($1,$2,'routine',$3,$4,$5)`, accountID, manifestID, manifestDigest,
			"routine:"+routine.Routine.ID, command.CreatedAt.UTC()); err != nil {
			return err
		}
		contextIDs := make([]string, 0, len(messageIDs))
		for ordinal, id := range messageIDs {
			if _, err := tx.exec(ctx, `insert into fort_private.context_manifest_message (
  account_id,context_manifest_id,ordinal,message_id
) values ($1,$2,$3,$4)`, accountID, manifestID, ordinal, id); err != nil {
				return err
			}
			contextIDs = append(contextIDs, "message:"+strconv.FormatInt(id, 10))
		}
		grant := conversation.AuthorityGrant{ID: grantID, Permissions: []string{}, ContextRecordIDs: contextIDs}
		grantJSON, _ := json.Marshal(grant)
		grantHash := sha256.Sum256(grantJSON)
		deadline := command.ScheduledFor.UTC().Add(24 * time.Hour)
		if deadline.Before(command.CreatedAt.UTC().Add(time.Hour)) {
			deadline = command.CreatedAt.UTC().Add(time.Hour)
		}
		if _, err := tx.exec(ctx, `insert into fort_private.delegation_grant (
  account_id,delegation_grant_id,source_kind,source_id,authority_grant,grant_digest,
  maximum_agent_messages,maximum_handoff_depth,hard_deadline,created_by,created_at
) values ($1,$2,'routine_occurrence',$3,$4::jsonb,$5,1,0,$6,$7,$8)`, accountID, grantID,
			command.OccurrenceID, string(grantJSON), hex.EncodeToString(grantHash[:]), deadline,
			"routine:"+routine.Routine.ID, command.CreatedAt.UTC()); err != nil {
			return err
		}
		cancellation := `{"kind":"human_or_deadline","revision":"1"}`
		approval, _ := json.Marshal(routineValue{Value: routine.CurrentRevision.ApprovalBoundary})
		if _, err := tx.exec(ctx, `insert into fort_private.conversation_turn (
  account_id,turn_id,conversation_id,client_turn_id,idempotency_key,command_digest,kind,
  prompt_message_id,through_message_id,membership_revision_id,context_manifest_id,delegation_grant_id,
  concurrency_policy,cancellation_policy,approval_policy,maximum_agent_messages,maximum_handoff_depth,
  cost_limit_classification,token_limit_classification,cost_limit,token_limit,hard_deadline,state,created_at,updated_at
		) values ($1,$2,$3,$4,$5,$6,'routine',$7,$7,$8,$9,$10,'serial',$11::jsonb,$12::jsonb,
	  1,0,'unknown','unknown',null,null,$13,$14,$15,$15)`, accountID, turnID,
			routine.CurrentRevision.ResultConversationID, command.OccurrenceID, command.IdempotencyKey,
			digest, messageID, parent.membershipRevisionID, manifestID, grantID, cancellation,
			string(approval), deadline, decision.TurnState, command.CreatedAt.UTC()); err != nil {
			return err
		}
		if _, err := tx.exec(ctx, `insert into fort_private.conversation_target (
  account_id,target_id,turn_id,conversation_id,agent_id,membership_revision_id,target_kind,
		  origin_id,run_id,state,attempt_count,error_code,hard_deadline,cancellation_policy,created_at,updated_at
		) values ($1,$2,$3,$4,$5,$6,'routine',$7,$8,$9,0,$10,$11,$12::jsonb,$13,$13)`, accountID,
			targetID, turnID, routine.CurrentRevision.ResultConversationID, routine.Routine.AgentID,
			parent.membershipRevisionID, command.OccurrenceID, command.RunID, decision.TargetState,
			nullablePostgresString(decision.FailureCode), deadline, cancellation,
			command.CreatedAt.UTC()); err != nil {
			return err
		}
		if _, err := tx.exec(ctx, `insert into fort_private.conversation_target_binding (
  account_id,target_id,conversation_id,agent_id,behavior_revision_id,binding_revision_id,
  participant_id,membership_revision_id,pinned_at
) values ($1,$2,$3,$4,$5,$6,$7,$8,$9)`, accountID, targetID,
			routine.CurrentRevision.ResultConversationID, routine.Routine.AgentID,
			routine.CurrentRevision.BehaviorRevisionID, routine.CurrentRevision.BindingRevisionID,
			participantID, parent.membershipRevisionID, command.CreatedAt.UTC()); err != nil {
			return err
		}
		if !occurrenceExists {
			if _, err := tx.exec(ctx, `insert into fort_private.routine_occurrence (
  account_id,routine_occurrence_id,routine_id,routine_revision_id,scheduled_for,is_test,state,
  idempotency_key,created_at,updated_at
) values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$9)`, accountID, command.OccurrenceID,
				command.RoutineID, command.RoutineRevisionID, command.ScheduledFor.UTC(),
				command.Kind == conversation.RoutineRunTest, decision.OccurrenceState, command.IdempotencyKey,
				command.CreatedAt.UTC()); err != nil {
				return translateRoutineWriteError(err, "Routine occurrence")
			}
		} else if affected, err := tx.exec(ctx, `update fort_private.routine_occurrence
set state=$1,updated_at=$2
where account_id=$3 and routine_occurrence_id=$4 and state in ('scheduled','queued','missed_needs_attention')`,
			decision.OccurrenceState, command.CreatedAt.UTC(), accountID, command.OccurrenceID); err != nil || affected != 1 {
			return changedRowsError("adopt Routine occurrence", affected, err)
		}
		var terminalAt any
		if decision.RunState == conversation.RoutineRunFailed {
			terminalAt = command.CreatedAt.UTC()
		}
		nextAction, _ := json.Marshal(routineValue{Value: decision.NextAction})
		var nextActionValue any
		if decision.NextAction != "" {
			nextActionValue = string(nextAction)
		}
		if _, err := tx.exec(ctx, `insert into fort_private.routine_run (
  account_id,routine_run_id,routine_occurrence_id,routine_id,routine_revision_id,
  behavior_revision_id,binding_revision_id,target_id,result_conversation_id,state,failure_code,
  next_action,terminal_at,created_at,updated_at
) values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12::jsonb,$13,$14,$14)`, accountID, command.RunID,
			command.OccurrenceID, command.RoutineID, command.RoutineRevisionID,
			routine.CurrentRevision.BehaviorRevisionID, routine.CurrentRevision.BindingRevisionID,
			targetID, routine.CurrentRevision.ResultConversationID, decision.RunState,
			nullablePostgresString(decision.FailureCode), nextActionValue, terminalAt, command.CreatedAt.UTC()); err != nil {
			return translateRoutineWriteError(err, "Routine run")
		}
		metadata := routineActivityMetadata{State: decision.RunState, Activity: decision.Activity,
			FailureCode: decision.FailureCode, NextAction: decision.NextAction,
			ApprovalEvidenceID: command.ApprovalEvidenceID}
		if err := appendRoutineRunActivity(ctx, tx, accountID, command.RunID, targetID,
			"routine.run.created", metadata, command.CreatedAt); err != nil {
			return err
		}
		record, err = getPostgresRoutineRun(ctx, tx, cipher, accountID, command.RunID)
		return err
	}
	if existingTransaction != nil {
		err = operation(existingTransaction)
	} else {
		err = store.withTransaction(ctx, accountID, operation)
	}
	if err != nil {
		return ledger.RoutineRunRecord{}, err
	}
	if needsRevalidation {
		return ledger.RoutineRunRecord{}, ledger.ErrRoutineNeedsRevalidation
	}
	return record, nil
}

func (store *Store) AdvanceRoutineRun(ctx context.Context, command ledger.AdvanceRoutineRunCommand) (ledger.RoutineRunRecord, error) {
	accountID, err := store.operationAccount(command.AccountID)
	if err != nil {
		return ledger.RoutineRunRecord{}, err
	}
	command.AccountID = accountID
	if err := command.Validate(); err != nil {
		return ledger.RoutineRunRecord{}, err
	}
	digest, err := command.Digest()
	if err != nil {
		return ledger.RoutineRunRecord{}, err
	}
	cipher, err := store.collaborationBodies()
	if err != nil {
		return ledger.RoutineRunRecord{}, err
	}
	var record ledger.RoutineRunRecord
	err = store.withTransaction(ctx, accountID, func(tx transaction) error {
		claimed, err := claimLifecycleIdempotency(ctx, tx, accountID, "routine.run.advance:"+command.RunID,
			command.IdempotencyKey, digest, "routine_run", command.RunID, command.OccurredAt)
		if err != nil {
			return err
		}
		if !claimed {
			record, err = getPostgresRoutineRun(ctx, tx, cipher, accountID, command.RunID)
			return err
		}
		record, err = getPostgresRoutineRun(ctx, tx, cipher, accountID, command.RunID)
		if err != nil {
			return err
		}
		if routineRunTerminal(record.Run.State) {
			return ledger.ErrRoutineRunTerminal
		}
		if record.Run.State != command.FromState {
			return fmt.Errorf("%w: Routine run state is %q, not %q", ledger.ErrStateConflict,
				record.Run.State, command.FromState)
		}
		if command.FromState == conversation.RoutineRunWorking &&
			(record.AttemptID != command.AttemptID || record.LeaseID != command.LeaseID ||
				!record.LeaseExpiresAt.Equal(command.LeaseExpiresAt.UTC())) {
			return fmt.Errorf("%w: Routine run lease does not match the exact working attempt", ledger.ErrStateConflict)
		}
		if command.ToState == conversation.RoutineRunWorking {
			var leaseState string
			var expiresAt time.Time
			err := tx.queryRow(ctx, `select lease.state,lease.expires_at
from fort_private.routine_run as run
join fort_private.conversation_target as target on target.account_id=run.account_id and target.target_id=run.target_id
join fort_private.execution_attempt as attempt on attempt.account_id=target.account_id and attempt.target_id=target.target_id
join fort_private.worker_lease as lease on lease.account_id=attempt.account_id and lease.execution_attempt_id=attempt.execution_attempt_id
where run.account_id=$1 and run.routine_run_id=$2 and attempt.execution_attempt_id=$3 and lease.lease_id=$4`,
				accountID, command.RunID, command.AttemptID, command.LeaseID).scan(&leaseState, &expiresAt)
			if isNoRows(err) || err == nil && (leaseState != "active" || !expiresAt.Equal(command.LeaseExpiresAt.UTC())) {
				return fmt.Errorf("%w: Routine working transition requires the exact active lease", ledger.ErrStateConflict)
			}
			if err != nil {
				return err
			}
		}
		var terminalAt any
		if routineRunTerminal(command.ToState) {
			terminalAt = command.OccurredAt.UTC()
		}
		nextAction, _ := json.Marshal(routineValue{Value: command.NextAction})
		var nextActionValue any
		if command.NextAction != "" {
			nextActionValue = string(nextAction)
		}
		if command.ToState == conversation.RoutineRunSucceeded {
			body, err := cipher.seal(securebody.Scope{AccountID: accountID,
				RecordType: "routine_result", RecordID: command.RunID}, command.NormalizedResult)
			if err != nil {
				return fmt.Errorf("encrypt Routine result: %w", err)
			}
			var messageID int64
			if err := tx.queryRow(ctx, `insert into fort_private.conversation_message (
  account_id,conversation_id,turn_id,target_id,handoff_id,routine_run_id,message_kind,
  author_kind,author_id,author_agent_id,body_ciphertext,body_key_id,body_nonce,body_digest,
  body_plaintext_length,created_at
) values ($1,$2,$3,$4,null,$5,'routine_result','agent',$6,$6,$7,$8,$9,$10,$11,$12)
returning message_id`, accountID, record.ResultConversationID, routineTurnID(command.RunID),
				routineTargetID(command.RunID), command.RunID, record.Run.AgentID, body.Ciphertext,
				body.KeyID, body.Nonce, body.Digest, body.PlaintextBytes, command.OccurredAt.UTC()).scan(&messageID); err != nil {
				return translateRoutineWriteError(err, "Routine result")
			}
		}
		var executionAttemptID any
		if command.ToState == conversation.RoutineRunWorking {
			executionAttemptID = command.AttemptID
		}
		affected, err := tx.exec(ctx, `update fort_private.routine_run
set state=$1,execution_attempt_id=coalesce(execution_attempt_id,$2),failure_code=$3,
    next_action=$4::jsonb,terminal_at=$5,updated_at=$6
where account_id=$7 and routine_run_id=$8 and state=$9
  and ($2::text is null or execution_attempt_id is null or execution_attempt_id=$2)`, command.ToState,
			executionAttemptID, nullablePostgresString(command.FailureCode), nextActionValue, terminalAt,
			command.OccurredAt.UTC(), accountID, command.RunID, command.FromState)
		if err != nil {
			return translateRoutineWriteError(err, "advance Routine run")
		}
		if affected != 1 {
			return fmt.Errorf("%w: Routine run state changed concurrently", ledger.ErrStateConflict)
		}
		occurrenceState := postgresRoutineOccurrenceState(command.ToState)
		if affected, err := tx.exec(ctx, `update fort_private.routine_occurrence
set state=$1,updated_at=$2 where account_id=$3 and routine_occurrence_id=$4`, occurrenceState,
			command.OccurredAt.UTC(), accountID, record.Occurrence.ID); err != nil || affected != 1 {
			return changedRowsError("advance Routine occurrence", affected, err)
		}
		metadata := routineActivityMetadata{State: command.ToState, AttemptID: command.AttemptID,
			LeaseID: command.LeaseID, LeaseExpiresAt: command.LeaseExpiresAt.UTC(), Activity: command.Activity,
			FailureCode: command.FailureCode, NextAction: command.NextAction}
		if err := appendRoutineRunActivity(ctx, tx, accountID, command.RunID, routineTargetID(command.RunID),
			"routine.run.advanced", metadata, command.OccurredAt); err != nil {
			return err
		}
		if command.ToState == conversation.RoutineRunSucceeded {
			if _, err := tx.exec(ctx, `update fort_private.conversation set updated_at=$1
where account_id=$2 and conversation_id=$3`, command.OccurredAt.UTC(), accountID,
				record.ResultConversationID); err != nil {
				return err
			}
		}
		record, err = getPostgresRoutineRun(ctx, tx, cipher, accountID, command.RunID)
		return err
	})
	return record, err
}

func (store *Store) GetRoutineRun(ctx context.Context, accountID, runID string) (ledger.RoutineRunRecord, error) {
	accountID, err := store.operationAccount(accountID)
	if err != nil {
		return ledger.RoutineRunRecord{}, err
	}
	cipher, err := store.collaborationBodies()
	if err != nil {
		return ledger.RoutineRunRecord{}, err
	}
	var record ledger.RoutineRunRecord
	err = store.withTransaction(ctx, accountID, func(tx transaction) error {
		record, err = getPostgresRoutineRun(ctx, tx, cipher, accountID, runID)
		return err
	})
	return record, err
}

func (store *Store) ListRoutineRuns(ctx context.Context, accountID, routineID string) ([]ledger.RoutineRunRecord, error) {
	accountID, err := store.operationAccount(accountID)
	if err != nil {
		return nil, err
	}
	cipher, err := store.collaborationBodies()
	if err != nil {
		return nil, err
	}
	items := make([]ledger.RoutineRunRecord, 0)
	err = store.withTransaction(ctx, accountID, func(tx transaction) error {
		if _, err := getPostgresRoutine(ctx, tx, accountID, routineID); err != nil {
			return err
		}
		rows, err := tx.query(ctx, `select routine_run_id from fort_private.routine_run
where account_id=$1 and routine_id=$2 order by created_at desc,routine_run_id desc`, accountID, routineID)
		if err != nil {
			return err
		}
		defer rows.close()
		ids := make([]string, 0)
		for rows.next() {
			var id string
			if err := rows.scan(&id); err != nil {
				return err
			}
			ids = append(ids, id)
		}
		if err := rows.errResult(); err != nil {
			return err
		}
		for _, id := range ids {
			item, err := getPostgresRoutineRun(ctx, tx, cipher, accountID, id)
			if err != nil {
				return err
			}
			items = append(items, item)
		}
		return nil
	})
	return items, err
}

func getPostgresRoutineRun(ctx context.Context, tx transaction, cipher collaborationBodyCipher,
	accountID, runID string) (ledger.RoutineRunRecord, error) {
	var record ledger.RoutineRunRecord
	var isTest bool
	var occurrenceState string
	var failureCode sql.NullString
	var nextActionJSON sql.NullString
	var messageID sql.NullInt64
	var encryptedCiphertext, encryptedNonce []byte
	var encryptedKeyID, encryptedDigest sql.NullString
	var encryptedLength sql.NullInt64
	err := tx.queryRow(ctx, `select run.routine_run_id,run.routine_occurrence_id,run.routine_id,
  run.routine_revision_id,routine.agent_id,run.behavior_revision_id,run.binding_revision_id,
  run.result_conversation_id,run.state,run.failure_code,run.next_action::text,run.created_at,
  occurrence.routine_occurrence_id,occurrence.account_id::text,occurrence.routine_id,
  occurrence.routine_revision_id,occurrence.is_test,occurrence.state,occurrence.scheduled_for,
  occurrence.idempotency_key,occurrence.created_at,occurrence.updated_at,
  message.message_id,message.body_ciphertext,message.body_key_id,message.body_nonce,
  message.body_digest,message.body_plaintext_length
from fort_private.routine_run as run
join fort_private.routine as routine on routine.account_id=run.account_id and routine.routine_id=run.routine_id
join fort_private.routine_occurrence as occurrence
  on occurrence.account_id=run.account_id and occurrence.routine_occurrence_id=run.routine_occurrence_id
left join fort_private.conversation_message as message
  on message.account_id=run.account_id and message.routine_run_id=run.routine_run_id
 and message.message_kind='routine_result'
where run.account_id=$1 and run.routine_run_id=$2`, accountID, runID).scan(&record.Run.ID,
		&record.Run.OccurrenceID, &record.Run.RoutineID, &record.Run.RoutineRevisionID,
		&record.Run.AgentID, &record.Run.BehaviorRevisionID, &record.Run.BindingRevisionID,
		&record.ResultConversationID, &record.Run.State, &failureCode, &nextActionJSON,
		&record.Run.CreatedAt, &record.Occurrence.ID, &record.Occurrence.AccountID,
		&record.Occurrence.RoutineID, &record.Occurrence.RoutineRevisionID, &isTest,
		&occurrenceState, &record.Occurrence.ScheduledFor, &record.Occurrence.IdempotencyKey,
		&record.Occurrence.CreatedAt, &record.Occurrence.UpdatedAt, &messageID, &encryptedCiphertext,
		&encryptedKeyID, &encryptedNonce, &encryptedDigest, &encryptedLength)
	if isNoRows(err) {
		return ledger.RoutineRunRecord{}, fmt.Errorf("%w: Routine run %q", ledger.ErrNotFound, runID)
	}
	if err != nil {
		return ledger.RoutineRunRecord{}, err
	}
	if isTest {
		record.Run.Kind = conversation.RoutineRunTest
		record.Occurrence.Kind = conversation.RoutineRunTest
	} else {
		record.Run.Kind = conversation.RoutineRunScheduled
		record.Occurrence.Kind = conversation.RoutineRunScheduled
	}
	record.Occurrence.State = domainRoutineOccurrenceState(occurrenceState)
	record.Occurrence.ApprovalEvidenceID = ""
	record.FailureCode = failureCode.String
	if nextActionJSON.Valid {
		record.NextAction, err = decodeRoutineValue(nextActionJSON.String)
		if err != nil {
			return ledger.RoutineRunRecord{}, err
		}
	}
	if messageID.Valid {
		if cipher == nil || !encryptedKeyID.Valid || !encryptedDigest.Valid || !encryptedLength.Valid {
			return ledger.RoutineRunRecord{}, errCollaborationKeyRingRequired
		}
		record.Run.NormalizedResult, err = cipher.open(securebody.Scope{AccountID: accountID,
			RecordType: "routine_result", RecordID: runID}, collaborationEncryptedBody{
			Ciphertext: encryptedCiphertext, KeyID: encryptedKeyID.String, Nonce: encryptedNonce,
			Digest: encryptedDigest.String, PlaintextBytes: int(encryptedLength.Int64),
		})
		if err != nil {
			return ledger.RoutineRunRecord{}, fmt.Errorf("decrypt Routine result: %w", err)
		}
		record.Run.ResultMessageID = strconv.FormatInt(messageID.Int64, 10)
	}
	record.Run.CreatedAt = record.Run.CreatedAt.UTC()
	record.Occurrence.ScheduledFor = record.Occurrence.ScheduledFor.UTC()
	record.Occurrence.CreatedAt = record.Occurrence.CreatedAt.UTC()
	record.Occurrence.UpdatedAt = record.Occurrence.UpdatedAt.UTC()
	record.Activities = make([]ledger.RoutineRunActivity, 0)
	activities, err := tx.query(ctx, `select event_id,event_metadata::text,created_at
from fort_private.ledger_event
where account_id=$1 and aggregate_kind='routine_run' and aggregate_id=$2
  and event_type in ('routine.run.created','routine.run.queued','routine.run.advanced')
order by event_id`, accountID, runID)
	if err != nil {
		return ledger.RoutineRunRecord{}, err
	}
	defer activities.close()
	for activities.next() {
		var sequence int64
		var metadataJSON string
		var createdAt time.Time
		if err := activities.scan(&sequence, &metadataJSON, &createdAt); err != nil {
			return ledger.RoutineRunRecord{}, err
		}
		var metadata routineActivityMetadata
		if err := json.Unmarshal([]byte(metadataJSON), &metadata); err != nil {
			return ledger.RoutineRunRecord{}, fmt.Errorf("decode Routine run activity: %w", err)
		}
		activity := ledger.RoutineRunActivity{Sequence: sequence, State: metadata.State,
			AttemptID: metadata.AttemptID, LeaseID: metadata.LeaseID,
			LeaseExpiresAt: metadata.LeaseExpiresAt.UTC(), Activity: metadata.Activity,
			FailureCode: metadata.FailureCode, NextAction: metadata.NextAction, CreatedAt: createdAt.UTC()}
		record.Activities = append(record.Activities, activity)
		if metadata.ApprovalEvidenceID != "" {
			record.Occurrence.ApprovalEvidenceID = metadata.ApprovalEvidenceID
		}
		if metadata.AttemptID != "" {
			record.AttemptID = metadata.AttemptID
		}
		if metadata.LeaseID != "" {
			record.LeaseID = metadata.LeaseID
		}
		if !metadata.LeaseExpiresAt.IsZero() {
			record.LeaseExpiresAt = metadata.LeaseExpiresAt.UTC()
		}
	}
	if err := activities.errResult(); err != nil {
		return ledger.RoutineRunRecord{}, err
	}
	return record, nil
}

func appendRoutineRunActivity(ctx context.Context, tx transaction, accountID, runID, targetID,
	eventType string, metadata routineActivityMetadata, createdAt time.Time) error {
	payload, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	_, err = tx.exec(ctx, `insert into fort_private.ledger_event (
  account_id,aggregate_kind,aggregate_id,event_type,target_id,event_metadata,created_at
) values ($1,'routine_run',$2,$3,$4,$5::jsonb,$6)`, accountID, runID, eventType,
		targetID, string(payload), createdAt.UTC())
	return err
}

// markPostgresRoutineRunWorking projects an already-validated worker lease
// heartbeat into its Routine aggregate without opening a nested transaction.
// A target not owned by a Routine is intentionally a no-op so the generic
// worker path can call this helper for every target kind. Routine targets fail
// closed unless the exact active attempt, lease, and persisted expiry match.
func markPostgresRoutineRunWorking(ctx context.Context, tx transaction, accountID, targetID, attemptID,
	leaseID string, exactLeaseExpiresAt, observedAt time.Time) error {
	if tx == nil || strings.TrimSpace(accountID) == "" || strings.TrimSpace(targetID) == "" ||
		strings.TrimSpace(attemptID) == "" || strings.TrimSpace(leaseID) == "" ||
		exactLeaseExpiresAt.IsZero() || observedAt.IsZero() || !exactLeaseExpiresAt.After(observedAt) {
		return fmt.Errorf("%w: Routine working transition scope is invalid", controlapi.ErrWorkerStaleLease)
	}

	var runID, occurrenceID, runState, occurrenceState string
	var runAttemptID, exactAttemptID, exactLeaseID, leaseState sql.NullString
	var persistedExpiry sql.NullTime
	err := tx.queryRow(ctx, `select run.routine_run_id,run.routine_occurrence_id,run.state,occurrence.state,
  run.execution_attempt_id,attempt.execution_attempt_id,lease.lease_id,lease.state,lease.expires_at
from fort_private.routine_run as run
join fort_private.routine_occurrence as occurrence
  on occurrence.account_id=run.account_id and occurrence.routine_occurrence_id=run.routine_occurrence_id
left join fort_private.execution_attempt as attempt
  on attempt.account_id=run.account_id and attempt.target_id=run.target_id
 and attempt.execution_attempt_id=$3
left join fort_private.worker_lease as lease
  on lease.account_id=attempt.account_id and lease.execution_attempt_id=attempt.execution_attempt_id
 and lease.target_id=run.target_id and lease.lease_id=$4
where run.account_id=$1 and run.target_id=$2
for update of run,occurrence`, accountID, targetID, attemptID, leaseID).scan(
		&runID, &occurrenceID, &runState, &occurrenceState, &runAttemptID, &exactAttemptID,
		&exactLeaseID, &leaseState, &persistedExpiry)
	if isNoRows(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !exactAttemptID.Valid || exactAttemptID.String != attemptID || !exactLeaseID.Valid ||
		exactLeaseID.String != leaseID || !leaseState.Valid || leaseState.String != "active" ||
		!persistedExpiry.Valid || !persistedExpiry.Time.Equal(exactLeaseExpiresAt.UTC()) {
		return fmt.Errorf("%w: Routine working transition requires the exact active lease", controlapi.ErrWorkerStaleLease)
	}
	if runState == string(conversation.RoutineRunWorking) && occurrenceState == "working" {
		if !runAttemptID.Valid || runAttemptID.String != attemptID {
			return fmt.Errorf("%w: Routine run is owned by another execution attempt", controlapi.ErrWorkerStaleLease)
		}
		return nil
	}
	if runState != string(conversation.RoutineRunQueued) || occurrenceState != "queued" ||
		(runAttemptID.Valid && runAttemptID.String != attemptID) {
		return fmt.Errorf("%w: Routine run cannot enter working from its persisted state", controlapi.ErrWorkerStaleLease)
	}

	if affected, err := tx.exec(ctx, `update fort_private.routine_run
set state='working',execution_attempt_id=$1,updated_at=$2
where account_id=$3 and routine_run_id=$4 and target_id=$5 and state='queued'
  and (execution_attempt_id is null or execution_attempt_id=$1)`, attemptID, observedAt.UTC(), accountID,
		runID, targetID); err != nil || affected != 1 {
		return changedRowsError("mark Routine run working", affected, err)
	}
	if affected, err := tx.exec(ctx, `update fort_private.routine_occurrence
set state='working',updated_at=$1
where account_id=$2 and routine_occurrence_id=$3 and state='queued'`, observedAt.UTC(), accountID,
		occurrenceID); err != nil || affected != 1 {
		return changedRowsError("mark Routine occurrence working", affected, err)
	}
	return appendRoutineRunActivity(ctx, tx, accountID, runID, targetID, "routine.run.advanced",
		routineActivityMetadata{State: conversation.RoutineRunWorking, AttemptID: attemptID, LeaseID: leaseID,
			LeaseExpiresAt: exactLeaseExpiresAt.UTC(), Activity: "worker execution started"}, observedAt)
}

// commitPostgresRoutineRunTerminal projects an already-validated worker
// terminal receipt into its Routine aggregate in the caller's transaction.
// outputBody is normalized renderable content: inline output for bounded
// results, or the canonical Fort artifact-reference JSON for large results.
func (store *Store) commitPostgresRoutineRunTerminal(ctx context.Context, tx transaction, accountID, targetID,
	attemptID, leaseID string, status coreworker.TerminalStatus, outputBody string, committedAt time.Time) error {
	if tx == nil || strings.TrimSpace(accountID) == "" || strings.TrimSpace(targetID) == "" ||
		strings.TrimSpace(attemptID) == "" || strings.TrimSpace(leaseID) == "" || committedAt.IsZero() {
		return fmt.Errorf("%w: Routine terminal transition scope is invalid", controlapi.ErrWorkerStaleLease)
	}
	var desiredState conversation.RoutineRunState
	var failureCode, activity, turnState string
	switch status {
	case coreworker.TerminalCompleted:
		if strings.TrimSpace(outputBody) == "" {
			return fmt.Errorf("%w: successful Routine terminal requires normalized output", controlapi.ErrWorkerRequestInvalid)
		}
		desiredState, activity, turnState = conversation.RoutineRunSucceeded, "worker completed Routine run", "settled"
	case coreworker.TerminalFailed:
		if outputBody != "" {
			return fmt.Errorf("%w: failed Routine terminal cannot persist a result", controlapi.ErrWorkerRequestInvalid)
		}
		desiredState, failureCode, activity, turnState = conversation.RoutineRunFailed, "worker_failed",
			"worker failed Routine run", "needs_you"
	case coreworker.TerminalCanceled:
		if outputBody != "" {
			return fmt.Errorf("%w: canceled Routine terminal cannot persist a result", controlapi.ErrWorkerRequestInvalid)
		}
		desiredState, failureCode, activity, turnState = conversation.RoutineRunCanceled, "worker_canceled",
			"worker canceled Routine run", "canceled"
	default:
		return fmt.Errorf("%w: unsupported Routine worker terminal status", controlapi.ErrWorkerRequestInvalid)
	}

	var runID, occurrenceID, resultConversationID, agentID, runState, occurrenceState string
	var runAttemptID, exactAttemptID, exactLeaseID, leaseState sql.NullString
	var resultCount int64
	err := tx.queryRow(ctx, `select run.routine_run_id,run.routine_occurrence_id,run.result_conversation_id,
  routine.agent_id,run.state,occurrence.state,run.execution_attempt_id,
  attempt.execution_attempt_id,lease.lease_id,lease.state,
  (select count(*) from fort_private.conversation_message as result
    where result.account_id=run.account_id and result.routine_run_id=run.routine_run_id
      and result.message_kind='routine_result')
from fort_private.routine_run as run
join fort_private.routine as routine
  on routine.account_id=run.account_id and routine.routine_id=run.routine_id
join fort_private.routine_occurrence as occurrence
  on occurrence.account_id=run.account_id and occurrence.routine_occurrence_id=run.routine_occurrence_id
left join fort_private.execution_attempt as attempt
  on attempt.account_id=run.account_id and attempt.target_id=run.target_id
 and attempt.execution_attempt_id=$3
left join fort_private.worker_lease as lease
  on lease.account_id=attempt.account_id and lease.execution_attempt_id=attempt.execution_attempt_id
 and lease.target_id=run.target_id and lease.lease_id=$4
where run.account_id=$1 and run.target_id=$2
for update of run,occurrence`, accountID, targetID, attemptID, leaseID).scan(&runID, &occurrenceID,
		&resultConversationID, &agentID, &runState, &occurrenceState, &runAttemptID, &exactAttemptID,
		&exactLeaseID, &leaseState, &resultCount)
	if isNoRows(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !exactAttemptID.Valid || exactAttemptID.String != attemptID || !exactLeaseID.Valid ||
		exactLeaseID.String != leaseID || !leaseState.Valid || leaseState.String != "active" {
		return fmt.Errorf("%w: Routine terminal requires the exact active lease", controlapi.ErrWorkerStaleLease)
	}
	if runState == string(desiredState) && occurrenceState == string(desiredState) {
		if !runAttemptID.Valid || runAttemptID.String != attemptID ||
			(desiredState == conversation.RoutineRunSucceeded && resultCount != 1) ||
			(desiredState != conversation.RoutineRunSucceeded && resultCount != 0) {
			return fmt.Errorf("%w: replayed Routine terminal is inconsistent", controlapi.ErrWorkerStaleLease)
		}
		return nil
	}
	if (runState != string(conversation.RoutineRunQueued) && runState != string(conversation.RoutineRunWorking)) ||
		(occurrenceState != "queued" && occurrenceState != "working") || resultCount != 0 ||
		(runAttemptID.Valid && runAttemptID.String != attemptID) {
		return fmt.Errorf("%w: Routine terminal conflicts with persisted state", controlapi.ErrWorkerStaleLease)
	}

	if affected, err := tx.exec(ctx, `update fort_private.routine_run
set state=$1,execution_attempt_id=$2,failure_code=$3,next_action=null,terminal_at=$4,updated_at=$4
where account_id=$5 and routine_run_id=$6 and target_id=$7 and state in ('queued','working')
  and (execution_attempt_id is null or execution_attempt_id=$2)`, string(desiredState), attemptID,
		nullablePostgresString(failureCode), committedAt.UTC(), accountID, runID, targetID); err != nil || affected != 1 {
		return changedRowsError("commit Routine worker terminal", affected, err)
	}
	if affected, err := tx.exec(ctx, `update fort_private.routine_occurrence
set state=$1,updated_at=$2
where account_id=$3 and routine_occurrence_id=$4 and state in ('queued','working')`, string(desiredState),
		committedAt.UTC(), accountID, occurrenceID); err != nil || affected != 1 {
		return changedRowsError("commit Routine occurrence terminal", affected, err)
	}
	if affected, err := tx.exec(ctx, `update fort_private.conversation_turn
set state=$1,updated_at=$2
where account_id=$3 and turn_id=$4 and state='open'`, turnState, committedAt.UTC(), accountID,
		routineTurnID(runID)); err != nil || affected != 1 {
		return changedRowsError("commit Routine turn terminal", affected, err)
	}
	if desiredState == conversation.RoutineRunSucceeded {
		cipher, err := store.collaborationBodies()
		if err != nil {
			return err
		}
		body, err := cipher.seal(securebody.Scope{AccountID: accountID,
			RecordType: "routine_result", RecordID: runID}, outputBody)
		if err != nil {
			return fmt.Errorf("encrypt Routine result: %w", err)
		}
		var messageID int64
		if err := tx.queryRow(ctx, `insert into fort_private.conversation_message (
  account_id,conversation_id,turn_id,target_id,handoff_id,routine_run_id,message_kind,
  author_kind,author_id,author_agent_id,body_ciphertext,body_key_id,body_nonce,body_digest,
  body_plaintext_length,created_at
) values ($1,$2,$3,$4,null,$5,'routine_result','agent',$6,$6,$7,$8,$9,$10,$11,$12)
returning message_id`, accountID, resultConversationID, routineTurnID(runID), targetID, runID, agentID,
			body.Ciphertext, body.KeyID, body.Nonce, body.Digest, body.PlaintextBytes, committedAt.UTC()).scan(&messageID); err != nil {
			return translateRoutineWriteError(err, "Routine result")
		}
		if affected, err := tx.exec(ctx, `update fort_private.conversation
set updated_at=$1 where account_id=$2 and conversation_id=$3`, committedAt.UTC(), accountID,
			resultConversationID); err != nil || affected != 1 {
			return changedRowsError("update Routine result Conversation", affected, err)
		}
	}
	return appendRoutineRunActivity(ctx, tx, accountID, runID, targetID, "routine.run.advanced",
		routineActivityMetadata{State: desiredState, AttemptID: attemptID, LeaseID: leaseID,
			Activity: activity, FailureCode: failureCode}, committedAt)
}

func markPostgresRoutineOccurrenceLate(ctx context.Context, tx transaction, accountID, occurrenceID string,
	observedAt time.Time) error {
	var runID, targetID, turnID string
	err := tx.queryRow(ctx, `select run.routine_run_id,run.target_id,target.turn_id
from fort_private.routine_run as run
join fort_private.conversation_target as target
  on target.account_id=run.account_id and target.target_id=run.target_id
where run.account_id=$1 and run.routine_occurrence_id=$2 and run.state='queued'
for update of run,target`, accountID, occurrenceID).scan(&runID, &targetID, &turnID)
	if isNoRows(err) {
		return nil
	}
	if err != nil {
		return err
	}
	nextAction, _ := json.Marshal(routineValue{Value: "review_missed_occurrence"})
	if affected, err := tx.exec(ctx, `update fort_private.routine_run
set state='needs_you',failure_code='routine_late',next_action=$1::jsonb,updated_at=$2
where account_id=$3 and routine_run_id=$4 and state='queued'`, string(nextAction), observedAt.UTC(),
		accountID, runID); err != nil || affected != 1 {
		return changedRowsError("mark late Routine run", affected, err)
	}
	if affected, err := tx.exec(ctx, `update fort_private.conversation_target
set state='needs_you',error_code='routine_late',updated_at=$1
where account_id=$2 and target_id=$3 and state='queued'`, observedAt.UTC(), accountID,
		targetID); err != nil || affected != 1 {
		return changedRowsError("mark late Routine target", affected, err)
	}
	if affected, err := tx.exec(ctx, `update fort_private.conversation_turn
set state='needs_you',updated_at=$1
where account_id=$2 and turn_id=$3 and state='open'`, observedAt.UTC(), accountID,
		turnID); err != nil || affected != 1 {
		return changedRowsError("mark late Routine turn", affected, err)
	}
	return appendRoutineRunActivity(ctx, tx, accountID, runID, targetID, "routine.run.advanced",
		routineActivityMetadata{State: conversation.RoutineRunNeedsYou, Activity: "occurrence exceeded lateness policy",
			FailureCode: "routine_late", NextAction: "review_missed_occurrence"}, observedAt)
}

func appendRoutineEvent(ctx context.Context, tx transaction, accountID, routineID, eventType string,
	metadata map[string]any, createdAt time.Time) error {
	payload, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	_, err = tx.exec(ctx, `insert into fort_private.ledger_event (
  account_id,aggregate_kind,aggregate_id,event_type,event_metadata,created_at
) values ($1,'routine',$2,$3,$4::jsonb,$5)`, accountID, routineID, eventType,
		string(payload), createdAt.UTC())
	return err
}

func domainRoutineState(state string) (conversation.RoutineState, ledger.RoutinePauseReason, error) {
	switch state {
	case "active":
		return conversation.RoutineActive, "", nil
	case "paused":
		return conversation.RoutinePaused, "", nil
	case "paused_needs_revalidation":
		return conversation.RoutinePaused, ledger.RoutinePauseNeedsRevalidation, nil
	case "archived":
		return conversation.RoutineArchived, "", nil
	default:
		return "", "", fmt.Errorf("unknown Postgres Routine state %q", state)
	}
}

func postgresRoutineOccurrenceState(state conversation.RoutineRunState) string {
	if state == conversation.RoutineRunNeedsYou {
		return "missed_needs_attention"
	}
	return string(state)
}

func domainRoutineOccurrenceState(state string) conversation.RoutineRunState {
	if state == "missed_needs_attention" || state == "scheduled" {
		if state == "missed_needs_attention" {
			return conversation.RoutineRunNeedsYou
		}
		return conversation.RoutineRunQueued
	}
	return conversation.RoutineRunState(state)
}

func routineRunTerminal(state conversation.RoutineRunState) bool {
	return state == conversation.RoutineRunSucceeded || state == conversation.RoutineRunFailed ||
		state == conversation.RoutineRunCanceled
}

func routineTurnID(runID string) string     { return "turn:routine:" + runID }
func routineTargetID(runID string) string   { return "target:routine:" + runID }
func routineManifestID(runID string) string { return "context:routine:" + runID }
func routineGrantID(runID string) string    { return "grant:routine:" + runID }

type postgresRoutineStartDecision struct {
	RunState        conversation.RoutineRunState
	OccurrenceState string
	TargetState     string
	TurnState       string
	FailureCode     string
	NextAction      string
	Activity        string
	Input           string
	SourceMessageID int64
}

func evaluatePostgresRoutineStart(ctx context.Context, tx transaction, cipher collaborationBodyCipher,
	accountID string, record ledger.RoutineRecord, command ledger.EnqueueRoutineOccurrenceCommand,
	existingOccurrenceState string) (postgresRoutineStartDecision, error) {
	revision := record.CurrentRevision
	if command.Kind == conversation.RoutineRunScheduled {
		if revision.LatenessPolicy != conversation.RoutineLatenessWithin90Seconds {
			return routineNeedsYouDecision("routine_lateness_policy_unsupported", "review_routine_policy"), nil
		}
		if command.CreatedAt.Before(command.ScheduledFor) {
			return routineNeedsYouDecision("routine_not_due", "review_routine_occurrence"), nil
		}
		if command.CreatedAt.Sub(command.ScheduledFor) > controlapi.MaximumScheduleLateness ||
			existingOccurrenceState == "missed_needs_attention" {
			return routineNeedsYouDecision("routine_late", "review_missed_occurrence"), nil
		}
	}
	if revision.ApprovalBoundary != conversation.RoutineApprovalNone {
		if revision.ApprovalBoundary == conversation.RoutineApprovalBeforeExternalSideEffect {
			return routineNeedsYouDecision("routine_approval_required", "approve_routine_run"), nil
		}
		return routineNeedsYouDecision("routine_approval_policy_unsupported", "review_routine_policy"), nil
	}
	if revision.InputSource == "none" {
		return routineQueuedDecision("No external input is required."), nil
	}
	const conversationPrefix = "fort:conversation:"
	if !strings.HasPrefix(revision.InputSource, conversationPrefix) {
		return routineNeedsYouDecision("routine_input_source_unsupported", "review_routine_input_source"), nil
	}
	conversationID := strings.TrimPrefix(revision.InputSource, conversationPrefix)
	if conversationID == "" || strings.TrimSpace(conversationID) != conversationID || strings.ContainsAny(conversationID, "\r\n\x00") {
		return routineNeedsYouDecision("routine_input_source_unsupported", "review_routine_input_source"), nil
	}

	var message ledger.AgentConversationMessage
	var handoffID, routineRunID, messageKind string
	var body collaborationEncryptedBody
	err := tx.queryRow(ctx, `select message.message_id,message.conversation_id,
  coalesce(message.turn_id,''),coalesce(message.target_id,''),coalesce(message.handoff_id,''),
  coalesce(message.routine_run_id,''),message.message_kind,message.author_kind,message.author_id,
  coalesce(message.author_agent_id,''),message.body_ciphertext,message.body_key_id,message.body_nonce,
  message.body_digest,message.body_plaintext_length,message.created_at
from fort_private.agent_conversation as relation
join fort_private.conversation as conversation
  on conversation.account_id=relation.account_id and conversation.conversation_id=relation.conversation_id
join fort_private.conversation_message as message
  on message.account_id=relation.account_id and message.conversation_id=relation.conversation_id
where relation.account_id=$1 and relation.agent_id=$2 and relation.conversation_id=$3
  and conversation.state='open' and message.message_kind <> 'system' and message.created_at <= $4
order by message.created_at desc,message.message_id desc limit 1`, accountID, record.Routine.AgentID,
		conversationID, command.CreatedAt.UTC()).scan(&message.ID, &message.ConversationID, &message.TurnID,
		&message.TargetID, &handoffID, &routineRunID, &messageKind, &message.AuthorKind, &message.AuthorID,
		&message.AuthorAgentID, &body.Ciphertext, &body.KeyID, &body.Nonce, &body.Digest,
		&body.PlaintextBytes, &message.CreatedAt)
	if isNoRows(err) {
		return routineMissingInputDecision(revision.MissingInputBehavior, false), nil
	}
	if err != nil {
		return postgresRoutineStartDecision{}, err
	}
	if message.CreatedAt.After(command.CreatedAt) || command.CreatedAt.Sub(message.CreatedAt) > time.Duration(revision.FreshnessSeconds)*time.Second {
		return routineMissingInputDecision(revision.MissingInputBehavior, true), nil
	}
	recordType, recordID, err := agentConversationMessageEncryptionScope(messageKind, message, handoffID, routineRunID)
	if err != nil {
		return postgresRoutineStartDecision{}, err
	}
	input, err := cipher.open(securebody.Scope{AccountID: accountID, RecordType: recordType, RecordID: recordID}, body)
	if err != nil {
		return postgresRoutineStartDecision{}, fmt.Errorf("decrypt Routine input: %w", err)
	}
	decision := routineQueuedDecision(input)
	decision.SourceMessageID = message.ID
	return decision, nil
}

func routineQueuedDecision(input string) postgresRoutineStartDecision {
	return postgresRoutineStartDecision{RunState: conversation.RoutineRunQueued, OccurrenceState: "queued",
		TargetState: "queued", TurnState: "open", Activity: "occurrence queued", Input: input}
}

func routineNeedsYouDecision(failureCode, nextAction string) postgresRoutineStartDecision {
	return postgresRoutineStartDecision{RunState: conversation.RoutineRunNeedsYou,
		OccurrenceState: "missed_needs_attention", TargetState: "needs_you", TurnState: "needs_you",
		FailureCode: failureCode, NextAction: nextAction, Activity: "occurrence needs human action"}
}

func routineMissingInputDecision(behavior string, stale bool) postgresRoutineStartDecision {
	reason := "missing"
	if stale {
		reason = "stale"
	}
	switch behavior {
	case "needs_you":
		return routineNeedsYouDecision("routine_input_"+reason, "provide_fresh_routine_input")
	case "skip":
		return postgresRoutineStartDecision{RunState: conversation.RoutineRunFailed,
			OccurrenceState: "failed", TargetState: "failed", TurnState: "settled",
			FailureCode: "routine_input_" + reason + "_skipped", Activity: "occurrence skipped without fresh input"}
	case "fail":
		return postgresRoutineStartDecision{RunState: conversation.RoutineRunFailed,
			OccurrenceState: "failed", TargetState: "failed", TurnState: "settled",
			FailureCode: "routine_input_" + reason, Activity: "occurrence failed without fresh input"}
	default:
		return routineNeedsYouDecision("routine_missing_input_policy_unsupported", "review_routine_policy")
	}
}

func routinePrompt(revision conversation.RoutineRevision, input string) string {
	return "Routine input source: " + revision.InputSource + "\nExact input:\n" + input +
		"\nExpected result: " + revision.ExpectedResult
}

func postgresRoutineManifestDigest(inputSource string, promptMessageID int64, messageIDs []int64) (string, error) {
	payload, err := json.Marshal(struct {
		Version         int     `json:"version"`
		InputSource     string  `json:"input_source"`
		PromptMessageID int64   `json:"prompt_message_id"`
		MessageIDs      []int64 `json:"message_ids"`
	}{1, inputSource, promptMessageID, messageIDs})
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}

func decodeRoutineValue(encoded string) (string, error) {
	var value routineValue
	if err := json.Unmarshal([]byte(encoded), &value); err != nil || strings.TrimSpace(value.Value) == "" {
		return "", fmt.Errorf("decode Routine policy value")
	}
	return value.Value, nil
}

func nullablePostgresTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value.UTC()
}

func nullablePostgresString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func translateRoutineWriteError(err error, subject string) error {
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) {
		switch postgresError.Code {
		case "23505":
			return fmt.Errorf("%w: %s", ledger.ErrIdempotencyConflict, subject)
		case "23503":
			return fmt.Errorf("%w: %s parent", ledger.ErrNotFound, subject)
		case "23514":
			if strings.Contains(postgresError.Message, "routine_run_terminal") {
				return ledger.ErrRoutineRunTerminal
			}
			return fmt.Errorf("%w: %s", ledger.ErrStateConflict, subject)
		}
	}
	return err
}

// Keep the pgx import anchored alongside pgconn for error compatibility.
var _ = pgx.ErrNoRows
