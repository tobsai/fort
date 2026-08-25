package messaging_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/tobsai/fort/core/messaging"
)

func TestHumanPostReturnsDeliveryAndAppearsInConversationEvents(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	hub, err := messaging.New(messaging.Config{
		HumanID:        "human:toby",
		PeerID:         "peer:hermes:scout",
		EndpointID:     "endpoint:hermes:scout:1",
		ConversationID: "conversation:hermes:scout:home",
	})
	if err != nil {
		t.Fatalf("create messaging Hub: %v", err)
	}

	receipt, err := hub.Post(ctx, messaging.Principal{
		Kind: messaging.PrincipalHuman,
		ID:   "human:toby",
	}, messaging.PostCommand{
		ClientMessageID: "ios-message:one",
		ConversationID:  "conversation:hermes:scout:home",
		Text:            "hello Hermes",
	})
	if err != nil {
		t.Fatalf("post human message: %v", err)
	}
	if receipt.MessageID == "" || receipt.ConversationID != "conversation:hermes:scout:home" ||
		receipt.Sequence != 1 || receipt.AcceptedAt.IsZero() || !receipt.Created {
		t.Fatalf("post receipt = %+v", receipt)
	}
	if receipt.Delivery == nil || receipt.Delivery.ID == "" ||
		receipt.Delivery.MessageID != receipt.MessageID ||
		receipt.Delivery.EndpointID != "endpoint:hermes:scout:1" ||
		receipt.Delivery.State != messaging.DeliveryPending {
		t.Fatalf("delivery receipt = %+v", receipt.Delivery)
	}

	page, err := hub.Events(ctx, messaging.Principal{
		Kind: messaging.PrincipalHuman,
		ID:   "human:toby",
	}, "conversation:hermes:scout:home", 0)
	if err != nil {
		t.Fatalf("read conversation events: %v", err)
	}
	if page.ConversationID != "conversation:hermes:scout:home" || page.AfterSequence != 0 ||
		page.NextSequence != 1 || len(page.Events) != 1 {
		t.Fatalf("event page = %+v", page)
	}
	event := page.Events[0]
	if event.Sequence != 1 || event.Type != messaging.EventMessagePosted ||
		event.Message.ID != receipt.MessageID || event.Message.ClientMessageID != "ios-message:one" ||
		event.Message.Author.Kind != messaging.PrincipalHuman || event.Message.Author.ID != "human:toby" ||
		event.Message.Text != "hello Hermes" || !event.Message.CreatedAt.Equal(receipt.AcceptedAt) {
		t.Fatalf("message event = %+v", event)
	}

	after, err := hub.Events(ctx, messaging.Principal{
		Kind: messaging.PrincipalHuman,
		ID:   "human:toby",
	}, "conversation:hermes:scout:home", receipt.Sequence)
	if err != nil {
		t.Fatalf("read after receipt: %v", err)
	}
	if after.Events == nil || len(after.Events) != 0 || after.NextSequence != receipt.Sequence {
		t.Fatalf("events after receipt = %+v", after)
	}
}

