package control

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	corecap "github.com/tobsai/fort/core/capability"
	"github.com/tobsai/fort/core/conversation"
	coreruntime "github.com/tobsai/fort/core/runtime"
	"github.com/tobsai/fort/core/store"
	"github.com/tobsai/fort/ui"
)

const (
	PrimaryAgentReady         = "ready"
	PrimaryAgentNotConfigured = "not_configured"
	PrimaryAgentUnready       = "unready"
	PrimaryAgentDrifted       = "drifted"
	PrimaryAgentIneligible    = "ineligible"

	PrimaryAgentReasonNotEligible = "not_eligible_for_text_only_chat"

	ErrorPrimaryAgentNotConfigured = "primary_agent_not_configured"
	ErrorPrimaryAgentUnready       = "primary_agent_unready"
	ErrorPrimaryAgentDrift         = "primary_agent_drift"
	ErrorChatPolicyUnavailable     = "chat_policy_unavailable"
	ErrorChatAuthorityViolation    = "chat_authority_violation"
	ErrorPrimaryChannelInvariant   = "primary_channel_invariant"
	ErrorProviderResultUnknown     = "provider_result_unknown"
	ErrorProviderIncomplete        = "provider_incomplete"
	ErrorProviderRefusal           = "provider_refusal"
	ErrorProviderFailed            = "provider_failed"
	ErrorAgentChannelState         = "agent_channel_state"
	ErrorAgentRecoveryUnavailable  = "agent_recovery_unavailable"
)

var primaryCapabilityAdapters = []string{
	"profile.codex-subscription.isolated",
	"model.chat.text-only.codex-subscription",
	"codex-subscription-chat",
}

// PrimaryChannelError keeps the Phase 1 closed recovery code independent of
// HTTP while retaining the underlying diagnostic for logs and tests.
type PrimaryChannelError struct {
	Code string
	Err  error
}

func (e *PrimaryChannelError) Error() string              { return e.Err.Error() }
func (e *PrimaryChannelError) Unwrap() error              { return e.Err }
func (e *PrimaryChannelError) PrimaryChannelCode() string { return e.Code }

func primaryError(code string, err error) error {
	if err == nil {
		err = errors.New(code)
	}
	return &PrimaryChannelError{Code: code, Err: err}
}

// ErrorCode returns only Phase 1's bounded service code. Storage and context
// errors remain available through errors.Is/As and are mapped by the caller.
func ErrorCode(err error) string {
	var bounded *PrimaryChannelError
	if errors.As(err, &bounded) {
		return bounded.Code
	}
	var conversationError *conversation.BoundedError
	if errors.As(err, &conversationError) {
		return string(conversationError.Code)
	}
	if errors.Is(err, conversation.ErrContextTooLarge) {
		return "conversation_context_limit"
	}
	return ""
}

type PrimaryAgentOption = ui.PrimaryAgentOption
type PrimaryAgentView = ui.PrimaryAgentView
type PrimaryNeedsYouItem = ui.PrimaryNeedsYouItem

// PrimaryOptionCapabilities is the only capability mutation surface exposed
// to Channels. Recheck and per-target preflight can invalidate only the three
// catalog rows that constitute the closed subscription binding.
type PrimaryOptionCapabilities interface {
	Capabilities() (corecap.Snapshot, uint64)
	Refresh(context.Context, corecap.RefreshMode, []string) (corecap.Snapshot, uint64, error)
	RefreshMachine(context.Context, string, corecap.RefreshMode, []string) (corecap.MachineInventory, error)
}

// PrimaryChannelService coordinates the canonical Channel rows and the one
// isolated subscription runtime. It has no generic target-selection method.
type PrimaryChannelService struct {
	store        *store.Store
	runtime      coreruntime.Runtime
	capabilities PrimaryOptionCapabilities
	now          func() time.Time
	ctx          context.Context
	cancel       context.CancelFunc

	mu     sync.Mutex
	active map[string]coreruntime.Run
	starts map[string]context.CancelFunc
	async  sync.WaitGroup
}

func NewPrimaryChannelService(st *store.Store, rt coreruntime.Runtime, capabilities PrimaryOptionCapabilities) *PrimaryChannelService {
	ctx, cancel := context.WithCancel(context.Background())
	return &PrimaryChannelService{
		store: st, runtime: rt, capabilities: capabilities, now: time.Now,
		ctx: ctx, cancel: cancel, active: map[string]coreruntime.Run{}, starts: map[string]context.CancelFunc{},
	}
}

func (s *PrimaryChannelService) PrimaryAgent(context.Context) (PrimaryAgentView, error) {
	options := s.currentOptions()
	setting, err := s.store.GetPrimaryAgentSetting()
	if errors.Is(err, sql.ErrNoRows) {
		return PrimaryAgentView{State: PrimaryAgentNotConfigured, Reason: ErrorPrimaryAgentNotConfigured, Options: options}, nil
	}
	if err != nil {
		return PrimaryAgentView{}, err
	}
	state, reason := settingState(setting, options)
	return PrimaryAgentView{Selection: &setting, State: state, Reason: reason, Options: options}, nil
}

func (s *PrimaryChannelService) SetPrimaryAgent(_ context.Context, optionID string) (PrimaryAgentView, error) {
	for _, option := range s.currentOptions() {
		if option.ID != optionID || option.State != PrimaryAgentReady {
			continue
		}
		setting := settingFromOption(option, s.now().UTC())
		if err := s.store.UpsertPrimaryAgentSetting(setting); err != nil {
			return PrimaryAgentView{}, err
		}
		return s.PrimaryAgent(context.Background())
	}
	return PrimaryAgentView{}, primaryError(ErrorPrimaryAgentUnready, fmt.Errorf("primary option %q is not currently ready", optionID))
}

