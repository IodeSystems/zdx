package devtools

import (
	"encoding/json"
	"fmt"
	"github.com/iodesystems/zdx-go/internal/cli"
	"net/url"

	"github.com/spf13/cobra"
)

type errorReportItem struct {
	ID         int64  `json:"id"`
	Source     string `json:"source"`
	Endpoint   string `json:"endpoint"`
	ErrorName  string `json:"error_name"`
	StackTrace string `json:"stack_trace"`
	CreatedAt  string `json:"created_at"`
}

type slowQueryItem struct {
	ID          int64  `json:"id"`
	SqlHash     string `json:"sql_hash"`
	SqlText     string `json:"sql_text"`
	Endpoint    string `json:"endpoint"`
	DurationMs  int32  `json:"duration_ms"`
	ExplainJson string `json:"explain_json"`
	CreatedAt   string `json:"created_at"`
}

type timedItem struct {
	ID          int64           `json:"id"`
	Name        string          `json:"name"`
	DurationMs  int32           `json:"duration_ms"`
	Source      string          `json:"source"`
	ContextJson json.RawMessage `json:"context_json"`
	CreatedAt   string          `json:"created_at"`
}

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
			var resp struct {
				Errors []errorReportItem `json:"errors"`
			}
			if err := c.Get("/api/dx/errors", cli.QuerySlug(c), &resp); err != nil {
				return err
			}
			if len(resp.Errors) == 0 {
				fmt.Println("no errors")
				return nil
			}
			for _, e := range resp.Errors {
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
			var resp errorReportItem
			if err := c.Post("/api/dx/errors/report", map[string]any{
				"slug":        c.SlugOrDie(),
				"source":      source,
				"endpoint":    endpoint,
				"error_name":  name,
				"stack_trace": stack,
			}, &resp); err != nil {
				return err
			}
			fmt.Printf("reported error #%d\n", resp.ID)
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
			var resp struct {
				Queries []slowQueryItem `json:"queries"`
			}
			if err := c.Get("/api/dx/slow-queries", url.Values{"slug": {c.SlugOrDie()}}, &resp); err != nil {
				return err
			}
			if len(resp.Queries) == 0 {
				fmt.Println("no slow queries")
				return nil
			}
			for _, q := range resp.Queries {
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
			var resp struct {
				Items []timedItem `json:"items"`
			}
			params := url.Values{}
			if slug := c.Slug(); slug != "" {
				params.Set("slug", slug)
			}
			if err := c.Get("/api/dx/timed", params, &resp); err != nil {
				return err
			}
			if len(resp.Items) == 0 {
				fmt.Println("no timed records")
				return nil
			}
			for _, t := range resp.Items {
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
