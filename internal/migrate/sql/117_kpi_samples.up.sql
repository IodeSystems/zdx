CREATE TABLE zdx_kpi_samples (
  id         bigserial PRIMARY KEY,
  project_id integer NOT NULL REFERENCES zdx_projects(id),
  sampled_at timestamptz NOT NULL DEFAULT NOW(),
  scope      text NOT NULL CHECK (scope IN ('tech', 'owner')),
  check_name text NOT NULL,
  value      double precision NOT NULL,
  unit       text NOT NULL DEFAULT ''
);
CREATE INDEX ON zdx_kpi_samples (project_id, scope, check_name, sampled_at DESC);
