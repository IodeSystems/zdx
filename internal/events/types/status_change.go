package types

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/iodesystems/zdx-go/internal/events"
)

// StatusChange renders an event_type="status_change" event — a transition
// tuple (target_kind, from_status, to_status, actor). target_kind is left as
// a free string because zdx tracks several entity kinds (issue, proposal,
// task, feature, ...) and new kinds are added without churning this file.
type StatusChange struct{}

// StatusChangeDetail is the parsed shape of a status_change event's detail JSON.
type StatusChangeDetail struct {
	TargetKind string `json:"target_kind"`
	FromStatus string `json:"from_status"`
	ToStatus   string `json:"to_status"`
	Actor      string `json:"actor"`
}

func (s *StatusChange) EventType() string          { return "status_change" }
func (s *StatusChange) Audience() []string         { return []string{events.AudienceUI, events.AudienceAgent} }
func (s *StatusChange) ReactComponentName() string { return "EventStatusChange" }

func (s *StatusChange) parse(detail json.RawMessage) (StatusChangeDetail, error) {
	var d StatusChangeDetail
	if len(detail) == 0 {
		return d, errors.New("status_change: detail is empty")
	}
	if err := json.Unmarshal(detail, &d); err != nil {
		return d, fmt.Errorf("status_change: invalid JSON: %w", err)
	}
	return d, nil
}

func (s *StatusChange) Validate(detail json.RawMessage) error {
	d, err := s.parse(detail)
	if err != nil {
		return err
	}
	if d.TargetKind == "" {
		return errors.New("status_change: target_kind is required")
	}
	if d.FromStatus == "" {
		return errors.New("status_change: from_status is required")
	}
	if d.ToStatus == "" {
		return errors.New("status_change: to_status is required")
	}
	if d.FromStatus == d.ToStatus {
		return errors.New("status_change: from_status and to_status are identical (no-op rejected)")
	}
	if d.Actor == "" {
		return errors.New("status_change: actor is required")
	}
	return nil
}

func (s *StatusChange) Summary(detail json.RawMessage) (string, error) {
	d, err := s.parse(detail)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s %s → %s", d.TargetKind, d.FromStatus, d.ToStatus), nil
}

func (s *StatusChange) Detail(detail json.RawMessage) (any, error) {
	d, err := s.parse(detail)
	if err != nil {
		return nil, err
	}
	return d, nil
}

func init() { events.Register(&StatusChange{}) }
