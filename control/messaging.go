package control

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/tobsai/fort/core/messaging"
	"github.com/tobsai/fort/ui"
)

const (
	MessagingRelayNotConnected = "hermes_relay_not_connected"
	MessagingDeliveryFailed    = "hermes_relay_delivery_failed"
	MessagingRecipientDenied   = "hermes_recipient_denied"
	MessagingPostConflict      = "messaging_post_conflict"
)

// MessagingRelay is the true-external seam for the Spec 052 proof. It says
// only whether the exact Hermes socket is present and writes one accepted Fort
// message; it cannot invoke a Runtime or select another provider.
type MessagingRelay interface {
	Connected() bool
	Send(context.Context, string, string) error
}

// MessagingRelaySnapshotter is an optional connection-pinning seam for a
// replaceable relay slot. A post uses one returned relay for both readiness
// and dispatch so a reconnect cannot retarget an already accepted message.
type MessagingRelaySnapshotter interface {
	SnapshotMessagingRelay() MessagingRelay
}

type MessagingConfig struct {
	HumanID        string
	PeerID         string
	BotDisplayName string
	MachineName    string
	ConversationID string
}

type PeerMessage struct {
	RequestID          string
	ConversationID     string
	Text               string
	InReplyToMessageID string
}

type MessagingService struct {
	config MessagingConfig
	hub    messaging.Hub
	relay  MessagingRelay

	postMu       sync.Mutex
	postOutcomes map[messagingPostIdentity]messagingPostOutcome
}

type messagingPostIdentity struct {
	conversationID  string
	clientMessageID string
}

type messagingPostOutcome struct {
	text    string
	receipt ui.MessagingPostReceipt
	err     error
}

var errMessagingDeliveryOutcomeUnknown = errors.New("Hermes delivery outcome is unknown")

func NewMessagingService(config MessagingConfig, hub messaging.Hub, relay MessagingRelay) (*MessagingService, error) {
	for label, value := range map[string]string{
		"human id": config.HumanID, "peer id": config.PeerID,
		"bot display name": config.BotDisplayName, "machine name": config.MachineName,
		"Conversation id": config.ConversationID,
	} {
		if strings.TrimSpace(value) == "" || strings.TrimSpace(value) != value {
			return nil, fmt.Errorf("messaging proof %s is required and canonical", label)
		}
	}
	if hub == nil || relay == nil {
		return nil, errors.New("messaging proof requires its Hub and exact Hermes relay")
	}
	if _, err := hub.Events(context.Background(), messaging.Principal{
		Kind: messaging.PrincipalHuman, ID: config.HumanID,
	}, config.ConversationID, 0); err != nil {
		return nil, fmt.Errorf("messaging proof identity does not match its Hub: %w", err)
	}
	return &MessagingService{
		config:       config,
		hub:          hub,
		relay:        relay,
		postOutcomes: make(map[messagingPostIdentity]messagingPostOutcome),
	}, nil
}

func (service *MessagingService) MessagingPeers(context.Context) ([]ui.MessagingPeer, error) {
	state := "offline"
	if service.relay.Connected() {
		state = "connected"
	}
	return []ui.MessagingPeer{{
		ID: service.config.PeerID, DisplayName: service.config.BotDisplayName,
		MachineName:    service.config.MachineName,
		ConversationID: service.config.ConversationID, State: state,
	}}, nil
}

func (service *MessagingService) MessagingEvents(ctx context.Context, conversationID string, after int64) (ui.MessagingEventPage, error) {
	page, err := service.hub.Events(ctx, messaging.Principal{
		Kind: messaging.PrincipalHuman, ID: service.config.HumanID,
	}, conversationID, after)
	if err != nil {
		return ui.MessagingEventPage{}, messagingServiceError(err)
	}
	events := make([]ui.MessagingEvent, 0, len(page.Events))
	for _, event := range page.Events {
		events = append(events, ui.MessagingEvent{Sequence: event.Sequence, Message: projectMessagingMessage(event.Message)})
	}
	return ui.MessagingEventPage{
		ConversationID: page.ConversationID, Events: events, NextAfter: page.NextSequence,
	}, nil
}

