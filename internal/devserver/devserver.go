// Package devserver provisions an ephemeral zdx-server against a disposable
// postgres, with sandboxed UploadsDir and DemosDir, for use by dx test and
// standalone e2e binaries.
package devserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	// VecDir overrides the server VEC_DIR. If empty, Start creates a
	// private tmpdir owned by the handle's cleanup so each run starts with
	// a clean vector index rather than inheriting stale zvec files.
	VecDir string
	// ProjectRoot points at the repo root (where docker/e2e.compose.yaml lives).
	// If empty, Start searches upward from cwd for go.mod.
	ProjectRoot string
	// ComposeProject is the docker compose -p label for isolation. Defaults
	// to "zdx-devserver".
	ComposeProject string
	// ValkeyAddr sets ZDX_VALKEY_ADDR for this server instance. If empty the
	// server uses an in-memory broker (single-instance mode).
	ValkeyAddr string
	// SkipBootstrap skips the /api/setup/bootstrap call. Use when starting a
	// second slot against an already-bootstrapped DB; set AdminToken to the
	// token from the first slot.
	SkipBootstrap bool
	// AdminToken is used as the handle token when SkipBootstrap is true.
	AdminToken string
}

// Handle carries the connection details of a running ephemeral server.
type Handle struct {
	URL        string
	AdminToken string
	DSN        string
	UploadsDir string
	DemosDir   string
	VecDir     string
	ValkeyAddr string
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

	vecDir := opts.VecDir
	if vecDir == "" {
		dir, err := os.MkdirTemp("", "zdx-vec-")
		if err != nil {
			cleanup()
			return nil, nil, fmt.Errorf("mkdir vec: %w", err)
		}
		vecDir = dir
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

	// server.New reads UPLOADS_DIR, DEMOS_DIR, VEC_DIR, and ZDX_VALKEY_ADDR at
	// construction time; stage the values so the ephemeral instance stays
	// sandboxed even when the caller's env has globals set.
	restoreUploads := setEnv("UPLOADS_DIR", uploadsDir)
	restoreDemos := setEnv("DEMOS_DIR", demosDir)
	restoreVec := setEnv("VEC_DIR", vecDir)
	restoreValkey := func() {}
	if opts.ValkeyAddr != "" {
		restoreValkey = setEnv("ZDX_VALKEY_ADDR", opts.ValkeyAddr)
	}
	srv := server.New(pool, server.NewTimingSink(), "", "devserver")
	restoreUploads()
	restoreDemos()
	restoreVec()
	restoreValkey()

	hs := &http.Server{Handler: srv}
	go hs.Serve(ln) //nolint:errcheck
	cleanups = append(cleanups, func() { _ = hs.Close() })

	base := "http://" + ln.Addr().String()

	if opts.SkipBootstrap {
		return &Handle{
			URL:        base,
			AdminToken: opts.AdminToken,
			DSN:        dsn,
			UploadsDir: uploadsDir,
			DemosDir:   demosDir,
			VecDir:     vecDir,
			ValkeyAddr: opts.ValkeyAddr,
		}, cleanup, nil
	}

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
		body, _ := io.ReadAll(resp.Body)
		cleanup()
		return nil, nil, fmt.Errorf("bootstrap: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
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
		VecDir:     vecDir,
		ValkeyAddr: opts.ValkeyAddr,
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

	// Tear down any leftover stack from a previous crashed run so the
	// ephemeral contract (fresh DB per Start) holds. Without this, a reused
	// postgres volume keeps the previous bootstrap's api key, and the next
	// /api/setup/bootstrap returns 409.
	down := exec.Command("docker", "compose", "-f", composeFile, "-p", project, "down", "-v")
	down.Stdout = os.Stderr
	down.Stderr = os.Stderr
	_ = down.Run()

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
