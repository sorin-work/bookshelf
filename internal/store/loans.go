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
	ErrMemberInactive  = errors.New("this member's account has been closed")
	// ErrCopyHeldForOther means the copy is on the hold shelf for someone
	// else.
	ErrCopyHeldForOther = errors.New("this copy is set aside for another member")
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
	MemberEmail  string
	MemberPhone  string
	CardNumber   string
	CheckedOutAt time.Time
	Due          domain.Date
	Renewals     int
	ReturnedAt   time.Time
	// HoldsWaiting counts members still queued for the title, which
	// blocks renewals. Holds that already have a copy set aside do not.
	HoldsWaiting int
	CardExpires  domain.Date
	// Lost is set when the loan ended with the book reported lost.
	Lost bool
}

func (l LoanRow) Rules() domain.Loan {
	return domain.Loan{Due: l.Due, Renewals: l.Renewals, HoldsWaiting: l.HoldsWaiting, CardExpires: l.CardExpires}
}

func (l LoanRow) Returned() bool { return !l.ReturnedAt.IsZero() }

const loanSelect = `SELECT l.id, l.copy_id, c.barcode, t.id, t.title, u.id, u.name, u.email, u.phone,
	coalesce(u.card_number, ''), l.checked_out_at, l.due_on, l.renewals, coalesce(l.returned_at, ''),
	(SELECT count(*) FROM holds h WHERE h.title_id = t.id AND h.status = 'waiting'),
	coalesce(u.card_expires, ''), l.lost
	FROM loans l
	JOIN copies c ON c.id = l.copy_id
	JOIN titles t ON t.id = c.title_id
	JOIN users u ON u.id = l.user_id `

func scanLoan(row interface{ Scan(...any) error }) (LoanRow, error) {
	var l LoanRow
	var out, due, returned, cardExpires string
	err := row.Scan(&l.ID, &l.CopyID, &l.Barcode, &l.TitleID, &l.Title, &l.UserID, &l.MemberName,
		&l.MemberEmail, &l.MemberPhone, &l.CardNumber, &out, &due, &l.Renewals, &returned, &l.HoldsWaiting, &cardExpires, &l.Lost)
	if errors.Is(err, sql.ErrNoRows) {
		return LoanRow{}, ErrNotFound
	}
	if err != nil {
		return LoanRow{}, err
	}
	l.CheckedOutAt = parseTime(out)
	l.Due, _ = domain.ParseDate(due)
	l.CardExpires, _ = domain.ParseDate(cardExpires)
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
	loans, _, err := s.LoanHistoryPage(ctx, userID, limit, 0)
	return loans, err
}

// LoanHistoryPage returns one page of a member's returned loans and how
// many there are in all.
func (s *Store) LoanHistoryPage(ctx context.Context, userID int64, limit, offset int) ([]LoanRow, int, error) {
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM loans WHERE user_id = ? AND returned_at IS NOT NULL`,
		userID).Scan(&total); err != nil {
		return nil, 0, err
	}
	loans, err := queryLoans(ctx, s.db, `WHERE l.user_id = ? AND l.returned_at IS NOT NULL
		ORDER BY l.returned_at DESC, l.id DESC LIMIT ? OFFSET ?`, userID, limit, offset)
	return loans, total, err
}

// DueBetween lists open loans due from first to last inclusive.
func (s *Store) DueBetween(ctx context.Context, first, last domain.Date) ([]LoanRow, error) {
	return queryLoans(ctx, s.db, `WHERE l.returned_at IS NULL AND l.due_on BETWEEN ? AND ?
		ORDER BY l.due_on, u.name COLLATE NOCASE`, first.String(), last.String())
}

// OverdueLoans lists open loans due before today, longest overdue first.
func (s *Store) OverdueLoans(ctx context.Context, today domain.Date) ([]LoanRow, error) {
	return queryLoans(ctx, s.db, `WHERE l.returned_at IS NULL AND l.due_on < ?
		ORDER BY l.due_on, u.name COLLATE NOCASE`, today.String())
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
		member, err := userByID(ctx, tx, req.UserID)
		if err != nil {
			return err
		}
		if !member.Active {
			return ErrMemberInactive
		}
		c, err := copyByBarcode(ctx, tx, req.Barcode)
		if err != nil {
			return err
		}
		switch c.Status {
		case domain.CopyAvailable:
		case domain.CopyOnHoldShelf:
			var holder int64
			if err := tx.QueryRowContext(ctx, `SELECT user_id FROM holds WHERE copy_id = ? AND status = 'ready'`,
				c.ID).Scan(&holder); err != nil && !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			if holder != member.ID {
				return ErrCopyHeldForOther
			}
		case domain.CopyOnLoan:
			return ErrCopyOnLoan
		default:
			return ErrCopyUnavailable
		}

		b := domain.Borrower{CardExpires: member.CardExpires}
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM loans WHERE user_id = ? AND returned_at IS NULL`,
			member.ID).Scan(&b.ActiveLoans); err != nil {
			return err
		}
		if b.FinesOwed, err = finesOwed(ctx, tx, member.ID); err != nil {
			return err
		}
		if err := policy.CanBorrow(b, req.Today); err != nil {
			return err
		}

		if _, err := tx.ExecContext(ctx, `UPDATE copies SET status = 'on_loan' WHERE id = ?`, c.ID); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, `INSERT INTO loans (copy_id, user_id, checked_out_at, checked_out_by, due_on)
			VALUES (?, ?, ?, ?, ?)`, c.ID, member.ID, fmtTime(req.Now), sql.NullInt64{Int64: req.StaffID, Valid: req.StaffID != 0}, policy.DueDate(req.Today).String())
		if err != nil {
			return fmt.Errorf("insert loan: %w", err)
		}
		if loanID, err = res.LastInsertId(); err != nil {
			return err
		}
		// Borrowing a title closes the member's own hold on it, whether
		// they collected the copy set aside for them or another one.
		_, err = tx.ExecContext(ctx, `UPDATE holds SET status = 'fulfilled', closed_at = ?
			WHERE title_id = ? AND user_id = ? AND status IN ('waiting', 'ready')`,
			fmtTime(req.Now), c.TitleID, member.ID)
		return err
	})
	if err != nil {
		return LoanRow{}, err
	}
	return s.LoanByID(ctx, loanID)
}

