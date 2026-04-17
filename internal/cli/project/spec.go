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
	cmd.AddCommand(specAddCmd(), specListCmd(), specLinkCmd(), specUnlinkCmd(), specDeferCmd(), specUndeferCmd())
	return cmd
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
	return &cobra.Command{
		Use:   "list <feature>",
		Short: "List specs for a feature",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
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
			c := cli.MustClient()
			resp, err := c.LinkSpecTestWithResponse(cmd.Context(), dxclient.LinkSpecTestRequest{
				SpecId: int32(specID),
				TestId: int32(testID),
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
			c := cli.MustClient()
			resp, err := c.DeferSpecWithResponse(cmd.Context(), dxclient.DeferSpecRequest{
				SpecId: int32(specID),
				Reason: reason,
			})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
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
