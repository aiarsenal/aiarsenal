-- ============================================================================
-- project.sql — 项目管理模块数据库与表结构
-- ============================================================================
-- 依赖：PostgreSQL 16（与 aiarsenal / aa_skill 同实例异库）
-- 关联：owner/assignee_id/reporter_id/user_id/dept_id 软引用 aiarsenal 库的 aa_user.user_id
--       （跨库无实际 FK，遵循 aiarsenal 既有模式：以 varchar(32) biz_id 软引用）
-- 约定：每表 id bigserial PK（内部）+ <实体>_id varchar(32) UNIQUE（对外业务 id）
--       时间 timestamp；布尔/状态 smallint；数组/复杂结构 text（jsonb）
-- 重建：psql -f resource/data/project.sql（每表 DROP IF EXISTS，可重复跑）
-- ============================================================================

-- ----------------------------------------------------------------------------
-- Part A — 独立数据库 aa_project
-- ----------------------------------------------------------------------------

SELECT 'CREATE DATABASE aa_project'
WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = 'aa_project')\gexec

\connect aa_project

-- ============================================================================
-- 基线固化说明（2026-09 squash）
-- 正文已替换为 pg_dump 固化版：合并了历史上全部增量迁移（原 migrations/ 已删，git 可查）。
-- 原文件尾部「
--
-- PostgreSQL database dump
--

\restrict cc1nIt63N3kXo2cP7c4qsCXEL8X0dPgB9F2T7h2EYTOjLm8BUic7UToUhaPdIwh

-- Dumped from database version 15.19 (Homebrew)
-- Dumped by pg_dump version 15.19 (Homebrew)

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

