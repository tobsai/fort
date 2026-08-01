package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/tobsai/fort/core/conversation"
)

type ConversationDetail struct {
	Conversation conversation.Conversation  `json:"conversation"`
	Participants []conversation.Participant `json:"participants"`
	Messages     []conversation.Message     `json:"messages"`
	Turns        []conversation.Turn        `json:"turns"`
	Targets      []conversation.Target      `json:"targets"`
}

type ConversationTurnTarget struct {
	ID            string
	ParticipantID string
	RunID         string
}

type CreateConversationTurnParams struct {
	TurnID         string
	ClientTurnID   string
	ConversationID string
	HumanID        string
	Body           string
	Targets        []ConversationTurnTarget
	CreatedAt      time.Time
}

type ConversationTargetDispatch struct {
	Target       conversation.Target
	Turn         conversation.Turn
	Conversation conversation.Conversation
	Participant  conversation.Participant
}

func (s *Store) CreateProject(project conversation.Project) error {
	name, err := conversation.ValidateProjectName(project.Name)
	if err != nil {
		return err
	}
	project.Name = name
	now := nowOr(project.CreatedAt)
	updated := nowOr(project.UpdatedAt)
	_, err = s.db.Exec(`INSERT INTO project(id,name,created_at,updated_at) VALUES(?,?,?,?)`, project.ID, project.Name, now, updated)
	return err
}

func (s *Store) ListProjects() ([]conversation.Project, error) {
	rows, err := s.db.Query(`SELECT p.id,p.name,p.created_at,p.updated_at
FROM project p LEFT JOIN conversation c ON c.project_id=p.id
GROUP BY p.id ORDER BY COALESCE(MAX(c.updated_at),p.updated_at) DESC,p.name COLLATE NOCASE,p.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []conversation.Project{}
	for rows.Next() {
		var project conversation.Project
		var created, updated string
		if err := rows.Scan(&project.ID, &project.Name, &created, &updated); err != nil {
			return nil, err
		}
		project.CreatedAt, project.UpdatedAt = parseTime(created), parseTime(updated)
		out = append(out, project)
	}
	return out, rows.Err()
}

func (s *Store) RenameProject(id, name string) error {
	validated, err := conversation.ValidateProjectName(name)
	if err != nil {
		return err
	}
	result, err := s.db.Exec(`UPDATE project SET name=?,updated_at=? WHERE id=?`, validated, nowOr(time.Time{}), id)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err == nil && changed != 1 {
		return sql.ErrNoRows
	}
	return err
}

func (s *Store) DeleteProject(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE conversation SET project_id=NULL,updated_at=? WHERE project_id=?`, nowOr(time.Time{}), id); err != nil {
		return err
	}
	result, err := tx.Exec(`DELETE FROM project WHERE id=?`, id)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return sql.ErrNoRows
	}
	return tx.Commit()
}

func (s *Store) CreateConversation(item conversation.Conversation, participants []conversation.Participant) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	projectID := nullableString(item.ProjectID)
	if item.State == "" {
		item.State = conversation.ConversationOpen
	}
	created, updated := nowOr(item.CreatedAt), nowOr(item.UpdatedAt)
	if _, err := tx.Exec(`INSERT INTO conversation(id,project_id,title,state,created_at,updated_at) VALUES(?,?,?,?,?,?)`, item.ID, projectID, item.Title, item.State, created, updated); err != nil {
		return err
	}
	for _, participant := range participants {
		if participant.ConversationID != "" && participant.ConversationID != item.ID {
			return fmt.Errorf("participant %s belongs to conversation %s", participant.ID, participant.ConversationID)
		}
		if participant.State == "" {
			participant.State = conversation.ParticipantActive
		}
		if _, err := tx.Exec(`INSERT INTO conversation_participant(
id,conversation_id,seat_id,profile,agent,model,machine,display_name,position,state,created_at,removed_at
) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, participant.ID, item.ID, participant.SeatID, participant.Profile, participant.Agent,
			nullableString(participant.Model), participant.Machine, participant.DisplayName, participant.Position, participant.State,
			nowOr(participant.CreatedAt), nullableTime(participant.RemovedAt)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) AddConversationParticipant(participant conversation.Participant) error {
	if participant.State == "" {
		participant.State = conversation.ParticipantActive
	}
	_, err := s.db.Exec(`INSERT INTO conversation_participant(
id,conversation_id,seat_id,profile,agent,model,machine,display_name,position,state,created_at,removed_at
) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, participant.ID, participant.ConversationID, participant.SeatID, participant.Profile,
		participant.Agent, nullableString(participant.Model), participant.Machine, participant.DisplayName,
		participant.Position, participant.State, nowOr(participant.CreatedAt), nullableTime(participant.RemovedAt))
	return err
}

