package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// missingColumn names a column the sqlc error claims does not exist.
// Table is "" when the error did not include "of relation X".
type missingColumn struct {
	Column string
	Table  string
}

var (
	// `column "X" of relation "Y" does not exist`
	colWithTableRe = regexp.MustCompile(`column "([^"]+)" of relation "([^"]+)" does not exist`)
	// `column "X" does not exist` (without relation context)
	colOnlyRe = regexp.MustCompile(`column "([^"]+)" does not exist`)
)

// parseMissingColumns extracts the unique (column, table) pairs from sqlc stderr.
// It prefers the table-qualified form when both forms refer to the same column.
func parseMissingColumns(stderr string) []missingColumn {
	seen := map[missingColumn]struct{}{}
	withTable := map[string]struct{}{}

	for _, m := range colWithTableRe.FindAllStringSubmatch(stderr, -1) {
		mc := missingColumn{Column: m[1], Table: m[2]}
		if _, ok := seen[mc]; !ok {
			seen[mc] = struct{}{}
			withTable[mc.Column] = struct{}{}
		}
	}
	for _, m := range colOnlyRe.FindAllStringSubmatch(stderr, -1) {
		col := m[1]
		if _, ok := withTable[col]; ok {
			continue // already captured with a relation
		}
		mc := missingColumn{Column: col}
		if _, ok := seen[mc]; !ok {
			seen[mc] = struct{}{}
		}
	}

	out := make([]missingColumn, 0, len(seen))
	for mc := range seen {
		out = append(out, mc)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Column != out[j].Column {
			return out[i].Column < out[j].Column
		}
		return out[i].Table < out[j].Table
	})
	return out
}

// findAddingMigration scans up.sql files under migDir for the migration that adds
// the given column. When table is non-empty, only ALTER TABLE statements on that
// table are considered. Returns the highest-numbered match (basename), or "" if
// no migration matches.
func findAddingMigration(migDir, column, table string) string {
	entries, err := os.ReadDir(migDir)
	if err != nil {
		return ""
	}
	colPat := regexp.QuoteMeta(column)
	// ALTER TABLE [IF EXISTS] [schema.]"?<table>"? ... ADD COLUMN [IF NOT EXISTS] "?<col>"?
	// We split into two regexes for clarity.
	addRe := regexp.MustCompile(`(?is)ADD\s+COLUMN(?:\s+IF\s+NOT\s+EXISTS)?\s+"?` + colPat + `"?\b`)
	var tableRe *regexp.Regexp
	if table != "" {
		tablePat := regexp.QuoteMeta(table)
		tableRe = regexp.MustCompile(`(?is)ALTER\s+TABLE(?:\s+IF\s+EXISTS)?\s+(?:[\w]+\.)?"?` + tablePat + `"?\b`)
	}

	type hit struct {
		num  int
		name string
	}
	var hits []hit
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".up.sql") {
			continue
		}
		path := filepath.Join(migDir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		text := string(data)
		if !addRe.MatchString(text) {
			continue
		}
		if tableRe != nil {
			// Require both the ALTER TABLE <table> and the ADD COLUMN <col> to
			// appear within the same statement (between semicolons), to avoid
			// matching unrelated statements that happen to share a file.
			if !statementMatchesBoth(text, tableRe, addRe) {
				continue
			}
		}
		num := migrationNumber(name)
		hits = append(hits, hit{num: num, name: name})
	}
	if len(hits) == 0 {
		return ""
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].num > hits[j].num })
	return hits[0].name
}

// statementMatchesBoth returns true when at least one ;-delimited SQL statement
// in text matches both regexes.
func statementMatchesBoth(text string, a, b *regexp.Regexp) bool {
	for stmt := range strings.SplitSeq(text, ";") {
		if a.MatchString(stmt) && b.MatchString(stmt) {
			return true
		}
	}
	return false
}

// migrationNumber returns the numeric prefix of a migration filename
// (e.g. "153_issue_completed_sha.up.sql" → 153). Returns -1 on parse failure.
func migrationNumber(name string) int {
	idx := strings.IndexByte(name, '_')
	if idx <= 0 {
		return -1
	}
	n := 0
	for i := range idx {
		c := name[i]
		if c < '0' || c > '9' {
			return -1
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// formatHint renders the hint block printed after a sqlc failure.
// migDir is the directory passed to findAddingMigration (used only for the
// fallback message). regenCmd is the command operators should run.
func formatHint(migDir string, cols []missingColumn, lookup func(col, table string) string) string {
	if len(cols) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n[db gen] hint: sqlc could not find the following column(s) in schema/shipped.sql:\n")
	for _, c := range cols {
		mig := lookup(c.Column, c.Table)
		qual := c.Column
		if c.Table != "" {
			qual = c.Table + "." + c.Column
		}
		if mig != "" {
			fmt.Fprintf(&b, "  - %s — added by %s\n",
				qual, filepath.Join(migDir, mig))
		} else {
			fmt.Fprintf(&b, "  - %s — no ADD COLUMN found in %s (check shipped.sql is current)\n",
				qual, migDir)
		}
	}
	b.WriteString("[db gen] schema/shipped.sql appears stale relative to the migrations.\n")
	b.WriteString("[db gen] Regenerate locally with: ./bin/regen-schema (requires Docker).\n")
	b.WriteString("[db gen] Worker contract: do NOT commit schema/shipped.sql — the merge-train owns it.\n")
	return b.String()
}
