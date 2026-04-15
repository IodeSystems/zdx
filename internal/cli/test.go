package cli

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
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/config"
	"github.com/iodesystems/zdx-go/internal/testharness"
)

// TestResult is written to .zdx/test-results.json after a run.
type TestResult struct {
	Component  string `json:"component"`
	Suite      string `json:"suite"`
	Runner     string `json:"runner"`
	Status     string `json:"status"` // pass | fail | skip
	DurationMs int64  `json:"duration_ms"`
	RunAt      string `json:"run_at"`
	Output     string `json:"output,omitempty"`
	Branch     string `json:"branch,omitempty"`
	GitSHA     string `json:"git_sha,omitempty"`
}

func gitBranch() string {
	out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func gitSHA() string {
	out, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func TestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Run tests across all components (vitest + Go e2e)",
		Long: `Fuses all test adapters into one run. Adapters are auto-detected:
  - vitest (ui/)         unit layer   — requires source checkout
  - bin/zdx-test          integration + demo — built with: dx test e2e build

Filters apply across all adapters. Use DX_TEST_* env vars to parameterise
without flags (useful for CI matrix). Pass --layer demo (or --filter Demo)
to target the browser/CLI recording tests.`,
		RunE: testHarnessRunE,
	}
	cmd.Flags().String("filter", "", "test name substring/regex (applied to all adapters)")
	cmd.Flags().String("component", "", "run only this component (e.g. ui, api)")
	cmd.Flags().String("feature", "", "run only tests whose name contains this feature token")
	cmd.Flags().String("layer", "", "unit | integration | demo")
	cmd.Flags().Bool("coverage", false, "collect Go binary-level coverage (GOCOVERDIR)")
	cmd.Flags().String("db-url", "", "database URL for e2e adapter (skips docker compose)")
	cmd.Flags().String("shard", "", "shard N/M across the e2e adapter")
	// Legacy sub-commands kept for compatibility.
	cmd.AddCommand(testListCmd(), testRunCmd(), testE2ECmd())
	return cmd
}

func testHarnessRunE(cmd *cobra.Command, _ []string) error {
	filter, _ := cmd.Flags().GetString("filter")
	component, _ := cmd.Flags().GetString("component")
	feature, _ := cmd.Flags().GetString("feature")
	layer, _ := cmd.Flags().GetString("layer")
	coverage, _ := cmd.Flags().GetBool("coverage")
	dbURL, _ := cmd.Flags().GetString("db-url")
	shard, _ := cmd.Flags().GetString("shard")

	f := testharness.Filter{
		Name:      filter,
		Component: component,
		Feature:   feature,
		Layer:     testharness.Layer(layer),
	}

	h := testharness.New()

	// ── Vitest adapter (UI / unit) ─────────────────────────────────────────
	if f.Component == "" || f.Component == "ui" {
		if f.Layer == "" || f.Layer == testharness.LayerUnit {
			if uiDir := detectUIDir(); uiDir != "" {
				h.Register(&testharness.VitestAdapter{Dir: uiDir, Comp: "ui"})
			}
		}
	}

	// ── Go binary adapter (API / integration + demo) ──────────────────────
	if f.Component == "" || f.Component == "api" || f.Component == "demo" {
		wantsDemo := f.Layer == testharness.LayerDemo || f.Component == "demo"
		wantsIntegration := f.Layer == "" || f.Layer == testharness.LayerIntegration
		if wantsIntegration || wantsDemo {
			if _, err := os.Stat(testBin); err == nil {
				env := buildE2EEnv(dbURL)
				coverDir := ""
				if coverage {
					coverDir = testharness.CoverageDir("api")
				}
				// --layer demo (or --component demo) narrows to TestDemo* when
				// the user hasn't already supplied a filter.
				if wantsDemo && !wantsIntegration && f.Name == "" {
					f.Name = "^TestDemo"
				}
				a := &testharness.GoBinAdapter{
					Bin:      testBin,
					Comp:     "api",
					Layer_:   []testharness.Layer{testharness.LayerIntegration, testharness.LayerDemo},
					Env:      env,
					CoverDir: coverDir,
				}
				if shard != "" {
					_ = shard
				}
				h.Register(a)
			} else {
				fmt.Fprintln(os.Stderr, "[test] e2e binary not found — skipping api adapter (run: dx test e2e build)")
			}
		}
	}

	runStart := time.Now()
	results, err := h.Run(context.Background(), f)
	if err != nil {
		return err
	}
	branch, sha := gitBranch(), gitSHA()
	for i := range results {
		results[i].Branch = branch
		results[i].GitSHA = sha
	}
	testharness.Summary(results)
	_ = testharness.WriteResults(filepath.Join(".zdx", "test-results.json"), results)

	if f.Layer == testharness.LayerDemo || f.Layer == "" {
		metas := testharness.CollectDemoMetadata(filepath.Join(".zdx", "demo"), runStart)
		if len(metas) > 0 {
			_ = testharness.WriteDemoMetadata(filepath.Join(".zdx", "demo", "metadata.jsonl"), metas)
			fmt.Fprintf(os.Stderr, "[demo] %d artifact(s) recorded → .zdx/demo/metadata.jsonl\n", len(metas))
		}
	}

	if coverage {
		for _, comp := range []string{"api"} {
			dir := testharness.CoverageDir(comp)
			profile := filepath.Join(".zdx", "coverage", comp+".txt")
			html := filepath.Join(".zdx", "coverage", comp+".html")
			if err := testharness.MergeCoverage(dir, profile); err != nil {
				fmt.Fprintf(os.Stderr, "[coverage] %v\n", err)
				continue
			}
			if err := testharness.CoverageReport(profile, html); err != nil {
				fmt.Fprintf(os.Stderr, "[coverage] %v\n", err)
				continue
			}
			fmt.Fprintf(os.Stderr, "[coverage] %s → %s\n", comp, html)
		}
	}

	if testharness.HasFailure(results) {
		return fmt.Errorf("tests failed")
	}
	return nil
}

