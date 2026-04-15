package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iodesystems/zdx-go/internal/migrate"
	"github.com/iodesystems/zdx-go/internal/server"
	"github.com/iodesystems/zdx-go/pkg/zdxclient"
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

	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		log.Fatalf("parse dsn: %v", err)
	}
	sink := server.NewTimingSink()
	poolCfg.ConnConfig.Tracer = server.QueryTracer{Sink: sink}
	pool, err := pgxpool.NewWithConfig(context.Background(), poolCfg)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	staticDir := os.Getenv("STATIC_DIR")
	srv := server.New(pool, sink, staticDir, buildSHA)

	// Self-integration: zdx-server pushes its own timings through its own
	// ingest endpoint. If the zdx project doesn't exist yet or no slug is
	// configured, bootstrap returns "" and we skip self-wiring — timings
	// will be dropped until someone sets up the project.
	bootCtx, bootCancel := context.WithTimeout(context.Background(), 10*time.Second)
	selfToken, bootErr := srv.BootstrapSelfIntegrationToken(bootCtx)
	bootCancel()
	if bootErr != nil {
		log.Printf("self-token bootstrap: %v (timings will drop)", bootErr)
	}
	if selfToken != "" {
		host, _ := os.Hostname()
		client, err := zdxclient.New(zdxclient.Config{
			Endpoint:    "http://127.0.0.1:" + port,
			Token:       selfToken,
			Component:   "zdx-server",
			Environment: os.Getenv("ZDX_SLOT"),
			Host:        host,
			OnError: func(err error, n int) {
				log.Printf("zdxclient: %d events dropped: %v", n, err)
			},
		})
		if err != nil {
			log.Printf("zdxclient init: %v (timings will drop)", err)
		} else {
			server.StartSinkToClient(sink, client)
			srv.SetErrorClient(client)
			defer func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				_ = client.Close(ctx)
				cancel()
			}()
		}
	}

	srv.StartTimedEventsRetention(context.Background())

	addr := fmt.Sprintf(":%s", port)
	log.Printf("dx-server listening on %s", addr)
	if err := http.ListenAndServe(addr, srv); err != nil {
		log.Fatal(err)
	}
}
