package project

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/dxclient"
)

func ConcernCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "concern", Short: "Concern management"}
	cmd.AddCommand(concernAddCmd(), concernListCmd(), concernShowCmd(), concernLinkCmd(), concernUnlinkCmd())
	return cmd
}

func concernAddCmd() *cobra.Command {
	var name, description string
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a concern to the project",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			resp, err := c.CreateConcernWithResponse(cmd.Context(), dxclient.CreateConcernRequest{
				Slug:        c.SlugOrDie(),
				Name:        name,
				Description: description,
			})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			fmt.Printf("%s created\n", name)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "concern name")
	cmd.Flags().StringVar(&description, "description", "", "concern description")
	cmd.MarkFlagRequired("name")
	return cmd
}

func concernListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List concerns for the project",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			slug := c.SlugOrDie()
			resp, err := c.ListConcernsWithResponse(cmd.Context(), &dxclient.ListConcernsParams{Slug: slug})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil || resp.JSON200.Concerns == nil || len(*resp.JSON200.Concerns) == 0 {
				fmt.Println("no concerns")
				return nil
			}
			for _, cn := range *resp.JSON200.Concerns {
				fmt.Printf("%-28s  %s\n", cn.Name, cn.Description)
			}
			return nil
		},
	}
}

func concernShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <name>",
		Short: "Show a concern",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			resp, err := c.GetConcernWithResponse(cmd.Context(), &dxclient.GetConcernParams{
				Slug: c.SlugOrDie(),
				Name: args[0],
			})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil {
				return fmt.Errorf("concern not found: %s", args[0])
			}
			cn := resp.JSON200
			fmt.Printf("Name:        %s\n", cn.Name)
			fmt.Printf("Description: %s\n", cn.Description)
			return nil
		},
	}
}

func concernLinkCmd() *cobra.Command {
	var concern, feature, issue string
	var specID int32
	cmd := &cobra.Command{
		Use:   "link",
		Short: "Link a concern to a feature, spec, or issue",
		Long: `Link a concern to exactly one target:
  dx concern link --concern=Security --feature=auth
  dx concern link --concern=Security --spec=42
  dx concern link --concern=Security --issue=IS-5`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			slug := c.SlugOrDie()
			req := dxclient.LinkConcernRequest{
				Slug:        slug,
				ConcernName: concern,
			}
			var target string
			switch {
			case feature != "":
				req.Feature = &feature
				target = "feature " + feature
			case cmd.Flags().Changed("spec"):
				req.SpecId = &specID
				target = fmt.Sprintf("spec %d", specID)
			case issue != "":
				req.Issue = &issue
				target = "issue " + issue
			default:
				return fmt.Errorf("specify exactly one of --feature, --spec, or --issue")
			}
			resp, err := c.LinkConcernWithResponse(cmd.Context(), req)
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			fmt.Printf("linked %s → %s\n", concern, target)
			return nil
		},
	}
	cmd.Flags().StringVar(&concern, "concern", "", "concern name")
	cmd.Flags().StringVar(&feature, "feature", "", "feature name")
	cmd.Flags().Int32Var(&specID, "spec", 0, "spec ID")
	cmd.Flags().StringVar(&issue, "issue", "", "issue ID (e.g. IS-5)")
	cmd.MarkFlagRequired("concern")
	return cmd
}

func concernUnlinkCmd() *cobra.Command {
	var concern, feature, issue string
	var specID int32
	cmd := &cobra.Command{
		Use:   "unlink",
		Short: "Unlink a concern from a feature, spec, or issue",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			slug := c.SlugOrDie()
			req := dxclient.UnlinkConcernRequest{
				Slug:        slug,
				ConcernName: concern,
			}
			var target string
			switch {
			case feature != "":
				req.Feature = &feature
				target = "feature " + feature
			case cmd.Flags().Changed("spec"):
				req.SpecId = &specID
				target = fmt.Sprintf("spec %d", specID)
			case issue != "":
				req.Issue = &issue
				target = "issue " + issue
			default:
				return fmt.Errorf("specify exactly one of --feature, --spec, or --issue")
			}
			resp, err := c.UnlinkConcernWithResponse(cmd.Context(), req)
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			fmt.Printf("unlinked %s from %s\n", concern, target)
			return nil
		},
	}
	cmd.Flags().StringVar(&concern, "concern", "", "concern name")
	cmd.Flags().StringVar(&feature, "feature", "", "feature name")
	cmd.Flags().Int32Var(&specID, "spec", 0, "spec ID")
	cmd.Flags().StringVar(&issue, "issue", "", "issue ID (e.g. IS-5)")
	cmd.MarkFlagRequired("concern")
	return cmd
}
