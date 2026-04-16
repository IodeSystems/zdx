package devtools

import (
	"github.com/iodesystems/zdx-go/internal/techmetrics"
)

type TechMetrics = techmetrics.TechMetrics
type MetricDelta = techmetrics.MetricDelta

var (
	collectTechMetrics   = techmetrics.Collect
	collectGitChurn      = techmetrics.CollectGitChurn
	computeDeltas        = techmetrics.ComputeDeltas
	metricsToJSON        = techmetrics.ToJSON
	deltasToJSON         = techmetrics.DeltasToJSON
	parseTechMetrics     = techmetrics.Parse
	formatMetricsSummary = techmetrics.FormatSummary
)
