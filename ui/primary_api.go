package ui

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/tobsai/fort/core/conversation"
	"github.com/tobsai/fort/core/transporttrust"
)

const primaryRequestLimit = conversation.MaxContextBytes + 4_096

var primaryConflictCodes = map[string]bool{
	"primary_agent_not_configured": true,
	"primary_agent_unready":        true,
	"primary_agent_drift":          true,
	"chat_authority_violation":     true,
	"primary_channel_invariant":    true,
	"provider_result_unknown":      true,
	"provider_incomplete":          true,
	"provider_refusal":             true,
	"provider_failed":              true,
	"seat_unready":                 true,
	"agent_channel_state":          true,
	"agent_recovery_unavailable":   true,
}

type primaryChannelCodedError interface {
	PrimaryChannelCode() string
}

type primaryAgentRequest struct {
	OptionID string `json:"option_id"`
}

type primaryChannelRequest struct {
	Name string `json:"name"`
}

type primaryChannelPatch struct {
	Name   *string                         `json:"name"`
	State  *conversation.ConversationState `json:"state"`
	Pinned *bool                           `json:"pinned"`
}

type primaryTurnRequest struct {
	ClientTurnID string `json:"client_turn_id"`
	Text         string `json:"text"`
}

// RegisterPrimaryRoutes mounts only the Phase 1 private Channel surface.
// Composition calls it in preview/primary mode; Register deliberately does
// not expose these routes in off mode.
func (s *Server) RegisterPrimaryRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/settings/primary-agent", phase1LocalOnly(s.handlePrimaryAgentGet))
	mux.HandleFunc("PUT /api/settings/primary-agent", primaryMutation(s.handlePrimaryAgentPut, true))
	mux.HandleFunc("DELETE /api/settings/primary-agent", primaryMutation(s.handlePrimaryAgentDelete, false))
	mux.HandleFunc("POST /api/settings/primary-agent/recheck", primaryMutation(s.handlePrimaryAgentRecheck, false))
	mux.HandleFunc("GET /api/channels", phase1LocalOnly(s.handlePrimaryChannelsList))
	mux.HandleFunc("POST /api/channels", primaryMutation(s.handlePrimaryChannelCreate, true))
	mux.HandleFunc("GET /api/channels/{id}", phase1LocalOnly(s.handlePrimaryChannelGet))
	mux.HandleFunc("PATCH /api/channels/{id}", primaryMutation(s.handlePrimaryChannelPatch, true))
	mux.HandleFunc("POST /api/channels/{id}/turns", primaryMutation(s.handlePrimaryChannelTurn, true))
	mux.HandleFunc("POST /api/channels/{id}/targets/{target_id}/retry", primaryMutation(s.handlePrimaryTargetRetry, false))
	mux.HandleFunc("POST /api/channels/{id}/targets/{target_id}/recheck-and-retry", primaryMutation(s.handlePrimaryTargetRecheckAndRetry, false))
	mux.HandleFunc("POST /api/channels/{id}/targets/{target_id}/cancel", primaryMutation(s.handlePrimaryTargetCancel, false))
	mux.HandleFunc("GET /api/channels/{id}/events", phase1LocalOnly(s.handlePrimaryChannelEvents))
	mux.HandleFunc("GET /api/needs-you", phase1LocalOnly(s.handlePrimaryNeedsYou))
}

func phase1LocalOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !phase1TrustedRequest(r) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "Primary Channels require loopback or authenticated relay transport"})
			return
		}
		next(w, r)
	}
}

func primaryMutation(next http.HandlerFunc, jsonBody bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !phase1TrustedRequest(r) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "Primary Channels require loopback or authenticated relay transport"})
			return
		}
		if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "cross-origin mutation denied"})
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			parsed, err := url.Parse(origin)
			if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") ||
				parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" ||
				!strings.EqualFold(parsed.Host, r.Host) {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "cross-origin mutation denied"})
				return
			}
		}
		if jsonBody {
			mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if err != nil || mediaType != "application/json" {
				writeJSON(w, http.StatusUnsupportedMediaType, map[string]string{"error": "Content-Type must be application/json"})
				return
			}
		}
		next(w, r)
	}
}

