package control

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	corecap "github.com/tobsai/fort/core/capability"
	"github.com/tobsai/fort/core/machines"
	"github.com/tobsai/fort/core/runtime"
	execcap "github.com/tobsai/fort/exec/capability"
	"github.com/tobsai/fort/exec/fake"
)

type fakeLocalCapabilities struct {
	inventory corecap.NodeInventory
}

type blockingLocalCapabilities struct {
	inventory corecap.NodeInventory
	started   chan struct{}
	release   chan struct{}
}

func (f *blockingLocalCapabilities) Current() corecap.NodeInventory { return f.inventory }
func (f *blockingLocalCapabilities) Refresh(context.Context, corecap.RecheckRequest) (corecap.NodeInventory, error) {
	close(f.started)
	<-f.release
	return f.inventory, nil
}

type readyCapabilityProber struct{}

func (readyCapabilityProber) Probe(context.Context, execcap.ProbeRequest) execcap.ProbeObservation {
	return execcap.ProbeObservation{
		State:         corecap.PredicateSatisfied,
		StableBinding: []string{"stable-test-binding"},
	}
}

func (f fakeLocalCapabilities) Current() corecap.NodeInventory { return f.inventory }
func (f fakeLocalCapabilities) Refresh(context.Context, corecap.RecheckRequest) (corecap.NodeInventory, error) {
	return f.inventory, nil
}

type fakePeerCapabilities struct {
	mu        sync.Mutex
	results   map[string]corecap.NodeInventory
	errors    map[string]error
	active    int
	maxActive int
}

type blockingPeerCapabilities struct {
	calls   atomic.Int32
	started chan struct{}
	release chan struct{}
}

func (f *blockingPeerCapabilities) Refresh(_ context.Context, _, _ string, _ corecap.RecheckRequest) (corecap.NodeInventory, error) {
	f.calls.Add(1)
	close(f.started)
	<-f.release
	return corecap.NodeInventory{}, &execcap.DiscoveryError{Reason: corecap.ReasonUnavailable}
}

func (f *fakePeerCapabilities) Refresh(_ context.Context, baseURL, expectedNodeID string, _ corecap.RecheckRequest) (corecap.NodeInventory, error) {
	f.mu.Lock()
	f.active++
	if f.active > f.maxActive {
		f.maxActive = f.active
	}
	f.mu.Unlock()
	time.Sleep(time.Millisecond)
	f.mu.Lock()
	f.active--
	result, err := f.results[expectedNodeID], f.errors[expectedNodeID]
	f.mu.Unlock()
	return result, err
}

