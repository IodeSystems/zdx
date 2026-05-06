package e2e

import (
	"fmt"
	"time"
)

// Scenario holds fixtures created during the "given" phase.
type Scenario struct {
	driver   Driver
	Issues   []int32
	Tasks    []int32
	Features []string
}

// ScenarioBuilder constructs test fixtures using the API driver (always fast).
type ScenarioBuilder struct {
	driver *ApiDriver
	sc     Scenario
}

func Given(driver *ApiDriver) *ScenarioBuilder {
	return &ScenarioBuilder{driver: driver, sc: Scenario{driver: driver}}
}

func (b *ScenarioBuilder) Issue(title, context string) *ScenarioBuilder {
	id := b.driver.AddIssue(title, context)
	b.sc.Issues = append(b.sc.Issues, id)
	return b
}

func (b *ScenarioBuilder) TriagedIssue(title, context string, priority int32) *ScenarioBuilder {
	id := b.driver.AddIssue(title, context)
	b.driver.TriageIssue(id, priority)
	b.sc.Issues = append(b.sc.Issues, id)
	return b
}

func (b *ScenarioBuilder) Task(issueIdx int, text string) *ScenarioBuilder {
	issueID := b.sc.Issues[issueIdx]
	id := b.driver.AddTask(issueID, text)
	b.sc.Tasks = append(b.sc.Tasks, id)
	return b
}

func (b *ScenarioBuilder) Feature(name, desc string) *ScenarioBuilder {
	b.driver.AddFeature(name, desc)
	b.sc.Features = append(b.sc.Features, name)
	return b
}

func (b *ScenarioBuilder) Spec(featureName, kind, desc string) *ScenarioBuilder {
	b.driver.AddSpec(featureName, kind, desc)
	return b
}

func (b *ScenarioBuilder) Goal(title string) *ScenarioBuilder {
	b.driver.AddGoal(title)
	return b
}

// Constraint was a project-constraint helper. zdx_project_constraints was
// dropped in IS-627 (migration 147), so this is a no-op kept for source
// compatibility with existing scenario chains. Remove call sites at leisure.
func (b *ScenarioBuilder) Constraint(_ string) *ScenarioBuilder {
	return b
}

func (b *ScenarioBuilder) Journal(role, date string) *ScenarioBuilder {
	b.driver.CheckinJournal(role, date)
	return b
}

func (b *ScenarioBuilder) Comment(targetType string, issueIdx int, body string) *ScenarioBuilder {
	targetID := fmt.Sprintf("IS-%d", b.sc.Issues[issueIdx])
	b.driver.AddComment(targetType, targetID, body)
	return b
}

func (b *ScenarioBuilder) FeatureComment(featureName, body string) *ScenarioBuilder {
	b.driver.AddComment("feature", featureName, body)
	return b
}

func (b *ScenarioBuilder) ReviewedFeature(name string) *ScenarioBuilder {
	b.driver.ReviewFeature(name)
	return b
}

// HealthPrereqs sets up goals + journals so solo doesn't gate on health
// checks. Constraints were dropped in IS-627 (migration 147 removed
// zdx_project_constraints + the /api/constraint endpoint); the synthetic
// owner:constraints health-check is also gone, so seeding constraints is
// unnecessary and would 404.
//
// Uses yesterday's date so the 7-day standup threshold never trips regardless
// of when tests run.
func (b *ScenarioBuilder) HealthPrereqs() *ScenarioBuilder {
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	b.driver.AddGoal("Test goal")
	b.driver.CheckinJournal("owner", yesterday)
	b.driver.CheckinJournal("tech", yesterday)
	return b
}

func (b *ScenarioBuilder) Build() Scenario {
	return b.sc
}
