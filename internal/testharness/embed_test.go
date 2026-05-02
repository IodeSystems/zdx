package testharness_test

import (
	"encoding/json"
	"io"
	"os/exec"
	"strings"
	"testing"
)

// TestEmbed_NoZdxInternalDeps asserts the testharness package can be embedded
// by external projects without pulling in zdx-internal packages. Spec 36.
//
// A refactor that adds an import of any github.com/iodesystems/zdx-go/internal/*
// package (other than testharness itself) breaks the embedding contract — this
// test fails fast so the regression cannot ship silently.
func TestEmbed_NoZdxInternalDeps(t *testing.T) {
	const self = "github.com/iodesystems/zdx-go/internal/testharness"
	const internalPrefix = "github.com/iodesystems/zdx-go/internal/"

	cmd := exec.Command("go", "list", "-json", "-deps", self)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("StdoutPipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("go list start: %v", err)
	}

	dec := json.NewDecoder(stdout)
	var leaks []string
	for {
		var pkg struct {
			ImportPath string
		}
		if err := dec.Decode(&pkg); err != nil {
			if err == io.EOF {
				break
			}
			t.Fatalf("decode go list output: %v", err)
		}
		if pkg.ImportPath == self {
			continue
		}
		if strings.HasPrefix(pkg.ImportPath, internalPrefix) {
			leaks = append(leaks, pkg.ImportPath)
		}
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("go list wait: %v", err)
	}

	if len(leaks) > 0 {
		t.Fatalf("testharness must not depend on zdx-internal packages; leaked deps:\n  %s", strings.Join(leaks, "\n  "))
	}
}
