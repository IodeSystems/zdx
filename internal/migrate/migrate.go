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

// MigrateTo applies migrations up or down to reach the given target version.
// Returns nil if already at the target version.
func MigrateTo(dsn string, version uint) error {
	src, err := iofs.New(migrationsFS, "sql")
	if err != nil {
		return fmt.Errorf("migration source: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, dsn)
	if err != nil {
		return fmt.Errorf("migrate init: %w", err)
	}
	defer m.Close()
	if err := m.Migrate(version); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate to %d: %w", version, err)
	}
	return nil
}

// AssertCurrent returns an error if the database schema is behind the
// embedded migrations. Used by dx-server in production — ops must run
// "dx migrate up" before the rolling deploy starts the new binary.
// Does NOT apply migrations.
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

	dbVer, dirty, err := m.Version()
	if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
		return fmt.Errorf("schema version: %w", err)
	}
	if dirty {
		return fmt.Errorf("schema is dirty at version %d — manual intervention required", dbVer)
	}

	// Check if any migration exists beyond current version.
	src2, _ := iofs.New(migrationsFS, "sql")
	if _, nextErr := src2.Next(dbVer); nextErr == nil {
		return fmt.Errorf("schema is at version %d but pending migrations exist — run: dx migrate up", dbVer)
	}
	return nil
}
