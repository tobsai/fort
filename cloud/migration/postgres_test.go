package migration

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestSnapshotPostgresValidatesAccountBeforeOpeningAndUsesReadOnlySnapshot(t *testing.T) {
	t.Parallel()

	key := bytes.Repeat([]byte{0x42}, ArchiveKeyBytes)
	opened := false
	_, err := snapshotPostgres(context.Background(), "postgres://migration.invalid/db", "not-an-account", key,
		func(context.Context, string, string) (postgresSnapshotSource, error) {
			opened = true
			return nil, errors.New("must not open")
		})
	if err == nil || opened {
		t.Fatalf("invalid account error/opened = %v/%t, want pre-connection rejection", err, opened)
	}

	source := &fakePostgresSnapshotSource{
		schemaVersion: "postgres:migration:20260821203821:schema:" + strings.Repeat("a", 64),
		tables: []Table{{
			Name: "stable_agent",
			Columns: []Column{
				{Name: "account_id", SourceType: "uuid", PrimaryKeyPosition: 1},
				{Name: "agent_id", SourceType: "text", PrimaryKeyPosition: 2},
			},
			Rows: []Row{{Values: []Value{{Kind: ValueText, Text: "4af424a4-d81a-47d5-a495-400868883b86"}, {Kind: ValueText, Text: "agent:1"}}}},
		}},
	}
	archive, err := snapshotPostgres(context.Background(), "postgres://migration.invalid/db",
		"4af424a4-d81a-47d5-a495-400868883b86", key,
		func(_ context.Context, databaseURL, accountID string) (postgresSnapshotSource, error) {
			if databaseURL != "postgres://migration.invalid/db" || accountID != "4af424a4-d81a-47d5-a495-400868883b86" {
				t.Fatalf("snapshot open = %q/%q", databaseURL, accountID)
			}
			return source, nil
		})
	if err != nil {
		t.Fatalf("snapshotPostgres: %v", err)
	}
	if !source.quiescenceChecked || !source.committed || source.rolledBack || archive.SourceEngine != SourcePostgres || archive.RowCount != 1 {
		t.Fatalf("snapshot lifecycle/archive = source %+v archive %+v", source, archive)
	}
}

func TestSnapshotPostgresRollsBackWhenActiveWorkExists(t *testing.T) {
	t.Parallel()

	source := &fakePostgresSnapshotSource{quiescenceErr: ErrDatabaseNotQuiescent}
	_, err := snapshotPostgres(context.Background(), "postgres://migration.invalid/db",
		"4af424a4-d81a-47d5-a495-400868883b86", bytes.Repeat([]byte{0x42}, ArchiveKeyBytes),
		func(context.Context, string, string) (postgresSnapshotSource, error) { return source, nil })
	if !errors.Is(err, ErrDatabaseNotQuiescent) || source.committed || !source.rolledBack {
		t.Fatalf("active work result = %v committed=%t rolledBack=%t", err, source.committed, source.rolledBack)
	}
}

func TestSnapshotPostgresDatabaseHashIsStableAcrossQueryRowOrder(t *testing.T) {
	t.Parallel()

	columns := []Column{
		{Name: "account_id", SourceType: "uuid", PrimaryKeyPosition: 1},
		{Name: "agent_id", SourceType: "text", PrimaryKeyPosition: 2},
	}
	rowA := Row{Values: []Value{{Kind: ValueText, Text: "4af424a4-d81a-47d5-a495-400868883b86"}, {Kind: ValueText, Text: "agent:a"}}}
	rowB := Row{Values: []Value{{Kind: ValueText, Text: "4af424a4-d81a-47d5-a495-400868883b86"}, {Kind: ValueText, Text: "agent:b"}}}
	snapshot := func(rows []Row) Archive {
		t.Helper()
		source := &fakePostgresSnapshotSource{
			schemaVersion: "postgres:migration:20260821203821:schema:" + strings.Repeat("a", 64),
			tables:        []Table{{Name: "stable_agent", Columns: columns, Rows: rows}},
		}
		archive, err := snapshotPostgres(context.Background(), "postgres://migration.invalid/db",
			"4af424a4-d81a-47d5-a495-400868883b86", bytes.Repeat([]byte{0x42}, ArchiveKeyBytes),
			func(context.Context, string, string) (postgresSnapshotSource, error) { return source, nil })
		if err != nil {
			t.Fatal(err)
		}
		return archive
	}
	forward := snapshot([]Row{rowA, rowB})
	reverse := snapshot([]Row{rowB, rowA})
	if forward.SourceDatabaseHash != reverse.SourceDatabaseHash {
		t.Fatalf("database hash changed with query row order: %s != %s", forward.SourceDatabaseHash, reverse.SourceDatabaseHash)
	}
}

