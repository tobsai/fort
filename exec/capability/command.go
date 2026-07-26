package capability

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

var (
	ErrUnsupportedPlatform    = errors.New("capability command: unsupported platform")
	ErrCommandAbsent          = errors.New("capability command: absent")
	ErrCommandOutputLimit     = errors.New("capability command: output limit exceeded")
	ErrCommandIdentityChanged = errors.New("capability command: executable identity changed")
)

const (
	maxExecutableBytes       = 384 << 20
	commandStreamBufferBytes = 64 << 10
)

func executableSizeAllowed(size int64) bool {
	return size >= 0 && size <= int64(maxExecutableBytes)
}

// A provider can exit its direct process while a detached descendant keeps an
// inherited stdout/stderr pipe open. Without WaitDelay, os/exec waits forever
// for its copy goroutines even after the probe context has expired.
const probePipeWaitDelay = 250 * time.Millisecond

type CommandResolverOptions struct {
	Platform    string
	StageDir    string
	Environment []string
}

// CommandResolver resolves a command once, reads it through a no-follow file
// descriptor, and stages immutable content-addressed bytes for both probes and
// dispatch. Paths and fingerprints stay execution-private.
type CommandResolver struct {
	platform string
	stageDir string
	env      []string
	path     string

	authorizedMu sync.RWMutex
	authorized   map[string]string
}

func NewCommandResolver(options CommandResolverOptions) (*CommandResolver, error) {
	if options.StageDir == "" {
		return nil, fmt.Errorf("capability command: stage directory is required")
	}
	path := ""
	for _, entry := range options.Environment {
		if strings.HasPrefix(entry, "PATH=") {
			path = strings.TrimPrefix(entry, "PATH=")
			break
		}
	}
	if path == "" {
		path = os.Getenv("PATH")
	}
	return &CommandResolver{
		platform: options.Platform, stageDir: options.StageDir,
		env: append([]string(nil), options.Environment...), path: path,
		authorized: map[string]string{},
	}, nil
}

type HeldCommand struct {
	path   string
	digest string
	env    []string
}

// Digest is an internal stable executable-content identity. It must never be
// copied into public inventory or errors.
func (h *HeldCommand) Digest() string { return h.digest }

// Executable returns the immutable staged path for an execution adapter. This
// is process-private and must not cross an API boundary.
func (h *HeldCommand) Executable() string { return h.path }

func (h *HeldCommand) Environment() []string {
	return append([]string(nil), h.env...)
}

func (r *CommandResolver) Hold(name string) (*HeldCommand, error) {
	if r.platform != "darwin/arm64" {
		return nil, ErrUnsupportedPlatform
	}
	resolved, err := r.resolve(name)
	if err != nil {
		return nil, err
	}
	canonical, err := filepath.EvalSymlinks(resolved)
	if err != nil {
		return nil, fmt.Errorf("capability command: unavailable")
	}
	fd, err := unix.Open(canonical, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, fmt.Errorf("capability command: unavailable")
	}
	file := os.NewFile(uintptr(fd), "held-executable")
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return nil, fmt.Errorf("capability command: executable is not a regular file")
	}
	if !executableSizeAllowed(info.Size()) {
		return nil, fmt.Errorf("capability command: executable cannot be held")
	}
	digest, size, err := streamDigest(file, int64(maxExecutableBytes))
	afterDigest, statErr := file.Stat()
	if err != nil || statErr != nil || size != info.Size() || !sameExecutableSnapshot(info, afterDigest) {
		return nil, fmt.Errorf("capability command: executable cannot be held")
	}
	staged, err := r.stage(digest, file, size)
	if err != nil {
		return nil, err
	}
	return &HeldCommand{path: staged, digest: digest, env: append([]string(nil), r.env...)}, nil
}

// AuthorizeExecutable pins one command name to bytes whose capability contract
// has completed successfully. A running daemon never silently adopts different
// bytes; an upgrade requires a restart and a fresh capability probe.
func (r *CommandResolver) AuthorizeExecutable(name, digest string) error {
	if r == nil || name == "" || len(digest) != sha256.Size*2 {
		return ErrCommandIdentityChanged
	}
	if _, err := hex.DecodeString(digest); err != nil {
		return ErrCommandIdentityChanged
	}
	r.authorizedMu.Lock()
	defer r.authorizedMu.Unlock()
	if existing := r.authorized[name]; existing != "" && existing != digest {
		return ErrCommandIdentityChanged
	}
	r.authorized[name] = digest
	return nil
}

