package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/sorin-work/bookshelf/internal/domain"
)

// HoldRow is a hold with its title, member and (once ready) copy.
type HoldRow struct {
	ID         int64
	TitleID    int64
	Title      string
	UserID     int64
	MemberName string
	CardNumber string
	Status     domain.HoldStatus
	PlacedAt   time.Time
	CopyID     int64
	Barcode    string
	ReadyOn    domain.Date
	ExpiresOn  domain.Date
	// Position is the place in the queue, starting at 1. Only set for
	// waiting holds.
	Position int
}

const holdSelect = `SELECT h.id, h.title_id, t.title, h.user_id, u.name, coalesce(u.card_number, ''),
	h.status, h.placed_at, coalesce(h.copy_id, 0), coalesce(c.barcode, ''),
	coalesce(h.ready_on, ''), coalesce(h.expires_on, ''),
	CASE WHEN h.status = 'waiting' THEN 1 + (SELECT count(*) FROM holds q
		WHERE q.title_id = h.title_id AND q.status = 'waiting'
		AND (q.placed_at < h.placed_at OR (q.placed_at = h.placed_at AND q.id < h.id))) ELSE 0 END
	FROM holds h
	JOIN titles t ON t.id = h.title_id
	JOIN users u ON u.id = h.user_id
	LEFT JOIN copies c ON c.id = h.copy_id `

func scanHold(row interface{ Scan(...any) error }) (HoldRow, error) {
	var h HoldRow
	var placed, ready, expires string
	err := row.Scan(&h.ID, &h.TitleID, &h.Title, &h.UserID, &h.MemberName, &h.CardNumber,
		&h.Status, &placed, &h.CopyID, &h.Barcode, &ready, &expires, &h.Position)
	if errors.Is(err, sql.ErrNoRows) {
		return HoldRow{}, ErrNotFound
	}
	if err != nil {
		return HoldRow{}, err
	}
	h.PlacedAt = parseTime(placed)
	h.ReadyOn, _ = domain.ParseDate(ready)
	h.ExpiresOn, _ = domain.ParseDate(expires)
	return h, nil
}

func queryHolds(ctx context.Context, q querier, where string, args ...any) ([]HoldRow, error) {
	rows, err := q.QueryContext(ctx, holdSelect+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []HoldRow
	for rows.Next() {
		h, err := scanHold(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

func (s *Store) HoldByID(ctx context.Context, id int64) (HoldRow, error) {
	return scanHold(s.db.QueryRowContext(ctx, holdSelect+`WHERE h.id = ?`, id))
}

// OpenHolds returns a member's waiting and ready holds, ready ones first.
func (s *Store) OpenHolds(ctx context.Context, userID int64) ([]HoldRow, error) {
	return queryHolds(ctx, s.db, `WHERE h.user_id = ? AND h.status IN ('waiting', 'ready')
		ORDER BY h.status = 'waiting', h.placed_at`, userID)
}

// HoldQueue returns the open holds on a title in queue order.
func (s *Store) HoldQueue(ctx context.Context, titleID int64) ([]HoldRow, error) {
	return queryHolds(ctx, s.db, `WHERE h.title_id = ? AND h.status IN ('waiting', 'ready')
		ORDER BY h.status = 'waiting', h.placed_at, h.id`, titleID)
}

// PlaceHold puts a member in the queue for a title.
func (s *Store) PlaceHold(ctx context.Context, userID, titleID int64, today domain.Date, now time.Time) (HoldRow, error) {
	var holdID int64
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		policy, err := loadPolicy(ctx, tx)
		if err != nil {
			return err
		}
		member, err := scanUser(tx.QueryRowContext(ctx, `SELECT `+userColumns+` FROM users WHERE id = ?`, userID))
		if err != nil {
			return err
		}
		req := domain.HoldRequest{CardExpires: member.CardExpires}
		err = tx.QueryRowContext(ctx, `SELECT
			(SELECT count(*) FROM copies WHERE title_id = ?1 AND status NOT IN ('lost', 'withdrawn')),
			(SELECT count(*) FROM copies WHERE title_id = ?1 AND status = 'available'),
			EXISTS (SELECT 1 FROM holds WHERE title_id = ?1 AND user_id = ?2 AND status IN ('waiting', 'ready')),
			EXISTS (SELECT 1 FROM loans l JOIN copies c ON c.id = l.copy_id
			        WHERE c.title_id = ?1 AND l.user_id = ?2 AND l.returned_at IS NULL),
			(SELECT count(*) FROM holds WHERE user_id = ?2 AND status IN ('waiting', 'ready'))`,
			titleID, userID).Scan(&req.Copies, &req.AvailableCopies, &req.HasOpenHold, &req.HasLoan, &req.OpenHolds)
		if err != nil {
			return err
		}
		var exists int
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM titles WHERE id = ?`, titleID).Scan(&exists); err != nil {
			return err
		}
		if exists == 0 {
			return ErrNotFound
		}
		if err := policy.CanPlaceHold(req, today); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, `INSERT INTO holds (title_id, user_id, placed_at) VALUES (?, ?, ?)`,
			titleID, userID, fmtTime(now))
		if err != nil {
			return fmt.Errorf("insert hold: %w", err)
		}
		holdID, err = res.LastInsertId()
		return err
	})
	if err != nil {
		return HoldRow{}, err
	}
	return s.HoldByID(ctx, holdID)
}
