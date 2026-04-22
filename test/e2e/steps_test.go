package e2e

// Step interfaces define domain operations. Each has an Api implementation (fast, HTTP-based)
// and optionally a Ui implementation (browser-based, delegates to Api for setup).

type IssueInfo struct {
	ID       int32  `json:"id"`
	Status   string `json:"status"`
	Priority string `json:"priority"`
}

type TaskInfo struct {
	ID          int32  `json:"id"`
	Text        string `json:"text"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
	CompletedAt string `json:"completed_at"`
}

type FeatureInfo struct {
	Name string `json:"name"`
}

type SpecInfo struct {
	ID          int32  `json:"id"`
	FeatureName string `json:"feature_name"`
}

type BlockerQuestionInfo struct {
	ID         int32  `json:"id"`
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
	Status     string `json:"status"`
}

type QAQuestionInfo struct {
	ID       int32  `json:"id"`
	Question string `json:"question"`
}

type StaleCommentInfo struct {
	ID         int32  `json:"id"`
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
	Author     string `json:"author"`
}

type ProjectSteps interface {
	CreateProject(slug, name string)
}

type IssueSteps interface {
	AddIssue(title, context string) int32
	TriageIssue(id, priority int32)
	CloseIssue(id int32)
	ListIssues() []IssueInfo
}

type TaskSteps interface {
	AddTask(issueID int32, text string) int32
	MarkTaskDone(taskID int32)
	ListTasks(issueID int32) []TaskInfo
}

type FeatureSteps interface {
	AddFeature(name, desc string) int32
	AddSpec(feature, kind, desc string)
	DeferSpec(specID int32)
	ReviewFeature(name string)
	ListFeatures() []FeatureInfo
	ListUncoveredSpecs() []SpecInfo
	ListStaleFeatures() []FeatureInfo
	LinkTestToSpec(specID, testID int32)
}

type CommentSteps interface {
	AddComment(targetType, targetID, body string)
	MarkCommentsRead(targetType, targetID, role string)
	HasUnreadComments(targetType, targetID string) bool
	ListStaleUnreadComments(ageHours int) []StaleCommentInfo
}

type BlockerQuestionSteps interface {
	AddBlockerQuestion(targetType, targetID, context string) int32
	AnswerBlockerQuestion(id int32, answer string)
	ListPendingBlockerQuestions() []BlockerQuestionInfo
}

type QASteps interface {
	AddQuestion(category, question string) int32
	AnswerQuestion(id int32, answer string)
	ListUnansweredQuestions() []QAQuestionInfo
}

type JournalSteps interface {
	CheckinJournal(role, date string)
}

type TestSteps interface {
	RegisterTest(name string) int32
}

type HealthSteps interface {
	GetHealth() (goalCount, constraintCount, closedTaskCount int64, ownerJournalDate, techJournalDate string)
}

type GoalSteps interface {
	AddGoal(title string)
}

type ConstraintSteps interface {
	AddConstraint(title string)
}

// Driver composes all step interfaces.
type Driver interface {
	ProjectSteps
	IssueSteps
	TaskSteps
	FeatureSteps
	CommentSteps
	BlockerQuestionSteps
	QASteps
	JournalSteps
	TestSteps
	HealthSteps
	GoalSteps
	ConstraintSteps
}
