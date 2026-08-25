package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tobsai/fort/core/channelturn"
)

const channelTurnSchema = `
CREATE TABLE IF NOT EXISTS channel_turn_attempt (
  id TEXT PRIMARY KEY,
  conversation_id TEXT NOT NULL,
  binding_revision_id TEXT NOT NULL,
  client_attempt_id TEXT NOT NULL,
  text TEXT NOT NULL CHECK(length(text)>0),
  transport_id TEXT NOT NULL CHECK(transport_id='hermes_platform_relay_v1'),
  protocol_revision TEXT NOT NULL,
  accepted_at TEXT NOT NULL,
  UNIQUE(conversation_id,client_attempt_id),
  FOREIGN KEY(conversation_id) REFERENCES conversation(id),
  FOREIGN KEY(binding_revision_id) REFERENCES agent_binding_revision(id)
);
CREATE INDEX IF NOT EXISTS idx_channel_turn_attempt_binding
  ON channel_turn_attempt(binding_revision_id,accepted_at,id);

CREATE TABLE IF NOT EXISTS channel_turn_event (
  event_id TEXT PRIMARY KEY,
  attempt_id TEXT NOT NULL,
  sequence INTEGER NOT NULL CHECK(sequence>0),
  type TEXT NOT NULL CHECK(type='accepted'),
  created_at TEXT NOT NULL,
  UNIQUE(attempt_id,sequence),
  FOREIGN KEY(attempt_id) REFERENCES channel_turn_attempt(id)
);

CREATE TRIGGER IF NOT EXISTS channel_turn_attempt_insert_invariant
BEFORE INSERT ON channel_turn_attempt
WHEN NOT EXISTS (
  SELECT 1
  FROM agent_binding_revision binding
  JOIN stable_agent agent ON agent.id=binding.agent_id
  WHERE binding.id=NEW.binding_revision_id
    AND agent.canonical_conversation_id=NEW.conversation_id
    AND binding.adapter_id=NEW.transport_id
    AND binding.adapter_revision=NEW.protocol_revision
)
BEGIN SELECT RAISE(ABORT,'channel_turn_binding_invariant'); END;

CREATE TRIGGER IF NOT EXISTS channel_turn_attempt_immutable_update
BEFORE UPDATE ON channel_turn_attempt
BEGIN SELECT RAISE(ABORT,'channel_turn_attempt_immutable'); END;
CREATE TRIGGER IF NOT EXISTS channel_turn_attempt_immutable_delete
BEFORE DELETE ON channel_turn_attempt
BEGIN SELECT RAISE(ABORT,'channel_turn_attempt_immutable'); END;
CREATE TRIGGER IF NOT EXISTS channel_turn_event_immutable_update
BEFORE UPDATE ON channel_turn_event
BEGIN SELECT RAISE(ABORT,'channel_turn_event_immutable'); END;
CREATE TRIGGER IF NOT EXISTS channel_turn_event_immutable_delete
BEFORE DELETE ON channel_turn_event
BEGIN SELECT RAISE(ABORT,'channel_turn_event_immutable'); END;
`

func (s *Store) migrateChannelTurns() error {
	_, err := s.db.Exec(channelTurnSchema)
	return err
}

type channelTurnModule struct{ store *Store }

var _ channelturn.Module = channelTurnModule{}

// ChannelTurns wires this Store to Fort's public durable channel-turn module.
func (s *Store) ChannelTurns() channelturn.Module { return channelTurnModule{store: s} }