func (service *MessagingService) PostMessagingMessage(ctx context.Context, conversationID, clientMessageID, text string) (ui.MessagingPostReceipt, error) {
	service.postMu.Lock()
	defer service.postMu.Unlock()

	identity := messagingPostIdentity{
		conversationID:  conversationID,
		clientMessageID: clientMessageID,
	}
	if previous, ok := service.postOutcomes[identity]; ok {
		if previous.text != text {
			return ui.MessagingPostReceipt{}, codedMessagingError{
				code: MessagingPostConflict,
				err:  messaging.ErrIdempotencyConflict,
			}
		}
		return previous.receipt, previous.err
	}
	relay := service.relay
	if snapshotter, ok := relay.(MessagingRelaySnapshotter); ok {
		relay = snapshotter.SnapshotMessagingRelay()
	}
	if relay == nil || !relay.Connected() {
		return ui.MessagingPostReceipt{}, codedMessagingError{code: MessagingRelayNotConnected, err: errors.New("Hermes is offline")}
	}
	command := messaging.PostCommand{
		ClientMessageID: clientMessageID, ConversationID: conversationID, Text: text,
	}
	receipt, err := service.hub.Post(ctx, messaging.Principal{
		Kind: messaging.PrincipalHuman, ID: service.config.HumanID,
	}, command)
	if err != nil {
		return ui.MessagingPostReceipt{}, messagingServiceError(err)
	}
	projected := ui.MessagingPostReceipt{
		Message: ui.MessagingMessage{
			ID: receipt.MessageID, ConversationID: receipt.ConversationID,
			AuthorKind: string(messaging.PrincipalHuman), AuthorID: service.config.HumanID,
			Body: text, CreatedAt: receipt.AcceptedAt,
		},
		AcceptedSequence: receipt.Sequence,
		DeliveryState:    ui.MessagingDeliveryPending,
	}
	if receipt.Created {
		if err := relay.Send(ctx, receipt.MessageID, text); err != nil {
			projected.DeliveryState = ui.MessagingDeliveryUnknown
			projected.DeliveryCode = MessagingDeliveryFailed
			outcomeErr := codedMessagingError{code: MessagingDeliveryFailed, err: errMessagingDeliveryOutcomeUnknown}
			service.postOutcomes[identity] = messagingPostOutcome{
				text: text, receipt: projected, err: outcomeErr,
			}
			return projected, outcomeErr
		}
	}
	service.postOutcomes[identity] = messagingPostOutcome{text: text, receipt: projected}
	return projected, nil
}

// AcceptPeerMessage is the inbound side of the concrete Hermes relay. A
// successful return is the Fort acceptance acknowledgement used on the wire.
func (service *MessagingService) AcceptPeerMessage(ctx context.Context, incoming PeerMessage) (string, error) {
	if strings.TrimSpace(incoming.RequestID) == "" {
		return "", codedMessagingError{code: MessagingPostConflict, err: errors.New("Hermes request id is required")}
	}
	receipt, err := service.hub.Post(ctx, messaging.Principal{
		Kind: messaging.PrincipalPeer, ID: service.config.PeerID,
	}, messaging.PostCommand{
		ClientMessageID:  "hermes:" + incoming.RequestID,
		ConversationID:   incoming.ConversationID,
		Text:             incoming.Text,
		ReplyToMessageID: incoming.InReplyToMessageID,
	})
	if err != nil {
		return "", messagingServiceError(err)
	}
	return receipt.MessageID, nil
}

func projectMessagingMessage(message messaging.Message) ui.MessagingMessage {
	return ui.MessagingMessage{
		ID: message.ID, ConversationID: message.ConversationID,
		AuthorKind: string(message.Author.Kind), AuthorID: message.Author.ID,
		Body: message.Text, InReplyToMessageID: message.ReplyToMessageID,
		CreatedAt: message.CreatedAt,
	}
}

type codedMessagingError struct {
	code string
	err  error
}

func (err codedMessagingError) Error() string         { return err.err.Error() }
func (err codedMessagingError) Unwrap() error         { return err.err }
func (err codedMessagingError) MessagingCode() string { return err.code }

func messagingServiceError(err error) error {
	switch {
	case errors.Is(err, messaging.ErrRecipientNotAllowed), errors.Is(err, messaging.ErrForbidden):
		return codedMessagingError{code: MessagingRecipientDenied, err: err}
	case errors.Is(err, messaging.ErrIdempotencyConflict), errors.Is(err, messaging.ErrInvalidPost),
		errors.Is(err, messaging.ErrTextTooLong), errors.Is(err, messaging.ErrReplyNotFound),
		errors.Is(err, messaging.ErrInvalidCursor):
		return codedMessagingError{code: MessagingPostConflict, err: err}
	default:
		return codedMessagingError{code: MessagingDeliveryFailed, err: err}
	}
}

var _ ui.MessagingPort = (*MessagingService)(nil)
