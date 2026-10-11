-- ============================================================================
-- mcp.sql — MCP 聚合模块数据库与表结构
-- ============================================================================
-- 依赖：PostgreSQL 16（与 aiarsenal / aa_skill / aa_llm 同实例异库）
-- 关联：user_id / owner / key_id 软引用 aiarsenal 库的 aa_user.user_id（跨库无 FK）
-- 约定：每表 id bigserial PK（内部）+ <实体>_id varchar(32) UNIQUE（对外业务 id）
--       FK 列存 biz_id 字符串；时间用 timestamp；布尔/状态用 smallint；复杂结构用 jsonb
-- 设计：按 P1 范围裁剪并命名 aa_mcp_*；
--       智能路由（aa_mcp_tool_embedding 向量检索 /mcp/$smart，256 维 FNV LocalEmbedding + pgvector
--       ivfflat）与 OAuth（oauth_client/code/token，授权主体复用 Bearer Key，secret/code/token 只存
--       sha256，PKCE S256）P2 已并入基线
-- 重建：psql -f resource/data/mcp.sql（每表 DROP IF EXISTS，可重复跑）
-- ============================================================================

-- ----------------------------------------------------------------------------
-- Part A — 独立数据库 aa_mcp
-- ----------------------------------------------------------------------------

SELECT 'CREATE DATABASE aa_mcp'
WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = 'aa_mcp')\gexec

\connect aa_mcp

-- ============================================================================
-- 基线固化说明（2026-09 squash）
-- 正文已替换为 pg_dump 固化版：合并了历史上全部增量迁移（原 migrations/ 已删，git 可查）。
-- 原文件尾部「
--
-- PostgreSQL database dump
--

\restrict 17DM3Bz01EQ1TUFDnusKNsKSvMRtCGzkINfkf28WxxqUI0USxeseAQwtlLwJYb2

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

--
-- Name: vector; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS vector WITH SCHEMA public;

DROP INDEX IF EXISTS public.idx_aa_mcp_user_config_user;
DROP INDEX IF EXISTS public.idx_aa_mcp_tool_embedding_vec;
DROP INDEX IF EXISTS public.idx_aa_mcp_tool_embedding_uniq;
DROP INDEX IF EXISTS public.idx_aa_mcp_server_type;
DROP INDEX IF EXISTS public.idx_aa_mcp_server_owner;
DROP INDEX IF EXISTS public.idx_aa_mcp_server_name;
DROP INDEX IF EXISTS public.idx_aa_mcp_server_id;
DROP INDEX IF EXISTS public.idx_aa_mcp_oauth_token_refresh;
DROP INDEX IF EXISTS public.idx_aa_mcp_oauth_token_key;
DROP INDEX IF EXISTS public.idx_aa_mcp_oauth_token_biz;
DROP INDEX IF EXISTS public.idx_aa_mcp_oauth_token_access;
DROP INDEX IF EXISTS public.idx_aa_mcp_oauth_code_hash;
DROP INDEX IF EXISTS public.idx_aa_mcp_oauth_code_client;
DROP INDEX IF EXISTS public.idx_aa_mcp_oauth_code_biz;
DROP INDEX IF EXISTS public.idx_aa_mcp_oauth_client_id;
DROP INDEX IF EXISTS public.idx_aa_mcp_oauth_client_biz;
DROP INDEX IF EXISTS public.idx_aa_mcp_group_owner;
DROP INDEX IF EXISTS public.idx_aa_mcp_group_name;
DROP INDEX IF EXISTS public.idx_aa_mcp_group_id;
DROP INDEX IF EXISTS public.idx_aa_mcp_group_grant_target;
DROP INDEX IF EXISTS public.idx_aa_mcp_builtin_resource_uri;
DROP INDEX IF EXISTS public.idx_aa_mcp_builtin_resource_id;
DROP INDEX IF EXISTS public.idx_aa_mcp_builtin_prompt_name;
DROP INDEX IF EXISTS public.idx_aa_mcp_builtin_prompt_id;
DROP INDEX IF EXISTS public.idx_aa_mcp_bearer_key_owner;
DROP INDEX IF EXISTS public.idx_aa_mcp_bearer_key_id;
DROP INDEX IF EXISTS public.idx_aa_mcp_bearer_key_hash;
DROP INDEX IF EXISTS public.idx_aa_mcp_bearer_key_enabled;
DROP INDEX IF EXISTS public.idx_aa_mcp_activity_user;
DROP INDEX IF EXISTS public.idx_aa_mcp_activity_ts;
DROP INDEX IF EXISTS public.idx_aa_mcp_activity_tool;
DROP INDEX IF EXISTS public.idx_aa_mcp_activity_status;
DROP INDEX IF EXISTS public.idx_aa_mcp_activity_server;
DROP INDEX IF EXISTS public.idx_aa_mcp_activity_key;
DROP INDEX IF EXISTS public.idx_aa_mcp_activity_group;
ALTER TABLE IF EXISTS ONLY public.aa_mcp_user_config DROP CONSTRAINT IF EXISTS aa_mcp_user_config_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_mcp_tool_embedding DROP CONSTRAINT IF EXISTS aa_mcp_tool_embedding_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_mcp_oauth_token DROP CONSTRAINT IF EXISTS aa_mcp_oauth_token_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_mcp_oauth_code DROP CONSTRAINT IF EXISTS aa_mcp_oauth_code_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_mcp_oauth_client DROP CONSTRAINT IF EXISTS aa_mcp_oauth_client_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_mcp_server DROP CONSTRAINT IF EXISTS aa_mcp_server_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_mcp_group DROP CONSTRAINT IF EXISTS aa_mcp_group_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_mcp_group_grant DROP CONSTRAINT IF EXISTS aa_mcp_group_grant_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_mcp_config DROP CONSTRAINT IF EXISTS aa_mcp_config_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_mcp_builtin_resource DROP CONSTRAINT IF EXISTS aa_mcp_builtin_resource_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_mcp_builtin_prompt DROP CONSTRAINT IF EXISTS aa_mcp_builtin_prompt_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_mcp_bearer_key DROP CONSTRAINT IF EXISTS aa_mcp_bearer_key_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_mcp_activity DROP CONSTRAINT IF EXISTS aa_mcp_activity_pkey;
ALTER TABLE IF EXISTS public.aa_mcp_user_config ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_mcp_tool_embedding ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_mcp_oauth_token ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_mcp_oauth_code ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_mcp_oauth_client ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_mcp_server ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_mcp_group_grant ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_mcp_group ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_mcp_builtin_resource ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_mcp_builtin_prompt ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_mcp_bearer_key ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_mcp_activity ALTER COLUMN id DROP DEFAULT;
DROP SEQUENCE IF EXISTS public.aa_mcp_user_config_id_seq;
DROP TABLE IF EXISTS public.aa_mcp_user_config;
DROP SEQUENCE IF EXISTS public.aa_mcp_tool_embedding_id_seq;
DROP TABLE IF EXISTS public.aa_mcp_tool_embedding;
DROP SEQUENCE IF EXISTS public.aa_mcp_server_id_seq;
DROP TABLE IF EXISTS public.aa_mcp_server;
DROP SEQUENCE IF EXISTS public.aa_mcp_oauth_token_id_seq;
DROP TABLE IF EXISTS public.aa_mcp_oauth_token;
DROP SEQUENCE IF EXISTS public.aa_mcp_oauth_code_id_seq;
DROP TABLE IF EXISTS public.aa_mcp_oauth_code;
DROP SEQUENCE IF EXISTS public.aa_mcp_oauth_client_id_seq;
DROP TABLE IF EXISTS public.aa_mcp_oauth_client;
DROP SEQUENCE IF EXISTS public.aa_mcp_group_id_seq;
DROP SEQUENCE IF EXISTS public.aa_mcp_group_grant_id_seq;
DROP TABLE IF EXISTS public.aa_mcp_group_grant;
DROP TABLE IF EXISTS public.aa_mcp_group;
DROP TABLE IF EXISTS public.aa_mcp_config;
DROP SEQUENCE IF EXISTS public.aa_mcp_builtin_resource_id_seq;
DROP TABLE IF EXISTS public.aa_mcp_builtin_resource;
DROP SEQUENCE IF EXISTS public.aa_mcp_builtin_prompt_id_seq;
DROP TABLE IF EXISTS public.aa_mcp_builtin_prompt;
DROP SEQUENCE IF EXISTS public.aa_mcp_bearer_key_id_seq;
DROP TABLE IF EXISTS public.aa_mcp_bearer_key;
DROP SEQUENCE IF EXISTS public.aa_mcp_activity_id_seq;
DROP TABLE IF EXISTS public.aa_mcp_activity;
SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: aa_mcp_activity; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_mcp_activity (
    id bigint NOT NULL,
    server_name character varying(100) DEFAULT ''::character varying NOT NULL,
    tool character varying(255) DEFAULT ''::character varying NOT NULL,
    duration_ms integer DEFAULT 0 NOT NULL,
    status character varying(20) DEFAULT 'success'::character varying NOT NULL,
    input text,
    output text,
    group_name character varying(100) DEFAULT ''::character varying NOT NULL,
    user_id character varying(32) DEFAULT ''::character varying NOT NULL,
    key_id character varying(32) DEFAULT ''::character varying NOT NULL,
    key_name character varying(100) DEFAULT ''::character varying NOT NULL,
    source_ip character varying(64) DEFAULT ''::character varying NOT NULL,
    error_message text,
    ts timestamp without time zone
);


