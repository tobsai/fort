package migration

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type sqliteTableDefinition struct {
	name         string
	createSQL    string
	columns      []Column
	dependencies []string
}

// SnapshotSQLite opens an already-frozen SQLite backup read-only. It refuses
// active work, WAL sidecars, integrity failures, and any file change observed
// while the snapshot transaction is open.
func SnapshotSQLite(ctx context.Context, path string, key []byte) (Archive, error) {
	if strings.TrimSpace(path) == "" || len(key) != ArchiveKeyBytes {
		return Archive{}, ErrArchiveInvalid
	}
	absolute, err := filepathAbs(path)
	if err != nil {
		return Archive{}, err
	}
	info, err := os.Stat(absolute)
	if err != nil || !info.Mode().IsRegular() {
		return Archive{}, fmt.Errorf("%w: SQLite backup must be a regular file", ErrArchiveInvalid)
	}
	if wal, statErr := os.Stat(absolute + "-wal"); statErr == nil && wal.Size() > 0 {
		return Archive{}, fmt.Errorf("%w: frozen SQLite backup has a non-empty WAL sidecar", ErrArchiveInvalid)
	} else if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return Archive{}, fmt.Errorf("inspect SQLite WAL: %w", statErr)
	}
	beforeHash, err := hashFile(absolute)
	if err != nil {
		return Archive{}, err
	}

	uri := (&url.URL{Scheme: "file", Path: absolute, RawQuery: "mode=ro"}).String()
	database, err := sql.Open("sqlite", uri)
	if err != nil {
		return Archive{}, fmt.Errorf("open frozen SQLite backup: %w", err)
	}
	database.SetMaxOpenConns(1)
	defer database.Close()
	if _, err := database.ExecContext(ctx, `pragma query_only=on`); err != nil {
		return Archive{}, fmt.Errorf("make SQLite snapshot query-only: %w", err)
	}
	tx, err := database.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelSerializable})
	if err != nil {
		return Archive{}, fmt.Errorf("begin SQLite snapshot: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	if err := validateSQLiteIntegrity(ctx, tx); err != nil {
		return Archive{}, err
	}
	if err := validateSQLiteQuiescence(ctx, tx); err != nil {
		return Archive{}, err
	}
	definitions, schemaVersion, err := inspectSQLiteSchema(ctx, tx)
	if err != nil {
		return Archive{}, err
	}
	tables := make([]Table, 0, len(definitions))
	for _, definition := range orderSQLiteTables(definitions) {
		table, err := snapshotSQLiteTable(ctx, tx, definition)
		if err != nil {
			return Archive{}, err
		}
		tables = append(tables, table)
	}
	if err := tx.Commit(); err != nil {
		return Archive{}, fmt.Errorf("commit SQLite snapshot: %w", err)
	}
	committed = true
	afterHash, err := hashFile(absolute)
	if err != nil {
		return Archive{}, err
	}
	if beforeHash != afterHash {
		return Archive{}, fmt.Errorf("%w: SQLite backup changed during export", ErrDatabaseNotQuiescent)
	}
	return NewArchive(SourceSQLite, schemaVersion, beforeHash, tables, key)
}

