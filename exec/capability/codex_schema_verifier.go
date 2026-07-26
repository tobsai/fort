package capability

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"golang.org/x/sys/unix"

	corecap "github.com/tobsai/fort/core/capability"
)

const (
	codexSchemaDomain           = "fort.codex-schema-bundle.v1"
	codexSchemaCommandOutputMax = 64 << 10
	codexSchemaEntryMax         = 2048
	codexSchemaFileMax          = 512
	codexSchemaPathBytesMax     = 4096
	codexSchemaSingleFileMax    = 8 << 20
	codexSchemaBundleBytesMax   = 16 << 20
)

// CodexSchemaGenerator is the bounded command seam for generating both schema
// bundles from one executable identity.
type CodexSchemaGenerator interface {
	Generate(context.Context, string, string) (string, error)
}

// ResolverCodexSchemaGenerator runs both schema commands from one held Codex
// executable identity. Command output is bounded and discarded.
type ResolverCodexSchemaGenerator struct {
	Resolver *CommandResolver
}

func (g ResolverCodexSchemaGenerator) Generate(ctx context.Context, normalDir, experimentalDir string) (string, error) {
	if g.Resolver == nil {
		return "", ErrCommandAbsent
	}
	held, err := g.Resolver.Hold("codex")
	if err != nil {
		return "", err
	}
	commands := [][]string{
		{"app-server", "generate-json-schema", "--out", normalDir},
		{"app-server", "generate-json-schema", "--experimental", "--out", experimentalDir},
	}
	for _, arguments := range commands {
		if _, err := held.Run(ctx, arguments, codexSchemaCommandOutputMax, probeTimeout); err != nil {
			return "", err
		}
	}
	return held.Digest(), nil
}

type codexBundleExpectation struct {
	digest string
	files  int
}

type codexSchemaExpectation struct {
	normal       codexBundleExpectation
	experimental codexBundleExpectation
}

var codexSchemaV1Expectation = codexSchemaExpectation{
	normal: codexBundleExpectation{
		digest: codexNormalSchemaDigest,
		files:  267,
	},
	experimental: codexBundleExpectation{
		digest: codexExperimentalSchemaDigest,
		files:  337,
	},
}

type CodexSchemaContractVerifier struct {
	generator   CodexSchemaGenerator
	expectation codexSchemaExpectation
}

// NewCodexSchemaContractVerifier constructs the catalog-v1 production
// verifier. Verify must succeed before its contract is supplied to the live
// app-server inspector.
func NewCodexSchemaContractVerifier(resolver *CommandResolver) *CodexSchemaContractVerifier {
	return newCodexSchemaContractVerifier(ResolverCodexSchemaGenerator{Resolver: resolver}, codexSchemaV1Expectation)
}

func newCodexSchemaContractVerifier(generator CodexSchemaGenerator, expectation codexSchemaExpectation) *CodexSchemaContractVerifier {
	return &CodexSchemaContractVerifier{generator: generator, expectation: expectation}
}

func (v *CodexSchemaContractVerifier) Verify(ctx context.Context) (CodexAppServerContract, error) {
	if err := ctx.Err(); err != nil {
		return CodexAppServerContract{}, err
	}
	if v == nil || v.generator == nil {
		return CodexAppServerContract{}, &ProbeError{Reason: corecap.ReasonIncompatibleVersion}
	}
	verifyCtx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	root, err := os.MkdirTemp("", ".fort-codex-schema-")
	if err != nil {
		return CodexAppServerContract{}, &ProbeError{Reason: corecap.ReasonProbeFailed}
	}
	defer os.RemoveAll(root)
	if err := os.Chmod(root, 0o700); err != nil {
		return CodexAppServerContract{}, &ProbeError{Reason: corecap.ReasonProbeFailed}
	}
	normalDir := filepath.Join(root, "normal")
	experimentalDir := filepath.Join(root, "experimental")
	if err := os.Mkdir(normalDir, 0o700); err != nil {
		return CodexAppServerContract{}, &ProbeError{Reason: corecap.ReasonProbeFailed}
	}
	if err := os.Mkdir(experimentalDir, 0o700); err != nil {
		return CodexAppServerContract{}, &ProbeError{Reason: corecap.ReasonProbeFailed}
	}
	executableDigest, err := v.generator.Generate(verifyCtx, normalDir, experimentalDir)
	if err != nil {
		return CodexAppServerContract{}, codexSchemaCommandError(verifyCtx, err)
	}
	if executableDigest == "" {
		return CodexAppServerContract{}, &ProbeError{Reason: corecap.ReasonIncompatibleVersion}
	}
	normal, err := codexSchemaBundleIdentity(normalDir)
	if err != nil {
		return CodexAppServerContract{}, &ProbeError{Reason: corecap.ReasonIncompatibleVersion}
	}
	experimental, err := codexSchemaBundleIdentity(experimentalDir)
	if err != nil {
		return CodexAppServerContract{}, &ProbeError{Reason: corecap.ReasonIncompatibleVersion}
	}
	if normal != v.expectation.normal || experimental != v.expectation.experimental {
		return CodexAppServerContract{}, &ProbeError{Reason: corecap.ReasonIncompatibleVersion}
	}
	return CodexAppServerContract{
		ExecutableDigest:         executableDigest,
		NormalSchemaDigest:       normal.digest,
		NormalSchemaFiles:        normal.files,
		ExperimentalSchemaDigest: experimental.digest,
		ExperimentalSchemaFiles:  experimental.files,
		GmailIsolationReady:      false,
		SupabaseIsolationReady:   false,
	}, nil
}

