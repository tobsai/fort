package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tobsai/fort/cloud/controlapi"
	"github.com/tobsai/fort/cloud/securebody"
	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/ledger"
)

// SharedPool owns one pgx connection pool and creates lightweight,
// account-bound Stores per authenticated request. This is the Vercel
// composition seam: account identity is still validated and applied only in
// an explicit transaction, while physical connections remain shared.
type SharedPool struct {
	database    database
	bodyKeyRing *securebody.KeyRing
}

var _ controlapi.NonceClaimer = (*SharedPool)(nil)
var _ controlapi.AgentLister = (*SharedPool)(nil)
var _ controlapi.AgentReader = (*SharedPool)(nil)
var _ controlapi.AgentLifecycleCommandRepository = (*SharedPool)(nil)
var _ controlapi.AgentMutationRepository = (*SharedPool)(nil)
var _ controlapi.AgentConversationReader = (*SharedPool)(nil)
var _ controlapi.AgentConversationCreateRepository = (*SharedPool)(nil)
var _ controlapi.AgentConversationMutationRepository = (*SharedPool)(nil)
var _ controlapi.RoutineOwnerRepository = (*SharedPool)(nil)
var _ controlapi.GroupLister = (*SharedPool)(nil)
var _ controlapi.GroupCreateRepository = (*SharedPool)(nil)
var _ controlapi.GroupDetailRepository = (*SharedPool)(nil)
var _ controlapi.GroupMutationRepository = (*SharedPool)(nil)
var _ controlapi.GroupMembersRepository = (*SharedPool)(nil)
var _ controlapi.GroupTurnRepository = (*SharedPool)(nil)
var _ controlapi.HumanHandoffRepository = (*SharedPool)(nil)
var _ controlapi.AgentDirectChatRepository = (*SharedPool)(nil)
var _ controlapi.WorkerRepository = (*SharedPool)(nil)
var _ ledger.RoutineRepository = (*SharedPool)(nil)

// OpenSharedPool opens a Supavisor transaction-safe pool suitable for reuse by
// all account requests in one warm function instance.
func OpenSharedPool(ctx context.Context, databaseURL string) (*SharedPool, error) {
	if err := validateSupavisorRuntimeDatabaseURL(databaseURL); err != nil {
		return nil, err
	}
	config, err := SupavisorTransactionConfig(databaseURL)
	if err != nil {
		return nil, err
	}
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("open shared Postgres pool: %w", err)
	}
	return &SharedPool{database: pgxDatabase{pool: pool}}, nil
}

// OpenSharedPoolWithKeyRing opens one warm-function pool whose account-bound
// Stores can encrypt and decrypt collaboration bodies. Key bytes are cloned at
// the pool boundary and again for each Store.
func OpenSharedPoolWithKeyRing(ctx context.Context, databaseURL string, ring securebody.KeyRing) (*SharedPool, error) {
	if err := validateSupavisorRuntimeDatabaseURL(databaseURL); err != nil {
		return nil, err
	}
	config, err := SupavisorTransactionConfig(databaseURL)
	if err != nil {
		return nil, err
	}
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("open shared Postgres pool: %w", err)
	}
	shared, err := newSharedPoolWithKeyRing(pgxDatabase{pool: pool}, ring)
	if err != nil {
		pool.Close()
		return nil, err
	}
	return shared, nil
}

// NewSharedPool wraps an existing pgx pool. SharedPool.Close closes it.
func NewSharedPool(pool *pgxpool.Pool) (*SharedPool, error) {
	if pool == nil {
		return nil, fmt.Errorf("Postgres pool is required")
	}
	return newSharedPool(pgxDatabase{pool: pool})
}

func NewSharedPoolWithKeyRing(pool *pgxpool.Pool, ring securebody.KeyRing) (*SharedPool, error) {
	if pool == nil {
		return nil, fmt.Errorf("Postgres pool is required")
	}
	return newSharedPoolWithKeyRing(pgxDatabase{pool: pool}, ring)
}