func validateSQLiteIntegrity(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `pragma integrity_check`)
	if err != nil {
		return fmt.Errorf("SQLite integrity check: %w", err)
	}
	defer rows.Close()
	resultCount := 0
	for rows.Next() {
		var result string
		if err := rows.Scan(&result); err != nil {
			return err
		}
		resultCount++
		if result != "ok" {
			return fmt.Errorf("%w: SQLite integrity check: %s", ErrArchiveIntegrity, result)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if resultCount != 1 {
		return fmt.Errorf("%w: SQLite integrity check returned %d rows", ErrArchiveIntegrity, resultCount)
	}
	foreignKeys, err := tx.QueryContext(ctx, `pragma foreign_key_check`)
	if err != nil {
		return fmt.Errorf("SQLite foreign key check: %w", err)
	}
	defer foreignKeys.Close()
	if foreignKeys.Next() {
		return fmt.Errorf("%w: SQLite foreign key check failed", ErrArchiveIntegrity)
	}
	return foreignKeys.Err()
}

func validateSQLiteQuiescence(ctx context.Context, tx *sql.Tx) error {
	checks := []struct {
		table  string
		column string
		states []string
	}{
		{"conversation_target", "state", []string{"queued", "working"}},
		{"stable_group_initial_target", "state", []string{"queued", "working"}},
		{"stable_handoff_target", "state", []string{"queued", "working"}},
		{"routine_run", "state", []string{"queued", "working"}},
		{"run", "status", []string{"queued", "running"}},
	}
	for _, check := range checks {
		exists, err := sqliteColumnExists(ctx, tx, check.table, check.column)
		if err != nil {
			return err
		}
		if !exists {
			continue
		}
		arguments := make([]any, len(check.states))
		placeholders := make([]string, len(check.states))
		for index, state := range check.states {
			arguments[index], placeholders[index] = state, "?"
		}
		query := `select count(*) from ` + quoteSQLiteIdentifier(check.table) + ` where ` +
			quoteSQLiteIdentifier(check.column) + ` in (` + strings.Join(placeholders, ",") + `)`
		var count int64
		if err := tx.QueryRowContext(ctx, query, arguments...).Scan(&count); err != nil {
			return fmt.Errorf("check SQLite quiescence for %s: %w", check.table, err)
		}
		if count != 0 {
			return fmt.Errorf("%w: %s has %d active rows", ErrDatabaseNotQuiescent, check.table, count)
		}
	}
	return nil
}

func inspectSQLiteSchema(ctx context.Context, tx *sql.Tx) ([]sqliteTableDefinition, string, error) {
	rows, err := tx.QueryContext(ctx, `select name, sql from sqlite_master
where type='table' and name not like 'sqlite_%' order by name`)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	definitions := make([]sqliteTableDefinition, 0)
	for rows.Next() {
		var definition sqliteTableDefinition
		if err := rows.Scan(&definition.name, &definition.createSQL); err != nil {
			return nil, "", err
		}
		definition.columns, err = sqliteColumns(ctx, tx, definition.name)
		if err != nil {
			return nil, "", err
		}
		definition.dependencies, err = sqliteDependencies(ctx, tx, definition.name)
		if err != nil {
			return nil, "", err
		}
		definitions = append(definitions, definition)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	var userVersion, applicationID int64
	if err := tx.QueryRowContext(ctx, `pragma user_version`).Scan(&userVersion); err != nil {
		return nil, "", err
	}
	if err := tx.QueryRowContext(ctx, `pragma application_id`).Scan(&applicationID); err != nil {
		return nil, "", err
	}
	hash := sha256.New()
	for _, definition := range definitions {
		_, _ = io.WriteString(hash, definition.name)
		_, _ = io.WriteString(hash, "\x00")
		_, _ = io.WriteString(hash, definition.createSQL)
		_, _ = io.WriteString(hash, "\x00")
	}
	schemaVersion := fmt.Sprintf("sqlite:user_version:%d:application_id:%d:schema:%s", userVersion, applicationID, hex.EncodeToString(hash.Sum(nil)))
	return definitions, schemaVersion, nil
}

func sqliteColumns(ctx context.Context, tx *sql.Tx, table string) ([]Column, error) {
	rows, err := tx.QueryContext(ctx, `pragma table_info(`+quoteSQLiteIdentifier(table)+`)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns := make([]Column, 0)
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, sourceType string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &sourceType, &notNull, &defaultValue, &primaryKey); err != nil {
			return nil, err
		}
		identity := primaryKey > 0 && strings.Contains(strings.ToUpper(sourceType), "INT")
		columns = append(columns, Column{
			Name: name, SourceType: sourceType, NotNull: notNull != 0,
			PrimaryKeyPosition: primaryKey, Identity: identity,
		})
	}
	return columns, rows.Err()
}

func sqliteDependencies(ctx context.Context, tx *sql.Tx, table string) ([]string, error) {
	rows, err := tx.QueryContext(ctx, `pragma foreign_key_list(`+quoteSQLiteIdentifier(table)+`)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	dependencies := make([]string, 0)
	for rows.Next() {
		var id, sequence int
		var dependency, from, to, onUpdate, onDelete, match string
		if err := rows.Scan(&id, &sequence, &dependency, &from, &to, &onUpdate, &onDelete, &match); err != nil {
			return nil, err
		}
		dependencies = append(dependencies, dependency)
	}
	sort.Strings(dependencies)
	return compactStrings(dependencies), rows.Err()
}

func snapshotSQLiteTable(ctx context.Context, tx *sql.Tx, definition sqliteTableDefinition) (Table, error) {
	columnNames := make([]string, len(definition.columns))
	for index := range definition.columns {
		columnNames[index] = quoteSQLiteIdentifier(definition.columns[index].Name)
	}
	rows, err := tx.QueryContext(ctx, `select `+strings.Join(columnNames, ",")+` from `+quoteSQLiteIdentifier(definition.name))
	if err != nil {
		return Table{}, fmt.Errorf("snapshot SQLite table %s: %w", definition.name, err)
	}
	defer rows.Close()
	table := Table{
		Name: definition.name, Columns: definition.columns, Dependencies: definition.dependencies,
		Rows: make([]Row, 0), IdentityMaxima: make(map[string]string), TimestampBounds: make(map[string]TimestampBounds),
	}
	for rows.Next() {
		values := make([]any, len(definition.columns))
		destinations := make([]any, len(values))
		for index := range values {
			destinations[index] = &values[index]
		}
		if err := rows.Scan(destinations...); err != nil {
			return Table{}, err
		}
		row := Row{Values: make([]Value, len(values))}
		for index, raw := range values {
			value, err := sqliteValue(raw)
			if err != nil {
				return Table{}, fmt.Errorf("snapshot %s.%s: %w", definition.name, definition.columns[index].Name, err)
			}
			row.Values[index] = value
			observeSQLiteEvidence(&table, definition.columns[index], value)
		}
		table.Rows = append(table.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return Table{}, err
	}
	if len(table.IdentityMaxima) == 0 {
		table.IdentityMaxima = nil
	}
	if len(table.TimestampBounds) == 0 {
		table.TimestampBounds = nil
	}
	return table, nil
}

func sqliteValue(raw any) (Value, error) {
	switch value := raw.(type) {
	case nil:
		return Value{Kind: ValueNull}, nil
	case int64:
		return Value{Kind: ValueInteger, Text: strconv.FormatInt(value, 10)}, nil
	case float64:
		if math.IsInf(value, 0) || math.IsNaN(value) {
			return Value{}, fmt.Errorf("non-finite floating point value")
		}
		return Value{Kind: ValueDecimal, Text: strconv.FormatFloat(value, 'g', -1, 64)}, nil
	case bool:
		return Value{Kind: ValueBoolean, Text: strconv.FormatBool(value)}, nil
	case string:
		return Value{Kind: ValueText, Text: value}, nil
	case []byte:
		return Value{Kind: ValueBytes, Text: base64.RawURLEncoding.EncodeToString(value)}, nil
	case time.Time:
		return Value{Kind: ValueTimestamp, Text: value.UTC().Format(time.RFC3339Nano)}, nil
	default:
		return Value{}, fmt.Errorf("unsupported SQLite value type %T", raw)
	}
}

func observeSQLiteEvidence(table *Table, column Column, value Value) {
	if column.Identity && value.Kind == ValueInteger {
		current, exists := table.IdentityMaxima[column.Name]
		if !exists || compareDecimalInteger(value.Text, current) > 0 {
			table.IdentityMaxima[column.Name] = value.Text
		}
	}
	if !strings.HasSuffix(column.Name, "_at") || (value.Kind != ValueText && value.Kind != ValueTimestamp) || value.Text == "" {
		return
	}
	parsed, err := time.Parse(time.RFC3339Nano, value.Text)
	if err != nil {
		return
	}
	observeTimestampEvidence(table, column.Name, parsed)
}

func observeTimestampEvidence(table *Table, columnName string, observed time.Time) {
	normalized := observed.UTC().Format(time.RFC3339Nano)
	observed = observed.UTC()
	bound, exists := table.TimestampBounds[columnName]
	minimum, minimumErr := time.Parse(time.RFC3339Nano, bound.Minimum)
	maximum, maximumErr := time.Parse(time.RFC3339Nano, bound.Maximum)
	if !exists || minimumErr != nil || observed.Before(minimum) {
		bound.Minimum = normalized
	}
	if !exists || maximumErr != nil || observed.After(maximum) {
		bound.Maximum = normalized
	}
	table.TimestampBounds[columnName] = bound
}

func orderSQLiteTables(definitions []sqliteTableDefinition) []sqliteTableDefinition {
	byName := make(map[string]sqliteTableDefinition, len(definitions))
	indegree := make(map[string]int, len(definitions))
	children := make(map[string][]string, len(definitions))
	for _, definition := range definitions {
		byName[definition.name] = definition
		indegree[definition.name] = 0
	}
	for _, definition := range definitions {
		for _, dependency := range definition.dependencies {
			if _, exists := byName[dependency]; !exists || dependency == definition.name {
				continue
			}
			indegree[definition.name]++
			children[dependency] = append(children[dependency], definition.name)
		}
	}
	result := make([]sqliteTableDefinition, 0, len(definitions))
	used := make(map[string]bool, len(definitions))
	for len(result) < len(definitions) {
		ready := make([]string, 0)
		for name, degree := range indegree {
			if degree == 0 && !used[name] {
				ready = append(ready, name)
			}
		}
		if len(ready) == 0 {
			for name := range byName {
				if !used[name] {
					ready = append(ready, name)
				}
			}
		}
		sort.Slice(ready, func(left, right int) bool {
			leftChildren, rightChildren := len(children[ready[left]]), len(children[ready[right]])
			if leftChildren != rightChildren {
				return leftChildren > rightChildren
			}
			return ready[left] < ready[right]
		})
		selected := ready[0]
		used[selected] = true
		result = append(result, byName[selected])
		for _, child := range children[selected] {
			indegree[child]--
		}
	}
	return result
}

func sqliteColumnExists(ctx context.Context, tx *sql.Tx, table, column string) (bool, error) {
	var count int
	err := tx.QueryRowContext(ctx, `select count(*) from pragma_table_info(?) where name=?`, table, column).Scan(&count)
	return count == 1, err
}

func quoteSQLiteIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func compareDecimalInteger(left, right string) int {
	left = strings.TrimLeft(left, "0")
	right = strings.TrimLeft(right, "0")
	if len(left) != len(right) {
		if len(left) < len(right) {
			return -1
		}
		return 1
	}
	return strings.Compare(left, right)
}

func hashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

var filepathAbs = func(path string) (string, error) {
	return filepath.Abs(path)
}
