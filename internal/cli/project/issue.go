package project

import (
	"fmt"
	"net/url"
	"os/exec"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"

	"github.com/iodesystems/zdx-go/internal/cli/clitypes"
)

func IssueCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "issue", Short: "Issue management"}
	cmd.AddCommand(issueListCmd(), issueAddCmd(), issueShowCmd(), issueCloseCmd(), issueEditCmd(), issueBlockCmd(), issueUnblockCmd(), issueReadyCmd(), issueResolveCmd(), issueResolutionsCmd(), issueReconcileCmd())
	return cmd
}

func issueListCmd() *cobra.Command {
	var status, branch string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List issues",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			var resp struct {
				Issues []clitypes.IssueItem `json:"issues"`
			}
			if err := c.Get("/api/dx/todo/issue/list", url.Values{"slug": {c.SlugOrDie()}}, &resp); err != nil {
				return err
			}
			if len(resp.Issues) == 0 {
				fmt.Println("no issues")
				return nil
			}
			resolutionCounts := map[int32]int{}
			if branch != "" {
				for _, iss := range resp.Issues {
					var resResp struct {
						Resolutions []struct {
							ID       string `json:"id"`
							Reverted bool   `json:"reverted"`
						} `json:"resolutions"`
					}
					_ = c.Get("/api/dx/todo/issue/resolutions", url.Values{
						"slug":   {c.SlugOrDie()},
						"id":     {clitypes.IssueIDStr(iss.ID)},
						"branch": {branch},
					}, &resResp)
					active := 0
					for _, r := range resResp.Resolutions {
						if !r.Reverted {
							active++
						}
					}
					resolutionCounts[iss.ID] = active
				}
			}
			for _, iss := range resp.Issues {
				if status != "" && iss.Status != status {
					continue
				}
				s := iss.Status
				if len(iss.BlockedBy) > 0 {
					s += " [blocked:" + strings.Join(iss.BlockedBy, ",") + "]"
				}
				if branch != "" {
					if resolutionCounts[iss.ID] > 0 {
						s += " [resolved]"
					} else {
						s += " [unresolved]"
					}
				}
				fmt.Printf("%-8s %-30s %s\n", clitypes.IssueIDStr(iss.ID), s, iss.Title)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&status, "status", "", "filter by status (open, closed)")
	cmd.Flags().StringVar(&branch, "branch", "", "show resolution state on this branch")
	return cmd
}

func issueAddCmd() *cobra.Command {
	var title, ctx, component, blockedBy, issueType, issueURL, parent string
	var autoReady bool
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Create an issue",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			body := map[string]any{
				"slug":       c.SlugOrDie(),
				"title":      title,
				"context":    ctx,
				"component":  component,
				"issue_type": issueType,
				"auto_ready": autoReady,
			}
			if issueURL != "" {
				body["url"] = issueURL
			}
			if blockedBy != "" {
				body["blocked_by"] = strings.Split(blockedBy, ",")
			}
			var resp clitypes.IssueAddResponse
			if err := c.Post("/api/dx/todo/issue/add", body, &resp); err != nil {
				return err
			}
			fmt.Printf("%s  %s\n", clitypes.IssueIDStr(resp.ID), resp.Title)
			if parent != "" {
				parentNum, _ := strconv.ParseInt(parent[3:], 10, 32)
				if err := c.Post("/api/dx/todo/issue/add-block", map[string]any{
					"slug":       c.SlugOrDie(),
					"id":         int32(parentNum),
					"blocked_by": clitypes.IssueIDStr(resp.ID),
				}, nil); err != nil {
					return fmt.Errorf("created issue but failed to add block on parent: %w", err)
				}
				fmt.Printf("  → blocks %s\n", parent)
			}
			if !autoReady && len(resp.Similar) > 0 {
				fmt.Println("\nSimilar issues:")
				for _, s := range resp.Similar {
					fmt.Printf("  %s  (%.0f%%)  %s  [%s]\n", s.ID, s.Score*100, s.Title, s.Status)
				}
				fmt.Printf("\nIssue created as draft (wip). To promote:\n")
				fmt.Printf("  dx issue ready %s\n", clitypes.IssueIDStr(resp.ID))
				fmt.Printf("To close as duplicate:\n")
				fmt.Printf("  dx issue close %s --reason=duplicate --duplicate-of=<IS-N>\n", clitypes.IssueIDStr(resp.ID))
			} else if !autoReady {
				if err := c.Post("/api/dx/todo/issue/ready", map[string]any{
					"slug": c.SlugOrDie(),
					"id":   resp.ID,
				}, nil); err != nil {
					return err
				}
				fmt.Println("(auto-promoted to open — no similar issues found)")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "issue title")
	cmd.Flags().StringVar(&ctx, "context", "", "context / description")
	cmd.Flags().StringVar(&component, "component", "", "component")
	cmd.Flags().StringVar(&blockedBy, "blocked-by", "", "blocking issues (IS-N, comma-separated)")
	cmd.Flags().StringVar(&issueType, "type", "ops", "issue type: ops, impl, ask, or tracker")
	cmd.Flags().StringVar(&issueURL, "url", "", "URL related to the issue")
	cmd.Flags().StringVar(&parent, "parent", "", "parent issue (IS-N): new issue blocks the parent")
	cmd.Flags().BoolVar(&autoReady, "auto-ready", false, "skip similarity check and create as open")
	cmd.MarkFlagRequired("title")
	return cmd
}

func issueReadyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ready <IS-N>",
		Short: "Promote a draft issue from wip to open",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			n, _ := strconv.ParseInt(args[0][3:], 10, 32)
			if err := c.Post("/api/dx/todo/issue/ready", map[string]any{
				"slug": c.SlugOrDie(),
				"id":   int32(n),
			}, nil); err != nil {
				return err
			}
			fmt.Printf("%s promoted to open\n", args[0])
			return nil
		},
	}
}

func issueShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <IS-N>",
		Short: "Show issue detail with work log",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			var resp struct {
				Issue clitypes.IssueItem       `json:"issue"`
				Work  []clitypes.IssueWorkItem `json:"work"`
			}
			if err := c.Get("/api/dx/todo/issue/show", url.Values{
				"slug": {c.SlugOrDie()},
				"id":   {args[0]},
			}, &resp); err != nil {
				return err
			}
			cli.PrintIssueItem(resp.Issue)

			var resResp struct {
				Resolutions []struct {
					ID             string `json:"id"`
					BranchOfOrigin string `json:"branch_of_origin"`
					ResolvedAt     string `json:"resolved_at"`
					Author         string `json:"author"`
					Source         string `json:"source"`
					Reverted       bool   `json:"reverted"`
					RevertedBy     string `json:"reverted_by"`
					Commits        []struct {
						SHA string `json:"sha"`
					} `json:"commits"`
				} `json:"resolutions"`
			}
			if err := c.Get("/api/dx/todo/issue/resolutions", url.Values{
				"slug": {c.SlugOrDie()},
				"id":   {args[0]},
			}, &resResp); err == nil && len(resResp.Resolutions) > 0 {
				fmt.Println("\nResolutions:")
				for _, r := range resResp.Resolutions {
					status := r.Source
					if r.Reverted {
						revertShort := r.RevertedBy
						if len(revertShort) > 10 {
							revertShort = revertShort[:10]
						}
						status = "REVERTED (by " + revertShort + ")"
					}
					date := r.ResolvedAt
					if len(date) >= 10 {
						date = date[:10]
					}
					idShort := r.ID
					if len(idShort) > 8 {
						idShort = idShort[:8]
					}
					fmt.Printf("  %s  %s  %s  [%s]  %s\n", idShort, date, r.BranchOfOrigin, status, r.Author)
					for _, commit := range r.Commits {
						sha := commit.SHA
						if len(sha) > 10 {
							sha = sha[:10]
						}
						fmt.Printf("    %s\n", sha)
					}
				}
			}

			if len(resp.Work) > 0 {
				fmt.Println("\nWork log:")
				for _, w := range resp.Work {
					date := w.CreatedAt
					if len(date) >= 10 {
						date = date[:10]
					}
					fmt.Printf("  [%s] %s: %s\n", date, w.Agent, w.Note)
				}
			}
			return nil
		},
	}
}

