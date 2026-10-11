-- ============================================================================
-- client.sql — Desktop 模块数据库与表结构（Win/Mac 桌面客户端）
-- ============================================================================
-- 依赖：PostgreSQL 16（与 aiarsenal / aa_skill / aa_llm / aa_mcp / aa_agent 同实例异库）
-- 关联：user_id 软引用 aiarsenal 库的 aa_user.user_id（跨库无实际 FK，
--       遵循 aiarsenal 既有模式：以 varchar(32) biz_id 软引用）
-- 约定：每表 id bigserial PK（内部）+ <实体>_id varchar(32) UNIQUE（对外业务 id）
--       时间用 timestamp；状态用 smallint；复杂结构用 jsonb
-- 范围：可安装 agent（类型+介绍）→ 多个 version（各自配置）；运行情况上报（每用户）
-- ============================================================================

-- ----------------------------------------------------------------------------
-- Part A — 独立数据库 aa_client
-- ----------------------------------------------------------------------------

SELECT 'CREATE DATABASE aa_client'
WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = 'aa_client')\gexec

\connect aa_client

-- ============================================================================
-- 基线固化说明（2026-09 squash）
-- 正文已替换为 pg_dump 固化版：合并了历史上全部增量迁移（原 migrations/ 已删，git 可查）。
-- 原文件尾部「
--
-- PostgreSQL database dump
--

\restrict jFeHXhMOihIiWuG0NtCwX8eAGHlroqP27Edrz94276FBeVJc2adza5GopgzUvqe

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

DROP INDEX IF EXISTS public.idx_aa_client_package_id;
DROP INDEX IF EXISTS public.idx_aa_client_package_cvp;
DROP INDEX IF EXISTS public.idx_aa_client_log_id;
DROP INDEX IF EXISTS public.idx_aa_client_log_device;
DROP INDEX IF EXISTS public.idx_aa_client_device_user;
DROP INDEX IF EXISTS public.idx_aa_client_device_mcp_pair;
DROP INDEX IF EXISTS public.idx_aa_client_device_mcp_id;
DROP INDEX IF EXISTS public.idx_aa_client_device_mcp_device;
DROP INDEX IF EXISTS public.idx_aa_client_device_id;
DROP INDEX IF EXISTS public.idx_aa_client_config_change_id;
DROP INDEX IF EXISTS public.idx_aa_client_config_change_device;
DROP INDEX IF EXISTS public.idx_aa_client_command_id;
DROP INDEX IF EXISTS public.idx_aa_client_command_device;
DROP INDEX IF EXISTS public.idx_aa_client_app_perm_id;
DROP INDEX IF EXISTS public.idx_aa_client_app_perm_dev;
DROP INDEX IF EXISTS public.idx_aa_client_app_id;
DROP INDEX IF EXISTS public.idx_aa_client_app_bundle;
DROP INDEX IF EXISTS public.idx_aa_client_agent_version_id;
DROP INDEX IF EXISTS public.idx_aa_client_agent_version_agent_ver;
DROP INDEX IF EXISTS public.idx_aa_client_agent_status_user;
DROP INDEX IF EXISTS public.idx_aa_client_agent_status_id;
DROP INDEX IF EXISTS public.idx_aa_client_agent_status_device_agent;
DROP INDEX IF EXISTS public.idx_aa_client_agent_key;
DROP INDEX IF EXISTS public.idx_aa_client_agent_id;
ALTER TABLE IF EXISTS ONLY public.aa_client_package DROP CONSTRAINT IF EXISTS aa_client_package_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_client_log DROP CONSTRAINT IF EXISTS aa_client_log_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_client_device DROP CONSTRAINT IF EXISTS aa_client_device_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_client_device_mcp_group DROP CONSTRAINT IF EXISTS aa_client_device_mcp_group_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_client_config_change DROP CONSTRAINT IF EXISTS aa_client_config_change_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_client_command DROP CONSTRAINT IF EXISTS aa_client_command_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_client_app DROP CONSTRAINT IF EXISTS aa_client_app_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_client_app_perm DROP CONSTRAINT IF EXISTS aa_client_app_perm_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_client_agent_version DROP CONSTRAINT IF EXISTS aa_client_agent_version_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_client_agent_status DROP CONSTRAINT IF EXISTS aa_client_agent_status_pkey;
ALTER TABLE IF EXISTS ONLY public.aa_client_agent DROP CONSTRAINT IF EXISTS aa_client_agent_pkey;
ALTER TABLE IF EXISTS public.aa_client_package ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_client_log ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_client_device_mcp_group ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_client_device ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_client_config_change ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_client_command ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_client_app_perm ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_client_app ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_client_agent_version ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_client_agent_status ALTER COLUMN id DROP DEFAULT;
ALTER TABLE IF EXISTS public.aa_client_agent ALTER COLUMN id DROP DEFAULT;
DROP SEQUENCE IF EXISTS public.aa_client_package_id_seq;
DROP TABLE IF EXISTS public.aa_client_package;
DROP SEQUENCE IF EXISTS public.aa_client_log_id_seq;
DROP TABLE IF EXISTS public.aa_client_log;
DROP SEQUENCE IF EXISTS public.aa_client_device_mcp_group_id_seq;
DROP TABLE IF EXISTS public.aa_client_device_mcp_group;
DROP SEQUENCE IF EXISTS public.aa_client_device_id_seq;
DROP TABLE IF EXISTS public.aa_client_device;
DROP SEQUENCE IF EXISTS public.aa_client_config_change_id_seq;
DROP TABLE IF EXISTS public.aa_client_config_change;
DROP SEQUENCE IF EXISTS public.aa_client_command_id_seq;
DROP TABLE IF EXISTS public.aa_client_command;
DROP SEQUENCE IF EXISTS public.aa_client_app_perm_id_seq;
DROP TABLE IF EXISTS public.aa_client_app_perm;
DROP SEQUENCE IF EXISTS public.aa_client_app_id_seq;
DROP TABLE IF EXISTS public.aa_client_app;
DROP SEQUENCE IF EXISTS public.aa_client_agent_version_id_seq;
DROP TABLE IF EXISTS public.aa_client_agent_version;
DROP SEQUENCE IF EXISTS public.aa_client_agent_status_id_seq;
DROP TABLE IF EXISTS public.aa_client_agent_status;
DROP SEQUENCE IF EXISTS public.aa_client_agent_id_seq;
DROP TABLE IF EXISTS public.aa_client_agent;
SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: aa_client_agent; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_client_agent (
    id bigint NOT NULL,
    agent_id character varying(32) DEFAULT ''::character varying NOT NULL,
    agent character varying(32) DEFAULT ''::character varying NOT NULL,
    name character varying(100) DEFAULT ''::character varying NOT NULL,
    intro text DEFAULT ''::text NOT NULL,
    icon character varying(64) DEFAULT ''::character varying NOT NULL,
    logo character varying(512) DEFAULT ''::character varying NOT NULL,
    enabled smallint DEFAULT 1 NOT NULL,
    weigh integer DEFAULT 0 NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone
);


