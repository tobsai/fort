package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/tobsai/fort/ui"
)

func TestHermesMessagingProofTraversesHTTPAndReleasedRelayProtocol(t *testing.T) {
	const (
		gatewayID      = "fort-hermes-gateway-one"
		secret         = "test-shared-secret"
		botID          = "hermes-bot-one"
		conversationID = "conversation:hermes:lewis:home"
	)
	dir := t.TempDir()
	config := fmt.Sprintf(`{
  "gateway_id":%q,
  "shared_secret":%q,
  "binding_id":"hermes-binding-one",
  "canonical_profile_id":"default",
  "bot_id":%q,
  "bot_display_name":"Lewis",
  "conversation_id":%q,
  "human_id":"human:toby",
  "human_name":"Toby",
  "peer_id":"peer:hermes:lewis",
  "endpoint_id":"endpoint:hermes:lewis:1"
}`, gatewayID, secret, botID, conversationID)
	if err := os.WriteFile(filepath.Join(dir, hermesMessagingConfigName), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	products, err := wireHermesMessaging(dir, "tobiass.macbook.pro.lan")
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	fort := ui.New(ui.Deps{Messaging: products.Service})
	if err := fort.RegisterProductMode(mux, ui.ProductMode{
		PrimaryChannels: ui.PrimaryChannelsPreview,
		AgentChannels:   ui.AgentChannelsPrimary,
	}); err != nil {
		t.Fatal(err)
	}
	mux.Handle("/relay", products.LocalRelay)
	server := httptest.NewServer(mux)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/relay"
	hermes, response, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{HTTPHeader: http.Header{
		"Authorization": {"Bearer " + messagingRelayToken(gatewayID, secret, time.Now().Add(5*time.Minute))},
	}})
	if err != nil {
		if response != nil {
			t.Fatalf("dial Hermes relay: %v (HTTP %s)", err, response.Status)
		}
		t.Fatal(err)
	}
	defer hermes.CloseNow()
	writeMessagingRelayFrame(t, ctx, hermes, map[string]any{
		"type": "hello", "platform": "relay", "botId": botID,
	})
	var descriptor map[string]any
	readMessagingRelayFrame(t, ctx, hermes, &descriptor)

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL+"/api/messaging/conversations/"+conversationID+"/messages", strings.NewReader(`{"client_message_id":"ios:proof:one","text":"hello Hermes proof"}`))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	postResponse, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer postResponse.Body.Close()
	if postResponse.StatusCode != http.StatusAccepted {
		t.Fatalf("post status=%s", postResponse.Status)
	}
	var receipt ui.MessagingPostReceipt
	if err := json.NewDecoder(postResponse.Body).Decode(&receipt); err != nil {
		t.Fatal(err)
	}

	var inbound struct {
		Type  string `json:"type"`
		Event struct {
			Text      string `json:"text"`
			MessageID string `json:"message_id"`
			Source    struct {
				ChatID string `json:"chat_id"`
			} `json:"source"`
		} `json:"event"`
	}
	readMessagingRelayFrame(t, ctx, hermes, &inbound)
	if inbound.Type != "inbound" || inbound.Event.Text != "hello Hermes proof" ||
		inbound.Event.MessageID != receipt.Message.ID || inbound.Event.Source.ChatID != conversationID {
		t.Fatalf("Hermes inbound=%+v receipt=%+v", inbound, receipt)
	}

	writeMessagingRelayFrame(t, ctx, hermes, map[string]any{
		"type": "outbound", "requestId": "hermes:proof:reply:one",
		"action": map[string]any{
			"op": "send", "chat_id": conversationID,
			"content": "Hermes proof reply", "reply_to": receipt.Message.ID,
		},
	})
	var result struct {
		Type      string `json:"type"`
		RequestID string `json:"requestId"`
		Result    struct {
			Success   bool   `json:"success"`
			MessageID string `json:"message_id"`
		} `json:"result"`
	}
	readMessagingRelayFrame(t, ctx, hermes, &result)
	if result.Type != "outbound_result" || result.RequestID != "hermes:proof:reply:one" || !result.Result.Success || result.Result.MessageID == "" {
		t.Fatalf("Hermes outbound result=%+v", result)
	}
	writeMessagingRelayFrame(t, ctx, hermes, map[string]any{
		"type": "outbound", "requestId": "hermes:proof:proactive:one",
		"action": map[string]any{
			"op": "send", "chat_id": conversationID,
			"content": "Hermes proactive proof",
		},
	})
	var proactiveResult struct {
		Type      string `json:"type"`
		RequestID string `json:"requestId"`
		Result    struct {
			Success   bool   `json:"success"`
			MessageID string `json:"message_id"`
		} `json:"result"`
	}
	readMessagingRelayFrame(t, ctx, hermes, &proactiveResult)
	if proactiveResult.Type != "outbound_result" || proactiveResult.RequestID != "hermes:proof:proactive:one" ||
		!proactiveResult.Result.Success || proactiveResult.Result.MessageID == "" {
		t.Fatalf("Hermes proactive result=%+v", proactiveResult)
	}

	eventsResponse, err := server.Client().Get(server.URL + "/api/messaging/conversations/" + conversationID + "/events?after=0")
	if err != nil {
		t.Fatal(err)
	}
	defer eventsResponse.Body.Close()
	if eventsResponse.StatusCode != http.StatusOK {
		t.Fatalf("events status=%s", eventsResponse.Status)
	}
	var page ui.MessagingEventPage
	if err := json.NewDecoder(eventsResponse.Body).Decode(&page); err != nil {
		t.Fatal(err)
	}
	if page.NextAfter != 3 || len(page.Events) != 3 ||
		page.Events[0].Message.AuthorKind != "human" || page.Events[1].Message.AuthorKind != "peer" ||
		page.Events[1].Message.Body != "Hermes proof reply" || page.Events[1].Message.InReplyToMessageID != receipt.Message.ID ||
		page.Events[2].Message.AuthorKind != "peer" || page.Events[2].Message.Body != "Hermes proactive proof" ||
		page.Events[2].Message.InReplyToMessageID != "" {
		t.Fatalf("final messaging transcript=%+v", page)
	}
}

