package capability

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

type boundedStreamingReader struct {
	remaining int64
	maximum   int
	largest   int
}

func (r *boundedStreamingReader) Read(buffer []byte) (int, error) {
	if len(buffer) > r.maximum {
		return 0, errors.New("read buffer exceeded bounded streaming contract")
	}
	if len(buffer) > r.largest {
		r.largest = len(buffer)
	}
	if r.remaining == 0 {
		return 0, io.EOF
	}
	count := len(buffer)
	if int64(count) > r.remaining {
		count = int(r.remaining)
	}
	for index := range buffer[:count] {
		buffer[index] = byte(index)
	}
	r.remaining -= int64(count)
	return count, nil
}

func TestExecutableDigestStreamsThroughBoundedBuffers(t *testing.T) {
	const size = int64(8<<20 + 17)
	source := &boundedStreamingReader{
		remaining: size,
		maximum:   commandStreamBufferBytes,
	}
	digest, copied, err := streamDigest(source, size)
	if err != nil {
		t.Fatal(err)
	}
	if copied != size {
		t.Fatalf("copied = %d, want %d", copied, size)
	}
	if len(digest) != 64 {
		t.Fatalf("digest length = %d, want 64", len(digest))
	}
	if source.largest > commandStreamBufferBytes {
		t.Fatalf("largest read = %d, want <= %d", source.largest, commandStreamBufferBytes)
	}
}

func TestExecutableDigestMatchesSHA256AndRejectsLimitPlusOne(t *testing.T) {
	const content = "fort capability identity"
	want := sha256.Sum256([]byte(content))
	digest, copied, err := streamDigest(strings.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatal(err)
	}
	if digest != hex.EncodeToString(want[:]) || copied != int64(len(content)) {
		t.Fatalf("digest = %q copied = %d", digest, copied)
	}
	overLimit := &boundedStreamingReader{remaining: 1025, maximum: commandStreamBufferBytes}
	if _, copied, err := streamDigest(overLimit, 1024); err == nil || copied != 1025 {
		t.Fatalf("limit+1 copied = %d err = %v", copied, err)
	}
}

func TestHeldCommandExecutesStagedBytesAfterSourceReplacement(t *testing.T) {
	bin := t.TempDir()
	source := filepath.Join(bin, "probe")
	if err := os.WriteFile(source, []byte("#!/bin/sh\necho original\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	resolver, err := NewCommandResolver(CommandResolverOptions{
		Platform: "darwin/arm64", StageDir: filepath.Join(t.TempDir(), "held"),
		Environment: []string{"PATH=" + bin},
	})
	if err != nil {
		t.Fatal(err)
	}
	held, err := resolver.Hold("probe")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("#!/bin/sh\necho replaced\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	output, err := held.Run(context.Background(), nil, 1024, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(output)) != "original" {
		t.Fatalf("output = %q", output)
	}
}

