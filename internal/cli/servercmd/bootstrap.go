package servercmd

import (
	"context"
	"fmt"
	"os"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/cli/clitypes"
	"github.com/iodesystems/zdx-go/internal/dxclient"
)

// bootstrapOnboardingIssue creates the initial project onboarding issue with
// blocker questions for goals, architecture, constraints, and workflow.
// Skips if the project already has issues.
func bootstrapOnboardingIssue(c *cli.Client) error {
	slug := c.Slug()
	if slug == "" {
		return nil
	}
	ctx := context.Background()

	// Check if project already has issues.
	var limit int32 = 1
	listResp, err := c.ListIssuesWithResponse(ctx, &dxclient.ListIssuesParams{
		Slug:  slug,
		Limit: &limit,
	})
	if err != nil {
		// Project may not exist yet or endpoint may fail — skip silently.
		return nil
	}
	if err := c.CheckStatus(listResp.StatusCode(), listResp.Body); err != nil {
		return nil
	}
	if listResp.JSON200 != nil && listResp.JSON200.Total > 0 {
		fmt.Println("bootstrap: project already has issues, skipping onboarding setup")
		return nil
	}

	// Create onboarding issue.
	title := "Project onboarding: define goals, architecture, and constraints"
	issueCtx := "Bootstrap issue for new project setup. Answer the attached questions to establish project direction so that automated workflows can make informed decisions."
	issueType := "ops"
	autoReady := true
	addResp, err := c.AddIssueWithResponse(ctx, dxclient.AddIssueRequest{
		Slug:      slug,
		Title:     &title,
		Context:   &issueCtx,
		IssueType: &issueType,
		AutoReady: &autoReady,
	})
	if err != nil {
		return fmt.Errorf("create onboarding issue: %w", err)
	}
	if err := c.CheckStatus(addResp.StatusCode(), addResp.Body); err != nil {
		return fmt.Errorf("create onboarding issue: %w", err)
	}
	if addResp.JSON200 == nil {
		return fmt.Errorf("create onboarding issue: empty response")
	}

	issueID := clitypes.IssueIDStr(addResp.JSON200.Id)
	fmt.Printf("created %s  %s\n", issueID, addResp.JSON200.Title)

	questions := []string{
		"What are the primary goals and success criteria for this project? Describe what the project should accomplish and how you'll measure success.",
		"What is the tech stack and high-level architecture? List languages, frameworks, databases, infrastructure, and any key architectural patterns (monolith, microservices, serverless, etc.).",
		"What are the key constraints? Consider: compliance/regulatory requirements, performance targets, backwards compatibility, supported platforms, budget, team size, or timeline.",
		"What development workflow does the team follow? Consider: branching strategy, code review process, CI/CD pipeline, release cadence, testing requirements.",
	}

	for _, q := range questions {
		bqResp, err := c.AddBlockerQuestionWithResponse(ctx, dxclient.AddBlockerQuestionRequest{
			Slug:       slug,
			TargetType: "issue",
			TargetId:   issueID,
			Context:    q,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not create question: %v\n", err)
			continue
		}
		if err := c.CheckStatus(bqResp.StatusCode(), bqResp.Body); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not create question: %v\n", err)
			continue
		}
		if bqResp.JSON200 == nil {
			continue
		}
		fmt.Printf("  BQ-%d  %s\n", bqResp.JSON200.Id, cli.Truncate(q, 70))
	}

	fmt.Println("\nanswer these questions to get started:")
	fmt.Printf("  dx question list\n")
	fmt.Printf("  dx question answer <ID> --answer=\"...\"\n")
	return nil
}
