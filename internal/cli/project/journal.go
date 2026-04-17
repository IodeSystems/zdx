package project

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/dxclient"
	"github.com/iodesystems/zdx-go/internal/techmetrics"
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

func JournalCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "standup", Aliases: []string{"journal"}, Short: "Owner/tech standup check-ins"}
	cmd.AddCommand(
		journalCheckinCmd(),
		journalShowCmd(),
		journalStateCmd(),
		journalAddCmd(),
		journalListCmd(),
		journalReviewCmd(),
	)
	return cmd
}

func journalReviewCmd() *cobra.Command {
	var role string
	cmd := &cobra.Command{
		Use:   "review",
		Short: "Acknowledge the latest unreviewed standup entry (clears the solo queue item)",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			resp, err := c.JournalReviewWithResponse(cmd.Context(), dxclient.JournalReviewRequest{
				Slug: c.SlugOrDie(),
				Role: role,
			})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			fmt.Printf("ok — %s check-in marked reviewed\n", role)
			return nil
		},
	}
	cmd.Flags().StringVar(&role, "role", "owner", "standup role (owner/tech)")
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

			body := dxclient.JournalCheckinRequest{
				Slug:       c.SlugOrDie(),
				Role:       role,
				Date:       date,
				Tldr:       tldr,
				Assessment: assessment,
				Concerns:   concerns,
				Next:       next,
			}

			if role == "tech" {
				root := projectRoot
				if root == "" {
					root, _ = os.Getwd()
				}

				metrics := collectTechMetrics(root)

				var prevDate string
				var prevStateJSON string
				showResp, _ := c.JournalShowWithResponse(cmd.Context(), &dxclient.JournalShowParams{
					Slug: c.SlugOrDie(),
					Role: "tech",
				})
				if showResp != nil && showResp.JSON200 != nil && showResp.JSON200.Entries != nil && len(*showResp.JSON200.Entries) > 0 {
					prev := (*showResp.JSON200.Entries)[0]
					prevDate = prev.Date
					prevStateJSON = prev.StateJson
				}

				commits, filesChanged := collectGitChurn(root, prevDate)
				metrics.GitCommits = commits
				metrics.GitFilesChanged = filesChanged

				stateJSON := metricsToJSON(metrics)
				body.StateJson = &stateJSON

				if prevMetrics, ok := parseTechMetrics(prevStateJSON); ok {
					deltas := computeDeltas(prevMetrics, metrics)
					changelog := deltasToJSON(deltas)
					body.ChangelogJson = &changelog
					fmt.Println("metrics (delta from last entry):")
					fmt.Print(formatMetricsSummary(metrics, deltas))
				} else {
					empty := "[]"
					body.ChangelogJson = &empty
					deltas := computeDeltas(TechMetrics{}, metrics)
					fmt.Println("metrics (baseline):")
					fmt.Print(formatMetricsSummary(metrics, deltas))
				}
			}

			resp, err := c.JournalCheckinWithResponse(cmd.Context(), body)
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
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
			resp, err := c.JournalShowWithResponse(cmd.Context(), &dxclient.JournalShowParams{
				Slug: c.SlugOrDie(),
				Role: role,
			})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil || resp.JSON200.Entries == nil || len(*resp.JSON200.Entries) == 0 {
				fmt.Println("no entries")
				return nil
			}
			for _, e := range *resp.JSON200.Entries {
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
				if e.StateJson != "" && e.StateJson != "{}" {
					if m, ok := parseTechMetrics(e.StateJson); ok {
						var deltas []MetricDelta
						if e.ChangelogJson != "" && e.ChangelogJson != "[]" {
							_ = json.Unmarshal([]byte(e.ChangelogJson), &deltas)
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
			resp, err := c.JournalStateWithResponse(cmd.Context(), &dxclient.JournalStateParams{
				Slug: c.SlugOrDie(),
				Role: role,
			})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil || resp.JSON200.StateJson == "" {
				fmt.Println("no state")
				return nil
			}
			fmt.Println(resp.JSON200.StateJson)
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
			body := dxclient.AppendIssueWorkRequest{
				IssueId: int32(id),
				Note:    note,
			}
			if role != "" {
				body.ByRole = &role
			}
			resp, err := c.AppendIssueWorkWithResponse(cmd.Context(), body)
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
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

func journalListCmd() *cobra.Command {
	var issue string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List work-log entries",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			resp, err := c.ListWorklogWithResponse(cmd.Context(), &dxclient.ListWorklogParams{
				Slug: c.SlugOrDie(),
			})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			var entries []dxclient.Item
			if resp.JSON200 != nil && resp.JSON200.Entries != nil {
				entries = *resp.JSON200.Entries
			}
			if issue != "" {
				var filtered []dxclient.Item
				for _, e := range entries {
					if e.IssueId == issue {
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
					title = e.IssueId
				}
				fmt.Printf("%s  %-8s  %-8s  %s — %s\n", date, e.IssueId, e.Agent, title, e.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&issue, "issue", "", "filter by issue ID (IS-N)")
	return cmd
}
