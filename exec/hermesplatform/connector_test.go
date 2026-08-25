package hermesplatform_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/tobsai/fort/exec/hermesplatform"
)

func TestDeriveProfileTokenUsesStableProfileScopedContract(t *testing.T) {
	got, err := hermesplatform.DeriveProfileToken("source-profile-key", "research")
	if err != nil {
		t.Fatal(err)
	}
	const want = "HiXJsopGY5ntYXmLJqTMs3vhLjH2-nZf07C7ix2AfMc"
	if got != want {
		t.Fatalf("research profile token = %q, want %q", got, want)
	}
	other, err := hermesplatform.DeriveProfileToken("source-profile-key", "writer")
	if err != nil {
		t.Fatal(err)
	}
	if other == got {
		t.Fatal("distinct canonical profiles derived the same token")
	}
}

func TestDeriveProfileTokenRejectsNonCanonicalIdentity(t *testing.T) {
	for name, input := range map[string]struct {
		rootKey   string
		profileID string
	}{
		"missing root key":  {profileID: "research"},
		"uppercase profile": {rootKey: "source-profile-key", profileID: "Research"},
		"reserved profile":  {rootKey: "source-profile-key", profileID: "root"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := hermesplatform.DeriveProfileToken(input.rootKey, input.profileID); err == nil {
				t.Fatal("non-canonical profile identity derived a token")
			}
		})
	}
}

