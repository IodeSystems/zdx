package project

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/dxclient"
)

func SpecCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "spec",
		Short: "Feature spec management (BDD statements)",
	}
	cmd.AddCommand(specAddCmd(), specShowCmd(), specListCmd(), specLinkCmd(), specUnlinkCmd(), specDeferCmd(), specUndeferCmd(), specMoveCmd(), specRmCmd())
	return cmd
}

func specMoveCmd() *cobra.Command {
	var feature string
	cmd := &cobra.Command{
		Use:   "move <spec-id>",
		Short: "Move a spec to a different feature (for decomposing over-specced features)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			specID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid spec-id: %s", args[0])
			}
			c := cli.MustClient()
			resp, err := c.MoveSpecWithResponse(cmd.Context(), dxclient.MoveSpecRequest{
				Slug:    c.SlugOrDie(),
				SpecId:  int32(specID),
				Feature: feature,
			})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			fmt.Printf("moved spec %d → %s\n", specID, feature)
			return nil
		},
	}
	cmd.Flags().StringVar(&feature, "feature", "", "target feature name")
	cmd.MarkFlagRequired("feature")
	return cmd
}

func specRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "rm <spec-id>",
		Aliases: []string{"remove", "delete"},
		Short:   "Delete a spec (cascades to spec↔test and spec↔issue links)",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			specID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid spec-id: %s", args[0])
			}
			c := cli.MustClient()
			resp, err := c.DeleteSpecWithResponse(cmd.Context(), dxclient.DeleteSpecRequest{
				SpecId: int32(specID),
			})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			fmt.Printf("deleted spec %d\n", specID)
			return nil
		},
	}
}

func specAddCmd() *cobra.Command {
	var feature, text, kind string
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a spec statement to a feature",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			// The update-specs endpoint uses field=kind, value=description.
			resp, err := c.UpdateSpecsWithResponse(cmd.Context(), dxclient.UpdateSpecsRequest{
				Slug:    c.SlugOrDie(),
				Feature: feature,
				Field:   kind,
				Value:   text,
			})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
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
	var deferred bool
	cmd := &cobra.Command{
		Use:   "list [feature]",
		Short: "List specs for a feature, or (with --deferred) all specs deferred by open issues",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			ctx := cmd.Context()

			if deferred {
				resp, err := c.ListDeferredSpecsWithResponse(ctx, &dxclient.ListDeferredSpecsParams{Slug: c.SlugOrDie()})
				if err != nil {
					return err
				}
				if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
					return err
				}
				if resp.JSON200 == nil || resp.JSON200.Specs == nil || len(*resp.JSON200.Specs) == 0 {
					fmt.Println("no deferred specs")
					return nil
				}
				for _, s := range *resp.JSON200.Specs {
					fmt.Printf("%-4d [%-14s]  %s\n  feature: %s\n", s.Id, s.Kind, s.Description, s.FeatureName)
					if s.Blockers != nil {
						for _, b := range *s.Blockers {
							line := fmt.Sprintf("  - %s (%s) — %s", b.IssueId, b.IssueStatus, b.IssueTitle)
							if b.Note != "" {
								line += "  [" + b.Note + "]"
							}
							fmt.Println(line)
						}
					}
				}
				return nil
			}

			if len(args) == 0 {
				return fmt.Errorf("feature name required (or pass --deferred to list deferred specs)")
			}
			resp, err := c.ListFeaturesWithResponse(ctx, &dxclient.ListFeaturesParams{Slug: c.SlugOrDie()})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil || resp.JSON200.Features == nil {
				return fmt.Errorf("feature not found: %s", args[0])
			}
			for _, f := range *resp.JSON200.Features {
				if !strings.EqualFold(f.Name, args[0]) {
					continue
				}
				if f.Specs == nil || len(*f.Specs) == 0 {
					fmt.Printf("%s: no specs\n", f.Name)
					return nil
				}
				for _, s := range *f.Specs {
					tag := ""
					if s.Deferred {
						tag = " (deferred)"
					}
					fmt.Printf("%-4d [%-14s]  %s%s\n", s.Id, s.Kind, s.Description, tag)
				}
				return nil
			}
			return fmt.Errorf("feature not found: %s", args[0])
		},
	}
	cmd.Flags().BoolVar(&deferred, "deferred", false, "list all specs deferred by open issues (feature arg ignored)")
	return cmd
}

func specLinkCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "link <spec-id> <test-id-or-name>",
		Short: "Link a test to a spec (test-id-or-name: numeric id or exact test name)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			specID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid spec-id: %s", args[0])
			}
			c := cli.MustClient()
			testID, err := cli.ResolveTestID(cmd.Context(), c, args[1])
			if err != nil {
				return err
			}
			resp, err := c.LinkSpecTestWithResponse(cmd.Context(), dxclient.LinkSpecTestRequest{
				SpecId: int32(specID),
				TestId: testID,
			})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			fmt.Printf("linked spec %d ↔ test %d\n", specID, testID)
			return nil
		},
	}
}

