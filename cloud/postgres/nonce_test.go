package postgres

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestServiceAssertionNonceClaimIsAtomicAndAccountScoped(t *testing.T) {
	t.Parallel()

	first := &fakeTransaction{}
	first.execHook = nonceRowsAffected(1)
	replay := &fakeTransaction{}
	replay.execHook = nonceRowsAffected(0)
	database := &fakeDatabase{transactions: []transaction{first, replay}}
	store, err := newStore(database, testAccountID)
	if err != nil {
		t.Fatalf("newStore: %v", err)
	}
	expiresAt := time.Date(2026, 8, 21, 20, 1, 0, 0, time.UTC)

	claimed, err := store.Claim(context.Background(), testAccountID, "service-2026-08", "nonce-one", expiresAt)
	if err != nil {
		t.Fatalf("first Claim: %v", err)
	}
	if !claimed {
		t.Fatal("first Claim = false, want true")
	}
	claimed, err = store.Claim(context.Background(), testAccountID, "service-2026-08", "nonce-one", expiresAt)
	if err != nil {
		t.Fatalf("replay Claim: %v", err)
	}
	if claimed {
		t.Fatal("replay Claim = true, want false")
	}

	for index, tx := range []*fakeTransaction{first, replay} {
		if tx.commits != 1 || tx.rollbacks != 0 {
			t.Fatalf("transaction %d lifecycle = commits %d rollbacks %d", index, tx.commits, tx.rollbacks)
		}
		if len(tx.execs) != 2 {
			t.Fatalf("transaction %d statements = %+v", index, tx.execs)
		}
		insert := tx.execs[1]
		if !strings.Contains(insert.sql, "insert into fort_private.service_assertion_nonce") ||
			!strings.Contains(insert.sql, "on conflict (account_id, key_id, nonce) do nothing") {
			t.Fatalf("nonce SQL = %q", insert.sql)
		}
		if len(insert.args) != 4 || insert.args[0] != testAccountID || insert.args[1] != "service-2026-08" || insert.args[2] != "nonce-one" {
			t.Fatalf("nonce arguments = %#v", insert.args)
		}
	}
}

func nonceRowsAffected(affected int64) func(string, []any) (int64, error) {
	return func(sql string, _ []any) (int64, error) {
		if strings.Contains(sql, "service_assertion_nonce") {
			return affected, nil
		}
		return 1, nil
	}
}
