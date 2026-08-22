package control

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	corecap "github.com/tobsai/fort/core/capability"
	"github.com/tobsai/fort/core/conversation"
	coreruntime "github.com/tobsai/fort/core/runtime"
	"github.com/tobsai/fort/core/store"
)

type primaryCapabilityFixture struct {
	mu               sync.Mutex
	snapshot         corecap.Snapshot
	machineOverride  *corecap.MachineInventory
	generation       uint64
	refreshes        [][]string
	machineRefreshes [][]string
}

func (f *primaryCapabilityFixture) Capabilities() (corecap.Snapshot, uint64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.snapshot, f.generation
}

func (f *primaryCapabilityFixture) Refresh(_ context.Context, mode corecap.RefreshMode, adapters []string) (corecap.Snapshot, uint64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if mode != corecap.RefreshUserRecheck {
		panic("primary capability refresh was not an explicit user recheck")
	}
	f.refreshes = append(f.refreshes, append([]string(nil), adapters...))
	return f.snapshot, f.generation, nil
}

func (f *primaryCapabilityFixture) RefreshMachine(_ context.Context, machine string, mode corecap.RefreshMode, adapters []string) (corecap.MachineInventory, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if mode != corecap.RefreshUserRecheck {
		panic("primary machine preflight was not fresh")
	}
	f.machineRefreshes = append(f.machineRefreshes, append([]string(nil), adapters...))
	if f.machineOverride != nil && f.machineOverride.Name == machine {
		return *f.machineOverride, nil
	}
	for _, item := range f.snapshot.Machines {
		if item.Name == machine {
			return item, nil
		}
	}
	return corecap.MachineInventory{}, context.Canceled
}

type primaryRuntimeFixture struct {
	mu     sync.Mutex
	specs  []coreruntime.RunSpec
	answer string
}

type primaryFailureRuntime struct {
	mu    sync.Mutex
	specs []coreruntime.RunSpec
	code  string
}

type primaryEventRuntime struct {
	events func(coreruntime.RunSpec) []coreruntime.RunEvent
}

func (r primaryEventRuntime) Name() string { return "primary-event-test" }
func (r primaryEventRuntime) Dispatch(_ context.Context, spec coreruntime.RunSpec) (coreruntime.Run, error) {
	return newPrimaryRun(spec.RunID, r.events(spec), coreruntime.Status{State: coreruntime.StateSucceeded}), nil
}

type primaryBlockingRuntime struct {
	mu   sync.Mutex
	runs []*primaryBlockingRun
}

func (r *primaryBlockingRuntime) Name() string { return "primary-blocking-test" }
func (r *primaryBlockingRuntime) Dispatch(_ context.Context, spec coreruntime.RunSpec) (coreruntime.Run, error) {
	run := &primaryBlockingRun{
		id: spec.RunID, events: make(chan coreruntime.RunEvent, 2), done: make(chan struct{}),
		status: coreruntime.Status{State: coreruntime.StateRunning},
	}
	run.events <- coreruntime.RunEvent{RunID: spec.RunID, Type: coreruntime.EventStarted, Time: time.Now().UTC()}
	r.mu.Lock()
	r.runs = append(r.runs, run)
	r.mu.Unlock()
	return run, nil
}

type primaryBlockingRun struct {
	id     string
	events chan coreruntime.RunEvent
	done   chan struct{}
	once   sync.Once
	mu     sync.Mutex
	status coreruntime.Status
}

func (r *primaryBlockingRun) ID() string                          { return r.id }
func (r *primaryBlockingRun) Stream() <-chan coreruntime.RunEvent { return r.events }
func (r *primaryBlockingRun) Signal(string) error                 { return nil }
func (r *primaryBlockingRun) Cancel() error {
	r.once.Do(func() {
		r.mu.Lock()
		r.status = coreruntime.Status{State: coreruntime.StateCanceled}
		r.mu.Unlock()
		r.events <- coreruntime.RunEvent{RunID: r.id, Type: coreruntime.EventExited, Time: time.Now().UTC()}
		close(r.events)
		close(r.done)
	})
	return nil
}
func (r *primaryBlockingRun) Status() coreruntime.Status {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.status
}
func (r *primaryBlockingRun) Wait() coreruntime.Status {
	<-r.done
	return r.Status()
}

func (r *primaryFailureRuntime) Name() string { return "primary-failure-test" }
func (r *primaryFailureRuntime) Dispatch(_ context.Context, spec coreruntime.RunSpec) (coreruntime.Run, error) {
	r.mu.Lock()
	r.specs = append(r.specs, spec)
	r.mu.Unlock()
	events := []coreruntime.RunEvent{
		{RunID: spec.RunID, Type: coreruntime.EventStarted, Time: time.Now().UTC()},
		{RunID: spec.RunID, Type: coreruntime.EventError, Time: time.Now().UTC(), Data: r.code, ErrorCode: r.code},
		{RunID: spec.RunID, Type: coreruntime.EventExited, Time: time.Now().UTC(), Code: -1},
	}
	return newPrimaryRun(spec.RunID, events, coreruntime.Status{State: coreruntime.StateFailed, ExitCode: -1, Err: r.code}), nil
}

func (r *primaryRuntimeFixture) Name() string { return "primary-test" }

