-- One-time data backfill: cannot reconstruct prior reopen_count / blocked
-- state. Down is a no-op; rerunning the up is idempotent (a re-blocked row
-- with the same reason and empty reference_issue_id will simply reset again).

SELECT 1;
