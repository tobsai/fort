package control

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/store"
	"github.com/tobsai/fort/ui"
)

// AgentOptionSource is the bounded inventory seam for the agent-first
// product. The accepted Primary option contract remains the default; an
// explicitly supplied source composes additively and cannot weaken the
// service's ready-only enrollment boundary.
type AgentOptionSource interface {
	AgentOptions(context.Context) ([]ui.AgentOption, error)
	RecheckAgentOptions(context.Context) ([]ui.AgentOption, error)
}

type primaryAgentOptionSource struct {
	primary *PrimaryChannelService
}

func (s primaryAgentOptionSource) AgentOptions(context.Context) ([]ui.AgentOption, error) {
	if s.primary == nil {
		return []ui.AgentOption{}, nil
	}
	return agentOptionsFromPrimary(s.primary.currentOptions()), nil
}

func (s primaryAgentOptionSource) RecheckAgentOptions(ctx context.Context) ([]ui.AgentOption, error) {
	if s.primary == nil {
		return []ui.AgentOption{}, primaryError(ErrorPrimaryAgentUnready, fmt.Errorf("agent capability inventory is unavailable"))
	}
	view, err := s.primary.RecheckPrimaryAgent(ctx)
	if err != nil {
		return nil, err
	}
	return agentOptionsFromPrimary(view.Options), nil
}

// AgentChannelService owns the agent-first hierarchy and delegates execution
// for compatible conversations to the existing Primary Channel lifecycle.
// It never consults the singleton Primary Agent setting.
type AgentChannelService struct {
	store   *store.Store
	primary *PrimaryChannelService
	options AgentOptionSource
	now     func() time.Time
}

func NewAgentChannelService(st *store.Store, primary *PrimaryChannelService, options AgentOptionSource) *AgentChannelService {
	if options == nil {
		options = primaryAgentOptionSource{primary: primary}
	} else if primary != nil {
		options = NewCompositeAgentOptionSource(primaryAgentOptionSource{primary: primary}, options)
	}
	return &AgentChannelService{store: st, primary: primary, options: options, now: time.Now}
}

func (s *AgentChannelService) AgentOptions(ctx context.Context) ([]ui.AgentOption, error) {
	items, err := s.options.AgentOptions(ctx)
	if items == nil {
		items = []ui.AgentOption{}
	}
	return items, err
}

func (s *AgentChannelService) RecheckAgentOptions(ctx context.Context) ([]ui.AgentOption, error) {
	items, err := s.options.RecheckAgentOptions(ctx)
	if items == nil {
		items = []ui.AgentOption{}
	}
	return items, err
}

func (s *AgentChannelService) CreateAgentChannel(ctx context.Context, optionID, name string) (ui.AgentChannelDetail, error) {
	name = strings.TrimSpace(name)
	if name == "" || len([]byte(name)) > 120 {
		return ui.AgentChannelDetail{}, fmt.Errorf("Agent Channel name must contain 1 to 120 UTF-8 bytes")
	}
	options, err := s.RecheckAgentOptions(ctx)
	if err != nil {
		return ui.AgentChannelDetail{}, err
	}
	for _, option := range options {
		if option.ID != optionID || option.State != PrimaryAgentReady {
			continue
		}
		id, err := conversation.AgentChannelID(option.Binding)
		if err != nil {
			return ui.AgentChannelDetail{}, err
		}
		channel := conversation.AgentChannel{
			ID: id, Name: name, State: conversation.AgentChannelOpen, OptionID: option.ID,
			Binding: option.Binding, CreatedAt: s.now().UTC(),
		}
		if err := s.store.CreateAgentChannel(channel); err != nil {
			return ui.AgentChannelDetail{}, err
		}
		return s.GetAgentChannel(ctx, channel.ID)
	}
	return ui.AgentChannelDetail{}, primaryError(ErrorPrimaryAgentUnready, fmt.Errorf("agent option %q is not currently ready", optionID))
}

