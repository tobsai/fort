package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tobsai/fort/cloud/migration"
	_ "modernc.org/sqlite"
)

func TestDBExportSQLiteWritesRestrictedEncryptedArchiveAndDryRunNeverOpensPostgres(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	databasePath := filepath.Join(directory, "frozen.db")
	database, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`
create table run (id text primary key, status text not null, body text not null, created_at text not null);
insert into run values ('run:1','completed','private rollback body','2026-08-21T20:00:00Z');
`); err != nil {
		database.Close()
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	key := bytes.Repeat([]byte{0x42}, migration.ArchiveKeyBytes)
	keyText := base64.RawURLEncoding.EncodeToString(key)
	archivePath := filepath.Join(directory, "sqlite.fort-migration")
	var output bytes.Buffer
	postgresCalls := 0
	dependencies := dbCommandDependencies{
		getenv: func(name string) string {
			if name == "FORT_MIGRATION_KEY" {
				return keyText
			}
			return ""
		},
		stdout:         &output,
		random:         bytes.NewReader(bytes.Repeat([]byte{0x17}, 128)),
		snapshotSQLite: migration.SnapshotSQLite,
		snapshotPostgres: func(context.Context, string, string, []byte) (migration.Archive, error) {
			postgresCalls++
			return migration.Archive{}, errors.New("must not connect")
		},
	}
	if err := cmdDBWithDependencies([]string{"export-sqlite", "--frozen", "--source", databasePath, "--out", archivePath}, dependencies); err != nil {
		t.Fatalf("export-sqlite: %v", err)
	}
	encoded, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(encoded, []byte("private rollback body")) || strings.Contains(output.String(), "private rollback body") || strings.Contains(output.String(), keyText) {
		t.Fatalf("export exposed secret content: output=%s", output.String())
	}
	info, err := os.Stat(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("archive mode = %o, want 600", info.Mode().Perm())
	}
	if _, err := migration.OpenArchive(encoded, key); err != nil {
		t.Fatalf("open exported archive: %v", err)
	}

	output.Reset()
	if err := cmdDBWithDependencies([]string{"import-postgres", "--dry-run", "--archive", archivePath}, dependencies); err != nil {
		t.Fatalf("import-postgres --dry-run: %v", err)
	}
	if postgresCalls != 0 || !strings.Contains(output.String(), `"legacy_retained":1`) || !strings.Contains(output.String(), `"resolved":true`) {
		t.Fatalf("dry-run output/calls = %s / %d", output.String(), postgresCalls)
	}
}

func TestDBImportPostgresRefusesApplyAndReportsUnresolvedRows(t *testing.T) {
	t.Parallel()

	key := bytes.Repeat([]byte{0x42}, migration.ArchiveKeyBytes)
	archive, err := migration.NewArchive(migration.SourceSQLite, "sqlite:schema:1", strings.Repeat("a", 64), []migration.Table{{
		Name: "schedule", Columns: []migration.Column{{Name: "id", SourceType: "TEXT", PrimaryKeyPosition: 1}},
		Rows: []migration.Row{{Values: []migration.Value{{Kind: migration.ValueText, Text: "schedule:1"}}}},
	}}, key)
	if err != nil {
		t.Fatal(err)
	}
	path := writeDBTestArchive(t, archive, key)
	var output bytes.Buffer
	dependencies := testDBDependencies(key, &output)
	if err := cmdDBWithDependencies([]string{"import-postgres", "--archive", path, "--apply"}, dependencies); err == nil || !strings.Contains(err.Error(), "no approved apply mapping") {
		t.Fatalf("apply error = %v, want explicit refusal", err)
	}
	output.Reset()
	if err := cmdDBWithDependencies([]string{"import-postgres", "--archive", path, "--dry-run"}, dependencies); !errors.Is(err, migration.ErrUnresolvedMappings) {
		t.Fatalf("unresolved dry-run error = %v", err)
	}
	if !strings.Contains(output.String(), `"needs_explicit_choice":1`) || !strings.Contains(output.String(), `"resolved":false`) {
		t.Fatalf("unresolved dry-run report = %s", output.String())
	}
}

func TestDBVerifyMigrationUsesTwoEncryptedArchivesAndExportPostgresUsesEnvironmentOnly(t *testing.T) {
	t.Parallel()

	key := bytes.Repeat([]byte{0x42}, migration.ArchiveKeyBytes)
	source, target := dbParityArchives(t, key)
	sourcePath := writeDBTestArchive(t, source, key)
	targetPath := writeDBTestArchive(t, target, key)
	var output bytes.Buffer
	dependencies := testDBDependencies(key, &output)
	called := false
	dependencies.getenv = func(name string) string {
		switch name {
		case "FORT_MIGRATION_KEY":
			return base64.RawURLEncoding.EncodeToString(key)
		case "DATABASE_URL":
			return "postgres://migration-secret@127.0.0.1/fort"
		case "FORT_ACCOUNT_ID":
			return "4af424a4-d81a-47d5-a495-400868883b86"
		default:
			return ""
		}
	}
	dependencies.snapshotPostgres = func(_ context.Context, databaseURL, accountID string, actualKey []byte) (migration.Archive, error) {
		called = true
		if databaseURL != "postgres://migration-secret@127.0.0.1/fort" || accountID != "4af424a4-d81a-47d5-a495-400868883b86" || !bytes.Equal(actualKey, key) {
			t.Fatalf("Postgres export arguments = %q/%q/key-match=%t", databaseURL, accountID, bytes.Equal(actualKey, key))
		}
		return target, nil
	}

	if err := cmdDBWithDependencies([]string{"verify-migration", "--sqlite-archive", sourcePath, "--postgres-archive", targetPath}, dependencies); err != nil {
		t.Fatalf("verify-migration: %v", err)
	}
	if !strings.Contains(output.String(), `"verified":true`) {
		t.Fatalf("verification output = %s", output.String())
	}

	output.Reset()
	postgresPath := filepath.Join(t.TempDir(), "postgres.fort-migration")
	if err := cmdDBWithDependencies([]string{"export-postgres", "--frozen", "--out", postgresPath}, dependencies); err != nil {
		t.Fatalf("export-postgres: %v", err)
	}
	if !called || strings.Contains(output.String(), "migration-secret") {
		t.Fatalf("Postgres export call/output = %t/%s", called, output.String())
	}
}

