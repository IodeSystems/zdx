package devtools

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

// UICmd returns the "dx ui" subcommand group.
func UICmd() *cobra.Command {
	ui := &cobra.Command{
		Use:   "ui",
		Short: "UI development utilities",
	}
	ui.AddCommand(genAPICmd())
	return ui
}

func genAPICmd() *cobra.Command {
	return &cobra.Command{
		Use:   "gen-api",
		Short: "Regenerate api.gen.ts and models.gen.go from the server's OpenAPI spec",
		Long: `Runs dx-server --openapi to get the current spec (no database or network
required), then regenerates both ui/src/api.gen.ts (via openapi-typescript) and
internal/dxclient/models.gen.go (via oapi-codegen). Updates all hash sidecars
so bin/lint's drift checks pass. Run bin/lint to validate the result.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := findRepoRoot()
			if err != nil {
				return err
			}
			return runGenAPI(root)
		},
	}
}

func runGenAPI(root string) error {
	serverBin, err := findServerBin(root)
	if err != nil {
		return err
	}

	fmt.Printf("● gen-api: fetching OpenAPI spec from %s --openapi\n", serverBin)
	specBytes, err := exec.Command(serverBin, "--openapi").Output()
	if err != nil {
		return fmt.Errorf("dx-server --openapi: %w", err)
	}

	sum := fmt.Sprintf("%x", sha256.Sum256(specBytes))

	uiHashFile := filepath.Join(root, "ui", "src", "api.gen.ts.sha256")
	goHashFile := filepath.Join(root, "internal", "dxclient", "openapi.json.sha256")

	uiUpToDate := func() bool {
		stored, _ := os.ReadFile(uiHashFile)
		if string(stored) != sum {
			return false
		}
		_, err := os.Stat(filepath.Join(root, "ui", "src", "api.gen.ts"))
		return err == nil
	}

	goUpToDate := func() bool {
		stored, _ := os.ReadFile(goHashFile)
		if string(stored) != sum {
			return false
		}
		_, err := os.Stat(filepath.Join(root, "internal", "dxclient", "models.gen.go"))
		return err == nil
	}

	if uiUpToDate() && goUpToDate() {
		fmt.Println("● gen-api: all clients are already up to date")
		return nil
	}

	tmp, err := os.CreateTemp("", "zdx-openapi-*.json")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(specBytes); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp: %w", err)
	}
	tmp.Close()

	if !uiUpToDate() {
		outFile := filepath.Join(root, "ui", "src", "api.gen.ts")
		fmt.Println("● gen-api: running openapi-typescript ...")
		start := time.Now()
		gen := exec.Command("pnpm", "exec", "openapi-typescript", tmp.Name(), "-o", outFile)
		gen.Dir = filepath.Join(root, "ui")
		gen.Stdout = os.Stdout
		gen.Stderr = os.Stderr
		if err := gen.Run(); err != nil {
			return fmt.Errorf("openapi-typescript: %w", err)
		}
		if err := os.WriteFile(uiHashFile, []byte(sum), 0644); err != nil {
			return fmt.Errorf("write ui hash: %w", err)
		}
		fmt.Printf("● gen-api: ui/src/api.gen.ts regenerated in %s\n", time.Since(start).Round(time.Millisecond))
	}

	if !goUpToDate() {
		specFile := filepath.Join(root, "internal", "dxclient", "openapi.json")
		fmt.Println("● gen-api: updating internal/dxclient/openapi.json ...")
		if err := os.WriteFile(specFile, specBytes, 0644); err != nil {
			return fmt.Errorf("write spec: %w", err)
		}
		start := time.Now()
		gen := exec.Command("go", "run", "./cmd/dx-client-gen")
		gen.Dir = root
		gen.Stdout = os.Stdout
		gen.Stderr = os.Stderr
		if err := gen.Run(); err != nil {
			return fmt.Errorf("dx-client-gen: %w", err)
		}
		if err := os.WriteFile(goHashFile, []byte(sum), 0644); err != nil {
			return fmt.Errorf("write go hash: %w", err)
		}
		fmt.Printf("● gen-api: internal/dxclient/models.gen.go regenerated in %s\n", time.Since(start).Round(time.Millisecond))
	}

	fmt.Println("● gen-api: done (run bin/lint for validation)")
	return nil
}

func findServerBin(root string) (string, error) {
	local := filepath.Join(root, "bin", "dx-server")
	if _, err := os.Stat(local); err == nil {
		return local, nil
	}
	path, err := exec.LookPath("dx-server")
	if err != nil {
		return "", fmt.Errorf("dx-server not found in bin/ or PATH; run 'make build' first")
	}
	return path, nil
}

func findRepoRoot() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("not in a git repo: %w", err)
	}
	root := string(out)
	if len(root) > 0 && root[len(root)-1] == '\n' {
		root = root[:len(root)-1]
	}
	return root, nil
}
