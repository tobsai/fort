package node

import (
	"context"
	"testing"
	"time"

	"github.com/tobsai/fort/core/runtime"
	"github.com/tobsai/fort/exec/native"
)

// TestDrainUnblocksAbandonedRun is the regression guard for the mid-stream
// client-disconnect leak the adversarial review confirmed: the native runtime's
// event channel is buffered (64), so a consumer that stops reading wedges the
// scanner goroutines on a full-channel send; Cancel() kills the process but
// cannot unblock a goroutine parked on a send. handleExec's fix is to Cancel
// then drain — which this test exercises directly. Before the fix, run.Wait()
// never returns and the child process is never reaped.
func TestDrainUnblocksAbandonedRun(t *testing.T) {
	rtn := native.New(t.TempDir(), native.Provider{
		Name: "chatty",
		Command: func(runtime.RunSpec) []string {
			// Flood well past the 64-event buffer, then exit. The process ending
			// closes the pipe (EOF), so the scanners only ever block on the
			// channel *send* — the exact leak drain() must clear. (A lingering
			// grandchild holding the pipe open is a separate, pre-existing native
			// concern and is deliberately not exercised here.)
			return []string{"/bin/sh", "-c", `for i in $(seq 1 2000); do echo line$i; done`}
		},
	})
	run, err := rtn.Dispatch(context.Background(), runtime.RunSpec{RunID: "r1", Agent: "chatty"})
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	// Let the scanner goroutines fill the buffer and block on emit, with no
	// consumer — exactly the state handleExec is in after a client disconnect.
	time.Sleep(300 * time.Millisecond)

	// This is handleExec's disconnect path: cancel, then drain in the background.
	_ = run.Cancel()
	go drain(run)

	done := make(chan struct{})
	go func() { run.Wait(); close(done) }()
	select {
	case <-done:
		if st := run.Status(); st.State != runtime.StateCanceled {
			t.Fatalf("status = %+v, want canceled", st)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("run did not terminate after Cancel+drain — goroutine/process leak")
	}
}
