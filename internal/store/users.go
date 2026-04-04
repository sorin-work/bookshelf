package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/sorin-work/bookshelf/internal/domain"
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
	Phone        string
	PasswordHash string
	Role         Role
	CardNumber   string
	CardExpires  domain.Date
	CreatedAt    time.Time
}

func (u User) IsLibrarian() bool { return u.Role == RoleLibrarian }

// CanLogIn is false for members registered at the desk without a password.
func (u User) CanLogIn() bool { return u.PasswordHash != "" }

func (u User) CardExpired(today domain.Date) bool {
	return !u.CardExpires.IsZero() && u.CardExpires.Before(today)
}

func (u User) FormattedCard() string { return domain.FormatCardNumber(u.CardNumber) }

type NewUser struct {
	Email        string
	Name         string
	Phone        string
	PasswordHash string
	Role         Role
	CardExpires  domain.Date
}

// CreateUser adds a user and issues their library card. The very first
// account becomes a librarian no matter what role was asked for: whoever
// installs the app runs it.
func (s *Store) CreateUser(ctx context.Context, u NewUser, now time.Time) (User, error) {
	if u.Role == "" {
		u.Role = RoleMember
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, err
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx,
		`INSERT INTO users (email, name, phone, password_hash, role, card_expires, created_at)
		 VALUES (?, ?, ?, ?, CASE WHEN EXISTS (SELECT 1 FROM users) THEN ? ELSE 'librarian' END, ?, ?)`,
		strings.TrimSpace(u.Email), strings.TrimSpace(u.Name), strings.TrimSpace(u.Phone),
		u.PasswordHash, string(u.Role), u.CardExpires.String(), fmtTime(now))
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
	if _, err := tx.ExecContext(ctx, `UPDATE users SET card_number = ? WHERE id = ?`, domain.CardNumber(id), id); err != nil {
		return User{}, err
	}
	if err := tx.Commit(); err != nil {
		return User{}, err
	}
	return s.UserByID(ctx, id)
}

// EnsureCards issues a card to every account that predates library cards.
func (s *Store) EnsureCards(ctx context.Context, expires domain.Date) (int, error) {
	ids, err := s.ids(ctx, `SELECT id FROM users WHERE card_number IS NULL`)
	if err != nil {
		return 0, err
	}
	for _, id := range ids {
		if _, err := s.db.ExecContext(ctx, `UPDATE users SET card_number = ?, card_expires = ? WHERE id = ?`,
			domain.CardNumber(id), expires.String(), id); err != nil {
			return 0, err
		}
	}
	return len(ids), nil
}

const userColumns = `id, email, name, phone, password_hash, role, coalesce(card_number, ''), coalesce(card_expires, ''), created_at`

func scanUser(row interface{ Scan(...any) error }) (User, error) {
	var u User
	var expires, created string
	if err := row.Scan(&u.ID, &u.Email, &u.Name, &u.Phone, &u.PasswordHash, &u.Role, &u.CardNumber, &expires, &created); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrNotFound
		}
		return User{}, err
	}
	u.CardExpires, _ = domain.ParseDate(expires)
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

func (s *Store) UserByCard(ctx context.Context, card string) (User, error) {
	return scanUser(s.db.QueryRowContext(ctx, `SELECT `+userColumns+` FROM users WHERE card_number = ?`, card))
}

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}

// SetPassword replaces a user's password hash and signs them out
// everywhere else by deleting their other sessions. keepSession is the
// hash of the session to keep, or "" to delete them all.
func (s *Store) SetPassword(ctx context.Context, userID int64, hash, keepSession string) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `UPDATE users SET password_hash = ? WHERE id = ?`, hash, userID)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrNotFound
		}
		_, err = tx.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ? AND token_hash != ?`, userID, keepSession)
		return err
	})
}
