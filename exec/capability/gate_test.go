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
	calls    int
}

func (f *fakeSnapshotRefresher) RefreshMachine(_ context.Context, target string, _ corecap.RefreshMode, adapters []string) (corecap.MachineInventory, error) {
	f.calls++
	f.target = target
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
