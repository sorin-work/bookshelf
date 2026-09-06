package store

import (
	"context"
	"database/sql"
	"errors"
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
