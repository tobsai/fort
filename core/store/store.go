// Package store is Fort's SQLite state store (backlog AO-016, spec §6.6):
// run, node_run, route_decision, and an append-only event log. The event log
// is the source the fort-ui live feed replays from.
package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	Body        string // markdown body from a multiline compose (spec 031); "" if title-only
	Agent       string
	Status      string
	MatchedRule string
	Machine     string // resolved target host (spec 022); "" = local/single-machine
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
	NodeID    string // DAG step this event came from (spec 027); "" for run-level/single-run events
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
  id TEXT PRIMARY KEY, title TEXT, body TEXT, agent TEXT, status TEXT, matched_rule TEXT,
  machine TEXT, flow_id TEXT, exit_code INTEGER, error TEXT,
  created_at TEXT, updated_at TEXT
);
CREATE TABLE IF NOT EXISTS node_run (
  id TEXT PRIMARY KEY, run_id TEXT, node_id TEXT, type TEXT, status TEXT,
  input TEXT, output TEXT, attempts INTEGER, created_at TEXT, updated_at TEXT
);
CREATE INDEX IF NOT EXISTS idx_node_run_run ON node_run(run_id);
CREATE TABLE IF NOT EXISTS event (
  id INTEGER PRIMARY KEY AUTOINCREMENT, run_id TEXT, node_id TEXT, type TEXT, data TEXT,
  code INTEGER, created_at TEXT
);
CREATE INDEX IF NOT EXISTS idx_event_run ON event(run_id);
CREATE TABLE IF NOT EXISTS invite (
  code_hash TEXT PRIMARY KEY, created_at TEXT, expires_at TEXT, used_at TEXT
);
CREATE TABLE IF NOT EXISTS backlog_item (
  id TEXT PRIMARY KEY, title TEXT, body TEXT, agent TEXT, machine TEXT,
  labels TEXT, source TEXT, created_at TEXT
);
CREATE TABLE IF NOT EXISTS playbook_revision (
  id TEXT NOT NULL, revision INTEGER NOT NULL, data TEXT NOT NULL, created_at TEXT,
  PRIMARY KEY(id, revision)
);
CREATE INDEX IF NOT EXISTS idx_playbook_revision_latest ON playbook_revision(id, revision DESC);
`
	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("store: migrate: %w", err)
	}
	// Additive migrations for databases created before a column existed. Each is
	// idempotent (skipped when the column is already present).
	if err := s.addColumn("run", "machine", "TEXT"); err != nil {
		return fmt.Errorf("store: migrate run.machine: %w", err)
	}
	if err := s.addColumn("event", "node_id", "TEXT"); err != nil {
		return fmt.Errorf("store: migrate event.node_id: %w", err)
	}
	if err := s.addColumn("run", "body", "TEXT"); err != nil {
		return fmt.Errorf("store: migrate run.body: %w", err)
	}
	return nil
}

// addColumn adds col to table if it is not already present (idempotent).
func (s *Store) addColumn(table, col, typ string) error {
	rows, err := s.db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			cid, notnull, pk int
			name, ctype      string
			dflt             sql.NullString
		)
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		if strings.EqualFold(name, col) {
			return nil // already present
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = s.db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, col, typ))
	return err
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
		`INSERT INTO run(id,title,body,agent,status,matched_rule,machine,flow_id,exit_code,error,created_at,updated_at)
		 VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`,
		r.ID, r.Title, r.Body, r.Agent, r.Status, r.MatchedRule, r.Machine, r.FlowID, r.ExitCode, r.Error, now, now)
	return err
}

// UpdateRunStatus updates a run's terminal fields.
func (s *Store) UpdateRunStatus(id, status string, exitCode int, errMsg string) error {
	_, err := s.db.Exec(
		`UPDATE run SET status=?, exit_code=?, error=?, updated_at=? WHERE id=?`,
		status, exitCode, errMsg, nowOr(time.Time{}), id)
	return err
}

// FailInterruptedDirectRuns reconciles direct tasks left running by an earlier
// daemon lifetime. Flow runs are intentionally excluded: their durable
// node_run state is the input to graph.Resume after a restart.
func (s *Store) FailInterruptedDirectRuns(reason string) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	rows, err := tx.Query(
		`SELECT id FROM run
		 WHERE status='running' AND (flow_id IS NULL OR flow_id='')`,
	)
	if err != nil {
		return 0, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}

	now := nowOr(time.Time{})
	for _, id := range ids {
		if _, err := tx.Exec(
			`UPDATE run SET status='failed', exit_code=-1, error=?, updated_at=?
			 WHERE id=? AND status='running'`,
			reason, now, id,
		); err != nil {
			return 0, err
		}
		if _, err := tx.Exec(
			`INSERT INTO event(run_id,node_id,type,data,code,created_at)
			 VALUES(?,NULL,'error',?,-1,?)`,
			id, reason, now,
		); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(ids), nil
}

// GetRun returns a run by id.
func (s *Store) GetRun(id string) (Run, error) {
	row := s.db.QueryRow(
		`SELECT id,title,body,agent,status,matched_rule,machine,flow_id,exit_code,error,created_at,updated_at
		 FROM run WHERE id=?`, id)
	return scanRun(row)
}

// ListRuns returns all runs, newest first.
func (s *Store) ListRuns() ([]Run, error) {
	rows, err := s.db.Query(
		`SELECT id,title,body,agent,status,matched_rule,machine,flow_id,exit_code,error,created_at,updated_at
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
	var body, machine sql.NullString
	err := row.Scan(&r.ID, &r.Title, &body, &r.Agent, &r.Status, &r.MatchedRule, &machine, &r.FlowID,
		&r.ExitCode, &r.Error, &created, &updated)
	if err != nil {
		return Run{}, err
	}
	r.Body = body.String       // NULL (pre-migration rows) -> ""
	r.Machine = machine.String // NULL (pre-migration rows) -> ""
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