func (s *PrimaryChannelService) ClearPrimaryAgent(context.Context) error {
	return s.store.ClearPrimaryAgentSetting()
}

func (s *PrimaryChannelService) RecheckPrimaryAgent(ctx context.Context) (PrimaryAgentView, error) {
	if s.capabilities == nil {
		return PrimaryAgentView{}, primaryError(ErrorPrimaryAgentUnready, errors.New("primary capability inventory is unavailable"))
	}
	if _, _, err := s.capabilities.Refresh(ctx, corecap.RefreshUserRecheck, append([]string(nil), primaryCapabilityAdapters...)); err != nil {
		return PrimaryAgentView{}, primaryError(ErrorPrimaryAgentUnready, err)
	}
	return s.PrimaryAgent(ctx)
}

func (s *PrimaryChannelService) ListChannels(_ context.Context, state string) ([]conversation.PrimaryChannelSummary, error) {
	items, err := s.store.ListPrimaryChannels(state)
	if items == nil {
		items = []conversation.PrimaryChannelSummary{}
	}
	return items, err
}

func (s *PrimaryChannelService) GetChannel(_ context.Context, id string) (ui.PrimaryChannelDetail, error) {
	detail, err := s.store.GetConversation(id)
	if err != nil {
		return ui.PrimaryChannelDetail{}, err
	}
	if detail.PrimaryChannel == nil || len(detail.Participants) != 1 || detail.Participants[0].ID != detail.PrimaryChannel.ParticipantID {
		return ui.PrimaryChannelDetail{}, primaryError(ErrorPrimaryChannelInvariant, fmt.Errorf("conversation %s is not a Primary Channel", id))
	}
	return ui.PrimaryChannelDetail{
		Conversation: detail.Conversation, Participants: detail.Participants, Messages: detail.Messages,
		Turns: detail.Turns, Targets: detail.Targets, PrimaryChannel: detail.PrimaryChannel,
		Readiness: s.channelReadiness(detail.Participants[0], *detail.PrimaryChannel),
	}, nil
}

func (s *PrimaryChannelService) channelReadiness(participant conversation.Participant, channel conversation.PrimaryChannel) ui.PrimaryChannelReadiness {
	readiness := ui.PrimaryChannelReadiness{State: PrimaryAgentUnready, Reason: ErrorPrimaryAgentUnready}
	if s.capabilities == nil {
		return readiness
	}
	snapshot, generation := s.capabilities.Capabilities()
	readiness.ObservedAt = snapshot.ObservedAt.UTC()
	if generation == 0 {
		return readiness
	}
	for _, machine := range snapshot.Machines {
		for _, option := range optionsFromMachine(machine) {
			if option.State == PrimaryAgentReady && compatibleChannelOption(option.Offer, participant, channel) {
				readiness.State = PrimaryAgentReady
				readiness.Reason = ""
				return readiness
			}
			if option.Seat.Profile != participant.Profile || !strings.EqualFold(option.Seat.Machine, participant.Machine) ||
				(option.Seat.ID != "" && option.Seat.ID != participant.SeatID) {
				continue
			}
			if option.State == PrimaryAgentReady {
				readiness.State = PrimaryAgentDrifted
				readiness.Reason = ErrorPrimaryAgentDrift
			} else {
				readiness.State = option.State
				readiness.Reason = option.Reason
			}
		}
	}
	return readiness
}

func (s *PrimaryChannelService) CreateChannel(ctx context.Context, name string) (ui.PrimaryChannelDetail, error) {
	name = strings.TrimSpace(name)
	if name == "" || len([]byte(name)) > 120 {
		return ui.PrimaryChannelDetail{}, fmt.Errorf("Channel name must contain 1 to 120 UTF-8 bytes")
	}
	view, err := s.PrimaryAgent(ctx)
	if err != nil {
		return ui.PrimaryChannelDetail{}, err
	}
	if view.Selection == nil {
		return ui.PrimaryChannelDetail{}, primaryError(ErrorPrimaryAgentNotConfigured, errors.New("choose a Primary Agent before creating a Channel"))
	}
	if view.State != PrimaryAgentReady {
		return ui.PrimaryChannelDetail{}, primaryError(view.Reason, errors.New("the selected Primary Agent is not currently ready"))
	}
	now := s.now().UTC()
	id := uuid.NewString()
	item := conversation.Conversation{ID: id, Title: name, State: conversation.ConversationOpen, CreatedAt: now, UpdatedAt: now}
	if err := s.store.CreatePrimaryChannel(item, uuid.NewString()); err != nil {
		return ui.PrimaryChannelDetail{}, err
	}
	return s.GetChannel(ctx, id)
}

func (s *PrimaryChannelService) RenameChannel(ctx context.Context, id, name string) error {
	name = strings.TrimSpace(name)
	if name == "" || len([]byte(name)) > 120 {
		return fmt.Errorf("Channel name must contain 1 to 120 UTF-8 bytes")
	}
	if _, err := s.GetChannel(ctx, id); err != nil {
		return err
	}
	return s.store.RenameConversation(id, name)
}

func (s *PrimaryChannelService) SetChannelState(ctx context.Context, id string, state conversation.ConversationState) error {
	if state != conversation.ConversationOpen && state != conversation.ConversationArchived {
		return fmt.Errorf("Channel state must be open or archived")
	}
	if _, err := s.GetChannel(ctx, id); err != nil {
		return err
	}
	return s.store.SetConversationState(id, state)
}

func (s *PrimaryChannelService) SetChannelPinned(ctx context.Context, id string, pinned bool) error {
	if _, err := s.GetChannel(ctx, id); err != nil {
		return err
	}
	return s.store.SetPrimaryChannelPinned(id, pinned, s.now().UTC())
}

