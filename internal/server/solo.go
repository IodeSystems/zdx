package server

import (
	"context"

	"github.com/iodesystems/dx/internal/apitypes"
	"github.com/iodesystems/dx/internal/db"
)

// computeSolo derives the next actionable item for a project (optionally filtered
// by issue). Priority order:
//  1. Untriaged issues (status = 'open', priority = '')
//  2. Issues needing tech plan (status = 'open', no linked feature with tasks)
//  3. Pending tasks (oldest first, optionally filtered by issue)
//  4. nil → nothing to do

func computeSolo(ctx context.Context, q *db.Queries, projectID int32, issueFilter string) (*apitypes.SoloItem, error) {
	issues, err := q.ListIssues(ctx, projectID)
	if err != nil {
		return nil, err
	}

	// Pass 1: owner triage — open issue with no priority
	for _, iss := range issues {
		if iss.Status != "open" {
			continue
		}
		if issueFilter != "" && iss.ID != issueFilter {
			continue
		}
		if iss.Priority == "" {
			return &apitypes.SoloItem{
				ID:      iss.ID,
				Kind:    "triage",
				Summary: iss.Title,
				Issue:   iss.ID,
			}, nil
		}
	}

	tasks, err := q.ListTasks(ctx, projectID)
	if err != nil {
		return nil, err
	}

	// Build set of issues that have at least one task
	issueHasTasks := map[string]bool{}
	for _, t := range tasks {
		if t.Issue != "" {
			issueHasTasks[t.Issue] = true
		}
	}

	// Pass 2: tech plan — open issue with priority but no tasks
	for _, iss := range issues {
		if iss.Status != "open" || iss.Priority == "" {
			continue
		}
		if issueFilter != "" && iss.ID != issueFilter {
			continue
		}
		if !issueHasTasks[iss.ID] {
			return &apitypes.SoloItem{
				ID:      iss.ID,
				Kind:    "plan",
				Summary: "Tech plan needed: " + iss.Title,
				Issue:   iss.ID,
			}, nil
		}
	}

	// Pass 3: oldest pending task
	for _, t := range tasks {
		if t.Status != "pending" {
			continue
		}
		if issueFilter != "" && t.Issue != issueFilter {
			continue
		}
		return &apitypes.SoloItem{
			ID:      t.ID,
			Kind:    "dev",
			Summary: t.Text,
			Issue:   t.Issue,
			Task:    t.ID,
			Feature: t.Feature,
		}, nil
	}

	return nil, nil
}
