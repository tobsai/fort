package capability

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	corecap "github.com/tobsai/fort/core/capability"
	"github.com/tobsai/fort/exec/codexsubscription"
)

const (
	codexVersion    = codexsubscription.CodexVersion
	claudeVersion   = "2.1.207 (Claude Code)"
	hermesVersion   = "Hermes Agent v0.15.1"
	openClawVersion = "2026.7.1-2"
	himalayaVersion = "1.2.0"

	codexNormalSchemaDigest       = codexsubscription.CodexNormalSchemaRevision
	codexNormalSchemaFiles        = codexsubscription.CodexNormalSchemaFiles
	codexExperimentalSchemaDigest = codexsubscription.CodexExperimentalSchemaRevision
	codexExperimentalSchemaFiles  = codexsubscription.CodexExperimentalSchemaFiles
)

// CodexInspection contains normalized process-private facts extracted from one
// no-turn app-server session and its generated schema bundles. The executable
// digest is compared internally and must never cross a public API boundary.
type CodexInspection struct {
	AccountReady             bool
	AccountHandle            string
	AccountType              string
	AccountPlan              string
	Models                   map[string]bool
	DefaultModel             string
	ExecutableDigest         string
	NormalSchemaDigest       string
	NormalSchemaFiles        int
	ExperimentalSchemaDigest string
	ExperimentalSchemaFiles  int
	GmailIsolationReady      bool
	SupabaseIsolationReady   bool
}

type CodexInspector interface {
	Inspect(context.Context) (CodexInspection, error)
}

type GmailInspection struct {
	Ready         bool
	BindingHandle string
}

type GmailInspector interface {
	InspectGmail(context.Context) (GmailInspection, error)
}

type SupabaseInspection struct {
	Ready         bool
	BindingHandle string
}

type SupabaseInspector interface {
	InspectSupabase(context.Context) (SupabaseInspection, error)
}

type commandIdentityAuthorizer interface {
	AuthorizeExecutable(name, digest string) error
}

// ProbeError permits a private inspector to return a closed public reason
// without exposing its underlying output or credentials.
type ProbeError struct {
	Reason corecap.Reason
}

func (e *ProbeError) Error() string { return "capability probe failed" }

// LocalProber implements only the catalog's closed predicate IDs. Raw command
// output and private connector identifiers are reduced to stable binding
// material before Registry hashes them into opaque revisions.
type LocalProber struct {
	commands CommandExecutor
	codex    CodexInspector
	gmail    GmailInspector
	supabase SupabaseInspector
}

func NewLocalProber(commands CommandExecutor, codex CodexInspector, gmail GmailInspector, supabase SupabaseInspector) *LocalProber {
	return &LocalProber{commands: commands, codex: codex, gmail: gmail, supabase: supabase}
}

func (p *LocalProber) Probe(ctx context.Context, request ProbeRequest) ProbeObservation {
	switch {
	case request.PredicateID == "predicate.codex-subscription.closed-contract.v1":
		return p.codexSubscription(ctx)
	case request.PredicateID == "predicate.codex.native-contract.v1":
		return p.codexNative(ctx, false)
	case request.PredicateID == "predicate.codex.capability-runtime.v1":
		return p.codexNative(ctx, true)
	case request.PredicateID == "predicate.codex.authenticated-subject.v1":
		return p.codexAccount(ctx)
	case strings.HasPrefix(request.PredicateID, "predicate.codex.model."):
		return p.codexModel(ctx, request.ProfileID)
	case request.PredicateID == "predicate.claude.native-contract.v1":
		return p.exactVersion(ctx, "claude", []string{"--version"}, claudeVersion)
	case request.PredicateID == "predicate.claude.authenticated-subject.v1":
		return p.claudeAccount(ctx)
	case request.PredicateID == "predicate.hermes.native-contract.v1":
		return p.hermesNative(ctx)
	case strings.HasPrefix(request.PredicateID, "predicate.hermes.provider-model."):
		return p.hermesModel(ctx, request.ProfileID)
	case request.PredicateID == "predicate.openclaw.native-contract.v1":
		return p.openClawNative(ctx)
	case request.PredicateID == "predicate.openclaw.main-ready.v1":
		return p.openClawMain(ctx)
	case request.PredicateID == "predicate.himalaya.preview-contract.v1":
		return p.himalayaNative(ctx)
	case request.PredicateID == "predicate.gmail.selected-imap-preview-read.v1":
		return p.gmailReady(ctx)
	case request.PredicateID == "predicate.supabase.selected-project-readonly.v1":
		return p.supabaseReady(ctx)
	case strings.HasPrefix(request.PredicateID, "predicate.binding.codex-appserver+gmail."):
		return p.codexBinding(ctx, true)
	case strings.HasPrefix(request.PredicateID, "predicate.binding.codex-appserver+supabase."):
		return p.codexBinding(ctx, false)
	default:
		return unsatisfied(corecap.ReasonProbeFailed)
	}
}

