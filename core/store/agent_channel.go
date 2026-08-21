package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/tobsai/fort/core/conversation"
)

var ErrAgentChannelState = errors.New("Agent Channel resource is not open")

const agentChannelSchema = `
CREATE TABLE IF NOT EXISTS agent_channel (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  state TEXT NOT NULL,
  option_id TEXT NOT NULL,
  binding_key TEXT NOT NULL UNIQUE,
  binding_json TEXT NOT NULL,
  created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS agent_channel_conversation (
  agent_channel_id TEXT NOT NULL,
  conversation_id TEXT PRIMARY KEY,
  created_at TEXT NOT NULL,
  FOREIGN KEY(agent_channel_id) REFERENCES agent_channel(id),
  FOREIGN KEY(conversation_id) REFERENCES conversation(id),
  UNIQUE(agent_channel_id,conversation_id)
);
CREATE INDEX IF NOT EXISTS idx_agent_channel_conversation_channel
  ON agent_channel_conversation(agent_channel_id,created_at DESC,conversation_id);
CREATE TABLE IF NOT EXISTS agent_conversation_pin (
  conversation_id TEXT PRIMARY KEY,
  pinned_at TEXT NOT NULL,
  FOREIGN KEY(conversation_id) REFERENCES agent_channel_conversation(conversation_id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS agent_channel_created_conversation (
  conversation_id TEXT PRIMARY KEY,
  FOREIGN KEY(conversation_id) REFERENCES agent_channel_conversation(conversation_id)
);

CREATE TRIGGER IF NOT EXISTS agent_channel_identity_immutable
BEFORE UPDATE OF id,option_id,binding_key,binding_json,created_at ON agent_channel
BEGIN
  SELECT RAISE(ABORT, 'agent_channel_invariant');
END;

CREATE TRIGGER IF NOT EXISTS agent_channel_delete_immutable
BEFORE DELETE ON agent_channel
BEGIN
  SELECT RAISE(ABORT, 'agent_channel_invariant');
END;

CREATE TRIGGER IF NOT EXISTS agent_channel_conversation_insert_exact_invariant_v2
BEFORE INSERT ON agent_channel_conversation
WHEN NOT EXISTS (
  SELECT 1
  FROM agent_channel channel
  JOIN conversation_participant participant ON participant.conversation_id=NEW.conversation_id
  WHERE channel.id=NEW.agent_channel_id
    AND participant.state='active'
    AND participant.seat_id=json_extract(channel.binding_json,'$.seat.id')
    AND participant.profile=json_extract(channel.binding_json,'$.seat.profile')
    AND participant.agent=json_extract(channel.binding_json,'$.seat.agent')
    AND COALESCE(participant.model,'')=json_extract(channel.binding_json,'$.seat.model')
    AND participant.machine=json_extract(channel.binding_json,'$.seat.machine')
    AND (SELECT COUNT(*) FROM conversation_participant active
         WHERE active.conversation_id=NEW.conversation_id AND active.state='active')=1
    AND (
      (
        json_extract(channel.binding_json,'$.authority.authority')<>'chat_subscription_isolated_v1'
        AND NOT EXISTS (SELECT 1 FROM primary_channel unexpected WHERE unexpected.conversation_id=NEW.conversation_id)
      )
      OR EXISTS (
        SELECT 1 FROM primary_channel exact
        WHERE exact.conversation_id=NEW.conversation_id
          AND exact.participant_id=participant.id
          AND exact.authority=json_extract(channel.binding_json,'$.authority.authority')
          AND exact.policy_id=json_extract(channel.binding_json,'$.authority.policy_id')
          AND exact.policy_revision=json_extract(channel.binding_json,'$.authority.policy_revision')
          AND exact.adapter_id=json_extract(channel.binding_json,'$.authority.adapter_id')
          AND exact.adapter_revision=json_extract(channel.binding_json,'$.authority.adapter_revision')
          AND exact.runtime_contract=json_extract(channel.binding_json,'$.authority.runtime_contract')
          AND exact.thread_mode=json_extract(channel.binding_json,'$.authority.session_mode')
          AND json_extract(channel.binding_json,'$.authority.requested_model')=json_extract(channel.binding_json,'$.seat.model')
          AND json_extract(channel.binding_json,'$.authority.resolved_model')='unknown'
          AND json_extract(channel.binding_json,'$.authority.memory_mode')='ephemeral'
          AND exact.codex_version=json_extract(channel.binding_json,'$.authority.execution_policy.codex_version')
          AND exact.codex_executable_revision=json_extract(channel.binding_json,'$.authority.execution_policy.codex_executable_revision')
          AND exact.codex_schema_revision=json_extract(channel.binding_json,'$.authority.execution_policy.codex_schema_revision')
          AND exact.reasoning_effort=json_extract(channel.binding_json,'$.authority.execution_policy.reasoning_effort')
          AND exact.reasoning_context=json_extract(channel.binding_json,'$.authority.execution_policy.reasoning_context')
          AND exact.request_timeout_millis=CAST(json_extract(channel.binding_json,'$.authority.execution_policy.request_timeout_millis') AS INTEGER)
          AND exact.developer_instruction_revision=json_extract(channel.binding_json,'$.authority.execution_policy.developer_instruction_revision')
          AND exact.account_type=json_extract(channel.binding_json,'$.authority.execution_policy.account_type')
          AND exact.account_plan=json_extract(channel.binding_json,'$.authority.execution_policy.account_plan')
          AND exact.sandbox_mode=json_extract(channel.binding_json,'$.authority.execution_policy.sandbox_mode')
          AND exact.approval_policy=json_extract(channel.binding_json,'$.authority.execution_policy.approval_policy')
          AND exact.workdir_mode=json_extract(channel.binding_json,'$.authority.execution_policy.workdir_mode')
          AND exact.dynamic_tools_mode=json_extract(channel.binding_json,'$.authority.execution_policy.dynamic_tools_mode')
          AND exact.mcp_mode=json_extract(channel.binding_json,'$.authority.execution_policy.mcp_mode')
          AND exact.command_policy=json_extract(channel.binding_json,'$.authority.execution_policy.command_policy')
          AND exact.file_read_policy=json_extract(channel.binding_json,'$.authority.execution_policy.file_read_policy')
          AND exact.isolation_revision=json_extract(channel.binding_json,'$.authority.execution_policy.isolation_revision')
      )
    )
)
BEGIN
  SELECT RAISE(ABORT, 'agent_channel_invariant');
END;

CREATE TRIGGER IF NOT EXISTS agent_channel_conversation_update_immutable
BEFORE UPDATE ON agent_channel_conversation
BEGIN
  SELECT RAISE(ABORT, 'agent_channel_invariant');
END;

CREATE TRIGGER IF NOT EXISTS agent_channel_conversation_delete_immutable
BEFORE DELETE ON agent_channel_conversation
BEGIN
  SELECT RAISE(ABORT, 'agent_channel_invariant');
END;

CREATE TRIGGER IF NOT EXISTS agent_channel_created_conversation_update_immutable
BEFORE UPDATE ON agent_channel_created_conversation
BEGIN
  SELECT RAISE(ABORT, 'agent_channel_invariant');
END;

CREATE TRIGGER IF NOT EXISTS agent_channel_created_conversation_delete_immutable
BEFORE DELETE ON agent_channel_created_conversation
BEGIN
  SELECT RAISE(ABORT, 'agent_channel_invariant');
END;

CREATE TRIGGER IF NOT EXISTS agent_channel_participant_insert_immutable
BEFORE INSERT ON conversation_participant
WHEN EXISTS (SELECT 1 FROM agent_channel_conversation WHERE conversation_id=NEW.conversation_id)
BEGIN
  SELECT RAISE(ABORT, 'agent_channel_invariant');
END;

CREATE TRIGGER IF NOT EXISTS agent_channel_participant_update_immutable
BEFORE UPDATE ON conversation_participant
WHEN EXISTS (SELECT 1 FROM agent_channel_conversation WHERE conversation_id=OLD.conversation_id OR conversation_id=NEW.conversation_id)
BEGIN
  SELECT RAISE(ABORT, 'agent_channel_invariant');
END;

CREATE TRIGGER IF NOT EXISTS agent_channel_participant_delete_immutable
BEFORE DELETE ON conversation_participant
WHEN EXISTS (SELECT 1 FROM agent_channel_conversation WHERE conversation_id=OLD.conversation_id)
BEGIN
  SELECT RAISE(ABORT, 'agent_channel_invariant');
END;

CREATE TRIGGER IF NOT EXISTS agent_channel_conversation_row_delete_immutable
BEFORE DELETE ON conversation
WHEN EXISTS (SELECT 1 FROM agent_channel_conversation WHERE conversation_id=OLD.id)
BEGIN
  SELECT RAISE(ABORT, 'agent_channel_invariant');
END;
`

