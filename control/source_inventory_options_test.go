package control

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	corecap "github.com/tobsai/fort/core/capability"
	"github.com/tobsai/fort/core/conversation"
	coreruntime "github.com/tobsai/fort/core/runtime"
	"github.com/tobsai/fort/ui"
)

func TestSourceInventoryAgentOptionsProjectCurrentHermesProfilesAsUnavailable(t *testing.T) {
	now := time.Date(2026, 8, 22, 15, 0, 0, 0, time.UTC)
	snapshot := sourceInventorySnapshot(now,
		sourceInventoryAgent("source:mini:profile:alpha", "alpha", "Alpha", now),
		sourceInventoryAgent("source:mini:profile:beta", "beta", "Beta", now),
	)
	repository := &sourceInventoryRepositoryFake{latest: map[string]coreruntime.LatestAgentSourceInventory{
		"account:one\x00source:mini": {
			HasSnapshot: true, Snapshot: snapshot, Freshness: coreruntime.AgentSourceInventoryCurrent,
			LastAttemptAt: now, SourceLastSeenAt: now,
		},
	}}
	source, err := NewSourceInventoryAgentOptionSource(repository, []SourceInventoryRegistration{{
		AccountID: "account:one", ExecutionSourceID: "source:mini",
	}})
	if err != nil {
		t.Fatal(err)
	}

	got, err := source.AgentOptions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("Agent options = %+v, want two Hermes profiles", got)
	}
	if got[0].ID != "source:mini:profile:alpha" || got[1].ID != "source:mini:profile:beta" {
		t.Fatalf("source-qualified option IDs = %q, %q", got[0].ID, got[1].ID)
	}
	for index, option := range got {
		if option.State != "unavailable" || option.Reason != "execution_adapter_not_approved" {
			t.Fatalf("option %d state = (%q, %q), want unavailable execution gate", index, option.State, option.Reason)
		}
		if option.Binding.Seat.ID != option.ID || option.Binding.Seat.Agent != "hermes" || option.Binding.Seat.Machine != "Hermes · Mac mini" {
			t.Fatalf("option %d discovery identity = %+v", index, option.Binding.Seat)
		}
		if option.Binding.Seat.Profile != "hermes:"+snapshot.Agents[index].SourceAgent.OpaqueSourceAgentID {
			t.Fatalf("option %d profile = %q, want framework-qualified discovery identity", index, option.Binding.Seat.Profile)
		}
		if option.Binding.Seat.Model != "unknown" || option.Binding.Authority.RequestedModel != "unknown" ||
			option.Binding.Authority.ResolvedModel != "unknown" {
			t.Fatalf("option %d invented provider/model identity: %+v", index, option.Binding)
		}
		if option.Binding.Authority.AdapterID != "unapproved" || option.Binding.Authority.ExecutionPolicy == nil ||
			len(option.Binding.Authority.ExecutionPolicy) != 0 {
			t.Fatalf("option %d execution placeholder = %+v", index, option.Binding.Authority)
		}
		if err := option.Binding.Validate(); err == nil {
			t.Fatalf("option %d discovery-only binding unexpectedly validates for execution", index)
		}
	}
}

