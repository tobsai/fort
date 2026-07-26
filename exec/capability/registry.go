// Package capability owns execution-side capability discovery. It converts
// private bounded probe observations into the closed, secret-free core
// inventory contract.
package capability

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	corecap "github.com/tobsai/fort/core/capability"
)

const probeTimeout = 15 * time.Second

type ProbeRequest struct {
	AdapterID   string
	TargetID    string
	ProfileID   string
	BindingID   string
	PredicateID string
}

type ProbeObservation struct {
	State         corecap.PredicateState
	Reason        corecap.Reason
	StableBinding []string
}

type Prober interface {
	Probe(context.Context, ProbeRequest) ProbeObservation
}

type RegistryOptions struct {
	NodeID      string
	Platform    string
	RevisionKey []byte
	Prober      Prober
	Now         func() time.Time
}

type cachedProbe struct {
	observation ProbeObservation
	observedAt  time.Time
	failures    int
	nextAllowed time.Time
}

type refreshMemo struct {
	mu     sync.Mutex
	values map[string]*refreshMemoEntry
}

type refreshMemoEntry struct {
	done        chan struct{}
	observation ProbeObservation
}

type probeFlight struct {
	done        chan struct{}
	observation ProbeObservation
}

func (m *refreshMemo) observe(key string, run func() ProbeObservation) ProbeObservation {
	m.mu.Lock()
	if entry, ok := m.values[key]; ok {
		m.mu.Unlock()
		<-entry.done
		return entry.observation
	}
	entry := &refreshMemoEntry{done: make(chan struct{})}
	m.values[key] = entry
	m.mu.Unlock()

	entry.observation = run()
	close(entry.done)
	return entry.observation
}

type Registry struct {
	nodeID      string
	platform    string
	revisionKey []byte
	prober      Prober
	now         func() time.Time
	catalog     corecap.Catalog
	semaphore   chan struct{}

	mu      sync.Mutex
	cache   map[string]cachedProbe
	current corecap.NodeInventory

	flightMu     sync.Mutex
	flights      map[string]*probeFlight
	adapterMu    sync.Mutex
	adapterGates map[string]chan struct{}
}

