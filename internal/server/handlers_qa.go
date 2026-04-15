package server

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/iodesystems/zdx-go/internal/db"
)

func (s *Server) registerQARoutes(api huma.API) {
	huma.Register(api, huma.Operation{OperationID: "add-question", Method: http.MethodPost, Path: "/api/dx/qa/add"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug             string `json:"slug"`
				Category         string `json:"category"`
				Question         string `json:"question"`
				ParentQuestionID *int32 `json:"parent_question_id,omitempty"`
			}
		}) (*struct{ Body QuestionItem }, error) {
			p, err := getProject(ctx, s.q, in.Body.Slug)
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
			row, err := s.q.InsertQuestion(ctx, params)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			go s.emb.upsertQuestion(context.Background(), p.ID, row.ID, row.Question)
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
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			results, err := s.findSimilarQuestions(ctx, p.ID, in.Body.Text, n)
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
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			row, err := s.q.AnswerQuestion(ctx, db.AnswerQuestionParams{
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
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			row, err := s.q.GetQuestion(ctx, db.GetQuestionParams{ProjectID: p.ID, ID: in.ID})
			if err != nil {
				return nil, apiErr(404, "question not found")
			}
			return &struct{ Body QuestionItem }{Body: toQuestionItem(row)}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-questions", Method: http.MethodGet, Path: "/api/dx/qa/list"},
		func(ctx context.Context, in *PaginatedSlugInput) (*struct {
			Body struct {
				Questions []QuestionItem `json:"questions"`
				Total     int64          `json:"total"`
			}
		}, error) {
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			total, _ := s.q.CountQuestions(ctx, p.ID)
			limit, offset := parsePage(in.Limit, in.Offset)
			rows, err := s.q.ListQuestionsPaginated(ctx, db.ListQuestionsPaginatedParams{ProjectID: p.ID, Limit: limit, Offset: offset})
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
				}{Questions: out, Total: total},
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
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			rows, err := s.q.ListChildQuestions(ctx, db.ListChildQuestionsParams{
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
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			rows, err := s.q.ListUnansweredQuestions(ctx, p.ID)
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
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			choicesJSON, _ := json.Marshal(in.Body.Choices)
			if in.Body.Choices == nil {
				choicesJSON = []byte("[]")
			}
			row, err := s.q.InsertBlockerQuestion(ctx, db.InsertBlockerQuestionParams{
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
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			if err := s.q.AnswerBlockerQuestion(ctx, db.AnswerBlockerQuestionParams{
				ProjectID:  p.ID,
				ID:         in.Body.ID,
				Answer:     in.Body.Answer,
				AnsweredBy: in.Body.AnsweredBy,
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			row, err := s.q.GetBlockerQuestion(ctx, db.GetBlockerQuestionParams{
				ProjectID: p.ID,
				ID:        in.Body.ID,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
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
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			var rows []db.ZdxBlockerQuestion
			var total int64
			if in.Status == "pending" {
				rows, err = s.q.ListPendingBlockerQuestions(ctx, p.ID)
				total = int64(len(rows))
			} else {
				total, _ = s.q.CountBlockerQuestions(ctx, p.ID)
				limit, offset := parsePage(in.Limit, in.Offset)
				rows, err = s.q.ListBlockerQuestionsPaginated(ctx, db.ListBlockerQuestionsPaginatedParams{ProjectID: p.ID, Limit: limit, Offset: offset})
			}
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
					Total     int64                 `json:"total"`
				}
			}{
				Body: struct {
					Questions []BlockerQuestionItem `json:"questions"`
					Total     int64                 `json:"total"`
				}{Questions: out, Total: total},
			}, nil
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
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			rows, err := s.q.ListBlockerQuestionsByTarget(ctx, db.ListBlockerQuestionsByTargetParams{
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
	return item
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