func (s *Store) migrateAgentChannels() error {
	_, err := s.db.Exec(agentChannelSchema)
	return err
}

func (s *Store) CreateAgentChannel(channel conversation.AgentChannel) error {
	if err := channel.Validate(); err != nil {
		return err
	}
	bindingKey, err := conversation.AgentChannelID(channel.Binding)
	if err != nil {
		return err
	}
	bindingJSON, err := json.Marshal(channel.Binding)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO agent_channel(id,name,state,option_id,binding_key,binding_json,created_at)
VALUES(?,?,?,?,?,?,?)`, channel.ID, channel.Name, channel.State, channel.OptionID, bindingKey, string(bindingJSON), nowOr(channel.CreatedAt))
	return err
}

func (s *Store) ListAgentChannels(state string) ([]conversation.AgentChannelDetail, error) {
	if state == "" {
		state = string(conversation.AgentChannelOpen)
	}
	if state != string(conversation.AgentChannelOpen) && state != string(conversation.AgentChannelArchived) && state != "all" {
		return nil, fmt.Errorf("invalid Agent Channel state %q", state)
	}
	query := `SELECT id FROM agent_channel`
	args := []any{}
	if state != "all" {
		query += ` WHERE state=?`
		args = append(args, state)
	}
	query += ` ORDER BY created_at,id`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	out := make([]conversation.AgentChannelDetail, 0, len(ids))
	for _, id := range ids {
		detail, err := s.GetAgentChannel(id)
		if err != nil {
			return nil, err
		}
		out = append(out, detail)
	}
	return out, nil
}

func (s *Store) RenameAgentChannel(id, name string) error {
	name = strings.TrimSpace(name)
	if name == "" || len([]byte(name)) > 120 {
		return fmt.Errorf("Agent Channel name must contain 1 to 120 UTF-8 bytes")
	}
	result, err := s.db.Exec(`UPDATE agent_channel SET name=? WHERE id=?`, name, id)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err == nil && changed != 1 {
		return sql.ErrNoRows
	}
	return err
}

func (s *Store) SetAgentChannelState(id string, state conversation.AgentChannelState) error {
	if state != conversation.AgentChannelOpen && state != conversation.AgentChannelArchived {
		return fmt.Errorf("Agent Channel state must be open or archived")
	}
	result, err := s.db.Exec(`UPDATE agent_channel SET state=? WHERE id=?`, state, id)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err == nil && changed != 1 {
		return sql.ErrNoRows
	}
	return err
}

func (s *Store) AgentConversationOwned(channelID, conversationID string) (bool, error) {
	var owned int
	err := s.db.QueryRow(`SELECT EXISTS(
SELECT 1 FROM agent_channel_conversation WHERE agent_channel_id=? AND conversation_id=?
)`, channelID, conversationID).Scan(&owned)
	return owned == 1, err
}

// AgentCreatedConversationChannel identifies Conversations created through the
// Agent-first product. Migrated legacy Primary Conversations deliberately do
// not have this provenance, so their legacy mutation contract remains intact.
func (s *Store) AgentCreatedConversationChannel(conversationID string) (conversation.AgentChannel, bool, error) {
	channel, err := scanAgentChannel(s.db.QueryRow(`SELECT channel.id,channel.name,channel.state,channel.option_id,channel.binding_json,channel.created_at
FROM agent_channel_created_conversation created
JOIN agent_channel_conversation ownership ON ownership.conversation_id=created.conversation_id
JOIN agent_channel channel ON channel.id=ownership.agent_channel_id
WHERE created.conversation_id=?`, conversationID))
	if errors.Is(err, sql.ErrNoRows) {
		return conversation.AgentChannel{}, false, nil
	}
	if err != nil {
		return conversation.AgentChannel{}, false, err
	}
	return channel, true, nil
}

// CreateAgentChannelConversation copies the parent's immutable seat into one
// new conversation and records its ownership atomically. A compatible current
// Codex binding also receives the existing Primary marker so its execution and
// rollback contracts remain unchanged.
func (s *Store) CreateAgentChannelConversation(channelID string, item conversation.Conversation, participantID string) error {
	tx, err := s.beginConversationTurnTransaction(true)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	channel, err := scanAgentChannel(tx.QueryRow(`SELECT id,name,state,option_id,binding_json,created_at FROM agent_channel WHERE id=?`, channelID))
	if err != nil {
		return err
	}
	if channel.State != conversation.AgentChannelOpen {
		return fmt.Errorf("%w: archived Agent Channel %s cannot create a Conversation", ErrAgentChannelState, channel.ID)
	}
	if item.State == "" {
		item.State = conversation.ConversationOpen
	}
	created := nowOr(item.CreatedAt)
	updated := created
	if !item.UpdatedAt.IsZero() {
		updated = nowOr(item.UpdatedAt)
	}
	if _, err := tx.Exec(`INSERT INTO conversation(id,project_id,title,state,created_at,updated_at) VALUES(?,?,?,?,?,?)`,
		item.ID, nullableString(item.ProjectID), item.Title, item.State, created, updated); err != nil {
		return err
	}
	seat := channel.Binding.Seat
	if _, err := tx.Exec(`INSERT INTO conversation_participant(
id,conversation_id,seat_id,profile,agent,model,machine,display_name,position,state,created_at,removed_at
) VALUES(?,?,?,?,?,?,?,?,0,?,?,NULL)`, participantID, item.ID, seat.ID, seat.Profile, seat.Agent,
		nullableString(seat.Model), seat.Machine, channel.Name, conversation.ParticipantActive, created); err != nil {
		return err
	}
	if primary, compatible, err := primaryChannelFromAgentBinding(channel.Binding, item.ID, participantID, parseTime(created)); err != nil {
		return err
	} else if compatible {
		if err := insertPrimaryChannelMarker(tx, primary); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`INSERT INTO agent_channel_conversation(agent_channel_id,conversation_id,created_at) VALUES(?,?,?)`, channel.ID, item.ID, created); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO agent_channel_created_conversation(conversation_id) VALUES(?)`, item.ID); err != nil {
		return err
	}
	return tx.Commit()
}

