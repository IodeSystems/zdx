# zdx — Developer Experience Platform

Self-hosted platform for human+LLM collaborative software development. Go API server + React UI + dx CLI.

## Build & Run

```bash
make build                    # compile Go binaries (dx, dx-server, db)
make ui                       # build React frontend
./bin/dev                     # dev server (auto-migrate + UI dev server)
./bin/ship                    # build, test, deploy to prod
./bin/dx merge-train run [branch]  # rebase worker branch onto dev, regen artifacts, lint, ff-merge
```

Server requires `DATABASE_URL` env var pointing to PostgreSQL 16+ with pgvector.

## Generated Code — Do Not Edit

- `internal/db/*.sql.go` — sqlc from `queries/*.sql` + `schema/shipped.sql`. Regenerate: `~/go/bin/sqlc generate`
- `internal/dxclient/models.gen.go` — oapi-codegen from `internal/dxclient/openapi.json`. Contains both model types AND typed `APIClient` with methods for every endpoint. Regenerate: `make gen-dxclient`. New CLI code should use `dxclient.APIClient` methods, not raw `c.Get("/api/...")` calls.
- `ui/src/api.gen.ts` — openapi-typescript from server's `/openapi.json`. Dev dx-server hashes the spec on startup and regenerates when it changes (no tsc; run `bin/lint` to type-check). Prod builds skip the regen. New UI code should use the generated openapi-fetch client, not raw `apiFetch`/`apiPost`.

### Worker Commit Contract — Intent Only

Workers commit **intent files only**. Generated artifacts are owned by the merge-train, which regenerates them on top of the worker's intent diff before ff-merging into `dev`. Staging generated files makes the rebase fight itself: workers race the merge-train and get their commits rewritten with stale codegen.

**DO commit (intent — author here):**
- `internal/migrate/sql/NNN_*.up.sql` / `*.down.sql` — schema migrations
- `queries/*.sql` — sqlc query sources
- `internal/server/handlers/*.go` and other hand-written Go source
- `schema/next.sql` — only when intentionally edited (rare; usually generated)
- `ui/src/**` (excluding `api.gen.ts`) — UI source
- `internal/dxclient/openapi.json` — when handlers change the OpenAPI shape

**DO NOT commit (generated — merge-train owns):**
- `internal/db/*.sql.go` — sqlc output
- `internal/dxclient/models.gen.go` — oapi-codegen output
- `ui/src/api.gen.ts` — openapi-typescript output
- `schema/shipped.sql` — pg_dump snapshot

**Enforcement:** use `dx commit --intent` — it inspects the staged set, warns on each generated file, unstages it, then commits the remainder. Falls back to `git commit` semantics for everything else (flags, message). Workers run `bin/lint --intent` (skips drift checks); the merge-train runs `bin/lint` (full drift check after regen).

### Codegen Toolchain — Verified Versions

The merge-train requires bit-identical codegen output across workers. All three generators are pinned; do not bump without re-running the double-regen idempotency audit (TK-1416).

| Tool                | Version | Pinned in                                              | Install / invoke                                                         |
|---------------------|---------|--------------------------------------------------------|--------------------------------------------------------------------------|
| sqlc                | v1.30.0 | (developer-installed)                                  | `go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.30.0`                   |
| oapi-codegen        | v2.6.0  | `internal/dxclient/gen/gen.go` (via `go run …@v2.6.0`) | No manual install — `make gen-dxclient` resolves & caches via `go run`. |
| openapi-typescript  | 7.13.0  | `ui/package.json` (exact, no caret) + pnpm lockfile    | `cd ui && pnpm install --frozen-lockfile`; invoked via `pnpm exec`.      |

Avoid `npx` for openapi-typescript — it can fall through to the npm registry and resolve a different version than the lockfile. Always use `pnpm exec` from the `ui/` directory.

## Database Workflow

1. Add migration in `internal/migrate/sql/` (NNN_name.up.sql + NNN_name.down.sql)
2. Rebuild `bin/db` so the new migration is in the embedded FS: `go build -o bin/db ./cmd/db` (or `make build`). `bin/db migrate` embeds the SQL at compile time, so a stale binary will silently skip new files.
3. Dev: migrations run automatically on server start; or run `./bin/db migrate` manually. It prints the version delta so you can see what actually applied.
4. Regenerate queries: `~/go/bin/sqlc generate`
5. Verify: `go build ./...`
6. shipped.sql is regenerated automatically by `bin/ship` during the compat-check (pg_dump in an ephemeral Postgres → `schema/next.sql` → `schema/shipped.sql`). No manual pg_dump step is needed during dev.

## Testing

