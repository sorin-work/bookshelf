package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/sorin-work/bookshelf/internal/domain"
)

var (
	ErrCopyOnLoan      = errors.New("this copy is already checked out")
	ErrCopyUnavailable = errors.New("this copy is marked lost or withdrawn")
	ErrNotOnLoan       = errors.New("this copy is not checked out")
)

// LoanRow is a loan with the copy, title and member it involves.
type LoanRow struct {
	ID           int64
	CopyID       int64
	Barcode      string
	TitleID      int64
	Title        string
	UserID       int64
	MemberName   string
	CardNumber   string
	CheckedOutAt time.Time
	Due          domain.Date
	Renewals     int
	ReturnedAt   time.Time
}

func (l LoanRow) Rules() domain.Loan { return domain.Loan{Due: l.Due, Renewals: l.Renewals} }

func (l LoanRow) Returned() bool { return !l.ReturnedAt.IsZero() }

const loanSelect = `SELECT l.id, l.copy_id, c.barcode, t.id, t.title, u.id, u.name,
	coalesce(u.card_number, ''), l.checked_out_at, l.due_on, l.renewals, coalesce(l.returned_at, '')
	FROM loans l
	JOIN copies c ON c.id = l.copy_id
	JOIN titles t ON t.id = c.title_id
	JOIN users u ON u.id = l.user_id `

func scanLoan(row interface{ Scan(...any) error }) (LoanRow, error) {
	var l LoanRow
	var out, due, returned string
	err := row.Scan(&l.ID, &l.CopyID, &l.Barcode, &l.TitleID, &l.Title, &l.UserID, &l.MemberName,
		&l.CardNumber, &out, &due, &l.Renewals, &returned)
	if errors.Is(err, sql.ErrNoRows) {
		return LoanRow{}, ErrNotFound
	}
	if err != nil {
		return LoanRow{}, err
	}
	l.CheckedOutAt = parseTime(out)
	l.Due, _ = domain.ParseDate(due)
	if returned != "" {
		l.ReturnedAt = parseTime(returned)
	}
	return l, nil
}

func queryLoans(ctx context.Context, q querier, where string, args ...any) ([]LoanRow, error) {
	rows, err := q.QueryContext(ctx, loanSelect+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LoanRow
	for rows.Next() {
		l, err := scanLoan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (s *Store) LoanByID(ctx context.Context, id int64) (LoanRow, error) {
	return scanLoan(s.db.QueryRowContext(ctx, loanSelect+`WHERE l.id = ?`, id))
}

// ActiveLoans returns a member's open loans, soonest due first.
func (s *Store) ActiveLoans(ctx context.Context, userID int64) ([]LoanRow, error) {
	return queryLoans(ctx, s.db, `WHERE l.user_id = ? AND l.returned_at IS NULL ORDER BY l.due_on, l.id`, userID)
}

// LoanHistory returns a member's returned loans, most recent first.
func (s *Store) LoanHistory(ctx context.Context, userID int64, limit int) ([]LoanRow, error) {
	return queryLoans(ctx, s.db, `WHERE l.user_id = ? AND l.returned_at IS NOT NULL
		ORDER BY l.returned_at DESC LIMIT ?`, userID, limit)
}

type CheckoutRequest struct {
	UserID  int64
	Barcode string
	StaffID int64
	Today   domain.Date
	Now     time.Time
}

// Checkout lends a copy to a member, applying the lending policy.
func (s *Store) Checkout(ctx context.Context, req CheckoutRequest) (LoanRow, error) {
	var loanID int64
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		policy, err := loadPolicy(ctx, tx)
		if err != nil {
			return err
		}
		member, err := scanUser(tx.QueryRowContext(ctx, `SELECT `+userColumns+` FROM users WHERE id = ?`, req.UserID))
		if err != nil {
			return err
		}
		c, err := scanCopy(tx.QueryRowContext(ctx, `SELECT `+copyColumns+` FROM copies WHERE barcode = ?`, req.Barcode))
		if err != nil {
			return err
		}
		switch c.Status {
		case domain.CopyAvailable:
		case domain.CopyOnLoan:
			return ErrCopyOnLoan
		default:
			return ErrCopyUnavailable
		}

		var active int
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM loans WHERE user_id = ? AND returned_at IS NULL`,
			member.ID).Scan(&active); err != nil {
			return err
		}
		if err := policy.CanBorrow(domain.Borrower{CardExpires: member.CardExpires, ActiveLoans: active}, req.Today); err != nil {
			return err
		}

		if _, err := tx.ExecContext(ctx, `UPDATE copies SET status = 'on_loan' WHERE id = ?`, c.ID); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, `INSERT INTO loans (copy_id, user_id, checked_out_at, checked_out_by, due_on)
			VALUES (?, ?, ?, ?, ?)`, c.ID, member.ID, fmtTime(req.Now), req.StaffID, policy.DueDate(req.Today).String())
		if err != nil {
			return fmt.Errorf("insert loan: %w", err)
		}
		loanID, err = res.LastInsertId()
		return err
	})
	if err != nil {
		return LoanRow{}, err
	}
	return s.LoanByID(ctx, loanID)
}

// Return checks a copy back in.
func (s *Store) Return(ctx context.Context, barcode string, now time.Time) (LoanRow, error) {
	var loanID int64
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		err := tx.QueryRowContext(ctx, `SELECT l.id FROM loans l JOIN copies c ON c.id = l.copy_id
			WHERE c.barcode = ? AND l.returned_at IS NULL`, barcode).Scan(&loanID)
		if errors.Is(err, sql.ErrNoRows) {
			if _, err := scanCopy(tx.QueryRowContext(ctx, `SELECT `+copyColumns+` FROM copies WHERE barcode = ?`, barcode)); err != nil {
				return err
			}
			return ErrNotOnLoan
		}
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE loans SET returned_at = ? WHERE id = ?`, fmtTime(now), loanID); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `UPDATE copies SET status = 'available'
			WHERE id = (SELECT copy_id FROM loans WHERE id = ?)`, loanID)
		return err
	})
	if err != nil {
		return LoanRow{}, err
	}
	return s.LoanByID(ctx, loanID)
}

// Renew extends an open loan if the policy allows it.
func (s *Store) Renew(ctx context.Context, loanID int64, today domain.Date) (LoanRow, error) {
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		policy, err := loadPolicy(ctx, tx)
		if err != nil {
			return err
		}
		l, err := scanLoan(tx.QueryRowContext(ctx, loanSelect+`WHERE l.id = ?`, loanID))
		if err != nil {
			return err
		}
		if l.Returned() {
			return ErrNotOnLoan
		}
		if err := policy.CanRenew(l.Rules(), today); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `UPDATE loans SET due_on = ?, renewals = renewals + 1 WHERE id = ?`,
			policy.RenewedDue(l.Rules(), today).String(), loanID)
		return err
	})
	if err != nil {
		return LoanRow{}, err
	}
	return s.LoanByID(ctx, loanID)
}
