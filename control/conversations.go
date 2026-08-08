package control

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	corecap "github.com/tobsai/fort/core/capability"
	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/runtime"
	"github.com/tobsai/fort/core/store"
	execcap "github.com/tobsai/fort/exec/capability"
	"github.com/tobsai/fort/ui"
)

type ConversationSeatSource interface {
	ConversationSeats() []conversation.Seat
}

type fixedReadySeats []conversation.Seat

func (s fixedReadySeats) ConversationSeats() []conversation.Seat {
	return append([]conversation.Seat(nil), s...)
}

// FakeConversationSeats is test/demo plumbing for FORT_FAKE only. Production
// composition must use SnapshotConversationSeats so static claims never become
// ready seats.
func FakeConversationSeats(machine string) ConversationSeatSource {
	if machine == "" {
		machine = "local"
	}
	catalog := corecap.CatalogV2()
	seats := make(fixedReadySeats, 0, len(catalog.Profiles))
	for _, definition := range catalog.Profiles {
		agent, model, ok := catalog.RuntimeSelection(definition.ID)
		if !ok || model == "" {
			continue
		}
		seats = append(seats, conversation.Seat{
			ID: conversationSeatID(definition.ID, machine, model), Profile: definition.ID, Agent: agent, Model: model,
			Machine: machine, DisplayName: definition.DisplayName + " on " + machine, State: string(corecap.OfferReady),
		})
	}
	return seats
}

type CapabilitySnapshotSource interface {
	Capabilities() (corecap.Snapshot, uint64)
}

// SnapshotConversationSeats projects the verified capability inventory into
// exact profile + provider/model + machine seats.
type SnapshotConversationSeats struct {
	Source CapabilitySnapshotSource
}

func (s SnapshotConversationSeats) ConversationSeats() []conversation.Seat {
	if s.Source == nil {
		return []conversation.Seat{}
	}
	snapshot, generation := s.Source.Capabilities()
	if generation == 0 {
		return []conversation.Seat{}
	}
	catalog := corecap.CatalogV2()
	definitions := make(map[string]corecap.ProfileDefinition, len(catalog.Profiles))
	for _, definition := range catalog.Profiles {
		definitions[definition.ID] = definition
	}
	out := []conversation.Seat{}
	for _, machine := range snapshot.Machines {
		for _, offer := range machine.Profiles {
			definition, defined := definitions[offer.ID]
			if !defined {
				continue
			}
			agent, model, ok := catalog.RuntimeSelection(offer.ID)
			if !ok {
				continue
			}
			state, reason := string(offer.State), string(offer.Reason)
			displayName := definition.DisplayName
			if definition.RequiresResolvedModel() {
				model = offer.ResolvedModel
				displayName = dynamicConversationSeatDisplayName(catalog, definition, model)
				if offer.State == corecap.OfferReady && model == "" {
					state, reason = string(corecap.OfferSetupRequired), string(corecap.ReasonModelUnavailable)
				}
			}
			seatID := conversationSeatID(offer.ID, machine.Name, model)
			if !machine.Reachable {
				state, reason = string(corecap.OfferUnavailable), string(corecap.ReasonUnavailable)
			}
			out = append(out, conversation.Seat{
				ID: seatID, Profile: offer.ID, Agent: agent, Model: model,
				Machine: machine.Name, DisplayName: displayName + " on " + machine.Name,
				State: state, Reason: reason,
			})
		}
	}
	return out
}

func conversationSeatID(profile, machine, model string) string {
	digest := sha256.Sum256([]byte(profile + "\x00" + machine + "\x00" + model))
	return fmt.Sprintf("seat:v1:%x", digest)
}

func dynamicConversationSeatDisplayName(catalog corecap.Catalog, definition corecap.ProfileDefinition, model string) string {
	if model == "" {
		return definition.DisplayName
	}
	agentName := strings.SplitN(definition.DisplayName, " · ", 2)[0]
	for _, candidate := range catalog.Profiles {
		if candidate.Agent != definition.Agent || candidate.RequiresResolvedModel() {
			continue
		}
		_, candidateModel, ok := catalog.RuntimeSelection(candidate.ID)
		if !ok || candidateModel != model {
			continue
		}
		modelName := strings.TrimPrefix(candidate.DisplayName, agentName+" · ")
		return agentName + " · " + modelName + " (default)"
	}
	return agentName + " · " + model + " (default)"
}

