package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/tobsai/fort/core/transporttrust"
	"github.com/tobsai/fort/ui"
)

func TestHermesMessagingWiringKeepsWebSocketLocalAndClientAPIOnNativeRelay(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, hermesMessagingConfigName)
	config := `{
  "gateway_id":"fort-hermes-gateway-one",
  "shared_secret":"test-shared-secret",
  "binding_id":"hermes-binding-one",
  "canonical_profile_id":"default",
  "bot_id":"hermes-bot-one",
  "bot_display_name":"Lewis",
  "conversation_id":"conversation:hermes:lewis:home",
  "human_id":"human:toby",
  "human_name":"Toby",
  "peer_id":"peer:hermes:lewis",
  "endpoint_id":"endpoint:hermes:lewis:1"
}`
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	products, err := wireHermesMessaging(dir, "taloss.mac.mini.lan")
	if err != nil {
		t.Fatal(err)
	}
	if products.Service == nil || products.LocalRelay == nil {
		t.Fatalf("Hermes messaging products=%+v", products)
	}
	peers, err := products.Service.MessagingPeers(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(peers) != 1 || peers[0].DisplayName != "Lewis" || peers[0].MachineName != "taloss.mac.mini.lan" || peers[0].ConversationID != "conversation:hermes:lewis:home" || peers[0].State != "offline" {
		t.Fatalf("peers=%+v", peers)
	}

	nativeMux := http.NewServeMux()
	server := ui.New(ui.Deps{Messaging: products.Service})
	if err := registerNativeProductRoutes(nativeMux, server, ui.ProductMode{
		PrimaryChannels: ui.PrimaryChannelsPreview,
		AgentChannels:   ui.AgentChannelsPrimary,
	}); err != nil {
		t.Fatal(err)
	}
	clientRequest := httptest.NewRequest(http.MethodGet, "/api/messaging/peers", nil)
	clientRequest = clientRequest.WithContext(transporttrust.WithTrusted(clientRequest.Context()))
	clientResponse := httptest.NewRecorder()
	nativeMux.ServeHTTP(clientResponse, clientRequest)
	if clientResponse.Code != http.StatusOK {
		t.Fatalf("native messaging API status=%d body=%s", clientResponse.Code, clientResponse.Body.String())
	}
	var projected []ui.MessagingPeer
	if err := json.NewDecoder(clientResponse.Body).Decode(&projected); err != nil {
		t.Fatal(err)
	}
	if len(projected) != 1 || projected[0].DisplayName != "Lewis" || projected[0].MachineName != "taloss.mac.mini.lan" {
		t.Fatalf("native messaging peer projection=%+v", projected)
	}
	relayRequest := httptest.NewRequest(http.MethodGet, "/relay", nil)
	relayRequest = relayRequest.WithContext(transporttrust.WithTrusted(relayRequest.Context()))
	relayResponse := httptest.NewRecorder()
	nativeMux.ServeHTTP(relayResponse, relayRequest)
	if relayResponse.Code != http.StatusNotFound {
		t.Fatalf("Hermes WebSocket leaked onto native relay: status=%d", relayResponse.Code)
	}
}
