package control

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	corecap "github.com/tobsai/fort/core/capability"
	"github.com/tobsai/fort/core/machines"
	execcap "github.com/tobsai/fort/exec/capability"
)

type LocalCapabilityRegistry interface {
	Current() corecap.NodeInventory
	Refresh(context.Context, corecap.RecheckRequest) (corecap.NodeInventory, error)
}

type PeerCapabilityClient interface {
	Refresh(context.Context, string, string, corecap.RecheckRequest) (corecap.NodeInventory, error)
}

type CapabilityCoordinatorOptions struct {
	Live      *machines.Live
	LocalName string
	Local     LocalCapabilityRegistry
	Peers     PeerCapabilityClient
	Now       func() time.Time
}

// CapabilityCoordinator refreshes local and peer inventories concurrently,
// binds every peer payload to registry-owned identity/rank, and publishes one
// normalized snapshot generation atomically.
type CapabilityCoordinator struct {
	live      *machines.Live
	localName string
	local     LocalCapabilityRegistry
	peers     PeerCapabilityClient
	now       func() time.Time

	refreshMu  sync.Mutex
	mu         sync.Mutex
	generation uint64
	current    corecap.Snapshot
}

func NewCapabilityCoordinator(options CapabilityCoordinatorOptions) (*CapabilityCoordinator, error) {
	if options.Live == nil || options.Local == nil || options.Peers == nil || options.LocalName == "" {
		return nil, fmt.Errorf("control capability: live registry, local identity, local registry, and peer client are required")
	}
	if registry := options.Live.Load(); registry != nil && len(registry.Machines) > 16 {
		return nil, fmt.Errorf("control capability: registry exceeds 16 machines")
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	return &CapabilityCoordinator{
		live: options.Live, localName: options.LocalName, local: options.Local,
		peers: options.Peers, now: options.Now,
	}, nil
}

func (c *CapabilityCoordinator) Refresh(ctx context.Context, mode corecap.RefreshMode, adapters []string) (corecap.Snapshot, uint64, error) {
	c.refreshMu.Lock()
	defer c.refreshMu.Unlock()
	registry := c.live.Load()
	if registry != nil && len(registry.Machines) > 16 {
		return corecap.Snapshot{}, 0, fmt.Errorf("control capability: registry exceeds 16 machines")
	}
	request := corecap.RecheckRequest{
		ProtocolVersion: corecap.ProtocolVersion, RequestID: uuid.NewString(),
		Mode: mode, Adapters: append([]string(nil), adapters...),
	}
	if mode == corecap.RefreshPlanning {
		request.MaxAgeSeconds = 60
	}
	previous := c.previousMachines()
	type peerResult struct {
		index     int
		name      string
		inventory corecap.NodeInventory
	}
	var peerMachines []machines.Machine
	if registry != nil {
		for _, machine := range registry.Machines {
			if machine.Name != c.localName {
				peerMachines = append(peerMachines, machine)
			}
		}
	}
	results := make(chan peerResult, len(peerMachines))
	var wg sync.WaitGroup
	for index, machine := range peerMachines {
		wg.Add(1)
		go func(index int, machine machines.Machine) {
			defer wg.Done()
			inventory, err := c.peers.Refresh(ctx, machine.URL, machine.Name, request)
			if err != nil {
				inventory = unknownNodeWithPrevious(machine.Name, discoveryReason(err), previous[machine.Name])
			} else if inventory.NodeID != machine.Name {
				inventory = unknownNodeWithPrevious(machine.Name, corecap.ReasonCommandContractChanged, previous[machine.Name])
			}
			results <- peerResult{
				index: index, name: machine.Name, inventory: inventory,
			}
		}(index, machine)
	}

	localInventory, localErr := c.local.Refresh(ctx, request)
	wg.Wait()
	close(results)
	if err := ctx.Err(); err != nil {
		return corecap.Snapshot{}, 0, err
	}
	if localErr != nil {
		localInventory = unknownNodeWithPrevious(c.localName, corecap.ReasonProbeFailed, previous[c.localName])
	}
	if localInventory.NodeID != c.localName {
		localInventory = unknownNodeWithPrevious(c.localName, corecap.ReasonCommandContractChanged, previous[c.localName])
	}
	receiptTime := c.now().UTC()
	localRow := bindInventory(localInventory, c.localName, true, 0, receiptTime)

	rows := make([]corecap.MachineInventory, 1+len(peerMachines))
	rows[0] = localRow
	for result := range results {
		rows[result.index+1] = bindInventory(result.inventory, result.name, false, result.index+1, receiptTime)
	}
	snapshot := corecap.Snapshot{
		CatalogVersion: corecap.CatalogVersion, ProfileMappingVersion: corecap.ProfileMappingVersion,
		ObservedAt: receiptTime, LocalMachine: c.localName, Machines: rows,
	}
	normalized, err := corecap.NormalizeSnapshot(snapshot)
	if err != nil {
		return corecap.Snapshot{}, 0, err
	}
	revision, err := corecap.SnapshotRevision(normalized)
	if err != nil {
		return corecap.Snapshot{}, 0, err
	}
	normalized.Revision = revision

	c.mu.Lock()
	c.generation++
	generation := c.generation
	c.current = normalized
	c.mu.Unlock()
	return normalized, generation, nil
}

// RecheckConversationSeats explicitly reprobes every execution-profile adapter
// and publishes one fresh snapshot generation. It is the bounded user action
// behind the shared-conversation seat picker; it performs no setup, placement,
// or runtime dispatch.
func (c *CapabilityCoordinator) RecheckConversationSeats(ctx context.Context) error {
	catalog := corecap.CatalogV2()
	seen := map[string]bool{}
	adapters := make([]string, 0, len(catalog.Profiles))
	for _, profile := range catalog.Profiles {
		if profile.Agent == "codex-subscription" {
			continue
		}
		if seen[profile.Adapter] {
			continue
		}
		seen[profile.Adapter] = true
		adapters = append(adapters, profile.Adapter)
	}
	_, _, err := c.Refresh(ctx, corecap.RefreshUserRecheck, adapters)
	return err
}

// RefreshMachine refreshes only the already-selected target. Dispatch uses
// this path so an unrelated slow or unavailable peer cannot delay provider
// startup or influence deterministic placement.
func (c *CapabilityCoordinator) RefreshMachine(ctx context.Context, target string, mode corecap.RefreshMode, adapters []string) (corecap.MachineInventory, error) {
	c.refreshMu.Lock()
	defer c.refreshMu.Unlock()
	registry := c.live.Load()
	if registry != nil && len(registry.Machines) > 16 {
		return corecap.MachineInventory{}, fmt.Errorf("control capability: registry exceeds 16 machines")
	}
	request := corecap.RecheckRequest{
		ProtocolVersion: corecap.ProtocolVersion, RequestID: uuid.NewString(),
		Mode: mode, Adapters: append([]string(nil), adapters...),
	}
	if mode == corecap.RefreshPlanning {
		request.MaxAgeSeconds = 60
	}
	receiptTime := c.now().UTC()

	if target == "" || strings.EqualFold(target, c.localName) {
		inventory, err := c.local.Refresh(ctx, request)
		if err != nil {
			inventory = unknownNode(c.localName, corecap.ReasonProbeFailed)
		}
		if inventory.NodeID != c.localName {
			inventory = unknownNode(c.localName, corecap.ReasonCommandContractChanged)
		}
		row := bindInventory(inventory, c.localName, true, 0, receiptTime)
		return row, c.publishRefreshedMachine(row)
	}

	peerRank := 1
	if registry != nil {
		for _, machine := range registry.Machines {
			if strings.EqualFold(machine.Name, c.localName) {
				continue
			}
			if !strings.EqualFold(machine.Name, target) {
				peerRank++
				continue
			}
			inventory, err := c.peers.Refresh(ctx, machine.URL, machine.Name, request)
			if err != nil {
				inventory = unknownNode(machine.Name, discoveryReason(err))
			} else if inventory.NodeID != machine.Name {
				inventory = unknownNode(machine.Name, corecap.ReasonCommandContractChanged)
			}
			row := bindInventory(inventory, machine.Name, false, peerRank, receiptTime)
			return row, c.publishRefreshedMachine(row)
		}
	}
	return corecap.MachineInventory{}, fmt.Errorf("control capability: selected machine %q is not in the registry", target)
}

// publishRefreshedMachine replaces only the selected machine in the current
// snapshot. RefreshMachine holds refreshMu, so a targeted dispatch preflight
// cannot race a full refresh and leave UI readiness behind the decision that
// admitted or rejected the turn.
func (c *CapabilityCoordinator) publishRefreshedMachine(row corecap.MachineInventory) error {
	c.mu.Lock()
	if c.generation == 0 {
		c.mu.Unlock()
		return nil
	}
	snapshot := c.current
	c.mu.Unlock()

	machines := append([]corecap.MachineInventory(nil), snapshot.Machines...)
	index := -1
	for candidate := range machines {
		if strings.EqualFold(machines[candidate].Name, row.Name) {
			index = candidate
			break
		}
	}
	if index < 0 {
		return nil
	}
	machines[index] = row
	snapshot.Machines = machines
	snapshot.Revision = ""
	if row.ObservedAt.After(snapshot.ObservedAt) {
		snapshot.ObservedAt = row.ObservedAt
	}
	normalized, err := corecap.NormalizeSnapshot(snapshot)
	if err != nil {
		return err
	}
	revision, err := corecap.SnapshotRevision(normalized)
	if err != nil {
		return err
	}
	normalized.Revision = revision

	c.mu.Lock()
	c.generation++
	c.current = normalized
	c.mu.Unlock()
	return nil
}

func (c *CapabilityCoordinator) Current() (corecap.Snapshot, uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.current, c.generation
}

func (c *CapabilityCoordinator) previousMachines() map[string]corecap.MachineInventory {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[string]corecap.MachineInventory, len(c.current.Machines))
	for _, machine := range c.current.Machines {
		out[machine.Name] = machine
	}
	return out
}