type CreateConversationRequest struct {
	ProjectID string   `json:"project_id,omitempty"`
	Title     string   `json:"title"`
	SeatIDs   []string `json:"seat_ids"`
}

type PostConversationTurnRequest struct {
	Text                 string   `json:"text"`
	TargetParticipantIDs []string `json:"target_participant_ids"`
}

// ConversationService is the durable coordinator for projects, shared
// conversations, target fan-out, attribution, retry, and cancellation.
type ConversationService struct {
	store   *store.Store
	runtime runtime.Runtime
	seats   ConversationSeatSource
	workdir string
	now     func() time.Time
	ctx     context.Context
	cancel  context.CancelFunc

	mu       sync.Mutex
	active   map[string]runtime.Run
	starting map[string]*conversationTargetStartup
	async    sync.WaitGroup
}

var _ ui.ConversationPort = (*ConversationService)(nil)

type conversationTargetStartup struct {
	mu      sync.Mutex
	claimed bool
	ctx     context.Context
	cancel  context.CancelFunc
}

func NewConversationService(st *store.Store, rt runtime.Runtime, seats ConversationSeatSource, workdir string) *ConversationService {
	ctx, cancel := context.WithCancel(context.Background())
	return &ConversationService{
		store: st, runtime: rt, seats: seats, workdir: workdir, now: time.Now, ctx: ctx, cancel: cancel,
		active: map[string]runtime.Run{}, starting: map[string]*conversationTargetStartup{},
	}
}

func (s *ConversationService) ConversationSeats(context.Context) ([]conversation.Seat, error) {
	if s.seats == nil {
		return []conversation.Seat{}, nil
	}
	return s.seats.ConversationSeats(), nil
}

func (s *ConversationService) ListProjects(context.Context) ([]conversation.Project, error) {
	return s.store.ListProjects()
}

func (s *ConversationService) CreateProject(_ context.Context, name string) (conversation.Project, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return conversation.Project{}, fmt.Errorf("project name is required")
	}
	now := s.now().UTC()
	project := conversation.Project{ID: uuid.NewString(), Name: name, CreatedAt: now, UpdatedAt: now}
	return project, s.store.CreateProject(project)
}

func (s *ConversationService) RenameProject(_ context.Context, id, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("project name is required")
	}
	return s.store.RenameProject(id, name)
}

func (s *ConversationService) DeleteProject(_ context.Context, id string) error {
	return s.store.DeleteProject(id)
}

func (s *ConversationService) ListConversations(_ context.Context, scope string) ([]conversation.Conversation, error) {
	return s.store.ListConversations(scope)
}

func (s *ConversationService) GetConversation(_ context.Context, id string) (ui.ConversationDetail, error) {
	detail, err := s.store.GetConversation(id)
	if err != nil {
		return ui.ConversationDetail{}, err
	}
	return toUIConversationDetail(detail), nil
}

func (s *ConversationService) CreateConversation(_ context.Context, projectID, title string, seatIDs []string) (ui.ConversationDetail, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return ui.ConversationDetail{}, fmt.Errorf("conversation title is required")
	}
	if len(seatIDs) == 0 {
		return ui.ConversationDetail{}, fmt.Errorf("at least one agent seat is required")
	}
	available := s.seatMap()
	seen := map[string]bool{}
	now := s.now().UTC()
	id := uuid.NewString()
	participants := make([]conversation.Participant, 0, len(seatIDs))
	for position, seatID := range seatIDs {
		if seen[seatID] {
			return ui.ConversationDetail{}, fmt.Errorf("agent seat %q was selected more than once", seatID)
		}
		seen[seatID] = true
		seat, ok := available[seatID]
		if !ok {
			return ui.ConversationDetail{}, conversation.NewBoundedError(conversation.ErrorSeatUnknown, fmt.Errorf("agent seat %q is not in the current inventory", seatID))
		}
		if seat.State != string(corecap.OfferReady) {
			return ui.ConversationDetail{}, conversation.NewBoundedError(conversation.ErrorSeatUnready, fmt.Errorf("agent seat %q is not ready: %s", seatID, closedSeatReason(seat)))
		}
		participants = append(participants, conversation.Participant{
			ID: uuid.NewString(), ConversationID: id, SeatID: seat.ID, Profile: seat.Profile,
			Agent: seat.Agent, Model: seat.Model, Machine: seat.Machine, DisplayName: seat.DisplayName,
			Position: position, State: conversation.ParticipantActive, CreatedAt: now,
		})
	}
	item := conversation.Conversation{ID: id, ProjectID: projectID, Title: title, CreatedAt: now, UpdatedAt: now}
	if err := s.store.CreateConversation(item, participants); err != nil {
		return ui.ConversationDetail{}, err
	}
	detail, err := s.store.GetConversation(id)
	if err != nil {
		return ui.ConversationDetail{}, err
	}
	return toUIConversationDetail(detail), nil
}