// ResolveVerifiedExecutable implements native.VerifiedExecutableResolver
// without importing the concrete native package. It re-holds the current PATH
// target, compares it with the authorized digest, and returns only the immutable
// staged path. All failures collapse to one secret-free drift error.
func (r *CommandResolver) ResolveVerifiedExecutable(name string) (string, error) {
	if r == nil {
		return "", ErrCommandIdentityChanged
	}
	r.authorizedMu.RLock()
	expected := r.authorized[name]
	r.authorizedMu.RUnlock()
	if expected == "" {
		return "", ErrCommandIdentityChanged
	}
	held, err := r.Hold(name)
	if err != nil || held.Digest() != expected {
		return "", ErrCommandIdentityChanged
	}
	return held.Executable(), nil
}

func (r *CommandResolver) resolve(name string) (string, error) {
	if strings.ContainsRune(name, filepath.Separator) {
		if !filepath.IsAbs(name) {
			absolute, err := filepath.Abs(name)
			if err != nil {
				return "", fmt.Errorf("capability command: unavailable")
			}
			name = absolute
		}
		return name, nil
	}
	for _, directory := range filepath.SplitList(r.path) {
		if directory == "" {
			continue
		}
		candidate := filepath.Join(directory, name)
		info, err := os.Stat(candidate)
		if err == nil && info.Mode().IsRegular() && info.Mode()&0o111 != 0 {
			return candidate, nil
		}
	}
	return "", ErrCommandAbsent
}

func streamDigest(source io.Reader, maximum int64) (string, int64, error) {
	return streamCopyAndDigest(io.Discard, source, maximum)
}

func streamCopyAndDigest(destination io.Writer, source io.Reader, maximum int64) (string, int64, error) {
	if maximum < 0 {
		return "", 0, fmt.Errorf("capability command: invalid executable size")
	}
	hasher := sha256.New()
	buffer := make([]byte, commandStreamBufferBytes)
	copied, err := io.CopyBuffer(
		io.MultiWriter(destination, hasher),
		io.LimitReader(source, maximum+1),
		buffer,
	)
	if err != nil {
		return "", copied, err
	}
	if copied > maximum {
		return "", copied, fmt.Errorf("capability command: executable cannot be held")
	}
	return hex.EncodeToString(hasher.Sum(nil)), copied, nil
}

func sameExecutableSnapshot(before, after os.FileInfo) bool {
	return before != nil && after != nil &&
		os.SameFile(before, after) &&
		before.Size() == after.Size() &&
		before.Mode() == after.Mode() &&
		before.ModTime().Equal(after.ModTime())
}

func verifyStagedExecutable(path, digest string) (bool, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		if errors.Is(err, unix.ENOENT) {
			return false, nil
		}
		return false, fmt.Errorf("capability command: stage unavailable")
	}
	file := os.NewFile(uintptr(fd), "staged-executable")
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o500 || !executableSizeAllowed(info.Size()) {
		return false, fmt.Errorf("capability command: stage unavailable")
	}
	actual, size, err := streamDigest(file, int64(maxExecutableBytes))
	afterDigest, statErr := file.Stat()
	if err != nil || statErr != nil || size != info.Size() || !sameExecutableSnapshot(info, afterDigest) {
		return false, fmt.Errorf("capability command: stage unavailable")
	}
	if actual != digest {
		return false, fmt.Errorf("capability command: staged identity conflict")
	}
	return true, nil
}