func issueCloseCmd() *cobra.Command {
	var reason string
	var duplicateOf string
	cmd := &cobra.Command{
		Use:   "close <IS-N>",
		Short: "Close an issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if reason == "duplicate" && duplicateOf == "" {
				return fmt.Errorf("--duplicate-of is required when --reason=duplicate")
			}
			if err := cli.RunCloseHooks(); err != nil {
				return err
			}
			id := args[0]
			n, _ := strconv.ParseInt(id[3:], 10, 32)
			c := cli.MustClient()
			var ok struct {
				OK bool `json:"ok"`
			}
			body := map[string]any{
				"slug":   c.SlugOrDie(),
				"id":     int32(n),
				"reason": reason,
			}
			if duplicateOf != "" {
				body["duplicate_of"] = duplicateOf
			}
			if err := c.Post("/api/dx/todo/issue/close", body, &ok); err != nil {
				return err
			}
			fmt.Printf("%s closed\n", id)
			return nil
		},
	}
	cmd.Flags().StringVar(&reason, "reason", "", "close reason")
	cmd.Flags().StringVar(&duplicateOf, "duplicate-of", "", "issue ID this duplicates (required when --reason=duplicate)")
	return cmd
}

func issueBlockCmd() *cobra.Command {
	var by string
	cmd := &cobra.Command{
		Use:   "block <IS-N>",
		Short: "Add a blocker to an issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			n, _ := strconv.ParseInt(id[3:], 10, 32)
			c := cli.MustClient()
			var ok struct {
				OK bool `json:"ok"`
			}
			if err := c.Post("/api/dx/todo/issue/add-block", map[string]any{
				"slug":       c.SlugOrDie(),
				"id":         int32(n),
				"blocked_by": by,
			}, &ok); err != nil {
				return err
			}
			fmt.Printf("%s blocked by %s\n", id, by)
			return nil
		},
	}
	cmd.Flags().StringVar(&by, "by", "", "blocking issue (IS-N)")
	cmd.MarkFlagRequired("by")
	return cmd
}