func (r *primaryRuntimeFixture) Dispatch(_ context.Context, spec coreruntime.RunSpec) (coreruntime.Run, error) {
	r.mu.Lock()
	r.specs = append(r.specs, spec)
	r.mu.Unlock()
	metadata := &coreruntime.ResponseMetadata{
		ProviderThreadID: "thread-ephemeral", RequestedModel: spec.Model,
		ResolvedModel:                   coreruntime.UnknownProviderIdentity,
		SelectedAdapterID:               spec.TextOnlyPolicy.SelectedAdapterID,
		SelectedAdapterRevision:         spec.TextOnlyPolicy.SelectedAdapterRevision,
		SelectedCodexVersion:            spec.TextOnlyPolicy.SelectedCodexVersion,
		SelectedCodexExecutableRevision: spec.TextOnlyPolicy.SelectedCodexExecutableRevision,
		SelectedCodexSchemaRevision:     spec.TextOnlyPolicy.SelectedCodexSchemaRevision,
		ObservedAdapterID:               spec.TextOnlyPolicy.SelectedAdapterID,
		ObservedAdapterRevision:         spec.TextOnlyPolicy.SelectedAdapterRevision,
		ObservedCodexVersion:            spec.TextOnlyPolicy.SelectedCodexVersion,
		ObservedCodexExecutableRevision: spec.TextOnlyPolicy.SelectedCodexExecutableRevision,
		ObservedCodexSchemaRevision:     spec.TextOnlyPolicy.SelectedCodexSchemaRevision,
		TerminalStatus:                  "completed", UsageSource: coreruntime.UsageSourceCodexExecJSONL,
		Usage: coreruntime.ProviderUsage{InputTokens: 9, CachedInputTokens: 2, OutputTokens: 4, ReasoningTokens: 1},
	}
	events := []coreruntime.RunEvent{
		{RunID: spec.RunID, Type: coreruntime.EventStarted, Time: time.Now().UTC()},
		{RunID: spec.RunID, Type: coreruntime.EventMessage, Time: time.Now().UTC(), Data: r.answer, Response: metadata},
		{RunID: spec.RunID, Type: coreruntime.EventExited, Time: time.Now().UTC()},
	}
	return newPrimaryRun(spec.RunID, events, coreruntime.Status{State: coreruntime.StateSucceeded}), nil
}

func (r *primaryRuntimeFixture) captured() []coreruntime.RunSpec {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]coreruntime.RunSpec(nil), r.specs...)
}

type primaryRun struct {
	id     string
	events chan coreruntime.RunEvent
	status coreruntime.Status
}

func newPrimaryRun(id string, events []coreruntime.RunEvent, status coreruntime.Status) *primaryRun {
	stream := make(chan coreruntime.RunEvent, len(events))
	for _, event := range events {
		stream <- event
	}
	close(stream)
	return &primaryRun{id: id, events: stream, status: status}
}

func (r *primaryRun) ID() string                          { return r.id }
func (r *primaryRun) Stream() <-chan coreruntime.RunEvent { return r.events }
func (r *primaryRun) Signal(string) error                 { return nil }
func (r *primaryRun) Cancel() error                       { return nil }
func (r *primaryRun) Status() coreruntime.Status          { return r.status }
func (r *primaryRun) Wait() coreruntime.Status            { return r.status }

func primaryOffer(machine string) (corecap.TextOnlyOptionOffer, string) {
	offer := corecap.TextOnlyOptionOffer{
		OfferVersion: 1, MachineID: machine, AgentKey: "codex-subscription",
		ProfileID: "codex-subscription:gpt-5.6-sol", RequestedModel: "gpt-5.6-sol", ResolvedModel: "unknown",
		AccountType: "chatgpt", AccountPlan: "pro", PolicyID: "codex-subscription-chat-v1",
		PolicyRevision: strings.Repeat("1", 64), RuntimeContract: "codex_subscription_exec_v1",
		ReasoningEffort: "medium", ReasoningContext: "current_turn", RequestTimeoutMillis: 120000,
		DeveloperInstructionRevision: strings.Repeat("2", 64), AdapterID: "model.chat.text-only.codex-subscription",
		AdapterRevision: strings.Repeat("3", 64), CodexVersion: "codex-cli 0.147.0-alpha.6.5",
		CodexExecutableRevision: strings.Repeat("4", 64), CodexSchemaRevision: strings.Repeat("5", 64),
		ThreadMode: "ephemeral", SandboxMode: "readOnly", ApprovalPolicy: "never", WorkdirMode: "empty_per_target",
		DynamicToolsMode: "none", MCPMode: "none", CommandPolicy: "deny_and_fail", FileReadPolicy: "deny_and_fail",
		IsolationRevision: strings.Repeat("6", 64),
	}
	offer.SeatID = corecap.TextOnlySeatID(offer.ProfileID, offer.MachineID, offer.RequestedModel)
	_, id, err := corecap.NormalizeTextOnlyOptionOffer(offer, machine)
	if err != nil {
		panic(err)
	}
	return offer, id
}

func primaryCapability(machine string) (*primaryCapabilityFixture, string) {
	offer, id := primaryOffer(machine)
	return &primaryCapabilityFixture{generation: 1, snapshot: corecap.Snapshot{
		CatalogVersion: corecap.CatalogVersion, ProfileMappingVersion: corecap.ProfileMappingVersion,
		LocalMachine: machine, Machines: []corecap.MachineInventory{{
			Name: machine, Local: true, Reachable: true, ProtocolVersion: corecap.ProtocolVersion,
			CatalogVersion: corecap.CatalogVersion, ProfileMappingVersion: corecap.ProfileMappingVersion,
			TextOnlyOptions: []corecap.TextOnlyOptionOffer{offer},
		}},
	}}, id
}

func openPrimaryStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "fort.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func TestPrimaryChannelServiceProjectsAndPersistsOnlyCurrentOption(t *testing.T) {
	st := openPrimaryStore(t)
	capabilities, optionID := primaryCapability("studio")
	svc := NewPrimaryChannelService(st, nil, capabilities)
	svc.now = func() time.Time { return time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC) }

	view, err := svc.PrimaryAgent(context.Background())
	if err != nil || view.Selection != nil || view.State != PrimaryAgentNotConfigured || len(view.Options) != 1 || view.Options[0].ID != optionID {
		t.Fatalf("initial primary agent = %+v, %v", view, err)
	}
	view, err = svc.SetPrimaryAgent(context.Background(), optionID)
	if err != nil || view.Selection == nil || view.State != PrimaryAgentReady || view.Selection.OptionID != optionID {
		t.Fatalf("set primary agent = %+v, %v", view, err)
	}
	if view.Selection.Seat.Machine != "studio" || view.Selection.Policy.AccountPlan != "pro" || view.Selection.Policy.AdapterRevision != strings.Repeat("3", 64) {
		t.Fatalf("persisted selection = %+v", view.Selection)
	}
	if _, err := svc.SetPrimaryAgent(context.Background(), "primary-option:v1:forged"); ErrorCode(err) != ErrorPrimaryAgentUnready {
		t.Fatalf("forged option error = %v (%q)", err, ErrorCode(err))
	}
	if err := svc.ClearPrimaryAgent(context.Background()); err != nil {
		t.Fatal(err)
	}
	view, _ = svc.PrimaryAgent(context.Background())
	if view.Selection != nil || view.State != PrimaryAgentNotConfigured {
		t.Fatalf("cleared view = %+v", view)
	}

	if _, err := svc.RecheckPrimaryAgent(context.Background()); err != nil {
		t.Fatal(err)
	}
	capabilities.mu.Lock()
	gotAdapters := append([]string(nil), capabilities.refreshes[0]...)
	capabilities.mu.Unlock()
	wantAdapters := []string{"profile.codex-subscription.isolated", "model.chat.text-only.codex-subscription", "codex-subscription-chat"}
	if !reflect.DeepEqual(gotAdapters, wantAdapters) {
		t.Fatalf("recheck adapters = %v, want %v", gotAdapters, wantAdapters)
	}
}

func TestPrimaryAgentInventoryKeepsIneligibleAndUnreadyProfilesVisible(t *testing.T) {
	st := openPrimaryStore(t)
	capabilities, readyOptionID := primaryCapability("studio")
	conflictingOffer, _ := primaryOffer("conflict")
	capabilities.snapshot.Machines[0].Profiles = []corecap.ProfileOffer{
		{ID: "claude:sonnet", Agent: "claude", Adapter: "profile.claude.native", State: corecap.OfferReady},
		{ID: "codex-subscription:gpt-5.6-sol", Agent: "codex-subscription", Adapter: "profile.codex-subscription.isolated", State: corecap.OfferReady, ResolvedModel: "gpt-5.6-sol"},
	}
	capabilities.snapshot.Machines = append(capabilities.snapshot.Machines,
		corecap.MachineInventory{
			Name: "needs-login", Reachable: true, ProtocolVersion: corecap.ProtocolVersion,
			CatalogVersion: corecap.CatalogVersion, ProfileMappingVersion: corecap.ProfileMappingVersion,
			Profiles: []corecap.ProfileOffer{{
				ID: "codex-subscription:gpt-5.6-sol", Agent: "codex-subscription", Adapter: "profile.codex-subscription.isolated",
				State: corecap.OfferSetupRequired, Reason: corecap.ReasonAuthRequired,
			}}, TextOnlyOptions: []corecap.TextOnlyOptionOffer{},
		},
		corecap.MachineInventory{
			Name: "offline", Reachable: true, ProtocolVersion: corecap.ProtocolVersion,
			CatalogVersion: corecap.CatalogVersion, ProfileMappingVersion: corecap.ProfileMappingVersion,
			Profiles: []corecap.ProfileOffer{{
				ID: "codex-subscription:gpt-5.6-sol", Agent: "codex-subscription", Adapter: "profile.codex-subscription.isolated",
				State: corecap.OfferUnavailable, Reason: corecap.ReasonProbeFailed,
			}}, TextOnlyOptions: []corecap.TextOnlyOptionOffer{},
		},
		corecap.MachineInventory{
			Name: "conflict", Reachable: true, ProtocolVersion: corecap.ProtocolVersion,
			CatalogVersion: corecap.CatalogVersion, ProfileMappingVersion: corecap.ProfileMappingVersion,
			Profiles: []corecap.ProfileOffer{{
				ID: "codex-subscription:gpt-5.6-sol", Agent: "codex-subscription", Adapter: "profile.codex-subscription.isolated",
				State: corecap.OfferReady, ResolvedModel: "gpt-5.6-sol",
			}}, TextOnlyOptions: []corecap.TextOnlyOptionOffer{conflictingOffer, conflictingOffer},
		},
	)

	svc := NewPrimaryChannelService(st, nil, capabilities)
	view, err := svc.PrimaryAgent(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	byProfileMachine := map[string]PrimaryAgentOption{}
	for _, option := range view.Options {
		byProfileMachine[option.Seat.Profile+"@"+option.Seat.Machine] = option
	}
	if option := byProfileMachine["codex-subscription:gpt-5.6-sol@studio"]; option.ID != readyOptionID || option.State != PrimaryAgentReady {
		t.Fatalf("ready subscription option = %+v", option)
	}
	ordinary := byProfileMachine["claude:sonnet@studio"]
	if ordinary.ID == "" || ordinary.State != "ineligible" || ordinary.Reason != "not_eligible_for_text_only_chat" {
		t.Fatalf("ordinary profile = %+v", ordinary)
	}
	setup := byProfileMachine["codex-subscription:gpt-5.6-sol@needs-login"]
	if setup.ID == "" || setup.State != string(corecap.OfferSetupRequired) || setup.Reason != string(corecap.ReasonAuthRequired) {
		t.Fatalf("setup-required subscription profile = %+v", setup)
	}
	unavailable := byProfileMachine["codex-subscription:gpt-5.6-sol@offline"]
	if unavailable.ID == "" || unavailable.State != string(corecap.OfferUnavailable) || unavailable.Reason != string(corecap.ReasonProbeFailed) {
		t.Fatalf("unavailable subscription profile = %+v", unavailable)
	}
	conflict := byProfileMachine["codex-subscription:gpt-5.6-sol@conflict"]
	if conflict.ID == "" || conflict.State != string(corecap.OfferUnavailable) || conflict.Reason != string(corecap.ReasonCapabilityDrift) {
		t.Fatalf("conflicting text-only offers = %+v", conflict)
	}
	if _, err := svc.SetPrimaryAgent(context.Background(), ordinary.ID); ErrorCode(err) != ErrorPrimaryAgentUnready {
		t.Fatalf("ineligible selection error = %v (%q)", err, ErrorCode(err))
	}
	if _, err := st.GetPrimaryAgentSetting(); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("ineligible option persisted: %v", err)
	}
}

