package migration

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type postgresSnapshotSource interface {
	validateQuiescence(context.Context) error
	snapshot(context.Context) (string, []Table, error)
	commit(context.Context) error
	rollback(context.Context) error
}

type postgresSnapshotOpener func(context.Context, string, string) (postgresSnapshotSource, error)

// SnapshotPostgres exports one account through a direct migration-operator
// connection. The session is repeatable-read and read-only; it never uses the
// runtime Store and cannot become a write path for a client or worker.
func SnapshotPostgres(ctx context.Context, databaseURL, accountID string, key []byte) (Archive, error) {
	return snapshotPostgres(ctx, databaseURL, accountID, key, openPostgresSnapshot)
}

func snapshotPostgres(ctx context.Context, databaseURL, accountID string, key []byte, open postgresSnapshotOpener) (Archive, error) {
	if strings.TrimSpace(databaseURL) == "" || len(key) != ArchiveKeyBytes || open == nil {
		return Archive{}, ErrArchiveInvalid
	}
	parsed, err := uuid.Parse(strings.TrimSpace(accountID))
	if err != nil || parsed.String() != strings.TrimSpace(accountID) {
		return Archive{}, fmt.Errorf("%w: Postgres account id must be a canonical UUID", ErrArchiveInvalid)
	}
	source, err := open(ctx, databaseURL, parsed.String())
	if err != nil {
		return Archive{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = source.rollback(ctx)
		}
	}()
	if err := source.validateQuiescence(ctx); err != nil {
		return Archive{}, err
	}
	schemaVersion, tables, err := source.snapshot(ctx)
	if err != nil {
		return Archive{}, err
	}
	databaseHash, err := canonicalPostgresDatabaseHash(schemaVersion, tables)
	if err != nil {
		return Archive{}, err
	}
	archive, err := NewArchive(SourcePostgres, schemaVersion, databaseHash, tables, key)
	if err != nil {
		return Archive{}, err
	}
	if err := source.commit(ctx); err != nil {
		return Archive{}, fmt.Errorf("commit Postgres export snapshot: %w", err)
	}
	committed = true
	return archive, nil
}

// canonicalPostgresDatabaseHash gives a logical snapshot (which has no
// physical database file) a deterministic source hash. Query row order and
// dependency-order tie breaking cannot change it.
func canonicalPostgresDatabaseHash(schemaVersion string, tables []Table) (string, error) {
	type canonicalTable struct {
		Name         string    `json:"name"`
		Columns      []Column  `json:"columns"`
		Dependencies []string  `json:"dependencies,omitempty"`
		Rows         [][]Value `json:"rows"`
	}
	canonicalTables := make([]canonicalTable, len(tables))
	for tableIndex, table := range tables {
		canonical := canonicalTable{
			Name: table.Name, Columns: append([]Column(nil), table.Columns...),
			Dependencies: append([]string(nil), table.Dependencies...), Rows: make([][]Value, len(table.Rows)),
		}
		sort.Strings(canonical.Dependencies)
		for rowIndex, row := range table.Rows {
			canonical.Rows[rowIndex] = append([]Value(nil), row.Values...)
		}
		sort.Slice(canonical.Rows, func(left, right int) bool {
			leftJSON, _ := json.Marshal(canonical.Rows[left])
			rightJSON, _ := json.Marshal(canonical.Rows[right])
			return string(leftJSON) < string(rightJSON)
		})
		canonicalTables[tableIndex] = canonical
	}
	sort.Slice(canonicalTables, func(left, right int) bool { return canonicalTables[left].Name < canonicalTables[right].Name })
	encoded, err := json.Marshal(struct {
		SchemaVersion string           `json:"schema_version"`
		Tables        []canonicalTable `json:"tables"`
	}{schemaVersion, canonicalTables})
	if err != nil {
		return "", fmt.Errorf("encode canonical Postgres snapshot: %w", err)
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

type pgSnapshot struct {
	connection  *pgx.Conn
	transaction pgx.Tx
	accountID   string
}

func openPostgresSnapshot(ctx context.Context, databaseURL, accountID string) (postgresSnapshotSource, error) {
	config, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse Postgres migration URL: %w", err)
	}
	config.DefaultQueryExecMode = pgx.QueryExecModeExec
	config.StatementCacheCapacity = 0
	config.DescriptionCacheCapacity = 0
	connection, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("connect Postgres migration source: %w", err)
	}
	transaction, err := connection.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		_ = connection.Close(ctx)
		return nil, fmt.Errorf("begin read-only Postgres snapshot: %w", err)
	}
	if _, err := transaction.Exec(ctx, `select set_config('fort.account_id', $1, true)`, accountID); err != nil {
		_ = transaction.Rollback(ctx)
		_ = connection.Close(ctx)
		return nil, fmt.Errorf("scope Postgres snapshot: %w", err)
	}
	return &pgSnapshot{connection: connection, transaction: transaction, accountID: accountID}, nil
}

