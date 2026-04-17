package project

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/dxclient"
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
			resp, err := c.ListFeaturesWithResponse(cmd.Context(), &dxclient.ListFeaturesParams{Slug: c.SlugOrDie()})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil || resp.JSON200.Features == nil || len(*resp.JSON200.Features) == 0 {
				fmt.Println("no features")
				return nil
			}
			for _, f := range *resp.JSON200.Features {
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
			resp, err := c.UpsertFeatureWithResponse(cmd.Context(), dxclient.UpsertFeatureRequest{
				Slug:        c.SlugOrDie(),
				Name:        args[0],
				Description: desc,
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
			fmt.Printf("%d  %s\n", resp.JSON200.Id, resp.JSON200.Name)
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
			resp, err := c.ListFeaturesWithResponse(cmd.Context(), &dxclient.ListFeaturesParams{Slug: c.SlugOrDie()})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil || resp.JSON200.Features == nil {
				return fmt.Errorf("feature not found: %s", args[0])
			}
			var found *dxclient.FeatureItem
			feats := *resp.JSON200.Features
			for i := range feats {
				if strings.EqualFold(feats[i].Name, args[0]) {
					found = &feats[i]
					break
				}
			}
			if found == nil {
				return fmt.Errorf("feature not found: %s", args[0])
			}
			cli.PrintFeatureItem(cli.FeatureToCli(*found))
			return nil
		},
	}
}

func featureSetCmd() *cobra.Command {
	var what, why, doneWhen, component, description, category string
	var kind, metricName, metricUnit, baselineValue, targetValue, graphURL string
	cmd := &cobra.Command{
		Use:   "set <name>",
		Short: "Set fields on a feature",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			slug := c.SlugOrDie()
			name := args[0]

			fields := map[string]string{
				"what":           what,
				"why":            why,
				"done_when":      doneWhen,
				"component":      component,
				"description":    description,
				"category":       category,
				"kind":           kind,
				"metric_name":    metricName,
				"metric_unit":    metricUnit,
				"baseline_value": baselineValue,
				"target_value":   targetValue,
				"graph_url":      graphURL,
			}
			changed := false
			for field, value := range fields {
				if !cmd.Flags().Changed(fieldFlag(field)) {
					continue
				}
				resp, err := c.SetFeatureFieldWithResponse(cmd.Context(), dxclient.SetFeatureFieldRequest{
					Slug:    slug,
					Feature: name,
					Field:   field,
					Value:   value,
				})
				if err != nil {
					return fmt.Errorf("set %s: %w", field, err)
				}
				if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
					return fmt.Errorf("set %s: %w", field, err)
				}
				changed = true
			}
			if !changed {
				return fmt.Errorf("no fields specified — use --what, --why, --done-when, --component, --category, --desc, --kind, --metric-name, etc.")
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
	cmd.Flags().StringVar(&kind, "kind", "", "feature kind: direct or multiplier")
	cmd.Flags().StringVar(&metricName, "metric-name", "", "measured metric name")
	cmd.Flags().StringVar(&metricUnit, "metric-unit", "", "metric unit")
	cmd.Flags().StringVar(&baselineValue, "baseline-value", "", "baseline measurement")
	cmd.Flags().StringVar(&targetValue, "target-value", "", "target measurement")
	cmd.Flags().StringVar(&graphURL, "graph-url", "", "URL to instrumentation graph/dashboard")
	return cmd
}

func featureReviewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "review <name>",
		Short: "Mark feature as owner-reviewed (resets cool-down timer)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			resp, err := c.MarkFeatureReviewedWithResponse(cmd.Context(), dxclient.MarkFeatureReviewedRequest{
				Slug:    c.SlugOrDie(),
				Feature: args[0],
			})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
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
	case "metric_name":
		return "metric-name"
	case "metric_unit":
		return "metric-unit"
	case "baseline_value":
		return "baseline-value"
	case "target_value":
		return "target-value"
	case "graph_url":
		return "graph-url"
	default:
		return field
	}
}