func phase1TrustedRequest(r *http.Request) bool {
	if loopbackRequest(r) && primaryLoopbackHost(r.Host) {
		return true
	}
	return strings.HasPrefix(r.URL.Path, "/api/") && transporttrust.Trusted(r.Context())
}

func primaryLoopbackHost(authority string) bool {
	host := authority
	if parsed, _, err := net.SplitHostPort(authority); err == nil {
		host = parsed
	}
	host = strings.Trim(host, "[]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (s *Server) handlePrimaryAgentGet(w http.ResponseWriter, r *http.Request) {
	if !primaryQuery(w, r) || !s.requirePrimary(w) {
		return
	}
	view, err := s.d.Primary.PrimaryAgent(r.Context())
	if err != nil {
		primaryAPIError(w, err)
		return
	}
	if err := s.attachScheduleInventory(r.Context(), &view); err != nil {
		primaryAPIError(w, err)
		return
	}
	normalizePrimaryAgentView(&view)
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) handlePrimaryAgentPut(w http.ResponseWriter, r *http.Request) {
	if !primaryQuery(w, r) || !s.requirePrimary(w) {
		return
	}
	var request primaryAgentRequest
	if !decodePrimaryJSON(w, r, &request) {
		return
	}
	request.OptionID = strings.TrimSpace(request.OptionID)
	if request.OptionID == "" || len(request.OptionID) > 512 {
		primaryBadRequest(w, "option_id must be non-empty and at most 512 bytes")
		return
	}
	view, err := s.d.Primary.SetPrimaryAgent(r.Context(), request.OptionID)
	if err != nil {
		primaryAPIError(w, err)
		return
	}
	_ = s.attachScheduleInventory(r.Context(), &view)
	normalizePrimaryAgentView(&view)
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) handlePrimaryAgentDelete(w http.ResponseWriter, r *http.Request) {
	if !primaryQuery(w, r) || !requirePrimaryEmptyBody(w, r) || !s.requirePrimary(w) {
		return
	}
	if err := s.d.Primary.ClearPrimaryAgent(r.Context()); err != nil {
		primaryAPIError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handlePrimaryAgentRecheck(w http.ResponseWriter, r *http.Request) {
	if !primaryQuery(w, r) || !requirePrimaryEmptyBody(w, r) || !s.requirePrimary(w) {
		return
	}
	view, err := s.d.Primary.RecheckPrimaryAgent(r.Context())
	if err != nil {
		primaryAPIError(w, err)
		return
	}
	_ = s.attachScheduleInventory(r.Context(), &view)
	normalizePrimaryAgentView(&view)
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) handlePrimaryChannelsList(w http.ResponseWriter, r *http.Request) {
	query, ok := primaryQueryValues(w, r, "state")
	if !ok || !s.requirePrimary(w) {
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
	items, err := s.d.Primary.ListChannels(r.Context(), state)
	if err != nil {
		primaryAPIError(w, err)
		return
	}
	if items == nil {
		items = []conversation.PrimaryChannelSummary{}
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handlePrimaryChannelCreate(w http.ResponseWriter, r *http.Request) {
	if !primaryQuery(w, r) || !s.requirePrimary(w) {
		return
	}
	var request primaryChannelRequest
	if !decodePrimaryJSON(w, r, &request) {
		return
	}
	name, ok := validatePrimaryChannelName(w, request.Name)
	if !ok {
		return
	}
	detail, err := s.d.Primary.CreateChannel(r.Context(), name)
	if err != nil {
		primaryAPIError(w, err)
		return
	}
	normalizePrimaryDetail(&detail)
	writeJSON(w, http.StatusCreated, detail)
}

func (s *Server) handlePrimaryChannelGet(w http.ResponseWriter, r *http.Request) {
	if !primaryQuery(w, r) || !s.requirePrimary(w) || !validPrimaryPathID(w, r.PathValue("id"), "Channel id") {
		return
	}
	detail, err := s.d.Primary.GetChannel(r.Context(), r.PathValue("id"))
	if err != nil {
		primaryAPIError(w, err)
		return
	}
	normalizePrimaryDetail(&detail)
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handlePrimaryChannelPatch(w http.ResponseWriter, r *http.Request) {
	if !primaryQuery(w, r) || !s.requirePrimary(w) || !validPrimaryPathID(w, r.PathValue("id"), "Channel id") {
		return
	}
	var request primaryChannelPatch
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
	if _, err := s.d.Primary.GetChannel(r.Context(), r.PathValue("id")); err != nil {
		primaryAPIError(w, err)
		return
	}
	var err error
	switch {
	case request.Name != nil:
		var name string
		var ok bool
		name, ok = validatePrimaryChannelName(w, *request.Name)
		if !ok {
			return
		}
		err = s.d.Primary.RenameChannel(r.Context(), r.PathValue("id"), name)
	case request.State != nil:
		if *request.State != conversation.ConversationOpen && *request.State != conversation.ConversationArchived {
			primaryBadRequest(w, "state must be open or archived")
			return
		}
		err = s.d.Primary.SetChannelState(r.Context(), r.PathValue("id"), *request.State)
	case request.Pinned != nil:
		err = s.d.Primary.SetChannelPinned(r.Context(), r.PathValue("id"), *request.Pinned)
	}
	if err != nil {
		primaryAPIError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handlePrimaryChannelTurn(w http.ResponseWriter, r *http.Request) {
	if !primaryQuery(w, r) || !s.requirePrimary(w) || !validPrimaryPathID(w, r.PathValue("id"), "Channel id") {
		return
	}
	var request primaryTurnRequest
	if !decodePrimaryJSON(w, r, &request) {
		return
	}
	if parsed, err := uuid.Parse(request.ClientTurnID); err != nil || parsed.String() != request.ClientTurnID {
		primaryBadRequest(w, "client_turn_id must be a canonical UUID")
		return
	}
	request.Text = strings.TrimSpace(request.Text)
	if request.Text == "" || len([]byte(request.Text)) > conversation.MaxContextBytes || !utf8.ValidString(request.Text) {
		primaryBadRequest(w, "text must contain 1 to 65536 UTF-8 bytes")
		return
	}
	result, err := s.d.Primary.PostTurn(r.Context(), r.PathValue("id"), request.ClientTurnID, request.Text)
	if err != nil {
		primaryAPIError(w, err)
		return
	}
	if result.Targets == nil {
		result.Targets = []conversation.Target{}
	}
	writeJSON(w, http.StatusAccepted, result)
}

func (s *Server) handlePrimaryTargetRetry(w http.ResponseWriter, r *http.Request) {
	s.handlePrimaryTargetRetryKind(w, r, false)
}

func (s *Server) handlePrimaryTargetRecheckAndRetry(w http.ResponseWriter, r *http.Request) {
	s.handlePrimaryTargetRetryKind(w, r, true)
}

func (s *Server) handlePrimaryTargetRetryKind(w http.ResponseWriter, r *http.Request, recheck bool) {
	if !primaryQuery(w, r) || !requirePrimaryEmptyBody(w, r) || !s.requirePrimary(w) ||
		!validPrimaryPathID(w, r.PathValue("id"), "Channel id") ||
		!validPrimaryPathID(w, r.PathValue("target_id"), "target id") {
		return
	}
	detail, target, ok := s.primaryNestedTarget(w, r)
	if !ok {
		return
	}
	if target.State != conversation.TargetFailed || !primaryLatestAttempt(detail.Targets, target) {
		primaryConflict(w, "only the latest failed target can be retried", "")
		return
	}
	var (
		created conversation.Target
		err     error
	)
	if recheck {
		created, err = s.d.Primary.RecheckAndRetryTarget(r.Context(), r.PathValue("id"), target.ID)
	} else {
		created, err = s.d.Primary.RetryTarget(r.Context(), r.PathValue("id"), target.ID)
	}
	if err != nil {
		if s.primaryTargetNowConflicts(r, target.ID, true) {
			primaryConflict(w, "only the latest failed target can be retried", "")
		} else {
			primaryAPIError(w, err)
		}
		return
	}
	writeJSON(w, http.StatusAccepted, created)
}

func (s *Server) handlePrimaryTargetCancel(w http.ResponseWriter, r *http.Request) {
	if !primaryQuery(w, r) || !requirePrimaryEmptyBody(w, r) || !s.requirePrimary(w) ||
		!validPrimaryPathID(w, r.PathValue("id"), "Channel id") ||
		!validPrimaryPathID(w, r.PathValue("target_id"), "target id") {
		return
	}
	_, target, ok := s.primaryNestedTarget(w, r)
	if !ok {
		return
	}
	if target.State != conversation.TargetQueued && target.State != conversation.TargetWorking {
		primaryConflict(w, "only a queued or working target can be canceled", "")
		return
	}
	if err := s.d.Primary.CancelTarget(r.Context(), r.PathValue("id"), target.ID); err != nil {
		if s.primaryTargetNowConflicts(r, target.ID, false) {
			primaryConflict(w, "only a queued or working target can be canceled", "")
		} else {
			primaryAPIError(w, err)
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handlePrimaryNeedsYou(w http.ResponseWriter, r *http.Request) {
	if !primaryQuery(w, r) || !s.requirePrimary(w) {
		return
	}
	items, err := s.d.Primary.NeedsYou(r.Context())
	if err != nil {
		primaryAPIError(w, err)
		return
	}
	if items == nil {
		items = []PrimaryNeedsYouItem{}
	}
	for index := range items {
		if items[index].RecoveryActions == nil {
			items[index].RecoveryActions = []string{}
		}
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handlePrimaryChannelEvents(w http.ResponseWriter, r *http.Request) {
	if !primaryQuery(w, r) || !s.requirePrimary(w) || !validPrimaryPathID(w, r.PathValue("id"), "Channel id") {
		return
	}
	detail, err := s.d.Primary.GetChannel(r.Context(), r.PathValue("id"))
	if err != nil {
		primaryAPIError(w, err)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Channel events are unavailable"})
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	last := ""
	for {
		normalizePrimaryDetail(&detail)
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
		detail, err = s.d.Primary.GetChannel(r.Context(), r.PathValue("id"))
		if err != nil {
			return
		}
	}
}

func (s *Server) requirePrimary(w http.ResponseWriter) bool {
	if s.d.Primary == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Primary Channels are unavailable"})
		return false
	}
	return true
}

func (s *Server) attachScheduleInventory(ctx context.Context, view *PrimaryAgentView) error {
	if view == nil || s.d.ScheduleInventory == nil {
		return nil
	}
	inventory, err := s.d.ScheduleInventory.Inventory(ctx, s.d.AcceptedScheduleInventory)
	if err != nil && inventory.State != ScheduleInventoryUnaccepted && inventory.State != ScheduleInventoryDrift {
		return err
	}
	view.ScheduleInventory = &inventory
	return nil
}

func (s *Server) primaryNestedTarget(w http.ResponseWriter, r *http.Request) (PrimaryChannelDetail, conversation.Target, bool) {
	detail, err := s.d.Primary.GetChannel(r.Context(), r.PathValue("id"))
	if err != nil {
		primaryAPIError(w, err)
		return PrimaryChannelDetail{}, conversation.Target{}, false
	}
	for _, target := range detail.Targets {
		if target.ID == r.PathValue("target_id") {
			return detail, target, true
		}
	}
	primaryNotFound(w)
	return PrimaryChannelDetail{}, conversation.Target{}, false
}

func (s *Server) primaryTargetNowConflicts(r *http.Request, targetID string, retry bool) bool {
	detail, err := s.d.Primary.GetChannel(r.Context(), r.PathValue("id"))
	if err != nil {
		return false
	}
	for _, target := range detail.Targets {
		if target.ID != targetID {
			continue
		}
		if retry {
			return target.State != conversation.TargetFailed || !primaryLatestAttempt(detail.Targets, target)
		}
		return target.State != conversation.TargetQueued && target.State != conversation.TargetWorking
	}
	return false
}

func primaryLatestAttempt(targets []conversation.Target, selected conversation.Target) bool {
	for _, target := range targets {
		if target.TurnID == selected.TurnID && target.ParticipantID == selected.ParticipantID && target.Attempt > selected.Attempt {
			return false
		}
	}
	return true
}

func normalizePrimaryAgentView(view *PrimaryAgentView) {
	if view.Options == nil {
		view.Options = []PrimaryAgentOption{}
	}
}

func normalizePrimaryDetail(detail *PrimaryChannelDetail) {
	if detail.Participants == nil {
		detail.Participants = []conversation.Participant{}
	}
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

func validatePrimaryChannelName(w http.ResponseWriter, raw string) (string, bool) {
	name := strings.TrimSpace(raw)
	if name == "" || len([]byte(name)) > 120 || !utf8.ValidString(name) {
		primaryBadRequest(w, "name must contain 1 to 120 UTF-8 bytes")
		return "", false
	}
	return name, true
}

func validPrimaryPathID(w http.ResponseWriter, id, label string) bool {
	if strings.TrimSpace(id) != id || id == "" || len(id) > 256 || !utf8.ValidString(id) {
		primaryBadRequest(w, label+" is invalid")
		return false
	}
	for _, value := range id {
		if value < 0x20 || value == 0x7f || value == '/' {
			primaryBadRequest(w, label+" is invalid")
			return false
		}
	}
	return true
}

func decodePrimaryJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, primaryRequestLimit)
	data, err := io.ReadAll(r.Body)
	if err != nil || !utf8.Valid(data) {
		primaryBadRequest(w, "invalid JSON request")
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		primaryBadRequest(w, "invalid JSON request")
		return false
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		primaryBadRequest(w, "request body must contain one JSON object")
		return false
	}
	return true
}

func requirePrimaryEmptyBody(w http.ResponseWriter, r *http.Request) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1)
	data, err := io.ReadAll(r.Body)
	if err != nil || len(strings.TrimSpace(string(data))) != 0 {
		primaryBadRequest(w, "request body must be empty")
		return false
	}
	return true
}

func primaryQuery(w http.ResponseWriter, r *http.Request) bool {
	_, ok := primaryQueryValues(w, r)
	return ok
}

func primaryQueryValues(w http.ResponseWriter, r *http.Request, allowed ...string) (url.Values, bool) {
	allowedKeys := make(map[string]bool, len(allowed))
	for _, key := range allowed {
		allowedKeys[key] = true
	}
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		primaryBadRequest(w, "invalid query")
		return nil, false
	}
	for key, values := range query {
		if !allowedKeys[key] || len(values) != 1 || values[0] == "" {
			primaryBadRequest(w, "invalid query")
			return nil, false
		}
	}
	return query, true
}

func primaryAPIError(w http.ResponseWriter, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		primaryNotFound(w)
		return
	}
	if errors.Is(err, conversation.ErrContextTooLarge) {
		primaryConflict(w, err.Error(), "conversation_context_limit")
		return
	}
	var bounded *conversation.BoundedError
	if errors.As(err, &bounded) {
		if bounded.Code == conversation.ErrorConversationActive {
			primaryConflict(w, bounded.Error(), string(bounded.Code))
		} else {
			writeJSON(w, http.StatusBadRequest, map[string]string{"code": string(bounded.Code), "error": bounded.Error()})
		}
		return
	}
	var coded primaryChannelCodedError
	if errors.As(err, &coded) {
		code := coded.PrimaryChannelCode()
		if code == "chat_policy_unavailable" {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"code": code, "error": err.Error()})
			return
		}
		if primaryConflictCodes[code] {
			primaryConflict(w, err.Error(), code)
			return
		}
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Primary Channels are unavailable"})
}

func primaryBadRequest(w http.ResponseWriter, message string) {
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": message})
}

func primaryNotFound(w http.ResponseWriter) {
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
}

func primaryConflict(w http.ResponseWriter, message, code string) {
	response := map[string]string{"error": message}
	if code != "" {
		response["code"] = code
	}
	writeJSON(w, http.StatusConflict, response)
}