// ReturnResult is a returned loan and, if the copy went to the hold shelf,
// the hold it is waiting for.
type ReturnResult struct {
	Loan LoanRow
	Hold *HoldRow
	// Fine is the late fine charged, in cents.
	Fine int
}

// Return checks a copy back in. If someone is waiting for the title the
// copy is set aside for them instead of going back on the shelf.
func (s *Store) Return(ctx context.Context, barcode string, today domain.Date, now time.Time) (ReturnResult, error) {
	var loanID, holdID int64
	var fine int
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		policy, err := loadPolicy(ctx, tx)
		if err != nil {
			return err
		}
		var copyID, userID int64
		var dueOn string
		err = tx.QueryRowContext(ctx, `SELECT l.id, l.copy_id, l.user_id, l.due_on FROM loans l
			JOIN copies c ON c.id = l.copy_id
			WHERE c.barcode = ? AND l.returned_at IS NULL`, barcode).Scan(&loanID, &copyID, &userID, &dueOn)
		if errors.Is(err, sql.ErrNoRows) {
			if _, err := copyByBarcode(ctx, tx, barcode); err != nil {
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
		due, err := domain.ParseDate(dueOn)
		if err != nil {
			return err
		}
		if fine = policy.LateFine(due, today); fine > 0 {
			if _, err := tx.ExecContext(ctx, `INSERT INTO fines (user_id, loan_id, kind, amount_cents, created_at)
				VALUES (?, ?, 'late', ?, ?)`, userID, loanID, fine, fmtTime(now)); err != nil {
				return err
			}
		}
		holdID, err = shelveCopy(ctx, tx, copyID, policy, today)
		return err
	})
	if err != nil {
		return ReturnResult{}, err
	}
	res := ReturnResult{Fine: fine}
	if res.Loan, err = s.LoanByID(ctx, loanID); err != nil {
		return ReturnResult{}, err
	}
	if holdID != 0 {
		h, err := s.HoldByID(ctx, holdID)
		if err != nil {
			return ReturnResult{}, err
		}
		res.Hold = &h
	}
	return res, nil
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

// DeclareLost ends a loan because the member lost the book. The copy is
// marked lost and, if charge is more than zero, a replacement charge is
// added to the member's account.
func (s *Store) DeclareLost(ctx context.Context, loanID int64, charge int, now time.Time) (LoanRow, error) {
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		var copyID, userID int64
		var barcode string
		err := tx.QueryRowContext(ctx, `SELECT l.copy_id, l.user_id, c.barcode FROM loans l
			JOIN copies c ON c.id = l.copy_id WHERE l.id = ? AND l.returned_at IS NULL`, loanID).Scan(&copyID, &userID, &barcode)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotOnLoan
		}
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE loans SET returned_at = ?, lost = 1 WHERE id = ?`, fmtTime(now), loanID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE copies SET status = 'lost' WHERE id = ?`, copyID); err != nil {
			return err
		}
		if charge > 0 {
			_, err = tx.ExecContext(ctx, `INSERT INTO fines (user_id, loan_id, kind, amount_cents, note, created_at)
				VALUES (?, ?, 'charge', ?, ?, ?)`, userID, loanID, charge, "Replacement for lost copy "+barcode, fmtTime(now))
		}
		return err
	})
	if err != nil {
		return LoanRow{}, err
	}
	return s.LoanByID(ctx, loanID)
}
