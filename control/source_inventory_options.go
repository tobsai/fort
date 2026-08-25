package control

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	corecap "github.com/tobsai/fort/core/capability"
	"github.com/tobsai/fort/core/conversation"
	coreruntime "github.com/tobsai/fort/core/runtime"
	"github.com/tobsai/fort/ui"
)

const (
	ExecutionAdapterNotApproved     = "execution_adapter_not_approved"
	unknownInventoryBindingValue    = "unknown"
	unapprovedInventoryBindingValue = "unapproved"
)

// SourceInventoryRegistration identifies one account-owned Execution Source.
// Inventory is optional because persisted discovery remains visible even when
// this process has no approved live transport with which to recheck it.
type SourceInventoryRegistration struct {
	AccountID         string
	ExecutionSourceID string
	Inventory         coreruntime.AgentSourceInventory
}

// SourceInventoryAgentOptionSource projects persisted source discovery into
// disabled v1 Agent options. Its deliberately invalid AgentBinding carries
// presentation identity only and cannot authorize or validate for execution.
type SourceInventoryAgentOptionSource struct {
	repository    coreruntime.AgentSourceInventoryRepository
	registrations []SourceInventoryRegistration
	now           func() time.Time
}

func NewSourceInventoryAgentOptionSource(repository coreruntime.AgentSourceInventoryRepository, registrations []SourceInventoryRegistration) (*SourceInventoryAgentOptionSource, error) {
	if repository == nil {
		return nil, errors.New("Agent Source inventory repository is required")
	}
	seen := make(map[string]struct{}, len(registrations))
	registered := append([]SourceInventoryRegistration(nil), registrations...)
	for _, registration := range registered {
		if strings.TrimSpace(registration.AccountID) == "" || registration.AccountID != strings.TrimSpace(registration.AccountID) ||
			strings.TrimSpace(registration.ExecutionSourceID) == "" || registration.ExecutionSourceID != strings.TrimSpace(registration.ExecutionSourceID) {
			return nil, errors.New("Agent Source inventory registration is invalid")
		}
		key := registration.AccountID + "\x00" + registration.ExecutionSourceID
		if _, duplicate := seen[key]; duplicate {
			return nil, errors.New("Agent Source inventory registration is duplicated")
		}
		seen[key] = struct{}{}
	}
	return &SourceInventoryAgentOptionSource{repository: repository, registrations: registered, now: time.Now}, nil
}

func (source *SourceInventoryAgentOptionSource) AgentOptions(ctx context.Context) ([]ui.AgentOption, error) {
	options := make([]ui.AgentOption, 0)
	seenIDs := make(map[string]struct{})
	for _, registration := range source.registrations {
		latest, err := source.repository.LatestAgentSourceInventory(ctx, registration.AccountID, registration.ExecutionSourceID)
		if err != nil {
			return nil, err
		}
		if !latest.HasSnapshot {
			continue
		}
		projected, err := projectSourceInventoryOptions(registration, latest)
		if err != nil {
			return nil, err
		}
		for _, option := range projected {
			if _, duplicate := seenIDs[option.ID]; duplicate {
				return nil, fmt.Errorf("Agent option id %q is duplicated", option.ID)
			}
			seenIDs[option.ID] = struct{}{}
		}
		options = append(options, projected...)
	}
	sort.Slice(options, func(left, right int) bool { return options[left].ID < options[right].ID })
	return options, nil
}

func (source *SourceInventoryAgentOptionSource) RecheckAgentOptions(ctx context.Context) ([]ui.AgentOption, error) {
	for _, registration := range source.registrations {
		if registration.Inventory == nil {
			continue
		}
		snapshot, err := registration.Inventory.Inventory(ctx, coreruntime.AgentSourceInventoryRequest{
			ExecutionSourceID: registration.ExecutionSourceID,
		})
		if err != nil {
			if contextErr := ctx.Err(); contextErr != nil {
				return nil, contextErr
			}
			failure := coreruntime.AgentSourceInventoryFailure{
				AccountID: registration.AccountID, ExecutionSourceID: registration.ExecutionSourceID,
				ObservedAt: source.now().UTC(), Reason: coreruntime.AgentSourceInventoryUnavailable,
			}
			if appendErr := source.repository.AppendAgentSourceInventoryFailure(ctx, failure); appendErr != nil {
				return nil, appendErr
			}
			continue
		}
		if contextErr := ctx.Err(); contextErr != nil {
			return nil, contextErr
		}
		if err := validateSourceInventorySnapshot(registration, snapshot); err != nil {
			return nil, err
		}
		if err := source.repository.AppendAgentSourceInventorySuccess(ctx, snapshot); err != nil {
			return nil, err
		}
	}
	return source.AgentOptions(ctx)
}

