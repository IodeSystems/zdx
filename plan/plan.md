# SDLC Model Revamp

Realign zdx's data model to match the canonical conceptual model:

- **Goal**: outcome with measured metric (nullable, maturity-gradient push).
- **Feature**: demonstratable value driver. `direct` (deposits currency into a goal) or `multiplier` (amplifies other features; requires metric + target + instrumentation). Over-specced features signal decomposition → parent/child tree.
- **Spec**: concern on a feature. Has a `concern_type` (functional, latency, security, ux, compatibility). Kind stays (must/should/nice-to-have).
- **Focus** (was "theme"): prioritization lens / sprint. M:N with features. Any number of features attributed to any number of active focuses.
- **Plan**: first-class living object. Commentable, updatable, referencable by issues. Anchored to a focus, feature, or issue. Has ordered steps that can spawn issues/features/tasks.

---

## Phase 0: Data migration prep

Capture current prod data so we can script backfills safely.

- [ ] `bin/db dump` or pg_dump of zdx_themes, zdx_project_goals, zdx_features, zdx_specs, zdx_plans, zdx_theme_blockers, zdx_issue_features, zdx_goal_issues tables

---

## Phase 1: Schema migrations (073–078)

Each migration is one .up.sql + .down.sql pair. Order matters — later migrations reference earlier tables.

### 073_rename_theme_to_focus.up.sql
```sql
ALTER TABLE zdx_themes RENAME TO zdx_focuses;
ALTER TABLE zdx_theme_blockers RENAME TO zdx_focus_blockers;
ALTER TABLE zdx_focus_blockers RENAME COLUMN theme_id TO focus_id;
ALTER SEQUENCE zdx_themes_id_seq RENAME TO zdx_focuses_id_seq;
-- Add sprint-like fields
ALTER TABLE zdx_focuses ADD COLUMN started_at timestamptz;
ALTER TABLE zdx_focuses ADD COLUMN ended_at timestamptz;
```

### 074_goal_metrics.up.sql
```sql
ALTER TABLE zdx_project_goals ADD COLUMN metric_name text NOT NULL DEFAULT '';
ALTER TABLE zdx_project_goals ADD COLUMN metric_unit text NOT NULL DEFAULT '';
```

### 075_feature_hierarchy_and_attribution.up.sql
```sql
-- Feature tree
ALTER TABLE zdx_features ADD COLUMN parent_feature_id integer REFERENCES zdx_features(id);
-- Feature kind + goal attribution
ALTER TABLE zdx_features ADD COLUMN kind text NOT NULL DEFAULT 'direct';
ALTER TABLE zdx_features ADD COLUMN goal_id integer REFERENCES zdx_project_goals(id);
-- Multiplier links
CREATE TABLE zdx_feature_multipliers (
    feature_id integer NOT NULL REFERENCES zdx_features(id),
    multiplies_feature_id integer NOT NULL REFERENCES zdx_features(id),
    PRIMARY KEY (feature_id, multiplies_feature_id),
    CHECK (feature_id != multiplies_feature_id)
);
-- Feature metrics (for multiplier readiness gate)
ALTER TABLE zdx_features ADD COLUMN metric_name text NOT NULL DEFAULT '';
ALTER TABLE zdx_features ADD COLUMN metric_unit text NOT NULL DEFAULT '';
ALTER TABLE zdx_features ADD COLUMN baseline_value text NOT NULL DEFAULT '';
ALTER TABLE zdx_features ADD COLUMN target_value text NOT NULL DEFAULT '';
ALTER TABLE zdx_features ADD COLUMN graph_url text NOT NULL DEFAULT '';
```

### 076_focus_features.up.sql
```sql
-- M:N focus ↔ feature
CREATE TABLE zdx_focus_features (
    focus_id integer NOT NULL REFERENCES zdx_focuses(id),
    feature_id integer NOT NULL REFERENCES zdx_features(id),
    PRIMARY KEY (focus_id, feature_id)
);
-- Migrate existing theme_blockers (issue-based) — kept as zdx_focus_blockers
-- The new M:N is focus↔feature; blockers stay separate (focus blocked by issues)
```