func (s *PrimaryChannelService) PostTurn(ctx context.Context, channelID, clientTurnID, text string) (conversation.TurnResult, error) {
	return s.postTurn(ctx, channelID, clientTurnID, text, "", "")
}

func (s *PrimaryChannelService) postTurn(
	ctx context.Context,
	channelID, clientTurnID, text, requiredAdapterRevision, agentChannelID string,
) (conversation.TurnResult, error) {
	if strings.TrimSpace(clientTurnID) == "" {
		return conversation.TurnResult{}, fmt.Errorf("client_turn_id is required")
	}
	if agentChannelID == "" {
		createdChannel, found, err := s.store.AgentCreatedConversationChannel(channelID)
		if err != nil {
			return conversation.TurnResult{}, err
		}
		if found {
			agentChannelID = createdChannel.ID
			requiredAdapterRevision = createdChannel.Binding.Authority.AdapterRevision
		}
	}
	detail, err := s.GetChannel(ctx, channelID)
	if err != nil {
		return conversation.TurnResult{}, err
	}
	if turn, targets, found, err := s.store.FindConversationTurnByClientID(channelID, clientTurnID); err != nil {
		return conversation.TurnResult{}, err
	} else if found {
		return conversation.TurnResult{Turn: turn, Targets: nonnilTargets(targets)}, nil
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return conversation.TurnResult{}, fmt.Errorf("message text is required")
	}
	if detail.Conversation.State != conversation.ConversationOpen {
		return conversation.TurnResult{}, fmt.Errorf("archived Channel %s must be reopened before sending", channelID)
	}
	for _, target := range detail.Targets {
		if target.State == conversation.TargetQueued || target.State == conversation.TargetWorking {
			return conversation.TurnResult{}, conversation.NewBoundedError(conversation.ErrorConversationActive, fmt.Errorf("Channel %s already has an active target", channelID))
		}
	}
	participant := detail.Participants[0]
	offer, err := s.freshCompatibleOption(ctx, participant.Machine, participant, *detail.PrimaryChannel, requiredAdapterRevision)
	if err != nil {
		return conversation.TurnResult{}, err
	}
	authority := targetAuthorityFromChannel(*detail.PrimaryChannel, participant.Model, offer.AdapterRevision)
	now := s.now().UTC()
	turn, targets, prompt, err := s.store.CreateConversationTurn(store.CreateConversationTurnParams{
		TurnID: uuid.NewString(), ClientTurnID: clientTurnID, ConversationID: channelID,
		AgentChannelID: agentChannelID,
		HumanID:        "human", Body: text, CreatedAt: now, PrimarySingleFlight: true,
		Targets: []store.ConversationTurnTarget{{
			ID: uuid.NewString(), ParticipantID: participant.ID, RunID: uuid.NewString(), Authority: authority,
		}},
	})
	if err != nil {
		if agentChannelID != "" {
			err = agentChannelStoreError(err)
		}
		return conversation.TurnResult{}, err
	}
	result := conversation.TurnResult{Turn: turn, Targets: nonnilTargets(targets)}
	if turn.Created && len(targets) == 1 {
		dispatch, dispatchErr := s.store.GetConversationTargetDispatch(targets[0].ID)
		if dispatchErr != nil {
			s.failQueued(targets[0], dispatchErr, ErrorProviderFailed)
		} else {
			s.startTarget(dispatch, prompt)
		}
	}
	return result, nil
}

func (s *PrimaryChannelService) CancelTarget(ctx context.Context, channelID, targetID string) error {
	dispatch, err := s.nestedTarget(ctx, channelID, targetID)
	if err != nil {
		return err
	}
	for {
		if dispatch.Target.State != conversation.TargetQueued && dispatch.Target.State != conversation.TargetWorking {
			return fmt.Errorf("target %s is already %s", targetID, dispatch.Target.State)
		}
		changed, transitionErr := s.store.TransitionConversationTargetWithReceipt(
			targetID, dispatch.Target.State, conversation.TargetCanceled, "", "canceled by user",
			unknownTargetReceipt(dispatch.Target, "canceled"),
		)
		if transitionErr != nil {
			return transitionErr
		}
		if changed {
			break
		}
		dispatch, err = s.nestedTarget(ctx, channelID, targetID)
		if err != nil {
			return err
		}
	}
	s.mu.Lock()
	startCancel := s.starts[targetID]
	run := s.active[targetID]
	s.mu.Unlock()
	if startCancel != nil {
		startCancel()
	}
	if run != nil {
		_ = run.Cancel()
	}
	_ = s.store.UpdateRunStatus(dispatch.Target.RunID, "canceled", 0, "canceled by user")
	return nil
}

func (s *PrimaryChannelService) RetryTarget(ctx context.Context, channelID, targetID string) (conversation.Target, error) {
	return s.retryTarget(ctx, channelID, targetID, "", "")
}

func (s *PrimaryChannelService) RecheckAndRetryTarget(ctx context.Context, channelID, targetID string) (conversation.Target, error) {
	return s.retryTarget(ctx, channelID, targetID, "", "")
}