func (source *pgSnapshot) validateQuiescence(ctx context.Context) error {
	var activeTargets, activeLeases, activeAttempts int64
	err := source.transaction.QueryRow(ctx, `select
  (select count(*) from fort_private.conversation_target
    where account_id = $1 and state in ('queued', 'claimed', 'working', 'cancel_requested')),
  (select count(*) from fort_private.worker_lease
    where account_id = $1 and state = 'active'),
  (select count(*) from fort_private.execution_attempt
    where account_id = $1 and state in ('queued', 'leased', 'working', 'cancel_requested'))`, source.accountID).Scan(
		&activeTargets, &activeLeases, &activeAttempts,
	)
	if err != nil {
		return fmt.Errorf("check Postgres export quiescence: %w", err)
	}
	if activeTargets != 0 || activeLeases != 0 || activeAttempts != 0 {
		return fmt.Errorf("%w: targets=%d leases=%d attempts=%d", ErrDatabaseNotQuiescent, activeTargets, activeLeases, activeAttempts)
	}
	return nil
}

type postgresTableDefinition struct {
	name         string
	columns      []Column
	dependencies []string
}

func (source *pgSnapshot) snapshot(ctx context.Context) (string, []Table, error) {
	definitions, migrationVersion, schemaHash, err := source.inspectSchema(ctx)
	if err != nil {
		return "", nil, err
	}
	tables := make([]Table, 0, len(definitions))
	for _, definition := range orderPostgresTables(definitions) {
		table, err := source.snapshotTable(ctx, definition)
		if err != nil {
			return "", nil, err
		}
		tables = append(tables, table)
	}
	return "postgres:migration:" + migrationVersion + ":schema:" + schemaHash, tables, nil
}