func toUIConversationDetail(detail store.ConversationDetail) ui.ConversationDetail {
	return ui.ConversationDetail{
		Conversation: detail.Conversation,
		Participants: detail.Participants,
		Messages:     detail.Messages,
		Turns:        detail.Turns,
		Targets:      detail.Targets,
	}
}

func (s *ConversationService) MoveConversation(_ context.Context, id, projectID string) error {
	return s.store.MoveConversation(id, projectID)
}

func (s *ConversationService) AddConversationParticipant(_ context.Context, conversationID, seatID string) (conversation.Participant, error) {
	detail, err := s.store.GetConversation(conversationID)
	if err != nil {
		return conversation.Participant{}, err
	}
	seat, ok := s.seatMap()[seatID]
	if !ok {
		return conversation.Participant{}, conversation.NewBoundedError(conversation.ErrorSeatUnknown, fmt.Errorf("agent seat %q is not in the current inventory", seatID))
	}
	if seat.State != string(corecap.OfferReady) {
		return conversation.Participant{}, conversation.NewBoundedError(conversation.ErrorSeatUnready, fmt.Errorf("agent seat %q is not ready: %s", seatID, closedSeatReason(seat)))
	}
	for _, participant := range detail.Participants {
		if participant.SeatID == seatID {
			return conversation.Participant{}, fmt.Errorf("agent seat %q already participates in conversation %s", seatID, conversationID)
		}
	}
	participant := conversation.Participant{
		ID: uuid.NewString(), ConversationID: conversationID, SeatID: seat.ID,
		Profile: seat.Profile, Agent: seat.Agent, Model: seat.Model, Machine: seat.Machine,
		DisplayName: seat.DisplayName, Position: len(detail.Participants), State: conversation.ParticipantActive, CreatedAt: s.now().UTC(),
	}
	if err := s.store.AddConversationParticipant(participant); err != nil {
		return conversation.Participant{}, err
	}
	return participant, nil
}

func (s *ConversationService) RenameConversation(_ context.Context, id, title string) error {
	title = strings.TrimSpace(title)
	if title == "" {
		return fmt.Errorf("conversation title is required")
	}
	return s.store.RenameConversation(id, title)
}

func (s *ConversationService) SetConversationState(_ context.Context, id string, state conversation.ConversationState) error {
	if state != conversation.ConversationOpen && state != conversation.ConversationArchived {
		return fmt.Errorf("conversation state must be open or archived")
	}
	return s.store.SetConversationState(id, state)
}

func (s *ConversationService) RemoveConversationParticipant(_ context.Context, conversationID, participantID string) error {
	return s.store.RemoveConversationParticipant(conversationID, participantID, s.now().UTC())
}

func (s *ConversationService) DeleteConversation(_ context.Context, id string) error {
	err := s.store.DeleteConversation(id)
	if errors.Is(err, conversation.ErrConversationActive) {
		return conversation.NewBoundedError(conversation.ErrorConversationActive, err)
	}
	return err
}

