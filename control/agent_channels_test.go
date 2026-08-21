package control

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	corecap "github.com/tobsai/fort/core/capability"
	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/ui"
)

type fixedAgentOptionSource struct {
	items []ui.AgentOption
}

func (s fixedAgentOptionSource) AgentOptions(context.Context) ([]ui.AgentOption, error) {
	return append([]ui.AgentOption(nil), s.items...), nil
}

func (s fixedAgentOptionSource) RecheckAgentOptions(context.Context) ([]ui.AgentOption, error) {
	return append([]ui.AgentOption(nil), s.items...), nil
}

func TestAgentChannelServiceCreatesAgentDestinationFromOpaqueReadyOption(t *testing.T) {
	st := openPrimaryStore(t)
	capabilities, optionID := primaryCapability("studio")
	primary := NewPrimaryChannelService(st, nil, capabilities)
	svc := NewAgentChannelService(st, primary, nil)
	svc.now = func() time.Time { return time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC) }

	options, err := svc.AgentOptions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(options) != 1 || options[0].ID != optionID || options[0].State != PrimaryAgentReady {
		t.Fatalf("Agent options = %+v", options)
	}

	created, err := svc.CreateAgentChannel(context.Background(), optionID, "Codex — Studio")
	if err != nil {
		t.Fatal(err)
	}
	capabilities.mu.Lock()
	createRechecks := len(capabilities.refreshes)
	capabilities.mu.Unlock()
	if createRechecks != 1 {
		t.Fatalf("Agent Channel create readiness rechecks = %d, want 1", createRechecks)
	}
	if created.Channel.Name != "Codex — Studio" || created.Channel.State != conversation.AgentChannelOpen {
		t.Fatalf("created Agent Channel = %+v", created.Channel)
	}
	if created.Channel.Binding.Seat.Agent != "codex-subscription" || created.Channel.Binding.Seat.Machine != "studio" {
		t.Fatalf("created immutable binding = %+v", created.Channel.Binding)
	}
	if len(created.Conversations) != 0 {
		t.Fatalf("new Agent Channel conversations = %+v", created.Conversations)
	}

	listed, err := svc.ListAgentChannels(context.Background(), string(conversation.AgentChannelOpen))
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].Channel.ID != created.Channel.ID {
		t.Fatalf("listed Agent Channels = %+v", listed)
	}
	if listed[0].Channel.Binding.Seat.ID != options[0].Binding.Seat.ID {
		t.Fatalf("listed binding = %+v, option = %+v", listed[0].Channel.Binding, options[0].Binding)
	}
}

func TestAgentChannelServiceKeepsProviderNeutralAttributionExact(t *testing.T) {
	st := openPrimaryStore(t)
	for _, agent := range []string{"claude", "hermes", "openclaw"} {
		binding := conversation.AgentBinding{
			Seat: conversation.AgentSeatIdentity{
				ID: "seat-" + agent, Profile: agent + ":personal", Agent: agent,
				Model: agent + "-exact-model", Machine: "studio",
			},
			Authority: conversation.AgentAuthoritySnapshot{
				RequestedModel: agent + "-exact-model", ResolvedModel: "unknown",
				Authority: "test-authority-" + agent, PolicyID: "test-policy-" + agent,
				PolicyRevision: "policy-revision-" + agent, AdapterID: "adapter-" + agent,
				AdapterRevision: "adapter-revision-" + agent, RuntimeContract: "runtime-" + agent,
				SessionMode: "ephemeral", MemoryMode: "agent_managed",
				ExecutionPolicy: map[string]string{"test_contract": agent},
			},
		}
		option := ui.AgentOption{
			ID: "option-" + agent, State: PrimaryAgentReady, DisplayName: agent,
			Binding: binding,
		}
		svc := NewAgentChannelService(st, nil, fixedAgentOptionSource{items: []ui.AgentOption{option}})
		svc.now = func() time.Time { return time.Date(2026, 8, 20, 13, 0, 0, 0, time.UTC) }

		created, err := svc.CreateAgentChannel(context.Background(), option.ID, agent+" personal")
		if err != nil {
			t.Fatalf("create %s: %v", agent, err)
		}
		if created.Channel.Binding.Seat.Agent != agent || created.Channel.Binding.Authority.AdapterID != "adapter-"+agent {
			t.Fatalf("%s attribution = %+v", agent, created.Channel.Binding)
		}
	}
}