func (s *AgentChannelService) GetAgentChannel(ctx context.Context, id string) (ui.AgentChannelDetail, error) {
	detail, err := s.store.GetAgentChannel(id)
	if err != nil {
		return ui.AgentChannelDetail{}, err
	}
	return s.projectAgentChannel(ctx, detail), nil
}

func (s *AgentChannelService) ListAgentChannels(ctx context.Context, state string) ([]ui.AgentChannelSummary, error) {
	items, err := s.store.ListAgentChannels(state)
	if err != nil {
		return nil, err
	}
	out := make([]ui.AgentChannelSummary, 0, len(items))
	for _, item := range items {
		projected := s.projectAgentChannel(ctx, item)
		open := make([]conversation.AgentConversationSummary, 0, len(projected.Conversations))
		for _, child := range projected.Conversations {
			if child.Conversation.State == conversation.ConversationOpen {
				open = append(open, child)
			}
		}
		projected.Conversations = open
		out = append(out, projected)
	}
	return out, nil
}

func (s *AgentChannelService) RenameAgentChannel(ctx context.Context, id, name string) error {
	name = strings.TrimSpace(name)
	if name == "" || len([]byte(name)) > 120 {
		return fmt.Errorf("Agent Channel name must contain 1 to 120 UTF-8 bytes")
	}
	if _, err := s.GetAgentChannel(ctx, id); err != nil {
		return err
	}
	return s.store.RenameAgentChannel(id, name)
}

func (s *AgentChannelService) SetAgentChannelState(ctx context.Context, id string, state conversation.AgentChannelState) error {
	if state != conversation.AgentChannelOpen && state != conversation.AgentChannelArchived {
		return fmt.Errorf("Agent Channel state must be open or archived")
	}
	if _, err := s.GetAgentChannel(ctx, id); err != nil {
		return err
	}
	return s.store.SetAgentChannelState(id, state)
}

func (s *AgentChannelService) ListAgentConversations(ctx context.Context, channelID, state string) ([]conversation.AgentConversationSummary, error) {
	detail, err := s.GetAgentChannel(ctx, channelID)
	if err != nil {
		return nil, err
	}
	if state != string(conversation.ConversationOpen) && state != string(conversation.ConversationArchived) && state != "all" {
		return nil, fmt.Errorf("Conversation state must be open, archived, or all")
	}
	out := make([]conversation.AgentConversationSummary, 0, len(detail.Conversations))
	for _, item := range detail.Conversations {
		if state == "all" || string(item.Conversation.State) == state {
			out = append(out, item)
		}
	}
	return out, nil
}

func (s *AgentChannelService) GetAgentConversation(ctx context.Context, channelID, conversationID string) (ui.AgentConversationDetail, error) {
	owned, err := s.store.AgentConversationOwned(channelID, conversationID)
	if err != nil {
		return ui.AgentConversationDetail{}, err
	}
	if !owned {
		return ui.AgentConversationDetail{}, sql.ErrNoRows
	}
	parent, err := s.GetAgentChannel(ctx, channelID)
	if err != nil {
		return ui.AgentConversationDetail{}, err
	}
	detail, err := s.store.GetConversation(conversationID)
	if err != nil {
		return ui.AgentConversationDetail{}, err
	}
	if len(detail.Participants) != 1 || detail.Participants[0].State != conversation.ParticipantActive ||
		!participantMatchesAgentBinding(detail.Participants[0], parent.Channel.Binding) {
		return ui.AgentConversationDetail{}, primaryError(ErrorPrimaryChannelInvariant, fmt.Errorf("agent_channel_invariant: conversation %s", conversationID))
	}
	out := ui.AgentConversationDetail{
		ChannelID: channelID, Conversation: detail.Conversation, Participant: detail.Participants[0],
		Messages: nonnilMessages(detail.Messages), Turns: nonnilTurns(detail.Turns), Targets: nonnilTargets(detail.Targets),
		Readiness: parent.Readiness, Binding: parent.Channel.Binding,
	}
	for _, item := range parent.Conversations {
		if item.Conversation.ID == conversationID {
			out.Pinned, out.PinnedAt = item.Pinned, item.PinnedAt
			break
		}
	}
	return out, nil
}