func newSharedPool(database database) (*SharedPool, error) {
	if database == nil {
		return nil, fmt.Errorf("Postgres database is required")
	}
	return &SharedPool{database: database}, nil
}

func newSharedPoolWithKeyRing(database database, ring securebody.KeyRing) (*SharedPool, error) {
	if database == nil {
		return nil, fmt.Errorf("Postgres database is required")
	}
	cloned, err := cloneBodyKeyRing(ring)
	if err != nil {
		return nil, err
	}
	return &SharedPool{database: database, bodyKeyRing: &cloned}, nil
}

// ForAccount returns a Store that cannot close or retarget the shared pool.
func (pool *SharedPool) ForAccount(accountID string) (*Store, error) {
	if pool == nil || pool.database == nil {
		return nil, fmt.Errorf("shared Postgres pool is required")
	}
	store, err := newAccountStore(pool.database, accountID, false)
	if err != nil {
		return nil, err
	}
	if pool.bodyKeyRing != nil {
		ring, err := cloneBodyKeyRing(*pool.bodyKeyRing)
		if err != nil {
			return nil, err
		}
		store.bodyCipher = secureCollaborationBodyCipher{ring: ring}
	}
	return store, nil
}

// Claim implements controlapi.NonceClaimer over the shared pool. accountID is
// the signature-verified claim passed by controlapi, not request input.
func (pool *SharedPool) Claim(ctx context.Context, accountID, keyID, nonce string, expiresAt time.Time) (bool, error) {
	store, err := pool.ForAccount(accountID)
	if err != nil {
		return false, err
	}
	return store.Claim(ctx, accountID, keyID, nonce, expiresAt)
}

// ListAgents implements the owner Agent projection without ever retargeting a
// Store: each request gets a new non-owning account-bound view of the pool.
func (pool *SharedPool) ListAgents(ctx context.Context, accountID string, state conversation.AgentState) ([]ledger.AgentRecord, error) {
	store, err := pool.ForAccount(accountID)
	if err != nil {
		return nil, err
	}
	return store.ListAgents(ctx, accountID, state)
}

// GetAgent resolves one stable Agent through a fresh account-bound Store.
func (pool *SharedPool) GetAgent(ctx context.Context, accountID, agentID string) (ledger.AgentRecord, error) {
	store, err := pool.ForAccount(accountID)
	if err != nil {
		return ledger.AgentRecord{}, err
	}
	return store.GetAgent(ctx, accountID, agentID)
}

// AppendAgentProfile advances only Fort-owned presentation state through a
// fresh account-bound Store. Execution identity is outside this contract.
func (pool *SharedPool) AppendAgentProfile(ctx context.Context, command ledger.AppendAgentProfileCommand) (ledger.AgentRecord, error) {
	store, err := pool.ForAccount(command.AccountID)
	if err != nil {
		return ledger.AgentRecord{}, err
	}
	return store.AppendAgentProfile(ctx, command)
}

// AppendAgentBehavior advances Fort-owned behavior while the Store enforces
// exact reuse of the current execution identity and immutable revision chain.
func (pool *SharedPool) AppendAgentBehavior(ctx context.Context, command ledger.AppendAgentBehaviorCommand) (ledger.AgentBindingAdvanceResult, error) {
	store, err := pool.ForAccount(command.AccountID)
	if err != nil {
		return ledger.AgentBindingAdvanceResult{}, err
	}
	return store.AppendAgentBehavior(ctx, command)
}

func (pool *SharedPool) CreateAgent(ctx context.Context, command ledger.CreateAgentCommand) (ledger.AgentRecord, error) {
	store, err := pool.ForAccount(command.Agent.AccountID)
	if err != nil {
		return ledger.AgentRecord{}, err
	}
	return store.CreateAgent(ctx, command)
}

func (pool *SharedPool) PreviewAgentRebind(ctx context.Context, command ledger.PreviewAgentRebindCommand) (ledger.AgentRebindPreview, error) {
	store, err := pool.ForAccount(command.AccountID)
	if err != nil {
		return ledger.AgentRebindPreview{}, err
	}
	return store.PreviewAgentRebind(ctx, command)
}

