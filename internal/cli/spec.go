package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func SpecCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "spec",
		Short: "Feature spec management (BDD statements)",
	}
	cmd.AddCommand(specAddCmd(), specListCmd())
	return cmd
}

func specAddCmd() *cobra.Command {
	var feature, text, kind string
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a spec statement to a feature",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := mustClient()
			var ok struct{ OK bool `json:"ok"` }
			// The update-specs endpoint uses field=kind, value=description.
			if err := c.post("/api/dx/specs/update", map[string]any{
				"slug":    c.SlugOrDie(),
				"feature": feature,
				"field":   kind,
				"value":   text,
			}, &ok); err != nil {
				return err
			}
			fmt.Printf("[%s] %s → %s\n", kind, feature, text)
			return nil
		},
	}
	cmd.Flags().StringVar(&feature, "feature", "", "feature name")
	cmd.Flags().StringVar(&text, "text", "", "spec statement (BDD style: 'given X, when Y, then Z')")
	cmd.Flags().StringVar(&kind, "kind", "must", "requirement tier: must | should | nice-to-have")
	cmd.MarkFlagRequired("feature")
	cmd.MarkFlagRequired("text")
	return cmd
}

func specListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <feature>",
		Short: "List specs for a feature",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := mustClient()
			var resp struct {
				Features []featureItem `json:"features"`
			}
			if err := c.get("/api/features", querySlug(c), &resp); err != nil {
				return err
			}
			for _, f := range resp.Features {
				if !strings.EqualFold(f.Name, args[0]) {
					continue
				}
				if len(f.Specs) == 0 {
					fmt.Printf("%s: no specs\n", f.Name)
					return nil
				}
				for _, s := range f.Specs {
					fmt.Printf("%-4d [%-14s]  %s\n", s.ID, s.Kind, s.Description)
				}
				return nil
			}
			return fmt.Errorf("feature not found: %s", args[0])
		},
	}
}
