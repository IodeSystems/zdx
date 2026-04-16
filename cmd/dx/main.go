package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/cli/agent"
	"github.com/iodesystems/zdx-go/internal/cli/devtools"
	"github.com/iodesystems/zdx-go/internal/cli/mcpcmd"
	"github.com/iodesystems/zdx-go/internal/cli/servercmd"
)

func main() {
	root := &cobra.Command{
		Use:           "dx",
		Short:         "Developer experience CLI",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(
		cli.TodoCmd(),
		cli.IssueCmd(),
		cli.FeatureCmd(),
		cli.GoalCmd(),
		cli.ConstraintCmd(),
		cli.JournalCmd(),
		cli.ThemeCmd(),
		cli.SpecCmd(),
		devtools.BuildCmd(),
		devtools.TestCmd(),
		devtools.LintCmd(),
		devtools.CheckCmd(),
		devtools.WatchCmd(),
		devtools.HooksCmd(),
		cli.CtxCmd(),
		servercmd.InitCmd(),
		servercmd.SetupCmd(),
		servercmd.DaemonCmd(),
		servercmd.ServeCmd(),
		servercmd.MigrateCmd(),
		devtools.ErrorsCmd(),
		cli.CommentCmd(),
		cli.RevisionCmd(),
		cli.RefCmd(),
		cli.QaCmd(),
		cli.QuestionCmd(),
		cli.QuestionProposalCmd(),
		mcpcmd.McpCmd(),
		devtools.TimeCmd(),
		devtools.ClaudeCmd(),
		agent.AgentCmd(),
		cli.TaskCmd(),
		cli.PatternCmd(),
		servercmd.IntegrateCmd(),
		servercmd.LoginCmd(),
	)

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