// AllNodeRuns returns every node_run row grouped by run (the board's
// checkpoint-summary source, spec 033).
func (s *Store) AllNodeRuns() ([]NodeRun, error) {
	rows, err := s.db.Query(
		`SELECT id,run_id,node_id,type,status,input,output,attempts,created_at,updated_at
		 FROM node_run ORDER BY run_id, created_at`)
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

// WaitingGates returns every gate node currently awaiting a human decision,
// across all runs (the gate-inbox source).
func (s *Store) WaitingGates() ([]NodeRun, error) {
	rows, err := s.db.Query(
		`SELECT id,run_id,node_id,type,status,input,output,attempts,created_at,updated_at
		 FROM node_run WHERE type='gate' AND status='waiting' ORDER BY created_at`)
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
		`INSERT INTO event(run_id,node_id,type,data,code,created_at) VALUES(?,?,?,?,?,?)`,
		e.RunID, e.NodeID, e.Type, e.Data, e.Code, nowOr(e.CreatedAt))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// Events returns all events for a run, in insertion order.
func (s *Store) Events(runID string) ([]Event, error) {
	return s.queryEvents(`SELECT id,run_id,node_id,type,data,code,created_at FROM event WHERE run_id=? ORDER BY id`, runID)
}

// EventsSince returns events with id greater than the cursor (the UI feed tail).
func (s *Store) EventsSince(cursor int64) ([]Event, error) {
	return s.queryEvents(`SELECT id,run_id,node_id,type,data,code,created_at FROM event WHERE id>? ORDER BY id`, cursor)
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
		var nodeID sql.NullString
		if err := rows.Scan(&e.ID, &e.RunID, &nodeID, &e.Type, &e.Data, &e.Code, &ts); err != nil {
			return nil, err
		}
		e.NodeID = nodeID.String
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
