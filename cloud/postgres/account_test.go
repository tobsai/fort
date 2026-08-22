package postgres

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/tobsai/fort/core/conversation"
)

const testAccountID = "4af424a4-d81a-47d5-a495-400868883b86"

func TestListAgentsUsesOneTransactionWithLocalAccountScope(t *testing.T) {
	t.Parallel()

	tx := &fakeTransaction{queryRows: &fakeRows{}}
	database := &fakeDatabase{transactions: []transaction{tx}}
	store, err := newStore(database, testAccountID)
	if err != nil {
		t.Fatalf("newStore: %v", err)
	}

	agents, err := store.ListAgents(context.Background(), testAccountID, conversation.AgentOpen)
	if err != nil {
		t.Fatalf("ListAgents: %v", err)
	}
	if agents == nil || len(agents) != 0 {
		t.Fatalf("empty agents = %#v, want allocated empty slice", agents)
	}
	if database.begins != 1 || tx.commits != 1 || tx.rollbacks != 0 {
		t.Fatalf("transaction lifecycle = begins %d commits %d rollbacks %d", database.begins, tx.commits, tx.rollbacks)
	}
	if len(tx.execs) != 1 || !strings.Contains(tx.execs[0].sql, "set_config('fort.account_id', $1, true)") {
		t.Fatalf("first transaction statement = %+v, want transaction-local account setting", tx.execs)
	}
	if len(tx.execs[0].args) != 1 || tx.execs[0].args[0] != testAccountID {
		t.Fatalf("set_config arguments = %#v", tx.execs[0].args)
	}
	if len(tx.queries) != 1 || !strings.Contains(tx.queries[0].sql, "where account_id = $1") {
		t.Fatalf("list query = %+v, want explicit account predicate", tx.queries)
	}
	if len(tx.queries[0].args) < 1 || tx.queries[0].args[0] != testAccountID {
		t.Fatalf("list query arguments = %#v", tx.queries[0].args)
	}
}

func TestAccountOperationsRejectMalformedOrDifferentAccountBeforeDatabaseUse(t *testing.T) {
	t.Parallel()

	database := &fakeDatabase{}
	store, err := newStore(database, testAccountID)
	if err != nil {
		t.Fatalf("newStore: %v", err)
	}

	if _, err := store.GetAgent(context.Background(), "account-from-header", "agent:one"); err == nil {
		t.Fatal("GetAgent accepted malformed account UUID")
	}
	if _, err := store.ListAgents(context.Background(), "21d9b714-9407-4302-853d-358fcb9abb91", conversation.AgentOpen); err == nil {
		t.Fatal("ListAgents accepted another account")
	}
	if database.begins != 0 {
		t.Fatalf("database began %d transactions for rejected accounts", database.begins)
	}
}

type recordedStatement struct {
	sql  string
	args []any
}

type fakeDatabase struct {
	transactions []transaction
	begins       int
	closed       bool
}

func (database *fakeDatabase) begin(context.Context) (transaction, error) {
	database.begins++
	if len(database.transactions) == 0 {
		return nil, errors.New("unexpected transaction")
	}
	tx := database.transactions[0]
	database.transactions = database.transactions[1:]
	return tx, nil
}

func (database *fakeDatabase) close() { database.closed = true }

type fakeTransaction struct {
	execs        []recordedStatement
	queries      []recordedStatement
	queryRows    rows
	queryErr     error
	execHook     func(string, []any) (int64, error)
	queryRowHook func(string, []any) row
	commits      int
	rollbacks    int
}

func (tx *fakeTransaction) exec(_ context.Context, sql string, args ...any) (int64, error) {
	tx.execs = append(tx.execs, recordedStatement{sql: sql, args: append([]any{}, args...)})
	if tx.execHook != nil {
		return tx.execHook(sql, args)
	}
	return 1, nil
}

func (tx *fakeTransaction) query(_ context.Context, sql string, args ...any) (rows, error) {
	tx.queries = append(tx.queries, recordedStatement{sql: sql, args: append([]any{}, args...)})
	if tx.queryErr != nil {
		return nil, tx.queryErr
	}
	if tx.queryRows == nil {
		return &fakeRows{}, nil
	}
	return tx.queryRows, nil
}

func (tx *fakeTransaction) queryRow(_ context.Context, sql string, args ...any) row {
	tx.queries = append(tx.queries, recordedStatement{sql: sql, args: append([]any{}, args...)})
	if tx.queryRowHook != nil {
		return tx.queryRowHook(sql, args)
	}
	return fakeRow{err: errors.New("unexpected query row")}
}

func (tx *fakeTransaction) commit(context.Context) error {
	tx.commits++
	return nil
}

func (tx *fakeTransaction) rollback(context.Context) error {
	tx.rollbacks++
	return nil
}

type fakeRows struct {
	values [][]any
	index  int
	err    error
}

func (result *fakeRows) next() bool { return result.index < len(result.values) }

func (result *fakeRows) scan(destinations ...any) error {
	if result.index >= len(result.values) {
		return errors.New("scan past rows")
	}
	err := assign(destinations, result.values[result.index])
	result.index++
	return err
}

func (result *fakeRows) errResult() error { return result.err }
func (result *fakeRows) close()           {}

type fakeRow struct {
	values []any
	err    error
}

func (result fakeRow) scan(destinations ...any) error {
	if result.err != nil {
		return result.err
	}
	return assign(destinations, result.values)
}

func assign(destinations []any, values []any) error {
	if len(destinations) != len(values) {
		return errors.New("fake scan destination mismatch")
	}
	for index, destination := range destinations {
		target := reflect.ValueOf(destination)
		if target.Kind() != reflect.Pointer || target.IsNil() {
			return errors.New("fake scan destination is not a pointer")
		}
		value := reflect.ValueOf(values[index])
		if value.Type().AssignableTo(target.Elem().Type()) {
			target.Elem().Set(value)
		} else if value.Type().ConvertibleTo(target.Elem().Type()) {
			target.Elem().Set(value.Convert(target.Elem().Type()))
		} else {
			return errors.New("unsupported fake scan destination")
		}
	}
	return nil
}
