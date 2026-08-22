package postgres

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/tobsai/fort/cloud/controlapi"
)

func TestCursorReaderReturnsStableAscendingBoundedLedgerPage(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, 8, 21, 21, 0, 0, 0, time.UTC)
	tx := &fakeTransaction{queryRows: &fakeRows{values: [][]any{
		{int64(13), "message", "message:13", "message.created", "turn:1", "", "", "", `{"message_id":"message:13"}`, createdAt},
		{int64(14), "target", "target:14", "target.finished", "turn:1", "target:14", "attempt:1", "worker:1", `{"state":"succeeded"}`, createdAt.Add(time.Second)},
	}}}
	store, err := newStore(&fakeDatabase{transactions: []transaction{tx}}, testAccountID)
	if err != nil {
		t.Fatalf("newStore: %v", err)
	}

	page, err := store.ReadCursorPage(context.Background(), testAccountID, "cursor-12")
	if err != nil {
		t.Fatalf("ReadCursorPage: %v", err)
	}
	if len(page.Events) != 2 || page.Events[0].Cursor != "cursor-13" ||
		page.Events[1].Cursor != "cursor-14" || page.NextCursor != "cursor-14" {
		t.Fatalf("cursor page = %+v", page)
	}
	if page.Events[0].Kind != "message.created" || page.Events[1].Kind != "target.finished" {
		t.Fatalf("event kinds = %+v", page.Events)
	}
	payload, err := json.Marshal(page)
	if err != nil {
		t.Fatalf("marshal page: %v", err)
	}
	if len(payload)+1 > controlapi.MaximumCursorPageBytes {
		t.Fatalf("cursor page bytes = %d", len(payload)+1)
	}
	if strings.Contains(string(payload), "sensitive") {
		t.Fatalf("cursor page leaked sensitive envelope fields: %s", payload)
	}
	if tx.commits != 1 || tx.rollbacks != 0 {
		t.Fatalf("cursor lifecycle = commits %d rollbacks %d", tx.commits, tx.rollbacks)
	}
	if len(tx.queries) != 1 || !strings.Contains(tx.queries[0].sql, "event_id > $2") ||
		!strings.Contains(tx.queries[0].sql, "order by event_id asc") {
		t.Fatalf("cursor query = %+v", tx.queries)
	}
	if len(tx.queries[0].args) != 3 || tx.queries[0].args[0] != testAccountID || tx.queries[0].args[1] != int64(12) {
		t.Fatalf("cursor arguments = %#v", tx.queries[0].args)
	}
}

func TestCursorReaderPreservesCursorOnEmptyPageAndRejectsForeignFormats(t *testing.T) {
	t.Parallel()

	emptyTx := &fakeTransaction{queryRows: &fakeRows{}}
	database := &fakeDatabase{transactions: []transaction{emptyTx}}
	store, err := newStore(database, testAccountID)
	if err != nil {
		t.Fatalf("newStore: %v", err)
	}
	page, err := store.ReadCursorPage(context.Background(), testAccountID, "cursor-42")
	if err != nil {
		t.Fatalf("ReadCursorPage: %v", err)
	}
	if page.Events == nil || len(page.Events) != 0 || page.NextCursor != "cursor-42" {
		t.Fatalf("empty cursor page = %#v", page)
	}
	if _, err := store.ReadCursorPage(context.Background(), testAccountID, "42"); err == nil {
		t.Fatal("ReadCursorPage accepted a non-Fort cursor")
	}
	if database.begins != 1 {
		t.Fatalf("invalid cursor began a database transaction; begins = %d", database.begins)
	}
}