func codexSchemaCommandError(ctx context.Context, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return &ProbeError{Reason: corecap.ReasonProbeTimedOut}
	}
	observation := commandFailure(err)
	reason := observation.Reason
	if reason == "" {
		reason = corecap.ReasonProbeFailed
	}
	return &ProbeError{Reason: reason}
}

type codexSchemaFile struct {
	rel  string
	path string
}

func codexSchemaBundleIdentity(root string) (codexBundleExpectation, error) {
	files := make([]codexSchemaFile, 0, 384)
	entries := 0
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		entries++
		if entries > codexSchemaEntryMax {
			return fmt.Errorf("schema entry bound exceeded")
		}
		if path == root {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || (!info.IsDir() && !info.Mode().IsRegular()) {
			return fmt.Errorf("unsupported schema entry")
		}
		if info.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if relative == "." || strings.HasPrefix(relative, "../") || !utf8.ValidString(relative) || len(relative) > codexSchemaPathBytesMax {
			return fmt.Errorf("invalid schema path")
		}
		files = append(files, codexSchemaFile{rel: relative, path: path})
		if len(files) > codexSchemaFileMax {
			return fmt.Errorf("schema file bound exceeded")
		}
		return nil
	})
	if err != nil || len(files) == 0 {
		return codexBundleExpectation{}, fmt.Errorf("invalid schema bundle")
	}
	sort.Slice(files, func(left, right int) bool {
		return bytes.Compare([]byte(files[left].rel), []byte(files[right].rel)) < 0
	})

	hash := sha256.New()
	_, _ = hash.Write([]byte(codexSchemaDomain))
	_, _ = hash.Write([]byte{0})
	totalBytes := int64(0)
	var length [8]byte
	for _, schemaFile := range files {
		raw, err := readBoundedSchemaFile(schemaFile.path)
		if err != nil {
			return codexBundleExpectation{}, err
		}
		totalBytes += int64(len(raw))
		if totalBytes > codexSchemaBundleBytesMax {
			return codexBundleExpectation{}, fmt.Errorf("schema bundle bound exceeded")
		}
		canonical, err := canonicalizeCodexSchemaJSON(raw)
		if err != nil {
			return codexBundleExpectation{}, err
		}
		binary.BigEndian.PutUint64(length[:], uint64(len(schemaFile.rel)))
		_, _ = hash.Write(length[:])
		_, _ = hash.Write([]byte(schemaFile.rel))
		binary.BigEndian.PutUint64(length[:], uint64(len(canonical)))
		_, _ = hash.Write(length[:])
		_, _ = hash.Write(canonical)
	}
	return codexBundleExpectation{digest: hex.EncodeToString(hash.Sum(nil)), files: len(files)}, nil
}

func readBoundedSchemaFile(path string) ([]byte, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), "codex-schema")
	defer file.Close()
	before, err := file.Stat()
	if err != nil || !before.Mode().IsRegular() || before.Size() < 0 || before.Size() > codexSchemaSingleFileMax {
		return nil, fmt.Errorf("invalid schema file")
	}
	raw, err := io.ReadAll(io.LimitReader(file, codexSchemaSingleFileMax+1))
	if err != nil || len(raw) > codexSchemaSingleFileMax {
		return nil, fmt.Errorf("invalid schema file")
	}
	after, err := file.Stat()
	if err != nil || after.Size() != before.Size() || int64(len(raw)) != before.Size() {
		return nil, fmt.Errorf("schema file changed")
	}
	return raw, nil
}