func NewRegistry(options RegistryOptions) (*Registry, error) {
	if options.NodeID == "" {
		return nil, fmt.Errorf("exec capability: node ID is required")
	}
	if len(options.RevisionKey) != 32 {
		return nil, fmt.Errorf("exec capability: revision key must contain 32 bytes")
	}
	if options.Prober == nil {
		return nil, fmt.Errorf("exec capability: prober is required")
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	return &Registry{
		nodeID: options.NodeID, platform: options.Platform,
		revisionKey: append([]byte(nil), options.RevisionKey...),
		prober:      options.Prober, now: options.Now, catalog: corecap.CatalogV1(),
		semaphore: make(chan struct{}, 2), cache: map[string]cachedProbe{},
		flights: map[string]*probeFlight{}, adapterGates: map[string]chan struct{}{},
	}, nil
}

func RecheckAll(mode corecap.RefreshMode, requestID string) corecap.RecheckRequest {
	adapters := allAdapterIDs(corecap.CatalogV1())
	maxAge := 0
	if mode == corecap.RefreshPlanning {
		maxAge = 60
	}
	return corecap.RecheckRequest{
		ProtocolVersion: corecap.ProtocolVersion, RequestID: requestID,
		Mode: mode, MaxAgeSeconds: maxAge, Adapters: adapters,
	}
}

func allAdapterIDs(catalog corecap.Catalog) []string {
	seen := map[string]bool{}
	var out []string
	for _, profile := range catalog.Profiles {
		if !seen[profile.Adapter] {
			out = append(out, profile.Adapter)
			seen[profile.Adapter] = true
		}
	}
	for _, logical := range catalog.Capabilities {
		if !seen[logical.Adapter] {
			out = append(out, logical.Adapter)
			seen[logical.Adapter] = true
		}
	}
	for _, binding := range catalog.Bindings {
		if !seen[binding.ID] {
			out = append(out, binding.ID)
			seen[binding.ID] = true
		}
	}
	return out
}

func ValidateRecheckRequest(request corecap.RecheckRequest) error {
	if request.ProtocolVersion != corecap.ProtocolVersion {
		return fmt.Errorf("exec capability: unsupported protocol version")
	}
	parsed, err := uuid.Parse(request.RequestID)
	if err != nil || parsed.String() != request.RequestID {
		return fmt.Errorf("exec capability: request_id must be a canonical UUID")
	}
	switch request.Mode {
	case corecap.RefreshPlanning:
		if request.MaxAgeSeconds != 60 {
			return fmt.Errorf("exec capability: planning max_age_seconds must be 60")
		}
	case corecap.RefreshUserRecheck:
		if request.MaxAgeSeconds != 0 {
			return fmt.Errorf("exec capability: user_recheck max_age_seconds must be 0")
		}
	default:
		return fmt.Errorf("exec capability: unknown refresh mode")
	}
	if len(request.Adapters) < 1 || len(request.Adapters) > 32 {
		return fmt.Errorf("exec capability: adapters must contain 1 to 32 rows")
	}
	known := map[string]bool{}
	for _, adapter := range allAdapterIDs(corecap.CatalogV1()) {
		known[adapter] = true
	}
	seen := map[string]bool{}
	for _, adapter := range request.Adapters {
		if !known[adapter] || seen[adapter] {
			return fmt.Errorf("exec capability: unknown or duplicate adapter %q", adapter)
		}
		seen[adapter] = true
	}
	return nil
}

func (r *Registry) Refresh(ctx context.Context, request corecap.RecheckRequest) (corecap.NodeInventory, error) {
	if err := ValidateRecheckRequest(request); err != nil {
		return corecap.NodeInventory{}, err
	}
	selected := make(map[string]bool, len(request.Adapters))
	for _, adapter := range request.Adapters {
		selected[adapter] = true
	}
	memo := &refreshMemo{values: map[string]*refreshMemoEntry{}}

	profiles := make([]corecap.ProfileOffer, len(r.catalog.Profiles))
	logicals := make([]corecap.LogicalOffer, len(r.catalog.Capabilities))
	var wg sync.WaitGroup
	for i, definition := range r.catalog.Profiles {
		wg.Add(1)
		go func(index int, profile corecap.ProfileDefinition) {
			defer wg.Done()
			profiles[index] = r.buildProfile(ctx, request, selected, memo, profile)
		}(i, definition)
	}
	for i, definition := range r.catalog.Capabilities {
		wg.Add(1)
		go func(index int, logical corecap.CapabilityDefinition) {
			defer wg.Done()
			logicals[index] = r.buildLogical(ctx, request, selected, memo, logical)
		}(i, definition)
	}
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return corecap.NodeInventory{}, err
	}

	var bindings []corecap.ExecutionBindingOffer
	for _, bindingDefinition := range r.catalog.Bindings {
		for profileIndex, profileDefinition := range r.catalog.Profiles {
			if profileDefinition.Agent != bindingDefinition.Agent {
				continue
			}
			bindings = append(bindings, r.buildBinding(
				ctx, request, selected, memo, bindingDefinition, profileDefinition,
				profiles[profileIndex], logicals,
			))
		}
	}
	if bindings == nil {
		bindings = []corecap.ExecutionBindingOffer{}
	}
	state, reason := machineSummary(profiles, logicals, bindings)
	inventory := corecap.NodeInventory{
		ProtocolVersion: corecap.ProtocolVersion, CatalogVersion: corecap.CatalogVersion,
		ProfileMappingVersion: corecap.ProfileMappingVersion, NodeID: r.nodeID,
		ObservedAt: r.now().UTC(), State: state, Reason: reason,
		Profiles: profiles, Offers: logicals, Bindings: bindings,
	}
	r.mu.Lock()
	r.current = inventory
	r.mu.Unlock()
	return inventory, nil
}

func (r *Registry) Current() corecap.NodeInventory {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.current
}

func (r *Registry) buildProfile(ctx context.Context, refresh corecap.RecheckRequest, selected map[string]bool, memo *refreshMemo, definition corecap.ProfileDefinition) corecap.ProfileOffer {
	templates, _ := r.catalog.ProfilePredicateTemplates(definition.ID)
	predicates, material := r.resolvePredicates(
		ctx, refresh, selected[definition.Adapter], memo,
		ProbeRequest{AdapterID: definition.Adapter, TargetID: definition.ID, ProfileID: definition.ID},
		templates, nil,
	)
	state, reason := offerSummary(predicates, nil)
	revision := ""
	if state == corecap.OfferReady {
		revision, _ = corecap.OpaqueRevision(r.revisionKey, "fort.profile-binding.v1", struct {
			CatalogVersion        int                       `json:"catalog_version"`
			ProfileMappingVersion int                       `json:"profile_mapping_version"`
			Profile               corecap.ProfileDefinition `json:"profile"`
			StableBinding         map[string][]string       `json:"stable_binding"`
		}{corecap.CatalogVersion, corecap.ProfileMappingVersion, definition, material})
	}
	return corecap.ProfileOffer{
		ID: definition.ID, Agent: definition.Agent, Adapter: definition.Adapter,
		State: state, Reason: reason, BindingRevision: revision, Predicates: predicates,
	}
}

