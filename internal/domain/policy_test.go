package domain

import (
	"testing"
	"time"
)

func mustDate(t *testing.T, s string) Date {
	t.Helper()
	d, err := ParseDate(s)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestDueDate(t *testing.T) {
	p := DefaultPolicy() // 21 days, closed on Sundays

	tests := []struct {
		name string
		from string
		want string
	}{
		{"lands on a weekday", "2026-03-10", "2026-03-31"},
		{"lands on a Sunday", "2026-03-08", "2026-03-30"},
		{"across the year end", "2025-12-20", "2026-01-10"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.DueDate(mustDate(t, tt.from))
			if got.String() != tt.want {
				t.Errorf("DueDate(%s) = %s (%s), want %s", tt.from, got, got.Weekday(), tt.want)
			}
		})
	}
}

func TestPolicyValidate(t *testing.T) {
	if err := DefaultPolicy().Validate(); err != nil {
		t.Errorf("default policy: %v", err)
	}
	tests := []struct {
		name   string
		modify func(*Policy)
	}{
		{"zero loan days", func(p *Policy) { p.LoanDays = 0 }},
		{"negative renewals", func(p *Policy) { p.MaxRenewals = -1 }},
		{"no loans allowed", func(p *Policy) { p.MaxLoans = 0 }},
		{"never open", func(p *Policy) {
			p.ClosedDays = []time.Weekday{0, 1, 2, 3, 4, 5, 6}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := DefaultPolicy()
			tt.modify(&p)
			if err := p.Validate(); err == nil {
				t.Error("expected an error")
			}
		})
	}
}
