package project

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/dxclient"
)

func AtlasCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "atlas", Short: "Atlas substrate (nodes, coderefs, chunks, edges)"}
	cmd.AddCommand(atlasNodeCmd(), atlasCoderefCmd(), atlasChunkCmd(), atlasEdgeCmd(), atlasChunkDriftCmd(), atlasReviewCmd())
	return cmd
}

// ── node ─────────────────────────────────────────────────────────────────────

func atlasNodeCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "node", Short: "Node management"}
	cmd.AddCommand(atlasNodeAddCmd(), atlasNodeShowCmd(), atlasNodeListCmd())
	return cmd
}

func atlasNodeAddCmd() *cobra.Command {
	var title, desc string
	cmd := &cobra.Command{
		Use:   "add <kind> <slug>",
		Short: "Add a node",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			resp, err := c.CreateAtlasNodeWithResponse(cmd.Context(), c.SlugOrDie(), dxclient.CreateAtlasNodeJSONRequestBody{
				Kind:        args[0],
				Slug:        args[1],
				Title:       &title,
				Description: &desc,
			})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			n := resp.JSON200
			fmt.Printf("%-6d %-10s %-20s %s\n", n.Id, n.Kind, n.Slug, n.Title)
			return nil
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "Node title")
	cmd.Flags().StringVar(&desc, "desc", "", "Node description")
	return cmd
}

func atlasNodeShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <kind> <slug>",
		Short: "Show node with chunks and edges",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			resp, err := c.GetAtlasNodeWithResponse(cmd.Context(), c.SlugOrDie(), args[0], args[1])
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			d := resp.JSON200
			fmt.Printf("ID:    %d\n", d.Id)
			fmt.Printf("Kind:  %s\n", d.Kind)
			fmt.Printf("Slug:  %s\n", d.Slug)
			if d.Title != "" {
				fmt.Printf("Title: %s\n", d.Title)
			}
			if d.Description != "" {
				fmt.Printf("Desc:  %s\n", d.Description)
			}
			if d.Chunks != nil && len(*d.Chunks) > 0 {
				fmt.Println("\nChunks:")
				for _, ch := range *d.Chunks {
					title := atlasStr(ch.Title)
					preview := ch.Body
					if len(preview) > 60 {
						preview = preview[:60]
					}
					fmt.Printf("  %-6d %-30s %s\n", ch.Id, title, preview)
				}
			}
			if d.Edges != nil && len(*d.Edges) > 0 {
				fmt.Println("\nEdges:")
				for _, e := range *d.Edges {
					label := ""
					if e.Label != nil && *e.Label != "" {
						label = "  " + *e.Label
					}
					fmt.Printf("  %-6d %-12s → %s/%s%s\n", e.Id, e.EdgeType, e.ToKind, e.ToSlug, label)
				}
			}
			return nil
		},
	}
}

func atlasNodeListCmd() *cobra.Command {
	var kind string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List nodes",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			params := &dxclient.ListAtlasNodesParams{}
			if kind != "" {
				params.Kind = &kind
			}
			resp, err := c.ListAtlasNodesWithResponse(cmd.Context(), c.SlugOrDie(), params)
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil || resp.JSON200.Nodes == nil || len(*resp.JSON200.Nodes) == 0 {
				fmt.Println("no nodes")
				return nil
			}
			for _, n := range *resp.JSON200.Nodes {
				fmt.Printf("%-6d %-10s %-20s %s\n", n.Id, n.Kind, n.Slug, n.Title)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&kind, "kind", "", "Filter by kind")
	return cmd
}

// ── coderef ───────────────────────────────────────────────────────────────────

func atlasCoderefCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "coderef", Short: "Coderef management"}
	cmd.AddCommand(atlasCoderefAddCmd(), atlasCoderefListCmd())
	return cmd
}

