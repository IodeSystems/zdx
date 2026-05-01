package handlers

import (
	"fmt"
	"strings"
)

// yieldBreach captures a single threshold breach detected during standup
// generation. The auto-file loop renders these to issues — the title is
// stable across runs so re-emission updates the existing issue with a fresh
// recurrence comment instead of duplicating it. (IS-684)
type yieldBreach struct {
	Key             string   // stable metric key (e.g. "claude_sessions_per_closed_issue") — for dedup and search
	Label           string   // human-readable name — drives the issue title
	Definition      string   // what the metric measures, including the underlying tables
	Units           string   // metric units (e.g. "sessions / closed issues (30d)")
	Threshold       string   // when the metric is bad and why
	CurrentValue    string   // the value that tripped this breach (e.g. "6.6")
	Diagnosis       string   // prose from concernParts — already user-facing in the journal
	SuggestedAction string   // concrete next step the triager can take
	EntityRefs      []string // optional entity IDs surfaced in the body (e.g. deferred spec IDs, thrashing issue IDs)
	EntityRefsLabel string   // label for the EntityRefs section (e.g. "Deferred specs", "Top thrashing issues")
}

// Title returns the stable auto-file title — same across runs so dedup works.
// Excludes the current value (which goes in the body) so re-emissions don't
// each create a new issue.
func (b yieldBreach) Title() string {
	return "Yield alert: " + b.Label
}

// Body renders the auto-file context block. entryID + entryDate scope the
// alert to the journal entry that produced it.
func (b yieldBreach) Body(entryID int32, entryDate string) string {
	var p []string
	p = append(p, fmt.Sprintf("**Metric:** `%s` — %s", b.Key, b.Definition))
	if b.Units != "" {
		p = append(p, fmt.Sprintf("**Units:** %s", b.Units))
	}
	p = append(p, fmt.Sprintf("**Threshold:** %s", b.Threshold))
	p = append(p, fmt.Sprintf("**Current:** %s", b.CurrentValue))
	if b.Diagnosis != "" {
		p = append(p, fmt.Sprintf("**Why this matters:** %s", b.Diagnosis))
	}
	if b.SuggestedAction != "" {
		p = append(p, fmt.Sprintf("**Suggested action:** %s", b.SuggestedAction))
	}
	if len(b.EntityRefs) > 0 {
		label := b.EntityRefsLabel
		if label == "" {
			label = "Entities"
		}
		p = append(p, fmt.Sprintf("**%s:** %s", label, strings.Join(b.EntityRefs, ", ")))
	}
	p = append(p, fmt.Sprintf("Auto-filed from journal entry %d on %s. Subsequent breaches of the same metric append a recurrence comment.", entryID, entryDate))
	return strings.Join(p, "\n\n")
}

// RecurrenceComment is appended to an existing alert issue when the metric
// breaches again — keeps the history visible without spawning duplicates.
func (b yieldBreach) RecurrenceComment(entryID int32, entryDate string) string {
	return fmt.Sprintf("Recurrence on %s (journal entry %d): `%s` = %s", entryDate, entryID, b.Key, b.CurrentValue)
}
