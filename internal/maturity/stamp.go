package maturity

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/iodesystems/zdx-go/internal/db"
)

// StampFromAnswer upserts maturity items derived from a single question answer.
// It is idempotent: re-stamping the same answer refreshes titles/descriptions
// but never downgrades existing done/snoozed/dismissed items.
// Returns the stamped items (or the existing rows if nothing changed).
func StampFromAnswer(ctx context.Context, q *db.Queries, projectID int32, questionKey, answer string) ([]db.ZdxMaturityItem, error) {
	byAnswer, ok := Templates[questionKey]
	if !ok {
		return nil, nil
	}
	tmplList, ok := byAnswer[answer]
	if !ok {
		return nil, nil
	}
	return stampTemplates(ctx, q, projectID, questionKey, tmplList)
}

// StampBaseline upserts the four project-level baseline maturity items for a
// project. It is safe to call repeatedly; existing items are preserved.
func StampBaseline(ctx context.Context, q *db.Queries, projectID int32) ([]db.ZdxMaturityItem, error) {
	tmplList, ok := Templates["baseline"]["yes"]
	if !ok {
		return nil, nil
	}
	return stampTemplates(ctx, q, projectID, "baseline", tmplList)
}

// HasAnyAnswers returns true if the project has answered at least one question.
func HasAnyAnswers(ctx context.Context, q *db.Queries, projectID int32) (bool, error) {
	answers, err := q.ListMaturityAnswers(ctx, projectID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return false, err
	}
	return len(answers) > 0, nil
}

func stampTemplates(ctx context.Context, q *db.Queries, projectID int32, sourceQuestion string, tmplList []ItemTemplate) ([]db.ZdxMaturityItem, error) {
	out := make([]db.ZdxMaturityItem, 0, len(tmplList))
	for _, t := range tmplList {
		item, err := q.UpsertMaturityItem(ctx, db.UpsertMaturityItemParams{
			ProjectID:      projectID,
			Kind:           t.Kind,
			TargetType:     t.TargetType,
			Title:          t.Title,
			Description:    t.Description,
			SourceQuestion: sourceQuestion,
			PriorityHint:   t.PriorityHint,
		})
		if err != nil {
			return out, err
		}
		out = append(out, item)
	}
	return out, nil
}
