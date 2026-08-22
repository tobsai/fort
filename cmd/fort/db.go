package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/tobsai/fort/cloud/migration"
)

const migrationKeyEnvironment = "FORT_MIGRATION_KEY"

type dbCommandDependencies struct {
	getenv           func(string) string
	stdout           io.Writer
	random           io.Reader
	snapshotSQLite   func(context.Context, string, []byte) (migration.Archive, error)
	snapshotPostgres func(context.Context, string, string, []byte) (migration.Archive, error)
}

func defaultDBCommandDependencies() dbCommandDependencies {
	return dbCommandDependencies{
		getenv: os.Getenv, stdout: os.Stdout, random: rand.Reader,
		snapshotSQLite: migration.SnapshotSQLite, snapshotPostgres: migration.SnapshotPostgres,
	}
}

func cmdDB(args []string) error {
	return cmdDBWithDependencies(args, defaultDBCommandDependencies())
}

func cmdDBWithDependencies(args []string, dependencies dbCommandDependencies) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: fort db <export-sqlite|import-postgres|verify-migration|export-postgres> ...")
	}
	if dependencies.getenv == nil || dependencies.stdout == nil || dependencies.random == nil ||
		dependencies.snapshotSQLite == nil || dependencies.snapshotPostgres == nil {
		return fmt.Errorf("fort db dependencies are unavailable")
	}
	switch args[0] {
	case "export-sqlite":
		return cmdDBExportSQLite(args[1:], dependencies)
	case "import-postgres":
		return cmdDBImportPostgres(args[1:], dependencies)
	case "verify-migration":
		return cmdDBVerifyMigration(args[1:], dependencies)
	case "export-postgres":
		return cmdDBExportPostgres(args[1:], dependencies)
	default:
		return fmt.Errorf("unknown fort db command %q", args[0])
	}
}