--
-- Name: aa_mcp_activity_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_mcp_activity_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_mcp_activity_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_mcp_activity_id_seq OWNED BY public.aa_mcp_activity.id;


--
-- Name: aa_mcp_bearer_key; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_mcp_bearer_key (
    id bigint NOT NULL,
    key_id character varying(32) DEFAULT ''::character varying NOT NULL,
    name character varying(100) DEFAULT ''::character varying NOT NULL,
    token_hash character varying(64) DEFAULT ''::character varying NOT NULL,
    enabled smallint DEFAULT 1 NOT NULL,
    kind character varying(20) DEFAULT 'user'::character varying NOT NULL,
    owner character varying(32) DEFAULT '0'::character varying NOT NULL,
    access_type character varying(20) DEFAULT 'all'::character varying NOT NULL,
    allowed_groups jsonb DEFAULT '[]'::jsonb NOT NULL,
    allowed_servers jsonb DEFAULT '[]'::jsonb NOT NULL,
    created_by character varying(32) DEFAULT '0'::character varying NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone
);


--
-- Name: aa_mcp_bearer_key_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_mcp_bearer_key_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_mcp_bearer_key_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_mcp_bearer_key_id_seq OWNED BY public.aa_mcp_bearer_key.id;


--
-- Name: aa_mcp_builtin_prompt; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_mcp_builtin_prompt (
    id bigint NOT NULL,
    prompt_id character varying(32) DEFAULT ''::character varying NOT NULL,
    name character varying(255) DEFAULT ''::character varying NOT NULL,
    title character varying(255) DEFAULT ''::character varying NOT NULL,
    description text,
    template text DEFAULT ''::text NOT NULL,
    arguments jsonb DEFAULT '[]'::jsonb NOT NULL,
    enabled smallint DEFAULT 1 NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone
);


--
-- Name: aa_mcp_builtin_prompt_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_mcp_builtin_prompt_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_mcp_builtin_prompt_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_mcp_builtin_prompt_id_seq OWNED BY public.aa_mcp_builtin_prompt.id;


--
-- Name: aa_mcp_builtin_resource; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_mcp_builtin_resource (
    id bigint NOT NULL,
    resource_id character varying(32) DEFAULT ''::character varying NOT NULL,
    uri character varying(1024) DEFAULT ''::character varying NOT NULL,
    name character varying(255) DEFAULT ''::character varying NOT NULL,
    description text,
    mime_type character varying(255) DEFAULT ''::character varying NOT NULL,
    content text DEFAULT ''::text NOT NULL,
    enabled smallint DEFAULT 1 NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone
);


--
-- Name: aa_mcp_builtin_resource_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_mcp_builtin_resource_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_mcp_builtin_resource_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_mcp_builtin_resource_id_seq OWNED BY public.aa_mcp_builtin_resource.id;


