-- IS-979: one-time backfill to recover rows blocked by the
-- ResolveTodosNotInKeys cycling bug fixed in IS-977 (TK-1651).
--
-- The claim path used to GC the queue on every poll, causing candidates that
-- transiently disappeared from one poll's key set to be resolved → re-emitted
-- → reopen_count++ on the next poll. After ~3 cycles the row hit the
-- upsert guard (queries/todos.sql:56-65) and stuck at blocked=true forever.
--
-- IS-977 removed the GC sweep from the claim path; IS-978 extended
-- UnblockAllTodos to reset reopen_count so the admin escape hatch survives
-- the next claim. But existing rows in affected projects still carry
-- inflated reopen_count, and the upsert guard re-fires on the next claim's
-- UpsertTodo even after IS-977/978 land. This migration resets them once.
--
-- Filter intent: only touch rows whose block reason matches the churn-guard
-- message AND whose reference_issue_id is empty. The reference_issue_id
-- filter excludes rows that were auto-filed against a real system-gap issue
-- (the cycle-count auto-file path), where the block is legitimate and
-- closing the gap issue is the correct unblock trigger.

UPDATE zdx_todos
SET reopen_count   = 0,
    blocked        = false,
    blocked_reason = ''
WHERE blocked = true
  AND reference_issue_id = ''
  AND blocked_reason LIKE 'Reopened 3+ times%';