func issueUnblockCmd() *cobra.Command {
	var by string
	cmd := &cobra.Command{
		Use:   "unblock <IS-N>",
		Short: "Remove blocker(s) from an issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			n, _ := strconv.ParseInt(id[3:], 10, 32)
			c := cli.MustClient()
			var ok struct {
				OK bool `json:"ok"`
			}
			body := map[string]any{
				"slug": c.SlugOrDie(),
				"id":   int32(n),
			}
			if by != "" {
				body["blocked_by"] = by
			}
			if err := c.Post("/api/dx/todo/issue/remove-block", body, &ok); err != nil {
				return err
			}
			if by != "" {
				fmt.Printf("%s unblocked from %s\n", id, by)
			} else {
				fmt.Printf("%s all blockers removed\n", id)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&by, "by", "", "specific blocker to remove (IS-N); omit to clear all")
	return cmd
}

func issueEditCmd() *cobra.Command {
	var title, ctx, component, issueType string
	var priority int
	cmd := &cobra.Command{
		Use:   "edit <IS-N>",
		Short: "Edit fields on an existing issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			n, _ := strconv.ParseInt(id[3:], 10, 32)
			c := cli.MustClient()
			body := map[string]any{
				"slug": c.SlugOrDie(),
				"id":   int32(n),
			}
			if cmd.Flags().Changed("title") {
				body["title"] = title
			}
			if cmd.Flags().Changed("context") {
				body["context"] = ctx
			}
			if cmd.Flags().Changed("priority") {
				body["priority"] = int32(priority)
			}
			if cmd.Flags().Changed("component") {
				body["component"] = component
			}
			if cmd.Flags().Changed("type") {
				body["issue_type"] = issueType
			}
			var ok struct {
				OK bool `json:"ok"`
			}
			if err := c.Post("/api/dx/todo/issue/edit", body, &ok); err != nil {
				return err
			}
			fmt.Printf("%s updated\n", id)
			return nil
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "issue title")
	cmd.Flags().StringVar(&ctx, "context", "", "context / description")
	cmd.Flags().IntVar(&priority, "priority", 0, "priority (1-4)")
	cmd.Flags().StringVar(&component, "component", "", "component")
	cmd.Flags().StringVar(&issueType, "type", "", "issue type: ops, impl, or tracker")
	return cmd
}

func issueResolveCmd() *cobra.Command {
	var branch string
	cmd := &cobra.Command{
		Use:   "resolve <IS-N> <sha> [sha...]",
		Short: "Record a commit-group resolution for an issue",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			n, _ := strconv.ParseInt(id[3:], 10, 32)
			shas := args[1:]
			for _, sha := range shas {
				out, err := exec.Command("git", "cat-file", "-t", sha).Output()
				if err != nil {
					return fmt.Errorf("SHA %s not found in local git repo", sha)
				}
				if strings.TrimSpace(string(out)) != "commit" {
					return fmt.Errorf("SHA %s is not a commit", sha)
				}
			}
			if branch == "" {
				out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
				if err == nil {
					branch = strings.TrimSpace(string(out))
				}
			}
			fullSHAs := make([]string, len(shas))
			for i, sha := range shas {
				out, err := exec.Command("git", "rev-parse", sha).Output()
				if err != nil {
					fullSHAs[i] = sha
				} else {
					fullSHAs[i] = strings.TrimSpace(string(out))
				}
			}
			c := cli.MustClient()
			body := map[string]any{
				"slug": c.SlugOrDie(),
				"id":   int32(n),
				"shas": fullSHAs,
			}
			if branch != "" {
				body["branch"] = branch
			}
			var resp struct {
				Resolution struct {
					ID             string `json:"id"`
					BranchOfOrigin string `json:"branch_of_origin"`
					Commits        []struct {
						SHA string `json:"sha"`
					} `json:"commits"`
				} `json:"resolution"`
			}
			if err := c.Post("/api/dx/todo/issue/resolve", body, &resp); err != nil {
				return err
			}
			fmt.Printf("%s resolved (resolution %s)\n", id, resp.Resolution.ID)
			for _, commit := range resp.Resolution.Commits {
				sha := commit.SHA
				if len(sha) > 10 {
					sha = sha[:10]
				}
				fmt.Printf("  %s\n", sha)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&branch, "branch", "", "branch of origin (default: auto-detect)")
	return cmd
}

func issueResolutionsCmd() *cobra.Command {
	var branch string
	cmd := &cobra.Command{
		Use:   "resolutions <IS-N>",
		Short: "List resolutions for an issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			params := url.Values{
				"slug": {c.SlugOrDie()},
				"id":   {args[0]},
			}
			if branch != "" {
				params.Set("branch", branch)
			}
			var resp struct {
				Resolutions []struct {
					ID             string `json:"id"`
					BranchOfOrigin string `json:"branch_of_origin"`
					ResolvedAt     string `json:"resolved_at"`
					Author         string `json:"author"`
					Source         string `json:"source"`
					Reverted       bool   `json:"reverted"`
					RevertedBy     string `json:"reverted_by"`
					Commits        []struct {
						SHA string `json:"sha"`
						Ord int    `json:"ord"`
					} `json:"commits"`
				} `json:"resolutions"`
			}
			if err := c.Get("/api/dx/todo/issue/resolutions", params, &resp); err != nil {
				return err
			}
			if len(resp.Resolutions) == 0 {
				fmt.Println("no resolutions")
				return nil
			}
			for _, r := range resp.Resolutions {
				status := r.Source
				if r.Reverted {
					status = "REVERTED (by " + r.RevertedBy[:10] + ")"
				}
				date := r.ResolvedAt
				if len(date) >= 10 {
					date = date[:10]
				}
				fmt.Printf("  %s  %s  %s  [%s]  %s\n", r.ID[:8], date, r.BranchOfOrigin, status, r.Author)
				for _, c := range r.Commits {
					sha := c.SHA
					if len(sha) > 10 {
						sha = sha[:10]
					}
					fmt.Printf("    %s\n", sha)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&branch, "branch", "", "check resolution validity on this branch")
	return cmd
}

func issueReconcileCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reconcile <branch>",
		Short: "Find equivalent patches on target branch and record reconciled resolutions",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			branch := args[0]
			c := cli.MustClient()
			var resp struct {
				Results []struct {
					IssueID      string   `json:"issue_id"`
					ResolutionID string   `json:"resolution_id"`
					MatchedSHAs  []string `json:"matched_shas"`
				} `json:"results"`
			}
			if err := c.Post("/api/dx/todo/issue/reconcile", map[string]any{
				"slug":   c.SlugOrDie(),
				"branch": branch,
			}, &resp); err != nil {
				return err
			}
			if len(resp.Results) == 0 {
				fmt.Println("no new reconciled resolutions found")
				return nil
			}
			for _, r := range resp.Results {
				fmt.Printf("  %s → %s", r.IssueID, r.ResolutionID[:8])
				for _, sha := range r.MatchedSHAs {
					if len(sha) > 10 {
						sha = sha[:10]
					}
					fmt.Printf(" %s", sha)
				}
				fmt.Println()
			}
			return nil
		},
	}
}

// RunIssue kept for compatibility.
func RunIssue(_ []string) {}