func (s *AgentChannelService) CreateAgentConversation(ctx context.Context, channelID, name string) (ui.AgentConversationDetail, error) {
	name = strings.TrimSpace(name)
	if name == "" || len([]byte(name)) > 120 {
		return ui.AgentConversationDetail{}, fmt.Errorf("Conversation name must contain 1 to 120 UTF-8 bytes")
	}
	parent, err := s.GetAgentChannel(ctx, channelID)
	if err != nil {
		return ui.AgentConversationDetail{}, err
	}
	if parent.Channel.State != conversation.AgentChannelOpen {
		return ui.AgentConversationDetail{}, primaryError(ErrorAgentChannelState, fmt.Errorf("archived Agent Channel %s must be reopened before creating a Conversation", channelID))
	}
	now := s.now().UTC()
	id := uuid.NewString()
	item := conversation.Conversation{ID: id, Title: name, State: conversation.ConversationOpen, CreatedAt: now, UpdatedAt: now}
	if err := s.store.CreateAgentChannelConversation(channelID, item, uuid.NewString()); err != nil {
		return ui.AgentConversationDetail{}, agentChannelStoreError(err)
	}
	return s.GetAgentConversation(ctx, channelID, id)
}

func agentChannelStoreError(err error) error {
	if errors.Is(err, store.ErrAgentChannelState) {
		return primaryError(ErrorAgentChannelState, err)
	}
	return err
}

func (s *AgentChannelService) RenameAgentConversation(ctx context.Context, channelID, conversationID, name string) error {
	name = strings.TrimSpace(name)
	if name == "" || len([]byte(name)) > 120 {
		return fmt.Errorf("Conversation name must contain 1 to 120 UTF-8 bytes")
	}
	if _, err := s.GetAgentConversation(ctx, channelID, conversationID); err != nil {
		return err
	}
	return s.store.RenameConversation(conversationID, name)
}

func (s *AgentChannelService) SetAgentConversationState(ctx context.Context, channelID, conversationID string, state conversation.ConversationState) error {
	if state != conversation.ConversationOpen && state != conversation.ConversationArchived {
		return fmt.Errorf("Conversation state must be open or archived")
	}
	if _, err := s.GetAgentConversation(ctx, channelID, conversationID); err != nil {
		return err
	}
	return s.store.SetConversationState(conversationID, state)
}

func (s *AgentChannelService) SetAgentConversationPinned(ctx context.Context, channelID, conversationID string, pinned bool) error {
	if _, err := s.GetAgentConversation(ctx, channelID, conversationID); err != nil {
		return err
	}
	return s.store.SetAgentConversationPinned(channelID, conversationID, pinned, s.now().UTC())
}

func (s *AgentChannelService) PostAgentTurn(ctx context.Context, channelID, conversationID, clientTurnID, text string) (conversation.TurnResult, error) {
	parent, err := s.GetAgentChannel(ctx, channelID)
	if err != nil {
		return conversation.TurnResult{}, err
	}
	if parent.Channel.State != conversation.AgentChannelOpen {
		return conversation.TurnResult{}, primaryError(ErrorAgentChannelState, fmt.Errorf("archived Agent Channel %s must be reopened before sending", channelID))
	}
	detail, err := s.GetAgentConversation(ctx, channelID, conversationID)
	if err != nil {
		return conversation.TurnResult{}, err
	}
	if detail.Conversation.State != conversation.ConversationOpen {
		return conversation.TurnResult{}, primaryError(ErrorAgentChannelState, fmt.Errorf("archived Conversation %s must be reopened before sending", conversationID))
	}
	if s.primary == nil {
		return conversation.TurnResult{}, primaryError(ErrorChatPolicyUnavailable, fmt.Errorf("Agent Channel has no approved execution adapter"))
	}
	result, err := s.primary.postTurn(
		ctx,
		conversationID,
		clientTurnID,
		text,
		parent.Channel.Binding.Authority.AdapterRevision,
		channelID,
	)
	return result, agentChannelStoreError(err)
}

