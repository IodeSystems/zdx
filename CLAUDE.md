# zdx — Developer Experience Platform

Self-hosted platform for human+LLM collaborative software development. Go API server + React UI + dx CLI.

## Build & Run

```bash
make build                    # compile Go binaries (dx, dx-server, db)
make ui                       # build React frontend
./bin/dev                     # dev server (auto-migrate + UI dev server)
./bin/ship                    # build, test, deploy to prod
```

Server requires `DATABASE_URL` env var pointing to PostgreSQL 16+ with pgvector.

## Generated Code — Do Not Edit

- `internal/db/*.sql.go` — sqlc from `queries/*.sql` + `schema/shipped.sql`. Regenerate: `~/go/bin/sqlc generate`
- `ui/src/api.gen.ts` — openapi-typescript from server's `/openapi.json`. Auto-regenerated on dx-server startup.
- `internal/db/tygo/` — Go→TypeScript types via tygo. Regenerate: `make generate`

## Database Workflow

1. Add migration in `internal/migrate/sql/` (NNN_name.up.sql + NNN_name.down.sql)
2. Dev: migrations run automatically on server start
3. Update shipped.sql: `./bin/db gen` (pg_dump of current schema)
4. Regenerate queries: `~/go/bin/sqlc generate`
5. Verify: `go build ./...`

## Testing

```bash
make test                     # Go tests
./bin/dx test                 # unified test runner (Go + vitest + demos)
```

## Linting

```bash
./bin/lint                    # gofmt, govet, sqlcvet, tsc, eslint, knip, sqlc-drift
```

## Key Directories

- `cmd/dx-server/` — API server entry point (port 7600)
- `cmd/dx/` — CLI entry point
- `internal/cli/` — cobra command implementations
- `internal/server/` — HTTP handlers, middleware, routes
- `internal/db/` — sqlc-generated database layer
- `queries/` — SQL source files (sqlc input)
- `schema/shipped.sql` — canonical schema snapshot
- `internal/migrate/sql/` — migration SQL files (up/down pairs)
- `ui/src/routes/` — TanStack Router file-based routes
- `ui/src/api/` — typed API client (openapi-fetch)
- `.zdx/config.yaml` — project zdx configuration

## DX CLI

All project management through `./bin/dx`: issues, tasks, features, goals, themes, todo queues.
Run `./bin/dx --help` for the full command tree.
