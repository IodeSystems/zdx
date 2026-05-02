package project

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/dxclient"
)

// proposalEvaluateCmd implements `dx proposal evaluate [--loop]`. It is the
// agent loop that closes the IS-610 invariant for proposal-target events:
// every user-authored event must end up with an agent_process_result.
//
// Single-pass logic per stale stream: list events, pick out user comments
// without an agent_process_result, ask Claude to classify them against the
// current proposal body (and optionally rewrite it), then post a revision
// event (when "addressed" with a new body) and a verdict per comment. The
// /api/events/{id}/verdict handler bumps stream.last_evaluated_at as a
// side effect, so a clean pass also clears the stream from the stale list.
//
// IS-619 will generalize this loop across target types; evaluateOnce
// already accepts a targetType so the proposal-specific body can be
// extracted into a typed StreamHandler later without reshaping the loop.
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
				summary, err := evaluateOnce(cmd.Context(), c, slug, "proposal")
				if err != nil {
					return err
				}
				fmt.Printf("[%s] streams=%d processed=%d revised=%d skipped=%d\n",
					time.Now().Format(time.RFC3339),
					summary.streams, summary.processed, summary.revised, summary.skipped)
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

type evaluateSummary struct {
	streams   int
	processed int
	revised   int
	skipped   int
}

// evaluateOnce processes every stale stream of the given targetType once.
// Failures on individual streams are logged to stderr and counted as
// skipped — the loop should keep advancing rather than wedging on a single
// bad event.
func evaluateOnce(ctx context.Context, c *cli.Client, slug, targetType string) (evaluateSummary, error) {
	var sum evaluateSummary
	tt := targetType
	staleResp, err := c.ListStaleStreamsWithResponse(ctx, &dxclient.ListStaleStreamsParams{
		Slug:       slug,
		TargetType: &tt,
	})
	if err != nil {
		return sum, fmt.Errorf("list stale streams: %w", err)
	}
	if err := c.CheckStatus(staleResp.StatusCode(), staleResp.Body); err != nil {
		return sum, err
	}
	if staleResp.JSON200 == nil {
		return sum, nil
	}
	streams := staleResp.JSON200.Streams
	sum.streams = len(streams)
	for _, s := range streams {
		processed, revised, err := evaluateStream(ctx, c, slug, s)
		if err != nil {
			fmt.Fprintf(os.Stderr, "stream %s/%s: %v\n", s.TargetType, s.TargetId, err)
			sum.skipped++
			continue
		}
		sum.processed += processed
		sum.revised += revised
	}
	return sum, nil
}

func evaluateStream(ctx context.Context, c *cli.Client, slug string, stream dxclient.StreamItem) (processed, revised int, err error) {
	evResp, err := c.ListEventsWithResponse(ctx, &dxclient.ListEventsParams{
		Slug:       slug,
		TargetType: stream.TargetType,
		TargetId:   stream.TargetId,
	})
	if err != nil {
		return 0, 0, fmt.Errorf("list events: %w", err)
	}
	if err := c.CheckStatus(evResp.StatusCode(), evResp.Body); err != nil {
		return 0, 0, err
	}
	if evResp.JSON200 == nil || evResp.JSON200.Events == nil {
		return 0, 0, nil
	}

	var pending []dxclient.EventItem
	for _, e := range *evResp.JSON200.Events {
		if e.AuthorKind != "user" {
			continue
		}
		if e.AgentProcessResult != nil {
			continue
		}
		pending = append(pending, e)
	}
	if len(pending) == 0 {
		return 0, 0, nil
	}

	// Only the proposal target type is wired here; future target types plug
	// in via IS-619's StreamHandler registry.
	if stream.TargetType != "proposal" {
		return 0, 0, fmt.Errorf("unsupported target_type %q", stream.TargetType)
	}
	pid, err := parseProposalID(stream.TargetId)
	if err != nil {
		return 0, 0, err
	}
	propResp, err := c.ShowProposalWithResponse(ctx, pid, &dxclient.ShowProposalParams{Slug: slug})
	if err != nil {
		return 0, 0, fmt.Errorf("show proposal: %w", err)
	}
	if err := c.CheckStatus(propResp.StatusCode(), propResp.Body); err != nil {
		return 0, 0, err
	}
	if propResp.JSON200 == nil {
		return 0, 0, fmt.Errorf("empty proposal response")
	}
	body := propResp.JSON200.Proposal.Body

	verdicts, err := classifyComments(ctx, body, pending)
	if err != nil {
		return 0, 0, err
	}

	for _, v := range verdicts {
		if v.Verdict != "addressed" && v.Verdict != "already_addressed" && v.Verdict != "for_someone_else" {
			fmt.Fprintf(os.Stderr, "stream %s/%s: skipping event %d with invalid verdict %q\n",
				stream.TargetType, stream.TargetId, v.EventID, v.Verdict)
			continue
		}
		if v.Verdict == "addressed" && strings.TrimSpace(v.RevisedBody) != "" {
			detail, _ := json.Marshal(map[string]any{
				"body":                v.RevisedBody,
				"addressing_event_id": v.EventID,
			})
			eventType := "revision"
			authorKind := "agent"
			revResp, err := c.AddEventCommentWithResponse(ctx, dxclient.AddEventCommentRequest{
				Slug:       slug,
				TargetType: stream.TargetType,
				TargetId:   stream.TargetId,
				EventType:  &eventType,
				AuthorKind: &authorKind,
				DetailJson: json.RawMessage(detail),
			})
			if err != nil {
				return processed, revised, fmt.Errorf("post revision: %w", err)
			}
			if err := c.CheckStatus(revResp.StatusCode(), revResp.Body); err != nil {
				return processed, revised, err
			}
			revised++
		}

		result := map[string]any{"verdict": v.Verdict}
		if v.RevisedBody != "" {
			result["revised_body"] = v.RevisedBody
		}
		raw, _ := json.Marshal(result)
		vResp, err := c.SetEventVerdictWithResponse(ctx, v.EventID, dxclient.SetEventVerdictRequest{
			Slug:               slug,
			AgentProcessResult: json.RawMessage(raw),
		})
		if err != nil {
			return processed, revised, fmt.Errorf("set verdict: %w", err)
		}
		if err := c.CheckStatus(vResp.StatusCode(), vResp.Body); err != nil {
			return processed, revised, err
		}
		processed++
	}
	return processed, revised, nil
}

