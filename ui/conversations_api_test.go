package ui_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tobsai/fort/control"
	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/scheduler"
	"github.com/tobsai/fort/core/store"
	"github.com/tobsai/fort/exec/fake"
	"github.com/tobsai/fort/ui"
)

type apiSeats []conversation.Seat

func (s apiSeats) ConversationSeats() []conversation.Seat {
	return append([]conversation.Seat(nil), s...)
}

type apiSeatRechecker struct {
	seats apiSeats
	index int
	calls int
}

func (r *apiSeatRechecker) RecheckConversationSeats(context.Context) error {
	r.calls++
	r.seats[r.index].State = "ready"
	r.seats[r.index].Reason = ""
	return nil
}

type apiSchedules struct {
	created       []scheduler.Definition
	persistedNext time.Time
}

func (s *apiSchedules) Create(_ context.Context, definition scheduler.Definition) (scheduler.Definition, error) {
	if !s.persistedNext.IsZero() {
		definition.NextFireAt = s.persistedNext
	}
	s.created = append(s.created, definition)
	return definition, nil
}

func TestConversationAndTodayAPI(t *testing.T) {
	st := openStore(t)
	rt := fake.New()
	conversationService := control.NewConversationService(st, rt, apiSeats{{
		ID: "codex@studio", Profile: "codex", Agent: "codex", Machine: "studio", DisplayName: "Codex on Studio", State: "ready",
	}}, t.TempDir())
	t.Cleanup(conversationService.Close)
	todayService := control.NewTodayService(st, conversationService)
	displayLocation, err := time.LoadLocation("America/Chicago")
	if err != nil {
		t.Fatal(err)
	}
	srv := ui.New(ui.Deps{
		Dispatcher: control.NewQueueDispatcher(st), Store: st,
		Conversations: conversationService, Today: todayService, TodayLocation: displayLocation,
	})
	mux := http.NewServeMux()
	srv.Register(mux)

	seats := requestJSON(t, mux, http.MethodGet, "/api/conversation-seats", nil, http.StatusOK)
	if !bytes.Contains(seats, []byte(`"id":"codex@studio"`)) {
		t.Fatalf("seats = %s", seats)
	}
	projectJSON := requestJSON(t, mux, http.MethodPost, "/api/projects", map[string]any{"name": "Fort"}, http.StatusCreated)
	var project conversation.Project
	if err := json.Unmarshal(projectJSON, &project); err != nil {
		t.Fatal(err)
	}
	conversationJSON := requestJSON(t, mux, http.MethodPost, "/api/conversations", map[string]any{
		"project_id": project.ID, "title": "Shared thread", "seat_ids": []string{"codex@studio"},
	}, http.StatusCreated)
	var detail struct {
		Conversation conversation.Conversation  `json:"conversation"`
		Participants []conversation.Participant `json:"participants"`
	}
	if err := json.Unmarshal(conversationJSON, &detail); err != nil {
		t.Fatal(err)
	}
	requestJSON(t, mux, http.MethodPost, "/api/conversations/"+detail.Conversation.ID+"/turns", map[string]any{
		"client_turn_id": "client-1", "text": "hello", "participant_ids": []string{detail.Participants[0].ID},
	}, http.StatusAccepted)
	today := requestJSON(t, mux, http.MethodGet, "/api/today?timezone=UTC", nil, http.StatusOK)
	if !bytes.Contains(today, []byte(`"timezone":"America/Chicago"`)) || !bytes.Contains(today, []byte(`"scheduled":[]`)) {
		t.Fatalf("today = %s", today)
	}
}