func TestAgentChannelReadinessReportsSameSeatAuthorityDriftWithoutRetargeting(t *testing.T) {
	st := openPrimaryStore(t)
	capabilities, optionID := primaryCapability("studio")
	primary := NewPrimaryChannelService(st, nil, capabilities)
	svc := NewAgentChannelService(st, primary, nil)
	created, err := svc.CreateAgentChannel(context.Background(), optionID, "Codex — Studio")
	if err != nil {
		t.Fatal(err)
	}
	wantBinding := created.Channel.Binding

	capabilities.mu.Lock()
	capabilities.snapshot.Machines[0].TextOnlyOptions[0].AdapterRevision = strings.Repeat("9", 64)
	capabilities.mu.Unlock()
	drifted, err := svc.GetAgentChannel(context.Background(), created.Channel.ID)
	if err != nil {
		t.Fatal(err)
	}
	if drifted.Readiness.State != PrimaryAgentDrifted || drifted.Readiness.Reason != ErrorPrimaryAgentDrift {
		t.Fatalf("drifted readiness = %+v", drifted.Readiness)
	}
	if drifted.Channel.Binding.Seat != wantBinding.Seat || drifted.Channel.Binding.Authority.AdapterRevision != wantBinding.Authority.AdapterRevision {
		t.Fatalf("stored binding changed under readiness drift: got %+v want %+v", drifted.Channel.Binding, wantBinding)
	}
}

func TestAgentChannelExistingConversationRejectsAdapterRevisionDriftBeforeProviderStart(t *testing.T) {
	st := openPrimaryStore(t)
	capabilities, optionID := primaryCapability("studio")
	runtime := &primaryRuntimeFixture{answer: "must not run"}
	primary := NewPrimaryChannelService(st, runtime, capabilities)
	svc := NewAgentChannelService(st, primary, nil)

	channel, err := svc.CreateAgentChannel(context.Background(), optionID, "Codex — Studio")
	if err != nil {
		t.Fatal(err)
	}
	child, err := svc.CreateAgentConversation(context.Background(), channel.Channel.ID, "Exact adapter")
	if err != nil {
		t.Fatal(err)
	}
	boundRevision := channel.Channel.Binding.Authority.AdapterRevision

	capabilities.mu.Lock()
	capabilities.snapshot.Machines[0].TextOnlyOptions[0].AdapterRevision = strings.Repeat("9", 64)
	capabilities.mu.Unlock()

	_, err = svc.PostAgentTurn(context.Background(), channel.Channel.ID, child.Conversation.ID, uuid.NewString(), "do not substitute")
	if ErrorCode(err) != ErrorPrimaryAgentDrift {
		t.Fatalf("adapter revision drift error = %v (%q), want %q", err, ErrorCode(err), ErrorPrimaryAgentDrift)
	}
	primary.Wait()
	if got := runtime.captured(); len(got) != 0 {
		t.Fatalf("provider starts after adapter revision drift = %+v", got)
	}
	detail, detailErr := svc.GetAgentConversation(context.Background(), channel.Channel.ID, child.Conversation.ID)
	if detailErr != nil {
		t.Fatal(detailErr)
	}
	if detail.Readiness.State != PrimaryAgentDrifted || detail.Readiness.Reason != ErrorPrimaryAgentDrift {
		t.Fatalf("nested Conversation readiness = %+v, want exact Agent binding drift", detail.Readiness)
	}
	if len(detail.Targets) != 0 || detail.Binding.Authority.AdapterRevision != boundRevision {
		t.Fatalf("drift changed durable Conversation identity or targets: %+v", detail)
	}
}

