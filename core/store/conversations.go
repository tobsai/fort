package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/tobsai/fort/core/conversation"
)

type ConversationDetail struct {
	Conversation   conversation.Conversation    `json:"conversation"`
	Participants   []conversation.Participant   `json:"participants"`
	Messages       []conversation.Message       `json:"messages"`
	Turns          []conversation.Turn          `json:"turns"`
	Targets        []conversation.Target        `json:"targets"`
	PrimaryChannel *conversation.PrimaryChannel `json:"primary_identity,omitempty"`
}

type ConversationTurnTarget struct {
	ID            string
	ParticipantID string
	RunID         string
	Authority     *conversation.TargetAuthority
}

type CreateConversationTurnParams struct {
	TurnID         string
	ClientTurnID   string
	ConversationID string
	HumanID        string
	Body           string
	Targets        []ConversationTurnTarget
	CreatedAt      time.Time
	// PrimarySingleFlight serializes the idempotency lookup and durable insert
	// before enforcing the one-active-target Primary Channel invariant. Legacy
	// conversation callers leave it false and retain their existing semantics.
	PrimarySingleFlight bool
}

type conversationTurnTransaction interface {
	rowsQueryer
	conversationTargetExecer
	QueryRow(query string, args ...any) *sql.Row
	Commit() error
	Rollback() error
}

type immediateConversationTurnTransaction struct {
	ctx  context.Context
	conn *sql.Conn
	done bool
}

func (s *Store) beginConversationTurnTransaction(immediate bool) (conversationTurnTransaction, error) {
	if !immediate {
		return s.db.Begin()
	}
	ctx := context.Background()
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := conn.ExecContext(ctx, `PRAGMA busy_timeout=5000`); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if _, err := conn.ExecContext(ctx, `BEGIN IMMEDIATE`); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return &immediateConversationTurnTransaction{ctx: ctx, conn: conn}, nil
}

func (tx *immediateConversationTurnTransaction) Exec(query string, args ...any) (sql.Result, error) {
	return tx.conn.ExecContext(tx.ctx, query, args...)
}

func (tx *immediateConversationTurnTransaction) Query(query string, args ...any) (*sql.Rows, error) {
	return tx.conn.QueryContext(tx.ctx, query, args...)
}

func (tx *immediateConversationTurnTransaction) QueryRow(query string, args ...any) *sql.Row {
	return tx.conn.QueryRowContext(tx.ctx, query, args...)
}

func (tx *immediateConversationTurnTransaction) Commit() error {
	if tx.done {
		return sql.ErrTxDone
	}
	if _, err := tx.conn.ExecContext(tx.ctx, `COMMIT`); err != nil {
		return err
	}
	tx.done = true
	// COMMIT is the durable boundary. A pooled connection-close failure cannot
	// turn an already committed queued target into an apparent creation failure,
	// because the caller would then skip dispatch and strand durable work.
	_ = tx.conn.Close()
	return nil
}

func (tx *immediateConversationTurnTransaction) Rollback() error {
	if tx.done {
		return sql.ErrTxDone
	}
	_, rollbackErr := tx.conn.ExecContext(tx.ctx, `ROLLBACK`)
	tx.done = true
	return errors.Join(rollbackErr, tx.conn.Close())
}

type ConversationTargetDispatch struct {
	Target       conversation.Target
	Turn         conversation.Turn
	Conversation conversation.Conversation
	Participant  conversation.Participant
}

// sqliteRFC3339NanoOrder normalizes Fort's UTC RFC3339Nano text to a fixed
// nine-digit fractional form. RFC3339Nano omits trailing zeroes, so raw TEXT
// ordering puts an exact second after a later fractional timestamp.
func sqliteRFC3339NanoOrder(column string) string {
	return fmt.Sprintf(`substr(%[1]s,1,19)||'.'||substr((CASE WHEN instr(%[1]s,'.')=0 THEN '' ELSE substr(%[1]s,21,length(%[1]s)-21) END)||'000000000',1,9)`, column)
}

