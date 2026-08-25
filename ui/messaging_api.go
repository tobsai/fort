package ui

import (
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"
)

type messagingPostRequest struct {
	ClientMessageID string `json:"client_message_id"`
	Text            string `json:"text"`
}

func (s *Server) registerConfiguredMessagingRoutes(mux *http.ServeMux) {
	if s.d.Messaging == nil {
		return
	}
	mux.HandleFunc("GET /api/messaging/peers", phase1LocalOnly(s.handleMessagingPeers))
	mux.HandleFunc("GET /api/messaging/channels", phase1LocalOnly(s.handleMessagingChannels))
	mux.HandleFunc("GET /api/messaging/conversations/{conversation_id}/events", phase1LocalOnly(s.handleMessagingEvents))
	mux.HandleFunc("POST /api/messaging/conversations/{conversation_id}/messages", primaryMutation(s.handleMessagingPost, true))
}

func (s *Server) handleMessagingChannels(w http.ResponseWriter, r *http.Request) {
	if !primaryQuery(w, r) || !requirePrimaryEmptyBody(w, r) {
		return
	}
	directory, ok := s.d.Messaging.(MessagingChannelDirectory)
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"code":  "messaging_channel_directory_unavailable",
			"error": "messaging channel directory is unavailable",
		})
		return
	}
	channels, err := directory.MessagingChannels(r.Context())
	if err != nil {
		messagingAPIError(w, err)
		return
	}
	if channels == nil {
		channels = []MessagingPeer{}
	}
	writeJSON(w, http.StatusOK, channels)
}

func (s *Server) handleMessagingPeers(w http.ResponseWriter, r *http.Request) {
	if !primaryQuery(w, r) || !requirePrimaryEmptyBody(w, r) {
		return
	}
	// The legacy proof endpoint may expose only its configured single peer.
	// Dynamic directories have their own /channels contract and must not leak
	// that roster through the historical route.
	if _, dynamic := s.d.Messaging.(MessagingChannelDirectory); dynamic {
		writeJSON(w, http.StatusOK, []MessagingPeer{})
		return
	}
	peers, err := s.d.Messaging.MessagingPeers(r.Context())
	if err != nil {
		messagingAPIError(w, err)
		return
	}
	if peers == nil {
		peers = []MessagingPeer{}
	}
	writeJSON(w, http.StatusOK, peers)
}

func (s *Server) handleMessagingEvents(w http.ResponseWriter, r *http.Request) {
	conversationID := r.PathValue("conversation_id")
	if !validPrimaryPathID(w, conversationID, "Conversation id") || !requirePrimaryEmptyBody(w, r) {
		return
	}
	query, ok := primaryQueryValues(w, r, "after")
	if !ok {
		return
	}
	var after int64
	if raw := query.Get("after"); raw != "" {
		var err error
		after, err = strconv.ParseInt(raw, 10, 64)
		if err != nil || after < 0 {
			primaryBadRequest(w, "after must be a non-negative event sequence")
			return
		}
	}
	page, err := s.d.Messaging.MessagingEvents(r.Context(), conversationID, after)
	if err != nil {
		messagingAPIError(w, err)
		return
	}
	if page.Events == nil {
		page.Events = []MessagingEvent{}
	}
	writeJSON(w, http.StatusOK, page)
}

func (s *Server) handleMessagingPost(w http.ResponseWriter, r *http.Request) {
	conversationID := r.PathValue("conversation_id")
	if !validPrimaryPathID(w, conversationID, "Conversation id") {
		return
	}
	var request messagingPostRequest
	if !decodePrimaryJSON(w, r, &request) {
		return
	}
	request.ClientMessageID = strings.TrimSpace(request.ClientMessageID)
	request.Text = strings.TrimSpace(request.Text)
	if request.ClientMessageID == "" || len(request.ClientMessageID) > 256 || !utf8.ValidString(request.ClientMessageID) {
		primaryBadRequest(w, "client_message_id is invalid")
		return
	}
	if request.Text == "" || len([]rune(request.Text)) > 4096 || !utf8.ValidString(request.Text) {
		primaryBadRequest(w, "text must contain 1 to 4096 UTF-8 characters")
		return
	}
	receipt, err := s.d.Messaging.PostMessagingMessage(r.Context(), conversationID, request.ClientMessageID, request.Text)
	if err != nil {
		if receipt.Message.ID != "" && receipt.Message.ConversationID == conversationID &&
			receipt.AcceptedSequence > 0 && receipt.DeliveryState == MessagingDeliveryUnknown &&
			receipt.DeliveryCode != "" {
			writeJSON(w, http.StatusAccepted, receipt)
			return
		}
		messagingAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, receipt)
}

type messagingCodedError interface {
	MessagingCode() string
}

func messagingAPIError(w http.ResponseWriter, err error) {
	if coded, ok := err.(messagingCodedError); ok {
		writeJSON(w, http.StatusConflict, map[string]string{"code": coded.MessagingCode(), "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]string{"code": "messaging_unavailable", "error": "messaging is unavailable"})
}
