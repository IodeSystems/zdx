// Package apitypes contains all HTTP request and response types for dx-server.
// tygo reads this package and generates ui/src/api/generated-types.ts.
// Keep this file free of business logic — plain structs with json tags only.
package apitypes

// ── shared ────────────────────────────────────────────────────────────────────

type OKResponse struct {
	OK bool `json:"ok"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

// ── projects ──────────────────────────────────────────────────────────────────

type ProjectResp struct {
	ID        int32  `json:"id"`
	Slug      string `json:"slug"`
	Name      string `json:"name"`
	CreatedAt string `json:"createdAt"`
}

type CreateProjectRequest struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

// ── issues ────────────────────────────────────────────────────────────────────

type IssueResp struct {
	ID        string          `json:"id"`
	Title     string          `json:"title"`
	Status    string          `json:"status"`
	Priority  string          `json:"priority"`
	Component string          `json:"component"`
	Context   string          `json:"context"`
	BlockedBy string          `json:"blockedBy"`
	CreatedAt string          `json:"createdAt"`
	Work      []IssueWorkResp `json:"work,omitempty"`
}

type IssueWorkResp struct {
	Agent     string `json:"agent"`
	Note      string `json:"note"`
	CreatedAt string `json:"createdAt"`
}

type CreateIssueRequest struct {
	Slug      string `json:"slug"`
	Title     string `json:"title"`
	Context   string `json:"context"`
	Priority  string `json:"priority"`
	Component string `json:"component"`
}

type UpdateIssueRequest struct {
	Slug      string `json:"slug"`
	ID        string `json:"id"`
	Title     string `json:"title,omitempty"`
	Context   string `json:"context,omitempty"`
	Priority  string `json:"priority,omitempty"`
	BlockedBy string `json:"blockedBy,omitempty"`
}

type CloseIssueRequest struct {
	Slug   string `json:"slug"`
	ID     string `json:"id"`
	Reason string `json:"reason"`
}

type AppendIssueWorkRequest struct {
	Slug  string `json:"slug"`
	ID    string `json:"id"`
	Agent string `json:"agent"`
	Note  string `json:"note"`
}

// ── tasks ─────────────────────────────────────────────────────────────────────

type TaskResp struct {
	ID          string  `json:"id"`
	Text        string  `json:"text"`
	Feature     string  `json:"feature"`
	Status      string  `json:"status"`
	Reason      string  `json:"reason"`
	Issue       string  `json:"issue"`
	Depends     string  `json:"depends"`
	TestPlan    string  `json:"testPlan"`
	TestRefs    string  `json:"testRefs"`
	CreatedAt   string  `json:"createdAt"`
	CompletedAt *string `json:"completedAt,omitempty"`
}

type CreateTaskRequest struct {
	Slug    string `json:"slug"`
	Text    string `json:"text"`
	Feature string `json:"feature"`
	Issue   string `json:"issue"`
}

type UpdateTaskStatusRequest struct {
	Slug   string `json:"slug"`
	ID     string `json:"id"`
	Status string `json:"status"`
	Reason string `json:"reason"`
}

type MarkTaskDoneRequest struct {
	Slug     string `json:"slug"`
	ID       string `json:"id"`
	TestPlan string `json:"testPlan"`
	TestRefs string `json:"testRefs"`
}

type MarkTaskUndoneRequest struct {
	Slug string `json:"slug"`
	ID   string `json:"id"`
}

// ── features ──────────────────────────────────────────────────────────────────

type FeatureResp struct {
	ID          int32      `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	What        string     `json:"what"`
	Why         string     `json:"why"`
	DoneWhen    string     `json:"doneWhen"`
	Component   string     `json:"component"`
	Specs       []SpecResp `json:"specs,omitempty"`
}

type SpecResp struct {
	ID          int32  `json:"id"`
	Description string `json:"description"`
	Kind        string `json:"kind"` // must | should | could
}

type CreateFeatureRequest struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type UpdateFeatureFieldRequest struct {
	Slug    string `json:"slug"`
	Feature string `json:"feature"`
	Field   string `json:"field"`
	Value   string `json:"value"`
}

type AddSpecRequest struct {
	Slug        string `json:"slug"`
	Feature     string `json:"feature"`
	Description string `json:"description"`
	Kind        string `json:"kind"`
}

// ── solo ──────────────────────────────────────────────────────────────────────

// SoloItem is what the solo queue returns as the next item to work on.
type SoloItem struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`    // triage | plan | dev
	Summary string `json:"summary"`
	Issue   string `json:"issue,omitempty"`
	Task    string `json:"task,omitempty"`
	Feature string `json:"feature,omitempty"`
}
