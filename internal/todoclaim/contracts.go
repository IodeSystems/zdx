package todoclaim

// Contract describes which agent persona handles a todo kind and how
// the claim context (base SHA, base branch) should be surfaced.
type Contract struct {
	// Name is the persona bucket: Metadata, Dev, or Ask.
	Name string
}

const (
	Metadata = "metadata"
	Dev      = "dev"
	Ask      = "ask"
)

// ByKind maps every known todo kind to its Contract.
// Unknown kinds default to Metadata.
var ByKind = map[string]Contract{
	"product:triage":         {Metadata},
	"add":                    {Metadata},
	"tech:decompose-tracker": {Metadata},
	"owner:spec":             {Metadata},
	"owner:review":           {Metadata},
	"owner:goals":            {Metadata},
	"owner:constraints":      {Metadata},
	"owner:standup":          {Metadata},
	"owner:orphan-task":      {Metadata},
	"owner:demo-gap":         {Metadata},
	"closable":               {Metadata},
	"close:tracker":          {Metadata},
	"review:deferred-spec":   {Metadata},
	// IS-1090 prep (TK-1760): contract surface only — sibling children own emission.
	"reviewer:review-decomposition": {Metadata},
	"reviewer:review-impl":          {Metadata},
	"product:goal-recalibrate":      {Metadata},
	"tech:resolve-disjoint":         {Metadata},
	"dev":                           {Dev},
	"tech:standup":                  {Dev},
	"tech:test-ref":                 {Dev},
	"clarify":                       {Ask},
	"answer":                        {Ask},
	"respond:discussion":            {Ask},
	"read:comments":                 {Ask},
	"respond:stale":                 {Ask},
}

// ForKind returns the Contract for the given kind, defaulting to Metadata.
func ForKind(kind string) Contract {
	if c, ok := ByKind[kind]; ok {
		return c
	}
	return Contract{Metadata}
}
