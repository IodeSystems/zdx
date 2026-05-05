package testharness

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// GoBinAdapter runs a compiled Go test binary (go test -c) and maps its
// -test.json output to testharness Results.
//
// The binary enumerates tests via -test.list, applies shard/filter, then runs
// with -test.json. If CoverDir is non-empty, coverage data is collected.
//
// Coverage flag note (Go 1.20+): go test -c -cover binaries require
// -test.gocoverdir; GOCOVERDIR env is silently ignored by them. go build -cover
// binaries use GOCOVERDIR. Both are passed so either binary type works.
// Use MergeCoverage to aggregate across shards afterward.
type GoBinAdapter struct {
	// Bin is the path to the compiled test binary.
	Bin string
	// Comp is the component name reported in results.
	Comp string
	// Layer_ is the layer(s) this binary covers.
	Layer_ []Layer
	// Env is extra environment variables injected when running the binary.
	Env []string
	// CoverDir, if set, is passed as both -test.gocoverdir and GOCOVERDIR to collect coverage.
	CoverDir string
	// Shard, if set, restricts the run to shard N/M (e.g. "2/3"). Tests are enumerated
	// via -test.list and partitioned round-robin; invalid specs run the full suite.
	Shard string
}

func (a *GoBinAdapter) ID() string        { return "go:" + a.Comp }
func (a *GoBinAdapter) Component() string { return a.Comp }
func (a *GoBinAdapter) Layers() []Layer {
	if len(a.Layer_) == 0 {
		return []Layer{LayerIntegration}
	}
	return a.Layer_
}

// gobinRunArgs returns the -test.* argv for the given filter.
func gobinRunArgs(f Filter) []string {
	runPattern := f.Name
	if runPattern == "" {
		runPattern = f.Feature
	}
	args := []string{"-test.v"}
	if runPattern != "" {
		args = append(args, "-test.run="+runPattern)
	}
	return args
}

// Run enumerates, optionally shards, filters, and runs the binary.
func (a *GoBinAdapter) Run(ctx context.Context, f Filter) ([]Result, error) {
	if _, err := os.Stat(a.Bin); err != nil {
		return nil, fmt.Errorf("go bin %q not found — build first", a.Bin)
	}

	if a.Shard != "" {
		n, m := gobinParseShard(a.Shard)
		tests, err := a.List(ctx)
		if err != nil {
			return nil, fmt.Errorf("list for shard: %w", err)
		}
		sharded := gobinShardTests(tests, n, m)
		if len(sharded) == 0 {
			return nil, fmt.Errorf("shard %d/%d: no tests assigned", n, m)
		}
		f.Name = "^(" + strings.Join(sharded, "|") + ")$"
	}

	args := gobinRunArgs(f)

	env := append(os.Environ(), a.Env...)
	if a.CoverDir != "" {
		if err := os.MkdirAll(a.CoverDir, 0755); err != nil {
			return nil, fmt.Errorf("cover dir: %w", err)
		}
		// go test -c binaries use -test.gocoverdir; go build -cover binaries use GOCOVERDIR.
		// Pass both so either binary type works.
		args = append(args, "-test.gocoverdir="+a.CoverDir)
		env = append(env, "GOCOVERDIR="+a.CoverDir)
	}

	// Pipe test binary output through test2json for structured JSON.
	testCmd := exec.CommandContext(ctx, a.Bin, args...)
	testCmd.Env = env
	testCmd.Stderr = os.Stderr

	testOut, err := testCmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	jsonCmd := exec.CommandContext(ctx, "go", "tool", "test2json", "-t")
	jsonCmd.Stdin = testOut

	var jsonBuf bytes.Buffer
	jsonCmd.Stdout = io.MultiWriter(os.Stdout, &jsonBuf)
	jsonCmd.Stderr = os.Stderr

	if err := jsonCmd.Start(); err != nil {
		return nil, fmt.Errorf("start test2json: %w", err)
	}
	if err := testCmd.Start(); err != nil {
		return nil, fmt.Errorf("start test binary: %w", err)
	}
	_ = testCmd.Wait()
	_ = jsonCmd.Wait()

	return parseGoTestJSON(jsonBuf.Bytes(), a.Comp, a.Layers()[0]), nil
}