func atlasCoderefAddCmd() *cobra.Command {
	var start, end int32
	var symbol, sha string
	cmd := &cobra.Command{
		Use:   "add <file>",
		Short: "Add a coderef",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			body := dxclient.CreateAtlasCoderefJSONRequestBody{
				File:   args[0],
				Symbol: &symbol,
				Sha:    &sha,
			}
			if cmd.Flags().Changed("start") {
				body.StartLine = &start
			}
			if cmd.Flags().Changed("end") {
				body.EndLine = &end
			}
			resp, err := c.CreateAtlasCoderefWithResponse(cmd.Context(), c.SlugOrDie(), body)
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			r := resp.JSON200
			loc := r.File
			if r.StartLine != nil {
				loc += fmt.Sprintf(":%d", *r.StartLine)
			}
			if r.EndLine != nil {
				loc += fmt.Sprintf("-%d", *r.EndLine)
			}
			fmt.Printf("%-6d %s\n", r.Id, loc)
			return nil
		},
	}
	cmd.Flags().Int32Var(&start, "start", 0, "Start line")
	cmd.Flags().Int32Var(&end, "end", 0, "End line")
	cmd.Flags().StringVar(&symbol, "symbol", "", "Symbol name")
	cmd.Flags().StringVar(&sha, "sha", "", "Git SHA")
	return cmd
}

func atlasCoderefListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List coderefs",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			resp, err := c.ListAtlasCoderefsWithResponse(cmd.Context(), c.SlugOrDie())
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil || resp.JSON200.Coderefs == nil || len(*resp.JSON200.Coderefs) == 0 {
				fmt.Println("no coderefs")
				return nil
			}
			for _, r := range *resp.JSON200.Coderefs {
				loc := r.File
				if r.StartLine != nil {
					loc += fmt.Sprintf(":%d", *r.StartLine)
				}
				if r.EndLine != nil {
					loc += fmt.Sprintf("-%d", *r.EndLine)
				}
				fmt.Printf("%-6d %s\n", r.Id, loc)
			}
			return nil
		},
	}
}

// ── chunk ─────────────────────────────────────────────────────────────────────

func atlasChunkCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "chunk", Short: "Narrative chunk management"}
	cmd.AddCommand(atlasChunkAddCmd(), atlasChunkListCmd(), atlasChunkVerifyCmd())
	return cmd
}

func atlasChunkAddCmd() *cobra.Command {
	var title, body, sha string
	var coderefID int64
	cmd := &cobra.Command{
		Use:   "add <node-kind> <node-slug>",
		Short: "Add a chunk to a node",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			slug := c.SlugOrDie()
			nodeResp, err := c.GetAtlasNodeWithResponse(cmd.Context(), slug, args[0], args[1])
			if err != nil {
				return err
			}
			if err := c.CheckStatus(nodeResp.StatusCode(), nodeResp.Body); err != nil {
				return err
			}
			nodeID := nodeResp.JSON200.Id

			chBody := dxclient.CreateAtlasChunkJSONRequestBody{
				Title:      &title,
				Body:       body,
				ShaAtWrite: &sha,
			}
			if cmd.Flags().Changed("coderef") {
				chBody.CoderefId = &coderefID
			}
			resp, err := c.CreateAtlasChunkWithResponse(cmd.Context(), slug, nodeID, chBody)
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			ch := resp.JSON200
			preview := ch.Body
			if len(preview) > 60 {
				preview = preview[:60]
			}
			fmt.Printf("%-6d %-30s %s\n", ch.Id, atlasStr(ch.Title), preview)
			return nil
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "Chunk title")
	cmd.Flags().StringVar(&body, "body", "", "Chunk body")
	cmd.Flags().StringVar(&sha, "sha", "", "SHA at write time")
	cmd.Flags().Int64Var(&coderefID, "coderef", 0, "Coderef ID to attach")
	_ = cmd.MarkFlagRequired("body")
	return cmd
}

func atlasChunkListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <node-kind> <node-slug>",
		Short: "List chunks for a node",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			slug := c.SlugOrDie()
			nodeResp, err := c.GetAtlasNodeWithResponse(cmd.Context(), slug, args[0], args[1])
			if err != nil {
				return err
			}
			if err := c.CheckStatus(nodeResp.StatusCode(), nodeResp.Body); err != nil {
				return err
			}
			nodeID := nodeResp.JSON200.Id

			resp, err := c.ListAtlasChunksWithResponse(cmd.Context(), slug, nodeID)
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil || resp.JSON200.Chunks == nil || len(*resp.JSON200.Chunks) == 0 {
				fmt.Println("no chunks")
				return nil
			}
			for _, ch := range *resp.JSON200.Chunks {
				preview := ch.Body
				if len(preview) > 60 {
					preview = preview[:60]
				}
				fmt.Printf("%-6d %-30s %s\n", ch.Id, atlasStr(ch.Title), preview)
			}
			return nil
		},
	}
}

func atlasChunkVerifyCmd() *cobra.Command {
	var verb, comment, body, sha string
	cmd := &cobra.Command{
		Use:   "verify <chunk-id>",
		Short: "Verify a chunk (still-accurate, edit-prose, mark-broken, agent-regen)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			var chunkID int64
			if _, err := fmt.Sscanf(args[0], "%d", &chunkID); err != nil {
				return fmt.Errorf("invalid chunk-id: %s", args[0])
			}
			if sha == "" {
				out, err := exec.Command("git", "rev-parse", "HEAD").Output()
				if err == nil {
					sha = strings.TrimSpace(string(out))
				}
			}
			resp, err := c.VerifyAtlasChunkWithResponse(cmd.Context(), c.SlugOrDie(), chunkID, dxclient.VerifyAtlasChunkJSONRequestBody{
				Verb:    verb,
				Comment: &comment,
				Body:    &body,
				Sha:     &sha,
			})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			ch := resp.JSON200
			fmt.Printf("chunk %-6d  verb=%-15s  status=%s\n", ch.Id, verb, ch.Status)
			return nil
		},
	}
	cmd.Flags().StringVar(&verb, "verb", "", "Verification verb: still-accurate, edit-prose, mark-broken, agent-regen")
	cmd.Flags().StringVar(&comment, "comment", "", "Proof-of-read comment (required for still-accurate)")
	cmd.Flags().StringVar(&body, "body", "", "New prose body (required for edit-prose)")
	cmd.Flags().StringVar(&sha, "sha", "", "Git SHA (defaults to HEAD)")
	_ = cmd.MarkFlagRequired("verb")
	return cmd
}

// ── edge ──────────────────────────────────────────────────────────────────────

func atlasEdgeCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "edge", Short: "Edge management"}
	cmd.AddCommand(atlasEdgeAddCmd())
	return cmd
}

func atlasEdgeAddCmd() *cobra.Command {
	var fromID, toID int64
	var edgeType, label string
	var weight int32
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add an edge between nodes",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			resp, err := c.CreateAtlasEdgeWithResponse(cmd.Context(), c.SlugOrDie(), dxclient.CreateAtlasEdgeJSONRequestBody{
				FromNodeId: fromID,
				ToNodeId:   toID,
				EdgeType:   edgeType,
				Label:      &label,
				Weight:     &weight,
			})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			e := resp.JSON200
			fmt.Printf("%-6d %d → %d  [%s]\n", e.Id, e.FromNodeId, e.ToNodeId, e.EdgeType)
			return nil
		},
	}
	cmd.Flags().Int64Var(&fromID, "from", 0, "From node ID")
	cmd.Flags().Int64Var(&toID, "to", 0, "To node ID")
	cmd.Flags().StringVar(&edgeType, "type", "contains", "Edge type")
	cmd.Flags().StringVar(&label, "label", "", "Edge label")
	cmd.Flags().Int32Var(&weight, "weight", 0, "Edge weight")
	_ = cmd.MarkFlagRequired("from")
	_ = cmd.MarkFlagRequired("to")
	return cmd
}

// ── review ───────────────────────────────────────────────────────────────────