func (p *LocalProber) codexSubscription(ctx context.Context) ProbeObservation {
	version := p.run(ctx, "codex", "--version")
	if version.Err != nil {
		return commandFailure(version.Err)
	}
	if strings.TrimSpace(string(version.Output)) != codexsubscription.CodexVersion ||
		!codexsubscription.AcceptsCodexExecutableRevision(version.ExecutableDigest) {
		return unsatisfied(corecap.ReasonIncompatibleVersion)
	}

	help := p.run(ctx, "codex", "exec", "--help")
	if help.Err != nil {
		return commandFailure(help.Err)
	}
	if help.ExecutableDigest != version.ExecutableDigest || !codexExecHelpAccepted(help.Output) {
		return unsatisfied(corecap.ReasonCommandContractChanged)
	}
	features := p.run(ctx, "codex", "features", "list")
	if features.Err != nil {
		return commandFailure(features.Err)
	}
	if features.ExecutableDigest != version.ExecutableDigest || !codexFeaturesAccepted(features.Output) {
		return unsatisfied(corecap.ReasonCommandContractChanged)
	}

	inspection, observation := p.inspectCodex(ctx)
	if observation.State != "" {
		return observation
	}
	if inspection.ExecutableDigest != version.ExecutableDigest ||
		inspection.NormalSchemaDigest != codexsubscription.CodexNormalSchemaRevision ||
		inspection.NormalSchemaFiles != codexsubscription.CodexNormalSchemaFiles ||
		inspection.ExperimentalSchemaDigest != codexsubscription.CodexExperimentalSchemaRevision ||
		inspection.ExperimentalSchemaFiles != codexsubscription.CodexExperimentalSchemaFiles {
		return unsatisfied(corecap.ReasonIncompatibleVersion)
	}
	if !inspection.AccountReady || inspection.AccountType != "chatgpt" {
		return unsatisfied(corecap.ReasonAuthRequired)
	}
	if !inspection.Models["gpt-5.6-sol"] {
		return unsatisfied(corecap.ReasonModelUnavailable)
	}

	offer := corecap.TextOnlyOptionOffer{
		OfferVersion: 1, AgentKey: "codex-subscription",
		ProfileID: "codex-subscription:gpt-5.6-sol", RequestedModel: "gpt-5.6-sol", ResolvedModel: "unknown",
		AccountType: inspection.AccountType, AccountPlan: inspection.AccountPlan,
		PolicyID: "codex-subscription-chat-v1", PolicyRevision: codexsubscription.PolicyRevision,
		RuntimeContract: "codex_subscription_exec_v1", ReasoningEffort: "medium", ReasoningContext: "current_turn",
		RequestTimeoutMillis:         codexsubscription.TargetTimeoutMillis,
		DeveloperInstructionRevision: codexsubscription.DeveloperInstructionRevision,
		AdapterID:                    "model.chat.text-only.codex-subscription", AdapterRevision: codexsubscription.AdapterRevision,
		CodexVersion: codexsubscription.CodexVersion, CodexExecutableRevision: version.ExecutableDigest,
		CodexSchemaRevision: codexsubscription.CodexSchemaRevision,
		ThreadMode:          "ephemeral", SandboxMode: "readOnly", ApprovalPolicy: "never", WorkdirMode: "empty_per_target",
		DynamicToolsMode: "none", MCPMode: "none", CommandPolicy: "deny_and_fail", FileReadPolicy: "deny_and_fail",
		IsolationRevision: codexsubscription.IsolationRevision,
	}
	probeOffer := offer
	probeOffer.MachineID = "local-probe"
	probeOffer.SeatID = corecap.TextOnlySeatID(probeOffer.ProfileID, probeOffer.MachineID, probeOffer.RequestedModel)
	if _, _, err := corecap.NormalizeTextOnlyOptionOffer(probeOffer, probeOffer.MachineID); err != nil {
		return unsatisfied(corecap.ReasonCommandContractChanged)
	}
	if !p.authorizeExecutable("codex", version.ExecutableDigest) {
		return unsatisfied(corecap.ReasonCapabilityDrift)
	}
	return ProbeObservation{
		State: corecap.PredicateSatisfied,
		StableBinding: []string{
			"account_type=" + offer.AccountType, "account_plan=" + offer.AccountPlan,
			"codex_version=" + offer.CodexVersion, "executable=" + offer.CodexExecutableRevision,
			"schema=" + offer.CodexSchemaRevision, "policy=" + offer.PolicyRevision,
			"adapter=" + offer.AdapterRevision, "isolation=" + offer.IsolationRevision,
		},
		TextOnlyOption: &offer,
	}
}

