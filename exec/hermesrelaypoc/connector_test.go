package hermesrelaypoc_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/tobsai/fort/exec/hermesrelaypoc"
)

func TestConnectorRequiresFortDeliveryCallback(t *testing.T) {
	_, err := hermesrelaypoc.New(hermesrelaypoc.Config{
		GatewayID:             "hermes-gateway-one",
		SharedSecret:          "test-shared-secret",
		BindingID:             "binding-one",
		CanonicalProfileID:    "profile-one",
		BotID:                 "hermes-bot-one",
		BotDisplayName:        "Scout",
		AllowedConversationID: "conversation-home-one",
		SenderID:              "fort-user-one",
	})
	if err == nil {
		t.Fatal("connector accepted Hermes outbound messages without a Fort delivery callback")
	}
}

func TestConnectorCarriesFortMessageAndHermesReply(t *testing.T) {
	deliveryStarted := make(chan hermesrelaypoc.Message, 1)
	finishDelivery := make(chan struct{})
	cfg := hermesrelaypoc.Config{
		GatewayID:             "hermes-gateway-one",
		SharedSecret:          "test-shared-secret",
		BindingID:             "binding-one",
		CanonicalProfileID:    "profile-one",
		BotID:                 "hermes-bot-one",
		BotDisplayName:        "Scout",
		AllowedConversationID: "conversation-home-one",
		SenderID:              "fort-user-one",
		SenderName:            "Toby",
		ObserveAccepted:       true,
		Deliver: func(ctx context.Context, message hermesrelaypoc.Message) (string, error) {
			select {
			case deliveryStarted <- message:
			case <-ctx.Done():
				return "", ctx.Err()
			}
			select {
			case <-finishDelivery:
				return "fort-persisted-message-one", nil
			case <-ctx.Done():
				return "", ctx.Err()
			}
		},
	}
	connector, err := hermesrelaypoc.New(cfg)
	if err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(connector.Handler())
	t.Cleanup(server.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	hermes := dialHermes(t, ctx, server.URL, cfg.GatewayID, cfg.SharedSecret)
	defer hermes.CloseNow()

	writeLine(t, ctx, hermes, map[string]any{
		"type":     "hello",
		"platform": "relay",
		"botId":    cfg.BotID,
	})

	var descriptor struct {
		Type       string `json:"type"`
		Descriptor struct {
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
		} `json:"descriptor"`
	}
	readLine(t, ctx, hermes, &descriptor)
	if descriptor.Type != "descriptor" || descriptor.Descriptor.ContractVersion != 1 {
		t.Fatalf("unexpected descriptor: %+v", descriptor)
	}
	if descriptor.Descriptor.Platform != "relay" || descriptor.Descriptor.Label != "Fort" {
		t.Fatalf("unexpected relay identity: %+v", descriptor.Descriptor)
	}
	if descriptor.Descriptor.MaxMessageLength != 4096 ||
		descriptor.Descriptor.SupportsDraftStreaming ||
		descriptor.Descriptor.SupportsEdit ||
		descriptor.Descriptor.SupportsThreads ||
		descriptor.Descriptor.MarkdownDialect != "plain" ||
		descriptor.Descriptor.LengthUnit != "chars" ||
		fmt.Sprint(descriptor.Descriptor.SupportedOperations) != "[send]" {
		t.Fatalf("descriptor advertised unsupported behavior: %+v", descriptor.Descriptor)
	}

	bot := connector.Bot()
	if !bot.Connected {
		t.Fatal("bot did not become connected after the relay handshake")
	}
	if bot.BindingID != cfg.BindingID || bot.CanonicalProfileID != cfg.CanonicalProfileID {
		t.Fatalf("connector changed the immutable binding: %+v", bot)
	}
	if bot.Label != "Hermes & Scout" {
		t.Fatalf("bot label = %q, want %q", bot.Label, "Hermes & Scout")
	}

	if err := connector.Send(ctx, "fort-message-one", "hello from Fort"); err != nil {
		t.Fatal(err)
	}
	var inbound struct {
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
	readLine(t, ctx, hermes, &inbound)
	if inbound.Type != "inbound" || inbound.Event.Text != "hello from Fort" ||
		inbound.Event.MessageType != "text" || inbound.Event.MessageID != "fort-message-one" {
		t.Fatalf("unexpected inbound message: %+v", inbound)
	}
	if inbound.Event.Source.Platform != "relay" ||
		inbound.Event.Source.ChatID != cfg.AllowedConversationID ||
		inbound.Event.Source.ChatType != "dm" ||
		inbound.Event.Source.ChatName != "Home" ||
		inbound.Event.Source.UserID != cfg.SenderID ||
		inbound.Event.Source.UserName != cfg.SenderName ||
		inbound.Event.Source.ThreadID != nil ||
		inbound.Event.Source.ChatTopic != nil ||
		inbound.Event.Source.MessageID != "fort-message-one" {
		t.Fatalf("unexpected inbound source: %+v", inbound.Event.Source)
	}

	writeLine(t, ctx, hermes, map[string]any{
		"type":      "outbound",
		"requestId": "hermes-request-one",
		"action": map[string]any{
			"op":       "send",
			"chat_id":  cfg.AllowedConversationID,
			"content":  "hello from Hermes",
			"reply_to": "fort-message-one",
		},
	})
	type wireRead struct {
		messageType websocket.MessageType
		payload     []byte
		err         error
	}
	resultRead := make(chan wireRead, 1)
	go func() {
		messageType, payload, err := hermes.Read(ctx)
		resultRead <- wireRead{messageType: messageType, payload: payload, err: err}
	}()

	var delivered hermesrelaypoc.Message
	select {
	case delivered = <-deliveryStarted:
	case observed := <-resultRead:
		t.Fatalf("Hermes was acknowledged before Fort delivery started: %+v", observed)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Fort delivery callback was not called")
	}
	if delivered.ID != "" || delivered.RequestID != "hermes-request-one" ||
		delivered.ConversationID != cfg.AllowedConversationID ||
		delivered.Text != "hello from Hermes" ||
		delivered.InReplyToMessageID != "fort-message-one" {
		t.Fatalf("unexpected Fort delivery: %+v", delivered)
	}
	select {
	case observed := <-resultRead:
		t.Fatalf("Hermes was acknowledged before Fort delivery completed: %+v", observed)
	case <-time.After(100 * time.Millisecond):
	}
	close(finishDelivery)

	var result struct {
		Type      string `json:"type"`
		RequestID string `json:"requestId"`
		Result    struct {
			Success   bool   `json:"success"`
			MessageID string `json:"message_id"`
			Error     string `json:"error"`
		} `json:"result"`
	}
	var observed wireRead
	select {
	case observed = <-resultRead:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if observed.err != nil {
		t.Fatal(observed.err)
	}
	if observed.messageType != websocket.MessageText ||
		len(observed.payload) == 0 || observed.payload[len(observed.payload)-1] != '\n' {
		t.Fatalf("relay frame is not newline-delimited text: type=%v payload=%q", observed.messageType, observed.payload)
	}
	if err := json.Unmarshal(observed.payload, &result); err != nil {
		t.Fatalf("decode relay frame %q: %v", observed.payload, err)
	}
	if result.Type != "outbound_result" || result.RequestID != "hermes-request-one" ||
		!result.Result.Success || result.Result.MessageID != "fort-persisted-message-one" || result.Result.Error != "" {
		t.Fatalf("unexpected outbound result: %+v", result)
	}

	reply, err := connector.Receive(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if reply.ID != "fort-persisted-message-one" ||
		reply.ConversationID != cfg.AllowedConversationID ||
		reply.Text != "hello from Hermes" ||
		reply.InReplyToMessageID != "fort-message-one" {
		t.Fatalf("unexpected Fort reply observation: %+v", reply)
	}
}

func TestProactiveHermesMessageOnlyReachesAllowedConversation(t *testing.T) {
	cfg := hermesrelaypoc.Config{
		GatewayID:             "hermes-gateway-one",
		SharedSecret:          "test-shared-secret",
		BindingID:             "binding-one",
		CanonicalProfileID:    "profile-one",
		BotID:                 "hermes-bot-one",
		BotDisplayName:        "Scout",
		AllowedConversationID: "conversation-home-one",
		SenderID:              "fort-user-one",
		SenderName:            "Toby",
		ObserveAccepted:       true,
		Deliver: func(context.Context, hermesrelaypoc.Message) (string, error) {
			return "fort-persisted-proactive-one", nil
		},
	}
	connector, err := hermesrelaypoc.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(connector.Handler())
	t.Cleanup(server.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	hermes := dialHermes(t, ctx, server.URL, cfg.GatewayID, cfg.SharedSecret)
	defer hermes.CloseNow()
	writeLine(t, ctx, hermes, map[string]any{
		"type":     "hello",
		"platform": "relay",
		"botId":    cfg.BotID,
	})
	var descriptor map[string]any
	readLine(t, ctx, hermes, &descriptor)

	writeLine(t, ctx, hermes, map[string]any{
		"type":      "outbound",
		"requestId": "hermes-proactive-one",
		"action": map[string]any{
			"op":      "send",
			"chat_id": cfg.AllowedConversationID,
			"content": "scheduled hello from Hermes",
		},
	})
	proactive, err := connector.Receive(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if proactive.ConversationID != cfg.AllowedConversationID ||
		proactive.Text != "scheduled hello from Hermes" ||
		proactive.InReplyToMessageID != "" {
		t.Fatalf("unexpected proactive message: %+v", proactive)
	}
	var allowedResult struct {
		Type      string `json:"type"`
		RequestID string `json:"requestId"`
		Result    struct {
			Success   bool   `json:"success"`
			MessageID string `json:"message_id"`
			Error     string `json:"error"`
		} `json:"result"`
	}
	readLine(t, ctx, hermes, &allowedResult)
	if !allowedResult.Result.Success || allowedResult.Result.MessageID != proactive.ID {
		t.Fatalf("allowed proactive message was not acknowledged: %+v", allowedResult)
	}

	writeLine(t, ctx, hermes, map[string]any{
		"type":      "outbound",
		"requestId": "hermes-proactive-denied",
		"action": map[string]any{
			"op":      "send",
			"chat_id": "some-other-conversation",
			"content": "must not be delivered",
		},
	})
	var deniedResult struct {
		Type      string `json:"type"`
		RequestID string `json:"requestId"`
		Result    struct {
			Success bool   `json:"success"`
			Error   string `json:"error"`
		} `json:"result"`
	}
	readLine(t, ctx, hermes, &deniedResult)
	if deniedResult.Type != "outbound_result" ||
		deniedResult.RequestID != "hermes-proactive-denied" ||
		deniedResult.Result.Success || deniedResult.Result.Error == "" {
		t.Fatalf("disallowed recipient was not rejected: %+v", deniedResult)
	}

	noDeliveryCtx, noDeliveryCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer noDeliveryCancel()
	if unexpected, err := connector.Receive(noDeliveryCtx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("disallowed message was delivered: message=%+v err=%v", unexpected, err)
	}
}

func TestConnectorDoesNotAcknowledgeOrEnqueueFailedFortDelivery(t *testing.T) {
	cfg := hermesrelaypoc.Config{
		GatewayID:             "hermes-gateway-one",
		SharedSecret:          "test-shared-secret",
		BindingID:             "binding-one",
		CanonicalProfileID:    "profile-one",
		BotID:                 "hermes-bot-one",
		BotDisplayName:        "Scout",
		AllowedConversationID: "conversation-home-one",
		SenderID:              "fort-user-one",
		ObserveAccepted:       true,
		Deliver: func(context.Context, hermesrelaypoc.Message) (string, error) {
			return "", errors.New("private store failure detail")
		},
	}
	connector, err := hermesrelaypoc.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(connector.Handler())
	t.Cleanup(server.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 750*time.Millisecond)
	defer cancel()
	hermes := dialHermes(t, ctx, server.URL, cfg.GatewayID, cfg.SharedSecret)
	defer hermes.CloseNow()
	writeLine(t, ctx, hermes, map[string]any{
		"type":     "hello",
		"platform": "relay",
		"botId":    cfg.BotID,
	})
	var descriptor map[string]any
	readLine(t, ctx, hermes, &descriptor)

	writeLine(t, ctx, hermes, map[string]any{
		"type":      "outbound",
		"requestId": "hermes-delivery-failed",
		"action": map[string]any{
			"op":      "send",
			"chat_id": cfg.AllowedConversationID,
			"content": "must not be acknowledged",
		},
	})
	var result struct {
		Type      string `json:"type"`
		RequestID string `json:"requestId"`
		Result    struct {
			Success   bool   `json:"success"`
			MessageID string `json:"message_id"`
			Error     string `json:"error"`
		} `json:"result"`
	}
	readLine(t, ctx, hermes, &result)
	if result.Type != "outbound_result" || result.RequestID != "hermes-delivery-failed" ||
		result.Result.Success || result.Result.MessageID != "" || result.Result.Error != "Fort delivery failed" {
		t.Fatalf("unexpected failed delivery result: %+v", result)
	}

	noDeliveryCtx, noDeliveryCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer noDeliveryCancel()
	if unexpected, err := connector.Receive(noDeliveryCtx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("failed Fort delivery was enqueued: message=%+v err=%v", unexpected, err)
	}
}

func TestConnectorRejectsTheWrongGatewaySecret(t *testing.T) {
	cfg := hermesrelaypoc.Config{
		GatewayID:             "hermes-gateway-one",
		SharedSecret:          "correct-shared-secret",
		BindingID:             "binding-one",
		CanonicalProfileID:    "profile-one",
		BotID:                 "hermes-bot-one",
		BotDisplayName:        "Scout",
		AllowedConversationID: "conversation-home-one",
		SenderID:              "fort-user-one",
		Deliver: func(context.Context, hermesrelaypoc.Message) (string, error) {
			return "unused", nil
		},
	}
	connector, err := hermesrelaypoc.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(connector.Handler())
	t.Cleanup(server.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/relay"
	conn, response, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{HTTPHeader: http.Header{
		"Authorization": {"Bearer " + relayToken(cfg.GatewayID, "wrong-shared-secret", time.Now().Add(5*time.Minute))},
	}})
	if err != nil {
		status := "no response"
		if response != nil {
			status = response.Status
		}
		t.Fatalf("connector did not use the Hermes 4401 authentication close: HTTP=%s err=%v", status, err)
	}
	defer conn.CloseNow()
	_, _, err = conn.Read(ctx)
	if websocket.CloseStatus(err) != websocket.StatusCode(4401) {
		t.Fatalf("wrong gateway secret close = %v, want 4401 (err=%v)", websocket.CloseStatus(err), err)
	}
	if connector.Bot().Connected {
		t.Fatal("unauthenticated Hermes socket changed connector readiness")
	}
}

func dialHermes(t *testing.T, ctx context.Context, serverURL, gatewayID, secret string) *websocket.Conn {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(serverURL, "http") + "/relay"
	conn, response, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{HTTPHeader: http.Header{
		"Authorization": {"Bearer " + relayToken(gatewayID, secret, time.Now().Add(5*time.Minute))},
	}})
	if err != nil {
		if response != nil {
			t.Fatalf("dial Hermes relay: %v (HTTP %s)", err, response.Status)
		}
		t.Fatalf("dial Hermes relay: %v", err)
	}
	return conn
}

func relayToken(gatewayID, secret string, expires time.Time) string {
	payload := fmt.Sprintf("%s:%d", gatewayID, expires.Unix())
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(payload))
	signed := payload + ":" + hex.EncodeToString(mac.Sum(nil))
	return base64.RawURLEncoding.EncodeToString([]byte(signed))
}

func writeLine(t *testing.T, ctx context.Context, conn *websocket.Conn, value any) {
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

func readLine(t *testing.T, ctx context.Context, conn *websocket.Conn, value any) {
	t.Helper()
	messageType, payload, err := conn.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if messageType != websocket.MessageText || len(payload) == 0 || payload[len(payload)-1] != '\n' {
		t.Fatalf("relay frame is not newline-delimited text: type=%v payload=%q", messageType, payload)
	}
	if err := json.Unmarshal(payload, value); err != nil {
		t.Fatalf("decode relay frame %q: %v", payload, err)
	}
}