func canonicalizeCodexSchemaJSON(raw []byte) ([]byte, error) {
	if !utf8.Valid(raw) {
		return nil, fmt.Errorf("invalid schema JSON")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	value, err := readCanonicalJSONValue(decoder)
	if err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("invalid trailing schema JSON")
	}
	var output bytes.Buffer
	if err := writeCodexCanonicalJSON(&output, value); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func readCanonicalJSONValue(decoder *json.Decoder) (any, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	delimiter, isDelimiter := token.(json.Delim)
	if !isDelimiter {
		switch token.(type) {
		case nil, bool, string, json.Number:
			return token, nil
		default:
			return nil, fmt.Errorf("unsupported schema JSON value")
		}
	}
	switch delimiter {
	case '{':
		object := make(map[string]any)
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return nil, err
			}
			key, ok := keyToken.(string)
			if !ok {
				return nil, fmt.Errorf("invalid schema JSON key")
			}
			if _, duplicate := object[key]; duplicate {
				return nil, fmt.Errorf("duplicate schema JSON key")
			}
			value, err := readCanonicalJSONValue(decoder)
			if err != nil {
				return nil, err
			}
			object[key] = value
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return nil, fmt.Errorf("invalid schema JSON object")
		}
		return object, nil
	case '[':
		array := make([]any, 0)
		for decoder.More() {
			value, err := readCanonicalJSONValue(decoder)
			if err != nil {
				return nil, err
			}
			array = append(array, value)
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim(']') {
			return nil, fmt.Errorf("invalid schema JSON array")
		}
		return array, nil
	default:
		return nil, fmt.Errorf("invalid schema JSON delimiter")
	}
}

func writeCodexCanonicalJSON(output *bytes.Buffer, value any) error {
	switch value := value.(type) {
	case nil:
		output.WriteString("null")
	case bool:
		if value {
			output.WriteString("true")
		} else {
			output.WriteString("false")
		}
	case string:
		encoded, _ := json.Marshal(value)
		encoded = bytes.ReplaceAll(encoded, []byte(`\u003c`), []byte("<"))
		encoded = bytes.ReplaceAll(encoded, []byte(`\u003e`), []byte(">"))
		encoded = bytes.ReplaceAll(encoded, []byte(`\u0026`), []byte("&"))
		encoded = bytes.ReplaceAll(encoded, []byte(`\u2028`), []byte(string(rune(0x2028))))
		encoded = bytes.ReplaceAll(encoded, []byte(`\u2029`), []byte(string(rune(0x2029))))
		output.Write(encoded)
	case json.Number:
		number, err := codexCanonicalNumber(value)
		if err != nil {
			return err
		}
		output.WriteString(number)
	case []any:
		output.WriteByte('[')
		for index, item := range value {
			if index > 0 {
				output.WriteByte(',')
			}
			if err := writeCodexCanonicalJSON(output, item); err != nil {
				return err
			}
		}
		output.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(value))
		for key := range value {
			keys = append(keys, key)
		}
		sort.Slice(keys, func(left, right int) bool {
			return codexUTF16Less(keys[left], keys[right])
		})
		output.WriteByte('{')
		for index, key := range keys {
			if index > 0 {
				output.WriteByte(',')
			}
			if err := writeCodexCanonicalJSON(output, key); err != nil {
				return err
			}
			output.WriteByte(':')
			if err := writeCodexCanonicalJSON(output, value[key]); err != nil {
				return err
			}
		}
		output.WriteByte('}')
	default:
		return fmt.Errorf("unsupported schema JSON value")
	}
	return nil
}

func codexCanonicalNumber(number json.Number) (string, error) {
	value, err := strconv.ParseFloat(number.String(), 64)
	if err != nil || math.IsInf(value, 0) || math.IsNaN(value) {
		return "", fmt.Errorf("invalid schema JSON number")
	}
	if value == 0 {
		return "0", nil
	}
	sign := ""
	if value < 0 {
		sign = "-"
		value = -value
	}
	scientific := strconv.FormatFloat(value, 'e', -1, 64)
	parts := strings.Split(scientific, "e")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid schema JSON number")
	}
	exponent, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", fmt.Errorf("invalid schema JSON number")
	}
	digits := strings.ReplaceAll(parts[0], ".", "")
	point := exponent + 1
	switch {
	case len(digits) <= point && point <= 21:
		return sign + digits + strings.Repeat("0", point-len(digits)), nil
	case 0 < point && point <= 21:
		return sign + digits[:point] + "." + digits[point:], nil
	case -6 < point && point <= 0:
		return sign + "0." + strings.Repeat("0", -point) + digits, nil
	default:
		mantissa := digits[:1]
		if len(digits) > 1 {
			mantissa += "." + digits[1:]
		}
		exponent = point - 1
		exponentSign := "+"
		if exponent < 0 {
			exponentSign = "-"
			exponent = -exponent
		}
		return sign + mantissa + "e" + exponentSign + strconv.Itoa(exponent), nil
	}
}

func codexUTF16Less(left, right string) bool {
	leftUnits := utf16.Encode([]rune(left))
	rightUnits := utf16.Encode([]rune(right))
	for index := 0; index < len(leftUnits) && index < len(rightUnits); index++ {
		if leftUnits[index] != rightUnits[index] {
			return leftUnits[index] < rightUnits[index]
		}
	}
	return len(leftUnits) < len(rightUnits)
}
