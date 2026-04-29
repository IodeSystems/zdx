package devtools

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

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/config"
	"github.com/iodesystems/zdx-go/internal/devserver"
	"github.com/iodesystems/zdx-go/internal/dxclient"
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
	cmd.Flags().String("driver", "", "api | ui — selects BDD step driver (default: api)")
	cmd.Flags().Bool("coverage", false, "collect Go binary-level coverage (GOCOVERDIR)")
	cmd.Flags().String("db-url", "", "database URL for e2e adapter (skips docker compose)")
	cmd.Flags().String("shard", "", "shard N/M across the e2e adapter")
	cmd.Flags().Bool("no-ephemeral", false, "skip ephemeral devserver bootstrap (use caller's DX_API_URL instead)")
	cmd.Flags().Bool("ephemeral-per-test", false, "spin a fresh devserver per demo test (coverage accuracy; multiplies wall time)")
	// Legacy sub-commands kept for compatibility.
	cmd.AddCommand(testListCmd(), testRunCmd(), testE2ECmd())
	return cmd
}

func testHarnessRunE(cmd *cobra.Command, _ []string) error {
	filter, _ := cmd.Flags().GetString("filter")
	component, _ := cmd.Flags().GetString("component")
	feature, _ := cmd.Flags().GetString("feature")
	layer, _ := cmd.Flags().GetString("layer")
	driver, _ := cmd.Flags().GetString("driver")
	coverage, _ := cmd.Flags().GetBool("coverage")
	dbURL, _ := cmd.Flags().GetString("db-url")
	shard, _ := cmd.Flags().GetString("shard")
	noEphemeral, _ := cmd.Flags().GetBool("no-ephemeral")
	ephemeralPerTest, _ := cmd.Flags().GetBool("ephemeral-per-test")

	f := testharness.Filter{
		Name:      filter,
		Component: component,
		Feature:   feature,
		Layer:     testharness.Layer(layer),
		Driver:    testharness.Driver(driver),
	}

	// ── Ephemeral devserver bootstrap ─────────────────────────────────────
	// Spin up an isolated zdx-server (tmpdir UploadsDir + DEMOS_DIR, disposable
	// DB) once per run so tests don't pollute or depend on the dev server.
	// The test binary reads DX_API_URL/DX_API_KEY and skips its own setup.
	// Skipped when no e2e/demo work is in scope (e.g. --component ui), and
	// deferred entirely when --ephemeral-per-test requests per-test sandboxes.
	wantsE2E := (f.Component == "" || f.Component == "api" || f.Component == "demo") &&
		(f.Layer == "" || f.Layer == testharness.LayerIntegration || f.Layer == testharness.LayerDemo)
	if ephemeralPerTest && f.Layer != testharness.LayerDemo && f.Component != "demo" {
		return fmt.Errorf("--ephemeral-per-test requires --layer demo or --component demo")
	}
	var ephemeralEnv []string
	ephemeralCleanup := func() {}
	if wantsE2E && !ephemeralPerTest {
		env, cleanup, err := maybeBootstrapEphemeral(noEphemeral, dbURL)
		if err != nil {
			return err
		}
		ephemeralEnv = env
		ephemeralCleanup = cleanup
	}
	defer ephemeralCleanup()

	// Per-test ephemeral mode short-circuits the adapter plumbing: enumerate
	// TestDemo* names, run each in its own sandbox, concatenate results.
	if ephemeralPerTest {
		return runPerTestEphemeral(context.Background(), dbURL, f, coverage)
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

	// ── Go unit adapter (source packages / unit) ──────────────────────────
	if f.Component == "" || f.Component == "api" {
		if f.Layer == "" || f.Layer == testharness.LayerUnit {
			h.Register(&testharness.GoUnitAdapter{
				Pkgs: []string{"./internal/...", "./test/scripts/..."},
				Comp: "api",
			})
		}
	}

	// ── Go binary adapter (API / integration + demo) ──────────────────────
	if f.Component == "" || f.Component == "api" || f.Component == "demo" {
		wantsDemo := f.Layer == testharness.LayerDemo || f.Component == "demo"
		wantsIntegration := f.Layer == "" || f.Layer == testharness.LayerIntegration
		if wantsIntegration || wantsDemo {
			if _, err := os.Stat(testBin); err == nil {
				env := append(buildE2EEnv(dbURL), ephemeralEnv...)
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
					Shard:    shard,
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

	var metas []testharness.DemoMeta
	if f.Layer == testharness.LayerDemo || f.Layer == "" {
		metas = testharness.CollectDemoMetadata(filepath.Join(".zdx", "demo"), runStart)
		if len(metas) > 0 {
			_ = testharness.WriteDemoMetadata(filepath.Join(".zdx", "demo", "metadata.jsonl"), metas)
			fmt.Fprintf(os.Stderr, "[demo] %d artifact(s) recorded → .zdx/demo/metadata.jsonl\n", len(metas))
		}
	}

	syncTestResults(results, metas)

	if kc, err := cli.DefaultClient(); err == nil {
		if slug := kc.Slug(); slug != "" {
			postKpiSamples(context.Background(), kc, slug, results)
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

// runPerTestEphemeral enumerates demo tests and runs each inside its own
// ephemeral devserver sandbox, so coverage samples stay independent.
// Expensive by design — opt-in via --ephemeral-per-test.
func runPerTestEphemeral(ctx context.Context, dbURL string, f testharness.Filter, coverage bool) error {
	if _, err := os.Stat(testBin); err != nil {
		return fmt.Errorf("e2e binary not found — run: dx test e2e build")
	}
	adapter := &testharness.GoBinAdapter{Bin: testBin, Comp: "api"}
	names, err := adapter.List(ctx)
	if err != nil {
		return fmt.Errorf("list tests: %w", err)
	}
	var demos []string
	for _, n := range names {
		if !strings.HasPrefix(n, "TestDemo") {
			continue
		}
		if f.Name != "" && !strings.Contains(n, f.Name) {
			continue
		}
		demos = append(demos, n)
	}
	if len(demos) == 0 {
		return fmt.Errorf("no TestDemo* tests matched filter %q", f.Name)
	}

	var all []testharness.Result
	for _, test := range demos {
		fmt.Fprintf(os.Stderr, "[per-test] %s — provisioning fresh devserver\n", test)
		h, cleanup, err := devserver.Start(devserver.Options{DSN: dbURL})
		if err != nil {
			return fmt.Errorf("%s: bootstrap: %w", test, err)
		}
		fmt.Fprintf(os.Stderr, "[per-test] %s — url=%s uploads=%s demos=%s\n", test, h.URL, h.UploadsDir, h.DemosDir)

		coverDir := ""
		if coverage {
			coverDir = filepath.Join(testharness.CoverageDir("api"), test)
		}
		one := &testharness.GoBinAdapter{
			Bin:    testBin,
			Comp:   "api",
			Layer_: []testharness.Layer{testharness.LayerDemo},
			Env: []string{
				"DX_API_URL=" + h.URL,
				"DX_API_KEY=" + h.AdminToken,
				"UPLOADS_DIR=" + h.UploadsDir,
				"DEMOS_DIR=" + h.DemosDir,
				"TEST_DATABASE_URL=" + h.DSN,
			},
			CoverDir: coverDir,
		}
		results, runErr := one.Run(ctx, testharness.Filter{Name: "^" + test + "$"})
		cleanup()
		if runErr != nil {
			return fmt.Errorf("%s: %w", test, runErr)
		}
		all = append(all, results...)
	}

	branch, sha := gitBranch(), gitSHA()
	for i := range all {
		all[i].Branch = branch
		all[i].GitSHA = sha
	}
	testharness.Summary(all)
	_ = testharness.WriteResults(filepath.Join(".zdx", "test-results.json"), all)
	if testharness.HasFailure(all) {
		return fmt.Errorf("tests failed")
	}
	return nil
}

// maybeBootstrapEphemeral spins up an ephemeral devserver by default and
// returns the env additions to pass through to the e2e binary plus a cleanup.
// Returns no-op cleanup and nil env when:
//   - --no-ephemeral is set, or
//   - the caller already exported DX_API_URL (indicates they're pointing at a
//     specific server on purpose).
func maybeBootstrapEphemeral(noEphemeral bool, dbURL string) ([]string, func(), error) {
	if noEphemeral || os.Getenv("DX_API_URL") != "" {
		return nil, func() {}, nil
	}
	fmt.Fprintln(os.Stderr, "[test] bootstrapping ephemeral devserver (pass --no-ephemeral to disable)")
	h, cleanup, err := devserver.Start(devserver.Options{DSN: dbURL})
	if err != nil {
		return nil, func() {}, fmt.Errorf("ephemeral devserver: %w", err)
	}
	fmt.Fprintf(os.Stderr, "[test] devserver ready: %s (uploads=%s demos=%s)\n", h.URL, h.UploadsDir, h.DemosDir)
	env := []string{
		"DX_API_URL=" + h.URL,
		"DX_API_KEY=" + h.AdminToken,
		"UPLOADS_DIR=" + h.UploadsDir,
		"DEMOS_DIR=" + h.DemosDir,
		"TEST_DATABASE_URL=" + h.DSN,
	}
	return env, cleanup, nil
}

func testListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all tests across all adapters",
		RunE: func(cmd *cobra.Command, args []string) error {
			component, _ := cmd.Flags().GetString("component")
			layer, _ := cmd.Flags().GetString("layer")
			fromDB, _ := cmd.Flags().GetBool("from-db")

			if fromDB {
				return testListFromDB(cmd.Context(), component, layer)
			}

			// ── vitest ────────────────────────────────────────────────────
			if component == "" || component == "ui" {
				if layer == "" || layer == string(testharness.LayerUnit) {
					if uiDir := detectUIDir(); uiDir != "" {
						fmt.Printf("  %-8s %-12s %s\n", "ui", "unit", uiDir+" (vitest)")
					}
				}
			}

			// ── Go unit (source packages) ──────────────────────────────────
			if component == "" || component == "api" {
				if layer == "" || layer == string(testharness.LayerUnit) {
					a := &testharness.GoUnitAdapter{Pkgs: []string{"./internal/...", "./test/scripts/..."}, Comp: "api"}
					names, err := a.List(context.Background())
					if err != nil {
						fmt.Fprintf(os.Stderr, "  [api/unit] list failed: %v\n", err)
					} else {
						for _, n := range names {
							fmt.Printf("  %-8s %-12s %s\n", "api", "unit", n)
						}
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
								comp := "api"
								if strings.HasPrefix(n, "TestDemo") {
									l = "demo"
									comp = "demo"
								}
								if wantsDemo && !wantsIntegration && l != "demo" {
									continue
								}
								if wantsIntegration && !wantsDemo && l != "integration" {
									continue
								}
								fmt.Printf("  %-8s %-12s %s\n", comp, l, n)
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
	cmd.Flags().Bool("from-db", false, "list tests registered in the server DB (shows IDs)")
	return cmd
}

func testListFromDB(ctx context.Context, component, layer string) error {
	c := cli.MustClient()
	slug := c.SlugOrDie()
	limit := int32(200)
	var offset int32
	total := int64(-1)
	fmt.Printf("  %-6s %-8s %-12s %s\n", "id", "comp", "layer", "name")
	fmt.Printf("  %-6s %-8s %-12s %s\n", "------", "--------", "------------", "----")
	for total < 0 || int64(offset) < total {
		resp, err := c.ListTestsWithResponse(ctx, &dxclient.ListTestsParams{
			Slug:   slug,
			Limit:  &limit,
			Offset: &offset,
		})
		if err != nil {
			return err
		}
		if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
			return err
		}
		if resp.JSON200 == nil || resp.JSON200.Tests == nil {
			break
		}
		total = resp.JSON200.Total
		for _, t := range *resp.JSON200.Tests {
			if component != "" && t.Component != component {
				continue
			}
			if layer != "" && t.Layer != layer {
				continue
			}
			fmt.Printf("  %-6d %-8s %-12s %s\n", t.Id, t.Component, t.Layer, t.Name)
		}
		offset += int32(len(*resp.JSON200.Tests))
		if len(*resp.JSON200.Tests) == 0 {
			break
		}
	}
	return nil
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
			if err := cli.RunShell(s.Setup, s.CWD); err != nil {
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
		err := cli.RunShell(s.Run, s.CWD)
		dur := time.Since(start).Milliseconds()

		if s.Teardown != "" {
			fmt.Printf("  teardown: %s\n", s.Teardown)
			_ = cli.RunShell(s.Teardown, s.CWD)
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

var testBin = "bin/zdx-test"

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
			return cli.RunShell("go test -c -o "+testBin+" "+testPkg, "")
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
		if err := cli.RunShell("go test -c -o "+testBin+" "+testPkg, ""); err != nil {
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

// syncTestResults submits test results (with demo artifacts) to the server.
// For each test whose output references a recorded demo file, a second pass
// uploads the file contents via POST /api/dx/demos/upload so the server can
// serve it from UploadsDir — the reference-only submit path works in dev (where
// the recorder and server share a filesystem) but 404s in production because
// deploy packages don't carry .zdx/demo/.
func syncTestResults(results []testharness.Result, metas []testharness.DemoMeta) {
	c, err := cli.DefaultClient()
	if err != nil {
		return
	}
	slug := c.Slug()
	if slug == "" {
		return
	}
	if len(results) == 0 {
		return
	}

	type demoUpload struct {
		component    string
		testName     string
		demoType     string
		artifactPath string
	}
	var uploads []demoUpload

	apiResults := make([]dxclient.TestResultInput, 0, len(results))
	for _, r := range results {
		comp := r.Component
		if strings.HasPrefix(r.Test, "TestDemo") {
			comp = "demo"
		}
		item := dxclient.TestResultInput{
			Driver:     comp,
			TestName:   r.Test,
			Feature:    r.Feature,
			Status:     r.Status,
			DurationMs: int32(r.DurationMs),
		}
		if r.Branch != "" {
			branch := r.Branch
			item.Branch = &branch
		}
		if r.GitSHA != "" {
			sha := r.GitSHA
			item.GitSha = &sha
		}
		// Match demo artifacts by checking if the test output references the artifact file.
		var demos []dxclient.DemoArtifactRef
		for _, m := range metas {
			base := filepath.Base(m.ArtifactPath)
			if strings.Contains(r.Output, base) {
				demos = append(demos, dxclient.DemoArtifactRef{
					DemoType:     m.DemoType,
					ArtifactPath: m.ArtifactPath,
				})
				uploads = append(uploads, demoUpload{
					component:    comp,
					testName:     r.Test,
					demoType:     m.DemoType,
					artifactPath: m.ArtifactPath,
				})
			}
		}
		if len(demos) > 0 {
			item.DemoArtifacts = &demos
		}
		apiResults = append(apiResults, item)
	}

	resp, err := c.SubmitTestResultsWithResponse(context.Background(), dxclient.SubmitTestResultsRequest{
		Slug:    slug,
		Results: &apiResults,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "[sync] submit failed: %v\n", err)
		return
	}
	if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
		fmt.Fprintf(os.Stderr, "[sync] submit failed: %v\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "[sync] %d test result(s) submitted\n", len(apiResults))

	uploaded := 0
	for _, u := range uploads {
		if err := uploadDemoArtifact(c, slug, u.component, u.testName, u.demoType, u.artifactPath); err != nil {
			fmt.Fprintf(os.Stderr, "[sync] demo upload (%s) failed: %v\n", filepath.Base(u.artifactPath), err)
			continue
		}
		uploaded++
	}
	if uploaded > 0 {
		fmt.Fprintf(os.Stderr, "[sync] %d demo artifact(s) uploaded\n", uploaded)
	}

	seen := make(map[string]bool)
	total := 0
	total += attachCodeRefsFromSidecars(c, slug, results, filepath.Join(".zdx", "demo", "coderefs"), seen)
	total += attachCodeRefsFromSidecars(c, slug, results, filepath.Join(".zdx", "coderefs"), seen)
	if total > 0 {
		fmt.Fprintf(os.Stderr, "[sync] %d coderef(s) attached to tests\n", total)
	}
}

type codeRefSidecar struct {
	FilePath  string `json:"file_path"`
	LineStart int32  `json:"line_start,omitempty"`
	LineEnd   int32  `json:"line_end,omitempty"`
	Note      string `json:"note,omitempty"`
}

// readCodeRefSidecars reads every *.json file in sidecarDir and returns a map
// keyed by test name (filename without .json). Non-JSON entries and subdirs
// are skipped. Missing dir returns (nil, nil).
func readCodeRefSidecars(sidecarDir string) (map[string][]codeRefSidecar, error) {
	entries, err := os.ReadDir(sidecarDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	out := make(map[string][]codeRefSidecar)
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		testName := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
		data, err := os.ReadFile(filepath.Join(sidecarDir, e.Name()))
		if err != nil {
			continue
		}
		var refs []codeRefSidecar
		if err := json.Unmarshal(data, &refs); err != nil || len(refs) == 0 {
			continue
		}
		out[testName] = refs
	}
	return out, nil
}

// attachCodeRefsFromSidecars reads sidecar files in sidecarDir and calls
// AttachCodeRefToTest for each entry. Test rows exist at this point because
// SubmitTestResults upserted them. Test names already present in seen are
// skipped so the same test isn't re-attached from multiple sidecar dirs.
// Best-effort: per-file errors are logged but do not abort.
func attachCodeRefsFromSidecars(c *cli.Client, slug string, results []testharness.Result, sidecarDir string, seen map[string]bool) int {
	sidecars, err := readCodeRefSidecars(sidecarDir)
	if err != nil || len(sidecars) == 0 {
		return 0
	}

	shaByName := make(map[string]string, len(results))
	for _, r := range results {
		if r.GitSHA != "" {
			shaByName[r.Test] = r.GitSHA
		}
	}

	ctx := context.Background()
	attached := 0
	for testName, refs := range sidecars {
		if seen[testName] {
			continue
		}
		seen[testName] = true
		testID, err := cli.ResolveTestID(ctx, c, testName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[sync] coderefs: resolve %q: %v\n", testName, err)
			continue
		}
		hash := shaByName[testName]
		for _, ref := range refs {
			body := dxclient.AttachCodeRefToTestRequest{
				Slug:     slug,
				TestId:   testID,
				FilePath: ref.FilePath,
			}
			if hash != "" {
				body.GitHash = &hash
			}
			if ref.LineStart > 0 {
				ls := ref.LineStart
				body.LineStart = &ls
			}
			if ref.LineEnd > 0 {
				le := ref.LineEnd
				body.LineEnd = &le
			}
			if ref.Note != "" {
				note := ref.Note
				body.Note = &note
			}
			resp, err := c.AttachCodeRefToTestWithResponse(ctx, body)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[sync] coderefs attach (%s → %s): %v\n", testName, ref.FilePath, err)
				continue
			}
			if resp.StatusCode() >= 400 {
				fmt.Fprintf(os.Stderr, "[sync] coderefs attach %s → %s: status %d\n", testName, ref.FilePath, resp.StatusCode())
				continue
			}
			attached++
		}
	}
	return attached
}

// uploadDemoArtifact reads a demo file from disk and uploads it to the server,
// keyed on (test_component, test_name, artifact_path) so that UpsertTestDemo's
// ON CONFLICT clause updates the file_id of the row that SubmitTestResults
// just created (rather than inserting a duplicate).
// postKpiSamples aggregates results by (component, layer) and posts four KPI
// samples per suite (test_time_ms / pass / fail / skip) via the typed POST
// helper. Best-effort: errors are logged to stderr but never abort the run.
// Caller is responsible for client bootstrap (so dx test still works without a
// configured server — see testHarnessRunE).
func postKpiSamples(ctx context.Context, c *cli.Client, slug string, results []testharness.Result) {
	if c == nil || slug == "" || len(results) == 0 {
		return
	}

	type key struct{ component, layer string }
	type agg struct {
		durationMs       int64
		pass, fail, skip int
	}
	suites := make(map[key]*agg)
	var order []key

	for _, r := range results {
		comp := r.Component
		if strings.HasPrefix(r.Test, "TestDemo") {
			comp = "demo"
		}
		if comp == "" {
			comp = "unknown"
		}
		layer := string(r.Layer)
		if layer == "" {
			layer = "unknown"
		}
		k := key{component: comp, layer: layer}
		a, ok := suites[k]
		if !ok {
			a = &agg{}
			suites[k] = a
			order = append(order, k)
		}
		a.durationMs += r.DurationMs
		switch r.Status {
		case "pass":
			a.pass++
		case "fail":
			a.fail++
		case "skip":
			a.skip++
		}
	}

	for _, k := range order {
		a := suites[k]
		suffix := k.component + "." + k.layer
		samples := []kpiSampleParams{
			{Slug: slug, Scope: "tech", CheckName: "test_time_ms." + suffix, Value: float64(a.durationMs), Unit: "ms", RegressionThresholdMs: 15000},
			{Slug: slug, Scope: "tech", CheckName: "test_pass_count." + suffix, Value: float64(a.pass), Unit: "count"},
			{Slug: slug, Scope: "tech", CheckName: "test_fail_count." + suffix, Value: float64(a.fail), Unit: "count"},
			{Slug: slug, Scope: "tech", CheckName: "test_skip_count." + suffix, Value: float64(a.skip), Unit: "count"},
		}
		for _, p := range samples {
			if err := runKpiSample(ctx, c, p); err != nil {
				fmt.Fprintf(os.Stderr, "[kpi] post failed: %v\n", err)
			}
		}
	}
}

func uploadDemoArtifact(c *cli.Client, slug, component, testName, demoType, artifactPath string) error {
	data, err := os.ReadFile(artifactPath)
	if err != nil {
		return err
	}
	contentType := "application/octet-stream"
	switch strings.ToLower(filepath.Ext(artifactPath)) {
	case ".json":
		contentType = "application/json"
	case ".webm":
		contentType = "video/webm"
	case ".mp4":
		contentType = "video/mp4"
	}
	fields := map[string]string{
		"slug":           slug,
		"test_component": component,
		"test_name":      testName,
		"demo_type":      demoType,
		"artifact_path":  artifactPath,
	}
	return c.PostMultipart("/api/dx/demos/upload", "file", filepath.Base(artifactPath), contentType, data, fields, nil)
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