func (s *ConversationService) PostTurn(_ context.Context, conversationID, clientTurnID, text string, targetParticipantIDs []string) (conversation.TurnResult, error) {
	if strings.TrimSpace(clientTurnID) == "" {
		return conversation.TurnResult{}, fmt.Errorf("client_turn_id is required")
	}
	if turn, targets, found, err := s.store.FindConversationTurnByClientID(conversationID, clientTurnID); err != nil {
		return conversation.TurnResult{}, err
	} else if found {
		return conversation.TurnResult{Turn: turn, Targets: targets}, nil
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return conversation.TurnResult{}, fmt.Errorf("message text is required")
	}
	detail, err := s.store.GetConversation(conversationID)
	if err != nil {
		return conversation.TurnResult{}, err
	}
	participants := make(map[string]conversation.Participant, len(detail.Participants))
	for _, participant := range detail.Participants {
		participants[participant.ID] = participant
	}
	if len(targetParticipantIDs) == 0 {
		return conversation.TurnResult{}, fmt.Errorf("at least one target is required")
	}
	seen := map[string]bool{}
	requested := make([]store.ConversationTurnTarget, 0, len(targetParticipantIDs))
	for _, participantID := range targetParticipantIDs {
		if seen[participantID] {
			return conversation.TurnResult{}, fmt.Errorf("participant %q was targeted more than once", participantID)
		}
		seen[participantID] = true
		participant, ok := participants[participantID]
		if !ok {
			return conversation.TurnResult{}, conversation.NewBoundedError(conversation.ErrorParticipantUnknown, fmt.Errorf("participant %q is not in conversation %s", participantID, conversationID))
		}
		if participant.State != conversation.ParticipantActive {
			return conversation.TurnResult{}, conversation.NewBoundedError(conversation.ErrorParticipantRemoved, fmt.Errorf("participant %q is removed", participantID))
		}
		requested = append(requested, store.ConversationTurnTarget{ID: uuid.NewString(), ParticipantID: participantID, RunID: uuid.NewString()})
	}
	now := s.now().UTC()
	turn, targets, prompt, err := s.store.CreateConversationTurn(store.CreateConversationTurnParams{
		TurnID: uuid.NewString(), ClientTurnID: clientTurnID, ConversationID: conversationID, HumanID: "human", Body: text,
		Targets: requested, CreatedAt: now,
	})
	if err != nil {
		return conversation.TurnResult{}, err
	}
	if !turn.Created {
		return conversation.TurnResult{Turn: turn, Targets: targets}, nil
	}
	for _, target := range targets {
		dispatch, dispatchErr := s.store.GetConversationTargetDispatch(target.ID)
		if dispatchErr != nil {
			s.failQueuedTarget(target.ID, target.RunID, dispatchErr)
			continue
		}
		s.startTarget(dispatch, prompt)
	}
	return conversation.TurnResult{Turn: turn, Targets: targets}, nil
}

func (s *ConversationService) CancelTarget(_ context.Context, targetID string) error {
	startup := s.lookupTargetStartup(targetID)
	dispatch, err := s.store.GetConversationTargetDispatch(targetID)
	if err != nil {
		return err
	}
	return s.cancelTargetFromDispatch(dispatch, startup)
}

func (s *ConversationService) cancelTargetFromDispatch(dispatch store.ConversationTargetDispatch, startup *conversationTargetStartup) error {
	for {
		targetID := dispatch.Target.ID
		if dispatch.Target.State != conversation.TargetQueued && dispatch.Target.State != conversation.TargetWorking {
			return fmt.Errorf("target %s is already %s", targetID, dispatch.Target.State)
		}
		changed, err := s.store.TransitionConversationTargetWithCode(targetID, dispatch.Target.State, conversation.TargetCanceled, "canceled_by_user", "canceled by user")
		if err != nil {
			return err
		}
		if changed {
			break
		}
		dispatch, err = s.store.GetConversationTargetDispatch(targetID)
		if err != nil {
			return err
		}
	}
	targetID := dispatch.Target.ID
	if current := s.lookupTargetStartup(targetID); current != nil {
		startup = current
	}
	if startup != nil {
		startup.cancel()
	}
	s.mu.Lock()
	run := s.active[targetID]
	s.mu.Unlock()
	if run != nil {
		_ = run.Cancel()
	}
	_ = s.store.UpdateRunStatus(dispatch.Target.RunID, "canceled", 0, "canceled by user")
	return nil
}

