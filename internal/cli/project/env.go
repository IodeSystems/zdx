package project

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/dxclient"
)

func gitOutput(args ...string) string {
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func EnvCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "env", Short: "Environment management"}
	cmd.AddCommand(
		envListCmd(),
		envAddCmd(),
		envShowCmd(),
		envDeployCmd(),
		envShipCmd(),
		envEditCmd(),
		envRmCmd(),
	)
	return cmd
}

func envListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List environments",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			resp, err := c.ListEnvironmentsWithResponse(cmd.Context(), c.SlugOrDie())
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil || resp.JSON200.Items == nil || len(*resp.JSON200.Items) == 0 {
				fmt.Println("no environments")
				return nil
			}
			for _, e := range *resp.JSON200.Items {
				branch := e.ReleaseBranch
				if branch == "" {
					branch = "(no release branch)"
				}
				fmt.Printf("%-20s  %-30s  %s\n", e.Name, branch, e.Url)
			}
			return nil
		},
	}
}

func envAddCmd() *cobra.Command {
	var url, releaseBranch, trunkBranch string
	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add an environment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			req := dxclient.CreateEnvironmentRequest{Name: args[0]}
			if url != "" {
				req.Url = &url
			}
			if releaseBranch != "" {
				req.ReleaseBranch = &releaseBranch
			}
			if trunkBranch != "" {
				req.TrunkBranch = &trunkBranch
			}
			resp, err := c.CreateEnvironmentWithResponse(cmd.Context(), c.SlugOrDie(), req)
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil {
				return fmt.Errorf("empty response")
			}
			fmt.Printf("%d  %s  %s  %s\n", resp.JSON200.Id, resp.JSON200.Name, resp.JSON200.ReleaseBranch, resp.JSON200.Url)
			return nil
		},
	}
	cmd.Flags().StringVar(&url, "url", "", "environment URL")
	cmd.Flags().StringVar(&releaseBranch, "release-branch", "", "git branch to deploy from (e.g. release/production)")
	cmd.Flags().StringVar(&trunkBranch, "trunk-branch", "", "git branch that is the integration trunk (e.g. dev, main)")
	return cmd
}

func envShowCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "show <name>",
		Short: "Show environment detail and deploy history",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			slug := c.SlugOrDie()
			name := args[0]

			resp, err := c.GetEnvironmentWithResponse(cmd.Context(), slug, name)
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil {
				return fmt.Errorf("empty response")
			}
			e := resp.JSON200
			if asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(e)
			}
			fmt.Printf("Name:           %s\n", e.Name)
			fmt.Printf("URL:            %s\n", e.Url)
			fmt.Printf("Release branch: %s\n", e.ReleaseBranch)
			if e.TrunkBranch != "" {
				fmt.Printf("Trunk branch:   %s\n", e.TrunkBranch)
			}
			fmt.Printf("Created:        %s\n", e.CreatedAt)
			fmt.Printf("Deployed:       %s\n", e.DeployedAt)
			fmt.Printf("Branch:         %s\n", e.CurrentBuildBranch)
			fmt.Printf("SHA:            %s\n", e.CurrentBuildSha)
			if e.CurrentBuildSha != "" {
				ahead := gitOutput("rev-list", e.CurrentBuildSha+"..HEAD", "--count")
				if ahead != "" && ahead != "0" {
					fmt.Printf("Behind:     %s commit(s) behind HEAD\n", ahead)
				} else if ahead == "0" {
					fmt.Printf("Behind:     up to date\n")
				}
			}

			deploys, err := c.ListEnvironmentDeploysWithResponse(cmd.Context(), slug, name, &dxclient.ListEnvironmentDeploysParams{})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(deploys.StatusCode(), deploys.Body); err != nil {
				return err
			}
			if deploys.JSON200 == nil || deploys.JSON200.Items == nil || len(*deploys.JSON200.Items) == 0 {
				fmt.Println("\nno deploy history")
				return nil
			}
			fmt.Println("\nDeploys:")
			for _, d := range *deploys.JSON200.Items {
				fmt.Printf("  %-8s  %-12s  %s  %s\n", d.Status, d.BuildBranch, d.BuildSha[:8], d.DeployedAt)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output environment as JSON (omits deploy history)")
	return cmd
}

