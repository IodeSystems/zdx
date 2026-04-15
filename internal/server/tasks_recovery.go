package server

import (
	"context"
	"log"
	"time"
)

func (s *Server) StartTaskRecovery(ctx context.Context) {
	sweep := func() {
		reclaimed, err := s.q.ReclaimExpiredTasks(ctx)
		if err != nil {
			log.Printf("task-recovery: reclaim expired: %v", err)
		} else if len(reclaimed) > 0 {
			log.Printf("task-recovery: reclaimed %d expired-lease tasks", len(reclaimed))
		}

		orphaned, err := s.q.CancelOrphanedTasks(ctx)
		if err != nil {
			log.Printf("task-recovery: cancel orphaned: %v", err)
		} else if len(orphaned) > 0 {
			log.Printf("task-recovery: cancelled %d orphaned tasks (parent issue closed)", len(orphaned))
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