// Capabilities implements ui.CapabilityLister without exposing refresh or
// private probe controls to the presentation layer.
func (c *CapabilityCoordinator) Capabilities() (corecap.Snapshot, uint64) {
	return c.Current()
}

func bindInventory(inventory corecap.NodeInventory, name string, local bool, rank int, receiptTime time.Time) corecap.MachineInventory {
	return corecap.MachineInventory{
		Name: name, Local: local, RegistryRank: rank,
		Reachable:       inventory.Reason != corecap.ReasonUnavailable,
		ProtocolVersion: inventory.ProtocolVersion, CatalogVersion: inventory.CatalogVersion,
		ProfileMappingVersion: inventory.ProfileMappingVersion,
		State:                 inventory.State, Reason: inventory.Reason, ObservedAt: receiptTime,
		Profiles: nonnilProfiles(inventory.Profiles), Offers: nonnilOffers(inventory.Offers),
		Bindings: nonnilBindings(inventory.Bindings), TextOnlyOptions: validTextOnlyOptions(inventory.TextOnlyOptions, name),
	}
}

func unknownNode(nodeID string, reason corecap.Reason) corecap.NodeInventory {
	return corecap.NodeInventory{
		NodeID: nodeID, State: corecap.MachineUnknown, Reason: reason,
		Profiles: []corecap.ProfileOffer{}, Offers: []corecap.LogicalOffer{},
		Bindings: []corecap.ExecutionBindingOffer{}, TextOnlyOptions: []corecap.TextOnlyOptionOffer{},
	}
}

