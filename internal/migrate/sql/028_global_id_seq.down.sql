CREATE TABLE zdx_id_seq_old (
    project_id INT  NOT NULL REFERENCES zdx_projects(id),
    kind       TEXT NOT NULL,
    next_val   INT  NOT NULL DEFAULT 1,
    PRIMARY KEY (project_id, kind)
);

DROP TABLE zdx_id_seq;
ALTER TABLE zdx_id_seq_old RENAME TO zdx_id_seq;