--
-- Name: aa_mcp_config; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_mcp_config (
    id character varying(50) DEFAULT 'default'::character varying NOT NULL,
    routing jsonb DEFAULT '{}'::jsonb NOT NULL,
    install jsonb DEFAULT '{}'::jsonb NOT NULL,
    tool_result_compression jsonb DEFAULT '{}'::jsonb NOT NULL,
    mcp_router jsonb DEFAULT '{}'::jsonb NOT NULL,
    name_separator character varying(10) DEFAULT '/'::character varying NOT NULL,
    auth jsonb DEFAULT '{}'::jsonb NOT NULL,
    discovery jsonb DEFAULT '{}'::jsonb NOT NULL,
    activity_log jsonb DEFAULT '{}'::jsonb NOT NULL,
    enable_session_rebuild smallint DEFAULT 0 NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone
);


--
-- Name: aa_mcp_group; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_mcp_group (
    id bigint NOT NULL,
    group_id character varying(32) DEFAULT ''::character varying NOT NULL,
    name character varying(100) DEFAULT ''::character varying NOT NULL,
    description text,
    servers jsonb DEFAULT '[]'::jsonb NOT NULL,
    visibility smallint DEFAULT 0 NOT NULL,
    owner character varying(32) DEFAULT '0'::character varying NOT NULL,
    created_by character varying(32) DEFAULT '0'::character varying NOT NULL,
    updated_by character varying(32) DEFAULT '0'::character varying NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    deleted_at timestamp without time zone
);


--
-- Name: aa_mcp_group_grant; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_mcp_group_grant (
    id bigint NOT NULL,
    target_id character varying(32) DEFAULT ''::character varying NOT NULL,
    user_ids jsonb DEFAULT '[]'::jsonb NOT NULL,
    dept_id character varying(32) DEFAULT ''::character varying NOT NULL,
    role_ids jsonb DEFAULT '[]'::jsonb NOT NULL,
    post_ids jsonb DEFAULT '[]'::jsonb NOT NULL,
    exclude boolean DEFAULT false NOT NULL,
    group_id character varying(32) DEFAULT ''::character varying NOT NULL,
    set_id character varying(32) DEFAULT ''::character varying NOT NULL,
    created_by character varying(32) DEFAULT '0'::character varying NOT NULL,
    created_at timestamp without time zone DEFAULT now()
);


--
-- Name: aa_mcp_group_grant_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_mcp_group_grant_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_mcp_group_grant_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_mcp_group_grant_id_seq OWNED BY public.aa_mcp_group_grant.id;


--
-- Name: aa_mcp_group_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_mcp_group_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_mcp_group_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_mcp_group_id_seq OWNED BY public.aa_mcp_group.id;


--
--
-- Name: aa_mcp_oauth_client; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_mcp_oauth_client (
    id bigint NOT NULL,
    biz_id character varying(32) DEFAULT ''::character varying NOT NULL,
    client_id character varying(64) DEFAULT ''::character varying NOT NULL,
    client_secret_hash character varying(64) DEFAULT ''::character varying NOT NULL,
    client_name character varying(255) DEFAULT ''::character varying NOT NULL,
    redirect_uris jsonb DEFAULT '[]'::jsonb NOT NULL,
    grant_types jsonb DEFAULT '[]'::jsonb NOT NULL,
    scope text DEFAULT ''::text NOT NULL,
    is_enabled smallint DEFAULT 1 NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone
);

--
-- Name: aa_mcp_oauth_client_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_mcp_oauth_client_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

--
-- Name: aa_mcp_oauth_client_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_mcp_oauth_client_id_seq OWNED BY public.aa_mcp_oauth_client.id;

--
-- Name: aa_mcp_oauth_code; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_mcp_oauth_code (
    id bigint NOT NULL,
    biz_id character varying(32) DEFAULT ''::character varying NOT NULL,
    code_hash character varying(64) DEFAULT ''::character varying NOT NULL,
    client_id character varying(64) DEFAULT ''::character varying NOT NULL,
    redirect_uri text DEFAULT ''::text NOT NULL,
    bearer_key_id character varying(32) DEFAULT ''::character varying NOT NULL,
    scope text DEFAULT ''::text NOT NULL,
    access_type character varying(20) DEFAULT 'all'::character varying NOT NULL,
    code_challenge character varying(128) DEFAULT ''::character varying NOT NULL,
    code_challenge_method character varying(10) DEFAULT 'S256'::character varying NOT NULL,
    expires_at timestamp without time zone,
    used_at timestamp without time zone,
    created_at timestamp without time zone
);

--
-- Name: aa_mcp_oauth_code_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_mcp_oauth_code_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

--
-- Name: aa_mcp_oauth_code_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_mcp_oauth_code_id_seq OWNED BY public.aa_mcp_oauth_code.id;

--
-- Name: aa_mcp_oauth_token; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_mcp_oauth_token (
    id bigint NOT NULL,
    biz_id character varying(32) DEFAULT ''::character varying NOT NULL,
    access_token_hash character varying(64) DEFAULT ''::character varying NOT NULL,
    refresh_token_hash character varying(64) DEFAULT ''::character varying NOT NULL,
    client_id character varying(64) DEFAULT ''::character varying NOT NULL,
    bearer_key_id character varying(32) DEFAULT ''::character varying NOT NULL,
    scope text DEFAULT ''::text NOT NULL,
    access_type character varying(20) DEFAULT 'all'::character varying NOT NULL,
    expires_at timestamp without time zone,
    revoked_at timestamp without time zone,
    created_at timestamp without time zone,
    updated_at timestamp without time zone
);

--
-- Name: aa_mcp_oauth_token_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_mcp_oauth_token_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

--
-- Name: aa_mcp_oauth_token_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_mcp_oauth_token_id_seq OWNED BY public.aa_mcp_oauth_token.id;

