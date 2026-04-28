package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"github.com/iodesystems/zdx-go/internal/db"
)

func (h *Handler) registerFocusRoutes(api huma.API) {
	huma.Register(api, huma.Operation{OperationID: "list-focuses", Method: http.MethodGet, Path: "/api/dx/focuses"},
		func(ctx context.Context, in *IssueSlugInput) (*struct {
			Body struct {
				Focuses []FocusItem `json:"focuses"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			rows, err := h.Q.ListFocuses(ctx, p.ID)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]FocusItem, len(rows))
			for i, r := range rows {
				var attributions []FocusAttribution
				if raw, ok := r.Attributions.([]byte); ok {
					_ = json.Unmarshal(raw, &attributions)
				} else if rawStr, ok := r.Attributions.(string); ok {
					_ = json.Unmarshal([]byte(rawStr), &attributions)
				}
				if attributions == nil {
					attributions = []FocusAttribution{}
				}
				out[i] = FocusItem{
					ID:           r.ID,
					Name:         r.Name,
					Description:  r.Description,
					Priority:     r.Priority,
					Status:       r.Status,
					Attributions: attributions,
					StartedAt:    fmtTS(r.StartedAt),
					EndedAt:      fmtTS(r.EndedAt),
					CreatedAt:    fmtTS(r.CreatedAt),
				}
			}
			return &struct {
				Body struct {
					Focuses []FocusItem `json:"focuses"`
				}
			}{Body: struct {
				Focuses []FocusItem `json:"focuses"`
			}{Focuses: out}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "search-focuses", Method: http.MethodPost, Path: "/api/dx/focuses/search"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug  string `json:"slug"`
				Query string `json:"query"`
			}
		}) (*struct {
			Body struct {
				Focuses []SearchFocusItem `json:"focuses"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			rows, err := h.Q.SearchFocuses(ctx, db.SearchFocusesParams{ProjectID: p.ID, Query: in.Body.Query})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]SearchFocusItem, len(rows))
			for i, r := range rows {
				out[i] = SearchFocusItem{ID: r.ID, Name: r.Name, Description: r.Description, Status: r.Status}
			}
			return &struct {
				Body struct {
					Focuses []SearchFocusItem `json:"focuses"`
				}
			}{Body: struct {
				Focuses []SearchFocusItem `json:"focuses"`
			}{Focuses: out}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "add-focus", Method: http.MethodPost, Path: "/api/dx/focuses/add"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug        string `json:"slug"`
				Name        string `json:"name"`
				Description string `json:"description"`
				Priority    int32  `json:"priority"`
				Blockers    string `json:"blockers"`
			}
		}) (*struct{ Body FocusItem }, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			row, err := h.Q.CreateFocus(ctx, db.CreateFocusParams{
				ProjectID:   p.ID,
				Name:        in.Body.Name,
				Description: in.Body.Description,
				Priority:    in.Body.Priority,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body FocusItem }{Body: FocusItem{
				ID:          row.ID,
				Name:        row.Name,
				Description: row.Description,
				Priority:    row.Priority,
				Status:      row.Status,
				CreatedAt:   fmtTS(row.CreatedAt),
			}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "set-focus-status", Method: http.MethodPost, Path: "/api/dx/focuses/status"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug   string `json:"slug"`
				Focus  string `json:"focus"` // "FO-N" or name
				Status string `json:"status"`
			}
		}) (*struct{ Body OKBody }, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			focus, err := h.resolveFocus(ctx, p.ID, in.Body.Focus)
			if err != nil {
				return nil, err
			}
			if err := h.Q.UpdateFocusStatus(ctx, db.UpdateFocusStatusParams{
				ProjectID: p.ID,
				ID:        focus.ID,
				Status:    in.Body.Status,
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "add-focus-blocker", Method: http.MethodPost, Path: "/api/dx/focuses/block"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug          string `json:"slug"`
				Focus         string `json:"focus"`
				Issue         string `json:"issue"`
				Justification string `json:"justification,omitempty"`
			}
		}) (*struct{ Body OKBody }, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			focus, err := h.resolveFocus(ctx, p.ID, in.Body.Focus)
			if err != nil {
				return nil, err
			}
			if err := h.Q.AddFocusBlocker(ctx, db.AddFocusBlockerParams{
				FocusID:       focus.ID,
				IssueID:       in.Body.Issue,
				Justification: in.Body.Justification,
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "remove-focus-blocker", Method: http.MethodPost, Path: "/api/dx/focuses/unblock"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug  string `json:"slug"`
				Focus string `json:"focus"`
				Issue string `json:"issue"`
			}
		}) (*struct{ Body OKBody }, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			focus, err := h.resolveFocus(ctx, p.ID, in.Body.Focus)
			if err != nil {
				return nil, err
			}
			if err := h.Q.RemoveFocusBlocker(ctx, db.RemoveFocusBlockerParams{
				FocusID: focus.ID,
				IssueID: in.Body.Issue,
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			h.refreshQueueAsync(p.ID)
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})
}

// ── Internal helper ───────────────────────────────────────────────────────

func (h *Handler) resolveFocus(ctx context.Context, projectID int32, ref string) (db.ZdxFocuse, error) {
	// "FO-N" or legacy "TH-N" → integer lookup
	for _, prefix := range []string{"FO-", "TH-"} {
		if strings.HasPrefix(ref, prefix) {
			id := intFromPrefixed(ref, prefix)
			t, err := h.Q.GetFocusByID(ctx, db.GetFocusByIDParams{ProjectID: projectID, ID: id})
			if err != nil {
				return db.ZdxFocuse{}, apiErr(http.StatusNotFound, "focus not found: "+ref)
			}
			return t, nil
		}
	}
	t, err := h.Q.GetFocusByName(ctx, db.GetFocusByNameParams{ProjectID: projectID, Name: ref})
	if err != nil {
		return db.ZdxFocuse{}, apiErr(http.StatusNotFound, "focus not found: "+ref)
	}
	return t, nil
}
