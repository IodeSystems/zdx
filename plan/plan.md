# Multi-Agent Parallel Workflows + Project Doctor

## Done

- [x] SDLC model revamp (073–078): focus, goal metrics, feature hierarchy, plans, spec concern_type
- [x] Data migration (079): retire features, create goals, attribute
- [x] Solo queue maturity nudges (quantify-goal, attribute-feature, instrument-feature, decompose-feature)
- [x] Reservable todos (080): claim/release/renew with FOR UPDATE SKIP LOCKED
- [x] Agent config in .zdx/config.yaml (AgentConfig struct, compose_file, max_worktrees, llm_provider)
- [x] Agent loop refactored to claim-based (claim todo → session → renew lease → release)
- [x] `dx todo take` command
- [x] Goal/focus realignment (081), issue/task audit, queue clean
- [x] Ship compat-check fix for renamed tables

## Current: dx doctor

Doctor is the project health engine. `dx init` is a thin wrapper over doctor on a fresh project. Doctor diagnoses, fixes, and proposes until the project is healthy or the user defers all recommendations.

### Model

```
dx init    → scaffold .zdx/ → run doctor (first visit)
dx doctor  → diagnose → fix auto-fixable → propose the rest → loop until healthy or deferred
```

**Project classification** — first question doctor asks on a new project:
- library, tool, service, saas, site
- Classification shapes the maturity vine (which rungs, which checks)
- Stored in zdx_projects (new column: classification)

**Maturity vine** — fixed presets shipped in code (not DB):
- Each classification has ordered rungs (e.g. library: tests → docs → CI → versioning → published)
- Each rung has checks (automated or manual)
- Doctor evaluates checks against project state
- Checks are: auto-fixable / proposable / informational

**Check types:**
- `claude_installed` — is claude CLI in PATH?
- `docker_available` — is docker running?
- `local_llm_configured` — is llm_local set in config?
- `has_goals` — project has ≥1 goal?
- `goals_quantified` — all goals have metrics?
- `has_features` — project has ≥1 feature?
- `features_attributed` — all features have goal_id?
- `has_specs` — all features have ≥1 spec?
- `specs_tested` — all non-deferred specs have test refs?
- `has_ci` — CI pipeline configured?
- `has_deploy` — deploy pipeline configured?
- `agent_configured` — agent section in config?
- etc. (classification-specific checks)

**Deferred proposals:**
- User can defer any proposal
- Deferred checks don't nag until the project reaches the next rung where they become blocking
- Stored as zdx_doctor_deferrals (project_id, check_name, deferred_at, rung)

**Solo queue integration:**
- Doctor findings ARE the maturity nudges
- Solo reads doctor state, not its own hardcoded checks
- The existing maturity nudges in handlers_solo.go become doctor checks

### Implementation

#### Phase 1: Doctor core + classification

- [ ] Add `classification` column to zdx_projects (migration 082)
- [ ] Add `zdx_doctor_deferrals` table (migration 082)
- [ ] Define maturity vines in code: `internal/doctor/vines.go` — classification → []rung → []check
- [ ] Implement `internal/doctor/doctor.go` — run checks, return findings (auto-fix / propose / info / deferred)
- [ ] `dx doctor` CLI command — interactive: show findings, apply fixes, accept/defer proposals
- [ ] `dx init` calls doctor after scaffolding

#### Phase 2: Detection checks

- [ ] Claude detection (exec.LookPath)
- [ ] Docker detection (docker info)
- [ ] Local LLM detection (config.LLMLocal)
- [ ] Goal/feature/spec/test checks (query project state via API or DB)
- [ ] Agent config check (config.Agent)

#### Phase 3: Solo queue reads doctor

- [ ] Replace hardcoded maturity nudges in handlers_solo.go with doctor check evaluation
- [ ] Doctor findings surface as todo items with appropriate priority
- [ ] Deferred findings don't surface

#### Phase 4: Agent readiness (from previous plan)

- [ ] Doctor checks: agent prerequisites met for classification
  - library: just needs claude or local LLM
  - service/saas: needs docker for isolation
- [ ] `dx agent start` consults doctor before launching
- [ ] `dx agent init` scaffolds Dockerfile + compose from classification template
- [ ] Single worktree on bare host is safe; multi-worktree warns without compose

### Remaining from previous plan (later)

- [ ] Phase 2 (admin LLM config UI) — IS-232
- [ ] Phase 4 (docker dev image templates)
- [ ] Phase 5 (task group reservation)
- [ ] Phase 6 (coordination + observability)

### Start with

Phase 1: classification column + doctor core + vine definitions → `dx doctor` command.
