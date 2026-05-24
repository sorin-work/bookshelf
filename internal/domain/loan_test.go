package domain

import (
	"errors"
	"testing"
)

func TestCanRenew(t *testing.T) {
	p := DefaultPolicy() // two renewals
	today := mustDate(t, "2026-03-10")

	tests := []struct {
		name string
		loan Loan
		want error
	}{
		{"fresh loan", Loan{Due: today.AddDays(14)}, nil},
		{"due today", Loan{Due: today}, nil},
		{"renewed once", Loan{Due: today.AddDays(3), Renewals: 1}, nil},
		{"limit reached", Loan{Due: today.AddDays(3), Renewals: 2}, ErrRenewalLimit},
		{"overdue", Loan{Due: today.AddDays(-1)}, ErrRenewOverdue},
		{"overdue and at the limit", Loan{Due: today.AddDays(-5), Renewals: 2}, ErrRenewOverdue},
		{"someone waiting", Loan{Due: today.AddDays(3), HoldsWaiting: 1}, ErrRenewHeld},
		{"card expired", Loan{Due: today.AddDays(3), CardExpires: today.AddDays(-1)}, ErrCardExpired},
		{"card expires today", Loan{Due: today.AddDays(3), CardExpires: today}, nil},
		{"someone waiting and at the limit", Loan{Due: today.AddDays(3), Renewals: 2, HoldsWaiting: 3}, ErrRenewHeld},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := p.CanRenew(tt.loan, today); !errors.Is(err, tt.want) {
				t.Errorf("CanRenew = %v, want %v", err, tt.want)
			}
		})
	}

	p.MaxRenewals = 0
	if err := p.CanRenew(Loan{Due: today.AddDays(10)}, today); !errors.Is(err, ErrRenewalLimit) {
		t.Errorf("renewals switched off: %v", err)
	}
}

func TestRenewedDue(t *testing.T) {
	p := DefaultPolicy()
	today := mustDate(t, "2026-03-10")

	tests := []struct {
		name string
		due  string
		want string
	}{
		{"due soon gets a full period from today", "2026-03-12", "2026-03-31"},
		{"never earlier than the current due date", "2026-04-20", "2026-04-20"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.RenewedDue(Loan{Due: mustDate(t, tt.due)}, today)
			if got.String() != tt.want {
				t.Errorf("RenewedDue = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestDaysOverdue(t *testing.T) {
	today := mustDate(t, "2026-03-10")
	tests := []struct {
		due  string
		want int
	}{
		{"2026-03-11", 0},
		{"2026-03-10", 0},
		{"2026-03-09", 1},
		{"2026-02-28", 10},
	}
	for _, tt := range tests {
		if got := (Loan{Due: mustDate(t, tt.due)}).DaysOverdue(today); got != tt.want {
			t.Errorf("due %s: DaysOverdue = %d, want %d", tt.due, got, tt.want)
		}
	}
}

func TestCanBorrow(t *testing.T) {
	p := DefaultPolicy() // five loans
	today := mustDate(t, "2026-03-10")

	tests := []struct {
		name string
		b    Borrower
		want error
	}{
		{"ok", Borrower{CardExpires: today.AddDays(100), ActiveLoans: 2}, nil},
		{"card expires today", Borrower{CardExpires: today, ActiveLoans: 0}, nil},
		{"card expired", Borrower{CardExpires: today.AddDays(-1)}, ErrCardExpired},
		{"at the loan limit", Borrower{CardExpires: today.AddDays(100), ActiveLoans: 5}, ErrTooManyLoans},
		{"no expiry recorded", Borrower{ActiveLoans: 1}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := p.CanBorrow(tt.b, today); !errors.Is(err, tt.want) {
				t.Errorf("CanBorrow = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestCanBorrowWithFines(t *testing.T) {
	p := DefaultPolicy() // blocked at 10.00
	today := mustDate(t, "2026-03-10")
	b := Borrower{CardExpires: today.AddDays(30)}

	b.FinesOwed = 999
	if err := p.CanBorrow(b, today); err != nil {
		t.Errorf("just under the limit: %v", err)
	}
	b.FinesOwed = 1000
	if err := p.CanBorrow(b, today); !errors.Is(err, ErrFinesOwed) {
		t.Errorf("at the limit: %v", err)
	}
	p.BlockAt = 0
	if err := p.CanBorrow(b, today); err != nil {
		t.Errorf("block switched off: %v", err)
	}
}
