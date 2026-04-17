package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	migrate4 "github.com/golang-migrate/migrate/v4"
	"github.com/iodesystems/zdx-go/internal/migrate"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: db <migrate|gen>")
		fmt.Fprintln(os.Stderr, "  migrate   apply pending migrations + sqlc generate (--no-gen to skip)")
		fmt.Fprintln(os.Stderr, "  gen       run sqlc generate")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "migrate":
		noGen := false
		for _, a := range os.Args[2:] {
			if a == "--no-gen" {
				noGen = true
			}
		}
		runMigrate(noGen)
	case "gen":
		runGen()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func runMigrate(noGen bool) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://localhost/zdx?sslmode=disable"
	}
	migrateDSN := strings.Replace(dsn, "postgres://", "pgx5://", 1)

	preVer, preDirty, preErr := migrate.Version(migrateDSN)
	preNil := errors.Is(preErr, migrate4.ErrNilVersion)
	if preErr != nil && !preNil {
		log.Fatalf("migrate: read current version: %v", preErr)
	}
	if preDirty {
		log.Fatalf("migrate: schema is dirty at version %d — manual intervention required", preVer)
	}

	if err := migrate.Up(migrateDSN); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	postVer, _, postErr := migrate.Version(migrateDSN)
	postNil := errors.Is(postErr, migrate4.ErrNilVersion)
	if postErr != nil && !postNil {
		log.Fatalf("migrate: read post version: %v", postErr)
	}

	migrationsApplied := false
	switch {
	case postNil:
		log.Println("migrate: no migrations embedded in this binary")
	case preNil:
		log.Printf("migrate: schema initialized at version %d", postVer)
		migrationsApplied = true
	case preVer == postVer:
		log.Printf("migrate: already up-to-date at version %d", postVer)
	default:
		log.Printf("migrate: applied migrations %d → %d", preVer, postVer)
		migrationsApplied = true
	}

	if migrationsApplied {
		if err := refreshShippedSQL(dsn); err != nil {
			log.Printf("migrate: warning: could not refresh schema/shipped.sql: %v", err)
			log.Println("migrate: sqlc generate may fail — run pg_dump manually:")
			log.Println("  pg_dump --schema-only --no-owner --no-privileges --exclude-table=schema_migrations $DATABASE_URL > schema/shipped.sql")
		}
	}

	if !noGen {
		runGen()
	}
}

func refreshShippedSQL(dsn string) error {
	pgDump, err := exec.LookPath("pg_dump")
	if err != nil {
		return fmt.Errorf("pg_dump not found in PATH")
	}
	var buf bytes.Buffer
	cmd := exec.Command(pgDump,
		"--schema-only",
		"--no-owner",
		"--no-privileges",
		"--exclude-table=schema_migrations",
		dsn,
	)
	cmd.Stdout = &buf
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pg_dump: %w", err)
	}
	f, err := os.Create("schema/shipped.sql")
	if err != nil {
		return fmt.Errorf("create schema/shipped.sql: %w", err)
	}
	defer f.Close()
	// pg_dump on Postgres 17+ emits \restrict / \unrestrict psql meta-commands
	// that sqlc cannot parse. Strip any line starting with a backslash — same
	// filter bin/ship applies to its compat-check pg_dump output.
	scan := bufio.NewScanner(&buf)
	scan.Buffer(make([]byte, 1<<20), 1<<24)
	for scan.Scan() {
		line := scan.Text()
		if strings.HasPrefix(line, "\\") {
			continue
		}
		if _, err := fmt.Fprintln(f, line); err != nil {
			return fmt.Errorf("write shipped.sql: %w", err)
		}
	}
	if err := scan.Err(); err != nil {
		return fmt.Errorf("scan pg_dump output: %w", err)
	}
	log.Println("migrate: schema/shipped.sql refreshed via pg_dump")
	return nil
}

func runGen() {
	sqlc := findSqlc()
	cmd := exec.Command(sqlc, "generate")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatalf("sqlc generate: %v", err)
	}
	log.Println("sqlc generate complete")
}

func findSqlc() string {
	if p, err := exec.LookPath("sqlc"); err == nil {
		return p
	}
	home, _ := os.UserHomeDir()
	candidate := home + "/go/bin/sqlc"
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	log.Fatal("sqlc not found — install with: go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest")
	return ""
}