DROP INDEX IF EXISTS public.idx_aa_project_task_proj_status;
DROP INDEX IF EXISTS public.idx_aa_project_task_milestone;
DROP INDEX IF EXISTS public.idx_aa_project_task_id;
DROP INDEX IF EXISTS public.idx_aa_project_task_comment_task;
DROP INDEX IF EXISTS public.idx_aa_project_task_comment_id;
DROP INDEX IF EXISTS public.idx_aa_project_task_activity_task;
DROP INDEX IF EXISTS public.idx_aa_project_task_activity_proj;
DROP INDEX IF EXISTS public.idx_aa_project_task_activity_id;
DROP INDEX IF EXISTS public.idx_aa_project_tag_proj_name;
DROP INDEX IF EXISTS public.idx_aa_project_tag_id;
DROP INDEX IF EXISTS public.idx_aa_project_status_proj_key;
DROP INDEX IF EXISTS public.idx_aa_project_status_id;
DROP INDEX IF EXISTS public.idx_aa_project_status;
DROP INDEX IF EXISTS public.idx_aa_project_milestone_proj;
DROP INDEX IF EXISTS public.idx_aa_project_milestone_id;
DROP INDEX IF EXISTS public.idx_aa_project_member_proj_user;
DROP INDEX IF EXISTS public.idx_aa_project_member_proj;
DROP INDEX IF EXISTS public.idx_aa_project_member_id;
DROP INDEX IF EXISTS public.idx_aa_project_id;
DROP INDEX IF EXISTS public.idx_aa_project_custom_field_proj;
DROP INDEX IF EXISTS public.idx_aa_project_custom_field_id;
DROP INDEX IF EXISTS public.idx_aa_project_binding_unique;
DROP INDEX IF EXISTS public.idx_aa_project_binding_proj;
DROP INDEX IF EXISTS public.idx_aa_project_binding_id;
ALTER TABLE IF EXISTS ONLY public.aa_project_task DROP CONSTRAINT IF EXISTS aa_project_task_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_project_task_comment DROP CONSTRAINT IF EXISTS aa_project_task_comment_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_project_task_activity DROP CONSTRAINT IF EXISTS aa_project_task_activity_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_project_tag DROP CONSTRAINT IF EXISTS aa_project_tag_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_project_status DROP CONSTRAINT IF EXISTS aa_project_status_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_project DROP CONSTRAINT IF EXISTS aa_project_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_project_milestone DROP CONSTRAINT IF EXISTS aa_project_milestone_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_project_member DROP CONSTRAINT IF EXISTS aa_project_member_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_project_custom_field DROP CONSTRAINT IF EXISTS aa_project_custom_field_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_project_binding DROP CONSTRAINT IF EXISTS aa_project_binding_pkey;
ALTER TABLE IF EXISTS public.aa_project_task_comment ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_project_task_activity ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_project_task ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_project_tag ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_project_status ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_project_milestone ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_project_member ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_project_custom_field ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_project_binding ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_project ALTER COLUMN id DROP DEFAULT;
DROP SEQUENCE IF EXISTS public.aa_project_task_id_seq;
DROP SEQUENCE IF EXISTS public.aa_project_task_comment_id_seq;
DROP TABLE IF EXISTS public.aa_project_task_comment;
DROP SEQUENCE IF EXISTS public.aa_project_task_activity_id_seq;
DROP TABLE IF EXISTS public.aa_project_task_activity;
DROP TABLE IF EXISTS public.aa_project_task;
DROP SEQUENCE IF EXISTS public.aa_project_tag_id_seq;
DROP TABLE IF EXISTS public.aa_project_tag;
DROP SEQUENCE IF EXISTS public.aa_project_status_id_seq;
DROP TABLE IF EXISTS public.aa_project_status;
DROP SEQUENCE IF EXISTS public.aa_project_milestone_id_seq;
DROP TABLE IF EXISTS public.aa_project_milestone;
DROP SEQUENCE IF EXISTS public.aa_project_member_id_seq;
DROP TABLE IF EXISTS public.aa_project_member;
DROP SEQUENCE IF EXISTS public.aa_project_id_seq;
DROP SEQUENCE IF EXISTS public.aa_project_custom_field_id_seq;
DROP TABLE IF EXISTS public.aa_project_custom_field;
DROP SEQUENCE IF EXISTS public.aa_project_binding_id_seq;
DROP TABLE IF EXISTS public.aa_project_binding;
DROP TABLE IF EXISTS public.aa_project;
SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: aa_project; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_project (
    id bigint NOT NULL,
    project_id character varying(32) DEFAULT ''::character varying NOT NULL,
    name character varying(200) DEFAULT ''::character varying NOT NULL,
    code character varying(64) DEFAULT ''::character varying NOT NULL,
    description text,
    status character varying(32) DEFAULT 'planning'::character varying NOT NULL,
    priority character varying(32) DEFAULT 'medium'::character varying NOT NULL,
    progress integer DEFAULT 0 NOT NULL,
    owner character varying(32) DEFAULT ''::character varying NOT NULL,
    visibility character varying(32) DEFAULT 'private'::character varying NOT NULL,
    dept_id character varying(32) DEFAULT ''::character varying NOT NULL,
    start_date timestamp without time zone,
    due_date timestamp without time zone,
    color character varying(32) DEFAULT ''::character varying NOT NULL,
    tags text DEFAULT '[]'::text NOT NULL,
    settings text DEFAULT '{}'::text NOT NULL,
    archived smallint DEFAULT 0 NOT NULL,
    created_by character varying(32) DEFAULT '0'::character varying NOT NULL,
    updated_by character varying(32) DEFAULT '0'::character varying NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    deleted_at timestamp without time zone
);


--
-- Name: aa_project_binding; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_project_binding (
    id bigint NOT NULL,
    binding_id character varying(32) DEFAULT ''::character varying NOT NULL,
    project_id character varying(32) DEFAULT ''::character varying NOT NULL,
    kind character varying(32) DEFAULT ''::character varying NOT NULL,
    target_id character varying(32) DEFAULT ''::character varying NOT NULL,
    target_name character varying(200) DEFAULT ''::character varying NOT NULL,
    config text DEFAULT '{}'::text NOT NULL,
    enabled smallint DEFAULT 1 NOT NULL,
    created_by character varying(32) DEFAULT '0'::character varying NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    deleted_at timestamp without time zone
);


--
-- Name: aa_project_binding_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_project_binding_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_project_binding_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_project_binding_id_seq OWNED BY public.aa_project_binding.id;