func (s *AgentChannelService) PostFirstAgentTurn(ctx context.Context, channelID, name, clientTurnID, text string) (ui.AgentFirstTurnResult, error) {
	name = strings.TrimSpace(name)
	text = strings.TrimSpace(text)
	if name == "" || len([]byte(name)) > 120 {
		return ui.AgentFirstTurnResult{}, fmt.Errorf("Conversation name must contain 1 to 120 UTF-8 bytes")
	}
	if clientTurnID == "" {
		return ui.AgentFirstTurnResult{}, fmt.Errorf("client_turn_id is required")
	}
	if text == "" {
		return ui.AgentFirstTurnResult{}, fmt.Errorf("message text is required")
	}
	conversationID := uuid.NewSHA1(uuid.NameSpaceURL, []byte("fort:agent-first:"+channelID+":"+clientTurnID)).String()
	owned, err := s.store.AgentConversationOwned(channelID, conversationID)
	if err != nil {
		return ui.AgentFirstTurnResult{}, err
	}
	if owned {
		turn, targets, found, findErr := s.store.FindConversationTurnByClientID(conversationID, clientTurnID)
		if findErr != nil {
			return ui.AgentFirstTurnResult{}, findErr
		}
		if !found {
			return ui.AgentFirstTurnResult{}, primaryError(ErrorPrimaryChannelInvariant, fmt.Errorf("atomic first Conversation %s has no turn %s", conversationID, clientTurnID))
		}
		detail, detailErr := s.GetAgentConversation(ctx, channelID, conversationID)
		if detailErr != nil {
			return ui.AgentFirstTurnResult{}, detailErr
		}
		return ui.AgentFirstTurnResult{Conversation: detail, Turn: turn, Targets: nonnilTargets(targets)}, nil
	}
	parent, err := s.GetAgentChannel(ctx, channelID)
	if err != nil {
		return ui.AgentFirstTurnResult{}, err
	}
	if parent.Channel.State != conversation.AgentChannelOpen {
		return ui.AgentFirstTurnResult{}, primaryError(ErrorAgentChannelState, fmt.Errorf("archived Agent Channel %s must be reopened before sending", channelID))
	}
	if s.primary == nil {
		return ui.AgentFirstTurnResult{}, primaryError(ErrorChatPolicyUnavailable, fmt.Errorf("Agent Channel has no approved execution adapter"))
	}
	participantID := uuid.NewString()
	participant, primaryChannel, adapterRevision, err := s.approvedPrimaryTarget(ctx, parent.Channel, conversationID, participantID)
	if err != nil {
		return ui.AgentFirstTurnResult{}, err
	}
	authority := targetAuthorityFromChannel(primaryChannel, participant.Model, adapterRevision)
	now := s.now().UTC()
	turn, targets, prompt, err := s.store.CreateAgentChannelConversationTurn(store.CreateAgentChannelConversationTurnParams{
		ChannelID: channelID,
		Conversation: conversation.Conversation{
			ID: conversationID, Title: name, State: conversation.ConversationOpen, CreatedAt: now, UpdatedAt: now,
		},
		ParticipantID: participantID, TurnID: uuid.NewString(), ClientTurnID: clientTurnID,
		TargetID: uuid.NewString(), RunID: uuid.NewString(), HumanID: "human", Body: text,
		Authority: authority, CreatedAt: now,
	})
	if err != nil {
		return ui.AgentFirstTurnResult{}, agentChannelStoreError(err)
	}
	if turn.Created && len(targets) == 1 {
		dispatch, dispatchErr := s.store.GetConversationTargetDispatch(targets[0].ID)
		if dispatchErr != nil {
			s.primary.failQueued(targets[0], dispatchErr, ErrorProviderFailed)
		} else {
			s.primary.startTarget(dispatch, prompt)
		}
	}
	detail, err := s.GetAgentConversation(ctx, channelID, conversationID)
	if err != nil {
		return ui.AgentFirstTurnResult{}, err
	}
	return ui.AgentFirstTurnResult{Conversation: detail, Turn: turn, Targets: nonnilTargets(targets)}, nil
}

