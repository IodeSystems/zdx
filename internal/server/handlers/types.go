package handlers

// ── Response types ─────────────────────────────────────────────────────────

type IssueItem struct {
	ID          int32    `json:"id" doc:"Server integer ID; CLI formats as IS-N"`
	Title       string   `json:"title"`
	Status      string   `json:"status"`
	Priority    string   `json:"priority"`
	Component   string   `json:"component"`
	Features    string   `json:"features"`
	BlockedBy   []string `json:"blocked_by"`
	Context     string   `json:"context"`
	Source      string   `json:"source"`
	IssueType   string   `json:"issue_type"`
	DuplicateOf string   `json:"duplicate_of,omitempty"`
	URL         string   `json:"url"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

type TaskItem struct {
	ID             int32  `json:"id" doc:"Server integer ID; CLI formats as TK-N"`
	Text           string `json:"text"`
	Feature        string `json:"feature"`
	Status         string `json:"status"`
	Reason         string `json:"reason"`
	IssueID        *int32 `json:"issue_id,omitempty" doc:"Linked issue integer ID; CLI formats as IS-N"`
	Depends        string `json:"depends"`
	TestPlan       string `json:"test_plan"`
	TestRefs       string `json:"test_refs"`
	TaskGroup      string `json:"task_group"`
	CreatedAt      string `json:"created_at"`
	CompletedAt    string `json:"completed_at"`
	UpdatedAt      string `json:"updated_at"`
	ReviewedAt     string `json:"reviewed_at,omitempty"`
	StaleSince     string `json:"stale_since,omitempty"`
	ClaimedBy      string `json:"claimed_by,omitempty"`
	ClaimedAt      string `json:"claimed_at,omitempty"`
	LeaseExpiresAt string `json:"lease_expires_at,omitempty"`
}

type FeatureItem struct {
	ID          int32      `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	What        string     `json:"what"`
	Why         string     `json:"why"`
	DoneWhen    string     `json:"done_when"`
	Component   string     `json:"component"`
	Category    string     `json:"category"`
	PlanType    string     `json:"plan_type"`
	PlanStatus  string     `json:"plan_status"`
	HasTestRefs bool       `json:"has_test_refs"`
	Specs       []SpecItem `json:"specs"`
}

type SpecItem struct {
	ID             int32  `json:"id"`
	Description    string `json:"description"`
	Kind           string `json:"kind"`
	Deferred       bool   `json:"deferred"`
	DeferredReason string `json:"deferred_reason"`
}

type SpecIssueItem struct {
	SpecID  int32  `json:"spec_id"`
	IssueID string `json:"issue_id"`
	Title   string `json:"title"`
	Status  string `json:"status"`
}

type SpecTestItem struct {
	ID        int32  `json:"id"`
	Component string `json:"component"`
	Name      string `json:"name"`
	Layer     string `json:"layer"`
	Status    string `json:"status"`
}

type SpecDemoItem struct {
	ID            int32  `json:"id"`
	Type          string `json:"type"`
	TestComponent string `json:"test_component"`
	TestName      string `json:"test_name"`
	URL           string `json:"url"`
	Name          string `json:"name"`
}

type UncoveredSpecItem struct {
	ID          int32  `json:"id"`
	FeatureID   int32  `json:"feature_id"`
	FeatureName string `json:"feature_name"`
	Description string `json:"description"`
	Kind        string `json:"kind"`
}

type ThemeItem struct {
	ID          int32  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Priority    int32  `json:"priority"`
	Status      string `json:"status"`
	Blockers    string `json:"blockers"`
	CreatedAt   string `json:"created_at"`
}

type TodoItem struct {
	ID         int32  `json:"id"`
	Text       string `json:"text"`
	Key        string `json:"key"`
	Persona    string `json:"persona"`
	Priority   int32  `json:"priority"`
	Status     string `json:"status"`
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
	Kind       string `json:"kind"`
	IssueRef   string `json:"issue_ref"`
	Blocked    bool   `json:"blocked"`
	ClaimedBy  string `json:"claimed_by,omitempty"`
	ClaimedAt  string `json:"claimed_at,omitempty"`
	CreatedAt  string `json:"created_at"`
	ResolvedAt string `json:"resolved_at,omitempty"`
}

