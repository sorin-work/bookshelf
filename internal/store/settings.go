package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/sorin-work/bookshelf/internal/domain"
)

const policyKey = "lending_policy"

// Policy returns the lending policy, falling back to the defaults for
// anything that was never saved. Pages ask for it several times per
// request, so it is cached until SavePolicy changes it.
func (s *Store) Policy(ctx context.Context) (domain.Policy, error) {
	s.policyMu.Lock()
	defer s.policyMu.Unlock()
	if s.policy != nil {
		return clonePolicy(*s.policy), nil
	}
	p, err := loadPolicy(ctx, s.db)
	if err != nil {
		return p, err
	}
	s.policy = &p
	return clonePolicy(p), nil
}

// clonePolicy copies the slices so callers can't change the cached policy.
func clonePolicy(p domain.Policy) domain.Policy {
	p.ClosedDays = append([]time.Weekday(nil), p.ClosedDays...)
	p.ClosedDates = append([]domain.Date(nil), p.ClosedDates...)
	return p
}

func loadPolicy(ctx context.Context, q querier) (domain.Policy, error) {
	p := domain.DefaultPolicy()
	var raw string
	err := q.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, policyKey).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return p, nil
	}
	if err != nil {
		return p, err
	}
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return p, fmt.Errorf("decode lending policy: %w", err)
	}
	return p, nil
}

func (s *Store) SavePolicy(ctx context.Context, p domain.Policy) error {
	if err := p.Validate(); err != nil {
		return err
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}
	s.policyMu.Lock()
	defer s.policyMu.Unlock()
	if _, err := s.db.ExecContext(ctx, `INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT (key) DO UPDATE SET value = excluded.value`, policyKey, string(raw)); err != nil {
		return err
	}
	s.policy = nil
	return nil
}