func envDeployCmd() *cobra.Command {
	var sha, branch, status string
	cmd := &cobra.Command{
		Use:   "deploy <name>",
		Short: "Record a deployment to an environment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if sha == "" {
				sha = gitOutput("rev-parse", "HEAD")
			}
			if branch == "" {
				branch = gitOutput("rev-parse", "--abbrev-ref", "HEAD")
			}
			if status == "" {
				status = "success"
			}
			c := cli.MustClient()
			req := dxclient.CreateEnvironmentDeployRequest{
				BuildSha:    sha,
				BuildBranch: &branch,
				Status:      &status,
			}
			resp, err := c.CreateEnvironmentDeployWithResponse(cmd.Context(), c.SlugOrDie(), args[0], req)
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil {
				return fmt.Errorf("empty response")
			}
			d := resp.JSON200
			shortSHA := d.BuildSha
			if len(shortSHA) > 8 {
				shortSHA = shortSHA[:8]
			}
			fmt.Printf("recorded  %s  %s  %s\n", d.Status, d.BuildBranch, shortSHA)
			return nil
		},
	}
	cmd.Flags().StringVar(&sha, "sha", "", "build SHA (default: current HEAD)")
	cmd.Flags().StringVar(&branch, "branch", "", "build branch (default: current branch)")
	cmd.Flags().StringVar(&status, "status", "", "deploy status: success|failure (default: success)")
	return cmd
}

