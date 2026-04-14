package cli

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/spf13/cobra"
)

type codeRefItem struct {
	ID        int32  `json:"id"`
	FilePath  string `json:"file_path"`
	GitHash   string `json:"git_hash"`
	LineStart int32  `json:"line_start"`
	LineEnd   int32  `json:"line_end"`
	Note      string `json:"note"`
	CreatedAt string `json:"created_at"`
}

func printCodeRef(r codeRefItem) {
	loc := r.FilePath
	if r.LineStart > 0 {
		if r.LineEnd > 0 && r.LineEnd != r.LineStart {
			loc += fmt.Sprintf(":%d-%d", r.LineStart, r.LineEnd)
		} else {
			loc += fmt.Sprintf(":%d", r.LineStart)
		}
	}
	hash := r.GitHash
	if len(hash) > 8 {
		hash = hash[:8]
	}
	note := ""
	if r.Note != "" {
		note = "  # " + r.Note
	}
	fmt.Printf("[%d] %s%s%s\n", r.ID, loc, func() string {
		if hash != "" {
			return " @" + hash
		}
		return ""
	}(), note)
}

func RefCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "ref", Short: "Attach code references to issues and tasks"}
	cmd.AddCommand(
		refIssueAttachCmd(),
		refIssueListCmd(),
		refIssueDetachCmd(),
		refTaskAttachCmd(),
		refTaskListCmd(),
		refTaskDetachCmd(),
	)
	return cmd
}

func refIssueAttachCmd() *cobra.Command {
	var filePath, gitHash, note string
	var lineStart, lineEnd int32
	cmd := &cobra.Command{
		Use:   "issue-attach <IS-N>",
		Short: "Attach a code reference to an issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := mustClient()
			body := map[string]any{
				"slug":      c.SlugOrDie(),
				"issue_id":  args[0],
				"file_path": filePath,
			}
			if gitHash != "" {
				body["git_hash"] = gitHash
			}
			if lineStart > 0 {
				body["line_start"] = lineStart
			}
			if lineEnd > 0 {
				body["line_end"] = lineEnd
			}
			if note != "" {
				body["note"] = note
			}
			var ref codeRefItem
			if err := c.post("/api/dx/code-refs/issue/attach", body, &ref); err != nil {
				return err
			}
			printCodeRef(ref)
			return nil
		},
	}
	cmd.Flags().StringVar(&filePath, "file", "", "file path (required)")
	cmd.Flags().StringVar(&gitHash, "hash", "", "git commit hash")
	cmd.Flags().Int32Var(&lineStart, "line-start", 0, "start line number")
	cmd.Flags().Int32Var(&lineEnd, "line-end", 0, "end line number")
	cmd.Flags().StringVar(&note, "note", "", "optional annotation")
	_ = cmd.MarkFlagRequired("file")
	return cmd
}

func refIssueListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "issue-list <IS-N>",
		Short: "List code references attached to an issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := mustClient()
			var resp struct {
				Refs []codeRefItem `json:"refs"`
			}
			if err := c.get("/api/dx/code-refs/issue", url.Values{
				"slug":     {c.SlugOrDie()},
				"issue_id": {args[0]},
			}, &resp); err != nil {
				return err
			}
			if len(resp.Refs) == 0 {
				fmt.Println("no code refs")
				return nil
			}
			for _, r := range resp.Refs {
				printCodeRef(r)
			}
			return nil
		},
	}
}

func refIssueDetachCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "issue-detach <IS-N> <ref-id>",
		Short: "Detach a code reference from an issue",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := mustClient()
			id, err := strconv.ParseInt(args[1], 10, 32)
			if err != nil {
				return fmt.Errorf("invalid ref id: %s", args[1])
			}
			var ok struct {
				OK bool `json:"ok"`
			}
			if err := c.post("/api/dx/code-refs/issue/detach", map[string]any{
				"slug":        c.SlugOrDie(),
				"issue_id":    args[0],
				"code_ref_id": int32(id),
			}, &ok); err != nil {
				return err
			}
			fmt.Println("detached")
			return nil
		},
	}
}

func refTaskAttachCmd() *cobra.Command {
	var filePath, gitHash, note string
	var lineStart, lineEnd int32
	cmd := &cobra.Command{
		Use:   "task-attach <TK-N>",
		Short: "Attach a code reference to a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := mustClient()
			body := map[string]any{
				"slug":      c.SlugOrDie(),
				"task_id":   args[0],
				"file_path": filePath,
			}
			if gitHash != "" {
				body["git_hash"] = gitHash
			}
			if lineStart > 0 {
				body["line_start"] = lineStart
			}
			if lineEnd > 0 {
				body["line_end"] = lineEnd
			}
			if note != "" {
				body["note"] = note
			}
			var ref codeRefItem
			if err := c.post("/api/dx/code-refs/task/attach", body, &ref); err != nil {
				return err
			}
			printCodeRef(ref)
			return nil
		},
	}
	cmd.Flags().StringVar(&filePath, "file", "", "file path (required)")
	cmd.Flags().StringVar(&gitHash, "hash", "", "git commit hash")
	cmd.Flags().Int32Var(&lineStart, "line-start", 0, "start line number")
	cmd.Flags().Int32Var(&lineEnd, "line-end", 0, "end line number")
	cmd.Flags().StringVar(&note, "note", "", "optional annotation")
	_ = cmd.MarkFlagRequired("file")
	return cmd
}

func refTaskListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "task-list <TK-N>",
		Short: "List code references attached to a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := mustClient()
			var resp struct {
				Refs []codeRefItem `json:"refs"`
			}
			if err := c.get("/api/dx/code-refs/task", url.Values{
				"slug":    {c.SlugOrDie()},
				"task_id": {args[0]},
			}, &resp); err != nil {
				return err
			}
			if len(resp.Refs) == 0 {
				fmt.Println("no code refs")
				return nil
			}
			for _, r := range resp.Refs {
				printCodeRef(r)
			}
			return nil
		},
	}
}

func refTaskDetachCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "task-detach <TK-N> <ref-id>",
		Short: "Detach a code reference from a task",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := mustClient()
			id, err := strconv.ParseInt(args[1], 10, 32)
			if err != nil {
				return fmt.Errorf("invalid ref id: %s", args[1])
			}
			var ok struct {
				OK bool `json:"ok"`
			}
			if err := c.post("/api/dx/code-refs/task/detach", map[string]any{
				"slug":        c.SlugOrDie(),
				"task_id":     args[0],
				"code_ref_id": int32(id),
			}, &ok); err != nil {
				return err
			}
			fmt.Println("detached")
			return nil
		},
	}
}