-- Name: aa_mcp_server; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_mcp_server (
    id bigint NOT NULL,
    server_id character varying(32) DEFAULT ''::character varying NOT NULL,
    name character varying(100) DEFAULT ''::character varying NOT NULL,
    type character varying(20) DEFAULT 'stdio'::character varying NOT NULL,
    description text,
    url character varying(500) DEFAULT ''::character varying NOT NULL,
    command character varying(500) DEFAULT ''::character varying NOT NULL,
    args jsonb DEFAULT '[]'::jsonb NOT NULL,
    env jsonb DEFAULT '{}'::jsonb NOT NULL,
    headers jsonb DEFAULT '{}'::jsonb NOT NULL,
    options jsonb DEFAULT '{}'::jsonb NOT NULL,
    oauth jsonb DEFAULT '{}'::jsonb NOT NULL,
    proxy jsonb DEFAULT '{}'::jsonb NOT NULL,
    openapi jsonb DEFAULT '{}'::jsonb NOT NULL,
    passthrough_headers jsonb DEFAULT '[]'::jsonb NOT NULL,
    enabled smallint DEFAULT 1 NOT NULL,
    owner character varying(32) DEFAULT '0'::character varying NOT NULL,
    visibility character varying(16) DEFAULT 'private'::character varying NOT NULL,
    enable_keep_alive smallint DEFAULT 0 NOT NULL,
    keep_alive_interval integer DEFAULT 60 NOT NULL,
    tools jsonb DEFAULT '{}'::jsonb NOT NULL,
    prompts jsonb DEFAULT '{}'::jsonb NOT NULL,
    resources jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_by character varying(32) DEFAULT '0'::character varying NOT NULL,
    updated_by character varying(32) DEFAULT '0'::character varying NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    deleted_at timestamp without time zone
);


--
-- Name: aa_mcp_server_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_mcp_server_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_mcp_server_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_mcp_server_id_seq OWNED BY public.aa_mcp_server.id;


--
--
-- Name: aa_mcp_tool_embedding; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_mcp_tool_embedding (
    id bigint NOT NULL,
    server_name character varying(100) DEFAULT ''::character varying NOT NULL,
    tool_name character varying(255) DEFAULT ''::character varying NOT NULL,
    description text DEFAULT ''::text NOT NULL,
    embedding public.vector(256),
    updated_at timestamp without time zone
);

--
-- Name: aa_mcp_tool_embedding_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_mcp_tool_embedding_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

--
-- Name: aa_mcp_tool_embedding_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_mcp_tool_embedding_id_seq OWNED BY public.aa_mcp_tool_embedding.id;

-- Name: aa_mcp_user_config; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_mcp_user_config (
    id bigint NOT NULL,
    user_id character varying(32) DEFAULT ''::character varying NOT NULL,
    routing jsonb DEFAULT '{}'::jsonb NOT NULL,
    additional_config jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone
);


--
-- Name: aa_mcp_user_config_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_mcp_user_config_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_mcp_user_config_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_mcp_user_config_id_seq OWNED BY public.aa_mcp_user_config.id;


--
-- Name: aa_mcp_activity id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_mcp_activity ALTER COLUMN id SET DEFAULT nextval('public.aa_mcp_activity_id_seq'::regclass);


--
-- Name: aa_mcp_bearer_key id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_mcp_bearer_key ALTER COLUMN id SET DEFAULT nextval('public.aa_mcp_bearer_key_id_seq'::regclass);


--
-- Name: aa_mcp_builtin_prompt id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_mcp_builtin_prompt ALTER COLUMN id SET DEFAULT nextval('public.aa_mcp_builtin_prompt_id_seq'::regclass);


--
-- Name: aa_mcp_builtin_resource id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_mcp_builtin_resource ALTER COLUMN id SET DEFAULT nextval('public.aa_mcp_builtin_resource_id_seq'::regclass);


--
-- Name: aa_mcp_group id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_mcp_group ALTER COLUMN id SET DEFAULT nextval('public.aa_mcp_group_id_seq'::regclass);


--
-- Name: aa_mcp_group_grant id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_mcp_group_grant ALTER COLUMN id SET DEFAULT nextval('public.aa_mcp_group_grant_id_seq'::regclass);


--
--
-- Name: aa_mcp_oauth_client id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_mcp_oauth_client ALTER COLUMN id SET DEFAULT nextval('public.aa_mcp_oauth_client_id_seq'::regclass);

--
-- Name: aa_mcp_oauth_code id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_mcp_oauth_code ALTER COLUMN id SET DEFAULT nextval('public.aa_mcp_oauth_code_id_seq'::regclass);

--
-- Name: aa_mcp_oauth_token id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_mcp_oauth_token ALTER COLUMN id SET DEFAULT nextval('public.aa_mcp_oauth_token_id_seq'::regclass);

-- Name: aa_mcp_server id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_mcp_server ALTER COLUMN id SET DEFAULT nextval('public.aa_mcp_server_id_seq'::regclass);


--
--
-- Name: aa_mcp_tool_embedding id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_mcp_tool_embedding ALTER COLUMN id SET DEFAULT nextval('public.aa_mcp_tool_embedding_id_seq'::regclass);

-- Name: aa_mcp_user_config id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_mcp_user_config ALTER COLUMN id SET DEFAULT nextval('public.aa_mcp_user_config_id_seq'::regclass);


--
-- Data for Name: aa_mcp_activity; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_mcp_activity (id, server_name, tool, duration_ms, status, input, output, group_name, user_id, key_id, key_name, source_ip, error_message, ts) FROM stdin;
\.


--
-- Data for Name: aa_mcp_bearer_key; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_mcp_bearer_key (id, key_id, name, token_hash, enabled, kind, owner, access_type, allowed_groups, allowed_servers, created_by, created_at, updated_at) FROM stdin;
\.


--
-- Data for Name: aa_mcp_builtin_prompt; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_mcp_builtin_prompt (id, prompt_id, name, title, description, template, arguments, enabled, created_at, updated_at) FROM stdin;
\.


--
-- Data for Name: aa_mcp_builtin_resource; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_mcp_builtin_resource (id, resource_id, uri, name, description, mime_type, content, enabled, created_at, updated_at) FROM stdin;
\.


--
-- Data for Name: aa_mcp_config; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_mcp_config (id, routing, install, tool_result_compression, mcp_router, name_separator, auth, discovery, activity_log, enable_session_rebuild, created_at, updated_at) FROM stdin;
default	{}	{}	{}	{}	/	{}	{}	{}	0	2026-09-16 22:49:37.445528	2026-09-16 22:49:37.445528
\.


--
-- Data for Name: aa_mcp_group; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_mcp_group (id, group_id, name, description, servers, visibility, owner, created_by, updated_by, created_at, updated_at, deleted_at) FROM stdin;
\.


--
-- Data for Name: aa_mcp_group_grant; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_mcp_group_grant (id, target_id, user_ids, dept_id, role_ids, post_ids, exclude, group_id, set_id, created_by, created_at) FROM stdin;
\.


