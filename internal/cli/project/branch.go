package project

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/dxclient"
)

func BranchCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "branch", Short: "Version branch management"}
	cmd.AddCommand(branchCutCmd(), branchAddRoleCmd(), branchListCmd(), branchShowCmd(), branchEolCmd(), branchSetSourceCmd(), branchUpgradeCmd())
	return cmd
}

// currentGitBranch returns the current branch name via `git rev-parse
// --abbrev-ref HEAD`, or "" on error / detached HEAD ("HEAD").
func currentGitBranch() string {
	out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return ""
	}
	name := strings.TrimSpace(string(out))
	if name == "HEAD" {
		return ""
	}
	return name
}

func branchCutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cut <name> [semver]",
		Short: "Create a named version branch",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			slug := c.SlugOrDie()
			body := dxclient.CreateVersionBranchRequest{Name: args[0]}
			if len(args) == 2 {
				s := args[1]
				body.Semver = &s
			}
			if src := currentGitBranch(); src != "" {
				body.SourceBranchName = &src
			}
			resp, err := c.CreateVersionBranchWithResponse(cmd.Context(), slug, body)
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			fmt.Printf("Branch %s created.\n", args[0])
			if resp.JSON200 != nil && resp.JSON200.BackportTasksCreated > 0 {
				fmt.Printf("Auto-generated %d backport task(s) for open issues (priority <= %d).\n",
					resp.JSON200.BackportTasksCreated, 2)
			}
			return nil
		},
	}
}

func branchAddRoleCmd() *cobra.Command {
	var role, source string
	cmd := &cobra.Command{
		Use:   "add-role <name>",
		Short: "Register a branch with a role (rolling-release|dev|pr-target|named-release)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch role {
			case "rolling-release", "dev", "pr-target", "named-release":
			default:
				return fmt.Errorf("--role must be one of rolling-release, dev, pr-target, named-release")
			}
			c := cli.MustClient()
			slug := c.SlugOrDie()
			body := dxclient.CreateVersionBranchRequest{Name: args[0], Role: &role}
			if source != "" {
				body.SourceBranchName = &source
			}
			resp, err := c.CreateVersionBranchWithResponse(cmd.Context(), slug, body)
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			fmt.Printf("Branch %s registered with role %s.\n", args[0], role)
			return nil
		},
	}
	cmd.Flags().StringVar(&role, "role", "", "branch role (rolling-release|dev|pr-target|named-release)")
	cmd.Flags().StringVar(&source, "source", "", "source branch this branch was cut from")
	_ = cmd.MarkFlagRequired("role")
	return cmd
}

func branchListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List version branches",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			slug := c.SlugOrDie()
			resp, err := c.ListVersionBranchesWithResponse(cmd.Context(), slug)
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil || resp.JSON200.Branches == nil || len(*resp.JSON200.Branches) == 0 {
				fmt.Println("no branches")
				return nil
			}
			branches := *resp.JSON200.Branches
			hasSemver := false
			hasSource := false
			for _, b := range branches {
				if b.Semver != nil && *b.Semver != "" {
					hasSemver = true
				}
				if b.SourceBranchName != nil && *b.SourceBranchName != "" {
					hasSource = true
				}
			}
			printBranchRow := func(name, role, status, semver, source string) {
				line := fmt.Sprintf("%-20s %-12s %-8s", name, role, status)
				if hasSemver {
					line += fmt.Sprintf(" %-12s", semver)
				}
				if hasSource {
					line += fmt.Sprintf(" %s", source)
				}
				fmt.Println(line)
			}
			semverHdr := ""
			if hasSemver {
				semverHdr = "SEMVER"
			}
			sourceHdr := ""
			if hasSource {
				sourceHdr = "SOURCE"
			}
			printBranchRow("NAME", "ROLE", "STATUS", semverHdr, sourceHdr)
			for _, b := range branches {
				role := b.Type
				if b.Role != nil && *b.Role != "" {
					role = *b.Role
				}
				sv := ""
				if b.Semver != nil {
					sv = *b.Semver
				}
				src := ""
				if b.SourceBranchName != nil {
					src = *b.SourceBranchName
				}
				printBranchRow(b.Name, role, b.Status, sv, src)
			}
			return nil
		},
	}
}

func branchShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <name>",
		Short: "Show version branch detail",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			slug := c.SlugOrDie()
			resp, err := c.ShowVersionBranchWithResponse(cmd.Context(), slug, args[0])
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil {
				return fmt.Errorf("empty response")
			}
			b := resp.JSON200
			role := b.Type
			if b.Role != nil && *b.Role != "" {
				role = *b.Role
			}
			fmt.Printf("Name:    %s\n", b.Name)
			fmt.Printf("Role:    %s\n", role)
			fmt.Printf("Status:  %s\n", b.Status)
			if b.Semver != nil && *b.Semver != "" {
				fmt.Printf("Semver:  %s\n", *b.Semver)
			}
			if b.SourceBranchName != nil && *b.SourceBranchName != "" {
				fmt.Printf("Source:  %s\n", *b.SourceBranchName)
			}
			fmt.Printf("Created: %s\n", b.CreatedAt)
			fmt.Printf("Open issues: %d / Resolved: %d\n", b.OpenCount, b.ResolvedCount)
			return nil
		},
	}
}

func branchSetSourceCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set-source <name> <source-branch>",
		Short: "Set the source branch for a version branch",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			slug := c.SlugOrDie()
			body := dxclient.SetVersionBranchSourceRequest{SourceBranchName: args[1]}
			resp, err := c.SetVersionBranchSourceWithResponse(cmd.Context(), slug, args[0], body)
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			fmt.Printf("Source of %s set to %s.\n", args[0], args[1])
			return nil
		},
	}
}

// branchUpgradeCmd walks the operator through the next branch-vine rung
// transition. The current rung is derived from registered branches' roles and
// source_branch_name fields; the command performs only the next transition.
// Rung 2→3 deviates from the literal plan (re-roling existing dev to pr-target)
// because there is no set-role endpoint yet — instead it inserts a fresh
// pr-target staging branch that PRs land on before dev.
func branchUpgradeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "upgrade",
		Short: "Interactively walk this project up the branch-vine to the next rung",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			slug := c.SlugOrDie()
			resp, err := c.ListVersionBranchesWithResponse(cmd.Context(), slug)
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			var branches []dxclient.VersionBranchItem
			if resp.JSON200 != nil && resp.JSON200.Branches != nil {
				branches = *resp.JSON200.Branches
			}
			rung, info := classifyBranchRung(branches)
			reader := bufio.NewReader(os.Stdin)
			switch rung {
			case 0:
				return upgradeRung0to1(cmd.Context(), c, slug, reader)
			case 1:
				return upgradeRung1to2(cmd.Context(), c, slug, reader, info)
			case 2:
				return upgradeRung2to3(cmd.Context(), c, slug, reader, info)
			default:
				fmt.Println("Project is at rung 3 — no further automated transitions available.")
				return nil
			}
		},
	}
}

type branchRungInfo struct {
	rollingRelease *dxclient.VersionBranchItem
	dev            *dxclient.VersionBranchItem
	prTarget       *dxclient.VersionBranchItem
	releaseFeeder  *dxclient.VersionBranchItem // any branch whose source points to dev
}

func classifyBranchRung(branches []dxclient.VersionBranchItem) (int, branchRungInfo) {
	info := branchRungInfo{}
	if len(branches) == 0 {
		return 0, info
	}
	for i := range branches {
		b := branches[i]
		if b.Role == nil {
			continue
		}
		switch *b.Role {
		case "rolling-release":
			info.rollingRelease = &branches[i]
		case "dev":
			info.dev = &branches[i]
		case "pr-target":
			info.prTarget = &branches[i]
		}
	}
	if info.prTarget != nil {
		return 3, info
	}
	if info.dev != nil {
		for i := range branches {
			b := branches[i]
			if b.Name == info.dev.Name {
				continue
			}
			if b.SourceBranchName != nil && *b.SourceBranchName == info.dev.Name {
				info.releaseFeeder = &branches[i]
				return 2, info
			}
		}
	}
	return 1, info
}

func confirmYes(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "y" || s == "yes"
}

func roleLabel(b *dxclient.VersionBranchItem) string {
	if b.Role != nil && *b.Role != "" {
		return *b.Role
	}
	return b.Type
}

func gitBranchFrom(name, from string) (created bool, err error) {
	out, gerr := exec.Command("git", "branch", name, from).CombinedOutput()
	if gerr == nil {
		return true, nil
	}
	if strings.Contains(string(out), "already exists") {
		return false, nil
	}
	return false, fmt.Errorf("git branch %s %s: %s", name, from, strings.TrimSpace(string(out)))
}

func upgradeRung0to1(ctx context.Context, c *cli.Client, slug string, reader *bufio.Reader) error {
	branch := currentGitBranch()
	if branch == "" {
		return fmt.Errorf("could not determine current git branch (detached HEAD?)")
	}
	fmt.Println("Current rung: 0 (no branches registered)")
	fmt.Printf("Register current branch '%s' as rolling-release? [y/N] ", branch)
	answer, _ := reader.ReadString('\n')
	if !confirmYes(answer) {
		fmt.Println("Aborted.")
		return nil
	}
	role := "rolling-release"
	body := dxclient.CreateVersionBranchRequest{Name: branch, Role: &role}
	resp, err := c.CreateVersionBranchWithResponse(ctx, slug, body)
	if err != nil {
		return err
	}
	if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
		return err
	}
	fmt.Printf("Rung 1 complete: '%s' registered with role=rolling-release.\n", branch)
	return nil
}

