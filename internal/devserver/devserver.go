// Package devserver provisions an ephemeral zdx-server against a disposable
// postgres, with sandboxed UploadsDir and DemosDir, for use by dx test and
// standalone e2e binaries.
package devserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iodesystems/zdx-go/internal/migrate"
	"github.com/iodesystems/zdx-go/internal/server"
)

// Options control what Start provisions.
type Options struct {
	// DSN is a postgres connection string. If empty, Start runs
	// docker compose to provide a disposable instance.
	DSN string
	// UploadsDir overrides the server UPLOADS_DIR. If empty, Start creates
	// a private tmpdir owned by the handle's cleanup.
	UploadsDir string
	// DemosDir overrides the server DEMOS_DIR. If empty, Start creates a
	// private tmpdir owned by the handle's cleanup.
	DemosDir string
	// ProjectRoot points at the repo root (where docker/e2e.compose.yaml lives).
	// If empty, Start searches upward from cwd for go.mod.
	ProjectRoot string
	// ComposeProject is the docker compose -p label for isolation. Defaults
	// to "zdx-devserver".
	ComposeProject string
}

// Handle carries the connection details of a running ephemeral server.
type Handle struct {
	URL        string
	AdminToken string
	DSN        string
	UploadsDir string
	DemosDir   string
}

// Start provisions the ephemeral server and returns a cleanup func that
// tears everything down.
func Start(opts Options) (*Handle, func(), error) {
	cleanups := []func(){}
	cleanup := func() {
		for i := len(cleanups) - 1; i >= 0; i-- {
			cleanups[i]()
		}
	}

	dsn := opts.DSN
	if dsn == "" {
		root := opts.ProjectRoot
		if root == "" {
			r, err := findRoot()
			if err != nil {
				return nil, nil, fmt.Errorf("find project root: %w", err)
			}
			root = r
		}
		project := opts.ComposeProject
		if project == "" {
			project = "zdx-devserver"
		}
		composeDSN, composeCleanup, err := startCompose(root, project)
		if err != nil {
			return nil, nil, err
		}
		dsn = composeDSN
		cleanups = append(cleanups, composeCleanup)
	}

	uploadsDir := opts.UploadsDir
	if uploadsDir == "" {
		dir, err := os.MkdirTemp("", "zdx-uploads-")
		if err != nil {
			cleanup()
			return nil, nil, fmt.Errorf("mkdir uploads: %w", err)
		}
		uploadsDir = dir
		cleanups = append(cleanups, func() { _ = os.RemoveAll(dir) })
	}

	demosDir := opts.DemosDir
	if demosDir == "" {
		dir, err := os.MkdirTemp("", "zdx-demos-")
		if err != nil {
			cleanup()
			return nil, nil, fmt.Errorf("mkdir demos: %w", err)
		}
		demosDir = dir
		cleanups = append(cleanups, func() { _ = os.RemoveAll(dir) })
	}

	migrateDSN := strings.Replace(dsn, "postgres://", "pgx5://", 1)
	if err := migrate.Up(migrateDSN); err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("migrate: %w", err)
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("pool: %w", err)
	}
	cleanups = append(cleanups, pool.Close)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("listen: %w", err)
	}

	// server.New reads UPLOADS_DIR at construction time; stage the values so
	// the ephemeral instance stays sandboxed even when the caller's env has
	// a global UPLOADS_DIR set.
	restoreUploads := setEnv("UPLOADS_DIR", uploadsDir)
	restoreDemos := setEnv("DEMOS_DIR", demosDir)
	srv := server.New(pool, server.NewTimingSink(), "", "devserver")
	restoreUploads()
	restoreDemos()

	hs := &http.Server{Handler: srv}
	go hs.Serve(ln) //nolint:errcheck
	cleanups = append(cleanups, func() { _ = hs.Close() })

	base := "http://" + ln.Addr().String()

	payload, _ := json.Marshal(map[string]string{
		"email": "admin@test.local",
		"name":  "Test Admin",
	})
	resp, err := http.Post(base+"/api/setup/bootstrap", "application/json", bytes.NewReader(payload))
	if err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("bootstrap: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		cleanup()
		return nil, nil, fmt.Errorf("bootstrap: HTTP %d", resp.StatusCode)
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("bootstrap decode: %w", err)
	}

	return &Handle{
		URL:        base,
		AdminToken: out.Token,
		DSN:        dsn,
		UploadsDir: uploadsDir,
		DemosDir:   demosDir,
	}, cleanup, nil
}

// setEnv sets an env var and returns a function that restores the previous value.
func setEnv(key, value string) func() {
	prev, had := os.LookupEnv(key)
	_ = os.Setenv(key, value)
	return func() {
		if had {
			_ = os.Setenv(key, prev)
		} else {
			_ = os.Unsetenv(key)
		}
	}
}

func startCompose(root, project string) (dsn string, cleanup func(), err error) {
	composeFile := filepath.Join(root, "docker", "e2e.compose.yaml")

	up := exec.Command("docker", "compose", "-f", composeFile, "-p", project, "up", "-d", "--wait")
	up.Stdout = os.Stderr
	up.Stderr = os.Stderr
	if err := up.Run(); err != nil {
		return "", nil, fmt.Errorf("docker compose up: %w", err)
	}

	cleanup = func() {
		exec.Command("docker", "compose", "-f", composeFile, "-p", project, "down", "-v").Run() //nolint:errcheck
	}

	portOut, err := exec.Command("docker", "compose", "-f", composeFile, "-p", project, "port", "postgres", "5432").Output()
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("get postgres port: %w", err)
	}
	addr := strings.TrimSpace(string(portOut))
	return fmt.Sprintf("postgres://zdx:zdx@%s/zdx_e2e?sslmode=disable", addr), cleanup, nil
}

func findRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("project root (go.mod) not found")
		}
		dir = parent
	}
}