--
-- Data for Name: aa_mcp_oauth_client; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_mcp_oauth_client (id, biz_id, client_id, client_secret_hash, client_name, redirect_uris, grant_types, scope, is_enabled, created_at, updated_at) FROM stdin;
\.

-- Data for Name: aa_mcp_oauth_code; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_mcp_oauth_code (id, biz_id, code_hash, client_id, redirect_uri, bearer_key_id, scope, access_type, code_challenge, code_challenge_method, expires_at, used_at, created_at) FROM stdin;
\.

-- Data for Name: aa_mcp_oauth_token; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_mcp_oauth_token (id, biz_id, access_token_hash, refresh_token_hash, client_id, bearer_key_id, scope, access_type, expires_at, revoked_at, created_at, updated_at) FROM stdin;
\.

-- Data for Name: aa_mcp_server; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_mcp_server (id, server_id, name, type, description, url, command, args, env, headers, options, oauth, proxy, openapi, passthrough_headers, enabled, owner, visibility, enable_keep_alive, keep_alive_interval, tools, prompts, resources, created_by, updated_by, created_at, updated_at, deleted_at) FROM stdin;
34	mcpdemoserverseed00000000000000a	demo	stdio	内置演示 server：echo/now/calc/longtext/slow		bin/mcp-demo	[]	{}	{}	{}	{}	{}	{}	[]	1		public	0	60	{}	{}	{}	0	0	2026-09-17 05:40:59.235063	2026-09-17 05:40:59.235063	\N
35	mcpdemohttpserverseed0000000000b	demo-http	streamable-http	演示 server http 模式（需另起：bin/mcp-demo --transport http --port 43219）	http://127.0.0.1:43219/mcp		[]	{}	{}	{}	{}	{}	{}	[]	0		public	0	60	{}	{}	{}	0	0	2026-09-17 05:40:59.235063	2026-09-17 05:40:59.235063	\N
36	mcpdemo1501demoweather00000000a	demo-weather	stdio	天气查询演示：weather/forecast（合成数据，同城市可复现）		bin/mcp-demo	["--name", "demo-weather"]	{}	{}	{}	{}	{}	{}	[]	1		public	0	60	{}	{}	{}	0	0	2026-09-17 05:40:59.2497	2026-09-17 05:40:59.2497	\N
37	mcpdemo1502demosearch000000000b	demo-search	stdio	网页搜索演示：search（合成结果，无真实检索）		bin/mcp-demo	["--name", "demo-search"]	{}	{}	{}	{}	{}	{}	[]	1		public	0	60	{}	{}	{}	0	0	2026-09-17 05:40:59.2497	2026-09-17 05:40:59.2497	\N
38	mcpdemo1503demotranslate000000c	demo-translate	stdio	文本翻译演示：translate（加注目标语言标签，无真实模型）		bin/mcp-demo	["--name", "demo-translate"]	{}	{}	{}	{}	{}	{}	[]	1		public	0	60	{}	{}	{}	0	0	2026-09-17 05:40:59.2497	2026-09-17 05:40:59.2497	\N
39	mcpdemo1504democalc00000000000d	demo-calc	stdio	数学计算演示：calc 四则/random 区间随机		bin/mcp-demo	["--name", "demo-calc"]	{}	{}	{}	{}	{}	{}	[]	1		public	0	60	{}	{}	{}	0	0	2026-09-17 05:40:59.2497	2026-09-17 05:40:59.2497	\N
40	mcpdemo1505demouuid00000000000e	demo-uuid	stdio	ID 生成演示：uuid 批量 v4 形态（crypto 随机）		bin/mcp-demo	["--name", "demo-uuid"]	{}	{}	{}	{}	{}	{}	[]	1		public	0	60	{}	{}	{}	0	0	2026-09-17 05:40:59.2497	2026-09-17 05:40:59.2497	\N
41	mcpdemo1506demoqrcode000000000f	demo-qrcode	stdio	二维码演示：qrcode 占位元数据（不生成真实图片）		bin/mcp-demo	["--name", "demo-qrcode"]	{}	{}	{}	{}	{}	{}	[]	1		public	0	60	{}	{}	{}	0	0	2026-09-17 05:40:59.2497	2026-09-17 05:40:59.2497	\N
42	mcpdemo1507demodatetime0000000g	demo-datetime	stdio	日期时间演示：now 当前时间/weekday 星期推算		bin/mcp-demo	["--name", "demo-datetime"]	{}	{}	{}	{}	{}	{}	[]	1		public	0	60	{}	{}	{}	0	0	2026-09-17 05:40:59.2497	2026-09-17 05:40:59.2497	\N
43	mcpdemo1508demoip0000000000000h	demo-ip	stdio	IP 归属演示：lookup（按 IP 哈希合成地区/运营商）		bin/mcp-demo	["--name", "demo-ip"]	{}	{}	{}	{}	{}	{}	[]	1		public	0	60	{}	{}	{}	0	0	2026-09-17 05:40:59.2497	2026-09-17 05:40:59.2497	\N
44	mcpdemo1509demoexchange0000000i	demo-exchange	stdio	汇率换算演示：rate/convert（固定演示牌价 CNY 基准）		bin/mcp-demo	["--name", "demo-exchange"]	{}	{}	{}	{}	{}	{}	[]	1		public	0	60	{}	{}	{}	0	0	2026-09-17 05:40:59.2497	2026-09-17 05:40:59.2497	\N
45	mcpdemo1510demostock0000000000j	demo-stock	stdio	行情演示：quote/trend（按代码哈希合成价格走势）		bin/mcp-demo	["--name", "demo-stock"]	{}	{}	{}	{}	{}	{}	[]	1		public	0	60	{}	{}	{}	0	0	2026-09-17 05:40:59.2497	2026-09-17 05:40:59.2497	\N
46	mcpdemo1511demomail00000000000k	demo-mail	stdio	邮件发送演示：send 登记并返回 message-id（不真实投递）		bin/mcp-demo	["--name", "demo-mail"]	{}	{}	{}	{}	{}	{}	[]	1		public	0	60	{}	{}	{}	0	0	2026-09-17 05:40:59.2497	2026-09-17 05:40:59.2497	\N
47	mcpdemo1512demostorage00000000l	demo-storage	stdio	对象存储演示：put 写入返回 etag/list 按前缀列对象		bin/mcp-demo	["--name", "demo-storage"]	{}	{}	{}	{}	{}	{}	[]	1		public	0	60	{}	{}	{}	0	0	2026-09-17 05:40:59.2497	2026-09-17 05:40:59.2497	\N
48	mcpdemo1513demonotify000000000m	demo-notify	stdio	消息通知演示：send 到 im/sms/webhook 返回回执		bin/mcp-demo	["--name", "demo-notify"]	{}	{}	{}	{}	{}	{}	[]	1		public	0	60	{}	{}	{}	0	0	2026-09-17 05:40:59.2497	2026-09-17 05:40:59.2497	\N
49	mcpdemo1514demogit000000000000n	demo-git	stdio	代码仓库演示：commits/branches（合成提交与分支）		bin/mcp-demo	["--name", "demo-git"]	{}	{}	{}	{}	{}	{}	[]	1		public	0	60	{}	{}	{}	0	0	2026-09-17 05:40:59.2497	2026-09-17 05:40:59.2497	\N
50	mcpdemo1515demotext00000000000o	demo-text	stdio	文本处理演示：transform 变形/count 统计		bin/mcp-demo	["--name", "demo-text"]	{}	{}	{}	{}	{}	{}	[]	1		public	0	60	{}	{}	{}	0	0	2026-09-17 05:40:59.2497	2026-09-17 05:40:59.2497	\N
51	mcpopsplatform0000000000000000a	platform-ops	streamable-http	企业内部能力（真实数据）：平台总览/agent实例/桌面设备/技能库/MCP审计 只读查询，代码 mcp-ops/	http://127.0.0.1:43220/mcp		[]	{}	{}	{}	{}	{}	{}	[]	1		public	0	60	{}	{}	{}	0	0	2026-09-17 05:40:59.263108	2026-09-17 05:40:59.263108	\N
52	mcpopsdocread000000000000000a	platform-doc	streamable-http	文档库只读（真实数据）：doc_search 语义检索 / doc_read 读全文 / doc_list 列表，代码 api/interfaces/ops	http://127.0.0.1:43220/mcp/doc		[]	{}	{}	{}	{}	{}	{}	[]	1		public	0	60	{}	{}	{}	0	0	2026-09-17 05:40:59.276108	2026-09-17 05:40:59.276108	\N
53	mcpopsdocwrite000000000000000a	platform-doc-write	streamable-http	文档库写入（真实数据）：doc_create 新建 / doc_update 更新（自动重嵌向量），代码 api/interfaces/ops	http://127.0.0.1:43220/mcp/doc-write		[]	{}	{}	{}	{}	{}	{}	[]	1		public	0	60	{}	{}	{}	0	0	2026-09-17 05:40:59.276108	2026-09-17 05:40:59.276108	\N
54	mcpopskbread000000000000000a	platform-kb	streamable-http	知识库只读（真实数据）：kb_search 语义检索 / kb_read 读全文 / kb_list 列表，代码 api/interfaces/ops	http://127.0.0.1:43220/mcp/kb		[]	{}	{}	{}	{}	{}	{}	[]	1		public	0	60	{}	{}	{}	0	0	2026-09-17 05:40:59.290758	2026-09-17 05:40:59.290758	\N
55	mcpopskbwrite0000000000000000a	platform-kb-write	streamable-http	知识库写入（真实数据）：kb_create 新建 / kb_update 更新（自动重嵌向量），代码 api/interfaces/ops	http://127.0.0.1:43220/mcp/kb-write		[]	{}	{}	{}	{}	{}	{}	[]	1		public	0	60	{}	{}	{}	0	0	2026-09-17 05:40:59.290758	2026-09-17 05:40:59.290758	\N
56	mcpopsskillread000000000000000a	platform-skill	streamable-http	技能库只读（真实数据）：skill_list/skill_get/skill_search 语义检索/版本/市场源/安全审计/实例已装技能 查询，代码 api/interfaces/ops	http://127.0.0.1:43220/mcp/skill		[]	{}	{}	{}	{}	{}	{}	[]	1		public	0	60	{}	{}	{}	0	0	2026-09-17 05:40:59.305323	2026-09-17 05:40:59.305323	\N
57	mcpopsskillwrite00000000000000a	platform-skill-write	streamable-http	技能库写入（真实数据）：市场同步/URL 导入/版本发布下架/给 agent 实例装卸技能，代码 api/interfaces/ops	http://127.0.0.1:43220/mcp/skill-write		[]	{}	{}	{}	{}	{}	{}	[]	1		public	0	60	{}	{}	{}	0	0	2026-09-17 05:40:59.305323	2026-09-17 05:40:59.305323	\N
58	mcpopsprojectread00000000000000a	platform-project	streamable-http	项目只读（真实数据）：项目/任务/里程碑/看板/动态/报告/时间线/成员 查询，代码 api/interfaces/ops	http://127.0.0.1:43220/mcp/project		[]	{}	{}	{}	{}	{}	{}	[]	1		public	0	60	{}	{}	{}	0	0	2026-09-17 05:40:59.320971	2026-09-17 05:40:59.320971	\N
59	mcpopsprojectwrite0000000000000a	platform-project-write	streamable-http	项目写入（真实数据）：项目与任务的创建更新/看板移动任务/任务评论/里程碑/加成员，代码 api/interfaces/ops	http://127.0.0.1:43220/mcp/project-write		[]	{}	{}	{}	{}	{}	{}	[]	1		public	0	60	{}	{}	{}	0	0	2026-09-17 05:40:59.320971	2026-09-17 05:40:59.320971	\N
60	mcpopsnoticeread000000000000000a	platform-notice	streamable-http	通知只读（真实数据）：notice_list 我的站内通知/notice_unread_count 未读数/notice_user_lookup 收件人解析，代码 api/interfaces/ops	http://127.0.0.1:43220/mcp/notice		[]	{}	{}	{}	{}	{}	{}	[]	1		public	0	60	{}	{}	{}	0	0	2026-09-17 05:40:59.336022	2026-09-17 05:40:59.336022	\N
61	mcpopsnoticewrite00000000000000a	platform-notice-write	streamable-http	通知写入（真实数据）：notice_send 发站内通知/标记已读，代码 api/interfaces/ops	http://127.0.0.1:43220/mcp/notice-write		[]	{}	{}	{}	{}	{}	{}	[]	1		public	0	60	{}	{}	{}	0	0	2026-09-17 05:40:59.336022	2026-09-17 05:40:59.336022	\N
62	mcpopsuserread00000000000000000a	platform-user	streamable-http	员工信息查询（真实数据）：user_lookup 姓名工号找人/user_get 员工档案/dept_list 部门树/dept_user_list 部门花名册，代码 api/interfaces/ops	http://127.0.0.1:43220/mcp/user		[]	{}	{}	{}	{}	{}	{}	[]	1		public	0	60	{}	{}	{}	0	0	2026-09-17 05:40:59.351426	2026-09-17 05:40:59.351426	\N
63	mcpopsbudgetread000000000000000a	platform-budget	streamable-http	预算只读（真实数据）：budget_my 我的命中预算（余量/是否不足）/budget_request_list 我的临时预算申请与待我审批，代码 api/interfaces/ops	http://127.0.0.1:43220/mcp		[]	{}	{}	{}	{}	{}	{}	[]	1		public	0	60	{}	{}	{}	0	0	2026-09-17 05:40:59.366515	2026-09-17 05:40:59.366515	\N
64	mcpopsbudgetwrite00000000000000a	platform-budget-write	streamable-http	临时预算申请写入（真实数据）：budget_request_create 申请临时额度（通知部门负责人审批）/budget_request_approve 通过/budget_request_reject 驳回，代码 api/interfaces/ops	http://127.0.0.1:43220/mcp/budget-write		[]	{}	{}	{}	{}	{}	{}	[]	1		public	0	60	{}	{}	{}	0	0	2026-09-17 05:40:59.366515	2026-09-17 05:40:59.366515	\N
65	mcpopsschedread0000000000000000a	platform-schedule	streamable-http	日程只读（真实数据）：schedule_list 我的日程/schedule_find_slots 共同空闲/schedule_rooms 可用会议室，代码 api/interfaces/ops	http://127.0.0.1:43220/mcp/schedule		[]	{}	{}	{}	{}	{}	{}	[]	1		public	0	60	{}	{}	{}	0	0	2026-09-17 05:40:59.41094	2026-09-17 05:40:59.41094	\N
66	mcpopsschedwrite000000000000000a	platform-schedule-write	streamable-http	日程写入（真实数据）：schedule_create 创建日程会议（含周期）/schedule_update 修改/schedule_cancel 取消（整series或单场），代码 api/interfaces/ops	http://127.0.0.1:43220/mcp/schedule-write		[]	{}	{}	{}	{}	{}	{}	[]	1		public	0	60	{}	{}	{}	0	0	2026-09-17 05:40:59.41094	2026-09-17 05:40:59.41094	\N
\.


