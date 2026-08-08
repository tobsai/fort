package capability

import (
	"fmt"
	"strings"
)

type InstructionStep struct {
	Kind  string   `json:"kind"`
	Text  string   `json:"text,omitempty"`
	Label string   `json:"label,omitempty"`
	URL   string   `json:"url,omitempty"`
	Argv  []string `json:"argv,omitempty"`
}

type Deficit struct {
	DeficitID       string         `json:"deficit_id"`
	Kind            string         `json:"kind"`
	ID              string         `json:"id"`
	PredicateID     string         `json:"predicate_id"`
	Machine         string         `json:"machine"`
	PredicateState  PredicateState `json:"predicate_state"`
	Reason          Reason         `json:"reason"`
	RemedyEffectID  string         `json:"remedy_effect_id"`
	PostconditionID string         `json:"postcondition_id"`
}

type Instruction struct {
	ID              string            `json:"id"`
	Version         int               `json:"version"`
	OperationID     string            `json:"operation_id"`
	TemplateID      string            `json:"template_id"`
	TemplateVersion int               `json:"template_version"`
	Target          string            `json:"target"`
	Covers          []string          `json:"covers"`
	RemedyEffectID  string            `json:"remedy_effect_id"`
	PostconditionID string            `json:"postcondition_id"`
	EffectSummary   string            `json:"effect_summary"`
	Steps           []InstructionStep `json:"steps"`
}

type InstructionBundle struct {
	ID            string        `json:"id"`
	Version       int           `json:"version"`
	EffectSummary string        `json:"effect_summary"`
	Instructions  []Instruction `json:"instructions"`
}

type InstructionBundleRef struct {
	ID      string `json:"id"`
	Version int    `json:"version"`
}

type HypotheticalMappingEntry struct {
	StageID  string   `json:"stage_id"`
	Machine  string   `json:"machine"`
	Profile  string   `json:"profile"`
	Binding  string   `json:"binding"`
	Deficits []string `json:"deficits"`
}

type SetupAlternative struct {
	Mode                 string                     `json:"mode"`
	Deficits             []Deficit                  `json:"deficits"`
	InstructionBundleRef InstructionBundleRef       `json:"instruction_bundle_ref"`
	HypotheticalMapping  []HypotheticalMappingEntry `json:"hypothetical_mapping"`
	EffectSummary        string                     `json:"effect_summary"`
	InstructionBundle    InstructionBundle          `json:"instruction_bundle"`
}

type remedy struct {
	TemplateID      string
	EffectID        string
	PostconditionID string
	EffectSummary   string
}

type predicateInstance struct {
	Kind        string
	ID          string
	MatchTarget string
	Predicate   Predicate
}

type remedyOperation struct {
	ID              string
	Machine         string
	TemplateID      string
	EffectID        string
	PostconditionID string
	EffectSummary   string
	Deficits        []Deficit
}

