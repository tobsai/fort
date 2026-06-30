package native

import (
	"context"
	"os"
	"testing"

	"github.com/tobsai/fort/core/runtime"
)

// TestLiveVersionProbe drives a real installed CLI through NativeRuntime to
// prove the executor spawns a real binary, streams its stdout as normalized
// events, and reports the exit code — without burning provider tokens (it runs
// `<cli> --version`). Gated behind FORT_LIVE_PROBE=1 so it never runs in CI.
func TestLiveVersionProbe(t *testing.T) {
	if os.Getenv("FORT_LIVE_PROBE") != "1" {
		t.Skip("set FORT_LIVE_PROBE=1 to run the live binary probe")
	}
	cli := os.Getenv("FORT_LIVE_CLI")
	if cli == "" {
		cli = "codex"
	}
	p := Provider{
		Name:    cli,
		Command: func(_ runtime.RunSpec) []string { return []string{cli, "--version"} },
	}
	rt := New(t.TempDir(), p)
	run, err := rt.Dispatch(context.Background(), runtime.RunSpec{RunID: "live", Agent: cli})
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	var sawOutput bool
	for e := range run.Stream() {
		if e.Type == runtime.EventStdout || e.Type == runtime.EventMessage {
			sawOutput = true
			t.Logf("[%s] %s", e.Type, e.Data)
		}
	}
	st := run.Wait()
	if st.State != runtime.StateSucceeded {
		t.Fatalf("status = %+v, want succeeded", st)
	}
	if !sawOutput {
		t.Fatal("expected at least one stdout/message event from the real CLI")
	}
}
