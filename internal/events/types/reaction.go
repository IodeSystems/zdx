package types

import (
	"encoding/json"
	"errors"
	"fmt"
	"unicode/utf8"

	"github.com/iodesystems/zdx-go/internal/events"
)

// reactionEmojiMaxRunes bounds emoji length; multi-codepoint emoji (with ZWJs
// and skin-tone modifiers) routinely run 4–7 runes, so 8 is the practical cap.
const reactionEmojiMaxRunes = 8

// Reaction renders an event_type="reaction" event. Reactions are agent-only:
// the UI hides them by default to keep threads readable.
type Reaction struct{}

// ReactionDetail is the parsed shape of a reaction event's detail JSON.
type ReactionDetail struct {
	Emoji         string `json:"emoji"`
	TargetEventID int64  `json:"target_event_id"`
}

func (r *Reaction) EventType() string          { return "reaction" }
func (r *Reaction) Audience() []string         { return []string{events.AudienceAgent} }
func (r *Reaction) ReactComponentName() string { return "EventReaction" }

func (r *Reaction) parse(detail json.RawMessage) (ReactionDetail, error) {
	var d ReactionDetail
	if len(detail) == 0 {
		return d, errors.New("reaction: detail is empty")
	}
	if err := json.Unmarshal(detail, &d); err != nil {
		return d, fmt.Errorf("reaction: invalid JSON: %w", err)
	}
	return d, nil
}

func (r *Reaction) Validate(detail json.RawMessage) error {
	d, err := r.parse(detail)
	if err != nil {
		return err
	}
	if d.Emoji == "" {
		return errors.New("reaction: emoji is required")
	}
	if n := utf8.RuneCountInString(d.Emoji); n > reactionEmojiMaxRunes {
		return fmt.Errorf("reaction: emoji too long (%d runes, max %d)", n, reactionEmojiMaxRunes)
	}
	if d.TargetEventID <= 0 {
		return errors.New("reaction: target_event_id must be > 0")
	}
	return nil
}

func (r *Reaction) Summary(detail json.RawMessage) (string, error) {
	d, err := r.parse(detail)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s on event %d", d.Emoji, d.TargetEventID), nil
}

func (r *Reaction) Detail(detail json.RawMessage) (any, error) {
	d, err := r.parse(detail)
	if err != nil {
		return nil, err
	}
	return d, nil
}

func init() { events.Register(&Reaction{}) }
