package ui_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tobsai/fort/control"
	"github.com/tobsai/fort/core/messaging"
	"github.com/tobsai/fort/ui"
)

type messagingAPIFake struct {
	peers   []ui.MessagingPeer
	page    ui.MessagingEventPage
	receipt ui.MessagingPostReceipt
	posted  struct {
		conversationID  string
		clientMessageID string
		text            string
	}
}

func (fake *messagingAPIFake) MessagingPeers(context.Context) ([]ui.MessagingPeer, error) {
	return fake.peers, nil
}

func (fake *messagingAPIFake) MessagingChannels(context.Context) ([]ui.MessagingPeer, error) {
	return fake.peers, nil
}

func (fake *messagingAPIFake) MessagingEvents(_ context.Context, conversationID string, after int64) (ui.MessagingEventPage, error) {
	if conversationID != "home-one" || after != 0 {
		panic("messaging API changed the requested Conversation or cursor")
	}
	return fake.page, nil
}

func (fake *messagingAPIFake) PostMessagingMessage(_ context.Context, conversationID, clientMessageID, text string) (ui.MessagingPostReceipt, error) {
	fake.posted.conversationID = conversationID
	fake.posted.clientMessageID = clientMessageID
	fake.posted.text = text
	return fake.receipt, nil
}

