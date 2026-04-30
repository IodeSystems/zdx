// Package clitypes holds JSON wire shapes shared across CLI subpackages
// (cli, cli/agent, cli/project, cli/work). Types live here so subpackages
// can depend on them without creating import cycles.
package clitypes

import (
	"encoding/json"
	"fmt"
)

// StringOrStrings unmarshals either a single string or an array of strings
// into []string. Used for fields the server may emit in either shape
// (e.g. IssueItem.BlockedBy).
type StringOrStrings []string

func (s *StringOrStrings) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '[' {
		var arr []string
		if err := json.Unmarshal(data, &arr); err != nil {
			return err
		}
		*s = arr
		return nil
	}
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	if str == "" {
		*s = nil
	} else {
		*s = []string{str}
	}
	return nil
}

type IssueItem struct {
	ID              int32             `json:"id"`
	Title           string            `json:"title"`
	Status          string            `json:"status"`
	Priority        string            `json:"priority"`
	Component       string            `json:"component"`
	BlockedBy       StringOrStrings   `json:"blocked_by"`
	BlockedByDetail []IssueBlockerRef `json:"blocked_by_detail,omitempty"`
	Context         string            `json:"context"`
	IssueType       string            `json:"issue_type"`
	DuplicateOf     string            `json:"duplicate_of,omitempty"`
	LinkOf          string            `json:"link_of,omitempty"`
	ReopenCount     int32             `json:"reopen_count,omitempty"`
	InteractiveOnly bool              `json:"interactive_only,omitempty"`
	URL             string            `json:"url"`
}

type IssueBlockerRef struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Kind   string `json:"kind,omitempty"` // 'sequencing' or 'composition'
}

type IssueWorkItem struct {
	Agent     string `json:"agent"`
	Note      string `json:"note"`
	CreatedAt string `json:"created_at"`
}

type SimilarIssueItem struct {
	ID      string  `json:"id"`
	Title   string  `json:"title"`
	Context string  `json:"context"`
	Status  string  `json:"status"`
	Score   float32 `json:"score"`
}

type SimilarTaskItem struct {
	ID     string  `json:"id"`
	Text   string  `json:"text"`
	Status string  `json:"status"`
	Issue  string  `json:"issue"`
	Score  float32 `json:"score"`
}

type IssueAddResponse struct {
	IssueItem
	Similar []SimilarIssueItem `json:"similar,omitempty"`
}

type TaskAddResponse struct {
	TaskItem
	Similar          []SimilarTaskItem `json:"similar,omitempty"`
	DuplicateBlocked bool              `json:"duplicate_blocked,omitempty"`
}

type TaskItem struct {
	ID         int32  `json:"id"`
	Title      string `json:"title"`
	Text       string `json:"text"`
	Feature    string `json:"feature"`
	Status     string `json:"status"`
	Reason     string `json:"reason"`
	IssueID    *int32 `json:"issue_id,omitempty"`
	TaskGroup  string `json:"task_group"`
	TestPlan   string `json:"test_plan"`
	TestRefs   string `json:"test_refs"`
	CreatedAt  string `json:"created_at"`
	StaleSince string `json:"stale_since,omitempty"`
}

type FeatureItem struct {
	ID              int32      `json:"id"`
	Name            string     `json:"name"`
	Description     string     `json:"description"`
	What            string     `json:"what"`
	Why             string     `json:"why"`
	DoneWhen        string     `json:"done_when"`
	Component       string     `json:"component"`
	Category        string     `json:"category"`
	Kind            string     `json:"kind"`
	GoalID          int32      `json:"goal_id"`
	ParentFeatureID int32      `json:"parent_feature_id"`
	MetricName      string     `json:"metric_name"`
	MetricUnit      string     `json:"metric_unit"`
	BaselineValue   string     `json:"baseline_value"`
	TargetValue     string     `json:"target_value"`
	GraphURL        string     `json:"graph_url"`
	PlanType        string     `json:"plan_type"`
	Specs           []SpecItem `json:"specs"`
}

type SpecItem struct {
	ID          int32  `json:"id"`
	Description string `json:"description"`
	Importance  string `json:"importance"`
	GreenDemos  int32  `json:"green_demos"`
	TotalDemos  int32  `json:"total_demos"`
}

type CommentItem struct {
	ID          int32  `json:"id"`
	TargetType  string `json:"target_type"`
	TargetID    string `json:"target_id"`
	Author      string `json:"author"`
	AuthorAlias string `json:"author_alias,omitempty"`
	Body        string `json:"body"`
	CreatedAt   string `json:"created_at"`
	ParentID    *int32 `json:"parent_id,omitempty"`
	Unread      *bool  `json:"unread,omitempty"`
}

type QuestionItem struct {
	ID        int32  `json:"id"`
	Category  string `json:"category"`
	Question  string `json:"question"`
	Answer    string `json:"answer"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type PatternItem struct {
	ID          int32           `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	CodeRefs    json.RawMessage `json:"code_refs"`
	CreatedAt   string          `json:"created_at"`
	UpdatedAt   string          `json:"updated_at"`
}

type ThemeItem = FocusItem

type FocusItem struct {
	ID          int32  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Priority    int32  `json:"priority"`
	Status      string `json:"status"`
	Blockers    string `json:"blockers"`
	StartedAt   string `json:"started_at"`
	EndedAt     string `json:"ended_at"`
	CreatedAt   string `json:"created_at"`
}

type GoalItem struct {
	ID          int32  `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Priority    int32  `json:"priority"`
	Status      string `json:"status"`
	MetricName  string `json:"metric_name"`
	MetricUnit  string `json:"metric_unit"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// AgentItem is the JSON shape of an agent row returned by /api/agents/*.
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

// BlockerQuestionItem is the JSON shape of a blocker question.
// Used by both project/ (question command) and work/ (todo solo surfaces them).
type BlockerQuestionItem struct {
	ID         int32    `json:"id"`
	TargetType string   `json:"target_type"`
	TargetID   string   `json:"target_id"`
	Context    string   `json:"context"`
	Choices    []string `json:"choices"`
	Answer     string   `json:"answer"`
	AnsweredBy string   `json:"answered_by"`
	Status     string   `json:"status"`
	CreatedAt  string   `json:"created_at"`
	AnsweredAt string   `json:"answered_at"`
}

// RevisionItem is the JSON shape of a revision row.
// Used by both project/ (revision command) and work/ (todo show pulls them in).
type RevisionItem struct {
	ID         int32  `json:"id"`
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
	Field      string `json:"field"`
	OldVal     string `json:"old_val"`
	NewVal     string `json:"new_val"`
	Agent      string `json:"agent"`
	CreatedAt  string `json:"created_at"`
}

func IssueIDStr(n int32) string { return fmt.Sprintf("IS-%d", n) }
func TaskIDStr(n int32) string  { return fmt.Sprintf("TK-%d", n) }