func (s *Store) ListConversations(scope string) ([]conversation.Conversation, error) {
	query := `SELECT id,project_id,title,state,created_at,updated_at FROM conversation`
	args := []any{}
	if scope == "inbox" {
		query += ` WHERE project_id IS NULL OR project_id=''`
	} else if scope != "" {
		query += ` WHERE project_id=?`
		args = append(args, scope)
	}
	query += ` ORDER BY updated_at DESC,id`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []conversation.Conversation{}
	for rows.Next() {
		item, err := scanConversation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) MoveConversation(id, projectID string) error {
	result, err := s.db.Exec(`UPDATE conversation SET project_id=? WHERE id=?`, nullableString(projectID), id)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err == nil && changed != 1 {
		return sql.ErrNoRows
	}
	return err
}

func (s *Store) RenameConversation(id, title string) error {
	result, err := s.db.Exec(`UPDATE conversation SET title=? WHERE id=?`, title, id)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err == nil && changed != 1 {
		return sql.ErrNoRows
	}
	return err
}

func (s *Store) SetConversationState(id string, state conversation.ConversationState) error {
	result, err := s.db.Exec(`UPDATE conversation SET state=?,updated_at=? WHERE id=?`, state, nowOr(time.Time{}), id)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err == nil && changed != 1 {
		return sql.ErrNoRows
	}
	return err
}

func (s *Store) RemoveConversationParticipant(conversationID, participantID string, removedAt time.Time) error {
	result, err := s.db.Exec(`UPDATE conversation_participant SET state=?,removed_at=? WHERE id=? AND conversation_id=? AND state=?`,
		conversation.ParticipantRemoved, nowOr(removedAt), participantID, conversationID, conversation.ParticipantActive)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err == nil && changed != 1 {
		return sql.ErrNoRows
	}
	return err
}

func (s *Store) DeleteConversation(id string) error {
	result, err := s.db.Exec(`DELETE FROM conversation WHERE id=?`, id)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err == nil && changed != 1 {
		return sql.ErrNoRows
	}
	return err
}

func (s *Store) GetConversation(id string) (ConversationDetail, error) {
	item, err := scanConversation(s.db.QueryRow(`SELECT id,project_id,title,state,created_at,updated_at FROM conversation WHERE id=?`, id))
	if err != nil {
		return ConversationDetail{}, err
	}
	detail := ConversationDetail{Conversation: item, Participants: []conversation.Participant{}, Messages: []conversation.Message{}, Turns: []conversation.Turn{}, Targets: []conversation.Target{}}
	if detail.Participants, err = s.conversationParticipants(id); err != nil {
		return ConversationDetail{}, err
	}
	if detail.Messages, err = s.conversationMessages(id); err != nil {
		return ConversationDetail{}, err
	}
	if detail.Turns, err = s.conversationTurns(id); err != nil {
		return ConversationDetail{}, err
	}
	if detail.Targets, err = s.conversationTargets(id); err != nil {
		return ConversationDetail{}, err
	}
	return detail, nil
}

func (s *Store) CreateConversationTurn(params CreateConversationTurnParams) (conversation.Turn, []conversation.Target, string, error) {
	if len(params.Targets) == 0 {
		return conversation.Turn{}, nil, "", fmt.Errorf("conversation turn needs at least one target")
	}
	if params.ClientTurnID == "" {
		params.ClientTurnID = params.TurnID
	}
	tx, err := s.db.Begin()
	if err != nil {
		return conversation.Turn{}, nil, "", err
	}
	defer tx.Rollback()
	var existing conversation.Turn
	var existingCreated string
	err = tx.QueryRow(`SELECT id,conversation_id,client_turn_id,prompt_message_id,through_message_id,created_at
FROM conversation_turn WHERE conversation_id=? AND client_turn_id=?`, params.ConversationID, params.ClientTurnID).Scan(
		&existing.ID, &existing.ConversationID, &existing.ClientTurnID, &existing.PromptMessageID, &existing.ThroughMessageID, &existingCreated,
	)
	if err == nil {
		existing.CreatedAt = parseTime(existingCreated)
		targets, targetErr := conversationTargetsForTurn(tx, existing.ID)
		if targetErr != nil {
			return conversation.Turn{}, nil, "", targetErr
		}
		messages, messageErr := conversationMessagesQuery(tx, params.ConversationID)
		if messageErr != nil {
			return conversation.Turn{}, nil, "", messageErr
		}
		contextJSON, contextErr := conversation.CompileContext(params.ConversationID, existing.ThroughMessageID, messages)
		if contextErr != nil {
			return conversation.Turn{}, nil, "", contextErr
		}
		return existing, targets, contextJSON, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return conversation.Turn{}, nil, "", err
	}
	created := nowOr(params.CreatedAt)
	result, err := tx.Exec(`INSERT INTO conversation_message(conversation_id,turn_id,target_id,author_kind,author_id,body,created_at)
VALUES(?,?,NULL,?,?,?,?)`, params.ConversationID, params.TurnID, string(conversation.AuthorHuman), params.HumanID, params.Body, created)
	if err != nil {
		return conversation.Turn{}, nil, "", err
	}
	messageID, err := result.LastInsertId()
	if err != nil {
		return conversation.Turn{}, nil, "", err
	}
	messages, err := conversationMessagesQuery(tx, params.ConversationID)
	if err != nil {
		return conversation.Turn{}, nil, "", err
	}
	contextJSON, err := conversation.CompileContext(params.ConversationID, messageID, messages)
	if err != nil {
		return conversation.Turn{}, nil, "", err
	}
	turn := conversation.Turn{ID: params.TurnID, ConversationID: params.ConversationID, ClientTurnID: params.ClientTurnID, PromptMessageID: messageID, ThroughMessageID: messageID, CreatedAt: parseTime(created), Created: true}
	if _, err := tx.Exec(`INSERT INTO conversation_turn(id,conversation_id,client_turn_id,prompt_message_id,through_message_id,created_at) VALUES(?,?,?,?,?,?)`, turn.ID, turn.ConversationID, turn.ClientTurnID, messageID, messageID, created); err != nil {
		return conversation.Turn{}, nil, "", err
	}
	targets := make([]conversation.Target, 0, len(params.Targets))
	for _, requested := range params.Targets {
		var participant conversation.Participant
		var model, removedAt sql.NullString
		var participantState, participantCreated string
		if err := tx.QueryRow(`SELECT id,conversation_id,seat_id,profile,agent,model,machine,display_name,position,state,created_at,removed_at FROM conversation_participant WHERE id=?`, requested.ParticipantID).Scan(
			&participant.ID, &participant.ConversationID, &participant.SeatID, &participant.Profile, &participant.Agent, &model,
			&participant.Machine, &participant.DisplayName, &participant.Position, &participantState, &participantCreated, &removedAt,
		); err != nil {
			return conversation.Turn{}, nil, "", err
		}
		participant.Model, participant.State, participant.CreatedAt = model.String, conversation.ParticipantState(participantState), parseTime(participantCreated)
		participant.RemovedAt = parseTime(removedAt.String)
		if participant.ConversationID != params.ConversationID {
			return conversation.Turn{}, nil, "", fmt.Errorf("participant %s is not in conversation %s", requested.ParticipantID, params.ConversationID)
		}
		if participant.State != conversation.ParticipantActive {
			return conversation.Turn{}, nil, "", fmt.Errorf("participant %s is removed", requested.ParticipantID)
		}
		if _, err := conversation.CompileParticipantPrompt(contextJSON, participant); err != nil {
			return conversation.Turn{}, nil, "", err
		}
		target := conversation.Target{ID: requested.ID, TurnID: turn.ID, ParticipantID: requested.ParticipantID, RunID: requested.RunID, Attempt: 1, State: conversation.TargetQueued, CreatedAt: parseTime(created), UpdatedAt: parseTime(created)}
		if _, err := tx.Exec(`INSERT INTO conversation_target(id,turn_id,participant_id,run_id,attempt,state,error_code,error,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?)`, target.ID, target.TurnID, target.ParticipantID, target.RunID, target.Attempt, target.State, "", "", created, created); err != nil {
			return conversation.Turn{}, nil, "", err
		}
		targets = append(targets, target)
	}
	if _, err := tx.Exec(`UPDATE conversation SET updated_at=? WHERE id=?`, created, params.ConversationID); err != nil {
		return conversation.Turn{}, nil, "", err
	}
	if err := tx.Commit(); err != nil {
		return conversation.Turn{}, nil, "", err
	}
	return turn, targets, contextJSON, nil
}

func (s *Store) TransitionConversationTarget(id string, from, to conversation.TargetState, errorMessage string) (bool, error) {
	return s.TransitionConversationTargetWithCode(id, from, to, "", errorMessage)
}

func (s *Store) TransitionConversationTargetWithCode(id string, from, to conversation.TargetState, errorCode, errorMessage string) (bool, error) {
	if !conversation.CanTransition(from, to) {
		return false, fmt.Errorf("invalid conversation target transition %s -> %s", from, to)
	}
	result, err := s.db.Exec(`UPDATE conversation_target SET state=?,error_code=?,error=?,updated_at=? WHERE id=? AND state=?`, to, errorCode, errorMessage, nowOr(time.Time{}), id, from)
	if err != nil {
		return false, err
	}
	changed, err := result.RowsAffected()
	return changed == 1, err
}

func (s *Store) TouchConversationTargetActivity(id string, observedAt time.Time) error {
	_, err := s.db.Exec(`UPDATE conversation_target SET updated_at=? WHERE id=? AND state=?`, nowOr(observedAt), id, conversation.TargetWorking)
	return err
}

func (s *Store) AppendConversationMessage(message conversation.Message) (conversation.Message, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return conversation.Message{}, err
	}
	defer tx.Rollback()
	created := nowOr(message.CreatedAt)
	result, err := tx.Exec(`INSERT INTO conversation_message(conversation_id,turn_id,target_id,author_kind,author_id,body,created_at) VALUES(?,?,?,?,?,?,?)`,
		message.ConversationID, nullableString(message.TurnID), nullableString(message.TargetID), message.AuthorKind, message.AuthorID, message.Body, created)
	if err != nil {
		return conversation.Message{}, err
	}
	message.ID, err = result.LastInsertId()
	if err != nil {
		return conversation.Message{}, err
	}
	if _, err := tx.Exec(`UPDATE conversation SET updated_at=? WHERE id=?`, created, message.ConversationID); err != nil {
		return conversation.Message{}, err
	}
	return message, tx.Commit()
}

