package control

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	corecap "github.com/tobsai/fort/core/capability"
	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/runtime"
	"github.com/tobsai/fort/core/store"
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
		if !ok {
			continue
		}
		seats = append(seats, conversation.Seat{
			ID: definition.ID + "@" + machine, Profile: definition.ID, Agent: agent, Model: model,
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
	displayNames := make(map[string]string, len(catalog.Profiles))
	for _, definition := range catalog.Profiles {
		displayNames[definition.ID] = definition.DisplayName
	}
	out := []conversation.Seat{}
	for _, machine := range snapshot.Machines {
		for _, offer := range machine.Profiles {
			agent, model, ok := catalog.RuntimeSelection(offer.ID)
			if !ok {
				continue
			}
			state, reason := string(offer.State), string(offer.Reason)
			if !machine.Reachable {
				state, reason = string(corecap.OfferUnavailable), string(corecap.ReasonUnavailable)
			}
			out = append(out, conversation.Seat{
				ID: offer.ID + "@" + machine.Name, Profile: offer.ID, Agent: agent, Model: model,
				Machine: machine.Name, DisplayName: displayNames[offer.ID] + " on " + machine.Name,
				State: state, Reason: reason,
			})
		}
	}
	return out
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

	mu     sync.Mutex
	active map[string]runtime.Run
	async  sync.WaitGroup
}

func NewConversationService(st *store.Store, rt runtime.Runtime, seats ConversationSeatSource, workdir string) *ConversationService {
	ctx, cancel := context.WithCancel(context.Background())
	return &ConversationService{store: st, runtime: rt, seats: seats, workdir: workdir, now: time.Now, ctx: ctx, cancel: cancel, active: map[string]runtime.Run{}}
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

func (s *ConversationService) GetConversation(_ context.Context, id string) (store.ConversationDetail, error) {
	return s.store.GetConversation(id)
}

func (s *ConversationService) CreateConversation(_ context.Context, projectID, title string, seatIDs []string) (store.ConversationDetail, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return store.ConversationDetail{}, fmt.Errorf("conversation title is required")
	}
	if len(seatIDs) == 0 {
		return store.ConversationDetail{}, fmt.Errorf("at least one agent seat is required")
	}
	available := s.seatMap()
	seen := map[string]bool{}
	now := s.now().UTC()
	id := uuid.NewString()
	participants := make([]conversation.Participant, 0, len(seatIDs))
	for position, seatID := range seatIDs {
		if seen[seatID] {
			return store.ConversationDetail{}, fmt.Errorf("agent seat %q was selected more than once", seatID)
		}
		seen[seatID] = true
		seat, ok := available[seatID]
		if !ok {
			return store.ConversationDetail{}, fmt.Errorf("agent seat %q is not in the current inventory", seatID)
		}
		if seat.State != string(corecap.OfferReady) {
			return store.ConversationDetail{}, fmt.Errorf("agent seat %q is not ready: %s", seatID, seat.Reason)
		}
		participants = append(participants, conversation.Participant{
			ID: uuid.NewString(), ConversationID: id, SeatID: seat.ID, Profile: seat.Profile,
			Agent: seat.Agent, Model: seat.Model, Machine: seat.Machine, DisplayName: seat.DisplayName,
			Position: position, State: conversation.ParticipantActive, CreatedAt: now,
		})
	}
	item := conversation.Conversation{ID: id, ProjectID: projectID, Title: title, CreatedAt: now, UpdatedAt: now}
	if err := s.store.CreateConversation(item, participants); err != nil {
		return store.ConversationDetail{}, err
	}
	return s.store.GetConversation(id)
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
	if !ok || seat.State != string(corecap.OfferReady) {
		return conversation.Participant{}, fmt.Errorf("agent seat %q is not ready", seatID)
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
	return s.store.DeleteConversation(id)
}

func (s *ConversationService) PostTurn(_ context.Context, conversationID, clientTurnID, text string, targetParticipantIDs []string) (conversation.TurnResult, error) {
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
	if strings.TrimSpace(clientTurnID) == "" {
		return conversation.TurnResult{}, fmt.Errorf("client_turn_id is required")
	}
	currentSeats := s.seatMap()
	seen := map[string]bool{}
	requested := make([]store.ConversationTurnTarget, 0, len(targetParticipantIDs))
	for _, participantID := range targetParticipantIDs {
		if seen[participantID] {
			return conversation.TurnResult{}, fmt.Errorf("participant %q was targeted more than once", participantID)
		}
		seen[participantID] = true
		participant, ok := participants[participantID]
		if !ok {
			return conversation.TurnResult{}, fmt.Errorf("participant %q is not in conversation %s", participantID, conversationID)
		}
		if participant.State != conversation.ParticipantActive {
			return conversation.TurnResult{}, fmt.Errorf("participant %q is removed", participantID)
		}
		seat, ok := currentSeats[participant.SeatID]
		if !ok || !sameSeat(participant, seat) || seat.State != string(corecap.OfferReady) {
			return conversation.TurnResult{}, fmt.Errorf("agent seat %q is not ready on %s", participant.Profile, participant.Machine)
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
	dispatch, err := s.store.GetConversationTargetDispatch(targetID)
	if err != nil {
		return err
	}
	if dispatch.Target.State != conversation.TargetQueued && dispatch.Target.State != conversation.TargetWorking {
		return fmt.Errorf("target %s is already %s", targetID, dispatch.Target.State)
	}
	changed, err := s.store.TransitionConversationTargetWithCode(targetID, dispatch.Target.State, conversation.TargetCanceled, "canceled_by_user", "canceled by user")
	if err != nil {
		return err
	}
	if !changed {
		return fmt.Errorf("target %s changed while cancellation was requested", targetID)
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
	seat, ok := s.seatMap()[original.Participant.SeatID]
	if !ok || !sameSeat(original.Participant, seat) || seat.State != string(corecap.OfferReady) {
		return conversation.Target{}, fmt.Errorf("agent seat %q is not ready on %s", original.Participant.Profile, original.Participant.Machine)
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

func sameSeat(participant conversation.Participant, seat conversation.Seat) bool {
	return participant.SeatID == seat.ID && participant.Profile == seat.Profile && participant.Agent == seat.Agent &&
		participant.Model == seat.Model && participant.Machine == seat.Machine
}

func (s *ConversationService) runTarget(dispatch store.ConversationTargetDispatch, prompt string) {
	target, participant := dispatch.Target, dispatch.Participant
	seat, ready := s.seatMap()[participant.SeatID]
	if !ready || !sameSeat(participant, seat) || seat.State != string(corecap.OfferReady) {
		s.failQueuedTargetCode(target.ID, target.RunID, "seat_unready", fmt.Errorf("agent seat %q is no longer ready on %s", participant.Profile, participant.Machine))
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
	run, err := s.runtime.Dispatch(s.ctx, runtime.RunSpec{
		RunID: target.RunID, Profile: participant.Profile, Agent: participant.Agent, Model: participant.Model,
		Prompt: participantPrompt, Workdir: s.workdir, Machine: participant.Machine,
	})
	if err != nil {
		s.failQueuedTarget(target.ID, target.RunID, err)
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
	current, currentErr := s.store.GetConversationTargetDispatch(target.ID)
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

func (s *ConversationService) failQueuedTarget(targetID, runID string, err error) {
	s.failQueuedTargetCode(targetID, runID, "startup_failed", err)
}

func (s *ConversationService) failQueuedTargetCode(targetID, runID, code string, err error) {
	_, _ = s.store.TransitionConversationTargetWithCode(targetID, conversation.TargetQueued, conversation.TargetFailed, code, err.Error())
	_ = s.store.UpdateRunStatus(runID, "failed", -1, err.Error())
}

func (s *ConversationService) failTarget(targetID, runID string, state conversation.TargetState, message string, code int) {
	s.failTargetCode(targetID, runID, state, "provider_failed", message, code)
}

func (s *ConversationService) failTargetCode(targetID, runID string, state conversation.TargetState, errorCode, message string, code int) {
	if message == "" {
		message = "provider failed"
	}
	if state == conversation.TargetQueued || state == conversation.TargetWorking {
		_, _ = s.store.TransitionConversationTargetWithCode(targetID, state, conversation.TargetFailed, errorCode, message)
	}
	_ = s.store.UpdateRunStatus(runID, "failed", code, message)
}

func (s *ConversationService) cancelTerminalTarget(targetID, runID string, state conversation.TargetState, message string) {
	if state == conversation.TargetQueued || state == conversation.TargetWorking {
		_, _ = s.store.TransitionConversationTargetWithCode(targetID, state, conversation.TargetCanceled, "canceled", message)
	}
	_ = s.store.UpdateRunStatus(runID, "canceled", 0, message)
}
