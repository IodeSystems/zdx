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

func (h *Handler) findSimilarProposals(ctx context.Context, projectID int32, queryText string, n int, excludeID int32) ([]SimilarProposalItem, error) {
	seen := make(map[int32]bool)
	if excludeID != 0 {
		seen[excludeID] = true
	}
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

func (h *Handler) findSimilarFeatures(ctx context.Context, projectID int32, queryText string, n int) ([]SimilarFeatureItem, error) {
	results, err := h.Emb.TopNFeatures(ctx, projectID, queryText, n)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return []SimilarFeatureItem{}, nil
	}
	out := make([]SimilarFeatureItem, 0, len(results))
	for _, r := range results {
		row, err := h.Q.GetFeatureByID(ctx, int32(r.ID)) //nolint:gosec
		if err != nil || row.ProjectID != projectID {
			continue
		}
		out = append(out, SimilarFeatureItem{
			ID:          row.ID,
			Name:        row.Name,
			Description: row.Description,
			Category:    row.Category,
			Kind:        row.Kind,
			Score:       r.Score,
		})
	}
	return out, nil
}

func (h *Handler) findSimilarSpecs(ctx context.Context, projectID int32, queryText string, n int) ([]SimilarSpecItem, error) {
	results, err := h.Emb.TopNSpecs(ctx, projectID, queryText, n)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return []SimilarSpecItem{}, nil
	}
	out := make([]SimilarSpecItem, 0, len(results))
	for _, r := range results {
		spec, err := h.Q.GetSpec(ctx, int32(r.ID)) //nolint:gosec
		if err != nil {
			continue
		}
		feat, err := h.Q.GetFeatureByID(ctx, spec.FeatureID)
		if err != nil || feat.ProjectID != projectID {
			continue
		}
		out = append(out, SimilarSpecItem{
			ID:          spec.ID,
			FeatureID:   spec.FeatureID,
			FeatureName: feat.Name,
			Description: spec.Description,
			Importance:  spec.Importance,
			Score:       r.Score,
		})
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
		out = append(out, SimilarTaskItem{ID: id, Title: task.Title, Text: task.Text, Status: task.Status, Reason: task.Reason, Issue: task.Issue, Score: r.Score})
	}
	return out, nil
}
