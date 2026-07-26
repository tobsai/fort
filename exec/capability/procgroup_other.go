//go:build !unix

package capability

import "os/exec"

func configureProbeProcess(command *exec.Cmd) {}

func killProbeProcessGroup(command *exec.Cmd) {
	if command.Process != nil {
		_ = command.Process.Kill()
	}
}
