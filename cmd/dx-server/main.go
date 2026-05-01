package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iodesystems/zdx-go/internal/migrate"
	"github.com/iodesystems/zdx-go/internal/server"
	"github.com/iodesystems/zdx-go/pkg/zdxclient"
)

// buildSHA is set at build time via -ldflags "-X main.buildSHA=<sha>".
var buildSHA string

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--openapi" {
		spec, err := server.SpecJSON()
		if err != nil {
			log.Fatalf("openapi: %v", err)
		}
		os.Stdout.Write(spec)
		return
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://localhost/zdx?sslmode=disable"
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "7600"
	}

	if buildSHA == "" {
		// Dev: auto-migrate on startup so a server restart is sufficient.
		if err := migrate.Up(dsn); err != nil {
			log.Fatalf("auto-migrate: %v", err)
		}
	} else {
		// Prod: schema must already be current — ops runs "dx migrate up"
		// before the rolling deploy starts the new binary.
		if err := migrate.AssertCurrent(dsn); err != nil {
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

	if buildSHA == "" {
		// Dev only: regenerate ui/src/api.gen.ts when the OpenAPI spec hash
		// changes. Skipped in prod because the built binary has no ui/ tree.
		srv.DevCheckClient(".")
	}

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
			// Dogfood: every log.Printf line is also shipped to the project
			// /logs dashboard. Stderr stays primary so journald/console output
			// is unchanged. Suppressed substrings short-circuit feedback loops
			// where the forwarding path itself would emit a log line.
			log.SetOutput(&server.LogWriter{
				Primary: os.Stderr,
				Forward: func(level, msg string) {
					client.RecordLog(level, msg, nil)
				},
				Suppress: []string{"zdxclient", "InsertLogEvent"},
			})
			defer func() {
				log.SetOutput(io.Writer(os.Stderr))
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				_ = client.Close(ctx)
				cancel()
			}()
		}
	}

	srv.StartTimedEventsRetention(context.Background())
	srv.StartTaskRecovery(context.Background())

	addr := fmt.Sprintf(":%s", port)
	log.Printf("dx-server listening on %s", addr)
	if err := http.ListenAndServe(addr, srv); err != nil {
		log.Fatal(err)
	}
}