--
-- Name: aa_project_custom_field; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_project_custom_field (
    id bigint NOT NULL,
    field_id character varying(32) DEFAULT ''::character varying NOT NULL,
    project_id character varying(32) DEFAULT ''::character varying NOT NULL,
    name character varying(64) DEFAULT ''::character varying NOT NULL,
    type character varying(32) DEFAULT 'text'::character varying NOT NULL,
    options text DEFAULT '[]'::text NOT NULL,
    sort_order integer DEFAULT 0 NOT NULL,
    created_by character varying(32) DEFAULT '0'::character varying NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    deleted_at timestamp without time zone
);


--
-- Name: aa_project_custom_field_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_project_custom_field_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_project_custom_field_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_project_custom_field_id_seq OWNED BY public.aa_project_custom_field.id;


--
-- Name: aa_project_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_project_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_project_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_project_id_seq OWNED BY public.aa_project.id;


--
-- Name: aa_project_member; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_project_member (
    id bigint NOT NULL,
    member_id character varying(32) DEFAULT ''::character varying NOT NULL,
    project_id character varying(32) DEFAULT ''::character varying NOT NULL,
    user_id character varying(32) DEFAULT ''::character varying NOT NULL,
    role character varying(32) DEFAULT 'member'::character varying NOT NULL,
    joined_at timestamp without time zone,
    created_by character varying(32) DEFAULT '0'::character varying NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    deleted_at timestamp without time zone
);


--
-- Name: aa_project_member_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_project_member_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_project_member_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_project_member_id_seq OWNED BY public.aa_project_member.id;


--
-- Name: aa_project_milestone; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_project_milestone (
    id bigint NOT NULL,
    milestone_id character varying(32) DEFAULT ''::character varying NOT NULL,
    project_id character varying(32) DEFAULT ''::character varying NOT NULL,
    name character varying(200) DEFAULT ''::character varying NOT NULL,
    description text,
    status character varying(32) DEFAULT 'open'::character varying NOT NULL,
    start_date timestamp without time zone,
    due_date timestamp without time zone,
    sort_order integer DEFAULT 0 NOT NULL,
    created_by character varying(32) DEFAULT '0'::character varying NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    deleted_at timestamp without time zone
);


--
-- Name: aa_project_milestone_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_project_milestone_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_project_milestone_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_project_milestone_id_seq OWNED BY public.aa_project_milestone.id;


--
-- Name: aa_project_status; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_project_status (
    id bigint NOT NULL,
    status_id character varying(32) DEFAULT ''::character varying NOT NULL,
    project_id character varying(32) DEFAULT ''::character varying NOT NULL,
    key character varying(32) DEFAULT ''::character varying NOT NULL,
    name character varying(64) DEFAULT ''::character varying NOT NULL,
    color character varying(32) DEFAULT ''::character varying NOT NULL,
    category character varying(16) DEFAULT 'todo'::character varying NOT NULL,
    sort_order integer DEFAULT 0 NOT NULL,
    is_default smallint DEFAULT 0 NOT NULL,
    created_by character varying(32) DEFAULT '0'::character varying NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    deleted_at timestamp without time zone
);


--
-- Name: aa_project_status_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_project_status_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_project_status_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_project_status_id_seq OWNED BY public.aa_project_status.id;


--
-- Name: aa_project_tag; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_project_tag (
    id bigint NOT NULL,
    tag_id character varying(32) DEFAULT ''::character varying NOT NULL,
    project_id character varying(32) DEFAULT ''::character varying NOT NULL,
    name character varying(64) DEFAULT ''::character varying NOT NULL,
    color character varying(32) DEFAULT ''::character varying NOT NULL,
    sort_order integer DEFAULT 0 NOT NULL,
    created_by character varying(32) DEFAULT '0'::character varying NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    deleted_at timestamp without time zone
);


--
-- Name: aa_project_tag_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_project_tag_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_project_tag_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_project_tag_id_seq OWNED BY public.aa_project_tag.id;