func TestAgentConversationSendLosesRaceToArchiveAtDurableBoundary(t *testing.T) {
	for _, archive := range []struct {
		name string
		run  func(*AgentChannelService, string, string) error
	}{
		{
			name: "parent Agent Channel",
			run: func(svc *AgentChannelService, channelID, _ string) error {
				return svc.SetAgentChannelState(context.Background(), channelID, conversation.AgentChannelArchived)
			},
		},
		{
			name: "child Conversation",
			run: func(svc *AgentChannelService, channelID, conversationID string) error {
				return svc.SetAgentConversationState(context.Background(), channelID, conversationID, conversation.ConversationArchived)
			},
		},
	} {
		t.Run(archive.name, func(t *testing.T) {
			st := openPrimaryStore(t)
			capabilities, optionID := primaryCapability("studio")
			runtime := &primaryRuntimeFixture{answer: "must not run"}
			primary := NewPrimaryChannelService(st, runtime, capabilities)
			svc := NewAgentChannelService(st, primary, nil)
			channel, err := svc.CreateAgentChannel(context.Background(), optionID, "Codex — Studio")
			if err != nil {
				t.Fatal(err)
			}
			child, err := svc.CreateAgentConversation(context.Background(), channel.Channel.ID, "Archive race")
			if err != nil {
				t.Fatal(err)
			}

			checked := make(chan struct{})
			continueSend := make(chan struct{})
			primary.now = func() time.Time {
				close(checked)
				<-continueSend
				return time.Date(2026, 8, 20, 14, 45, 0, 0, time.UTC)
			}
			result := make(chan error, 1)
			go func() {
				_, sendErr := svc.PostAgentTurn(context.Background(), channel.Channel.ID, child.Conversation.ID, uuid.NewString(), "do not commit")
				result <- sendErr
			}()
			select {
			case <-checked:
			case <-time.After(5 * time.Second):
				t.Fatal("PostAgentTurn did not reach the pre-write boundary")
			}
			if err := archive.run(svc, channel.Channel.ID, child.Conversation.ID); err != nil {
				t.Fatal(err)
			}
			close(continueSend)
			select {
			case err := <-result:
				if ErrorCode(err) != ErrorAgentChannelState {
					t.Fatalf("concurrent archive error = %v (%q), want %q", err, ErrorCode(err), ErrorAgentChannelState)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("PostAgentTurn did not finish after concurrent archive")
			}
			primary.Wait()
			if got := runtime.captured(); len(got) != 0 {
				t.Fatalf("provider starts after concurrent archive = %+v", got)
			}
			detail, detailErr := st.GetConversation(child.Conversation.ID)
			if detailErr != nil {
				t.Fatal(detailErr)
			}
			if len(detail.Turns) != 0 || len(detail.Targets) != 0 || len(detail.Messages) != 0 {
				t.Fatalf("concurrent archive committed turn state = %+v", detail)
			}
		})
	}
}

func TestAgentChannelRetryRejectsAdapterRevisionDriftBeforeProviderStart(t *testing.T) {
	st := openPrimaryStore(t)
	capabilities, optionID := primaryCapability("studio")
	runtime := &primaryFailureRuntime{code: ErrorProviderFailed}
	primary := NewPrimaryChannelService(st, runtime, capabilities)
	svc := NewAgentChannelService(st, primary, nil)

	channel, err := svc.CreateAgentChannel(context.Background(), optionID, "Codex — Studio")
	if err != nil {
		t.Fatal(err)
	}
	child, err := svc.CreateAgentConversation(context.Background(), channel.Channel.ID, "Exact retry adapter")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.PostAgentTurn(context.Background(), channel.Channel.ID, child.Conversation.ID, uuid.NewString(), "fail once"); err != nil {
		t.Fatal(err)
	}
	primary.Wait()
	detail, err := svc.GetAgentConversation(context.Background(), channel.Channel.ID, child.Conversation.ID)
	if err != nil || len(detail.Targets) != 1 || detail.Targets[0].State != conversation.TargetFailed {
		t.Fatalf("failed target = %+v, %v", detail.Targets, err)
	}

	capabilities.mu.Lock()
	capabilities.snapshot.Machines[0].TextOnlyOptions[0].AdapterRevision = strings.Repeat("9", 64)
	capabilities.mu.Unlock()

	_, err = svc.RetryAgentTarget(context.Background(), channel.Channel.ID, child.Conversation.ID, detail.Targets[0].ID)
	if ErrorCode(err) != ErrorPrimaryAgentDrift {
		t.Fatalf("retry adapter revision drift error = %v (%q), want %q", err, ErrorCode(err), ErrorPrimaryAgentDrift)
	}
	primary.Wait()
	runtime.mu.Lock()
	dispatches := len(runtime.specs)
	runtime.mu.Unlock()
	if dispatches != 1 {
		t.Fatalf("provider starts after adapter revision drift = %d, want initial attempt only", dispatches)
	}
}

func TestAgentChannelRetryLosesRaceToArchiveAtDurableBoundary(t *testing.T) {
	for _, archive := range []struct {
		name string
		run  func(*AgentChannelService, string, string) error
	}{
		{
			name: "parent Agent Channel",
			run: func(svc *AgentChannelService, channelID, _ string) error {
				return svc.SetAgentChannelState(context.Background(), channelID, conversation.AgentChannelArchived)
			},
		},
		{
			name: "child Conversation",
			run: func(svc *AgentChannelService, channelID, conversationID string) error {
				return svc.SetAgentConversationState(context.Background(), channelID, conversationID, conversation.ConversationArchived)
			},
		},
	} {
		t.Run(archive.name, func(t *testing.T) {
			st := openPrimaryStore(t)
			capabilities, optionID := primaryCapability("studio")
			runtime := &primaryFailureRuntime{code: ErrorProviderFailed}
			primary := NewPrimaryChannelService(st, runtime, capabilities)
			svc := NewAgentChannelService(st, primary, nil)
			channel, err := svc.CreateAgentChannel(context.Background(), optionID, "Codex — Studio")
			if err != nil {
				t.Fatal(err)
			}
			child, err := svc.CreateAgentConversation(context.Background(), channel.Channel.ID, "Retry archive race")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := svc.PostAgentTurn(context.Background(), channel.Channel.ID, child.Conversation.ID, uuid.NewString(), "fail once"); err != nil {
				t.Fatal(err)
			}
			primary.Wait()
			detail, err := svc.GetAgentConversation(context.Background(), channel.Channel.ID, child.Conversation.ID)
			if err != nil || len(detail.Targets) != 1 || detail.Targets[0].State != conversation.TargetFailed {
				t.Fatalf("failed target = %+v, %v", detail.Targets, err)
			}

			checked := make(chan struct{})
			continueRetry := make(chan struct{})
			primary.now = func() time.Time {
				close(checked)
				<-continueRetry
				return time.Date(2026, 8, 20, 14, 50, 0, 0, time.UTC)
			}
			result := make(chan error, 1)
			go func() {
				_, retryErr := svc.RetryAgentTarget(context.Background(), channel.Channel.ID, child.Conversation.ID, detail.Targets[0].ID)
				result <- retryErr
			}()
			select {
			case <-checked:
			case <-time.After(5 * time.Second):
				t.Fatal("RetryAgentTarget did not reach the pre-write boundary")
			}
			if err := archive.run(svc, channel.Channel.ID, child.Conversation.ID); err != nil {
				t.Fatal(err)
			}
			close(continueRetry)
			select {
			case err := <-result:
				if ErrorCode(err) != ErrorAgentChannelState {
					t.Fatalf("concurrent archive error = %v (%q), want %q", err, ErrorCode(err), ErrorAgentChannelState)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("RetryAgentTarget did not finish after concurrent archive")
			}
			primary.Wait()
			runtime.mu.Lock()
			dispatches := len(runtime.specs)
			runtime.mu.Unlock()
			if dispatches != 1 {
				t.Fatalf("provider starts after concurrent archive = %d, want initial attempt only", dispatches)
			}
			durable, durableErr := st.GetConversation(child.Conversation.ID)
			if durableErr != nil || len(durable.Targets) != 1 {
				t.Fatalf("concurrent archive queued a retry = %+v, %v", durable.Targets, durableErr)
			}
		})
	}
}

func TestAgentChannelServiceOwnsNestedConversationAndDelegatesExecution(t *testing.T) {
	st := openPrimaryStore(t)
	capabilities, optionID := primaryCapability("studio")
	runtime := &primaryRuntimeFixture{answer: "agent answer"}
	primary := NewPrimaryChannelService(st, runtime, capabilities)
	svc := NewAgentChannelService(st, primary, nil)
	svc.now = func() time.Time { return time.Date(2026, 8, 20, 14, 0, 0, 0, time.UTC) }

	channel, err := svc.CreateAgentChannel(context.Background(), optionID, "Codex — Studio")
	if err != nil {
		t.Fatal(err)
	}
	child, err := svc.CreateAgentConversation(context.Background(), channel.Channel.ID, "Product direction")
	if err != nil {
		t.Fatal(err)
	}
	if child.ChannelID != channel.Channel.ID || child.Participant.SeatID != channel.Channel.Binding.Seat.ID {
		t.Fatalf("nested Conversation = %+v", child)
	}
	if _, err := st.GetPrimaryAgentSetting(); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("Agent Channel unexpectedly mutated singleton Primary Agent: %v", err)
	}
	if _, err := svc.GetAgentConversation(context.Background(), "foreign-agent", child.Conversation.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("foreign parent error = %v, want sql.ErrNoRows", err)
	}

	turn, err := svc.PostAgentTurn(context.Background(), channel.Channel.ID, child.Conversation.ID, uuid.NewString(), "hello")
	if err != nil || len(turn.Targets) != 1 {
		t.Fatalf("post Agent turn = %+v, %v", turn, err)
	}
	primary.Wait()
	got, err := svc.GetAgentConversation(context.Background(), channel.Channel.ID, child.Conversation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Messages) != 2 || got.Messages[1].Body != "agent answer" {
		t.Fatalf("Conversation messages = %+v", got.Messages)
	}

	if err := svc.SetAgentConversationPinned(context.Background(), channel.Channel.ID, child.Conversation.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := svc.RenameAgentConversation(context.Background(), channel.Channel.ID, child.Conversation.ID, "Pinned direction"); err != nil {
		t.Fatal(err)
	}
	items, err := svc.ListAgentConversations(context.Background(), channel.Channel.ID, string(conversation.ConversationOpen))
	if err != nil || len(items) != 1 || !items[0].Pinned || items[0].Conversation.Title != "Pinned direction" {
		t.Fatalf("nested Conversation list = %+v, %v", items, err)
	}
	if err := svc.SetAgentConversationState(context.Background(), channel.Channel.ID, child.Conversation.ID, conversation.ConversationArchived); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.PostAgentTurn(context.Background(), channel.Channel.ID, child.Conversation.ID, uuid.NewString(), "archived"); ErrorCode(err) != ErrorAgentChannelState {
		t.Fatalf("archived Conversation Send error = %v (%q)", err, ErrorCode(err))
	}
	rail, err := svc.ListAgentChannels(context.Background(), string(conversation.AgentChannelOpen))
	if err != nil || len(rail) != 1 || len(rail[0].Conversations) != 0 {
		t.Fatalf("open Agent rail included archived Conversation: %+v, %v", rail, err)
	}
	all, err := svc.ListAgentConversations(context.Background(), channel.Channel.ID, "all")
	if err != nil || len(all) != 1 || all[0].Conversation.State != conversation.ConversationArchived {
		t.Fatalf("archived Conversation was not retained: %+v, %v", all, err)
	}
}

func TestAgentConversationCreationLosesRaceToArchiveWithBoundedState(t *testing.T) {
	st := openPrimaryStore(t)
	capabilities, optionID := primaryCapability("studio")
	primary := NewPrimaryChannelService(st, &primaryRuntimeFixture{}, capabilities)
	svc := NewAgentChannelService(st, primary, nil)
	channel, err := svc.CreateAgentChannel(context.Background(), optionID, "Codex — Studio")
	if err != nil {
		t.Fatal(err)
	}

	checked := make(chan struct{})
	continueCreate := make(chan struct{})
	svc.now = func() time.Time {
		close(checked)
		<-continueCreate
		return time.Date(2026, 8, 20, 14, 30, 0, 0, time.UTC)
	}
	result := make(chan error, 1)
	go func() {
		_, createErr := svc.CreateAgentConversation(context.Background(), channel.Channel.ID, "Archive race")
		result <- createErr
	}()
	select {
	case <-checked:
	case <-time.After(5 * time.Second):
		t.Fatal("CreateAgentConversation did not reach the post-read boundary")
	}
	if err := svc.SetAgentChannelState(context.Background(), channel.Channel.ID, conversation.AgentChannelArchived); err != nil {
		t.Fatal(err)
	}
	close(continueCreate)
	select {
	case err := <-result:
		if ErrorCode(err) != ErrorAgentChannelState {
			t.Fatalf("concurrent archive error = %v (%q), want %q", err, ErrorCode(err), ErrorAgentChannelState)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("CreateAgentConversation did not finish after concurrent archive")
	}
	children, listErr := svc.ListAgentConversations(context.Background(), channel.Channel.ID, "all")
	if listErr != nil || len(children) != 0 {
		t.Fatalf("concurrent archive left children = %+v, %v", children, listErr)
	}
}

func TestAgentChannelServiceFirstSendAtomicallyCreatesOneConversationAndTurn(t *testing.T) {
	st := openPrimaryStore(t)
	capabilities, optionID := primaryCapability("studio")
	runtime := &primaryRuntimeFixture{answer: "first answer"}
	primary := NewPrimaryChannelService(st, runtime, capabilities)
	svc := NewAgentChannelService(st, primary, nil)
	svc.now = func() time.Time { return time.Date(2026, 8, 20, 15, 0, 0, 0, time.UTC) }

	channel, err := svc.CreateAgentChannel(context.Background(), optionID, "Codex — Studio")
	if err != nil {
		t.Fatal(err)
	}
	clientTurnID := uuid.NewString()
	first, err := svc.PostFirstAgentTurn(context.Background(), channel.Channel.ID, "New conversation", clientTurnID, "start here")
	if err != nil {
		t.Fatal(err)
	}
	if first.Conversation.Conversation.ID == "" || first.Turn.ConversationID != first.Conversation.Conversation.ID || len(first.Targets) != 1 {
		t.Fatalf("first Send = %+v", first)
	}
	primary.Wait()
	capabilities.mu.Lock()
	preflightsBeforeReplay := len(capabilities.machineRefreshes)
	capabilities.mu.Unlock()

	replayed, err := svc.PostFirstAgentTurn(context.Background(), channel.Channel.ID, "ignored on replay", clientTurnID, "ignored on replay")
	if err != nil {
		t.Fatal(err)
	}
	if replayed.Conversation.Conversation.ID != first.Conversation.Conversation.ID || replayed.Turn.ID != first.Turn.ID {
		t.Fatalf("replayed first Send = %+v, want Conversation %s Turn %s", replayed, first.Conversation.Conversation.ID, first.Turn.ID)
	}
	items, err := svc.ListAgentConversations(context.Background(), channel.Channel.ID, string(conversation.ConversationOpen))
	if err != nil || len(items) != 1 {
		t.Fatalf("Conversations after replay = %+v, %v", items, err)
	}
	if got := len(runtime.captured()); got != 1 {
		t.Fatalf("provider starts = %d, want 1", got)
	}
	capabilities.mu.Lock()
	preflights := len(capabilities.machineRefreshes)
	capabilities.mu.Unlock()
	if preflights != preflightsBeforeReplay {
		t.Fatalf("fresh readiness probes after replay = %d, want unchanged %d; idempotent replay must use durable state", preflights, preflightsBeforeReplay)
	}
}

func TestAgentChannelFirstSendRejectsAdapterRevisionChangedDuringFreshPreflight(t *testing.T) {
	st := openPrimaryStore(t)
	capabilities, optionID := primaryCapability("studio")
	runtime := &primaryRuntimeFixture{answer: "must not run"}
	primary := NewPrimaryChannelService(st, runtime, capabilities)
	svc := NewAgentChannelService(st, primary, nil)

	channel, err := svc.CreateAgentChannel(context.Background(), optionID, "Codex — Studio")
	if err != nil {
		t.Fatal(err)
	}
	capabilities.mu.Lock()
	driftedMachine := capabilities.snapshot.Machines[0]
	driftedMachine.TextOnlyOptions = append([]corecap.TextOnlyOptionOffer(nil), driftedMachine.TextOnlyOptions...)
	driftedMachine.TextOnlyOptions[0].AdapterRevision = strings.Repeat("9", 64)
	capabilities.machineOverride = &driftedMachine
	capabilities.mu.Unlock()

	_, err = svc.PostFirstAgentTurn(context.Background(), channel.Channel.ID, "Fresh preflight", uuid.NewString(), "do not substitute")
	if ErrorCode(err) != ErrorPrimaryAgentDrift {
		t.Fatalf("first Send adapter revision drift error = %v (%q), want %q", err, ErrorCode(err), ErrorPrimaryAgentDrift)
	}
	primary.Wait()
	if got := runtime.captured(); len(got) != 0 {
		t.Fatalf("provider starts after first-Send adapter revision drift = %+v", got)
	}
	children, listErr := svc.ListAgentConversations(context.Background(), channel.Channel.ID, "all")
	if listErr != nil || len(children) != 0 {
		t.Fatalf("first-Send drift left Conversations = %+v, %v", children, listErr)
	}
}

func TestAgentChannelFirstSendLosesRaceToArchiveWithBoundedState(t *testing.T) {
	st := openPrimaryStore(t)
	capabilities, optionID := primaryCapability("studio")
	runtime := &primaryRuntimeFixture{answer: "must not run"}
	primary := NewPrimaryChannelService(st, runtime, capabilities)
	svc := NewAgentChannelService(st, primary, nil)
	channel, err := svc.CreateAgentChannel(context.Background(), optionID, "Codex — Studio")
	if err != nil {
		t.Fatal(err)
	}

	checked := make(chan struct{})
	continueSend := make(chan struct{})
	var pause sync.Once
	svc.now = func() time.Time {
		pause.Do(func() {
			close(checked)
			<-continueSend
		})
		return time.Date(2026, 8, 20, 15, 30, 0, 0, time.UTC)
	}
	result := make(chan error, 1)
	go func() {
		_, sendErr := svc.PostFirstAgentTurn(context.Background(), channel.Channel.ID, "Archive race", uuid.NewString(), "do not commit")
		result <- sendErr
	}()
	select {
	case <-checked:
	case <-time.After(5 * time.Second):
		t.Fatal("PostFirstAgentTurn did not reach the post-read boundary")
	}
	if err := svc.SetAgentChannelState(context.Background(), channel.Channel.ID, conversation.AgentChannelArchived); err != nil {
		t.Fatal(err)
	}
	close(continueSend)
	select {
	case err := <-result:
		if ErrorCode(err) != ErrorAgentChannelState {
			t.Fatalf("concurrent archive error = %v (%q), want %q", err, ErrorCode(err), ErrorAgentChannelState)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("PostFirstAgentTurn did not finish after concurrent archive")
	}
	primary.Wait()
	if got := runtime.captured(); len(got) != 0 {
		t.Fatalf("provider starts after concurrent archive = %+v", got)
	}
	children, listErr := svc.ListAgentConversations(context.Background(), channel.Channel.ID, "all")
	if listErr != nil || len(children) != 0 {
		t.Fatalf("concurrent archive left Conversations = %+v, %v", children, listErr)
	}
}

func TestAgentChannelServiceRejectsRetryForNonRecoverableFailure(t *testing.T) {
	st := openPrimaryStore(t)
	capabilities, optionID := primaryCapability("studio")
	failing := &primaryFailureRuntime{code: ErrorProviderRefusal}
	primary := NewPrimaryChannelService(st, failing, capabilities)
	svc := NewAgentChannelService(st, primary, nil)

	channel, err := svc.CreateAgentChannel(context.Background(), optionID, "Codex — Studio")
	if err != nil {
		t.Fatal(err)
	}
	child, err := svc.CreateAgentConversation(context.Background(), channel.Channel.ID, "Refused request")
	if err != nil {
		t.Fatal(err)
	}
	posted, err := svc.PostAgentTurn(context.Background(), channel.Channel.ID, child.Conversation.ID, uuid.NewString(), "decline this")
	if err != nil {
		t.Fatal(err)
	}
	primary.Wait()

	_, err = svc.RetryAgentTarget(context.Background(), channel.Channel.ID, child.Conversation.ID, posted.Targets[0].ID)
	if ErrorCode(err) != ErrorAgentRecoveryUnavailable {
		t.Fatalf("non-recoverable retry error = %v (%q)", err, ErrorCode(err))
	}
	needs, needsErr := svc.AgentNeedsYou(context.Background())
	if needsErr != nil || len(needs) != 0 {
		t.Fatalf("non-recoverable refusal appeared in Needs You: %+v, %v", needs, needsErr)
	}
	failing.mu.Lock()
	dispatches := len(failing.specs)
	failing.mu.Unlock()
	if dispatches != 1 {
		t.Fatalf("provider dispatches = %d, want 1", dispatches)
	}
}

func TestUnavailableAgentChannelStartsNoProviderAndCreatesNoFirstConversation(t *testing.T) {
	st := openPrimaryStore(t)
	capabilities, optionID := primaryCapability("studio")
	runtime := &primaryRuntimeFixture{answer: "must not run"}
	primary := NewPrimaryChannelService(st, runtime, capabilities)
	svc := NewAgentChannelService(st, primary, nil)
	channel, err := svc.CreateAgentChannel(context.Background(), optionID, "Codex — Studio")
	if err != nil {
		t.Fatal(err)
	}

	capabilities.mu.Lock()
	capabilities.snapshot.Machines[0].TextOnlyOptions = nil
	capabilities.mu.Unlock()
	_, err = svc.PostFirstAgentTurn(context.Background(), channel.Channel.ID, "Unavailable", uuid.NewString(), "do not dispatch")
	if ErrorCode(err) != ErrorPrimaryAgentDrift {
		t.Fatalf("unavailable first Send error = %v (%q)", err, ErrorCode(err))
	}
	primary.Wait()
	if got := len(runtime.captured()); got != 0 {
		t.Fatalf("provider starts = %d, want 0", got)
	}
	items, listErr := svc.ListAgentConversations(context.Background(), channel.Channel.ID, "all")
	if listErr != nil || len(items) != 0 {
		t.Fatalf("atomic failure left Conversations = %+v, %v", items, listErr)
	}
}

func TestMigratedPrimaryConversationRemainsReadyAndDispatchableByExactBinding(t *testing.T) {
	st := openPrimaryStore(t)
	capabilities, optionID := primaryCapability("studio")
	runtime := &primaryRuntimeFixture{answer: "migrated answer"}
	primary := NewPrimaryChannelService(st, runtime, capabilities)
	if _, err := primary.SetPrimaryAgent(context.Background(), optionID); err != nil {
		t.Fatal(err)
	}
	legacy, err := primary.CreateChannel(context.Background(), "Existing Primary chat")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.MigratePrimaryAgentChannels(); err != nil {
		t.Fatal(err)
	}
	svc := NewAgentChannelService(st, primary, nil)
	channels, err := svc.ListAgentChannels(context.Background(), string(conversation.AgentChannelOpen))
	if err != nil || len(channels) != 1 || len(channels[0].Conversations) != 1 {
		t.Fatalf("migrated hierarchy = %+v, %v", channels, err)
	}
	if channels[0].Readiness.State != PrimaryAgentReady {
		t.Fatalf("migrated Agent Channel readiness = %+v", channels[0].Readiness)
	}
	if channels[0].Conversations[0].Conversation.ID != legacy.Conversation.ID {
		t.Fatalf("migrated Conversation = %+v, want %s", channels[0].Conversations[0], legacy.Conversation.ID)
	}
	if _, err := svc.PostAgentTurn(context.Background(), channels[0].Channel.ID, legacy.Conversation.ID, uuid.NewString(), "continue"); err != nil {
		t.Fatal(err)
	}
	primary.Wait()
	if got := runtime.captured(); len(got) != 1 || got[0].Prompt == "" {
		t.Fatalf("migrated dispatches = %+v", got)
	}
}

func TestMigratedConversationPreservesLegacyPrimaryFreshAdapterBehavior(t *testing.T) {
	st := openPrimaryStore(t)
	capabilities, optionID := primaryCapability("studio")
	runtime := &primaryRuntimeFixture{answer: "legacy answer"}
	primary := NewPrimaryChannelService(st, runtime, capabilities)
	if _, err := primary.SetPrimaryAgent(context.Background(), optionID); err != nil {
		t.Fatal(err)
	}
	legacy, err := primary.CreateChannel(context.Background(), "Legacy Primary chat")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.MigratePrimaryAgentChannels(); err != nil {
		t.Fatal(err)
	}
	driftedRevision := strings.Repeat("9", 64)
	capabilities.mu.Lock()
	capabilities.snapshot.Machines[0].TextOnlyOptions[0].AdapterRevision = driftedRevision
	capabilities.mu.Unlock()

	result, err := primary.PostTurn(context.Background(), legacy.Conversation.ID, uuid.NewString(), "legacy route")
	if err != nil {
		t.Fatalf("legacy Primary Send after Agent migration: %v", err)
	}
	if len(result.Targets) != 1 || result.Targets[0].Authority == nil || result.Targets[0].Authority.Policy.AdapterRevision != driftedRevision {
		t.Fatalf("legacy Primary target = %+v, want fresh adapter %s", result.Targets, driftedRevision)
	}
	primary.Wait()
	if got := runtime.captured(); len(got) != 1 || got[0].TextOnlyPolicy.SelectedAdapterRevision != driftedRevision {
		t.Fatalf("legacy Primary dispatch = %+v", got)
	}
}

func TestAgentCreatedConversationRejectsLegacyPrimaryAdapterBypass(t *testing.T) {
	st := openPrimaryStore(t)
	capabilities, optionID := primaryCapability("studio")
	runtime := &primaryRuntimeFixture{answer: "must not run"}
	primary := NewPrimaryChannelService(st, runtime, capabilities)
	svc := NewAgentChannelService(st, primary, nil)
	channel, err := svc.CreateAgentChannel(context.Background(), optionID, "Codex — Studio")
	if err != nil {
		t.Fatal(err)
	}
	child, err := svc.CreateAgentConversation(context.Background(), channel.Channel.ID, "Agent-created")
	if err != nil {
		t.Fatal(err)
	}
	capabilities.mu.Lock()
	capabilities.snapshot.Machines[0].TextOnlyOptions[0].AdapterRevision = strings.Repeat("9", 64)
	capabilities.mu.Unlock()

	_, err = primary.PostTurn(context.Background(), child.Conversation.ID, uuid.NewString(), "legacy bypass")
	if ErrorCode(err) != ErrorPrimaryAgentDrift {
		t.Fatalf("legacy bypass error = %v (%q), want %q", err, ErrorCode(err), ErrorPrimaryAgentDrift)
	}
	primary.Wait()
	if got := runtime.captured(); len(got) != 0 {
		t.Fatalf("legacy bypass started provider = %+v", got)
	}
	detail, detailErr := st.GetConversation(child.Conversation.ID)
	if detailErr != nil || len(detail.Turns) != 0 || len(detail.Targets) != 0 {
		t.Fatalf("legacy bypass persisted turn = %+v, %v", detail, detailErr)
	}
}

func TestAgentCreatedConversationRejectsLegacyPrimaryRetryAdapterBypass(t *testing.T) {
	st := openPrimaryStore(t)
	capabilities, optionID := primaryCapability("studio")
	runtime := &primaryFailureRuntime{code: ErrorProviderFailed}
	primary := NewPrimaryChannelService(st, runtime, capabilities)
	svc := NewAgentChannelService(st, primary, nil)
	channel, err := svc.CreateAgentChannel(context.Background(), optionID, "Codex — Studio")
	if err != nil {
		t.Fatal(err)
	}
	child, err := svc.CreateAgentConversation(context.Background(), channel.Channel.ID, "Agent-created retry")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.PostAgentTurn(context.Background(), channel.Channel.ID, child.Conversation.ID, uuid.NewString(), "fail once"); err != nil {
		t.Fatal(err)
	}
	primary.Wait()
	detail, err := svc.GetAgentConversation(context.Background(), channel.Channel.ID, child.Conversation.ID)
	if err != nil || len(detail.Targets) != 1 || detail.Targets[0].State != conversation.TargetFailed {
		t.Fatalf("failed target = %+v, %v", detail.Targets, err)
	}
	capabilities.mu.Lock()
	capabilities.snapshot.Machines[0].TextOnlyOptions[0].AdapterRevision = strings.Repeat("9", 64)
	capabilities.mu.Unlock()

	_, err = primary.RetryTarget(context.Background(), child.Conversation.ID, detail.Targets[0].ID)
	if ErrorCode(err) != ErrorPrimaryAgentDrift {
		t.Fatalf("legacy retry bypass error = %v (%q), want %q", err, ErrorCode(err), ErrorPrimaryAgentDrift)
	}
	primary.Wait()
	runtime.mu.Lock()
	dispatches := len(runtime.specs)
	runtime.mu.Unlock()
	if dispatches != 1 {
		t.Fatalf("legacy retry bypass provider starts = %d, want initial attempt only", dispatches)
	}
	durable, durableErr := st.GetConversation(child.Conversation.ID)
	if durableErr != nil || len(durable.Targets) != 1 {
		t.Fatalf("legacy retry bypass persisted attempt = %+v, %v", durable.Targets, durableErr)
	}
}

func TestAtomicFirstAgentConversationRejectsLegacyPrimaryAdapterBypass(t *testing.T) {
	st := openPrimaryStore(t)
	capabilities, optionID := primaryCapability("studio")
	runtime := &primaryRuntimeFixture{answer: "first answer"}
	primary := NewPrimaryChannelService(st, runtime, capabilities)
	svc := NewAgentChannelService(st, primary, nil)
	channel, err := svc.CreateAgentChannel(context.Background(), optionID, "Codex — Studio")
	if err != nil {
		t.Fatal(err)
	}
	first, err := svc.PostFirstAgentTurn(context.Background(), channel.Channel.ID, "Atomic first", uuid.NewString(), "first")
	if err != nil {
		t.Fatal(err)
	}
	primary.Wait()
	capabilities.mu.Lock()
	capabilities.snapshot.Machines[0].TextOnlyOptions[0].AdapterRevision = strings.Repeat("9", 64)
	capabilities.mu.Unlock()

	_, err = primary.PostTurn(context.Background(), first.Conversation.Conversation.ID, uuid.NewString(), "legacy bypass")
	if ErrorCode(err) != ErrorPrimaryAgentDrift {
		t.Fatalf("atomic first legacy bypass error = %v (%q), want %q", err, ErrorCode(err), ErrorPrimaryAgentDrift)
	}
	primary.Wait()
	if got := runtime.captured(); len(got) != 1 {
		t.Fatalf("atomic first legacy bypass provider starts = %d, want initial attempt only", len(got))
	}
}
