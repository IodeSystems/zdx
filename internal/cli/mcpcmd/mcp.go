package mcpcmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/cli/clitypes"
	"github.com/iodesystems/zdx-go/internal/dxclient"
	"github.com/iodesystems/zdx-go/internal/workflowhints"
)

func McpCmd() *cobra.Command {
	var withFS, withShell bool
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Start MCP server (stdio transport)",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := cli.DefaultClient()
			if err != nil {
				return err
			}
			srv := mcp.NewServer(&mcp.Implementation{
				Name:    "dx",
				Version: "0.1.0",
			}, nil)
			RegisterMCPTools(srv, c)
			if withFS {
				root, err := cli.GitRepoRoot()
				if err != nil {
					return fmt.Errorf("--with-fs requires a git repo: %w", err)
				}
				RegisterFSTools(srv, root)
			}
			if withShell {
				root, err := cli.GitRepoRoot()
				if err != nil {
					return fmt.Errorf("--with-shell requires a git repo: %w", err)
				}
				RegisterShellTools(srv, root)
			}
			return srv.Run(cmd.Context(), &mcp.StdioTransport{})
		},
	}
	cmd.Flags().BoolVar(&withFS, "with-fs", false, "expose filesystem tools (read_file, write_file, edit_file, grep, glob, list_dir)")
	cmd.Flags().BoolVar(&withShell, "with-shell", false, "expose run_bash shell tool")
	return cmd
}

