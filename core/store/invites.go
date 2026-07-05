package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Invite errors distinguish the join responses (spec 024): 401 vs 410.
var (
	ErrInviteInvalid = errors.New("store: invite invalid or already used")
	ErrInviteExpired = errors.New("store: invite expired")
)

// CreateInvite records a hashed single-use invite code.
func (s *Store) CreateInvite(codeHash string, expires time.Time) error {
	_, err := s.db.Exec(
		`INSERT INTO invite (code_hash, created_at, expires_at) VALUES (?, ?, ?)`,
		codeHash, nowOr(time.Time{}), expires.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("store: create invite: %w", err)
	}
	return nil
}

// CheckInvite verifies codeHash names an unused, unexpired invite. It does
// not consume it — the join flow persists the registry first and only then
// calls MarkInviteUsed (spec 024 ordering).
func (s *Store) CheckInvite(codeHash string, now time.Time) error {
	var expiresAt string
	var usedAt sql.NullString
	err := s.db.QueryRow(
		`SELECT expires_at, used_at FROM invite WHERE code_hash = ?`, codeHash,
	).Scan(&expiresAt, &usedAt)
	if err == sql.ErrNoRows {
		return ErrInviteInvalid
	}
	if err != nil {
		return fmt.Errorf("store: check invite: %w", err)
	}
	if usedAt.Valid {
		return ErrInviteInvalid
	}
	if now.UTC().After(parseTime(expiresAt)) {
		return ErrInviteExpired
	}
	return nil
}

// MarkInviteUsed consumes the invite. The WHERE used_at IS NULL guard makes
// consumption single-use even under concurrent joins.
func (s *Store) MarkInviteUsed(codeHash string, now time.Time) error {
	res, err := s.db.Exec(
		`UPDATE invite SET used_at = ? WHERE code_hash = ? AND used_at IS NULL`,
		now.UTC().Format(time.RFC3339Nano), codeHash,
	)
	if err != nil {
		return fmt.Errorf("store: mark invite used: %w", err)
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return ErrInviteInvalid
	}
	return nil
}