--
-- Name: aa_client_agent_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_client_agent_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_client_agent_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_client_agent_id_seq OWNED BY public.aa_client_agent.id;


--
-- Name: aa_client_agent_status; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_client_agent_status (
    id bigint NOT NULL,
    status_id character varying(32) DEFAULT ''::character varying NOT NULL,
    device_id character varying(32) DEFAULT ''::character varying NOT NULL,
    user_id character varying(32) DEFAULT ''::character varying NOT NULL,
    agent character varying(32) DEFAULT ''::character varying NOT NULL,
    kind character varying(16) DEFAULT 'agent'::character varying NOT NULL,
    status character varying(32) DEFAULT ''::character varying NOT NULL,
    version character varying(64) DEFAULT NULL::character varying,
    last_error text,
    extra jsonb,
    reported_at timestamp without time zone DEFAULT now() NOT NULL,
    created_at timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: aa_client_agent_status_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_client_agent_status_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_client_agent_status_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_client_agent_status_id_seq OWNED BY public.aa_client_agent_status.id;


--
-- Name: aa_client_agent_version; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_client_agent_version (
    id bigint NOT NULL,
    version_id character varying(32) DEFAULT ''::character varying NOT NULL,
    agent_id character varying(32) DEFAULT ''::character varying NOT NULL,
    version character varying(64) DEFAULT ''::character varying NOT NULL,
    is_default smallint DEFAULT 0 NOT NULL,
    enabled smallint DEFAULT 1 NOT NULL,
    download_url character varying(512) DEFAULT ''::character varying NOT NULL,
    container_image character varying(256) DEFAULT ''::character varying NOT NULL,
    channel character varying(64) DEFAULT ''::character varying NOT NULL,
    config_template jsonb DEFAULT '{}'::jsonb NOT NULL,
    env_vars jsonb DEFAULT '{}'::jsonb NOT NULL,
    extra jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone
);


--
-- Name: aa_client_agent_version_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_client_agent_version_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_client_agent_version_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_client_agent_version_id_seq OWNED BY public.aa_client_agent_version.id;


--
-- Name: aa_client_app; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_client_app (
    id bigint NOT NULL,
    app_id character varying(32) DEFAULT ''::character varying NOT NULL,
    bundle_id character varying(256) DEFAULT ''::character varying NOT NULL,
    app_name character varying(100) DEFAULT ''::character varying NOT NULL,
    category character varying(32) DEFAULT ''::character varying NOT NULL,
    logo character varying(32) DEFAULT ''::character varying NOT NULL,
    weigh integer DEFAULT 0 NOT NULL,
    remarks character varying(255) DEFAULT ''::character varying NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone
);


--
-- Name: aa_client_app_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_client_app_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_client_app_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_client_app_id_seq OWNED BY public.aa_client_app.id;


--
-- Name: aa_client_app_perm; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_client_app_perm (
    id bigint NOT NULL,
    perm_id character varying(32) DEFAULT ''::character varying NOT NULL,
    device_id character varying(64) DEFAULT ''::character varying NOT NULL,
    bundle_id character varying(256) DEFAULT ''::character varying NOT NULL,
    granted boolean DEFAULT false NOT NULL,
    denied boolean DEFAULT false NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone
);


--
-- Name: aa_client_app_perm_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_client_app_perm_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_client_app_perm_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_client_app_perm_id_seq OWNED BY public.aa_client_app_perm.id;


--
-- Name: aa_client_command; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_client_command (
    id bigint NOT NULL,
    command_id character varying(32) DEFAULT ''::character varying NOT NULL,
    device_id character varying(32) DEFAULT ''::character varying NOT NULL,
    action character varying(32) DEFAULT ''::character varying NOT NULL,
    agent character varying(32) DEFAULT ''::character varying NOT NULL,
    version character varying(64) DEFAULT ''::character varying NOT NULL,
    payload jsonb DEFAULT '{}'::jsonb NOT NULL,
    status character varying(16) DEFAULT 'pending'::character varying NOT NULL,
    result jsonb DEFAULT '{}'::jsonb NOT NULL,
    progress jsonb DEFAULT '{}'::jsonb NOT NULL,
    error text,
    chain_id character varying(32) DEFAULT ''::character varying NOT NULL,
    created_by character varying(32) DEFAULT ''::character varying NOT NULL,
    created_at timestamp without time zone,
    started_at timestamp without time zone,
    finished_at timestamp without time zone,
    timeout_seconds integer DEFAULT 0,
    last_progress_at timestamp without time zone
);


--
-- Name: aa_client_command_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_client_command_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_client_command_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_client_command_id_seq OWNED BY public.aa_client_command.id;


--
-- Name: aa_client_config_change; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_client_config_change (
    id bigint NOT NULL,
    change_id character varying(32) DEFAULT ''::character varying NOT NULL,
    device_id character varying(32) DEFAULT ''::character varying NOT NULL,
    agent character varying(32) DEFAULT ''::character varying NOT NULL,
    config_hash character varying(64) DEFAULT ''::character varying NOT NULL,
    config_size bigint DEFAULT 0 NOT NULL,
    config_mtime timestamp without time zone,
    content text DEFAULT ''::text NOT NULL,
    source character varying(32) DEFAULT ''::character varying NOT NULL,
    source_detail text DEFAULT ''::text NOT NULL,
    changed_at timestamp without time zone
);


--
-- Name: aa_client_config_change_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_client_config_change_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_client_config_change_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_client_config_change_id_seq OWNED BY public.aa_client_config_change.id;


--
-- Name: aa_client_device; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_client_device (
    id bigint NOT NULL,
    device_id character varying(32) DEFAULT ''::character varying NOT NULL,
    user_id character varying(32) DEFAULT ''::character varying NOT NULL,
    user_name character varying(100) DEFAULT ''::character varying NOT NULL,
    name character varying(128) DEFAULT ''::character varying NOT NULL,
    platform character varying(16) DEFAULT ''::character varying NOT NULL,
    os character varying(64) DEFAULT ''::character varying NOT NULL,
    arch character varying(32) DEFAULT ''::character varying NOT NULL,
    app_version character varying(32) DEFAULT ''::character varying NOT NULL,
    mac_address character varying(64) DEFAULT ''::character varying NOT NULL,
    mac_extra jsonb DEFAULT '[]'::jsonb NOT NULL,
    ip character varying(64) DEFAULT ''::character varying NOT NULL,
    status character varying(16) DEFAULT 'active'::character varying NOT NULL,
    blocked smallint DEFAULT 0 NOT NULL,
    last_login_at timestamp without time zone,
    last_login_ip character varying(64) DEFAULT NULL::character varying,
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    extra jsonb DEFAULT '{}'::jsonb NOT NULL,
    channels jsonb DEFAULT '[]'::jsonb NOT NULL,
    skills jsonb DEFAULT '[]'::jsonb NOT NULL,
    mcp_key_plain character varying(128) DEFAULT ''::character varying NOT NULL
);


--
-- Name: aa_client_device_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_client_device_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_client_device_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_client_device_id_seq OWNED BY public.aa_client_device.id;


--
-- Name: aa_client_device_mcp_group; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_client_device_mcp_group (
    id bigint NOT NULL,
    device_mcp_id character varying(32) DEFAULT ''::character varying NOT NULL,
    device_id character varying(32) DEFAULT ''::character varying NOT NULL,
    group_id character varying(32) DEFAULT ''::character varying NOT NULL,
    group_name character varying(200) DEFAULT ''::character varying NOT NULL,
    created_by character varying(32) DEFAULT ''::character varying NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone
);


--
-- Name: aa_client_device_mcp_group_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_client_device_mcp_group_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_client_device_mcp_group_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_client_device_mcp_group_id_seq OWNED BY public.aa_client_device_mcp_group.id;


--
-- Name: aa_client_log; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_client_log (
    id bigint NOT NULL,
    log_id character varying(32) DEFAULT ''::character varying NOT NULL,
    device_id character varying(32) DEFAULT ''::character varying NOT NULL,
    agent character varying(32) DEFAULT ''::character varying NOT NULL,
    command_id character varying(32) DEFAULT ''::character varying NOT NULL,
    level character varying(16) DEFAULT 'info'::character varying NOT NULL,
    line text DEFAULT ''::text NOT NULL,
    ts timestamp without time zone
);


--
-- Name: aa_client_log_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_client_log_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_client_log_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_client_log_id_seq OWNED BY public.aa_client_log.id;


--
-- Name: aa_client_package; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.aa_client_package (
    id bigint NOT NULL,
    package_id character varying(32) DEFAULT ''::character varying NOT NULL,
    agent character varying(32) DEFAULT ''::character varying NOT NULL,
    version character varying(64) DEFAULT ''::character varying NOT NULL,
    platform character varying(16) DEFAULT ''::character varying NOT NULL,
    filename character varying(256) DEFAULT ''::character varying NOT NULL,
    file_size bigint DEFAULT 0 NOT NULL,
    sha256 character varying(64) DEFAULT ''::character varying NOT NULL,
    official_url character varying(512) DEFAULT ''::character varying NOT NULL,
    local_path character varying(512) DEFAULT ''::character varying NOT NULL,
    status character varying(16) DEFAULT 'ready'::character varying NOT NULL,
    fetched_by character varying(32) DEFAULT ''::character varying NOT NULL,
    created_at timestamp without time zone,
    updated_at timestamp without time zone,
    fetched_at timestamp without time zone
);


--
-- Name: aa_client_package_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.aa_client_package_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: aa_client_package_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.aa_client_package_id_seq OWNED BY public.aa_client_package.id;


--
-- Name: aa_client_agent id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_client_agent ALTER COLUMN id SET DEFAULT nextval('public.aa_client_agent_id_seq'::regclass);


--
-- Name: aa_client_agent_status id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_client_agent_status ALTER COLUMN id SET DEFAULT nextval('public.aa_client_agent_status_id_seq'::regclass);


--
-- Name: aa_client_agent_version id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_client_agent_version ALTER COLUMN id SET DEFAULT nextval('public.aa_client_agent_version_id_seq'::regclass);


--
-- Name: aa_client_app id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_client_app ALTER COLUMN id SET DEFAULT nextval('public.aa_client_app_id_seq'::regclass);


--
-- Name: aa_client_app_perm id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_client_app_perm ALTER COLUMN id SET DEFAULT nextval('public.aa_client_app_perm_id_seq'::regclass);


--
-- Name: aa_client_command id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_client_command ALTER COLUMN id SET DEFAULT nextval('public.aa_client_command_id_seq'::regclass);


--
-- Name: aa_client_config_change id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_client_config_change ALTER COLUMN id SET DEFAULT nextval('public.aa_client_config_change_id_seq'::regclass);


--
-- Name: aa_client_device id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_client_device ALTER COLUMN id SET DEFAULT nextval('public.aa_client_device_id_seq'::regclass);


--
-- Name: aa_client_device_mcp_group id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_client_device_mcp_group ALTER COLUMN id SET DEFAULT nextval('public.aa_client_device_mcp_group_id_seq'::regclass);


--
-- Name: aa_client_log id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_client_log ALTER COLUMN id SET DEFAULT nextval('public.aa_client_log_id_seq'::regclass);


--
-- Name: aa_client_package id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_client_package ALTER COLUMN id SET DEFAULT nextval('public.aa_client_package_id_seq'::regclass);


--
-- Data for Name: aa_client_agent; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_client_agent (id, agent_id, agent, name, intro, icon, logo, enabled, weigh, created_at, updated_at) FROM stdin;
1	clientagentopenclaw00000000000a	openclaw	OpenClaw	Node CLI 客户端，openclaw gateway run 启动；需本机已装 Node。	🟢	/client-logos/openclaw.svg	1	30	2026-07-05 00:00:00	2026-07-05 00:00:00
2	clientagenthermes0000000000000a	hermes	Hermes	原生 Python agent（公网源码包安装），hermes gateway run 启动；App 内安装/启停/微信扫码绑定。	🐳	/client-logos/hermes.png	1	20	2026-07-05 00:00:00	2026-07-05 00:00:00
3	clientagentobsidian00000000000a	obsidian	Obsidian	原生桌面笔记应用，官方 .exe/.dmg 安装。	📝	/client-logos/obsidian.svg	1	10	2026-07-05 00:00:00	2026-07-05 00:00:00
\.


--
-- Data for Name: aa_client_agent_status; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_client_agent_status (id, status_id, device_id, user_id, agent, kind, status, version, last_error, extra, reported_at, created_at) FROM stdin;
\.


--
-- Data for Name: aa_client_agent_version; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_client_agent_version (id, version_id, agent_id, version, is_default, enabled, download_url, container_image, channel, config_template, env_vars, extra, created_at, updated_at) FROM stdin;
1	clientagentopenclawver00000000a	clientagentopenclaw00000000000a	2026.7.1	1	1				{"apiKey": "", "baseUrl": "https://api.aiarsenal.cc", "channels": []}	{}	{}	2026-07-05 00:00:00	2026-07-05 00:00:00
2	clientagenthermesver0000000000a	clientagenthermes0000000000000a	2026.9.14	1	1				{"skills": [], "enabled": true}	{}	{}	2026-07-05 00:00:00	2026-07-05 00:00:00
3	clientagentobsidianver00000000a	clientagentobsidian00000000000a	1.13.4	1	1	https://obsidian.md/download			{"vaultPath": ""}	{}	{"downloads": {"mac": "https://github.com/obsidianmd/obsidian-releases/releases/download/v1.13.4/Obsidian-1.13.4.dmg", "win": "https://github.com/obsidianmd/obsidian-releases/releases/download/v1.13.4/Obsidian-1.13.4.exe"}}	2026-07-05 00:00:00	2026-07-05 00:00:00
\.


--
-- Data for Name: aa_client_app; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_client_app (id, app_id, bundle_id, app_name, category, logo, weigh, remarks, created_at, updated_at) FROM stdin;
5	clientappmessages00000000000000a	com.apple.MobileSMS	信息	comm	💬	994	种子预置	2026-08-18 00:00:00	2026-08-18 00:00:00
6	clientappmail000000000000000000a	com.apple.mail	邮件	comm	📧	993	种子预置	2026-08-18 00:00:00	2026-08-18 00:00:00
15	clientappfacetime00000000000000a	com.apple.FaceTime	FaceTime	comm	📞	974	种子预置	2026-08-18 00:00:00	2026-08-18 00:00:00
18	clientappwechat0000000000000000a	com.tencent.xinWeChat	微信	comm	💚	951	种子预置	2026-08-18 00:00:00	2026-08-18 00:00:00
19	clientappqq00000000000000000000a	com.tencent.qq	QQ	comm	🐧	950	种子预置	2026-08-18 00:00:00	2026-08-18 00:00:00
20	clientappwework0000000000000000a	com.tencent.WeWorkMac	企业微信	comm	💼	949	种子预置	2026-08-18 00:00:00	2026-08-18 00:00:00
21	clientapplark000000000000000000a	com.ss.lark	飞书	comm	🕊️	948	种子预置	2026-08-18 00:00:00	2026-08-18 00:00:00
22	clientappdingtalk00000000000000a	com.alibaba.DingTalk	钉钉	comm	📌	947	种子预置	2026-08-18 00:00:00	2026-08-18 00:00:00
23	clientappmeeting000000000000000a	com.tencent.meeting	腾讯会议	comm	📹	946	种子预置	2026-08-18 00:00:00	2026-08-18 00:00:00
4	clientappnotes00000000000000000a	com.apple.Notes	备忘录	docs	📝	996	种子预置	2026-08-18 00:00:00	2026-08-18 00:00:00
8	clientapppreview000000000000000a	com.apple.Preview	预览	docs	👁️	987	种子预置	2026-08-18 00:00:00	2026-08-18 00:00:00
9	clientapptextedit00000000000000a	com.apple.TextEdit	文本编辑	docs	📄	986	种子预置	2026-08-18 00:00:00	2026-08-18 00:00:00
12	clientappfreeform00000000000000a	com.apple.freeform	无边记	docs	🎨	982	种子预置	2026-08-18 00:00:00	2026-08-18 00:00:00
14	clientappstickies00000000000000a	com.apple.Stickies	便签	docs	🗒️	977	种子预置	2026-08-18 00:00:00	2026-08-18 00:00:00
25	clientappwps0000000000000000000a	com.kingsoft.wpsoffice.mac	WPS Office	docs	📑	940	种子预置	2026-08-18 00:00:00	2026-08-18 00:00:00
7	clientappquicktime0000000000000a	com.apple.QuickTimePlayerX	QuickTime	tools	🎬	988	种子预置	2026-08-18 00:00:00	2026-08-18 00:00:00
10	clientappcalculator000000000000a	com.apple.calculator	计算器	tools	🧮	985	种子预置	2026-08-18 00:00:00	2026-08-18 00:00:00
11	clientappsyssettings00000000000a	com.apple.systempreferences	系统设置	tools	⚙️	984	种子预置	2026-08-18 00:00:00	2026-08-18 00:00:00
13	clientappappstore00000000000000a	com.apple.AppStore	App Store	tools	🛍️	981	种子预置	2026-08-18 00:00:00	2026-08-18 00:00:00
31	clientappbaidunetdisk0000000000a	com.baidu.BaiduNetdisk-mac	百度网盘	tools	☁️	926	种子预置	2026-08-18 00:00:00	2026-08-18 00:00:00
32	clientappthunder000000000000000a	com.xunlei.thunder	迅雷	tools	⚡	925	种子预置	2026-08-18 00:00:00	2026-08-18 00:00:00
33	clientappbob0000000000000000000a	com.hegenberg.Bob	Bob	tools	🔤	921	种子预置	2026-08-18 00:00:00	2026-08-18 00:00:00
34	clientappsnipaste00000000000000a	com.Snipaste	Snipaste	tools	📸	916	种子预置	2026-08-18 00:00:00	2026-08-18 00:00:00
35	clientapputools0000000000000000a	cn.utapps.uTools	uTools	tools	🧰	915	种子预置	2026-08-18 00:00:00	2026-08-18 00:00:00
3	clientappsafari0000000000000000a	com.apple.Safari	Safari	browser	🧭	997	种子预置	2026-08-18 00:00:00	2026-08-18 00:00:00
16	clientappchrome0000000000000000a	com.google.Chrome	Chrome	browser	🌐	973	种子预置	2026-08-18 00:00:00	2026-08-18 00:00:00
36	clientappchatgpt000000000000000a	com.openai.chat	ChatGPT	ai	🪄	914	种子预置	2026-08-18 00:00:00	2026-08-18 00:00:00
37	clientappclaude0000000000000000a	com.anthropic.claudefordesktop	Claude	ai	🟠	913	种子预置	2026-08-18 00:00:00	2026-08-18 00:00:00
1	clientappfinder0000000000000000a	com.apple.finder	访达	system	🗂️	999	种子预置	2026-08-18 00:00:00	2026-08-18 00:00:00
2	clientappterminal00000000000000a	com.apple.Terminal	终端	system	📟	998	种子预置	2026-08-18 00:00:00	2026-08-18 00:00:00
122	clientappteamviewer000000000000a	com.teamviewer.TeamViewer	TeamViewer	comm	🔗	941	种子预置	2026-08-18 00:00:00	2026-08-18 00:00:00
126	clientappoutlook000000000000000a	com.microsoft.Outlook	Outlook	comm	📨	936	种子预置	2026-08-18 00:00:00	2026-08-18 00:00:00
123	clientappword000000000000000000a	com.microsoft.Word	Word	docs	📘	939	种子预置	2026-08-18 00:00:00	2026-08-18 00:00:00
124	clientappexcel00000000000000000a	com.microsoft.Excel	Excel	docs	📈	938	种子预置	2026-08-18 00:00:00	2026-08-18 00:00:00
125	clientappppt0000000000000000000a	com.microsoft.Powerpoint	PowerPoint	docs	📽️	937	种子预置	2026-08-18 00:00:00	2026-08-18 00:00:00
127	clientappevernote00000000000000a	com.evernote.Evernote	印象笔记	docs	🐘	934	种子预置	2026-08-18 00:00:00	2026-08-18 00:00:00
121	clientappedge000000000000000000a	com.microsoft.edgemac	Edge	browser	🌊	972	种子预置	2026-08-18 00:00:00	2026-08-18 00:00:00
\.


--
-- Data for Name: aa_client_app_perm; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_client_app_perm (id, perm_id, device_id, bundle_id, granted, denied, created_at, updated_at) FROM stdin;
\.


--
-- Data for Name: aa_client_command; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_client_command (id, command_id, device_id, action, agent, version, payload, status, result, progress, error, chain_id, created_by, created_at, started_at, finished_at, timeout_seconds, last_progress_at) FROM stdin;
\.


--
-- Data for Name: aa_client_config_change; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_client_config_change (id, change_id, device_id, agent, config_hash, config_size, config_mtime, content, source, source_detail, changed_at) FROM stdin;
\.


--
-- Data for Name: aa_client_device; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_client_device (id, device_id, user_id, user_name, name, platform, os, arch, app_version, mac_address, mac_extra, ip, status, blocked, last_login_at, last_login_ip, created_at, updated_at, extra, channels, skills, mcp_key_plain) FROM stdin;
\.


--
-- Data for Name: aa_client_device_mcp_group; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_client_device_mcp_group (id, device_mcp_id, device_id, group_id, group_name, created_by, created_at, updated_at) FROM stdin;
\.


--
-- Data for Name: aa_client_log; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_client_log (id, log_id, device_id, agent, command_id, level, line, ts) FROM stdin;
\.


--
-- Data for Name: aa_client_package; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.aa_client_package (id, package_id, agent, version, platform, filename, file_size, sha256, official_url, local_path, status, fetched_by, created_at, updated_at, fetched_at) FROM stdin;
\.


--
-- Name: aa_client_agent_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_client_agent_id_seq', 3, true);


--
-- Name: aa_client_agent_status_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_client_agent_status_id_seq', 1, false);


--
-- Name: aa_client_agent_version_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_client_agent_version_id_seq', 3, true);


--
-- Name: aa_client_app_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_client_app_id_seq', 127, true);


--
-- Name: aa_client_app_perm_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_client_app_perm_id_seq', 1, false);


--
-- Name: aa_client_command_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_client_command_id_seq', 1, false);


--
-- Name: aa_client_config_change_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_client_config_change_id_seq', 1, false);


--
-- Name: aa_client_device_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_client_device_id_seq', 1, false);


--
-- Name: aa_client_device_mcp_group_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_client_device_mcp_group_id_seq', 1, false);


--
-- Name: aa_client_log_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_client_log_id_seq', 1, false);


--
-- Name: aa_client_package_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.aa_client_package_id_seq', 1, false);


--
-- Name: aa_client_agent aa_client_agent_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_client_agent
    ADD CONSTRAINT aa_client_agent_pkey PRIMARY KEY (id);


--
-- Name: aa_client_agent_status aa_client_agent_status_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_client_agent_status
    ADD CONSTRAINT aa_client_agent_status_pkey PRIMARY KEY (id);


--
-- Name: aa_client_agent_version aa_client_agent_version_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_client_agent_version
    ADD CONSTRAINT aa_client_agent_version_pkey PRIMARY KEY (id);


--
-- Name: aa_client_app_perm aa_client_app_perm_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_client_app_perm
    ADD CONSTRAINT aa_client_app_perm_pkey PRIMARY KEY (id);


--
-- Name: aa_client_app aa_client_app_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_client_app
    ADD CONSTRAINT aa_client_app_pkey PRIMARY KEY (id);


--
-- Name: aa_client_command aa_client_command_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_client_command
    ADD CONSTRAINT aa_client_command_pkey PRIMARY KEY (id);


--
-- Name: aa_client_config_change aa_client_config_change_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_client_config_change
    ADD CONSTRAINT aa_client_config_change_pkey PRIMARY KEY (id);


--
-- Name: aa_client_device_mcp_group aa_client_device_mcp_group_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_client_device_mcp_group
    ADD CONSTRAINT aa_client_device_mcp_group_pkey PRIMARY KEY (id);


--
-- Name: aa_client_device aa_client_device_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_client_device
    ADD CONSTRAINT aa_client_device_pkey PRIMARY KEY (id);


--
-- Name: aa_client_log aa_client_log_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_client_log
    ADD CONSTRAINT aa_client_log_pkey PRIMARY KEY (id);


--
-- Name: aa_client_package aa_client_package_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.aa_client_package
    ADD CONSTRAINT aa_client_package_pkey PRIMARY KEY (id);


--
-- Name: idx_aa_client_agent_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_client_agent_id ON public.aa_client_agent USING btree (agent_id);


--
-- Name: idx_aa_client_agent_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_client_agent_key ON public.aa_client_agent USING btree (agent);


--
-- Name: idx_aa_client_agent_status_device_agent; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_client_agent_status_device_agent ON public.aa_client_agent_status USING btree (device_id, agent);


--
-- Name: idx_aa_client_agent_status_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_client_agent_status_id ON public.aa_client_agent_status USING btree (status_id);


--
-- Name: idx_aa_client_agent_status_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_client_agent_status_user ON public.aa_client_agent_status USING btree (user_id);


--
-- Name: idx_aa_client_agent_version_agent_ver; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_client_agent_version_agent_ver ON public.aa_client_agent_version USING btree (agent_id, version);


--
-- Name: idx_aa_client_agent_version_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_client_agent_version_id ON public.aa_client_agent_version USING btree (version_id);


--
-- Name: idx_aa_client_app_bundle; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_client_app_bundle ON public.aa_client_app USING btree (bundle_id);


--
-- Name: idx_aa_client_app_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_client_app_id ON public.aa_client_app USING btree (app_id);


--
-- Name: idx_aa_client_app_perm_dev; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_client_app_perm_dev ON public.aa_client_app_perm USING btree (device_id, bundle_id);


--
-- Name: idx_aa_client_app_perm_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_client_app_perm_id ON public.aa_client_app_perm USING btree (perm_id);


--
-- Name: idx_aa_client_command_device; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_client_command_device ON public.aa_client_command USING btree (device_id, status, created_at);


--
-- Name: idx_aa_client_command_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_client_command_id ON public.aa_client_command USING btree (command_id);


--
-- Name: idx_aa_client_config_change_device; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_client_config_change_device ON public.aa_client_config_change USING btree (device_id, agent, changed_at DESC);


--
-- Name: idx_aa_client_config_change_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_client_config_change_id ON public.aa_client_config_change USING btree (change_id);


--
-- Name: idx_aa_client_device_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_client_device_id ON public.aa_client_device USING btree (device_id);


--
-- Name: idx_aa_client_device_mcp_device; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_client_device_mcp_device ON public.aa_client_device_mcp_group USING btree (device_id);


--
-- Name: idx_aa_client_device_mcp_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_client_device_mcp_id ON public.aa_client_device_mcp_group USING btree (device_mcp_id);


--
-- Name: idx_aa_client_device_mcp_pair; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_client_device_mcp_pair ON public.aa_client_device_mcp_group USING btree (device_id, group_id);


--
-- Name: idx_aa_client_device_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_client_device_user ON public.aa_client_device USING btree (user_id);


--
-- Name: idx_aa_client_log_device; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_aa_client_log_device ON public.aa_client_log USING btree (device_id, agent, ts DESC);


--
-- Name: idx_aa_client_log_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_client_log_id ON public.aa_client_log USING btree (log_id);


--
-- Name: idx_aa_client_package_cvp; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_client_package_cvp ON public.aa_client_package USING btree (agent, version, platform);


--
-- Name: idx_aa_client_package_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_aa_client_package_id ON public.aa_client_package USING btree (package_id);


--
-- PostgreSQL database dump complete
--

\unrestrict jFeHXhMOihIiWuG0NtCwX8eAGHlroqP27Edrz94276FBeVJc2adza5GopgzUvqe

