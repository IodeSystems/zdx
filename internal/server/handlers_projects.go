package server

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/iodesystems/zdx-go/internal/db"
)

func (s *Server) registerProjectRoutes(api huma.API) {
	// ── Goals & Constraints ──────────────────────────────────────────────────

	type GoalItem struct {
		ID          int32  `json:"id"`
		Title       string `json:"title"`
		Description string `json:"description"`
		Priority    int32  `json:"priority"`
		Status      string `json:"status"`
		CreatedAt   string `json:"created_at"`
		UpdatedAt   string `json:"updated_at"`
	}

	type ConstraintItem struct {
		ID          int32  `json:"id"`
		Title       string `json:"title"`
		Description string `json:"description"`
		Priority    int32  `json:"priority"`
		Status      string `json:"status"`
		CreatedAt   string `json:"created_at"`
		UpdatedAt   string `json:"updated_at"`
	}

	huma.Register(api, huma.Operation{OperationID: "list-goals", Method: http.MethodGet, Path: "/api/goals"},
		func(ctx context.Context, in *IssueSlugInput) (*struct {
			Body struct {
				Goals []GoalItem `json:"goals"`
			}
		}, error) {
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			rows, err := s.q.ListProjectGoals(ctx, p.ID)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]GoalItem, len(rows))
			for i, r := range rows {
				out[i] = GoalItem{ID: r.ID, Title: r.Title, Description: r.Description, Priority: r.Priority, Status: r.Status, CreatedAt: fmtTS(r.CreatedAt), UpdatedAt: fmtTS(r.UpdatedAt)}
			}
			return &struct {
				Body struct {
					Goals []GoalItem `json:"goals"`
				}
			}{Body: struct {
				Goals []GoalItem `json:"goals"`
			}{Goals: out}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "create-goal", Method: http.MethodPost, Path: "/api/goal"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug        string `json:"slug"`
				Title       string `json:"title"`
				Description string `json:"description"`
				Priority    int32  `json:"priority"`
				Status      string `json:"status"`
			}
		}) (*struct{ Body GoalItem }, error) {
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			status := in.Body.Status
			if status == "" {
				status = "active"
			}
			row, err := s.q.CreateProjectGoal(ctx, db.CreateProjectGoalParams{
				ProjectID:   p.ID,
				Title:       in.Body.Title,
				Description: in.Body.Description,
				Priority:    in.Body.Priority,
				Status:      status,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body GoalItem }{Body: GoalItem{ID: row.ID, Title: row.Title, Description: row.Description, Priority: row.Priority, Status: row.Status, CreatedAt: fmtTS(row.CreatedAt), UpdatedAt: fmtTS(row.UpdatedAt)}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "update-goal", Method: http.MethodPut, Path: "/api/goal"},
		func(ctx context.Context, in *struct {
			Body struct {
				ID          int32  `json:"id"`
				Title       string `json:"title"`
				Description string `json:"description"`
				Priority    int32  `json:"priority"`
				Status      string `json:"status"`
			}
		}) (*struct{ Body OKBody }, error) {
			if err := s.q.UpdateProjectGoal(ctx, db.UpdateProjectGoalParams{
				ID:          in.Body.ID,
				Title:       in.Body.Title,
				Description: in.Body.Description,
				Priority:    in.Body.Priority,
				Status:      in.Body.Status,
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "delete-goal", Method: http.MethodDelete, Path: "/api/goal"},
		func(ctx context.Context, in *struct {
			Body struct {
				ID int32 `json:"id"`
			}
		}) (*struct{ Body OKBody }, error) {
			if err := s.q.DeleteProjectGoal(ctx, in.Body.ID); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-constraints", Method: http.MethodGet, Path: "/api/constraints"},
		func(ctx context.Context, in *IssueSlugInput) (*struct {
			Body struct {
				Constraints []ConstraintItem `json:"constraints"`
			}
		}, error) {
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			rows, err := s.q.ListProjectConstraints(ctx, p.ID)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]ConstraintItem, len(rows))
			for i, r := range rows {
				out[i] = ConstraintItem{ID: r.ID, Title: r.Title, Description: r.Description, Priority: r.Priority, Status: r.Status, CreatedAt: fmtTS(r.CreatedAt), UpdatedAt: fmtTS(r.UpdatedAt)}
			}
			return &struct {
				Body struct {
					Constraints []ConstraintItem `json:"constraints"`
				}
			}{Body: struct {
				Constraints []ConstraintItem `json:"constraints"`
			}{Constraints: out}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "create-constraint", Method: http.MethodPost, Path: "/api/constraint"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug        string `json:"slug"`
				Title       string `json:"title"`
				Description string `json:"description"`
				Priority    int32  `json:"priority"`
				Status      string `json:"status"`
			}
		}) (*struct{ Body ConstraintItem }, error) {
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			status := in.Body.Status
			if status == "" {
				status = "active"
			}
			row, err := s.q.CreateProjectConstraint(ctx, db.CreateProjectConstraintParams{
				ProjectID:   p.ID,
				Title:       in.Body.Title,
				Description: in.Body.Description,
				Priority:    in.Body.Priority,
				Status:      status,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body ConstraintItem }{Body: ConstraintItem{ID: row.ID, Title: row.Title, Description: row.Description, Priority: row.Priority, Status: row.Status, CreatedAt: fmtTS(row.CreatedAt), UpdatedAt: fmtTS(row.UpdatedAt)}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "update-constraint", Method: http.MethodPut, Path: "/api/constraint"},
		func(ctx context.Context, in *struct {
			Body struct {
				ID          int32  `json:"id"`
				Title       string `json:"title"`
				Description string `json:"description"`
				Priority    int32  `json:"priority"`
				Status      string `json:"status"`
			}
		}) (*struct{ Body OKBody }, error) {
			if err := s.q.UpdateProjectConstraint(ctx, db.UpdateProjectConstraintParams{
				ID:          in.Body.ID,
				Title:       in.Body.Title,
				Description: in.Body.Description,
				Priority:    in.Body.Priority,
				Status:      in.Body.Status,
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "delete-constraint", Method: http.MethodDelete, Path: "/api/constraint"},
		func(ctx context.Context, in *struct {
			Body struct {
				ID int32 `json:"id"`
			}
		}) (*struct{ Body OKBody }, error) {
			if err := s.q.DeleteProjectConstraint(ctx, in.Body.ID); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	// ── Proposals ─────────────────────────────────────────────────────────────

	type ProposalItem struct {
		ID             int32   `json:"id"`
		JournalEntryID *int32  `json:"journal_entry_id,omitempty"`
		Title          string  `json:"title"`
		Context        string  `json:"context"`
		Status         string  `json:"status"`
		Priority       int32   `json:"priority"`
		FiledIssueID   *string `json:"filed_issue_id,omitempty"`
		CreatedAt      string  `json:"created_at"`
	}

	proposalFromRow := func(r db.ZdxProposal) ProposalItem {
		p := ProposalItem{
			ID:        r.ID,
			Title:     r.Title,
			Context:   r.Context,
			Status:    r.Status,
			Priority:  r.Priority,
			CreatedAt: fmtTS(r.CreatedAt),
		}
		if r.JournalEntryID.Valid {
			p.JournalEntryID = &r.JournalEntryID.Int32
		}
		if r.FiledIssueID.Valid {
			p.FiledIssueID = &r.FiledIssueID.String
		}
		return p
	}

	huma.Register(api, huma.Operation{OperationID: "list-proposals", Method: http.MethodGet, Path: "/api/proposals"},
		func(ctx context.Context, in *struct {
			Slug   string `query:"slug" required:"true"`
			Status string `query:"status"`
		}) (*struct {
			Body struct {
				Proposals []ProposalItem `json:"proposals"`
			}
		}, error) {
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			var rows []db.ZdxProposal
			if in.Status != "" {
				rows, err = s.q.ListProposalsByStatus(ctx, db.ListProposalsByStatusParams{ProjectID: p.ID, Status: in.Status})
			} else {
				rows, err = s.q.ListProposals(ctx, p.ID)
			}
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]ProposalItem, len(rows))
			for i, r := range rows {
				out[i] = proposalFromRow(r)
			}
			return &struct {
				Body struct {
					Proposals []ProposalItem `json:"proposals"`
				}
			}{Body: struct {
				Proposals []ProposalItem `json:"proposals"`
			}{Proposals: out}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "create-proposal", Method: http.MethodPost, Path: "/api/proposal"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug           string `json:"slug"`
				JournalEntryID *int32 `json:"journal_entry_id,omitempty"`
				Title          string `json:"title"`
				Context        string `json:"context"`
				Priority       int32  `json:"priority"`
			}
		}) (*struct{ Body ProposalItem }, error) {
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			params := db.CreateProposalParams{
				ProjectID: p.ID,
				Title:     in.Body.Title,
				Context:   in.Body.Context,
				Priority:  in.Body.Priority,
			}
			if in.Body.JournalEntryID != nil {
				params.JournalEntryID = pgtype.Int4{Int32: *in.Body.JournalEntryID, Valid: true}
			}
			row, err := s.q.CreateProposal(ctx, params)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body ProposalItem }{Body: proposalFromRow(row)}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "update-proposal", Method: http.MethodPut, Path: "/api/proposal"},
		func(ctx context.Context, in *struct {
			Body struct {
				ID     int32  `json:"id"`
				Status string `json:"status"`
			}
		}) (*struct{ Body OKBody }, error) {
			if err := s.q.UpdateProposalStatus(ctx, db.UpdateProposalStatusParams{ID: in.Body.ID, Status: in.Body.Status}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "file-proposal", Method: http.MethodPost, Path: "/api/proposal/file"},
		func(ctx context.Context, in *struct {
			Body struct {
				ID           int32  `json:"id"`
				FiledIssueID string `json:"filed_issue_id"`
			}
		}) (*struct{ Body OKBody }, error) {
			if err := s.q.FileProposal(ctx, db.FileProposalParams{ID: in.Body.ID, FiledIssueID: pgtype.Text{String: in.Body.FiledIssueID, Valid: true}}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})
}