type ErrorReportItem struct {
	ID            int64  `json:"id"`
	Source        string `json:"source"`
	Endpoint      string `json:"endpoint"`
	ErrorName     string `json:"error_name"`
	StackTrace    string `json:"stack_trace"`
	CreatedAt     string `json:"created_at"`
	LinkedIssueID string `json:"linked_issue_id,omitempty"`
}

type SlowQueryItem struct {
	ID          int64  `json:"id"`
	SqlHash     string `json:"sql_hash"`
	SqlText     string `json:"sql_text"`
	Endpoint    string `json:"endpoint"`
	DurationMs  int32  `json:"duration_ms"`
	ExplainJson string `json:"explain_json"`
	CreatedAt   string `json:"created_at"`
}

type JournalEntryItem struct {
	ID            int32  `json:"id"`
	Date          string `json:"date"`
	Baseline      bool   `json:"baseline"`
	Tldr          string `json:"tldr"`
	Assessment    string `json:"assessment"`
	Concerns      string `json:"concerns"`
	Next          string `json:"next"`
	ChangelogJSON string `json:"changelog_json"`
	StateJSON     string `json:"state_json"`
}

type IssueWorkItem struct {
	Agent     string `json:"agent"`
	Note      string `json:"note"`
	CreatedAt string `json:"created_at"`
}

type OKBody struct {
	OK bool `json:"ok"`
}

type CodeRefItem struct {
	ID        int32  `json:"id"`
	FilePath  string `json:"file_path"`
	GitHash   string `json:"git_hash"`
	LineStart int32  `json:"line_start"`
	LineEnd   int32  `json:"line_end"`
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

type SimilarQuestionItem struct {
	ID       int32   `json:"id"`
	Question string  `json:"question"`
	Answer   string  `json:"answer"`
	Score    float32 `json:"score"`
}

type QuestionItem struct {
	ID               int32  `json:"id"`
	Category         string `json:"category"`
	Question         string `json:"question"`
	Answer           string `json:"answer"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at"`
	ParentQuestionID *int32 `json:"parent_question_id"`
}

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

type QuestionProposalItem struct {
	ID             int32  `json:"id"`
	QuestionID     int32  `json:"question_id"`
	QuestionType   string `json:"question_type"`
	Title          string `json:"title"`
	Context        string `json:"context"`
	Status         string `json:"status"`
	DeniedReason   string `json:"denied_reason"`
	CreatedIssueID string `json:"created_issue_id"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

type StaleCommentItem struct {
	ID          int32  `json:"id"`
	TargetType  string `json:"target_type"`
	TargetID    string `json:"target_id"`
	Author      string `json:"author"`
	AuthorAlias string `json:"author_alias,omitempty"`
	Body        string `json:"body"`
	CreatedAt   string `json:"created_at"`
	ParentID    *int32 `json:"parent_id,omitempty"`
}

type WriteTodoInput struct {
	Text       string `json:"text"`
	Key        string `json:"key"`
	Persona    string `json:"persona"`
	Priority   int32  `json:"priority"`
	Status     string `json:"status"`
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
	Kind       string `json:"kind"`
	IssueRef   string `json:"issue_ref"`
	Blocked    bool   `json:"blocked"`
	ClaimedBy  string `json:"claimed_by,omitempty"`
}

type DemoArtifactRef struct {
	DemoType     string `json:"demo_type"`
	ArtifactPath string `json:"artifact_path"`
}

type TestResultInput struct {
	Driver        string            `json:"driver"`
	TestName      string            `json:"test_name"`
	Feature       string            `json:"feature"`
	Status        string            `json:"status"`
	DurationMS    int32             `json:"duration_ms"`
	Branch        string            `json:"branch,omitempty"`
	GitSHA        string            `json:"git_sha,omitempty"`
	DemoArtifacts []DemoArtifactRef `json:"demo_artifacts,omitempty"`
}

// ── Shared input types ───────────────────────────────────────────────────

type PaginatedSlugInput struct {
	Slug   string `query:"slug" required:"true"`
	Limit  int32  `query:"limit"`
	Offset int32  `query:"offset"`
	Status string `query:"status"`
	Search string `query:"search"`
}

type IssueSlugInput struct {
	Slug string `query:"slug" required:"true"`
}