func (s *PrimaryChannelService) retryTarget(ctx context.Context, channelID, targetID, requiredAdapterRevision, agentChannelID string) (conversation.Target, error) {
	if agentChannelID == "" {
		createdChannel, found, err := s.store.AgentCreatedConversationChannel(channelID)
		if err != nil {
			return conversation.Target{}, err
		}
		if found {
			agentChannelID = createdChannel.ID
			requiredAdapterRevision = createdChannel.Binding.Authority.AdapterRevision
		}
	}
	original, err := s.nestedTarget(ctx, channelID, targetID)
	if err != nil {
		return conversation.Target{}, err
	}
	if original.Target.State != conversation.TargetFailed {
		return conversation.Target{}, fmt.Errorf("target %s is %s; only failed targets can be retried", targetID, original.Target.State)
	}
	detail, err := s.GetChannel(ctx, channelID)
	if err != nil {
		return conversation.Target{}, err
	}
	if !isLatestAttempt(detail.Targets, original.Target) {
		return conversation.Target{}, fmt.Errorf("target %s has a newer attempt", targetID)
	}
	offer, err := s.freshCompatibleOption(ctx, original.Participant.Machine, original.Participant, *detail.PrimaryChannel, requiredAdapterRevision)
	if err != nil {
		return conversation.Target{}, err
	}
	var retry store.ConversationTargetDispatch
	if agentChannelID == "" {
		retry, err = s.store.RetryConversationTargetWithAdapterRevision(
			targetID, uuid.NewString(), uuid.NewString(), offer.AdapterRevision, s.now().UTC(),
		)
	} else {
		retry, err = s.store.RetryAgentConversationTargetWithAdapterRevision(
			agentChannelID, targetID, uuid.NewString(), uuid.NewString(), offer.AdapterRevision, s.now().UTC(),
		)
	}
	if err != nil {
		if agentChannelID != "" {
			err = agentChannelStoreError(err)
		}
		return conversation.Target{}, err
	}
	prompt, err := s.store.ConversationContext(channelID, retry.Turn.ThroughMessageID)
	if err != nil {
		s.failQueued(retry.Target, err, ErrorProviderFailed)
		return conversation.Target{}, err
	}
	s.startTarget(retry, prompt)
	return retry.Target, nil
}

func (s *PrimaryChannelService) NeedsYou(ctx context.Context) ([]PrimaryNeedsYouItem, error) {
	channels, err := s.ListChannels(ctx, string(conversation.ConversationOpen))
	if err != nil {
		return nil, err
	}
	items := []PrimaryNeedsYouItem{}
	for _, channel := range channels {
		detail, detailErr := s.GetChannel(ctx, channel.Conversation.ID)
		if detailErr != nil {
			return nil, detailErr
		}
		latest, ok := latestPrimaryTarget(store.ConversationDetail{Turns: detail.Turns, Targets: detail.Targets})
		if !ok {
			continue
		}
		if latest.State != conversation.TargetFailed {
			continue
		}
		actions := recoveryActions(latest.ErrorCode)
		if len(actions) == 0 {
			continue
		}
		items = append(items, PrimaryNeedsYouItem{Channel: channel, Target: latest, RecoveryActions: actions})
	}
	return items, nil
}

func (s *PrimaryChannelService) Wait() { s.async.Wait() }

func (s *PrimaryChannelService) Close() {
	s.cancel()
	s.mu.Lock()
	for _, cancel := range s.starts {
		cancel()
	}
	for _, run := range s.active {
		_ = run.Cancel()
	}
	s.mu.Unlock()
	s.Wait()
}

func (s *PrimaryChannelService) currentOptions() []PrimaryAgentOption {
	if s.capabilities == nil {
		return []PrimaryAgentOption{}
	}
	snapshot, generation := s.capabilities.Capabilities()
	if generation == 0 {
		return []PrimaryAgentOption{}
	}
	options := []PrimaryAgentOption{}
	for _, machine := range snapshot.Machines {
		options = append(options, optionsFromMachine(machine)...)
	}
	sort.SliceStable(options, func(i, j int) bool {
		if options[i].Seat.Machine != options[j].Seat.Machine {
			return options[i].Seat.Machine < options[j].Seat.Machine
		}
		return options[i].ID < options[j].ID
	})
	return options
}

func optionsFromMachine(machine corecap.MachineInventory) []PrimaryAgentOption {
	out := []PrimaryAgentOption{}
	readyProfiles := map[string]bool{}
	compatible := machine.Reachable && machine.ProtocolVersion == corecap.ProtocolVersion &&
		machine.CatalogVersion == corecap.CatalogVersion && machine.ProfileMappingVersion == corecap.ProfileMappingVersion
	contractValid := compatible
	ready := []PrimaryAgentOption{}
	seen := map[string]bool{}
	if compatible {
		for _, offered := range machine.TextOnlyOptions {
			offer, id, err := corecap.NormalizeTextOnlyOptionOffer(offered, machine.Name)
			if err != nil || seen[id] {
				contractValid = false
				ready = nil
				readyProfiles = map[string]bool{}
				break
			}
			seen[id] = true
			display := primaryProfileDisplayName(offer.ProfileID) + " on " + offer.MachineID
			ready = append(ready, PrimaryAgentOption{
				ID: id, State: PrimaryAgentReady, Offer: offer, DisplayName: display,
				Seat: conversation.Seat{
					ID: offer.SeatID, Profile: offer.ProfileID, Agent: offer.AgentKey, Model: offer.RequestedModel,
					Machine: offer.MachineID, DisplayName: display, State: string(corecap.OfferReady),
				},
			})
			readyProfiles[offer.ProfileID] = true
		}
	}
	out = append(out, ready...)
	for _, profile := range machine.Profiles {
		if readyProfiles[profile.ID] {
			continue
		}
		out = append(out, inventoryOptionFromProfile(machine, profile, compatible, contractValid))
	}
	return out
}