func (s *Store) AnswerConversationTarget(id string, message conversation.Message) (bool, error) {
	if message.TargetID != id {
		return false, fmt.Errorf("answer target %s does not match message target %s", id, message.TargetID)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	result, err := tx.Exec(`UPDATE conversation_target SET state=?,error='',updated_at=? WHERE id=? AND state=?`,
		conversation.TargetAnswered, nowOr(message.CreatedAt), id, conversation.TargetWorking)
	if err != nil {
		return false, err
	}
	changed, err := result.RowsAffected()
	if err != nil || changed != 1 {
		return false, err
	}
	if _, err := tx.Exec(`INSERT INTO conversation_message(conversation_id,turn_id,target_id,author_kind,author_id,body,created_at) VALUES(?,?,?,?,?,?,?)`,
		message.ConversationID, nullableString(message.TurnID), message.TargetID, message.AuthorKind, message.AuthorID, message.Body, nowOr(message.CreatedAt)); err != nil {
		return false, err
	}
	if _, err := tx.Exec(`UPDATE conversation SET updated_at=? WHERE id=?`, nowOr(message.CreatedAt), message.ConversationID); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func (s *Store) FailInterruptedConversationTargets(reason string) (int, error) {
	result, err := s.db.Exec(`UPDATE conversation_target SET state=?,error_code='daemon_interrupted',error=?,updated_at=? WHERE state IN (?,?)`,
		conversation.TargetFailed, reason, nowOr(time.Time{}), conversation.TargetQueued, conversation.TargetWorking)
	if err != nil {
		return 0, err
	}
	changed, err := result.RowsAffected()
	return int(changed), err
}

func (s *Store) GetConversationTargetDispatch(id string) (ConversationTargetDispatch, error) {
	row := s.db.QueryRow(`SELECT
t.id,t.turn_id,t.participant_id,t.run_id,t.attempt,t.state,t.error_code,t.error,t.created_at,t.updated_at,
tr.conversation_id,tr.client_turn_id,tr.prompt_message_id,tr.through_message_id,tr.created_at,
c.project_id,c.title,c.state,c.created_at,c.updated_at,
p.seat_id,p.profile,p.agent,p.model,p.machine,p.display_name,p.position,p.state,p.created_at,p.removed_at
FROM conversation_target t
JOIN conversation_turn tr ON tr.id=t.turn_id
JOIN conversation c ON c.id=tr.conversation_id
JOIN conversation_participant p ON p.id=t.participant_id
WHERE t.id=?`, id)
	var out ConversationTargetDispatch
	var targetState, conversationState, participantState string
	var targetErrorCode, targetError, projectID, model, participantRemoved sql.NullString
	var targetCreated, targetUpdated, turnCreated, conversationCreated, conversationUpdated, participantCreated string
	err := row.Scan(
		&out.Target.ID, &out.Target.TurnID, &out.Target.ParticipantID, &out.Target.RunID, &out.Target.Attempt, &targetState, &targetErrorCode, &targetError, &targetCreated, &targetUpdated,
		&out.Turn.ConversationID, &out.Turn.ClientTurnID, &out.Turn.PromptMessageID, &out.Turn.ThroughMessageID, &turnCreated,
		&projectID, &out.Conversation.Title, &conversationState, &conversationCreated, &conversationUpdated,
		&out.Participant.SeatID, &out.Participant.Profile, &out.Participant.Agent, &model, &out.Participant.Machine, &out.Participant.DisplayName, &out.Participant.Position, &participantState, &participantCreated, &participantRemoved,
	)
	if err != nil {
		return ConversationTargetDispatch{}, err
	}
	out.Target.State, out.Target.ErrorCode, out.Target.Error = conversation.TargetState(targetState), targetErrorCode.String, targetError.String
	out.Target.CreatedAt, out.Target.UpdatedAt = parseTime(targetCreated), parseTime(targetUpdated)
	out.Turn.ID, out.Turn.CreatedAt = out.Target.TurnID, parseTime(turnCreated)
	out.Conversation.ID, out.Conversation.ProjectID = out.Turn.ConversationID, projectID.String
	out.Conversation.State = conversation.ConversationState(conversationState)
	out.Conversation.CreatedAt, out.Conversation.UpdatedAt = parseTime(conversationCreated), parseTime(conversationUpdated)
	out.Participant.ID, out.Participant.ConversationID, out.Participant.Model = out.Target.ParticipantID, out.Turn.ConversationID, model.String
	out.Participant.State, out.Participant.RemovedAt = conversation.ParticipantState(participantState), parseTime(participantRemoved.String)
	out.Participant.CreatedAt = parseTime(participantCreated)
	return out, nil
}

func (s *Store) RetryConversationTarget(originalID, newID, newRunID string, createdAt time.Time) (ConversationTargetDispatch, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return ConversationTargetDispatch{}, err
	}
	defer tx.Rollback()
	var turnID, participantID, state string
	var attempt int
	if err := tx.QueryRow(`SELECT turn_id,participant_id,attempt,state FROM conversation_target WHERE id=?`, originalID).Scan(&turnID, &participantID, &attempt, &state); err != nil {
		return ConversationTargetDispatch{}, err
	}
	if state != string(conversation.TargetFailed) && state != string(conversation.TargetCanceled) {
		return ConversationTargetDispatch{}, fmt.Errorf("target %s is %s; only failed or canceled targets can be retried", originalID, state)
	}
	created := nowOr(createdAt)
	if _, err := tx.Exec(`INSERT INTO conversation_target(id,turn_id,participant_id,run_id,attempt,state,error_code,error,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?)`,
		newID, turnID, participantID, newRunID, attempt+1, conversation.TargetQueued, "", "", created, created); err != nil {
		return ConversationTargetDispatch{}, err
	}
	if err := tx.Commit(); err != nil {
		return ConversationTargetDispatch{}, err
	}
	return s.GetConversationTargetDispatch(newID)
}

func (s *Store) ConversationContext(conversationID string, throughMessageID int64) (string, error) {
	messages, err := s.conversationMessages(conversationID)
	if err != nil {
		return "", err
	}
	return conversation.CompileContext(conversationID, throughMessageID, messages)
}

func (s *Store) ListConversationTargetDispatches(states ...conversation.TargetState) ([]ConversationTargetDispatch, error) {
	if len(states) == 0 {
		return []ConversationTargetDispatch{}, nil
	}
	query := `SELECT id FROM conversation_target WHERE state IN (`
	args := make([]any, 0, len(states))
	for i, state := range states {
		if i > 0 {
			query += ","
		}
		query += "?"
		args = append(args, state)
	}
	query += `) ORDER BY created_at,id`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	var ids []string
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
	out := make([]ConversationTargetDispatch, 0, len(ids))
	for _, id := range ids {
		item, err := s.GetConversationTargetDispatch(id)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

func scanConversation(row scanner) (conversation.Conversation, error) {
	var item conversation.Conversation
	var projectID sql.NullString
	var created, updated string
	var state string
	if err := row.Scan(&item.ID, &projectID, &item.Title, &state, &created, &updated); err != nil {
		return conversation.Conversation{}, err
	}
	item.ProjectID = projectID.String
	item.State = conversation.ConversationState(state)
	item.CreatedAt, item.UpdatedAt = parseTime(created), parseTime(updated)
	return item, nil
}

func (s *Store) conversationParticipants(id string) ([]conversation.Participant, error) {
	rows, err := s.db.Query(`SELECT id,conversation_id,seat_id,profile,agent,model,machine,display_name,position,state,created_at,removed_at FROM conversation_participant WHERE conversation_id=? ORDER BY position,id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []conversation.Participant{}
	for rows.Next() {
		var participant conversation.Participant
		var model, removedAt sql.NullString
		var state, created string
		if err := rows.Scan(&participant.ID, &participant.ConversationID, &participant.SeatID, &participant.Profile, &participant.Agent, &model, &participant.Machine, &participant.DisplayName, &participant.Position, &state, &created, &removedAt); err != nil {
			return nil, err
		}
		participant.Model, participant.State, participant.CreatedAt = model.String, conversation.ParticipantState(state), parseTime(created)
		participant.RemovedAt = parseTime(removedAt.String)
		out = append(out, participant)
	}
	return out, rows.Err()
}

func (s *Store) conversationMessages(id string) ([]conversation.Message, error) {
	return conversationMessagesQuery(s.db, id)
}

type rowsQueryer interface {
	Query(query string, args ...any) (*sql.Rows, error)
}

func conversationTargetsForTurn(queryer rowsQueryer, turnID string) ([]conversation.Target, error) {
	rows, err := queryer.Query(`SELECT id,turn_id,participant_id,run_id,attempt,state,error_code,error,created_at,updated_at FROM conversation_target WHERE turn_id=? ORDER BY created_at,id`, turnID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []conversation.Target{}
	for rows.Next() {
		var target conversation.Target
		var state, created, updated string
		var errorCode, targetError sql.NullString
		if err := rows.Scan(&target.ID, &target.TurnID, &target.ParticipantID, &target.RunID, &target.Attempt, &state, &errorCode, &targetError, &created, &updated); err != nil {
			return nil, err
		}
		target.State, target.ErrorCode, target.Error = conversation.TargetState(state), errorCode.String, targetError.String
		target.CreatedAt, target.UpdatedAt = parseTime(created), parseTime(updated)
		out = append(out, target)
	}
	return out, rows.Err()
}

func conversationMessagesQuery(queryer rowsQueryer, id string) ([]conversation.Message, error) {
	rows, err := queryer.Query(`SELECT id,conversation_id,turn_id,target_id,author_kind,author_id,body,created_at FROM conversation_message WHERE conversation_id=? ORDER BY id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []conversation.Message{}
	for rows.Next() {
		var message conversation.Message
		var turnID, targetID sql.NullString
		var authorKind, created string
		if err := rows.Scan(&message.ID, &message.ConversationID, &turnID, &targetID, &authorKind, &message.AuthorID, &message.Body, &created); err != nil {
			return nil, err
		}
		message.TurnID, message.TargetID, message.AuthorKind, message.CreatedAt = turnID.String, targetID.String, conversation.AuthorKind(authorKind), parseTime(created)
		out = append(out, message)
	}
	return out, rows.Err()
}

func (s *Store) conversationTurns(id string) ([]conversation.Turn, error) {
	rows, err := s.db.Query(`SELECT id,conversation_id,client_turn_id,prompt_message_id,through_message_id,created_at FROM conversation_turn WHERE conversation_id=? ORDER BY created_at,id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []conversation.Turn{}
	for rows.Next() {
		var turn conversation.Turn
		var created string
		if err := rows.Scan(&turn.ID, &turn.ConversationID, &turn.ClientTurnID, &turn.PromptMessageID, &turn.ThroughMessageID, &created); err != nil {
			return nil, err
		}
		turn.CreatedAt = parseTime(created)
		out = append(out, turn)
	}
	return out, rows.Err()
}

func (s *Store) conversationTargets(conversationID string) ([]conversation.Target, error) {
	rows, err := s.db.Query(`SELECT t.id,t.turn_id,t.participant_id,t.run_id,t.attempt,t.state,t.error_code,t.error,t.created_at,t.updated_at FROM conversation_target t JOIN conversation_turn tr ON tr.id=t.turn_id WHERE tr.conversation_id=? ORDER BY t.created_at,t.id`, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []conversation.Target{}
	for rows.Next() {
		var target conversation.Target
		var state, created, updated string
		var errorCode, targetError sql.NullString
		if err := rows.Scan(&target.ID, &target.TurnID, &target.ParticipantID, &target.RunID, &target.Attempt, &state, &errorCode, &targetError, &created, &updated); err != nil {
			return nil, err
		}
		target.State, target.ErrorCode, target.Error = conversation.TargetState(state), errorCode.String, targetError.String
		target.CreatedAt, target.UpdatedAt = parseTime(created), parseTime(updated)
		out = append(out, target)
	}
	return out, rows.Err()
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullableTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return nowOr(value)
}
