CREATE TABLE zdx_agent_budgets (
  id           BIGSERIAL PRIMARY KEY,
  project_id   INT  REFERENCES zdx_projects(id) ON DELETE CASCADE,
  agent_id     TEXT REFERENCES zdx_agents(id)   ON DELETE CASCADE,
  token_ceiling BIGINT,
  cost_ceiling  DOUBLE PRECISION,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CHECK (
    (project_id IS NOT NULL AND agent_id IS NULL) OR
    (project_id IS NULL     AND agent_id IS NOT NULL)
  )
);

-- Partial unique indexes allow ON CONFLICT targeting for each scope.
CREATE UNIQUE INDEX zdx_agent_budgets_agent_uniq
  ON zdx_agent_budgets (agent_id)
  WHERE agent_id IS NOT NULL;

CREATE UNIQUE INDEX zdx_agent_budgets_project_uniq
  ON zdx_agent_budgets (project_id)
  WHERE project_id IS NOT NULL AND agent_id IS NULL;

CREATE TABLE zdx_budget_pauses (
  id         BIGSERIAL PRIMARY KEY,
  agent_id   TEXT NOT NULL REFERENCES zdx_agents(id)   ON DELETE CASCADE,
  project_id INT           REFERENCES zdx_projects(id) ON DELETE CASCADE,
  reason     TEXT NOT NULL DEFAULT '',
  paused_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  lifted_at  TIMESTAMPTZ,
  lifted_by  TEXT
);