func (s *Store) CreateProject(project conversation.Project) error {
	name, err := conversation.ValidateProjectName(project.Name)
	if err != nil {
		return err
	}
	project.Name = name
	now := nowOr(project.CreatedAt)
	updated := nowOr(project.UpdatedAt)
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := ensureProjectNameAvailable(tx, "", project.Name); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO project(id,name,created_at,updated_at) VALUES(?,?,?,?)`, project.ID, project.Name, now, updated); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ListProjects() ([]conversation.Project, error) {
	messageActivity := sqliteRFC3339NanoOrder("m.created_at")
	conversationCreated := sqliteRFC3339NanoOrder("c.created_at")
	projectCreated := sqliteRFC3339NanoOrder("p.created_at")
	rows, err := s.db.Query(fmt.Sprintf(`SELECT p.id,p.name,p.created_at,p.updated_at
	FROM project p
	LEFT JOIN conversation c ON c.project_id=p.id
	LEFT JOIN conversation_message m ON m.conversation_id=c.id
	GROUP BY p.id
	ORDER BY COALESCE(MAX(%s),MAX(%s),MAX(%s)) DESC,p.name COLLATE NOCASE,p.id`, messageActivity, conversationCreated, projectCreated))
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
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var exists int
	if err := tx.QueryRow(`SELECT 1 FROM project WHERE id=?`, id).Scan(&exists); err != nil {
		return err
	}
	if err := ensureProjectNameAvailable(tx, id, validated); err != nil {
		return err
	}
	result, err := tx.Exec(`UPDATE project SET name=?,updated_at=? WHERE id=?`, validated, nowOr(time.Time{}), id)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err == nil && changed != 1 {
		return sql.ErrNoRows
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}

func ensureProjectNameAvailable(tx *sql.Tx, exceptID, name string) error {
	rows, err := tx.Query(`SELECT id,name FROM project`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id, existing string
		if err := rows.Scan(&id, &existing); err != nil {
			return err
		}
		if id != exceptID && strings.EqualFold(existing, name) {
			return fmt.Errorf("UNIQUE constraint failed: project.name")
		}
	}
	return rows.Err()
}

func (s *Store) DeleteProject(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE conversation SET project_id=NULL WHERE project_id=?`, id); err != nil {
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
	query := `SELECT c.id,c.project_id,c.title,c.state,c.created_at,c.updated_at FROM conversation c`
	args := []any{}
	if scope == "inbox" {
		query += ` WHERE c.project_id IS NULL OR c.project_id=''`
	} else if scope != "" {
		query += ` WHERE c.project_id=?`
		args = append(args, scope)
	}
	messageActivity := sqliteRFC3339NanoOrder("m.created_at")
	conversationCreated := sqliteRFC3339NanoOrder("c.created_at")
	query += fmt.Sprintf(` ORDER BY COALESCE((SELECT MAX(%s) FROM conversation_message m WHERE m.conversation_id=c.id),%s) DESC,c.id`, messageActivity, conversationCreated)
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
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var active int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM conversation_target t
		JOIN conversation_turn tr ON tr.id=t.turn_id
		WHERE tr.conversation_id=? AND t.state IN (?,?)`, id, conversation.TargetQueued, conversation.TargetWorking).Scan(&active); err != nil {
		return err
	}
	if active != 0 {
		return fmt.Errorf("%w: %s", conversation.ErrConversationActive, id)
	}
	result, err := tx.Exec(`DELETE FROM conversation WHERE id=?`, id)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err == nil && changed != 1 {
		return sql.ErrNoRows
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) GetConversation(id string) (ConversationDetail, error) {
	item, err := scanConversation(s.db.QueryRow(`SELECT id,project_id,title,state,created_at,updated_at FROM conversation WHERE id=?`, id))
	if err != nil {
		return ConversationDetail{}, err
	}
	detail := ConversationDetail{Conversation: item, Participants: []conversation.Participant{}, Messages: []conversation.Message{}, Turns: []conversation.Turn{}, Targets: []conversation.Target{}}
	if detail.PrimaryChannel, err = optionalPrimaryChannel(s, id); err != nil {
		return ConversationDetail{}, err
	}
	if detail.Participants, err = s.conversationParticipants(id); err != nil {
		return ConversationDetail{}, err
	}
	if detail.PrimaryChannel != nil {
		if _, err := primaryChannelParticipant(detail.PrimaryChannel, detail.Participants); err != nil {
			return ConversationDetail{}, err
		}
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

func (s *Store) FindConversationTurnByClientID(conversationID, clientTurnID string) (conversation.Turn, []conversation.Target, bool, error) {
	var turn conversation.Turn
	var created string
	err := s.db.QueryRow(`SELECT id,conversation_id,client_turn_id,prompt_message_id,through_message_id,created_at
	FROM conversation_turn WHERE conversation_id=? AND client_turn_id=?`, conversationID, clientTurnID).Scan(
		&turn.ID, &turn.ConversationID, &turn.ClientTurnID, &turn.PromptMessageID, &turn.ThroughMessageID, &created,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return conversation.Turn{}, nil, false, nil
	}
	if err != nil {
		return conversation.Turn{}, nil, false, err
	}
	turn.CreatedAt = parseTime(created)
	targets, err := conversationTargetsForTurn(s.db, turn.ID)
	if err != nil {
		return conversation.Turn{}, nil, false, err
	}
	return turn, targets, true, nil
}

func (s *Store) CreateConversationTurn(params CreateConversationTurnParams) (conversation.Turn, []conversation.Target, string, error) {
	if len(params.Targets) == 0 {
		return conversation.Turn{}, nil, "", fmt.Errorf("conversation turn needs at least one target")
	}
	if params.ClientTurnID == "" {
		params.ClientTurnID = params.TurnID
	}
	tx, err := s.beginConversationTurnTransaction(params.PrimarySingleFlight)
	if err != nil {
		return conversation.Turn{}, nil, "", err
	}
	defer tx.Rollback()
	var existing conversation.Turn
	var existingCreated string
	var existingContext sql.NullString
	err = tx.QueryRow(`SELECT id,conversation_id,client_turn_id,prompt_message_id,through_message_id,created_at,context_json
FROM conversation_turn WHERE conversation_id=? AND client_turn_id=?`, params.ConversationID, params.ClientTurnID).Scan(
		&existing.ID, &existing.ConversationID, &existing.ClientTurnID, &existing.PromptMessageID, &existing.ThroughMessageID, &existingCreated, &existingContext,
	)
	if err == nil {
		existing.CreatedAt = parseTime(existingCreated)
		targets, targetErr := conversationTargetsForTurn(tx, existing.ID)
		if targetErr != nil {
			return conversation.Turn{}, nil, "", targetErr
		}
		if existingContext.Valid && existingContext.String != "" {
			return existing, targets, existingContext.String, nil
		}
		contextJSON, contextErr := conversationContextQuery(tx, params.ConversationID, existing.ThroughMessageID)
		if contextErr != nil {
			return conversation.Turn{}, nil, "", contextErr
		}
		if _, contextErr = tx.Exec(`UPDATE conversation_turn SET context_json=? WHERE id=? AND (context_json IS NULL OR context_json='')`, contextJSON, existing.ID); contextErr != nil {
			return conversation.Turn{}, nil, "", contextErr
		}
		if contextErr = tx.Commit(); contextErr != nil {
			return conversation.Turn{}, nil, "", contextErr
		}
		return existing, targets, contextJSON, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return conversation.Turn{}, nil, "", err
	}
	if params.PrimarySingleFlight {
		var marked, active int
		if err := tx.QueryRow(`SELECT
	EXISTS(SELECT 1 FROM primary_channel WHERE conversation_id=?),
	EXISTS(
	  SELECT 1
	  FROM conversation_target target
	  JOIN conversation_turn turn ON turn.id=target.turn_id
	  WHERE turn.conversation_id=? AND target.state IN (?,?)
	)`, params.ConversationID, params.ConversationID, conversation.TargetQueued, conversation.TargetWorking).Scan(&marked, &active); err != nil {
			return conversation.Turn{}, nil, "", err
		}
		if marked != 1 {
			return conversation.Turn{}, nil, "", fmt.Errorf("primary_channel_invariant: conversation %s is not a Primary Channel", params.ConversationID)
		}
		if active != 0 {
			return conversation.Turn{}, nil, "", conversation.NewBoundedError(
				conversation.ErrorConversationActive, conversation.ErrConversationActive,
			)
		}
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
	contextJSON, err := conversationContextQuery(tx, params.ConversationID, messageID)
	if err != nil {
		return conversation.Turn{}, nil, "", err
	}
	turn := conversation.Turn{ID: params.TurnID, ConversationID: params.ConversationID, ClientTurnID: params.ClientTurnID, PromptMessageID: messageID, ThroughMessageID: messageID, CreatedAt: parseTime(created), Created: true}
	insertTurn, err := tx.Exec(`INSERT INTO conversation_turn(id,conversation_id,client_turn_id,prompt_message_id,through_message_id,context_json,created_at)
	VALUES(?,?,?,?,?,?,?) ON CONFLICT(conversation_id,client_turn_id) DO NOTHING`, turn.ID, turn.ConversationID, turn.ClientTurnID, messageID, messageID, contextJSON, created)
	if err != nil {
		return conversation.Turn{}, nil, "", err
	}
	inserted, err := insertTurn.RowsAffected()
	if err != nil {
		return conversation.Turn{}, nil, "", err
	}
	if inserted == 0 {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			return conversation.Turn{}, nil, "", rollbackErr
		}
		return s.conversationTurnAfterClientConflict(
			params.ConversationID, params.ClientTurnID, errors.New("conversation client turn conflict"),
		)
	}
	targets := make([]conversation.Target, 0, len(params.Targets))
	for _, requested := range params.Targets {
		var participant conversation.Participant
		var model, removedAt sql.NullString
		var participantState, participantCreated string
		participantErr := tx.QueryRow(`SELECT id,conversation_id,seat_id,profile,agent,model,machine,display_name,position,state,created_at,removed_at FROM conversation_participant WHERE id=?`, requested.ParticipantID).Scan(
			&participant.ID, &participant.ConversationID, &participant.SeatID, &participant.Profile, &participant.Agent, &model,
			&participant.Machine, &participant.DisplayName, &participant.Position, &participantState, &participantCreated, &removedAt,
		)
		if errors.Is(participantErr, sql.ErrNoRows) {
			return conversation.Turn{}, nil, "", conversation.NewBoundedError(conversation.ErrorParticipantUnknown, fmt.Errorf("participant %s is not in conversation %s", requested.ParticipantID, params.ConversationID))
		}
		if participantErr != nil {
			return conversation.Turn{}, nil, "", participantErr
		}
		participant.Model, participant.State, participant.CreatedAt = model.String, conversation.ParticipantState(participantState), parseTime(participantCreated)
		participant.RemovedAt = parseTime(removedAt.String)
		if participant.ConversationID != params.ConversationID {
			return conversation.Turn{}, nil, "", conversation.NewBoundedError(conversation.ErrorParticipantUnknown, fmt.Errorf("participant %s is not in conversation %s", requested.ParticipantID, params.ConversationID))
		}
		if participant.State != conversation.ParticipantActive {
			return conversation.Turn{}, nil, "", conversation.NewBoundedError(conversation.ErrorParticipantRemoved, fmt.Errorf("participant %s is removed", requested.ParticipantID))
		}
		if _, err := conversation.CompileParticipantPrompt(contextJSON, participant); err != nil {
			return conversation.Turn{}, nil, "", err
		}
		if requested.Authority != nil {
			if err := requested.Authority.Validate(); err != nil {
				return conversation.Turn{}, nil, "", err
			}
		}
		target := conversation.Target{ID: requested.ID, TurnID: turn.ID, ParticipantID: requested.ParticipantID, RunID: requested.RunID, Attempt: 1, State: conversation.TargetQueued, Authority: cloneTargetAuthority(requested.Authority), CreatedAt: parseTime(created), UpdatedAt: parseTime(created)}
		if err := insertConversationTarget(tx, target); err != nil {
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

func (s *Store) conversationTurnAfterClientConflict(conversationID, clientTurnID string, conflict error) (conversation.Turn, []conversation.Target, string, error) {
	turn, targets, found, err := s.FindConversationTurnByClientID(conversationID, clientTurnID)
	if err != nil {
		return conversation.Turn{}, nil, "", err
	}
	if !found {
		return conversation.Turn{}, nil, "", conflict
	}
	var contextJSON sql.NullString
	if err := s.db.QueryRow(`SELECT context_json FROM conversation_turn WHERE id=?`, turn.ID).Scan(&contextJSON); err != nil {
		return conversation.Turn{}, nil, "", err
	}
	if !contextJSON.Valid || contextJSON.String == "" {
		return conversation.Turn{}, nil, "", fmt.Errorf("conversation turn %s has no frozen context", turn.ID)
	}
	return turn, targets, contextJSON.String, nil
}

func (s *Store) TransitionConversationTarget(id string, from, to conversation.TargetState, errorMessage string) (bool, error) {
	return s.TransitionConversationTargetWithCode(id, from, to, "", errorMessage)
}

func (s *Store) TransitionConversationTargetWithCode(id string, from, to conversation.TargetState, errorCode, errorMessage string) (bool, error) {
	return s.transitionConversationTarget(id, from, to, errorCode, errorMessage, nil)
}

func (s *Store) TransitionConversationTargetWithReceipt(id string, from, to conversation.TargetState, errorCode, errorMessage string, receipt conversation.TargetReceipt) (bool, error) {
	return s.transitionConversationTarget(id, from, to, errorCode, errorMessage, &receipt)
}

func (s *Store) transitionConversationTarget(id string, from, to conversation.TargetState, errorCode, errorMessage string, receipt *conversation.TargetReceipt) (bool, error) {
	if !conversation.CanTransition(from, to) {
		return false, fmt.Errorf("invalid conversation target transition %s -> %s", from, to)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	target, err := scanConversationTarget(tx.QueryRow(`SELECT `+conversationTargetColumns+` FROM conversation_target WHERE id=?`, id))
	if err != nil {
		return false, err
	}
	if target.State != from {
		return false, nil
	}
	terminal := to == conversation.TargetAnswered || to == conversation.TargetFailed || to == conversation.TargetCanceled
	if target.Authority != nil && terminal {
		if receipt == nil {
			return false, fmt.Errorf("subscription target terminal transition requires a typed receipt")
		}
		if err := receipt.ValidateFor(*target.Authority); err != nil {
			return false, err
		}
	} else if receipt != nil {
		return false, fmt.Errorf("target receipt is valid only for an authorized terminal transition")
	}
	query := `UPDATE conversation_target SET state=?,error_code=?,error=?,updated_at=?`
	args := []any{to, errorCode, errorMessage, nowOr(time.Time{})}
	if receipt != nil {
		query += `,observed_adapter_id=?,observed_adapter_revision=?,observed_codex_version=?,
observed_codex_executable_revision=?,observed_codex_schema_revision=?,resolved_model=?,provider_thread_id=?,
provider_terminal_status=?,usage_source=?,input_tokens=?,cached_input_tokens=?,output_tokens=?,reasoning_tokens=?`
		args = append(args, targetReceiptValues(receipt)...)
	}
	query += ` WHERE id=? AND state=?`
	args = append(args, id, from)
	result, err := tx.Exec(query, args...)
	if err != nil {
		return false, err
	}
	changed, err := result.RowsAffected()
	if err != nil || changed != 1 {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
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
	return s.answerConversationTarget(id, message, nil)
}

func (s *Store) AnswerConversationTargetWithReceipt(id string, message conversation.Message, receipt conversation.TargetReceipt) (bool, error) {
	return s.answerConversationTarget(id, message, &receipt)
}

func (s *Store) answerConversationTarget(id string, message conversation.Message, receipt *conversation.TargetReceipt) (bool, error) {
	if message.TargetID != id {
		return false, fmt.Errorf("answer target %s does not match message target %s", id, message.TargetID)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	target, err := scanConversationTarget(tx.QueryRow(`SELECT `+conversationTargetColumns+` FROM conversation_target WHERE id=?`, id))
	if err != nil {
		return false, err
	}
	if target.Authority != nil {
		if receipt == nil {
			return false, fmt.Errorf("subscription target answer requires a typed receipt")
		}
		if err := receipt.ValidateFor(*target.Authority); err != nil {
			return false, err
		}
	} else if receipt != nil {
		return false, fmt.Errorf("legacy target cannot persist a subscription receipt")
	}
	failAnswer := func(cause error) (bool, error) {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			cause = errors.Join(cause, rollbackErr)
		}
		var terminalErr error
		if receipt != nil {
			_, terminalErr = s.TransitionConversationTargetWithReceipt(
				id, conversation.TargetWorking, conversation.TargetFailed,
				"answer_persist_failed", "failed to persist attributed answer", *receipt,
			)
		} else {
			_, terminalErr = s.TransitionConversationTargetWithCode(
				id, conversation.TargetWorking, conversation.TargetFailed,
				"answer_persist_failed", "failed to persist attributed answer",
			)
		}
		if terminalErr != nil {
			cause = errors.Join(cause, terminalErr)
		}
		return false, cause
	}
	query := `UPDATE conversation_target SET state=?,error_code='',error='',updated_at=?`
	args := []any{conversation.TargetAnswered, nowOr(message.CreatedAt)}
	if receipt != nil {
		query += `,observed_adapter_id=?,observed_adapter_revision=?,observed_codex_version=?,
observed_codex_executable_revision=?,observed_codex_schema_revision=?,resolved_model=?,provider_thread_id=?,
provider_terminal_status=?,usage_source=?,input_tokens=?,cached_input_tokens=?,output_tokens=?,reasoning_tokens=?`
		args = append(args, targetReceiptValues(receipt)...)
	}
	query += ` WHERE id=? AND state=?`
	args = append(args, id, conversation.TargetWorking)
	result, err := tx.Exec(query, args...)
	if err != nil {
		return failAnswer(err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return failAnswer(err)
	}
	if changed != 1 {
		return false, err
	}
	if _, err := tx.Exec(`INSERT INTO conversation_message(conversation_id,turn_id,target_id,author_kind,author_id,body,created_at) VALUES(?,?,?,?,?,?,?)`,
		message.ConversationID, nullableString(message.TurnID), message.TargetID, message.AuthorKind, message.AuthorID, message.Body, nowOr(message.CreatedAt)); err != nil {
		return failAnswer(err)
	}
	if _, err := tx.Exec(`UPDATE conversation SET updated_at=? WHERE id=?`, nowOr(message.CreatedAt), message.ConversationID); err != nil {
		return failAnswer(err)
	}
	if err := tx.Commit(); err != nil {
		return failAnswer(err)
	}
	return true, nil
}

func (s *Store) FailInterruptedConversationTargets(reason string) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	rows, err := tx.Query(`SELECT id,run_id FROM conversation_target WHERE state IN (?,?) ORDER BY created_at,id`, conversation.TargetQueued, conversation.TargetWorking)
	if err != nil {
		return 0, err
	}
	type interruptedTarget struct{ id, runID string }
	var targets []interruptedTarget
	for rows.Next() {
		var target interruptedTarget
		if err := rows.Scan(&target.id, &target.runID); err != nil {
			rows.Close()
			return 0, err
		}
		targets = append(targets, target)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}

	now := nowOr(time.Time{})
	changed := 0
	for _, target := range targets {
		result, updateErr := tx.Exec(`UPDATE conversation_target SET
			state=?,error_code='daemon_interrupted',error=?,updated_at=?,
			observed_adapter_id=CASE WHEN authority=? THEN 'unknown' ELSE observed_adapter_id END,
			observed_adapter_revision=CASE WHEN authority=? THEN 'unknown' ELSE observed_adapter_revision END,
			observed_codex_version=CASE WHEN authority=? THEN 'unknown' ELSE observed_codex_version END,
			observed_codex_executable_revision=CASE WHEN authority=? THEN 'unknown' ELSE observed_codex_executable_revision END,
			observed_codex_schema_revision=CASE WHEN authority=? THEN 'unknown' ELSE observed_codex_schema_revision END,
			provider_terminal_status=CASE WHEN authority=? THEN 'daemon_interrupted' ELSE provider_terminal_status END,
			usage_source=CASE WHEN authority=? THEN 'unknown' ELSE usage_source END,
			input_tokens=CASE WHEN authority=? THEN 0 ELSE input_tokens END,
			cached_input_tokens=CASE WHEN authority=? THEN 0 ELSE cached_input_tokens END,
			output_tokens=CASE WHEN authority=? THEN 0 ELSE output_tokens END,
			reasoning_tokens=CASE WHEN authority=? THEN 0 ELSE reasoning_tokens END
			WHERE id=? AND state IN (?,?)`,
			conversation.TargetFailed, reason, now,
			conversation.AuthorityChatSubscriptionIsolatedV1, conversation.AuthorityChatSubscriptionIsolatedV1,
			conversation.AuthorityChatSubscriptionIsolatedV1, conversation.AuthorityChatSubscriptionIsolatedV1,
			conversation.AuthorityChatSubscriptionIsolatedV1, conversation.AuthorityChatSubscriptionIsolatedV1,
			conversation.AuthorityChatSubscriptionIsolatedV1, conversation.AuthorityChatSubscriptionIsolatedV1,
			conversation.AuthorityChatSubscriptionIsolatedV1, conversation.AuthorityChatSubscriptionIsolatedV1,
			conversation.AuthorityChatSubscriptionIsolatedV1,
			target.id, conversation.TargetQueued, conversation.TargetWorking)
		if updateErr != nil {
			return 0, updateErr
		}
		rowsAffected, rowsErr := result.RowsAffected()
		if rowsErr != nil {
			return 0, rowsErr
		}
		if rowsAffected == 0 {
			continue
		}
		changed++
		runResult, runErr := tx.Exec(`UPDATE run SET status='failed',exit_code=-1,error=?,updated_at=? WHERE id=? AND status IN ('queued','running')`, reason, now, target.runID)
		if runErr != nil {
			return 0, runErr
		}
		runChanged, runRowsErr := runResult.RowsAffected()
		if runRowsErr != nil {
			return 0, runRowsErr
		}
		if runChanged > 0 {
			if _, eventErr := tx.Exec(`INSERT INTO event(run_id,node_id,type,data,code,created_at) VALUES(?,NULL,'error',?,-1,?)`, target.runID, reason, now); eventErr != nil {
				return 0, eventErr
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return changed, nil
}

func (s *Store) GetConversationTargetDispatch(id string) (ConversationTargetDispatch, error) {
	target, err := scanConversationTarget(s.db.QueryRow(`SELECT `+conversationTargetColumns+` FROM conversation_target WHERE id=?`, id))
	if err != nil {
		return ConversationTargetDispatch{}, err
	}
	row := s.db.QueryRow(`SELECT
tr.conversation_id,tr.client_turn_id,tr.prompt_message_id,tr.through_message_id,tr.created_at,
c.project_id,c.title,c.state,c.created_at,c.updated_at,
p.seat_id,p.profile,p.agent,p.model,p.machine,p.display_name,p.position,p.state,p.created_at,p.removed_at
FROM conversation_turn tr
JOIN conversation c ON c.id=tr.conversation_id
JOIN conversation_participant p ON p.id=?
WHERE tr.id=?`, target.ParticipantID, target.TurnID)
	out := ConversationTargetDispatch{Target: target}
	var conversationState, participantState string
	var projectID, model, participantRemoved sql.NullString
	var turnCreated, conversationCreated, conversationUpdated, participantCreated string
	err = row.Scan(
		&out.Turn.ConversationID, &out.Turn.ClientTurnID, &out.Turn.PromptMessageID, &out.Turn.ThroughMessageID, &turnCreated,
		&projectID, &out.Conversation.Title, &conversationState, &conversationCreated, &conversationUpdated,
		&out.Participant.SeatID, &out.Participant.Profile, &out.Participant.Agent, &model, &out.Participant.Machine, &out.Participant.DisplayName, &out.Participant.Position, &participantState, &participantCreated, &participantRemoved,
	)
	if err != nil {
		return ConversationTargetDispatch{}, err
	}
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
	return s.retryConversationTarget(originalID, newID, newRunID, "", createdAt, false)
}

func (s *Store) RetryConversationTargetWithAdapterRevision(originalID, newID, newRunID, selectedAdapterRevision string, createdAt time.Time) (ConversationTargetDispatch, error) {
	if strings.TrimSpace(selectedAdapterRevision) == "" {
		return ConversationTargetDispatch{}, fmt.Errorf("selected adapter revision is required")
	}
	return s.retryConversationTarget(originalID, newID, newRunID, selectedAdapterRevision, createdAt, true)
}

func (s *Store) retryConversationTarget(originalID, newID, newRunID, selectedAdapterRevision string, createdAt time.Time, primarySingleFlight bool) (ConversationTargetDispatch, error) {
	tx, err := s.beginConversationTurnTransaction(primarySingleFlight)
	if err != nil {
		return ConversationTargetDispatch{}, err
	}
	defer tx.Rollback()
	original, err := scanConversationTarget(tx.QueryRow(`SELECT `+conversationTargetColumns+` FROM conversation_target WHERE id=?`, originalID))
	if err != nil {
		return ConversationTargetDispatch{}, err
	}
	if original.State != conversation.TargetFailed && original.State != conversation.TargetCanceled {
		return ConversationTargetDispatch{}, fmt.Errorf("target %s is %s; only failed or canceled targets can be retried", originalID, original.State)
	}
	retry := original
	retry.ID, retry.RunID, retry.Attempt, retry.State = newID, newRunID, original.Attempt+1, conversation.TargetQueued
	retry.ErrorCode, retry.Error, retry.Receipt = "", "", nil
	if selectedAdapterRevision != "" {
		if retry.Authority == nil {
			return ConversationTargetDispatch{}, fmt.Errorf("legacy target cannot select a subscription adapter revision")
		}
		retry.Authority = cloneTargetAuthority(retry.Authority)
		retry.Authority.Policy.AdapterRevision = selectedAdapterRevision
	}
	retry.CreatedAt, retry.UpdatedAt = createdAt, createdAt
	if err := insertConversationTarget(tx, retry); err != nil {
		return ConversationTargetDispatch{}, err
	}
	if err := tx.Commit(); err != nil {
		return ConversationTargetDispatch{}, err
	}
	return s.GetConversationTargetDispatch(newID)
}

func (s *Store) ConversationContext(conversationID string, throughMessageID int64) (string, error) {
	var turnID string
	var frozen sql.NullString
	err := s.db.QueryRow(`SELECT id,context_json FROM conversation_turn WHERE conversation_id=? AND through_message_id=? ORDER BY created_at,id LIMIT 1`, conversationID, throughMessageID).Scan(&turnID, &frozen)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	if err == nil && frozen.Valid && frozen.String != "" {
		return frozen.String, nil
	}
	contextJSON, err := conversationContextQuery(s.db, conversationID, throughMessageID)
	if err != nil {
		return "", err
	}
	if turnID != "" {
		if _, err := s.db.Exec(`UPDATE conversation_turn SET context_json=? WHERE id=? AND (context_json IS NULL OR context_json='')`, contextJSON, turnID); err != nil {
			return "", err
		}
	}
	return contextJSON, nil
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
	return conversationParticipantsQuery(s.db, id)
}

func conversationParticipantsQuery(queryer rowsQueryer, id string) ([]conversation.Participant, error) {
	rows, err := queryer.Query(`SELECT id,conversation_id,seat_id,profile,agent,model,machine,display_name,position,state,created_at,removed_at FROM conversation_participant WHERE conversation_id=? ORDER BY position,id`, id)
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
	rows, err := queryer.Query(`SELECT `+conversationTargetColumns+` FROM conversation_target WHERE turn_id=? ORDER BY created_at,id`, turnID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []conversation.Target{}
	for rows.Next() {
		target, err := scanConversationTarget(rows)
		if err != nil {
			return nil, err
		}
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

func conversationContextQuery(queryer rowsQueryer, conversationID string, throughMessageID int64) (string, error) {
	participants, err := conversationParticipantsQuery(queryer, conversationID)
	if err != nil {
		return "", err
	}
	messages, err := conversationMessagesQuery(queryer, conversationID)
	if err != nil {
		return "", err
	}
	return conversation.CompileContext(conversationID, throughMessageID, participants, messages)
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
	rows, err := s.db.Query(`SELECT `+qualifiedTargetColumns("t")+` FROM conversation_target t JOIN conversation_turn tr ON tr.id=t.turn_id WHERE tr.conversation_id=? ORDER BY t.created_at,t.id`, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []conversation.Target{}
	for rows.Next() {
		target, err := scanConversationTarget(rows)
		if err != nil {
			return nil, err
		}
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
