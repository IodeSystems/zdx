package testharness

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// GoUnitAdapter runs go test -json directly against Go source packages
// and maps results to testharness Results with LayerUnit.
type GoUnitAdapter struct {
	// Pkgs is the package patterns to test (e.g. "./internal/...").
	Pkgs []string
	// Comp is the component name reported in results.
	Comp string
}

func (a *GoUnitAdapter) ID() string        { return "go-unit:" + a.Comp }
func (a *GoUnitAdapter) Component() string { return a.Comp }
func (a *GoUnitAdapter) Layers() []Layer   { return []Layer{LayerUnit} }

func (a *GoUnitAdapter) Run(ctx context.Context, f Filter) ([]Result, error) {
	args := []string{"test", "-json"}
	if f.Name != "" {
		args = append(args, "-run="+f.Name)
	} else if f.Feature != "" {
		args = append(args, "-run="+f.Feature)
	}
	args = append(args, a.Pkgs...)

	cmd := exec.CommandContext(ctx, "go", args...)
	var buf bytes.Buffer
	cmd.Stdout = io.MultiWriter(os.Stdout, &buf)
	cmd.Stderr = os.Stderr
	_ = cmd.Run()

	return parseGoTestJSON(buf.Bytes(), a.Comp, LayerUnit), nil
}

// List returns test names from the source packages via go test -list.
func (a *GoUnitAdapter) List(ctx context.Context) ([]string, error) {
	args := append([]string{"test", "-list=.*"}, a.Pkgs...)
	out, err := exec.CommandContext(ctx, "go", args...).Output()
	if err != nil && len(out) == 0 {
		return nil, fmt.Errorf("list unit tests: %w", err)
	}
	var names []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "ok ") || strings.HasPrefix(line, "?") || strings.HasPrefix(line, "FAIL") {
			continue
		}
		names = append(names, line)
	}
	return names, nil
}