func (r *Registry) buildLogical(ctx context.Context, refresh corecap.RecheckRequest, selected map[string]bool, memo *refreshMemo, definition corecap.CapabilityDefinition) corecap.LogicalOffer {
	templates, _ := r.catalog.LogicalPredicateTemplates(definition.ID)
	predicates, material := r.resolvePredicates(
		ctx, refresh, selected[definition.Adapter], memo,
		ProbeRequest{AdapterID: definition.Adapter, TargetID: definition.ID},
		templates, nil,
	)
	state, reason := offerSummary(predicates, nil)
	revision := ""
	if state == corecap.OfferReady {
		revision, _ = corecap.OpaqueRevision(r.revisionKey, "fort.logical-binding.v1", struct {
			CatalogVersion int                 `json:"catalog_version"`
			ID             string              `json:"id"`
			Adapter        string              `json:"adapter"`
			StableBinding  map[string][]string `json:"stable_binding"`
		}{corecap.CatalogVersion, definition.ID, definition.Adapter, material})
	}
	available := []string{}
	for _, binding := range r.catalog.Bindings {
		for _, capabilityID := range binding.CapabilityIDs {
			if capabilityID == definition.ID {
				available = append(available, binding.ID)
			}
		}
	}
	return corecap.LogicalOffer{
		ID: definition.ID, Adapter: definition.Adapter, State: state, Reason: reason,
		BindingRevision: revision, AvailableThrough: available, Predicates: predicates,
	}
}

func (r *Registry) buildBinding(
	ctx context.Context,
	refresh corecap.RecheckRequest,
	selected map[string]bool,
	memo *refreshMemo,
	definition corecap.BindingDefinition,
	profileDefinition corecap.ProfileDefinition,
	profile corecap.ProfileOffer,
	logicals []corecap.LogicalOffer,
) corecap.ExecutionBindingOffer {
	templates, _ := r.catalog.BindingPredicateTemplates(profile.ID, definition.ID, definition.CapabilityIDs)
	seed := map[string]bool{}
	for _, predicate := range profile.Predicates {
		seed[predicate.ID] = predicate.State == corecap.PredicateSatisfied
	}
	var leafReasons []corecap.Reason
	if profile.State != corecap.OfferReady {
		leafReasons = append(leafReasons, profile.Reason)
	}
	logicalRevisions := map[string]string{}
	for _, capabilityID := range definition.CapabilityIDs {
		for _, logical := range logicals {
			if logical.ID != capabilityID {
				continue
			}
			for _, predicate := range logical.Predicates {
				seed[predicate.ID] = predicate.State == corecap.PredicateSatisfied
			}
			if logical.State != corecap.OfferReady {
				leafReasons = append(leafReasons, logical.Reason)
			} else {
				logicalRevisions[logical.ID] = logical.BindingRevision
			}
		}
	}
	predicates, material := r.resolvePredicates(
		ctx, refresh, selected[definition.ID], memo,
		ProbeRequest{
			AdapterID: definition.ID, TargetID: definition.ID,
			ProfileID: profileDefinition.ID, BindingID: definition.ID,
		},
		templates, seed,
	)
	state, reason := offerSummary(predicates, leafReasons)
	revision := ""
	if state == corecap.OfferReady {
		revision, _ = corecap.OpaqueRevision(r.revisionKey, "fort.execution-binding.v1", struct {
			CatalogVersion        int                 `json:"catalog_version"`
			ProfileMappingVersion int                 `json:"profile_mapping_version"`
			BindingID             string              `json:"binding_id"`
			ProfileID             string              `json:"profile_id"`
			ProfileRevision       string              `json:"profile_revision"`
			LogicalRevisions      map[string]string   `json:"logical_revisions"`
			StableBinding         map[string][]string `json:"stable_binding"`
		}{
			corecap.CatalogVersion, corecap.ProfileMappingVersion,
			definition.ID, profile.ID, profile.BindingRevision, logicalRevisions, material,
		})
	}
	return corecap.ExecutionBindingOffer{
		ID: definition.ID, Profile: profile.ID,
		Capabilities: append([]string{}, definition.CapabilityIDs...),
		State:        state, Reason: reason, BindingRevision: revision, Predicates: predicates,
	}
}

