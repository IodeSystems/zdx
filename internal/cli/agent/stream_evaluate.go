package agent

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/events/streams"
)

// streamEvaluateCmd implements `dx agent stream-evaluate [--loop] [--target-type=...]`.
// It drives the StreamHandler registry so every registered target_type gets
// its poll/classify/verdict loop for free — the generic agent loop for
// IS-637..IS-647 rollouts.
func streamEvaluateCmd() *cobra.Command {
	var loop bool
	var interval time.Duration
	var targetTypesStr string

	cmd := &cobra.Command{
		Use:   "stream-evaluate",
		Short: "Evaluate user comments on stale event streams across all registered target types",
		Long: `Evaluate user comments on stale event streams across all registered target types.

Without --target-type the command processes every registered StreamHandler
(proposal, task, etc.). Pass --target-type to restrict to specific handlers.

The registry plug-in model means new target_types are automatically picked up
the moment a StreamHandler is registered — IS-637..IS-647 authors just need to
register their handler; this command does the rest.

Examples:
  dx agent stream-evaluate                    # all registered handlers
  dx agent stream-evaluate --target-type=proposal  # proposal only
  dx agent stream-evaluate --loop             # continuous polling
  dx agent stream-evaluate --loop --interval=60s   # 60s loop interval`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			slug := c.SlugOrDie()

			var targetTypes []string
			if targetTypesStr != "" {
				for _, tt := range strings.Split(targetTypesStr, ",") {
					tt = strings.TrimSpace(tt)
					if tt == "" {
						continue
					}
					if _, err := streams.Lookup(tt); err != nil {
						return fmt.Errorf("no handler registered for target_type %q: %w", tt, err)
					}
					targetTypes = append(targetTypes, tt)
				}
			}

			run := func() error {
				summary, err := streams.RunOnce(cmd.Context(), c, slug, targetTypes, nil)
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
	cmd.Flags().StringVar(&targetTypesStr, "target-type", "", "comma-separated list of target types to process (empty = all registered)")
	return cmd
}
