package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/iodesystems/sqlc-go-codegen-metaquery/metaquery"
	"github.com/iodesystems/sqlc-go-codegen-metaquery/metaquery/mqpgx"

	"github.com/iodesystems/zdx-go/internal/atlas/trace"
	"github.com/iodesystems/zdx-go/internal/db"
)

// issueRefRe matches an "IS-N" reference within free-text. Word-boundaries on
// either side prevent matching things like "MIS-1" or "IS-12abc".
var issueRefRe = regexp.MustCompile(`\bIS-\d+\b`)

// validNodeRef checks that ref is exactly one "kind:slug" pair with both halves
// non-empty. Soft reference — no FK lookup; the renderer resolves at read time.
func validNodeRef(ref string) bool {
	parts := strings.SplitN(ref, ":", 2)
	if len(parts) != 2 {
		return false
	}
	if strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return false
	}
	if strings.Contains(parts[1], ":") {
		return false
	}
	return true
}

// extractIssueRefs returns deduped IS-N references found in text, excluding
// excludeID (the newly-created issue's own ID). Order is preserved by first
// occurrence so the agent sees references in the order they appear in the body.
func extractIssueRefs(text, excludeID string) []string {
	matches := issueRefRe.FindAllString(text, -1)
	seen := map[string]bool{excludeID: true}
	var out []string
	for _, m := range matches {
		if seen[m] {
			continue
		}
		seen[m] = true
		out = append(out, m)
	}
	return out
}

