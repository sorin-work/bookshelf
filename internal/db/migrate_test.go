package db

import (
	"context"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestMigrateAppliesOnce(t *testing.T) {
	ctx := context.Background()
	conn, err := Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	n, err := Migrate(ctx, conn)
	if err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		t.Fatal("expected migrations to be applied")
	}
	again, err := Migrate(ctx, conn)
	if err != nil {
		t.Fatal(err)
	}
	if again != 0 {
		t.Errorf("second run applied %d migrations", again)
	}

	var fk int
	if err := conn.QueryRowContext(ctx, `PRAGMA foreign_keys`).Scan(&fk); err != nil {
		t.Fatal(err)
	}
	if fk != 1 {
		t.Error("foreign keys are off")
	}
}

func TestLoadMigrationsRejectsBadNames(t *testing.T) {
	tests := []struct {
		name  string
		files fstest.MapFS
	}{
		{"no underscore", fstest.MapFS{"migrations/0001.sql": {Data: []byte("")}}},
		{"not a number", fstest.MapFS{"migrations/abc_init.sql": {Data: []byte("")}}},
		{"duplicate", fstest.MapFS{
			"migrations/0001_a.sql": {Data: []byte("")},
			"migrations/0001_b.sql": {Data: []byte("")},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := loadMigrations(tt.files); err == nil {
				t.Error("expected an error")
			}
		})
	}
}

func TestMigrateRollsBackFailedMigration(t *testing.T) {
	ctx := context.Background()
	conn, err := Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	files := fstest.MapFS{
		"migrations/0001_ok.sql":     {Data: []byte("CREATE TABLE a (id INTEGER);")},
		"migrations/0002_broken.sql": {Data: []byte("CREATE TABLE b (id INTEGER); NOT SQL;")},
	}
	n, err := migrate(ctx, conn, files)
	if err == nil {
		t.Fatal("expected an error")
	}
	if n != 1 {
		t.Errorf("applied %d, want 1", n)
	}
	var count int
	if err := conn.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_master WHERE name = 'b'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Error("table from the failed migration was kept")
	}
}
