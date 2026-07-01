//go:build unix

package native

import (
	"os/exec"
	"syscall"
)

// setProcGroup makes the spawned CLI the leader of a new process group so Fort
// can signal the whole tree — the CLI plus any grandchildren it forks — on
// cancellation. On Unix the new group's ID equals the child's PID (kill(2)).
func setProcGroup(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}

// killProcGroup SIGKILLs every process in the group led by pgid. A negative pid
// targets the whole process group (kill(2)). A missing group (already reaped)
// is not an error worth surfacing.
func killProcGroup(pgid int) {
	if pgid <= 0 {
		return
	}
	_ = syscall.Kill(-pgid, syscall.SIGKILL)
}
