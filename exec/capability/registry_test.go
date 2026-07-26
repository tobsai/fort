package capability

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	corecap "github.com/tobsai/fort/core/capability"
)

type fakeProber struct {
	mu                 sync.Mutex
	active             int
	maxActive          int
	activeByAdapter    map[string]int
	maxActiveByAdapter map[string]int
	calls              map[string]int
	observations       map[string]ProbeObservation
}

type probeFunc func(context.Context, ProbeRequest) ProbeObservation

func (f probeFunc) Probe(ctx context.Context, request ProbeRequest) ProbeObservation {
	return f(ctx, request)
}

type blockingPredicateProber struct {
	mu      sync.Mutex
	calls   int
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

type cancelablePredicateProber struct {
	mu      sync.Mutex
	block   bool
	calls   int
	entered chan struct{}
	once    sync.Once
}

func (p *cancelablePredicateProber) Probe(ctx context.Context, request ProbeRequest) ProbeObservation {
	if request.PredicateID == "predicate.codex.native-contract.v1" {
		p.mu.Lock()
		p.calls++
		block := p.block
		p.mu.Unlock()
		if block {
			p.once.Do(func() { close(p.entered) })
			<-ctx.Done()
			return unsatisfied(corecap.ReasonProbeTimedOut)
		}
	}
	return satisfied("stable=" + request.PredicateID)
}

func (p *cancelablePredicateProber) setBlocked() {
	p.mu.Lock()
	p.block = true
	p.mu.Unlock()
}

func (p *cancelablePredicateProber) callCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.calls
}

func (p *blockingPredicateProber) Probe(_ context.Context, request ProbeRequest) ProbeObservation {
	if request.PredicateID == "predicate.codex.native-contract.v1" {
		p.mu.Lock()
		p.calls++
		p.mu.Unlock()
		p.once.Do(func() { close(p.entered) })
		<-p.release
	}
	return satisfied("stable=" + request.PredicateID)
}

func (p *blockingPredicateProber) callCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.calls
}

func (f *fakeProber) Probe(ctx context.Context, request ProbeRequest) ProbeObservation {
	f.mu.Lock()
	if f.calls == nil {
		f.calls = map[string]int{}
	}
	if f.activeByAdapter == nil {
		f.activeByAdapter = map[string]int{}
		f.maxActiveByAdapter = map[string]int{}
	}
	f.calls[request.PredicateID]++
	f.active++
	f.activeByAdapter[request.AdapterID]++
	if f.active > f.maxActive {
		f.maxActive = f.active
	}
	if f.activeByAdapter[request.AdapterID] > f.maxActiveByAdapter[request.AdapterID] {
		f.maxActiveByAdapter[request.AdapterID] = f.activeByAdapter[request.AdapterID]
	}
	observation, ok := f.observations[request.PredicateID]
	f.mu.Unlock()

	select {
	case <-ctx.Done():
		observation = ProbeObservation{State: corecap.PredicateUnsatisfied, Reason: corecap.ReasonProbeTimedOut}
	case <-time.After(time.Millisecond):
	}
	if !ok {
		observation = ProbeObservation{
			State:         corecap.PredicateSatisfied,
			StableBinding: []string{"stable:" + request.PredicateID},
		}
	}

	f.mu.Lock()
	f.active--
	f.activeByAdapter[request.AdapterID]--
	f.mu.Unlock()
	return observation
}

func (f *fakeProber) callCount(predicate string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[predicate]
}

func (f *fakeProber) maximumForAdapter(adapter string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.maxActiveByAdapter[adapter]
}

