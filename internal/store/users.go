package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Role string

const (
	RoleLibrarian Role = "librarian"
	RoleMember    Role = "member"
)

type User struct {
	ID           int64
	Email        string
	Name         string
	PasswordHash string
	Role         Role
	CreatedAt    time.Time
}

func (u User) IsLibrarian() bool { return u.Role == RoleLibrarian }

type NewUser struct {
	Email        string
	Name         string
	PasswordHash string
	Role         Role
}

// CreateUser adds a user. The very first account becomes a librarian no
// matter what role was asked for: whoever installs the app runs it.
func (s *Store) CreateUser(ctx context.Context, u NewUser, now time.Time) (User, error) {
	if u.Role == "" {
		u.Role = RoleMember
	}
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO users (email, name, password_hash, role, created_at)
		 VALUES (?, ?, ?, CASE WHEN EXISTS (SELECT 1 FROM users) THEN ? ELSE 'librarian' END, ?)`,
		strings.TrimSpace(u.Email), strings.TrimSpace(u.Name), u.PasswordHash, string(u.Role), fmtTime(now))
	if err != nil {
		if isUniqueViolation(err) {
			return User{}, ErrConflict
		}
		return User{}, fmt.Errorf("create user: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return User{}, err
	}
	return s.UserByID(ctx, id)
}

const userColumns = `id, email, name, password_hash, role, created_at`

func scanUser(row interface{ Scan(...any) error }) (User, error) {
	var u User
	var created string
	if err := row.Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.Role, &created); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrNotFound
		}
		return User{}, err
	}
	u.CreatedAt = parseTime(created)
	return u, nil
}

func (s *Store) UserByID(ctx context.Context, id int64) (User, error) {
	return scanUser(s.db.QueryRowContext(ctx, `SELECT `+userColumns+` FROM users WHERE id = ?`, id))
}

func (s *Store) UserByEmail(ctx context.Context, email string) (User, error) {
	return scanUser(s.db.QueryRowContext(ctx,
		`SELECT `+userColumns+` FROM users WHERE email = ?`, strings.TrimSpace(email)))
}

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}