func (module channelTurnModule) Submit(
	ctx context.Context,
	bindingID string,
	clientAttemptID string,
	text string,
) (channelturn.AcceptanceReceipt, error) {
	if !canonicalChannelTurnID(bindingID) || !canonicalChannelTurnID(clientAttemptID) || strings.TrimSpace(text) == "" {
		return channelturn.AcceptanceReceipt{}, channelturn.ErrInvalidInput
	}
	if len([]byte(text)) > channelturn.MaxTextBytes {
		return channelturn.AcceptanceReceipt{}, fmt.Errorf("%w: text exceeds %d bytes", channelturn.ErrInvalidInput, channelturn.MaxTextBytes)
	}

	tx, err := module.store.db.BeginTx(ctx, nil)
	if err != nil {
		return channelturn.AcceptanceReceipt{}, err
	}
	defer tx.Rollback()

	var conversationID, transportID, protocolRevision string
	err = tx.QueryRowContext(ctx, `SELECT agent.canonical_conversation_id,binding.adapter_id,binding.adapter_revision
FROM agent_binding_revision binding
JOIN stable_agent agent ON agent.id=binding.agent_id
WHERE binding.id=?`, bindingID).Scan(&conversationID, &transportID, &protocolRevision)
	if errors.Is(err, sql.ErrNoRows) {
		return channelturn.AcceptanceReceipt{}, channelturn.ErrBindingNotFound
	}
	if err != nil {
		return channelturn.AcceptanceReceipt{}, err
	}
	if transportID != channelturn.TransportHermesPlatformRelayV1 {
		return channelturn.AcceptanceReceipt{}, fmt.Errorf("%w: %s", channelturn.ErrUnsupportedBinding, transportID)
	}

	existing, existingBindingID, existingText, err := readChannelTurnReceiptByClientAttempt(
		ctx, tx, conversationID, clientAttemptID,
	)
	if err == nil {
		if existingBindingID != bindingID || existingText != text {
			return channelturn.AcceptanceReceipt{}, channelturn.ErrIdempotencyConflict
		}
		return existing, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return channelturn.AcceptanceReceipt{}, err
	}

	attemptID := uuid.NewSHA1(
		uuid.NameSpaceURL,
		[]byte("fort:channel-turn:v1\x00"+conversationID+"\x00"+clientAttemptID),
	).String()
	acceptedAt := time.Now().UTC()
	if _, err := tx.ExecContext(ctx, `INSERT INTO channel_turn_attempt(
id,conversation_id,binding_revision_id,client_attempt_id,text,transport_id,protocol_revision,accepted_at
) VALUES(?,?,?,?,?,?,?,?)`, attemptID, conversationID, bindingID, clientAttemptID, text, transportID,
		protocolRevision, nowOr(acceptedAt)); err != nil {
		return channelturn.AcceptanceReceipt{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO channel_turn_event(
event_id,attempt_id,sequence,type,created_at
) VALUES(?,?,?,?,?)`, attemptID+":1", attemptID, 1, channelturn.EventAccepted, nowOr(acceptedAt)); err != nil {
		return channelturn.AcceptanceReceipt{}, err
	}
	if err := tx.Commit(); err != nil {
		return channelturn.AcceptanceReceipt{}, err
	}
	return channelturn.AcceptanceReceipt{
		AttemptID: attemptID, ConversationID: conversationID, BindingID: bindingID,
		ClientAttemptID: clientAttemptID, State: channelturn.StateAccepted,
		AcceptedSequence: 1, AcceptedAt: acceptedAt,
	}, nil
}

func (module channelTurnModule) Events(
	ctx context.Context,
	attemptID string,
	afterSequence int64,
) (channelturn.ReplayableEventStream, error) {
	stream := channelturn.ReplayableEventStream{
		AttemptID: attemptID, AfterSequence: afterSequence, Events: []channelturn.Event{},
	}
	if !canonicalChannelTurnID(attemptID) || afterSequence < 0 {
		return channelturn.ReplayableEventStream{}, channelturn.ErrInvalidInput
	}
	var exists int
	if err := module.store.db.QueryRowContext(ctx, `SELECT 1 FROM channel_turn_attempt WHERE id=?`, attemptID).Scan(&exists); errors.Is(err, sql.ErrNoRows) {
		return channelturn.ReplayableEventStream{}, channelturn.ErrAttemptNotFound
	} else if err != nil {
		return channelturn.ReplayableEventStream{}, err
	}
	rows, err := module.store.db.QueryContext(ctx, `SELECT
event.event_id,event.attempt_id,attempt.conversation_id,attempt.binding_revision_id,attempt.client_attempt_id,
event.sequence,event.type,attempt.protocol_revision,event.created_at
FROM channel_turn_event event
JOIN channel_turn_attempt attempt ON attempt.id=event.attempt_id
WHERE event.attempt_id=? AND event.sequence>? ORDER BY event.sequence`, attemptID, afterSequence)
	if err != nil {
		return channelturn.ReplayableEventStream{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var event channelturn.Event
		var eventType, createdAt string
		if err := rows.Scan(
			&event.ID, &event.AttemptID, &event.ConversationID, &event.BindingID, &event.ClientAttemptID,
			&event.Sequence, &eventType, &event.ProtocolRevision, &createdAt,
		); err != nil {
			return channelturn.ReplayableEventStream{}, err
		}
		event.Type = channelturn.EventType(eventType)
		event.CreatedAt = parseTime(createdAt)
		stream.Events = append(stream.Events, event)
	}
	if err := rows.Err(); err != nil {
		return channelturn.ReplayableEventStream{}, err
	}
	return stream, nil
}

func readChannelTurnReceiptByClientAttempt(
	ctx context.Context,
	tx *sql.Tx,
	conversationID string,
	clientAttemptID string,
) (channelturn.AcceptanceReceipt, string, string, error) {
	var receipt channelturn.AcceptanceReceipt
	var bindingID, text, acceptedAt string
	err := tx.QueryRowContext(ctx, `SELECT id,binding_revision_id,text,accepted_at
FROM channel_turn_attempt WHERE conversation_id=? AND client_attempt_id=?`,
		conversationID, clientAttemptID,
	).Scan(&receipt.AttemptID, &bindingID, &text, &acceptedAt)
	if err != nil {
		return channelturn.AcceptanceReceipt{}, "", "", err
	}
	receipt.ConversationID = conversationID
	receipt.BindingID = bindingID
	receipt.ClientAttemptID = clientAttemptID
	receipt.State = channelturn.StateAccepted
	receipt.AcceptedSequence = 1
	receipt.AcceptedAt = parseTime(acceptedAt)
	return receipt, bindingID, text, nil
}

func canonicalChannelTurnID(value string) bool {
	return strings.TrimSpace(value) != "" && strings.TrimSpace(value) == value
}
