package capability

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	corecap "github.com/tobsai/fort/core/capability"
)

type fakeCodexSchemaGenerator struct {
	normal           map[string]string
	experimental     map[string]string
	err              error
	calls            int
	normalDir        string
	experimentalDir  string
	privateEmptyDirs bool
	executableDigest string
}

func (g *fakeCodexSchemaGenerator) Generate(ctx context.Context, normalDir, experimentalDir string) (string, error) {
	g.calls++
	g.normalDir, g.experimentalDir = normalDir, experimentalDir
	normalEntries, normalErr := os.ReadDir(normalDir)
	experimentalEntries, experimentalErr := os.ReadDir(experimentalDir)
	normalInfo, normalStatErr := os.Stat(normalDir)
	experimentalInfo, experimentalStatErr := os.Stat(experimentalDir)
	g.privateEmptyDirs = normalErr == nil && experimentalErr == nil &&
		normalStatErr == nil && experimentalStatErr == nil &&
		len(normalEntries) == 0 && len(experimentalEntries) == 0 &&
		normalInfo.Mode().Perm() == 0o700 && experimentalInfo.Mode().Perm() == 0o700
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if g.err != nil {
		return "", g.err
	}
	if err := writeSchemaFixture(normalDir, g.normal); err != nil {
		return "", err
	}
	if err := writeSchemaFixture(experimentalDir, g.experimental); err != nil {
		return "", err
	}
	if g.executableDigest == "" {
		g.executableDigest = "fixture-executable"
	}
	return g.executableDigest, nil
}

func TestCodexSchemaContractVerifierReturnsOnlyExactCanonicalBundles(t *testing.T) {
	generator := &fakeCodexSchemaGenerator{
		normal: map[string]string{
			"b.json":        `{ "z": 1.0, "a": "<>&", "fraction": 0.000001, "scientific": 1e+21 }`,
			"nested/😀.json": `{"\uE000":"bmp","😀":"astral","line":"\u2028"}`,
		},
		experimental: map[string]string{
			"array.json": `[true,null,-0.0,333333333.33333329,1e-7]`,
		},
	}
	verifier := newCodexSchemaContractVerifier(generator, codexSchemaExpectation{
		normal: codexBundleExpectation{
			digest: "abb20df1a8adda13da715101688a499762f31c975d4771d2df7170748e11fb3a", files: 2,
		},
		experimental: codexBundleExpectation{
			digest: "f532339f7bcebed19820c2c967b57087f8b85fd1a89f705055a474935b0073f3", files: 1,
		},
	})

	contract, err := verifier.Verify(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if contract.NormalSchemaDigest != "abb20df1a8adda13da715101688a499762f31c975d4771d2df7170748e11fb3a" ||
		contract.NormalSchemaFiles != 2 ||
		contract.ExperimentalSchemaDigest != "f532339f7bcebed19820c2c967b57087f8b85fd1a89f705055a474935b0073f3" ||
		contract.ExperimentalSchemaFiles != 1 {
		t.Fatalf("contract = %#v", contract)
	}
	if contract.ExecutableDigest != "fixture-executable" {
		t.Fatalf("executable digest = %q", contract.ExecutableDigest)
	}
	if contract.GmailIsolationReady || contract.SupabaseIsolationReady {
		t.Fatalf("unimplemented isolation advertised: %#v", contract)
	}
	if generator.calls != 1 || generator.normalDir == generator.experimentalDir || !generator.privateEmptyDirs {
		t.Fatalf("generator = %#v", generator)
	}
	for _, directory := range []string{generator.normalDir, generator.experimentalDir} {
		if _, err := os.Stat(directory); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("private directory still exists: %q, err=%v", directory, err)
		}
	}
}