func TestHermesMessagingProofAcknowledgesMoreThanObserverBuffer(t *testing.T) {
	const (
		gatewayID      = "fort-hermes-gateway-many"
		secret         = "test-shared-secret"
		botID          = "hermes-bot-many"
		conversationID = "conversation:hermes:lewis:many"
	)
	dir := t.TempDir()
	config := fmt.Sprintf(`{
  "gateway_id":%q,
  "shared_secret":%q,
  "binding_id":"hermes-binding-many",
  "canonical_profile_id":"default",
  "bot_id":%q,
  "bot_display_name":"Lewis",
  "conversation_id":%q,
  "human_id":"human:toby",
  "human_name":"Toby",
  "peer_id":"peer:hermes:lewis",
  "endpoint_id":"endpoint:hermes:lewis:many"
}`, gatewayID, secret, botID, conversationID)
	if err := os.WriteFile(filepath.Join(dir, hermesMessagingConfigName), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	products, err := wireHermesMessaging(dir, "tobiass.macbook.pro.lan")
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	fort := ui.New(ui.Deps{Messaging: products.Service})
	if err := fort.RegisterProductMode(mux, ui.ProductMode{
		PrimaryChannels: ui.PrimaryChannelsPreview,
		AgentChannels:   ui.AgentChannelsPrimary,
	}); err != nil {
		t.Fatal(err)
	}
	mux.Handle("/relay", products.LocalRelay)
	server := httptest.NewServer(mux)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/relay"
	hermes, response, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{HTTPHeader: http.Header{
		"Authorization": {"Bearer " + messagingRelayToken(gatewayID, secret, time.Now().Add(5*time.Minute))},
	}})
	if err != nil {
		if response != nil {
			t.Fatalf("dial Hermes relay: %v (HTTP %s)", err, response.Status)
		}
		t.Fatal(err)
	}
	defer hermes.CloseNow()
	writeMessagingRelayFrame(t, ctx, hermes, map[string]any{
		"type": "hello", "platform": "relay", "botId": botID,
	})
	var descriptor map[string]any
	readMessagingRelayFrame(t, ctx, hermes, &descriptor)

	for index := 1; index <= 9; index++ {
		requestID := fmt.Sprintf("hermes:many:%d", index)
		body := fmt.Sprintf("Hermes message %d", index)
		writeMessagingRelayFrame(t, ctx, hermes, map[string]any{
			"type": "outbound", "requestId": requestID,
			"action": map[string]any{
				"op": "send", "chat_id": conversationID, "content": body,
			},
		})
		var result struct {
			Type      string `json:"type"`
			RequestID string `json:"requestId"`
			Result    struct {
				Success   bool   `json:"success"`
				MessageID string `json:"message_id"`
			} `json:"result"`
		}
		readMessagingRelayFrame(t, ctx, hermes, &result)
		if result.Type != "outbound_result" || result.RequestID != requestID ||
			!result.Result.Success || result.Result.MessageID == "" {
			t.Fatalf("Hermes outbound result %d=%+v", index, result)
		}
	}

	eventsRequest, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		server.URL+"/api/messaging/conversations/"+conversationID+"/events?after=0",
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	eventsResponse, err := server.Client().Do(eventsRequest)
	if err != nil {
		t.Fatal(err)
	}
	defer eventsResponse.Body.Close()
	if eventsResponse.StatusCode != http.StatusOK {
		t.Fatalf("events status=%s", eventsResponse.Status)
	}
	var page ui.MessagingEventPage
	if err := json.NewDecoder(eventsResponse.Body).Decode(&page); err != nil {
		t.Fatal(err)
	}
	if page.NextAfter != 9 || len(page.Events) != 9 {
		t.Fatalf("final messaging transcript=%+v", page)
	}
	for index, event := range page.Events {
		want := fmt.Sprintf("Hermes message %d", index+1)
		if event.Sequence != int64(index+1) || event.Message.AuthorKind != "peer" || event.Message.Body != want {
			t.Fatalf("messaging event %d=%+v, want body %q", index+1, event, want)
		}
	}
}

func messagingRelayToken(gatewayID, secret string, expires time.Time) string {
	payload := fmt.Sprintf("%s:%d", gatewayID, expires.Unix())
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(payload))
	signed := payload + ":" + hex.EncodeToString(mac.Sum(nil))
	return base64.RawURLEncoding.EncodeToString([]byte(signed))
}

func writeMessagingRelayFrame(t *testing.T, ctx context.Context, conn *websocket.Conn, value any) {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	payload = append(payload, '\n')
	if err := conn.Write(ctx, websocket.MessageText, payload); err != nil {
		t.Fatal(err)
	}
}

func readMessagingRelayFrame(t *testing.T, ctx context.Context, conn *websocket.Conn, target any) {
	t.Helper()
	messageType, payload, err := conn.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if messageType != websocket.MessageText || len(payload) == 0 || payload[len(payload)-1] != '\n' {
		t.Fatalf("relay frame type=%v payload=%q", messageType, payload)
	}
	if err := json.Unmarshal(payload, target); err != nil {
		t.Fatal(err)
	}
}