--
-- Name: aa_project_task; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_project_task (
    id bigint NOT NULL,
    task_id character varying(32) DEFAULT ''::character varying NOT NULL,
    project_id character varying(32) DEFAULT ''::character varying NOT NULL,
    parent_task_id character varying(32) DEFAULT ''::character varying NOT NULL,
    milestone_id character varying(32) DEFAULT ''::character varying NOT NULL,
    title character varying(255) DEFAULT ''::character varying NOT NULL,
    description text,
    status character varying(32) DEFAULT ''::character varying NOT NULL,
    priority character varying(32) DEFAULT 'medium'::character varying NOT NULL,
    assignee_id character varying(32) DEFAULT ''::character varying NOT NULL,
    reporter_id character varying(32) DEFAULT ''::character varying NOT NULL,
    start_date timestamp without time zone,
    due_date timestamp without time zone,
    done_at timestamp without time zone,
    sort_order integer DEFAULT 0 NOT NULL,
    tags text DEFAULT '[]'::text NOT NULL,
    custom_fields text DEFAULT '{}'::text NOT NULL,
    estimate_hours numeric(10,2) DEFAULT 0 NOT NULL,
    spent_hours numeric(10,2) DEFAULT 0 NOT NULL,
    created_by character varying(32) DEFAULT '0'::character varying NOT NULL,
    updated_by character varying(32) DEFAULT '0'::character varying NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    deleted_at timestamp without time zone
);


--
-- Name: aa_project_task_activity; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_project_task_activity (
    id bigint NOT NULL,
    activity_id character varying(32) DEFAULT ''::character varying NOT NULL,
    project_id character varying(32) DEFAULT ''::character varying NOT NULL,
    task_id character varying(32) DEFAULT ''::character varying NOT NULL,
    user_id character varying(32) DEFAULT ''::character varying NOT NULL,
    action character varying(32) DEFAULT ''::character varying NOT NULL,
    target character varying(255) DEFAULT ''::character varying NOT NULL,
    detail text,
    created_at timestamp without time zone
);


--
-- Name: aa_project_task_activity_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_project_task_activity_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_project_task_activity_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_project_task_activity_id_seq OWNED BY public.aa_project_task_activity.id;


--
-- Name: aa_project_task_comment; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_project_task_comment (
    id bigint NOT NULL,
    comment_id character varying(32) DEFAULT ''::character varying NOT NULL,
    task_id character varying(32) DEFAULT ''::character varying NOT NULL,
    project_id character varying(32) DEFAULT ''::character varying NOT NULL,
    user_id character varying(32) DEFAULT ''::character varying NOT NULL,
    content text,
    mentioned text DEFAULT '[]'::text NOT NULL,
    parent_comment_id character varying(32) DEFAULT ''::character varying NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    deleted_at timestamp without time zone
);


--
-- Name: aa_project_task_comment_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_project_task_comment_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_project_task_comment_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_project_task_comment_id_seq OWNED BY public.aa_project_task_comment.id;


--
-- Name: aa_project_task_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_project_task_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_project_task_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_project_task_id_seq OWNED BY public.aa_project_task.id;


--
-- Name: aa_project id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_project ALTER COLUMN id SET DEFAULT nextval('public.aa_project_id_seq'::regclass);


--
-- Name: aa_project_binding id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_project_binding ALTER COLUMN id SET DEFAULT nextval('public.aa_project_binding_id_seq'::regclass);


--
-- Name: aa_project_custom_field id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_project_custom_field ALTER COLUMN id SET DEFAULT nextval('public.aa_project_custom_field_id_seq'::regclass);


--
-- Name: aa_project_member id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_project_member ALTER COLUMN id SET DEFAULT nextval('public.aa_project_member_id_seq'::regclass);


--
-- Name: aa_project_milestone id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_project_milestone ALTER COLUMN id SET DEFAULT nextval('public.aa_project_milestone_id_seq'::regclass);


--
-- Name: aa_project_status id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_project_status ALTER COLUMN id SET DEFAULT nextval('public.aa_project_status_id_seq'::regclass);


--
-- Name: aa_project_tag id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_project_tag ALTER COLUMN id SET DEFAULT nextval('public.aa_project_tag_id_seq'::regclass);


--
-- Name: aa_project_task id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_project_task ALTER COLUMN id SET DEFAULT nextval('public.aa_project_task_id_seq'::regclass);


--
-- Name: aa_project_task_activity id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_project_task_activity ALTER COLUMN id SET DEFAULT nextval('public.aa_project_task_activity_id_seq'::regclass);


--
-- Name: aa_project_task_comment id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_project_task_comment ALTER COLUMN id SET DEFAULT nextval('public.aa_project_task_comment_id_seq'::regclass);