type CreateAgentChannelConversationTurnParams struct {
	ChannelID     string
	Conversation  conversation.Conversation
	ParticipantID string
	TurnID        string
	ClientTurnID  string
	TargetID      string
	RunID         string
	HumanID       string
	Body          string
	Authority     *conversation.TargetAuthority
	CreatedAt     time.Time
}

// CreateAgentChannelConversationTurn is the first-Send durable boundary. It
// creates the child conversation, copied participant, ownership, compatibility
// marker, prompt, turn, and queued target in one immediate transaction.
func (s *Store) CreateAgentChannelConversationTurn(params CreateAgentChannelConversationTurnParams) (conversation.Turn, []conversation.Target, string, error) {
	if strings.TrimSpace(params.ClientTurnID) == "" {
		return conversation.Turn{}, nil, "", fmt.Errorf("client turn id is required")
	}
	tx, err := s.beginConversationTurnTransaction(true)
	if err != nil {
		return conversation.Turn{}, nil, "", err
	}
	defer tx.Rollback()

	var existing conversation.Turn
	var existingCreated, existingContext string
	err = tx.QueryRow(`SELECT turn.id,turn.conversation_id,turn.client_turn_id,turn.prompt_message_id,
turn.through_message_id,turn.created_at,turn.context_json
FROM agent_channel_conversation ownership
JOIN conversation_turn turn ON turn.conversation_id=ownership.conversation_id
WHERE ownership.agent_channel_id=? AND ownership.conversation_id=? AND turn.client_turn_id=?`,
		params.ChannelID, params.Conversation.ID, params.ClientTurnID).Scan(
		&existing.ID, &existing.ConversationID, &existing.ClientTurnID, &existing.PromptMessageID,
		&existing.ThroughMessageID, &existingCreated, &existingContext,
	)
	if err == nil {
		existing.CreatedAt = parseTime(existingCreated)
		targets, targetErr := conversationTargetsForTurn(tx, existing.ID)
		if targetErr != nil {
			return conversation.Turn{}, nil, "", targetErr
		}
		if existingContext == "" {
			return conversation.Turn{}, nil, "", fmt.Errorf("conversation turn %s has no frozen context", existing.ID)
		}
		if err := tx.Commit(); err != nil {
			return conversation.Turn{}, nil, "", err
		}
		return existing, targets, existingContext, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return conversation.Turn{}, nil, "", err
	}
	var alreadyOwned int
	err = tx.QueryRow(`SELECT 1 FROM agent_channel_conversation WHERE agent_channel_id=? AND conversation_id=?`, params.ChannelID, params.Conversation.ID).Scan(&alreadyOwned)
	if err == nil {
		return conversation.Turn{}, nil, "", fmt.Errorf("conversation %s already exists without client turn %s", params.Conversation.ID, params.ClientTurnID)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return conversation.Turn{}, nil, "", err
	}

	channel, err := scanAgentChannel(tx.QueryRow(`SELECT id,name,state,option_id,binding_json,created_at FROM agent_channel WHERE id=?`, params.ChannelID))
	if err != nil {
		return conversation.Turn{}, nil, "", err
	}
	if channel.State != conversation.AgentChannelOpen {
		return conversation.Turn{}, nil, "", fmt.Errorf("%w: archived Agent Channel %s cannot start a conversation", ErrAgentChannelState, channel.ID)
	}
	item := params.Conversation
	if item.State == "" {
		item.State = conversation.ConversationOpen
	}
	created := nowOr(item.CreatedAt)
	updated := created
	if !item.UpdatedAt.IsZero() {
		updated = nowOr(item.UpdatedAt)
	}
	if _, err := tx.Exec(`INSERT INTO conversation(id,project_id,title,state,created_at,updated_at) VALUES(?,?,?,?,?,?)`,
		item.ID, nullableString(item.ProjectID), item.Title, item.State, created, updated); err != nil {
		return conversation.Turn{}, nil, "", err
	}
	seat := channel.Binding.Seat
	if _, err := tx.Exec(`INSERT INTO conversation_participant(
id,conversation_id,seat_id,profile,agent,model,machine,display_name,position,state,created_at,removed_at
) VALUES(?,?,?,?,?,?,?,?,0,?,?,NULL)`, params.ParticipantID, item.ID, seat.ID, seat.Profile, seat.Agent,
		nullableString(seat.Model), seat.Machine, channel.Name, conversation.ParticipantActive, created); err != nil {
		return conversation.Turn{}, nil, "", err
	}
	primary, compatible, err := primaryChannelFromAgentBinding(channel.Binding, item.ID, params.ParticipantID, parseTime(created))
	if err != nil {
		return conversation.Turn{}, nil, "", err
	}
	if !compatible {
		return conversation.Turn{}, nil, "", fmt.Errorf("Agent Channel binding has no approved execution contract")
	}
	if !agentTargetAuthorityMatchesBinding(params.Authority, channel.Binding) {
		return conversation.Turn{}, nil, "", fmt.Errorf("agent_channel_invariant: target authority does not match Agent Channel %s", channel.ID)
	}
	if err := insertPrimaryChannelMarker(tx, primary); err != nil {
		return conversation.Turn{}, nil, "", err
	}
	if _, err := tx.Exec(`INSERT INTO agent_channel_conversation(agent_channel_id,conversation_id,created_at) VALUES(?,?,?)`, channel.ID, item.ID, created); err != nil {
		return conversation.Turn{}, nil, "", err
	}
	if _, err := tx.Exec(`INSERT INTO agent_channel_created_conversation(conversation_id) VALUES(?)`, item.ID); err != nil {
		return conversation.Turn{}, nil, "", err
	}

	turnCreated := nowOr(params.CreatedAt)
	messageResult, err := tx.Exec(`INSERT INTO conversation_message(conversation_id,turn_id,target_id,author_kind,author_id,body,created_at)
VALUES(?,?,NULL,?,?,?,?)`, item.ID, params.TurnID, string(conversation.AuthorHuman), params.HumanID, params.Body, turnCreated)
	if err != nil {
		return conversation.Turn{}, nil, "", err
	}
	messageID, err := messageResult.LastInsertId()
	if err != nil {
		return conversation.Turn{}, nil, "", err
	}
	contextJSON, err := conversationContextQuery(tx, item.ID, messageID)
	if err != nil {
		return conversation.Turn{}, nil, "", err
	}
	turn := conversation.Turn{
		ID: params.TurnID, ConversationID: item.ID, ClientTurnID: params.ClientTurnID,
		PromptMessageID: messageID, ThroughMessageID: messageID, CreatedAt: parseTime(turnCreated), Created: true,
	}
	if _, err := tx.Exec(`INSERT INTO conversation_turn(id,conversation_id,client_turn_id,prompt_message_id,through_message_id,context_json,created_at)
VALUES(?,?,?,?,?,?,?)`, turn.ID, turn.ConversationID, turn.ClientTurnID, messageID, messageID, contextJSON, turnCreated); err != nil {
		return conversation.Turn{}, nil, "", err
	}
	if params.Authority != nil {
		if err := params.Authority.Validate(); err != nil {
			return conversation.Turn{}, nil, "", err
		}
	}
	target := conversation.Target{
		ID: params.TargetID, TurnID: turn.ID, ParticipantID: params.ParticipantID, RunID: params.RunID,
		Attempt: 1, State: conversation.TargetQueued, Authority: cloneTargetAuthority(params.Authority),
		CreatedAt: parseTime(turnCreated), UpdatedAt: parseTime(turnCreated),
	}
	if err := insertConversationTarget(tx, target); err != nil {
		return conversation.Turn{}, nil, "", err
	}
	if _, err := tx.Exec(`UPDATE conversation SET updated_at=? WHERE id=?`, turnCreated, item.ID); err != nil {
		return conversation.Turn{}, nil, "", err
	}
	if err := tx.Commit(); err != nil {
		return conversation.Turn{}, nil, "", err
	}
	return turn, []conversation.Target{target}, contextJSON, nil
}

