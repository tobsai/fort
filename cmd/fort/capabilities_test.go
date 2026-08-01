package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tobsai/fort/core/config"
	"github.com/tobsai/fort/core/machines"
	execcap "github.com/tobsai/fort/exec/capability"
	"github.com/tobsai/fort/exec/fake"
)

type blockingCapabilityVerifier struct {
	started chan struct{}
	once    sync.Once
}

func (v *blockingCapabilityVerifier) Verify(ctx context.Context) (execcap.CodexAppServerContract, error) {
	v.once.Do(func() { close(v.started) })
	<-ctx.Done()
	return execcap.CodexAppServerContract{}, ctx.Err()
}

func TestBuildCapabilitySubsystemWiresRegistryCoordinatorAndGate(t *testing.T) {
	live := &machines.Live{}
	registry, err := machines.Parse([]byte(`
version: 1
machines:
  - {name: laptop, url: "http://127.0.0.1:4087", agents: [codex]}
`), "laptop")
	if err != nil {
		t.Fatal(err)
	}
	live.Store(registry)
	cfg := config.Config{
		DBPath: filepath.Join(t.TempDir(), "fort.db"), WorkRoot: t.TempDir(),
		NodeName: "laptop", NodeToken: "mesh-token",
	}
	subsystem, err := buildCapabilitySubsystem(
		cfg, live, fake.New(), []byte("01234567890123456789012345678901"),
		[]string{"PATH=/usr/bin:/bin"}, "darwin/arm64", func() string { return "mesh-token" },
	)
	if err != nil {
		t.Fatal(err)
	}
	if subsystem.local == nil || subsystem.coordinator == nil || subsystem.runtime == nil || subsystem.executables == nil {
		t.Fatalf("subsystem=%+v", subsystem)
	}
	if got := subsystem.runtime.Name(); !strings.HasPrefix(got, "profile-gate(") {
		t.Fatalf("runtime name=%q", got)
	}
	if _, generation := subsystem.coordinator.Current(); generation != 0 {
		t.Fatalf("initial generation=%d, want 0 before first refresh", generation)
	}
}

func TestBuildCapabilitySubsystemDoesNotRunCodexVerifier(t *testing.T) {
	root := t.TempDir()
	binDir := filepath.Join(root, "bin")
	if err := os.Mkdir(binDir, 0o700); err != nil {
		t.Fatal(err)
	}
	called := filepath.Join(root, "called")
	script := "#!/bin/sh\ntouch \"$FORT_CAPABILITY_CALLED\"\nexit 1\n"
	if err := os.WriteFile(filepath.Join(binDir, "codex"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{
		DBPath: filepath.Join(root, "fort.db"), WorkRoot: root, NodeName: "laptop",
	}
	if _, err := buildCapabilitySubsystem(
		cfg, &machines.Live{}, fake.New(), []byte("01234567890123456789012345678901"),
		[]string{"PATH=" + binDir + ":/usr/bin:/bin", "FORT_CAPABILITY_CALLED=" + called},
		"darwin/arm64", func() string { return "" },
	); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(called); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("schema verifier ran while wiring: %v", err)
	}
}

func TestCapabilityStartupRefreshIsAsynchronous(t *testing.T) {
	verifier := &blockingCapabilityVerifier{started: make(chan struct{})}
	root := t.TempDir()
	binDir := filepath.Join(root, "bin")
	if err := os.Mkdir(binDir, 0o700); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\nif [ \"$1\" = \"--version\" ]; then\n  printf '%s\\n' 'codex-cli 0.146.0-alpha.3.1'\n  exit 0\nfi\nexit 1\n"
	if err := os.WriteFile(filepath.Join(binDir, "codex"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{
		DBPath: filepath.Join(root, "fort.db"), WorkRoot: root, NodeName: "laptop",
	}
	subsystem, err := buildCapabilitySubsystemWithVerifier(
		cfg, &machines.Live{}, fake.New(), []byte("01234567890123456789012345678901"),
		[]string{"PATH=" + binDir + ":/usr/bin:/bin"}, "darwin/arm64", func() string { return "" }, verifier,
	)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	returned := make(chan (<-chan struct{}), 1)
	go func() {
		returned <- subsystem.start(ctx, time.Hour)
	}()
	var stopped <-chan struct{}
	select {
	case stopped = <-returned:
	case <-time.After(time.Second):
		t.Fatal("startup blocked on capability inventory")
	}
	select {
	case <-verifier.started:
	case <-time.After(5 * time.Second):
		t.Fatal("background inventory did not begin")
	}
	cancel()
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("background inventory did not stop after cancellation")
	}
}

func TestCapabilityPollWaitsFullIntervalAfterRefreshSettles(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	starts := make(chan time.Time, 2)
	calls := 0
	done := make(chan struct{})
	go func() {
		defer close(done)
		pollCapabilityRefresh(ctx, 20*time.Millisecond, func(context.Context) (uint64, error) {
			calls++
			starts <- time.Now()
			if calls == 1 {
				time.Sleep(40 * time.Millisecond)
			} else {
				cancel()
			}
			return uint64(calls), nil
		})
	}()
	first := <-starts
	second := <-starts
	if delta := second.Sub(first); delta < 55*time.Millisecond {
		t.Fatalf("next refresh started only %s after prior start; interval was not reset after settlement", delta)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("poll did not stop after cancellation")
	}
}

func TestCapabilityPlanningEnabledHonorsRollbackAndFakeRuntime(t *testing.T) {
	for _, tc := range []struct {
		name string
		env  map[string]string
		want bool
	}{
		{name: "default", want: true},
		{name: "rollback", env: map[string]string{"FORT_CAPABILITY_PLANNING": "0"}},
		{name: "fake", env: map[string]string{"FORT_FAKE": "1"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			getenv := func(key string) string { return tc.env[key] }
			if got := capabilityPlanningEnabled(getenv); got != tc.want {
				t.Fatalf("enabled=%v, want %v", got, tc.want)
			}
		})
	}
}
