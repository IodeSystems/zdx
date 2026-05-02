-- One-shot backfill: synthesize capability_gap incomplete-reports for
-- pre-existing stuck read:comments todos (the cohort from before IS-677
-- gave agents a way to mark a comment as read without authoring a reply).
--
-- The aggregator buckets reports by (reason, evidence_fingerprint), so the
-- fingerprint MUST match what handlers_todos.go incompleteEvidenceFingerprint
-- would emit for the same evidence map: SHA256 of the canonical JSON form
-- [["missing_capability","mark-comment-as-read"]] (sorted keys, [k,v] pairs).
INSERT INTO zdx_todo_incomplete_reports (
    project_id,
    todo_id,
    reason,
    explanation,
    suggested_next,
    evidence,
    evidence_fingerprint,
    agent_id
)
SELECT
    t.project_id,
    t.id,
    'capability_gap',
    'auto-backfilled — pre-existing stuck claim before IS-662 shipped',
    '"block on IS-677"'::jsonb,
    jsonb_build_object('missing_capability', 'mark-comment-as-read'),
    '2ef57015260059ce554142f844e54bb9618e6cb695e69c558087dfa570c7e7fe',
    'backfill'
FROM zdx_todos t
WHERE t.kind = 'read:comments'
  AND t.status = 'open'
  AND NOT EXISTS (
        SELECT 1
          FROM zdx_todo_incomplete_reports r
         WHERE r.todo_id = t.id
           AND r.reason = 'capability_gap'
           AND r.evidence_fingerprint = '2ef57015260059ce554142f844e54bb9618e6cb695e69c558087dfa570c7e7fe'
  );
