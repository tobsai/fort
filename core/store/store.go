// Package store is Fort's SQLite state store (backlog AO-016, spec §6.6):
// run, node_run, route_decision, and an append-only event log. The event log
// is the source the fort-ui live feed replays from.
package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite" // pure-Go driver, registered as "sqlite"
)

// Store wraps the SQLite database.
type Store struct{ db *sql.DB }

// RouteDecision is a persisted routing outcome.
type RouteDecision struct {
	ID          string
	TaskID      string
	Route       string
	MatchedRule string
	IsDefault   bool
	Reason      string
	CreatedAt   time.Time
}

// Run is a persisted execution (a routed task or a flow run).
type Run struct {
	ID          string
	Title       string
	Agent       string
	Status      string
	MatchedRule string
	FlowID      string
	ExitCode    int
	Error       string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NodeRun is a persisted DAG node execution (Phase 2).
type NodeRun struct {
	ID        string // runID:nodeID
	RunID     string
	NodeID    string
	Type      string
	Status    string
	Input     string
	Output    string
	Attempts  int
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Event is one append-only event row.
type Event struct {
	ID        int64
	RunID     string
	Type      string
	Data      string
	Code      int
	CreatedAt time.Time
}

// Open opens (creating if needed) the database at path and applies migrations.
func Open(path string) (*Store, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("store: mkdir %s: %w", dir, err)
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("store: open: %w", err)
	}
	db.SetMaxOpenConns(1) // serialize writes; SQLite single-writer
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	const schema = `
CREATE TABLE IF NOT EXISTS route_decision (
  id TEXT PRIMARY KEY, task_id TEXT, route TEXT, matched_rule TEXT,
  is_default INTEGER, reason TEXT, created_at TEXT
);
CREATE INDEX IF NOT EXISTS idx_route_decision_task ON route_decision(task_id);
CREATE TABLE IF NOT EXISTS run (
  id TEXT PRIMARY KEY, title TEXT, agent TEXT, status TEXT, matched_rule TEXT,
  flow_id TEXT, exit_code INTEGER, error TEXT, created_at TEXT, updated_at TEXT
);
CREATE TABLE IF NOT EXISTS node_run (
  id TEXT PRIMARY KEY, run_id TEXT, node_id TEXT, type TEXT, status TEXT,
  input TEXT, output TEXT, attempts INTEGER, created_at TEXT, updated_at TEXT
);
CREATE INDEX IF NOT EXISTS idx_node_run_run ON node_run(run_id);
CREATE TABLE IF NOT EXISTS event (
  id INTEGER PRIMARY KEY AUTOINCREMENT, run_id TEXT, type TEXT, data TEXT,
  code INTEGER, created_at TEXT
);
CREATE INDEX IF NOT EXISTS idx_event_run ON event(run_id);
`
	_, err := s.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("store: migrate: %w", err)
	}
	return nil
}

func nowOr(t time.Time) string {
	if t.IsZero() {
		t = time.Now()
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func parseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339Nano, s)
	return t
}

// SaveRouteDecision persists a routing decision.
func (s *Store) SaveRouteDecision(d RouteDecision) error {
	_, err := s.db.Exec(
		`INSERT INTO route_decision(id,task_id,route,matched_rule,is_default,reason,created_at)
		 VALUES(?,?,?,?,?,?,?)`,
		d.ID, d.TaskID, d.Route, d.MatchedRule, boolToInt(d.IsDefault), d.Reason, nowOr(d.CreatedAt))
	return err
}