### 077_plan_first_class.up.sql
```sql
-- Upgrade plans from feature-only to first-class objects
ALTER TABLE zdx_plans ADD COLUMN project_id integer;
ALTER TABLE zdx_plans ADD COLUMN title text NOT NULL DEFAULT '';
ALTER TABLE zdx_plans ADD COLUMN body text NOT NULL DEFAULT '';
ALTER TABLE zdx_plans ADD COLUMN focus_id integer REFERENCES zdx_focuses(id);
ALTER TABLE zdx_plans ADD COLUMN issue_id text;
-- Backfill project_id from feature
UPDATE zdx_plans p SET project_id = f.project_id FROM zdx_features f WHERE f.id = p.feature_id;
ALTER TABLE zdx_plans ALTER COLUMN project_id SET NOT NULL;
-- Make feature_id nullable (plans can anchor to focus or issue too)
ALTER TABLE zdx_plans ALTER COLUMN feature_id DROP NOT NULL;

-- Plan steps
CREATE TABLE zdx_plan_steps (
    id serial PRIMARY KEY,
    plan_id integer NOT NULL REFERENCES zdx_plans(id) ON DELETE CASCADE,
    seq integer NOT NULL DEFAULT 0,
    text text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'pending',
    depends_on integer REFERENCES zdx_plan_steps(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- Step → spawned refs (issues, features, tasks created from discovery)
CREATE TABLE zdx_plan_step_refs (
    step_id integer NOT NULL REFERENCES zdx_plan_steps(id) ON DELETE CASCADE,
    target_type text NOT NULL,
    target_id text NOT NULL,
    PRIMARY KEY (step_id, target_type, target_id)
);
```

### 078_spec_concern_type.up.sql
```sql
ALTER TABLE zdx_specs ADD COLUMN concern_type text NOT NULL DEFAULT 'functional';
```

---

## Phase 2: Queries (sqlc)

### Rename theme → focus
- [ ] `queries/themes.sql` → `queries/focuses.sql`
  - Rename all queries: ListThemes→ListFocuses, GetThemeByID→GetFocusByID, etc.
  - Update table names: zdx_themes→zdx_focuses, zdx_theme_blockers→zdx_focus_blockers
  - Add: ListFocusFeatures, AddFocusFeature, RemoveFocusFeature

### Goal metrics
- [ ] `queries/goals.sql` — update ListProjectGoals/GetProjectGoal/CreateProjectGoal to include metric_name, metric_unit

### Feature hierarchy + attribution
- [ ] `queries/features.sql`
  - Add parent_feature_id, kind, goal_id, metric fields to all SELECT/INSERT/UPDATE queries
  - Add: ListChildFeatures, GetFeatureTree, ListFeatureMultipliers, AddFeatureMultiplier, RemoveFeatureMultiplier
  - Add: ListFeaturesNeedingInstrumentation (multiplier features without baseline/target)
  - Add: ListUnattributedFeatures (no goal_id and not a child feature)

### Plans (new file)
- [ ] `queries/plans.sql` — new file
  - GetPlan, ListPlans (by project), ListPlansByFocus, ListPlansByFeature, ListPlansByIssue
  - CreatePlan, UpdatePlan, DeletePlan
  - CreatePlanStep, UpdatePlanStep, DeletePlanStep, ListPlanSteps, ReorderPlanSteps
  - CreatePlanStepRef, DeletePlanStepRef, ListPlanStepRefs
  - Move UpsertPlan/GetPlanByFeature from features.sql → plans.sql, update shape

### Spec concern_type
- [ ] `queries/features.sql` — add concern_type to all spec queries (ListSpecs, GetSpec, AddSpec, ListSpecsForProject, ListUncoveredSpecs)

### Regenerate
- [ ] `~/go/bin/sqlc generate`

---

## Phase 3: Server handlers

### Rename theme → focus
- [ ] `handlers_themes.go` → `handlers_focuses.go`
  - Rename all endpoints: /api/dx/themes → /api/dx/focuses, /api/themes → /api/focuses
  - Rename operation IDs: list-themes→list-focuses, etc.
  - Update huma structs to use Focus naming
  - Add: POST /api/dx/focuses/{id}/features (add feature to focus), DELETE (remove)
- [ ] `register.go` — update route registration
- [ ] `handlers_dx.go` — update any dx endpoints referencing themes

### Goal metrics
- [ ] `handlers_dx.go` or relevant handler — add metric_name, metric_unit to goal create/update/list responses

### Feature hierarchy + attribution
- [ ] `handlers_features.go`
  - Add parent_feature_id, kind, goal_id, metric fields to feature create/update/list endpoints
  - Add: GET /api/dx/features/{id}/children, POST /api/dx/features/{id}/multipliers

### Plans
- [ ] `handlers_plans.go` — new file
  - CRUD for plans (project-scoped, with optional focus_id/feature_id/issue_id anchor)
  - CRUD for plan steps (nested under plan)
  - Plan step refs (discovery spawns)
  - Plans are commentable → add 'plan' to target_type validation in comment handlers
  - Plans are revisionable → add 'plan' to revision tracking

### Spec concern_type
- [ ] `handlers_features.go` — add concern_type to spec create/update/detail responses

### OpenAPI spec
- [ ] Verify /openapi.json reflects all new endpoints (huma auto-generates)
- [ ] Dev server auto-regens ui/src/api.gen.ts