func (s *Store) GetAgentChannel(id string) (conversation.AgentChannelDetail, error) {
	channel, err := scanAgentChannel(s.db.QueryRow(`SELECT id,name,state,option_id,binding_json,created_at FROM agent_channel WHERE id=?`, id))
	if err != nil {
		return conversation.AgentChannelDetail{}, err
	}
	messageActivity := sqliteRFC3339NanoOrder("message.created_at")
	conversationCreated := sqliteRFC3339NanoOrder("child.created_at")
	pinnedOrder := sqliteRFC3339NanoOrder("pin.pinned_at")
	rows, err := s.db.Query(fmt.Sprintf(`SELECT child.id,pin.pinned_at
FROM agent_channel_conversation ownership
JOIN conversation child ON child.id=ownership.conversation_id
LEFT JOIN agent_conversation_pin pin ON pin.conversation_id=child.id
WHERE ownership.agent_channel_id=?
ORDER BY CASE WHEN pin.pinned_at IS NULL THEN 1 ELSE 0 END,
CASE WHEN pin.pinned_at IS NULL THEN '' ELSE %s END DESC,
CASE WHEN pin.pinned_at IS NULL THEN '' ELSE child.id END,
CASE WHEN pin.pinned_at IS NULL THEN COALESCE((SELECT MAX(%s) FROM conversation_message message WHERE message.conversation_id=child.id),%s) ELSE '' END DESC,
child.id`, pinnedOrder, messageActivity, conversationCreated), id)
	if err != nil {
		return conversation.AgentChannelDetail{}, err
	}
	type childKey struct {
		id       string
		pinnedAt sql.NullString
	}
	keys := []childKey{}
	for rows.Next() {
		var key childKey
		if err := rows.Scan(&key.id, &key.pinnedAt); err != nil {
			rows.Close()
			return conversation.AgentChannelDetail{}, err
		}
		keys = append(keys, key)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return conversation.AgentChannelDetail{}, err
	}
	if err := rows.Close(); err != nil {
		return conversation.AgentChannelDetail{}, err
	}
	out := conversation.AgentChannelDetail{Channel: channel, Conversations: make([]conversation.AgentConversationSummary, 0, len(keys))}
	for _, key := range keys {
		detail, err := s.GetConversation(key.id)
		if err != nil {
			return conversation.AgentChannelDetail{}, err
		}
		if len(detail.Participants) != 1 || detail.Participants[0].State != conversation.ParticipantActive {
			return conversation.AgentChannelDetail{}, fmt.Errorf("agent_channel_invariant: conversation %s", key.id)
		}
		expectedPrimary, compatible, identityErr := primaryChannelFromAgentBinding(
			channel.Binding, key.id, detail.Participants[0].ID, detail.Conversation.CreatedAt,
		)
		if identityErr != nil {
			return conversation.AgentChannelDetail{}, identityErr
		}
		if compatible {
			if detail.PrimaryChannel == nil || detail.PrimaryChannel.ConversationID != expectedPrimary.ConversationID ||
				detail.PrimaryChannel.ParticipantID != expectedPrimary.ParticipantID ||
				detail.PrimaryChannel.Authority != expectedPrimary.Authority || detail.PrimaryChannel.Policy != expectedPrimary.Policy {
				return conversation.AgentChannelDetail{}, fmt.Errorf("agent_channel_invariant: conversation %s authority", key.id)
			}
		} else if detail.PrimaryChannel != nil {
			return conversation.AgentChannelDetail{}, fmt.Errorf("agent_channel_invariant: conversation %s unexpected authority", key.id)
		}
		summary := conversation.AgentConversationSummary{
			Conversation: detail.Conversation,
			Participant:  detail.Participants[0],
			Pinned:       key.pinnedAt.Valid,
			PinnedAt:     parseTime(key.pinnedAt.String),
		}
		out.Conversations = append(out.Conversations, summary)
	}
	return out, nil
}