func (r *Registry) resolvePredicates(
	ctx context.Context,
	refresh corecap.RecheckRequest,
	selected bool,
	memo *refreshMemo,
	base ProbeRequest,
	templates []corecap.PredicateTemplate,
	seed map[string]bool,
) ([]corecap.Predicate, map[string][]string) {
	satisfied := map[string]bool{}
	for key, value := range seed {
		satisfied[key] = value
	}
	predicates := make([]corecap.Predicate, len(templates))
	material := map[string][]string{}
	for i, template := range templates {
		predicate := corecap.Predicate{
			ID: template.ID, Resolution: template.Resolution,
			DependsOn:       append([]string{}, template.DependsOn...),
			RemedyEffectIDs: append([]string{}, template.RemedyEffectIDs...),
		}
		if !dependenciesReady(template.DependsOn, satisfied) {
			predicate.State = corecap.PredicateBlocked
			if template.Resolution == corecap.ResolutionProbe {
				predicate.Reason = blockedReason(template.ID)
			}
			predicates[i] = predicate
			continue
		}
		if template.Resolution == corecap.ResolutionDerived {
			predicate.State = corecap.PredicateSatisfied
			satisfied[template.ID] = true
			predicates[i] = predicate
			continue
		}
		request := base
		request.PredicateID = template.ID
		observation := r.observe(ctx, refresh, selected, memo, request)
		predicate.State, predicate.Reason = observation.State, observation.Reason
		if predicate.State == corecap.PredicateSatisfied {
			predicate.Reason = ""
			satisfied[template.ID] = true
			material[template.ID] = append([]string(nil), observation.StableBinding...)
		}
		predicates[i] = predicate
	}
	return predicates, material
}

func dependenciesReady(dependencies []string, satisfied map[string]bool) bool {
	for _, dependency := range dependencies {
		if !satisfied[dependency] {
			return false
		}
	}
	return true
}

func blockedReason(predicateID string) corecap.Reason {
	switch {
	case contains(predicateID, ".model."):
		return corecap.ReasonModelUnavailable
	case contains(predicateID, "binding."):
		return corecap.ReasonIncompatibleVersion
	default:
		return corecap.ReasonAuthRequired
	}
}

func contains(value, fragment string) bool {
	for i := 0; i+len(fragment) <= len(value); i++ {
		if value[i:i+len(fragment)] == fragment {
			return true
		}
	}
	return false
}

func (r *Registry) observe(ctx context.Context, refresh corecap.RecheckRequest, selected bool, memo *refreshMemo, request ProbeRequest) ProbeObservation {
	key := probeKey(request)
	return memo.observe(key, func() ProbeObservation {
		return r.observeOnce(ctx, refresh, selected, key, request)
	})
}

func (r *Registry) observeOnce(ctx context.Context, refresh corecap.RecheckRequest, selected bool, key string, request ProbeRequest) ProbeObservation {
	now := r.now()
	r.mu.Lock()
	cached, hasCached := r.cache[key]
	r.mu.Unlock()
	if !selected {
		if hasCached {
			return cached.observation
		}
		return ProbeObservation{State: corecap.PredicateUnsatisfied, Reason: corecap.ReasonStale}
	}
	if refresh.Mode == corecap.RefreshPlanning && hasCached {
		if now.Sub(cached.observedAt) <= time.Duration(refresh.MaxAgeSeconds)*time.Second {
			return cached.observation
		}
		if now.Before(cached.nextAllowed) {
			return ProbeObservation{State: corecap.PredicateUnsatisfied, Reason: corecap.ReasonStale}
		}
	}
	if r.platform != "darwin/arm64" {
		return ProbeObservation{State: corecap.PredicateUnsatisfied, Reason: corecap.ReasonUnsupportedPlatform}
	}
	return r.singleFlight(ctx, key, func() ProbeObservation {
		return r.runProbe(ctx, key, request, cached)
	})
}

func (r *Registry) runProbe(ctx context.Context, key string, request ProbeRequest, cached cachedProbe) ProbeObservation {
	releaseAdapter, acquired := r.acquireAdapter(ctx, request.AdapterID)
	if !acquired {
		return unsatisfied(corecap.ReasonProbeTimedOut)
	}
	defer releaseAdapter()
	select {
	case r.semaphore <- struct{}{}:
	case <-ctx.Done():
		return unsatisfied(corecap.ReasonProbeTimedOut)
	}
	defer func() { <-r.semaphore }()
	probeCtx, cancel := context.WithTimeout(ctx, probeTimeout)
	observation := r.prober.Probe(probeCtx, request)
	cancel()
	observation = normalizeObservation(observation)
	if ctx.Err() != nil {
		return unsatisfied(corecap.ReasonProbeTimedOut)
	}

	observedAt := r.now()
	r.mu.Lock()
	entry := cachedProbe{observation: observation, observedAt: observedAt}
	if shouldBackoff(observation) {
		entry.failures = cached.failures + 1
		backoff := 30 * time.Second
		for i := 1; i < entry.failures && backoff < 15*time.Minute; i++ {
			backoff *= 2
		}
		if backoff > 15*time.Minute {
			backoff = 15 * time.Minute
		}
		entry.nextAllowed = observedAt.Add(backoff)
	}
	r.cache[key] = entry
	r.mu.Unlock()
	return observation
}