--
-- Data for Name: aa_mcp_tool_embedding; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_mcp_tool_embedding (id, server_name, tool_name, description, embedding, updated_at) FROM stdin;
\.

-- Data for Name: aa_mcp_user_config; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_mcp_user_config (id, user_id, routing, additional_config, created_at, updated_at) FROM stdin;
\.


--
-- Name: aa_mcp_activity_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_mcp_activity_id_seq', 1, false);


--
-- Name: aa_mcp_bearer_key_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_mcp_bearer_key_id_seq', 1, false);


--
-- Name: aa_mcp_builtin_prompt_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_mcp_builtin_prompt_id_seq', 1, false);


--
-- Name: aa_mcp_builtin_resource_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_mcp_builtin_resource_id_seq', 1, false);


--
-- Name: aa_mcp_group_grant_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_mcp_group_grant_id_seq', 1, false);


--
-- Name: aa_mcp_group_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_mcp_group_id_seq', 1, false);


--
--
-- Name: aa_mcp_oauth_client_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_mcp_oauth_client_id_seq', 1, false);

--
-- Name: aa_mcp_oauth_code_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_mcp_oauth_code_id_seq', 1, false);

--
-- Name: aa_mcp_oauth_token_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_mcp_oauth_token_id_seq', 1, false);

