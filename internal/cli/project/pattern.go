package project

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/dxclient"
)

func PatternCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "pattern", Short: "Pattern library management"}
	cmd.AddCommand(patternListCmd(), patternAddCmd(), patternShowCmd(), patternDeleteCmd(), patternSearchCmd())
	return cmd
}

func patternListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List patterns",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			resp, err := c.ListPatternsWithResponse(cmd.Context(), &dxclient.ListPatternsParams{Slug: c.SlugOrDie()})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil || resp.JSON200.Patterns == nil || len(*resp.JSON200.Patterns) == 0 {
				fmt.Println("no patterns")
				return nil
			}
			for _, p := range *resp.JSON200.Patterns {
				refs := countCodeRefs(p.CodeRefs)
				fmt.Printf("PT-%-4d  %-30s  %d refs  %s\n", p.Id, p.Name, refs, cli.Truncate(p.Description, 60))
			}
			return nil
		},
	}
}

func patternAddCmd() *cobra.Command {
	var desc, codeRefsStr string
	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a pattern",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			body := dxclient.AddPatternRequest{
				Slug:        c.SlugOrDie(),
				Name:        args[0],
				Description: desc,
			}
			if codeRefsStr != "" {
				body.CodeRefs = parseCodeRefs(codeRefsStr)
			}
			resp, err := c.AddPatternWithResponse(cmd.Context(), body)
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil {
				return fmt.Errorf("empty response")
			}
			fmt.Printf("PT-%d  %s\n", resp.JSON200.Id, resp.JSON200.Name)
			return nil
		},
	}
	cmd.Flags().StringVar(&desc, "desc", "", "pattern description")
	cmd.Flags().StringVar(&codeRefsStr, "refs", "", "code references (comma-separated file:line paths)")
	return cmd
}

func patternShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <PT-N|id>",
		Short: "Show pattern details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			id := parsePatternID(args[0])
			resp, err := c.GetPatternWithResponse(cmd.Context(), &dxclient.GetPatternParams{
				Slug: c.SlugOrDie(),
				Id:   id,
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
			p := resp.JSON200
			fmt.Printf("PT-%d  %s\n", p.Id, p.Name)
			fmt.Printf("  Description: %s\n", p.Description)
			refs := decodeCodeRefs(p.CodeRefs)
			if len(refs) > 0 {
				fmt.Println("  Code refs:")
				for _, r := range refs {
					fmt.Printf("    %v\n", r["path"])
				}
			}
			fmt.Printf("  Created: %s  Updated: %s\n", p.CreatedAt, p.UpdatedAt)
			return nil
		},
	}
}

func patternDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <PT-N|id>",
		Short: "Delete a pattern",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			id := parsePatternID(args[0])
			resp, err := c.DeletePatternWithResponse(cmd.Context(), dxclient.DeletePatternRequest{
				Slug: c.SlugOrDie(),
				Id:   id,
			})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			fmt.Printf("PT-%d deleted\n", id)
			return nil
		},
	}
}

func patternSearchCmd() *cobra.Command {
	var n int
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search patterns by similarity",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			nInt := int64(n)
			resp, err := c.SimilarPatternsWithResponse(cmd.Context(), dxclient.SimilarPatternsRequest{
				Slug: c.SlugOrDie(),
				Text: args[0],
				N:    &nInt,
			})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil || resp.JSON200.Patterns == nil || len(*resp.JSON200.Patterns) == 0 {
				fmt.Println("no matching patterns")
				return nil
			}
			for _, r := range *resp.JSON200.Patterns {
				fmt.Printf("  %.3f  PT-%-4d  %s\n", r.Score, r.Pattern.Id, r.Pattern.Name)
			}
			return nil
		},
	}
	cmd.Flags().IntVarP(&n, "count", "n", 5, "number of results")
	return cmd
}

func parsePatternID(s string) int32 {
	s = strings.TrimPrefix(s, "PT-")
	var id int32
	fmt.Sscanf(s, "%d", &id)
	return id
}

func countCodeRefs(raw interface{}) int {
	return len(decodeCodeRefs(raw))
}

func decodeCodeRefs(raw interface{}) []map[string]any {
	if raw == nil {
		return nil
	}
	if refs, ok := raw.([]any); ok {
		out := make([]map[string]any, 0, len(refs))
		for _, r := range refs {
			if m, ok := r.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out
	}
	// Fall back to JSON round-trip for other shapes.
	b, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var refs []map[string]any
	_ = json.Unmarshal(b, &refs)
	return refs
}

func parseCodeRefs(s string) []map[string]string {
	parts := strings.Split(s, ",")
	refs := make([]map[string]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		refs = append(refs, map[string]string{"path": p})
	}
	return refs
}
