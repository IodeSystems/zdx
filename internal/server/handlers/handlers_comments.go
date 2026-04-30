package handlers

import (
	"context"
	"fmt"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/iodesystems/sqlc-go-codegen-metaquery/metaquery"
	"github.com/iodesystems/sqlc-go-codegen-metaquery/metaquery/mqpgx"

	"github.com/iodesystems/zdx-go/internal/db"
)

func (h *Handler) registerCommentRoutes(api huma.API) {
	type CommentItem struct {
		ID          int32  `json:"id"`
		TargetType  string `json:"target_type"`
		TargetID    string `json:"target_id"`
		Author      string `json:"author"`
		AuthorAlias string `json:"author_alias,omitempty"`
		Body        string `json:"body"`
		CreatedAt   string `json:"created_at"`
		ParentID    *int32 `json:"parent_id,omitempty"`
		Unread      *bool  `json:"unread,omitempty"`
	}

	huma.Register(api, huma.Operation{OperationID: "list-my-comments", Method: http.MethodGet, Path: "/api/dx/comment/mine"},
		func(ctx context.Context, in *PaginatedSlugInput) (*struct {
			Body struct {
				Comments []CommentItem `json:"comments"`
				Total    int64         `json:"total"`
			}
		}, error) {
			uid := ctxUserIDVal(ctx)
			if uid == 0 {
				return nil, apiErr(http.StatusUnauthorized, "not authenticated")
			}
			user, err := h.Q.GetUserByID(ctx, uid)
			if err != nil {
				return nil, apiErr(http.StatusUnauthorized, "user not found")
			}
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			limit, offset := parsePage(in.Limit, in.Offset)
			b := db.WrapListCommentsByAuthor(db.ListCommentsByAuthorParams{ProjectID: p.ID, Author: user.Email}).
				ApplyPagination(metaquery.PageRequest{Page: int(offset / limit), Size: int(limit), Total: true})
			res, err := mqpgx.Scan[db.ListCommentsByAuthorRow](ctx, h.Pool, b)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]CommentItem, len(res.Data))
			for i, r := range res.Data {
				ci := CommentItem{
					ID: r.ID, TargetType: r.TargetType, TargetID: r.TargetID,
					Author: r.Author, AuthorAlias: r.AuthorAlias, Body: r.Body, CreatedAt: fmtTS(r.CreatedAt),
				}
				if r.ParentID.Valid {
					ci.ParentID = &r.ParentID.Int32
				}
				out[i] = ci
			}
			return &struct {
				Body struct {
					Comments []CommentItem `json:"comments"`
					Total    int64         `json:"total"`
				}
			}{
				Body: struct {
					Comments []CommentItem `json:"comments"`
					Total    int64         `json:"total"`
				}{Comments: out, Total: res.Meta.Pagination.Total},
			}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "add-comment", Method: http.MethodPost, Path: "/api/dx/comment/add"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug        string `json:"slug"`
				TargetType  string `json:"target_type"`
				TargetID    string `json:"target_id"`
				Body        string `json:"body"`
				ParentID    *int32 `json:"parent_id,omitempty"`
				AuthorAlias string `json:"author_alias,omitempty"`
			}
		}) (*struct{ Body CommentItem }, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			author := ""
			if uid := ctxUserIDVal(ctx); uid != 0 {
				if u, err := h.Q.GetUserByID(ctx, uid); err == nil {
					author = u.Email
				}
			}
			params := db.AddCommentParams{
				ProjectID:   p.ID,
				TargetType:  in.Body.TargetType,
				TargetID:    in.Body.TargetID,
				Author:      author,
				Body:        in.Body.Body,
				AuthorAlias: in.Body.AuthorAlias,
			}
			if in.Body.ParentID != nil {
				params.ParentID = pgtype.Int4{Int32: *in.Body.ParentID, Valid: true}
			}
			c, err := h.Q.AddComment(ctx, params)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			item := CommentItem{
				ID: c.ID, TargetType: c.TargetType, TargetID: c.TargetID,
				Author: c.Author, AuthorAlias: c.AuthorAlias, Body: c.Body, CreatedAt: fmtTS(c.CreatedAt),
			}
			if c.ParentID.Valid {
				item.ParentID = &c.ParentID.Int32
			}
			h.Broker.Publish(fmt.Sprintf("project:%s:comments", in.Body.Slug), "comment.added", item)
			h.Broker.Publish(fmt.Sprintf("%s:%s", in.Body.TargetType, in.Body.TargetID), "comment.added", item)
			return &struct{ Body CommentItem }{Body: item}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-comments", Method: http.MethodGet, Path: "/api/dx/comment/list"},
		func(ctx context.Context, in *struct {
			Slug       string `query:"slug"`
			TargetType string `query:"target_type"`
			TargetID   string `query:"target_id"`
			Role       string `query:"role"`
			Limit      int32  `query:"limit"`
			Offset     int32  `query:"offset"`
		}) (*struct {
			Body struct {
				Comments []CommentItem `json:"comments"`
				Total    int64         `json:"total"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			limit, offset := parsePage(in.Limit, in.Offset)
			b := db.WrapListComments(db.ListCommentsParams{ProjectID: p.ID, TargetType: in.TargetType, TargetID: in.TargetID}).
				ApplyPagination(metaquery.PageRequest{Page: int(offset / limit), Size: int(limit), Total: true})
			res, err := mqpgx.Scan[db.ListCommentsRow](ctx, h.Pool, b)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]CommentItem, len(res.Data))
			for i, r := range res.Data {
				ci := CommentItem{
					ID: r.ID, TargetType: r.TargetType, TargetID: r.TargetID,
					Author: r.Author, AuthorAlias: r.AuthorAlias, Body: r.Body, CreatedAt: fmtTS(r.CreatedAt),
				}
				if r.ParentID.Valid {
					ci.ParentID = &r.ParentID.Int32
				}
				out[i] = ci
			}
			return &struct {
				Body struct {
					Comments []CommentItem `json:"comments"`
					Total    int64         `json:"total"`
				}
			}{
				Body: struct {
					Comments []CommentItem `json:"comments"`
					Total    int64         `json:"total"`
				}{Comments: out, Total: res.Meta.Pagination.Total},
			}, nil
		})


	huma.Register(api, huma.Operation{OperationID: "get-comment", Method: http.MethodGet, Path: "/api/dx/comment/get"},
		func(ctx context.Context, in *struct {
			ID int32 `query:"id" required:"true"`
		}) (*struct{ Body CommentItem }, error) {
			row, err := h.Q.GetCommentByID(ctx, in.ID)
			if err != nil {
				return nil, apiErr(404, "comment not found")
			}
			ci := CommentItem{
				ID: row.ID, TargetType: row.TargetType, TargetID: row.TargetID,
				Author: row.Author, AuthorAlias: row.AuthorAlias, Body: row.Body, CreatedAt: fmtTS(row.CreatedAt),
			}
			if row.ParentID.Valid {
				ci.ParentID = &row.ParentID.Int32
			}
			return &struct{ Body CommentItem }{Body: ci}, nil
		})


	// ── Revisions ─────────────────────────────────────────────────────────────

	type RevisionItem struct {
		ID         int32  `json:"id"`
		TargetType string `json:"target_type"`
		TargetID   string `json:"target_id"`
		Field      string `json:"field"`
		OldVal     string `json:"old_val"`
		NewVal     string `json:"new_val"`
		Agent      string `json:"agent"`
		SessionID  string `json:"session_id"`
		UserID     string `json:"user_id"`
		CreatedAt  string `json:"created_at"`
	}

	huma.Register(api, huma.Operation{OperationID: "list-revisions", Method: http.MethodGet, Path: "/api/dx/revisions"},
		func(ctx context.Context, in *struct {
			Slug       string `query:"slug"`
			TargetType string `query:"target_type"`
			TargetID   string `query:"target_id"`
			Limit      int32  `query:"limit"`
			Offset     int32  `query:"offset"`
		}) (*struct {
			Body struct {
				Revisions []RevisionItem `json:"revisions"`
				Total     int64          `json:"total"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			limit, offset := parsePage(in.Limit, in.Offset)
			b := db.WrapListRevisions(db.ListRevisionsParams{ProjectID: p.ID, TargetType: in.TargetType, TargetID: in.TargetID}).
				ApplyPagination(metaquery.PageRequest{Page: int(offset / limit), Size: int(limit), Total: true})
			res, err := mqpgx.Scan[db.ListRevisionsRow](ctx, h.Pool, b)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]RevisionItem, len(res.Data))
			for i, r := range res.Data {
				out[i] = RevisionItem{
					ID: r.ID, TargetType: r.TargetType, TargetID: r.TargetID,
					Field: r.Field, OldVal: r.OldVal, NewVal: r.NewVal,
					Agent: r.Agent, SessionID: r.SessionID, UserID: r.UserID,
					CreatedAt: fmtTS(r.CreatedAt),
				}
			}
			return &struct {
				Body struct {
					Revisions []RevisionItem `json:"revisions"`
					Total     int64          `json:"total"`
				}
			}{
				Body: struct {
					Revisions []RevisionItem `json:"revisions"`
					Total     int64          `json:"total"`
				}{Revisions: out, Total: res.Meta.Pagination.Total},
			}, nil
		})
}