func TestDeleteActiveConversationReturnsBoundedConflict(t *testing.T) {
	st := openStore(t)
	now := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)
	conv := conversation.Conversation{ID: "c-active", Title: "Active thread", CreatedAt: now, UpdatedAt: now}
	participant := conversation.Participant{
		ID: "p-active", ConversationID: conv.ID, SeatID: "codex@studio", Profile: "codex",
		Agent: "codex", Machine: "studio", State: conversation.ParticipantActive, CreatedAt: now,
	}
	if err := st.CreateConversation(conv, []conversation.Participant{participant}); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := st.CreateConversationTurn(store.CreateConversationTurnParams{
		TurnID: "turn-active", ConversationID: conv.ID, HumanID: "human", Body: "keep working",
		Targets: []store.ConversationTurnTarget{{ID: "target-active", ParticipantID: participant.ID, RunID: "run-active"}}, CreatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	service := control.NewConversationService(st, fake.New(), apiSeats{}, t.TempDir())
	t.Cleanup(service.Close)
	srv := ui.New(ui.Deps{Conversations: service})
	mux := http.NewServeMux()
	srv.Register(mux)

	raw := requestJSON(t, mux, http.MethodDelete, "/api/conversations/"+conv.ID, nil, http.StatusConflict)
	var response map[string]string
	if err := json.Unmarshal(raw, &response); err != nil {
		t.Fatal(err)
	}
	if response["code"] != "conversation_active" || response["error"] == "" {
		t.Fatalf("delete conflict = %+v, want bounded conversation_active", response)
	}
	requestJSON(t, mux, http.MethodGet, "/api/conversations/"+conv.ID, nil, http.StatusOK)
}

func TestMissingProjectRenamePrefersNotFoundOverNameConflict(t *testing.T) {
	st := openStore(t)
	service := control.NewConversationService(st, fake.New(), apiSeats{}, t.TempDir())
	t.Cleanup(service.Close)
	srv := ui.New(ui.Deps{Conversations: service})
	mux := http.NewServeMux()
	srv.Register(mux)

	requestJSON(t, mux, http.MethodPost, "/api/projects", map[string]any{"name": "Occupied"}, http.StatusCreated)
	requestJSON(t, mux, http.MethodPatch, "/api/projects/missing", map[string]any{"name": "occupied"}, http.StatusNotFound)
}

func TestConversationListScopesUseNewestDurableMessageWithoutDispatch(t *testing.T) {
	st := openStore(t)
	rt := fake.New()
	service := control.NewConversationService(st, rt, apiSeats{}, t.TempDir())
	t.Cleanup(service.Close)

	base := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)
	project := conversation.Project{
		ID: "p1", Name: "Fort", CreatedAt: base, UpdatedAt: base,
	}
	if err := st.CreateProject(project); err != nil {
		t.Fatal(err)
	}

	conversations := []conversation.Conversation{
		{ID: "c-inbox-old", Title: "Inbox old", CreatedAt: base, UpdatedAt: base},
		{ID: "c-inbox-new", Title: "Inbox new", CreatedAt: base, UpdatedAt: base},
		{ID: "c-project-old", ProjectID: project.ID, Title: "Project old", CreatedAt: base, UpdatedAt: base},
		{ID: "c-project-new", ProjectID: project.ID, Title: "Project new", CreatedAt: base, UpdatedAt: base},
	}
	for _, item := range conversations {
		if err := st.CreateConversation(item, nil); err != nil {
			t.Fatal(err)
		}
	}

	activity := []struct {
		conversationID string
		at             time.Time
	}{
		{"c-inbox-old", base.Add(time.Minute)},
		{"c-project-old", base.Add(2 * time.Minute)},
		{"c-project-new", base.Add(3 * time.Minute)},
		{"c-inbox-new", base.Add(4 * time.Minute)},
	}
	for _, item := range activity {
		if _, err := st.AppendConversationMessage(conversation.Message{
			ConversationID: item.conversationID,
			AuthorKind:     conversation.AuthorHuman,
			AuthorID:       "toby",
			Body:           "durable activity",
			CreatedAt:      item.at,
		}); err != nil {
			t.Fatal(err)
		}
	}

	srv := ui.New(ui.Deps{Conversations: service})
	mux := http.NewServeMux()
	srv.Register(mux)

	assertIDs := func(path string, want ...string) {
		t.Helper()
		var got []conversation.Conversation
		raw := requestJSON(t, mux, http.MethodGet, path, nil, http.StatusOK)
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatal(err)
		}
		if len(got) != len(want) {
			t.Fatalf("%s returned %d conversations, want %d: %+v", path, len(got), len(want), got)
		}
		for i, id := range want {
			if got[i].ID != id {
				t.Fatalf("%s order[%d] = %q, want %q: %+v", path, i, got[i].ID, id, got)
			}
		}
	}

	assertIDs("/api/conversations",
		"c-inbox-new",
		"c-project-new",
		"c-project-old",
		"c-inbox-old",
	)
	assertIDs("/api/conversations?project_id=inbox",
		"c-inbox-new",
		"c-inbox-old",
	)
	assertIDs("/api/conversations?project_id=p1",
		"c-project-new",
		"c-project-old",
	)

	if got := len(rt.Dispatched()); got != 0 {
		t.Fatalf("conversation list scopes dispatched %d runtime calls, want 0", got)
	}
}

