package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type patternItem struct {
	ID          int32           `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	CodeRefs    json.RawMessage `json:"code_refs"`
	CreatedAt   string          `json:"created_at"`
	UpdatedAt   string          `json:"updated_at"`
}

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
			c := MustClient()
			var resp struct {
				Patterns []patternItem `json:"patterns"`
				Total    int64         `json:"total"`
			}
			if err := c.Get("/api/dx/patterns", QuerySlug(c), &resp); err != nil {
				return err
			}
			if len(resp.Patterns) == 0 {
				fmt.Println("no patterns")
				return nil
			}
			for _, p := range resp.Patterns {
				refs := countCodeRefs(p.CodeRefs)
				fmt.Printf("PT-%-4d  %-30s  %d refs  %s\n", p.ID, p.Name, refs, Truncate(p.Description, 60))
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
			c := MustClient()
			body := map[string]any{
				"slug":        c.SlugOrDie(),
				"name":        args[0],
				"description": desc,
			}
			if codeRefsStr != "" {
				refs := parseCodeRefs(codeRefsStr)
				body["code_refs"] = refs
			}
			var p patternItem
			if err := c.Post("/api/dx/patterns/add", body, &p); err != nil {
				return err
			}
			fmt.Printf("PT-%d  %s\n", p.ID, p.Name)
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
			c := MustClient()
			id := parsePatternID(args[0])
			var p patternItem
			q := QuerySlug(c)
			q.Set("id", fmt.Sprintf("%d", id))
			if err := c.Get("/api/dx/patterns/get", q, &p); err != nil {
				return err
			}
			fmt.Printf("PT-%d  %s\n", p.ID, p.Name)
			fmt.Printf("  Description: %s\n", p.Description)
			var refs []map[string]any
			_ = json.Unmarshal(p.CodeRefs, &refs)
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
			c := MustClient()
			id := parsePatternID(args[0])
			var ok struct {
				OK bool `json:"ok"`
			}
			if err := c.Post("/api/dx/patterns/delete", map[string]any{
				"slug": c.SlugOrDie(),
				"id":   id,
			}, &ok); err != nil {
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
			c := MustClient()
			var resp struct {
				Patterns []struct {
					Pattern patternItem `json:"pattern"`
					Score   float64     `json:"score"`
				} `json:"patterns"`
			}
			if err := c.Post("/api/dx/patterns/similar", map[string]any{
				"slug": c.SlugOrDie(),
				"text": args[0],
				"n":    n,
			}, &resp); err != nil {
				return err
			}
			if len(resp.Patterns) == 0 {
				fmt.Println("no matching patterns")
				return nil
			}
			for _, r := range resp.Patterns {
				fmt.Printf("  %.3f  PT-%-4d  %s\n", r.Score, r.Pattern.ID, r.Pattern.Name)
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

func countCodeRefs(raw json.RawMessage) int {
	var refs []any
	_ = json.Unmarshal(raw, &refs)
	return len(refs)
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
