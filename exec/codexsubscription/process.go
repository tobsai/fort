package codexsubscription

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type StartRequest struct {
	Executable  string
	Args        []string
	Workdir     string
	Environment []string
	StdinNull   bool
}

type Process interface {
	Stdout() io.Reader
	Stderr() io.Reader
	Wait() (int, error)
	KillProcessGroup() error
}

type Starter interface {
	Start(context.Context, StartRequest) (Process, error)
}

type osStarter struct{}

func (osStarter) Start(ctx context.Context, request StartRequest) (Process, error) {
	if !filepath.IsAbs(request.Executable) || !filepath.IsAbs(request.Workdir) || !request.StdinNull {
		return nil, fmt.Errorf("codex subscription: invalid process request")
	}
	stdin, err := os.Open(os.DevNull)
	if err != nil {
		return nil, fmt.Errorf("codex subscription: stdin unavailable")
	}
	defer stdin.Close()
	cmd := exec.CommandContext(ctx, request.Executable, request.Args...)
	cmd.Dir = request.Workdir
	cmd.Env = append([]string(nil), request.Environment...)
	cmd.Stdin = stdin
	cmd.WaitDelay = 250 * time.Millisecond
	setProcGroup(cmd)
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return killProcGroup(cmd.Process.Pid)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("codex subscription: stdout unavailable")
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("codex subscription: stderr unavailable")
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("codex subscription: process start failed")
	}
	return &osProcess{cmd: cmd, stdout: stdout, stderr: stderr}, nil
}

type osProcess struct {
	cmd    *exec.Cmd
	stdout io.Reader
	stderr io.Reader
}

func (p *osProcess) Stdout() io.Reader { return p.stdout }
func (p *osProcess) Stderr() io.Reader { return p.stderr }
func (p *osProcess) Wait() (int, error) {
	err := p.cmd.Wait()
	code := -1
	if p.cmd.ProcessState != nil {
		code = p.cmd.ProcessState.ExitCode()
	}
	return code, err
}
func (p *osProcess) KillProcessGroup() error {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return nil
	}
	return killProcGroup(p.cmd.Process.Pid)
}
