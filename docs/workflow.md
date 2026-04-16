# Solo Workflow State Machine

`dx todo solo` is a priority-ordered state machine that returns the single highest-priority actionable item for an agent. Each invocation returns at most one todo; the agent acts on it and re-runs solo to get the next.

## Precedence Order

Solo checks conditions in this exact order. First match wins.

| # | Type | Scope | Trigger |
|---|------|-------|---------|
| 0a | `[read:comments]` | issue | Unread LLM comments on any open issue |
| 0b | `[read:comments]` | feature | Unread LLM comments on any feature |
| 0c | `[clarify]` | issue (--issue only) | Pending blocker-question on the scoped issue |
| 0d | `[bootstrap]` | global only | Zero issues AND zero features in project |
| 0e | `[owner:goals]` | global only | Project has no goals defined |
| 0f | `[owner:constraints]` | global only | Project has no constraints defined |
| 0g | `[owner:standup]` | global only | Owner standup check-in overdue |
| 0h | `[tech:standup]` | global only | Tech standup check-in overdue |
| 1 | `[triage]` | both | Open issue with no priority set |
| 1b | `[owner:spec]` | global only | Feature with zero specs |
| 1c | `[owner:review]` | global only | Feature not reviewed in >30 days (or never) |
| 2a | `[tech:test-ref]` | global only | Spec with no linked test refs |
| 2 | `[add]` | both | Open triaged issue with no ready/active tasks (and has no completed tasks) |
| 2* | `[closable]` | both | Open triaged issue with all tasks done (has completed tasks, none ready) |
| 3 | `[dev]` | both | Open issue with a ready task |
| - | `nothing to do` | both | No actionable items found |

## Scoping Rules

- **Global mode** (`dx todo solo`): Checks all open issues and all cross-cutting concerns (health, specs, reviews, test refs). Issues with pending blocker-questions are skipped silently.
- **Issue mode** (`dx todo solo --issue=IS-N`): Restricts to the single issue. Skips cross-cutting checks (0d-0h, 1b, 1c, 2a). Blocker-questions surface as `[clarify]` instead of silently skipping.
- **Agent mode** (`dx todo solo --agent-id=AGT`): Like global/issue mode but uses atomic task claiming via the ClaimTask API instead of iterating tasks.

## Todo Types Reference

### `[bootstrap]`
- **Trigger**: No issues and no features exist in the project.
- **Agent action**: Scan codebase, create features via `dx feature add`, create initial setup issue via `dx issue add`.
- **Advances when**: At least one issue or feature exists.

### `[read:comments]`
- **Trigger**: Unread comments (for `llm` role) on an open issue or feature.
- **Agent action**: Read and respond to comments via `dx comment add`. Solo auto-marks comments as read after displaying them.
- **Advances when**: Comments are marked read.

### `[clarify]`
- **Trigger**: Pending blocker-question on the scoped issue (--issue mode only).
- **Agent action**: Answer the question via `dx question answer <BQ-ID> --answer="..."`.
- **Advances when**: All blocker-questions for the issue are answered.

### `[owner:goals]`
- **Trigger**: Project has zero goals.
- **Agent action**: Define goals via `dx goal add <title>`.
- **Advances when**: At least one goal exists.

### `[owner:constraints]`
- **Trigger**: Project has zero constraints (goals must already exist).
- **Agent action**: Define constraints via `dx constraint add <title>`.
- **Advances when**: At least one constraint exists.

### `[owner:standup]` / `[tech:standup]`
- **Trigger**: Standup check-in is overdue based on cadence (30 days / max(1, closed_tasks/10), clamped to min 7 days). Owner is checked before tech.
- **Agent action**: Run `dx standup checkin --owner` or `dx standup checkin --tech`.
- **Advances when**: Standup entry is recorded with a recent date.

### `[triage]`
- **Trigger**: Open issue with no priority set.
- **Agent action**: Verify the issue independently, dup-check, rewrite title/context prescriptively, then apply via `dx todo owner triage IS-N --title=... --context=... --type=<ops|impl> --priority=<1-4>`.
- **Advances when**: Issue has a priority value.

### `[owner:spec]`
- **Trigger**: Feature with zero specs defined.
- **Agent action**: Add specs via `dx feature spec add <feature-name>`.
- **Advances when**: Feature has at least one spec.

### `[owner:review]`
- **Trigger**: Feature with `last_reviewed_at` NULL or older than 30 days.
- **Agent action**: Review the feature via `dx feature review <feature-name>`.
- **Advances when**: Feature `last_reviewed_at` is updated.

### `[tech:test-ref]`
- **Trigger**: Spec with no linked test references (non-deferred specs only).
- **Agent action**: Link a test via `dx spec link` or defer the spec.
- **Advances when**: Spec has at least one test ref linked, or is deferred.

### `[add]`
- **Trigger**: Open triaged issue with no tasks (ready or active). Issue has zero completed tasks.
- **Agent action**: Decompose the issue into tasks via `dx todo tech add --issue=IS-N --text="..."`.
- **Advances when**: At least one task exists for the issue.

### `[closable]`
- **Trigger**: Open triaged issue where all tasks are done (has completed tasks, none ready/active).
- **Agent action**: Close the issue via `dx issue close IS-N --reason=done`.
- **Advances when**: Issue status is closed.

### `[dev]`
- **Trigger**: Open issue with at least one ready task.
- **Agent action**: Work on the task, then mark done via `dx todo dev done TK-N`.
- **Advances when**: Task status changes from ready to done.

## Journal Cadence Formula

```
cadence_days = 30 / max(1, closed_tasks / 10)
cadence_days = max(7, cadence_days)
```

More closed tasks → more frequent standup check-ins. Minimum cadence is 7 days. With no closed tasks, cadence is 30 days.

## Blocker-Question Behavior

- **Global mode**: Issues with pending blocker-questions are filtered out entirely. Solo picks other work.
- **Issue mode**: Pending blocker-questions surface as `[clarify]` and block all other work on that issue until answered.
