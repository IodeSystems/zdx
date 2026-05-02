package handlers

import (
	"testing"

	"github.com/iodesystems/zdx-go/internal/db"
)

func TestSpecCoverageMapping(t *testing.T) {
	cases := []struct {
		name string
		row  db.GetFeatureCoverageRow
		want SpecCoverageItem
	}{
		{
			name: "all layers covered",
			row:  db.GetFeatureCoverageRow{SpecID: 1, HasUnit: true, HasIntegration: true, HasDemo: true},
			want: SpecCoverageItem{SpecID: 1, HasUnit: true, HasIntegration: true, HasDemo: true},
		},
		{
			name: "no coverage",
			row:  db.GetFeatureCoverageRow{SpecID: 2, HasUnit: false, HasIntegration: false, HasDemo: false},
			want: SpecCoverageItem{SpecID: 2, HasUnit: false, HasIntegration: false, HasDemo: false},
		},
		{
			name: "unit only",
			row:  db.GetFeatureCoverageRow{SpecID: 3, HasUnit: true},
			want: SpecCoverageItem{SpecID: 3, HasUnit: true},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := SpecCoverageItem{
				SpecID:         tc.row.SpecID,
				HasUnit:        tc.row.HasUnit,
				HasIntegration: tc.row.HasIntegration,
				HasDemo:        tc.row.HasDemo,
			}
			if got != tc.want {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}