func detectUIDir() string {
	if d := os.Getenv("DX_UI_DIR"); d != "" {
		return d
	}
	// Resolve relative to cwd.
	if _, err := os.Stat(filepath.Join("ui", "package.json")); err == nil {
		return "ui"
	}
	return ""
}

func buildE2EEnv(dbURL string) []string {
	if dbURL != "" {
		return []string{"TEST_DATABASE_URL=" + dbURL}
	}
	return nil
}

func testListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all tests across all adapters",
		RunE: func(cmd *cobra.Command, args []string) error {
			component, _ := cmd.Flags().GetString("component")
			layer, _ := cmd.Flags().GetString("layer")

			// ── vitest ────────────────────────────────────────────────────
			if component == "" || component == "ui" {
				if layer == "" || layer == string(testharness.LayerUnit) {
					if uiDir := detectUIDir(); uiDir != "" {
						fmt.Printf("  %-8s %-12s %s\n", "ui", "unit", uiDir+" (vitest)")
					}
				}
			}

			// ── zdx-test binary (integration + demo) ──────────────────────
			if component == "" || component == "api" || component == "demo" {
				wantsDemo := layer == string(testharness.LayerDemo) || component == "demo"
				wantsIntegration := layer == "" || layer == string(testharness.LayerIntegration)
				if wantsIntegration || wantsDemo {
					if _, err := os.Stat(testBin); err == nil {
						a := &testharness.GoBinAdapter{Bin: testBin, Comp: "api"}
						names, err := a.List(context.Background())
						if err != nil {
							fmt.Fprintf(os.Stderr, "  [api] list failed: %v\n", err)
						} else {
							for _, n := range names {
								l := "integration"
								if strings.HasPrefix(n, "TestDemo") {
									l = "demo"
								}
								if wantsDemo && !wantsIntegration && l != "demo" {
									continue
								}
								if wantsIntegration && !wantsDemo && l != "integration" {
									continue
								}
								fmt.Printf("  %-8s %-12s %s\n", "api", l, n)
							}
						}
					} else {
						fmt.Printf("  %-8s %-12s %s\n", "api", "integration", "(build: dx test e2e build)")
					}
				}
			}
			return nil
		},
	}
	cmd.Flags().String("component", "", "filter to component")
	cmd.Flags().String("layer", "", "filter to layer")
	return cmd
}

func testRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run [component]",
		Short: "Run test suites",
		RunE:  testRunE,
	}
	cmd.Flags().String("filter", "", "only run suites whose name contains this string")
	cmd.Flags().String("shard", "", "run shard N/M (e.g. 1/3)")
	return cmd
}

func testRunE(cmd *cobra.Command, args []string) error {
	listOnly, _ := cmd.Flags().GetBool("list")
	if listOnly {
		return testListCmd().RunE(cmd, args)
	}

	component := config.ActiveComponent("")
	if len(args) > 0 {
		component = args[0]
	}
	filter, _ := cmd.Flags().GetString("filter")
	shardStr, _ := cmd.Flags().GetString("shard")

	cfg := config.Load()
	if cfg == nil {
		return fmt.Errorf("no .zdx/config.yaml found")
	}

	suites := sortedSuites(cfg.AllTestSuites(component))
	if filter != "" {
		var filtered []config.NamedSuite
		for _, s := range suites {
			if strings.Contains(s.Component+":"+s.Name, filter) {
				filtered = append(filtered, s)
			}
		}
		suites = filtered
	}
	if shardStr != "" {
		suites = applyShard(suites, shardStr)
	}
	if len(suites) == 0 {
		return fmt.Errorf("no test suites matched")
	}

	var results []TestResult
	failed := false
	legacyBranch, legacySHA := gitBranch(), gitSHA()

	for _, s := range suites {
		fmt.Printf("\n● %s:%s", s.Component, s.Name)
		if s.Runner != "" {
			fmt.Printf(" [%s]", s.Runner)
		}
		fmt.Println()

		if s.Setup != "" {
			fmt.Printf("  setup: %s\n", s.Setup)
			if err := runShell(s.Setup, s.CWD); err != nil {
				fmt.Fprintf(os.Stderr, "  setup FAILED: %v\n", err)
				results = append(results, TestResult{
					Component: s.Component, Suite: s.Name, Runner: s.Runner,
					Status: "fail", RunAt: time.Now().Format(time.RFC3339),
					Output: fmt.Sprintf("setup failed: %v", err),
					Branch: legacyBranch, GitSHA: legacySHA,
				})
				failed = true
				continue
			}
		}

		start := time.Now()
		err := runShell(s.Run, s.CWD)
		dur := time.Since(start).Milliseconds()

		if s.Teardown != "" {
			fmt.Printf("  teardown: %s\n", s.Teardown)
			_ = runShell(s.Teardown, s.CWD)
		}

		status := "pass"
		if err != nil {
			status = "fail"
			failed = true
			fmt.Fprintf(os.Stderr, "  FAILED: %v\n", err)
		}
		results = append(results, TestResult{
			Component: s.Component, Suite: s.Name, Runner: s.Runner,
			Status: status, DurationMs: dur,
			RunAt:  time.Now().Format(time.RFC3339),
			Branch: legacyBranch, GitSHA: legacySHA,
		})
	}

	// Write results
	if err := os.MkdirAll(".zdx", 0755); err == nil {
		if b, err := json.MarshalIndent(results, "", "  "); err == nil {
			_ = os.WriteFile(".zdx/test-results.json", b, 0644)
		}
	}

	if failed {
		return fmt.Errorf("tests failed")
	}
	fmt.Println("\n✓ all tests passed")
	return nil
}

func sortedSuites(suites []config.NamedSuite) []config.NamedSuite {
	sort.Slice(suites, func(i, j int) bool {
		if suites[i].Component != suites[j].Component {
			return suites[i].Component < suites[j].Component
		}
		return suites[i].Name < suites[j].Name
	})
	return suites
}

// applyShard selects the Nth bucket out of M (1-indexed).
func applyShard(suites []config.NamedSuite, spec string) []config.NamedSuite {
	parts := strings.SplitN(spec, "/", 2)
	if len(parts) != 2 {
		fmt.Fprintf(os.Stderr, "invalid shard spec %q, expected N/M\n", spec)
		return suites
	}
	n, err1 := strconv.Atoi(parts[0])
	m, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil || n < 1 || m < 1 || n > m {
		fmt.Fprintf(os.Stderr, "invalid shard spec %q\n", spec)
		return suites
	}
	var out []config.NamedSuite
	for i, s := range suites {
		if (i%m)+1 == n {
			out = append(out, s)
		}
	}
	return out
}

