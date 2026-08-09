// Package conversation defines Fort's durable shared-conversation model.
// It is deliberately independent of storage, HTTP, and concrete runtimes.
package conversation

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

const MaxContextBytes = 65_536

var (
	ErrContextTooLarge    = errors.New("conversation context exceeds 65536 bytes")
	ErrConversationActive = errors.New("conversation has active targets")
)

type ErrorCode string

const (
	ErrorSeatUnknown        ErrorCode = "seat_unknown"
	ErrorSeatUnready        ErrorCode = "seat_unready"
	ErrorParticipantUnknown ErrorCode = "participant_unknown"
	ErrorParticipantRemoved ErrorCode = "participant_removed"
	ErrorConversationActive ErrorCode = "conversation_active"
)

// BoundedError carries one closed conversation error code without coupling the
// domain to HTTP. Transports may project Code while errors.Is/As can still
// inspect the underlying cause.
type BoundedError struct {
	Code ErrorCode
	Err  error
}

func (e *BoundedError) Error() string { return e.Err.Error() }
func (e *BoundedError) Unwrap() error { return e.Err }

func NewBoundedError(code ErrorCode, err error) error {
	return &BoundedError{Code: code, Err: err}
}

type AuthorKind string

const (
	AuthorHuman     AuthorKind = "human"
	AuthorAssistant AuthorKind = "agent"
	AuthorSystem    AuthorKind = "system"
)

type TargetState string

const (
	TargetQueued   TargetState = "queued"
	TargetWorking  TargetState = "working"
	TargetAnswered TargetState = "answered"
	TargetFailed   TargetState = "failed"
	TargetCanceled TargetState = "canceled"
)

