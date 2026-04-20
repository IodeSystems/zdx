package project

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/cli/clitypes"
	"github.com/iodesystems/zdx-go/internal/dxclient"
	"github.com/iodesystems/zdx-go/internal/workflowhints"
)

func IssueCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "issue", Short: "Issue management"}
	cmd.AddCommand(issueListCmd(), issueAddCmd(), issueShowCmd(), issueCloseCmd(), issueReopenCmd(), issueEditCmd(), issueBlockCmd(), issueUnblockCmd(), issueReadyCmd(), issueResolveCmd(), issueResolutionsCmd(), issueReconcileCmd(), issueClaimCmd(), issueReleaseCmd())
	return cmd
}

func issueListCmd() *cobra.Command {
	var status, branch string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List issues",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			slug := c.SlugOrDie()
			resp, err := c.ListIssuesWithResponse(cmd.Context(), &dxclient.ListIssuesParams{Slug: slug})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil || resp.JSON200.Issues == nil || len(*resp.JSON200.Issues) == 0 {
				fmt.Println("no issues")
				return nil
			}
			issues := *resp.JSON200.Issues
			resolutionCounts := map[int32]int{}
			if branch != "" {
				for _, iss := range issues {
					resResp, err := c.ListIssueResolutionsWithResponse(cmd.Context(), &dxclient.ListIssueResolutionsParams{
						Slug:   slug,
						Id:     clitypes.IssueIDStr(iss.Id),
						Branch: &branch,
					})
					if err != nil || resResp.JSON200 == nil || resResp.JSON200.Resolutions == nil {
						continue
					}
					active := 0
					for _, r := range *resResp.JSON200.Resolutions {
						if !r.Reverted {
							active++
						}
					}
					resolutionCounts[iss.Id] = active
				}
			}
			for _, iss := range issues {
				if status != "" && iss.Status != status {
					continue
				}
				s := iss.Status
				if iss.BlockedBy != nil && len(*iss.BlockedBy) > 0 {
					s += " [blocked:" + strings.Join(*iss.BlockedBy, ",") + "]"
				}
				if branch != "" {
					if resolutionCounts[iss.Id] > 0 {
						s += " [resolved]"
					} else {
						s += " [unresolved]"
					}
				}
				fmt.Printf("%-8s %-30s %s\n", clitypes.IssueIDStr(iss.Id), s, iss.Title)
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
			body := dxclient.AddIssueRequest{
				Slug:      c.SlugOrDie(),
				AutoReady: &autoReady,
			}
			if title != "" {
				body.Title = &title
			}
			if ctx != "" {
				body.Context = &ctx
			}
			if component != "" {
				body.Component = &component
			}
			if issueType != "" {
				body.IssueType = &issueType
			}
			if issueURL != "" {
				body.Url = &issueURL
			}
			if blockedBy != "" {
				blockers := strings.Split(blockedBy, ",")
				body.BlockedBy = &blockers
			}
			resp, err := c.AddIssueWithResponse(cmd.Context(), body)
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil {
				return fmt.Errorf("empty response")
			}
			addResp := resp.JSON200
			fmt.Printf("%s  %s\n", clitypes.IssueIDStr(addResp.Id), addResp.Title)
			if parent != "" {
				parentNum, _ := strconv.ParseInt(parent[3:], 10, 32)
				blkResp, err := c.IssueAddBlockWithResponse(cmd.Context(), dxclient.IssueAddBlockRequest{
					Slug:      c.SlugOrDie(),
					Id:        int32(parentNum),
					BlockedBy: clitypes.IssueIDStr(addResp.Id),
				})
				if err != nil {
					return fmt.Errorf("created issue but failed to add block on parent: %w", err)
				}
				if err := c.CheckStatus(blkResp.StatusCode(), blkResp.Body); err != nil {
					return fmt.Errorf("created issue but failed to add block on parent: %w", err)
				}
				fmt.Printf("  → blocks %s\n", parent)
			}
			hasSimilar := addResp.Similar != nil && len(*addResp.Similar) > 0
			if !autoReady && hasSimilar {
				fmt.Println("\nSimilar issues:")
				for _, s := range *addResp.Similar {
					fmt.Printf("  %s  (%.0f%%)  %s  [%s]\n", s.Id, s.Score*100, s.Title, s.Status)
				}
				fmt.Print(workflowhints.SimilarIssuesCLIMessage(clitypes.IssueIDStr(addResp.Id)))
			} else if !autoReady {
				rdyResp, err := c.ReadyIssueWithResponse(cmd.Context(), dxclient.ReadyIssueRequest{
					Slug: c.SlugOrDie(),
					Id:   addResp.Id,
				})
				if err != nil {
					return err
				}
				if err := c.CheckStatus(rdyResp.StatusCode(), rdyResp.Body); err != nil {
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
	cmd.Flags().StringVar(&issueType, "type", "", "issue type: ops, impl, ask, or tracker (default: unknown)")
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
			resp, err := c.ReadyIssueWithResponse(cmd.Context(), dxclient.ReadyIssueRequest{
				Slug: c.SlugOrDie(),
				Id:   int32(n),
			})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
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
			slug := c.SlugOrDie()
			showResp, err := c.ShowIssueWithResponse(cmd.Context(), &dxclient.ShowIssueParams{
				Slug: slug,
				Id:   args[0],
			})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(showResp.StatusCode(), showResp.Body); err != nil {
				return err
			}
			if showResp.JSON200 == nil {
				return fmt.Errorf("empty response")
			}
			cli.PrintIssueItem(cli.IssueToCli(showResp.JSON200.Issue))

			resResp, err := c.ListIssueResolutionsWithResponse(cmd.Context(), &dxclient.ListIssueResolutionsParams{
				Slug: slug,
				Id:   args[0],
			})
			if err == nil && resResp.JSON200 != nil && resResp.JSON200.Resolutions != nil && len(*resResp.JSON200.Resolutions) > 0 {
				fmt.Println("\nResolutions:")
				for _, r := range *resResp.JSON200.Resolutions {
					status := r.Source
					if r.Reverted {
						revertShort := ""
						if r.RevertedBy != nil {
							revertShort = *r.RevertedBy
						}
						if len(revertShort) > 10 {
							revertShort = revertShort[:10]
						}
						status = "REVERTED (by " + revertShort + ")"
					}
					date := r.ResolvedAt
					if len(date) >= 10 {
						date = date[:10]
					}
					idShort := r.Id
					if len(idShort) > 8 {
						idShort = idShort[:8]
					}
					fmt.Printf("  %s  %s  %s  [%s]  %s\n", idShort, date, r.BranchOfOrigin, status, r.Author)
					if r.Commits != nil {
						for _, commit := range *r.Commits {
							sha := commit.Sha
							if len(sha) > 10 {
								sha = sha[:10]
							}
							fmt.Printf("    %s\n", sha)
						}
					}
				}
			}

			if showResp.JSON200.Work != nil && len(*showResp.JSON200.Work) > 0 {
				fmt.Println("\nWork log:")
				for _, w := range *showResp.JSON200.Work {
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
	var linkOf string
	cmd := &cobra.Command{
		Use:   "close <IS-N>",
		Short: "Close an issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if reason == "duplicate" && duplicateOf == "" {
				return fmt.Errorf("--duplicate-of is required when --reason=duplicate")
			}
			if reason == "link" && linkOf == "" {
				return fmt.Errorf("--link-of is required when --reason=link")
			}
			if duplicateOf != "" && linkOf != "" {
				return fmt.Errorf("--duplicate-of and --link-of are mutually exclusive")
			}
			if err := cli.RunCloseHooks(); err != nil {
				return err
			}
			id := args[0]
			n, _ := strconv.ParseInt(id[3:], 10, 32)
			c := cli.MustClient()
			body := dxclient.CloseIssueRequest{
				Slug: c.SlugOrDie(),
				Id:   int32(n),
			}
			if reason != "" {
				body.Reason = &reason
			}
			if duplicateOf != "" {
				body.DuplicateOf = &duplicateOf
			}
			if linkOf != "" {
				body.LinkOf = &linkOf
			}
			resp, err := c.CloseIssueWithResponse(cmd.Context(), body)
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			fmt.Printf("%s closed\n", id)
			return nil
		},
	}
	cmd.Flags().StringVar(&reason, "reason", "", "close reason (done|duplicate|link|...)")
	cmd.Flags().StringVar(&duplicateOf, "duplicate-of", "", "issue ID this duplicates (required when --reason=duplicate)")
	cmd.Flags().StringVar(&linkOf, "link-of", "", "issue ID this is a narrow-slice link of (required when --reason=link; cascade-closes with target, no reopen-cascade)")
	return cmd
}

func issueReopenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reopen <IS-N>",
		Short: "Reopen a closed issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			n, _ := strconv.ParseInt(id[3:], 10, 32)
			c := cli.MustClient()
			resp, err := c.ReopenIssueWithResponse(cmd.Context(), dxclient.IssueIntIDInput{
				Id: int32(n),
			})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			fmt.Printf("%s reopened\n", id)
			return nil
		},
	}
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
			resp, err := c.IssueAddBlockWithResponse(cmd.Context(), dxclient.IssueAddBlockRequest{
				Slug:      c.SlugOrDie(),
				Id:        int32(n),
				BlockedBy: by,
			})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
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
			body := dxclient.IssueRemoveBlockRequest{
				Slug: c.SlugOrDie(),
				Id:   int32(n),
			}
			if by != "" {
				body.BlockedBy = &by
			}
			resp, err := c.IssueRemoveBlockWithResponse(cmd.Context(), body)
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
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
			body := dxclient.EditIssueRequest{
				Slug: c.SlugOrDie(),
				Id:   int32(n),
			}
			if cmd.Flags().Changed("title") {
				body.Title = &title
			}
			if cmd.Flags().Changed("context") {
				body.Context = &ctx
			}
			if cmd.Flags().Changed("priority") {
				p := int32(priority)
				body.Priority = &p
			}
			if cmd.Flags().Changed("component") {
				body.Component = &component
			}
			if cmd.Flags().Changed("type") {
				body.IssueType = &issueType
			}
			resp, err := c.EditIssueWithResponse(cmd.Context(), body)
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
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
			body := dxclient.ResolveIssueRequest{
				Slug: c.SlugOrDie(),
				Id:   int32(n),
				Shas: &fullSHAs,
			}
			if branch != "" {
				body.Branch = &branch
			}
			resp, err := c.ResolveIssueWithResponse(cmd.Context(), body)
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil {
				return fmt.Errorf("empty response")
			}
			res := resp.JSON200.Resolution
			fmt.Printf("%s resolved (resolution %s)\n", id, res.Id)
			if res.Commits != nil {
				for _, commit := range *res.Commits {
					sha := commit.Sha
					if len(sha) > 10 {
						sha = sha[:10]
					}
					fmt.Printf("  %s\n", sha)
				}
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
			params := &dxclient.ListIssueResolutionsParams{
				Slug: c.SlugOrDie(),
				Id:   args[0],
			}
			if branch != "" {
				params.Branch = &branch
			}
			resp, err := c.ListIssueResolutionsWithResponse(cmd.Context(), params)
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil || resp.JSON200.Resolutions == nil || len(*resp.JSON200.Resolutions) == 0 {
				fmt.Println("no resolutions")
				return nil
			}
			for _, r := range *resp.JSON200.Resolutions {
				status := r.Source
				if r.Reverted {
					revertedBy := ""
					if r.RevertedBy != nil {
						revertedBy = *r.RevertedBy
					}
					if len(revertedBy) > 10 {
						revertedBy = revertedBy[:10]
					}
					status = "REVERTED (by " + revertedBy + ")"
				}
				date := r.ResolvedAt
				if len(date) >= 10 {
					date = date[:10]
				}
				idShort := r.Id
				if len(idShort) > 8 {
					idShort = idShort[:8]
				}
				fmt.Printf("  %s  %s  %s  [%s]  %s\n", idShort, date, r.BranchOfOrigin, status, r.Author)
				if r.Commits != nil {
					for _, commit := range *r.Commits {
						sha := commit.Sha
						if len(sha) > 10 {
							sha = sha[:10]
						}
						fmt.Printf("    %s\n", sha)
					}
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
			resp, err := c.ReconcileBranchWithResponse(cmd.Context(), dxclient.ReconcileBranchRequest{
				Slug:   c.SlugOrDie(),
				Branch: branch,
			})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil || resp.JSON200.Results == nil || len(*resp.JSON200.Results) == 0 {
				fmt.Println("no new reconciled resolutions found")
				return nil
			}
			for _, r := range *resp.JSON200.Results {
				resID := r.ResolutionId
				if len(resID) > 8 {
					resID = resID[:8]
				}
				fmt.Printf("  %s → %s", r.IssueId, resID)
				if r.MatchedShas != nil {
					for _, sha := range *r.MatchedShas {
						if len(sha) > 10 {
							sha = sha[:10]
						}
						fmt.Printf(" %s", sha)
					}
				}
				fmt.Println()
			}
			return nil
		},
	}
}

func issueClaimCmd() *cobra.Command {
	var as string
	var leaseDuration int32
	cmd := &cobra.Command{
		Use:   "claim <IS-N>",
		Short: "Claim an issue reservation for exclusive work",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			slug := c.SlugOrDie()
			claimedBy := as
			if claimedBy == "" {
				claimedBy = os.Getenv("DX_AUTHOR_ALIAS")
			}
			if claimedBy == "" {
				claimedBy = "agent"
			}
			resp, err := c.ClaimIssueWithResponse(cmd.Context(), slug, args[0], dxclient.ClaimIssueJSONRequestBody{
				ClaimedBy:        claimedBy,
				LeaseDurationMin: leaseDuration,
			})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil {
				return fmt.Errorf("empty response")
			}
			fmt.Printf("%s claimed by %s (expires %s)\n", args[0], resp.JSON200.ClaimedBy, resp.JSON200.LeaseExpiresAt)
			return nil
		},
	}
	cmd.Flags().StringVar(&as, "as", "", "agent alias; falls back to $DX_AUTHOR_ALIAS")
	cmd.Flags().Int32Var(&leaseDuration, "lease-minutes", 30, "lease duration in minutes")
	return cmd
}

func issueReleaseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "release <IS-N>",
		Short: "Release the active reservation on an issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			slug := c.SlugOrDie()
			resp, err := c.ReleaseIssueWithResponse(cmd.Context(), slug, args[0])
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			fmt.Printf("%s released\n", args[0])
			return nil
		},
	}
}

// RunIssue kept for compatibility.
func RunIssue(_ []string) {}