func shouldBackoff(observation ProbeObservation) bool {
	if observation.State == corecap.PredicateSatisfied {
		return false
	}
	switch observation.Reason {
	case corecap.ReasonProbeFailed, corecap.ReasonProbeTimedOut, corecap.ReasonOutputLimitExceeded:
		return true
	default:
		return false
	}
}

func (r *Registry) acquireAdapter(ctx context.Context, adapter string) (func(), bool) {
	r.adapterMu.Lock()
	gate := r.adapterGates[adapter]
	if gate == nil {
		gate = make(chan struct{}, 1)
		r.adapterGates[adapter] = gate
	}
	r.adapterMu.Unlock()
	select {
	case gate <- struct{}{}:
		return func() { <-gate }, true
	case <-ctx.Done():
		return nil, false
	}
}

func (r *Registry) singleFlight(ctx context.Context, key string, run func() ProbeObservation) ProbeObservation {
	r.flightMu.Lock()
	if flight, ok := r.flights[key]; ok {
		r.flightMu.Unlock()
		select {
		case <-flight.done:
			return flight.observation
		case <-ctx.Done():
			return unsatisfied(corecap.ReasonProbeTimedOut)
		}
	}
	flight := &probeFlight{done: make(chan struct{})}
	r.flights[key] = flight
	r.flightMu.Unlock()

	observation := run()
	r.flightMu.Lock()
	flight.observation = observation
	close(flight.done)
	delete(r.flights, key)
	r.flightMu.Unlock()
	return observation
}

func normalizeObservation(observation ProbeObservation) ProbeObservation {
	if observation.State == corecap.PredicateSatisfied {
		observation.Reason = ""
		return observation
	}
	if observation.State != corecap.PredicateUnsatisfied ||
		corecap.FirstReason(observation.Reason) == "" {
		return ProbeObservation{State: corecap.PredicateUnsatisfied, Reason: corecap.ReasonProbeFailed}
	}
	observation.StableBinding = nil
	return observation
}

func probeKey(request ProbeRequest) string {
	// Native/auth predicates are intentionally shared across all exact profiles;
	// model and binding predicates already carry their profile/binding identity.
	return request.AdapterID + "\x00" + request.PredicateID + "\x00" + request.BindingID
}

func offerSummary(predicates []corecap.Predicate, leafReasons []corecap.Reason) (corecap.OfferState, corecap.Reason) {
	reasons := append([]corecap.Reason(nil), leafReasons...)
	for _, predicate := range predicates {
		if predicate.State == corecap.PredicateUnsatisfied {
			reasons = append(reasons, predicate.Reason)
		}
	}
	reason := corecap.FirstReason(reasons...)
	if reason == "" {
		return corecap.OfferReady, ""
	}
	switch reason {
	case corecap.ReasonUnsupportedPlatform:
		return corecap.OfferUnavailable, reason
	case corecap.ReasonStale, corecap.ReasonProbeFailed, corecap.ReasonProbeTimedOut:
		return corecap.OfferUnknown, reason
	default:
		return corecap.OfferSetupRequired, reason
	}
}

func machineSummary(profiles []corecap.ProfileOffer, logicals []corecap.LogicalOffer, bindings []corecap.ExecutionBindingOffer) (corecap.MachineState, corecap.Reason) {
	var reasons []corecap.Reason
	allReady := len(profiles)+len(logicals)+len(bindings) > 0
	for _, profile := range profiles {
		if profile.State != corecap.OfferReady {
			allReady = false
			reasons = append(reasons, profile.Reason)
		}
	}
	for _, logical := range logicals {
		if logical.State != corecap.OfferReady {
			allReady = false
			reasons = append(reasons, logical.Reason)
		}
	}
	for _, binding := range bindings {
		if binding.State != corecap.OfferReady {
			allReady = false
			reasons = append(reasons, binding.Reason)
		}
	}
	if allReady {
		return corecap.MachineReady, ""
	}
	return corecap.MachinePartial, corecap.FirstReason(reasons...)
}

// Keep deterministic ordering helpers local to the execution registry.
func sortedKeys(values map[string][]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