func TestPostsAreOrderedAndClientMessageIDsAreIdempotentPerPrincipal(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	hub, err := messaging.New(messaging.Config{
		HumanID:        "human:toby",
		PeerID:         "peer:hermes:scout",
		EndpointID:     "endpoint:hermes:scout:1",
		ConversationID: "conversation:hermes:scout:home",
	})
	if err != nil {
		t.Fatalf("create messaging Hub: %v", err)
	}
	human := messaging.Principal{Kind: messaging.PrincipalHuman, ID: "human:toby"}
	peer := messaging.Principal{Kind: messaging.PrincipalPeer, ID: "peer:hermes:scout"}
	firstCommand := messaging.PostCommand{
		ClientMessageID: "client-message:shared",
		ConversationID:  "conversation:hermes:scout:home",
		Text:            "hello Hermes",
	}

	first, err := hub.Post(ctx, human, firstCommand)
	if err != nil {
		t.Fatalf("post first message: %v", err)
	}
	replayed, err := hub.Post(ctx, human, firstCommand)
	if err != nil {
		t.Fatalf("replay first message: %v", err)
	}
	if replayed.Created {
		t.Fatalf("idempotent replay was reported as newly created: %+v", replayed)
	}
	replayed.Created = true
	if !reflect.DeepEqual(replayed, first) {
		t.Fatalf("idempotent receipt = %+v, want %+v", replayed, first)
	}
	conflicting := firstCommand
	conflicting.Text = "changed text"
	if _, err := hub.Post(ctx, human, conflicting); !errors.Is(err, messaging.ErrIdempotencyConflict) {
		t.Fatalf("conflicting replay error = %v, want %v", err, messaging.ErrIdempotencyConflict)
	}
	if _, err := hub.Post(ctx, peer, messaging.PostCommand{
		ClientMessageID:  "client-message:missing-reply",
		ConversationID:   "conversation:hermes:scout:home",
		Text:             "reply to unknown message",
		ReplyToMessageID: "message:missing",
	}); !errors.Is(err, messaging.ErrReplyNotFound) {
		t.Fatalf("unknown reply error = %v, want %v", err, messaging.ErrReplyNotFound)
	}

	reply, err := hub.Post(ctx, peer, messaging.PostCommand{
		ClientMessageID:  "client-message:shared",
		ConversationID:   "conversation:hermes:scout:home",
		Text:             "hello from Hermes",
		ReplyToMessageID: first.MessageID,
	})
	if err != nil {
		t.Fatalf("post peer reply: %v", err)
	}
	if reply.Sequence != 2 || reply.Delivery != nil || !reply.Created {
		t.Fatalf("peer reply receipt = %+v", reply)
	}

	page, err := hub.Events(ctx, human, "conversation:hermes:scout:home", 0)
	if err != nil {
		t.Fatalf("read ordered events: %v", err)
	}
	if len(page.Events) != 2 || page.Events[0].Sequence != 1 || page.Events[1].Sequence != 2 ||
		page.Events[0].Message.Author != human || page.Events[1].Message.Author != peer ||
		page.Events[1].Message.ReplyToMessageID != first.MessageID {
		t.Fatalf("ordered events = %+v", page.Events)
	}
}

func TestFreshHubsDoNotReuseExternallyVisibleMessageOrDeliveryIDs(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	config := messaging.Config{
		HumanID:        "human:toby",
		PeerID:         "peer:hermes:scout",
		EndpointID:     "endpoint:hermes:scout:1",
		ConversationID: "conversation:hermes:scout:home",
	}
	human := messaging.Principal{Kind: messaging.PrincipalHuman, ID: "human:toby"}

	beforeRestart, err := messaging.New(config)
	if err != nil {
		t.Fatalf("create Hub before restart: %v", err)
	}
	first, err := beforeRestart.Post(ctx, human, messaging.PostCommand{
		ClientMessageID: "ios-message:before-restart",
		ConversationID:  config.ConversationID,
		Text:            "first message",
	})
	if err != nil {
		t.Fatalf("post before restart: %v", err)
	}

	afterRestart, err := messaging.New(config)
	if err != nil {
		t.Fatalf("create Hub after restart: %v", err)
	}
	second, err := afterRestart.Post(ctx, human, messaging.PostCommand{
		ClientMessageID: "ios-message:after-restart",
		ConversationID:  config.ConversationID,
		Text:            "second message",
	})
	if err != nil {
		t.Fatalf("post after restart: %v", err)
	}

	if first.MessageID == second.MessageID {
		t.Errorf("fresh Hubs reused externally visible MessageID %q", first.MessageID)
	}
	if first.Delivery == nil || second.Delivery == nil {
		t.Fatalf("human delivery receipts = first %+v, second %+v", first.Delivery, second.Delivery)
	}
	if first.Delivery.ID == second.Delivery.ID {
		t.Errorf("fresh Hubs reused externally visible delivery ID %q", first.Delivery.ID)
	}
}