// unknownNodeWithPrevious preserves only profile identities that were already
// verified for this registry-owned machine, and only across a transient loss
// of reachability. Identity, protocol, and contract failures stay empty; a
// stale logical capability or composite binding is never carried forward.
func unknownNodeWithPrevious(nodeID string, reason corecap.Reason, previous corecap.MachineInventory) corecap.NodeInventory {
	if previous.Name == "" || reason != corecap.ReasonUnavailable {
		return unknownNode(nodeID, reason)
	}
	profiles := append([]corecap.ProfileOffer(nil), previous.Profiles...)
	for index := range profiles {
		profiles[index].State = corecap.OfferUnknown
		profiles[index].Reason = reason
		profiles[index].BindingRevision = ""
		profiles[index].Predicates = unknownPredicates(profiles[index].Predicates, reason)
	}
	return corecap.NodeInventory{
		ProtocolVersion: previous.ProtocolVersion, CatalogVersion: previous.CatalogVersion,
		ProfileMappingVersion: previous.ProfileMappingVersion, NodeID: nodeID,
		State: corecap.MachineUnknown, Reason: reason,
		Profiles: profiles, Offers: []corecap.LogicalOffer{}, Bindings: []corecap.ExecutionBindingOffer{},
		TextOnlyOptions: []corecap.TextOnlyOptionOffer{},
	}
}

func unknownPredicates(values []corecap.Predicate, reason corecap.Reason) []corecap.Predicate {
	out := append([]corecap.Predicate(nil), values...)
	for index := range out {
		out[index].DependsOn = append([]string{}, out[index].DependsOn...)
		out[index].RemedyEffectIDs = append([]string{}, out[index].RemedyEffectIDs...)
		if out[index].Resolution == corecap.ResolutionDerived {
			out[index].State = corecap.PredicateBlocked
			out[index].Reason = ""
			continue
		}
		out[index].State = corecap.PredicateUnsatisfied
		out[index].Reason = reason
	}
	return out
}

func discoveryReason(err error) corecap.Reason {
	var discovery *execcap.DiscoveryError
	if errors.As(err, &discovery) {
		return discovery.Reason
	}
	return corecap.ReasonUnavailable
}

func nonnilProfiles(values []corecap.ProfileOffer) []corecap.ProfileOffer {
	if values == nil {
		return []corecap.ProfileOffer{}
	}
	return values
}

func nonnilOffers(values []corecap.LogicalOffer) []corecap.LogicalOffer {
	if values == nil {
		return []corecap.LogicalOffer{}
	}
	return values
}

func nonnilBindings(values []corecap.ExecutionBindingOffer) []corecap.ExecutionBindingOffer {
	if values == nil {
		return []corecap.ExecutionBindingOffer{}
	}
	return values
}

func validTextOnlyOptions(values []corecap.TextOnlyOptionOffer, machine string) []corecap.TextOnlyOptionOffer {
	if values == nil || len(values) > 8 {
		return []corecap.TextOnlyOptionOffer{}
	}
	out := make([]corecap.TextOnlyOptionOffer, len(values))
	ids := make(map[string]bool, len(values))
	seats := make(map[string]bool, len(values))
	for index, offer := range values {
		normalized, id, err := corecap.NormalizeTextOnlyOptionOffer(offer, machine)
		if err != nil || ids[id] || seats[normalized.SeatID] {
			return []corecap.TextOnlyOptionOffer{}
		}
		ids[id] = true
		seats[normalized.SeatID] = true
		out[index] = normalized
	}
	return out
}
