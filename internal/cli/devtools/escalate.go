// Package devtools — escalation sub-module.
//
// escalate.go implements the --escalate flag for dx test: when preexisting
// test failures are detected (via --classify-preexisting), each failure is
// fingerprinted and searched against existing issues via the embedder's
// similarity endpoint. If no sufficiently similar issue exists, a new
// high-priority blocker issue is filed; otherwise a recurrence comment is
// appended to the existing issue.
package devtools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/dxclient"
	"github.com/iodesystems/zdx-go/internal/testharness"
)

// EscalationConfig holds the parameters for escalating preexisting test failures.
type EscalationConfig struct {
	// AgentIssue is the DX_AGENT_ISSUE env var value (e.g. "IS-1234").
	AgentIssue string
	// SimilarityThreshold is the minimum embedder similarity score to consider
	// an existing issue as a match. Default: 0.85.
	SimilarityThreshold float32
	// MaxResults is the number of similar issues to fetch from the embedder.
	MaxResults int
}

// EscalationResult records what happened for a single preexisting failure.
type EscalationResult struct {
	TestName    string `json:"test_name"`
	Action      string `json:"action"` // "filed" or "recurrence"
	IssueID     string `json:"issue_id,omitempty"`
	Fingerprint string `json:"fingerprint"`
}

// EscalatePreexisting iterates over the preexisting failures in classified
// results and, for each one, searches for a similar existing issue via the
// embedder. If no match is found above the similarity threshold, a new
// priority-1 impl issue is filed. Otherwise a recurrence comment is appended.
//
// Requires a configured CLI client connected to a server with embedder support.
// Returns a list of EscalationResult describing what happened per test.
func EscalatePreexisting(ctx context.Context, c *cli.Client, slug string, classified testharness.ClassifiedResults, cfg EscalationConfig) ([]EscalationResult, error) {
	if len(classified.Preexisting) == 0 {
		return nil, nil
	}
	if cfg.AgentIssue == "" {
		return nil, fmt.Errorf("escalation requires DX_AGENT_ISSUE or --agent-issue flag")
	}

	threshold := cfg.SimilarityThreshold
	if threshold == 0 {
		threshold = 0.85
	}
	maxN := cfg.MaxResults
	if maxN == 0 {
		maxN = 5
	}

	var results []EscalationResult
	sha := gitSHA()

	for _, r := range classified.Preexisting {
		fingerprint := buildFingerprint(r)

		// Search for similar issues.
		similar, err := searchSimilarIssues(ctx, c, slug, fingerprint, maxN)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[escalate] warning: similarity search for %s failed: %v\n", r.Test, err)
			// Continue: file a new issue as fallback.
		}

		// Find the best match above threshold.
		var bestMatch *dxclient.SimilarIssueItem
		for i := range similar {
			if similar[i].Score >= threshold {
				bestMatch = &similar[i]
				break // results are ordered by score descending
			}
		}

		if bestMatch != nil {
			// Append recurrence comment.
			commentBody := fmt.Sprintf("Recurrence seen during agent session %s at SHA %s.\n\nTest: %s\nComponent: %s\nOutput: %s",
				cfg.AgentIssue, sha, r.Test, r.Component, truncate(r.Output, 500))
			if err := addCommentToIssue(ctx, c, slug, bestMatch.Id, commentBody); err != nil {
				fmt.Fprintf(os.Stderr, "[escalate] warning: could not add comment to %s: %v\n", bestMatch.Id, err)
			}
			results = append(results, EscalationResult{
				TestName:    r.Test,
				Action:      "recurrence",
				IssueID:     bestMatch.Id,
				Fingerprint: fingerprint,
			})
		} else {
			// File a new issue.
			issueID, err := fileTestFailureIssue(ctx, c, slug, r, cfg.AgentIssue, sha, fingerprint)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[escalate] warning: could not file issue for %s: %v\n", r.Test, err)
				results = append(results, EscalationResult{
					TestName:    r.Test,
					Action:      "failed",
					Fingerprint: fingerprint,
				})
				continue
			}
			results = append(results, EscalationResult{
				TestName:    r.Test,
				Action:      "filed",
				IssueID:     issueID,
				Fingerprint: fingerprint,
			})
		}
	}

	return results, nil
}

