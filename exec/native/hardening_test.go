package native

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

// spec-022 adversarial review: Cancel must kill the whole process group. The
// provider shell backgrounds a long sleep that inherits stdout/stderr; that
// grandchild outlives a SIGKILL aimed only at the direct child and keeps the
// pipe open, so the scanner goroutines block on Scan() and teardown stalls.
// A group kill terminates the grandchild too, letting the run tear down
// promptly.
func TestCancelKillsProcessGroup(t *testing.T) {
	// `sleep 30 &` is a backgrounded grandchild holding the inherited pipes;
	// the foreground `sleep 30` keeps the direct child alive until Cancel.
	rt := New(t.TempDir(), shProvider("orphaner", "sleep 30 & sleep 30"))
	run, err := rt.Dispatch(context.Background(), runtime.RunSpec{RunID: "pg", Agent: "orphaner"})
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	// Give the shell time to fork both sleeps before cancelling.
	time.Sleep(200 * time.Millisecond)
	if err := run.Cancel(); err != nil {
		t.Fatalf("cancel: %v", err)
	}

	done := make(chan runtime.Status, 1)
	go func() { done <- run.Wait() }()
	select {
	case st := <-done:
		if st.State != runtime.StateCanceled {
			t.Errorf("state = %v, want canceled", st.State)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("run did not terminate within 5s of Cancel; process group not killed")
	}
}
