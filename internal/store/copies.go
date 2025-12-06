package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/sorin-work/bookshelf/internal/domain"
)

type Copy struct {
	ID        int64
	TitleID   int64
	Barcode   string
	Status    domain.CopyStatus
	Shelf     string
	Note      string
	CreatedAt time.Time
}

type CopyInput struct {
	Barcode string
	Shelf   string
	Note    string
}

func (s *Store) CreateCopy(ctx context.Context, titleID int64, in CopyInput, now time.Time) (Copy, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO copies (title_id, barcode, shelf, note, created_at) VALUES (?, ?, ?, ?, ?)`,
		titleID, in.Barcode, in.Shelf, in.Note, fmtTime(now))
	if err != nil {
		if isUniqueViolation(err) {
			return Copy{}, ErrConflict
		}
		return Copy{}, fmt.Errorf("insert copy: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Copy{}, err
	}
	return s.CopyByID(ctx, id)
}

const copyColumns = `id, title_id, barcode, status, shelf, note, created_at`

func scanCopy(row interface{ Scan(...any) error }) (Copy, error) {
	var c Copy
	var created string
	if err := row.Scan(&c.ID, &c.TitleID, &c.Barcode, &c.Status, &c.Shelf, &c.Note, &created); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Copy{}, ErrNotFound
		}
		return Copy{}, err
	}
	c.CreatedAt = parseTime(created)
	return c, nil
}

func (s *Store) CopyByID(ctx context.Context, id int64) (Copy, error) {
	return scanCopy(s.db.QueryRowContext(ctx, `SELECT `+copyColumns+` FROM copies WHERE id = ?`, id))
}

func (s *Store) CopyByBarcode(ctx context.Context, barcode string) (Copy, error) {
	return scanCopy(s.db.QueryRowContext(ctx, `SELECT `+copyColumns+` FROM copies WHERE barcode = ?`, barcode))
}

func (s *Store) CopiesForTitle(ctx context.Context, titleID int64) ([]Copy, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+copyColumns+` FROM copies WHERE title_id = ? ORDER BY barcode`, titleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Copy
	for rows.Next() {
		c, err := scanCopy(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// SetCopyStatus moves a copy between the statuses a librarian controls by
// hand: available, lost and withdrawn. Copies on loan or on the hold shelf
// change status through circulation instead.
func (s *Store) SetCopyStatus(ctx context.Context, id int64, status domain.CopyStatus) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE copies SET status = ? WHERE id = ? AND status IN ('available', 'lost', 'withdrawn')`,
		string(status), id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
