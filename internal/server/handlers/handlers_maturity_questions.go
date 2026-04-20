package handlers

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/iodesystems/zdx-go/internal/db"
	"github.com/iodesystems/zdx-go/internal/maturity"
)

type MaturityQuestion struct {
	Key                       string   `json:"key"`
	Prompt                    string   `json:"prompt"`
	AnswerType                string   `json:"answer_type"`
	PriorityHint              int32    `json:"priority_hint"`
	ApplicableClassifications []string `json:"applicable_classifications"`
	Answer                    string   `json:"answer"`
	AnsweredBy                string   `json:"answered_by"`
	AnsweredAt                string   `json:"answered_at"`
}

type MaturityAnswer struct {
	ID          int32  `json:"id"`
	ProjectID   int32  `json:"project_id"`
	QuestionKey string `json:"question_key"`
	Answer      string `json:"answer"`
	AnsweredBy  string `json:"answered_by"`
	AnsweredAt  string `json:"answered_at"`
}

func maturityAnswerFrom(a db.ZdxMaturityAnswer) MaturityAnswer {
	return MaturityAnswer{
		ID:          a.ID,
		ProjectID:   a.ProjectID,
		QuestionKey: a.QuestionKey,
		Answer:      a.Answer,
		AnsweredBy:  a.AnsweredBy,
		AnsweredAt:  fmtTS(a.AnsweredAt),
	}
}

func (h *Handler) registerMaturityQuestionRoutes(api huma.API) {
	// GET /api/dx/maturity/questions?slug=...
	// Returns question catalog with per-project answer state.
	// On first call for a project, stamps baseline maturity items.
	huma.Register(api, huma.Operation{OperationID: "list-maturity-questions", Method: http.MethodGet, Path: "/api/dx/maturity/questions"},
		func(ctx context.Context, in *struct {
			Slug string `query:"slug" required:"true"`
		}) (*struct {
			Body struct {
				Questions []MaturityQuestion `json:"questions"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}

			// Stamp baseline items on first project contact.
			hasAnswers, err := maturity.HasAnyAnswers(ctx, h.Q, p.ID)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			if !hasAnswers {
				if _, err := maturity.StampBaseline(ctx, h.Q, p.ID); err != nil {
					return nil, apiErr(500, "baseline stamp failed: "+err.Error())
				}
			}

			questions, err := h.Q.ListMaturityQuestions(ctx)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			answers, err := h.Q.ListMaturityAnswers(ctx, p.ID)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			answerMap := make(map[string]db.ZdxMaturityAnswer, len(answers))
			for _, a := range answers {
				answerMap[a.QuestionKey] = a
			}

			out := make([]MaturityQuestion, len(questions))
			for i, q := range questions {
				mq := MaturityQuestion{
					Key:                       q.Key,
					Prompt:                    q.Prompt,
					AnswerType:                q.AnswerType,
					PriorityHint:              q.PriorityHint,
					ApplicableClassifications: q.ApplicableClassifications,
				}
				if a, ok := answerMap[q.Key]; ok {
					mq.Answer = a.Answer
					mq.AnsweredBy = a.AnsweredBy
					mq.AnsweredAt = fmtTS(a.AnsweredAt)
				}
				out[i] = mq
			}
			return &struct {
				Body struct {
					Questions []MaturityQuestion `json:"questions"`
				}
			}{Body: struct {
				Questions []MaturityQuestion `json:"questions"`
			}{Questions: out}}, nil
		})

	// POST /api/dx/maturity/answers?slug=...
	// Records an answer and stamps the corresponding maturity items.
	huma.Register(api, huma.Operation{OperationID: "submit-maturity-answer", Method: http.MethodPost, Path: "/api/dx/maturity/answers"},
		func(ctx context.Context, in *struct {
			Slug string `query:"slug" required:"true"`
			Body struct {
				QuestionKey string `json:"question_key" required:"true"`
				Answer      string `json:"answer" required:"true"`
				AnsweredBy  string `json:"answered_by"`
			}
		}) (*struct {
			Body struct {
				Answer MaturityAnswer `json:"answer"`
				Items  []MaturityItem `json:"items"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			if in.Body.QuestionKey == "" || in.Body.Answer == "" {
				return nil, apiErr(400, "question_key and answer are required")
			}

			a, err := h.Q.UpsertMaturityAnswer(ctx, db.UpsertMaturityAnswerParams{
				ProjectID:   p.ID,
				QuestionKey: in.Body.QuestionKey,
				Answer:      in.Body.Answer,
				AnsweredBy:  in.Body.AnsweredBy,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}

			stamped, err := maturity.StampFromAnswer(ctx, h.Q, p.ID, in.Body.QuestionKey, in.Body.Answer)
			if err != nil {
				return nil, apiErr(500, "stamp failed: "+err.Error())
			}
			items := make([]MaturityItem, len(stamped))
			for i, s := range stamped {
				items[i] = maturityItemFrom(s)
			}

			return &struct {
				Body struct {
					Answer MaturityAnswer `json:"answer"`
					Items  []MaturityItem `json:"items"`
				}
			}{Body: struct {
				Answer MaturityAnswer `json:"answer"`
				Items  []MaturityItem `json:"items"`
			}{Answer: maturityAnswerFrom(a), Items: items}}, nil
		})
}
