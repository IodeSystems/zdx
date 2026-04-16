package main

import (
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

	switch {
	case postNil:
		log.Println("migrate: no migrations embedded in this binary")
	case preNil:
		log.Printf("migrate: schema initialized at version %d", postVer)
	case preVer == postVer:
		log.Printf("migrate: already up-to-date at version %d", postVer)
	default:
		log.Printf("migrate: applied migrations %d → %d", preVer, postVer)
	}

	if !noGen {
		runGen()
	}
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
