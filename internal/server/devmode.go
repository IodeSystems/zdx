package server

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

// DevCheckClient compares the current OpenAPI spec to the hash stored alongside
// ui/src/api.gen.ts. If the spec changed, it regenerates the TS client and then
// runs tsc --noEmit to validate the result. Fatal on tsc failure so the dev
// gets an immediate, readable error rather than silent drift.
//
// projectRoot must be the repository root (i.e., the working directory when
// running the server in development).
func (s *Server) DevCheckClient(projectRoot string) {
	specBytes, err := json.Marshal(s.api.OpenAPI())
	if err != nil {
		log.Printf("[devmode] could not marshal OpenAPI spec: %v", err)
		return
	}

	sum := fmt.Sprintf("%x", sha256.Sum256(specBytes))
	hashFile := filepath.Join(projectRoot, "ui", "src", "api.gen.ts.sha256")

	stored, _ := os.ReadFile(hashFile)
	if string(stored) == sum {
		log.Printf("[devmode] OpenAPI spec unchanged — skipping client regen")
		return
	}

	log.Printf("[devmode] OpenAPI spec changed — regenerating ui/src/api.gen.ts ...")

	// Write spec to temp file.
	tmp, err := os.CreateTemp("", "zdx-openapi-*.json")
	if err != nil {
		log.Printf("[devmode] could not create temp file: %v", err)
		return
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(specBytes); err != nil {
		log.Printf("[devmode] could not write spec: %v", err)
		return
	}
	tmp.Close()

	outFile := filepath.Join(projectRoot, "ui", "src", "api.gen.ts")

	// Run openapi-typescript.
	cmd := exec.Command("npx", "--yes", "openapi-typescript", tmp.Name(), "-o", outFile)
	cmd.Dir = projectRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Printf("[devmode] openapi-typescript failed: %v", err)
		return
	}

	// Run tsc to validate the generated file compiles cleanly.
	tsc := exec.Command("npx", "tsc", "--noEmit", "--project", filepath.Join(projectRoot, "ui", "tsconfig.json"))
	tsc.Dir = projectRoot
	out, err := tsc.CombinedOutput()
	if err != nil {
		log.Fatalf(
			"[devmode] api.gen.ts was regenerated but TypeScript type-check failed.\n"+
				"Fix the errors below, or run: bin/api-types && npm --prefix ui run build\n\n%s",
			out,
		)
	}

	// Store new hash only after successful tsc.
	if err := os.WriteFile(hashFile, []byte(sum), 0644); err != nil {
		log.Printf("[devmode] could not write hash file: %v", err)
	}

	log.Printf("[devmode] client regenerated and type-checked OK")
}
