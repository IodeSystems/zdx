package db

import (
	"context"
	"fmt"
)

// NextIssueID allocates and returns the next globally-unique IS-N identifier.
func (q *Queries) NextIssueID(ctx context.Context) (string, error) {
	n, err := q.NextID(ctx, "issue")
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("IS-%d", n), nil
}

// NextTaskID allocates and returns the next globally-unique TK-N identifier.
func (q *Queries) NextTaskID(ctx context.Context) (string, error) {
	n, err := q.NextID(ctx, "task")
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("TK-%d", n), nil
}