func inventoryOptionFromProfile(machine corecap.MachineInventory, profile corecap.ProfileOffer, compatible, contractValid bool) PrimaryAgentOption {
	definition, cataloged := primaryProfileDefinition(profile.ID)
	displayName := primaryProfileDisplayName(profile.ID)
	agent := profile.Agent
	requestedModel := profile.ResolvedModel
	if cataloged {
		agent = definition.Agent
		if definition.Selection.ModelID != "" {
			requestedModel = definition.Selection.ModelID
		}
	}
	display := displayName + " on " + machine.Name
	option := PrimaryAgentOption{
		ID:          "primary-inventory:" + machine.Name + ":" + profile.ID,
		DisplayName: display,
		Seat: conversation.Seat{
			Profile: profile.ID, Agent: agent, Model: requestedModel, Machine: machine.Name,
			DisplayName: display,
		},
	}
	if profile.ID != "codex-subscription:gpt-5.6-sol" {
		option.State = PrimaryAgentIneligible
		option.Reason = PrimaryAgentReasonNotEligible
		option.Seat.State = option.State
		option.Seat.Reason = option.Reason
		return option
	}
	if requestedModel != "" {
		option.Seat.ID = corecap.TextOnlySeatID(profile.ID, machine.Name, requestedModel)
	}
	if !compatible {
		option.State = string(corecap.OfferUnavailable)
		option.Reason = string(machine.Reason)
		if option.Reason == "" {
			if !machine.Reachable {
				option.Reason = string(corecap.ReasonUnavailable)
			} else {
				option.Reason = string(corecap.ReasonOldNode)
			}
		}
	} else if !contractValid {
		option.State = string(corecap.OfferUnavailable)
		option.Reason = string(corecap.ReasonCapabilityDrift)
	} else {
		switch profile.State {
		case corecap.OfferSetupRequired:
			option.State = string(corecap.OfferSetupRequired)
			option.Reason = string(profile.Reason)
		case corecap.OfferUnavailable, corecap.OfferUnknown:
			option.State = string(corecap.OfferUnavailable)
			option.Reason = string(profile.Reason)
		default:
			option.State = PrimaryAgentIneligible
			option.Reason = PrimaryAgentReasonNotEligible
		}
	}
	option.Seat.State = option.State
	option.Seat.Reason = option.Reason
	return option
}

func primaryProfileDefinition(profileID string) (corecap.ProfileDefinition, bool) {
	for _, profile := range corecap.CatalogV2().Profiles {
		if profile.ID == profileID {
			return profile, true
		}
	}
	return corecap.ProfileDefinition{}, false
}

func primaryProfileDisplayName(profileID string) string {
	for _, profile := range corecap.CatalogV2().Profiles {
		if profile.ID == profileID {
			return profile.DisplayName
		}
	}
	return profileID
}

func settingFromOption(option PrimaryAgentOption, updatedAt time.Time) conversation.PrimaryAgentSetting {
	offer := option.Offer
	return conversation.PrimaryAgentSetting{
		OptionID: option.ID, Seat: option.Seat, Authority: conversation.AuthorityChatSubscriptionIsolatedV1,
		Policy: subscriptionPolicyFromOffer(offer), UpdatedAt: updatedAt,
	}
}

func subscriptionPolicyFromOffer(offer corecap.TextOnlyOptionOffer) conversation.SubscriptionPolicy {
	return conversation.SubscriptionPolicy{
		PolicyID: offer.PolicyID, PolicyRevision: offer.PolicyRevision,
		AdapterID: offer.AdapterID, AdapterRevision: offer.AdapterRevision,
		CodexVersion: offer.CodexVersion, CodexExecutableRevision: offer.CodexExecutableRevision,
		CodexSchemaRevision: offer.CodexSchemaRevision, RuntimeContract: offer.RuntimeContract,
		ReasoningEffort: offer.ReasoningEffort, ReasoningContext: offer.ReasoningContext,
		RequestTimeoutMillis: offer.RequestTimeoutMillis, DeveloperInstructionRevision: offer.DeveloperInstructionRevision,
		AccountType: offer.AccountType, AccountPlan: offer.AccountPlan, ThreadMode: offer.ThreadMode,
		SandboxMode: offer.SandboxMode, ApprovalPolicy: offer.ApprovalPolicy, WorkdirMode: offer.WorkdirMode,
		DynamicToolsMode: offer.DynamicToolsMode, MCPMode: offer.MCPMode, CommandPolicy: offer.CommandPolicy,
		FileReadPolicy: offer.FileReadPolicy, IsolationRevision: offer.IsolationRevision,
	}
}

func settingState(setting conversation.PrimaryAgentSetting, options []PrimaryAgentOption) (string, string) {
	for _, option := range options {
		if option.State != PrimaryAgentReady {
			continue
		}
		if option.ID == setting.OptionID && sameSettingIdentity(setting, settingFromOption(option, setting.UpdatedAt)) {
			return PrimaryAgentReady, ""
		}
	}
	for _, option := range options {
		if option.State != PrimaryAgentReady {
			continue
		}
		if option.Seat.ID == setting.Seat.ID ||
			(option.Seat.Profile == setting.Seat.Profile && strings.EqualFold(option.Seat.Machine, setting.Seat.Machine)) {
			return PrimaryAgentDrifted, ErrorPrimaryAgentDrift
		}
	}
	return PrimaryAgentUnready, ErrorPrimaryAgentUnready
}

func sameSettingIdentity(left, right conversation.PrimaryAgentSetting) bool {
	left.UpdatedAt, right.UpdatedAt = time.Time{}, time.Time{}
	left.Seat.State, right.Seat.State = "", ""
	left.Seat.Reason, right.Seat.Reason = "", ""
	return left == right
}

func targetAuthorityFromChannel(channel conversation.PrimaryChannel, model, adapterRevision string) *conversation.TargetAuthority {
	policy := channel.Policy
	policy.AdapterRevision = adapterRevision
	return &conversation.TargetAuthority{Authority: channel.Authority, Policy: policy, RequestedModel: model}
}