-- Name: aa_mcp_server_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_mcp_server_id_seq', 66, true);


--
--
-- Name: aa_mcp_tool_embedding_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_mcp_tool_embedding_id_seq', 1, false);

-- Name: aa_mcp_user_config_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_mcp_user_config_id_seq', 1, false);


--
-- Name: aa_mcp_activity aa_mcp_activity_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_mcp_activity
    ADD CONSTRAINT aa_mcp_activity_pkey PRIMARY KEY (id);


--
-- Name: aa_mcp_bearer_key aa_mcp_bearer_key_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_mcp_bearer_key
    ADD CONSTRAINT aa_mcp_bearer_key_pkey PRIMARY KEY (id);


--
-- Name: aa_mcp_builtin_prompt aa_mcp_builtin_prompt_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_mcp_builtin_prompt
    ADD CONSTRAINT aa_mcp_builtin_prompt_pkey PRIMARY KEY (id);


--
-- Name: aa_mcp_builtin_resource aa_mcp_builtin_resource_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_mcp_builtin_resource
    ADD CONSTRAINT aa_mcp_builtin_resource_pkey PRIMARY KEY (id);


--
-- Name: aa_mcp_config aa_mcp_config_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_mcp_config
    ADD CONSTRAINT aa_mcp_config_pkey PRIMARY KEY (id);


--
-- Name: aa_mcp_group_grant aa_mcp_group_grant_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_mcp_group_grant
    ADD CONSTRAINT aa_mcp_group_grant_pkey PRIMARY KEY (id);


--
-- Name: aa_mcp_group aa_mcp_group_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_mcp_group
    ADD CONSTRAINT aa_mcp_group_pkey PRIMARY KEY (id);


--
--
-- Name: aa_mcp_oauth_client aa_mcp_oauth_client_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_mcp_oauth_client
    ADD CONSTRAINT aa_mcp_oauth_client_pkey PRIMARY KEY (id);

--
-- Name: aa_mcp_oauth_code aa_mcp_oauth_code_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_mcp_oauth_code
    ADD CONSTRAINT aa_mcp_oauth_code_pkey PRIMARY KEY (id);

--
-- Name: aa_mcp_oauth_token aa_mcp_oauth_token_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_mcp_oauth_token
    ADD CONSTRAINT aa_mcp_oauth_token_pkey PRIMARY KEY (id);