func (s *AgentChannelService) RetryAgentTarget(ctx context.Context, channelID, conversationID, targetID string) (conversation.Target, error) {
	parent, err := s.GetAgentChannel(ctx, channelID)
	if err != nil {
		return conversation.Target{}, err
	}
	if parent.Channel.State != conversation.AgentChannelOpen {
		return conversation.Target{}, primaryError(ErrorAgentChannelState, fmt.Errorf("archived Agent Channel %s must be reopened before retry", channelID))
	}
	detail, err := s.GetAgentConversation(ctx, channelID, conversationID)
	if err != nil {
		return conversation.Target{}, err
	}
	if detail.Conversation.State != conversation.ConversationOpen {
		return conversation.Target{}, primaryError(ErrorAgentChannelState, fmt.Errorf("archived Conversation %s must be reopened before retry", conversationID))
	}
	var selected conversation.Target
	found := false
	for _, target := range detail.Targets {
		if target.ID == targetID {
			selected, found = target, true
			break
		}
	}
	if !found {
		return conversation.Target{}, sql.ErrNoRows
	}
	if !containsRecoveryAction(recoveryActions(selected.ErrorCode), "retry") {
		return conversation.Target{}, primaryError(
			ErrorAgentRecoveryUnavailable,
			fmt.Errorf("target %s failure %q has no approved retry action", targetID, selected.ErrorCode),
		)
	}
	if s.primary == nil {
		return conversation.Target{}, primaryError(ErrorChatPolicyUnavailable, fmt.Errorf("Agent Channel has no approved execution adapter"))
	}
	target, err := s.primary.retryTarget(
		ctx,
		conversationID,
		targetID,
		parent.Channel.Binding.Authority.AdapterRevision,
		channelID,
	)
	return target, agentChannelStoreError(err)
}

func containsRecoveryAction(actions []string, want string) bool {
	for _, action := range actions {
		if action == want {
			return true
		}
	}
	return false
}

func (s *AgentChannelService) CancelAgentTarget(ctx context.Context, channelID, conversationID, targetID string) error {
	if _, err := s.GetAgentConversation(ctx, channelID, conversationID); err != nil {
		return err
	}
	if s.primary == nil {
		return primaryError(ErrorChatPolicyUnavailable, fmt.Errorf("Agent Channel has no approved execution adapter"))
	}
	return s.primary.CancelTarget(ctx, conversationID, targetID)
}

func (s *AgentChannelService) AgentNeedsYou(ctx context.Context) ([]ui.AgentNeedsYouItem, error) {
	channels, err := s.ListAgentChannels(ctx, string(conversation.AgentChannelOpen))
	if err != nil {
		return nil, err
	}
	items := []ui.AgentNeedsYouItem{}
	for _, channel := range channels {
		for _, child := range channel.Conversations {
			if child.Conversation.State != conversation.ConversationOpen {
				continue
			}
			detail, detailErr := s.GetAgentConversation(ctx, channel.Channel.ID, child.Conversation.ID)
			if detailErr != nil {
				return nil, detailErr
			}
			latest, ok := latestPrimaryTarget(store.ConversationDetail{Turns: detail.Turns, Targets: detail.Targets})
			if !ok || latest.State != conversation.TargetFailed {
				continue
			}
			actions := recoveryActions(latest.ErrorCode)
			if len(actions) > 0 {
				items = append(items, ui.AgentNeedsYouItem{
					AgentChannel: channel.Channel, Conversation: child.Conversation,
					Target: latest, Actions: actions,
				})
			}
		}
	}
	return items, nil
}

