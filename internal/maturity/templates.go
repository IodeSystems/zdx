package maturity

// ItemTemplate describes a maturity item to stamp when a question is answered.
type ItemTemplate struct {
	Kind         string
	TargetType   string
	Title        string
	Description  string
	PriorityHint int32
}

// Templates maps question_key → answer → list of item templates to stamp.
// An empty answer key "" means "stamp regardless of answer" (baseline items).
var Templates = map[string]map[string][]ItemTemplate{
	// Baseline items: stamped for every project on first contact.
	"baseline": {
		"yes": {
			{
				Kind:         "owner:attribute-feature",
				Title:        "Attribute all features to a goal or parent feature",
				Description:  "Unattributed features make it harder to prioritize. Link each feature to a goal or parent.",
				PriorityHint: 34,
			},
			{
				Kind:         "owner:quantify-goal",
				Title:        "Quantify all goals with metrics",
				Description:  "Goals without metrics cannot be measured. Add metric_name and metric_unit to each goal.",
				PriorityHint: 33,
			},
			{
				Kind:         "tech:instrument-feature",
				Title:        "Instrument all multiplier features",
				Description:  "Multiplier features need baseline, target, and graph_url to track impact.",
				PriorityHint: 34,
			},
			{
				Kind:         "tech:decompose-feature",
				Title:        "Decompose over-specced features",
				Description:  "Features with >8 specs are too broad. Break them into focused child features.",
				PriorityHint: 35,
			},
		},
	},
	"stores-data": {
		"yes": {
			{
				Kind:         "tech:backup",
				Title:        "Define and test a backup strategy",
				Description:  "Verify backups are scheduled, tested for restore, and cover all data stores.",
				PriorityHint: 50,
			},
			{
				Kind:         "tech:retention-policy",
				Title:        "Document data retention policy",
				Description:  "Define how long data is kept and when it is purged or archived.",
				PriorityHint: 55,
			},
		},
	},
	"has-ui": {
		"yes": {
			{
				Kind:         "tech:a11y",
				Title:        "Audit UI for accessibility (a11y)",
				Description:  "Run an automated a11y scan and resolve critical violations.",
				PriorityHint: 60,
			},
			{
				Kind:         "tech:mobile-responsive",
				Title:        "Verify mobile-responsive layout",
				Description:  "Test key pages at mobile viewport widths; fix layout breaks.",
				PriorityHint: 65,
			},
		},
	},
	"is-saas": {
		"yes": {
			{
				Kind:         "tech:tenant-isolation",
				Title:        "Verify tenant data isolation",
				Description:  "Confirm no query can cross tenant boundaries; add tests if missing.",
				PriorityHint: 20,
			},
			{
				Kind:         "tech:usage-billing",
				Title:        "Wire usage metrics to billing",
				Description:  "Ensure billable events are tracked and reconciled against invoices.",
				PriorityHint: 25,
			},
		},
	},
	"handles-pii": {
		"yes": {
			{
				Kind:         "tech:pii-audit",
				Title:        "Audit PII storage and access logging",
				Description:  "Identify all PII fields; ensure access is logged and retention is bounded.",
				PriorityHint: 15,
			},
			{
				Kind:         "tech:data-deletion",
				Title:        "Implement right-to-deletion (GDPR)",
				Description:  "Add a tested code path to fully delete a user's PII on request.",
				PriorityHint: 18,
			},
		},
	},
	"exposes-api": {
		"yes": {
			{
				Kind:         "tech:api-versioning",
				Title:        "Define API versioning strategy",
				Description:  "Document how breaking changes are signaled and how old versions sunset.",
				PriorityHint: 45,
			},
			{
				Kind:         "tech:api-rate-limiting",
				Title:        "Implement API rate limiting",
				Description:  "Add per-client rate limits to prevent abuse and protect capacity.",
				PriorityHint: 48,
			},
		},
	},
	"multi-region": {
		"yes": {
			{
				Kind:         "tech:data-replication",
				Title:        "Verify cross-region data replication",
				Description:  "Confirm replication lag SLA; add alerting if lag exceeds threshold.",
				PriorityHint: 40,
			},
			{
				Kind:         "tech:region-failover",
				Title:        "Test region failover runbook",
				Description:  "Execute a failover drill; document recovery time and any manual steps.",
				PriorityHint: 42,
			},
		},
	},
	"background-jobs": {
		"yes": {
			{
				Kind:         "tech:job-observability",
				Title:        "Add observability to background jobs",
				Description:  "Ensure each job emits duration, error rate, and queue-depth metrics.",
				PriorityHint: 55,
			},
			{
				Kind:         "tech:job-retry-policy",
				Title:        "Define retry and dead-letter policy for jobs",
				Description:  "Document max retries, backoff strategy, and DLQ handling for each job type.",
				PriorityHint: 58,
			},
		},
	},
}
