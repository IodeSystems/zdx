package tokencmd

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/dxclient"
)

func TokenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "token",
		Short: "Mint, list, and revoke project-scoped admin API tokens",
	}
	cmd.AddCommand(mintCmd(), listCmd(), revokeCmd())
	return cmd
}

func mintCmd() *cobra.Command {
	var project, name string
	cmd := &cobra.Command{
		Use:   "mint",
		Short: "Mint a project-scoped admin token (admin-only)",
		Long: `Creates an API key with project_scope=[<slug>] and prints the raw token to stdout.
The token is shown exactly once and cannot be retrieved later.

Pipe-friendly: stdout is just the token; the warning goes to stderr, e.g.
    dx token mint --project=foo > .token`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMint(cmd.Context(), cli.MustClient(), project, name, cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "project slug to scope the token to (required)")
	cmd.Flags().StringVar(&name, "name", "", "human-readable token label (default: admin-token)")
	return cmd
}

func runMint(ctx context.Context, c *cli.Client, project, name string, stdout, stderr io.Writer) error {
	if project == "" {
		return fmt.Errorf("--project is required")
	}
	body := dxclient.CreateAdminTokenJSONRequestBody{ProjectSlug: project}
	if name != "" {
		body.Name = &name
	}
	resp, err := c.CreateAdminTokenWithResponse(ctx, body)
	if err != nil {
		return err
	}
	if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
		return err
	}
	if resp.JSON200 == nil || resp.JSON200.Token == nil {
		return fmt.Errorf("mint: server returned no token")
	}
	fmt.Fprintln(stdout, *resp.JSON200.Token)
	fmt.Fprintln(stderr, "# Token shown once; not retrievable later.")
	return nil
}

func listCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all admin API tokens (admin-only)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd.Context(), cli.MustClient(), cmd.OutOrStdout())
		},
	}
}

func runList(ctx context.Context, c *cli.Client, stdout io.Writer) error {
	resp, err := c.ListAdminTokensWithResponse(ctx)
	if err != nil {
		return err
	}
	if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
		return err
	}
	if resp.JSON200 == nil || resp.JSON200.Tokens == nil || len(*resp.JSON200.Tokens) == 0 {
		fmt.Fprintln(stdout, "(no tokens)")
		return nil
	}
	fmt.Fprintf(stdout, "%-6s %-24s %-30s %-30s %-20s %s\n",
		"ID", "NAME", "USER", "PROJECT_SCOPE", "LAST_USED", "CREATED")
	for _, t := range *resp.JSON200.Tokens {
		email := ""
		if t.UserEmail != nil {
			email = *t.UserEmail
		}
		scope := "-"
		if t.ProjectScope != nil && len(*t.ProjectScope) > 0 {
			scope = strings.Join(*t.ProjectScope, ",")
		}
		lastUsed := "-"
		if t.LastUsedAt != nil && *t.LastUsedAt != "" {
			lastUsed = *t.LastUsedAt
		}
		fmt.Fprintf(stdout, "%-6d %-24s %-30s %-30s %-20s %s\n",
			t.Id, cli.Truncate(t.Name, 24), cli.Truncate(email, 30),
			cli.Truncate(scope, 30), lastUsed, t.CreatedAt)
	}
	return nil
}

func revokeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "revoke <id>",
		Short: "Revoke an admin token by id (admin-only)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRevoke(cmd.Context(), cli.MustClient(), args[0], cmd.OutOrStdout())
		},
	}
}

func runRevoke(ctx context.Context, c *cli.Client, idArg string, stdout io.Writer) error {
	id, err := parseInt32(idArg)
	if err != nil {
		return fmt.Errorf("invalid id %q: %w", idArg, err)
	}
	resp, err := c.DeleteAdminTokenWithResponse(ctx, id)
	if err != nil {
		return err
	}
	if resp.StatusCode() == 404 {
		return fmt.Errorf("no such token")
	}
	if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
		return err
	}
	fmt.Fprintln(stdout, "revoked")
	return nil
}

func parseInt32(s string) (int32, error) {
	var n int32
	_, err := fmt.Sscanf(s, "%d", &n)
	if err != nil {
		return 0, err
	}
	return n, nil
}