func codexExecHelpAccepted(output []byte) bool {
	available := make(map[string]bool)
	for _, field := range strings.Fields(string(output)) {
		available[strings.TrimSuffix(field, ",")] = true
	}
	for _, required := range []string{
		"--json", "--sandbox", "--skip-git-repo-check", "--ephemeral", "--ignore-user-config",
		"--ignore-rules", "--strict-config", "--cd", "--model", "--config", "--disable",
	} {
		if !available[required] {
			return false
		}
	}
	return true
}

func codexFeaturesAccepted(output []byte) bool {
	available := make(map[string]bool)
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 {
			available[fields[0]] = true
		}
	}
	for _, feature := range codexsubscription.DisabledFeatures() {
		if !available[feature] {
			return false
		}
	}
	return true
}

func (p *LocalProber) exactVersion(ctx context.Context, command string, args []string, expected string) ProbeObservation {
	result := p.run(ctx, command, args...)
	if result.Err != nil {
		return commandFailure(result.Err)
	}
	if strings.TrimSpace(string(result.Output)) != expected {
		return unsatisfied(corecap.ReasonIncompatibleVersion)
	}
	if !p.authorizeExecutable(command, result.ExecutableDigest) {
		return unsatisfied(corecap.ReasonCapabilityDrift)
	}
	return satisfied("command="+command, "version="+expected, "executable="+result.ExecutableDigest)
}

func (p *LocalProber) codexNative(ctx context.Context, experimental bool) ProbeObservation {
	version := p.run(ctx, "codex", "--version")
	if version.Err != nil {
		return commandFailure(version.Err)
	}
	if strings.TrimSpace(string(version.Output)) != codexVersion {
		return unsatisfied(corecap.ReasonIncompatibleVersion)
	}
	if p.codex == nil {
		return unsatisfied(corecap.ReasonIncompatibleVersion)
	}
	inspection, err := p.codex.Inspect(ctx)
	if err != nil {
		return inspectionFailure(err, corecap.ReasonIncompatibleVersion)
	}
	if inspection.ExecutableDigest == "" || inspection.ExecutableDigest != version.ExecutableDigest {
		return unsatisfied(corecap.ReasonIncompatibleVersion)
	}
	if inspection.NormalSchemaDigest != codexNormalSchemaDigest || inspection.NormalSchemaFiles != codexNormalSchemaFiles {
		return unsatisfied(corecap.ReasonIncompatibleVersion)
	}
	binding := []string{
		"command=codex", "version=" + codexVersion,
		"executable=" + version.ExecutableDigest,
		"schema.normal=" + inspection.NormalSchemaDigest,
	}
	if experimental {
		if inspection.ExperimentalSchemaDigest != codexExperimentalSchemaDigest ||
			inspection.ExperimentalSchemaFiles != codexExperimentalSchemaFiles {
			return unsatisfied(corecap.ReasonIncompatibleVersion)
		}
		binding = append(binding, "schema.experimental="+inspection.ExperimentalSchemaDigest)
	}
	if !p.authorizeExecutable("codex", version.ExecutableDigest) {
		return unsatisfied(corecap.ReasonCapabilityDrift)
	}
	return satisfied(binding...)
}

func (p *LocalProber) codexAccount(ctx context.Context) ProbeObservation {
	inspection, observation := p.inspectCodex(ctx)
	if observation.State != "" {
		return observation
	}
	if !inspection.AccountReady {
		return unsatisfied(corecap.ReasonAuthRequired)
	}
	return satisfied("account=" + inspection.AccountHandle)
}