func TestRegistryBuildsCompleteClosedInventoryWithTwoProbeLimit(t *testing.T) {
	prober := &fakeProber{}
	registry, err := NewRegistry(RegistryOptions{
		NodeID: "node-laptop", Platform: "darwin/arm64",
		RevisionKey: []byte("01234567890123456789012345678901"),
		Prober:      prober,
	})
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := registry.Refresh(context.Background(), RecheckAll(corecap.RefreshUserRecheck, uuid.NewString()))
	if err != nil {
		t.Fatal(err)
	}
	if len(inventory.Profiles) != 9 || len(inventory.Offers) != 2 || len(inventory.Bindings) != 15 {
		t.Fatalf("inventory sizes = profiles:%d offers:%d bindings:%d", len(inventory.Profiles), len(inventory.Offers), len(inventory.Bindings))
	}
	if inventory.State != corecap.MachineReady || inventory.Reason != "" {
		t.Fatalf("machine state = %s/%s", inventory.State, inventory.Reason)
	}
	if prober.maxActive > 2 {
		t.Fatalf("max concurrent probes = %d, want <=2", prober.maxActive)
	}
	for _, adapter := range allAdapterIDs(corecap.CatalogV1()) {
		if got := prober.maximumForAdapter(adapter); got > 1 {
			t.Fatalf("adapter %s ran %d probes concurrently, want <=1", adapter, got)
		}
	}
	for _, profile := range inventory.Profiles {
		if profile.State != corecap.OfferReady || !strings.HasPrefix(profile.BindingRevision, "opaque:") {
			t.Fatalf("profile = %#v", profile)
		}
	}
	encoded := mustJSON(t, inventory)
	if strings.Contains(encoded, "stable:") || strings.Contains(encoded, "0123456789") {
		t.Fatal("private probe material or revision key leaked into inventory")
	}
}

func TestRegistryBlocksUnavailableExactModelBeforeBinding(t *testing.T) {
	prober := &fakeProber{observations: map[string]ProbeObservation{
		"predicate.codex.model.codex:gpt-5.6-sol.v1": {
			State: corecap.PredicateUnsatisfied, Reason: corecap.ReasonModelUnavailable,
		},
	}}
	registry, err := NewRegistry(RegistryOptions{
		NodeID: "node-laptop", Platform: "darwin/arm64",
		RevisionKey: []byte("01234567890123456789012345678901"), Prober: prober,
	})
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := registry.Refresh(context.Background(), RecheckAll(corecap.RefreshUserRecheck, uuid.NewString()))
	if err != nil {
		t.Fatal(err)
	}
	profile := findProfile(t, inventory, "codex:gpt-5.6-sol")
	if profile.State != corecap.OfferSetupRequired || profile.Reason != corecap.ReasonModelUnavailable || profile.BindingRevision != "" {
		t.Fatalf("profile = %#v", profile)
	}
	for _, binding := range inventory.Bindings {
		if binding.Profile == profile.ID && binding.State == corecap.OfferReady {
			t.Fatalf("binding became ready despite unavailable exact model: %#v", binding)
		}
	}
}

