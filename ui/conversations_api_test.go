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
	"github.com/tobsai/fort/exec/fake"
	"github.com/tobsai/fort/ui"
)

type apiSeats []conversation.Seat

func (s apiSeats) ConversationSeats() []conversation.Seat {
	return append([]conversation.Seat(nil), s...)
}

type apiSchedules struct{ created []scheduler.Definition }

func (s *apiSchedules) Create(_ context.Context, definition scheduler.Definition) error {
	s.created = append(s.created, definition)
	return nil
}

func TestConversationAndTodayAPI(t *testing.T) {
	st := openStore(t)
	rt := fake.New()
	conversationService := control.NewConversationService(st, rt, apiSeats{{
		ID: "codex@studio", Profile: "codex", Agent: "codex", Machine: "studio", DisplayName: "Codex on Studio", State: "ready",
	}}, t.TempDir())
	t.Cleanup(conversationService.Close)
	todayService := control.NewTodayService(st, nil, conversationService)
	srv := ui.New(ui.Deps{
		Dispatcher: control.NewQueueDispatcher(st), Store: st,
		Conversations: conversationService, Today: todayService,
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
	today := requestJSON(t, mux, http.MethodGet, "/api/today?timezone=America%2FChicago", nil, http.StatusOK)
	if !bytes.Contains(today, []byte(`"timezone":"America/Chicago"`)) || !bytes.Contains(today, []byte(`"scheduled":[]`)) {
		t.Fatalf("today = %s", today)
	}
}

func TestScheduleAPIIsLoopbackOnlyAndHotRegisters(t *testing.T) {
	st := openStore(t)
	schedules := &apiSchedules{}
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