// RouteDecisions returns the decisions recorded for a task, oldest first.
func (s *Store) RouteDecisions(taskID string) ([]RouteDecision, error) {
	rows, err := s.db.Query(
		`SELECT id,task_id,route,matched_rule,is_default,reason,created_at
		 FROM route_decision WHERE task_id=? ORDER BY created_at`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RouteDecision
	for rows.Next() {
		var d RouteDecision
		var isDef int
		var ts string
		if err := rows.Scan(&d.ID, &d.TaskID, &d.Route, &d.MatchedRule, &isDef, &d.Reason, &ts); err != nil {
			return nil, err
		}
		d.IsDefault = isDef != 0
		d.CreatedAt = parseTime(ts)
		out = append(out, d)
	}
	return out, rows.Err()
}

// CreateRun inserts a new run.
func (s *Store) CreateRun(r Run) error {
	now := nowOr(r.CreatedAt)
	_, err := s.db.Exec(
		`INSERT INTO run(id,title,agent,status,matched_rule,flow_id,exit_code,error,created_at,updated_at)
		 VALUES(?,?,?,?,?,?,?,?,?,?)`,
		r.ID, r.Title, r.Agent, r.Status, r.MatchedRule, r.FlowID, r.ExitCode, r.Error, now, now)
	return err
}

// UpdateRunStatus updates a run's terminal fields.
func (s *Store) UpdateRunStatus(id, status string, exitCode int, errMsg string) error {
	_, err := s.db.Exec(
		`UPDATE run SET status=?, exit_code=?, error=?, updated_at=? WHERE id=?`,
		status, exitCode, errMsg, nowOr(time.Time{}), id)
	return err
}

// GetRun returns a run by id.
func (s *Store) GetRun(id string) (Run, error) {
	row := s.db.QueryRow(
		`SELECT id,title,agent,status,matched_rule,flow_id,exit_code,error,created_at,updated_at
		 FROM run WHERE id=?`, id)
	return scanRun(row)
}

// ListRuns returns all runs, newest first.
func (s *Store) ListRuns() ([]Run, error) {
	rows, err := s.db.Query(
		`SELECT id,title,agent,status,matched_rule,flow_id,exit_code,error,created_at,updated_at
		 FROM run ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Run
	for rows.Next() {
		r, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type scanner interface {
	Scan(dest ...any) error
}

func scanRun(row scanner) (Run, error) {
	var r Run
	var created, updated string
	err := row.Scan(&r.ID, &r.Title, &r.Agent, &r.Status, &r.MatchedRule, &r.FlowID,
		&r.ExitCode, &r.Error, &created, &updated)
	if err != nil {
		return Run{}, err
	}
	r.CreatedAt = parseTime(created)
	r.UpdatedAt = parseTime(updated)
	return r, nil
}

// UpsertNodeRun inserts or updates a node run (keyed by id = runID:nodeID).
func (s *Store) UpsertNodeRun(n NodeRun) error {
	now := nowOr(time.Time{})
	_, err := s.db.Exec(
		`INSERT INTO node_run(id,run_id,node_id,type,status,input,output,attempts,created_at,updated_at)
		 VALUES(?,?,?,?,?,?,?,?,?,?)
		 ON CONFLICT(id) DO UPDATE SET status=excluded.status, input=excluded.input,
		   output=excluded.output, attempts=excluded.attempts, updated_at=excluded.updated_at`,
		n.ID, n.RunID, n.NodeID, n.Type, n.Status, n.Input, n.Output, n.Attempts, now, now)
	return err
}

// NodeRuns returns the node runs for a run, in creation order.
func (s *Store) NodeRuns(runID string) ([]NodeRun, error) {
	rows, err := s.db.Query(
		`SELECT id,run_id,node_id,type,status,input,output,attempts,created_at,updated_at
		 FROM node_run WHERE run_id=? ORDER BY created_at`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []NodeRun
	for rows.Next() {
		var n NodeRun
		var created, updated string
		if err := rows.Scan(&n.ID, &n.RunID, &n.NodeID, &n.Type, &n.Status,
			&n.Input, &n.Output, &n.Attempts, &created, &updated); err != nil {
			return nil, err
		}
		n.CreatedAt = parseTime(created)
		n.UpdatedAt = parseTime(updated)
		out = append(out, n)
	}
	return out, rows.Err()
}

// AppendEvent appends an event (append-only) and returns its id.
func (s *Store) AppendEvent(e Event) (int64, error) {
	res, err := s.db.Exec(
		`INSERT INTO event(run_id,type,data,code,created_at) VALUES(?,?,?,?,?)`,
		e.RunID, e.Type, e.Data, e.Code, nowOr(e.CreatedAt))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// Events returns all events for a run, in insertion order.
func (s *Store) Events(runID string) ([]Event, error) {
	return s.queryEvents(`SELECT id,run_id,type,data,code,created_at FROM event WHERE run_id=? ORDER BY id`, runID)
}

// EventsSince returns events with id greater than the cursor (the UI feed tail).
func (s *Store) EventsSince(cursor int64) ([]Event, error) {
	return s.queryEvents(`SELECT id,run_id,type,data,code,created_at FROM event WHERE id>? ORDER BY id`, cursor)
}

func (s *Store) queryEvents(q string, arg any) ([]Event, error) {
	rows, err := s.db.Query(q, arg)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Event
	for rows.Next() {
		var e Event
		var ts string
		if err := rows.Scan(&e.ID, &e.RunID, &e.Type, &e.Data, &e.Code, &ts); err != nil {
			return nil, err
		}
		e.CreatedAt = parseTime(ts)
		out = append(out, e)
	}
	return out, rows.Err()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
