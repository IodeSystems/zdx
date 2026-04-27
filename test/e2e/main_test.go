package e2e

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/iodesystems/zdx-go/internal/testharness"
	"github.com/iodesystems/zdx-go/internal/testserver"
)

var srv *testserver.Handle

func TestMain(m *testing.M) {
	// When listing tests (-test.list), skip infra setup entirely.
	if isListMode() {
		os.Exit(m.Run())
	}

	initDriverMode()

	// When invoked by `dx test`, the outer runner bootstraps an ephemeral
	// devserver and exports DX_API_URL/DX_API_KEY. Reuse it instead of
	// double-provisioning postgres + server.
	if url := os.Getenv("DX_API_URL"); url != "" {
		srv = &testserver.Handle{URL: url, AdminToken: os.Getenv("DX_API_KEY")}
		code := m.Run()
		runExternalAdapters()
		os.Exit(code)
	}

	dsn := os.Getenv("TEST_DATABASE_URL")
	dbCleanup := func() {}

	if dsn == "" {
		var err error
		dsn, dbCleanup, err = startCompose()
		if err != nil {
			log.Fatalf("start postgres: %v", err)
		}
	}

	var srvCleanup func()
	var err error
	srv, srvCleanup, err = testserver.New(dsn)
	if err != nil {
		dbCleanup()
		log.Fatalf("start server: %v", err)
	}

	// Run Go integration tests.
	code := m.Run()

	// Run external adapters (vitest for UI) if source tree is present.
	runExternalAdapters()

	srvCleanup()
	dbCleanup()
	os.Exit(code)
}

// runExternalAdapters runs non-Go adapters registered for this binary,
// writing results to .zdx/test-results-external.json.
// The vitest adapter is auto-registered when ui/ is found in the source tree.
func runExternalAdapters() {
	uiDir := os.Getenv("DX_UI_DIR")
	if uiDir == "" {
		if root, err := findRoot(); err == nil {
			candidate := filepath.Join(root, "ui")
			if _, err := os.Stat(filepath.Join(candidate, "package.json")); err == nil {
				uiDir = candidate
			}
		}
	}
	if uiDir == "" {
		return
	}

	h := testharness.New()
	h.Register(&testharness.VitestAdapter{Dir: uiDir, Comp: "ui"})

	results, err := h.Run(context.Background(), testharness.FilterFromEnv())
	if err != nil {
		log.Printf("external adapters: %v", err)
		return
	}
	testharness.Summary(results)
	_ = testharness.WriteResults(".zdx/test-results-external.json", results)
}

func startCompose() (dsn string, cleanup func(), err error) {
	root, err := findRoot()
	if err != nil {
		return "", nil, err
	}
	composeFile := filepath.Join(root, "docker", "e2e.compose.yaml")

	up := exec.Command("docker", "compose", "-f", composeFile, "-p", "zdx-e2e", "up", "-d", "--wait")
	up.Stdout = os.Stderr
	up.Stderr = os.Stderr
	if err := up.Run(); err != nil {
		return "", nil, fmt.Errorf("docker compose up: %w", err)
	}

	cleanup = func() {
		exec.Command("docker", "compose", "-f", composeFile, "-p", "zdx-e2e", "down", "-v").Run() //nolint:errcheck
	}

	portOut, err := exec.Command("docker", "compose", "-f", composeFile, "-p", "zdx-e2e", "port", "postgres", "5432").Output()
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("get postgres port: %w", err)
	}
	addr := strings.TrimSpace(string(portOut))

	// Export Valkey address so rolling-deploy demo tests can create broker-sharing slots.
	if valkeyOut, verr := exec.Command("docker", "compose", "-f", composeFile, "-p", "zdx-e2e", "port", "valkey", "6379").Output(); verr == nil {
		valkeyAddr := strings.TrimSpace(string(valkeyOut))
		os.Setenv("ZDX_VALKEY_ADDR", valkeyAddr) //nolint:errcheck
		origCleanup := cleanup
		cleanup = func() {
			os.Unsetenv("ZDX_VALKEY_ADDR") //nolint:errcheck
			origCleanup()
		}
	}

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

func isListMode() bool {
	for _, arg := range os.Args {
		if strings.HasPrefix(arg, "-test.list") {
			return true
		}
	}
	return false
}