// ── Revision recording ────────────────────────────────────────────────────

func (h *Handler) recordRevision(ctx context.Context, projectID int32, targetType, targetID, field, oldVal, newVal string) {
	RecordRevision(ctx, h.Q, projectID, targetType, targetID, field, oldVal, newVal)
}

// RecordRevision is the package-level helper so non-handler callers (e.g.
// server task-recovery sweeps) can write revisions without constructing a
// full Handler.
func RecordRevision(ctx context.Context, q *db.Queries, projectID int32, targetType, targetID, field, oldVal, newVal string) {
	userID := ""
	if uid := ctxUserIDVal(ctx); uid != 0 {
		userID = fmt.Sprintf("%d", uid)
	}
	_ = q.AddRevision(ctx, db.AddRevisionParams{
		ProjectID:  projectID,
		TargetType: targetType,
		TargetID:   targetID,
		Field:      field,
		OldVal:     oldVal,
		NewVal:     newVal,
		Agent:      ctxAgentIDVal(ctx),
		SessionID:  ctxSessionIDVal(ctx),
		UserID:     userID,
	})
}

// recordStatusChange writes a zdx_revisions row for a status transition.
// agentIDOverride takes precedence over the request-context agent ID when
// the mutation itself identifies the acting agent (e.g. task claim or
// lease-expiry sweep); otherwise attribution falls back to context.
func (h *Handler) recordStatusChange(ctx context.Context, projectID int32, targetType, targetID, fromStatus, toStatus, agentIDOverride string) {
	RecordStatusChange(ctx, h.Q, projectID, targetType, targetID, fromStatus, toStatus, agentIDOverride)
}

