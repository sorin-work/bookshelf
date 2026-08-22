// Command bookshelf runs the library web app.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata" // embedded zone database, so LIBRARY_TIMEZONE works anywhere

	"github.com/sorin-work/bookshelf/internal/config"
	"github.com/sorin-work/bookshelf/internal/db"
	"github.com/sorin-work/bookshelf/internal/domain"
	"github.com/sorin-work/bookshelf/internal/openlibrary"
	"github.com/sorin-work/bookshelf/internal/seed"
	"github.com/sorin-work/bookshelf/internal/store"
	"github.com/sorin-work/bookshelf/internal/version"
	"github.com/sorin-work/bookshelf/internal/web"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	log := newLogger(cfg)
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	conn, err := db.Open(ctx, cfg.DBPath)
	if err != nil {
		return err
	}
	defer conn.Close()

	applied, err := db.Migrate(ctx, conn)
	if err != nil {
		return err
	}
	log.Info("database ready", "path", cfg.DBPath, "migrations_applied", applied)

	st := store.New(conn)
	if n, err := st.EnsureCards(ctx, domain.DateOf(time.Now().In(cfg.Location)).AddYears(1)); err != nil {
		return fmt.Errorf("issue library cards: %w", err)
	} else if n > 0 {
		log.Info("issued library cards to existing accounts", "count", n)
	}

	if cfg.SeedDemo {
		if err := seed.Demo(ctx, st, seed.Options{Now: time.Now(), Location: cfg.Location, Logger: log}); err != nil {
			return fmt.Errorf("seed demo data: %w", err)
		}
	}

	var books web.BookLookup
	if cfg.OpenLibraryURL != "" {
		books = openlibrary.New(cfg.OpenLibraryURL)
	}

	app, err := web.New(web.Options{
		Books:         books,
		DisableSignup: !cfg.AllowSignup,
		LibraryName:   cfg.LibraryName,
		Store:         st,
		Logger:        log,
		SecureCookies: cfg.SecureCookies,
		SessionTTL:    cfg.SessionTTL,
		Location:      cfg.Location,
	})
	if err != nil {
		return err
	}

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           app.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go expireHolds(ctx, st, cfg.Location, log)

	errCh := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", cfg.Addr, "version", version.String())
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		log.Info("shutting down")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

func newLogger(cfg config.Config) *slog.Logger {
	opts := &slog.HandlerOptions{Level: cfg.LogLevel}
	if cfg.LogFormat == "text" {
		return slog.New(slog.NewTextHandler(os.Stdout, opts))
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, opts))
}

// expireHolds closes uncollected holds once at start and then every hour,
// handing their copies to the next member in each queue. It also clears
// out expired sessions.
func expireHolds(ctx context.Context, st *store.Store, loc *time.Location, log *slog.Logger) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		now := time.Now()
		n, err := st.ExpireHolds(ctx, domain.DateOf(now.In(loc)), now)
		switch {
		case err != nil && ctx.Err() == nil:
			log.Error("expire holds", "err", err)
		case n > 0:
			log.Info("expired uncollected holds", "count", n)
		}
		if _, err := st.DeleteExpiredSessions(ctx, now); err != nil && ctx.Err() == nil {
			log.Error("delete expired sessions", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