func specDeferCmd() *cobra.Command {
	var (
		reason    string
		blockedBy []string
		remove    string
		note      string
	)
	cmd := &cobra.Command{
		Use:   "defer <spec-id>",
		Short: "Defer a spec to one or more blocking issues (preferred) or a free-text reason (legacy)",
		Long: `Defer a spec:
  dx spec defer <id> --blocked-by=IS-N [--blocked-by=IS-M] [--note="..."]
  dx spec defer <id> --remove=IS-N
  dx spec defer <id> --reason="..."   (legacy: sets the boolean deferred flag)`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			specID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid spec-id: %s", args[0])
			}
			c := cli.MustClient()
			ctx := cmd.Context()

			usedNew := len(blockedBy) > 0 || remove != ""
			usedLegacy := reason != ""
			if usedNew && usedLegacy {
				return fmt.Errorf("--reason cannot be combined with --blocked-by/--remove")
			}
			if !usedNew && !usedLegacy {
				return fmt.Errorf("specify --blocked-by=IS-N (or --remove=IS-N) to defer against an issue; --reason is the legacy boolean-flag path")
			}

			if remove != "" {
				resp, err := c.RemoveSpecDeferralWithResponse(ctx, dxclient.RemoveSpecDeferralRequest{
					SpecId:  int32(specID),
					IssueId: remove,
				})
				if err != nil {
					return err
				}
				if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
					return err
				}
				fmt.Printf("removed deferral: spec %d ↮ %s\n", specID, remove)
			}

			slug := c.SlugOrDie()
			for _, issueID := range blockedBy {
				n := note
				resp, err := c.AddSpecDeferralWithResponse(ctx, dxclient.AddSpecDeferralRequest{
					Slug:    slug,
					SpecId:  int32(specID),
					IssueId: issueID,
					Note:    &n,
				})
				if err != nil {
					return err
				}
				if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
					return err
				}
				fmt.Printf("deferred spec %d ← %s\n", specID, issueID)
			}

			if usedLegacy {
				resp, err := c.DeferSpecWithResponse(ctx, dxclient.DeferSpecRequest{
					SpecId: int32(specID),
					Reason: reason,
				})
				if err != nil {
					return err
				}
				if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
					return err
				}
				fmt.Printf("deferred spec %d (legacy reason: %q)\n", specID, reason)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&reason, "reason", "", "legacy: free-text reason (sets boolean deferred flag)")
	cmd.Flags().StringArrayVar(&blockedBy, "blocked-by", nil, "issue IDs blocking this spec (repeatable)")
	cmd.Flags().StringVar(&remove, "remove", "", "remove a deferral by issue ID")
	cmd.Flags().StringVar(&note, "note", "", "optional note attached to each new deferral")
	return cmd
}

func specShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <spec-id>",
		Short: "Show a spec, its linked issues, and any issue-backed deferrals",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			specID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid spec-id: %s", args[0])
			}
			c := cli.MustClient()
			ctx := cmd.Context()

			specResp, err := c.GetSpecWithResponse(ctx, &dxclient.GetSpecParams{SpecId: int32(specID)})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(specResp.StatusCode(), specResp.Body); err != nil {
				return err
			}
			if specResp.JSON200 == nil {
				return fmt.Errorf("spec %d not found", specID)
			}
			s := specResp.JSON200.Spec
			fmt.Printf("Spec %d  [%s / %s]\n", s.Id, s.Kind, s.ConcernType)
			fmt.Printf("  %s\n", s.Description)
			if s.Deferred {
				tag := "(legacy deferred"
				if s.DeferredReason != "" {
					tag += ": " + s.DeferredReason
				}
				tag += ")"
				fmt.Printf("  %s\n", tag)
			}

			defResp, err := c.ListSpecDeferralsWithResponse(ctx, &dxclient.ListSpecDeferralsParams{SpecId: int32(specID)})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(defResp.StatusCode(), defResp.Body); err != nil {
				return err
			}
			if defResp.JSON200 != nil && defResp.JSON200.Deferrals != nil && len(*defResp.JSON200.Deferrals) > 0 {
				fmt.Println("\nBlocked by:")
				for _, d := range *defResp.JSON200.Deferrals {
					line := fmt.Sprintf("  %s (%s) — %s", d.IssueId, d.IssueStatus, d.IssueTitle)
					if d.Note != "" {
						line += "  [" + d.Note + "]"
					}
					fmt.Println(line)
				}
			}

			if specResp.JSON200.Issues != nil && len(*specResp.JSON200.Issues) > 0 {
				fmt.Println("\nLinked issues:")
				for _, is := range *specResp.JSON200.Issues {
					fmt.Printf("  %s (%s) — %s\n", is.IssueId, is.Status, is.Title)
				}
			}
			return nil
		},
	}
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
			c := cli.MustClient()
			resp, err := c.UndeferSpecWithResponse(cmd.Context(), dxclient.UndeferSpecRequest{
				SpecId: int32(specID),
			})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
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
			c := cli.MustClient()
			resp, err := c.UnlinkSpecTestWithResponse(cmd.Context(), dxclient.UnlinkSpecTestRequest{
				SpecId: int32(specID),
				TestId: int32(testID),
			})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			fmt.Printf("unlinked spec %d ↔ test %d\n", specID, testID)
			return nil
		},
	}
}
