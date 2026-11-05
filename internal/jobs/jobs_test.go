package jobs

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/sorin-work/bookshelf/internal/db"
	"github.com/sorin-work/bookshelf/internal/domain"
	"github.com/sorin-work/bookshelf/internal/store"
)

func TestRunOnce(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open(ctx, filepath.Join(t.TempDir(), "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := db.Migrate(ctx, conn); err != nil {
		t.Fatal(err)
	}
	st := store.New(conn)
	now := time.Date(2026, time.March, 10, 9, 0, 0, 0, time.UTC)
	today := domain.DateOf(now)

	lib, _ := st.CreateUser(ctx, store.NewUser{Email: "l@example.com", Name: "L", CardExpires: today.AddYears(1)}, now)
	mo, _ := st.CreateUser(ctx, store.NewUser{Email: "m@example.com", Name: "Mo", CardExpires: today.AddYears(1)}, now)
	ana, _ := st.CreateUser(ctx, store.NewUser{Email: "a@example.com", Name: "Ana", CardExpires: today.AddYears(1)}, now)
	title, _ := st.CreateTitle(ctx, store.TitleInput{Title: "Dune"}, now)
	if _, err := st.CreateCopy(ctx, title, store.CopyInput{Barcode: "D-01"}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Checkout(ctx, store.CheckoutRequest{UserID: mo.ID, Barcode: "D-01", StaffID: lib.ID, Today: today, Now: now}); err != nil {
		t.Fatal(err)
	}
	hold, err := st.PlaceHold(ctx, ana.ID, title, today, now)
	if err != nil {
		t.Fatal(err)
	}
	res, err := st.Return(ctx, "D-01", today, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateSession(ctx, "old", ana.ID, now, time.Hour); err != nil {
		t.Fatal(err)
	}

	h := Housekeeping{Store: st, Location: time.UTC, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	later := res.Hold.ExpiresOn.AddDays(1).Time().Add(9 * time.Hour)
	if err := h.RunOnce(ctx, later); err != nil {
		t.Fatal(err)
	}
	got, _ := st.HoldByID(ctx, hold.ID)
	if got.Status != domain.HoldExpired {
		t.Errorf("hold is %s", got.Status)
	}
	c, _ := st.CopyByBarcode(ctx, "D-01")
	if c.Status != domain.CopyAvailable {
		t.Errorf("copy is %s", c.Status)
	}
	if _, err := st.SessionUser(ctx, "old", now); err == nil {
		t.Error("expired session still there")
	}
}
