package db

import (
	"context"
	"path/filepath"
	"testing"
)

func TestBackup(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	conn, err := Open(ctx, filepath.Join(dir, "live.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := Migrate(ctx, conn); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, `INSERT INTO settings (key, value) VALUES ('k', 'v')`); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(dir, "backup.db")
	if err := Backup(ctx, conn, dst); err != nil {
		t.Fatal(err)
	}
	if err := Backup(ctx, conn, dst); err == nil {
		t.Error("overwrote an existing backup")
	}

	copied, err := Open(ctx, dst)
	if err != nil {
		t.Fatal(err)
	}
	defer copied.Close()
	var v string
	if err := copied.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = 'k'`).Scan(&v); err != nil || v != "v" {
		t.Errorf("backup content: %q, %v", v, err)
	}
	if n, err := Migrate(ctx, copied); err != nil || n != 0 {
		t.Errorf("backup is missing migrations: %d, %v", n, err)
	}
}
