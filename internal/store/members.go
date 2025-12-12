package store

import (
	"context"
	"strings"

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
		like := "%" + q + "%"
		card := strings.NewReplacer(" ", "", "-", "").Replace(q)
		where = `WHERE name LIKE ? OR email LIKE ? OR card_number LIKE ?`
		args = append(args, like, like, "%"+card+"%")
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
}

func (s *Store) UpdateMember(ctx context.Context, id int64, in MemberInput) error {
	res, err := s.db.ExecContext(ctx, `UPDATE users SET name = ?, email = ?, phone = ? WHERE id = ?`,
		strings.TrimSpace(in.Name), strings.TrimSpace(in.Email), strings.TrimSpace(in.Phone), id)
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