func TestWrongPrincipalOrConversationCannotPostOrReadEvents(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	hub, err := messaging.New(messaging.Config{
		HumanID:        "human:toby",
		PeerID:         "peer:hermes:scout",
		EndpointID:     "endpoint:hermes:scout:1",
		ConversationID: "conversation:hermes:scout:home",
	})
	if err != nil {
		t.Fatalf("create messaging Hub: %v", err)
	}
	exactHuman := messaging.Principal{Kind: messaging.PrincipalHuman, ID: "human:toby"}
	exactConversation := "conversation:hermes:scout:home"

	for _, test := range []struct {
		name         string
		principal    messaging.Principal
		conversation string
		want         error
	}{
		{name: "another human", principal: messaging.Principal{Kind: messaging.PrincipalHuman, ID: "human:other"}, conversation: exactConversation, want: messaging.ErrForbidden},
		{name: "another peer", principal: messaging.Principal{Kind: messaging.PrincipalPeer, ID: "peer:other"}, conversation: exactConversation, want: messaging.ErrForbidden},
		{name: "unknown principal kind", principal: messaging.Principal{Kind: "runtime", ID: "runtime:hermes"}, conversation: exactConversation, want: messaging.ErrForbidden},
		{name: "another Conversation", principal: exactHuman, conversation: "conversation:other", want: messaging.ErrRecipientNotAllowed},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := hub.Post(ctx, test.principal, messaging.PostCommand{
				ClientMessageID: "client:" + test.name,
				ConversationID:  test.conversation,
				Text:            "must not be accepted",
			})
			if !errors.Is(err, test.want) {
				t.Fatalf("Post error = %v, want %v", err, test.want)
			}
			if _, err := hub.Events(ctx, test.principal, test.conversation, 0); !errors.Is(err, test.want) {
				t.Fatalf("Events error = %v, want %v", err, test.want)
			}
		})
	}

	page, err := hub.Events(ctx, exactHuman, exactConversation, 0)
	if err != nil {
		t.Fatalf("read exact Conversation after rejections: %v", err)
	}
	if page.Events == nil || len(page.Events) != 0 || page.NextSequence != 0 {
		t.Fatalf("rejected messages changed events: %+v", page)
	}
}

func TestTextLimitCountsCharactersAndRejectsTheWholeOversizedPost(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	hub, err := messaging.New(messaging.Config{
		HumanID:        "human:toby",
		PeerID:         "peer:hermes:scout",
		EndpointID:     "endpoint:hermes:scout:1",
		ConversationID: "conversation:hermes:scout:home",
	})
	if err != nil {
		t.Fatalf("create messaging Hub: %v", err)
	}
	human := messaging.Principal{Kind: messaging.PrincipalHuman, ID: "human:toby"}

	accepted, err := hub.Post(ctx, human, messaging.PostCommand{
		ClientMessageID: "client-message:at-limit",
		ConversationID:  "conversation:hermes:scout:home",
		Text:            strings.Repeat("🙂", 4096),
	})
	if err != nil {
		t.Fatalf("post 4096-character text: %v", err)
	}
	if accepted.Sequence != 1 || accepted.Delivery == nil || !accepted.Created {
		t.Fatalf("at-limit receipt = %+v", accepted)
	}

	if _, err := hub.Post(ctx, human, messaging.PostCommand{
		ClientMessageID: "client-message:over-limit",
		ConversationID:  "conversation:hermes:scout:home",
		Text:            strings.Repeat("🙂", 4097),
	}); !errors.Is(err, messaging.ErrTextTooLong) {
		t.Fatalf("4097-character Post error = %v, want %v", err, messaging.ErrTextTooLong)
	}
	page, err := hub.Events(ctx, human, "conversation:hermes:scout:home", 0)
	if err != nil {
		t.Fatalf("read events after oversized Post: %v", err)
	}
	if len(page.Events) != 1 || page.NextSequence != 1 {
		t.Fatalf("oversized Post changed events: %+v", page)
	}
}