// RunTest kept for compatibility.
func RunTest(_ []string) {}

// ── e2e ───────────────────────────────────────────────────────────────────────

const testBin = "bin/zdx-test"
const testPkg = "./test/e2e/"

func testE2ECmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "e2e",
		Short: "Distributable e2e test binary",
	}
	cmd.AddCommand(testE2EBuildCmd(), testE2ERunCmd())
	return cmd
}

func testE2EBuildCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "build",
		Short: "Compile e2e test binary to " + testBin,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runShell("go test -c -o "+testBin+" "+testPkg, "")
		},
	}
}

func testE2ERunCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "run",
		Short: "Run e2e tests (local or remote)",
		RunE:  testE2ERunE,
	}
	c.Flags().StringArray("host", nil, "remote SSH host(s); auto-shards across hosts when more than one")
	c.Flags().String("shard", "", "explicit shard N/M (e.g. 2/3)")
	c.Flags().String("filter", "", "test name regex (-test.run)")
	c.Flags().String("db-url", "", "database URL; skips docker compose when set")
	return c
}

func testE2ERunE(cmd *cobra.Command, _ []string) error {
	hosts, _ := cmd.Flags().GetStringArray("host")
	shard, _ := cmd.Flags().GetString("shard")
	filter, _ := cmd.Flags().GetString("filter")
	dbURL, _ := cmd.Flags().GetString("db-url")

	// Ensure binary exists.
	if _, err := os.Stat(testBin); err != nil {
		fmt.Fprintf(os.Stderr, "[e2e] binary not found — building...\n")
		if err := runShell("go test -c -o "+testBin+" "+testPkg, ""); err != nil {
			return fmt.Errorf("build: %w", err)
		}
	}

	if len(hosts) == 0 {
		run, err := e2eRunArgs(shard, filter, 0, 1)
		if err != nil {
			return err
		}
		return e2eRunLocal(run, dbURL)
	}

	// Remote: build for each host (assume same arch for now).
	// If no explicit shard, auto-assign shard i/N per host.
	var allResults []TestResult
	failed := false

	for i, host := range hosts {
		n, m := i+1, len(hosts)
		if shard != "" {
			// Caller controls sharding; same shard on every host.
			n, m = parseShard(shard)
		}
		run, err := e2eRunArgs(fmt.Sprintf("%d/%d", n, m), filter, 0, 0)
		if err != nil {
			return err
		}
		results, err := e2eRunRemote(host, run, dbURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[e2e] host %s: %v\n", host, err)
			failed = true
			continue
		}
		allResults = append(allResults, results...)
		for _, r := range results {
			if r.Status == "fail" {
				failed = true
			}
		}
	}

	writeE2EResults(allResults)
	if failed {
		return fmt.Errorf("e2e tests failed")
	}
	return nil
}

// e2eRunArgs builds the argument list for the e2e binary, handling sharding
// (enumerating tests and splitting them) and filter.
func e2eRunArgs(shard, filter string, n, m int) ([]string, error) {
	args := []string{"-test.v"}

	pattern := filter
	if shard != "" {
		n, m = parseShard(shard)
	}
	if m > 1 {
		tests, err := e2eListTests()
		if err != nil {
			return nil, err
		}
		if filter != "" {
			tests = matchTests(tests, filter)
		}
		sharded := shardTests(tests, n, m)
		if len(sharded) == 0 {
			return nil, fmt.Errorf("shard %d/%d: no tests assigned", n, m)
		}
		pattern = "^(" + strings.Join(sharded, "|") + ")$"
	}
	if pattern != "" {
		args = append(args, "-test.run="+pattern)
	}
	return args, nil
}

func e2eListTests() ([]string, error) {
	out, err := exec.Command(testBin, "-test.list=.*").Output()
	if err != nil {
		return nil, fmt.Errorf("list tests: %w", err)
	}
	var tests []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			tests = append(tests, line)
		}
	}
	return tests, nil
}