func cmdDBExportSQLite(args []string, dependencies dbCommandDependencies) error {
	flags := flag.NewFlagSet("db export-sqlite", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	source := flags.String("source", strings.TrimSpace(dependencies.getenv("FORT_DB")), "path to the frozen SQLite backup")
	output := flags.String("out", "", "restricted encrypted archive path")
	frozen := flags.Bool("frozen", false, "acknowledge writes and scheduler ticks are disabled")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || !*frozen || strings.TrimSpace(*source) == "" || strings.TrimSpace(*output) == "" {
		return fmt.Errorf("usage: fort db export-sqlite --frozen --source PATH --out ARCHIVE")
	}
	key, err := migrationArchiveKey(dependencies.getenv)
	if err != nil {
		return err
	}
	archive, err := dependencies.snapshotSQLite(context.Background(), *source, key)
	if err != nil {
		return err
	}
	if err := writeMigrationArchive(*output, archive, key, dependencies.random); err != nil {
		return err
	}
	return writeDBJSON(dependencies.stdout, map[string]any{
		"operation": "export-sqlite", "archive": *output, "manifest_mac": archive.ManifestMAC,
		"table_count": archive.TableCount, "row_count": archive.RowCount,
	})
}

func cmdDBImportPostgres(args []string, dependencies dbCommandDependencies) error {
	flags := flag.NewFlagSet("db import-postgres", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	archivePath := flags.String("archive", "", "encrypted SQLite migration archive")
	dryRun := flags.Bool("dry-run", false, "classify every row without connecting to Postgres")
	apply := flags.Bool("apply", false, "reserved until a v1-to-v2 mapping contract is approved")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || strings.TrimSpace(*archivePath) == "" {
		return fmt.Errorf("usage: fort db import-postgres --dry-run --archive ARCHIVE")
	}
	if *apply {
		return fmt.Errorf("import-postgres: no approved apply mapping; run --dry-run and resolve every explicit choice")
	}
	if !*dryRun {
		return fmt.Errorf("import-postgres requires --dry-run; no remote apply path is approved")
	}
	key, err := migrationArchiveKey(dependencies.getenv)
	if err != nil {
		return err
	}
	archive, err := readMigrationArchive(*archivePath, key)
	if err != nil {
		return err
	}
	report := migration.PlanPostgresImport(archive)
	if err := writeDBJSON(dependencies.stdout, report); err != nil {
		return err
	}
	return report.RequireResolved()
}

func cmdDBVerifyMigration(args []string, dependencies dbCommandDependencies) error {
	flags := flag.NewFlagSet("db verify-migration", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	sourcePath := flags.String("sqlite-archive", "", "encrypted source SQLite archive")
	targetPath := flags.String("postgres-archive", "", "encrypted target Postgres archive")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || strings.TrimSpace(*sourcePath) == "" || strings.TrimSpace(*targetPath) == "" {
		return fmt.Errorf("usage: fort db verify-migration --sqlite-archive SOURCE --postgres-archive TARGET")
	}
	key, err := migrationArchiveKey(dependencies.getenv)
	if err != nil {
		return err
	}
	source, err := readMigrationArchive(*sourcePath, key)
	if err != nil {
		return err
	}
	target, err := readMigrationArchive(*targetPath, key)
	if err != nil {
		return err
	}
	plan := migration.PlanPostgresImport(source)
	if err := plan.RequireResolved(); err != nil {
		if writeErr := writeDBJSON(dependencies.stdout, plan); writeErr != nil {
			return errors.Join(err, writeErr)
		}
		return err
	}
	report := migration.VerifyMigration(source, target, key)
	if err := writeDBJSON(dependencies.stdout, report); err != nil {
		return err
	}
	if !report.Verified {
		return fmt.Errorf("migration parity verification failed")
	}
	return nil
}

func cmdDBExportPostgres(args []string, dependencies dbCommandDependencies) error {
	flags := flag.NewFlagSet("db export-postgres", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	output := flags.String("out", "", "restricted encrypted rollback archive path")
	frozen := flags.Bool("frozen", false, "acknowledge cloud writes and scheduler ticks are disabled")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || !*frozen || strings.TrimSpace(*output) == "" {
		return fmt.Errorf("usage: fort db export-postgres --frozen --out ARCHIVE")
	}
	databaseURL := strings.TrimSpace(dependencies.getenv("DATABASE_URL"))
	accountID := strings.TrimSpace(dependencies.getenv("FORT_ACCOUNT_ID"))
	if databaseURL == "" || accountID == "" {
		return fmt.Errorf("export-postgres requires DATABASE_URL and FORT_ACCOUNT_ID in the operator environment")
	}
	key, err := migrationArchiveKey(dependencies.getenv)
	if err != nil {
		return err
	}
	archive, err := dependencies.snapshotPostgres(context.Background(), databaseURL, accountID, key)
	if err != nil {
		return err
	}
	if err := writeMigrationArchive(*output, archive, key, dependencies.random); err != nil {
		return err
	}
	return writeDBJSON(dependencies.stdout, map[string]any{
		"operation": "export-postgres", "archive": *output, "manifest_mac": archive.ManifestMAC,
		"table_count": archive.TableCount, "row_count": archive.RowCount,
	})
}

func migrationArchiveKey(getenv func(string) string) ([]byte, error) {
	encoded := strings.TrimSpace(getenv(migrationKeyEnvironment))
	if encoded == "" {
		return nil, fmt.Errorf("%s is required and must be canonical base64url for 32 random bytes", migrationKeyEnvironment)
	}
	key, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || len(key) != migration.ArchiveKeyBytes || base64.RawURLEncoding.EncodeToString(key) != encoded {
		return nil, fmt.Errorf("%s is invalid; expected canonical base64url for 32 random bytes", migrationKeyEnvironment)
	}
	return key, nil
}

func writeMigrationArchive(path string, archive migration.Archive, key []byte, random io.Reader) (resultErr error) {
	sealed, err := migration.SealArchive(archive, key, random)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create migration archive without overwrite: %w", err)
	}
	removeOnFailure := true
	defer func() {
		closeErr := file.Close()
		if resultErr == nil && closeErr != nil {
			resultErr = closeErr
		}
		if removeOnFailure || resultErr != nil {
			_ = os.Remove(path)
		}
	}()
	if _, err := io.Copy(file, bytes.NewReader(sealed)); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	removeOnFailure = false
	return nil
}

func readMigrationArchive(path string, key []byte) (migration.Archive, error) {
	info, err := os.Stat(path)
	if err != nil {
		return migration.Archive{}, fmt.Errorf("migration archive must be a regular file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return migration.Archive{}, fmt.Errorf("migration archive must be a regular file")
	}
	if info.Mode().Perm()&0o077 != 0 {
		return migration.Archive{}, fmt.Errorf("migration archive permissions %o expose operator data; require 0600 or stricter", info.Mode().Perm())
	}
	if info.Size() <= 0 || info.Size() > migration.MaximumArchiveBytes {
		return migration.Archive{}, migration.ErrArchiveInvalid
	}
	file, err := os.Open(path)
	if err != nil {
		return migration.Archive{}, err
	}
	defer file.Close()
	encoded, err := io.ReadAll(io.LimitReader(file, migration.MaximumArchiveBytes+1))
	if err != nil || len(encoded) > migration.MaximumArchiveBytes {
		return migration.Archive{}, migration.ErrArchiveInvalid
	}
	return migration.OpenArchive(encoded, key)
}

func writeDBJSON(destination io.Writer, value any) error {
	encoder := json.NewEncoder(destination)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(value)
}