func (pool *SharedPool) AcceptAgentRebind(ctx context.Context, command ledger.AcceptAgentRebindCommand) (ledger.AgentBindingAdvanceResult, error) {
	store, err := pool.ForAccount(command.Preview.AccountID)
	if err != nil {
		return ledger.AgentBindingAdvanceResult{}, err
	}
	return store.AcceptAgentRebind(ctx, command)
}

func (pool *SharedPool) RecordSourceRoutineProjection(ctx context.Context, projection ledger.SourceRoutineProjection) (ledger.SourceRoutineProjection, error) {
	store, err := pool.ForAccount(projection.AccountID)
	if err != nil {
		return ledger.SourceRoutineProjection{}, err
	}
	return store.RecordSourceRoutineProjection(ctx, projection)
}

func (pool *SharedPool) ListSourceRoutineProjections(ctx context.Context, accountID, executionSourceID string) ([]ledger.SourceRoutineProjection, error) {
	store, err := pool.ForAccount(accountID)
	if err != nil {
		return nil, err
	}
	return store.ListSourceRoutineProjections(ctx, accountID, executionSourceID)
}

func (pool *SharedPool) CreateRoutine(ctx context.Context, command ledger.CreateRoutineCommand) (ledger.RoutineRecord, error) {
	store, err := pool.ForAccount(command.Routine.AccountID)
	if err != nil {
		return ledger.RoutineRecord{}, err
	}
	return store.CreateRoutine(ctx, command)
}

func (pool *SharedPool) ImportSourceRoutine(ctx context.Context, command ledger.ImportRoutineCommand) (ledger.RoutineRecord, error) {
	store, err := pool.ForAccount(command.Create.Routine.AccountID)
	if err != nil {
		return ledger.RoutineRecord{}, err
	}
	return store.ImportSourceRoutine(ctx, command)
}

func (pool *SharedPool) GetRoutine(ctx context.Context, accountID, routineID string) (ledger.RoutineRecord, error) {
	store, err := pool.ForAccount(accountID)
	if err != nil {
		return ledger.RoutineRecord{}, err
	}
	return store.GetRoutine(ctx, accountID, routineID)
}

func (pool *SharedPool) ListRoutines(ctx context.Context, accountID, agentID string) ([]ledger.RoutineRecord, error) {
	store, err := pool.ForAccount(accountID)
	if err != nil {
		return nil, err
	}
	return store.ListRoutines(ctx, accountID, agentID)
}

func (pool *SharedPool) EnqueueRoutineOccurrence(ctx context.Context, command ledger.EnqueueRoutineOccurrenceCommand) (ledger.RoutineRunRecord, error) {
	store, err := pool.ForAccount(command.AccountID)
	if err != nil {
		return ledger.RoutineRunRecord{}, err
	}
	return store.EnqueueRoutineOccurrence(ctx, command)
}

func (pool *SharedPool) AdvanceRoutineRun(ctx context.Context, command ledger.AdvanceRoutineRunCommand) (ledger.RoutineRunRecord, error) {
	store, err := pool.ForAccount(command.AccountID)
	if err != nil {
		return ledger.RoutineRunRecord{}, err
	}
	return store.AdvanceRoutineRun(ctx, command)
}

func (pool *SharedPool) GetRoutineRun(ctx context.Context, accountID, runID string) (ledger.RoutineRunRecord, error) {
	store, err := pool.ForAccount(accountID)
	if err != nil {
		return ledger.RoutineRunRecord{}, err
	}
	return store.GetRoutineRun(ctx, accountID, runID)
}

func (pool *SharedPool) ListRoutineRuns(ctx context.Context, accountID, routineID string) ([]ledger.RoutineRunRecord, error) {
	store, err := pool.ForAccount(accountID)
	if err != nil {
		return nil, err
	}
	return store.ListRoutineRuns(ctx, accountID, routineID)
}

func (pool *SharedPool) RevalidateRoutine(ctx context.Context, command ledger.RevalidateRoutineCommand) (ledger.RoutineRecord, error) {
	store, err := pool.ForAccount(command.AccountID)
	if err != nil {
		return ledger.RoutineRecord{}, err
	}
	return store.RevalidateRoutine(ctx, command)
}