type Project struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Conversation struct {
	ID        string            `json:"id"`
	ProjectID string            `json:"project_id,omitempty"`
	Title     string            `json:"title"`
	State     ConversationState `json:"state"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

type ConversationState string

const (
	ConversationOpen     ConversationState = "open"
	ConversationArchived ConversationState = "archived"
)

// Seat is one exact profile + provider/model + machine choice advertised by
// the capability inventory. A conversation persists the resolved fields, not
// merely this ephemeral identifier.
type Seat struct {
	ID          string `json:"id"`
	Profile     string `json:"profile"`
	Agent       string `json:"agent"`
	Model       string `json:"model,omitempty"`
	Machine     string `json:"machine"`
	DisplayName string `json:"display_name"`
	State       string `json:"state"`
	Reason      string `json:"reason,omitempty"`
}

type Participant struct {
	ID             string           `json:"id"`
	ConversationID string           `json:"conversation_id"`
	SeatID         string           `json:"seat_id"`
	Profile        string           `json:"profile"`
	Agent          string           `json:"agent"`
	Model          string           `json:"model,omitempty"`
	Machine        string           `json:"machine"`
	DisplayName    string           `json:"display_name"`
	Position       int              `json:"position"`
	State          ParticipantState `json:"state"`
	CreatedAt      time.Time        `json:"created_at"`
	RemovedAt      time.Time        `json:"removed_at,omitempty"`
}

type ParticipantState string

const (
	ParticipantActive  ParticipantState = "active"
	ParticipantRemoved ParticipantState = "removed"
)

type Turn struct {
	ID               string    `json:"id"`
	ConversationID   string    `json:"conversation_id"`
	ClientTurnID     string    `json:"client_turn_id"`
	PromptMessageID  int64     `json:"prompt_message_id"`
	ThroughMessageID int64     `json:"through_message_id"`
	CreatedAt        time.Time `json:"created_at"`
	Created          bool      `json:"-"`
}

type Target struct {
	ID            string           `json:"id"`
	TurnID        string           `json:"turn_id"`
	ParticipantID string           `json:"participant_id"`
	RunID         string           `json:"run_id"`
	Attempt       int              `json:"attempt"`
	State         TargetState      `json:"state"`
	ErrorCode     string           `json:"error_code,omitempty"`
	Error         string           `json:"error,omitempty"`
	Authority     *TargetAuthority `json:"authority,omitempty"`
	Receipt       *TargetReceipt   `json:"receipt,omitempty"`
	CreatedAt     time.Time        `json:"created_at"`
	UpdatedAt     time.Time        `json:"updated_at"`
}

func ValidateProjectName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("project name is required")
	}
	if len([]byte(name)) > 120 {
		return "", fmt.Errorf("project name exceeds 120 UTF-8 bytes")
	}
	return name, nil
}

type TurnResult struct {
	Turn    Turn     `json:"turn"`
	Targets []Target `json:"targets"`
}

type Message struct {
	ID             int64      `json:"id"`
	ConversationID string     `json:"conversation_id"`
	TurnID         string     `json:"turn_id,omitempty"`
	TargetID       string     `json:"target_id,omitempty"`
	AuthorKind     AuthorKind `json:"author_kind"`
	AuthorID       string     `json:"author_id"`
	Body           string     `json:"body"`
	CreatedAt      time.Time  `json:"created_at"`
}

// CanTransition keeps target lifecycle reporting explicit and monotonic.
func CanTransition(from, to TargetState) bool {
	switch from {
	case TargetQueued:
		return to == TargetWorking || to == TargetFailed || to == TargetCanceled
	case TargetWorking:
		return to == TargetAnswered || to == TargetFailed || to == TargetCanceled
	default:
		return false
	}
}

type compiledContext struct {
	Version          int                 `json:"version"`
	ConversationID   string              `json:"conversation_id"`
	ThroughMessageID int64               `json:"through_message_id"`
	Participants     []promptParticipant `json:"participants"`
	Messages         []contextMessage    `json:"messages"`
}

type contextMessage struct {
	ID         int64      `json:"id"`
	AuthorKind AuthorKind `json:"author_kind"`
	AuthorID   string     `json:"author_id"`
	Body       string     `json:"body"`
}

// CompileContext creates the frozen provider prompt shared by every target in
// a turn. Sorting copies by durable participant position/ID and message ID
// makes the JSON canonical. The size check rejects the whole turn; history is
// never silently truncated.
func CompileContext(conversationID string, throughMessageID int64, participants []Participant, messages []Message) (string, error) {
	orderedParticipants := append([]Participant(nil), participants...)
	sort.SliceStable(orderedParticipants, func(i, j int) bool {
		if orderedParticipants[i].Position != orderedParticipants[j].Position {
			return orderedParticipants[i].Position < orderedParticipants[j].Position
		}
		return orderedParticipants[i].ID < orderedParticipants[j].ID
	})
	ordered := append([]Message(nil), messages...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].ID < ordered[j].ID })
	envelope := compiledContext{
		Version:          1,
		ConversationID:   conversationID,
		ThroughMessageID: throughMessageID,
		Participants:     []promptParticipant{},
		Messages:         []contextMessage{},
	}
	for _, participant := range orderedParticipants {
		envelope.Participants = append(envelope.Participants, promptParticipant{
			ParticipantID: participant.ID, Profile: participant.Profile, Agent: participant.Agent,
			Model: participant.Model, Machine: participant.Machine, DisplayName: participant.DisplayName,
		})
	}
	for _, message := range ordered {
		if message.ID > throughMessageID {
			continue
		}
		envelope.Messages = append(envelope.Messages, contextMessage{
			ID: message.ID, AuthorKind: message.AuthorKind, AuthorID: message.AuthorID, Body: message.Body,
		})
	}
	data, err := json.Marshal(envelope)
	if err != nil {
		return "", fmt.Errorf("compile conversation context: %w", err)
	}
	if len(data) > MaxContextBytes {
		return "", ErrContextTooLarge
	}
	return string(data), nil
}

type participantPrompt struct {
	Version     int               `json:"version"`
	Instruction string            `json:"instruction"`
	Participant promptParticipant `json:"participant"`
	Context     json.RawMessage   `json:"context"`
}

type promptParticipant struct {
	ParticipantID string `json:"participant_id"`
	Profile       string `json:"profile"`
	Agent         string `json:"agent"`
	Model         string `json:"model,omitempty"`
	Machine       string `json:"machine"`
	DisplayName   string `json:"display_name"`
}

func CompileParticipantPrompt(contextJSON string, participant Participant) (string, error) {
	prompt := participantPrompt{
		Version: 1, Instruction: "Answer as this exact participant. Return one final answer for the human. Do not dispatch, mention, or wake another agent.",
		Participant: promptParticipant{
			ParticipantID: participant.ID, Profile: participant.Profile, Agent: participant.Agent,
			Model: participant.Model, Machine: participant.Machine, DisplayName: participant.DisplayName,
		},
		Context: json.RawMessage(contextJSON),
	}
	data, err := json.Marshal(prompt)
	if err != nil {
		return "", fmt.Errorf("compile participant prompt: %w", err)
	}
	if len(data) > MaxContextBytes {
		return "", ErrContextTooLarge
	}
	return string(data), nil
}
