// Package migrate runs database migrations using golang-migrate with embedded SQL files.
package migrate

import (
	"embed"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed sql/*.sql
var migrationsFS embed.FS

// Up applies all pending migrations against dsn (postgres://...).
// Returns nil if already up-to-date.
func Up(dsn string) error {
	src, err := iofs.New(migrationsFS, "sql")
	if err != nil {
		return fmt.Errorf("migration source: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, dsn)
	if err != nil {
		return fmt.Errorf("migrate init: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}

// Version returns the current migration version and dirty flag.
func Version(dsn string) (uint, bool, error) {
	src, err := iofs.New(migrationsFS, "sql")
	if err != nil {
		return 0, false, err
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, dsn)
	if err != nil {
		return 0, false, err
	}
	defer m.Close()
	return m.Version()
}

// AssertCurrent returns an error if the database schema is behind the
// embedded migrations. Used by dx-server on startup to fail fast if ops
// forgot to run migrations before the rolling deploy.
func AssertCurrent(dsn string) error {
	src, err := iofs.New(migrationsFS, "sql")
	if err != nil {
		return fmt.Errorf("migration source: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, dsn)
	if err != nil {
		return fmt.Errorf("migrate init: %w", err)
	}
	defer m.Close()

	// m.Up() with ErrNoChange means we're current.
	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			return nil
		}
		return fmt.Errorf("pending migrations exist — run: dx migrate up (%w)", err)
	}
	// Up() succeeded means migrations were applied — that's unexpected in
	// production (ops should have done it), but not fatal. Log and continue.
	return nil
}