func TestConversationSeatRecheckReturnsOnlyFreshlyPublishedSeats(t *testing.T) {
	st := openStore(t)
	seats := apiSeats{{
		ID: "codex@studio", Profile: "codex", Agent: "codex", Machine: "studio",
		DisplayName: "Codex on Studio", State: "unavailable", Reason: "stale",
	}}
	conversationService := control.NewConversationService(st, fake.New(), seats, t.TempDir())
	t.Cleanup(conversationService.Close)
	rechecker := &apiSeatRechecker{seats: seats}
	srv := ui.New(ui.Deps{
		Dispatcher: control.NewQueueDispatcher(st), Store: st,
		Conversations: conversationService, SeatRechecker: rechecker,
	})
	mux := http.NewServeMux()
	srv.Register(mux)

	var refreshed []conversation.Seat
	if err := json.Unmarshal(requestJSON(t, mux, http.MethodPost, "/api/conversation-seats/recheck", map[string]any{}, http.StatusOK), &refreshed); err != nil {
		t.Fatal(err)
	}
	if rechecker.calls != 1 || len(refreshed) != 1 || refreshed[0].State != "ready" || refreshed[0].Reason != "" {
		t.Fatalf("calls=%d refreshed=%+v, want one explicit refresh before projection", rechecker.calls, refreshed)
	}
}

func TestConversationSeatRecheckFailsClosedWithoutFunctionalProbes(t *testing.T) {
	st := openStore(t)
	conversationService := control.NewConversationService(st, fake.New(), apiSeats{{
		ID: "codex@studio", Profile: "codex", Agent: "codex", Machine: "studio", State: "ready",
	}}, t.TempDir())
	t.Cleanup(conversationService.Close)
	srv := ui.New(ui.Deps{Dispatcher: control.NewQueueDispatcher(st), Store: st, Conversations: conversationService})
	mux := http.NewServeMux()
	srv.Register(mux)

	requestJSON(t, mux, http.MethodPost, "/api/conversation-seats/recheck", map[string]any{}, http.StatusServiceUnavailable)
}

func TestTodayAPIRequiresTheFortConfiguredLocation(t *testing.T) {
	st := openStore(t)
	todayService := control.NewTodayService(st, nil)
	srv := ui.New(ui.Deps{Today: todayService})
	mux := http.NewServeMux()
	srv.Register(mux)

	requestJSON(t, mux, http.MethodGet, "/api/today", nil, http.StatusServiceUnavailable)
}

