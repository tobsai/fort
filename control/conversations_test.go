package control

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	corecap "github.com/tobsai/fort/core/capability"
	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/runtime"
	"github.com/tobsai/fort/core/store"
	execcap "github.com/tobsai/fort/exec/capability"
	"github.com/tobsai/fort/exec/cluster"
	"github.com/tobsai/fort/exec/fake"
	remoteexec "github.com/tobsai/fort/exec/remote"
)

type fixedConversationSeats []conversation.Seat

func (s fixedConversationSeats) ConversationSeats() []conversation.Seat {
	return append([]conversation.Seat(nil), s...)
}

type mutableConversationSeats struct {
	mu    sync.Mutex
	seats []conversation.Seat
}

func (s *mutableConversationSeats) ConversationSeats() []conversation.Seat {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]conversation.Seat(nil), s.seats...)
}

func (s *mutableConversationSeats) Set(seats ...conversation.Seat) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seats = append([]conversation.Seat(nil), seats...)
}

type conversationProfileDriftRefresher struct{}

func (conversationProfileDriftRefresher) RefreshMachine(context.Context, string, corecap.RefreshMode, []string) (corecap.MachineInventory, error) {
	return corecap.MachineInventory{
		Name: "studio", Reachable: true,
		Profiles: []corecap.ProfileOffer{{
			ID: "codex:gpt-5.6-sol", State: corecap.OfferSetupRequired, Reason: corecap.ReasonUnavailable,
		}},
	}, nil
}

type blockingConversationSeats struct {
	seats       []conversation.Seat
	entered     chan struct{}
	release     chan struct{}
	enteredOnce sync.Once
	releaseOnce sync.Once
}

func (s *blockingConversationSeats) ConversationSeats() []conversation.Seat {
	s.enteredOnce.Do(func() { close(s.entered) })
	<-s.release
	return append([]conversation.Seat(nil), s.seats...)
}

func (s *blockingConversationSeats) Unblock() {
	s.releaseOnce.Do(func() { close(s.release) })
}

type cancelableConversationDispatchRuntime struct {
	entered  chan struct{}
	canceled chan struct{}
	release  chan struct{}
}

func (r *cancelableConversationDispatchRuntime) Name() string { return "cancelable-dispatch" }

func (r *cancelableConversationDispatchRuntime) Dispatch(ctx context.Context, _ runtime.RunSpec) (runtime.Run, error) {
	close(r.entered)
	select {
	case <-ctx.Done():
		close(r.canceled)
		return nil, ctx.Err()
	case <-r.release:
		return nil, errors.New("dispatch released")
	}
}

type successfulExitWithoutMessageRuntime struct{}

func (successfulExitWithoutMessageRuntime) Name() string { return "successful-exit-without-message" }

func (successfulExitWithoutMessageRuntime) Dispatch(_ context.Context, spec runtime.RunSpec) (runtime.Run, error) {
	events := make(chan runtime.RunEvent, 2)
	now := time.Now()
	events <- runtime.RunEvent{RunID: spec.RunID, Type: runtime.EventStarted, Time: now, Data: spec.Agent}
	events <- runtime.RunEvent{RunID: spec.RunID, Type: runtime.EventExited, Time: now, Code: 0}
	close(events)
	return successfulExitWithoutMessageRun{id: spec.RunID, events: events}, nil
}

type successfulExitWithoutMessageRun struct {
	id     string
	events <-chan runtime.RunEvent
}

func (r successfulExitWithoutMessageRun) ID() string                      { return r.id }
func (r successfulExitWithoutMessageRun) Stream() <-chan runtime.RunEvent { return r.events }
func (successfulExitWithoutMessageRun) Signal(string) error               { return nil }
func (successfulExitWithoutMessageRun) Cancel() error                     { return nil }
func (successfulExitWithoutMessageRun) Status() runtime.Status {
	return runtime.Status{State: runtime.StateSucceeded, ExitCode: 0}
}
func (successfulExitWithoutMessageRun) Wait() runtime.Status {
	return runtime.Status{State: runtime.StateSucceeded, ExitCode: 0}
}

type barrierConversationRelease struct {
	once sync.Once
	ch   chan struct{}
}

type barrierConversationRuntime struct {
	entered  chan runtime.RunSpec
	returned chan string
	releases map[string]*barrierConversationRelease
	answers  map[string]string
}

func newBarrierConversationRuntime(profiles ...string) *barrierConversationRuntime {
	releases := make(map[string]*barrierConversationRelease, len(profiles))
	answers := make(map[string]string, len(profiles))
	for _, profile := range profiles {
		releases[profile] = &barrierConversationRelease{ch: make(chan struct{})}
		answers[profile] = "answer from " + profile
	}
	return &barrierConversationRuntime{
		entered: make(chan runtime.RunSpec, len(profiles)), returned: make(chan string, len(profiles)),
		releases: releases, answers: answers,
	}
}

func (r *barrierConversationRuntime) Name() string { return "barrier-conversation" }

