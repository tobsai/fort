package migration_test

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/tobsai/fort/cloud/migration"
	fortstore "github.com/tobsai/fort/core/store"
	_ "modernc.org/sqlite"
)

func TestSQLiteSnapshotCapturesEveryRowInDependencyOrderAndSealsPlaintext(t *testing.T) {
	t.Parallel()

	path := sqliteFixture(t)
	key := bytes.Repeat([]byte{0x42}, migration.ArchiveKeyBytes)
	snapshot, err := migration.SnapshotSQLite(context.Background(), path, key)
	if err != nil {
		t.Fatalf("SnapshotSQLite: %v", err)
	}
	if snapshot.SourceEngine != migration.SourceSQLite || snapshot.SchemaVersion == "" || snapshot.SourceDatabaseHash == "" {
		t.Fatalf("snapshot identity = %+v", snapshot)
	}
	if got, want := tableNames(snapshot.Tables), []string{"parent", "child", "conversation_target"}; !equalStrings(got, want) {
		t.Fatalf("dependency order = %v, want %v", got, want)
	}
	if snapshot.RowCount != 3 || snapshot.Tables[1].Rows[0].Digest == "" || snapshot.Tables[1].Digest == "" {
		t.Fatalf("snapshot row evidence = %+v", snapshot)
	}

	sealed, err := migration.SealArchive(snapshot, key, bytes.NewReader(bytes.Repeat([]byte{0x17}, 64)))
	if err != nil {
		t.Fatalf("SealArchive: %v", err)
	}
	if bytes.Contains(sealed, []byte("private prompt")) {
		t.Fatal("sealed archive exposed plaintext row content")
	}
	opened, err := migration.OpenArchive(sealed, key)
	if err != nil {
		t.Fatalf("OpenArchive: %v", err)
	}
	if opened.ManifestMAC != snapshot.ManifestMAC || opened.RowCount != snapshot.RowCount {
		t.Fatalf("opened snapshot = %+v, want manifest %s rows %d", opened, snapshot.ManifestMAC, snapshot.RowCount)
	}
	if _, err := migration.OpenArchive(sealed, bytes.Repeat([]byte{0x24}, migration.ArchiveKeyBytes)); !errors.Is(err, migration.ErrArchiveAuthentication) {
		t.Fatalf("wrong-key error = %v, want archive authentication failure", err)
	}
}

func TestSQLiteSnapshotFailsClosedWhenTargetWorkIsNotQuiescent(t *testing.T) {
	t.Parallel()

	path := sqliteFixture(t)
	database, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`update conversation_target set state='queued'`); err != nil {
		database.Close()
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}

	_, err = migration.SnapshotSQLite(context.Background(), path, bytes.Repeat([]byte{0x42}, migration.ArchiveKeyBytes))
	if !errors.Is(err, migration.ErrDatabaseNotQuiescent) {
		t.Fatalf("active target error = %v, want database not quiescent", err)
	}
}

func sqliteFixture(t *testing.T) string {
	t.Helper()
	path := t.TempDir() + "/frozen.db"
	database, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err := database.Exec(`
pragma foreign_keys=on;
create table parent (id text primary key, created_at text not null);
create table child (
  id integer primary key autoincrement,
  parent_id text not null references parent(id),
  body text not null,
  created_at text not null
);
create table conversation_target (id text primary key, state text not null, updated_at text not null);
insert into parent values ('parent:1','2026-08-21T20:00:00Z');
insert into child(parent_id,body,created_at) values ('parent:1','private prompt','2026-08-21T20:00:01Z');
insert into conversation_target values ('target:1','answered','2026-08-21T20:00:02Z');
`); err != nil {
		t.Fatal(err)
	}
	return path
}

func tableNames(tables []migration.Table) []string {
	result := make([]string, len(tables))
	for index := range tables {
		result[index] = tables[index].Name
	}
	return result
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func TestArchiveRejectsTamperedManifestEvidence(t *testing.T) {
	t.Parallel()

	key := bytes.Repeat([]byte{0x42}, migration.ArchiveKeyBytes)
	archive, err := migration.NewArchive(migration.SourceSQLite, "sqlite:user_version:0", strings.Repeat("a", 64), []migration.Table{{
		Name:    "stable_agent",
		Columns: []migration.Column{{Name: "id", SourceType: "TEXT", PrimaryKeyPosition: 1}},
		Rows:    []migration.Row{{Values: []migration.Value{{Kind: migration.ValueText, Text: "agent:1"}}}},
	}}, key)
	if err != nil {
		t.Fatal(err)
	}
	archive.Tables[0].Rows[0].Values[0].Text = "agent:forged"
	sealed, err := migration.SealArchive(archive, key, bytes.NewReader(bytes.Repeat([]byte{0x17}, 64)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := migration.OpenArchive(sealed, key); !errors.Is(err, migration.ErrArchiveIntegrity) {
		t.Fatalf("tampered evidence error = %v, want archive integrity failure", err)
	}
}

func TestCurrentSQLiteSchemaHasAnExplicitClassificationForEveryTable(t *testing.T) {
	t.Parallel()

	path := t.TempDir() + "/fort.db"
	store, err := fortstore.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	key := bytes.Repeat([]byte{0x42}, migration.ArchiveKeyBytes)
	archive, err := migration.SnapshotSQLite(context.Background(), path, key)
	if err != nil {
		t.Fatal(err)
	}
	plan := migration.PlanPostgresImport(archive)
	for _, table := range plan.Tables {
		if table.Class == migration.MappingIncompatible {
			t.Errorf("current SQLite table %q has no explicit migration classification", table.SourceTable)
		}
	}
}