type commentVerdict struct {
	EventID     int64  `json:"event_id"`
	Verdict     string `json:"verdict"`
	RevisedBody string `json:"revised_body,omitempty"`
}

// classifyComments runs the Claude CLI against the proposal body + pending
// comments and parses the JSON array verdict back. The prompt deliberately
// constrains the output shape and the allowed verdict vocabulary so the
// loop can run without per-call human review.
func classifyComments(ctx context.Context, body string, pending []dxclient.EventItem) ([]commentVerdict, error) {
	type promptComment struct {
		EventID int64  `json:"event_id"`
		Author  string `json:"author"`
		Detail  string `json:"detail"`
	}
	pcs := make([]promptComment, 0, len(pending))
	for _, e := range pending {
		pcs = append(pcs, promptComment{
			EventID: e.Id,
			Author:  e.Author,
			Detail:  detailText(e.DetailJson),
		})
	}
	pcsJSON, err := json.MarshalIndent(pcs, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal comments: %w", err)
	}

	prompt := `You are evaluating user comments on a software proposal. Classify each comment against the current proposal body.

For each comment, return one verdict:
- "addressed": the comment raised a concern that should change the proposal; include a revised_body that incorporates the change.
- "already_addressed": the proposal as written already covers the concern; no revision needed.
- "for_someone_else": the comment is out of scope for this proposal (e.g. belongs in a different issue, asks the human reviewer a question).

Return ONLY a JSON array, no markdown fences, no commentary. Each element:
  {"event_id": <int>, "verdict": "<one of the above>", "revised_body": "<full revised proposal body, only when verdict=addressed>"}

Proposal body:
"""
` + body + `
"""

Pending comments (JSON):
` + string(pcsJSON)

	claudePath, err := exec.LookPath("claude")
	if err != nil {
		return nil, fmt.Errorf("claude CLI not found in PATH")
	}
	out, err := exec.CommandContext(ctx, claudePath, "-p", prompt, "--output-format", "text").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("claude -p failed: %w\n%s", err, out)
	}
	raw := strings.TrimSpace(string(out))
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)
	var verdicts []commentVerdict
	if err := json.Unmarshal([]byte(raw), &verdicts); err != nil {
		return nil, fmt.Errorf("parse claude response: %w\nraw: %s", err, raw)
	}
	return verdicts, nil
}

func detailText(detail any) string {
	if detail == nil {
		return ""
	}
	switch v := detail.(type) {
	case string:
		return v
	case map[string]any:
		if b, ok := v["body"].(string); ok {
			return b
		}
		if t, ok := v["text"].(string); ok {
			return t
		}
		raw, _ := json.Marshal(v)
		return string(raw)
	default:
		raw, _ := json.Marshal(v)
		return string(raw)
	}
}
