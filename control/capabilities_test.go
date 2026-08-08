package control

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	corecap "github.com/tobsai/fort/core/capability"
	"github.com/tobsai/fort/core/machines"
	"github.com/tobsai/fort/core/runtime"
	execcap "github.com/tobsai/fort/exec/capability"
	"github.com/tobsai/fort/exec/fake"
)

type fakeLocalCapabilities struct {
	inventory corecap.NodeInventory
}

type recordingLocalCapabilities struct {
	inventory corecap.NodeInventory
	request   corecap.RecheckRequest
}

func (f *recordingLocalCapabilities) Current() corecap.NodeInventory { return f.inventory }
func (f *recordingLocalCapabilities) Refresh(_ context.Context, request corecap.RecheckRequest) (corecap.NodeInventory, error) {
	f.request = request
	return f.inventory, nil
}

type recordingPeerCapabilities struct {
	inventory corecap.NodeInventory
	request   corecap.RecheckRequest
}

func (f *recordingPeerCapabilities) Refresh(_ context.Context, _, _ string, request corecap.RecheckRequest) (corecap.NodeInventory, error) {
	f.request = request
	return f.inventory, nil
}

type blockingLocalCapabilities struct {
	inventory corecap.NodeInventory
	started   chan struct{}
	release   chan struct{}
}

type sequencedLocalCapabilities struct {
	calls         atomic.Int32
	firstStarted  chan struct{}
	secondStarted chan struct{}
	releaseFirst  chan struct{}
	first         corecap.NodeInventory
	second        corecap.NodeInventory
}

func (f *sequencedLocalCapabilities) Current() corecap.NodeInventory { return f.first }
func (f *sequencedLocalCapabilities) Refresh(context.Context, corecap.RecheckRequest) (corecap.NodeInventory, error) {
	switch f.calls.Add(1) {
	case 1:
		close(f.firstStarted)
		<-f.releaseFirst
		return f.first, nil
	default:
		close(f.secondStarted)
		return f.second, nil
	}
}

func (f *blockingLocalCapabilities) Current() corecap.NodeInventory { return f.inventory }
func (f *blockingLocalCapabilities) Refresh(context.Context, corecap.RecheckRequest) (corecap.NodeInventory, error) {
	close(f.started)
	<-f.release
	return f.inventory, nil
}

type readyCapabilityProber struct{}

func (readyCapabilityProber) Probe(_ context.Context, request execcap.ProbeRequest) execcap.ProbeObservation {
	observation := execcap.ProbeObservation{
		State:         corecap.PredicateSatisfied,
		StableBinding: []string{"stable-test-binding"},
	}
	if request.ProfileID == "codex:configured-default" && request.PredicateID == "predicate.codex.model.codex:configured-default.v1" {
		observation.ResolvedModel = "gpt-5.6-sol"
	}
	return observation
}

type mutableDynamicCapabilityProber struct {
	mu    sync.Mutex
	model string
}

func (p *mutableDynamicCapabilityProber) Probe(_ context.Context, request execcap.ProbeRequest) execcap.ProbeObservation {
	p.mu.Lock()
	model := p.model
	p.mu.Unlock()
	observation := execcap.ProbeObservation{
		State:         corecap.PredicateSatisfied,
		StableBinding: []string{"stable-test-binding"},
	}
	if request.ProfileID == "codex:configured-default" && request.PredicateID == "predicate.codex.model.codex:configured-default.v1" {
		observation.ResolvedModel = model
	}
	return observation
}

