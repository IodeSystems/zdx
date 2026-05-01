package handlers

import (
	"context"

	"github.com/iodesystems/zdx-go/internal/db"
)

// Hybrid text+vector pattern: text search first (errors ignored, embedder-offline
// tolerant), then vector-augment (errors ignored). Dedup by ID; matched_via
// tracks the source — "text", "vector", or "both" when an ID surfaces in both.

// findSimilarIssues runs text search + vector and returns up to n similar issues.
// Tracker-type issues are excluded from both paths.
func (h *Handler) findSimilarIssues(ctx context.Context, projectID int32, queryText string, n int) ([]SimilarIssueItem, error) {
	seen := map[string]int{} // id -> index in out
	out := make([]SimilarIssueItem, 0, n)

	rows, _ := h.Q.SearchIssues(ctx, db.SearchIssuesParams{ProjectID: projectID, Query: queryText})
	for _, r := range rows {
		if r.IssueType == "tracker" {
			continue
		}
		if _, exists := seen[r.ID]; exists {
			continue
		}
		seen[r.ID] = len(out)
		out = append(out, SimilarIssueItem{ID: r.ID, Title: r.Title, Context: r.Context, Status: r.Status, MatchedVia: "text"})
	}

	results, _ := h.Emb.TopN(ctx, projectID, queryText, n)
	for _, r := range results {
		id := issueIDFromInt(int32(r.ID)) //nolint:gosec
		if idx, exists := seen[id]; exists {
			out[idx].MatchedVia = "both"
			out[idx].Score = r.Score
			continue
		}
		iss, err := h.Q.GetIssue(ctx, db.GetIssueParams{ProjectID: projectID, ID: id})
		if err != nil {
			continue // stale index entry — skip
		}
		if iss.IssueType == "tracker" {
			continue
		}
		seen[id] = len(out)
		out = append(out, SimilarIssueItem{ID: id, Title: iss.Title, Context: iss.Context, Status: iss.Status, Score: r.Score, MatchedVia: "vector"})
	}

	if len(out) > n {
		out = out[:n]
	}
	return out, nil
}

func (h *Handler) findSimilarQuestions(ctx context.Context, projectID int32, queryText string, n int) ([]SimilarQuestionItem, error) {
	seen := map[int32]int{}
	out := make([]SimilarQuestionItem, 0, n)

	rows, _ := h.Q.SearchQuestions(ctx, db.SearchQuestionsParams{ProjectID: projectID, Query: queryText})
	for _, r := range rows {
		if _, exists := seen[r.ID]; exists {
			continue
		}
		seen[r.ID] = len(out)
		out = append(out, SimilarQuestionItem{
			ID:         r.ID,
			Question:   r.Question,
			Answer:     r.Answer.String,
			MatchedVia: "text",
		})
	}

	results, _ := h.Emb.TopNQuestions(ctx, projectID, queryText, n)
	for _, r := range results {
		id := int32(r.ID) //nolint:gosec
		if idx, exists := seen[id]; exists {
			out[idx].MatchedVia = "both"
			out[idx].Score = r.Score
			continue
		}
		q, err := h.Q.GetQuestion(ctx, db.GetQuestionParams{ProjectID: projectID, ID: id})
		if err != nil {
			continue
		}
		seen[id] = len(out)
		out = append(out, SimilarQuestionItem{
			ID:         q.ID,
			Question:   q.Question,
			Answer:     q.Answer.String,
			Score:      r.Score,
			MatchedVia: "vector",
		})
	}

	if len(out) > n {
		out = out[:n]
	}
	return out, nil
}

func (h *Handler) findSimilarProposals(ctx context.Context, projectID int32, queryText string, n int, excludeID int32) ([]SimilarProposalItem, error) {
	seen := map[int32]int{} // id -> index in out (-1 means excluded)
	if excludeID != 0 {
		seen[excludeID] = -1
	}
	out := make([]SimilarProposalItem, 0, n)

	rows, _ := h.Q.SearchProposals(ctx, db.SearchProposalsParams{ProjectID: projectID, Query: queryText})
	for _, r := range rows {
		if _, exists := seen[r.ID]; exists {
			continue
		}
		seen[r.ID] = len(out)
		out = append(out, SimilarProposalItem{ID: r.ID, Title: r.Title, Body: r.Body, Status: r.Status, MatchedVia: "text"})
	}

	results, _ := h.Emb.TopNProposals(ctx, projectID, queryText, n)
	for _, r := range results {
		id := int32(r.ID) //nolint:gosec
		if idx, exists := seen[id]; exists {
			if idx >= 0 {
				out[idx].MatchedVia = "both"
				out[idx].Score = r.Score
			}
			continue
		}
		p, err := h.Q.GetProposal(ctx, db.GetProposalParams{ProjectID: projectID, ID: id})
		if err != nil {
			continue
		}
		if p.Status == "rejected" || p.Status == "approved" {
			continue
		}
		seen[id] = len(out)
		out = append(out, SimilarProposalItem{ID: p.ID, Title: p.Title, Body: p.Body, Status: p.Status, Score: r.Score, MatchedVia: "vector"})
	}

	if len(out) > n {
		out = out[:n]
	}
	return out, nil
}