func TestPrimaryAgentOptionsGroupMachinesAndPresentReadyBeforeNonReadyInventory(t *testing.T) {
	st := openPrimaryStore(t)
	capabilities, readyOptionID := primaryCapability("zeta")
	capabilities.snapshot.Machines[0].Profiles = []corecap.ProfileOffer{
		{ID: "claude:sonnet", Agent: "claude", Adapter: "profile.claude.native", State: corecap.OfferReady},
		{ID: "codex:gpt-5.6-sol", Agent: "codex", Adapter: "profile.codex.native", State: corecap.OfferReady},
	}
	capabilities.snapshot.Machines = append(capabilities.snapshot.Machines,
		corecap.MachineInventory{
			Name: "alpha", Reachable: true, ProtocolVersion: corecap.ProtocolVersion,
			CatalogVersion: corecap.CatalogVersion, ProfileMappingVersion: corecap.ProfileMappingVersion,
			Profiles: []corecap.ProfileOffer{
				{ID: "claude:sonnet", Agent: "claude", Adapter: "profile.claude.native", State: corecap.OfferReady},
				{ID: "codex-subscription:gpt-5.6-sol", Agent: "codex-subscription", Adapter: "profile.codex-subscription.isolated", State: corecap.OfferSetupRequired, Reason: corecap.ReasonAuthRequired},
			}, TextOnlyOptions: []corecap.TextOnlyOptionOffer{},
		},
		corecap.MachineInventory{
			Name: "middle", Reachable: true, ProtocolVersion: corecap.ProtocolVersion,
			CatalogVersion: corecap.CatalogVersion, ProfileMappingVersion: corecap.ProfileMappingVersion,
			Profiles: []corecap.ProfileOffer{{
				ID: "codex-subscription:gpt-5.6-sol", Agent: "codex-subscription", Adapter: "profile.codex-subscription.isolated",
				State: corecap.OfferUnavailable, Reason: corecap.ReasonProbeFailed,
			}}, TextOnlyOptions: []corecap.TextOnlyOptionOffer{},
		},
	)

	view, err := NewPrimaryChannelService(st, nil, capabilities).PrimaryAgent(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	wantMachines := []string{"alpha", "alpha", "middle", "zeta", "zeta", "zeta"}
	gotMachines := make([]string, 0, len(view.Options))
	states := map[string]bool{}
	sawNonReady := map[string]bool{}
	for _, option := range view.Options {
		machine := option.Seat.Machine
		gotMachines = append(gotMachines, machine)
		states[option.State] = true
		if option.State == PrimaryAgentReady {
			if sawNonReady[machine] {
				t.Fatalf("ready option followed non-ready inventory on %q: %+v", machine, view.Options)
			}
		} else {
			sawNonReady[machine] = true
		}
	}
	if !reflect.DeepEqual(gotMachines, wantMachines) {
		t.Fatalf("primary option machines = %v, want grouped order %v", gotMachines, wantMachines)
	}
	for _, state := range []string{PrimaryAgentReady, string(corecap.OfferSetupRequired), string(corecap.OfferUnavailable), PrimaryAgentIneligible} {
		if !states[state] {
			t.Fatalf("primary options = %+v, missing state %q", view.Options, state)
		}
	}
	if firstOnZeta := view.Options[3]; firstOnZeta.ID != readyOptionID || firstOnZeta.State != PrimaryAgentReady {
		t.Fatalf("first zeta option = %+v, want ready choice %q", firstOnZeta, readyOptionID)
	}
}

func TestPrimaryChannelReadinessProjectsCurrentInventoryWithoutRetargetingIdentity(t *testing.T) {
	st := openPrimaryStore(t)
	capabilities, optionID := primaryCapability("studio")
	svc := NewPrimaryChannelService(st, nil, capabilities)
	if _, err := svc.SetPrimaryAgent(context.Background(), optionID); err != nil {
		t.Fatal(err)
	}
	created, err := svc.CreateChannel(context.Background(), "Frozen but observable")
	if err != nil {
		t.Fatal(err)
	}
	if created.Readiness.State != PrimaryAgentReady {
		t.Fatalf("initial readiness = %+v", created.Readiness)
	}
	wantParticipant := created.Participants[0]
	wantIdentity := *created.PrimaryChannel

	capabilities.mu.Lock()
	capabilities.snapshot.ObservedAt = time.Date(2026, 8, 8, 15, 0, 0, 0, time.UTC)
	capabilities.snapshot.Machines[0].TextOnlyOptions = []corecap.TextOnlyOptionOffer{}
	capabilities.snapshot.Machines[0].Profiles = []corecap.ProfileOffer{{
		ID: "codex-subscription:gpt-5.6-sol", Agent: "codex-subscription", Adapter: "profile.codex-subscription.isolated",
		State: corecap.OfferUnavailable, Reason: corecap.ReasonProbeFailed,
	}}
	capabilities.mu.Unlock()

	detail, err := svc.GetChannel(context.Background(), created.Conversation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Readiness.State != string(corecap.OfferUnavailable) || detail.Readiness.Reason != string(corecap.ReasonProbeFailed) ||
		!detail.Readiness.ObservedAt.Equal(capabilities.snapshot.ObservedAt) {
		t.Fatalf("current readiness = %+v", detail.Readiness)
	}
	if detail.Participants[0] != wantParticipant || *detail.PrimaryChannel != wantIdentity {
		t.Fatalf("readiness projection retargeted identity: participant=%+v primary=%+v", detail.Participants[0], detail.PrimaryChannel)
	}
}

func TestPrimaryChannelServiceCreatesServerSelectedTurnAndPersistsTypedReceipt(t *testing.T) {
	st := openPrimaryStore(t)
	capabilities, optionID := primaryCapability("studio")
	rt := &primaryRuntimeFixture{answer: "the answer"}
	svc := NewPrimaryChannelService(st, rt, capabilities)
	svc.now = func() time.Time { return time.Date(2026, 8, 8, 13, 0, 0, 0, time.UTC) }
	if _, err := svc.SetPrimaryAgent(context.Background(), optionID); err != nil {
		t.Fatal(err)
	}
	channel, err := svc.CreateChannel(context.Background(), "Private notes")
	if err != nil {
		t.Fatal(err)
	}
	if len(channel.Participants) != 1 || channel.PrimaryChannel == nil {
		t.Fatalf("channel = %+v", channel)
	}

	accepted, err := svc.PostTurn(context.Background(), channel.Conversation.ID, "client-turn-1", "hello exactly once")
	if err != nil || len(accepted.Targets) != 1 || accepted.Targets[0].Authority == nil {
		t.Fatalf("accepted = %+v, %v", accepted, err)
	}
	svc.Wait()
	detail, err := svc.GetChannel(context.Background(), channel.Conversation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Targets) != 1 || detail.Targets[0].State != conversation.TargetAnswered || detail.Targets[0].Receipt == nil ||
		detail.Targets[0].Receipt.ProviderThreadID != "thread-ephemeral" || len(detail.Messages) != 2 || detail.Messages[1].Body != "the answer" {
		t.Fatalf("terminal detail = %+v", detail)
	}
	specs := rt.captured()
	if len(specs) != 1 {
		t.Fatalf("dispatches = %d", len(specs))
	}
	spec := specs[0]
	if spec.Authority != coreruntime.AuthorityChatSubscriptionIsolatedV1 || spec.RuntimeContract != coreruntime.RuntimeContractCodexSubscriptionExecV1 ||
		spec.ExpectedPolicyRevision != strings.Repeat("1", 64) || spec.Workdir != "" || len(spec.Env) != 0 || spec.TextOnlyPolicy == nil ||
		spec.TextOnlyPolicy.SelectedCodexExecutableRevision != strings.Repeat("4", 64) || strings.Count(spec.Prompt, "hello exactly once") != 1 {
		t.Fatalf("run spec = %#v", spec)
	}
	if len(capabilities.machineRefreshes) != 2 {
		t.Fatalf("preflight count = %d, want create + dispatch", len(capabilities.machineRefreshes))
	}
}

func TestPrimaryChannelServiceRejectsLegacyConversationOnIdempotentTurnLookup(t *testing.T) {
	st := openPrimaryStore(t)
	now := time.Date(2026, 8, 8, 13, 30, 0, 0, time.UTC)
	legacy := conversation.Conversation{ID: "legacy", Title: "Legacy conversation", CreatedAt: now, UpdatedAt: now}
	participant := conversation.Participant{
		ID: "legacy-participant", ConversationID: legacy.ID, SeatID: "legacy-seat", Profile: "codex",
		Agent: "codex", Machine: "local", State: conversation.ParticipantActive, CreatedAt: now,
	}
	if err := st.CreateConversation(legacy, []conversation.Participant{participant}); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := st.CreateConversationTurn(store.CreateConversationTurnParams{
		TurnID: "legacy-turn", ClientTurnID: "shared-client-id", ConversationID: legacy.ID,
		HumanID: "human", Body: "legacy body", CreatedAt: now,
		Targets: []store.ConversationTurnTarget{{
			ID: "legacy-target", ParticipantID: participant.ID, RunID: "legacy-run",
		}},
	}); err != nil {
		t.Fatal(err)
	}

	svc := NewPrimaryChannelService(st, nil, nil)
	if result, err := svc.PostTurn(context.Background(), legacy.ID, "shared-client-id", "must not escape through Primary"); ErrorCode(err) != ErrorPrimaryChannelInvariant || result.Turn.ID != "" || len(result.Targets) != 0 {
		t.Fatalf("legacy idempotent lookup = %+v, %v (%q), want primary_channel_invariant", result, err, ErrorCode(err))
	}
}

func TestPrimaryChannelServiceSerializesConcurrentTurnsAcrossStoreConnections(t *testing.T) {
	setup := func(t *testing.T) (*PrimaryChannelService, *PrimaryChannelService, *primaryBlockingRuntime, string) {
		t.Helper()
		path := filepath.Join(t.TempDir(), "fort.db")
		firstStore, err := store.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = firstStore.Close() })
		capabilities, optionID := primaryCapability("studio")
		runtime := &primaryBlockingRuntime{}
		first := NewPrimaryChannelService(firstStore, runtime, capabilities)
		if _, err := first.SetPrimaryAgent(context.Background(), optionID); err != nil {
			t.Fatal(err)
		}
		channel, err := first.CreateChannel(context.Background(), "Concurrent")
		if err != nil {
			t.Fatal(err)
		}
		secondStore, err := store.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = secondStore.Close() })
		second := NewPrimaryChannelService(secondStore, runtime, capabilities)
		t.Cleanup(first.Close)
		t.Cleanup(second.Close)
		return first, second, runtime, channel.Conversation.ID
	}

	t.Run("same client turn returns the winner", func(t *testing.T) {
		first, second, runtime, channelID := setup(t)
		type outcome struct {
			result conversation.TurnResult
			err    error
		}
		start := make(chan struct{})
		outcomes := make(chan outcome, 2)
		var wait sync.WaitGroup
		for _, service := range []*PrimaryChannelService{first, second} {
			wait.Add(1)
			go func() {
				defer wait.Done()
				<-start
				result, err := service.PostTurn(context.Background(), channelID, "same-client", "exactly once")
				outcomes <- outcome{result: result, err: err}
			}()
		}
		close(start)
		wait.Wait()
		close(outcomes)
		turnID := ""
		for outcome := range outcomes {
			if outcome.err != nil {
				t.Fatalf("idempotent outcome: %v", outcome.err)
			}
			if turnID == "" {
				turnID = outcome.result.Turn.ID
			} else if outcome.result.Turn.ID != turnID {
				t.Fatalf("turn IDs differ: %q and %q", turnID, outcome.result.Turn.ID)
			}
		}
		deadline := time.Now().Add(time.Second)
		for {
			runtime.mu.Lock()
			dispatches := len(runtime.runs)
			runtime.mu.Unlock()
			if dispatches == 1 {
				break
			}
			if dispatches > 1 || time.Now().After(deadline) {
				t.Fatalf("dispatches = %d, want one", dispatches)
			}
			time.Sleep(time.Millisecond)
		}
		detail, err := first.GetChannel(context.Background(), channelID)
		if err != nil || len(detail.Messages) != 1 || len(detail.Turns) != 1 || len(detail.Targets) != 1 {
			t.Fatalf("idempotent detail = %+v, %v", detail, err)
		}
	})

	t.Run("different client turns permit one active target", func(t *testing.T) {
		first, second, runtime, channelID := setup(t)
		start := make(chan struct{})
		errorsOut := make(chan error, 2)
		var wait sync.WaitGroup
		for index, service := range []*PrimaryChannelService{first, second} {
			index, service := index, service
			wait.Add(1)
			go func() {
				defer wait.Done()
				<-start
				_, err := service.PostTurn(context.Background(), channelID, fmt.Sprintf("client-%d", index), fmt.Sprintf("body %d", index))
				errorsOut <- err
			}()
		}
		close(start)
		wait.Wait()
		close(errorsOut)
		successes, active := 0, 0
		for err := range errorsOut {
			switch {
			case err == nil:
				successes++
			case ErrorCode(err) == string(conversation.ErrorConversationActive) && errors.Is(err, conversation.ErrConversationActive):
				active++
			default:
				t.Fatalf("losing turn error = %v, want conversation_active", err)
			}
		}
		if successes != 1 || active != 1 {
			t.Fatalf("outcomes: success=%d conversation_active=%d", successes, active)
		}
		deadline := time.Now().Add(time.Second)
		for {
			runtime.mu.Lock()
			dispatches := len(runtime.runs)
			runtime.mu.Unlock()
			if dispatches == 1 {
				break
			}
			if dispatches > 1 || time.Now().After(deadline) {
				t.Fatalf("dispatches = %d, want one", dispatches)
			}
			time.Sleep(time.Millisecond)
		}
		detail, err := first.GetChannel(context.Background(), channelID)
		if err != nil || len(detail.Messages) != 1 || len(detail.Turns) != 1 || len(detail.Targets) != 1 {
			t.Fatalf("single-flight detail = %+v, %v", detail, err)
		}
	})
}