func TestReadMigrationArchiveRejectsNonRegularFilesWithoutFormattingArtifacts(t *testing.T) {
	t.Parallel()

	_, err := readMigrationArchive(t.TempDir(), bytes.Repeat([]byte{0x42}, migration.ArchiveKeyBytes))
	if err == nil || strings.Contains(err.Error(), "%!w") {
		t.Fatalf("non-regular archive error = %v", err)
	}
}

func testDBDependencies(key []byte, output *bytes.Buffer) dbCommandDependencies {
	return dbCommandDependencies{
		getenv: func(name string) string {
			if name == "FORT_MIGRATION_KEY" {
				return base64.RawURLEncoding.EncodeToString(key)
			}
			return ""
		},
		stdout:           output,
		random:           bytes.NewReader(bytes.Repeat([]byte{0x17}, 256)),
		snapshotSQLite:   migration.SnapshotSQLite,
		snapshotPostgres: migration.SnapshotPostgres,
	}
}

func writeDBTestArchive(t *testing.T, archive migration.Archive, key []byte) string {
	t.Helper()
	sealed, err := migration.SealArchive(archive, key, bytes.NewReader(bytes.Repeat([]byte{0x17}, 64)))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "archive.fort-migration")
	if err := os.WriteFile(path, sealed, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func dbParityArchives(t *testing.T, key []byte) (migration.Archive, migration.Archive) {
	t.Helper()
	sourceColumns := []migration.Column{
		{Name: "id", SourceType: "TEXT", PrimaryKeyPosition: 1}, {Name: "account_id", SourceType: "TEXT"},
		{Name: "state", SourceType: "TEXT"}, {Name: "current_profile_revision_id", SourceType: "TEXT"},
		{Name: "current_behavior_revision_id", SourceType: "TEXT"}, {Name: "current_binding_revision_id", SourceType: "TEXT"},
		{Name: "canonical_conversation_id", SourceType: "TEXT"}, {Name: "canonical_participant_id", SourceType: "TEXT"},
		{Name: "created_at", SourceType: "TEXT"},
	}
	sourceValues := []migration.Value{
		{Kind: migration.ValueText, Text: "agent:1"}, {Kind: migration.ValueText, Text: "4af424a4-d81a-47d5-a495-400868883b86"},
		{Kind: migration.ValueText, Text: "open"}, {Kind: migration.ValueText, Text: "profile:1"},
		{Kind: migration.ValueText, Text: "behavior:1"}, {Kind: migration.ValueText, Text: "binding:1"},
		{Kind: migration.ValueText, Text: "conversation:1"}, {Kind: migration.ValueText, Text: "participant:1"},
		{Kind: migration.ValueText, Text: "2026-08-21T20:00:00Z"},
	}
	targetColumns := []migration.Column{
		{Name: "account_id", SourceType: "uuid", PrimaryKeyPosition: 1}, {Name: "agent_id", SourceType: "text", PrimaryKeyPosition: 2},
		{Name: "state", SourceType: "text"}, {Name: "current_profile_revision_id", SourceType: "text"},
		{Name: "current_behavior_revision_id", SourceType: "text"}, {Name: "current_binding_revision_id", SourceType: "text"},
		{Name: "canonical_conversation_id", SourceType: "text"}, {Name: "created_at", SourceType: "timestamp with time zone"},
	}
	targetValues := []migration.Value{
		{Kind: migration.ValueText, Text: "4af424a4-d81a-47d5-a495-400868883b86"}, {Kind: migration.ValueText, Text: "agent:1"},
		{Kind: migration.ValueText, Text: "open"}, {Kind: migration.ValueText, Text: "profile:1"},
		{Kind: migration.ValueText, Text: "behavior:1"}, {Kind: migration.ValueText, Text: "binding:1"},
		{Kind: migration.ValueText, Text: "conversation:1"}, {Kind: migration.ValueTimestamp, Text: "2026-08-21T20:00:00Z"},
	}
	source, err := migration.NewArchive(migration.SourceSQLite, "sqlite:schema:1", strings.Repeat("a", 64), []migration.Table{{
		Name: "stable_agent", Columns: sourceColumns, Rows: []migration.Row{{Values: sourceValues}},
	}}, key)
	if err != nil {
		t.Fatal(err)
	}
	target, err := migration.NewArchive(migration.SourcePostgres, "postgres:schema:1", strings.Repeat("b", 64), []migration.Table{{
		Name: "stable_agent", Columns: targetColumns, Rows: []migration.Row{{Values: targetValues}},
	}}, key)
	if err != nil {
		t.Fatal(err)
	}
	return source, target
}