func (s *ConversationService) RetryTarget(_ context.Context, targetID string) (conversation.Target, error) {
	original, err := s.store.GetConversationTargetDispatch(targetID)
	if err != nil {
		return conversation.Target{}, err
	}
	seat, ok := s.seatForParticipant(original.Participant)
	if reason := seatUnreadyReason(original.Participant, seat, ok); reason != "" {
		return conversation.Target{}, conversation.NewBoundedError(conversation.ErrorSeatUnready, fmt.Errorf("agent seat %q is not ready on %s: %s", original.Participant.Profile, original.Participant.Machine, reason))
	}
	retry, err := s.store.RetryConversationTarget(targetID, uuid.NewString(), uuid.NewString(), s.now().UTC())
	if err != nil {
		return conversation.Target{}, err
	}
	prompt, err := s.store.ConversationContext(retry.Conversation.ID, retry.Turn.ThroughMessageID)
	if err != nil {
		s.failQueuedTarget(retry.Target.ID, retry.Target.RunID, err)
		return conversation.Target{}, err
	}
	s.startTarget(retry, prompt)
	return retry.Target, nil
}

func (s *ConversationService) startTarget(dispatch store.ConversationTargetDispatch, prompt string) {
	s.targetStartup(dispatch.Target.ID)
	s.async.Add(1)
	go func() {
		defer s.async.Done()
		s.runTarget(dispatch, prompt)
	}()
}

// Wait joins every target accepted before the call. Shutdown and tests use it
// before closing the durable store that target completions persist to.
func (s *ConversationService) Wait() { s.async.Wait() }

// Close stops active providers and joins their persistence loops.
func (s *ConversationService) Close() {
	_, _ = s.store.FailInterruptedConversationTargets("interrupted when the Fort daemon stopped")
	s.cancel()
	s.mu.Lock()
	for _, run := range s.active {
		_ = run.Cancel()
	}
	s.mu.Unlock()
	s.Wait()
}

func (s *ConversationService) seatMap() map[string]conversation.Seat {
	out := map[string]conversation.Seat{}
	if s.seats == nil {
		return out
	}
	for _, seat := range s.seats.ConversationSeats() {
		out[seat.ID] = seat
	}
	return out
}

func (s *ConversationService) seatForParticipant(participant conversation.Participant) (conversation.Seat, bool) {
	if s.seats == nil {
		return conversation.Seat{}, false
	}
	var matched conversation.Seat
	found := false
	for _, seat := range s.seats.ConversationSeats() {
		if seat.ID == participant.SeatID {
			return seat, true
		}
		if seat.Profile != participant.Profile || !strings.EqualFold(seat.Machine, participant.Machine) {
			continue
		}
		if found {
			return conversation.Seat{}, false
		}
		matched, found = seat, true
	}
	return matched, found
}

func sameSeat(participant conversation.Participant, seat conversation.Seat) bool {
	return participant.Profile == seat.Profile && participant.Agent == seat.Agent && participant.Model == seat.Model &&
		strings.EqualFold(participant.Machine, seat.Machine)
}

func seatUnreadyReason(participant conversation.Participant, seat conversation.Seat, found bool) corecap.Reason {
	if !found {
		return corecap.ReasonProfileUnmapped
	}
	if seat.State != string(corecap.OfferReady) {
		reason := corecap.Reason(seat.Reason)
		if corecap.FirstReason(reason) == "" {
			return corecap.ReasonProbeFailed
		}
		return reason
	}
	if !sameSeat(participant, seat) {
		return corecap.ReasonCapabilityDrift
	}
	return ""
}

func closedSeatReason(seat conversation.Seat) corecap.Reason {
	reason := corecap.Reason(seat.Reason)
	if corecap.FirstReason(reason) == "" {
		return corecap.ReasonProbeFailed
	}
	return reason
}

