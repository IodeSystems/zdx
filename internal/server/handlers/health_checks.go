package handlers

import (
	"context"
	"fmt"
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

// dbHealthLatencyThreshold is the round-trip ping latency above which the DB
// subsystem reports "degraded". Tuned for IS-809: a healthy local connection
// pings in under 5ms; a half-second round-trip indicates real trouble.
const dbHealthLatencyThreshold = 500 * time.Millisecond

// checkDB pings the Postgres pool with a short deadline. Reports "degraded"
// on error or when the ping exceeds dbHealthLatencyThreshold.
func (h *Handler) checkDB(ctx context.Context) SubsystemState {
	pingCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	start := time.Now()
	err := h.Pool.Ping(pingCtx)
	latency := time.Since(start)
	now := time.Now().UTC()
	if err != nil {
		reason := err.Error()
		return SubsystemState{State: "degraded", Reason: &reason, Since: &now}
	}
	if latency > dbHealthLatencyThreshold {
		reason := fmt.Sprintf("ping latency %s exceeds %s threshold", latency.Round(time.Millisecond), dbHealthLatencyThreshold)
		return SubsystemState{State: "degraded", Reason: &reason, Since: &now}
	}
	return SubsystemState{State: "ok"}
}

// checkEmbedder reports the embedder subsystem state for IS-809:
//   - no live client → "unconfigured" (covers both: schema lacks the
//     zdx_llm_configs table, and table exists but has no config row — both
//     mean "no LLM is set up", which is expected, not a degradation).
//   - live client → "ok".
//
// A future "degraded" state will be reported when last-successful-embed-time
// tracking is added (per IS-809) so we can detect a configured-but-failing
// embedder. Today the bare presence of the client is the only liveness signal.
func (h *Handler) checkEmbedder(_ context.Context) SubsystemState {
	if h.Emb != nil && h.Emb.IsAlive() {
		return SubsystemState{State: "ok"}
	}
	now := time.Now().UTC()
	var reason string
	if !h.Features.HasLLMConfig {
		reason = "no zdx_llm_configs table in this schema"
	} else {
		reason = "no LLM config row present (or Reload not yet run)"
	}
	return SubsystemState{State: "unconfigured", Reason: &reason, Since: &now}
}
