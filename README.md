# zdx

A self-hosted developer-experience platform for human+LLM collaborative software development. Track issues, features, themes, and goals; manage work queues; ingest observability data (timings, errors, logs, counters); and ship with confidence — all from a single portal and CLI.

**Live instance:** https://zdx.iodesystems.com

## Architecture

| Layer | Tech | Location |
|-------|------|----------|
| API server | Go, huma/chi, pgx | `cmd/dx-server/`, `internal/server/` |
| CLI | Go, cobra | `cmd/dx/`, `internal/cli/` |
| UI | React 19, Vite, MUI, TanStack Router | `ui/` |
| Database | PostgreSQL, sqlc | `queries/`, `internal/db/`, `internal/migrate/sql/` |
| Vector search | pgvector via zvec | `internal/zvec/` |
| Realtime | WebSockets + Valkey | `internal/ws/` |

## Prerequisites

- Go 1.25+
- Node.js 20+ / npm
- PostgreSQL 16+ with pgvector
- sqlc (`go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest`)

## Quickstart

```bash
# Clone and build
git clone https://github.com/iodesystems/zdx-go.git
cd zdx-go
make build        # compiles dx, dx-server, db binaries
make ui           # builds the React frontend

# Database
export DATABASE_URL="postgres://user:pass@localhost:5432/zdx?sslmode=disable"
./bin/db migrate   # run migrations

# Run
./bin/dev          # starts server with auto-migrate + UI dev server
```

The server listens on `:7600` by default.

## Key commands

```bash
./bin/dx todo solo           # grab the next work item
./bin/dx issue add           # file an issue
./bin/dx feature list        # list features
./bin/dx goal list           # list project goals
./bin/dx theme list          # list roadmap themes
make test                    # run all tests
./bin/ship                   # build, test, deploy
```

## Directory layout

```
cmd/
  dx-server/       API server entry point
  dx/              CLI entry point
  db/              Database migration tool
internal/
  cli/             Cobra command tree
  config/          Configuration loading
  db/              sqlc-generated database layer
  llm/             LLM integration
  migrate/         Migration runner
  server/          HTTP handlers and middleware
  testharness/     Unified test adapter framework
  ws/              WebSocket realtime events
  zvec/            Vector similarity search
ui/                React frontend (Vite + MUI)
queries/           SQL query files (sqlc input)
internal/migrate/sql/  Database migration SQL files (up/down pairs)
deploy/            Deployment artifacts and provisioning
infra/             Infrastructure and provisioning clients
```

## License

Proprietary — IODesystems.
