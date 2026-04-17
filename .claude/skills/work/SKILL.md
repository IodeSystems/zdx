---
name: work
description: Work a single issue vertical (owner triage → tech plan → dev done). Pass an issue ID or omit to let todo take claim one. One vertical per session.
disable-model-invocation: true
argument-hint: "[IS-N]"
---

Drive `./bin/dx todo` in remote mode. Issue: $ARGUMENTS (optional). **One vertical per invocation — stop after the first
issue is closed and shipped.**

**Entry:**

- No issue given: run `./bin/dx todo take` — atomically claim the next todo item. If the claimed todo targets a task
  (TK-N), run `./bin/dx todo show TK-N` to find its parent issue; use that issue for the vertical. If the todo targets
  an issue directly, use that issue.
- Issue given: proceed directly to the vertical loop.

**Vertical loop** for IS-N:

1. `./bin/dx todo solo --issue=IS-N`
2. Do whatever the pick says (owner triage → tech plan → tech add → dev done)
3. Repeat until solo prints nothing to do
4. `./bin/dx issue close IS-N --reason=done`
5. If the vertical produced shippable fixes/features (server, UI, schema, queries, or any code that runs in prod):
    - Commit all changes first: `git add <files> && git commit -m '...'`
    - If schema (internal/migrate/sql/) or queries (queries/*.sql) changed: `~/go/bin/sqlc generate` then
      `go build ./...` to verify
    - `bin/ship` (never `--allow-dirty` during normal dev — that is an emergency-only escape hatch)
    - Note: migrations run automatically on dev server restart; prod migrations run via ship
    - Skip ship for docs-only / skill-only / planning-only changes — just commit.
6. **Stop. Report what was done. Do not pick up another vertical.**

Vertical scope is automatic: solo --issue=IS-N picks only triage:IS-N, decompose:IS-N, plan: for linked features, and
dev tasks with task.issue == IS-N.

**Bootstrap** (when solo emits `[bootstrap] <slug>`):

This means the project has zero issues and zero features — it's brand new. Run `dx doctor --fix` first to classify
the project and scaffold it, then follow the printed guidance:

1. **Scan the codebase** thoroughly — read directory trees, entry points, configs, schema, routes, UI structure.
2. **Create features** for each conceptual capability you discover. Use `dx feature add <name> --desc="..."` for each.
   Set `--kind=direct` or `--kind=multiplier` and link to a goal with `dx feature set <name> --goal <id>`.
3. **Create a setup issue** for zdx integration: close-hooks, component config, and verifying the solo cycle.
4. **Re-run `dx todo solo`** — the normal triage flow will now engage on the setup issue.
5. Continue the vertical loop from there.

**Triage** (when solo emits `[triage] IS-N`):

1. **Verify independently.** Reproduce or read the relevant code/UI before accepting the report at face value.
2. **Dup-check.** `./bin/dx issue list` and scan open + recent closed issues for similar work. If a close match exists,
   close the new one as duplicate (`--reason=duplicate --duplicate-of=IS-X`).
3. **Rewrite prescriptively.** Title = intended outcome (not symptom). Context covers: (a) what *should* happen, (b)
   what *did* happen, (c) implementation direction if known.
4. **Apply** via `./bin/dx todo owner triage IS-N --title=... --context=... --type=<ops|impl|ask> --priority=<1-4> --focus=<FO-N> --goal=<G-N>`.


**Comments** (when solo emits `[read:comments] IS-N` or `[read:comments] <feature-name>`):

Solo surfaces unread comments that need a response. After reading the comments:

1. **Understand the comment.** Read the context — is it a question, a request for clarification, feedback, or a decision?
2. **Reply.** Post a response: `./bin/dx comment add <target-type> <target-id> --body="<your reply>"`.
   - target-type is `issue`, `task`, or `feature` depending on what solo showed.
   - Answer questions, acknowledge feedback, or explain decisions.
   - Author alias: the dx CLI auto-tags comments with `$DX_AUTHOR_ALIAS` (pre-set by the agent harness, typically `claude`). Pass `--as <alias>` only to override.
3. **Mark read.** Solo marks comments read automatically after showing them, but if you need to manually:
   `./bin/dx comment mark-read <target-type> <target-id> --role=llm`
4. Continue the vertical loop — comments are handled inline, not as separate work items.

**Closing a dev task with code refs** — when running `./bin/dx todo dev done TK-N`, attach the files you touched so the
parent issue records them as `zdx_issue_code_refs`. Pass `--file` once per file (repeatable). Syntax
`<path>[:start[-end]][@hash]`:

- `--file internal/cli/work/todo.go`                         — whole file, hash = git HEAD
- `--file internal/cli/work/todo.go:1005`                    — single line
- `--file internal/cli/work/todo.go:1005-1036`               — line range
- `--file internal/cli/work/todo.go:1005-1036@abc1234`       — explicit commit

Use when a dev task involves real edits and you want the issue's code-ref trail populated before closing.

**Stale tasks** (when solo emits `[review:stale] TK-N` or `[dev]` with a `⚠ state unknown` warning):

The task was created but never claimed, and enough time has passed that the codebase may have changed.

1. **Read the referenced files first.** Check if the work described in the task is already done, superseded, or still needed.
2. If already implemented: `./bin/dx todo dev done TK-N`
3. If still needed: proceed with the implementation as normal.
4. Do not start editing code until you have verified the task is still relevant.

**Orphan tasks** (when solo emits `[owner:orphan-task] TK-N`):

The task has no parent issue — invisible to the normal issue-based workflow.

1. If the task is done or stale: `./bin/dx todo dev done TK-N`
2. If still needed: file an issue to host it, then link the task.

**Maturity nudges** (when solo emits `[owner:quantify-goal]`, `[owner:attribute-feature]`, `[tech:instrument-feature]`, `[owner:decompose-feature]`):

These are maturity-gradient items — the project is healthy enough to work but could be healthier. Handle them if they're quick (add a metric, link a goal), defer if they require product decisions.

**Blocked issues:** if the vertical is empty because IS-N is blocked by other issues:

1. Run `./bin/dx todo show IS-N` to read the `Blocked:` list.
2. For each blocking issue, check if it is itself blocked: `./bin/dx todo show IS-X`.
3. Recursively find the unblocked leaf issues (issues with no `Blocked:` field, or whose blockers are all closed).
4. Work each leaf vertical in turn using the same vertical loop above.
5. After all leaves are done, re-run `./bin/dx todo solo --issue=IS-N` — the original issue may now be unblocked.
6. **Stop after closing the original issue.**

**Blockers:** if you hit a DX gap (missing flag, broken endpoint, 500, field that doesn't round-trip) or any blocker you
can't resolve in one step — file it and stop:

1. `./bin/dx issue add --title="..." --context="..."`
2. Report what blocked you and the new issue ID. Do not work around it.

**Blocker questions** (`dx question add`) — questions that block progress and require a human answer. Before filing one,
exhaust every investigation avenue available to you:

1. **Read the code.** Search handlers, schema, models, routes, configs, and tests for the answer. grep for keywords,
   read surrounding context, trace call chains.
2. **Read the data.** Check migrations, seed data, existing DB state, API responses, logs — anything that reveals how
   the system actually behaves.
3. **Read the docs.** CLAUDE.md, skill files, plan docs, README, inline comments, commit messages (`git log --grep`).
4. **Experiment.** Run the code, hit the endpoint, query the DB, write a quick test — prove or disprove your hypothesis.

Only file a blocker question when the answer **genuinely requires human judgment** — product direction, priority calls,
user-facing wording preferences, business rules not derivable from code, or "which of these two valid approaches do you
prefer." If the answer is discoverable by reading code, schema, data, or docs, discover it yourself.

When you do file:
- `./bin/dx question add --target-type=<issue|task|feature> --target-id=<ID> --context="<what you need decided and why you can't decide it yourself>"`
- Include what you already investigated so the human doesn't retrace your steps.

**Plans** — if a vertical involves complex multi-step work, create a living plan:

- `./bin/dx plan add "<title>" --issue=IS-N` to anchor a plan to the issue
- `./bin/dx plan step add PL-N "<step description>"` to add ordered steps
- `./bin/dx plan step update <step-id> --status=done` to track progress
- If a step spawns new work: `./bin/dx plan step ref <step-id> issue IS-X` to link the discovery

**Done** when the vertical is empty and no unblocked leaf work remains, or when a blocker stops progress. Then stop — do
not pick up a new issue.

**Before stopping:** the git tree must be clean. Run `git status` — if anything is uncommitted (staged, unstaged, or
untracked work-product), commit it (or ship it per step 5) before reporting done. Do not leave dangling changes for the
next session: you have the context now, the next dev won't.

**Post-work DX analysis**
If you when you flailed — retried commands, had to look up documentation for confusing command, or had to work around a
lack of commands for your intent,
Or if you hit unexpected errors, or had to deviate from the happy path and do weird surgery,
Or if you had to discover what SHOULD have been obvious, for instance, a request to act on something without obvious
next steps or details about what it was and had to spend time figuring out what could have easily been provided to you
in the first place:

1. For each point of friction, ask: could a better error message, flag, or guard have guided me correctly on the first
   attempt?
2. If yes, file an issue:
   `./bin/dx issue add --title="..." --context="<what happened, what the tool said, what it should have said or done instead>"`
3. Report the filed issue IDs.
   Do not file issues for expected complexity or user error. Only file when the tool itself failed to guide correctly.
