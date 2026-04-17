package devtools

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/dxclient"
)

func ErrorsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "errors", Short: "Error and slow query observability"}
	cmd.AddCommand(errorsListCmd(), errorsReportCmd(), slowQueriesListCmd(), timedListCmd())
	return cmd
}

func errorsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List recent error reports for this project",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			resp, err := c.ListErrorsWithResponse(cmd.Context(), &dxclient.ListErrorsParams{
				Slug: c.SlugOrDie(),
			})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil || resp.JSON200.Errors == nil || len(*resp.JSON200.Errors) == 0 {
				fmt.Println("no errors")
				return nil
			}
			for _, e := range *resp.JSON200.Errors {
				ts := e.CreatedAt
				if len(ts) >= 10 {
					ts = ts[:10]
				}
				fmt.Printf("[%s] %-40s  %s  %s\n", ts, e.ErrorName, e.Source, e.Endpoint)
			}
			return nil
		},
	}
}

func errorsReportCmd() *cobra.Command {
	var source, endpoint, name, stack string
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Report an error to this project",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			resp, err := c.ReportErrorWithResponse(cmd.Context(), dxclient.ReportErrorRequest{
				Slug:       c.SlugOrDie(),
				Source:     source,
				Endpoint:   endpoint,
				ErrorName:  name,
				StackTrace: stack,
			})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil {
				return fmt.Errorf("empty response")
			}
			fmt.Printf("reported error #%d\n", resp.JSON200.Id)
			return nil
		},
	}
	cmd.Flags().StringVar(&source, "source", "", "source service or component")
	cmd.Flags().StringVar(&endpoint, "endpoint", "", "affected endpoint or URL")
	cmd.Flags().StringVar(&name, "name", "", "error name or type")
	cmd.Flags().StringVar(&stack, "stack", "", "stack trace")
	cmd.MarkFlagRequired("source")
	cmd.MarkFlagRequired("name")
	return cmd
}

func slowQueriesListCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "slow-queries", Short: "List slow queries"}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List recent slow queries for this project",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			resp, err := c.ListSlowQueriesWithResponse(cmd.Context(), &dxclient.ListSlowQueriesParams{
				Slug: c.SlugOrDie(),
			})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil || resp.JSON200.Queries == nil || len(*resp.JSON200.Queries) == 0 {
				fmt.Println("no slow queries")
				return nil
			}
			for _, q := range *resp.JSON200.Queries {
				ts := q.CreatedAt
				if len(ts) >= 10 {
					ts = ts[:10]
				}
				text := q.SqlText
				if len(text) > 60 {
					text = text[:57] + "..."
				}
				fmt.Printf("[%s] %5dms  %-20s  %s\n", ts, q.DurationMs, q.Endpoint, text)
			}
			return nil
		},
	})
	return cmd
}

func timedListCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "timed", Short: "List high-water-mark timed operations"}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List slowest-ever operations",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			params := &dxclient.ListTimedParams{}
			if slug := c.Slug(); slug != "" {
				params.Slug = &slug
			}
			resp, err := c.ListTimedWithResponse(cmd.Context(), params)
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil || resp.JSON200.Items == nil || len(*resp.JSON200.Items) == 0 {
				fmt.Println("no timed records")
				return nil
			}
			for _, t := range *resp.JSON200.Items {
				ts := t.CreatedAt
				if len(ts) >= 10 {
					ts = ts[:10]
				}
				fmt.Printf("[%s] %5dms  %s\n", ts, t.DurationMs, t.Name)
			}
			return nil
		},
	})
	return cmd
}
