package handlers

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/iodesystems/zdx-go/internal/db"
)

func (h *Handler) registerPlanRoutes(api huma.API) {
	// ── List plans ──────────────────────────────────────────────────────────
	huma.Register(api, huma.Operation{OperationID: "list-plans", Method: http.MethodGet, Path: "/api/dx/plans"},
		func(ctx context.Context, in *IssueSlugInput) (*struct {
			Body struct {
				Plans []PlanItem `json:"plans"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			rows, err := h.Q.ListPlans(ctx, p.ID)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]PlanItem, len(rows))
			for i, r := range rows {
				out[i] = planItemFromList(r)
			}
			return &struct {
				Body struct {
					Plans []PlanItem `json:"plans"`
				}
			}{Body: struct {
				Plans []PlanItem `json:"plans"`
			}{Plans: out}}, nil
		})

	// ── Get plan ────────────────────────────────────────────────────────────
	huma.Register(api, huma.Operation{OperationID: "get-plan", Method: http.MethodGet, Path: "/api/dx/plan"},
		func(ctx context.Context, in *struct {
			Slug string `query:"slug" required:"true"`
			ID   int32  `query:"id" required:"true"`
		}) (*struct{ Body PlanItem }, error) {
			_, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			row, err := h.Q.GetPlan(ctx, in.ID)
			if err != nil {
				return nil, apiErr(http.StatusNotFound, "plan not found")
			}
			item := planItemFromGet(row)

			// Load steps + refs
			steps, err := h.Q.ListPlanSteps(ctx, row.ID)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			for _, s := range steps {
				si := planStepItemFromRow(s)
				refs, err := h.Q.ListPlanStepRefs(ctx, s.ID)
				if err != nil {
					return nil, apiErr(500, err.Error())
				}
				for _, ref := range refs {
					si.Refs = append(si.Refs, PlanStepRefItem{
						StepID:     ref.StepID,
						TargetType: ref.TargetType,
						TargetID:   ref.TargetID,
					})
				}
				item.Steps = append(item.Steps, si)
			}
			return &struct{ Body PlanItem }{Body: item}, nil
		})

	// ── Add plan ────────────────────────────────────────────────────────────
	huma.Register(api, huma.Operation{OperationID: "add-plan", Method: http.MethodPost, Path: "/api/dx/plan/add"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug        string `json:"slug"`
				Title       string `json:"title"`
				Body        string `json:"body" required:"false"`
				PlanType    string `json:"plan_type" required:"false"`
				Complexity  string `json:"complexity" required:"false"`
				Approach    string `json:"approach" required:"false"`
				FeatureName string `json:"feature_name" required:"false"`
				FocusID     int32  `json:"focus_id" required:"false"`
				IssueID     string `json:"issue_id" required:"false"`
			}
		}) (*struct{ Body PlanItem }, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}

			featureID := pgtype.Int4{}
			if in.Body.FeatureName != "" {
				feat, err := h.Q.GetFeature(ctx, db.GetFeatureParams{
					ProjectID: p.ID,
					Name:      in.Body.FeatureName,
				})
				if err != nil {
					return nil, apiErr(http.StatusNotFound, "feature not found: "+in.Body.FeatureName)
				}
				featureID = pgtype.Int4{Int32: feat.ID, Valid: true}
			}

			focusID := pgtype.Int4{}
			if in.Body.FocusID != 0 {
				focusID = pgtype.Int4{Int32: in.Body.FocusID, Valid: true}
			}

			issueID := pgtype.Text{}
			if in.Body.IssueID != "" {
				issueID = pgtype.Text{String: in.Body.IssueID, Valid: true}
			}

			row, err := h.Q.CreatePlan(ctx, db.CreatePlanParams{
				ProjectID:  p.ID,
				Title:      in.Body.Title,
				Body:       in.Body.Body,
				PlanType:   in.Body.PlanType,
				Complexity: in.Body.Complexity,
				Approach:   in.Body.Approach,
				FeatureID:  featureID,
				FocusID:    focusID,
				IssueID:    issueID,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			traceEvent(ctx, h.Q, p.ID, "plan.created", "plan_id", row.ID, "title", row.Title)
			return &struct{ Body PlanItem }{Body: planItemFromCreate(row)}, nil
		})

	// ── Update plan ─────────────────────────────────────────────────────────
	huma.Register(api, huma.Operation{OperationID: "update-plan", Method: http.MethodPost, Path: "/api/dx/plan/update"},
		func(ctx context.Context, in *struct {
			Body struct {
				ID         int32  `json:"id"`
				Title      string `json:"title"`
				Body       string `json:"body"`
				PlanType   string `json:"plan_type"`
				Status     string `json:"status"`
				Complexity string `json:"complexity"`
				Approach   string `json:"approach"`
			}
		}) (*struct{ Body OKBody }, error) {
			if err := h.Q.UpdatePlan(ctx, db.UpdatePlanParams{
				ID:         in.Body.ID,
				Title:      in.Body.Title,
				Body:       in.Body.Body,
				PlanType:   in.Body.PlanType,
				Status:     in.Body.Status,
				Complexity: in.Body.Complexity,
				Approach:   in.Body.Approach,
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	// ── Add plan step ───────────────────────────────────────────────────────
	huma.Register(api, huma.Operation{OperationID: "add-plan-step", Method: http.MethodPost, Path: "/api/dx/plan/step/add"},
		func(ctx context.Context, in *struct {
			Body struct {
				PlanID    int32  `json:"plan_id"`
				Text      string `json:"text"`
				Seq       int32  `json:"seq" required:"false"`
				DependsOn int32  `json:"depends_on" required:"false"`
			}
		}) (*struct{ Body PlanStepItem }, error) {
			dependsOn := pgtype.Int4{}
			if in.Body.DependsOn != 0 {
				dependsOn = pgtype.Int4{Int32: in.Body.DependsOn, Valid: true}
			}
			row, err := h.Q.CreatePlanStep(ctx, db.CreatePlanStepParams{
				PlanID:    in.Body.PlanID,
				Seq:       in.Body.Seq,
				Text:      in.Body.Text,
				DependsOn: dependsOn,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body PlanStepItem }{Body: planStepItemFromRow(row)}, nil
		})

	// ── Update plan step ────────────────────────────────────────────────────
	huma.Register(api, huma.Operation{OperationID: "update-plan-step", Method: http.MethodPost, Path: "/api/dx/plan/step/update"},
		func(ctx context.Context, in *struct {
			Body struct {
				ID        int32  `json:"id"`
				Text      string `json:"text"`
				Status    string `json:"status"`
				DependsOn int32  `json:"depends_on"`
			}
		}) (*struct{ Body OKBody }, error) {
			dependsOn := pgtype.Int4{}
			if in.Body.DependsOn != 0 {
				dependsOn = pgtype.Int4{Int32: in.Body.DependsOn, Valid: true}
			}
			if err := h.Q.UpdatePlanStep(ctx, db.UpdatePlanStepParams{
				ID:        in.Body.ID,
				Text:      in.Body.Text,
				Status:    in.Body.Status,
				DependsOn: dependsOn,
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	// ── Add plan step ref ───────────────────────────────────────────────────
	huma.Register(api, huma.Operation{OperationID: "add-plan-step-ref", Method: http.MethodPost, Path: "/api/dx/plan/step/ref/add"},
		func(ctx context.Context, in *struct {
			Body struct {
				StepID     int32  `json:"step_id"`
				TargetType string `json:"target_type"`
				TargetID   string `json:"target_id"`
			}
		}) (*struct{ Body OKBody }, error) {
			if err := h.Q.CreatePlanStepRef(ctx, db.CreatePlanStepRefParams{
				StepID:     in.Body.StepID,
				TargetType: in.Body.TargetType,
				TargetID:   in.Body.TargetID,
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})
}

// ── Converters ───────────────────────────────────────────────────────────────

func textVal(v pgtype.Text) string {
	if v.Valid {
		return v.String
	}
	return ""
}

func planItemFromList(r db.ListPlansRow) PlanItem {
	return PlanItem{
		ID:         r.ID,
		ProjectID:  r.ProjectID,
		Title:      r.Title,
		Body:       r.Body,
		PlanType:   r.PlanType,
		Status:     r.Status,
		Complexity: r.Complexity,
		Approach:   r.Approach,
		FeatureID:  int4Val(r.FeatureID),
		FocusID:    int4Val(r.FocusID),
		IssueID:    textVal(r.IssueID),
		CreatedAt:  fmtTS(r.CreatedAt),
		UpdatedAt:  fmtTS(r.UpdatedAt),
	}
}

func planItemFromGet(r db.GetPlanRow) PlanItem {
	return PlanItem{
		ID:         r.ID,
		ProjectID:  r.ProjectID,
		Title:      r.Title,
		Body:       r.Body,
		PlanType:   r.PlanType,
		Status:     r.Status,
		Complexity: r.Complexity,
		Approach:   r.Approach,
		FeatureID:  int4Val(r.FeatureID),
		FocusID:    int4Val(r.FocusID),
		IssueID:    textVal(r.IssueID),
		CreatedAt:  fmtTS(r.CreatedAt),
		UpdatedAt:  fmtTS(r.UpdatedAt),
	}
}

func planItemFromCreate(r db.CreatePlanRow) PlanItem {
	return PlanItem{
		ID:         r.ID,
		ProjectID:  r.ProjectID,
		Title:      r.Title,
		Body:       r.Body,
		PlanType:   r.PlanType,
		Status:     r.Status,
		Complexity: r.Complexity,
		Approach:   r.Approach,
		FeatureID:  int4Val(r.FeatureID),
		FocusID:    int4Val(r.FocusID),
		IssueID:    textVal(r.IssueID),
		CreatedAt:  fmtTS(r.CreatedAt),
		UpdatedAt:  fmtTS(r.UpdatedAt),
	}
}

func planStepItemFromRow(r db.ZdxPlanStep) PlanStepItem {
	return PlanStepItem{
		ID:        r.ID,
		PlanID:    r.PlanID,
		Seq:       r.Seq,
		Text:      r.Text,
		Status:    r.Status,
		DependsOn: int4Val(r.DependsOn),
		CreatedAt: fmtTS(r.CreatedAt),
		UpdatedAt: fmtTS(r.UpdatedAt),
	}
}
