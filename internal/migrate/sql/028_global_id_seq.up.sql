-- Switch from per-project ID sequences to a single global sequence per kind,
-- so IS-N / TK-N values are unique across all projects.
ALTER TABLE zdx_id_seq RENAME TO zdx_id_seq_old;

CREATE TABLE zdx_id_seq (
    kind     TEXT PRIMARY KEY,
    next_val INT  NOT NULL DEFAULT 1
);

INSERT INTO zdx_id_seq (kind, next_val)
SELECT kind, MAX(next_val)
FROM zdx_id_seq_old
GROUP BY kind;

DROP TABLE zdx_id_seq_old;