func TestPrimaryChannelServiceFailsClosedOnFreshDriftBeforeDurableTurn(t *testing.T) {
	st := openPrimaryStore(t)
	capabilities, optionID := primaryCapability("studio")
	rt := &primaryRuntimeFixture{answer: "must not run"}
	svc := NewPrimaryChannelService(st, rt, capabilities)
	if _, err := svc.SetPrimaryAgent(context.Background(), optionID); err != nil {
		t.Fatal(err)
	}
	channel, err := svc.CreateChannel(context.Background(), "Frozen identity")
	if err != nil {
		t.Fatal(err)
	}

	capabilities.mu.Lock()
	drifted := capabilities.snapshot.Machines[0].TextOnlyOptions[0]
	drifted.AccountPlan = "plus"
	capabilities.snapshot.Machines[0].TextOnlyOptions[0] = drifted
	capabilities.mu.Unlock()

	if _, err := svc.PostTurn(context.Background(), channel.Conversation.ID, "client-drift", "must remain atomic"); ErrorCode(err) != ErrorPrimaryAgentDrift {
		t.Fatalf("drift error = %v (%q)", err, ErrorCode(err))
	}
	detail, err := svc.GetChannel(context.Background(), channel.Conversation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Messages) != 0 || len(detail.Turns) != 0 || len(detail.Targets) != 0 || len(rt.captured()) != 0 {
		t.Fatalf("drift left work: detail=%+v dispatches=%d", detail, len(rt.captured()))
	}
}

