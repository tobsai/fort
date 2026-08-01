package store

import (
	"database/sql"
	"time"

	"github.com/tobsai/fort/core/scheduler"
)

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
