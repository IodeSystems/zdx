package cli

import (
	"fmt"
	"net/url"
	"os"
)

// bootstrapOnboardingIssue creates the initial project onboarding issue with
// blocker questions for goals, architecture, constraints, and workflow.
// Skips if the project already has issues.
func bootstrapOnboardingIssue(c *Client) error {
	slug := c.Slug()
	if slug == "" {
		return nil
	}

	// Check if project already has issues.
	var listing struct {
		Items []struct{ ID int32 } `json:"items"`
		Total int                  `json:"total"`
	}
	if err := c.Get("/api/dx/todo/issue/list", url.Values{
		"slug":  {slug},
		"limit": {"1"},
	}, &listing); err != nil {
		// Project may not exist yet or endpoint may fail — skip silently.
		return nil
	}
	if listing.Total > 0 {
		fmt.Println("bootstrap: project already has issues, skipping onboarding setup")
		return nil
	}

	// Create onboarding issue.
	var resp struct {
		ID    int32  `json:"id"`
		Title string `json:"title"`
	}
	if err := c.Post("/api/dx/todo/issue/add", map[string]any{
		"slug":       slug,
		"title":      "Project onboarding: define goals, architecture, and constraints",
		"context":    "Bootstrap issue for new project setup. Answer the attached questions to establish project direction so that automated workflows can make informed decisions.",
		"issue_type": "ops",
		"auto_ready": true,
	}, &resp); err != nil {
		return fmt.Errorf("create onboarding issue: %w", err)
	}

	issueID := issueIDStr(resp.ID)
	fmt.Printf("created %s  %s\n", issueID, resp.Title)

	questions := []string{
		"What are the primary goals and success criteria for this project? Describe what the project should accomplish and how you'll measure success.",
		"What is the tech stack and high-level architecture? List languages, frameworks, databases, infrastructure, and any key architectural patterns (monolith, microservices, serverless, etc.).",
		"What are the key constraints? Consider: compliance/regulatory requirements, performance targets, backwards compatibility, supported platforms, budget, team size, or timeline.",
		"What development workflow does the team follow? Consider: branching strategy, code review process, CI/CD pipeline, release cadence, testing requirements.",
	}

	for _, q := range questions {
		var bq struct {
			ID int32 `json:"id"`
		}
		body := map[string]any{
			"slug":        slug,
			"target_type": "issue",
			"target_id":   issueID,
			"context":     q,
		}
		if err := c.Post("/api/dx/blocker-questions/add", body, &bq); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not create question: %v\n", err)
			continue
		}
		fmt.Printf("  BQ-%d  %s\n", bq.ID, truncate(q, 70))
	}

	fmt.Println("\nanswer these questions to get started:")
	fmt.Printf("  dx question list\n")
	fmt.Printf("  dx question answer <ID> --answer=\"...\"\n")
	return nil
}
