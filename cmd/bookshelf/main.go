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
	"github.com/sorin-work/bookshelf/internal/jobs"
	"github.com/sorin-work/bookshelf/internal/openlibrary"
	"github.com/sorin-work/bookshelf/internal/seed"
	"github.com/sorin-work/bookshelf/internal/store"
	"github.com/sorin-work/bookshelf/internal/version"
	"github.com/sorin-work/bookshelf/internal/web"
)

func main() {
	var err error
	switch {
	case len(os.Args) == 3 && os.Args[1] == "backup":
		err = backup(os.Args[2])
	case len(os.Args) > 1:
		fmt.Fprintln(os.Stderr, "usage: bookshelf            run the web app\n       bookshelf backup FILE  copy the database to FILE")
		os.Exit(2)
	default:
		err = run()
	}
	if err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

// backup copies the database the app is configured to use, which is safe
// to do while the app is running.
func backup(path string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	ctx := context.Background()
	conn, err := db.Open(ctx, cfg.DBPath)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := db.Backup(ctx, conn, path); err != nil {
		return err
	}
	fmt.Println("backed up", cfg.DBPath, "to", path)
	return nil
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
	defer func() {
		// Lets SQLite refresh its query planner statistics; cheap, and
		// recommended before closing a long-lived connection.
		if _, err := conn.ExecContext(context.Background(), `PRAGMA optimize`); err != nil {
			log.Warn("optimise database", "err", err)
		}
		conn.Close()
	}()

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

	go jobs.Housekeeping{Store: st, Location: cfg.Location, Logger: log}.Run(ctx, time.Hour)

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
