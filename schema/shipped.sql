--
-- PostgreSQL database dump
--


-- Dumped from database version 17.9 (Debian 17.9-1.pgdg13+1)
-- Dumped by pg_dump version 18.3 (Ubuntu 18.3-1.pgdg24.04+1)

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET transaction_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: zdx_agents; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_agents (
    id text NOT NULL,
    project_id integer NOT NULL,
    session_id text DEFAULT ''::text NOT NULL,
    worktree_path text DEFAULT ''::text NOT NULL,
    worktree_branch text DEFAULT ''::text NOT NULL,
    pid integer DEFAULT 0 NOT NULL,
    status text DEFAULT 'active'::text NOT NULL,
    task_group text DEFAULT ''::text NOT NULL,
    compose_project text DEFAULT ''::text NOT NULL,
    server_port integer DEFAULT 0 NOT NULL,
    database_url text DEFAULT ''::text NOT NULL,
    last_heartbeat timestamp with time zone DEFAULT now() NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    valkey_url text DEFAULT ''::text NOT NULL
);


--
-- Name: zdx_api_keys; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_api_keys (
    id integer NOT NULL,
    user_id integer NOT NULL,
    token text NOT NULL,
    name text NOT NULL,
    last_used_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: zdx_api_keys_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.zdx_api_keys_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: zdx_api_keys_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.zdx_api_keys_id_seq OWNED BY public.zdx_api_keys.id;


--
-- Name: zdx_blocker_questions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_blocker_questions (
    id integer NOT NULL,
    project_id integer NOT NULL,
    target_type text NOT NULL,
    target_id text NOT NULL,
    context text DEFAULT ''::text NOT NULL,
    choices jsonb DEFAULT '[]'::jsonb NOT NULL,
    answer text DEFAULT ''::text NOT NULL,
    answered_by text DEFAULT ''::text NOT NULL,
    status text DEFAULT 'pending'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    answered_at timestamp with time zone
);


--
-- Name: zdx_blocker_questions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.zdx_blocker_questions_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: zdx_blocker_questions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.zdx_blocker_questions_id_seq OWNED BY public.zdx_blocker_questions.id;


--
-- Name: zdx_claude_events; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_claude_events (
    id bigint NOT NULL,
    session_pk bigint NOT NULL,
    seq integer NOT NULL,
    event_type text DEFAULT ''::text NOT NULL,
    event_json jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    agent_id text DEFAULT ''::text NOT NULL,
    is_sidechain boolean DEFAULT false NOT NULL,
    agent_type text DEFAULT ''::text NOT NULL,
    agent_description text DEFAULT ''::text NOT NULL
);


--
-- Name: zdx_claude_events_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.zdx_claude_events_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: zdx_claude_events_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.zdx_claude_events_id_seq OWNED BY public.zdx_claude_events.id;


--
-- Name: zdx_claude_sessions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_claude_sessions (
    id bigint NOT NULL,
    project_id integer NOT NULL,
    issue_id text DEFAULT ''::text NOT NULL,
    session_id text NOT NULL,
    title text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    alias text DEFAULT ''::text NOT NULL,
    header text DEFAULT ''::text NOT NULL,
    summary text DEFAULT ''::text NOT NULL,
    status text DEFAULT ''::text NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    closed_at timestamp with time zone,
    todo_id integer
);


--
-- Name: zdx_claude_sessions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.zdx_claude_sessions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: zdx_claude_sessions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.zdx_claude_sessions_id_seq OWNED BY public.zdx_claude_sessions.id;


--
-- Name: zdx_code_refs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_code_refs (
    id integer NOT NULL,
    project_id integer NOT NULL,
    file_path text NOT NULL,
    git_hash text DEFAULT ''::text NOT NULL,
    line_start integer DEFAULT 0 NOT NULL,
    line_end integer DEFAULT 0 NOT NULL,
    note text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: zdx_code_refs_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.zdx_code_refs_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: zdx_code_refs_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.zdx_code_refs_id_seq OWNED BY public.zdx_code_refs.id;


--
-- Name: zdx_comment_reactions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_comment_reactions (
    id integer NOT NULL,
    project_id integer NOT NULL,
    comment_id integer NOT NULL,
    emoji text NOT NULL,
    reactor text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: zdx_comment_reactions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.zdx_comment_reactions_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: zdx_comment_reactions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.zdx_comment_reactions_id_seq OWNED BY public.zdx_comment_reactions.id;


--
-- Name: zdx_comment_reads; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_comment_reads (
    id integer NOT NULL,
    project_id integer NOT NULL,
    target_type text NOT NULL,
    target_id text NOT NULL,
    role text NOT NULL,
    last_read_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: zdx_comment_reads_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.zdx_comment_reads_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: zdx_comment_reads_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.zdx_comment_reads_id_seq OWNED BY public.zdx_comment_reads.id;


--
-- Name: zdx_comments; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_comments (
    id integer NOT NULL,
    project_id integer NOT NULL,
    target_type text NOT NULL,
    target_id text NOT NULL,
    body text NOT NULL,
    author text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    parent_id integer,
    author_alias text DEFAULT ''::text NOT NULL
);


--
-- Name: zdx_comments_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.zdx_comments_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: zdx_comments_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.zdx_comments_id_seq OWNED BY public.zdx_comments.id;


--
-- Name: zdx_counted; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_counted (
    id bigint NOT NULL,
    project_id integer,
    component text DEFAULT ''::text NOT NULL,
    environment text DEFAULT ''::text NOT NULL,
    name text NOT NULL,
    value integer NOT NULL,
    source text DEFAULT ''::text NOT NULL,
    context_json jsonb DEFAULT '{}'::jsonb NOT NULL,
    count integer DEFAULT 1 NOT NULL,
    total_value bigint DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: zdx_counted_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.zdx_counted_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: zdx_counted_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.zdx_counted_id_seq OWNED BY public.zdx_counted.id;


--
-- Name: zdx_counter_events; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_counter_events (
    id bigint NOT NULL,
    project_id integer,
    component text DEFAULT ''::text NOT NULL,
    environment text DEFAULT ''::text NOT NULL,
    name text NOT NULL,
    value integer NOT NULL,
    source text DEFAULT ''::text NOT NULL,
    context_json jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: zdx_counter_events_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.zdx_counter_events_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: zdx_counter_events_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.zdx_counter_events_id_seq OWNED BY public.zdx_counter_events.id;


--
-- Name: zdx_deploys; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_deploys (
    id integer NOT NULL,
    environment_id integer NOT NULL,
    build_sha text NOT NULL,
    build_branch text DEFAULT ''::text NOT NULL,
    deployed_at timestamp with time zone DEFAULT now() NOT NULL,
    deployed_by_user_id integer,
    status text DEFAULT 'success'::text NOT NULL
);


--
-- Name: zdx_deploys_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.zdx_deploys_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: zdx_deploys_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.zdx_deploys_id_seq OWNED BY public.zdx_deploys.id;


--
-- Name: zdx_doctor_deferrals; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_doctor_deferrals (
    id integer NOT NULL,
    project_id integer NOT NULL,
    check_name text NOT NULL,
    rung text DEFAULT ''::text NOT NULL,
    deferred_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: zdx_doctor_deferrals_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.zdx_doctor_deferrals_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: zdx_doctor_deferrals_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.zdx_doctor_deferrals_id_seq OWNED BY public.zdx_doctor_deferrals.id;


--
-- Name: zdx_environments; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_environments (
    id integer NOT NULL,
    project_id integer NOT NULL,
    name text NOT NULL,
    url text DEFAULT ''::text NOT NULL,
    current_build_sha text DEFAULT ''::text NOT NULL,
    current_build_branch text DEFAULT ''::text NOT NULL,
    deployed_at timestamp with time zone,
    deployed_by_user_id integer,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: zdx_environments_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.zdx_environments_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: zdx_environments_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.zdx_environments_id_seq OWNED BY public.zdx_environments.id;


--
-- Name: zdx_error_events; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_error_events (
    id bigint NOT NULL,
    project_id integer,
    component text DEFAULT ''::text NOT NULL,
    environment text DEFAULT ''::text NOT NULL,
    name text NOT NULL,
    message text DEFAULT ''::text NOT NULL,
    stack_trace text DEFAULT ''::text NOT NULL,
    source text DEFAULT ''::text NOT NULL,
    context_json jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: zdx_error_events_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.zdx_error_events_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: zdx_error_events_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.zdx_error_events_id_seq OWNED BY public.zdx_error_events.id;


--
-- Name: zdx_error_reports; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_error_reports (
    id bigint NOT NULL,
    project_id integer,
    source text NOT NULL,
    endpoint text DEFAULT ''::text NOT NULL,
    error_name text DEFAULT ''::text NOT NULL,
    stack_trace text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: zdx_error_reports_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.zdx_error_reports_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: zdx_error_reports_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.zdx_error_reports_id_seq OWNED BY public.zdx_error_reports.id;


--
-- Name: zdx_feature_multipliers; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_feature_multipliers (
    feature_id integer NOT NULL,
    multiplies_feature_id integer NOT NULL,
    CONSTRAINT zdx_feature_multipliers_check CHECK ((feature_id <> multiplies_feature_id))
);


--
-- Name: zdx_features; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_features (
    id integer NOT NULL,
    project_id integer NOT NULL,
    name text NOT NULL,
    description text DEFAULT ''::text NOT NULL,
    what text DEFAULT ''::text NOT NULL,
    why text DEFAULT ''::text NOT NULL,
    done_when text DEFAULT ''::text NOT NULL,
    component text DEFAULT ''::text NOT NULL,
    category text DEFAULT ''::text NOT NULL,
    last_reviewed_at timestamp with time zone,
    parent_feature_id integer,
    kind text DEFAULT 'direct'::text NOT NULL,
    goal_id integer,
    metric_name text DEFAULT ''::text NOT NULL,
    metric_unit text DEFAULT ''::text NOT NULL,
    baseline_value text DEFAULT ''::text NOT NULL,
    target_value text DEFAULT ''::text NOT NULL,
    graph_url text DEFAULT ''::text NOT NULL
);


--
-- Name: zdx_features_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.zdx_features_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: zdx_features_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.zdx_features_id_seq OWNED BY public.zdx_features.id;


--
-- Name: zdx_files; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_files (
    id integer NOT NULL,
    provider text NOT NULL,
    path text NOT NULL,
    mime_type text DEFAULT ''::text NOT NULL,
    size_bytes bigint DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: zdx_files_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.zdx_files_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: zdx_files_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.zdx_files_id_seq OWNED BY public.zdx_files.id;


--
-- Name: zdx_focus_blockers; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_focus_blockers (
    focus_id integer NOT NULL,
    issue_id text NOT NULL
);


--
-- Name: zdx_focus_features; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_focus_features (
    focus_id integer NOT NULL,
    feature_id integer NOT NULL
);


--
-- Name: zdx_focuses; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_focuses (
    id integer NOT NULL,
    project_id integer NOT NULL,
    name text NOT NULL,
    description text DEFAULT ''::text NOT NULL,
    priority integer DEFAULT 2 NOT NULL,
    status text DEFAULT 'active'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    started_at timestamp with time zone,
    ended_at timestamp with time zone
);


--
-- Name: zdx_focuses_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.zdx_focuses_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: zdx_focuses_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.zdx_focuses_id_seq OWNED BY public.zdx_focuses.id;


--
-- Name: zdx_goal_issues; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_goal_issues (
    goal_id integer NOT NULL,
    issue_id text NOT NULL
);


--
-- Name: zdx_id_seq; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_id_seq (
    kind text NOT NULL,
    next_val integer DEFAULT 1 NOT NULL
);


--
-- Name: zdx_integration_token; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_integration_token (
    id integer NOT NULL,
    project_id integer NOT NULL,
    component text,
    name text DEFAULT ''::text NOT NULL,
    token_hash text NOT NULL,
    token_prefix text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    revoked_at timestamp with time zone
);


--
-- Name: zdx_integration_token_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.zdx_integration_token_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: zdx_integration_token_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.zdx_integration_token_id_seq OWNED BY public.zdx_integration_token.id;


--
-- Name: zdx_invites; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_invites (
    id integer NOT NULL,
    email text NOT NULL,
    token text NOT NULL,
    invited_by integer NOT NULL,
    expires_at timestamp with time zone DEFAULT (now() + '7 days'::interval) NOT NULL,
    used_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: zdx_invites_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.zdx_invites_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: zdx_invites_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.zdx_invites_id_seq OWNED BY public.zdx_invites.id;


--
-- Name: zdx_issue_blocks; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_issue_blocks (
    issue_id text NOT NULL,
    blocked_by_id text NOT NULL
);


--
-- Name: zdx_issue_code_refs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_issue_code_refs (
    issue_id text NOT NULL,
    code_ref_id integer NOT NULL
);


--
-- Name: zdx_issue_features; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_issue_features (
    issue_id text NOT NULL,
    feature_id integer NOT NULL
);


--
-- Name: zdx_issue_files; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_issue_files (
    id integer NOT NULL,
    issue_id text NOT NULL,
    file_id integer NOT NULL,
    kind text DEFAULT 'attachment'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: zdx_issue_files_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.zdx_issue_files_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: zdx_issue_files_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.zdx_issue_files_id_seq OWNED BY public.zdx_issue_files.id;


--
-- Name: zdx_issue_resolution_commits; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_issue_resolution_commits (
    resolution_id text NOT NULL,
    sha text NOT NULL,
    ord integer DEFAULT 0 NOT NULL
);


--
-- Name: zdx_issue_resolutions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_issue_resolutions (
    id text NOT NULL,
    issue_id text NOT NULL,
    branch_of_origin text DEFAULT ''::text NOT NULL,
    resolved_at timestamp with time zone DEFAULT now() NOT NULL,
    author text DEFAULT ''::text NOT NULL,
    source text DEFAULT 'manual'::text NOT NULL,
    parent_resolution_id text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT zdx_issue_resolutions_source_check CHECK ((source = ANY (ARRAY['manual'::text, 'reconciled'::text, 'merged'::text])))
);


--
-- Name: zdx_issue_work; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_issue_work (
    id integer NOT NULL,
    issue_id text NOT NULL,
    agent text NOT NULL,
    note text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: zdx_issue_work_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.zdx_issue_work_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: zdx_issue_work_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.zdx_issue_work_id_seq OWNED BY public.zdx_issue_work.id;


--
-- Name: zdx_issues; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_issues (
    id text NOT NULL,
    project_id integer NOT NULL,
    title text NOT NULL,
    status text DEFAULT 'open'::text NOT NULL,
    priority text DEFAULT ''::text NOT NULL,
    component text DEFAULT ''::text NOT NULL,
    context text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    issue_type text DEFAULT 'unknown'::text NOT NULL,
    duplicate_of text DEFAULT ''::text NOT NULL,
    url text DEFAULT ''::text NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    source_error_id bigint,
    link_of text DEFAULT ''::text NOT NULL,
    reopen_count integer DEFAULT 0 NOT NULL
);


--
-- Name: zdx_journal_entries; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_journal_entries (
    id integer NOT NULL,
    project_id integer NOT NULL,
    role text NOT NULL,
    date text NOT NULL,
    baseline boolean DEFAULT false NOT NULL,
    tldr text DEFAULT ''::text NOT NULL,
    assessment text DEFAULT ''::text NOT NULL,
    concerns text DEFAULT ''::text NOT NULL,
    next text DEFAULT ''::text NOT NULL,
    changelog_json text DEFAULT '{}'::text NOT NULL,
    state_json text DEFAULT '{}'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    needs_review boolean DEFAULT false NOT NULL
);


--
-- Name: zdx_journal_entries_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.zdx_journal_entries_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: zdx_journal_entries_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.zdx_journal_entries_id_seq OWNED BY public.zdx_journal_entries.id;


--
-- Name: zdx_llm_configs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_llm_configs (
    type text DEFAULT 'openai'::text NOT NULL,
    url text DEFAULT ''::text NOT NULL,
    embedding_model text,
    api_key text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    agent_type text DEFAULT 'openai'::text NOT NULL,
    model_low text,
    model_medium text,
    model_high text,
    id bigint NOT NULL,
    name text DEFAULT 'default'::text NOT NULL,
    priority integer NOT NULL,
    CONSTRAINT zdx_llm_configs_claude_no_embedding CHECK (((agent_type <> 'claude'::text) OR (embedding_model IS NULL)))
);


--
-- Name: zdx_llm_configs_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.zdx_llm_configs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: zdx_llm_configs_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.zdx_llm_configs_id_seq OWNED BY public.zdx_llm_configs.id;


--
-- Name: zdx_log_events; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_log_events (
    id bigint NOT NULL,
    project_id integer,
    component text DEFAULT ''::text NOT NULL,
    environment text DEFAULT ''::text NOT NULL,
    level text DEFAULT 'info'::text NOT NULL,
    message text NOT NULL,
    source text DEFAULT ''::text NOT NULL,
    context_json jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: zdx_log_events_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.zdx_log_events_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: zdx_log_events_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.zdx_log_events_id_seq OWNED BY public.zdx_log_events.id;


--
-- Name: zdx_maturity_answers; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_maturity_answers (
    id integer NOT NULL,
    project_id integer NOT NULL,
    question_key text NOT NULL,
    answer text NOT NULL,
    answered_by text DEFAULT ''::text NOT NULL,
    answered_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: zdx_maturity_answers_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.zdx_maturity_answers_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: zdx_maturity_answers_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.zdx_maturity_answers_id_seq OWNED BY public.zdx_maturity_answers.id;


--
-- Name: zdx_maturity_items; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_maturity_items (
    id integer NOT NULL,
    project_id integer NOT NULL,
    kind text NOT NULL,
    target_type text DEFAULT ''::text NOT NULL,
    target_id integer,
    title text NOT NULL,
    description text DEFAULT ''::text NOT NULL,
    status text DEFAULT 'open'::text NOT NULL,
    justification text DEFAULT ''::text NOT NULL,
    snooze_until timestamp with time zone,
    source_question text DEFAULT ''::text NOT NULL,
    priority_hint integer DEFAULT 100 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT zdx_maturity_items_status_chk CHECK ((status = ANY (ARRAY['open'::text, 'done'::text, 'snoozed'::text, 'dismissed'::text])))
);


--
-- Name: zdx_maturity_items_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.zdx_maturity_items_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: zdx_maturity_items_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.zdx_maturity_items_id_seq OWNED BY public.zdx_maturity_items.id;


--
-- Name: zdx_maturity_questions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_maturity_questions (
    key text NOT NULL,
    prompt text NOT NULL,
    answer_type text DEFAULT 'yes_no'::text NOT NULL,
    priority_hint integer DEFAULT 100 NOT NULL,
    applicable_classifications text[] DEFAULT '{}'::text[] NOT NULL
);


--
-- Name: zdx_oauth_identities; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_oauth_identities (
    id integer NOT NULL,
    user_id integer NOT NULL,
    provider text NOT NULL,
    sub text NOT NULL,
    email text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: zdx_oauth_identities_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.zdx_oauth_identities_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: zdx_oauth_identities_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.zdx_oauth_identities_id_seq OWNED BY public.zdx_oauth_identities.id;


--
-- Name: zdx_oauth_states; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_oauth_states (
    state text NOT NULL,
    provider text NOT NULL,
    code_verifier text DEFAULT ''::text NOT NULL,
    redirect_to text DEFAULT '/'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    expires_at timestamp with time zone DEFAULT (now() + '00:10:00'::interval) NOT NULL
);


--
-- Name: zdx_patterns; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_patterns (
    id integer NOT NULL,
    project_id integer NOT NULL,
    name text NOT NULL,
    description text DEFAULT ''::text NOT NULL,
    code_refs jsonb DEFAULT '[]'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: zdx_patterns_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.zdx_patterns_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: zdx_patterns_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.zdx_patterns_id_seq OWNED BY public.zdx_patterns.id;


--
-- Name: zdx_plan_step_refs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_plan_step_refs (
    step_id integer NOT NULL,
    target_type text NOT NULL,
    target_id text NOT NULL
);


--
-- Name: zdx_plan_steps; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_plan_steps (
    id integer NOT NULL,
    plan_id integer NOT NULL,
    seq integer DEFAULT 0 NOT NULL,
    text text DEFAULT ''::text NOT NULL,
    status text DEFAULT 'pending'::text NOT NULL,
    depends_on integer,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: zdx_plan_steps_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.zdx_plan_steps_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: zdx_plan_steps_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.zdx_plan_steps_id_seq OWNED BY public.zdx_plan_steps.id;


--
-- Name: zdx_plans; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_plans (
    id integer NOT NULL,
    feature_id integer,
    plan_type text DEFAULT 'implement'::text NOT NULL,
    status text DEFAULT 'pending'::text NOT NULL,
    complexity text DEFAULT ''::text NOT NULL,
    approach text DEFAULT ''::text NOT NULL,
    last_reviewed_at timestamp with time zone,
    project_id integer NOT NULL,
    title text DEFAULT ''::text NOT NULL,
    body text DEFAULT ''::text NOT NULL,
    focus_id integer,
    issue_id text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: zdx_plans_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.zdx_plans_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: zdx_plans_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.zdx_plans_id_seq OWNED BY public.zdx_plans.id;


--
-- Name: zdx_project_constraints; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_project_constraints (
    id integer NOT NULL,
    project_id integer NOT NULL,
    title text NOT NULL,
    description text DEFAULT ''::text NOT NULL,
    priority integer DEFAULT 1 NOT NULL,
    status text DEFAULT 'active'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: zdx_project_constraints_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.zdx_project_constraints_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: zdx_project_constraints_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.zdx_project_constraints_id_seq OWNED BY public.zdx_project_constraints.id;


--
-- Name: zdx_project_git_config; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_project_git_config (
    id integer NOT NULL,
    project_id integer NOT NULL,
    clone_url text DEFAULT ''::text NOT NULL,
    auth_type text DEFAULT 'none'::text NOT NULL,
    auth_token text DEFAULT ''::text NOT NULL
);


--
-- Name: zdx_project_git_config_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.zdx_project_git_config_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: zdx_project_git_config_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.zdx_project_git_config_id_seq OWNED BY public.zdx_project_git_config.id;


--
-- Name: zdx_project_goals; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_project_goals (
    id integer NOT NULL,
    project_id integer NOT NULL,
    title text NOT NULL,
    description text DEFAULT ''::text NOT NULL,
    priority integer DEFAULT 1 NOT NULL,
    status text DEFAULT 'active'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    metric_name text DEFAULT ''::text NOT NULL,
    metric_unit text DEFAULT ''::text NOT NULL
);


--
-- Name: zdx_project_goals_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.zdx_project_goals_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: zdx_project_goals_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.zdx_project_goals_id_seq OWNED BY public.zdx_project_goals.id;


--
-- Name: zdx_project_permissions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_project_permissions (
    id integer NOT NULL,
    user_id integer NOT NULL,
    project_id integer NOT NULL,
    role text DEFAULT 'member'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: zdx_project_permissions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.zdx_project_permissions_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: zdx_project_permissions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.zdx_project_permissions_id_seq OWNED BY public.zdx_project_permissions.id;


--
-- Name: zdx_projects; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_projects (
    id integer NOT NULL,
    slug text NOT NULL,
    name text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    git_url text DEFAULT ''::text NOT NULL,
    git_branch text DEFAULT 'main'::text NOT NULL,
    git_token text DEFAULT ''::text NOT NULL,
    stage text DEFAULT ''::text NOT NULL,
    classification text DEFAULT ''::text NOT NULL,
    upstream_url text DEFAULT ''::text NOT NULL,
    upstream_credentials text DEFAULT ''::text NOT NULL,
    git_enabled boolean DEFAULT false NOT NULL
);


--
-- Name: zdx_projects_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.zdx_projects_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: zdx_projects_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.zdx_projects_id_seq OWNED BY public.zdx_projects.id;


--
-- Name: zdx_question_proposals; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_question_proposals (
    id integer NOT NULL,
    project_id integer NOT NULL,
    question_id integer NOT NULL,
    question_type text DEFAULT 'qa'::text NOT NULL,
    title text NOT NULL,
    context text DEFAULT ''::text NOT NULL,
    status text DEFAULT 'proposed'::text NOT NULL,
    denied_reason text DEFAULT ''::text NOT NULL,
    created_issue_id text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: zdx_question_proposals_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.zdx_question_proposals_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: zdx_question_proposals_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.zdx_question_proposals_id_seq OWNED BY public.zdx_question_proposals.id;


--
-- Name: zdx_questions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_questions (
    id integer NOT NULL,
    project_id integer NOT NULL,
    category text DEFAULT ''::text NOT NULL,
    question text NOT NULL,
    answer text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    parent_question_id integer
);


--
-- Name: zdx_questions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.zdx_questions_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: zdx_questions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.zdx_questions_id_seq OWNED BY public.zdx_questions.id;


--
-- Name: zdx_reservations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_reservations (
    id bigint NOT NULL,
    project_id integer NOT NULL,
    target_type text NOT NULL,
    target_id text NOT NULL,
    claimed_by text DEFAULT ''::text NOT NULL,
    claimed_at timestamp with time zone DEFAULT now() NOT NULL,
    released_at timestamp with time zone,
    lease_expires_at timestamp with time zone
);


--
-- Name: zdx_reservations_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.zdx_reservations_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: zdx_reservations_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.zdx_reservations_id_seq OWNED BY public.zdx_reservations.id;


--
-- Name: zdx_revisions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_revisions (
    id integer NOT NULL,
    project_id integer NOT NULL,
    target_type text NOT NULL,
    target_id text NOT NULL,
    field text NOT NULL,
    old_val text DEFAULT ''::text NOT NULL,
    new_val text DEFAULT ''::text NOT NULL,
    agent text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    session_id text DEFAULT ''::text NOT NULL,
    user_id text DEFAULT ''::text NOT NULL
);


--
-- Name: zdx_revisions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.zdx_revisions_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: zdx_revisions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.zdx_revisions_id_seq OWNED BY public.zdx_revisions.id;


--
-- Name: zdx_sessions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_sessions (
    id integer NOT NULL,
    user_id integer NOT NULL,
    token text NOT NULL,
    expires_at timestamp with time zone DEFAULT (now() + '30 days'::interval) NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: zdx_sessions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.zdx_sessions_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: zdx_sessions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.zdx_sessions_id_seq OWNED BY public.zdx_sessions.id;


--
-- Name: zdx_slow_queries; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_slow_queries (
    id bigint NOT NULL,
    project_id integer,
    sql_hash text NOT NULL,
    sql_text text NOT NULL,
    endpoint text DEFAULT ''::text NOT NULL,
    duration_ms integer NOT NULL,
    explain_json text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: zdx_slow_queries_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.zdx_slow_queries_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: zdx_slow_queries_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.zdx_slow_queries_id_seq OWNED BY public.zdx_slow_queries.id;


--
-- Name: zdx_spec_code_refs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_spec_code_refs (
    spec_id integer NOT NULL,
    code_ref_id integer NOT NULL
);


--
-- Name: zdx_spec_issues; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_spec_issues (
    spec_id integer NOT NULL,
    issue_id text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: zdx_spec_tests; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_spec_tests (
    spec_id integer NOT NULL,
    test_id integer NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: zdx_specs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_specs (
    id integer NOT NULL,
    feature_id integer NOT NULL,
    description text NOT NULL,
    kind text DEFAULT 'must'::text NOT NULL,
    deferred boolean DEFAULT false NOT NULL,
    deferred_reason text DEFAULT ''::text NOT NULL,
    concern_type text DEFAULT 'functional'::text NOT NULL
);


--
-- Name: zdx_specs_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.zdx_specs_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: zdx_specs_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.zdx_specs_id_seq OWNED BY public.zdx_specs.id;


--
-- Name: zdx_sprints; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_sprints (
    id integer NOT NULL,
    project_id integer NOT NULL,
    last_owner_review timestamp with time zone,
    last_tech_review timestamp with time zone,
    last_owner_journal timestamp with time zone,
    last_tech_journal timestamp with time zone
);


--
-- Name: zdx_sprints_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.zdx_sprints_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: zdx_sprints_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.zdx_sprints_id_seq OWNED BY public.zdx_sprints.id;


--
-- Name: zdx_state; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_state (
    project_id integer NOT NULL,
    key text NOT NULL,
    value text DEFAULT ''::text NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: zdx_task_code_refs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_task_code_refs (
    task_id text NOT NULL,
    code_ref_id integer NOT NULL
);


--
-- Name: zdx_task_reviews; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_task_reviews (
    id integer NOT NULL,
    project_id integer NOT NULL,
    task_id text NOT NULL,
    reviewer_role text DEFAULT 'reviewer'::text NOT NULL,
    reviewer_user_id integer,
    verdict text DEFAULT ''::text NOT NULL,
    body text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: zdx_task_reviews_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.zdx_task_reviews_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: zdx_task_reviews_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.zdx_task_reviews_id_seq OWNED BY public.zdx_task_reviews.id;


--
-- Name: zdx_tasks; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_tasks (
    id text NOT NULL,
    project_id integer NOT NULL,
    text text NOT NULL,
    feature text DEFAULT ''::text NOT NULL,
    status text DEFAULT 'wip'::text NOT NULL,
    reason text DEFAULT ''::text NOT NULL,
    issue text DEFAULT ''::text NOT NULL,
    depends text DEFAULT ''::text NOT NULL,
    test_plan text DEFAULT ''::text NOT NULL,
    test_refs text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    completed_at timestamp with time zone,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    task_group text DEFAULT ''::text NOT NULL,
    reviewed_at timestamp with time zone,
    stale_since timestamp with time zone,
    title text DEFAULT ''::text NOT NULL
);


--
-- Name: zdx_test_code_refs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_test_code_refs (
    test_id integer NOT NULL,
    code_ref_id integer NOT NULL
);


--
-- Name: zdx_test_demos; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_test_demos (
    id integer NOT NULL,
    test_id integer NOT NULL,
    demo_type text NOT NULL,
    artifact_path text NOT NULL,
    file_id integer,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    recorded_branch text DEFAULT ''::text NOT NULL,
    recorded_sha text DEFAULT ''::text NOT NULL
);


--
-- Name: zdx_test_demos_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.zdx_test_demos_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: zdx_test_demos_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.zdx_test_demos_id_seq OWNED BY public.zdx_test_demos.id;


--
-- Name: zdx_test_result_history; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_test_result_history (
    id integer NOT NULL,
    project_id integer NOT NULL,
    driver text NOT NULL,
    test_name text NOT NULL,
    feature text DEFAULT ''::text NOT NULL,
    status text NOT NULL,
    duration_ms integer DEFAULT 0 NOT NULL,
    run_at timestamp with time zone DEFAULT now() NOT NULL,
    branch text DEFAULT ''::text NOT NULL,
    git_sha text DEFAULT ''::text NOT NULL
);


--
-- Name: zdx_test_result_history_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.zdx_test_result_history_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: zdx_test_result_history_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.zdx_test_result_history_id_seq OWNED BY public.zdx_test_result_history.id;


--
-- Name: zdx_test_results; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_test_results (
    id integer NOT NULL,
    project_id integer NOT NULL,
    driver text NOT NULL,
    test_name text NOT NULL,
    feature text DEFAULT ''::text NOT NULL,
    status text NOT NULL,
    duration_ms integer DEFAULT 0 NOT NULL,
    run_at timestamp with time zone DEFAULT now() NOT NULL,
    branch text DEFAULT ''::text NOT NULL,
    git_sha text DEFAULT ''::text NOT NULL
);


--
-- Name: zdx_test_results_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.zdx_test_results_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: zdx_test_results_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.zdx_test_results_id_seq OWNED BY public.zdx_test_results.id;


--
-- Name: zdx_tests; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_tests (
    id integer NOT NULL,
    project_id integer NOT NULL,
    component text NOT NULL,
    name text NOT NULL,
    layer text DEFAULT 'integration'::text NOT NULL,
    status text DEFAULT 'unknown'::text NOT NULL,
    last_run_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    duration_ms integer DEFAULT 0 NOT NULL
);


--
-- Name: zdx_tests_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.zdx_tests_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: zdx_tests_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.zdx_tests_id_seq OWNED BY public.zdx_tests.id;


--
-- Name: zdx_timed; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_timed (
    id bigint NOT NULL,
    project_id integer,
    name text NOT NULL,
    duration_ms integer NOT NULL,
    source text DEFAULT ''::text NOT NULL,
    context_json jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    count integer DEFAULT 1 NOT NULL,
    total_ms bigint DEFAULT 0 NOT NULL,
    component text DEFAULT ''::text NOT NULL,
    environment text DEFAULT ''::text NOT NULL
);


--
-- Name: zdx_timed_events; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_timed_events (
    id bigint NOT NULL,
    project_id integer,
    component text DEFAULT ''::text NOT NULL,
    environment text DEFAULT ''::text NOT NULL,
    name text NOT NULL,
    duration_ms integer NOT NULL,
    source text DEFAULT ''::text NOT NULL,
    context_json jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: zdx_timed_events_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.zdx_timed_events_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: zdx_timed_events_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.zdx_timed_events_id_seq OWNED BY public.zdx_timed_events.id;


--
-- Name: zdx_timed_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.zdx_timed_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: zdx_timed_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.zdx_timed_id_seq OWNED BY public.zdx_timed.id;


--
-- Name: zdx_todos; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_todos (
    id integer NOT NULL,
    project_id integer NOT NULL,
    text text NOT NULL,
    key text NOT NULL,
    persona text DEFAULT ''::text NOT NULL,
    priority integer DEFAULT 50 NOT NULL,
    status text DEFAULT 'open'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    resolved_at timestamp with time zone,
    target_type text DEFAULT ''::text NOT NULL,
    target_id text DEFAULT ''::text NOT NULL,
    kind text DEFAULT ''::text NOT NULL,
    issue_ref text DEFAULT ''::text NOT NULL,
    blocked boolean DEFAULT false NOT NULL,
    claimed_by text DEFAULT ''::text NOT NULL,
    claimed_at timestamp with time zone,
    lease_expires_at timestamp with time zone,
    reopen_count integer DEFAULT 0 NOT NULL,
    title text DEFAULT ''::text NOT NULL,
    description text DEFAULT ''::text NOT NULL
);


--
-- Name: zdx_todos_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.zdx_todos_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: zdx_todos_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.zdx_todos_id_seq OWNED BY public.zdx_todos.id;


--
-- Name: zdx_users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_users (
    id integer NOT NULL,
    email text NOT NULL,
    name text NOT NULL,
    password_hash text DEFAULT ''::text NOT NULL,
    role text DEFAULT 'member'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: zdx_users_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.zdx_users_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: zdx_users_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.zdx_users_id_seq OWNED BY public.zdx_users.id;


--
-- Name: zdx_work_log; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_work_log (
    id integer NOT NULL,
    issue_id text NOT NULL,
    entry_type text NOT NULL,
    by_role text NOT NULL,
    note text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: zdx_work_log_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.zdx_work_log_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: zdx_work_log_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.zdx_work_log_id_seq OWNED BY public.zdx_work_log.id;


--
-- Name: zdx_api_keys id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_api_keys ALTER COLUMN id SET DEFAULT nextval('public.zdx_api_keys_id_seq'::regclass);


--
-- Name: zdx_blocker_questions id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_blocker_questions ALTER COLUMN id SET DEFAULT nextval('public.zdx_blocker_questions_id_seq'::regclass);


--
-- Name: zdx_claude_events id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_claude_events ALTER COLUMN id SET DEFAULT nextval('public.zdx_claude_events_id_seq'::regclass);


--
-- Name: zdx_claude_sessions id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_claude_sessions ALTER COLUMN id SET DEFAULT nextval('public.zdx_claude_sessions_id_seq'::regclass);


--
-- Name: zdx_code_refs id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_code_refs ALTER COLUMN id SET DEFAULT nextval('public.zdx_code_refs_id_seq'::regclass);


--
-- Name: zdx_comment_reactions id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_comment_reactions ALTER COLUMN id SET DEFAULT nextval('public.zdx_comment_reactions_id_seq'::regclass);


--
-- Name: zdx_comment_reads id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_comment_reads ALTER COLUMN id SET DEFAULT nextval('public.zdx_comment_reads_id_seq'::regclass);


--
-- Name: zdx_comments id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_comments ALTER COLUMN id SET DEFAULT nextval('public.zdx_comments_id_seq'::regclass);


--
-- Name: zdx_counted id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_counted ALTER COLUMN id SET DEFAULT nextval('public.zdx_counted_id_seq'::regclass);


--
-- Name: zdx_counter_events id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_counter_events ALTER COLUMN id SET DEFAULT nextval('public.zdx_counter_events_id_seq'::regclass);


--
-- Name: zdx_deploys id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_deploys ALTER COLUMN id SET DEFAULT nextval('public.zdx_deploys_id_seq'::regclass);


--
-- Name: zdx_doctor_deferrals id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_doctor_deferrals ALTER COLUMN id SET DEFAULT nextval('public.zdx_doctor_deferrals_id_seq'::regclass);


--
-- Name: zdx_environments id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_environments ALTER COLUMN id SET DEFAULT nextval('public.zdx_environments_id_seq'::regclass);


--
-- Name: zdx_error_events id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_error_events ALTER COLUMN id SET DEFAULT nextval('public.zdx_error_events_id_seq'::regclass);


--
-- Name: zdx_error_reports id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_error_reports ALTER COLUMN id SET DEFAULT nextval('public.zdx_error_reports_id_seq'::regclass);


--
-- Name: zdx_features id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_features ALTER COLUMN id SET DEFAULT nextval('public.zdx_features_id_seq'::regclass);


--
-- Name: zdx_files id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_files ALTER COLUMN id SET DEFAULT nextval('public.zdx_files_id_seq'::regclass);


--
-- Name: zdx_focuses id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_focuses ALTER COLUMN id SET DEFAULT nextval('public.zdx_focuses_id_seq'::regclass);


--
-- Name: zdx_integration_token id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_integration_token ALTER COLUMN id SET DEFAULT nextval('public.zdx_integration_token_id_seq'::regclass);


--
-- Name: zdx_invites id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_invites ALTER COLUMN id SET DEFAULT nextval('public.zdx_invites_id_seq'::regclass);


--
-- Name: zdx_issue_files id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_issue_files ALTER COLUMN id SET DEFAULT nextval('public.zdx_issue_files_id_seq'::regclass);


--
-- Name: zdx_issue_work id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_issue_work ALTER COLUMN id SET DEFAULT nextval('public.zdx_issue_work_id_seq'::regclass);


--
-- Name: zdx_journal_entries id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_journal_entries ALTER COLUMN id SET DEFAULT nextval('public.zdx_journal_entries_id_seq'::regclass);


--
-- Name: zdx_llm_configs id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_llm_configs ALTER COLUMN id SET DEFAULT nextval('public.zdx_llm_configs_id_seq'::regclass);


--
-- Name: zdx_log_events id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_log_events ALTER COLUMN id SET DEFAULT nextval('public.zdx_log_events_id_seq'::regclass);


--
-- Name: zdx_maturity_answers id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_maturity_answers ALTER COLUMN id SET DEFAULT nextval('public.zdx_maturity_answers_id_seq'::regclass);


--
-- Name: zdx_maturity_items id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_maturity_items ALTER COLUMN id SET DEFAULT nextval('public.zdx_maturity_items_id_seq'::regclass);


--
-- Name: zdx_oauth_identities id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_oauth_identities ALTER COLUMN id SET DEFAULT nextval('public.zdx_oauth_identities_id_seq'::regclass);


--
-- Name: zdx_patterns id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_patterns ALTER COLUMN id SET DEFAULT nextval('public.zdx_patterns_id_seq'::regclass);


--
-- Name: zdx_plan_steps id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_plan_steps ALTER COLUMN id SET DEFAULT nextval('public.zdx_plan_steps_id_seq'::regclass);


--
-- Name: zdx_plans id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_plans ALTER COLUMN id SET DEFAULT nextval('public.zdx_plans_id_seq'::regclass);


--
-- Name: zdx_project_constraints id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_project_constraints ALTER COLUMN id SET DEFAULT nextval('public.zdx_project_constraints_id_seq'::regclass);


--
-- Name: zdx_project_git_config id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_project_git_config ALTER COLUMN id SET DEFAULT nextval('public.zdx_project_git_config_id_seq'::regclass);


--
-- Name: zdx_project_goals id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_project_goals ALTER COLUMN id SET DEFAULT nextval('public.zdx_project_goals_id_seq'::regclass);


--
-- Name: zdx_project_permissions id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_project_permissions ALTER COLUMN id SET DEFAULT nextval('public.zdx_project_permissions_id_seq'::regclass);


--
-- Name: zdx_projects id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_projects ALTER COLUMN id SET DEFAULT nextval('public.zdx_projects_id_seq'::regclass);


--
-- Name: zdx_question_proposals id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_question_proposals ALTER COLUMN id SET DEFAULT nextval('public.zdx_question_proposals_id_seq'::regclass);


--
-- Name: zdx_questions id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_questions ALTER COLUMN id SET DEFAULT nextval('public.zdx_questions_id_seq'::regclass);


--
-- Name: zdx_reservations id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_reservations ALTER COLUMN id SET DEFAULT nextval('public.zdx_reservations_id_seq'::regclass);


--
-- Name: zdx_revisions id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_revisions ALTER COLUMN id SET DEFAULT nextval('public.zdx_revisions_id_seq'::regclass);


--
-- Name: zdx_sessions id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_sessions ALTER COLUMN id SET DEFAULT nextval('public.zdx_sessions_id_seq'::regclass);


--
-- Name: zdx_slow_queries id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_slow_queries ALTER COLUMN id SET DEFAULT nextval('public.zdx_slow_queries_id_seq'::regclass);


--
-- Name: zdx_specs id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_specs ALTER COLUMN id SET DEFAULT nextval('public.zdx_specs_id_seq'::regclass);


--
-- Name: zdx_sprints id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_sprints ALTER COLUMN id SET DEFAULT nextval('public.zdx_sprints_id_seq'::regclass);


--
-- Name: zdx_task_reviews id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_task_reviews ALTER COLUMN id SET DEFAULT nextval('public.zdx_task_reviews_id_seq'::regclass);


--
-- Name: zdx_test_demos id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_test_demos ALTER COLUMN id SET DEFAULT nextval('public.zdx_test_demos_id_seq'::regclass);


--
-- Name: zdx_test_result_history id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_test_result_history ALTER COLUMN id SET DEFAULT nextval('public.zdx_test_result_history_id_seq'::regclass);


--
-- Name: zdx_test_results id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_test_results ALTER COLUMN id SET DEFAULT nextval('public.zdx_test_results_id_seq'::regclass);


--
-- Name: zdx_tests id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_tests ALTER COLUMN id SET DEFAULT nextval('public.zdx_tests_id_seq'::regclass);


--
-- Name: zdx_timed id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_timed ALTER COLUMN id SET DEFAULT nextval('public.zdx_timed_id_seq'::regclass);


--
-- Name: zdx_timed_events id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_timed_events ALTER COLUMN id SET DEFAULT nextval('public.zdx_timed_events_id_seq'::regclass);


--
-- Name: zdx_todos id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_todos ALTER COLUMN id SET DEFAULT nextval('public.zdx_todos_id_seq'::regclass);


--
-- Name: zdx_users id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_users ALTER COLUMN id SET DEFAULT nextval('public.zdx_users_id_seq'::regclass);


--
-- Name: zdx_work_log id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_work_log ALTER COLUMN id SET DEFAULT nextval('public.zdx_work_log_id_seq'::regclass);


--
-- Name: zdx_agents zdx_agents_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_agents
    ADD CONSTRAINT zdx_agents_pkey PRIMARY KEY (id);


--
-- Name: zdx_api_keys zdx_api_keys_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_api_keys
    ADD CONSTRAINT zdx_api_keys_pkey PRIMARY KEY (id);


--
-- Name: zdx_api_keys zdx_api_keys_token_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_api_keys
    ADD CONSTRAINT zdx_api_keys_token_key UNIQUE (token);


--
-- Name: zdx_blocker_questions zdx_blocker_questions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_blocker_questions
    ADD CONSTRAINT zdx_blocker_questions_pkey PRIMARY KEY (id);


--
-- Name: zdx_claude_events zdx_claude_events_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_claude_events
    ADD CONSTRAINT zdx_claude_events_pkey PRIMARY KEY (id);


--
-- Name: zdx_claude_sessions zdx_claude_sessions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_claude_sessions
    ADD CONSTRAINT zdx_claude_sessions_pkey PRIMARY KEY (id);


--
-- Name: zdx_claude_sessions zdx_claude_sessions_project_id_session_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_claude_sessions
    ADD CONSTRAINT zdx_claude_sessions_project_id_session_id_key UNIQUE (project_id, session_id);


--
-- Name: zdx_code_refs zdx_code_refs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_code_refs
    ADD CONSTRAINT zdx_code_refs_pkey PRIMARY KEY (id);


--
-- Name: zdx_comment_reactions zdx_comment_reactions_comment_id_emoji_reactor_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_comment_reactions
    ADD CONSTRAINT zdx_comment_reactions_comment_id_emoji_reactor_key UNIQUE (comment_id, emoji, reactor);


--
-- Name: zdx_comment_reactions zdx_comment_reactions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_comment_reactions
    ADD CONSTRAINT zdx_comment_reactions_pkey PRIMARY KEY (id);


--
-- Name: zdx_comment_reads zdx_comment_reads_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_comment_reads
    ADD CONSTRAINT zdx_comment_reads_pkey PRIMARY KEY (id);


--
-- Name: zdx_comment_reads zdx_comment_reads_project_id_target_type_target_id_role_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_comment_reads
    ADD CONSTRAINT zdx_comment_reads_project_id_target_type_target_id_role_key UNIQUE (project_id, target_type, target_id, role);


--
-- Name: zdx_comments zdx_comments_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_comments
    ADD CONSTRAINT zdx_comments_pkey PRIMARY KEY (id);


--
-- Name: zdx_counted zdx_counted_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_counted
    ADD CONSTRAINT zdx_counted_pkey PRIMARY KEY (id);


--
-- Name: zdx_counter_events zdx_counter_events_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_counter_events
    ADD CONSTRAINT zdx_counter_events_pkey PRIMARY KEY (id);


--
-- Name: zdx_deploys zdx_deploys_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_deploys
    ADD CONSTRAINT zdx_deploys_pkey PRIMARY KEY (id);


--
-- Name: zdx_doctor_deferrals zdx_doctor_deferrals_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_doctor_deferrals
    ADD CONSTRAINT zdx_doctor_deferrals_pkey PRIMARY KEY (id);


--
-- Name: zdx_doctor_deferrals zdx_doctor_deferrals_project_id_check_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_doctor_deferrals
    ADD CONSTRAINT zdx_doctor_deferrals_project_id_check_name_key UNIQUE (project_id, check_name);


--
-- Name: zdx_environments zdx_environments_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_environments
    ADD CONSTRAINT zdx_environments_pkey PRIMARY KEY (id);


--
-- Name: zdx_error_events zdx_error_events_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_error_events
    ADD CONSTRAINT zdx_error_events_pkey PRIMARY KEY (id);


--
-- Name: zdx_error_reports zdx_error_reports_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_error_reports
    ADD CONSTRAINT zdx_error_reports_pkey PRIMARY KEY (id);


--
-- Name: zdx_feature_multipliers zdx_feature_multipliers_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_feature_multipliers
    ADD CONSTRAINT zdx_feature_multipliers_pkey PRIMARY KEY (feature_id, multiplies_feature_id);


--
-- Name: zdx_features zdx_features_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_features
    ADD CONSTRAINT zdx_features_pkey PRIMARY KEY (id);


--
-- Name: zdx_features zdx_features_project_id_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_features
    ADD CONSTRAINT zdx_features_project_id_name_key UNIQUE (project_id, name);


--
-- Name: zdx_files zdx_files_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_files
    ADD CONSTRAINT zdx_files_pkey PRIMARY KEY (id);


--
-- Name: zdx_focus_features zdx_focus_features_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_focus_features
    ADD CONSTRAINT zdx_focus_features_pkey PRIMARY KEY (focus_id, feature_id);


--
-- Name: zdx_goal_issues zdx_goal_issues_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_goal_issues
    ADD CONSTRAINT zdx_goal_issues_pkey PRIMARY KEY (goal_id, issue_id);


--
-- Name: zdx_id_seq zdx_id_seq_pkey1; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_id_seq
    ADD CONSTRAINT zdx_id_seq_pkey1 PRIMARY KEY (kind);


--
-- Name: zdx_integration_token zdx_integration_token_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_integration_token
    ADD CONSTRAINT zdx_integration_token_pkey PRIMARY KEY (id);


--
-- Name: zdx_invites zdx_invites_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_invites
    ADD CONSTRAINT zdx_invites_pkey PRIMARY KEY (id);


--
-- Name: zdx_invites zdx_invites_token_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_invites
    ADD CONSTRAINT zdx_invites_token_key UNIQUE (token);


--
-- Name: zdx_issue_blocks zdx_issue_blocks_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_issue_blocks
    ADD CONSTRAINT zdx_issue_blocks_pkey PRIMARY KEY (issue_id, blocked_by_id);


--
-- Name: zdx_issue_code_refs zdx_issue_code_refs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_issue_code_refs
    ADD CONSTRAINT zdx_issue_code_refs_pkey PRIMARY KEY (issue_id, code_ref_id);


--
-- Name: zdx_issue_features zdx_issue_features_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_issue_features
    ADD CONSTRAINT zdx_issue_features_pkey PRIMARY KEY (issue_id, feature_id);


--
-- Name: zdx_issue_files zdx_issue_files_issue_id_file_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_issue_files
    ADD CONSTRAINT zdx_issue_files_issue_id_file_id_key UNIQUE (issue_id, file_id);


--
-- Name: zdx_issue_files zdx_issue_files_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_issue_files
    ADD CONSTRAINT zdx_issue_files_pkey PRIMARY KEY (id);


--
-- Name: zdx_issue_resolution_commits zdx_issue_resolution_commits_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_issue_resolution_commits
    ADD CONSTRAINT zdx_issue_resolution_commits_pkey PRIMARY KEY (resolution_id, sha);


--
-- Name: zdx_issue_resolutions zdx_issue_resolutions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_issue_resolutions
    ADD CONSTRAINT zdx_issue_resolutions_pkey PRIMARY KEY (id);


--
-- Name: zdx_issue_work zdx_issue_work_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_issue_work
    ADD CONSTRAINT zdx_issue_work_pkey PRIMARY KEY (id);


--
-- Name: zdx_issues zdx_issues_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_issues
    ADD CONSTRAINT zdx_issues_pkey PRIMARY KEY (id);


--
-- Name: zdx_journal_entries zdx_journal_entries_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_journal_entries
    ADD CONSTRAINT zdx_journal_entries_pkey PRIMARY KEY (id);


--
-- Name: zdx_llm_configs zdx_llm_configs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_llm_configs
    ADD CONSTRAINT zdx_llm_configs_pkey PRIMARY KEY (id);


--
-- Name: zdx_llm_configs zdx_llm_configs_priority_uniq; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_llm_configs
    ADD CONSTRAINT zdx_llm_configs_priority_uniq UNIQUE (priority) DEFERRABLE;


--
-- Name: zdx_log_events zdx_log_events_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_log_events
    ADD CONSTRAINT zdx_log_events_pkey PRIMARY KEY (id);


--
-- Name: zdx_maturity_answers zdx_maturity_answers_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_maturity_answers
    ADD CONSTRAINT zdx_maturity_answers_pkey PRIMARY KEY (id);


--
-- Name: zdx_maturity_answers zdx_maturity_answers_project_question_uq; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_maturity_answers
    ADD CONSTRAINT zdx_maturity_answers_project_question_uq UNIQUE (project_id, question_key);


--
-- Name: zdx_maturity_items zdx_maturity_items_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_maturity_items
    ADD CONSTRAINT zdx_maturity_items_pkey PRIMARY KEY (id);


--
-- Name: zdx_maturity_questions zdx_maturity_questions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_maturity_questions
    ADD CONSTRAINT zdx_maturity_questions_pkey PRIMARY KEY (key);


--
-- Name: zdx_oauth_identities zdx_oauth_identities_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_oauth_identities
    ADD CONSTRAINT zdx_oauth_identities_pkey PRIMARY KEY (id);


--
-- Name: zdx_oauth_identities zdx_oauth_identities_provider_sub_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_oauth_identities
    ADD CONSTRAINT zdx_oauth_identities_provider_sub_key UNIQUE (provider, sub);


--
-- Name: zdx_oauth_states zdx_oauth_states_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_oauth_states
    ADD CONSTRAINT zdx_oauth_states_pkey PRIMARY KEY (state);


--
-- Name: zdx_patterns zdx_patterns_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_patterns
    ADD CONSTRAINT zdx_patterns_pkey PRIMARY KEY (id);


--
-- Name: zdx_plan_step_refs zdx_plan_step_refs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_plan_step_refs
    ADD CONSTRAINT zdx_plan_step_refs_pkey PRIMARY KEY (step_id, target_type, target_id);


--
-- Name: zdx_plan_steps zdx_plan_steps_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_plan_steps
    ADD CONSTRAINT zdx_plan_steps_pkey PRIMARY KEY (id);


--
-- Name: zdx_plans zdx_plans_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_plans
    ADD CONSTRAINT zdx_plans_pkey PRIMARY KEY (id);


--
-- Name: zdx_project_constraints zdx_project_constraints_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_project_constraints
    ADD CONSTRAINT zdx_project_constraints_pkey PRIMARY KEY (id);


--
-- Name: zdx_project_constraints zdx_project_constraints_project_id_title_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_project_constraints
    ADD CONSTRAINT zdx_project_constraints_project_id_title_key UNIQUE (project_id, title);


--
-- Name: zdx_project_git_config zdx_project_git_config_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_project_git_config
    ADD CONSTRAINT zdx_project_git_config_pkey PRIMARY KEY (id);


--
-- Name: zdx_project_git_config zdx_project_git_config_project_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_project_git_config
    ADD CONSTRAINT zdx_project_git_config_project_id_key UNIQUE (project_id);


--
-- Name: zdx_project_goals zdx_project_goals_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_project_goals
    ADD CONSTRAINT zdx_project_goals_pkey PRIMARY KEY (id);


--
-- Name: zdx_project_goals zdx_project_goals_project_id_title_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_project_goals
    ADD CONSTRAINT zdx_project_goals_project_id_title_key UNIQUE (project_id, title);


--
-- Name: zdx_project_permissions zdx_project_permissions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_project_permissions
    ADD CONSTRAINT zdx_project_permissions_pkey PRIMARY KEY (id);


--
-- Name: zdx_project_permissions zdx_project_permissions_user_id_project_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_project_permissions
    ADD CONSTRAINT zdx_project_permissions_user_id_project_id_key UNIQUE (user_id, project_id);


--
-- Name: zdx_projects zdx_projects_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_projects
    ADD CONSTRAINT zdx_projects_pkey PRIMARY KEY (id);


--
-- Name: zdx_projects zdx_projects_slug_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_projects
    ADD CONSTRAINT zdx_projects_slug_key UNIQUE (slug);


--
-- Name: zdx_question_proposals zdx_question_proposals_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_question_proposals
    ADD CONSTRAINT zdx_question_proposals_pkey PRIMARY KEY (id);


--
-- Name: zdx_questions zdx_questions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_questions
    ADD CONSTRAINT zdx_questions_pkey PRIMARY KEY (id);


--
-- Name: zdx_reservations zdx_reservations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_reservations
    ADD CONSTRAINT zdx_reservations_pkey PRIMARY KEY (id);


--
-- Name: zdx_revisions zdx_revisions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_revisions
    ADD CONSTRAINT zdx_revisions_pkey PRIMARY KEY (id);


--
-- Name: zdx_sessions zdx_sessions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_sessions
    ADD CONSTRAINT zdx_sessions_pkey PRIMARY KEY (id);


--
-- Name: zdx_sessions zdx_sessions_token_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_sessions
    ADD CONSTRAINT zdx_sessions_token_key UNIQUE (token);


--
-- Name: zdx_slow_queries zdx_slow_queries_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_slow_queries
    ADD CONSTRAINT zdx_slow_queries_pkey PRIMARY KEY (id);


--
-- Name: zdx_spec_code_refs zdx_spec_code_refs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_spec_code_refs
    ADD CONSTRAINT zdx_spec_code_refs_pkey PRIMARY KEY (spec_id, code_ref_id);


--
-- Name: zdx_spec_issues zdx_spec_issues_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_spec_issues
    ADD CONSTRAINT zdx_spec_issues_pkey PRIMARY KEY (spec_id, issue_id);


--
-- Name: zdx_spec_tests zdx_spec_tests_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_spec_tests
    ADD CONSTRAINT zdx_spec_tests_pkey PRIMARY KEY (spec_id, test_id);


--
-- Name: zdx_specs zdx_specs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_specs
    ADD CONSTRAINT zdx_specs_pkey PRIMARY KEY (id);


--
-- Name: zdx_sprints zdx_sprints_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_sprints
    ADD CONSTRAINT zdx_sprints_pkey PRIMARY KEY (id);


--
-- Name: zdx_sprints zdx_sprints_project_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_sprints
    ADD CONSTRAINT zdx_sprints_project_id_key UNIQUE (project_id);


--
-- Name: zdx_state zdx_state_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_state
    ADD CONSTRAINT zdx_state_pkey PRIMARY KEY (project_id, key);


--
-- Name: zdx_task_code_refs zdx_task_code_refs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_task_code_refs
    ADD CONSTRAINT zdx_task_code_refs_pkey PRIMARY KEY (task_id, code_ref_id);


--
-- Name: zdx_task_reviews zdx_task_reviews_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_task_reviews
    ADD CONSTRAINT zdx_task_reviews_pkey PRIMARY KEY (id);


--
-- Name: zdx_task_reviews zdx_task_reviews_project_id_task_id_reviewer_role_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_task_reviews
    ADD CONSTRAINT zdx_task_reviews_project_id_task_id_reviewer_role_key UNIQUE (project_id, task_id, reviewer_role);


--
-- Name: zdx_tasks zdx_tasks_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_tasks
    ADD CONSTRAINT zdx_tasks_pkey PRIMARY KEY (id);


--
-- Name: zdx_test_code_refs zdx_test_code_refs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_test_code_refs
    ADD CONSTRAINT zdx_test_code_refs_pkey PRIMARY KEY (test_id, code_ref_id);


--
-- Name: zdx_test_demos zdx_test_demos_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_test_demos
    ADD CONSTRAINT zdx_test_demos_pkey PRIMARY KEY (id);


--
-- Name: zdx_test_demos zdx_test_demos_test_id_demo_type_artifact_path_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_test_demos
    ADD CONSTRAINT zdx_test_demos_test_id_demo_type_artifact_path_key UNIQUE (test_id, demo_type, artifact_path);


--
-- Name: zdx_test_result_history zdx_test_result_history_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_test_result_history
    ADD CONSTRAINT zdx_test_result_history_pkey PRIMARY KEY (id);


--
-- Name: zdx_test_results zdx_test_results_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_test_results
    ADD CONSTRAINT zdx_test_results_pkey PRIMARY KEY (id);


--
-- Name: zdx_test_results zdx_test_results_project_id_driver_test_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_test_results
    ADD CONSTRAINT zdx_test_results_project_id_driver_test_name_key UNIQUE (project_id, driver, test_name);


--
-- Name: zdx_tests zdx_tests_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_tests
    ADD CONSTRAINT zdx_tests_pkey PRIMARY KEY (id);


--
-- Name: zdx_tests zdx_tests_project_id_component_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_tests
    ADD CONSTRAINT zdx_tests_project_id_component_name_key UNIQUE (project_id, component, name);


--
-- Name: zdx_focus_blockers zdx_theme_blockers_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_focus_blockers
    ADD CONSTRAINT zdx_theme_blockers_pkey PRIMARY KEY (focus_id, issue_id);


--
-- Name: zdx_focuses zdx_themes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_focuses
    ADD CONSTRAINT zdx_themes_pkey PRIMARY KEY (id);


--
-- Name: zdx_focuses zdx_themes_project_id_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_focuses
    ADD CONSTRAINT zdx_themes_project_id_name_key UNIQUE (project_id, name);


--
-- Name: zdx_timed_events zdx_timed_events_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_timed_events
    ADD CONSTRAINT zdx_timed_events_pkey PRIMARY KEY (id);


--
-- Name: zdx_timed zdx_timed_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_timed
    ADD CONSTRAINT zdx_timed_pkey PRIMARY KEY (id);


--
-- Name: zdx_todos zdx_todos_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_todos
    ADD CONSTRAINT zdx_todos_pkey PRIMARY KEY (id);


--
-- Name: zdx_todos zdx_todos_project_id_key_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_todos
    ADD CONSTRAINT zdx_todos_project_id_key_key UNIQUE (project_id, key);


--
-- Name: zdx_users zdx_users_email_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_users
    ADD CONSTRAINT zdx_users_email_key UNIQUE (email);


--
-- Name: zdx_users zdx_users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_users
    ADD CONSTRAINT zdx_users_pkey PRIMARY KEY (id);


--
-- Name: zdx_work_log zdx_work_log_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_work_log
    ADD CONSTRAINT zdx_work_log_pkey PRIMARY KEY (id);


--
-- Name: idx_blocker_questions_pending; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_blocker_questions_pending ON public.zdx_blocker_questions USING btree (project_id, status) WHERE (status = 'pending'::text);


--
-- Name: idx_blocker_questions_target; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_blocker_questions_target ON public.zdx_blocker_questions USING btree (project_id, target_type, target_id);


--
-- Name: idx_comment_reactions_comment; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_comment_reactions_comment ON public.zdx_comment_reactions USING btree (comment_id);


--
-- Name: idx_comments_parent; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_comments_parent ON public.zdx_comments USING btree (parent_id) WHERE (parent_id IS NOT NULL);


--
-- Name: idx_comments_target; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_comments_target ON public.zdx_comments USING btree (project_id, target_type, target_id);


--
-- Name: idx_error_reports_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_error_reports_created_at ON public.zdx_error_reports USING btree (created_at DESC);


--
-- Name: idx_error_reports_source; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_error_reports_source ON public.zdx_error_reports USING btree (source);


--
-- Name: idx_goal_issues_issue; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_goal_issues_issue ON public.zdx_goal_issues USING btree (issue_id);


--
-- Name: idx_issue_code_refs_issue; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_issue_code_refs_issue ON public.zdx_issue_code_refs USING btree (issue_id);


--
-- Name: idx_issue_files_issue; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_issue_files_issue ON public.zdx_issue_files USING btree (issue_id);


--
-- Name: idx_issue_resolution_commits_sha; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_issue_resolution_commits_sha ON public.zdx_issue_resolution_commits USING btree (sha);


--
-- Name: idx_issue_resolutions_issue; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_issue_resolutions_issue ON public.zdx_issue_resolutions USING btree (issue_id);


--
-- Name: idx_issues_source_error_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_issues_source_error_id ON public.zdx_issues USING btree (source_error_id) WHERE (source_error_id IS NOT NULL);


--
-- Name: idx_journal_project_role; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_journal_project_role ON public.zdx_journal_entries USING btree (project_id, role, date DESC);


--
-- Name: idx_oauth_identities_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_oauth_identities_user ON public.zdx_oauth_identities USING btree (user_id);


--
-- Name: idx_patterns_name; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_patterns_name ON public.zdx_patterns USING btree (project_id, name);


--
-- Name: idx_patterns_project; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_patterns_project ON public.zdx_patterns USING btree (project_id);


--
-- Name: idx_project_constraints_project; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_project_constraints_project ON public.zdx_project_constraints USING btree (project_id);


--
-- Name: idx_project_goals_project; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_project_goals_project ON public.zdx_project_goals USING btree (project_id);


--
-- Name: idx_question_proposals_question; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_question_proposals_question ON public.zdx_question_proposals USING btree (project_id, question_id, question_type);


--
-- Name: idx_question_proposals_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_question_proposals_status ON public.zdx_question_proposals USING btree (project_id, status);


--
-- Name: idx_questions_parent; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_questions_parent ON public.zdx_questions USING btree (parent_question_id) WHERE (parent_question_id IS NOT NULL);


--
-- Name: idx_questions_project; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_questions_project ON public.zdx_questions USING btree (project_id);


--
-- Name: idx_slow_queries_created_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_slow_queries_created_at ON public.zdx_slow_queries USING btree (created_at DESC);


--
-- Name: idx_slow_queries_endpoint; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_slow_queries_endpoint ON public.zdx_slow_queries USING btree (endpoint);


--
-- Name: idx_slow_queries_sql_hash; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_slow_queries_sql_hash ON public.zdx_slow_queries USING btree (sql_hash);


--
-- Name: idx_spec_code_refs_spec; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_spec_code_refs_spec ON public.zdx_spec_code_refs USING btree (spec_id);


--
-- Name: idx_spec_issues_issue; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_spec_issues_issue ON public.zdx_spec_issues USING btree (issue_id);


--
-- Name: idx_spec_tests_test; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_spec_tests_test ON public.zdx_spec_tests USING btree (test_id);


--
-- Name: idx_task_code_refs_task; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_task_code_refs_task ON public.zdx_task_code_refs USING btree (task_id);


--
-- Name: idx_test_code_refs_test; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_test_code_refs_test ON public.zdx_test_code_refs USING btree (test_id);


--
-- Name: idx_test_result_history_lookup; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_test_result_history_lookup ON public.zdx_test_result_history USING btree (project_id, test_name, run_at DESC);


--
-- Name: idx_tests_layer; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_tests_layer ON public.zdx_tests USING btree (project_id, layer);


--
-- Name: idx_tests_project; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_tests_project ON public.zdx_tests USING btree (project_id);


--
-- Name: idx_tests_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_tests_status ON public.zdx_tests USING btree (project_id, status);


--
-- Name: zdx_claude_events_session; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX zdx_claude_events_session ON public.zdx_claude_events USING btree (session_pk, seq);


--
-- Name: zdx_claude_sessions_project; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX zdx_claude_sessions_project ON public.zdx_claude_sessions USING btree (project_id);


--
-- Name: zdx_claude_sessions_project_updated; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX zdx_claude_sessions_project_updated ON public.zdx_claude_sessions USING btree (project_id, updated_at DESC);


--
-- Name: zdx_claude_sessions_todo; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX zdx_claude_sessions_todo ON public.zdx_claude_sessions USING btree (todo_id) WHERE (todo_id IS NOT NULL);


--
-- Name: zdx_counted_context_gin; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX zdx_counted_context_gin ON public.zdx_counted USING gin (context_json jsonb_path_ops);


--
-- Name: zdx_counted_name; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX zdx_counted_name ON public.zdx_counted USING btree (COALESCE(project_id, 0), component, environment, name);


--
-- Name: zdx_counted_project_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX zdx_counted_project_created ON public.zdx_counted USING btree (project_id, created_at);


--
-- Name: zdx_counter_events_context_gin; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX zdx_counter_events_context_gin ON public.zdx_counter_events USING gin (context_json jsonb_path_ops);


--
-- Name: zdx_counter_events_project_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX zdx_counter_events_project_created ON public.zdx_counter_events USING btree (project_id, created_at);


--
-- Name: zdx_deploys_env_time; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX zdx_deploys_env_time ON public.zdx_deploys USING btree (environment_id, deployed_at DESC);


--
-- Name: zdx_environments_project_name; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX zdx_environments_project_name ON public.zdx_environments USING btree (project_id, name);


--
-- Name: zdx_error_events_context_gin; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX zdx_error_events_context_gin ON public.zdx_error_events USING gin (context_json jsonb_path_ops);


--
-- Name: zdx_error_events_project_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX zdx_error_events_project_created ON public.zdx_error_events USING btree (project_id, created_at);


--
-- Name: zdx_integration_token_hash; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX zdx_integration_token_hash ON public.zdx_integration_token USING btree (token_hash);


--
-- Name: zdx_integration_token_prefix; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX zdx_integration_token_prefix ON public.zdx_integration_token USING btree (token_prefix);


--
-- Name: zdx_integration_token_project; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX zdx_integration_token_project ON public.zdx_integration_token USING btree (project_id);


--
-- Name: zdx_log_events_context_gin; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX zdx_log_events_context_gin ON public.zdx_log_events USING gin (context_json jsonb_path_ops);


--
-- Name: zdx_log_events_project_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX zdx_log_events_project_created ON public.zdx_log_events USING btree (project_id, created_at);


--
-- Name: zdx_maturity_answers_by_project; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX zdx_maturity_answers_by_project ON public.zdx_maturity_answers USING btree (project_id);


--
-- Name: zdx_maturity_items_by_project_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX zdx_maturity_items_by_project_status ON public.zdx_maturity_items USING btree (project_id, status);


--
-- Name: zdx_maturity_items_dedup; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX zdx_maturity_items_dedup ON public.zdx_maturity_items USING btree (project_id, kind, target_type, COALESCE(target_id, 0));


--
-- Name: zdx_reservations_claimed_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX zdx_reservations_claimed_at ON public.zdx_reservations USING btree (project_id, claimed_at DESC);


--
-- Name: zdx_reservations_project_target; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX zdx_reservations_project_target ON public.zdx_reservations USING btree (project_id, target_type, target_id);


--
-- Name: zdx_revisions_target; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX zdx_revisions_target ON public.zdx_revisions USING btree (project_id, target_type, target_id);


--
-- Name: zdx_task_reviews_task_idx; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX zdx_task_reviews_task_idx ON public.zdx_task_reviews USING btree (project_id, task_id);


--
-- Name: zdx_timed_context_gin; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX zdx_timed_context_gin ON public.zdx_timed USING gin (context_json jsonb_path_ops);


--
-- Name: zdx_timed_events_context_gin; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX zdx_timed_events_context_gin ON public.zdx_timed_events USING gin (context_json jsonb_path_ops);


--
-- Name: zdx_timed_events_project_created; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX zdx_timed_events_project_created ON public.zdx_timed_events USING btree (project_id, created_at);


--
-- Name: zdx_timed_name; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX zdx_timed_name ON public.zdx_timed USING btree (COALESCE(project_id, 0), component, environment, name);


--
-- Name: zdx_timed_project; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX zdx_timed_project ON public.zdx_timed USING btree (project_id);


--
-- Name: zdx_agents zdx_agents_project_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_agents
    ADD CONSTRAINT zdx_agents_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.zdx_projects(id) ON DELETE CASCADE;


--
-- Name: zdx_api_keys zdx_api_keys_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_api_keys
    ADD CONSTRAINT zdx_api_keys_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.zdx_users(id) ON DELETE CASCADE;


--
-- Name: zdx_blocker_questions zdx_blocker_questions_project_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_blocker_questions
    ADD CONSTRAINT zdx_blocker_questions_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.zdx_projects(id) ON DELETE CASCADE;


--
-- Name: zdx_claude_events zdx_claude_events_session_pk_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_claude_events
    ADD CONSTRAINT zdx_claude_events_session_pk_fkey FOREIGN KEY (session_pk) REFERENCES public.zdx_claude_sessions(id) ON DELETE CASCADE;


--
-- Name: zdx_claude_sessions zdx_claude_sessions_project_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_claude_sessions
    ADD CONSTRAINT zdx_claude_sessions_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.zdx_projects(id) ON DELETE CASCADE;


--
-- Name: zdx_claude_sessions zdx_claude_sessions_todo_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_claude_sessions
    ADD CONSTRAINT zdx_claude_sessions_todo_id_fkey FOREIGN KEY (todo_id) REFERENCES public.zdx_todos(id) ON DELETE SET NULL;


--
-- Name: zdx_code_refs zdx_code_refs_project_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_code_refs
    ADD CONSTRAINT zdx_code_refs_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.zdx_projects(id) ON DELETE CASCADE;


--
-- Name: zdx_comment_reactions zdx_comment_reactions_comment_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_comment_reactions
    ADD CONSTRAINT zdx_comment_reactions_comment_id_fkey FOREIGN KEY (comment_id) REFERENCES public.zdx_comments(id) ON DELETE CASCADE;


--
-- Name: zdx_comment_reactions zdx_comment_reactions_project_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_comment_reactions
    ADD CONSTRAINT zdx_comment_reactions_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.zdx_projects(id) ON DELETE CASCADE;


--
-- Name: zdx_comment_reads zdx_comment_reads_project_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_comment_reads
    ADD CONSTRAINT zdx_comment_reads_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.zdx_projects(id) ON DELETE CASCADE;


--
-- Name: zdx_comments zdx_comments_parent_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_comments
    ADD CONSTRAINT zdx_comments_parent_id_fkey FOREIGN KEY (parent_id) REFERENCES public.zdx_comments(id) ON DELETE CASCADE;


--
-- Name: zdx_comments zdx_comments_project_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_comments
    ADD CONSTRAINT zdx_comments_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.zdx_projects(id) ON DELETE CASCADE;


--
-- Name: zdx_counted zdx_counted_project_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_counted
    ADD CONSTRAINT zdx_counted_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.zdx_projects(id) ON DELETE CASCADE;


--
-- Name: zdx_counter_events zdx_counter_events_project_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_counter_events
    ADD CONSTRAINT zdx_counter_events_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.zdx_projects(id) ON DELETE CASCADE;


--
-- Name: zdx_deploys zdx_deploys_deployed_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_deploys
    ADD CONSTRAINT zdx_deploys_deployed_by_user_id_fkey FOREIGN KEY (deployed_by_user_id) REFERENCES public.zdx_users(id) ON DELETE SET NULL;


--
-- Name: zdx_deploys zdx_deploys_environment_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_deploys
    ADD CONSTRAINT zdx_deploys_environment_id_fkey FOREIGN KEY (environment_id) REFERENCES public.zdx_environments(id) ON DELETE CASCADE;


--
-- Name: zdx_doctor_deferrals zdx_doctor_deferrals_project_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_doctor_deferrals
    ADD CONSTRAINT zdx_doctor_deferrals_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.zdx_projects(id);


--
-- Name: zdx_environments zdx_environments_deployed_by_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_environments
    ADD CONSTRAINT zdx_environments_deployed_by_user_id_fkey FOREIGN KEY (deployed_by_user_id) REFERENCES public.zdx_users(id) ON DELETE SET NULL;


--
-- Name: zdx_environments zdx_environments_project_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_environments
    ADD CONSTRAINT zdx_environments_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.zdx_projects(id) ON DELETE CASCADE;


--
-- Name: zdx_error_events zdx_error_events_project_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_error_events
    ADD CONSTRAINT zdx_error_events_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.zdx_projects(id) ON DELETE CASCADE;


--
-- Name: zdx_error_reports zdx_error_reports_project_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_error_reports
    ADD CONSTRAINT zdx_error_reports_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.zdx_projects(id) ON DELETE CASCADE;


--
-- Name: zdx_feature_multipliers zdx_feature_multipliers_feature_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_feature_multipliers
    ADD CONSTRAINT zdx_feature_multipliers_feature_id_fkey FOREIGN KEY (feature_id) REFERENCES public.zdx_features(id) ON DELETE CASCADE;


--
-- Name: zdx_feature_multipliers zdx_feature_multipliers_multiplies_feature_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_feature_multipliers
    ADD CONSTRAINT zdx_feature_multipliers_multiplies_feature_id_fkey FOREIGN KEY (multiplies_feature_id) REFERENCES public.zdx_features(id) ON DELETE CASCADE;


--
-- Name: zdx_features zdx_features_goal_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_features
    ADD CONSTRAINT zdx_features_goal_id_fkey FOREIGN KEY (goal_id) REFERENCES public.zdx_project_goals(id);


--
-- Name: zdx_features zdx_features_parent_feature_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_features
    ADD CONSTRAINT zdx_features_parent_feature_id_fkey FOREIGN KEY (parent_feature_id) REFERENCES public.zdx_features(id);


--
-- Name: zdx_features zdx_features_project_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_features
    ADD CONSTRAINT zdx_features_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.zdx_projects(id);


--
-- Name: zdx_focus_features zdx_focus_features_feature_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_focus_features
    ADD CONSTRAINT zdx_focus_features_feature_id_fkey FOREIGN KEY (feature_id) REFERENCES public.zdx_features(id) ON DELETE CASCADE;


--
-- Name: zdx_focus_features zdx_focus_features_focus_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_focus_features
    ADD CONSTRAINT zdx_focus_features_focus_id_fkey FOREIGN KEY (focus_id) REFERENCES public.zdx_focuses(id) ON DELETE CASCADE;


--
-- Name: zdx_goal_issues zdx_goal_issues_goal_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_goal_issues
    ADD CONSTRAINT zdx_goal_issues_goal_id_fkey FOREIGN KEY (goal_id) REFERENCES public.zdx_project_goals(id) ON DELETE CASCADE;


--
-- Name: zdx_goal_issues zdx_goal_issues_issue_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_goal_issues
    ADD CONSTRAINT zdx_goal_issues_issue_id_fkey FOREIGN KEY (issue_id) REFERENCES public.zdx_issues(id) ON DELETE CASCADE;


--
-- Name: zdx_integration_token zdx_integration_token_project_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_integration_token
    ADD CONSTRAINT zdx_integration_token_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.zdx_projects(id) ON DELETE CASCADE;


--
-- Name: zdx_invites zdx_invites_invited_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_invites
    ADD CONSTRAINT zdx_invites_invited_by_fkey FOREIGN KEY (invited_by) REFERENCES public.zdx_users(id);


--
-- Name: zdx_issue_blocks zdx_issue_blocks_blocked_by_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_issue_blocks
    ADD CONSTRAINT zdx_issue_blocks_blocked_by_id_fkey FOREIGN KEY (blocked_by_id) REFERENCES public.zdx_issues(id) ON DELETE CASCADE;


--
-- Name: zdx_issue_blocks zdx_issue_blocks_issue_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_issue_blocks
    ADD CONSTRAINT zdx_issue_blocks_issue_id_fkey FOREIGN KEY (issue_id) REFERENCES public.zdx_issues(id) ON DELETE CASCADE;


--
-- Name: zdx_issue_code_refs zdx_issue_code_refs_code_ref_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_issue_code_refs
    ADD CONSTRAINT zdx_issue_code_refs_code_ref_id_fkey FOREIGN KEY (code_ref_id) REFERENCES public.zdx_code_refs(id) ON DELETE CASCADE;


--
-- Name: zdx_issue_code_refs zdx_issue_code_refs_issue_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_issue_code_refs
    ADD CONSTRAINT zdx_issue_code_refs_issue_id_fkey FOREIGN KEY (issue_id) REFERENCES public.zdx_issues(id) ON DELETE CASCADE;


--
-- Name: zdx_issue_features zdx_issue_features_feature_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_issue_features
    ADD CONSTRAINT zdx_issue_features_feature_id_fkey FOREIGN KEY (feature_id) REFERENCES public.zdx_features(id) ON DELETE CASCADE;


--
-- Name: zdx_issue_features zdx_issue_features_issue_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_issue_features
    ADD CONSTRAINT zdx_issue_features_issue_id_fkey FOREIGN KEY (issue_id) REFERENCES public.zdx_issues(id) ON DELETE CASCADE;


--
-- Name: zdx_issue_files zdx_issue_files_file_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_issue_files
    ADD CONSTRAINT zdx_issue_files_file_id_fkey FOREIGN KEY (file_id) REFERENCES public.zdx_files(id) ON DELETE CASCADE;


--
-- Name: zdx_issue_files zdx_issue_files_issue_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_issue_files
    ADD CONSTRAINT zdx_issue_files_issue_id_fkey FOREIGN KEY (issue_id) REFERENCES public.zdx_issues(id) ON DELETE CASCADE;


--
-- Name: zdx_issue_resolution_commits zdx_issue_resolution_commits_resolution_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_issue_resolution_commits
    ADD CONSTRAINT zdx_issue_resolution_commits_resolution_id_fkey FOREIGN KEY (resolution_id) REFERENCES public.zdx_issue_resolutions(id) ON DELETE CASCADE;


--
-- Name: zdx_issue_resolutions zdx_issue_resolutions_issue_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_issue_resolutions
    ADD CONSTRAINT zdx_issue_resolutions_issue_id_fkey FOREIGN KEY (issue_id) REFERENCES public.zdx_issues(id) ON DELETE CASCADE;


--
-- Name: zdx_issue_resolutions zdx_issue_resolutions_parent_resolution_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_issue_resolutions
    ADD CONSTRAINT zdx_issue_resolutions_parent_resolution_id_fkey FOREIGN KEY (parent_resolution_id) REFERENCES public.zdx_issue_resolutions(id) ON DELETE SET NULL;


--
-- Name: zdx_issue_work zdx_issue_work_issue_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_issue_work
    ADD CONSTRAINT zdx_issue_work_issue_id_fkey FOREIGN KEY (issue_id) REFERENCES public.zdx_issues(id) ON DELETE CASCADE;


--
-- Name: zdx_issues zdx_issues_project_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_issues
    ADD CONSTRAINT zdx_issues_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.zdx_projects(id);


--
-- Name: zdx_issues zdx_issues_source_error_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_issues
    ADD CONSTRAINT zdx_issues_source_error_id_fkey FOREIGN KEY (source_error_id) REFERENCES public.zdx_error_reports(id) ON DELETE SET NULL;


--
-- Name: zdx_journal_entries zdx_journal_entries_project_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_journal_entries
    ADD CONSTRAINT zdx_journal_entries_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.zdx_projects(id);


--
-- Name: zdx_log_events zdx_log_events_project_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_log_events
    ADD CONSTRAINT zdx_log_events_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.zdx_projects(id) ON DELETE CASCADE;


--
-- Name: zdx_maturity_answers zdx_maturity_answers_project_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_maturity_answers
    ADD CONSTRAINT zdx_maturity_answers_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.zdx_projects(id);


--
-- Name: zdx_maturity_answers zdx_maturity_answers_question_key_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_maturity_answers
    ADD CONSTRAINT zdx_maturity_answers_question_key_fkey FOREIGN KEY (question_key) REFERENCES public.zdx_maturity_questions(key);


--
-- Name: zdx_maturity_items zdx_maturity_items_project_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_maturity_items
    ADD CONSTRAINT zdx_maturity_items_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.zdx_projects(id);


--
-- Name: zdx_oauth_identities zdx_oauth_identities_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_oauth_identities
    ADD CONSTRAINT zdx_oauth_identities_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.zdx_users(id) ON DELETE CASCADE;


--
-- Name: zdx_patterns zdx_patterns_project_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_patterns
    ADD CONSTRAINT zdx_patterns_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.zdx_projects(id) ON DELETE CASCADE;


--
-- Name: zdx_plan_step_refs zdx_plan_step_refs_step_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_plan_step_refs
    ADD CONSTRAINT zdx_plan_step_refs_step_id_fkey FOREIGN KEY (step_id) REFERENCES public.zdx_plan_steps(id) ON DELETE CASCADE;


--
-- Name: zdx_plan_steps zdx_plan_steps_depends_on_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_plan_steps
    ADD CONSTRAINT zdx_plan_steps_depends_on_fkey FOREIGN KEY (depends_on) REFERENCES public.zdx_plan_steps(id);


--
-- Name: zdx_plan_steps zdx_plan_steps_plan_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_plan_steps
    ADD CONSTRAINT zdx_plan_steps_plan_id_fkey FOREIGN KEY (plan_id) REFERENCES public.zdx_plans(id) ON DELETE CASCADE;


--
-- Name: zdx_plans zdx_plans_feature_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_plans
    ADD CONSTRAINT zdx_plans_feature_id_fkey FOREIGN KEY (feature_id) REFERENCES public.zdx_features(id) ON DELETE CASCADE;


--
-- Name: zdx_plans zdx_plans_focus_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_plans
    ADD CONSTRAINT zdx_plans_focus_id_fkey FOREIGN KEY (focus_id) REFERENCES public.zdx_focuses(id);


--
-- Name: zdx_plans zdx_plans_project_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_plans
    ADD CONSTRAINT zdx_plans_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.zdx_projects(id);


--
-- Name: zdx_project_constraints zdx_project_constraints_project_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_project_constraints
    ADD CONSTRAINT zdx_project_constraints_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.zdx_projects(id);


--
-- Name: zdx_project_git_config zdx_project_git_config_project_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_project_git_config
    ADD CONSTRAINT zdx_project_git_config_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.zdx_projects(id) ON DELETE CASCADE;


--
-- Name: zdx_project_goals zdx_project_goals_project_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_project_goals
    ADD CONSTRAINT zdx_project_goals_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.zdx_projects(id);


--
-- Name: zdx_project_permissions zdx_project_permissions_project_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_project_permissions
    ADD CONSTRAINT zdx_project_permissions_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.zdx_projects(id) ON DELETE CASCADE;


--
-- Name: zdx_project_permissions zdx_project_permissions_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_project_permissions
    ADD CONSTRAINT zdx_project_permissions_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.zdx_users(id) ON DELETE CASCADE;


--
-- Name: zdx_question_proposals zdx_question_proposals_project_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_question_proposals
    ADD CONSTRAINT zdx_question_proposals_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.zdx_projects(id) ON DELETE CASCADE;


--
-- Name: zdx_questions zdx_questions_parent_question_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_questions
    ADD CONSTRAINT zdx_questions_parent_question_id_fkey FOREIGN KEY (parent_question_id) REFERENCES public.zdx_questions(id) ON DELETE SET NULL;


--
-- Name: zdx_questions zdx_questions_project_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_questions
    ADD CONSTRAINT zdx_questions_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.zdx_projects(id) ON DELETE CASCADE;


--
-- Name: zdx_reservations zdx_reservations_project_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_reservations
    ADD CONSTRAINT zdx_reservations_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.zdx_projects(id) ON DELETE CASCADE;


--
-- Name: zdx_revisions zdx_revisions_project_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_revisions
    ADD CONSTRAINT zdx_revisions_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.zdx_projects(id) ON DELETE CASCADE;


--
-- Name: zdx_sessions zdx_sessions_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_sessions
    ADD CONSTRAINT zdx_sessions_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.zdx_users(id) ON DELETE CASCADE;


--
-- Name: zdx_slow_queries zdx_slow_queries_project_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_slow_queries
    ADD CONSTRAINT zdx_slow_queries_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.zdx_projects(id) ON DELETE CASCADE;


--
-- Name: zdx_spec_code_refs zdx_spec_code_refs_code_ref_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_spec_code_refs
    ADD CONSTRAINT zdx_spec_code_refs_code_ref_id_fkey FOREIGN KEY (code_ref_id) REFERENCES public.zdx_code_refs(id) ON DELETE CASCADE;


--
-- Name: zdx_spec_code_refs zdx_spec_code_refs_spec_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_spec_code_refs
    ADD CONSTRAINT zdx_spec_code_refs_spec_id_fkey FOREIGN KEY (spec_id) REFERENCES public.zdx_specs(id) ON DELETE CASCADE;


--
-- Name: zdx_spec_issues zdx_spec_issues_issue_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_spec_issues
    ADD CONSTRAINT zdx_spec_issues_issue_id_fkey FOREIGN KEY (issue_id) REFERENCES public.zdx_issues(id) ON DELETE CASCADE;


--
-- Name: zdx_spec_issues zdx_spec_issues_spec_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_spec_issues
    ADD CONSTRAINT zdx_spec_issues_spec_id_fkey FOREIGN KEY (spec_id) REFERENCES public.zdx_specs(id) ON DELETE CASCADE;


--
-- Name: zdx_spec_tests zdx_spec_tests_spec_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_spec_tests
    ADD CONSTRAINT zdx_spec_tests_spec_id_fkey FOREIGN KEY (spec_id) REFERENCES public.zdx_specs(id) ON DELETE CASCADE;


--
-- Name: zdx_spec_tests zdx_spec_tests_test_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_spec_tests
    ADD CONSTRAINT zdx_spec_tests_test_id_fkey FOREIGN KEY (test_id) REFERENCES public.zdx_tests(id) ON DELETE RESTRICT;


--
-- Name: zdx_specs zdx_specs_feature_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_specs
    ADD CONSTRAINT zdx_specs_feature_id_fkey FOREIGN KEY (feature_id) REFERENCES public.zdx_features(id) ON DELETE CASCADE;


--
-- Name: zdx_sprints zdx_sprints_project_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_sprints
    ADD CONSTRAINT zdx_sprints_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.zdx_projects(id);


--
-- Name: zdx_state zdx_state_project_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_state
    ADD CONSTRAINT zdx_state_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.zdx_projects(id) ON DELETE CASCADE;


--
-- Name: zdx_task_code_refs zdx_task_code_refs_code_ref_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_task_code_refs
    ADD CONSTRAINT zdx_task_code_refs_code_ref_id_fkey FOREIGN KEY (code_ref_id) REFERENCES public.zdx_code_refs(id) ON DELETE CASCADE;


--
-- Name: zdx_task_code_refs zdx_task_code_refs_task_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_task_code_refs
    ADD CONSTRAINT zdx_task_code_refs_task_id_fkey FOREIGN KEY (task_id) REFERENCES public.zdx_tasks(id) ON DELETE CASCADE;


--
-- Name: zdx_task_reviews zdx_task_reviews_project_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_task_reviews
    ADD CONSTRAINT zdx_task_reviews_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.zdx_projects(id);


--
-- Name: zdx_task_reviews zdx_task_reviews_reviewer_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_task_reviews
    ADD CONSTRAINT zdx_task_reviews_reviewer_user_id_fkey FOREIGN KEY (reviewer_user_id) REFERENCES public.zdx_users(id);


--
-- Name: zdx_task_reviews zdx_task_reviews_task_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_task_reviews
    ADD CONSTRAINT zdx_task_reviews_task_id_fkey FOREIGN KEY (task_id) REFERENCES public.zdx_tasks(id) ON DELETE CASCADE;


--
-- Name: zdx_tasks zdx_tasks_project_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_tasks
    ADD CONSTRAINT zdx_tasks_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.zdx_projects(id);


--
-- Name: zdx_test_code_refs zdx_test_code_refs_code_ref_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_test_code_refs
    ADD CONSTRAINT zdx_test_code_refs_code_ref_id_fkey FOREIGN KEY (code_ref_id) REFERENCES public.zdx_code_refs(id) ON DELETE CASCADE;


--
-- Name: zdx_test_code_refs zdx_test_code_refs_test_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_test_code_refs
    ADD CONSTRAINT zdx_test_code_refs_test_id_fkey FOREIGN KEY (test_id) REFERENCES public.zdx_tests(id) ON DELETE CASCADE;


--
-- Name: zdx_test_demos zdx_test_demos_file_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_test_demos
    ADD CONSTRAINT zdx_test_demos_file_id_fkey FOREIGN KEY (file_id) REFERENCES public.zdx_files(id) ON DELETE SET NULL;


--
-- Name: zdx_test_demos zdx_test_demos_test_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_test_demos
    ADD CONSTRAINT zdx_test_demos_test_id_fkey FOREIGN KEY (test_id) REFERENCES public.zdx_tests(id) ON DELETE CASCADE;


--
-- Name: zdx_test_result_history zdx_test_result_history_project_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_test_result_history
    ADD CONSTRAINT zdx_test_result_history_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.zdx_projects(id) ON DELETE CASCADE;


--
-- Name: zdx_test_results zdx_test_results_project_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_test_results
    ADD CONSTRAINT zdx_test_results_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.zdx_projects(id) ON DELETE CASCADE;


--
-- Name: zdx_tests zdx_tests_project_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_tests
    ADD CONSTRAINT zdx_tests_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.zdx_projects(id) ON DELETE CASCADE;


--
-- Name: zdx_focus_blockers zdx_theme_blockers_issue_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_focus_blockers
    ADD CONSTRAINT zdx_theme_blockers_issue_id_fkey FOREIGN KEY (issue_id) REFERENCES public.zdx_issues(id) ON DELETE CASCADE;


--
-- Name: zdx_focus_blockers zdx_theme_blockers_theme_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_focus_blockers
    ADD CONSTRAINT zdx_theme_blockers_theme_id_fkey FOREIGN KEY (focus_id) REFERENCES public.zdx_focuses(id) ON DELETE CASCADE;


--
-- Name: zdx_focuses zdx_themes_project_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_focuses
    ADD CONSTRAINT zdx_themes_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.zdx_projects(id);


--
-- Name: zdx_timed_events zdx_timed_events_project_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_timed_events
    ADD CONSTRAINT zdx_timed_events_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.zdx_projects(id) ON DELETE CASCADE;


--
-- Name: zdx_timed zdx_timed_project_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_timed
    ADD CONSTRAINT zdx_timed_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.zdx_projects(id) ON DELETE CASCADE;


--
-- Name: zdx_todos zdx_todos_project_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_todos
    ADD CONSTRAINT zdx_todos_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.zdx_projects(id) ON DELETE CASCADE;


--
-- Name: zdx_work_log zdx_work_log_issue_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_work_log
    ADD CONSTRAINT zdx_work_log_issue_id_fkey FOREIGN KEY (issue_id) REFERENCES public.zdx_issues(id) ON DELETE CASCADE;


--
-- PostgreSQL database dump complete
--


