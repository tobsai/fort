package runtime

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/tobsai/fort/core/conversation"
)

// AgentSourceInventory discovers source-qualified framework identities and
// their readiness evidence. It intentionally has no execution, enrollment, or
// binding method; those remain separate explicit Fort commands.
type AgentSourceInventory interface {
	Inventory(context.Context, AgentSourceInventoryRequest) (AgentSourceInventorySnapshot, error)
}

type AgentSourceInventoryRequest struct {
	ExecutionSourceID string `json:"execution_source_id"`
}

type AgentSourceInventorySnapshot struct {
	ExecutionSource conversation.ExecutionSource `json:"execution_source"`
	Agents          []SourceAgentInventory       `json:"agents"`
	ObservedAt      time.Time                    `json:"observed_at"`
}

type SourceAgentInventory struct {
	SourceAgent  conversation.SourceAgent `json:"source_agent"`
	Capabilities []string                 `json:"capabilities"`
	Readiness    SourceAgentReadiness     `json:"readiness"`
}

type SourceAgentReadiness struct {
	Ready            bool     `json:"ready"`
	ContractID       string   `json:"contract_id"`
	ContractRevision string   `json:"contract_revision"`
	Evidence         []string `json:"evidence"`
}

func (snapshot AgentSourceInventorySnapshot) Validate() error {
	if err := snapshot.ExecutionSource.Validate(); err != nil {
		return err
	}
	if snapshot.ObservedAt.IsZero() {
		return fmt.Errorf("Agent Source inventory observation time is required")
	}
	if snapshot.Agents == nil {
		return fmt.Errorf("Agent Source inventory Agents must be an allocated list")
	}
	identities := make(map[conversation.SourceAgentIdentity]struct{}, len(snapshot.Agents))
	for _, item := range snapshot.Agents {
		if err := item.SourceAgent.Validate(); err != nil {
			return err
		}
		if item.SourceAgent.ExecutionSourceID != snapshot.ExecutionSource.ID {
			return fmt.Errorf("Source Agent belongs to another Execution Source")
		}
		identity, err := item.SourceAgent.Identity()
		if err != nil {
			return err
		}
		if _, exists := identities[identity]; exists {
			return fmt.Errorf("Source Agent identity is duplicated")
		}
		identities[identity] = struct{}{}
		if err := validateInventoryStrings("capability", item.Capabilities, item.Readiness.Ready); err != nil {
			return err
		}
		if strings.TrimSpace(item.Readiness.ContractID) == "" || strings.TrimSpace(item.Readiness.ContractRevision) == "" {
			return fmt.Errorf("Source Agent readiness contract and revision are required")
		}
		if err := validateInventoryStrings("readiness evidence", item.Readiness.Evidence, true); err != nil {
			return err
		}
	}
	return nil
}

func validateInventoryStrings(subject string, values []string, required bool) error {
	if values == nil {
		return fmt.Errorf("Source Agent %s must be an allocated list", subject)
	}
	if required && len(values) == 0 {
		return fmt.Errorf("Source Agent %s is required", subject)
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) == "" || value != strings.TrimSpace(value) {
			return fmt.Errorf("Source Agent %s is not normalized", subject)
		}
		if _, exists := seen[value]; exists {
			return fmt.Errorf("Source Agent %s is duplicated", subject)
		}
		seen[value] = struct{}{}
	}
	return nil
}
