package control_test

import (
	"context"
	"errors"
	"testing"

	"github.com/tobsai/fort/control"
	"github.com/tobsai/fort/core/messaging"
)

type proofRelay struct {
	connected  bool
	attempts   int
	sends      int
	messageID  string
	text       string
	beforeSend func() error
}

type reconnectingRelay struct {
	current     control.MessagingRelay
	replacement control.MessagingRelay
}

func (relay *reconnectingRelay) SnapshotMessagingRelay() control.MessagingRelay {
	return relay.current
}

func (relay *reconnectingRelay) Connected() bool {
	connected := relay.current.Connected()
	relay.current = relay.replacement
	return connected
}

func (relay *reconnectingRelay) Send(ctx context.Context, messageID, text string) error {
	return relay.current.Send(ctx, messageID, text)
}

func (relay *proofRelay) Connected() bool { return relay.connected }

func (relay *proofRelay) Send(_ context.Context, messageID, text string) error {
	relay.attempts++
	if relay.beforeSend != nil {
		if err := relay.beforeSend(); err != nil {
			return err
		}
	}
	relay.sends++
	relay.messageID = messageID
	relay.text = text
	return nil
}

func TestMessagingServiceRecordsBeforeHermesDispatchAndAcceptsReply(t *testing.T) {
	ctx := context.Background()
	hub, err := messaging.New(messaging.Config{
		HumanID: "human:toby", PeerID: "peer:hermes:lewis",
		EndpointID: "endpoint:hermes:lewis:1", ConversationID: "conversation:hermes:lewis:home",
	})
	if err != nil {
		t.Fatal(err)
	}
	relay := &proofRelay{connected: true}
	relay.beforeSend = func() error {
		page, err := hub.Events(ctx, messaging.Principal{Kind: messaging.PrincipalHuman, ID: "human:toby"}, "conversation:hermes:lewis:home", 0)
		if err != nil {
			return err
		}
		if len(page.Events) != 1 || page.Events[0].Message.Text != "hello Hermes" {
			return errors.New("human message was not accepted before relay dispatch")
		}
		return nil
	}
	service, err := control.NewMessagingService(control.MessagingConfig{
		HumanID: "human:toby", PeerID: "peer:hermes:lewis", BotDisplayName: "Lewis",
		MachineName:    "tobiass.macbook.pro.lan",
		ConversationID: "conversation:hermes:lewis:home",
	}, hub, relay)
	if err != nil {
		t.Fatal(err)
	}

	receipt, err := service.PostMessagingMessage(ctx, "conversation:hermes:lewis:home", "ios:message:one", "hello Hermes")
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Message.Body != "hello Hermes" || receipt.AcceptedSequence != 1 || receipt.DeliveryState != "pending" ||
		receipt.DeliveryCode != "" || relay.sends != 1 || relay.messageID != receipt.Message.ID {
		t.Fatalf("receipt=%+v relay=%+v", receipt, relay)
	}
	relay.connected = false
	replayed, err := service.PostMessagingMessage(ctx, "conversation:hermes:lewis:home", "ios:message:one", "hello Hermes")
	if err != nil {
		t.Fatal(err)
	}
	if replayed.Message.ID != receipt.Message.ID || replayed.AcceptedSequence != receipt.AcceptedSequence ||
		replayed.DeliveryState != receipt.DeliveryState {
		t.Fatalf("offline idempotent replay = %+v, want %+v", replayed, receipt)
	}
	if relay.sends != 1 {
		t.Fatalf("idempotent replay dispatched %d times, want once", relay.sends)
	}
	if fresh, freshErr := service.PostMessagingMessage(
		ctx,
		"conversation:hermes:lewis:home",
		"ios:message:fresh-while-offline",
		"do not accept me offline",
	); freshErr == nil || fresh.Message.ID != "" {
		t.Fatalf("fresh offline post receipt=%+v err=%v, want fail-closed without acceptance", fresh, freshErr)
	}
	relay.connected = true

	messageID, err := service.AcceptPeerMessage(ctx, control.PeerMessage{
		RequestID: "hermes:request:one", ConversationID: "conversation:hermes:lewis:home",
		Text: "hello from Hermes", InReplyToMessageID: receipt.Message.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if messageID == "" {
		t.Fatal("Hermes message did not receive a Fort message ID")
	}
	page, err := service.MessagingEvents(ctx, "conversation:hermes:lewis:home", 0)
	if err != nil {
		t.Fatal(err)
	}
	if page.NextAfter != 2 || len(page.Events) != 2 || page.Events[1].Message.AuthorKind != "peer" || page.Events[1].Message.Body != "hello from Hermes" {
		t.Fatalf("conversation page=%+v", page)
	}
	peers, err := service.MessagingPeers(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(peers) != 1 || peers[0].DisplayName != "Lewis" || peers[0].MachineName != "tobiass.macbook.pro.lan" || peers[0].State != "connected" {
		t.Fatalf("peers=%+v", peers)
	}
}

func TestMessagingServicePinsOneRelayConnectionAcrossAcceptanceAndDispatch(t *testing.T) {
	ctx := context.Background()
	hub, err := messaging.New(messaging.Config{
		HumanID: "human:toby", PeerID: "peer:hermes:lewis",
		EndpointID: "endpoint:hermes:lewis:1", ConversationID: "conversation:hermes:lewis:home",
	})
	if err != nil {
		t.Fatal(err)
	}
	original := &proofRelay{connected: true}
	replacement := &proofRelay{connected: true}
	relay := &reconnectingRelay{current: original, replacement: replacement}
	service, err := control.NewMessagingService(control.MessagingConfig{
		HumanID: "human:toby", PeerID: "peer:hermes:lewis", BotDisplayName: "Lewis",
		MachineName:    "tobiass.macbook.pro.lan",
		ConversationID: "conversation:hermes:lewis:home",
	}, hub, relay)
	if err != nil {
		t.Fatal(err)
	}

	receipt, err := service.PostMessagingMessage(
		ctx,
		"conversation:hermes:lewis:home",
		"ios:message:pinned-connection",
		"stay on the accepted connection",
	)
	if err != nil {
		t.Fatal(err)
	}
	if original.sends != 1 || original.messageID != receipt.Message.ID {
		t.Fatalf("original relay = %+v, want the accepted message", original)
	}
	if replacement.attempts != 0 {
		t.Fatalf("reconnected relay received %d dispatch attempts, want none", replacement.attempts)
	}
}

func TestMessagingServiceReplaysUnknownDeliveryOutcomeWithoutRedispatch(t *testing.T) {
	ctx := context.Background()
	hub, err := messaging.New(messaging.Config{
		HumanID: "human:toby", PeerID: "peer:hermes:lewis",
		EndpointID: "endpoint:hermes:lewis:1", ConversationID: "conversation:hermes:lewis:home",
	})
	if err != nil {
		t.Fatal(err)
	}
	relay := &proofRelay{
		connected: true,
		beforeSend: func() error {
			return errors.New("relay write outcome is unknown")
		},
	}
	service, err := control.NewMessagingService(control.MessagingConfig{
		HumanID: "human:toby", PeerID: "peer:hermes:lewis", BotDisplayName: "Lewis",
		MachineName:    "tobiass.macbook.pro.lan",
		ConversationID: "conversation:hermes:lewis:home",
	}, hub, relay)
	if err != nil {
		t.Fatal(err)
	}

	first, firstErr := service.PostMessagingMessage(
		ctx,
		"conversation:hermes:lewis:home",
		"ios:message:unknown-delivery",
		"hello Hermes",
	)
	if firstErr == nil {
		t.Fatal("first relay failure was reported as successful")
	}
	firstCoded, ok := firstErr.(interface{ MessagingCode() string })
	if !ok || firstCoded.MessagingCode() != control.MessagingDeliveryFailed {
		t.Fatalf("first relay failure = %v, want %s", firstErr, control.MessagingDeliveryFailed)
	}

	replayed, replayErr := service.PostMessagingMessage(
		ctx,
		"conversation:hermes:lewis:home",
		"ios:message:unknown-delivery",
		"hello Hermes",
	)
	if replayErr == nil {
		t.Errorf("idempotent replay reported success; want the same unknown delivery outcome as %v", firstErr)
	} else {
		replayCoded, ok := replayErr.(interface{ MessagingCode() string })
		if !ok || replayCoded.MessagingCode() != firstCoded.MessagingCode() || replayErr.Error() != firstErr.Error() {
			t.Errorf("idempotent replay error = %v, want same outcome as %v", replayErr, firstErr)
		}
	}
	if relay.attempts != 1 {
		t.Errorf("idempotent replay attempted Hermes dispatch %d times, want 1", relay.attempts)
	}
	if replayed.Message.ID != first.Message.ID || replayed.AcceptedSequence != first.AcceptedSequence {
		t.Errorf("idempotent replay receipt = %+v, want accepted receipt %+v", replayed, first)
	}
	if first.DeliveryState != "unknown" || first.DeliveryCode != control.MessagingDeliveryFailed {
		t.Errorf("first delivery outcome = %+v, want explicit unknown/%s", first, control.MessagingDeliveryFailed)
	}
	if replayed.DeliveryState != first.DeliveryState || replayed.DeliveryCode != first.DeliveryCode {
		t.Errorf("idempotent replay delivery outcome = %+v, want %+v", replayed, first)
	}
}
