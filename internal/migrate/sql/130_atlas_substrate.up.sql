CREATE TABLE zdx_coderefs (
    id         BIGSERIAL PRIMARY KEY,
    project_id INTEGER NOT NULL REFERENCES zdx_projects(id) ON DELETE CASCADE,
    file       TEXT NOT NULL,
    start_line INTEGER,
    end_line   INTEGER,
    symbol     TEXT NOT NULL DEFAULT '',
    sha        TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE zdx_nodes (
    id          BIGSERIAL PRIMARY KEY,
    project_id  INTEGER NOT NULL REFERENCES zdx_projects(id) ON DELETE CASCADE,
    kind        TEXT NOT NULL,
    slug        TEXT NOT NULL,
    title       TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (project_id, kind, slug)
);

CREATE TABLE zdx_edges (
    id          BIGSERIAL PRIMARY KEY,
    project_id  INTEGER NOT NULL REFERENCES zdx_projects(id) ON DELETE CASCADE,
    from_node_id BIGINT NOT NULL REFERENCES zdx_nodes(id) ON DELETE CASCADE,
    to_node_id   BIGINT NOT NULL REFERENCES zdx_nodes(id) ON DELETE CASCADE,
    edge_type   TEXT NOT NULL,
    label       TEXT NOT NULL DEFAULT '',
    weight      INTEGER NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (from_node_id, to_node_id, edge_type)
);

CREATE TABLE zdx_narrative_chunks (
    id            BIGSERIAL PRIMARY KEY,
    project_id    INTEGER NOT NULL REFERENCES zdx_projects(id) ON DELETE CASCADE,
    node_id       BIGINT NOT NULL REFERENCES zdx_nodes(id) ON DELETE CASCADE,
    coderef_id    BIGINT REFERENCES zdx_coderefs(id) ON DELETE SET NULL,
    title         TEXT NOT NULL DEFAULT '',
    body          TEXT NOT NULL,
    sha_at_write  TEXT NOT NULL DEFAULT '',
    verifier_kind TEXT NOT NULL DEFAULT 'agent',
    verified_at   TIMESTAMPTZ,
    verified_by   TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX zdx_narrative_chunks_node_id_idx ON zdx_narrative_chunks(node_id);
CREATE INDEX zdx_edges_from_node_id_idx ON zdx_edges(from_node_id);
CREATE INDEX zdx_edges_to_node_id_idx ON zdx_edges(to_node_id);
