package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
)

// Backup writes a consistent copy of the database to path with VACUUM
// INTO, which is safe while the app is running. It refuses to overwrite
// an existing file.
func Backup(ctx context.Context, conn *sql.DB, path string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists", path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if _, err := conn.ExecContext(ctx, `VACUUM INTO ?`, path); err != nil {
		return fmt.Errorf("backup to %s: %w", path, err)
	}
	return nil
}
