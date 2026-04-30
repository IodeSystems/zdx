package types

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/iodesystems/zdx-go/internal/events"
)

// Revision renders an event_type="revision" event — a structured edit to a
// canonical text field on the parent entity. The "from" text may be empty
// only when the prior version is 0 (initial creation).
type Revision struct{}

// RevisionDetail is the parsed shape of a revision event's detail JSON. The
// from/to fields use *string so we can distinguish "missing" from "empty".
type RevisionDetail struct {
	Field       string  `json:"field"`
	From        *string `json:"from"`
	To          *string `json:"to"`
	FromVersion int     `json:"from_version"`
	ToVersion   int     `json:"to_version"`
	Actor       string  `json:"actor"`
}

var revisionAllowedFields = map[string]bool{
	"body":        true,
	"title":       true,
	"description": true,
}

func (r *Revision) EventType() string          { return "revision" }
func (r *Revision) Audience() []string         { return []string{events.AudienceUI, events.AudienceAgent} }
func (r *Revision) ReactComponentName() string { return "EventRevision" }

func (r *Revision) parse(detail json.RawMessage) (RevisionDetail, error) {
	var d RevisionDetail
	if len(detail) == 0 {
		return d, errors.New("revision: detail is empty")
	}
	if err := json.Unmarshal(detail, &d); err != nil {
		return d, fmt.Errorf("revision: invalid JSON: %w", err)
	}
	return d, nil
}

func (r *Revision) Validate(detail json.RawMessage) error {
	d, err := r.parse(detail)
	if err != nil {
		return err
	}
	if d.Field == "" {
		return errors.New("revision: field is required")
	}
	if !revisionAllowedFields[d.Field] {
		return fmt.Errorf("revision: invalid field %q", d.Field)
	}
	if d.From == nil {
		return errors.New("revision: from is required (use empty string for first revision)")
	}
	if d.To == nil || *d.To == "" {
		return errors.New("revision: to is required and non-empty")
	}
	if d.FromVersion < 0 {
		return fmt.Errorf("revision: from_version must be >= 0 (got %d)", d.FromVersion)
	}
	if d.ToVersion <= d.FromVersion {
		return fmt.Errorf("revision: to_version (%d) must exceed from_version (%d)", d.ToVersion, d.FromVersion)
	}
	if *d.From == *d.To {
		return errors.New("revision: from and to are identical (no-op rejected)")
	}
	if d.Actor == "" {
		return errors.New("revision: actor is required")
	}
	return nil
}

func (r *Revision) Summary(detail json.RawMessage) (string, error) {
	d, err := r.parse(detail)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s updated (v%d → v%d)", d.Field, d.FromVersion, d.ToVersion), nil
}

func (r *Revision) Detail(detail json.RawMessage) (any, error) {
	d, err := r.parse(detail)
	if err != nil {
		return nil, err
	}
	return d, nil
}

func init() { events.Register(&Revision{}) }
