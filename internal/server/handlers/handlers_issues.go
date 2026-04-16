package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/iodesystems/zdx-go/internal/db"
)

func (h *Handler) registerIssueRoutes(api huma.API) {
	type IssueIntIDInput struct {
		ID int32 `json:"id"`
	}

	huma.Register(api, huma.Operation{OperationID: "list-issues", Method: http.MethodGet, Path: "/api/dx/todo/issue/list"},
		func(ctx context.Context, in *PaginatedSlugInput) (*struct {
			Body struct {
				Issues []IssueItem `json:"issues"`
				Total  int64       `json:"total"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			total, _ := h.Q.CountIssues(ctx, p.ID)
			limit, offset := parsePage(in.Limit, in.Offset)
			rows, err := h.Q.ListIssuesPaginated(ctx, db.ListIssuesPaginatedParams{ProjectID: p.ID, Limit: limit, Offset: offset})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]IssueItem, len(rows))
			for i, r := range rows {
				out[i] = toIssueItem(r)
			}
			return &struct {
				Body struct {
					Issues []IssueItem `json:"issues"`
					Total  int64       `json:"total"`
				}
			}{Body: struct {
				Issues []IssueItem `json:"issues"`
				Total  int64       `json:"total"`
			}{Issues: out, Total: total}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "search-issues", Method: http.MethodGet, Path: "/api/dx/todo/issue/search"},
		func(ctx context.Context, in *struct {
			Slug string `query:"slug" required:"true"`
			Q    string `query:"q" required:"true"`
		}) (*struct {
			Body struct {
				Issues []IssueItem `json:"issues"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			rows, err := h.Q.SearchIssues(ctx, db.SearchIssuesParams{ProjectID: p.ID, Query: in.Q})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]IssueItem, len(rows))
			for i, r := range rows {
				out[i] = toIssueItem(r)
			}
			return &struct {
				Body struct {
					Issues []IssueItem `json:"issues"`
				}
			}{
				Body: struct {
					Issues []IssueItem `json:"issues"`
				}{Issues: out},
			}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "show-issue", Method: http.MethodGet, Path: "/api/dx/todo/issue/show"},
		func(ctx context.Context, in *struct {
			Slug string `query:"slug" required:"true"`
			ID   string `query:"id" required:"true"`
		}) (*struct {
			Body struct {
				Issue IssueItem       `json:"issue"`
				Work  []IssueWorkItem `json:"work"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			issueID := issueIDFromInt(intFromPrefixed(in.ID, "IS-"))
			row, err := h.Q.GetIssue(ctx, db.GetIssueParams{ProjectID: p.ID, ID: issueID})
			if err != nil {
				return nil, apiErr(http.StatusNotFound, "issue not found: "+in.ID)
			}
			work, _ := h.Q.GetIssueWork(ctx, issueID)
			workItems := make([]IssueWorkItem, len(work))
			for i, w := range work {
				workItems[i] = IssueWorkItem{Agent: w.Agent, Note: w.Note, CreatedAt: fmtTS(w.CreatedAt)}
			}
			type respBody = struct {
				Issue IssueItem       `json:"issue"`
				Work  []IssueWorkItem `json:"work"`
			}
			return &struct{ Body respBody }{Body: respBody{Issue: h.toIssueItemWithBlockers(ctx, row), Work: workItems}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "add-issue", Method: http.MethodPost, Path: "/api/dx/todo/issue/add"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug          string   `json:"slug"`
				Title         *string  `json:"title,omitempty"`
				Source        *string  `json:"source,omitempty"`
				Context       *string  `json:"context,omitempty"`
				BlockedBy     []string `json:"blocked_by,omitempty"`
				Component     *string  `json:"component,omitempty"`
				IssueType     *string  `json:"issue_type,omitempty"`
				ScreenshotIDs []int32  `json:"screenshot_ids,omitempty"`
				AutoReady     bool     `json:"auto_ready,omitempty"`
				URL           *string  `json:"url,omitempty"`
				SourceErrorID *int64   `json:"source_error_id,omitempty"`
			}
		}) (*struct {
			Body struct {
				IssueItem
				Similar []SimilarIssueItem `json:"similar,omitempty"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			id, err := h.Q.NextIssueID(ctx)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			status := "wip"
			if in.Body.AutoReady {
				status = "open"
			}
			params := db.CreateIssueParams{
				ID:        id,
				ProjectID: p.ID,
				IssueType: "ops",
				Status:    status,
			}
			if in.Body.Title != nil {
				params.Title = *in.Body.Title
			}
			if in.Body.Context != nil {
				params.Context = *in.Body.Context
			}
			if in.Body.Component != nil {
				params.Component = *in.Body.Component
			}
			if in.Body.IssueType != nil {
				params.IssueType = *in.Body.IssueType
			}
			if in.Body.URL != nil {
				params.Url = *in.Body.URL
			}
			if in.Body.SourceErrorID != nil {
				params.SourceErrorID = pgtype.Int8{Int64: *in.Body.SourceErrorID, Valid: true}
			}

			issueText := params.Title + " " + params.Context
			var similar []SimilarIssueItem
			if !in.Body.AutoReady {
				similar, _ = h.findSimilarIssues(ctx, p.ID, issueText, 5)
			}

			row, err := h.Q.CreateIssue(ctx, params)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			for _, blockerID := range in.Body.BlockedBy {
				_ = h.Q.AddIssueBlock(ctx, db.AddIssueBlockParams{IssueID: row.ID, BlockedByID: blockerID})
			}
			for _, fid := range in.Body.ScreenshotIDs {
				_ = h.Q.AttachFileToIssue(ctx, db.AttachFileToIssueParams{
					IssueID: row.ID,
					FileID:  fid,
					Kind:    "screenshot",
				})
			}
			go h.Emb.UpsertIssue(context.Background(), p.ID, row.ID, issueText)
			item := toIssueItem(row)
			h.Broker.PublishIssue(in.Body.Slug, row.ID, "issue.created", item)
			return &struct {
				Body struct {
					IssueItem
					Similar []SimilarIssueItem `json:"similar,omitempty"`
				}
			}{Body: struct {
				IssueItem
				Similar []SimilarIssueItem `json:"similar,omitempty"`
			}{IssueItem: item, Similar: similar}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "triage-issue", Method: http.MethodPost, Path: "/api/dx/todo/owner/triage"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug      string  `json:"slug"`
				ID        int32   `json:"id"`
				Priority  int32   `json:"priority"`
				Title     *string `json:"title,omitempty"`
				IssueType *string `json:"issue_type,omitempty"`
				Context   *string `json:"context,omitempty"`
				ThemeIDs  []int32 `json:"theme_ids,omitempty"`
				GoalIDs   []int32 `json:"goal_ids,omitempty"`
			}
		}) (*struct{ Body OKBody }, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			issueID := issueIDFromInt(in.Body.ID)
			// Capture old values for revision log
			oldIssue, _ := h.Q.GetIssue(ctx, db.GetIssueParams{ProjectID: p.ID, ID: issueID})
			newPriority := strconv.Itoa(int(in.Body.Priority))
			if err := h.Q.SetIssuePriority(ctx, db.SetIssuePriorityParams{
				ID:        issueID,
				Priority:  newPriority,
				ProjectID: p.ID,
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			if oldIssue.Priority != newPriority {
				h.recordRevision(ctx, p.ID, "issue", issueID, "priority", oldIssue.Priority, newPriority)
			}
			for field, val := range map[string]*string{
				"title":      in.Body.Title,
				"issue_type": in.Body.IssueType,
				"context":    in.Body.Context,
			} {
				if val != nil && *val != "" {
					if err := h.Q.SetIssueField(ctx, db.SetIssueFieldParams{
						Field:     field,
						Value:     *val,
						ProjectID: p.ID,
						ID:        issueID,
					}); err != nil {
						return nil, apiErr(500, err.Error())
					}
					oldVal := ""
					switch field {
					case "title":
						oldVal = oldIssue.Title
					case "issue_type":
						oldVal = oldIssue.IssueType
					case "context":
						oldVal = oldIssue.Context
					}
					h.recordRevision(ctx, p.ID, "issue", issueID, field, oldVal, *val)
				}
			}
			for _, tid := range in.Body.ThemeIDs {
				_ = h.Q.AddFocusBlocker(ctx, db.AddFocusBlockerParams{FocusID: tid, IssueID: issueID})
			}
			for _, gid := range in.Body.GoalIDs {
				_ = h.Q.LinkGoalIssue(ctx, db.LinkGoalIssueParams{GoalID: gid, IssueID: issueID})
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "edit-issue", Method: http.MethodPost, Path: "/api/dx/todo/issue/edit"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug      string  `json:"slug"`
				ID        int32   `json:"id"`
				Title     *string `json:"title,omitempty"`
				Context   *string `json:"context,omitempty"`
				Priority  *int32  `json:"priority,omitempty"`
				IssueType *string `json:"issue_type,omitempty"`
				Component *string `json:"component,omitempty"`
				URL       *string `json:"url,omitempty"`
			}
		}) (*struct{ Body OKBody }, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			issueID := issueIDFromInt(in.Body.ID)
			oldIssue, _ := h.Q.GetIssue(ctx, db.GetIssueParams{ProjectID: p.ID, ID: issueID})

			if in.Body.Priority != nil {
				newPriority := strconv.Itoa(int(*in.Body.Priority))
				if err := h.Q.SetIssuePriority(ctx, db.SetIssuePriorityParams{
					ID:        issueID,
					Priority:  newPriority,
					ProjectID: p.ID,
				}); err != nil {
					return nil, apiErr(500, err.Error())
				}
				if oldIssue.Priority != newPriority {
					h.recordRevision(ctx, p.ID, "issue", issueID, "priority", oldIssue.Priority, newPriority)
				}
			}

			fieldMap := map[string]*string{
				"title":      in.Body.Title,
				"issue_type": in.Body.IssueType,
				"context":    in.Body.Context,
				"component":  in.Body.Component,
				"url":        in.Body.URL,
			}
			oldValMap := map[string]string{
				"title":      oldIssue.Title,
				"issue_type": oldIssue.IssueType,
				"context":    oldIssue.Context,
				"component":  oldIssue.Component,
				"url":        oldIssue.Url,
			}
			for field, val := range fieldMap {
				if val == nil {
					continue
				}
				if err := h.Q.SetIssueField(ctx, db.SetIssueFieldParams{
					Field:     field,
					Value:     *val,
					ProjectID: p.ID,
					ID:        issueID,
				}); err != nil {
					return nil, apiErr(500, err.Error())
				}
				if oldValMap[field] != *val {
					h.recordRevision(ctx, p.ID, "issue", issueID, field, oldValMap[field], *val)
				}
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "close-issue", Method: http.MethodPost, Path: "/api/dx/todo/issue/close"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug        string  `json:"slug"`
				ID          int32   `json:"id"`
				Reason      *string `json:"reason,omitempty"`
				Notes       *string `json:"notes,omitempty"`
				DuplicateOf *string `json:"duplicate_of,omitempty"`
			}
		}) (*struct{ Body OKBody }, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			issueID := issueIDFromInt(in.Body.ID)
			reason := ptrStr(in.Body.Reason)
			duplicateOf := ptrStr(in.Body.DuplicateOf)
			if reason == "duplicate" && duplicateOf == "" {
				return nil, apiErr(400, "duplicate_of is required when reason is duplicate")
			}
			agent := ""
			if uid := ctxUserIDVal(ctx); uid != 0 {
				if u, uErr := h.Q.GetUserByID(ctx, uid); uErr == nil {
					agent = u.Email
				}
			}
			if reason == "done" || reason == "" {
				issue, gErr := h.Q.GetIssue(ctx, db.GetIssueParams{ProjectID: p.ID, ID: issueID})
				if gErr == nil && issue.IssueType == "impl" {
					count, _ := h.Q.CountIssueResolutions(ctx, issueID)
					if count == 0 {
						return nil, apiErr(400, "impl issues require at least one resolution before closing; use 'dx issue resolve' first")
					}
				}
			}
			prevStatus := "open"
			if issue, gErr := h.Q.GetIssue(ctx, db.GetIssueParams{ProjectID: p.ID, ID: issueID}); gErr == nil {
				prevStatus = issue.Status
			}
			if err := h.Q.CloseIssue(ctx, db.CloseIssueParams{ProjectID: p.ID, ID: issueID, DuplicateOf: duplicateOf}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			h.recordRevision(ctx, p.ID, "issue", issueID, "status", prevStatus, "closed")
			notes := ptrStr(in.Body.Notes)
			note := "[closed"
			if reason != "" {
				note += ":" + reason
			}
			note += "] "
			if duplicateOf != "" {
				note += "duplicate of " + duplicateOf + " "
			}
			if notes != "" {
				note += notes
			}
			_ = h.Q.AppendIssueWork(ctx, db.AppendIssueWorkParams{IssueID: issueID, Agent: agent, Note: note})
			h.Broker.PublishIssue(in.Body.Slug, issueID, "issue.closed", map[string]any{"id": issueID, "reason": reason})
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "reopen-issue", Method: http.MethodPost, Path: "/api/dx/todo/issue/reopen"},
		func(ctx context.Context, in *struct {
			Body IssueIntIDInput
		}) (*struct{ Body OKBody }, error) {
			issueID := issueIDFromInt(in.Body.ID)
			prevStatus := ""
			var projectID int32
			if issue, gErr := h.Q.GetIssueByAnyProject(ctx, issueID); gErr == nil {
				prevStatus = issue.Status
				projectID = issue.ProjectID
			}
			_ = h.Q.ReopenIssue(ctx, db.ReopenIssueParams{ID: issueID, ProjectID: 0})
			if projectID != 0 {
				h.recordRevision(ctx, projectID, "issue", issueID, "status", prevStatus, "open")
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "update-issue", Method: http.MethodPost, Path: "/api/dx/todo/issue/update"},
		func(ctx context.Context, in *struct {
			Body struct {
				ID    int32  `json:"id"`
				Field string `json:"field"`
				Value string `json:"value"`
			}
		}) (*struct{ Body OKBody }, error) {
			issueID := issueIDFromInt(in.Body.ID)
			err := h.Q.SetIssueField(ctx, db.SetIssueFieldParams{
				Field: in.Body.Field,
				Value: in.Body.Value,
				ID:    issueID,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "issue-kind", Method: http.MethodPost, Path: "/api/dx/todo/issue/kind"},
		func(ctx context.Context, in *struct {
			Body struct {
				ID   int32  `json:"id"`
				Kind string `json:"kind"`
			}
		}) (*struct{ Body OKBody }, error) {
			issueID := issueIDFromInt(in.Body.ID)
			err := h.Q.SetIssueField(ctx, db.SetIssueFieldParams{
				Field: "component",
				Value: in.Body.Kind,
				ID:    issueID,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "issue-add-block", Method: http.MethodPost, Path: "/api/dx/todo/issue/add-block"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug      string `json:"slug"`
				ID        int32  `json:"id"`
				BlockedBy string `json:"blocked_by"`
			}
		}) (*struct{ Body OKBody }, error) {
			_, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			issueID := issueIDFromInt(in.Body.ID)
			if err := h.Q.AddIssueBlock(ctx, db.AddIssueBlockParams{IssueID: issueID, BlockedByID: in.Body.BlockedBy}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "issue-remove-block", Method: http.MethodPost, Path: "/api/dx/todo/issue/remove-block"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug      string `json:"slug"`
				ID        int32  `json:"id"`
				BlockedBy string `json:"blocked_by,omitempty"`
			}
		}) (*struct{ Body OKBody }, error) {
			_, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			issueID := issueIDFromInt(in.Body.ID)
			if in.Body.BlockedBy == "" {
				if err := h.Q.RemoveAllIssueBlocks(ctx, issueID); err != nil {
					return nil, apiErr(500, err.Error())
				}
			} else {
				if err := h.Q.RemoveIssueBlock(ctx, db.RemoveIssueBlockParams{IssueID: issueID, BlockedByID: in.Body.BlockedBy}); err != nil {
					return nil, apiErr(500, err.Error())
				}
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "issue-list-blockers", Method: http.MethodGet, Path: "/api/dx/todo/issue/blockers"},
		func(ctx context.Context, in *struct {
			Slug string `query:"slug" required:"true"`
			ID   string `query:"id" required:"true"`
		}) (*struct {
			Body struct {
				Blockers []string `json:"blockers"`
			}
		}, error) {
			_, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			issueID := issueIDFromInt(intFromPrefixed(in.ID, "IS-"))
			rows, err := h.Q.ListIssueBlockers(ctx, issueID)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			type respBody = struct {
				Blockers []string `json:"blockers"`
			}
			return &struct{ Body respBody }{Body: respBody{Blockers: rows}}, nil
		})

	// set-features is a no-op on the Go side — features are linked via task.issue
	huma.Register(api, huma.Operation{OperationID: "issue-set-features", Method: http.MethodPost, Path: "/api/dx/todo/issue/set-features"},
		func(ctx context.Context, in *struct {
			Body struct {
				ID       int32  `json:"id"`
				Features string `json:"features"`
			}
		}) (*struct{ Body OKBody }, error) {
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	// Issue work
	huma.Register(api, huma.Operation{OperationID: "append-issue-work", Method: http.MethodPost, Path: "/api/issue-work"},
		func(ctx context.Context, in *struct {
			Body struct {
				IssueID   int32   `json:"issue_id"`
				EntryType *string `json:"entry_type,omitempty"`
				ByRole    *string `json:"by_role,omitempty"`
				Note      string  `json:"note"`
			}
		}) (*struct{ Body OKBody }, error) {
			issueID := issueIDFromInt(in.Body.IssueID)
			note := in.Body.Note
			if in.Body.EntryType != nil && *in.Body.EntryType != "" {
				note = "[" + *in.Body.EntryType + "] " + note
			}
			agent := ""
			if in.Body.ByRole != nil {
				agent = *in.Body.ByRole
			}
			if err := h.Q.AppendIssueWork(ctx, db.AppendIssueWorkParams{
				IssueID: issueID,
				Agent:   agent,
				Note:    note,
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-worklog", Method: http.MethodGet, Path: "/api/dx/worklog"},
		func(ctx context.Context, in *PaginatedSlugInput) (*struct {
			Body struct {
				Entries []struct {
					IssueID    string `json:"issue_id"`
					IssueTitle string `json:"issue_title"`
					Agent      string `json:"agent"`
					Note       string `json:"note"`
					CreatedAt  string `json:"created_at"`
				} `json:"entries"`
				Total int64 `json:"total"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			total, _ := h.Q.CountWorklogForProject(ctx, p.ID)
			limit, offset := parsePage(in.Limit, in.Offset)
			rows, err := h.Q.ListWorklogForProjectPaginated(ctx, db.ListWorklogForProjectPaginatedParams{ProjectID: p.ID, Limit: limit, Offset: offset})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			type entry = struct {
				IssueID    string `json:"issue_id"`
				IssueTitle string `json:"issue_title"`
				Agent      string `json:"agent"`
				Note       string `json:"note"`
				CreatedAt  string `json:"created_at"`
			}
			type respBody = struct {
				Entries []entry `json:"entries"`
				Total   int64   `json:"total"`
			}
			out := make([]entry, len(rows))
			for i, r := range rows {
				out[i] = entry{IssueID: r.IssueID, IssueTitle: r.IssueTitle, Agent: r.Agent, Note: r.Note, CreatedAt: fmtTS(r.CreatedAt)}
			}
			return &struct{ Body respBody }{Body: respBody{Entries: out, Total: total}}, nil
		})

	// ── Issue similarity ───────────────────────────────────────────────────────

	huma.Register(api, huma.Operation{OperationID: "similar-issues", Method: http.MethodPost, Path: "/api/dx/issues/similar"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug string `json:"slug"`
				Text string `json:"text"`
				N    int    `json:"n,omitempty"`
			}
		}) (*struct {
			Body struct {
				Issues []SimilarIssueItem `json:"issues"`
			}
		}, error) {
			n := in.Body.N
			if n <= 0 {
				n = 5
			}
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			results, err := h.findSimilarIssues(ctx, p.ID, in.Body.Text, n)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct {
				Body struct {
					Issues []SimilarIssueItem `json:"issues"`
				}
			}{Body: struct {
				Issues []SimilarIssueItem `json:"issues"`
			}{Issues: results}}, nil
		})

	// ── Issue ready (wip → open) ─────────────────────────────────────────────

	huma.Register(api, huma.Operation{OperationID: "ready-issue", Method: http.MethodPost, Path: "/api/dx/todo/issue/ready"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug string `json:"slug"`
				ID   int32  `json:"id"`
			}
		}) (*struct{ Body struct{ OK bool } }, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			issueID := issueIDFromInt(in.Body.ID)
			if err := h.Q.ReadyIssue(ctx, db.ReadyIssueParams{ProjectID: p.ID, ID: issueID}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			h.recordRevision(ctx, p.ID, "issue", issueID, "status", "wip", "open")
			return &struct{ Body struct{ OK bool } }{Body: struct{ OK bool }{OK: true}}, nil
		})

	// ── Git commits ───────────────────────────────────────────────────────────

	huma.Register(api, huma.Operation{OperationID: "list-git-commits", Method: http.MethodGet, Path: "/api/dx/git/commits"},
		func(ctx context.Context, in *struct {
			Slug string `query:"slug" required:"true"`
			N    int    `query:"n"`
		}) (*struct {
			Body struct {
				Commits []GitCommit `json:"commits"`
			}
		}, error) {
			if !h.Features.HasProjectGitConfig {
				return &struct {
					Body struct {
						Commits []GitCommit `json:"commits"`
					}
				}{}, nil
			}
			n := in.N
			if n <= 0 || n > 100 {
				n = 20
			}
			row, err := h.Q.GetProjectGitConfig(ctx, in.Slug)
			if err != nil {
				return nil, apiErr(http.StatusNotFound, "project not found: "+in.Slug)
			}
			if row.GitUrl == "" {
				return nil, apiErr(http.StatusUnprocessableEntity, "git URL not configured for project "+in.Slug)
			}
			// Embed token into URL for HTTPS auth if provided.
			gitURL := row.GitUrl
			if row.GitToken != "" && strings.HasPrefix(gitURL, "https://") {
				gitURL = "https://" + row.GitToken + "@" + strings.TrimPrefix(gitURL, "https://")
			}
			dir := h.Reconciler.RepoDir(in.Slug)
			branch := row.GitBranch
			if branch == "" {
				branch = "main"
			}
			if err := EnsureRepo(dir, gitURL, branch); err != nil {
				return nil, apiErr(500, "git: "+err.Error())
			}
			commits, err := RecentCommits(dir, branch, n)
			if err != nil {
				return nil, apiErr(500, "git: "+err.Error())
			}
			return &struct {
				Body struct {
					Commits []GitCommit `json:"commits"`
				}
			}{Body: struct {
				Commits []GitCommit `json:"commits"`
			}{Commits: commits}}, nil
		})

	// ── Issue resolutions ─────────────────────────────────────────────────────

	type ResolutionCommitItem struct {
		SHA string `json:"sha"`
		Ord int32  `json:"ord"`
	}

	type ResolutionItem struct {
		ID               string                 `json:"id"`
		IssueID          string                 `json:"issue_id"`
		BranchOfOrigin   string                 `json:"branch_of_origin"`
		ResolvedAt       string                 `json:"resolved_at"`
		Author           string                 `json:"author"`
		Source           string                 `json:"source"`
		ParentResolution string                 `json:"parent_resolution,omitempty"`
		Commits          []ResolutionCommitItem `json:"commits"`
		Reverted         bool                   `json:"reverted"`
		RevertedBy       string                 `json:"reverted_by,omitempty"`
	}

	huma.Register(api, huma.Operation{OperationID: "resolve-issue", Method: http.MethodPost, Path: "/api/dx/todo/issue/resolve"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug        string   `json:"slug"`
				ID          int32    `json:"id"`
				SHAs        []string `json:"shas"`
				Branch      string   `json:"branch,omitempty"`
				Author      string   `json:"author,omitempty"`
				Source      string   `json:"source,omitempty"`
				ParentResID string   `json:"parent_resolution_id,omitempty"`
			}
		}) (*struct {
			Body struct {
				Resolution ResolutionItem `json:"resolution"`
			}
		}, error) {
			if len(in.Body.SHAs) == 0 {
				return nil, apiErr(400, "at least one SHA is required")
			}
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			issueID := issueIDFromInt(in.Body.ID)
			if _, err := h.Q.GetIssue(ctx, db.GetIssueParams{ProjectID: p.ID, ID: issueID}); err != nil {
				return nil, apiErr(http.StatusNotFound, "issue not found")
			}
			source := in.Body.Source
			if source == "" {
				source = "manual"
			}
			branch := in.Body.Branch
			author := in.Body.Author
			if author == "" {
				if uid := ctxUserIDVal(ctx); uid != 0 {
					if u, uErr := h.Q.GetUserByID(ctx, uid); uErr == nil {
						author = u.Email
					}
				}
			}
			var rnd [8]byte
			_, _ = rand.Read(rnd[:])
			resID := hex.EncodeToString(rnd[:])
			var parentRes pgtype.Text
			if in.Body.ParentResID != "" {
				parentRes = pgtype.Text{String: in.Body.ParentResID, Valid: true}
			}
			now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
			res, err := h.Q.CreateIssueResolution(ctx, db.CreateIssueResolutionParams{
				ID:                 resID,
				IssueID:            issueID,
				BranchOfOrigin:     branch,
				ResolvedAt:         now,
				Author:             author,
				Source:             source,
				ParentResolutionID: parentRes,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			commits := make([]ResolutionCommitItem, len(in.Body.SHAs))
			for i, sha := range in.Body.SHAs {
				if err := h.Q.AddResolutionCommit(ctx, db.AddResolutionCommitParams{
					ResolutionID: resID,
					Sha:          sha,
					Ord:          int32(i),
				}); err != nil {
					return nil, apiErr(500, err.Error())
				}
				commits[i] = ResolutionCommitItem{SHA: sha, Ord: int32(i)}
			}
			parentStr := ""
			if res.ParentResolutionID.Valid {
				parentStr = res.ParentResolutionID.String
			}
			item := ResolutionItem{
				ID:               res.ID,
				IssueID:          fmt.Sprintf("IS-%d", in.Body.ID),
				BranchOfOrigin:   res.BranchOfOrigin,
				ResolvedAt:       fmtTS(res.ResolvedAt),
				Author:           res.Author,
				Source:           res.Source,
				ParentResolution: parentStr,
				Commits:          commits,
			}
			return &struct {
				Body struct {
					Resolution ResolutionItem `json:"resolution"`
				}
			}{Body: struct {
				Resolution ResolutionItem `json:"resolution"`
			}{Resolution: item}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-issue-resolutions", Method: http.MethodGet, Path: "/api/dx/todo/issue/resolutions"},
		func(ctx context.Context, in *struct {
			Slug   string `query:"slug" required:"true"`
			ID     string `query:"id" required:"true"`
			Branch string `query:"branch"`
		}) (*struct {
			Body struct {
				Resolutions []ResolutionItem `json:"resolutions"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			issueID := issueIDFromInt(intFromPrefixed(in.ID, "IS-"))
			resolutions, err := h.Q.ListIssueResolutions(ctx, issueID)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}

			var repoDir, gitBranch string
			if in.Branch != "" && h.Features.HasProjectGitConfig {
				row, gErr := h.Q.GetProjectGitConfig(ctx, in.Slug)
				if gErr == nil && row.GitUrl != "" {
					gitURL := row.GitUrl
					if row.GitToken != "" && strings.HasPrefix(gitURL, "https://") {
						gitURL = "https://" + row.GitToken + "@" + strings.TrimPrefix(gitURL, "https://")
					}
					repoDir = h.Reconciler.RepoDir(in.Slug)
					gitBranch = row.GitBranch
					if gitBranch == "" {
						gitBranch = "main"
					}
					_ = EnsureRepo(repoDir, gitURL, gitBranch)
				}
			}

			items := make([]ResolutionItem, 0, len(resolutions))
			for _, r := range resolutions {
				commits, cErr := h.Q.ListResolutionCommits(ctx, r.ID)
				if cErr != nil {
					continue
				}
				cItems := make([]ResolutionCommitItem, len(commits))
				for i, c := range commits {
					cItems[i] = ResolutionCommitItem{SHA: c.Sha, Ord: c.Ord}
				}
				parentStr := ""
				if r.ParentResolutionID.Valid {
					parentStr = r.ParentResolutionID.String
				}
				item := ResolutionItem{
					ID:               r.ID,
					IssueID:          r.IssueID,
					BranchOfOrigin:   r.BranchOfOrigin,
					ResolvedAt:       fmtTS(r.ResolvedAt),
					Author:           r.Author,
					Source:           r.Source,
					ParentResolution: parentStr,
					Commits:          cItems,
				}
				if in.Branch != "" && repoDir != "" {
					shas := make([]commitSHA, len(cItems))
					for j, ci := range cItems {
						shas[j] = commitSHA{SHA: ci.SHA}
					}
					reverted, revertSHAStr := checkResolutionReverted(repoDir, in.Branch, shas)
					item.Reverted = reverted
					item.RevertedBy = revertSHAStr
				}
				items = append(items, item)
			}
			_ = p
			return &struct {
				Body struct {
					Resolutions []ResolutionItem `json:"resolutions"`
				}
			}{Body: struct {
				Resolutions []ResolutionItem `json:"resolutions"`
			}{Resolutions: items}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "delete-issue-resolution", Method: http.MethodPost, Path: "/api/dx/todo/issue/resolution/delete"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug         string `json:"slug"`
				ResolutionID string `json:"resolution_id"`
			}
		}) (*struct{ Body OKBody }, error) {
			_, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			if err := h.Q.DeleteIssueResolution(ctx, in.Body.ResolutionID); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "reconcile-branch", Method: http.MethodPost, Path: "/api/dx/todo/issue/reconcile"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug   string `json:"slug"`
				Branch string `json:"branch"`
			}
		}) (*struct {
			Body struct {
				Results []ReconcileResult `json:"results"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			results, err := h.Reconciler.ReconcileBranch(ctx, p.ID, in.Body.Slug, in.Body.Branch)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			if results == nil {
				results = []ReconcileResult{}
			}
			return &struct {
				Body struct {
					Results []ReconcileResult `json:"results"`
				}
			}{Body: struct {
				Results []ReconcileResult `json:"results"`
			}{Results: results}}, nil
		})
}

// ── Model → response converter ────────────────────────────────────────────

func toIssueItem(r db.ZdxIssue) IssueItem {
	return IssueItem{
		ID:          issueIntID(r.ID),
		Title:       r.Title,
		Status:      r.Status,
		Priority:    r.Priority,
		Component:   r.Component,
		Context:     r.Context,
		IssueType:   r.IssueType,
		DuplicateOf: r.DuplicateOf,
		URL:         r.Url,
		CreatedAt:   fmtTS(r.CreatedAt),
		UpdatedAt:   fmtTS(r.UpdatedAt),
		BlockedBy:   []string{},
	}
}

func (h *Handler) toIssueItemWithBlockers(ctx context.Context, r db.ZdxIssue) IssueItem {
	item := toIssueItem(r)
	blockers, err := h.Q.ListIssueBlockers(ctx, r.ID)
	if err == nil && len(blockers) > 0 {
		item.BlockedBy = blockers
	}
	return item
}

type commitSHA struct {
	SHA string
}

func checkResolutionReverted(repoDir, branch string, commits []commitSHA) (reverted bool, revertSHA string) {
	for _, c := range commits {
		sha := c.SHA
		if len(sha) < 7 {
			continue
		}
		cmd := exec.Command("git", "-C", repoDir, "log", "--format=%H", "--all", "--grep=This reverts commit "+sha)
		cmd.Env = GitEnv()
		out, err := cmd.Output()
		if err != nil {
			continue
		}
		revertCommit := strings.TrimSpace(string(out))
		if revertCommit == "" {
			continue
		}
		firstLine := strings.SplitN(revertCommit, "\n", 2)[0]
		isAncestor := exec.Command("git", "-C", repoDir, "merge-base", "--is-ancestor", firstLine, "origin/"+branch)
		isAncestor.Env = GitEnv()
		if isAncestor.Run() == nil {
			return true, firstLine
		}
	}
	return false, ""
}
