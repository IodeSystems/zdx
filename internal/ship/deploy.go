package ship

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/iodesystems/zdx-go/internal/dxclient"
)

// maxLogBytes is the tail-truncation limit for the combined stage log sent to
// the deploy event endpoint. Chosen to fit the TEXT NOT NULL column (IS-666)
// without bloating row size.
const maxLogBytes = 64 * 1024

// deployPoster is the subset of dxclient.ClientWithResponsesInterface that
// PostDeployEvent needs. Satisfied by *cli.Client and any httptest stub.
type deployPoster interface {
	CreateEnvironmentDeployWithResponse(ctx context.Context, slug, name string, body dxclient.CreateEnvironmentDeployJSONRequestBody, reqEditors ...dxclient.RequestEditorFn) (*dxclient.CreateEnvironmentDeployResponse, error)
}

// PostDeployEvent derives status/duration/log from results and POSTs a
// best-effort deploy event to the dx-server. Errors are logged to stderr
// and nil is returned so the caller's runErr is the CLI exit code.
func PostDeployEvent(ctx context.Context, c deployPoster, slug, envName, sha, branch string, results []StageResult) error {
	status := "success"
	var totalDur time.Duration
	var logParts []string

	for _, r := range results {
		totalDur += r.Duration
		logParts = append(logParts, fmt.Sprintf("=== %s [%s] (%s) ===\n%s\n", r.Name, r.Status, r.Duration, r.Log))
		if r.Status != "ok" && r.Status != "skipped" {
			status = "failure"
		}
	}

	combined := strings.Join(logParts, "")
	if len(combined) > maxLogBytes {
		overflow := len(combined) - maxLogBytes
		marker := fmt.Sprintf("[truncated %d bytes]\n", overflow)
		keepFrom := len(combined) - (maxLogBytes - len(marker))
		combined = marker + combined[keepFrom:]
	}

	durSecs := int32(totalDur / time.Second)
	req := dxclient.CreateEnvironmentDeployRequest{
		BuildSha:     sha,
		BuildBranch:  &branch,
		Status:       &status,
		DurationSecs: &durSecs,
		Log:          &combined,
	}

	resp, err := c.CreateEnvironmentDeployWithResponse(ctx, slug, envName, req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ship: deploy event POST failed: %v\n", err)
		return nil
	}
	if resp.StatusCode() >= 400 {
		fmt.Fprintf(os.Stderr, "ship: deploy event POST failed: HTTP %d\n", resp.StatusCode())
		return nil
	}
	return nil
}