func (p *mutableDynamicCapabilityProber) setModel(model string) {
	p.mu.Lock()
	p.model = model
	p.mu.Unlock()
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
	if len(snapshot.Machines[2].Profiles) != 0 || len(snapshot.Machines[2].Offers) != 0 || len(snapshot.Machines[2].Bindings) != 0 {
		t.Fatalf("first-seen offline registry claim became capability inventory: %#v", snapshot.Machines[2])
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

func TestCapabilityCoordinatorPublishesRefreshesInRequestOrder(t *testing.T) {
	local := &sequencedLocalCapabilities{
		firstStarted: make(chan struct{}), secondStarted: make(chan struct{}), releaseFirst: make(chan struct{}),
		first: minimalNodeInventory("laptop"), second: readyNodeInventory(t, "laptop"),
	}
	coordinator, err := NewCapabilityCoordinator(CapabilityCoordinatorOptions{
		Live: &machines.Live{}, LocalName: "laptop", Local: local, Peers: &fakePeerCapabilities{},
	})
	if err != nil {
		t.Fatal(err)
	}
	firstDone := make(chan error, 1)
	go func() {
		_, _, refreshErr := coordinator.Refresh(context.Background(), corecap.RefreshPlanning, []string{"profile.codex.native"})
		firstDone <- refreshErr
	}()
	<-local.firstStarted
	secondDone := make(chan error, 1)
	go func() {
		_, _, refreshErr := coordinator.Refresh(context.Background(), corecap.RefreshUserRecheck, []string{"profile.codex.native"})
		secondDone <- refreshErr
	}()
	var secondErr error
	secondCompleted := false
	select {
	case secondErr = <-secondDone:
		secondCompleted = true
	case <-time.After(100 * time.Millisecond):
	}
	close(local.releaseFirst)
	if err := <-firstDone; err != nil {
		t.Fatal(err)
	}
	if !secondCompleted {
		secondErr = <-secondDone
	}
	if secondErr != nil {
		t.Fatal(secondErr)
	}
	select {
	case <-local.secondStarted:
	default:
		t.Fatal("later refresh never reached the local registry")
	}
	snapshot, generation := coordinator.Current()
	readyResolvedDefault := false
	if len(snapshot.Machines) == 1 {
		for _, profile := range snapshot.Machines[0].Profiles {
			if profile.ID == "codex:configured-default" && profile.State == corecap.OfferReady && profile.ResolvedModel == "gpt-5.6-sol" {
				readyResolvedDefault = true
			}
		}
	}
	if generation != 2 || !readyResolvedDefault {
		t.Fatalf("current refresh generation=%d snapshot=%#v, want the later explicit recheck", generation, snapshot)
	}
}

func TestConversationSeatRecheckPublishesExplicitProfileRefresh(t *testing.T) {
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
	local := &recordingLocalCapabilities{inventory: minimalNodeInventory("laptop")}
	peer := &recordingPeerCapabilities{inventory: minimalNodeInventory("mini")}
	coordinator, err := NewCapabilityCoordinator(CapabilityCoordinatorOptions{
		Live: live, LocalName: "laptop", Local: local, Peers: peer,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := coordinator.RecheckConversationSeats(context.Background()); err != nil {
		t.Fatal(err)
	}
	_, generation := coordinator.Current()
	if generation != 1 {
		t.Fatalf("published generation = %d, want 1", generation)
	}
	wantAdapters := []string{
		"profile.claude.native",
		"profile.codex.native",
		"profile.hermes.native",
		"profile.openclaw.main",
	}
	for name, request := range map[string]corecap.RecheckRequest{"local": local.request, "peer": peer.request} {
		if request.Mode != corecap.RefreshUserRecheck || request.MaxAgeSeconds != 0 {
			t.Errorf("%s mode = %q max_age=%d, want explicit user recheck", name, request.Mode, request.MaxAgeSeconds)
		}
		if !reflect.DeepEqual(request.Adapters, wantAdapters) {
			t.Errorf("%s adapters = %#v, want exact execution profiles %#v", name, request.Adapters, wantAdapters)
		}
		if request.RequestID == "" || request.RequestID != local.request.RequestID {
			t.Errorf("%s request id = %q, want one shared non-empty refresh id", name, request.RequestID)
		}
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

func TestCapabilityCoordinatorKeepsKnownPeerProfilesUnavailableWhenPeerGoesOffline(t *testing.T) {
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
	peerInventory := readyNodeInventory(t, "mini")
	peers := &fakePeerCapabilities{results: map[string]corecap.NodeInventory{"mini": peerInventory}, errors: map[string]error{}}
	coordinator, err := NewCapabilityCoordinator(CapabilityCoordinatorOptions{
		Live: live, LocalName: "laptop", Local: fakeLocalCapabilities{inventory: minimalNodeInventory("laptop")},
		Peers: peers,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := coordinator.Refresh(context.Background(), corecap.RefreshUserRecheck, []string{"profile.codex.native"}); err != nil {
		t.Fatal(err)
	}

	peers.mu.Lock()
	peers.errors["mini"] = &execcap.DiscoveryError{Reason: corecap.ReasonUnavailable}
	peers.mu.Unlock()
	snapshot, _, err := coordinator.Refresh(context.Background(), corecap.RefreshUserRecheck, []string{"profile.codex.native"})
	if err != nil {
		t.Fatal(err)
	}
	mini := snapshot.Machines[1]
	if mini.Reachable || mini.State != corecap.MachineUnknown || mini.Reason != corecap.ReasonUnavailable {
		t.Fatalf("offline machine = %#v", mini)
	}
	if len(mini.Profiles) == 0 {
		t.Fatal("offline peer lost every previously verified profile identity")
	}
	if len(mini.Offers) != 0 || len(mini.Bindings) != 0 {
		t.Fatalf("offline peer retained stale logical capabilities or bindings: offers=%#v bindings=%#v", mini.Offers, mini.Bindings)
	}
	for _, profile := range mini.Profiles {
		if profile.State != corecap.OfferUnknown || profile.Reason != corecap.ReasonUnavailable || profile.BindingRevision != "" {
			t.Fatalf("offline profile = %#v, want unknown/unreachable with no reusable revision", profile)
		}
		if len(profile.Predicates) == 0 {
			t.Fatalf("offline profile lost its complete predicate shape: %#v", profile)
		}
		for _, predicate := range profile.Predicates {
			if predicate.State == corecap.PredicateSatisfied {
				t.Fatalf("offline profile retained a satisfied predicate: %#v", profile)
			}
		}
	}
	seats := (SnapshotConversationSeats{Source: coordinator}).ConversationSeats()
	found := false
	for _, seat := range seats {
		if seat.Profile != "codex:gpt-5.6-sol" || seat.Model != "gpt-5.6-sol" || seat.Machine != "mini" {
			continue
		}
		found = true
		if !strings.HasPrefix(seat.ID, "seat:v1:") || strings.Contains(seat.ID, seat.Profile) ||
			strings.Contains(seat.ID, seat.Machine) || seat.State != string(corecap.OfferUnavailable) ||
			seat.Reason != string(corecap.ReasonUnavailable) {
			t.Fatalf("offline conversation seat = %#v", seat)
		}
	}
	if !found {
		t.Fatalf("known offline seat missing from projection: %#v", seats)
	}
	snapshot, _, err = coordinator.Refresh(context.Background(), corecap.RefreshUserRecheck, []string{"profile.codex.native"})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Machines[1].Profiles) != len(mini.Profiles) {
		t.Fatalf("repeated outage lost known profile identities: first=%d second=%d", len(mini.Profiles), len(snapshot.Machines[1].Profiles))
	}

	peers.mu.Lock()
	peers.errors["mini"] = &execcap.DiscoveryError{Reason: corecap.ReasonOldNode}
	peers.mu.Unlock()
	snapshot, _, err = coordinator.Refresh(context.Background(), corecap.RefreshUserRecheck, []string{"profile.codex.native"})
	if err != nil {
		t.Fatal(err)
	}
	if profiles := snapshot.Machines[1].Profiles; len(profiles) != 0 {
		t.Fatalf("old-node response retained hypothetical profile mappings: %#v", profiles)
	}
	peers.mu.Lock()
	peers.errors["mini"] = &execcap.DiscoveryError{Reason: corecap.ReasonUnavailable}
	peers.mu.Unlock()
	snapshot, _, err = coordinator.Refresh(context.Background(), corecap.RefreshUserRecheck, []string{"profile.codex.native"})
	if err != nil {
		t.Fatal(err)
	}
	if profiles := snapshot.Machines[1].Profiles; len(profiles) != 0 {
		t.Fatalf("later outage resurrected profiles discarded after old-node result: %#v", profiles)
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

func TestProfileGateBypassesPlanningCacheBeforeDynamicDispatch(t *testing.T) {
	prober := &mutableDynamicCapabilityProber{model: "gpt-5.6-sol"}
	registry, err := execcap.NewRegistry(execcap.RegistryOptions{
		NodeID: "laptop", Platform: "darwin/arm64",
		RevisionKey: []byte("01234567890123456789012345678901"),
		Prober:      prober,
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
	if _, _, err := coordinator.Refresh(context.Background(), corecap.RefreshUserRecheck, []string{"profile.codex.native"}); err != nil {
		t.Fatal(err)
	}
	prober.setModel("gpt-5.6-terra")

	next := fake.New()
	gate := execcap.NewProfileGate(next, coordinator)
	_, err = gate.Dispatch(context.Background(), runtime.RunSpec{
		RunID: "run-dynamic", Profile: "codex:configured-default", Agent: "codex",
		Model: "gpt-5.6-sol", Machine: "laptop",
	})
	var blocked *execcap.ProfilePreflightError
	if !errors.As(err, &blocked) || blocked.Reason != corecap.ReasonCapabilityDrift {
		t.Fatalf("error=%v, want capability_drift", err)
	}
	if starts := len(next.Dispatched()); starts != 0 {
		t.Fatalf("provider starts=%d, want 0", starts)
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

func readyNodeInventory(t *testing.T, nodeID string) corecap.NodeInventory {
	t.Helper()
	registry, err := execcap.NewRegistry(execcap.RegistryOptions{
		NodeID: nodeID, Platform: "darwin/arm64",
		RevisionKey: []byte("01234567890123456789012345678901"),
		Prober:      readyCapabilityProber{},
	})
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := registry.Refresh(context.Background(), execcap.RecheckAll(corecap.RefreshUserRecheck, uuid.NewString()))
	if err != nil {
		t.Fatal(err)
	}
	return inventory
}
