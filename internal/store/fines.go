package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type FineStatus string

const (
	FineOpen   FineStatus = "open"
	FinePaid   FineStatus = "paid"
	FineWaived FineStatus = "waived"
)

type Fine struct {
	ID         int64
	UserID     int64
	LoanID     int64
	Title      string
	Kind       string
	Amount     int
	Note       string
	Status     FineStatus
	CreatedAt  time.Time
	ResolvedAt time.Time
}

const fineSelect = `SELECT f.id, f.user_id, coalesce(f.loan_id, 0), coalesce(t.title, ''), f.kind,
	f.amount_cents, f.note, f.status, f.created_at, coalesce(f.resolved_at, '')
	FROM fines f
	LEFT JOIN loans l ON l.id = f.loan_id
	LEFT JOIN copies c ON c.id = l.copy_id
	LEFT JOIN titles t ON t.id = c.title_id `

func scanFine(row interface{ Scan(...any) error }) (Fine, error) {
	var f Fine
	var created, resolved string
	err := row.Scan(&f.ID, &f.UserID, &f.LoanID, &f.Title, &f.Kind, &f.Amount, &f.Note, &f.Status, &created, &resolved)
	if errors.Is(err, sql.ErrNoRows) {
		return Fine{}, ErrNotFound
	}
	if err != nil {
		return Fine{}, err
	}
	f.CreatedAt = parseTime(created)
	if resolved != "" {
		f.ResolvedAt = parseTime(resolved)
	}
	return f, nil
}

func (s *Store) FineByID(ctx context.Context, id int64) (Fine, error) {
	return scanFine(s.db.QueryRowContext(ctx, fineSelect+`WHERE f.id = ?`, id))
}

// Fines returns a member's fines, open ones first, newest first.
func (s *Store) Fines(ctx context.Context, userID int64, limit int) ([]Fine, error) {
	rows, err := s.db.QueryContext(ctx, fineSelect+`WHERE f.user_id = ?
		ORDER BY f.status != 'open', f.created_at DESC, f.id DESC LIMIT ?`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Fine
	for rows.Next() {
		f, err := scanFine(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// FinesOwed is the total of a member's open fines in cents.
func (s *Store) FinesOwed(ctx context.Context, userID int64) (int, error) {
	return finesOwed(ctx, s.db, userID)
}

func finesOwed(ctx context.Context, q querier, userID int64) (int, error) {
	var n int
	err := q.QueryRowContext(ctx, `SELECT coalesce(sum(amount_cents), 0) FROM fines
		WHERE user_id = ? AND status = 'open'`, userID).Scan(&n)
	return n, err
}

// AddCharge records a fee a librarian charges by hand, for a damaged or
// lost book for example.
func (s *Store) AddCharge(ctx context.Context, userID int64, amount int, note string, now time.Time) (Fine, error) {
	res, err := s.db.ExecContext(ctx, `INSERT INTO fines (user_id, kind, amount_cents, note, created_at)
		VALUES (?, 'charge', ?, ?, ?)`, userID, amount, note, fmtTime(now))
	if err != nil {
		return Fine{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Fine{}, err
	}
	return s.FineByID(ctx, id)
}

var ErrFineClosed = errors.New("this fine has already been paid or waived")

// ResolveFine marks an open fine as paid or waived.
func (s *Store) ResolveFine(ctx context.Context, id int64, status FineStatus, by int64, now time.Time) error {
	if status != FinePaid && status != FineWaived {
		return errors.New("a fine can only be paid or waived")
	}
	res, err := s.db.ExecContext(ctx, `UPDATE fines SET status = ?, resolved_at = ?, resolved_by = ?
		WHERE id = ? AND status = 'open'`, string(status), fmtTime(now), by, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		if _, err := s.FineByID(ctx, id); err != nil {
			return err
		}
		return ErrFineClosed
	}
	return nil
}
