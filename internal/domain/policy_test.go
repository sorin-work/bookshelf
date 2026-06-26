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

func TestDueDateSkipsSeveralClosedDays(t *testing.T) {
	p := DefaultPolicy()
	p.LoanDays = 14
	p.ClosedDays = []time.Weekday{time.Saturday, time.Sunday, time.Monday}

	tests := []struct {
		from, want string
	}{
		{"2026-03-07", "2026-03-24"}, // Saturday -> Saturday, skip to Tuesday
		{"2026-03-08", "2026-03-24"}, // Sunday -> Sunday
		{"2026-03-09", "2026-03-24"}, // Monday -> Monday
		{"2026-03-10", "2026-03-24"}, // Tuesday -> Tuesday
	}
	for _, tt := range tests {
		got := p.DueDate(mustDate(t, tt.from))
		if got.String() != tt.want {
			t.Errorf("DueDate(%s) = %s (%s), want %s", tt.from, got, got.Weekday(), tt.want)
		}
	}
}

func TestDueDateSkipsClosureDates(t *testing.T) {
	p := DefaultPolicy() // 21 days, closed Sundays
	p.ClosedDates = []Date{mustDate(t, "2025-12-24"), mustDate(t, "2025-12-25"), mustDate(t, "2025-12-26")}
	// 3 Dec + 21 = 24 Dec, closed until the 26th, then the 27th is a
	// Saturday and open.
	if got := p.DueDate(mustDate(t, "2025-12-03")); got.String() != "2025-12-27" {
		t.Errorf("DueDate = %s", got)
	}
	// A closure running into a Sunday.
	p.ClosedDates = []Date{mustDate(t, "2026-04-04")} // Saturday
	if got := p.DueDate(mustDate(t, "2026-03-14")); got.String() != "2026-04-06" {
		t.Errorf("DueDate = %s", got)
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
		{"too many closures", func(p *Policy) {
			for i := 0; i <= MaxClosedDates; i++ {
				p.ClosedDates = append(p.ClosedDates, Date{2026, time.January, 1}.AddDays(i))
			}
		}},
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