func upgradeRung1to2(ctx context.Context, c *cli.Client, slug string, reader *bufio.Reader, info branchRungInfo) error {
	rel := info.rollingRelease
	if rel == nil {
		rel = info.dev
	}
	if rel == nil {
		return fmt.Errorf("rung 1 detected but no rolling-release or dev branch found; inspect 'dx branch list'")
	}
	fmt.Printf("Current rung: 1 (%s on '%s')\n", roleLabel(rel), rel.Name)
	fmt.Println("Rung 2 introduces a dev integration branch:")
	fmt.Printf("  1. Create git branch 'dev' from %s\n", rel.Name)
	fmt.Println("  2. Register 'dev' with role=dev")
	fmt.Printf("  3. Set %s's source to 'dev' (releases track dev)\n", rel.Name)
	fmt.Print("Proceed? [y/N] ")
	answer, _ := reader.ReadString('\n')
	if !confirmYes(answer) {
		fmt.Println("Aborted.")
		return nil
	}
	created, err := gitBranchFrom("dev", rel.Name)
	if err != nil {
		return err
	}
	if created {
		fmt.Printf("  - created git branch 'dev' from %s\n", rel.Name)
	} else {
		fmt.Println("  - git branch 'dev' already exists; reusing")
	}
	if info.dev == nil {
		role := "dev"
		cresp, err := c.CreateVersionBranchWithResponse(ctx, slug, dxclient.CreateVersionBranchRequest{Name: "dev", Role: &role})
		if err != nil {
			return err
		}
		if err := c.CheckStatus(cresp.StatusCode(), cresp.Body); err != nil {
			return err
		}
		fmt.Println("  - registered 'dev' with role=dev")
	} else {
		fmt.Println("  - 'dev' already registered with role=dev; reusing")
	}
	sresp, err := c.SetVersionBranchSourceWithResponse(ctx, slug, rel.Name, dxclient.SetVersionBranchSourceRequest{SourceBranchName: "dev"})
	if err != nil {
		return err
	}
	if err := c.CheckStatus(sresp.StatusCode(), sresp.Body); err != nil {
		return err
	}
	fmt.Printf("  - set source of '%s' to 'dev'\n", rel.Name)
	fmt.Println("Rung 2 complete.")
	return nil
}

func upgradeRung2to3(ctx context.Context, c *cli.Client, slug string, reader *bufio.Reader, info branchRungInfo) error {
	dev := info.dev
	if dev == nil {
		return fmt.Errorf("rung 2 detected but no dev-role branch found; inspect 'dx branch list'")
	}
	feederName := ""
	if info.releaseFeeder != nil {
		feederName = info.releaseFeeder.Name
	}
	fmt.Printf("Current rung: 2 (dev on '%s'", dev.Name)
	if feederName != "" {
		fmt.Printf(", '%s' tracks dev", feederName)
	}
	fmt.Println(")")
	fmt.Printf("Rung 3 adds a pr-target staging branch: PRs land there and merge into '%s'.\n", dev.Name)
	fmt.Print("Name for new pr-target branch [pr-target]: ")
	answer, _ := reader.ReadString('\n')
	name := strings.TrimSpace(answer)
	if name == "" {
		name = "pr-target"
	}
	fmt.Println("Plan:")
	fmt.Printf("  1. Create git branch '%s' from %s\n", name, dev.Name)
	fmt.Printf("  2. Register '%s' with role=pr-target, source=%s\n", name, dev.Name)
	fmt.Print("Proceed? [y/N] ")
	answer, _ = reader.ReadString('\n')
	if !confirmYes(answer) {
		fmt.Println("Aborted.")
		return nil
	}
	created, err := gitBranchFrom(name, dev.Name)
	if err != nil {
		return err
	}
	if created {
		fmt.Printf("  - created git branch '%s' from %s\n", name, dev.Name)
	} else {
		fmt.Printf("  - git branch '%s' already exists; reusing\n", name)
	}
	role := "pr-target"
	src := dev.Name
	cresp, err := c.CreateVersionBranchWithResponse(ctx, slug, dxclient.CreateVersionBranchRequest{Name: name, Role: &role, SourceBranchName: &src})
	if err != nil {
		return err
	}
	if err := c.CheckStatus(cresp.StatusCode(), cresp.Body); err != nil {
		return err
	}
	fmt.Printf("  - registered '%s' with role=pr-target, source=%s\n", name, dev.Name)
	fmt.Println("Rung 3 complete.")
	return nil
}

func branchEolCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "eol <name>",
		Short: "Mark a version branch end-of-life",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			slug := c.SlugOrDie()
			resp, err := c.MarkVersionBranchEolWithResponse(cmd.Context(), slug, args[0])
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			fmt.Printf("Branch %s marked end-of-life.\n", args[0])
			return nil
		},
	}
}
