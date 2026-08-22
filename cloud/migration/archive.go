// Package migration implements Fort's offline, encrypted SQLite/Postgres
// migration evidence. It deliberately does not apply semantic v1-to-v2
// mappings: unresolved choices are reported and block an import.
package migration

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
)

const (
	ArchiveFormatVersion = 1
	ArchiveKeyBytes      = 32
	MaximumArchiveBytes  = 512 << 20
	sealedArchiveFormat  = "fort-migration-archive"
)

var (
	ErrArchiveInvalid        = errors.New("migration archive is invalid")
	ErrArchiveAuthentication = errors.New("migration archive authentication failed")
	ErrArchiveIntegrity      = errors.New("migration archive integrity failed")
	ErrDatabaseNotQuiescent  = errors.New("migration source database is not quiescent")
	ErrUnresolvedMappings    = errors.New("migration has unresolved mappings")
)

type SourceEngine string

const (
	SourceSQLite   SourceEngine = "sqlite"
	SourcePostgres SourceEngine = "postgres"
)

type ValueKind string

const (
	ValueNull      ValueKind = "null"
	ValueBoolean   ValueKind = "boolean"
	ValueInteger   ValueKind = "integer"
	ValueDecimal   ValueKind = "decimal"
	ValueText      ValueKind = "text"
	ValueBytes     ValueKind = "bytes"
	ValueTimestamp ValueKind = "timestamp"
	ValueJSON      ValueKind = "json"
)

// Value is a lossless, engine-independent database scalar. Bytes use
// canonical raw-URL base64; JSON is canonical compact JSON.
type Value struct {
	Kind ValueKind `json:"kind"`
	Text string    `json:"text,omitempty"`
}

type Column struct {
	Name               string `json:"name"`
	SourceType         string `json:"source_type"`
	NotNull            bool   `json:"not_null,omitempty"`
	PrimaryKeyPosition int    `json:"primary_key_position,omitempty"`
	Identity           bool   `json:"identity,omitempty"`
}

type TimestampBounds struct {
	Minimum string `json:"minimum"`
	Maximum string `json:"maximum"`
}

type Row struct {
	Values []Value `json:"values"`
	Digest string  `json:"digest"`
}

type Table struct {
	Name            string                     `json:"name"`
	Columns         []Column                   `json:"columns"`
	Dependencies    []string                   `json:"dependencies,omitempty"`
	Rows            []Row                      `json:"rows"`
	Count           int64                      `json:"count"`
	SchemaDigest    string                     `json:"schema_digest"`
	Digest          string                     `json:"digest"`
	IdentityMaxima  map[string]string          `json:"identity_maxima,omitempty"`
	TimestampBounds map[string]TimestampBounds `json:"timestamp_bounds,omitempty"`
}

type Archive struct {
	FormatVersion      int          `json:"format_version"`
	SourceEngine       SourceEngine `json:"source_engine"`
	SchemaVersion      string       `json:"schema_version"`
	SourceDatabaseHash string       `json:"source_database_hash"`
	Tables             []Table      `json:"tables"`
	TableCount         int          `json:"table_count"`
	RowCount           int64        `json:"row_count"`
	ManifestMAC        string       `json:"manifest_mac"`
}

