package ship

import (
	"context"

	"github.com/iodesystems/zdx-go/internal/config"
)

// simpleStrategy runs all declared stages exactly once with no extra
// env injection. This is the default when comp.Ship.Strategy is empty.
type simpleStrategy struct{}

func (simpleStrategy) Run(ctx context.Context, comp config.Component, env map[string]string) ([]StageResult, error) {
	return runStages(ctx, comp, env, nil, comp.Ship.Stages)
}
