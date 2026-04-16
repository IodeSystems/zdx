package cli

import "time"

// AgentItem is the JSON shape of an agent row returned by /api/agents/*.
// Kept here (not in internal/cli/agent) because todo.go consumes it and the
// agent subpackage imports cli for Client — reversing that would create a cycle.
type AgentItem struct {
	ID             string `json:"id"`
	ProjectID      int32  `json:"project_id"`
	SessionID      string `json:"session_id"`
	WorktreePath   string `json:"worktree_path"`
	WorktreeBranch string `json:"worktree_branch"`
	Pid            int32  `json:"pid"`
	Status         string `json:"status"`
	TaskGroup      string `json:"task_group"`
	ComposeProject string `json:"compose_project"`
	ServerPort     int32  `json:"server_port"`
	DatabaseUrl    string `json:"database_url"`
	ValkeyUrl      string `json:"valkey_url"`
	LastHeartbeat  string `json:"last_heartbeat"`
	CreatedAt      string `json:"created_at"`
}

// AgentTaskItem is the JSON shape of a task row returned by /api/tasks/claim.
type AgentTaskItem struct {
	ID             string `json:"id"`
	Text           string `json:"text"`
	Feature        string `json:"feature"`
	Status         string `json:"status"`
	Issue          string `json:"issue"`
	TaskGroup      string `json:"task_group"`
	CreatedAt      string `json:"created_at"`
	ClaimedAt      string `json:"claimed_at"`
	LeaseExpiresAt string `json:"lease_expires_at"`
}

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