func remedyFor(instance predicateInstance) (remedy, bool) {
	predicate := instance.Predicate
	reason := predicate.Reason
	if predicate.State == PredicateSatisfied {
		return remedy{}, false
	}
	var template, effect, summary string
	switch {
	case instance.MatchTarget == "profile.codex.native" && predicate.ID == "predicate.codex.native-contract.v1" &&
		oneOfReason(reason, ReasonAbsent, ReasonIncompatibleVersion, ReasonCommandContractChanged):
		template, effect, summary = "setup.codex.capability-runtime-update.v1", codexCapabilityRuntimeEffectID, "Install the catalog-supported Codex capability runtime."
	case instance.MatchTarget == "profile.codex.native" && predicate.ID == "predicate.codex.authenticated-subject.v1" && reason == ReasonAuthRequired:
		template, effect, summary = "setup.codex.login.v1", "effect.codex.authenticated-subject.v1", "Authenticate the selected local Codex account."
	case instance.Kind == "profile" && strings.HasPrefix(instance.ID, "codex:") &&
		predicate.ID == "predicate.codex.model."+instance.ID+".v1" && reason == ReasonModelUnavailable:
		template, effect, summary = "setup.codex.model-availability.v1", "effect.codex.model-ready."+instance.ID+".v1", "Make the exact Codex model available, then Recheck."
	case instance.MatchTarget == "profile.claude.native" && predicate.ID == "predicate.claude.native-contract.v1" &&
		oneOfReason(reason, ReasonAbsent, ReasonIncompatibleVersion, ReasonCommandContractChanged):
		template, effect, summary = "setup.claude.install-or-update.v1", "effect.claude-2.1.207.v1", "Install the catalog-supported Claude Code version."
	case instance.MatchTarget == "profile.claude.native" && predicate.ID == "predicate.claude.authenticated-subject.v1" && reason == ReasonAuthRequired:
		template, effect, summary = "setup.claude.login.v1", "effect.claude.authenticated-subject.v1", "Authenticate Claude Code locally."
	case instance.MatchTarget == "profile.hermes.native" && predicate.ID == "predicate.hermes.native-contract.v1" &&
		oneOfReason(reason, ReasonAbsent, ReasonIncompatibleVersion, ReasonCommandContractChanged):
		template, effect, summary = "setup.hermes.install-or-update.v1", "effect.hermes-0.15.1.v1", "Install the catalog-supported Hermes Agent version."
	case instance.MatchTarget == "profile.hermes.native" && strings.HasPrefix(predicate.ID, "predicate.hermes.provider-model.") &&
		oneOfReason(reason, ReasonAuthRequired, ReasonModelUnavailable):
		template, effect, summary = "setup.hermes.configure-provider.v1", "effect.hermes.provider-model."+instance.ID+".v1", "Configure the exact Hermes provider and model."
	case instance.MatchTarget == "profile.openclaw.main" && predicate.ID == "predicate.openclaw.native-contract.v1" &&
		oneOfReason(reason, ReasonAbsent, ReasonIncompatibleVersion, ReasonCommandContractChanged):
		template, effect, summary = "setup.openclaw.install-or-update.v1", "effect.openclaw-2026.7.1-2.v1", "Install the catalog-supported OpenClaw main-agent contract."
	case instance.MatchTarget == "profile.openclaw.main" && predicate.ID == "predicate.openclaw.main-ready.v1" &&
		oneOfReason(reason, ReasonAuthRequired, ReasonModelUnavailable):
		template, effect, summary = "setup.openclaw.configure-main.v1", "effect.openclaw.main-ready.v1", "Configure and authenticate the OpenClaw main agent."
	case instance.MatchTarget == "email.gmail.read.himalaya-broker" && predicate.ID == "predicate.himalaya.preview-contract.v1" &&
		oneOfReason(reason, ReasonAbsent, ReasonIncompatibleVersion, ReasonCommandContractChanged):
		template, effect, summary = "setup.himalaya.install-or-update.v1", "effect.himalaya-1.2.0-preview.v1", "Install Himalaya 1.2.0 with non-mutating preview support."
	case instance.MatchTarget == "email.gmail.read.himalaya-broker" && predicate.ID == "predicate.gmail.selected-imap-preview-read.v1" && reason == ReasonAuthRequired:
		template, effect, summary = "setup.gmail.configure-readonly.v1", "effect.gmail.selected-imap-read.v1", "Configure read-only Gmail access for Fort."
	case instance.MatchTarget == "database.supabase.inspect.codex-broker" && predicate.ID == "predicate.codex.capability-runtime.v1" &&
		oneOfReason(reason, ReasonAbsent, ReasonIncompatibleVersion, ReasonCommandContractChanged):
		template, effect, summary = "setup.codex.capability-runtime-update.v1", codexCapabilityRuntimeEffectID, "Install the catalog-supported Codex capability runtime."
	case instance.MatchTarget == "database.supabase.inspect.codex-broker" && predicate.ID == "predicate.supabase.selected-project-readonly.v1" &&
		oneOfReason(reason, ReasonAuthRequired, ReasonPluginUnready, ReasonProjectUnavailable):
		template, effect, summary = "setup.supabase.connect-readonly-project.v1", "effect.supabase.selected-project-readonly.v1", "Connect one project-scoped read-only Supabase root."
	case (instance.MatchTarget == "codex-appserver+gmail" || instance.MatchTarget == "codex-appserver+supabase") &&
		predicate.ID == "predicate.binding."+instance.MatchTarget+".v1" &&
		oneOfReason(reason, ReasonIncompatibleVersion, ReasonCommandContractChanged):
		template, effect, summary = "setup.codex.capability-runtime-update.v1", codexCapabilityRuntimeEffectID, "Install the catalog-supported Codex capability runtime."
	default:
		return remedy{}, false
	}
	return remedy{
		TemplateID: template, EffectID: effect,
		PostconditionID: strings.Replace(effect, "effect.", "postcondition.", 1),
		EffectSummary:   summary,
	}, true
}

func oneOfReason(got Reason, allowed ...Reason) bool {
	for _, reason := range allowed {
		if got == reason {
			return true
		}
	}
	return false
}

