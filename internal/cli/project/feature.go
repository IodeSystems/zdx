package project

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"

	"github.com/iodesystems/zdx-go/internal/cli/clitypes"
)

func FeatureCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "feature", Short: "Feature management"}
	cmd.AddCommand(
		featureListCmd(),
		featureAddCmd(),
		featureShowCmd(),
		featureSetCmd(),
		featureReviewCmd(),
	)
	return cmd
}

func featureListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List features",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			var resp struct {
				Features []clitypes.FeatureItem `json:"features"`
			}
			if err := c.Get("/api/features", cli.QuerySlug(c), &resp); err != nil {
				return err
			}
			if len(resp.Features) == 0 {
				fmt.Println("no features")
				return nil
			}
			for _, f := range resp.Features {
				comp := f.Component
				if comp == "" {
					comp = "-"
				}
				cat := f.Category
				if cat == "" {
					cat = "-"
				}
				fmt.Printf("%-28s [%-5s]  %-16s  %s\n", f.Name, comp, cat, f.Description)
			}
			return nil
		},
	}
}

func featureAddCmd() *cobra.Command {
	var desc string
	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Create or update a feature",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			var f clitypes.FeatureItem
			if err := c.Post("/api/feature", map[string]any{
				"slug":        c.SlugOrDie(),
				"name":        args[0],
				"description": desc,
			}, &f); err != nil {
				return err
			}
			fmt.Printf("%d  %s\n", f.ID, f.Name)
			return nil
		},
	}
	cmd.Flags().StringVar(&desc, "desc", "", "feature description")
	return cmd
}

func featureShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <name>",
		Short: "Show feature detail with specs",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			// Fetch the full list and find by name (no single-feature GET endpoint yet).
			var resp struct {
				Features []clitypes.FeatureItem `json:"features"`
			}
			if err := c.Get("/api/features", cli.QuerySlug(c), &resp); err != nil {
				return err
			}
			var f *clitypes.FeatureItem
			for i := range resp.Features {
				if strings.EqualFold(resp.Features[i].Name, args[0]) {
					f = &resp.Features[i]
					break
				}
			}
			if f == nil {
				return fmt.Errorf("feature not found: %s", args[0])
			}
			cli.PrintFeatureItem(*f)
			return nil
		},
	}
}

func featureSetCmd() *cobra.Command {
	var what, why, doneWhen, component, description, category string
	cmd := &cobra.Command{
		Use:   "set <name>",
		Short: "Set fields on a feature",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			slug := c.SlugOrDie()
			name := args[0]

			fields := map[string]string{
				"what":        what,
				"why":         why,
				"done_when":   doneWhen,
				"component":   component,
				"description": description,
				"category":    category,
			}
			changed := false
			for field, value := range fields {
				if !cmd.Flags().Changed(fieldFlag(field)) {
					continue
				}
				if err := c.Post("/api/dx/features/field", map[string]any{
					"slug":    slug,
					"feature": name,
					"field":   field,
					"value":   value,
				}, nil); err != nil {
					return fmt.Errorf("set %s: %w", field, err)
				}
				changed = true
			}
			if !changed {
				return fmt.Errorf("no fields specified — use --what, --why, --done-when, --component, --category, or --desc")
			}
			fmt.Printf("%s updated\n", name)
			return nil
		},
	}
	cmd.Flags().StringVar(&what, "what", "", "what this feature does")
	cmd.Flags().StringVar(&why, "why", "", "why this feature matters")
	cmd.Flags().StringVar(&doneWhen, "done-when", "", "definition of done")
	cmd.Flags().StringVar(&component, "component", "", "component (e.g. ui, api, cli)")
	cmd.Flags().StringVar(&description, "desc", "", "short description")
	cmd.Flags().StringVar(&category, "category", "", "category (e.g. Testing, Dx, UI)")
	return cmd
}

func featureReviewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "review <name>",
		Short: "Mark feature as owner-reviewed (resets cool-down timer)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			var ok struct {
				OK bool `json:"ok"`
			}
			if err := c.Post("/api/dx/feature/review", map[string]any{
				"slug":    c.SlugOrDie(),
				"feature": args[0],
			}, &ok); err != nil {
				return err
			}
			fmt.Printf("%s reviewed\n", args[0])
			return nil
		},
	}
}

// fieldFlag maps a snake_case field name to its cobra flag name.
func fieldFlag(field string) string {
	switch field {
	case "done_when":
		return "done-when"
	case "description":
		return "desc"
	default:
		return field
	}
}
