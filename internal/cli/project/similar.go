package project

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/cli/clitypes"
	"github.com/iodesystems/zdx-go/internal/dxclient"
)

func SimilarCmd() *cobra.Command {
	var n int64
	cmd := &cobra.Command{
		Use:   "similar <IS-N | TK-N | text...>",
		Short: "Find items similar to an ID or free text",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			slug := c.SlugOrDie()
			ctx := cmd.Context()

			query, err := resolveQuery(cmd, c, slug, args)
			if err != nil {
				return err
			}

			type issueResult struct {
				items []dxclient.SimilarIssueItem
				err   error
			}
			type taskResult struct {
				items []dxclient.SimilarTaskItem
				err   error
			}
			type questionResult struct {
				items []dxclient.SimilarQuestionItem
				err   error
			}
			type patternResult struct {
				items []dxclient.SimilarPatternItem
				err   error
			}

			var (
				issuesCh    = make(chan issueResult, 1)
				tasksCh     = make(chan taskResult, 1)
				questionsCh = make(chan questionResult, 1)
				patternsCh  = make(chan patternResult, 1)
				wg          sync.WaitGroup
			)

			wg.Add(4)
			go func() {
				defer wg.Done()
				resp, err := c.SimilarIssuesWithResponse(ctx, dxclient.SimilarIssuesJSONRequestBody{Slug: slug, Text: query, N: &n})
				if err != nil {
					issuesCh <- issueResult{err: err}
					return
				}
				if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
					issuesCh <- issueResult{err: err}
					return
				}
				var items []dxclient.SimilarIssueItem
				if resp.JSON200 != nil && resp.JSON200.Issues != nil {
					items = *resp.JSON200.Issues
				}
				issuesCh <- issueResult{items: items}
			}()

			go func() {
				defer wg.Done()
				resp, err := c.SimilarTasksWithResponse(ctx, dxclient.SimilarTasksJSONRequestBody{Slug: slug, Text: query, N: &n})
				if err != nil {
					tasksCh <- taskResult{err: err}
					return
				}
				if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
					tasksCh <- taskResult{err: err}
					return
				}
				var items []dxclient.SimilarTaskItem
				if resp.JSON200 != nil && resp.JSON200.Tasks != nil {
					items = *resp.JSON200.Tasks
				}
				tasksCh <- taskResult{items: items}
			}()

			go func() {
				defer wg.Done()
				resp, err := c.SimilarQuestionsWithResponse(ctx, dxclient.SimilarQuestionsJSONRequestBody{Slug: slug, Text: query, N: &n})
				if err != nil {
					questionsCh <- questionResult{err: err}
					return
				}
				if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
					questionsCh <- questionResult{err: err}
					return
				}
				var items []dxclient.SimilarQuestionItem
				if resp.JSON200 != nil && resp.JSON200.Questions != nil {
					items = *resp.JSON200.Questions
				}
				questionsCh <- questionResult{items: items}
			}()

			go func() {
				defer wg.Done()
				resp, err := c.SimilarPatternsWithResponse(ctx, dxclient.SimilarPatternsJSONRequestBody{Slug: slug, Text: query, N: &n})
				if err != nil {
					patternsCh <- patternResult{err: err}
					return
				}
				if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
					patternsCh <- patternResult{err: err}
					return
				}
				var items []dxclient.SimilarPatternItem
				if resp.JSON200 != nil && resp.JSON200.Patterns != nil {
					items = *resp.JSON200.Patterns
				}
				patternsCh <- patternResult{items: items}
			}()

			wg.Wait()
			close(issuesCh)
			close(tasksCh)
			close(questionsCh)
			close(patternsCh)

			issues := <-issuesCh
			tasks := <-tasksCh
			questions := <-questionsCh
			patterns := <-patternsCh

			printed := false

			if issues.err == nil && len(issues.items) > 0 {
				fmt.Println("Issues:")
				for _, s := range issues.items {
					fmt.Printf("  %-8s  (%.0f%%)  %-10s  %s\n", s.Id, s.Score*100, "["+s.Status+"]", s.Title)
				}
				printed = true
			}

			if tasks.err == nil && len(tasks.items) > 0 {
				if printed {
					fmt.Println()
				}
				fmt.Println("Tasks:")
				for _, s := range tasks.items {
					issue := ""
					if s.Issue != "" {
						issue = "  " + s.Issue
					}
					fmt.Printf("  %-8s  (%.0f%%)  %-10s  %s%s\n", s.Id, s.Score*100, "["+s.Status+"]", s.Title, issue)
				}
				printed = true
			}

			if questions.err == nil && len(questions.items) > 0 {
				if printed {
					fmt.Println()
				}
				fmt.Println("Questions:")
				for _, s := range questions.items {
					answered := ""
					if s.Answer != "" {
						answered = "  [answered]"
					}
					fmt.Printf("  BQ-%-5d  (%.0f%%)  %s%s\n", s.Id, s.Score*100, cli.Truncate(s.Question, 70), answered)
				}
				printed = true
			}

			if patterns.err == nil && len(patterns.items) > 0 {
				if printed {
					fmt.Println()
				}
				fmt.Println("Patterns:")
				for _, s := range patterns.items {
					fmt.Printf("  PT-%-5d  (%.0f%%)  %-20s  %s\n", s.Pattern.Id, s.Score*100, s.Pattern.Name, cli.Truncate(s.Pattern.Description, 60))
				}
				printed = true
			}

			if !printed {
				fmt.Println("no similar items found")
			}

			return nil
		},
	}
	cmd.Flags().Int64Var(&n, "n", 5, "max results per type")
	return cmd
}