func deficitFor(machine string, instance predicateInstance, matched remedy) (Deficit, error) {
	input := struct {
		Kind            string `json:"kind"`
		TargetID        string `json:"target_id"`
		PredicateID     string `json:"predicate_id"`
		Machine         string `json:"machine"`
		Reason          Reason `json:"reason"`
		TemplateID      string `json:"template_id"`
		TemplateVersion int    `json:"template_version"`
		RemedyEffectID  string `json:"remedy_effect_id"`
		PostconditionID string `json:"postcondition_id"`
	}{
		Kind: instance.Kind, TargetID: instance.ID, PredicateID: instance.Predicate.ID,
		Machine: machine, Reason: instance.Predicate.Reason, TemplateID: matched.TemplateID,
		TemplateVersion: 1, RemedyEffectID: matched.EffectID, PostconditionID: matched.PostconditionID,
	}
	id, err := shortContentID("fort.setup-deficit.v1", "def_", input)
	if err != nil {
		return Deficit{}, err
	}
	return Deficit{
		DeficitID: id, Kind: instance.Kind, ID: instance.ID,
		PredicateID: instance.Predicate.ID, Machine: machine,
		PredicateState: instance.Predicate.State, Reason: instance.Predicate.Reason,
		RemedyEffectID: matched.EffectID, PostconditionID: matched.PostconditionID,
	}, nil
}

func operationID(machine, effect, postcondition string) (string, error) {
	return shortContentID("fort.remedy-operation.v1", "remop_", struct {
		Machine         string `json:"machine"`
		RemedyEffectID  string `json:"remedy_effect_id"`
		PostconditionID string `json:"postcondition_id"`
	}{machine, effect, postcondition})
}

func instructionFor(operation remedyOperation) (Instruction, error) {
	steps := instructionSteps(operation.TemplateID)
	covers := make([]string, len(operation.Deficits))
	for i, deficit := range operation.Deficits {
		covers[i] = deficit.DeficitID
	}
	input := struct {
		Version         int               `json:"version"`
		OperationID     string            `json:"operation_id"`
		TemplateID      string            `json:"template_id"`
		TemplateVersion int               `json:"template_version"`
		Target          string            `json:"target"`
		Covers          []string          `json:"covers"`
		RemedyEffectID  string            `json:"remedy_effect_id"`
		PostconditionID string            `json:"postcondition_id"`
		EffectSummary   string            `json:"effect_summary"`
		Steps           []InstructionStep `json:"steps"`
	}{
		Version: 1, OperationID: operation.ID, TemplateID: operation.TemplateID,
		TemplateVersion: 1, Target: operation.Machine, Covers: covers,
		RemedyEffectID: operation.EffectID, PostconditionID: operation.PostconditionID,
		EffectSummary: operation.EffectSummary, Steps: steps,
	}
	id, err := shortContentID("fort.setup-instruction.v1", "instruction_", input)
	if err != nil {
		return Instruction{}, err
	}
	return Instruction{
		ID: id, Version: 1, OperationID: operation.ID, TemplateID: operation.TemplateID,
		TemplateVersion: 1, Target: operation.Machine, Covers: covers,
		RemedyEffectID: operation.EffectID, PostconditionID: operation.PostconditionID,
		EffectSummary: operation.EffectSummary, Steps: steps,
	}, nil
}

func instructionSteps(template string) []InstructionStep {
	switch template {
	case "setup.codex.capability-runtime-update.v1", "setup.codex.model-availability.v1":
		return []InstructionStep{
			{Kind: "text", Text: "On this Mac, install the catalog-supported Codex CLI and verify the exact required model or capability runtime, then return to Fort and choose Recheck."},
			{Kind: "link", Label: "Codex installation guide", URL: "https://help.openai.com/en/articles/11096431"},
			{Kind: "link", Label: "Codex app-server protocol", URL: "https://learn.chatgpt.com/docs/developer-commands?surface=cli#cli-codex-app-server"},
		}
	case "setup.codex.login.v1":
		return []InstructionStep{
			{Kind: "text", Text: "On this Mac, authenticate the selected Codex account interactively, then return to Fort and choose Recheck."},
			{Kind: "display_command", Argv: []string{"codex", "login"}},
		}
	case "setup.claude.install-or-update.v1", "setup.claude.login.v1":
		return []InstructionStep{
			{Kind: "text", Text: "On this Mac, install or authenticate the catalog-supported Claude Code version, then return to Fort and choose Recheck."},
			{Kind: "link", Label: "Claude Code setup guide", URL: "https://docs.anthropic.com/en/docs/claude-code/getting-started"},
		}
	case "setup.hermes.install-or-update.v1", "setup.hermes.configure-provider.v1":
		return []InstructionStep{
			{Kind: "text", Text: "On this Mac, install Hermes Agent or configure the exact cataloged provider and model, then return to Fort and choose Recheck."},
			{Kind: "link", Label: "Hermes Agent documentation", URL: "https://hermes-agent.nousresearch.com/docs/"},
		}
	case "setup.openclaw.install-or-update.v1", "setup.openclaw.configure-main.v1":
		return []InstructionStep{{Kind: "text", Text: "On this Mac, use its approved package source to install OpenClaw 2026.7.1-2 and configure the main agent, then return to Fort and choose Recheck."}}
	case "setup.himalaya.install-or-update.v1", "setup.gmail.configure-readonly.v1":
		return []InstructionStep{
			{Kind: "text", Text: "On this Mac, configure the Fort-selected Gmail IMAP account with imap.gmail.com, TLS, and non-mutating preview access, then return to Fort and choose Recheck."},
			{Kind: "link", Label: "Himalaya v1.2 setup", URL: "https://github.com/pimalaya/himalaya/tree/v1.2.0"},
		}
	case "setup.supabase.connect-readonly-project.v1":
		return []InstructionStep{
			{Kind: "text", Text: "On this Mac, connect exactly one project-scoped Supabase capability root using OAuth and read-only mode, then return to Fort and choose Recheck."},
			{Kind: "link", Label: "Supabase MCP setup guide", URL: "https://supabase.com/docs/guides/ai-tools/mcp"},
		}
	default:
		return []InstructionStep{{Kind: "text", Text: "Complete the cataloged setup on this Mac, then return to Fort and choose Recheck."}}
	}
}

