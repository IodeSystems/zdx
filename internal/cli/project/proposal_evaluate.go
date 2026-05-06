package project

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/events/streams"
)

// proposalEvaluateCmd implements `dx proposal evaluate [--loop]`. It is the
// agent loop that closes the IS-610 invariant for proposal-target events:
// every user-authored event must end up with an agent_process_result.
//
// IS-619-T2 lifted the per-pass loop into internal/events/streams so every
// target_type shares the same poll/classify/verdict machinery; the proposal
// flow is just one registered StreamHandler.
func proposalEvaluateCmd() *cobra.Command {
	var loop bool
	var interval time.Duration
	cmd := &cobra.Command{
		Use:   "evaluate",
		Short: "Evaluate user comments on stale proposal streams (sets verdicts; optionally revises body)",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			slug := c.SlugOrDie()
			run := func() error {
				summary, err := streams.RunOnce(cmd.Context(), c, slug, []string{"proposal"}, nil)
				if err != nil {
					return err
				}
				fmt.Printf("[%s] streams=%d processed=%d revised=%d skipped=%d\n",
					time.Now().Format(time.RFC3339),
					summary.Streams, summary.Processed, summary.Revised, summary.Skipped)
				return nil
			}
			if !loop {
				return run()
			}
			if interval <= 0 {
				interval = 30 * time.Second
			}
			t := time.NewTicker(interval)
			defer t.Stop()
			if err := run(); err != nil {
				fmt.Fprintf(os.Stderr, "evaluate error: %v\n", err)
			}
			for {
				select {
				case <-cmd.Context().Done():
					return nil
				case <-t.C:
					if err := run(); err != nil {
						fmt.Fprintf(os.Stderr, "evaluate error: %v\n", err)
					}
				}
			}
		},
	}
	cmd.Flags().BoolVar(&loop, "loop", false, "run continuously, polling between passes")
	cmd.Flags().DurationVar(&interval, "interval", 30*time.Second, "loop interval (only with --loop)")
	return cmd
}
