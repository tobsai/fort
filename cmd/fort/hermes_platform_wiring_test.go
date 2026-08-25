package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/tobsai/fort/core/config"
	"github.com/tobsai/fort/exec/hermesplatform"
	"github.com/tobsai/fort/ui"
)

func TestHermesPlatformWiringDiscoversNamedProfileAndCarriesConversation(t *testing.T) {
	dir := t.TempDir()
	platformConfig := `{
  "profile_token_key":"source-profile-key",
  "human_id":"human:toby",
  "human_name":"Toby"
}`
	if err := os.WriteFile(filepath.Join(dir, "hermes-platform.json"), []byte(platformConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := config.SaveRelay(dir, config.RelayConfig{
		GatewayURL: "https://gateway.example", DeviceToken: "device-token",
		MachineID: "machine-macbook", PrivateKey: make([]byte, 32), PublicKey: make([]byte, 32),
	}); err != nil {
		t.Fatal(err)
	}
	products, err := wireHermesMessaging(dir, "tobiass.macbook.pro.lan")
	if err != nil {
		t.Fatal(err)
	}
	if products.Service == nil || products.LocalRelay == nil || products.LocalPath != "/platforms/hermes" {
		t.Fatalf("Hermes platform products = %+v", products)
	}
	mux := http.NewServeMux()
	fort := ui.New(ui.Deps{Messaging: products.Service})
	if err := fort.RegisterProductMode(mux, ui.ProductMode{
		PrimaryChannels: ui.PrimaryChannelsPreview,
		AgentChannels:   ui.AgentChannelsPrimary,
	}); err != nil {
		t.Fatal(err)
	}
	mux.Handle(products.LocalPath, products.LocalRelay)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	profileToken, err := hermesplatform.DeriveProfileToken("source-profile-key", "default")
	if err != nil {
		t.Fatal(err)
	}
	header := http.Header{
		"Authorization":         []string{"Bearer " + profileToken},
		"X-Fort-Hermes-Profile": []string{"default"},
	}
	socket, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+products.LocalPath, &websocket.DialOptions{HTTPHeader: header})
	if err != nil {
		t.Fatal(err)
	}
	defer socket.CloseNow()
	writeHermesPlatformFrame(t, ctx, socket, map[string]any{
		"type": "register", "contract_version": 1,
		"profile_id": "default", "display_name": "Lewis",
	})
	var registered struct {
		Type           string `json:"type"`
		ChannelID      string `json:"channel_id"`
		ConversationID string `json:"conversation_id"`
	}
	readHermesPlatformFrame(t, ctx, socket, &registered)
	if registered.Type != "registered" || registered.ChannelID == "" || registered.ConversationID == "" {
		t.Fatalf("registered = %+v", registered)
	}

	channelsResponse, err := server.Client().Get(server.URL + "/api/messaging/channels")
	if err != nil {
		t.Fatal(err)
	}
	defer channelsResponse.Body.Close()
	if channelsResponse.StatusCode != http.StatusOK {
		t.Fatalf("channels status = %s", channelsResponse.Status)
	}
	var channels []ui.MessagingPeer
	if err := json.NewDecoder(channelsResponse.Body).Decode(&channels); err != nil {
		t.Fatal(err)
	}
	if len(channels) != 1 || channels[0].ID != registered.ChannelID ||
		channels[0].SourceID != "messaging-source:fort-machine:v1:machine-macbook" ||
		channels[0].CanonicalProfileID != "default" || channels[0].DisplayName != "Lewis" ||
		channels[0].MachineName != "tobiass.macbook.pro.lan" || channels[0].State != "connected" {
		t.Fatalf("channels = %+v", channels)
	}

	postRequest, err := http.NewRequestWithContext(ctx, http.MethodPost,
		server.URL+"/api/messaging/conversations/"+registered.ConversationID+"/messages",
		strings.NewReader(`{"client_message_id":"ios:platform:one","text":"hello Hermes"}`))
	if err != nil {
		t.Fatal(err)
	}
	postRequest.Header.Set("Content-Type", "application/json")
	postResponse, err := server.Client().Do(postRequest)
	if err != nil {
		t.Fatal(err)
	}
	defer postResponse.Body.Close()
	if postResponse.StatusCode != http.StatusAccepted {
		t.Fatalf("post status = %s", postResponse.Status)
	}
	var postReceipt ui.MessagingPostReceipt
	if err := json.NewDecoder(postResponse.Body).Decode(&postReceipt); err != nil {
		t.Fatal(err)
	}
	var inbound struct {
		Type           string `json:"type"`
		MessageID      string `json:"message_id"`
		ConversationID string `json:"conversation_id"`
		Text           string `json:"text"`
	}
	readHermesPlatformFrame(t, ctx, socket, &inbound)
	if inbound.Type != "inbound" || inbound.MessageID != postReceipt.Message.ID ||
		inbound.ConversationID != registered.ConversationID || inbound.Text != "hello Hermes" {
		t.Fatalf("inbound = %+v", inbound)
	}

	writeHermesPlatformFrame(t, ctx, socket, map[string]any{
		"type": "outbound", "request_id": "hermes:platform:reply:one",
		"conversation_id": registered.ConversationID, "text": "hello Fort",
		"in_reply_to_message_id": postReceipt.Message.ID,
	})
	var replyReceipt struct {
		Type      string `json:"type"`
		RequestID string `json:"request_id"`
		MessageID string `json:"message_id"`
	}
	readHermesPlatformFrame(t, ctx, socket, &replyReceipt)
	if replyReceipt.Type != "receipt" || replyReceipt.RequestID != "hermes:platform:reply:one" || replyReceipt.MessageID == "" {
		t.Fatalf("reply receipt = %+v", replyReceipt)
	}

	eventsResponse, err := server.Client().Get(server.URL + "/api/messaging/conversations/" + registered.ConversationID + "/events?after=0")
	if err != nil {
		t.Fatal(err)
	}
	defer eventsResponse.Body.Close()
	var page ui.MessagingEventPage
	if eventsResponse.StatusCode != http.StatusOK {
		t.Fatalf("events status = %s", eventsResponse.Status)
	}
	if err := json.NewDecoder(eventsResponse.Body).Decode(&page); err != nil {
		t.Fatal(err)
	}
	if len(page.Events) != 2 || page.Events[0].Message.Body != "hello Hermes" ||
		page.Events[1].Message.Body != "hello Fort" || page.Events[1].Message.AuthorKind != "peer" {
		t.Fatalf("events = %+v", page.Events)
	}
}

func TestHermesPlatformWiringRejectsOperatorProvidedSourceIdentity(t *testing.T) {
	dir := t.TempDir()
	platformConfig := `{
  "profile_token_key":"source-profile-key",
  "source_id":"messaging-source:operator-selected",
  "human_id":"human:toby",
  "human_name":"Toby"
}`
	if err := os.WriteFile(filepath.Join(dir, "hermes-platform.json"), []byte(platformConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := config.SaveRelay(dir, config.RelayConfig{
		GatewayURL: "https://gateway.example", DeviceToken: "device-token",
		MachineID: "machine-macbook", PrivateKey: make([]byte, 32), PublicKey: make([]byte, 32),
	}); err != nil {
		t.Fatal(err)
	}

	_, err := wireHermesMessaging(dir, "tobiass.macbook.pro.lan")
	if err == nil || !strings.Contains(err.Error(), `unknown field "source_id"`) {
		t.Fatalf("operator-provided source identity err = %v", err)
	}
}

func writeHermesPlatformFrame(t *testing.T, ctx context.Context, socket *websocket.Conn, value any) {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := socket.Write(ctx, websocket.MessageText, payload); err != nil {
		t.Fatal(err)
	}
}

func readHermesPlatformFrame(t *testing.T, ctx context.Context, socket *websocket.Conn, value any) {
	t.Helper()
	messageType, payload, err := socket.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if messageType != websocket.MessageText {
		t.Fatalf("message type = %v", messageType)
	}
	if err := json.Unmarshal(payload, value); err != nil {
		t.Fatalf("decode %q: %v", payload, err)
	}
}