func TestMessagingAPISeparatesDynamicHermesChannelsFromHistoricalPeerProof(t *testing.T) {
	now := time.Date(2026, 8, 23, 18, 30, 0, 0, time.UTC)
	message := ui.MessagingMessage{
		ID: "message-one", ConversationID: "home-one", AuthorKind: "peer",
		AuthorID: "hermes-peer-one", Body: "hello from Hermes", CreatedAt: now,
	}
	fake := &messagingAPIFake{
		peers: []ui.MessagingPeer{{
			ID: "hermes-peer-one", SourceID: "messaging-source:macbook", CanonicalProfileID: "default",
			DisplayName: "Lewis", MachineName: "tobiass.macbook.pro.lan",
			ConversationID: "home-one", State: "connected",
		}},
		page: ui.MessagingEventPage{
			ConversationID: "home-one",
			Events:         []ui.MessagingEvent{{Sequence: 1, Message: message}},
			NextAfter:      1,
		},
		receipt: ui.MessagingPostReceipt{Message: ui.MessagingMessage{
			ID: "message-two", ConversationID: "home-one", AuthorKind: "human",
			AuthorID: "fort-user", Body: "hello from Fort", CreatedAt: now,
		}, AcceptedSequence: 2, DeliveryState: ui.MessagingDeliveryPending},
	}
	server := ui.New(ui.Deps{Messaging: fake})
	mux := http.NewServeMux()
	if err := server.RegisterProductMode(mux, ui.ProductMode{
		PrimaryChannels: ui.PrimaryChannelsPreview,
		AgentChannels:   ui.AgentChannelsPrimary,
	}); err != nil {
		t.Fatal(err)
	}

	peersRequest := httptest.NewRequest(http.MethodGet, "/api/messaging/peers", nil)
	peersRequest.RemoteAddr = "127.0.0.1:50000"
	peersRequest.Host = "127.0.0.1:4087"
	peersResponse := httptest.NewRecorder()
	mux.ServeHTTP(peersResponse, peersRequest)
	if peersResponse.Code != http.StatusOK {
		t.Fatalf("GET peers status=%d body=%s", peersResponse.Code, peersResponse.Body.String())
	}
	var peers []ui.MessagingPeer
	if err := json.Unmarshal(peersResponse.Body.Bytes(), &peers); err != nil {
		t.Fatal(err)
	}
	if len(peers) != 0 {
		t.Fatalf("historical peers exposed the dynamic channel roster: %+v", peers)
	}

	channelsRequest := httptest.NewRequest(http.MethodGet, "/api/messaging/channels", nil)
	channelsRequest.RemoteAddr = "127.0.0.1:50000"
	channelsRequest.Host = "127.0.0.1:4087"
	channelsResponse := httptest.NewRecorder()
	mux.ServeHTTP(channelsResponse, channelsRequest)
	if channelsResponse.Code != http.StatusOK {
		t.Fatalf("GET channels status=%d body=%s", channelsResponse.Code, channelsResponse.Body.String())
	}
	var channels []ui.MessagingPeer
	if err := json.Unmarshal(channelsResponse.Body.Bytes(), &channels); err != nil {
		t.Fatal(err)
	}
	if len(channels) != 1 || channels[0].SourceID != "messaging-source:macbook" ||
		channels[0].CanonicalProfileID != "default" || channels[0].DisplayName != "Lewis" {
		t.Fatalf("channels=%+v", channels)
	}

	eventsRequest := httptest.NewRequest(http.MethodGet, "/api/messaging/conversations/home-one/events?after=0", nil)
	eventsRequest.RemoteAddr = "127.0.0.1:50000"
	eventsRequest.Host = "127.0.0.1:4087"
	eventsResponse := httptest.NewRecorder()
	mux.ServeHTTP(eventsResponse, eventsRequest)
	if eventsResponse.Code != http.StatusOK {
		t.Fatalf("GET events status=%d body=%s", eventsResponse.Code, eventsResponse.Body.String())
	}
	var page ui.MessagingEventPage
	if err := json.Unmarshal(eventsResponse.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if page.ConversationID != "home-one" || page.NextAfter != 1 || len(page.Events) != 1 || page.Events[0].Message.Body != "hello from Hermes" {
		t.Fatalf("events=%+v", page)
	}

	postRequest := httptest.NewRequest(
		http.MethodPost,
		"/api/messaging/conversations/home-one/messages",
		strings.NewReader(`{"client_message_id":"ios-message-one","text":"hello from Fort"}`),
	)
	postRequest.RemoteAddr = "127.0.0.1:50000"
	postRequest.Host = "127.0.0.1:4087"
	postRequest.Header.Set("Content-Type", "application/json")
	postResponse := httptest.NewRecorder()
	mux.ServeHTTP(postResponse, postRequest)
	if postResponse.Code != http.StatusAccepted {
		t.Fatalf("POST message status=%d body=%s", postResponse.Code, postResponse.Body.String())
	}
	if fake.posted.conversationID != "home-one" || fake.posted.clientMessageID != "ios-message-one" || fake.posted.text != "hello from Fort" {
		t.Fatalf("posted=%+v", fake.posted)
	}
	var receipt ui.MessagingPostReceipt
	if err := json.Unmarshal(postResponse.Body.Bytes(), &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.AcceptedSequence != 2 || receipt.Message.Body != "hello from Fort" || receipt.DeliveryState != "pending" {
		t.Fatalf("receipt=%+v", receipt)
	}
}

type ambiguousMessagingRelay struct {
	attempts int
}

func (*ambiguousMessagingRelay) Connected() bool { return true }

func (relay *ambiguousMessagingRelay) Send(context.Context, string, string) error {
	relay.attempts++
	return errors.New("relay write outcome is unknown")
}

func TestMessagingAPIKeepsSinglePeerProofOnHistoricalEndpoint(t *testing.T) {
	hub, err := messaging.New(messaging.Config{
		HumanID: "human:toby", PeerID: "peer:hermes:lewis",
		EndpointID: "endpoint:hermes:lewis:1", ConversationID: "conversation:hermes:lewis:home",
	})
	if err != nil {
		t.Fatal(err)
	}
	service, err := control.NewMessagingService(control.MessagingConfig{
		HumanID: "human:toby", PeerID: "peer:hermes:lewis", BotDisplayName: "Lewis",
		MachineName:    "tobiass.macbook.pro.lan",
		ConversationID: "conversation:hermes:lewis:home",
	}, hub, &ambiguousMessagingRelay{})
	if err != nil {
		t.Fatal(err)
	}
	server := ui.New(ui.Deps{Messaging: service})
	mux := http.NewServeMux()
	if err := server.RegisterProductMode(mux, ui.ProductMode{
		PrimaryChannels: ui.PrimaryChannelsPreview,
		AgentChannels:   ui.AgentChannelsPrimary,
	}); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/messaging/peers", nil)
	request.RemoteAddr = "127.0.0.1:50000"
	request.Host = "127.0.0.1:4087"
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("GET historical peer status=%d body=%s", response.Code, response.Body.String())
	}
	var peers []ui.MessagingPeer
	if err := json.Unmarshal(response.Body.Bytes(), &peers); err != nil {
		t.Fatal(err)
	}
	if len(peers) != 1 || peers[0].DisplayName != "Lewis" || peers[0].MachineName != "tobiass.macbook.pro.lan" {
		t.Fatalf("historical peers=%+v, want the configured proof peer", peers)
	}
}

func TestMessagingAPIReturnsAcceptedIdentityForUnknownDeliveryReplay(t *testing.T) {
	hub, err := messaging.New(messaging.Config{
		HumanID: "human:toby", PeerID: "peer:hermes:lewis",
		EndpointID: "endpoint:hermes:lewis:1", ConversationID: "conversation:hermes:lewis:home",
	})
	if err != nil {
		t.Fatal(err)
	}
	relay := &ambiguousMessagingRelay{}
	service, err := control.NewMessagingService(control.MessagingConfig{
		HumanID: "human:toby", PeerID: "peer:hermes:lewis", BotDisplayName: "Lewis",
		MachineName:    "tobiass.macbook.pro.lan",
		ConversationID: "conversation:hermes:lewis:home",
	}, hub, relay)
	if err != nil {
		t.Fatal(err)
	}
	server := ui.New(ui.Deps{Messaging: service})
	mux := http.NewServeMux()
	if err := server.RegisterProductMode(mux, ui.ProductMode{
		PrimaryChannels: ui.PrimaryChannelsPreview,
		AgentChannels:   ui.AgentChannelsPrimary,
	}); err != nil {
		t.Fatal(err)
	}

	post := func() (*httptest.ResponseRecorder, ui.MessagingPostReceipt) {
		request := httptest.NewRequest(
			http.MethodPost,
			"/api/messaging/conversations/conversation:hermes:lewis:home/messages",
			strings.NewReader(`{"client_message_id":"ios-message-unknown","text":"hello Hermes"}`),
		)
		request.RemoteAddr = "127.0.0.1:50000"
		request.Host = "127.0.0.1:4087"
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, request)
		var receipt ui.MessagingPostReceipt
		if response.Code == http.StatusAccepted {
			if err := json.Unmarshal(response.Body.Bytes(), &receipt); err != nil {
				t.Fatal(err)
			}
		}
		return response, receipt
	}

	firstResponse, first := post()
	if firstResponse.Code != http.StatusAccepted {
		t.Fatalf("first POST status=%d body=%s", firstResponse.Code, firstResponse.Body.String())
	}
	replayResponse, replay := post()
	if replayResponse.Code != http.StatusAccepted {
		t.Fatalf("replay POST status=%d body=%s", replayResponse.Code, replayResponse.Body.String())
	}
	if first.Message.ID == "" || replay.Message.ID != first.Message.ID ||
		replay.AcceptedSequence != first.AcceptedSequence {
		t.Fatalf("replay receipt=%+v, want same accepted identity as %+v", replay, first)
	}
	if first.DeliveryState != "unknown" || first.DeliveryCode != control.MessagingDeliveryFailed ||
		replay.DeliveryState != first.DeliveryState || replay.DeliveryCode != first.DeliveryCode {
		t.Fatalf("delivery outcomes first=%+v replay=%+v", first, replay)
	}
	if relay.attempts != 1 {
		t.Fatalf("unknown delivery replay dispatched %d times, want once", relay.attempts)
	}
}
