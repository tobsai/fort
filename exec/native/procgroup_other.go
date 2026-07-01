//go:build !unix

package native

import "os/exec"

// On non-Unix platforms Fort has no process-group primitive; cancellation falls
// back to context-driven termination of the direct child only.
func setProcGroup(cmd *exec.Cmd) {}

func killProcGroup(pgid int) {}
