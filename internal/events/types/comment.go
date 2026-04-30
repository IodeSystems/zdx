package types

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/iodesystems/zdx-go/internal/events"
)

// commentSummaryMaxLen bounds the length of Comment.Summary so agent context
// stays compact; full body lives in Detail.
const commentSummaryMaxLen = 200

// Comment renders an event_type="comment" event. The detail blob is a free-form
// authored message; format is "markdown" by default and may be "text".
type Comment struct{}

// CommentDetail is the parsed shape of a comment event's detail JSON.
type CommentDetail struct {
	Body   string `json:"body"`
	Format string `json:"format,omitempty"`
}

func (c *Comment) EventType() string          { return "comment" }
func (c *Comment) Audience() []string         { return []string{events.AudienceUI, events.AudienceAgent} }
func (c *Comment) ReactComponentName() string { return "EventComment" }

func (c *Comment) parse(detail json.RawMessage) (CommentDetail, error) {
	var d CommentDetail
	if len(detail) == 0 {
		return d, errors.New("comment: detail is empty")
	}
	if err := json.Unmarshal(detail, &d); err != nil {
		return d, fmt.Errorf("comment: invalid JSON: %w", err)
	}
	return d, nil
}

func (c *Comment) Validate(detail json.RawMessage) error {
	d, err := c.parse(detail)
	if err != nil {
		return err
	}
	if strings.TrimSpace(d.Body) == "" {
		return errors.New("comment: body is required")
	}
	if d.Format != "" && d.Format != "markdown" && d.Format != "text" {
		return fmt.Errorf("comment: invalid format %q (want markdown or text)", d.Format)
	}
	return nil
}

func (c *Comment) Summary(detail json.RawMessage) (string, error) {
	d, err := c.parse(detail)
	if err != nil {
		return "", err
	}
	return truncate(collapseWhitespace(d.Body), commentSummaryMaxLen), nil
}

func (c *Comment) Detail(detail json.RawMessage) (any, error) {
	d, err := c.parse(detail)
	if err != nil {
		return nil, err
	}
	if d.Format == "" {
		d.Format = "markdown"
	}
	return d, nil
}

func init() { events.Register(&Comment{}) }

func collapseWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func truncate(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return string(r[:maxRunes-1]) + "…"
}
