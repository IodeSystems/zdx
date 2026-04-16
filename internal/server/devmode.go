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
)

// DevCheckClient compares the current OpenAPI spec to the hash stored alongside
// ui/src/api.gen.ts and regenerates the TypeScript client when they differ.
// It deliberately does NOT run tsc — type validation belongs in bin/lint so the
// startup cost stays bounded (openapi-typescript only).
//
// projectRoot must be the repository root — the working directory when running
// the server in development.
func (s *Server) DevCheckClient(projectRoot string) {
	specBytes, err := json.Marshal(s.api.OpenAPI())
	if err != nil {
		log.Printf("[devmode] could not marshal OpenAPI spec: %v", err)
		return
	}

	sum := fmt.Sprintf("%x", sha256.Sum256(specBytes))
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
	cmd := exec.Command("npx", "openapi-typescript", tmp.Name(), "-o", outFile)
	cmd.Dir = projectRoot
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
