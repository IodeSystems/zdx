package cli

import "time"

// HeartbeatLoop sends periodic heartbeats to the agent until stop closes.
func HeartbeatLoop(c *Client, agentID string, interval time.Duration, stop <-chan struct{}) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			_ = c.Post("/api/agents/"+agentID+"/heartbeat", nil, nil)
		}
	}
}
