package server

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

// DevCheck runs dev-mode startup validations in parallel:
//   - sqlc vet: validates SQL queries against the schema
//   - If the OpenAPI spec changed, regenerates ui/src/api.gen.ts then runs
//     tsc --noEmit; fatal on tsc failure
//
// projectRoot must be the repository root (i.e., the working directory when
// running the server in development).
func (s *Server) DevCheck(projectRoot string) {
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		runSqlcVet(projectRoot)
	}()

	go func() {
		defer wg.Done()
		s.checkAndRegenClient(projectRoot)
	}()

	wg.Wait()
}

func runSqlcVet(projectRoot string) {
	// sqlc may live in ~/go/bin which isn't always in PATH; resolve it.
	sqlc, err := exec.LookPath("sqlc")
	if err != nil {
		// fall back to common install location
		candidate := filepath.Join(os.Getenv("HOME"), "go", "bin", "sqlc")
		if _, ferr := os.Stat(candidate); ferr == nil {
			sqlc = candidate
		} else {
			log.Printf("[devmode] sqlc not found — skipping vet (install with: go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest)")
			return
		}
	}

	cmd := exec.Command(sqlc, "vet")
	cmd.Dir = projectRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("[devmode] sqlc vet failed — fix SQL queries before starting:\n\n%s", out)
	}
	log.Printf("[devmode] sqlc vet OK")
}

func (s *Server) checkAndRegenClient(projectRoot string) {
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
	gen := exec.Command("npx", "--yes", "openapi-typescript", tmp.Name(), "-o", outFile)
	gen.Dir = projectRoot
	gen.Stdout = os.Stdout
	gen.Stderr = os.Stderr
	if err := gen.Run(); err != nil {
		log.Printf("[devmode] openapi-typescript failed: %v", err)
		return
	}

	// tsc --noEmit must pass before we persist the new hash.
	tsc := exec.Command("npx", "tsc", "--noEmit", "--project", filepath.Join("ui", "tsconfig.json"))
	tsc.Dir = projectRoot
	var tscOut bytes.Buffer
	tsc.Stdout = &tscOut
	tsc.Stderr = &tscOut
	if err := tsc.Run(); err != nil {
		log.Fatalf(
			"[devmode] api.gen.ts was regenerated but TypeScript type-check failed.\n"+
				"Fix the TypeScript errors below, then restart dx-server.\n\n%s",
			tscOut.String(),
		)
	}

	if err := os.WriteFile(hashFile, []byte(sum), 0644); err != nil {
		log.Printf("[devmode] could not write hash file: %v", err)
	}
	log.Printf("[devmode] client regenerated and type-checked OK")
}
