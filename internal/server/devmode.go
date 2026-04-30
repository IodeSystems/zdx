package server

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/iodesystems/zdx-go/internal/dxclient/gen"
)

// DevCheckClient regenerates the UI and Go OpenAPI clients when the server's
// spec has drifted from their last recorded hash. Runs only when the server
// was built without a release SHA (i.e. in dev).
//
// The two regenerators are independent: each has its own hash sidecar, so a
// failure in one does not prevent the other from running. tsc and go build
// validation belong in bin/lint — we deliberately keep this fast.
//
// projectRoot must be the repository root — the working directory when
// running the server in development.
func (s *Server) DevCheckClient(projectRoot string) {
	specBytes, err := json.Marshal(s.api.OpenAPI())
	if err != nil {
		log.Printf("[devmode] could not marshal OpenAPI spec: %v", err)
		return
	}
	sum := fmt.Sprintf("%x", sha256.Sum256(specBytes))

	s.devCheckUIClient(projectRoot, specBytes, sum)
	s.devCheckGoClient(projectRoot, specBytes, sum)
}

// devCheckUIClient regenerates ui/src/api.gen.ts via openapi-typescript when
// the spec hash differs from ui/src/api.gen.ts.sha256.
func (s *Server) devCheckUIClient(projectRoot string, specBytes []byte, sum string) {
	hashFile := filepath.Join(projectRoot, "ui", "src", "api.gen.ts.sha256")
	outFile := filepath.Join(projectRoot, "ui", "src", "api.gen.ts")

	if stored, _ := os.ReadFile(hashFile); string(stored) == sum {
		if _, err := os.Stat(outFile); err == nil {
			return
		}
	}

	log.Printf("[devmode] OpenAPI spec changed — regenerating ui/src/api.gen.ts ...")

	tmp, err := os.CreateTemp("", "zdx-openapi-*.json")
	if err != nil {
		log.Printf("[devmode] could not create temp file: %v", err)
		return
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(specBytes); err != nil {
		tmp.Close()
		log.Printf("[devmode] could not write spec: %v", err)
		return
	}
	tmp.Close()

	start := time.Now()
	cmd := exec.Command("pnpm", "exec", "openapi-typescript", tmp.Name(), "-o", outFile)
	cmd.Dir = filepath.Join(projectRoot, "ui")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Printf("[devmode] openapi-typescript failed: %v", err)
		return
	}

	if err := os.WriteFile(hashFile, []byte(sum), 0644); err != nil {
		log.Printf("[devmode] could not write hash file: %v", err)
		return
	}

	log.Printf("[devmode] ui/src/api.gen.ts regenerated in %s (run bin/lint for tsc validation)", time.Since(start).Round(time.Millisecond))
}

// devCheckGoClient regenerates internal/dxclient/models.gen.go via
// oapi-codegen when the spec hash differs from
// internal/dxclient/openapi.json.sha256. Also writes the raw 3.1 spec to
// internal/dxclient/openapi.json so the snapshot is self-contained (lint can
// regenerate without a running server).
func (s *Server) devCheckGoClient(projectRoot string, specBytes []byte, sum string) {
	clientDir := filepath.Join(projectRoot, "internal", "dxclient")
	hashFile := filepath.Join(clientDir, "openapi.json.sha256")
	specFile := filepath.Join(clientDir, "openapi.json")
	outFile := filepath.Join(clientDir, "models.gen.go")
	configFile := filepath.Join(clientDir, "oapi-codegen.yaml")

	if stored, _ := os.ReadFile(hashFile); string(stored) == sum {
		if _, err := os.Stat(outFile); err == nil {
			return
		}
	}

	log.Printf("[devmode] OpenAPI spec changed — regenerating internal/dxclient/models.gen.go ...")

	if err := os.WriteFile(specFile, specBytes, 0644); err != nil {
		log.Printf("[devmode] could not write %s: %v", specFile, err)
		return
	}

	start := time.Now()
	if _, err := gen.Generate(specFile, configFile); err != nil {
		log.Printf("[devmode] oapi-codegen failed: %v", err)
		return
	}

	if err := os.WriteFile(hashFile, []byte(sum), 0644); err != nil {
		log.Printf("[devmode] could not write hash file: %v", err)
		return
	}

	log.Printf("[devmode] internal/dxclient/models.gen.go regenerated in %s (run bin/lint for go build validation)", time.Since(start).Round(time.Millisecond))
}
