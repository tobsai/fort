// Package channelturn defines Fort's durable channel-turn module interface.
// Acceptance records user intent in Fort's canonical ledger; it does not claim
// that any transport, machine, or Agent accepted or executed the turn.
package channelturn

import (
	"context"
	"errors"
	"time"
)

const (
	TransportHermesPlatformRelayV1 = "hermes_platform_relay_v1"
	MaxTextBytes                   = 2 << 20
)

var (
	ErrInvalidInput        = errors.New("channel turn input is invalid")
	ErrBindingNotFound     = errors.New("channel turn binding not found")
	ErrUnsupportedBinding  = errors.New("channel turn binding uses another transport")
	ErrAttemptNotFound     = errors.New("channel turn attempt not found")
	ErrIdempotencyConflict = errors.New("channel turn idempotency conflict")
)

type State string

const StateAccepted State = "accepted"

type EventType string

const EventAccepted EventType = "accepted"

// AcceptanceReceipt proves only that Fort committed the exact user intent and
// immutable Binding Revision to its local ledger.
type AcceptanceReceipt struct {
	AttemptID        string    `json:"attempt_id"`
	ConversationID   string    `json:"conversation_id"`
	BindingID        string    `json:"binding_id"`
	ClientAttemptID  string    `json:"client_attempt_id"`
	State            State     `json:"state"`
	AcceptedSequence int64     `json:"accepted_sequence"`
	AcceptedAt       time.Time `json:"accepted_at"`
}

type Event struct {
	ID               string    `json:"id"`
	AttemptID        string    `json:"attempt_id"`
	ConversationID   string    `json:"conversation_id"`
	BindingID        string    `json:"binding_id"`
	ClientAttemptID  string    `json:"client_attempt_id"`
	Sequence         int64     `json:"sequence"`
	Type             EventType `json:"type"`
	ProtocolRevision string    `json:"protocol_revision"`
	CreatedAt        time.Time `json:"created_at"`
}

// ReplayableEventStream is one ordered durable replay after a caller-supplied
// sequence cursor. Live transport tailing is deliberately outside Slice 1.
type ReplayableEventStream struct {
	AttemptID     string  `json:"attempt_id"`
	AfterSequence int64   `json:"after_sequence"`
	Events        []Event `json:"events"`
}

// Module is the public durable channel-turn seam. Cancel is deliberately
// absent until a later behavior-first slice can return a truthful outcome.
type Module interface {
	Submit(ctx context.Context, bindingID, clientAttemptID, text string) (AcceptanceReceipt, error)
	Events(ctx context.Context, attemptID string, afterSequence int64) (ReplayableEventStream, error)
}