// RecordStatusChange is the exported counterpart so server-package background
// sweeps (task recovery) can record status transitions without a Handler.
func RecordStatusChange(ctx context.Context, q *db.Queries, projectID int32, targetType, targetID, fromStatus, toStatus, agentIDOverride string) {
	agentID := agentIDOverride
	if agentID == "" {
		agentID = ctxAgentIDVal(ctx)
	}
	userID := ""
	if uid := ctxUserIDVal(ctx); uid != 0 {
		userID = fmt.Sprintf("%d", uid)
	}
	_ = q.AddRevision(ctx, db.AddRevisionParams{
		ProjectID:  projectID,
		TargetType: targetType,
		TargetID:   targetID,
		Field:      "status",
		OldVal:     fromStatus,
		NewVal:     toStatus,
		Agent:      agentID,
		SessionID:  ctxSessionIDVal(ctx),
		UserID:     userID,
	})
}

// recordTaskStatusChange looks up the task's project and records a status
// change if toStatus differs from prevStatus. Call after the mutation has
// been applied. If the task can't be found, recording is silently skipped —
// the primary mutation has already succeeded.
func (h *Handler) recordTaskStatusChange(ctx context.Context, taskID, prevStatus, toStatus, agentIDOverride string) {
	if prevStatus == toStatus {
		return
	}
	t, err := h.Q.GetTask(ctx, taskID)
	if err != nil {
		return
	}
	h.recordStatusChange(ctx, t.ProjectID, "task", taskID, prevStatus, toStatus, agentIDOverride)
}

// recordTaskFieldRevisions writes zdx_revisions rows for each user-editable
// field in newVals whose value differs from old. Intended for handlers that
// mutate task fields alongside (or independent of) a status change — status
// itself is recorded via recordTaskStatusChange and must not appear in newVals.
func (h *Handler) recordTaskFieldRevisions(ctx context.Context, old db.GetTaskRow, newVals map[string]string) {
	oldVals := map[string]string{
		"text":       old.Text,
		"feature":    old.Feature,
		"reason":     old.Reason,
		"issue":      old.Issue,
		"depends":    old.Depends,
		"test_plan":  old.TestPlan,
		"test_refs":  old.TestRefs,
		"task_group": old.TaskGroup,
	}
	for field, newVal := range newVals {
		oldVal, ok := oldVals[field]
		if !ok || oldVal == newVal {
			continue
		}
		h.recordRevision(ctx, old.ProjectID, "task", old.ID, field, oldVal, newVal)
	}
}
