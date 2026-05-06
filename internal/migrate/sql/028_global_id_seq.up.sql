-- Switch from per-project ID sequences to a single global sequence per kind,
-- so IS-N / TK-N values are unique across all projects.
--
-- The pkey constraint is named explicitly. Inline `kind TEXT PRIMARY KEY`
-- would let postgres pick the name, and because zdx_id_seq_old still holds
-- the original `zdx_id_seq_pkey` name (carried by the rename above), the new
-- table's auto-named pkey would clash and end up as `zdx_id_seq_pkey1`. Prod
-- ran an earlier shape of this migration that produced `zdx_id_seq_global_pkey`,
-- so pinning the name here keeps fresh-migrate output aligned with the live
-- prod schema.
ALTER TABLE zdx_id_seq RENAME TO zdx_id_seq_old;

CREATE TABLE zdx_id_seq (
    kind     TEXT NOT NULL,
    next_val INT  NOT NULL DEFAULT 1,
    CONSTRAINT zdx_id_seq_global_pkey PRIMARY KEY (kind)
);

INSERT INTO zdx_id_seq (kind, next_val)
SELECT kind, MAX(next_val)
FROM zdx_id_seq_old
GROUP BY kind;

DROP TABLE zdx_id_seq_old;
