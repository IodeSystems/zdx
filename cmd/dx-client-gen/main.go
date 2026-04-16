// Command dx-client-gen regenerates internal/dxclient/models.gen.go from the
// committed OpenAPI spec snapshot at internal/dxclient/openapi.json.
//
// Flags:
//
//	-check  regenerate into a tmpdir and diff against the committed file;
//	        exit non-zero on drift. Used by bin/lint.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/iodesystems/zdx-go/internal/dxclient/gen"
)

func main() {
	log.SetFlags(0)
	check := flag.Bool("check", false, "regenerate into a tmpdir and diff against the committed file; exit non-zero on drift")
	flag.Parse()

	root, err := findRepoRoot()
	if err != nil {
		log.Fatal(err)
	}
	specPath := filepath.Join(root, "internal", "dxclient", "openapi.json")
	configPath := filepath.Join(root, "internal", "dxclient", "oapi-codegen.yaml")

	if *check {
		if err := runCheck(root, specPath, configPath); err != nil {
			log.Fatalf("dx-client-gen -check: %v", err)
		}
		return
	}

	outPath, err := gen.Generate(specPath, configPath)
	if err != nil {
		log.Fatalf("dx-client-gen: %v", err)
	}
	fmt.Printf("regenerated %s\n", outPath)
}

// runCheck regenerates models.gen.go into a temp directory and diffs the
// result against the committed copy. Returns an error if they differ so
// bin/lint can fail with a clear remediation message.
func runCheck(root, specPath, configPath string) error {
	tmp, err := os.MkdirTemp("", "zdx-dxclient-drift-*")
	if err != nil {
		return fmt.Errorf("create tmp: %w", err)
	}
	defer os.RemoveAll(tmp)

	tmpConfigPath := filepath.Join(tmp, "oapi-codegen.yaml")
	tmpOutPath := filepath.Join(tmp, "models.gen.go")
	cfgBytes, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	patched := bytes.ReplaceAll(cfgBytes,
		[]byte("output: internal/dxclient/models.gen.go"),
		[]byte("output: "+tmpOutPath),
	)
	if !bytes.Contains(patched, []byte("output: "+tmpOutPath)) {
		return fmt.Errorf("config file does not contain expected output line; update runCheck")
	}
	if err := os.WriteFile(tmpConfigPath, patched, 0644); err != nil {
		return fmt.Errorf("write tmp config: %w", err)
	}

	if _, err := gen.Generate(specPath, tmpConfigPath); err != nil {
		return fmt.Errorf("regenerate: %w", err)
	}

	committedPath := filepath.Join(root, "internal", "dxclient", "models.gen.go")
	committed, err := os.ReadFile(committedPath)
	if err != nil {
		return fmt.Errorf("read committed: %w", err)
	}
	regenerated, err := os.ReadFile(tmpOutPath)
	if err != nil {
		return fmt.Errorf("read regenerated: %w", err)
	}
	if bytes.Equal(committed, regenerated) {
		return nil
	}

	// Surface the diff so the operator sees what changed.
	diff := exec.Command("diff", "-u", committedPath, tmpOutPath)
	diff.Stdout = os.Stdout
	diff.Stderr = os.Stderr
	_ = diff.Run()

	return fmt.Errorf("drift detected — committed internal/dxclient/models.gen.go does not match regenerated output.\nRun: make gen-dxclient")
}

func findRepoRoot() (string, error) {
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
			return "", fmt.Errorf("go.mod not found in %s or any parent", dir)
		}
		dir = parent
	}
}
