//go:build !unix

package codexsubscription

import (
	"errors"
	"os"
	"os/exec"
)

func setProcGroup(cmd *exec.Cmd) {}

func killProcGroup(pid int) error {
	if pid <= 0 {
		return nil
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	err = process.Kill()
	if errors.Is(err, os.ErrProcessDone) {
		return nil
	}
	return err
}