func (h *Handler) registerIssueRoutes(api huma.API) {
	type IssueIntIDInput struct {
		ID int32 `json:"id"`
	}

	type ListIssuesInput struct {
		PaginatedSlugInput
		Component string `query:"component"`
	}
	huma.Register(api, huma.Operation{OperationID: "list-issues", Method: http.MethodGet, Path: "/api/dx/todo/issue/list"},
		func(ctx context.Context, in *ListIssuesInput) (*struct {
			Body struct {
				Issues       []IssueItem      `json:"issues"`
				Total        int64            `json:"total"`
				StatusCounts map[string]int64 `json:"status_counts"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			limit, offset := parsePage(in.Limit, in.Offset)
			b := db.WrapListIssues(p.ID)
			if in.Component != "" {
				b = b.Where("component", metaquery.OpEq, in.Component)
			}
			if in.Status != "" {
				b = b.Where("status", metaquery.OpEq, in.Status)
			}
			b = b.ApplyPagination(metaquery.PageRequest{Page: int(offset / limit), Size: int(limit), Total: true})
			res, err := mqpgx.Scan[db.ZdxIssue](ctx, h.Pool, b)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]IssueItem, len(res.Data))
			for i, r := range res.Data {
				out[i] = toIssueItem(r)
			}
			total := res.Meta.Pagination.Total

			countRows, err := h.Q.CountIssuesByStatus(ctx, db.CountIssuesByStatusParams{ProjectID: p.ID, Component: in.Component})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			statusCounts := make(map[string]int64, len(countRows))
			for _, r := range countRows {
				statusCounts[r.Status] = r.Count
			}
			return &struct {
				Body struct {
					Issues       []IssueItem      `json:"issues"`
					Total        int64            `json:"total"`
					StatusCounts map[string]int64 `json:"status_counts"`
				}
			}{Body: struct {
				Issues       []IssueItem      `json:"issues"`
				Total        int64            `json:"total"`
				StatusCounts map[string]int64 `json:"status_counts"`
			}{Issues: out, Total: total, StatusCounts: statusCounts}}, nil
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
				NodeRef       *string  `json:"node_ref,omitempty" doc:"Atlas node this issue is filed against, formatted as kind:slug"`
			}
		}) (*struct {
			Body struct {
				IssueItem
				Similar           []SimilarIssueItem `json:"similar,omitempty"`
				SuggestedBlockers []string           `json:"suggested_blockers,omitempty"`
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
				IssueType: "unknown",
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
			if in.Body.NodeRef != nil && *in.Body.NodeRef != "" {
				ref := strings.TrimSpace(*in.Body.NodeRef)
				if !validNodeRef(ref) {
					return nil, apiErr(400, "node_ref must be formatted as kind:slug with both parts non-empty")
				}
				params.NodeRef = pgtype.Text{String: ref, Valid: true}
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
				// Sequencing: --blocked-by names real "this depends on that" deps.
				_ = h.Q.AddIssueBlock(ctx, db.AddIssueBlockParams{IssueID: row.ID, BlockedByID: blockerID, Kind: "sequencing"})
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
			if in.Body.AutoReady {
				h.refreshQueueAsync(p.ID)
			}
			suggested := extractIssueRefs(issueText, row.ID)
			return &struct {
				Body struct {
					IssueItem
					Similar           []SimilarIssueItem `json:"similar,omitempty"`
					SuggestedBlockers []string           `json:"suggested_blockers,omitempty"`
				}
			}{Body: struct {
				IssueItem
				Similar           []SimilarIssueItem `json:"similar,omitempty"`
				SuggestedBlockers []string           `json:"suggested_blockers,omitempty"`
			}{IssueItem: item, Similar: similar, SuggestedBlockers: suggested}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "triage-issue", Method: http.MethodPost, Path: "/api/dx/todo/owner/triage"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug         string       `json:"slug"`
				ID           int32        `json:"id"`
				Priority     int32        `json:"priority"`
				Title        *string      `json:"title,omitempty"`
				IssueType    *string      `json:"issue_type,omitempty"`
				Context      *string      `json:"context,omitempty"`
				TargetBranch *string      `json:"target_branch,omitempty"`
				ThemeIDs     []int32      `json:"theme_ids,omitempty"`
				GoalIDs      []int32      `json:"goal_ids,omitempty"`
				BranchState  *BranchState `json:"branch_state,omitempty" required:"false"`
			}
		}) (*struct{ Body OKBody }, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			issueID := issueIDFromInt(in.Body.ID)

			// IS-915: contract validation. Look up the active triage todo for this issue.
			if in.Body.BranchState != nil {
				todoKey := "triage-" + issueID
				if claimedTodo, kErr := h.Q.GetTodoByKey(ctx, db.GetTodoByKeyParams{ProjectID: p.ID, Key: todoKey}); kErr == nil {
					if vErr := validateClaimContract(claimedTodo.Kind, claimedTodo.ClaimBaseSha, claimedTodo.ClaimBaseBranch, in.Body.BranchState); vErr != nil {
						return nil, vErr
					}
				}
			}
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
				"title":         in.Body.Title,
				"issue_type":    in.Body.IssueType,
				"context":       in.Body.Context,
				"target_branch": in.Body.TargetBranch,
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
					case "target_branch":
						oldVal = oldIssue.TargetBranch
					}
					if oldVal != *val {
						h.recordRevision(ctx, p.ID, "issue", issueID, field, oldVal, *val)
					}
				}
			}
			for _, tid := range in.Body.ThemeIDs {
				_ = h.Q.AddFocusBlocker(ctx, db.AddFocusBlockerParams{FocusID: tid, IssueID: issueID})
			}
			for _, gid := range in.Body.GoalIDs {
				_ = h.Q.LinkGoalIssue(ctx, db.LinkGoalIssueParams{GoalID: gid, IssueID: issueID})
			}
			h.refreshQueueAsync(p.ID)
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "edit-issue", Method: http.MethodPost, Path: "/api/dx/todo/issue/edit"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug            string  `json:"slug"`
				ID              int32   `json:"id"`
				Title           *string `json:"title,omitempty"`
				Context         *string `json:"context,omitempty"`
				Priority        *int32  `json:"priority,omitempty"`
				IssueType       *string `json:"issue_type,omitempty"`
				Component       *string `json:"component,omitempty"`
				URL             *string `json:"url,omitempty"`
				InteractiveOnly *bool   `json:"interactive_only,omitempty"`
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
			if in.Body.InteractiveOnly != nil {
				if err := h.Q.SetIssueInteractiveOnly(ctx, db.SetIssueInteractiveOnlyParams{
					InteractiveOnly: *in.Body.InteractiveOnly,
					ProjectID:       p.ID,
					ID:              issueID,
				}); err != nil {
					return nil, apiErr(500, err.Error())
				}
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "close-issue", Method: http.MethodPost, Path: "/api/dx/todo/issue/close"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug        string       `json:"slug"`
				ID          int32        `json:"id"`
				Reason      *string      `json:"reason,omitempty"`
				Force       *bool        `json:"force,omitempty"`
				Notes       *string      `json:"notes,omitempty"`
				DuplicateOf *string      `json:"duplicate_of,omitempty"`
				LinkOf      *string      `json:"link_of,omitempty"`
				BranchState *BranchState `json:"branch_state,omitempty" required:"false"`
			}
		}) (*struct{ Body OKBody }, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			issueID := issueIDFromInt(in.Body.ID)

			// IS-915: contract validation. Look up the active closable/close:tracker todo for this issue.
			if in.Body.BranchState != nil {
				for _, keyFmt := range []string{"closable-%s", "close-tracker-%s"} {
					todoKey := fmt.Sprintf(keyFmt, issueID)
					if claimedTodo, kErr := h.Q.GetTodoByKey(ctx, db.GetTodoByKeyParams{ProjectID: p.ID, Key: todoKey}); kErr == nil && claimedTodo.Status == "claimed" {
						if vErr := validateClaimContract(claimedTodo.Kind, claimedTodo.ClaimBaseSha, claimedTodo.ClaimBaseBranch, in.Body.BranchState); vErr != nil {
							return nil, vErr
						}
						break
					}
				}
			}
			reason := ptrStr(in.Body.Reason)
			force := in.Body.Force != nil && *in.Body.Force
			duplicateOf := ptrStr(in.Body.DuplicateOf)
			linkOf := ptrStr(in.Body.LinkOf)
			forceReasons := map[string]bool{"duplicate": true, "wontfix": true, "superseded": true}
			if forceReasons[reason] && !force {
				return nil, apiErr(400, "closing with --reason="+reason+" requires --force")
			}
			if force && !forceReasons[reason] {
				return nil, apiErr(400, "--force requires --reason=duplicate|wontfix|superseded")
			}
			if reason == "duplicate" && duplicateOf == "" {
				return nil, apiErr(400, "duplicate_of is required when reason is duplicate")
			}
			if reason == "link" && linkOf == "" {
				return nil, apiErr(400, "link_of is required when reason is link")
			}
			if duplicateOf != "" && linkOf != "" {
				return nil, apiErr(400, "duplicate_of and link_of are mutually exclusive")
			}
			agent := ""
			if uid := ctxUserIDVal(ctx); uid != 0 {
				if u, uErr := h.Q.GetUserByID(ctx, uid); uErr == nil {
					agent = u.Email
				}
			}
			if !force && (reason == "done" || reason == "") {
				// Close-gate: refuse while open tasks (wip/ready/active) remain.
				openTasks, _ := h.Q.ListOpenTasksByIssue(ctx, db.ListOpenTasksByIssueParams{
					Issue:     issueID,
					ProjectID: p.ID,
				})
				if len(openTasks) > 0 {
					var sb strings.Builder
					fmt.Fprintf(&sb, "%s has %d open task(s); resolve each before closing:\n", issueID, len(openTasks))
					for _, t := range openTasks {
						fmt.Fprintf(&sb, "\n  %s (%s) %q\n", t.ID, t.Status, t.Title)
						switch t.Status {
						case "wip":
							fmt.Fprintf(&sb, "    dx task ready %s\n", t.ID)
							fmt.Fprintf(&sb, "    dx task delete %s\n", t.ID)
							fmt.Fprintf(&sb, "    dx task convert %s --to-issue\n", t.ID)
							fmt.Fprintf(&sb, "    dx task block %s --by=IS-M --reason=...\n", t.ID)
						case "ready":
							fmt.Fprintf(&sb, "    dx todo dev done %s --test-plan=\"...\" --file ...\n", t.ID)
							fmt.Fprintf(&sb, "    dx task release %s\n", t.ID)
						case "active":
							fmt.Fprintf(&sb, "    dx task release %s\n", t.ID)
						}
					}
					return nil, apiErr(422, sb.String())
				}

				issue, gErr := h.Q.GetIssue(ctx, db.GetIssueParams{ProjectID: p.ID, ID: issueID})
				if gErr == nil && issue.IssueType == "impl" {
					count, _ := h.Q.CountIssueResolutions(ctx, issueID)
					if count == 0 {
						return nil, apiErr(400, "impl issues require at least one resolution before closing; use 'dx issue resolve' first")
					}
				}
				if gErr == nil && issue.IssueType != "tracker" && issue.IssueType != "ops" {
					subCount, _ := h.Q.CountSubstantiveIssueWork(ctx, issueID)
					if subCount == 0 {
						return nil, apiErr(422, "cannot close "+issueID+" — work-log has no substantive entries. Add one with: dx journal add --issue="+issueID+" --note=\"...\" (or close with --reason=wontfix/duplicate/link if not done)")
					}
				}
			}
			if !force {
				unresolvedBranches, _ := h.Q.ListUnresolvedNamedBranchesForIssue(ctx, db.ListUnresolvedNamedBranchesForIssueParams{
					ProjectID: p.ID,
					IssueID:   issueID,
				})
				if len(unresolvedBranches) > 0 {
					return nil, apiErr(422, "unresolved_branches: "+strings.Join(unresolvedBranches, ", "))
				}
			}
			prevStatus := "open"
			if issue, gErr := h.Q.GetIssue(ctx, db.GetIssueParams{ProjectID: p.ID, ID: issueID}); gErr == nil {
				prevStatus = issue.Status
			}
			closeReason := ""
			if force {
				closeReason = reason
			}
			if err := h.Q.CloseIssue(ctx, db.CloseIssueParams{ProjectID: p.ID, ID: issueID, DuplicateOf: duplicateOf, LinkOf: linkOf, CloseReason: closeReason}); err != nil {
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
			if linkOf != "" {
				note += "link of " + linkOf + " "
			}
			if notes != "" {
				note += notes
			}
			_ = h.Q.AppendIssueWork(ctx, db.AppendIssueWorkParams{IssueID: issueID, Agent: agent, Note: note})
			h.Broker.PublishIssue(in.Body.Slug, issueID, "issue.closed", map[string]any{"id": issueID, "reason": reason})

			// Warn about ready tasks with no test_refs that will not be cascade-closed.
			// These were never verified; they stay open for re-adoption.
			if unverified, uErr := h.Q.ListReadyTasksWithoutTestRefsByIssue(ctx, db.ListReadyTasksWithoutTestRefsByIssueParams{
				ProjectID: p.ID,
				Issue:     issueID,
			}); uErr == nil && len(unverified) > 0 {
				ids := make([]string, len(unverified))
				for i, t := range unverified {
					ids[i] = t.ID
				}
				warnNote := fmt.Sprintf("[cascade-skip] %d task(s) remain open (status=ready, no test_refs): %s — re-file or adopt", len(unverified), strings.Join(ids, ", "))
				_ = h.Q.AppendIssueWork(ctx, db.AppendIssueWorkParams{IssueID: issueID, Agent: agent, Note: warnNote})
			}

			// Cascade-close on open→closed transition only: close any open issues
			// that reference this issue via duplicate_of or link_of. Reopen does
			// not reverse this (see reopen-issue handler).
			if prevStatus == "open" {
				linked, lErr := h.Q.ListOpenLinkedIssues(ctx, db.ListOpenLinkedIssuesParams{ProjectID: p.ID, TargetID: issueID})
				if lErr == nil {
					for _, li := range linked {
						_ = h.Q.CloseIssue(ctx, db.CloseIssueParams{ProjectID: p.ID, ID: li.ID, DuplicateOf: li.DuplicateOf, LinkOf: li.LinkOf, CloseReason: ""})
						h.recordRevision(ctx, p.ID, "issue", li.ID, "status", "open", "closed")
						cascadeNote := "[closed:cascade] target " + issueID + " closed"
						_ = h.Q.AppendIssueWork(ctx, db.AppendIssueWorkParams{IssueID: li.ID, Agent: agent, Note: cascadeNote})
						h.Broker.PublishIssue(in.Body.Slug, li.ID, "issue.closed", map[string]any{"id": li.ID, "reason": "cascade"})
					}
				}

				// Auto-close tracker issues whose last open child just closed.
				parentIDs, pErr := h.Q.ListIssuesBlockedBy(ctx, issueID)
				if pErr == nil {
					for _, parentID := range parentIDs {
						parent, gErr := h.Q.GetIssue(ctx, db.GetIssueParams{ProjectID: p.ID, ID: parentID})
						if gErr != nil || parent.IssueType != "tracker" || parent.Status != "open" {
							continue
						}
						blockers, bErr := h.Q.ListIssueBlockersWithStatus(ctx, parentID)
						if bErr != nil {
							continue
						}
						allClosed := len(blockers) > 0
						for _, b := range blockers {
							if b.Status != "closed" {
								allClosed = false
								break
							}
						}
						if !allClosed {
							continue
						}
						_ = h.Q.CloseIssue(ctx, db.CloseIssueParams{ProjectID: p.ID, ID: parentID, CloseReason: ""})
						h.recordRevision(ctx, p.ID, "issue", parentID, "status", "open", "closed")
						trackerNote := "[closed:tracker-cascade] all children closed; last was " + issueID
						_ = h.Q.AppendIssueWork(ctx, db.AppendIssueWorkParams{IssueID: parentID, Agent: agent, Note: trackerNote})
						h.Broker.PublishIssue(in.Body.Slug, parentID, "issue.closed", map[string]any{"id": parentID, "reason": "tracker-cascade"})
					}
				}
			}
			// Auto-unblock todos that reference this issue as their system-gap issue.
			_ = h.Q.UnblockTodosByReferenceIssue(ctx, db.UnblockTodosByReferenceIssueParams{
				ProjectID:        p.ID,
				ReferenceIssueID: issueID,
			})
			h.refreshQueueAsync(p.ID)
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-spec-close-gate-offenders", Method: http.MethodGet, Path: "/api/dx/todo/issue/spec-close-gate"},
		func(ctx context.Context, in *struct {
			Slug string `query:"slug" required:"true"`
			ID   string `query:"id" required:"true"`
		}) (*struct {
			Body struct {
				Offenders []SpecCloseGateOffender `json:"offenders"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			rows, err := h.Q.ListCloseGateOffenders(ctx, db.ListCloseGateOffendersParams{
				ProjectID: p.ID,
				IssueID:   in.ID,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			offenders := make([]SpecCloseGateOffender, len(rows))
			for i, r := range rows {
				offenders[i] = SpecCloseGateOffender{
					SpecID:      r.SpecID,
					Description: r.Description,
					Feature:     r.FeatureName,
					Reason:      r.Reason,
				}
			}
			return &struct {
				Body struct {
					Offenders []SpecCloseGateOffender `json:"offenders"`
				}
			}{Body: struct {
				Offenders []SpecCloseGateOffender `json:"offenders"`
			}{Offenders: offenders}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-must-spec-demo-gate-offenders", Method: http.MethodGet, Path: "/api/dx/todo/issue/demo-gate"},
		func(ctx context.Context, in *struct {
			Slug string `query:"slug" required:"true"`
			ID   string `query:"id" required:"true"`
		}) (*struct {
			Body struct {
				Offenders []SpecCloseGateOffender `json:"offenders"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			rows, err := h.Q.ListMustSpecDemoGateOffenders(ctx, db.ListMustSpecDemoGateOffendersParams{
				ProjectID: p.ID,
				IssueID:   in.ID,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			offenders := make([]SpecCloseGateOffender, len(rows))
			for i, r := range rows {
				offenders[i] = SpecCloseGateOffender{
					SpecID:      r.SpecID,
					Description: r.Description,
					Feature:     r.FeatureName,
					Reason:      "no-demo",
				}
			}
			return &struct {
				Body struct {
					Offenders []SpecCloseGateOffender `json:"offenders"`
				}
			}{Body: struct {
				Offenders []SpecCloseGateOffender `json:"offenders"`
			}{Offenders: offenders}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-must-spec-ship-gate-offenders", Method: http.MethodGet, Path: "/api/dx/ship/must-spec-gate"},
		func(ctx context.Context, in *struct {
			Slug        string `query:"slug" required:"true"`
			Debug       string `query:"debug" required:"false"`
			XAtlasDebug string `header:"X-Atlas-Debug" required:"false"`
		}) (*struct {
			Body struct {
				Offenders []SpecCloseGateOffender `json:"offenders"`
				Debug     *DebugOutput            `json:"debug,omitempty"`
			}
		}, error) {
			var err error
			ctx, _, err = debugStart(ctx, in.Debug, in.XAtlasDebug)
			if err != nil {
				return nil, err
			}
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			rows, err := h.Q.ListMustSpecShipGateOffenders(ctx, p.ID)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			offenders := make([]SpecCloseGateOffender, len(rows))
			for i, r := range rows {
				offenders[i] = SpecCloseGateOffender{
					SpecID:      r.SpecID,
					Description: r.Description,
					Feature:     r.FeatureName,
					Reason:      r.Reason,
				}
				trace.Note(ctx, "gate offender", map[string]any{
					"spec":    r.SpecID,
					"feature": r.FeatureName,
					"reason":  r.Reason,
				})
			}
			trace.Note(ctx, "offender_count", len(offenders))
			return &struct {
				Body struct {
					Offenders []SpecCloseGateOffender `json:"offenders"`
					Debug     *DebugOutput            `json:"debug,omitempty"`
				}
			}{Body: struct {
				Offenders []SpecCloseGateOffender `json:"offenders"`
				Debug     *DebugOutput            `json:"debug,omitempty"`
			}{Offenders: offenders, Debug: debugOutput(ctx)}}, nil
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
			// Reopening a target does NOT reopen linked/duplicate issues: a
			// narrow-slice link may have been fixed independently, or the new
			// target reopen may describe a different aspect. Closed links stay
			// closed until explicitly reopened.
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
				// Kind: 'sequencing' (default) for "X waits for Y" deps; 'composition' for tracker → child relationships.
				// Composition edges are written by `dx issue add --parent` so a tracker can list its children
				// without conflating them with real sequencing blockers.
				Kind string `json:"kind,omitempty"`
			}
		}) (*struct{ Body OKBody }, error) {
			_, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			issueID := issueIDFromInt(in.Body.ID)
			kind := in.Body.Kind
			if kind == "" {
				kind = "sequencing"
			}
			if kind != "sequencing" && kind != "composition" {
				return nil, apiErr(http.StatusUnprocessableEntity, "kind must be 'sequencing' or 'composition'")
			}
			if err := h.Q.AddIssueBlock(ctx, db.AddIssueBlockParams{IssueID: issueID, BlockedByID: in.Body.BlockedBy, Kind: kind}); err != nil {
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
			limit, offset := parsePage(in.Limit, in.Offset)
			b := db.WrapListWorklogForProject(p.ID).
				ApplyPagination(metaquery.PageRequest{Page: int(offset / limit), Size: int(limit), Total: true})
			res, err := mqpgx.Scan[db.ListWorklogForProjectRow](ctx, h.Pool, b)
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
			out := make([]entry, len(res.Data))
			for i, r := range res.Data {
				out[i] = entry{IssueID: r.IssueID, IssueTitle: r.IssueTitle, Agent: r.Agent, Note: r.Note, CreatedAt: fmtTS(r.CreatedAt)}
			}
			return &struct{ Body respBody }{Body: respBody{Entries: out, Total: res.Meta.Pagination.Total}}, nil
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
			h.refreshQueueAsync(p.ID)
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
			issueRow, err := h.Q.GetIssue(ctx, db.GetIssueParams{ProjectID: p.ID, ID: issueID})
			if err != nil {
				return nil, apiErr(http.StatusNotFound, "issue not found")
			}
			source := in.Body.Source
			if source == "" {
				source = "manual"
			}
			branch := in.Body.Branch
			if branch == "" {
				branch = "dev"
			}
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
			if branch == "dev" {
				unresolved, _ := h.Q.ListUnresolvedNamedBranchesForIssue(ctx, db.ListUnresolvedNamedBranchesForIssueParams{
					ProjectID: p.ID,
					IssueID:   issueID,
				})
				created := 0
				for _, targetBranch := range unresolved {
					if _, ok, bErr := h.createBackportTask(ctx, p.ID, issueID, issueRow.Title, targetBranch, "auto-generated by IS-825 trigger 1 on dev resolution"); ok && bErr == nil {
						created++
					}
				}
				if created > 0 {
					log.Printf("IS-825 trigger 1: %s resolved on dev → auto-created %d backport task(s)", issueID, created)
				}
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

	huma.Register(api, huma.Operation{OperationID: "claim-issue", Method: http.MethodPost, Path: "/api/dx/projects/{slug}/issues/{id}/claim"},
		func(ctx context.Context, in *struct {
			Slug string `path:"slug" required:"true"`
			ID   string `path:"id" required:"true"`
			Body struct {
				ClaimedBy        string `json:"claimed_by" required:"true"`
				LeaseDurationMin int32  `json:"lease_duration_min"`
			}
		}) (*struct {
			Body ReservationItem
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			existing, err := h.Q.GetActiveIssueReservation(ctx, db.GetActiveIssueReservationParams{
				ProjectID: p.ID,
				TargetID:  in.ID,
			})
			if err == nil {
				return nil, huma.Error409Conflict("issue already claimed by " + existing.ClaimedBy)
			}
			dur := in.Body.LeaseDurationMin
			if dur <= 0 {
				dur = 30
			}
			leaseExpiry := pgtype.Timestamptz{Time: time.Now().Add(time.Duration(dur) * time.Minute), Valid: true}
			res, err := h.Q.InsertReservation(ctx, db.InsertReservationParams{
				ProjectID:      p.ID,
				TargetType:     "issue",
				TargetID:       in.ID,
				ClaimedBy:      in.Body.ClaimedBy,
				LeaseExpiresAt: leaseExpiry,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body ReservationItem }{Body: ReservationItem{
				ID:             res.ID,
				TargetType:     res.TargetType,
				TargetID:       res.TargetID,
				ClaimedBy:      res.ClaimedBy,
				ClaimedAt:      fmtTS(res.ClaimedAt),
				ReleasedAt:     fmtTS(res.ReleasedAt),
				LeaseExpiresAt: fmtTS(res.LeaseExpiresAt),
			}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "release-issue", Method: http.MethodPost, Path: "/api/dx/projects/{slug}/issues/{id}/release"},
		func(ctx context.Context, in *struct {
			Slug string `path:"slug" required:"true"`
			ID   string `path:"id" required:"true"`
		}) (*struct{ Body OKBody }, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			if err := h.Q.ReleaseReservation(ctx, db.ReleaseReservationParams{
				ProjectID:  p.ID,
				TargetType: "issue",
				TargetID:   in.ID,
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "renew-issue-lease", Method: http.MethodPost, Path: "/api/dx/projects/{slug}/issues/{id}/renew"},
		func(ctx context.Context, in *struct {
			Slug string `path:"slug" required:"true"`
			ID   string `path:"id" required:"true"`
			Body struct {
				ClaimedBy        string `json:"claimed_by" required:"true"`
				LeaseDurationMin int32  `json:"lease_duration_min"`
			}
		}) (*struct{ Body OKBody }, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			dur := in.Body.LeaseDurationMin
			if dur <= 0 {
				dur = 30
			}
			interval := pgtype.Interval{Microseconds: int64(dur) * 60 * 1_000_000, Valid: true}
			if err := h.Q.RenewIssueReservation(ctx, db.RenewIssueReservationParams{
				LeaseDuration: interval,
				ProjectID:     p.ID,
				TargetID:      in.ID,
				ClaimedBy:     in.Body.ClaimedBy,
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-issue-reservations", Method: http.MethodGet, Path: "/api/dx/projects/{slug}/issues/{id}/reservations"},
		func(ctx context.Context, in *struct {
			Slug string `path:"slug" required:"true"`
			ID   string `path:"id" required:"true"`
		}) (*struct {
			Body struct {
				Reservations []ReservationItem `json:"reservations"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			rows, err := h.Q.ListReservationsByIssueID(ctx, db.ListReservationsByIssueIDParams{
				ProjectID: p.ID,
				IssueID:   in.ID,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]ReservationItem, len(rows))
			for i, r := range rows {
				out[i] = ReservationItem{
					ID:             r.ID,
					TargetType:     r.TargetType,
					TargetID:       r.TargetID,
					ClaimedBy:      r.ClaimedBy,
					ClaimedAt:      fmtTS(r.ClaimedAt),
					ReleasedAt:     fmtTS(r.ReleasedAt),
					LeaseExpiresAt: fmtTS(r.LeaseExpiresAt),
				}
			}
			return &struct {
				Body struct {
					Reservations []ReservationItem `json:"reservations"`
				}
			}{Body: struct {
				Reservations []ReservationItem `json:"reservations"`
			}{Reservations: out}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-issue-todos", Method: http.MethodGet, Path: "/api/dx/projects/{slug}/issues/{id}/todos"},
		func(ctx context.Context, in *struct {
			Slug string `path:"slug" required:"true"`
			ID   string `path:"id" required:"true"`
		}) (*struct {
			Body struct {
				Todos []TodoItem `json:"todos"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			rows, err := h.Q.ListTodosFiltered(ctx, db.ListTodosFilteredParams{
				ProjectID: p.ID,
				IssueRef:  pgtype.Text{String: in.ID, Valid: true},
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]TodoItem, len(rows))
			for i, r := range rows {
				out[i] = toTodoItemFromFiltered(r)
			}
			return &struct {
				Body struct {
					Todos []TodoItem `json:"todos"`
				}
			}{Body: struct {
				Todos []TodoItem `json:"todos"`
			}{Todos: out}}, nil
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
	nodeRef := ""
	if r.NodeRef.Valid {
		nodeRef = r.NodeRef.String
	}
	return IssueItem{
		ID:              issueIntID(r.ID),
		Title:           r.Title,
		Status:          r.Status,
		Priority:        r.Priority,
		Component:       r.Component,
		Context:         r.Context,
		IssueType:       r.IssueType,
		DuplicateOf:     r.DuplicateOf,
		LinkOf:          r.LinkOf,
		CloseReason:     r.CloseReason,
		ReopenCount:     r.ReopenCount,
		InteractiveOnly: r.InteractiveOnly,
		TargetBranch:    r.TargetBranch,
		URL:             r.Url,
		NodeRef:         nodeRef,
		CreatedAt:       fmtTS(r.CreatedAt),
		UpdatedAt:       fmtTS(r.UpdatedAt),
		BlockedBy:       []string{},
		BlockedByDetail: []IssueBlockerRef{},
	}
}

func (h *Handler) toIssueItemWithBlockers(ctx context.Context, r db.ZdxIssue) IssueItem {
	item := toIssueItem(r)
	rows, err := h.Q.ListIssueBlockersWithStatus(ctx, r.ID)
	if err == nil && len(rows) > 0 {
		ids := make([]string, 0, len(rows))
		detail := make([]IssueBlockerRef, 0, len(rows))
		for _, b := range rows {
			kind := b.Kind
			if kind == "" {
				kind = "sequencing"
			}
			// BlockedBy stays composition-inclusive for backwards compat with callers that
			// just want IDs; structured display via BlockedByDetail can split on kind.
			ids = append(ids, b.ID)
			detail = append(detail, IssueBlockerRef{ID: b.ID, Status: b.Status, Kind: kind})
		}
		item.BlockedBy = ids
		item.BlockedByDetail = detail
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
