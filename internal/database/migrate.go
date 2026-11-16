package database

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5" // registers the "pgx5://" driver
	_ "github.com/golang-migrate/migrate/v4/source/file"     // registers the "file://" source
)

// Migrate applies every pending migration from the migrations folder.
// It runs on startup, so each deploy brings the schema up to date by itself.
func Migrate(databaseURL, dir string) error {
	m, err := migrate.New("file://"+dir, toMigrateURL(databaseURL))
	if err != nil {
		// An empty migrations folder is not an error yet, there is just nothing to apply.
		if errors.Is(err, fs.ErrNotExist) {
			slog.Info("no migrations found, skipping", "dir", dir)
			return nil
		}
		return fmt.Errorf("init migrations: %w", err)
	}
	defer func() {
		if srcErr, dbErr := m.Close(); srcErr != nil || dbErr != nil {
			slog.Warn("close migrator", "source_error", srcErr, "db_error", dbErr)
		}
	}()

	if err := m.Up(); err != nil {
		switch {
		case errors.Is(err, migrate.ErrNoChange):
			// schema is already up to date
		case errors.Is(err, fs.ErrNotExist):
			slog.Info("no migrations found, skipping", "dir", dir)
			return nil
		default:
			return fmt.Errorf("apply migrations: %w", err)
		}
	}

	version, dirty, err := m.Version()
	if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
		return fmt.Errorf("read migration version: %w", err)
	}
	slog.Info("migrations applied", "version", version, "dirty", dirty)
	return nil
}

// golang-migrate's pgx v5 driver expects the "pgx5://" scheme
// instead of "postgres://" or "postgresql://".
func toMigrateURL(url string) string {
	for _, prefix := range []string{"postgresql://", "postgres://"} {
		if strings.HasPrefix(url, prefix) {
			return "pgx5://" + strings.TrimPrefix(url, prefix)
		}
	}
	return url
}
