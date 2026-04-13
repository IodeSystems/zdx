--
-- PostgreSQL database dump
--

\restrict gPnroT0Dcv1iEN9U0JAvkg1Fq0OjnOZbfu9jMJzflJYtD3ljeZsA3JkUZrB5fMV

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
-- Name: schema_migrations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.schema_migrations (
    version bigint NOT NULL,
    dirty boolean NOT NULL
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
    component text DEFAULT ''::text NOT NULL
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
-- Name: zdx_id_seq; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_id_seq (
    project_id integer NOT NULL,
    kind text NOT NULL,
    next_val integer DEFAULT 1 NOT NULL
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
    blocked_by text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: zdx_projects; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_projects (
    id integer NOT NULL,
    slug text NOT NULL,
    name text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
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
-- Name: zdx_specs; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.zdx_specs (
    id integer NOT NULL,
    feature_id integer NOT NULL,
    description text NOT NULL,
    kind text DEFAULT 'must'::text NOT NULL
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
    completed_at timestamp with time zone
);


--
-- Name: zdx_features id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_features ALTER COLUMN id SET DEFAULT nextval('public.zdx_features_id_seq'::regclass);


--
-- Name: zdx_issue_work id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_issue_work ALTER COLUMN id SET DEFAULT nextval('public.zdx_issue_work_id_seq'::regclass);


--
-- Name: zdx_projects id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_projects ALTER COLUMN id SET DEFAULT nextval('public.zdx_projects_id_seq'::regclass);


--
-- Name: zdx_specs id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_specs ALTER COLUMN id SET DEFAULT nextval('public.zdx_specs_id_seq'::regclass);


--
-- Name: schema_migrations schema_migrations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.schema_migrations
    ADD CONSTRAINT schema_migrations_pkey PRIMARY KEY (version);


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
-- Name: zdx_id_seq zdx_id_seq_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_id_seq
    ADD CONSTRAINT zdx_id_seq_pkey PRIMARY KEY (project_id, kind);


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
-- Name: zdx_specs zdx_specs_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_specs
    ADD CONSTRAINT zdx_specs_pkey PRIMARY KEY (id);


--
-- Name: zdx_tasks zdx_tasks_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_tasks
    ADD CONSTRAINT zdx_tasks_pkey PRIMARY KEY (id);


--
-- Name: zdx_features zdx_features_project_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_features
    ADD CONSTRAINT zdx_features_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.zdx_projects(id);


--
-- Name: zdx_id_seq zdx_id_seq_project_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_id_seq
    ADD CONSTRAINT zdx_id_seq_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.zdx_projects(id);


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
-- Name: zdx_specs zdx_specs_feature_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_specs
    ADD CONSTRAINT zdx_specs_feature_id_fkey FOREIGN KEY (feature_id) REFERENCES public.zdx_features(id) ON DELETE CASCADE;


--
-- Name: zdx_tasks zdx_tasks_project_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.zdx_tasks
    ADD CONSTRAINT zdx_tasks_project_id_fkey FOREIGN KEY (project_id) REFERENCES public.zdx_projects(id);


--
-- PostgreSQL database dump complete
--

\unrestrict gPnroT0Dcv1iEN9U0JAvkg1Fq0OjnOZbfu9jMJzflJYtD3ljeZsA3JkUZrB5fMV

