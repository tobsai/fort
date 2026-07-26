//go:build unix

package capability

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// configureProbeProcess gives each bounded probe its own process group and
// makes context cancellation terminate the entire tree, not only its leader.
func configureProbeProcess(command *exec.Cmd) {
	if command.SysProcAttr == nil {
		command.SysProcAttr = &syscall.SysProcAttr{}
	}
	command.SysProcAttr.Setpgid = true
	command.Cancel = func() error {
		if command.Process == nil {
			return os.ErrProcessDone
		}
		err := syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
}

func killProbeProcessGroup(command *exec.Cmd) {
	if command.Process == nil || command.Process.Pid <= 0 {
		return
	}
	_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
}