func TestConversationTurnRecoversOnlyDriftedTargetAfterExplicitRecheck(t *testing.T) {
	st := openStore(t)
	rt := fake.New()
	seats := apiSeats{
		{ID: "codex@studio", Profile: "codex", Agent: "codex", Machine: "studio", DisplayName: "Codex on Studio", State: "ready"},
		{ID: "hermes@mini", Profile: "hermes", Agent: "hermes", Machine: "mini", DisplayName: "Hermes on Mini", State: "ready"},
	}
	conversationService := control.NewConversationService(st, rt, seats, t.TempDir())
	t.Cleanup(conversationService.Close)
	rechecker := &apiSeatRechecker{seats: seats, index: 1}
	srv := ui.New(ui.Deps{
		Dispatcher: control.NewQueueDispatcher(st), Store: st,
		Conversations: conversationService, SeatRechecker: rechecker,
	})
	mux := http.NewServeMux()
	srv.Register(mux)

	createdJSON := requestJSON(t, mux, http.MethodPost, "/api/conversations", map[string]any{
		"title": "Two computers", "seat_ids": []string{"codex@studio", "hermes@mini"},
	}, http.StatusCreated)
	var created struct {
		Conversation conversation.Conversation  `json:"conversation"`
		Participants []conversation.Participant `json:"participants"`
	}
	if err := json.Unmarshal(createdJSON, &created); err != nil {
		t.Fatal(err)
	}
	seats[1].State = "unavailable"
	requestJSON(t, mux, http.MethodPost, "/api/conversations/"+created.Conversation.ID+"/turns", map[string]any{
		"client_turn_id": "client-drift", "text": "answer if you can", "participant_ids": []string{created.Participants[0].ID, created.Participants[1].ID},
	}, http.StatusAccepted)
	conversationService.Wait()

	restoredJSON := requestJSON(t, mux, http.MethodGet, "/api/conversations/"+created.Conversation.ID, nil, http.StatusOK)
	var restored struct {
		Messages []conversation.Message `json:"messages"`
		Targets  []conversation.Target  `json:"targets"`
	}
	if err := json.Unmarshal(restoredJSON, &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Messages) != 2 || len(restored.Targets) != 2 {
		t.Fatalf("restored conversation = %+v", restored)
	}
	states := map[conversation.TargetState]int{}
	failedTargetID := ""
	for _, target := range restored.Targets {
		states[target.State]++
		if target.State == conversation.TargetFailed && target.ErrorCode != "seat_unready" {
			t.Fatalf("failed target = %+v", target)
		}
		if target.State == conversation.TargetFailed {
			failedTargetID = target.ID
		}
	}
	if states[conversation.TargetAnswered] != 1 || states[conversation.TargetFailed] != 1 {
		t.Fatalf("target states = %+v", restored.Targets)
	}
	if got := len(rt.Dispatched()); got != 1 {
		t.Fatalf("runtime dispatches before recovery = %d, want only the ready peer", got)
	}

	requestJSON(t, mux, http.MethodPost, "/api/conversation-seats/recheck", map[string]any{}, http.StatusOK)
	if got := len(rt.Dispatched()); got != 1 {
		t.Fatalf("readiness recheck invoked the runtime: %d dispatches", got)
	}
	requestJSON(t, mux, http.MethodPost, "/api/conversations/"+created.Conversation.ID+"/targets/"+failedTargetID+"/retry", map[string]any{}, http.StatusAccepted)
	conversationService.Wait()

	var recovered struct {
		Messages []conversation.Message `json:"messages"`
		Targets  []conversation.Target  `json:"targets"`
	}
	if err := json.Unmarshal(requestJSON(t, mux, http.MethodGet, "/api/conversations/"+created.Conversation.ID, nil, http.StatusOK), &recovered); err != nil {
		t.Fatal(err)
	}
	states = map[conversation.TargetState]int{}
	for _, target := range recovered.Targets {
		states[target.State]++
	}
	if len(recovered.Messages) != 3 || len(recovered.Targets) != 3 || states[conversation.TargetAnswered] != 2 || states[conversation.TargetFailed] != 1 {
		t.Fatalf("recovered conversation = %+v, want one original failure and two exact answers", recovered)
	}
	dispatchedByMachine := map[string]int{}
	for _, spec := range rt.Dispatched() {
		dispatchedByMachine[spec.Machine]++
	}
	if dispatchedByMachine["studio"] != 1 || dispatchedByMachine["mini"] != 1 || len(rt.Dispatched()) != 2 {
		t.Fatalf("recovery dispatches = %+v, want each exact seat once", rt.Dispatched())
	}
}