func (s *PrimaryChannelService) freshCompatibleOption(
	ctx context.Context,
	machine string,
	participant conversation.Participant,
	channel conversation.PrimaryChannel,
	requiredAdapterRevision string,
) (corecap.TextOnlyOptionOffer, error) {
	if s.capabilities == nil {
		return corecap.TextOnlyOptionOffer{}, primaryError(ErrorPrimaryAgentUnready, errors.New("primary capability inventory is unavailable"))
	}
	row, err := s.capabilities.RefreshMachine(ctx, machine, corecap.RefreshUserRecheck, append([]string(nil), primaryCapabilityAdapters...))
	if err != nil {
		return corecap.TextOnlyOptionOffer{}, primaryError(ErrorPrimaryAgentUnready, err)
	}
	options := optionsFromMachine(row)
	for _, option := range options {
		if compatibleChannelOption(option.Offer, participant, channel) &&
			(requiredAdapterRevision == "" || option.Offer.AdapterRevision == requiredAdapterRevision) {
			return option.Offer, nil
		}
	}
	if len(options) == 0 {
		return corecap.TextOnlyOptionOffer{}, primaryError(ErrorPrimaryAgentUnready, fmt.Errorf("Primary Agent is unavailable on %s", machine))
	}
	return corecap.TextOnlyOptionOffer{}, primaryError(ErrorPrimaryAgentDrift, fmt.Errorf("Primary Agent authority drifted on %s", machine))
}

func compatibleChannelOption(offer corecap.TextOnlyOptionOffer, participant conversation.Participant, channel conversation.PrimaryChannel) bool {
	if participant.ID != channel.ParticipantID || participant.SeatID != offer.SeatID ||
		participant.Profile != offer.ProfileID || participant.Agent != offer.AgentKey ||
		participant.Model != offer.RequestedModel || !strings.EqualFold(participant.Machine, offer.MachineID) {
		return false
	}
	want := subscriptionPolicyFromOffer(offer)
	want.AdapterRevision = channel.Policy.AdapterRevision
	return channel.Authority == conversation.AuthorityChatSubscriptionIsolatedV1 && channel.Policy == want
}

func (s *PrimaryChannelService) nestedTarget(ctx context.Context, channelID, targetID string) (store.ConversationTargetDispatch, error) {
	if _, err := s.GetChannel(ctx, channelID); err != nil {
		return store.ConversationTargetDispatch{}, err
	}
	dispatch, err := s.store.GetConversationTargetDispatch(targetID)
	if err != nil {
		return store.ConversationTargetDispatch{}, err
	}
	if dispatch.Conversation.ID != channelID || dispatch.Target.Authority == nil {
		return store.ConversationTargetDispatch{}, sql.ErrNoRows
	}
	return dispatch, nil
}

func (s *PrimaryChannelService) startTarget(dispatch store.ConversationTargetDispatch, frozenContext string) {
	ctx, cancel := context.WithCancel(s.ctx)
	s.mu.Lock()
	if _, exists := s.starts[dispatch.Target.ID]; exists {
		s.mu.Unlock()
		cancel()
		return
	}
	s.starts[dispatch.Target.ID] = cancel
	s.mu.Unlock()
	s.async.Add(1)
	go func() {
		defer s.async.Done()
		defer func() {
			cancel()
			s.mu.Lock()
			delete(s.starts, dispatch.Target.ID)
			delete(s.active, dispatch.Target.ID)
			s.mu.Unlock()
		}()
		s.runTarget(ctx, dispatch, frozenContext)
	}()
}

