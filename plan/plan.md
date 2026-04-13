# zdx-go pending work

## Where we are

Live at https://zdx.iodesystems.com. huma+chi server with ~35 endpoints mapped to
the Zig CLI's `http_adapter.zig`. Fresh empty DB. The `zdx` project row was
inserted by hand (`psql INSERT INTO zdx_projects(slug, name) VALUES('zdx', 'zdx')`).

Key conventions in the server:
- Issues stored as `id TEXT = "IS-N"`; API wire format is `{"id": N}` (int32).
  `issueIDFromInt(n) → "IS-N"`, `issueIntID(s) → N`. Same pattern for tasks (`TK-N`).
- `huma.Register` derives schema names from Go types. Anonymous `[]struct` fields
  get auto-named `"Item"` — two in the same API will collide and panic at
  registration, silently dropping every route that comes after. Always extract
  slice element types to named types.
- `bin/api-types` regenerates `ui/src/api.gen.ts` from `/openapi.json`. Run
  after handler type changes, like `bin/db gen`.

## 1. Auth middleware  [BLOCKER]

Server accepts any request. Zig CLI already sends `X-Api-Key: <token>`. Need to
validate against `zdx_api_keys` and reject with 401 if missing/invalid.

- Add chi middleware that runs before huma-dispatched routes
- Look up `X-Api-Key` in `zdx_api_keys` (add the query to `queries/dx.sql` →
  `./bin/db gen`)
- Skip auth for `GET /api/health` and `GET /openapi.*` (so version check works
  without creds)
- Plumb the resolved user into request context so future per-user features
  (permissions, audit) have something to work with
- Add a test: request without header → 401, with valid key → 200

**Done when:** anon request to `/api/dx/todo/issue/list` returns 401;
the Zig CLI still works end-to-end.

## 2. Project endpoints  [BLOCKER for new project onboarding]

Right now a new project has to be hand-inserted. Add:

- `POST /api/project` — body `{slug, name}`, returns the project row
- `GET /api/projects` — lists all projects (scoped by authed user once #1 is
  in)
- Both as huma operations in `internal/server/handlers.go`

**Done when:** `curl -X POST /api/project -d '{"slug":"foo","name":"Foo"}'`
creates a row and returns it.

## 3. /api/health build_sha  [NOISE FIX]

Zig CLI's `warnIfVersionMismatch` looks for `build_sha` on every invocation;
currently prints a stray message because the field is empty.

- In `cmd/dx-server/main.go`, ldflag-set `var buildSHA string` the same way
  `bin/ship` already does for `main.version`
- Thread into the server (`server.New(pool, staticDir, buildSHA)`)
- Health handler returns `{"status":"ok","build_sha":"<sha>"}`

**Done when:** `curl /api/health` includes `build_sha`; CLI version warning
fires only when it legitimately mismatches.

## 4. Go CLI rewrite  [THIS IS THE FUTURE CLIENT]

The Go CLI is the primary client going forward — Zig build times killed the
Zig CLI as a daily driver. The Zig `http_adapter.zig` just needs to keep
working well enough to run out existing checkouts; we don't invest further in it.

Current state:
- `internal/cli/todo.go` and `internal/cli/issue.go` hit old paths
  (`/api/dx/issue`, `/api/dx/task/done`) that no longer exist.
- They import `internal/apitypes.*` which targets pre-huma shapes with
  string IDs.
- `dx migrate up` subcommand is still correct and is called by
  `bin/ship`'s `run_migrate`.

Work:
- Generate a typed Go client from the huma server's OpenAPI spec
  (`oapi-codegen -package api -generate types,client` against `/openapi.json`).
  Same pattern as `bin/api-types` does for TS — commit the generated file,
  regenerate on API change.
- Delete `internal/apitypes/` and the easyjson-generated file.
- Rewrite `internal/cli/*.go` against the generated client.
- Now that the Go CLI controls both sides, drop the integer-ID wire wrapping
  for new endpoints — return `"IS-N"` strings directly in responses. Keep the
  integer-ID form on existing endpoints for Zig CLI compatibility during the
  transition, but new work goes string-native.
- Decide CLI surface: should the Go CLI match the Zig CLI's command set
  (`dx todo`, `dx issue`, `dx feature`, `dx ctx`, `dx test`, `dx lint`, …)?
  The Zig version is ambitious — some of it (test drivers, lint rules, watch)
  probably doesn't belong in the Go CLI since they're Zig-project-specific.
  Scope down to the workflow commands: `todo`, `issue`, `feature`, `task`,
  `theme`, `journal`, `state`, `todos`, `migrate`, `daemon`. Test/lint/watch
  stay out.

**Done when:** `bin/dx todo solo` against production works end-to-end; the
Zig CLI can be deprecated on personal machines.

This is the biggest remaining chunk. Budget a full session.

## 5. Data carryover  [RESOLVED — not porting]

Fresh start. Old Zig-server data is abandoned. The prod DB stays empty until
real work recreates what we care about.

## 6. TS client generation  [POLISH]

`bin/api-types` is wired but never run. `ui/src/api.gen.ts` doesn't exist;
the UI uses hand-written fetch.

- Start a slot locally (or hit prod), run `bin/api-types`
- Commit `ui/src/api.gen.ts`
- `npm install openapi-fetch` in `ui/`
- Convert one UI call site as a pattern (e.g. the issue list) to use
  `createClient<paths>()`
- Leave the rest for incremental conversion

**Done when:** at least one UI fetch uses the generated client; commit shows
the gen step works.

## 7. Handler tests  [POLISH]

35+ huma operations, zero coverage. At minimum:
- Table-driven tests for the ID-conversion helpers (`issueIntID`,
  `taskIDFromInt`, etc.)
- Integration test that spins up the full server + a test DB container, walks
  through: create issue → triage → add task → mark done. Catches schema/handler
  drift.

Not blocking anything, but every handler change right now is "deploy and pray."

## Execution order

1. #1 auth — cheap, unblocks anything real (half session)
2. #2 project endpoints — required for #4 to onboard this project cleanly
3. #3 build_sha — 15 min drive-by
4. #4 Go CLI rewrite — the main event (full session, maybe two)
5. #6 TS client — parallel with anything
6. #7 tests — ongoing, add as we touch each handler
7. #5 data port — only if we decide it's worth it

Do #1–#3 in one session before #4 so the Go CLI is built against a
production-equivalent server (with auth enforced and projects creatable
through the API).

## Ground rules for next session

- **Fresh start.** Old Zig-server data is gone, not ported. Build against
  empty tables.
- **Go CLI is the primary client.** Zig CLI stays working only while it
  already works — no new endpoints optimized for the Zig adapter. When the
  Go CLI can do a task, the Zig CLI is deprecated for that task.
- **Types are the contract.** Server handlers define huma Input/Output. Go
  CLI generates from `/openapi.json` via `oapi-codegen`. UI generates the
  same way via `openapi-typescript`. No hand-maintained wire types on either
  side.

## Note on zest repo

Separate from this plan: `iodesystems/zest` has uncommitted WIP on main
(`V027__issue_clarifications.sql`, `V028__llm_config.sql`, modified
FeatureDetail/IssueDetail/WorkLogTab, new `admin_llm_config.zig` and
`issue_clarifications.zig`). Predates this session. Handle there, not here.
