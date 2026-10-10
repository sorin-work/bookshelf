package store

import (
	"context"
	"testing"
	"time"
)

func TestDashboardStats(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if _, err := f.checkout(t, "B1"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.checkout(t, "B2"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.s.Return(ctx, "B2", f.today, now); err != nil {
		t.Fatal(err)
	}
	day := now.Truncate(24 * time.Hour)
	st, err := f.s.DashboardStats(ctx, f.today, day, day.Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	want := Stats{OnLoan: 1, CheckoutsToday: 2, ReturnsToday: 1, Titles: 1, Copies: 6, Available: 5, Members: 1}
	if st != want {
		t.Errorf("stats\n got %+v\nwant %+v", st, want)
	}

	popular, err := f.s.PopularTitles(ctx, now.Add(-time.Hour), 5)
	if err != nil || len(popular) != 1 || popular[0].Checkouts != 2 {
		t.Errorf("popular %+v, %v", popular, err)
	}
	activity, err := f.s.RecentActivity(ctx, 10)
	if err != nil || len(activity) != 3 {
		t.Errorf("activity %+v, %v", activity, err)
	}
}

func TestMonthlyCirculation(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if _, err := f.checkout(t, "B1"); err != nil {
		t.Fatal(err)
	}
	// Returned on the 1st of the next month, 01:30 in a UTC+2 library but
	// still the 31st in UTC.
	returned := time.Date(2026, time.January, 31, 23, 30, 0, 0, time.UTC)
	if _, err := f.s.Return(ctx, "B1", f.today, returned); err != nil {
		t.Fatal(err)
	}
	months, err := f.s.MonthlyCirculation(ctx, "2026-01", "2026-02", 2*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if len(months) != 2 || months[0].Checkouts != 1 || months[0].Returns != 0 || months[1].Returns != 1 {
		t.Errorf("months %+v", months)
	}
}