func (p *LocalProber) codexModel(ctx context.Context, profile string) ProbeObservation {
	inspection, observation := p.inspectCodex(ctx)
	if observation.State != "" {
		return observation
	}
	if !inspection.AccountReady {
		return unsatisfied(corecap.ReasonAuthRequired)
	}
	model := strings.TrimPrefix(profile, "codex:")
	if model == "configured-default" {
		model = inspection.DefaultModel
	}
	if model == "" || !inspection.Models[model] {
		return unsatisfied(corecap.ReasonModelUnavailable)
	}
	observation = satisfied("model=" + model)
	if profile == "codex:configured-default" {
		observation.ResolvedModel = model
	}
	return observation
}

func (p *LocalProber) inspectCodex(ctx context.Context) (CodexInspection, ProbeObservation) {
	if p.codex == nil {
		return CodexInspection{}, unsatisfied(corecap.ReasonIncompatibleVersion)
	}
	inspection, err := p.codex.Inspect(ctx)
	if err != nil {
		return CodexInspection{}, inspectionFailure(err, corecap.ReasonIncompatibleVersion)
	}
	if inspection.Models == nil {
		inspection.Models = map[string]bool{}
	}
	return inspection, ProbeObservation{}
}

func (p *LocalProber) claudeAccount(ctx context.Context) ProbeObservation {
	result := p.run(ctx, "claude", "auth", "status", "--json")
	var status struct {
		LoggedIn bool `json:"loggedIn"`
	}
	parseErr := json.Unmarshal(result.Output, &status)
	if parseErr == nil && !status.LoggedIn {
		return unsatisfied(corecap.ReasonAuthRequired)
	}
	if result.Err != nil {
		return commandFailure(result.Err)
	}
	if parseErr != nil {
		return unsatisfied(corecap.ReasonCommandContractChanged)
	}
	return satisfied("authenticated=true", "executable="+result.ExecutableDigest)
}

func (p *LocalProber) hermesNative(ctx context.Context) ProbeObservation {
	result := p.run(ctx, "hermes", "--version")
	if result.Err != nil {
		return commandFailure(result.Err)
	}
	if !matchesHermesVersion(result.Output) {
		return unsatisfied(corecap.ReasonIncompatibleVersion)
	}
	if !p.authorizeExecutable("hermes", result.ExecutableDigest) {
		return unsatisfied(corecap.ReasonCapabilityDrift)
	}
	return satisfied("command=hermes", "version="+hermesVersion, "executable="+result.ExecutableDigest)
}

func matchesHermesVersion(output []byte) bool {
	version := ""
	for _, line := range strings.Split(string(output), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			version = line
			break
		}
	}
	if version == hermesVersion {
		return true
	}
	prefix := hermesVersion + " ("
	if !strings.HasPrefix(version, prefix) || !strings.HasSuffix(version, ")") {
		return false
	}
	buildDate := strings.TrimSuffix(strings.TrimPrefix(version, prefix), ")")
	_, err := time.Parse("2006.1.2", buildDate)
	return err == nil
}

func (p *LocalProber) hermesModel(ctx context.Context, profile string) ProbeObservation {
	result := p.run(ctx, "hermes", "status", "--deep")
	if result.Err != nil {
		return commandFailure(result.Err)
	}
	output := string(result.Output)
	switch profile {
	case "hermes:configured-default":
		if strings.TrimSpace(output) == "" {
			return unsatisfied(corecap.ReasonAuthRequired)
		}
	case "hermes:openai-codex/gpt-5.6-sol":
		normalizedOutput := strings.Join(strings.Fields(output), " ")
		providerReady := strings.Contains(output, "openai-codex") || strings.Contains(normalizedOutput, "Provider: OpenAI Codex")
		if !providerReady || !strings.Contains(output, "gpt-5.6-sol") {
			return unsatisfied(corecap.ReasonModelUnavailable)
		}
	default:
		return unsatisfied(corecap.ReasonModelUnavailable)
	}
	return satisfied("profile="+profile, "executable="+result.ExecutableDigest)
}

func (p *LocalProber) openClawNative(ctx context.Context) ProbeObservation {
	result := p.run(ctx, "openclaw", "--version")
	if result.Err != nil {
		return commandFailure(result.Err)
	}
	if !strings.Contains(strings.TrimSpace(string(result.Output)), openClawVersion) {
		return unsatisfied(corecap.ReasonIncompatibleVersion)
	}
	if !p.authorizeExecutable("openclaw", result.ExecutableDigest) {
		return unsatisfied(corecap.ReasonCapabilityDrift)
	}
	return satisfied("command=openclaw", "version="+openClawVersion, "executable="+result.ExecutableDigest)
}

