package project

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/dxclient"
)

func BranchCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "branch", Short: "Version branch management"}
	cmd.AddCommand(branchCutCmd(), branchListCmd(), branchShowCmd(), branchEolCmd())
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
			for _, b := range branches {
				if b.Semver != nil && *b.Semver != "" {
					hasSemver = true
					break
				}
			}
			if hasSemver {
				fmt.Printf("%-20s %-8s %-8s %s\n", "NAME", "TYPE", "STATUS", "SEMVER")
				for _, b := range branches {
					semver := ""
					if b.Semver != nil {
						semver = *b.Semver
					}
					fmt.Printf("%-20s %-8s %-8s %s\n", b.Name, b.Type, b.Status, semver)
				}
			} else {
				fmt.Printf("%-20s %-8s %s\n", "NAME", "TYPE", "STATUS")
				for _, b := range branches {
					fmt.Printf("%-20s %-8s %s\n", b.Name, b.Type, b.Status)
				}
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
			semver := ""
			if b.Semver != nil {
				semver = *b.Semver
			}
			fmt.Printf("Name:    %s\n", b.Name)
			fmt.Printf("Type:    %s\n", b.Type)
			fmt.Printf("Status:  %s\n", b.Status)
			if semver != "" {
				fmt.Printf("Semver:  %s\n", semver)
			}
			fmt.Printf("Created: %s\n", b.CreatedAt)
			fmt.Printf("Open issues: %d / Resolved: %d\n", b.OpenCount, b.ResolvedCount)
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