func TestSourceInventoryRecheckPreservesLastRosterAsStaleAfterClosedFailure(t *testing.T) {
	lastSeen := time.Date(2026, 8, 22, 15, 0, 0, 0, time.UTC)
	failureAt := lastSeen.Add(5 * time.Minute)
	snapshot := sourceInventorySnapshot(lastSeen, sourceInventoryAgent("source:mini:profile:alpha", "alpha", "Alpha", lastSeen))
	repository := &sourceInventoryRepositoryFake{latest: map[string]coreruntime.LatestAgentSourceInventory{
		"account:one\x00source:mini": {
			HasSnapshot: true, Snapshot: snapshot, Freshness: coreruntime.AgentSourceInventoryCurrent,
			LastAttemptAt: lastSeen, SourceLastSeenAt: lastSeen,
		},
	}}
	inventory := &sourceInventoryFake{err: errors.New("private upstream detail must not persist")}
	source, err := NewSourceInventoryAgentOptionSource(repository, []SourceInventoryRegistration{{
		AccountID: "account:one", ExecutionSourceID: "source:mini", Inventory: inventory,
	}})
	if err != nil {
		t.Fatal(err)
	}
	source.now = func() time.Time { return failureAt }

	got, err := source.RecheckAgentOptions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(inventory.requests, []coreruntime.AgentSourceInventoryRequest{{ExecutionSourceID: "source:mini"}}) {
		t.Fatalf("inventory requests = %+v", inventory.requests)
	}
	wantFailure := coreruntime.AgentSourceInventoryFailure{
		AccountID: "account:one", ExecutionSourceID: "source:mini", ObservedAt: failureAt,
		Reason: coreruntime.AgentSourceInventoryUnavailable,
	}
	if !reflect.DeepEqual(repository.failures, []coreruntime.AgentSourceInventoryFailure{wantFailure}) {
		t.Fatalf("closed failures = %+v, want %+v", repository.failures, wantFailure)
	}
	if len(repository.success) != 0 {
		t.Fatalf("failed recheck appended successes = %+v", repository.success)
	}
	if len(got) != 1 || got[0].ID != "source:mini:profile:alpha" ||
		got[0].State != string(corecap.OfferUnavailable) || got[0].Reason != string(coreruntime.AgentSourceInventoryUnavailable) {
		t.Fatalf("stale Agent options = %+v", got)
	}
	latest := repository.latest["account:one\x00source:mini"]
	if !latest.HasSnapshot || latest.Snapshot.ObservedAt != lastSeen || latest.SourceLastSeenAt != lastSeen || latest.FailureObservedAt != failureAt {
		t.Fatalf("latest state failed to keep snapshot, source last-seen, and failure distinct: %+v", latest)
	}
}

func TestSourceInventoryRecheckDoesNotRecordCallerCancellationAsSourceFailure(t *testing.T) {
	now := time.Date(2026, 8, 22, 15, 10, 0, 0, time.UTC)
	repository := &sourceInventoryRepositoryFake{latest: make(map[string]coreruntime.LatestAgentSourceInventory)}
	inventory := &sourceInventoryFake{err: context.Canceled}
	source, err := NewSourceInventoryAgentOptionSource(repository, []SourceInventoryRegistration{{
		AccountID: "account:one", ExecutionSourceID: "source:mini", Inventory: inventory,
	}})
	if err != nil {
		t.Fatal(err)
	}
	source.now = func() time.Time { return now }
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if got, err := source.RecheckAgentOptions(ctx); !errors.Is(err, context.Canceled) || got != nil {
		t.Fatalf("canceled source recheck = %+v, err = %v", got, err)
	}
	if len(repository.success) != 0 || len(repository.failures) != 0 {
		t.Fatalf("caller cancellation persisted success=%+v failure=%+v", repository.success, repository.failures)
	}
}

func TestCompositeAgentOptionSourceMergesDeterministicallyAndRejectsDuplicateIDs(t *testing.T) {
	ready := uiAgentOption("primary:z", PrimaryAgentReady, "Codex")
	unavailable := uiAgentOption("source:mini:profile:alpha", string(corecap.OfferUnavailable), "Alpha")
	secondaryUnavailable := uiAgentOption("primary:a", string(corecap.OfferUnavailable), "Claude")
	source := NewCompositeAgentOptionSource(
		fixedAgentOptionSource{items: []ui.AgentOption{secondaryUnavailable, ready}},
		fixedAgentOptionSource{items: []ui.AgentOption{unavailable}},
	)

	got, err := source.AgentOptions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	wantIDs := []string{"primary:z", "primary:a", "source:mini:profile:alpha"}
	if gotIDs := agentOptionIDs(got); !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Fatalf("merged option order = %v, want ready first then stable IDs %v", gotIDs, wantIDs)
	}

	duplicate := NewCompositeAgentOptionSource(
		fixedAgentOptionSource{items: []ui.AgentOption{ready}},
		fixedAgentOptionSource{items: []ui.AgentOption{ready}},
	)
	if _, err := duplicate.AgentOptions(context.Background()); err == nil {
		t.Fatal("duplicate option IDs were accepted")
	}
}