-- Name: aa_mcp_server aa_mcp_server_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_mcp_server
    ADD CONSTRAINT aa_mcp_server_pkey PRIMARY KEY (id);


--
--
-- Name: aa_mcp_tool_embedding aa_mcp_tool_embedding_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_mcp_tool_embedding
    ADD CONSTRAINT aa_mcp_tool_embedding_pkey PRIMARY KEY (id);

-- Name: aa_mcp_user_config aa_mcp_user_config_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_mcp_user_config
    ADD CONSTRAINT aa_mcp_user_config_pkey PRIMARY KEY (id);


--
-- Name: idx_aa_mcp_activity_group; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_mcp_activity_group ON public.aa_mcp_activity USING btree (group_name);


--
-- Name: idx_aa_mcp_activity_key; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_mcp_activity_key ON public.aa_mcp_activity USING btree (key_id);


--
-- Name: idx_aa_mcp_activity_server; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_mcp_activity_server ON public.aa_mcp_activity USING btree (server_name);


--
-- Name: idx_aa_mcp_activity_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_mcp_activity_status ON public.aa_mcp_activity USING btree (status);


--
-- Name: idx_aa_mcp_activity_tool; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_mcp_activity_tool ON public.aa_mcp_activity USING btree (tool);


--
-- Name: idx_aa_mcp_activity_ts; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_mcp_activity_ts ON public.aa_mcp_activity USING btree (ts);


--
-- Name: idx_aa_mcp_activity_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_mcp_activity_user ON public.aa_mcp_activity USING btree (user_id);


--
-- Name: idx_aa_mcp_bearer_key_enabled; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_mcp_bearer_key_enabled ON public.aa_mcp_bearer_key USING btree (enabled);


--
-- Name: idx_aa_mcp_bearer_key_hash; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_mcp_bearer_key_hash ON public.aa_mcp_bearer_key USING btree (token_hash);


--
-- Name: idx_aa_mcp_bearer_key_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_mcp_bearer_key_id ON public.aa_mcp_bearer_key USING btree (key_id);


--
-- Name: idx_aa_mcp_bearer_key_owner; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_mcp_bearer_key_owner ON public.aa_mcp_bearer_key USING btree (owner);


--
-- Name: idx_aa_mcp_builtin_prompt_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_mcp_builtin_prompt_id ON public.aa_mcp_builtin_prompt USING btree (prompt_id);


--
-- Name: idx_aa_mcp_builtin_prompt_name; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_mcp_builtin_prompt_name ON public.aa_mcp_builtin_prompt USING btree (name);


--
-- Name: idx_aa_mcp_builtin_resource_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_mcp_builtin_resource_id ON public.aa_mcp_builtin_resource USING btree (resource_id);


--
-- Name: idx_aa_mcp_builtin_resource_uri; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_mcp_builtin_resource_uri ON public.aa_mcp_builtin_resource USING btree (uri);


--
-- Name: idx_aa_mcp_group_grant_target; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_mcp_group_grant_target ON public.aa_mcp_group_grant USING btree (target_id);


--
-- Name: idx_aa_mcp_group_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_mcp_group_id ON public.aa_mcp_group USING btree (group_id);


--
-- Name: idx_aa_mcp_group_name; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_mcp_group_name ON public.aa_mcp_group USING btree (name) WHERE (deleted_at IS NULL);


--
-- Name: idx_aa_mcp_group_owner; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_mcp_group_owner ON public.aa_mcp_group USING btree (owner) WHERE (deleted_at IS NULL);


--
--
-- Name: idx_aa_mcp_oauth_client_biz; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_mcp_oauth_client_biz ON public.aa_mcp_oauth_client USING btree (biz_id);

--
-- Name: idx_aa_mcp_oauth_client_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_mcp_oauth_client_id ON public.aa_mcp_oauth_client USING btree (client_id);

--
-- Name: idx_aa_mcp_oauth_code_biz; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_mcp_oauth_code_biz ON public.aa_mcp_oauth_code USING btree (biz_id);

--
-- Name: idx_aa_mcp_oauth_code_client; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_mcp_oauth_code_client ON public.aa_mcp_oauth_code USING btree (client_id);

--
-- Name: idx_aa_mcp_oauth_code_hash; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_mcp_oauth_code_hash ON public.aa_mcp_oauth_code USING btree (code_hash);

--
-- Name: idx_aa_mcp_oauth_token_access; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_mcp_oauth_token_access ON public.aa_mcp_oauth_token USING btree (access_token_hash);

--
-- Name: idx_aa_mcp_oauth_token_biz; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_mcp_oauth_token_biz ON public.aa_mcp_oauth_token USING btree (biz_id);

--
-- Name: idx_aa_mcp_oauth_token_key; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_mcp_oauth_token_key ON public.aa_mcp_oauth_token USING btree (bearer_key_id);

--
-- Name: idx_aa_mcp_oauth_token_refresh; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_mcp_oauth_token_refresh ON public.aa_mcp_oauth_token USING btree (refresh_token_hash);

-- Name: idx_aa_mcp_server_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_mcp_server_id ON public.aa_mcp_server USING btree (server_id);


--
-- Name: idx_aa_mcp_server_name; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_mcp_server_name ON public.aa_mcp_server USING btree (name) WHERE (deleted_at IS NULL);


--
-- Name: idx_aa_mcp_server_owner; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_mcp_server_owner ON public.aa_mcp_server USING btree (owner) WHERE (deleted_at IS NULL);


--
-- Name: idx_aa_mcp_server_type; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_mcp_server_type ON public.aa_mcp_server USING btree (type) WHERE (deleted_at IS NULL);


--
--
-- Name: idx_aa_mcp_tool_embedding_uniq; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_mcp_tool_embedding_uniq ON public.aa_mcp_tool_embedding USING btree (server_name, tool_name);

--
-- Name: idx_aa_mcp_tool_embedding_vec; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_mcp_tool_embedding_vec ON public.aa_mcp_tool_embedding USING ivfflat (embedding public.vector_cosine_ops) WITH (lists='100');

-- Name: idx_aa_mcp_user_config_user; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_mcp_user_config_user ON public.aa_mcp_user_config USING btree (user_id);


--
-- PostgreSQL database dump complete
--

\unrestrict 17DM3Bz01EQ1TUFDnusKNsKSvMRtCGzkINfkf28WxxqUI0USxeseAQwtlLwJYb2