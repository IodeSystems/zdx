package ship

import (
	"context"

	"github.com/iodesystems/zdx-go/internal/config"
)

// simpleStrategy runs all declared stages exactly once with no extra
// env injection. This is the default when comp.Ship.Strategy is empty.
type simpleStrategy struct{}

func (simpleStrategy) Run(ctx context.Context, comp config.Component, env map[string]string, opts RunOptions) ([]StageResult, error) {
	main, fin := splitStages(comp.Ship.Stages)
	main = filterStagesByTag(main, opts.IncludeTag, opts.ExcludeTag)
	results, err := runStages(ctx, comp, env, nil, main, nil, "")
	results = append(results, runFinalize(ctx, comp, env, nil, fin, err)...)
	return results, err
}