// ListAgentConversations returns Home first, followed by pinned and recently
// active secondary Conversations, without retargeting the shared pool.
func (pool *SharedPool) ListAgentConversations(ctx context.Context, accountID, agentID string) ([]ledger.AgentConversationRecord, error) {
	store, err := pool.ForAccount(accountID)
	if err != nil {
		return nil, err
	}
	return store.ListAgentConversations(ctx, accountID, agentID)
}

func (pool *SharedPool) CreateSecondaryConversation(ctx context.Context, command ledger.CreateSecondaryConversationCommand) (ledger.AgentConversationRecord, error) {
	store, err := pool.ForAccount(command.AccountID)
	if err != nil {
		return ledger.AgentConversationRecord{}, err
	}
	return store.CreateSecondaryConversation(ctx, command)
}

func (pool *SharedPool) RenameAgentConversation(ctx context.Context, command ledger.RenameAgentConversationCommand) (ledger.AgentConversationRecord, error) {
	store, err := pool.ForAccount(command.AccountID)
	if err != nil {
		return ledger.AgentConversationRecord{}, err
	}
	return store.RenameAgentConversation(ctx, command)
}

func (pool *SharedPool) SetAgentConversationState(ctx context.Context, command ledger.SetAgentConversationStateCommand) (ledger.AgentConversationRecord, error) {
	store, err := pool.ForAccount(command.AccountID)
	if err != nil {
		return ledger.AgentConversationRecord{}, err
	}
	return store.SetAgentConversationState(ctx, command)
}

func (pool *SharedPool) SetAgentConversationPin(ctx context.Context, command ledger.SetAgentConversationPinCommand) (ledger.AgentConversationRecord, error) {
	store, err := pool.ForAccount(command.AccountID)
	if err != nil {
		return ledger.AgentConversationRecord{}, err
	}
	return store.SetAgentConversationPin(ctx, command)
}

// ListGroups resolves the stable Group roster for one verified account.
func (pool *SharedPool) ListGroups(ctx context.Context, accountID string, state conversation.ConversationState) ([]ledger.GroupRecord, error) {
	store, err := pool.ForAccount(accountID)
	if err != nil {
		return nil, err
	}
	return store.ListGroups(ctx, accountID, state)
}

func (pool *SharedPool) CreateGroup(ctx context.Context, command ledger.CreateGroupCommand) (ledger.GroupRecord, error) {
	store, err := pool.ForAccount(command.Group.AccountID)
	if err != nil {
		return ledger.GroupRecord{}, err
	}
	return store.CreateGroup(ctx, command)
}

func (pool *SharedPool) GetGroup(ctx context.Context, accountID, groupID string) (ledger.GroupRecord, error) {
	store, err := pool.ForAccount(accountID)
	if err != nil {
		return ledger.GroupRecord{}, err
	}
	return store.GetGroup(ctx, accountID, groupID)
}

func (pool *SharedPool) RenameGroup(ctx context.Context, command ledger.RenameGroupCommand) (ledger.GroupRecord, error) {
	store, err := pool.ForAccount(command.AccountID)
	if err != nil {
		return ledger.GroupRecord{}, err
	}
	return store.RenameGroup(ctx, command)
}

func (pool *SharedPool) SetGroupState(ctx context.Context, command ledger.SetGroupStateCommand) (ledger.GroupRecord, error) {
	store, err := pool.ForAccount(command.AccountID)
	if err != nil {
		return ledger.GroupRecord{}, err
	}
	return store.SetGroupState(ctx, command)
}

func (pool *SharedPool) ReplaceGroupMembers(ctx context.Context, command ledger.ReplaceGroupMembersCommand) (ledger.GroupRecord, error) {
	store, err := pool.ForAccount(command.AccountID)
	if err != nil {
		return ledger.GroupRecord{}, err
	}
	return store.ReplaceGroupMembers(ctx, command)
}

