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

type cancelableBindingProber struct {
	entered chan struct{}
	once    sync.Once
}

func (p *cancelableBindingProber) Probe(ctx context.Context, request ProbeRequest) ProbeObservation {
	if request.BindingID == "" {
		return satisfied("stable=" + request.PredicateID)
	}
	p.once.Do(func() { close(p.entered) })
	<-ctx.Done()
	return unsatisfied(corecap.ReasonProbeTimedOut)
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
		if request.PredicateID == "predicate.codex.model.codex:configured-default.v1" {
			observation.ResolvedModel = "gpt-5.6-sol"
		}
		if request.PredicateID == "predicate.codex-subscription.closed-contract.v1" {
			offer := validRegistryTextOnlyOption()
			observation.TextOnlyOption = &offer
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
	if len(inventory.Profiles) != 12 || len(inventory.Offers) != 3 || len(inventory.Bindings) != 22 || len(inventory.TextOnlyOptions) != 1 {
		t.Fatalf("inventory sizes = profiles:%d offers:%d bindings:%d", len(inventory.Profiles), len(inventory.Offers), len(inventory.Bindings))
	}
	if inventory.State != corecap.MachinePartial || inventory.Reason != corecap.ReasonModelUnavailable {
		t.Fatalf("machine state = %s/%s", inventory.State, inventory.Reason)
	}
	if prober.maxActive > 2 {
		t.Fatalf("max concurrent probes = %d, want <=2", prober.maxActive)
	}
	for _, adapter := range allAdapterIDs(corecap.CatalogV2()) {
		if got := prober.maximumForAdapter(adapter); got > 1 {
			t.Fatalf("adapter %s ran %d probes concurrently, want <=1", adapter, got)
		}
	}
	for _, profile := range inventory.Profiles {
		unsupportedDynamic := profile.ID == "claude:configured-default" ||
			profile.ID == "hermes:configured-default" || profile.ID == "openclaw:main"
		if unsupportedDynamic {
			if profile.State != corecap.OfferSetupRequired || profile.Reason != corecap.ReasonModelUnavailable ||
				profile.ResolvedModel != "" || profile.BindingRevision != "" {
				t.Fatalf("unsupported dynamic profile = %#v", profile)
			}
			continue
		}
		if profile.State != corecap.OfferReady || !strings.HasPrefix(profile.BindingRevision, "opaque:") {
			t.Fatalf("profile = %#v", profile)
		}
		if profile.ID == "codex:configured-default" && profile.ResolvedModel != "gpt-5.6-sol" {
			t.Fatalf("Codex dynamic profile = %#v", profile)
		}
	}
	encoded := mustJSON(t, inventory)
	if strings.Contains(encoded, "stable:") || strings.Contains(encoded, "0123456789") {
		t.Fatal("private probe material or revision key leaked into inventory")
	}
}

func TestRegistryPublishesTextOnlyOptionOnlyFromTypedClosedProbe(t *testing.T) {
	offer := validRegistryTextOnlyOption()
	prober := &fakeProber{observations: map[string]ProbeObservation{
		"predicate.codex-subscription.closed-contract.v1": {
			State: corecap.PredicateSatisfied, StableBinding: []string{"closed=true"}, TextOnlyOption: &offer,
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
	if len(inventory.TextOnlyOptions) != 1 {
		t.Fatalf("text-only options = %#v", inventory.TextOnlyOptions)
	}
	got := inventory.TextOnlyOptions[0]
	if got.MachineID != "node-laptop" || got.SeatID != corecap.TextOnlySeatID(got.ProfileID, got.MachineID, got.RequestedModel) {
		t.Fatalf("normalized option identity = %#v", got)
	}

	bad := offer
	bad.AccountPlan = "unknown-private-plan"
	prober.mu.Lock()
	prober.observations["predicate.codex-subscription.closed-contract.v1"] = ProbeObservation{
		State: corecap.PredicateSatisfied, TextOnlyOption: &bad,
	}
	prober.mu.Unlock()
	second, err := registry.Refresh(context.Background(), RecheckAll(corecap.RefreshUserRecheck, uuid.NewString()))
	if err != nil {
		t.Fatal(err)
	}
	if second.TextOnlyOptions == nil || len(second.TextOnlyOptions) != 0 {
		t.Fatalf("invalid option escaped = %#v", second.TextOnlyOptions)
	}
	profile := findProfile(t, second, "codex-subscription:gpt-5.6-sol")
	if profile.State == corecap.OfferReady || profile.Reason != corecap.ReasonCommandContractChanged {
		t.Fatalf("invalid option retained ready profile = %#v", profile)
	}
}

func validRegistryTextOnlyOption() corecap.TextOnlyOptionOffer {
	return corecap.TextOnlyOptionOffer{
		OfferVersion: 1, AgentKey: "codex-subscription",
		ProfileID: "codex-subscription:gpt-5.6-sol", RequestedModel: "gpt-5.6-sol", ResolvedModel: "unknown",
		AccountType: "chatgpt", AccountPlan: "pro",
		PolicyID: "codex-subscription-chat-v1", PolicyRevision: strings.Repeat("a", 64),
		RuntimeContract: "codex_subscription_exec_v1", ReasoningEffort: "medium", ReasoningContext: "current_turn",
		RequestTimeoutMillis: 120000, DeveloperInstructionRevision: strings.Repeat("b", 64),
		AdapterID: "model.chat.text-only.codex-subscription", AdapterRevision: strings.Repeat("c", 64),
		CodexVersion: "codex-cli 0.147.0-alpha.6.5", CodexExecutableRevision: strings.Repeat("d", 64),
		CodexSchemaRevision: strings.Repeat("e", 64), ThreadMode: "ephemeral", SandboxMode: "readOnly",
		ApprovalPolicy: "never", WorkdirMode: "empty_per_target", DynamicToolsMode: "none", MCPMode: "none",
		CommandPolicy: "deny_and_fail", FileReadPolicy: "deny_and_fail", IsolationRevision: strings.Repeat("f", 64),
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

func TestRegistryPublishesOnlyTypedDynamicResolvedModel(t *testing.T) {
	modelPredicate := "predicate.codex.model.codex:configured-default.v1"
	prober := &fakeProber{observations: map[string]ProbeObservation{
		modelPredicate: {
			State: corecap.PredicateSatisfied, StableBinding: []string{"model=PRIVATE-NOT-A-CONTRACT"},
			ResolvedModel: "gpt-5.6-sol",
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
	dynamic := findProfile(t, inventory, "codex:configured-default")
	if dynamic.State != corecap.OfferReady || dynamic.ResolvedModel != "gpt-5.6-sol" || dynamic.BindingRevision == "" {
		t.Fatalf("dynamic profile = %#v", dynamic)
	}
	explicit := findProfile(t, inventory, "codex:gpt-5.6-sol")
	if explicit.ResolvedModel != "" {
		t.Fatalf("explicit profile published resolved_model = %#v", explicit)
	}
	encoded := mustJSON(t, dynamic)
	if !strings.Contains(encoded, `"resolved_model":"gpt-5.6-sol"`) || strings.Contains(encoded, "PRIVATE-NOT-A-CONTRACT") {
		t.Fatalf("public dynamic profile = %s", encoded)
	}
}

func TestRegistryDoesNotParseDynamicModelFromStableBinding(t *testing.T) {
	modelPredicate := "predicate.codex.model.codex:configured-default.v1"
	prober := &fakeProber{observations: map[string]ProbeObservation{
		modelPredicate: {
			State:         corecap.PredicateSatisfied,
			StableBinding: []string{"model=gpt-5.6-sol"},
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
	profile := findProfile(t, inventory, "codex:configured-default")
	if profile.State != corecap.OfferSetupRequired || profile.Reason != corecap.ReasonModelUnavailable ||
		profile.ResolvedModel != "" || profile.BindingRevision != "" {
		t.Fatalf("unresolved dynamic profile = %#v", profile)
	}
}

func TestRegistryKeepsUnsupportedDynamicProfilesUnready(t *testing.T) {
	registry, err := NewRegistry(RegistryOptions{
		NodeID: "node-laptop", Platform: "darwin/arm64",
		RevisionKey: []byte("01234567890123456789012345678901"), Prober: &fakeProber{},
	})
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := registry.Refresh(context.Background(), RecheckAll(corecap.RefreshUserRecheck, uuid.NewString()))
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"claude:configured-default", "hermes:configured-default", "openclaw:main"} {
		profile := findProfile(t, inventory, id)
		if profile.State != corecap.OfferSetupRequired || profile.Reason != corecap.ReasonModelUnavailable ||
			profile.ResolvedModel != "" || profile.BindingRevision != "" {
			t.Errorf("unsupported dynamic profile %s = %#v", id, profile)
		}
	}
}

func TestRegistryResolvedModelChangesProfileBindingRevision(t *testing.T) {
	modelPredicate := "predicate.codex.model.codex:configured-default.v1"
	prober := &fakeProber{observations: map[string]ProbeObservation{
		modelPredicate: {State: corecap.PredicateSatisfied, ResolvedModel: "gpt-5.6-sol"},
	}}
	registry, err := NewRegistry(RegistryOptions{
		NodeID: "node-laptop", Platform: "darwin/arm64",
		RevisionKey: []byte("01234567890123456789012345678901"), Prober: prober,
	})
	if err != nil {
		t.Fatal(err)
	}
	first, err := registry.Refresh(context.Background(), RecheckAll(corecap.RefreshUserRecheck, uuid.NewString()))
	if err != nil {
		t.Fatal(err)
	}
	firstRevision := findProfile(t, first, "codex:configured-default").BindingRevision
	prober.mu.Lock()
	prober.observations[modelPredicate] = ProbeObservation{State: corecap.PredicateSatisfied, ResolvedModel: "gpt-5.6-terra"}
	prober.mu.Unlock()
	second, err := registry.Refresh(context.Background(), RecheckAll(corecap.RefreshUserRecheck, uuid.NewString()))
	if err != nil {
		t.Fatal(err)
	}
	secondProfile := findProfile(t, second, "codex:configured-default")
	if secondProfile.ResolvedModel != "gpt-5.6-terra" || secondProfile.BindingRevision == firstRevision {
		t.Fatalf("second profile = %#v, first revision = %q", secondProfile, firstRevision)
	}
}

func TestNormalizeObservationClearsResolvedModelOnFailure(t *testing.T) {
	got := normalizeObservation(ProbeObservation{
		State: corecap.PredicateUnsatisfied, Reason: corecap.ReasonModelUnavailable,
		StableBinding: []string{"private"}, ResolvedModel: "gpt-5.6-sol",
	})
	if got.ResolvedModel != "" || got.StableBinding != nil {
		t.Fatalf("failed observation retained private facts: %#v", got)
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
		ProtocolVersion: corecap.ProtocolVersion, RequestID: uuid.NewString(), Mode: corecap.RefreshPlanning,
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
		ProtocolVersion: corecap.ProtocolVersion, RequestID: uuid.NewString(), Mode: corecap.RefreshUserRecheck,
		MaxAgeSeconds: 0, Adapters: adapter,
	}
	if _, err := registry.Refresh(context.Background(), user); err != nil {
		t.Fatal(err)
	}
	if got := prober.callCount(predicate); got <= first {
		t.Fatalf("user Recheck did not bypass cache: %d -> %d", first, got)
	}
}

func TestUserRecheckInvalidatesEverySelectedAdapterCacheKeyBeforeProbe(t *testing.T) {
	registry, err := NewRegistry(RegistryOptions{
		NodeID: "node-laptop", Platform: "darwin/arm64",
		RevisionKey: []byte("01234567890123456789012345678901"),
		Prober:      &fakeProber{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Refresh(context.Background(), RecheckAll(corecap.RefreshUserRecheck, uuid.NewString())); err != nil {
		t.Fatal(err)
	}

	blocker := &blockingPredicateProber{
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
	registry.prober = blocker
	done := make(chan error, 1)
	go func() {
		_, err := registry.Refresh(context.Background(), corecap.RecheckRequest{
			ProtocolVersion: corecap.ProtocolVersion,
			RequestID:       uuid.NewString(),
			Mode:            corecap.RefreshUserRecheck,
			MaxAgeSeconds:   0,
			Adapters:        []string{"profile.codex.native"},
		})
		done <- err
	}()
	select {
	case <-blocker.entered:
	case <-time.After(time.Second):
		t.Fatal("selected adapter probe did not start")
	}

	registry.mu.Lock()
	selectedKeys := 0
	otherKeys := 0
	for key := range registry.cache {
		if strings.HasPrefix(key, "profile.codex.native\x00") {
			selectedKeys++
		} else {
			otherKeys++
		}
	}
	registry.mu.Unlock()
	if selectedKeys != 0 {
		t.Fatalf("user Recheck retained %d selected-adapter cache keys while its probe was running", selectedKeys)
	}
	if otherKeys == 0 {
		t.Fatal("user Recheck cleared unrelated adapter cache keys")
	}
	close(blocker.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestUserRecheckInvalidationPreservesFailureBackoffHistory(t *testing.T) {
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
		ProtocolVersion: corecap.ProtocolVersion, Mode: corecap.RefreshUserRecheck, MaxAgeSeconds: 0,
		Adapters: []string{"profile.codex.native"},
	}
	for range 3 {
		request.RequestID = uuid.NewString()
		if _, err := registry.Refresh(context.Background(), request); err != nil {
			t.Fatal(err)
		}
		now = now.Add(time.Second)
	}
	before := prober.callCount("predicate.codex.native-contract.v1")
	now = now.Add(61 * time.Second)
	request.Mode = corecap.RefreshPlanning
	request.MaxAgeSeconds = 60
	request.RequestID = uuid.NewString()
	inventory, err := registry.Refresh(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if got := prober.callCount("predicate.codex.native-contract.v1"); got != before {
		t.Fatalf("user Recheck reset failure history: probes %d -> %d", before, got)
	}
	profile := findProfile(t, inventory, "codex:gpt-5.5")
	if profile.State != corecap.OfferUnknown || profile.Reason != corecap.ReasonStale {
		t.Fatalf("backoff profile = %#v, want unknown/stale", profile)
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
		ProtocolVersion: corecap.ProtocolVersion, RequestID: uuid.NewString(), Mode: corecap.RefreshUserRecheck,
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
			ProtocolVersion: corecap.ProtocolVersion, RequestID: uuid.NewString(), Mode: corecap.RefreshUserRecheck,
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
		ProtocolVersion: corecap.ProtocolVersion, RequestID: uuid.NewString(), Mode: corecap.RefreshUserRecheck,
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
		ProtocolVersion: corecap.ProtocolVersion, RequestID: uuid.NewString(), Mode: corecap.RefreshUserRecheck,
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

func TestCanceledBindingRefreshDoesNotPublishOrDiscardInvalidatedCache(t *testing.T) {
	registry, err := NewRegistry(RegistryOptions{
		NodeID: "node-laptop", Platform: "darwin/arm64",
		RevisionKey: []byte("01234567890123456789012345678901"), Prober: &fakeProber{},
	})
	if err != nil {
		t.Fatal(err)
	}
	before, err := registry.Refresh(context.Background(), RecheckAll(corecap.RefreshUserRecheck, uuid.NewString()))
	if err != nil {
		t.Fatal(err)
	}
	prober := &cancelableBindingProber{entered: make(chan struct{})}
	registry.prober = prober
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := registry.Refresh(ctx, RecheckAll(corecap.RefreshUserRecheck, uuid.NewString()))
		done <- err
	}()
	select {
	case <-prober.entered:
	case <-time.After(time.Second):
		t.Fatal("binding probe did not start")
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled binding refresh error = %v, want context canceled", err)
	}
	after := registry.Current()
	if before.ObservedAt != after.ObservedAt || before.State != after.State || before.Reason != after.Reason {
		t.Fatalf("canceled binding refresh replaced current inventory: before=%#v after=%#v", before, after)
	}

	probeAfterCancel := &fakeProber{}
	registry.prober = probeAfterCancel
	planning := RecheckAll(corecap.RefreshPlanning, uuid.NewString())
	if _, err := registry.Refresh(context.Background(), planning); err != nil {
		t.Fatal(err)
	}
	probeAfterCancel.mu.Lock()
	calls := len(probeAfterCancel.calls)
	probeAfterCancel.mu.Unlock()
	if calls != 0 {
		t.Fatalf("planning refresh reprobed %d predicates after canceled cache invalidation", calls)
	}
}

func TestOverlappingCanceledRechecksRestoreNewestRollbackBaseline(t *testing.T) {
	registry, err := NewRegistry(RegistryOptions{
		NodeID: "node-laptop", Platform: "darwin/arm64",
		RevisionKey: []byte("01234567890123456789012345678901"), Prober: &fakeProber{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Refresh(context.Background(), RecheckAll(corecap.RefreshUserRecheck, uuid.NewString())); err != nil {
		t.Fatal(err)
	}
	prober := &cancelablePredicateProber{entered: make(chan struct{})}
	prober.setBlocked()
	registry.prober = prober

	firstCtx, cancelFirst := context.WithCancel(context.Background())
	firstDone := make(chan error, 1)
	go func() {
		_, err := registry.Refresh(firstCtx, corecap.RecheckRequest{
			ProtocolVersion: corecap.ProtocolVersion, RequestID: uuid.NewString(), Mode: corecap.RefreshUserRecheck,
			Adapters: []string{"profile.codex.native"},
		})
		firstDone <- err
	}()
	select {
	case <-prober.entered:
	case <-time.After(time.Second):
		t.Fatal("first codex Recheck did not start")
	}

	secondCtx, cancelSecond := context.WithCancel(context.Background())
	cancelSecond()
	_, err = registry.Refresh(secondCtx, corecap.RecheckRequest{
		ProtocolVersion: corecap.ProtocolVersion, RequestID: uuid.NewString(), Mode: corecap.RefreshUserRecheck,
		Adapters: []string{"profile.codex.native"},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("newer Recheck error = %v, want context canceled", err)
	}
	registry.mu.Lock()
	codexKeys := 0
	for key := range registry.cache {
		if strings.HasPrefix(key, "profile.codex.native\x00") {
			codexKeys++
		}
	}
	registry.mu.Unlock()
	if codexKeys != 0 {
		t.Fatalf("newer canceled Recheck restored %d stale keys while an older Recheck remained active", codexKeys)
	}

	cancelFirst()
	if err := <-firstDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("first Recheck error = %v, want context canceled", err)
	}
	registry.mu.Lock()
	codexKeys = 0
	for key := range registry.cache {
		if strings.HasPrefix(key, "profile.codex.native\x00") {
			codexKeys++
		}
	}
	registry.mu.Unlock()
	if codexKeys == 0 {
		t.Fatal("overlapping canceled Rechecks lost the original cache rollback baseline")
	}

	probeAfterCancel := &fakeProber{}
	registry.prober = probeAfterCancel
	planning := corecap.RecheckRequest{
		ProtocolVersion: corecap.ProtocolVersion, RequestID: uuid.NewString(), Mode: corecap.RefreshPlanning,
		MaxAgeSeconds: 60, Adapters: []string{"profile.codex.native"},
	}
	if _, err := registry.Refresh(context.Background(), planning); err != nil {
		t.Fatal(err)
	}
	probeAfterCancel.mu.Lock()
	calls := len(probeAfterCancel.calls)
	probeAfterCancel.mu.Unlock()
	if calls != 0 {
		t.Fatalf("planning refresh reprobed %d predicates after overlapping canceled Rechecks", calls)
	}
}

func TestSuccessfulOverlapClosesInvalidationGroupBeforeNextRecheck(t *testing.T) {
	registry, err := NewRegistry(RegistryOptions{
		NodeID: "node-laptop", Platform: "darwin/arm64",
		RevisionKey: []byte("01234567890123456789012345678901"), Prober: &fakeProber{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Refresh(context.Background(), RecheckAll(corecap.RefreshUserRecheck, uuid.NewString())); err != nil {
		t.Fatal(err)
	}
	selected := map[string]bool{"profile.codex.native": true}
	first := registry.invalidateAdapters(selected)
	var refreshedKey string
	var refreshed cachedProbe
	for key, cached := range first.entries {
		if strings.HasPrefix(key, "profile.codex.native\x00") {
			refreshedKey, refreshed = key, cached
			break
		}
	}
	if refreshedKey == "" {
		t.Fatal("seed refresh produced no codex cache key")
	}
	refreshed.failures++
	refreshed.observedAt = refreshed.observedAt.Add(time.Second)
	registry.mu.Lock()
	registry.cache[refreshedKey] = refreshed
	registry.mu.Unlock()
	second := registry.invalidateAdapters(selected)
	registry.restoreInvalidated(second)
	registry.mu.Lock()
	registry.commitInvalidatedLocked(first)
	restored, retainedAfterSuccess := registry.cache[refreshedKey]
	registry.mu.Unlock()
	if !retainedAfterSuccess || restored.failures != refreshed.failures || restored.observedAt != refreshed.observedAt {
		t.Fatalf("successful overlap lost its latest observation: got %#v, want %#v", restored, refreshed)
	}

	third := registry.invalidateAdapters(selected)
	registry.mu.Lock()
	_, retained := registry.cache[refreshedKey]
	registry.mu.Unlock()
	if retained {
		t.Fatal("new Recheck retained cache from an already-successful overlap group")
	}
	if third.groups["profile.codex.native"] == first.groups["profile.codex.native"] {
		t.Fatal("new Recheck joined an already-successful invalidation group")
	}
}

func TestOverlappingRecheckInvalidatesCacheRepopulatedByActivePeer(t *testing.T) {
	registry, err := NewRegistry(RegistryOptions{
		NodeID: "node-laptop", Platform: "darwin/arm64",
		RevisionKey: []byte("01234567890123456789012345678901"), Prober: &fakeProber{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Refresh(context.Background(), RecheckAll(corecap.RefreshUserRecheck, uuid.NewString())); err != nil {
		t.Fatal(err)
	}
	selected := map[string]bool{"profile.codex.native": true}
	first := registry.invalidateAdapters(selected)
	var refreshedKey string
	var refreshed cachedProbe
	for key, cached := range first.entries {
		if strings.HasPrefix(key, "profile.codex.native\x00") {
			refreshedKey, refreshed = key, cached
			break
		}
	}
	if refreshedKey == "" {
		t.Fatal("seed refresh produced no codex cache key")
	}
	original := refreshed
	refreshed.failures++
	refreshed.observedAt = refreshed.observedAt.Add(time.Second)
	registry.mu.Lock()
	registry.cache[refreshedKey] = refreshed
	registry.mu.Unlock()

	second := registry.invalidateAdapters(selected)
	registry.mu.Lock()
	_, retained := registry.cache[refreshedKey]
	registry.mu.Unlock()
	if retained {
		t.Fatal("overlapping Recheck retained cache repopulated by its active peer")
	}
	if second.groups["profile.codex.native"] != first.groups["profile.codex.native"] {
		t.Fatal("overlapping Recheck did not join the active invalidation group")
	}
	third := registry.invalidateAdapters(selected)
	if got := third.entries[refreshedKey]; got.failures != refreshed.failures || got.observedAt != refreshed.observedAt {
		t.Fatalf("later overlap inherited %#v, want latest removed observation %#v", got, refreshed)
	}
	registry.restoreInvalidated(third)
	registry.restoreInvalidated(second)
	registry.restoreInvalidated(first)
	registry.mu.Lock()
	rolledBack := registry.cache[refreshedKey]
	registry.mu.Unlock()
	if rolledBack.failures != original.failures || rolledBack.observedAt != original.observedAt {
		t.Fatalf("all-canceled rollback = %#v, want original observation %#v", rolledBack, original)
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
		ProtocolVersion: corecap.ProtocolVersion, RequestID: uuid.NewString(), Mode: corecap.RefreshPlanning,
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
		ProtocolVersion: corecap.ProtocolVersion, RequestID: uuid.NewString(), Mode: corecap.RefreshPlanning,
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
		ProtocolVersion: corecap.ProtocolVersion, Mode: corecap.RefreshPlanning, MaxAgeSeconds: 60,
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
		ProtocolVersion: corecap.ProtocolVersion, RequestID: uuid.NewString(), Mode: corecap.RefreshPlanning,
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
		ProtocolVersion: corecap.ProtocolVersion, RequestID: uuid.NewString(), Mode: corecap.RefreshPlanning,
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
	invalid = valid
	invalid.ProtocolVersion = 1
	if err := ValidateRecheckRequest(invalid); err == nil {
		t.Fatal("protocol version 1 was accepted after the version-2 contract")
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
