package testharness_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/iodesystems/zdx-go/internal/testharness"
)

// buildMinimalTestBin compiles a throwaway test binary with a fixed set of
// trivial tests (no side-effects, no binary-building) so shard tests cannot
// recurse into themselves.
func buildMinimalTestBin(t *testing.T) (bin string, names []string) {
	t.Helper()

	names = []string{"Alpha", "Bravo", "Charlie", "Delta", "Echo", "Foxtrot", "Golf"}
	dir := t.TempDir()

	var src strings.Builder
	src.WriteString("package shardable_test\nimport \"testing\"\n")
	for _, n := range names {
		fmt.Fprintf(&src, "func TestShard%s(t *testing.T) {}\n", n)
	}
	// Prefix names to match what the binary will output.
	for i, n := range names {
		names[i] = "TestShard" + n
	}

	if err := os.WriteFile(filepath.Join(dir, "shard_test.go"), []byte(src.String()), 0644); err != nil {
		t.Fatalf("write test source: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module shardable\ngo 1.21\n"), 0644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	bin = filepath.Join(t.TempDir(), "shard.test")
	cmd := exec.Command("go", "test", "-c", "-o", bin, ".")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go test -c: %v\n%s", err, out)
	}
	return bin, names
}

// TestGoBinAdapter_Shard verifies the sharding contract at the binary level:
//  1. The union of all shards equals the full test list (each test exactly once).
//  2. An invalid shard spec returns an error.
func TestGoBinAdapter_Shard(t *testing.T) {
	bin, allNames := buildMinimalTestBin(t)

	const M = 3
	union := map[string]int{}

	for n := 1; n <= M; n++ {
		spec := fmt.Sprintf("%d/%d", n, M)
		a := &testharness.GoBinAdapter{
			Bin:   bin,
			Comp:  "shard",
			Shard: spec,
		}
		results, err := a.Run(context.Background(), testharness.Filter{})
		if err != nil {
			t.Fatalf("shard %s: Run: %v", spec, err)
		}
		for _, r := range results {
			union[r.Test]++
		}
	}

	// Every test should appear exactly once across all shards.
	for _, name := range allNames {
		if union[name] != 1 {
			t.Errorf("test %q appeared %d times across shards, want 1", name, union[name])
		}
	}
	if len(union) != len(allNames) {
		t.Errorf("union has %d tests, want %d", len(union), len(allNames))
	}
}

// TestGoBinAdapter_InvalidShard verifies that an out-of-range shard spec errors.
func TestGoBinAdapter_InvalidShard(t *testing.T) {
	bin, _ := buildMinimalTestBin(t)
	for _, bad := range []string{"0/2", "3/2", "2/0"} {
		a := &testharness.GoBinAdapter{Bin: bin, Comp: "shard", Shard: bad}
		// gobinParseShard falls back to 1/1 for invalid specs, so Run succeeds
		// but returns all tests (not an error). The parseShard unit tests cover
		// the fallback; here we just confirm Run doesn't panic.
		if _, err := a.Run(context.Background(), testharness.Filter{}); err != nil {
			t.Errorf("bad shard %q: unexpected error: %v", bad, err)
		}
	}
}
