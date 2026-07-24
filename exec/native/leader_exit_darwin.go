//go:build darwin

package native

import (
	"errors"
	"syscall"
	"unsafe"
)

type kqueueLeaderExitWatcher struct {
	fd int
}

func newLeaderExitWatcher(pid int) (leaderExitWatcher, error) {
	fd, err := syscall.Kqueue()
	if err != nil {
		return nil, err
	}
	change := syscall.Kevent_t{
		Ident:  uint64(pid),
		Filter: syscall.EVFILT_PROC,
		Flags:  syscall.EV_ADD | syscall.EV_ENABLE | syscall.EV_ONESHOT,
		Fflags: syscall.NOTE_EXIT,
	}
	if _, err := syscall.Kevent(fd, []syscall.Kevent_t{change}, nil, nil); err == nil {
		return &kqueueLeaderExitWatcher{fd: fd}, nil
	}
	_ = syscall.Close(fd)

	// A very short-lived child may exit between Start and EVFILT_PROC
	// registration. waitid with WNOWAIT handles that state without reaping, so
	// the PID/PGID remains pinned until cmd.Wait.
	return &darwinWaitIDLeaderExitWatcher{pid: pid}, nil
}

func (w *kqueueLeaderExitWatcher) Wait() error {
	events := make([]syscall.Kevent_t, 1)
	for {
		n, err := syscall.Kevent(w.fd, nil, events, nil)
		if errors.Is(err, syscall.EINTR) {
			continue
		}
		if err != nil {
			return err
		}
		if n == 0 {
			continue
		}
		if events[0].Flags&syscall.EV_ERROR != 0 && events[0].Data != 0 {
			return syscall.Errno(events[0].Data)
		}
		if events[0].Fflags&syscall.NOTE_EXIT != 0 {
			return nil
		}
	}
}

func (w *kqueueLeaderExitWatcher) Close() error {
	if w.fd < 0 {
		return nil
	}
	err := syscall.Close(w.fd)
	w.fd = -1
	return err
}

type darwinWaitIDLeaderExitWatcher struct {
	pid int
}

func (w *darwinWaitIDLeaderExitWatcher) Wait() error {
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

func (*darwinWaitIDLeaderExitWatcher) Close() error { return nil }
