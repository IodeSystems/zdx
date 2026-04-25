package project

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/dxclient"
)

func VisionCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "vision", Short: "Project vision / tagline"}
	cmd.AddCommand(visionShowCmd(), visionSetCmd())
	return cmd
}

func visionShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show project vision",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			resp, err := c.ListProjectsWithResponse(cmd.Context())
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			slug := c.SlugOrDie()
			if resp.JSON200 != nil && resp.JSON200.Projects != nil {
				for _, p := range *resp.JSON200.Projects {
					if p.Slug == slug {
						if p.Title != nil && *p.Title != "" {
							fmt.Println(*p.Title)
						} else {
							fmt.Println("(no title set)")
						}
						fmt.Println()
						if p.Description != nil && *p.Description != "" {
							fmt.Println(*p.Description)
						} else {
							fmt.Println("(no description set)")
						}
						return nil
					}
				}
			}
			return fmt.Errorf("project %s not found", slug)
		},
	}
}

func visionSetCmd() *cobra.Command {
	var title, description string
	cmd := &cobra.Command{
		Use:   "set",
		Short: "Set project vision",
		Long: `Set the project's guiding star — a short title and description that
frame all goals, features, and specs.

Title:       one-line tagline (what the project is)
Description: 1-3 sentences (who it's for, what it does, why it matters)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			resp, err := c.SetProjectVisionWithResponse(cmd.Context(), dxclient.SetProjectVisionRequest{
				Slug:        c.SlugOrDie(),
				Title:       title,
				Description: description,
			})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			fmt.Println("vision updated")
			return nil
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "one-line tagline")
	cmd.Flags().StringVar(&description, "desc", "", "description (who, what, why)")
	return cmd
}