// PrintEscalationSummary prints a human-readable summary of escalation results to stderr.
func PrintEscalationSummary(results []EscalationResult) {
	if len(results) == 0 {
		return
	}
	fmt.Fprintln(os.Stderr, "\nEscalated preexisting failures:")
	for _, r := range results {
		switch r.Action {
		case "filed":
			fmt.Fprintf(os.Stderr, "  %s → filed %s (new)\n", r.TestName, r.IssueID)
		case "recurrence":
			fmt.Fprintf(os.Stderr, "  %s → recurrence noted on %s\n", r.TestName, r.IssueID)
		case "failed":
			fmt.Fprintf(os.Stderr, "  %s → escalation failed\n", r.TestName)
		}
	}
}

// WriteEscalationJSON writes escalation results as JSON to the given path.
func WriteEscalationJSON(path string, results []EscalationResult) error {
	if len(results) == 0 {
		return nil
	}
	b, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0644)
}

// buildFingerprint creates a deterministic fingerprint string for a test failure.
func buildFingerprint(r testharness.Result) string {
	// Extract the first meaningful error line from output.
	errorLine := firstErrorLine(r.Output)
	return fmt.Sprintf("test failure: %s runner=%s error=%s", r.Test, r.Component, errorLine)
}

// firstErrorLine extracts the first non-empty, non-verbose line from test output
// that looks like an error message.
func firstErrorLine(output string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "=== ") || strings.HasPrefix(line, "--- ") {
			continue
		}
		// Skip very long lines (likely stack traces or JSON dumps).
		if len(line) > 200 {
			continue
		}
		return line
	}
	return "(no error output)"
}

// searchSimilarIssues calls the server's /api/dx/issues/similar endpoint.
func searchSimilarIssues(ctx context.Context, c *cli.Client, slug string, text string, n int) ([]dxclient.SimilarIssueItem, error) {
	n64 := int64(n)
	resp, err := c.SimilarIssuesWithResponse(ctx, dxclient.SimilarIssuesJSONRequestBody{
		Slug: slug,
		Text: text,
		N:    &n64,
	})
	if err != nil {
		return nil, err
	}
	if resp.JSON200 == nil || resp.JSON200.Issues == nil {
		return nil, nil
	}
	return *resp.JSON200.Issues, nil
}

// addCommentToIssue posts a comment to an existing issue.
func addCommentToIssue(ctx context.Context, c *cli.Client, slug string, issueID string, body string) error {
	alias := "auto-escalate"
	_, err := c.AddCommentWithResponse(ctx, dxclient.AddCommentJSONRequestBody{
		Slug:        slug,
		TargetType:  "issue",
		TargetId:    issueID,
		AuthorAlias: &alias,
		Body:        body,
	})
	return err
}

// fileTestFailureIssue creates a new priority-1 issue for a test failure.
func fileTestFailureIssue(ctx context.Context, c *cli.Client, slug string, r testharness.Result, agentIssue string, sha string, fingerprint string) (string, error) {
	title := fmt.Sprintf("Test failure: %s", r.Test)
	context := fmt.Sprintf("Preexisting test failure detected during agent session %s.\n\n"+
		"Test: %s\n"+
		"Component: %s\n"+
		"Layer: %s\n"+
		"SHA: %s\n\n"+
		"Fingerprint: %s\n\n"+
		"Output:\n%s",
		agentIssue, r.Test, r.Component, r.Layer, sha, fingerprint, r.Output)

	// Use the CLI Post method to create the issue.
	var resp struct {
		ID string `json:"id"`
	}
	err := c.Post("/api/dx/projects/"+slug+"/issues", map[string]interface{}{
		"title":     title,
		"context":   context,
		"priority":  "1",
		"type":      "impl",
		"component": "test",
		"source":    "auto-escalate",
	}, &resp)
	if err != nil {
		return "", fmt.Errorf("create issue: %w", err)
	}
	if resp.ID == "" {
		// Fallback: try to extract from response or construct.
		return "(issue filed, ID unknown)", nil
	}
	return resp.ID, nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
