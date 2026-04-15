package server

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgtype"
)

// ── ID conversion helpers ─────────────────────────────────────────────────

func intFromPrefixed(s, prefix string) int32 {
	if !strings.HasPrefix(s, prefix) {
		return 0
	}
	n, _ := strconv.ParseInt(s[len(prefix):], 10, 32)
	return int32(n)
}

func issueIntID(id string) int32 { return intFromPrefixed(id, "IS-") }
func taskIntID(id string) int32  { return intFromPrefixed(id, "TK-") }

func issueIDFromInt(n int32) string { return fmt.Sprintf("IS-%d", n) }
func taskIDFromInt(n int32) string  { return fmt.Sprintf("TK-%d", n) }

func fmtTS(ts pgtype.Timestamptz) string {
	if !ts.Valid {
		return ""
	}
	return ts.Time.UTC().Format(time.RFC3339)
}

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
}

type TaskItem struct {
	ID          int32  `json:"id" doc:"Server integer ID; CLI formats as TK-N"`
	Text        string `json:"text"`
	Feature     string `json:"feature"`
	Status      string `json:"status"`
	Reason      string `json:"reason"`
	IssueID     *int32 `json:"issue_id,omitempty" doc:"Linked issue integer ID; CLI formats as IS-N"`
	Depends     string `json:"depends"`
	TestPlan    string `json:"test_plan"`
	TestRefs    string `json:"test_refs"`
	TaskGroup   string `json:"task_group"`
	CreatedAt   string `json:"created_at"`
	CompletedAt string `json:"completed_at"`
	UpdatedAt   string `json:"updated_at"`
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
	ID          int32  `json:"id"`
	Description string `json:"description"`
	Kind        string `json:"kind"`
	Deferred    bool   `json:"deferred"`
}

type SpecTestItem struct {
	ID        int32  `json:"id"`
	Component string `json:"component"`
	Name      string `json:"name"`
	Layer     string `json:"layer"`
	Status    string `json:"status"`
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
	CreatedAt  string `json:"created_at"`
	ResolvedAt string `json:"resolved_at"`
}

type ErrorReportItem struct {
	ID         int64  `json:"id"`
	Source     string `json:"source"`
	Endpoint   string `json:"endpoint"`
	ErrorName  string `json:"error_name"`
	StackTrace string `json:"stack_trace"`
	CreatedAt  string `json:"created_at"`
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

type StaleCommentItem struct {
	ID         int32  `json:"id"`
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
	Author     string `json:"author"`
	Body       string `json:"body"`
	CreatedAt  string `json:"created_at"`
}

type WriteTodoInput struct {
	Text     string `json:"text"`
	Key      string `json:"key"`
	Persona  string `json:"persona"`
	Priority int32  `json:"priority"`
	Status   string `json:"status"`
}

type TestResultInput struct {
	Driver     string `json:"driver"`
	TestName   string `json:"test_name"`
	Feature    string `json:"feature"`
	Status     string `json:"status"`
	DurationMS int32  `json:"duration_ms"`
	Branch     string `json:"branch"`
	GitSHA     string `json:"git_sha"`
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

// ── Shared helpers ───────────────────────────────────────────────────────

// ptrStr dereferences a *string, returning "" for nil.
func ptrStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// ptrInt32 dereferences a *int32, returning 0 for nil.
func ptrInt32(n *int32) int32 {
	if n == nil {
		return 0
	}
	return *n
}

func parsePage(limit, offset int32) (int32, int32) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

// ── Route registration ─────────────────────────────────────────────────────

func (s *Server) registerRoutes(api huma.API) {
	s.registerAuthRoutes(api)
	s.registerIssueRoutes(api)
	s.registerTaskRoutes(api)
	s.registerFeatureRoutes(api)
	s.registerProjectRoutes(api)
	s.registerThemeRoutes(api)
	s.registerDxRoutes(api)
	s.registerErrorRoutes(api)
	s.registerCommentRoutes(api)
	s.registerAdminRoutes(api)
	s.registerCodeRefRoutes(api)
	s.registerQARoutes(api)
	s.registerClaudeRoutes(api)
	s.registerCounterRoutes(api)
	s.registerErrorEventRoutes(api)
	s.registerLogEventRoutes(api)
	s.registerAgentRoutes(api)
	s.registerFileRoutes()
}