func TestAgentChannelServiceComposesPrimaryAndSourceInventoryOptions(t *testing.T) {
	now := time.Date(2026, 8, 22, 15, 30, 0, 0, time.UTC)
	snapshot := sourceInventorySnapshot(now,
		sourceInventoryAgent("source:mini:profile:alpha", "alpha", "Alpha", now),
	)
	repository := &sourceInventoryRepositoryFake{latest: map[string]coreruntime.LatestAgentSourceInventory{
		"account:one\x00source:mini": {
			HasSnapshot: true, Snapshot: snapshot, Freshness: coreruntime.AgentSourceInventoryCurrent,
			LastAttemptAt: now, SourceLastSeenAt: now,
		},
	}}
	source, err := NewSourceInventoryAgentOptionSource(repository, []SourceInventoryRegistration{{
		AccountID: "account:one", ExecutionSourceID: "source:mini",
	}})
	if err != nil {
		t.Fatal(err)
	}
	store := openPrimaryStore(t)
	capabilities, primaryOptionID := primaryCapability("studio")
	primary := NewPrimaryChannelService(store, nil, capabilities)
	service := NewAgentChannelService(store, primary, source)

	got, err := service.AgentOptions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	wantIDs := []string{primaryOptionID, "source:mini:profile:alpha"}
	if ids := agentOptionIDs(got); !reflect.DeepEqual(ids, wantIDs) {
		t.Fatalf("Agent Channel service options = %v, want Primary plus source inventory %v", ids, wantIDs)
	}
}

func TestSourceInventoryRecheckAppendsSuccessAndProjectsCurrentRoster(t *testing.T) {
	observedAt := time.Date(2026, 8, 22, 16, 0, 0, 0, time.UTC)
	snapshot := sourceInventorySnapshot(observedAt, sourceInventoryAgent("source:mini:profile:beta", "beta", "Beta", observedAt))
	repository := &sourceInventoryRepositoryFake{latest: make(map[string]coreruntime.LatestAgentSourceInventory)}
	inventory := &sourceInventoryFake{snapshot: snapshot}
	source, err := NewSourceInventoryAgentOptionSource(repository, []SourceInventoryRegistration{{
		AccountID: "account:one", ExecutionSourceID: "source:mini", Inventory: inventory,
	}})
	if err != nil {
		t.Fatal(err)
	}

	got, err := source.RecheckAgentOptions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(repository.success, []coreruntime.AgentSourceInventorySnapshot{snapshot}) || len(repository.failures) != 0 {
		t.Fatalf("successful observations = %+v, failures = %+v", repository.success, repository.failures)
	}
	if len(got) != 1 || got[0].ID != "source:mini:profile:beta" || got[0].Reason != "execution_adapter_not_approved" {
		t.Fatalf("current rechecked options = %+v", got)
	}
}

func TestSourceInventoryRecheckRejectsSnapshotOutsideRegisteredAccountAndSource(t *testing.T) {
	now := time.Date(2026, 8, 22, 16, 2, 0, 0, time.UTC)
	for _, test := range []struct {
		name   string
		mutate func(*coreruntime.AgentSourceInventorySnapshot)
	}{
		{name: "other account", mutate: func(snapshot *coreruntime.AgentSourceInventorySnapshot) {
			snapshot.ExecutionSource.AccountID = "account:other"
		}},
		{name: "other Execution Source", mutate: func(snapshot *coreruntime.AgentSourceInventorySnapshot) {
			snapshot.ExecutionSource.ID = "source:other"
			snapshot.Agents[0].SourceAgent.ExecutionSourceID = "source:other"
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			snapshot := sourceInventorySnapshot(now,
				sourceInventoryAgent("source:mini:profile:alpha", "alpha", "Alpha", now),
			)
			test.mutate(&snapshot)
			repository := &sourceInventoryRepositoryFake{latest: make(map[string]coreruntime.LatestAgentSourceInventory)}
			inventory := &sourceInventoryFake{snapshot: snapshot}
			source, err := NewSourceInventoryAgentOptionSource(repository, []SourceInventoryRegistration{{
				AccountID: "account:one", ExecutionSourceID: "source:mini", Inventory: inventory,
			}})
			if err != nil {
				t.Fatal(err)
			}

			if got, err := source.RecheckAgentOptions(context.Background()); err == nil || got != nil {
				t.Fatalf("out-of-scope inventory recheck = %+v, err = %v", got, err)
			}
			if len(repository.success) != 0 || len(repository.failures) != 0 {
				t.Fatalf("out-of-scope inventory persisted success=%+v failure=%+v", repository.success, repository.failures)
			}
		})
	}
}