func (s *Store) SetAgentConversationPinned(channelID, conversationID string, pinned bool, pinnedAt time.Time) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var exists int
	if err := tx.QueryRow(`SELECT 1 FROM agent_channel_conversation WHERE agent_channel_id=? AND conversation_id=?`, channelID, conversationID).Scan(&exists); err != nil {
		return err
	}
	if pinned {
		_, err = tx.Exec(`INSERT INTO agent_conversation_pin(conversation_id,pinned_at) VALUES(?,?) ON CONFLICT(conversation_id) DO NOTHING`, conversationID, nowOr(pinnedAt))
	} else {
		_, err = tx.Exec(`DELETE FROM agent_conversation_pin WHERE conversation_id=?`, conversationID)
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}

func scanAgentChannel(row scanner) (conversation.AgentChannel, error) {
	var channel conversation.AgentChannel
	var state, bindingJSON, created string
	if err := row.Scan(&channel.ID, &channel.Name, &state, &channel.OptionID, &bindingJSON, &created); err != nil {
		return conversation.AgentChannel{}, err
	}
	if err := json.Unmarshal([]byte(bindingJSON), &channel.Binding); err != nil {
		return conversation.AgentChannel{}, fmt.Errorf("decode Agent Channel binding: %w", err)
	}
	channel.State = conversation.AgentChannelState(state)
	channel.CreatedAt = parseTime(created)
	return channel, nil
}

