# Multi-Agent Parallel Workflows

Goal: enable multiple agents to work concurrently on a project, each in an isolated worktree + docker compose stack, coordinated through reservable todos.

## Status

SDLC model revamp is complete (migrations 073–080):
- [x] Focus (was theme), goal metrics, feature hierarchy, plans, spec concern_type
- [x] Solo queue maturity nudges
- [x] Reservable todos: claim/release/renew with FOR UPDATE SKIP LOCKED

## Remaining work

### Phase 1: Agent config in .zdx/config.yaml

Add `agent` section to the project config so projects declare their dev environment:

```yaml
agent:
  compose_file: docker-compose.agent.yaml   # compose stack for isolated dev services
  dev_dockerfile: Dockerfile.agent           # dev image with tooling
  max_worktrees: 4                           # concurrent agent slots
  llm_provider: claude                       # claude | local | server
  claude_model: claude-sonnet-4-6            # when provider=claude
```

- [ ] Add `Agent` struct to `internal/config/config.go`
- [ ] Update `dx agent start` to read compose_file / dev_dockerfile from config instead of hardcoding
- [ ] `dx agent` loop uses `max_worktrees` to check slot availability

### Phase 2: Admin LLM config UI

Server already has `zdx_llm_configs` table + admin endpoints:
- `GET /api/admin/llm-config`
- `PUT /api/admin/llm-config`
- `POST /api/admin/llm-config/test`

- [ ] Add admin UI page for LLM provider configuration
- [ ] Show provider type, URL, model, test-connection button
- [ ] Surface configured provider in agent start flow

### Phase 3: Agent loop with todo claiming

The `dx agent claude` loop already exists. Extend it to use the new claim system:

```
loop:
  1. POST /api/dx/solo/claim → get highest-priority unclaimed todo
  2. if no todo → sleep, retry
  3. create worktree + compose stack (if agent.compose_file configured)
  4. start agent session (claude or local) with claimed todo as prompt
  5. heartbeat: POST /api/dx/solo/renew every N minutes
  6. on completion: POST /api/dx/solo/release (resolve=true if success)
  7. tear down worktree + compose
  8. goto 1
```

- [ ] Refactor `runLoop` in `internal/cli/agent/claude.go` to claim todos instead of using the old solo→issue flow
- [ ] Add worktree slot check: count active worktrees vs max_worktrees
- [ ] Pass claimed todo context as agent seed prompt
- [ ] Wire heartbeat to renew todo lease (not just agent heartbeat)

### Phase 4: Docker dev image

Projects with agents need a dev machine image for isolation:

- [ ] Template `Dockerfile.agent` that installs project tooling (go, node, etc.)
- [ ] Template `docker-compose.agent.yaml` with postgres, valkey, and the dev image
- [ ] Dev image mounts the worktree as a volume
- [ ] Non-static ports (docker picks) — discovered via `docker compose port`
- [ ] `dx agent init` scaffolds the Dockerfile + compose file from project config
- [ ] Agent binary (dx or zdx-agent) runs inside the container
- [ ] Claude token exported into container env when provider=claude

### Phase 5: Task groups and reservation

Task groups allow an agent to claim a batch of related tasks:

- [ ] `task_group` field already exists on tasks — wire it into claiming
- [ ] When an agent claims a todo that references an issue, also claim all ready tasks on that issue
- [ ] Agent releases all claimed tasks on session close
- [ ] Prevent double-claiming: task claim checks todo claim ownership

### Phase 6: Coordination and observability

- [ ] Agent dashboard in UI: active agents, their worktrees, claimed todos, session status
- [ ] Conflict detection: warn when two agents touch the same files
- [ ] Merge queue: agent PRs integrate through the standard review flow

---

## Execution order

Phase 1 (config) → Phase 3 (loop refactor) are the critical path.
Phase 2 (admin UI) is independent, can run in parallel.
Phase 4 (docker) builds on Phase 1 config.
Phase 5–6 are enhancements after the core loop works.

**Start with:** Phase 1 (config struct) → Phase 3 (claim-based loop).
