package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli/agent"
	"github.com/iodesystems/zdx-go/internal/cli/configcmd"
	"github.com/iodesystems/zdx-go/internal/cli/credentialhelper"
	"github.com/iodesystems/zdx-go/internal/cli/devtools"
	"github.com/iodesystems/zdx-go/internal/cli/project"
	"github.com/iodesystems/zdx-go/internal/cli/servercmd"
	"github.com/iodesystems/zdx-go/internal/cli/work"
)

func main() {
	root := &cobra.Command{
		Use:           "dx",
		Short:         "Developer experience CLI",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(
		work.TodoCmd(),
		project.IssueCmd(),
		project.FeatureCmd(),
		project.GoalCmd(),
		project.VisionCmd(),
		work.ConstraintCmd(),
		project.JournalCmd(),
		project.FocusCmd(),
		project.ThemeCmd(), // legacy alias
		project.SpecCmd(),
		project.PlanCmd(),
		project.MaturityCmd(),
		project.DoctorCmd(),
		devtools.BuildCmd(),
		devtools.TestCmd(),
		devtools.LintCmd(),
		devtools.UICmd(),
		devtools.CheckCmd(),
		devtools.WatchCmd(),
		devtools.HooksCmd(),
		servercmd.InitCmd(),
		servercmd.SetupCmd(),
		servercmd.DaemonCmd(),
		servercmd.ServeCmd(),
		servercmd.MigrateCmd(),
		devtools.ErrorsCmd(),
		project.CommentCmd(),
		project.RevisionCmd(),
		project.RefCmd(),
		project.QaCmd(),
		project.QuestionCmd(),
		project.QuestionProposalCmd(),
		project.ProposalCmd(),
		devtools.TimeCmd(),
		devtools.ClaudeCmd(),
		agent.AgentCmd(),
		project.TaskCmd(),
		project.ReviewCmd(),
		project.PatternCmd(),
		project.EnvCmd(),
		project.DiscussionCmd(),
		servercmd.IntegrateCmd(),
		servercmd.ImportCmd(),
		servercmd.LoginCmd(),
		configcmd.ConfigCmd(),
		credentialhelper.CredentialHelperCmd(),
	)

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
