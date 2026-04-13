package db

import (
	"context"
	"fmt"
)

// NextIssueID allocates and returns the next IS-N identifier for the project.
func (q *Queries) NextIssueID(ctx context.Context, projectID int32) (string, error) {
	n, err := q.NextID(ctx, NextIDParams{ProjectID: projectID, Kind: "issue"})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("IS-%d", n), nil
}

// NextTaskID allocates and returns the next TK-N identifier for the project.
func (q *Queries) NextTaskID(ctx context.Context, projectID int32) (string, error) {
	n, err := q.NextID(ctx, NextIDParams{ProjectID: projectID, Kind: "task"})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("TK-%d", n), nil
}
