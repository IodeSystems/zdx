package db

import "time"

// Row types mirror the table columns exactly. sqlc generates these from schema.

type Project struct {
	ID        int32     `db:"id"         json:"id"`
	Slug      string    `db:"slug"        json:"slug"`
	Name      string    `db:"name"        json:"name"`
	CreatedAt time.Time `db:"created_at"  json:"created_at"`
}

type Issue struct {
	ID        string    `db:"id"          json:"id"`
	ProjectID int32     `db:"project_id"  json:"project_id"`
	Title     string    `db:"title"       json:"title"`
	Status    string    `db:"status"      json:"status"`
	Priority  string    `db:"priority"    json:"priority"`
	Component string    `db:"component"   json:"component"`
	Context   string    `db:"context"     json:"context"`
	BlockedBy string    `db:"blocked_by"  json:"blocked_by"`
	CreatedAt time.Time `db:"created_at"  json:"created_at"`
}

type IssueWork struct {
	ID        int32     `db:"id"          json:"id"`
	IssueID   string    `db:"issue_id"    json:"issue_id"`
	Agent     string    `db:"agent"       json:"agent"`
	Note      string    `db:"note"        json:"note"`
	CreatedAt time.Time `db:"created_at"  json:"created_at"`
}

type Task struct {
	ID          string     `db:"id"           json:"id"`
	ProjectID   int32      `db:"project_id"   json:"project_id"`
	Text        string     `db:"text"         json:"text"`
	Feature     string     `db:"feature"      json:"feature"`
	Status      string     `db:"status"       json:"status"`
	Reason      string     `db:"reason"       json:"reason"`
	Issue       string     `db:"issue"        json:"issue"`
	Depends     string     `db:"depends"      json:"depends"`
	TestPlan    string     `db:"test_plan"    json:"test_plan"`
	TestRefs    string     `db:"test_refs"    json:"test_refs"`
	CreatedAt   time.Time  `db:"created_at"   json:"created_at"`
	CompletedAt *time.Time `db:"completed_at" json:"completed_at"`
}

type Feature struct {
	ID          int32  `db:"id"          json:"id"`
	ProjectID   int32  `db:"project_id"  json:"project_id"`
	Name        string `db:"name"        json:"name"`
	Description string `db:"description" json:"description"`
	What        string `db:"what"        json:"what"`
	Why         string `db:"why"         json:"why"`
	DoneWhen    string `db:"done_when"   json:"done_when"`
	Component   string `db:"component"   json:"component"`
}

type Spec struct {
	ID          int32  `db:"id"          json:"id"`
	FeatureID   int32  `db:"feature_id"  json:"feature_id"`
	Description string `db:"description" json:"description"`
	Kind        string `db:"kind"        json:"kind"`
}
