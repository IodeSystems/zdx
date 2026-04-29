package handlers

// ── Response types ─────────────────────────────────────────────────────────

type IssueItem struct {
	ID              int32             `json:"id" doc:"Server integer ID; CLI formats as IS-N"`
	Title           string            `json:"title"`
	Status          string            `json:"status"`
	Priority        string            `json:"priority"`
	Component       string            `json:"component"`
	Features        string            `json:"features"`
	BlockedBy       []string          `json:"blocked_by"`
	BlockedByDetail []IssueBlockerRef `json:"blocked_by_detail"`
	Context         string            `json:"context"`
	Source          string            `json:"source"`
	IssueType       string            `json:"issue_type"`
	DuplicateOf     string            `json:"duplicate_of,omitempty"`
	LinkOf          string            `json:"link_of,omitempty"`
	ReopenCount     int32             `json:"reopen_count,omitempty" doc:"Number of times this issue has been reopened — a churn signal for stabilization candidates"`
	InteractiveOnly bool              `json:"interactive_only,omitempty"`
	TargetBranch    string            `json:"target_branch,omitempty"`
	URL             string            `json:"url"`
	CreatedAt       string            `json:"created_at"`
	UpdatedAt       string            `json:"updated_at"`
}

type IssueBlockerRef struct {
	ID     string `json:"id" doc:"Blocking issue ID formatted as IS-N"`
	Status string `json:"status" doc:"Current status of the blocking issue (open, closed, ...)"`
}

