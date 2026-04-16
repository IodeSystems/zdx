package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os/exec"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
)

func McpCmd() *cobra.Command {
	var withFS, withShell bool
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Start MCP server (stdio transport)",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := DefaultClient()
			if err != nil {
				return err
			}
			srv := mcp.NewServer(&mcp.Implementation{
				Name:    "dx",
				Version: "0.1.0",
			}, nil)
			registerMCPTools(srv, c)
			if withFS {
				root, err := gitRepoRoot()
				if err != nil {
					return fmt.Errorf("--with-fs requires a git repo: %w", err)
				}
				RegisterFSTools(srv, root)
			}
			if withShell {
				root, err := gitRepoRoot()
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

func registerMCPTools(srv *mcp.Server, c *Client) {
	slug := c.Slug()

	// ── issue tools ──────────────────────────────────────────────────────────

	type issueListInput struct {
		Status string `json:"status,omitempty" jsonschema:"filter by status (open, closed)"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "issue_list",
		Description: "List issues, optionally filtered by status",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in issueListInput) (*mcp.CallToolResult, any, error) {
		var resp struct {
			Issues []issueItem `json:"issues"`
		}
		if err := c.get("/api/dx/todo/issue/list", url.Values{"slug": {slug}}, &resp); err != nil {
			return nil, nil, err
		}
		if in.Status != "" {
			var filtered []issueItem
			for _, iss := range resp.Issues {
				if iss.Status == in.Status {
					filtered = append(filtered, iss)
				}
			}
			resp.Issues = filtered
		}
		return nil, resp, nil
	})

	type issueAddInput struct {
		Title     string `json:"title" jsonschema:"required,issue title"`
		Context   string `json:"context,omitempty" jsonschema:"context / description"`
		Component string `json:"component,omitempty" jsonschema:"component"`
		BlockedBy string `json:"blocked_by,omitempty" jsonschema:"blocking issue (IS-N)"`
		Parent    string `json:"parent,omitempty" jsonschema:"parent issue (IS-N): new issue blocks the parent"`
		IssueType string `json:"issue_type,omitempty" jsonschema:"issue type: ops, impl, or tracker"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "issue_add",
		Description: "Create a new issue",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in issueAddInput) (*mcp.CallToolResult, any, error) {
		issType := in.IssueType
		if issType == "" {
			issType = "ops"
		}
		var iss issueItem
		if err := c.post("/api/dx/todo/issue/add", map[string]any{
			"slug":       slug,
			"title":      in.Title,
			"context":    in.Context,
			"component":  in.Component,
			"blocked_by": in.BlockedBy,
			"issue_type": issType,
		}, &iss); err != nil {
			return nil, nil, err
		}
		if in.Parent != "" {
			parentNum, _ := strconv.ParseInt(in.Parent[3:], 10, 32)
			_ = c.post("/api/dx/todo/issue/add-block", map[string]any{
				"slug":       slug,
				"id":         int32(parentNum),
				"blocked_by": issueIDStr(int32(iss.ID)),
			}, nil)
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
		var resp struct {
			Issue issueItem       `json:"issue"`
			Work  []issueWorkItem `json:"work"`
		}
		if err := c.get("/api/dx/todo/issue/show", url.Values{"slug": {slug}, "id": {in.ID}}, &resp); err != nil {
			return nil, nil, err
		}
		return nil, resp, nil
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
		var ok struct {
			OK bool `json:"ok"`
		}
		if err := c.post("/api/dx/todo/issue/close", map[string]any{
			"slug":   slug,
			"id":     n,
			"reason": in.Reason,
		}, &ok); err != nil {
			return nil, nil, err
		}
		return nil, map[string]string{"status": "closed", "id": in.ID}, nil
	})

	type issueEditInput struct {
		ID        string `json:"id" jsonschema:"required,issue ID (IS-N)"`
		Title     string `json:"title,omitempty" jsonschema:"issue title"`
		Context   string `json:"context,omitempty" jsonschema:"context / description"`
		Priority  *int   `json:"priority,omitempty" jsonschema:"priority (1-4)"`
		Component string `json:"component,omitempty" jsonschema:"component"`
		IssueType string `json:"issue_type,omitempty" jsonschema:"issue type: ops, impl, or tracker"`
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
		body := map[string]any{"slug": slug, "id": n}
		raw, _ := json.Marshal(in)
		var fields map[string]any
		json.Unmarshal(raw, &fields)
		for k, v := range fields {
			if k == "id" {
				continue
			}
			body[k] = v
		}
		var ok struct {
			OK bool `json:"ok"`
		}
		if err := c.post("/api/dx/todo/issue/edit", body, &ok); err != nil {
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
		var ok struct {
			OK bool `json:"ok"`
		}
		if err := c.post("/api/dx/todo/issue/set-blocked-by", map[string]any{
			"slug":       slug,
			"id":         n,
			"blocked_by": in.By,
		}, &ok); err != nil {
			return nil, nil, err
		}
		return nil, map[string]string{"status": "blocked", "id": in.ID, "by": in.By}, nil
	})

	type issueUnblockInput struct {
		ID string `json:"id" jsonschema:"required,issue ID (IS-N)"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "issue_unblock",
		Description: "Clear blockers on an issue",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in issueUnblockInput) (*mcp.CallToolResult, any, error) {
		n, err := parseIssueNum(in.ID)
		if err != nil {
			return nil, nil, err
		}
		var ok struct {
			OK bool `json:"ok"`
		}
		if err := c.post("/api/dx/todo/issue/set-blocked-by", map[string]any{
			"slug":       slug,
			"id":         n,
			"blocked_by": "",
		}, &ok); err != nil {
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
			var resp struct {
				Issue issueItem       `json:"issue"`
				Work  []issueWorkItem `json:"work"`
			}
			if err := c.get("/api/dx/todo/issue/show", url.Values{"slug": {slug}, "id": {id}}, &resp); err != nil {
				return nil, nil, err
			}
			return nil, resp, nil
		case len(id) > 3 && id[:3] == "TK-":
			n, _ := strconv.ParseInt(id[3:], 10, 32)
			var taskList struct {
				Tasks []taskItem `json:"tasks"`
			}
			if err := c.get("/api/tasks", url.Values{"slug": {slug}}, &taskList); err != nil {
				return nil, nil, err
			}
			for _, t := range taskList.Tasks {
				if t.ID == int32(n) {
					return nil, t, nil
				}
			}
			return nil, nil, fmt.Errorf("task %s not found", id)
		default:
			var featList struct {
				Features []featureItem `json:"features"`
			}
			if err := c.get("/api/features", url.Values{"slug": {slug}}, &featList); err != nil {
				return nil, nil, err
			}
			for _, f := range featList.Features {
				if f.Name == id {
					return nil, f, nil
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
		var taskList struct {
			Tasks []taskItem `json:"tasks"`
		}
		if in.Issue != "" {
			if err := c.get("/api/dx/todo/issue/tasks", url.Values{
				"slug":     {slug},
				"issue_id": {in.Issue},
			}, &taskList); err != nil {
				return nil, nil, err
			}
		} else if in.Feature != "" {
			if err := c.get("/api/tasks-by-feature", url.Values{
				"slug":    {slug},
				"feature": {in.Feature},
			}, &taskList); err != nil {
				return nil, nil, err
			}
		} else {
			if err := c.get("/api/tasks", url.Values{"slug": {slug}}, &taskList); err != nil {
				return nil, nil, err
			}
		}
		return nil, taskList, nil
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
		var ok struct {
			OK bool `json:"ok"`
		}
		if err := c.post("/api/dx/todo/dev/done", map[string]any{
			"id":        n,
			"test_plan": in.TestPlan,
			"test_refs": in.TestRefs,
		}, &ok); err != nil {
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
		Description: "Mark a task as in-progress",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in todoDevStartInput) (*mcp.CallToolResult, any, error) {
		n, err := parseTaskNum(in.ID)
		if err != nil {
			return nil, nil, err
		}
		var ok struct {
			OK bool `json:"ok"`
		}
		if err := c.put("/api/task-status", map[string]any{
			"id":     n,
			"status": "in_progress",
			"reason": in.Reason,
		}, &ok); err != nil {
			return nil, nil, err
		}
		return nil, map[string]string{"status": "in_progress", "id": in.ID}, nil
	})

	type todoOwnerTriageInput struct {
		ID        string `json:"id" jsonschema:"required,issue ID (IS-N)"`
		Priority  int    `json:"priority" jsonschema:"required,priority 1-4 (1=highest)"`
		Title     string `json:"title,omitempty" jsonschema:"set issue title"`
		IssueType string `json:"issue_type,omitempty" jsonschema:"issue type: ops, impl, or tracker"`
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
		body := map[string]any{
			"slug":     slug,
			"id":       n,
			"priority": int32(in.Priority),
		}
		if in.Title != "" {
			body["title"] = in.Title
		}
		if in.IssueType != "" {
			body["issue_type"] = in.IssueType
		}
		if in.Context != "" {
			body["context"] = in.Context
		}
		if len(in.ThemeIDs) > 0 {
			body["theme_ids"] = in.ThemeIDs
		}
		if len(in.GoalIDs) > 0 {
			body["goal_ids"] = in.GoalIDs
		}
		var ok struct {
			OK bool `json:"ok"`
		}
		if err := c.post("/api/dx/todo/owner/triage", body, &ok); err != nil {
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
		var resp taskAddResponse
		if err := c.post("/api/dx/todo/tech/add", map[string]any{
			"slug":       slug,
			"text":       in.Text,
			"feature":    in.Feature,
			"issue":      in.Issue,
			"task_group": in.TaskGroup,
			"force":      in.Force,
		}, &resp); err != nil {
			return nil, nil, err
		}
		if resp.DuplicateBlocked {
			return nil, map[string]any{
				"duplicate_blocked": true,
				"similar":           resp.Similar,
				"message":           "near-duplicate tasks found (>85% similarity); re-run with force=true to override",
			}, nil
		}
		result := map[string]any{"task": resp.taskItem}
		if len(resp.Similar) > 0 {
			result["similar"] = resp.Similar
		}
		return nil, result, nil
	})

	// ── feature tools ────────────────────────────────────────────────────────

	type featureListInput struct{}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "feature_list",
		Description: "List all features",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in featureListInput) (*mcp.CallToolResult, any, error) {
		var resp struct {
			Features []featureItem `json:"features"`
		}
		if err := c.get("/api/features", url.Values{"slug": {slug}}, &resp); err != nil {
			return nil, nil, err
		}
		return nil, resp, nil
	})

	type featureShowInput struct {
		Name string `json:"name" jsonschema:"required,feature name"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "feature_show",
		Description: "Show feature detail with specs",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in featureShowInput) (*mcp.CallToolResult, any, error) {
		var resp struct {
			Features []featureItem `json:"features"`
		}
		if err := c.get("/api/features", url.Values{"slug": {slug}}, &resp); err != nil {
			return nil, nil, err
		}
		for _, f := range resp.Features {
			if f.Name == in.Name {
				return nil, f, nil
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
		var resp struct {
			Patterns []struct {
				Pattern patternItem `json:"pattern"`
				Score   float64     `json:"score"`
			} `json:"patterns"`
		}
		if err := c.post("/api/dx/patterns/similar", map[string]any{
			"slug": slug,
			"text": in.Text,
			"n":    n,
		}, &resp); err != nil {
			return nil, nil, err
		}
		return nil, resp, nil
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
		body := map[string]any{"slug": slug, "id": in.ID}
		if in.Name != "" {
			body["name"] = in.Name
		}
		if in.Description != "" {
			body["description"] = in.Description
		}
		if len(in.CodeRefs) > 0 {
			body["code_refs"] = in.CodeRefs
		}
		var p patternItem
		if err := c.post("/api/dx/patterns/update", body, &p); err != nil {
			return nil, nil, err
		}
		return nil, p, nil
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
		var resp struct {
			Questions []struct {
				ID       int32   `json:"id"`
				Question string  `json:"question"`
				Answer   string  `json:"answer"`
				Score    float32 `json:"score"`
			} `json:"questions"`
		}
		if err := c.post("/api/dx/qa/similar", map[string]any{
			"slug": slug,
			"text": in.Text,
			"n":    n,
		}, &resp); err != nil {
			return nil, nil, err
		}
		return nil, resp, nil
	})

	type questionAddInput struct {
		Category string `json:"category" jsonschema:"required,question category"`
		Question string `json:"question" jsonschema:"required,the question text"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "question_add",
		Description: "Add a new question to the project's Q&A knowledge base",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in questionAddInput) (*mcp.CallToolResult, any, error) {
		var q questionItem
		if err := c.post("/api/dx/qa/add", map[string]any{
			"slug":     slug,
			"category": in.Category,
			"question": in.Question,
		}, &q); err != nil {
			return nil, nil, err
		}
		return nil, q, nil
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
		q := url.Values{
			"slug":        {slug},
			"target_type": {in.TargetType},
			"target_id":   {in.TargetID},
		}
		if in.Role != "" {
			q.Set("role", in.Role)
		}
		var resp struct {
			Comments []commentItem `json:"comments"`
		}
		if err := c.get("/api/dx/comment/list", q, &resp); err != nil {
			return nil, nil, err
		}
		return nil, resp, nil
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
		var cm commentItem
		if err := c.post("/api/dx/comment/add", map[string]any{
			"slug":        slug,
			"target_type": in.TargetType,
			"target_id":   in.TargetID,
			"body":        in.Body,
		}, &cm); err != nil {
			return nil, nil, err
		}
		return nil, cm, nil
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
