package project

import (
	"encoding/json"
	"fmt"
	"strings"
)

type kpiDelta struct {
	CheckName string  `json:"check_name"`
	Unit      string  `json:"unit"`
	Prev      float64 `json:"prev"`
	Curr      float64 `json:"curr"`
	Pct       float64 `json:"pct"`
}

// formatKPIDeltaLine formats a single delta as "  <check>  <curr><unit>  (<+pct>%)[*]".
// The regression marker * is added when |pct| > 50 or curr > 15 for time-unit checks.
func formatKPIDeltaLine(d kpiDelta) string {
	sign := "+"
	if d.Pct < 0 {
		sign = ""
	}
	marker := ""
	isTime := strings.ToLower(d.Unit) == "s" || strings.ToLower(d.Unit) == "ms"
	if d.Pct > 50 || (isTime && d.Curr > 15) {
		marker = "*"
	}
	return fmt.Sprintf("  %-24s  %g%s  (%s%.1f%%)%s", d.CheckName, d.Curr, d.Unit, sign, d.Pct, marker)
}

// printKPIDeltaSummary prints the kpi delta block if the JSON payload is non-empty.
func printKPIDeltaSummary(deltaJSON string) {
	if deltaJSON == "" || deltaJSON == "[]" {
		return
	}
	var deltas []kpiDelta
	if err := json.Unmarshal([]byte(deltaJSON), &deltas); err != nil || len(deltas) == 0 {
		return
	}
	fmt.Println("kpi deltas (last 2 samples per check):")
	for _, d := range deltas {
		fmt.Println(formatKPIDeltaLine(d))
	}
}