// envShipCmd is the full deploy pipeline: lint trunk → fast-forward release
// branch from trunk → run bin/ship → record the deploy in zdx. Replaces the
// "switch to release branch, merge, ship, paste sha into dx env deploy" dance
// the operator used to do by hand.
//
// Why this lives here instead of in bin/ship:
//   - bin/ship is intentionally branch-agnostic — it takes whatever HEAD is
//     and deploys it. It has no concept of "trunk integration".
//   - The trunk → release fast-forward is a per-environment decision (each env
//     has its own release_branch + trunk_branch), so the env model is the
//     right place to anchor the policy.
//   - Running it from `dx` means the deploy record is posted as part of the
//     same flow rather than relying on bin/ship's deploy.api_url config that
//     most setups omit (we hit "skipping deploy record POST" warnings on
//     every recent ship).
func envShipCmd() *cobra.Command {
	var trunkOverride string
	var skipLint, skipTests, skipPush, skipPromote, dryRun bool
	cmd := &cobra.Command{
		Use:   "ship <name>",
		Short: "Promote trunk → release branch, deploy to environment, record the deploy",
		Long: `Run the full deploy pipeline for one environment:

  1. Fetch the env config — release_branch must be set; trunk_branch defaults
     to 'dev' when unset (override with --trunk).
  2. Refuse if the working tree is dirty.
  3. ` + "`git fetch origin`" + ` so trunk/release tracking is current.
  4. Lint the trunk branch (skip with --skip-lint, or auto-skip if ./bin/lint
     doesn't exist) so we don't promote churn.
  5. Test the trunk branch (skip with --skip-tests, or auto-skip if ./bin/test
     doesn't exist).
  6. Switch to release_branch, ` + "`git merge --ff-only trunk_branch`" + `. Refuse on
     non-fast-forward (operator must rebase trunk onto release first).
  7. ` + "`git push origin release_branch`" + ` (skip with --skip-push).
  8. Run ./bin/ship to build + roll out.
  9. POST a deploy record so the env shows updated SHA/branch.

Promotion-only mode: if trunk_branch == release_branch (or --skip-promote is set),
steps 4–7 are skipped. Use this for envs that deploy a single branch as-is
(e.g. trunk=main, release=main). To "sync" such an env from another branch,
pass --trunk <other> on the command line.

Restores the original branch on exit, even on failure.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			envName := args[0]
			c := cli.MustClient()
			slug := c.SlugOrDie()

			// 1. Pull env config from server — single source of truth for
			// release_branch + trunk_branch. Don't infer from local config.
			envResp, err := c.GetEnvironmentWithResponse(cmd.Context(), slug, envName)
			if err != nil {
				return err
			}
			if err := c.CheckStatus(envResp.StatusCode(), envResp.Body); err != nil {
				return err
			}
			if envResp.JSON200 == nil {
				return fmt.Errorf("environment %q: empty response", envName)
			}
			env := envResp.JSON200
			if env.ReleaseBranch == "" {
				return fmt.Errorf("environment %q has no release_branch — set one with `dx env edit %s --release-branch=…`", envName, envName)
			}
			trunk := trunkOverride
			if trunk == "" {
				trunk = env.TrunkBranch
			}
			if trunk == "" {
				trunk = "dev" // sensible default for the dev→main flow
			}
			// trunk == release_branch means promotion is a no-op — deploy whatever
			// is on the branch as-is. Auto-imply --skip-promote so callers don't
			// have to pass the flag for the natural single-branch case.
			if trunk == env.ReleaseBranch {
				skipPromote = true
			}

			if skipPromote {
				fmt.Printf("[ship] env=%s  release=%s (promotion skipped)\n", envName, env.ReleaseBranch)
			} else {
				fmt.Printf("[ship] env=%s  trunk=%s → release=%s\n", envName, trunk, env.ReleaseBranch)
			}

			// 2. Refuse on dirty tree. The pipeline does branch switches + merge
			// commits, both of which interact badly with uncommitted edits.
			if cli.GitTreeDirty() {
				return fmt.Errorf("working tree dirty — commit or stash before shipping")
			}

			startBranch := gitOutput("rev-parse", "--abbrev-ref", "HEAD")
			if startBranch == "" {
				return fmt.Errorf("could not resolve current branch — detached HEAD?")
			}
			// Restore the operator's branch on exit so a successful (or failed)
			// ship doesn't leave them sitting on release_branch by accident.
			defer func() {
				if cur := gitOutput("rev-parse", "--abbrev-ref", "HEAD"); cur != startBranch {
					if out, gerr := exec.Command("git", "switch", startBranch).CombinedOutput(); gerr != nil {
						fmt.Fprintf(os.Stderr, "[ship] warn: failed to restore %q: %s\n", startBranch, strings.TrimSpace(string(out)))
					}
				}
			}()

			if dryRun {
				if skipPromote {
					fmt.Println("[ship] --dry-run: would fetch, ship (current branch), record")
				} else {
					fmt.Println("[ship] --dry-run: would fetch, lint, test, ff-merge, push, ship, record")
				}
				return nil
			}

			// 3. Fetch so trunk/release reflect what's on origin. Best-effort —
			// a missing remote is OK if the operator is shipping a local-only
			// branch (rare, but bin/ship handles it).
			if out, gerr := exec.Command("git", "fetch", "origin").CombinedOutput(); gerr != nil {
				fmt.Fprintf(os.Stderr, "[ship] warn: git fetch origin: %s\n", strings.TrimSpace(string(out)))
			}

			if !skipPromote {
				// 4. Lint trunk. Best-effort: skip if ./bin/lint isn't present
				// (some projects don't have one), or if the operator has
				// already linted in this session and passed --skip-lint.
				if !skipLint {
					if _, statErr := os.Stat("./bin/lint"); statErr != nil {
						fmt.Println("[ship] no ./bin/lint — skipping lint")
					} else {
						if out, gerr := exec.Command("git", "switch", trunk).CombinedOutput(); gerr != nil {
							return fmt.Errorf("git switch %s: %s", trunk, strings.TrimSpace(string(out)))
						}
						fmt.Printf("[ship] linting %s...\n", trunk)
						lint := exec.Command("./bin/lint")
						lint.Stdout = os.Stdout
						lint.Stderr = os.Stderr
						if lerr := lint.Run(); lerr != nil {
							return fmt.Errorf("lint failed on %s: %w", trunk, lerr)
						}
					}
				}

				// 5. Test trunk. Best-effort: skip if ./bin/test isn't present.
				// The release gate should run the project's own test suite; if
				// the project doesn't ship one, that's a doctor concern, not a
				// ship concern.
				if !skipTests {
					if _, statErr := os.Stat("./bin/test"); statErr != nil {
						fmt.Println("[ship] no ./bin/test — skipping tests")
					} else {
						if out, gerr := exec.Command("git", "switch", trunk).CombinedOutput(); gerr != nil {
							return fmt.Errorf("git switch %s: %s", trunk, strings.TrimSpace(string(out)))
						}
						fmt.Printf("[ship] testing %s...\n", trunk)
						test := exec.Command("./bin/test")
						test.Stdout = os.Stdout
						test.Stderr = os.Stderr
						if terr := test.Run(); terr != nil {
							return fmt.Errorf("tests failed on %s: %w", trunk, terr)
						}
					}
				}

				// 6. Fast-forward release_branch from trunk. --ff-only is the
				// invariant: a merge commit on the release branch hides what
				// the next ship will actually deploy. If trunk has diverged,
				// the operator must rebase trunk onto release first.
				if out, gerr := exec.Command("git", "switch", env.ReleaseBranch).CombinedOutput(); gerr != nil {
					return fmt.Errorf("git switch %s: %s", env.ReleaseBranch, strings.TrimSpace(string(out)))
				}
				fmt.Printf("[ship] fast-forwarding %s ← %s...\n", env.ReleaseBranch, trunk)
				if out, gerr := exec.Command("git", "merge", "--ff-only", trunk).CombinedOutput(); gerr != nil {
					return fmt.Errorf("ff-merge %s ← %s failed: %s\n(rebase %s onto %s and retry)",
						env.ReleaseBranch, trunk, strings.TrimSpace(string(out)), trunk, env.ReleaseBranch)
				}

				// 7. Push release_branch so origin tracks the deployable SHA.
				// Some flows ship from local without pushing first; --skip-push
				// lets those pass through.
				if !skipPush {
					if out, gerr := exec.Command("git", "push", "origin", env.ReleaseBranch).CombinedOutput(); gerr != nil {
						return fmt.Errorf("git push origin %s: %s", env.ReleaseBranch, strings.TrimSpace(string(out)))
					}
				}
			} else {
				// Promotion-only mode skipped: ensure we're on the release
				// branch so bin/ship deploys the right thing.
				if out, gerr := exec.Command("git", "switch", env.ReleaseBranch).CombinedOutput(); gerr != nil {
					return fmt.Errorf("git switch %s: %s", env.ReleaseBranch, strings.TrimSpace(string(out)))
				}
			}

			// 7. Run bin/ship. It enforces deploy.release_branch matching the
			// current branch (which is now release_branch after the switch
			// above), runs its own compat-check, and rolls out.
			fmt.Println("[ship] running bin/ship...")
			ship := exec.Command("./bin/ship")
			ship.Stdout = os.Stdout
			ship.Stderr = os.Stderr
			if serr := ship.Run(); serr != nil {
				return fmt.Errorf("bin/ship failed: %w", serr)
			}

			// 8. Record the deploy so dx env show / Atlas / dashboards reflect
			// the new SHA. The bin/ship finalize step already does this when
			// deploy.api_url is configured; calling here is the canonical path
			// that doesn't depend on that config.
			deployedSHA := gitOutput("rev-parse", "HEAD")
			deployedBranch := env.ReleaseBranch
			status := "success"
			req := dxclient.CreateEnvironmentDeployRequest{
				BuildSha:    deployedSHA,
				BuildBranch: &deployedBranch,
				Status:      &status,
			}
			recResp, rerr := c.CreateEnvironmentDeployWithResponse(cmd.Context(), slug, envName, req)
			if rerr != nil {
				fmt.Fprintf(os.Stderr, "[ship] warn: failed to record deploy: %v (deploy itself succeeded)\n", rerr)
				return nil
			}
			if rerr := c.CheckStatus(recResp.StatusCode(), recResp.Body); rerr != nil {
				fmt.Fprintf(os.Stderr, "[ship] warn: deploy record API rejected: %v (deploy itself succeeded)\n", rerr)
				return nil
			}
			short := deployedSHA
			if len(short) > 8 {
				short = short[:8]
			}
			fmt.Printf("[ship] recorded deploy: env=%s branch=%s sha=%s\n", envName, deployedBranch, short)
			return nil
		},
	}
	cmd.Flags().StringVar(&trunkOverride, "trunk", "", "override trunk branch (default: env.trunk_branch or 'dev')")
	cmd.Flags().BoolVar(&skipLint, "skip-lint", false, "skip the trunk lint step (use only when you've just linted)")
	cmd.Flags().BoolVar(&skipTests, "skip-tests", false, "skip the trunk test step")
	cmd.Flags().BoolVar(&skipPush, "skip-push", false, "skip pushing release_branch to origin")
	cmd.Flags().BoolVar(&skipPromote, "skip-promote", false, "skip lint/test/ff-merge/push; just ship the current state of release_branch (auto-set when trunk == release_branch)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "validate args + restore branch, but don't run any git/ship steps")
	return cmd
}

func envEditCmd() *cobra.Command {
	var url, releaseBranch, trunkBranch string
	cmd := &cobra.Command{
		Use:   "edit <name>",
		Short: "Edit an environment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			req := dxclient.UpdateEnvironmentRequest{}
			if url != "" {
				req.Url = &url
			}
			if releaseBranch != "" {
				req.ReleaseBranch = &releaseBranch
			}
			if trunkBranch != "" {
				req.TrunkBranch = &trunkBranch
			}
			resp, err := c.UpdateEnvironmentWithResponse(cmd.Context(), c.SlugOrDie(), args[0], req)
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			fmt.Println("updated")
			return nil
		},
	}
	cmd.Flags().StringVar(&url, "url", "", "environment URL")
	cmd.Flags().StringVar(&releaseBranch, "release-branch", "", "git branch to deploy from (e.g. release/production)")
	cmd.Flags().StringVar(&trunkBranch, "trunk-branch", "", "git branch that is the integration trunk (e.g. dev, main)")
	return cmd
}

func envRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <name>",
		Short: "Remove an environment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			resp, err := c.DeleteEnvironmentWithResponse(cmd.Context(), c.SlugOrDie(), args[0])
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			fmt.Println("deleted")
			return nil
		},
	}
}