func TestNestedTargetActionsRejectAnotherConversationsTarget(t *testing.T) {
	st := openStore(t)
	seats := apiSeats{{
		ID: "codex@studio", Profile: "codex", Agent: "codex", Machine: "studio", DisplayName: "Codex on Studio", State: "ready",
	}}
	conversationService := control.NewConversationService(st, fake.New(), seats, t.TempDir())
	t.Cleanup(conversationService.Close)
	srv := ui.New(ui.Deps{Dispatcher: control.NewQueueDispatcher(st), Store: st, Conversations: conversationService})
	mux := http.NewServeMux()
	srv.Register(mux)

	create := func(title string) struct {
		Conversation conversation.Conversation  `json:"conversation"`
		Participants []conversation.Participant `json:"participants"`
	} {
		t.Helper()
		var detail struct {
			Conversation conversation.Conversation  `json:"conversation"`
			Participants []conversation.Participant `json:"participants"`
		}
		raw := requestJSON(t, mux, http.MethodPost, "/api/conversations", map[string]any{
			"title": title, "seat_ids": []string{"codex@studio"},
		}, http.StatusCreated)
		if err := json.Unmarshal(raw, &detail); err != nil {
			t.Fatal(err)
		}
		return detail
	}
	first := create("First")
	second := create("Second")
	seats[0].State = "unavailable"
	requestJSON(t, mux, http.MethodPost, "/api/conversations/"+first.Conversation.ID+"/turns", map[string]any{
		"client_turn_id": "failed-turn", "text": "hello", "participant_ids": []string{first.Participants[0].ID},
	}, http.StatusAccepted)
	conversationService.Wait()
	var failed struct {
		Targets []conversation.Target `json:"targets"`
	}
	if err := json.Unmarshal(requestJSON(t, mux, http.MethodGet, "/api/conversations/"+first.Conversation.ID, nil, http.StatusOK), &failed); err != nil {
		t.Fatal(err)
	}
	if len(failed.Targets) != 1 || failed.Targets[0].State != conversation.TargetFailed {
		t.Fatalf("failed targets = %+v", failed.Targets)
	}
	wrongBase := "/api/conversations/" + second.Conversation.ID + "/targets/" + failed.Targets[0].ID
	requestJSON(t, mux, http.MethodPost, wrongBase+"/retry", map[string]any{}, http.StatusNotFound)
	requestJSON(t, mux, http.MethodPost, wrongBase+"/cancel", map[string]any{}, http.StatusNotFound)
	var unchanged struct {
		Targets []conversation.Target `json:"targets"`
	}
	if err := json.Unmarshal(requestJSON(t, mux, http.MethodGet, "/api/conversations/"+first.Conversation.ID, nil, http.StatusOK), &unchanged); err != nil {
		t.Fatal(err)
	}
	if len(unchanged.Targets) != 1 {
		t.Fatalf("mismatched route mutated targets: %+v", unchanged.Targets)
	}
}