type sealedArchive struct {
	Format     string `json:"format"`
	Version    int    `json:"version"`
	KeyID      string `json:"key_id"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

// NewArchive freezes stable HMAC evidence over each logical record, table,
// and the complete manifest. The evidence remains inside the encrypted file.
func NewArchive(engine SourceEngine, schemaVersion, sourceDatabaseHash string, tables []Table, key []byte) (Archive, error) {
	if err := validateArchiveIdentity(engine, schemaVersion, sourceDatabaseHash, key); err != nil {
		return Archive{}, err
	}
	archive := Archive{
		FormatVersion: ArchiveFormatVersion, SourceEngine: engine,
		SchemaVersion: schemaVersion, SourceDatabaseHash: sourceDatabaseHash,
		Tables: cloneTables(tables),
	}
	if err := finalizeArchive(&archive, key); err != nil {
		return Archive{}, err
	}
	return archive, nil
}

func SealArchive(archive Archive, key []byte, random io.Reader) ([]byte, error) {
	if len(key) != ArchiveKeyBytes || archive.FormatVersion != ArchiveFormatVersion {
		return nil, ErrArchiveInvalid
	}
	if random == nil {
		return nil, fmt.Errorf("%w: randomness source is required", ErrArchiveInvalid)
	}
	plaintext, err := json.Marshal(archive)
	if err != nil {
		return nil, fmt.Errorf("encode migration manifest: %w", err)
	}
	if len(plaintext) > MaximumArchiveBytes {
		return nil, fmt.Errorf("%w: manifest exceeds %d bytes", ErrArchiveInvalid, MaximumArchiveBytes)
	}
	aead, err := archiveAEAD(key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(random, nonce); err != nil {
		return nil, fmt.Errorf("generate migration nonce: %w", err)
	}
	keyID := archiveKeyID(key)
	header := sealedArchive{Format: sealedArchiveFormat, Version: ArchiveFormatVersion, KeyID: keyID}
	ciphertext := aead.Seal(nil, nonce, plaintext, sealedArchiveAAD(header))
	header.Nonce = base64.RawURLEncoding.EncodeToString(nonce)
	header.Ciphertext = base64.RawURLEncoding.EncodeToString(ciphertext)
	encoded, err := json.Marshal(header)
	if err != nil {
		return nil, fmt.Errorf("encode sealed migration archive: %w", err)
	}
	if len(encoded) > MaximumArchiveBytes {
		return nil, fmt.Errorf("%w: sealed archive exceeds %d bytes", ErrArchiveInvalid, MaximumArchiveBytes)
	}
	return append(encoded, '\n'), nil
}

func OpenArchive(encoded, key []byte) (Archive, error) {
	if len(key) != ArchiveKeyBytes || len(encoded) == 0 || len(encoded) > MaximumArchiveBytes {
		return Archive{}, ErrArchiveInvalid
	}
	var sealed sealedArchive
	if err := decodeStrictJSON(encoded, &sealed); err != nil || sealed.Format != sealedArchiveFormat ||
		sealed.Version != ArchiveFormatVersion || sealed.KeyID != archiveKeyID(key) {
		return Archive{}, ErrArchiveAuthentication
	}
	aead, err := archiveAEAD(key)
	if err != nil {
		return Archive{}, err
	}
	nonce, err := decodeCanonicalBase64(sealed.Nonce)
	if err != nil || len(nonce) != aead.NonceSize() {
		return Archive{}, ErrArchiveInvalid
	}
	ciphertext, err := decodeCanonicalBase64(sealed.Ciphertext)
	if err != nil || len(ciphertext) < aead.Overhead() {
		return Archive{}, ErrArchiveInvalid
	}
	header := sealed
	header.Nonce, header.Ciphertext = "", ""
	plaintext, err := aead.Open(nil, nonce, ciphertext, sealedArchiveAAD(header))
	if err != nil {
		return Archive{}, ErrArchiveAuthentication
	}
	var archive Archive
	if err := decodeStrictJSON(plaintext, &archive); err != nil {
		return Archive{}, ErrArchiveInvalid
	}
	if err := validateArchiveEvidence(archive, key); err != nil {
		return Archive{}, err
	}
	return archive, nil
}

func finalizeArchive(archive *Archive, key []byte) error {
	seen := make(map[string]struct{}, len(archive.Tables))
	var rowCount int64
	for tableIndex := range archive.Tables {
		table := &archive.Tables[tableIndex]
		if !validName(table.Name) {
			return fmt.Errorf("%w: invalid table name", ErrArchiveInvalid)
		}
		if _, duplicate := seen[table.Name]; duplicate {
			return fmt.Errorf("%w: duplicate table %s", ErrArchiveInvalid, table.Name)
		}
		seen[table.Name] = struct{}{}
		if err := finalizeTable(table, key); err != nil {
			return err
		}
		rowCount += table.Count
	}
	archive.TableCount = len(archive.Tables)
	archive.RowCount = rowCount
	archive.ManifestMAC = ""
	manifestMAC, err := keyedDigest(key, "manifest", archive)
	if err != nil {
		return err
	}
	archive.ManifestMAC = manifestMAC
	return nil
}

func finalizeTable(table *Table, key []byte) error {
	columnNames := make(map[string]struct{}, len(table.Columns))
	for _, column := range table.Columns {
		if !validName(column.Name) || strings.TrimSpace(column.SourceType) == "" || column.PrimaryKeyPosition < 0 {
			return fmt.Errorf("%w: invalid column in %s", ErrArchiveInvalid, table.Name)
		}
		if _, duplicate := columnNames[column.Name]; duplicate {
			return fmt.Errorf("%w: duplicate column %s.%s", ErrArchiveInvalid, table.Name, column.Name)
		}
		columnNames[column.Name] = struct{}{}
	}
	sort.Strings(table.Dependencies)
	table.Dependencies = compactStrings(table.Dependencies)
	for rowIndex := range table.Rows {
		row := &table.Rows[rowIndex]
		if len(row.Values) != len(table.Columns) {
			return fmt.Errorf("%w: row width differs for %s", ErrArchiveInvalid, table.Name)
		}
		for _, value := range row.Values {
			if !validValue(value) {
				return fmt.Errorf("%w: invalid value in %s", ErrArchiveInvalid, table.Name)
			}
		}
		row.Digest = ""
		digest, err := keyedDigest(key, "record", struct {
			Table   string   `json:"table"`
			Columns []Column `json:"columns"`
			Values  []Value  `json:"values"`
		}{table.Name, table.Columns, row.Values})
		if err != nil {
			return err
		}
		row.Digest = digest
	}
	sort.SliceStable(table.Rows, func(left, right int) bool {
		leftJSON, _ := json.Marshal(table.Rows[left].Values)
		rightJSON, _ := json.Marshal(table.Rows[right].Values)
		return bytes.Compare(leftJSON, rightJSON) < 0
	})
	schemaDigest, err := keyedDigest(key, "schema", struct {
		Name         string   `json:"name"`
		Columns      []Column `json:"columns"`
		Dependencies []string `json:"dependencies"`
	}{table.Name, table.Columns, table.Dependencies})
	if err != nil {
		return err
	}
	table.SchemaDigest = schemaDigest
	table.Count = int64(len(table.Rows))
	tableDigest, err := keyedDigest(key, "table", struct {
		Name           string                     `json:"name"`
		SchemaDigest   string                     `json:"schema_digest"`
		RowDigests     []string                   `json:"row_digests"`
		IdentityMaxima map[string]string          `json:"identity_maxima,omitempty"`
		Bounds         map[string]TimestampBounds `json:"timestamp_bounds,omitempty"`
	}{table.Name, table.SchemaDigest, rowDigests(table.Rows), table.IdentityMaxima, table.TimestampBounds})
	if err != nil {
		return err
	}
	table.Digest = tableDigest
	return nil
}

func validateArchiveEvidence(archive Archive, key []byte) error {
	if err := validateArchiveIdentity(archive.SourceEngine, archive.SchemaVersion, archive.SourceDatabaseHash, key); err != nil ||
		archive.FormatVersion != ArchiveFormatVersion {
		return ErrArchiveIntegrity
	}
	expected := archive
	if err := finalizeArchive(&expected, key); err != nil {
		return ErrArchiveIntegrity
	}
	actualJSON, _ := json.Marshal(archive)
	expectedJSON, _ := json.Marshal(expected)
	if subtle.ConstantTimeCompare(actualJSON, expectedJSON) != 1 {
		return ErrArchiveIntegrity
	}
	return nil
}

func validateArchiveIdentity(engine SourceEngine, schemaVersion, sourceDatabaseHash string, key []byte) error {
	if len(key) != ArchiveKeyBytes || (engine != SourceSQLite && engine != SourcePostgres) ||
		strings.TrimSpace(schemaVersion) == "" || !lowerHexDigest(sourceDatabaseHash) {
		return ErrArchiveInvalid
	}
	return nil
}

func keyedDigest(master []byte, domain string, value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, deriveArchiveKey(master, "digest:"+domain))
	_, _ = mac.Write(encoded)
	return hex.EncodeToString(mac.Sum(nil)), nil
}

func deriveArchiveKey(master []byte, label string) []byte {
	mac := hmac.New(sha256.New, master)
	_, _ = mac.Write([]byte("fort-migration-v1:" + label))
	return mac.Sum(nil)
}

func archiveAEAD(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(deriveArchiveKey(key, "encryption"))
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func archiveKeyID(key []byte) string {
	digest := sha256.Sum256(append([]byte("fort-migration-key-id:"), key...))
	return hex.EncodeToString(digest[:16])
}

func sealedArchiveAAD(archive sealedArchive) []byte {
	encoded, _ := json.Marshal(struct {
		Format  string `json:"format"`
		Version int    `json:"version"`
		KeyID   string `json:"key_id"`
	}{archive.Format, archive.Version, archive.KeyID})
	return encoded
}

func decodeStrictJSON(encoded []byte, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return ErrArchiveInvalid
	}
	return nil
}

func decodeCanonicalBase64(value string) ([]byte, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || base64.RawURLEncoding.EncodeToString(decoded) != value {
		return nil, ErrArchiveInvalid
	}
	return decoded, nil
}

func lowerHexDigest(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && hex.EncodeToString(decoded) == value
}

func validName(value string) bool {
	return value != "" && value == strings.TrimSpace(value) && len(value) <= 256 && !strings.ContainsAny(value, "\x00\r\n")
}

func validValue(value Value) bool {
	switch value.Kind {
	case ValueNull:
		return value.Text == ""
	case ValueBoolean:
		return value.Text == "true" || value.Text == "false"
	case ValueInteger, ValueDecimal, ValueText, ValueTimestamp:
		return true
	case ValueBytes:
		_, err := decodeCanonicalBase64(value.Text)
		return err == nil
	case ValueJSON:
		var decoded any
		return json.Unmarshal([]byte(value.Text), &decoded) == nil
	default:
		return false
	}
}

func rowDigests(rows []Row) []string {
	result := make([]string, len(rows))
	for index := range rows {
		result[index] = rows[index].Digest
	}
	return result
}

func compactStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	result := values[:0]
	for _, value := range values {
		if len(result) == 0 || result[len(result)-1] != value {
			result = append(result, value)
		}
	}
	return result
}

func cloneTables(tables []Table) []Table {
	encoded, _ := json.Marshal(tables)
	var cloned []Table
	_ = json.Unmarshal(encoded, &cloned)
	return cloned
}
