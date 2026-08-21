package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/tobsai/fort/core/conversation"
)

type agentChannelCreateRequest struct {
	AgentOptionID string `json:"agent_option_id"`
	Name          string `json:"name"`
}

type agentChannelPatchRequest struct {
	Name  *string                         `json:"name"`
	State *conversation.AgentChannelState `json:"state"`
}

type agentConversationRequest struct {
	Name string `json:"name"`
}

type agentConversationPatchRequest struct {
	Name   *string                         `json:"name"`
	State  *conversation.ConversationState `json:"state"`
	Pinned *bool                           `json:"pinned"`
}

type agentFirstTurnRequest struct {
	Name         string `json:"name"`
	ClientTurnID string `json:"client_turn_id"`
	Text         string `json:"text"`
}

// RegisterAgentChannelRoutes mounts only the additive agent-first contract.
// The legacy /api/channels and /api/needs-you handlers remain separate.
func (s *Server) RegisterAgentChannelRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/agent-options", phase1LocalOnly(s.handleAgentOptions))
	mux.HandleFunc("POST /api/agent-options/recheck", primaryMutation(s.handleAgentOptionsRecheck, false))
	mux.HandleFunc("GET /api/agent-needs-you", phase1LocalOnly(s.handleAgentNeedsYou))
	mux.HandleFunc("GET /api/agent-channels", phase1LocalOnly(s.handleAgentChannelsList))
	mux.HandleFunc("POST /api/agent-channels", primaryMutation(s.handleAgentChannelCreate, true))
	mux.HandleFunc("GET /api/agent-channels/{channel_id}", phase1LocalOnly(s.handleAgentChannelGet))
	mux.HandleFunc("PATCH /api/agent-channels/{channel_id}", primaryMutation(s.handleAgentChannelPatch, true))
	mux.HandleFunc("POST /api/agent-channels/{channel_id}/turns", primaryMutation(s.handleAgentFirstTurn, true))
	mux.HandleFunc("GET /api/agent-channels/{channel_id}/conversations", phase1LocalOnly(s.handleAgentConversationsList))
	mux.HandleFunc("POST /api/agent-channels/{channel_id}/conversations", primaryMutation(s.handleAgentConversationCreate, true))
	mux.HandleFunc("GET /api/agent-channels/{channel_id}/conversations/{conversation_id}", phase1LocalOnly(s.handleAgentConversationGet))
	mux.HandleFunc("PATCH /api/agent-channels/{channel_id}/conversations/{conversation_id}", primaryMutation(s.handleAgentConversationPatch, true))
	mux.HandleFunc("POST /api/agent-channels/{channel_id}/conversations/{conversation_id}/turns", primaryMutation(s.handleAgentConversationTurn, true))
	mux.HandleFunc("POST /api/agent-channels/{channel_id}/conversations/{conversation_id}/targets/{target_id}/retry", primaryMutation(s.handleAgentTargetRetry, false))
	mux.HandleFunc("POST /api/agent-channels/{channel_id}/conversations/{conversation_id}/targets/{target_id}/cancel", primaryMutation(s.handleAgentTargetCancel, false))
	mux.HandleFunc("GET /api/agent-channels/{channel_id}/conversations/{conversation_id}/events", phase1LocalOnly(s.handleAgentConversationEvents))
}

func (s *Server) requireAgentChannels(w http.ResponseWriter) bool {
	if s.d.AgentChannels == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Agent Channels are unavailable"})
		return false
	}
	return true
}