func bundleFor(operations []remedyOperation) (InstructionBundle, error) {
	if len(operations) < 1 || len(operations) > 8 {
		return InstructionBundle{}, fmt.Errorf("capability: setup bundle requires 1 to 8 instructions")
	}
	ordered, err := orderOperations(operations)
	if err != nil {
		return InstructionBundle{}, err
	}
	instructions := make([]Instruction, len(ordered))
	for i, operation := range ordered {
		instructions[i], err = instructionFor(operation)
		if err != nil {
			return InstructionBundle{}, err
		}
	}
	summary := "Complete the disclosed setup on the selected machines, then Recheck."
	input := struct {
		Version       int           `json:"version"`
		EffectSummary string        `json:"effect_summary"`
		Instructions  []Instruction `json:"instructions"`
	}{1, summary, instructions}
	id, err := shortContentID("fort.setup-bundle.v1", "instruction-bundle_", input)
	if err != nil {
		return InstructionBundle{}, err
	}
	bundle := InstructionBundle{ID: id, Version: 1, EffectSummary: summary, Instructions: instructions}
	canonical, err := canonicalJSON(bundle)
	if err != nil || len(canonical) > 32*1024 {
		return InstructionBundle{}, fmt.Errorf("capability: setup bundle exceeds the 32 KiB bound")
	}
	return bundle, nil
}

func orderOperations(operations []remedyOperation) ([]remedyOperation, error) {
	remaining := append([]remedyOperation(nil), operations...)
	ordered := make([]remedyOperation, 0, len(remaining))
	for len(remaining) > 0 {
		progress := false
		for i := 0; i < len(remaining); {
			candidate := remaining[i]
			if operationDependenciesSatisfied(candidate, operations, ordered) {
				ordered = append(ordered, candidate)
				remaining = append(remaining[:i], remaining[i+1:]...)
				progress = true
				continue
			}
			i++
		}
		if !progress {
			return nil, fmt.Errorf("capability: setup operation dependency cycle")
		}
	}
	return ordered, nil
}

func operationDependenciesSatisfied(candidate remedyOperation, all, ordered []remedyOperation) bool {
	for _, dependencyEffect := range effectDependencies(candidate.EffectID) {
		needed := false
		for _, operation := range all {
			if operation.Machine == candidate.Machine && operation.EffectID == dependencyEffect {
				needed = true
				break
			}
		}
		if !needed {
			continue
		}
		found := false
		for _, operation := range ordered {
			if operation.Machine == candidate.Machine && operation.EffectID == dependencyEffect {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func effectDependencies(effect string) []string {
	switch {
	case effect == "effect.codex.authenticated-subject.v1":
		return []string{codexCapabilityRuntimeEffectID}
	case strings.HasPrefix(effect, "effect.codex.model-ready."):
		return []string{"effect.codex.authenticated-subject.v1"}
	case effect == "effect.claude.authenticated-subject.v1":
		return []string{"effect.claude-2.1.207.v1"}
	case strings.HasPrefix(effect, "effect.hermes.provider-model."):
		return []string{"effect.hermes-0.15.1.v1"}
	case effect == "effect.openclaw.main-ready.v1":
		return []string{"effect.openclaw-2026.7.1-2.v1"}
	case effect == "effect.gmail.selected-imap-read.v1":
		return []string{"effect.himalaya-1.2.0-preview.v1"}
	case effect == "effect.supabase.selected-project-readonly.v1":
		return []string{codexCapabilityRuntimeEffectID}
	default:
		return nil
	}
}