func TestSourceInventoryAgentOptionsFailClosedOnExecutionReadySnapshot(t *testing.T) {
	now := time.Date(2026, 8, 22, 16, 5, 0, 0, time.UTC)
	agent := sourceInventoryAgent("source:mini:profile:alpha", "alpha", "Alpha", now)
	agent.Capabilities = []string{"chat"}
	agent.Readiness.Ready = true
	snapshot := sourceInventorySnapshot(now, agent)
	repository := &sourceInventoryRepositoryFake{latest: map[string]coreruntime.LatestAgentSourceInventory{
		"account:one\x00source:mini": {
			HasSnapshot: true, Snapshot: snapshot, Freshness: coreruntime.AgentSourceInventoryCurrent,
			LastAttemptAt: now, SourceLastSeenAt: now,
		},
	}}
	source, err := NewSourceInventoryAgentOptionSource(repository, []SourceInventoryRegistration{{
		AccountID: "account:one", ExecutionSourceID: "source:mini",
	}})
	if err != nil {
		t.Fatal(err)
	}

	if got, err := source.AgentOptions(context.Background()); err == nil || got != nil {
		t.Fatalf("execution-ready discovery projected options = %+v, err = %v", got, err)
	}
}

func TestSourceInventoryAgentOptionsRejectDuplicateOpaqueOptionIDs(t *testing.T) {
	now := time.Date(2026, 8, 22, 16, 7, 0, 0, time.UTC)
	snapshot := sourceInventorySnapshot(now,
		sourceInventoryAgent("source:mini:duplicate", "alpha", "Alpha", now),
		sourceInventoryAgent("source:mini:duplicate", "beta", "Beta", now),
	)
	repository := &sourceInventoryRepositoryFake{latest: map[string]coreruntime.LatestAgentSourceInventory{
		"account:one\x00source:mini": {
			HasSnapshot: true, Snapshot: snapshot, Freshness: coreruntime.AgentSourceInventoryCurrent,
			LastAttemptAt: now, SourceLastSeenAt: now,
		},
	}}
	source, err := NewSourceInventoryAgentOptionSource(repository, []SourceInventoryRegistration{{
		AccountID: "account:one", ExecutionSourceID: "source:mini",
	}})
	if err != nil {
		t.Fatal(err)
	}

	if got, err := source.AgentOptions(context.Background()); err == nil || got != nil {
		t.Fatalf("duplicate source option IDs projected = %+v, err = %v", got, err)
	}
}

func TestSourceInventoryAgentOptionsRejectInconsistentStaleEvidence(t *testing.T) {
	now := time.Date(2026, 8, 22, 16, 8, 0, 0, time.UTC)
	snapshot := sourceInventorySnapshot(now,
		sourceInventoryAgent("source:mini:profile:alpha", "alpha", "Alpha", now),
	)
	repository := &sourceInventoryRepositoryFake{latest: map[string]coreruntime.LatestAgentSourceInventory{
		"account:one\x00source:mini": {
			HasSnapshot: true, Snapshot: snapshot, Freshness: coreruntime.AgentSourceInventoryStale,
			LastAttemptAt: now.Add(time.Minute), SourceLastSeenAt: now,
		},
	}}
	source, err := NewSourceInventoryAgentOptionSource(repository, []SourceInventoryRegistration{{
		AccountID: "account:one", ExecutionSourceID: "source:mini",
	}})
	if err != nil {
		t.Fatal(err)
	}

	if got, err := source.AgentOptions(context.Background()); err == nil || got != nil {
		t.Fatalf("inconsistent stale inventory projected = %+v, err = %v", got, err)
	}
}

func TestAgentChannelServiceCannotCreateFromUnavailableSourceInventoryRow(t *testing.T) {
	now := time.Date(2026, 8, 22, 16, 10, 0, 0, time.UTC)
	snapshot := sourceInventorySnapshot(now, sourceInventoryAgent("source:mini:profile:alpha", "alpha", "Alpha", now))
	repository := &sourceInventoryRepositoryFake{latest: map[string]coreruntime.LatestAgentSourceInventory{
		"account:one\x00source:mini": {
			HasSnapshot: true, Snapshot: snapshot, Freshness: coreruntime.AgentSourceInventoryCurrent,
			LastAttemptAt: now, SourceLastSeenAt: now,
		},
	}}
	optionSource, err := NewSourceInventoryAgentOptionSource(repository, []SourceInventoryRegistration{{
		AccountID: "account:one", ExecutionSourceID: "source:mini",
	}})
	if err != nil {
		t.Fatal(err)
	}
	service := NewAgentChannelService(openPrimaryStore(t), nil, optionSource)

	if _, err := service.CreateAgentChannel(context.Background(), "source:mini:profile:alpha", "Alpha"); ErrorCode(err) != ErrorPrimaryAgentUnready {
		t.Fatalf("unavailable Hermes row create error = %v (%q)", err, ErrorCode(err))
	}
	channels, err := service.ListAgentChannels(context.Background(), "all")
	if err != nil {
		t.Fatal(err)
	}
	if len(channels) != 0 {
		t.Fatalf("unavailable row created Agent Channels = %+v", channels)
	}
}