func (r *barrierConversationRuntime) Dispatch(ctx context.Context, spec runtime.RunSpec) (runtime.Run, error) {
	release, ok := r.releases[spec.Profile]
	if !ok {
		return nil, errors.New("unexpected conversation profile " + spec.Profile)
	}
	select {
	case r.entered <- spec:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	select {
	case <-release.ch:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	r.returned <- spec.Profile
	return newBarrierConversationRun(spec, r.answers[spec.Profile]), nil
}

func (r *barrierConversationRuntime) release(profile string) {
	if gate := r.releases[profile]; gate != nil {
		gate.once.Do(func() { close(gate.ch) })
	}
}

func (r *barrierConversationRuntime) releaseAll() {
	for profile := range r.releases {
		r.release(profile)
	}
}

type barrierConversationRun struct {
	id     string
	events <-chan runtime.RunEvent
}

func newBarrierConversationRun(spec runtime.RunSpec, answer string) barrierConversationRun {
	events := make(chan runtime.RunEvent, 3)
	now := time.Now().UTC()
	events <- runtime.RunEvent{RunID: spec.RunID, Type: runtime.EventStarted, Time: now, Data: spec.Agent}
	events <- runtime.RunEvent{RunID: spec.RunID, Type: runtime.EventMessage, Time: now, Data: answer}
	events <- runtime.RunEvent{RunID: spec.RunID, Type: runtime.EventExited, Time: now, Code: 0}
	close(events)
	return barrierConversationRun{id: spec.RunID, events: events}
}

func (r barrierConversationRun) ID() string                      { return r.id }
func (r barrierConversationRun) Stream() <-chan runtime.RunEvent { return r.events }
func (barrierConversationRun) Signal(string) error               { return nil }
func (barrierConversationRun) Cancel() error                     { return nil }
func (barrierConversationRun) Status() runtime.Status {
	return runtime.Status{State: runtime.StateSucceeded, ExitCode: 0}
}
func (barrierConversationRun) Wait() runtime.Status {
	return runtime.Status{State: runtime.StateSucceeded, ExitCode: 0}
}

type fixedCapabilitySnapshot struct{}

func (fixedCapabilitySnapshot) Capabilities() (corecap.Snapshot, uint64) {
	return corecap.Snapshot{Machines: []corecap.MachineInventory{{
		Name: "studio", Reachable: true,
		Profiles: []corecap.ProfileOffer{{ID: "codex:gpt-5.6-sol", Agent: "codex", State: corecap.OfferReady}},
	}}}, 1
}

type conversationCapabilitySnapshot struct {
	snapshot   corecap.Snapshot
	generation uint64
}

func (s conversationCapabilitySnapshot) Capabilities() (corecap.Snapshot, uint64) {
	return s.snapshot, s.generation
}

func TestConversationServiceFansOneFrozenTurnAcrossExactSeats(t *testing.T) {
	st := newStore(t)
	rt := fake.New()
	seats := fixedConversationSeats{
		{ID: "codex-5@studio", Profile: "codex-5", Agent: "codex", Model: "gpt-5", Machine: "studio", DisplayName: "Codex on Studio", State: "ready"},
		{ID: "hermes@mini", Profile: "hermes", Agent: "hermes", Machine: "mini", DisplayName: "Hermes on Mini", State: "ready"},
	}
	service := NewConversationService(st, rt, seats, t.TempDir())
	t.Cleanup(service.Close)
	detail, err := service.CreateConversation(context.Background(), "", "Shared thread", []string{"codex-5@studio", "hermes@mini"})
	if err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	result, err := service.PostTurn(context.Background(), detail.Conversation.ID, "client-1", "Compare your answers.", []string{detail.Participants[0].ID, detail.Participants[1].ID})
	if err != nil {
		t.Fatalf("post turn: %v", err)
	}
	if len(result.Targets) != 2 {
		t.Fatalf("targets = %+v", result.Targets)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		got, getErr := service.GetConversation(context.Background(), detail.Conversation.ID)
		if getErr != nil {
			t.Fatal(getErr)
		}
		if len(got.Targets) == 2 && got.Targets[0].State == conversation.TargetAnswered && got.Targets[1].State == conversation.TargetAnswered {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	got, err := service.GetConversation(context.Background(), detail.Conversation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Targets[0].State != conversation.TargetAnswered || got.Targets[1].State != conversation.TargetAnswered {
		t.Fatalf("target states = %+v", got.Targets)
	}
	if len(got.Messages) != 3 {
		t.Fatalf("messages = %+v, want one human and two attributed replies", got.Messages)
	}

	specs := rt.Dispatched()
	if len(specs) != 2 {
		t.Fatalf("dispatched = %+v", specs)
	}
	var sharedContext string
	for _, spec := range specs {
		if !strings.Contains(spec.Prompt, `"conversation_id":"`+detail.Conversation.ID+`"`) || !strings.Contains(spec.Prompt, `"body":"Compare your answers."`) {
			t.Fatalf("target did not receive the shared snapshot: %s", spec.Prompt)
		}
		var prompt struct {
			Participant struct {
				ParticipantID string `json:"participant_id"`
			} `json:"participant"`
			Context json.RawMessage `json:"context"`
		}
		if err := json.Unmarshal([]byte(spec.Prompt), &prompt); err != nil {
			t.Fatalf("decode provider prompt: %v\n%s", err, spec.Prompt)
		}
		if prompt.Participant.ParticipantID == "" || !strings.Contains(string(prompt.Context), `"profile":"codex-5"`) || !strings.Contains(string(prompt.Context), `"profile":"hermes"`) || !strings.Contains(string(prompt.Context), `"machine":"studio"`) || !strings.Contains(string(prompt.Context), `"machine":"mini"`) {
			t.Fatalf("provider context lost exact participant registry: %s", prompt.Context)
		}
		if sharedContext == "" {
			sharedContext = string(prompt.Context)
		} else if string(prompt.Context) != sharedContext {
			t.Fatalf("targets received different frozen contexts:\nfirst:  %s\nsecond: %s", sharedContext, prompt.Context)
		}
	}
	byMachine := map[string]bool{}
	for _, spec := range specs {
		byMachine[spec.Machine] = true
		if spec.Profile == "" || spec.Agent == "" {
			t.Fatalf("dispatch lost exact seat identity: %+v", spec)
		}
	}
	if !byMachine["studio"] || !byMachine["mini"] {
		t.Fatalf("machines = %+v, want studio and mini", byMachine)
	}
}

func TestConversationServiceRunsTargetsConcurrentlyAcrossCompletionOrders(t *testing.T) {
	for _, order := range [][]string{{"alpha", "beta"}, {"beta", "alpha"}} {
		order := order
		t.Run(strings.Join(order, "-then-"), func(t *testing.T) {
			st := newStore(t)
			rt := newBarrierConversationRuntime("alpha", "beta")
			seats := fixedConversationSeats{
				{ID: "alpha@studio", Profile: "alpha", Agent: "codex", Machine: "studio", DisplayName: "Alpha", State: "ready"},
				{ID: "beta@mini", Profile: "beta", Agent: "hermes", Machine: "mini", DisplayName: "Beta", State: "ready"},
			}
			service := NewConversationService(st, rt, seats, t.TempDir())
			t.Cleanup(service.Close)
			t.Cleanup(rt.releaseAll)
			detail, err := service.CreateConversation(context.Background(), "", "Concurrent thread", []string{"alpha@studio", "beta@mini"})
			if err != nil {
				t.Fatal(err)
			}
			participantByProfile := make(map[string]conversation.Participant, len(detail.Participants))
			for _, participant := range detail.Participants {
				participantByProfile[participant.Profile] = participant
			}
			turn, err := service.PostTurn(context.Background(), detail.Conversation.ID, "client-concurrent", "answer together", []string{
				participantByProfile["alpha"].ID, participantByProfile["beta"].ID,
			})
			if err != nil {
				t.Fatal(err)
			}
			targetByProfile := make(map[string]conversation.Target, len(turn.Targets))
			for _, target := range turn.Targets {
				for profile, participant := range participantByProfile {
					if target.ParticipantID == participant.ID {
						targetByProfile[profile] = target
					}
				}
			}

			entered := map[string]runtime.RunSpec{}
			for len(entered) < 2 {
				select {
				case spec := <-rt.entered:
					entered[spec.Profile] = spec
				case <-time.After(time.Second):
					t.Fatalf("dispatch barrier entries = %v, want both profiles", entered)
				}
			}
			if len(entered) != 2 || entered["alpha"].RunID == "" || entered["beta"].RunID == "" {
				t.Fatalf("dispatch barrier entries = %+v", entered)
			}
			select {
			case profile := <-rt.returned:
				t.Fatalf("dispatch for %s completed before both targets entered", profile)
			default:
			}

			for index, profile := range order {
				rt.release(profile)
				select {
				case returned := <-rt.returned:
					if returned != profile {
						t.Fatalf("dispatch returned for %s, want %s", returned, profile)
					}
				case <-time.After(time.Second):
					t.Fatalf("dispatch for %s did not return", profile)
				}
				waitForConversationTargetState(t, service, detail.Conversation.ID, targetByProfile[profile].ID, conversation.TargetAnswered)
				if index == 0 {
					midway, getErr := service.GetConversation(context.Background(), detail.Conversation.ID)
					if getErr != nil {
						t.Fatal(getErr)
					}
					for _, target := range midway.Targets {
						if target.ID == targetByProfile[order[1]].ID && target.State != conversation.TargetQueued {
							t.Fatalf("unreleased peer target = %+v, want queued", target)
						}
					}
				}
			}

			got, err := service.GetConversation(context.Background(), detail.Conversation.ID)
			if err != nil {
				t.Fatal(err)
			}
			if len(got.Messages) != 3 || got.Messages[0].AuthorKind != conversation.AuthorHuman || got.Messages[0].Body != "answer together" {
				t.Fatalf("messages = %+v, want one human message and two answers", got.Messages)
			}
			if got.Messages[1].Body != rt.answers[order[0]] || got.Messages[2].Body != rt.answers[order[1]] {
				t.Fatalf("answer completion order = %+v, want %q then %q", got.Messages, rt.answers[order[0]], rt.answers[order[1]])
			}
			answersByTarget := map[string]conversation.Message{}
			for _, message := range got.Messages[1:] {
				answersByTarget[message.TargetID] = message
			}
			for profile, original := range targetByProfile {
				var persisted conversation.Target
				for _, target := range got.Targets {
					if target.ID == original.ID {
						persisted = target
						break
					}
				}
				answer := answersByTarget[original.ID]
				participant := participantByProfile[profile]
				if persisted.State != conversation.TargetAnswered || persisted.ErrorCode != "" || answer.AuthorKind != conversation.AuthorAssistant || answer.AuthorID != participant.ID || answer.TurnID != turn.Turn.ID || answer.Body != rt.answers[profile] {
					t.Fatalf("profile %s target=%+v answer=%+v participant=%+v", profile, persisted, answer, participant)
				}
				run, getErr := st.GetRun(original.RunID)
				if getErr != nil {
					t.Fatal(getErr)
				}
				if run.Status != "succeeded" || run.ExitCode != 0 || run.Error != "" {
					t.Fatalf("profile %s run = %+v, want clean terminal success", profile, run)
				}
			}
		})
	}
}

func TestConversationServiceRejectsDeletionWhileTargetIsNonterminal(t *testing.T) {
	st := newStore(t)
	rt := newBarrierConversationRuntime("alpha")
	seats := fixedConversationSeats{{
		ID: "alpha@studio", Profile: "alpha", Agent: "codex", Machine: "studio", DisplayName: "Alpha", State: "ready",
	}}
	service := NewConversationService(st, rt, seats, t.TempDir())
	t.Cleanup(service.Close)
	t.Cleanup(rt.releaseAll)

	detail, err := service.CreateConversation(context.Background(), "", "Active thread", []string{"alpha@studio"})
	if err != nil {
		t.Fatal(err)
	}
	posted, err := service.PostTurn(context.Background(), detail.Conversation.ID, "client-active-delete", "keep working", []string{detail.Participants[0].ID})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-rt.entered:
	case <-time.After(time.Second):
		t.Fatal("target did not enter runtime dispatch")
	}

	err = service.DeleteConversation(context.Background(), detail.Conversation.ID)
	var bounded *conversation.BoundedError
	if !errors.As(err, &bounded) || bounded.Code != conversation.ErrorCode("conversation_active") {
		t.Fatalf("delete active conversation error = %v, want conversation_active", err)
	}
	if _, err := service.GetConversation(context.Background(), detail.Conversation.ID); err != nil {
		t.Fatalf("active conversation was deleted: %v", err)
	}

	rt.release("alpha")
	waitForConversationTargetState(t, service, detail.Conversation.ID, posted.Targets[0].ID, conversation.TargetAnswered)
	service.Wait()
	if err := service.DeleteConversation(context.Background(), detail.Conversation.ID); err != nil {
		t.Fatalf("delete completed conversation: %v", err)
	}
	if _, err := service.GetConversation(context.Background(), detail.Conversation.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("get deleted conversation error = %v, want sql.ErrNoRows", err)
	}
	run, err := st.GetRun(posted.Targets[0].RunID)
	if err != nil {
		t.Fatalf("get preserved run: %v", err)
	}
	if run.Status != "succeeded" {
		t.Fatalf("preserved run status = %q, want succeeded", run.Status)
	}
}

func TestConversationServicePersistsTurnAndFailsOnlyDriftedSeat(t *testing.T) {
	st := newStore(t)
	rt := fake.New()
	seats := fixedConversationSeats{
		{ID: "codex@studio", Profile: "codex", Agent: "codex", Machine: "studio", DisplayName: "Codex on Studio", State: "ready"},
		{ID: "hermes@mini", Profile: "hermes", Agent: "hermes", Machine: "mini", DisplayName: "Hermes on Mini", State: "ready"},
	}
	service := NewConversationService(st, rt, seats, t.TempDir())
	t.Cleanup(service.Close)
	detail, err := service.CreateConversation(context.Background(), "", "Thread", []string{"codex@studio", "hermes@mini"})
	if err != nil {
		t.Fatal(err)
	}
	seats[1].State = "unavailable"
	posted, err := service.PostTurn(context.Background(), detail.Conversation.ID, "client-1", "hello", []string{detail.Participants[0].ID, detail.Participants[1].ID})
	if err != nil {
		t.Fatalf("post turn: %v", err)
	}
	if len(posted.Targets) != 2 {
		t.Fatalf("targets = %+v", posted.Targets)
	}
	service.Wait()
	got, err := service.GetConversation(context.Background(), detail.Conversation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Messages) != 2 || len(got.Turns) != 1 {
		t.Fatalf("durable turn = %+v", got)
	}
	byParticipant := make(map[string]conversation.Target, len(got.Targets))
	for _, target := range got.Targets {
		byParticipant[target.ParticipantID] = target
	}
	if target := byParticipant[detail.Participants[0].ID]; target.State != conversation.TargetAnswered {
		t.Fatalf("ready target = %+v", target)
	}
	if target := byParticipant[detail.Participants[1].ID]; target.State != conversation.TargetFailed || target.ErrorCode != "seat_unready" {
		t.Fatalf("drifted target = %+v", target)
	}
	if dispatched := rt.Dispatched(); len(dispatched) != 1 || dispatched[0].Machine != "studio" {
		t.Fatalf("dispatched = %+v", dispatched)
	}
}

func TestConversationProfilePreflightDriftUsesSeatUnreadyRecovery(t *testing.T) {
	st := newStore(t)
	downstream := fake.New()
	rt := execcap.NewProfileGate(downstream, conversationProfileDriftRefresher{})
	seat := conversation.Seat{
		ID: "codex:gpt-5.6-sol@studio", Profile: "codex:gpt-5.6-sol", Agent: "codex",
		Model: "gpt-5.6-sol", Machine: "studio", State: string(corecap.OfferReady),
	}
	service := NewConversationService(st, rt, fixedConversationSeats{seat}, t.TempDir())
	t.Cleanup(service.Close)
	detail, err := service.CreateConversation(context.Background(), "", "Fresh preflight", []string{seat.ID})
	if err != nil {
		t.Fatal(err)
	}
	posted, err := service.PostTurn(context.Background(), detail.Conversation.ID, "client-preflight-drift", "hello", []string{detail.Participants[0].ID})
	if err != nil {
		t.Fatal(err)
	}
	service.Wait()
	got, err := service.GetConversation(context.Background(), detail.Conversation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Targets) != 1 || got.Targets[0].ID != posted.Targets[0].ID || got.Targets[0].State != conversation.TargetFailed || got.Targets[0].ErrorCode != "seat_unready" {
		t.Fatalf("fresh-preflight target = %+v, want failed seat_unready", got.Targets)
	}
	if dispatched := downstream.Dispatched(); len(dispatched) != 0 {
		t.Fatalf("fresh preflight dispatched provider: %+v", dispatched)
	}
}

func TestCapabilityConversationSeatsUseOpaqueStableIdentity(t *testing.T) {
	source := SnapshotConversationSeats{Source: fixedCapabilitySnapshot{}}
	first := source.ConversationSeats()
	second := source.ConversationSeats()
	if len(first) != 1 || len(second) != 1 {
		t.Fatalf("seats = %+v / %+v", first, second)
	}
	seat := first[0]
	wantID := conversationSeatID("codex:gpt-5.6-sol", "studio", "gpt-5.6-sol")
	if seat.ID != wantID || seat.ID != second[0].ID || strings.Contains(seat.ID, seat.Profile) ||
		strings.Contains(seat.ID, seat.Machine) || strings.Contains(seat.ID, seat.Model) ||
		seat.Model != "gpt-5.6-sol" || seat.State != "ready" {
		t.Fatalf("seat identity is not opaque and stable: first=%+v second=%+v", seat, second[0])
	}
}

func TestCapabilityConversationSeatsResolveDynamicModelIntoOpaqueStableIdentity(t *testing.T) {
	source := conversationCapabilitySnapshot{
		generation: 1,
		snapshot: corecap.Snapshot{Machines: []corecap.MachineInventory{{
			Name: "studio.local", Reachable: true,
			Profiles: []corecap.ProfileOffer{{
				ID: "codex:configured-default", Agent: "codex", State: corecap.OfferReady,
				ResolvedModel: "gpt-5.6-sol", BindingRevision: "opaque:sol",
			}},
		}}},
	}
	first := (SnapshotConversationSeats{Source: source}).ConversationSeats()
	second := (SnapshotConversationSeats{Source: source}).ConversationSeats()
	if len(first) != 1 || len(second) != 1 {
		t.Fatalf("dynamic seats = %+v / %+v", first, second)
	}
	seat := first[0]
	if seat.Profile != "codex:configured-default" || seat.Agent != "codex" || seat.Model != "gpt-5.6-sol" || seat.Machine != "studio.local" || seat.State != string(corecap.OfferReady) {
		t.Fatalf("dynamic seat = %+v", seat)
	}
	if seat.DisplayName != "Codex · GPT-5.6 Sol (default) on studio.local" {
		t.Fatalf("dynamic display name = %q", seat.DisplayName)
	}
	if seat.ID == "codex:configured-default@studio.local" || strings.Contains(seat.ID, "gpt-5.6-sol") || seat.ID != second[0].ID {
		t.Fatalf("dynamic seat ID is not opaque and stable: first=%q second=%q", seat.ID, second[0].ID)
	}

	changed := source
	changed.snapshot.Machines = []corecap.MachineInventory{{
		Name: "studio.local", Reachable: true,
		Profiles: []corecap.ProfileOffer{{
			ID: "codex:configured-default", Agent: "codex", State: corecap.OfferReady,
			ResolvedModel: "gpt-5.6-terra", BindingRevision: "opaque:terra",
		}},
	}}
	changedSeat := (SnapshotConversationSeats{Source: changed}).ConversationSeats()[0]
	if changedSeat.ID == seat.ID || changedSeat.Model != "gpt-5.6-terra" || changedSeat.DisplayName != "Codex · GPT-5.6 Terra (default) on studio.local" {
		t.Fatalf("changed dynamic seat = %+v, original=%+v", changedSeat, seat)
	}
}

func TestCapabilityConversationSeatsFailClosedWithoutDynamicResolvedModel(t *testing.T) {
	source := conversationCapabilitySnapshot{
		generation: 1,
		snapshot: corecap.Snapshot{Machines: []corecap.MachineInventory{{
			Name: "studio.local", Reachable: true,
			Profiles: []corecap.ProfileOffer{{
				ID: "codex:configured-default", Agent: "codex", State: corecap.OfferReady,
				BindingRevision: "opaque:invalid-ready",
			}},
		}}},
	}
	seats := (SnapshotConversationSeats{Source: source}).ConversationSeats()
	if len(seats) != 1 {
		t.Fatalf("dynamic seats = %+v", seats)
	}
	if seats[0].State == string(corecap.OfferReady) || seats[0].Reason != string(corecap.ReasonModelUnavailable) || seats[0].Model != "" {
		t.Fatalf("unresolved dynamic seat did not fail closed: %+v", seats[0])
	}
}

func TestConversationDispatchRevalidatesDynamicSeatByProfileAndMachine(t *testing.T) {
	st := newStore(t)
	rt := fake.New()
	original := conversation.Seat{
		ID: "legacy-dynamic-seat", Profile: "codex:configured-default", Agent: "codex",
		Model: "gpt-5.6-sol", Machine: "studio.local", DisplayName: "Codex · GPT-5.6 Sol (default) on studio.local",
		State: string(corecap.OfferReady),
	}
	current := original
	current.ID = conversationSeatID(current.Profile, current.Machine, current.Model)
	seats := &mutableConversationSeats{seats: []conversation.Seat{original}}
	service := NewConversationService(st, rt, seats, t.TempDir())
	t.Cleanup(service.Close)
	detail, err := service.CreateConversation(context.Background(), "", "Pinned model", []string{original.ID})
	if err != nil {
		t.Fatal(err)
	}
	seats.Set(current)
	posted, err := service.PostTurn(context.Background(), detail.Conversation.ID, "client-pinned", "hello", []string{detail.Participants[0].ID})
	if err != nil {
		t.Fatal(err)
	}
	service.Wait()
	got, err := service.GetConversation(context.Background(), detail.Conversation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Targets) != 1 || got.Targets[0].ID != posted.Targets[0].ID || got.Targets[0].State != conversation.TargetAnswered {
		t.Fatalf("target = %+v, want answered", got.Targets)
	}
	if got.Participants[0].SeatID != original.ID || got.Participants[0].Profile != original.Profile || got.Participants[0].Model != original.Model || got.Participants[0].Machine != original.Machine {
		t.Fatalf("persisted participant lost selected identity: %+v", got.Participants[0])
	}
	run, err := st.GetRun(posted.Targets[0].RunID)
	if err != nil {
		t.Fatal(err)
	}
	if run.Profile != original.Profile || run.Agent != original.Agent || run.Model != original.Model || run.Machine != original.Machine {
		t.Fatalf("persisted run lost selected identity: %+v", run)
	}
	dispatched := rt.Dispatched()
	if len(dispatched) != 1 || dispatched[0].Profile != original.Profile || dispatched[0].Model != original.Model || dispatched[0].Machine != original.Machine {
		t.Fatalf("dispatch lost persisted dynamic identity: %+v", dispatched)
	}
}

func TestConversationDispatchFailsDynamicModelDriftBeforeProviderStart(t *testing.T) {
	st := newStore(t)
	rt := fake.New()
	original := conversation.Seat{
		ID:      conversationSeatID("codex:configured-default", "studio.local", "gpt-5.6-sol"),
		Profile: "codex:configured-default", Agent: "codex", Model: "gpt-5.6-sol", Machine: "studio.local",
		DisplayName: "Codex · GPT-5.6 Sol (default) on studio.local", State: string(corecap.OfferReady),
	}
	seats := &mutableConversationSeats{seats: []conversation.Seat{original}}
	service := NewConversationService(st, rt, seats, t.TempDir())
	t.Cleanup(service.Close)
	detail, err := service.CreateConversation(context.Background(), "", "Pinned model", []string{original.ID})
	if err != nil {
		t.Fatal(err)
	}
	changed := original
	changed.ID = conversationSeatID(changed.Profile, changed.Machine, "gpt-5.6-terra")
	changed.Model = "gpt-5.6-terra"
	changed.DisplayName = "Codex · GPT-5.6 Terra (default) on studio.local"
	seats.Set(changed)
	posted, err := service.PostTurn(context.Background(), detail.Conversation.ID, "client-drift", "hello", []string{detail.Participants[0].ID})
	if err != nil {
		t.Fatal(err)
	}
	service.Wait()
	got, err := service.GetConversation(context.Background(), detail.Conversation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Targets) != 1 || got.Targets[0].ID != posted.Targets[0].ID || got.Targets[0].State != conversation.TargetFailed || got.Targets[0].ErrorCode != "seat_unready" || !strings.Contains(got.Targets[0].Error, string(corecap.ReasonCapabilityDrift)) {
		t.Fatalf("drift target = %+v", got.Targets)
	}
	if dispatched := rt.Dispatched(); len(dispatched) != 0 {
		t.Fatalf("drift started provider: %+v", dispatched)
	}
	if _, err := service.RetryTarget(context.Background(), posted.Targets[0].ID); err == nil || !strings.Contains(err.Error(), string(corecap.ReasonCapabilityDrift)) {
		t.Fatalf("retry drift error = %v, want capability_drift", err)
	}
}

func TestConversationDispatchFailsMissingDynamicModelBeforeProviderStart(t *testing.T) {
	st := newStore(t)
	rt := fake.New()
	original := conversation.Seat{
		ID:      conversationSeatID("codex:configured-default", "studio.local", "gpt-5.6-sol"),
		Profile: "codex:configured-default", Agent: "codex", Model: "gpt-5.6-sol", Machine: "studio.local",
		DisplayName: "Codex · GPT-5.6 Sol (default) on studio.local", State: string(corecap.OfferReady),
	}
	seats := &mutableConversationSeats{seats: []conversation.Seat{original}}
	service := NewConversationService(st, rt, seats, t.TempDir())
	t.Cleanup(service.Close)
	detail, err := service.CreateConversation(context.Background(), "", "Pinned model", []string{original.ID})
	if err != nil {
		t.Fatal(err)
	}
	missing := original
	missing.ID = conversationSeatID(missing.Profile, missing.Machine, "")
	missing.Model = ""
	missing.State = string(corecap.OfferSetupRequired)
	missing.Reason = string(corecap.ReasonModelUnavailable)
	seats.Set(missing)
	posted, err := service.PostTurn(context.Background(), detail.Conversation.ID, "client-missing-model", "hello", []string{detail.Participants[0].ID})
	if err != nil {
		t.Fatal(err)
	}
	service.Wait()
	got, err := service.GetConversation(context.Background(), detail.Conversation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Targets) != 1 || got.Targets[0].ID != posted.Targets[0].ID || got.Targets[0].State != conversation.TargetFailed || got.Targets[0].ErrorCode != "seat_unready" || !strings.Contains(got.Targets[0].Error, string(corecap.ReasonModelUnavailable)) {
		t.Fatalf("missing-model target = %+v", got.Targets)
	}
	if dispatched := rt.Dispatched(); len(dispatched) != 0 {
		t.Fatalf("missing model started provider: %+v", dispatched)
	}
}

func TestFakeConversationSeatsAreExplicitAndOpaque(t *testing.T) {
	first := FakeConversationSeats("studio").ConversationSeats()
	second := FakeConversationSeats("studio").ConversationSeats()
	if len(first) == 0 || len(first) != len(second) {
		t.Fatal("fake conversation inventory has no explicit seats")
	}
	for index, seat := range first {
		if seat.Model == "" {
			t.Fatalf("fake seat %+v advertises an ambient configured model", seat)
		}
		if seat.ID != second[index].ID || !strings.HasPrefix(seat.ID, "seat:v1:") ||
			strings.Contains(seat.ID, seat.Profile) || strings.Contains(seat.ID, seat.Model) || strings.Contains(seat.ID, seat.Machine) {
			t.Fatalf("fake seat identity is not opaque and stable: first=%+v second=%+v", seat, second[index])
		}
	}
}

func TestConversationRetryUsesOriginalTurnBoundary(t *testing.T) {
	st := newStore(t)
	rt := fake.New()
	rt.ExitCode = 2
	seat := conversation.Seat{
		ID:      conversationSeatID("codex:configured-default", "studio", "gpt-5.6-sol"),
		Profile: "codex:configured-default", Agent: "codex", Model: "gpt-5.6-sol", Machine: "studio",
		DisplayName: "Codex · GPT-5.6 Sol (default) on studio", State: string(corecap.OfferReady),
	}
	seats := fixedConversationSeats{seat}
	service := NewConversationService(st, rt, seats, t.TempDir())
	t.Cleanup(service.Close)
	detail, err := service.CreateConversation(context.Background(), "", "Thread", []string{seat.ID})
	if err != nil {
		t.Fatal(err)
	}
	posted, err := service.PostTurn(context.Background(), detail.Conversation.ID, "client-1", "original", []string{detail.Participants[0].ID})
	if err != nil {
		t.Fatal(err)
	}
	waitForConversationTargetState(t, service, detail.Conversation.ID, posted.Targets[0].ID, conversation.TargetFailed)
	if _, err := st.AppendConversationMessage(conversation.Message{
		ConversationID: detail.Conversation.ID, AuthorKind: conversation.AuthorHuman,
		AuthorID: "human", Body: "later message outside the original turn boundary",
	}); err != nil {
		t.Fatal(err)
	}

	rt.ExitCode = 0
	retried, err := service.RetryTarget(context.Background(), posted.Targets[0].ID)
	if err != nil {
		t.Fatalf("retry: %v", err)
	}
	waitForConversationTargetState(t, service, detail.Conversation.ID, retried.ID, conversation.TargetAnswered)
	specs := rt.Dispatched()
	if len(specs) != 2 || specs[0].Prompt != specs[1].Prompt {
		t.Fatalf("retry changed frozen prompt: %+v", specs)
	}
	for _, spec := range specs {
		if spec.Profile != seat.Profile || spec.Agent != seat.Agent || spec.Model != seat.Model || spec.Machine != seat.Machine {
			t.Fatalf("retry changed exact dynamic identity: %+v", specs)
		}
		if strings.Contains(spec.Prompt, "later message outside the original turn boundary") {
			t.Fatalf("retry included later conversation history: %s", spec.Prompt)
		}
	}
	for _, target := range []conversation.Target{posted.Targets[0], retried} {
		run, err := st.GetRun(target.RunID)
		if err != nil {
			t.Fatal(err)
		}
		if run.Profile != seat.Profile || run.Agent != seat.Agent || run.Model != seat.Model || run.Machine != seat.Machine {
			t.Fatalf("retry run changed exact dynamic identity: %+v", run)
		}
	}
	reloaded, err := service.GetConversation(context.Background(), detail.Conversation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(reloaded.Participants) != 1 || reloaded.Participants[0].SeatID != seat.ID ||
		reloaded.Participants[0].Profile != seat.Profile || reloaded.Participants[0].Model != seat.Model ||
		reloaded.Participants[0].Machine != seat.Machine {
		t.Fatalf("retry changed persisted participant identity: %+v", reloaded.Participants)
	}
}

func TestConversationRetryReportsClosedReasonWithoutCreatingOfflineAttempt(t *testing.T) {
	st := newStore(t)
	rt := fake.New()
	rt.ExitCode = 2
	seats := fixedConversationSeats{{
		ID: "codex@mini", Profile: "codex:gpt-5.6-sol", Agent: "codex", Model: "gpt-5.6-sol",
		Machine: "mini", DisplayName: "Codex Sol on mini", State: string(corecap.OfferReady),
	}}
	service := NewConversationService(st, rt, seats, t.TempDir())
	t.Cleanup(service.Close)
	detail, err := service.CreateConversation(context.Background(), "", "Offline retry", []string{"codex@mini"})
	if err != nil {
		t.Fatal(err)
	}
	posted, err := service.PostTurn(context.Background(), detail.Conversation.ID, "client-offline", "try once", []string{detail.Participants[0].ID})
	if err != nil {
		t.Fatal(err)
	}
	waitForConversationTargetState(t, service, detail.Conversation.ID, posted.Targets[0].ID, conversation.TargetFailed)

	seats[0].State = string(corecap.OfferUnavailable)
	seats[0].Reason = string(corecap.ReasonUnavailable)
	if _, err := service.RetryTarget(context.Background(), posted.Targets[0].ID); err == nil || !strings.Contains(err.Error(), string(corecap.ReasonUnavailable)) {
		t.Fatalf("offline retry error = %v, want closed %q reason", err, corecap.ReasonUnavailable)
	}
	got, err := service.GetConversation(context.Background(), detail.Conversation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Targets) != 1 {
		t.Fatalf("offline retry created a new attempt: %#v", got.Targets)
	}
	if dispatched := rt.Dispatched(); len(dispatched) != 1 {
		t.Fatalf("offline retry invoked runtime: %#v", dispatched)
	}
}

func waitForConversationTargetState(t *testing.T, service *ConversationService, conversationID, targetID string, want conversation.TargetState) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		detail, err := service.GetConversation(context.Background(), conversationID)
		if err != nil {
			t.Fatal(err)
		}
		for _, target := range detail.Targets {
			if target.ID == targetID && target.State == want {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("target %s did not reach %s", targetID, want)
}

func TestConversationClientTurnRetryIsIdempotentAtServiceBoundary(t *testing.T) {
	st := newStore(t)
	rt := fake.New()
	service := NewConversationService(st, rt, fixedConversationSeats{{ID: "codex@studio", Profile: "codex", Agent: "codex", Machine: "studio", State: "ready"}}, t.TempDir())
	t.Cleanup(service.Close)
	detail, err := service.CreateConversation(context.Background(), "", "Thread", []string{"codex@studio"})
	if err != nil {
		t.Fatal(err)
	}
	first, err := service.PostTurn(context.Background(), detail.Conversation.ID, "same-client-id", "hello", []string{detail.Participants[0].ID})
	if err != nil {
		t.Fatal(err)
	}
	waitForConversationTargetState(t, service, detail.Conversation.ID, first.Targets[0].ID, conversation.TargetAnswered)
	second, err := service.PostTurn(context.Background(), detail.Conversation.ID, "same-client-id", "hello", []string{detail.Participants[0].ID})
	if err != nil {
		t.Fatal(err)
	}
	if second.Turn.ID != first.Turn.ID || len(rt.Dispatched()) != 1 {
		t.Fatalf("duplicate turn dispatched: first=%+v second=%+v specs=%+v", first, second, rt.Dispatched())
	}
}

func TestConversationClientTurnRetryRemainsIdempotentAfterParticipantRemoval(t *testing.T) {
	st := newStore(t)
	rt := fake.New()
	service := NewConversationService(st, rt, fixedConversationSeats{{
		ID: "codex@studio", Profile: "codex", Agent: "codex", Machine: "studio", State: "ready",
	}}, t.TempDir())
	t.Cleanup(service.Close)
	detail, err := service.CreateConversation(context.Background(), "", "Thread", []string{"codex@studio"})
	if err != nil {
		t.Fatal(err)
	}
	participantID := detail.Participants[0].ID
	first, err := service.PostTurn(context.Background(), detail.Conversation.ID, "durable-client-id", "hello", []string{participantID})
	if err != nil {
		t.Fatal(err)
	}
	waitForConversationTargetState(t, service, detail.Conversation.ID, first.Targets[0].ID, conversation.TargetAnswered)
	if err := service.RemoveConversationParticipant(context.Background(), detail.Conversation.ID, participantID); err != nil {
		t.Fatal(err)
	}

	second, err := service.PostTurn(context.Background(), detail.Conversation.ID, "durable-client-id", "hello", []string{participantID})
	if err != nil {
		t.Fatalf("retry accepted turn after participant removal: %v", err)
	}
	if second.Turn.ID != first.Turn.ID || len(second.Targets) != 1 || second.Targets[0].ID != first.Targets[0].ID {
		t.Fatalf("retry returned a different durable turn: first=%+v second=%+v", first, second)
	}
	if got := len(rt.Dispatched()); got != 1 {
		t.Fatalf("retry after participant removal dispatched %d runtimes, want 1 original dispatch", got)
	}
}

func TestConversationClusterCombinesLocalAndRemoteAnswers(t *testing.T) {
	st := newStore(t)
	local := fake.New()
	remote := fake.New()
	rt := cluster.New("studio", local, map[string]runtime.Runtime{"mini": remote})
	seats := fixedConversationSeats{
		{ID: "codex@studio", Profile: "codex", Agent: "codex", Machine: "studio", DisplayName: "Codex on Studio", State: "ready"},
		{ID: "hermes@mini", Profile: "hermes", Agent: "hermes", Machine: "mini", DisplayName: "Hermes on Mini", State: "ready"},
	}
	service := NewConversationService(st, rt, seats, t.TempDir())
	t.Cleanup(service.Close)
	detail, err := service.CreateConversation(context.Background(), "", "Two computers", []string{"codex@studio", "hermes@mini"})
	if err != nil {
		t.Fatal(err)
	}
	turn, err := service.PostTurn(context.Background(), detail.Conversation.ID, "client-two", "answer together", []string{detail.Participants[0].ID, detail.Participants[1].ID})
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range turn.Targets {
		waitForConversationTargetState(t, service, detail.Conversation.ID, target.ID, conversation.TargetAnswered)
	}
	got, _ := service.GetConversation(context.Background(), detail.Conversation.ID)
	if len(got.Messages) != 3 || len(local.Dispatched()) != 1 || len(remote.Dispatched()) != 1 {
		t.Fatalf("detail=%+v local=%+v remote=%+v", got, local.Dispatched(), remote.Dispatched())
	}
	if got.Messages[1].AuthorID == got.Messages[2].AuthorID {
		t.Fatalf("answers lost participant attribution: %+v", got.Messages)
	}
}

func TestConversationAnswerSurvivesStoreRestartAndFeedsNextPrompt(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "fort.db")
	firstStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	firstOpen := true
	seat := conversation.Seat{
		ID: "codex@studio", Profile: "codex", Agent: "codex", Machine: "studio",
		DisplayName: "Codex on Studio", State: "ready",
	}
	firstRuntime := fake.New()
	firstService := NewConversationService(firstStore, firstRuntime, fixedConversationSeats{seat}, t.TempDir())
	t.Cleanup(func() {
		if firstOpen {
			firstService.Close()
			_ = firstStore.Close()
		}
	})

	detail, err := firstService.CreateConversation(context.Background(), "", "Restarted context", []string{seat.ID})
	if err != nil {
		t.Fatal(err)
	}
	firstTurn, err := firstService.PostTurn(context.Background(), detail.Conversation.ID, "client-before-restart", "Remember this request.", []string{detail.Participants[0].ID})
	if err != nil {
		t.Fatal(err)
	}
	waitForConversationTargetState(t, firstService, detail.Conversation.ID, firstTurn.Targets[0].ID, conversation.TargetAnswered)
	firstService.Wait()
	beforeRestart, err := firstService.GetConversation(context.Background(), detail.Conversation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(beforeRestart.Messages) != 2 || beforeRestart.Messages[1].AuthorKind != conversation.AuthorAssistant || strings.TrimSpace(beforeRestart.Messages[1].Body) == "" {
		t.Fatalf("completed transcript = %+v, want one durable attributed answer", beforeRestart.Messages)
	}
	persistedAnswer := beforeRestart.Messages[1]

	firstService.Close()
	if err := firstStore.Close(); err != nil {
		t.Fatal(err)
	}
	firstOpen = false

	reopenedStore, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopenedStore.Close() })
	restartedRuntime := fake.New()
	restartedService := NewConversationService(reopenedStore, restartedRuntime, fixedConversationSeats{seat}, t.TempDir())
	t.Cleanup(restartedService.Close)

	restored, err := restartedService.GetConversation(context.Background(), detail.Conversation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(restored.Messages) != 2 || restored.Messages[1].ID != persistedAnswer.ID || restored.Messages[1].AuthorID != persistedAnswer.AuthorID || restored.Messages[1].Body != persistedAnswer.Body {
		t.Fatalf("restored transcript = %+v, want persisted answer %+v", restored.Messages, persistedAnswer)
	}
	if len(restored.Targets) != 1 || restored.Targets[0].State != conversation.TargetAnswered {
		t.Fatalf("restored targets = %+v, want answered target", restored.Targets)
	}

	secondTurn, err := restartedService.PostTurn(context.Background(), detail.Conversation.ID, "client-after-restart", "Build on your prior answer.", []string{detail.Participants[0].ID})
	if err != nil {
		t.Fatal(err)
	}
	waitForConversationTargetState(t, restartedService, detail.Conversation.ID, secondTurn.Targets[0].ID, conversation.TargetAnswered)
	dispatched := restartedRuntime.Dispatched()
	if len(dispatched) != 1 {
		t.Fatalf("restarted dispatches = %+v, want one provider prompt", dispatched)
	}
	var providerPrompt struct {
		Context struct {
			Messages []struct {
				ID         int64                   `json:"id"`
				AuthorKind conversation.AuthorKind `json:"author_kind"`
				AuthorID   string                  `json:"author_id"`
				Body       string                  `json:"body"`
			} `json:"messages"`
		} `json:"context"`
	}
	if err := json.Unmarshal([]byte(dispatched[0].Prompt), &providerPrompt); err != nil {
		t.Fatalf("decode provider prompt after restart: %v\n%s", err, dispatched[0].Prompt)
	}
	if len(providerPrompt.Context.Messages) != 3 {
		t.Fatalf("provider context messages = %+v, want first request, persisted answer, and next request", providerPrompt.Context.Messages)
	}
	priorAnswer := providerPrompt.Context.Messages[1]
	if priorAnswer.ID != persistedAnswer.ID || priorAnswer.AuthorKind != conversation.AuthorAssistant || priorAnswer.AuthorID != persistedAnswer.AuthorID || priorAnswer.Body != persistedAnswer.Body {
		t.Fatalf("provider context prior answer = %+v, want persisted answer %+v", priorAnswer, persistedAnswer)
	}
	if got := providerPrompt.Context.Messages[2]; got.AuthorKind != conversation.AuthorHuman || got.Body != "Build on your prior answer." {
		t.Fatalf("provider context latest request = %+v", got)
	}
}

func TestConversationSuccessfulExitWithoutMessageFailsMissingTerminalOutput(t *testing.T) {
	st := newStore(t)
	service := NewConversationService(st, successfulExitWithoutMessageRuntime{}, fixedConversationSeats{{
		ID: "codex@studio", Profile: "codex", Agent: "codex", Machine: "studio", DisplayName: "Codex on Studio", State: "ready",
	}}, t.TempDir())
	t.Cleanup(service.Close)
	detail, err := service.CreateConversation(context.Background(), "", "Missing answer", []string{"codex@studio"})
	if err != nil {
		t.Fatal(err)
	}
	posted, err := service.PostTurn(context.Background(), detail.Conversation.ID, "client-missing-answer", "Answer this.", []string{detail.Participants[0].ID})
	if err != nil {
		t.Fatal(err)
	}
	waitForConversationTargetState(t, service, detail.Conversation.ID, posted.Targets[0].ID, conversation.TargetFailed)

	got, err := service.GetConversation(context.Background(), detail.Conversation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Targets) != 1 || got.Targets[0].ErrorCode != "missing_terminal_output" || got.Targets[0].Error != "provider completed without an attributed answer" {
		t.Fatalf("target = %+v, want missing_terminal_output failure", got.Targets)
	}
	if len(got.Messages) != 1 || got.Messages[0].AuthorKind != conversation.AuthorHuman || got.Messages[0].Body != "Answer this." {
		t.Fatalf("messages = %+v, want only the durable human request", got.Messages)
	}
	run, err := st.GetRun(posted.Targets[0].RunID)
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != "failed" || run.ExitCode != 0 || run.Error != "provider completed without an attributed answer" {
		t.Fatalf("run = %+v, want durable failed status for missing terminal output", run)
	}
	events, err := st.Events(posted.Targets[0].RunID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].Type != string(runtime.EventStarted) || events[1].Type != string(runtime.EventExited) {
		t.Fatalf("events = %+v, want successful start/exit without message", events)
	}
}

func TestConversationRemoteFailuresPersistOneFailedTargetWithoutAssistantMessage(t *testing.T) {
	tests := []struct {
		name      string
		terminal  runtime.RunEvent
		wantError string
	}{
		{
			name:      "stream ends before terminal event",
			wantError: "remote stream ended before completion",
		},
		{
			name:      "provider reports an error",
			terminal:  runtime.RunEvent{Type: runtime.EventError, Data: "provider exhausted retries"},
			wantError: "provider exhausted retries",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("POST /api/exec", func(w http.ResponseWriter, r *http.Request) {
				var spec runtime.RunSpec
				if err := json.NewDecoder(r.Body).Decode(&spec); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				frames := []runtime.RunEvent{
					{RunID: spec.RunID, Type: runtime.EventStarted, Time: time.Now(), Data: "codex"},
					{RunID: spec.RunID, Type: runtime.EventMessage, Time: time.Now(), Data: "provisional answer"},
				}
				if tt.terminal.Type != "" {
					terminal := tt.terminal
					terminal.RunID = spec.RunID
					terminal.Time = time.Now()
					frames = append(frames, terminal)
				}
				for _, frame := range frames {
					if err := json.NewEncoder(w).Encode(frame); err != nil {
						return
					}
					w.(http.Flusher).Flush()
				}
			})
			remoteServer := httptest.NewServer(mux)
			t.Cleanup(remoteServer.Close)

			st := newStore(t)
			remoteRuntime := remoteexec.New("mini", remoteServer.URL, "tok")
			rt := cluster.New("studio", fake.New(), map[string]runtime.Runtime{"mini": remoteRuntime})
			service := NewConversationService(st, rt, fixedConversationSeats{{
				ID: "codex@mini", Profile: "codex", Agent: "codex", Machine: "mini", DisplayName: "Codex on Mini", State: "ready",
			}}, t.TempDir())
			t.Cleanup(service.Close)

			detail, err := service.CreateConversation(context.Background(), "", "Remote failure", []string{"codex@mini"})
			if err != nil {
				t.Fatal(err)
			}
			posted, err := service.PostTurn(context.Background(), detail.Conversation.ID, "client-remote-failure", "fail here", []string{detail.Participants[0].ID})
			if err != nil {
				t.Fatal(err)
			}

			waitDone := make(chan struct{})
			go func() {
				service.Wait()
				close(waitDone)
			}()
			select {
			case <-waitDone:
			case <-time.After(2 * time.Second):
				t.Fatal("remote conversation target did not terminate")
			}

			got, err := service.GetConversation(context.Background(), detail.Conversation.ID)
			if err != nil {
				t.Fatal(err)
			}
			if len(got.Targets) != 1 {
				t.Fatalf("targets = %+v, want exactly one failed target", got.Targets)
			}
			target := got.Targets[0]
			if len(posted.Targets) != 1 || target.ID != posted.Targets[0].ID || target.State != conversation.TargetFailed || target.ErrorCode != "provider_failed" || !strings.Contains(target.Error, tt.wantError) {
				t.Fatalf("target = %+v, posted = %+v, want one provider failure containing %q", target, posted.Targets, tt.wantError)
			}
			if len(got.Messages) != 1 || got.Messages[0].AuthorKind != conversation.AuthorHuman || got.Messages[0].Body != "fail here" {
				t.Fatalf("messages = %+v, want only the durable human message", got.Messages)
			}

			persistedRun, err := st.GetRun(target.RunID)
			if err != nil {
				t.Fatal(err)
			}
			if persistedRun.Status != "failed" || persistedRun.ExitCode != -1 || !strings.Contains(persistedRun.Error, tt.wantError) {
				t.Fatalf("run = %+v, want durable failed/-1 evidence containing %q", persistedRun, tt.wantError)
			}
			events, err := st.Events(target.RunID)
			if err != nil {
				t.Fatal(err)
			}
			if len(events) != 3 || events[0].Type != string(runtime.EventStarted) || events[1].Type != string(runtime.EventMessage) || events[2].Type != string(runtime.EventError) || !strings.Contains(events[2].Data, tt.wantError) {
				t.Fatalf("events = %+v, want persisted started/message/error evidence containing %q", events, tt.wantError)
			}
			for _, event := range events {
				if event.RunID != target.RunID {
					t.Fatalf("event run ID = %q, want %q", event.RunID, target.RunID)
				}
			}
			if service.ConversationTargetActive(target.ID) {
				t.Fatalf("failed target %s still has an active remote run", target.ID)
			}
		})
	}
}

func TestCancelingOneConversationTargetLeavesPeerWorking(t *testing.T) {
	st := newStore(t)
	local, remote := fake.New(), fake.New()
	local.Block, remote.Block = true, true
	rt := cluster.New("studio", local, map[string]runtime.Runtime{"mini": remote})
	service := NewConversationService(st, rt, fixedConversationSeats{
		{ID: "codex@studio", Profile: "codex", Agent: "codex", Machine: "studio", State: "ready"},
		{ID: "hermes@mini", Profile: "hermes", Agent: "hermes", Machine: "mini", State: "ready"},
	}, t.TempDir())
	t.Cleanup(service.Close)
	detail, err := service.CreateConversation(context.Background(), "", "Cancelable", []string{"codex@studio", "hermes@mini"})
	if err != nil {
		t.Fatal(err)
	}
	turn, err := service.PostTurn(context.Background(), detail.Conversation.ID, "client-cancel", "keep one going", []string{detail.Participants[0].ID, detail.Participants[1].ID})
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range turn.Targets {
		waitForConversationTargetState(t, service, detail.Conversation.ID, target.ID, conversation.TargetWorking)
	}
	if err := service.CancelTarget(context.Background(), turn.Targets[0].ID); err != nil {
		t.Fatal(err)
	}
	waitForConversationTargetState(t, service, detail.Conversation.ID, turn.Targets[0].ID, conversation.TargetCanceled)
	got, _ := service.GetConversation(context.Background(), detail.Conversation.ID)
	var peer conversation.Target
	for _, target := range got.Targets {
		if target.ID == turn.Targets[1].ID {
			peer = target
			break
		}
	}
	if peer.ID == "" || peer.State != conversation.TargetWorking || !service.ConversationTargetActive(turn.Targets[1].ID) {
		t.Fatalf("peer stopped with canceled target: %+v", got.Targets)
	}
	if err := service.CancelTarget(context.Background(), turn.Targets[1].ID); err != nil {
		t.Fatal(err)
	}
	waitForConversationTargetState(t, service, detail.Conversation.ID, turn.Targets[1].ID, conversation.TargetCanceled)
}

func TestConversationServiceShutdownFailsActiveTargetWithoutClaimingUserCancellation(t *testing.T) {
	st := newStore(t)
	rt := fake.New()
	rt.Block = true
	seats := fixedConversationSeats{{
		ID: "codex@studio", Profile: "codex", Agent: "codex", Machine: "studio", State: "ready",
	}}
	service := NewConversationService(st, rt, seats, t.TempDir())
	t.Cleanup(service.Close)
	detail, err := service.CreateConversation(context.Background(), "", "Restart-safe thread", []string{"codex@studio"})
	if err != nil {
		t.Fatal(err)
	}
	first, err := service.PostTurn(context.Background(), detail.Conversation.ID, "client-before-restart", "remember this", []string{detail.Participants[0].ID})
	if err != nil {
		t.Fatal(err)
	}
	waitForConversationTargetState(t, service, detail.Conversation.ID, first.Targets[0].ID, conversation.TargetWorking)

	service.Close()
	afterClose, err := service.GetConversation(context.Background(), detail.Conversation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := afterClose.Targets[0]; got.State != conversation.TargetFailed || got.ErrorCode != "daemon_interrupted" {
		t.Fatalf("shutdown target = %+v, want failed daemon interruption", got)
	}
	if len(afterClose.Messages) != 1 || afterClose.Messages[0].Body != "remember this" {
		t.Fatalf("shutdown invented or lost transcript messages: %+v", afterClose.Messages)
	}
	interruptedRun, err := st.GetRun(first.Targets[0].RunID)
	if err != nil {
		t.Fatal(err)
	}
	if interruptedRun.Status != "failed" || !strings.Contains(interruptedRun.Error, "daemon stopped") {
		t.Fatalf("shutdown run = %+v, want durable failure", interruptedRun)
	}

	resumed := NewConversationService(st, fake.New(), seats, t.TempDir())
	t.Cleanup(resumed.Close)
	second, err := resumed.PostTurn(context.Background(), detail.Conversation.ID, "client-after-restart", "continue from that", []string{detail.Participants[0].ID})
	if err != nil {
		t.Fatal(err)
	}
	waitForConversationTargetState(t, resumed, detail.Conversation.ID, second.Targets[0].ID, conversation.TargetAnswered)
	restored, err := resumed.GetConversation(context.Background(), detail.Conversation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(restored.Messages) != 3 || restored.Messages[0].Body != "remember this" || restored.Messages[1].Body != "continue from that" {
		t.Fatalf("restart transcript = %+v", restored.Messages)
	}
}

func TestConversationServiceShutdownBeforeRunCreationFailsLateRun(t *testing.T) {
	st := newStore(t)
	rt := fake.New()
	seat := conversation.Seat{ID: "codex@studio", Profile: "codex", Agent: "codex", Machine: "studio", State: "ready"}
	service := NewConversationService(st, rt, fixedConversationSeats{seat}, t.TempDir())
	t.Cleanup(service.Close)
	detail, err := service.CreateConversation(context.Background(), "", "Shutdown before run", []string{seat.ID})
	if err != nil {
		t.Fatal(err)
	}
	blockedSeats := &blockingConversationSeats{
		seats: []conversation.Seat{seat}, entered: make(chan struct{}), release: make(chan struct{}),
	}
	t.Cleanup(blockedSeats.Unblock)
	service.seats = blockedSeats
	turn, err := service.PostTurn(context.Background(), detail.Conversation.ID, "client-shutdown-before-run", "do not leave a ghost run", []string{detail.Participants[0].ID})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-blockedSeats.entered:
	case <-time.After(time.Second):
		t.Fatal("target startup did not reach seat validation")
	}

	closeDone := make(chan struct{})
	go func() {
		service.Close()
		close(closeDone)
	}()
	waitForConversationTargetState(t, service, detail.Conversation.ID, turn.Targets[0].ID, conversation.TargetFailed)
	blockedSeats.Unblock()
	select {
	case <-closeDone:
	case <-time.After(time.Second):
		t.Fatal("conversation service shutdown did not finish")
	}

	if dispatched := rt.Dispatched(); len(dispatched) != 0 {
		t.Fatalf("shutdown-failed target dispatched provider: %+v", dispatched)
	}
	run, err := st.GetRun(turn.Targets[0].RunID)
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != "failed" || !strings.Contains(run.Error, "daemon stopped") {
		t.Fatalf("late-created run = %+v, want durable shutdown failure", run)
	}
}

func TestCanceledQueuedConversationTargetNeverDispatchesProvider(t *testing.T) {
	st := newStore(t)
	rt := fake.New()
	service := NewConversationService(st, rt, fixedConversationSeats{{
		ID: "codex@studio", Profile: "codex", Agent: "codex", Machine: "studio", State: "ready",
	}}, t.TempDir())
	t.Cleanup(service.Close)
	detail, err := service.CreateConversation(context.Background(), "", "Cancelable before startup", []string{"codex@studio"})
	if err != nil {
		t.Fatal(err)
	}
	_, targets, prompt, err := st.CreateConversationTurn(store.CreateConversationTurnParams{
		TurnID: "turn-cancel-before-start", ClientTurnID: "client-cancel-before-start",
		ConversationID: detail.Conversation.ID, HumanID: "human", Body: "do not launch",
		Targets: []store.ConversationTurnTarget{{
			ID: "target-cancel-before-start", ParticipantID: detail.Participants[0].ID, RunID: "run-cancel-before-start",
		}},
		CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	dispatch, err := st.GetConversationTargetDispatch(targets[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.CancelTarget(context.Background(), targets[0].ID); err != nil {
		t.Fatal(err)
	}
	service.runTarget(dispatch, prompt)
	if dispatched := rt.Dispatched(); len(dispatched) != 0 {
		t.Fatalf("canceled queued target dispatched provider: %+v", dispatched)
	}
	got, err := service.GetConversation(context.Background(), detail.Conversation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Targets[0].State != conversation.TargetCanceled {
		t.Fatalf("target state = %s, want canceled", got.Targets[0].State)
	}
}

func TestCancelingQueuedConversationTargetInterruptsProviderStartup(t *testing.T) {
	st := newStore(t)
	rt := &cancelableConversationDispatchRuntime{
		entered: make(chan struct{}), canceled: make(chan struct{}), release: make(chan struct{}),
	}
	t.Cleanup(func() { close(rt.release) })
	service := NewConversationService(st, rt, fixedConversationSeats{{
		ID: "codex@studio", Profile: "codex", Agent: "codex", Machine: "studio", State: "ready",
	}}, t.TempDir())
	t.Cleanup(service.Close)
	detail, err := service.CreateConversation(context.Background(), "", "Cancelable startup", []string{"codex@studio"})
	if err != nil {
		t.Fatal(err)
	}
	turn, err := service.PostTurn(context.Background(), detail.Conversation.ID, "client-cancel-startup", "stop startup", []string{detail.Participants[0].ID})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-rt.entered:
	case <-time.After(time.Second):
		t.Fatal("provider dispatch did not begin")
	}
	cancelDone := make(chan error, 1)
	go func() {
		cancelDone <- service.CancelTarget(context.Background(), turn.Targets[0].ID)
	}()
	select {
	case err := <-cancelDone:
		if err != nil {
			t.Fatalf("cancel target: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancel target blocked behind provider startup")
	}
	select {
	case <-rt.canceled:
	case <-time.After(time.Second):
		t.Fatal("provider startup context was not canceled")
	}
	service.Wait()
	run, err := st.GetRun(turn.Targets[0].RunID)
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != "canceled" {
		t.Fatalf("run status = %q, want canceled", run.Status)
	}
	got, err := service.GetConversation(context.Background(), detail.Conversation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Targets[0].State != conversation.TargetCanceled {
		t.Fatalf("target state = %s, want canceled", got.Targets[0].State)
	}
}

func TestCancelingQueuedConversationTargetBeforeRunCreationSkipsDispatch(t *testing.T) {
	st := newStore(t)
	rt := fake.New()
	seat := conversation.Seat{ID: "codex@studio", Profile: "codex", Agent: "codex", Machine: "studio", State: "ready"}
	service := NewConversationService(st, rt, fixedConversationSeats{seat}, t.TempDir())
	t.Cleanup(service.Close)
	detail, err := service.CreateConversation(context.Background(), "", "Cancel before run", []string{seat.ID})
	if err != nil {
		t.Fatal(err)
	}
	blockedSeats := &blockingConversationSeats{
		seats: []conversation.Seat{seat}, entered: make(chan struct{}), release: make(chan struct{}),
	}
	t.Cleanup(blockedSeats.Unblock)
	service.seats = blockedSeats
	turn, err := service.PostTurn(context.Background(), detail.Conversation.ID, "client-cancel-before-run", "do not dispatch", []string{detail.Participants[0].ID})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-blockedSeats.entered:
	case <-time.After(time.Second):
		t.Fatal("target startup did not reach seat validation")
	}
	if err := service.CancelTarget(context.Background(), turn.Targets[0].ID); err != nil {
		t.Fatal(err)
	}
	blockedSeats.Unblock()
	service.Wait()
	if dispatched := rt.Dispatched(); len(dispatched) != 0 {
		t.Fatalf("canceled target dispatched after cancellation won startup: %+v", dispatched)
	}
	run, err := st.GetRun(turn.Targets[0].RunID)
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != "canceled" {
		t.Fatalf("run status = %q, want canceled", run.Status)
	}
}

func TestCancelingQueuedConversationTargetRetriesAfterItStartsWorking(t *testing.T) {
	st := newStore(t)
	rt := fake.New()
	rt.Block = true
	service := NewConversationService(st, rt, fixedConversationSeats{{
		ID: "codex@studio", Profile: "codex", Agent: "codex", Machine: "studio", State: "ready",
	}}, t.TempDir())
	t.Cleanup(service.Close)
	detail, err := service.CreateConversation(context.Background(), "", "Cancel crossover", []string{"codex@studio"})
	if err != nil {
		t.Fatal(err)
	}
	_, targets, _, err := st.CreateConversationTurn(store.CreateConversationTurnParams{
		TurnID: "turn-cancel-crossover", ClientTurnID: "client-cancel-crossover",
		ConversationID: detail.Conversation.ID, HumanID: "human", Body: "stop during startup",
		Targets: []store.ConversationTurnTarget{{
			ID: "target-cancel-crossover", ParticipantID: detail.Participants[0].ID, RunID: "run-cancel-crossover",
		}},
		CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	target := targets[0]
	if err := st.CreateRun(store.Run{
		ID: target.RunID, Title: detail.Conversation.Title, Agent: "codex", Status: "running",
		Machine: "studio", CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	run, err := rt.Dispatch(context.Background(), runtime.RunSpec{RunID: target.RunID, Agent: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	startup := service.targetStartup(target.ID)
	service.mu.Lock()
	service.active[target.ID] = run
	service.mu.Unlock()

	stale, err := st.GetConversationTargetDispatch(target.ID)
	if err != nil {
		t.Fatal(err)
	}
	changed, err := st.TransitionConversationTarget(target.ID, conversation.TargetQueued, conversation.TargetWorking, "")
	if err != nil || !changed {
		t.Fatalf("start target: changed=%v err=%v", changed, err)
	}

	if err := service.cancelTargetFromDispatch(stale, startup); err != nil {
		t.Fatalf("cancel stale queued target: %v", err)
	}
	if status := run.Wait(); status.State != runtime.StateCanceled {
		t.Fatalf("owner state = %s, want canceled", status.State)
	}
	if startup.ctx.Err() != context.Canceled {
		t.Fatalf("startup context error = %v, want context canceled", startup.ctx.Err())
	}
	got, err := service.GetConversation(context.Background(), detail.Conversation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Targets[0].State != conversation.TargetCanceled {
		t.Fatalf("target state = %s, want canceled", got.Targets[0].State)
	}
	storedRun, err := st.GetRun(target.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if storedRun.Status != "canceled" {
		t.Fatalf("stored run status = %q, want canceled", storedRun.Status)
	}
}

func TestDuplicateConversationTargetStartupCannotCancelTheOwner(t *testing.T) {
	service := NewConversationService(newStore(t), nil, nil, t.TempDir())
	t.Cleanup(service.Close)
	startup := service.targetStartup("target-owned-startup")
	startup.mu.Lock()
	startup.claimed = true
	startup.mu.Unlock()
	service.runTarget(store.ConversationTargetDispatch{Target: conversation.Target{ID: "target-owned-startup"}}, "")
	if err := startup.ctx.Err(); err != nil {
		t.Fatalf("duplicate startup canceled the owner context: %v", err)
	}
	if got := service.lookupTargetStartup("target-owned-startup"); got != startup {
		t.Fatal("duplicate startup removed the owner registration")
	}
	service.releaseTargetStartup("target-owned-startup", startup)
}

func TestLateProviderTerminalStatusCannotOverwriteDurableTargetOutcome(t *testing.T) {
	newTarget := func(t *testing.T, st *store.Store, suffix string) conversation.Target {
		t.Helper()
		now := time.Now().UTC()
		conversationID := "conversation-" + suffix
		participantID := "participant-" + suffix
		targetID := "target-" + suffix
		runID := "run-" + suffix
		if err := st.CreateConversation(conversation.Conversation{ID: conversationID, Title: "Terminal race", CreatedAt: now, UpdatedAt: now}, []conversation.Participant{{
			ID: participantID, ConversationID: conversationID, SeatID: "codex@studio", Profile: "codex", Agent: "codex", Machine: "studio", CreatedAt: now,
		}}); err != nil {
			t.Fatal(err)
		}
		_, targets, _, err := st.CreateConversationTurn(store.CreateConversationTurnParams{
			TurnID: "turn-" + suffix, ConversationID: conversationID, HumanID: "human", Body: "hello",
			Targets: []store.ConversationTurnTarget{{ID: targetID, ParticipantID: participantID, RunID: runID}}, CreatedAt: now,
		})
		if err != nil {
			t.Fatal(err)
		}
		return targets[0]
	}

	t.Run("late failure cannot replace user cancellation", func(t *testing.T) {
		st := newStore(t)
		service := NewConversationService(st, nil, nil, t.TempDir())
		t.Cleanup(service.Close)
		target := newTarget(t, st, "canceled")
		if err := st.CreateRun(store.Run{ID: target.RunID, Title: "Terminal race", Agent: "codex", Status: "canceled", Machine: "studio", CreatedAt: time.Now().UTC()}); err != nil {
			t.Fatal(err)
		}
		changed, err := st.TransitionConversationTargetWithCode(target.ID, conversation.TargetQueued, conversation.TargetCanceled, "canceled_by_user", "canceled by user")
		if err != nil || !changed {
			t.Fatalf("cancel target: changed=%v err=%v", changed, err)
		}
		service.failTargetCode(target.ID, target.RunID, conversation.TargetWorking, "provider_failed", "late provider failure", 2)
		run, err := st.GetRun(target.RunID)
		if err != nil {
			t.Fatal(err)
		}
		if run.Status != "canceled" {
			t.Fatalf("run status = %q, want canceled", run.Status)
		}
	})

	t.Run("late cancellation cannot replace answer", func(t *testing.T) {
		st := newStore(t)
		service := NewConversationService(st, nil, nil, t.TempDir())
		t.Cleanup(service.Close)
		target := newTarget(t, st, "answered")
		if err := st.CreateRun(store.Run{ID: target.RunID, Title: "Terminal race", Agent: "codex", Status: "succeeded", Machine: "studio", CreatedAt: time.Now().UTC()}); err != nil {
			t.Fatal(err)
		}
		changed, err := st.TransitionConversationTarget(target.ID, conversation.TargetQueued, conversation.TargetWorking, "")
		if err != nil || !changed {
			t.Fatalf("start target: changed=%v err=%v", changed, err)
		}
		dispatch, err := st.GetConversationTargetDispatch(target.ID)
		if err != nil {
			t.Fatal(err)
		}
		changed, err = st.AnswerConversationTarget(target.ID, conversation.Message{
			ConversationID: dispatch.Conversation.ID, TurnID: dispatch.Turn.ID, TargetID: target.ID,
			AuthorKind: conversation.AuthorAssistant, AuthorID: dispatch.Participant.ID, Body: "done", CreatedAt: time.Now().UTC(),
		})
		if err != nil || !changed {
			t.Fatalf("answer target: changed=%v err=%v", changed, err)
		}
		service.cancelTerminalTarget(target.ID, target.RunID, conversation.TargetWorking, "late cancellation")
		run, err := st.GetRun(target.RunID)
		if err != nil {
			t.Fatal(err)
		}
		if run.Status != "succeeded" {
			t.Fatalf("run status = %q, want succeeded", run.Status)
		}
	})
}
