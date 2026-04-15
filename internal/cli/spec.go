package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func SpecCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "spec",
		Short: "Feature spec management (BDD statements)",
	}
	cmd.AddCommand(specAddCmd(), specListCmd(), specLinkCmd(), specUnlinkCmd(), specDeferCmd(), specUndeferCmd())
	return cmd
}

func specAddCmd() *cobra.Command {
	var feature, text, kind string
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a spec statement to a feature",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := mustClient()
			var ok struct {
				OK bool `json:"ok"`
			}
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
					tag := ""
					if s.Deferred {
						tag = " (deferred)"
					}
					fmt.Printf("%-4d [%-14s]  %s%s\n", s.ID, s.Kind, s.Description, tag)
				}
				return nil
			}
			return fmt.Errorf("feature not found: %s", args[0])
		},
	}
}

func specLinkCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "link <spec-id> <test-id>",
		Short: "Link a test to a spec",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			specID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid spec-id: %s", args[0])
			}
			testID, err := strconv.Atoi(args[1])
			if err != nil {
				return fmt.Errorf("invalid test-id: %s", args[1])
			}
			c := mustClient()
			var ok struct{ OK bool }
			if err := c.post("/api/dx/specs/link-test", map[string]any{
				"spec_id": specID,
				"test_id": testID,
			}, &ok); err != nil {
				return err
			}
			fmt.Printf("linked spec %d ↔ test %d\n", specID, testID)
			return nil
		},
	}
}

func specDeferCmd() *cobra.Command {
	var reason string
	cmd := &cobra.Command{
		Use:   "defer <spec-id>",
		Short: "Mark a spec as deferred (skipped by solo tech:test-ref)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			specID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid spec-id: %s", args[0])
			}
			c := mustClient()
			var ok struct{ OK bool }
			if err := c.post("/api/dx/specs/defer", map[string]any{
				"spec_id": specID,
				"reason":  reason,
			}, &ok); err != nil {
				return err
			}
			fmt.Printf("deferred spec %d\n", specID)
			return nil
		},
	}
	cmd.Flags().StringVar(&reason, "reason", "", "reason for deferring")
	return cmd
}

func specUndeferCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "undefer <spec-id>",
		Short: "Un-defer a spec (re-enable solo tech:test-ref checks)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			specID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid spec-id: %s", args[0])
			}
			c := mustClient()
			var ok struct{ OK bool }
			if err := c.post("/api/dx/specs/undefer", map[string]any{
				"spec_id": specID,
			}, &ok); err != nil {
				return err
			}
			fmt.Printf("un-deferred spec %d\n", specID)
			return nil
		},
	}
}

func specUnlinkCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "unlink <spec-id> <test-id>",
		Short: "Unlink a test from a spec",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			specID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid spec-id: %s", args[0])
			}
			testID, err := strconv.Atoi(args[1])
			if err != nil {
				return fmt.Errorf("invalid test-id: %s", args[1])
			}
			c := mustClient()
			var ok struct{ OK bool }
			if err := c.post("/api/dx/specs/unlink-test", map[string]any{
				"spec_id": specID,
				"test_id": testID,
			}, &ok); err != nil {
				return err
			}
			fmt.Printf("unlinked spec %d ↔ test %d\n", specID, testID)
			return nil
		},
	}
}
