package kpidelta

import (
	"context"
	"encoding/json"

	"github.com/iodesystems/zdx-go/internal/db"
)

type Delta struct {
	CheckName string  `json:"check_name"`
	Unit      string  `json:"unit"`
	Prev      float64 `json:"prev"`
	Curr      float64 `json:"curr"`
	Pct       float64 `json:"pct"`
}

// Compute returns KPI deltas for the given project+scope by comparing the
// latest two samples per check_name. Checks with only one sample are skipped.
// Returns an empty slice (never nil) if no samples exist.
func Compute(ctx context.Context, q db.Querier, projectID int32, scope string) ([]Delta, error) {
	rows, err := q.LatestTwoKPISamplesPerCheck(ctx, db.LatestTwoKPISamplesPerCheckParams{
		ProjectID: projectID,
		Scope:     scope,
	})
	if err != nil {
		return []Delta{}, err
	}

	// Group rows by check_name; rows are ordered check_name ASC, rn ASC (1=latest, 2=prev).
	type pair struct {
		curr *db.LatestTwoKPISamplesPerCheckRow
		prev *db.LatestTwoKPISamplesPerCheckRow
		unit string
	}
	groups := map[string]*pair{}
	order := []string{}
	for i := range rows {
		r := &rows[i]
		if _, ok := groups[r.CheckName]; !ok {
			groups[r.CheckName] = &pair{unit: r.Unit}
			order = append(order, r.CheckName)
		}
		p := groups[r.CheckName]
		if p.curr == nil {
			p.curr = r
		} else {
			p.prev = r
		}
	}

	deltas := make([]Delta, 0, len(order))
	for _, name := range order {
		p := groups[name]
		if p.curr == nil || p.prev == nil {
			continue
		}
		curr := p.curr.Value
		prev := p.prev.Value
		var pct float64
		if prev != 0 {
			pct = (curr - prev) / prev * 100
		}
		deltas = append(deltas, Delta{
			CheckName: name,
			Unit:      p.unit,
			Prev:      prev,
			Curr:      curr,
			Pct:       pct,
		})
	}
	return deltas, nil
}

func ToJSON(deltas []Delta) string {
	if len(deltas) == 0 {
		return "[]"
	}
	b, _ := json.Marshal(deltas)
	return string(b)
}
