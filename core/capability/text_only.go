package capability

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// TextOnlyOptionOffer is the complete, immutable authority envelope for the
// Phase 1 primary-channel agent. It is deliberately separate from ordinary
// profile readiness: a profile cannot become selectable unless every pinned
// subscription and isolation property is present and valid.
type TextOnlyOptionOffer struct {
	OfferVersion                 int    `json:"offer_version"`
	MachineID                    string `json:"machine_id"`
	SeatID                       string `json:"seat_id"`
	AgentKey                     string `json:"agent_key"`
	ProfileID                    string `json:"profile_id"`
	RequestedModel               string `json:"requested_model"`
	ResolvedModel                string `json:"resolved_model"`
	AccountType                  string `json:"account_type"`
	AccountPlan                  string `json:"account_plan"`
	PolicyID                     string `json:"policy_id"`
	PolicyRevision               string `json:"policy_revision"`
	RuntimeContract              string `json:"runtime_contract"`
	ReasoningEffort              string `json:"reasoning_effort"`
	ReasoningContext             string `json:"reasoning_context"`
	RequestTimeoutMillis         int    `json:"request_timeout_millis"`
	DeveloperInstructionRevision string `json:"developer_instruction_revision"`
	AdapterID                    string `json:"adapter_id"`
	AdapterRevision              string `json:"adapter_revision"`
	CodexVersion                 string `json:"codex_version"`
	CodexExecutableRevision      string `json:"codex_executable_revision"`
	CodexSchemaRevision          string `json:"codex_schema_revision"`
	ThreadMode                   string `json:"thread_mode"`
	SandboxMode                  string `json:"sandbox_mode"`
	ApprovalPolicy               string `json:"approval_policy"`
	WorkdirMode                  string `json:"workdir_mode"`
	DynamicToolsMode             string `json:"dynamic_tools_mode"`
	MCPMode                      string `json:"mcp_mode"`
	CommandPolicy                string `json:"command_policy"`
	FileReadPolicy               string `json:"file_read_policy"`
	IsolationRevision            string `json:"isolation_revision"`
}

var knownChatGPTPlans = map[string]bool{
	"free": true, "go": true, "plus": true, "pro": true, "prolite": true,
	"team": true, "self_serve_business_prolite": true,
	"self_serve_business_usage_based": true, "business": true,
	"ent26": true, "enterprise_cbp_automation": true,
	"enterprise_cbp_usage_based": true, "enterprise": true, "edu": true,
}

// TextOnlySeatID binds a selectable seat to one exact profile, machine, and
// requested model. Callers cannot provide an alternate identity.
func TextOnlySeatID(profileID, machineID, requestedModel string) string {
	digest := sha256.Sum256([]byte(profileID + "\x00" + machineID + "\x00" + requestedModel))
	return "seat:v1:" + hex.EncodeToString(digest[:])
}

// NormalizeTextOnlyOptionOffer validates the closed Phase 1 authority
// contract and returns its canonical server-computed option ID.
func NormalizeTextOnlyOptionOffer(offer TextOnlyOptionOffer, expectedMachine string) (TextOnlyOptionOffer, string, error) {
	if offer.OfferVersion != 1 {
		return TextOnlyOptionOffer{}, "", fmt.Errorf("capability: unsupported text-only offer version %d", offer.OfferVersion)
	}
	if expectedMachine == "" || offer.MachineID != expectedMachine {
		return TextOnlyOptionOffer{}, "", fmt.Errorf("capability: text-only offer machine mismatch")
	}
	if offer.AgentKey != "codex-subscription" ||
		offer.ProfileID != "codex-subscription:gpt-5.6-sol" ||
		offer.RequestedModel != "gpt-5.6-sol" || offer.ResolvedModel != "unknown" {
		return TextOnlyOptionOffer{}, "", fmt.Errorf("capability: text-only offer model identity mismatch")
	}
	if offer.SeatID != TextOnlySeatID(offer.ProfileID, offer.MachineID, offer.RequestedModel) {
		return TextOnlyOptionOffer{}, "", fmt.Errorf("capability: text-only offer seat mismatch")
	}
	if offer.AccountType != "chatgpt" || !knownChatGPTPlans[offer.AccountPlan] {
		return TextOnlyOptionOffer{}, "", fmt.Errorf("capability: unsupported subscription authority")
	}
	if offer.PolicyID != "codex-subscription-chat-v1" || !lowerHexRevision(offer.PolicyRevision) ||
		offer.RuntimeContract != "codex_subscription_exec_v1" {
		return TextOnlyOptionOffer{}, "", fmt.Errorf("capability: text-only offer policy mismatch")
	}
	if offer.ReasoningEffort != "medium" || offer.ReasoningContext != "current_turn" ||
		offer.RequestTimeoutMillis != 120_000 || !lowerHexRevision(offer.DeveloperInstructionRevision) {
		return TextOnlyOptionOffer{}, "", fmt.Errorf("capability: text-only offer request contract mismatch")
	}
	if offer.AdapterID != "model.chat.text-only.codex-subscription" || !lowerHexRevision(offer.AdapterRevision) {
		return TextOnlyOptionOffer{}, "", fmt.Errorf("capability: text-only offer adapter mismatch")
	}
	if offer.CodexVersion != strings.TrimSpace(offer.CodexVersion) ||
		!strings.HasPrefix(offer.CodexVersion, "codex-cli ") || len(offer.CodexVersion) > 128 ||
		!lowerHexRevision(offer.CodexExecutableRevision) || !lowerHexRevision(offer.CodexSchemaRevision) {
		return TextOnlyOptionOffer{}, "", fmt.Errorf("capability: text-only offer Codex revision mismatch")
	}
	if offer.ThreadMode != "ephemeral" || offer.SandboxMode != "readOnly" ||
		offer.ApprovalPolicy != "never" || offer.WorkdirMode != "empty_per_target" ||
		offer.DynamicToolsMode != "none" || offer.MCPMode != "none" ||
		offer.CommandPolicy != "deny_and_fail" || offer.FileReadPolicy != "deny_and_fail" ||
		!lowerHexRevision(offer.IsolationRevision) {
		return TextOnlyOptionOffer{}, "", fmt.Errorf("capability: text-only offer isolation mismatch")
	}

	wire, err := json.Marshal(offer)
	if err != nil {
		return TextOnlyOptionOffer{}, "", fmt.Errorf("capability: encode text-only offer: %w", err)
	}
	digest := sha256.Sum256(append(append([]byte("primary-option:v1\n"), wire...), '\n'))
	return offer, "primary-option:v1:" + hex.EncodeToString(digest[:]), nil
}

func lowerHexRevision(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, ch := range value {
		if (ch < '0' || ch > '9') && (ch < 'a' || ch > 'f') {
			return false
		}
	}
	return true
}
