// Package jobs runs the background housekeeping: expiring uncollected
// holds and deleting expired sessions.
package jobs

import (
	"context"
	"log/slog"
	"time"

	"github.com/sorin-work/bookshelf/internal/domain"
	"github.com/sorin-work/bookshelf/internal/store"
)

// Housekeeping is one round of background work at the given moment.
type Housekeeping struct {
	Store    *store.Store
	Location *time.Location
	Logger   *slog.Logger
}

// RunOnce expires holds whose pickup deadline has passed (handing their
// copies on) and deletes expired sessions.
func (h Housekeeping) RunOnce(ctx context.Context, now time.Time) error {
	n, err := h.Store.ExpireHolds(ctx, domain.DateOf(now.In(h.Location)), now)
	if err != nil {
		return err
	}
	if n > 0 {
		h.Logger.Info("expired uncollected holds", "count", n)
	}
	if _, err := h.Store.DeleteExpiredSessions(ctx, now); err != nil {
		return err
	}
	return nil
}

// Run calls RunOnce straight away and then every interval until ctx is
// done.
func (h Housekeeping) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if err := h.RunOnce(ctx, time.Now()); err != nil && ctx.Err() == nil {
			h.Logger.Error("housekeeping", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
