package server

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"github.com/iodesystems/zdx-go/internal/db"
)

func (s *Server) registerFeatureRoutes(api huma.API) {
	type FeaturesOutput = struct {
		Body struct {
			Features []FeatureItem `json:"features"`
		}
	}

	// /api/dx/todo/list — feature list with specs and plan (used by CLI todo queue)
	huma.Register(api, huma.Operation{OperationID: "list-features-todo", Method: http.MethodGet, Path: "/api/dx/todo/list"},
		func(ctx context.Context, in *IssueSlugInput) (*FeaturesOutput, error) {
			return s.featuresWithSpecs(ctx, in.Slug)
		})

	// /api/features — same data, used by removeFeature lookup
	huma.Register(api, huma.Operation{OperationID: "list-features", Method: http.MethodGet, Path: "/api/features"},
		func(ctx context.Context, in *IssueSlugInput) (*FeaturesOutput, error) {
			return s.featuresWithSpecs(ctx, in.Slug)
		})

	huma.Register(api, huma.Operation{OperationID: "get-feature", Method: http.MethodGet, Path: "/api/dx/feature"},
		func(ctx context.Context, in *struct {
			Slug string `query:"slug" required:"true"`
			Name string `query:"name" required:"true"`
		}) (*struct{ Body FeatureItem }, error) {
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			feat, err := s.q.GetFeature(ctx, db.GetFeatureParams{ProjectID: p.ID, Name: in.Name})
			if err != nil {
				return nil, apiErr(http.StatusNotFound, "feature not found: "+in.Name)
			}
			specs, _ := s.q.ListSpecs(ctx, feat.ID)
			return &struct{ Body FeatureItem }{Body: toFeatureItem(feat, specs)}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "upsert-feature", Method: http.MethodPost, Path: "/api/feature"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug        string `json:"slug"`
				Name        string `json:"name"`
				Description string `json:"description"`
			}
		}) (*struct{ Body FeatureItem }, error) {
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			row, err := s.q.UpsertFeature(ctx, db.UpsertFeatureParams{
				ProjectID:   p.ID,
				Name:        in.Body.Name,
				Description: in.Body.Description,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body FeatureItem }{Body: toFeatureItem(row, nil)}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "delete-feature", Method: http.MethodDelete, Path: "/api/feature"},
		func(ctx context.Context, in *struct {
			Body struct {
				ID int32 `json:"id"`
			}
		}) (*struct{ Body OKBody }, error) {
			if err := s.q.DeleteFeature(ctx, in.Body.ID); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "set-feature-field", Method: http.MethodPost, Path: "/api/dx/features/field"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug    string `json:"slug"`
				Feature string `json:"feature"`
				Field   string `json:"field"`
				Value   string `json:"value"`
			}
		}) (*struct{ Body OKBody }, error) {
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			if err := s.q.UpdateFeatureField(ctx, db.UpdateFeatureFieldParams{
				ProjectID: p.ID,
				Name:      in.Body.Feature,
				Field:     in.Body.Field,
				Value:     in.Body.Value,
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "update-specs", Method: http.MethodPost, Path: "/api/dx/specs/update"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug    string `json:"slug"`
				Feature string `json:"feature"`
				Field   string `json:"field"`
				Value   string `json:"value"`
			}
		}) (*struct{ Body OKBody }, error) {
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			f, err := s.q.GetFeature(ctx, db.GetFeatureParams{ProjectID: p.ID, Name: in.Body.Feature})
			if err != nil {
				return nil, apiErr(http.StatusNotFound, "feature not found")
			}
			// field is the spec kind (unit_test, api_test, ui_test); value is description
			_, err = s.q.AddSpec(ctx, db.AddSpecParams{
				FeatureID:   f.ID,
				Description: in.Body.Value,
				Kind:        in.Body.Field,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "link-spec-test", Method: http.MethodPost, Path: "/api/dx/specs/link-test"},
		func(ctx context.Context, in *struct {
			Body struct {
				SpecID int32 `json:"spec_id"`
				TestID int32 `json:"test_id"`
			}
		}) (*struct{ Body OKBody }, error) {
			if err := s.q.LinkSpecTest(ctx, db.LinkSpecTestParams{
				SpecID: in.Body.SpecID,
				TestID: in.Body.TestID,
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "unlink-spec-test", Method: http.MethodPost, Path: "/api/dx/specs/unlink-test"},
		func(ctx context.Context, in *struct {
			Body struct {
				SpecID int32 `json:"spec_id"`
				TestID int32 `json:"test_id"`
			}
		}) (*struct{ Body OKBody }, error) {
			if err := s.q.UnlinkSpecTest(ctx, db.UnlinkSpecTestParams{
				SpecID: in.Body.SpecID,
				TestID: in.Body.TestID,
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "defer-spec", Method: http.MethodPost, Path: "/api/dx/specs/defer"},
		func(ctx context.Context, in *struct {
			Body struct {
				SpecID int32  `json:"spec_id"`
				Reason string `json:"reason"`
			}
		}) (*struct{ Body OKBody }, error) {
			if err := s.q.DeferSpec(ctx, db.DeferSpecParams{ID: in.Body.SpecID, Reason: in.Body.Reason}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "undefer-spec", Method: http.MethodPost, Path: "/api/dx/specs/undefer"},
		func(ctx context.Context, in *struct {
			Body struct {
				SpecID int32 `json:"spec_id"`
			}
		}) (*struct{ Body OKBody }, error) {
			if err := s.q.UndeferSpec(ctx, in.Body.SpecID); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "get-spec", Method: http.MethodGet, Path: "/api/dx/specs/detail"},
		func(ctx context.Context, in *struct {
			SpecID int32 `query:"spec_id" required:"true"`
		}) (*struct {
			Body struct {
				Spec   SpecItem        `json:"spec"`
				Issues []SpecIssueItem `json:"issues"`
			}
		}, error) {
			spec, err := s.q.GetSpec(ctx, in.SpecID)
			if err != nil {
				return nil, apiErr(404, "spec not found")
			}
			issueRows, _ := s.q.ListSpecIssues(ctx, in.SpecID)
			issues := make([]SpecIssueItem, len(issueRows))
			for i, r := range issueRows {
				issues[i] = SpecIssueItem{SpecID: r.SpecID, IssueID: r.IssueID, Title: r.Title, Status: r.Status}
			}
			return &struct {
				Body struct {
					Spec   SpecItem        `json:"spec"`
					Issues []SpecIssueItem `json:"issues"`
				}
			}{Body: struct {
				Spec   SpecItem        `json:"spec"`
				Issues []SpecIssueItem `json:"issues"`
			}{
				Spec: SpecItem{
					ID:             spec.ID,
					Description:    spec.Description,
					Kind:           spec.Kind,
					Deferred:       spec.Deferred,
					DeferredReason: spec.DeferredReason,
				},
				Issues: issues,
			}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "link-spec-issue", Method: http.MethodPost, Path: "/api/dx/specs/link-issue"},
		func(ctx context.Context, in *struct {
			Body struct {
				SpecID  int32  `json:"spec_id"`
				IssueID string `json:"issue_id"`
			}
		}) (*struct{ Body OKBody }, error) {
			if err := s.q.LinkSpecIssue(ctx, db.LinkSpecIssueParams{SpecID: in.Body.SpecID, IssueID: in.Body.IssueID}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "unlink-spec-issue", Method: http.MethodPost, Path: "/api/dx/specs/unlink-issue"},
		func(ctx context.Context, in *struct {
			Body struct {
				SpecID  int32  `json:"spec_id"`
				IssueID string `json:"issue_id"`
			}
		}) (*struct{ Body OKBody }, error) {
			if err := s.q.UnlinkSpecIssue(ctx, db.UnlinkSpecIssueParams{SpecID: in.Body.SpecID, IssueID: in.Body.IssueID}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-spec-tests", Method: http.MethodGet, Path: "/api/dx/specs/tests"},
		func(ctx context.Context, in *struct {
			SpecID int32 `query:"spec_id" required:"true"`
		}) (*struct {
			Body struct {
				Tests []SpecTestItem `json:"tests"`
			}
		}, error) {
			rows, err := s.q.ListTestsForSpec(ctx, in.SpecID)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]SpecTestItem, len(rows))
			for i, r := range rows {
				out[i] = SpecTestItem{ID: r.ID, Component: r.Component, Name: r.Name, Layer: r.Layer, Status: r.Status}
			}
			return &struct {
				Body struct {
					Tests []SpecTestItem `json:"tests"`
				}
			}{Body: struct {
				Tests []SpecTestItem `json:"tests"`
			}{Tests: out}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-spec-demos", Method: http.MethodGet, Path: "/api/dx/specs/demos"},
		func(ctx context.Context, in *struct {
			SpecID int32 `query:"spec_id" required:"true"`
		}) (*struct {
			Body struct {
				Demos []SpecDemoItem `json:"demos"`
			}
		}, error) {
			rows, err := s.q.ListDemosForSpec(ctx, in.SpecID)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]SpecDemoItem, len(rows))
			for i, r := range rows {
				name := strings.TrimSuffix(filepath.Base(r.ArtifactPath), filepath.Ext(r.ArtifactPath))
				url := fmt.Sprintf("/api/dx/demos/%s/%s", r.DemoType, name)
				if r.FileID.Valid {
					url = fmt.Sprintf("/api/files/%d", r.FileID.Int32)
				}
				out[i] = SpecDemoItem{
					ID:            r.ID,
					Type:          r.DemoType,
					TestComponent: r.TestComponent,
					TestName:      r.TestName,
					URL:           url,
					Name:          name,
				}
			}
			return &struct {
				Body struct {
					Demos []SpecDemoItem `json:"demos"`
				}
			}{Body: struct {
				Demos []SpecDemoItem `json:"demos"`
			}{Demos: out}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-stale-features", Method: http.MethodGet, Path: "/api/dx/features/stale"},
		func(ctx context.Context, in *struct {
			Slug      string `query:"slug" required:"true"`
			StaleDays int32  `query:"stale_days"`
		}) (*struct {
			Body struct {
				Features []FeatureItem `json:"features"`
			}
		}, error) {
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			days := in.StaleDays
			if days == 0 {
				days = 30
			}
			rows, err := s.q.ListStaleFeatures(ctx, db.ListStaleFeaturesParams{ProjectID: p.ID, StaleDays: days})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]FeatureItem, len(rows))
			for i, r := range rows {
				out[i] = toFeatureItem(r, nil)
			}
			return &struct {
				Body struct {
					Features []FeatureItem `json:"features"`
				}
			}{Body: struct {
				Features []FeatureItem `json:"features"`
			}{Features: out}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "mark-feature-reviewed", Method: http.MethodPost, Path: "/api/dx/feature/review"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug    string `json:"slug"`
				Feature string `json:"feature"`
			}
		}) (*struct{ Body OKBody }, error) {
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			if err := s.q.MarkFeatureReviewed(ctx, db.MarkFeatureReviewedParams{ProjectID: p.ID, Name: in.Body.Feature}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-uncovered-specs", Method: http.MethodGet, Path: "/api/dx/specs/uncovered"},
		func(ctx context.Context, in *IssueSlugInput) (*struct {
			Body struct {
				Specs []UncoveredSpecItem `json:"specs"`
			}
		}, error) {
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			rows, err := s.q.ListUncoveredSpecs(ctx, p.ID)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]UncoveredSpecItem, len(rows))
			for i, r := range rows {
				out[i] = UncoveredSpecItem{
					ID:          r.ID,
					FeatureID:   r.FeatureID,
					FeatureName: r.FeatureName,
					Description: r.Description,
					Kind:        r.Kind,
				}
			}
			return &struct {
				Body struct {
					Specs []UncoveredSpecItem `json:"specs"`
				}
			}{Body: struct {
				Specs []UncoveredSpecItem `json:"specs"`
			}{Specs: out}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-specs-without-demos", Method: http.MethodGet, Path: "/api/dx/specs/demo-gap"},
		func(ctx context.Context, in *IssueSlugInput) (*struct {
			Body struct {
				Specs []UncoveredSpecItem `json:"specs"`
			}
		}, error) {
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			rows, err := s.q.ListSpecsWithoutDemos(ctx, p.ID)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]UncoveredSpecItem, len(rows))
			for i, r := range rows {
				out[i] = UncoveredSpecItem{
					ID:          r.ID,
					FeatureID:   r.FeatureID,
					FeatureName: r.FeatureName,
					Description: r.Description,
					Kind:        r.Kind,
				}
			}
			return &struct {
				Body struct {
					Specs []UncoveredSpecItem `json:"specs"`
				}
			}{Body: struct {
				Specs []UncoveredSpecItem `json:"specs"`
			}{Specs: out}}, nil
		})

	// ── Plans ─────────────────────────────────────────────────────────────────

	huma.Register(api, huma.Operation{OperationID: "create-plan", Method: http.MethodPost, Path: "/api/dx/plan/create"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug       string `json:"slug"`
				Feature    string `json:"feature"`
				PlanType   string `json:"plan_type"`
				Complexity string `json:"complexity"`
				Approach   string `json:"approach"`
			}
		}) (*struct{ Body OKBody }, error) {
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			f, err := s.q.GetFeature(ctx, db.GetFeatureParams{ProjectID: p.ID, Name: in.Body.Feature})
			if err != nil {
				return nil, apiErr(http.StatusNotFound, "feature not found")
			}
			_, err = s.q.UpsertPlan(ctx, db.UpsertPlanParams{
				FeatureID:  f.ID,
				PlanType:   in.Body.PlanType,
				Complexity: in.Body.Complexity,
				Approach:   in.Body.Approach,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})
}

// ── Internal helpers ───────────────────────────────────────────────────────

func (s *Server) featuresWithSpecs(ctx context.Context, slug string) (*struct {
	Body struct {
		Features []FeatureItem `json:"features"`
	}
}, error) {
	p, err := getProject(ctx, s.q, slug)
	if err != nil {
		return nil, err
	}
	rows, err := s.q.ListFeatures(ctx, p.ID)
	if err != nil {
		return nil, apiErr(500, err.Error())
	}
	// Fetch all specs in one query and group by feature_id.
	allSpecs, _ := s.q.ListSpecsForProject(ctx, p.ID)
	specsByFeature := make(map[int32][]db.ZdxSpec, len(allSpecs))
	for _, sp := range allSpecs {
		specsByFeature[sp.FeatureID] = append(specsByFeature[sp.FeatureID], sp)
	}
	out := make([]FeatureItem, len(rows))
	for i, f := range rows {
		out[i] = toFeatureItem(f, specsByFeature[f.ID])
	}
	return &struct {
		Body struct {
			Features []FeatureItem `json:"features"`
		}
	}{Body: struct {
		Features []FeatureItem `json:"features"`
	}{Features: out}}, nil
}

// ── Model → response converter ────────────────────────────────────────────

func toFeatureItem(f db.ZdxFeature, specs []db.ZdxSpec) FeatureItem {
	item := FeatureItem{
		ID:          f.ID,
		Name:        f.Name,
		Description: f.Description,
		What:        f.What,
		Why:         f.Why,
		DoneWhen:    f.DoneWhen,
		Component:   f.Component,
		Category:    f.Category,
		Specs:       make([]SpecItem, len(specs)),
	}
	for i, sp := range specs {
		item.Specs[i] = SpecItem{
			ID:             sp.ID,
			Description:    sp.Description,
			Kind:           sp.Kind,
			Deferred:       sp.Deferred,
			DeferredReason: sp.DeferredReason,
		}
	}
	return item
}