func primaryChannelFromAgentBinding(binding conversation.AgentBinding, conversationID, participantID string, createdAt time.Time) (conversation.PrimaryChannel, bool, error) {
	authority := binding.Authority
	seat := binding.Seat
	if seat.Agent != "codex-subscription" || authority.Authority != conversation.AuthorityChatSubscriptionIsolatedV1 ||
		authority.PolicyID != conversation.PolicyCodexSubscriptionChatV1 || authority.AdapterID != conversation.AdapterCodexSubscription ||
		authority.RuntimeContract != conversation.RuntimeContractCodexSubscriptionExecV1 {
		return conversation.PrimaryChannel{}, false, nil
	}
	timeout, err := strconv.Atoi(authority.ExecutionPolicy["request_timeout_millis"])
	if err != nil {
		return conversation.PrimaryChannel{}, false, fmt.Errorf("invalid Agent Channel request timeout: %w", err)
	}
	policy := conversation.SubscriptionPolicy{
		PolicyID: authority.PolicyID, PolicyRevision: authority.PolicyRevision,
		AdapterID: authority.AdapterID, AdapterRevision: authority.AdapterRevision,
		CodexVersion:                 authority.ExecutionPolicy["codex_version"],
		CodexExecutableRevision:      authority.ExecutionPolicy["codex_executable_revision"],
		CodexSchemaRevision:          authority.ExecutionPolicy["codex_schema_revision"],
		RuntimeContract:              authority.RuntimeContract,
		ReasoningEffort:              authority.ExecutionPolicy["reasoning_effort"],
		ReasoningContext:             authority.ExecutionPolicy["reasoning_context"],
		RequestTimeoutMillis:         timeout,
		DeveloperInstructionRevision: authority.ExecutionPolicy["developer_instruction_revision"],
		AccountType:                  authority.ExecutionPolicy["account_type"], AccountPlan: authority.ExecutionPolicy["account_plan"],
		ThreadMode:  authority.SessionMode,
		SandboxMode: authority.ExecutionPolicy["sandbox_mode"], ApprovalPolicy: authority.ExecutionPolicy["approval_policy"],
		WorkdirMode: authority.ExecutionPolicy["workdir_mode"], DynamicToolsMode: authority.ExecutionPolicy["dynamic_tools_mode"],
		MCPMode: authority.ExecutionPolicy["mcp_mode"], CommandPolicy: authority.ExecutionPolicy["command_policy"],
		FileReadPolicy: authority.ExecutionPolicy["file_read_policy"], IsolationRevision: authority.ExecutionPolicy["isolation_revision"],
	}
	primary := conversation.PrimaryChannel{
		ConversationID: conversationID, ParticipantID: participantID,
		Authority: authority.Authority, Policy: policy, CreatedAt: createdAt,
	}
	if err := primary.Validate(); err != nil {
		return conversation.PrimaryChannel{}, false, err
	}
	if authority.RequestedModel != seat.Model || seat.Profile != "codex-subscription:"+seat.Model ||
		authority.ResolvedModel != conversation.UnknownProviderIdentity || authority.MemoryMode != conversation.AgentMemoryEphemeral {
		return conversation.PrimaryChannel{}, false, fmt.Errorf("invalid Codex Agent Channel binding")
	}
	return primary, true, nil
}

func agentTargetAuthorityMatchesBinding(authority *conversation.TargetAuthority, binding conversation.AgentBinding) bool {
	if authority == nil {
		return false
	}
	primary, compatible, err := primaryChannelFromAgentBinding(binding, "agent-binding-check", "participant-binding-check", time.Time{})
	if err != nil || !compatible {
		return false
	}
	want := conversation.TargetAuthority{
		Authority: primary.Authority, Policy: primary.Policy, RequestedModel: binding.Authority.RequestedModel,
	}
	return *authority == want
}

func insertPrimaryChannelMarker(execer conversationTargetExecer, primary conversation.PrimaryChannel) error {
	values := []any{primary.ConversationID, primary.ParticipantID, primary.Authority}
	values = append(values, subscriptionPolicyValues(primary.Policy)...)
	values = append(values, nowOr(primary.CreatedAt))
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(values)), ",")
	_, err := execer.Exec(`INSERT INTO primary_channel(conversation_id,participant_id,authority,`+subscriptionPolicyColumns+`,created_at) VALUES(`+placeholders+`)`, values...)
	return err
}