type TaskItem struct {
	ID             int32  `json:"id" doc:"Server integer ID; CLI formats as TK-N"`
	Title          string `json:"title"`
	Text           string `json:"text"`
	Feature        string `json:"feature"`
	Status         string `json:"status"`
	Reason         string `json:"reason"`
	IssueID        *int32 `json:"issue_id,omitempty" doc:"Linked issue integer ID; CLI formats as IS-N"`
	Spec           string `json:"spec,omitempty" doc:"Linked spec integer ID"`
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

type TaskReviewItem struct {
	ID            int32  `json:"id"`
	TaskID        string `json:"task_id"`
	ReviewerRole  string `json:"reviewer_role"`
	ReviewerEmail string `json:"reviewer_email,omitempty"`
	Verdict       string `json:"verdict"`
	Body          string `json:"body"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
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
	PlanStatus      string     `json:"plan_status"`
	HasTestRefs     bool       `json:"has_test_refs"`
	Specs           []SpecItem `json:"specs"`
}

type SpecItem struct {
	ID          int32  `json:"id"`
	Description string `json:"description"`
	Importance  string `json:"importance"`
}

type SpecIssueItem struct {
	SpecID  int32  `json:"spec_id"`
	IssueID string `json:"issue_id"`
	Title   string `json:"title"`
	Status  string `json:"status"`
}

type SpecDeferralItem struct {
	IssueID     string `json:"issue_id"`
	IssueTitle  string `json:"issue_title"`
	IssueStatus string `json:"issue_status"`
	Note        string `json:"note"`
}

type DeferredSpecItem struct {
	ID          int32              `json:"id"`
	FeatureID   int32              `json:"feature_id"`
	FeatureName string             `json:"feature_name"`
	Description string             `json:"description"`
	Importance  string             `json:"importance"`
	Blockers    []SpecDeferralItem `json:"blockers"`
}

type SpecTestItem struct {
	ID        int32  `json:"id"`
	Component string `json:"component"`
	Name      string `json:"name"`
	Layer     string `json:"layer"`
	Status    string `json:"status"`
}

type SpecCloseGateOffender struct {
	SpecID      int32  `json:"spec_id"`
	Description string `json:"description"`
	Feature     string `json:"feature"`
	Reason      string `json:"reason"`
}

type SpecTaskItem struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

type SpecDemoItem struct {
	ID            int32  `json:"id"`
	TestID        int32  `json:"test_id"`
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
	Importance  string `json:"importance"`
}

type FocusItem struct {
	ID           int32              `json:"id"`
	Name         string             `json:"name"`
	Description  string             `json:"description"`
	Priority     int32              `json:"priority"`
	Status       string             `json:"status"`
	Attributions []FocusAttribution `json:"attributions"`
	StartedAt    string             `json:"started_at"`
	EndedAt      string             `json:"ended_at"`
	CreatedAt    string             `json:"created_at"`
}

type FocusAttribution struct {
	ID            string `json:"id" doc:"Issue ID formatted as IS-N"`
	Status        string `json:"status" doc:"Current status of the attributed issue"`
	Justification string `json:"justification" doc:"Why this issue is attributed to the focus"`
}

type TodoItem struct {
	ID               int32  `json:"id"`
	Text             string `json:"text"`
	Title            string `json:"title,omitempty"`
	Description      string `json:"description,omitempty"`
	Key              string `json:"key"`
	Persona          string `json:"persona"`
	Priority         int32  `json:"priority"`
	Status           string `json:"status"`
	TargetType       string `json:"target_type"`
	TargetID         string `json:"target_id"`
	Kind             string `json:"kind"`
	IssueRef         string `json:"issue_ref"`
	TargetBranch     string `json:"target_branch,omitempty"`
	ProjectSlug      string `json:"project_slug,omitempty"`
	Blocked          bool   `json:"blocked"`
	BlockedReason    string `json:"blocked_reason,omitempty"`
	CycleCount       int32  `json:"cycle_count,omitempty"`
	ReferenceIssueID string `json:"reference_issue_id,omitempty"`
	Instructions     string `json:"instructions,omitempty"`
	SuggestedAction  string `json:"suggested_action,omitempty"`
	ClaimedBy        string `json:"claimed_by,omitempty"`
	ClaimedAt        string `json:"claimed_at,omitempty"`
	CreatedAt        string `json:"created_at"`
	ResolvedAt       string `json:"resolved_at,omitempty"`
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

type PlanItem struct {
	ID         int32          `json:"id"`
	ProjectID  int32          `json:"project_id"`
	Title      string         `json:"title"`
	Body       string         `json:"body"`
	PlanType   string         `json:"plan_type"`
	Status     string         `json:"status"`
	Complexity string         `json:"complexity"`
	Approach   string         `json:"approach"`
	FeatureID  int32          `json:"feature_id"`
	FocusID    int32          `json:"focus_id"`
	IssueID    string         `json:"issue_id"`
	CreatedAt  string         `json:"created_at"`
	UpdatedAt  string         `json:"updated_at"`
	Steps      []PlanStepItem `json:"steps,omitempty"`
}

type PlanStepItem struct {
	ID        int32             `json:"id"`
	PlanID    int32             `json:"plan_id"`
	Seq       int32             `json:"seq"`
	Text      string            `json:"text"`
	Status    string            `json:"status"`
	DependsOn int32             `json:"depends_on"`
	CreatedAt string            `json:"created_at"`
	UpdatedAt string            `json:"updated_at"`
	Refs      []PlanStepRefItem `json:"refs,omitempty"`
}

type PlanStepRefItem struct {
	StepID     int32  `json:"step_id"`
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
}

type OKBody struct {
	OK bool `json:"ok"`
}

type ReservationItem struct {
	ID              int64  `json:"id"`
	TargetType      string `json:"target_type"`
	TargetID        string `json:"target_id"`
	ClaimedBy       string `json:"claimed_by"`
	ClaimedAt       string `json:"claimed_at"`
	ReleasedAt      string `json:"released_at"`
	LeaseExpiresAt  string `json:"lease_expires_at"`
	TodoText        string `json:"todo_text,omitempty"`
	SessionID       int64  `json:"session_id,omitempty"`
	SessionStatus   string `json:"session_status,omitempty"`
	SessionClosedAt string `json:"session_closed_at,omitempty"`
	SessionHeader   string `json:"session_header,omitempty"`
	SessionAlias    string `json:"session_alias,omitempty"`
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
	Title  string  `json:"title"`
	Text   string  `json:"text"`
	Status string  `json:"status"`
	Reason string  `json:"reason"`
	Issue  string  `json:"issue"`
	Score  float32 `json:"score"`
}

type SimilarQuestionItem struct {
	ID       int32   `json:"id"`
	Question string  `json:"question"`
	Answer   string  `json:"answer"`
	Score    float32 `json:"score"`
}

type SearchFeatureItem struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Kind        string `json:"kind"`
}

type SearchFocusItem struct {
	ID          int32  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
}

type SimilarFeatureItem struct {
	ID          int32   `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Category    string  `json:"category"`
	Kind        string  `json:"kind"`
	Score       float32 `json:"score"`
}

type SimilarSpecItem struct {
	ID          int32   `json:"id"`
	FeatureID   int32   `json:"feature_id"`
	FeatureName string  `json:"feature_name"`
	Description string  `json:"description"`
	Importance  string  `json:"importance"`
	Score       float32 `json:"score"`
}

type SimilarProposalItem struct {
	ID     int32   `json:"id"`
	Title  string  `json:"title"`
	Body   string  `json:"body"`
	Status string  `json:"status"`
	Score  float32 `json:"score"`
}

type ProposalDedupGroup struct {
	Proposal       ProposalItem          `json:"proposal"`
	Duplicates     []SimilarProposalItem `json:"duplicates"`
	ExistingIssues []SimilarIssueItem    `json:"existing_issues"`
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
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	Text        string `json:"text"`
	Key         string `json:"key"`
	Persona     string `json:"persona"`
	Priority    int32  `json:"priority"`
	Status      string `json:"status"`
	TargetType  string `json:"target_type"`
	TargetID    string `json:"target_id"`
	Kind        string `json:"kind"`
	IssueRef    string `json:"issue_ref"`
	Blocked     bool   `json:"blocked"`
	ClaimedBy   string `json:"claimed_by,omitempty"`
}

type DemoArtifactRef struct {
	DemoType     string `json:"demo_type"`
	ArtifactPath string `json:"artifact_path"`
}

type TestResultInput struct {
	Driver        string            `json:"driver"`
	TestName      string            `json:"test_name"`
	Feature       string            `json:"feature"`
	Layer         string            `json:"layer,omitempty"`
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
