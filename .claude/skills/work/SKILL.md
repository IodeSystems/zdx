---
name: work
description: Work a single issue vertical (owner triage → tech plan → dev done). Pass an issue ID or omit to let solo pick. One vertical per session.
disable-model-invocation: true
argument-hint: "[IS-N]"
---

Drive `./bin/dx todo` in remote mode. Issue: $ARGUMENTS (optional). **One vertical per invocation — stop after the first issue is closed and shipped.**

**Entry:**
- No issue given: run `./bin/dx todo solo` — take whatever it picks. If the pick is a task (TK-N), run `./bin/dx todo show TK-N` to find its parent issue; use that issue for the vertical.
- Issue given: proceed directly to the vertical loop.

**Vertical loop** for IS-N:
1. `./bin/dx todo solo --issue=IS-N`
2. Do whatever the pick says (owner triage → tech plan → tech add → dev done)
3. Repeat until solo prints nothing to do
4. `./bin/dx issue close IS-N --reason=done`
5. If the vertical produced shippable fixes/features (server, UI, schema, queries, or any code that runs in prod):
   - Commit all changes first: `git add <files> && git commit -m '...'`
   - If schema (internal/migrate/sql/) or queries (queries/*.sql) changed: `~/go/bin/sqlc generate` then `go build ./...` to verify
   - `bin/ship` (never `--allow-dirty` during normal dev — that is an emergency-only escape hatch)
   - Note: migrations run automatically on dev server restart; prod migrations run via ship
   - Skip ship for docs-only / skill-only / planning-only changes — just commit.
6. **Stop. Report what was done. Do not pick up another vertical.**

Vertical scope is automatic: solo --issue=IS-N picks only triage:IS-N, decompose:IS-N, plan: for linked features, and dev tasks with task.issue == IS-N.

**Triage** (when solo emits `[triage] IS-N`):
1. **Verify independently.** Reproduce or read the relevant code/UI before accepting the report at face value.
2. **Dup-check.** `./bin/dx issue list` and scan open + recent closed issues for similar work. If a close match exists, close the new one as duplicate (`--reason=duplicate`) and reopen the prior issue with the new context if it adds detail.
3. **Rewrite prescriptively.** Title = intended outcome (not symptom). Context covers: (a) what *should* happen, (b) what *did* happen, (c) implementation direction if known.
4. **Apply** via `./bin/dx todo owner triage IS-N --title=... --context=... --type=<ops|impl> --priority=<1-4>`.

**Blocked issues:** if the vertical is empty because IS-N is blocked by other issues:
1. Run `./bin/dx todo show IS-N` to read the `Blocked:` list.
2. For each blocking issue, check if it is itself blocked: `./bin/dx todo show IS-X`.
3. Recursively find the unblocked leaf issues (issues with no `Blocked:` field, or whose blockers are all closed).
4. Work each leaf vertical in turn using the same vertical loop above.
5. After all leaves are done, re-run `./bin/dx todo solo --issue=IS-N` — the original issue may now be unblocked.
6. **Stop after closing the original issue.**

**Blockers:** if you hit a DX gap (missing flag, broken endpoint, 500, field that doesn't round-trip) or any blocker you can't resolve in one step — file it and stop:
1. `./bin/dx issue add --title="..." --context="..."`
2. Report what blocked you and the new issue ID. Do not work around it.

**Done** when the vertical is empty and no unblocked leaf work remains, or when a blocker stops progress. Then stop — do not pick up a new issue.

**Before stopping:** the git tree must be clean. Run `git status` — if anything is uncommitted (staged, unstaged, or untracked work-product), commit it (or ship it per step 5) before reporting done. Do not leave dangling changes for the next session: you have the context now, the next dev won't.

**Post-work DX analysis** (only when you flailed — retried commands, had to look up documentation for confusing command, or had to work around a lack of commands for your intent, hit unexpected errors, or had to deviate from the happy path):
1. For each point of friction, ask: could a better error message, flag, or guard have guided me correctly on the first attempt?
2. If yes, file an issue: `./bin/dx issue add --title="..." --context="<what happened, what the tool said, what it should have said or done instead>"`
3. Report the filed issue IDs.
Do not file issues for expected complexity or user error. Only file when the tool itself failed to guide correctly.