func TestPrimaryChannelServiceRetriesOnlyLatestFailedAttemptWithFrozenAuthority(t *testing.T) {
	st := openPrimaryStore(t)
	capabilities, optionID := primaryCapability("studio")
	failing := &primaryFailureRuntime{code: ErrorProviderFailed}
	svc := NewPrimaryChannelService(st, failing, capabilities)
	svc.now = func() time.Time { return time.Date(2026, 8, 8, 14, 0, 0, 0, time.UTC) }
	if _, err := svc.SetPrimaryAgent(context.Background(), optionID); err != nil {
		t.Fatal(err)
	}
	channel, err := svc.CreateChannel(context.Background(), "Recovery")
	if err != nil {
		t.Fatal(err)
	}
	accepted, err := svc.PostTurn(context.Background(), channel.Conversation.ID, "client-fail", "keep this frozen")
	if err != nil {
		t.Fatal(err)
	}
	svc.Wait()
	needs, err := svc.NeedsYou(context.Background())
	if err != nil || len(needs) != 1 || needs[0].Target.ID != accepted.Targets[0].ID || !reflect.DeepEqual(needs[0].RecoveryActions, []string{"retry"}) {
		t.Fatalf("needs you = %+v, %v", needs, err)
	}
	failureDetail, _ := svc.GetChannel(context.Background(), channel.Conversation.ID)
	original := failureDetail.Targets[0]
	if original.Receipt == nil || original.Receipt.ProviderTerminalStatus != ErrorProviderFailed || original.Receipt.ObservedAdapterID != "unknown" {
		t.Fatalf("failed target receipt = %+v", original)
	}

	success := &primaryRuntimeFixture{answer: "recovered"}
	svc.runtime = success
	retry, err := svc.RetryTarget(context.Background(), channel.Conversation.ID, original.ID)
	if err != nil || retry.Attempt != 2 || retry.Authority == nil || !reflect.DeepEqual(retry.Authority, original.Authority) {
		t.Fatalf("retry = %+v, %v; original=%+v", retry, err, original)
	}
	svc.Wait()
	detail, _ := svc.GetChannel(context.Background(), channel.Conversation.ID)
	if len(detail.Messages) != 2 || detail.Messages[0].Body != "keep this frozen" || detail.Messages[1].Body != "recovered" {
		t.Fatalf("retry transcript = %+v", detail.Messages)
	}
	needs, err = svc.NeedsYou(context.Background())
	if err != nil || len(needs) != 0 {
		t.Fatalf("answered retry left Needs you = %+v, %v", needs, err)
	}
	if _, err := svc.RetryTarget(context.Background(), channel.Conversation.ID, original.ID); err == nil {
		t.Fatal("historical failed attempt was retried after a newer attempt")
	}
	specs := success.captured()
	if len(specs) != 1 || strings.Count(specs[0].Prompt, "keep this frozen") != 1 {
		t.Fatalf("retry prompt/specs = %#v", specs)
	}
}