func (s *AgentChannelService) approvedPrimaryTarget(
	ctx context.Context,
	channel conversation.AgentChannel,
	conversationID string,
	participantID string,
) (conversation.Participant, conversation.PrimaryChannel, string, error) {
	for _, option := range s.primary.currentOptions() {
		if option.State != PrimaryAgentReady {
			continue
		}
		candidateID, idErr := conversation.AgentChannelID(agentBindingFromPrimaryOption(option))
		if idErr != nil || candidateID != channel.ID {
			continue
		}
		setting := settingFromOption(option, s.now().UTC())
		participant := conversation.Participant{
			ID: participantID, ConversationID: conversationID, SeatID: setting.Seat.ID,
			Profile: setting.Seat.Profile, Agent: setting.Seat.Agent, Model: setting.Seat.Model,
			Machine: setting.Seat.Machine, DisplayName: channel.Name, State: conversation.ParticipantActive,
		}
		primaryChannel := conversation.PrimaryChannel{
			ConversationID: conversationID, ParticipantID: participantID,
			Authority: setting.Authority, Policy: setting.Policy, CreatedAt: s.now().UTC(),
		}
		fresh, err := s.primary.freshCompatibleOption(
			ctx,
			participant.Machine,
			participant,
			primaryChannel,
			channel.Binding.Authority.AdapterRevision,
		)
		if err != nil {
			return conversation.Participant{}, conversation.PrimaryChannel{}, "", err
		}
		return participant, primaryChannel, fresh.AdapterRevision, nil
	}
	return conversation.Participant{}, conversation.PrimaryChannel{}, "", primaryError(
		ErrorPrimaryAgentDrift, fmt.Errorf("Agent Channel %s no longer matches an approved option", channel.ID),
	)
}

func (s *AgentChannelService) projectAgentChannel(ctx context.Context, detail conversation.AgentChannelDetail) ui.AgentChannelDetail {
	return ui.AgentChannelDetail{
		Channel: detail.Channel, Conversations: nonnilAgentConversations(detail.Conversations),
		Readiness: s.agentChannelReadiness(ctx, detail.Channel),
	}
}

func (s *AgentChannelService) agentChannelReadiness(ctx context.Context, channel conversation.AgentChannel) ui.PrimaryChannelReadiness {
	readiness := ui.PrimaryChannelReadiness{State: PrimaryAgentUnready, Reason: ErrorPrimaryAgentUnready}
	options, err := s.AgentOptions(ctx)
	if err != nil {
		return readiness
	}
	for _, option := range options {
		optionChannelID, idErr := conversation.AgentChannelID(option.Binding)
		if idErr != nil || optionChannelID != channel.ID {
			continue
		}
		if option.State == PrimaryAgentReady {
			return ui.PrimaryChannelReadiness{State: PrimaryAgentReady}
		}
		return ui.PrimaryChannelReadiness{State: option.State, Reason: option.Reason}
	}
	for _, option := range options {
		if option.ID != channel.OptionID {
			continue
		}
		optionChannelID, idErr := conversation.AgentChannelID(option.Binding)
		if idErr == nil && optionChannelID == channel.ID && option.State == PrimaryAgentReady {
			return ui.PrimaryChannelReadiness{State: PrimaryAgentReady}
		}
		readiness.State = option.State
		readiness.Reason = option.Reason
		if option.State == PrimaryAgentReady {
			readiness.State = PrimaryAgentDrifted
			readiness.Reason = ErrorPrimaryAgentDrift
		}
		return readiness
	}
	for _, option := range options {
		if !sameAgentSeat(option.Binding.Seat, channel.Binding.Seat) {
			continue
		}
		readiness.State = option.State
		readiness.Reason = option.Reason
		if option.State == PrimaryAgentReady {
			readiness.State = PrimaryAgentDrifted
			readiness.Reason = ErrorPrimaryAgentDrift
		}
		return readiness
	}
	return readiness
}