func (pool *SharedPool) SendGroupTurn(ctx context.Context, command ledger.SendGroupTurnCommand) (ledger.GroupTurnRecord, error) {
	store, err := pool.ForAccount(command.AccountID)
	if err != nil {
		return ledger.GroupTurnRecord{}, err
	}
	return store.SendGroupTurn(ctx, command)
}

func (pool *SharedPool) ListGroupTurns(ctx context.Context, accountID, groupID string) ([]ledger.GroupTurnRecord, error) {
	store, err := pool.ForAccount(accountID)
	if err != nil {
		return nil, err
	}
	return store.ListGroupTurns(ctx, accountID, groupID)
}

func (pool *SharedPool) ListGroupMessages(ctx context.Context, accountID, groupID string) ([]ledger.AgentConversationMessage, error) {
	store, err := pool.ForAccount(accountID)
	if err != nil {
		return nil, err
	}
	return store.ListGroupMessages(ctx, accountID, groupID)
}

func (pool *SharedPool) CreateHumanHandoff(ctx context.Context, command ledger.CreateHumanHandoffCommand) (ledger.HandoffRecord, error) {
	store, err := pool.ForAccount(command.AccountID)
	if err != nil {
		return ledger.HandoffRecord{}, err
	}
	return store.CreateHumanHandoff(ctx, command)
}

func (pool *SharedPool) ListHandoffs(ctx context.Context, accountID string) ([]ledger.HandoffRecord, error) {
	store, err := pool.ForAccount(accountID)
	if err != nil {
		return nil, err
	}
	return store.ListHandoffs(ctx, accountID)
}

func (pool *SharedPool) GetHandoff(ctx context.Context, accountID, handoffID string) (ledger.HandoffRecord, error) {
	store, err := pool.ForAccount(accountID)
	if err != nil {
		return ledger.HandoffRecord{}, err
	}
	return store.GetHandoff(ctx, accountID, handoffID)
}

func (pool *SharedPool) CancelHandoff(ctx context.Context, command ledger.CancelHandoffCommand) (ledger.HandoffRecord, error) {
	store, err := pool.ForAccount(command.AccountID)
	if err != nil {
		return ledger.HandoffRecord{}, err
	}
	return store.CancelHandoff(ctx, command)
}

// ReadAgentConversation resolves one exact stable-Agent child projection
// through a fresh account-bound view of the shared pool.
func (pool *SharedPool) ReadAgentConversation(ctx context.Context, accountID, agentID, conversationID string) (ledger.AgentConversationProjection, error) {
	store, err := pool.ForAccount(accountID)
	if err != nil {
		return ledger.AgentConversationProjection{}, err
	}
	return store.ReadAgentConversation(ctx, accountID, agentID, conversationID)
}

func (pool *SharedPool) SendAgentTurn(ctx context.Context, command ledger.SendAgentTurnCommand) (ledger.AgentTurnDispatch, error) {
	store, err := pool.ForAccount(command.AccountID)
	if err != nil {
		return ledger.AgentTurnDispatch{}, err
	}
	return store.SendAgentTurn(ctx, command)
}

func (pool *SharedPool) RetryAgentTarget(ctx context.Context, command ledger.RetryAgentTargetCommand) (ledger.AgentConversationTarget, error) {
	store, err := pool.ForAccount(command.AccountID)
	if err != nil {
		return ledger.AgentConversationTarget{}, err
	}
	return store.RetryAgentTarget(ctx, command)
}

func (pool *SharedPool) CancelAgentTarget(ctx context.Context, command ledger.CancelAgentTargetCommand) (ledger.AgentConversationTarget, error) {
	store, err := pool.ForAccount(command.AccountID)
	if err != nil {
		return ledger.AgentConversationTarget{}, err
	}
	return store.CancelAgentTarget(ctx, command)
}

func (pool *SharedPool) MachineCredential(ctx context.Context, accountID, workerID, machineID string) (controlapi.MachineCredential, error) {
	store, err := pool.ForAccount(accountID)
	if err != nil {
		return controlapi.MachineCredential{}, err
	}
	return store.MachineCredential(ctx, accountID, workerID, machineID)
}

