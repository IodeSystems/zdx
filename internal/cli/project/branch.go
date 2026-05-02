package project

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/dxclient"
)

func BranchCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "branch", Short: "Version branch management"}
	cmd.AddCommand(branchCutCmd(), branchListCmd(), branchShowCmd(), branchEolCmd(), branchSetSourceCmd())
	return cmd
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
