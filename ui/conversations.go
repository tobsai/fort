package ui

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/scheduler"
)

type projectRequest struct {
	Name string `json:"name"`
}

type conversationRequest struct {
	ProjectID string   `json:"project_id"`
	Title     string   `json:"title"`
	SeatIDs   []string `json:"seat_ids"`
}

type conversationPatch struct {
	ProjectID *string                         `json:"project_id"`
	Title     *string                         `json:"title"`
	State     *conversation.ConversationState `json:"state"`
}

type participantRequest struct {
	SeatID string `json:"seat_id"`
}

type conversationTurnRequest struct {
	ClientTurnID         string   `json:"client_turn_id"`
	Text                 string   `json:"text"`
	ParticipantIDs       []string `json:"participant_ids"`
	TargetParticipantIDs []string `json:"target_participant_ids,omitempty"`
}

type scheduleRequest struct {
	ID         string         `json:"id"`
	Title      string         `json:"title"`
	Kind       scheduler.Kind `json:"kind"`
	Expression string         `json:"expression"`
	FlowID     string         `json:"flow_id"`
	Timezone   string         `json:"timezone"`
	Enabled    *bool          `json:"enabled,omitempty"`
}

func (s *Server) requireConversations(w http.ResponseWriter) bool {
	if s.d.Conversations == nil {
		http.Error(w, "shared conversations are unavailable", http.StatusServiceUnavailable)
		return false
	}
	return true
}

