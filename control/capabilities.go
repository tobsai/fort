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
				inventory = unknownNode(machine.Name, discoveryReason(err))
			} else if inventory.NodeID != machine.Name {
				inventory = unknownNode(machine.Name, corecap.ReasonCommandContractChanged)
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
		localInventory = unknownNode(c.localName, corecap.ReasonProbeFailed)
	}
	if localInventory.NodeID != c.localName {
		localInventory = unknownNode(c.localName, corecap.ReasonCommandContractChanged)
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

// RefreshMachine refreshes only the already-selected target. Dispatch uses
// this path so an unrelated slow or unavailable peer cannot delay provider
// startup or influence deterministic placement.
func (c *CapabilityCoordinator) RefreshMachine(ctx context.Context, target string, mode corecap.RefreshMode, adapters []string) (corecap.MachineInventory, error) {
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
		return bindInventory(inventory, c.localName, true, 0, receiptTime), nil
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
			return bindInventory(inventory, machine.Name, false, peerRank, receiptTime), nil
		}
	}
	return corecap.MachineInventory{}, fmt.Errorf("control capability: selected machine %q is not in the registry", target)
}

func (c *CapabilityCoordinator) Current() (corecap.Snapshot, uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.current, c.generation
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
		Bindings: nonnilBindings(inventory.Bindings),
	}
}

func unknownNode(nodeID string, reason corecap.Reason) corecap.NodeInventory {
	return corecap.NodeInventory{
		NodeID: nodeID, State: corecap.MachineUnknown, Reason: reason,
		Profiles: []corecap.ProfileOffer{}, Offers: []corecap.LogicalOffer{},
		Bindings: []corecap.ExecutionBindingOffer{},
	}
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
