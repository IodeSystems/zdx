package server

import (
	"context"
	"log"
	"time"

	"github.com/iodesystems/zdx-go/internal/db"
)

func (s *Server) StartTaskRecovery(ctx context.Context) {
	sweep := func() {
		reclaimed, err := s.q.ReclaimExpiredTasks(ctx)
		if err != nil {
			log.Printf("task-recovery: reclaim expired: %v", err)
		} else if len(reclaimed) > 0 {
			log.Printf("task-recovery: reclaimed %d expired-lease tasks", len(reclaimed))
			for _, r := range reclaimed {
				prevAgent := ""
				if r.ClaimedBy.Valid {
					prevAgent = r.ClaimedBy.String
				}
				s.recordStatusChange(ctx, r.ProjectID, "task", r.ID, "active", "ready", prevAgent)
			}
		}

		orphaned, err := s.q.CancelOrphanedTasks(ctx)
		if err != nil {
			log.Printf("task-recovery: cancel orphaned: %v", err)
		} else if len(orphaned) > 0 {
			log.Printf("task-recovery: cancelled %d orphaned tasks (parent issue closed)", len(orphaned))
			for _, r := range orphaned {
				s.recordStatusChange(ctx, r.ProjectID, "task", r.ID, "", "done", "")
			}
		}

		flagged, err := s.q.FlagStaleTasks(ctx, db.FlagStaleTasksParams{StaleDays: 3, ProjectID: 0})
		if err != nil {
			log.Printf("task-recovery: flag stale: %v", err)
		} else if len(flagged) > 0 {
			log.Printf("task-recovery: flagged %d stale unclaimed tasks", len(flagged))
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