func TestCodexSchemaContractVerifierFailsClosedWithoutLeakingDetails(t *testing.T) {
	tests := []struct {
		name        string
		generator   *fakeCodexSchemaGenerator
		expectation codexSchemaExpectation
		wantReason  corecap.Reason
	}{
		{
			name: "bundle mismatch",
			generator: &fakeCodexSchemaGenerator{
				normal:       map[string]string{"PRIVATE-NAME.json": `{"secret":"PRIVATE-CONTENT"}`},
				experimental: map[string]string{"schema.json": `{}`},
			},
			expectation: codexSchemaExpectation{
				normal:       codexBundleExpectation{digest: strings.Repeat("0", 64), files: 1},
				experimental: codexBundleExpectation{digest: strings.Repeat("0", 64), files: 1},
			},
			wantReason: corecap.ReasonIncompatibleVersion,
		},
		{
			name:        "generator timeout",
			generator:   &fakeCodexSchemaGenerator{err: context.DeadlineExceeded},
			expectation: codexSchemaExpectation{},
			wantReason:  corecap.ReasonProbeTimedOut,
		},
		{
			name:        "generator output limit",
			generator:   &fakeCodexSchemaGenerator{err: ErrCommandOutputLimit},
			expectation: codexSchemaExpectation{},
			wantReason:  corecap.ReasonOutputLimitExceeded,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := newCodexSchemaContractVerifier(test.generator, test.expectation).Verify(context.Background())
			var probeError *ProbeError
			if !errors.As(err, &probeError) || probeError.Reason != test.wantReason {
				t.Fatalf("error = %#v", err)
			}
			if strings.Contains(fmt.Sprint(err), "PRIVATE") || strings.Contains(fmt.Sprint(err), test.generator.normalDir) {
				t.Fatalf("private detail leaked: %v", err)
			}
		})
	}
}

func TestCodexSchemaContractVerifierBoundsFilesAndBytes(t *testing.T) {
	tooMany := make(map[string]string, codexSchemaFileMax+1)
	for index := 0; index <= codexSchemaFileMax; index++ {
		tooMany[fmt.Sprintf("schema-%03d.json", index)] = `{}`
	}
	tests := []struct {
		name   string
		normal map[string]string
	}{
		{name: "file count", normal: tooMany},
		{name: "single file bytes", normal: map[string]string{
			"schema.json": `"` + strings.Repeat("x", codexSchemaSingleFileMax) + `"`,
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			generator := &fakeCodexSchemaGenerator{
				normal: test.normal, experimental: map[string]string{"schema.json": `{}`},
			}
			_, err := newCodexSchemaContractVerifier(generator, codexSchemaExpectation{}).Verify(context.Background())
			var probeError *ProbeError
			if !errors.As(err, &probeError) || probeError.Reason != corecap.ReasonIncompatibleVersion {
				t.Fatalf("error = %#v", err)
			}
		})
	}
}

type blockingCodexSchemaGenerator struct{}

func (blockingCodexSchemaGenerator) Generate(ctx context.Context, _, _ string) (string, error) {
	<-ctx.Done()
	return "", ctx.Err()
}

func TestCodexSchemaContractVerifierHonorsCallerDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_, err := newCodexSchemaContractVerifier(blockingCodexSchemaGenerator{}, codexSchemaExpectation{}).Verify(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %#v", err)
	}
}

func TestResolverCodexSchemaGeneratorUsesExactHeldCommands(t *testing.T) {
	root := t.TempDir()
	binDir := filepath.Join(root, "bin")
	stageDir := filepath.Join(root, "stage")
	logPath := filepath.Join(root, "argv.log")
	if err := os.Mkdir(binDir, 0o700); err != nil {
		t.Fatal(err)
	}
	script := `#!/bin/sh
printf '%s\n' "$@" >> "$FORT_SCHEMA_TEST_LOG"
for arg in "$@"; do output="$arg"; done
mkdir -p "$output"
printf '{"ok":true}' > "$output/schema.json"
`
	if err := os.WriteFile(filepath.Join(binDir, "codex"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	resolver, err := NewCommandResolver(CommandResolverOptions{
		Platform: "darwin/arm64", StageDir: stageDir,
		Environment: []string{"PATH=" + binDir + ":/bin:/usr/bin", "FORT_SCHEMA_TEST_LOG=" + logPath},
	})
	if err != nil {
		t.Fatal(err)
	}
	normalDir, experimentalDir := filepath.Join(root, "normal"), filepath.Join(root, "experimental")
	if err := os.Mkdir(normalDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(experimentalDir, 0o700); err != nil {
		t.Fatal(err)
	}
	digest, err := (ResolverCodexSchemaGenerator{Resolver: resolver}).Generate(context.Background(), normalDir, experimentalDir)
	if err != nil {
		t.Fatal(err)
	}
	if digest == "" {
		t.Fatal("held executable digest is empty")
	}
	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"app-server", "generate-json-schema", "--out", normalDir,
		"app-server", "generate-json-schema", "--experimental", "--out", experimentalDir,
	}
	if got := strings.Fields(string(log)); !reflect.DeepEqual(got, want) {
		t.Fatalf("argv = %#v, want %#v", got, want)
	}
}

func writeSchemaFixture(root string, files map[string]string) error {
	for relative, content := range files {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			return err
		}
	}
	return nil
}
