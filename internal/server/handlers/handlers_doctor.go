package handlers

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/iodesystems/zdx-go/internal/db"
)

func (h *Handler) registerDoctorRoutes(api huma.API) {
	// GET /api/dx/project/info — project metadata including classification
	// and integration branch (git_branch) so agent loops can resolve the
	// authoritative base branch for slot worktrees instead of guessing from
	// cwd HEAD.
	huma.Register(api, huma.Operation{OperationID: "get-project-info", Method: http.MethodGet, Path: "/api/dx/project/info"},
		func(ctx context.Context, in *IssueSlugInput) (*struct {
			Body struct {
				ID             int32  `json:"id"`
				Slug           string `json:"slug"`
				Name           string `json:"name"`
				Title          string `json:"title"`
				Description    string `json:"description"`
				Classification string `json:"classification"`
				GitBranch      string `json:"git_branch"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			return &struct {
				Body struct {
					ID             int32  `json:"id"`
					Slug           string `json:"slug"`
					Name           string `json:"name"`
					Title          string `json:"title"`
					Description    string `json:"description"`
					Classification string `json:"classification"`
					GitBranch      string `json:"git_branch"`
				}
			}{Body: struct {
				ID             int32  `json:"id"`
				Slug           string `json:"slug"`
				Name           string `json:"name"`
				Title          string `json:"title"`
				Description    string `json:"description"`
				Classification string `json:"classification"`
				GitBranch      string `json:"git_branch"`
			}{ID: p.ID, Slug: p.Slug, Name: p.Name, Title: p.Title, Description: p.Description, Classification: p.Classification, GitBranch: p.GitBranch}}, nil
		})

	// POST /api/dx/doctor/classify — set project classification
	huma.Register(api, huma.Operation{OperationID: "set-classification", Method: http.MethodPost, Path: "/api/dx/doctor/classify"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug           string `json:"slug"`
				Classification string `json:"classification"`
			}
		}) (*struct{ Body OKBody }, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			if err := h.Q.SetProjectClassification(ctx, db.SetProjectClassificationParams{
				ID:             p.ID,
				Classification: in.Body.Classification,
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	// POST /api/dx/doctor/defer — defer a doctor check
	huma.Register(api, huma.Operation{OperationID: "defer-doctor-check", Method: http.MethodPost, Path: "/api/dx/doctor/defer"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug      string `json:"slug"`
				CheckName string `json:"check_name"`
				Rung      string `json:"rung" required:"false"`
			}
		}) (*struct{ Body OKBody }, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			if err := h.Q.DeferDoctorCheck(ctx, db.DeferDoctorCheckParams{
				ProjectID: p.ID,
				CheckName: in.Body.CheckName,
				Rung:      in.Body.Rung,
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	// GET /api/dx/doctor/deferrals — list deferred checks
	huma.Register(api, huma.Operation{OperationID: "list-doctor-deferrals", Method: http.MethodGet, Path: "/api/dx/doctor/deferrals"},
		func(ctx context.Context, in *IssueSlugInput) (*struct {
			Body struct {
				Deferrals []DeferralItem `json:"deferrals"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			rows, err := h.Q.ListDoctorDeferrals(ctx, p.ID)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]DeferralItem, len(rows))
			for i, r := range rows {
				out[i] = DeferralItem{
					CheckName:  r.CheckName,
					Rung:       r.Rung,
					DeferredAt: fmtTS(r.DeferredAt),
				}
			}
			return &struct {
				Body struct {
					Deferrals []DeferralItem `json:"deferrals"`
				}
			}{Body: struct {
				Deferrals []DeferralItem `json:"deferrals"`
			}{Deferrals: out}}, nil
		})

	// GET /api/dx/doctor/force-closed-no-substance — closed issues force-closed
	// (wontfix/duplicate/link) with zero substantive work-log entries.
	huma.Register(api, huma.Operation{OperationID: "list-force-closed-no-substance", Method: http.MethodGet, Path: "/api/dx/doctor/force-closed-no-substance"},
		func(ctx context.Context, in *IssueSlugInput) (*struct {
			Body struct {
				Issues []ForceClosedNoSubstanceItem `json:"issues"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			rows, err := h.Q.ListForceClosedNoSubstance(ctx, p.ID)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]ForceClosedNoSubstanceItem, len(rows))
			for i, r := range rows {
				out[i] = ForceClosedNoSubstanceItem{
					ID:        r.ID,
					Title:     r.Title,
					CloseNote: r.CloseNote,
				}
			}
			return &struct {
				Body struct {
					Issues []ForceClosedNoSubstanceItem `json:"issues"`
				}
			}{Body: struct {
				Issues []ForceClosedNoSubstanceItem `json:"issues"`
			}{Issues: out}}, nil
		})

	// GET /api/dx/doctor/historical-close-gate — closed issues that fail the
	// IS-560 close-gate retroactively (one row per (issue, gate) offense).
	huma.Register(api, huma.Operation{OperationID: "list-historical-close-gate-offenders", Method: http.MethodGet, Path: "/api/dx/doctor/historical-close-gate"},
		func(ctx context.Context, in *IssueSlugInput) (*struct {
			Body struct {
				Offenders []HistoricalCloseGateOffender `json:"offenders"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			rows, err := h.Q.ListHistoricalCloseGateOffenders(ctx, p.ID)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]HistoricalCloseGateOffender, len(rows))
			for i, r := range rows {
				out[i] = HistoricalCloseGateOffender{
					IssueID: r.IssueID,
					Gate:    r.Gate,
					Detail:  r.Detail,
				}
			}
			return &struct {
				Body struct {
					Offenders []HistoricalCloseGateOffender `json:"offenders"`
				}
			}{Body: struct {
				Offenders []HistoricalCloseGateOffender `json:"offenders"`
			}{Offenders: out}}, nil
		})

	// GET /api/dx/doctor/concern-state — aggregate concern coverage counts for doctor
	huma.Register(api, huma.Operation{OperationID: "get-concern-doctor-state", Method: http.MethodGet, Path: "/api/dx/doctor/concern-state"},
		func(ctx context.Context, in *IssueSlugInput) (*struct {
			Body struct {
				ConcernCount               int32 `json:"concern_count"`
				FeaturesWithConcerns       int32 `json:"features_with_concerns"`
				ConcernsWithSpecsNoPattern int32 `json:"concerns_with_specs_no_pattern"`
				SecurityConcernSpecCount   int32 `json:"security_concern_spec_count"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			row, err := h.Q.GetConcernDoctorState(ctx, p.ID)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := &struct {
				Body struct {
					ConcernCount               int32 `json:"concern_count"`
					FeaturesWithConcerns       int32 `json:"features_with_concerns"`
					ConcernsWithSpecsNoPattern int32 `json:"concerns_with_specs_no_pattern"`
					SecurityConcernSpecCount   int32 `json:"security_concern_spec_count"`
				}
			}{}
			out.Body.ConcernCount = row.ConcernCount
			out.Body.FeaturesWithConcerns = row.FeaturesWithConcerns
			out.Body.ConcernsWithSpecsNoPattern = row.ConcernsWithSpecsNoPattern
			out.Body.SecurityConcernSpecCount = row.SecurityConcernSpecCount
			return out, nil
		})
}

type DeferralItem struct {
	CheckName  string `json:"check_name"`
	Rung       string `json:"rung"`
	DeferredAt string `json:"deferred_at"`
}

type ForceClosedNoSubstanceItem struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	CloseNote string `json:"close_note"`
}

type HistoricalCloseGateOffender struct {
	IssueID string `json:"issue_id"`
	Gate    string `json:"gate"`
	Detail  string `json:"detail"`
}