func (s *ConversationService) runTarget(dispatch store.ConversationTargetDispatch, prompt string) {
	startup := s.targetStartup(dispatch.Target.ID)
	startup.mu.Lock()
	if startup.claimed {
		startup.mu.Unlock()
		return
	}
	startup.claimed = true
	startup.mu.Unlock()
	defer s.releaseTargetStartup(dispatch.Target.ID, startup)
	current, currentErr := s.store.GetConversationTargetDispatch(dispatch.Target.ID)
	if currentErr != nil {
		s.failQueuedTarget(dispatch.Target.ID, dispatch.Target.RunID, currentErr)
		return
	}
	if current.Target.State != conversation.TargetQueued {
		return
	}
	dispatch = current
	target, participant := dispatch.Target, dispatch.Participant
	seat, ready := s.seatForParticipant(participant)
	if reason := seatUnreadyReason(participant, seat, ready); reason != "" {
		s.failQueuedTargetCode(target.ID, target.RunID, "seat_unready", fmt.Errorf("agent seat %q is no longer ready on %s: %s", participant.Profile, participant.Machine, reason))
		return
	}
	if s.runtime == nil {
		s.failQueuedTargetCode(target.ID, target.RunID, "no_execution_plane", fmt.Errorf("no execution plane is available"))
		return
	}
	participantPrompt, err := conversation.CompileParticipantPrompt(prompt, participant)
	if err != nil {
		s.failQueuedTarget(target.ID, target.RunID, err)
		return
	}
	runRow := store.Run{
		ID: target.RunID, Title: dispatch.Conversation.Title, Body: participantPrompt, Agent: participant.Agent,
		Profile: participant.Profile, Model: participant.Model, Status: "queued", Machine: participant.Machine,
		MatchedRule: "conversation", CreatedAt: s.now().UTC(),
	}
	if err := s.store.CreateRun(runRow); err != nil {
		s.failQueuedTarget(target.ID, target.RunID, err)
		return
	}
	current, currentErr = s.store.GetConversationTargetDispatch(target.ID)
	if currentErr != nil {
		s.failQueuedTarget(target.ID, target.RunID, currentErr)
		return
	}
	if current.Target.State != conversation.TargetQueued {
		switch current.Target.State {
		case conversation.TargetCanceled:
			_ = s.store.UpdateRunStatus(target.RunID, "canceled", 0, current.Target.Error)
		case conversation.TargetFailed:
			_ = s.store.UpdateRunStatus(target.RunID, "failed", -1, current.Target.Error)
		}
		return
	}
	run, err := s.runtime.Dispatch(startup.ctx, runtime.RunSpec{
		RunID: target.RunID, Profile: participant.Profile, Agent: participant.Agent, Model: participant.Model,
		Prompt: participantPrompt, Workdir: s.workdir, Machine: participant.Machine,
	})
	if err != nil {
		var preflight *execcap.ProfilePreflightError
		if errors.As(err, &preflight) {
			s.failQueuedTargetCode(target.ID, target.RunID, "seat_unready", err)
		} else {
			s.failQueuedTarget(target.ID, target.RunID, err)
		}
		return
	}
	s.mu.Lock()
	s.active[target.ID] = run
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.active, target.ID)
		s.mu.Unlock()
	}()
	current, currentErr = s.store.GetConversationTargetDispatch(target.ID)
	if currentErr != nil {
		_ = run.Cancel()
		return
	}
	if current.Target.State == conversation.TargetCanceled {
		_ = run.Cancel()
		for range run.Stream() {
		}
		_ = run.Wait()
		_ = s.store.UpdateRunStatus(target.RunID, "canceled", 0, current.Target.Error)
		return
	}

	state := conversation.TargetQueued
	lastMessage := ""
	for event := range run.Stream() {
		_, _ = s.store.AppendEvent(store.Event{RunID: target.RunID, Type: string(event.Type), Data: event.Data, Code: event.Code, CreatedAt: event.Time})
		activity := event.Type == runtime.EventStarted || event.Type == runtime.EventStdout || event.Type == runtime.EventStderr || event.Type == runtime.EventMessage || event.Type == runtime.EventTool || event.Type == runtime.EventSubagent
		if activity && state == conversation.TargetQueued {
			if changed, transitionErr := s.store.TransitionConversationTarget(target.ID, conversation.TargetQueued, conversation.TargetWorking, ""); transitionErr == nil && changed {
				state = conversation.TargetWorking
				_ = s.store.UpdateRunStatus(target.RunID, "running", 0, "")
			}
		}
		if activity && state == conversation.TargetWorking {
			_ = s.store.TouchConversationTargetActivity(target.ID, event.Time)
		}
		if event.Type == runtime.EventMessage {
			lastMessage = event.Data
		}
	}
	status := run.Wait()
	switch status.State {
	case runtime.StateSucceeded:
		if state != conversation.TargetWorking || strings.TrimSpace(lastMessage) == "" {
			s.failTargetCode(target.ID, target.RunID, state, "missing_terminal_output", "provider completed without an attributed answer", status.ExitCode)
			return
		}
		changed, answerErr := s.store.AnswerConversationTarget(target.ID, conversation.Message{
			ConversationID: dispatch.Conversation.ID, TurnID: dispatch.Turn.ID, TargetID: target.ID,
			AuthorKind: conversation.AuthorAssistant, AuthorID: participant.ID, Body: lastMessage, CreatedAt: s.now().UTC(),
		})
		if answerErr != nil || !changed {
			_, _ = s.store.AppendEvent(store.Event{RunID: target.RunID, Type: string(runtime.EventError), Data: "failed to persist attributed answer", Code: -1})
			if answerErr != nil {
				_ = s.store.UpdateRunStatus(target.RunID, "failed", -1, answerErr.Error())
			}
			return
		}
		_ = s.store.UpdateRunStatus(target.RunID, "succeeded", status.ExitCode, "")
	case runtime.StateCanceled:
		s.cancelTerminalTarget(target.ID, target.RunID, state, status.Err)
	default:
		s.failTarget(target.ID, target.RunID, state, status.Err, status.ExitCode)
	}
}