func (s *PrimaryChannelService) runTarget(ctx context.Context, dispatch store.ConversationTargetDispatch, frozenContext string) {
	current, err := s.store.GetConversationTargetDispatch(dispatch.Target.ID)
	if err != nil || current.Target.State != conversation.TargetQueued {
		return
	}
	dispatch = current
	if dispatch.Target.Authority == nil {
		s.failQueued(dispatch.Target, errors.New("target lacks subscription authority"), ErrorPrimaryChannelInvariant)
		return
	}
	detail, err := s.GetChannel(ctx, dispatch.Conversation.ID)
	if err != nil {
		s.failQueued(dispatch.Target, err, ErrorPrimaryChannelInvariant)
		return
	}
	_, err = s.freshCompatibleOption(
		ctx, dispatch.Participant.Machine, dispatch.Participant, *detail.PrimaryChannel,
		dispatch.Target.Authority.Policy.AdapterRevision,
	)
	if err != nil {
		code := ErrorCode(err)
		if code == "" {
			code = ErrorPrimaryAgentUnready
		}
		s.failQueued(dispatch.Target, err, code)
		return
	}
	prompt, err := conversation.CompileParticipantPrompt(frozenContext, dispatch.Participant)
	if err != nil {
		s.failQueued(dispatch.Target, err, ErrorProviderFailed)
		return
	}
	runRow := store.Run{
		ID: dispatch.Target.RunID, Title: dispatch.Conversation.Title, Body: prompt,
		Agent: dispatch.Participant.Agent, Profile: dispatch.Participant.Profile, Model: dispatch.Participant.Model,
		Status: "queued", MatchedRule: "primary-channel", Machine: dispatch.Participant.Machine, CreatedAt: s.now().UTC(),
	}
	if err := s.store.CreateRun(runRow); err != nil {
		s.failQueued(dispatch.Target, err, ErrorProviderFailed)
		return
	}
	if s.runtime == nil {
		s.failQueued(dispatch.Target, errors.New("subscription execution plane is unavailable"), ErrorChatPolicyUnavailable)
		return
	}
	spec := runSpecFromDispatch(dispatch, prompt)
	run, err := s.runtime.Dispatch(ctx, spec)
	if err != nil {
		code := ErrorProviderFailed
		if strings.Contains(err.Error(), ErrorChatPolicyUnavailable) {
			code = ErrorChatPolicyUnavailable
		}
		s.failQueued(dispatch.Target, err, code)
		return
	}
	s.mu.Lock()
	s.active[dispatch.Target.ID] = run
	s.mu.Unlock()

	state := conversation.TargetQueued
	message := ""
	var response *coreruntime.ResponseMetadata
	failureCode := ""
	startedCount, exitedCount := 0, 0
	for event := range run.Stream() {
		_, _ = s.store.AppendEvent(store.Event{
			RunID: dispatch.Target.RunID, Type: string(event.Type), Data: event.Data, Code: event.Code, CreatedAt: event.Time,
		})
		if event.Response != nil && event.Type != coreruntime.EventMessage {
			failureCode = ErrorProviderIncomplete
		}
		switch event.Type {
		case coreruntime.EventStarted:
			startedCount++
			if startedCount != 1 || exitedCount != 0 {
				failureCode = ErrorProviderIncomplete
				continue
			}
			if state == conversation.TargetQueued {
				if changed, transitionErr := s.store.TransitionConversationTarget(dispatch.Target.ID, conversation.TargetQueued, conversation.TargetWorking, ""); transitionErr == nil && changed {
					state = conversation.TargetWorking
					_ = s.store.UpdateRunStatus(dispatch.Target.RunID, "running", 0, "")
				}
			}
		case coreruntime.EventMessage:
			if startedCount != 1 || state != conversation.TargetWorking || exitedCount != 0 || message != "" || event.Response == nil {
				failureCode = ErrorProviderIncomplete
			} else {
				message, response = event.Data, event.Response
			}
		case coreruntime.EventError:
			failureCode = closedProviderCode(event.ErrorCode)
		case coreruntime.EventExited:
			exitedCount++
			if exitedCount != 1 || startedCount != 1 {
				failureCode = ErrorProviderIncomplete
			}
		default:
			failureCode = ErrorChatAuthorityViolation
		}
	}
	status := run.Wait()
	latest, latestErr := s.store.GetConversationTargetDispatch(dispatch.Target.ID)
	if latestErr != nil || latest.Target.State == conversation.TargetCanceled || latest.Target.State == conversation.TargetFailed {
		return
	}
	if status.State == coreruntime.StateSucceeded && failureCode == "" && startedCount == 1 && exitedCount == 1 &&
		state == conversation.TargetWorking && strings.TrimSpace(message) != "" && response != nil {
		receipt, receiptErr := targetReceiptFromResponse(dispatch.Target, response)
		if receiptErr == nil {
			changed, answerErr := s.store.AnswerConversationTargetWithReceipt(dispatch.Target.ID, conversation.Message{
				ConversationID: dispatch.Conversation.ID, TurnID: dispatch.Turn.ID, TargetID: dispatch.Target.ID,
				AuthorKind: conversation.AuthorAssistant, AuthorID: dispatch.Participant.ID, Body: message, CreatedAt: s.now().UTC(),
			}, receipt)
			if answerErr == nil && changed {
				_ = s.store.UpdateRunStatus(dispatch.Target.RunID, "succeeded", status.ExitCode, "")
				return
			}
			if answerErr != nil {
				failureCode = ErrorProviderFailed
			}
		} else {
			failureCode = ErrorProviderIncomplete
		}
	}
	if failureCode == "" {
		switch status.State {
		case coreruntime.StateCanceled:
			failureCode = "canceled"
		case coreruntime.StateSucceeded:
			failureCode = ErrorProviderIncomplete
		default:
			failureCode = closedProviderCode(status.Err)
		}
	}
	if failureCode == "canceled" {
		s.cancelTerminal(dispatch.Target, state, status.Err)
		return
	}
	s.failTarget(dispatch.Target, state, failureCode, status.Err, status.ExitCode)
}

func runSpecFromDispatch(dispatch store.ConversationTargetDispatch, prompt string) coreruntime.RunSpec {
	authority := dispatch.Target.Authority
	policy := authority.Policy
	return coreruntime.RunSpec{
		RunID: dispatch.Target.RunID, Profile: dispatch.Participant.Profile, Agent: dispatch.Participant.Agent,
		Model: dispatch.Participant.Model, Prompt: prompt, Machine: dispatch.Participant.Machine,
		Authority: coreruntime.AuthorityMode(authority.Authority), RuntimeContract: policy.RuntimeContract,
		ExpectedPolicyRevision: policy.PolicyRevision,
		TextOnlyPolicy: &coreruntime.TextOnlyPolicy{
			PolicyID: policy.PolicyID, PolicyRevision: policy.PolicyRevision, Model: authority.RequestedModel,
			ReasoningEffort: coreruntime.ReasoningEffort(policy.ReasoningEffort), ReasoningContext: policy.ReasoningContext,
			RequestTimeoutMillis: policy.RequestTimeoutMillis, DeveloperInstructionRevision: policy.DeveloperInstructionRevision,
			AccountType: policy.AccountType, AccountPlan: policy.AccountPlan,
			SelectedAdapterID: policy.AdapterID, SelectedAdapterRevision: policy.AdapterRevision,
			SelectedCodexVersion: policy.CodexVersion, SelectedCodexExecutableRevision: policy.CodexExecutableRevision,
			SelectedCodexSchemaRevision: policy.CodexSchemaRevision, ThreadMode: policy.ThreadMode,
			SandboxMode: policy.SandboxMode, ApprovalPolicy: policy.ApprovalPolicy, WorkdirMode: policy.WorkdirMode,
			DynamicToolsMode: policy.DynamicToolsMode, MCPMode: policy.MCPMode, CommandPolicy: policy.CommandPolicy,
			FileReadPolicy: policy.FileReadPolicy, IsolationRevision: policy.IsolationRevision,
		},
	}
}

