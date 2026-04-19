-- Stamped maturity checklist items: persisted per-project records that the
-- questionnaire engine upserts from answers. Status drives whether they
-- surface as solo-queue nudges.
CREATE TABLE zdx_maturity_items (
    id serial PRIMARY KEY,
    project_id integer NOT NULL REFERENCES zdx_projects(id),
    kind text NOT NULL,
    target_type text NOT NULL DEFAULT '',
    target_id integer,
    title text NOT NULL,
    description text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'open',
    justification text NOT NULL DEFAULT '',
    snooze_until timestamptz,
    source_question text NOT NULL DEFAULT '',
    priority_hint integer NOT NULL DEFAULT 100,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT zdx_maturity_items_status_chk
        CHECK (status IN ('open', 'done', 'snoozed', 'dismissed'))
);

-- Dedup by (project, kind, target). COALESCE lets nullable target_id
-- participate in uniqueness (distinct nulls would otherwise bypass it).
CREATE UNIQUE INDEX zdx_maturity_items_dedup
    ON zdx_maturity_items (project_id, kind, target_type, COALESCE(target_id, 0));

CREATE INDEX zdx_maturity_items_by_project_status
    ON zdx_maturity_items (project_id, status);
