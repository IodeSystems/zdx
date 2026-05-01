package agent

import (
	"context"
	"fmt"
	"os"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/dxclient"
)

// mintScopedToken creates a project-scoped API token via the admin endpoint.
// Returns the token string and its numeric ID for later revocation.
// Errors if rc.slug is empty (cannot scope without a project).
func mintScopedToken(ctx context.Context, rc remoteConfig, label string) (string, int32, error) {
	if rc.slug == "" {
		return "", 0, fmt.Errorf("mintScopedToken: rc.slug is empty — cannot mint scoped token without a project")
	}
	c := cli.NewClient(rc.url, rc.key)
	resp, err := c.CreateAdminTokenWithResponse(ctx, dxclient.CreateAdminTokenRequest{
		ProjectSlug: rc.slug,
		Name:        &label,
	})
	if err != nil {
		return "", 0, fmt.Errorf("mint scoped token: %w", err)
	}
	if resp.JSON200 == nil {
		return "", 0, fmt.Errorf("mint scoped token: unexpected status %d", resp.StatusCode())
	}
	item := resp.JSON200
	if item.Token == nil || *item.Token == "" {
		return "", 0, fmt.Errorf("mint scoped token: server returned empty token")
	}
	return *item.Token, item.Id, nil
}

// revokeScopedToken deletes the token with the given ID. Best-effort: errors
// are printed to stderr but do not propagate. Callers should pass a fresh
// context.Background() so a cancelled session context does not prevent cleanup.
func revokeScopedToken(ctx context.Context, rc remoteConfig, id int32) {
	if id == 0 {
		return
	}
	c := cli.NewClient(rc.url, rc.key)
	_, err := c.DeleteAdminTokenWithResponse(ctx, id)
	if err != nil {
		fmt.Fprintf(os.Stderr, "revoke scoped token %d: %v\n", id, err)
	}
}
