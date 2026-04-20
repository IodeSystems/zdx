package server

import (
	"context"
	"log"
	"time"

	"github.com/iodesystems/zdx-go/internal/db"
	"github.com/iodesystems/zdx-go/internal/server/handlers"
)

// staleAgentSessionMinutes is how long an agent session's updated_at can stall
// before the sweeper auto-closes it and marks it 'orphaned'. Agent event
// ingestion touches updated_at on every streamed event, so a 30m silence is a
// safe "the process died without closing" signal.
const staleAgentSessionMinutes int32 = 30

func (s *Server) StartTaskRecovery(ctx context.Context) {
	sweep := func() {
		released, err := s.q.ReleaseExpiredReservations(ctx)
		if err != nil {
			log.Printf("task-recovery: release expired reservations: %v", err)
		} else if len(released) > 0 {
			byType := map[string]int{}
			for _, r := range released {
				byType[r.TargetType]++
			}
			log.Printf("task-recovery: released %d expired reservations (todo=%d, task=%d, issue=%d)",
				len(released), byType["todo"], byType["task"], byType["issue"])
		}

		reclaimed, err := s.q.ReclaimExpiredTasks(ctx)
		if err != nil {
			log.Printf("task-recovery: reclaim expired: %v", err)
		} else if len(reclaimed) > 0 {
			log.Printf("task-recovery: reclaimed %d expired-lease tasks", len(reclaimed))
			for _, r := range reclaimed {
				handlers.RecordStatusChange(ctx, s.q, r.ProjectID, "task", r.ID, "active", "ready", "")
			}
		}

		orphaned, err := s.q.CancelOrphanedTasks(ctx)
		if err != nil {
			log.Printf("task-recovery: cancel orphaned: %v", err)
		} else if len(orphaned) > 0 {
			log.Printf("task-recovery: cancelled %d orphaned tasks (parent issue closed)", len(orphaned))
			for _, r := range orphaned {
				handlers.RecordStatusChange(ctx, s.q, r.ProjectID, "task", r.ID, "", "done", "")
			}
		}

		flagged, err := s.q.FlagStaleTasks(ctx, db.FlagStaleTasksParams{StaleDays: 3, ProjectID: 0})
		if err != nil {
			log.Printf("task-recovery: flag stale: %v", err)
		} else if len(flagged) > 0 {
			log.Printf("task-recovery: flagged %d stale unclaimed tasks", len(flagged))
		}

		staleSessions, err := s.q.CloseStaleClaudeSessions(ctx, staleAgentSessionMinutes)
		if err != nil {
			log.Printf("task-recovery: close stale agent sessions: %v", err)
		} else if len(staleSessions) > 0 {
			log.Printf("task-recovery: closed %d stale agent sessions (updated_at > %dm old)", len(staleSessions), staleAgentSessionMinutes)
		}
	}

	sweep()
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				sweep()
			}
		}
	}()
}
