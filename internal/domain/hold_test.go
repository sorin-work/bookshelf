package domain

import (
	"errors"
	"testing"
	"time"
)

func TestCanPlaceHold(t *testing.T) {
	p := DefaultPolicy() // five holds
	today := mustDate(t, "2026-03-10")
	ok := HoldRequest{CardExpires: today.AddDays(30), Copies: 2, AvailableCopies: 0, OpenHolds: 1}

	tests := []struct {
		name   string
		modify func(*HoldRequest)
		want   error
	}{
		{"all copies out", func(*HoldRequest) {}, nil},
		{"copy on the shelf", func(r *HoldRequest) { r.AvailableCopies = 1 }, ErrHoldOnShelf},
		{"no copies at all", func(r *HoldRequest) { r.Copies = 0 }, ErrHoldNoCopies},
		{"already in the queue", func(r *HoldRequest) { r.HasOpenHold = true }, ErrHoldDuplicate},
		{"already borrowing it", func(r *HoldRequest) { r.HasLoan = true }, ErrHoldBorrowing},
		{"hold limit", func(r *HoldRequest) { r.OpenHolds = 5 }, ErrTooManyHolds},
		{"card expired", func(r *HoldRequest) { r.CardExpires = today.AddDays(-1) }, ErrCardExpired},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := ok
			tt.modify(&r)
			if err := p.CanPlaceHold(r, today); !errors.Is(err, tt.want) {
				t.Errorf("CanPlaceHold = %v, want %v", err, tt.want)
			}
		})
	}

	p.MaxHolds = 0
	if err := p.CanPlaceHold(ok, today); !errors.Is(err, ErrHoldsSwitchedOff) {
		t.Errorf("holds switched off: %v", err)
	}
}

func TestPickupDeadline(t *testing.T) {
	p := DefaultPolicy() // seven days, closed Sundays
	tests := []struct {
		ready, want string
	}{
		{"2026-03-10", "2026-03-17"},
		{"2026-03-08", "2026-03-16"}, // Sunday + 7 is a Sunday
	}
	for _, tt := range tests {
		if got := p.PickupDeadline(mustDate(t, tt.ready)); got.String() != tt.want {
			t.Errorf("PickupDeadline(%s) = %s, want %s", tt.ready, got, tt.want)
		}
	}
}

func TestNextInQueue(t *testing.T) {
	base := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	tests := []struct {
		name  string
		queue []QueuedHold
		want  int64
		ok    bool
	}{
		{"empty", nil, 0, false},
		{"one", []QueuedHold{{ID: 7, PlacedAt: base}}, 7, true},
		{"earliest wins", []QueuedHold{
			{ID: 1, PlacedAt: base.Add(time.Hour)},
			{ID: 2, PlacedAt: base},
			{ID: 3, PlacedAt: base.Add(2 * time.Hour)},
		}, 2, true},
		{"ties go to the lower id", []QueuedHold{
			{ID: 9, PlacedAt: base},
			{ID: 4, PlacedAt: base},
		}, 4, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := NextInQueue(tt.queue)
			if ok != tt.ok || got.ID != tt.want {
				t.Errorf("NextInQueue = %d, %v; want %d, %v", got.ID, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestHoldStatusOpen(t *testing.T) {
	for s, want := range map[HoldStatus]bool{
		HoldWaiting: true, HoldReady: true,
		HoldFulfilled: false, HoldCancelled: false, HoldExpired: false,
	} {
		if s.Open() != want {
			t.Errorf("%s.Open() = %v", s, !want)
		}
	}
}

func TestEstimatedWait(t *testing.T) {
	p := DefaultPolicy() // 21 day loans
	tests := []struct {
		position, copies, want int
	}{
		{1, 1, 21},
		{2, 1, 42},
		{2, 2, 21},
		{3, 2, 42},
		{0, 2, 0},
		{1, 0, 0},
	}
	for _, tt := range tests {
		if got := p.EstimatedWait(tt.position, tt.copies); got != tt.want {
			t.Errorf("EstimatedWait(%d, %d) = %d, want %d", tt.position, tt.copies, got, tt.want)
		}
	}
}
