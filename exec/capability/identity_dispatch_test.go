package capability

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	corecap "github.com/tobsai/fort/core/capability"
	"github.com/tobsai/fort/core/runtime"
	"github.com/tobsai/fort/exec/native"
)

func TestAuthorizedNativeDispatchRejectsPATHTargetSwapBeforeStart(t *testing.T) {
	bin := t.TempDir()
	command := filepath.Join(bin, "claude")
	marker := filepath.Join(t.TempDir(), "started")
	writeIdentityTestCommand(t, command, "")

	resolver, err := NewCommandResolver(CommandResolverOptions{
		Platform:    "darwin/arm64",
		StageDir:    filepath.Join(t.TempDir(), "held"),
		Environment: []string{"PATH=" + bin},
	})
	if err != nil {
		t.Fatal(err)
	}
	prober := NewLocalProber(ResolverExecutor{Resolver: resolver}, nil, nil, nil)
	observation := prober.Probe(context.Background(), ProbeRequest{
		AdapterID: "profile.claude.native", TargetID: "claude:configured-default",
		ProfileID: "claude:configured-default", PredicateID: "predicate.claude.native-contract.v1",
	})
	if observation.State != corecap.PredicateSatisfied {
		t.Fatalf("readiness observation = %#v", observation)
	}

	provider := native.Provider{
		Name:  "claude",
		Probe: []string{"claude", "--version"},
		Command: func(runtime.RunSpec) []string {
			return []string{"claude", "run"}
		},
	}
	nativeRuntime := native.New(t.TempDir(), provider)
	nativeRuntime.UseVerifiedExecutables(resolver)
	run, err := nativeRuntime.Dispatch(context.Background(), runtime.RunSpec{
		RunID: "identity-stable", Agent: "claude",
	})
	if err != nil {
		t.Fatalf("dispatch authorized executable: %v", err)
	}
	for range run.Stream() {
	}
	if status := run.Wait(); status.State != runtime.StateSucceeded {
		t.Fatalf("authorized executable status = %+v", status)
	}

	// Keep the same name and compatible version output but replace the bytes
	// after readiness. Dispatch must fail before either the probe or run starts.
	writeIdentityTestCommand(t, command, marker)
	if _, err := nativeRuntime.Dispatch(context.Background(), runtime.RunSpec{
		RunID: "identity-swap", Agent: "claude",
	}); err == nil {
		t.Fatal("dispatch accepted executable bytes that differed from readiness")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("replacement executable started; marker error = %v", err)
	}
}

func writeIdentityTestCommand(t *testing.T, path, marker string) {
	t.Helper()
	script := "#!/bin/sh\n"
	if marker != "" {
		script += "printf started > \"" + marker + "\"\n"
	}
	script += "if [ \"$1\" = \"--version\" ]; then printf '%s\\n' '2.1.207 (Claude Code)'; exit 0; fi\n"
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
}