```bash
make test                     # Go tests
./bin/dx test                 # unified test runner (Go + vitest + demos)
```

### Test Tiers (--importance)
- `must`         → every PR and deploy (default CI gate; also runs in `bin/ship`)
- `should`       → release-branch merge
- `nice-to-have` → manual / periodic only

## Linting

```bash
./bin/lint                    # gofmt, govet, sqlcvet, tsc, eslint, knip, sqlc-drift, openapi-drift
```

Workers run `bin/lint --intent` (skips drift checks); the merge train runs `bin/lint` (full).

Advisory (non-blocking): `raw-api-calls` reports CLI/UI callsites not yet migrated to typed clients.

## SDLC Model

The data model follows a canonical hierarchy:

- **Goal** — outcome with optional metric (metric_name, metric_unit). Maturity gradient: metrics encouraged but not gated.
- **Feature** — demonstrable value driver. `kind`: `direct` (deposits goal currency) or `multiplier` (amplifies other features; needs metric + baseline + target + graph_url). Over-specced features (>8 specs) signal decomposition via parent_feature_id.
- **Spec** — concern on a feature. `kind` (must/should/nice-to-have).
- **Focus** (was "theme") — prioritization lens / sprint. M:N with features via zdx_focus_features. Any feature can belong to multiple active focuses.
- **Plan** — first-class living object anchored to a focus, feature, or issue. Has ordered steps with discovery refs. Commentable, updatable, referencable.

## Project Health

```bash
./bin/dx doctor               # diagnose project against classification-specific maturity vine
./bin/dx doctor --fix         # auto-fix what it can, propose the rest
```

Doctor checks scaffold, identity, planning, verification, agents, and classification-specific rungs (operations, multi-tenancy, distribution). `dx init` runs doctor after scaffolding.

Project classifications: library, tool, service, saas, site. Each shapes the maturity vine.

## Todo Queue + Agent Workflows

```bash
./bin/dx todo take            # claim next todo item (atomic reservation)
./bin/dx todo solo            # legacy: evaluate queue client-side
./bin/dx focus list           # list active focuses (was: dx theme list)
./bin/dx plan list            # list living plans
./bin/dx plan show PL-N       # show plan with steps and discovery refs
```

Todos are **reservable** — agents claim via `POST /api/dx/solo/claim` with `FOR UPDATE SKIP LOCKED`. Lease renewal prevents stale claims. The solo queue generates candidates (high→low priority), merges into persisted todos preserving claim state, and returns the next unclaimed item.

Agent config in `.zdx/config.yaml`:
```yaml
agent:
  llm_provider: claude        # claude | local | server
  claude_model: claude-sonnet-4-6
  max_worktrees: 4
  lease_minutes: 30
```

`dx agent claude --loop` claims todos, runs sessions, renews leases, resolves on completion.

## Key Directories

- `cmd/dx-server/` — API server entry point (port 7600)
- `cmd/dx/` — CLI entry point
- `internal/cli/` — cobra command implementations
- `internal/cli/project/` — project entity commands (issue, feature, goal, focus, plan, spec, doctor)
- `internal/cli/work/` — workflow commands (todo, constraint)
- `internal/cli/agent/` — agent lifecycle (claude, local, start/stop/list)
- `internal/server/handlers/` — HTTP handlers (huma-based)
- `internal/db/` — sqlc-generated database layer
- `internal/doctor/` — project health checks and maturity vines
- `internal/dxclient/` — generated Go API client (oapi-codegen)
- `queries/` — SQL source files (sqlc input)
- `schema/shipped.sql` — canonical schema snapshot
- `internal/migrate/sql/` — migration SQL files (up/down pairs, currently 082)
- `ui/src/routes/` — TanStack Router file-based routes
- `ui/src/api/` — typed API client (openapi-fetch)
- `.zdx/config.yaml` — project zdx configuration
- `plan/plan.md` — current implementation plan

## DX CLI

All project management through `./bin/dx`. Key commands:

```
dx doctor                     # project health check
dx todo take                  # claim next work item
dx issue add/list/close       # issue management
dx feature add/show/set       # feature management (kind, goal, metrics)
dx goal add/list              # goals with optional metrics
dx focus add/list/status      # prioritization lenses (was: theme)
dx plan add/show/step         # living plans with steps
dx spec add                   # specs with kind (must/should/nice-to-have)
dx agent claude --loop        # agent work loop with todo claiming
```

Run `./bin/dx --help` for the full command tree.

Agents (Claude Code, `claude -p`, `dx agent local`) interact with the project tracker by shelling out to `dx` CLI commands rather than via a bespoke MCP server — the previous `dx mcp` server was deprecated in IS-465 to avoid CLI/MCP drift.
