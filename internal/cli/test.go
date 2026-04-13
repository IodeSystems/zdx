package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/config"
)

// TestResult is written to .zdx/test-results.json after a run.
type TestResult struct {
	Component string  `json:"component"`
	Suite     string  `json:"suite"`
	Runner    string  `json:"runner"`
	Status    string  `json:"status"` // pass | fail | skip
	DurationMs int64  `json:"duration_ms"`
	RunAt     string  `json:"run_at"`
	Output    string  `json:"output,omitempty"`
}

func TestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test [component]",
		Short: "Run test suites",
		RunE:  testRunE,
	}
	cmd.Flags().String("filter", "", "only run suites whose name contains this string")
	cmd.Flags().String("shard", "", "run shard N/M (e.g. 1/3)")
	cmd.Flags().Bool("list", false, "list suites without running")
	cmd.AddCommand(testListCmd(), testRunCmd())
	return cmd
}

func testListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list [component]",
		Short: "List configured test suites",
		RunE: func(cmd *cobra.Command, args []string) error {
			component := config.ActiveComponent("")
			if len(args) > 0 {
				component = args[0]
			}
			cfg := config.Load()
			if cfg == nil {
				return fmt.Errorf("no .zdx/config.yaml found")
			}
			suites := sortedSuites(cfg.AllTestSuites(component))
			for _, s := range suites {
				fmt.Printf("  %-30s %-10s %s\n", s.Component+":"+s.Name, s.Runner, s.Run)
			}
			return nil
		},
	}
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
			RunAt: time.Now().Format(time.RFC3339),
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