func (pool *SharedPool) RecordWorkerReadiness(ctx context.Context, command controlapi.WorkerReadinessCommand) (controlapi.WorkerReadinessResult, error) {
	store, err := pool.ForAccount(command.AccountID)
	if err != nil {
		return controlapi.WorkerReadinessResult{}, err
	}
	return store.RecordWorkerReadiness(ctx, command)
}

func (pool *SharedPool) ClaimWorkerTarget(ctx context.Context, command controlapi.WorkerClaimCommand) (controlapi.WorkerAssignment, error) {
	store, err := pool.ForAccount(command.AccountID)
	if err != nil {
		return controlapi.WorkerAssignment{}, err
	}
	return store.ClaimWorkerTarget(ctx, command)
}

func (pool *SharedPool) ClaimNextWorkerTarget(ctx context.Context, command controlapi.WorkerClaimNextCommand) (controlapi.WorkerAssignment, error) {
	store, err := pool.ForAccount(command.AccountID)
	if err != nil {
		return controlapi.WorkerAssignment{}, err
	}
	return store.ClaimNextWorkerTarget(ctx, command)
}

func (pool *SharedPool) HeartbeatWorkerLease(ctx context.Context, command controlapi.WorkerLeaseHeartbeatCommand) (controlapi.WorkerLeaseHeartbeatResult, error) {
	store, err := pool.ForAccount(command.AccountID)
	if err != nil {
		return controlapi.WorkerLeaseHeartbeatResult{}, err
	}
	return store.HeartbeatWorkerLease(ctx, command)
}

func (pool *SharedPool) AcknowledgeWorkerCancellation(ctx context.Context, command controlapi.WorkerCancellationAckCommand) (controlapi.WorkerCancellationAck, error) {
	store, err := pool.ForAccount(command.AccountID)
	if err != nil {
		return controlapi.WorkerCancellationAck{}, err
	}
	return store.AcknowledgeWorkerCancellation(ctx, command)
}

func (pool *SharedPool) CommitWorkerTerminal(ctx context.Context, command controlapi.WorkerTerminalCommand) (controlapi.WorkerTerminalResult, error) {
	store, err := pool.ForAccount(command.AccountID)
	if err != nil {
		return controlapi.WorkerTerminalResult{}, err
	}
	return store.CommitWorkerTerminal(ctx, command)
}

func (pool *SharedPool) CreateWorkerArtifact(ctx context.Context, command controlapi.WorkerArtifactCreateCommand) (controlapi.WorkerArtifact, error) {
	store, err := pool.ForAccount(command.AccountID)
	if err != nil {
		return controlapi.WorkerArtifact{}, err
	}
	return store.CreateWorkerArtifact(ctx, command)
}

func (pool *SharedPool) GetWorkerArtifactStatus(ctx context.Context, command controlapi.WorkerArtifactStatusCommand) (controlapi.WorkerArtifact, error) {
	store, err := pool.ForAccount(command.AccountID)
	if err != nil {
		return controlapi.WorkerArtifact{}, err
	}
	return store.GetWorkerArtifactStatus(ctx, command)
}

func (pool *SharedPool) AppendWorkerArtifactChunk(ctx context.Context, command controlapi.WorkerArtifactChunkCommand) (controlapi.WorkerArtifactChunk, error) {
	store, err := pool.ForAccount(command.AccountID)
	if err != nil {
		return controlapi.WorkerArtifactChunk{}, err
	}
	return store.AppendWorkerArtifactChunk(ctx, command)
}

func (pool *SharedPool) FinalizeWorkerArtifact(ctx context.Context, command controlapi.WorkerArtifactFinalizeCommand) (controlapi.WorkerArtifact, error) {
	store, err := pool.ForAccount(command.AccountID)
	if err != nil {
		return controlapi.WorkerArtifact{}, err
	}
	return store.FinalizeWorkerArtifact(ctx, command)
}

func (pool *SharedPool) Close() error {
	if pool == nil || pool.database == nil {
		return nil
	}
	pool.database.close()
	return nil
}