func (r *CommandResolver) stage(digest string, source *os.File, size int64) (string, error) {
	if err := os.MkdirAll(r.stageDir, 0o700); err != nil {
		return "", fmt.Errorf("capability command: stage unavailable")
	}
	if err := os.Chmod(r.stageDir, 0o700); err != nil {
		return "", fmt.Errorf("capability command: stage unavailable")
	}
	destination := filepath.Join(r.stageDir, digest)
	if existing, err := verifyStagedExecutable(destination, digest); err != nil {
		return "", err
	} else if existing {
		return destination, nil
	}
	temp, err := os.CreateTemp(r.stageDir, ".held-*")
	if err != nil {
		return "", fmt.Errorf("capability command: stage unavailable")
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if _, err := source.Seek(0, io.SeekStart); err != nil {
		temp.Close()
		return "", fmt.Errorf("capability command: stage unavailable")
	}
	sourceBeforeCopy, err := source.Stat()
	if err != nil {
		temp.Close()
		return "", fmt.Errorf("capability command: stage unavailable")
	}
	stagedDigest, copied, err := streamCopyAndDigest(temp, source, size)
	sourceAfterCopy, statErr := source.Stat()
	if err != nil || statErr != nil || copied != size || !sameExecutableSnapshot(sourceBeforeCopy, sourceAfterCopy) {
		temp.Close()
		return "", fmt.Errorf("capability command: stage unavailable")
	}
	if stagedDigest != digest {
		temp.Close()
		return "", ErrCommandIdentityChanged
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return "", fmt.Errorf("capability command: stage unavailable")
	}
	if err := temp.Chmod(0o500); err != nil {
		temp.Close()
		return "", fmt.Errorf("capability command: stage unavailable")
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return "", fmt.Errorf("capability command: stage unavailable")
	}
	if err := temp.Close(); err != nil {
		return "", fmt.Errorf("capability command: stage unavailable")
	}
	if err := os.Link(tempName, destination); err != nil && !errors.Is(err, os.ErrExist) {
		return "", fmt.Errorf("capability command: stage unavailable")
	}
	existing, verifyErr := verifyStagedExecutable(destination, digest)
	if verifyErr != nil {
		return "", verifyErr
	}
	if !existing {
		return "", fmt.Errorf("capability command: stage unavailable")
	}
	directory, err := os.Open(r.stageDir)
	if err == nil {
		_ = directory.Sync()
		_ = directory.Close()
	}
	return destination, nil
}

// Run executes the held identity with bounded combined output. On an output
// limit error no probe bytes are returned.
func (h *HeldCommand) Run(ctx context.Context, args []string, maximum int, timeout time.Duration) ([]byte, error) {
	if maximum < 1 || timeout <= 0 || timeout > probeTimeout {
		return nil, fmt.Errorf("capability command: invalid execution bounds")
	}
	runContext, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	command := exec.CommandContext(runContext, h.path, args...)
	command.Env = append([]string(nil), h.env...)
	command.WaitDelay = probePipeWaitDelay
	configureProbeProcess(command)
	output := &boundedOutput{maximum: maximum}
	command.Stdout, command.Stderr = output, output
	err := command.Run()
	// Probe commands are one-shot and may not leave daemons behind. Run has
	// already reaped the direct child here; kill any descendants that inherited
	// its process group, including helpers still holding output pipes.
	killProbeProcessGroup(command)
	if output.exceeded {
		return nil, ErrCommandOutputLimit
	}
	if runContext.Err() == context.DeadlineExceeded {
		return nil, context.DeadlineExceeded
	}
	if errors.Is(err, exec.ErrWaitDelay) {
		return nil, context.DeadlineExceeded
	}
	return output.bytes(), err
}

// CommandResult is the private result of one bounded, held executable probe.
// Output must be parsed and discarded by the prober; it never enters public
// inventory or error text.
type CommandResult struct {
	Output           []byte
	ExecutableDigest string
	Err              error
}

// CommandExecutor is the narrow command-probe seam used by LocalProber.
type CommandExecutor interface {
	Run(context.Context, string, ...string) CommandResult
}

// ResolverExecutor holds the exact executable bytes before every probe and
// runs that staged identity with the capability probe bounds.
type ResolverExecutor struct {
	Resolver *CommandResolver
}

func (e ResolverExecutor) AuthorizeExecutable(name, digest string) error {
	if e.Resolver == nil {
		return ErrCommandIdentityChanged
	}
	return e.Resolver.AuthorizeExecutable(name, digest)
}

func (e ResolverExecutor) Run(ctx context.Context, name string, args ...string) CommandResult {
	if e.Resolver == nil {
		return CommandResult{Err: ErrCommandAbsent}
	}
	held, err := e.Resolver.Hold(name)
	if err != nil {
		return CommandResult{Err: err}
	}
	output, err := held.Run(ctx, args, 64<<10, probeTimeout)
	return CommandResult{
		Output: output, ExecutableDigest: held.Digest(), Err: err,
	}
}

type boundedOutput struct {
	mu       sync.Mutex
	maximum  int
	buffer   []byte
	exceeded bool
}

func (b *boundedOutput) Write(value []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	remaining := b.maximum - len(b.buffer)
	if remaining > 0 {
		take := len(value)
		if take > remaining {
			take = remaining
		}
		b.buffer = append(b.buffer, value[:take]...)
	}
	if len(value) > remaining {
		b.exceeded = true
	}
	return len(value), nil
}

func (b *boundedOutput) bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]byte(nil), b.buffer...)
}
