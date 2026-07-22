package store

import (
	"errors"
	"fmt"
	"time"
)

// ErrPlaybookRevisionStale identifies a failed compare-and-append. Callers can
// map it to a conflict without depending on SQLite or matching error text.
var ErrPlaybookRevisionStale = errors.New("store: stale playbook revision")

// PlaybookRevision stores one opaque, immutable definition revision. The core
// store owns durability while the bounded control adapter owns validation and
// JSON interpretation (spec 036).
type PlaybookRevision struct {
	ID        string
	Revision  int
	Data      string
	CreatedAt time.Time
}

// SeedPlaybookRevisions atomically installs an initial catalog when the table
// is empty. Existing catalogs are left untouched, so startup is idempotent and
// a crash can never expose a partially seeded set.
func (s *Store) SeedPlaybookRevisions(revisions []PlaybookRevision) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var count int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM playbook_revision`).Scan(&count); err != nil {
		return err
	}
	if count != 0 {
		return nil
	}
	for _, revision := range revisions {
		if revision.ID == "" || revision.Revision < 1 || revision.Data == "" {
			return fmt.Errorf("store: invalid initial playbook revision")
		}
		if _, err := tx.Exec(
			`INSERT INTO playbook_revision(id,revision,data,created_at) VALUES(?,?,?,?)`,
			revision.ID, revision.Revision, revision.Data, nowOr(revision.CreatedAt),
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// SavePlaybookRevision appends the next immutable revision for id.
func (s *Store) SavePlaybookRevision(id, data string) (PlaybookRevision, error) {
	return s.savePlaybookRevision(id, nil, data)
}

// SavePlaybookRevisionIfLatest appends only when expected is still the latest
// revision (zero means the id does not exist). The compare and insert share one
// transaction, preventing stale whole-document edits from becoming revisions.
func (s *Store) SavePlaybookRevisionIfLatest(id string, expected int, data string) (PlaybookRevision, error) {
	return s.savePlaybookRevision(id, &expected, data)
}

func (s *Store) savePlaybookRevision(id string, expected *int, data string) (PlaybookRevision, error) {
	if id == "" {
		return PlaybookRevision{}, fmt.Errorf("store: playbook id is required")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return PlaybookRevision{}, err
	}
	defer tx.Rollback()
	var latest int
	if err := tx.QueryRow(`SELECT COALESCE(MAX(revision), 0) FROM playbook_revision WHERE id=?`, id).Scan(&latest); err != nil {
		return PlaybookRevision{}, err
	}
	if expected != nil && latest != *expected {
		return PlaybookRevision{}, fmt.Errorf("%w for %q: latest %d, expected %d", ErrPlaybookRevisionStale, id, latest, *expected)
	}
	r := PlaybookRevision{ID: id, Revision: latest + 1, Data: data, CreatedAt: time.Now().UTC()}
	if _, err := tx.Exec(
		`INSERT INTO playbook_revision(id,revision,data,created_at) VALUES(?,?,?,?)`,
		r.ID, r.Revision, r.Data, nowOr(r.CreatedAt),
	); err != nil {
		return PlaybookRevision{}, err
	}
	if err := tx.Commit(); err != nil {
		return PlaybookRevision{}, err
	}
	return r, nil
}

// PlaybookRevision returns exactly revision; edits never rewrite old rows.
func (s *Store) PlaybookRevision(id string, revision int) (PlaybookRevision, error) {
	var r PlaybookRevision
	var created string
	err := s.db.QueryRow(
		`SELECT id,revision,data,created_at FROM playbook_revision WHERE id=? AND revision=?`,
		id, revision,
	).Scan(&r.ID, &r.Revision, &r.Data, &created)
	if err != nil {
		return PlaybookRevision{}, err
	}
	r.CreatedAt = parseTime(created)
	return r, nil
}

// LatestPlaybookRevisions returns the newest immutable revision for every id,
// ordered deterministically by id.
func (s *Store) LatestPlaybookRevisions() ([]PlaybookRevision, error) {
	rows, err := s.db.Query(`
		SELECT p.id,p.revision,p.data,p.created_at
		FROM playbook_revision p
		JOIN (
		  SELECT id,MAX(revision) AS revision
		  FROM playbook_revision GROUP BY id
		) latest ON latest.id=p.id AND latest.revision=p.revision
		ORDER BY p.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PlaybookRevision{}
	for rows.Next() {
		var r PlaybookRevision
		var created string
		if err := rows.Scan(&r.ID, &r.Revision, &r.Data, &created); err != nil {
			return nil, err
		}
		r.CreatedAt = parseTime(created)
		out = append(out, r)
	}
	return out, rows.Err()
}
