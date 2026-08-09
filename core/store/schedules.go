package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/tobsai/fort/core/scheduler"
)

var ErrScheduleCatalogLimit = errors.New("schedule_catalog_limit")

type ScheduleChannelLink struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ScheduleReadRow struct {
	Definition       scheduler.Definition
	LatestOccurrence *scheduler.Occurrence
	RelatedChannel   *ScheduleChannelLink
}

type ScheduleReadDetail struct {
	Row      ScheduleReadRow
	Upcoming []scheduler.Occurrence
	Recent   []scheduler.Occurrence
}

func (s *Store) CreateSchedule(definition scheduler.Definition) error {
	created, updated := nowOr(definition.CreatedAt), nowOr(definition.UpdatedAt)
	_, err := s.db.Exec(`INSERT INTO schedule(id,title,kind,expression,flow_id,timezone,enabled,next_fire_at,last_fire_at,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
		definition.ID, definition.Title, definition.Kind, definition.Expression, definition.FlowID, definition.Timezone,
		boolToInt(definition.Enabled), nullableTime(definition.NextFireAt), nullableTime(definition.LastFireAt), created, updated)
	return err
}

func (s *Store) ListSchedules() ([]scheduler.Definition, error) {
	rows, err := s.db.Query(`SELECT id,title,kind,expression,flow_id,timezone,enabled,next_fire_at,last_fire_at,created_at,updated_at FROM schedule ORDER BY created_at,id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []scheduler.Definition{}
	for rows.Next() {
		var definition scheduler.Definition
		var kind, created, updated string
		var nextFire, lastFire sql.NullString
		var enabled int
		if err := rows.Scan(&definition.ID, &definition.Title, &kind, &definition.Expression, &definition.FlowID, &definition.Timezone, &enabled, &nextFire, &lastFire, &created, &updated); err != nil {
			return nil, err
		}
		definition.Kind, definition.Enabled = scheduler.Kind(kind), enabled != 0
		definition.CreatedAt, definition.UpdatedAt = parseTime(created), parseTime(updated)
		definition.NextFireAt, definition.LastFireAt = parseTime(nextFire.String), parseTime(lastFire.String)
		out = append(out, definition)
	}
	return out, rows.Err()
}

// ReadScheduleCatalog returns one bounded SQLite read snapshot. enabled=nil
// includes active and paused definitions; a non-nil value filters on the
// durable enabled bit. The correlated occurrence is evidence only: this read
// never materializes, registers, or otherwise mutates scheduler state.
func (s *Store) ReadScheduleCatalog(ctx context.Context, enabled *bool, limit int) ([]ScheduleReadRow, error) {
	if limit < 1 {
		return nil, fmt.Errorf("schedule catalog limit must be positive")
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	where := ""
	args := []any{}
	if enabled != nil {
		where = " WHERE s.enabled=?"
		args = append(args, boolToInt(*enabled))
	}
	args = append(args, limit+1)
	rows, err := tx.QueryContext(ctx, scheduleReadSelect()+where+" ORDER BY s.id LIMIT ?", args...)
	if err != nil {
		return nil, err
	}
	items := []ScheduleReadRow{}
	for rows.Next() {
		item, scanErr := scanScheduleReadRow(rows)
		if scanErr != nil {
			rows.Close()
			return nil, scanErr
		}
		items = append(items, item)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(items) > limit {
		return nil, ErrScheduleCatalogLimit
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return items, nil
}

// ReadScheduleDetail returns the definition and both bounded occurrence
// projections from one read transaction at the caller's observed instant.
func (s *Store) ReadScheduleDetail(ctx context.Context, id string, observedAt time.Time, occurrenceLimit int) (ScheduleReadDetail, error) {
	if occurrenceLimit < 1 {
		return ScheduleReadDetail{}, fmt.Errorf("schedule occurrence limit must be positive")
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return ScheduleReadDetail{}, err
	}
	defer tx.Rollback()
	row, err := scanScheduleReadRow(tx.QueryRowContext(ctx, scheduleReadSelect()+" WHERE s.id=?", id))
	if err != nil {
		return ScheduleReadDetail{}, err
	}
	timeKey := sqliteTimeOrderValue(observedAt)
	upcoming, err := queryScheduleOccurrences(ctx, tx, `schedule_id=? AND `+sqliteRFC3339NanoOrder("scheduled_for")+`>=?`,
		sqliteRFC3339NanoOrder("scheduled_for")+" ASC,id ASC", occurrenceLimit, id, timeKey)
	if err != nil {
		return ScheduleReadDetail{}, err
	}
	recent, err := queryScheduleOccurrences(ctx, tx, `schedule_id=? AND `+sqliteRFC3339NanoOrder("scheduled_for")+`<?`,
		sqliteRFC3339NanoOrder("scheduled_for")+" DESC,id DESC", occurrenceLimit, id, timeKey)
	if err != nil {
		return ScheduleReadDetail{}, err
	}
	if err := tx.Commit(); err != nil {
		return ScheduleReadDetail{}, err
	}
	return ScheduleReadDetail{Row: row, Upcoming: upcoming, Recent: recent}, nil
}

// ReadScheduleOccurrences returns newest-first persisted occurrences. A cursor
// is absent only when both before and beforeID are empty; otherwise it is the
// exclusive (scheduled_for,id) tuple.
func (s *Store) ReadScheduleOccurrences(ctx context.Context, id string, limit int, before time.Time, beforeID string) ([]scheduler.Occurrence, error) {
	if limit < 1 {
		return nil, fmt.Errorf("schedule occurrence limit must be positive")
	}
	if before.IsZero() != (beforeID == "") {
		return nil, fmt.Errorf("schedule occurrence cursor requires before and before_id")
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var exists int
	if err := tx.QueryRowContext(ctx, `SELECT 1 FROM schedule WHERE id=?`, id).Scan(&exists); err != nil {
		return nil, err
	}
	predicate := `schedule_id=?`
	args := []any{id}
	if !before.IsZero() {
		orderTime := sqliteRFC3339NanoOrder("scheduled_for")
		predicate += ` AND (` + orderTime + `<? OR (` + orderTime + `=? AND id<?))`
		key := sqliteTimeOrderValue(before)
		args = append(args, key, key, beforeID)
	}
	items, err := queryScheduleOccurrences(ctx, tx, predicate, sqliteRFC3339NanoOrder("scheduled_for")+" DESC,id DESC", limit, args...)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return items, nil
}

func scheduleReadSelect() string {
	occurrenceOrder := sqliteRFC3339NanoOrder("candidate.scheduled_for")
	return `SELECT
		s.id,s.title,s.kind,s.expression,s.flow_id,s.timezone,s.enabled,s.next_fire_at,s.last_fire_at,s.created_at,s.updated_at,
		o.id,o.schedule_id,o.run_id,o.scheduled_for,o.state,o.error,o.created_at,o.updated_at,
		l.conversation_id,c.title
	FROM schedule s
	LEFT JOIN schedule_occurrence o ON o.id=(
		SELECT candidate.id FROM schedule_occurrence candidate
		WHERE candidate.schedule_id=s.id
		ORDER BY ` + occurrenceOrder + ` DESC,candidate.id DESC LIMIT 1
	)
	LEFT JOIN schedule_channel_link l ON l.schedule_id=s.id
	LEFT JOIN conversation c ON c.id=l.conversation_id`
}

func scanScheduleReadRow(row scanner) (ScheduleReadRow, error) {
	var item ScheduleReadRow
	var kind, created, updated string
	var enabled int
	var nextFire, lastFire sql.NullString
	var occurrenceID, scheduleID, runID, scheduledFor, state, occurrenceError, occurrenceCreated, occurrenceUpdated sql.NullString
	var channelID, channelName sql.NullString
	err := row.Scan(
		&item.Definition.ID, &item.Definition.Title, &kind, &item.Definition.Expression, &item.Definition.FlowID,
		&item.Definition.Timezone, &enabled, &nextFire, &lastFire, &created, &updated,
		&occurrenceID, &scheduleID, &runID, &scheduledFor, &state, &occurrenceError, &occurrenceCreated, &occurrenceUpdated,
		&channelID, &channelName,
	)
	if err != nil {
		return ScheduleReadRow{}, err
	}
	item.Definition.Kind, item.Definition.Enabled = scheduler.Kind(kind), enabled != 0
	item.Definition.NextFireAt, item.Definition.LastFireAt = parseTime(nextFire.String), parseTime(lastFire.String)
	item.Definition.CreatedAt, item.Definition.UpdatedAt = parseTime(created), parseTime(updated)
	if occurrenceID.Valid {
		item.LatestOccurrence = &scheduler.Occurrence{
			ID: occurrenceID.String, ScheduleID: scheduleID.String, RunID: runID.String,
			ScheduledFor: parseTime(scheduledFor.String), State: scheduler.OccurrenceState(state.String), Error: occurrenceError.String,
			CreatedAt: parseTime(occurrenceCreated.String), UpdatedAt: parseTime(occurrenceUpdated.String),
		}
	}
	if channelID.Valid {
		item.RelatedChannel = &ScheduleChannelLink{ID: channelID.String, Name: channelName.String}
	}
	return item, nil
}

type scheduleQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func queryScheduleOccurrences(ctx context.Context, queryer scheduleQueryer, predicate, order string, limit int, args ...any) ([]scheduler.Occurrence, error) {
	args = append(args, limit)
	rows, err := queryer.QueryContext(ctx, `SELECT id,schedule_id,run_id,scheduled_for,state,error,created_at,updated_at
		FROM schedule_occurrence WHERE `+predicate+` ORDER BY `+order+` LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []scheduler.Occurrence{}
	for rows.Next() {
		var item scheduler.Occurrence
		var runID, occurrenceError sql.NullString
		var scheduledFor, state, created, updated string
		if err := rows.Scan(&item.ID, &item.ScheduleID, &runID, &scheduledFor, &state, &occurrenceError, &created, &updated); err != nil {
			return nil, err
		}
		item.RunID, item.Error = runID.String, occurrenceError.String
		item.ScheduledFor, item.State = parseTime(scheduledFor), scheduler.OccurrenceState(state)
		item.CreatedAt, item.UpdatedAt = parseTime(created), parseTime(updated)
		items = append(items, item)
	}
	return items, rows.Err()
}

func sqliteTimeOrderValue(value time.Time) string {
	return value.UTC().Format("2006-01-02T15:04:05.000000000")
}

func (s *Store) UpdateScheduleFire(id string, lastFireAt, nextFireAt time.Time) error {
	_, err := s.db.Exec(`UPDATE schedule SET last_fire_at=?,next_fire_at=?,updated_at=? WHERE id=?`,
		nullableTime(lastFireAt), nullableTime(nextFireAt), nowOr(time.Time{}), id)
	return err
}

func (s *Store) UpsertScheduleOccurrence(occurrence scheduler.Occurrence) error {
	created, updated := nowOr(occurrence.CreatedAt), nowOr(occurrence.UpdatedAt)
	_, err := s.db.Exec(`INSERT INTO schedule_occurrence(id,schedule_id,run_id,scheduled_for,state,error,created_at,updated_at)
VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(id) DO NOTHING`,
		occurrence.ID, occurrence.ScheduleID, nullableString(occurrence.RunID), nowOr(occurrence.ScheduledFor), occurrence.State, nullableString(occurrence.Error), created, updated)
	return err
}

func (s *Store) ScheduleOccurrencesBetween(start, end time.Time) ([]scheduler.Occurrence, error) {
	rows, err := s.db.Query(`SELECT id,schedule_id,run_id,scheduled_for,state,error,created_at,updated_at FROM schedule_occurrence WHERE scheduled_for>=? AND scheduled_for<? ORDER BY scheduled_for,id`, nowOr(start), nowOr(end))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []scheduler.Occurrence{}
	for rows.Next() {
		var occurrence scheduler.Occurrence
		var runID, occurrenceError sql.NullString
		var scheduledFor, state, created, updated string
		if err := rows.Scan(&occurrence.ID, &occurrence.ScheduleID, &runID, &scheduledFor, &state, &occurrenceError, &created, &updated); err != nil {
			return nil, err
		}
		occurrence.RunID, occurrence.Error = runID.String, occurrenceError.String
		occurrence.ScheduledFor, occurrence.State = parseTime(scheduledFor), scheduler.OccurrenceState(state)
		occurrence.CreatedAt, occurrence.UpdatedAt = parseTime(created), parseTime(updated)
		out = append(out, occurrence)
	}
	return out, rows.Err()
}

func (s *Store) TransitionScheduleOccurrence(id string, from, to scheduler.OccurrenceState, runID, errorMessage string) (bool, error) {
	result, err := s.db.Exec(`UPDATE schedule_occurrence SET state=?,run_id=?,error=?,updated_at=? WHERE id=? AND state=?`,
		to, nullableString(runID), nullableString(errorMessage), nowOr(time.Time{}), id, from)
	if err != nil {
		return false, err
	}
	changed, err := result.RowsAffected()
	return changed == 1, err
}

func updateScheduleOccurrenceForRun(tx *sql.Tx, runID, runStatus, errorMessage, updatedAt string) error {
	var state scheduler.OccurrenceState
	switch runStatus {
	case "queued", "blocked", "paused":
		state = scheduler.OccurrenceFired
	case "running":
		state = scheduler.OccurrenceRunning
	case "succeeded", "completed":
		state = scheduler.OccurrenceSucceeded
	case "failed", "error":
		state = scheduler.OccurrenceFailed
	case "canceled", "cancelled":
		state = scheduler.OccurrenceCanceled
	default:
		return nil
	}
	if state != scheduler.OccurrenceFailed && state != scheduler.OccurrenceCanceled {
		errorMessage = ""
	}
	_, err := tx.Exec(`UPDATE schedule_occurrence SET state=?,error=?,updated_at=? WHERE run_id=?`,
		state, nullableString(errorMessage), updatedAt, runID)
	return err
}