func targetReceiptFromResponse(target conversation.Target, response *coreruntime.ResponseMetadata) (conversation.TargetReceipt, error) {
	if target.Authority == nil || response == nil {
		return conversation.TargetReceipt{}, errors.New("subscription response metadata is required")
	}
	if err := response.Validate(); err != nil || response.RequestedModel != target.Authority.RequestedModel {
		return conversation.TargetReceipt{}, errors.New("subscription response metadata does not match selected target")
	}
	receipt := conversation.TargetReceipt{
		ObservedAdapterID: response.ObservedAdapterID, ObservedAdapterRevision: response.ObservedAdapterRevision,
		ObservedCodexVersion:            response.ObservedCodexVersion,
		ObservedCodexExecutableRevision: response.ObservedCodexExecutableRevision,
		ObservedCodexSchemaRevision:     response.ObservedCodexSchemaRevision, ResolvedModel: response.ResolvedModel,
		ProviderThreadID: response.ProviderThreadID, ProviderTerminalStatus: response.TerminalStatus,
		UsageSource: response.UsageSource, InputTokens: response.Usage.InputTokens,
		CachedInputTokens: response.Usage.CachedInputTokens, OutputTokens: response.Usage.OutputTokens,
		ReasoningTokens: response.Usage.ReasoningTokens,
	}
	if err := receipt.ValidateFor(*target.Authority); err != nil {
		return conversation.TargetReceipt{}, err
	}
	return receipt, nil
}

func unknownTargetReceipt(target conversation.Target, terminal string) conversation.TargetReceipt {
	return conversation.TargetReceipt{
		ObservedAdapterID:               conversation.UnknownProviderIdentity,
		ObservedAdapterRevision:         conversation.UnknownProviderIdentity,
		ObservedCodexVersion:            conversation.UnknownProviderIdentity,
		ObservedCodexExecutableRevision: conversation.UnknownProviderIdentity,
		ObservedCodexSchemaRevision:     conversation.UnknownProviderIdentity,
		ResolvedModel:                   conversation.UnknownProviderIdentity,
		ProviderTerminalStatus:          terminal, UsageSource: conversation.UnknownProviderIdentity,
	}
}

func (s *PrimaryChannelService) failQueued(target conversation.Target, cause error, code string) {
	s.failTarget(target, conversation.TargetQueued, code, cause.Error(), -1)
}

func (s *PrimaryChannelService) failTarget(target conversation.Target, state conversation.TargetState, code, message string, exitCode int) {
	if state != conversation.TargetQueued && state != conversation.TargetWorking {
		return
	}
	if message == "" {
		message = code
	}
	changed, err := s.store.TransitionConversationTargetWithReceipt(
		target.ID, state, conversation.TargetFailed, code, message, unknownTargetReceipt(target, code),
	)
	if err == nil && changed {
		_ = s.store.UpdateRunStatus(target.RunID, "failed", exitCode, message)
	}
}

func (s *PrimaryChannelService) cancelTerminal(target conversation.Target, state conversation.TargetState, message string) {
	if state != conversation.TargetQueued && state != conversation.TargetWorking {
		return
	}
	if message == "" {
		message = "provider canceled"
	}
	changed, err := s.store.TransitionConversationTargetWithReceipt(
		target.ID, state, conversation.TargetCanceled, "", message, unknownTargetReceipt(target, "canceled"),
	)
	if err == nil && changed {
		_ = s.store.UpdateRunStatus(target.RunID, "canceled", 0, message)
	}
}

func closedProviderCode(code string) string {
	switch code {
	case ErrorChatPolicyUnavailable, ErrorChatAuthorityViolation, ErrorProviderResultUnknown,
		ErrorProviderIncomplete, ErrorProviderRefusal, ErrorProviderFailed:
		return code
	default:
		return ErrorProviderFailed
	}
}

func nonnilTargets(values []conversation.Target) []conversation.Target {
	if values == nil {
		return []conversation.Target{}
	}
	return values
}

func isLatestAttempt(targets []conversation.Target, selected conversation.Target) bool {
	for _, target := range targets {
		if target.TurnID == selected.TurnID && target.ParticipantID == selected.ParticipantID && target.Attempt > selected.Attempt {
			return false
		}
	}
	return true
}

func latestPrimaryTarget(detail store.ConversationDetail) (conversation.Target, bool) {
	turnRank := make(map[string]int, len(detail.Turns))
	for index, turn := range detail.Turns {
		turnRank[turn.ID] = index
	}
	var latest conversation.Target
	latestRank := -1
	found := false
	for _, target := range detail.Targets {
		rank, known := turnRank[target.TurnID]
		if !known {
			continue
		}
		if !found || rank > latestRank ||
			(rank == latestRank && target.Attempt > latest.Attempt) ||
			(rank == latestRank && target.Attempt == latest.Attempt && target.ID > latest.ID) {
			latest, latestRank, found = target, rank, true
		}
	}
	return latest, found
}

func recoveryActions(code string) []string {
	switch code {
	case "seat_unready", ErrorPrimaryAgentUnready, ErrorPrimaryAgentDrift, ErrorChatPolicyUnavailable:
		return []string{"recheck_and_retry", "retry"}
	case "daemon_interrupted", ErrorProviderResultUnknown, ErrorProviderIncomplete, ErrorProviderFailed:
		return []string{"retry"}
	default:
		return []string{}
	}
}
