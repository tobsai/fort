package native

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tobsai/fort/core/runtime"
)

// AO-041: a run executes inside its scoped workdir.
func TestRunExecutesInScopedWorkdir(t *testing.T) {
	root := t.TempDir()
	work := filepath.Join(root, "run-abc")
	rt := New(root, shProvider("pwd", "pwd"))
	run, err := rt.Dispatch(context.Background(), runtime.RunSpec{RunID: "x", Agent: "pwd", Workdir: work})
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	var out string
	for e := range run.Stream() {
		if e.Type == runtime.EventStdout {
			out = e.Data
		}
	}
	run.Wait()
	// macOS resolves /var -> /private/var; compare suffix.
	if !strings.HasSuffix(out, "run-abc") {
		t.Errorf("pwd = %q, want it to end in the scoped workdir run-abc", out)
	}
}

// AO-041: with an env allowlist, secrets in the host env are NOT leaked to the
// spawned CLI (least privilege).
func TestEnvAllowlistWithholdsSecrets(t *testing.T) {
	t.Setenv("FORT_SECRET_TOKEN", "leak-me")
	rt := New(t.TempDir(), shProvider("envdump", "echo SECRET=$FORT_SECRET_TOKEN"))
	rt.EnvAllow = []string{"PATH"} // FORT_SECRET_TOKEN deliberately excluded

	run, _ := rt.Dispatch(context.Background(), runtime.RunSpec{RunID: "y", Agent: "envdump"})
	var out []string
	for e := range run.Stream() {
		if e.Type == runtime.EventStdout {
			out = append(out, e.Data)
		}
	}
	run.Wait()
	joined := strings.Join(out, "\n")
	if strings.Contains(joined, "leak-me") {
		t.Errorf("secret leaked into runtime env: %q", joined)
	}
	if !strings.Contains(joined, "SECRET=") {
		t.Errorf("expected the echo line, got %q", joined)
	}
}
