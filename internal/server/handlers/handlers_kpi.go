package handlers

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/iodesystems/zdx-go/internal/db"
)

func (h *Handler) registerKpiRoutes(api huma.API) {
	type KpiSampleItem struct {
		ID        int64   `json:"id"`
		SampledAt string  `json:"sampled_at"`
		Scope     string  `json:"scope"`
		CheckName string  `json:"check_name"`
		Value     float64 `json:"value"`
		Unit      string  `json:"unit"`
	}

	huma.Register(api, huma.Operation{OperationID: "post-kpi-sample", Method: http.MethodPost, Path: "/api/dx/kpi/sample"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug      string  `json:"slug"`
				Scope     string  `json:"scope"`
				CheckName string  `json:"check_name"`
				Value     float64 `json:"value"`
				Unit      string  `json:"unit"`
			}
		}) (*struct {
			Body struct {
				ID        int64  `json:"id"`
				SampledAt string `json:"sampled_at"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, apiErr(404, "project not found")
			}
			row, err := h.Q.InsertKpiSample(ctx, db.InsertKpiSampleParams{
				ProjectID: p.ID,
				Scope:     in.Body.Scope,
				CheckName: in.Body.CheckName,
				Value:     in.Body.Value,
				Unit:      in.Body.Unit,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct {
				Body struct {
					ID        int64  `json:"id"`
					SampledAt string `json:"sampled_at"`
				}
			}{Body: struct {
				ID        int64  `json:"id"`
				SampledAt string `json:"sampled_at"`
			}{ID: row.ID, SampledAt: fmtTS(row.SampledAt)}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "get-kpi-trend", Method: http.MethodGet, Path: "/api/dx/kpi/trend"},
		func(ctx context.Context, in *struct {
			Slug      string `query:"slug"`
			Scope     string `query:"scope"`
			CheckName string `query:"check_name"`
			N         int32  `query:"n"`
		}) (*struct {
			Body struct {
				Items []KpiSampleItem `json:"items"`
			}
		}, error) {
			n := in.N
			if n <= 0 {
				n = 50
			}
			if n > 1000 {
				n = 1000
			}
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return &struct {
					Body struct {
						Items []KpiSampleItem `json:"items"`
					}
				}{Body: struct {
					Items []KpiSampleItem `json:"items"`
				}{Items: []KpiSampleItem{}}}, nil
			}
			rows, err := h.Q.ListKpiTrend(ctx, db.ListKpiTrendParams{
				ProjectID: p.ID,
				Scope:     in.Scope,
				CheckName: in.CheckName,
				N:         n,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			items := make([]KpiSampleItem, len(rows))
			for i, r := range rows {
				items[i] = KpiSampleItem{
					ID:        r.ID,
					SampledAt: fmtTS(r.SampledAt),
					Scope:     r.Scope,
					CheckName: r.CheckName,
					Value:     r.Value,
					Unit:      r.Unit,
				}
			}
			return &struct {
				Body struct {
					Items []KpiSampleItem `json:"items"`
				}
			}{Body: struct {
				Items []KpiSampleItem `json:"items"`
			}{Items: items}}, nil
		})
}
