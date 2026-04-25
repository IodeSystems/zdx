# zdx Product Restructuring Plan

## Vision

zdx: Principled SDLC for everyone, augmented by agents.

zdx is an ad-hoc SDLC platform that gives any developer — solo or on a team —
product management superpowers without requiring a PM background. It provides a
structured workflow (goals -> features -> specs -> issues -> tasks) that can be
adopted incrementally: start with `dx init` and an issue, grow into quantified
goals and maturity tracking as the project demands it. The CLI is the primary
interface for both humans and LLM agents. Agents participate as first-class
collaborators through a safe execution harness (docker + worktrees) that lets
them claim work, implement changes, and surface decisions for human review —
without risking the host system. The web UI provides visibility and owner-level
controls. Self-hosted, no vendor lock-in.

## New Goals

| # | Goal | Outcome | Metric |
|---|------|---------|--------|
| 1 | Agents ship real work safely | Agents claim todos, work in isolated environments, deliver tested branches | agent_pr_merge_rate (percent) |
| 2 | Humans stay in control | Owners triage, review, unblock with minimal effort. System surfaces decisions, not busywork | owner_decision_latency (minutes) |
| 3 | Anyone can be a PM | Developers adopt structured SDLC incrementally — the CLI teaches the workflow | onboarding_time_to_first_value (minutes) |
| 4 | Projects improve over time | Doctor + maturity vines nudge projects toward health without gating progress | projects_at_maturity_rung_3 (percent) |
| 5 | Platform runs itself | Deploy, observe, administer. Self-hosted operators need zero vendor involvement | api_uptime (percent) |

## Feature Mapping

Reparent existing features to new goals, consolidate where obvious.

### Goal 1: Agents ship real work safely
- dx-todo (keep, reparent from G-9)
  - dx-todo-queue (reparent)
  - dx-todo-persistence (reparent)
  - dx-todo-agent (reparent)
- (NEW) agent-harness — safe execution environment: docker containers, worktree isolation, resource limits, sandboxed shell
- Consolidate 6 test/coverage features into 1-2 (verification infrastructure)

### Goal 2: Humans stay in control
- blocker-questions + cli + ui children (reparent from G-9)
- dx-todo-tasks (reparent — it's the human lifecycle: triage, review, done/undone)

### Goal 3: Anyone can be a PM
- (NEW) sdlc-workflow — the goal/feature/spec/issue/task hierarchy, CLI commands, incremental adoption
- (NEW) onboarding — dx init, dx doctor --fix, bootstrap guidance

### Goal 4: Projects improve over time
- project maturity guidance (reparent from G-8)
- (NEW) doctor-maturity-vines — classification-specific health checks and auto-fix

### Goal 5: Platform runs itself
- admin-llm-config (reparent from G-12)
- admin-project-management (reparent from G-12)
- stable deployments (reparent from G-12)

## Deprecations

### MCP tool server (dx mcp)
The current MCP server mirrors the CLI with a separate code path that drifts.
Agents with shell access can call `dx` directly. Plan:
1. Stop adding features/specs to MCP tools
2. Migrate agent loop to use CLI calls instead of MCP tool calls
3. Remove MCP server once nothing depends on it

### Old goals to archive
Once features are reparented:
- G-9 Collaborative Dev -> archived
- G-11 Correctness -> archived
- G-10 Experience -> archived
- G-7 Install & Distribution -> archived
- G-8 Planning & Maturity -> archived
- G-12 Platform Operations -> archived
- G-13 Observability -> folded into G-5 (Platform runs itself)

## Execution Order

1. ~~**Write vision**~~ — done (in this file)
2. ~~**Create new goals**~~ — done: G-14 through G-18
3. ~~**Reparent features**~~ — done: all 19 existing features reparented, categories updated
4. ~~**Archive old goals**~~ — done: G-7 through G-13 archived
5. ~~**Create missing features**~~ — done: agent-harness, sdlc-workflow, onboarding, doctor-maturity-vines
6. ~~**Consolidate test features**~~ — done: 7 → 3 (dx test unified absorbed 4, kept demo recordings + feature test layers)
7. **Deprecate MCP server** — IS-465: migrate agent loop to CLI calls, then remove dx mcp
8. ~~**Project vision model**~~ — done: migration 106 adds title+description to zdx_projects, API endpoint, CLI `dx vision show/set`, specs on sdlc-workflow + onboarding
9. ~~**Set zdx vision on production**~~ — done
10. ~~**Wire vision into doctor/agent context**~~ — done: has_vision doctor check in identity rung, agent sessions get vision in prompt
