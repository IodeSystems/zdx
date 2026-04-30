package cli

import (
	"fmt"
	"strings"

	"github.com/iodesystems/zdx-go/internal/cli/clitypes"
)

func PrintIssueItem(iss clitypes.IssueItem) {
	fmt.Printf("ID:        %s\n", clitypes.IssueIDStr(iss.ID))
	fmt.Printf("Title:     %s\n", iss.Title)
	fmt.Printf("Status:    %s\n", iss.Status)
	fmt.Printf("Priority:  %s\n", iss.Priority)
	issType := iss.IssueType
	if issType == "" {
		issType = "unknown"
	}
	fmt.Printf("Type:      %s\n", issType)
	if iss.Component != "" {
		fmt.Printf("Component: %s\n", iss.Component)
	}
	if iss.URL != "" {
		fmt.Printf("URL:       %s\n", iss.URL)
	}
	if iss.DuplicateOf != "" {
		fmt.Printf("Duplicate of: %s\n", iss.DuplicateOf)
	}
	if iss.LinkOf != "" {
		fmt.Printf("Link of:   %s  (cascade-close, no reopen-cascade)\n", iss.LinkOf)
	}
	if iss.ReopenCount > 0 {
		fmt.Printf("Reopens:   %d  (churn signal)\n", iss.ReopenCount)
	}
	if iss.InteractiveOnly {
		fmt.Printf("Interactive only: yes  (hidden from autonomous agent loop)\n")
	}
	if len(iss.BlockedByDetail) > 0 {
		var openBlock, closedBlock, openChild, closedChild []string
		for _, b := range iss.BlockedByDetail {
			isComposition := b.Kind == "composition"
			isClosed := b.Status == "closed"
			switch {
			case isComposition && isClosed:
				closedChild = append(closedChild, b.ID)
			case isComposition:
				openChild = append(openChild, b.ID)
			case isClosed:
				closedBlock = append(closedBlock, b.ID)
			default:
				openBlock = append(openBlock, b.ID)
			}
		}
		if len(openChild) > 0 {
			fmt.Printf("Children:  %s\n", strings.Join(openChild, ", "))
		}
		if len(closedChild) > 0 {
			fmt.Printf("Children (closed): %s\n", strings.Join(closedChild, ", "))
		}
		if len(openBlock) > 0 {
			fmt.Printf("Blocked by: %s\n", strings.Join(openBlock, ", "))
		}
		if len(closedBlock) > 0 {
			fmt.Printf("Depends on: %s\n", strings.Join(closedBlock, ", "))
		}
	} else if len(iss.BlockedBy) > 0 {
		fmt.Printf("Blocked:   %s\n", strings.Join(iss.BlockedBy, ", "))
	}
	if iss.Context != "" {
		fmt.Printf("\n%s\n", iss.Context)
	}
}

func PrintComments(comments []clitypes.CommentItem) {
	for _, cm := range comments {
		date := cm.CreatedAt
		if len(date) >= 10 {
			date = date[:10]
		}
		dot := ""
		if cm.Unread != nil {
			if *cm.Unread {
				dot = "○ "
			} else {
				dot = "● "
			}
		}
		indent := ""
		if cm.ParentID != nil {
			indent = "  ↳ "
		}
		author := cm.Author
		if cm.AuthorAlias != "" {
			author = cm.AuthorAlias + " (" + cm.Author + ")"
		}
		fmt.Printf("%s%sC-%d [%s] %s: %s\n", indent, dot, cm.ID, date, author, cm.Body)
	}
}

func PrintFeatureItem(f clitypes.FeatureItem) {
	fmt.Printf("Name:      %s\n", f.Name)
	if f.Kind != "" && f.Kind != "direct" {
		fmt.Printf("Kind:      %s\n", f.Kind)
	}
	if f.Category != "" {
		fmt.Printf("Category:  %s\n", f.Category)
	}
	if f.Component != "" {
		fmt.Printf("Component: %s\n", f.Component)
	}
	if f.GoalID > 0 {
		fmt.Printf("Goal:      G-%d\n", f.GoalID)
	}
	if f.ParentFeatureID > 0 {
		fmt.Printf("Parent:    F-%d\n", f.ParentFeatureID)
	}
	if f.Description != "" {
		fmt.Printf("Desc:      %s\n", f.Description)
	}
	if f.What != "" {
		fmt.Printf("What:      %s\n", f.What)
	}
	if f.Why != "" {
		fmt.Printf("Why:       %s\n", f.Why)
	}
	if f.DoneWhen != "" {
		fmt.Printf("Done when: %s\n", f.DoneWhen)
	}
	if f.MetricName != "" {
		fmt.Printf("Metric:    %s (%s)\n", f.MetricName, f.MetricUnit)
		if f.BaselineValue != "" {
			fmt.Printf("Baseline:  %s\n", f.BaselineValue)
		}
		if f.TargetValue != "" {
			fmt.Printf("Target:    %s\n", f.TargetValue)
		}
		if f.GraphURL != "" {
			fmt.Printf("Graph:     %s\n", f.GraphURL)
		}
	}
	if len(f.Specs) > 0 {
		fmt.Printf("\nSpecs (%d):\n", len(f.Specs))
		tiers := []struct {
			name  string
			label string
		}{
			{"must", "MUST (deal-breaker)"},
			{"should", "SHOULD (friction)"},
			{"nice-to-have", "NICE-TO-HAVE (polish)"},
		}
		for _, tier := range tiers {
			var matched []clitypes.SpecItem
			for _, s := range f.Specs {
				if s.Importance == tier.name {
					matched = append(matched, s)
				}
			}
			if len(matched) == 0 {
				continue
			}
			fmt.Printf("  %s\n", tier.label)
			for _, s := range matched {
				fmt.Printf("    %-4d  %s\n", s.ID, s.Description)
			}
		}
	}
}
