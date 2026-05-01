package ship

import (
	"context"

	"github.com/iodesystems/zdx-go/internal/config"
)

// maintenanceStrategy is identical to simpleStrategy but injects
// ZDX_MAINTENANCE=1 so stage scripts can drain connections, suppress
// health checks, or emit a maintenance page.
type maintenanceStrategy struct{}

func (maintenanceStrategy) Run(ctx context.Context, comp config.Component, env map[string]string, opts RunOptions) ([]StageResult, error) {
	extra := map[string]string{"ZDX_MAINTENANCE": "1"}
	main, fin := splitStages(comp.Ship.Stages)
	main = filterStagesByTag(main, opts.IncludeTag, opts.ExcludeTag)
	results, err := runStages(ctx, comp, env, extra, main, nil, "")
	results = append(results, runFinalize(ctx, comp, env, extra, fin, err)...)
	return results, err
}
