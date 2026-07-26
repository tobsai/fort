package capability

import (
	"context"
	"strings"
	"testing"

	corecap "github.com/tobsai/fort/core/capability"
)

type fakeCommandExecutor struct {
	results map[string]CommandResult
}

func (f fakeCommandExecutor) Run(_ context.Context, command string, args ...string) CommandResult {
	key := command + "\x00" + strings.Join(args, "\x00")
	if result, ok := f.results[key]; ok {
		return result
	}
	return CommandResult{Err: ErrCommandAbsent}
}

type fakeCodexInspector struct {
	result CodexInspection
	err    error
}

type panicCommandExecutor struct{}

func (panicCommandExecutor) Run(context.Context, string, ...string) CommandResult {
	panic("unsafe capability command was started")
}

func (f fakeCodexInspector) Inspect(context.Context) (CodexInspection, error) {
	return f.result, f.err
}

func TestLocalProberAcceptsOnlyExactCodexContractAndModelCatalog(t *testing.T) {
	commands := fakeCommandExecutor{results: map[string]CommandResult{
		"codex\x00--version": {
			Output: []byte("codex-cli 0.143.0\n"), ExecutableDigest: "codex-digest",
		},
	}}
	inspection := CodexInspection{
		AccountReady: true, AccountHandle: "account-handle",
		Models: map[string]bool{"gpt-5.5": true}, DefaultModel: "gpt-5.5",
		ExecutableDigest:         "codex-digest",
		NormalSchemaDigest:       "44b0f3e1bcddcee69d9b2dbdcfbfbb9252757f884946aeae698af6f82e439ebd",
		NormalSchemaFiles:        267,
		ExperimentalSchemaDigest: "e0ee3ce1d6b9aee796d4d0b00536d4aefeaf77641875577775e832cfae6445db",
		ExperimentalSchemaFiles:  337,
	}
	prober := NewLocalProber(commands, fakeCodexInspector{result: inspection}, nil, nil)

	native := prober.Probe(context.Background(), ProbeRequest{
		AdapterID: "profile.codex.native", TargetID: "codex:gpt-5.5",
		ProfileID: "codex:gpt-5.5", PredicateID: "predicate.codex.native-contract.v1",
	})
	if native.State != corecap.PredicateSatisfied || len(native.StableBinding) == 0 {
		t.Fatalf("native = %#v", native)
	}
	model := prober.Probe(context.Background(), ProbeRequest{
		AdapterID: "profile.codex.native", TargetID: "codex:gpt-5.5",
		ProfileID: "codex:gpt-5.5", PredicateID: "predicate.codex.model.codex:gpt-5.5.v1",
	})
	if model.State != corecap.PredicateSatisfied {
		t.Fatalf("model = %#v", model)
	}
	unavailable := prober.Probe(context.Background(), ProbeRequest{
		AdapterID: "profile.codex.native", TargetID: "codex:gpt-5.6-sol",
		ProfileID: "codex:gpt-5.6-sol", PredicateID: "predicate.codex.model.codex:gpt-5.6-sol.v1",
	})
	if unavailable.State != corecap.PredicateUnsatisfied || unavailable.Reason != corecap.ReasonModelUnavailable {
		t.Fatalf("unavailable = %#v", unavailable)
	}
}

func TestLocalProberRejectsConfiguredCodexDefaultMissingFromRuntimeCatalog(t *testing.T) {
	inspection := CodexInspection{
		AccountReady: true, AccountHandle: "account-handle",
		Models: map[string]bool{"gpt-5.5": true}, DefaultModel: "gpt-5.6-sol",
	}
	prober := NewLocalProber(fakeCommandExecutor{}, fakeCodexInspector{result: inspection}, nil, nil)
	observation := prober.Probe(context.Background(), ProbeRequest{
		AdapterID: "profile.codex.native", TargetID: "codex:configured-default",
		ProfileID: "codex:configured-default", PredicateID: "predicate.codex.model.codex:configured-default.v1",
	})
	if observation.State != corecap.PredicateUnsatisfied || observation.Reason != corecap.ReasonModelUnavailable {
		t.Fatalf("observation = %#v", observation)
	}
}

func TestLocalProberRejectsCodexFactsFromDifferentExecutableIdentity(t *testing.T) {
	commands := fakeCommandExecutor{results: map[string]CommandResult{
		"codex\x00--version": {Output: []byte("codex-cli 0.143.0\n"), ExecutableDigest: "version-digest"},
	}}
	inspection := CodexInspection{
		ExecutableDigest:   "schema-and-app-server-digest",
		NormalSchemaDigest: codexNormalSchemaDigest, NormalSchemaFiles: 267,
	}
	prober := NewLocalProber(commands, fakeCodexInspector{result: inspection}, nil, nil)
	observation := prober.Probe(context.Background(), ProbeRequest{
		AdapterID: "profile.codex.native", TargetID: "codex:gpt-5.5",
		ProfileID: "codex:gpt-5.5", PredicateID: "predicate.codex.native-contract.v1",
	})
	if observation.State != corecap.PredicateUnsatisfied || observation.Reason != corecap.ReasonIncompatibleVersion {
		t.Fatalf("observation = %#v", observation)
	}
}