func TestPlanningCacheAndUserRecheckSemantics(t *testing.T) {
	now := time.Unix(1000, 0).UTC()
	prober := &fakeProber{}
	registry, err := NewRegistry(RegistryOptions{
		NodeID: "node-laptop", Platform: "darwin/arm64",
		RevisionKey: []byte("01234567890123456789012345678901"), Prober: prober,
		Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	adapter := []string{"profile.codex.native"}
	request := corecap.RecheckRequest{
		ProtocolVersion: 1, RequestID: uuid.NewString(), Mode: corecap.RefreshPlanning,
		MaxAgeSeconds: 60, Adapters: adapter,
	}
	if _, err := registry.Refresh(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	predicate := "predicate.codex.native-contract.v1"
	first := prober.callCount(predicate)
	request.RequestID = uuid.NewString()
	if _, err := registry.Refresh(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if got := prober.callCount(predicate); got != first {
		t.Fatalf("fresh planning refresh reprobed: %d -> %d", first, got)
	}

	user := corecap.RecheckRequest{
		ProtocolVersion: 1, RequestID: uuid.NewString(), Mode: corecap.RefreshUserRecheck,
		MaxAgeSeconds: 0, Adapters: adapter,
	}
	if _, err := registry.Refresh(context.Background(), user); err != nil {
		t.Fatal(err)
	}
	if got := prober.callCount(predicate); got <= first {
		t.Fatalf("user Recheck did not bypass cache: %d -> %d", first, got)
	}
}

func TestRegistrySingleFlightsSharedPredicatesWithinRefresh(t *testing.T) {
	prober := &fakeProber{}
	registry, err := NewRegistry(RegistryOptions{
		NodeID: "node-laptop", Platform: "darwin/arm64",
		RevisionKey: []byte("01234567890123456789012345678901"), Prober: prober,
	})
	if err != nil {
		t.Fatal(err)
	}
	request := corecap.RecheckRequest{
		ProtocolVersion: 1, RequestID: uuid.NewString(), Mode: corecap.RefreshUserRecheck,
		MaxAgeSeconds: 0, Adapters: []string{"profile.codex.native"},
	}
	if _, err := registry.Refresh(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if got := prober.callCount("predicate.codex.native-contract.v1"); got != 1 {
		t.Fatalf("shared native predicate probed %d times, want one", got)
	}
	if got := prober.callCount("predicate.codex.authenticated-subject.v1"); got != 1 {
		t.Fatalf("shared auth predicate probed %d times, want one", got)
	}
}

func TestRegistrySingleFlightsSharedPredicateAcrossConcurrentRefreshes(t *testing.T) {
	prober := &blockingPredicateProber{entered: make(chan struct{}), release: make(chan struct{})}
	registry, err := NewRegistry(RegistryOptions{
		NodeID: "node-laptop", Platform: "darwin/arm64",
		RevisionKey: []byte("01234567890123456789012345678901"), Prober: prober,
	})
	if err != nil {
		t.Fatal(err)
	}
	refresh := func() error {
		_, err := registry.Refresh(context.Background(), corecap.RecheckRequest{
			ProtocolVersion: 1, RequestID: uuid.NewString(), Mode: corecap.RefreshUserRecheck,
			MaxAgeSeconds: 0, Adapters: []string{"profile.codex.native"},
		})
		return err
	}
	first := make(chan error, 1)
	second := make(chan error, 1)
	go func() { first <- refresh() }()
	<-prober.entered
	go func() { second <- refresh() }()
	time.Sleep(20 * time.Millisecond)
	close(prober.release)
	if err := <-first; err != nil {
		t.Fatal(err)
	}
	if err := <-second; err != nil {
		t.Fatal(err)
	}
	if got := prober.callCount(); got != 1 {
		t.Fatalf("concurrent refreshes ran shared predicate %d times, want one", got)
	}
}

func TestRegistrySemaphoreWaitHonorsRefreshCancellation(t *testing.T) {
	prober := &fakeProber{}
	registry, err := NewRegistry(RegistryOptions{
		NodeID: "node-laptop", Platform: "darwin/arm64",
		RevisionKey: []byte("01234567890123456789012345678901"), Prober: prober,
	})
	if err != nil {
		t.Fatal(err)
	}
	registry.semaphore <- struct{}{}
	registry.semaphore <- struct{}{}
	go func() {
		time.Sleep(200 * time.Millisecond)
		<-registry.semaphore
		<-registry.semaphore
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err = registry.Refresh(ctx, corecap.RecheckRequest{
		ProtocolVersion: 1, RequestID: uuid.NewString(), Mode: corecap.RefreshUserRecheck,
		MaxAgeSeconds: 0, Adapters: []string{"profile.codex.native"},
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("refresh error = %v, want deadline exceeded", err)
	}
	if elapsed := time.Since(started); elapsed >= 150*time.Millisecond {
		t.Fatalf("canceled refresh waited for semaphore release: %s", elapsed)
	}
	if got := prober.callCount("predicate.codex.native-contract.v1"); got != 0 {
		t.Fatalf("canceled semaphore wait still started %d probes", got)
	}
}

func TestCanceledRefreshDoesNotOverwriteCacheOrCurrentInventory(t *testing.T) {
	prober := &cancelablePredicateProber{entered: make(chan struct{})}
	registry, err := NewRegistry(RegistryOptions{
		NodeID: "node-laptop", Platform: "darwin/arm64",
		RevisionKey: []byte("01234567890123456789012345678901"), Prober: prober,
	})
	if err != nil {
		t.Fatal(err)
	}
	request := corecap.RecheckRequest{
		ProtocolVersion: 1, RequestID: uuid.NewString(), Mode: corecap.RefreshUserRecheck,
		MaxAgeSeconds: 0, Adapters: []string{"profile.codex.native"},
	}
	before, err := registry.Refresh(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	prober.setBlocked()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	request.RequestID = uuid.NewString()
	go func() {
		_, err := registry.Refresh(ctx, request)
		done <- err
	}()
	<-prober.entered
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled refresh error = %v, want context canceled", err)
	}
	after := registry.Current()
	if before.ObservedAt != after.ObservedAt || before.State != after.State || before.Reason != after.Reason {
		t.Fatalf("canceled refresh replaced current inventory: before=%#v after=%#v", before, after)
	}
	request.Mode = corecap.RefreshPlanning
	request.MaxAgeSeconds = 60
	request.RequestID = uuid.NewString()
	inventory, err := registry.Refresh(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	profile := findProfile(t, inventory, "codex:gpt-5.5")
	if profile.State != corecap.OfferReady {
		t.Fatalf("canceled refresh poisoned ready cache: %#v", profile)
	}
	if got := prober.callCount(); got != 2 {
		t.Fatalf("native probe calls = %d, want initial plus canceled probe", got)
	}
}

func TestReachableMachineWithUnscheduledOffersIsPartial(t *testing.T) {
	registry, err := NewRegistry(RegistryOptions{
		NodeID: "node-laptop", Platform: "darwin/arm64",
		RevisionKey: []byte("01234567890123456789012345678901"), Prober: &fakeProber{},
	})
	if err != nil {
		t.Fatal(err)
	}
	request := corecap.RecheckRequest{
		ProtocolVersion: 1, RequestID: uuid.NewString(), Mode: corecap.RefreshPlanning,
		MaxAgeSeconds: 60, Adapters: []string{"profile.codex.native"},
	}
	inventory, err := registry.Refresh(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if inventory.State != corecap.MachinePartial {
		t.Fatalf("reachable machine state = %s/%s, want partial", inventory.State, inventory.Reason)
	}
}

func TestPlanningBackoffPublishesStaleWithoutReprobe(t *testing.T) {
	now := time.Unix(1000, 0).UTC()
	prober := &fakeProber{observations: map[string]ProbeObservation{
		"predicate.codex.native-contract.v1": {
			State: corecap.PredicateUnsatisfied, Reason: corecap.ReasonProbeFailed,
		},
	}}
	registry, err := NewRegistry(RegistryOptions{
		NodeID: "node-laptop", Platform: "darwin/arm64",
		RevisionKey: []byte("01234567890123456789012345678901"), Prober: prober,
		Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	request := corecap.RecheckRequest{
		ProtocolVersion: 1, RequestID: uuid.NewString(), Mode: corecap.RefreshPlanning,
		MaxAgeSeconds: 60, Adapters: []string{"profile.codex.native"},
	}
	for _, advance := range []time.Duration{0, 61 * time.Second, 61 * time.Second} {
		now = now.Add(advance)
		request.RequestID = uuid.NewString()
		if _, err := registry.Refresh(context.Background(), request); err != nil {
			t.Fatal(err)
		}
	}
	before := prober.callCount("predicate.codex.native-contract.v1")
	now = now.Add(61 * time.Second) // cache expired; third-failure backoff remains active.
	request.RequestID = uuid.NewString()
	inventory, err := registry.Refresh(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if got := prober.callCount("predicate.codex.native-contract.v1"); got != before {
		t.Fatalf("planning refresh ignored failure backoff: probes %d -> %d", before, got)
	}
	profile := findProfile(t, inventory, "codex:gpt-5.5")
	if profile.State != corecap.OfferUnknown || profile.Reason != corecap.ReasonStale {
		t.Fatalf("backoff profile = %#v, want unknown/stale", profile)
	}
}

func TestStableNegativeDoesNotEnterFailureBackoff(t *testing.T) {
	now := time.Unix(1000, 0).UTC()
	prober := &fakeProber{observations: map[string]ProbeObservation{
		"predicate.codex.native-contract.v1": {
			State: corecap.PredicateUnsatisfied, Reason: corecap.ReasonIncompatibleVersion,
		},
	}}
	registry, err := NewRegistry(RegistryOptions{
		NodeID: "node-laptop", Platform: "darwin/arm64",
		RevisionKey: []byte("01234567890123456789012345678901"), Prober: prober,
		Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	request := corecap.RecheckRequest{
		ProtocolVersion: 1, Mode: corecap.RefreshPlanning, MaxAgeSeconds: 60,
		Adapters: []string{"profile.codex.native"},
	}
	var inventory corecap.NodeInventory
	for range 4 {
		request.RequestID = uuid.NewString()
		inventory, err = registry.Refresh(context.Background(), request)
		if err != nil {
			t.Fatal(err)
		}
		now = now.Add(61 * time.Second)
	}
	if got := prober.callCount("predicate.codex.native-contract.v1"); got != 4 {
		t.Fatalf("stable mismatch probes = %d, want one per expired TTL", got)
	}
	profile := findProfile(t, inventory, "codex:gpt-5.5")
	if profile.State != corecap.OfferSetupRequired || profile.Reason != corecap.ReasonIncompatibleVersion {
		t.Fatalf("stable mismatch profile = %#v", profile)
	}
}

func TestProbeFreshnessStartsWhenProbeSettles(t *testing.T) {
	now := time.Unix(1000, 0).UTC()
	var mu sync.Mutex
	nativeCalls := 0
	prober := probeFunc(func(_ context.Context, request ProbeRequest) ProbeObservation {
		if request.PredicateID == "predicate.codex.native-contract.v1" {
			mu.Lock()
			nativeCalls++
			now = now.Add(50 * time.Second)
			mu.Unlock()
		}
		return satisfied("stable=" + request.PredicateID)
	})
	registry, err := NewRegistry(RegistryOptions{
		NodeID: "node-laptop", Platform: "darwin/arm64",
		RevisionKey: []byte("01234567890123456789012345678901"), Prober: prober,
		Now: func() time.Time {
			mu.Lock()
			defer mu.Unlock()
			return now
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	request := corecap.RecheckRequest{
		ProtocolVersion: 1, RequestID: uuid.NewString(), Mode: corecap.RefreshPlanning,
		MaxAgeSeconds: 60, Adapters: []string{"profile.codex.native"},
	}
	if _, err := registry.Refresh(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	now = now.Add(20 * time.Second)
	mu.Unlock()
	request.RequestID = uuid.NewString()
	if _, err := registry.Refresh(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if nativeCalls != 1 {
		t.Fatalf("successful slow probe was immediately treated as expired; calls=%d", nativeCalls)
	}
}

func TestValidateRecheckRequestIsStrict(t *testing.T) {
	valid := corecap.RecheckRequest{
		ProtocolVersion: 1, RequestID: uuid.NewString(), Mode: corecap.RefreshPlanning,
		MaxAgeSeconds: 60, Adapters: []string{"profile.codex.native"},
	}
	if err := ValidateRecheckRequest(valid); err != nil {
		t.Fatal(err)
	}
	invalid := valid
	invalid.MaxAgeSeconds = 0
	if err := ValidateRecheckRequest(invalid); err == nil {
		t.Fatal("planning max age 0 was accepted")
	}
	invalid = valid
	invalid.Adapters = []string{"raw.shell"}
	if err := ValidateRecheckRequest(invalid); err == nil {
		t.Fatal("unknown adapter was accepted")
	}
}

func findProfile(t *testing.T, inventory corecap.NodeInventory, id string) corecap.ProfileOffer {
	t.Helper()
	for _, profile := range inventory.Profiles {
		if profile.ID == id {
			return profile
		}
	}
	t.Fatalf("profile %q not found", id)
	return corecap.ProfileOffer{}
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}
