// Package testserver spins up a fully migrated dx-server on a random port
// and bootstraps an admin account, for use in e2e tests.
package testserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iodesystems/zdx-go/internal/migrate"
	"github.com/iodesystems/zdx-go/internal/server"
)

// Handle holds the connection details for a running test server.
type Handle struct {
	URL        string
	AdminToken string
}

// New migrates dsn to the latest schema, starts a server on a random local
// port, and bootstraps an admin account. The returned cleanup func stops the
// HTTP server and closes the connection pool; call it in TestMain or t.Cleanup.
func New(dsn string) (*Handle, func(), error) {
	migrateDSN := strings.Replace(dsn, "postgres://", "pgx5://", 1)
	if err := migrate.Up(migrateDSN); err != nil {
		return nil, nil, fmt.Errorf("migrate: %w", err)
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		return nil, nil, fmt.Errorf("pool: %w", err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("listen: %w", err)
	}

	srv := server.New(pool, server.NewTimingSink(), "", "e2e-test")
	hs := &http.Server{Handler: srv}
	go hs.Serve(ln) //nolint:errcheck

	base := "http://" + ln.Addr().String()

	payload, _ := json.Marshal(map[string]string{
		"email": "admin@test.local",
		"name":  "Test Admin",
	})
	resp, err := http.Post(base+"/api/setup/bootstrap", "application/json", bytes.NewReader(payload))
	if err != nil {
		hs.Close()
		pool.Close()
		return nil, nil, fmt.Errorf("bootstrap: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		hs.Close()
		pool.Close()
		return nil, nil, fmt.Errorf("bootstrap: HTTP %d", resp.StatusCode)
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		hs.Close()
		pool.Close()
		return nil, nil, fmt.Errorf("bootstrap decode: %w", err)
	}

	cleanup := func() {
		hs.Close()
		pool.Close()
	}
	return &Handle{URL: base, AdminToken: out.Token}, cleanup, nil
}