func TestPrimaryInterruptedTargetOffersRetryRecovery(t *testing.T) {
	if got, want := recoveryActions("daemon_interrupted"), []string{"retry"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("daemon-interrupted recovery actions = %v, want %v", got, want)
	}
}

func TestPrimaryChannelServicePresentationMutationsStayInsideMarkedChannels(t *testing.T) {
	st := openPrimaryStore(t)
	capabilities, optionID := primaryCapability("studio")
	svc := NewPrimaryChannelService(st, nil, capabilities)
	if _, err := svc.SetPrimaryAgent(context.Background(), optionID); err != nil {
		t.Fatal(err)
	}
	channel, err := svc.CreateChannel(context.Background(), "Original")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.RenameChannel(context.Background(), channel.Conversation.ID, "Renamed"); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetChannelPinned(context.Background(), channel.Conversation.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetChannelState(context.Background(), channel.Conversation.ID, conversation.ConversationArchived); err != nil {
		t.Fatal(err)
	}
	archived, err := svc.ListChannels(context.Background(), "archived")
	if err != nil || len(archived) != 1 || archived[0].Conversation.Title != "Renamed" || !archived[0].Pinned {
		t.Fatalf("archived list = %+v, %v", archived, err)
	}
	if _, err := svc.PostTurn(context.Background(), channel.Conversation.ID, "archived-turn", "no"); err == nil {
		t.Fatal("archived Channel accepted a turn")
	}
}

func TestPrimaryChannelServiceCancelValidatesNestedChannelAndPersistsCanceledReceipt(t *testing.T) {
	st := openPrimaryStore(t)
	capabilities, optionID := primaryCapability("studio")
	rt := &primaryBlockingRuntime{}
	svc := NewPrimaryChannelService(st, rt, capabilities)
	if _, err := svc.SetPrimaryAgent(context.Background(), optionID); err != nil {
		t.Fatal(err)
	}
	channel, err := svc.CreateChannel(context.Background(), "Cancelable")
	if err != nil {
		t.Fatal(err)
	}
	other, err := svc.CreateChannel(context.Background(), "Other")
	if err != nil {
		t.Fatal(err)
	}
	accepted, err := svc.PostTurn(context.Background(), channel.Conversation.ID, "cancel-turn", "wait")
	if err != nil {
		t.Fatal(err)
	}
	targetID := accepted.Targets[0].ID
	deadline := time.Now().Add(2 * time.Second)
	for {
		detail, getErr := svc.GetChannel(context.Background(), channel.Conversation.ID)
		if getErr != nil {
			t.Fatal(getErr)
		}
		if len(detail.Targets) == 1 && detail.Targets[0].State == conversation.TargetWorking {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("target did not become working: %+v", detail.Targets)
		}
		time.Sleep(time.Millisecond)
	}
	if err := svc.CancelTarget(context.Background(), other.Conversation.ID, targetID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("foreign nested cancel error = %v, want not found", err)
	}
	if err := svc.CancelTarget(context.Background(), channel.Conversation.ID, targetID); err != nil {
		t.Fatal(err)
	}
	svc.Wait()
	detail, err := svc.GetChannel(context.Background(), channel.Conversation.ID)
	if err != nil {
		t.Fatal(err)
	}
	target := detail.Targets[0]
	if target.State != conversation.TargetCanceled || target.ErrorCode != "" || target.Receipt == nil ||
		target.Receipt.ProviderTerminalStatus != "canceled" || target.Receipt.ObservedCodexSchemaRevision != "unknown" || len(detail.Messages) != 1 {
		t.Fatalf("canceled detail = %+v", detail)
	}
	if err := svc.CancelTarget(context.Background(), channel.Conversation.ID, targetID); err == nil {
		t.Fatal("terminal canceled target was canceled twice")
	}
}

func TestLatestPrimaryTargetUsesDurableTurnOrderThenAttempt(t *testing.T) {
	detail := store.ConversationDetail{
		Turns: []conversation.Turn{{ID: "turn-old"}, {ID: "turn-new"}},
		Targets: []conversation.Target{
			{ID: "z-new-first", TurnID: "turn-new", Attempt: 1, State: conversation.TargetFailed},
			{ID: "old", TurnID: "turn-old", Attempt: 7, State: conversation.TargetFailed},
			{ID: "a-new-retry", TurnID: "turn-new", Attempt: 2, State: conversation.TargetAnswered},
		},
	}
	got, ok := latestPrimaryTarget(detail)
	if !ok || got.ID != "a-new-retry" {
		t.Fatalf("latest target = %+v, %v", got, ok)
	}
}

func TestPrimaryChannelServiceRejectsOutOfOrderNormalizedRuntimeEvents(t *testing.T) {
	st := openPrimaryStore(t)
	capabilities, optionID := primaryCapability("studio")
	rt := primaryEventRuntime{events: func(spec coreruntime.RunSpec) []coreruntime.RunEvent {
		policy := spec.TextOnlyPolicy
		metadata := &coreruntime.ResponseMetadata{
			ProviderThreadID: "thread", RequestedModel: spec.Model, ResolvedModel: "unknown",
			SelectedAdapterID: policy.SelectedAdapterID, SelectedAdapterRevision: policy.SelectedAdapterRevision,
			SelectedCodexVersion:            policy.SelectedCodexVersion,
			SelectedCodexExecutableRevision: policy.SelectedCodexExecutableRevision,
			SelectedCodexSchemaRevision:     policy.SelectedCodexSchemaRevision,
			ObservedAdapterID:               policy.SelectedAdapterID, ObservedAdapterRevision: policy.SelectedAdapterRevision,
			ObservedCodexVersion:            policy.SelectedCodexVersion,
			ObservedCodexExecutableRevision: policy.SelectedCodexExecutableRevision,
			ObservedCodexSchemaRevision:     policy.SelectedCodexSchemaRevision,
			TerminalStatus:                  "completed", UsageSource: "codex_exec_jsonl",
		}
		return []coreruntime.RunEvent{
			{RunID: spec.RunID, Type: coreruntime.EventMessage, Data: "must be discarded", Response: metadata},
			{RunID: spec.RunID, Type: coreruntime.EventStarted},
			{RunID: spec.RunID, Type: coreruntime.EventExited},
		}
	}}
	svc := NewPrimaryChannelService(st, rt, capabilities)
	if _, err := svc.SetPrimaryAgent(context.Background(), optionID); err != nil {
		t.Fatal(err)
	}
	channel, err := svc.CreateChannel(context.Background(), "Terminal shape")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.PostTurn(context.Background(), channel.Conversation.ID, "shape", "question"); err != nil {
		t.Fatal(err)
	}
	svc.Wait()
	detail, err := svc.GetChannel(context.Background(), channel.Conversation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Targets) != 1 || detail.Targets[0].State != conversation.TargetFailed ||
		detail.Targets[0].ErrorCode != ErrorProviderIncomplete || len(detail.Messages) != 1 {
		t.Fatalf("out-of-order terminal detail = %+v", detail)
	}
}