func TestConnectorRegistersExactHermesProfileAndCarriesMessages(t *testing.T) {
	profileToken, err := hermesplatform.DeriveProfileToken("source-profile-key", "research")
	if err != nil {
		t.Fatal(err)
	}
	var registered hermesplatform.Registration
	var sender hermesplatform.Sender
	var connectedDuringRegistration bool
	var deliveredChannelID string
	var delivered hermesplatform.Message
	connector, err := hermesplatform.New(hermesplatform.Config{
		ProfileTokenKey: "source-profile-key", SenderID: "human:toby", SenderName: "Toby",
		Register: func(_ context.Context, registration hermesplatform.Registration, exactSender hermesplatform.Sender) (hermesplatform.RegistrationReceipt, error) {
			registered = registration
			sender = exactSender
			connectedDuringRegistration = exactSender.Connected()
			return hermesplatform.RegistrationReceipt{
				ChannelID:      "messaging-channel:hermes:v1:research",
				ConversationID: "conversation:messaging:home:v1:research",
			}, nil
		},
		Deliver: func(_ context.Context, channelID string, message hermesplatform.Message) (string, error) {
			deliveredChannelID = channelID
			delivered = message
			return "fort-message:hermes-one", nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(connector.Handler())
	t.Cleanup(server.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	header := http.Header{
		"Authorization":         []string{"Bearer " + profileToken},
		"X-Fort-Hermes-Profile": []string{"research"},
	}
	socket, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/platforms/hermes", &websocket.DialOptions{HTTPHeader: header})
	if err != nil {
		t.Fatal(err)
	}
	defer socket.CloseNow()
	writeJSON(t, ctx, socket, map[string]any{
		"type": "register", "contract_version": 1,
		"profile_id": "research", "display_name": "Ada",
	})
	var registrationReceipt struct {
		Type           string `json:"type"`
		ChannelID      string `json:"channel_id"`
		ConversationID string `json:"conversation_id"`
	}
	readJSON(t, ctx, socket, &registrationReceipt)
	if registrationReceipt.Type != "registered" ||
		registrationReceipt.ChannelID != "messaging-channel:hermes:v1:research" ||
		registrationReceipt.ConversationID != "conversation:messaging:home:v1:research" {
		t.Fatalf("registration receipt = %+v", registrationReceipt)
	}
	if registered.ConnectionID == "" || registered.CanonicalProfileID != "research" || registered.DisplayName != "Ada" {
		t.Fatalf("registration = %+v", registered)
	}
	if connectedDuringRegistration {
		t.Fatal("sender reported connected before the registered acknowledgement")
	}
	if sender == nil || !sender.Connected() {
		t.Fatal("registered exact sender is not connected")
	}

	if err := sender.Send(ctx, "fort-message-one", "hello from Fort"); err != nil {
		t.Fatal(err)
	}
	var inbound struct {
		Type           string `json:"type"`
		RequestID      string `json:"request_id"`
		MessageID      string `json:"message_id"`
		ConversationID string `json:"conversation_id"`
		Text           string `json:"text"`
		AuthorID       string `json:"author_id"`
		AuthorName     string `json:"author_name"`
	}
	readJSON(t, ctx, socket, &inbound)
	if inbound.Type != "inbound" || inbound.RequestID == "" ||
		inbound.MessageID != "fort-message-one" || inbound.ConversationID != registrationReceipt.ConversationID ||
		inbound.Text != "hello from Fort" || inbound.AuthorID != "human:toby" || inbound.AuthorName != "Toby" {
		t.Fatalf("inbound = %+v", inbound)
	}

	writeJSON(t, ctx, socket, map[string]any{
		"type": "outbound", "request_id": "hermes-request-one",
		"conversation_id": registrationReceipt.ConversationID,
		"text":            "hello from Hermes", "in_reply_to_message_id": "fort-message-one",
	})
	var receipt struct {
		Type      string `json:"type"`
		RequestID string `json:"request_id"`
		MessageID string `json:"message_id"`
	}
	readJSON(t, ctx, socket, &receipt)
	if receipt.Type != "receipt" || receipt.RequestID != "hermes-request-one" || receipt.MessageID != "fort-message:hermes-one" {
		t.Fatalf("receipt = %+v", receipt)
	}
	if deliveredChannelID != registrationReceipt.ChannelID || delivered.RequestID != "hermes-request-one" ||
		delivered.ConversationID != registrationReceipt.ConversationID || delivered.Text != "hello from Hermes" ||
		delivered.InReplyToMessageID != "fort-message-one" {
		t.Fatalf("delivered channel=%q message=%+v", deliveredChannelID, delivered)
	}

	if err := socket.Close(websocket.StatusNormalClosure, "done"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for sender.Connected() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if sender.Connected() {
		t.Fatal("sender remained connected after Hermes disconnected")
	}
}

func TestConnectorRejectsWrongBearerTokenBeforeRegistration(t *testing.T) {
	registered := false
	connector, err := hermesplatform.New(hermesplatform.Config{
		ProfileTokenKey: "correct-source-key", SenderID: "human:toby", SenderName: "Toby",
		Register: func(context.Context, hermesplatform.Registration, hermesplatform.Sender) (hermesplatform.RegistrationReceipt, error) {
			registered = true
			return hermesplatform.RegistrationReceipt{}, nil
		},
		Deliver: func(context.Context, string, hermesplatform.Message) (string, error) { return "", nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(connector.Handler())
	t.Cleanup(server.Close)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	header := http.Header{
		"Authorization":         []string{"Bearer wrong-profile-token"},
		"X-Fort-Hermes-Profile": []string{"research"},
	}
	socket, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/platforms/hermes", &websocket.DialOptions{HTTPHeader: header})
	if err == nil {
		defer socket.CloseNow()
		_, _, err = socket.Read(ctx)
	}
	if err == nil || registered {
		t.Fatalf("wrong token result err=%v registered=%t", err, registered)
	}
}

func TestConnectorRejectsRegistrationThatDoesNotMatchAuthenticatedProfile(t *testing.T) {
	profileToken, err := hermesplatform.DeriveProfileToken("source-profile-key", "research")
	if err != nil {
		t.Fatal(err)
	}
	registered := false
	connector, err := hermesplatform.New(hermesplatform.Config{
		ProfileTokenKey: "source-profile-key", SenderID: "human:toby", SenderName: "Toby",
		Register: func(context.Context, hermesplatform.Registration, hermesplatform.Sender) (hermesplatform.RegistrationReceipt, error) {
			registered = true
			return hermesplatform.RegistrationReceipt{}, nil
		},
		Deliver: func(context.Context, string, hermesplatform.Message) (string, error) { return "", nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(connector.Handler())
	t.Cleanup(server.Close)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	header := http.Header{
		"Authorization":         []string{"Bearer " + profileToken},
		"X-Fort-Hermes-Profile": []string{"research"},
	}
	socket, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/platforms/hermes", &websocket.DialOptions{HTTPHeader: header})
	if err != nil {
		t.Fatal(err)
	}
	defer socket.CloseNow()
	writeJSON(t, ctx, socket, map[string]any{
		"type": "register", "contract_version": 1,
		"profile_id": "writer", "display_name": "Pascal",
	})
	_, _, err = socket.Read(ctx)
	if websocket.CloseStatus(err) != websocket.StatusPolicyViolation || registered {
		t.Fatalf("mismatched registration err=%v registered=%t", err, registered)
	}
}

func writeJSON(t *testing.T, ctx context.Context, socket *websocket.Conn, value any) {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := socket.Write(ctx, websocket.MessageText, payload); err != nil {
		t.Fatal(err)
	}
}

func readJSON(t *testing.T, ctx context.Context, socket *websocket.Conn, value any) {
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