--
-- Data for Name: aa_project; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_project (id, project_id, name, code, description, status, priority, progress, owner, visibility, dept_id, start_date, due_date, color, tags, settings, archived, created_by, updated_by, created_at, updated_at, deleted_at) FROM stdin;
\.


--
-- Data for Name: aa_project_binding; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_project_binding (id, binding_id, project_id, kind, target_id, target_name, config, enabled, created_by, created_at, updated_at, deleted_at) FROM stdin;
\.


--
-- Data for Name: aa_project_custom_field; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_project_custom_field (id, field_id, project_id, name, type, options, sort_order, created_by, created_at, updated_at, deleted_at) FROM stdin;
\.


--
-- Data for Name: aa_project_member; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_project_member (id, member_id, project_id, user_id, role, joined_at, created_by, created_at, updated_at, deleted_at) FROM stdin;
\.


--
-- Data for Name: aa_project_milestone; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_project_milestone (id, milestone_id, project_id, name, description, status, start_date, due_date, sort_order, created_by, created_at, updated_at, deleted_at) FROM stdin;
\.


--
-- Data for Name: aa_project_status; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_project_status (id, status_id, project_id, key, name, color, category, sort_order, is_default, created_by, created_at, updated_at, deleted_at) FROM stdin;
\.


--
-- Data for Name: aa_project_tag; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_project_tag (id, tag_id, project_id, name, color, sort_order, created_by, created_at, updated_at, deleted_at) FROM stdin;
\.


--
-- Data for Name: aa_project_task; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_project_task (id, task_id, project_id, parent_task_id, milestone_id, title, description, status, priority, assignee_id, reporter_id, start_date, due_date, done_at, sort_order, tags, custom_fields, estimate_hours, spent_hours, created_by, updated_by, created_at, updated_at, deleted_at) FROM stdin;
\.


--
-- Data for Name: aa_project_task_activity; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_project_task_activity (id, activity_id, project_id, task_id, user_id, action, target, detail, created_at) FROM stdin;
\.


--
-- Data for Name: aa_project_task_comment; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_project_task_comment (id, comment_id, task_id, project_id, user_id, content, mentioned, parent_comment_id, created_at, updated_at, deleted_at) FROM stdin;
\.


--
-- Name: aa_project_binding_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_project_binding_id_seq', 1, false);


--
-- Name: aa_project_custom_field_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_project_custom_field_id_seq', 1, false);


--
-- Name: aa_project_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_project_id_seq', 1, false);


--
-- Name: aa_project_member_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_project_member_id_seq', 1, false);


--
-- Name: aa_project_milestone_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_project_milestone_id_seq', 1, false);


--
-- Name: aa_project_status_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_project_status_id_seq', 1, false);


--
-- Name: aa_project_tag_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_project_tag_id_seq', 1, false);


--
-- Name: aa_project_task_activity_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_project_task_activity_id_seq', 1, false);


--
-- Name: aa_project_task_comment_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_project_task_comment_id_seq', 1, false);


--
-- Name: aa_project_task_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_project_task_id_seq', 1, false);


--
-- Name: aa_project_binding aa_project_binding_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_project_binding
    ADD CONSTRAINT aa_project_binding_pkey PRIMARY KEY (id);


--
-- Name: aa_project_custom_field aa_project_custom_field_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_project_custom_field
    ADD CONSTRAINT aa_project_custom_field_pkey PRIMARY KEY (id);


--
-- Name: aa_project_member aa_project_member_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_project_member
    ADD CONSTRAINT aa_project_member_pkey PRIMARY KEY (id);


--
-- Name: aa_project_milestone aa_project_milestone_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_project_milestone
    ADD CONSTRAINT aa_project_milestone_pkey PRIMARY KEY (id);


--
-- Name: aa_project aa_project_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_project
    ADD CONSTRAINT aa_project_pkey PRIMARY KEY (id);


--
-- Name: aa_project_status aa_project_status_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_project_status
    ADD CONSTRAINT aa_project_status_pkey PRIMARY KEY (id);


--
-- Name: aa_project_tag aa_project_tag_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_project_tag
    ADD CONSTRAINT aa_project_tag_pkey PRIMARY KEY (id);


