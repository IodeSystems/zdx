package project

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
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

// parseIssueID validates an "IS-N" identifier and returns the numeric portion.
// Returns an error rather than panicking on malformed/empty input — the prior
// inline `id[3:]` slice panicked on any input shorter than 3 chars.
func parseIssueID(id string) (int32, error) {
	if !strings.HasPrefix(id, "IS-") {
		return 0, fmt.Errorf("invalid issue ID %q: expected IS-N", id)
	}
	n, err := strconv.ParseInt(id[3:], 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid issue ID %q: %w", id, err)
	}
	return int32(n), nil
}

// decodeEscapes interprets the common backslash escape sequences (\n, \t, \\)
// in CLI flag values. LLM-driven agents often pass JSON-style strings through
// shell command substitution where the shell does not interpret \n itself, so
// the CLI receives a literal backslash-n that would otherwise be persisted as
// such. Multi-paragraph contexts have round-tripped as one wrapped line because
// of this. (IS-655 newline decoding fix.)
func decodeEscapes(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 >= len(s) {
			b.WriteByte(s[i])
			continue
		}
		switch s[i+1] {
		case 'n':
			b.WriteByte('\n')
			i++
		case 't':
			b.WriteByte('\t')
			i++
		case 'r':
			b.WriteByte('\r')
			i++
		case '\\':
			b.WriteByte('\\')
			i++
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
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
				if iss.CloseReason != nil && *iss.CloseReason != "" {
					s += " [force:" + *iss.CloseReason + "]"
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
		Long: `Create an issue — a unit of tracked work or decision.

Issue types (--type):
  impl    implementation work (default if unclear)
  ops     operational / infrastructure work
  ask     question or decision that needs an answer
  tracker rollup / parent for a group of related issues

A good issue title is specific and outcome-oriented:
  - "Add --metric-name flag to dx goal add" (impl)
  - "Deploy pgvector extension on prod" (ops)
  - "Decide: store agent logs in DB or object storage?" (ask)

Use --context to capture motivation and constraints — this is what agents read.
Use --parent to nest under a tracker issue when decomposing large work.

Example:
  dx issue add --title="Add inline guidance to entity-creation CLI commands" \
    --type=impl --context="Spec 110: users need examples when running --help"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			title = decodeEscapes(title)
			ctx = decodeEscapes(ctx)
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
				parentNum, err := parseIssueID(parent)
				if err != nil {
					return fmt.Errorf("created issue but parent flag invalid: %w", err)
				}
				compositionKind := "composition"
				blkResp, err := c.IssueAddBlockWithResponse(cmd.Context(), dxclient.IssueAddBlockRequest{
					Slug:      c.SlugOrDie(),
					Id:        parentNum,
					BlockedBy: clitypes.IssueIDStr(addResp.Id),
					Kind:      &compositionKind,
				})
				if err != nil {
					return fmt.Errorf("created issue but failed to add block on parent: %w", err)
				}
				if err := c.CheckStatus(blkResp.StatusCode(), blkResp.Body); err != nil {
					return fmt.Errorf("created issue but failed to add block on parent: %w", err)
				}
				fmt.Printf("  → child of %s\n", parent)
			}
			if addResp.SuggestedBlockers != nil && len(*addResp.SuggestedBlockers) > 0 {
				newID := clitypes.IssueIDStr(addResp.Id)
				fmt.Println("\nContext references these issues — wire any real deps:")
				for _, ref := range *addResp.SuggestedBlockers {
					fmt.Printf("  dx issue block %s --by=%s\n", newID, ref)
				}
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
	var force bool
	var duplicateOf string
	var linkOf string
	cmd := &cobra.Command{
		Use:   "close <IS-N>",
		Short: "Close an issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			forceReasons := map[string]bool{"duplicate": true, "wontfix": true, "superseded": true}
			if forceReasons[reason] && !force {
				return fmt.Errorf("closing with --reason=%s requires --force", reason)
			}
			if force && !forceReasons[reason] {
				return fmt.Errorf("--force requires --reason=duplicate|wontfix|superseded")
			}
			if reason == "duplicate" && duplicateOf == "" {
				return fmt.Errorf("--duplicate-of is required when --reason=duplicate")
			}
			if reason == "link" && linkOf == "" {
				return fmt.Errorf("--link-of is required when --reason=link")
			}
			if duplicateOf != "" && linkOf != "" {
				return fmt.Errorf("--duplicate-of and --link-of are mutually exclusive")
			}
			id := args[0]
			n, err := parseIssueID(id)
			if err != nil {
				return err
			}
			c := cli.MustClient()

			// Test gate: run targeted tests before closing as "done".
			// Skip for force-close reasons (bypass gate) and for
			// tracker/ops issues that don't have code-level tests.
			if !force && reason != "link" {
				showResp, err := c.ShowIssueWithResponse(cmd.Context(), &dxclient.ShowIssueParams{Slug: c.SlugOrDie(), Id: id})
				if err == nil && showResp.JSON200 != nil {
					issueType := showResp.JSON200.Issue.IssueType
					if issueType != "tracker" && issueType != "ops" {
						if err := runDecompositionPathGate(cmd, c, id, showResp.JSON200.Issue.Context, showResp.JSON200.Issue.Title); err != nil {
							return err
						}
						if err := runSpecCoverageGate(cmd, c, id); err != nil {
							return err
						}
						if err := runMustSpecDemoGate(cmd, c, id); err != nil {
							return err
						}
						if err := runIssueTestGate(cmd, c, id); err != nil {
							return err
						}
					}
				}
			}
			body := dxclient.CloseIssueRequest{
				Slug: c.SlugOrDie(),
				Id:   int32(n),
			}
			if reason != "" {
				body.Reason = &reason
			}
			if force {
				body.Force = &force
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
	cmd.Flags().StringVar(&reason, "reason", "", "close reason (done|duplicate|link|wontfix|superseded)")
	cmd.Flags().BoolVar(&force, "force", false, "bypass close gate (required with --reason=duplicate|wontfix|superseded)")
	cmd.Flags().StringVar(&duplicateOf, "duplicate-of", "", "issue ID this duplicates (required when --reason=duplicate)")
	cmd.Flags().StringVar(&linkOf, "link-of", "", "issue ID this is a narrow-slice link of (required when --reason=link; cascade-closes with target, no reopen-cascade)")
	return cmd
}

// runDecompositionPathGate blocks close-as-done when the issue context
// describes unfinished work (list items, future-tense signals, or an explicit
// DECOMPOSITION section) without those items being filed as child issues.
// Pure function over the already-fetched context — no extra round-trip.
func runDecompositionPathGate(cmd *cobra.Command, c *cli.Client, issueID, issueContext, issueTitle string) error {
	_ = cmd
	_ = c
	_ = issueTitle
	candidates := extractDecompositionCandidates(issueContext)
	if len(candidates) == 0 {
		return nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "cannot close %s as done — context describes unfinished work:\n", issueID)
	for _, ch := range candidates {
		fmt.Fprintf(&b, "  - %s\n", ch)
	}
	b.WriteString("File children, then retry close:\n")
	for _, ch := range candidates {
		fmt.Fprintf(&b, "  dx issue add --parent=%s --type=impl --title=%q\n", issueID, ch)
	}
	b.WriteString("Or override:\n")
	b.WriteString("  - --reason=wontfix to abandon, --reason=duplicate --duplicate-of=IS-X to dedupe\n")
	fmt.Fprintf(&b, "  - dx issue edit %s --type=tracker if this is a tracker, not an impl\n", issueID)
	return fmt.Errorf("%s", b.String())
}

var (
	decompListItemRe = regexp.MustCompile(`^\s*(?:\d+\.|[-*])\s+(.+)$`)
	decompHeaderRe   = regexp.MustCompile(`(?i)^\s*#*\s*DECOMPOSITION\b`)
	decompAnyHeader  = regexp.MustCompile(`^\s*#+\s+\S`)
	// decompExemptHeaderRe matches the standard impl-issue template section
	// headers. Lines inside these sections are descriptive prose, not pending
	// work — list items and future-tense signals inside them must not be
	// extracted as decomposition candidates. Matches with or without leading
	// markdown hashes and optional trailing colon. (IS-675)
	decompExemptHeaderRe = regexp.MustCompile(`(?i)^\s*(?:#+\s*)?(WHAT SHOULD HAPPEN|WHAT DID HAPPEN|WHAT THE TOOL SHOULD DO|IMPLEMENTATION DIRECTION|REPRO|CANDIDATE FIXES|FIX|OUT OF SCOPE|EXAMPLES)\b`)
	// decompCodeFenceRe matches a markdown code-fence opening or closing line.
	// Lines inside fences must not be extracted — they are example commands or
	// quoted output, not pending work. Without this guard, a diff line like
	// "- -- Dumped by pg_dump..." inside a code block matches decompListItemRe
	// and shows up as a bogus candidate.
	decompCodeFenceRe = regexp.MustCompile("^\\s*```")
)

// decompFutureSignals are case-insensitive substrings that flag a line as
// describing pending work even when it is not in a list. Signals that end with
// a letter/digit are matched with a word-boundary check so "todo" does not
// match "todos". Signals with trailing spaces are already boundary-safe.
var decompFutureSignals = []string{"will ", "should ", "needs to ", "todo", "next step"}

// decompFutureSignalMatches returns true when lc contains sig. For signals
// whose last character is a word character (a-z, 0-9, _), the character
// immediately after the match must not also be a word character — this
// prevents "todo" from firing on "todos".
func decompFutureSignalMatches(lc, sig string) bool {
	idx := strings.Index(lc, sig)
	if idx < 0 {
		return false
	}
	last := sig[len(sig)-1]
	isWordChar := func(c byte) bool { return (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' }
	if isWordChar(last) {
		end := idx + len(sig)
		if end < len(lc) && isWordChar(lc[end]) {
			return false
		}
	}
	return true
}

const decompCandidateCap = 20

// extractDecompositionCandidates inspects an issue context for unfinished-work
// signals and returns deduped candidate child-issue titles, capped at
// decompCandidateCap. Detected signals: list items (numbered or bulleted),
// future-tense lines, and any non-empty non-header line under an explicit
// DECOMPOSITION header.
func extractDecompositionCandidates(context string) []string {
	if strings.TrimSpace(context) == "" {
		return nil
	}
	var candidates []string
	seen := map[string]bool{}
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] || len(candidates) >= decompCandidateCap {
			return
		}
		seen[s] = true
		candidates = append(candidates, s)
	}
	inDecomp := false
	inExempt := false
	inFence := false
	for _, line := range strings.Split(context, "\n") {
		if decompCodeFenceRe.MatchString(line) {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if decompHeaderRe.MatchString(line) {
			inDecomp = true
			inExempt = false
			continue
		}
		if decompExemptHeaderRe.MatchString(line) {
			inExempt = true
			inDecomp = false
			continue
		}
		isHeader := decompAnyHeader.MatchString(line)
		if isHeader {
			// Any other markdown header exits both modes.
			inDecomp = false
			inExempt = false
		}
		if inExempt {
			continue
		}
		if m := decompListItemRe.FindStringSubmatch(line); m != nil {
			add(m[1])
			continue
		}
		if inDecomp {
			if !isHeader && strings.TrimSpace(line) != "" {
				add(line)
			}
			continue
		}
		lc := strings.ToLower(line)
		for _, sig := range decompFutureSignals {
			if decompFutureSignalMatches(lc, sig) {
				add(line)
				break
			}
		}
	}
	return candidates
}

// runSpecCoverageGate blocks close-as-done when any spec on a linked feature
// is not deferred by an open blocking issue and lacks passing test coverage.
// Warns (non-fatal) if the endpoint is unavailable — matches test-gate
// behavior so a transient server error does not strand an otherwise-ready
// close.
func runSpecCoverageGate(cmd *cobra.Command, c *cli.Client, issueID string) error {
	resp, err := c.ListSpecCloseGateOffendersWithResponse(cmd.Context(), &dxclient.ListSpecCloseGateOffendersParams{
		Slug: c.SlugOrDie(),
		Id:   issueID,
	})
	if err != nil || resp.JSON200 == nil || resp.JSON200.Offenders == nil {
		fmt.Fprintf(os.Stderr, "[warn] could not fetch spec coverage for %s — skipping spec-coverage gate\n", issueID)
		return nil
	}
	offenders := *resp.JSON200.Offenders
	if len(offenders) == 0 {
		return nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "cannot close %s as done — these non-deferred specs have no passing tests:\n", issueID)
	for _, o := range offenders {
		fmt.Fprintf(&b, "  Spec %d (feature %q): %s [%s]\n", o.SpecId, o.Feature, o.Description, o.Reason)
	}
	b.WriteString("Options:\n")
	b.WriteString("  - Write/fix the tests, then retry close\n")
	b.WriteString("  - Defer the spec against a blocking issue (dx spec defer <spec-id> --blocked-by=IS-X)\n")
	b.WriteString("  - Close for a different reason: --reason=wontfix, --reason=duplicate")
	return fmt.Errorf("%s", b.String())
}

// runMustSpecDemoGate blocks close-as-done when any must-spec on a linked
// feature is not deferred and lacks a passing demo (a passing test with
// component=demo or an attached zdx_test_demos artifact). Demos are the
// user-visible counterpart to the spec-coverage gate's tests.
func runMustSpecDemoGate(cmd *cobra.Command, c *cli.Client, issueID string) error {
	resp, err := c.ListMustSpecDemoGateOffendersWithResponse(cmd.Context(), &dxclient.ListMustSpecDemoGateOffendersParams{
		Slug: c.SlugOrDie(),
		Id:   issueID,
	})
	if err != nil || resp.JSON200 == nil || resp.JSON200.Offenders == nil {
		fmt.Fprintf(os.Stderr, "[warn] could not fetch demo coverage for %s — skipping demo gate\n", issueID)
		return nil
	}
	offenders := *resp.JSON200.Offenders
	if len(offenders) == 0 {
		return nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "cannot close %s as done — these must-specs have no passing demo:\n", issueID)
	for _, o := range offenders {
		fmt.Fprintf(&b, "  Spec %d (feature %q): %s\n", o.SpecId, o.Feature, o.Description)
	}
	b.WriteString("Options:\n")
	b.WriteString("  - Record a passing demo: dx test demo <spec-id>\n")
	b.WriteString("  - Defer the spec against a blocking issue (dx spec defer <spec-id> --blocked-by=IS-X)\n")
	b.WriteString("  - Close for a different reason: --reason=wontfix, --reason=duplicate")
	return fmt.Errorf("%s", b.String())
}

// runIssueTestGate collects features from the issue's tasks and runs dx test
// --feature=<name> for each. Returns an error if any test suite fails.
// Warns (non-fatal) if no features are linked.
func runIssueTestGate(cmd *cobra.Command, c *cli.Client, issueID string) error {
	tasksResp, err := c.ListTasksForIssueWithResponse(cmd.Context(), &dxclient.ListTasksForIssueParams{
		Slug:    c.SlugOrDie(),
		IssueId: issueID,
	})
	if err != nil || tasksResp.JSON200 == nil || tasksResp.JSON200.Tasks == nil {
		fmt.Fprintf(os.Stderr, "[warn] could not fetch tasks for %s — skipping test gate\n", issueID)
		return nil
	}

	seen := map[string]bool{}
	for _, task := range *tasksResp.JSON200.Tasks {
		if task.Feature != "" {
			seen[task.Feature] = true
		}
	}

	if len(seen) == 0 {
		fmt.Fprintf(os.Stderr, "[warn] %s has no features linked to its tasks — skipping test gate (maturity gap)\n", issueID)
		return nil
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot determine executable path for test gate: %w", err)
	}

	var failed []string
	for feature := range seen {
		fmt.Printf("[test] feature %q\n", feature)
		tc := exec.Command(exe, "test", "--feature="+feature)
		tc.Stdout = os.Stdout
		tc.Stderr = os.Stderr
		if err := tc.Run(); err != nil {
			failed = append(failed, feature)
		}
	}

	if len(failed) > 0 {
		return fmt.Errorf("tests failed for features: %s\n  fix failures or file a task to track them, then retry close", strings.Join(failed, ", "))
	}
	return nil
}

func issueReopenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reopen <IS-N>",
		Short: "Reopen a closed issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			n, err := parseIssueID(id)
			if err != nil {
				return err
			}
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
	var by, kind string
	cmd := &cobra.Command{
		Use:   "block <IS-N>",
		Short: "Add a blocker to an issue",
		Long: `Add a blocker edge to an issue.

Kinds (--kind):
  sequencing   (default) "X waits for Y" — Y must close before X is claimable
  composition  tracker → child — Y is a sub-vertical that contributes to X closing

Re-running with a different --kind on an existing edge re-classifies it.
Use composition when retagging pre-migration tracker children whose edges
defaulted to sequencing.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			n, err := parseIssueID(id)
			if err != nil {
				return err
			}
			if kind != "sequencing" && kind != "composition" {
				return fmt.Errorf("--kind must be 'sequencing' or 'composition' (got %q)", kind)
			}
			c := cli.MustClient()
			resp, err := c.IssueAddBlockWithResponse(cmd.Context(), dxclient.IssueAddBlockRequest{
				Slug:      c.SlugOrDie(),
				Id:        int32(n),
				BlockedBy: by,
				Kind:      &kind,
			})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			label := "blocked by"
			if kind == "composition" {
				label = "is parent of"
			}
			fmt.Printf("%s %s %s\n", id, label, by)
			return nil
		},
	}
	cmd.Flags().StringVar(&by, "by", "", "blocking issue (IS-N)")
	cmd.Flags().StringVar(&kind, "kind", "sequencing", "edge kind: sequencing (default) or composition")
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
			n, err := parseIssueID(id)
			if err != nil {
				return err
			}
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
	var interactiveOnly bool
	cmd := &cobra.Command{
		Use:   "edit <IS-N>",
		Short: "Edit fields on an existing issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			n, err := parseIssueID(id)
			if err != nil {
				return err
			}
			c := cli.MustClient()
			title = decodeEscapes(title)
			ctx = decodeEscapes(ctx)
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
			if cmd.Flags().Changed("interactive-only") {
				body.InteractiveOnly = &interactiveOnly
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
	cmd.Flags().BoolVar(&interactiveOnly, "interactive-only", false, "mark issue as requiring human interaction (hides from autonomous agent loop)")
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
			n, err := parseIssueID(id)
			if err != nil {
				return err
			}
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
