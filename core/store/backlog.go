package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// BacklogItem is a task queued for later dispatch (spec 025). It becomes a run
// only when dispatched (dragged onto the board / the Run action).
type BacklogItem struct {
	ID        string
	Title     string
	Body      string
	Agent     string // optional forced agent
	Machine   string // optional pinned host
	Labels    []string
	Source    string // "user" | "agent"
	CreatedAt time.Time
}

// CreateBacklogItem inserts a pending item.
func (s *Store) CreateBacklogItem(b BacklogItem) error {
	labels, _ := json.Marshal(b.Labels)
	_, err := s.db.Exec(
		`INSERT INTO backlog_item(id,title,body,agent,machine,labels,source,created_at)
		 VALUES(?,?,?,?,?,?,?,?)`,
		b.ID, b.Title, b.Body, b.Agent, b.Machine, string(labels), b.Source, nowOr(b.CreatedAt))
	if err != nil {
		return fmt.Errorf("store: create backlog item: %w", err)
	}
	return nil
}

// ListBacklog returns pending items, newest first.
func (s *Store) ListBacklog() ([]BacklogItem, error) {
	rows, err := s.db.Query(
		`SELECT id,title,body,agent,machine,labels,source,created_at
		 FROM backlog_item ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BacklogItem
	for rows.Next() {
		b, err := scanBacklog(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// GetBacklogItem returns one item by id.
func (s *Store) GetBacklogItem(id string) (BacklogItem, error) {
	row := s.db.QueryRow(
		`SELECT id,title,body,agent,machine,labels,source,created_at
		 FROM backlog_item WHERE id=?`, id)
	return scanBacklog(row)
}

// DeleteBacklogItem removes an item (called after it is dispatched or discarded).
func (s *Store) DeleteBacklogItem(id string) error {
	_, err := s.db.Exec(`DELETE FROM backlog_item WHERE id=?`, id)
	return err
}

func scanBacklog(row scanner) (BacklogItem, error) {
	var b BacklogItem
	var created string
	var labels sql.NullString
	if err := row.Scan(&b.ID, &b.Title, &b.Body, &b.Agent, &b.Machine, &labels, &b.Source, &created); err != nil {
		return BacklogItem{}, err
	}
	if labels.Valid && labels.String != "" {
		_ = json.Unmarshal([]byte(labels.String), &b.Labels)
	}
	b.CreatedAt = parseTime(created)
	return b, nil
}