func (p *LocalProber) openClawMain(ctx context.Context) ProbeObservation {
	// OpenClaw 2026.7.1-2's config/model status commands spawn detached
	// openclaw-config helpers that re-parent themselves and create new process
	// groups. A bounded os/exec context cannot prove ownership or reap them.
	// Quarantine this readiness contract until Fort has a process-private API;
	// falsely reporting ready is less safe than a closed setup-required result.
	return unsatisfied(corecap.ReasonCommandContractChanged)
}

func (p *LocalProber) himalayaNative(ctx context.Context) ProbeObservation {
	result := p.run(ctx, "himalaya", "--version")
	if result.Err != nil {
		return commandFailure(result.Err)
	}
	if !strings.Contains(strings.TrimSpace(string(result.Output)), himalayaVersion) {
		return unsatisfied(corecap.ReasonIncompatibleVersion)
	}
	return satisfied("command=himalaya", "version="+himalayaVersion, "executable="+result.ExecutableDigest)
}

func (p *LocalProber) gmailReady(ctx context.Context) ProbeObservation {
	if p.gmail == nil {
		return unsatisfied(corecap.ReasonAuthRequired)
	}
	inspection, err := p.gmail.InspectGmail(ctx)
	if err != nil {
		return inspectionFailure(err, corecap.ReasonAuthRequired)
	}
	if !inspection.Ready {
		return unsatisfied(corecap.ReasonAuthRequired)
	}
	return satisfied("gmail=" + inspection.BindingHandle)
}

func (p *LocalProber) supabaseReady(ctx context.Context) ProbeObservation {
	if p.supabase == nil {
		return unsatisfied(corecap.ReasonProjectUnavailable)
	}
	inspection, err := p.supabase.InspectSupabase(ctx)
	if err != nil {
		return inspectionFailure(err, corecap.ReasonProjectUnavailable)
	}
	if !inspection.Ready {
		return unsatisfied(corecap.ReasonProjectUnavailable)
	}
	return satisfied("supabase=" + inspection.BindingHandle)
}

func (p *LocalProber) codexBinding(ctx context.Context, gmail bool) ProbeObservation {
	inspection, observation := p.inspectCodex(ctx)
	if observation.State != "" {
		return observation
	}
	if gmail && inspection.GmailIsolationReady {
		return satisfied("isolation=gmail")
	}
	if !gmail && inspection.SupabaseIsolationReady {
		return satisfied("isolation=supabase")
	}
	return unsatisfied(corecap.ReasonIncompatibleVersion)
}

func (p *LocalProber) run(ctx context.Context, command string, args ...string) CommandResult {
	if p.commands == nil {
		return CommandResult{Err: ErrCommandAbsent}
	}
	return p.commands.Run(ctx, command, args...)
}

func (p *LocalProber) authorizeExecutable(command, digest string) bool {
	authorizer, ok := p.commands.(commandIdentityAuthorizer)
	if !ok {
		// Test and alternate probers may not bind a native runtime. Production's
		// ResolverExecutor implements this seam and native dispatch independently
		// fails closed when no authorization exists.
		return true
	}
	return authorizer.AuthorizeExecutable(command, digest) == nil
}

func commandFailure(err error) ProbeObservation {
	switch {
	case errors.Is(err, ErrCommandAbsent):
		return unsatisfied(corecap.ReasonAbsent)
	case errors.Is(err, context.DeadlineExceeded):
		return unsatisfied(corecap.ReasonProbeTimedOut)
	case errors.Is(err, ErrCommandOutputLimit):
		return unsatisfied(corecap.ReasonOutputLimitExceeded)
	case errors.Is(err, ErrUnsupportedPlatform):
		return unsatisfied(corecap.ReasonUnsupportedPlatform)
	default:
		return unsatisfied(corecap.ReasonProbeFailed)
	}
}

func inspectionFailure(err error, fallback corecap.Reason) ProbeObservation {
	var probeError *ProbeError
	if errors.As(err, &probeError) && corecap.FirstReason(probeError.Reason) != "" {
		return unsatisfied(probeError.Reason)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return unsatisfied(corecap.ReasonProbeTimedOut)
	}
	return unsatisfied(fallback)
}

func satisfied(binding ...string) ProbeObservation {
	return ProbeObservation{State: corecap.PredicateSatisfied, StableBinding: binding}
}

func unsatisfied(reason corecap.Reason) ProbeObservation {
	return ProbeObservation{State: corecap.PredicateUnsatisfied, Reason: reason}
}
