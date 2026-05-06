package streams

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/dxclient"
)

// Summary mirrors the per-pass counts the proposal-evaluate command has
// printed since IS-618. Streams is the number of stale streams visited;
// Processed is verdicts posted; Revised is revision events posted; Skipped
// is streams that errored (logged to stderr) but did not abort the pass.
type Summary struct {
	Streams   int
	Processed int
	Revised   int
	Skipped   int
}

// CommentVerdict is the row Classify returns per pending user event. The
// JSON tags match the shape we ask the LLM to emit so the same struct can
// be unmarshaled directly.
type CommentVerdict struct {
	EventID     int64  `json:"event_id"`
	Verdict     string `json:"verdict"`
	RevisedBody string `json:"revised_body,omitempty"`
}

// Classifier is the swappable comment-classification step. Production
// uses ClaudeClassifier; tests inject a fake.
type Classifier func(ctx context.Context, body string, pending []dxclient.EventItem) ([]CommentVerdict, error)

// RunOnce processes every stale stream once. If targetTypes is empty,
// every registered handler is polled; otherwise only the listed types
// are polled (each must be registered or RunOnce returns an error).
//
// Errors on individual streams go to stderr and are counted as Skipped —
// the loop keeps advancing rather than wedging on a single bad event.
func RunOnce(ctx context.Context, c *cli.Client, slug string, targetTypes []string, classify Classifier) (Summary, error) {
	if classify == nil {
		classify = ClaudeClassifier
	}
	var handlers []StreamHandler
	if len(targetTypes) == 0 {
		handlers = All()
	} else {
		for _, tt := range targetTypes {
			h, err := Lookup(tt)
			if err != nil {
				return Summary{}, err
			}
			handlers = append(handlers, h)
		}
	}

	var sum Summary
	for _, h := range handlers {
		s, err := runForHandler(ctx, c, slug, h, classify)
		if err != nil {
			return sum, err
		}
		sum.Streams += s.Streams
		sum.Processed += s.Processed
		sum.Revised += s.Revised
		sum.Skipped += s.Skipped
	}
	return sum, nil
}

func runForHandler(ctx context.Context, c *cli.Client, slug string, h StreamHandler, classify Classifier) (Summary, error) {
	var sum Summary
	tt := h.TargetType()
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
	if staleResp.JSON200 == nil || staleResp.JSON200.Streams == nil {
		return sum, nil
	}
	streamList := *staleResp.JSON200.Streams
	sum.Streams = len(streamList)
	for _, s := range streamList {
		processed, revised, err := evaluateStream(ctx, c, slug, s, h, classify)
		if err != nil {
			fmt.Fprintf(os.Stderr, "stream %s/%s: %v\n", s.TargetType, s.TargetId, err)
			sum.Skipped++
			continue
		}
		sum.Processed += processed
		sum.Revised += revised
	}
	return sum, nil
}

func evaluateStream(ctx context.Context, c *cli.Client, slug string, stream dxclient.StreamItem, h StreamHandler, classify Classifier) (processed, revised int, err error) {
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

	pending := pendingUserEvents(*evResp.JSON200.Events)
	if len(pending) == 0 {
		return 0, 0, nil
	}

	body, err := h.FetchBody(ctx, c, slug, stream.TargetId)
	if err != nil {
		return 0, 0, fmt.Errorf("fetch body: %w", err)
	}

	verdicts, err := classify(ctx, body, pending)
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
			if err := h.ApplyAddressed(ctx, c, slug, stream.TargetId, v.EventID, v.RevisedBody); err != nil {
				return processed, revised, fmt.Errorf("apply addressed: %w", err)
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

func pendingUserEvents(events []dxclient.EventItem) []dxclient.EventItem {
	var pending []dxclient.EventItem
	for _, e := range events {
		if e.AuthorKind != "user" {
			continue
		}
		if e.AgentProcessResult != nil {
			continue
		}
		pending = append(pending, e)
	}
	return pending
}

// ClaudeClassifier shells out to the `claude` CLI to classify pending
// user comments against the canonical body. Extracted from
// internal/cli/project/proposal_evaluate.go so every target_type shares
// the same prompt shape and parsing rules.
func ClaudeClassifier(ctx context.Context, body string, pending []dxclient.EventItem) ([]CommentVerdict, error) {
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
	var verdicts []CommentVerdict
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