func TestLocalProberFailsClosedOnVersionDriftWithoutLeakingOutput(t *testing.T) {
	sentinel := "PRIVATE-PROBE-OUTPUT"
	commands := fakeCommandExecutor{results: map[string]CommandResult{
		"codex\x00--version": {
			Output: []byte("codex-cli 0.144.0 " + sentinel), ExecutableDigest: "changed",
		},
	}}
	prober := NewLocalProber(commands, fakeCodexInspector{}, nil, nil)
	observation := prober.Probe(context.Background(), ProbeRequest{
		AdapterID: "profile.codex.native", TargetID: "codex:gpt-5.5",
		ProfileID: "codex:gpt-5.5", PredicateID: "predicate.codex.native-contract.v1",
	})
	if observation.State != corecap.PredicateUnsatisfied || observation.Reason != corecap.ReasonIncompatibleVersion {
		t.Fatalf("observation = %#v", observation)
	}
	if strings.Contains(strings.Join(observation.StableBinding, " "), sentinel) {
		t.Fatal("raw probe output leaked into stable binding")
	}
}

func TestLocalProberAcceptsHermesBuildDateSuffixWithoutAcceptingVersionDrift(t *testing.T) {
	tests := []struct {
		name   string
		output string
		state  corecap.PredicateState
	}{
		{name: "bare catalog version", output: "Hermes Agent v0.15.1\n", state: corecap.PredicateSatisfied},
		{name: "real build date suffix", output: "Hermes Agent v0.15.1 (2026.5.29)\n", state: corecap.PredicateSatisfied},
		{
			name:   "real multiline diagnostics",
			output: "Hermes Agent v0.15.1 (2026.5.29)\nProject: /private/path\nPython: 3.11.15\nUpdate available: run hermes update\n",
			state:  corecap.PredicateSatisfied,
		},
		{name: "patch drift", output: "Hermes Agent v0.15.10 (2026.5.29)\n", state: corecap.PredicateUnsatisfied},
		{name: "unrecognized suffix", output: "Hermes Agent v0.15.1 (private build)\n", state: corecap.PredicateUnsatisfied},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			prober := NewLocalProber(fakeCommandExecutor{results: map[string]CommandResult{
				"hermes\x00--version": {Output: []byte(test.output), ExecutableDigest: "hermes-digest"},
			}}, nil, nil, nil)
			observation := prober.Probe(context.Background(), ProbeRequest{
				AdapterID: "profile.hermes.native", TargetID: "hermes:configured-default",
				PredicateID: "predicate.hermes.native-contract.v1",
			})
			if observation.State != test.state {
				t.Fatalf("observation = %#v", observation)
			}
			if test.state == corecap.PredicateUnsatisfied && observation.Reason != corecap.ReasonIncompatibleVersion {
				t.Fatalf("reason = %q", observation.Reason)
			}
		})
	}
}

func TestLocalProberQuarantinesUnsafeOpenClawReadinessContractWithoutStartingIt(t *testing.T) {
	prober := NewLocalProber(panicCommandExecutor{}, nil, nil, nil)
	observation := prober.Probe(context.Background(), ProbeRequest{
		AdapterID: "profile.openclaw.main", TargetID: "openclaw:main",
		ProfileID: "openclaw:main", PredicateID: "predicate.openclaw.main-ready.v1",
	})
	if observation.State != corecap.PredicateUnsatisfied || observation.Reason != corecap.ReasonCommandContractChanged {
		t.Fatalf("observation = %#v", observation)
	}
}

func TestLocalProberClassifiesAbsentAndTimedOutCommands(t *testing.T) {
	prober := NewLocalProber(fakeCommandExecutor{}, nil, nil, nil)
	absent := prober.Probe(context.Background(), ProbeRequest{
		AdapterID: "profile.openclaw.main", TargetID: "openclaw:main",
		PredicateID: "predicate.openclaw.native-contract.v1",
	})
	if absent.Reason != corecap.ReasonAbsent {
		t.Fatalf("absent = %#v", absent)
	}

	prober = NewLocalProber(fakeCommandExecutor{results: map[string]CommandResult{
		"claude\x00--version": {Err: context.DeadlineExceeded},
	}}, nil, nil, nil)
	timedOut := prober.Probe(context.Background(), ProbeRequest{
		AdapterID: "profile.claude.native", TargetID: "claude:configured-default",
		PredicateID: "predicate.claude.native-contract.v1",
	})
	if timedOut.Reason != corecap.ReasonProbeTimedOut {
		t.Fatalf("timed out = %#v", timedOut)
	}
}

func TestLocalProberTreatsMissingPrivateBindingsAsSetup(t *testing.T) {
	prober := NewLocalProber(fakeCommandExecutor{}, nil, nil, nil)
	gmail := prober.Probe(context.Background(), ProbeRequest{
		AdapterID: "email.gmail.read.himalaya-broker", TargetID: "email.gmail.read",
		PredicateID: "predicate.gmail.selected-imap-preview-read.v1",
	})
	if gmail.Reason != corecap.ReasonAuthRequired {
		t.Fatalf("gmail = %#v", gmail)
	}
	supabase := prober.Probe(context.Background(), ProbeRequest{
		AdapterID: "database.supabase.inspect.codex-broker", TargetID: "database.supabase.inspect",
		PredicateID: "predicate.supabase.selected-project-readonly.v1",
	})
	if supabase.Reason != corecap.ReasonProjectUnavailable {
		t.Fatalf("supabase = %#v", supabase)
	}
}