// List returns test names from the binary via -test.list.
func (a *GoBinAdapter) List(ctx context.Context) ([]string, error) {
	out, err := exec.CommandContext(ctx, a.Bin, "-test.list=.*").Output()
	if err != nil {
		return nil, fmt.Errorf("list tests: %w", err)
	}
	var names []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			names = append(names, line)
		}
	}
	return names, nil
}

// ── go test -json parsing ─────────────────────────────────────────────────

func parseGoTestJSON(data []byte, component string, layer Layer) []Result {
	type event struct {
		Action  string  `json:"Action"`
		Test    string  `json:"Test"`
		Elapsed float64 `json:"Elapsed"`
		Output  string  `json:"Output"`
	}

	// Accumulate output per test.
	outputs := map[string]*strings.Builder{}
	var results []Result
	now := time.Now().UTC().Format(time.RFC3339)

	// IS-1030: test2json emits one JSON event per line of test output. When a
	// test shells out to a chatty subprocess (e.g. TestDemoCLI_* spawning
	// vitest with verbose UI test logs), a single Output event can easily
	// exceed Scanner's default 64KB buffer. The Scanner errors silently,
	// every subsequent line — including the test's terminal pass/fail event —
	// gets dropped, results comes back empty, and syncTestResults skips with
	// "0 events" while CI actually had useful pass/fail to record. Allow up
	// to 16MB per line so realistic verbose output streams parse cleanly.
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		var ev event
		if err := json.Unmarshal(sc.Bytes(), &ev); err != nil || ev.Test == "" {
			continue
		}
		switch ev.Action {
		case "output":
			b := outputs[ev.Test]
			if b == nil {
				b = &strings.Builder{}
				outputs[ev.Test] = b
			}
			b.WriteString(ev.Output)
		case "pass", "fail", "skip":
			status := ev.Action
			out := ""
			if b := outputs[ev.Test]; b != nil {
				out = b.String()
			}
			delete(outputs, ev.Test)
			results = append(results, Result{
				Test:       ev.Test,
				Component:  component,
				Layer:      layer,
				Status:     status,
				DurationMs: int64(ev.Elapsed * 1000),
				Output:     out,
				RunAt:      now,
			})
		}
	}
	return results
}

// gobinParseShard parses "N/M" and returns (n, m); falls back to (1,1) for invalid specs.
func gobinParseShard(s string) (n, m int) {
	parts := strings.SplitN(s, "/", 2)
	if len(parts) != 2 {
		return 1, 1
	}
	n, _ = strconv.Atoi(parts[0])
	m, _ = strconv.Atoi(parts[1])
	if n < 1 || m < 1 || n > m {
		return 1, 1
	}
	return n, m
}

// gobinShardTests selects the Nth bucket of tests out of M (1-indexed, round-robin).
func gobinShardTests(tests []string, n, m int) []string {
	var out []string
	for i, t := range tests {
		if (i%m)+1 == n {
			out = append(out, t)
		}
	}
	return out
}

// CoverageDir returns a default coverage directory for the given component.
func CoverageDir(component string) string {
	return filepath.Join(".zdx", "coverage", component)
}

// MergeCoverage merges coverage data from dir into a text profile at outPath.
// Requires Go 1.20+.
func MergeCoverage(dir, outPath string) error {
	if _, err := os.Stat(dir); err != nil {
		return fmt.Errorf("coverage dir %q not found", dir)
	}
	cmd := exec.Command("go", "tool", "covdata", "textfmt",
		"-i="+dir, "-o="+outPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// CoverageReport renders an HTML coverage report from a text profile.
func CoverageReport(profilePath, htmlPath string) error {
	cmd := exec.Command("go", "tool", "cover",
		"-html="+profilePath, "-o="+htmlPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
