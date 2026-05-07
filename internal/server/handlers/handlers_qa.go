package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/iodesystems/sqlc-go-codegen-metaquery/metaquery"
	"github.com/iodesystems/sqlc-go-codegen-metaquery/metaquery/mqpgx"

	"github.com/iodesystems/zdx-go/internal/db"
)

func (h *Handler) registerQARoutes(api huma.API) {
	huma.Register(api, huma.Operation{OperationID: "add-question", Method: http.MethodPost, Path: "/api/dx/qa/add"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug             string `json:"slug"`
				Category         string `json:"category"`
				Question         string `json:"question"`
				ParentQuestionID *int32 `json:"parent_question_id,omitempty"`
				OwnerUserID      *int32 `json:"owner_user_id,omitempty"`
			}
		}) (*struct{ Body QuestionItem }, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			params := db.InsertQuestionParams{
				ProjectID: p.ID,
				Category:  in.Body.Category,
				Question:  in.Body.Question,
			}
			if in.Body.ParentQuestionID != nil {
				params.ParentQuestionID = pgtype.Int4{Int32: *in.Body.ParentQuestionID, Valid: true}
			}
			if in.Body.OwnerUserID != nil {
				params.OwnerUserID = pgtype.Int4{Int32: *in.Body.OwnerUserID, Valid: true}
			} else if uid := ctxUserIDVal(ctx); uid != 0 {
				// Default owner to the asker when not explicitly set.
				params.OwnerUserID = pgtype.Int4{Int32: uid, Valid: true}
			}
			row, err := h.Q.InsertQuestion(ctx, params)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			go h.Emb.UpsertQuestion(context.Background(), p.ID, row.ID, row.Question)
			return &struct{ Body QuestionItem }{Body: toQuestionItem(row)}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "similar-questions", Method: http.MethodPost, Path: "/api/dx/qa/similar"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug string `json:"slug"`
				Text string `json:"text"`
				N    int    `json:"n,omitempty"`
			}
		}) (*struct {
			Body struct {
				Questions []SimilarQuestionItem `json:"questions"`
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
			results, err := h.findSimilarQuestions(ctx, p.ID, in.Body.Text, n)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct {
				Body struct {
					Questions []SimilarQuestionItem `json:"questions"`
				}
			}{Body: struct {
				Questions []SimilarQuestionItem `json:"questions"`
			}{Questions: results}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "answer-question", Method: http.MethodPost, Path: "/api/dx/qa/answer"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug   string `json:"slug"`
				ID     int32  `json:"id"`
				Answer string `json:"answer"`
			}
		}) (*struct{ Body QuestionItem }, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			row, err := h.Q.AnswerQuestion(ctx, db.AnswerQuestionParams{
				ProjectID: p.ID,
				ID:        in.Body.ID,
				Answer:    pgtype.Text{String: in.Body.Answer, Valid: true},
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body QuestionItem }{Body: toQuestionItem(row)}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "get-question", Method: http.MethodGet, Path: "/api/dx/qa/get"},
		func(ctx context.Context, in *struct {
			Slug string `query:"slug" required:"true"`
			ID   int32  `query:"id" required:"true"`
		}) (*struct{ Body QuestionItem }, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			row, err := h.Q.GetQuestion(ctx, db.GetQuestionParams{ProjectID: p.ID, ID: in.ID})
			if err != nil {
				return nil, apiErr(404, "question not found")
			}
			return &struct{ Body QuestionItem }{Body: toQuestionItem(row)}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-questions", Method: http.MethodGet, Path: "/api/dx/qa/list"},
		func(ctx context.Context, in *struct {
			Slug   string `query:"slug" required:"true"`
			Owner  string `query:"owner"`
			Limit  int32  `query:"limit"`
			Offset int32  `query:"offset"`
		}) (*struct {
			Body struct {
				Questions []QuestionItem `json:"questions"`
				Total     int64          `json:"total"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			// When owner filter is set, use ListQuestionsByOwner.
			if in.Owner != "" {
				ownerID, err := resolveOwnerID(ctx, h.Q, in.Owner)
				if err != nil {
					return nil, apiErr(400, "resolve owner: "+err.Error())
				}
				rows, err := h.Q.ListQuestionsByOwner(ctx, db.ListQuestionsByOwnerParams{
					ProjectID:   p.ID,
					OwnerUserID: pgtype.Int4{Int32: ownerID, Valid: true},
				})
				if err != nil {
					return nil, apiErr(500, err.Error())
				}
				out := make([]QuestionItem, len(rows))
				for i, r := range rows {
					out[i] = toQuestionItem(r)
				}
				return &struct {
					Body struct {
						Questions []QuestionItem `json:"questions"`
						Total     int64          `json:"total"`
					}
				}{
					Body: struct {
						Questions []QuestionItem `json:"questions"`
						Total     int64          `json:"total"`
					}{Questions: out, Total: int64(len(out))},
				}, nil
			}
			limit, offset := parsePage(in.Limit, in.Offset)
			b := db.WrapListQuestions(p.ID).
				ApplyPagination(metaquery.PageRequest{Page: int(offset / limit), Size: int(limit), Total: true})
			res, err := mqpgx.Scan[db.ZdxQuestion](ctx, h.Pool, b)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]QuestionItem, len(res.Data))
			for i, r := range res.Data {
				out[i] = toQuestionItem(r)
			}
			return &struct {
				Body struct {
					Questions []QuestionItem `json:"questions"`
					Total     int64          `json:"total"`
				}
			}{
				Body: struct {
					Questions []QuestionItem `json:"questions"`
					Total     int64          `json:"total"`
				}{Questions: out, Total: res.Meta.Pagination.Total},
			}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-child-questions", Method: http.MethodGet, Path: "/api/dx/qa/children"},
		func(ctx context.Context, in *struct {
			Slug             string `query:"slug" required:"true"`
			ParentQuestionID int32  `query:"parent_question_id" required:"true"`
		}) (*struct {
			Body struct {
				Questions []QuestionItem `json:"questions"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			rows, err := h.Q.ListChildQuestions(ctx, db.ListChildQuestionsParams{
				ProjectID:        p.ID,
				ParentQuestionID: pgtype.Int4{Int32: in.ParentQuestionID, Valid: true},
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]QuestionItem, len(rows))
			for i, r := range rows {
				out[i] = toQuestionItem(r)
			}
			return &struct {
				Body struct {
					Questions []QuestionItem `json:"questions"`
				}
			}{
				Body: struct {
					Questions []QuestionItem `json:"questions"`
				}{Questions: out},
			}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-unanswered-questions", Method: http.MethodGet, Path: "/api/dx/qa/unanswered"},
		func(ctx context.Context, in *struct {
			Slug string `query:"slug" required:"true"`
		}) (*struct {
			Body struct {
				Questions []QuestionItem `json:"questions"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			rows, err := h.Q.ListUnansweredQuestions(ctx, p.ID)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]QuestionItem, len(rows))
			for i, r := range rows {
				out[i] = toQuestionItem(r)
			}
			return &struct {
				Body struct {
					Questions []QuestionItem `json:"questions"`
				}
			}{
				Body: struct {
					Questions []QuestionItem `json:"questions"`
				}{Questions: out},
			}, nil
		})

	// ── Blocker questions ──────────────────────────────────────────────────────

	huma.Register(api, huma.Operation{OperationID: "update-question-owner", Method: http.MethodPatch, Path: "/api/dx/qa/owner"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug        string `json:"slug"`
				ID          int32  `json:"id"`
				OwnerUserID int32  `json:"owner_user_id"`
			}
		}) (*struct{ Body QuestionItem }, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			row, err := h.Q.UpdateQuestionOwner(ctx, db.UpdateQuestionOwnerParams{
				ProjectID:   p.ID,
				ID:          in.Body.ID,
				OwnerUserID: pgtype.Int4{Int32: in.Body.OwnerUserID, Valid: true},
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body QuestionItem }{Body: toQuestionItem(row)}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "add-blocker-question", Method: http.MethodPost, Path: "/api/dx/blocker-questions/add"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug       string   `json:"slug"`
				TargetType string   `json:"target_type"`
				TargetID   string   `json:"target_id"`
				Context    string   `json:"context"`
				Choices    []string `json:"choices,omitempty"`
			}
		}) (*struct{ Body BlockerQuestionItem }, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			choicesJSON, _ := json.Marshal(in.Body.Choices)
			if in.Body.Choices == nil {
				choicesJSON = []byte("[]")
			}
			row, err := h.Q.InsertBlockerQuestion(ctx, db.InsertBlockerQuestionParams{
				ProjectID:  p.ID,
				TargetType: in.Body.TargetType,
				TargetID:   in.Body.TargetID,
				Context:    in.Body.Context,
				Choices:    choicesJSON,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body BlockerQuestionItem }{Body: toBlockerQuestionItem(row)}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "answer-blocker-question", Method: http.MethodPost, Path: "/api/dx/blocker-questions/answer"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug       string `json:"slug"`
				ID         int32  `json:"id"`
				Answer     string `json:"answer"`
				AnsweredBy string `json:"answered_by,omitempty"`
			}
		}) (*struct{ Body BlockerQuestionItem }, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			if err := h.Q.AnswerBlockerQuestion(ctx, db.AnswerBlockerQuestionParams{
				ProjectID:  p.ID,
				ID:         in.Body.ID,
				Answer:     in.Body.Answer,
				AnsweredBy: in.Body.AnsweredBy,
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			row, err := h.Q.GetBlockerQuestion(ctx, db.GetBlockerQuestionParams{
				ProjectID: p.ID,
				ID:        in.Body.ID,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			traceEvent(ctx, h.Q, p.ID, "blocker_question.answered", "question_id", in.Body.ID, "target_type", row.TargetType, "target_id", row.TargetID)
			h.refreshQueueAsync(p.ID)
			return &struct{ Body BlockerQuestionItem }{Body: toBlockerQuestionItem(row)}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-blocker-questions", Method: http.MethodGet, Path: "/api/dx/blocker-questions/list"},
		func(ctx context.Context, in *struct {
			Slug   string `query:"slug" required:"true"`
			Status string `query:"status"`
			Limit  int32  `query:"limit"`
			Offset int32  `query:"offset"`
		}) (*struct {
			Body struct {
				Questions []BlockerQuestionItem `json:"questions"`
				Total     int64                 `json:"total"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			var out []BlockerQuestionItem
			var total int64
			if in.Status == "pending" {
				rows, err := h.Q.ListPendingBlockerQuestions(ctx, p.ID)
				if err != nil {
					return nil, apiErr(500, err.Error())
				}
				total = int64(len(rows))
				out = make([]BlockerQuestionItem, len(rows))
				for i, r := range rows {
					out[i] = toBlockerQuestionItem(r)
				}
			} else {
				limit, offset := parsePage(in.Limit, in.Offset)
				b := db.WrapListBlockerQuestions(p.ID).
					ApplyPagination(metaquery.PageRequest{Page: int(offset / limit), Size: int(limit), Total: true})
				res, err := mqpgx.Scan[db.ZdxBlockerQuestion](ctx, h.Pool, b)
				if err != nil {
					return nil, apiErr(500, err.Error())
				}
				total = res.Meta.Pagination.Total
				out = make([]BlockerQuestionItem, len(res.Data))
				for i, r := range res.Data {
					out[i] = toBlockerQuestionItem(r)
				}
			}
			return &struct {
				Body struct {
					Questions []BlockerQuestionItem `json:"questions"`
					Total     int64                 `json:"total"`
				}
			}{
				Body: struct {
					Questions []BlockerQuestionItem `json:"questions"`
					Total     int64                 `json:"total"`
				}{Questions: out, Total: total},
			}, nil
		})

	// ── Question proposals ───────────────────────────────────────────────────

	huma.Register(api, huma.Operation{OperationID: "add-question-proposal", Method: http.MethodPost, Path: "/api/dx/question-proposals/add"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug         string `json:"slug"`
				QuestionID   int32  `json:"question_id"`
				QuestionType string `json:"question_type"`
				Title        string `json:"title"`
				Context      string `json:"context"`
			}
		}) (*struct{ Body QuestionProposalItem }, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			row, err := h.Q.InsertQuestionProposal(ctx, db.InsertQuestionProposalParams{
				ProjectID:    p.ID,
				QuestionID:   in.Body.QuestionID,
				QuestionType: in.Body.QuestionType,
				Title:        in.Body.Title,
				Context:      in.Body.Context,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body QuestionProposalItem }{Body: toQuestionProposalItem(row)}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-question-proposals", Method: http.MethodGet, Path: "/api/dx/question-proposals/by-question"},
		func(ctx context.Context, in *struct {
			Slug         string `query:"slug" required:"true"`
			QuestionID   int32  `query:"question_id" required:"true"`
			QuestionType string `query:"question_type" required:"true"`
		}) (*struct {
			Body struct {
				Proposals []QuestionProposalItem `json:"proposals"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			rows, err := h.Q.ListQuestionProposalsByQuestion(ctx, db.ListQuestionProposalsByQuestionParams{
				ProjectID:    p.ID,
				QuestionID:   in.QuestionID,
				QuestionType: in.QuestionType,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]QuestionProposalItem, len(rows))
			for i, r := range rows {
				out[i] = toQuestionProposalItem(r)
			}
			return &struct {
				Body struct {
					Proposals []QuestionProposalItem `json:"proposals"`
				}
			}{
				Body: struct {
					Proposals []QuestionProposalItem `json:"proposals"`
				}{Proposals: out},
			}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "accept-question-proposal", Method: http.MethodPost, Path: "/api/dx/question-proposals/accept"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug string `json:"slug"`
				ID   int32  `json:"id"`
			}
		}) (*struct {
			Body struct {
				Proposal QuestionProposalItem `json:"proposal"`
				Issue    IssueItem            `json:"issue"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			proposal, err := h.Q.GetQuestionProposal(ctx, db.GetQuestionProposalParams{
				ProjectID: p.ID,
				ID:        in.Body.ID,
			})
			if err != nil {
				return nil, apiErr(404, "proposal not found")
			}
			issueID, err := h.Q.NextIssueID(ctx)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			issue, err := h.Q.CreateIssue(ctx, db.CreateIssueParams{
				ID:        issueID,
				ProjectID: p.ID,
				Title:     proposal.Title,
				Context:   proposal.Context,
				IssueType: "unknown",
				Status:    "open",
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			go h.Emb.UpsertIssue(context.Background(), p.ID, issue.ID, proposal.Title+" "+proposal.Context)
			accepted, err := h.Q.AcceptQuestionProposal(ctx, db.AcceptQuestionProposalParams{
				ProjectID:      p.ID,
				ID:             in.Body.ID,
				CreatedIssueID: pgtype.Text{String: issue.ID, Valid: true},
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			issueItem := toIssueItem(issue)
			h.Broker.PublishIssue(in.Body.Slug, issue.ID, "issue.created", issueItem)
			traceEvent(ctx, h.Q, p.ID, "issue.created", "issue_id", issue.ID, "title", issue.Title, "via", "proposal-approval")
			return &struct {
				Body struct {
					Proposal QuestionProposalItem `json:"proposal"`
					Issue    IssueItem            `json:"issue"`
				}
			}{
				Body: struct {
					Proposal QuestionProposalItem `json:"proposal"`
					Issue    IssueItem            `json:"issue"`
				}{Proposal: toQuestionProposalItem(accepted), Issue: issueItem},
			}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "deny-question-proposal", Method: http.MethodPost, Path: "/api/dx/question-proposals/deny"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug   string `json:"slug"`
				ID     int32  `json:"id"`
				Reason string `json:"reason"`
			}
		}) (*struct{ Body QuestionProposalItem }, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			row, err := h.Q.DenyQuestionProposal(ctx, db.DenyQuestionProposalParams{
				ProjectID:    p.ID,
				ID:           in.Body.ID,
				DeniedReason: in.Body.Reason,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body QuestionProposalItem }{Body: toQuestionProposalItem(row)}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-blocker-questions-by-target", Method: http.MethodGet, Path: "/api/dx/blocker-questions/by-target"},
		func(ctx context.Context, in *struct {
			Slug       string `query:"slug" required:"true"`
			TargetType string `query:"target_type" required:"true"`
			TargetID   string `query:"target_id" required:"true"`
		}) (*struct {
			Body struct {
				Questions []BlockerQuestionItem `json:"questions"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			rows, err := h.Q.ListBlockerQuestionsByTarget(ctx, db.ListBlockerQuestionsByTargetParams{
				ProjectID:  p.ID,
				TargetType: in.TargetType,
				TargetID:   in.TargetID,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]BlockerQuestionItem, len(rows))
			for i, r := range rows {
				out[i] = toBlockerQuestionItem(r)
			}
			return &struct {
				Body struct {
					Questions []BlockerQuestionItem `json:"questions"`
				}
			}{
				Body: struct {
					Questions []BlockerQuestionItem `json:"questions"`
				}{Questions: out},
			}, nil
		})
}

// ── Model → response converters ────────────────────────────────────────────

func toQuestionItem(r db.ZdxQuestion) QuestionItem {
	item := QuestionItem{
		ID:        r.ID,
		Category:  r.Category,
		Question:  r.Question,
		Answer:    r.Answer.String,
		CreatedAt: fmtTS(r.CreatedAt),
		UpdatedAt: fmtTS(r.UpdatedAt),
	}
	if r.ParentQuestionID.Valid {
		v := r.ParentQuestionID.Int32
		item.ParentQuestionID = &v
	}
	if r.OwnerUserID.Valid {
		v := r.OwnerUserID.Int32
		item.OwnerUserID = &v
	}
	return item
}

func toQuestionProposalItem(r db.ZdxQuestionProposal) QuestionProposalItem {
	return QuestionProposalItem{
		ID:             r.ID,
		QuestionID:     r.QuestionID,
		QuestionType:   r.QuestionType,
		Title:          r.Title,
		Context:        r.Context,
		Status:         r.Status,
		DeniedReason:   r.DeniedReason,
		CreatedIssueID: r.CreatedIssueID.String,
		CreatedAt:      fmtTS(r.CreatedAt),
		UpdatedAt:      fmtTS(r.UpdatedAt),
	}
}

func toBlockerQuestionItem(r db.ZdxBlockerQuestion) BlockerQuestionItem {
	var choices []string
	_ = json.Unmarshal(r.Choices, &choices)
	if choices == nil {
		choices = []string{}
	}
	return BlockerQuestionItem{
		ID:         r.ID,
		TargetType: r.TargetType,
		TargetID:   r.TargetID,
		Context:    r.Context,
		Choices:    choices,
		Answer:     r.Answer,
		AnsweredBy: r.AnsweredBy,
		Status:     r.Status,
		CreatedAt:  fmtTS(r.CreatedAt),
		AnsweredAt: fmtTS(r.AnsweredAt),
	}
}

// resolveOwnerID resolves "me" to the current user's ID or looks up by email.
func resolveOwnerID(ctx context.Context, q *db.Queries, owner string) (int32, error) {
	if owner == "me" {
		if uid := ctxUserIDVal(ctx); uid != 0 {
			return uid, nil
		}
		return 0, fmt.Errorf("not authenticated")
	}
	// Try email lookup.
	u, err := q.GetUserByEmail(ctx, owner)
	if err == nil {
		return u.ID, nil
	}
	return 0, fmt.Errorf("user %q not found", owner)
}
