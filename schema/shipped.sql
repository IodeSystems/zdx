--
-- PostgreSQL database dump
--


-- Dumped from database version 17.9 (Debian 17.9-1.pgdg13+1)
-- Dumped by pg_dump version 17.9 (Debian 17.9-1.pgdg13+1)

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
    status text DEFAULT ''::text NOT NULL
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
    created_at timestamp with time zone DEFAULT now() NOT NULL
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
    last_reviewed_at timestamp with time zone
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
    issue_type text DEFAULT 'ops'::text NOT NULL,
    duplicate_of text DEFAULT ''::text NOT NULL,
    url text DEFAULT ''::text NOT NULL
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
    created_at timestamp with time zone DEFAULT now() NOT NULL
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
    id boolean DEFAULT true NOT NULL,
    type text DEFAULT 'openai'::text NOT NULL,
    url text DEFAULT ''::text NOT NULL,
    model text DEFAULT ''::text NOT NULL,
    api_key text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT zdx_llm_configs_singleton CHECK ((id = true))
);


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
-- Name: zdx_plans; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_plans (
    id integer NOT NULL,
    feature_id integer NOT NULL,
    plan_type text DEFAULT 'implement'::text NOT NULL,
    status text DEFAULT 'pending'::text NOT NULL,
    complexity text DEFAULT ''::text NOT NULL,
    approach text DEFAULT ''::text NOT NULL,
    last_reviewed_at timestamp with time zone
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
    updated_at timestamp with time zone DEFAULT now() NOT NULL
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
    stage text DEFAULT ''::text NOT NULL
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
    created_at timestamp with time zone DEFAULT now() NOT NULL
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
    deferred boolean DEFAULT false NOT NULL
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
-- Name: zdx_tasks; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_tasks (
    id text NOT NULL,
    project_id integer NOT NULL,
    text text NOT NULL,
    feature text DEFAULT ''::text NOT NULL,
    status text DEFAULT 'pending'::text NOT NULL,
    reason text DEFAULT ''::text NOT NULL,
    issue text DEFAULT ''::text NOT NULL,
    depends text DEFAULT ''::text NOT NULL,
    test_plan text DEFAULT ''::text NOT NULL,
    test_refs text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    completed_at timestamp with time zone,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    task_group text DEFAULT ''::text NOT NULL,
    claimed_by text,
    claimed_at timestamp with time zone,
    lease_expires_at timestamp with time zone
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
    created_at timestamp with time zone DEFAULT now() NOT NULL
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
-- Name: zdx_theme_blockers; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_theme_blockers (
    theme_id integer NOT NULL,
    issue_id text NOT NULL
);


--
-- Name: zdx_themes; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_themes (
    id integer NOT NULL,
    project_id integer NOT NULL,
    name text NOT NULL,
    description text DEFAULT ''::text NOT NULL,
    priority integer DEFAULT 2 NOT NULL,
    status text DEFAULT 'active'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: zdx_themes_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.zdx_themes_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: zdx_themes_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.zdx_themes_id_seq OWNED BY public.zdx_themes.id;


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
    feature_id integer,
    text text NOT NULL,
    key text NOT NULL,
    persona text DEFAULT ''::text NOT NULL,
    priority integer DEFAULT 50 NOT NULL,
    status text DEFAULT 'open'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    resolved_at timestamp with time zone
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
-- Name: zdx_log_events id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_log_events ALTER COLUMN id SET DEFAULT nextval('public.zdx_log_events_id_seq'::regclass);


--
-- Name: zdx_oauth_identities id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_oauth_identities ALTER COLUMN id SET DEFAULT nextval('public.zdx_oauth_identities_id_seq'::regclass);


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
-- Name: zdx_questions id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_questions ALTER COLUMN id SET DEFAULT nextval('public.zdx_questions_id_seq'::regclass);


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
-- Name: zdx_themes id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_themes ALTER COLUMN id SET DEFAULT nextval('public.zdx_themes_id_seq'::regclass);


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
-- Name: zdx_log_events zdx_log_events_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_log_events
    ADD CONSTRAINT zdx_log_events_pkey PRIMARY KEY (id);


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
-- Name: zdx_plans zdx_plans_feature_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_plans
    ADD CONSTRAINT zdx_plans_feature_id_key UNIQUE (feature_id);


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
-- Name: zdx_questions zdx_questions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_questions
    ADD CONSTRAINT zdx_questions_pkey PRIMARY KEY (id);


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
-- Name: zdx_theme_blockers zdx_theme_blockers_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_theme_blockers
    ADD CONSTRAINT zdx_theme_blockers_pkey PRIMARY KEY (theme_id, issue_id);


--
-- Name: zdx_themes zdx_themes_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_themes
    ADD CONSTRAINT zdx_themes_pkey PRIMARY KEY (id);


--
-- Name: zdx_themes zdx_themes_project_id_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_themes
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
-- Name: idx_journal_project_role; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_journal_project_role ON public.zdx_journal_entries USING btree (project_id, role, date DESC);


--
-- Name: idx_oauth_identities_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_oauth_identities_user ON public.zdx_oauth_identities USING btree (user_id);


--
-- Name: idx_project_constraints_project; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_project_constraints_project ON public.zdx_project_constraints USING btree (project_id);


--
-- Name: idx_project_goals_project; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_project_goals_project ON public.zdx_project_goals USING btree (project_id);


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
-- Name: zdx_revisions_target; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX zdx_revisions_target ON public.zdx_revisions USING btree (project_id, target_type, target_id);


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
-- Name: zdx_features zdx_features_project_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_features
    ADD CONSTRAINT zdx_features_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.zdx_projects(id);


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
-- Name: zdx_oauth_identities zdx_oauth_identities_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_oauth_identities
    ADD CONSTRAINT zdx_oauth_identities_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.zdx_users(id) ON DELETE CASCADE;


--
-- Name: zdx_plans zdx_plans_feature_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_plans
    ADD CONSTRAINT zdx_plans_feature_id_fkey FOREIGN KEY (feature_id) REFERENCES public.zdx_features(id) ON DELETE CASCADE;


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
-- Name: zdx_tasks zdx_tasks_claimed_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_tasks
    ADD CONSTRAINT zdx_tasks_claimed_by_fkey FOREIGN KEY (claimed_by) REFERENCES public.zdx_agents(id);


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
-- Name: zdx_theme_blockers zdx_theme_blockers_issue_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_theme_blockers
    ADD CONSTRAINT zdx_theme_blockers_issue_id_fkey FOREIGN KEY (issue_id) REFERENCES public.zdx_issues(id) ON DELETE CASCADE;


--
-- Name: zdx_theme_blockers zdx_theme_blockers_theme_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_theme_blockers
    ADD CONSTRAINT zdx_theme_blockers_theme_id_fkey FOREIGN KEY (theme_id) REFERENCES public.zdx_themes(id) ON DELETE CASCADE;


--
-- Name: zdx_themes zdx_themes_project_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_themes
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
-- Name: zdx_todos zdx_todos_feature_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_todos
    ADD CONSTRAINT zdx_todos_feature_id_fkey FOREIGN KEY (feature_id) REFERENCES public.zdx_features(id) ON DELETE SET NULL;


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


