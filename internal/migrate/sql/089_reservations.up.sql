CREATE TABLE zdx_reservations (
    id          bigserial PRIMARY KEY,
    project_id  integer NOT NULL REFERENCES zdx_projects(id) ON DELETE CASCADE,
    target_type text    NOT NULL,
    target_id   text    NOT NULL,
    claimed_by  text    NOT NULL DEFAULT '',
    claimed_at  timestamptz NOT NULL DEFAULT NOW(),
    released_at timestamptz,
    lease_expires_at timestamptz
);

CREATE INDEX zdx_reservations_project_target ON zdx_reservations (project_id, target_type, target_id);
CREATE INDEX zdx_reservations_claimed_at ON zdx_reservations (project_id, claimed_at DESC);
