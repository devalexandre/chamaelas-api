package database

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/vingarcia/ksql"
	"github.com/vingarcia/ksql/adapters/kpgx"
	ksqlite "github.com/vingarcia/ksql/adapters/modernc-ksqlite"

	"chamaelas-api/internal/config"
)

// Connect opens a ksql.DB using the adapter selected by cfg.DBDriver. This is
// the only place that knows about kpgx/ksqlite — every repository just uses
// the returned ksql.DB, so switching databases never touches business code.
func Connect(ctx context.Context, cfg config.Config) (ksql.DB, error) {
	switch cfg.DBDriver {
	case "postgres":
		return kpgx.New(ctx, cfg.DBDSN, ksql.Config{})
	case "sqlite", "":
		return ksqlite.New(ctx, cfg.DBDSN, ksql.Config{})
	default:
		return ksql.DB{}, fmt.Errorf("unsupported DB_DRIVER %q (use \"sqlite\" or \"postgres\")", cfg.DBDriver)
	}
}

// RunMigrations applies every pending migration embedded in migrationsFS
// (see the //go:embed directive on main's migrationsFS), picking the right
// database driver URL scheme from cfg.DBDriver. Migrations ship inside the
// binary itself — no "migrations" directory needs to exist next to it in
// production, which matters once this runs somewhere like Render instead of
// a local checkout.
func RunMigrations(cfg config.Config, migrationsFS fs.FS) error {
	sourceDriver, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("iofs.New: %w", err)
	}

	databaseURL, err := migrationDatabaseURL(cfg)
	if err != nil {
		return err
	}

	m, err := migrate.NewWithSourceInstance("iofs", sourceDriver, databaseURL)
	if err != nil {
		return fmt.Errorf("migrate.NewWithSourceInstance: %w", err)
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate.Up: %w", err)
	}

	log.Printf("database: migrations up to date (driver=%s)", cfg.DBDriver)
	return nil
}

// Placeholder returns the parameter marker a raw SQL query should use for
// argument n (1-indexed), matching cfg.DBDriver — Postgres wants "$1", "$2",
// SQLite (and MySQL) want "?". Repositories that write raw WHERE/UPDATE
// clauses use this instead of hardcoding a style, so they work unchanged on
// either database.
func Placeholder(cfg config.Config, n int) string {
	if cfg.DBDriver == "postgres" {
		return fmt.Sprintf("$%d", n)
	}
	return "?"
}

// DateOnly returns a SQL expression that extracts just the "YYYY-MM-DD"
// part of a timestamp column, for GROUP BY / ORDER BY on the date alone.
// Postgres stores created_at as a native timestamp, so DATE() works
// directly. SQLite (via modernc's driver) stores time.Time as Go's default
// string format ("2026-09-07 23:04:39.263121587 +0000 UTC") rather than
// ISO8601, which SQLite's own DATE() can't parse — but the year-month-day
// always lands in the first 10 characters, so a substring does the job.
func DateOnly(cfg config.Config, column string) string {
	if cfg.DBDriver == "postgres" {
		return fmt.Sprintf("DATE(%s)", column)
	}
	return fmt.Sprintf("SUBSTR(%s, 1, 10)", column)
}

func migrationDatabaseURL(cfg config.Config) (string, error) {
	switch cfg.DBDriver {
	case "postgres":
		return cfg.DBDSN, nil
	case "sqlite", "":
		return "sqlite://" + cfg.DBDSN, nil
	default:
		return "", fmt.Errorf("unsupported DB_DRIVER %q (use \"sqlite\" or \"postgres\")", cfg.DBDriver)
	}
}