func (s *Server) handleConversationSeats(w http.ResponseWriter, r *http.Request) {
	if !s.requireConversations(w) {
		return
	}
	items, err := s.d.Conversations.ConversationSeats(r.Context())
	if err != nil {
		conversationAPIError(w, err)
		return
	}
	if items == nil {
		items = []conversation.Seat{}
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleProjectsList(w http.ResponseWriter, r *http.Request) {
	if !s.requireConversations(w) {
		return
	}
	items, err := s.d.Conversations.ListProjects(r.Context())
	if err != nil {
		conversationAPIError(w, err)
		return
	}
	if items == nil {
		items = []conversation.Project{}
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleProjectCreate(w http.ResponseWriter, r *http.Request) {
	if !s.requireConversations(w) {
		return
	}
	var request projectRequest
	if err := decodeJSON(r, &request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	item, err := s.d.Conversations.CreateProject(r.Context(), request.Name)
	if err != nil {
		conversationAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) handleProjectPatch(w http.ResponseWriter, r *http.Request) {
	if !s.requireConversations(w) {
		return
	}
	var request projectRequest
	if err := decodeJSON(r, &request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.d.Conversations.RenameProject(r.Context(), r.PathValue("id"), request.Name); err != nil {
		conversationAPIError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleProjectDelete(w http.ResponseWriter, r *http.Request) {
	if !s.requireConversations(w) {
		return
	}
	if err := s.d.Conversations.DeleteProject(r.Context(), r.PathValue("id")); err != nil {
		conversationAPIError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleConversationsList(w http.ResponseWriter, r *http.Request) {
	if !s.requireConversations(w) {
		return
	}
	scope := r.URL.Query().Get("project_id")
	items, err := s.d.Conversations.ListConversations(r.Context(), scope)
	if err != nil {
		conversationAPIError(w, err)
		return
	}
	if items == nil {
		items = []conversation.Conversation{}
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleConversationCreate(w http.ResponseWriter, r *http.Request) {
	if !s.requireConversations(w) {
		return
	}
	var request conversationRequest
	if err := decodeJSON(r, &request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	item, err := s.d.Conversations.CreateConversation(r.Context(), request.ProjectID, request.Title, request.SeatIDs)
	if err != nil {
		conversationAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) handleConversationGet(w http.ResponseWriter, r *http.Request) {
	if !s.requireConversations(w) {
		return
	}
	item, err := s.d.Conversations.GetConversation(r.Context(), r.PathValue("id"))
	if err != nil {
		conversationAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleConversationPatch(w http.ResponseWriter, r *http.Request) {
	if !s.requireConversations(w) {
		return
	}
	var request conversationPatch
	if err := decodeJSON(r, &request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if request.Title != nil {
		if err := s.d.Conversations.RenameConversation(r.Context(), r.PathValue("id"), *request.Title); err != nil {
			conversationAPIError(w, err)
			return
		}
	}
	if request.ProjectID != nil {
		if err := s.d.Conversations.MoveConversation(r.Context(), r.PathValue("id"), *request.ProjectID); err != nil {
			conversationAPIError(w, err)
			return
		}
	}
	if request.State != nil {
		if err := s.d.Conversations.SetConversationState(r.Context(), r.PathValue("id"), *request.State); err != nil {
			conversationAPIError(w, err)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleConversationDelete(w http.ResponseWriter, r *http.Request) {
	if !s.requireConversations(w) {
		return
	}
	if err := s.d.Conversations.DeleteConversation(r.Context(), r.PathValue("id")); err != nil {
		conversationAPIError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleConversationParticipantAdd(w http.ResponseWriter, r *http.Request) {
	if !s.requireConversations(w) {
		return
	}
	var request participantRequest
	if err := decodeJSON(r, &request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	participant, err := s.d.Conversations.AddConversationParticipant(r.Context(), r.PathValue("id"), request.SeatID)
	if err != nil {
		conversationAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, participant)
}

func (s *Server) handleConversationParticipantDelete(w http.ResponseWriter, r *http.Request) {
	if !s.requireConversations(w) {
		return
	}
	if err := s.d.Conversations.RemoveConversationParticipant(r.Context(), r.PathValue("id"), r.PathValue("participant_id")); err != nil {
		conversationAPIError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleConversationTurn(w http.ResponseWriter, r *http.Request) {
	if !s.requireConversations(w) {
		return
	}
	var request conversationTurnRequest
	if err := decodeJSON(r, &request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	participantIDs := request.ParticipantIDs
	if len(participantIDs) == 0 {
		participantIDs = request.TargetParticipantIDs
	}
	result, err := s.d.Conversations.PostTurn(r.Context(), r.PathValue("id"), request.ClientTurnID, request.Text, participantIDs)
	if err != nil {
		conversationAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, result)
}

func (s *Server) handleConversationTargetRetry(w http.ResponseWriter, r *http.Request) {
	if !s.requireConversations(w) {
		return
	}
	targetID := r.PathValue("target_id")
	if targetID == "" {
		targetID = r.PathValue("id")
	}
	item, err := s.d.Conversations.RetryTarget(r.Context(), targetID)
	if err != nil {
		conversationAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, item)
}

func (s *Server) handleConversationTargetCancel(w http.ResponseWriter, r *http.Request) {
	if !s.requireConversations(w) {
		return
	}
	targetID := r.PathValue("target_id")
	if targetID == "" {
		targetID = r.PathValue("id")
	}
	if err := s.d.Conversations.CancelTarget(r.Context(), targetID); err != nil {
		conversationAPIError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleConversationEvents(w http.ResponseWriter, r *http.Request) {
	if !s.requireConversations(w) {
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	last := ""
	for {
		detail, err := s.d.Conversations.GetConversation(r.Context(), r.PathValue("id"))
		if err != nil {
			return
		}
		data, _ := json.Marshal(detail)
		if string(data) != last {
			fmt.Fprintf(w, "event: conversation\ndata: %s\n\n", data)
			flusher.Flush()
			last = string(data)
		}
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Server) handleToday(w http.ResponseWriter, r *http.Request) {
	if s.d.Today == nil {
		http.Error(w, "today is unavailable", http.StatusServiceUnavailable)
		return
	}
	location := time.Local
	if name := strings.TrimSpace(r.URL.Query().Get("timezone")); name != "" {
		parsed, err := time.LoadLocation(name)
		if err != nil {
			http.Error(w, "unknown timezone", http.StatusBadRequest)
			return
		}
		location = parsed
	}
	view, err := s.d.Today.Today(r.Context(), time.Now(), location)
	if err != nil {
		conversationAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) handleScheduleCreate(w http.ResponseWriter, r *http.Request) {
	if !loopbackRequest(r) {
		http.Error(w, "schedule administration is loopback-only", http.StatusForbidden)
		return
	}
	if s.d.Schedules == nil {
		http.Error(w, "durable scheduler is unavailable", http.StatusServiceUnavailable)
		return
	}
	var request scheduleRequest
	if err := decodeJSON(r, &request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if request.ID == "" {
		request.ID = uuid.NewString()
	}
	enabled := true
	if request.Enabled != nil {
		enabled = *request.Enabled
	}
	title := strings.TrimSpace(request.Title)
	if title == "" {
		title = request.FlowID
	}
	definition := scheduler.Definition{ID: request.ID, Title: title, Kind: request.Kind, Expression: request.Expression, FlowID: request.FlowID, Timezone: request.Timezone, Enabled: enabled}
	nextFireAt, err := scheduler.NextFire(definition, time.Now())
	if err != nil {
		conversationAPIError(w, err)
		return
	}
	definition.NextFireAt = nextFireAt
	if err := s.d.Schedules.Create(r.Context(), definition); err != nil {
		conversationAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, definition)
}

func decodeJSON(r *http.Request, target any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("bad request: %w", err)
	}
	return nil
}

func conversationAPIError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, sql.ErrNoRows):
		http.Error(w, "not found", http.StatusNotFound)
	case errors.Is(err, conversation.ErrContextTooLarge):
		writeJSON(w, http.StatusConflict, map[string]string{"code": "conversation_context_limit", "error": err.Error()})
	case strings.Contains(strings.ToLower(err.Error()), "unique constraint"):
		http.Error(w, err.Error(), http.StatusConflict)
	default:
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
}

func loopbackRequest(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}
