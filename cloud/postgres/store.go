package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/ledger"
)

type row interface {
	scan(...any) error
}

type rows interface {
	next() bool
	scan(...any) error
	errResult() error
	close()
}

type transaction interface {
	exec(context.Context, string, ...any) (int64, error)
	query(context.Context, string, ...any) (rows, error)
	queryRow(context.Context, string, ...any) row
	commit(context.Context) error
	rollback(context.Context) error
}

type database interface {
	begin(context.Context) (transaction, error)
	close()
}

// Store is an account-bound Postgres implementation of Fort's cloud ledger.
// Binding the account at construction prevents a request from selecting its
// tenant through SQL text, headers, or connection-session state.
type Store struct {
	database      database
	accountID     string
	closeDatabase bool
	bodyCipher    collaborationBodyCipher
}

// Open creates an account-bound Store using Supavisor transaction-safe pgx
// settings. Close must be called when the Store is no longer needed.
func Open(ctx context.Context, databaseURL, accountID string) (*Store, error) {
	if err := validateSupavisorRuntimeDatabaseURL(databaseURL); err != nil {
		return nil, err
	}
	config, err := SupavisorTransactionConfig(databaseURL)
	if err != nil {
		return nil, err
	}
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("open Postgres pool: %w", err)
	}
	store, err := New(pool, accountID)
	if err != nil {
		pool.Close()
		return nil, err
	}
	return store, nil
}

// New binds an existing pgx pool to one Fort account. Store.Close closes the
// supplied pool; callers that share pools should create one Store per pool.
func New(pool *pgxpool.Pool, accountID string) (*Store, error) {
	if pool == nil {
		return nil, fmt.Errorf("Postgres pool is required")
	}
	return newStore(pgxDatabase{pool: pool}, accountID)
}

func newStore(database database, accountID string) (*Store, error) {
	return newAccountStore(database, accountID, true)
}

func newAccountStore(database database, accountID string, closeDatabase bool) (*Store, error) {
	if database == nil {
		return nil, fmt.Errorf("Postgres database is required")
	}
	normalized, err := normalizeAccountID(accountID)
	if err != nil {
		return nil, err
	}
	return &Store{database: database, accountID: normalized, closeDatabase: closeDatabase}, nil
}

func (store *Store) Close() error {
	if store == nil || store.database == nil {
		return nil
	}
	if store.closeDatabase {
		store.database.close()
	}
	return nil
}

func (store *Store) GetAgent(ctx context.Context, accountID, agentID string) (ledger.AgentRecord, error) {
	accountID, err := store.operationAccount(accountID)
	if err != nil {
		return ledger.AgentRecord{}, err
	}
	if strings.TrimSpace(agentID) == "" {
		return ledger.AgentRecord{}, fmt.Errorf("Agent id is required")
	}
	var record ledger.AgentRecord
	err = store.withTransaction(ctx, accountID, func(tx transaction) error {
		var queryErr error
		record, queryErr = getAgentRecord(ctx, tx, accountID, agentID)
		return queryErr
	})
	return record, err
}

func (store *Store) ListAgents(ctx context.Context, accountID string, state conversation.AgentState) ([]ledger.AgentRecord, error) {
	accountID, err := store.operationAccount(accountID)
	if err != nil {
		return nil, err
	}
	if state == "" {
		state = conversation.AgentOpen
	}
	if state != conversation.AgentOpen && state != conversation.AgentArchived {
		return nil, fmt.Errorf("Agent state must be open or archived")
	}

	records := make([]ledger.AgentRecord, 0)
	err = store.withTransaction(ctx, accountID, func(tx transaction) error {
		result, queryErr := tx.query(ctx, `select agent_id
from fort_private.stable_agent
where account_id = $1 and state = $2
order by created_at, agent_id`, accountID, state)
		if queryErr != nil {
			return queryErr
		}
		defer result.close()

		ids := make([]string, 0)
		for result.next() {
			var id string
			if err := result.scan(&id); err != nil {
				return err
			}
			ids = append(ids, id)
		}
		if err := result.errResult(); err != nil {
			return err
		}
		for _, id := range ids {
			record, err := getAgentRecord(ctx, tx, accountID, id)
			if err != nil {
				return err
			}
			records = append(records, record)
		}
		return nil
	})
	return records, err
}

func (store *Store) operationAccount(accountID string) (string, error) {
	normalized, err := normalizeAccountID(accountID)
	if err != nil {
		return "", err
	}
	if normalized != store.accountID {
		return "", fmt.Errorf("Postgres Store is bound to another account")
	}
	return normalized, nil
}

func normalizeAccountID(accountID string) (string, error) {
	parsed, err := uuid.Parse(strings.TrimSpace(accountID))
	if err != nil {
		return "", fmt.Errorf("Fort account id must be a UUID: %w", err)
	}
	return parsed.String(), nil
}

func (store *Store) withTransaction(ctx context.Context, accountID string, operation func(transaction) error) error {
	tx, err := store.database.begin(ctx)
	if err != nil {
		return fmt.Errorf("begin Postgres transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.rollback(ctx)
		}
	}()

	if _, err := tx.exec(ctx, `select set_config('fort.account_id', $1, true)`, accountID); err != nil {
		return fmt.Errorf("scope Postgres transaction: %w", err)
	}
	if err := operation(tx); err != nil {
		return err
	}
	if err := tx.commit(ctx); err != nil {
		return fmt.Errorf("commit Postgres transaction: %w", err)
	}
	committed = true
	return nil
}

type pgxDatabase struct {
	pool *pgxpool.Pool
}

func (database pgxDatabase) begin(ctx context.Context) (transaction, error) {
	tx, err := database.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return pgxTransaction{tx: tx}, nil
}

func (database pgxDatabase) close() { database.pool.Close() }

type pgxTransaction struct {
	tx pgx.Tx
}

func (tx pgxTransaction) exec(ctx context.Context, sql string, arguments ...any) (int64, error) {
	tag, err := tx.tx.Exec(ctx, sql, arguments...)
	return tag.RowsAffected(), err
}

func (tx pgxTransaction) query(ctx context.Context, sql string, arguments ...any) (rows, error) {
	result, err := tx.tx.Query(ctx, sql, arguments...)
	if err != nil {
		return nil, err
	}
	return pgxRows{rows: result}, nil
}

func (tx pgxTransaction) queryRow(ctx context.Context, sql string, arguments ...any) row {
	return pgxRow{row: tx.tx.QueryRow(ctx, sql, arguments...)}
}

func (tx pgxTransaction) commit(ctx context.Context) error   { return tx.tx.Commit(ctx) }
func (tx pgxTransaction) rollback(ctx context.Context) error { return tx.tx.Rollback(ctx) }

type pgxRow struct {
	row pgx.Row
}

func (result pgxRow) scan(destinations ...any) error { return result.row.Scan(destinations...) }

type pgxRows struct {
	rows pgx.Rows
}

func (result pgxRows) next() bool                     { return result.rows.Next() }
func (result pgxRows) scan(destinations ...any) error { return result.rows.Scan(destinations...) }
func (result pgxRows) errResult() error               { return result.rows.Err() }
func (result pgxRows) close()                         { result.rows.Close() }

func isNoRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }
