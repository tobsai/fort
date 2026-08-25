// Package hermesrelaypoc is a disposable proof that Hermes can use Fort as a
// messaging platform through Hermes's released relay contract.
package hermesrelaypoc

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// Config fixes the one Hermes identity and one Fort recipient used by the
// proof. SharedSecret is used only to authenticate Hermes's outbound socket.
type Config struct {
	GatewayID             string
	SharedSecret          string
	BindingID             string
	CanonicalProfileID    string // enrollment mapping; the v1 relay wire does not attest it
	BotID                 string
	BotDisplayName        string
	AllowedConversationID string
	SenderID              string
	SenderName            string
	ObserveAccepted       bool
	Deliver               func(context.Context, Message) (string, error)
}

// Bot is the enrolled Hermes identity presented by the connector.
type Bot struct {
	BindingID          string
	CanonicalProfileID string
	BotID              string
	Label              string
	Connected          bool
}

// Message is one Hermes-authored text message addressed to Fort.
type Message struct {
	ID                 string
	RequestID          string
	ConversationID     string
	Text               string
	InReplyToMessageID string
}

// Connector serves one authenticated Hermes relay socket.
type Connector struct {
	cfg Config

	mu        sync.RWMutex
	conn      *websocket.Conn
	connected bool
	writeMu   sync.Mutex
	received  chan Message
}

// ErrAcceptedObservationDisabled reports that the optional terminal-only
// accepted-message observer was not enabled for this connector.
var ErrAcceptedObservationDisabled = errors.New("accepted Hermes message observation is disabled")

// New creates the single-binding connector described by Spec 051.
func New(cfg Config) (*Connector, error) {
	if cfg.GatewayID == "" || cfg.SharedSecret == "" || cfg.BindingID == "" ||
		cfg.CanonicalProfileID == "" || cfg.BotID == "" || cfg.BotDisplayName == "" ||
		cfg.AllowedConversationID == "" || cfg.SenderID == "" || cfg.Deliver == nil {
		return nil, errors.New("Hermes relay proof requires one complete immutable binding")
	}
	connector := &Connector{cfg: cfg}
	if cfg.ObserveAccepted {
		connector.received = make(chan Message, 8)
	}
	return connector, nil
}

// Handler accepts Hermes's outbound WebSocket at /relay.
func (c *Connector) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/relay", c.handleRelay)
	return mux
}

// Bot reports the exact enrolled identity and current socket state.
func (c *Connector) Bot() Bot {
	c.mu.RLock()
	connected := c.connected
	c.mu.RUnlock()
	return Bot{
		BindingID:          c.cfg.BindingID,
		CanonicalProfileID: c.cfg.CanonicalProfileID,
		BotID:              c.cfg.BotID,
		Label:              "Hermes & " + c.cfg.BotDisplayName,
		Connected:          connected,
	}
}

// Send carries one Fort-authored text message to the connected Hermes profile.
func (c *Connector) Send(ctx context.Context, messageID, text string) error {
	c.mu.RLock()
	conn := c.conn
	connected := c.connected
	c.mu.RUnlock()
	if conn == nil || !connected {
		return errors.New("Hermes relay is not connected")
	}

	frame := inboundFrame{Type: "inbound"}
	frame.Event.Text = text
	frame.Event.MessageType = "text"
	frame.Event.MessageID = messageID
	frame.Event.Source.Platform = "relay"
	frame.Event.Source.ChatID = c.cfg.AllowedConversationID
	frame.Event.Source.ChatType = "dm"
	frame.Event.Source.ChatName = "Home"
	frame.Event.Source.UserID = c.cfg.SenderID
	frame.Event.Source.UserName = c.cfg.SenderName
	frame.Event.Source.MessageID = messageID
	return c.write(ctx, conn, frame)
}

// Receive waits for the next Hermes-authored message accepted for the one
// configured Fort Conversation.
func (c *Connector) Receive(ctx context.Context) (Message, error) {
	if c.received == nil {
		return Message{}, ErrAcceptedObservationDisabled
	}
	select {
	case message := <-c.received:
		return message, nil
	case <-ctx.Done():
		return Message{}, ctx.Err()
	}
}

func (c *Connector) handleRelay(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer conn.CloseNow()
	if !validAuthorization(r.Header.Get("Authorization"), c.cfg.GatewayID, c.cfg.SharedSecret, time.Now()) {
		_ = conn.Close(websocket.StatusCode(4401), "unauthorized")
		return
	}
	conn.SetReadLimit(1 << 20)

	var hello helloFrame
	if err := readLine(r.Context(), conn, &hello); err != nil ||
		hello.Type != "hello" || hello.Platform != "relay" || hello.BotID != c.cfg.BotID {
		_ = conn.Close(websocket.StatusPolicyViolation, "invalid Hermes relay identity")
		return
	}
	if !c.attach(conn) {
		_ = conn.Close(websocket.StatusPolicyViolation, "Hermes relay already connected")
		return
	}
	defer c.detach(conn)

	if err := c.write(r.Context(), conn, descriptorFrame{
		Type: "descriptor",
		Descriptor: descriptor{
			ContractVersion:        1,
			Platform:               "relay",
			Label:                  "Fort",
			MaxMessageLength:       4096,
			SupportsDraftStreaming: false,
			SupportsEdit:           false,
			SupportsThreads:        false,
			MarkdownDialect:        "plain",
			LengthUnit:             "chars",
			SupportedOperations:    []string{"send"},
		},
	}); err != nil {
		return
	}

	for {
		var outbound outboundFrame
		if err := readLine(r.Context(), conn, &outbound); err != nil {
			return
		}
		if outbound.Type != "outbound" {
			_ = conn.Close(websocket.StatusUnsupportedData, "unsupported Hermes relay frame")
			return
		}
		c.acceptOutbound(r.Context(), conn, outbound)
	}
}

func (c *Connector) attach(conn *websocket.Conn) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		return false
	}
	c.conn = conn
	c.connected = true
	return true
}

func (c *Connector) detach(conn *websocket.Conn) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == conn {
		c.conn = nil
		c.connected = false
	}
}

func (c *Connector) acceptOutbound(ctx context.Context, conn *websocket.Conn, outbound outboundFrame) {
	if outbound.RequestID == "" || outbound.Action.Operation != "send" {
		c.writeFailure(ctx, conn, outbound.RequestID, "unsupported relay operation")
		return
	}
	if (outbound.Platform != "" && outbound.Platform != "relay") ||
		(outbound.BotID != "" && outbound.BotID != c.cfg.BotID) {
		c.writeFailure(ctx, conn, outbound.RequestID, "relay identity mismatch")
		return
	}
	if outbound.Action.ChatID != c.cfg.AllowedConversationID {
		c.writeFailure(ctx, conn, outbound.RequestID, "recipient is not allowed")
		return
	}

	message := Message{
		RequestID:          outbound.RequestID,
		ConversationID:     outbound.Action.ChatID,
		Text:               outbound.Action.Content,
		InReplyToMessageID: outbound.Action.ReplyTo,
	}
	messageID, err := c.cfg.Deliver(ctx, message)
	if err != nil || messageID == "" {
		c.writeFailure(ctx, conn, outbound.RequestID, "Fort delivery failed")
		return
	}
	message.ID = messageID
	if c.received != nil {
		select {
		case c.received <- message:
		case <-ctx.Done():
			return
		}
	}
	_ = c.write(ctx, conn, outboundResultFrame{
		Type:      "outbound_result",
		RequestID: outbound.RequestID,
		Result: outboundResult{
			Success:   true,
			MessageID: message.ID,
		},
	})
}

func (c *Connector) writeFailure(ctx context.Context, conn *websocket.Conn, requestID, message string) {
	_ = c.write(ctx, conn, outboundResultFrame{
		Type:      "outbound_result",
		RequestID: requestID,
		Result: outboundResult{
			Success: false,
			Error:   message,
		},
	})
}

func (c *Connector) write(ctx context.Context, conn *websocket.Conn, value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return conn.Write(ctx, websocket.MessageText, payload)
}

func readLine(ctx context.Context, conn *websocket.Conn, value any) error {
	messageType, payload, err := conn.Read(ctx)
	if err != nil {
		return err
	}
	if messageType != websocket.MessageText || len(payload) == 0 || payload[len(payload)-1] != '\n' {
		return errors.New("Hermes relay frame must be newline-delimited text")
	}
	payload = bytes.TrimSpace(payload)
	if len(payload) == 0 || bytes.ContainsRune(payload, '\n') {
		return errors.New("Hermes relay frame must contain one JSON value")
	}
	return json.Unmarshal(payload, value)
}

func validAuthorization(header, gatewayID, secret string, now time.Time) bool {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) || strings.Contains(header[len(prefix):], " ") {
		return false
	}
	raw, err := base64.RawURLEncoding.DecodeString(header[len(prefix):])
	if err != nil {
		return false
	}
	sigSeparator := bytes.LastIndexByte(raw, ':')
	if sigSeparator <= 0 {
		return false
	}
	payload := raw[:sigSeparator]
	providedMAC, err := hex.DecodeString(string(raw[sigSeparator+1:]))
	if err != nil {
		return false
	}
	expSeparator := bytes.LastIndexByte(payload, ':')
	if expSeparator <= 0 || string(payload[:expSeparator]) != gatewayID {
		return false
	}
	expires, err := strconv.ParseInt(string(payload[expSeparator+1:]), 10, 64)
	if err != nil || expires < now.Unix() {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(payload)
	return hmac.Equal(providedMAC, mac.Sum(nil))
}

type helloFrame struct {
	Type     string `json:"type"`
	Platform string `json:"platform"`
	BotID    string `json:"botId"`
}

type descriptorFrame struct {
	Type       string     `json:"type"`
	Descriptor descriptor `json:"descriptor"`
}

type descriptor struct {
	ContractVersion        int      `json:"contract_version"`
	Platform               string   `json:"platform"`
	Label                  string   `json:"label"`
	MaxMessageLength       int      `json:"max_message_length"`
	SupportsDraftStreaming bool     `json:"supports_draft_streaming"`
	SupportsEdit           bool     `json:"supports_edit"`
	SupportsThreads        bool     `json:"supports_threads"`
	MarkdownDialect        string   `json:"markdown_dialect"`
	LengthUnit             string   `json:"len_unit"`
	SupportedOperations    []string `json:"supported_ops"`
}

type inboundFrame struct {
	Type  string `json:"type"`
	Event struct {
		Text        string `json:"text"`
		MessageType string `json:"message_type"`
		MessageID   string `json:"message_id"`
		Source      struct {
			Platform  string  `json:"platform"`
			ChatID    string  `json:"chat_id"`
			ChatType  string  `json:"chat_type"`
			ChatName  string  `json:"chat_name"`
			UserID    string  `json:"user_id"`
			UserName  string  `json:"user_name"`
			ThreadID  *string `json:"thread_id"`
			ChatTopic *string `json:"chat_topic"`
			MessageID string  `json:"message_id"`
		} `json:"source"`
	} `json:"event"`
}

type outboundFrame struct {
	Type      string `json:"type"`
	RequestID string `json:"requestId"`
	Platform  string `json:"platform,omitempty"`
	BotID     string `json:"botId,omitempty"`
	Action    struct {
		Operation string `json:"op"`
		ChatID    string `json:"chat_id"`
		Content   string `json:"content"`
		ReplyTo   string `json:"reply_to"`
	} `json:"action"`
}

type outboundResultFrame struct {
	Type      string         `json:"type"`
	RequestID string         `json:"requestId"`
	Result    outboundResult `json:"result"`
}

type outboundResult struct {
	Success   bool   `json:"success"`
	MessageID string `json:"message_id,omitempty"`
	Error     string `json:"error,omitempty"`
}