func TestCapabilityCoordinatorBindsRegistryIdentityAndPartialFailures(t *testing.T) {
	live := &machines.Live{}
	registry, err := machines.Parse([]byte(`
version: 1
machines:
  - {name: laptop, url: "http://laptop:4087", agents: [codex]}
  - {name: mini, url: "http://mini:4087", agents: [codex]}
  - {name: offline, url: "http://offline:4087", agents: [codex]}
`), "laptop")
	if err != nil {
		t.Fatal(err)
	}
	live.Store(registry)
	now := time.Unix(2000, 0).UTC()
	local := minimalNodeInventory("laptop")
	mini := minimalNodeInventory("mini")
	mini.ObservedAt = time.Unix(1, 0).UTC() // peer wall time is not freshness
	peers := &fakePeerCapabilities{
		results: map[string]corecap.NodeInventory{"mini": mini},
		errors: map[string]error{
			"offline": &execcap.DiscoveryError{Reason: corecap.ReasonUnavailable},
		},
	}
	coordinator, err := NewCapabilityCoordinator(CapabilityCoordinatorOptions{
		Live: live, LocalName: "laptop", Local: fakeLocalCapabilities{inventory: local},
		Peers: peers, Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, generation, err := coordinator.Refresh(context.Background(), corecap.RefreshPlanning, []string{"profile.codex.native"})
	if err != nil {
		t.Fatal(err)
	}
	if generation != 1 || len(snapshot.Machines) != 3 {
		t.Fatalf("generation=%d machines=%d", generation, len(snapshot.Machines))
	}
	if !snapshot.Machines[0].Local || snapshot.Machines[0].Name != "laptop" || snapshot.Machines[0].RegistryRank != 0 {
		t.Fatalf("local row = %#v", snapshot.Machines[0])
	}
	if snapshot.Machines[1].Name != "mini" || snapshot.Machines[1].Local || snapshot.Machines[1].RegistryRank != 1 ||
		!snapshot.Machines[1].ObservedAt.Equal(now) {
		t.Fatalf("mini row = %#v", snapshot.Machines[1])
	}
	if snapshot.Machines[2].Name != "offline" || snapshot.Machines[2].State != corecap.MachineUnknown ||
		snapshot.Machines[2].Reason != corecap.ReasonUnavailable || snapshot.Machines[2].Reachable {
		t.Fatalf("offline row = %#v", snapshot.Machines[2])
	}
	if peers.maxActive < 2 {
		t.Fatalf("peer refresh was not concurrent; max active = %d", peers.maxActive)
	}
}

func TestCapabilityCoordinatorRefreshesLocalAndPeerConcurrently(t *testing.T) {
	live := &machines.Live{}
	registry, err := machines.Parse([]byte(`
version: 1
machines:
  - {name: laptop, url: "http://laptop:4087", agents: [codex]}
  - {name: mini, url: "http://mini:4087", agents: [codex]}
`), "laptop")
	if err != nil {
		t.Fatal(err)
	}
	live.Store(registry)
	local := &blockingLocalCapabilities{
		inventory: minimalNodeInventory("laptop"),
		started:   make(chan struct{}), release: make(chan struct{}),
	}
	peer := &blockingPeerCapabilities{started: make(chan struct{}), release: make(chan struct{})}
	t.Cleanup(func() {
		select {
		case <-local.release:
		default:
			close(local.release)
		}
		select {
		case <-peer.release:
		default:
			close(peer.release)
		}
	})
	coordinator, err := NewCapabilityCoordinator(CapabilityCoordinatorOptions{
		Live: live, LocalName: "laptop", Local: local, Peers: peer,
	})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, _, err := coordinator.Refresh(context.Background(), corecap.RefreshPlanning, []string{"profile.codex.native"})
		done <- err
	}()
	select {
	case <-local.started:
	case <-time.After(time.Second):
		t.Fatal("local refresh did not start")
	}
	select {
	case <-peer.started:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("peer refresh did not overlap blocked local refresh")
	}
	close(local.release)
	close(peer.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestCapabilityCoordinatorFreshnessStartsAfterRefreshSettles(t *testing.T) {
	now := time.Unix(3000, 0).UTC()
	local := &blockingLocalCapabilities{
		inventory: minimalNodeInventory("laptop"),
		started:   make(chan struct{}), release: make(chan struct{}),
	}
	coordinator, err := NewCapabilityCoordinator(CapabilityCoordinatorOptions{
		Live: &machines.Live{}, LocalName: "laptop", Local: local,
		Peers: &fakePeerCapabilities{}, Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	result := make(chan corecap.Snapshot, 1)
	go func() {
		snapshot, _, _ := coordinator.Refresh(context.Background(), corecap.RefreshPlanning, []string{"profile.codex.native"})
		result <- snapshot
	}()
	<-local.started
	now = now.Add(20 * time.Second)
	close(local.release)
	snapshot := <-result
	if !snapshot.ObservedAt.Equal(now) || !snapshot.Machines[0].ObservedAt.Equal(now) {
		t.Fatalf("freshness = snapshot %s machine %s, want settlement %s", snapshot.ObservedAt, snapshot.Machines[0].ObservedAt, now)
	}
}

func TestCapabilityCoordinatorGenerationAdvancesWithoutTimestampRevisionChurn(t *testing.T) {
	now := time.Unix(3000, 0).UTC()
	coordinator, err := NewCapabilityCoordinator(CapabilityCoordinatorOptions{
		Live: &machines.Live{}, LocalName: "laptop",
		Local: fakeLocalCapabilities{inventory: minimalNodeInventory("laptop")},
		Peers: &fakePeerCapabilities{}, Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	first, generation1, err := coordinator.Refresh(context.Background(), corecap.RefreshPlanning, []string{"profile.codex.native"})
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute)
	second, generation2, err := coordinator.Refresh(context.Background(), corecap.RefreshPlanning, []string{"profile.codex.native"})
	if err != nil {
		t.Fatal(err)
	}
	if generation1 != 1 || generation2 != 2 || first.Revision != second.Revision {
		t.Fatalf("generations=%d/%d revisions=%s/%s", generation1, generation2, first.Revision, second.Revision)
	}
	current, currentGeneration := coordinator.Capabilities()
	if currentGeneration != generation2 || current.Revision != second.Revision {
		t.Fatalf("current generation=%d revision=%s", currentGeneration, current.Revision)
	}
}

func TestCapabilityCoordinatorPublishesRegistryInventoryWithNonNullVectors(t *testing.T) {
	registry, err := execcap.NewRegistry(execcap.RegistryOptions{
		NodeID: "laptop", Platform: "darwin/arm64",
		RevisionKey: []byte("01234567890123456789012345678901"),
		Prober:      readyCapabilityProber{},
	})
	if err != nil {
		t.Fatal(err)
	}
	coordinator, err := NewCapabilityCoordinator(CapabilityCoordinatorOptions{
		Live: &machines.Live{}, LocalName: "laptop", Local: registry,
		Peers: &fakePeerCapabilities{},
	})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, generation, err := coordinator.Refresh(context.Background(), corecap.RefreshPlanning, []string{
		"profile.claude.native",
		"profile.codex.native",
		"profile.hermes.native",
		"profile.openclaw.main",
	})
	if err != nil {
		t.Fatal(err)
	}
	if generation != 1 || len(snapshot.Machines) != 1 {
		t.Fatalf("generation=%d machines=%d", generation, len(snapshot.Machines))
	}
	machine := snapshot.Machines[0]
	if !machine.Reachable {
		t.Fatalf("successful local refresh was marked unreachable: state=%s reason=%s", machine.State, machine.Reason)
	}
	assertVectors := func(kind, id string, predicates []corecap.Predicate) {
		t.Helper()
		for _, predicate := range predicates {
			if predicate.DependsOn == nil || predicate.RemedyEffectIDs == nil {
				t.Fatalf("%s %q predicate %q contains a null public vector", kind, id, predicate.ID)
			}
		}
	}
	for _, profile := range machine.Profiles {
		assertVectors("profile", profile.ID, profile.Predicates)
	}
	for _, offer := range machine.Offers {
		assertVectors("offer", offer.ID, offer.Predicates)
	}
	for _, binding := range machine.Bindings {
		if binding.Capabilities == nil {
			t.Fatalf("binding %q/%q contains null capabilities", binding.ID, binding.Profile)
		}
		assertVectors("binding", binding.ID+"/"+binding.Profile, binding.Predicates)
	}
	next := fake.New()
	gate := execcap.NewProfileGate(next, coordinator)
	if _, err := gate.Dispatch(context.Background(), runtime.RunSpec{
		RunID: "run-ready-profile", Agent: "codex", Machine: "laptop",
	}); err != nil {
		t.Fatalf("ready local profile was blocked: %v", err)
	}
	if starts := len(next.Dispatched()); starts != 1 {
		t.Fatalf("provider starts=%d, want 1", starts)
	}
}

func TestProfileGateRefreshesOnlySelectedLocalMachine(t *testing.T) {
	live := &machines.Live{}
	registry, err := machines.Parse([]byte(`
version: 1
machines:
  - {name: laptop, url: "http://laptop:4087", agents: [codex]}
  - {name: unavailable, url: "http://unavailable:4087", agents: [codex]}
`), "laptop")
	if err != nil {
		t.Fatal(err)
	}
	live.Store(registry)
	local := minimalNodeInventory("laptop")
	local.State = corecap.MachineReady
	local.Reason = ""
	local.Profiles = []corecap.ProfileOffer{{
		ID: "codex:gpt-5.6-sol", Agent: "codex", Adapter: "profile.codex.native",
		State: corecap.OfferReady, BindingRevision: "opaque:test",
	}}
	peer := &blockingPeerCapabilities{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	defer close(peer.release)
	coordinator, err := NewCapabilityCoordinator(CapabilityCoordinatorOptions{
		Live: live, LocalName: "laptop", Local: fakeLocalCapabilities{inventory: local},
		Peers: peer,
	})
	if err != nil {
		t.Fatal(err)
	}
	next := fake.New()
	gate := execcap.NewProfileGate(next, coordinator)
	dispatched := make(chan error, 1)
	go func() {
		_, dispatchErr := gate.Dispatch(context.Background(), runtime.RunSpec{
			RunID: "run-1", Agent: "codex", Model: "5.6 Sol", Machine: "laptop",
		})
		dispatched <- dispatchErr
	}()

	select {
	case <-peer.started:
		t.Fatal("dispatch refreshed an unrelated peer")
	case dispatchErr := <-dispatched:
		if dispatchErr != nil {
			t.Fatal(dispatchErr)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("dispatch waited on an unrelated peer")
	}
	if calls := peer.calls.Load(); calls != 0 {
		t.Fatalf("peer refresh calls=%d, want 0", calls)
	}
	if starts := len(next.Dispatched()); starts != 1 {
		t.Fatalf("provider starts=%d, want 1", starts)
	}
}

func TestCapabilityCoordinatorRejectsMoreThanSixteenMachines(t *testing.T) {
	live := &machines.Live{}
	registry := &machines.Registry{Version: 1}
	for i := 0; i < 17; i++ {
		registry.Machines = append(registry.Machines, machines.Machine{
			Name: string(rune('a' + i)), URL: "http://127.0.0.1:4087", Agents: []string{"codex"},
		})
	}
	live.Store(registry)
	_, err := NewCapabilityCoordinator(CapabilityCoordinatorOptions{
		Live: live, LocalName: "a", Local: fakeLocalCapabilities{inventory: minimalNodeInventory("a")},
		Peers: &fakePeerCapabilities{},
	})
	if err == nil {
		t.Fatal("expected >16-machine registry to fail")
	}
}

func minimalNodeInventory(nodeID string) corecap.NodeInventory {
	return corecap.NodeInventory{
		ProtocolVersion: corecap.ProtocolVersion, CatalogVersion: corecap.CatalogVersion, ProfileMappingVersion: corecap.ProfileMappingVersion,
		NodeID: nodeID, State: corecap.MachinePartial, Reason: corecap.ReasonAuthRequired,
		Profiles: []corecap.ProfileOffer{}, Offers: []corecap.LogicalOffer{},
		Bindings: []corecap.ExecutionBindingOffer{},
	}
}