func projectSourceInventoryOptions(registration SourceInventoryRegistration, latest coreruntime.LatestAgentSourceInventory) ([]ui.AgentOption, error) {
	snapshot := latest.Snapshot
	if err := validateSourceInventorySnapshot(registration, snapshot); err != nil {
		return nil, err
	}
	if latest.Freshness != coreruntime.AgentSourceInventoryCurrent && latest.Freshness != coreruntime.AgentSourceInventoryStale {
		return nil, errors.New("persisted Agent Source inventory freshness is invalid")
	}
	if !sameInventoryInstant(latest.SourceLastSeenAt, snapshot.ExecutionSource.LastSeenAt) {
		return nil, errors.New("persisted Agent Source inventory last-seen evidence is invalid")
	}
	reason := ExecutionAdapterNotApproved
	switch latest.Freshness {
	case coreruntime.AgentSourceInventoryCurrent:
		if !sameInventoryInstant(latest.LastAttemptAt, snapshot.ObservedAt) || latest.FailureReason != "" || !latest.FailureObservedAt.IsZero() {
			return nil, errors.New("persisted current Agent Source inventory evidence is invalid")
		}
	case coreruntime.AgentSourceInventoryStale:
		if latest.FailureReason != coreruntime.AgentSourceInventoryUnavailable || latest.FailureObservedAt.IsZero() ||
			!sameInventoryInstant(latest.LastAttemptAt, latest.FailureObservedAt) {
			return nil, errors.New("persisted stale Agent Source inventory evidence is invalid")
		}
		reason = string(coreruntime.AgentSourceInventoryUnavailable)
	}
	options := make([]ui.AgentOption, 0, len(snapshot.Agents))
	for _, item := range snapshot.Agents {
		agent := item.SourceAgent
		options = append(options, ui.AgentOption{
			ID: agent.ID, State: string(corecap.OfferUnavailable), Reason: reason, DisplayName: agent.DisplayName,
			Binding: discoveryOnlyAgentBinding(snapshot.ExecutionSource, agent),
		})
	}
	return options, nil
}

func sameInventoryInstant(left, right time.Time) bool {
	if left.IsZero() || right.IsZero() {
		return left.IsZero() && right.IsZero()
	}
	return left.Equal(right)
}

func validateSourceInventorySnapshot(registration SourceInventoryRegistration, snapshot coreruntime.AgentSourceInventorySnapshot) error {
	if err := snapshot.Validate(); err != nil {
		return fmt.Errorf("Agent Source inventory is invalid: %w", err)
	}
	if snapshot.ExecutionSource.AccountID != registration.AccountID || snapshot.ExecutionSource.ID != registration.ExecutionSourceID {
		return errors.New("Agent Source inventory does not match registration")
	}
	for _, item := range snapshot.Agents {
		if item.Readiness.Ready {
			return errors.New("source inventory execution readiness is not approved")
		}
	}
	return nil
}

func discoveryOnlyAgentBinding(source conversation.ExecutionSource, agent conversation.SourceAgent) conversation.AgentBinding {
	return conversation.AgentBinding{
		Seat: conversation.AgentSeatIdentity{
			ID: agent.ID, Profile: source.Framework + ":" + agent.OpaqueSourceAgentID, Agent: source.Framework,
			Model: unknownInventoryBindingValue, Machine: source.DisplayName,
		},
		Authority: conversation.AgentAuthoritySnapshot{
			RequestedModel: unknownInventoryBindingValue, ResolvedModel: unknownInventoryBindingValue,
			Authority: unapprovedInventoryBindingValue, PolicyID: unapprovedInventoryBindingValue,
			PolicyRevision: unapprovedInventoryBindingValue, AdapterID: unapprovedInventoryBindingValue,
			AdapterRevision: unapprovedInventoryBindingValue, RuntimeContract: unapprovedInventoryBindingValue,
			SessionMode: unapprovedInventoryBindingValue, MemoryMode: unapprovedInventoryBindingValue,
			ExecutionPolicy: map[string]string{},
		},
	}
}

var _ AgentOptionSource = (*SourceInventoryAgentOptionSource)(nil)

type compositeAgentOptionSource struct {
	sources []AgentOptionSource
}

// NewCompositeAgentOptionSource combines independently bounded option sources.
// Ready rows sort first; all other rows sort by their opaque source-qualified
// IDs so discovery order cannot change the picker contract.
func NewCompositeAgentOptionSource(sources ...AgentOptionSource) AgentOptionSource {
	return &compositeAgentOptionSource{sources: append([]AgentOptionSource(nil), sources...)}
}

func (source *compositeAgentOptionSource) AgentOptions(ctx context.Context) ([]ui.AgentOption, error) {
	return source.collect(ctx, false)
}

func (source *compositeAgentOptionSource) RecheckAgentOptions(ctx context.Context) ([]ui.AgentOption, error) {
	return source.collect(ctx, true)
}

func (source *compositeAgentOptionSource) collect(ctx context.Context, recheck bool) ([]ui.AgentOption, error) {
	options := make([]ui.AgentOption, 0)
	seen := make(map[string]struct{})
	for _, itemSource := range source.sources {
		if itemSource == nil {
			continue
		}
		var (
			items []ui.AgentOption
			err   error
		)
		if recheck {
			items, err = itemSource.RecheckAgentOptions(ctx)
		} else {
			items, err = itemSource.AgentOptions(ctx)
		}
		if err != nil {
			return nil, err
		}
		for _, option := range items {
			if _, duplicate := seen[option.ID]; duplicate {
				return nil, fmt.Errorf("Agent option id %q is duplicated", option.ID)
			}
			seen[option.ID] = struct{}{}
			options = append(options, option)
		}
	}
	sort.Slice(options, func(left, right int) bool {
		leftReady := options[left].State == PrimaryAgentReady
		rightReady := options[right].State == PrimaryAgentReady
		if leftReady != rightReady {
			return leftReady
		}
		return options[left].ID < options[right].ID
	})
	return options, nil
}

var _ AgentOptionSource = (*compositeAgentOptionSource)(nil)
