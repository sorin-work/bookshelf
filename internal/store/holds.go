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
		member, err := userByID(ctx, tx, userID)
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

// shelveCopy decides where a copy goes when it comes free: to the first
// waiting hold on its title (onto the hold shelf), or back on the shelf.
// It returns the hold the copy was set aside for, if any.
func shelveCopy(ctx context.Context, tx *sql.Tx, copyID int64, policy domain.Policy, today domain.Date) (int64, error) {
	var titleID int64
	if err := tx.QueryRowContext(ctx, `SELECT title_id FROM copies WHERE id = ?`, copyID).Scan(&titleID); err != nil {
		return 0, err
	}
	queue, err := waitingQueue(ctx, tx, titleID)
	if err != nil {
		return 0, err
	}
	next, ok := domain.NextInQueue(queue)
	if !ok {
		_, err := tx.ExecContext(ctx, `UPDATE copies SET status = 'available' WHERE id = ?`, copyID)
		return 0, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE holds SET status = 'ready', copy_id = ?, ready_on = ?, expires_on = ?
		WHERE id = ?`, copyID, today.String(), policy.PickupDeadline(today).String(), next.ID); err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE copies SET status = 'on_hold_shelf' WHERE id = ?`, copyID); err != nil {
		return 0, err
	}
	return next.ID, nil
}

func waitingQueue(ctx context.Context, q querier, titleID int64) ([]domain.QueuedHold, error) {
	rows, err := q.QueryContext(ctx, `SELECT id, placed_at FROM holds WHERE title_id = ? AND status = 'waiting'`, titleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.QueuedHold
	for rows.Next() {
		var h domain.QueuedHold
		var placed string
		if err := rows.Scan(&h.ID, &placed); err != nil {
			return nil, err
		}
		h.PlacedAt = parseTime(placed)
		out = append(out, h)
	}
	return out, rows.Err()
}

var ErrHoldClosed = errors.New("this hold is no longer open")

// CancelHold cancels an open hold. A copy that was waiting for it moves on
// to the next member in the queue.
func (s *Store) CancelHold(ctx context.Context, holdID int64, today domain.Date, now time.Time) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		policy, err := loadPolicy(ctx, tx)
		if err != nil {
			return err
		}
		var status domain.HoldStatus
		var copyID sql.NullInt64
		err = tx.QueryRowContext(ctx, `SELECT status, copy_id FROM holds WHERE id = ?`, holdID).Scan(&status, &copyID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if !status.Open() {
			return ErrHoldClosed
		}
		if _, err := tx.ExecContext(ctx, `UPDATE holds SET status = 'cancelled', closed_at = ? WHERE id = ?`,
			fmtTime(now), holdID); err != nil {
			return err
		}
		if status == domain.HoldReady && copyID.Valid {
			_, err = shelveCopy(ctx, tx, copyID.Int64, policy, today)
		}
		return err
	})
}

// ExpireHolds closes ready holds whose pickup deadline has passed and
// passes their copies on. It returns how many holds expired.
func (s *Store) ExpireHolds(ctx context.Context, today domain.Date, now time.Time) (int, error) {
	n := 0
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		policy, err := loadPolicy(ctx, tx)
		if err != nil {
			return err
		}
		type expired struct{ holdID, copyID int64 }
		var list []expired
		rows, err := tx.QueryContext(ctx, `SELECT id, copy_id FROM holds
			WHERE status = 'ready' AND expires_on < ? ORDER BY expires_on, id`, today.String())
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var e expired
			if err := rows.Scan(&e.holdID, &e.copyID); err != nil {
				return err
			}
			list = append(list, e)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		for _, e := range list {
			if _, err := tx.ExecContext(ctx, `UPDATE holds SET status = 'expired', closed_at = ? WHERE id = ?`,
				fmtTime(now), e.holdID); err != nil {
				return err
			}
			if _, err := shelveCopy(ctx, tx, e.copyID, policy, today); err != nil {
				return err
			}
			n++
		}
		return nil
	})
	return n, err
}

// HoldShelf lists the copies waiting to be collected, the ones expiring
// soonest first.
func (s *Store) HoldShelf(ctx context.Context) ([]HoldRow, error) {
	return queryHolds(ctx, s.db, `WHERE h.status = 'ready' ORDER BY h.expires_on, h.id`)
}

// WaitingHolds lists every hold still in a queue, oldest first.
func (s *Store) WaitingHolds(ctx context.Context, limit int) ([]HoldRow, error) {
	return queryHolds(ctx, s.db, `WHERE h.status = 'waiting' ORDER BY h.placed_at, h.id LIMIT ?`, limit)
}