func TestCommandResolverRejectsNonExecutableAbsoluteFile(t *testing.T) {
	source := filepath.Join(t.TempDir(), "not-executable")
	if err := os.WriteFile(source, []byte("#!/bin/sh\nexit 0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	resolver, err := NewCommandResolver(CommandResolverOptions{
		Platform: "darwin/arm64", StageDir: filepath.Join(t.TempDir(), "held"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := resolver.Hold(source); err == nil {
		t.Fatal("non-executable absolute file was held")
	}
}

func TestCommandResolverRejectsWritableStagedIdentity(t *testing.T) {
	source := filepath.Join(t.TempDir(), "probe")
	if err := os.WriteFile(source, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	resolver, err := NewCommandResolver(CommandResolverOptions{
		Platform: "darwin/arm64", StageDir: filepath.Join(t.TempDir(), "held"),
	})
	if err != nil {
		t.Fatal(err)
	}
	held, err := resolver.Hold(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(held.Executable(), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := resolver.Hold(source); err == nil {
		t.Fatal("writable staged executable was trusted")
	}
}

func TestCommandResolverRejectsSymlinkedStagedIdentity(t *testing.T) {
	content := []byte("#!/bin/sh\nexit 0\n")
	source := filepath.Join(t.TempDir(), "probe")
	if err := os.WriteFile(source, content, 0o700); err != nil {
		t.Fatal(err)
	}
	stageDir := t.TempDir()
	digest := sha256.Sum256(content)
	target := filepath.Join(t.TempDir(), "matching-target")
	if err := os.WriteFile(target, content, 0o500); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(stageDir, hex.EncodeToString(digest[:]))); err != nil {
		t.Fatal(err)
	}
	resolver, err := NewCommandResolver(CommandResolverOptions{
		Platform: "darwin/arm64", StageDir: stageDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := resolver.Hold(source); err == nil {
		t.Fatal("symlinked staged executable was trusted")
	}
}

func TestExecutableSnapshotRejectsSameSizeMutation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "probe")
	if err := os.WriteFile(path, []byte("first"), 0o700); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	before, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("other"), 0o700); err != nil {
		t.Fatal(err)
	}
	changed := before.ModTime().Add(time.Second)
	if err := os.Chtimes(path, changed, changed); err != nil {
		t.Fatal(err)
	}
	after, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if sameExecutableSnapshot(before, after) {
		t.Fatal("same-size source mutation was treated as stable")
	}
}

func TestHoldRejectsExecutableAboveHardLimitWithoutStageArtifact(t *testing.T) {
	path := filepath.Join(t.TempDir(), "oversized")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o700)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(int64(maxExecutableBytes) + 1); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	stageDir := t.TempDir()
	resolver, err := NewCommandResolver(CommandResolverOptions{
		Platform: "darwin/arm64", StageDir: stageDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := resolver.Hold(path); err == nil {
		t.Fatal("oversized executable was held")
	}
	entries, err := os.ReadDir(stageDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("stage artifacts = %d, want 0", len(entries))
	}
}

func TestExecutableSizeBoundAdmitsObservedCLIsAndRemainsBounded(t *testing.T) {
	const miniCodexBytes = int64(271_134_288)
	if !executableSizeAllowed(miniCodexBytes) {
		t.Fatalf("observed Codex executable size %d is outside the staging bound", miniCodexBytes)
	}
	if executableSizeAllowed(int64(maxExecutableBytes) + 1) {
		t.Fatalf("size above hard limit %d was accepted", maxExecutableBytes)
	}
}

func TestHeldCommandCapsOutputWithoutReturningProbeText(t *testing.T) {
	bin := t.TempDir()
	source := filepath.Join(bin, "loud")
	if err := os.WriteFile(source, []byte("#!/bin/sh\nprintf '%02048d' 1\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	resolver, err := NewCommandResolver(CommandResolverOptions{
		Platform: "darwin/arm64", StageDir: filepath.Join(t.TempDir(), "held"),
		Environment: []string{"PATH=" + bin},
	})
	if err != nil {
		t.Fatal(err)
	}
	held, err := resolver.Hold("loud")
	if err != nil {
		t.Fatal(err)
	}
	output, err := held.Run(context.Background(), nil, 64, time.Second)
	if !errors.Is(err, ErrCommandOutputLimit) || output != nil {
		t.Fatalf("output bytes=%d err=%v", len(output), err)
	}
}

func TestHeldCommandTimeoutDoesNotWaitForDescendantPipe(t *testing.T) {
	// A real OpenClaw readiness command can leave a descendant holding the
	// inherited stdout/stderr pipe after the command context kills its parent.
	// The probe deadline must bound the whole process tree, not wait for that
	// descendant to close the pipe on its own.
	testBinary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	pidFile := filepath.Join(t.TempDir(), "descendant.pid")
	resolver, err := NewCommandResolver(CommandResolverOptions{
		Platform: "darwin/arm64", StageDir: filepath.Join(t.TempDir(), "held"),
		Environment: []string{
			"FORT_CAPABILITY_DETACHED_HELPER=1",
			"FORT_CAPABILITY_HELPER_PID_FILE=" + pidFile,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	held, err := resolver.Hold(testBinary)
	if err != nil {
		t.Fatal(err)
	}

	started := time.Now()
	_, err = held.Run(context.Background(), []string{"-test.run=^TestCapabilityDetachedHelper$"}, 1024, 5*time.Second)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want context deadline exceeded", err)
	}
	if elapsed := time.Since(started); elapsed >= 4*time.Second {
		t.Fatalf("probe returned after %s; descendant kept output pipe open", elapsed)
	}
	pidBytes, readErr := os.ReadFile(pidFile)
	if readErr != nil {
		t.Fatalf("read descendant pid: %v", readErr)
	}
	pid, parseErr := strconv.Atoi(strings.TrimSpace(string(pidBytes)))
	if parseErr != nil {
		t.Fatalf("parse descendant pid: %v", parseErr)
	}
	t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })
	deadline := time.Now().Add(2 * time.Second)
	for {
		if err := syscall.Kill(pid, 0); errors.Is(err, syscall.ESRCH) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("descendant pid %d survived the bounded probe", pid)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestCapabilityDetachedHelper(t *testing.T) {
	if os.Getenv("FORT_CAPABILITY_DETACHED_HELPER") != "1" {
		t.Skip("helper process")
	}
	child := exec.Command("/bin/sleep", "30")
	child.Stdout = os.Stdout
	child.Stderr = os.Stderr
	if err := child.Start(); err != nil {
		os.Exit(2)
	}
	if err := os.WriteFile(os.Getenv("FORT_CAPABILITY_HELPER_PID_FILE"), []byte(strconv.Itoa(child.Process.Pid)), 0o600); err != nil {
		_ = child.Process.Kill()
		os.Exit(3)
	}
	os.Exit(0)
}

func TestCommandResolverRejectsUnsupportedPlatformBeforeLookup(t *testing.T) {
	resolver, err := NewCommandResolver(CommandResolverOptions{
		Platform: "linux/amd64", StageDir: filepath.Join(t.TempDir(), "held"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := resolver.Hold("anything"); !errors.Is(err, ErrUnsupportedPlatform) {
		t.Fatalf("error = %v", err)
	}
}
