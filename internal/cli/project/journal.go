package project

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/techmetrics"

	"github.com/iodesystems/zdx-go/internal/cli"
)

type (
	TechMetrics = techmetrics.TechMetrics
	MetricDelta = techmetrics.MetricDelta
)

var (
	collectTechMetrics   = techmetrics.Collect
	collectGitChurn      = techmetrics.CollectGitChurn
	computeDeltas        = techmetrics.ComputeDeltas
	metricsToJSON        = techmetrics.ToJSON
	deltasToJSON         = techmetrics.DeltasToJSON
	parseTechMetrics     = techmetrics.Parse
	formatMetricsSummary = techmetrics.FormatSummary
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
		journalAddCmd(),
		journalListCmd(),
	)
	return cmd
}

func journalCheckinCmd() *cobra.Command {
	var role, tldr, assessment, concerns, next, date, projectRoot string
	cmd := &cobra.Command{
		Use:   "checkin",
		Short: "Record a standup check-in",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
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
				_ = c.Get("/api/dx/journal/show", params, &showResp)
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
			if err := c.Post("/api/dx/journal/checkin", body, &resp); err != nil {
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
			c := cli.MustClient()
			params := url.Values{
				"slug": {c.SlugOrDie()},
				"role": {role},
			}
			var resp struct {
				Entries []journalEntry `json:"entries"`
			}
			if err := c.Get("/api/dx/journal/show", params, &resp); err != nil {
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
			c := cli.MustClient()
			params := url.Values{
				"slug": {c.SlugOrDie()},
				"role": {role},
			}
			var resp struct {
				StateJSON string `json:"state_json"`
			}
			if err := c.Get("/api/dx/journal/state", params, &resp); err != nil {
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

func journalAddCmd() *cobra.Command {
	var issue, note, role string
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Append a work-log entry to an issue",
		RunE: func(cmd *cobra.Command, args []string) error {
			if issue == "" {
				return fmt.Errorf("--issue is required")
			}
			if note == "" {
				return fmt.Errorf("--note is required")
			}
			id, err := strconv.Atoi(strings.TrimPrefix(issue, "IS-"))
			if err != nil {
				return fmt.Errorf("invalid issue ID %q (expected IS-N)", issue)
			}
			c := cli.MustClient()
			body := map[string]any{
				"issue_id": int32(id),
				"by_role":  role,
				"note":     note,
			}
			var resp struct{ OK bool }
			if err := c.Post("/api/issue-work", body, &resp); err != nil {
				return err
			}
			fmt.Printf("ok — work-log entry added to %s\n", issue)
			return nil
		},
	}
	cmd.Flags().StringVar(&issue, "issue", "", "issue ID (IS-N)")
	cmd.Flags().StringVar(&note, "note", "", "work-log note")
	cmd.Flags().StringVar(&role, "role", "llm", "attribution role")
	return cmd
}

type worklogEntry struct {
	IssueID    string `json:"issue_id"`
	IssueTitle string `json:"issue_title"`
	Agent      string `json:"agent"`
	Note       string `json:"note"`
	CreatedAt  string `json:"created_at"`
}

func journalListCmd() *cobra.Command {
	var issue string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List work-log entries",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			params := url.Values{
				"slug": {c.SlugOrDie()},
			}
			var resp struct {
				Entries []worklogEntry `json:"entries"`
				Total   int64          `json:"total"`
			}
			if err := c.Get("/api/dx/worklog", params, &resp); err != nil {
				return err
			}
			entries := resp.Entries
			if issue != "" {
				var filtered []worklogEntry
				for _, e := range entries {
					if e.IssueID == issue {
						filtered = append(filtered, e)
					}
				}
				entries = filtered
			}
			if len(entries) == 0 {
				fmt.Println("no work-log entries")
				return nil
			}
			for _, e := range entries {
				date := e.CreatedAt
				if len(date) >= 16 {
					date = date[:16]
				}
				title := e.IssueTitle
				if title == "" {
					title = e.IssueID
				}
				fmt.Printf("%s  %-8s  %-8s  %s — %s\n", date, e.IssueID, e.Agent, title, e.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&issue, "issue", "", "filter by issue ID (IS-N)")
	return cmd
}