func (source *pgSnapshot) inspectSchema(ctx context.Context) ([]postgresTableDefinition, string, string, error) {
	rows, err := source.transaction.Query(ctx, `select
  relation.relname,
  attribute.attname,
  pg_catalog.format_type(attribute.atttypid, attribute.atttypmod),
  attribute.attnotnull,
  coalesce(array_position(primary_key.indkey::smallint[], attribute.attnum), 0),
  attribute.attidentity <> ''
from pg_catalog.pg_class as relation
join pg_catalog.pg_namespace as namespace on namespace.oid = relation.relnamespace
join pg_catalog.pg_attribute as attribute on attribute.attrelid = relation.oid
left join pg_catalog.pg_index as primary_key
  on primary_key.indrelid = relation.oid and primary_key.indisprimary
where namespace.nspname = 'fort_private'
  and relation.relkind in ('r', 'p')
  and attribute.attnum > 0 and not attribute.attisdropped
order by relation.relname, attribute.attnum`)
	if err != nil {
		return nil, "", "", fmt.Errorf("inspect Postgres schema: %w", err)
	}
	defer rows.Close()
	byName := make(map[string]*postgresTableDefinition)
	order := make([]string, 0)
	for rows.Next() {
		var tableName, columnName, sourceType string
		var notNull, identity bool
		var primaryKeyPosition int
		if err := rows.Scan(&tableName, &columnName, &sourceType, &notNull, &primaryKeyPosition, &identity); err != nil {
			return nil, "", "", err
		}
		definition := byName[tableName]
		if definition == nil {
			definition = &postgresTableDefinition{name: tableName}
			byName[tableName] = definition
			order = append(order, tableName)
		}
		definition.columns = append(definition.columns, Column{
			Name: columnName, SourceType: sourceType, NotNull: notNull,
			PrimaryKeyPosition: primaryKeyPosition, Identity: identity,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, "", "", err
	}
	dependencyRows, err := source.transaction.Query(ctx, `select child.relname, parent.relname
from pg_catalog.pg_constraint as foreign_key
join pg_catalog.pg_class as child on child.oid = foreign_key.conrelid
join pg_catalog.pg_namespace as child_namespace on child_namespace.oid = child.relnamespace
join pg_catalog.pg_class as parent on parent.oid = foreign_key.confrelid
join pg_catalog.pg_namespace as parent_namespace on parent_namespace.oid = parent.relnamespace
where foreign_key.contype = 'f'
  and child_namespace.nspname = 'fort_private'
  and parent_namespace.nspname = 'fort_private'
order by child.relname, parent.relname`)
	if err != nil {
		return nil, "", "", err
	}
	for dependencyRows.Next() {
		var child, parent string
		if err := dependencyRows.Scan(&child, &parent); err != nil {
			dependencyRows.Close()
			return nil, "", "", err
		}
		if child != parent && byName[child] != nil {
			byName[child].dependencies = append(byName[child].dependencies, parent)
		}
	}
	if err := dependencyRows.Err(); err != nil {
		dependencyRows.Close()
		return nil, "", "", err
	}
	dependencyRows.Close()
	definitions := make([]postgresTableDefinition, 0, len(order))
	for _, name := range order {
		definition := *byName[name]
		sort.Strings(definition.dependencies)
		definition.dependencies = compactStrings(definition.dependencies)
		definitions = append(definitions, definition)
	}
	var migrationVersion string
	if err := source.transaction.QueryRow(ctx, `select coalesce(max(version), '') from supabase_migrations.schema_migrations`).Scan(&migrationVersion); err != nil {
		return nil, "", "", fmt.Errorf("read Supabase migration version: %w", err)
	}
	if strings.TrimSpace(migrationVersion) == "" {
		return nil, "", "", fmt.Errorf("Supabase migration version is unavailable")
	}
	schemaJSON, _ := json.Marshal(definitions)
	schemaDigest := sha256.Sum256(schemaJSON)
	return definitions, migrationVersion, hex.EncodeToString(schemaDigest[:]), nil
}

func (source *pgSnapshot) snapshotTable(ctx context.Context, definition postgresTableDefinition) (Table, error) {
	accountColumn := false
	columnNames := make([]string, len(definition.columns))
	for index, column := range definition.columns {
		columnNames[index] = pgx.Identifier{column.Name}.Sanitize()
		accountColumn = accountColumn || column.Name == "account_id"
	}
	if !accountColumn {
		return Table{}, fmt.Errorf("%w: fort_private.%s has no account_id", ErrArchiveInvalid, definition.name)
	}
	query := `select ` + strings.Join(columnNames, ",") + ` from ` +
		pgx.Identifier{"fort_private", definition.name}.Sanitize() + ` where account_id = $1`
	rows, err := source.transaction.Query(ctx, query, source.accountID)
	if err != nil {
		return Table{}, fmt.Errorf("snapshot Postgres table %s: %w", definition.name, err)
	}
	defer rows.Close()
	table := Table{
		Name: definition.name, Columns: definition.columns, Dependencies: definition.dependencies,
		Rows: make([]Row, 0), IdentityMaxima: make(map[string]string), TimestampBounds: make(map[string]TimestampBounds),
	}
	for rows.Next() {
		rawValues, err := rows.Values()
		if err != nil {
			return Table{}, err
		}
		if len(rawValues) != len(definition.columns) {
			return Table{}, fmt.Errorf("Postgres row width differs for %s", definition.name)
		}
		row := Row{Values: make([]Value, len(rawValues))}
		for index, raw := range rawValues {
			value, err := postgresValue(raw, definition.columns[index].SourceType)
			if err != nil {
				return Table{}, fmt.Errorf("snapshot %s.%s: %w", definition.name, definition.columns[index].Name, err)
			}
			row.Values[index] = value
			observePostgresEvidence(&table, definition.columns[index], value)
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

func (source *pgSnapshot) commit(ctx context.Context) error {
	err := source.transaction.Commit(ctx)
	return errors.Join(err, source.connection.Close(ctx))
}

func (source *pgSnapshot) rollback(ctx context.Context) error {
	err := source.transaction.Rollback(ctx)
	if errors.Is(err, pgx.ErrTxClosed) {
		err = nil
	}
	return errors.Join(err, source.connection.Close(ctx))
}

func postgresValue(raw any, sourceType string) (Value, error) {
	if raw == nil {
		return Value{Kind: ValueNull}, nil
	}
	normalizedType := strings.ToLower(sourceType)
	if normalizedType == "json" || normalizedType == "jsonb" {
		var decoded any
		switch value := raw.(type) {
		case []byte:
			if err := json.Unmarshal(value, &decoded); err != nil {
				return Value{}, err
			}
		case string:
			if err := json.Unmarshal([]byte(value), &decoded); err != nil {
				return Value{}, err
			}
		default:
			decoded = value
		}
		canonical, err := json.Marshal(decoded)
		if err != nil {
			return Value{}, err
		}
		return Value{Kind: ValueJSON, Text: string(canonical)}, nil
	}
	switch value := raw.(type) {
	case bool:
		return Value{Kind: ValueBoolean, Text: strconv.FormatBool(value)}, nil
	case int:
		return Value{Kind: ValueInteger, Text: strconv.Itoa(value)}, nil
	case int8:
		return Value{Kind: ValueInteger, Text: strconv.FormatInt(int64(value), 10)}, nil
	case int16:
		return Value{Kind: ValueInteger, Text: strconv.FormatInt(int64(value), 10)}, nil
	case int32:
		return Value{Kind: ValueInteger, Text: strconv.FormatInt(int64(value), 10)}, nil
	case int64:
		return Value{Kind: ValueInteger, Text: strconv.FormatInt(value, 10)}, nil
	case uint:
		return Value{Kind: ValueInteger, Text: strconv.FormatUint(uint64(value), 10)}, nil
	case uint8:
		return Value{Kind: ValueInteger, Text: strconv.FormatUint(uint64(value), 10)}, nil
	case uint16:
		return Value{Kind: ValueInteger, Text: strconv.FormatUint(uint64(value), 10)}, nil
	case uint32:
		return Value{Kind: ValueInteger, Text: strconv.FormatUint(uint64(value), 10)}, nil
	case uint64:
		return Value{Kind: ValueInteger, Text: strconv.FormatUint(value, 10)}, nil
	case float32:
		if math.IsInf(float64(value), 0) || math.IsNaN(float64(value)) {
			return Value{}, fmt.Errorf("non-finite Postgres float")
		}
		return Value{Kind: ValueDecimal, Text: strconv.FormatFloat(float64(value), 'g', -1, 32)}, nil
	case float64:
		if math.IsInf(value, 0) || math.IsNaN(value) {
			return Value{}, fmt.Errorf("non-finite Postgres float")
		}
		return Value{Kind: ValueDecimal, Text: strconv.FormatFloat(value, 'g', -1, 64)}, nil
	case time.Time:
		return Value{Kind: ValueTimestamp, Text: value.UTC().Format(time.RFC3339Nano)}, nil
	case []byte:
		return Value{Kind: ValueBytes, Text: base64.RawURLEncoding.EncodeToString(value)}, nil
	case string:
		if strings.Contains(normalizedType, "timestamp") {
			parsed, err := time.Parse(time.RFC3339Nano, value)
			if err != nil {
				return Value{}, err
			}
			return Value{Kind: ValueTimestamp, Text: parsed.UTC().Format(time.RFC3339Nano)}, nil
		}
		return Value{Kind: ValueText, Text: value}, nil
	case uuid.UUID:
		return Value{Kind: ValueText, Text: value.String()}, nil
	}
	if stringer, ok := raw.(fmt.Stringer); ok {
		return Value{Kind: ValueText, Text: stringer.String()}, nil
	}
	reflected := reflect.ValueOf(raw)
	if normalizedType == "uuid" && reflected.Kind() == reflect.Array && reflected.Len() == 16 {
		bytes := make([]byte, 16)
		for index := 0; index < 16; index++ {
			bytes[index] = byte(reflected.Index(index).Uint())
		}
		parsed, err := uuid.FromBytes(bytes)
		if err != nil {
			return Value{}, err
		}
		return Value{Kind: ValueText, Text: parsed.String()}, nil
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return Value{}, fmt.Errorf("unsupported Postgres value type %T", raw)
	}
	var decoded any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		return Value{}, err
	}
	canonical, _ := json.Marshal(decoded)
	return Value{Kind: ValueJSON, Text: string(canonical)}, nil
}

func observePostgresEvidence(table *Table, column Column, value Value) {
	if column.Identity && value.Kind == ValueInteger {
		current, exists := table.IdentityMaxima[column.Name]
		if !exists || compareDecimalInteger(value.Text, current) > 0 {
			table.IdentityMaxima[column.Name] = value.Text
		}
	}
	if value.Kind != ValueTimestamp || value.Text == "" {
		return
	}
	observed, err := time.Parse(time.RFC3339Nano, value.Text)
	if err != nil {
		return
	}
	observeTimestampEvidence(table, column.Name, observed)
}

func orderPostgresTables(definitions []postgresTableDefinition) []postgresTableDefinition {
	byName := make(map[string]postgresTableDefinition, len(definitions))
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
	result := make([]postgresTableDefinition, 0, len(definitions))
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
