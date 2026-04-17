package project

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/dxclient"
)

func EnvCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "env", Short: "Environment management"}
	cmd.AddCommand(
		envListCmd(),
		envAddCmd(),
		envShowCmd(),
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
				fmt.Printf("%-20s  %s\n", e.Name, e.Url)
			}
			return nil
		},
	}
}

func envAddCmd() *cobra.Command {
	var url string
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
			fmt.Printf("%d  %s  %s\n", resp.JSON200.Id, resp.JSON200.Name, resp.JSON200.Url)
			return nil
		},
	}
	cmd.Flags().StringVar(&url, "url", "", "environment URL")
	return cmd
}

func envShowCmd() *cobra.Command {
	return &cobra.Command{
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
			fmt.Printf("Name:       %s\n", e.Name)
			fmt.Printf("URL:        %s\n", e.Url)
			fmt.Printf("Created:    %s\n", e.CreatedAt)
			fmt.Printf("Deployed:   %s\n", e.DeployedAt)
			fmt.Printf("Branch:     %s\n", e.CurrentBuildBranch)
			fmt.Printf("SHA:        %s\n", e.CurrentBuildSha)

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
}

func envEditCmd() *cobra.Command {
	var url string
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