--
-- Name: aa_project_task_activity aa_project_task_activity_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_project_task_activity
    ADD CONSTRAINT aa_project_task_activity_pkey PRIMARY KEY (id);


--
-- Name: aa_project_task_comment aa_project_task_comment_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_project_task_comment
    ADD CONSTRAINT aa_project_task_comment_pkey PRIMARY KEY (id);


--
-- Name: aa_project_task aa_project_task_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_project_task
    ADD CONSTRAINT aa_project_task_pkey PRIMARY KEY (id);


--
-- Name: idx_aa_project_binding_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_project_binding_id ON public.aa_project_binding USING btree (binding_id);


--
-- Name: idx_aa_project_binding_proj; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_project_binding_proj ON public.aa_project_binding USING btree (project_id) WHERE (deleted_at IS NULL);


--
-- Name: idx_aa_project_binding_unique; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_project_binding_unique ON public.aa_project_binding USING btree (project_id, kind, target_id) WHERE (deleted_at IS NULL);


--
-- Name: idx_aa_project_custom_field_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_project_custom_field_id ON public.aa_project_custom_field USING btree (field_id);


--
-- Name: idx_aa_project_custom_field_proj; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_project_custom_field_proj ON public.aa_project_custom_field USING btree (project_id) WHERE (deleted_at IS NULL);


--
-- Name: idx_aa_project_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_project_id ON public.aa_project USING btree (project_id);


--
-- Name: idx_aa_project_member_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_project_member_id ON public.aa_project_member USING btree (member_id);


--
-- Name: idx_aa_project_member_proj; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_project_member_proj ON public.aa_project_member USING btree (project_id) WHERE (deleted_at IS NULL);


--
-- Name: idx_aa_project_member_proj_user; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_project_member_proj_user ON public.aa_project_member USING btree (project_id, user_id) WHERE (deleted_at IS NULL);


--
-- Name: idx_aa_project_milestone_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_project_milestone_id ON public.aa_project_milestone USING btree (milestone_id);


--
-- Name: idx_aa_project_milestone_proj; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_project_milestone_proj ON public.aa_project_milestone USING btree (project_id) WHERE (deleted_at IS NULL);


--
-- Name: idx_aa_project_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_project_status ON public.aa_project USING btree (status) WHERE (deleted_at IS NULL);


--
-- Name: idx_aa_project_status_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_project_status_id ON public.aa_project_status USING btree (status_id);


--
-- Name: idx_aa_project_status_proj_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_project_status_proj_key ON public.aa_project_status USING btree (project_id, key) WHERE (deleted_at IS NULL);


--
-- Name: idx_aa_project_tag_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_project_tag_id ON public.aa_project_tag USING btree (tag_id);


--
-- Name: idx_aa_project_tag_proj_name; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_project_tag_proj_name ON public.aa_project_tag USING btree (project_id, name) WHERE (deleted_at IS NULL);


--
-- Name: idx_aa_project_task_activity_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_project_task_activity_id ON public.aa_project_task_activity USING btree (activity_id);


--
-- Name: idx_aa_project_task_activity_proj; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_project_task_activity_proj ON public.aa_project_task_activity USING btree (project_id);


--
-- Name: idx_aa_project_task_activity_task; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_project_task_activity_task ON public.aa_project_task_activity USING btree (task_id);


--
-- Name: idx_aa_project_task_comment_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_project_task_comment_id ON public.aa_project_task_comment USING btree (comment_id);


--
-- Name: idx_aa_project_task_comment_task; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_project_task_comment_task ON public.aa_project_task_comment USING btree (task_id) WHERE (deleted_at IS NULL);


--
-- Name: idx_aa_project_task_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_project_task_id ON public.aa_project_task USING btree (task_id);


--
-- Name: idx_aa_project_task_milestone; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_project_task_milestone ON public.aa_project_task USING btree (milestone_id) WHERE (deleted_at IS NULL);


--
-- Name: idx_aa_project_task_proj_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_project_task_proj_status ON public.aa_project_task USING btree (project_id, status) WHERE (deleted_at IS NULL);


--
-- PostgreSQL database dump complete
--

\unrestrict cc1nIt63N3kXo2cP7c4qsCXEL8X0dPgB9F2T7h2EYTOjLm8BUic7UToUhaPdIwh