func (h *Handler) findSimilarFeatures(ctx context.Context, projectID int32, queryText string, n int) ([]SimilarFeatureItem, error) {
	seen := map[int32]int{}
	out := make([]SimilarFeatureItem, 0, n)

	rows, _ := h.Q.SearchFeatures(ctx, db.SearchFeaturesParams{ProjectID: projectID, Query: queryText})
	for _, r := range rows {
		if _, exists := seen[r.ID]; exists {
			continue
		}
		seen[r.ID] = len(out)
		out = append(out, SimilarFeatureItem{
			ID:          r.ID,
			Name:        r.Name,
			Description: r.Description,
			Category:    r.Category,
			Kind:        r.Kind,
			MatchedVia:  "text",
		})
	}

	results, _ := h.Emb.TopNFeatures(ctx, projectID, queryText, n)
	for _, r := range results {
		id := int32(r.ID) //nolint:gosec
		if idx, exists := seen[id]; exists {
			out[idx].MatchedVia = "both"
			out[idx].Score = r.Score
			continue
		}
		row, err := h.Q.GetFeatureByID(ctx, id)
		if err != nil || row.ProjectID != projectID {
			continue
		}
		seen[id] = len(out)
		out = append(out, SimilarFeatureItem{
			ID:          row.ID,
			Name:        row.Name,
			Description: row.Description,
			Category:    row.Category,
			Kind:        row.Kind,
			Score:       r.Score,
			MatchedVia:  "vector",
		})
	}

	if len(out) > n {
		out = out[:n]
	}
	return out, nil
}

func (h *Handler) findSimilarSpecs(ctx context.Context, projectID int32, queryText string, n int) ([]SimilarSpecItem, error) {
	seen := map[int32]int{}
	out := make([]SimilarSpecItem, 0, n)

	rows, _ := h.Q.SearchSpecs(ctx, db.SearchSpecsParams{ProjectID: projectID, Query: queryText})
	for _, r := range rows {
		if _, exists := seen[r.ID]; exists {
			continue
		}
		seen[r.ID] = len(out)
		out = append(out, SimilarSpecItem{
			ID:          r.ID,
			FeatureID:   r.FeatureID,
			FeatureName: r.FeatureName,
			Description: r.Description,
			Importance:  r.Importance,
			MatchedVia:  "text",
		})
	}

	results, _ := h.Emb.TopNSpecs(ctx, projectID, queryText, n)
	for _, r := range results {
		id := int32(r.ID) //nolint:gosec
		if idx, exists := seen[id]; exists {
			out[idx].MatchedVia = "both"
			out[idx].Score = r.Score
			continue
		}
		spec, err := h.Q.GetSpec(ctx, id)
		if err != nil {
			continue
		}
		feat, err := h.Q.GetFeatureByID(ctx, spec.FeatureID)
		if err != nil || feat.ProjectID != projectID {
			continue
		}
		seen[id] = len(out)
		out = append(out, SimilarSpecItem{
			ID:          spec.ID,
			FeatureID:   spec.FeatureID,
			FeatureName: feat.Name,
			Description: spec.Description,
			Importance:  spec.Importance,
			Score:       r.Score,
			MatchedVia:  "vector",
		})
	}

	if len(out) > n {
		out = out[:n]
	}
	return out, nil
}

func (h *Handler) findSimilarTasks(ctx context.Context, projectID int32, queryText string, n int) ([]SimilarTaskItem, error) {
	seen := map[string]int{}
	out := make([]SimilarTaskItem, 0, n)

	rows, _ := h.Q.SearchTasks(ctx, db.SearchTasksParams{ProjectID: projectID, Query: queryText})
	for _, r := range rows {
		if _, exists := seen[r.ID]; exists {
			continue
		}
		seen[r.ID] = len(out)
		out = append(out, SimilarTaskItem{
			ID:         r.ID,
			Title:      r.Title,
			Text:       r.Text,
			Status:     r.Status,
			Reason:     r.Reason,
			Issue:      r.Issue,
			MatchedVia: "text",
		})
	}

	results, _ := h.Emb.TopNTasks(ctx, projectID, queryText, n)
	for _, r := range results {
		id := taskIDFromInt(int32(r.ID)) //nolint:gosec
		if idx, exists := seen[id]; exists {
			out[idx].MatchedVia = "both"
			out[idx].Score = r.Score
			continue
		}
		task, err := h.Q.GetTask(ctx, id)
		if err != nil {
			continue
		}
		seen[id] = len(out)
		out = append(out, SimilarTaskItem{
			ID:         id,
			Title:      task.Title,
			Text:       task.Text,
			Status:     task.Status,
			Reason:     task.Reason,
			Issue:      task.Issue,
			Score:      r.Score,
			MatchedVia: "vector",
		})
	}

	if len(out) > n {
		out = out[:n]
	}
	return out, nil
}
