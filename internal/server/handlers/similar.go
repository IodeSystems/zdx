package handlers

import (
	"context"

	"github.com/iodesystems/zdx-go/internal/db"
)

// findSimilarIssues embeds queryText and returns the top-n similar open issues.
func (h *Handler) findSimilarIssues(ctx context.Context, projectID int32, queryText string, n int) ([]SimilarIssueItem, error) {
	results, err := h.Emb.TopN(ctx, projectID, queryText, n)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return []SimilarIssueItem{}, nil
	}
	out := make([]SimilarIssueItem, 0, len(results))
	for _, r := range results {
		id := issueIDFromInt(int32(r.ID)) //nolint:gosec
		iss, err := h.Q.GetIssue(ctx, db.GetIssueParams{ProjectID: projectID, ID: id})
		if err != nil {
			continue // stale index entry — skip
		}
		if iss.IssueType == "tracker" {
			continue
		}
		out = append(out, SimilarIssueItem{ID: id, Title: iss.Title, Context: iss.Context, Status: iss.Status, Score: r.Score})
	}
	return out, nil
}

func (h *Handler) findSimilarQuestions(ctx context.Context, projectID int32, queryText string, n int) ([]SimilarQuestionItem, error) {
	results, err := h.Emb.TopNQuestions(ctx, projectID, queryText, n)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return []SimilarQuestionItem{}, nil
	}
	out := make([]SimilarQuestionItem, 0, len(results))
	for _, r := range results {
		q, err := h.Q.GetQuestion(ctx, db.GetQuestionParams{ProjectID: projectID, ID: int32(r.ID)}) //nolint:gosec
		if err != nil {
			continue
		}
		out = append(out, SimilarQuestionItem{
			ID:       q.ID,
			Question: q.Question,
			Answer:   q.Answer.String,
			Score:    r.Score,
		})
	}
	return out, nil
}

func (h *Handler) findSimilarProposals(ctx context.Context, projectID int32, queryText string, n int) ([]SimilarProposalItem, error) {
	seen := make(map[int32]bool)
	out := make([]SimilarProposalItem, 0, n)

	// text search first
	rows, _ := h.Q.SearchProposals(ctx, db.SearchProposalsParams{ProjectID: projectID, Query: queryText})
	for _, r := range rows {
		if seen[r.ID] {
			continue
		}
		seen[r.ID] = true
		out = append(out, SimilarProposalItem{ID: r.ID, Title: r.Title, Body: r.Body, Status: r.Status})
	}

	// vector search
	results, _ := h.Emb.TopNProposals(ctx, projectID, queryText, n)
	for _, r := range results {
		id := int32(r.ID) //nolint:gosec
		if seen[id] {
			continue
		}
		seen[id] = true
		p, err := h.Q.GetProposal(ctx, db.GetProposalParams{ProjectID: projectID, ID: id})
		if err != nil {
			continue
		}
		if p.Status == "rejected" || p.Status == "approved" {
			continue
		}
		out = append(out, SimilarProposalItem{ID: p.ID, Title: p.Title, Body: p.Body, Status: p.Status, Score: r.Score})
	}

	if len(out) > n {
		out = out[:n]
	}
	return out, nil
}

func (h *Handler) findSimilarTasks(ctx context.Context, projectID int32, queryText string, n int) ([]SimilarTaskItem, error) {
	results, err := h.Emb.TopNTasks(ctx, projectID, queryText, n)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return []SimilarTaskItem{}, nil
	}
	out := make([]SimilarTaskItem, 0, len(results))
	for _, r := range results {
		id := taskIDFromInt(int32(r.ID)) //nolint:gosec
		task, err := h.Q.GetTask(ctx, id)
		if err != nil {
			continue
		}
		out = append(out, SimilarTaskItem{ID: id, Text: task.Text, Status: task.Status, Issue: task.Issue, Score: r.Score})
	}
	return out, nil
}
