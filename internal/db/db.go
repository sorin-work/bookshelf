// Package db opens the SQLite database and keeps its schema up to date.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"

	// Pure Go SQLite driver, registered as "sqlite".
	_ "modernc.org/sqlite"
)

// Open opens (creating if needed) the SQLite file at path with foreign keys
// on, WAL journaling and a busy timeout.
//
// SQLite allows one writer at a time, so the pool is limited to a single
// connection. That keeps transactions simple and is plenty for a small
// library.
func Open(ctx context.Context, path string) (*sql.DB, error) {
	q := url.Values{}
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "busy_timeout(5000)")
	q.Add("_pragma", "synchronous(NORMAL)")
	dsn := "file:" + path + "?" + q.Encode()

	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	conn.SetMaxOpenConns(1)
	if err := conn.PingContext(ctx); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	return conn, nil
}