func (s *ConversationService) targetStartup(targetID string) *conversationTargetStartup {
	s.mu.Lock()
	defer s.mu.Unlock()
	if startup := s.starting[targetID]; startup != nil {
		return startup
	}
	ctx, cancel := context.WithCancel(s.ctx)
	startup := &conversationTargetStartup{ctx: ctx, cancel: cancel}
	s.starting[targetID] = startup
	return startup
}

func (s *ConversationService) lookupTargetStartup(targetID string) *conversationTargetStartup {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.starting[targetID]
}

func (s *ConversationService) releaseTargetStartup(targetID string, startup *conversationTargetStartup) {
	startup.cancel()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.starting[targetID] == startup {
		delete(s.starting, targetID)
	}
}

func (s *ConversationService) failQueuedTarget(targetID, runID string, err error) {
	s.failQueuedTargetCode(targetID, runID, "startup_failed", err)
}

func (s *ConversationService) failQueuedTargetCode(targetID, runID, code string, err error) {
	changed, transitionErr := s.store.TransitionConversationTargetWithCode(targetID, conversation.TargetQueued, conversation.TargetFailed, code, err.Error())
	if transitionErr == nil && changed {
		_ = s.store.UpdateRunStatus(runID, "failed", -1, err.Error())
	}
}

func (s *ConversationService) failTarget(targetID, runID string, state conversation.TargetState, message string, code int) {
	s.failTargetCode(targetID, runID, state, "provider_failed", message, code)
}

func (s *ConversationService) failTargetCode(targetID, runID string, state conversation.TargetState, errorCode, message string, code int) {
	if message == "" {
		message = "provider failed"
	}
	if state != conversation.TargetQueued && state != conversation.TargetWorking {
		return
	}
	changed, err := s.store.TransitionConversationTargetWithCode(targetID, state, conversation.TargetFailed, errorCode, message)
	if err == nil && changed {
		_ = s.store.UpdateRunStatus(runID, "failed", code, message)
	}
}

func (s *ConversationService) cancelTerminalTarget(targetID, runID string, state conversation.TargetState, message string) {
	if state != conversation.TargetQueued && state != conversation.TargetWorking {
		return
	}
	changed, err := s.store.TransitionConversationTargetWithCode(targetID, state, conversation.TargetCanceled, "canceled", message)
	if err == nil && changed {
		_ = s.store.UpdateRunStatus(runID, "canceled", 0, message)
	}
}
