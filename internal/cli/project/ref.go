package project

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/dxclient"
)

func printCodeRef(r dxclient.CodeRefItem) {
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
	hashStr := ""
	if hash != "" {
		hashStr = " @" + hash
	}
	fmt.Printf("[%d] %s%s%s\n", r.Id, loc, hashStr, note)
}

func RefCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "ref", Short: "Attach code references to issues, tasks, and tests"}
	cmd.AddCommand(
		refIssueAttachCmd(),
		refIssueListCmd(),
		refIssueDetachCmd(),
		refTaskAttachCmd(),
		refTaskListCmd(),
		refTaskDetachCmd(),
		refTestAttachCmd(),
		refTestListCmd(),
		refTestDetachCmd(),
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
			c := cli.MustClient()
			body := dxclient.AttachCodeRefToIssueRequest{
				Slug:     c.SlugOrDie(),
				IssueId:  args[0],
				FilePath: filePath,
			}
			if gitHash != "" {
				body.GitHash = &gitHash
			}
			if lineStart > 0 {
				body.LineStart = &lineStart
			}
			if lineEnd > 0 {
				body.LineEnd = &lineEnd
			}
			if note != "" {
				body.Note = &note
			}
			resp, err := c.AttachCodeRefToIssueWithResponse(cmd.Context(), body)
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil {
				return fmt.Errorf("empty response")
			}
			printCodeRef(*resp.JSON200)
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
			c := cli.MustClient()
			resp, err := c.ListCodeRefsForIssueWithResponse(cmd.Context(), &dxclient.ListCodeRefsForIssueParams{
				Slug:    c.SlugOrDie(),
				IssueId: args[0],
			})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil || resp.JSON200.Refs == nil || len(*resp.JSON200.Refs) == 0 {
				fmt.Println("no code refs")
				return nil
			}
			for _, r := range *resp.JSON200.Refs {
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
			c := cli.MustClient()
			id, err := strconv.ParseInt(args[1], 10, 32)
			if err != nil {
				return fmt.Errorf("invalid ref id: %s", args[1])
			}
			resp, err := c.DetachCodeRefFromIssueWithResponse(cmd.Context(), dxclient.DetachCodeRefFromIssueRequest{
				Slug:      c.SlugOrDie(),
				IssueId:   args[0],
				CodeRefId: int32(id),
			})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
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
			c := cli.MustClient()
			body := dxclient.AttachCodeRefToTaskRequest{
				Slug:     c.SlugOrDie(),
				TaskId:   args[0],
				FilePath: filePath,
			}
			if gitHash != "" {
				body.GitHash = &gitHash
			}
			if lineStart > 0 {
				body.LineStart = &lineStart
			}
			if lineEnd > 0 {
				body.LineEnd = &lineEnd
			}
			if note != "" {
				body.Note = &note
			}
			resp, err := c.AttachCodeRefToTaskWithResponse(cmd.Context(), body)
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil {
				return fmt.Errorf("empty response")
			}
			printCodeRef(*resp.JSON200)
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
			c := cli.MustClient()
			resp, err := c.ListCodeRefsForTaskWithResponse(cmd.Context(), &dxclient.ListCodeRefsForTaskParams{
				Slug:   c.SlugOrDie(),
				TaskId: args[0],
			})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil || resp.JSON200.Refs == nil || len(*resp.JSON200.Refs) == 0 {
				fmt.Println("no code refs")
				return nil
			}
			for _, r := range *resp.JSON200.Refs {
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
			c := cli.MustClient()
			id, err := strconv.ParseInt(args[1], 10, 32)
			if err != nil {
				return fmt.Errorf("invalid ref id: %s", args[1])
			}
			resp, err := c.DetachCodeRefFromTaskWithResponse(cmd.Context(), dxclient.DetachCodeRefFromTaskRequest{
				Slug:      c.SlugOrDie(),
				TaskId:    args[0],
				CodeRefId: int32(id),
			})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			fmt.Println("detached")
			return nil
		},
	}
}

func refTestAttachCmd() *cobra.Command {
	var filePath, gitHash, note string
	var lineStart, lineEnd int32
	cmd := &cobra.Command{
		Use:   "test-attach <test-id>",
		Short: "Attach a code reference to a test",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			testID, err := strconv.ParseInt(args[0], 10, 32)
			if err != nil {
				return fmt.Errorf("invalid test id: %s", args[0])
			}
			body := dxclient.AttachCodeRefToTestRequest{
				Slug:     c.SlugOrDie(),
				TestId:   int32(testID),
				FilePath: filePath,
			}
			if gitHash != "" {
				body.GitHash = &gitHash
			}
			if lineStart > 0 {
				body.LineStart = &lineStart
			}
			if lineEnd > 0 {
				body.LineEnd = &lineEnd
			}
			if note != "" {
				body.Note = &note
			}
			resp, err := c.AttachCodeRefToTestWithResponse(cmd.Context(), body)
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil {
				return fmt.Errorf("empty response")
			}
			printCodeRef(*resp.JSON200)
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

func refTestListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "test-list <test-id>",
		Short: "List code references attached to a test",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			testID, err := strconv.ParseInt(args[0], 10, 32)
			if err != nil {
				return fmt.Errorf("invalid test id: %s", args[0])
			}
			resp, err := c.ListCodeRefsForTestWithResponse(cmd.Context(), &dxclient.ListCodeRefsForTestParams{
				Slug:   c.SlugOrDie(),
				TestId: int32(testID),
			})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil || resp.JSON200.Refs == nil || len(*resp.JSON200.Refs) == 0 {
				fmt.Println("no code refs")
				return nil
			}
			for _, r := range *resp.JSON200.Refs {
				printCodeRef(r)
			}
			return nil
		},
	}
}

func refTestDetachCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "test-detach <test-id> <ref-id>",
		Short: "Detach a code reference from a test",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			testID, err := strconv.ParseInt(args[0], 10, 32)
			if err != nil {
				return fmt.Errorf("invalid test id: %s", args[0])
			}
			id, err := strconv.ParseInt(args[1], 10, 32)
			if err != nil {
				return fmt.Errorf("invalid ref id: %s", args[1])
			}
			resp, err := c.DetachCodeRefFromTestWithResponse(cmd.Context(), dxclient.DetachCodeRefFromTestRequest{
				Slug:      c.SlugOrDie(),
				TestId:    int32(testID),
				CodeRefId: int32(id),
			})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			fmt.Println("detached")
			return nil
		},
	}
}
