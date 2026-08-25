// Package messaging defines the process-local messaging proof module from
// Spec 052. It models one authenticated human, one external peer, and one Home
// Conversation without introducing execution targets or runtime contracts.
package messaging

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const MaxTextCharacters = 4096

type PrincipalKind string

const (
	PrincipalHuman PrincipalKind = "human"
	PrincipalPeer  PrincipalKind = "peer"
)

type DeliveryState string

const DeliveryPending DeliveryState = "pending"

type EventType string

const EventMessagePosted EventType = "message_posted"

var (
	ErrInvalidConfig       = errors.New("messaging proof configuration is invalid")
	ErrInvalidPost         = errors.New("messaging post is invalid")
	ErrForbidden           = errors.New("messaging principal is forbidden")
	ErrRecipientNotAllowed = errors.New("messaging recipient is not allowed")
	ErrIdempotencyConflict = errors.New("messaging client message id conflicts with an earlier post")
	ErrTextTooLong         = errors.New("messaging text exceeds 4096 characters")
	ErrReplyNotFound       = errors.New("messaging reply target was not found in the Conversation")
	ErrInvalidCursor       = errors.New("messaging event cursor is invalid")
)

// Config fixes the complete identity of the process-local proof. None of its
// identifiers may be selected or substituted by a Post caller.
type Config struct {
	HumanID        string
	PeerID         string
	EndpointID     string
	ConversationID string
}

type Principal struct {
	Kind PrincipalKind `json:"kind"`
	ID   string        `json:"id"`
}

type PostCommand struct {
	ClientMessageID  string `json:"client_message_id"`
	ConversationID   string `json:"conversation_id"`
	Text             string `json:"text"`
	ReplyToMessageID string `json:"reply_to_message_id,omitempty"`
}

type DeliveryReceipt struct {
	ID         string        `json:"id"`
	MessageID  string        `json:"message_id"`
	EndpointID string        `json:"endpoint_id"`
	State      DeliveryState `json:"state"`
}

type PostReceipt struct {
	MessageID      string           `json:"message_id"`
	ConversationID string           `json:"conversation_id"`
	Sequence       int64            `json:"sequence"`
	AcceptedAt     time.Time        `json:"accepted_at"`
	Delivery       *DeliveryReceipt `json:"delivery,omitempty"`
	Created        bool             `json:"created"`
}

