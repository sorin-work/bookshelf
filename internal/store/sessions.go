package store

import (
	"context"
	"time"
)

func (s *Store) CreateSession(ctx context.Context, tokenHash string, userID int64, now time.Time, ttl time.Duration) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sessions (token_hash, user_id, created_at, expires_at) VALUES (?, ?, ?, ?)`,
		tokenHash, userID, fmtTime(now), fmtTime(now.Add(ttl)))
	return err
}

// SessionUser returns the user a session token hash belongs to, as long as
// the session hasn't expired.
func (s *Store) SessionUser(ctx context.Context, tokenHash string, now time.Time) (User, error) {
	return scanUser(s.db.QueryRowContext(ctx, `SELECT `+userColumns+` FROM users
		WHERE id = (SELECT user_id FROM sessions WHERE token_hash = ? AND expires_at > ?)`,
		tokenHash, fmtTime(now)))
}

// DeleteExpiredSessions removes sessions that can no longer be used.
func (s *Store) DeleteExpiredSessions(ctx context.Context, now time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= ?`, fmtTime(now))
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *Store) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, tokenHash)
	return err
}