func (s *Server) handleAgentOptions(w http.ResponseWriter, r *http.Request) {
	if !primaryQuery(w, r) || !s.requireAgentChannels(w) {
		return
	}
	items, err := s.d.AgentChannels.AgentOptions(r.Context())
	if err != nil {
		primaryAPIError(w, err)
		return
	}
	if items == nil {
		items = []AgentOption{}
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleAgentOptionsRecheck(w http.ResponseWriter, r *http.Request) {
	if !primaryQuery(w, r) || !requirePrimaryEmptyBody(w, r) || !s.requireAgentChannels(w) {
		return
	}
	items, err := s.d.AgentChannels.RecheckAgentOptions(r.Context())
	if err != nil {
		primaryAPIError(w, err)
		return
	}
	if items == nil {
		items = []AgentOption{}
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleAgentNeedsYou(w http.ResponseWriter, r *http.Request) {
	if !primaryQuery(w, r) || !s.requireAgentChannels(w) {
		return
	}
	items, err := s.d.AgentChannels.AgentNeedsYou(r.Context())
	if err != nil {
		primaryAPIError(w, err)
		return
	}
	if items == nil {
		items = []AgentNeedsYouItem{}
	}
	for index := range items {
		if items[index].Actions == nil {
			items[index].Actions = []string{}
		}
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleAgentChannelsList(w http.ResponseWriter, r *http.Request) {
	query, ok := primaryQueryValues(w, r, "state")
	if !ok || !s.requireAgentChannels(w) {
		return
	}
	state := string(conversation.AgentChannelOpen)
	if values, present := query["state"]; present {
		state = values[0]
	}
	if state != string(conversation.AgentChannelOpen) && state != string(conversation.AgentChannelArchived) && state != "all" {
		primaryBadRequest(w, "state must be open, archived, or all")
		return
	}
	items, err := s.d.AgentChannels.ListAgentChannels(r.Context(), state)
	if err != nil {
		primaryAPIError(w, err)
		return
	}
	if items == nil {
		items = []AgentChannelSummary{}
	}
	for index := range items {
		normalizeAgentChannel(&items[index])
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleAgentChannelCreate(w http.ResponseWriter, r *http.Request) {
	if !primaryQuery(w, r) || !s.requireAgentChannels(w) {
		return
	}
	var request agentChannelCreateRequest
	if !decodePrimaryJSON(w, r, &request) {
		return
	}
	request.AgentOptionID = strings.TrimSpace(request.AgentOptionID)
	if request.AgentOptionID == "" || len(request.AgentOptionID) > 512 {
		primaryBadRequest(w, "agent_option_id must be non-empty and at most 512 bytes")
		return
	}
	name, ok := validatePrimaryChannelName(w, request.Name)
	if !ok {
		return
	}
	detail, err := s.d.AgentChannels.CreateAgentChannel(r.Context(), request.AgentOptionID, name)
	if err != nil {
		primaryAPIError(w, err)
		return
	}
	normalizeAgentChannel(&detail)
	writeJSON(w, http.StatusCreated, detail)
}

func (s *Server) handleAgentChannelGet(w http.ResponseWriter, r *http.Request) {
	if !s.validAgentChannelRequest(w, r, false) {
		return
	}
	detail, err := s.d.AgentChannels.GetAgentChannel(r.Context(), r.PathValue("channel_id"))
	if err != nil {
		primaryAPIError(w, err)
		return
	}
	normalizeAgentChannel(&detail)
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleAgentChannelPatch(w http.ResponseWriter, r *http.Request) {
	if !s.validAgentChannelRequest(w, r, false) {
		return
	}
	var request agentChannelPatchRequest
	if !decodePrimaryJSON(w, r, &request) {
		return
	}
	changes := 0
	if request.Name != nil {
		changes++
	}
	if request.State != nil {
		changes++
	}
	if changes != 1 {
		primaryBadRequest(w, "exactly one of name or state is required")
		return
	}
	var err error
	if request.Name != nil {
		name, ok := validatePrimaryChannelName(w, *request.Name)
		if !ok {
			return
		}
		err = s.d.AgentChannels.RenameAgentChannel(r.Context(), r.PathValue("channel_id"), name)
	} else {
		if *request.State != conversation.AgentChannelOpen && *request.State != conversation.AgentChannelArchived {
			primaryBadRequest(w, "state must be open or archived")
			return
		}
		err = s.d.AgentChannels.SetAgentChannelState(r.Context(), r.PathValue("channel_id"), *request.State)
	}
	if err != nil {
		primaryAPIError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAgentFirstTurn(w http.ResponseWriter, r *http.Request) {
	if !s.validAgentChannelRequest(w, r, false) {
		return
	}
	var request agentFirstTurnRequest
	if !decodePrimaryJSON(w, r, &request) {
		return
	}
	name, ok := validatePrimaryChannelName(w, request.Name)
	if !ok || !validateAgentTurn(w, &request.ClientTurnID, &request.Text) {
		return
	}
	result, err := s.d.AgentChannels.PostFirstAgentTurn(r.Context(), r.PathValue("channel_id"), name, request.ClientTurnID, request.Text)
	if err != nil {
		primaryAPIError(w, err)
		return
	}
	normalizeAgentConversation(&result.Conversation)
	if result.Targets == nil {
		result.Targets = []conversation.Target{}
	}
	writeJSON(w, http.StatusAccepted, result)
}

func (s *Server) handleAgentConversationsList(w http.ResponseWriter, r *http.Request) {
	query, ok := primaryQueryValues(w, r, "state")
	if !ok || !s.validAgentChannelRequestWithoutQuery(w, r) {
		return
	}
	state := string(conversation.ConversationOpen)
	if values, present := query["state"]; present {
		state = values[0]
	}
	if state != string(conversation.ConversationOpen) && state != string(conversation.ConversationArchived) && state != "all" {
		primaryBadRequest(w, "state must be open, archived, or all")
		return
	}
	items, err := s.d.AgentChannels.ListAgentConversations(r.Context(), r.PathValue("channel_id"), state)
	if err != nil {
		primaryAPIError(w, err)
		return
	}
	if items == nil {
		items = []conversation.AgentConversationSummary{}
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleAgentConversationCreate(w http.ResponseWriter, r *http.Request) {
	if !s.validAgentChannelRequest(w, r, false) {
		return
	}
	var request agentConversationRequest
	if !decodePrimaryJSON(w, r, &request) {
		return
	}
	name, ok := validatePrimaryChannelName(w, request.Name)
	if !ok {
		return
	}
	detail, err := s.d.AgentChannels.CreateAgentConversation(r.Context(), r.PathValue("channel_id"), name)
	if err != nil {
		primaryAPIError(w, err)
		return
	}
	normalizeAgentConversation(&detail)
	writeJSON(w, http.StatusCreated, detail)
}

func (s *Server) handleAgentConversationGet(w http.ResponseWriter, r *http.Request) {
	if !s.validAgentConversationRequest(w, r, false) {
		return
	}
	detail, err := s.d.AgentChannels.GetAgentConversation(r.Context(), r.PathValue("channel_id"), r.PathValue("conversation_id"))
	if err != nil {
		primaryAPIError(w, err)
		return
	}
	normalizeAgentConversation(&detail)
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleAgentConversationPatch(w http.ResponseWriter, r *http.Request) {
	if !s.validAgentConversationRequest(w, r, false) {
		return
	}
	var request agentConversationPatchRequest
	if !decodePrimaryJSON(w, r, &request) {
		return
	}
	changes := 0
	if request.Name != nil {
		changes++
	}
	if request.State != nil {
		changes++
	}
	if request.Pinned != nil {
		changes++
	}
	if changes != 1 {
		primaryBadRequest(w, "exactly one of name, state, or pinned is required")
		return
	}
	channelID, conversationID := r.PathValue("channel_id"), r.PathValue("conversation_id")
	var err error
	switch {
	case request.Name != nil:
		name, ok := validatePrimaryChannelName(w, *request.Name)
		if !ok {
			return
		}
		err = s.d.AgentChannels.RenameAgentConversation(r.Context(), channelID, conversationID, name)
	case request.State != nil:
		if *request.State != conversation.ConversationOpen && *request.State != conversation.ConversationArchived {
			primaryBadRequest(w, "state must be open or archived")
			return
		}
		err = s.d.AgentChannels.SetAgentConversationState(r.Context(), channelID, conversationID, *request.State)
	case request.Pinned != nil:
		err = s.d.AgentChannels.SetAgentConversationPinned(r.Context(), channelID, conversationID, *request.Pinned)
	}
	if err != nil {
		primaryAPIError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAgentConversationTurn(w http.ResponseWriter, r *http.Request) {
	if !s.validAgentConversationRequest(w, r, false) {
		return
	}
	var request primaryTurnRequest
	if !decodePrimaryJSON(w, r, &request) || !validateAgentTurn(w, &request.ClientTurnID, &request.Text) {
		return
	}
	result, err := s.d.AgentChannels.PostAgentTurn(r.Context(), r.PathValue("channel_id"), r.PathValue("conversation_id"), request.ClientTurnID, request.Text)
	if err != nil {
		primaryAPIError(w, err)
		return
	}
	if result.Targets == nil {
		result.Targets = []conversation.Target{}
	}
	writeJSON(w, http.StatusAccepted, result)
}

func (s *Server) handleAgentTargetRetry(w http.ResponseWriter, r *http.Request) {
	if !requirePrimaryEmptyBody(w, r) || !s.validAgentTargetRequest(w, r) {
		return
	}
	detail, selected, ok := s.agentNestedTarget(w, r)
	if !ok {
		return
	}
	if selected.State != conversation.TargetFailed || !primaryLatestAttempt(detail.Targets, selected) {
		primaryConflict(w, "target is not the latest failed attempt", "target_state_conflict")
		return
	}
	target, err := s.d.AgentChannels.RetryAgentTarget(r.Context(), r.PathValue("channel_id"), r.PathValue("conversation_id"), r.PathValue("target_id"))
	if err != nil {
		primaryAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, target)
}

func (s *Server) handleAgentTargetCancel(w http.ResponseWriter, r *http.Request) {
	if !requirePrimaryEmptyBody(w, r) || !s.validAgentTargetRequest(w, r) {
		return
	}
	_, selected, ok := s.agentNestedTarget(w, r)
	if !ok {
		return
	}
	if selected.State != conversation.TargetQueued && selected.State != conversation.TargetWorking {
		primaryConflict(w, "target is no longer active", "target_state_conflict")
		return
	}
	if err := s.d.AgentChannels.CancelAgentTarget(r.Context(), r.PathValue("channel_id"), r.PathValue("conversation_id"), r.PathValue("target_id")); err != nil {
		primaryAPIError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAgentConversationEvents(w http.ResponseWriter, r *http.Request) {
	if !s.validAgentConversationRequest(w, r, false) {
		return
	}
	detail, err := s.d.AgentChannels.GetAgentConversation(r.Context(), r.PathValue("channel_id"), r.PathValue("conversation_id"))
	if err != nil {
		primaryAPIError(w, err)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Conversation events are unavailable"})
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	last := ""
	for {
		normalizeAgentConversation(&detail)
		data, marshalErr := json.Marshal(detail)
		if marshalErr != nil {
			return
		}
		if string(data) != last {
			if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
				return
			}
			flusher.Flush()
			last = string(data)
		}
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
		}
		detail, err = s.d.AgentChannels.GetAgentConversation(r.Context(), r.PathValue("channel_id"), r.PathValue("conversation_id"))
		if err != nil {
			return
		}
	}
}

func (s *Server) validAgentChannelRequest(w http.ResponseWriter, r *http.Request, allowQuery bool) bool {
	if (!allowQuery && !primaryQuery(w, r)) || !s.validAgentChannelRequestWithoutQuery(w, r) {
		return false
	}
	return true
}

func (s *Server) validAgentChannelRequestWithoutQuery(w http.ResponseWriter, r *http.Request) bool {
	return s.requireAgentChannels(w) && validPrimaryPathID(w, r.PathValue("channel_id"), "Agent Channel id")
}

func (s *Server) validAgentConversationRequest(w http.ResponseWriter, r *http.Request, allowQuery bool) bool {
	return s.validAgentChannelRequest(w, r, allowQuery) && validPrimaryPathID(w, r.PathValue("conversation_id"), "Conversation id")
}

func (s *Server) validAgentTargetRequest(w http.ResponseWriter, r *http.Request) bool {
	return s.validAgentConversationRequest(w, r, false) && validPrimaryPathID(w, r.PathValue("target_id"), "target id")
}

func (s *Server) agentNestedTarget(w http.ResponseWriter, r *http.Request) (AgentConversationDetail, conversation.Target, bool) {
	detail, err := s.d.AgentChannels.GetAgentConversation(r.Context(), r.PathValue("channel_id"), r.PathValue("conversation_id"))
	if err != nil {
		primaryAPIError(w, err)
		return AgentConversationDetail{}, conversation.Target{}, false
	}
	for _, target := range detail.Targets {
		if target.ID == r.PathValue("target_id") {
			return detail, target, true
		}
	}
	primaryNotFound(w)
	return AgentConversationDetail{}, conversation.Target{}, false
}

func validateAgentTurn(w http.ResponseWriter, clientTurnID, text *string) bool {
	parsed, err := uuid.Parse(*clientTurnID)
	if err != nil || parsed.String() != *clientTurnID {
		primaryBadRequest(w, "client_turn_id must be a canonical UUID")
		return false
	}
	*text = strings.TrimSpace(*text)
	if *text == "" || len([]byte(*text)) > conversation.MaxContextBytes || !utf8.ValidString(*text) {
		primaryBadRequest(w, "text must contain 1 to 65536 UTF-8 bytes")
		return false
	}
	return true
}

func normalizeAgentChannel(detail *AgentChannelSummary) {
	if detail.Conversations == nil {
		detail.Conversations = []conversation.AgentConversationSummary{}
	}
}

func normalizeAgentConversation(detail *AgentConversationDetail) {
	if detail.Messages == nil {
		detail.Messages = []conversation.Message{}
	}
	if detail.Turns == nil {
		detail.Turns = []conversation.Turn{}
	}
	if detail.Targets == nil {
		detail.Targets = []conversation.Target{}
	}
}
