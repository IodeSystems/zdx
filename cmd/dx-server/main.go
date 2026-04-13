package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iodesystems/zdx-go/internal/migrate"
	"github.com/iodesystems/zdx-go/internal/server"
)

// buildSHA is set at build time via -ldflags "-X main.buildSHA=<sha>".
var buildSHA string

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://localhost/zdx?sslmode=disable"
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "7600"
	}

	// Refuse to start if schema is behind. Migrations are run by ops
	// (dx migrate up) before the rolling deploy starts the new binary.
	migrateDSN := strings.Replace(dsn, "postgres://", "pgx5://", 1)
	if err := migrate.AssertCurrent(migrateDSN); err != nil {
		log.Fatalf("schema check: %v\nRun: dx migrate up", err)
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	staticDir := os.Getenv("STATIC_DIR")
	srv := server.New(pool, staticDir, buildSHA)
	addr := fmt.Sprintf(":%s", port)
	log.Printf("dx-server listening on %s", addr)
	if err := http.ListenAndServe(addr, srv); err != nil {
		log.Fatal(err)
	}
}
