package handlers

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/iodesystems/zdx-go/internal/db"
)

const demoURLTTL = time.Hour

func (h *Handler) registerFeatureRoutes(api huma.API) {
	type FeaturesOutput = struct {
		Body struct {
			Features []FeatureItem `json:"features"`
		}
	}

	// /api/dx/todo/list — feature list with specs and plan (used by CLI todo queue)
	huma.Register(api, huma.Operation{OperationID: "list-features-todo", Method: http.MethodGet, Path: "/api/dx/todo/list"},
		func(ctx context.Context, in *IssueSlugInput) (*FeaturesOutput, error) {
			return h.featuresWithSpecs(ctx, in.Slug)
		})

	// /api/features — same data, used by removeFeature lookup
	huma.Register(api, huma.Operation{OperationID: "list-features", Method: http.MethodGet, Path: "/api/features"},
		func(ctx context.Context, in *IssueSlugInput) (*FeaturesOutput, error) {
			return h.featuresWithSpecs(ctx, in.Slug)
		})

	huma.Register(api, huma.Operation{OperationID: "get-feature", Method: http.MethodGet, Path: "/api/dx/feature"},
		func(ctx context.Context, in *struct {
			Slug string `query:"slug" required:"true"`
			Name string `query:"name" required:"true"`
		}) (*struct{ Body FeatureItem }, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			feat, err := h.Q.GetFeature(ctx, db.GetFeatureParams{ProjectID: p.ID, Name: in.Name})
			if err != nil {
				return nil, apiErr(http.StatusNotFound, "feature not found: "+in.Name)
			}
			specs, _ := h.Q.ListSpecs(ctx, feat.ID)
			return &struct{ Body FeatureItem }{Body: toFeatureItemFromGet(feat, specs)}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "upsert-feature", Method: http.MethodPost, Path: "/api/feature"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug        string `json:"slug"`
				Name        string `json:"name"`
				Description string `json:"description"`
			}
		}) (*struct{ Body FeatureItem }, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			row, err := h.Q.UpsertFeature(ctx, db.UpsertFeatureParams{
				ProjectID:   p.ID,
				Name:        in.Body.Name,
				Description: in.Body.Description,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body FeatureItem }{Body: toFeatureItemFromUpsert(row)}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "delete-feature", Method: http.MethodDelete, Path: "/api/feature"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug  string `json:"slug"`
				Name  string `json:"name"`
				Force bool   `json:"force,omitempty"`
			}
		}) (*struct{ Body OKBody }, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			feat, err := h.Q.GetFeature(ctx, db.GetFeatureParams{ProjectID: p.ID, Name: in.Body.Name})
			if err != nil {
				return nil, apiErr(http.StatusNotFound, "feature not found: "+in.Body.Name)
			}
			specs, _ := h.Q.ListSpecs(ctx, feat.ID)
			children, _ := h.Q.ListChildFeatures(ctx, pgtype.Int4{Int32: feat.ID, Valid: true})
			if !in.Body.Force && (len(specs) > 0 || len(children) > 0) {
				return nil, apiErr(http.StatusBadRequest, fmt.Sprintf(
					"feature %q has %d spec(s) and %d child feature(s) — pass force=true to cascade (specs will be deleted, children detached)",
					in.Body.Name, len(specs), len(children)))
			}
			for _, child := range children {
				if err := h.Q.ClearFeatureParent(ctx, child.ID); err != nil {
					return nil, apiErr(500, err.Error())
				}
			}
			if err := h.Q.DeleteFeature(ctx, feat.ID); err != nil {
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
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			if err := h.Q.UpdateFeatureField(ctx, db.UpdateFeatureFieldParams{
				ProjectID: p.ID,
				Name:      in.Body.Feature,
				Field:     in.Body.Field,
				Value:     in.Body.Value,
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "set-feature-parent", Method: http.MethodPost, Path: "/api/dx/features/parent"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug    string `json:"slug"`
				Feature string `json:"feature"`
				Parent  string `json:"parent"`
			}
		}) (*struct{ Body OKBody }, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			child, err := h.Q.GetFeature(ctx, db.GetFeatureParams{ProjectID: p.ID, Name: in.Body.Feature})
			if err != nil {
				return nil, apiErr(http.StatusNotFound, "feature not found: "+in.Body.Feature)
			}
			if in.Body.Parent == "" {
				if err := h.Q.ClearFeatureParent(ctx, child.ID); err != nil {
					return nil, apiErr(500, err.Error())
				}
				return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
			}
			parent, err := h.Q.GetFeature(ctx, db.GetFeatureParams{ProjectID: p.ID, Name: in.Body.Parent})
			if err != nil {
				return nil, apiErr(http.StatusNotFound, "parent feature not found: "+in.Body.Parent)
			}
			if parent.ID == child.ID {
				return nil, apiErr(http.StatusBadRequest, "feature cannot be its own parent")
			}
			if err := h.Q.UpdateFeatureParent(ctx, db.UpdateFeatureParentParams{
				ID:              child.ID,
				ParentFeatureID: pgtype.Int4{Int32: parent.ID, Valid: true},
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "set-feature-goal", Method: http.MethodPost, Path: "/api/dx/features/goal"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug    string `json:"slug"`
				Feature string `json:"feature"`
				GoalID  int32  `json:"goal_id"`
			}
		}) (*struct{ Body OKBody }, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			feature, err := h.Q.GetFeature(ctx, db.GetFeatureParams{ProjectID: p.ID, Name: in.Body.Feature})
			if err != nil {
				return nil, apiErr(http.StatusNotFound, "feature not found: "+in.Body.Feature)
			}
			if err := h.Q.UpdateFeatureGoal(ctx, db.UpdateFeatureGoalParams{
				ID:     feature.ID,
				GoalID: pgtype.Int4{Int32: in.Body.GoalID, Valid: in.Body.GoalID != 0},
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "move-spec", Method: http.MethodPost, Path: "/api/dx/specs/move"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug    string `json:"slug"`
				SpecID  int32  `json:"spec_id"`
				Feature string `json:"feature"`
			}
		}) (*struct{ Body OKBody }, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			target, err := h.Q.GetFeature(ctx, db.GetFeatureParams{ProjectID: p.ID, Name: in.Body.Feature})
			if err != nil {
				return nil, apiErr(http.StatusNotFound, "target feature not found: "+in.Body.Feature)
			}
			spec, err := h.Q.GetSpec(ctx, in.Body.SpecID)
			if err != nil {
				return nil, apiErr(http.StatusNotFound, "spec not found")
			}
			// Validate current spec belongs to the same project.
			curFeat, err := h.Q.GetFeatureByID(ctx, spec.FeatureID)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			if curFeat.ProjectID != p.ID {
				return nil, apiErr(http.StatusBadRequest, "spec belongs to a different project")
			}
			if err := h.Q.UpdateSpecFeature(ctx, db.UpdateSpecFeatureParams{
				ID:        in.Body.SpecID,
				FeatureID: target.ID,
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "delete-spec", Method: http.MethodPost, Path: "/api/dx/specs/delete"},
		func(ctx context.Context, in *struct {
			Body struct {
				SpecID int32 `json:"spec_id"`
			}
		}) (*struct{ Body OKBody }, error) {
			if err := h.Q.DeleteSpec(ctx, in.Body.SpecID); err != nil {
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
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			f, err := h.Q.GetFeature(ctx, db.GetFeatureParams{ProjectID: p.ID, Name: in.Body.Feature})
			if err != nil {
				return nil, apiErr(http.StatusNotFound, "feature not found")
			}
			// field is the spec kind (unit_test, api_test, ui_test); value is description
			_, err = h.Q.AddSpec(ctx, db.AddSpecParams{
				FeatureID:   f.ID,
				Description: in.Body.Value,
				Kind:        in.Body.Field,
				ConcernType: "functional",
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
			if err := h.Q.LinkSpecTest(ctx, db.LinkSpecTestParams{
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
			if err := h.Q.UnlinkSpecTest(ctx, db.UnlinkSpecTestParams{
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
			if err := h.Q.DeferSpec(ctx, db.DeferSpecParams{ID: in.Body.SpecID, Reason: in.Body.Reason}); err != nil {
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
			if err := h.Q.UndeferSpec(ctx, in.Body.SpecID); err != nil {
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
			spec, err := h.Q.GetSpec(ctx, in.SpecID)
			if err != nil {
				return nil, apiErr(404, "spec not found")
			}
			issueRows, _ := h.Q.ListSpecIssues(ctx, in.SpecID)
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
					ConcernType:    spec.ConcernType,
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
			if err := h.Q.LinkSpecIssue(ctx, db.LinkSpecIssueParams{SpecID: in.Body.SpecID, IssueID: in.Body.IssueID}); err != nil {
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
			if err := h.Q.UnlinkSpecIssue(ctx, db.UnlinkSpecIssueParams{SpecID: in.Body.SpecID, IssueID: in.Body.IssueID}); err != nil {
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
			rows, err := h.Q.ListTestsForSpec(ctx, in.SpecID)
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
			rows, err := h.Q.ListDemosForSpec(ctx, in.SpecID)
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
					TestID:        r.TestID,
					Type:          r.DemoType,
					TestComponent: r.TestComponent,
					TestName:      r.TestName,
					URL:           SignAssetURL(h.WSSecret, url, demoURLTTL),
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
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			days := in.StaleDays
			if days == 0 {
				days = 30
			}
			rows, err := h.Q.ListStaleFeatures(ctx, db.ListStaleFeaturesParams{ProjectID: p.ID, StaleDays: days})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]FeatureItem, len(rows))
			for i, r := range rows {
				out[i] = toFeatureItemFromStale(r)
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
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			if err := h.Q.MarkFeatureReviewed(ctx, db.MarkFeatureReviewedParams{ProjectID: p.ID, Name: in.Body.Feature}); err != nil {
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
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			rows, err := h.Q.ListUncoveredSpecs(ctx, p.ID)
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
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			rows, err := h.Q.ListSpecsWithoutDemos(ctx, p.ID)
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

	huma.Register(api, huma.Operation{OperationID: "list-specs-blocker-resolved", Method: http.MethodGet, Path: "/api/dx/specs/blocker-resolved"},
		func(ctx context.Context, in *IssueSlugInput) (*struct {
			Body struct {
				Specs []UncoveredSpecItem `json:"specs"`
			}
		}, error) {
			rows, err := h.Q.ListSpecsWithAllBlockersClosed(ctx)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]UncoveredSpecItem, len(rows))
			for i, r := range rows {
				out[i] = UncoveredSpecItem{
					ID:          r.ID,
					FeatureID:   r.FeatureID,
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

	// ── Spec deferrals (issue-backed) ─────────────────────────────────────────

	huma.Register(api, huma.Operation{OperationID: "add-spec-deferral", Method: http.MethodPost, Path: "/api/dx/specs/deferrals/add"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug    string `json:"slug"`
				SpecID  int32  `json:"spec_id"`
				IssueID string `json:"issue_id"`
				Note    string `json:"note,omitempty"`
			}
		}) (*struct{ Body OKBody }, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			if _, err := h.Q.GetSpec(ctx, in.Body.SpecID); err != nil {
				return nil, apiErr(http.StatusNotFound, fmt.Sprintf("spec not found: %d", in.Body.SpecID))
			}
			iss, err := h.Q.GetIssue(ctx, db.GetIssueParams{ProjectID: p.ID, ID: in.Body.IssueID})
			if err != nil {
				return nil, apiErr(http.StatusNotFound, fmt.Sprintf("issue not found: %s", in.Body.IssueID))
			}
			if iss.Status != "open" {
				return nil, apiErr(http.StatusBadRequest, fmt.Sprintf("cannot defer to closed issue %s (status=%s) — deferrals only make sense for open work", in.Body.IssueID, iss.Status))
			}
			if err := h.Q.AddSpecDeferral(ctx, db.AddSpecDeferralParams{
				SpecID:  in.Body.SpecID,
				IssueID: in.Body.IssueID,
				Note:    in.Body.Note,
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "remove-spec-deferral", Method: http.MethodPost, Path: "/api/dx/specs/deferrals/remove"},
		func(ctx context.Context, in *struct {
			Body struct {
				SpecID  int32  `json:"spec_id"`
				IssueID string `json:"issue_id"`
			}
		}) (*struct{ Body OKBody }, error) {
			if err := h.Q.RemoveSpecDeferral(ctx, db.RemoveSpecDeferralParams{
				SpecID:  in.Body.SpecID,
				IssueID: in.Body.IssueID,
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-spec-deferrals", Method: http.MethodGet, Path: "/api/dx/specs/deferrals"},
		func(ctx context.Context, in *struct {
			SpecID int32 `query:"spec_id" required:"true"`
		}) (*struct {
			Body struct {
				Deferrals []SpecDeferralItem `json:"deferrals"`
			}
		}, error) {
			rows, err := h.Q.ListSpecDeferrals(ctx, in.SpecID)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]SpecDeferralItem, len(rows))
			for i, r := range rows {
				out[i] = SpecDeferralItem{
					IssueID:     r.IssueID,
					IssueTitle:  r.IssueTitle,
					IssueStatus: r.IssueStatus,
					Note:        r.Note,
				}
			}
			return &struct {
				Body struct {
					Deferrals []SpecDeferralItem `json:"deferrals"`
				}
			}{Body: struct {
				Deferrals []SpecDeferralItem `json:"deferrals"`
			}{Deferrals: out}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-deferred-specs", Method: http.MethodGet, Path: "/api/dx/specs/deferred"},
		func(ctx context.Context, in *IssueSlugInput) (*struct {
			Body struct {
				Specs []DeferredSpecItem `json:"specs"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			rows, err := h.Q.ListDeferredSpecsWithFeatureForProject(ctx, p.ID)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]DeferredSpecItem, 0, len(rows))
			for _, r := range rows {
				defRows, _ := h.Q.ListSpecDeferrals(ctx, r.ID)
				blockers := make([]SpecDeferralItem, 0, len(defRows))
				for _, d := range defRows {
					if d.IssueStatus != "open" {
						continue
					}
					blockers = append(blockers, SpecDeferralItem{
						IssueID:     d.IssueID,
						IssueTitle:  d.IssueTitle,
						IssueStatus: d.IssueStatus,
						Note:        d.Note,
					})
				}
				out = append(out, DeferredSpecItem{
					ID:          r.ID,
					FeatureID:   r.FeatureID,
					FeatureName: r.FeatureName,
					Description: r.Description,
					Kind:        r.Kind,
					ConcernType: r.ConcernType,
					Blockers:    blockers,
				})
			}
			return &struct {
				Body struct {
					Specs []DeferredSpecItem `json:"specs"`
				}
			}{Body: struct {
				Specs []DeferredSpecItem `json:"specs"`
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
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			f, err := h.Q.GetFeature(ctx, db.GetFeatureParams{ProjectID: p.ID, Name: in.Body.Feature})
			if err != nil {
				return nil, apiErr(http.StatusNotFound, "feature not found")
			}
			_, err = h.Q.CreatePlan(ctx, db.CreatePlanParams{
				ProjectID:  p.ID,
				FeatureID:  pgtype.Int4{Int32: f.ID, Valid: true},
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

func (h *Handler) featuresWithSpecs(ctx context.Context, slug string) (*struct {
	Body struct {
		Features []FeatureItem `json:"features"`
	}
}, error) {
	p, err := getProject(ctx, h.Q, slug)
	if err != nil {
		return nil, err
	}
	rows, err := h.Q.ListFeatures(ctx, p.ID)
	if err != nil {
		return nil, apiErr(500, err.Error())
	}
	// Fetch all specs in one query and group by feature_id.
	allSpecs, _ := h.Q.ListSpecsForProject(ctx, p.ID)
	specsByFeature := make(map[int32][]SpecItem, len(allSpecs))
	for _, sp := range allSpecs {
		specsByFeature[sp.FeatureID] = append(specsByFeature[sp.FeatureID], SpecItem{
			ID: sp.ID, Description: sp.Description, Kind: sp.Kind,
			ConcernType: sp.ConcernType, Deferred: sp.Deferred, DeferredReason: sp.DeferredReason,
		})
	}
	out := make([]FeatureItem, len(rows))
	for i, f := range rows {
		out[i] = toFeatureItemFromList(f, specsByFeature[f.ID])
	}
	return &struct {
		Body struct {
			Features []FeatureItem `json:"features"`
		}
	}{Body: struct {
		Features []FeatureItem `json:"features"`
	}{Features: out}}, nil
}

// ── Model → response converters ───────────────────────────────────────────

func int4Val(v pgtype.Int4) int32 {
	if v.Valid {
		return v.Int32
	}
	return 0
}

func toFeatureItemFromGet(f db.GetFeatureRow, specs []db.ListSpecsRow) FeatureItem {
	item := FeatureItem{
		ID: f.ID, Name: f.Name, Description: f.Description,
		What: f.What, Why: f.Why, DoneWhen: f.DoneWhen,
		Component: f.Component, Category: f.Category,
		Kind: f.Kind, GoalID: int4Val(f.GoalID), ParentFeatureID: int4Val(f.ParentFeatureID),
		MetricName: f.MetricName, MetricUnit: f.MetricUnit,
		BaselineValue: f.BaselineValue, TargetValue: f.TargetValue, GraphURL: f.GraphUrl,
		Specs: make([]SpecItem, len(specs)),
	}
	for i, sp := range specs {
		item.Specs[i] = SpecItem{ID: sp.ID, Description: sp.Description, Kind: sp.Kind, ConcernType: sp.ConcernType, Deferred: sp.Deferred, DeferredReason: sp.DeferredReason}
	}
	return item
}

func toFeatureItemFromList(f db.ListFeaturesRow, specs []SpecItem) FeatureItem {
	return FeatureItem{
		ID: f.ID, Name: f.Name, Description: f.Description,
		What: f.What, Why: f.Why, DoneWhen: f.DoneWhen,
		Component: f.Component, Category: f.Category,
		Kind: f.Kind, GoalID: int4Val(f.GoalID), ParentFeatureID: int4Val(f.ParentFeatureID),
		MetricName: f.MetricName, MetricUnit: f.MetricUnit,
		BaselineValue: f.BaselineValue, TargetValue: f.TargetValue, GraphURL: f.GraphUrl,
		Specs: specs,
	}
}

func toFeatureItemFromUpsert(f db.UpsertFeatureRow) FeatureItem {
	return FeatureItem{
		ID: f.ID, Name: f.Name, Description: f.Description,
		What: f.What, Why: f.Why, DoneWhen: f.DoneWhen,
		Component: f.Component, Category: f.Category,
		Kind: f.Kind, GoalID: int4Val(f.GoalID), ParentFeatureID: int4Val(f.ParentFeatureID),
		MetricName: f.MetricName, MetricUnit: f.MetricUnit,
		BaselineValue: f.BaselineValue, TargetValue: f.TargetValue, GraphURL: f.GraphUrl,
	}
}

func toFeatureItemFromStale(f db.ListStaleFeaturesRow) FeatureItem {
	return FeatureItem{
		ID: f.ID, Name: f.Name, Description: f.Description,
		What: f.What, Why: f.Why, DoneWhen: f.DoneWhen,
		Component: f.Component, Category: f.Category,
		Kind: f.Kind, GoalID: int4Val(f.GoalID), ParentFeatureID: int4Val(f.ParentFeatureID),
	}
}
