//go:build !darwin && !linux

package native

import "fmt"

func newLeaderExitWatcher(pid int) (leaderExitWatcher, error) {
	return nil, fmt.Errorf("safe non-reaping process-exit watcher unavailable for pid %d", pid)
}
