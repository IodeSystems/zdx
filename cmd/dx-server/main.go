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

	migrateDSN := strings.Replace(dsn, "postgres://", "pgx5://", 1)
	if buildSHA == "" {
		// Dev: auto-migrate on startup so a server restart is sufficient.
		if err := migrate.Up(migrateDSN); err != nil {
			log.Fatalf("auto-migrate: %v", err)
		}
	} else {
		// Prod: schema must already be current — ops runs "dx migrate up"
		// before the rolling deploy starts the new binary.
		if err := migrate.AssertCurrent(migrateDSN); err != nil {
			log.Fatalf("schema check: %v", err)
		}
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	staticDir := os.Getenv("STATIC_DIR")
	srv := server.New(pool, staticDir, buildSHA)

	// In dev (no buildSHA injected), check if the OpenAPI spec changed and
	// regenerate ui/src/api.gen.ts if so. Fatal on tsc failure.
	if buildSHA == "" {
		projectRoot, _ := os.Getwd()
		srv.DevCheck(projectRoot)
	}

	addr := fmt.Sprintf(":%s", port)
	log.Printf("dx-server listening on %s", addr)
	if err := http.ListenAndServe(addr, srv); err != nil {
		log.Fatal(err)
	}
}
