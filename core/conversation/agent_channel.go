package conversation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type AgentChannelState string

const (
	AgentChannelOpen     AgentChannelState = "open"
	AgentChannelArchived AgentChannelState = "archived"

	AgentMemoryEphemeral = "ephemeral"
)

// AgentSeatIdentity is the immutable dispatch identity stored by an Agent
// Channel. Presentation and live readiness deliberately live elsewhere.
type AgentSeatIdentity struct {
	ID      string `json:"id"`
	Profile string `json:"profile"`
	Agent   string `json:"agent"`
	Model   string `json:"model"`
	Machine string `json:"machine"`
}

// AgentAuthoritySnapshot is the accepted, non-secret adapter contract for an
// Agent Channel. ExecutionPolicy keeps adapter-specific normalized fields
// exact without coupling the conversation domain to a concrete provider.
type AgentAuthoritySnapshot struct {
	RequestedModel  string            `json:"requested_model"`
	ResolvedModel   string            `json:"resolved_model"`
	Authority       string            `json:"authority"`
	PolicyID        string            `json:"policy_id"`
	PolicyRevision  string            `json:"policy_revision"`
	AdapterID       string            `json:"adapter_id"`
	AdapterRevision string            `json:"adapter_revision"`
	RuntimeContract string            `json:"runtime_contract"`
	SessionMode     string            `json:"session_mode"`
	MemoryMode      string            `json:"memory_mode"`
	ExecutionPolicy map[string]string `json:"execution_policy"`
}

type AgentBinding struct {
	Seat      AgentSeatIdentity      `json:"seat"`
	Authority AgentAuthoritySnapshot `json:"authority"`
}

type AgentChannel struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	State     AgentChannelState `json:"state"`
	OptionID  string            `json:"option_id"`
	Binding   AgentBinding      `json:"binding"`
	CreatedAt time.Time         `json:"created_at"`
}

type AgentConversationSummary struct {
	Conversation Conversation `json:"conversation"`
	Participant  Participant  `json:"participant"`
	Pinned       bool         `json:"pinned"`
	PinnedAt     time.Time    `json:"pinned_at,omitempty"`
}

type AgentChannelConversation struct {
	AgentChannelID string    `json:"agent_channel_id"`
	ConversationID string    `json:"conversation_id"`
	CreatedAt      time.Time `json:"created_at"`
}

type AgentConversationPin struct {
	ConversationID string    `json:"conversation_id"`
	PinnedAt       time.Time `json:"pinned_at"`
}

type AgentChannelDetail struct {
	Channel       AgentChannel               `json:"channel"`
	Conversations []AgentConversationSummary `json:"conversations"`
}

func (binding AgentBinding) Validate() error {
	required := []struct {
		name  string
		value string
	}{
		{"seat id", binding.Seat.ID}, {"profile", binding.Seat.Profile}, {"agent", binding.Seat.Agent},
		{"model", binding.Seat.Model}, {"machine", binding.Seat.Machine},
		{"requested model", binding.Authority.RequestedModel}, {"resolved model", binding.Authority.ResolvedModel},
		{"authority", binding.Authority.Authority}, {"policy id", binding.Authority.PolicyID},
		{"policy revision", binding.Authority.PolicyRevision}, {"adapter id", binding.Authority.AdapterID},
		{"adapter revision", binding.Authority.AdapterRevision}, {"runtime contract", binding.Authority.RuntimeContract},
		{"session mode", binding.Authority.SessionMode}, {"memory mode", binding.Authority.MemoryMode},
	}
	for _, field := range required {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("Agent Channel %s is required", field.name)
		}
	}
	if binding.Seat.Model != binding.Authority.RequestedModel {
		return fmt.Errorf("Agent Channel requested model must match its seat")
	}
	if len(binding.Authority.ExecutionPolicy) == 0 {
		return fmt.Errorf("Agent Channel execution policy is required")
	}
	for key, value := range binding.Authority.ExecutionPolicy {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			return fmt.Errorf("Agent Channel execution policy keys and values are required")
		}
	}
	return nil
}

func (channel AgentChannel) Validate() error {
	name := strings.TrimSpace(channel.Name)
	if name == "" || len([]byte(name)) > 120 {
		return fmt.Errorf("Agent Channel name must contain 1 to 120 UTF-8 bytes")
	}
	if channel.State != AgentChannelOpen && channel.State != AgentChannelArchived {
		return fmt.Errorf("Agent Channel state must be open or archived")
	}
	if strings.TrimSpace(channel.OptionID) == "" {
		return fmt.Errorf("Agent Channel option id is required")
	}
	if channel.CreatedAt.IsZero() {
		return fmt.Errorf("Agent Channel creation time is required")
	}
	if err := channel.Binding.Validate(); err != nil {
		return err
	}
	want, err := AgentChannelID(channel.Binding)
	if err != nil {
		return err
	}
	if channel.ID != want {
		return fmt.Errorf("Agent Channel id does not match its immutable binding")
	}
	return nil
}

// AgentChannelID derives one stable logical ID from the complete immutable
// binding. encoding/json sorts string-map keys, keeping the digest canonical.
func AgentChannelID(binding AgentBinding) (string, error) {
	if err := binding.Validate(); err != nil {
		return "", err
	}
	wire, err := json.Marshal(binding)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(append(append([]byte("agent-channel:v1\n"), wire...), '\n'))
	return "agent-channel:v1:" + hex.EncodeToString(digest[:]), nil
}
