package cli

import (
	"github.com/iodesystems/zdx-go/internal/cli/clitypes"
	"github.com/iodesystems/zdx-go/internal/dxclient"
)

// CommentsToCli converts the typed client's CommentItem slice into the
// clitypes shape consumed by cli.PrintComments. Only the fields the printer
// reads are populated.
func CommentsToCli(src *[]dxclient.CommentItem) []clitypes.CommentItem {
	if src == nil {
		return nil
	}
	out := make([]clitypes.CommentItem, 0, len(*src))
	for _, c := range *src {
		alias := ""
		if c.AuthorAlias != nil {
			alias = *c.AuthorAlias
		}
		var parentID *int32
		if c.ParentId != nil {
			v := *c.ParentId
			parentID = &v
		}
		var unread *bool
		if c.Unread != nil {
			v := *c.Unread
			unread = &v
		}
		out = append(out, clitypes.CommentItem{
			ID:          c.Id,
			TargetType:  c.TargetType,
			TargetID:    c.TargetId,
			Author:      c.Author,
			AuthorAlias: alias,
			Body:        c.Body,
			CreatedAt:   c.CreatedAt,
			ParentID:    parentID,
			Unread:      unread,
		})
	}
	return out
}

// CommentToCli converts a single dxclient.CommentItem into the clitypes shape.
func CommentToCli(c dxclient.CommentItem) clitypes.CommentItem {
	alias := ""
	if c.AuthorAlias != nil {
		alias = *c.AuthorAlias
	}
	var parentID *int32
	if c.ParentId != nil {
		v := *c.ParentId
		parentID = &v
	}
	var unread *bool
	if c.Unread != nil {
		v := *c.Unread
		unread = &v
	}
	return clitypes.CommentItem{
		ID:          c.Id,
		TargetType:  c.TargetType,
		TargetID:    c.TargetId,
		Author:      c.Author,
		AuthorAlias: alias,
		Body:        c.Body,
		CreatedAt:   c.CreatedAt,
		ParentID:    parentID,
		Unread:      unread,
	}
}

// IssueToCli converts a typed IssueItem into the clitypes shape expected by
// cli.PrintIssueItem.
func IssueToCli(iss dxclient.IssueItem) clitypes.IssueItem {
	var blocked clitypes.StringOrStrings
	if iss.BlockedBy != nil {
		blocked = clitypes.StringOrStrings(*iss.BlockedBy)
	}
	var detail []clitypes.IssueBlockerRef
	if iss.BlockedByDetail != nil {
		detail = make([]clitypes.IssueBlockerRef, 0, len(*iss.BlockedByDetail))
		for _, b := range *iss.BlockedByDetail {
			detail = append(detail, clitypes.IssueBlockerRef{ID: b.Id, Status: b.Status})
		}
	}
	return clitypes.IssueItem{
		ID:              iss.Id,
		Title:           iss.Title,
		Status:          iss.Status,
		Priority:        iss.Priority,
		Component:       iss.Component,
		BlockedBy:       blocked,
		BlockedByDetail: detail,
		Context:         iss.Context,
		IssueType:       iss.IssueType,
		URL:             iss.Url,
	}
}

// FeatureToCli converts a typed FeatureItem into the clitypes shape expected
// by cli.PrintFeatureItem.
func FeatureToCli(f dxclient.FeatureItem) clitypes.FeatureItem {
	var specs []clitypes.SpecItem
	if f.Specs != nil {
		specs = make([]clitypes.SpecItem, 0, len(*f.Specs))
		for _, s := range *f.Specs {
			specs = append(specs, clitypes.SpecItem{
				ID:          s.Id,
				Description: s.Description,
				Kind:        s.Kind,
				ConcernType: s.ConcernType,
				Deferred:    s.Deferred,
			})
		}
	}
	return clitypes.FeatureItem{
		ID:              f.Id,
		Name:            f.Name,
		Description:     f.Description,
		What:            f.What,
		Why:             f.Why,
		DoneWhen:        f.DoneWhen,
		Component:       f.Component,
		Category:        f.Category,
		Kind:            f.Kind,
		GoalID:          f.GoalId,
		ParentFeatureID: f.ParentFeatureId,
		MetricName:      f.MetricName,
		MetricUnit:      f.MetricUnit,
		BaselineValue:   f.BaselineValue,
		TargetValue:     f.TargetValue,
		GraphURL:        f.GraphUrl,
		PlanType:        f.PlanType,
		Specs:           specs,
	}
}

// TaskToCli converts a typed TaskItem into the clitypes shape used by
// printTasks/printTaskItem.
func TaskToCli(t dxclient.TaskItem) clitypes.TaskItem {
	var issueID *int32
	if t.IssueId != nil {
		v := *t.IssueId
		issueID = &v
	}
	stale := ""
	if t.StaleSince != nil {
		stale = *t.StaleSince
	}
	return clitypes.TaskItem{
		ID:         t.Id,
		Title:      t.Title,
		Text:       t.Text,
		Feature:    t.Feature,
		Status:     t.Status,
		Reason:     t.Reason,
		IssueID:    issueID,
		TaskGroup:  t.TaskGroup,
		TestPlan:   t.TestPlan,
		TestRefs:   t.TestRefs,
		CreatedAt:  t.CreatedAt,
		StaleSince: stale,
	}
}
