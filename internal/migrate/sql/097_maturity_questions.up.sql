-- Static question catalog for the maturity questionnaire engine.
CREATE TABLE zdx_maturity_questions (
    key text PRIMARY KEY,
    prompt text NOT NULL,
    answer_type text NOT NULL DEFAULT 'yes_no',
    priority_hint integer NOT NULL DEFAULT 100,
    applicable_classifications text[] NOT NULL DEFAULT '{}'
);

-- Per-project answers. Unique on (project_id, question_key); last answer wins.
CREATE TABLE zdx_maturity_answers (
    id serial PRIMARY KEY,
    project_id integer NOT NULL REFERENCES zdx_projects(id),
    question_key text NOT NULL REFERENCES zdx_maturity_questions(key),
    answer text NOT NULL,
    answered_by text NOT NULL DEFAULT '',
    answered_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT zdx_maturity_answers_project_question_uq UNIQUE (project_id, question_key)
);

CREATE INDEX zdx_maturity_answers_by_project
    ON zdx_maturity_answers (project_id);

-- Seed the static question catalog.
INSERT INTO zdx_maturity_questions (key, prompt, answer_type, priority_hint) VALUES
    ('stores-data',       'Does this project store or manage persistent data?',                           'yes_no', 10),
    ('has-ui',            'Does this project have a user interface?',                                     'yes_no', 20),
    ('is-saas',           'Is this project a SaaS product (multi-tenant, hosted)?',                       'yes_no', 30),
    ('handles-pii',       'Does this project handle personally identifiable information (PII)?',           'yes_no', 40),
    ('exposes-api',       'Does this project expose a public or partner-facing API?',                     'yes_no', 50),
    ('multi-region',      'Is this project deployed across multiple regions?',                            'yes_no', 60),
    ('background-jobs',   'Does this project run background jobs or scheduled tasks?',                    'yes_no', 70);
