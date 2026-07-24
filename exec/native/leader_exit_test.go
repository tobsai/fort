//go:build darwin || linux

package native

import (
	"os/exec"
	"testing"
	"time"
)

func TestLeaderExitWatcherHandlesAlreadyExitedUnreapedProcess(t *testing.T) {
	cmd := exec.Command("sh", "-c", "exit 0")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() { _ = cmd.Wait() })

	// Let the leader become a zombie before watcher registration. Its PID is
	// still pinned because Wait has deliberately not reaped it.
	time.Sleep(50 * time.Millisecond)
	watcher, err := newLeaderExitWatcher(cmd.Process.Pid)
	if err != nil {
		t.Fatalf("register watcher after exit: %v", err)
	}
	defer watcher.Close()

	done := make(chan error, 1)
	go func() { done <- watcher.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("watch already-exited leader: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("watcher missed an already-exited unreaped leader")
	}
}
