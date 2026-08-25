// Package hermesplatform accepts authenticated profile-scoped connections from
// Hermes' Fort platform adapter. It owns only the transport; Fort's control
// module remains authoritative for channel identity, roster, and transcripts.
package hermesplatform

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/coder/websocket"
)

type Registration struct {
	ConnectionID       string
	CanonicalProfileID string
	DisplayName        string
}

type RegistrationReceipt struct {
	ChannelID      string
	ConversationID string
}

type Message struct {
	RequestID          string
	ConversationID     string
	Text               string
	InReplyToMessageID string
}

// Sender is the exact authenticated Hermes connection registered for one
// profile. A disconnected sender never selects or falls back to another one.
type Sender interface {
	Connected() bool
	Send(context.Context, string, string) error
}

type Config struct {
	ProfileTokenKey string
	SenderID        string
	SenderName      string
	Register        func(context.Context, Registration, Sender) (RegistrationReceipt, error)
	Deliver         func(context.Context, string, Message) (string, error)
}

type Connector struct {
	cfg Config
}

func New(cfg Config) (*Connector, error) {
	for label, value := range map[string]string{
		"profile token key": cfg.ProfileTokenKey, "sender id": cfg.SenderID, "sender name": cfg.SenderName,
	} {
		if value == "" || strings.TrimSpace(value) != value || strings.ContainsAny(value, "\r\n") {
			return nil, fmt.Errorf("Hermes platform %s is required and canonical", label)
		}
	}
	if cfg.Register == nil || cfg.Deliver == nil {
		return nil, errors.New("Hermes platform requires registration and delivery callbacks")
	}
	return &Connector{cfg: cfg}, nil
}

func (connector *Connector) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/platforms/hermes", connector.handle)
	return mux
}

func (connector *Connector) handle(w http.ResponseWriter, r *http.Request) {
	profileID := r.Header.Get("X-Fort-Hermes-Profile")
	profileToken, err := DeriveProfileToken(connector.cfg.ProfileTokenKey, profileID)
	if r.Method != http.MethodGet || err != nil ||
		!validBearer(r.Header.Get("Authorization"), profileToken) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	socket, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer socket.CloseNow()
	socket.SetReadLimit(1 << 20)

	var frame registerFrame
	if err := readJSON(r.Context(), socket, &frame); err != nil ||
		frame.Type != "register" || frame.ContractVersion != 1 || frame.ProfileID != profileID {
		_ = socket.Close(websocket.StatusPolicyViolation, "invalid registration")
		return
	}
	connectionID, err := randomConnectionID()
	if err != nil {
		_ = socket.Close(websocket.StatusInternalError, "registration unavailable")
		return
	}
	session := &session{
		connector: connector, socket: socket, connected: true, ready: make(chan struct{}),
	}
	receipt, err := connector.cfg.Register(r.Context(), Registration{
		ConnectionID: connectionID, CanonicalProfileID: frame.ProfileID, DisplayName: frame.DisplayName,
	}, session)
	if err != nil || receipt.ChannelID == "" || receipt.ConversationID == "" {
		session.disconnect()
		_ = socket.Close(websocket.StatusPolicyViolation, "registration rejected")
		return
	}
	if err := session.write(r.Context(), registeredFrame{
		Type: "registered", ChannelID: receipt.ChannelID, ConversationID: receipt.ConversationID,
	}); err != nil {
		session.disconnect()
		return
	}
	session.activate(receipt)
	defer session.disconnect()

	for {
		var outbound outboundFrame
		if err := readJSON(r.Context(), socket, &outbound); err != nil {
			return
		}
		if outbound.Type != "outbound" || outbound.RequestID == "" ||
			outbound.ConversationID != receipt.ConversationID || strings.TrimSpace(outbound.Text) == "" {
			_ = session.write(r.Context(), failureFrame{
				Type: "failure", RequestID: outbound.RequestID, Code: "invalid_outbound_message",
			})
			continue
		}
		messageID, err := connector.cfg.Deliver(r.Context(), receipt.ChannelID, Message{
			RequestID: outbound.RequestID, ConversationID: outbound.ConversationID,
			Text: outbound.Text, InReplyToMessageID: outbound.InReplyToMessageID,
		})
		if err != nil || messageID == "" {
			_ = session.write(r.Context(), failureFrame{
				Type: "failure", RequestID: outbound.RequestID, Code: "fort_delivery_failed",
			})
			continue
		}
		if err := session.write(r.Context(), receiptFrame{
			Type: "receipt", RequestID: outbound.RequestID, MessageID: messageID,
		}); err != nil {
			return
		}
	}
}

var canonicalHermesProfileID = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)

// DeriveProfileToken isolates one canonical Hermes profile under a
// machine-scoped root key. The profile boundary is length-prefixed so the
// derivation cannot become ambiguous if the domain evolves to include more
// fields.
func DeriveProfileToken(rootKey, canonicalProfileID string) (string, error) {
	if rootKey == "" || strings.TrimSpace(rootKey) != rootKey || strings.ContainsAny(rootKey, "\r\n") {
		return "", errors.New("Hermes platform profile token key is required and canonical")
	}
	if !validHermesProfileID(canonicalProfileID) {
		return "", errors.New("Hermes platform profile id is not canonical")
	}
	digest := hmac.New(sha256.New, []byte(rootKey))
	_, _ = digest.Write([]byte("fort-hermes-profile-token:v1\n"))
	_, _ = digest.Write([]byte(strconv.Itoa(len([]byte(canonicalProfileID)))))
	_, _ = digest.Write([]byte{':'})
	_, _ = digest.Write([]byte(canonicalProfileID))
	return base64.RawURLEncoding.EncodeToString(digest.Sum(nil)), nil
}

func validHermesProfileID(value string) bool {
	if !canonicalHermesProfileID.MatchString(value) {
		return false
	}
	switch value {
	case "hermes", "test", "tmp", "root", "sudo":
		return false
	default:
		return true
	}
}

type session struct {
	connector *Connector
	socket    *websocket.Conn

	mu             sync.RWMutex
	connected      bool
	active         bool
	channelID      string
	conversationID string
	ready          chan struct{}
	activateOnce   sync.Once
	writeMu        sync.Mutex
}

func (session *session) Connected() bool {
	session.mu.RLock()
	connected := session.connected && session.active
	session.mu.RUnlock()
	return connected
}

// Reserved distinguishes an authenticated registration in progress from a
// channel that Hermes may use. Fort uses it only to reject a concurrent socket;
// Connected remains false until the registered acknowledgement is written.
func (session *session) Reserved() bool {
	session.mu.RLock()
	reserved := session.connected
	session.mu.RUnlock()
	return reserved
}

func (session *session) Send(ctx context.Context, messageID, text string) error {
	select {
	case <-session.ready:
	case <-ctx.Done():
		return ctx.Err()
	}
	session.mu.RLock()
	connected := session.connected
	conversationID := session.conversationID
	session.mu.RUnlock()
	if !connected {
		return errors.New("Hermes platform adapter is offline")
	}
	return session.write(ctx, inboundFrame{
		Type: "inbound", RequestID: "fort:" + messageID, MessageID: messageID,
		ConversationID: conversationID, Text: text,
		AuthorID: session.connector.cfg.SenderID, AuthorName: session.connector.cfg.SenderName,
	})
}

func (session *session) activate(receipt RegistrationReceipt) {
	session.mu.Lock()
	session.channelID = receipt.ChannelID
	session.conversationID = receipt.ConversationID
	session.active = true
	session.mu.Unlock()
	session.activateOnce.Do(func() { close(session.ready) })
}

func (session *session) disconnect() {
	session.mu.Lock()
	session.connected = false
	session.active = false
	session.mu.Unlock()
}

func (session *session) write(ctx context.Context, value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	session.writeMu.Lock()
	defer session.writeMu.Unlock()
	return session.socket.Write(ctx, websocket.MessageText, payload)
}

type registerFrame struct {
	Type            string `json:"type"`
	ContractVersion int    `json:"contract_version"`
	ProfileID       string `json:"profile_id"`
	DisplayName     string `json:"display_name"`
}

type registeredFrame struct {
	Type           string `json:"type"`
	ChannelID      string `json:"channel_id"`
	ConversationID string `json:"conversation_id"`
}

type inboundFrame struct {
	Type           string `json:"type"`
	RequestID      string `json:"request_id"`
	MessageID      string `json:"message_id"`
	ConversationID string `json:"conversation_id"`
	Text           string `json:"text"`
	AuthorID       string `json:"author_id"`
	AuthorName     string `json:"author_name"`
}

type outboundFrame struct {
	Type               string `json:"type"`
	RequestID          string `json:"request_id"`
	ConversationID     string `json:"conversation_id"`
	Text               string `json:"text"`
	InReplyToMessageID string `json:"in_reply_to_message_id"`
}

type receiptFrame struct {
	Type      string `json:"type"`
	RequestID string `json:"request_id"`
	MessageID string `json:"message_id"`
}

type failureFrame struct {
	Type      string `json:"type"`
	RequestID string `json:"request_id,omitempty"`
	Code      string `json:"code"`
}

func readJSON(ctx context.Context, socket *websocket.Conn, value any) error {
	messageType, payload, err := socket.Read(ctx)
	if err != nil {
		return err
	}
	if messageType != websocket.MessageText || len(payload) == 0 {
		return errors.New("Hermes platform frame must be non-empty text")
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return errors.New("Hermes platform frame must contain one JSON object")
	}
	return nil
}

func validBearer(header, token string) bool {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) || len(header) != len(prefix)+len(token) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(header[len(prefix):]), []byte(token)) == 1
}

func randomConnectionID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return "hermes-platform-connection:" + hex.EncodeToString(value[:]), nil
}