func TestPostgresValueCanonicalizesJSONTimestampsAndBytes(t *testing.T) {
	t.Parallel()

	jsonValue, err := postgresValue([]byte(`{"z":1,"a":[true]}`), "jsonb")
	if err != nil || jsonValue.Kind != ValueJSON || jsonValue.Text != `{"a":[true],"z":1}` {
		t.Fatalf("JSON value = %+v, %v", jsonValue, err)
	}
	timestamp, err := postgresValue(time.Date(2026, 8, 21, 15, 0, 0, 123, time.FixedZone("test", -5*60*60)), "timestamp with time zone")
	if err != nil || timestamp.Kind != ValueTimestamp || timestamp.Text != "2026-08-21T20:00:00.000000123Z" {
		t.Fatalf("timestamp value = %+v, %v", timestamp, err)
	}
	bytesValue, err := postgresValue([]byte{0x00, 0xff}, "bytea")
	if err != nil || bytesValue.Kind != ValueBytes || bytesValue.Text != "AP8" {
		t.Fatalf("bytea value = %+v, %v", bytesValue, err)
	}
}

func TestTimestampBoundsUseChronologyRatherThanRFC3339TextOrder(t *testing.T) {
	t.Parallel()

	column := Column{Name: "created_at", SourceType: "timestamp with time zone"}
	for name, observe := range map[string]func(*Table, Column, Value){
		"sqlite":   observeSQLiteEvidence,
		"postgres": observePostgresEvidence,
	} {
		t.Run(name, func(t *testing.T) {
			table := Table{TimestampBounds: make(map[string]TimestampBounds)}
			observe(&table, column, Value{Kind: ValueTimestamp, Text: "2026-08-21T20:00:00Z"})
			observe(&table, column, Value{Kind: ValueTimestamp, Text: "2026-08-21T20:00:00.1Z"})
			bound := table.TimestampBounds[column.Name]
			if bound.Minimum != "2026-08-21T20:00:00Z" || bound.Maximum != "2026-08-21T20:00:00.1Z" {
				t.Fatalf("timestamp bounds = %+v", bound)
			}
		})
	}
}

func TestPostgresSnapshotLocalIntegration(t *testing.T) {
	databaseURL := os.Getenv("FORT_POSTGRES_MIGRATION_TEST_URL")
	if databaseURL == "" {
		t.Skip("FORT_POSTGRES_MIGRATION_TEST_URL is not set")
	}
	ctx := context.Background()
	accountID := uuid.New().String()
	connection, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close(ctx)
	if _, err := connection.Exec(ctx, `insert into fort_private.fort_account (
  account_id, normalized_email, state, created_at, updated_at
) values ($1, $2, 'open', clock_timestamp(), clock_timestamp())`, accountID, "migration-"+accountID+"@example.invalid"); err != nil {
		t.Fatal(err)
	}
	defer connection.Exec(ctx, `delete from fort_private.fort_account where account_id = $1`, accountID)

	archive, err := SnapshotPostgres(ctx, databaseURL,
		accountID, bytes.Repeat([]byte{0x42}, ArchiveKeyBytes))
	if err != nil {
		t.Fatalf("SnapshotPostgres local Supabase: %v", err)
	}
	if archive.SourceEngine != SourcePostgres || archive.TableCount < 30 || archive.SchemaVersion == "" {
		t.Fatalf("local Postgres archive identity = %+v", archive)
	}
	for _, table := range archive.Tables {
		if table.Name == "fort_account" && table.Count == 1 {
			return
		}
	}
	t.Fatal("local Postgres archive did not include the account-scoped fort_account row")
}

type fakePostgresSnapshotSource struct {
	quiescenceErr     error
	schemaVersion     string
	tables            []Table
	quiescenceChecked bool
	committed         bool
	rolledBack        bool
}

func (source *fakePostgresSnapshotSource) validateQuiescence(context.Context) error {
	source.quiescenceChecked = true
	return source.quiescenceErr
}

func (source *fakePostgresSnapshotSource) snapshot(context.Context) (string, []Table, error) {
	return source.schemaVersion, source.tables, nil
}

func (source *fakePostgresSnapshotSource) commit(context.Context) error {
	source.committed = true
	return nil
}

func (source *fakePostgresSnapshotSource) rollback(context.Context) error {
	source.rolledBack = true
	return nil
}