---

## Phase 4: CLI

### Rename theme → focus
- [ ] `internal/cli/project/theme.go` → `internal/cli/project/focus.go`
  - `dx focus list/add/status` (replacing `dx theme list/add/status`)
  - Add: `dx focus features add/remove FO-N F-name`
  - Add: `dx focus show FO-N` (features, plan, blockers)
- [ ] `internal/cli/work/todo.go` — update all theme references to focus
  - `--theme` flag → `--focus` flag
  - Solo prompt text: "active themes" → "active focuses"

### Goal metrics
- [ ] `internal/cli/project/goal.go` — add --metric-name, --metric-unit flags to `dx goal add`

### Feature hierarchy + attribution
- [ ] `internal/cli/project/feature.go`
  - Add --parent, --kind, --goal flags to `dx feature add`
  - Add --metric-name, --metric-unit, --baseline, --target, --graph-url flags to `dx feature set`
  - `dx feature show` displays parent/children, goal attribution, multiplier links, metric status

### Plans
- [ ] `internal/cli/project/plan.go` — new file
  - `dx plan add --focus FO-N / --feature F-name / --issue IS-N`
  - `dx plan show PL-N` (steps + comments + linked refs)
  - `dx plan step add/done/block/reorder PL-N`
  - `dx plan list` (project-scoped)
  - Plans are commentable: `dx comment PL-N "text"` (comment system already polymorphic)

### Spec concern_type
- [ ] `internal/cli/project/spec.go` — add --concern-type flag to spec add

---

## Phase 5: UI

### Rename theme → focus
- [ ] Rename route dir: `ui/src/routes/project/$slug/themes/` → `focuses/`
- [ ] Update all TSX files: theme→focus in labels, API calls, types
- [ ] Navigation: "Themes" → "Focuses" in sidebar/nav

### Goal metrics
- [ ] `goals/index.tsx` — show metric_name + metric_unit columns

### Feature hierarchy + attribution
- [ ] `features/index.tsx` — show parent/children tree view, goal badge, kind badge
- [ ] `features/$name.tsx` — detail page: multiplier links, metric status, instrumentation gate warning

### Plans
- [ ] `plans/` — new route directory
  - Plan list page (by project, filterable by focus/feature/issue)
  - Plan detail page: ordered steps, comments, step refs, discovery trail
  - Inline plan view on focus detail and feature detail pages
- [ ] Comment component already polymorphic — just add 'plan' target_type support

### Spec concern_type
- [ ] Spec detail: show concern_type badge
- [ ] Feature detail spec list: filterable by concern_type

### Progress rollup dashboards
- [ ] Focus detail: % features shipped, % specs passing, blocker count
- [ ] Goal detail: rollup of attributed features, metric progress
- [ ] Project maturity dashboard: % goals quantified, % multiplier features instrumented, % specs with tests

---

## Phase 6: Data migration (backfill existing data)

After schema + code changes land:

- [ ] Backfill `kind` on existing features (all default to 'direct' from migration)
- [ ] Attribute features to goals (manual review — file audit issues for unattributed)
- [ ] Retire surface-level features (dx journal cmd, dx plan cmd, dx feature cmd, dx theme cmd → fold specs into parent capabilities or delete if no specs)
- [ ] Decompose over-specced features (dx-todo 24 specs → split into queue-evaluation, task-claiming, owner-triage, dev-lifecycle, standup-cadence)
- [ ] Create initial focuses from existing themes (already renamed by migration)
- [ ] Set focus↔feature links based on existing theme_blockers issue links (resolve through issue→feature links)
- [ ] Convert existing zdx_plans (feature-only) to first-class plans (backfilled project_id already in migration)
- [ ] Backfill spec concern_type where obvious (default functional is fine for most)

---

## Phase 7: Solo queue maturity nudges

After everything above is working:

- [ ] "Quantify goal G-N" — goal active ≥14 days without metric_name
- [ ] "Instrument surface for F-N" — multiplier feature without baseline/target/graph_url
- [ ] "Attribute feature F-name" — feature with no goal_id and no parent
- [ ] "Decompose feature F-name" — feature with >8 non-deferred specs
- [ ] "Set concern-type on spec S-N" — spec with concern_type='functional' that looks non-functional (heuristic or manual)

---

## Execution order

Phases 1-2 are pure backend, can ship incrementally.
Phase 3-4 can run in parallel (server + CLI).
Phase 5 follows Phase 3 (needs API endpoints).
Phase 6 is manual/scripted after code lands.
Phase 7 is enhancement after core model is stable.

**Start with:** Phase 1 (migrations) → Phase 2 (sqlc) → verify `go build ./...` passes → proceed.