func matchTests(tests []string, pattern string) []string {
	var out []string
	for _, t := range tests {
		if strings.Contains(t, pattern) {
			out = append(out, t)
		}
	}
	return out
}

func shardTests(tests []string, n, m int) []string {
	var out []string
	for i, t := range tests {
		if (i%m)+1 == n {
			out = append(out, t)
		}
	}
	return out
}

func parseShard(s string) (n, m int) {
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

// e2eRunLocal runs the e2e binary locally, streams output, and writes results.
// Output is piped through "go tool test2json" so parseTestJSON gets proper JSON events.
func e2eRunLocal(args []string, dbURL string) error {
	t2jArgs := append([]string{"tool", "test2json", testBin}, args...)
	c := exec.Command("go", t2jArgs...)
	if dbURL != "" {
		c.Env = append(os.Environ(), "TEST_DATABASE_URL="+dbURL)
	} else {
		c.Env = os.Environ()
	}

	var jsonBuf bytes.Buffer
	c.Stdout = io.MultiWriter(os.Stdout, &jsonBuf)
	c.Stderr = os.Stderr
	_ = c.Run()

	results := parseTestJSON(jsonBuf.Bytes())
	writeE2EResults(results)

	for _, r := range results {
		if r.Status == "fail" {
			return fmt.Errorf("e2e tests failed")
		}
	}
	return nil
}

// e2eRunRemote scps the binary to host, runs it via ssh, and returns results.
func e2eRunRemote(host string, args []string, dbURL string) ([]TestResult, error) {
	remote := "/tmp/zdx-test-" + fmt.Sprintf("%d", time.Now().UnixNano())

	// Copy binary.
	scp := exec.Command("scp", "-q", testBin, host+":"+remote)
	scp.Stdout = os.Stderr
	scp.Stderr = os.Stderr
	if err := scp.Run(); err != nil {
		return nil, fmt.Errorf("scp: %w", err)
	}

	// Build remote command.
	remoteCmd := remote + " " + strings.Join(args, " ")
	if dbURL != "" {
		remoteCmd = "TEST_DATABASE_URL=" + shellQuote(dbURL) + " " + remoteCmd
	}
	// Clean up after.
	remoteCmd = "chmod +x " + remote + " && " + remoteCmd + "; rm -f " + remote

	// Pipe remote output through local test2json for JSON event parsing.
	ssh := exec.Command("ssh", host, remoteCmd)
	t2j := exec.Command("go", "tool", "test2json")

	sshOut, err := ssh.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("ssh stdout pipe: %w", err)
	}
	ssh.Stderr = os.Stderr
	t2j.Stdin = sshOut

	var out bytes.Buffer
	t2j.Stdout = io.MultiWriter(os.Stdout, &out)
	t2j.Stderr = os.Stderr

	if err := ssh.Start(); err != nil {
		return nil, fmt.Errorf("ssh start: %w", err)
	}
	if err := t2j.Start(); err != nil {
		_ = ssh.Process.Kill()
		return nil, fmt.Errorf("test2json start: %w", err)
	}

	_ = ssh.Wait()
	_ = t2j.Wait()

	return parseTestJSON(out.Bytes()), nil
}

// parseTestJSON converts go test -json output lines into TestResult records.
func parseTestJSON(data []byte) []TestResult {
	type event struct {
		Action  string  `json:"Action"`
		Test    string  `json:"Test"`
		Elapsed float64 `json:"Elapsed"`
		Output  string  `json:"Output"`
	}
	var results []TestResult
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		var ev event
		if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
			continue
		}
		if ev.Test == "" {
			continue
		}
		switch ev.Action {
		case "pass", "fail", "skip":
			results = append(results, TestResult{
				Suite:      ev.Test,
				Status:     ev.Action,
				DurationMs: int64(ev.Elapsed * 1000),
				RunAt:      time.Now().Format(time.RFC3339),
			})
		}
	}
	return results
}

func writeE2EResults(results []TestResult) {
	if len(results) == 0 {
		return
	}
	_ = os.MkdirAll(".zdx", 0755)
	if b, err := json.MarshalIndent(results, "", "  "); err == nil {
		_ = os.WriteFile(".zdx/test-results.json", b, 0644)
	}
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
