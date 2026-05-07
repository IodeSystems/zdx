# Spike — `dx agent loop` should not multiplex

## Problem

`dx agent loop --container=docker` reads `agent.max_worktrees`
(default 4) and **eagerly** spawns N slot containers + N independent
claim loops, each running its own `RunManagedLoop`. With queue
depth ≥ N that means **N todos go in-flight immediately on `loop`
start**.

Surprised an operator who watched 4 docker containers spin up and
4 issues claimed at once when they expected a single sequential
loop.

## Decision (2026-05-06)

**`dx agent loop` defaults to single-agent, sequential.** No
splitting under the hood. If an operator wants 3 agents running
concurrently, they run 3 `dx agent loop` processes — each is its
own visible OS process, alias, container, claim stream.
Multiplexing was unexpected.

Optional escape hatch: `--concurrency=N` flag on `dx agent loop`
that does the today-style fan-out. **Default is 1.** Operators
who explicitly want a single-process N-fan-out keep the
capability; everyone else gets the predictable "one process =
one agent" model.

The existing `agent.max_worktrees` config field is a **project-wide
ceiling on concurrent agents** (used by `dx agent start` to refuse
over-provisioning, see `agent.go:515`). It is *not* a slot count
for `dx agent loop`. The current usage in `mcp_container.go` is
a misread of the field.

Project config (`.zdx/config.yaml`) overrides global
(`~/.zdx/config.yaml`) for `max_worktrees`, but only `dx agent
start` consults it.

## Fix

- `mcp_container.go`: read concurrency from `opts.Concurrency`
  (new field), default 1. Remove the read from
  `agentCfg.MaxWorktrees`.
- `agent.go`: add `--concurrency` flag to `dx agent loop`, default
  1. Drop `--max-worktrees` from the `loop` flagset (still on
  `dx agent start`).
- Update loop help: "single agent, sequential task claims; pass
  `--concurrency=N` to fan-out into N slot containers within one
  process (most operators should run N separate processes
  instead)".
- Smoke: `dx agent loop --container=docker` → exactly one slot
  under `~/.zdx/projects/zdx/slots/<alias>-0/`, one claim at a
  time. `dx agent loop --container=docker --concurrency=3` →
  three slots, three concurrent claims (today's fan-out preserved
  behind explicit opt-in).

Sized: ~30 minutes. Lands in the same session as the workspace
relocation.

## Out of scope

The "lazy-parallel pool with ceiling" model from the earlier draft
of this doc — rejected. Multiplex behind a flag at user request,
no pool coordinator, no claim fan-out from a single slot. Kept in
`plan/` as a record of the alternatives considered.