// resolveQuery converts the args into a query string. If args[0] matches a
// known ID prefix, it fetches the item and uses its title + body as the query.
// Otherwise it joins all args as free text.
func resolveQuery(cmd *cobra.Command, c *cli.Client, slug string, args []string) (string, error) {
	id := args[0]
	ctx := cmd.Context()

	if len(id) > 3 && strings.EqualFold(id[:3], "is-") {
		n, err := strconv.ParseInt(id[3:], 10, 32)
		if err != nil {
			return strings.Join(args, " "), nil
		}
		resp, err := c.ShowIssueWithResponse(ctx, &dxclient.ShowIssueParams{Slug: slug, Id: clitypes.IssueIDStr(int32(n))})
		if err != nil {
			return "", fmt.Errorf("fetch %s: %w", id, err)
		}
		if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
			return "", fmt.Errorf("fetch %s: %w", id, err)
		}
		if resp.JSON200 == nil {
			return "", fmt.Errorf("fetch %s: empty response", id)
		}
		iss := resp.JSON200.Issue
		parts := []string{iss.Title}
		if iss.Context != "" {
			parts = append(parts, iss.Context)
		}
		fmt.Printf("query: %s  %s\n\n", id, iss.Title)
		return strings.Join(parts, "\n"), nil
	}

	if len(id) > 3 && strings.EqualFold(id[:3], "tk-") {
		n, err := strconv.ParseInt(id[3:], 10, 32)
		if err != nil {
			return strings.Join(args, " "), nil
		}
		resp, err := c.GetTaskWithResponse(ctx, &dxclient.GetTaskParams{Slug: slug, Id: clitypes.TaskIDStr(int32(n))})
		if err != nil {
			return "", fmt.Errorf("fetch %s: %w", id, err)
		}
		if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
			return "", fmt.Errorf("fetch %s: %w", id, err)
		}
		if resp.JSON200 == nil {
			return "", fmt.Errorf("fetch %s: empty response", id)
		}
		task := resp.JSON200
		parts := []string{task.Title}
		if task.Text != "" {
			parts = append(parts, task.Text)
		}
		fmt.Printf("query: %s  %s\n\n", id, task.Title)
		return strings.Join(parts, "\n"), nil
	}

	return strings.Join(args, " "), nil
}
