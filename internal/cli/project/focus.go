package project

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/dxclient"
)

func FocusCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "focus", Short: "Focus / prioritization management"}
	cmd.AddCommand(focusListCmd(), focusAddCmd(), focusStatusCmd())
	return cmd
}

// ThemeCmd is a legacy alias for FocusCmd.
func ThemeCmd() *cobra.Command {
	cmd := FocusCmd()
	cmd.Use = "theme"
	cmd.Short = "Theme management (legacy alias for focus)"
	return cmd
}

func focusListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List focuses",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			resp, err := c.ListFocusesWithResponse(cmd.Context(), &dxclient.ListFocusesParams{
				Slug: c.SlugOrDie(),
			})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil || resp.JSON200.Focuses == nil || len(*resp.JSON200.Focuses) == 0 {
				fmt.Println("no focuses")
				return nil
			}
			for _, t := range *resp.JSON200.Focuses {
				blockers := ""
				if t.Blockers != "" {
					blockers = "  blocked:" + t.Blockers
				}
				fmt.Printf("FO-%-3d p%-2d  %-12s  %-24s  %s%s\n",
					t.Id, t.Priority, t.Status, t.Name, t.Description, blockers)
			}
			return nil
		},
	}
}

func focusAddCmd() *cobra.Command {
	var desc, blockers string
	var priority int
	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a focus",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			resp, err := c.AddFocusWithResponse(cmd.Context(), dxclient.AddFocusRequest{
				Slug:        c.SlugOrDie(),
				Name:        args[0],
				Description: desc,
				Priority:    int32(priority),
				Blockers:    blockers,
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
			fmt.Printf("FO-%d  %s\n", resp.JSON200.Id, resp.JSON200.Name)
			return nil
		},
	}
	cmd.Flags().StringVar(&desc, "desc", "", "focus description")
	cmd.Flags().StringVar(&blockers, "blockers", "", "blocking issue IDs (comma-separated)")
	cmd.Flags().IntVar(&priority, "priority", 10, "priority (lower = higher priority)")
	return cmd
}

func focusStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status <FO-N|name> <status>",
		Short: "Set focus status (active|done|dropped)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			resp, err := c.SetFocusStatusWithResponse(cmd.Context(), dxclient.SetFocusStatusRequest{
				Slug:   c.SlugOrDie(),
				Focus:  args[0],
				Status: args[1],
			})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			fmt.Printf("%s → %s\n", args[0], args[1])
			return nil
		},
	}
}

func focusIDStr(n int32) string { return fmt.Sprintf("FO-%d", n) }
