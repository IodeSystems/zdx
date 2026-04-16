package project

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"

	"github.com/iodesystems/zdx-go/internal/cli/clitypes"
)

func ThemeCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "theme", Short: "Theme / roadmap management"}
	cmd.AddCommand(themeListCmd(), themeAddCmd(), themeStatusCmd())
	return cmd
}

func themeListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List themes",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			var resp struct {
				Themes []clitypes.ThemeItem `json:"themes"`
			}
			if err := c.Get("/api/dx/themes", cli.QuerySlug(c), &resp); err != nil {
				return err
			}
			if len(resp.Themes) == 0 {
				fmt.Println("no themes")
				return nil
			}
			for _, t := range resp.Themes {
				blockers := ""
				if t.Blockers != "" {
					blockers = "  blocked:" + t.Blockers
				}
				fmt.Printf("TH-%-3d p%-2d  %-12s  %-24s  %s%s\n",
					t.ID, t.Priority, t.Status, t.Name, t.Description, blockers)
			}
			return nil
		},
	}
}

func themeAddCmd() *cobra.Command {
	var desc, blockers string
	var priority int
	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a theme",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			var t clitypes.ThemeItem
			if err := c.Post("/api/dx/themes/add", map[string]any{
				"slug":        c.SlugOrDie(),
				"name":        args[0],
				"description": desc,
				"priority":    int32(priority),
				"blockers":    blockers,
			}, &t); err != nil {
				return err
			}
			fmt.Printf("TH-%d  %s\n", t.ID, t.Name)
			return nil
		},
	}
	cmd.Flags().StringVar(&desc, "desc", "", "theme description")
	cmd.Flags().StringVar(&blockers, "blockers", "", "blocking issue IDs (comma-separated)")
	cmd.Flags().IntVar(&priority, "priority", 10, "priority (lower = higher priority)")
	return cmd
}

func themeStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status <TH-N|name> <status>",
		Short: "Set theme status (active|done|dropped)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			var ok struct {
				OK bool `json:"ok"`
			}
			if err := c.Post("/api/dx/themes/status", map[string]any{
				"slug":   c.SlugOrDie(),
				"theme":  args[0],
				"status": args[1],
			}, &ok); err != nil {
				return err
			}
			fmt.Printf("%s → %s\n", args[0], args[1])
			return nil
		},
	}
}

// themeIDStr formats a theme ID.
func themeIDStr(n int32) string { return fmt.Sprintf("TH-%d", n) }