func sameAgentSeat(left, right conversation.AgentSeatIdentity) bool {
	if left.ID != "" && right.ID != "" {
		return left.ID == right.ID
	}
	return left.Profile == right.Profile && left.Agent == right.Agent &&
		left.Model == right.Model && strings.EqualFold(left.Machine, right.Machine)
}

func agentOptionsFromPrimary(options []ui.PrimaryAgentOption) []ui.AgentOption {
	out := make([]ui.AgentOption, 0, len(options))
	for _, option := range options {
		out = append(out, ui.AgentOption{
			ID: option.ID, State: option.State, Reason: option.Reason, DisplayName: option.DisplayName,
			Binding: agentBindingFromPrimaryOption(option),
		})
	}
	return out
}

func agentBindingFromPrimaryOption(option ui.PrimaryAgentOption) conversation.AgentBinding {
	offer := option.Offer
	execution := map[string]string{}
	put := func(key, value string) {
		if value != "" {
			execution[key] = value
		}
	}
	put("request_timeout_millis", strconv.Itoa(offer.RequestTimeoutMillis))
	put("codex_version", offer.CodexVersion)
	put("codex_executable_revision", offer.CodexExecutableRevision)
	put("codex_schema_revision", offer.CodexSchemaRevision)
	put("reasoning_effort", offer.ReasoningEffort)
	put("reasoning_context", offer.ReasoningContext)
	put("developer_instruction_revision", offer.DeveloperInstructionRevision)
	put("account_type", offer.AccountType)
	put("account_plan", offer.AccountPlan)
	put("sandbox_mode", offer.SandboxMode)
	put("approval_policy", offer.ApprovalPolicy)
	put("workdir_mode", offer.WorkdirMode)
	put("dynamic_tools_mode", offer.DynamicToolsMode)
	put("mcp_mode", offer.MCPMode)
	put("command_policy", offer.CommandPolicy)
	put("file_read_policy", offer.FileReadPolicy)
	put("isolation_revision", offer.IsolationRevision)
	return conversation.AgentBinding{
		Seat: conversation.AgentSeatIdentity{
			ID: option.Seat.ID, Profile: option.Seat.Profile, Agent: option.Seat.Agent,
			Model: option.Seat.Model, Machine: option.Seat.Machine,
		},
		Authority: conversation.AgentAuthoritySnapshot{
			RequestedModel: offer.RequestedModel, ResolvedModel: offer.ResolvedModel,
			Authority: conversation.AuthorityChatSubscriptionIsolatedV1,
			PolicyID:  offer.PolicyID, PolicyRevision: offer.PolicyRevision,
			AdapterID: offer.AdapterID, AdapterRevision: offer.AdapterRevision,
			RuntimeContract: offer.RuntimeContract, SessionMode: offer.ThreadMode,
			MemoryMode: conversation.AgentMemoryEphemeral, ExecutionPolicy: execution,
		},
	}
}

func nonnilAgentConversations(items []conversation.AgentConversationSummary) []conversation.AgentConversationSummary {
	if items == nil {
		return []conversation.AgentConversationSummary{}
	}
	return items
}

func nonnilMessages(items []conversation.Message) []conversation.Message {
	if items == nil {
		return []conversation.Message{}
	}
	return items
}

func nonnilTurns(items []conversation.Turn) []conversation.Turn {
	if items == nil {
		return []conversation.Turn{}
	}
	return items
}

func participantMatchesAgentBinding(participant conversation.Participant, binding conversation.AgentBinding) bool {
	seat := binding.Seat
	return participant.SeatID == seat.ID && participant.Profile == seat.Profile &&
		participant.Agent == seat.Agent && participant.Model == seat.Model &&
		strings.EqualFold(participant.Machine, seat.Machine)
}