func TestConversationSelectionErrorsUseBoundedCodes(t *testing.T) {
	st := openStore(t)
	rt := fake.New()
	seats := apiSeats{
		{ID: "codex@studio", Profile: "codex", Agent: "codex", Machine: "studio", DisplayName: "Codex on Studio", State: "ready"},
		{ID: "claude@mini", Profile: "claude", Agent: "claude", Machine: "mini", DisplayName: "Claude on Mini", State: "setup_required", Reason: "auth_required"},
	}
	conversationService := control.NewConversationService(st, rt, seats, t.TempDir())
	t.Cleanup(conversationService.Close)
	srv := ui.New(ui.Deps{Conversations: conversationService})
	mux := http.NewServeMux()
	srv.Register(mux)

	assertCode := func(method, path string, body any, want string) []byte {
		t.Helper()
		raw := requestJSON(t, mux, method, path, body, http.StatusBadRequest)
		var response map[string]string
		if err := json.Unmarshal(raw, &response); err != nil {
			t.Fatalf("decode bounded error %q: %v body=%s", want, err, raw)
		}
		if response["code"] != want || response["error"] == "" {
			t.Fatalf("bounded error = %+v, want code %q", response, want)
		}
		return raw
	}

	assertCode(http.MethodPost, "/api/conversations", map[string]any{
		"title": "Unknown seat", "seat_ids": []string{"missing@nowhere"},
	}, "seat_unknown")
	assertCode(http.MethodPost, "/api/conversations", map[string]any{
		"title": "Unready seat", "seat_ids": []string{"claude@mini"},
	}, "seat_unready")

	var created struct {
		Conversation conversation.Conversation  `json:"conversation"`
		Participants []conversation.Participant `json:"participants"`
	}
	if err := json.Unmarshal(requestJSON(t, mux, http.MethodPost, "/api/conversations", map[string]any{
		"title": "Participant errors", "seat_ids": []string{"codex@studio"},
	}, http.StatusCreated), &created); err != nil {
		t.Fatal(err)
	}
	turnPath := "/api/conversations/" + created.Conversation.ID + "/turns"
	assertCode(http.MethodPost, turnPath, map[string]any{
		"client_turn_id": "unknown-participant", "text": "hello", "participant_ids": []string{"missing-participant"},
	}, "participant_unknown")
	requestJSON(t, mux, http.MethodDelete, "/api/conversations/"+created.Conversation.ID+"/participants/"+created.Participants[0].ID, nil, http.StatusNoContent)
	assertCode(http.MethodPost, turnPath, map[string]any{
		"client_turn_id": "removed-participant", "text": "hello", "participant_ids": []string{created.Participants[0].ID},
	}, "participant_removed")
	if got := len(rt.Dispatched()); got != 0 {
		t.Fatalf("rejected selections dispatched %d runtimes, want 0", got)
	}
}

func TestConversationRetryReadinessErrorsUseBoundedCodes(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*conversation.Seat)
	}{
		{name: "identity mismatch", mutate: func(seat *conversation.Seat) { seat.Model = "different-model" }},
		{name: "offline", mutate: func(seat *conversation.Seat) {
			seat.State, seat.Reason = "unavailable", "unreachable"
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			st := openStore(t)
			rt := fake.New()
			rt.ExitCode = 2
			seats := apiSeats{{
				ID: "codex@mini", Profile: "codex", Agent: "codex", Model: "gpt", Machine: "mini", DisplayName: "Codex on Mini", State: "ready",
			}}
			conversationService := control.NewConversationService(st, rt, seats, t.TempDir())
			t.Cleanup(conversationService.Close)
			srv := ui.New(ui.Deps{Conversations: conversationService})
			mux := http.NewServeMux()
			srv.Register(mux)

			var created struct {
				Conversation conversation.Conversation  `json:"conversation"`
				Participants []conversation.Participant `json:"participants"`
			}
			if err := json.Unmarshal(requestJSON(t, mux, http.MethodPost, "/api/conversations", map[string]any{
				"title": "Retry readiness", "seat_ids": []string{"codex@mini"},
			}, http.StatusCreated), &created); err != nil {
				t.Fatal(err)
			}
			requestJSON(t, mux, http.MethodPost, "/api/conversations/"+created.Conversation.ID+"/turns", map[string]any{
				"client_turn_id": "failed-turn", "text": "hello", "participant_ids": []string{created.Participants[0].ID},
			}, http.StatusAccepted)
			conversationService.Wait()
			var detail struct {
				Targets []conversation.Target `json:"targets"`
			}
			if err := json.Unmarshal(requestJSON(t, mux, http.MethodGet, "/api/conversations/"+created.Conversation.ID, nil, http.StatusOK), &detail); err != nil {
				t.Fatal(err)
			}
			if len(detail.Targets) != 1 || detail.Targets[0].State != conversation.TargetFailed {
				t.Fatalf("initial failed target = %+v", detail.Targets)
			}
			test.mutate(&seats[0])
			raw := requestJSON(t, mux, http.MethodPost, "/api/conversations/"+created.Conversation.ID+"/targets/"+detail.Targets[0].ID+"/retry", map[string]any{}, http.StatusBadRequest)
			var response map[string]string
			if err := json.Unmarshal(raw, &response); err != nil {
				t.Fatalf("decode retry error: %v body=%s", err, raw)
			}
			if response["code"] != "seat_unready" || response["error"] == "" {
				t.Fatalf("retry error = %+v, want seat_unready", response)
			}
			if got := len(rt.Dispatched()); got != 1 {
				t.Fatalf("rejected retry dispatched %d runtimes, want original only", got)
			}
			var unchanged struct {
				Targets []conversation.Target `json:"targets"`
			}
			if err := json.Unmarshal(requestJSON(t, mux, http.MethodGet, "/api/conversations/"+created.Conversation.ID, nil, http.StatusOK), &unchanged); err != nil {
				t.Fatal(err)
			}
			if len(unchanged.Targets) != 1 {
				t.Fatalf("rejected retry created an attempt: %+v", unchanged.Targets)
			}
		})
	}
}

