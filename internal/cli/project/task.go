package project

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/dxclient"
)

func TaskCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "task", Short: "Task management"}
	cmd.AddCommand(taskReadyCmd())
	cmd.AddCommand(taskDeleteCmd())
	cmd.AddCommand(taskReleaseCmd())
	cmd.AddCommand(taskAdoptCmd())
	cmd.AddCommand(taskBlockCmd())
	return cmd
}

func taskAdoptCmd() *cobra.Command {
	var issueFlag string
	cmd := &cobra.Command{
		Use:   "adopt <TK-N> --issue=<IS-N>",
		Short: "Link an orphan task to a parent issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			taskID := args[0]
			if !strings.HasPrefix(taskID, "TK-") {
				return fmt.Errorf("expected TK-N, got %q", taskID)
			}
			if issueFlag == "" {
				return fmt.Errorf("--issue is required")
			}
			if !strings.HasPrefix(issueFlag, "IS-") {
				return fmt.Errorf("expected --issue=IS-N, got %q", issueFlag)
			}
			c := cli.MustClient()
			slug := c.SlugOrDie()
			resp, err := c.AdoptTaskWithResponse(cmd.Context(), dxclient.AdoptTaskRequest{
				Slug:    slug,
				TaskId:  taskID,
				IssueId: issueFlag,
			})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			fmt.Printf("linked %s → %s\n", taskID, issueFlag)
			return nil
		},
	}
	cmd.Flags().StringVar(&issueFlag, "issue", "", "parent issue (IS-N)")
	_ = cmd.MarkFlagRequired("issue")
	return cmd
}

func taskReadyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ready <TK-N>",
		Short: "Promote a draft task from wip to ready",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			n, _ := strconv.ParseInt(args[0][3:], 10, 32)
			resp, err := c.ReadyTaskWithResponse(cmd.Context(), dxclient.ReadyTaskRequest{
				Id: int32(n),
			})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			fmt.Printf("%s promoted to ready\n", args[0])
			return nil
		},
	}
}

func taskReleaseCmd() *cobra.Command {
	var forceFlag bool
	cmd := &cobra.Command{
		Use:   "release <TK-N>",
		Short: "Release a claimed task reservation (admin, no agent-id required)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return taskReleaseRun(cmd, args[0], forceFlag)
		},
	}
	cmd.Flags().BoolVar(&forceFlag, "force", false, "release even if the lease is still active")
	return cmd
}

func taskReleaseRun(cmd *cobra.Command, arg string, force bool) error {
	if len(arg) < 4 || arg[:3] != "TK-" {
		return fmt.Errorf("expected TK-N, got %q", arg)
	}
	c := cli.MustClient()
	slug := c.SlugOrDie()

	// Fetch current task state to check for active lease.
	taskResp, err := c.GetTaskWithResponse(cmd.Context(), &dxclient.GetTaskParams{Slug: slug, Id: arg[3:]})
	if err != nil {
		return err
	}
	if err := c.CheckStatus(taskResp.StatusCode(), taskResp.Body); err != nil {
		return err
	}
	t := taskResp.JSON200
	if t != nil && t.ClaimedBy != nil && *t.ClaimedBy != "" && t.LeaseExpiresAt != nil {
		if !force {
			return fmt.Errorf("%s has an active lease (claimed-by=%s, expires=%s); use --force to release anyway",
				arg, *t.ClaimedBy, *t.LeaseExpiresAt)
		}
	}

	releaseResp, err := c.ReleaseTaskWithResponse(cmd.Context(), arg[3:], dxclient.ReleaseTaskRequest{AgentId: ""})
	if err != nil {
		return err
	}
	if err := c.CheckStatus(releaseResp.StatusCode(), releaseResp.Body); err != nil {
		return err
	}
	fmt.Printf("%s released\n", arg)
	return nil
}

func taskDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <TK-N>",
		Short: "Delete a draft task (only permitted while in wip state)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			if len(args[0]) < 4 || args[0][:3] != "TK-" {
				return fmt.Errorf("expected TK-N, got %q", args[0])
			}
			n, err := strconv.ParseInt(args[0][3:], 10, 32)
			if err != nil {
				return fmt.Errorf("invalid task id %q: %w", args[0], err)
			}
			resp, err := c.DeleteDraftTaskWithResponse(cmd.Context(), dxclient.DeleteDraftTaskRequest{
				Id: int32(n),
			})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			fmt.Printf("%s deleted\n", args[0])
			return nil
		},
	}
}

func taskBlockCmd() *cobra.Command {
	var byFlag string
	var reasonFlag string
	cmd := &cobra.Command{
		Use:   "block <TK-N> --by=IS-M",
		Short: "Convert a wip task into a blocker on its parent issue, then delete the task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			taskID := args[0]
			if !strings.HasPrefix(taskID, "TK-") {
				return fmt.Errorf("expected TK-N, got %q", taskID)
			}
			if !strings.HasPrefix(byFlag, "IS-") {
				return fmt.Errorf("expected --by=IS-N, got %q", byFlag)
			}
			n, err := strconv.ParseInt(taskID[3:], 10, 32)
			if err != nil {
				return fmt.Errorf("invalid task id %q: %w", taskID, err)
			}
			c := cli.MustClient()
			var reason *string
			if reasonFlag != "" {
				reason = &reasonFlag
			}
			resp, err := c.ConvertTaskToBlockerWithResponse(cmd.Context(), dxclient.ConvertTaskToBlockerRequest{
				Id:               int32(n),
				BlockedByIssueId: byFlag,
				Reason:           reason,
			})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			fmt.Printf("%s deleted; parent issue blocked-by %s\n", taskID, byFlag)
			return nil
		},
	}
	cmd.Flags().StringVar(&byFlag, "by", "", "issue that blocks the parent (IS-N)")
	_ = cmd.MarkFlagRequired("by")
	cmd.Flags().StringVar(&reasonFlag, "reason", "", "reason for the dependency (optional but encouraged)")
	return cmd
}