type Message struct {
	ID               string    `json:"id"`
	ConversationID   string    `json:"conversation_id"`
	ClientMessageID  string    `json:"client_message_id"`
	Author           Principal `json:"author"`
	Text             string    `json:"text"`
	ReplyToMessageID string    `json:"reply_to_message_id,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

type Event struct {
	Sequence int64     `json:"sequence"`
	Type     EventType `json:"type"`
	Message  Message   `json:"message"`
}

type EventPage struct {
	ConversationID string  `json:"conversation_id"`
	AfterSequence  int64   `json:"after_sequence"`
	NextSequence   int64   `json:"next_sequence"`
	Events         []Event `json:"events"`
}

// Hub is the complete public interface for the process-local proof. Tests and
// callers observe accepted messages only through Post and Events.
type Hub interface {
	Post(context.Context, Principal, PostCommand) (PostReceipt, error)
	Events(context.Context, Principal, string, int64) (EventPage, error)
}

type localHub struct {
	mu       sync.RWMutex
	config   Config
	events   []Event
	accepted map[idempotencyKey]acceptedPost
}

type idempotencyKey struct {
	principalKind PrincipalKind
	principalID   string
	conversation  string
	clientMessage string
}

type acceptedPost struct {
	command PostCommand
	receipt PostReceipt
}

func New(config Config) (Hub, error) {
	for label, value := range map[string]string{
		"human id":        config.HumanID,
		"peer id":         config.PeerID,
		"endpoint id":     config.EndpointID,
		"Conversation id": config.ConversationID,
	} {
		if strings.TrimSpace(value) == "" || strings.TrimSpace(value) != value {
			return nil, fmt.Errorf("%w: %s is required and must be canonical", ErrInvalidConfig, label)
		}
	}
	return &localHub{
		config: config, events: make([]Event, 0),
		accepted: make(map[idempotencyKey]acceptedPost),
	}, nil
}

func (hub *localHub) Post(ctx context.Context, principal Principal, command PostCommand) (PostReceipt, error) {
	if err := ctx.Err(); err != nil {
		return PostReceipt{}, err
	}
	if !hub.validPrincipal(principal) {
		return PostReceipt{}, ErrForbidden
	}
	if command.ConversationID != hub.config.ConversationID {
		return PostReceipt{}, ErrRecipientNotAllowed
	}
	if strings.TrimSpace(command.ClientMessageID) == "" || strings.TrimSpace(command.ClientMessageID) != command.ClientMessageID || strings.TrimSpace(command.Text) == "" {
		return PostReceipt{}, ErrInvalidPost
	}
	if !utf8.ValidString(command.Text) {
		return PostReceipt{}, ErrInvalidPost
	}
	if utf8.RuneCountInString(command.Text) > MaxTextCharacters {
		return PostReceipt{}, ErrTextTooLong
	}

	hub.mu.Lock()
	defer hub.mu.Unlock()
	key := idempotencyKey{
		principalKind: principal.Kind, principalID: principal.ID,
		conversation: command.ConversationID, clientMessage: command.ClientMessageID,
	}
	if accepted, ok := hub.accepted[key]; ok {
		if accepted.command != command {
			return PostReceipt{}, ErrIdempotencyConflict
		}
		replayed := clonePostReceipt(accepted.receipt)
		replayed.Created = false
		return replayed, nil
	}
	if command.ReplyToMessageID != "" && !hub.hasMessage(command.ReplyToMessageID) {
		return PostReceipt{}, ErrReplyNotFound
	}
	sequence := int64(len(hub.events) + 1)
	acceptedAt := time.Now().UTC()
	messageID := externalID(
		"message:v1:",
		string(principal.Kind),
		principal.ID,
		command.ConversationID,
		command.ClientMessageID,
	)
	var delivery *DeliveryReceipt
	if principal.Kind == PrincipalHuman {
		delivery = &DeliveryReceipt{
			ID: externalID("delivery:v1:", messageID, hub.config.EndpointID), MessageID: messageID,
			EndpointID: hub.config.EndpointID, State: DeliveryPending,
		}
	}
	hub.events = append(hub.events, Event{
		Sequence: sequence,
		Type:     EventMessagePosted,
		Message: Message{
			ID: messageID, ConversationID: command.ConversationID,
			ClientMessageID: command.ClientMessageID, Author: principal,
			Text: command.Text, ReplyToMessageID: command.ReplyToMessageID,
			CreatedAt: acceptedAt,
		},
	})
	receipt := PostReceipt{
		MessageID: messageID, ConversationID: command.ConversationID,
		Sequence: sequence, AcceptedAt: acceptedAt, Delivery: delivery, Created: true,
	}
	hub.accepted[key] = acceptedPost{command: command, receipt: receipt}
	return clonePostReceipt(receipt), nil
}

// externalID makes an accepted message stable for the same immutable post
// identity while preventing a fresh process from reusing sequence-based IDs.
// Length-prefixing keeps field boundaries unambiguous.
func externalID(prefix string, fields ...string) string {
	digest := sha256.New()
	var size [8]byte
	for _, field := range fields {
		binary.BigEndian.PutUint64(size[:], uint64(len(field)))
		_, _ = digest.Write(size[:])
		_, _ = digest.Write([]byte(field))
	}
	return prefix + hex.EncodeToString(digest.Sum(nil))
}

func (hub *localHub) Events(ctx context.Context, principal Principal, conversationID string, afterSequence int64) (EventPage, error) {
	if err := ctx.Err(); err != nil {
		return EventPage{}, err
	}
	if !hub.validPrincipal(principal) {
		return EventPage{}, ErrForbidden
	}
	if conversationID != hub.config.ConversationID {
		return EventPage{}, ErrRecipientNotAllowed
	}

	hub.mu.RLock()
	defer hub.mu.RUnlock()
	latest := int64(len(hub.events))
	if afterSequence < 0 || afterSequence > latest {
		return EventPage{}, ErrInvalidCursor
	}
	events := append([]Event(nil), hub.events[afterSequence:]...)
	if events == nil {
		events = make([]Event, 0)
	}
	return EventPage{
		ConversationID: conversationID, AfterSequence: afterSequence,
		NextSequence: latest, Events: events,
	}, nil
}

func (hub *localHub) validPrincipal(principal Principal) bool {
	switch principal.Kind {
	case PrincipalHuman:
		return principal.ID == hub.config.HumanID
	case PrincipalPeer:
		return principal.ID == hub.config.PeerID
	default:
		return false
	}
}

func (hub *localHub) hasMessage(messageID string) bool {
	for _, event := range hub.events {
		if event.Message.ID == messageID {
			return true
		}
	}
	return false
}

func clonePostReceipt(receipt PostReceipt) PostReceipt {
	if receipt.Delivery != nil {
		delivery := *receipt.Delivery
		receipt.Delivery = &delivery
	}
	return receipt
}
