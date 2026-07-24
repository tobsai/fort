//go:build linux

package native

import (
	"syscall"
	"unsafe"
)

type linuxWaitIDLeaderExitWatcher struct {
	pid int
}

func newLeaderExitWatcher(pid int) (leaderExitWatcher, error) {
	return &linuxWaitIDLeaderExitWatcher{pid: pid}, nil
}

func (w *linuxWaitIDLeaderExitWatcher) Wait() error {
	var info [32]uintptr
	for {
		_, _, errno := syscall.Syscall6(
			syscall.SYS_WAITID,
			1, // P_PID
			uintptr(w.pid),
			uintptr(unsafe.Pointer(&info[0])),
			uintptr(syscall.WEXITED|syscall.WNOWAIT),
			0,
			0,
		)
		if errno == syscall.EINTR {
			continue
		}
		if errno != 0 {
			return errno
		}
		return nil
	}
}

func (*linuxWaitIDLeaderExitWatcher) Close() error { return nil }