func atlasReviewCmd() *cobra.Command {
	var since, branch string
	cmd := &cobra.Command{
		Use:   "review",
		Short: "Detect and mark stale chunks from git diff",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()

			var base string
			switch {
			case since != "":
				base = since
			case branch != "":
				out, err := exec.Command("git", "merge-base", "HEAD", branch).Output()
				if err != nil {
					return fmt.Errorf("git merge-base: %w", err)
				}
				base = strings.TrimSpace(string(out))
			default:
				// detect default branch from remote HEAD
				remHead, err := exec.Command("git", "symbolic-ref", "refs/remotes/origin/HEAD").Output()
				if err != nil {
					// fallback to dev
					base = "dev"
				} else {
					parts := strings.Split(strings.TrimSpace(string(remHead)), "/")
					defaultBranch := parts[len(parts)-1]
					mbOut, mbErr := exec.Command("git", "merge-base", "HEAD", defaultBranch).Output()
					if mbErr != nil {
						return fmt.Errorf("git merge-base HEAD %s: %w", defaultBranch, mbErr)
					}
					base = strings.TrimSpace(string(mbOut))
				}
			}

			diffOut, err := exec.Command("git", "diff", "--name-only", base+"..HEAD").Output()
			if err != nil {
				return fmt.Errorf("git diff: %w", err)
			}
			var files []string
			for _, f := range strings.Split(strings.TrimSpace(string(diffOut)), "\n") {
				if f != "" {
					files = append(files, f)
				}
			}

			shaOut, err := exec.Command("git", "rev-parse", "HEAD").Output()
			if err != nil {
				return fmt.Errorf("git rev-parse HEAD: %w", err)
			}
			currentSHA := strings.TrimSpace(string(shaOut))

			resp, err := c.ReviewAtlasWithResponse(cmd.Context(), c.SlugOrDie(), dxclient.ReviewAtlasJSONRequestBody{
				Files:      &files,
				CurrentSha: currentSHA,
			})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			r := resp.JSON200
			if r.StaleCount == 0 {
				fmt.Println("atlas: no stale chunks in diff")
				return nil
			}
			fmt.Printf("atlas: %d stale chunk(s)\n", r.StaleCount)
			if r.Chunks != nil {
				for _, ch := range *r.Chunks {
					fmt.Printf("  %-6d  %s:%s  %s  %s\n",
						ch.Id, ch.NodeKind, ch.NodeSlug,
						cli.Truncate(atlasStr(ch.Title), 30),
						atlasStr(ch.File),
					)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "Mark chunks stale for files changed since this SHA")
	cmd.Flags().StringVar(&branch, "branch", "", "Mark chunks stale for files changed since merge-base with this branch")
	return cmd
}

// ── drift ────────────────────────────────────────────────────────────────────

func atlasChunkDriftCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "drift",
		Short: "Show stale chunk count (exit 1 if any stale)",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := cli.DefaultClient()
			if err != nil || c.Slug() == "" {
				return nil
			}
			var result struct {
				StaleCount int `json:"stale_count"`
				Chunks     []struct {
					ID       int64  `json:"id"`
					NodeKind string `json:"node_kind"`
					NodeSlug string `json:"node_slug"`
					Title    string `json:"title"`
					File     string `json:"file"`
				} `json:"chunks"`
			}
			if err := c.Get(
				"/api/projects/"+c.Slug()+"/atlas/chunks/stale",
				nil,
				&result,
			); err != nil {
				return nil // server doesn't support endpoint yet; exit 0
			}
			if result.StaleCount == 0 {
				return nil
			}
			fmt.Printf("atlas-drift: %d stale chunk(s)\n", result.StaleCount)
			for _, ch := range result.Chunks {
				fmt.Printf("  %-6d  %s:%s  %s  %s\n",
					ch.ID, ch.NodeKind, ch.NodeSlug,
					cli.Truncate(ch.Title, 30),
					ch.File,
				)
			}
			os.Exit(1)
			return nil
		},
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func atlasStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
