package cli

import (
	"context"
	"fmt"
	"strconv"

	"github.com/iodesystems/zdx-go/internal/dxclient"
)

// ResolveTestID accepts either a numeric DB id or a test name (e.g. "TestFoo")
// and returns the corresponding test row id. Name resolution walks the
// ListTests endpoint using search+pagination and requires an exact Name match.
func ResolveTestID(ctx context.Context, c *Client, arg string) (int32, error) {
	if n, err := strconv.ParseInt(arg, 10, 32); err == nil {
		return int32(n), nil
	}
	slug := c.SlugOrDie()
	limit := int32(200)
	var offset int32
	var matches []dxclient.TestItem
	for {
		resp, err := c.ListTestsWithResponse(ctx, &dxclient.ListTestsParams{
			Slug:   slug,
			Limit:  &limit,
			Offset: &offset,
		})
		if err != nil {
			return 0, err
		}
		if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
			return 0, err
		}
		if resp.JSON200 == nil || resp.JSON200.Tests == nil {
			break
		}
		page := *resp.JSON200.Tests
		for _, t := range page {
			if t.Name == arg {
				matches = append(matches, t)
			}
		}
		offset += int32(len(page))
		if int64(offset) >= resp.JSON200.Total || len(page) == 0 {
			break
		}
	}
	switch len(matches) {
	case 0:
		return 0, fmt.Errorf("no test matches %q (tried exact name lookup)", arg)
	case 1:
		return matches[0].Id, nil
	default:
		// Multiple rows for the same name (e.g. component drift between runs).
		// Pick the most-recently-run row; only error if we still can't decide.
		best := matches[0]
		for _, m := range matches[1:] {
			if m.LastRunAt != nil && (best.LastRunAt == nil || *m.LastRunAt > *best.LastRunAt) {
				best = m
			}
		}
		return best.Id, nil
	}
}
