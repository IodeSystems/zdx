package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type journalEntry struct {
	Date          string `json:"date"`
	Baseline      bool   `json:"baseline"`
	Tldr          string `json:"tldr"`
	Assessment    string `json:"assessment"`
	Concerns      string `json:"concerns"`
	Next          string `json:"next"`
	ChangelogJSON string `json:"changelog_json"`
	StateJSON     string `json:"state_json"`
}

func JournalCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "standup", Aliases: []string{"journal"}, Short: "Owner/tech standup check-ins"}
	cmd.AddCommand(
		journalCheckinCmd(),
		journalShowCmd(),
		journalStateCmd(),
	)
	return cmd
}

func journalCheckinCmd() *cobra.Command {
	var role, tldr, assessment, concerns, next, date, projectRoot string
	cmd := &cobra.Command{
		Use:   "checkin",
		Short: "Record a standup check-in",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := mustClient()
			if date == "" {
				date = time.Now().Format("2006-01-02")
			}

			body := map[string]any{
				"slug":       c.SlugOrDie(),
				"role":       role,
				"date":       date,
				"tldr":       tldr,
				"assessment": assessment,
				"concerns":   concerns,
				"next":       next,
			}

			if role == "tech" {
				root := projectRoot
				if root == "" {
					root, _ = os.Getwd()
				}

				metrics := collectTechMetrics(root)

				var prevDate string
				var prevStateJSON string
				params := url.Values{
					"slug": {c.SlugOrDie()},
					"role": {"tech"},
				}
				var showResp struct {
					Entries []json.RawMessage `json:"entries"`
				}
				_ = c.get("/api/dx/journal/show", params, &showResp)
				if len(showResp.Entries) > 0 {
					var prev journalEntry
					if json.Unmarshal(showResp.Entries[0], &prev) == nil {
						prevDate = prev.Date
						prevStateJSON = prev.StateJSON
					}
				}

				commits, filesChanged := collectGitChurn(root, prevDate)
				metrics.GitCommits = commits
				metrics.GitFilesChanged = filesChanged

				stateJSON := metricsToJSON(metrics)
				body["state_json"] = stateJSON

				if prevMetrics, ok := parseTechMetrics(prevStateJSON); ok {
					deltas := computeDeltas(prevMetrics, metrics)
					body["changelog_json"] = deltasToJSON(deltas)
					fmt.Println("metrics (delta from last entry):")
					fmt.Print(formatMetricsSummary(metrics, deltas))
				} else {
					body["changelog_json"] = "[]"
					deltas := computeDeltas(TechMetrics{}, metrics)
					fmt.Println("metrics (baseline):")
					fmt.Print(formatMetricsSummary(metrics, deltas))
				}
			}

			var resp struct{ OK bool }
			if err := c.post("/api/dx/journal/checkin", body, &resp); err != nil {
				return err
			}
			fmt.Println("ok")
			return nil
		},
	}
	cmd.Flags().StringVar(&role, "role", "owner", "standup role (owner/tech)")
	cmd.Flags().StringVar(&tldr, "tldr", "", "one-line summary")
	cmd.Flags().StringVar(&assessment, "assessment", "", "assessment text")
	cmd.Flags().StringVar(&concerns, "concerns", "", "concerns text")
	cmd.Flags().StringVar(&next, "next", "", "next steps text")
	cmd.Flags().StringVar(&date, "date", "", "entry date (default: today)")
	cmd.Flags().StringVar(&projectRoot, "project-root", "", "project root directory for tech metrics (default: cwd)")
	return cmd
}

func journalShowCmd() *cobra.Command {
	var role string
	cmd := &cobra.Command{
		Use:   "show",
		Short: "List standup entries",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := mustClient()
			params := url.Values{
				"slug": {c.SlugOrDie()},
				"role": {role},
			}
			var resp struct {
				Entries []journalEntry `json:"entries"`
			}
			if err := c.get("/api/dx/journal/show", params, &resp); err != nil {
				return err
			}
			if len(resp.Entries) == 0 {
				fmt.Println("no entries")
				return nil
			}
			for _, e := range resp.Entries {
				tag := ""
				if e.Baseline {
					tag = " [baseline]"
				}
				fmt.Printf("%s%s  %s\n", e.Date, tag, e.Tldr)
				if e.Assessment != "" {
					fmt.Printf("  assessment: %s\n", e.Assessment)
				}
				if e.Concerns != "" {
					fmt.Printf("  concerns:   %s\n", e.Concerns)
				}
				if e.Next != "" {
					fmt.Printf("  next:       %s\n", e.Next)
				}
				if e.StateJSON != "" && e.StateJSON != "{}" {
					if m, ok := parseTechMetrics(e.StateJSON); ok {
						var deltas []MetricDelta
						if e.ChangelogJSON != "" && e.ChangelogJSON != "[]" {
							_ = json.Unmarshal([]byte(e.ChangelogJSON), &deltas)
						}
						if len(deltas) == 0 {
							deltas = computeDeltas(TechMetrics{}, m)
						}
						fmt.Println("  metrics:")
						for _, line := range strings.Split(formatMetricsSummary(m, deltas), "\n") {
							if line != "" {
								fmt.Printf("  %s\n", line)
							}
						}
					}
				}
				fmt.Println()
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&role, "role", "owner", "standup role (owner/tech)")
	return cmd
}

func journalStateCmd() *cobra.Command {
	var role string
	cmd := &cobra.Command{
		Use:   "state",
		Short: "Show latest standup state snapshot",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := mustClient()
			params := url.Values{
				"slug": {c.SlugOrDie()},
				"role": {role},
			}
			var resp struct {
				StateJSON string `json:"state_json"`
			}
			if err := c.get("/api/dx/journal/state", params, &resp); err != nil {
				return err
			}
			if resp.StateJSON == "" {
				fmt.Println("no state")
				return nil
			}
			fmt.Println(resp.StateJSON)
			return nil
		},
	}
	cmd.Flags().StringVar(&role, "role", "owner", "standup role (owner/tech)")
	return cmd
}
