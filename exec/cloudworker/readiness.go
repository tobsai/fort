package cloudworker

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/tobsai/fort/cloud/controlapi"
	"github.com/tobsai/fort/exec/native"
)

// CommandReadiness delegates capability observation to one explicitly
// configured, absolute executable. The executable must emit one bounded JSON
// object. Recheck runs it again and requires byte-identical evidence before it
// probes the exact built-in native provider and verifies the pinned workdir.
type CommandReadiness struct {
	capabilityRevisionID string
	revision             int
	command              []string
	providers            map[string]native.Provider

	mu         sync.Mutex
	lastDigest string
}

func NewCommandReadiness(capabilityRevisionID string, revision int, command []string, providers []native.Provider) (*CommandReadiness, error) {
	if capabilityRevisionID == "" || revision < 1 || len(command) == 0 || !filepath.IsAbs(command[0]) {
		return nil, fmt.Errorf("%w: readiness command", ErrWorkerInvalid)
	}
	info, err := os.Lstat(command[0])
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return nil, fmt.Errorf("%w: readiness executable", ErrWorkerInvalid)
	}
	providerMap := make(map[string]native.Provider, len(providers))
	for _, provider := range providers {
		if provider.Name == "" || providerMap[provider.Name].Name != "" {
			return nil, fmt.Errorf("%w: native provider registry", ErrWorkerInvalid)
		}
		providerMap[provider.Name] = provider
	}
	return &CommandReadiness{capabilityRevisionID: capabilityRevisionID, revision: revision,
		command: append([]string(nil), command...), providers: providerMap}, nil
}

func (readiness *CommandReadiness) Snapshot(ctx context.Context) (ReadinessSnapshot, error) {
	evidence, digest, err := readiness.observe(ctx)
	if err != nil {
		return ReadinessSnapshot{}, err
	}
	readiness.mu.Lock()
	readiness.lastDigest = digest
	readiness.mu.Unlock()
	return ReadinessSnapshot{CapabilityRevisionID: readiness.capabilityRevisionID, Revision: readiness.revision,
		Evidence: evidence, EvidenceDigest: digest}, nil
}

func (readiness *CommandReadiness) Recheck(ctx context.Context, assignment controlapi.WorkerAssignment) error {
	if assignment.CapabilityRevisionID != readiness.capabilityRevisionID {
		return fmt.Errorf("%w: capability revision changed", ErrWorkerInvalid)
	}
	_, digest, err := readiness.observe(ctx)
	if err != nil {
		return err
	}
	readiness.mu.Lock()
	expected := readiness.lastDigest
	readiness.mu.Unlock()
	if expected == "" || digest != expected {
		return fmt.Errorf("%w: readiness evidence changed after claim", ErrWorkerInvalid)
	}
	provider, ok := readiness.providers[assignment.Execution.Provider]
	if !ok || assignment.Execution.AdapterID != "model.chat."+provider.Name {
		return ErrAdapterNotApproved
	}
	if err := native.CheckProvider(ctx, provider); err != nil {
		return fmt.Errorf("recheck native provider: %w", err)
	}
	workdir := assignment.Execution.Workdir
	if !filepath.IsAbs(workdir) || filepath.Clean(workdir) != workdir {
		return fmt.Errorf("%w: pinned workdir", ErrWorkerInvalid)
	}
	info, err := os.Lstat(workdir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: pinned workdir unavailable", ErrWorkerInvalid)
	}
	resolved, err := filepath.EvalSymlinks(workdir)
	if err != nil || resolved != workdir {
		return fmt.Errorf("%w: pinned workdir identity changed", ErrWorkerInvalid)
	}
	return nil
}

func (readiness *CommandReadiness) observe(ctx context.Context) (json.RawMessage, string, error) {
	probeCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	var stdout boundedReadinessBuffer
	command := exec.CommandContext(probeCtx, readiness.command[0], readiness.command[1:]...)
	command.Stdin, command.Stdout, command.Stderr = nil, &stdout, io.Discard
	if err := command.Run(); err != nil {
		return nil, "", fmt.Errorf("readiness command failed: %w", err)
	}
	evidence := bytes.TrimSpace(stdout.Bytes())
	var object map[string]any
	if len(evidence) == 0 || json.Unmarshal(evidence, &object) != nil || object == nil {
		return nil, "", fmt.Errorf("%w: readiness command did not emit one JSON object", ErrWorkerInvalid)
	}
	digest := sha256.Sum256(evidence)
	return append(json.RawMessage(nil), evidence...), hex.EncodeToString(digest[:]), nil
}

type boundedReadinessBuffer struct {
	bytes.Buffer
}

func (buffer *boundedReadinessBuffer) Write(value []byte) (int, error) {
	if buffer.Len()+len(value) > 64<<10 {
		return 0, fmt.Errorf("readiness evidence exceeds 64 KiB")
	}
	return buffer.Buffer.Write(value)
}
