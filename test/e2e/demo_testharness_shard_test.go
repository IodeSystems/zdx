package e2e

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

// TestDemoCLI_ShardSplitsTestsForDistributedRuns is the demo for spec 29:
// given the test binary, when run with --shard N/M, then only the Nth shard of
// tests executes for distributed runs.
//
// The demo compiles a throwaway Go test binary with a fixed list of trivial
// tests and runs the GoBinAdapter (the same component dx test plumbs --shard
// into) once per shard at M=3. It then asserts the distribution contract that
// makes distributed runs meaningful:
//   - Each test appears in exactly one shard (no duplicates → no wasted work).
//   - The union of all shards equals the full test list (no gaps → no missed
//     coverage).
//
// These two properties together are what lets a CI fan-out across N hosts and
// trust that running 1/N..N/N covers the suite exactly once.
func TestDemoCLI_ShardSplitsTestsForDistributedRuns(t *testing.T) {
	writeDemoCoderefs(t, t.Name(), []coderef{
		{FilePath: "test/e2e/demo_testharness_shard_test.go", Note: "shard distribution demo source"},
		{FilePath: "internal/testharness/gobin.go", LineStart: 67, LineEnd: 84, Note: "GoBinAdapter.Run enumerates -test.list and partitions for --shard N/M"},
		{FilePath: "internal/testharness/gobin.go", LineStart: 192, LineEnd: 215, Note: "gobinParseShard + gobinShardTests round-robin partitioning"},
		{FilePath: "internal/cli/devtools/test.go", LineStart: 76, LineEnd: 76, Note: "dx test --shard flag wiring"},
		{FilePath: "internal/cli/devtools/test.go", LineStart: 170, LineEnd: 178, Note: "shard value passed to GoBinAdapter"},
	})

	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not available")
	}

	bin, allNames := buildShardableTestBin(t)

	const M = 3
	shards := make(map[int][]string, M)
	union := map[string]int{}

	for n := 1; n <= M; n++ {
		spec := fmt.Sprintf("%d/%d", n, M)
		a := &testharness.GoBinAdapter{Bin: bin, Comp: "shard-demo", Shard: spec}
		results, err := a.Run(context.Background(), testharness.Filter{})
		if err != nil {
			t.Fatalf("shard %s: Run: %v", spec, err)
		}
		var got []string
		for _, r := range results {
			got = append(got, r.Test)
			union[r.Test]++
		}
		shards[n] = got
	}

	for _, name := range allNames {
		if union[name] != 1 {
			t.Errorf("test %q ran %d times across shards 1/%d..%d/%d, want exactly 1\nshards: %v",
				name, union[name], M, M, M, shards)
		}
	}
	if len(union) != len(allNames) {
		t.Errorf("union covered %d tests, want %d\nshards: %v\nall: %v",
			len(union), len(allNames), shards, allNames)
	}
}

// buildShardableTestBin compiles a tiny test binary with a fixed roster of
// no-op tests so the demo can exercise sharding without depending on the
// project's real test surface (which would recurse into this very test).
func buildShardableTestBin(t *testing.T) (bin string, names []string) {
	t.Helper()

	letters := []string{"Alpha", "Bravo", "Charlie", "Delta", "Echo", "Foxtrot", "Golf"}
	dir := t.TempDir()

	var src strings.Builder
	src.WriteString("package shardable_test\nimport \"testing\"\n")
	for _, n := range letters {
		fmt.Fprintf(&src, "func TestShard%s(t *testing.T) {}\n", n)
	}
	names = make([]string, len(letters))
	for i, n := range letters {
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
		t.Fatalf("compile shardable test bin: %v\n%s", err, out)
	}
	return bin, names
}
