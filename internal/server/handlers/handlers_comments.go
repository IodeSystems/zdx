package handlers

import (
	"context"
	"fmt"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgtype"

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
			total, _ := h.Q.CountCommentsByAuthor(ctx, db.CountCommentsByAuthorParams{ProjectID: p.ID, Author: user.Email})
			limit, offset := parsePage(in.Limit, in.Offset)
			rows, err := h.Q.ListCommentsByAuthorPaginated(ctx, db.ListCommentsByAuthorPaginatedParams{
				ProjectID: p.ID, Author: user.Email, Limit: limit, Offset: offset,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]CommentItem, len(rows))
			for i, r := range rows {
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
				}{Comments: out, Total: total},
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
			total, _ := h.Q.CountComments(ctx, db.CountCommentsParams{ProjectID: p.ID, TargetType: in.TargetType, TargetID: in.TargetID})
			limit, offset := parsePage(in.Limit, in.Offset)
			rows, err := h.Q.ListCommentsPaginated(ctx, db.ListCommentsPaginatedParams{
				ProjectID: p.ID, TargetType: in.TargetType, TargetID: in.TargetID, Limit: limit, Offset: offset,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			var lastReadTS = func() *int64 { return nil }() // placeholder
			_ = lastReadTS
			var lastReadValid bool
			var lastReadTime int64
			if in.Role != "" {
				ts, _ := h.Q.GetCommentRead(ctx, db.GetCommentReadParams{
					ProjectID: p.ID, TargetType: in.TargetType, TargetID: in.TargetID, Role: in.Role,
				})
				lastReadValid = ts.Valid
				if ts.Valid {
					lastReadTime = ts.Time.UnixNano()
				}
			}
			out := make([]CommentItem, len(rows))
			for i, r := range rows {
				ci := CommentItem{
					ID: r.ID, TargetType: r.TargetType, TargetID: r.TargetID,
					Author: r.Author, AuthorAlias: r.AuthorAlias, Body: r.Body, CreatedAt: fmtTS(r.CreatedAt),
				}
				if r.ParentID.Valid {
					ci.ParentID = &r.ParentID.Int32
				}
				if in.Role != "" {
					unread := !lastReadValid || r.CreatedAt.Time.UnixNano() > lastReadTime
					ci.Unread = &unread
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
				}{Comments: out, Total: total},
			}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "mark-comments-read", Method: http.MethodPost, Path: "/api/dx/comment/mark-read"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug       string `json:"slug"`
				TargetType string `json:"target_type"`
				TargetID   string `json:"target_id"`
				Role       string `json:"role"`
			}
		}) (*struct{ Body OKBody }, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			if err := h.Q.UpsertCommentRead(ctx, db.UpsertCommentReadParams{
				ProjectID:  p.ID,
				TargetType: in.Body.TargetType,
				TargetID:   in.Body.TargetID,
				Role:       in.Body.Role,
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "comment-unread-check", Method: http.MethodGet, Path: "/api/dx/comment/unread-check"},
		func(ctx context.Context, in *struct {
			Slug       string `query:"slug" required:"true"`
			TargetType string `query:"target_type" required:"true"`
			TargetID   string `query:"target_id" required:"true"`
			Role       string `query:"role" required:"true"`
		}) (*struct {
			Body struct {
				HasUnread bool `json:"has_unread"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			has, err := h.Q.HasUnreadCommentsForTarget(ctx, db.HasUnreadCommentsForTargetParams{
				ProjectID: p.ID, TargetType: in.TargetType, TargetID: in.TargetID, Role: in.Role,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct {
				Body struct {
					HasUnread bool `json:"has_unread"`
				}
			}{
				Body: struct {
					HasUnread bool `json:"has_unread"`
				}{HasUnread: has},
			}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "comment-stale-unread", Method: http.MethodGet, Path: "/api/dx/comment/stale-unread"},
		func(ctx context.Context, in *struct {
			Slug     string `query:"slug" required:"true"`
			Role     string `query:"role" required:"true"`
			AgeHours int32  `query:"age_hours"`
		}) (*struct {
			Body struct {
				Comments []StaleCommentItem `json:"comments"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			ageHours := in.AgeHours
			if ageHours < 0 {
				ageHours = 24
			}
			rows, err := h.Q.ListStaleUnreadComments(ctx, db.ListStaleUnreadCommentsParams{
				ProjectID: p.ID,
				Role:      in.Role,
				AgeHours:  ageHours,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]StaleCommentItem, len(rows))
			for i, r := range rows {
				sci := StaleCommentItem{
					ID:          r.ID,
					TargetType:  r.TargetType,
					TargetID:    r.TargetID,
					Author:      r.Author,
					AuthorAlias: r.AuthorAlias,
					Body:        r.Body,
					CreatedAt:   fmtTS(r.CreatedAt),
				}
				if r.ParentID.Valid {
					sci.ParentID = &r.ParentID.Int32
				}
				out[i] = sci
			}
			return &struct {
				Body struct {
					Comments []StaleCommentItem `json:"comments"`
				}
			}{
				Body: struct {
					Comments []StaleCommentItem `json:"comments"`
				}{Comments: out},
			}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "notifications-unread-count", Method: http.MethodGet, Path: "/api/dx/notifications/unread-count"},
		func(ctx context.Context, in *struct {
			Slug string `query:"slug" required:"true"`
		}) (*struct {
			Body struct {
				Count int32 `json:"count"`
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
			role := fmt.Sprintf("web:%d", uid)
			count, err := h.Q.CountUnreadResponsesForUser(ctx, db.CountUnreadResponsesForUserParams{
				ProjectID: p.ID,
				Author:    user.Email,
				Role:      role,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct {
				Body struct {
					Count int32 `json:"count"`
				}
			}{Body: struct {
				Count int32 `json:"count"`
			}{Count: count}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "notifications-unread-threads", Method: http.MethodGet, Path: "/api/dx/notifications/unread-threads"},
		func(ctx context.Context, in *struct {
			Slug string `query:"slug" required:"true"`
		}) (*struct {
			Body struct {
				Threads []db.ListUnreadResponseThreadsForUserRow `json:"threads"`
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
			role := fmt.Sprintf("web:%d", uid)
			threads, err := h.Q.ListUnreadResponseThreadsForUser(ctx, db.ListUnreadResponseThreadsForUserParams{
				ProjectID: p.ID,
				Author:    user.Email,
				Role:      role,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			if threads == nil {
				threads = []db.ListUnreadResponseThreadsForUserRow{}
			}
			return &struct {
				Body struct {
					Threads []db.ListUnreadResponseThreadsForUserRow `json:"threads"`
				}
			}{Body: struct {
				Threads []db.ListUnreadResponseThreadsForUserRow `json:"threads"`
			}{Threads: threads}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "notifications-dismiss-all", Method: http.MethodPost, Path: "/api/dx/notifications/dismiss-all"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug string `json:"slug"`
			}
		}) (*struct{ Body OKBody }, error) {
			uid := ctxUserIDVal(ctx)
			if uid == 0 {
				return nil, apiErr(http.StatusUnauthorized, "not authenticated")
			}
			user, err := h.Q.GetUserByID(ctx, uid)
			if err != nil {
				return nil, apiErr(http.StatusUnauthorized, "user not found")
			}
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			role := fmt.Sprintf("web:%d", uid)
			if err := h.Q.DismissAllUnreadResponsesForUser(ctx, db.DismissAllUnreadResponsesForUserParams{
				ProjectID: p.ID,
				Author:    user.Email,
				Role:      role,
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
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

	huma.Register(api, huma.Operation{OperationID: "react-to-comment", Method: http.MethodPost, Path: "/api/dx/comment/react"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug      string `json:"slug"`
				CommentID int32  `json:"comment_id"`
				Emoji     string `json:"emoji"`
			}
		}) (*struct {
			Body struct {
				ID    int32  `json:"id"`
				Emoji string `json:"emoji"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			reactor := ""
			if uid := ctxUserIDVal(ctx); uid != 0 {
				if u, err := h.Q.GetUserByID(ctx, uid); err == nil {
					reactor = u.Email
				}
			}
			r, err := h.Q.AddCommentReaction(ctx, db.AddCommentReactionParams{
				ProjectID: p.ID,
				CommentID: in.Body.CommentID,
				Emoji:     in.Body.Emoji,
				Reactor:   reactor,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct {
				Body struct {
					ID    int32  `json:"id"`
					Emoji string `json:"emoji"`
				}
			}{Body: struct {
				ID    int32  `json:"id"`
				Emoji string `json:"emoji"`
			}{ID: r.ID, Emoji: r.Emoji}}, nil
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
			total, _ := h.Q.CountRevisions(ctx, db.CountRevisionsParams{ProjectID: p.ID, TargetType: in.TargetType, TargetID: in.TargetID})
			limit, offset := parsePage(in.Limit, in.Offset)
			rows, err := h.Q.ListRevisionsPaginated(ctx, db.ListRevisionsPaginatedParams{
				ProjectID: p.ID, TargetType: in.TargetType, TargetID: in.TargetID, Limit: limit, Offset: offset,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]RevisionItem, len(rows))
			for i, r := range rows {
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
				}{Revisions: out, Total: total},
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
