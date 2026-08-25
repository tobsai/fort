package runtime

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type AgentSourceInventoryFailureReason string

const AgentSourceInventoryUnavailable AgentSourceInventoryFailureReason = "source_inventory_unavailable"

type AgentSourceInventoryFreshness string

const (
	AgentSourceInventoryCurrent AgentSourceInventoryFreshness = "current"
	AgentSourceInventoryStale   AgentSourceInventoryFreshness = "stale"
)

// AgentSourceInventoryFailure records a closed, non-sensitive explanation for
// a failed inventory attempt. It contains no provider response or secret.
type AgentSourceInventoryFailure struct {
	AccountID         string                            `json:"account_id"`
	ExecutionSourceID string                            `json:"execution_source_id"`
	ObservedAt        time.Time                         `json:"observed_at"`
	Reason            AgentSourceInventoryFailureReason `json:"reason"`
}

func (failure AgentSourceInventoryFailure) Validate() error {
	if strings.TrimSpace(failure.AccountID) == "" || failure.AccountID != strings.TrimSpace(failure.AccountID) {
		return fmt.Errorf("Agent Source inventory failure account id is required and normalized")
	}
	if strings.TrimSpace(failure.ExecutionSourceID) == "" || failure.ExecutionSourceID != strings.TrimSpace(failure.ExecutionSourceID) {
		return fmt.Errorf("Agent Source inventory failure Execution Source id is required and normalized")
	}
	if failure.ObservedAt.IsZero() {
		return fmt.Errorf("Agent Source inventory failure observation time is required")
	}
	if failure.Reason != AgentSourceInventoryUnavailable {
		return fmt.Errorf("Agent Source inventory failure reason is not recognized")
	}
	return nil
}

// LatestAgentSourceInventory is a projection over append-only observations.
// A failure makes an earlier successful snapshot stale without erasing it.
type LatestAgentSourceInventory struct {
	HasSnapshot       bool                              `json:"has_snapshot"`
	Snapshot          AgentSourceInventorySnapshot      `json:"snapshot,omitempty"`
	Freshness         AgentSourceInventoryFreshness     `json:"freshness,omitempty"`
	LastAttemptAt     time.Time                         `json:"last_attempt_at,omitempty"`
	SourceLastSeenAt  time.Time                         `json:"source_last_seen_at,omitempty"`
	FailureReason     AgentSourceInventoryFailureReason `json:"failure_reason,omitempty"`
	FailureObservedAt time.Time                         `json:"failure_observed_at,omitempty"`
}

// AgentSourceInventoryRepository is the provider-neutral persistence seam for
// inventory observations. Append order, not observer clocks, defines latest.
type AgentSourceInventoryRepository interface {
	AppendAgentSourceInventorySuccess(context.Context, AgentSourceInventorySnapshot) error
	AppendAgentSourceInventoryFailure(context.Context, AgentSourceInventoryFailure) error
	LatestAgentSourceInventory(context.Context, string, string) (LatestAgentSourceInventory, error)
}