type sourceInventoryRepositoryFake struct {
	latest   map[string]coreruntime.LatestAgentSourceInventory
	success  []coreruntime.AgentSourceInventorySnapshot
	failures []coreruntime.AgentSourceInventoryFailure
}

type sourceInventoryFake struct {
	snapshot coreruntime.AgentSourceInventorySnapshot
	err      error
	requests []coreruntime.AgentSourceInventoryRequest
}

func (f *sourceInventoryFake) Inventory(_ context.Context, request coreruntime.AgentSourceInventoryRequest) (coreruntime.AgentSourceInventorySnapshot, error) {
	f.requests = append(f.requests, request)
	return f.snapshot, f.err
}

func (f *sourceInventoryRepositoryFake) AppendAgentSourceInventorySuccess(_ context.Context, snapshot coreruntime.AgentSourceInventorySnapshot) error {
	f.success = append(f.success, snapshot)
	f.latest[snapshot.ExecutionSource.AccountID+"\x00"+snapshot.ExecutionSource.ID] = coreruntime.LatestAgentSourceInventory{
		HasSnapshot: true, Snapshot: snapshot, Freshness: coreruntime.AgentSourceInventoryCurrent,
		LastAttemptAt: snapshot.ObservedAt, SourceLastSeenAt: snapshot.ExecutionSource.LastSeenAt,
	}
	return nil
}

func (f *sourceInventoryRepositoryFake) AppendAgentSourceInventoryFailure(_ context.Context, failure coreruntime.AgentSourceInventoryFailure) error {
	f.failures = append(f.failures, failure)
	key := failure.AccountID + "\x00" + failure.ExecutionSourceID
	latest := f.latest[key]
	latest.Freshness = coreruntime.AgentSourceInventoryStale
	latest.LastAttemptAt = failure.ObservedAt
	latest.FailureReason = failure.Reason
	latest.FailureObservedAt = failure.ObservedAt
	f.latest[key] = latest
	return nil
}

func (f *sourceInventoryRepositoryFake) LatestAgentSourceInventory(_ context.Context, accountID, executionSourceID string) (coreruntime.LatestAgentSourceInventory, error) {
	return f.latest[accountID+"\x00"+executionSourceID], nil
}

func sourceInventorySnapshot(observedAt time.Time, agents ...coreruntime.SourceAgentInventory) coreruntime.AgentSourceInventorySnapshot {
	return coreruntime.AgentSourceInventorySnapshot{
		ExecutionSource: conversation.ExecutionSource{
			ID: "source:mini", AccountID: "account:one", Framework: "hermes",
			InstanceID: "instance:mini", GatewayID: "gateway:mini", DisplayName: "Hermes · Mac mini",
			ResourceSharing: conversation.ResourceSharingDisclosure{
				ProviderCredentials: conversation.ResourceUnknown,
				Filesystem:          conversation.ResourceMachineShared, BrowserSessions: conversation.ResourceMachineShared,
				FrameworkSessions: conversation.ResourceProfileScoped, SourceMemory: conversation.ResourceUnknown,
				ToolConfiguration: conversation.ResourceProfileScoped,
			},
			LastSeenAt: observedAt,
		},
		Agents: agents, ObservedAt: observedAt,
	}
}

func sourceInventoryAgent(id, profile, display string, observedAt time.Time) coreruntime.SourceAgentInventory {
	return coreruntime.SourceAgentInventory{
		SourceAgent: conversation.SourceAgent{
			ID: id, ExecutionSourceID: "source:mini", OpaqueSourceAgentID: profile,
			DisplayName: display, LastSeenAt: observedAt,
		},
		Capabilities: []string{},
		Readiness: coreruntime.SourceAgentReadiness{
			Ready: false, ContractID: "source-agent.inventory.hermes-bot.v1",
			ContractRevision: "inventory-revision", Evidence: []string{"profile_discovered", "execution_adapter_not_approved"},
		},
	}
}

func uiAgentOption(id, state, displayName string) ui.AgentOption {
	return ui.AgentOption{ID: id, State: state, DisplayName: displayName}
}

func agentOptionIDs(options []ui.AgentOption) []string {
	ids := make([]string, len(options))
	for index, option := range options {
		ids[index] = option.ID
	}
	return ids
}

var _ coreruntime.AgentSourceInventoryRepository = (*sourceInventoryRepositoryFake)(nil)