// RegisterMCPTools wires the issue/task/feature/etc MCP tools onto srv, scoped
// to the project slug reachable via c.
func RegisterMCPTools(srv *mcp.Server, c *cli.Client) {
	slug := c.Slug()

	// ── issue tools ──────────────────────────────────────────────────────────

	type issueListInput struct {
		Status string `json:"status,omitempty" jsonschema:"filter by status (open, closed)"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "issue_list",
		Description: "List issues, optionally filtered by status",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in issueListInput) (*mcp.CallToolResult, any, error) {
		resp, err := c.ListIssuesWithResponse(ctx, &dxclient.ListIssuesParams{Slug: slug})
		if err != nil {
			return nil, nil, err
		}
		if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
			return nil, nil, err
		}
		var issues []dxclient.IssueItem
		if resp.JSON200 != nil && resp.JSON200.Issues != nil {
			issues = *resp.JSON200.Issues
		}
		if in.Status != "" {
			filtered := issues[:0]
			for _, iss := range issues {
				if iss.Status == in.Status {
					filtered = append(filtered, iss)
				}
			}
			issues = filtered
		}
		return nil, map[string]any{"issues": issues}, nil
	})

	type issueAddInput struct {
		Title     string `json:"title" jsonschema:"required,issue title"`
		Context   string `json:"context,omitempty" jsonschema:"context / description"`
		Component string `json:"component,omitempty" jsonschema:"component"`
		BlockedBy string `json:"blocked_by,omitempty" jsonschema:"blocking issue (IS-N)"`
		Parent    string `json:"parent,omitempty" jsonschema:"parent issue (IS-N): new issue blocks the parent"`
		IssueType string `json:"issue_type,omitempty" jsonschema:"issue type: ops, impl, ask, or tracker"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "issue_add",
		Description: "Create a new issue",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in issueAddInput) (*mcp.CallToolResult, any, error) {
		issType := in.IssueType
		if issType == "" {
			issType = "unknown"
		}
		addBody := dxclient.AddIssueRequest{
			Slug:      slug,
			IssueType: &issType,
		}
		if in.Title != "" {
			addBody.Title = &in.Title
		}
		if in.Context != "" {
			addBody.Context = &in.Context
		}
		if in.Component != "" {
			addBody.Component = &in.Component
		}
		if in.BlockedBy != "" {
			blockers := []string{in.BlockedBy}
			addBody.BlockedBy = &blockers
		}
		addResp, err := c.AddIssueWithResponse(ctx, addBody)
		if err != nil {
			return nil, nil, err
		}
		if err := c.CheckStatus(addResp.StatusCode(), addResp.Body); err != nil {
			return nil, nil, err
		}
		if addResp.JSON200 == nil {
			return nil, nil, fmt.Errorf("empty response")
		}
		iss := *addResp.JSON200
		if in.Parent != "" {
			parentNum, _ := strconv.ParseInt(in.Parent[3:], 10, 32)
			_, _ = c.IssueAddBlockWithResponse(ctx, dxclient.IssueAddBlockRequest{
				Slug:      slug,
				Id:        int32(parentNum),
				BlockedBy: clitypes.IssueIDStr(iss.Id),
			})
		}
		// If similar issues were found, add guidance to the response.
		if iss.Similar != nil && len(*iss.Similar) > 0 {
			return nil, map[string]any{
				"issue":    iss,
				"guidance": workflowhints.SimilarIssuesMCPGuidance(clitypes.IssueIDStr(iss.Id)),
			}, nil
		}
		return nil, iss, nil
	})

	type issueShowInput struct {
		ID string `json:"id" jsonschema:"required,issue ID (IS-N)"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "issue_show",
		Description: "Show issue detail with work log",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in issueShowInput) (*mcp.CallToolResult, any, error) {
		resp, err := c.ShowIssueWithResponse(ctx, &dxclient.ShowIssueParams{Slug: slug, Id: in.ID})
		if err != nil {
			return nil, nil, err
		}
		if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
			return nil, nil, err
		}
		if resp.JSON200 == nil {
			return nil, nil, fmt.Errorf("empty response")
		}
		return nil, resp.JSON200, nil
	})

	type issueCloseInput struct {
		ID     string `json:"id" jsonschema:"required,issue ID (IS-N)"`
		Reason string `json:"reason,omitempty" jsonschema:"close reason"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "issue_close",
		Description: "Close an issue",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in issueCloseInput) (*mcp.CallToolResult, any, error) {
		n, err := parseIssueNum(in.ID)
		if err != nil {
			return nil, nil, err
		}
		closeBody := dxclient.CloseIssueRequest{
			Slug: slug,
			Id:   n,
		}
		if in.Reason != "" {
			closeBody.Reason = &in.Reason
		}
		resp, err := c.CloseIssueWithResponse(ctx, closeBody)
		if err != nil {
			return nil, nil, err
		}
		if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
			return nil, nil, err
		}
		return nil, map[string]string{"status": "closed", "id": in.ID}, nil
	})

	type issueReopenInput struct {
		ID     string `json:"id" jsonschema:"required,issue ID (IS-N)"`
		Reason string `json:"reason" jsonschema:"required,why the issue is being reopened"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "issue_reopen",
		Description: "Reopen a closed issue with a reason (increments reopen_count as a churn signal)",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in issueReopenInput) (*mcp.CallToolResult, any, error) {
		n, err := parseIssueNum(in.ID)
		if err != nil {
			return nil, nil, err
		}
		resp, err := c.ReopenIssueWithResponse(ctx, dxclient.ReopenIssueJSONRequestBody{Id: n})
		if err != nil {
			return nil, nil, err
		}
		if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
			return nil, nil, err
		}
		// Record the reason as a comment so it's visible in the work log.
		if in.Reason != "" {
			_, _ = c.AddCommentWithResponse(ctx, dxclient.AddCommentRequest{
				Slug:       slug,
				TargetType: "issue",
				TargetId:   in.ID,
				Body:       "[reopen] " + in.Reason,
			})
		}
		return nil, map[string]string{"status": "reopened", "id": in.ID}, nil
	})

	type issueEditInput struct {
		ID        string `json:"id" jsonschema:"required,issue ID (IS-N)"`
		Title     string `json:"title,omitempty" jsonschema:"issue title"`
		Context   string `json:"context,omitempty" jsonschema:"context / description"`
		Priority  *int   `json:"priority,omitempty" jsonschema:"priority (1-4)"`
		Component string `json:"component,omitempty" jsonschema:"component"`
		IssueType string `json:"issue_type,omitempty" jsonschema:"issue type: ops, impl, ask, or tracker"`
		BlockedBy string `json:"blocked_by,omitempty" jsonschema:"blocking issue (IS-N) or empty to clear"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "issue_edit",
		Description: "Edit fields on an existing issue",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in issueEditInput) (*mcp.CallToolResult, any, error) {
		n, err := parseIssueNum(in.ID)
		if err != nil {
			return nil, nil, err
		}
		body := dxclient.EditIssueRequest{
			Slug: slug,
			Id:   n,
		}
		if in.Title != "" {
			body.Title = &in.Title
		}
		if in.Context != "" {
			body.Context = &in.Context
		}
		if in.Priority != nil {
			p := int32(*in.Priority)
			body.Priority = &p
		}
		if in.Component != "" {
			body.Component = &in.Component
		}
		if in.IssueType != "" {
			body.IssueType = &in.IssueType
		}
		// Note: EditIssueRequest has no blocked_by; MCP schema advertises it
		// but it's ignored. Separate issue_block / issue_unblock tools handle that.
		resp, err := c.EditIssueWithResponse(ctx, body)
		if err != nil {
			return nil, nil, err
		}
		if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
			return nil, nil, err
		}
		return nil, map[string]string{"status": "updated", "id": in.ID}, nil
	})

	type issueBlockInput struct {
		ID string `json:"id" jsonschema:"required,issue ID (IS-N)"`
		By string `json:"by" jsonschema:"required,blocking issue (IS-N)"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "issue_block",
		Description: "Set a blocker on an issue",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in issueBlockInput) (*mcp.CallToolResult, any, error) {
		n, err := parseIssueNum(in.ID)
		if err != nil {
			return nil, nil, err
		}
		resp, err := c.IssueAddBlockWithResponse(ctx, dxclient.IssueAddBlockRequest{
			Slug:      slug,
			Id:        n,
			BlockedBy: in.By,
		})
		if err != nil {
			return nil, nil, err
		}
		if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
			return nil, nil, err
		}
		return nil, map[string]string{"status": "blocked", "id": in.ID, "by": in.By}, nil
	})

	type issueUnblockInput struct {
		ID string `json:"id" jsonschema:"required,issue ID (IS-N)"`
		By string `json:"by,omitempty" jsonschema:"specific blocker (IS-N) to remove; omit to clear all"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "issue_unblock",
		Description: "Clear blockers on an issue",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in issueUnblockInput) (*mcp.CallToolResult, any, error) {
		n, err := parseIssueNum(in.ID)
		if err != nil {
			return nil, nil, err
		}
		body := dxclient.IssueRemoveBlockRequest{
			Slug: slug,
			Id:   n,
		}
		if in.By != "" {
			body.BlockedBy = &in.By
		}
		resp, err := c.IssueRemoveBlockWithResponse(ctx, body)
		if err != nil {
			return nil, nil, err
		}
		if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
			return nil, nil, err
		}
		return nil, map[string]string{"status": "unblocked", "id": in.ID}, nil
	})

	// ── todo tools ───────────────────────────────────────────────────────────

	type todoSoloInput struct {
		Issue string `json:"issue,omitempty" jsonschema:"filter to specific issue (IS-N)"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "todo_solo",
		Description: "Get next actionable item from the workflow queue",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in todoSoloInput) (*mcp.CallToolResult, any, error) {
		args := []string{"todo", "solo"}
		if in.Issue != "" {
			args = append(args, "--issue="+in.Issue)
		}
		out, err := runDX(args...)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %s", err, out)
		}
		return nil, map[string]string{"output": strings.TrimSpace(out)}, nil
	})

	type todoShowInput struct {
		ID string `json:"id" jsonschema:"required,entity ID (IS-N, TK-N, or feature name)"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "todo_show",
		Description: "Show issue, task, or feature details",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in todoShowInput) (*mcp.CallToolResult, any, error) {
		id := in.ID
		switch {
		case len(id) > 3 && id[:3] == "IS-":
			resp, err := c.ShowIssueWithResponse(ctx, &dxclient.ShowIssueParams{Slug: slug, Id: id})
			if err != nil {
				return nil, nil, err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return nil, nil, err
			}
			if resp.JSON200 == nil {
				return nil, nil, fmt.Errorf("empty response")
			}
			return nil, resp.JSON200, nil
		case len(id) > 3 && id[:3] == "TK-":
			n, _ := strconv.ParseInt(id[3:], 10, 32)
			resp, err := c.ListTasksWithResponse(ctx, &dxclient.ListTasksParams{Slug: slug})
			if err != nil {
				return nil, nil, err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return nil, nil, err
			}
			if resp.JSON200 != nil && resp.JSON200.Tasks != nil {
				for _, t := range *resp.JSON200.Tasks {
					if t.Id == int32(n) {
						return nil, t, nil
					}
				}
			}
			return nil, nil, fmt.Errorf("task %s not found", id)
		default:
			resp, err := c.ListFeaturesWithResponse(ctx, &dxclient.ListFeaturesParams{Slug: slug})
			if err != nil {
				return nil, nil, err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return nil, nil, err
			}
			if resp.JSON200 != nil && resp.JSON200.Features != nil {
				for _, f := range *resp.JSON200.Features {
					if f.Name == id {
						return nil, f, nil
					}
				}
			}
			return nil, nil, fmt.Errorf("feature %q not found", id)
		}
	})

	type todoListInput struct {
		Issue   string `json:"issue,omitempty" jsonschema:"filter by issue (IS-N)"`
		Feature string `json:"feature,omitempty" jsonschema:"filter by feature name"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "todo_list",
		Description: "List tasks, optionally filtered by issue or feature",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in todoListInput) (*mcp.CallToolResult, any, error) {
		var tasks []dxclient.TaskItem
		if in.Issue != "" {
			resp, err := c.ListTasksForIssueWithResponse(ctx, &dxclient.ListTasksForIssueParams{
				Slug:    slug,
				IssueId: in.Issue,
			})
			if err != nil {
				return nil, nil, err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return nil, nil, err
			}
			if resp.JSON200 != nil && resp.JSON200.Tasks != nil {
				tasks = *resp.JSON200.Tasks
			}
		} else if in.Feature != "" {
			resp, err := c.ListTasksByFeatureWithResponse(ctx, &dxclient.ListTasksByFeatureParams{
				Slug:    slug,
				Feature: in.Feature,
			})
			if err != nil {
				return nil, nil, err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return nil, nil, err
			}
			if resp.JSON200 != nil && resp.JSON200.Tasks != nil {
				tasks = *resp.JSON200.Tasks
			}
		} else {
			resp, err := c.ListTasksWithResponse(ctx, &dxclient.ListTasksParams{Slug: slug})
			if err != nil {
				return nil, nil, err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return nil, nil, err
			}
			if resp.JSON200 != nil && resp.JSON200.Tasks != nil {
				tasks = *resp.JSON200.Tasks
			}
		}
		return nil, map[string]any{"tasks": tasks}, nil
	})

	type todoDevDoneInput struct {
		ID       string `json:"id" jsonschema:"required,task ID (TK-N)"`
		TestPlan string `json:"test_plan,omitempty" jsonschema:"test plan description"`
		TestRefs string `json:"test_refs,omitempty" jsonschema:"test references"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "todo_dev_done",
		Description: "Mark a task as done",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in todoDevDoneInput) (*mcp.CallToolResult, any, error) {
		n, err := parseTaskNum(in.ID)
		if err != nil {
			return nil, nil, err
		}
		doneBody := dxclient.MarkTaskDoneRequest{Id: n}
		if in.TestPlan != "" {
			doneBody.TestPlan = &in.TestPlan
		}
		if in.TestRefs != "" {
			doneBody.TestRefs = &in.TestRefs
		}
		resp, err := c.MarkTaskDoneWithResponse(ctx, doneBody)
		if err != nil {
			return nil, nil, err
		}
		if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
			return nil, nil, err
		}
		return nil, map[string]string{"status": "done", "id": in.ID}, nil
	})

	type todoDevStartInput struct {
		ID     string `json:"id" jsonschema:"required,task ID (TK-N)"`
		Reason string `json:"reason,omitempty" jsonschema:"notes"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "todo_dev_start",
		Description: "Mark a task as active",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in todoDevStartInput) (*mcp.CallToolResult, any, error) {
		n, err := parseTaskNum(in.ID)
		if err != nil {
			return nil, nil, err
		}
		startBody := dxclient.UpdateTaskStatusRequest{
			Id:     n,
			Status: "active",
		}
		if in.Reason != "" {
			startBody.Reason = &in.Reason
		}
		resp, err := c.UpdateTaskStatusWithResponse(ctx, startBody)
		if err != nil {
			return nil, nil, err
		}
		if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
			return nil, nil, err
		}
		return nil, map[string]string{"status": "active", "id": in.ID}, nil
	})

	type todoOwnerTriageInput struct {
		ID        string `json:"id" jsonschema:"required,issue ID (IS-N)"`
		Priority  int    `json:"priority" jsonschema:"required,priority 1-4 (1=highest)"`
		Title     string `json:"title,omitempty" jsonschema:"set issue title"`
		IssueType string `json:"issue_type,omitempty" jsonschema:"issue type: ops, impl, ask, or tracker"`
		Context   string `json:"context,omitempty" jsonschema:"rewrite issue context"`
		ThemeIDs  []int  `json:"theme_ids,omitempty" jsonschema:"theme IDs to link"`
		GoalIDs   []int  `json:"goal_ids,omitempty" jsonschema:"goal IDs to link"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "todo_owner_triage",
		Description: "Triage an issue by setting priority and optionally rewriting title/context",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in todoOwnerTriageInput) (*mcp.CallToolResult, any, error) {
		n, err := parseIssueNum(in.ID)
		if err != nil {
			return nil, nil, err
		}
		body := dxclient.TriageIssueRequest{
			Slug:     slug,
			Id:       n,
			Priority: int32(in.Priority),
		}
		if in.Title != "" {
			body.Title = &in.Title
		}
		if in.IssueType != "" {
			body.IssueType = &in.IssueType
		}
		if in.Context != "" {
			body.Context = &in.Context
		}
		if len(in.ThemeIDs) > 0 {
			ids := make([]int32, len(in.ThemeIDs))
			for i, v := range in.ThemeIDs {
				ids[i] = int32(v)
			}
			body.ThemeIds = &ids
		}
		if len(in.GoalIDs) > 0 {
			ids := make([]int32, len(in.GoalIDs))
			for i, v := range in.GoalIDs {
				ids[i] = int32(v)
			}
			body.GoalIds = &ids
		}
		resp, err := c.TriageIssueWithResponse(ctx, body)
		if err != nil {
			return nil, nil, err
		}
		if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
			return nil, nil, err
		}
		return nil, map[string]any{"status": "triaged", "id": in.ID, "priority": in.Priority}, nil
	})

	type todoTechAddInput struct {
		Text      string `json:"text" jsonschema:"required,task description"`
		Issue     string `json:"issue,omitempty" jsonschema:"link to issue (IS-N)"`
		Feature   string `json:"feature,omitempty" jsonschema:"link to feature name"`
		TaskGroup string `json:"task_group,omitempty" jsonschema:"logical task group name for batch branch workflows"`
		Force     bool   `json:"force,omitempty" jsonschema:"bypass duplicate detection"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "todo_tech_add",
		Description: "Add a task to an issue or feature",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in todoTechAddInput) (*mcp.CallToolResult, any, error) {
		addBody := dxclient.AddTaskRequest{
			Slug: slug,
			Text: in.Text,
		}
		if in.Feature != "" {
			addBody.Feature = &in.Feature
		}
		if in.Issue != "" {
			addBody.Issue = &in.Issue
		}
		if in.TaskGroup != "" {
			addBody.TaskGroup = &in.TaskGroup
		}
		if in.Force {
			addBody.Force = &in.Force
		}
		resp, err := c.AddTaskWithResponse(ctx, addBody)
		if err != nil {
			return nil, nil, err
		}
		if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
			return nil, nil, err
		}
		if resp.JSON200 == nil {
			return nil, nil, fmt.Errorf("empty response")
		}
		body := *resp.JSON200
		if body.DuplicateBlocked != nil && *body.DuplicateBlocked {
			return nil, map[string]any{
				"duplicate_blocked": true,
				"similar":           body.Similar,
				"message":           "near-duplicate tasks found (>85% similarity); re-run with force=true to override",
			}, nil
		}
		result := map[string]any{"task": body}
		if body.Similar != nil && len(*body.Similar) > 0 {
			result["similar"] = body.Similar
		}
		return nil, result, nil
	})

	// ── feature tools ────────────────────────────────────────────────────────

	type featureListInput struct{}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "feature_list",
		Description: "List all features",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in featureListInput) (*mcp.CallToolResult, any, error) {
		resp, err := c.ListFeaturesWithResponse(ctx, &dxclient.ListFeaturesParams{Slug: slug})
		if err != nil {
			return nil, nil, err
		}
		if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
			return nil, nil, err
		}
		var features []dxclient.FeatureItem
		if resp.JSON200 != nil && resp.JSON200.Features != nil {
			features = *resp.JSON200.Features
		}
		return nil, map[string]any{"features": features}, nil
	})

	type featureShowInput struct {
		Name string `json:"name" jsonschema:"required,feature name"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "feature_show",
		Description: "Show feature detail with specs",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in featureShowInput) (*mcp.CallToolResult, any, error) {
		resp, err := c.ListFeaturesWithResponse(ctx, &dxclient.ListFeaturesParams{Slug: slug})
		if err != nil {
			return nil, nil, err
		}
		if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
			return nil, nil, err
		}
		if resp.JSON200 != nil && resp.JSON200.Features != nil {
			for _, f := range *resp.JSON200.Features {
				if f.Name == in.Name {
					return nil, f, nil
				}
			}
		}
		return nil, nil, fmt.Errorf("feature %q not found", in.Name)
	})

	// ── pattern tools ────────────────────────────────────────────────────────

	type patternSearchInput struct {
		Text string `json:"text" jsonschema:"required,natural-language query to find similar patterns"`
		N    int    `json:"n,omitempty" jsonschema:"max results (default 5)"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "pattern_search",
		Description: "Semantic similarity search over the project's pattern library. Returns the top-N patterns matching a natural-language query.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in patternSearchInput) (*mcp.CallToolResult, any, error) {
		n := in.N
		if n <= 0 {
			n = 5
		}
		nInt := int64(n)
		resp, err := c.SimilarPatternsWithResponse(ctx, dxclient.SimilarPatternsRequest{
			Slug: slug,
			Text: in.Text,
			N:    &nInt,
		})
		if err != nil {
			return nil, nil, err
		}
		if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
			return nil, nil, err
		}
		if resp.JSON200 == nil {
			return nil, map[string]any{"patterns": nil}, nil
		}
		return nil, resp.JSON200, nil
	})

	type patternRefineInput struct {
		ID          int32           `json:"id" jsonschema:"required,pattern ID to refine"`
		Name        string          `json:"name,omitempty" jsonschema:"updated name"`
		Description string          `json:"description,omitempty" jsonschema:"updated description with added detail/clarifications"`
		CodeRefs    json.RawMessage `json:"code_refs,omitempty" jsonschema:"updated code refs array"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "pattern_refine",
		Description: "Refine a pattern by updating its name, description, or code refs to be more helpful in future searches",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in patternRefineInput) (*mcp.CallToolResult, any, error) {
		body := dxclient.UpdatePatternRequest{
			Slug:        slug,
			Id:          in.ID,
			Name:        in.Name,
			Description: in.Description,
		}
		if len(in.CodeRefs) > 0 {
			body.CodeRefs = in.CodeRefs
		}
		resp, err := c.UpdatePatternWithResponse(ctx, body)
		if err != nil {
			return nil, nil, err
		}
		if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
			return nil, nil, err
		}
		return nil, resp.JSON200, nil
	})

	// ── question tools ───────────────────────────────────────────────────────

	type questionSearchInput struct {
		Text string `json:"text" jsonschema:"required,natural-language query to find similar questions"`
		N    int    `json:"n,omitempty" jsonschema:"max results (default 5)"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "question_search",
		Description: "Semantic similarity search over the project's Q&A knowledge base. Returns the top-N questions (with answers) matching a natural-language query.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in questionSearchInput) (*mcp.CallToolResult, any, error) {
		n := in.N
		if n <= 0 {
			n = 5
		}
		nInt := int64(n)
		resp, err := c.SimilarQuestionsWithResponse(ctx, dxclient.SimilarQuestionsRequest{
			Slug: slug,
			Text: in.Text,
			N:    &nInt,
		})
		if err != nil {
			return nil, nil, err
		}
		if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
			return nil, nil, err
		}
		if resp.JSON200 == nil {
			return nil, map[string]any{"questions": nil}, nil
		}
		return nil, resp.JSON200, nil
	})

	type questionAddInput struct {
		Category string `json:"category" jsonschema:"required,question category"`
		Question string `json:"question" jsonschema:"required,the question text"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "question_add",
		Description: "Add a new question to the project's Q&A knowledge base",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in questionAddInput) (*mcp.CallToolResult, any, error) {
		resp, err := c.AddQuestionWithResponse(ctx, dxclient.AddQuestionRequest{
			Slug:     slug,
			Category: in.Category,
			Question: in.Question,
		})
		if err != nil {
			return nil, nil, err
		}
		if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
			return nil, nil, err
		}
		return nil, resp.JSON200, nil
	})

	// ── comment tools ────────────────────────────────────────────────────────

	type commentListInput struct {
		TargetType string `json:"target_type" jsonschema:"required,target type (issue, task, feature)"`
		TargetID   string `json:"target_id" jsonschema:"required,target ID (IS-N, TK-N, or feature name)"`
		Role       string `json:"role,omitempty" jsonschema:"show read/unread indicator for this role (e.g. llm, dev)"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "comment_list",
		Description: "List comments on an issue, task, or feature",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in commentListInput) (*mcp.CallToolResult, any, error) {
		params := &dxclient.ListCommentsParams{
			Slug:       &slug,
			TargetType: &in.TargetType,
			TargetId:   &in.TargetID,
		}
		if in.Role != "" {
			params.Role = &in.Role
		}
		resp, err := c.ListCommentsWithResponse(ctx, params)
		if err != nil {
			return nil, nil, err
		}
		if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
			return nil, nil, err
		}
		var comments []dxclient.CommentItem
		if resp.JSON200 != nil && resp.JSON200.Comments != nil {
			comments = *resp.JSON200.Comments
		}
		return nil, map[string]any{"comments": comments}, nil
	})

	type commentAddInput struct {
		TargetType string `json:"target_type" jsonschema:"required,target type (issue, task, feature)"`
		TargetID   string `json:"target_id" jsonschema:"required,target ID (IS-N, TK-N, or feature name)"`
		Body       string `json:"body" jsonschema:"required,comment body"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "comment_add",
		Description: "Add a comment to an issue, task, or feature",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in commentAddInput) (*mcp.CallToolResult, any, error) {
		resp, err := c.AddCommentWithResponse(ctx, dxclient.AddCommentRequest{
			Slug:       slug,
			TargetType: in.TargetType,
			TargetId:   in.TargetID,
			Body:       in.Body,
		})
		if err != nil {
			return nil, nil, err
		}
		if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
			return nil, nil, err
		}
		return nil, resp.JSON200, nil
	})

	// ── proposal tools ───────────────────────────────────────────────────────

	type proposalAddInput struct {
		Title      string  `json:"title" jsonschema:"required,short headline for the proposal"`
		Body       string  `json:"body" jsonschema:"required,full description: what happened, what the tool said, what should have happened instead"`
		SourceType string  `json:"source_type,omitempty" jsonschema:"session-review, maturity, or historical (default: session-review)"`
		SourceRef  *string `json:"source_ref,omitempty" jsonschema:"optional reference (issue ID, task ID, URL, etc.)"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "proposal_add",
		Description: "File a DX friction item or improvement idea for operator review. Use this instead of issue_add when the improvement is not a confirmed bug — the operator will review, refine, and promote it to a real issue if appropriate.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in proposalAddInput) (*mcp.CallToolResult, any, error) {
		sourceType := in.SourceType
		if sourceType == "" {
			sourceType = "session-review"
		}
		body := dxclient.CreateProposalRequest{
			Slug:       slug,
			Title:      in.Title,
			Body:       in.Body,
			SourceType: sourceType,
			SourceRef:  in.SourceRef,
		}
		resp, err := c.CreateProposalWithResponse(ctx, body)
		if err != nil {
			return nil, nil, err
		}
		if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
			return nil, nil, err
		}
		if resp.JSON200 == nil {
			return nil, nil, fmt.Errorf("empty response")
		}
		return nil, map[string]any{
			"proposal": resp.JSON200,
			"guidance": "Proposal filed for operator review. Do NOT file a follow-up issue_add for this item.",
		}, nil
	})
}

func runDX(args ...string) (string, error) {
	cmd := exec.Command("dx", args...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	return buf.String(), err
}

func parseIssueNum(id string) (int32, error) {
	if len(id) < 4 || id[:3] != "IS-" {
		return 0, fmt.Errorf("invalid issue ID %q (expected IS-N)", id)
	}
	n, err := strconv.ParseInt(id[3:], 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid issue ID %q: %w", id, err)
	}
	return int32(n), nil
}

func parseTaskNum(id string) (int32, error) {
	if len(id) < 4 || id[:3] != "TK-" {
		return 0, fmt.Errorf("invalid task ID %q (expected TK-N)", id)
	}
	n, err := strconv.ParseInt(id[3:], 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid task ID %q: %w", id, err)
	}
	return int32(n), nil
}
