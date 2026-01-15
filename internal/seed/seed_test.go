package seed

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/sorin-work/bookshelf/internal/auth"
	"github.com/sorin-work/bookshelf/internal/db"
	"github.com/sorin-work/bookshelf/internal/domain"
	"github.com/sorin-work/bookshelf/internal/store"
)

func TestDemo(t *testing.T) {
	auth.Cost = bcrypt.MinCost
	ctx := context.Background()
	conn, err := db.Open(ctx, filepath.Join(t.TempDir(), "seed.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := db.Migrate(ctx, conn); err != nil {
		t.Fatal(err)
	}
	st := store.New(conn)
	now := time.Date(2026, time.March, 10, 15, 0, 0, 0, time.UTC)
	opts := Options{Now: now, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}

	if err := Demo(ctx, st, opts); err != nil {
		t.Fatal(err)
	}
	demo, err := st.UserByEmail(ctx, DemoEmail)
	if err != nil {
		t.Fatal(err)
	}
	if !demo.IsLibrarian() || !auth.CheckPassword(demo.PasswordHash, DemoPassword) {
		t.Errorf("demo account: %+v", demo)
	}

	today := domain.DateOf(now)
	stats, err := st.DashboardStats(ctx, today, now.Truncate(24*time.Hour), now.Truncate(24*time.Hour).Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if stats.Titles != len(books) || stats.Members != len(people)+1 {
		t.Errorf("stats %+v", stats)
	}
	if stats.OnLoan == 0 || stats.Overdue == 0 || stats.HoldsWaiting+stats.HoldsReady == 0 {
		t.Errorf("history looks too quiet: %+v", stats)
	}
	sam, _ := st.UserByEmail(ctx, MemberEmail)
	if holds, _ := st.OpenHolds(ctx, sam.ID); len(holds) == 0 {
		t.Error("the member demo account has no holds")
	}

	// Running it again changes nothing.
	if err := Demo(ctx, st, opts); err != nil {
		t.Fatal(err)
	}
	again, _ := st.DashboardStats(ctx, today, now.Truncate(24*time.Hour), now.Truncate(24*time.Hour).Add(24*time.Hour))
	if again != stats {
		t.Errorf("second run changed the data: %+v vs %+v", again, stats)
	}
}
