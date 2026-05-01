package handlers

import (
	"context"
	"time"
)

// SubsystemState reports the health of a single subsystem on /api/health.
// State is one of: "ok", "degraded", "unconfigured".
type SubsystemState struct {
	State  string     `json:"state"`
	Since  *time.Time `json:"since,omitempty"`
	Reason *string    `json:"reason,omitempty"`
	Depth  *int64     `json:"depth,omitempty"`
}

// HealthOutput is the response shape for GET /api/health.
type HealthOutput struct {
	Status     string                    `json:"status"`
	BuildSHA   string                    `json:"build_sha"`
	Subsystems map[string]SubsystemState `json:"subsystems"`
}

// checkQueue probes the solo-queue worker subsystem. Reports the count of
// unclaimed open todos as `depth`. Per IS-809, this subsystem is non-fatal:
// queue health never degrades the top-level status.
func (h *Handler) checkQueue(ctx context.Context) SubsystemState {
	depth, err := h.Q.CountUnclaimedTodos(ctx)
	if err != nil {
		reason := err.Error()
		now := time.Now().UTC()
		return SubsystemState{State: "unconfigured", Reason: &reason, Since: &now}
	}
	// TODO(IS-809): when an in-server worker heartbeat is added, mark `degraded`
	// (with a since/reason) if the worker has not heartbeated within ~2 minutes.
	return SubsystemState{State: "ok", Depth: &depth}
}

// rollupTopLevelStatus returns "ok" if every subsystem is "ok" or "unconfigured",
// otherwise "degraded". Queue is excluded from the rollup (non-fatal).
func rollupTopLevelStatus(subsystems map[string]SubsystemState) string {
	for name, s := range subsystems {
		if name == "queue" {
			continue
		}
		if s.State != "ok" && s.State != "unconfigured" {
			return "degraded"
		}
	}
	return "ok"
}
