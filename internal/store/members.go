package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/sorin-work/bookshelf/internal/domain"
)

type MemberFilter struct {
	Query  string
	Limit  int
	Offset int
}

// ListMembers returns accounts matching a name, email or card number,
// sorted by name, and the total number of matches.
func (s *Store) ListMembers(ctx context.Context, f MemberFilter) ([]User, int, error) {
	if f.Limit <= 0 {
		f.Limit = 25
	}
	where := ""
	var args []any
	if q := strings.TrimSpace(f.Query); q != "" {
		like := likeContains(q)
		card := strings.NewReplacer(" ", "", "-", "").Replace(q)
		where = `WHERE name LIKE ? ESCAPE '\' OR email LIKE ? ESCAPE '\' OR card_number LIKE ? ESCAPE '\'`
		args = append(args, like, like, likeContains(card))
	}
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM users `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+userColumns+` FROM users `+where+`
		ORDER BY name COLLATE NOCASE, id LIMIT ? OFFSET ?`, append(args, f.Limit, f.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, u)
	}
	return out, total, rows.Err()
}

type MemberInput struct {
	Name  string
	Email string
	Phone string
	Notes string
}

func (s *Store) UpdateMember(ctx context.Context, id int64, in MemberInput) error {
	res, err := s.db.ExecContext(ctx, `UPDATE users SET name = ?, email = ?, phone = ?, notes = ? WHERE id = ?`,
		strings.TrimSpace(in.Name), strings.TrimSpace(in.Email), strings.TrimSpace(in.Phone), strings.TrimSpace(in.Notes), id)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrConflict
		}
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) RenewCard(ctx context.Context, id int64, expires domain.Date) error {
	res, err := s.db.ExecContext(ctx, `UPDATE users SET card_expires = ? WHERE id = ?`, expires.String(), id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) SetRole(ctx context.Context, id int64, role Role) error {
	res, err := s.db.ExecContext(ctx, `UPDATE users SET role = ? WHERE id = ?`, string(role), id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) CountLibrarians(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM users WHERE role = 'librarian'`).Scan(&n)
	return n, err
}

var ErrHasLoans = errors.New("the member still has items on loan")

// SetActive deactivates or reactivates a member. Deactivating cancels
// their holds and signs them out; it is refused while they have items
// out.
func (s *Store) SetActive(ctx context.Context, id int64, active bool, today domain.Date, now time.Time) error {
	if active {
		res, err := s.db.ExecContext(ctx, `UPDATE users SET active = 1 WHERE id = ?`, id)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrNotFound
		}
		return nil
	}

	var loans int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM loans WHERE user_id = ? AND returned_at IS NULL`, id).Scan(&loans); err != nil {
		return err
	}
	if loans > 0 {
		return ErrHasLoans
	}
	holds, err := s.ids(ctx, `SELECT id FROM holds WHERE user_id = ? AND status IN ('waiting', 'ready')`, id)
	if err != nil {
		return err
	}
	for _, h := range holds {
		if err := s.CancelHold(ctx, h, today, now); err != nil {
			return err
		}
	}
	return s.inTx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `UPDATE users SET active = 0 WHERE id = ?`, id)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrNotFound
		}
		_, err = tx.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, id)
		return err
	})
}