func TestScheduleAPIIsLoopbackOnlyAndHotRegisters(t *testing.T) {
	st := openStore(t)
	persistedNext := time.Date(2031, 4, 5, 6, 7, 8, 0, time.UTC)
	schedules := &apiSchedules{persistedNext: persistedNext}
	srv := ui.New(ui.Deps{Dispatcher: control.NewQueueDispatcher(st), Store: st, Schedules: schedules})
	mux := http.NewServeMux()
	srv.Register(mux)
	body := map[string]any{"kind": "once", "expression": time.Now().Add(time.Hour).Format(time.RFC3339), "flow_id": "brief", "timezone": "UTC"}

	data, _ := json.Marshal(body)
	remote := httptest.NewRequest(http.MethodPost, "/api/schedules", bytes.NewReader(data))
	remote.RemoteAddr = "203.0.113.8:4000"
	remoteResult := httptest.NewRecorder()
	mux.ServeHTTP(remoteResult, remote)
	if remoteResult.Code != http.StatusForbidden {
		t.Fatalf("remote status = %d, want 403", remoteResult.Code)
	}

	local := httptest.NewRequest(http.MethodPost, "/api/schedules", bytes.NewReader(data))
	local.RemoteAddr = "127.0.0.1:4000"
	localResult := httptest.NewRecorder()
	mux.ServeHTTP(localResult, local)
	if localResult.Code != http.StatusCreated || len(schedules.created) != 1 || !schedules.created[0].Enabled {
		t.Fatalf("local status=%d schedules=%+v body=%s", localResult.Code, schedules.created, localResult.Body.String())
	}
	var response scheduler.Definition
	if err := json.Unmarshal(localResult.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.NextFireAt.Equal(persistedNext) {
		t.Fatalf("schedule response next_fire_at = %s, want persisted %s", response.NextFireAt, persistedNext)
	}
}

func requestJSON(t *testing.T, handler http.Handler, method, path string, body any, wantStatus int) []byte {
	t.Helper()
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(data)
	}
	request := httptest.NewRequest(method, path, reader)
	request.RemoteAddr = "127.0.0.1:4000"
	result := httptest.NewRecorder()
	handler.ServeHTTP(result, request)
	if result.Code != wantStatus {
		t.Fatalf("%s %s status=%d want=%d body=%s", method, path, result.Code, wantStatus, result.Body.String())
	}
	return result.Body.Bytes()
}
