---
name: work
description: Work a single issue vertical (owner triage → tech plan → dev done). Pass an issue ID or omit to let todo take claim one. One vertical per session.
disable-model-invocation: true
argument-hint: "[IS-N]"
---

Drive `./bin/dx todo` in remote mode. Issue: $ARGUMENTS (optional). **One vertical per invocation — stop after the first
issue is closed and shipped.**

The todo text returned by `dx todo take` / `dx todo solo` is the authoritative per-kind playbook. Read it and follow it
verbatim. This skill covers only entry routing and cross-cutting rules.

**Entry:**

- **Claimed todo already provided in the prompt** (e.g. "Claimed todo N [kind] target=type:id"): use it directly — do
  NOT call `./bin/dx todo take` again. Follow the todo's text verbatim.
- **Issue given as argument:** run `./bin/dx todo solo --issue=IS-N` and follow the picks until the issue closes.
- **Neither:** run `./bin/dx todo take` — atomically claim the next todo. If it returns "no work available", stop.

For any claimed todo, route by target:
- `target=task:TK-N` — `./bin/dx todo show TK-N` to find the parent issue; run the vertical on that issue.
- `target=issue:IS-N` — run the vertical on IS-N.
- Anything else (`read:comments`, `respond:stale`, `close:tracker`, maturity nudges, orphans, etc.) — follow the todo
  text directly. Stop after handling. No vertical loop.

**Vertical loop** (impl/ops issues — NOT trackers; trackers auto-decompose via their todo text):

1. `./bin/dx todo solo --issue=IS-N`
2. Follow the returned todo's text (triage / add tasks / dev / closable) — the text tells you exactly what to do.
3. Repeat until solo prints nothing to do.
4. `./bin/dx issue close IS-N --reason=done` (if not already closed by the closable candidate).
5. **Ship** if the vertical produced production code:
   - Commit: `dx commit --intent -m "..."` (or: `git add <intent-only files> && git commit`). Do NOT stage `internal/db/*.sql.go`, `internal/dxclient/models.gen.go`, `ui/src/api.gen.ts`, `schema/shipped.sql` — the merge-train regenerates them.
   - If `internal/migrate/sql/` or `queries/*.sql` changed: `~/go/bin/sqlc generate && go build ./...`
   - `./bin/ship` (never `--allow-dirty` — emergency only).
   - Skip ship for docs/skill/planning-only changes — just commit.
6. **Stop.** Report what was done. Do not pick up another vertical.

**Blocked issues:** if IS-N is blocked, `dx todo show IS-N` lists blockers. Recurse to unblocked leaves, work each leaf
vertical, then re-run solo on IS-N. Stop after closing the original.

**Generated files:** Never stage or commit `*.sql.go`, `models.gen.go`, `api.gen.ts`, `shipped.sql`. Use `dx commit --intent` to enforce this automatically.

**Blockers:** if you hit a DX gap you can't resolve in one step: `./bin/dx issue add --title=... --context=...`, report
the new issue ID, stop. Do not work around it.

**Blocker questions** (`dx question add`): file ONLY when the answer genuinely requires human judgment (product
direction, priority, user-facing wording, business rules). First exhaust: read the code, read the data (schema, logs,
DB state), read the docs (CLAUDE.md, git log, skill files), experiment. Include what you investigated so the human
doesn't retrace your steps.

**Before stopping:** git tree must be clean. `git status` — commit or ship anything uncommitted. Don't leave dangling
changes.

**Post-work DX analysis:** if you flailed on a command, hit an unclear error, or had to discover what should have been
obvious, file a **proposal** (not an issue) via the `proposal_add` MCP tool with `source_type="session-review"` — or
fall back to `./bin/dx issue add` if MCP is unavailable. Report filed IDs. Do not file for expected complexity or user
error — only when the tool itself failed to guide correctly.
