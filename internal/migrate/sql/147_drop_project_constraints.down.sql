-- One-way migration: row data was migrated into zdx_patterns / zdx_concern_patterns
-- by migration 137 (migrate_constraints_to_patterns) and is NOT recoverable from
-- this rollback. The down migration recreates only the empty schema.

CREATE TABLE zdx_project_constraints (
    id integer NOT NULL,
    project_id integer NOT NULL,
    title text NOT NULL,
    description text DEFAULT ''::text NOT NULL,
    priority integer DEFAULT 1 NOT NULL,
    status text DEFAULT 'active'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);

CREATE SEQUENCE zdx_project_constraints_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE zdx_project_constraints_id_seq OWNED BY zdx_project_constraints.id;

ALTER TABLE ONLY zdx_project_constraints
    ALTER COLUMN id SET DEFAULT nextval('zdx_project_constraints_id_seq'::regclass);

ALTER TABLE ONLY zdx_project_constraints
    ADD CONSTRAINT zdx_project_constraints_pkey PRIMARY KEY (id);

ALTER TABLE ONLY zdx_project_constraints
    ADD CONSTRAINT zdx_project_constraints_project_id_title_key UNIQUE (project_id, title);

ALTER TABLE ONLY zdx_project_constraints
    ADD CONSTRAINT zdx_project_constraints_project_id_fkey
    FOREIGN KEY (project_id) REFERENCES zdx_projects(id);

CREATE INDEX idx_project_constraints_project
    ON zdx_project_constraints USING btree (project_id);
