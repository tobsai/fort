package capability

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	corecap "github.com/tobsai/fort/core/capability"
	"github.com/tobsai/fort/core/runtime"
	"github.com/tobsai/fort/exec/fake"
)

type fakeSnapshotRefresher struct {
	machine  corecap.MachineInventory
	err      error
	adapters []string
	target   string
	mode     corecap.RefreshMode
	calls    int
}

func (f *fakeSnapshotRefresher) RefreshMachine(_ context.Context, target string, mode corecap.RefreshMode, adapters []string) (corecap.MachineInventory, error) {
	f.calls++
	f.target = target
	f.mode = mode
	f.adapters = append([]string(nil), adapters...)
	return f.machine, f.err
}

func TestProfileGateDispatchesOnlyReadyExactProfile(t *testing.T) {
	next := fake.New()
	refresh := &fakeSnapshotRefresher{machine: machineWithProfile(
		"laptop", "codex:gpt-5.6-sol", corecap.OfferReady, "",
	)}
	gate := NewProfileGate(next, refresh)
	run, err := gate.Dispatch(context.Background(), runtime.RunSpec{
		RunID: "run-1", Agent: "codex", Model: "5.6 Sol", Machine: "laptop",
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = run.Wait()
	if got := len(next.Dispatched()); got != 1 {
		t.Fatalf("provider starts=%d, want 1", got)
	}
	if len(refresh.adapters) != 1 || refresh.adapters[0] != "profile.codex.native" {
		t.Fatalf("refreshed adapters=%v", refresh.adapters)
	}
	if refresh.target != "laptop" {
		t.Fatalf("refreshed machine=%q, want laptop", refresh.target)
	}
	if refresh.mode != corecap.RefreshUserRecheck {
		t.Fatalf("refresh mode=%q, want uncached target guard", refresh.mode)
	}
}

func TestProfileGateDispatchesFirstClassGPT55Profile(t *testing.T) {
	next := fake.New()
	refresh := &fakeSnapshotRefresher{machine: machineWithProfile(
		"laptop", "codex:gpt-5.5", corecap.OfferReady, "",
	)}
	gate := NewProfileGate(next, refresh)
	run, err := gate.Dispatch(context.Background(), runtime.RunSpec{
		RunID: "run-1", Profile: "codex:gpt-5.5", Agent: "codex", Model: "gpt-5.5", Machine: "laptop",
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = run.Wait()
	got := next.Dispatched()
	if len(got) != 1 || got[0].Profile != "" || got[0].Agent != "codex" || got[0].Model != "gpt-5.5" {
		t.Fatalf("dispatched = %+v, want lowered GPT-5.5 runtime selection", got)
	}
}

func TestProfileGatePinsResolvedDynamicModelForConversationDispatch(t *testing.T) {
	next := fake.New()
	refresh := &fakeSnapshotRefresher{machine: corecap.MachineInventory{
		Name: "laptop", Reachable: true,
		Profiles: []corecap.ProfileOffer{{
			ID: "codex:configured-default", Agent: "codex", State: corecap.OfferReady,
			ResolvedModel: "gpt-5.6-sol", BindingRevision: "opaque:dynamic-sol",
		}},
	}}
	gate := NewProfileGate(next, refresh)
	run, err := gate.Dispatch(context.Background(), runtime.RunSpec{
		RunID: "run-dynamic", Profile: "codex:configured-default", Agent: "codex",
		Model: "gpt-5.6-sol", Machine: "laptop",
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = run.Wait()
	got := next.Dispatched()
	if len(got) != 1 || got[0].Profile != "" || got[0].Agent != "codex" || got[0].Model != "gpt-5.6-sol" || got[0].Machine != "laptop" {
		t.Fatalf("dispatched = %+v, want unchanged pinned dynamic model", got)
	}
}

func TestProfileGateBlocksDynamicModelDriftBeforeProviderStart(t *testing.T) {
	next := fake.New()
	refresh := &fakeSnapshotRefresher{machine: corecap.MachineInventory{
		Name: "laptop", Reachable: true,
		Profiles: []corecap.ProfileOffer{{
			ID: "codex:configured-default", Agent: "codex", State: corecap.OfferReady,
			ResolvedModel: "gpt-5.6-terra", BindingRevision: "opaque:dynamic-terra",
		}},
	}}
	gate := NewProfileGate(next, refresh)
	_, err := gate.Dispatch(context.Background(), runtime.RunSpec{
		RunID: "run-dynamic", Profile: "codex:configured-default", Agent: "codex",
		Model: "gpt-5.6-sol", Machine: "laptop",
	})
	var blocked *ProfilePreflightError
	if !errors.As(err, &blocked) || blocked.Reason != corecap.ReasonCapabilityDrift {
		t.Fatalf("error=%v, want capability_drift", err)
	}
	if got := len(next.Dispatched()); got != 0 {
		t.Fatalf("provider starts=%d, want 0", got)
	}
}

func TestProfileGateBlocksMissingDynamicModelBeforeProviderStart(t *testing.T) {
	next := fake.New()
	refresh := &fakeSnapshotRefresher{machine: corecap.MachineInventory{
		Name: "laptop", Reachable: true,
		Profiles: []corecap.ProfileOffer{{
			ID: "codex:configured-default", Agent: "codex", State: corecap.OfferSetupRequired,
			Reason: corecap.ReasonModelUnavailable,
		}},
	}}
	gate := NewProfileGate(next, refresh)
	_, err := gate.Dispatch(context.Background(), runtime.RunSpec{
		RunID: "run-dynamic", Profile: "codex:configured-default", Agent: "codex",
		Model: "gpt-5.6-sol", Machine: "laptop",
	})
	var blocked *ProfilePreflightError
	if !errors.As(err, &blocked) || blocked.Reason != corecap.ReasonModelUnavailable {
		t.Fatalf("error=%v, want model_unavailable", err)
	}
	if got := len(next.Dispatched()); got != 0 {
		t.Fatalf("provider starts=%d, want 0", got)
	}
}

func TestProfileGatePreservesLegacyAmbientDynamicDispatch(t *testing.T) {
	next := fake.New()
	refresh := &fakeSnapshotRefresher{machine: corecap.MachineInventory{
		Name: "laptop", Reachable: true,
		Profiles: []corecap.ProfileOffer{{
			ID: "codex:configured-default", Agent: "codex", State: corecap.OfferReady,
			ResolvedModel: "gpt-5.6-sol", BindingRevision: "opaque:dynamic-sol",
		}},
	}}
	gate := NewProfileGate(next, refresh)
	run, err := gate.Dispatch(context.Background(), runtime.RunSpec{
		RunID: "run-legacy", Profile: "codex:configured-default", Agent: "codex", Machine: "laptop",
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = run.Wait()
	got := next.Dispatched()
	if len(got) != 1 || got[0].Profile != "" || got[0].Agent != "codex" || got[0].Model != "" {
		t.Fatalf("legacy ambient dispatch = %+v, want empty model preserved", got)
	}
}

func TestProfileGatePreservesSubscriptionAuthorityProfileForClosedRuntimeMux(t *testing.T) {
	next := fake.New()
	refresh := &fakeSnapshotRefresher{machine: machineWithProfile(
		"laptop", "codex-subscription:gpt-5.6-sol", corecap.OfferReady, "",
	)}
	gate := NewProfileGate(next, refresh)
	revision := strings.Repeat("a", 64)
	spec := runtime.RunSpec{
		RunID: "run-subscription", Profile: "codex-subscription:gpt-5.6-sol",
		Agent: "codex-subscription", Model: "gpt-5.6-sol", Machine: "laptop",
		Authority:              runtime.AuthorityChatSubscriptionIsolatedV1,
		RuntimeContract:        runtime.RuntimeContractCodexSubscriptionExecV1,
		ExpectedPolicyRevision: revision,
		TextOnlyPolicy: &runtime.TextOnlyPolicy{
			PolicyID: runtime.PolicyCodexSubscriptionChatV1, PolicyRevision: revision,
			Model: "gpt-5.6-sol", ReasoningEffort: runtime.ReasoningEffortMedium,
			ReasoningContext: runtime.ReasoningContextCurrentTurn, RequestTimeoutMillis: 120000,
			DeveloperInstructionRevision: revision, AccountType: runtime.AccountTypeChatGPT,
			AccountPlan: "pro", SelectedAdapterID: runtime.AdapterCodexSubscription,
			SelectedAdapterRevision: revision, SelectedCodexVersion: "codex-cli test",
			SelectedCodexExecutableRevision: revision, SelectedCodexSchemaRevision: revision,
			ThreadMode: runtime.ThreadModeEphemeral, SandboxMode: runtime.SandboxModeReadOnly,
			ApprovalPolicy: runtime.ApprovalPolicyNever, WorkdirMode: runtime.WorkdirModeEmptyPerTarget,
			DynamicToolsMode: runtime.ToolsModeNone, MCPMode: runtime.ToolsModeNone,
			CommandPolicy: runtime.ResourcePolicyDenyAndFail, FileReadPolicy: runtime.ResourcePolicyDenyAndFail,
			IsolationRevision: revision,
		},
	}
	if _, err := gate.Dispatch(context.Background(), spec); err != nil {
		t.Fatal(err)
	}
	got := next.Dispatched()
	if len(got) != 1 || got[0].Profile != spec.Profile || got[0].Authority != spec.Authority ||
		got[0].RuntimeContract != spec.RuntimeContract || got[0].TextOnlyPolicy == nil {
		t.Fatalf("subscription dispatch = %+v, want complete authority preserved", got)
	}
}

func TestProfileGateRejectsProfileIdentityMismatchWithoutProbe(t *testing.T) {
	next := fake.New()
	refresh := &fakeSnapshotRefresher{}
	gate := NewProfileGate(next, refresh)
	_, err := gate.Dispatch(context.Background(), runtime.RunSpec{
		RunID: "run-1", Profile: "codex:gpt-5.5", Agent: "codex", Model: "gpt-5.6-sol", Machine: "laptop",
	})
	var blocked *ProfilePreflightError
	if !errors.As(err, &blocked) || blocked.Reason != corecap.ReasonProfileUnmapped {
		t.Fatalf("error=%v, want profile_unmapped", err)
	}
	if refresh.calls != 0 || len(next.Dispatched()) != 0 {
		t.Fatalf("refresh calls=%d provider starts=%d", refresh.calls, len(next.Dispatched()))
	}
}

func TestFirstClassProfileDoesNotExtendLegacyExecJSON(t *testing.T) {
	data, err := json.Marshal(runtime.RunSpec{
		RunID: "run-1", Profile: "codex:gpt-5.5", Agent: "codex", Model: "gpt-5.5",
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "Profile") || strings.Contains(string(data), "codex:gpt-5.5") {
		t.Fatalf("legacy execution JSON leaked control-plane profile: %s", data)
	}
}

func TestProfileGateBlocksUnavailableProfileBeforeProviderStart(t *testing.T) {
	next := fake.New()
	refresh := &fakeSnapshotRefresher{machine: machineWithProfile(
		"laptop", "hermes:openai-codex/gpt-5.6-sol", corecap.OfferSetupRequired, corecap.ReasonModelUnavailable,
	)}
	gate := NewProfileGate(next, refresh)
	_, err := gate.Dispatch(context.Background(), runtime.RunSpec{
		RunID: "run-1", Agent: "hermes", Model: "Codex 5.6 Sol", Machine: "laptop",
	})
	var blocked *ProfilePreflightError
	if !errors.As(err, &blocked) || blocked.Reason != corecap.ReasonModelUnavailable {
		t.Fatalf("error=%v, want model_unavailable preflight", err)
	}
	if got := len(next.Dispatched()); got != 0 {
		t.Fatalf("provider starts=%d, want 0", got)
	}
}

func TestProfileGateBlocksUnmappedProfileWithoutProbe(t *testing.T) {
	next := fake.New()
	refresh := &fakeSnapshotRefresher{}
	gate := NewProfileGate(next, refresh)
	_, err := gate.Dispatch(context.Background(), runtime.RunSpec{RunID: "run-1", Agent: "hermes", Model: "invented"})
	var blocked *ProfilePreflightError
	if !errors.As(err, &blocked) || blocked.Reason != corecap.ReasonProfileUnmapped {
		t.Fatalf("error=%v, want profile_unmapped", err)
	}
	if refresh.calls != 0 || len(next.Dispatched()) != 0 {
		t.Fatalf("refresh calls=%d provider starts=%d", refresh.calls, len(next.Dispatched()))
	}
}

func TestProfileGatePreservesClosedMachineReasonWhenProfileIsMissing(t *testing.T) {
	next := fake.New()
	refresh := &fakeSnapshotRefresher{machine: corecap.MachineInventory{
		Name: "mini", Reachable: true, State: corecap.MachineUnknown,
		Reason: corecap.ReasonOldNode, Profiles: []corecap.ProfileOffer{},
	}}
	gate := NewProfileGate(next, refresh)
	_, err := gate.Dispatch(context.Background(), runtime.RunSpec{
		RunID: "run-1", Agent: "codex", Machine: "mini",
	})
	var blocked *ProfilePreflightError
	if !errors.As(err, &blocked) || blocked.Reason != corecap.ReasonOldNode {
		t.Fatalf("error=%v, want old_node preflight", err)
	}
	if got := len(next.Dispatched()); got != 0 {
		t.Fatalf("provider starts=%d, want 0", got)
	}
}

func machineWithProfile(machine, profile string, state corecap.OfferState, reason corecap.Reason) corecap.MachineInventory {
	revision := ""
	if state == corecap.OfferReady {
		revision = "opaque:test"
	}
	return corecap.MachineInventory{
		Name: machine, Local: true, Reachable: true, State: corecap.MachinePartial,
		Profiles: []corecap.ProfileOffer{{ID: profile, State: state, Reason: reason, BindingRevision: revision}},
	}
}
